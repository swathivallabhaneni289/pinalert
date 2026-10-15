package api

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestRequestLogHoldsNoReaderData pins the promise that Pinalert stores no
// reader location. The host keeps the request log, so a line must never carry
// a search text or a map centre. The real router is driven with the log
// captured: one signed in client sends the place search and the report poll
// with a query string full of secrets, and one anonymous client sends the same
// requests and is turned away (a refused request still logs a line).
func TestRequestLogHoldsNoReaderData(t *testing.T) {
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("request-log-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	queries := sqlcgen.New(pool)
	fm := &gateFakeMailer{}
	var captured bytes.Buffer
	router := NewRouter(Deps{
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
		RequestLog:  log.New(&captured, "", 0),
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	get := func(client *http.Client, target string) int {
		t.Helper()
		resp, err := client.Get(srv.URL + target)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}

	place := "/api/geocode/place?q=SecretStreet&lat=13.0826&lon=80.2707"
	nearby := "/api/reports?lat=13.0826&lon=80.2707&radius_km=5"

	anon := newGateJarClient(t)
	get(anon, place)
	get(anon, nearby)

	verified := newGateJarClient(t)
	gateVerifySession(t, verified, srv, fm, "request-log@example.com")
	get(verified, place)
	get(verified, nearby)

	logged := captured.String()
	for _, path := range []string{"GET /api/geocode/place ", "GET /api/reports "} {
		if got := strings.Count(logged, path); got != 2 {
			t.Errorf("log holds %d lines for %q, want 2\nlog:\n%s", got, path, logged)
		}
	}
	for _, secret := range []string{"SecretStreet", "13.0826", "80.2707", "radius_km", "q=", "lat=", "lon=", "token=", "?"} {
		if strings.Contains(logged, secret) {
			t.Errorf("log contains %q\nlog:\n%s", secret, logged)
		}
	}
}
