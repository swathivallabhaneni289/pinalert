// settings_username_e2e_test.go: Quick 261108-fku (owner decision 2026-11-08).
// Amended 2026-11-13 by quick 261113-t4y (owner decision 2026-11-13, the compact
// profile): the Username window moved from the Settings page to your own profile
// page (/u/yourname, behind Edit profile), so the page read here is that one.
// The profile page carries the moment the signed in account's 7 day rename wait
// ends (data-wait-until on the Username window), so the window can follow the
// real deadline and open read only with one OK button. It runs through the real
// router over a real Postgres (the disposable database the e2e harness opens).
//
// Amended 2026-11-08 (review fix WR-02): the page carries the end of the wait as
// an RFC 3339 UTC time in place of the data-wait-message and data-wait-after-change
// sentences, so TestSettingsUsernameWaitLine became TestSettingsUsernameWaitUntil.
package handlers_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// fkuWindowTag returns the opening tag of the Username window.
func fkuWindowTag(t *testing.T, body string) string {
	t.Helper()
	tag := regexp.MustCompile(`<div[^>]*id="settings-username-window"[^>]*>`).FindString(body)
	if tag == "" {
		t.Fatalf("the profile page has no Username window opening tag")
	}
	return tag
}

// fkuWaitUntil returns the data-wait-until value of the window, and whether the
// attribute is there at all.
func fkuWaitUntil(t *testing.T, body string) (string, bool) {
	t.Helper()
	m := regexp.MustCompile(`data-wait-until="([^"]*)"`).FindStringSubmatch(fkuWindowTag(t, body))
	if m == nil {
		return "", false
	}
	return m[1], true
}

// TestSettingsUsernameWaitUntil: a first username starts no wait, so the window
// carries no data-wait-until; a changed username puts the end of the wait on it,
// which is the stored change time plus the 7 day gap as RFC 3339 UTC rounded up
// to the second; the value follows the stored time; and the attribute is gone
// once the gap ends. The old sentence attributes are not drawn any more.
func TestSettingsUsernameWaitUntil(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	email := "fku-wait-line@example.com"
	client := newVerifiedClient(t, srv, mailer, email)
	// current is the username the page is read under; an account with no username has
	// no profile page, so the first read comes after the first username is saved.
	current := ""

	// wantUntil reads the stored change time and checks the page against it.
	wantUntil := func(label string) {
		t.Helper()
		_, body := getPlacePage(t, client, srv.URL, "/u/"+current)
		got, ok := fkuWaitUntil(t, body)
		if !ok {
			t.Errorf("%s: the window lacks data-wait-until: %s", label, fkuWindowTag(t, body))
			return
		}
		var changed time.Time
		if err := pool.QueryRow(context.Background(), `SELECT username_changed_at FROM accounts WHERE email = $1`, email).Scan(&changed); err != nil {
			t.Fatalf("%s: reading username_changed_at: %v", label, err)
		}
		want := changed.Add(service.UsernameRenameGap).UTC()
		if want.Nanosecond() != 0 {
			want = want.Truncate(time.Second).Add(time.Second)
		}
		if got != want.Format(time.RFC3339) {
			t.Errorf("%s: data-wait-until = %q, want %q", label, got, want.Format(time.RFC3339))
		}
		if !strings.HasSuffix(got, "Z") {
			t.Errorf("%s: data-wait-until %q is not UTC", label, got)
		}
	}
	noWait := func(label string) {
		t.Helper()
		_, body := getPlacePage(t, client, srv.URL, "/u/"+current)
		if tag := fkuWindowTag(t, body); strings.Contains(tag, "data-wait") {
			t.Errorf("%s: the window carries a wait: %s", label, tag)
		}
	}

	// A first username is free and starts no wait.
	meWant(t, "PATCH first username", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"username":"wait_one"}`), http.StatusOK)
	current = "wait_one"
	noWait("a first username")

	// A different username is a change, and the wait starts.
	meWant(t, "PATCH second username", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"username":"wait_two"}`), http.StatusOK)
	current = "wait_two"
	wantUntil("after a change")
	_, body := getPlacePage(t, client, srv.URL, "/u/"+current)
	if tag := fkuWindowTag(t, body); strings.Contains(tag, "data-wait-message") || strings.Contains(tag, "data-wait-after-change") {
		t.Errorf("the window still carries the old sentence attributes: %s", tag)
	}

	// The value follows the stored time: 6 days and an hour ago.
	testutil.MustExec(t, pool, `UPDATE accounts SET username_changed_at = now() - interval '6 days 1 hour' WHERE email = $1`, email)
	wantUntil("6 days and an hour after the change")

	// Past the gap the attribute is gone.
	testutil.MustExec(t, pool, `UPDATE accounts SET username_changed_at = now() - interval '7 days 1 minute' WHERE email = $1`, email)
	noWait("after the gap")
}
