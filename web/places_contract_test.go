package web

// Phase 3.4 plan 04: the person menu, the bell, Official alerts and Settings.
// These tests read the shipped templates and scripts from the embedded trees,
// never a copy, so they can only drift with the product.

import (
	"strings"
	"testing"
)

// TestPersonMenuItems pins D-11, D-02 and D-26: the laptop person menu holds
// exactly Settings and Log out, shows no email, and links nowhere that has its
// own place (Settings holds the Disclaimer, Policy and Low data view rows).
func TestPersonMenuItems(t *testing.T) {
	header := p421ReadTemplate(t, "account_header.html.tmpl")
	if got := strings.Count(header, `role="menuitem"`); got != 2 {
		t.Errorf("the person menu has %d items, want exactly 2 (Settings and Log out)", got)
	}
	for _, want := range []string{`href="/settings"`, "Settings", `action="/auth/logout"`, "Log out", `auth-icon--settings`, `auth-icon--log-out`} {
		if !strings.Contains(header, want) {
			t.Errorf("the person menu lacks %q", want)
		}
	}
	for _, banned := range []string{
		"{{.Email}}", "{{if .Chrome.LowDataPage}}", `href="/profile"`, `href="/low"`,
		`href="/disclaimer"`, `href="/policy"`, `href="/?full=1"`, "My profile",
	} {
		if strings.Contains(header, banned) {
			t.Errorf("the person menu still holds %q", banned)
		}
	}
}

