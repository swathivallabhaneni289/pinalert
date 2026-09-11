package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestIdentitySchemaSupportsMagicLinks proves migration 00002 shipped the
// shape 01.1-01-PLAN.md requires: sessions.account_id exists and is
// nullable (T-01-56 / AR-27 — a valid signed cookie can exist with no
// sessions row, let alone an account_id, so this column must never be NOT
// NULL), and a magic_link_tokens row inserted via the generated Querier is
// readable back by its token_hash.
func TestIdentitySchemaSupportsMagicLinks(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	var isNullable string
	err := pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'sessions' AND column_name = 'account_id'
	`).Scan(&isNullable)
	if err != nil {
		t.Fatalf("querying information_schema.columns for sessions.account_id: %v", err)
	}
	if isNullable != "YES" {
		t.Fatalf("sessions.account_id is_nullable = %q, want YES", isNullable)
	}

	expiresAt := time.Now().Add(5 * time.Minute).Truncate(time.Microsecond)
	inserted, err := q.InsertMagicLinkToken(ctx, sqlcgen.InsertMagicLinkTokenParams{
		TokenHash: "deadbeef",
		Email:     "visitor@example.com",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("InsertMagicLinkToken: %v", err)
	}
	if inserted.ID == 0 {
		t.Fatalf("InsertMagicLinkToken returned zero ID")
	}
	if !inserted.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("InsertMagicLinkToken ExpiresAt = %v, want %v", inserted.ExpiresAt, expiresAt)
	}

	var readBackEmail string
	err = pool.QueryRow(ctx, `SELECT email FROM magic_link_tokens WHERE token_hash = $1`, "deadbeef").Scan(&readBackEmail)
	if err != nil {
		t.Fatalf("reading back magic_link_tokens row by token_hash: %v", err)
	}
	if readBackEmail != "visitor@example.com" {
		t.Fatalf("readBackEmail = %q, want visitor@example.com", readBackEmail)
	}
}

// TestTruncateClearsIdentityTables proves testutil.Truncate's updated
// statement actually empties accounts and magic_link_tokens, not just the
// pre-existing reports/sessions tables (RESEARCH.md Pitfall 5: rows in the
// new tables otherwise leak between packages under CI's `-p 1`).
func TestTruncateClearsIdentityTables(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	if _, err := q.InsertMagicLinkToken(ctx, sqlcgen.InsertMagicLinkTokenParams{
		TokenHash: "seed-hash",
		Email:     "seed@example.com",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("seeding magic_link_tokens: %v", err)
	}
	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "seed@example.com")

	testutil.Truncate(t, pool)

	var accountCount, tokenCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&accountCount); err != nil {
		t.Fatalf("counting accounts: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM magic_link_tokens`).Scan(&tokenCount); err != nil {
		t.Fatalf("counting magic_link_tokens: %v", err)
	}
	if accountCount != 0 {
		t.Fatalf("accounts has %d rows after Truncate, want 0", accountCount)
	}
	if tokenCount != 0 {
		t.Fatalf("magic_link_tokens has %d rows after Truncate, want 0", tokenCount)
	}
}

// TestBindSessionAccountUpsertsWhenSessionRowAbsent proves the upsert shape
// from Task 1 (RESEARCH.md Pitfall 2): a browser can hold a valid signed
// cookie with no sessions row at all (internal/session/cookie.go's
// log-and-continue persist-failure path), and BindSessionAccount must still
// succeed and leave the session verified — never silently no-op like a bare
// UPDATE would.
func TestBindSessionAccountUpsertsWhenSessionRowAbsent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "visitor@example.com")
	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, "visitor@example.com").Scan(&accountID); err != nil {
		t.Fatalf("reading back account id: %v", err)
	}

	var sessionRowCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE session_id = $1`, "absent-session").Scan(&sessionRowCount); err != nil {
		t.Fatalf("counting sessions rows: %v", err)
	}
	if sessionRowCount != 0 {
		t.Fatalf("expected no sessions row before BindSessionAccount, found %d", sessionRowCount)
	}

	if err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{
		SessionID: "absent-session",
		AccountID: &accountID,
	}); err != nil {
		t.Fatalf("BindSessionAccount: %v", err)
	}

	row, err := q.GetAccountBySessionID(ctx, "absent-session")
	if err != nil {
		t.Fatalf("GetAccountBySessionID after bind: %v", err)
	}
	if row.ID != accountID || row.Email != "visitor@example.com" {
		t.Fatalf("GetAccountBySessionID = %+v, want account %d/visitor@example.com", row, accountID)
	}
}

// TestBindSessionAccountIsIdempotent covers Task 1's second BindSessionAccount
// behaviour claim: calling it twice for the same session and account leaves
// exactly one sessions row.
func TestBindSessionAccountIsIdempotent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "repeat@example.com")
	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, "repeat@example.com").Scan(&accountID); err != nil {
		t.Fatalf("reading back account id: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{
			SessionID: "repeat-session",
			AccountID: &accountID,
		}); err != nil {
			t.Fatalf("BindSessionAccount call %d: %v", i, err)
		}
	}

	var sessionRowCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE session_id = $1`, "repeat-session").Scan(&sessionRowCount); err != nil {
		t.Fatalf("counting sessions rows: %v", err)
	}
	if sessionRowCount != 1 {
		t.Fatalf("sessions rows for repeat-session = %d, want 1", sessionRowCount)
	}
}

