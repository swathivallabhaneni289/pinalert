package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// Test inputs that hold special characters are built from rune values, never
// typed, so no dash or invisible character sits in this file.
var (
	credDecomposedPassword = "caf" + string(rune(0x65)) + string(rune(0x0301)) + " tulip 4821"
	credComposedPassword   = "caf" + string(rune(0xE9)) + " tulip 4821"
	credNBSP               = string(rune(0xA0))
)

const (
	credEmail    = "ravi@example.com"
	credPassword = "violet kettle 9271"
)

var (
	credCodeKey     = bytes.Repeat([]byte("k"), 32)
	credThrottleKey = bytes.Repeat([]byte("t"), 32)
)

// credClock is a clock tests can move.
type credClock struct{ t time.Time }

func (c *credClock) Now() time.Time          { return c.t }
func (c *credClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

type credCodeRow struct {
	id           int64
	session      string
	purpose      string
	delivery     string
	email        string
	codeHash     string
	passwordHash *string
	attempts     int32
	created      time.Time
	expires      time.Time
	used         *time.Time
}

type credAccount struct {
	id           int64
	email        string
	passwordHash *string
	deleted      bool
}

type credSession struct {
	account *int64
	revoked bool
}

type credClaim struct {
	last        time.Time
	windowStart *time.Time
	sends       int32
}

type credThrottle struct {
	failures int32
	last     time.Time
	locked   *time.Time
}

// credFakeQuerier is a small stateful stand in for the sqlc queries the
// CredentialService reads and writes. It follows the SQL closely enough that
// the unit tests pin the service's order of work and its use of each guard.
type credFakeQuerier struct {
	clk   *credClock
	calls []string

	codes     []credCodeRow
	nextCode  int64
	accounts  map[string]*credAccount
	nextAcct  int64
	sessions  map[string]*credSession
	claims    map[string]*credClaim
	throttles map[string]*credThrottle
	sendMax   int32

	pruneCodes     int
	pruneThrottles int
	pushDeleted    []string
	revokedAll     []int64
	failures       []string
}

func newCredFakeQuerier(clk *credClock) *credFakeQuerier {
	return &credFakeQuerier{
		clk:       clk,
		accounts:  map[string]*credAccount{},
		sessions:  map[string]*credSession{},
		claims:    map[string]*credClaim{},
		throttles: map[string]*credThrottle{},
	}
}

func (f *credFakeQuerier) log(s string) { f.calls = append(f.calls, s) }

func (f *credFakeQuerier) count(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (f *credFakeQuerier) addAccount(email string, passwordHash *string) *credAccount {
	f.nextAcct++
	a := &credAccount{id: f.nextAcct, email: email, passwordHash: passwordHash}
	f.accounts[email] = a
	return a
}

func (f *credFakeQuerier) bindSession(id string, account int64) {
	a := account
	f.sessions[id] = &credSession{account: &a}
}

func (f *credFakeQuerier) ClaimAuthEmailSend(_ context.Context, arg sqlcgen.ClaimAuthEmailSendParams) (sqlcgen.ClaimAuthEmailSendRow, error) {
	f.log("ClaimAuthEmailSend")
	f.sendMax = arg.MaxSends
	c, ok := f.claims[arg.Email]
	if !ok {
		w := arg.Now
		f.claims[arg.Email] = &credClaim{last: arg.Now, windowStart: &w, sends: 1}
		return sqlcgen.ClaimAuthEmailSendRow{LastRequestedAt: arg.Now, WindowSends: 1}, nil
	}
	windowOld := c.windowStart == nil || !c.windowStart.After(arg.WindowCutoff)
	if !(!c.last.After(arg.CooldownCutoff) && (windowOld || c.sends < arg.MaxSends)) {
		return sqlcgen.ClaimAuthEmailSendRow{}, pgx.ErrNoRows
	}
	c.last = arg.Now
	if windowOld {
		w := arg.Now
		c.windowStart = &w
		c.sends = 1
	} else {
		c.sends++
	}
	return sqlcgen.ClaimAuthEmailSendRow{LastRequestedAt: c.last, WindowSends: c.sends}, nil
}

func (f *credFakeQuerier) GetAuthEmailSendWindow(_ context.Context, email string) (sqlcgen.GetAuthEmailSendWindowRow, error) {
	f.log("GetAuthEmailSendWindow")
	c, ok := f.claims[email]
	if !ok {
		return sqlcgen.GetAuthEmailSendWindowRow{}, pgx.ErrNoRows
	}
	row := sqlcgen.GetAuthEmailSendWindowRow{LastRequestedAt: c.last, WindowSends: c.sends}
	if c.windowStart != nil {
		row.WindowStartedAt = pgtype.Timestamptz{Time: *c.windowStart, Valid: true}
	}
	return row, nil
}

func (f *credFakeQuerier) IssueAuthCode(_ context.Context, arg sqlcgen.IssueAuthCodeParams) (int64, error) {
	f.log("IssueAuthCode:" + arg.Purpose + ":" + arg.Delivery)
	now := f.clk.Now()
	for i := range f.codes {
		r := &f.codes[i]
		if r.session == arg.SessionID && r.purpose == arg.Purpose && r.used == nil {
			u := now
			r.used = &u
		}
	}
	f.nextCode++
	f.codes = append(f.codes, credCodeRow{
		id: f.nextCode, session: arg.SessionID, purpose: arg.Purpose, delivery: arg.Delivery,
		email: arg.Email, codeHash: arg.CodeHash, passwordHash: arg.PasswordHash,
		created: now, expires: arg.ExpiresAt,
	})
	return f.nextCode, nil
}

func (f *credFakeQuerier) BumpAuthCodeAttempt(_ context.Context, arg sqlcgen.BumpAuthCodeAttemptParams) (sqlcgen.BumpAuthCodeAttemptRow, error) {
	f.log("BumpAuthCodeAttempt")
	idx := -1
	for i := range f.codes {
		r := f.codes[i]
		if r.session == arg.SessionID && r.purpose == arg.Purpose && r.used == nil {
			idx = i
		}
	}
	if idx < 0 {
		return sqlcgen.BumpAuthCodeAttemptRow{}, pgx.ErrNoRows
	}
	r := &f.codes[idx]
	if !r.expires.After(f.clk.Now()) || r.attempts >= arg.MaxAttempts {
		return sqlcgen.BumpAuthCodeAttemptRow{}, pgx.ErrNoRows
	}
	r.attempts++
	return sqlcgen.BumpAuthCodeAttemptRow{
		ID: r.id, Delivery: r.delivery, Email: r.email, CodeHash: r.codeHash,
		PasswordHash: r.passwordHash, Attempts: r.attempts,
	}, nil
}

func (f *credFakeQuerier) ConsumeAuthCode(_ context.Context, id int64) (int64, error) {
	f.log("ConsumeAuthCode")
	for i := range f.codes {
		r := &f.codes[i]
		if r.id == id && r.used == nil && r.expires.After(f.clk.Now()) {
			u := f.clk.Now()
			r.used = &u
			return 1, nil
		}
	}
	return 0, nil
}

func (f *credFakeQuerier) GetLatestAuthCode(_ context.Context, arg sqlcgen.GetLatestAuthCodeParams) (sqlcgen.GetLatestAuthCodeRow, error) {
	f.log("GetLatestAuthCode")
	for i := len(f.codes) - 1; i >= 0; i-- {
		r := f.codes[i]
		if r.session == arg.SessionID && r.created.After(arg.Since) {
			row := sqlcgen.GetLatestAuthCodeRow{
				ID: r.id, Purpose: r.purpose, Delivery: r.delivery, Email: r.email,
				PasswordHash: r.passwordHash, CreatedAt: r.created, ExpiresAt: r.expires, Attempts: r.attempts,
			}
			if r.used != nil {
				row.UsedAt = pgtype.Timestamptz{Time: *r.used, Valid: true}
			}
			return row, nil
		}
	}
	return sqlcgen.GetLatestAuthCodeRow{}, pgx.ErrNoRows
}

func (f *credFakeQuerier) DeleteAuthCodesForEmail(_ context.Context, email string) error {
	f.log("DeleteAuthCodesForEmail")
	kept := f.codes[:0]
	for _, r := range f.codes {
		if r.email != email {
			kept = append(kept, r)
		}
	}
	f.codes = kept
	return nil
}

func (f *credFakeQuerier) PruneAuthCodes(_ context.Context, _ time.Time) (int64, error) {
	f.log("PruneAuthCodes")
	f.pruneCodes++
	return 0, nil
}

func (f *credFakeQuerier) GetAuthThrottle(_ context.Context, arg sqlcgen.GetAuthThrottleParams) (sqlcgen.GetAuthThrottleRow, error) {
	f.log("GetAuthThrottle")
	t, ok := f.throttles[arg.KeyHash]
	if !ok {
		return sqlcgen.GetAuthThrottleRow{}, pgx.ErrNoRows
	}
	row := sqlcgen.GetAuthThrottleRow{Failures: t.failures, LastFailureAt: t.last}
	if t.locked != nil {
		row.LockedUntil = pgtype.Timestamptz{Time: *t.locked, Valid: true}
	}
	return row, nil
}

// RecordAuthFailure follows the SQL: the count restarts only when both the last
// failure and the end of the last lock are at or before the forget cutoff.
func (f *credFakeQuerier) RecordAuthFailure(_ context.Context, arg sqlcgen.RecordAuthFailureParams) (sqlcgen.RecordAuthFailureRow, error) {
	f.log("RecordAuthFailure")
	if arg.Scope != "sign_in" {
		return sqlcgen.RecordAuthFailureRow{}, fmt.Errorf("fake: bad throttle scope %q", arg.Scope)
	}
	t, ok := f.throttles[arg.KeyHash]
	n := int32(1)
	if ok {
		forget := !t.last.After(arg.ForgetCutoff) && (t.locked == nil || !t.locked.After(arg.ForgetCutoff))
		if !forget {
			n = t.failures + 1
		}
	}
	var locked *time.Time
	if n > arg.FreeFailures {
		secs := int64(arg.BaseLockSeconds)
		for i := int32(0); i < n-arg.FreeFailures-1 && secs < int64(arg.MaxLockSeconds); i++ {
			secs *= 2
		}
		if secs > int64(arg.MaxLockSeconds) {
			secs = int64(arg.MaxLockSeconds)
		}
		l := arg.Now.Add(time.Duration(secs) * time.Second)
		locked = &l
	}
	f.throttles[arg.KeyHash] = &credThrottle{failures: n, last: arg.Now, locked: locked}
	row := sqlcgen.RecordAuthFailureRow{Failures: n}
	if locked != nil {
		row.LockedUntil = pgtype.Timestamptz{Time: *locked, Valid: true}
	}
	return row, nil
}

func (f *credFakeQuerier) ClearAuthThrottle(_ context.Context, arg sqlcgen.ClearAuthThrottleParams) error {
	f.log("ClearAuthThrottle")
	delete(f.throttles, arg.KeyHash)
	return nil
}

func (f *credFakeQuerier) PruneAuthThrottles(_ context.Context, _ time.Time) (int64, error) {
	f.log("PruneAuthThrottles")
	f.pruneThrottles++
	return 0, nil
}

func (f *credFakeQuerier) GetAccountForSignIn(_ context.Context, email string) (sqlcgen.GetAccountForSignInRow, error) {
	f.log("GetAccountForSignIn")
	a, ok := f.accounts[email]
	if !ok || a.deleted {
		return sqlcgen.GetAccountForSignInRow{}, pgx.ErrNoRows
	}
	return sqlcgen.GetAccountForSignInRow{ID: a.id, PasswordHash: a.passwordHash}, nil
}

func (f *credFakeQuerier) InsertAccount(_ context.Context, email string) error {
	f.log("InsertAccount")
	if _, ok := f.accounts[email]; !ok {
		f.addAccount(email, nil)
	}
	return nil
}

func (f *credFakeQuerier) SetAccountPassword(_ context.Context, arg sqlcgen.SetAccountPasswordParams) (int64, error) {
	f.log("SetAccountPassword")
	for _, a := range f.accounts {
		if a.id == arg.ID && !a.deleted {
			h := arg.PasswordHash
			a.passwordHash = &h
			return 1, nil
		}
	}
	return 0, nil
}

func (f *credFakeQuerier) BindSessionAccount(_ context.Context, arg sqlcgen.BindSessionAccountParams) (int64, error) {
	f.log("BindSessionAccount:" + arg.SessionID)
	s, ok := f.sessions[arg.SessionID]
	if !ok {
		a := *arg.AccountID
		f.sessions[arg.SessionID] = &credSession{account: &a}
		return 1, nil
	}
	if s.revoked || (s.account != nil && *s.account != *arg.AccountID) {
		return 0, nil
	}
	a := *arg.AccountID
	s.account = &a
	return 1, nil
}

func (f *credFakeQuerier) GetSessionBinding(_ context.Context, sessionID string) (sqlcgen.GetSessionBindingRow, error) {
	f.log("GetSessionBinding")
	s, ok := f.sessions[sessionID]
	if !ok {
		return sqlcgen.GetSessionBindingRow{}, pgx.ErrNoRows
	}
	return sqlcgen.GetSessionBindingRow{AccountID: s.account, Revoked: s.revoked}, nil
}

func (f *credFakeQuerier) GetAccountBySessionID(_ context.Context, sessionID string) (sqlcgen.GetAccountBySessionIDRow, error) {
	f.log("GetAccountBySessionID")
	s, ok := f.sessions[sessionID]
	if !ok || s.revoked || s.account == nil {
		return sqlcgen.GetAccountBySessionIDRow{}, pgx.ErrNoRows
	}
	for _, a := range f.accounts {
		if a.id == *s.account && !a.deleted {
			return sqlcgen.GetAccountBySessionIDRow{ID: a.id, Email: a.email, HasPassword: a.passwordHash != nil}, nil
		}
	}
	return sqlcgen.GetAccountBySessionIDRow{}, pgx.ErrNoRows
}

func (f *credFakeQuerier) SessionEndedByPasswordSwitch(_ context.Context, sessionID string) (bool, error) {
	f.log("SessionEndedByPasswordSwitch")
	s, ok := f.sessions[sessionID]
	if !ok || s.account == nil {
		return false, nil
	}
	for _, a := range f.accounts {
		if a.id == *s.account && !a.deleted && a.passwordHash == nil {
			return true, nil
		}
	}
	return false, nil
}

func (f *credFakeQuerier) RevokeSession(_ context.Context, arg sqlcgen.RevokeSessionParams) (int64, error) {
	f.log("RevokeSession:" + arg.SessionID)
	s, ok := f.sessions[arg.SessionID]
	if !ok || s.revoked {
		return 0, nil
	}
	s.revoked = true
	return 1, nil
}

func (f *credFakeQuerier) DeletePushSubscriptionsForSession(_ context.Context, sessionID string) (int64, error) {
	f.log("DeletePushSubscriptionsForSession:" + sessionID)
	f.pushDeleted = append(f.pushDeleted, sessionID)
	return 1, nil
}

func (f *credFakeQuerier) RevokeAccountSessions(_ context.Context, arg sqlcgen.RevokeAccountSessionsParams) (int64, error) {
	f.log("RevokeAccountSessions")
	var n int64
	for _, s := range f.sessions {
		if s.account != nil && arg.AccountID != nil && *s.account == *arg.AccountID && !s.revoked {
			s.revoked = true
			n++
		}
	}
	if arg.AccountID != nil {
		f.revokedAll = append(f.revokedAll, *arg.AccountID)
	}
	return n, nil
}

func (f *credFakeQuerier) DeleteAccountPushSubscriptions(_ context.Context, _ int64) (int64, error) {
	f.log("DeleteAccountPushSubscriptions")
	return 0, nil
}

// credMailer records every auth email.
type credMailer struct {
	sent []credSent
	err  error
}

type credSent struct {
	to  string
	msg mailer.AuthEmail
}

func (m *credMailer) SendVerificationLink(context.Context, string, string) error {
	return errors.New("credMailer: the link flow is not used")
}

func (m *credMailer) SendAuthEmail(_ context.Context, to string, msg mailer.AuthEmail) error {
	m.sent = append(m.sent, credSent{to: to, msg: msg})
	return m.err
}

func (m *credMailer) last(t *testing.T) credSent {
	t.Helper()
	if len(m.sent) == 0 {
		t.Fatal("no auth email was sent")
	}
	return m.sent[len(m.sent)-1]
}

// credHasher wraps a real, cheap hasher and records its calls.
type credHasher struct {
	inner     *auth.Hasher
	hashed    []string
	verified  []credVerify
	hashErr   error
	verifyErr error
}

type credVerify struct {
	encoded  string
	password string
}

func newCredHasher() *credHasher {
	return &credHasher{inner: auth.NewHasher(auth.TestParams())}
}

func (h *credHasher) Hash(ctx context.Context, password string) (string, error) {
	h.hashed = append(h.hashed, password)
	if h.hashErr != nil {
		return "", h.hashErr
	}
	return h.inner.Hash(ctx, password)
}

func (h *credHasher) Verify(ctx context.Context, encoded, password string) (bool, bool, error) {
	h.verified = append(h.verified, credVerify{encoded: encoded, password: password})
	if h.verifyErr != nil {
		return false, false, h.verifyErr
	}
	return h.inner.Verify(ctx, encoded, password)
}

func (h *credHasher) DummyHash() string { return h.inner.DummyHash() }

// credRig is one service over the fakes.
type credRig struct {
	svc    *service.CredentialService
	q      *credFakeQuerier
	mail   *credMailer
	hasher *credHasher
	clk    *credClock
}

func newCredRig(t *testing.T, opts ...service.CredentialOption) *credRig {
	t.Helper()
	clk := &credClock{t: time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)}
	q := newCredFakeQuerier(clk)
	m := &credMailer{}
	h := newCredHasher()
	all := append([]service.CredentialOption{
		service.WithCredentialClock(clk.Now),
		service.WithHasher(h),
		service.WithCodeKey(credCodeKey),
		service.WithThrottleKey(credThrottleKey),
	}, opts...)
	return &credRig{svc: service.NewCredentialService(q, m, all...), q: q, mail: m, hasher: h, clk: clk}
}

// codeSent returns the code of the newest email, failing when it has none.
func (r *credRig) codeSent(t *testing.T) string {
	t.Helper()
	s := r.mail.last(t)
	if s.msg.Code == "" {
		t.Fatalf("the newest email (%s) holds no code", s.msg.Kind)
	}
	return s.msg.Code
}

// wrongCode returns a 6 digit code that differs from code.
func credWrongCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

func credStrPtr(s string) *string { return &s }

func credSessionIDs(prefix string) func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("%s-%d", prefix, n)
	}
}

