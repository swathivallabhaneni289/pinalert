// welcome_e2e_test.go: Phase 3.4 plan 10. The first-time flow at /welcome and
// the onboarding gate switched on, driven through the real router over a real
// Postgres (D-27, orchestrator decision 5). They need the disposable database
// the e2e harness opens (testutil.NewTestDB); never run them on the owner's
// data. Every package level identifier carries the welcome prefix.
package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
)

// welcomeServer builds the e2e router with the onboarding gate on, as
// cmd/server/main.go does.
func welcomeServer(t *testing.T) (srvURL string, newClient func(email string) (*http.Client, *http.Client), pool *pgxpool.Pool) {
	t.Helper()
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.OnboardingGate = true
	})
	return srv.URL, func(email string) (*http.Client, *http.Client) {
		follow := newJarClient(t)
		verifySession(t, follow, srv, mailer, email)
		// The same cookies, but a redirect is returned and not followed, so
		// the test can read the 302 and the 303 themselves.
		still := &http.Client{
			Jar:           follow.Jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		return follow, still
	}, pool
}

// welcomeGet GETs path and returns the response and its body.
func welcomeGet(t *testing.T, client *http.Client, base, path string) (*http.Response, string) {
	t.Helper()
	resp, err := client.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp, meBody(t, resp)
}

// welcomeWantRedirect requires a redirect with the given status to the given
// constant target.
func welcomeWantRedirect(t *testing.T, client *http.Client, base, path string, status int, target string) {
	t.Helper()
	resp, body := welcomeGet(t, client, base, path)
	if resp.StatusCode != status {
		t.Fatalf("GET %s: status %d, want %d, body %.200s", path, resp.StatusCode, status, body)
	}
	if got := resp.Header.Get("Location"); got != target {
		t.Fatalf("GET %s: Location %q, want %q", path, got, target)
	}
}

// welcomeSetState puts the account into the state the test wants by writing
// the two onboarding columns directly, as an account created before migration
// 00008 or under older terms would hold them.
func welcomeSetState(t *testing.T, pool *pgxpool.Pool, email, terms string, done bool) {
	t.Helper()
	var termsArg any
	if terms != "" {
		termsArg = terms
	}
	_, err := pool.Exec(context.Background(),
		`UPDATE accounts
		 SET terms_version = $2,
		     terms_agreed_at = CASE WHEN $2::text IS NULL THEN NULL ELSE now() END,
		     first_run_done_at = CASE WHEN $3 THEN now() ELSE NULL END
		 WHERE email = $1`, email, termsArg, done)
	if err != nil {
		t.Fatalf("setting the onboarding state of %s: %v", email, err)
	}
}

