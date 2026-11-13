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
	// Six Arabic-Indic digits: six runes, not six ASCII digits.
	arabicIndicDigits = string([]rune{0x661, 0x662, 0x663, 0x664, 0x665, 0x666})
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
	verified     *time.Time
}

type credAccount struct {
	id           int64
	email        string
	passwordHash *string
	deleted      bool
	epoch        int64
}

type credSession struct {
	account *int64
	revoked bool
	epoch   int64
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

	consumeZero    bool
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

func (f *credFakeQuerier) RefundAuthEmailSend(_ context.Context, arg sqlcgen.RefundAuthEmailSendParams) error {
	f.log("RefundAuthEmailSend")
	c, ok := f.claims[arg.Email]
	if !ok || !c.last.Equal(arg.ClaimedAt) {
		return nil
	}
	c.last = arg.Released
	if c.sends > 0 {
		c.sends--
	}
	return nil
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

func (f *credFakeQuerier) RetireAuthCode(_ context.Context, id int64) error {
	f.log("RetireAuthCode")
	for i := range f.codes {
		if f.codes[i].id == id && f.codes[i].used == nil {
			u := f.clk.Now()
			f.codes[i].used = &u
		}
	}
	return nil
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
	if f.consumeZero {
		return 0, nil
	}
	for i := range f.codes {
		r := &f.codes[i]
		if r.id == id && r.used == nil && r.expires.After(f.clk.Now()) {
			u := f.clk.Now()
			r.used = &u
			// Review fix round 3 (SK2-01): the life ends at the spend and the
			// password hash is cleared, as the real query does.
			r.expires = u
			r.passwordHash = nil
			return 1, nil
		}
	}
	return 0, nil
}

// MarkAuthCodeVerified follows the real query: a live reset row of the old
// session that was emailed a code gets verified_at (a second mark restarts the
// window) and moves to the new session id.
func (f *credFakeQuerier) MarkAuthCodeVerified(_ context.Context, arg sqlcgen.MarkAuthCodeVerifiedParams) (int64, error) {
	f.log("MarkAuthCodeVerified")
	for i := range f.codes {
		r := &f.codes[i]
		if r.id == arg.ID && r.session == arg.OldSessionID && r.used == nil && r.expires.After(f.clk.Now()) &&
			r.purpose == service.PurposeReset && r.delivery == service.DeliveryCode {
			v := arg.VerifiedAt
			r.verified = &v
			r.session = arg.NewSessionID
			return 1, nil
		}
	}
	return 0, nil
}

// ClearAuthCodeVerified follows the real query: the newest unused reset row of
// the session loses its verified_at.
func (f *credFakeQuerier) ClearAuthCodeVerified(_ context.Context, sessionID string) (int64, error) {
	f.log("ClearAuthCodeVerified")
	for i := len(f.codes) - 1; i >= 0; i-- {
		r := &f.codes[i]
		if r.session == sessionID && r.purpose == service.PurposeReset && r.used == nil {
			if r.verified == nil {
				return 0, nil
			}
			r.verified = nil
			return 1, nil
		}
	}
	return 0, nil
}

// ConsumeVerifiedResetCode follows the real query: the spend of a verified reset
// row inside the confirm window, with the same equal used and expires pair as
// ConsumeAuthCode.
func (f *credFakeQuerier) ConsumeVerifiedResetCode(_ context.Context, arg sqlcgen.ConsumeVerifiedResetCodeParams) (int64, error) {
	f.log("ConsumeVerifiedResetCode")
	if f.consumeZero {
		return 0, nil
	}
	newest := int64(-1) // the newest row of the session, used or not (review fix IN-07)
	for i := range f.codes {
		if f.codes[i].session == arg.SessionID && f.codes[i].id > newest {
			newest = f.codes[i].id
		}
	}
	for i := range f.codes {
		r := &f.codes[i]
		if r.id == arg.ID && r.id == newest && r.session == arg.SessionID && r.used == nil &&
			r.purpose == service.PurposeReset && r.delivery == service.DeliveryCode &&
			r.verified != nil && r.verified.After(arg.VerifiedAfter) {
			u := f.clk.Now()
			r.used = &u
			r.expires = u
			r.passwordHash = nil
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
			if r.verified != nil {
				row.VerifiedAt = pgtype.Timestamptz{Time: *r.verified, Valid: true}
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

// MarkAuthCodePasswordSet marks the spent row as a password set (review fix
// round 4, skeptic SK3-03), as the real query does.
func (f *credFakeQuerier) MarkAuthCodePasswordSet(_ context.Context, id int64) error {
	f.log("MarkAuthCodePasswordSet")
	for i := range f.codes {
		r := &f.codes[i]
		if r.id == id && r.purpose == service.PurposeSignUp && r.used != nil && r.passwordHash == nil {
			r.purpose = service.PurposeReset
		}
	}
	return nil
}

// DeleteOtherAuthCodesForEmail is the confirm's delete: every row of the
// address but the spent one (review fix round 3, skeptic SK2-01).
func (f *credFakeQuerier) DeleteOtherAuthCodesForEmail(_ context.Context, arg sqlcgen.DeleteOtherAuthCodesForEmailParams) error {
	f.log("DeleteOtherAuthCodesForEmail")
	kept := f.codes[:0]
	for _, r := range f.codes {
		if r.email != arg.Email || r.id == arg.KeepID {
			kept = append(kept, r)
		}
	}
	f.codes = kept
	return nil
}

// LockCredentialAddress stands for the advisory lock of the confirm
// transactions (review fix skeptic SK-05); the fake runs one call at a time.
func (f *credFakeQuerier) LockCredentialAddress(_ context.Context, key string) error {
	f.log("LockCredentialAddress")
	if key == "" {
		return fmt.Errorf("fake: empty address lock key")
	}
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

// ReserveSignInTry follows the SQL: a row locked at now is refused with no
// write (pgx.ErrNoRows); otherwise the count restarts only when both the last
// failure and the end of the last lock are at or before the forget cutoff.
func (f *credFakeQuerier) ReserveSignInTry(_ context.Context, arg sqlcgen.ReserveSignInTryParams) (sqlcgen.ReserveSignInTryRow, error) {
	f.log("ReserveSignInTry")
	if arg.Scope != "sign_in" {
		return sqlcgen.ReserveSignInTryRow{}, fmt.Errorf("fake: bad throttle scope %q", arg.Scope)
	}
	t, ok := f.throttles[arg.KeyHash]
	if ok && t.locked != nil && t.locked.After(arg.Now) {
		return sqlcgen.ReserveSignInTryRow{}, pgx.ErrNoRows
	}
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
	row := sqlcgen.ReserveSignInTryRow{Failures: n, LastFailureAt: arg.Now}
	if locked != nil {
		row.LockedUntil = pgtype.Timestamptz{Time: *locked, Valid: true}
	}
	return row, nil
}

// RefundSignInTry follows the SQL (review fix skeptic SK-02): only while the
// row still holds the reserved try, failures goes down by one and the lock goes.
func (f *credFakeQuerier) RefundSignInTry(_ context.Context, arg sqlcgen.RefundSignInTryParams) (int64, error) {
	f.log("RefundSignInTry")
	t, ok := f.throttles[arg.KeyHash]
	if !ok || t.failures != arg.Failures || t.failures <= 0 || !t.last.Equal(arg.LastFailureAt) {
		return 0, nil
	}
	t.failures--
	t.locked = nil
	return 1, nil
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
	return sqlcgen.GetAccountForSignInRow{ID: a.id, PasswordHash: a.passwordHash, PasswordEpoch: a.epoch}, nil
}

func (f *credFakeQuerier) LockAccountForPassword(_ context.Context, email string) (sqlcgen.LockAccountForPasswordRow, error) {
	f.log("LockAccountForPassword")
	a, ok := f.accounts[email]
	if !ok || a.deleted {
		return sqlcgen.LockAccountForPasswordRow{}, pgx.ErrNoRows
	}
	return sqlcgen.LockAccountForPasswordRow{ID: a.id, PasswordHash: a.passwordHash, PasswordEpoch: a.epoch}, nil
}

func (f *credFakeQuerier) accountByID(id int64) *credAccount {
	for _, a := range f.accounts {
		if a.id == id && !a.deleted {
			return a
		}
	}
	return nil
}

func (f *credFakeQuerier) SetAccountPasswordNewEpoch(_ context.Context, arg sqlcgen.SetAccountPasswordNewEpochParams) (int64, error) {
	f.log("SetAccountPasswordNewEpoch")
	a := f.accountByID(arg.ID)
	if a == nil {
		return 0, pgx.ErrNoRows
	}
	h := arg.PasswordHash
	a.passwordHash = &h
	a.epoch++
	return a.epoch, nil
}

func (f *credFakeQuerier) SetAccountPasswordIfEpoch(_ context.Context, arg sqlcgen.SetAccountPasswordIfEpochParams) (int64, error) {
	f.log("SetAccountPasswordIfEpoch")
	a := f.accountByID(arg.ID)
	if a == nil || a.epoch != arg.PasswordEpoch {
		return 0, nil
	}
	h := arg.PasswordHash
	a.passwordHash = &h
	return 1, nil
}

// BindSessionAccountAtEpoch follows the SQL: the account must be live at the
// given epoch, and an existing row is never moved to another account nor
// revived. It logs as BindSessionAccount so the call order checks still read.
func (f *credFakeQuerier) BindSessionAccountAtEpoch(_ context.Context, arg sqlcgen.BindSessionAccountAtEpochParams) (int64, error) {
	f.log("BindSessionAccount:" + arg.SessionID)
	a := f.accountByID(arg.AccountID)
	if a == nil || a.epoch != arg.PasswordEpoch {
		return 0, nil
	}
	s, ok := f.sessions[arg.SessionID]
	if !ok {
		id := arg.AccountID
		f.sessions[arg.SessionID] = &credSession{account: &id, epoch: a.epoch}
		return 1, nil
	}
	if s.revoked || (s.account != nil && *s.account != arg.AccountID) {
		return 0, nil
	}
	id := arg.AccountID
	s.account = &id
	s.epoch = a.epoch
	return 1, nil
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
		if a.id == *s.account && !a.deleted && a.epoch == s.epoch {
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

// checkReset runs step one of Forgot password with a predictable fresh session
// id, the browser's id plus "-fresh" (review fix WR-02: a right code moves the
// browser to a new id, and step two must then be asked on that id).
func checkReset(svc *service.CredentialService, session, code string) (string, error) {
	return svc.CheckResetCode(context.Background(), session, code, func() string { return session + "-fresh" })
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

// TestSignUpBusyKeepsClaim is IN-S01 of the review: when the hasher is busy,
// the per address claim is given back, so a busy hasher spends neither the 45
// second cooldown nor an hourly send, and the person's immediate retry goes
// through. A refused claim still costs no hash (TestSignUpSendLimits).
func TestSignUpBusyKeepsClaim(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	r.hasher.hashErr = auth.ErrHasherBusy
	if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); !errors.Is(err, service.ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	r.hasher.hashErr = nil
	if err := r.svc.SignUp(ctx, "sess-a", credEmail, credPassword); err != nil {
		t.Fatalf("the retry right after a busy hasher: %v", err)
	}
	if c := r.q.claims[credEmail]; c == nil || c.sends != 1 {
		t.Errorf("hourly sends after a busy try and a good one = %+v, want 1", c)
	}
}

// TestFailedSendRetiresRow is the other half of IN-S01: when the email does
// not go out (the ceiling refused it or the provider failed), the row is marked
// used, so a reload of /login does not show a code screen for an email that
// was never sent. The same holds for every branch of Create account and Forgot
// password.
func TestFailedSendRetiresRow(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		run  func(r *credRig) error
	}{
		{"sign up, new address", func(r *credRig) error { return r.svc.SignUp(ctx, "sess-a", credEmail, credPassword) }},
		{"sign up, account exists", func(r *credRig) error {
			r.addPasswordAccount(t, credEmail, credPassword)
			return r.svc.SignUp(ctx, "sess-a", credEmail, credPassword)
		}},
		{"reset, known", func(r *credRig) error {
			r.addPasswordAccount(t, credEmail, credPassword)
			return r.svc.RequestReset(ctx, "sess-a", credEmail)
		}},
		{"reset, unknown", func(r *credRig) error { return r.svc.RequestReset(ctx, "sess-a", "ghost@example.com") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, sendErr := range []error{mailer.ErrDailyCeiling, errors.New("provider said no")} {
				r := newCredRig(t)
				r.mail.err = sendErr
				if err := tc.run(r); err == nil {
					t.Fatalf("a failed send answered nil")
				}
				if _, ok, err := r.svc.LiveCode(ctx, "sess-a"); err != nil || ok {
					t.Errorf("after a failed send (%v) the code screen would show again: ok=%v err=%v", sendErr, ok, err)
				}
			}
		})
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
	for _, m := range messages {
		if strings.ContainsRune(m, 0x2019) {
			t.Errorf("message %q holds a curly apostrophe", m)
		}
	}
}

// ---------------------------------------------------------------- Task 2 ----

// addPasswordAccount makes an account whose password is credPassword (or the
// given one), hashed with the rig's cheap hasher.
func (r *credRig) addPasswordAccount(t *testing.T, email, password string) *credAccount {
	t.Helper()
	h, err := r.hasher.inner.Hash(context.Background(), auth.NormalizePassword(password))
	if err != nil {
		t.Fatal(err)
	}
	return r.q.addAccount(email, &h)
}

// signUpAndCode runs Create account on a session and returns the emailed code.
func (r *credRig) signUpAndCode(t *testing.T, session, email, password string) string {
	t.Helper()
	if err := r.svc.SignUp(context.Background(), session, email, password); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	return r.codeSent(t)
}

func (r *credRig) callIndex(prefix string) int {
	for i, c := range r.q.calls {
		if strings.HasPrefix(c, prefix) {
			return i
		}
	}
	return -1
}

func TestConfirmSignUpCreatesAccountAndRotates(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
	// A stale throttle row for the address is cleared by the confirm.
	r.q.throttles[r.svc.ThrottleKey(credEmail)] = &credThrottle{failures: 3, last: r.clk.Now()}

	// A pasted code with spaces and a no break space still works.
	typed := code[:3] + " " + credNBSP + code[3:]
	res, err := r.svc.ConfirmSignUp(ctx, "sess-a", typed, credSessionIDs("fresh"))
	if err != nil {
		t.Fatalf("ConfirmSignUp: %v", err)
	}
	if res.SessionID != "fresh-1" || res.SessionID == "sess-a" {
		t.Errorf("session id = %q, want a freshly drawn id", res.SessionID)
	}
	acct := r.q.accounts[credEmail]
	if acct == nil || res.AccountID != acct.id {
		t.Fatalf("account = %+v result = %+v", acct, res)
	}
	if acct.passwordHash == nil {
		t.Fatal("the password was not set")
	}
	if ok, _, err := r.hasher.inner.Verify(ctx, *acct.passwordHash, credPassword); err != nil || !ok {
		t.Errorf("the stored password does not verify: ok=%v err=%v", ok, err)
	}
	if s := r.q.sessions["fresh-1"]; s == nil || s.account == nil || *s.account != acct.id {
		t.Errorf("the fresh session is not bound to the account")
	}
	if s := r.q.sessions["sess-a"]; s != nil && s.account != nil {
		t.Errorf("the pre sign in id must never be bound")
	}
	if len(r.q.throttles) != 0 {
		t.Error("the throttle was not cleared")
	}
	// Amended 2026-10-29 by the Phase 7.1 review fix round 3 (skeptic SK2-01):
	// every other code row of the address is deleted and the spent row is kept,
	// used and with no password hash, so the browser can be told after a lost
	// answer that its code was confirmed.
	if len(r.q.codes) != 1 || r.q.codes[0].used == nil || r.q.codes[0].passwordHash != nil {
		t.Errorf("only the spent row of the address may be kept, used and with no hash: %+v", r.q.codes)
	}
	if consume, del := r.callIndex("ConsumeAuthCode"), r.callIndex("DeleteOtherAuthCodesForEmail"); consume < 0 || del < consume {
		t.Errorf("delete (%d) must follow consume (%d)", del, consume)
	}
	if got := r.q.count("BindSessionAccount"); got != 1 {
		t.Errorf("bind calls = %d, want 1", got)
	}
}

func TestConfirmSignUpLegacyKeepsAccount(t *testing.T) {
	ctx := context.Background()
	t.Run("an account made with the old link keeps its id", func(t *testing.T) {
		r := newCredRig(t)
		legacy := r.q.addAccount(credEmail, nil)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		res, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("fresh"))
		if err != nil {
			t.Fatal(err)
		}
		if res.AccountID != legacy.id || len(r.q.accounts) != 1 {
			t.Errorf("result = %+v, accounts = %d; the legacy account and its reports must be kept", res, len(r.q.accounts))
		}
		if legacy.passwordHash == nil {
			t.Error("the password was not set on the legacy account")
		}
		// Review WR-S02: a session the old binary bound from an emailed link is
		// ended, and the password epoch moves, so it never passes the gate.
		if r.q.count("RevokeAccountSessions") != 1 || legacy.epoch != 1 {
			t.Errorf("legacy account: revokes %d, epoch %d; want 1 and 1", r.q.count("RevokeAccountSessions"), legacy.epoch)
		}
		if s := r.q.sessions[res.SessionID]; s == nil || s.epoch != legacy.epoch {
			t.Error("the fresh session must be bound at the new epoch")
		}
	})

	t.Run("an account that gained a password meanwhile has every session ended", func(t *testing.T) {
		r := newCredRig(t)
		acct := r.q.addAccount(credEmail, nil)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		old := "$argon2id$v=19$m=64,t=1,p=1$c2FsdA$a2V5"
		acct.passwordHash = &old
		r.q.bindSession("other-browser", acct.id)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("fresh")); err != nil {
			t.Fatal(err)
		}
		// Review CR-S02: inside one transaction the password and its epoch move
		// first, under the account row lock, then every session ends.
		set, logout := r.callIndex("SetAccountPasswordNewEpoch"), r.callIndex("RevokeAccountSessions")
		if set < 0 || logout < set {
			t.Errorf("the password (%d) must be set with a new epoch before the sessions end (%d)", set, logout)
		}
		if !r.q.sessions["other-browser"].revoked {
			t.Error("the other browser's session must be revoked")
		}
	})
}

func TestConfirmSignUpWrongCode(t *testing.T) {
	ctx := context.Background()

	t.Run("a wrong code bumps once", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		_, err := r.svc.ConfirmSignUp(ctx, "sess-a", credWrongCode(code), credSessionIDs("f"))
		if !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v", err)
		}
		if r.q.codes[0].attempts != 1 || r.q.count("BumpAuthCodeAttempt") != 1 {
			t.Errorf("attempts = %d", r.q.codes[0].attempts)
		}
		if len(r.q.accounts) != 0 {
			t.Error("a wrong code must not create an account")
		}
	})

	t.Run("a code that is not 6 digits never bumps", func(t *testing.T) {
		r := newCredRig(t)
		r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		for _, bad := range []string{"", "12345", "1234567", "12345a", "12 34", arabicIndicDigits} {
			if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", bad, credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
				t.Errorf("%q: err = %v", bad, err)
			}
		}
		if r.q.count("BumpAuthCodeAttempt") != 0 {
			t.Error("a malformed code must not spend a try")
		}
	})

	t.Run("after five wrong codes even the right one is refused", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		for i := 0; i < 5; i++ {
			if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", credWrongCode(code), credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
				t.Fatalf("try %d: %v", i+1, err)
			}
		}
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("the right code after the budget: %v", err)
		}
		if len(r.q.accounts) != 0 {
			t.Error("no account may exist")
		}
	})

	t.Run("an expired code", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		r.clk.Advance(11 * time.Minute)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("an account exists row can never be confirmed", func(t *testing.T) {
		r := newCredRig(t)
		now := r.clk.Now()
		r.q.codes = append(r.q.codes, credCodeRow{
			id: 1, session: "sess-a", purpose: service.PurposeSignUp, delivery: service.DeliveryAccountExists,
			email: credEmail, codeHash: auth.CodeHash(credCodeKey, "signup", credEmail, "123456"),
			created: now, expires: now.Add(10 * time.Minute),
		})
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", "123456", credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v", err)
		}
		if len(r.q.accounts) != 0 || r.q.count("ConsumeAuthCode") != 0 {
			t.Error("nothing may be consumed or created")
		}
	})

	t.Run("a parallel submit that already consumed the row", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		r.q.consumeZero = true
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v", err)
		}
		if len(r.q.accounts) != 0 {
			t.Error("no account may be created when the consume lost")
		}
	})

	t.Run("a code typed in another browser", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-b", code, credSessionIDs("f")); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestConfirmSignUpCleansReplacedSessionPush(t *testing.T) {
	ctx := context.Background()

	t.Run("bound to a different account", func(t *testing.T) {
		r := newCredRig(t)
		other := r.addPasswordAccount(t, "other@example.com", credPassword)
		r.q.bindSession("sess-a", other.id)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		res, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("fresh"))
		if err != nil {
			t.Fatal(err)
		}
		if len(r.q.pushDeleted) != 1 || r.q.pushDeleted[0] != "sess-a" {
			t.Errorf("push subscriptions deleted for %v, want [sess-a]", r.q.pushDeleted)
		}
		if !r.q.sessions["sess-a"].revoked {
			t.Error("the replaced session must be revoked so its cookie stops working")
		}
		// Amended 2026-10-29 by the Phase 7.1 review fix round 2 (skeptic
		// SK-07): the fresh id is bound first and the replaced session is
		// cleaned after it, push subscriptions before the revoke, so a refused
		// bind leaves the previous account's session and alerts alone.
		push, revoke, bind := r.callIndex("DeletePushSubscriptionsForSession"), r.callIndex("RevokeSession"), r.callIndex("BindSessionAccount")
		if !(bind >= 0 && push > bind && revoke > push) {
			t.Errorf("order: bind %d, push %d, revoke %d; want bind, then push, then revoke", bind, push, revoke)
		}
		if res.SessionID != "fresh-1" {
			t.Errorf("session = %q", res.SessionID)
		}
	})

	t.Run("unbound", func(t *testing.T) {
		r := newCredRig(t)
		r.q.sessions["sess-a"] = &credSession{}
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("fresh")); err != nil {
			t.Fatal(err)
		}
		if r.q.count("DeletePushSubscriptionsForSession") != 0 || r.q.count("RevokeSession") != 0 {
			t.Error("an unbound session is left alone")
		}
	})

	t.Run("bound to the same account", func(t *testing.T) {
		r := newCredRig(t)
		acct := r.q.addAccount(credEmail, nil)
		r.q.bindSession("sess-a", acct.id)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("fresh")); err != nil {
			t.Fatal(err)
		}
		if r.q.count("DeletePushSubscriptionsForSession") != 0 || r.q.count("RevokeSession") != 0 {
			t.Error("a session of the same account is left alone")
		}
	})
}

