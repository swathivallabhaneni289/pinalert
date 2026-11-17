// Package handlers_test also covers the sign in slice of the login page, the
// Profile page and Logout: it drives the real router (internal/api.NewRouter)
// over a real HTTP server against a real Postgres, with a recording fake Mailer
// injected so the code it mails can be typed back. The same
// real-router-plus-real-Postgres shape reports_e2e_test.go established for the
// report submission slice.
//
// Amended 2026-10-29 by Phase 7.1 plan 14: the emailed link tests that used to
// live here are gone with the link; the code tests that took over are below.
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
	"strings"
	"sync"
	"testing"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/auth"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

// recordingMailer captures the auth emails by address (codeFor, authSendCount).
// Amended 2026-10-29 by Phase 7.1 plan 14b: the link members are gone with the
// link method on the mailer seam.
type recordingMailer struct {
	mu sync.Mutex
	// auth holds every auth email by address, oldest first, and authSends
	// counts them (plan 07.1-03).
	auth      map[string][]mailer.AuthEmail
	authSends int
}

// SendAuthEmail records an auth email under the address it was sent to.
func (m *recordingMailer) SendAuthEmail(ctx context.Context, to string, msg mailer.AuthEmail) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.auth == nil {
		m.auth = map[string][]mailer.AuthEmail{}
	}
	m.auth[to] = append(m.auth[to], msg)
	m.authSends++
	return nil
}

// lastAuthFor returns the newest auth email sent to email.
func (m *recordingMailer) lastAuthFor(email string) (mailer.AuthEmail, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sent := m.auth[email]
	if len(sent) == 0 {
		return mailer.AuthEmail{}, false
	}
	return sent[len(sent)-1], true
}

// codeFor returns the code of the newest auth email sent to email. It fails
// the test when nothing was sent to that address or when the newest email
// carries no code (an account_exists or no_account email).
func (m *recordingMailer) codeFor(t *testing.T, email string) string {
	t.Helper()
	msg, ok := m.lastAuthFor(email)
	if !ok {
		t.Fatalf("no auth email was sent to %q", email)
	}
	if msg.Code == "" {
		t.Fatalf("the newest auth email to %q is %q and has no code", email, msg.Kind)
	}
	return msg.Code
}

// authSendCount reports how many auth emails have been sent in all.
func (m *recordingMailer) authSendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.authSends
}

// newJarClient returns an *http.Client backed by a fresh in-memory cookie
// jar, so a session cookie set by one request (e.g. requesting a link) is
// carried automatically to a later request against the same server (e.g.
// clicking the link) — the same browser identity across both requests,
// exactly what the verify flow depends on.
func newJarClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}

// sessionIDFromJar extracts the unsigned session id portion (before the
// "." HMAC signature) of the pinalert_session cookie the jar is currently
// holding for srvURL.
func sessionIDFromJar(t *testing.T, client *http.Client, srvURL string) string {
	t.Helper()
	parsed, err := url.Parse(srvURL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	for _, c := range client.Jar.Cookies(parsed) {
		if c.Name == "pinalert_session" {
			id := strings.SplitN(c.Value, ".", 2)[0]
			if id == "" {
				t.Fatalf("session cookie %q has no id portion before the signature", c.Value)
			}
			return id
		}
	}
	t.Fatalf("no pinalert_session cookie found in jar for %s", srvURL)
	return ""
}

// submitReport drives POST /api/reports for category/description using
// client, failing the test on anything but 201. 01.1-07's profile tests
// need real report rows to list, the same real-router-plus-real-Postgres
// shape every other e2e test in this file already uses.
func submitReport(t *testing.T, client *http.Client, srv *httptest.Server, category, description string) {
	t.Helper()
	payload := map[string]any{
		"category":    category,
		"severity":    "low",
		"description": description,
		"latitude":    12.9716,
		"longitude":   77.5946,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshaling report payload: %v", err)
	}
	resp, err := client.Post(srv.URL+"/api/reports", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /api/reports: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/reports status = %d, want 201: %s", resp.StatusCode, body)
	}
}

// newAuthE2EServer builds the real router over a fresh test database with the
// code and password service, the test hasher and open auth buckets.
func newAuthE2EServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *recordingMailer) {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("auth-e2e-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	queries := sqlcgen.New(pool)
	// Amended 2026-10-29 by Phase 9 plan 14 (D-02): the same ratings source
	// cmd/server/main.go wires, so the Activity read in this suite runs the wired
	// path.
	ratings := service.NewRatingService(queries)
	fm := &recordingMailer{}
	deps := api.Deps{
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries, service.WithReportSignals(ratings)),
		AuthService: service.NewAuthService(queries, fm, "https://pinalert.example", service.WithAuthSignals(ratings)),
		Ratings:     ratings,
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
		// Amended 2026-10-29 by Phase 7.1 plan 08: the code and password
		// service with the test hasher and open buckets, so a test that signs
		// in through verifySession never meets a 503 or a bucket.
		Credentials: newTestCredentials(queries, fm),
		AuthLimits:  e2eGenerousAuthLimits,
	}
	router := api.NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, pool, fm
}