// TestOnboardingFlowEndToEnd walks a fresh verified account through the whole
// flow (T-34-42, T-34-43, T-34-46): every app page redirects to /welcome, the
// page renders, first-run-done before consent is refused, consent moves the
// page to step 3, the profile and done calls finish it, and then the app opens
// and /welcome sends the account to /.
func TestOnboardingFlowEndToEnd(t *testing.T) {
	base, newClient, _ := welcomeServer(t)
	client, still := newClient("welcome-flow@example.com")

	for _, path := range []string{"/", "/profile", "/settings", "/alerts", "/low"} {
		welcomeWantRedirect(t, still, base, path, http.StatusFound, "/welcome")
	}

	resp, body := welcomeGet(t, still, base, "/welcome")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /welcome: status %d, want 200: %.200s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	for _, want := range []string{
		`data-start-step="1"`, "Step 1 of 4", "Step 2 of 4", "Step 3 of 4", "Step 4 of 4",
		"Welcome to Pinalert", "Disclaimer and policy", "Show what is happening near you", "About you",
		"call 112 first.", "Please read this before you use Pinalert. Tap Agree to continue.",
		"Filled in from your location.", "Not now, I will search a place",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/welcome lacks %q", want)
		}
	}
	// The text agreed to is the published text: its headings and a sentence of
	// each part are in the scroll box.
	box := body[strings.Index(body, `id="welcome-scrollbox"`):]
	for _, want := range []string{"Disclaimer", "Policy", `id="policy"`} {
		if !strings.Contains(box, want) {
			t.Errorf("the scroll box lacks %q", want)
		}
	}
	for _, banned := range []string{"Get alerts when it matters", "Demo", "welcome-flow@example.com"} {
		if strings.Contains(body, banned) {
			t.Errorf("/welcome holds %q", banned)
		}
	}
	// The step 1 section shows and the rest start hidden.
	if strings.Contains(body, `id="welcome-step-1" aria-labelledby="welcome-step-1-title" hidden`) {
		t.Errorf("step 1 starts hidden")
	}
	for _, n := range []string{"2", "3", "4"} {
		if !strings.Contains(body, `id="welcome-step-`+n+`" aria-labelledby="welcome-step-`+n+`-title" hidden`) {
			t.Errorf("step %s does not start hidden", n)
		}
	}

	// Done before consent is refused and changes nothing.
	meWant(t, "done before consent", meSend(t, client, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusConflict)
	welcomeWantRedirect(t, still, base, "/", http.StatusFound, "/welcome")

	// Agree: the page now starts at step 3, and the app is still closed.
	meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	resp, body = welcomeGet(t, still, base, "/welcome")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `data-start-step="3"`) {
		t.Fatalf("after Agree /welcome should start at step 3: status %d", resp.StatusCode)
	}
	if !strings.Contains(body, `id="welcome-step-1" aria-labelledby="welcome-step-1-title" hidden`) ||
		strings.Contains(body, `id="welcome-step-3" aria-labelledby="welcome-step-3-title" hidden`) {
		t.Errorf("after Agree step 3 must show and step 1 must be hidden")
	}
	welcomeWantRedirect(t, still, base, "/", http.StatusFound, "/welcome")

	// Profile then done.
	meWant(t, "profile", meSend(t, client, http.MethodPatch, base+"/api/me/profile", `{"display_name":"Asha"}`), http.StatusOK)
	meWant(t, "done", meSend(t, client, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusNoContent)

	resp, body = welcomeGet(t, still, base, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / after the flow: status %d, want 200: %.200s", resp.StatusCode, body)
	}
	for _, path := range []string{"/profile", "/settings", "/alerts"} {
		if resp, _ := welcomeGet(t, still, base, path); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s after the flow: status %d, want 200", path, resp.StatusCode)
		}
	}
	welcomeWantRedirect(t, still, base, "/welcome", http.StatusSeeOther, "/")

	// A client that follows every redirect reaches the app with no loop.
	resp, _ = welcomeGet(t, client, base, "/welcome")
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/" {
		t.Errorf("following /welcome ended at %s with %d, want the app at /", resp.Request.URL.Path, resp.StatusCode)
	}

	// The Low data view opens too. It goes last: visiting it sets the remembered
	// Low data choice, after which a bare / goes there (D-17).
	if resp, _ := welcomeGet(t, still, base, "/low"); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /low after the flow: status %d, want 200", resp.StatusCode)
	}
}

// TestWelcomeStartsAtStep3AfterAgree pins that a person who left after Agree
// does not read and agree twice, and that an older agreement does not count.
func TestWelcomeStartsAtStep3AfterAgree(t *testing.T) {
	base, newClient, pool := welcomeServer(t)
	const email = "welcome-step3@example.com"
	client, still := newClient(email)

	_, body := welcomeGet(t, still, base, "/welcome")
	if !strings.Contains(body, `data-start-step="1"`) {
		t.Fatalf("a fresh account starts at step 1")
	}
	meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	_, body = welcomeGet(t, still, base, "/welcome")
	if !strings.Contains(body, `data-start-step="3"`) {
		t.Errorf("after Agree the page starts at step 3")
	}

	// Agreement is to a text: an older version starts again at step 1.
	welcomeSetState(t, pool, email, "2020-01-01", false)
	_, body = welcomeGet(t, still, base, "/welcome")
	if !strings.Contains(body, `data-start-step="1"`) {
		t.Errorf("an older agreement must start at step 1")
	}
}

