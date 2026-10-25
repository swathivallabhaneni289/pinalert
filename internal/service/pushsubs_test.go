package service

// pushsubs_test.go proves PushSubscriptionService with a fake querier that
// records every call: validation happens before any query, every query carries
// the caller's own account id, the device cap is applied after each register,
// and no error or log line holds the endpoint or the keys.

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"pinalert/internal/push"
	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	testSubEndpoint = "https://fcm.googleapis.com/fcm/send/secret-endpoint-token-1"
)

// pushSubsFake records every call of the six subscription queries.
type pushSubsFake struct {
	calls []string

	upserts    []sqlcgen.UpsertPushSubscriptionParams
	trims      []sqlcgen.TrimPushSubscriptionsParams
	registered []sqlcgen.PushSubscriptionRegisteredParams
	deletedFor []sqlcgen.DeletePushSubscriptionForAccountParams
	sessionDel []string
	accountDel []int64

	registeredAnswer bool
	upsertErr        error
	trimErr          error
	registeredErr    error
	deleteErr        error
}

func (f *pushSubsFake) UpsertPushSubscription(_ context.Context, arg sqlcgen.UpsertPushSubscriptionParams) (int64, error) {
	f.calls = append(f.calls, "upsert")
	f.upserts = append(f.upserts, arg)
	return 1, f.upsertErr
}

func (f *pushSubsFake) TrimPushSubscriptions(_ context.Context, arg sqlcgen.TrimPushSubscriptionsParams) (int64, error) {
	f.calls = append(f.calls, "trim")
	f.trims = append(f.trims, arg)
	return 0, f.trimErr
}

func (f *pushSubsFake) PushSubscriptionRegistered(_ context.Context, arg sqlcgen.PushSubscriptionRegisteredParams) (bool, error) {
	f.calls = append(f.calls, "registered")
	f.registered = append(f.registered, arg)
	return f.registeredAnswer, f.registeredErr
}

func (f *pushSubsFake) DeletePushSubscriptionForAccount(_ context.Context, arg sqlcgen.DeletePushSubscriptionForAccountParams) (int64, error) {
	f.calls = append(f.calls, "delete-for-account-endpoint")
	f.deletedFor = append(f.deletedFor, arg)
	return 0, f.deleteErr
}

func (f *pushSubsFake) DeletePushSubscriptionsForSession(_ context.Context, sessionID string) (int64, error) {
	f.calls = append(f.calls, "delete-for-session")
	f.sessionDel = append(f.sessionDel, sessionID)
	return 2, f.deleteErr
}

func (f *pushSubsFake) DeleteAccountPushSubscriptions(_ context.Context, accountID int64) (int64, error) {
	f.calls = append(f.calls, "delete-for-account")
	f.accountDel = append(f.accountDel, accountID)
	return 3, f.deleteErr
}

// validSubKeys returns a well formed p256dh and auth pair built in code.
func validSubKeys() (p256dh, auth string) {
	pub := make([]byte, 65)
	pub[0] = 0x04
	for i := 1; i < len(pub); i++ {
		pub[i] = byte(i)
	}
	secret := make([]byte, 16)
	for i := range secret {
		secret[i] = byte(100 + i)
	}
	return base64.RawURLEncoding.EncodeToString(pub), base64.RawURLEncoding.EncodeToString(secret)
}

func validSubInput() PushSubscriptionInput {
	p, a := validSubKeys()
	return PushSubscriptionInput{Endpoint: testSubEndpoint, P256dh: p, Auth: a}
}

func newPushSubsForTest(f *pushSubsFake) (*PushSubscriptionService, time.Time) {
	now := time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)
	return NewPushSubscriptionService(f, push.DefaultEndpointPolicy(), WithPushSubsClock(func() time.Time { return now })), now
}

