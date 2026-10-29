package store_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// argonPlaceholder passes the accounts_password_hash_shape CHECK, which tests
// only the prefix. It is not a real hash.
const argonPlaceholder = "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA"

func newAccount(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), `INSERT INTO accounts (email) VALUES ($1) RETURNING id`, email).Scan(&id); err != nil {
		t.Fatalf("inserting account %s: %v", email, err)
	}
	return id
}

func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func setUsername(t *testing.T, q *sqlcgen.Queries, id int64, name string, now, waitCutoff time.Time) (int64, error) {
	t.Helper()
	return q.SetAccountUsername(context.Background(), sqlcgen.SetAccountUsernameParams{
		ID: id, Username: name, UsernameScreen: "clear", Now: now, WaitCutoff: waitCutoff,
	})
}

// TestUsernameShapeAndUnique proves the shape CHECK and the unique constraint
// through the query that writes a username.
func TestUsernameShapeAndUnique(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	first := newAccount(t, pool, "first@example.com")
	second := newAccount(t, pool, "second@example.com")

	for _, bad := range []string{"Asha", ".asha", "asha.", "as..ha", "123", "ab", "asha-k", "a2345678901234567890x", "asha k"} {
		t.Run("shape "+bad, func(t *testing.T) {
			_, err := setUsername(t, q, first, bad, now, now.Add(-7*24*time.Hour))
			if got := sqlState(err); got != "23514" {
				t.Fatalf("username %q: SQLSTATE %q (%v), want 23514", bad, got, err)
			}
		})
	}
	t.Run("a good name is stored", func(t *testing.T) {
		n, err := setUsername(t, q, first, "asha_k.1", now, now.Add(-7*24*time.Hour))
		if err != nil || n != 1 {
			t.Fatalf("set = %d (%v), want 1", n, err)
		}
		id, err := q.GetAccountIDByUsername(context.Background(), "asha_k.1")
		if err != nil || id != first {
			t.Fatalf("GetAccountIDByUsername = %d (%v), want %d", id, err, first)
		}
	})
	t.Run("the same name on a second account is 23505", func(t *testing.T) {
		_, err := setUsername(t, q, second, "asha_k.1", now, now.Add(-7*24*time.Hour))
		if got := sqlState(err); got != "23505" {
			t.Fatalf("SQLSTATE %q (%v), want 23505", got, err)
		}
	})
	t.Run("an unknown name finds no account", func(t *testing.T) {
		if _, err := q.GetAccountIDByUsername(context.Background(), "nobody_here"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("err = %v, want pgx.ErrNoRows", err)
		}
	})
}

// TestSetAccountUsernameRenameGap proves the first set is free, one change
// follows at once, a second change waits 7 days, and the same name again is
// always allowed without moving the wait.
func TestSetAccountUsernameRenameGap(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	id := newAccount(t, pool, "rename@example.com")
	day := 24 * time.Hour
	t0 := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	wait := 7 * day

	changedAt := func() (string, *time.Time) {
		t.Helper()
		p, err := q.GetAccountProfile(ctx, id)
		if err != nil {
			t.Fatalf("GetAccountProfile: %v", err)
		}
		name := ""
		if p.Username != nil {
			name = *p.Username
		}
		if !p.UsernameChangedAt.Valid {
			return name, nil
		}
		at := p.UsernameChangedAt.Time
		return name, &at
	}

	if n, err := setUsername(t, q, id, "first_name", t0, t0.Add(-wait)); err != nil || n != 1 {
		t.Fatalf("first set = %d (%v), want 1", n, err)
	}
	if name, at := changedAt(); name != "first_name" || at != nil {
		t.Fatalf("after the first set: %q %v, want first_name and no change time", name, at)
	}

	t1 := t0.Add(time.Hour)
	if n, err := setUsername(t, q, id, "second_name", t1, t1.Add(-wait)); err != nil || n != 1 {
		t.Fatalf("first change = %d (%v), want 1 (changed_at was NULL)", n, err)
	}
	if name, at := changedAt(); name != "second_name" || at == nil || !at.Equal(t1) {
		t.Fatalf("after the first change: %q %v, want second_name at %v", name, at, t1)
	}

	t2 := t0.Add(2 * day)
	if n, err := setUsername(t, q, id, "third_name", t2, t2.Add(-wait)); err != nil || n != 0 {
		t.Fatalf("second change too soon = %d (%v), want 0 rows", n, err)
	}
	if name, _ := changedAt(); name != "second_name" {
		t.Fatalf("a refused change must not write, name = %q", name)
	}

	t3 := t0.Add(8 * day)
	if n, err := setUsername(t, q, id, "third_name", t3, t3.Add(-wait)); err != nil || n != 1 {
		t.Fatalf("change after the wait = %d (%v), want 1", n, err)
	}
	if name, at := changedAt(); name != "third_name" || at == nil || !at.Equal(t3) {
		t.Fatalf("after the third name: %q %v, want third_name at %v", name, at, t3)
	}

	t4 := t3.Add(time.Hour)
	if n, err := setUsername(t, q, id, "third_name", t4, t4.Add(-wait)); err != nil || n != 1 {
		t.Fatalf("the same name again = %d (%v), want 1", n, err)
	}
	if _, at := changedAt(); at == nil || !at.Equal(t3) {
		t.Fatalf("the same name must not move the wait, changed_at = %v, want %v", at, t3)
	}
}