// ---------------------------------------------------------------- Task 1 ----

func TestSignUpBranches(t *testing.T) {
	t.Run("unknown address gets a code", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(context.Background(), "sess-a", " Ravi@Example.com ", credPassword); err != nil {
			t.Fatalf("SignUp: %v", err)
		}
		if got := r.q.count("ClaimAuthEmailSend"); got != 1 {
			t.Errorf("claims = %d, want 1", got)
		}
		if len(r.hasher.hashed) != 1 {
			t.Fatalf("hash calls = %d, want 1", len(r.hasher.hashed))
		}
		if len(r.q.codes) != 1 {
			t.Fatalf("code rows = %d, want 1", len(r.q.codes))
		}
		row := r.q.codes[0]
		if row.purpose != service.PurposeSignUp || row.delivery != service.DeliveryCode || row.email != credEmail {
			t.Errorf("row = %+v", row)
		}
		if row.passwordHash == nil {
			t.Fatal("the password hash was not stored")
		}
		ok, _, err := r.hasher.inner.Verify(context.Background(), *row.passwordHash, credPassword)
		if err != nil || !ok {
			t.Errorf("stored hash does not verify the typed password: ok=%v err=%v", ok, err)
		}
		if want := row.expires.Sub(r.clk.Now()); want != 10*time.Minute {
			t.Errorf("code life = %v, want 10m", want)
		}
		sent := r.mail.last(t)
		if len(r.mail.sent) != 1 || sent.to != credEmail || sent.msg.Kind != mailer.KindCodeSignup || sent.msg.MinutesValid != 10 {
			t.Errorf("email = %+v (count %d)", sent, len(r.mail.sent))
		}
		if want := auth.CodeHash(credCodeKey, "signup", credEmail, sent.msg.Code); row.codeHash != want {
			t.Errorf("stored code hash does not match the emailed code")
		}
	})

	t.Run("account made with the old link is treated as new", func(t *testing.T) {
		r := newCredRig(t)
		r.q.addAccount(credEmail, nil)
		if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
			t.Fatalf("SignUp: %v", err)
		}
		row := r.q.codes[0]
		if row.delivery != service.DeliveryCode || row.passwordHash == nil {
			t.Errorf("row = %+v", row)
		}
		if k := r.mail.last(t).msg.Kind; k != mailer.KindCodeSignup {
			t.Errorf("kind = %s, want code_signup", k)
		}
	})

	t.Run("address with a password gets the account exists note", func(t *testing.T) {
		r := newCredRig(t)
		r.q.addAccount(credEmail, credStrPtr("$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5"))
		if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
			t.Fatalf("SignUp must return the same nil result: %v", err)
		}
		if got := r.q.count("ClaimAuthEmailSend"); got != 1 {
			t.Errorf("claims = %d, want 1", got)
		}
		if len(r.hasher.hashed) != 1 {
			t.Errorf("hash calls = %d, want 1 (same work for every branch)", len(r.hasher.hashed))
		}
		if len(r.q.codes) != 1 {
			t.Fatalf("code rows = %d, want 1 (every branch writes one row)", len(r.q.codes))
		}
		row := r.q.codes[0]
		if row.delivery != service.DeliveryAccountExists || row.passwordHash != nil {
			t.Errorf("row = %+v", row)
		}
		sent := r.mail.last(t)
		if len(r.mail.sent) != 1 || sent.msg.Kind != mailer.KindAccountExists || sent.msg.Code != "" {
			t.Errorf("email = %+v", sent)
		}
	})

	t.Run("a bad email names the field", func(t *testing.T) {
		r := newCredRig(t)
		err := r.svc.SignUp(context.Background(), "sess-a", "not an email", credPassword)
		var ve service.ValidationError
		if !errors.As(err, &ve) || ve.Field != "email" || ve.Message != service.InvalidEmailMessage {
			t.Fatalf("err = %v", err)
		}
		if r.q.count("ClaimAuthEmailSend") != 0 || len(r.hasher.hashed) != 0 || len(r.mail.sent) != 0 {
			t.Error("a bad email must not claim, hash or mail")
		}
	})
}

