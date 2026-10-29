package service_test

// commentworker_test.go proves the background retry of held comment checks
// (Phase 8 plan 14, D-12, D-16, PAGE-02): a held comment is rechecked until it
// passes or is refused, only its author sees it meanwhile, the recipient is told
// once, a moderator's decision is never undone by an in flight check, and the
// worker backs off, stops at expiry and logs ids and outcome words only. It
// reuses the cmsRig of comments_test.go and the fakeTextScreener of
// textcheck_test.go.

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// cmwNotifier records the comment ids NotifyCommentVisible was called with.
type cmwNotifier struct {
	mu  sync.Mutex
	ids []int64
}

func (n *cmwNotifier) NotifyCommentVisible(_ context.Context, id int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ids = append(n.ids, id)
	return nil
}

func (n *cmwNotifier) seen() []int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]int64(nil), n.ids...)
}

// cmwHookScreener answers like a provider and runs fn (if set) during the call,
// which is where a test places a moderator's act or an author's delete.
type cmwHookScreener struct {
	mu     sync.Mutex
	result moderation.Result
	err    error
	fn     func()
	texts  []string
}

func (s *cmwHookScreener) Screen(_ context.Context, text string) (moderation.Result, error) {
	s.mu.Lock()
	s.texts = append(s.texts, text)
	fn, res, err := s.fn, s.result, s.err
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
	return res, err
}

func (s *cmwHookScreener) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.texts)
}

func cmwResult(scores map[string]float64) moderation.Result {
	return moderation.Result{Model: moderation.DefaultModel, Scores: scores}
}

// cmwNow is a worker clock a minute after the real time, so every comment the
// seed helpers made due "now" is due, truncated to the microsecond the
// database keeps.
func cmwNow() time.Time {
	return time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
}

// cmwWorker builds a worker with a fixed clock and a jitter source of 0.5, so
// the backoff factor is exactly 1.0.
func cmwWorker(r *cmsRig, s service.TextScreener, n service.CommentVisibleNotifier, at time.Time) *service.CommentWorker {
	return service.NewCommentWorker(r.q, s, n,
		service.WithCommentWorkerClock(func() time.Time { return at }),
		service.WithCommentWorkerJitter(func() float64 { return 0.5 }),
	)
}

type cmwRow struct {
	State       string
	NeedsReview bool
	Attempts    int
	Verdict     *string
	LastError   *string
	Lease       *time.Time
	Next        *time.Time
	Model       *string
	Scores      []byte
	CheckedAt   *time.Time
}

func cmwRowOf(t *testing.T, r *cmsRig, id int64) cmwRow {
	t.Helper()
	var c cmwRow
	err := r.pool.QueryRow(context.Background(),
		`SELECT moderation_state, needs_review, check_attempts, verdict, last_error, lease_until,
		        next_attempt_at, model, scores, checked_at
		   FROM comments WHERE id = $1`, id).
		Scan(&c.State, &c.NeedsReview, &c.Attempts, &c.Verdict, &c.LastError, &c.Lease,
			&c.Next, &c.Model, &c.Scores, &c.CheckedAt)
	if err != nil {
		t.Fatalf("reading comment %d: %v", id, err)
	}
	return c
}

// cmwHeld seeds a held comment by alice on a fresh open report.
func cmwHeld(t *testing.T, r *cmsRig, report int64, body string) int64 {
	t.Helper()
	return testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: report, AccountID: r.alice, Body: body, State: "pending"})
}

