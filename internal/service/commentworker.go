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

// CommentWorkerQuerier is the part of the generated queries the comment worker
// uses.
type CommentWorkerQuerier interface {
	ClaimDueCommentChecks(ctx context.Context, arg sqlcgen.ClaimDueCommentChecksParams) ([]sqlcgen.ClaimDueCommentChecksRow, error)
	FinishCommentCheck(ctx context.Context, arg sqlcgen.FinishCommentCheckParams) (int64, error)
	RescheduleCommentCheck(ctx context.Context, arg sqlcgen.RescheduleCommentCheckParams) (int64, error)
}

var _ CommentWorkerQuerier = (*sqlcgen.Queries)(nil)

// CommentWorker retries, in the background, the text checks that could not
// finish when a comment was posted (a comment held as pending, D-12). Until a
// check settles only the comment's author sees it. Comments have no critical
// exemption, so unlike a report a held comment stays off the page for everyone
// else however long the check takes.
//
// It is a sibling of ModerationWorker, not a user of it, because
// moderation_checks is keyed and locked by report and three race tests pin that
// shape (research section 5). A comment carries its own check columns, so this
// worker claims, leases and settles the comment row itself. It reuses the
// constants (30 s sweep, 60 s lease, batch 5, the 8 s call bound), the error
// class words and ModerationBackoff by calling them, and it classifies provider
// errors the same three ways.
//
// Per sweep:
//  1. SweepOnce claims at most ModerationBatchLimit due held comments with a
//     ModerationLeaseSeconds lease in one statement, so two workers sweeping at
//     once screen each comment exactly once. A comment its author deleted while
//     it was held (its words are blanked) and a comment on an expired or removed
//     report are never claimed.
//  2. For each row, in claim order, the trimmed body alone goes to the screener
//     with a ModerationWorkerCallTimeout deadline. One call per row per sweep, no
//     retry loop and no sleep: waiting happens only through next_attempt_at.
//  3. A finished answer goes through Decide, the policy the inline check uses.
//     Pass releases the comment as clear. Borderline releases it as clear and
//     marks it for the moderator list (D-16). Reject removes it; nobody but its
//     author ever saw it. FinishCommentCheck writes the outcome in one statement
//     that changes the row only while it is still pending, so a moderator's Keep
//     or Remove that got there first stands and this worker's late answer writes
//     nothing (08-13).
//  4. NotifyCommentVisible runs only when the comment became clear by this
//     worker's own write (exactly one row changed). Zero rows means a moderator's
//     Keep or another finish got there first and has already told the recipient.
//  5. An unfinished call is classified. A quota or permanent provider error is
//     never retried: the comment is rescheduled to its report's expires_at, which
//     the claim never reaches, so it stays pending and shows in the moderator
//     list as "Check not finished". Anything else (429, 5xx, timeout, the
//     client's local budget refusal, an error that is not a CallError) backs off
//     through ModerationBackoff. A parent context cancelled during the call
//     writes nothing and the lease expires.
//
// There is no attempt count cap for transient errors, for the same reason as the
// report worker: the report's expiry, enforced by the claim, is the bound.
//
// Log lines carry only the numeric comment id, the attempt number, the verdict
// or class word, the status number and the wait, never comment text (T-08-62).
type CommentWorker struct {
	q        CommentWorkerQuerier
	screener TextScreener
	notifier CommentVisibleNotifier
	now      func() time.Time
	jitter   func() float64
	interval time.Duration
}

// CommentWorkerOption adjusts a CommentWorker at construction. WorkerOption
// cannot be reused because it is a function of the report worker's type.
type CommentWorkerOption func(*CommentWorker)

// WithCommentWorkerClock replaces the clock. It decides which held comments are
// due and when the next attempt is.
func WithCommentWorkerClock(now func() time.Time) CommentWorkerOption {
	return func(w *CommentWorker) {
		if now != nil {
			w.now = now
		}
	}
}

// WithCommentWorkerJitter replaces the jitter source, which returns a value in
// [0, 1). The worker's multiplying factor is 0.5 plus that value.
func WithCommentWorkerJitter(f func() float64) CommentWorkerOption {
	return func(w *CommentWorker) {
		if f != nil {
			w.jitter = f
		}
	}
}

