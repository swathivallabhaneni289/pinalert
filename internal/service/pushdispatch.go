// pushdispatch.go is the part of phone alerts that decides who should get a
// notification and queues it in the outbox (plan 03.3-09). The dispatcher
// queues; the worker (plan 11) sends. Nothing here talks to a push service.
//
// The dispatcher is poked after a report is saved or a vote is cast, and again
// by a sweep, but a poke carries no decision (D-07: never notify from Submit or
// CastVote). Every branch starts from loadPushFacts, which reads the report
// fresh and asks the same resolver the feed uses, with the stored moderation
// state set. So a push is never queued for a report the feed would not show.
//
// What reaches a phone (owner decision 2026-11-03, quick 261103-jal, D2 and D3):
// Critical reports near the home area, notes about people the reader follows
// with the Notify me bell on, and the confirmed update of those two. Severe
// official alerts have their own lane (pushworker_official.go). The stored
// critical_push, reliable_alerts and changes_push flags are kept and no longer
// decide anything; the device switch is the only way to stop the phone notes.
// Reliable reporter posts and changes or comments on a person's reports reach
// the bell only: their paths below refresh bells early and queue nothing.
//
// Kinds:
//   - Critical (kind 1) goes through MayNotifyAboutReport(UpdateCriticalNearby,
//     ...), the one predicate every notification uses (D-29 of 3.2).
//   - Reliable reporter (kind 4) is a bell refresh only: the dispatcher asks the
//     updates service to Refresh each subscribed neighbour so the
//     reliable_nearby bell row arrives early, and queues no phone note (D3).
//   - Follow (Phase 9 plan 25, D-09) is a report by a person the reader follows
//     with the Notify me bell on. The same one row, two channels shape: the
//     dispatcher asks the updates service to Refresh each follower of the
//     reporter who has the bell on, and pushes exactly the followed_posted rows
//     that step wrote. There is no second rule; the bell step applies
//     MayNotifyAboutReport(UpdateFollowedPosted, ...), the Reliable reporter
//     check and every mute, block and privacy rule. A follow note is not about
//     distance, so a follower needs no home area.
//   - Changed (kind 3, and the comment notes) is a bell refresh only. After a
//     vote the report's posters, voters and commenters are refreshed, and a
//     periodic sweep does the same for the accounts with a device, so the bell
//     rows exist early. No phone note is queued (D3); the bell poll stays the
//     producer for everyone else (D-06, D-13 of 3.2).
//   - Confirmed update (kind 5) is queued once for each initial alert that went
//     out saying Not confirmed yet, when ConfirmedByPeople becomes true. One
//     whose base is Reliable is still queued by SQL for an old sent note and the
//     worker skips it at send time.
//
// One first alert per person and report (D-09, research 5.2 item 7): when one
// report matches several kinds for one person only the first is queued, because
// the branches run in this order and uq_push_outbox_first allows one row per
// account and report across critical, follow and reliable:
//
//	Critical, then Follow, then the confirmed update. The Reliable and Changes
//	branches refresh bells only.
//
// Critical wins over Follow. The phone default is few notes that never pile up
// (D-25): at most one first alert per report, and the hourly cap counts follow
// pushes.
//
// Disputes: a Critical report that most nearby cells dispute stays Live and
// listed, because disputes cannot hide a Critical report. Its push follows the
// feed, so while it is fresh it is still queued and still reads Not confirmed
// yet, and ConfirmedByPeople, not a dispute count read here, keeps the confirmed
// update from going out. Push code holds no second dispute rule (review note N3).
//
// Caps (decision 6): at most PushCriticalReportsPerReporterPerHour Critical and
// PushReliableReportsPerReporterPerHour Reliable reports of one reporter fan
// out in a rolling hour, across all recipients. This stands in for TRUSTX-03
// burst detection, which is not built; the gap is recorded in docs by plan 15. A
// report over its cap is marked capped for good and stays in the feed.
//
// Choices a later reader would otherwise question:
//   - A report posted without an account has no reporter to count, so the
//     per reporter cap cannot apply to it. It is never pushed (it fails closed).
//     Posting needs a verified account, so this only meets rows that predate it.
//   - A fan out that reached nobody is not recorded, so it does not use up one
//     of its reporter's slots; the next run looks again.
//   - The sweep looks back PushChangedWindow, not only PushInitialWindow, so a
//     confirmation that arrives later (and a lost vote poke) still gets its
//     confirmed update. Initial alerts are limited to PushInitialWindow inside
//     EvaluateReport.
//   - Logs hold numeric ids and counts only: never a coordinate, an area name,
//     report text or an error that came from outside the database.

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// PushDispatchSweepInterval is the pause between sweeps over recent reports.
	// One more sweep runs at process start because the free host cold starts.
	PushDispatchSweepInterval = 30 * time.Second
	// PushChangesSweepInterval is the pause between sweeps that turn 3.2 update
	// rows into change pushes.
	PushChangesSweepInterval = 5 * time.Minute
	// PushPokeBuffer is how many pokes wait for the runner. A full buffer drops
	// the poke; the sweep recovers it.
	PushPokeBuffer = 64
	// PushAreaLookupTimeout bounds the reverse area lookup of one report (D-16).
	PushAreaLookupTimeout = 2 * time.Second
	// PushSweepReportCap is the most reports one sweep evaluates.
	PushSweepReportCap = 50
	// PushChangesSweepAccountCap is the most accounts one changes sweep serves.
	PushChangesSweepAccountCap = 500
	// FollowFanoutCap is the most followers (people with the Notify me bell on)
	// whose bell the follow pass refreshes for one report (T-09-87). It bounds
	// the work of one pass and does not promise that nobody past it gets a phone
	// note: the phone note is queued from every followed_posted row of the report
	// (EnqueueFollowFromUpdates), so a follower past the cap whose own poll
	// already wrote the row still gets it. The report is marked fanned once, so
	// a follower past the cap whose poll came after the pass gets the bell note
	// only. The followers are taken in follower id order (NotifyFollowersOf), so
	// the ones left out are always the highest ids (review K4 and IN-S05,
	// 2026-10-30; accepted at this scale).
	FollowFanoutCap = 500
)

