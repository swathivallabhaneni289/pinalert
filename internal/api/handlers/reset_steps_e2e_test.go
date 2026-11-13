// reset_steps_e2e_test.go drives Forgot password as two steps over the real
// router and a real Postgres (quick 261113-r5w): POST /api/auth/reset/code takes
// the typed code, POST /api/auth/reset/password takes the new password, and the
// second only works for a browser whose code the first confirmed.
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sessionAccountIDOrNone is the account a session row is bound to, nil for an
// unbound row and for an id with no stored row at all (the fresh id step one
// draws is stored only when something binds it).
func sessionAccountIDOrNone(t *testing.T, pool *pgxpool.Pool, sessionID string) *int64 {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE session_id = $1", sessionID).Scan(&n); err != nil {
		t.Fatalf("counting the session row %q: %v", sessionID, err)
	}
	if n == 0 {
		return nil
	}
	return sessionAccountID(t, pool, sessionID)
}

// plantedClient is a browser that holds the attacker's session cookie, the way a
// cookie planted from a sibling subdomain would: the same signed value, copied
// from the attacker's jar into a fresh one.
func plantedClient(t *testing.T, srv *httptest.Server, attacker *http.Client) *http.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parsing server URL: %v", err)
	}
	victim := newPlainJarClient(t)
	victim.Jar.SetCookies(u, attacker.Jar.Cookies(u))
	if got, want := sessionIDFromJar(t, victim, srv.URL), sessionIDFromJar(t, attacker, srv.URL); got != want {
		t.Fatalf("the planted cookie did not land in the victim's jar")
	}
	return victim
}

func TestResetTwoStepsEndToEnd(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-two-steps@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)

	r := newLoginClient(t, srv)
	if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)

	// Step two before step one is refused and changes nothing.
	early := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	expectError(t, "step two before step one", early, http.StatusBadRequest, "code", secWrongCodeSentence)
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("the old password after a refused step two: status = %d, want 200", status)
	}

	// The page of an unconfirmed code is step one.
	_, page := loginGatePage(t, r, srv, "")
	if !strings.Contains(page, `data-code-step="code"`) || !strings.Contains(page, ">Confirm code<") {
		t.Errorf("an unconfirmed reset code does not draw step one")
	}

	// Step one: a wrong code is refused, the right one answers code_ok.
	wrong := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(code)})
	expectError(t, "step one with a wrong code", wrong, http.StatusBadRequest, "code", secWrongCodeSentence)
	if a := doAuth(t, newLoginClient(t, srv), srv, "/api/auth/reset/code", map[string]string{"code": code}); a.setCookie {
		t.Errorf("a wrong browser's step one set a cookie")
	}
	before := sessionIDFromJar(t, r, srv.URL)
	if wrong.setCookie {
		t.Errorf("a wrong code set a cookie")
	}
	right := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": code})
	if right.status != http.StatusOK || strings.TrimSpace(string(right.body)) != `{"code_ok":true}` {
		t.Fatalf("step one with the right code = %d %s, want 200 and code_ok", right.status, right.body)
	}
	// Review fix WR-02: a right code moves the browser to a freshly drawn session
	// id, which is still bound to no account, so step one signs nobody in.
	if !right.setCookie {
		t.Errorf("step one set no cookie: the browser must move to a fresh session id")
	}
	after := sessionIDFromJar(t, r, srv.URL)
	if after == before {
		t.Errorf("the session id did not change at step one: still %q", after)
	}
	if id := sessionAccountIDOrNone(t, pool, after); id != nil {
		t.Errorf("the fresh session %q is bound to account %d: step one must sign nobody in", after, *id)
	}
	if status := reportsStatus(t, r, srv); status != http.StatusUnauthorized {
		t.Errorf("GET /api/reports after step one = %d, want 401: step one must sign nobody in", status)
	}

	// A reload shows step two, with the address as page text only.
	_, page = loginGatePage(t, r, srv, "")
	for _, want := range []string{`data-code-step="password"`, ">Set new password<", "Code confirmed. Choose a new password."} {
		if !strings.Contains(page, want) {
			t.Errorf("a reload after step one lacks %q", want)
		}
	}
	if strings.Contains(page, `value="`+email) || strings.Contains(page, "="+email) {
		t.Errorf("the address is in an attribute of the page")
	}

	// Another browser holds no confirm.
	other := newLoginClient(t, srv)
	got := doAuth(t, other, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	expectError(t, "step two in another browser", got, http.StatusBadRequest, "code", secWrongCodeSentence)

	// A refused password keeps the confirm.
	short := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": "short"})
	expectError(t, "a short new password", short, http.StatusBadRequest, "password", "Use at least 8 characters.")
	_, page = loginGatePage(t, r, srv, "")
	if !strings.Contains(page, `data-code-step="password"`) {
		t.Errorf("a refused new password lost the confirm")
	}

	// Step two sets the password, signs nobody in and spends the code.
	done := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	if done.status != http.StatusOK || strings.TrimSpace(string(done.body)) != `{"password_set":true}` {
		t.Fatalf("step two = %d %s, want 200 and password_set", done.status, done.body)
	}
	if done.setCookie {
		t.Errorf("step two set a cookie: it must not sign the browser in")
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, secNewPassword); status != http.StatusOK {
		t.Fatalf("sign in with the new password: status = %d, want 200", status)
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, e2eTestPassword); status != http.StatusBadRequest {
		t.Fatalf("sign in with the old password: status = %d, want 400", status)
	}
	again := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": "yet another password 91"})
	expectError(t, "step two twice", again, http.StatusBadRequest, "code", secWrongCodeSentence)
	if status := confirmResetCodeRequest(t, r, srv, code); status != http.StatusBadRequest {
		t.Errorf("the spent code at step one: status = %d, want 400", status)
	}
}

