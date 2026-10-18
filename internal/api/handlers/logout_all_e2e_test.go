// Package handlers_test covers Phase 3.2 plan 16 end to end: POST
// /api/me/logout-all ends every session of the calling account on every device,
// leaves authorship and every count on other people's screens as they were, and
// the sign in page shows one status line from a closed list.
package handlers_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// logoutAllPost POSTs body to /api/me/logout-all as client and returns the
// status, the Set-Cookie cookies, the Cache-Control header and the body.
func logoutAllPost(t *testing.T, client *http.Client, baseURL, body string) (int, []*http.Cookie, string, string) {
	t.Helper()
	resp, err := client.Post(baseURL+"/api/me/logout-all", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/me/logout-all: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the logout-all response: %v", err)
	}
	return resp.StatusCode, resp.Cookies(), resp.Header.Get("Cache-Control"), string(b)
}

// TestLogoutAllRevokesEverySession: A and a copy of A's cookie both work, then
// A posts the route. The answer is 204 with no-store and a cleared cookie; both
// browsers are refused afterwards (401 on the API, 302 on pages); a third
// account is unaffected; an unknown body field is a 400 that ends nothing.
func TestLogoutAllRevokesEverySession(t *testing.T) {
	srv, mailer, _ := newRevServer(t)
	a := newVerifiedClient(t, srv, mailer, "lo-a@example.com")
	aCopy := revCookieCopy(t, a, srv)
	other := newVerifiedClient(t, srv, mailer, "lo-other@example.com")

	revExpect(t, "A before", a, srv, http.StatusOK, http.StatusOK)
	revExpect(t, "A copy before", aCopy, srv, http.StatusOK, http.StatusOK)

	// A refused body ends nothing.
	for _, bad := range []string{`{"extra": 1}`, `not json`} {
		if status, _, _, got := logoutAllPost(t, a, srv.URL, bad); status != http.StatusBadRequest {
			t.Errorf("body %q: status %d, want 400: %s", bad, status, got)
		}
	}
	revExpect(t, "A after refused bodies", a, srv, http.StatusOK, http.StatusOK)

	status, cookies, cacheControl, body := logoutAllPost(t, a, srv.URL, `{}`)
	if status != http.StatusNoContent {
		t.Fatalf("POST /api/me/logout-all = %d, want 204: %s", status, body)
	}
	if body != "" {
		t.Errorf("the 204 carried a body: %q", body)
	}
	if !strings.Contains(cacheControl, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cacheControl)
	}
	cleared := false
	for _, ck := range cookies {
		if ck.Name == "pinalert_session" && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("the response did not clear the session cookie: %v", cookies)
	}

	revExpect(t, "A after", a, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "A copy after", aCopy, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "other after", other, srv, http.StatusOK, http.StatusOK)
}

// TestLogoutAllEndsASecondSignIn: the same address signed in on a second
// browser is ended too, not only the browser that pressed the button.
func TestLogoutAllEndsASecondSignIn(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	first := newVerifiedClient(t, srv, mailer, "lo-two-a@example.com")
	second := newJarClient(t)
	if status, body, _ := revSignIn(t, second, srv, mailer, pool, "lo-two-a@example.com"); status != http.StatusOK || !strings.Contains(body, "Email verified") {
		t.Fatalf("second browser sign in: status %d: %s", status, body)
	}
	revExpect(t, "second before", second, srv, http.StatusOK, http.StatusOK)

	if status, _, _, body := logoutAllPost(t, first, srv.URL, ``); status != http.StatusNoContent {
		t.Fatalf("POST /api/me/logout-all with no body = %d, want 204: %s", status, body)
	}
	revExpect(t, "first after", first, srv, http.StatusUnauthorized, http.StatusFound)
	revExpect(t, "second after", second, srv, http.StatusUnauthorized, http.StatusFound)
}

