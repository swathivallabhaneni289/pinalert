// Package api's own test file (not api_test) so it can drive the real
// NewRouter through an httptest server while still reaching this package's
// unexported requireVerifiedAccount indirectly, the same way router.go
// itself is only reachable as a whole rather than middleware-by-middleware.
package api

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

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// gateFakeMailer captures the most recently mailed verification link so
// gate tests can extract the token and drive a real verify without a real
// mail provider — the same recording-mailer shape
// internal/api/handlers/auth_e2e_test.go already established for the
// handlers_test package; kept local here since internal/api cannot import
// that external test package.
type gateFakeMailer struct {
	mu   sync.Mutex
	link string
}

func (m *gateFakeMailer) SendVerificationLink(_ context.Context, _, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.link = link
	return nil
}

func (m *gateFakeMailer) lastLink() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.link
}

// newGateTestServer builds a real NewRouter router against a real,
// migrated Postgres (testutil.NewTestDB) with every dependency the gated
// and exempt routes both need, so the whole route table — not just
// requireVerifiedAccount in isolation — is under test.
func newGateTestServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *gateFakeMailer) {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("gate-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}

	queries := sqlcgen.New(pool)
	fm := &gateFakeMailer{}
	deps := Deps{
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
	}
	router := NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, pool, fm
}

// newGateJarClient returns a cookie-jar-backed client that never follows a
// redirect automatically — gate tests need to observe the 302 to /login
// itself, not whatever /login eventually returns.
func newGateJarClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// gateVerifySession drives the real request-link-then-verify flow for
// client against srv, so the browser it represents ends up bound to a
// verified account.
func gateVerifySession(t *testing.T, client *http.Client, srv *httptest.Server, mailer *gateFakeMailer, email string) {
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

	link := mailer.lastLink()
	parsedLink, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parsing mailed link %q: %v", link, err)
	}
	token := parsedLink.Query().Get("token")
	if token == "" {
		t.Fatalf("mailed link %q has no token query parameter", link)
	}

	resp2, err := client.Get(srv.URL + "/auth/verify?token=" + url.QueryEscape(token))
	if err != nil {
		t.Fatalf("GET /auth/verify: %v", err)
	}
	defer resp2.Body.Close()
	body, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("reading /auth/verify body: %v", err)
	}
	if resp2.StatusCode != http.StatusOK || !strings.Contains(string(body), "Email verified") {
		t.Fatalf("verifying session for %s: status=%d body=%s", email, resp2.StatusCode, body)
	}
}

// gateSessionIDFromJar extracts the unsigned session id portion of the
// pinalert_session cookie the jar currently holds for srvURL.
func gateSessionIDFromJar(t *testing.T, client *http.Client, srvURL string) string {
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

// TestAccessGateBlocksUnverified is this plan's core claim: an unverified
// session gets no app-shell markup from GET / (a redirect, not a partial
// render) and no report data from either method on /api/reports (a 401
// with the standard error envelope, and no row written on POST).
func TestAccessGateBlocksUnverified(t *testing.T) {
	srv, pool, _ := newGateTestServer(t)
	client := newGateJarClient(t)

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET / status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("GET / Location = %q, want %q", loc, "/login")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET / body: %v", err)
	}
	if strings.Contains(string(body), `id="app-shell"`) {
		t.Fatalf("unverified GET / response body contains the app-shell element: %s", body)
	}

	getResp, err := client.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("GET /api/reports status = %d, want 401: %s", getResp.StatusCode, b)
	}
	var errBody struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&errBody); err != nil {
		t.Fatalf("decoding GET /api/reports error body: %v", err)
	}
	if errBody.Error.Field != "auth" {
		t.Fatalf("GET /api/reports error field = %q, want %q", errBody.Error.Field, "auth")
	}

	postResp, err := client.Post(srv.URL+"/api/reports", "application/json", bytes.NewReader([]byte(`{
		"category": "flood",
		"severity": "low",
		"description": "unverified post attempt should be refused before any row is written",
		"latitude": 12.9716,
		"longitude": 77.5946
	}`)))
	if err != nil {
		t.Fatalf("POST /api/reports: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(postResp.Body)
		t.Fatalf("POST /api/reports status = %d, want 401: %s", postResp.StatusCode, b)
	}

	var reportCount int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&reportCount); err != nil {
		t.Fatalf("counting reports: %v", err)
	}
	if reportCount != 0 {
		t.Fatalf("expected zero reports rows after an unverified POST, got %d", reportCount)
	}
}

// TestAccessGateAllowsVerified proves the gate does not regress Phase 1's
// walking skeleton: a session verified through the real magic-link flow
// gets the app shell from GET / and report data from GET /api/reports,
// exactly as it did before this plan.
func TestAccessGateAllowsVerified(t *testing.T) {
	srv, _, mailer := newGateTestServer(t)
	client := newGateJarClient(t)

	gateVerifySession(t, client, srv, mailer, "verified-gate@example.com")

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading GET / body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `id="app-shell"`) {
		t.Fatalf("verified GET / response is missing the app-shell element: %s", body)
	}

	getResp, err := client.Get(srv.URL + "/api/reports?lat=12.9716&lon=77.5946")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("GET /api/reports status = %d, want 200: %s", getResp.StatusCode, b)
	}
}

// TestAccessGateExemptRoutes is a table-driven check over DEC-G's exempt
// list: an unverified session must reach every one of these five routes and
// get neither a redirect nor a 401.
func TestAccessGateExemptRoutes(t *testing.T) {
	srv, _, _ := newGateTestServer(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"login gate", http.MethodGet, "/login", ""},
		{"request link", http.MethodPost, "/api/auth/request-link", `{"email":"exempt-route@example.com"}`},
		{"verify landing", http.MethodGet, "/auth/verify", ""},
		{"static asset", http.MethodGet, "/static/css/main.css", ""},
		{"swagger ui", http.MethodGet, "/swagger/index.html", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newGateJarClient(t)

			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req, err := http.NewRequest(tc.method, srv.URL+tc.path, bodyReader)
			if err != nil {
				t.Fatalf("building request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.method, tc.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				t.Fatalf("%s %s returned a redirect (%d) — this route must be reachable without verification", tc.method, tc.path, resp.StatusCode)
			}
			if resp.StatusCode == http.StatusUnauthorized {
				t.Fatalf("%s %s returned 401 — this route must be reachable without verification", tc.method, tc.path)
			}
		})
	}
}

// TestAccessGateTreatsMissingSessionRowAsUnverified proves RESEARCH.md
// Pattern 2 end to end through the real gate: a browser whose sessions row
// is deleted out from under it is redirected to /login exactly like a
// never-verified visitor, never a 500.
func TestAccessGateTreatsMissingSessionRowAsUnverified(t *testing.T) {
	srv, pool, mailer := newGateTestServer(t)
	client := newGateJarClient(t)

	gateVerifySession(t, client, srv, mailer, "missing-row@example.com")
	sessionID := gateSessionIDFromJar(t, client, srv.URL)

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

	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET / status = %d, want %d (redirect, not 500) after the sessions row was deleted: %s", resp.StatusCode, http.StatusFound, b)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("GET / Location = %q, want %q", loc, "/login")
	}
}