func TestSignInSuccessRotates(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	acct := r.addPasswordAccount(t, credEmail, credPassword)
	key := r.svc.ThrottleKey(credEmail)
	r.q.throttles[key] = &credThrottle{failures: 2, last: r.clk.Now()}

	res, err := r.svc.SignIn(ctx, "sess-c", " Ravi@Example.com", credPassword, credSessionIDs("fresh"))
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if res.AccountID != acct.id || res.SessionID != "fresh-1" {
		t.Errorf("result = %+v", res)
	}
	if s := r.q.sessions["sess-c"]; s != nil && s.account != nil {
		t.Error("the request's own id must never be bound")
	}
	if s := r.q.sessions["fresh-1"]; s == nil || s.account == nil || *s.account != acct.id {
		t.Error("the fresh id is not bound")
	}
	if _, left := r.q.throttles[key]; left {
		t.Error("a good sign in clears the throttle")
	}
	if len(r.hasher.verified) != 1 {
		t.Errorf("verify calls = %d, want 1", len(r.hasher.verified))
	}
}

func TestSignInWrongPasswordRecordsFailure(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	r.addPasswordAccount(t, credEmail, credPassword)
	_, err := r.svc.SignIn(ctx, "sess-c", credEmail, "some other password 1", credSessionIDs("f"))
	if !errors.Is(err, service.ErrWrongCredentials) {
		t.Fatalf("err = %v", err)
	}
	th := r.q.throttles[r.svc.ThrottleKey(credEmail)]
	if th == nil || th.failures != 1 {
		t.Fatalf("throttle = %+v, want one failure", th)
	}
	if r.q.count("BindSessionAccount") != 0 {
		t.Error("a wrong password must not bind a session")
	}
	if strings.Contains(err.Error(), credEmail) {
		t.Error("the error carries the address")
	}
}

