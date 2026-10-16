package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// These tests run a moderator write and the worker's verdict write at the same
// time against a real Postgres (WR-S01). Each one holds one side inside an open
// transaction, waits until the other side is really blocked on a row lock, and
// only then commits, so the two statements overlap every run.

// modWaitForLockWaiters polls until at least n sessions of this database are
// waiting on a lock, and fails the test if that never happens. A race test
// that commits the holder before the other side is blocked would run the two
// sides one after the other and prove nothing.
func modWaitForLockWaiters(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM pg_stat_activity
			  WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)
		if err != nil {
			t.Fatalf("reading pg_stat_activity: %v", err)
		}
		if waiting >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d sessions waiting on a lock after 5 s, want %d", waiting, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// modVerdict reads the verdict of one check, empty when it is NULL.
func modVerdict(t *testing.T, pool *pgxpool.Pool, reportID int64) string {
	t.Helper()
	var v string
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(verdict, '') FROM moderation_checks WHERE report_id = $1`, reportID).Scan(&v); err != nil {
		t.Fatalf("reading verdict of %d: %v", reportID, err)
	}
	return v
}

// modAwait waits for the result of a goroutine, failing the test after 10 s.
func modAwait(t *testing.T, done <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not finish within 10 s", what)
		return nil
	}
}

// modWorkerWrite runs the worker's verdict write in a goroutine with its own
// deadline, so a hang fails the test instead of stalling it.
func modWorkerWrite(q *sqlcgen.Queries, arg sqlcgen.FinishCheckAndSetStateParams) <-chan error {
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- q.FinishCheckAndSetState(ctx, arg)
	}()
	return done
}

// TestWorkerRejectWaitingOnKeepLeavesReportKept is the WR-S01 race: Keep runs
// inside an open transaction (as a single autocommit statement does in
// production while it is in flight), the worker's reject arrives and blocks,
// then Keep commits. The report must end kept and the check closed by Keep,
// with no worker verdict written over it.
func TestWorkerRejectWaitingOnKeepLeavesReportKept(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Now().UTC()
	r := modRawReport(t, pool, "race keep", "low", "pending", now, now.Add(24*time.Hour))
	modPendingCheck(t, q, r)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := sqlcgen.New(tx).KeepReport(ctx, r); err != nil {
		t.Fatalf("KeepReport: %v", err)
	}

	done := modWorkerWrite(q, sqlcgen.FinishCheckAndSetStateParams{
		ModerationState: "removed", Verdict: "reject", ReportID: r,
	})
	modWaitForLockWaiters(t, pool, 1)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit Keep: %v", err)
	}
	if err := modAwait(t, done, "worker write"); err != nil {
		t.Fatalf("worker write: %v", err)
	}

	if s := modState(t, pool, r); s != "clear" {
		t.Errorf("state after Keep then a late reject = %q, want clear", s)
	}
	if c := modCheck(t, pool, r); c.Status != "done" || c.NeedsReview {
		t.Errorf("check = %+v, want done and no review", c)
	}
	if v := modVerdict(t, pool, r); v != "" {
		t.Errorf("verdict = %q, want none (the worker must not write over Keep)", v)
	}
}

// TestWorkerPassWaitingOnRemoveLeavesReportRemoved is the same race for
// Remove: a late worker pass must not release a report a moderator removed.
func TestWorkerPassWaitingOnRemoveLeavesReportRemoved(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Now().UTC()
	r := modRawReport(t, pool, "race remove", "low", "pending", now, now.Add(24*time.Hour))
	modPendingCheck(t, q, r)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := sqlcgen.New(tx).RemoveReport(ctx, r); err != nil {
		t.Fatalf("RemoveReport: %v", err)
	}

	done := modWorkerWrite(q, sqlcgen.FinishCheckAndSetStateParams{
		ModerationState: "clear", Verdict: "pass", ReportID: r,
	})
	modWaitForLockWaiters(t, pool, 1)
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit Remove: %v", err)
	}
	if err := modAwait(t, done, "worker write"); err != nil {
		t.Fatalf("worker write: %v", err)
	}

	if s := modState(t, pool, r); s != "removed" {
		t.Errorf("state after Remove then a late pass = %q, want removed", s)
	}
	if c := modCheck(t, pool, r); c.Status != "done" {
		t.Errorf("check status = %q, want done", c.Status)
	}
	if v := modVerdict(t, pool, r); v != "" {
		t.Errorf("verdict = %q, want none (the worker must not write over Remove)", v)
	}
}
