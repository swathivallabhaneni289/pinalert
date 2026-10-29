// credentials_security_e2e_test.go is the security suite of the code and
// password sign in (Phase 7.1, plan 09, IDENT-06). It drives the six public POST
// routes over the real router and a real Postgres and pins: uniform answers
// (enumeration, the header set included), the lock schedule, the per device
// buckets (CGNAT), the code budget, login CSRF, session fixation, the reset and
// the rule that no address ever appears in a URL.
//
// Every client is a fresh cookie jar, so it is its own device. The clients of
// the comparison tests turn response compression off, so Content-Length is a
// real header on both sides of a compare and a leak by length would show.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	// secWrongPassword passes the password rule and is never an account's password.
	secWrongPassword = "not the right one 99"
	// secNewPassword is the password a reset sets.
	secNewPassword = "silver harbor evening 81"

	secWrongCredentialsSentence = "Email or password is wrong."
	secWrongCodeSentence        = "That code is wrong or has expired."
	secHourSentence             = "Too many codes for this email. Try again in an hour."
	secLockFiveMinutes          = "Too many tries. Wait 5 minutes, or reset your password."
	secMinuteSentence           = "Too many requests. Try again in a minute."

	secReportsPath = "/api/reports?lat=12.9716&lon=77.5946&radius_km=5"
)

// newPlainJarClient is a fresh browser with response compression off.
func newPlainJarClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	tr := &http.Transport{DisableCompression: true}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Jar: jar, Transport: tr}
}

// newLoginClient is a fresh plain browser that has visited GET /login, so it
// already holds its session cookie the way a browser on the sign in page does.
func newLoginClient(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	c := newPlainJarClient(t)
	getLoginPage(t, c, srv)
	return c
}

// authAnswer is everything of a response the enumeration compare looks at.
type authAnswer struct {
	status        int
	body          []byte
	setCookie     bool
	retryAfter    string
	contentLength string
	contentType   string
	cacheControl  string
}

func doAuth(t *testing.T, client *http.Client, srv *httptest.Server, path string, body any) authAnswer {
	t.Helper()
	status, raw, resp := postAuthJSON(t, client, srv, path, body)
	a := authAnswer{
		status:        status,
		body:          raw,
		setCookie:     len(resp.Header.Values("Set-Cookie")) > 0,
		retryAfter:    resp.Header.Get("Retry-After"),
		contentLength: resp.Header.Get("Content-Length"),
		contentType:   resp.Header.Get("Content-Type"),
		cacheControl:  resp.Header.Get("Cache-Control"),
	}
	if a.contentLength == "" {
		t.Fatalf("POST %s answered with no Content-Length header: the header compare would check nothing", path)
	}
	return a
}

// compareAuthAnswers fails when two answers differ in status, body bytes,
// Set-Cookie present or absent (the cookie value is ignored), Retry-After,
// Content-Length, Content-Type or Cache-Control (W10).
func compareAuthAnswers(t *testing.T, label string, a, b authAnswer) {
	t.Helper()
	if a.status != b.status {
		t.Errorf("%s: status %d vs %d", label, a.status, b.status)
	}
	if !bytes.Equal(a.body, b.body) {
		t.Errorf("%s: bodies differ: %q vs %q", label, a.body, b.body)
	}
	if a.setCookie != b.setCookie {
		t.Errorf("%s: Set-Cookie present %v vs %v", label, a.setCookie, b.setCookie)
	}
	if a.retryAfter != b.retryAfter {
		t.Errorf("%s: Retry-After %q vs %q", label, a.retryAfter, b.retryAfter)
	}
	if a.contentLength != b.contentLength {
		t.Errorf("%s: Content-Length %q vs %q", label, a.contentLength, b.contentLength)
	}
	if a.contentType != b.contentType {
		t.Errorf("%s: Content-Type %q vs %q", label, a.contentType, b.contentType)
	}
	if a.cacheControl != b.cacheControl {
		t.Errorf("%s: Cache-Control %q vs %q", label, a.cacheControl, b.cacheControl)
	}
}

// ageClaim moves the per address send claim back by a minute and keeps the
// hourly window, so the next send is not refused by the cooldown while the
// hourly count still stands. The harness cooldown is 1 ms, so this is a no-op
// in effect; it keeps the tests true to the 45 second production rule.
func ageClaim(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	testutil.MustExec(t, pool,
		"UPDATE email_cooldowns SET last_requested_at = last_requested_at - interval '1 minute' WHERE email = $1", email)
}

// seedLegacyAccount makes an account of the old link flow: a row with no password.
func seedLegacyAccount(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	testutil.MustExec(t, pool, "INSERT INTO accounts (email) VALUES ($1)", email)
}

