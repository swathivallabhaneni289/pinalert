// Package handlers_test also covers the request-a-magic-link slice: it
// drives the real router (internal/api.NewRouter) over a real HTTP server
// against a real Postgres, with a recording fake Mailer injected so the
// mailed link's token can be hashed and compared against what was actually
// persisted — the same real-router-plus-real-Postgres shape
// reports_e2e_test.go already established for the report submission slice.
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
	"time"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/auth"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"

	"github.com/jackc/pgx/v5/pgxpool"
)

// recordingMailer captures the most recent SendVerificationLink call so
// TestRequestLinkValidEmailMailsHashMatchingLink can assert the emailed
// link's token hashes to the persisted token_hash — not merely that some
// link was produced.
type recordingMailer struct {
	mu    sync.Mutex
	email string
	link  string
	sends int
}

func (m *recordingMailer) SendVerificationLink(ctx context.Context, email, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.email = email
	m.link = link
	m.sends++
	return nil
}

func (m *recordingMailer) last() (email, link string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.email, m.link
}

// sendCount reports how many times SendVerificationLink has been called,
// read under the same mutex last() uses — TestRequestLinkConcurrentSameAddressSendsExactlyOneEmail's
// core assertion.
func (m *recordingMailer) sendCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sends
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

// requestLink drives POST /api/auth/request-link for email using client
// (so the returned session cookie is captured in its jar) and returns the
// raw token pulled out of the mailer's captured link.
func requestLink(t *testing.T, client *http.Client, srv *httptest.Server, mailer *recordingMailer, email string) string {
	t.Helper()
	resp, err := client.Post(srv.URL+"/api/auth/request-link", "application/json",
		bytes.NewReader([]byte(`{"email":"`+email+`"}`)))
	if err != nil {
		t.Fatalf("POST /api/auth/request-link: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/auth/request-link status = %d, want 200: %s", resp.StatusCode, b)
	}

	_, link := mailer.last()
	parsedLink, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parsing mailed link %q: %v", link, err)
	}
	token := parsedLink.Query().Get("token")
	if token == "" {
		t.Fatalf("mailed link %q has no token query parameter", link)
	}
	return token
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

// getVerify drives GET /auth/verify?token=token using client and returns
// the response status and body.
func getVerify(t *testing.T, client *http.Client, srv *httptest.Server, token string) (int, string) {
	t.Helper()
	resp, err := client.Get(srv.URL + "/auth/verify?token=" + url.QueryEscape(token))
	if err != nil {
		t.Fatalf("GET /auth/verify: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /auth/verify body: %v", err)
	}
	return resp.StatusCode, string(body)
}

// newAuthE2EServer delegates to newAuthE2EServerWithRateLimit using the zero
// value, so RequestLinkRateLimitDefault is still what every existing call
// site gets — no existing test's signature or behavior changes.
func newAuthE2EServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *recordingMailer) {
	t.Helper()
	return newAuthE2EServerWithRateLimit(t, api.RequestLinkRateLimit{})
}

