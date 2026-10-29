package api

// onboarding_test.go proves the onboarding redirect of plan 03.4-01 (D-27,
// orchestrator decision 5): it sends a not onboarded account's page GET to
// /welcome, lets /welcome, every /api/ path and every non GET request through,
// and does nothing at all while the flag is off.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// onboardingNext is the handler behind the middleware. It answers 200 and a
// body that proves it ran.
var onboardingNext = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("passed"))
})

// onboardingDo runs one request through requireOnboarded with an account.
func onboardingDo(enabled bool, acc *account.Account, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if acc != nil {
		req = req.WithContext(account.WithAccount(req.Context(), *acc))
	}
	rec := httptest.NewRecorder()
	requireOnboarded(enabled)(onboardingNext).ServeHTTP(rec, req)
	return rec
}

func TestOnboardingRedirect(t *testing.T) {
	notOnboarded := map[string]account.Account{
		"no terms version":    {ID: 1, Email: "a@example.com"},
		"older terms version": {ID: 1, Email: "a@example.com", TermsVersion: "2020-01-01", FirstRunDone: true},
		"terms but no flow":   {ID: 1, Email: "a@example.com", TermsVersion: service.CurrentTermsVersion},
		"flow but no terms":   {ID: 1, Email: "a@example.com", FirstRunDone: true},
		// Added 2026-10-29 by Phase 7.1 plan 10: step 4 is required, so a finished
		// flow on the current terms with any one of the three fields missing is
		// not onboarded either.
		"no username": {ID: 1, Email: "a@example.com", TermsVersion: service.CurrentTermsVersion, FirstRunDone: true, HasName: true, HasHomeArea: true},
		"no name":     {ID: 1, Email: "a@example.com", TermsVersion: service.CurrentTermsVersion, FirstRunDone: true, HasUsername: true, HasHomeArea: true},
		"no home":     {ID: 1, Email: "a@example.com", TermsVersion: service.CurrentTermsVersion, FirstRunDone: true, HasUsername: true, HasName: true},
	}
	for label, acc := range notOnboarded {
		for _, path := range []string{"/", "/low", "/profile", "/settings", "/alerts"} {
			t.Run(label+" "+path, func(t *testing.T) {
				rec := onboardingDo(true, &acc, http.MethodGet, path)
				if rec.Code != http.StatusFound {
					t.Fatalf("status = %d, want 302", rec.Code)
				}
				if got := rec.Header().Get("Location"); got != "/welcome" {
					t.Fatalf("Location = %q, want exactly /welcome", got)
				}
				if got := rec.Header().Get("Cache-Control"); got != "no-store" {
					t.Fatalf("Cache-Control = %q, want no-store", got)
				}
				if rec.Body.String() == "passed" {
					t.Fatal("the next handler ran")
				}
			})
		}
	}

	// Amended 2026-10-29 by Phase 7.1 plan 10: an onboarded account also has the
	// three profile facts. Every other assertion is unchanged.
	done := account.Account{
		ID: 1, Email: "a@example.com", TermsVersion: service.CurrentTermsVersion, FirstRunDone: true,
		HasUsername: true, HasName: true, HasHomeArea: true,
	}
	for _, path := range []string{"/", "/low", "/profile", "/settings", "/alerts"} {
		rec := onboardingDo(true, &done, http.MethodGet, path)
		if rec.Code != http.StatusOK || rec.Body.String() != "passed" {
			t.Fatalf("an onboarded account on %s: status %d body %q, want it to pass", path, rec.Code, rec.Body.String())
		}
	}
}

