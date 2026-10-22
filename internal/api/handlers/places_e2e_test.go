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
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): the Disclaimer and Policy
	// rows keep their plain hrefs and also carry data-open-reader, so a click
	// opens the in app reader window.
	rows := regexp.MustCompile(`<a class="place-row" href="([^"]*)"(?: data-open-reader="[^"]*")?>`).FindAllStringSubmatch(body, -1)
	var hrefs []string
	for _, m := range rows {
		hrefs = append(hrefs, m[1])
	}
	wantHrefs := []string{"/disclaimer", "/policy#policy", "/low"}
	if strings.Join(hrefs, ",") != strings.Join(wantHrefs, ",") {
		t.Errorf("About Pinalert rows go to %v, want %v", hrefs, wantHrefs)
	}
	// Amended 2026-10-17 by Phase 3.4 plan 06 (D-25, D-24b): About you renders
	// first with its two rows, which are buttons that open edit windows, so the
	// link rows above are still exactly the three About Pinalert ones.
	buttons := regexp.MustCompile(`<button type="button" class="place-row" id="([^"]*)"`).FindAllStringSubmatch(body, -1)
	var buttonIDs []string
	for _, m := range buttons {
		buttonIDs = append(buttonIDs, m[1])
	}
	if strings.Join(buttonIDs, ",") != "settings-name-row,settings-home-row" {
		t.Errorf("About you rows are %v, want the Name row then the Home area row and nothing else", buttonIDs)
	}
	for _, want := range []string{
		// Amended 2026-10-22 (owner: "too much details under the heading for
		// each section"): the lines under the section headings are gone.
		"About you",
		"Shown only if you let others follow you", "Used for alerts, never your exact spot", "Not set",
		"About Pinalert", "Disclaimer", "Policy", "Low data view", "Text only, no map",
		"Account",
		`<form class="place-row-form" method="post" action="/auth/logout">`,
		// Amended 2026-10-21 by plan 43 (owner, "I don't see a logout option
		// on my laptop version, so I think we should add that"): the Account
		// section was pinned as phone only (place-section--phone). It is now
		// drawn on both sizes, so the pin moves to the plain section tag, and
		// the phone only class is checked absent just below.
		`<section class="place-section" aria-labelledby="settings-account-title">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings lacks %q", want)
		}
	}
	if strings.Contains(body, "place-section--phone") {
		t.Errorf("/settings still marks a section phone only; the Account section with Log out is drawn on both sizes (plan 43)")
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

// TestSettingsAboutYouValues proves D-24b and T-34-26: About you is the first
// section of /settings, it shows Not set for a person with no name and no home
// area, and after the person saves both through the route of plan 01 it shows
// the stored text, escaped, in the rows, in the data-value attributes the
// windows prefill from, and with the Remove home area button visible.
func TestSettingsAboutYouValues(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "places-about-you@example.com")

	_, body := getPlacePage(t, client, srv.URL, "/settings")
	for _, want := range []string{
		"About you",
		"Shown only if you let others follow you", "Used for alerts, never your exact spot",
		`id="settings-name-value">Not set</span>`, `id="settings-home-value">Not set</span>`,
		`id="settings-name-row" aria-haspopup="dialog" data-value=""`,
		`id="settings-home-row" aria-haspopup="dialog" data-value=""`,
		`id="settings-name-window"`, `id="settings-home-window"`,
		`id="settings-home-remove" hidden>Remove home area</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings (nothing stored) lacks %q", want)
		}
	}
	if you, about := strings.Index(body, "settings-you-title"), strings.Index(body, "settings-about-title"); you == -1 || about == -1 || you > about {
		t.Errorf("About you must come before About Pinalert (you at %d, about at %d)", you, about)
	}
	if strings.Contains(body, "View my profile") {
		t.Errorf("/settings draws View my profile, which waits for Phase 9")
	}

	// Save a name and a home area with markup in them, the way the windows do.
	meWant(t, "PATCH name", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"display_name":"<b>Ravi</b> & \"Co\""}`), http.StatusOK)
	meWant(t, "PATCH home area", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"home_area":{"name":"Indiranagar <i>x</i>, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)

	_, body = getPlacePage(t, client, srv.URL, "/settings")
	for _, want := range []string{
		`id="settings-name-value">&lt;b&gt;Ravi&lt;/b&gt; &amp; &#34;Co&#34;</span>`,
		`id="settings-home-value">Indiranagar &lt;i&gt;x&lt;/i&gt;, Bengaluru</span>`,
		`data-value="&lt;b&gt;Ravi&lt;/b&gt; &amp; &#34;Co&#34;"`,
		`data-value="Indiranagar &lt;i&gt;x&lt;/i&gt;, Bengaluru"`,
		`id="settings-home-remove">Remove home area</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings (stored values) lacks %q", want)
		}
	}
	for _, banned := range []string{"<b>Ravi</b>", "<i>x</i>", `id="settings-name-value">Not set`, `id="settings-home-value">Not set`} {
		if strings.Contains(body, banned) {
			t.Errorf("/settings holds %q", banned)
		}
	}
	// Only the place name is ever drawn: no coordinate reaches the page.
	for _, banned := range []string{"12.9784", "77.6408"} {
		if strings.Contains(body, banned) {
			t.Errorf("/settings holds the coordinate %q", banned)
		}
	}
}
