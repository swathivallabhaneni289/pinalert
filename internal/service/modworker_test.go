package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// V-05: the background moderation worker. These tests use fakeWorkerQuerier and
// the shared fakeTextScreener (textcheck_test.go); the database tests that
// follow in this file run the same worker through the real statements.

var mwT0 = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

const (
	mwDesc    = "  Water is knee deep on Station Road near the market.  "
	mwTrimmed = "Water is knee deep on Station Road near the market."
)

var (
	mwPassScores       = map[string]float64{"hate": 0.01, "harassment": 0.01, "sexual": 0.01, "violence": 0.02}
	mwBorderlineScores = map[string]float64{"harassment": 0.5}
	mwRejectScores     = map[string]float64{"hate": 0.95}
)

// fakeWorkerQuerier implements service.WorkerQuerier. The claimed rows are
// returned once and then the claim comes back empty.
type fakeWorkerQuerier struct {
	mu sync.Mutex

	rows []sqlcgen.ClaimDueModerationChecksRow

	claimErr, setErr, finishErr, rescheduleErr error

	claimParams []sqlcgen.ClaimDueModerationChecksParams
	sets        []sqlcgen.SetReportModerationStateParams
	finishes    []sqlcgen.FinishModerationCheckParams
	reschedules []sqlcgen.RescheduleModerationCheckParams
	calls       []string

	// claimed, when set, receives one value per claim call (never blocks).
	claimed chan struct{}
}

var _ service.WorkerQuerier = (*fakeWorkerQuerier)(nil)

func (f *fakeWorkerQuerier) ClaimDueModerationChecks(_ context.Context, arg sqlcgen.ClaimDueModerationChecksParams) ([]sqlcgen.ClaimDueModerationChecksRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "claim")
	f.claimParams = append(f.claimParams, arg)
	if f.claimed != nil {
		select {
		case f.claimed <- struct{}{}:
		default:
		}
	}
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	rows := f.rows
	f.rows = nil
	return rows, nil
}

func (f *fakeWorkerQuerier) SetReportModerationState(_ context.Context, arg sqlcgen.SetReportModerationStateParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "set")
	f.sets = append(f.sets, arg)
	return f.setErr
}

func (f *fakeWorkerQuerier) FinishModerationCheck(_ context.Context, arg sqlcgen.FinishModerationCheckParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "finish")
	f.finishes = append(f.finishes, arg)
	return f.finishErr
}

func (f *fakeWorkerQuerier) RescheduleModerationCheck(_ context.Context, arg sqlcgen.RescheduleModerationCheckParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "reschedule")
	f.reschedules = append(f.reschedules, arg)
	return f.rescheduleErr
}

func (f *fakeWorkerQuerier) snapshotCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func mwRow(id int64, attempts int32) sqlcgen.ClaimDueModerationChecksRow {
	return sqlcgen.ClaimDueModerationChecksRow{
		ReportID:        id,
		Attempts:        attempts,
		Description:     mwDesc,
		Severity:        string(service.SeverityLow),
		Category:        string(service.CategoryFlood),
		ModerationState: string(service.ModerationPending),
		ExpiresAt:       mwT0.Add(72 * time.Hour),
	}
}

// mwWorker builds a worker with the fixed clock mwT0 and a jitter source of j
// (factor 0.5 + j).
func mwWorker(q service.WorkerQuerier, s service.TextScreener, j float64, opts ...service.WorkerOption) *service.ModerationWorker {
	all := []service.WorkerOption{
		service.WithWorkerClock(func() time.Time { return mwT0 }),
		service.WithWorkerJitter(func() float64 { return j }),
	}
	return service.NewModerationWorker(q, s, append(all, opts...)...)
}