// newAuthE2EServerWithRateLimit is newAuthE2EServer's full implementation,
// taking an explicit per-IP rate limit so
// TestRequestLinkConcurrentSameAddressSendsExactlyOneEmail can open the
// limiter up (a high burst) and isolate the per-address cooldown as the
// only control under test.
func newAuthE2EServerWithRateLimit(t *testing.T, limit api.RequestLinkRateLimit) (*httptest.Server, *pgxpool.Pool, *recordingMailer) {
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
	fm := &recordingMailer{}
	deps := api.Deps{
		Session:              mgr,
		Sessions:             queries,
		Reports:              service.NewReportService(queries),
		AuthService:          service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:                 handlers.AuthConfig{AssetVersion: "test"},
		Template:             tmpl,
		RequestLinkRateLimit: limit,
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
	if !bytes.Contains(body, []byte(`id="login-email"`)) {
		t.Fatalf("GET /login response does not contain the email input")
	}
}

// TestRequestLinkValidEmailMailsHashMatchingLink is this plan's core
// end-to-end claim: a valid POST persists a magic_link_tokens row, and the
// token inside the link the fake Mailer captured hashes (via
// auth.HashToken, the same function the verify path in plan 01.1-02 will
// reuse) to exactly that row's stored token_hash — proving the raw secret
// never touches the database while the emailed link is still genuinely
// usable.
func TestRequestLinkValidEmailMailsHashMatchingLink(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)

	resp, err := http.Post(srv.URL+"/api/auth/request-link", "application/json",
		bytes.NewReader([]byte(`{"email":"visitor@example.com"}`)))
	if err != nil {
		t.Fatalf("POST /api/auth/request-link: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var out struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if out.Email != "visitor@example.com" {
		t.Fatalf("response email = %q, want visitor@example.com", out.Email)
	}

	mailedEmail, link := mailer.last()
	if mailedEmail != "visitor@example.com" {
		t.Fatalf("mailed email = %q, want visitor@example.com", mailedEmail)
	}

	parsedLink, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parsing mailed link %q: %v", link, err)
	}
	token := parsedLink.Query().Get("token")
	if token == "" {
		t.Fatalf("mailed link %q has no token query parameter", link)
	}

	var storedHash string
	err = pool.QueryRow(context.Background(),
		`SELECT token_hash FROM magic_link_tokens WHERE email = $1 ORDER BY created_at DESC LIMIT 1`,
		"visitor@example.com",
	).Scan(&storedHash)
	if err != nil {
		t.Fatalf("querying persisted token_hash: %v", err)
	}

	if auth.HashToken(token) != storedHash {
		t.Fatalf("hash of mailed token (%s) does not equal the persisted token_hash (%s)",
			auth.HashToken(token), storedHash)
	}
	if token == storedHash {
		t.Fatalf("the raw token must never equal the stored hash")
	}
}

// TestRequestLinkMalformedEmailReturns400 asserts the field-named 400 path
// and that no row was written for a malformed address.
func TestRequestLinkMalformedEmailReturns400(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)

	resp, err := http.Post(srv.URL+"/api/auth/request-link", "application/json",
		bytes.NewReader([]byte(`{"email":"not-an-email"}`)))
	if err != nil {
		t.Fatalf("POST /api/auth/request-link: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, b)
	}

	var out struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if out.Error.Field != "email" {
		t.Fatalf("error field = %q, want %q", out.Error.Field, "email")
	}

	if mailedEmail, _ := mailer.last(); mailedEmail != "" {
		t.Fatalf("mailer should never have been called for a malformed address, but saw %q", mailedEmail)
	}

	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM magic_link_tokens`).Scan(&count); err != nil {
		t.Fatalf("counting magic_link_tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero magic_link_tokens rows after a malformed request, got %d", count)
	}
}

// TestRequestLinkConcurrentSameAddressSendsExactlyOneEmail is this whole
// gap-closure plan's core claim (T-01-93): eight independent callers
// converging on one victim address must send exactly one email and mint
// exactly one token row, never eight. The server is stood up with the
// per-IP limiter deliberately opened (Burst: 100) — at the default burst of
// 5 the limiter would refuse most of these requests by itself and this test
// would pass green against the unfixed cooldown, proving nothing. With the
// limiter opened up, the per-address cooldown claim is the only control
// left standing between one address and eight emails.
//
// The eight clients are deliberately independent, with no shared cookie
// jar — the attack being modeled is eight unrelated callers converging on
// one victim address, not one browser retrying. A side effect is that the
// session middleware mints eight session ids and writes eight sessions
// rows concurrently; that noise is expected and irrelevant to the three
// assertions below, none of which reads the sessions table.
func TestRequestLinkConcurrentSameAddressSendsExactlyOneEmail(t *testing.T) {
	srv, pool, mailer := newAuthE2EServerWithRateLimit(t, api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond})

	const callers = 8
	const targetEmail = "mail-bomb-target@example.com"

	var wg sync.WaitGroup
	var mu sync.Mutex
	statusCodes := make([]int, 0, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{}
			resp, err := client.Post(srv.URL+"/api/auth/request-link", "application/json",
				bytes.NewReader([]byte(`{"email":"`+targetEmail+`"}`)))
			if err != nil {
				t.Errorf("POST /api/auth/request-link: %v", err)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
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
	if okCount != 1 {
		t.Fatalf("okCount = %d, want exactly 1 (statuses: %v)", okCount, statusCodes)
	}
	if refusedCount != callers-1 {
		t.Fatalf("refusedCount = %d, want exactly %d (statuses: %v)", refusedCount, callers-1, statusCodes)
	}

	if got := mailer.sendCount(); got != 1 {
		t.Fatalf("mailer.sendCount() = %d, want exactly 1", got)
	}

	var tokenRows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM magic_link_tokens WHERE email = $1`, targetEmail,
	).Scan(&tokenRows); err != nil {
		t.Fatalf("counting magic_link_tokens for %s: %v", targetEmail, err)
	}
	if tokenRows != 1 {
		t.Fatalf("magic_link_tokens rows for %s = %d, want exactly 1", targetEmail, tokenRows)
	}
}