func TestSignInUnknownUsesDummyHash(t *testing.T) {
	r := newCredRig(t)
	_, err := r.svc.SignIn(context.Background(), "sess-c", "nobody@example.com", credPassword, credSessionIDs("f"))
	if !errors.Is(err, service.ErrWrongCredentials) {
		t.Fatalf("err = %v", err)
	}
	if len(r.hasher.verified) != 1 || r.hasher.verified[0].encoded != r.hasher.DummyHash() {
		t.Errorf("verified = %+v, want exactly one verify against the dummy hash", r.hasher.verified)
	}
	if th := r.q.throttles[r.svc.ThrottleKey("nobody@example.com")]; th == nil || th.failures != 1 {
		t.Errorf("an unknown address is throttled like a known one: %+v", th)
	}
}

func TestSignInPasswordlessUsesDummyHash(t *testing.T) {
	r := newCredRig(t)
	r.q.addAccount(credEmail, nil)
	_, err := r.svc.SignIn(context.Background(), "sess-c", credEmail, credPassword, credSessionIDs("f"))
	if !errors.Is(err, service.ErrWrongCredentials) {
		t.Fatalf("err = %v", err)
	}
	if len(r.hasher.verified) != 1 || r.hasher.verified[0].encoded != r.hasher.DummyHash() {
		t.Errorf("verified = %+v, want exactly one verify against the dummy hash (W3)", r.hasher.verified)
	}
	if th := r.q.throttles[r.svc.ThrottleKey(credEmail)]; th == nil || th.failures != 1 {
		t.Errorf("throttle = %+v", th)
	}
}

