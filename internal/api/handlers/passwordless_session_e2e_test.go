// passwordless_session_e2e_test.go: Phase 7.1 plan 10b (revision W1, D-14,
// IDENT-06, IDENT-07). The migration revokes every session that existed when it
// ran, but the old binary can still create a session from an emailed link in the
// deploy gap before the new binary starts. A live, unrevoked session of an
// account with no password must be signed out everywhere, and the only way back
// is Create account with the address and a typed code. These tests need the
// disposable database the e2e harness opens (testutil.NewTestDB). Every package
// level identifier carries the passwordless prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// passwordlessSwitchLine is the one time notice of the switch to passwords
// (plan 11).
const passwordlessSwitchLine = "Pinalert now uses passwords. Choose Create account with the same email to set yours. Your reports stay with you."

// passwordlessReportsPath is a gated read that answers 200 for a signed in browser.
const passwordlessReportsPath = "/api/reports?lat=12.9716&lon=77.5946"

// passwordlessSeed makes the account the old link flow left behind (an email
// and nothing else) and a LIVE, unrevoked session bound to it, which is what
// the old binary can create after the migration ran. It returns the account id.
func passwordlessSeed(t *testing.T, pool *pgxpool.Pool, email, sessionID string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO accounts (email) VALUES ($1) RETURNING id`, email).Scan(&id); err != nil {
		t.Fatalf("seeding the password-less account %s: %v", email, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)`, sessionID, id); err != nil {
		t.Fatalf("seeding the live session of %s: %v", email, err)
	}
	return id
}

// passwordlessBrowser returns a client holding the signed cookie of sessionID.
// The cookie is signed with the secret newE2EServerWith uses, the way a real
// browser would hold the cookie the old binary set.
func passwordlessBrowser(t *testing.T, srvURL, sessionID string) *http.Client {
	t.Helper()
	mgr, err := session.NewManager([]byte(legacySessionSecret), false)
	if err != nil {
		t.Fatalf("constructing the session manager: %v", err)
	}
	c := newJarClient(t)
	u, err := url.Parse(srvURL)
	if err != nil {
		t.Fatalf("parsing the server URL: %v", err)
	}
	c.Jar.SetCookies(u, []*http.Cookie{{Name: session.CookieName, Value: mgr.Sign(sessionID), Path: "/"}})
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return c
}

func passwordlessTweak(email string) func(d *api.Deps, _ *sqlcgen.Queries) {
	return func(d *api.Deps, _ *sqlcgen.Queries) {
		d.ModeratorEmails = map[string]struct{}{email: {}}
	}
}

func TestPasswordlessLiveSessionIsSignedOut(t *testing.T) {
	const email = "passwordless-live@example.com"
	const sid = "passwordless-live-session"
	srv, _, pool := newE2EServerWith(t, passwordlessTweak(email))
	passwordlessSeed(t, pool, email, sid)

	// The session row is live: it is not revoked.
	var revoked bool
	if err := pool.QueryRow(context.Background(),
		`SELECT revoked_at IS NOT NULL FROM sessions WHERE session_id = $1`, sid).Scan(&revoked); err != nil || revoked {
		t.Fatalf("the seeded session should be live and unrevoked (revoked=%v, err=%v)", revoked, err)
	}
	c := passwordlessBrowser(t, srv.URL, sid)

	// The API answers 401 with field auth and the standard sentence.
	resp, body := welcomeGet(t, c, srv.URL, passwordlessReportsPath)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET reports = %d, want 401 (%.120s)", resp.StatusCode, body)
	}
	var env handlers.ErrorResponse
	if err := json.Unmarshal([]byte(body), &env); err != nil || env.Error.Field != "auth" || env.Error.Message != "Sign in to continue." {
		t.Errorf("401 body = %q (%v), want field auth and \"Sign in to continue.\"", body, err)
	}

	// A page goes to /login, never cached.
	resp, body = welcomeGet(t, c, srv.URL, "/")
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/login" {
		t.Errorf("GET / = %d to %q, want 302 to /login (%.120s)", resp.StatusCode, resp.Header.Get("Location"), body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("GET / Cache-Control = %q, want no-store", got)
	}

	// The moderator gate holds too, though the address is on the list: a plain
	// 404, the same as any other refusal.
	for _, path := range []string{"/moderate", "/api/moderation/queue"} {
		resp, body := welcomeGet(t, c, srv.URL, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (%.120s)", path, resp.StatusCode, body)
		}
	}

	// /login shows the sign in card with the notice of the switch, on Create
	// account, and does not redirect this browser away as a signed in one.
	resp, body = welcomeGet(t, c, srv.URL, "/login")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login = %d, want 200 (%.120s)", resp.StatusCode, body)
	}
	if !strings.Contains(body, passwordlessSwitchLine) {
		t.Errorf("GET /login does not show the switch notice")
	}
	if !gatePanelShown(gatePanelTag(t, body, "auth-panel-create")) {
		t.Errorf("the switch notice does not open on Create account")
	}
}