// TestVerifyEndToEnd is this plan's core end-to-end claim: requesting a
// link and then clicking it in the same browser (same cookie jar) marks
// that browser's session verified, renders "Email verified", and sets
// sessions.account_id for that session — not merely a 200 status.
func TestVerifyEndToEnd(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	token := requestLink(t, client, srv, mailer, "verify-e2e@example.com")

	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK {
		t.Fatalf("GET /auth/verify status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, "Email verified") {
		t.Fatalf("verify response does not contain \"Email verified\": %s", body)
	}

	sessionID := sessionIDFromJar(t, client, srv.URL)
	var accountID *int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sessionID).Scan(&accountID); err != nil {
		t.Fatalf("reading back sessions.account_id: %v", err)
	}
	if accountID == nil {
		t.Fatalf("sessions.account_id is NULL after a successful verify")
	}
}

// TestVerifyRejectsSecondUse proves re-clicking the exact same link renders
// the already-used outcome and does not change anything further.
func TestVerifyRejectsSecondUse(t *testing.T) {
	srv, _, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	token := requestLink(t, client, srv, mailer, "second-use@example.com")

	firstStatus, firstBody := getVerify(t, client, srv, token)
	if firstStatus != http.StatusOK || !strings.Contains(firstBody, "Email verified") {
		t.Fatalf("first click: status=%d body=%s, want 200 with \"Email verified\"", firstStatus, firstBody)
	}

	secondStatus, secondBody := getVerify(t, client, srv, token)
	if secondStatus != http.StatusOK {
		t.Fatalf("second click status = %d, want 200", secondStatus)
	}
	if !strings.Contains(secondBody, "This link was already used") {
		t.Fatalf("second click does not contain \"This link was already used\": %s", secondBody)
	}
}