func TestModerationWorkerBackoff(t *testing.T) {
	const s = time.Second
	table := []struct {
		name       string
		attempts   int
		retryAfter time.Duration
		factor     float64
		want       time.Duration
	}{
		{"first retry", 1, 0, 1.0, 60 * s},
		{"first retry low jitter", 1, 0, 0.5, 30 * s},
		{"first retry high jitter", 1, 0, 1.5, 90 * s},
		{"second", 2, 0, 1.0, 120 * s},
		{"sixth", 6, 0, 1.0, 32 * time.Minute},
		{"seventh hits the cap", 7, 0, 1.0, time.Hour},
		{"week of attempts, no overflow", 170, 0, 1.0, time.Hour},
		{"week of attempts, high jitter", 170, 0, 1.5, 90 * time.Minute},
		{"retry after floor", 1, 120 * s, 0.5, 120 * s},
		{"retry after below computed", 7, 10 * time.Minute, 0.5, 30 * time.Minute},
		{"zero attempts", 0, 0, 1.0, 30 * s},
		{"negative attempts", -3, 0, 1.0, 30 * s},
	}
	for _, c := range table {
		t.Run(c.name, func(t *testing.T) {
			got := service.ModerationBackoff(c.attempts, c.retryAfter, c.factor)
			if got != c.want {
				t.Fatalf("ModerationBackoff(%d, %v, %v) = %v, want %v", c.attempts, c.retryAfter, c.factor, got, c.want)
			}
		})
	}

	// The same ladder and the stops, through a worker on the fake querier.
	row := mwRow(41, 1)
	cases := []struct {
		name      string
		err       error
		jitter    float64
		wantAt    time.Time
		wantClass string
	}{
		{"transient with retry after", &moderation.CallError{Status: 429, Transient: true, RetryAfter: 120 * s}, 0.5, mwT0.Add(120 * s), service.CheckErrorTransient},
		{"transient low jitter", &moderation.CallError{Status: 503, Transient: true}, 0, mwT0.Add(30 * s), service.CheckErrorTransient},
		{"quota stops at expiry", &moderation.CallError{Status: 429, Quota: true}, 0.5, row.ExpiresAt, service.CheckErrorQuota},
		{"permanent stops at expiry", &moderation.CallError{Status: 401}, 0.5, row.ExpiresAt, service.CheckErrorPermanent},
		{"plain error is transient", errors.New("budget refused"), 0.5, mwT0.Add(60 * s), service.CheckErrorTransient},
		{"deadline is transient", context.DeadlineExceeded, 0.5, mwT0.Add(60 * s), service.CheckErrorTransient},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := &fakeWorkerQuerier{rows: []sqlcgen.ClaimDueModerationChecksRow{row}}
			fs := &fakeTextScreener{err: c.err}
			if err := mwWorker(q, fs, c.jitter).SweepOnce(context.Background()); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if len(q.reschedules) != 1 || len(q.sets) != 0 || len(q.finishes) != 0 {
				t.Fatalf("writes: %d reschedules, %d sets, %d finishes", len(q.reschedules), len(q.sets), len(q.finishes))
			}
			got := q.reschedules[0]
			if !got.NextAttemptAt.Equal(c.wantAt) {
				t.Fatalf("next attempt %v, want %v", got.NextAttemptAt, c.wantAt)
			}
			if got.LastError == nil || *got.LastError != c.wantClass {
				t.Fatalf("last error %v, want %q", got.LastError, c.wantClass)
			}
			if got.ReportID != 41 {
				t.Fatalf("report id %d, want 41", got.ReportID)
			}
		})
	}
}