// TestResetStepOneRefusesTheOldBody: the one step body with the password in it
// is not accepted any more, so a script written for it learns at once.
func TestResetStepOneRefusesTheOldBody(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-old-body@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)
	r := newLoginClient(t, srv)
	if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	old := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": mail.codeFor(t, email), "password": secNewPassword})
	if old.status != http.StatusBadRequest {
		t.Fatalf("the old two field body = %d, want 400", old.status)
	}
	if status := signInRequest(t, newLoginClient(t, srv), srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("the old password after the refused body: status = %d, want 200", status)
	}
}

// TestResetStepsEnumerationIdentical: both steps answer the same for an address
// with an account and one without, byte for byte and header for header, for a
// wrong code and for step two before any confirm.
func TestResetStepsEnumerationIdentical(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const (
		knownAddr   = "enum-steps-known@example.com"
		unknownAddr = "enum-steps-unknown@example.com"
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
	knownCode := mail.codeFor(t, knownAddr)

	// A wrong code at step one. The unknown address was never sent a code at all,
	// so any code is wrong; a made up six digits answers the same as the known
	// address's wrong code.
	a := doAuth(t, known, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(knownCode)})
	b := doAuth(t, unknown, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(knownCode)})
	expectError(t, "known address, wrong code", a, http.StatusBadRequest, "code", secWrongCodeSentence)
	compareAuthAnswers(t, "step one: wrong code on a known vs an unknown address", a, b)

	// Step two before any confirm.
	c := doAuth(t, known, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	d := doAuth(t, unknown, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	expectError(t, "known address, step two unconfirmed", c, http.StatusBadRequest, "code", secWrongCodeSentence)
	compareAuthAnswers(t, "step two: unconfirmed on a known vs an unknown address", c, d)
}

// TestResetStepOneBudget: the fifth wrong code switches the code off at step
// one, as it did when the code and the password went together.
func TestResetStepOneBudget(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-budget@example.com"
	verifySession(t, newPlainJarClient(t), srv, mail, email)
	ageClaim(t, pool, email)
	r := newLoginClient(t, srv)
	if status := requestResetRequest(t, r, srv, email); status != http.StatusOK {
		t.Fatalf("reset request status = %d, want 200", status)
	}
	code := mail.codeFor(t, email)
	for i := 0; i < 5; i++ {
		a := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": wrongCodeFor(code)})
		expectError(t, "a wrong code", a, http.StatusBadRequest, "code", secWrongCodeSentence)
	}
	a := doAuth(t, r, srv, "/api/auth/reset/code", map[string]string{"code": code})
	expectError(t, "the right code after five wrong ones", a, http.StatusBadRequest, "code", secWrongCodeSentence)
	b := doAuth(t, r, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	expectError(t, "step two after a spent budget", b, http.StatusBadRequest, "code", secWrongCodeSentence)
}

// TestResetPasswordRefusesASignUpCode: a Create account code cannot open step
// two, however the browser got there.
func TestResetPasswordRefusesASignUpCode(t *testing.T) {
	srv, mail, pool := newE2EServer(t)
	const email = "reset-signup-row@example.com"
	seedLegacyAccount(t, pool, email)
	c := newLoginClient(t, srv)
	if status := signUpRequest(t, c, srv, email, e2eTestPassword); status != http.StatusOK {
		t.Fatalf("sign up status = %d, want 200", status)
	}
	a := doAuth(t, c, srv, "/api/auth/reset/password", map[string]string{"password": secNewPassword})
	expectError(t, "step two on a sign up row", a, http.StatusBadRequest, "code", secWrongCodeSentence)
	b := doAuth(t, c, srv, "/api/auth/reset/code", map[string]string{"code": mail.codeFor(t, email)})
	expectError(t, "step one on a sign up row", b, http.StatusBadRequest, "code", secWrongCodeSentence)
}