func TestOnboardingExemptions(t *testing.T) {
	acc := account.Account{ID: 1, Email: "a@example.com"}

	passes := []struct {
		method, path string
	}{
		{http.MethodGet, "/welcome"},
		{http.MethodGet, "/api/reports"},
		{http.MethodPost, "/api/reports"},
		{http.MethodPost, "/api/me/consent"},
		{http.MethodPatch, "/api/me/profile"},
		{http.MethodPost, "/api/reports/5/confirm"},
		{http.MethodPost, "/auth/logout"},
		{http.MethodPost, "/profile"},
	}
	for _, c := range passes {
		rec := onboardingDo(true, &acc, c.method, c.path)
		if rec.Code != http.StatusOK || rec.Body.String() != "passed" {
			t.Errorf("%s %s: status %d body %q, want it to pass through", c.method, c.path, rec.Code, rec.Body.String())
		}
	}

	// HEAD is treated like GET.
	rec := onboardingDo(true, &acc, http.MethodHead, "/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/welcome" {
		t.Errorf("HEAD /: status %d Location %q, want 302 to /welcome", rec.Code, rec.Header().Get("Location"))
	}

	// A request that carries no account is a wiring mistake: it passes through,
	// because requireVerifiedAccount already refused every unverified caller.
	rec = onboardingDo(true, nil, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Errorf("no account in the context: status %d, want it to pass through", rec.Code)
	}

	// No part of the request builds the redirect target.
	rec = onboardingDo(true, &acc, http.MethodGet, "/?next=https://evil.example")
	if rec.Header().Get("Location") != "/welcome" {
		t.Errorf("Location = %q, want exactly /welcome", rec.Header().Get("Location"))
	}
}

func TestOnboardingOffByDefault(t *testing.T) {
	acc := account.Account{ID: 1, Email: "a@example.com"}
	for _, path := range []string{"/", "/low", "/profile", "/settings", "/alerts", "/welcome"} {
		rec := onboardingDo(false, &acc, http.MethodGet, path)
		if rec.Code != http.StatusOK || rec.Body.String() != "passed" {
			t.Errorf("flag off, GET %s: status %d, want it to pass through", path, rec.Code)
		}
	}
}

// TestOnboardingRouterOffByDefault builds the real router from a zero Deps
// flag: a verified account is never sent to /welcome. It needs a database and
// skips without DATABASE_URL.
func TestOnboardingRouterOffByDefault(t *testing.T) {
	srv, _, mailer := newGateTestServer(t)
	client := newGateJarClient(t)
	gateVerifySession(t, client, srv, mailer, "off-by-default@example.com")
	for _, path := range []string{"/", "/profile", "/low"} {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if loc := resp.Header.Get("Location"); loc == "/welcome" {
			t.Errorf("GET %s with the flag off redirected to /welcome", path)
		}
	}
}

// TestOnboardingRouterWiring turns the flag on in the real router: a verified
// account with no consent is sent to /welcome from a page, while its /api/
// calls are untouched, and finishing the flow lets the page through.
func TestOnboardingRouterWiring(t *testing.T) {
	srv, pool, mailer := newGateTestServerWith(t, func(d *Deps, _ *sqlcgen.Queries) {
		d.OnboardingGate = true
	})
	client := newGateJarClient(t)
	gateVerifySession(t, client, srv, mailer, "wiring@example.com")

	resp, err := client.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/welcome" {
		t.Fatalf("GET /profile before the flow: status %d Location %q, want 302 to /welcome", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp, err = client.Get(srv.URL + "/api/reports?lat=12.97&lon=77.59&radius_km=5")
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports before the flow: status %d, want 200", resp.StatusCode)
	}

	if _, err := pool.Exec(context.Background(),
		`UPDATE accounts SET terms_version = $1, terms_agreed_at = now(), first_run_done_at = now() WHERE email = $2`,
		service.CurrentTermsVersion, "wiring@example.com"); err != nil {
		t.Fatalf("marking the account onboarded: %v", err)
	}
	// Amended 2026-10-29 by Phase 7.1 plan 10: the terms and the flow flag alone
	// no longer open the pages; step 4 (username, name, home area) is required.
	// The profile is filled through the real route, the way step 4 does it.
	gateFillProfile(t, client, srv, "wiring.user")
	resp, err = client.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile after the flow: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /profile after the flow: status %d, want 200", resp.StatusCode)
	}
}

// gateFillProfile does what step 4 About you does for an account in the gate
// harness: it saves a username, a name and a home area through PATCH
// /api/me/profile and fails on anything but 200 (Phase 7.1 plan 10). username
// must be a free, usable lower case name.
func gateFillProfile(t *testing.T, client *http.Client, srv *httptest.Server, username string) {
	t.Helper()
	if name, class := service.ClassifyUsername(username); class != service.UsernameUsable || name != username {
		t.Fatalf("gateFillProfile: %q is not a usable lower case username (normalised %q, class %d)", username, name, class)
	}
	body := `{"username":"` + username + `","display_name":"Test Person",` +
		`"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/me/profile", strings.NewReader(body))
	if err != nil {
		t.Fatalf("building the profile request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PATCH /api/me/profile: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("PATCH /api/me/profile for %s: status %d, body %s", username, resp.StatusCode, raw)
	}
}

// TestOnboardingRoutesIncompleteToWelcome pins the one predicate in the real
// router (Phase 7.1, D-06, D-08): an account on the current terms that finished
// the flow before, but has no username, is sent from a page to /welcome, while
// its /api/ calls are untouched, and after step 4 is filled the pages open.
func TestOnboardingRoutesIncompleteToWelcome(t *testing.T) {
	srv, pool, mailer := newGateTestServerWith(t, func(d *Deps, _ *sqlcgen.Queries) {
		d.OnboardingGate = true
	})
	client := newGateJarClient(t)
	gateVerifySession(t, client, srv, mailer, "incomplete@example.com")
	if _, err := pool.Exec(context.Background(),
		`UPDATE accounts SET terms_version = $1, terms_agreed_at = now(), first_run_done_at = now() WHERE email = $2`,
		service.CurrentTermsVersion, "incomplete@example.com"); err != nil {
		t.Fatalf("marking the flow finished: %v", err)
	}

	still := &http.Client{Jar: client.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(c *http.Client, path string) *http.Response {
		t.Helper()
		resp, err := c.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		return resp
	}

	resp := get(still, "/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/welcome" {
		t.Fatalf("GET / without a username: status %d Location %q, want 302 to /welcome", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp := get(still, "/api/reports?lat=12.97&lon=77.59&radius_km=5"); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports without a username: status %d, want 200 (the API never meets the gate)", resp.StatusCode)
	}

	gateFillProfile(t, client, srv, "incomplete.user")
	if resp := get(still, "/"); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / after step 4: status %d, want 200", resp.StatusCode)
	}
}