// pushFanoutWindow is the rolling window of the per reporter fan out caps.
const pushFanoutWindow = time.Hour

// The two stored states of a fan out (push_report_progress).
const (
	pushStateFanned = "fanned"
	pushStateCapped = "capped"
)

// ReportChangeNotifier is what Submit and CastVote (wired by a later plan) call
// after they succeed. It never blocks and carries no decision.
type ReportChangeNotifier interface {
	// ReportChanged says a report was posted or voted on. actorAccountID is the
	// voter, or 0 for a new report.
	ReportChanged(reportID, actorAccountID int64)
}

// UpdatesRefresher is 3.2's per account refresh, the one rule that writes the
// updates rows both the bell and the phone read.
type UpdatesRefresher interface {
	Refresh(ctx context.Context, accountID int64) error
}

// AreaNamer is the reverse area lookup (plan 07).
type AreaNamer interface {
	ReverseArea(ctx context.Context, lat, lon float64) (string, error)
}

// PushDispatchQuerier is the store surface of the dispatcher.
type PushDispatchQuerier interface {
	PushFactsQuerier
	ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)
	GetPushReportProgress(ctx context.Context, reportID int64) (sqlcgen.PushReportProgress, error)
	MarkReportEvaluated(ctx context.Context, arg sqlcgen.MarkReportEvaluatedParams) error
	MarkCriticalFanout(ctx context.Context, arg sqlcgen.MarkCriticalFanoutParams) error
	MarkReliableFanout(ctx context.Context, arg sqlcgen.MarkReliableFanoutParams) error
	MarkFollowFanout(ctx context.Context, arg sqlcgen.MarkFollowFanoutParams) error
	MarkAreaLookupTried(ctx context.Context, arg sqlcgen.MarkAreaLookupTriedParams) error
	SetReportAreaName(ctx context.Context, arg sqlcgen.SetReportAreaNameParams) error
	CountReporterFanouts(ctx context.Context, arg sqlcgen.CountReporterFanoutsParams) (int64, error)
	PushHomeCandidates(ctx context.Context, arg sqlcgen.PushHomeCandidatesParams) ([]sqlcgen.PushHomeCandidatesRow, error)
	EnqueueInitialPush(ctx context.Context, arg sqlcgen.EnqueueInitialPushParams) (int64, error)
	EnqueueFollowFromUpdates(ctx context.Context, arg sqlcgen.EnqueueFollowFromUpdatesParams) (int64, error)
	NotifyFollowersOf(ctx context.Context, arg sqlcgen.NotifyFollowersOfParams) ([]int64, error)
	EnqueueConfirmedUpdates(ctx context.Context, arg sqlcgen.EnqueueConfirmedUpdatesParams) (int64, error)
	FollowersForBellRefresh(ctx context.Context, reportID int64) ([]int64, error)
	AccountsForBellRefresh(ctx context.Context, arg sqlcgen.AccountsForBellRefreshParams) ([]int64, error)
	RecentReportsForPush(ctx context.Context, arg sqlcgen.RecentReportsForPushParams) ([]int64, error)
}

