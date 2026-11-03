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
// account with the filled shell (the status line, the blocks the script fills,
// the attribution and the noscript line), no claim that nothing is in force,
// Official alerts as the one current place, no notice strip, and the stylesheet
// order that ends with phone.css. A visitor without a session never gets the
// page (T-34-19).
//
// Amended 2026-10-29 by plan 05-09 (ALERT-04): the page is filled; the honesty
// sentences live in official-alerts.js and are pinned by
// web/official_alerts_test.mjs against every state. The two "not connected"
// wants are replaced by the shell, the noscript line and the attribution line;
// the title, the headings, the no-store header, the claim bans, the stylesheet
// order and the T-34-19 redirect are kept.
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
		"Turn on JavaScript to see official alerts, or check your state disaster management authority.",
		"Alerts are copied from NDMA SACHET. Pinalert is not an official channel. Hazard events come from the Global Disaster Alert and Coordination System, GDACS.",
		`id="official-status"`,
		`id="official-list"`,
		`/static/js/official-alerts.js`,
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
	official := strings.Index(body, "/static/css/official.css")
	if places == -1 || phone < places || strings.Count(body, "/static/css/phone.css") != 1 {
		t.Errorf("/alerts must link places.css and then phone.css once, last")
	}
	if official == -1 || official < places || official > phone {
		t.Errorf("/alerts must link official.css after places.css and before phone.css")
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
	// Amended 2026-10-19 by Phase 3.2 plan 12 (D-23, LAYOUT-06): the Muted reports
	// row (People and reports) and the Log out of all devices and Delete my
	// account rows (Account) are buttons that open windows too, so the button
	// rows are those five in page order.
	//
	// Amended 2026-10-19 by Phase 3.2 plan 15 (D-23, D-26, UI-SPEC E1): an
	// account with no home area gets the Notifications row "A Reliable reporter
	// posts near me / Add a home area first", a button that opens the Home area
	// window, so it sits between Home area and Muted reports. The switch and the
	// distance rows are not buttons and are hidden for this account.
	//
	// Amended 2026-10-18 by Phase 3.3 plan 14 (D-12, UI-SPEC Surface 2): the
	// Notifications box also holds the On this device install row (a button the
	// script shows on an iPhone tab, rendered hidden) and the Critical reports
	// "Add a home area first" row, both before the Reliable one.
	//
	// Amended 2026-10-29 by plan 05-14 (D-11): Phase 5 draws the official row, whose
	// no home variant is a button too, so it sits between Critical and Reliable.
	//
	// Amended 2026-10-29 by Phase 7.1 plan 15 (D-07, UI-SPEC Surface 8): the
	// Username row is the first of About you, a button that opens its window.
	//
	// Amended 2026-10-29 by Phase 9 plan 12 (D-04, owner decision 2026-10-16, UI-SPEC
	// 7a): People and reports draws Following and Followers before Muted reports and
	// Blocked people after it, each a button row that opens the list window. The
	// Let others follow me switch row is a label, and View my profile is a link, so
	// neither is in this list.
	//
	// Amended 2026-10-29 by Phase 9 plan 20 (D-12, UI-SPEC Defaults 7): the Muted
	// people button row sits between Muted reports and Blocked people. Every
	// other id is unchanged.
	//
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
	// D1, C9): the three Add a home area first rows (Critical, official, Reliable)
	// are gone. The one that is left is the distance row's button, which sits
	// after the install row, so the list holds settings-device-install and then
	// settings-radius-nohome where the three were.
	if strings.Join(buttonIDs, ",") != "settings-username-row,settings-name-row,settings-home-row,settings-device-install,settings-radius-nohome,settings-following-row,settings-followers-row,settings-muted-row,settings-muted-people-row,settings-blocked-row,settings-logout-all-row,settings-delete-row" {
		t.Errorf("Settings button rows are %v, want Username, Name, Home area, the Alerts on this device install row, the distance row's Add a home area first, Following, Followers, Muted reports, Muted people, Blocked people, Log out of all devices, Delete my account", buttonIDs)
	}
	for _, want := range []string{
		// Amended 2026-10-22 (owner: "too much details under the heading for
		// each section"): the lines under the section headings are gone.
		"About you",
		"Shown only if you let others follow you", "Used for alerts, never your exact spot", "Not set",
		"About Pinalert", "Disclaimer", "Policy", "Low data view", "Text only, no map",
		// Amended 2026-10-21 when Phase 3.4 plan 43 met Phase 3.2: the owner put
		// Log out on the laptop too ("I don't see a logout option on my laptop
		// version, so I think we should add that"), so the Log out form carries
		// no phone only class and the Account section is pinned by its plain tag.
		//
		// Amended 2026-10-22 when Phase 3.4 met Phase 3.2: with the owner's
		// sketch 008 laptop picture each section has its line again, shown on
		// the laptop only. The Account line is the sketch's; the Notifications
		// and People and reports lines stay the 3.2 ones.
		"Account", "Sign out, or remove your account and its data.",
		// Amended 2026-10-18 by Phase 3.3 plan 14 (UI-SPEC N1): the Notifications
		// line is "Choose what Pinalert tells you about."
		//
		// Amended 2026-10-29 by Phase 9 plan 12 (D-04, UI-SPEC P9-C17): the People and
		// reports line names the public profile, in place of "Reports you have turned
		// updates off for.".
		"Choose what Pinalert tells you about.", "Your public profile and the lists that go with it.",
		`<form class="place-row-form" method="post" action="/auth/logout">`,
		`<section class="place-section" aria-labelledby="settings-account-title">`,
		"People and reports", "Muted reports", "Log out of all devices", "Delete my account",
		// Amended 2026-10-19 by Phase 3.2 plan 15: the Notifications section.
		// Amended 2026-11-03 by quick 261103-jal (D1): the Reliable label left the box
		// and the device row reads Alerts on this device.
		`id="settings-phone-title">Notifications</h2>`, "Alerts on this device", "Add a home area first",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings lacks %q", want)
		}
	}
	if strings.Contains(body, "place-section--phone") {
		t.Errorf("/settings still marks a section phone only; the Account section with Log out is drawn on both sizes (plan 43)")
	}
	// Amended 2026-10-29 by Phase 9 plan 12 (D-04, owner decision 2026-10-16): View
	// my profile, My profile and Let others follow me are drawn now (see
	// TestSettingsPeopleRows), so they left this list. The rest still holds.
	for _, banned := range []string{
		"Phone notifications", "place-section--phone", email, "disclaimer-bar",
	} {
		if strings.Contains(body, banned) {
			t.Errorf("/settings holds %q", banned)
		}
	}
}

