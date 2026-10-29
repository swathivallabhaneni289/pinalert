// login_gate_e2e_test.go drives GET /login over the real router and a real
// Postgres (Phase 7.1, plan 11): the signed in redirect, the code screen that
// comes back after a reload, the one time notice of the switch to passwords and
// the closed list of done words. The page address never holds an email, so no
// test here puts one in a URL.
package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"pinalert/internal/testutil"
)

// gatePanelTag finds the opening tag of the section with the given id and
// returns it. It fails the test when the section is missing.
func gatePanelTag(t *testing.T, body, id string) string {
	t.Helper()
	re := regexp.MustCompile(`<section[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>`)
	tag := re.FindString(body)
	if tag == "" {
		t.Fatalf("the page has no section with id %q", id)
	}
	return tag
}

// gatePanelShown reports whether the section's opening tag lacks the hidden
// attribute.
func gatePanelShown(tag string) bool {
	return !regexp.MustCompile(`\shidden[\s>=]`).MatchString(tag)
}

// loginGatePage GETs /login with a client that never follows a redirect.
func loginGatePage(t *testing.T, client *http.Client, srv *httptest.Server, query string) (*http.Response, string) {
	t.Helper()
	return p421Get(t, client, srv.URL+"/login"+query)
}

func TestLoginGateRedirectsSignedIn(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	browser := newJarClient(t)
	verifySession(t, browser, srv, mailer, "gate-signed-in@example.com")
	noFollow := revNoFollow(browser)

	resp, _ := loginGatePage(t, noFollow, srv, "")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("signed in GET /login = %d to %q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("signed in redirect Cache-Control = %q, want no-store", cc)
	}

	// ?low=1 is saved first, and then the same request redirects to /low.
	resp, _ = loginGatePage(t, noFollow, srv, "?low=1")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/low" {
		t.Fatalf("signed in GET /login?low=1 = %d to %q, want 303 to /low", resp.StatusCode, resp.Header.Get("Location"))
	}
	if _, ok := p421LowCookie(resp.Cookies()); !ok {
		t.Errorf("signed in GET /login?low=1 did not set the Low data cookie before redirecting")
	}

	// The remembered choice now sends a plain visit to /low.
	resp, _ = loginGatePage(t, noFollow, srv, "")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/low" {
		t.Errorf("signed in GET /login with the Low data cookie = %d to %q, want 303 to /low", resp.StatusCode, resp.Header.Get("Location"))
	}

	// ?low=0 forgets it first, and then the redirect goes to /.
	resp, _ = loginGatePage(t, noFollow, srv, "?low=0")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Errorf("signed in GET /login?low=0 = %d to %q, want 303 to /", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestLoginGateComesBackToCode(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)

	t.Run("create", func(t *testing.T) {
		const email = "gate-code-create@example.com"
		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := signUpRequest(t, browser, srv, email, e2eTestPassword); status != http.StatusOK {
			t.Fatalf("sign up status = %d, want 200", status)
		}
		_ = mailer.codeFor(t, email)

		resp, body := loginGatePage(t, revNoFollow(browser), srv, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /login after a sign up request = %d, want 200", resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "" {
			t.Errorf("GET /login after a sign up request has a Location header %q", loc)
		}
		if !gatePanelShown(gatePanelTag(t, body, "auth-panel-code")) {
			t.Errorf("the code panel is hidden after a sign up request")
		}
		if gatePanelShown(gatePanelTag(t, body, "auth-panel-create")) {
			t.Errorf("the Create account panel is shown next to the code panel")
		}
		if !strings.Contains(body, `data-code-mode="create"`) {
			t.Errorf("the page does not carry data-code-mode=\"create\"")
		}
		if !regexp.MustCompile(`id="code-email"[^>]*>` + regexp.QuoteMeta(email) + `<`).MatchString(body) {
			t.Errorf("the address is not the text of the code-email element")
		}
		if n := strings.Count(body, email); n != 1 {
			t.Errorf("the address appears %d times in the page, want exactly once (page text only)", n)
		}
		if !regexp.MustCompile(`data-code-seconds-left="\d+"`).MatchString(body) {
			t.Errorf("the page carries no data-code-seconds-left")
		}
	})

	t.Run("reset", func(t *testing.T) {
		const email = "gate-code-reset@example.com"
		owner := newJarClient(t)
		verifySession(t, owner, srv, mailer, email)

		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := requestResetRequest(t, browser, srv, email); status != http.StatusOK {
			t.Fatalf("reset request status = %d, want 200", status)
		}
		resp, body := loginGatePage(t, revNoFollow(browser), srv, "")
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Location") != "" {
			t.Fatalf("GET /login after a reset request = %d with Location %q, want 200 and none", resp.StatusCode, resp.Header.Get("Location"))
		}
		if !gatePanelShown(gatePanelTag(t, body, "auth-panel-code")) {
			t.Errorf("the code panel is hidden after a reset request")
		}
		if !strings.Contains(body, `data-code-mode="reset"`) {
			t.Errorf("the page does not carry data-code-mode=\"reset\"")
		}
		if n := strings.Count(body, email); n != 1 {
			t.Errorf("the address appears %d times in the reset page, want exactly once", n)
		}
	})

	t.Run("fresh browser sees the entry panels", func(t *testing.T) {
		browser := newJarClient(t)
		_, body := loginGatePage(t, revNoFollow(browser), srv, "")
		if gatePanelShown(gatePanelTag(t, body, "auth-panel-code")) {
			t.Errorf("a browser with no code sees the code panel")
		}
		if !gatePanelShown(gatePanelTag(t, body, "auth-panel-create")) {
			t.Errorf("a fresh browser does not open on Create account")
		}
	})
}