func TestSignUpPasswordRule(t *testing.T) {
	cases := []struct {
		name     string
		password string
		want     string
	}{
		{"seven characters", "abc1234", service.PasswordTooShortMessage},
		{"common", "password123", service.PasswordCommonMessage},
		{"the address", credEmail, service.PasswordIsEmailMessage},
		{"129 characters", strings.Repeat("ab", 64) + "c", service.PasswordTooLongMessage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newCredRig(t)
			err := r.svc.SignUp(context.Background(), "sess-a", credEmail, c.password)
			var ve service.ValidationError
			if !errors.As(err, &ve) || ve.Field != "password" || ve.Message != c.want {
				t.Fatalf("err = %v, want password / %q", err, c.want)
			}
			if r.q.count("ClaimAuthEmailSend") != 0 || len(r.hasher.hashed) != 0 || len(r.q.codes) != 0 || len(r.mail.sent) != 0 {
				t.Error("a refused password must not claim, hash, write or mail")
			}
		})
	}
}

func TestSignUpSendLimits(t *testing.T) {
	t.Run("a second request inside the cooldown is rate limited", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		hashed, rows, sent := len(r.hasher.hashed), len(r.q.codes), len(r.mail.sent)
		err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword)
		if !errors.Is(err, service.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
		if len(r.hasher.hashed) != hashed || len(r.q.codes) != rows || len(r.mail.sent) != sent {
			t.Error("a refused claim must not hash, write or mail")
		}
		r.clk.Advance(46 * time.Second)
		if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
			t.Fatalf("after the cooldown: %v", err)
		}
	})

	t.Run("a sixth send inside the hour says too many codes", func(t *testing.T) {
		r := newCredRig(t)
		for i := 0; i < 5; i++ {
			if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
				t.Fatalf("send %d: %v", i+1, err)
			}
			r.clk.Advance(46 * time.Second)
		}
		hashed, sent := len(r.hasher.hashed), len(r.mail.sent)
		err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword)
		if !errors.Is(err, service.ErrTooManyCodes) {
			t.Fatalf("err = %v, want ErrTooManyCodes", err)
		}
		if len(r.hasher.hashed) != hashed || len(r.mail.sent) != sent {
			t.Error("a refused claim must not hash or mail")
		}
		if r.q.sendMax != 5 {
			t.Errorf("MaxSends = %d, want 5", r.q.sendMax)
		}
		// The hour passes: sending works again.
		r.clk.Advance(time.Hour)
		if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
			t.Fatalf("after the hour: %v", err)
		}
	})

	t.Run("a forgot password request spends the same count", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		if err := r.svc.RequestReset(context.Background(), "sess-b", credEmail); !errors.Is(err, service.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
	})
}

