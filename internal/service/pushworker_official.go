// pushworker_official.go is the official lane of the phone alert worker (plan
// 05-16, owner decisions D-11 and D-16). It sends what plan 05-11 queued in
// official_push_outbox, and it re-checks everything at send time: the alert
// (cancelled, replaced, over, or about to end), that the alert is current, the
// person's switch, their stored home area and its match to the alert, and the
// cap of OfficialPushPerAccountPer6Hours pushes in six hours. A row that fails a
// check is marked skipped with a word from the closed list and nothing more is
// shown.
//
// Delivery is silent on every platform: the service worker asks for no sound and
// no vibration for every kind, and a tap opens /alerts. An iPhone's own settings
// may still decide, which docs/PUSH.md says (plan 05-20). The payload holds only
// words built from closed tables (BuildOfficialPushText); the feed's headline,
// description and instruction are never read here.
//
// The lane shares Phase 3.3's plumbing: the same guarded sender and endpoint
// policy, the same bounded fan out (fanOutSend), the same per device answer table
// (handleDeviceAnswers and judge), the same backoff (PushBackoff) and the same
// lease discipline. Only the holder of a row's current lease sends it: the worker
// renews the lease with a conditional update just before it works a row and stops
// when that fails, and every mark compares the lease it holds. Delivery is at
// least once: a crash between a push service's 201 and the mark sends the row
// again after its lease passes; Android and desktop show the second send as the
// same notification, replaced through its tag.
//
// One alert series is one notification. SACHET sends an Update under a new guid
// that names the earlier message in its references, and the refresher stores it
// as a new row, so the same alert means the alert plus every stored alert it
// names, followed back step by step (OfficialAlertSeries). Topic and Tag come
// from the series root's guid, so an escalation replaces the earlier notification
// on platforms that honour the tag. An Apple endpoint does not replace a note by
// its tag, so it gets only the first official push about a series
// (deviceSkipsRepeatPush, the owner's few notifications rule of 2026-10-28); if
// no device is left the row is skipped as already_notified. The check is per
// account and series by design, as in 3.3: it only ever means fewer notes.
//
// Logs and the sweep's returned error hold numeric ids, a status, the host of a
// push service and a closed class word, and nothing else. The text of a send
// error is never logged, wrapped or joined, because a transport failure's text
// carries the full endpoint, which is a capability URL (3.3 review W3). No guid,
// topic, tag, area name or alert text is logged either.

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/areas"
	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
)

// OfficialPushFreshWindow is how recent proof of a current feed must be: the
// alert row stored or seen by a refresh within this window, or SACHET's last
// success within it. It equals official.FreshWindow, which
// TestOfficialPushFreshWindowMatches pins from the official package (this package
// cannot import it).
const OfficialPushFreshWindow = 30 * time.Minute

// officialFeedSachet is the feed name of the SACHET status row; it equals
// official.FeedSachet, pinned by the database test of the real refresh.
const officialFeedSachet = "sachet"

// The nine words official_push_outbox.skip_reason accepts (migration 00013).
// lease_lost is a log word only; it is never stored.
const (
	officialSkipNotEligible = "not_eligible"
	officialSkipStale       = "stale"
	officialSkipNoHome      = "no_home_area"
	officialSkipOutOfArea   = "out_of_area"
	officialSkipNoDevice    = "no_device"
	officialSkipCap         = "cap_account"
	officialSkipSwitchedOff = "switched_off"
	officialSkipFeedStale   = "feed_not_fresh"
	officialSkipAlreadySent = "already_notified"
)

// officialPushPath is the one address a tap on an official notification opens.
const officialPushPath = "/alerts"