func TestLoginGateSwitchNoticeOnce(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	const email = "gate-switch@example.com"
	browser := newJarClient(t)
	verifySession(t, browser, srv, mailer, email)

	// The state the migration leaves an old account in: no password, and every
	// session of it ended.
	accountID := revAccountID(t, pool, email)
	testutil.MustExec(t, pool, `UPDATE accounts SET password_hash = NULL WHERE id = $1`, accountID)
	revRevoke(t, pool, accountID)

	const switchLine = "Pinalert now uses passwords. Choose Create account with the same email to set yours. Your reports stay with you."

	resp, body := loginGatePage(t, revNoFollow(browser), srv, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first GET /login = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(body, switchLine) {
		t.Fatalf("the first visit does not show the switch notice")
	}
	if !gatePanelShown(gatePanelTag(t, body, "auth-panel-create")) {
		t.Errorf("the switch notice does not open on Create account")
	}
	cleared := false
	for _, c := range resp.Cookies() {
		if c.Name == "pinalert_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("the first visit did not clear the pinalert_session cookie")
	}

	_, body = loginGatePage(t, revNoFollow(browser), srv, "")
	if strings.Contains(body, switchLine) {
		t.Errorf("the second visit shows the switch notice again")
	}

	// A stranger's browser never sees it.
	_, body = loginGatePage(t, revNoFollow(newJarClient(t)), srv, "")
	if strings.Contains(body, switchLine) {
		t.Errorf("a browser with no ended session sees the switch notice")
	}
}

func TestLoginGateDoneWords(t *testing.T) {
	srv, _, _ := newE2EServer(t)

	cases := []struct {
		query      string
		line       string
		wantCreate bool
	}{
		{"?done=old-link", "That sign in link no longer works. Choose Create account with the same email to set a password.", true},
		{"?done=all-devices", "You are logged out of all devices.", false},
		{"?done=deleted", "Your account has been deleted.", true},
		{"?done=whatever", "", true},
		{"?done=OLD-LINK", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			resp, body := loginGatePage(t, revNoFollow(newJarClient(t)), srv, tc.query)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /login%s = %d, want 200", tc.query, resp.StatusCode)
			}
			doneRE := regexp.MustCompile(`(?s)<p[^>]*\bid="login-done"[^>]*>(.*?)</p>`)
			m := doneRE.FindStringSubmatch(body)
			if tc.line == "" {
				if m != nil {
					t.Errorf("%s shows a done line %q, want none", tc.query, m[1])
				}
			} else if m == nil || strings.TrimSpace(m[1]) != tc.line {
				t.Errorf("%s done line = %v, want %q", tc.query, m, tc.line)
			}
			if got := gatePanelShown(gatePanelTag(t, body, "auth-panel-create")); got != tc.wantCreate {
				t.Errorf("%s: Create account panel shown = %v, want %v", tc.query, got, tc.wantCreate)
			}
			if got := gatePanelShown(gatePanelTag(t, body, "auth-panel-signin")); got == tc.wantCreate {
				t.Errorf("%s: Sign in panel shown = %v, want %v", tc.query, got, !tc.wantCreate)
			}
			// The word itself is never echoed.
			if strings.Contains(body, "whatever") {
				t.Errorf("%s echoed the unknown word into the page", tc.query)
			}
		})
	}
}
