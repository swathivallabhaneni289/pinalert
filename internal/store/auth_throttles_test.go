package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	throttleKey    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	throttleFree   = 5
	throttleBase   = 300
	throttleCap    = 3600
	throttleForget = 30 * time.Minute
)

func recordFailure(t *testing.T, q *sqlcgen.Queries, key string, now time.Time) sqlcgen.RecordAuthFailureRow {
	t.Helper()
	row, err := q.RecordAuthFailure(context.Background(), sqlcgen.RecordAuthFailureParams{
		Scope:           "sign_in",
		KeyHash:         key,
		Now:             now,
		ForgetCutoff:    now.Add(-throttleForget),
		FreeFailures:    throttleFree,
		BaseLockSeconds: throttleBase,
		MaxLockSeconds:  throttleCap,
	})
	if err != nil {
		t.Fatalf("RecordAuthFailure at %v: %v", now, err)
	}
	return row
}

func wantLock(t *testing.T, row sqlcgen.RecordAuthFailureRow, now time.Time, d time.Duration) {
	t.Helper()
	if !row.LockedUntil.Valid {
		t.Fatalf("failure %d at %v is not locked, want a lock of %v", row.Failures, now, d)
	}
	if got := row.LockedUntil.Time.Sub(now); got != d {
		t.Fatalf("failure %d locks for %v, want %v", row.Failures, got, d)
	}
}

// TestThrottleSchedule pins the wait schedule: 5 free failures, then 5, 10, 20,
// 40 and 60 minutes, and 60 again. Each later failure is made exactly when the
// previous lock ends, so the 40 minute lock must not reset the count (B1).
func TestThrottleSchedule(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	for i := int32(1); i <= throttleFree; i++ {
		row := recordFailure(t, q, throttleKey, now)
		if row.Failures != i || row.LockedUntil.Valid {
			t.Fatalf("failure %d = %+v, want %d failures and no lock", i, row, i)
		}
		now = now.Add(time.Minute)
	}

	waits := []time.Duration{5, 10, 20, 40, 60, 60}
	for i, w := range waits {
		want := int32(throttleFree + 1 + i)
		d := w * time.Minute
		row := recordFailure(t, q, throttleKey, now)
		if row.Failures != want {
			t.Fatalf("failure at %v counted %d, want %d", now, row.Failures, want)
		}
		wantLock(t, row, now, d)
		now = row.LockedUntil.Time
	}

	t.Run("the stored row matches the last answer", func(t *testing.T) {
		got, err := q.GetAuthThrottle(context.Background(), sqlcgen.GetAuthThrottleParams{Scope: "sign_in", KeyHash: throttleKey})
		if err != nil {
			t.Fatalf("GetAuthThrottle: %v", err)
		}
		if got.Failures != throttleFree+int32(len(waits)) || !got.LockedUntil.Valid {
			t.Fatalf("stored = %+v", got)
		}
	})
	t.Run("another address is counted apart", func(t *testing.T) {
		other := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		row := recordFailure(t, q, other, now)
		if row.Failures != 1 || row.LockedUntil.Valid {
			t.Fatalf("other address = %+v, want 1 failure and no lock", row)
		}
	})
}

