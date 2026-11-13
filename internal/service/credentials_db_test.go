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
	"github.com/jackc/pgx/v5/pgxpool"

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
	if err := svc.CheckResetCode(ctx, "sess-D", resetCode); err != nil {
		t.Fatalf("CheckResetCode: %v", err)
	}
	if err := svc.ConfirmReset(ctx, "sess-D", newPassword); err != nil {
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

// failingCredQuerier is the real querier with two switches that make one
// statement fail, so a test can stop a multi step write half way.
type failingCredQuerier struct {
	service.CredentialQuerier
	failRevoke *atomic.Bool
	failClear  *atomic.Bool
}

var errInjected = errors.New("injected failure")

func (f failingCredQuerier) RevokeAccountSessions(ctx context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error) {
	if f.failRevoke.Load() {
		return 0, errInjected
	}
	return f.CredentialQuerier.RevokeAccountSessions(ctx, arg)
}

func (f failingCredQuerier) ClearAuthThrottle(ctx context.Context, arg sqlcgen.ClearAuthThrottleParams) error {
	if f.failClear.Load() {
		return errInjected
	}
	return f.CredentialQuerier.ClearAuthThrottle(ctx, arg)
}

// credDBRig is a CredentialService on the real database whose querier, and
// the querier of every transaction it opens, can be made to fail.
type credDBRig struct {
	pool       *pgxpool.Pool
	q          *sqlcgen.Queries
	svc        *service.CredentialService
	mail       *credMailer
	failRevoke *atomic.Bool
	failClear  *atomic.Bool
}

func newCredDBRig(t *testing.T, h service.PasswordHasher) *credDBRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	r := &credDBRig{pool: pool, q: sqlcgen.New(pool), mail: &credMailer{}, failRevoke: &atomic.Bool{}, failClear: &atomic.Bool{}}
	limits := service.DefaultCredentialLimits
	limits.SendCooldown = 0
	wrap := func(q service.CredentialQuerier) service.CredentialQuerier {
		return failingCredQuerier{CredentialQuerier: q, failRevoke: r.failRevoke, failClear: r.failClear}
	}
	r.svc = service.NewCredentialService(wrap(r.q), r.mail,
		service.WithHasher(h),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
		service.WithCredentialLimits(limits),
		service.WithCredentialTx(func(ctx context.Context, fn func(q service.CredentialQuerier) error) error {
			return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return fn(wrap(sqlcgen.New(tx))) })
		}),
	)
	return r
}

func (r *credDBRig) createAccount(t *testing.T, email, password string) service.SignInResult {
	t.Helper()
	ctx := context.Background()
	if err := r.svc.SignUp(ctx, "make-"+email, email, password); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	res, err := r.svc.ConfirmSignUp(ctx, "make-"+email, r.mail.last(t).msg.Code, credSessionIDs("made-"+email))
	if err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	return res
}

func (r *credDBRig) passwordState(t *testing.T, email string) (hash string, epoch int64) {
	t.Helper()
	if err := r.pool.QueryRow(context.Background(),
		`SELECT password_hash, password_epoch FROM accounts WHERE email = $1`, email).Scan(&hash, &epoch); err != nil {
		t.Fatalf("reading the password of %s: %v", email, err)
	}
	return hash, epoch
}