// TestVerifyExpiredToken inserts a token row whose expires_at is already
// past and asserts the expired outcome renders.
func TestVerifyExpiredToken(t *testing.T) {
	srv, pool, _ := newAuthE2EServer(t)
	client := newJarClient(t)

	raw, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("auth.GenerateToken: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO magic_link_tokens (token_hash, email, expires_at) VALUES ($1, $2, $3)`,
		hash, "expired-e2e@example.com", time.Now().Add(-1*time.Minute),
	); err != nil {
		t.Fatalf("inserting expired magic_link_tokens row: %v", err)
	}

	status, body := getVerify(t, client, srv, raw)
	if status != http.StatusOK {
		t.Fatalf("GET /auth/verify status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, "This link has expired") {
		t.Fatalf("verify response does not contain \"This link has expired\": %s", body)
	}
}

// TestVerifyMalformedToken asserts a token that cannot possibly be a real
// GenerateToken output renders the malformed outcome.
func TestVerifyMalformedToken(t *testing.T) {
	srv, _, _ := newAuthE2EServer(t)
	client := newJarClient(t)

	status, body := getVerify(t, client, srv, "not-a-real-token")
	if status != http.StatusOK {
		t.Fatalf("GET /auth/verify status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, "This link isn't valid") {
		t.Fatalf("verify response does not contain \"This link isn't valid\": %s", body)
	}
}

// TestVerifyAccountConflict is DEC-D's core claim: a browser already
// verified as one account that opens a token minted for a second address
// is refused, not silently re-bound, and the second token is left
// unconsumed.
func TestVerifyAccountConflict(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	tokenA := requestLink(t, client, srv, mailer, "conflict-a@example.com")
	statusA, bodyA := getVerify(t, client, srv, tokenA)
	if statusA != http.StatusOK || !strings.Contains(bodyA, "Email verified") {
		t.Fatalf("verifying account A: status=%d body=%s", statusA, bodyA)
	}
	sessionID := sessionIDFromJar(t, client, srv.URL)
	var accountIDBefore *int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sessionID).Scan(&accountIDBefore); err != nil {
		t.Fatalf("reading account_id after verifying A: %v", err)
	}
	if accountIDBefore == nil {
		t.Fatalf("account_id is NULL after verifying A")
	}

	// Same jar (same browser, still verified as A) requests and opens a
	// second address's link.
	tokenB := requestLink(t, client, srv, mailer, "conflict-b@example.com")
	statusB, bodyB := getVerify(t, client, srv, tokenB)
	if statusB != http.StatusOK {
		t.Fatalf("GET /auth/verify (conflict) status = %d, want 200: %s", statusB, bodyB)
	}
	if !strings.Contains(bodyB, "This link is for a different account") {
		t.Fatalf("conflict response does not contain the conflict heading: %s", bodyB)
	}
	if !strings.Contains(bodyB, "conflict-a@example.com") {
		t.Fatalf("conflict response does not name the currently-signed-in address: %s", bodyB)
	}

	var accountIDAfter *int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sessionID).Scan(&accountIDAfter); err != nil {
		t.Fatalf("reading account_id after the conflict attempt: %v", err)
	}
	if *accountIDAfter != *accountIDBefore {
		t.Fatalf("account_id changed after a conflict outcome: before=%d after=%d", *accountIDBefore, *accountIDAfter)
	}

	var usedAt *time.Time
	if err := pool.QueryRow(context.Background(),
		`SELECT used_at FROM magic_link_tokens WHERE token_hash = $1`, auth.HashToken(tokenB),
	).Scan(&usedAt); err != nil {
		t.Fatalf("reading back token B's used_at: %v", err)
	}
	if usedAt != nil {
		t.Fatalf("token B's used_at = %v after a conflict outcome, want NULL (unconsumed)", *usedAt)
	}
}