// createAccountViaCode makes an account through the code routes only (so no
// sign in bucket is touched) and returns the browser that is now signed in.
func createAccountViaCode(t *testing.T, srv *httptest.Server, mail *recordingMailer, email string) *http.Client {
	t.Helper()
	c := newLoginClient(t, srv)
	if status := signUpRequest(t, c, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	if status := confirmSignUpCode(t, c, srv, mail.codeFor(t, email)); status != http.StatusOK {
		t.Fatalf("confirm sign up code status = %d, want 200", status)
	}
	return c
}

// wrongCodeFor is a 6 digit code that differs from code in its last digit.
func wrongCodeFor(code string) string {
	repl := "0"
	if code[len(code)-1] == '0' {
		repl = "1"
	}
	return code[:len(code)-1] + repl
}

func reportsStatus(t *testing.T, client *http.Client, srv *httptest.Server) int {
	t.Helper()
	resp, err := client.Get(srv.URL + secReportsPath)
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// expectError asserts a 400 or 429 envelope with the given field and sentence.
func expectError(t *testing.T, label string, a authAnswer, status int, field, message string) {
	t.Helper()
	if a.status != status {
		t.Fatalf("%s: status = %d, want %d: %s", label, a.status, status, a.body)
	}
	gotField, gotMessage := errorField(t, a.body)
	if gotField != field || gotMessage != message {
		t.Fatalf("%s: error = field %q message %q, want field %q message %q", label, gotField, gotMessage, field, message)
	}
}

// throttleFailures reads the failures column of the address's sign in throttle
// row, and whether a row exists.
func throttleFailures(t *testing.T, pool *pgxpool.Pool, creds *service.CredentialService, email string) (int, bool) {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		"SELECT failures FROM auth_throttles WHERE scope = 'sign_in' AND key_hash = $1", creds.ThrottleKey(email)).Scan(&n)
	if err != nil {
		return 0, false
	}
	return n, true
}

func retryAfterOf(t *testing.T, a authAnswer) int {
	t.Helper()
	n, err := strconv.Atoi(a.retryAfter)
	if err != nil {
		t.Fatalf("Retry-After = %q, want an integer", a.retryAfter)
	}
	return n
}

func mailKind(t *testing.T, mail *recordingMailer, email string) mailer.AuthEmailKind {
	t.Helper()
	msg, ok := mail.lastAuthFor(email)
	if !ok {
		t.Fatalf("no auth email was sent to %q", email)
	}
	return msg.Kind
}

// ---- Enumeration --------------------------------------------------------

func TestEnumerationSignUpIdentical(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		newAddr    = "enum-su-new@example.com"
		legacyAddr = "enum-su-legacy@example.com"
		knownAddr  = "enum-su-known@example.com"
	)
	seedLegacyAccount(t, pool, legacyAddr)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)
	ageClaim(t, pool, knownAddr)

	answers := map[string]authAnswer{}
	for _, addr := range []string{newAddr, legacyAddr, knownAddr} {
		c := newLoginClient(t, srv)
		answers[addr] = doAuth(t, c, srv, "/api/auth/sign-up", map[string]string{"email": addr, "password": e2eTestPassword})
		if answers[addr].status != http.StatusOK {
			t.Fatalf("sign up %s status = %d, want 200: %s", addr, answers[addr].status, answers[addr].body)
		}
	}
	for _, other := range []string{legacyAddr, knownAddr} {
		compareAuthAnswers(t, "sign up "+other+" vs new", answers[newAddr], answers[other])
		if !bytes.Equal(answers[newAddr].body, answers[other].body) {
			t.Errorf("sign up body for %s differs from the new address body", other)
		}
	}

	if got := mailKind(t, mail, newAddr); got != mailer.KindCodeSignup {
		t.Errorf("new address mail = %q, want %q", got, mailer.KindCodeSignup)
	}
	if got := mailKind(t, mail, legacyAddr); got != mailer.KindCodeSignup {
		t.Errorf("link account mail = %q, want %q", got, mailer.KindCodeSignup)
	}
	if got := mailKind(t, mail, knownAddr); got != mailer.KindAccountExists {
		t.Errorf("account with a password mail = %q, want %q", got, mailer.KindAccountExists)
	}
}

func TestEnumerationResetIdentical(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		knownAddr   = "enum-re-known@example.com"
		legacyAddr  = "enum-re-legacy@example.com"
		unknownAddr = "enum-re-unknown@example.com"
	)
	seedLegacyAccount(t, pool, legacyAddr)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)
	ageClaim(t, pool, knownAddr)

	answers := map[string]authAnswer{}
	for _, addr := range []string{unknownAddr, knownAddr, legacyAddr} {
		c := newLoginClient(t, srv)
		answers[addr] = doAuth(t, c, srv, "/api/auth/reset", map[string]string{"email": addr})
		if answers[addr].status != http.StatusOK {
			t.Fatalf("reset %s status = %d, want 200: %s", addr, answers[addr].status, answers[addr].body)
		}
	}
	for _, other := range []string{knownAddr, legacyAddr} {
		compareAuthAnswers(t, "reset "+other+" vs unknown", answers[unknownAddr], answers[other])
		if !bytes.Equal(answers[unknownAddr].body, answers[other].body) {
			t.Errorf("reset body for %s differs from the unknown address body", other)
		}
	}
	if got := mailKind(t, mail, knownAddr); got != mailer.KindCodeReset {
		t.Errorf("known address mail = %q, want %q", got, mailer.KindCodeReset)
	}
	if got := mailKind(t, mail, legacyAddr); got != mailer.KindCodeReset {
		t.Errorf("link account mail = %q, want %q", got, mailer.KindCodeReset)
	}
	if got := mailKind(t, mail, unknownAddr); got != mailer.KindNoAccount {
		t.Errorf("unknown address mail = %q, want %q", got, mailer.KindNoAccount)
	}
}

