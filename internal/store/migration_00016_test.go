package store_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver, used by goose
	"github.com/pressly/goose/v3"

	"pinalert/internal/store"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestMigration00016 covers the password epoch migration of the Phase 7.1
// review (CR-S02). The text part runs without a database: Up adds the two
// columns and Down drops them. With DATABASE_URL it proves the promise the
// migration makes: a live session of an account that already has a password
// stays signed in after Up (only 00014 signed people out), a session whose
// epoch differs from its account's is refused by the gate query, and Down then
// Up again work.
func TestMigration00016(t *testing.T) {
	const file = "migrations/00016_add_password_epoch.sql"
	contents, err := store.MigrationsFS.ReadFile(file)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", file, err)
	}
	text := string(contents)
	up := strings.Index(text, "+goose Up")
	down := strings.Index(text, "+goose Down")
	if up < 0 || down < 0 || down < up {
		t.Fatalf("%s must carry +goose Up then +goose Down", file)
	}
	for _, want := range []string{
		"ALTER TABLE accounts\n    ADD COLUMN password_epoch BIGINT NOT NULL DEFAULT 0;",
		"ALTER TABLE sessions\n    ADD COLUMN password_epoch BIGINT NOT NULL DEFAULT 0;",
	} {
		if !strings.Contains(text[up:down], want) {
			t.Errorf("migration 00016 Up must contain %q", want)
		}
	}
	for _, want := range []string{"DROP COLUMN password_epoch;"} {
		if strings.Count(text[down:], want) != 2 {
			t.Errorf("migration 00016 Down must contain %q twice", want)
		}
	}
	if strings.Contains(text[up:down], "UPDATE sessions") {
		t.Error("migration 00016 must not revoke or change any session")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping the database part of the 00016 migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	goose.SetBaseFS(store.MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up: %v", err)
	}
	t.Cleanup(func() {
		_ = goose.Up(db, "migrations")
		_, _ = db.Exec(`DELETE FROM sessions WHERE session_id LIKE 'm16-%'`)
		_, _ = db.Exec(`DELETE FROM accounts WHERE email LIKE 'm16-%@example.com'`)
		_ = db.Close()
	})
	if err := goose.DownTo(db, "migrations", 15); err != nil {
		t.Fatalf("goose down to 15: %v", err)
	}
	columns := func() int {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name IN ('accounts', 'sessions')
			AND column_name = 'password_epoch'`).Scan(&n); err != nil {
			t.Fatalf("counting columns: %v", err)
		}
		return n
	}
	if n := columns(); n != 0 {
		t.Fatalf("after Down: %d password_epoch columns present, want 0", n)
	}

	var withPassword, other int64
	if err := db.QueryRow(`INSERT INTO accounts (email, password_hash)
		VALUES ('m16-a@example.com', '$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5') RETURNING id`).Scan(&withPassword); err != nil {
		t.Fatalf("inserting an account with a password: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO accounts (email, password_hash)
		VALUES ('m16-b@example.com', '$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5') RETURNING id`).Scan(&other); err != nil {
		t.Fatalf("inserting a second account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (session_id, account_id) VALUES ('m16-live', $1), ('m16-other', $2)`, withPassword, other); err != nil {
		t.Fatalf("inserting live sessions: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		t.Fatalf("goose up again: %v", err)
	}
	if n := columns(); n != 2 {
		t.Fatalf("after Up: %d password_epoch columns present, want 2", n)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer conn.Close(ctx)
	q := sqlcgen.New(conn)
	row, err := q.GetAccountBySessionID(ctx, "m16-live")
	if err != nil || row.ID != withPassword || !row.HasPassword {
		t.Fatalf("a live session of an account with a password must stay signed in after 00016: row %+v err %v", row, err)
	}

	// The gate compares the two epochs: moving the account's epoch ends its
	// session, and leaves every other account's session alone.
	if _, err := conn.Exec(ctx, `UPDATE accounts SET password_epoch = password_epoch + 1 WHERE id = $1`, withPassword); err != nil {
		t.Fatalf("moving the epoch: %v", err)
	}
	if _, err := q.GetAccountBySessionID(ctx, "m16-live"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a session on an old epoch must read as signed out, err = %v", err)
	}
	if _, err := q.GetAccountBySessionID(ctx, "m16-other"); err != nil {
		t.Errorf("another account's session must stay signed in: %v", err)
	}
}
