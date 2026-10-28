// pushworker.go sends the notifications the dispatcher queued (plan 03.3-11).
//
// The worker sends what the dispatcher queued, and it reads every report again
// through loadPushFacts and the same predicates, so nothing the feed hides can
// reach a lock screen even if the report changed after the row was queued
// (D-07, TRUST-02). A notification that is already shown is never followed by a
// second push when its report is later removed or retracted (decision 10): the
// row is simply skipped. Words are built now from current facts with
// BuildPushText and are never stored. The platform is asked for no noise by the
// service worker on every kind (decision 4); an iPhone's own settings may still
// decide the sound, which docs/PUSH.md says. No confirmed update goes to an
// Apple endpoint, because iPhone does not replace a notification by its tag and
// a second note would appear (decision 3). For the same reason an Apple endpoint
// gets only the first push about a report (deviceSkipsRepeatPush); later news
// about it stays in the bell. A Critical alert is never held by the per person
// hourly cap, because it is never stored in the bell and a dropped one would
// show nowhere, but it counts toward the cap; a Reliable or change push over the
// cap is dropped with nothing extra shown (decision 6) and stays in the bell
// (heldByHourlyCap).
//
// Delivery is at least once. A crash, or a lost database write, between a push
// service's 201 and the mark sends the row again once its lease passes. Android
// and desktop show the second send as the same notification, replaced through
// its tag; an iPhone may show it twice. Once one device accepted a row, the
// row is sent and its failed devices get no retry for that row. The Topic, TTL
// and Urgency headers reach the push service unencrypted, so it can see report
// ids and urgency; the payload itself stays encrypted (review notes N5 and N7).
//
// Only the holder of a row's current lease sends it. The claim leases a batch;
// the worker renews the lease with a conditional update just before it works a
// row and skips the row when that fails, and every mark compares the lease it
// holds. One row's whole fan out runs under one PushSendTimeout, well inside the
// renewed lease (review W2).
//
// Logs and the sweep's returned error hold numeric ids, a status, the host of a
// push service and a closed class word, and nothing else. The text of a send
// error is never logged, wrapped or joined, because a transport failure's text
// carries the full endpoint, which is a capability URL (review W3, T-03.3-02).

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// PushWorkerInterval is the pause between sweeps. One more sweep runs at
	// process start because the free host cold starts.
	PushWorkerInterval = 5 * time.Second
	// PushClaimBatch is the most rows one sweep claims.
	PushClaimBatch = 50
	// PushLeaseSeconds is how long a claimed or renewed row stays reserved.
	PushLeaseSeconds = 60
	// PushLeaseMargin is the room one row's database work needs inside its
	// renewed lease, besides the fan out itself.
	PushLeaseMargin = 20 * time.Second
	// PushSendConcurrency is the most sends in flight for one row.
	PushSendConcurrency = 8
	// PushSendTimeout bounds one row's whole fan out.
	PushSendTimeout = 10 * time.Second
	// PushMaxAttempts is the most claims of one row. A claim counts as an
	// attempt, so the fourth failed claim fails the row.
	PushMaxAttempts = 4
	// PushBackoffBase is the wait before the first retry, before the factor.
	PushBackoffBase = 15 * time.Second
	// PushBackoffCap is the longest wait.
	PushBackoffCap = 5 * time.Minute
	// PushDeviceFailureLimit is the number of consecutive answers that point at
	// the device (400, 403 and an endpoint that no longer passes the policy)
	// after which a device may be deleted.
	PushDeviceFailureLimit = 5
	// PushDeviceStaleAfter is how old a device's last success must be, or
	// missing, before the failure limit may delete it. A mistake on our side,
	// such as a bad VAPID key answered with 403, therefore cannot wipe devices
	// that worked last week (review W6).
	PushDeviceStaleAfter = 7 * 24 * time.Hour

	// pushWriteTimeout bounds the database writes that follow a fan out, which
	// run on a context that outlives a shutdown so an accepted send is marked.
	pushWriteTimeout = 10 * time.Second
)

// The lease must outlast one row's fan out and its database work, or a second
// claimer could take a row that is still being sent. A negative difference is a
// compile error.
const _ = uint(PushLeaseSeconds*time.Second - PushSendTimeout - PushLeaseMargin)

// The six words push_outbox.skip_reason accepts (migration 00011). lease_lost is
// a log word only; it is never stored.
const (
	pushSkipCap         = "cap_account_hour"
	pushSkipNotEligible = "not_eligible"
	pushSkipNoHome      = "no_home_area"
	pushSkipOutOfRange  = "out_of_range"
	pushSkipNoDevice    = "no_device"
	pushSkipStale       = "stale"
)

