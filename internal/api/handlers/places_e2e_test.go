// places_e2e_test.go: Phase 3.4 plan 04. The Official alerts place and the one
// Settings page, rendered through the real router over a real Postgres. They
// need the disposable database the e2e harness opens (testutil.NewTestDB); never
// run them on the owner's data.
package handlers_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// getPlacePage GETs path with client and returns the response and its body.
func getPlacePage(t *testing.T, client *http.Client, baseURL, path string) (*http.Response, string) {
	t.Helper()
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s body: %v", path, err)
	}
	return resp, string(b)
}

// TestAlertsPageRenders proves D-22 and T-34-17: /alerts renders for a verified
// account with the not connected heading and body, no claim that nothing is in
// force, Official alerts as the one current place, no notice strip, and the
// stylesheet order that ends with phone.css. A visitor without a session never
// gets the page (T-34-19).
func TestAlertsPageRenders(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "places-alerts@example.com")

	resp, body := getPlacePage(t, client, srv.URL, "/alerts")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /alerts status = %d, want 200: %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	for _, want := range []string{
		"<title>Pinalert: Official alerts</title>",
		"Official alerts are not connected yet",
		"Pinalert does not show government warnings yet. Check your state disaster management authority for official warnings.",
		`<h1 class="place-head">Official alerts</h1>`,
		`<h1 class="app-topbar__title">Official alerts</h1>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/alerts lacks %q", want)
		}
	}
	lower := strings.ToLower(body)
	for _, banned := range []string{"no" + " alerts", "no" + " warnings", "all" + " clear", "in" + " force", "disclaimer-bar"} {
		if strings.Contains(lower, banned) {
			t.Errorf("/alerts holds %q", banned)
		}
	}

	top, bottom := appBarLinks(t, body)
	for name, links := range map[string][]appBarLink{"top bar": top, "bottom bar": bottom} {
		var current []string
		for _, l := range links {
			if l.current {
				current = append(current, l.label)
			}
		}
		if len(current) != 1 || current[0] != "Official alerts" {
			t.Errorf("/alerts %s: current places are %v, want only Official alerts", name, current)
		}
	}
	if !strings.Contains(body, `href="/settings" aria-label="Settings"`) {
		t.Errorf("/alerts: the phone top bar has no Settings gear")
	}

	phone := strings.LastIndex(body, "/static/css/phone.css")
	places := strings.Index(body, "/static/css/places.css")
	if places == -1 || phone < places || strings.Count(body, "/static/css/phone.css") != 1 {
		t.Errorf("/alerts must link places.css and then phone.css once, last")
	}

	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, path := range []string{"/alerts", "/settings"} {
		resp, _ := getPlacePage(t, anon, srv.URL, path)
		if resp.StatusCode == http.StatusOK {
			t.Errorf("GET %s without a session answered 200", path)
		}
	}
}

// TestSettingsRows proves D-25 and D-26: /settings renders the three About
// Pinalert rows with their links, the phone Account section with Log out, and
// none of the rows that wait for a later phase. It also proves the signed in
// email appears nowhere on the page (T-34-18).
func TestSettingsRows(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	const email = "places-settings@example.com"
	client := newVerifiedClient(t, srv, mailer, email)

	resp, body := getPlacePage(t, client, srv.URL, "/settings")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /settings status = %d, want 200: %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	rows := regexp.MustCompile(`<a class="place-row" href="([^"]*)">`).FindAllStringSubmatch(body, -1)
	var hrefs []string
	for _, m := range rows {
		hrefs = append(hrefs, m[1])
	}
	wantHrefs := []string{"/disclaimer", "/policy#policy", "/low"}
	if strings.Join(hrefs, ",") != strings.Join(wantHrefs, ",") {
		t.Errorf("About Pinalert rows go to %v, want %v", hrefs, wantHrefs)
	}
	for _, want := range []string{
		"About Pinalert", "Disclaimer", "Policy", "Low data view", "Text only, no map",
		"Account", "Sign out, or remove your account and its data.",
		`<form class="place-row-form" method="post" action="/auth/logout">`,
		"place-section--phone",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings lacks %q", want)
		}
	}
	for _, banned := range []string{
		"View my profile", "My profile", "Phone notifications", "People and reports",
		"Log out of all devices", "Delete my account", "Let others follow me", email, "disclaimer-bar",
	} {
		if strings.Contains(body, banned) {
			t.Errorf("/settings holds %q", banned)
		}
	}
}

// TestSettingsBackAndNoGear proves D-11: on Settings the phone top bar has a
// back arrow whose address is the constant "/", the title Settings and the
// bell, no Settings gear, and no main place is current.
func TestSettingsBackAndNoGear(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "places-settings-back@example.com")

	_, body := getPlacePage(t, client, srv.URL, "/settings")
	for _, want := range []string{
		`<a class="app-topbar__back" href="/" aria-label="Back">`,
		`<h1 class="app-topbar__title">Settings</h1>`,
		`id="bell-trigger"`,
		`<script src="/static/js/app-bar.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings lacks %q", want)
		}
	}
	if strings.Contains(body, `class="app-topbar__gear"`) {
		t.Errorf("/settings draws the Settings gear, which is hidden on the Settings screen")
	}
	top, bottom := appBarLinks(t, body)
	for name, links := range map[string][]appBarLink{"top bar": top, "bottom bar": bottom} {
		for _, l := range links {
			if l.current {
				t.Errorf("/settings %s marks %q current, want no main place current", name, l.label)
			}
		}
	}
	// The person menu on a laptop holds Settings and Log out only.
	menu := p421Text(p421Region(t, body, `<div id="account-menu"`))
	if menu != "Settings Log out" {
		t.Errorf("person menu text = %q, want %q", menu, "Settings Log out")
	}
}