func TestSignInNormalizesPassword(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	code := r.signUpAndCode(t, "sess-a", credEmail, credDecomposedPassword)
	if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("up")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.SignIn(ctx, "sess-b", credEmail, credComposedPassword, credSessionIDs("in")); err != nil {
		t.Fatalf("a password typed decomposed must sign in when typed composed: %v", err)
	}
	if len(r.hasher.hashed) != 1 || r.hasher.hashed[0] != credComposedPassword {
		t.Errorf("hash input = %q, want the NFKC form", r.hasher.hashed)
	}
	for _, v := range r.hasher.verified {
		if v.password != credComposedPassword {
			t.Errorf("verify input = %q, want the NFKC form", v.password)
		}
	}
	if len(r.hasher.verified) == 0 {
		t.Error("sign in verified nothing")
	}
}

func TestSignInHashBusy(t *testing.T) {
	r := newCredRig(t)
	r.addPasswordAccount(t, credEmail, credPassword)
	r.hasher.verifyErr = auth.ErrHasherBusy
	_, err := r.svc.SignIn(context.Background(), "sess-c", credEmail, credPassword, credSessionIDs("f"))
	if !errors.Is(err, service.ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	// The try is counted before the check (Phase 7.1 review CR-S01).
	// Amended 2026-10-29 by the Phase 7.1 review fix round 2 (skeptic SK-02): a
	// busy hasher checked no password, so it gives the counted try back, the
	// same for every address, and binds nothing.
	if r.q.count("ReserveSignInTry") != 1 || r.q.count("RefundSignInTry") != 1 || r.q.count("BindSessionAccount") != 0 {
		t.Errorf("a busy sign in: tries counted %d, given back %d, binds %d; want 1, 1 and 0",
			r.q.count("ReserveSignInTry"), r.q.count("RefundSignInTry"), r.q.count("BindSessionAccount"))
	}
	if th, ok := r.q.throttles[r.svc.ThrottleKey(credEmail)]; ok && (th.failures != 0 || th.locked != nil) {
		t.Errorf("a busy sign in left failures %d and a lock %v; want neither", th.failures, th.locked)
	}
}

func TestConfirmDeletesOtherCodesForAddress(t *testing.T) {
	ctx := context.Background()

	t.Run("sign up", func(t *testing.T) {
		r := newCredRig(t)
		codeA := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		r.clk.Advance(46 * time.Second)
		codeB := r.signUpAndCode(t, "sess-b", credEmail, credPassword)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", codeA, credSessionIDs("f")); err != nil {
			t.Fatal(err)
		}
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-b", codeB, credSessionIDs("g")); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("the other browser's pending code must be dead: %v", err)
		}
	})

	t.Run("reset", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		if err := r.svc.RequestReset(ctx, "sess-a", credEmail); err != nil {
			t.Fatal(err)
		}
		codeA := r.codeSent(t)
		r.clk.Advance(46 * time.Second)
		if err := r.svc.RequestReset(ctx, "sess-b", credEmail); err != nil {
			t.Fatal(err)
		}
		codeB := r.codeSent(t)
		// Quick 261113-r5w: the other browser confirmed its code first, and
		// then the first browser's whole two steps spent the address.
		freshB, err := checkReset(r.svc, "sess-b", codeB)
		if err != nil {
			t.Fatal(err)
		}
		freshA, err := checkReset(r.svc, "sess-a", codeA)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.svc.ConfirmReset(ctx, freshA, "a brand new password 55"); err != nil {
			t.Fatal(err)
		}
		if err := r.svc.ConfirmReset(ctx, freshB, "another new password 66"); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("the other browser's confirmed code must be dead: %v", err)
		}
		if _, err := checkReset(r.svc, freshB, codeB); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("the other browser's pending code must be dead: %v", err)
		}
	})
}