// TestLoginGateServesInteractiveForm asserts GET /login returns 200 and
// contains the email input id — the field is interactive from first paint
// per D-06, so its mere presence in the initial response (not something
// added later by client-side JS) is the load-bearing claim.
func TestLoginGateServesInteractiveForm(t *testing.T) {
	srv, _, _ := newAuthE2EServer(t)

	resp, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /login body: %v", err)
	}
	// Amended 2026-10-29 by Phase 7.1 plan 11: the page opens on Create account
	// with its email field (id create-email) in the first response, and the Sign in
	// panel's field too; the old single login-email field is gone. The claim is
	// unchanged: the fields are in the initial response, not added by script.
	for _, want := range []string{`id="create-email"`, `id="signin-email"`, `id="create-password"`, `id="login-tab-create"`} {
		if !bytes.Contains(body, []byte(want)) {
			t.Fatalf("GET /login response does not contain %s", want)
		}
	}
	if !bytes.Contains(body, []byte(`data-start-panel="create"`)) {
		t.Fatalf("GET /login does not open on Create account")
	}
}

// Amended 2026-10-29 by Phase 7.1 plan 14: the link tests that stood here
// (TestRequestLinkValidEmailMailsHashMatchingLink, TestRequestLinkMalformedEmailReturns400,
// TestRequestLinkConcurrentSameAddressSendsExactlyOneEmail, TestVerifyEndToEnd,
// TestVerifyRejectsSecondUse, TestVerifyExpiredToken, TestVerifyMalformedToken,
// TestVerifyAccountConflict and TestVerifyBindsWhenSessionRowAbsent) are removed
// with the emailed link. The properties they pinned live on as the tests below
// and in the credential tests; the SUMMARY of plan 14 maps each one.

// TestSignUpMalformedEmailReturns400 asserts the field named 400 path of Create
// account and that a malformed address mails nothing and writes no code row.
func TestSignUpMalformedEmailReturns400(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	client := newLoginClient(t, srv)

	status, raw, _ := postAuthJSON(t, client, srv, "/api/auth/sign-up", map[string]string{"email": "not-an-email", "password": e2eTestPassword})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", status, raw)
	}
	if field, _ := errorField(t, raw); field != "email" {
		t.Fatalf("error field = %q, want %q", field, "email")
	}
	if got := mail.authSendCount(); got != 0 {
		t.Fatalf("a malformed address mailed %d emails, want 0", got)
	}
	var rows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM auth_codes`).Scan(&rows); err != nil {
		t.Fatalf("counting auth_codes: %v", err)
	}
	if rows != 0 {
		t.Fatalf("auth_codes rows after a malformed address = %d, want 0", rows)
	}
}

// TestSignUpConcurrentSameAddressSendsOneEmail is the code flow's form of the
// mail bomb claim (T-01-93): eight independent callers converging on one victim
// address must send exactly one email and write exactly one code row, never
// eight. The server is built with the production send limits (the 45 second
// per address claim; the harness default of one millisecond would let every
// caller through), so the per address claim is the only control under test: the
// per device and per network buckets stay open. The eight clients share no
// cookie jar, because the attack being modeled is eight unrelated callers, not
// one browser retrying.
func TestSignUpConcurrentSameAddressSendsOneEmail(t *testing.T) {
	mail := &recordingMailer{}
	srv, _, pool := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.Credentials = service.NewCredentialService(q, mail,
			service.WithHasher(auth.NewHasher(auth.TestParams())),
			service.WithCodeKey(bytes.Repeat([]byte("c"), 32)),
			service.WithThrottleKey(bytes.Repeat([]byte("t"), 32)),
		)
	})

	const callers = 8
	const targetEmail = "mail-bomb-target@example.com"

	var wg sync.WaitGroup
	var mu sync.Mutex
	statusCodes := make([]int, 0, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, err := json.Marshal(map[string]string{"email": targetEmail, "password": e2eTestPassword})
			if err != nil {
				t.Errorf("marshalling the request: %v", err)
				return
			}
			resp, err := (&http.Client{}).Post(srv.URL+"/api/auth/sign-up", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Errorf("POST /api/auth/sign-up: %v", err)
				return
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			mu.Lock()
			statusCodes = append(statusCodes, resp.StatusCode)
			mu.Unlock()
		}()
	}
	wg.Wait()

	okCount, refusedCount := 0, 0
	for _, code := range statusCodes {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			refusedCount++
		default:
			t.Fatalf("unexpected status code %d among %v", code, statusCodes)
		}
	}
	if okCount != 1 || refusedCount != callers-1 {
		t.Fatalf("answers = %d ok and %d refused, want 1 and %d (statuses: %v)", okCount, refusedCount, callers-1, statusCodes)
	}
	if got := mail.authSendCount(); got != 1 {
		t.Fatalf("emails sent = %d, want exactly 1", got)
	}
	var rows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM auth_codes WHERE email = $1`, targetEmail).Scan(&rows); err != nil {
		t.Fatalf("counting auth_codes for %s: %v", targetEmail, err)
	}
	if rows != 1 {
		t.Fatalf("auth_codes rows for %s = %d, want exactly 1", targetEmail, rows)
	}
}