func TestEnumerationSignInIdentical(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		knownAddr   = "enum-si-known@example.com"
		legacyAddr  = "enum-si-legacy@example.com"
		unknownAddr = "enum-si-unknown@example.com"
	)
	seedLegacyAccount(t, pool, legacyAddr)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)

	unknown := doAuth(t, newLoginClient(t, srv), srv, "/api/auth/sign-in", map[string]string{"email": unknownAddr, "password": secWrongPassword})
	wrong := doAuth(t, newLoginClient(t, srv), srv, "/api/auth/sign-in", map[string]string{"email": knownAddr, "password": secWrongPassword})
	noPassword := doAuth(t, newLoginClient(t, srv), srv, "/api/auth/sign-in", map[string]string{"email": legacyAddr, "password": secWrongPassword})

	expectError(t, "unknown address", unknown, http.StatusBadRequest, "credentials", secWrongCredentialsSentence)
	compareAuthAnswers(t, "wrong password vs unknown address", unknown, wrong)
	compareAuthAnswers(t, "no password vs unknown address", unknown, noPassword)
	if !bytes.Equal(unknown.body, wrong.body) || !bytes.Equal(unknown.body, noPassword.body) {
		t.Errorf("the three sign in refusals are not byte identical: %q %q %q", unknown.body, wrong.body, noPassword.body)
	}
}

func TestEnumerationLockIdentical(t *testing.T) {
	srv, mail, _ := newE2EServer(t)
	const (
		knownAddr   = "enum-lock-known@example.com"
		unknownAddr = "enum-lock-unknown@example.com"
	)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)

	lock := func(addr string) authAnswer {
		c := newLoginClient(t, srv)
		for i := 1; i <= 6; i++ {
			a := doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": addr, "password": secWrongPassword})
			expectError(t, "wrong attempt "+strconv.Itoa(i), a, http.StatusBadRequest, "credentials", secWrongCredentialsSentence)
		}
		return doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": addr, "password": secWrongPassword})
	}
	known := lock(knownAddr)
	unknown := lock(unknownAddr)

	expectError(t, "known address seventh attempt", known, http.StatusTooManyRequests, "email", secLockFiveMinutes)
	compareAuthAnswers(t, "lock on a known vs an unknown address", known, unknown)
	if !bytes.Equal(known.body, unknown.body) {
		t.Errorf("lock bodies differ: %q vs %q", known.body, unknown.body)
	}
}

func TestEnumerationResendIdentical(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		newAddr     = "enum-rs-new@example.com"
		legacyAddr  = "enum-rs-legacy@example.com"
		knownAddr   = "enum-rs-known@example.com"
		resetKnown  = "enum-rs-reset-known@example.com"
		resetUnknwn = "enum-rs-reset-unknown@example.com"
	)
	seedLegacyAccount(t, pool, legacyAddr)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)
	verifySession(t, newPlainJarClient(t), srv, mail, resetKnown)
	ageClaim(t, pool, knownAddr)
	ageClaim(t, pool, resetKnown)

	type branch struct {
		addr, path string
		body       map[string]string
		kind       mailer.AuthEmailKind
	}
	signUp := map[string]string{"password": e2eTestPassword}
	branches := []branch{
		{newAddr, "/api/auth/sign-up", map[string]string{"email": newAddr, "password": signUp["password"]}, mailer.KindCodeSignup},
		{legacyAddr, "/api/auth/sign-up", map[string]string{"email": legacyAddr, "password": signUp["password"]}, mailer.KindCodeSignup},
		{knownAddr, "/api/auth/sign-up", map[string]string{"email": knownAddr, "password": signUp["password"]}, mailer.KindAccountExists},
		{resetKnown, "/api/auth/reset", map[string]string{"email": resetKnown}, mailer.KindCodeReset},
		{resetUnknwn, "/api/auth/reset", map[string]string{"email": resetUnknwn}, mailer.KindNoAccount},
	}
	var first authAnswer
	for i, b := range branches {
		c := newLoginClient(t, srv)
		if got := doAuth(t, c, srv, b.path, b.body); got.status != http.StatusOK {
			t.Fatalf("%s request status = %d: %s", b.addr, got.status, got.body)
		}
		ageClaim(t, pool, b.addr)
		before := mail.authSendCount()
		a := doAuth(t, c, srv, "/api/auth/code/resend", map[string]string{})
		if a.status != http.StatusOK {
			t.Fatalf("resend for %s status = %d, want 200: %s", b.addr, a.status, a.body)
		}
		if after := mail.authSendCount(); after != before+1 {
			t.Errorf("resend for %s sent %d emails, want 1", b.addr, after-before)
		}
		if got := mailKind(t, mail, b.addr); got != b.kind {
			t.Errorf("resend mail for %s = %q, want %q", b.addr, got, b.kind)
		}
		if i == 0 {
			first = a
			continue
		}
		compareAuthAnswers(t, "resend "+b.addr+" vs new", first, a)
		if !bytes.Equal(first.body, a.body) {
			t.Errorf("resend body for %s differs from the new address body", b.addr)
		}
	}
}

// ---- Codes ----------------------------------------------------------------

