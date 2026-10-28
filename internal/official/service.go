package official

// The official alerts service: the read side of the official feed. Alerts
// answers "which official alerts cover me, and can I trust that answer right
// now" for one signed in reader, and Kick keeps SACHET fresh on every read,
// quietly, so a host that sleeps between cron calls still catches up (the third
// refresh trigger, D-15, research G1 layer 3).
//
// The reader's position arrives only as arguments and is turned into a district
// at once. It is never stored and never logged, and the home area cell is read
// from the account row, used to find districts and never returned. Logs here
// hold the feed name and one closed class word.

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/areas"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// kickTimeout bounds one background refresh. Render's free host allows a
// request to run a minute, and the refresher bounds the CAP fetches itself.
const kickTimeout = 55 * time.Second

// Config is what the service knows about this deployment, used only to say who
// is calling NDMA.
type Config struct {
	ContactEmail string
	BaseURL      string
}

// Service reads the official alerts for a reader and refreshes them lazily.
// Safe for concurrent use.
type Service struct {
	refresher *Refresher
	q         *sqlcgen.Queries
	ix        *areas.Index
	now       func() time.Time
	kicking   atomic.Bool
	// newTicker builds Run's ticker; nil means a real time.Ticker. Only tests set it.
	newTicker func(d time.Duration) (<-chan time.Time, func())
}

// NewService builds the SACHET client and the refresher over the database. The
// options come after the SACHET registration and the push hook, so a later plan
// can add its own version hook or register another feed here without main.go
// changing, and a test can replace the SACHET fetcher by registering its own.
//
// The push hook (plan 05-11) queues phone alerts inside the transaction of each
// stored alert version, so it is wired here and nowhere else.
func NewService(pool TxBeginner, q *sqlcgen.Queries, cfg Config, opts ...RefresherOption) *Service {
	ix := areas.Default()
	client := NewSachetClient(ClientConfig{ContactEmail: cfg.ContactEmail, BaseURL: cfg.BaseURL})
	all := make([]RefresherOption, 0, len(opts)+2)
	all = append(all, WithSachet(client, ix))
	all = append(all, WithVersionHook(PushEnqueueHook(areas.Default())))
	all = append(all, opts...)
	return newService(NewRefresher(pool, q, all...), q, ix)
}

func newService(r *Refresher, q *sqlcgen.Queries, ix *areas.Index) *Service {
	return &Service{refresher: r, q: q, ix: ix, now: time.Now}
}

// Refresher returns the refresher the service owns.
func (s *Service) Refresher() *Refresher { return s.refresher }

// TickerInterval is the pause between the sweeps Run makes while the host is
// awake (plan 05-12, D-15, research G1 layer 2). It is longer than MinRefreshGap,
// so a sweep right after a cron call finds the feed not due and does nothing.
const TickerInterval = 5 * time.Minute

// RefreshAll runs every registered feed through RefreshIfDue, one after the
// other in the refresher's fixed order, so each feed's minimum gap, single flight
// and CAP bound apply however often this is called. A feed that failed carries
// its closed word in ErrorClass; one failure does not stop the others. The cron
// endpoint and the ticker call this and nothing else (the lazy kick refreshes
// SACHET alone).
func (s *Service) RefreshAll(ctx context.Context) []Result {
	feeds := s.refresher.Feeds()
	out := make([]Result, 0, len(feeds))
	for _, feed := range feeds {
		res, err := s.refresher.RefreshIfDue(ctx, feed)
		res.Feed = feed
		if err != nil && res.ErrorClass == "" {
			res.ErrorClass = ErrorClass(err)
		}
		out = append(out, res)
	}
	return out
}

// Run keeps the feeds fresh while the host is awake: one sweep at start, because
// the free host cold starts, then one every TickerInterval until ctx ends. A
// sleeping host refreshes only when a request, usually the external cron, wakes
// it. Each sweep is bounded by kickTimeout and a panic in one is logged and the
// loop goes on. Like the moderation worker it takes a context so a test can end it.
func (s *Service) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.sweep(ctx)
	every := s.newTicker
	if every == nil {
		every = realTicker
	}
	ticks, stop := every(TickerInterval)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			s.sweep(ctx)
		}
	}
}

// sweep is one RefreshAll on a context of its own deadline. The refresher logs a
// failed run itself (feed name and class word), and nobody is waiting for the
// result, so it is dropped.
func (s *Service) sweep(ctx context.Context) {
	defer func() {
		if recover() != nil {
			log.Printf("official: ticker sweep class=panic")
		}
	}()
	sctx, cancel := context.WithTimeout(ctx, kickTimeout)
	defer cancel()
	_ = s.RefreshAll(sctx)
}