func TestPasswordlessSessionCanSignInAgain(t *testing.T) {
	const email = "passwordless-back@example.com"
	const sid = "passwordless-back-session"
	srv, mail, pool := newE2EServerWith(t, passwordlessTweak(email))
	id := passwordlessSeed(t, pool, email, sid)
	c := passwordlessBrowser(t, srv.URL, sid)

	if resp, _ := welcomeGet(t, c, srv.URL, passwordlessReportsPath); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("before the code: GET reports = %d, want 401", resp.StatusCode)
	}

	// Create account with the address and a typed code.
	if got := signUpRequest(t, c, srv, email, e2eTestPassword); got != http.StatusOK {
		t.Fatalf("Create account: status %d, want 200", got)
	}
	if got := confirmSignUpCode(t, c, srv, mail.codeFor(t, email)); got != http.StatusOK {
		t.Fatalf("typing the code: status %d, want 200", got)
	}

	// The same account, now with a password, on a session id that is not the old one.
	var after int64
	var hasPassword bool
	if err := pool.QueryRow(context.Background(),
		`SELECT id, password_hash IS NOT NULL FROM accounts WHERE email = $1`, email).Scan(&after, &hasPassword); err != nil || after != id || !hasPassword {
		t.Fatalf("the account after the code = %d, password %v (%v), want the seeded %d with a password", after, hasPassword, err, id)
	}
	var fresh int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM sessions WHERE account_id = $1 AND session_id <> $2 AND revoked_at IS NULL`, id, sid).Scan(&fresh); err != nil || fresh < 1 {
		t.Fatalf("live sessions of the account other than the old one = %d (%v), want at least 1", fresh, err)
	}

	if resp, body := welcomeGet(t, c, srv.URL, passwordlessReportsPath); resp.StatusCode != http.StatusOK {
		t.Fatalf("after the code: GET reports = %d, want 200 (%.120s)", resp.StatusCode, body)
	}
	// The listed address is a moderator again once it has a password.
	if resp, body := welcomeGet(t, c, srv.URL, "/api/moderation/queue"); resp.StatusCode == http.StatusNotFound {
		t.Errorf("after the code: GET /api/moderation/queue = 404, want the moderator to be let in (%.120s)", body)
	}
}

// TestSeedPasswordMatchesHelper pins that the password the seed helpers store is
// the one the e2e helpers type, so verifySession's sign in first path works for
// seeded accounts.
// TestSwitchNoticeKeepsDeviceBucket is the second half of WR-S01 of the Phase
// 7.1 review: the switch notice must leave the browser with a fresh valid
// cookie, not clear it. Before the fix the notice cleared the cookie, the
// next Create account arrived with none, and the person landed in the one
// small bucket shared by every cookieless sender on the server; once another
// client had drained it, the person was refused.
func TestSwitchNoticeKeepsDeviceBucket(t *testing.T) {
	const email = "passwordless-notice@example.com"
	const sid = "passwordless-notice-session"
	srv, mail, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.AuthLimits.CodeSendNoCookie = api.BucketLimit{Every: time.Hour, Burst: 1}
	})
	passwordlessSeed(t, pool, email, sid)
	c := passwordlessBrowser(t, srv.URL, sid)

	resp, body := welcomeGet(t, c, srv.URL, "/login")
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, passwordlessSwitchLine) {
		t.Fatalf("GET /login = %d, notice shown %v; want 200 with the switch notice", resp.StatusCode, strings.Contains(body, passwordlessSwitchLine))
	}

	// Another client with no cookie at all drains the shared bucket.
	stranger := &http.Client{}
	if got, _, _ := postAuthJSON(t, stranger, srv, "/api/auth/sign-up",
		map[string]string{"email": "stranger@example.com", "password": e2eTestPassword}); got != http.StatusOK {
		t.Fatalf("the cookieless stranger's sign up = %d, want 200", got)
	}

	if got := signUpRequest(t, c, srv, email, e2eTestPassword); got != http.StatusOK {
		t.Fatalf("Create account after the switch notice = %d, want 200: the notice must leave a valid cookie", got)
	}
	if got := confirmSignUpCode(t, c, srv, mail.codeFor(t, email)); got != http.StatusOK {
		t.Fatalf("typing the code after the notice = %d, want 200", got)
	}
}

func TestSeedPasswordMatchesHelper(t *testing.T) {
	if e2eTestPassword != testutil.TestAccountPassword {
		t.Fatalf("e2eTestPassword = %q but testutil.TestAccountPassword = %q; they must be equal", e2eTestPassword, testutil.TestAccountPassword)
	}
}