// TestLogoutAllKeepsAuthorship: A filed a report and B confirmed it. After A's
// logout-all another reader's JSON for that report is byte for byte the same,
// and every session row of A still names A.
func TestLogoutAllKeepsAuthorship(t *testing.T) {
	srv, mailer, pool := newRevServer(t)
	a := newVerifiedClient(t, srv, mailer, "lo-auth-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "lo-auth-b@example.com")
	reader := newVerifiedClient(t, srv, mailer, "lo-auth-reader@example.com")
	aID := revAccountID(t, pool, "lo-auth-a@example.com")

	id := revPost(t, a, srv)
	resp := postVote(t, b, srv.URL, id, "confirm", map[string]any{"latitude": revLat, "longitude": revLon})
	decodeVoteResponse(t, resp) // fails the test unless B's confirm answered 200

	before := revReportJSON(t, reader, srv, id)
	if status, _, _, body := logoutAllPost(t, a, srv.URL, `{}`); status != http.StatusNoContent {
		t.Fatalf("POST /api/me/logout-all = %d, want 204: %s", status, body)
	}
	after := revReportJSON(t, reader, srv, id)
	if !bytes.Equal(before, after) {
		t.Errorf("the report changed for another reader after A logged out of all devices:\nbefore %s\nafter  %s", before, after)
	}

	var bound, revoked, unbound int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*), count(*) FILTER (WHERE revoked_at IS NOT NULL) FROM sessions WHERE account_id = $1`, aID,
	).Scan(&bound, &revoked); err != nil {
		t.Fatalf("counting A's sessions: %v", err)
	}
	if bound == 0 || bound != revoked {
		t.Errorf("A's sessions: %d still name A, %d revoked; want every one kept and revoked", bound, revoked)
	}
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM sessions WHERE revoked_at IS NOT NULL AND account_id IS NULL`).Scan(&unbound); err != nil {
		t.Fatalf("counting unbound revoked sessions: %v", err)
	}
	if unbound != 0 {
		t.Errorf("%d revoked sessions lost their account", unbound)
	}
}

// TestLogoutAllNeedsAnAccount: a caller with no verified session gets the
// gate's 401 JSON and nothing is revoked.
func TestLogoutAllNeedsAnAccount(t *testing.T) {
	srv, mailer, _ := newRevServer(t)
	other := newVerifiedClient(t, srv, mailer, "lo-need-other@example.com")

	anon := newJarClient(t)
	status, _, _, body := logoutAllPost(t, anon, srv.URL, `{}`)
	if status != http.StatusUnauthorized {
		t.Errorf("an anonymous caller got %d, want 401: %s", status, body)
	}
	if !strings.Contains(body, "{") {
		t.Errorf("the 401 body is not JSON: %q", body)
	}
	revExpect(t, "other after an anonymous call", other, srv, http.StatusOK, http.StatusOK)
}

// loginBody GETs path as an anonymous browser and returns the status and body.
func loginBody(t *testing.T, srv string, path string) (int, string) {
	t.Helper()
	resp, err := newJarClient(t).Get(srv + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return resp.StatusCode, string(b)
}

// TestLoginDoneLineIsClosedList: the sign in page shows the line for exactly
// the two done words, inside role=status, and never echoes any other value.
func TestLoginDoneLineIsClosedList(t *testing.T) {
	srv, _, _ := newRevServer(t)

	for _, tc := range []struct{ path, line string }{
		{"/login?done=all-devices", "You are logged out of all devices."},
		{"/login?done=deleted", "Your account has been deleted."},
	} {
		status, body := loginBody(t, srv.URL, tc.path)
		if status != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.path, status)
			continue
		}
		want := `<p class="login-done" role="status">` + tc.line + `</p>`
		if !strings.Contains(body, want) {
			t.Errorf("GET %s: body does not contain %q", tc.path, want)
		}
	}

	// Every other value, including markup, shows no line and is never echoed.
	for _, path := range []string{
		"/login",
		"/login?done=",
		"/login?done=%3Cb%3Ex",
		"/login?done=ALL-DEVICES",
		"/login?done=all-devices%20",
		"/login?done=deleted&done=%3Cb%3Ex",
	} {
		status, body := loginBody(t, srv.URL, path)
		if status != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, status)
			continue
		}
		wantLine := path == "/login?done=deleted&done=%3Cb%3Ex" // Query().Get reads the first value
		if has := strings.Contains(body, `class="login-done"`); has != wantLine {
			t.Errorf("GET %s: login-done line present = %v, want %v", path, has, wantLine)
		}
		for _, raw := range []string{"<b>x", "&lt;b&gt;x", "%3Cb%3Ex", "ALL-DEVICES"} {
			if strings.Contains(body, raw) {
				t.Errorf("GET %s: the page echoes the done value %q", path, raw)
			}
		}
	}
}