func TestSignInLockedRefusedBeforeHash(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	r.addPasswordAccount(t, credEmail, credPassword)
	for i := 1; i <= 6; i++ {
		_, err := r.svc.SignIn(ctx, "sess-c", credEmail, "wrong password "+fmt.Sprint(i), credSessionIDs("f"))
		if !errors.Is(err, service.ErrWrongCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	verifies, lookups := len(r.hasher.verified), r.q.count("GetAccountForSignIn")
	before := *r.q.throttles[r.svc.ThrottleKey(credEmail)]

	_, err := r.svc.SignIn(ctx, "sess-c", credEmail, credPassword, credSessionIDs("f"))
	var locked service.SignInLockedError
	if !errors.As(err, &locked) {
		t.Fatalf("the 7th attempt with the right password: err = %v, want SignInLockedError", err)
	}
	if locked.Wait <= 0 || locked.Wait > 5*time.Minute {
		t.Errorf("wait = %v, want up to 5 minutes", locked.Wait)
	}
	after := *r.q.throttles[r.svc.ThrottleKey(credEmail)]
	if len(r.hasher.verified) != verifies || r.q.count("GetAccountForSignIn") != lookups ||
		after.failures != before.failures || !after.last.Equal(before.last) || !after.locked.Equal(*before.locked) {
		t.Error("a locked address is refused before any hashing or lookup, and the refused try changes nothing")
	}
	// The lock ends: the right password works again.
	r.clk.Advance(5*time.Minute + time.Second)
	if _, err := r.svc.SignIn(ctx, "sess-c", credEmail, credPassword, credSessionIDs("g")); err != nil {
		t.Fatalf("after the lock: %v", err)
	}
}

func TestSignInRehashesOldParams(t *testing.T) {
	ctx := context.Background()
	oldHasher := auth.NewHasher(auth.Params{MemoryKiB: 64, Time: 2, Threads: 1})

	t.Run("an old hash is upgraded", func(t *testing.T) {
		r := newCredRig(t)
		old, err := oldHasher.Hash(ctx, auth.NormalizePassword(credPassword))
		if err != nil {
			t.Fatal(err)
		}
		acct := r.q.addAccount(credEmail, &old)
		if _, err := r.svc.SignIn(ctx, "sess-c", credEmail, credPassword, credSessionIDs("f")); err != nil {
			t.Fatal(err)
		}
		if *acct.passwordHash == old {
			t.Fatal("the hash was not upgraded")
		}
		ok, needs, err := r.hasher.inner.Verify(ctx, *acct.passwordHash, credPassword)
		if err != nil || !ok || needs {
			t.Errorf("new hash: ok=%v needsRehash=%v err=%v", ok, needs, err)
		}
	})

	t.Run("a failed upgrade never fails a good sign in", func(t *testing.T) {
		r := newCredRig(t)
		old, err := oldHasher.Hash(ctx, auth.NormalizePassword(credPassword))
		if err != nil {
			t.Fatal(err)
		}
		acct := r.q.addAccount(credEmail, &old)
		r.hasher.hashErr = auth.ErrHasherBusy
		if _, err := r.svc.SignIn(ctx, "sess-c", credEmail, credPassword, credSessionIDs("f")); err != nil {
			t.Fatalf("err = %v", err)
		}
		if *acct.passwordHash != old {
			t.Error("the stored hash must be unchanged")
		}
	})
}

// resetCodeFor asks Forgot password for credEmail on the browser and returns the
// emailed code. The account must exist.
func (r *credRig) resetCodeFor(t *testing.T, session string) string {
	t.Helper()
	if err := r.svc.RequestReset(context.Background(), session, credEmail); err != nil {
		t.Fatalf("RequestReset: %v", err)
	}
	return r.codeSent(t)
}

// resetVerified runs step one of Forgot password for the browser and returns the
// session id the browser is moved to, which holds the confirm.
func (r *credRig) resetVerified(t *testing.T, session string) string {
	t.Helper()
	code := r.resetCodeFor(t, session)
	fresh, err := checkReset(r.svc, session, code)
	if err != nil {
		t.Fatalf("CheckResetCode: %v", err)
	}
	return fresh
}

func (r *credRig) newestCode(t *testing.T) *credCodeRow {
	t.Helper()
	if len(r.q.codes) == 0 {
		t.Fatal("no code rows")
	}
	return &r.q.codes[len(r.q.codes)-1]
}

// TestConfirmResetSetsPasswordAndEndsSessions is step two of Forgot password
// (quick 261113-r5w): with the code confirmed, the new password does everything
// the one step confirm did before.
func TestConfirmResetSetsPasswordAndEndsSessions(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	acct := r.addPasswordAccount(t, credEmail, credPassword)
	r.q.bindSession("phone", acct.id)
	r.q.bindSession("laptop", acct.id)
	r.q.throttles[r.svc.ThrottleKey(credEmail)] = &credThrottle{failures: 9, last: r.clk.Now()}
	epochBefore := acct.epoch

	fresh := r.resetVerified(t, "sess-d")
	newPassword := "a brand new password 55"
	if err := r.svc.ConfirmReset(ctx, fresh, newPassword); err != nil {
		t.Fatalf("ConfirmReset: %v", err)
	}
	if ok, _, err := r.hasher.inner.Verify(ctx, *acct.passwordHash, newPassword); err != nil || !ok {
		t.Errorf("the new password does not verify: ok=%v err=%v", ok, err)
	}
	if acct.epoch == epochBefore {
		t.Error("the password epoch must move")
	}
	for _, id := range []string{"phone", "laptop"} {
		if !r.q.sessions[id].revoked {
			t.Errorf("session %s must be revoked", id)
		}
	}
	if len(r.q.revokedAll) != 1 || r.q.revokedAll[0] != acct.id {
		t.Errorf("LogOutAllDevices calls = %v", r.q.revokedAll)
	}
	if r.q.count("DeleteAccountPushSubscriptions") != 1 {
		t.Error("push subscriptions of the account must be deleted")
	}
	if len(r.q.throttles) != 0 {
		t.Error("a reset clears the throttle")
	}
	if r.q.count("BindSessionAccount") != 0 {
		t.Error("a reset never signs the browser in")
	}
	// Amended 2026-10-29 by the Phase 7.1 review fix round 3 (skeptic SK2-01):
	// every other code row of the address is deleted and the spent row is kept,
	// used, so the browser can be told after a lost answer that its password
	// was set.
	if len(r.q.codes) != 1 || r.q.codes[0].used == nil {
		t.Errorf("only the spent row of the address may be kept, used: %+v", r.q.codes)
	}
	if spent, ok, err := r.svc.SpentCode(ctx, fresh); err != nil || !ok || spent.Purpose != service.PurposeReset || spent.Email != credEmail {
		t.Errorf("SpentCode after step two = %+v, %v, %v; want the reset of %s", spent, ok, err, credEmail)
	}
	if err := r.svc.ConfirmReset(ctx, fresh, "another new password 66"); !errors.Is(err, service.ErrWrongCode) {
		t.Errorf("step two twice: err = %v, want ErrWrongCode", err)
	}

	t.Run("a busy hasher does not burn the confirmed code", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		fresh := r.resetVerified(t, "sess-d")
		r.hasher.hashErr = auth.ErrHasherBusy
		if err := r.svc.ConfirmReset(ctx, fresh, "a brand new password 55"); !errors.Is(err, service.ErrBusy) {
			t.Fatalf("err = %v, want ErrBusy", err)
		}
		r.hasher.hashErr = nil
		if err := r.svc.ConfirmReset(ctx, fresh, "a brand new password 55"); err != nil {
			t.Fatalf("the same confirm must still work: %v", err)
		}
	})

	t.Run("a vanished account is the wrong code answer", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		fresh := r.resetVerified(t, "sess-d")
		a.deleted = true
		if err := r.svc.ConfirmReset(ctx, fresh, "a brand new password 55"); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
	})
}

