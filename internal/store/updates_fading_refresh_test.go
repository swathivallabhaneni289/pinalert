package store_test

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestFadingRefreshedInPlaceNoSecondPush pins review WR-03 of quick task
// 261030-64q (2026-10-30). A confirmation from a new spot can move a report's
// fade time later (backlog 999.12), so a report can enter its last hour more
// than once. The owner's rule is few notes, never piled up: the bell keeps ONE
// fading row per account and report, refreshed to the latest state (moved to
// the top and unread again), and the phone gets the "fades in about an hour"
// note at most once, because the changed push is keyed on the update row and a
// refreshed row keeps its id.
func TestFadingRefreshedInPlaceNoSecondPush(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "fading-refresh@example.com", "sess-fading-refresh")
	r := updReport(t, pool, "sess-fading-refresh", updNow.Add(30*time.Minute))
	updCursor(t, pool, a, r, "live")

	enqueue := func(now time.Time) int64 {
		t.Helper()
		n, err := q.EnqueueChangedFromUpdates(ctx, sqlcgen.EnqueueChangedFromUpdatesParams{
			Now: now, AccountID: a, Since: now.Add(-6 * time.Hour),
		})
		if err != nil {
			t.Fatalf("EnqueueChangedFromUpdates: %v", err)
		}
		return n
	}

	// The first fade: one bell row and one phone note.
	e1 := updNow.Add(30 * time.Minute)
	firstID, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, e1, updNow))
	if err != nil {
		t.Fatalf("first fading: %v", err)
	}
	if n := enqueue(updNow); n != 1 {
		t.Fatalf("first fading queued %d phone notes, want 1", n)
	}
	// The person reads the bell row.
	testutil.MustExec(t, pool, `UPDATE updates SET read_at = $2 WHERE id = $1`, firstID, updNow.Add(time.Minute))

	// A confirmation from a new spot moves the fade time to e2, and the report
	// enters its last hour again after the hour gap.
	e2 := updNow.Add(4 * time.Hour)
	later := updNow.Add(3*time.Hour + 30*time.Minute)
	secondID, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, e2, later))
	if err != nil {
		t.Fatalf("second fading: %v", err)
	}
	if secondID != firstID {
		t.Errorf("second fading returned update %d, want the same row %d refreshed in place", secondID, firstID)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'fading'`, a); n != 1 {
		t.Errorf("%d fading rows, want 1 (the bell must not pile up)", n)
	}
	var createdAt time.Time
	var readAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT created_at, read_at FROM updates WHERE id = $1`, firstID).Scan(&createdAt, &readAt); err != nil {
		t.Fatalf("reading the refreshed row: %v", err)
	}
	if !createdAt.Equal(later) || readAt != nil {
		t.Errorf("refreshed row = created_at %v read_at %v, want %v and unread (the latest state)", createdAt, readAt, later)
	}

	// The refreshed row writes no second phone note.
	if n := enqueue(later); n != 0 {
		t.Errorf("the refreshed fading row queued %d more phone notes, want 0", n)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM push_outbox WHERE account_id = $1 AND kind = 'changed'`, a); n != 1 {
		t.Errorf("%d changed pushes for the report, want exactly 1", n)
	}
}

// TestFadingRefreshRespectsCeiling keeps the soft ceiling of the bell: a refresh
// is an event like a new row, so over the ceiling it rolls into the "more" row
// and the existing fading row is left as it was.
func TestFadingRefreshRespectsCeiling(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "fading-ceiling@example.com", "sess-fading-ceiling")
	r := updReport(t, pool, "sess-fading-ceiling", updNow.Add(30*time.Minute))
	updCursor(t, pool, a, r, "live")
	if _, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, updNow.Add(30*time.Minute), updNow)); err != nil {
		t.Fatalf("first fading: %v", err)
	}

	later := updNow.Add(3*time.Hour + 30*time.Minute)
	for i := 0; i < updCeiling; i++ {
		other := updReport(t, pool, "sess-fading-ceiling", later.Add(time.Hour))
		updRow(t, pool, a, other, "reached_live", later.Add(-time.Minute))
	}
	if _, err := q.MarkFadingAndInsertUpdate(ctx, fadingArg(a, r, updNow.Add(4*time.Hour), later)); err != nil {
		t.Fatalf("fading over the ceiling: %v", err)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'fading'`, a); n != 1 {
		t.Errorf("%d fading rows, want 1", n)
	}
	if n := updCount(t, pool, `SELECT count(*) FROM updates WHERE account_id = $1 AND kind = 'more'`, a); n != 1 {
		t.Errorf("%d rolled up rows, want 1 (over the ceiling the event rolls up)", n)
	}
	var createdAt time.Time
	if err := pool.QueryRow(ctx, `SELECT created_at FROM updates WHERE account_id = $1 AND kind = 'fading'`, a).Scan(&createdAt); err != nil {
		t.Fatalf("reading the fading row: %v", err)
	}
	if !createdAt.Equal(updNow) {
		t.Errorf("fading row created_at = %v, want it left at %v over the ceiling", createdAt, updNow)
	}
}
