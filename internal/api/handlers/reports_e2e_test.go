// Package handlers_test drives the real router (internal/api.NewRouter)
// over a real HTTP server against a real Postgres — the automated proof
// that the Walking Skeleton's server half works end to end. It lives
// outside package handlers because it needs to import internal/api, which
// imports internal/api/handlers itself.
package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// newE2EServer builds a real router with every dependency the access gate
// added in 01.1-04 needs, not just the reports slice — POST and GET
// /api/reports are now both gated (D-05), so every caller of this helper
// must verify a session (see verifySession) before driving either endpoint.
// The returned *recordingMailer is the same fake-mailer harness
// auth_e2e_test.go established in 01.1-02, reused here rather than
// duplicated.
func newE2EServer(t *testing.T) (*httptest.Server, *recordingMailer) {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("e2e-test-secret-not-for-production"), false)
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
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
	}
	router := api.NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, fm
}

// verifySession drives the real request-link-then-verify flow for client
// against srv (reusing requestLink/getVerify from auth_e2e_test.go), so the
// session it represents is bound to a verified account before the caller
// drives /api/reports — the access gate added in 01.1-04 refuses an
// unverified session with 401, not report data.
func verifySession(t *testing.T, client *http.Client, srv *httptest.Server, mailer *recordingMailer, email string) {
	t.Helper()
	token := requestLink(t, client, srv, mailer, email)
	status, body := getVerify(t, client, srv, token)
	if status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("verifying session for %s: status=%d body=%s", email, status, body)
	}
}

func postReport(t *testing.T, client *http.Client, baseURL string, body map[string]any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling request body: %v", err)
	}
	resp, err := client.Post(baseURL+"/api/reports", "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST /api/reports: %v", err)
	}
	return resp
}

func TestSubmitThenNearbyReturnsReport(t *testing.T) {
	srv, mailer := newE2EServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	verifySession(t, client, srv, mailer, "submit-then-nearby@example.com")

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	var gotSessionCookie bool
	for _, c := range jar.Cookies(srvURL) {
		if c.Name == "pinalert_session" {
			gotSessionCookie = true
		}
	}
	if !gotSessionCookie {
		t.Fatalf("expected a pinalert_session cookie to have been set while verifying")
	}

	resp := postReport(t, client, srv.URL, map[string]any{
		"category":    "flood",
		"severity":    "critical",
		"description": "Main road under water near the bridge",
		"latitude":    12.9716,
		"longitude":   77.5946,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 from POST, got %d: %s", resp.StatusCode, b)
	}

	nearbyURL := fmt.Sprintf("%s/api/reports?lat=12.9716&lon=77.5946&radius_km=5", srv.URL)
	resp2, err := client.Get(nearbyURL)
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 from GET, got %d: %s", resp2.StatusCode, b)
	}

	var out struct {
		Reports []struct {
			Category    string  `json:"category"`
			Severity    string  `json:"severity"`
			Description string  `json:"description"`
			DistanceKm  float64 `json:"distance_km"`
		} `json:"reports"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&out); err != nil {
		t.Fatalf("decoding nearby response: %v", err)
	}

	if len(out.Reports) != 1 {
		t.Fatalf("expected exactly 1 nearby report, got %d: %+v", len(out.Reports), out.Reports)
	}
	got := out.Reports[0]
	if got.Category != "flood" || got.Severity != "critical" ||
		got.Description != "Main road under water near the bridge" {
		t.Fatalf("nearby report does not match what was submitted: %+v", got)
	}
}

func TestNearbyResponseOmitsSessionID(t *testing.T) {
	srv, mailer := newE2EServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	verifySession(t, client, srv, mailer, "omits-session-id@example.com")

	resp := postReport(t, client, srv.URL, map[string]any{
		"category":    "fire",
		"severity":    "low",
		"description": "Small fire behind the market, contained",
		"latitude":    12.9716,
		"longitude":   77.5946,
	})
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201 from POST, got %d: %s", resp.StatusCode, b)
	}
	resp.Body.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	var sessionCookieValue string
	for _, c := range jar.Cookies(srvURL) {
		if c.Name == "pinalert_session" {
			sessionCookieValue = c.Value
		}
	}
	if sessionCookieValue == "" {
		t.Fatalf("expected a session cookie to have been set after POST")
	}
	sessionID := strings.SplitN(sessionCookieValue, ".", 2)[0]
	if sessionID == "" {
		t.Fatalf("expected a non-empty session id portion of the cookie")
	}

	resp2, err := client.Get(fmt.Sprintf("%s/api/reports?lat=12.9716&lon=77.5946&radius_km=5", srv.URL))
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer resp2.Body.Close()

	raw, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("reading nearby response body: %v", err)
	}
	if strings.Contains(string(raw), sessionID) {
		t.Fatalf("nearby response leaked the session identifier: %s", raw)
	}
}