// TestCheckResetCode is step one of Forgot password: the typed code confirms the
// row and nothing else changes.
func TestCheckResetCode(t *testing.T) {
	ctx := context.Background()

	t.Run("a right code verifies the row and spends nothing", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		r.q.bindSession("phone", a.id)
		before := *a.passwordHash
		epoch := a.epoch
		r.q.throttles[r.svc.ThrottleKey(credEmail)] = &credThrottle{failures: 3, last: r.clk.Now()}
		code := r.resetCodeFor(t, "sess-d")
		r.q.sessions["sess-d"] = &credSession{} // stored, unbound: the id the browser was on
		fresh, err := checkReset(r.svc, "sess-d", code)
		if err != nil {
			t.Fatalf("CheckResetCode: %v", err)
		}
		if fresh != "sess-d-fresh" {
			t.Errorf("fresh session id = %q, want the one newID drew", fresh)
		}
		row := r.newestCode(t)
		if row.verified == nil || row.used != nil || row.attempts != 1 {
			t.Errorf("row after step one: verified=%v used=%v attempts=%d; want verified, unused, 1 try", row.verified, row.used, row.attempts)
		}
		// Review fix WR-02: the confirm moved to the fresh id and the old id was
		// ended, so a session id planted before the code was typed holds nothing.
		if row.session != fresh {
			t.Errorf("the verified row is on session %q, want the fresh %q", row.session, fresh)
		}
		if lc, ok, _ := r.svc.LiveCode(ctx, "sess-d"); ok {
			t.Errorf("the old session id still shows a code screen: %+v", lc)
		}
		if err := r.svc.ConfirmReset(ctx, "sess-d", "a brand new password 55"); !errors.Is(err, service.ErrWrongCode) {
			t.Errorf("step two on the old session id: err = %v, want ErrWrongCode", err)
		}
		if !r.q.sessions["sess-d"].revoked {
			t.Error("the old session id was not revoked")
		}
		if len(r.q.pushDeleted) != 0 {
			t.Errorf("an unbound old session had its alerts deleted: %v", r.q.pushDeleted)
		}
		if *a.passwordHash != before || a.epoch != epoch || r.q.sessions["phone"].revoked || len(r.q.throttles) != 1 {
			t.Error("step one must not set a password, move the epoch, end a session or clear the throttle")
		}
		if len(r.hasher.hashed) != 0 {
			t.Errorf("step one hashed %q; it must hash nothing", r.hasher.hashed)
		}
		if r.q.count("BindSessionAccount") != 0 {
			t.Error("step one never signs the browser in")
		}
		// The same right code again restarts the confirm window.
		first := *row.verified
		r.clk.Advance(time.Minute)
		again, err := checkReset(r.svc, fresh, code)
		if err != nil {
			t.Fatalf("the right code twice: %v", err)
		}
		if !row.verified.Equal(first.Add(time.Minute)) {
			t.Errorf("verified_at is %v after the second right code, want %v (the window restarts)", *row.verified, first.Add(time.Minute))
		}
		// Each right code moves the browser on again; only the newest id holds the row.
		if row.session != again || again == fresh {
			t.Errorf("after the second right code the row is on %q (answer %q, first %q)", row.session, again, fresh)
		}
	})

	t.Run("a wrong code counts a try and the fifth locks", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		for i := 1; i <= 5; i++ {
			if id, err := checkReset(r.svc, "sess-d", credWrongCode(code)); !errors.Is(err, service.ErrWrongCode) || id != "" {
				t.Fatalf("wrong code %d: id %q err %v", i, id, err)
			}
		}
		row := r.newestCode(t)
		if row.attempts != 5 || row.verified != nil || row.session != "sess-d" {
			t.Errorf("after five wrong codes: attempts=%d verified=%v session=%q; want 5, none and the old session (a wrong code rotates nothing)", row.attempts, row.verified, row.session)
		}
		if _, err := checkReset(r.svc, "sess-d", code); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("the right code after the fifth wrong one: %v, want ErrWrongCode", err)
		}
		if row.verified != nil {
			t.Error("a spent try budget must not verify")
		}
	})

	t.Run("the try is counted before the compare", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		if _, err := checkReset(r.svc, "sess-d", code); err != nil {
			t.Fatal(err)
		}
		bump, mark := r.callIndex("BumpAuthCodeAttempt"), r.callIndex("MarkAuthCodeVerified")
		if bump < 0 || mark < 0 || bump > mark {
			t.Errorf("order of calls: bump at %d, mark at %d; the try must be counted first", bump, mark)
		}
	})

	t.Run("a code that is not six digits counts nothing and is refused", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		r.resetCodeFor(t, "sess-d")
		for _, typed := range []string{"", "12345", "1234567", "12a456", arabicIndicDigits} {
			if _, err := checkReset(r.svc, "sess-d", typed); !errors.Is(err, service.ErrWrongCode) {
				t.Errorf("%q: err = %v, want ErrWrongCode", typed, err)
			}
		}
	})

	t.Run("spaces in the typed code are dropped", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		if _, err := checkReset(r.svc, "sess-d", code[:3]+" "+code[3:]); err != nil {
			t.Fatalf("a pasted code with a space: %v", err)
		}
	})

	t.Run("a no account row answers like a wrong code and is never verified", func(t *testing.T) {
		r := newCredRig(t)
		if err := r.svc.RequestReset(ctx, "sess-d", "ghost@example.com"); err != nil {
			t.Fatal(err)
		}
		// Give the row a code we know, so the only thing refusing it is its delivery.
		now := r.clk.Now()
		row := r.newestCode(t)
		row.codeHash = auth.CodeHash(credCodeKey, "reset", "ghost@example.com", "123456")
		row.expires = now.Add(time.Minute)
		ghostID, ghost := checkReset(r.svc, "sess-d", "123456")

		known := newCredRig(t)
		known.addPasswordAccount(t, credEmail, credPassword)
		known.resetCodeFor(t, "sess-d")
		wrongID, wrong := checkReset(known.svc, "sess-d", "123456")
		if wrong == nil {
			wrongID, wrong = checkReset(known.svc, "sess-d", "123457")
		}
		// Review fix WR-02: neither answer rotates the session, so the cookie
		// behaviour of a no account address is that of a wrong code.
		if ghostID != "" || wrongID != "" || row.session != "sess-d" {
			t.Errorf("a refused code rotated a session: ghost %q wrong %q row on %q", ghostID, wrongID, row.session)
		}
		if !errors.Is(ghost, service.ErrWrongCode) || !errors.Is(wrong, service.ErrWrongCode) || ghost.Error() != wrong.Error() {
			t.Fatalf("no account row: %v; wrong code on a known address: %v; want the same ErrWrongCode", ghost, wrong)
		}
		if row.verified != nil || r.q.count("MarkAuthCodeVerified") != 0 {
			t.Error("a no_account row must never be verified")
		}
		if row.attempts != 1 {
			t.Errorf("no account row attempts = %d, want 1 (counted like any wrong code)", row.attempts)
		}
		if len(r.q.accounts) != 0 {
			t.Error("step one must not create an account")
		}
	})

	t.Run("a sign up row is never read as a reset", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		if _, err := checkReset(r.svc, "sess-a", code); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
		if r.q.count("BumpAuthCodeAttempt") != 0 || r.q.count("MarkAuthCodeVerified") != 0 {
			t.Error("a sign up row must not be counted or verified by step one")
		}
	})

	t.Run("an expired code is refused", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		r.clk.Advance(service.DefaultCredentialLimits.CodeTTL + time.Second)
		if _, err := checkReset(r.svc, "sess-d", code); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
	})

	t.Run("another browser holds no confirm", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		if _, err := checkReset(r.svc, "sess-other", code); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
	})
}

