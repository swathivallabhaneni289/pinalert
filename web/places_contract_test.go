package web

// Phase 3.4 plan 04: the person menu, the bell, Official alerts and Settings.
// These tests read the shipped templates and scripts from the embedded trees,
// never a copy, so they can only drift with the product.

import (
	"strings"
	"testing"
)

// TestPersonMenuItems pins D-11, D-02 and D-26: the laptop person menu holds
// My profile, Settings and Log out, in that order, shows no email, and links
// nowhere that has its own place (Settings holds the Disclaimer, Policy and Low
// data view rows).
//
// Amended 2026-10-29 by Phase 9 plan 12 (D-04, owner decision 2026-10-16, UI-SPEC
// 7c): the menu holds three items, My profile first (a link to /me with the user
// icon). The note in this test that waited for Phase 9 is met. href="/profile"
// stays banned: that address is My activity, not the profile page. Every other
// check is as it was.
func TestPersonMenuItems(t *testing.T) {
	header := p421ReadTemplate(t, "account_header.html.tmpl")
	if got := strings.Count(header, `role="menuitem"`); got != 3 {
		t.Errorf("the person menu has %d items, want exactly 3 (My profile, Settings and Log out)", got)
	}
	p421InOrder(t, "the person menu items", header, `href="/me"`, "My profile", `href="/settings"`, "Settings", `action="/auth/logout"`, "Log out")
	for _, want := range []string{`href="/me"`, "My profile", `auth-icon--user`, `href="/settings"`, "Settings", `action="/auth/logout"`, "Log out", `auth-icon--settings`, `auth-icon--log-out`} {
		if !strings.Contains(header, want) {
			t.Errorf("the person menu lacks %q", want)
		}
	}
	for _, banned := range []string{
		"{{.Email}}", "{{if .Chrome.LowDataPage}}", `href="/profile"`, `href="/low"`,
		`href="/disclaimer"`, `href="/policy"`, `href="/?full=1"`,
	} {
		if strings.Contains(header, banned) {
			t.Errorf("the person menu still holds %q", banned)
		}
	}
}