// TestUsernamesByAccountIDs proves the batched read returns only accounts that
// have a username and are not deleted.
func TestUsernamesByAccountIDs(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	cut := now.Add(-7 * 24 * time.Hour)

	named := newAccount(t, pool, "named@example.com")
	pending := newAccount(t, pool, "pending@example.com")
	unnamed := newAccount(t, pool, "unnamed@example.com")
	deleted := newAccount(t, pool, "deleted@example.com")
	notAsked := newAccount(t, pool, "not-asked@example.com")

	if _, err := setUsername(t, q, named, "named_one", now, cut); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := q.SetAccountUsername(ctx, sqlcgen.SetAccountUsernameParams{
		ID: pending, Username: "pending_one", UsernameScreen: "pending", Now: now, WaitCutoff: cut,
	}); err != nil {
		t.Fatalf("set pending: %v", err)
	}
	if _, err := setUsername(t, q, deleted, "deleted_one", now, cut); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := setUsername(t, q, notAsked, "other_one", now, cut); err != nil {
		t.Fatalf("set: %v", err)
	}
	testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, deleted)

	rows, err := q.UsernamesByAccountIDs(ctx, []int64{named, pending, unnamed, deleted})
	if err != nil {
		t.Fatalf("UsernamesByAccountIDs: %v", err)
	}
	var got []string
	for _, r := range rows {
		if r.Username == nil || r.UsernameScreen == nil {
			t.Fatalf("row %+v has a NULL username or screen", r)
		}
		got = append(got, *r.Username+":"+*r.UsernameScreen)
	}
	sort.Strings(got)
	want := []string{"named_one:clear", "pending_one:pending"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("rows = %v, want %v (no unnamed, no deleted, none outside the ids)", got, want)
	}

	if empty, err := q.UsernamesByAccountIDs(ctx, nil); err != nil || len(empty) != 0 {
		t.Fatalf("no ids = %v (%v), want none", empty, err)
	}
}