var _ ReportChangeNotifier = (*PushDispatcher)(nil)
var _ PushDispatchQuerier = (*sqlcgen.Queries)(nil)
var _ UpdatesRefresher = (*UpdatesService)(nil)

// pushPoke is one "this report changed" message.
type pushPoke struct {
	reportID int64
	actor    int64
}

// PushDispatcher queues phone notifications. It sends nothing.
type PushDispatcher struct {
	q          PushDispatchQuerier
	updates    UpdatesRefresher
	area       AreaNamer
	now        func() time.Time
	sweepEvery time.Duration
	pokes      chan pushPoke

	// signals supplies extra reporter tag inputs (plan 09-13). Nil reads outcomes only.
	signals SignalSource

	// changesMu guards changesAfter, the last account id the changes sweep
	// served when its page was full. The next sweep starts after it, and a short
	// page wraps it back to 0, so every account is reached in turn (review S-04).
	changesMu    sync.Mutex
	changesAfter int64
}

// DispatchOption adjusts a PushDispatcher at construction.
type DispatchOption func(*PushDispatcher)

// WithDispatchSignals hands the dispatcher the source of extra reporter tag
// inputs (plan 09-13 is the first real one), so the Reliable check reads the
// same tag as the feed. With none it reads outcomes only.
func WithDispatchSignals(src SignalSource) DispatchOption {
	return func(d *PushDispatcher) {
		d.signals = src
	}
}

// WithDispatchClock replaces the clock. The database clock still decides what
// the queries compare against inside the outbox itself.
func WithDispatchClock(now func() time.Time) DispatchOption {
	return func(d *PushDispatcher) {
		if now != nil {
			d.now = now
		}
	}
}

// WithDispatchSweepInterval replaces PushDispatchSweepInterval.
func WithDispatchSweepInterval(every time.Duration) DispatchOption {
	return func(d *PushDispatcher) {
		if every > 0 {
			d.sweepEvery = every
		}
	}
}

// NewPushDispatcher builds a dispatcher. updates and area may be nil: a nil
// updates skips the Reliable and Changed refreshes, a nil area leaves every area
// name empty.
func NewPushDispatcher(q PushDispatchQuerier, updates UpdatesRefresher, area AreaNamer, opts ...DispatchOption) *PushDispatcher {
	d := &PushDispatcher{
		q:          q,
		updates:    updates,
		area:       area,
		now:        time.Now,
		sweepEvery: PushDispatchSweepInterval,
		pokes:      make(chan pushPoke, PushPokeBuffer),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(d)
		}
	}
	return d
}

