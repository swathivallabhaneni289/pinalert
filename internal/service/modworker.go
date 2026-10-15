package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	"pinalert/internal/moderation"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// ModerationSweepInterval is the pause between background sweeps. One more
	// sweep runs at process start because the free host cold starts.
	ModerationSweepInterval = 30 * time.Second
	// ModerationLeaseSeconds is how long a claimed check stays reserved for
	// the claimant. Five rows at ModerationWorkerCallTimeout each (40 s) fit
	// inside it.
	ModerationLeaseSeconds = 60
	// ModerationBatchLimit is the most checks one sweep claims.
	ModerationBatchLimit = 5
	// ModerationBackoffBase is the wait before the first retry, before the
	// exponent and the jitter factor.
	ModerationBackoffBase = 30 * time.Second
	// ModerationBackoffCap is the longest base wait.
	ModerationBackoffCap = time.Hour
	// ModerationBackoffMaxShift clamps the exponent before shifting, so a week
	// long report (about 170 attempts) cannot overflow a time.Duration.
	ModerationBackoffMaxShift = 7
	// ModerationWorkerCallTimeout bounds one provider call.
	ModerationWorkerCallTimeout = 8 * time.Second

	// The three words checks.last_error can hold. Never report text.
	CheckErrorTransient = "transient"
	CheckErrorQuota     = "quota"
	CheckErrorPermanent = "permanent"
)

// WorkerQuerier is the part of the generated queries the worker uses.
type WorkerQuerier interface {
	ClaimDueModerationChecks(ctx context.Context, arg sqlcgen.ClaimDueModerationChecksParams) ([]sqlcgen.ClaimDueModerationChecksRow, error)
	SetReportModerationState(ctx context.Context, arg sqlcgen.SetReportModerationStateParams) error
	FinishModerationCheck(ctx context.Context, arg sqlcgen.FinishModerationCheckParams) error
	RescheduleModerationCheck(ctx context.Context, arg sqlcgen.RescheduleModerationCheckParams) error
}

var _ WorkerQuerier = (*sqlcgen.Queries)(nil)

// ModerationWorker retries, in the background, the text checks that could not
// finish when a report was submitted (a pending row in moderation_checks).
//
// Per sweep (D-01 background retry, D-02, D-04, R-03):
//  1. SweepOnce claims at most ModerationBatchLimit due checks with a
//     ModerationLeaseSeconds lease in one statement, so two workers sweeping at
//     once screen each check exactly once. Expired and removed reports are
//     never claimed, because the claim filters on the database clock.
//  2. For each row, in claim order, the trimmed description alone goes to the
//     screener with a ModerationWorkerCallTimeout deadline. One call per row per
//     sweep, no retry loop and no sleep: waiting happens only through the
//     check's next attempt time.
//  3. A finished answer goes through Decide, the policy the inline check uses
//     (D-03: severity never changes the verdict). Pass releases the report as
//     clear. Borderline releases it as clear and marks the check for review
//     (D-04). Reject withdraws it as removed at every severity, a published
//     serious report included (R-03). The report state is written FIRST and
//     the check closed SECOND: a crash between the two leaves a pending check
//     beside a released report, which is re-screened harmlessly, where the
//     other order would leave a done check beside a report stranded in pending
//     that nothing would ever claim again. If the state write fails, the check
//     is not closed, and the lease lapses so a later sweep tries again.
//  4. An unfinished call is classified. A quota or permanent provider error is
//     never retried: the check is rescheduled to the report's expires_at,
//     which the claim never reaches, so the check stays pending and the
//     report stays in the moderator Pending list for Keep or Remove (D-08 c).
//     Anything else (429, 5xx, timeout, the client's local budget refusal, an
//     error that is not a CallError) backs off through ModerationBackoff.
//     A parent context cancelled during the call writes nothing and the lease
//     expires.
//
// Two readings of Claude's Discretion, kept visible so the owner can veto them.
// There is no attempt count cap for transient errors: this departs from
// RESEARCH Area A Landmine 1, which gives the "at most 2 retries" to the
// worker, because the pinned formula min(30 s * 2^attempts, 1 h) has a 1 h cap
// that two retries could never reach, and D-02 needs a held report released
// after an outage of any length. The report's expiry, enforced by the claim,
// is the bound. And quota and permanent errors are not retried (CONTEXT:
// never retry credit or quota errors); with no new query the stop is the
// reschedule to expires_at described above.
//
// A moderator Keep that races an in flight provider call cannot be undone by a
// late reject: KeepReport closes the check in the same statement, and
// SetReportModerationState, FinishModerationCheck and RescheduleModerationCheck
// each write only while the check is still pending. The worker issues no
// UPDATE of its own, and removed stays terminal through the same guard.
//
// Log lines carry only the numeric report id, the attempt number, the verdict
// or class word, the status number and the wait (T-04-04).
type ModerationWorker struct {
	q        WorkerQuerier
	screener TextScreener
	now      func() time.Time
	jitter   func() float64
	interval time.Duration
}

// WorkerOption adjusts a ModerationWorker at construction.
type WorkerOption func(*ModerationWorker)

// WithWorkerClock replaces the clock used to compute the next attempt time.
// The database clock still decides which checks are due.
func WithWorkerClock(now func() time.Time) WorkerOption {
	return func(w *ModerationWorker) {
		if now != nil {
			w.now = now
		}
	}
}

// WithWorkerJitter replaces the jitter source, which returns a value in [0, 1).
// The worker's multiplying factor is 0.5 plus that value.
func WithWorkerJitter(f func() float64) WorkerOption {
	return func(w *ModerationWorker) {
		if f != nil {
			w.jitter = f
		}
	}
}

