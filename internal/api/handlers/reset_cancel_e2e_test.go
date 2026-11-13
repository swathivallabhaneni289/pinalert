// reset_cancel_e2e_test.go pins POST /api/auth/reset/cancel (quick 261113-r5w,
// review fix WR-01): Change email on step two of Forgot password turns the confirm
// off on the server, so a reload afterwards draws the code screen and never step
// two, and the route answers the same whether or not there was anything to clear.
package handlers_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestResetCancelTurnsTheConfirmOff(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-cancel@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)

	r := newLoginClient(t, srv)
	if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)
	if status := confirmResetCodeRequest(t, r, srv, code); status != http.StatusOK {
		t.Fatalf("step one status = %d, want 200", status)
	}
	_, page := loginGatePage(t, r, srv, "")
	if !strings.Contains(page, `data-code-step="password"`) {
		t.Fatalf("a reload after step one does not draw step two")
	}

	// Another browser with its own confirm, to show the cancel touches only the
	// caller's own session.
	other := newLoginClient(t, srv)
	if status := requestResetRequest(t, other, srv, email); status != http.StatusOK {
		t.Fatalf("the other browser's reset request status = %d, want 200", status)
	}
	otherCode := mail.codeFor(t, email)
	if status := confirmResetCodeRequest(t, other, srv, otherCode); status != http.StatusOK {
		t.Fatalf("the other browser's step one status = %d, want 200", status)
	}

	// Change email on step two: the cancel.
	cancelled := doAuth(t, r, srv, "/api/auth/reset/cancel", map[string]string{})
	if cancelled.status != http.StatusOK || strings.TrimSpace(string(cancelled.body)) != `{"cancelled":true}` {
		t.Fatalf("cancel = %d %s, want 200 and cancelled", cancelled.status, cancelled.body)
	}
	if cancelled.setCookie {
		t.Errorf("the cancel set a cookie")
	}

	// A reload shows the code screen, never step two, and step two is refused.
	_, page = loginGatePage(t, r, srv, "")
	if strings.Contains(page, `data-code-step="password"`) || !strings.Contains(page, `data-code-step="code"`) {
		t.Errorf("a reload after the cancel does not draw step one")
	}
	got := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	expectError(t, "step two after the cancel", got, http.StatusBadRequest, "code", secWrongCodeSentence)
	// The other browser's confirm is its own and is untouched.
	if _, otherPage := loginGatePage(t, other, srv, ""); !strings.Contains(otherPage, `data-code-step="password"`) {
		t.Errorf("the cancel of one browser turned off the confirm of another")
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("the old password after a refused step two: status = %d, want 200", status)
	}

	// The right code typed again confirms again, and the reset then completes.
	if status := confirmResetCodeRequest(t, r, srv, code); status != http.StatusOK {
		t.Fatalf("the code typed again: status = %d, want 200", status)
	}
	_, page = loginGatePage(t, r, srv, "")
	if !strings.Contains(page, `data-code-step="password"`) {
		t.Errorf("the code typed again does not draw step two")
	}
	if status := setResetPasswordRequest(t, r, srv, secNewPassword); status != http.StatusOK {
		t.Fatalf("step two after the code was typed again: status = %d, want 200", status)
	}
}

// TestResetCancelAnswersTheSameEverywhere: a browser with a confirm, one with a
// code not yet confirmed, one with no code at all and one holding a no account
// row get the same bytes and headers, so the route says nothing about any of them.
func TestResetCancelAnswersTheSameEverywhere(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		knownAddr   = "cancel-same-known@example.com"
		unknownAddr = "cancel-same-unknown@example.com"
	)
	verifySession(t, newPlainJarClient(t), srv, mail, knownAddr)
	ageClaim(t, pool, knownAddr)

	confirmed := newLoginClient(t, srv)
	pending := newLoginClient(t, srv)
	none := newLoginClient(t, srv)
	ghost := newLoginClient(t, srv)
	if status := requestResetRequest(t, confirmed, srv, knownAddr); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	if status := confirmResetCodeRequest(t, confirmed, srv, mail.codeFor(t, knownAddr)); status != http.StatusOK {
		t.Fatalf("step one status = %d, want 200", status)
	}
	if status := requestResetRequest(t, pending, srv, knownAddr); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	if status := requestResetRequest(t, ghost, srv, unknownAddr); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}

	a := doAuth(t, confirmed, srv, "/api/auth/reset/cancel", map[string]string{})
	b := doAuth(t, pending, srv, "/api/auth/reset/cancel", map[string]string{})
	c := doAuth(t, none, srv, "/api/auth/reset/cancel", map[string]string{})
	d := doAuth(t, ghost, srv, "/api/auth/reset/cancel", map[string]string{})
	if a.status != http.StatusOK || strings.TrimSpace(string(a.body)) != `{"cancelled":true}` {
		t.Fatalf("cancel with a confirm = %d %s", a.status, a.body)
	}
	compareAuthAnswers(t, "cancel: a confirm vs a pending code", a, b)
	compareAuthAnswers(t, "cancel: a confirm vs no code", a, c)
	compareAuthAnswers(t, "cancel: a confirm vs a no account row", a, d)
	if a.setCookie {
		t.Errorf("the cancel set a cookie")
	}

	// The strict body rule of its siblings: a body with a member is refused.
	bad := doAuth(t, none, srv, "/api/auth/reset/cancel", map[string]string{"code": "123456"})
	if bad.status != http.StatusBadRequest {
		t.Errorf("a cancel with a member = %d, want 400", bad.status)
	}
}