func TestModerationWorkerWriteOrder(t *testing.T) {
	cases := []struct {
		name        string
		scores      map[string]float64
		wantState   service.ModerationState
		wantVerdict string
		wantReview  bool
	}{
		{"pass", mwPassScores, service.ModerationClear, "pass", false},
		{"borderline", mwBorderlineScores, service.ModerationClear, "borderline", true},
		{"reject", mwRejectScores, service.ModerationRemoved, "reject", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := &fakeWorkerQuerier{rows: []sqlcgen.ClaimDueModerationChecksRow{mwRow(7, 1)}}
			fs := screenerWith(c.scores)
			if err := mwWorker(q, fs, 0.5).SweepOnce(context.Background()); err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
			if got, want := q.snapshotCalls(), []string{"claim", "set", "finish"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("calls %v, want %v", got, want)
			}
			if len(q.claimParams) != 1 || q.claimParams[0].LeaseSeconds != service.ModerationLeaseSeconds || q.claimParams[0].BatchLimit != service.ModerationBatchLimit {
				t.Fatalf("claim params %+v", q.claimParams)
			}
			if q.sets[0].ReportID != 7 || service.ModerationState(q.sets[0].ModerationState) != c.wantState {
				t.Fatalf("set %+v, want state %s", q.sets[0], c.wantState)
			}
			fin := q.finishes[0]
			if fin.ReportID != 7 || fin.Verdict != c.wantVerdict || fin.NeedsReview != c.wantReview {
				t.Fatalf("finish %+v", fin)
			}
			var back map[string]float64
			if err := json.Unmarshal(fin.Scores, &back); err != nil || !reflect.DeepEqual(back, c.scores) {
				t.Fatalf("scores %s (%v), want %v", fin.Scores, err, c.scores)
			}
			if fin.Model == nil || *fin.Model != moderation.DefaultModel {
				t.Fatalf("model %v", fin.Model)
			}
			if !reflect.DeepEqual(fs.texts, []string{mwTrimmed}) {
				t.Fatalf("screener got %q, want exactly the trimmed text", fs.texts)
			}
			if !fs.hadDeadline[0] || fs.deadlines[0] > service.ModerationWorkerCallTimeout {
				t.Fatalf("per call deadline missing or too long: %v", fs.deadlines)
			}
		})
	}

	t.Run("set error skips finish", func(t *testing.T) {
		q := &fakeWorkerQuerier{rows: []sqlcgen.ClaimDueModerationChecksRow{mwRow(7, 1)}, setErr: errors.New("db down")}
		err := mwWorker(q, screenerWith(mwPassScores), 0.5).SweepOnce(context.Background())
		if err == nil {
			t.Fatal("SweepOnce returned nil, want the write error")
		}
		if got, want := q.snapshotCalls(), []string{"claim", "set"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("calls %v, want %v", got, want)
		}
	})

	t.Run("claim error is returned", func(t *testing.T) {
		q := &fakeWorkerQuerier{claimErr: errors.New("db down")}
		if err := mwWorker(q, screenerWith(mwPassScores), 0.5).SweepOnce(context.Background()); err == nil {
			t.Fatal("SweepOnce returned nil, want the claim error")
		}
	})

	t.Run("cancelled parent writes nothing", func(t *testing.T) {
		q := &fakeWorkerQuerier{rows: []sqlcgen.ClaimDueModerationChecksRow{mwRow(7, 1)}}
		fs := &fakeTextScreener{block: true}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			_ = mwWorker(q, fs, 0.5).SweepOnce(ctx)
			close(done)
		}()
		deadline := time.Now().Add(2 * time.Second)
		for fs.callCount() == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if fs.callCount() != 1 {
			t.Fatalf("screener calls %d, want 1", fs.callCount())
		}
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("SweepOnce did not return after cancel")
		}
		if got, want := q.snapshotCalls(), []string{"claim"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("calls %v, want only the claim", got)
		}
	})
}

func TestModerationWorkerRunSweepsAtStartAndStops(t *testing.T) {
	run := func(t *testing.T, interval time.Duration) (*fakeWorkerQuerier, context.CancelFunc, chan struct{}) {
		q := &fakeWorkerQuerier{claimed: make(chan struct{}, 100)}
		w := mwWorker(q, screenerWith(mwPassScores), 0.5, service.WithSweepInterval(interval))
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			w.Run(ctx)
			close(done)
		}()
		return q, cancel, done
	}
	stop := func(t *testing.T, cancel context.CancelFunc, done chan struct{}) {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return within 2s of cancel")
		}
	}

	t.Run("one sweep at start, none before the tick", func(t *testing.T) {
		q, cancel, done := run(t, time.Hour)
		select {
		case <-q.claimed:
		case <-time.After(2 * time.Second):
			t.Fatal("no claim at start")
		}
		select {
		case <-q.claimed:
			t.Fatal("second claim before any tick")
		case <-time.After(100 * time.Millisecond):
		}
		stop(t, cancel, done)
	})

	t.Run("ticks sweep again", func(t *testing.T) {
		q, cancel, done := run(t, 20*time.Millisecond)
		for i := 0; i < 2; i++ {
			select {
			case <-q.claimed:
			case <-time.After(2 * time.Second):
				t.Fatalf("claim %d did not arrive", i+1)
			}
		}
		stop(t, cancel, done)
	})
}

// ---- database tests (disposable database p4_0414) ----