// TestWelcomeRedirectsOnboardedAccount pins that an account through the flow
// gets a 303 to the constant "/" and never the flow again.
func TestWelcomeRedirectsOnboardedAccount(t *testing.T) {
	base, newClient, _ := welcomeServer(t)
	client, still := newClient("welcome-done@example.com")

	meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	meWant(t, "done", meSend(t, client, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusNoContent)

	resp, _ := welcomeGet(t, still, base, "/welcome")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Errorf("GET /welcome for an onboarded account: %d to %q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
	}
	// A query string on the request never reaches the target.
	resp, _ = welcomeGet(t, still, base, "/welcome?next=https://evil.example/")
	if resp.Header.Get("Location") != "/" {
		t.Errorf("Location = %q, the target is a constant", resp.Header.Get("Location"))
	}
	// Without a session the page is not served at all.
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if resp, _ := welcomeGet(t, anon, base, "/welcome"); resp.StatusCode == http.StatusOK {
		t.Errorf("GET /welcome without a session answered 200")
	}
}

// TestExistingAccountSeesFlowOnce pins orchestrator decision 5: an account that
// existed before the flow (no consent, no done flag) or agreed to older terms is
// redirected, sees the flow, and is never redirected again after finishing. No
// consent is backfilled.
func TestExistingAccountSeesFlowOnce(t *testing.T) {
	base, newClient, pool := welcomeServer(t)

	cases := []struct {
		name  string
		email string
		terms string
		done  bool
	}{
		{"an account made before the flow", "welcome-old-1@example.com", "", false},
		{"older terms with the flow marked done", "welcome-old-2@example.com", "2020-01-01", true},
		{"current terms but the flow not finished", "welcome-old-3@example.com", "current", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, still := newClient(c.email)
			terms := c.terms
			if terms == "current" {
				meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
			} else {
				welcomeSetState(t, pool, c.email, terms, c.done)
			}

			welcomeWantRedirect(t, still, base, "/", http.StatusFound, "/welcome")
			if resp, _ := welcomeGet(t, still, base, "/welcome"); resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /welcome: %d, want 200", resp.StatusCode)
			}

			meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
			meWant(t, "done", meSend(t, client, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusNoContent)

			for i := 0; i < 2; i++ {
				if resp, _ := welcomeGet(t, still, base, "/"); resp.StatusCode != http.StatusOK {
					t.Fatalf("GET / after finishing (visit %d): %d, want 200", i+1, resp.StatusCode)
				}
			}
			welcomeWantRedirect(t, still, base, "/welcome", http.StatusSeeOther, "/")
		})
	}
}