// WithCommentSweepInterval replaces ModerationSweepInterval.
func WithCommentSweepInterval(d time.Duration) CommentWorkerOption {
	return func(w *CommentWorker) {
		if d > 0 {
			w.interval = d
		}
	}
}

// NewCommentWorker builds a worker. The notifier may be nil (nobody is told).
// A nil option leaves the default.
func NewCommentWorker(q CommentWorkerQuerier, s TextScreener, n CommentVisibleNotifier, opts ...CommentWorkerOption) *CommentWorker {
	w := &CommentWorker{
		q:        q,
		screener: s,
		notifier: n,
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

// Run sweeps once at once, then every interval, until ctx ends.
func (w *CommentWorker) Run(ctx context.Context) {
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

func (w *CommentWorker) sweepAndLog(ctx context.Context) {
	if err := w.SweepOnce(ctx); err != nil && ctx.Err() == nil {
		log.Printf("comment worker: sweep: %v", err)
	}
}

// SweepOnce claims the due held comments and screens each one once. Per row
// write errors are collected and returned joined; a claim error is returned at
// once.
func (w *CommentWorker) SweepOnce(ctx context.Context) error {
	now := w.now()
	rows, err := w.q.ClaimDueCommentChecks(ctx, sqlcgen.ClaimDueCommentChecksParams{
		Now:        now,
		LeaseUntil: now.Add(ModerationLeaseSeconds * time.Second),
		BatchLimit: ModerationBatchLimit,
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

func (w *CommentWorker) screenRow(ctx context.Context, row sqlcgen.ClaimDueCommentChecksRow) error {
	callCtx, cancel := context.WithTimeout(ctx, ModerationWorkerCallTimeout)
	result, err := w.screener.Screen(callCtx, strings.TrimSpace(row.Body))
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			// Shutdown: write nothing, the lease expires.
			return nil
		}
		return w.reschedule(ctx, row, err)
	}

	// Comments are never "serious" the way a severe report is, so the plain
	// thresholds apply (D-12, D-16).
	decision := Decide(result.Scores, false)
	state := "clear"
	needsReview := false
	switch decision.Verdict {
	case VerdictBorderline:
		needsReview = true
	case VerdictReject:
		state = "removed"
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

	n, err := w.q.FinishCommentCheck(ctx, sqlcgen.FinishCommentCheckParams{
		ModerationState: state,
		NeedsReview:     needsReview,
		Verdict:         string(decision.Verdict),
		Scores:          scoresJSON,
		Model:           model,
		CheckedAt:       w.now(),
		CommentID:       row.ID,
	})
	if err != nil {
		log.Printf("comment worker: comment %d attempt %d: verdict write failed", row.ID, row.CheckAttempts)
		return err
	}
	if n == 0 {
		// A moderator's Keep or Remove, or the author's delete, got there first.
		log.Printf("comment worker: comment %d attempt %d verdict %s, already settled", row.ID, row.CheckAttempts, decision.Verdict)
		return nil
	}
	log.Printf("comment worker: comment %d attempt %d verdict %s", row.ID, row.CheckAttempts, decision.Verdict)

	if state == "clear" && w.notifier != nil {
		if err := w.notifier.NotifyCommentVisible(ctx, row.ID); err != nil {
			// The comment is already visible; the failure costs only a bell row.
			log.Printf("comment worker: comment %d: notify failed", row.ID)
		}
	}
	return nil
}

// reschedule records an unfinished call. Quota and permanent errors move the
// next attempt to the report's expiry, which the claim never reaches.
func (w *CommentWorker) reschedule(ctx context.Context, row sqlcgen.ClaimDueCommentChecksRow, callErr error) error {
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
		wait := ModerationBackoff(int(row.CheckAttempts), retryAfter, 0.5+w.jitter())
		next = w.now().Add(wait)
		log.Printf("comment worker: comment %d attempt %d %s status %d, next try in %s", row.ID, row.CheckAttempts, class, status, wait)
	} else {
		next = row.ExpiresAt
		log.Printf("comment worker: ERROR comment %d attempt %d %s status %d, not retried", row.ID, row.CheckAttempts, class, status)
	}
	_, err := w.q.RescheduleCommentCheck(ctx, sqlcgen.RescheduleCommentCheckParams{
		NextAttemptAt: next,
		LastError:     class,
		CommentID:     row.ID,
	})
	return err
}
