// Package testutil provides the shared, migrated Postgres test database
// handle used by every store-layer test in this project. Any later plan can
// call testutil.NewTestDB(t) and get a migrated, truncated database with no
// external CLI tool installed — CI's bare postgres:16 service container has
// no goose binary, so migrations are applied here in Go, the same way
// cmd/migrate applies them in production.
package testutil

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver, used by goose
	"github.com/pressly/goose/v3"

	"pinalert/internal/store"
)

// NewTestDB reads DATABASE_URL, applies the embedded migrations, truncates
// the identity and report tables, and returns a ready-to-use connection
// pool. It skips (via t.Skip) rather than fails when DATABASE_URL is unset,
// so `go test ./... -short` passes cleanly on a machine with no database
// configured.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping test that needs a real Postgres")
	}

	ctx := context.Background()

	// goose operates on a database/sql handle; pgx's stdlib shim provides one
	// against the same DSN the pgxpool below will also use.
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database/sql handle for migrations: %v", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(store.MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("setting goose dialect: %v", err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("opening pgxpool: %v", err)
	}

	Truncate(t, pool)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// Truncate resets every identity, report, vote, and moderation table
// (accounts / email_cooldowns / reports / sessions /
// votes / moderation_checks / report_flags / moderation_actions / updates /
// update_cursors / report_mutes / account_alert_settings / push_subscriptions /
// push_outbox / push_report_progress / official_alerts / official_alert_areas /
// official_feed_status / official_hazards / official_push_outbox / auth_codes / auth_throttles) to
// empty, restarting identity sequences, so tests that need a clean slate
// between sub-cases don't have to open a new pool via NewTestDB. The two
// official_feed_status rows that migration 00012 seeds (sachet and gdacs) are put
// back as fresh rows with no attempt, so a test never has to make them.
// Amended 2026-10-29 by Phase 7.1 plan 16: the sign in link token table is
// dropped by migration 00015, so it is no longer in this list.
// Amended 2026-10-29 by Phase 8 plan 08: the four comment tables (comments,
// comment_likes, comment_flags, comment_moderation_actions) are appended last.
// Amended 2026-10-29 by Phase 9 plan 02: account_follows and person_blocks are
// appended after them.
// Amended 2026-10-29 by Phase 9 plan 13: profile_ratings is appended last.
// Amended 2026-10-29 by Phase 9 plan 18: person_mutes is appended last.
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// votes and the Phase 3.2 and 3.3 tables are named explicitly for clarity, not
	// correctness: CASCADE already reaches them today through their foreign
	// keys on reports and accounts. Naming them keeps this list an honest
	// inventory of what it clears.
	MustExec(t, pool, "TRUNCATE accounts, email_cooldowns, reports, sessions, votes, moderation_checks, report_flags, moderation_actions, updates, update_cursors, report_mutes, account_alert_settings, push_subscriptions, push_outbox, push_report_progress, official_alerts, official_alert_areas, official_feed_status, official_hazards, official_push_outbox, auth_codes, auth_throttles, comments, comment_likes, comment_flags, comment_moderation_actions, account_follows, person_blocks, profile_ratings, person_mutes RESTART IDENTITY CASCADE")
	MustExec(t, pool, "INSERT INTO official_feed_status (feed) VALUES ('sachet'), ('gdacs')")
}

// MustExec runs sql against pool and fails the test immediately on error, so
// store tests that need setup queries stay readable.
func MustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("executing %q: %v", sql, err)
	}
}
