// login_gate_spent_e2e_test.go is the server half of the Phase 7.1 review fix
// round 3 (skeptic SK2-01). A code submit that ran out of time reloads the sign
// in page; when the server had already confirmed the code, the reloaded page
// must say what happened instead of drawing a blank Create account card. The
// browser never received the answer, so it still carries the cookie it sent the
// code with. Each test copies that cookie into a second client, confirms the code
// there and throws the answer away, which is what a lost answer leaves behind.
package handlers_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

const (
	spentCreateLine = "Account created. Sign in to continue."
	spentResetLine  = "Password set. Sign in to continue."
)

// signinStatusOf returns the opening tag and the text of the signin-status line.
func signinStatusOf(t *testing.T, body string) (string, string) {
	t.Helper()
	m := regexp.MustCompile(`(?s)(<p[^>]*\bid="signin-status"[^>]*>)(.*?)</p>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the page has no signin-status line")
	}
	return m[1], strings.TrimSpace(m[2])
}

// assertSpentPage checks the page drawn for a code this browser spent: the Sign
// in panel, the status line, and the address as page text of the code-email
// element only, never in an attribute, a form value or the page address.
func assertSpentPage(t *testing.T, resp *http.Response, body, email, line string) {
	t.Helper()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Location") != "" {
		t.Fatalf("GET /login = %d with Location %q, want 200 and none", resp.StatusCode, resp.Header.Get("Location"))
	}
	if !gatePanelShown(gatePanelTag(t, body, "auth-panel-signin")) {
		t.Errorf("the Sign in panel is hidden")
	}
	for _, id := range []string{"auth-panel-create", "auth-panel-code", "auth-panel-forgot"} {
		if gatePanelShown(gatePanelTag(t, body, id)) {
			t.Errorf("the %s panel is shown next to Sign in", id)
		}
	}
	if !strings.Contains(body, `data-start-panel="signin"`) {
		t.Errorf("the card does not start on signin")
	}
	tag, text := signinStatusOf(t, body)
	if text != line {
		t.Errorf("signin-status = %q, want %q", text, line)
	}
	if regexp.MustCompile(`\shidden[\s>=]`).MatchString(tag) {
		t.Errorf("signin-status is hidden: %s", tag)
	}
	if !regexp.MustCompile(`id="code-email"[^>]*>` + regexp.QuoteMeta(email) + `<`).MatchString(body) {
		t.Errorf("the address is not the text of the code-email element")
	}
	if n := strings.Count(body, email); n != 1 {
		t.Errorf("the address appears %d times in the page, want exactly once (page text only)", n)
	}
	if strings.Contains(body, `="`+email) || strings.Contains(body, `value=`+email) {
		t.Errorf("the address is written into an attribute")
	}
	if strings.Contains(body, `id="login-done"`) {
		t.Errorf("the spent state wrote a done line; the done list stays closed")
	}
}

