package demo

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"pinalert/internal/service"
)

// Synthetic account ids. They exist only inside a World's memory and are never
// written to any table. They sit far above any id the accounts sequence will
// reach, so a synthetic id can never be mistaken for a real account.
const (
	// VisitorAccountID is the one account the visitor acts as in a Demo.
	VisitorAccountID int64 = 1_000_000_001
	// NeighbourAccountBase is the first of the pretend neighbours who confirm.
	NeighbourAccountBase int64 = 1_000_000_100
	// SeedReporterAccountBase is the first of the accounts that posted the sample reports.
	SeedReporterAccountBase int64 = 1_000_000_200
	// SeedVoterAccountBase is the first of the accounts that voted on the sample reports.
	SeedVoterAccountBase int64 = 1_000_000_300
)

// VisitorSessionPrefix starts every session id a world uses. The session of an
// account is the prefix plus its decimal id (SessionFor), so it can never equal
// the random session id of a real visitor.
const VisitorSessionPrefix = "demo-"

// World is one visit's sandbox. Reports and Votes are the real services, built
// on the in memory querier with no text screener and no change notifier: the
// practice text is checked by the local prefilter only and goes to no vendor,
// and no phone alert is ever queued from a Demo.
type World struct {
	ID      string
	Reports *service.ReportService
	Votes   *service.VotingService

	q      *memQuerier
	wall   func() time.Time
	offset atomic.Int64 // nanoseconds added to the wall clock

	// mu guards the neighbour plans and the visitor's activity. It is never held
	// by the querier, and the querier never calls back into the World, so holding
	// it across a service call cannot deadlock.
	mu        sync.Mutex
	plans     map[int64][]PlannedVote // report id to the neighbour votes still to come, in DueAt order
	posted    []int64
	confirmed []int64
	disputed  []int64

	// created and lastUsed are kept by the Store under its own lock.
	created  time.Time
	lastUsed time.Time
}

// NewWorld builds an empty world. wallNow is the clock the world is offset
// from; nil means time.Now.
func NewWorld(id string, wallNow func() time.Time) *World {
	if wallNow == nil {
		wallNow = time.Now
	}
	w := &World{ID: id, wall: wallNow, plans: make(map[int64][]PlannedVote)}
	w.q = newMemQuerier(w.Now)
	w.Reports = service.NewReportService(w.q, service.WithReportClock(w.Now))
	w.Votes = service.NewVotingService(w.q, service.WithVotingClock(w.Now))
	return w
}

// Now is the world clock: the wall clock plus the offset. It takes no lock the
// querier holds, so the querier may call it while locked.
func (w *World) Now() time.Time {
	return w.wall().Add(time.Duration(w.offset.Load()))
}

// SetClockOffset moves the world clock to the wall clock plus d. Seeding a
// world sets it to the past while the sample is written ("20 minutes ago"), then
// back to zero.
func (w *World) SetClockOffset(d time.Duration) {
	w.offset.Store(int64(d))
}

// SessionFor returns the session id the world uses for accountID and binds it
// to that account, so a report posted under it has that reporter. Calling it
// again for the same account gives the same session.
func (w *World) SessionFor(accountID int64) string {
	session := VisitorSessionPrefix + strconv.FormatInt(accountID, 10)
	w.q.BindSession(session, accountID)
	return session
}

// PostPractice posts the visitor's practice report through the real
// SubmitChecked, as the visitor's synthetic account, and plans the pretend
// neighbours who will confirm it. The session is VisitorSessionPrefix plus the
// world id, bound to the visitor's account so the real self vote block knows who
// posted it. A replay of the same request id plans nothing a second time. A
// refusal by the real service (validation, the gibberish check, ErrWorldFull)
// is returned unchanged.
func (w *World) PostPractice(ctx context.Context, in service.SubmitInput) (service.SubmitResult, error) {
	session := VisitorSessionPrefix + w.ID
	w.q.BindSession(session, VisitorAccountID)

	w.mu.Lock()
	defer w.mu.Unlock()
	res, err := w.Reports.SubmitChecked(ctx, session, VisitorAccountID, in)
	if err != nil {
		return res, err
	}
	if res.Replayed {
		return res, nil
	}
	w.posted = append(w.posted, res.Report.ID)
	w.plans[res.Report.ID] = PlanNeighbours(res.Report.ID, res.Report.Latitude, res.Report.Longitude, res.Report.CreatedAt)
	return res, nil
}