func TestLimitsCodeTries(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "limits-tries@example.com"
	c := newLoginClient(t, srv)
	if status := signUpRequest(t, c, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)
	for i := 1; i <= 5; i++ {
		a := doAuth(t, c, srv, "/api/auth/sign-up/code", map[string]string{"code": wrongCodeFor(code)})
		expectError(t, "wrong code "+strconv.Itoa(i), a, http.StatusBadRequest, "code", secWrongCodeSentence)
	}
	// The sixth try fails even with the right code: the budget is spent.
	a := doAuth(t, c, srv, "/api/auth/sign-up/code", map[string]string{"code": code})
	expectError(t, "right code after five tries", a, http.StatusBadRequest, "code", secWrongCodeSentence)
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM accounts WHERE email = $1", email).Scan(&n); err != nil || n != 0 {
		t.Fatalf("accounts for %s = %d (err %v), want 0: a spent code must not create the account", email, n, err)
	}
}

func TestCodeBoundToSession(t *testing.T) {
	srv, mail, _ := newE2EServer(t)
	const email = "code-bound@example.com"
	a := newLoginClient(t, srv)
	b := newLoginClient(t, srv)
	if status := signUpRequest(t, a, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)

	got := doAuth(t, b, srv, "/api/auth/sign-up/code", map[string]string{"code": code})
	expectError(t, "A's code typed in browser B", got, http.StatusBadRequest, "code", secWrongCodeSentence)

	// A's own code still works: B's try cost A nothing.
	if status := confirmSignUpCode(t, a, srv, code); status != http.StatusOK {
		t.Fatalf("A's code after B's try: status = %d, want 200", status)
	}
}

func TestResendReplacesCode(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "resend-replaces@example.com"
	c := newLoginClient(t, srv)
	if status := signUpRequest(t, c, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	first := mail.codeFor(t, email)
	second := first
	// Two draws can in theory match (one in a million); draw again then.
	for tries := 0; second == first && tries < 3; tries++ {
		ageClaim(t, pool, email)
		a := doAuth(t, c, srv, "/api/auth/code/resend", map[string]string{})
		if a.status != http.StatusOK {
			t.Fatalf("resend status = %d, want 200: %s", a.status, a.body)
		}
		second = mail.codeFor(t, email)
	}
	if second == first {
		t.Fatalf("three resends produced the same code again")
	}
	old := doAuth(t, c, srv, "/api/auth/sign-up/code", map[string]string{"code": first})
	expectError(t, "the code before the resend", old, http.StatusBadRequest, "code", secWrongCodeSentence)
	if status := confirmSignUpCode(t, c, srv, second); status != http.StatusOK {
		t.Fatalf("the resent code: status = %d, want 200", status)
	}
}

func TestLimitsHourlyCodeCap(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "limits-hourly@example.com"
	c := newLoginClient(t, srv)
	if status := signUpRequest(t, c, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("send 1 status = %d, want 200", status)
	}
	for i := 2; i <= 5; i++ {
		ageClaim(t, pool, email)
		a := doAuth(t, c, srv, "/api/auth/code/resend", map[string]string{})
		if a.status != http.StatusOK {
			t.Fatalf("send %d status = %d, want 200: %s", i, a.status, a.body)
		}
	}
	before := mail.authSendCount()
	ageClaim(t, pool, email)
	a := doAuth(t, c, srv, "/api/auth/code/resend", map[string]string{})
	expectError(t, "sixth send", a, http.StatusTooManyRequests, "email", secHourSentence)
	if got := retryAfterOf(t, a); got != 3600 {
		t.Errorf("Retry-After = %d, want 3600", got)
	}
	if after := mail.authSendCount(); after != before {
		t.Errorf("the refused sixth send still mailed %d emails", after-before)
	}
}

// TestLimitsHourlyCapSharedAndIdentical (W10): Create account and Forgot
// password draw on one count of 5 an hour, and the cap answers a known and an
// unknown address with the same bytes and headers.
func TestLimitsHourlyCapSharedAndIdentical(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		knownAddr   = "hourly-shared-known@example.com"
		unknownAddr = "hourly-shared-unknown@example.com"
	)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)
	// Test side setup: verifySession's own sign up spent one of the five sends
	// of the known address. Start both addresses from no claim row.
	testutil.MustExec(t, pool, "DELETE FROM email_cooldowns WHERE email = $1", knownAddr)

	signUp := func(addr string) map[string]string {
		return map[string]string{"email": addr, "password": e2eTestPassword}
	}
	reset := func(addr string) map[string]string { return map[string]string{"email": addr} }
	steps := []struct {
		name, path string
		body       func(string) map[string]string
		ok         bool
	}{
		{"create account 1", "/api/auth/sign-up", signUp, true},
		{"forgot password 1", "/api/auth/reset", reset, true},
		{"create account 2", "/api/auth/sign-up", signUp, true},
		{"forgot password 2", "/api/auth/reset", reset, true},
		{"create account 3", "/api/auth/sign-up", signUp, true},
		{"sixth send, create account", "/api/auth/sign-up", signUp, false},
		{"seventh send, forgot password", "/api/auth/reset", reset, false},
	}
	known := newLoginClient(t, srv)
	unknown := newLoginClient(t, srv)
	for _, s := range steps {
		ageClaim(t, pool, knownAddr)
		ageClaim(t, pool, unknownAddr)
		k := doAuth(t, known, srv, s.path, s.body(knownAddr))
		u := doAuth(t, unknown, srv, s.path, s.body(unknownAddr))
		compareAuthAnswers(t, s.name, k, u)
		if !bytes.Equal(k.body, u.body) {
			t.Errorf("%s: bodies differ between the known and the unknown address", s.name)
		}
		if s.ok && k.status != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200: %s", s.name, k.status, k.body)
		}
		if !s.ok {
			expectError(t, s.name, k, http.StatusTooManyRequests, "email", secHourSentence)
			if got := retryAfterOf(t, k); got != 3600 {
				t.Errorf("%s: Retry-After = %d, want 3600", s.name, got)
			}
		}
	}
}