// TestConfirmResetNeedsAConfirmedCode is the point of the two steps: step two
// cannot be used to skip the code.
func TestConfirmResetNeedsAConfirmedCode(t *testing.T) {
	ctx := context.Background()
	const newPassword = "a brand new password 55"

	refused := func(t *testing.T, r *credRig, a *credAccount, session string) {
		t.Helper()
		before := *a.passwordHash
		epoch := a.epoch
		if err := r.svc.ConfirmReset(ctx, session, newPassword); !errors.Is(err, service.ErrWrongCode) {
			t.Fatalf("err = %v, want ErrWrongCode", err)
		}
		if *a.passwordHash != before || a.epoch != epoch || len(r.q.revokedAll) != 0 {
			t.Error("a refused step two must change nothing")
		}
		if len(r.hasher.hashed) != 0 {
			t.Errorf("a refused step two hashed %q; it must hash nothing", r.hasher.hashed)
		}
	}

	t.Run("no code asked at all", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		refused(t, r, a, "sess-d")
	})

	t.Run("a code asked for and not confirmed", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		r.resetCodeFor(t, "sess-d")
		refused(t, r, a, "sess-d")
	})

	t.Run("a confirm that ran out", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		fresh := r.resetVerified(t, "sess-d")
		r.clk.Advance(service.ResetConfirmWindow + time.Second)
		refused(t, r, a, fresh)
	})

	t.Run("a newer code replaces the confirm", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		fresh := r.resetVerified(t, "sess-d")
		r.clk.Advance(46 * time.Second)
		r.resetCodeFor(t, fresh)
		refused(t, r, a, fresh)
	})

	t.Run("Resend replaces the confirm", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		fresh := r.resetVerified(t, "sess-d")
		r.clk.Advance(46 * time.Second)
		if err := r.svc.ResendCode(ctx, fresh); err != nil {
			t.Fatalf("ResendCode: %v", err)
		}
		refused(t, r, a, fresh)
	})

	t.Run("a sign up row", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		r.signUpAndCode(t, "sess-a", "new@example.com", credPassword)
		r.hasher.hashed = nil // Create account hashed its own password
		refused(t, r, a, "sess-a")
	})

	t.Run("a row that was never verified, even with a right code typed elsewhere", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		r.resetVerified(t, "sess-d")
		refused(t, r, a, "sess-other")
	})

	t.Run("a no account row", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		if err := r.svc.RequestReset(ctx, "sess-g", "ghost@example.com"); err != nil {
			t.Fatal(err)
		}
		refused(t, r, a, "sess-g")
	})

	t.Run("the confirm window counts from the confirm, not from the send", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		ttl := service.DefaultCredentialLimits.CodeTTL
		code := r.resetCodeFor(t, "sess-d")
		r.clk.Advance(ttl - time.Minute)
		fresh, err := checkReset(r.svc, "sess-d", code)
		if err != nil {
			t.Fatalf("CheckResetCode: %v", err)
		}
		// The code itself is now past its life, but the confirm is two minutes old.
		r.clk.Advance(2 * time.Minute)
		if lc, ok, _ := r.svc.LiveCode(ctx, fresh); !ok || !lc.Verified {
			t.Errorf("LiveCode = %+v, %v; want a verified row", lc, ok)
		}
		if err := r.svc.ConfirmReset(ctx, fresh, newPassword); err != nil {
			t.Fatalf("ConfirmReset inside the confirm window: %v", err)
		}
	})

	// A confirm that ran out while the code is still live is renewed by typing the
	// code once more; without that, step two would stay refused until a Resend,
	// because the window (5 minutes) is shorter than the code's life (10).
	t.Run("a confirm that ran out while the code is live is renewed by typing the code again", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		fresh, err := checkReset(r.svc, "sess-d", code)
		if err != nil {
			t.Fatalf("CheckResetCode: %v", err)
		}
		r.clk.Advance(service.ResetConfirmWindow + time.Minute) // 6 minutes: the code has 4 left
		refused(t, r, a, fresh)
		if lc, ok, _ := r.svc.LiveCode(ctx, fresh); !ok || lc.Verified || lc.SecondsLeft <= 0 {
			t.Fatalf("LiveCode = %+v ok=%v; want a live code that is not confirmed", lc, ok)
		}
		again, err := checkReset(r.svc, fresh, code)
		if err != nil {
			t.Fatalf("the code typed again: %v", err)
		}
		if err := r.svc.ConfirmReset(ctx, again, newPassword); err != nil {
			t.Fatalf("ConfirmReset after the renewal: %v", err)
		}
	})

	// Review fix WR-01: Change email on step two turns the confirm off.
	t.Run("the cancel turns the confirm off and the code can be typed again", func(t *testing.T) {
		r := newCredRig(t)
		a := r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-d")
		fresh, err := checkReset(r.svc, "sess-d", code)
		if err != nil {
			t.Fatalf("CheckResetCode: %v", err)
		}
		if err := r.svc.CancelResetConfirm(ctx, fresh); err != nil {
			t.Fatalf("CancelResetConfirm: %v", err)
		}
		if lc, ok, _ := r.svc.LiveCode(ctx, fresh); !ok || lc.Verified {
			t.Errorf("after the cancel LiveCode = %+v ok=%v, want the code screen and not step two", lc, ok)
		}
		refused(t, r, a, fresh)
		// The row is still live: the same code confirms again, then step two works.
		again, err := checkReset(r.svc, fresh, code)
		if err != nil {
			t.Fatalf("the code typed again after the cancel: %v", err)
		}
		if err := r.svc.ConfirmReset(ctx, again, newPassword); err != nil {
			t.Fatalf("ConfirmReset after the code was typed again: %v", err)
		}
	})

	t.Run("the cancel with nothing to clear is a plain nil and touches no other browser", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		for _, session := range []string{"nobody", "sess-g"} {
			if err := r.svc.CancelResetConfirm(ctx, session); err != nil {
				t.Errorf("CancelResetConfirm(%q) = %v, want nil", session, err)
			}
		}
		other := r.resetVerified(t, "sess-o")
		if err := r.svc.CancelResetConfirm(ctx, "sess-d"); err != nil {
			t.Fatalf("CancelResetConfirm: %v", err)
		}
		if lc, ok, _ := r.svc.LiveCode(ctx, other); !ok || !lc.Verified {
			t.Errorf("another browser's confirm was turned off: %+v ok=%v", lc, ok)
		}
	})

	// Review fix WR-01: the confirm lasts 5 minutes, not a code lifetime.
	t.Run("the confirm window is five minutes from the confirm", func(t *testing.T) {
		if service.ResetConfirmWindow != 5*time.Minute {
			t.Fatalf("ResetConfirmWindow = %v, want 5m", service.ResetConfirmWindow)
		}
		for _, c := range []struct {
			name string
			wait time.Duration
			ok   bool
		}{
			{"4 minutes 59 seconds after the confirm", 5*time.Minute - time.Second, true},
			{"5 minutes after the confirm", 5 * time.Minute, false},
			{"6 minutes after the confirm", 6 * time.Minute, false},
		} {
			r := newCredRig(t)
			a := r.addPasswordAccount(t, credEmail, credPassword)
			fresh := r.resetVerified(t, "sess-d")
			r.clk.Advance(c.wait)
			lc, live, _ := r.svc.LiveCode(ctx, fresh)
			if live && lc.Verified != c.ok {
				t.Errorf("%s: LiveCode.Verified = %v, want %v", c.name, lc.Verified, c.ok)
			}
			if c.ok {
				if err := r.svc.ConfirmReset(ctx, fresh, newPassword); err != nil {
					t.Errorf("%s: ConfirmReset: %v", c.name, err)
				}
				continue
			}
			refused(t, r, a, fresh)
		}
	})
}