// TestBellListContract pins D-32 and UI-SPEC Surface 9: the bell is a button
// named Updates with a hidden count span that Phase 3.2 fills, it opens a dialog
// list that says there are no updates yet and links to the Updates part of My
// activity, and the script that runs it builds no markup and makes no request.
func TestBellListContract(t *testing.T) {
	bar := p421ReadTemplate(t, "app_bar.html.tmpl")
	trigger := bellTagWith(t, bar, `id="bell-trigger"`)
	for _, want := range []string{
		`aria-label="Updates"`, `aria-haspopup="dialog"`, `aria-expanded="false"`, `aria-controls="bell-list"`,
		`class="app-topbar__bell"`, `type="button"`,
	} {
		if !strings.Contains(trigger, want) {
			t.Errorf("the bell button %s lacks %s", trigger, want)
		}
	}
	list := bellTagWith(t, bar, `id="bell-list"`)
	for _, want := range []string{`role="dialog"`, `aria-label="Updates"`, ` hidden`} {
		if !strings.Contains(list, want) {
			t.Errorf("the bell list %s lacks %s", list, want)
		}
	}
	p421InOrder(t, "bell list order", bar,
		`id="bell-list"`, "Updates", `aria-label="Close"`, "No updates yet", `href="/profile?part=updates"`, "See all updates")
	if strings.Count(bar, "No updates yet") != 1 || strings.Count(bar, "See all updates") != 1 {
		t.Errorf("the bell list words must each appear once")
	}
	// Amended 2026-10-17 by Phase 3.2 plan 08 (late amendment, red count;
	// orchestrator decision 3): Phase 3.2 adds the count. The bell holds one
	// hidden count span the script fills, and its poll interval as a literal.
	if !strings.Contains(trigger, `data-poll-interval-ms="30000"`) {
		t.Errorf("the bell button %s lacks data-poll-interval-ms=\"30000\"", trigger)
	}
	countTag := bellTagWith(t, bar, `id="bell-count"`)
	for _, want := range []string{`id="bell-count"`, `aria-hidden="true"`, ` hidden>`} {
		if !strings.Contains(countTag, want) {
			t.Errorf("the bell count %s lacks %s", countTag, want)
		}
	}
	p421InOrder(t, "the count sits inside the bell button", bar, `id="bell-trigger"`, `id="bell-count"`, `</button>`, `id="bell-list"`)
	// The old class names stay banned: the count is app-topbar__count.
	for _, banned := range []string{"bell-badge", "app-topbar__badge", "data-count"} {
		if strings.Contains(bar, banned) {
			t.Errorf("the bell carries %q; the count is app-topbar__count", banned)
		}
	}
	// The bell sits left of the gear and before the person menu.
	// Amended 2026-10-17 by Phase 3.4 plan 26 (code review part 3 WR-05): the
	// person menu is drawn by account_header_menu inside a div, so the order is
	// pinned on that call and the div that wraps it.
	p421InOrder(t, "bell before gear before person", bar, `id="bell-trigger"`, "app-topbar__gear", `<div class="app-header">{{template "account_header_menu" .}}</div>`)

	js := stripJSComments(p421ReadStatic(t, "js/app-bar.js"))
	for _, want := range []string{"aria-expanded", "'Escape'", "bell-trigger", "bell-list", "history.back()", "document.referrer"} {
		if !strings.Contains(js, want) {
			t.Errorf("app-bar.js lacks %q", want)
		}
	}
	for _, banned := range []string{"fetch(", "XMLHttpRequest", "innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment", "eval("} {
		if strings.Contains(js, banned) {
			t.Errorf("app-bar.js uses %q, a markup parsing sink or a request", banned)
		}
	}
	// The back link target is the server's own href: the script never reads a
	// target out of the address or the referrer (T-34-20).
	for _, banned := range []string{"location.href =", "location.assign", "location.replace", "window.open"} {
		if strings.Contains(js, banned) {
			t.Errorf("app-bar.js navigates with %q; the back arrow follows its own href", banned)
		}
	}

	css := stripCSSComments(p421ReadStatic(t, "css/app-bar.css"))
	for _, want := range []string{".bell-list:not([hidden])", "width: 340px", "max-width: calc(100% - 24px)", "z-index: 1060", "var(--shadow-menu)", "border-radius: 8px"} {
		if !strings.Contains(css, want) {
			t.Errorf("app-bar.css lacks %q for the bell list", want)
		}
	}
}

// bellTagWith returns the opening tag of the element in the template that
// carries the given attribute text.
func bellTagWith(t *testing.T, tmpl, attr string) string {
	t.Helper()
	at := strings.Index(tmpl, attr)
	if at == -1 {
		t.Fatalf("the template has no element with %s", attr)
	}
	start := strings.LastIndex(tmpl[:at], "<")
	end := strings.Index(tmpl[at:], ">")
	if start == -1 || end == -1 {
		t.Fatalf("cannot find the tag around %s", attr)
	}
	return tmpl[start : at+end+1]
}

