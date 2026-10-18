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

// fakeSessionQuerier records the one call SessionService makes.
type fakeSessionQuerier struct {
	args    []sqlcgen.RevokeAccountSessionsParams
	revoked int64
	err     error
}

func (f *fakeSessionQuerier) RevokeAccountSessions(ctx context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error) {
	f.args = append(f.args, arg)
	return f.revoked, f.err
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
