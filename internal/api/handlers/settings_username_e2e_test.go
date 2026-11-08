// settings_username_e2e_test.go: Quick 261108-fku (owner decision 2026-11-08).
// The Settings page carries the server's rename wait sentence on the Username
// window while the signed in account is inside the 7 day gap, so the window can
// open read only with one OK button. It runs through the real router over a real
// Postgres (the disposable database the e2e harness opens).
package handlers_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"pinalert/internal/testutil"
)

// fkuWindowTag returns the opening tag of the Username window.
func fkuWindowTag(t *testing.T, body string) string {
	t.Helper()
	tag := regexp.MustCompile(`<div[^>]*id="settings-username-window"[^>]*>`).FindString(body)
	if tag == "" {
		t.Fatalf("the Settings page has no Username window opening tag")
	}
	return tag
}

// TestSettingsUsernameWaitLine: a first username starts no wait (no
// data-wait-message) and the window always carries data-wait-after-change with
// the whole gap sentence; a changed username puts the 7 days sentence on the
// window; the day count follows the clock; the attribute goes once the gap ends.
func TestSettingsUsernameWaitLine(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	email := "fku-wait-line@example.com"
	client := newVerifiedClient(t, srv, mailer, email)
	const seven = "You can change your username again in 7 days."

	_, body := getPlacePage(t, client, srv.URL, "/settings")
	tag := fkuWindowTag(t, body)
	if strings.Contains(tag, "data-wait-message") {
		t.Errorf("an account with no username has a wait on the window: %s", tag)
	}
	if !strings.Contains(tag, `data-wait-after-change="`+seven+`"`) {
		t.Errorf("the window lacks data-wait-after-change with the 7 days sentence: %s", tag)
	}

	// A first username is free and starts no wait.
	meWant(t, "PATCH first username", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"username":"wait_one"}`), http.StatusOK)
	_, body = getPlacePage(t, client, srv.URL, "/settings")
	if tag = fkuWindowTag(t, body); strings.Contains(tag, "data-wait-message") {
		t.Errorf("a first username started a wait: %s", tag)
	}

	// A different username is a change, and the wait starts.
	meWant(t, "PATCH second username", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"username":"wait_two"}`), http.StatusOK)
	_, body = getPlacePage(t, client, srv.URL, "/settings")
	if tag = fkuWindowTag(t, body); !strings.Contains(tag, `data-wait-message="`+seven+`"`) {
		t.Errorf("after a change the window lacks the 7 days wait sentence: %s", tag)
	}

	// The day count follows the stored time: 6 days and an hour ago is 1 day left.
	testutil.MustExec(t, pool, `UPDATE accounts SET username_changed_at = now() - interval '6 days 1 hour' WHERE email = $1`, email)
	_, body = getPlacePage(t, client, srv.URL, "/settings")
	if tag = fkuWindowTag(t, body); !strings.Contains(tag, `data-wait-message="You can change your username again in 1 day."`) {
		t.Errorf("6 days and an hour after the change the window lacks the 1 day sentence: %s", tag)
	}

	// Past the gap the attribute is gone.
	testutil.MustExec(t, pool, `UPDATE accounts SET username_changed_at = now() - interval '7 days 1 minute' WHERE email = $1`, email)
	_, body = getPlacePage(t, client, srv.URL, "/settings")
	if tag = fkuWindowTag(t, body); strings.Contains(tag, "data-wait-message") {
		t.Errorf("after the gap the window still carries a wait: %s", tag)
	}
}