// TestAccountPresenceFields proves the four presence fields the gate reads
// follow their columns.
func TestAccountPresenceFields(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	id := newAccount(t, pool, "presence@example.com")
	if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "presence-session", AccountID: &id}); err != nil {
		t.Fatalf("BindSessionAccount: %v", err)
	}

	read := func() sqlcgen.GetAccountBySessionIDRow {
		t.Helper()
		row, err := q.GetAccountBySessionID(ctx, "presence-session")
		if err != nil {
			t.Fatalf("GetAccountBySessionID: %v", err)
		}
		return row
	}

	if r := read(); r.HasPassword || r.HasUsername || r.HasName || r.HasHomeArea {
		t.Fatalf("a bare account = %+v, want all four false", r)
	}

	hash := argonPlaceholder
	if n, err := q.SetAccountPassword(ctx, sqlcgen.SetAccountPasswordParams{ID: id, PasswordHash: hash}); err != nil || n != 1 {
		t.Fatalf("SetAccountPassword = %d (%v), want 1", n, err)
	}
	if r := read(); !r.HasPassword || r.HasUsername || r.HasName || r.HasHomeArea {
		t.Fatalf("after a password = %+v, want only has_password", r)
	}

	if _, err := setUsername(t, q, id, "presence_one", now, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatalf("set username: %v", err)
	}
	if r := read(); !r.HasPassword || !r.HasUsername || r.HasName || r.HasHomeArea {
		t.Fatalf("after a username = %+v", r)
	}

	name := "Presence"
	if err := q.UpdateAccountProfile(ctx, sqlcgen.UpdateAccountProfileParams{ID: id, SetName: true, DisplayName: &name}); err != nil {
		t.Fatalf("UpdateAccountProfile name: %v", err)
	}
	if r := read(); !r.HasName || r.HasHomeArea {
		t.Fatalf("after a name = %+v, want has_name only of the two", r)
	}

	area, cell := "Benz Circle, Vijayawada", "tdr1w"
	if err := q.UpdateAccountProfile(ctx, sqlcgen.UpdateAccountProfileParams{ID: id, SetHomeArea: true, HomeAreaName: &area, HomeAreaCell: &cell}); err != nil {
		t.Fatalf("UpdateAccountProfile area: %v", err)
	}
	if r := read(); !r.HasPassword || !r.HasUsername || !r.HasName || !r.HasHomeArea {
		t.Fatalf("a finished account = %+v, want all four true", r)
	}

	t.Run("sign in read follows the same columns", func(t *testing.T) {
		row, err := q.GetAccountForSignIn(ctx, "presence@example.com")
		if err != nil || row.ID != id || row.PasswordHash == nil || *row.PasswordHash != hash || !row.HasName {
			t.Fatalf("GetAccountForSignIn = %+v (%v)", row, err)
		}
		if _, err := q.GetAccountForSignIn(ctx, "nobody@example.com"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("unknown address err = %v, want pgx.ErrNoRows", err)
		}
	})
	t.Run("a deleted account reads as no session and no sign in", func(t *testing.T) {
		testutil.MustExec(t, pool, `UPDATE accounts SET deleted_at = now() WHERE id = $1`, id)
		if _, err := q.GetAccountBySessionID(ctx, "presence-session"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("session read err = %v, want pgx.ErrNoRows", err)
		}
		if _, err := q.GetAccountForSignIn(ctx, "presence@example.com"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("sign in read err = %v, want pgx.ErrNoRows", err)
		}
		if n, err := q.SetAccountPassword(ctx, sqlcgen.SetAccountPasswordParams{ID: id, PasswordHash: hash}); err != nil || n != 0 {
			t.Fatalf("SetAccountPassword on a deleted account = %d (%v), want 0", n, err)
		}
	})
}

// TestSessionEndedByPasswordSwitch proves the ended-session read: true for a
// live or revoked session of a password-less, not deleted account, false
// otherwise.
func TestSessionEndedByPasswordSwitch(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	id := newAccount(t, pool, "switch@example.com")
	for _, s := range []string{"switch-live", "switch-revoked"} {
		if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: s, AccountID: &id}); err != nil {
			t.Fatalf("BindSessionAccount %s: %v", s, err)
		}
	}
	testutil.MustExec(t, pool, `UPDATE sessions SET revoked_at = now() WHERE session_id = 'switch-revoked'`)
	if err := q.UpsertSession(ctx, "switch-anon"); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	ended := func(session string) bool {
		t.Helper()
		v, err := q.SessionEndedByPasswordSwitch(ctx, session)
		if err != nil {
			t.Fatalf("SessionEndedByPasswordSwitch(%s): %v", session, err)
		}
		return v
	}

	t.Run("a password-less account: live and revoked both true", func(t *testing.T) {
		if !ended("switch-live") || !ended("switch-revoked") {
			t.Fatal("want true for both sessions of a password-less account")
		}
	})
	t.Run("an unbound or unknown session is false", func(t *testing.T) {
		if ended("switch-anon") || ended("switch-unknown") {
			t.Fatal("want false for a session with no account")
		}
	})
	t.Run("once the account has a password it is false", func(t *testing.T) {
		testutil.MustExec(t, pool, `UPDATE accounts SET password_hash = $2 WHERE id = $1`, id, argonPlaceholder)
		if ended("switch-live") || ended("switch-revoked") {
			t.Fatal("want false after a password is set")
		}
	})
	t.Run("a deleted account is false", func(t *testing.T) {
		testutil.MustExec(t, pool, `UPDATE accounts SET password_hash = NULL, deleted_at = now() WHERE id = $1`, id)
		if ended("switch-live") {
			t.Fatal("want false for a deleted account")
		}
	})
}

