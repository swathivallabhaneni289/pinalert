package store_test

import (
	"context"
	"testing"
	"time"

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