func TestLoginGateSpentCode(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)

	t.Run("create account confirmed but the answer was lost", func(t *testing.T) {
		const email = "gate-spent-create@example.com"
		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := signUpRequest(t, browser, srv, email, e2eTestPassword); status != http.StatusOK {
			t.Fatalf("sign up status = %d, want 200", status)
		}
		lost := revCookieCopy(t, browser, srv)
		if status := confirmSignUpCode(t, lost, srv, mailer.codeFor(t, email)); status != http.StatusOK {
			t.Fatalf("confirm status = %d, want 200", status)
		}

		resp, body := loginGatePage(t, revNoFollow(browser), srv, "")
		assertSpentPage(t, resp, body, email, spentCreateLine)

		// The same page again: still a plain page, no redirect, so nothing loops.
		resp, body = loginGatePage(t, revNoFollow(browser), srv, "")
		assertSpentPage(t, resp, body, email, spentCreateLine)

		// The person signs in with the password chosen at Create account.
		if status := signInRequest(t, browser, srv, email, e2eTestPassword); status != http.StatusOK {
			t.Fatalf("sign in after the spent page = %d, want 200", status)
		}
	})

	t.Run("forgot password confirmed but the answer was lost", func(t *testing.T) {
		const email = "gate-spent-reset@example.com"
		verifySession(t, newJarClient(t), srv, mailer, email)

		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := requestResetRequest(t, browser, srv, email); status != http.StatusOK {
			t.Fatalf("reset request status = %d, want 200", status)
		}
		lost := revCookieCopy(t, browser, srv)
		if status := confirmResetRequest(t, lost, srv, mailer.codeFor(t, email), "a brand new password 77"); status != http.StatusOK {
			t.Fatalf("reset confirm status = %d, want 200", status)
		}

		resp, body := loginGatePage(t, revNoFollow(browser), srv, "")
		assertSpentPage(t, resp, body, email, spentResetLine)
	})

	t.Run("a wrong code keeps the code screen", func(t *testing.T) {
		const email = "gate-spent-wrong@example.com"
		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := signUpRequest(t, browser, srv, email, e2eTestPassword); status != http.StatusOK {
			t.Fatalf("sign up status = %d, want 200", status)
		}
		code := mailer.codeFor(t, email)
		wrong := "000000"
		if code == wrong {
			wrong = "111111"
		}
		if status := confirmSignUpCode(t, revCookieCopy(t, browser, srv), srv, wrong); status != http.StatusBadRequest {
			t.Fatalf("wrong code status = %d, want 400", status)
		}
		_, body := loginGatePage(t, revNoFollow(browser), srv, "")
		if !gatePanelShown(gatePanelTag(t, body, "auth-panel-code")) {
			t.Errorf("a wrong code did not keep the code screen")
		}
		if strings.Contains(body, spentCreateLine) {
			t.Errorf("a wrong code shows the account created line")
		}
	})

	t.Run("create account for an address that has a password keeps the code screen", func(t *testing.T) {
		// The account_exists row can never be confirmed, so a browser that typed
		// a code for it sees the same code screen as for a new address.
		const email = "gate-spent-exists@example.com"
		verifySession(t, newJarClient(t), srv, mailer, email)
		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := signUpRequest(t, browser, srv, email, e2eTestPassword); status != http.StatusOK {
			t.Fatalf("sign up status = %d, want 200", status)
		}
		if status := confirmSignUpCode(t, revCookieCopy(t, browser, srv), srv, "123456"); status != http.StatusBadRequest {
			t.Fatalf("confirm on an account_exists row = %d, want 400", status)
		}
		_, body := loginGatePage(t, revNoFollow(browser), srv, "")
		if !gatePanelShown(gatePanelTag(t, body, "auth-panel-code")) {
			t.Errorf("an account_exists row did not keep the code screen")
		}
		if strings.Contains(body, spentCreateLine) || strings.Contains(body, spentResetLine) {
			t.Errorf("an account_exists row shows a spent line")
		}
	})

	t.Run("a done word wins over a spent code", func(t *testing.T) {
		const email = "gate-spent-done@example.com"
		browser := newJarClient(t)
		getLoginPage(t, browser, srv)
		if status := signUpRequest(t, browser, srv, email, e2eTestPassword); status != http.StatusOK {
			t.Fatalf("sign up status = %d, want 200", status)
		}
		if status := confirmSignUpCode(t, revCookieCopy(t, browser, srv), srv, mailer.codeFor(t, email)); status != http.StatusOK {
			t.Fatalf("confirm status = %d, want 200", status)
		}
		_, body := loginGatePage(t, revNoFollow(browser), srv, "?done=all-devices")
		if !strings.Contains(body, "You are logged out of all devices.") {
			t.Errorf("the done line is missing")
		}
		if strings.Contains(body, spentCreateLine) || strings.Contains(body, email) {
			t.Errorf("the spent state overrode the done word")
		}
	})

	t.Run("a fresh browser sees neither line", func(t *testing.T) {
		_, body := loginGatePage(t, revNoFollow(newJarClient(t)), srv, "")
		if strings.Contains(body, spentCreateLine) || strings.Contains(body, spentResetLine) {
			t.Errorf("a fresh browser sees a spent line")
		}
		tag, text := signinStatusOf(t, body)
		if text != "" || !regexp.MustCompile(`\shidden[\s>=]`).MatchString(tag) {
			t.Errorf("a fresh browser's signin-status = %q in %s, want empty and hidden", text, tag)
		}
	})
}