// TestConsumeTokenIsSingleUse proves ConsumeToken returns the email on the
// first call and pgx.ErrNoRows on a second call against the same hash.
func TestConsumeTokenIsSingleUse(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	if _, err := q.InsertMagicLinkToken(ctx, sqlcgen.InsertMagicLinkTokenParams{
		TokenHash: "single-use-hash",
		Email:     "onceonly@example.com",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("InsertMagicLinkToken: %v", err)
	}

	email, err := q.ConsumeToken(ctx, "single-use-hash")
	if err != nil {
		t.Fatalf("first ConsumeToken: %v", err)
	}
	if email != "onceonly@example.com" {
		t.Fatalf("first ConsumeToken email = %q, want onceonly@example.com", email)
	}

	_, err = q.ConsumeToken(ctx, "single-use-hash")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second ConsumeToken err = %v, want pgx.ErrNoRows", err)
	}
}

// TestConsumeTokenRejectsExpired proves ConsumeToken reports no rows for a
// token whose expires_at is already in the past, and that used_at is left
// NULL — an expired token must never be marked used by a failed consume
// attempt.
func TestConsumeTokenRejectsExpired(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.MustExec(t, pool,
		`INSERT INTO magic_link_tokens (token_hash, email, expires_at) VALUES ($1, $2, $3)`,
		"expired-hash", "expired@example.com", time.Now().Add(-1*time.Minute),
	)

	_, err := q.ConsumeToken(ctx, "expired-hash")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("ConsumeToken on expired token err = %v, want pgx.ErrNoRows", err)
	}

	var usedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT used_at FROM magic_link_tokens WHERE token_hash = $1`, "expired-hash").Scan(&usedAt); err != nil {
		t.Fatalf("reading back used_at: %v", err)
	}
	if usedAt != nil {
		t.Fatalf("used_at = %v after a failed expired consume, want NULL", *usedAt)
	}
}

// TestConsumeTokenConcurrent proves exactly one of two simultaneous
// ConsumeToken calls against one token succeeds — the atomic conditional
// UPDATE's whole reason for existing over a read-then-write shape
// (RESEARCH.md "Don't Hand-Roll" TOCTOU row).
func TestConsumeTokenConcurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	if _, err := q.InsertMagicLinkToken(ctx, sqlcgen.InsertMagicLinkTokenParams{
		TokenHash: "concurrent-hash",
		Email:     "concurrent@example.com",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("InsertMagicLinkToken: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	noRowsCount := 0

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := q.ConsumeToken(ctx, "concurrent-hash")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if errors.Is(err, pgx.ErrNoRows) {
				noRowsCount++
			} else {
				t.Errorf("unexpected ConsumeToken error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("successCount = %d, want exactly 1", successCount)
	}
	if noRowsCount != 1 {
		t.Fatalf("noRowsCount = %d, want exactly 1", noRowsCount)
	}
}

// TestResolveAccountIsIdempotent proves InsertAccount followed by
// GetAccountByEmail returns a stable id, and a second InsertAccount for the
// same email neither creates a duplicate row nor errors.
func TestResolveAccountIsIdempotent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	if err := q.InsertAccount(ctx, "resolve@example.com"); err != nil {
		t.Fatalf("first InsertAccount: %v", err)
	}
	first, err := q.GetAccountByEmail(ctx, "resolve@example.com")
	if err != nil {
		t.Fatalf("GetAccountByEmail after first insert: %v", err)
	}

	if err := q.InsertAccount(ctx, "resolve@example.com"); err != nil {
		t.Fatalf("second InsertAccount: %v", err)
	}
	second, err := q.GetAccountByEmail(ctx, "resolve@example.com")
	if err != nil {
		t.Fatalf("GetAccountByEmail after second insert: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("account id changed across InsertAccount calls: %d != %d", first.ID, second.ID)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE email = $1`, "resolve@example.com").Scan(&count); err != nil {
		t.Fatalf("counting accounts: %v", err)
	}
	if count != 1 {
		t.Fatalf("accounts rows for resolve@example.com = %d, want 1", count)
	}
}

// TestGetAccountBySessionIDReturnsNoRowsWhenUnverified proves a session
// whose account_id is NULL — the default for every anonymous session —
// yields no row, which callers must treat as "unverified", never as an
// error.
func TestGetAccountBySessionIDReturnsNoRowsWhenUnverified(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	if err := q.UpsertSession(ctx, "unverified-session"); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	_, err := q.GetAccountBySessionID(ctx, "unverified-session")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetAccountBySessionID for unverified session err = %v, want pgx.ErrNoRows", err)
	}
}
