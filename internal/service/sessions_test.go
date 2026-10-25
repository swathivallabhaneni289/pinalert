package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeSessionQuerier records the calls SessionService makes. Amended
// 2026-10-18 by Phase 3.3 plan 10 (decision 11, UI-SPEC O-3): it also records
// DeleteAccountPushSubscriptions, which runs after the revoke.
type fakeSessionQuerier struct {
	args    []sqlcgen.RevokeAccountSessionsParams
	revoked int64
	err     error

	calls         []string
	pushDeleteIDs []int64
	pushDeleteErr error
}

func (f *fakeSessionQuerier) RevokeAccountSessions(ctx context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error) {
	f.args = append(f.args, arg)
	f.calls = append(f.calls, "RevokeAccountSessions")
	return f.revoked, f.err
}

func (f *fakeSessionQuerier) DeleteAccountPushSubscriptions(ctx context.Context, accountID int64) (int64, error) {
	f.pushDeleteIDs = append(f.pushDeleteIDs, accountID)
	f.calls = append(f.calls, "DeleteAccountPushSubscriptions")
	return 0, f.pushDeleteErr
}

// TestLogOutAllDevicesDeletesPushSubscriptions: after the sessions are revoked
// every push subscription of the same account is deleted, in that order. A
// failed delete returns a wrapped error after the revoke already ran, and a
// failed revoke deletes nothing.
func TestLogOutAllDevicesDeletesPushSubscriptions(t *testing.T) {
	q := &fakeSessionQuerier{revoked: 2}
	svc := service.NewSessionService(q)
	n, err := svc.LogOutAllDevices(context.Background(), 42)
	if err != nil {
		t.Fatalf("LogOutAllDevices: %v", err)
	}
	if n != 2 {
		t.Fatalf("count = %d, want 2", n)
	}
	want := []string{"RevokeAccountSessions", "DeleteAccountPushSubscriptions"}
	if len(q.calls) != 2 || q.calls[0] != want[0] || q.calls[1] != want[1] {
		t.Fatalf("calls = %v, want %v", q.calls, want)
	}
	if len(q.pushDeleteIDs) != 1 || q.pushDeleteIDs[0] != 42 {
		t.Fatalf("push delete account ids = %v, want [42]", q.pushDeleteIDs)
	}

	boom := errors.New("boom")
	failing := &fakeSessionQuerier{revoked: 1, pushDeleteErr: boom}
	_, err = service.NewSessionService(failing).LogOutAllDevices(context.Background(), 7)
	if !errors.Is(err, boom) {
		t.Fatalf("a failed subscription delete gave %v, want it wrapped", err)
	}
	if len(failing.args) != 1 {
		t.Fatalf("the revoke ran %d times before the failed delete, want 1", len(failing.args))
	}
	if strings.Contains(err.Error(), "@") {
		t.Fatalf("error %q names an address", err)
	}

	revokeFails := &fakeSessionQuerier{err: boom}
	if _, err := service.NewSessionService(revokeFails).LogOutAllDevices(context.Background(), 7); !errors.Is(err, boom) {
		t.Fatalf("a failed revoke gave %v", err)
	}
	if len(revokeFails.pushDeleteIDs) != 0 {
		t.Fatalf("a failed revoke still deleted subscriptions: %v", revokeFails.pushDeleteIDs)
	}
}

// TestLogOutAllDevicesKeyedByAccount: the call revokes by the account id it is
// given, stamps the service clock, and returns the affected count.
func TestLogOutAllDevicesKeyedByAccount(t *testing.T) {
	fixed := time.Date(2026, 10, 18, 9, 30, 0, 0, time.UTC)
	q := &fakeSessionQuerier{revoked: 3}
	svc := service.NewSessionService(q, service.WithSessionClock(func() time.Time { return fixed }))

	n, err := svc.LogOutAllDevices(context.Background(), 42)
	if err != nil {
		t.Fatalf("LogOutAllDevices: %v", err)
	}
	if n != 3 {
		t.Fatalf("count = %d, want 3", n)
	}
	if len(q.args) != 1 {
		t.Fatalf("RevokeAccountSessions calls = %d, want 1", len(q.args))
	}
	got := q.args[0]
	if got.AccountID == nil || *got.AccountID != 42 {
		t.Fatalf("AccountID = %v, want 42", got.AccountID)
	}
	if want := (pgtype.Timestamptz{Time: fixed, Valid: true}); got.Now != want {
		t.Fatalf("Now = %+v, want %+v", got.Now, want)
	}
}

// TestLogOutAllDevicesWrapsErrorWithoutEmail: a database error is wrapped and
// names the account id only, never an address.
func TestLogOutAllDevicesWrapsErrorWithoutEmail(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewSessionService(&fakeSessionQuerier{err: boom})

	_, err := svc.LogOutAllDevices(context.Background(), 7)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the database error", err)
	}
	if strings.Contains(err.Error(), "@") {
		t.Fatalf("error %q names an address", err)
	}
}

// TestNewSessionServiceNeverDereferences: the constructor takes a nil querier
// without touching it, as the other services' constructors do.
func TestNewSessionServiceNeverDereferences(t *testing.T) {
	if svc := service.NewSessionService(nil); svc == nil {
		t.Fatal("NewSessionService(nil) = nil")
	}
}