func TestRegisterValidatesBeforeStoring(t *testing.T) {
	goodP, goodA := validSubKeys()
	cases := []struct {
		name      string
		in        PushSubscriptionInput
		wantField string
		wantMsg   string
	}{
		{"unknown https host", PushSubscriptionInput{Endpoint: "https://evil.example.com/send/abc", P256dh: goodP, Auth: goodA}, "endpoint", PushServiceUnsupportedMessage},
		{"look alike host", PushSubscriptionInput{Endpoint: "https://evil-fcm.googleapis.com.example.org/x", P256dh: goodP, Auth: goodA}, "endpoint", PushServiceUnsupportedMessage},
		{"plain http", PushSubscriptionInput{Endpoint: "http://fcm.googleapis.com/fcm/send/x", P256dh: goodP, Auth: goodA}, "endpoint", PushSubscriptionInvalidMessage},
		{"empty endpoint", PushSubscriptionInput{Endpoint: "", P256dh: goodP, Auth: goodA}, "endpoint", PushSubscriptionInvalidMessage},
		{"ip literal", PushSubscriptionInput{Endpoint: "https://10.0.0.5/send", P256dh: goodP, Auth: goodA}, "endpoint", PushSubscriptionInvalidMessage},
		{"bad p256dh", PushSubscriptionInput{Endpoint: testSubEndpoint, P256dh: "not-a-key", Auth: goodA}, "keys", PushSubscriptionInvalidMessage},
		{"bad auth", PushSubscriptionInput{Endpoint: testSubEndpoint, P256dh: goodP, Auth: "AAAA"}, "keys", PushSubscriptionInvalidMessage},
		{"missing keys", PushSubscriptionInput{Endpoint: testSubEndpoint}, "keys", PushSubscriptionInvalidMessage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &pushSubsFake{}
			svc, _ := newPushSubsForTest(f)
			err := svc.Register(context.Background(), 7, "sess-1", tc.in)
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Register error = %v, want a ValidationError", err)
			}
			if ve.Field != tc.wantField || ve.Message != tc.wantMsg {
				t.Fatalf("ValidationError = %+v, want field %q message %q", ve, tc.wantField, tc.wantMsg)
			}
			if len(f.calls) != 0 {
				t.Fatalf("a refused subscription ran queries: %v", f.calls)
			}
		})
	}

	t.Run("empty session id", func(t *testing.T) {
		f := &pushSubsFake{}
		svc, _ := newPushSubsForTest(f)
		err := svc.Register(context.Background(), 7, "", validSubInput())
		if err == nil {
			t.Fatal("Register with no session id succeeded")
		}
		var ve ValidationError
		if errors.As(err, &ve) {
			t.Fatalf("an empty session id is a programming error, not a ValidationError: %v", err)
		}
		if len(f.calls) != 0 {
			t.Fatalf("an empty session id ran queries: %v", f.calls)
		}
	})
}

func TestRegisterStoresAndThenTrims(t *testing.T) {
	f := &pushSubsFake{}
	svc, now := newPushSubsForTest(f)
	in := validSubInput()
	if err := svc.Register(context.Background(), 7, "sess-1", in); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got, want := strings.Join(f.calls, ","), "upsert,trim"; got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
	up := f.upserts[0]
	if up.AccountID != 7 || up.SessionID != "sess-1" || up.Endpoint != in.Endpoint ||
		up.EndpointHost != "fcm.googleapis.com" || up.P256dh != in.P256dh || up.Auth != in.Auth || !up.Now.Equal(now) {
		t.Fatalf("upsert params = %+v", up)
	}
}