// The closed class words sendErrorClass can return.
const (
	pushClassTimeout  = "timeout"
	pushClassCanceled = "canceled"
	pushClassRefused  = "refused_address"
	pushClassPolicy   = "policy"
	pushClassPayload  = "payload"
	pushClassNetwork  = "network"
	pushClassHTTP     = "http"
)

// PushWorkerQuerier is the part of the generated queries the worker uses.
type PushWorkerQuerier interface {
	PushFactsQuerier
	SkipStalePushes(ctx context.Context, now time.Time) (int64, error)
	PrunePushOutbox(ctx context.Context, edge time.Time) error
	ClaimDuePushes(ctx context.Context, arg sqlcgen.ClaimDuePushesParams) ([]sqlcgen.ClaimDuePushesRow, error)
	ExtendPushLease(ctx context.Context, arg sqlcgen.ExtendPushLeaseParams) (time.Time, error)
	MarkPushSent(ctx context.Context, arg sqlcgen.MarkPushSentParams) (int64, error)
	MarkPushSkipped(ctx context.Context, arg sqlcgen.MarkPushSkippedParams) (int64, error)
	ReschedulePush(ctx context.Context, arg sqlcgen.ReschedulePushParams) (int64, error)
	MarkPushFailed(ctx context.Context, arg sqlcgen.MarkPushFailedParams) (int64, error)
	PushAccountHome(ctx context.Context, arg sqlcgen.PushAccountHomeParams) (sqlcgen.PushAccountHomeRow, error)
	CountAccountPushesInWindow(ctx context.Context, arg sqlcgen.CountAccountPushesInWindowParams) (int64, error)
	AccountPushSentAboutReport(ctx context.Context, arg sqlcgen.AccountPushSentAboutReportParams) (bool, error)
	VoteLogForReport(ctx context.Context, reportID int64) ([]sqlcgen.VoteLogForReportRow, error)
	PushSubscriptionsForAccount(ctx context.Context, accountID int64) ([]sqlcgen.PushSubscriptionsForAccountRow, error)
	MarkPushSubscriptionOK(ctx context.Context, arg sqlcgen.MarkPushSubscriptionOKParams) error
	DeletePushSubscriptionByID(ctx context.Context, id int64) error
	BumpPushSubscriptionFailure(ctx context.Context, id int64) (sqlcgen.BumpPushSubscriptionFailureRow, error)
}

var _ PushWorkerQuerier = (*sqlcgen.Queries)(nil)

// PushWorker sends the outbox. It reads and decides; the dispatcher queues.
type PushWorker struct {
	q        PushWorkerQuerier
	sender   push.Sender
	policy   push.EndpointPolicy
	now      func() time.Time
	jitter   func() float64
	interval time.Duration
}

// PushWorkerOption adjusts a PushWorker at construction.
type PushWorkerOption func(*PushWorker)

// WithPushWorkerClock replaces the clock the worker decides with. The database
// clock still decides which rows are due and when a lease passes.
func WithPushWorkerClock(now func() time.Time) PushWorkerOption {
	return func(w *PushWorker) {
		if now != nil {
			w.now = now
		}
	}
}

// WithPushWorkerJitter replaces the jitter source, which returns a value in
// [0, 1). The backoff factor is 0.5 plus that value.
func WithPushWorkerJitter(f func() float64) PushWorkerOption {
	return func(w *PushWorker) {
		if f != nil {
			w.jitter = f
		}
	}
}

// WithPushWorkerInterval replaces PushWorkerInterval.
func WithPushWorkerInterval(d time.Duration) PushWorkerOption {
	return func(w *PushWorker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// WithPushWorkerPolicy replaces the endpoint policy. The server passes the one
// value it also gave the sender; the worker takes a device's host from it.
func WithPushWorkerPolicy(p push.EndpointPolicy) PushWorkerOption {
	return func(w *PushWorker) {
		w.policy = p
	}
}

// NewPushWorker builds a worker. A nil option leaves the default.
func NewPushWorker(q PushWorkerQuerier, sender push.Sender, opts ...PushWorkerOption) *PushWorker {
	w := &PushWorker{
		q:        q,
		sender:   sender,
		policy:   push.DefaultEndpointPolicy(),
		now:      time.Now,
		jitter:   rand.Float64,
		interval: PushWorkerInterval,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(w)
		}
	}
	return w
}

// PushBackoff is the wait before the next attempt. attempts is the post
// increment value the claim returns, so the first retry (attempts 1) waits about
// 15 seconds, then 60 seconds, then 4 minutes, never above PushBackoffCap. The
// base is multiplied by factor (the worker passes 0.5 plus its jitter, so 0.5 to
// 1.5), then limited to the cap, then raised to at least retryAfter: a push
// service's own Retry-After always wins.
func PushBackoff(attempts int, retryAfter time.Duration, factor float64) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	shift := 2 * (attempts - 1)
	if shift > 8 {
		shift = 8
	}
	wait := time.Duration(float64(PushBackoffBase<<uint(shift)) * factor)
	if wait > PushBackoffCap {
		wait = PushBackoffCap
	}
	if retryAfter > wait {
		wait = retryAfter
	}
	return wait
}