// WithSweepInterval replaces ModerationSweepInterval.
func WithSweepInterval(d time.Duration) WorkerOption {
	return func(w *ModerationWorker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// NewModerationWorker builds a worker. A nil option leaves the default.
func NewModerationWorker(q WorkerQuerier, s TextScreener, opts ...WorkerOption) *ModerationWorker {
	w := &ModerationWorker{
		q:        q,
		screener: s,
		now:      time.Now,
		jitter:   rand.Float64,
		interval: ModerationSweepInterval,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(w)
		}
	}
	return w
}

// ModerationBackoff is the wait before the next attempt. attempts is the post
// increment value the claim returns, so the first retry (attempts 1) waits
// about a minute. The base is min(30 s * 2^attempts, 1 h) with the exponent
// clamped at ModerationBackoffMaxShift before shifting; it is multiplied by
// factor, then raised to at least retryAfter. factor is expected in 0.5 to
// 1.5 (the worker passes 0.5 plus its jitter source, never the raw source).
func ModerationBackoff(attempts int, retryAfter time.Duration, factor float64) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > ModerationBackoffMaxShift {
		attempts = ModerationBackoffMaxShift
	}
	base := ModerationBackoffBase << uint(attempts)
	if base > ModerationBackoffCap {
		base = ModerationBackoffCap
	}
	wait := time.Duration(float64(base) * factor)
	if retryAfter > wait {
		wait = retryAfter
	}
	return wait
}

// Run sweeps once at once, then every interval, until ctx ends.
func (w *ModerationWorker) Run(ctx context.Context) {
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

func (w *ModerationWorker) sweepAndLog(ctx context.Context) {
	if err := w.SweepOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("moderation worker: sweep: %v", err)
	}
}

// SweepOnce claims the due checks and screens each one once. Per row write
// errors are collected and returned joined; a claim error is returned at once.
func (w *ModerationWorker) SweepOnce(ctx context.Context) error {
	rows, err := w.q.ClaimDueModerationChecks(ctx, sqlcgen.ClaimDueModerationChecksParams{
		LeaseSeconds: ModerationLeaseSeconds,
		BatchLimit:   ModerationBatchLimit,
	})
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		if ctx.Err() != nil {
			break
		}
		if err := w.screenRow(ctx, row); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (w *ModerationWorker) screenRow(ctx context.Context, row sqlcgen.ClaimDueModerationChecksRow) error {
	meta := ReportMeta{
		Severity:   Severity(row.Severity),
		Category:   Category(row.Category),
		Moderation: ModerationState(row.ModerationState),
	}

	callCtx, cancel := context.WithTimeout(ctx, ModerationWorkerCallTimeout)
	result, err := w.screener.Screen(callCtx, strings.TrimSpace(row.Description))
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			// Shutdown: write nothing, the lease expires.
			return nil
		}
		return w.reschedule(ctx, row, err)
	}

	decision := Decide(result.Scores, meta.Serious())
	state := ModerationClear
	needsReview := false
	switch decision.Verdict {
	case VerdictBorderline:
		needsReview = true
	case VerdictReject:
		state = ModerationRemoved
	}

	scores := result.Scores
	if scores == nil {
		scores = map[string]float64{}
	}
	scoresJSON, err := json.Marshal(scores)
	if err != nil {
		return err
	}
	var model *string
	if result.Model != "" {
		m := result.Model
		model = &m
	}

	if err := w.q.SetReportModerationState(ctx, sqlcgen.SetReportModerationStateParams{
		ModerationState: string(state),
		ReportID:        row.ReportID,
	}); err != nil {
		log.Printf("moderation worker: report %d attempt %d: state write failed", row.ReportID, row.Attempts)
		return err
	}
	if err := w.q.FinishModerationCheck(ctx, sqlcgen.FinishModerationCheckParams{
		Verdict:     string(decision.Verdict),
		Scores:      scoresJSON,
		Model:       model,
		NeedsReview: needsReview,
		ReportID:    row.ReportID,
	}); err != nil {
		log.Printf("moderation worker: report %d attempt %d: check close failed", row.ReportID, row.Attempts)
		return err
	}
	log.Printf("moderation worker: report %d attempt %d verdict %s", row.ReportID, row.Attempts, decision.Verdict)
	return nil
}

// reschedule records an unfinished call. Quota and permanent errors move the
// next attempt to the report's expiry, which the claim never reaches.
func (w *ModerationWorker) reschedule(ctx context.Context, row sqlcgen.ClaimDueModerationChecksRow, callErr error) error {
	class := CheckErrorTransient
	status := 0
	var retryAfter time.Duration
	var ce *moderation.CallError
	if errors.As(callErr, &ce) {
		status = ce.Status
		retryAfter = ce.RetryAfter
		switch {
		case ce.Quota:
			class = CheckErrorQuota
		case !ce.Transient:
			class = CheckErrorPermanent
		}
	}

	var next time.Time
	if class == CheckErrorTransient {
		wait := ModerationBackoff(int(row.Attempts), retryAfter, 0.5+w.jitter())
		next = w.now().Add(wait)
		log.Printf("moderation worker: report %d attempt %d %s status %d, next try in %s", row.ReportID, row.Attempts, class, status, wait)
	} else {
		next = row.ExpiresAt
		log.Printf("moderation worker: ERROR report %d attempt %d %s status %d, not retried", row.ReportID, row.Attempts, class, status)
	}
	return w.q.RescheduleModerationCheck(ctx, sqlcgen.RescheduleModerationCheckParams{
		NextAttemptAt: next,
		LastError:     &class,
		ReportID:      row.ReportID,
	})
}