// insertPendingReport stores a report in the pending moderation state with a
// pending check that is already due by the database clock, and returns its id.
func insertPendingReport(t *testing.T, pool *pgxpool.Pool, q *sqlcgen.Queries, sev service.Severity, cat service.Category, description string, createdAt, expiresAt time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	const lat, lon = 12.9716, 77.5946
	row, err := q.InsertReportIdempotent(ctx, sqlcgen.InsertReportIdempotentParams{
		SessionID:       "p4-0414-session",
		Category:        string(cat),
		Severity:        string(sev),
		Description:     description,
		Latitude:        lat,
		Longitude:       lon,
		Geohash:         geohash.EncodeWithPrecision(lat, lon, 8),
		CreatedAt:       createdAt,
		ExpiresAt:       expiresAt,
		ModerationState: string(service.ModerationPending),
	})
	if err != nil {
		t.Fatalf("InsertReportIdempotent: %v", err)
	}
	if err := q.InsertModerationCheck(ctx, sqlcgen.InsertModerationCheckParams{
		ReportID:      row.ID,
		Status:        "pending",
		NextAttemptAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("InsertModerationCheck: %v", err)
	}
	return row.ID
}

type mwCheckState struct {
	Status        string
	Attempts      int
	Verdict       *string
	NeedsReview   bool
	Model         *string
	Scores        []byte
	LastError     *string
	LeaseUntil    *time.Time
	NextAttemptAt time.Time
}

func mwReportState(t *testing.T, pool *pgxpool.Pool, id int64) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT moderation_state FROM reports WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatalf("reading report %d: %v", id, err)
	}
	return s
}

func mwCheckOf(t *testing.T, pool *pgxpool.Pool, id int64) mwCheckState {
	t.Helper()
	var c mwCheckState
	err := pool.QueryRow(context.Background(),
		`SELECT status, attempts, verdict, needs_review, model, scores, last_error, lease_until, next_attempt_at
		   FROM moderation_checks WHERE report_id = $1`, id).
		Scan(&c.Status, &c.Attempts, &c.Verdict, &c.NeedsReview, &c.Model, &c.Scores, &c.LastError, &c.LeaseUntil, &c.NextAttemptAt)
	if err != nil {
		t.Fatalf("reading check %d: %v", id, err)
	}
	return c
}

func mwMeta(sev service.Severity, cat service.Category, state string) service.ReportMeta {
	return service.ReportMeta{Severity: sev, Category: cat, Moderation: service.ModerationState(state)}
}

func mwScoresEqual(t *testing.T, raw []byte, want map[string]float64) {
	t.Helper()
	var got map[string]float64
	if err := json.Unmarshal(raw, &got); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("stored scores %s (%v), want %v", raw, err, want)
	}
}

func mwDBWorker(q *sqlcgen.Queries, fs *fakeTextScreener, now time.Time) *service.ModerationWorker {
	return service.NewModerationWorker(q, fs,
		service.WithWorkerClock(func() time.Time { return now }),
		service.WithWorkerJitter(func() float64 { return 0.5 }),
	)
}

func mwSetAnswer(fs *fakeTextScreener, scores map[string]float64, err error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.result = moderation.Result{Model: moderation.DefaultModel, Scores: scores}
	fs.err = err
}

func mwQueueIDs(t *testing.T, q *sqlcgen.Queries) (review, pending []int64) {
	t.Helper()
	ctx := context.Background()
	rv, err := q.ReviewQueue(ctx)
	if err != nil {
		t.Fatalf("ReviewQueue: %v", err)
	}
	for _, r := range rv {
		review = append(review, r.ID)
	}
	pd, err := q.PendingQueue(ctx)
	if err != nil {
		t.Fatalf("PendingQueue: %v", err)
	}
	for _, r := range pd {
		pending = append(pending, r.ID)
	}
	return review, pending
}