// Run sweeps once at once, then every interval, until ctx ends.
func (w *PushWorker) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	w.sweepAndLog(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweepAndLog(ctx)
		}
	}
}

func (w *PushWorker) sweepAndLog(ctx context.Context) {
	if err := w.SweepOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("push worker: sweep: %v", err)
	}
}

// SweepOnce drops the rows that are over, claims the due ones and works each in
// claim order. The errors it returns are database errors only, each tagged with
// the row id, joined; a claim error is returned at once. It stops between rows
// when ctx is done.
func (w *PushWorker) SweepOnce(ctx context.Context) error {
	now := w.now().UTC()
	var errs []error
	if _, err := w.q.SkipStalePushes(ctx, now); err != nil {
		errs = append(errs, fmt.Errorf("push worker: skipping stale rows: %w", err))
	}
	if err := w.q.PrunePushOutbox(ctx, now.Add(-PushOutboxRetention)); err != nil {
		errs = append(errs, fmt.Errorf("push worker: pruning finished rows: %w", err))
	}
	rows, err := w.q.ClaimDuePushes(ctx, sqlcgen.ClaimDuePushesParams{
		BatchLimit:   PushClaimBatch,
		LeaseSeconds: PushLeaseSeconds,
	})
	if err != nil {
		return errors.Join(append(errs, err)...)
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			break
		}
		if err := w.sendRow(ctx, row); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// pushRowRun is the state of one row while it is worked.
type pushRowRun struct {
	claim sqlcgen.ClaimDuePushesRow
	kind  PushKind
	lease time.Time
	now   time.Time
	facts pushReportFacts
	// wasConfirmed is filled in sendRow for a changed row whose event is hidden:
	// whether people nearby had confirmed the report at some point before.
	wasConfirmed bool
}

// reportWasConfirmed says whether people nearby had confirmed a report at some
// point in its vote history. It serves the now disputed phone note (03.3-UAT gap
// for test 8, owner 2026-10-28): the owner wants phone notifications few, so a
// report nobody confirmed that is then disputed stays in the bell, and only a
// report that was confirmed before reaches the lock screen.
//
// history is the report's whole vote log, oldest first (VoteLogForReport). The
// function walks it in order and keeps, for each account and vote kind, the
// latest row seen so far, which is what CurrentVotesForReports would have
// answered at that moment. After every row it builds the tally with
// BuildVoteTally, asks Resolve for the visibility with the report's current
// meta, and returns true as soon as ConfirmedByPeople says the report was
// confirmed. Because it replays the existing rules and holds no confirmation
// rule of its own, push cannot drift from the feed or from the Not confirmed yet
// words (D-07, TRUST-02); it is exact for votes, so disputes that came first give
// no false yes, and a confirming vote later changed by the same person gives no
// false no. Moderation history is not replayed: meta carries the state now, and a
// Withheld report is never pushed anyway. There is no I/O, no clock read and no
// log line; now is passed to Resolve only because its signature takes it.
func reportWasConfirmed(meta ReportMeta, history []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporter *int64, now time.Time) bool {
	type voteKey struct {
		account int64
		kind    string
	}
	// The resolver input is written out field by field, moderation state included,
	// like every other Resolve caller (R-01, TestResolveCallersMentionModeration).
	replayMeta := ReportMeta{Severity: meta.Severity, Category: meta.Category, Moderation: meta.Moderation}
	standing := make(map[voteKey]sqlcgen.CurrentVotesForReportsRow, len(history))
	for _, row := range history {
		if row.ReportID != reportID {
			continue
		}
		standing[voteKey{account: row.AccountID, kind: row.Kind}] = row
		rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(standing))
		for _, r := range standing {
			rows = append(rows, r)
		}
		tally := BuildVoteTally(rows, reportID, reporter)
		vis, _ := Resolve(replayMeta, tally, now)
		if ConfirmedByPeople(vis, FeedViewActive, tally) {
			return true
		}
	}
	return false
}

// pushDevice is one live subscription that passed the policy, with the host the
// policy parsed from its endpoint.
type pushDevice struct {
	id   int64
	host string
	sub  push.Subscription
}

// pushDeviceResult is one device's answer. class is set instead of status when
// the send failed before an answer; the error value itself is dropped at once.
type pushDeviceResult struct {
	dev        pushDevice
	status     int
	retryAfter time.Duration
	class      string
}