func TestSignUpMailCeiling(t *testing.T) {
	r := newCredRig(t)
	r.mail.err = mailer.ErrDailyCeiling
	err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword)
	if !errors.Is(err, service.ErrMailUnavailable) {
		t.Fatalf("err = %v, want ErrMailUnavailable", err)
	}
	if len(r.q.codes) != 1 {
		t.Error("the row is written before the send, so a failed send still spends the claim")
	}

	r2 := newCredRig(t)
	r2.mail.err = errors.New("provider said no")
	err = r2.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword)
	if err == nil || errors.Is(err, service.ErrMailUnavailable) {
		t.Fatalf("err = %v, want a wrapped generic error", err)
	}
	if strings.Contains(err.Error(), credEmail) {
		t.Errorf("the error carries the address: %v", err)
	}
}

func TestSignUpHashBusy(t *testing.T) {
	r := newCredRig(t)
	r.hasher.hashErr = auth.ErrHasherBusy
	err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword)
	if !errors.Is(err, service.ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	if len(r.q.codes) != 0 || len(r.mail.sent) != 0 {
		t.Error("a busy hasher must not write a row or send")
	}
}

func TestSignUpPrunesStaleRows(t *testing.T) {
	r := newCredRig(t)
	if err := r.svc.SignUp(context.Background(), "sess-a", credEmail, credPassword); err != nil {
		t.Fatal(err)
	}
	if r.q.pruneCodes != 1 || r.q.pruneThrottles != 1 {
		t.Errorf("prunes = codes %d, throttles %d, want 1 and 1", r.q.pruneCodes, r.q.pruneThrottles)
	}
}

