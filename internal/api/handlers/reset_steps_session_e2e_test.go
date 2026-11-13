// reset_steps_session_e2e_test.go pins what the review of Forgot password in two
// steps asked for (quick 261113-r5w, review fix WR-02): step one moves the
// browser to a freshly drawn session id, so a session id planted in the browser
// before the code was typed cannot be used at step two.
package handlers_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestResetStepTwoRefusesAPlantedSession: an attacker who holds a session id S
// plants it in the victim's browser and asks for the reset on S; the victim
// types the emailed code, and the confirm moves to a fresh id that only the
// victim's browser received. The attacker, still on S, can neither see step two
// nor use it.
func TestResetStepTwoRefusesAPlantedSession(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-planted@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)

	attacker := newLoginClient(t, srv)
	planted := sessionIDFromJar(t, attacker, srv.URL)
	victim := plantedClient(t, srv, attacker)
	if status := requestResetRequest(t, attacker, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}

	// The victim's page shows the code screen for the address, and the victim
	// types the emailed code.
	_, page := loginGatePage(t, victim, srv, "")
	if !strings.Contains(page, `data-code-step="code"`) {
		t.Fatalf("the victim's page does not show the code screen")
	}
	code := mail.codeFor(t, email)
	right := doAuth(t, victim, srv, "/api/auth/reset/code", map[string]string{"code": code})
	if right.status != http.StatusOK || !right.setCookie {
		t.Fatalf("the victim's step one = %d with a cookie %v, want 200 and a fresh cookie", right.status, right.setCookie)
	}
	if got := sessionIDFromJar(t, victim, srv.URL); got == planted {
		t.Fatalf("the victim kept the planted session id after step one")
	}

	// The attacker's id holds nothing: no step two page, no step two.
	_, page = loginGatePage(t, attacker, srv, "")
	if strings.Contains(page, `data-code-step="password"`) {
		t.Errorf("the planted session still draws step two")
	}
	got := doAuth(t, attacker, srv, "/api/auth/reset/password", map[string]string{"password": "attacker chosen password 17"})
	expectError(t, "step two on the planted session", got, http.StatusBadRequest, "code", secWrongCodeSentence)
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("the old password after the attacker's step two: status = %d, want 200", status)
	}

	// The victim, on the fresh id, still gets step two.
	_, page = loginGatePage(t, victim, srv, "")
	if !strings.Contains(page, `data-code-step="password"`) {
		t.Errorf("the victim's reload does not show step two")
	}
	done := doAuth(t, victim, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	if done.status != http.StatusOK {
		t.Fatalf("the victim's step two = %d %s, want 200", done.status, done.body)
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, secNewPassword); status != http.StatusOK {
		t.Fatalf("sign in with the victim's new password: status = %d, want 200", status)
	}
}

// TestResetStepOneCookieBehaviourIsTheSame pins the claim behind the rotation:
// the only way to a new cookie at step one is a right code on a code delivery
// row, so an address with no account is never told apart from a wrong code. Both
// kinds of browser answer a wrong code without a cookie and keep their session
// id, and a right code for a no account row does not exist to be typed.
func TestResetStepOneCookieBehaviourIsTheSame(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		knownAddr   = "cookie-same-known@example.com"
		unknownAddr = "cookie-same-unknown@example.com"
	)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)
	ageClaim(t, pool, knownAddr)

	known := newLoginClient(t, srv)
	unknown := newLoginClient(t, srv)
	for _, c := range []struct {
		client *http.Client
		addr   string
	}{{known, knownAddr}, {unknown, unknownAddr}} {
		if status := requestResetRequest(t, c.client, srv, c.addr); status != http.StatusOK {
			t.Fatalf("reset request for %s status = %d, want 200", c.addr, status)
		}
	}
	knownBefore := sessionIDFromJar(t, known, srv.URL)
	unknownBefore := sessionIDFromJar(t, unknown, srv.URL)
	knownCode := mail.codeFor(t, knownAddr)

	for i := 0; i < 3; i++ {
		a := doAuth(t, known, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(knownCode)})
		b := doAuth(t, unknown, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(knownCode)})
		compareAuthAnswers(t, "wrong code on a known vs an unknown address", a, b)
		if a.setCookie || b.setCookie {
			t.Fatalf("a wrong code set a cookie (known %v, unknown %v)", a.setCookie, b.setCookie)
		}
	}
	// Even the known address's own right code, typed in the unknown browser, is a
	// plain wrong code: it sets no cookie and moves nothing.
	c := doAuth(t, unknown, srv, "/api/auth/reset/code", map[string]string{"code": knownCode})
	expectError(t, "a right code typed in the no account browser", c, http.StatusBadRequest, "code", secWrongCodeSentence)
	if c.setCookie {
		t.Errorf("a right code typed in the no account browser set a cookie")
	}
	if got := sessionIDFromJar(t, known, srv.URL); got != knownBefore {
		t.Errorf("a wrong code moved the known browser to %q", got)
	}
	if got := sessionIDFromJar(t, unknown, srv.URL); got != unknownBefore {
		t.Errorf("a wrong code moved the no account browser to %q", got)
	}
}