// TestAlertsNeverSaysNoAlerts pins D-22 and T-34-17: the Official alerts
// template never claims that no warning is in force, because every state
// sentence is written by official-alerts.js from the server's state and the
// template itself may not make a claim Pinalert cannot back.
//
// Amended 2026-10-29 by plan 05-09 (ALERT-04): the page is filled; the honesty
// sentences live in official-alerts.js and are pinned by
// web/official_alerts_test.mjs against every state. The two "not connected"
// wants are replaced by the noscript line and the attribution line; every claim
// ban, the list and count bans and the notice strip ban are kept as they were.
func TestAlertsNeverSaysNoAlerts(t *testing.T) {
	page := p421ReadTemplate(t, "alerts.html.tmpl")
	for _, want := range []string{
		"Turn on JavaScript to see official alerts, or check your state disaster management authority.",
		"Alerts are copied from NDMA SACHET. Pinalert is not an official channel. Hazard events come from the Global Disaster Alert and Coordination System, GDACS.",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("alerts.html.tmpl lacks %q", want)
		}
	}
	lower := strings.ToLower(page)
	for _, banned := range []string{
		"no" + " alerts", "no official" + " alerts", "no" + " warnings", "all" + " clear",
		"nothing in" + " force", "in" + " force", "last" + " checked", "0" + " alerts",
	} {
		if strings.Contains(lower, banned) {
			t.Errorf("alerts.html.tmpl holds %q, a claim only official-alerts.js may write from the server's state", banned)
		}
	}
	// The script builds the list and the count: the template holds no list.
	for _, banned := range []string{"<ul", "<ol", "<table", "data-count"} {
		if strings.Contains(lower, banned) {
			t.Errorf("alerts.html.tmpl holds %q; official-alerts.js builds the list", banned)
		}
	}
	// D-29: the notice strip is not drawn here.
	if strings.Contains(page, "disclaimer_bar.html.tmpl") {
		t.Errorf("alerts.html.tmpl calls the notice strip partial")
	}
}

// waitingSettingsLabels are the rows and the person menu item that wait for a
// later phase (UI-SPEC Surface 6, Rows that wait, and Surface 1c). A row or a
// control renders only when its feature exists (D-26). About you arrives in
// plan 06 and is not listed here.
//
// Amended 2026-10-19 by Phase 3.2 plan 12 (D-23, LAYOUT-06): People and reports,
// Muted reports, Log out of all devices and Delete my account left this list,
// because their features now exist and Settings draws them.
//
// Amended 2026-10-19 by Phase 3.2 plan 15 (D-23, D-26, late amendment rows,
// UI-SPEC E1): "Phone notifications" (the section is now called Notifications),
// "A Reliable reporter posts near me" and "How far from my home area" left this
// list, because Settings draws them now. "Critical reports near my home area",
// "Severe official alerts for my area", "Changes to reports I posted or voted
// on" and "Someone I follow posts" stay for Phase 3.3 and Phase 9.
//
// Amended 2026-10-18 by Phase 3.3 plan 14 (D-12, decision 12, UI-SPEC Surface 2):
// "Critical reports near my home area" and "Changes to reports I posted or voted
// on" left this list, because Settings draws them now. "Someone I follow posts"
// left it too: the owner moved that choice to Phase 9's per person Notify me
// bell on 2026-10-18, so it never comes to Settings (TestNotificationsNoFollowOrOfficialRow
// pins its absence). "Severe official alerts for my area" stays for Phase 5.
//
// Amended 2026-10-29 by plan 05-14 (D-11): Phase 5 draws the official row, so
// "Severe official alerts for my area" left this list. TestNotificationsOfficialRow
// pins the row itself and the welcome banned list still bans the words.
//
// Amended 2026-10-29 by Phase 9 plan 12 (D-04, owner decision 2026-10-16, UI-SPEC
// 7a and 7c): "View my profile", "My profile", "Let others follow me",
// "Following", "Followers" and "Blocked people" left this list, because Settings
// and the person menu draw them now. The list is empty and stays as the place a
// later phase adds a label that waits; the loop below still runs over it.
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D1):
// "Critical reports near my home area", "Severe official alerts for my area",
// "Changes and comments on reports I posted, voted on or commented on" and "A
// Reliable reporter posts near me" left Settings for good on 2026-11-03. They are
// not waiting for a later phase, so the list stays empty and does not take them.
var waitingSettingsLabels = []string{}

