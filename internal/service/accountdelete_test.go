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
	blankArg    sqlcgen.BlankAccountCommentsParams
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

// Amended 2026-10-29 by Phase 8 plan 08 (A-P8-6): the fake gained the three
// comment steps. blankArg records what BlankAccountComments was given.
func (f *fakeDeleteQuerier) BlankAccountComments(ctx context.Context, arg sqlcgen.BlankAccountCommentsParams) (int64, error) {
	f.blankArg = arg
	f.accountIDs["BlankAccountComments"] = arg.AccountID
	return 2, f.step("BlankAccountComments")
}

func (f *fakeDeleteQuerier) DeleteAccountCommentLikes(ctx context.Context, accountID int64) error {
	f.accountIDs["DeleteAccountCommentLikes"] = accountID
	return f.step("DeleteAccountCommentLikes")
}

func (f *fakeDeleteQuerier) DeleteAccountCommentFlags(ctx context.Context, accountID int64) error {
	f.accountIDs["DeleteAccountCommentFlags"] = accountID
	return f.step("DeleteAccountCommentFlags")
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

// Amended 2026-10-29 by Phase 9 plan 02 (research G6): the fake gained the two
// social graph deletes.
func (f *fakeDeleteQuerier) DeleteAccountFollows(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountFollows"] = accountID
	return 0, f.step("DeleteAccountFollows")
}

// Amended 2026-10-30 by review IN-E06: the fake gained the queued follow notes
// delete, which runs just before DeleteAccountFollows.
func (f *fakeDeleteQuerier) DeleteQueuedFollowPushesForFollowee(ctx context.Context, followeeID int64) (int64, error) {
	f.accountIDs["DeleteQueuedFollowPushesForFollowee"] = followeeID
	return 0, f.step("DeleteQueuedFollowPushesForFollowee")
}

func (f *fakeDeleteQuerier) DeleteAccountBlocks(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountBlocks"] = accountID
	return 0, f.step("DeleteAccountBlocks")
}

// Amended 2026-10-29 by Phase 9 plan 13 (D-02, D-21, G6): the fake gained the
// ratings delete.
func (f *fakeDeleteQuerier) DeleteAccountRatings(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountRatings"] = accountID
	return 0, f.step("DeleteAccountRatings")
}

// Amended 2026-10-29 by Phase 9 plan 18 (D-12, G6): the fake gained the person
// mutes delete.
func (f *fakeDeleteQuerier) DeleteAccountPersonMutes(ctx context.Context, accountID int64) (int64, error) {
	f.accountIDs["DeleteAccountPersonMutes"] = accountID
	return 0, f.step("DeleteAccountPersonMutes")
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
//
// Amended 2026-10-29 by Phase 8 plan 08 (D-02, A-P8-6): BlankAccountComments,
// DeleteAccountCommentLikes and DeleteAccountCommentFlags now run right after
// the Phase 7.1 steps and before DeleteAccountUpdates, so the twelve steps
// became fifteen (and thirteen became sixteen in wantDeleteOrderKeyed). Every
// old step and its order is kept.
//
// Amended 2026-10-29 by Phase 9 plan 02 (research G6, review M5):
// DeleteAccountFollows and DeleteAccountBlocks now run after every Phase 5, 7.1
// and 8 step, right after DeleteAccountPushSubscriptions and just before
// RevokeAccountSessions, so the fifteen steps became seventeen (and sixteen
// became eighteen in wantDeleteOrderKeyed). Every old step and its order is
// kept. The plan text said the list ends with DeleteAccountCommentFlags then the
// two new steps; DeleteAccountCommentFlags is an early Phase 8 step with the
// updates, cursors, mutes and push steps after it, so those are not reordered.
//
// Amended 2026-10-29 by Phase 9 plan 13 (D-02, D-21, G6, review M5):
// DeleteAccountRatings now runs right after DeleteAccountBlocks and just before
// RevokeAccountSessions, so the seventeen steps became eighteen (and eighteen
// became nineteen in wantDeleteOrderKeyed). Every old step and its order is
// kept. For the same reason as plan 09-02 the list does not end with
// DeleteAccountCommentFlags: it ends with the Phase 9 steps Follows, Blocks,
// Ratings, then RevokeAccountSessions and ScrubDeletedAccount.
//
// Amended 2026-10-29 by Phase 9 plan 18 (D-12, G6, review M5):
// DeleteAccountPersonMutes now runs right after DeleteAccountRatings and just
// before RevokeAccountSessions, so the eighteen steps became nineteen (and
// nineteen became twenty in wantDeleteOrderKeyed). Every old step and its order
// is kept. The list now ends with the Phase 9 steps Follows, Blocks, Ratings,
// PersonMutes, then RevokeAccountSessions and ScrubDeletedAccount.
//
// Amended 2026-10-30 by review IN-E06: DeleteQueuedFollowPushesForFollowee now
// runs right before DeleteAccountFollows, so the twenty steps became twenty one
// (and twenty became twenty one in wantDeleteOrderKeyed too). Every old step and
// its order is kept.
var wantDeleteOrder = []string{
	"AccountEmailForDelete",
	"DeleteEmailCooldownForEmail",
	"DeleteAuthCodesForEmail",
	"BlankAccountComments",
	"DeleteAccountCommentLikes",
	"DeleteAccountCommentFlags",
	"DeleteAccountUpdates",
	"DeleteAccountCursors",
	"DeleteAccountMutes",
	"DeleteAccountAlertSettings",
	"DeleteAccountPushOutbox",
	"DeleteAccountOfficialPushOutbox",
	"DeleteAccountPushSubscriptions",
	"DeleteQueuedFollowPushesForFollowee",
	"DeleteAccountFollows",
	"DeleteAccountBlocks",
	"DeleteAccountRatings",
	"DeleteAccountPersonMutes",
	"RevokeAccountSessions",
	"ScrubDeletedAccount",
}

// wantDeleteOrderKeyed is the order with a throttle keyer set.
var wantDeleteOrderKeyed = []string{
	"AccountEmailForDelete",
	"DeleteEmailCooldownForEmail",
	"DeleteAuthCodesForEmail",
	"ClearAuthThrottle",
	"BlankAccountComments",
	"DeleteAccountCommentLikes",
	"DeleteAccountCommentFlags",
	"DeleteAccountUpdates",
	"DeleteAccountCursors",
	"DeleteAccountMutes",
	"DeleteAccountAlertSettings",
	"DeleteAccountPushOutbox",
	"DeleteAccountOfficialPushOutbox",
	"DeleteAccountPushSubscriptions",
	"DeleteQueuedFollowPushesForFollowee",
	"DeleteAccountFollows",
	"DeleteAccountBlocks",
	"DeleteAccountRatings",
	"DeleteAccountPersonMutes",
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
	// Amended 2026-10-29 by Phase 8 plan 08 (A-P8-6): the comment blanking gets
	// the account and the same clock.
	if wantBlank := (sqlcgen.BlankAccountCommentsParams{Now: wantNow, AccountID: 42}); q.blankArg != wantBlank {
		t.Errorf("BlankAccountComments arg = %+v, want %+v", q.blankArg, wantBlank)
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
//
// Amended 2026-10-29 by Phase 9 plan 02 (D-12, D-21): every other assertion is
// kept. DeleteAccountFollows and DeleteAccountBlocks are the named exception:
// they delete follow and block rows that name the person in both directions, so
// a follower's following list and a followee's follower list each lose the row.
// They touch no vote, report, flag or moderation action.
//
// Amended 2026-10-29 by Phase 9 plan 13 (D-02, G6): DeleteAccountRatings is a
// second named exception of the same kind: it deletes the rating rows that name
// the person as rater or reporter. It touches no vote, report, flag or
// moderation action.
//
// Amended 2026-10-29 by Phase 9 plan 18 (D-12, G6): DeleteAccountPersonMutes is
// a third named exception of the same kind: it deletes the person_mutes rows
// that name the person as muter or muted. It touches no vote, report, flag or
// moderation action.
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

// failingScrubQuerier is a real querier whose scrub step fails, so a test can
// see every earlier step roll back with the transaction.
type failingScrubQuerier struct{ *sqlcgen.Queries }

func (failingScrubQuerier) ScrubDeletedAccount(context.Context, sqlcgen.ScrubDeletedAccountParams) (int64, error) {
	return 0, errBoom
}

// TestDeleteAccountComments (Phase 8 plan 08, A-P8-6, T-08-36): deleting an
// account blanks its comment words and marks them deleted, removes the likes it
// gave and its comment flags, keeps the other person's reply and likes on its
// comments, and does all of it inside the one transaction. It needs a database
// and skips without DATABASE_URL.
func TestDeleteAccountComments(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	a := testutil.SeedReporterAccount(t, pool, "comment-del-a@example.com", "comment-del-a-sess")
	b := testutil.SeedReporterAccount(t, pool, "comment-del-b@example.com", "comment-del-b-sess")
	testutil.SeedUsername(t, pool, a, "going_away", "clear")
	testutil.SeedUsername(t, pool, b, "staying_on", "clear")
	report := testutil.SeedHistoryReport(t, pool, "comment-del-b-sess", "low", "flood", time.Now().Add(time.Hour))

	withReply := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: a, Body: "words that go"})
	alone := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: a, Body: "more words that go"})
	reply := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: b, ParentID: withReply, Body: "an answer that stays"})
	b1 := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: b, Body: "b one"})
	b2 := testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: b, Body: "b two"})
	for _, id := range []int64{b1, b2, reply} {
		testutil.SeedCommentLike(t, pool, id, a) // three likes the account gave
	}
	testutil.SeedCommentLike(t, pool, withReply, b) // a like given to the account's comment
	testutil.SeedCommentFlag(t, pool, b1, a)        // one flag the account sent
	testutil.SeedCommentFlag(t, pool, withReply, b) // a flag on the account's comment

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	liveWords := func() int {
		return count(`SELECT count(*) FROM comments WHERE account_id = $1 AND deleted_at IS NULL AND body <> ''`, a)
	}

	// A failed scrub rolls every comment step back.
	failTx := func(ctx context.Context, fn func(q service.AccountDeleteQuerier) error) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return fn(failingScrubQuerier{sqlcgen.New(tx)}) })
	}
	if err := service.NewAccountDeletionService(q, failTx).DeleteAccount(ctx, a); !errors.Is(err, errBoom) {
		t.Fatalf("DeleteAccount with a failing scrub: want errBoom, got %v", err)
	}
	if liveWords() != 2 || count(`SELECT count(*) FROM comment_likes WHERE account_id = $1`, a) != 3 ||
		count(`SELECT count(*) FROM comment_flags WHERE account_id = $1`, a) != 1 {
		t.Fatalf("a failed deletion must change nothing: %d live comments, %d likes, %d flags",
			liveWords(), count(`SELECT count(*) FROM comment_likes WHERE account_id = $1`, a),
			count(`SELECT count(*) FROM comment_flags WHERE account_id = $1`, a))
	}

	if err := service.NewAccountDeletionService(q, service.PgxAccountTx(pool)).DeleteAccount(ctx, a); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if n := liveWords(); n != 0 {
		t.Errorf("the deleted account still has %d comments with words", n)
	}
	if n := count(`SELECT count(*) FROM comments WHERE account_id = $1 AND deleted_at IS NOT NULL AND body = ''`, a); n != 2 {
		t.Errorf("blanked comments of the deleted account = %d, want 2", n)
	}
	if n := count(`SELECT count(*) FROM comment_likes WHERE account_id = $1`, a); n != 0 {
		t.Errorf("likes the deleted account gave = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM comment_flags WHERE account_id = $1`, a); n != 0 {
		t.Errorf("flags the deleted account sent = %d, want 0", n)
	}
	// The other person's rows are untouched.
	if n := count(`SELECT count(*) FROM comment_likes WHERE comment_id = $1 AND account_id = $2`, withReply, b); n != 1 {
		t.Errorf("the other person's like on the account's comment = %d, want 1", n)
	}
	if n := count(`SELECT count(*) FROM comment_flags WHERE comment_id = $1 AND account_id = $2`, withReply, b); n != 1 {
		t.Errorf("the other person's flag on the account's comment = %d, want 1", n)
	}
	if n := count(`SELECT count(*) FROM comments WHERE id = ANY($1) AND deleted_at IS NULL AND body <> ''`, []int64{reply, b1, b2}); n != 3 {
		t.Errorf("the other person's comments with words = %d, want 3", n)
	}

	// What a reader sees: the answered comment is a placeholder with its reply,
	// the unanswered one is gone, and nothing names the deleted account.
	rows, err := q.CommentsForReport(ctx, sqlcgen.CommentsForReportParams{
		ViewerAccountID: b, ReportID: report, RootCap: 100, ReplyCap: 1000,
	})
	if err != nil {
		t.Fatalf("CommentsForReport: %v", err)
	}
	seen := map[int64]sqlcgen.CommentsForReportRow{}
	for _, r := range rows {
		seen[r.ID] = r
	}
	ph, ok := seen[withReply]
	if !ok || ph.Gone != "deleted" || ph.Body != "" || ph.AuthorUsername != nil || ph.AuthorKind != "" {
		t.Errorf("answered comment = listed %v %+v, want a deleted placeholder with no name", ok, ph)
	}
	if _, ok := seen[reply]; !ok {
		t.Error("the other person's reply must stay listed under the placeholder")
	}
	if _, ok := seen[alone]; ok {
		t.Error("the unanswered comment of the deleted account must not be listed")
	}
}

