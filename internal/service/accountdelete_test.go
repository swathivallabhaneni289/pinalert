package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// fakeDeleteQuerier records every call AccountDeletionService makes, in order.
type fakeDeleteQuerier struct {
	name string

	calls []string
	// emailRow is what AccountEmailForDelete returns.
	emailRow sqlcgen.AccountEmailForDeleteRow
	emailErr error
	// failAt names a call that returns boom.
	failAt string
	// scrubRows is how many rows ScrubDeletedAccount reports changed.
	scrubRows int64

	cooldown    string
	codesFor    string
	throttleArg sqlcgen.ClearAuthThrottleParams
	scrubArg    sqlcgen.ScrubDeletedAccountParams
	revokeArg   sqlcgen.RevokeAccountSessionsParams
	accountIDs  map[string]int64
}

var errBoom = errors.New("boom")

func newFakeDeleteQuerier(name string) *fakeDeleteQuerier {
	return &fakeDeleteQuerier{
		name:       name,
		emailRow:   sqlcgen.AccountEmailForDeleteRow{Email: "person@example.com"},
		scrubRows:  1,
		accountIDs: map[string]int64{},
	}
}

func (f *fakeDeleteQuerier) step(call string) error {
	f.calls = append(f.calls, call)
	if f.failAt == call {
		return errBoom
	}
	return nil
}

func (f *fakeDeleteQuerier) AccountEmailForDelete(ctx context.Context, id int64) (sqlcgen.AccountEmailForDeleteRow, error) {
	f.accountIDs["AccountEmailForDelete"] = id
	if err := f.step("AccountEmailForDelete"); err != nil {
		return sqlcgen.AccountEmailForDeleteRow{}, err
	}
	return f.emailRow, f.emailErr
}

func (f *fakeDeleteQuerier) DeleteEmailCooldownForEmail(ctx context.Context, email string) error {
	f.cooldown = email
	return f.step("DeleteEmailCooldownForEmail")
}

// Amended 2026-10-29 by Phase 7.1 plan 06 (T-07.1-22): the fake gained the two
// auth row deletes. codesFor and throttleArg record what they were given.
func (f *fakeDeleteQuerier) DeleteAuthCodesForEmail(ctx context.Context, email string) error {
	f.codesFor = email
	return f.step("DeleteAuthCodesForEmail")
}

func (f *fakeDeleteQuerier) ClearAuthThrottle(ctx context.Context, arg sqlcgen.ClearAuthThrottleParams) error {
	f.throttleArg = arg
	return f.step("ClearAuthThrottle")
}

func (f *fakeDeleteQuerier) DeleteAccountUpdates(ctx context.Context, accountID int64) error {
	f.accountIDs["DeleteAccountUpdates"] = accountID
	return f.step("DeleteAccountUpdates")
}

func (f *fakeDeleteQuerier) DeleteAccountCursors(ctx context.Context, accountID int64) error {
	f.accountIDs["DeleteAccountCursors"] = accountID
	return f.step("DeleteAccountCursors")
}

func (f *fakeDeleteQuerier) DeleteAccountMutes(ctx context.Context, accountID int64) error {
	f.accountIDs["DeleteAccountMutes"] = accountID
	return f.step("DeleteAccountMutes")
}

func (f *fakeDeleteQuerier) DeleteAccountAlertSettings(ctx context.Context, accountID int64) error {
	f.accountIDs["DeleteAccountAlertSettings"] = accountID
	return f.step("DeleteAccountAlertSettings")
}

func (f *fakeDeleteQuerier) DeleteAccountPushOutbox(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountPushOutbox"] = accountID
	return 0, f.step("DeleteAccountPushOutbox")
}

func (f *fakeDeleteQuerier) DeleteAccountOfficialPushOutbox(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountOfficialPushOutbox"] = accountID
	return 0, f.step("DeleteAccountOfficialPushOutbox")
}

func (f *fakeDeleteQuerier) DeleteAccountPushSubscriptions(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountPushSubscriptions"] = accountID
	return 0, f.step("DeleteAccountPushSubscriptions")
}

func (f *fakeDeleteQuerier) RevokeAccountSessions(ctx context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error) {
	f.revokeArg = arg
	return 2, f.step("RevokeAccountSessions")
}

func (f *fakeDeleteQuerier) ScrubDeletedAccount(ctx context.Context, arg sqlcgen.ScrubDeletedAccountParams) (int64, error) {
	f.scrubArg = arg
	if err := f.step("ScrubDeletedAccount"); err != nil {
		return 0, err
	}
	return f.scrubRows, nil
}

