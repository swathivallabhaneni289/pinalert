package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// codeHash is a well formed 64 hex keyed hash. The tests never compute a real
// one: the store only checks the shape.
const codeHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func issueCode(t *testing.T, q *sqlcgen.Queries, session, purpose, delivery string, expires time.Time) int64 {
	t.Helper()
	id, err := q.IssueAuthCode(context.Background(), sqlcgen.IssueAuthCodeParams{
		SessionID: session,
		Purpose:   purpose,
		Delivery:  delivery,
		Email:     "code-owner@example.com",
		CodeHash:  codeHash,
		ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("IssueAuthCode(%s, %s): %v", session, purpose, err)
	}
	return id
}

func liveCodeIDs(t *testing.T, pool *pgxpool.Pool, session, purpose string) []int64 {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT id FROM auth_codes WHERE session_id = $1 AND purpose = $2 AND used_at IS NULL ORDER BY id`, session, purpose)
	if err != nil {
		t.Fatalf("reading live codes: %v", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// TestIssueAuthCodeRetiresPrevious proves a second request from one session
// retires the first code of the same purpose, so resending never multiplies the
// try budget, and leaves a code of another purpose or session alone.
func TestIssueAuthCodeRetiresPrevious(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	expires := time.Now().Add(10 * time.Minute)

	first := issueCode(t, q, "sess-a", "signup", "code", expires)
	reset := issueCode(t, q, "sess-a", "reset", "code", expires)
	other := issueCode(t, q, "sess-b", "signup", "code", expires)
	second := issueCode(t, q, "sess-a", "signup", "code", expires)

	if first == second {
		t.Fatal("a second request must write a new row")
	}
	t.Run("the signup purpose has exactly the newer live row", func(t *testing.T) {
		ids := liveCodeIDs(t, pool, "sess-a", "signup")
		if len(ids) != 1 || ids[0] != second {
			t.Fatalf("live signup rows = %v, want [%d]", ids, second)
		}
	})
	t.Run("the other purpose is untouched", func(t *testing.T) {
		ids := liveCodeIDs(t, pool, "sess-a", "reset")
		if len(ids) != 1 || ids[0] != reset {
			t.Fatalf("live reset rows = %v, want [%d]", ids, reset)
		}
	})
	t.Run("another session is untouched", func(t *testing.T) {
		ids := liveCodeIDs(t, pool, "sess-b", "signup")
		if len(ids) != 1 || ids[0] != other {
			t.Fatalf("live rows of the other session = %v, want [%d]", ids, other)
		}
	})
	t.Run("the retired row is kept, marked used", func(t *testing.T) {
		var used bool
		if err := pool.QueryRow(context.Background(),
			`SELECT used_at IS NOT NULL FROM auth_codes WHERE id = $1`, first).Scan(&used); err != nil || !used {
			t.Fatalf("retired row used = %v (%v), want true", used, err)
		}
	})
}

// TestBumpAuthCodeAttemptBudget proves the try budget: five bumps succeed and
// count up, the sixth is refused with pgx.ErrNoRows, and a bump never reaches
// another session's or another purpose's row.
func TestBumpAuthCodeAttemptBudget(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	id := issueCode(t, q, "sess-a", "signup", "code", time.Now().Add(10*time.Minute))

	bump := func(session, purpose string) (sqlcgen.BumpAuthCodeAttemptRow, error) {
		return q.BumpAuthCodeAttempt(ctx, sqlcgen.BumpAuthCodeAttemptParams{
			SessionID: session, Purpose: purpose, MaxAttempts: 5,
		})
	}

	t.Run("another session gets nothing", func(t *testing.T) {
		if _, err := bump("sess-b", "signup"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
	})
	t.Run("another purpose gets nothing", func(t *testing.T) {
		if _, err := bump("sess-a", "reset"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
	})
	t.Run("five bumps count up and carry the row", func(t *testing.T) {
		for want := int32(1); want <= 5; want++ {
			row, err := bump("sess-a", "signup")
			if err != nil {
				t.Fatalf("bump %d: %v", want, err)
			}
			if row.Attempts != want || row.ID != id || row.CodeHash != codeHash || row.Delivery != "code" {
				t.Fatalf("bump %d = %+v", want, row)
			}
		}
	})
	t.Run("the sixth is refused and counts nothing", func(t *testing.T) {
		if _, err := bump("sess-a", "signup"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
		var attempts int32
		if err := pool.QueryRow(ctx, `SELECT attempts FROM auth_codes WHERE id = $1`, id).Scan(&attempts); err != nil || attempts != 5 {
			t.Fatalf("attempts = %d (%v), want 5", attempts, err)
		}
	})
	t.Run("a used row is refused at once", func(t *testing.T) {
		fresh := issueCode(t, q, "sess-c", "signup", "code", time.Now().Add(10*time.Minute))
		if n, err := q.ConsumeAuthCode(ctx, fresh); err != nil || n != 1 {
			t.Fatalf("consume = %d (%v), want 1", n, err)
		}
		if _, err := bump("sess-c", "signup"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
	})
}

// TestBumpAuthCodeAttemptExpired proves an expired row is refused at once and
// that nothing is counted against it.
func TestBumpAuthCodeAttemptExpired(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	id := issueCode(t, q, "sess-a", "signup", "code", time.Now().Add(-time.Minute))

	_, err := q.BumpAuthCodeAttempt(ctx, sqlcgen.BumpAuthCodeAttemptParams{
		SessionID: "sess-a", Purpose: "signup", MaxAttempts: 5,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
	var attempts int32
	if err := pool.QueryRow(ctx, `SELECT attempts FROM auth_codes WHERE id = $1`, id).Scan(&attempts); err != nil || attempts != 0 {
		t.Fatalf("attempts = %d (%v), want 0", attempts, err)
	}
	if n, err := q.ConsumeAuthCode(ctx, id); err != nil || n != 0 {
		t.Fatalf("consuming an expired code = %d (%v), want 0", n, err)
	}
}

// TestConsumeAuthCodeConcurrent proves exactly one of twenty parallel submits
// spends a code.
func TestConsumeAuthCodeConcurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	id := issueCode(t, q, "sess-a", "signup", "code", time.Now().Add(10*time.Minute))

	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := q.ConsumeAuthCode(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("ConsumeAuthCode: %v", err)
				return
			}
			won += int(n)
		}()
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("winners = %d, want exactly 1", won)
	}
}

// TestGetLatestAuthCode proves the screen reload read: the newest row of the
// session whatever its purpose, and nothing older than since.
func TestGetLatestAuthCode(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Now().Truncate(time.Second)
	expires := now.Add(10 * time.Minute)

	older := issueCode(t, q, "sess-a", "signup", "code", expires)
	newer := issueCode(t, q, "sess-a", "reset", "no_account", expires)
	issueCode(t, q, "sess-b", "signup", "code", expires)
	testutil.MustExec(t, pool, `UPDATE auth_codes SET created_at = $1 WHERE id = $2`, now.Add(-20*time.Minute), older)
	testutil.MustExec(t, pool, `UPDATE auth_codes SET created_at = $1 WHERE id = $2`, now.Add(-5*time.Minute), newer)

	t.Run("the newest row of the session wins whatever its purpose", func(t *testing.T) {
		row, err := q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{SessionID: "sess-a", Since: now.Add(-time.Hour)})
		if err != nil {
			t.Fatalf("GetLatestAuthCode: %v", err)
		}
		if row.ID != newer || row.Purpose != "reset" || row.Delivery != "no_account" || row.Email != "code-owner@example.com" {
			t.Fatalf("row = %+v, want the reset row %d", row, newer)
		}
	})
	t.Run("a row older than since is not returned", func(t *testing.T) {
		_, err := q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{SessionID: "sess-a", Since: now.Add(-time.Minute)})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
	})
	t.Run("a session with no row gets none", func(t *testing.T) {
		_, err := q.GetLatestAuthCode(ctx, sqlcgen.GetLatestAuthCodeParams{SessionID: "sess-none", Since: now.Add(-time.Hour)})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
	})
	t.Run("delete by address and prune remove rows", func(t *testing.T) {
		if err := q.DeleteAuthCodesForEmail(ctx, "code-owner@example.com"); err != nil {
			t.Fatalf("DeleteAuthCodesForEmail: %v", err)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_codes`).Scan(&n); err != nil || n != 0 {
			t.Fatalf("rows left = %d (%v), want 0", n, err)
		}
		issueCode(t, q, "sess-p", "signup", "code", now.Add(-2*time.Hour))
		issueCode(t, q, "sess-q", "signup", "code", now.Add(time.Hour))
		pruned, err := q.PruneAuthCodes(ctx, now)
		if err != nil || pruned != 1 {
			t.Fatalf("PruneAuthCodes = %d (%v), want 1", pruned, err)
		}
	})
}