// TestSettingsShowsOnlyExistingRows pins D-25 and D-26: Settings holds the
// About Pinalert rows and the phone Account row, and none of the labels that
// wait for a later phase, in the page or in the person menu.
func TestSettingsShowsOnlyExistingRows(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	// Amended 2026-10-22 (owner: "too much details under the heading for each
	// section"): the lines under the section headings are gone, so only the
	// headings and rows are pinned here.
	for _, want := range []string{
		"About Pinalert",
		"Disclaimer", "Unofficial service, what it can and cannot promise", `href="/disclaimer"`,
		"Policy", "Privacy, data kept, reporting rules", `href="/policy#policy"`,
		"Low data view", "Text only, no map", `href="/low"`,
		"Account", `action="/auth/logout"`, "Log out",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("settings.html.tmpl lacks %q", want)
		}
	}
	// Amended 2026-11-13 by quick 261113-t4y (owner decision 2026-11-13, the compact
	// profile): About you left Settings. The Username, Name and Home area rows and
	// their windows are on your profile page behind Edit profile, so Settings holds
	// none of them, and the profile page holds the rows and the windows.
	for _, gone := range []string{
		`>About you</h2>`, "settings-you-title", "Shown only if you let others follow you", "Used for alerts, never your exact spot",
		`id="settings-username-row"`, `id="settings-name-row"`, `id="settings-home-row"`,
		`id="settings-username-window"`, `id="settings-name-window"`, `id="settings-home-window"`,
	} {
		if strings.Contains(page, gone) {
			t.Errorf("settings.html.tmpl still holds %q, About you moved to the profile page", gone)
		}
	}
	person := p421ReadTemplate(t, "person.html.tmpl")
	windows := p421ReadTemplate(t, "profile_edit_windows.html.tmpl")
	for _, want := range []string{`id="settings-username-row"`, `id="settings-name-row"`, `id="settings-home-row"`, `id="person-edit-panel"`, "Not set"} {
		if !strings.Contains(person, want) {
			t.Errorf("person.html.tmpl lacks %q", want)
		}
	}
	for _, want := range []string{`id="settings-username-window"`, `id="settings-name-window"`, `id="settings-home-window"`} {
		if !strings.Contains(windows, want) {
			t.Errorf("profile_edit_windows.html.tmpl lacks %q", want)
		}
	}
	// Amended 2026-10-19 by Phase 3.2 plan 12 (D-23, LAYOUT-06, UI-SPEC D3 and D4):
	// the Account line is the new sentence, the section order is About you,
	// People and reports, About Pinalert, Account, and eight rows draw a chevron
	// (Name, Home area, Muted reports, Disclaimer, Policy, Low data view, Log out
	// of all devices, Delete my account).
	//
	// Amended 2026-10-19 by Phase 3.2 plan 15 (D-23, D-26, LAYOUT-06, UI-SPEC E1):
	// the Notifications section sits after About you, so the order is you,
	// settings-phone-title, people, About Pinalert, account; and a ninth row
	// draws a chevron, the "Add a home area first" row that opens the Home area
	// window.
	p421InOrder(t, "section order", page, `id="settings-phone-title"`, `id="settings-people-title"`, "About Pinalert", `id="settings-account-title"`)
	//
	// Amended 2026-10-18 by Phase 3.3 plan 14 (D-12, UI-SPEC Surface 2): two more
	// rows draw a chevron, the On this device row that opens the install guide
	// and the Critical reports "Add a home area first" row, so eleven.
	//
	// Amended 2026-10-29 by plan 05-14 (D-11): Phase 5 draws the official row, whose
	// no home variant "Add a home area first" opens the Home area window, so twelve.
	//
	// Amended 2026-10-29 by Phase 7.1 plan 15 (D-07, UI-SPEC Surface 8): the
	// Username row opens a window too, so thirteen.
	//
	// Amended 2026-10-29 by Phase 9 plan 12 (D-04, UI-SPEC 7a and 7c): the Following,
	// Followers and Blocked people rows open the list window and the phone View my
	// profile row opens the profile page, so seventeen.
	//
	// Amended 2026-10-29 by Phase 9 plan 20 (D-12, UI-SPEC Defaults 7): the Muted
	// people row opens the same list window, so eighteen.
	//
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
	// D1, C9): the Critical, official and Reliable "Add a home area first" rows are
	// gone and the distance row has its own "Add a home area first" button, so
	// three chevrons left and one came: sixteen.
	//
	// Amended 2026-11-13 by quick 261113-t4y (owner decision 2026-11-13): the
	// Username, Name and Home area rows left Settings with About you, so thirteen.
	//
	// Amended 2026-11-17 (quick 261117-lq0, owner on the 7.1 UAT): the Following,
	// Followers and View my profile rows left Settings and the My activity
	// section's five part rows came. The five are one range over ActivityLinks,
	// so the template source holds one chevron for them: eleven in the source,
	// fifteen drawn. My activity is the first section.
	//
	// Amended 2026-11-17 (quick 261117-vuf, owner on the 7.1 UAT): the section is
	// called Activity and holds one My activity row, written out in the source, so
	// still eleven in the source and now eleven drawn.
	//
	// Amended 2026-11-18 by quick 261118-srq (owner, 7.1 UAT test 8): the first
	// About Pinalert row, Add to Dock, opens a window and draws a chevron; its
	// Install app variant does not. Twelve in the source.
	p421InOrder(t, "section order", page, `id="settings-activity-title"`, `id="settings-phone-title"`, `id="settings-people-title"`)
	if got := strings.Count(page, "app-icon--chevron-right"); got != 12 {
		t.Errorf("settings.html.tmpl holds %d chevrons, want twelve (ten rows, the My activity row and the Add to Dock row)", got)
	}
	header := p421ReadTemplate(t, "account_header.html.tmpl")
	for _, label := range waitingSettingsLabels {
		if strings.Contains(page, label) {
			t.Errorf("settings.html.tmpl holds %q, a row that waits for a later phase", label)
		}
		if strings.Contains(header, label) {
			t.Errorf("account_header.html.tmpl holds %q, an item that waits for a later phase", label)
		}
	}
	if strings.Contains(page, "disclaimer_bar.html.tmpl") {
		t.Errorf("settings.html.tmpl calls the notice strip partial")
	}
}

