package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pinalert/internal/auth"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestCredentialFlowOnDatabase walks Create account, the typed code, Sign in,
// the lockout, Forgot password and a legacy account through the real queries.
// It uses the real clock: the try budget, the consume and the live code read
// compare against the database's own now(). It skips when DATABASE_URL is unset.
func TestCredentialFlowOnDatabase(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	mail := &credMailer{}

	limits := service.DefaultCredentialLimits
	// The flow asks for a code, then a reset code for the same address, seconds
	// apart. The default 45 second cooldown would refuse the second request, so
	// it is set to zero here; the hourly count of 5 stays in force.
	limits.SendCooldown = 0
	svc := service.NewCredentialService(q, mail,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
	)

	// Create account on browser A, typed code, signed in on a fresh id B.
	if err := svc.SignUp(ctx, "sess-A", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if live, ok, err := svc.LiveCode(ctx, "sess-A"); err != nil || !ok || live.Email != credEmail || live.SecondsLeft <= 0 {
		t.Fatalf("LiveCode = %+v ok=%v err=%v", live, ok, err)
	}
	code := mail.last(t).msg.Code
	if _, err := svc.ConfirmSignUp(ctx, "sess-A", credWrongCode(code), credSessionIDs("x")); !errors.Is(err, service.ErrWrongCode) {
		t.Fatalf("wrong code: %v", err)
	}
	res, err := svc.ConfirmSignUp(ctx, "sess-A", code, credSessionIDs("sess-B"))
	if err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	if res.SessionID != "sess-B-1" {
		t.Fatalf("session id = %q", res.SessionID)
	}
	row, err := q.GetAccountBySessionID(ctx, res.SessionID)
	if err != nil || row.ID != res.AccountID || !row.HasPassword || row.Email != credEmail {
		t.Fatalf("GetAccountBySessionID(B) = %+v err=%v", row, err)
	}
	if _, err := q.GetAccountBySessionID(ctx, "sess-A"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("the pre sign in id must stay unbound, err = %v", err)
	}
	if ok, err := svc.SignedIn(ctx, res.SessionID); err != nil || !ok {
		t.Fatalf("SignedIn(B) = %v, %v", ok, err)
	}
	if _, ok, _ := svc.LiveCode(ctx, "sess-A"); ok {
		t.Error("the spent code must not be shown again")
	}

	// Sign in on browser C.
	in, err := svc.SignIn(ctx, "sess-C", credEmail, credPassword, credSessionIDs("sess-C2"))
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if in.AccountID != res.AccountID || in.SessionID != "sess-C2-1" {
		t.Fatalf("sign in = %+v", in)
	}

	// Six wrong passwords lock the address; the seventh, with the right one,
	// is refused as locked.
	for i := 1; i <= 6; i++ {
		_, err := svc.SignIn(ctx, "sess-E", credEmail, "not the password 12", credSessionIDs("sess-E2"))
		if !errors.Is(err, service.ErrWrongCredentials) {
			t.Fatalf("wrong sign in %d: %v", i, err)
		}
	}
	_, err = svc.SignIn(ctx, "sess-E", credEmail, credPassword, credSessionIDs("sess-E2"))
	var locked service.SignInLockedError
	if !errors.As(err, &locked) || locked.Wait <= 0 {
		t.Fatalf("the 7th sign in: err = %v, want SignInLockedError", err)
	}
	var throttleRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_throttles`).Scan(&throttleRows); err != nil || throttleRows != 1 {
		t.Fatalf("throttle rows = %d err=%v", throttleRows, err)
	}

	// Forgot password on browser D clears the lock and ends every session.
	if err := svc.RequestReset(ctx, "sess-D", credEmail); err != nil {
		t.Fatalf("RequestReset: %v", err)
	}
	resetCode := mail.last(t).msg.Code
	newPassword := "a brand new password 55"
	if err := svc.ConfirmReset(ctx, "sess-D", resetCode, newPassword); err != nil {
		t.Fatalf("ConfirmReset: %v", err)
	}
	for _, id := range []string{res.SessionID, in.SessionID} {
		if ok, err := svc.SignedIn(ctx, id); err != nil || ok {
			t.Errorf("SignedIn(%s) after the reset = %v, %v; every session must be revoked", id, ok, err)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_throttles`).Scan(&throttleRows); err != nil || throttleRows != 0 {
		t.Fatalf("throttle rows after the reset = %d err=%v", throttleRows, err)
	}
	if _, err := q.GetAccountBySessionID(ctx, "sess-D"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("a reset must not sign the browser in, err = %v", err)
	}
	if _, err := svc.SignIn(ctx, "sess-F", credEmail, credPassword, credSessionIDs("f")); !errors.Is(err, service.ErrWrongCredentials) {
		t.Errorf("the old password must stop working: %v", err)
	}
	if _, err := svc.SignIn(ctx, "sess-G", credEmail, newPassword, credSessionIDs("sess-G2")); err != nil {
		t.Fatalf("sign in with the new password: %v", err)
	}

	// An account made with the old link keeps its id when it sets a password.
	if err := q.InsertAccount(ctx, "old@example.com"); err != nil {
		t.Fatal(err)
	}
	legacy, err := q.GetAccountForSignIn(ctx, "old@example.com")
	if err != nil || legacy.PasswordHash != nil {
		t.Fatalf("legacy account = %+v err=%v", legacy, err)
	}
	if err := svc.SignUp(ctx, "sess-L", "old@example.com", credPassword); err != nil {
		t.Fatalf("SignUp for a legacy address: %v", err)
	}
	legacyRes, err := svc.ConfirmSignUp(ctx, "sess-L", mail.last(t).msg.Code, credSessionIDs("sess-L2"))
	if err != nil {
		t.Fatalf("ConfirmSignUp for a legacy address: %v", err)
	}
	if legacyRes.AccountID != legacy.ID {
		t.Errorf("account id = %d, want the legacy %d", legacyRes.AccountID, legacy.ID)
	}
}