func (w *PushWorker) rowErr(id int64, err error) error {
	return fmt.Errorf("push row %d: %w", id, err)
}

// sendRow works one claimed row, in this order: renew the lease; read the report
// again; check the kind's predicate; read the switches and the mute; find the
// home area and distance; apply the hourly cap; find the devices; word it; fan
// out; settle each device; mark the row.
func (w *PushWorker) sendRow(ctx context.Context, claim sqlcgen.ClaimDuePushesRow) error {
	// 1. Only the holder of the current lease may work the row.
	lease, err := w.q.ExtendPushLease(ctx, sqlcgen.ExtendPushLeaseParams{
		LeaseSeconds: PushLeaseSeconds,
		ID:           claim.ID,
		HeldLease:    claim.LeaseUntil,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("push worker: row %d: lease_lost", claim.ID)
			return nil
		}
		return w.rowErr(claim.ID, err)
	}
	run := &pushRowRun{claim: claim, kind: PushKind(claim.Kind), lease: lease, now: w.now().UTC()}

	// 2. The report, fresh, through the one function push code reads reports by.
	facts, ok, err := loadPushFacts(ctx, w.q, claim.ReportID, run.now)
	if err != nil {
		return w.rowErr(claim.ID, err)
	}
	if !ok {
		return w.skip(ctx, run, pushSkipNotEligible)
	}
	run.facts = facts
	remaining := facts.ExpiresAt.Sub(run.now)
	if remaining < time.Second || !run.now.Before(claim.ExpiresAt) {
		return w.skip(ctx, run, pushSkipStale)
	}

	// A now disputed note reaches a phone only for a report people had confirmed
	// before, replayed from the report's own vote log. Only a changed row whose
	// event is hidden reads the log; every other row reads nothing new.
	if run.kind == PushKindChanged && claim.UpdateKind != nil && UpdateKind(*claim.UpdateKind) == UpdateHidden {
		logRows, err := w.q.VoteLogForReport(ctx, claim.ReportID)
		if err != nil {
			return w.rowErr(claim.ID, err)
		}
		history := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(logRows))
		for _, v := range logRows {
			history = append(history, sqlcgen.CurrentVotesForReportsRow(v))
		}
		run.wasConfirmed = reportWasConfirmed(facts.Meta, history, claim.ReportID, facts.ReporterAccountID, run.now)
	}

	// 3. The kind's predicate, the one the feed and the bell use.
	if !w.stillHolds(run) {
		return w.skip(ctx, run, pushSkipNotEligible)
	}

	// 4. The switches and the mute, read again (W5).
	home, err := w.q.PushAccountHome(ctx, sqlcgen.PushAccountHomeParams{ReportID: claim.ReportID, AccountID: claim.AccountID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return w.skip(ctx, run, pushSkipNotEligible)
		}
		return w.rowErr(claim.ID, err)
	}
	if home.Muted || !w.kindSwitchOn(run.kind, claim.FirstKind, facts.Meta.Serious(), home) {
		return w.skip(ctx, run, pushSkipNotEligible)
	}

	// 5. The home area and the distance, measured the way the dispatcher
	// measured it: from the middle of the home cell.
	var distance *float64
	haveHome := false
	if home.HomeAreaCell != nil {
		if hlat, hlon, ok := HomeAreaAnchor(*home.HomeAreaCell); ok {
			haveHome = true
			if d := haversineKm(facts.Lat, facts.Lon, hlat, hlon); !math.IsNaN(d) {
				distance = &d
			}
		}
	}
	if run.kind != PushKindChanged {
		if !haveHome || distance == nil {
			return w.skip(ctx, run, pushSkipNoHome)
		}
		if *distance > float64(home.AlertRadiusKm) {
			return w.skip(ctx, run, pushSkipOutOfRange)
		}
	}

	// 6. The hourly cap; a confirmed update replaces a note already shown and is
	// not counted. A Critical alert is counted but never held (heldByHourlyCap):
	// it is never stored in the bell, so a dropped one would show nowhere.
	if run.kind != PushKindConfirmedUpdate {
		sent, err := w.q.CountAccountPushesInWindow(ctx, sqlcgen.CountAccountPushesInWindowParams{
			AccountID: claim.AccountID,
			Since:     run.now.Add(-time.Hour),
		})
		if err != nil {
			return w.rowErr(claim.ID, err)
		}
		if heldByHourlyCap(run.kind, sent) {
			return w.skip(ctx, run, pushSkipCap)
		}
	}

	// 7. The devices. The host comes from the policy's reading of the endpoint,
	// never from a stored column, so a wrong stored value cannot let a confirmed
	// update through to an Apple endpoint (N6).
	subs, err := w.q.PushSubscriptionsForAccount(ctx, claim.AccountID)
	if err != nil {
		return w.rowErr(claim.ID, err)
	}
	var errs []error
	var devices []pushDevice
	for _, s := range subs {
		host, perr := w.policy.Check(s.Endpoint)
		if perr != nil {
			log.Printf("push worker: row %d device %d: class %s", claim.ID, s.ID, pushClassPolicy)
			if err := w.countFailure(ctx, run, s.ID, ""); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if run.kind == PushKindConfirmedUpdate && push.IsAppleHost(host) {
			continue
		}
		devices = append(devices, pushDevice{
			id:   s.ID,
			host: host,
			sub:  push.Subscription{Endpoint: s.Endpoint, P256dh: s.P256dh, Auth: s.Auth},
		})
	}
	// One notification per report on an Apple endpoint: when an earlier push about
	// this report was sent to the account, an Apple device is left out
	// (deviceSkipsRepeatPush). The check is read only when an Apple device is in
	// the list. A confirmed update already left every Apple device out above.
	if run.kind != PushKindConfirmedUpdate && anyAppleDevice(devices) {
		already, err := w.q.AccountPushSentAboutReport(ctx, sqlcgen.AccountPushSentAboutReportParams{
			AccountID: claim.AccountID,
			ReportID:  claim.ReportID,
		})
		if err != nil {
			return w.rowErr(claim.ID, err)
		}
		kept := devices[:0:0]
		for _, d := range devices {
			if deviceSkipsRepeatPush(d.host, already) {
				log.Printf("push worker: row %d device %d: apple_repeat", claim.ID, d.id)
				continue
			}
			kept = append(kept, d)
		}
		devices = kept
	}
	if len(devices) == 0 {
		errs = append(errs, w.skip(ctx, run, pushSkipNoDevice))
		return errors.Join(errs...)
	}

	// 8. The words and the message, from current facts.
	text, ok := w.buildText(run, distance)
	if !ok {
		log.Printf("push worker: row %d: no words for this row", claim.ID)
		errs = append(errs, w.skip(ctx, run, pushSkipNotEligible))
		return errors.Join(errs...)
	}
	msg := w.buildMessage(run.kind, claim.ReportID, text, remaining)

	// 9. The fan out, bounded in time and in concurrency.
	results := w.fanOut(ctx, devices, msg)

	// 10 and 11. Everything after the sends is written on a context that
	// outlives a shutdown, so a send a push service accepted is still marked.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pushWriteTimeout)
	defer cancel()
	errs = append(errs, w.settle(wctx, run, results, strings.Contains(text.Body, NotConfirmedYetWords))...)
	return errors.Join(errs...)
}