// TestConfirmCodeBindsWhenSessionRowAbsent proves the upsert path (research
// Pitfall 2): a browser whose sessions row is gone between asking for a code and
// typing it still ends up signed in, on a fresh session id whose row carries the
// account. The session middleware logs and continues when its own write fails, so
// a valid signed cookie with no row behind it is a real state.
func TestConfirmCodeBindsWhenSessionRowAbsent(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	client := newLoginClient(t, srv)
	const email = "absent-row@example.com"

	if status := signUpRequest(t, client, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)

	before := sessionIDFromJar(t, client, srv.URL)
	if _, err := pool.Exec(context.Background(), `DELETE FROM sessions WHERE session_id = $1`, before); err != nil {
		t.Fatalf("deleting the sessions row: %v", err)
	}
	var rowCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE session_id = $1`, before).Scan(&rowCount); err != nil || rowCount != 0 {
		t.Fatalf("sessions rows after the delete = %d (err %v), want 0", rowCount, err)
	}

	if status := confirmSignUpCode(t, client, srv, code); status != http.StatusOK {
		t.Fatalf("confirm code status = %d, want 200", status)
	}

	after := sessionIDFromJar(t, client, srv.URL)
	if after == before {
		t.Fatalf("the session id did not change at sign in")
	}
	var accountID *int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, after).Scan(&accountID); err != nil {
		t.Fatalf("reading sessions.account_id of the new session: %v", err)
	}
	if accountID == nil {
		t.Fatalf("sessions.account_id is NULL after confirming with no pre existing sessions row")
	}
}

// TestConfirmCodeSecondUseRefused proves a typed code works once: replaying the
// same code with the cookie the browser had before it signed in creates nothing
// more. One account exists and no code row is left for the address.
func TestConfirmCodeSecondUseRefused(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	client := newLoginClient(t, srv)
	const email = "second-use@example.com"

	if status := signUpRequest(t, client, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)

	parsed, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing the server URL: %v", err)
	}
	var old *http.Cookie
	for _, c := range client.Jar.Cookies(parsed) {
		if c.Name == "pinalert_session" {
			old = c
		}
	}
	if old == nil {
		t.Fatalf("no pinalert_session cookie before the confirm")
	}

	if status := confirmSignUpCode(t, client, srv, code); status != http.StatusOK {
		t.Fatalf("first confirm status = %d, want 200", status)
	}

	replayer := newPlainJarClient(t)
	replayer.Jar.SetCookies(parsed, []*http.Cookie{old})
	a := doAuth(t, replayer, srv, "/api/auth/sign-up/code", map[string]string{"code": code})
	expectError(t, "the same code a second time", a, http.StatusBadRequest, "code", secWrongCodeSentence)

	var accounts int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM accounts WHERE email = $1`, email).Scan(&accounts); err != nil || accounts != 1 {
		t.Fatalf("accounts for %s = %d (err %v), want exactly 1", email, accounts, err)
	}
	// A confirmed code is consumed and every code row of the address is deleted,
	// so nothing is left to replay.
	// Amended 2026-10-29 by the Phase 7.1 review fix round 3 (skeptic SK2-01):
	// the spent row itself is kept, used and with no password hash, so a browser
	// whose confirm answer was lost can be told its code was confirmed. The
	// replay above shows it cannot be used again.
	var left, unused, withHash int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*), count(*) FILTER (WHERE used_at IS NULL), count(password_hash) FROM auth_codes WHERE email = $1`,
		email).Scan(&left, &unused, &withHash); err != nil || left != 1 || unused != 0 || withHash != 0 {
		t.Fatalf("code rows left for %s = %d (%d unused, %d with a hash, err %v), want only the spent row", email, left, unused, withHash, err)
	}
}

// TestConfirmCodeExpired proves, against the real query, that a code past its
// expiry is refused with the shared sentence and creates no account. The service
// tests pin the same rule with a fake clock and a fake store.
func TestConfirmCodeExpired(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	client := newLoginClient(t, srv)
	const email = "expired-code@example.com"

	if status := signUpRequest(t, client, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)
	if _, err := pool.Exec(context.Background(), `UPDATE auth_codes SET expires_at = now() - interval '1 minute' WHERE email = $1`, email); err != nil {
		t.Fatalf("expiring the code: %v", err)
	}

	a := doAuth(t, client, srv, "/api/auth/sign-up/code", map[string]string{"code": code})
	expectError(t, "an expired code", a, http.StatusBadRequest, "code", secWrongCodeSentence)
	var accounts int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM accounts WHERE email = $1`, email).Scan(&accounts); err != nil || accounts != 0 {
		t.Fatalf("accounts for %s = %d (err %v), want 0", email, accounts, err)
	}
}