func TestModerationWorker(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, expires := now.Add(-time.Minute), now.Add(24*time.Hour)

	t.Run("a_late_pass_releases", func(t *testing.T) {
		id := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, mwDesc, created, expires)
		if !mwMeta(service.SeverityLow, service.CategoryFlood, mwReportState(t, pool, id)).Withheld() {
			t.Fatal("the held report should be withheld before the sweep")
		}
		fs := screenerWith(mwPassScores)
		w := mwDBWorker(q, fs, now)
		if err := w.SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		state := mwReportState(t, pool, id)
		if state != "clear" {
			t.Fatalf("state %q, want clear", state)
		}
		c := mwCheckOf(t, pool, id)
		if c.Status != "done" || c.Verdict == nil || *c.Verdict != "pass" || c.NeedsReview || c.Attempts != 1 || c.LeaseUntil != nil {
			t.Fatalf("check %+v", c)
		}
		if c.Model == nil || *c.Model != moderation.DefaultModel {
			t.Fatalf("model %v", c.Model)
		}
		mwScoresEqual(t, c.Scores, mwPassScores)
		if !reflect.DeepEqual(fs.texts, []string{mwTrimmed}) {
			t.Fatalf("screener got %q, want the trimmed text once", fs.texts)
		}
		if err := w.SweepOnce(ctx); err != nil {
			t.Fatalf("second SweepOnce: %v", err)
		}
		if fs.callCount() != 1 {
			t.Fatalf("screener called %d times after a second sweep, want 1", fs.callCount())
		}
		if mwMeta(service.SeverityLow, service.CategoryFlood, state).Withheld() {
			t.Fatal("the released report is still withheld")
		}
	})

	t.Run("b_unfinished_then_released", func(t *testing.T) {
		id := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, "Road near the bridge is under water", created, expires)
		fs := &fakeTextScreener{err: &moderation.CallError{Status: 503, Transient: true}}
		w := mwDBWorker(q, fs, now)
		if err := w.SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if got := mwReportState(t, pool, id); got != "pending" {
			t.Fatalf("state %q, want pending", got)
		}
		c := mwCheckOf(t, pool, id)
		if c.Status != "pending" || c.Attempts != 1 || c.LeaseUntil != nil {
			t.Fatalf("check %+v", c)
		}
		if c.LastError == nil || *c.LastError != service.CheckErrorTransient {
			t.Fatalf("last error %v", c.LastError)
		}
		if want := now.Add(60 * time.Second); !c.NextAttemptAt.Equal(want) {
			t.Fatalf("next attempt %v, want %v", c.NextAttemptAt, want)
		}
		if err := w.SweepOnce(ctx); err != nil {
			t.Fatalf("immediate SweepOnce: %v", err)
		}
		if fs.callCount() != 1 {
			t.Fatalf("screener called %d times, want 1 (not due yet)", fs.callCount())
		}
		testutil.MustExec(t, pool, `UPDATE moderation_checks SET next_attempt_at = now() - interval '1 second' WHERE report_id = $1`, id)
		mwSetAnswer(fs, mwPassScores, nil)
		if err := w.SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce after re-arm: %v", err)
		}
		if got := mwReportState(t, pool, id); got != "clear" {
			t.Fatalf("state %q, want clear", got)
		}
		if c := mwCheckOf(t, pool, id); c.Status != "done" || c.Attempts != 2 {
			t.Fatalf("check %+v", c)
		}
	})

	t.Run("c_single_claimant", func(t *testing.T) {
		texts := []string{"Shelter open at the school", "Power is out on Park Road", "Boat needed near the temple"}
		var ids []int64
		for _, text := range texts {
			ids = append(ids, insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, text, created, expires))
		}
		fs := screenerWith(mwPassScores)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = mwDBWorker(q, fs, now).SweepOnce(ctx)
			}()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				t.Fatalf("SweepOnce: %v", err)
			}
		}
		got := append([]string(nil), fs.texts...)
		slices.Sort(got)
		want := append([]string(nil), texts...)
		slices.Sort(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("screened %q, want each text exactly once: %q", got, want)
		}
		for _, id := range ids {
			if c := mwCheckOf(t, pool, id); c.Status != "done" || c.Attempts != 1 {
				t.Fatalf("check %d: %+v", id, c)
			}
		}
	})

	t.Run("d_seeded_rows_are_never_claimed", func(t *testing.T) {
		row, err := q.InsertReportIdempotent(ctx, sqlcgen.InsertReportIdempotentParams{
			SessionID:       "p4-0414-seed",
			Category:        string(service.CategoryFlood),
			Severity:        string(service.SeverityLow),
			Description:     "Seeded demo report about the canal",
			Latitude:        12.9716,
			Longitude:       77.5946,
			Geohash:         geohash.EncodeWithPrecision(12.9716, 77.5946, 8),
			CreatedAt:       created,
			ExpiresAt:       expires,
			ModerationState: string(service.ModerationClear),
		})
		if err != nil {
			t.Fatalf("InsertReportIdempotent: %v", err)
		}
		fs := screenerWith(mwPassScores)
		if err := mwDBWorker(q, fs, now).SweepOnce(ctx); err != nil {
			t.Fatalf("SweepOnce: %v", err)
		}
		if fs.callCount() != 0 {
			t.Fatalf("screener called %d times for a seeded row, want 0", fs.callCount())
		}
		if got := mwReportState(t, pool, row.ID); got != "clear" {
			t.Fatalf("state %q, want clear", got)
		}
	})
}