// wantDeleteOrder is the documented step order. Amended 2026-10-18 by Phase 3.3
// plan 10 (decision 11, UI-SPEC O-3): DeleteAccountPushOutbox and
// DeleteAccountPushSubscriptions now run after the four personal row deletes
// and before RevokeAccountSessions, so the nine steps became eleven.
//
// Amended 2026-10-29 by plan 05-11 (T-05-62): DeleteAccountOfficialPushOutbox
// now runs right after DeleteAccountPushOutbox, so the eleven steps became
// twelve. Every old step and its order is kept.
//
// Amended 2026-10-29 by Phase 7.1 plan 06 (T-07.1-22): DeleteAuthCodesForEmail
// now runs right after DeleteEmailCooldownForEmail, so the twelve steps became
// thirteen. ClearAuthThrottle runs right after it only when a throttle keyer is
// set (WithThrottleKeyer), so it is not in this list; wantDeleteOrderKeyed has
// it. Every old step and its order is kept.
//
// Amended 2026-10-29 by Phase 7.1 plan 16: the sign in link token delete step is
// removed with its table, so the thirteen steps became twelve (eleven plus
// ClearAuthThrottle in wantDeleteOrderKeyed). Every other step and its order is
// kept.
var wantDeleteOrder = []string{
	"AccountEmailForDelete",
	"DeleteEmailCooldownForEmail",
	"DeleteAuthCodesForEmail",
	"DeleteAccountUpdates",
	"DeleteAccountCursors",
	"DeleteAccountMutes",
	"DeleteAccountAlertSettings",
	"DeleteAccountPushOutbox",
	"DeleteAccountOfficialPushOutbox",
	"DeleteAccountPushSubscriptions",
	"RevokeAccountSessions",
	"ScrubDeletedAccount",
}

// wantDeleteOrderKeyed is the order with a throttle keyer set.
var wantDeleteOrderKeyed = []string{
	"AccountEmailForDelete",
	"DeleteEmailCooldownForEmail",
	"DeleteAuthCodesForEmail",
	"ClearAuthThrottle",
	"DeleteAccountUpdates",
	"DeleteAccountCursors",
	"DeleteAccountMutes",
	"DeleteAccountAlertSettings",
	"DeleteAccountPushOutbox",
	"DeleteAccountOfficialPushOutbox",
	"DeleteAccountPushSubscriptions",
	"RevokeAccountSessions",
	"ScrubDeletedAccount",
}

// TestDeleteAccountClearsTheThrottleWithAKeyer: with a keyer the sign in
// throttle row of the real address is cleared under scope sign_in and the
// keyer's hash, right after the code rows; the code rows get the real address.
func TestDeleteAccountClearsTheThrottleWithAKeyer(t *testing.T) {
	q := newFakeDeleteQuerier("plain")
	q.emailRow = sqlcgen.AccountEmailForDeleteRow{Email: "real@example.com"}
	keyer := func(email string) string { return "key:" + email }
	svc := service.NewAccountDeletionService(q, nil, service.WithThrottleKeyer(keyer))
	if err := svc.DeleteAccount(context.Background(), 42); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if !reflect.DeepEqual(q.calls, wantDeleteOrderKeyed) {
		t.Fatalf("calls =\n%v\nwant\n%v", q.calls, wantDeleteOrderKeyed)
	}
	if q.codesFor != "real@example.com" {
		t.Errorf("code rows deleted for %q, want the real address", q.codesFor)
	}
	want := sqlcgen.ClearAuthThrottleParams{Scope: "sign_in", KeyHash: "key:real@example.com"}
	if q.throttleArg != want {
		t.Errorf("ClearAuthThrottle arg = %+v, want %+v", q.throttleArg, want)
	}

	// Without a keyer the throttle is left alone.
	plain := newFakeDeleteQuerier("plain")
	if err := service.NewAccountDeletionService(plain, nil).DeleteAccount(context.Background(), 42); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	for _, c := range plain.calls {
		if c == "ClearAuthThrottle" {
			t.Errorf("ClearAuthThrottle ran without a keyer")
		}
	}
}