// --- Profile page (01.1-07-PLAN.md Task 2) ---

// TestProfileListsOwnReportsAcrossSessions is this plan's core claim: an
// account's reports filed from two different sessions (devices) both
// appear on one GET /profile response read through the FIRST device's
// cookie jar. The second device is simulated by binding a freshly-issued
// session directly to the same account row (sqlcgen.BindSessionAccount)
// rather than running it through a second real sign in; the DB-level bind
// is exactly what a second successful sign in to the same account would
// leave behind.
//
// Amended 2026-10-29 by Phase 7.1 plan 14: device A signs in with verifySession
// (the password flow) instead of requesting and opening an emailed link.
func TestProfileListsOwnReportsAcrossSessions(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)
	clientA := newJarClient(t)

	verifySession(t, clientA, srv, mailer, "profile-multi@example.com")
	submitReport(t, clientA, srv, "flood", "report from device A")

	sessionA := sessionIDFromJar(t, clientA, srv.URL)
	var accountID int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sessionA).Scan(&accountID); err != nil {
		t.Fatalf("reading device A's account_id: %v", err)
	}

	clientB := newJarClient(t)
	if resp, err := clientB.Get(srv.URL + "/login"); err != nil {
		t.Fatalf("GET /login (issuing device B's session cookie): %v", err)
	} else {
		resp.Body.Close()
	}
	sessionB := sessionIDFromJar(t, clientB, srv.URL)
	q := sqlcgen.New(pool)
	if _, err := q.BindSessionAccount(context.Background(), sqlcgen.BindSessionAccountParams{SessionID: sessionB, AccountID: &accountID}); err != nil {
		t.Fatalf("binding device B to the same account: %v", err)
	}
	submitReport(t, clientB, srv, "fire", "report from device B")

	resp, err := clientA.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /profile status = %d, want 200: %s", resp.StatusCode, b)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile body: %v", err)
	}
	bodyStr := string(respBody)
	if !strings.Contains(bodyStr, "report from device A") {
		t.Errorf("/profile is missing device A's report: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "report from device B") {
		t.Errorf("/profile is missing device B's report: %s", bodyStr)
	}
}

// TestProfileEmptyState proves a freshly-verified account with no reports
// sees the honest "No reports yet" empty state — never a fabricated row.
func TestProfileEmptyState(t *testing.T) {
	srv, _, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	verifySession(t, client, srv, mailer, "profile-empty@example.com")

	resp, err := client.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /profile status = %d, want 200: %s", resp.StatusCode, b)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile body: %v", err)
	}
	if !strings.Contains(string(respBody), "No reports yet") {
		t.Fatalf("/profile does not contain the empty-state heading: %s", respBody)
	}
}

