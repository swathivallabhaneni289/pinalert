// welcome_e2e_test.go: Phase 3.4 plan 10. The first-time flow at /welcome and
// the onboarding gate switched on, driven through the real router over a real
// Postgres (D-27, orchestrator decision 5). They need the disposable database
// the e2e harness opens (testutil.NewTestDB); never run them on the owner's
// data. Every package level identifier carries the welcome prefix.
package handlers_test

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// welcomeServer builds the e2e router with the onboarding gate on, as
// cmd/server/main.go does.
func welcomeServer(t *testing.T) (srvURL string, newClient func(email string) (*http.Client, *http.Client), pool *pgxpool.Pool) {
	t.Helper()
	return welcomeServerWithKey(t, "")
}

// welcomeServerWithKey is welcomeServer with the page config's public push key
// set (Phase 3.3 plan 13), so a test can read it back as data-push-key. The key
// is a plain placeholder word, never key material.
func welcomeServerWithKey(t *testing.T, pushKey string) (srvURL string, newClient func(email string) (*http.Client, *http.Client), pool *pgxpool.Pool) {
	t.Helper()
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.OnboardingGate = true
		d.Page.PushPublicKey = pushKey
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
		// Amended 2026-10-27 by quick 261027-t5w (owner): the alerts ask is a
		// pop-up after step 4, so the flow has four steps and every label reads
		// "of 4".
		`data-start-step="1"`, "Step 1 of 4", "Step 2 of 4", "Step 3 of 4", "Step 4 of 4",
		"Welcome to Pinalert", "Disclaimer and policy", "Show what is happening near you", "About you",
		"call 112 first.", "Please read this to the end before you use Pinalert.",
		// Plan 33: step 3 holds only Allow location and Not now, and the Disagree
		// box is drawn with the existing log out form.
		`id="welcome-allow-location">Allow location</button>`, `id="welcome-not-now">Not now</button>`,
		// Owner amendments the same day: the box is one short line and two
		// buttons, with no call line and no number.
		"Sorry, you need to agree before you can continue.",
		`id="welcome-reread">Read again</button>`,
		`<form class="welcome__notice-form" method="post" action="/auth/logout">`,
		`id="welcome-logout">Log out</button>`,
		// Plan 30: free movement between the steps. Back on steps 2 to 4, and
		// each name in the step list is a button; the current one is marked.
		`id="welcome-back-2"`, `id="welcome-back-3"`, `id="welcome-back-4"`,
		`<button type="button" class="welcome__steps-button" data-step="1" aria-current="step">`,
		`<button type="button" class="welcome__steps-button" data-step="4">`,
		// Quick 261027-jhd (owner, 2026-10-27): the Agree and Disagree row has an
		// id and the end-of-text marker closes the text box, so welcome.js can hold
		// the row back until the end has been seen.
		`<div class="welcome__actions welcome__actions--pair" id="welcome-consent-actions">`,
		`<div class="welcome__scrollbox-end" id="welcome-scrollbox-end" aria-hidden="true"></div>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/welcome lacks %q", want)
		}
	}
	// The hold is welcome.js's, never the server's: a page whose script fails
	// must still show Agree.
	if strings.Contains(body, "data-unread") {
		t.Errorf("/welcome holds data-unread: the script sets the hold, the server never does")
	}
	// The text agreed to is the published text: its headings and a sentence of
	// each part are in the scroll box.
	box := body[strings.Index(body, `id="welcome-scrollbox"`):]
	for _, want := range []string{"Disclaimer", "Policy", `id="policy"`} {
		if !strings.Contains(box, want) {
			t.Errorf("the scroll box lacks %q", want)
		}
	}
	// Amended 2026-10-27 by quick 261027-t5w: there is no step 5 any more, so
	// none of its parts may render.
	//
	// Amended 2026-10-29 by plan 05-18 (D-03): Phase 5 adds Try the demo to
	// step 1 and the Disagree box, so "Demo" left this list; the rendered
	// links are pinned in TestWelcomeOffersTheDemoForANewAccount. Every other
	// case stays.
	for _, banned := range []string{"welcome-flow@example.com",
		`id="welcome-step-5`, `id="welcome-back-5"`, `data-step="5"`, "Step 5 of", "Get alerts when it matters", "Allow alerts"} {
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
	//
	// Amended 2026-10-29 by Phase 7.1 plan 10 (D-06): step 4 is required, so the
	// profile is the username, the name and the home area (a name alone is no
	// longer enough to finish). Every other assertion is unchanged.
	fillProfileAt(t, client, base, "flow.user")
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

// TestWelcomeOffersTheDemoForANewAccount pins plan 05-18 (D-03): the rendered
// first-time flow for a new account offers the Demo beside Continue on step 1
// and in the Disagree box, both as links to /demo?from=welcome, and the Disagree
// lead holds the restored second sentence. The links are plain links, so nothing
// here depends on a script.
func TestWelcomeOffersTheDemoForANewAccount(t *testing.T) {
	base, newClient, _ := welcomeServer(t)
	_, still := newClient("welcome-demo@example.com")

	resp, body := welcomeGet(t, still, base, "/welcome")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /welcome: %d", resp.StatusCode)
	}
	for _, want := range []string{
		`<a class="btn" id="welcome-try-demo-1" href="/demo?from=welcome">Try the demo</a>`,
		`<a class="btn" id="welcome-try-demo-2" href="/demo?from=welcome">Try the demo</a>`,
		`Sorry, you need to agree before you can continue. You can still look around in the Demo without agreeing.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/welcome lacks %q", want)
		}
	}
	// Step 1 holds Try the demo before Continue, and the flow is still four steps.
	one := strings.Index(body, `id="welcome-try-demo-1"`)
	two := strings.Index(body, `id="welcome-continue-1"`)
	if one == -1 || two == -1 || one > two {
		t.Errorf("step 1 should hold Try the demo (%d) before Continue (%d)", one, two)
	}
	if n := strings.Count(body, `<section class="welcome__step"`); n != 4 {
		t.Errorf("/welcome holds %d steps, want 4", n)
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

// welcomeAttr reads one data attribute of the flow root, or "" when absent.
func welcomeAttr(t *testing.T, body, name string) string {
	t.Helper()
	tag := p25OpeningTag(t, body, `<div class="welcome" id="welcome"`)
	m := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

// TestWelcomeStartsAtFirstMissingStep pins the start step of the one time fill
// in (Phase 7.1, D-08, SC5): the first missing step among steps 1 to 4. A
// finished account on the current terms that lacks a username goes to step 4;
// one on older terms goes to step 2 (and, after Agree, straight on to step 4);
// a complete account is sent to /; an account mid-flow starts where it did.
func TestWelcomeStartsAtFirstMissingStep(t *testing.T) {
	base, newClient, pool := welcomeServer(t)

	step := func(label string, still *http.Client) string {
		t.Helper()
		resp, body := welcomeGet(t, still, base, "/welcome")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: GET /welcome: %d, want 200", label, resp.StatusCode)
		}
		return welcomeAttr(t, body, "data-start-step")
	}

	// Current terms, the flow finished before, nothing of step 4 saved.
	client, still := newClient("welcome-missing-4@example.com")
	meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	welcomeSetState(t, pool, "welcome-missing-4@example.com", service.CurrentTermsVersion, true)
	if got := step("terms current, finished, no profile", still); got != "4" {
		t.Errorf("terms current, finished, no profile: data-start-step = %q, want 4", got)
	}
	// Still step 4 with only the name saved, and the stored name is on the page.
	meWant(t, "name", meSend(t, client, http.MethodPatch, base+"/api/me/profile", `{"display_name":"Asha"}`), http.StatusOK)
	_, body := welcomeGet(t, still, base, "/welcome")
	if welcomeAttr(t, body, "data-start-step") != "4" || !strings.Contains(body, `value="Asha"`) {
		t.Errorf("a finished account with only a name: want step 4 with the stored name on the page")
	}

	// Older terms, the flow finished before: step 2, then Agree, then step 4.
	client2, still2 := newClient("welcome-missing-2@example.com")
	welcomeSetState(t, pool, "welcome-missing-2@example.com", "2020-01-01", true)
	if got := step("terms older, finished", still2); got != "2" {
		t.Errorf("terms older, finished: data-start-step = %q, want 2", got)
	}
	meWant(t, "consent", meSend(t, client2, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	if got := step("after Agree", still2); got != "4" {
		t.Errorf("after Agree for a finished account: data-start-step = %q, want 4", got)
	}

	// A new account starts at 1, and at 3 after Agree (unchanged).
	client3, still3 := newClient("welcome-missing-1@example.com")
	if got := step("new account", still3); got != "1" {
		t.Errorf("new account: data-start-step = %q, want 1", got)
	}
	meWant(t, "consent", meSend(t, client3, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	if got := step("new account after Agree", still3); got != "3" {
		t.Errorf("new account after Agree: data-start-step = %q, want 3", got)
	}

	// A complete account is sent to /.
	client4, still4 := newClient("welcome-complete@example.com")
	meWant(t, "consent", meSend(t, client4, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	fillProfileAt(t, client4, base, "complete.user")
	meWant(t, "done", meSend(t, client4, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusNoContent)
	welcomeWantRedirect(t, still4, base, "/welcome", http.StatusSeeOther, "/")
}

// TestWelcomeRedirectsOnboardedAccount pins that an account through the flow
// gets a 303 to the constant "/" and never the flow again.
func TestWelcomeRedirectsOnboardedAccount(t *testing.T) {
	base, newClient, _ := welcomeServer(t)
	client, still := newClient("welcome-done@example.com")

	meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	// Amended 2026-10-29 by Phase 7.1 plan 10: done needs the full profile.
	fillProfileAt(t, client, base, "done.account")
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
	for i, c := range cases {
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
			// Amended 2026-10-29 by Phase 7.1 plan 10: done needs the full profile.
			fillProfileAt(t, client, base, "existing.account"+strconv.Itoa(i))
			meWant(t, "done", meSend(t, client, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusNoContent)

			for visit := 0; visit < 2; visit++ {
				if resp, _ := welcomeGet(t, still, base, "/"); resp.StatusCode != http.StatusOK {
					t.Fatalf("GET / after finishing (visit %d): %d, want 200", visit+1, resp.StatusCode)
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
	// Plan 33: a stored home area is a value in the field, and no line says it
	// was filled in from the person's location any more.
	for _, banned := range []string{"welcome-home-prefilled", "Filled in from your location."} {
		if strings.Contains(body, banned) {
			t.Errorf("/welcome still holds %q: step 3 no longer fills the home area", banned)
		}
	}
}

// TestWelcomeAlertsAskRendersForNewAccount pins quick 261027-t5w (owner,
// 2026-10-27, which replaced Phase 3.3 plan 13's step 5) as amended by plan
// 03.3-19 (03.3-UAT gap for test 4, owner 2026-10-27): the alerts pop-up on
// /welcome names the home area in its title, holds the home area search when no
// area is stored, offers the unticked Reliable reporter tick box, and carries
// the configured public key on the flow root; there is no fifth step. A page
// with no key renders an empty attribute, which welcome.js reads as no pop-up.
// One subtest per case.
func TestWelcomeAlertsAskRendersForNewAccount(t *testing.T) {
	const key = "test-public-key-placeholder"
	base, newClient, _ := welcomeServerWithKey(t, key)

	const searchOpen = `<div class="welcome-ask__home" id="welcome-alerts-home"`
	common := func(t *testing.T, body string) {
		t.Helper()
		for _, want := range []string{
			`<dialog id="welcome-alerts-ask" class="logout-confirm" aria-labelledby="welcome-alerts-ask-title"`,
			`<button type="button" class="btn" id="welcome-alerts-later">Not now</button>`,
			`<button type="button" class="btn btn--primary" id="welcome-alerts-allow">Allow</button>`,
			`id="welcome-alerts-reliable"`,
			`A Reliable reporter posts near me`,
			`/static/css/auth.css`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("/welcome lacks %q", want)
			}
		}
		if n := strings.Count(body, `id="welcome-alerts-ask"`); n != 1 {
			t.Errorf("/welcome holds id=welcome-alerts-ask %d times, want 1", n)
		}
		if strings.Contains(body, "near you?") {
			t.Errorf("/welcome still holds a title ending in near you?")
		}
		box := regexp.MustCompile(`<input[^>]*id="welcome-alerts-reliable"[^>]*>`).FindString(body)
		if box == "" || strings.Contains(box, "checked") {
			t.Errorf("the Reliable tick box %q must exist and start unticked", box)
		}
		tag := p25OpeningTag(t, body, `<div class="welcome" id="welcome"`)
		if !strings.Contains(tag, `data-push-key="`+key+`"`) {
			t.Errorf("the flow root %q does not carry data-push-key=%q", tag, key)
		}
		// Amended 2026-10-29 by Phase 7.1 plan 13b (D-17): "device-guide" and
		// "Home Screen" are no longer banned, because the install pop-up for an
		// iPhone or iPad Safari tab holds the install guide. The other banned strings
		// stay, and the install pop-up is asserted below: it is rendered once, closed
		// (no open attribute), with one Not now button and no Allow of its own.
		for _, banned := range []string{`id="welcome-step-5"`, "Step 5 of", "welcome__example"} {
			if strings.Contains(body, banned) {
				t.Errorf("/welcome holds %q: the alerts ask is a pop-up, not a fifth step", banned)
			}
		}
		if n := strings.Count(body, `id="welcome-install-ask"`); n != 1 {
			t.Errorf("/welcome holds id=welcome-install-ask %d times, want 1", n)
		}
		install := p25OpeningTag(t, body, `<dialog id="welcome-install-ask"`)
		if strings.Contains(install, " open") {
			t.Errorf("the install pop-up %q must start closed", install)
		}
		for _, want := range []string{
			`<button type="button" class="btn" id="welcome-install-later">Not now</button>`,
			`<p class="device-guide__lead">On this device, alerts work once Pinalert is on your Home Screen.</p>`,
			`Open Pinalert from your Home Screen and sign in. Tap <strong>Allow</strong> when it asks about alerts.`,
			`/static/css/device-guide.css`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("/welcome lacks %q", want)
			}
		}
	}

	t.Run("a new account is asked for the home area first", func(t *testing.T) {
		_, still := newClient("welcome-ask@example.com")
		resp, body := welcomeGet(t, still, base, "/welcome")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /welcome: %d", resp.StatusCode)
		}
		common(t, body)
		if !strings.Contains(body, `id="welcome-alerts-ask-title" tabindex="-1">Get alerts for critical reports near your home area?</h2>`) {
			t.Errorf("/welcome lacks the your home area title")
		}
		if !strings.Contains(body, `id="welcome-install-ask-title" tabindex="-1">Get alerts for critical reports near your home area</h2>`) {
			t.Errorf("/welcome lacks the install pop-up title for an account with no home area")
		}
		if tag := p25OpeningTag(t, body, searchOpen); strings.Contains(tag, "hidden") {
			t.Errorf("the search block %q is hidden for an account with no home area", tag)
		}
		dialog := p25OpeningTag(t, body, `<dialog id="welcome-alerts-ask"`)
		if !strings.Contains(dialog, `data-home-area=""`) {
			t.Errorf("the dialog %q should carry an empty data-home-area", dialog)
		}
	})

	t.Run("a stored home area names the title and hides the search", func(t *testing.T) {
		client, still := newClient("welcome-ask-home@example.com")
		meWant(t, "profile", meSend(t, client, http.MethodPatch, base+"/api/me/profile",
			`{"home_area":{"name":"Benz Circle","lat":12.9,"lon":77.6}}`), http.StatusOK)
		resp, body := welcomeGet(t, still, base, "/welcome")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /welcome: %d", resp.StatusCode)
		}
		common(t, body)
		if !strings.Contains(body, `id="welcome-alerts-ask-title" tabindex="-1">Get alerts for critical reports near Benz Circle?</h2>`) {
			t.Errorf("/welcome lacks the Benz Circle title")
		}
		if !strings.Contains(body, `id="welcome-install-ask-title" tabindex="-1">Get alerts for critical reports near Benz Circle</h2>`) {
			t.Errorf("/welcome lacks the install pop-up title naming Benz Circle")
		}
		if tag := p25OpeningTag(t, body, searchOpen); !strings.Contains(tag, "hidden") {
			t.Errorf("the search block %q should be hidden when a home area is stored", tag)
		}
		dialog := p25OpeningTag(t, body, `<dialog id="welcome-alerts-ask"`)
		if !strings.Contains(dialog, `data-home-area="Benz Circle"`) {
			t.Errorf("the dialog %q should carry data-home-area=Benz Circle", dialog)
		}
	})

	t.Run("a stored name holding markup is escaped in the title and the attribute", func(t *testing.T) {
		client, still := newClient("welcome-ask-markup@example.com")
		meWant(t, "profile", meSend(t, client, http.MethodPatch, base+"/api/me/profile",
			`{"home_area":{"name":"Park <i>X</i>","lat":12.9,"lon":77.6}}`), http.StatusOK)
		resp, body := welcomeGet(t, still, base, "/welcome")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /welcome: %d", resp.StatusCode)
		}
		if strings.Contains(body, "<i>X</i>") {
			t.Errorf("/welcome holds the stored markup unescaped")
		}
		if !strings.Contains(body, `tabindex="-1">Get alerts for critical reports near Park &lt;i&gt;X&lt;/i&gt;?</h2>`) {
			t.Errorf("/welcome lacks the escaped name in the title")
		}
		dialog := p25OpeningTag(t, body, `<dialog id="welcome-alerts-ask"`)
		if !strings.Contains(dialog, `data-home-area="Park &lt;i&gt;X&lt;/i&gt;"`) {
			t.Errorf("the dialog %q lacks the escaped name in data-home-area", dialog)
		}
	})
}

// TestWelcomeIsNotDoneBeforeFirstRunDone pins decision 15: the flow is recorded
// as done only by POST /api/me/first-run-done, which welcome.js sends after
// step 4 (directly, or after the alerts pop-up's answer; quick 261027-t5w). A
// person who agreed and saved the profile of step 4 but never sent it still has
// first_run_done_at NULL, and /welcome shows the flow again.
func TestWelcomeIsNotDoneBeforeFirstRunDone(t *testing.T) {
	base, newClient, pool := welcomeServer(t)
	const email = "welcome-notdone@example.com"
	client, still := newClient(email)

	meWant(t, "consent", meSend(t, client, http.MethodPost, base+"/api/me/consent", ""), http.StatusNoContent)
	// Amended 2026-10-29 by Phase 7.1 plan 10: step 4 saves the username, the name
	// and the home area (all three are required). Still not done by that alone.
	fillProfileAt(t, client, base, "notdone.user")

	var done bool
	if err := pool.QueryRow(context.Background(),
		`SELECT first_run_done_at IS NOT NULL FROM accounts WHERE email = $1`, email).Scan(&done); err != nil {
		t.Fatalf("reading first_run_done_at: %v", err)
	}
	if done {
		t.Errorf("first_run_done_at is set after step 4 alone: the flow is done only by first-run-done")
	}
	resp, _ := welcomeGet(t, still, base, "/welcome")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /welcome before first-run-done: %d, want the flow (200)", resp.StatusCode)
	}
	welcomeWantRedirect(t, still, base, "/", http.StatusFound, "/welcome")

	// The first-run-done call finishes it.
	meWant(t, "done", meSend(t, client, http.MethodPost, base+"/api/me/first-run-done", ""), http.StatusNoContent)
	welcomeWantRedirect(t, still, base, "/welcome", http.StatusSeeOther, "/")
}

// TestWelcomeRendersOwnAccountID pins skeptic S-2 (Phase 3.4 plan 27): the
// first time flow page carries the signed in account's own numeric id on its
// root element, so the position it saves in step 3 has an owner. It is a
// number, never the address, and differs per account.
func TestWelcomeRendersOwnAccountID(t *testing.T) {
	base, newClient, pool := welcomeServer(t)
	const emailA, emailB = "welcome-id-a@example.com", "welcome-id-b@example.com"
	_, stillA := newClient(emailA)
	_, stillB := newClient(emailB)
	idA := p25AccountID(t, pool, emailA)
	idB := p25AccountID(t, pool, emailB)
	if idA == idB {
		t.Fatalf("two accounts share the id %d", idA)
	}

	_, bodyA := welcomeGet(t, stillA, base, "/welcome")
	tagA := p25OpeningTag(t, bodyA, `<div class="welcome" id="welcome"`)
	wantA := `data-account-id="` + strconv.FormatInt(idA, 10) + `"`
	if !strings.Contains(tagA, wantA) {
		t.Errorf("account A's flow root %q does not carry %s", tagA, wantA)
	}
	if strings.Contains(tagA, "@") || strings.Count(bodyA, "data-account-id=") != 1 {
		t.Errorf("the id must appear once and never as an address: %q", tagA)
	}

	_, bodyB := welcomeGet(t, stillB, base, "/welcome")
	tagB := p25OpeningTag(t, bodyB, `<div class="welcome" id="welcome"`)
	wantB := `data-account-id="` + strconv.FormatInt(idB, 10) + `"`
	if !strings.Contains(tagB, wantB) || strings.Contains(bodyB, wantA) {
		t.Errorf("account B's flow root %q must carry %s and not account A's id", tagB, wantB)
	}
}
