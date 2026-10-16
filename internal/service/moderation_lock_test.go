package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// These tests run the real ModerationService (with PgxModerationTx) and the
// worker's verdict write against each other on a real Postgres, in both lock
// orders, and check that neither side ever fails with a deadlock (WR-S01).
// Each one holds one side open, waits until the other side is really blocked
// on a lock, and only then lets the first side commit.

// lkWaitForLockWaiters polls until at least one session of this database is
// waiting on a lock, failing the test after 5 s.
func lkWaitForLockWaiters(t *testing.T, pool *pgxpool.Pool) {
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
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no session waiting on a lock after 5 s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func lkAwait(t *testing.T, done <-chan error, what string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not finish within 10 s", what)
		return nil
	}
}

// lkNoDeadlock fails the test when err is a Postgres deadlock (40P01).
func lkNoDeadlock(t *testing.T, err error, who string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
		t.Fatalf("%s failed with a deadlock: %v", who, err)
	}
}

// lkHeldTx is a ModerationTxFunc that runs the real Keep or Remove body and
// then, before the commit, signals inside and waits for release.
func lkHeldTx(pool *pgxpool.Pool, inside chan<- struct{}, release <-chan struct{}) service.ModerationTxFunc {
	return func(ctx context.Context, fn func(service.ModerationQuerier) error) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if err := fn(sqlcgen.New(tx)); err != nil {
				return err
			}
			close(inside)
			<-release
			return nil
		})
	}
}

func lkAuditRows(t *testing.T, pool *pgxpool.Pool, id int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM moderation_actions WHERE report_id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("counting audit rows: %v", err)
	}
	return n
}

// lkWorkerWrite runs the worker's verdict write on its own connection.
func lkWorkerWrite(q *sqlcgen.Queries, id int64, state, verdict string) <-chan error {
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done <- q.FinishCheckAndSetState(ctx, sqlcgen.FinishCheckAndSetStateParams{
			ModerationState: state, Verdict: verdict, ReportID: id,
		})
	}()
	return done
}

func lkAct(s *service.ModerationService, action string, id, mod int64) <-chan error {
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		if action == service.ModerationActionKeep {
			_, err = s.Keep(ctx, id, mod)
		} else {
			_, err = s.Remove(ctx, id, mod)
		}
		done <- err
	}()
	return done
}