// heldByHourlyCap says whether the hourly limit stops a push of this kind when
// sentInHour pushes were already sent to the person in the rolling hour. It is
// the cap order of the 03.3-UAT gap for test 8 (decided 2026-10-28 at the
// owner's request): a Critical alert near the home area is never held back by
// the hourly limit, because it is a push only kind that is never stored
// (UpdateCriticalNearby), so a dropped one would show nowhere. It still counts
// toward the limit through CountAccountPushesInWindow, so changed and Reliable
// pushes are held back first. A confirmed update is never counted and never
// held, because it replaces a note already on the screen. Every other kind
// (today reliable and changed; Phase 9 adds follow) is held once sentInHour
// reaches PushPerAccountPerHour. A kind counts toward the limit when
// CountAccountPushesInWindow lists it.
func heldByHourlyCap(kind PushKind, sentInHour int64) bool {
	switch kind {
	case PushKindCritical, PushKindConfirmedUpdate:
		return false
	}
	return sentInHour >= PushPerAccountPerHour
}

// deviceSkipsRepeatPush is the per device first push check of the one
// notification per report rule (owner, 2026-10-28). Apple endpoints (iPhone, and
// Safari on a Mac) ignore the tag that lets other browsers replace a shown note,
// so a second push about the same report would stack; such a device gets only
// the first push about a report, and later news about it stays in the bell. It
// reports true when push.IsAppleHost(host) and alreadyPushed (an earlier push
// about the report was sent to this account). host must come from the endpoint
// policy's reading of the endpoint, never from a stored column (review N6). The
// check is per account and report (P-1 of plan 03.3-16), so it only ever means
// fewer notes.
func deviceSkipsRepeatPush(host string, alreadyPushed bool) bool {
	return alreadyPushed && push.IsAppleHost(host)
}

// anyAppleDevice says whether at least one device sits on an Apple endpoint.
func anyAppleDevice(devices []pushDevice) bool {
	for _, d := range devices {
		if push.IsAppleHost(d.host) {
			return true
		}
	}
	return false
}