func TestRegisterTrimsToDeviceCap(t *testing.T) {
	f := &pushSubsFake{}
	svc, _ := newPushSubsForTest(f)
	if err := svc.Register(context.Background(), 41, "sess-9", validSubInput()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(f.trims) != 1 {
		t.Fatalf("trim ran %d times, want once", len(f.trims))
	}
	if f.trims[0].AccountID != 41 || int(f.trims[0].Keep) != PushDevicesPerAccount || PushDevicesPerAccount != 8 {
		t.Fatalf("trim params = %+v, want account 41 keep 8", f.trims[0])
	}

	// A failed upsert must not trim: the older devices stay when nothing new
	// was stored.
	g := &pushSubsFake{upsertErr: errors.New("boom")}
	svc2, _ := newPushSubsForTest(g)
	if err := svc2.Register(context.Background(), 41, "sess-9", validSubInput()); err == nil {
		t.Fatal("Register ignored an upsert failure")
	}
	if len(g.trims) != 0 {
		t.Fatalf("trim ran after a failed upsert: %v", g.calls)
	}
}

func TestRegisteredAndUnregisterKeyedByAccount(t *testing.T) {
	f := &pushSubsFake{registeredAnswer: true}
	svc, _ := newPushSubsForTest(f)
	ctx := context.Background()

	got, err := svc.Registered(ctx, 5, testSubEndpoint)
	if err != nil || !got {
		t.Fatalf("Registered = %v, %v, want true, nil", got, err)
	}
	if len(f.registered) != 1 || f.registered[0].AccountID != 5 || f.registered[0].Endpoint != testSubEndpoint {
		t.Fatalf("registered params = %+v", f.registered)
	}

	if err := svc.Unregister(ctx, 5, testSubEndpoint); err != nil {
		t.Fatalf("Unregister with no row: %v", err)
	}
	if len(f.deletedFor) != 1 || f.deletedFor[0].AccountID != 5 || f.deletedFor[0].Endpoint != testSubEndpoint {
		t.Fatalf("delete params = %+v", f.deletedFor)
	}

	// An off list endpoint can still be asked about and removed: a service
	// dropped from the list later must not strand a stored device.
	if _, err := svc.Registered(ctx, 5, "https://gone.example.org/x"); err != nil {
		t.Fatalf("Registered with an off list endpoint: %v", err)
	}
	if err := svc.Unregister(ctx, 5, "https://gone.example.org/x"); err != nil {
		t.Fatalf("Unregister with an off list endpoint: %v", err)
	}

	// An empty endpoint asks nothing.
	f2 := &pushSubsFake{}
	svc2, _ := newPushSubsForTest(f2)
	for _, call := range []func() error{
		func() error { _, err := svc2.Registered(ctx, 5, ""); return err },
		func() error { return svc2.Unregister(ctx, 5, "") },
	} {
		var ve ValidationError
		if err := call(); !errors.As(err, &ve) || ve.Field != "endpoint" {
			t.Fatalf("empty endpoint error = %v, want a ValidationError on endpoint", err)
		}
	}
	if len(f2.calls) != 0 {
		t.Fatalf("an empty endpoint ran queries: %v", f2.calls)
	}
}

func TestPushSubsDeleteForSessionAndAccount(t *testing.T) {
	f := &pushSubsFake{}
	svc, _ := newPushSubsForTest(f)
	ctx := context.Background()
	if err := svc.DeleteForSession(ctx, "sess-3"); err != nil {
		t.Fatalf("DeleteForSession: %v", err)
	}
	if err := svc.DeleteForAccount(ctx, 12); err != nil {
		t.Fatalf("DeleteForAccount: %v", err)
	}
	if len(f.sessionDel) != 1 || f.sessionDel[0] != "sess-3" {
		t.Fatalf("session deletes = %v", f.sessionDel)
	}
	if len(f.accountDel) != 1 || f.accountDel[0] != 12 {
		t.Fatalf("account deletes = %v", f.accountDel)
	}
	if err := svc.DeleteForSession(ctx, ""); err == nil {
		t.Fatal("DeleteForSession with no session id succeeded")
	}
}

// TestPushSubsNeverLogsEndpoint makes every query fail with a database error
// whose detail holds the endpoint, as a driver error could, and checks that
// neither the returned error nor the standard logger holds the endpoint or a
// key.
func TestPushSubsNeverLogsEndpoint(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	in := validSubInput()
	leaky := &pgconn.PgError{Code: "23503", Message: "insert failed", Detail: "Key (endpoint)=(" + in.Endpoint + ") " + in.P256dh + " " + in.Auth}
	f := &pushSubsFake{upsertErr: leaky, trimErr: leaky, registeredErr: leaky, deleteErr: leaky}
	svc, _ := newPushSubsForTest(f)
	ctx := context.Background()

	var errs []error
	errs = append(errs, svc.Register(ctx, 3, "sess-1", in))
	_, rerr := svc.Registered(ctx, 3, in.Endpoint)
	errs = append(errs, rerr, svc.Unregister(ctx, 3, in.Endpoint), svc.DeleteForSession(ctx, "sess-1"), svc.DeleteForAccount(ctx, 3))

	// A refused subscription too.
	errs = append(errs, svc.Register(ctx, 3, "sess-1", PushSubscriptionInput{Endpoint: "https://evil.example.com/" + in.Endpoint, P256dh: in.P256dh, Auth: in.Auth}))

	for i, err := range errs {
		if err == nil {
			t.Fatalf("call %d returned nil, want an error", i)
		}
		msg := err.Error()
		for _, secret := range []string{in.Endpoint, "secret-endpoint-token-1", in.P256dh, in.Auth} {
			if strings.Contains(msg, secret) {
				t.Fatalf("call %d error holds a secret: %q", i, msg)
			}
		}
	}
	if logged.Len() != 0 {
		t.Fatalf("the service wrote to the log: %q", logged.String())
	}

	// The error keeps the account id and the SQLSTATE for the log.
	if msg := errs[0].Error(); !strings.Contains(msg, "account 3") || !strings.Contains(msg, "23503") {
		t.Fatalf("Register error = %q, want the account id and the sqlstate", msg)
	}
}