// OfficialPushWorkerQuerier is the part of the generated queries the official
// worker uses.
type OfficialPushWorkerQuerier interface {
	deviceQuerier
	SkipStaleOfficialPushes(ctx context.Context, now time.Time) (int64, error)
	PruneOfficialPushOutbox(ctx context.Context, edge time.Time) error
	ClaimDueOfficialPushes(ctx context.Context, arg sqlcgen.ClaimDueOfficialPushesParams) ([]sqlcgen.ClaimDueOfficialPushesRow, error)
	ExtendOfficialPushLease(ctx context.Context, arg sqlcgen.ExtendOfficialPushLeaseParams) (time.Time, error)
	MarkOfficialPushSent(ctx context.Context, arg sqlcgen.MarkOfficialPushSentParams) (int64, error)
	MarkOfficialPushSkipped(ctx context.Context, arg sqlcgen.MarkOfficialPushSkippedParams) (int64, error)
	RescheduleOfficialPush(ctx context.Context, arg sqlcgen.RescheduleOfficialPushParams) (int64, error)
	MarkOfficialPushFailed(ctx context.Context, arg sqlcgen.MarkOfficialPushFailedParams) (int64, error)
	OfficialAlertForPush(ctx context.Context, id int64) (sqlcgen.OfficialAlertForPushRow, error)
	OfficialAlertSeries(ctx context.Context, alertID int64) ([]sqlcgen.OfficialAlertSeriesRow, error)
	OfficialPushSentForSeries(ctx context.Context, arg sqlcgen.OfficialPushSentForSeriesParams) (bool, error)
	GetFeedStatus(ctx context.Context, feed string) (sqlcgen.OfficialFeedStatus, error)
	GetOfficialPushSetting(ctx context.Context, accountID int64) (bool, error)
	GetAccountHomeCell(ctx context.Context, id int64) (*string, error)
	CountAccountOfficialPushesInWindow(ctx context.Context, arg sqlcgen.CountAccountOfficialPushesInWindowParams) (int64, error)
	PushSubscriptionsForAccount(ctx context.Context, accountID int64) ([]sqlcgen.PushSubscriptionsForAccountRow, error)
}

var _ OfficialPushWorkerQuerier = (*sqlcgen.Queries)(nil)

// OfficialPushWorker sends the official outbox. Plan 05-11's hook queues; this
// reads and decides.
type OfficialPushWorker struct {
	q        OfficialPushWorkerQuerier
	sender   push.Sender
	ix       *areas.Index
	policy   push.EndpointPolicy
	now      func() time.Time
	jitter   func() float64
	interval time.Duration
}

// OfficialPushWorkerOption adjusts an OfficialPushWorker at construction.
type OfficialPushWorkerOption func(*OfficialPushWorker)

// WithOfficialPushWorkerClock replaces the clock the worker decides with. The
// database clock still decides which rows are due and when a lease passes.
func WithOfficialPushWorkerClock(now func() time.Time) OfficialPushWorkerOption {
	return func(w *OfficialPushWorker) {
		if now != nil {
			w.now = now
		}
	}
}

// WithOfficialPushWorkerPolicy replaces the endpoint policy. The server passes the
// one value it also gave the sender; the worker takes a device's host from it.
func WithOfficialPushWorkerPolicy(p push.EndpointPolicy) OfficialPushWorkerOption {
	return func(w *OfficialPushWorker) {
		w.policy = p
	}
}