// TestVerifyBindsWhenSessionRowAbsent proves the upsert path from Task 1
// (RESEARCH.md Pitfall 2): a browser whose sessions row is deleted between
// requesting a link and clicking it still ends up verified, with a fresh
// sessions row created carrying the bound account_id.
func TestVerifyBindsWhenSessionRowAbsent(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	token := requestLink(t, client, srv, mailer, "absent-row@example.com")

	sessionID := sessionIDFromJar(t, client, srv.URL)
	if _, err := pool.Exec(context.Background(), `DELETE FROM sessions WHERE session_id = $1`, sessionID); err != nil {
		t.Fatalf("deleting sessions row to simulate the persist-failure path: %v", err)
	}
	var rowCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE session_id = $1`, sessionID).Scan(&rowCount); err != nil {
		t.Fatalf("counting sessions rows after delete: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("expected zero sessions rows after delete, got %d", rowCount)
	}

	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK {
		t.Fatalf("GET /auth/verify status = %d, want 200: %s", status, body)
	}
	if !strings.Contains(body, "Email verified") {
		t.Fatalf("verify response does not contain \"Email verified\": %s", body)
	}

	var accountID *int64
	if err := pool.QueryRow(context.Background(), `SELECT account_id FROM sessions WHERE session_id = $1`, sessionID).Scan(&accountID); err != nil {
		t.Fatalf("reading back sessions.account_id after bind-through-absent-row: %v", err)
	}
	if accountID == nil {
		t.Fatalf("sessions.account_id is NULL after verifying with no pre-existing sessions row")
	}
}

// --- Profile page (01.1-07-PLAN.md Task 2) ---

// TestProfileListsOwnReportsAcrossSessions is this plan's core claim: an
// account's reports filed from two different sessions (devices) both
// appear on one GET /profile response read through the FIRST device's
// cookie jar. The second device is simulated by binding a freshly-issued
// session directly to the same account row (sqlcgen.BindSessionAccount)
// rather than running it through a second real RequestLink/Verify round
// trip, which would collide with RequestLink's own per-address resend
// cooldown (D-03/D-04) inside this test — the DB-level bind is exactly what
// a second successful verification of the same address would leave behind.
func TestProfileListsOwnReportsAcrossSessions(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)
	clientA := newJarClient(t)

	token := requestLink(t, clientA, srv, mailer, "profile-multi@example.com")
	status, body := getVerify(t, clientA, srv, token)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("verifying device A: status=%d body=%s", status, body)
	}
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
	if err := q.BindSessionAccount(context.Background(), sqlcgen.BindSessionAccountParams{SessionID: sessionB, AccountID: &accountID}); err != nil {
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

	token := requestLink(t, client, srv, mailer, "profile-empty@example.com")
	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("verifying: status=%d body=%s", status, body)
	}

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

	token := requestLink(t, client, srv, mailer, "profile-header@example.com")
	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("verifying: status=%d body=%s", status, body)
	}

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

	wantSubstrings := []string{
		`aria-label="Account menu"`,
		`aria-haspopup="menu"`,
		`role="menu"`,
		"profile-header@example.com",
		`href="/profile"`,
		`action="/auth/logout"`,
		"account-menu.js?v=",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(bodyStr, want) {
			t.Errorf("/profile is missing shared-header contract substring: %s", want)
		}
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

	token := requestLink(t, client, srv, mailer, "logout-e2e@example.com")
	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("verifying: status=%d body=%s", status, body)
	}

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
// unbinds sessions.account_id, so re-verifying the same address afterward
// restores the full report history. Re-verification here inserts a fresh
// magic_link_tokens row directly (the same technique TestVerifyExpiredToken
// above uses) rather than a second real POST /api/auth/request-link for
// the same address, which would collide with RequestLink's own per-address
// resend cooldown (D-03/D-04) purely as an artifact of running two requests
// for one address inside a single fast test — GET /auth/verify, the actual
// re-verification event this test needs to prove, still runs for real
// against the real router either way.
func TestLogoutPreservesReportHistory(t *testing.T) {
	srv, pool, mailer := newAuthE2EServer(t)
	client := newJarClient(t)

	token := requestLink(t, client, srv, mailer, "logout-preserve@example.com")
	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("verifying: status=%d body=%s", status, body)
	}
	submitReport(t, client, srv, "flood", "report before logout")

	logoutResp, err := client.Post(srv.URL+"/auth/logout", "", nil)
	if err != nil {
		t.Fatalf("POST /auth/logout: %v", err)
	}
	logoutResp.Body.Close()

	raw, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("auth.GenerateToken: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO magic_link_tokens (token_hash, email, expires_at) VALUES ($1, $2, $3)`,
		hash, "logout-preserve@example.com", time.Now().Add(5*time.Minute),
	); err != nil {
		t.Fatalf("inserting a fresh magic_link_tokens row for re-verification: %v", err)
	}

	status2, body2 := getVerify(t, client, srv, raw)
	if status2 != http.StatusOK || !strings.Contains(body2, "Email verified") {
		t.Fatalf("re-verifying after logout: status=%d body=%s", status2, body2)
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
