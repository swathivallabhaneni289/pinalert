package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
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