// TestProfileRequiresVerification proves an unverified session hitting
// GET /profile is redirected by the gate (302 to /login) rather than served
// the page — the same gate every other protected route already goes
// through.
func TestProfileRequiresVerification(t *testing.T) {
	srv, _, _ := newAuthE2EServer(t)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET /profile (unverified) status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

// TestProfileRendersSharedHeader is 01.1-06's header contract, re-asserted
// against this second page (01.1-07-PLAN.md Task 2): the account trigger's
// aria-label, the verified address, the /profile and /auth/logout menu
// targets, the account-menu.js tag with a cache-busting ?v= suffix, and
// Cache-Control: no-store — proving the header renders identically here as
// a fact, not merely an intention.
func TestProfileRendersSharedHeader(t *testing.T) {
	srv, _, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	verifySession(t, client, srv, mailer, "profile-header@example.com")

	resp, err := client.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /profile status = %d, want 200: %s", resp.StatusCode, b)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want %q (DEC-T)", cc, "no-store")
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile body: %v", err)
	}
	bodyStr := string(respBody)

	// Amended 2026-11-17 (quick 261117-lq0): the menu's links are /me and
	// /settings (the menu is unchanged), and the link to /profile was the bar's
	// My activity, which the bar no longer draws; so the menu target pinned here
	// is /me.
	wantSubstrings := []string{
		`aria-label="Account menu"`,
		`aria-haspopup="menu"`,
		`role="menu"`,
		`role="menuitem" class="account-menu__item" href="/me"`,
		`action="/auth/logout"`,
		"account-menu.js?v=",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("/profile is missing shared-header contract substring: %s", want)
		}
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21,
	// T-34-32): the signed in address is no longer printed on the page, because
	// the page's own email line is removed and the person menu does not show it.
	// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
	if strings.Contains(bodyStr, "profile-header@example.com") {
		t.Errorf("/profile prints the signed in address")
	}
}

// --- Logout (01.1-07-PLAN.md Task 3) ---

// TestLogoutReturnsToLoginGate proves POST /auth/logout actually clears the
// browser's session cookie: a verified GET / succeeds before logout, the
// logout response itself redirects to /login with StatusSeeOther, and a
// follow-up GET / with the SAME client (jar) is gated again — the cleared
// cookie, not merely the redirect response, is what makes the second GET
// come back unverified.
func TestLogoutReturnsToLoginGate(t *testing.T) {
	srv, _, mailer := newAuthE2EServer(t)
	client := newJarClient(t)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	verifySession(t, client, srv, mailer, "logout-e2e@example.com")

	beforeResp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / (verified): %v", err)
	}
	beforeResp.Body.Close()
	if beforeResp.StatusCode != http.StatusOK {
		t.Fatalf("GET / (verified) status = %d, want 200", beforeResp.StatusCode)
	}

	logoutResp, err := client.Post(srv.URL+"/auth/logout", "", nil)
	if err != nil {
		t.Fatalf("POST /auth/logout: %v", err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /auth/logout status = %d, want %d", logoutResp.StatusCode, http.StatusSeeOther)
	}
	if loc := logoutResp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("POST /auth/logout Location = %q, want /login", loc)
	}

	afterResp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET / (after logout): %v", err)
	}
	defer afterResp.Body.Close()
	if afterResp.StatusCode != http.StatusFound {
		t.Fatalf("GET / (after logout) status = %d, want %d (gated again)", afterResp.StatusCode, http.StatusFound)
	}
	if loc := afterResp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("GET / (after logout) Location = %q, want /login", loc)
	}
}

// TestLogoutPreservesReportHistory is DEC-K's core claim: logging out never
// unbinds sessions.account_id, so signing in again to the same account
// afterward restores the full report history.
//
// Amended 2026-10-29 by Phase 7.1 plan 14: the second sign in is a real
// POST /api/auth/sign-in with the password. It used to insert a sign in
// link token row and open GET /auth/verify.
func TestLogoutPreservesReportHistory(t *testing.T) {
	srv, _, mailer := newAuthE2EServer(t)
	client := newJarClient(t)
	const email = "logout-preserve@example.com"

	verifySession(t, client, srv, mailer, email)
	submitReport(t, client, srv, "flood", "report before logout")

	logoutResp, err := client.Post(srv.URL+"/auth/logout", "", nil)
	if err != nil {
		t.Fatalf("POST /auth/logout: %v", err)
	}
	logoutResp.Body.Close()

	if status := signInRequest(t, client, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("signing in again after logout: status=%d, want 200", status)
	}

	resp, err := client.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile body: %v", err)
	}
	if !strings.Contains(string(respBody), "report before logout") {
		t.Fatalf("/profile after re-verification is missing the report filed before logout (DEC-K): %s", respBody)
	}
}