func TestModerationWorkerLateRejectRemovesSerious(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, expires := now.Add(-time.Minute), now.Add(24*time.Hour)

	type kase struct {
		name string
		sev  service.Severity
		cat  service.Category
		text string
		// withheldBefore is what the meta says before the sweep: published
		// serious reports are live through the bypass, the held one is not.
		withheldBefore bool
	}
	cases := []kase{
		{"critical", service.SeverityCritical, service.CategoryFlood, "Water is rising fast near the old bridge", false},
		{"rescue", service.SeverityLow, service.CategoryRescueNeeded, "Family stuck on the roof at the junction", false},
		{"held", service.SeverityLow, service.CategoryFlood, "Road closed near the petrol pump", true},
	}
	ids := make([]int64, len(cases))
	for i, c := range cases {
		ids[i] = insertPendingReport(t, pool, q, c.sev, c.cat, c.text, created, expires)
		if got := mwMeta(c.sev, c.cat, mwReportState(t, pool, ids[i])).Withheld(); got != c.withheldBefore {
			t.Fatalf("%s: Withheld() before the sweep = %v, want %v", c.name, got, c.withheldBefore)
		}
	}

	if err := mwDBWorker(q, screenerWith(mwRejectScores), now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}

	extras, err := q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{ReportIds: ids, ViewerAccountID: 0})
	if err != nil {
		t.Fatalf("ReportExtrasForReports: %v", err)
	}
	byID := map[int64]string{}
	for _, e := range extras {
		byID[e.ReportID] = e.ModerationState
	}
	review, pending := mwQueueIDs(t, q)
	for i, c := range cases {
		state := mwReportState(t, pool, ids[i])
		if state != "removed" {
			t.Fatalf("%s: state %q, want removed", c.name, state)
		}
		ck := mwCheckOf(t, pool, ids[i])
		if ck.Status != "done" || ck.Verdict == nil || *ck.Verdict != "reject" || ck.NeedsReview {
			t.Fatalf("%s: check %+v", c.name, ck)
		}
		if !mwMeta(c.sev, c.cat, state).Withheld() {
			t.Fatalf("%s: Withheld() after the sweep is false", c.name)
		}
		if byID[ids[i]] != "removed" {
			t.Fatalf("%s: ReportExtrasForReports says %q, want removed", c.name, byID[ids[i]])
		}
		if slices.Contains(review, ids[i]) || slices.Contains(pending, ids[i]) {
			t.Fatalf("%s: still listed for a moderator (review %v, pending %v)", c.name, review, pending)
		}
	}
}

func TestModerationWorkerSkipsExpired(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	expired := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood,
		"Expired report about a blocked drain", now.Add(-2*time.Hour), now.Add(-time.Minute))
	sibling := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood,
		"Fresh report about the same drain", now.Add(-time.Minute), now.Add(24*time.Hour))

	fs := screenerWith(mwPassScores)
	if err := mwDBWorker(q, fs, now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if !reflect.DeepEqual(fs.texts, []string{"Fresh report about the same drain"}) {
		t.Fatalf("screened %q, want only the unexpired sibling", fs.texts)
	}
	if got := mwReportState(t, pool, sibling); got != "clear" {
		t.Fatalf("sibling state %q, want clear", got)
	}
	c := mwCheckOf(t, pool, expired)
	if c.Status != "pending" || c.Attempts != 0 || c.LeaseUntil != nil {
		t.Fatalf("expired check %+v, want pending with 0 attempts and no lease", c)
	}
	if got := mwReportState(t, pool, expired); got != "pending" {
		t.Fatalf("expired report state %q, want pending", got)
	}
}

func TestModerationWorkerLateBorderlineNeedsReview(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	created, expires := now.Add(-time.Minute), now.Add(24*time.Hour)

	held := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, "Road closed near the post office", created, expires)
	serious := insertPendingReport(t, pool, q, service.SeverityCritical, service.CategoryFlood, "Water is chest deep near the market", created, expires)

	if err := mwDBWorker(q, screenerWith(mwBorderlineScores), now).SweepOnce(ctx); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	review, pending := mwQueueIDs(t, q)
	for name, id := range map[string]int64{"held": held, "serious": serious} {
		if got := mwReportState(t, pool, id); got != "clear" {
			t.Fatalf("%s: state %q, want clear", name, got)
		}
		c := mwCheckOf(t, pool, id)
		if c.Status != "done" || c.Verdict == nil || *c.Verdict != "borderline" || !c.NeedsReview {
			t.Fatalf("%s: check %+v", name, c)
		}
		mwScoresEqual(t, c.Scores, mwBorderlineScores)
		if !slices.Contains(review, id) {
			t.Fatalf("%s: missing from ReviewQueue %v", name, review)
		}
		if slices.Contains(pending, id) {
			t.Fatalf("%s: still in PendingQueue %v", name, pending)
		}
	}
}
