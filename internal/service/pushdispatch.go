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
// Kinds:
//   - Critical (kind 1) goes through MayNotifyAboutReport(UpdateCriticalNearby,
//     ...), the one predicate every notification uses (D-29 of 3.2).
//   - Reliable reporter (kind 4) goes through MayNotifyAboutReport(
//     UpdateReliableNearby, ...) and then through 3.2's own rule: the dispatcher
//     asks the updates service to Refresh each nearby account that turned the
//     switch on and pushes exactly the reliable_nearby rows that rule wrote. One
//     row, two channels, never a second rule (D-06, owner amendment 2026-10-18).
//   - Changed (kind 3) is pushed from the same 3.2 updates rows the bell reads.
//     After a vote the report's followers are refreshed and their new event rows
//     are queued, and a periodic sweep does the same for fading reports and for
//     rows a bell poll wrote first, so a race with the bell never gives two
//     pushes (D-06, D-13 of 3.2).
//   - Confirmed update (kind 5) is queued once for each initial alert that went
//     out saying Not confirmed yet, when ConfirmedByPeople becomes true.
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
	"strings"
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
	MarkAreaLookupTried(ctx context.Context, arg sqlcgen.MarkAreaLookupTriedParams) error
	SetReportAreaName(ctx context.Context, arg sqlcgen.SetReportAreaNameParams) error
	CountReporterFanouts(ctx context.Context, arg sqlcgen.CountReporterFanoutsParams) (int64, error)
	PushHomeCandidates(ctx context.Context, arg sqlcgen.PushHomeCandidatesParams) ([]sqlcgen.PushHomeCandidatesRow, error)
	EnqueueInitialPush(ctx context.Context, arg sqlcgen.EnqueueInitialPushParams) (int64, error)
	EnqueueReliableFromUpdates(ctx context.Context, arg sqlcgen.EnqueueReliableFromUpdatesParams) (int64, error)
	EnqueueChangedFromUpdates(ctx context.Context, arg sqlcgen.EnqueueChangedFromUpdatesParams) (int64, error)
	EnqueueConfirmedUpdates(ctx context.Context, arg sqlcgen.EnqueueConfirmedUpdatesParams) (int64, error)
	FollowersWithChangesPush(ctx context.Context, reportID int64) ([]int64, error)
	AccountsWithChangesPush(ctx context.Context, rowCap int32) ([]int64, error)
	RecentReportsForPush(ctx context.Context, arg sqlcgen.RecentReportsForPushParams) ([]int64, error)
	GetPushKindSettings(ctx context.Context, accountID int64) (sqlcgen.GetPushKindSettingsRow, error)
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
}

// DispatchOption adjusts a PushDispatcher at construction.
type DispatchOption func(*PushDispatcher)

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

// SweepOnce evaluates the recent reports with no actor, newest first. A failure
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