// gateHasher is a password hasher whose Verify blocks until release is closed,
// so a test can hold many sign ins inside the password check at once. It
// counts the Verify calls and the calls in flight.
type gateHasher struct {
	inner    *auth.Hasher
	release  chan struct{}
	verifies atomic.Int64
	inFlight atomic.Int64
}

func (h *gateHasher) Hash(ctx context.Context, password string) (string, error) {
	return h.inner.Hash(ctx, password)
}

func (h *gateHasher) Verify(ctx context.Context, encoded, password string) (bool, bool, error) {
	h.verifies.Add(1)
	h.inFlight.Add(1)
	<-h.release
	return h.inner.Verify(ctx, encoded, password)
}

func (h *gateHasher) DummyHash() string { return h.inner.DummyHash() }

// TestSignInLockCountsBeforeVerify is CR-S01 of the Phase 7.1 review: the per
// address lock must count a try before the password check, in one guarded
// statement, so a parallel burst of wrong passwords for one address gets at
// most FreeFailures + 1 password checks and every other request is refused as
// locked. Before the fix every request read "not locked" and all of them
// reached the password check.
func TestSignInLockCountsBeforeVerify(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	h := &gateHasher{inner: auth.NewHasher(auth.TestParams()), release: make(chan struct{})}
	svc := service.NewCredentialService(q, &credMailer{},
		service.WithHasher(h),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
	)
	const burst = 20
	var returned atomic.Int64
	var locked atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer returned.Add(1)
			_, err := svc.SignIn(ctx, fmt.Sprintf("burst-%d", i), credEmail, "not the password 12", credSessionIDs("x"))
			var le service.SignInLockedError
			if errors.As(err, &le) {
				locked.Add(1)
			}
		}(i)
	}
	deadline := time.Now().Add(10 * time.Second)
	for h.inFlight.Load()+returned.Load() < burst && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(h.release)
	wg.Wait()

	free := int64(service.DefaultCredentialLimits.FreeFailures)
	if got := h.verifies.Load(); got > free+1 {
		t.Errorf("password checks in a parallel burst = %d, want at most %d", got, free+1)
	}
	if got := locked.Load(); got < burst-(free+1) {
		t.Errorf("requests refused as locked = %d, want at least %d", got, burst-(free+1))
	}
}

// TestSignInGoodPasswordClearsCount is the other half of CR-S01: a try is
// counted before the check, so a right password must clear the count, or a
// person who mistypes a few times and then signs in would lock themselves out
// later.
func TestSignInGoodPasswordClearsCount(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	mail := &credMailer{}
	svc := service.NewCredentialService(q, mail,
		service.WithHasher(auth.NewHasher(auth.TestParams())),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
	)
	if err := svc.SignUp(ctx, "sess-A", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := svc.ConfirmSignUp(ctx, "sess-A", mail.last(t).msg.Code, credSessionIDs("sess-B")); err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	for round := 1; round <= 2; round++ {
		for i := 1; i <= service.DefaultCredentialLimits.FreeFailures; i++ {
			if _, err := svc.SignIn(ctx, "sess-C", credEmail, "not the password 12", credSessionIDs("c")); !errors.Is(err, service.ErrWrongCredentials) {
				t.Fatalf("round %d wrong sign in %d: %v", round, i, err)
			}
		}
		if _, err := svc.SignIn(ctx, "sess-C", credEmail, credPassword, credSessionIDs(fmt.Sprintf("good%d", round))); err != nil {
			t.Fatalf("round %d: the right password after %d wrong ones: %v", round, service.DefaultCredentialLimits.FreeFailures, err)
		}
		var rows int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_throttles`).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("round %d: throttle rows after a good sign in = %d err=%v, want 0", round, rows, err)
		}
	}
}