func TestConfirmResetPolicyBeforeAttempt(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	r.addPasswordAccount(t, credEmail, credPassword)
	fresh := r.resetVerified(t, "sess-d")
	tries := r.q.count("BumpAuthCodeAttempt")
	cases := []struct{ password, message string }{
		{"short1", service.PasswordTooShortMessage},
		{"password123", service.PasswordCommonMessage},
		{credEmail, service.PasswordIsEmailMessage},
	}
	for _, c := range cases {
		err := r.svc.ConfirmReset(ctx, fresh, c.password)
		var ve service.ValidationError
		if !errors.As(err, &ve) || ve.Field != "password" || ve.Message != c.message {
			t.Errorf("%q: err = %v, want password / %q", c.password, err, c.message)
		}
	}
	if r.q.count("BumpAuthCodeAttempt") != tries || r.q.codes[0].attempts != 1 {
		t.Error("a refused password must not spend a try")
	}
	if len(r.hasher.hashed) != 0 || r.q.codes[0].used != nil {
		t.Error("a refused password must hash nothing and spend nothing")
	}
	// The confirm is kept: the same browser still sets an acceptable password.
	if err := r.svc.ConfirmReset(ctx, fresh, "a brand new password 55"); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestLiveCode(t *testing.T) {
	ctx := context.Background()

	t.Run("none", func(t *testing.T) {
		r := newCredRig(t)
		if _, ok, err := r.svc.LiveCode(ctx, "sess-a"); err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("seconds left round up and never go negative", func(t *testing.T) {
		r := newCredRig(t)
		r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		lc, ok, err := r.svc.LiveCode(ctx, "sess-a")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if lc.Purpose != service.PurposeSignUp || lc.Email != credEmail || lc.SecondsLeft != 600 {
			t.Errorf("live code = %+v", lc)
		}
		r.clk.Advance(500*time.Second + 400*time.Millisecond)
		if lc, _, _ = r.svc.LiveCode(ctx, "sess-a"); lc.SecondsLeft != 100 {
			t.Errorf("seconds left = %d, want 100 (rounded up)", lc.SecondsLeft)
		}
		r.clk.Advance(5 * time.Minute)
		lc, ok, _ = r.svc.LiveCode(ctx, "sess-a")
		if !ok || lc.SecondsLeft != 0 {
			t.Errorf("expired row: ok=%v live=%+v, want shown with 0 seconds", ok, lc)
		}
	})

	// Review WR-B02 (server half): the resend wait a reload shows is what is
	// left of the cooldown since the stored send, rounded up, never negative.
	t.Run("resend seconds left follow the stored send", func(t *testing.T) {
		r := newCredRig(t)
		r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		cooldown := int(service.DefaultCredentialLimits.SendCooldown / time.Second)
		if lc, _, _ := r.svc.LiveCode(ctx, "sess-a"); lc.ResendSecondsLeft != cooldown {
			t.Errorf("right after the send: resend left %d, want %d", lc.ResendSecondsLeft, cooldown)
		}
		r.clk.Advance(20*time.Second + 300*time.Millisecond)
		if lc, _, _ := r.svc.LiveCode(ctx, "sess-a"); lc.ResendSecondsLeft != cooldown-20 {
			t.Errorf("20.3 s later: resend left %d, want %d (rounded up)", lc.ResendSecondsLeft, cooldown-20)
		}
		r.clk.Advance(2 * time.Minute)
		if lc, _, _ := r.svc.LiveCode(ctx, "sess-a"); lc.ResendSecondsLeft != 0 {
			t.Errorf("after the cooldown: resend left %d, want 0", lc.ResendSecondsLeft)
		}
	})

	t.Run("a consumed row is not live", func(t *testing.T) {
		r := newCredRig(t)
		code := r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		if _, err := r.svc.ConfirmSignUp(ctx, "sess-a", code, credSessionIDs("f")); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := r.svc.LiveCode(ctx, "sess-a"); ok {
			t.Error("the row was consumed and deleted")
		}
		used := r.clk.Now()
		r.q.codes = append(r.q.codes, credCodeRow{id: 99, session: "sess-z", purpose: "reset", delivery: "code",
			email: credEmail, codeHash: strings.Repeat("a", 64), created: used, expires: used.Add(time.Minute), used: &used})
		if _, ok, _ := r.svc.LiveCode(ctx, "sess-z"); ok {
			t.Error("a used row is not live")
		}
	})

	// Quick 261113-r5w: a reload on step two of Forgot password shows step two,
	// from Verified.
	t.Run("verified is true only for a confirmed reset code inside the confirm window", func(t *testing.T) {
		r := newCredRig(t)
		r.addPasswordAccount(t, credEmail, credPassword)
		code := r.resetCodeFor(t, "sess-a")
		if lc, ok, _ := r.svc.LiveCode(ctx, "sess-a"); !ok || lc.Verified {
			t.Errorf("a reset code not yet confirmed: %+v ok=%v, want live and not verified", lc, ok)
		}
		freshA, err := checkReset(r.svc, "sess-a", code)
		if err != nil {
			t.Fatal(err)
		}
		lc, ok, _ := r.svc.LiveCode(ctx, freshA)
		if !ok || !lc.Verified || lc.Purpose != service.PurposeReset {
			t.Errorf("a confirmed reset code: %+v ok=%v, want live and verified", lc, ok)
		}
		r.clk.Advance(service.ResetConfirmWindow + time.Second)
		if lc, _, _ := r.svc.LiveCode(ctx, freshA); lc.Verified {
			t.Error("the confirm ran out: Verified must be false")
		}
		// A sign up code is never verified.
		up := newCredRig(t)
		up.signUpAndCode(t, "sess-b", credEmail, credPassword)
		if lc, _, _ := up.svc.LiveCode(ctx, "sess-b"); lc.Verified {
			t.Error("a sign up code is never Verified")
		}
		// A newer code replaces the confirm.
		again := newCredRig(t)
		again.addPasswordAccount(t, credEmail, credPassword)
		freshC := again.resetVerified(t, "sess-c")
		again.clk.Advance(46 * time.Second)
		again.resetCodeFor(t, freshC)
		if lc, ok, _ := again.svc.LiveCode(ctx, freshC); !ok || lc.Verified {
			t.Errorf("after a newer code: %+v ok=%v, want live and not verified", lc, ok)
		}
		// A spent row is not live at all.
		done := newCredRig(t)
		done.addPasswordAccount(t, credEmail, credPassword)
		freshD := done.resetVerified(t, "sess-d")
		if err := done.svc.ConfirmReset(ctx, freshD, "a brand new password 55"); err != nil {
			t.Fatal(err)
		}
		if _, ok, _ := done.svc.LiveCode(ctx, freshD); ok {
			t.Error("a spent reset row is not live")
		}
	})

	t.Run("older than the resume window", func(t *testing.T) {
		r := newCredRig(t)
		r.signUpAndCode(t, "sess-a", credEmail, credPassword)
		r.clk.Advance(61 * time.Minute)
		if _, ok, _ := r.svc.LiveCode(ctx, "sess-a"); ok {
			t.Error("a row older than the resume window is not shown")
		}
	})
}

func TestSignedInAndEndedByPasswordSwitch(t *testing.T) {
	r := newCredRig(t)
	ctx := context.Background()
	withPw := r.addPasswordAccount(t, credEmail, credPassword)
	noPw := r.q.addAccount("old@example.com", nil)
	r.q.bindSession("good", withPw.id)
	r.q.bindSession("old", noPw.id)
	r.q.bindSession("gone", withPw.id)
	r.q.sessions["gone"].revoked = true

	for id, want := range map[string]bool{"good": true, "old": false, "gone": false, "never": false} {
		got, err := r.svc.SignedIn(ctx, id)
		if err != nil || got != want {
			t.Errorf("SignedIn(%s) = %v, %v; want %v", id, got, err, want)
		}
	}
	for id, want := range map[string]bool{"good": false, "old": true, "never": false} {
		got, err := r.svc.EndedByPasswordSwitch(ctx, id)
		if err != nil || got != want {
			t.Errorf("EndedByPasswordSwitch(%s) = %v, %v; want %v", id, got, err, want)
		}
	}
}

func TestSignInLockedMessage(t *testing.T) {
	cases := []struct {
		wait time.Duration
		want string
	}{
		{4*time.Minute + 10*time.Second, "Too many tries. Wait 5 minutes, or reset your password."},
		{time.Minute, "Too many tries. Wait 1 minute, or reset your password."},
		{30 * time.Second, "Too many tries. Wait 1 minute, or reset your password."},
		{61 * time.Second, "Too many tries. Wait 2 minutes, or reset your password."},
		{60 * time.Minute, "Too many tries. Wait 60 minutes, or reset your password."},
	}
	for _, c := range cases {
		if got := service.SignInLockedMessage(c.wait); got != c.want {
			t.Errorf("SignInLockedMessage(%v) = %q, want %q", c.wait, got, c.want)
		}
	}
	if (service.SignInLockedError{Wait: 5 * time.Minute}).Error() != service.SignInLockedMessage(5*time.Minute) {
		t.Error("the error text must be the message")
	}
}
