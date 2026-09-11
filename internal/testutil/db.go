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

// Truncate resets every identity and report table (accounts / magic_link_tokens
// / reports / sessions) to empty, restarting identity sequences, so tests
// that need a clean slate between sub-cases don't have to open a new pool
// via NewTestDB.
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	MustExec(t, pool, "TRUNCATE accounts, magic_link_tokens, reports, sessions RESTART IDENTITY CASCADE")
}

// MustExec runs sql against pool and fails the test immediately on error, so
// store tests that need setup queries stay readable.
func MustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("executing %q: %v", sql, err)
	}
}