// ChangesSweepOnce turns the 3.2 event rows of every account that wants change
// pushes into outbox rows. It catches fading reports and rows a bell poll wrote
// first; the unique index makes it idempotent.
func (d *PushDispatcher) ChangesSweepOnce(ctx context.Context) error {
	accounts, err := d.q.AccountsWithChangesPush(ctx, PushChangesSweepAccountCap)
	if err != nil {
		return err
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
	areaTry  bool   // the area lookup was already considered in this call
	areaDone bool   // the stored progress says it was tried
	cands    []sqlcgen.PushHomeCandidatesRow
	candsOK  bool
}

// EvaluateReport decides and queues everything about one report. actor is the
// account whose vote caused the call, or 0 for a new report or a sweep.
//
// Order: the facts, the evaluated mark, the Critical branch, the Reliable
// branch (Critical rows are queued first, so a report that is both goes out as
// Critical), the changes branch (only with an actor) and the confirmed update.
// Each branch is independent; their errors come back joined.
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

	var errs []error
	if err := d.criticalBranch(ctx, run); err != nil {
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

// earlier returns the earlier of two times.
func earlier(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// criticalBranch queues the Critical alert for accounts whose home area lies
// within their chosen distance of a fresh Critical report.
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
	recipients := filterRecipients(rows, f.Lat, f.Lon, f.ReporterAccountID, func(r sqlcgen.PushHomeCandidatesRow) bool { return r.CriticalPushOn })
	if len(recipients) == 0 {
		return nil
	}
	d.ensureAreaName(ctx, run)

	expires := earlier(f.ExpiresAt, run.now.Add(PushInitialWindow))
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

// reliableBranch asks 3.2's rule to write the reliable_nearby rows of the
// accounts that turned the switch on, then queues a push for each row the rule
// wrote for this report. The tag check before it only avoids needless work: the
// rule itself decides again (D-29).
//
// One fan out per report (review note N4): a report capped under Critical is
// never sent through here, and a report fanned under Critical takes no Reliable
// slot, so it counts once toward its reporter's caps across both kinds. For a
// serious report this branch only reaches accounts with Critical off, since an
// account with Critical on already got its one initial alert as Critical.
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
	recipients := filterRecipients(rows, f.Lat, f.Lon, f.ReporterAccountID, func(r sqlcgen.PushHomeCandidatesRow) bool {
		return r.ReliableAlertsOn && !(serious && r.CriticalPushOn)
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
	if _, err := d.q.EnqueueReliableFromUpdates(ctx, sqlcgen.EnqueueReliableFromUpdatesParams{
		ExpiresAt: earlier(f.ExpiresAt, run.now.Add(PushOtherTTL)),
		Now:       run.now,
		ReportID:  f.ID,
	}); err != nil {
		return err
	}
	if run.reliable == "" {
		if err := d.q.MarkReliableFanout(ctx, sqlcgen.MarkReliableFanoutParams{ReportID: f.ID, State: pushStateFanned, Now: run.now}); err != nil {
			return err
		}
		run.reliable = pushStateFanned
	}
	return nil
}

// changesBranch refreshes the report's followers (the person who posted it and
// anyone who voted on it) except the voter who caused the call, and queues
// their new event rows. It runs only with an actor: the sweep covers the rest.
func (d *PushDispatcher) changesBranch(ctx context.Context, run *evalRun) error {
	if run.actor == 0 || run.facts.Vis == VisibilityWithheld {
		return nil
	}
	followers, err := d.q.FollowersWithChangesPush(ctx, run.facts.ID)
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

// changesFor brings one account's 3.2 rows up to date and queues the event rows
// after the later of the moment Changes went on and the changed window.
func (d *PushDispatcher) changesFor(ctx context.Context, accountID int64, now time.Time) error {
	if d.updates != nil {
		if err := d.updates.Refresh(ctx, accountID); err != nil {
			log.Printf("push dispatch: refresh failed for account %d", accountID)
		}
	}
	settings, err := d.q.GetPushKindSettings(ctx, accountID)
	if err != nil {
		return err
	}
	since := now.Add(-PushChangedWindow)
	if settings.ChangesEnabledAt.Valid && settings.ChangesEnabledAt.Time.After(since) {
		since = settings.ChangesEnabledAt.Time
	}
	_, err = d.q.EnqueueChangedFromUpdates(ctx, sqlcgen.EnqueueChangedFromUpdatesParams{
		Now:       now,
		AccountID: accountID,
		Since:     since,
	})
	return err
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
		ExpiresAt: earlier(run.facts.ExpiresAt, run.now.Add(PushOtherTTL)),
		Now:       run.now,
		ReportID:  run.facts.ID,
	})
	return err
}

// reporterIsReliable reads the one reporter's history and returns whether the
// feed would tag them Reliable reporter: the same history read, the same votes
// and the same ReporterSignals as the feed.
func (d *PushDispatcher) reporterIsReliable(ctx context.Context, accountID int64, now time.Time) (bool, error) {
	history, err := d.q.ReporterHistoryReports(ctx, reporterHistoryParams([]int64{accountID}, now))
	if err != nil {
		return false, err
	}
	if len(history) == 0 {
		return false, nil
	}
	votes, err := d.q.CurrentVotesForReports(ctx, voteReadIDs(nil, history))
	if err != nil {
		return false, err
	}
	signals := ReporterSignals([]int64{accountID}, history, votes, now)
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
	if err := d.q.MarkAreaLookupTried(ctx, sqlcgen.MarkAreaLookupTriedParams{ReportID: run.facts.ID, Now: run.now}); err != nil {
		log.Printf("push dispatch: marking the area lookup of report %d: %v", run.facts.ID, err)
		return
	}
	callCtx, cancel := context.WithTimeout(ctx, PushAreaLookupTimeout)
	name, err := d.area.ReverseArea(callCtx, run.facts.Lat, run.facts.Lon)
	cancel()
	if err != nil {
		log.Printf("push dispatch: no area name for report %d", run.facts.ID)
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if err := d.q.SetReportAreaName(ctx, sqlcgen.SetReportAreaNameParams{AreaName: name, ReportID: run.facts.ID}); err != nil {
		log.Printf("push dispatch: storing the area name of report %d: %v", run.facts.ID, err)
		return
	}
	run.facts.AreaName = name
}