// TestResetCodeBoundToSession (W10): a reset code works only in the browser
// that asked for it.
func TestResetCodeBoundToSession(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-bound@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)

	a := newLoginClient(t, srv)
	b := newLoginClient(t, srv)
	if status := requestResetRequest(t, a, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)

	got := doAuth(t, b, srv, "/api/auth/reset/code", map[string]string{"code": code, "password": secNewPassword})
	expectError(t, "A's reset code typed in browser B", got, http.StatusBadRequest, "code", secWrongCodeSentence)

	if status := confirmResetRequest(t, a, srv, code, secNewPassword); status != http.StatusOK {
		t.Fatalf("A's reset code after B's try: status = %d, want 200", status)
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, secNewPassword); status != http.StatusOK {
		t.Fatalf("sign in with the new password: status = %d, want 200", status)
	}
}

// TestSignUpCodeRefusedOnReset (W10): a Create account code cannot set a
// password through the reset route.
func TestSignUpCodeRefusedOnReset(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "signup-code-on-reset@example.com"
	seedLegacyAccount(t, pool, email)
	c := newLoginClient(t, srv)
	if status := signUpRequest(t, c, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)

	a := doAuth(t, c, srv, "/api/auth/reset/code", map[string]string{"code": code, "password": secNewPassword})
	expectError(t, "sign up code on the reset route", a, http.StatusBadRequest, "code", secWrongCodeSentence)

	var hasPassword bool
	if err := pool.QueryRow(context.Background(),
		"SELECT password_hash IS NOT NULL FROM accounts WHERE email = $1", email).Scan(&hasPassword); err != nil {
		t.Fatalf("reading the account: %v", err)
	}
	if hasPassword {
		t.Fatalf("the reset route set a password from a sign up code")
	}
	// The refusal did not burn the code: it still confirms the sign up.
	if status := confirmSignUpCode(t, c, srv, code); status != http.StatusOK {
		t.Fatalf("the sign up code after the refusal: status = %d, want 200", status)
	}
}

// ---- Throttle and buckets ----------------------------------------------------

func TestLimitsAccountThrottle(t *testing.T) {
	srv, mail, _ := newE2EServer(t)
	const email = "limits-throttle@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)

	c := newLoginClient(t, srv)
	for i := 1; i <= 6; i++ {
		a := doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": secWrongPassword})
		expectError(t, "wrong attempt "+strconv.Itoa(i), a, http.StatusBadRequest, "credentials", secWrongCredentialsSentence)
	}
	// Locked: even the right password is refused, without being checked.
	a := doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": e2eTestPassword})
	expectError(t, "right password while locked", a, http.StatusTooManyRequests, "email", secLockFiveMinutes)
	if got := retryAfterOf(t, a); got < 290 || got > 300 {
		t.Errorf("Retry-After = %d, want 290 to 300", got)
	}
	// Another browser is locked out of the same address too: the lock is per address.
	other := doAuth(t, newLoginClient(t, srv), srv, "/api/auth/sign-in", map[string]string{"email": email, "password": e2eTestPassword})
	expectError(t, "right password from another browser", other, http.StatusTooManyRequests, "email", secLockFiveMinutes)
}

func TestLimitsResetClearsThrottle(t *testing.T) {
	var creds *service.CredentialService
	srv, mail, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) { creds = d.Credentials })
	const email = "limits-reset-clears@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)

	c := newLoginClient(t, srv)
	for i := 1; i <= 6; i++ {
		doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": secWrongPassword})
	}
	locked := doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": e2eTestPassword})
	expectError(t, "locked", locked, http.StatusTooManyRequests, "email", secLockFiveMinutes)
	if _, ok := throttleFailures(t, pool, creds, email); !ok {
		t.Fatalf("no throttle row after six failures")
	}

	ageClaim(t, pool, email)
	r := newLoginClient(t, srv)
	if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	if status := confirmResetRequest(t, r, srv, mail.codeFor(t, email), secNewPassword); status != http.StatusOK {
		t.Fatalf("reset confirm status = %d, want 200", status)
	}
	if _, ok := throttleFailures(t, pool, creds, email); ok {
		t.Errorf("the reset left the throttle row in place")
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, secNewPassword); status != http.StatusOK {
		t.Fatalf("sign in with the new password right after the reset: status = %d, want 200", status)
	}
}