// settingsWindowRegion returns the markup of the edit window with the given id,
// up to the next window or the bottom bar.
func settingsWindowRegion(t *testing.T, page, id string) string {
	t.Helper()
	start := strings.Index(page, `id="`+id+`"`)
	if start == -1 {
		t.Fatalf("settings.html.tmpl has no window %s", id)
	}
	end := len(page)
	for _, stop := range []string{`<div class="app-window `, `{{template "app_bottombar"`} {
		if i := strings.Index(page[start:], stop); i != -1 && start+i < end {
			end = start + i
		}
	}
	return page[start:end]
}

// TestSettingsWindowsCloseWithXOnly pins D-02 and D-24b: each Settings edit
// window is a modal dialog with one close control, an X named Close, and no
// Cancel or Done; it holds the labels, placeholders and the one save button the
// copy contract names (P34-C29); and the shared window sheet is linked before
// phone.css, centres the laptop window at min(480px, 92%) and keeps the hidden
// attribute working.
func TestSettingsWindowsCloseWithXOnly(t *testing.T) {
	// Amended 2026-11-13 by quick 261113-t4y (owner decision 2026-11-13): the three
	// edit windows moved from Settings to the profile page, so they are read from
	// their partial, and the sheets and the script are read from the profile page.
	page := p421ReadTemplate(t, "profile_edit_windows.html.tmpl")
	person := p421ReadTemplate(t, "person.html.tmpl")
	windows := []struct {
		id    string
		title string
		wants []string
		save  string
	}{
		{"settings-name-window", "Name", []string{
			// Amended 2026-10-29 by Phase 7.1 plan 15 (UI-SPEC P71-C4): the label reads Name.
			`for="settings-name-input">Name</label>`, `id="settings-name-input" name="display_name"`,
			`maxlength="60"`, `autocomplete="name"`,
		}, "Save name"},
		{"settings-home-window", "Home area", []string{
			`for="settings-home-input">Home area</label>`, `placeholder="Search a place"`, `enterkeyhint="search"`,
			"A town or area, never your street. Used for alerts.", `role="listbox"`, `role="status"`,
			// Amended 2026-10-29 by Phase 7.1 plan 15 (D-15): the Remove home area button is
			// gone; TestSettingsHomeAreaCannotBeRemoved pins its absence.
		}, "Save home area"},
	}
	for _, w := range windows {
		region := settingsWindowRegion(t, page, w.id)
		for _, want := range append([]string{
			`role="dialog"`, `aria-modal="true"`, `aria-labelledby=`, `class="app-window__title"`, ">" + w.title + "</h2>",
			`class="inline-error"`, `role="alert"`, ">" + w.save + "</button>",
		}, w.wants...) {
			if !strings.Contains(region, want) {
				t.Errorf("%s lacks %q", w.id, want)
			}
		}
		if got := strings.Count(region, `aria-label="Close"`); got != 1 {
			t.Errorf("%s has %d controls named Close, want exactly 1 (the X)", w.id, got)
		}
		if got := strings.Count(region, "app-icon--x"); got != 1 {
			t.Errorf("%s draws %d X glyphs, want 1", w.id, got)
		}
		lower := strings.ToLower(region)
		for _, banned := range []string{">cancel<", ">done<", ">close<", "cancel", "data-dismiss", "data-close-backdrop"} {
			if strings.Contains(lower, banned) {
				t.Errorf("%s holds %q; the X is the only way to close (D-02)", w.id, banned)
			}
		}
	}
	// Only the Home area window carries the Remove button, and it can be hidden.
	if strings.Contains(settingsWindowRegion(t, page, "settings-name-window"), "Remove") {
		t.Errorf("the Name window holds a Remove button")
	}

	p421InOrder(t, "stylesheet order", person,
		`/static/css/places.css?v={{.AssetVersion}}`, `/static/css/window.css?v={{.AssetVersion}}`, `/static/css/phone.css?v={{.AssetVersion}}`)

	css := stripCSSComments(p421ReadStatic(t, "css/window.css"))
	for _, want := range []string{
		".app-window:not([hidden])", "z-index: 1100", ".app-window__panel", ".app-window__head", ".app-window__title",
		".app-window__close", ".app-window__body", ".app-window__actions",
		"@media (min-width: 900px)", "min(var(--app-window-width, 480px), 92%)", "border-radius: 12px", "var(--shadow-window)",
		"width: var(--touch-target-min)", "height: var(--touch-target-min)",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("window.css lacks %q", want)
		}
	}
	for _, banned := range []string{"!important", "999px"} {
		if strings.Contains(css, banned) {
			t.Errorf("window.css holds %q", banned)
		}
	}
	if !strings.Contains(person, `<script src="/static/js/profile-edit.js?v={{.AssetVersion}}" defer></script>`) {
		t.Errorf("person.html.tmpl does not load profile-edit.js with defer and the asset version")
	}
	rawCSS := p421ReadStatic(t, "css/window.css")
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(rawCSS, dash) {
			t.Errorf("window.css holds the dash character U+%04X", dash)
		}
	}
}