// Vote casts the visitor's vote through the real CastVote. The account is always
// the visitor's, whatever in.AccountID says, so a caller cannot vote as a
// neighbour or a seed voter. Nothing is exempted: the real distance rule, the
// self vote block and the expiry rule all apply. A vote that the service accepts
// is remembered for Activity. Neighbour votes that have fallen due are applied
// first, so the answer reflects the world as of now.
func (w *World) Vote(ctx context.Context, in service.CastVoteInput) (service.CastVoteResult, error) {
	if err := w.ApplyDue(ctx); err != nil {
		return service.CastVoteResult{}, err
	}
	in.AccountID = VisitorAccountID
	res, err := w.Votes.CastVote(ctx, in)
	if err != nil {
		return res, err
	}
	if in.Kind == service.VoteKindContent {
		w.mu.Lock()
		switch in.Value {
		case service.VoteConfirm:
			w.confirmed = addID(w.confirmed, in.ReportID)
			w.disputed = removeID(w.disputed, in.ReportID)
		case service.VoteDispute:
			w.disputed = addID(w.disputed, in.ReportID)
			w.confirmed = removeID(w.confirmed, in.ReportID)
		}
		w.mu.Unlock()
	}
	return res, nil
}

// Nearby reads the reports around q as the visitor sees them, after applying the
// neighbour votes that have fallen due. The viewer is always the visitor's
// synthetic account, whatever q says, so a caller cannot ask what another
// account voted. The real service does the reading.
func (w *World) Nearby(ctx context.Context, q service.NearbyQuery) ([]service.ReportView, error) {
	if err := w.ApplyDue(ctx); err != nil {
		return nil, err
	}
	q.ViewerAccountID = VisitorAccountID
	return w.Reports.Nearby(ctx, q)
}

// ApplyDue casts, in DueAt order, every planned neighbour vote whose time is not
// after the world clock, through the real CastVote with the neighbour's own
// account and spot. After each vote the report is judged by what the service
// answered: once its distinct confirming cells reach
// service.IndependentAgreementThreshold the rest of that report's plan is
// dropped, so the neighbours stop exactly where the real rule says the report is
// confirmed. A vote the service refuses (the report expired, the world is full)
// drops that report's plan instead of failing the read: a later read must not
// break because a pretend neighbour could not vote. Only a cancelled context is
// returned. It starts no goroutine and no timer; time is only read.
func (w *World) ApplyDue(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.plans) == 0 {
		return nil
	}
	now := w.Now()
	var due []PlannedVote
	for _, plan := range w.plans {
		for _, pv := range plan {
			if !pv.DueAt.After(now) {
				due = append(due, pv)
			}
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		if !due[i].DueAt.Equal(due[j].DueAt) {
			return due[i].DueAt.Before(due[j].DueAt)
		}
		if due[i].ReportID != due[j].ReportID {
			return due[i].ReportID < due[j].ReportID
		}
		return due[i].AccountID < due[j].AccountID
	})
	for _, pv := range due {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, still := w.plans[pv.ReportID]; !still {
			continue // dropped earlier in this pass
		}
		w.popPlanned(pv)
		res, err := w.Votes.CastVote(ctx, service.CastVoteInput{
			ReportID:  pv.ReportID,
			AccountID: pv.AccountID,
			Kind:      service.VoteKindContent,
			Value:     service.VoteConfirm,
			Latitude:  pv.Lat,
			Longitude: pv.Lon,
		})
		if err != nil || res.Trust.ConfirmedPlaces >= service.IndependentAgreementThreshold {
			delete(w.plans, pv.ReportID)
		}
	}
	return nil
}

// popPlanned removes pv from its report's plan and drops an emptied plan.
func (w *World) popPlanned(pv PlannedVote) {
	plan := w.plans[pv.ReportID]
	kept := plan[:0:0]
	for _, p := range plan {
		if p.AccountID != pv.AccountID {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		delete(w.plans, pv.ReportID)
		return
	}
	w.plans[pv.ReportID] = kept
}

// pendingNeighbourVotes is how many planned neighbour votes have not been cast.
func (w *World) pendingNeighbourVotes() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, plan := range w.plans {
		n += len(plan)
	}
	return n
}

// Activity is what the visitor did in this world: the practice reports posted and
// the reports confirmed or disputed (a report is in at most one of the last two,
// the visitor's newest content vote).
type Activity struct {
	Posted    []int64
	Confirmed []int64
	Disputed  []int64
}

// Activity returns copies of the visitor's practice report ids and of the ids the
// visitor confirmed or disputed, in the order they were done.
func (w *World) Activity() Activity {
	w.mu.Lock()
	defer w.mu.Unlock()
	return Activity{
		Posted:    append([]int64{}, w.posted...),
		Confirmed: append([]int64{}, w.confirmed...),
		Disputed:  append([]int64{}, w.disputed...),
	}
}

func addID(ids []int64, id int64) []int64 {
	for _, x := range ids {
		if x == id {
			return ids
		}
	}
	return append(ids, id)
}

func removeID(ids []int64, id int64) []int64 {
	out := ids[:0:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}