// TestDeleteAccountEndsFollowsAndBlocks (Phase 9 plan 02, research G6, T-09-07):
// deleting an account hard deletes every follow and block row that names it, in
// both directions, active or ended, switches its profile off, and leaves every
// row between other people alone. It needs a database and skips without
// DATABASE_URL.
func TestDeleteAccountEndsFollowsAndBlocks(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)

	a := testutil.SeedPerson(t, pool, "graph-del-a@example.com", "going.away", true)
	b := testutil.SeedPerson(t, pool, "graph-del-b@example.com", "staying.one", true)
	c := testutil.SeedPerson(t, pool, "graph-del-c@example.com", "staying.two", true)
	d := testutil.SeedPerson(t, pool, "graph-del-d@example.com", "staying.three", true)
	testutil.SeedFollow(t, pool, a, b, now) // a follows b
	testutil.SeedFollow(t, pool, b, a, now) // b follows a back
	testutil.SeedFollow(t, pool, c, a, now) // an ended follow of a
	testutil.MustExec(t, pool, `UPDATE account_follows SET ended_at = now(), end_reason = 'unfollowed' WHERE follower_id = $1`, c)
	testutil.SeedBlock(t, pool, c, a)       // a block against a
	testutil.SeedBlock(t, pool, a, d)       // a block by a
	testutil.SeedFollow(t, pool, b, c, now) // rows between other people stay
	testutil.SeedBlock(t, pool, d, c)

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 OR followee_id = $1`, a); n != 3 {
		t.Fatalf("seed failed: %d follow rows name the account, want 3", n)
	}

	if err := service.NewAccountDeletionService(q, service.PgxAccountTx(pool)).DeleteAccount(ctx, a); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if n := count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 OR followee_id = $1`, a); n != 0 {
		t.Errorf("follow rows still naming the deleted account = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM person_blocks WHERE blocker_id = $1 OR blocked_id = $1`, a); n != 0 {
		t.Errorf("block rows still naming the deleted account = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 AND followee_id = $2 AND ended_at IS NULL`, b, c); n != 1 {
		t.Errorf("the follow between two other people = %d, want 1 untouched", n)
	}
	if n := count(`SELECT count(*) FROM person_blocks WHERE blocker_id = $1 AND blocked_id = $2`, d, c); n != 1 {
		t.Errorf("the block between two other people = %d, want 1 untouched", n)
	}
	for _, id := range []int64{b, c, d} {
		if n := count(`SELECT count(*) FROM accounts WHERE id = $1 AND profile_public AND deleted_at IS NULL AND username IS NOT NULL`, id); n != 1 {
			t.Errorf("account %d changed otherwise: want still public, live and named", id)
		}
	}
	if n := count(`SELECT count(*) FROM accounts WHERE id = $1 AND NOT profile_public AND deleted_at IS NOT NULL`, a); n != 1 {
		t.Errorf("the scrubbed row must have profile_public false and deleted_at set")
	}
}

// TestDeleteAccountRemovesRatings (plan 09-13, research G6, Pitfall 11): deleting
// an account hard deletes the ratings it gave and the ratings it received, and
// leaves every other rating alone. It needs a database and skips without
// DATABASE_URL.
func TestDeleteAccountRemovesRatings(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	now := time.Now().UTC()

	reporterA := testutil.SeedPublicReporter(t, pool, "del-rate-a@example.com", "rated.one", "del-rate-sess-a", now)
	reporterB := testutil.SeedPublicReporter(t, pool, "del-rate-b@example.com", "rated.two", "del-rate-sess-b", now)
	reportA := testutil.SeedHistoryReport(t, pool, "del-rate-sess-a", "low", "flood", now.Add(time.Hour))
	reportB := testutil.SeedHistoryReport(t, pool, "del-rate-sess-b", "low", "flood", now.Add(time.Hour))
	ratersA := testutil.SeedRaters(t, pool, reporterA, reportA, []int{5, 4, 3})
	testutil.SeedRaters(t, pool, reporterB, reportB, []int{2, 2})

	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("counting: %v", err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM profile_ratings`); n != 5 {
		t.Fatalf("seed failed: %d ratings, want 5", n)
	}
	// One rater of A also rated B, so a rating it gave sits on another reporter.
	testutil.MustExec(t, pool, `INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 1)`, ratersA[0], reporterB)

	svc := service.NewAccountDeletionService(q, service.PgxAccountTx(pool))
	if err := svc.DeleteAccount(ctx, ratersA[0]); err != nil {
		t.Fatalf("DeleteAccount (a rater): %v", err)
	}
	if n := count(`SELECT count(*) FROM profile_ratings WHERE rater_id = $1`, ratersA[0]); n != 0 {
		t.Errorf("ratings still given by the deleted rater = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM profile_ratings`); n != 4 {
		t.Errorf("after deleting one rater %d ratings remain, want 4 of the 6 (its two are gone)", n)
	}

	if err := svc.DeleteAccount(ctx, reporterB); err != nil {
		t.Fatalf("DeleteAccount (a reporter): %v", err)
	}
	if n := count(`SELECT count(*) FROM profile_ratings WHERE reporter_id = $1 OR rater_id = $1`, reporterB); n != 0 {
		t.Errorf("ratings still naming the deleted reporter = %d, want 0", n)
	}
	if n := count(`SELECT count(*) FROM profile_ratings WHERE reporter_id = $1`, reporterA); n != 2 {
		t.Errorf("the other reporter's remaining ratings = %d, want the 2 of the live raters", n)
	}
}

// TestDeleteConfirmMessage: the wording is fixed and has no dash.
func TestDeleteConfirmMessage(t *testing.T) {
	if service.DeleteConfirmMessage != "Confirm to delete your account." {
		t.Errorf("DeleteConfirmMessage = %q", service.DeleteConfirmMessage)
	}
}