// ReportChanged pokes the dispatcher. It never blocks: with the buffer full the
// poke is dropped, and the sweep recovers it.
func (d *PushDispatcher) ReportChanged(reportID, actorAccountID int64) {
	select {
	case d.pokes <- pushPoke{reportID: reportID, actor: actorAccountID}:
	default:
	}
}

// Run sweeps once at once (the host cold starts), then handles pokes and sweeps
// on two tickers until ctx ends. It logs only while ctx is live.
func (d *PushDispatcher) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	d.sweepAndLog(ctx)
	d.changesSweepAndLog(ctx)

	sweep := time.NewTicker(d.sweepEvery)
	defer sweep.Stop()
	changes := time.NewTicker(PushChangesSweepInterval)
	defer changes.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-d.pokes:
			if err := d.EvaluateReport(ctx, p.reportID, p.actor); err != nil && ctx.Err() == nil {
				log.Printf("push dispatch: report %d: %v", p.reportID, err)
			}
		case <-sweep.C:
			d.sweepAndLog(ctx)
		case <-changes.C:
			d.changesSweepAndLog(ctx)
		}
	}
}

func (d *PushDispatcher) sweepAndLog(ctx context.Context) {
	if err := d.SweepOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("push dispatch: sweep: %v", err)
	}
}

func (d *PushDispatcher) changesSweepAndLog(ctx context.Context) {
	if err := d.ChangesSweepOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("push dispatch: changes sweep: %v", err)
	}
}