// cmwVisibleTo returns the ids of the comments of a report that the viewer sees.
func cmwVisibleTo(t *testing.T, r *cmsRig, report, viewer int64) []int64 {
	t.Helper()
	rows, err := r.q.CommentsForReport(context.Background(), sqlcgen.CommentsForReportParams{
		ViewerAccountID: viewer, ReportID: report, RootCap: 50, ReplyCap: 200,
	})
	if err != nil {
		t.Fatalf("CommentsForReport: %v", err)
	}
	var ids []int64
	for _, row := range rows {
		if row.Gone == "" {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

func TestCommentWorkerReleasesClear(t *testing.T) {
	r := newCmsRig(t)
	logs := captureLog(t)
	ctx := context.Background()
	rep := r.open(t)
	const secret = "  the underpass is dry again  "
	id := cmwHeld(t, r, rep, secret)

	if got := cmwVisibleTo(t, r, rep, r.carol); len(got) != 0 {
		t.Fatalf("a held comment is visible to another reader: %v", got)
	}
	if got := cmwVisibleTo(t, r, rep, r.alice); !slices.Equal(got, []int64{id}) {
		t.Fatalf("the author sees %v, want only the held comment", got)
	}

	fs := screenerWith(mwPassScores)
	note := &cmwNotifier{}
	at := cmwNow()
	if err := cmwWorker(r, fs, note, at).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	row := cmwRowOf(t, r, id)
	if row.State != "clear" || row.NeedsReview {
		t.Fatalf("state %q needs_review %v, want clear and false", row.State, row.NeedsReview)
	}
	if row.Verdict == nil || *row.Verdict != "pass" {
		t.Errorf("verdict %v, want pass", row.Verdict)
	}
	if row.Attempts != 1 || row.Lease != nil || row.Next != nil || row.LastError != nil {
		t.Errorf("attempts %d lease %v next %v last_error %v, want 1 and all cleared", row.Attempts, row.Lease, row.Next, row.LastError)
	}
	if row.Model == nil || *row.Model != moderation.DefaultModel || row.CheckedAt == nil || !row.CheckedAt.Equal(at) {
		t.Errorf("model %v checked_at %v, want the screener model and the worker clock", row.Model, row.CheckedAt)
	}
	mwScoresEqual(t, row.Scores, mwPassScores)
	if fs.callCount() != 1 || fs.texts[0] != strings.TrimSpace(secret) {
		t.Errorf("screener calls %d texts %q, want one call with the trimmed body", fs.callCount(), fs.texts)
	}
	if got := note.seen(); !slices.Equal(got, []int64{id}) {
		t.Errorf("notifier saw %v, want exactly [%d]", got, id)
	}
	if got := cmwVisibleTo(t, r, rep, r.carol); !slices.Equal(got, []int64{id}) {
		t.Errorf("other readers see %v after the release, want the comment", got)
	}
	if strings.Contains(logs.String(), "underpass") {
		t.Errorf("the log holds comment text: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "comment worker: comment ") {
		t.Errorf("no outcome line in the log: %q", logs.String())
	}

	// Nothing is left to claim, so a second sweep screens nothing.
	if err := cmwWorker(r, fs, note, at).SweepOnce(ctx); err != nil {
		t.Fatalf("second SweepOnce: %v", err)
	}
	if fs.callCount() != 1 || len(note.seen()) != 1 {
		t.Errorf("second sweep: calls %d notifies %d, want 1 and 1", fs.callCount(), len(note.seen()))
	}
}

func TestCommentWorkerBorderline(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	id := cmwHeld(t, r, rep, "you people are useless")

	note := &cmwNotifier{}
	if err := cmwWorker(r, screenerWith(mwBorderlineScores), note, cmwNow()).SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	row := cmwRowOf(t, r, id)
	if row.State != "clear" || !row.NeedsReview {
		t.Fatalf("state %q needs_review %v, want clear and true", row.State, row.NeedsReview)
	}
	if row.Verdict == nil || *row.Verdict != "borderline" {
		t.Errorf("verdict %v, want borderline", row.Verdict)
	}
	if got := note.seen(); !slices.Equal(got, []int64{id}) {
		t.Errorf("notifier saw %v, want exactly [%d]", got, id)
	}
	if got := cmwVisibleTo(t, r, rep, r.carol); !slices.Equal(got, []int64{id}) {
		t.Errorf("other readers see %v, want the borderline comment", got)
	}
	// It is listed for moderators.
	items, err := r.q.CommentModerationQueue(context.Background(), 50)
	if err != nil {
		t.Fatalf("CommentModerationQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != id || !items[0].NeedsReview {
		t.Errorf("moderator list %+v, want the one borderline comment", items)
	}
}

func TestCommentWorkerRejectRemoves(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	id := cmwHeld(t, r, rep, "something abusive")

	note := &cmwNotifier{}
	if err := cmwWorker(r, screenerWith(mwRejectScores), note, cmwNow()).SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	row := cmwRowOf(t, r, id)
	if row.State != "removed" || row.NeedsReview {
		t.Fatalf("state %q needs_review %v, want removed and false", row.State, row.NeedsReview)
	}
	if row.Verdict == nil || *row.Verdict != "reject" {
		t.Errorf("verdict %v, want reject", row.Verdict)
	}
	if row.Lease != nil || row.Next != nil {
		t.Errorf("lease %v next %v, want both cleared", row.Lease, row.Next)
	}
	if got := note.seen(); len(got) != 0 {
		t.Errorf("notifier saw %v for a refused comment, want none", got)
	}
	if got := cmwVisibleTo(t, r, rep, r.alice); len(got) != 0 {
		t.Errorf("the author still sees %v, want the comment gone", got)
	}
	if got := cmwVisibleTo(t, r, rep, r.carol); len(got) != 0 {
		t.Errorf("another reader sees %v, want nothing", got)
	}
	if n := cmuCount(t, r, r.reporter, "comment_on_yours", rep, false); n != 0 {
		t.Errorf("the reporter got %d bell rows for a refused comment", n)
	}
}

func TestCommentWorkerBacksOff(t *testing.T) {
	r := newCmsRig(t)
	logs := captureLog(t)
	ctx := context.Background()
	at := cmwNow()
	rep := r.open(t)

	var expires time.Time
	if err := r.pool.QueryRow(ctx, `SELECT expires_at FROM reports WHERE id = $1`, rep).Scan(&expires); err != nil {
		t.Fatalf("reading the report expiry: %v", err)
	}

	cases := []struct {
		name      string
		err       error
		wantClass string
		wantNext  time.Time
	}{
		{"transient", &moderation.CallError{Status: 503, Transient: true}, service.CheckErrorTransient,
			at.Add(service.ModerationBackoff(1, 0, 1.0))},
		{"retry after is a floor", &moderation.CallError{Status: 429, Transient: true, RetryAfter: 10 * time.Minute}, service.CheckErrorTransient,
			at.Add(10 * time.Minute)},
		{"not a call error is transient", errors.New("connection reset"), service.CheckErrorTransient,
			at.Add(service.ModerationBackoff(1, 0, 1.0))},
		{"quota is parked at the expiry", &moderation.CallError{Status: 429, Quota: true}, service.CheckErrorQuota, expires},
		{"permanent is parked at the expiry", &moderation.CallError{Status: 401}, service.CheckErrorPermanent, expires},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := cmwHeld(t, r, rep, "held words that stay private")
			fs := &cmwHookScreener{err: c.err}
			note := &cmwNotifier{}
			if err := cmwWorker(r, fs, note, at).SweepOnce(ctx); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			row := cmwRowOf(t, r, id)
			if row.State != "pending" {
				t.Fatalf("state %q, want it still pending", row.State)
			}
			if row.LastError == nil || *row.LastError != c.wantClass {
				t.Errorf("last_error %v, want %q", row.LastError, c.wantClass)
			}
			if row.Next == nil || !row.Next.Equal(c.wantNext) {
				t.Errorf("next_attempt_at %v, want %v", row.Next, c.wantNext)
			}
			if row.Lease != nil || row.Attempts != 1 || row.Verdict != nil {
				t.Errorf("lease %v attempts %d verdict %v, want no lease, 1 and no verdict", row.Lease, row.Attempts, row.Verdict)
			}
			if len(note.seen()) != 0 {
				t.Errorf("notifier called for a comment that is still held")
			}

			// Not due again at the same clock: a second sweep does not call.
			if err := cmwWorker(r, fs, note, at).SweepOnce(ctx); err != nil {
				t.Fatalf("second SweepOnce: %v", err)
			}
			if fs.calls() != 1 {
				t.Errorf("screener calls %d after a second sweep, want 1", fs.calls())
			}
			// Park it so the next case counts only its own comment.
			if _, err := r.pool.Exec(ctx, `UPDATE comments SET moderation_state = 'removed', next_attempt_at = NULL WHERE id = $1`, id); err != nil {
				t.Fatalf("parking the comment: %v", err)
			}
		})
	}

	// A transient comment comes back once its wait has passed, and a parked
	// one never does.
	id := cmwHeld(t, r, rep, "second chance")
	fs := &cmwHookScreener{err: &moderation.CallError{Status: 503, Transient: true}}
	if err := cmwWorker(r, fs, &cmwNotifier{}, at).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	later := at.Add(2 * time.Minute)
	fs.mu.Lock()
	fs.err = nil
	fs.result = cmwResult(mwPassScores)
	fs.mu.Unlock()
	if err := cmwWorker(r, fs, &cmwNotifier{}, later).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce later: %v", err)
	}
	if row := cmwRowOf(t, r, id); row.State != "clear" || row.Attempts != 2 {
		t.Errorf("after the wait: state %q attempts %d, want clear and 2", row.State, row.Attempts)
	}
	if strings.Contains(logs.String(), "stay private") || strings.Contains(logs.String(), "second chance") {
		t.Errorf("the log holds comment text: %q", logs.String())
	}
}

func TestCommentWorkerStopsAtExpiry(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	at := cmwNow()

	expired := r.report(t, "flood", "low", "clear", -time.Hour)
	removedReport := r.report(t, "flood", "low", "removed", 6*time.Hour)
	endsSoon := r.report(t, "flood", "low", "clear", 6*time.Hour)
	live := r.open(t)

	onExpired := cmwHeld(t, r, expired, "on an expired report")
	onRemoved := cmwHeld(t, r, removedReport, "on a removed report")
	onEndsSoon := cmwHeld(t, r, endsSoon, "on a report that ends before the clock")
	onLive := cmwHeld(t, r, live, "on a live report")

	// A clock past the six hour reports' expiry: only the live one would
	// qualify if it did not also end then, so move the live report out.
	if _, err := r.pool.Exec(ctx, `UPDATE reports SET expires_at = $2 WHERE id = $1`, live, at.Add(24*time.Hour)); err != nil {
		t.Fatalf("extending the live report: %v", err)
	}
	later := at.Add(7 * time.Hour)

	fs := screenerWith(mwPassScores)
	if err := cmwWorker(r, fs, &cmwNotifier{}, later).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	for name, id := range map[string]int64{"expired": onExpired, "removed report": onRemoved, "ended by the clock": onEndsSoon} {
		row := cmwRowOf(t, r, id)
		if row.State != "pending" || row.Attempts != 0 || row.Lease != nil {
			t.Errorf("%s: state %q attempts %d lease %v, want untouched", name, row.State, row.Attempts, row.Lease)
		}
	}
	if row := cmwRowOf(t, r, onLive); row.State != "clear" {
		t.Errorf("the comment on the live report is %q, want clear", row.State)
	}
	if fs.callCount() != 1 {
		t.Errorf("screener calls %d, want only the live report's comment", fs.callCount())
	}
}

func TestCommentWorkerNoDoubleClaim(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)
	const total = 12
	for i := 0; i < total; i++ {
		cmwHeld(t, r, rep, "held comment number "+string(rune('a'+i)))
	}

	fs := screenerWith(mwPassScores)
	note := &cmwNotifier{}
	at := cmwNow()
	for round := 0; round < 6; round++ {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		start := make(chan struct{})
		for w := 0; w < 2; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs <- cmwWorker(r, fs, note, at).SweepOnce(ctx)
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
		}
	}

	seen := map[string]int{}
	fs.mu.Lock()
	for _, text := range fs.texts {
		seen[text]++
	}
	fs.mu.Unlock()
	if len(seen) != total {
		t.Errorf("screened %d distinct comments, want %d", len(seen), total)
	}
	for text, n := range seen {
		if n != 1 {
			t.Errorf("%q was screened %d times, want once", text, n)
		}
	}
	ids := note.seen()
	slices.Sort(ids)
	if len(ids) != total || len(slices.Compact(slices.Clone(ids))) != total {
		t.Errorf("notifier saw %v, want %d distinct ids", ids, total)
	}
	var attempts int
	if err := r.pool.QueryRow(ctx, `SELECT max(check_attempts) FROM comments`).Scan(&attempts); err != nil || attempts != 1 {
		t.Errorf("max check_attempts %d (%v), want 1", attempts, err)
	}
}