// jalBoxChildren splits the inside of the Notifications box into its direct
// children, all of which must be divs: the text outside them must be blank.
func jalBoxChildren(t *testing.T, inner string) []string {
	t.Helper()
	tag := regexp.MustCompile(`</?div\b[^>]*>`)
	var tops []string
	var outside strings.Builder
	depth, start, pos := 0, 0, 0
	for _, loc := range tag.FindAllStringIndex(inner, -1) {
		if strings.HasPrefix(inner[loc[0]:loc[1]], "</div") {
			depth--
			if depth == 0 {
				tops = append(tops, inner[start:loc[1]])
				pos = loc[1]
			}
			continue
		}
		if depth == 0 {
			outside.WriteString(inner[pos:loc[0]])
			start = loc[0]
		}
		depth++
	}
	outside.WriteString(inner[pos:])
	if strings.TrimSpace(outside.String()) != "" {
		t.Errorf("the Notifications box holds markup outside its two wrappers: %q", strings.TrimSpace(outside.String()))
	}
	return tops
}

// TestSettingsNotificationsTwoRows is D9 (e) of quick 261103-jal (owner decision
// 2026-11-03): /settings renders the Notifications box with exactly two direct
// children, the device row wrapper and the distance wrapper. The device switch
// row reads Alerts on this device with the one sub line, the install, blocked
// and unsupported variants read Alerts on this device with their own sub lines,
// and the select shows with a home area while the Add a home area first button
// shows without one. The four removed rows are nowhere on the page.
func TestSettingsNotificationsTwoRows(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)

	check := func(t *testing.T, body string, hasHome bool) {
		t.Helper()
		const open = `<div class="place-box" id="settings-alerts-box">`
		start := strings.Index(body, open)
		end := strings.Index(body, `<p class="inline-error" id="settings-device-error"`)
		if start == -1 || end == -1 || end < start {
			t.Fatalf("the Notifications box or its error line is missing (box at %d, error line at %d)", start, end)
		}
		inner := strings.TrimSpace(body[start+len(open) : end])
		inner = strings.TrimSpace(strings.TrimSuffix(inner, "</div>"))
		tops := jalBoxChildren(t, inner)
		if len(tops) != 2 {
			t.Fatalf("the Notifications box has %d direct children, want 2", len(tops))
		}
		if !strings.HasPrefix(tops[0], `<div id="settings-device-row">`) || !strings.HasPrefix(tops[1], `<div id="settings-radius-wrap">`) {
			t.Errorf("the two wrappers are %q and %q, want the device row and the distance wrapper", tops[0][:40], tops[1][:40])
		}

		device := tops[0]
		if n := strings.Count(device, `<span class="place-row__label">Alerts on this device</span>`); n != 4 {
			t.Errorf("the device row carries %d Alerts on this device labels, want 4 (switch, install, blocked, unsupported)", n)
		}
		const sub = `<span class="place-row__sub">Critical reports and severe official alerts near your home area, and posts by people you follow.</span>`
		if n := strings.Count(device, sub); n != 1 {
			t.Errorf("the device switch row carries the sub line %d times, want once", n)
		}
		switchRow := device[strings.Index(device, `id="settings-device-on"`):strings.Index(device, `id="settings-device-install"`)]
		if !strings.Contains(switchRow, sub) || !strings.Contains(switchRow, `id="settings-device-switch"`) {
			t.Errorf("the sub line is not on the device switch row")
		}
		for _, want := range []string{
			"Add Pinalert to your Home Screen first",
			"Blocked in your browser settings. Allow notifications for Pinalert there, then come back.",
			"This browser can&#39;t show notifications. Updates still show in the bell.",
		} {
			if !strings.Contains(device, want) && !strings.Contains(device, strings.ReplaceAll(want, "&#39;", "'")) {
				t.Errorf("the device row lacks the variant sub line %q", want)
			}
		}
		// The device guide lead "On this device, alerts work once Pinalert is on
		// your Home Screen." is not the row label and stays.
		if strings.Contains(body, `<span class="place-row__label">On this device</span>`) {
			t.Errorf("/settings still holds the old row label On this device")
		}

		radius := tops[1]
		selectRow := regexp.MustCompile(`<div class="place-row" id="settings-radius-row"( hidden)?>`).FindStringSubmatch(radius)
		noHome := regexp.MustCompile(`<button type="button" class="place-row" id="settings-radius-nohome" aria-haspopup="dialog"( hidden)?>`).FindStringSubmatch(radius)
		if selectRow == nil || noHome == nil {
			t.Fatalf("the distance wrapper lacks its select row or its Add a home area first button: %s", radius)
		}
		if shown := selectRow[1] == ""; shown != hasHome {
			t.Errorf("select row shown = %v, want %v (home area %v)", shown, hasHome, hasHome)
		}
		if shown := noHome[1] == ""; shown == hasHome {
			t.Errorf("Add a home area first button shown = %v, want %v (home area %v)", shown, !hasHome, hasHome)
		}
		if n := strings.Count(radius, `How far from my home area`); n != 2 {
			t.Errorf("the distance wrapper carries the label %d times, want 2 (select row and button)", n)
		}
		if !strings.Contains(radius, `<span class="place-row__sub">Add a home area first</span>`) {
			t.Errorf("the distance button lacks its Add a home area first sub line")
		}

		for _, gone := range []string{
			"settings-critical", "settings-official", "settings-changes", "settings-reliable",
			"Critical reports near my home area", "Severe official alerts for my area",
			"Changes and comments on reports I posted", "A Reliable reporter posts near me",
		} {
			if strings.Contains(body, gone) {
				t.Errorf("/settings still holds the removed row text %q", gone)
			}
		}
	}

	t.Run("an account with no home area", func(t *testing.T) {
		client := newVerifiedClient(t, srv, mailer, "places-two-rows-nohome@example.com")
		_, body := getPlacePage(t, client, srv.URL, "/settings")
		check(t, body, false)
	})
	t.Run("an account with a home area", func(t *testing.T) {
		client := newVerifiedClient(t, srv, mailer, "places-two-rows-home@example.com")
		meWant(t, "set home area", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
			`{"home_area":{"name":"Indiranagar, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)
		_, body := getPlacePage(t, client, srv.URL, "/settings")
		check(t, body, true)
	})
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
	// The person menu on a laptop holds My profile, Settings and Log out.
	//
	// Amended 2026-10-29 by Phase 9 plan 12 (D-04, owner decision 2026-10-16, UI-SPEC
	// 7c): My profile is the first item, so the text starts with it.
	menu := p421Text(p421Region(t, body, `<div id="account-menu"`))
	if menu != "My profile Settings Log out" {
		t.Errorf("person menu text = %q, want %q", menu, "My profile Settings Log out")
	}
}

// TestSettingsAboutYouValues proves D-24b and T-34-26: About you is the first
// section of /settings, it shows Not set for a person with no name and no home
// area, and after the person saves both through the route of plan 01 it shows
// the stored text, escaped, in the rows, in the data-value attributes the
// windows prefill from. Amended 2026-10-29 by Phase 7.1 plan 15 (D-07, D-15):
// the Username row is the first of About you, and there is no Remove home area
// button, stored or not.
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
		`id="settings-username-window"`,
		`id="settings-username-row" aria-haspopup="dialog" data-value=""`,
		`id="settings-username-value">Not set</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/settings (nothing stored) lacks %q", want)
		}
	}
	if strings.Contains(body, "settings-home-remove") || strings.Contains(body, "Remove home area") {
		t.Errorf("/settings (nothing stored) draws a Remove home area button")
	}
	if you, about := strings.Index(body, "settings-you-title"), strings.Index(body, "settings-about-title"); you == -1 || about == -1 || you > about {
		t.Errorf("About you must come before About Pinalert (you at %d, about at %d)", you, about)
	}
	// Amended 2026-10-29 by Phase 9 plan 12 (D-04, UI-SPEC 7c): View my profile is
	// the phone row of About you, a link to /me, so the old ban is replaced by the
	// check that it is there and comes after Home area.
	if home, view := strings.Index(body, `id="settings-home-row"`), strings.Index(body, `id="settings-view-profile-row" href="/me"`); home == -1 || view == -1 || view < home {
		t.Errorf("/settings must draw View my profile after Home area (home at %d, view at %d)", home, view)
	}

	// Save a name and a home area with markup in them, the way the windows do.
	meWant(t, "PATCH name", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"display_name":"<b>Ravi</b> & \"Co\""}`), http.StatusOK)
	meWant(t, "PATCH username", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"username":"about_you"}`), http.StatusOK)
	meWant(t, "PATCH home area", meSend(t, client, http.MethodPatch, srv.URL+"/api/me/profile",
		`{"home_area":{"name":"Indiranagar <i>x</i>, Bengaluru","lat":12.9784,"lon":77.6408}}`), http.StatusOK)

	_, body = getPlacePage(t, client, srv.URL, "/settings")
	for _, want := range []string{
		`id="settings-name-value">&lt;b&gt;Ravi&lt;/b&gt; &amp; &#34;Co&#34;</span>`,
		`id="settings-home-value">Indiranagar &lt;i&gt;x&lt;/i&gt;, Bengaluru</span>`,
		`data-value="&lt;b&gt;Ravi&lt;/b&gt; &amp; &#34;Co&#34;"`,
		`data-value="Indiranagar &lt;i&gt;x&lt;/i&gt;, Bengaluru"`,
		`id="settings-username-value">about_you</span>`,
		`id="settings-username-row" aria-haspopup="dialog" data-value="about_you"`,
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
