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
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

func newE2EServer(t *testing.T) *httptest.Server {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("e2e-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	queries := sqlcgen.New(pool)
	deps := api.Deps{
		Session:  mgr,
		Sessions: queries,
		Reports:  service.NewReportService(queries),
	}
	router := api.NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
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
	srv := newE2EServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

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

	var gotSetCookie bool
	for _, c := range resp.Cookies() {
		if c.Name == "pinalert_session" {
			gotSetCookie = true
		}
	}
	if !gotSetCookie {
		t.Fatalf("expected a Set-Cookie for pinalert_session on the first request")
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
	srv := newE2EServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

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