func TestCommentWorkerShutdownWritesNothing(t *testing.T) {
	r := newCmsRig(t)
	rep := r.open(t)
	id := cmwHeld(t, r, rep, "in flight when the server stops")
	before := cmwRowOf(t, r, id)

	fs := &fakeTextScreener{block: true}
	note := &cmwNotifier{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for fs.callCount() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()
	if err := cmwWorker(r, fs, note, cmwNow()).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	after := cmwRowOf(t, r, id)
	if after.State != "pending" || after.Verdict != nil || after.LastError != nil {
		t.Errorf("state %q verdict %v last_error %v, want nothing written", after.State, after.Verdict, after.LastError)
	}
	if before.Next == nil || after.Next == nil || !after.Next.Equal(*before.Next) {
		t.Errorf("next_attempt_at %v -> %v, want unchanged", before.Next, after.Next)
	}
	if after.Lease == nil {
		t.Errorf("the claim's lease is gone; it should lapse by itself")
	}
	if len(note.seen()) != 0 {
		t.Errorf("notifier called after a shutdown")
	}
}

func TestCommentWorkerSkipsDeleted(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()
	rep := r.open(t)

	t.Run("deleted while held is never claimed", func(t *testing.T) {
		id := testutil.SeedComment(t, r.pool, testutil.CommentSeed{ReportID: rep, AccountID: r.alice, State: "pending", Deleted: true})
		fs := screenerWith(mwPassScores)
		note := &cmwNotifier{}
		if err := cmwWorker(r, fs, note, cmwNow()).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if fs.callCount() != 0 {
			t.Errorf("screener called %d times (texts %q), want 0", fs.callCount(), fs.texts)
		}
		if row := cmwRowOf(t, r, id); row.State != "pending" || row.Attempts != 0 || row.Lease != nil {
			t.Errorf("state %q attempts %d lease %v, want untouched", row.State, row.Attempts, row.Lease)
		}
		if len(note.seen()) != 0 {
			t.Errorf("notifier called for a deleted comment")
		}
	})

	t.Run("deleted during the call is not released", func(t *testing.T) {
		id := cmwHeld(t, r, rep, "deleted while being checked")
		fs := &cmwHookScreener{result: cmwResult(mwPassScores), fn: func() {
			if err := r.svc.Delete(ctx, id, r.alice); err != nil {
				t.Errorf("Delete: %v", err)
			}
		}}
		note := &cmwNotifier{}
		if err := cmwWorker(r, fs, note, cmwNow()).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if row := cmwRowOf(t, r, id); row.State != "pending" || row.Verdict != nil {
			t.Errorf("state %q verdict %v, want a deleted comment left alone", row.State, row.Verdict)
		}
		if len(note.seen()) != 0 {
			t.Errorf("notifier called for a deleted comment")
		}
	})
}

func TestCommentWorkerNoDoubleNotify(t *testing.T) {
	r := newCmsRig(t)
	ctx := context.Background()

	// keep is what a moderator's Keep does, in the order plan 08-13 runs it:
	// the row lock, the release, then the notification.
	keep := func(id int64) func() {
		return func() {
			if _, err := r.q.LockCommentForModeration(ctx, id); err != nil {
				t.Errorf("LockCommentForModeration: %v", err)
			}
			if n, err := r.q.KeepComment(ctx, id); err != nil || n != 1 {
				t.Errorf("KeepComment: %d rows, %v", n, err)
			}
			if err := r.svc.NotifyCommentVisible(ctx, id); err != nil {
				t.Errorf("NotifyCommentVisible: %v", err)
			}
		}
	}

	t.Run("the worker alone tells the reporter once", func(t *testing.T) {
		rep := r.open(t)
		id := cmwHeld(t, r, rep, "a plain held comment")
		calls := r.note.calls
		if err := cmwWorker(r, screenerWith(mwPassScores), r.svc, cmwNow()).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := cmuCount(t, r, r.reporter, "comment_on_yours", rep, false); got != 1 {
			t.Errorf("bell rows = %d, want 1", got)
		}
		if r.note.calls != calls+1 {
			t.Errorf("change pokes %d -> %d, want one more", calls, r.note.calls)
		}
		if row := cmwRowOf(t, r, id); row.State != "clear" {
			t.Errorf("state %q, want clear", row.State)
		}
	})

	t.Run("a Keep that lands during the check is not announced twice", func(t *testing.T) {
		rep := r.open(t)
		id := cmwHeld(t, r, rep, "kept while being checked")
		fs := &cmwHookScreener{result: cmwResult(mwPassScores), fn: keep(id)}
		calls := r.note.calls
		spy := &cmwNotifier{}
		if err := cmwWorker(r, fs, &cmwBoth{r.svc, spy}, cmwNow()).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := cmuCount(t, r, r.reporter, "comment_on_yours", rep, false); got != 1 {
			t.Errorf("bell rows = %d, want 1", got)
		}
		if r.note.calls != calls+1 {
			t.Errorf("change pokes %d -> %d, want exactly one (the moderator's)", calls, r.note.calls)
		}
		if got := spy.seen(); len(got) != 0 {
			t.Errorf("the worker notified %v after a Keep had already released the comment", got)
		}
		if row := cmwRowOf(t, r, id); row.State != "clear" {
			t.Errorf("state %q, want clear", row.State)
		}
	})

	t.Run("a late refusal cannot undo a Keep", func(t *testing.T) {
		rep := r.open(t)
		id := cmwHeld(t, r, rep, "kept, then refused too late")
		fs := &cmwHookScreener{result: cmwResult(mwRejectScores), fn: keep(id)}
		spy := &cmwNotifier{}
		if err := cmwWorker(r, fs, spy, cmwNow()).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		row := cmwRowOf(t, r, id)
		if row.State != "clear" || row.Verdict != nil {
			t.Errorf("state %q verdict %v, want the moderator's clear left as it was", row.State, row.Verdict)
		}
		if len(spy.seen()) != 0 {
			t.Errorf("the worker notified after losing the race")
		}
	})

	t.Run("a late pass cannot undo a Remove", func(t *testing.T) {
		rep := r.open(t)
		id := cmwHeld(t, r, rep, "removed, then passed too late")
		fs := &cmwHookScreener{result: cmwResult(mwPassScores), fn: func() {
			if _, err := r.q.LockCommentForModeration(ctx, id); err != nil {
				t.Errorf("LockCommentForModeration: %v", err)
			}
			if n, err := r.q.RemoveComment(ctx, id); err != nil || n != 1 {
				t.Errorf("RemoveComment: %d rows, %v", n, err)
			}
		}}
		spy := &cmwNotifier{}
		if err := cmwWorker(r, fs, spy, cmwNow()).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if row := cmwRowOf(t, r, id); row.State != "removed" {
			t.Errorf("state %q, want removed to stay removed", row.State)
		}
		if len(spy.seen()) != 0 {
			t.Errorf("the worker notified after losing the race")
		}
	})
}

// cmwBoth sends a notification to the real service and records it.
type cmwBoth struct {
	real service.CommentVisibleNotifier
	spy  *cmwNotifier
}

func (b *cmwBoth) NotifyCommentVisible(ctx context.Context, id int64) error {
	_ = b.spy.NotifyCommentVisible(ctx, id)
	return b.real.NotifyCommentVisible(ctx, id)
}

// fakeCommentWorkerQuerier counts claims for the Run test.
type fakeCommentWorkerQuerier struct {
	mu     sync.Mutex
	claims int
	got    chan struct{}
}

func (f *fakeCommentWorkerQuerier) ClaimDueCommentChecks(context.Context, sqlcgen.ClaimDueCommentChecksParams) ([]sqlcgen.ClaimDueCommentChecksRow, error) {
	f.mu.Lock()
	f.claims++
	f.mu.Unlock()
	select {
	case f.got <- struct{}{}:
	default:
	}
	return nil, nil
}

func (f *fakeCommentWorkerQuerier) FinishCommentCheck(context.Context, sqlcgen.FinishCommentCheckParams) (int64, error) {
	return 0, nil
}

func (f *fakeCommentWorkerQuerier) RescheduleCommentCheck(context.Context, sqlcgen.RescheduleCommentCheckParams) (int64, error) {
	return 0, nil
}

func TestCommentWorkerRunSweepsAtStartAndStops(t *testing.T) {
	f := &fakeCommentWorkerQuerier{got: make(chan struct{}, 8)}
	w := service.NewCommentWorker(f, screenerWith(mwPassScores), nil, service.WithCommentSweepInterval(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-f.got:
		case <-time.After(2 * time.Second):
			t.Fatalf("claim %d did not happen: want one sweep at start, then one per interval", i+1)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context ended")
	}

	// A context that is already over sweeps nothing.
	f.mu.Lock()
	before := f.claims
	f.mu.Unlock()
	w.Run(ctx)
	f.mu.Lock()
	after := f.claims
	f.mu.Unlock()
	if after != before {
		t.Errorf("Run on a finished context claimed %d times", after-before)
	}
}