// TestOnboardingGateNeverLocksAnyoneOut proves rule 6 for a signed in account
// that has not been through the flow: every /api/ path, the legal pages, static
// files, the login and verify pages, logout and /welcome itself stay reachable
// and none is redirected to /welcome (T-34-42).
func TestOnboardingGateNeverLocksAnyoneOut(t *testing.T) {
	base, newClient, _ := welcomeServer(t)
	client, still := newClient("welcome-exempt@example.com")
	_ = client

	notRedirected := func(label string, resp *http.Response) {
		t.Helper()
		if resp.StatusCode >= 300 && resp.StatusCode < 400 && resp.Header.Get("Location") == "/welcome" {
			t.Errorf("%s was redirected to /welcome", label)
		}
	}

	// Every /api/ path, reads and writes: a refusal of its own is fine, a
	// redirect to /welcome is not. A queued offline report must still arrive.
	apiCalls := []struct{ method, path, body string }{
		{http.MethodGet, "/api/reports?lat=12.97&lon=77.59&radius_km=5", ""},
		{http.MethodPost, "/api/reports", `{}`},
		{http.MethodPost, "/api/reports/1/confirm", `{"lat":12.97,"lon":77.59}`},
		{http.MethodPost, "/api/reports/1/flag", `{}`},
		{http.MethodGet, "/api/geocode?q=Bengaluru", ""},
		{http.MethodGet, "/api/geocode/place?q=Bengaluru", ""},
		{http.MethodPatch, "/api/me/profile", `{"display_name":"Asha"}`},
	}
	for _, c := range apiCalls {
		resp := meSend(t, still, c.method, base+c.path, c.body)
		meBody(t, resp)
		notRedirected(c.method+" "+c.path, resp)
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			t.Errorf("%s %s answered a redirect (%d); the API never redirects", c.method, c.path, resp.StatusCode)
		}
	}

	// Public and exempt pages.
	for _, path := range []string{"/disclaimer", "/policy", "/static/css/welcome.css", "/static/js/welcome.js", "/static/js/position.js", "/login", "/welcome"} {
		resp, _ := welcomeGet(t, still, base, path)
		notRedirected("GET "+path, resp)
		if path != "/login" && resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200", path, resp.StatusCode)
		}
	}
	// The verify page with a bad token still answers its own page.
	resp, _ := welcomeGet(t, still, base, "/auth/verify?token=not-a-real-token")
	notRedirected("GET /auth/verify", resp)

	// Logout is a POST and passes.
	resp = meSend(t, still, http.MethodPost, base+"/auth/logout", "")
	meBody(t, resp)
	notRedirected("POST /auth/logout", resp)
	if resp.Header.Get("Location") == "/welcome" {
		t.Errorf("logout redirected to /welcome")
	}

	// No loop: following every redirect from the front page ends at /welcome
	// with a 200 (the Go client stops with an error after 10 redirects), and a
	// second visit to /welcome is the same page.
	client2, _ := newClient("welcome-loop@example.com")
	resp, _ = welcomeGet(t, client2, base, "/")
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/welcome" {
		t.Errorf("following / ended at %s with %d, want 200 at /welcome", resp.Request.URL.Path, resp.StatusCode)
	}
	resp, _ = welcomeGet(t, client2, base, "/welcome")
	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path != "/welcome" {
		t.Errorf("GET /welcome ended at %s with %d, want 200", resp.Request.URL.Path, resp.StatusCode)
	}
}

// TestWelcomeEscapesStoredValues pins T-34-45: the stored name and home area
// are written escaped into the field values, and the page holds no email.
func TestWelcomeEscapesStoredValues(t *testing.T) {
	base, newClient, _ := welcomeServer(t)
	client, still := newClient("welcome-escape@example.com")

	meWant(t, "profile", meSend(t, client, http.MethodPatch, base+"/api/me/profile",
		`{"display_name":"A<script>alert(1)</script>","home_area":{"name":"Park <i>X</i>","lat":12.9,"lon":77.6}}`), http.StatusOK)

	resp, body := welcomeGet(t, still, base, "/welcome")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /welcome: %d", resp.StatusCode)
	}
	for _, banned := range []string{"<script>alert(1)</script>", "<i>X</i>", "welcome-escape@example.com"} {
		if strings.Contains(body, banned) {
			t.Errorf("/welcome holds %q unescaped or at all", banned)
		}
	}
	for _, want := range []string{`value="A&lt;script&gt;alert(1)&lt;/script&gt;"`, `value="Park &lt;i&gt;X&lt;/i&gt;"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/welcome lacks the escaped value %q", want)
		}
	}
	// A stored home area is a value in the field and the prefilled line stays
	// hidden until script shows it for a step 3 search.
	if !strings.Contains(body, `id="welcome-home-prefilled" hidden`) {
		t.Errorf("the prefilled line must start hidden")
	}
}