// TestThrottleForgetsAfterQuietWindow proves the count restarts only when both
// the last failure and the end of the last lock are more than 30 minutes old.
func TestThrottleForgetsAfterQuietWindow(t *testing.T) {
	base := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	t.Run("no lock: a failure after the quiet window restarts at 1", func(t *testing.T) {
		q := sqlcgen.New(testutil.NewTestDB(t))
		for i := 0; i < 3; i++ {
			recordFailure(t, q, throttleKey, base.Add(time.Duration(i)*time.Minute))
		}
		row := recordFailure(t, q, throttleKey, base.Add(2*time.Minute+throttleForget+time.Second))
		if row.Failures != 1 || row.LockedUntil.Valid {
			t.Fatalf("after the quiet window = %+v, want 1 and no lock", row)
		}
	})
	t.Run("no lock: a failure inside the window keeps counting", func(t *testing.T) {
		q := sqlcgen.New(testutil.NewTestDB(t))
		for i := 0; i < 3; i++ {
			recordFailure(t, q, throttleKey, base.Add(time.Duration(i)*time.Minute))
		}
		row := recordFailure(t, q, throttleKey, base.Add(2*time.Minute+throttleForget-time.Second))
		if row.Failures != 4 {
			t.Fatalf("inside the window = %+v, want 4", row)
		}
	})
	t.Run("a lock: the window counts from the end of the lock", func(t *testing.T) {
		q := sqlcgen.New(testutil.NewTestDB(t))
		now := base
		var last sqlcgen.RecordAuthFailureRow
		for i := 0; i < throttleFree+1; i++ {
			last = recordFailure(t, q, throttleKey, now)
			now = now.Add(time.Minute)
		}
		if !last.LockedUntil.Valid {
			t.Fatal("the sixth failure must lock")
		}
		end := last.LockedUntil.Time

		inside := recordFailure(t, q, throttleKey, end.Add(throttleForget-time.Second))
		if inside.Failures != throttleFree+2 {
			t.Fatalf("inside the window after a lock = %+v, want %d", inside, throttleFree+2)
		}
		wantLock(t, inside, end.Add(throttleForget-time.Second), 10*time.Minute)

		// Quiet for more than 30 minutes after the lock that just began.
		after := inside.LockedUntil.Time.Add(throttleForget + time.Second)
		reset := recordFailure(t, q, throttleKey, after)
		if reset.Failures != 1 || reset.LockedUntil.Valid {
			t.Fatalf("after the quiet window after a lock = %+v, want 1 and no lock", reset)
		}
	})
}

// TestThrottleClear proves ClearAuthThrottle removes the row.
func TestThrottleClear(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	recordFailure(t, q, throttleKey, time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC))

	if err := q.ClearAuthThrottle(ctx, sqlcgen.ClearAuthThrottleParams{Scope: "sign_in", KeyHash: throttleKey}); err != nil {
		t.Fatalf("ClearAuthThrottle: %v", err)
	}
	_, err := q.GetAuthThrottle(ctx, sqlcgen.GetAuthThrottleParams{Scope: "sign_in", KeyHash: throttleKey})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
}

// TestPruneAuthThrottles proves a row goes only when its last failure and the
// end of its lock are both at or before the cutoff.
func TestPruneAuthThrottles(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	cutoff := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	keys := map[string]string{
		"stale":        "1111111111111111111111111111111111111111111111111111111111111111",
		"stale lock":   "2222222222222222222222222222222222222222222222222222222222222222",
		"live lock":    "3333333333333333333333333333333333333333333333333333333333333333",
		"recent":       "4444444444444444444444444444444444444444444444444444444444444444",
		"stale locked": "5555555555555555555555555555555555555555555555555555555555555555",
	}
	const insert = `INSERT INTO auth_throttles (scope, key_hash, failures, last_failure_at, locked_until) VALUES ('sign_in', $1, 6, $2, $3)`
	testutil.MustExec(t, pool, insert, keys["stale"], cutoff.Add(-time.Hour), nil)
	testutil.MustExec(t, pool, insert, keys["stale lock"], cutoff.Add(-2*time.Hour), cutoff.Add(-time.Hour))
	testutil.MustExec(t, pool, insert, keys["live lock"], cutoff.Add(-time.Hour), cutoff.Add(time.Hour))
	testutil.MustExec(t, pool, insert, keys["recent"], cutoff.Add(time.Minute), nil)
	testutil.MustExec(t, pool, insert, keys["stale locked"], cutoff.Add(-time.Hour), cutoff)

	pruned, err := q.PruneAuthThrottles(ctx, cutoff)
	if err != nil || pruned != 3 {
		t.Fatalf("PruneAuthThrottles = %d (%v), want 3", pruned, err)
	}
	for name, key := range keys {
		_, err := q.GetAuthThrottle(ctx, sqlcgen.GetAuthThrottleParams{Scope: "sign_in", KeyHash: key})
		kept := err == nil
		wantKept := name == "live lock" || name == "recent"
		if kept != wantKept {
			t.Errorf("row %q kept = %v, want %v", name, kept, wantKept)
		}
	}
}