// TestDeleteAccountOrder: the thirteen steps run in the documented order, the
// scrub last, each keyed by the account id (or the real address) it needs.
func TestDeleteAccountOrder(t *testing.T) {
	fixed := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
	q := newFakeDeleteQuerier("plain")
	svc := service.NewAccountDeletionService(q, nil, service.WithAccountDeleteClock(func() time.Time { return fixed }))

	if err := svc.DeleteAccount(context.Background(), 42); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if !reflect.DeepEqual(q.calls, wantDeleteOrder) {
		t.Fatalf("calls =\n%v\nwant\n%v", q.calls, wantDeleteOrder)
	}
	for name, id := range q.accountIDs {
		if id != 42 {
			t.Errorf("%s was given account %d, want 42", name, id)
		}
	}
	if q.revokeArg.AccountID == nil || *q.revokeArg.AccountID != 42 {
		t.Errorf("RevokeAccountSessions account = %v, want 42", q.revokeArg.AccountID)
	}
	wantNow := pgtype.Timestamptz{Time: fixed, Valid: true}
	if q.revokeArg.Now != wantNow {
		t.Errorf("RevokeAccountSessions now = %+v, want %+v", q.revokeArg.Now, wantNow)
	}
	want := sqlcgen.ScrubDeletedAccountParams{ScrubbedEmail: "deleted-42@deleted.invalid", Now: wantNow, ID: 42}
	if q.scrubArg != want {
		t.Errorf("ScrubDeletedAccount arg = %+v, want %+v", q.scrubArg, want)
	}
}

// TestDeleteAccountReadsEmailFirst: the address read before the scrub is the
// one the token and cooldown deletes use, and an account already deleted
// changes nothing.
func TestDeleteAccountReadsEmailFirst(t *testing.T) {
	q := newFakeDeleteQuerier("plain")
	q.emailRow = sqlcgen.AccountEmailForDeleteRow{Email: "real@example.com"}
	svc := service.NewAccountDeletionService(q, nil)
	if err := svc.DeleteAccount(context.Background(), 9); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	// Amended 2026-10-29 by Phase 7.1 plan 16: the sign in link token delete is
	// gone with its table, so the code rows delete takes its place in this check.
	if q.codesFor != "real@example.com" || q.cooldown != "real@example.com" {
		t.Errorf("code rows delete used %q and cooldown delete used %q, want the real address for both", q.codesFor, q.cooldown)
	}

	done := newFakeDeleteQuerier("done")
	done.emailRow = sqlcgen.AccountEmailForDeleteRow{
		Email:     "deleted-9@deleted.invalid",
		DeletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	err := service.NewAccountDeletionService(done, nil).DeleteAccount(context.Background(), 9)
	if !errors.Is(err, service.ErrAccountAlreadyDeleted) {
		t.Fatalf("an already deleted account gave %v, want ErrAccountAlreadyDeleted", err)
	}
	if !reflect.DeepEqual(done.calls, []string{"AccountEmailForDelete"}) {
		t.Errorf("an already deleted account ran %v, want only the read", done.calls)
	}
}

// TestDeleteAccountScrubRaceIsAlreadyDeleted: a scrub that changes no row (a
// second delete won the race) is reported as already deleted.
func TestDeleteAccountScrubRaceIsAlreadyDeleted(t *testing.T) {
	q := newFakeDeleteQuerier("plain")
	q.scrubRows = 0
	err := service.NewAccountDeletionService(q, nil).DeleteAccount(context.Background(), 5)
	if !errors.Is(err, service.ErrAccountAlreadyDeleted) {
		t.Fatalf("a scrub of zero rows gave %v, want ErrAccountAlreadyDeleted", err)
	}
}

// TestDeleteAccountRunsInOneTx: with a tx func every step runs on the querier
// the tx func hands out and none on the plain one, the tx func runs once, and
// a failure at any step returns the wrapped error without the address.
func TestDeleteAccountRunsInOneTx(t *testing.T) {
	plain := newFakeDeleteQuerier("plain")
	inTx := newFakeDeleteQuerier("tx")
	ran := 0
	tx := func(ctx context.Context, fn func(q service.AccountDeleteQuerier) error) error {
		ran++
		return fn(inTx)
	}
	svc := service.NewAccountDeletionService(plain, tx)
	if err := svc.DeleteAccount(context.Background(), 3); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if ran != 1 {
		t.Errorf("the tx func ran %d times, want 1", ran)
	}
	if len(plain.calls) != 0 {
		t.Errorf("the plain querier was called %v, want nothing", plain.calls)
	}
	if !reflect.DeepEqual(inTx.calls, wantDeleteOrder) {
		t.Errorf("tx calls = %v, want %v", inTx.calls, wantDeleteOrder)
	}

	for i, failAt := range wantDeleteOrder {
		q := newFakeDeleteQuerier("tx")
		q.failAt = failAt
		failTx := func(ctx context.Context, fn func(q service.AccountDeleteQuerier) error) error { return fn(q) }
		err := service.NewAccountDeletionService(newFakeDeleteQuerier("plain"), failTx).DeleteAccount(context.Background(), 3)
		if !errors.Is(err, errBoom) {
			t.Fatalf("failure at %s: %v does not wrap the cause", failAt, err)
		}
		if !strings.Contains(err.Error(), "account 3") {
			t.Errorf("failure at %s: %q does not name the account id", failAt, err)
		}
		if strings.Contains(err.Error(), "person@example.com") {
			t.Errorf("failure at %s leaks the address: %q", failAt, err)
		}
		if got := q.calls; len(got) != i+1 {
			t.Errorf("failure at %s: %d steps ran, want %d (stop at the failure)", failAt, len(got), i+1)
		}
	}
}

// TestDeleteAccountTouchesNoOtherPersonsData: the querier interface the service
// uses has no method that could reach votes, reports, flags or moderation
// actions.
func TestDeleteAccountTouchesNoOtherPersonsData(t *testing.T) {
	typ := reflect.TypeOf((*service.AccountDeleteQuerier)(nil)).Elem()
	// Amended 2026-10-29 by Phase 7.1 plan 06: the interface also holds
	// ClearAuthThrottle, which runs only with a throttle keyer, so the method
	// set is the keyed order.
	if typ.NumMethod() != len(wantDeleteOrderKeyed) {
		t.Fatalf("AccountDeleteQuerier has %d methods, want %d", typ.NumMethod(), len(wantDeleteOrderKeyed))
	}
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i).Name
		found := false
		for _, w := range wantDeleteOrderKeyed {
			if w == m {
				found = true
			}
		}
		if !found {
			t.Errorf("unexpected method %s on AccountDeleteQuerier", m)
		}
	}
}