// TestBellListContract pins D-32 and UI-SPEC Surface 9: the bell is a button
// named Updates with no count, it opens a dialog list that says there are no
// updates yet and links to the Updates part of My activity, and the script that
// runs it builds no markup and makes no request.
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
	// No count: the bell draws no badge while no updates exist.
	for _, banned := range []string{"bell-badge", "app-topbar__badge", "data-count"} {
		if strings.Contains(bar, banned) {
			t.Errorf("the bell carries %q, but no count exists before Phase 3.2", banned)
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

// TestAlertsNeverSaysNoAlerts pins D-22 and T-34-17: the Official alerts page
// says plainly that alerts are not connected yet and points to the state
// disaster management authority, and it never claims that no warning is in
// force, because Pinalert is not looking and that would be false reassurance in
// an emergency.
func TestAlertsNeverSaysNoAlerts(t *testing.T) {
	page := p421ReadTemplate(t, "alerts.html.tmpl")
	for _, want := range []string{
		"Official alerts are not connected yet",
		"Pinalert does not show government warnings yet. Check your state disaster management authority for official warnings.",
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
			t.Errorf("alerts.html.tmpl holds %q, a claim Pinalert cannot back while alerts are not connected", banned)
		}
	}
	// The page is a place with an empty state: no list, no count.
	for _, banned := range []string{"<ul", "<ol", "<table", "data-count"} {
		if strings.Contains(lower, banned) {
			t.Errorf("alerts.html.tmpl holds %q; the list arrives with Phase 5", banned)
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
var waitingSettingsLabels = []string{
	"View my profile", "My profile",
	"Phone notifications", "Critical reports near my home area", "Severe official alerts for my area",
	"Changes to reports I posted or voted on", "A Reliable reporter posts near me", "Someone I follow posts",
	"How far from my home area",
	"People and reports", "Let others follow me", "Following", "Followers", "Muted reports", "Blocked people",
	"Log out of all devices", "Delete my account",
}

// TestSettingsShowsOnlyExistingRows pins D-25 and D-26: Settings holds the
// About Pinalert rows and the phone Account row, and none of the labels that
// wait for a later phase, in the page or in the person menu.
func TestSettingsShowsOnlyExistingRows(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	for _, want := range []string{
		"About Pinalert", "The rules, how your data is used, and a text only version for weak networks.",
		"Disclaimer", "Unofficial service, what it can and cannot promise", `href="/disclaimer"`,
		"Policy", "Privacy, data kept, reporting rules", `href="/policy#policy"`,
		"Low data view", "Text only, no map", `href="/low"`,
		"Account", "Sign out, or remove your account and its data.", `action="/auth/logout"`, "Log out",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("settings.html.tmpl lacks %q", want)
		}
	}
	// Amended 2026-10-17 by Phase 3.4 plan 06 (owner decision, 03.4-CONTEXT D-25
	// and D-24b): About you is the first section and its two rows open edit
	// windows, so the order starts with it and five rows draw a chevron.
	for _, want := range []string{
		"About you", "Your name and home area. Shown only on your public profile.",
		"Shown only if you let others follow you", "Used for alerts, never your exact spot",
		`id="settings-name-row"`, `id="settings-home-row"`, "Not set",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("settings.html.tmpl lacks %q", want)
		}
	}
	p421InOrder(t, "section order", page, `id="settings-you-title"`, "About Pinalert", `id="settings-account-title"`)
	if got := strings.Count(page, "app-icon--chevron-right"); got != 5 {
		t.Errorf("settings.html.tmpl draws %d chevrons, want one on each of the five rows that open something", got)
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
	page := p421ReadTemplate(t, "settings.html.tmpl")
	windows := []struct {
		id    string
		title string
		wants []string
		save  string
	}{
		{"settings-name-window", "Name", []string{
			`for="settings-name-input">Your name</label>`, `placeholder="For example: Ravi Kumar"`,
			`maxlength="60"`, `autocomplete="name"`,
		}, "Save name"},
		{"settings-home-window", "Home area", []string{
			`for="settings-home-input">Home area</label>`, `placeholder="Search a place"`, `enterkeyhint="search"`,
			"A town or area, never your street. Used for alerts.", `role="listbox"`, `role="status"`,
			"Remove home area",
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

	p421InOrder(t, "stylesheet order", page,
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
	if !strings.Contains(page, `<script src="/static/js/settings.js?v={{.AssetVersion}}" defer></script>`) {
		t.Errorf("settings.html.tmpl does not load settings.js with defer and the asset version")
	}
	rawCSS := p421ReadStatic(t, "css/window.css")
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(rawCSS, dash) {
			t.Errorf("window.css holds the dash character U+%04X", dash)
		}
	}
}

// TestSettingsScriptContract pins D-24b, D-02, T-34-26 and T-34-27 on
// settings.js: it never asks for the device position, never parses markup,
// never sends a map centre with the place search, saves through the one
// profile route with PATCH, and has no handler that closes a window from a
// click on its backdrop.
func TestSettingsScriptContract(t *testing.T) {
	raw := p421ReadStatic(t, "js/settings.js")
	js := stripJSComments(raw)
	for _, want := range []string{
		"'PATCH'", "/api/me/profile", "/api/geocode/place?kind=area&q=", "textContent", "'Escape'", "display_name", "home_area",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("settings.js lacks %q", want)
		}
	}
	for _, banned := range []string{
		"geolocation", "watchPosition", "getCurrentPosition",
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment", "eval(", "new Function",
		"&lat=", "&lon=", "localStorage", "sessionStorage",
	} {
		if strings.Contains(js, banned) {
			t.Errorf("settings.js uses %q: the device position, a markup parsing sink or a map centre has no place here", banned)
		}
	}
	// Only the row buttons, the X, the forms, the save and remove buttons and
	// the keyboard are bound. A window itself gets no click handler, so a click
	// on the backdrop closes nothing (D-02).
	for _, banned := range []string{"nameWin.addEventListener('click'", "homeWin.addEventListener('click'", "win.addEventListener('click'"} {
		if strings.Contains(js, banned) {
			t.Errorf("settings.js binds %q: a window closes with the X and Escape only", banned)
		}
	}
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(raw, dash) {
			t.Errorf("settings.js holds the dash character U+%04X", dash)
		}
	}
	page := p421ReadTemplate(t, "settings.html.tmpl")
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(page, dash) {
			t.Errorf("settings.html.tmpl holds the dash character U+%04X", dash)
		}
	}
}
