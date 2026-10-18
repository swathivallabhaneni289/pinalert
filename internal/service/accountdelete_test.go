package service_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
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

	emailFor   string
	cooldown   string
	scrubArg   sqlcgen.ScrubDeletedAccountParams
	revokeArg  sqlcgen.RevokeAccountSessionsParams
	accountIDs map[string]int64
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

func (f *fakeDeleteQuerier) DeleteMagicLinkTokensForEmail(ctx context.Context, email string) error {
	f.emailFor = email
	return f.step("DeleteMagicLinkTokensForEmail")
}

func (f *fakeDeleteQuerier) DeleteEmailCooldownForEmail(ctx context.Context, email string) error {
	f.cooldown = email
	return f.step("DeleteEmailCooldownForEmail")
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

var wantDeleteOrder = []string{
	"AccountEmailForDelete",
	"DeleteMagicLinkTokensForEmail",
	"DeleteEmailCooldownForEmail",
	"DeleteAccountUpdates",
	"DeleteAccountCursors",
	"DeleteAccountMutes",
	"DeleteAccountAlertSettings",
	"RevokeAccountSessions",
	"ScrubDeletedAccount",
}

// TestDeleteAccountOrder: the nine steps run in the documented order, the
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
	if q.emailFor != "real@example.com" || q.cooldown != "real@example.com" {
		t.Errorf("token delete used %q and cooldown delete used %q, want the real address for both", q.emailFor, q.cooldown)
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
	if typ.NumMethod() != len(wantDeleteOrder) {
		t.Fatalf("AccountDeleteQuerier has %d methods, want %d", typ.NumMethod(), len(wantDeleteOrder))
	}
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i).Name
		found := false
		for _, w := range wantDeleteOrder {
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

// TestDeleteConfirmMessage: the wording is fixed and has no dash.
func TestDeleteConfirmMessage(t *testing.T) {
	if service.DeleteConfirmMessage != "Confirm to delete your account." {
		t.Errorf("DeleteConfirmMessage = %q", service.DeleteConfirmMessage)
	}
}