// WithOfficialPushWorkerInterval replaces PushWorkerInterval.
func WithOfficialPushWorkerInterval(d time.Duration) OfficialPushWorkerOption {
	return func(w *OfficialPushWorker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// NewOfficialPushWorker builds a worker. A nil option leaves the default; a nil
// index leaves every row skipped as not_eligible, because no issuer or area can
// be read without it.
func NewOfficialPushWorker(q OfficialPushWorkerQuerier, sender push.Sender, ix *areas.Index, opts ...OfficialPushWorkerOption) *OfficialPushWorker {
	w := &OfficialPushWorker{
		q:        q,
		sender:   sender,
		ix:       ix,
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

// Run sweeps once at once, then every interval, until ctx ends. The sweep at boot
// matters because the free host cold starts.
func (w *OfficialPushWorker) Run(ctx context.Context) {
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

func (w *OfficialPushWorker) sweepAndLog(ctx context.Context) {
	if err := w.SweepOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("official push worker: sweep: %v", err)
	}
}

// SweepOnce drops the rows that are over, claims the due ones and works each in
// claim order (escalated rows first). The errors it returns are database errors
// only, each tagged with the row id, joined; a claim error is returned at once.
// It stops between rows when ctx is done.
func (w *OfficialPushWorker) SweepOnce(ctx context.Context) error {
	now := w.now().UTC()
	var errs []error
	if _, err := w.q.SkipStaleOfficialPushes(ctx, now); err != nil {
		errs = append(errs, fmt.Errorf("official push worker: skipping stale rows: %w", err))
	}
	if err := w.q.PruneOfficialPushOutbox(ctx, now.Add(-PushOutboxRetention)); err != nil {
		errs = append(errs, fmt.Errorf("official push worker: pruning finished rows: %w", err))
	}
	rows, err := w.q.ClaimDueOfficialPushes(ctx, sqlcgen.ClaimDueOfficialPushesParams{
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
		if err := w.sendOfficialRow(ctx, row); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// officialRowRun is the state of one row while it is worked.
type officialRowRun struct {
	claim sqlcgen.ClaimDueOfficialPushesRow
	lease time.Time
	now   time.Time
	alert sqlcgen.OfficialAlertForPushRow
}

func (w *OfficialPushWorker) rowErr(id int64, err error) error {
	return fmt.Errorf("official push row %d: %w", id, err)
}

// rowLogf is the log function the shared answer handling writes through: each
// line starts with the worker's prefix and the row id.
func (w *OfficialPushWorker) rowLogf(rowID int64) func(format string, args ...any) {
	return func(format string, args ...any) {
		log.Printf("official push worker: row %d "+format, append([]any{rowID}, args...)...)
	}
}

// sendOfficialRow works one claimed row, in this order: renew the lease; read the
// alert again; read its series; prove the alert is current; read the switch; find
// the home area and match it to the alert; apply the 6 hour cap; find the
// devices; apply the one notification per series rule; word it; fan out; settle
// each device; mark the row.
func (w *OfficialPushWorker) sendOfficialRow(ctx context.Context, claim sqlcgen.ClaimDueOfficialPushesRow) error {
	// 1. Only the holder of the current lease may work the row.
	lease, err := w.q.ExtendOfficialPushLease(ctx, sqlcgen.ExtendOfficialPushLeaseParams{
		LeaseSeconds: PushLeaseSeconds,
		ID:           claim.ID,
		HeldLease:    claim.LeaseUntil,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("official push worker: row %d: lease_lost", claim.ID)
			return nil
		}
		return w.rowErr(claim.ID, err)
	}
	run := &officialRowRun{claim: claim, lease: lease, now: w.now().UTC()}

	// 2. The alert, fresh. A cancelled, replaced or finished alert is over; one
	// ending within the margin is not worth a lock screen note.
	alert, err := w.q.OfficialAlertForPush(ctx, claim.AlertID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return w.skip(ctx, run, officialSkipNotEligible)
		}
		return w.rowErr(claim.ID, err)
	}
	run.alert = alert
	if alert.CancelledAt.Valid || alert.SupersededAt.Valid {
		return w.skip(ctx, run, officialSkipStale)
	}
	remaining := alert.ExpiresAt.Sub(run.now)
	if remaining <= OfficialPushExpiryMargin || !run.now.Before(claim.ExpiresAt) {
		return w.skip(ctx, run, officialSkipStale)
	}

	// D-16 once more, with no I/O: Severe or Extreme only, placed on districts or
	// (Extreme only) on a state, from an issuer in the closed table.
	level := strings.ToLower(strings.TrimSpace(alert.Severity))
	if level != "extreme" && level != "severe" {
		return w.skip(ctx, run, officialSkipNotEligible)
	}
	if alert.PlacementTier != "district" && !(alert.PlacementTier == "state" && level == "extreme") {
		return w.skip(ctx, run, officialSkipNotEligible)
	}
	issuer := OfficialIssuer(alert.Sender, w.ix)
	if issuer == "" || w.ix == nil {
		return w.skip(ctx, run, officialSkipNotEligible)
	}

	// 3. The series: the same alert across guids. Topic and Tag follow its root.
	series, err := w.q.OfficialAlertSeries(ctx, alert.ID)
	if err != nil {
		return w.rowErr(claim.ID, err)
	}
	rootGUID, seriesIDs := officialSeriesRoot(alert, series)

	// 4. The alert must be current: stored or seen by a refresh within the window,
	// or SACHET's last success within it. The alert row test comes first, so a row
	// queued by a refresh that is still running after the host slept (its success
	// not yet marked) is sent: a cold start never drops the push it exists to send.
	if run.now.Sub(alert.LastSeenAt) > OfficialPushFreshWindow {
		status, err := w.q.GetFeedStatus(ctx, officialFeedSachet)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return w.rowErr(claim.ID, err)
		}
		if err != nil || !status.LastSuccessAt.Valid || run.now.Sub(status.LastSuccessAt.Time) > OfficialPushFreshWindow {
			return w.skip(ctx, run, officialSkipFeedStale)
		}
	}

	// 5. The switch, read again. An account that is gone has no row.
	on, err := w.q.GetOfficialPushSetting(ctx, claim.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return w.skip(ctx, run, officialSkipNotEligible)
		}
		return w.rowErr(claim.ID, err)
	}
	if !on {
		return w.skip(ctx, run, officialSkipSwitchedOff)
	}

	// 6. The home area and its match to the alert, from the stored cell only.
	home, err := w.q.GetAccountHomeCell(ctx, claim.AccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return w.skip(ctx, run, officialSkipNotEligible)
		}
		return w.rowErr(claim.ID, err)
	}
	if home == nil || *home == "" {
		return w.skip(ctx, run, officialSkipNoHome)
	}
	area, areaIsState, ok := officialArea(alert, w.ix.CellDistricts(*home))
	if !ok {
		return w.skip(ctx, run, officialSkipOutOfArea)
	}

	// 7. The 6 hour cap. These rows live in their own table, so they never count
	// toward 3.3's hourly cap and that cap never counts them.
	sent, err := w.q.CountAccountOfficialPushesInWindow(ctx, sqlcgen.CountAccountOfficialPushesInWindowParams{
		AccountID: claim.AccountID,
		Since:     run.now.Add(-OfficialPushCapWindow),
	})
	if err != nil {
		return w.rowErr(claim.ID, err)
	}
	if sent >= OfficialPushPerAccountPer6Hours {
		return w.skip(ctx, run, officialSkipCap)
	}

	// 8. The devices. The host comes from the policy's reading of the endpoint,
	// never from a stored column, so a wrong stored value cannot let a second push
	// through to an Apple endpoint (3.3 review N6).
	subs, err := w.q.PushSubscriptionsForAccount(ctx, claim.AccountID)
	if err != nil {
		return w.rowErr(claim.ID, err)
	}
	var errs []error
	var devices []pushDevice
	for _, s := range subs {
		host, perr := w.policy.Check(s.Endpoint)
		if perr != nil {
			log.Printf("official push worker: row %d device %d: class %s", claim.ID, s.ID, pushClassPolicy)
			if err := countDeviceFailure(ctx, w.q, run.now, s.ID, "", w.rowLogf(claim.ID)); err != nil {
				errs = append(errs, w.rowErr(claim.ID, err))
			}
			continue
		}
		devices = append(devices, pushDevice{
			id:   s.ID,
			host: host,
			sub:  push.Subscription{Endpoint: s.Endpoint, P256dh: s.P256dh, Auth: s.Auth},
		})
	}
	if len(devices) == 0 {
		errs = append(errs, w.skip(ctx, run, officialSkipNoDevice))
		return errors.Join(errs...)
	}
	// One notification per series on an Apple endpoint: when an earlier official
	// push about this series was sent to the account, an Apple device is left out
	// (deviceSkipsRepeatPush). The check is read only when an Apple device is in
	// the list.
	if anyAppleDevice(devices) {
		already, err := w.q.OfficialPushSentForSeries(ctx, sqlcgen.OfficialPushSentForSeriesParams{
			AccountID: claim.AccountID,
			AlertIds:  seriesIDs,
		})
		if err != nil {
			return w.rowErr(claim.ID, err)
		}
		kept := devices[:0:0]
		for _, d := range devices {
			if deviceSkipsRepeatPush(d.host, already) {
				log.Printf("official push worker: row %d device %d: apple_repeat", claim.ID, d.id)
				continue
			}
			kept = append(kept, d)
		}
		devices = kept
		if len(devices) == 0 {
			errs = append(errs, w.skip(ctx, run, officialSkipAlreadySent))
			return errors.Join(errs...)
		}
	}

	// 9. The words and the message, from closed tables.
	text, ok := BuildOfficialPushText(OfficialPushInput{
		Level:       alert.Severity,
		EventWord:   OfficialEventWord(alert.Event),
		Area:        area,
		AreaIsState: areaIsState,
		Issuer:      issuer,
	})
	if !ok {
		log.Printf("official push worker: row %d: no words for this row", claim.ID)
		errs = append(errs, w.skip(ctx, run, officialSkipNotEligible))
		return errors.Join(errs...)
	}
	msg := officialMessage(rootGUID, level, text, remaining)

	// 10. The fan out, bounded in time and in concurrency, by 3.3's function.
	results := fanOutSend(ctx, w.sender, devices, msg)

	// 11. Everything after the sends is written on a context that outlives a
	// shutdown, so a send a push service accepted is still marked.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pushWriteTimeout)
	defer cancel()
	errs = append(errs, w.settle(wctx, run, results)...)
	return errors.Join(errs...)
}

// officialSeriesRoot reads a series. The root is the member that names no other
// member; with several, the earliest sent_at, then the lowest id; with none (a
// references cycle), the earliest sent_at then the lowest id over all members. An
// empty series is the alert alone. The ids always include the alert itself.
func officialSeriesRoot(alert sqlcgen.OfficialAlertForPushRow, series []sqlcgen.OfficialAlertSeriesRow) (rootGUID string, ids []int64) {
	if len(series) == 0 {
		return alert.Guid, []int64{alert.ID}
	}
	earlier := func(a, b sqlcgen.OfficialAlertSeriesRow) bool {
		if !a.SentAt.Equal(b.SentAt) {
			return a.SentAt.Before(b.SentAt)
		}
		return a.ID < b.ID
	}
	var best, bestAny *sqlcgen.OfficialAlertSeriesRow
	haveSelf := false
	for i := range series {
		r := &series[i]
		ids = append(ids, r.ID)
		if r.ID == alert.ID {
			haveSelf = true
		}
		if bestAny == nil || earlier(*r, *bestAny) {
			bestAny = r
		}
		if !r.NamesMember && (best == nil || earlier(*r, *best)) {
			best = r
		}
	}
	if !haveSelf {
		ids = append(ids, alert.ID)
	}
	if best == nil {
		best = bestAny
	}
	return best.Guid, ids
}

// officialArea picks the words of the place for an alert and a person's home
// districts. A district placed alert matches by district only, so the state rows
// it also carries do not widen it to the state; a state placed Extreme alert
// matches by state; anything else matches nothing. It returns the district name
// followed by " district", or the state words, and whether it is a state.
func officialArea(alert sqlcgen.OfficialAlertForPushRow, home []areas.District) (area string, isState, ok bool) {
	switch alert.PlacementTier {
	case "district":
		for _, d := range home {
			for _, c := range alert.DistrictCodes {
				if int(c) == d.Code {
					return d.Name + " district", false, true
				}
			}
		}
	case "state":
		for _, d := range home {
			for _, c := range alert.StateCodes {
				if int(c) == d.StateCode {
					return OfficialStateWords(d.State), true, true
				}
			}
		}
	}
	return "", false, false
}

// officialMessage is the envelope and the delivery options. The tag and the topic
// follow the series root, so a later version of the alert replaces this one where
// the platform allows it. Urgency is high for Extreme only. The TTL is at most
// OfficialPushTTL and never past the alert's end.
func officialMessage(rootGUID, level string, text PushText, remaining time.Duration) push.Message {
	urgency := push.UrgencyNormal
	if level == "extreme" {
		urgency = push.UrgencyHigh
	}
	ttl := OfficialPushTTL
	if remaining < ttl {
		ttl = remaining
	}
	return push.Message{
		Payload: push.Payload{
			V:     push.PayloadVersion,
			Kind:  string(PushKindOfficial),
			Tag:   OfficialTag(rootGUID),
			Title: text.Title,
			Body:  text.Body,
			URL:   officialPushPath,
		},
		TTL:     ttl,
		Urgency: urgency,
		Topic:   OfficialTopic(rootGUID),
	}
}

// settle handles every device's answer through the shared rules, then marks the
// row. Database errors are collected, each tagged with the row id.
func (w *OfficialPushWorker) settle(ctx context.Context, run *officialRowRun, results []pushDeviceResult) []error {
	var errs []error
	accepted, retry, retryAfter, derrs := handleDeviceAnswers(ctx, w.q, run.now, results, w.rowLogf(run.claim.ID))
	for _, err := range derrs {
		errs = append(errs, w.rowErr(run.claim.ID, err))
	}

	switch {
	case accepted:
		n, err := w.q.MarkOfficialPushSent(ctx, sqlcgen.MarkOfficialPushSentParams{
			Now:       run.now,
			ID:        run.claim.ID,
			HeldLease: run.lease,
		})
		errs = append(errs, w.afterMark(run, n, err))
	case retry && int(run.claim.Attempts) < PushMaxAttempts:
		wait := PushBackoff(int(run.claim.Attempts), retryAfter, 0.5+w.jitter())
		next := run.now.Add(wait)
		if next.Before(pushEarlier(run.claim.ExpiresAt, run.alert.ExpiresAt)) {
			n, err := w.q.RescheduleOfficialPush(ctx, sqlcgen.RescheduleOfficialPushParams{
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

// skip marks the row skipped with a reason from the closed list.
func (w *OfficialPushWorker) skip(ctx context.Context, run *officialRowRun, reason string) error {
	n, err := w.q.MarkOfficialPushSkipped(ctx, sqlcgen.MarkOfficialPushSkippedParams{
		SkipReason: reason,
		ID:         run.claim.ID,
		HeldLease:  run.lease,
	})
	return w.afterMark(run, n, err)
}

// fail marks the row failed: no retry.
func (w *OfficialPushWorker) fail(ctx context.Context, run *officialRowRun) error {
	n, err := w.q.MarkOfficialPushFailed(ctx, sqlcgen.MarkOfficialPushFailedParams{ID: run.claim.ID, HeldLease: run.lease})
	return w.afterMark(run, n, err)
}

// afterMark turns a mark's result into an error or a log line. A mark that
// changed no row means another worker holds the row now; nothing more is written
// for it.
func (w *OfficialPushWorker) afterMark(run *officialRowRun, changed int64, err error) error {
	if err != nil {
		return w.rowErr(run.claim.ID, err)
	}
	if changed == 0 {
		log.Printf("official push worker: row %d: lease_lost", run.claim.ID)
	}
	return nil
}
