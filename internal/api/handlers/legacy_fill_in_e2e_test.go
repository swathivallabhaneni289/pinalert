// legacy_fill_in_e2e_test.go: Phase 7.1 plan 10, Task 3 (D-08, SC5, IDENT-06).
// An account made with the old emailed link has no password, no username and
// maybe no home area, and the migration revoked its sessions (Option X). It
// comes back through Create account with its email and a typed code, lands on
// the fill in once, and reaches the app after step 4 with no second stop and no
// loop between / and /welcome. They need the disposable database the e2e
// harness opens (testutil.NewTestDB). Every package level identifier carries
// the legacy prefix.
package handlers_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
)

// welcomeGateTweak turns the first-time flow redirect on, as cmd/server/main.go
// does.
func welcomeGateTweak(d *api.Deps, _ *sqlcgen.Queries) { d.OnboardingGate = true }

// legacySessionSecret is the secret newE2EServerWith signs session cookies
// with, so a test can mint the cookie an old browser still holds.
const legacySessionSecret = "e2e-test-secret-not-for-production"

// legacySeed makes the account the link flow left behind: an email, the terms
// version given (agreed now), the first-time flow finished, a stored name, and
// no password, no username and no home area. It also inserts the session that
// the migration revoked, and returns the account id.
func legacySeed(t *testing.T, pool *pgxpool.Pool, email, terms, sessionID string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (email, terms_version, terms_agreed_at, first_run_done_at, display_name)
		 VALUES ($1, $2, now(), now(), 'Old Ravi') RETURNING id`, email, terms).Scan(&id); err != nil {
		t.Fatalf("seeding the legacy account %s: %v", email, err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO sessions (session_id, account_id, revoked_at) VALUES ($1, $2, now())`, sessionID, id); err != nil {
		t.Fatalf("seeding the revoked session of %s: %v", email, err)
	}
	return id
}