func TestRequestResetBranches(t *testing.T) {
	t.Run("known address with a password", func(t *testing.T) {
		r := newCredRig(t)
		r.q.addAccount(credEmail, credStrPtr("$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5"))
		if err := r.svc.RequestReset(context.Background(), "sess-a", credEmail); err != nil {
			t.Fatal(err)
		}
		row := r.q.codes[0]
		if row.purpose != service.PurposeReset || row.delivery != service.DeliveryCode || row.passwordHash != nil {
			t.Errorf("row = %+v", row)
		}
		sent := r.mail.last(t)
		if sent.msg.Kind != mailer.KindCodeReset || sent.msg.MinutesValid != 10 {
			t.Errorf("email = %+v", sent)
		}
		if row.codeHash != auth.CodeHash(credCodeKey, "reset", credEmail, sent.msg.Code) {
			t.Error("stored hash does not match the emailed code")
		}
	})
	t.Run("known address with no password", func(t *testing.T) {
		r := newCredRig(t)
		r.q.addAccount(credEmail, nil)
		if err := r.svc.RequestReset(context.Background(), "sess-a", credEmail); err != nil {
			t.Fatal(err)
		}
		if r.q.codes[0].delivery != service.DeliveryCode || r.mail.last(t).msg.Kind != mailer.KindCodeReset {
			t.Errorf("row = %+v email = %+v", r.q.codes[0], r.mail.last(t))
		}
	})
	t.Run("unknown address", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.RequestReset(context.Background(), "sess-a", credEmail); err != nil {
			t.Fatalf("must return the same nil result: %v", err)
		}
		row := r.q.codes[0]
		if row.delivery != service.DeliveryNoAccount || row.passwordHash != nil {
			t.Errorf("row = %+v", row)
		}
		sent := r.mail.last(t)
		if len(r.mail.sent) != 1 || sent.msg.Kind != mailer.KindNoAccount || sent.msg.Code != "" {
			t.Errorf("email = %+v", sent)
		}
		if len(r.hasher.hashed) != 0 {
			t.Error("a reset request hashes no password")
		}
	})
	t.Run("a bad email names the field", func(t *testing.T) {
		r := newCredRig(t)
		var ve service.ValidationError
		if err := r.svc.RequestReset(context.Background(), "sess-a", "x"); !errors.As(err, &ve) || ve.Field != "email" {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestResendCodeReissues(t *testing.T) {
	t.Run("a sign up code is reissued with the same password hash", func(t *testing.T) {
		r := newCredRig(t)
		ctx := context.Background()
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		first := r.q.codes[0]
		r.clk.Advance(46 * time.Second)
		if err := r.svc.ResendCode(ctx, "sess-a"); err != nil {
			t.Fatalf("ResendCode: %v", err)
		}
		if len(r.q.codes) != 2 {
			t.Fatalf("rows = %d, want 2", len(r.q.codes))
		}
		old, fresh := r.q.codes[0], r.q.codes[1]
		if old.used == nil {
			t.Error("the old row must be retired")
		}
		if fresh.used != nil || fresh.attempts != 0 {
			t.Errorf("the new row must be live with a fresh budget: %+v", fresh)
		}
		if fresh.passwordHash == nil || *fresh.passwordHash != *first.passwordHash {
			t.Error("the password hash must be carried over")
		}
		sent := r.mail.last(t)
		if len(r.mail.sent) != 2 || sent.msg.Kind != mailer.KindCodeSignup || sent.to != credEmail {
			t.Errorf("email = %+v", sent)
		}
		if fresh.codeHash != auth.CodeHash(credCodeKey, "signup", credEmail, sent.msg.Code) {
			t.Error("new hash does not match the new code")
		}
		if len(r.hasher.hashed) != 1 {
			t.Errorf("hash calls = %d, a resend must not hash again", len(r.hasher.hashed))
		}
		if r.q.pruneCodes != 2 || r.q.pruneThrottles != 2 {
			t.Errorf("prunes = %d and %d, want 2 and 2 (one per request)", r.q.pruneCodes, r.q.pruneThrottles)
		}
	})

	t.Run("a reset code for an unknown address stays the no account note", func(t *testing.T) {
		r := newCredRig(t)
		ctx := context.Background()
		if err := r.svc.RequestReset(ctx, "sess-a", credEmail); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(46 * time.Second)
		if err := r.svc.ResendCode(ctx, "sess-a"); err != nil {
			t.Fatal(err)
		}
		if r.q.codes[1].delivery != service.DeliveryNoAccount || r.mail.last(t).msg.Kind != mailer.KindNoAccount {
			t.Errorf("row = %+v email = %+v", r.q.codes[1], r.mail.last(t))
		}
	})

	t.Run("delivery is recomputed: an address that gained a password gets the exists note", func(t *testing.T) {
		r := newCredRig(t)
		ctx := context.Background()
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.q.addAccount(credEmail, credStrPtr("$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5"))
		r.clk.Advance(46 * time.Second)
		if err := r.svc.ResendCode(ctx, "sess-a"); err != nil {
			t.Fatal(err)
		}
		if r.q.codes[1].delivery != service.DeliveryAccountExists || r.mail.last(t).msg.Kind != mailer.KindAccountExists {
			t.Errorf("row = %+v", r.q.codes[1])
		}
	})
}

func TestResendCodeLimits(t *testing.T) {
	ctx := context.Background()

	t.Run("no row for this browser", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.ResendCode(ctx, "sess-none"); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
		if len(r.mail.sent) != 0 {
			t.Error("nothing may be sent")
		}
	})

	t.Run("a row older than the resume window", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		r.clk.Advance(61 * time.Minute)
		if err := r.svc.ResendCode(ctx, "sess-a"); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
	})

	t.Run("inside the cooldown", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		before := len(r.q.codes)
		if err := r.svc.ResendCode(ctx, "sess-a"); !errors.Is(err, service.ErrRateLimited) {
			t.Fatalf("err = %v, want ErrRateLimited", err)
		}
		if len(r.q.codes) != before || len(r.mail.sent) != 1 {
			t.Error("a refused resend must not write or send")
		}
	})

	t.Run("the hourly count is shared with the first send", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 4; i++ {
			r.clk.Advance(46 * time.Second)
			if err := r.svc.ResendCode(ctx, "sess-a"); err != nil {
				t.Fatalf("resend %d: %v", i+1, err)
			}
		}
		r.clk.Advance(46 * time.Second)
		if err := r.svc.ResendCode(ctx, "sess-a"); !errors.Is(err, service.ErrTooManyCodes) {
			t.Fatalf("err = %v, want ErrTooManyCodes", err)
		}
	})

	t.Run("a sign up row without a password hash cannot be reissued as a code", func(t *testing.T) {
		r := newCredRig(t)
		r.q.codes = append(r.q.codes, credCodeRow{
			id: 1, session: "sess-a", purpose: service.PurposeSignUp, delivery: service.DeliveryCode,
			email: credEmail, codeHash: strings.Repeat("a", 64), created: r.clk.Now(), expires: r.clk.Now().Add(time.Minute),
		})
		r.clk.Advance(time.Minute)
		if err := r.svc.ResendCode(ctx, "sess-a"); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
		if len(r.mail.sent) != 0 {
			t.Error("nothing may be sent")
		}
	})

	t.Run("a consumed row", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
			t.Fatal(err)
		}
		used := r.clk.Now()
		r.q.codes[0].used = &used
		r.clk.Advance(46 * time.Second)
		if err := r.svc.ResendCode(ctx, "sess-a"); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
	})
}

func TestCredentialMessagesPlainPunctuation(t *testing.T) {
	messages := []string{
		service.WrongCredentialsMessage, service.WrongCodeMessage, service.TooManyCodesMessage,
		service.InvalidEmailMessage, service.PasswordTooShortMessage, service.PasswordTooLongMessage,
		service.PasswordCommonMessage, service.PasswordIsEmailMessage,
		service.SignInLockedMessage(time.Minute), service.SignInLockedMessage(5 * time.Minute),
		service.SignInLockedMessage(59 * time.Minute),
	}
	for _, m := range messages {
		for _, r := range m {
			if r > 0x7E || (r < 0x20) {
				t.Errorf("message %q holds a character outside plain ASCII: U+%04X", m, r)
			}
		}
	}
	if !strings.Contains(service.PasswordIsEmailMessage, "'") {
		t.Error("the apostrophe in the email-as-password sentence must be the plain one")
	}
	if service.WrongCredentialsMessage != "Email or password is wrong." {
		t.Errorf("wrong credentials sentence = %q", service.WrongCredentialsMessage)
	}
}