// TestRevokeSession proves RevokeSession ends one live session, keeps its
// account binding and leaves an already revoked session unchanged.
func TestRevokeSession(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	id := newAccount(t, pool, "revoke@example.com")
	for _, s := range []string{"revoke-a", "revoke-b"} {
		if _, err := q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: s, AccountID: &id}); err != nil {
			t.Fatalf("BindSessionAccount %s: %v", s, err)
		}
	}
	first := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	if n, err := q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{SessionID: "revoke-a", Now: first}); err != nil || n != 1 {
		t.Fatalf("RevokeSession = %d (%v), want 1", n, err)
	}
	t.Run("the account binding stays", func(t *testing.T) {
		var bound int64
		if err := pool.QueryRow(ctx, `SELECT account_id FROM sessions WHERE session_id = 'revoke-a'`).Scan(&bound); err != nil || bound != id {
			t.Fatalf("account_id = %d (%v), want %d", bound, err, id)
		}
	})
	t.Run("a second revoke changes nothing", func(t *testing.T) {
		if n, err := q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{SessionID: "revoke-a", Now: first.Add(time.Hour)}); err != nil || n != 0 {
			t.Fatalf("RevokeSession again = %d (%v), want 0", n, err)
		}
		var kept bool
		if err := pool.QueryRow(ctx, `SELECT revoked_at = $1 FROM sessions WHERE session_id = 'revoke-a'`, first).Scan(&kept); err != nil || !kept {
			t.Fatalf("revoked_at kept = %v (%v), want the first time", kept, err)
		}
	})
	t.Run("the other session of the account stays live", func(t *testing.T) {
		if _, err := q.GetAccountBySessionID(ctx, "revoke-b"); err != nil {
			t.Fatalf("the other session must still read as an account: %v", err)
		}
		if _, err := q.GetAccountBySessionID(ctx, "revoke-a"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("the revoked session err = %v, want pgx.ErrNoRows", err)
		}
	})
	t.Run("an unknown session is zero rows", func(t *testing.T) {
		if n, err := q.RevokeSession(ctx, sqlcgen.RevokeSessionParams{SessionID: "revoke-none", Now: first}); err != nil || n != 0 {
			t.Fatalf("RevokeSession unknown = %d (%v), want 0", n, err)
		}
	})
}

// TestScrubClearsPasswordAndUsername proves Delete my account clears the
// password and frees the username for another account.
func TestScrubClearsPasswordAndUsername(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	id := newAccount(t, pool, "scrub@example.com")
	other := newAccount(t, pool, "taker@example.com")

	if n, err := q.SetAccountPassword(ctx, sqlcgen.SetAccountPasswordParams{ID: id, PasswordHash: argonPlaceholder}); err != nil || n != 1 {
		t.Fatalf("SetAccountPassword = %d (%v)", n, err)
	}
	if _, err := setUsername(t, q, id, "scrub_me", now, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatalf("set username: %v", err)
	}
	testutil.MustExec(t, pool, `UPDATE accounts SET username_changed_at = $2 WHERE id = $1`, id, now)

	if _, err := setUsername(t, q, other, "scrub_me", now, now.Add(-7*24*time.Hour)); sqlState(err) != "23505" {
		t.Fatalf("before the scrub the name is taken, err = %v", err)
	}

	n, err := q.ScrubDeletedAccount(ctx, sqlcgen.ScrubDeletedAccountParams{
		ID: id, ScrubbedEmail: "deleted-1@deleted.invalid",
		Now: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil || n != 1 {
		t.Fatalf("ScrubDeletedAccount = %d (%v), want 1", n, err)
	}

	var nulls bool
	if err := pool.QueryRow(ctx, `SELECT password_hash IS NULL AND username IS NULL AND username_screen IS NULL AND username_changed_at IS NULL
		FROM accounts WHERE id = $1`, id).Scan(&nulls); err != nil || !nulls {
		t.Fatalf("scrubbed columns all NULL = %v (%v), want true", nulls, err)
	}
	if got, err := setUsername(t, q, other, "scrub_me", now, now.Add(-7*24*time.Hour)); err != nil || got != 1 {
		t.Fatalf("the freed name must be takeable: %d (%v)", got, err)
	}
}