// stillHolds is the kind's predicate at send time.
func (w *PushWorker) stillHolds(run *pushRowRun) bool {
	f := run.facts
	switch run.kind {
	case PushKindCritical:
		return f.Meta.Serious() && MayNotifyAboutReport(UpdateCriticalNearby, f.Vis, f.ExpiresAt, run.now)
	case PushKindReliable:
		return MayNotifyAboutReport(UpdateReliableNearby, f.Vis, f.ExpiresAt, run.now)
	case PushKindConfirmedUpdate:
		return f.Confirmed && f.View == FeedViewActive
	case PushKindChanged:
		return run.claim.UpdateKind != nil && w.eventStillHolds(UpdateKind(*run.claim.UpdateKind), f, run.now, run.wasConfirmed)
	}
	return false
}

// eventStillHolds says whether a report still shows the event a changed row
// announces (review W4): a row queued for "is now confirmed" must not go out
// once the report became Hidden or Retracted. A Withheld report holds no event.
//
// The confirmation words follow ConfirmedByPeople, the same rule that decides
// Not confirmed yet, and not Live alone: a Critical report stays Live through a
// dispute majority, so Live would let "is now confirmed" reach a lock screen for
// a report the feed does not list as confirmed (repo rule 15).
//
// The fading note goes through MayNotifyAboutReport, the one predicate, so it
// needs a Live report like every kind except the Reliable one (D-07, review
// S-01, 2026-10-25): a Provisional report's fading row stays in the bell and
// never reaches a lock screen. The hidden and resolved notes announce a report
// leaving the feed and cannot use the predicate, whose view check is false for
// them by design.
//
// The now disputed note announces a report leaving the feed, and since
// 2026-10-28 it reaches a phone only when people had confirmed the report before
// (reportWasConfirmed, filled on the run by sendRow and passed in as
// wasConfirmed so this function stays free of I/O). A report nobody confirmed
// that is then disputed stays in the bell, because the owner wants phone
// notifications few.
func (w *PushWorker) eventStillHolds(event UpdateKind, f pushReportFacts, now time.Time, wasConfirmed bool) bool {
	if f.Vis == VisibilityWithheld {
		return false
	}
	switch event {
	case UpdateReachedLive:
		return f.Confirmed
	case UpdateHidden:
		return f.Vis == VisibilityHidden && wasConfirmed
	case UpdateResolved:
		return f.Vis == VisibilityRetracted
	case UpdateFading:
		return MayNotifyAboutReport(UpdateFading, f.Vis, f.ExpiresAt, now)
	}
	return false
}

// confirmedBaseKind is the kind of the alert a confirmed update replaces: the
// first row's kind while that row exists, otherwise Critical for a serious
// report and Reliable for any other.
func confirmedBaseKind(first *string, serious bool) PushKind {
	if first != nil {
		switch k := PushKind(*first); k {
		case PushKindCritical, PushKindReliable:
			return k
		}
	}
	if serious {
		return PushKindCritical
	}
	return PushKindReliable
}

// kindSwitchOn says whether the switch of the row's kind is on now (review W5).
// A confirmed update follows the switch of the initial alert it updates.
func (w *PushWorker) kindSwitchOn(kind PushKind, first *string, serious bool, h sqlcgen.PushAccountHomeRow) bool {
	if kind == PushKindConfirmedUpdate {
		kind = confirmedBaseKind(first, serious)
	}
	switch kind {
	case PushKindCritical:
		return h.CriticalPushOn
	case PushKindReliable:
		return h.ReliableAlertsOn
	case PushKindChanged:
		return h.ChangesPushOn
	}
	return false
}

// buildText words the row from the current facts. A confirmed update is built
// from the original kind with Confirmed set; its base kind comes from the claim.
func (w *PushWorker) buildText(run *pushRowRun, distance *float64) (PushText, bool) {
	f := run.facts
	in := PushTextInput{
		Kind:       run.kind,
		Happening:  f.Happening,
		Category:   f.Meta.Category,
		Severity:   f.Meta.Severity,
		AreaName:   f.AreaName,
		DistanceKm: distance,
		Confirmed:  f.Confirmed,
		OwnReport:  f.ReporterAccountID != nil && *f.ReporterAccountID == run.claim.AccountID,
	}
	switch run.kind {
	case PushKindConfirmedUpdate:
		in.Kind = confirmedBaseKind(run.claim.FirstKind, f.Meta.Serious())
		in.Confirmed = true
	case PushKindChanged:
		if run.claim.UpdateKind != nil {
			in.Event = UpdateKind(*run.claim.UpdateKind)
		}
	}
	return BuildPushText(in)
}