// TestSettingsScriptContract pins D-24b, D-02, T-34-26 and T-34-27 on
// profile-edit.js: it never asks for the device position, never parses markup,
// never sends a map centre with the place search, saves through the one
// profile route with PATCH, and has no handler that closes a window from a
// click on its backdrop.
func TestSettingsScriptContract(t *testing.T) {
	raw := p421ReadStatic(t, "js/profile-edit.js")
	js := stripJSComments(raw)
	for _, want := range []string{
		"'PATCH'", "/api/me/profile", "/api/geocode/place?kind=area&q=", "textContent", "'Escape'", "display_name", "home_area",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("profile-edit.js lacks %q", want)
		}
	}
	for _, banned := range []string{
		"geolocation", "watchPosition", "getCurrentPosition",
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment", "eval(", "new Function",
		"&lat=", "&lon=", "localStorage", "sessionStorage",
	} {
		if strings.Contains(js, banned) {
			t.Errorf("profile-edit.js uses %q: the device position, a markup parsing sink or a map centre has no place here", banned)
		}
	}
	// Only the row buttons, the X, the forms, the save and remove buttons and
	// the keyboard are bound. A window itself gets no click handler, so a click
	// on the backdrop closes nothing (D-02).
	for _, banned := range []string{"nameWin.addEventListener('click'", "homeWin.addEventListener('click'", "win.addEventListener('click'"} {
		if strings.Contains(js, banned) {
			t.Errorf("profile-edit.js binds %q: a window closes with the X and Escape only", banned)
		}
	}
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(raw, dash) {
			t.Errorf("profile-edit.js holds the dash character U+%04X", dash)
		}
	}
	page := p421ReadTemplate(t, "settings.html.tmpl")
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(page, dash) {
			t.Errorf("settings.html.tmpl holds the dash character U+%04X", dash)
		}
	}
}