// TestLimitsLockScheduleReachesCap (B1, R2-W2): the lock grows 5, 10, 20, 40 and
// then stays at 60 minutes, even when each wrong attempt is made the moment the
// previous lock ends, so the last failure is older than the forget window while
// the lock end is not.
func TestLimitsLockScheduleReachesCap(t *testing.T) {
	var creds *service.CredentialService
	srv, mail, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) { creds = d.Credentials })
	const email = "limits-schedule@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	key := creds.ThrottleKey(email)

	c := newLoginClient(t, srv)
	wrong := func() authAnswer {
		return doAuth(t, c, srv, "/api/auth/sign-in", map[string]string{"email": email, "password": secWrongPassword})
	}
	wantFailures := func(want int) {
		t.Helper()
		got, ok := throttleFailures(t, pool, creds, email)
		if !ok || got != want {
			t.Fatalf("failures = %d (row %v), want %d", got, ok, want)
		}
	}
	wantLock := func(label string, want int) {
		t.Helper()
		a := wrong()
		if a.status != http.StatusTooManyRequests {
			t.Fatalf("%s: probe status = %d, want 429: %s", label, a.status, a.body)
		}
		got := retryAfterOf(t, a)
		if got > want || got < want-15 {
			t.Fatalf("%s: lock Retry-After = %d, want about %d", label, got, want)
		}
	}

	for i := 1; i <= 6; i++ {
		if a := wrong(); a.status != http.StatusBadRequest {
			t.Fatalf("failure %d status = %d, want 400", i, a.status)
		}
	}
	wantFailures(6)
	wantLock("after failure 6", 300)

	schedule := []struct {
		failure     int
		previousMin int
		wantLockSec int
	}{
		{7, 5, 600},
		{8, 10, 1200},
		{9, 20, 2400},
		{10, 40, 3600},
		{11, 60, 3600},
	}
	for _, s := range schedule {
		// As if the whole previous lock had passed in real time.
		testutil.MustExec(t, pool,
			"UPDATE auth_throttles SET locked_until = now() - interval '1 second', last_failure_at = last_failure_at - make_interval(mins => $2) WHERE scope = 'sign_in' AND key_hash = $1",
			key, s.previousMin)
		if a := wrong(); a.status != http.StatusBadRequest {
			t.Fatalf("failure %d status = %d, want 400: %s", s.failure, a.status, a.body)
		}
		wantFailures(s.failure)
		wantLock("after failure "+strconv.Itoa(s.failure), s.wantLockSec)
	}
}

func TestLimitsDeviceBucketCGNAT(t *testing.T) {
	srv, mail, _ := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		limits := e2eGenerousAuthLimits
		limits.SignInDevice = api.BucketLimit{Every: time.Hour, Burst: 2}
		d.AuthLimits = limits
	})
	const (
		emailX = "cgnat-x@example.com"
		emailY = "cgnat-y@example.com"
	)
	// Both accounts are made through the code routes, which spend no sign in token.
	createAccountViaCode(t, srv, mail, emailX)
	createAccountViaCode(t, srv, mail, emailY)

	a := newLoginClient(t, srv)
	for i := 1; i <= 2; i++ {
		got := doAuth(t, a, srv, "/api/auth/sign-in", map[string]string{"email": emailX, "password": secWrongPassword})
		expectError(t, "A wrong attempt "+strconv.Itoa(i), got, http.StatusBadRequest, "credentials", secWrongCredentialsSentence)
	}
	third := doAuth(t, a, srv, "/api/auth/sign-in", map[string]string{"email": emailX, "password": secWrongPassword})
	expectError(t, "A third attempt", third, http.StatusTooManyRequests, "email", secMinuteSentence)

	// Browser B shares 127.0.0.1 with A and has no CLIENT_IP_HEADER to tell them
	// apart by network: its own bucket is untouched.
	b := newLoginClient(t, srv)
	got := doAuth(t, b, srv, "/api/auth/sign-in", map[string]string{"email": emailY, "password": e2eTestPassword})
	if got.status != http.StatusOK {
		t.Fatalf("B's sign in after A's lockout: status = %d, want 200: %s", got.status, got.body)
	}
}

// ---- Login CSRF -----------------------------------------------------------