// pushTTL is how long a push service holds the message for an offline device:
// the report's remaining life, up to an hour for Critical and up to 30 minutes
// for every other kind.
func (w *PushWorker) pushTTL(kind PushKind, remaining time.Duration) time.Duration {
	longest := PushOtherTTL
	if kind == PushKindCritical {
		longest = PushCriticalTTLMax
	}
	if remaining > longest {
		return longest
	}
	return remaining
}

// buildMessage is the envelope and the delivery options. The tag makes a later
// push about the same report replace this one; the topic does the same at the
// push service for a device that is offline. Urgency is high for Critical only.
func (w *PushWorker) buildMessage(kind PushKind, reportID int64, text PushText, remaining time.Duration) push.Message {
	urgency := push.UrgencyNormal
	if kind == PushKindCritical {
		urgency = push.UrgencyHigh
	}
	return push.Message{
		Payload: push.Payload{
			V:     push.PayloadVersion,
			Kind:  string(kind),
			Tag:   fmt.Sprintf("report-%d", reportID),
			Title: text.Title,
			Body:  text.Body,
			URL:   fmt.Sprintf("/?report=%d", reportID),
		},
		TTL:     w.pushTTL(kind, remaining),
		Urgency: urgency,
		Topic:   fmt.Sprintf("r%d", reportID),
	}
}

// fanOut sends the message to every device, at most PushSendConcurrency at
// once, all under one PushSendTimeout. Results are collected and no database
// write happens inside a goroutine. A send error is reduced to a class word at
// once and the error value is dropped (W3).
func (w *PushWorker) fanOut(ctx context.Context, devices []pushDevice, msg push.Message) []pushDeviceResult {
	// A shutdown signal must not abort a request already on the wire: the push
	// service may have accepted it, and the row would be sent again after the
	// restart. The send is still bounded by PushSendTimeout, which the shutdown
	// grace covers (skeptic WR-03). Claiming stops on ctx elsewhere.
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), PushSendTimeout)
	defer cancel()

	results := make([]pushDeviceResult, len(devices))
	sem := make(chan struct{}, PushSendConcurrency)
	var wg sync.WaitGroup
	for i, d := range devices {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res := pushDeviceResult{dev: d}
			answer, err := w.sender.Send(sendCtx, d.sub, msg)
			if err != nil {
				res.class = sendErrorClass(err)
			} else {
				res.status = answer.Status
				res.retryAfter = answer.RetryAfter
			}
			results[i] = res
		}()
	}
	wg.Wait()
	return results
}

// sendErrorClass reduces a send error to one closed word. The caller drops the
// error value after this, because a transport failure's text can hold the full
// endpoint (W3).
func sendErrorClass(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return pushClassTimeout
	case errors.Is(err, context.Canceled):
		return pushClassCanceled
	case errors.Is(err, push.ErrRefusedAddress):
		return pushClassRefused
	case errors.Is(err, push.ErrUnsupportedService), errors.Is(err, push.ErrBadEndpoint), errors.Is(err, push.ErrBadKeys):
		return pushClassPolicy
	case errors.Is(err, push.ErrPayloadTooLarge), errors.Is(err, push.ErrNoTTL), errors.Is(err, push.ErrBadTopic), errors.Is(err, push.ErrBadUrgency):
		return pushClassPayload
	case errors.As(err, &netErr) && netErr.Timeout():
		return pushClassTimeout
	}
	return pushClassNetwork
}

// answerVerdict is what one device's answer means for the row.
type answerVerdict int

const (
	verdictAccepted  answerVerdict = iota // the push service took it
	verdictRetry                          // try again later; says nothing about the device
	verdictPermanent                      // do not try this device again for this row
)

// pushJudge reads one answer. bump is true only for answers that point at the
// device; drop is true when the push service says the subscription is gone.
// Trouble on the push service's side or our own (5xx, timeouts, canceled sends,
// network errors, a refused address, our own key being refused) never counts
// against a device (review W6).
func judge(r pushDeviceResult) (v answerVerdict, bump, drop bool) {
	if r.class != "" {
		switch r.class {
		case pushClassPolicy:
			return verdictPermanent, true, false
		case pushClassPayload:
			return verdictPermanent, false, false
		}
		return verdictRetry, false, false
	}
	switch {
	case r.status >= 200 && r.status < 300:
		return verdictAccepted, false, false
	case r.status == 404 || r.status == 410:
		return verdictPermanent, false, true
	case r.status == 429 || r.status >= 500:
		return verdictRetry, false, false
	case r.status == 400 || r.status == 403:
		return verdictPermanent, true, false
	}
	return verdictPermanent, false, false
}