// TestSettingsIsAGroupedList pins plan 03.4-43 (owner, 2026-10-21): Settings
// reads as a grouped settings list. Each group is one rounded filled surface,
// rows are divided by a hairline that wins over the row's own border: 0 and
// clears 3 to 1 on the fill, the value is in the text colour, the Log out row
// is in the critical colour. Amended 2026-10-22 (owner): the laptop uses the
// full width grid of sketch 008 with bordered boxes and the line under each
// heading; the phone shows the headings without it. It is scoped to the Settings page, so Official
// alerts keeps its look.
func TestSettingsIsAGroupedList(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	// Amended 2026-10-18 by Phase 3.3 plan 14 (D-12): the main element also
	// carries data-push-key, the public key settings-alerts.js hands to notify.js.
	if !strings.Contains(page, `<main class="place-page place-page--settings" data-account-id="{{.AccountID}}" data-push-key="{{.PushPublicKey}}">`) {
		t.Errorf("settings.html.tmpl's main lacks the place-page--settings modifier")
	}
	if strings.Contains(p421ReadTemplate(t, "alerts.html.tmpl"), "place-page--settings") {
		t.Errorf("alerts.html.tmpl carries the Settings modifier")
	}
	const css = "static/css/places.css"
	p16MustDecl(t, "settings vars", p31mRule(t, css, "", ".place-page--settings"), map[string]string{
		"--settings-sub":  "color-mix(in srgb, var(--color-text-muted) 85%, var(--color-text))",
		"--settings-line": "color-mix(in srgb, var(--color-text-muted) 85%, var(--color-surface))",
	})
	p16MustDecl(t, "settings group", p31mRule(t, css, "", ".place-page--settings .place-box"), map[string]string{
		"border": "0", "border-radius": "12px", "background": "var(--color-surface)",
	})
	p16MustDecl(t, "settings hairline", p31mRule(t, css, "", ".place-page--settings .place-box > * + *::before"), map[string]string{
		"border-top": "1px solid var(--settings-line)", "left": "var(--space-md)", "right": "0", "top": "0",
	})
	p16MustDecl(t, "settings value", p31mRule(t, css, "", ".place-page--settings .place-row__value"), map[string]string{
		"color": "var(--color-text)",
	})
	p16MustDecl(t, "settings second line", p31mRule(t, css, "", ".place-page--settings .place-row__sub"), map[string]string{
		"color": "var(--settings-sub)",
	})
	p16MustDecl(t, "settings Log out", p31mRule(t, css, "", ".place-page--settings .place-row--button .place-row__label"), map[string]string{
		"color": "var(--color-severity-critical)", "font-weight": "var(--font-weight-semibold)",
	})
	// Amended 2026-10-22 (owner, with the sketch 008 laptop Settings: "i need
	// this display for settings for laptop"): plan 43's centred 640px column
	// is gone; the laptop uses the full width 260px | 1fr grid of Official
	// alerts, each group a bordered page coloured box with full width lines.
	const laptop = "@media (min-width: 900px)"
	if got := p31mRules(t, css, ".place-page--settings"); len(got) > 0 {
		for _, r := range got {
			if r.Media == laptop && (r.Decls["max-width"] != "" || r.Decls["margin"] != "") {
				t.Errorf("the laptop Settings page must not be a centred column: %v", r.Decls)
			}
		}
	}
	p16MustDecl(t, "settings laptop vars", p31mRule(t, css, laptop, ".place-page--settings"), map[string]string{
		"--settings-sub": "var(--color-text-muted)", "--settings-line": "var(--color-border)",
	})
	p16MustDecl(t, "settings laptop section", p31mRule(t, css, laptop, ".place-page--settings .place-section"), map[string]string{
		"padding": "var(--space-lg) var(--space-lg) 40px", "align-items": "start",
	})
	p16MustDecl(t, "shared laptop grid", p31mRule(t, css, laptop, ".place-section"), map[string]string{
		"display": "grid", "grid-template-columns": "260px minmax(0, 1fr)", "border-top": "1px solid var(--color-border)",
	})
	p16MustDecl(t, "settings laptop box", p31mRule(t, css, laptop, ".place-page--settings .place-box"), map[string]string{
		"border": "1px solid var(--color-border)", "background": "var(--color-bg)",
	})
	p16MustDecl(t, "settings laptop lines", p31mRule(t, css, laptop, ".place-page--settings .place-box > * + *::before"), map[string]string{
		"left": "0",
	})
	p16MustDecl(t, "settings laptop value", p31mRule(t, css, laptop, ".place-page--settings .place-row__value"), map[string]string{
		"color": "var(--color-text-muted)",
	})
	// Owner, 2026-10-22: "too much details under the heading for each
	// section" on the phone, then with the sketch 008 picture of the laptop
	// left column, "stuff like this should also be included": each section
	// has its line, shown on the laptop only.
	for _, line := range []string{
		// Amended 2026-11-13 by quick 261113-t4y: the About you section and its line left
		// Settings, so the line is no longer pinned.
		`<p class="place-section__line">The rules, how your data is used, and a lighter version for weak networks.</p>`,
		`<p class="place-section__line">Sign out, or remove your account and its data.</p>`,
	} {
		if !strings.Contains(page, line) {
			t.Errorf("settings.html.tmpl lacks the section line %s", line)
		}
	}
	p16MustDecl(t, "no section line on the phone", p31mRule(t, css, "", ".place-page--settings .place-section__line"), map[string]string{
		"display": "none",
	})
	p16MustDecl(t, "section line on the laptop", p31mRule(t, css, laptop, ".place-page--settings .place-section__line:not([hidden])"), map[string]string{
		"display": "block",
	})
	// The owner asked for Log out on the laptop Settings page too (2026-10-21,
	// "I don't see a logout option on my laptop version, so I think we should
	// add that"): the Account section is drawn on both sizes, as the last
	// group, through the same POST /auth/logout form the phone uses, and the
	// person menu keeps its own Log out (TestPersonMenuItems).
	if strings.Contains(page, "place-section--phone") || strings.Contains(stripCSSComments(p421ReadStatic(t, "css/places.css")), "place-section--phone") {
		t.Errorf("the Settings Account section is still phone only; Log out must be a row on the laptop too")
	}
	if strings.Count(page, `action="/auth/logout"`) != 1 || strings.Count(page, "place-row--button") != 1 {
		t.Errorf("settings.html.tmpl must hold exactly one Log out row posting to /auth/logout")
	}
	p421InOrder(t, "Account is the last group", page, `id="settings-about-title"`, `id="settings-account-title"`, `action="/auth/logout"`, "</main>")
	for _, banned := range []string{"opacity", "gradient", "999px", "text-decoration: underline", "!important"} {
		if strings.Contains(stripCSSComments(p421ReadStatic(t, "css/places.css")), banned) {
			t.Errorf("places.css holds %q", banned)
		}
	}
}