// TestDeletedEmailFor: the scrubbed address uses the reserved .invalid top
// level domain and the account id.
func TestDeletedEmailFor(t *testing.T) {
	if got := service.DeletedEmailFor(42); got != "deleted-42@deleted.invalid" {
		t.Errorf("DeletedEmailFor(42) = %q", got)
	}
	if service.DeletedEmailFor(1) == service.DeletedEmailFor(2) {
		t.Errorf("two accounts share a scrubbed address")
	}
}

// TestDeleteAccountRemovesOfficialPushRows (plan 05-11, T-05-62): on a real
// database an account with official push rows has none after DeleteAccount, the
// other account's rows stay, and the deletion still runs in one transaction (a
// failure at the scrub, simulated by deleting an already deleted account, leaves
// the rows of the account that was not deleted alone). It needs a database and
// skips without DATABASE_URL. The account's official push rows are also covered
// by the rollback check below: they are back when the transaction rolls back.
func TestDeleteAccountRemovesOfficialPushRows(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()

	a := testutil.SeedReporterAccount(t, pool, "official-del-a@example.com", "official-del-a-sess")
	b := testutil.SeedReporterAccount(t, pool, "official-del-b@example.com", "official-del-b-sess")
	alert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{Severity: "Severe", Tier: "district", DistrictCodes: []int{749}})
	testutil.MustExec(t, pool,
		`INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, expires_at)
		 VALUES ($1, $3, 'IN-del_1', 'new', now() + interval '1 hour'),
		        ($1, $3, 'IN-del_2', 'escalated', now() + interval '1 hour'),
		        ($2, $3, 'IN-del_1', 'new', now() + interval '1 hour')`, a, b, alert)
	count := func(account int64) int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM official_push_outbox WHERE account_id = $1`, account).Scan(&n); err != nil {
			t.Fatalf("counting rows: %v", err)
		}
		return n
	}

	// The deletion runs inside the one transaction: when that transaction is
	// rolled back after every step succeeded, the official push rows are back.
	errRollback := errors.New("roll back after the last step")
	rollback := func(ctx context.Context, fn func(q service.AccountDeleteQuerier) error) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if err := fn(sqlcgen.New(tx)); err != nil {
				return err
			}
			return errRollback
		})
	}
	if err := service.NewAccountDeletionService(sqlcgen.New(pool), rollback).DeleteAccount(ctx, a); !errors.Is(err, errRollback) {
		t.Fatalf("DeleteAccount in a transaction that rolls back gave %v, want the rollback error", err)
	}
	if n := count(a); n != 2 {
		t.Errorf("after a rolled back deletion the account has %d official push rows, want 2", n)
	}

	svc := service.NewAccountDeletionService(sqlcgen.New(pool), service.PgxAccountTx(pool))
	if err := svc.DeleteAccount(ctx, a); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if n := count(a); n != 0 {
		t.Errorf("the deleted account still has %d official push rows", n)
	}
	if n := count(b); n != 1 {
		t.Errorf("the other account has %d official push rows, want 1", n)
	}
	// A second delete of the same account fails before it changes anything, and
	// the other account's row is still there.
	if err := svc.DeleteAccount(ctx, a); !errors.Is(err, service.ErrAccountAlreadyDeleted) {
		t.Errorf("a second delete gave %v, want ErrAccountAlreadyDeleted", err)
	}
	if n := count(b); n != 1 {
		t.Errorf("the other account has %d official push rows after the failed delete, want 1", n)
	}
}

// TestDeleteAccountRemovesAuthRows (Phase 7.1 plan 06, T-07.1-22, research
// Pitfall 12): on a real database an account with a code row, a sign in
// throttle row, a password and a username has none of them after DeleteAccount,
// the username is free for another account to take, and another address's code
// and throttle rows stay. It needs a database and skips without DATABASE_URL.
func TestDeleteAccountRemovesAuthRows(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	const (
		addrA = "auth-del-a@example.com"
		addrB = "auth-del-b@example.com"
	)
	a := testutil.SeedReporterAccount(t, pool, addrA, "auth-del-a-sess")
	b := testutil.SeedReporterAccount(t, pool, addrB, "auth-del-b-sess")
	testutil.MustExec(t, pool, `UPDATE accounts SET password_hash = '$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA',
		username = 'gone.soon', username_screen = 'clear' WHERE id = $1`, a)

	// The table requires 64 hex characters; a plain digest stands in for the
	// sign in service's keyed hash.
	keyer := func(email string) string {
		sum := sha256.Sum256([]byte(email))
		return hex.EncodeToString(sum[:])
	}
	hash := strings.Repeat("0", 64)
	for _, addr := range []string{addrA, addrB} {
		if _, err := q.IssueAuthCode(ctx, sqlcgen.IssueAuthCodeParams{
			SessionID: "sess-" + addr, Purpose: "reset", Delivery: "code", Email: addr,
			CodeHash: hash, ExpiresAt: time.Now().Add(10 * time.Minute),
		}); err != nil {
			t.Fatalf("IssueAuthCode %s: %v", addr, err)
		}
		if _, err := q.RecordAuthFailure(ctx, sqlcgen.RecordAuthFailureParams{
			Scope: "sign_in", KeyHash: keyer(addr), Now: time.Now(), ForgetCutoff: time.Now().Add(-time.Hour),
			FreeFailures: 5, BaseLockSeconds: 60, MaxLockSeconds: 3600,
		}); err != nil {
			t.Fatalf("RecordAuthFailure %s: %v", addr, err)
		}
	}
	count := func(query string, args ...any) int {
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	codes := func(addr string) int { return count(`SELECT count(*) FROM auth_codes WHERE email = $1`, addr) }
	throttles := func(addr string) int {
		return count(`SELECT count(*) FROM auth_throttles WHERE key_hash = $1`, keyer(addr))
	}
	if codes(addrA) != 1 || throttles(addrA) != 1 {
		t.Fatalf("seed failed: %d code rows and %d throttle rows for A", codes(addrA), throttles(addrA))
	}

	svc := service.NewAccountDeletionService(q, service.PgxAccountTx(pool), service.WithThrottleKeyer(keyer))
	if err := svc.DeleteAccount(ctx, a); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if n := codes(addrA); n != 0 {
		t.Errorf("the deleted account still has %d code rows", n)
	}
	if n := throttles(addrA); n != 0 {
		t.Errorf("the deleted account still has %d throttle rows", n)
	}
	if codes(addrB) != 1 || throttles(addrB) != 1 {
		t.Errorf("the other address lost rows: %d code rows, %d throttle rows, want 1 and 1", codes(addrB), throttles(addrB))
	}
	var passwordNull, usernameNull bool
	if err := pool.QueryRow(ctx, `SELECT password_hash IS NULL, username IS NULL AND username_screen IS NULL AND username_changed_at IS NULL
		FROM accounts WHERE id = $1`, a).Scan(&passwordNull, &usernameNull); err != nil || !passwordNull || !usernameNull {
		t.Errorf("password hash NULL = %v, username fields NULL = %v (%v), want both true", passwordNull, usernameNull, err)
	}

	// The name is free again: another account can take it.
	accounts := service.NewAccountService(q)
	if _, err := accounts.UpdateProfile(ctx, b, service.ProfileUpdate{Username: strPtr("gone.soon")}); err != nil {
		t.Errorf("the freed username could not be taken: %v", err)
	}
}

// TestDeleteConfirmMessage: the wording is fixed and has no dash.
func TestDeleteConfirmMessage(t *testing.T) {
	if service.DeleteConfirmMessage != "Confirm to delete your account." {
		t.Errorf("DeleteConfirmMessage = %q", service.DeleteConfirmMessage)
	}
}