func TestModerationKeepRemoveAndWorkerLockOrder(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	mod := testutil.SeedReporterAccount(t, pool, "lock-order-mod@example.com")
	now := time.Now().UTC()
	created, expires := now.Add(-time.Minute), now.Add(24*time.Hour)

	// Moderator first: Keep or Remove holds its transaction open, the worker's
	// write blocks on the check row, then the moderator commits.
	moderatorFirst := []struct {
		name, action, workerState, workerVerdict, wantState string
	}{
		{"keep first then a late reject", service.ModerationActionKeep, "removed", "reject", "clear"},
		{"remove first then a late pass", service.ModerationActionRemove, "clear", "pass", "removed"},
	}
	for _, c := range moderatorFirst {
		t.Run(c.name, func(t *testing.T) {
			id := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, mwDesc, created, expires)
			inside, release := make(chan struct{}), make(chan struct{})
			s := service.NewModerationService(q, service.WithModerationTx(lkHeldTx(pool, inside, release)))
			actDone := lkAct(s, c.action, id, mod)
			select {
			case <-inside:
			case err := <-actDone:
				t.Fatalf("%s returned before holding its transaction: %v", c.action, err)
			case <-time.After(10 * time.Second):
				t.Fatalf("%s never reached its commit", c.action)
			}
			workerDone := lkWorkerWrite(q, id, c.workerState, c.workerVerdict)
			lkWaitForLockWaiters(t, pool)
			close(release)

			actErr := lkAwait(t, actDone, c.action)
			workerErr := lkAwait(t, workerDone, "worker write")
			lkNoDeadlock(t, actErr, c.action)
			lkNoDeadlock(t, workerErr, "worker write")
			if actErr != nil || workerErr != nil {
				t.Fatalf("%s error %v, worker error %v; want both nil", c.action, actErr, workerErr)
			}
			if got := mwReportState(t, pool, id); got != c.wantState {
				t.Errorf("state = %q, want %q", got, c.wantState)
			}
			chk := mwCheckOf(t, pool, id)
			if chk.Status != "done" || chk.Verdict != nil {
				t.Errorf("check status %q verdict %v, want done with no worker verdict", chk.Status, chk.Verdict)
			}
			if n := lkAuditRows(t, pool, id); n != 1 {
				t.Errorf("audit rows = %d, want 1", n)
			}
		})
	}

	// Worker first: the worker's write holds its transaction open, Keep or
	// Remove blocks on the check row lock, then the worker commits. Keep then
	// sees the report removed and answers 409; Remove still removes the
	// report the worker released.
	workerFirst := []struct {
		name, action, workerState, workerVerdict, wantState string
		wantErr                                             error
		wantAudit                                           int
	}{
		{"reject first then keep", service.ModerationActionKeep, "removed", "reject", "removed", service.ErrAlreadyHandled, 0},
		{"pass first then remove", service.ModerationActionRemove, "clear", "pass", "removed", nil, 1},
	}
	for _, c := range workerFirst {
		t.Run(c.name, func(t *testing.T) {
			id := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, mwDesc, created, expires)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if err := sqlcgen.New(tx).FinishCheckAndSetState(ctx, sqlcgen.FinishCheckAndSetStateParams{
				ModerationState: c.workerState, Verdict: c.workerVerdict, ReportID: id,
			}); err != nil {
				t.Fatalf("worker write: %v", err)
			}

			s := service.NewModerationService(q, service.WithModerationTx(service.PgxModerationTx(pool)))
			actDone := lkAct(s, c.action, id, mod)
			lkWaitForLockWaiters(t, pool)
			if err := tx.Commit(ctx); err != nil {
				lkNoDeadlock(t, err, "worker commit")
				t.Fatalf("worker commit: %v", err)
			}

			actErr := lkAwait(t, actDone, c.action)
			lkNoDeadlock(t, actErr, c.action)
			if !errors.Is(actErr, c.wantErr) {
				t.Fatalf("%s error = %v, want %v", c.action, actErr, c.wantErr)
			}
			if got := mwReportState(t, pool, id); got != c.wantState {
				t.Errorf("state = %q, want %q", got, c.wantState)
			}
			chk := mwCheckOf(t, pool, id)
			if chk.Status != "done" || chk.Verdict == nil || *chk.Verdict != c.workerVerdict {
				t.Errorf("check status %q verdict %v, want done with verdict %s", chk.Status, chk.Verdict, c.workerVerdict)
			}
			if n := lkAuditRows(t, pool, id); n != c.wantAudit {
				t.Errorf("audit rows = %d, want %d", n, c.wantAudit)
			}
		})
	}

	// Unheld: many rounds of a moderator and the worker racing freely on a
	// fresh report each time. Whichever wins, neither side may deadlock.
	t.Run("free races never deadlock", func(t *testing.T) {
		s := service.NewModerationService(q, service.WithModerationTx(service.PgxModerationTx(pool)))
		for i := 0; i < 60; i++ {
			id := insertPendingReport(t, pool, q, service.SeverityLow, service.CategoryFlood, mwDesc, created, expires)
			action := service.ModerationActionKeep
			if i%2 == 1 {
				action = service.ModerationActionRemove
			}
			actDone := lkAct(s, action, id, mod)
			workerDone := lkWorkerWrite(q, id, "removed", "reject")
			actErr := lkAwait(t, actDone, action)
			workerErr := lkAwait(t, workerDone, "worker write")
			lkNoDeadlock(t, actErr, action)
			lkNoDeadlock(t, workerErr, "worker write")
			if workerErr != nil {
				t.Fatalf("round %d worker error: %v", i, workerErr)
			}
			if actErr != nil && !errors.Is(actErr, service.ErrAlreadyHandled) {
				t.Fatalf("round %d %s error: %v", i, action, actErr)
			}
			if chk := mwCheckOf(t, pool, id); chk.Status != "done" {
				t.Fatalf("round %d check status %q, want done", i, chk.Status)
			}
		}
	})
}