// realTicker is a time.Ticker as a channel and a stop function.
func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Alerts answers for one reader. lat and lon are both set or both nil; a point
// outside every district counts as no point. The home area is read from the
// account. The feed state comes from the stored last success only.
func (s *Service) Alerts(ctx context.Context, accountID int64, lat, lon *float64) (View, error) {
	now := s.now()
	status, err := s.refresher.Status(ctx, FeedSachet)
	if err != nil {
		return View{}, err
	}

	var reader *Area
	if lat != nil && lon != nil {
		if d, ok := s.ix.Locate(*lat, *lon); ok {
			reader = districtArea(d)
		}
	}
	var home *Area
	cell, err := s.q.GetAccountHomeCell(ctx, accountID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return View{}, err
	case cell != nil:
		home = s.homeArea(*cell)
	}

	in := ViewInput{Now: now, Status: status, Reader: reader, Home: home}
	if state, _ := State(status.LastSuccessAt, now); state != StateUnavailable && (reader != nil || home != nil) {
		var districts, states []int32
		for _, a := range []*Area{reader, home} {
			if a == nil {
				continue
			}
			for _, c := range a.DistrictCodes {
				districts = append(districts, int32(c))
			}
			states = append(states, int32(a.StateCode))
		}
		rows, err := s.q.OfficialAlertsInForce(ctx, sqlcgen.OfficialAlertsInForceParams{
			Now: now, DistrictCodes: districts, StateCodes: states,
		})
		if err != nil {
			return View{}, err
		}
		in.Alerts = make([]StoredAlert, 0, len(rows))
		for _, r := range rows {
			in.Alerts = append(in.Alerts, s.storedAlert(r))
		}
	}
	return BuildView(in), nil
}

// Kick starts a background refresh of SACHET when the last attempt is more than
// LazyKickAfter old. It returns at once, runs at most one refresh at a time on
// a context of its own (the request that triggered it may end first), and never
// reports an error; a failure is logged as the feed name and one class word.
// The refresher's own minimum gap and single flight still apply, so however
// many readers kick, NDMA is asked rarely.
func (s *Service) Kick() {
	if !s.kicking.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.kicking.Store(false)
		defer func() {
			if recover() != nil {
				log.Printf("official: lazy refresh feed=%s class=panic", FeedSachet)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), kickTimeout)
		defer cancel()

		st, err := s.refresher.Status(ctx, FeedSachet)
		if err != nil {
			log.Printf("official: lazy refresh feed=%s class=%s", FeedSachet, ErrorClass(err))
			return
		}
		if st.LastAttemptAt != nil && s.now().Sub(*st.LastAttemptAt) < LazyKickAfter {
			return
		}
		// RefreshIfDue logs a failed run itself (feed name and class word), and
		// a lazy read has nobody to tell, so its result is dropped.
		_, _ = s.refresher.RefreshIfDue(ctx, FeedSachet)
	}()
}

// homeArea turns a stored 5 character home cell into the districts found at its
// centre and its four corners, so a cell on a district edge counts for both
// sides (research G3, biased toward telling the person). The name and state
// are those of the district at the centre. It is nil for anything that is not
// such a cell and for a cell outside every district. The cell itself is never
// returned.
func (s *Service) homeArea(cell string) *Area {
	districts := s.ix.CellDistricts(cell)
	if len(districts) == 0 {
		return nil
	}
	primary := districts[0]
	if lat, lon, ok := service.HomeAreaAnchor(cell); ok {
		if d, found := s.ix.Locate(lat, lon); found {
			primary = d
		}
	}
	a := districtArea(primary)
	a.DistrictCodes = make([]int, 0, len(districts))
	for _, d := range districts {
		a.DistrictCodes = append(a.DistrictCodes, d.Code)
	}
	return a
}

// districtArea is the area of one district in our own boundary data names.
func districtArea(d areas.District) *Area {
	return &Area{
		DistrictCodes: []int{d.Code},
		DistrictName:  d.Name,
		StateCode:     d.StateCode,
		StateName:     titleName(d.State),
	}
}

// storedAlert converts one in force row, with the boundary names of its
// districts, into the view's input.
func (s *Service) storedAlert(r sqlcgen.OfficialAlertsInForceRow) StoredAlert {
	a := StoredAlert{
		ID:                  r.ID,
		Source:              sourceSachet,
		Sender:              r.Sender,
		Issuer:              r.Issuer,
		Event:               r.Event,
		Severity:            r.Severity,
		SentAt:              r.SentAt.UTC(),
		ExpiresAt:           r.ExpiresAt.UTC(),
		ExpiresInferred:     r.ExpiresInferred,
		HeadlineEN:          r.HeadlineEn,
		HeadlineOriginal:    r.HeadlineOriginal,
		OriginalLang:        r.OriginalLang,
		DescriptionEN:       r.DescriptionEn,
		DescriptionOriginal: r.DescriptionOriginal,
		Instruction:         r.Instruction,
		AreaDesc:            r.AreaDesc,
		Tier:                Tier(r.PlacementTier),
	}
	for _, c := range r.DistrictCodes {
		a.DistrictCodes = append(a.DistrictCodes, int(c))
		name := ""
		if d, ok := s.ix.ByCode(int(c)); ok {
			name = d.Name
		}
		a.DistrictNames = append(a.DistrictNames, name)
	}
	for _, c := range r.StateCodes {
		a.StateCodes = append(a.StateCodes, int(c))
	}
	return a
}