func rawAuthPost(t *testing.T, srv *httptest.Server, path, contentType string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestCSRFCrossSiteRefused(t *testing.T) {
	srv, _, _ := newE2EServer(t)
	for _, path := range authRoutePaths {
		if got := rawAuthPost(t, srv, path, "application/json", map[string]string{"Sec-Fetch-Site": "cross-site"}); got != http.StatusForbidden {
			t.Errorf("POST %s with Sec-Fetch-Site cross-site = %d, want 403", path, got)
		}
		if got := rawAuthPost(t, srv, path, "application/json", map[string]string{"Origin": "https://evil.example"}); got != http.StatusForbidden {
			t.Errorf("POST %s with a foreign Origin = %d, want 403", path, got)
		}
		// The Go client sends neither header, like a script or a curl: it passes
		// the cross origin check and reaches the handler.
		if got := rawAuthPost(t, srv, path, "application/json", nil); got == http.StatusForbidden || got == http.StatusUnsupportedMediaType {
			t.Errorf("POST %s without browser headers = %d, want it to reach the handler", path, got)
		}
	}
}

func TestCSRFTextPlainRefused(t *testing.T) {
	srv, _, _ := newE2EServer(t)
	for _, path := range authRoutePaths {
		for _, contentType := range []string{"text/plain", "text/plain;charset=UTF-8", "application/x-www-form-urlencoded"} {
			if got := rawAuthPost(t, srv, path, contentType, nil); got != http.StatusUnsupportedMediaType {
				t.Errorf("POST %s as %s = %d, want 415", path, contentType, got)
			}
		}
		if got := rawAuthPost(t, srv, path, "application/json", nil); got == http.StatusForbidden || got == http.StatusUnsupportedMediaType {
			t.Errorf("POST %s as application/json = %d, want it to reach the handler", path, got)
		}
	}
}

// ---- Fixation -----------------------------------------------------------------

func sessionAccountID(t *testing.T, pool *pgxpool.Pool, sessionID string) *int64 {
	t.Helper()
	var id *int64
	if err := pool.QueryRow(context.Background(), "SELECT account_id FROM sessions WHERE session_id = $1", sessionID).Scan(&id); err != nil {
		t.Fatalf("reading the session row %q: %v", sessionID, err)
	}
	return id
}

func TestFixationFreshSessionID(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "fixation-fresh@example.com"

	// Through the sign up code.
	first := newLoginClient(t, srv)
	pre := sessionIDFromJar(t, first, srv.URL)
	if status := signUpRequest(t, first, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	if status := confirmSignUpCode(t, first, srv, mail.codeFor(t, email)); status != http.StatusOK {
		t.Fatalf("confirm code status = %d, want 200", status)
	}
	post := sessionIDFromJar(t, first, srv.URL)
	if post == pre {
		t.Fatalf("the session id did not change at the sign up code: still %q", post)
	}
	if id := sessionAccountID(t, pool, pre); id != nil {
		t.Errorf("the pre sign in session %q is bound to account %d, want unbound", pre, *id)
	}
	if id := sessionAccountID(t, pool, post); id == nil {
		t.Errorf("the post sign in session %q is not bound to the account", post)
	}

	// Through the password.
	second := newLoginClient(t, srv)
	pre2 := sessionIDFromJar(t, second, srv.URL)
	if status := signInRequest(t, second, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign in status = %d, want 200", status)
	}
	post2 := sessionIDFromJar(t, second, srv.URL)
	if post2 == pre2 {
		t.Fatalf("the session id did not change at sign in: still %q", post2)
	}
	if id := sessionAccountID(t, pool, pre2); id != nil {
		t.Errorf("the pre sign in session %q is bound to account %d, want unbound", pre2, *id)
	}
}

func TestFixationPlantedCookieStaysUnbound(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "fixation-planted@example.com"
	createAccountViaCode(t, srv, mail, email)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	// The attacker holds a pre sign in session and plants its cookie in the
	// victim's browser.
	attacker := newLoginClient(t, srv)
	planted := sessionIDFromJar(t, attacker, srv.URL)
	victim := newPlainJarClient(t)
	victim.Jar.SetCookies(u, attacker.Jar.Cookies(u))
	if got := sessionIDFromJar(t, victim, srv.URL); got != planted {
		t.Fatalf("the planted cookie did not land in the victim's jar")
	}

	if status := signInRequest(t, victim, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("victim sign in status = %d, want 200", status)
	}
	if got := sessionIDFromJar(t, victim, srv.URL); got == planted {
		t.Fatalf("the victim kept the planted session id after signing in")
	}
	if status := reportsStatus(t, victim, srv); status != http.StatusOK {
		t.Errorf("the victim after sign in: GET /api/reports = %d, want 200", status)
	}
	if status := reportsStatus(t, attacker, srv); status != http.StatusUnauthorized {
		t.Errorf("the attacker with the planted id: GET /api/reports = %d, want 401", status)
	}
	if id := sessionAccountID(t, pool, planted); id != nil {
		t.Errorf("the planted session %q was bound to account %d", planted, *id)
	}
}

// ---- Reset ends every session ---------------------------------------------------

func TestResetEndsEverySession(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-ends-sessions@example.com"
	one := newVerifiedClient(t, srv, mail, email)
	two := newVerifiedClient(t, srv, mail, email)
	if reportsStatus(t, one, srv) != http.StatusOK || reportsStatus(t, two, srv) != http.StatusOK {
		t.Fatalf("both browsers should be signed in before the reset")
	}
	ageClaim(t, pool, email)

	third := newLoginClient(t, srv)
	thirdID := sessionIDFromJar(t, third, srv.URL)
	if status := requestResetRequest(t, third, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	answer := doAuth(t, third, srv, "/api/auth/reset/code", map[string]string{"code": mail.codeFor(t, email), "password": secNewPassword})
	if answer.status != http.StatusOK {
		t.Fatalf("reset confirm status = %d, want 200: %s", answer.status, answer.body)
	}
	if answer.setCookie {
		t.Errorf("the reset answer set a cookie: it must not sign the resetting browser in")
	}

	if got := reportsStatus(t, one, srv); got != http.StatusUnauthorized {
		t.Errorf("first browser after the reset: GET /api/reports = %d, want 401", got)
	}
	if got := reportsStatus(t, two, srv); got != http.StatusUnauthorized {
		t.Errorf("second browser after the reset: GET /api/reports = %d, want 401", got)
	}
	if got := reportsStatus(t, third, srv); got != http.StatusUnauthorized {
		t.Errorf("the resetting browser: GET /api/reports = %d, want 401 (signed out)", got)
	}
	if got := sessionIDFromJar(t, third, srv.URL); got != thirdID {
		t.Errorf("the resetting browser changed its session id")
	}
	if id := sessionAccountID(t, pool, thirdID); id != nil {
		t.Errorf("the resetting browser's session is bound to account %d", *id)
	}

	old := doAuth(t, newLoginClient(t, srv), srv, "/api/auth/sign-in", map[string]string{"email": email, "password": e2eTestPassword})
	expectError(t, "old password after the reset", old, http.StatusBadRequest, "credentials", secWrongCredentialsSentence)
	if status := signInRequest(t, third, srv, email, secNewPassword); status != http.StatusOK {
		t.Fatalf("new password after the reset: status = %d, want 200", status)
	}
}

// ---- No address in any URL ----------------------------------------------------------

// urlRecorder is a transport that remembers every URL it was asked for and every
// Location header it received, redirect hops included.
type urlRecorder struct {
	mu        sync.Mutex
	next      http.RoundTripper
	urls      []string
	locations []string
}

func (r *urlRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.next.RoundTrip(req)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, req.URL.String())
	if err == nil {
		if loc := resp.Header.Get("Location"); loc != "" {
			r.locations = append(r.locations, loc)
		}
	}
	return resp, err
}

func TestNoEmailInAddress(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "no-email-in-address@example.com"

	recTransport := &http.Transport{DisableCompression: true}
	t.Cleanup(recTransport.CloseIdleConnections)
	rec := &urlRecorder{next: recTransport}
	newRecClient := func() *http.Client {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("constructing cookie jar: %v", err)
		}
		return &http.Client{Jar: jar, Transport: rec}
	}
	var bodies []string
	post := func(c *http.Client, path string, body any) {
		t.Helper()
		_, raw, _ := postAuthJSON(t, c, srv, path, body)
		bodies = append(bodies, string(raw))
	}

	a := newRecClient()
	getLoginPage(t, a, srv)
	post(a, "/api/auth/sign-up", map[string]string{"email": email, "password": e2eTestPassword})
	post(a, "/api/auth/code/resend", map[string]string{})
	post(a, "/api/auth/sign-up/code", map[string]string{"code": mail.codeFor(t, email)})
	getLoginPage(t, a, srv) // signed in, the page may send the person on
	b := newRecClient()
	getLoginPage(t, b, srv)
	post(b, "/api/auth/sign-in", map[string]string{"email": email, "password": secWrongPassword})
	post(b, "/api/auth/sign-in", map[string]string{"email": email, "password": e2eTestPassword})
	ageClaim(t, pool, email)
	c := newRecClient()
	getLoginPage(t, c, srv)
	post(c, "/api/auth/reset", map[string]string{"email": email})
	post(c, "/api/auth/reset/code", map[string]string{"code": mail.codeFor(t, email), "password": secNewPassword})
	getLoginPage(t, c, srv)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.urls) < 10 {
		t.Fatalf("recorded only %d requests, the test did not drive the flow", len(rec.urls))
	}
	for _, u := range rec.urls {
		if strings.Contains(u, "@") || strings.Contains(strings.ToLower(u), "%40") || strings.Contains(u, email) {
			t.Errorf("a request URL carries the address: %s", u)
		}
	}
	for _, loc := range rec.locations {
		if strings.Contains(loc, "@") || strings.Contains(strings.ToLower(loc), "%40") || strings.Contains(loc, email) {
			t.Errorf("a Location header carries the address: %s", loc)
		}
	}
	for _, body := range bodies {
		if strings.Contains(strings.ToLower(body), email) {
			t.Errorf("a JSON auth answer carries the address: %s", body)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Errorf("an auth answer is not JSON: %q", body)
		}
	}
}

