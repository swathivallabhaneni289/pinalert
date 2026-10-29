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

// TestIdentitySchemaSupportsMagicLinks proves migration 00002 shipped the
// shape 01.1-01-PLAN.md requires: sessions.account_id exists and is
// nullable (T-01-56 / AR-27: a valid signed cookie can exist with no
// sessions row, let alone an account_id, so this column must never be NOT
// NULL).
//
// Amended 2026-10-29 by Phase 7.1 plan 16: the second half of this test, which
// inserted a sign in link token row and read it back by its hash, is removed
// with the link token table (migration 00015). The test keeps its name so its
// history stays easy to follow.
func TestIdentitySchemaSupportsMagicLinks(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

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
}

// TestTruncateClearsIdentityTables proves testutil.Truncate's updated
// statement actually empties accounts, auth_codes and auth_throttles, not just
// the pre-existing reports/sessions tables (RESEARCH.md Pitfall 5: rows in the
// new tables otherwise leak between packages under CI's `-p 1`).
//
// Amended 2026-10-29 by Phase 7.1 plan 16: the sign in link token row it used to
// seed and check is gone with that table (migration 00015); it now seeds an
// auth_codes row and an auth_throttles row instead. The accounts check is kept.
func TestTruncateClearsIdentityTables(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "seed@example.com")
	testutil.MustExec(t, pool,
		`INSERT INTO auth_codes (session_id, purpose, delivery, email, code_hash, expires_at)
		 VALUES ('truncate-seed', 'signup', 'code', $1, $2, now() + interval '10 minutes')`,
		"seed@example.com", hash)
	testutil.MustExec(t, pool,
		`INSERT INTO auth_throttles (scope, key_hash, failures, last_failure_at) VALUES ('sign_in', $1, 1, now())`,
		hash)

	testutil.Truncate(t, pool)

	for _, table := range []string{"accounts", "auth_codes", "auth_throttles"} {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("counting %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s has %d rows after Truncate, want 0", table, n)
		}
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

	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{
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
		if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{
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

// TestReportsByAccountSpansMultipleSessions is 01.1-07-PLAN.md Task 1's core
// claim: an account with two different verified sessions (multi-device,
// D-10) sees reports filed from BOTH sessions on one ReportsByAccount call,
// newest first — and nothing filed by a different account's session, or by
// a session with no account bound at all.
func TestReportsByAccountSpansMultipleSessions(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "multi-device@example.com")
	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, "multi-device@example.com").Scan(&accountID); err != nil {
		t.Fatalf("reading back account id: %v", err)
	}
	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "other-account@example.com")
	var otherAccountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, "other-account@example.com").Scan(&otherAccountID); err != nil {
		t.Fatalf("reading back other account id: %v", err)
	}

	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "device-a", AccountID: &accountID}); err != nil {
		t.Fatalf("binding device-a: %v", err)
	}
	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "device-b", AccountID: &accountID}); err != nil {
		t.Fatalf("binding device-b: %v", err)
	}
	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "device-c-other-account", AccountID: &otherAccountID}); err != nil {
		t.Fatalf("binding device-c-other-account: %v", err)
	}
	if err := q.UpsertSession(ctx, "device-d-unbound"); err != nil {
		t.Fatalf("UpsertSession device-d-unbound: %v", err)
	}

	insertReportAt := func(sessionID, description string, createdAt time.Time) {
		testutil.MustExec(t, pool,
			`INSERT INTO reports
				(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
			 VALUES ($1, 'flood', 'low', $2, 12.9716, 77.5946, 'tdr1qgzn', $3::timestamptz, $3::timestamptz + interval '1 hour')`,
			sessionID, description, createdAt,
		)
	}

	base := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	insertReportAt("device-a", "from device A", base)
	insertReportAt("device-b", "from device B", base.Add(1*time.Minute))
	insertReportAt("device-c-other-account", "from a different account", base.Add(2*time.Minute))
	insertReportAt("device-d-unbound", "from an unbound session", base.Add(3*time.Minute))

	rows, err := q.ReportsByAccount(ctx, &accountID)
	if err != nil {
		t.Fatalf("ReportsByAccount: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want exactly 2: %+v", len(rows), rows)
	}
	if rows[0].Description != "from device B" || rows[1].Description != "from device A" {
		t.Fatalf("rows not newest-first: got [%q, %q], want [\"from device B\", \"from device A\"]",
			rows[0].Description, rows[1].Description)
	}
}

// TestReportsByAccountReturnsNothingForAccountWithNoReports proves an
// account that has never filed anything gets an empty (not nil-panicking,
// not error) result — the profile page's honest empty state depends on
// this returning a clean zero-length slice.
func TestReportsByAccountReturnsNothingForAccountWithNoReports(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "no-reports@example.com")
	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, "no-reports@example.com").Scan(&accountID); err != nil {
		t.Fatalf("reading back account id: %v", err)
	}
	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "empty-device", AccountID: &accountID}); err != nil {
		t.Fatalf("binding empty-device: %v", err)
	}

	rows, err := q.ReportsByAccount(ctx, &accountID)
	if err != nil {
		t.Fatalf("ReportsByAccount: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("len(rows) = %d, want 0 for an account with no reports", len(rows))
	}
}

// TestReportsByAccountIncludesExpired proves a report whose expires_at is
// already in the past still appears on the owner's own profile — a
// person's history does not vanish from their own profile when a report
// ages out of the public feed (unlike NearbyReports, which applies
// expires_at > now()).
func TestReportsByAccountIncludesExpired(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	testutil.MustExec(t, pool, `INSERT INTO accounts (email) VALUES ($1)`, "expired-owner@example.com")
	var accountID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM accounts WHERE email = $1`, "expired-owner@example.com").Scan(&accountID); err != nil {
		t.Fatalf("reading back account id: %v", err)
	}
	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "expired-device", AccountID: &accountID}); err != nil {
		t.Fatalf("binding expired-device: %v", err)
	}

	testutil.MustExec(t, pool,
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		 VALUES ($1, 'flood', 'low', 'a report that has since expired', 12.9716, 77.5946, 'tdr1qgzn',
		         now() - interval '2 hours', now() - interval '1 hour')`,
		"expired-device",
	)

	rows, err := q.ReportsByAccount(ctx, &accountID)
	if err != nil {
		t.Fatalf("ReportsByAccount: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (an expired report must still appear on the owner's own profile)", len(rows))
	}
	if rows[0].Description != "a report that has since expired" {
		t.Fatalf("unexpected report returned: %+v", rows[0])
	}
}