// legacyBrowser returns a client holding the signed cookie of the revoked
// session, and the same cookies without following redirects.
func legacyBrowser(t *testing.T, srvURL, sessionID string) (follow, still *http.Client) {
	t.Helper()
	mgr, err := session.NewManager([]byte(legacySessionSecret), false)
	if err != nil {
		t.Fatalf("constructing the session manager: %v", err)
	}
	follow = newJarClient(t)
	u, err := url.Parse(srvURL)
	if err != nil {
		t.Fatalf("parsing the server URL: %v", err)
	}
	follow.Jar.SetCookies(u, []*http.Cookie{{Name: session.CookieName, Value: mgr.Sign(sessionID), Path: "/"}})
	still = &http.Client{Jar: follow.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return follow, still
}

// legacyWantRedirect requires a redirect of the given status to target.
func legacyWantRedirect(t *testing.T, label string, c *http.Client, base, path string, status int, target string) {
	t.Helper()
	resp, body := welcomeGet(t, c, base, path)
	if resp.StatusCode != status || resp.Header.Get("Location") != target {
		t.Fatalf("%s: GET %s answered %d to %q, want %d to %q (%.120s)", label, path, resp.StatusCode, resp.Header.Get("Location"), status, target, body)
	}
}

func TestLegacyFillIn(t *testing.T) {
	srv, mail, pool := newE2EServerWith(t, welcomeGateTweak)

	t.Run("current terms: one stop at step 4", func(t *testing.T) {
		const email = "legacy-current@example.com"
		const sid = "legacy-session-current"
		id := legacySeed(t, pool, email, service.CurrentTermsVersion, sid)
		follow, still := legacyBrowser(t, srv.URL, sid)

		// The old session reads as signed out.
		legacyWantRedirect(t, "the revoked session", still, srv.URL, "/", http.StatusFound, "/login")

		// Create account with the address and a new password. The account has no
		// password yet, so the mail is the sign up code, not the account exists
		// notice, and typing it signs in as the same account.
		if got := signUpRequest(t, follow, srv, email, e2eTestPassword); got != http.StatusOK {
			t.Fatalf("Create account for the legacy address: status %d, want 200", got)
		}
		msg, ok := mail.lastAuthFor(email)
		if !ok || msg.Kind != mailer.KindCodeSignup {
			t.Fatalf("mail to the legacy address = %+v (sent %v), want kind %s", msg, ok, mailer.KindCodeSignup)
		}
		if got := confirmSignUpCode(t, follow, srv, mail.codeFor(t, email)); got != http.StatusOK {
			t.Fatalf("typing the code: status %d, want 200", got)
		}
		var after int64
		if err := pool.QueryRow(context.Background(), `SELECT id FROM accounts WHERE email = $1`, email).Scan(&after); err != nil || after != id {
			t.Fatalf("the account after the code = %d (%v), want the seeded %d", after, err, id)
		}
		var kept int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions WHERE account_id = $1`, id).Scan(&kept); err != nil || kept < 2 {
			t.Fatalf("sessions of the account = %d (%v), want the old one and the new one", kept, err)
		}

		// Signed in, but the profile is missing: / goes to /welcome, which opens
		// at step 4 with the stored name.
		legacyWantRedirect(t, "after the code", still, srv.URL, "/", http.StatusFound, "/welcome")
		resp, body := welcomeGet(t, still, srv.URL, "/welcome")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /welcome: %d, want 200", resp.StatusCode)
		}
		if got := welcomeAttr(t, body, "data-start-step"); got != "4" {
			t.Errorf("data-start-step = %q, want 4", got)
		}
		if !strings.Contains(body, `value="Old Ravi"`) {
			t.Errorf("step 4 should show the stored name")
		}

		// Step 4 saves the username and the home area (the name is there), then
		// the end of the flow sends first-run-done, and the app opens.
		meWant(t, "step 4", meSend(t, follow, http.MethodPatch, srv.URL+"/api/me/profile",
			`{"username":"old.ravi","home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
		meWant(t, "first-run-done", meSend(t, follow, http.MethodPost, srv.URL+"/api/me/first-run-done", ""), http.StatusNoContent)
		if resp, body := welcomeGet(t, still, srv.URL, "/"); resp.StatusCode != http.StatusOK {
			t.Fatalf("GET / after the fill in: %d, want 200 (%.120s)", resp.StatusCode, body)
		}

		// Nobody is stopped twice: a new browser signs in with the new password
		// and reaches the app at once.
		again := newJarClient(t)
		verifySession(t, again, srv, mail, email)
		stillAgain := &http.Client{Jar: again.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		if resp, body := welcomeGet(t, stillAgain, srv.URL, "/"); resp.StatusCode != http.StatusOK {
			t.Fatalf("GET / after signing in again: %d, want 200 with no second stop (%.120s)", resp.StatusCode, body)
		}
	})

	t.Run("older terms: step 2, then still the fill in", func(t *testing.T) {
		const email = "legacy-older@example.com"
		const sid = "legacy-session-older"
		legacySeed(t, pool, email, "2020-01-01", sid)
		follow, still := legacyBrowser(t, srv.URL, sid)

		if got := signUpRequest(t, follow, srv, email, e2eTestPassword); got != http.StatusOK {
			t.Fatalf("Create account for the legacy address: status %d, want 200", got)
		}
		if got := confirmSignUpCode(t, follow, srv, mail.codeFor(t, email)); got != http.StatusOK {
			t.Fatalf("typing the code: status %d, want 200", got)
		}

		legacyWantRedirect(t, "older terms", still, srv.URL, "/", http.StatusFound, "/welcome")
		_, body := welcomeGet(t, still, srv.URL, "/welcome")
		if got := welcomeAttr(t, body, "data-start-step"); got != "2" {
			t.Errorf("older terms: data-start-step = %q, want 2", got)
		}

		// Agree. The profile is still missing, so the app stays closed and the
		// page now opens at step 4 (the client goes there from Agree, plan 13).
		meWant(t, "consent", meSend(t, follow, http.MethodPost, srv.URL+"/api/me/consent", ""), http.StatusNoContent)
		legacyWantRedirect(t, "after Agree", still, srv.URL, "/", http.StatusFound, "/welcome")
		_, body = welcomeGet(t, still, srv.URL, "/welcome")
		if got := welcomeAttr(t, body, "data-start-step"); got != "4" {
			t.Errorf("after Agree: data-start-step = %q, want 4", got)
		}

		fillProfile(t, follow, srv, "older.ravi")
		meWant(t, "first-run-done", meSend(t, follow, http.MethodPost, srv.URL+"/api/me/first-run-done", ""), http.StatusNoContent)
		if resp, body := welcomeGet(t, still, srv.URL, "/"); resp.StatusCode != http.StatusOK {
			t.Fatalf("GET / after the fill in: %d, want 200 (%.120s)", resp.StatusCode, body)
		}
	})
}

// TestLegacyFillInNoLoop pins research Pitfall 3: an account on the current
// terms that finished the flow but has no username is sent from / to /welcome
// once, and /welcome answers the page. It never sends the browser back to /.
func TestLegacyFillInNoLoop(t *testing.T) {
	srv, mail, pool := newE2EServerWith(t, welcomeGateTweak)
	const email = "legacy-noloop@example.com"
	client := newJarClient(t)
	verifySession(t, client, srv, mail, email)
	welcomeSetState(t, pool, email, service.CurrentTermsVersion, true)

	// Follow the redirects by hand and record every hop.
	var hops []string
	still := &http.Client{Jar: client.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	path := "/"
	var final *http.Response
	for i := 0; i < 6; i++ {
		hops = append(hops, path)
		resp, err := still.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		meBody(t, resp)
		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			final = resp
			break
		}
		path = resp.Header.Get("Location")
	}
	if final == nil {
		t.Fatalf("the redirects never ended: %v", hops)
	}
	if len(hops) != 2 || hops[0] != "/" || hops[1] != "/welcome" || final.StatusCode != http.StatusOK {
		t.Fatalf("the chain was %v ending in %d, want exactly / then /welcome (200)", hops, final.StatusCode)
	}

	// A second visit to /welcome is the same page, not a redirect.
	resp, _ := welcomeGet(t, still, srv.URL, "/welcome")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /welcome again: %d, want 200", resp.StatusCode)
	}

	// After step 4 the same account is let in and /welcome sends it to /.
	fillProfile(t, client, srv, "noloop.user")
	meWant(t, "first-run-done", meSend(t, client, http.MethodPost, srv.URL+"/api/me/first-run-done", ""), http.StatusNoContent)
	legacyWantRedirect(t, "a complete account", still, srv.URL, "/welcome", http.StatusSeeOther, "/")
	if resp, _ := welcomeGet(t, still, srv.URL, "/"); resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / after step 4: %d, want 200", resp.StatusCode)
	}
}