// ---- A shared phone ---------------------------------------------------------------

func TestSignInCleansReplacedSessionPush(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		emailA = "shared-phone-a@example.com"
		emailB = "shared-phone-b@example.com"
	)
	phone := newVerifiedClient(t, srv, mail, emailA)
	createAccountViaCode(t, srv, mail, emailB)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	var oldCookie *http.Cookie
	for _, ck := range phone.Jar.Cookies(u) {
		if ck.Name == "pinalert_session" {
			oldCookie = ck
		}
	}
	if oldCookie == nil {
		t.Fatalf("the signed in phone holds no session cookie")
	}
	oldID := sessionIDFromJar(t, phone, srv.URL)
	accountA := sessionAccountID(t, pool, oldID)
	if accountA == nil {
		t.Fatalf("the phone's session is not bound to account A")
	}
	testutil.SeedPushSubscription(t, pool, *accountA, oldID, "")

	// The same phone now signs in as B.
	if status := signInRequest(t, phone, srv, emailB, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign in as B status = %d, want 200", status)
	}

	var subs int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM push_subscriptions WHERE session_id = $1", oldID).Scan(&subs); err != nil || subs != 0 {
		t.Errorf("push subscriptions of the replaced session = %d (err %v), want 0", subs, err)
	}
	var revoked *time.Time
	var bound *int64
	if err := pool.QueryRow(context.Background(), "SELECT revoked_at, account_id FROM sessions WHERE session_id = $1", oldID).Scan(&revoked, &bound); err != nil {
		t.Fatalf("reading the replaced session: %v", err)
	}
	if revoked == nil {
		t.Errorf("the replaced session was not revoked")
	}
	if bound == nil || *bound != *accountA {
		t.Errorf("the replaced session lost its account binding: %v, want %d", bound, *accountA)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+secReportsPath, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: oldCookie.Name, Value: oldCookie.Value})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/reports with the old cookie: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("old cookie: GET /api/reports = %d, want 401", resp.StatusCode)
	}
	if got := reportsStatus(t, phone, srv); got != http.StatusOK {
		t.Errorf("the phone after signing in as B: GET /api/reports = %d, want 200", got)
	}
}