// TestClaimAuthEmailSendCooldownAndHour proves one atomic claim enforces the 45
// second cooldown and five sends per hour, and that a refused claim writes
// nothing.
func TestClaimAuthEmailSendCooldownAndHour(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	const email = "send-window@example.com"
	t0 := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	claim := func(now time.Time) (sqlcgen.ClaimAuthEmailSendRow, error) {
		return q.ClaimAuthEmailSend(ctx, sqlcgen.ClaimAuthEmailSendParams{
			Email:          email,
			Now:            now,
			CooldownCutoff: now.Add(-45 * time.Second),
			WindowCutoff:   now.Add(-time.Hour),
			MaxSends:       5,
		})
	}
	window := func() sqlcgen.GetAuthEmailSendWindowRow {
		t.Helper()
		row, err := q.GetAuthEmailSendWindow(ctx, email)
		if err != nil {
			t.Fatalf("GetAuthEmailSendWindow: %v", err)
		}
		return row
	}

	first, err := claim(t0)
	if err != nil || first.WindowSends != 1 {
		t.Fatalf("first claim = %+v (%v), want 1 send", first, err)
	}

	t.Run("a claim inside the cooldown is refused and writes nothing", func(t *testing.T) {
		if _, err := claim(t0.Add(10 * time.Second)); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
		w := window()
		if !w.LastRequestedAt.Equal(t0) || w.WindowSends != 1 {
			t.Fatalf("window after a refusal = %+v, want it unchanged", w)
		}
	})
	t.Run("five claims fit in the hour", func(t *testing.T) {
		for i := 1; i <= 4; i++ {
			row, err := claim(t0.Add(time.Duration(i) * time.Minute))
			if err != nil || row.WindowSends != int32(i+1) {
				t.Fatalf("claim %d = %+v (%v), want %d sends", i+1, row, err, i+1)
			}
		}
	})
	t.Run("the sixth in the hour is refused and writes nothing", func(t *testing.T) {
		if _, err := claim(t0.Add(5 * time.Minute)); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
		w := window()
		if w.WindowSends != 5 || !w.LastRequestedAt.Equal(t0.Add(4*time.Minute)) || !w.WindowStartedAt.Valid || !w.WindowStartedAt.Time.Equal(t0) {
			t.Fatalf("window after the hourly refusal = %+v, want it unchanged", w)
		}
	})
	t.Run("once the window cutoff passes the count restarts at 1", func(t *testing.T) {
		now := t0.Add(61 * time.Minute)
		row, err := claim(now)
		if err != nil || row.WindowSends != 1 {
			t.Fatalf("claim after the hour = %+v (%v), want 1 send", row, err)
		}
		w := window()
		if !w.WindowStartedAt.Valid || !w.WindowStartedAt.Time.Equal(now) {
			t.Fatalf("window start = %+v, want %v", w.WindowStartedAt, now)
		}
	})
	t.Run("the old link flow claim still works on the same row", func(t *testing.T) {
		_, err := q.ClaimEmailCooldown(ctx, sqlcgen.ClaimEmailCooldownParams{
			Email:          "legacy@example.com",
			RequestedAt:    t0,
			CooldownCutoff: t0.Add(-45 * time.Second),
		})
		if err != nil {
			t.Fatalf("ClaimEmailCooldown: %v", err)
		}
	})
}