// settle handles every device's answer, then marks the row. Database errors are
// collected, each tagged with the row id.
func (w *PushWorker) settle(ctx context.Context, run *pushRowRun, results []pushDeviceResult, unconfirmed bool) []error {
	var errs []error
	accepted, retry := false, false
	var retryAfter time.Duration
	for _, res := range results {
		verdict, bump, drop := judge(res)
		switch verdict {
		case verdictAccepted:
			accepted = true
			if err := w.q.MarkPushSubscriptionOK(ctx, sqlcgen.MarkPushSubscriptionOKParams{Now: run.now, ID: res.dev.id}); err != nil {
				errs = append(errs, w.rowErr(run.claim.ID, err))
			}
			continue
		case verdictRetry:
			retry = true
			if res.retryAfter > retryAfter {
				retryAfter = res.retryAfter
			}
		}
		word := res.class
		if word == "" {
			word = pushClassHTTP
		}
		log.Printf("push worker: row %d device %d host %s status %d class %s", run.claim.ID, res.dev.id, res.dev.host, res.status, word)
		if drop {
			if err := w.q.DeletePushSubscriptionByID(ctx, res.dev.id); err != nil {
				errs = append(errs, w.rowErr(run.claim.ID, err))
			}
		}
		if bump {
			if err := w.countFailure(ctx, run, res.dev.id, res.dev.host); err != nil {
				errs = append(errs, err)
			}
		}
	}

	switch {
	case accepted:
		n, err := w.q.MarkPushSent(ctx, sqlcgen.MarkPushSentParams{
			Now:             run.now,
			SentUnconfirmed: unconfirmed,
			ID:              run.claim.ID,
			HeldLease:       run.lease,
		})
		errs = append(errs, w.afterMark(run, n, err))
	case retry && int(run.claim.Attempts) < PushMaxAttempts:
		wait := PushBackoff(int(run.claim.Attempts), retryAfter, 0.5+w.jitter())
		next := run.now.Add(wait)
		if next.Before(pushEarlier(run.claim.ExpiresAt, run.facts.ExpiresAt)) {
			n, err := w.q.ReschedulePush(ctx, sqlcgen.ReschedulePushParams{
				NextAttemptAt: next,
				ID:            run.claim.ID,
				HeldLease:     run.lease,
			})
			errs = append(errs, w.afterMark(run, n, err))
			break
		}
		errs = append(errs, w.fail(ctx, run))
	default:
		errs = append(errs, w.fail(ctx, run))
	}

	var out []error
	for _, err := range errs {
		if err != nil {
			out = append(out, err)
		}
	}
	return out
}

// countFailure adds one to a device's failure count and deletes the device only
// when the count reached the limit and its last success is missing or older
// than the stale window (W6). host may be empty when the endpoint did not pass
// the policy.
func (w *PushWorker) countFailure(ctx context.Context, run *pushRowRun, deviceID int64, host string) error {
	r, err := w.q.BumpPushSubscriptionFailure(ctx, deviceID)
	if err != nil {
		return w.rowErr(run.claim.ID, err)
	}
	stale := !r.LastOkAt.Valid || r.LastOkAt.Time.Before(run.now.Add(-PushDeviceStaleAfter))
	if int(r.Failures) >= PushDeviceFailureLimit && stale {
		if err := w.q.DeletePushSubscriptionByID(ctx, deviceID); err != nil {
			return w.rowErr(run.claim.ID, err)
		}
		log.Printf("push worker: row %d device %d host %s: removed after %d failures", run.claim.ID, deviceID, host, r.Failures)
	}
	return nil
}

// skip marks the row skipped with a reason from the closed list.
func (w *PushWorker) skip(ctx context.Context, run *pushRowRun, reason string) error {
	n, err := w.q.MarkPushSkipped(ctx, sqlcgen.MarkPushSkippedParams{
		SkipReason: reason,
		ID:         run.claim.ID,
		HeldLease:  run.lease,
	})
	return w.afterMark(run, n, err)
}

// fail marks the row failed: no retry.
func (w *PushWorker) fail(ctx context.Context, run *pushRowRun) error {
	n, err := w.q.MarkPushFailed(ctx, sqlcgen.MarkPushFailedParams{ID: run.claim.ID, HeldLease: run.lease})
	return w.afterMark(run, n, err)
}

// afterMark turns a mark's result into an error or a log line. A mark that
// changed no row means another worker holds the row now; nothing more is written
// for it.
func (w *PushWorker) afterMark(run *pushRowRun, changed int64, err error) error {
	if err != nil {
		return w.rowErr(run.claim.ID, err)
	}
	if changed == 0 {
		log.Printf("push worker: row %d: lease_lost", run.claim.ID)
	}
	return nil
}