// SweepOnce evaluates the recent reports with no actor, the never evaluated and
// least recently evaluated first, then the newest (review S-04). A failure
// on one report does not stop the rest; the errors come back joined.
func (d *PushDispatcher) SweepOnce(ctx context.Context) error {
	now := d.now().UTC()
	ids, err := d.q.RecentReportsForPush(ctx, sqlcgen.RecentReportsForPushParams{
		Since:  now.Add(-PushChangedWindow),
		Now:    now,
		RowCap: PushSweepReportCap,
	})
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		if err := d.EvaluateReport(ctx, id, 0); err != nil {
			errs = append(errs, fmt.Errorf("report %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// ChangesSweepOnce refreshes the bell of every account with a device, so the 3.2
// event rows exist early. It queues no phone note: since the owner decision of
// 2026-11-03 (quick 261103-jal, D3) change and comment notes stay in the bell.
//
// It serves at most PushChangesSweepAccountCap accounts, starting after the
// last account the previous full page served, and wraps to the start after a
// short page, so an account past the cap is reached on a later sweep instead
// of never (review S-04).
func (d *PushDispatcher) ChangesSweepOnce(ctx context.Context) error {
	d.changesMu.Lock()
	defer d.changesMu.Unlock()
	accounts, err := d.q.AccountsForBellRefresh(ctx, sqlcgen.AccountsForBellRefreshParams{
		AfterID: d.changesAfter,
		RowCap:  PushChangesSweepAccountCap,
	})
	if err != nil {
		return err
	}
	if len(accounts) >= PushChangesSweepAccountCap {
		d.changesAfter = accounts[len(accounts)-1]
	} else {
		d.changesAfter = 0
	}
	now := d.now().UTC()
	var errs []error
	for _, id := range accounts {
		if ctx.Err() != nil {
			break
		}
		if err := d.changesFor(ctx, id, now); err != nil {
			errs = append(errs, fmt.Errorf("account %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// evalRun is the state of one EvaluateReport call: the fresh facts, the stored
// progress as the call goes on, and what has been read once.
type evalRun struct {
	facts pushReportFacts
	now   time.Time
	actor int64

	critical string // stored or just written critical_state, "" while NULL
	reliable string // the same for reliable_state
	follow   string // the same for follow_state
	areaTry  bool   // the area lookup was already considered in this call
	areaDone bool   // the stored progress says it was tried
	cands    []sqlcgen.PushHomeCandidatesRow
	candsOK  bool
}

// EvaluateReport decides and queues everything about one report. actor is the
// account whose vote caused the call, or 0 for a new report or a sweep.
//
// Order: the facts, the evaluated mark, the Critical branch, the Follow branch,
// the Reliable branch, the changes branch (only with an actor) and the confirmed
// update. The first alert kinds are queued in that order, so a report that
// matches several goes out once, as the first of Critical and Follow. The
// Reliable and changes branches refresh bells early and queue nothing (owner
// decision 2026-11-03, quick 261103-jal, D3). Each branch is independent; their
// errors come back joined.
func (d *PushDispatcher) EvaluateReport(ctx context.Context, reportID, actorAccountID int64) error {
	now := d.now().UTC()
	facts, ok, err := loadPushFacts(ctx, d.q, reportID, now)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := d.q.MarkReportEvaluated(ctx, sqlcgen.MarkReportEvaluatedParams{ReportID: reportID, Now: now}); err != nil {
		return err
	}
	progress, err := d.q.GetPushReportProgress(ctx, reportID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	run := &evalRun{facts: facts, now: now, actor: actorAccountID, areaDone: progress.AreaLookupAt.Valid}
	if progress.CriticalState != nil {
		run.critical = *progress.CriticalState
	}
	if progress.ReliableState != nil {
		run.reliable = *progress.ReliableState
	}
	if progress.FollowState != nil {
		run.follow = *progress.FollowState
	}

	var errs []error
	if err := d.criticalBranch(ctx, run); err != nil {
		errs = append(errs, err)
	}
	if err := d.followBranch(ctx, run); err != nil {
		errs = append(errs, err)
	}
	if err := d.reliableBranch(ctx, run); err != nil {
		errs = append(errs, err)
	}
	if err := d.changesBranch(ctx, run); err != nil {
		errs = append(errs, err)
	}
	if err := d.confirmedBranch(ctx, run); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// candidates reads the home cell candidates once per call, nearest cell first.
func (d *PushDispatcher) candidates(ctx context.Context, run *evalRun) ([]sqlcgen.PushHomeCandidatesRow, error) {
	if run.candsOK {
		return run.cands, nil
	}
	rows, err := d.q.PushHomeCandidates(ctx, sqlcgen.PushHomeCandidatesParams{
		Cells:    candidateCells(run.facts.Lat, run.facts.Lon, PushCandidateMaxRadiusKm),
		ReportID: run.facts.ID,
		RowCap:   PushRecipientsPerReport * 2,
	})
	if err != nil {
		return nil, err
	}
	run.cands, run.candsOK = rows, true
	return rows, nil
}

// pushEarlier returns the earlier of two times.
func pushEarlier(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// criticalBranch queues the Critical alert for accounts whose home area lies
// within their chosen distance of a fresh Critical report. The stored
// critical_push flag is not read: every subscribed device gets it (owner
// decision 2026-11-03, quick 261103-jal, D2).
//
// A re run, the 30 second sweep with actor 0 or a later poke, evaluates a fanned
// report again so accounts that subscribed or set a home area since the first
// run are served. The unique index keeps one row per account and report, and the
// reporter count leaves the report itself out, so the re run never counts its
// own earlier fan out (review note N2). A fanned report is never turned into
// capped (MarkCriticalFanout only writes while the state is NULL); when the
// reporter's other reports alone reach the cap, the re run queues nothing more
// and the state stays fanned.
func (d *PushDispatcher) criticalBranch(ctx context.Context, run *evalRun) error {
	f := run.facts
	if !f.Meta.Serious() || f.ReporterAccountID == nil {
		return nil
	}
	if !MayNotifyAboutReport(UpdateCriticalNearby, f.Vis, f.ExpiresAt, run.now) {
		return nil
	}
	if run.now.Sub(f.CreatedAt) >= PushInitialWindow || run.critical == pushStateCapped {
		return nil
	}

	others, err := d.q.CountReporterFanouts(ctx, sqlcgen.CountReporterFanoutsParams{
		ReporterAccountID: *f.ReporterAccountID,
		ExcludeReportID:   f.ID,
		Kind:              string(PushKindCritical),
		Since:             run.now.Add(-pushFanoutWindow),
	})
	if err != nil {
		return err
	}
	if others >= PushCriticalReportsPerReporterPerHour {
		if run.critical == "" {
			if err := d.q.MarkCriticalFanout(ctx, sqlcgen.MarkCriticalFanoutParams{ReportID: f.ID, State: pushStateCapped, Now: run.now}); err != nil {
				return err
			}
			run.critical = pushStateCapped
		}
		return nil
	}

	rows, err := d.candidates(ctx, run)
	if err != nil {
		return err
	}
	recipients := filterRecipients(rows, f.Lat, f.Lon, f.ReporterAccountID, nil)
	if len(recipients) == 0 {
		return nil
	}
	d.ensureAreaName(ctx, run)

	expires := pushEarlier(f.ExpiresAt, run.now.Add(PushInitialWindow))
	for _, r := range recipients {
		_, err := d.q.EnqueueInitialPush(ctx, sqlcgen.EnqueueInitialPushParams{
			AccountID: r.AccountID,
			ReportID:  f.ID,
			Kind:      string(PushKindCritical),
			ExpiresAt: expires,
			Now:       run.now,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	if run.critical == "" {
		if err := d.q.MarkCriticalFanout(ctx, sqlcgen.MarkCriticalFanoutParams{ReportID: f.ID, State: pushStateFanned, Now: run.now}); err != nil {
			return err
		}
		run.critical = pushStateFanned
	}
	return nil
}

// followBranch carries the follow note to the phone (Phase 9 plan 25, D-09,
// research 5.2). It reads the reporter's followers who have the Notify me bell on
// (at most FollowFanoutCap), asks the updates service to Refresh each one, which
// is where the one rule writes the followed_posted row, and then queues a follow
// push for exactly the rows that step wrote for this report. The rule lives
// once, in the bell step; this branch only follows it (T-09-88). It runs after
// criticalBranch and before reliableBranch, so a follower who was already queued
// a Critical push keeps it and a follower who would also get the Reliable alert
// gets the follow note instead (uq_push_outbox_first holds one first alert).
//
// A pass is one that got past the reporter, predicate and window checks. It is
// recorded as fanned after one full pass whether or not any row was queued
// (review W2): a pass that found nobody must not be repeated on every later
// evaluation, which would refresh the followers each time. This differs on
// purpose from the Critical and Reliable branches, which mark a report fanned
// only when they found a recipient. A report that is Hidden or Withheld at its
// first evaluation fails the predicate, is not marked, and gets its one pass
// when it first becomes listable. A report with no reporter account is never
// pushed. A failed Refresh logs ids only and does not stop the pass.
func (d *PushDispatcher) followBranch(ctx context.Context, run *evalRun) error {
	f := run.facts
	if f.ReporterAccountID == nil || d.updates == nil {
		return nil
	}
	if run.follow != "" {
		return nil
	}
	if !MayNotifyAboutReport(UpdateFollowedPosted, f.Vis, f.ExpiresAt, run.now) {
		return nil
	}
	if run.now.Sub(f.CreatedAt) >= PushInitialWindow {
		return nil
	}

	followers, err := d.q.NotifyFollowersOf(ctx, sqlcgen.NotifyFollowersOfParams{
		FolloweeID: *f.ReporterAccountID,
		RowLimit:   FollowFanoutCap,
	})
	if err != nil {
		return err
	}
	if len(followers) > 0 {
		d.ensureAreaName(ctx, run)
		for _, id := range followers {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := d.updates.Refresh(ctx, id); err != nil {
				// The text of this error is not logged; it comes from a layer
				// that reads reports and places.
				log.Printf("push dispatch: follow refresh failed for account %d, report %d", id, f.ID)
			}
		}
		if _, err := d.q.EnqueueFollowFromUpdates(ctx, sqlcgen.EnqueueFollowFromUpdatesParams{
			ExpiresAt: pushEarlier(f.ExpiresAt, run.now.Add(PushOtherTTL)),
			Now:       run.now,
			ReportID:  f.ID,
		}); err != nil {
			return err
		}
	}
	if err := d.q.MarkFollowFanout(ctx, sqlcgen.MarkFollowFanoutParams{ReportID: f.ID, State: pushStateFanned, Now: run.now}); err != nil {
		return err
	}
	run.follow = pushStateFanned
	return nil
}

// reliableBranch asks 3.2's rule to write the reliable_nearby bell rows of the
// subscribed neighbours of a Reliable reporter's report, so the rows arrive
// early. It queues no phone note: since the owner decision of 2026-11-03 (quick
// 261103-jal, D3, C2) a Reliable reporter note never reaches a phone, and the
// stored reliable_alerts flag is not read. The bell poll covers everyone the
// early refresh does not reach (filterRecipients keeps the nearest
// PushRecipientsPerReport accounts). The tag check before it only avoids
// needless work: the rule itself decides again (D-29).
//
// One fan out per report (review note N4): a report capped under Critical is
// never sent through here, and a report fanned under Critical takes no Reliable
// slot, so it counts once toward its reporter's caps across both kinds. A
// serious report reaches nobody here, since its neighbours already got the
// Critical push.
func (d *PushDispatcher) reliableBranch(ctx context.Context, run *evalRun) error {
	f := run.facts
	if f.ReporterAccountID == nil {
		return nil
	}
	if !MayNotifyAboutReport(UpdateReliableNearby, f.Vis, f.ExpiresAt, run.now) {
		return nil
	}
	if run.now.Sub(f.CreatedAt) >= PushInitialWindow || run.reliable == pushStateCapped {
		return nil
	}
	// A sweep leaves a fanned report alone, so it never repeats the history read
	// and the refreshes; a poke from a post or a vote still evaluates it fully.
	if run.reliable == pushStateFanned && run.actor == 0 {
		return nil
	}

	isReliable, err := d.reporterIsReliable(ctx, *f.ReporterAccountID, run.now)
	if err != nil {
		return err
	}
	if !isReliable {
		return nil
	}

	if run.critical == pushStateCapped {
		if run.reliable == "" {
			if err := d.q.MarkReliableFanout(ctx, sqlcgen.MarkReliableFanoutParams{ReportID: f.ID, State: pushStateCapped, Now: run.now}); err != nil {
				return err
			}
			run.reliable = pushStateCapped
		}
		return nil
	}
	if run.critical != pushStateFanned {
		others, err := d.q.CountReporterFanouts(ctx, sqlcgen.CountReporterFanoutsParams{
			ReporterAccountID: *f.ReporterAccountID,
			ExcludeReportID:   f.ID,
			Kind:              string(PushKindReliable),
			Since:             run.now.Add(-pushFanoutWindow),
		})
		if err != nil {
			return err
		}
		if others >= PushReliableReportsPerReporterPerHour {
			if run.reliable == "" {
				if err := d.q.MarkReliableFanout(ctx, sqlcgen.MarkReliableFanoutParams{ReportID: f.ID, State: pushStateCapped, Now: run.now}); err != nil {
					return err
				}
				run.reliable = pushStateCapped
			}
			return nil
		}
	}

	serious := f.Meta.Serious()
	rows, err := d.candidates(ctx, run)
	if err != nil {
		return err
	}
	recipients := filterRecipients(rows, f.Lat, f.Lon, f.ReporterAccountID, func(sqlcgen.PushHomeCandidatesRow) bool {
		return !serious
	})
	if len(recipients) == 0 {
		return nil
	}
	d.ensureAreaName(ctx, run)

	if d.updates != nil {
		for _, r := range recipients {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := d.updates.Refresh(ctx, r.AccountID); err != nil {
				// The text of this error is not logged; it comes from a layer
				// that reads reports and places.
				log.Printf("push dispatch: refresh failed for account %d, report %d", r.AccountID, f.ID)
			}
		}
	}
	if run.reliable == "" {
		if err := d.q.MarkReliableFanout(ctx, sqlcgen.MarkReliableFanoutParams{ReportID: f.ID, State: pushStateFanned, Now: run.now}); err != nil {
			return err
		}
		run.reliable = pushStateFanned
	}
	return nil
}

// changesBranch refreshes the bell of the report's followers (the person who
// posted it, anyone who voted on it and anyone who commented) except the voter
// who caused the call, so their event rows arrive early. It queues no phone note
// (owner decision 2026-11-03, quick 261103-jal, D3, C3). It runs only with an
// actor: the sweep covers the rest.
func (d *PushDispatcher) changesBranch(ctx context.Context, run *evalRun) error {
	if run.actor == 0 || run.facts.Vis == VisibilityWithheld {
		return nil
	}
	followers, err := d.q.FollowersForBellRefresh(ctx, run.facts.ID)
	if err != nil {
		return err
	}
	var errs []error
	for _, id := range followers {
		if id == run.actor {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := d.changesFor(ctx, id, run.now); err != nil {
			errs = append(errs, fmt.Errorf("account %d: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// changesFor brings one account's 3.2 rows up to date so the bell rows exist
// early. It queues nothing: change and comment notes stay in the bell (owner
// decision 2026-11-03, quick 261103-jal, D3, C3). now stays in the signature for
// the callers.
func (d *PushDispatcher) changesFor(ctx context.Context, accountID int64, _ time.Time) error {
	if d.updates != nil {
		if err := d.updates.Refresh(ctx, accountID); err != nil {
			log.Printf("push dispatch: refresh failed for account %d", accountID)
		}
	}
	return nil
}

// confirmedBranch queues the in place confirmed update when ConfirmedByPeople is
// true: one row for each initial alert that was sent saying Not confirmed yet.
// The query leaves out an alert that is still queued or whose words did not say
// so, and the unique index keeps it to one.
func (d *PushDispatcher) confirmedBranch(ctx context.Context, run *evalRun) error {
	if !run.facts.Confirmed {
		return nil
	}
	_, err := d.q.EnqueueConfirmedUpdates(ctx, sqlcgen.EnqueueConfirmedUpdatesParams{
		ExpiresAt: pushEarlier(run.facts.ExpiresAt, run.now.Add(PushOtherTTL)),
		Now:       run.now,
		ReportID:  run.facts.ID,
	})
	return err
}

// reporterIsReliable reads the one reporter's history and returns whether the
// feed would tag them Reliable reporter: the same history read, the same votes
// and the same signals loader as the feed (LoadReporterSignals), so a source
// of extra inputs changes this check with every other surface. A reporter with
// no history reads as the none tag, which is not Reliable.
func (d *PushDispatcher) reporterIsReliable(ctx context.Context, accountID int64, now time.Time) (bool, error) {
	signals, err := LoadReporterSignals(ctx, d.q, d.signals, []int64{accountID}, now)
	if err != nil {
		return false, err
	}
	return signals[accountID].Tag == ReporterReliable, nil
}

// ensureAreaName looks up the coarse area name of the report once, after it is
// saved and only when something is about to be pushed (D-16, D-19). The attempt
// is recorded first, so a failing lookup is not retried on every sweep; a
// failure or an empty answer leaves the name empty and the push says distance
// only. The error text is not logged.
func (d *PushDispatcher) ensureAreaName(ctx context.Context, run *evalRun) {
	if run.areaTry {
		return
	}
	run.areaTry = true
	if d.area == nil || run.areaDone || run.facts.AreaName != "" {
		return
	}
	// The bounded lookup is shared with Share (areaname.go): the attempt is
	// recorded first, the call is bounded and trimmed, and the error text is
	// not logged.
	if name := LookUpReportAreaName(ctx, d.q, d.area, run.facts.ID, run.facts.Lat, run.facts.Lon, run.now); name != "" {
		run.facts.AreaName = name
	}
}