// TestResetEndsSignInInFlight is CR-S02 part 1: a sign in that checked the old
// password before a reset and binds its session after the reset must not end
// up with a live session. The reset moves the account's password epoch, the
// bind is refused when the epoch moved since the sign in read it, and the gate
// refuses a session whose epoch is not the account's.
func TestResetEndsSignInInFlight(t *testing.T) {
	h := &gateHasher{inner: auth.NewHasher(auth.TestParams()), release: make(chan struct{})}
	r := newCredDBRig(t, h)
	ctx := context.Background()
	// The account is made before the gate closes: Hash never blocks, and the
	// sign up path verifies nothing.
	r.createAccount(t, credEmail, credPassword)

	type outcome struct {
		res service.SignInResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := r.svc.SignIn(ctx, "sess-old", credEmail, credPassword, credSessionIDs("old-pw"))
		done <- outcome{res, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for h.inFlight.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if h.inFlight.Load() < 1 {
		t.Fatal("the sign in never reached the password check")
	}

	if err := r.svc.RequestReset(ctx, "sess-reset", credEmail); err != nil {
		t.Fatalf("RequestReset: %v", err)
	}
	if err := r.svc.CheckResetCode(ctx, "sess-reset", r.mail.last(t).msg.Code); err != nil {
		t.Fatalf("CheckResetCode: %v", err)
	}
	if err := r.svc.ConfirmReset(ctx, "sess-reset", "a brand new password 55"); err != nil {
		t.Fatalf("ConfirmReset: %v", err)
	}
	close(h.release)
	got := <-done
	if got.err == nil {
		live, err := r.svc.SignedIn(ctx, got.res.SessionID)
		if err != nil {
			t.Fatalf("SignedIn: %v", err)
		}
		if live {
			t.Fatal("a sign in with the old password that bound after the reset left a live session")
		}
	}
}

// TestConfirmResetIsAtomic is CR-S02 part 2: when ending the sessions fails,
// the reset must change nothing, so the person can type the same code again
// and the old sessions are not left alive under a new password.
func TestConfirmResetIsAtomic(t *testing.T) {
	r := newCredDBRig(t, auth.NewHasher(auth.TestParams()))
	ctx := context.Background()
	old := r.createAccount(t, credEmail, credPassword)
	hashBefore, epochBefore := r.passwordState(t, credEmail)

	if err := r.svc.RequestReset(ctx, "sess-reset", credEmail); err != nil {
		t.Fatalf("RequestReset: %v", err)
	}
	code := r.mail.last(t).msg.Code
	if err := r.svc.CheckResetCode(ctx, "sess-reset", code); err != nil {
		t.Fatalf("CheckResetCode: %v", err)
	}
	r.failRevoke.Store(true)
	if err := r.svc.ConfirmReset(ctx, "sess-reset", "a brand new password 55"); err == nil {
		t.Fatal("ConfirmReset succeeded although ending the sessions failed")
	}
	if hash, epoch := r.passwordState(t, credEmail); hash != hashBefore || epoch != epochBefore {
		t.Error("a failed reset changed the password")
	}
	if live, err := r.svc.SignedIn(ctx, old.SessionID); err != nil || !live {
		t.Errorf("a failed reset changed the old session: live=%v err=%v", live, err)
	}
	r.failRevoke.Store(false)
	if err := r.svc.ConfirmReset(ctx, "sess-reset", "a brand new password 55"); err != nil {
		t.Fatalf("the same code after a failed reset must still work: %v", err)
	}
	if live, _ := r.svc.SignedIn(ctx, old.SessionID); live {
		t.Error("the old session survived the reset")
	}
}

// TestConfirmSignUpEndsLegacySessions is WR-S02: Create account on an account
// made with the old link (no password) must end every session still bound to
// it, so a session made from an emailed link in the deploy gap never becomes
// signed in without a typed code.
func TestConfirmSignUpEndsLegacySessions(t *testing.T) {
	r := newCredDBRig(t, auth.NewHasher(auth.TestParams()))
	ctx := context.Background()
	const legacy = "old-link@example.com"
	if err := r.q.InsertAccount(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	acct, err := r.q.GetAccountForSignIn(ctx, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.q.BindSessionAccount(ctx, sqlcgen.BindSessionAccountParams{SessionID: "gap-session", AccountID: &acct.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.SignUp(ctx, "sess-new", legacy, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	res, err := r.svc.ConfirmSignUp(ctx, "sess-new", r.mail.last(t).msg.Code, credSessionIDs("fresh"))
	if err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	if live, err := r.svc.SignedIn(ctx, "gap-session"); err != nil || live {
		t.Errorf("the session bound before Create account is signed in: live=%v err=%v", live, err)
	}
	if live, err := r.svc.SignedIn(ctx, res.SessionID); err != nil || !live {
		t.Errorf("the session of the typed code is not signed in: live=%v err=%v", live, err)
	}
}

// busyOnceHasher answers ErrHasherBusy to its first Hash and then hashes.
type busyOnceHasher struct {
	*auth.Hasher
	busy atomic.Bool
}

func (h *busyOnceHasher) Hash(ctx context.Context, password string) (string, error) {
	if h.busy.CompareAndSwap(true, false) {
		return "", auth.ErrHasherBusy
	}
	return h.Hasher.Hash(ctx, password)
}

// TestSignUpBusyRefundsClaimOnDatabase is IN-S01 on the real queries, with the
// 45 second cooldown in force: a busy hasher gives the claim back, so the
// person's immediate retry is sent and the hourly count holds one send.
func TestSignUpBusyRefundsClaimOnDatabase(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	h := &busyOnceHasher{Hasher: auth.NewHasher(auth.TestParams())}
	h.busy.Store(true)
	mail := &credMailer{}
	svc := service.NewCredentialService(sqlcgen.New(pool), mail,
		service.WithHasher(h),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
	)
	if err := svc.SignUp(ctx, "sess-a", credEmail, credPassword); !errors.Is(err, service.ErrBusy) {
		t.Fatalf("first SignUp: %v, want ErrBusy", err)
	}
	if err := svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
		t.Fatalf("the retry right after a busy hasher: %v", err)
	}
	var sends int
	if err := pool.QueryRow(ctx, `SELECT window_sends FROM email_cooldowns WHERE email = $1`, credEmail).Scan(&sends); err != nil || sends != 1 {
		t.Errorf("hourly sends = %d err=%v, want 1", sends, err)
	}
	if err := svc.SignUp(ctx, "sess-a", credEmail, credPassword); !errors.Is(err, service.ErrRateLimited) {
		t.Errorf("a third request inside the cooldown: %v, want ErrRateLimited", err)
	}
}

// TestRefusedBindKeepsOtherAccountSession is SK-07 of the Phase 7.1 review fix
// skeptic: a sign in on a browser signed in to another account revokes that
// other session only after the new session is bound. When the bind is refused
// (a reset moved the password epoch while the password was being checked), the
// browser stays signed in to its previous account and keeps its phone alert
// subscription. Before the fix the old session was revoked and its
// subscriptions deleted first, and the answer was still wrong credentials.
func TestRefusedBindKeepsOtherAccountSession(t *testing.T) {
	h := &gateHasher{inner: auth.NewHasher(auth.TestParams()), release: make(chan struct{})}
	r := newCredDBRig(t, h)
	ctx := context.Background()
	const other = "other-account@example.com"
	prev := r.createAccount(t, other, credPassword)
	r.createAccount(t, credEmail, credPassword)
	testutil.MustExec(t, r.pool,
		`INSERT INTO push_subscriptions (account_id, session_id, endpoint, endpoint_host, p256dh, auth)
		 VALUES ($1, $2, 'https://push.example.com/sub/sk07', 'push.example.com', 'key', 'auth')`,
		prev.AccountID, prev.SessionID)

	done := make(chan error, 1)
	go func() {
		_, err := r.svc.SignIn(ctx, prev.SessionID, credEmail, credPassword, credSessionIDs("switch"))
		done <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for h.inFlight.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if h.inFlight.Load() < 1 {
		t.Fatal("the sign in never reached the password check")
	}
	if err := r.svc.RequestReset(ctx, "sess-reset", credEmail); err != nil {
		t.Fatalf("RequestReset: %v", err)
	}
	if err := r.svc.CheckResetCode(ctx, "sess-reset", r.mail.last(t).msg.Code); err != nil {
		t.Fatalf("CheckResetCode: %v", err)
	}
	if err := r.svc.ConfirmReset(ctx, "sess-reset", "a brand new password 55"); err != nil {
		t.Fatalf("ConfirmReset: %v", err)
	}
	close(h.release)
	if err := <-done; !errors.Is(err, service.ErrWrongCredentials) {
		t.Fatalf("the sign in whose bind was refused: %v, want ErrWrongCredentials", err)
	}
	if live, err := r.svc.SignedIn(ctx, prev.SessionID); err != nil || !live {
		t.Errorf("the browser's previous account session after a refused bind: live=%v err=%v, want still signed in", live, err)
	}
	var subs int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM push_subscriptions WHERE session_id = $1`, prev.SessionID).Scan(&subs); err != nil || subs != 1 {
		t.Errorf("push subscriptions of the previous session = %d err=%v, want 1", subs, err)
	}
}

// switchBusyHasher answers ErrHasherBusy to every Verify while busy is set,
// and checks for real otherwise.
type switchBusyHasher struct {
	*auth.Hasher
	busy atomic.Bool
}

func (h *switchBusyHasher) Verify(ctx context.Context, encoded, password string) (bool, bool, error) {
	if h.busy.Load() {
		return false, false, auth.ErrHasherBusy
	}
	return h.Hasher.Verify(ctx, encoded, password)
}

// TestSignInBusyGivesTryBack is SK-02 of the Phase 7.1 review fix skeptic: a
// sign in try is counted before the password check (CR-S01), and a busy hasher
// never checked the password, so it must give the counted try back. Otherwise a
// person pressing Sign in with the right password during a rush is locked out
// on the sixth busy answer. The give back is the same for a known and an
// unknown address. Before the fix the busy tries stayed counted and the right
// password then got SignInLockedError.
func TestSignInBusyGivesTryBack(t *testing.T) {
	h := &switchBusyHasher{Hasher: auth.NewHasher(auth.TestParams())}
	r := newCredDBRig(t, h)
	ctx := context.Background()
	r.createAccount(t, credEmail, credPassword)
	const unknown = "nobody-here@example.com"

	failures := func(email string) int {
		t.Helper()
		var n int
		err := r.pool.QueryRow(ctx, `SELECT failures FROM auth_throttles WHERE scope = 'sign_in' AND key_hash = $1`,
			r.svc.ThrottleKey(email)).Scan(&n)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0
		}
		if err != nil {
			t.Fatalf("reading the throttle row: %v", err)
		}
		return n
	}

	h.busy.Store(true)
	tries := service.DefaultCredentialLimits.FreeFailures + 1
	for _, email := range []string{credEmail, unknown} {
		for i := 1; i <= tries; i++ {
			_, err := r.svc.SignIn(ctx, "sess-busy", email, credPassword, credSessionIDs(fmt.Sprintf("busy-%d", i)))
			if !errors.Is(err, service.ErrBusy) {
				t.Fatalf("%s busy try %d: %v, want ErrBusy", email, i, err)
			}
		}
		if n := failures(email); n != 0 {
			t.Errorf("%s: failures after %d busy tries = %d, want 0", email, tries, n)
		}
	}

	// A wrong password after the busy tries still counts from nothing.
	h.busy.Store(false)
	if _, err := r.svc.SignIn(ctx, "sess-busy", credEmail, "not the password 12", credSessionIDs("wrong")); !errors.Is(err, service.ErrWrongCredentials) {
		t.Fatalf("a wrong password after the busy tries: %v, want ErrWrongCredentials", err)
	}
	if n := failures(credEmail); n != 1 {
		t.Errorf("failures after one wrong password = %d, want 1", n)
	}
	if _, err := r.svc.SignIn(ctx, "sess-busy", credEmail, credPassword, credSessionIDs("good")); err != nil {
		t.Fatalf("the right password after %d busy tries: %v", tries, err)
	}
	if n := failures(credEmail); n != 0 {
		t.Errorf("failures after a good sign in = %d, want 0", n)
	}
}

// TestConfirmSignUpKeepsCodeOnFailure is IN-S07: a database error after the
// code is proved must not spend it, so the person can type it again.
func TestConfirmSignUpKeepsCodeOnFailure(t *testing.T) {
	r := newCredDBRig(t, auth.NewHasher(auth.TestParams()))
	ctx := context.Background()
	if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	code := r.mail.last(t).msg.Code
	r.failClear.Store(true)
	if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("x")); err == nil {
		t.Fatal("ConfirmSignUp succeeded although a step failed")
	}
	r.failClear.Store(false)
	if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("y")); err != nil {
		t.Fatalf("the same code after a failed confirm must still work: %v", err)
	}
}
