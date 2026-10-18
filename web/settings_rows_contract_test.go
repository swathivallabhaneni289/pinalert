package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 3.2 plan 12: the People and reports and Account sections of Settings
// and the three windows they open (D-11, D-23, LAYOUT-06, UI-SPEC Surface E2 and
// E3). The markup is pinned here; the script is pinned in the same file below.

// settingsSectionRegion returns the markup of the section labelled by titleID,
// up to the next section or the end of main.
func settingsSectionRegion(t *testing.T, page, titleID string) string {
	t.Helper()
	at := strings.Index(page, `aria-labelledby="`+titleID+`"`)
	if at == -1 {
		t.Fatalf("settings.html.tmpl has no section labelled by %s", titleID)
	}
	end := len(page)
	for _, stop := range []string{"</section>", "</main>"} {
		if i := strings.Index(page[at:], stop); i != -1 && at+i < end {
			end = at + i
		}
	}
	return page[at:end]
}

// TestPeopleAndReportsSection pins D-23 and UI-SPEC E2: one section with one
// row, Muted reports, whose value is the count and which opens a dialog.
func TestPeopleAndReportsSection(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-people-title")
	for _, want := range []string{
		`id="settings-people-title">People and reports</h2>`,
		"Reports you have turned updates off for.",
		`<button type="button" class="place-row" id="settings-muted-row" aria-haspopup="dialog">`,
		`<span class="place-row__label">Muted reports</span>`,
		`<span class="place-row__value" id="settings-muted-value">{{.MutedCount}}</span>`,
		"app-icon--chevron-right",
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the People and reports section lacks %q", want)
		}
	}
	if got := strings.Count(region, `class="place-row"`); got != 1 {
		t.Errorf("the People and reports section has %d rows, want exactly 1 (Muted reports)", got)
	}
	// No second line under the row: a simple option carries no hint text.
	if strings.Contains(region, "place-row__sub") {
		t.Errorf("the Muted reports row carries a second line")
	}
}

// TestAccountSectionOnBothSizes pins UI-SPEC E3 and D3: the Account section is
// drawn on the phone and the laptop, the Log out row alone is phone only, and
// both new rows exist.
func TestAccountSectionOnBothSizes(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	if strings.Contains(page, "place-section--phone") {
		t.Errorf("settings.html.tmpl still carries place-section--phone: the Account section draws on both sizes")
	}
	region := settingsSectionRegion(t, page, "settings-account-title")
	for _, want := range []string{
		"Log out everywhere, or delete your account.",
		`<form class="place-row-form place-row-form--phone" method="post" action="/auth/logout">`,
		`<span class="place-row__label">Log out</span>`,
		`<button type="button" class="place-row" id="settings-logout-all-row" aria-haspopup="dialog">`,
		`<span class="place-row__label">Log out of all devices</span>`,
		`<button type="button" class="place-row" id="settings-delete-row" aria-haspopup="dialog">`,
		`<span class="place-row__label place-row__label--danger">Delete my account</span>`,
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the Account section lacks %q", want)
		}
	}
	if got := strings.Count(page, "place-row-form--phone"); got != 1 {
		t.Errorf("place-row-form--phone appears %d times in settings.html.tmpl, want 1 (the Log out form only)", got)
	}
	css := stripCSSComments(p421ReadStatic(t, "css/places.css"))
	if !regexp.MustCompile(`@media \(min-width: 900px\)[\s\S]*\.place-row-form--phone \{\s*display: none;`).MatchString(css) {
		t.Errorf("places.css does not hide .place-row-form--phone at 900px and up")
	}
	if strings.Contains(css, "place-section--phone") {
		t.Errorf("places.css still names place-section--phone")
	}
}

// TestAccountConfirmWindows pins UI-SPEC E2 and E3 for the three new windows:
// each is a modal dialog with the X as the only way out, the exact words, and
// in the two confirm windows exactly one filled button.
func TestAccountConfirmWindows(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	windows := []struct {
		id      string
		title   string
		wants   []string
		primary int
	}{
		{"settings-muted-window", "Muted reports", []string{
			`id="settings-muted-loading"`, `id="settings-muted-list"`, `class="muted-list"`,
			`id="settings-muted-empty"`, "No muted reports", "Reports you mute show here, with a way to turn them back on.",
			`id="settings-muted-error"`, "Couldn't load muted reports", "Check your connection and try again.",
			`id="settings-muted-retry"`, ">Try again</button>",
		}, 0},
		{"settings-logout-all-window", "Log out of all devices", []string{
			"This logs you out of Pinalert on every phone and computer, including this one. You can sign in again any time with your email.",
			`id="settings-logout-all-go"`, `<p class="inline-error" id="settings-logout-all-error" role="alert" hidden></p>`,
		}, 1},
		{"settings-delete-window", "Delete my account", []string{
			"<p>This cannot be undone.</p>",
			"Your email, name, home area, updates, muted reports and settings are deleted, and you are logged out on every device.",
			`id="settings-delete-go"`, `<p class="inline-error" id="settings-delete-error" role="alert" hidden></p>`,
		}, 1},
	}
	for _, w := range windows {
		region := settingsWindowRegion(t, page, w.id)
		for _, want := range append([]string{
			`role="dialog"`, `aria-modal="true"`, `aria-labelledby=`, `class="app-window__title"`, `tabindex="-1">` + w.title + "</h2>",
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
		if got := strings.Count(region, "btn--primary"); got != w.primary {
			t.Errorf("%s has %d filled buttons, want %d", w.id, got, w.primary)
		}
		lower := strings.ToLower(region)
		for _, banned := range []string{">cancel<", ">done<", ">close<", "cancel", "data-dismiss", "data-close-backdrop", "backdrop"} {
			if strings.Contains(lower, banned) {
				t.Errorf("%s holds %q; the X is the only way to close (D-02)", w.id, banned)
			}
		}
		// The title takes the first focus, never an action button.
		if strings.Contains(region, "autofocus") {
			t.Errorf("%s holds autofocus; focus starts on the title", w.id)
		}
		// Every window is hidden until opened.
		if !strings.Contains(page, `id="`+w.id+`" hidden>`) {
			t.Errorf("%s is not hidden at load", w.id)
		}
	}
	// The Delete button is the dark filled one; the red lives on the row label only.
	del := settingsWindowRegion(t, page, "settings-delete-window")
	if strings.Contains(del, "critical") || strings.Contains(del, "danger") {
		t.Errorf("the Delete window button or text carries the destructive colour; the red is on the row label only")
	}
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(page, dash) {
			t.Errorf("settings.html.tmpl holds the dash character U+%04X", dash)
		}
	}
	p421InOrder(t, "script order", page,
		`/static/js/settings.js?v={{.AssetVersion}}`, `/static/js/settings-alerts.js?v={{.AssetVersion}}`)
}

// TestDeleteRuleSentenceIsOneString pins the owner's answer to Q2 (2026-10-18):
// Option A is one sentence in one paragraph, with the Q2 comment just before
// it, so changing the rule later is a one sentence change.
func TestDeleteRuleSentenceIsOneString(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	const sentence = "Reports you posted and votes you made stay, with no name or link to you, so nobody else's report changes."
	if got := strings.Count(page, sentence); got != 1 {
		t.Fatalf("the Option A sentence appears %d times in settings.html.tmpl, want exactly 1", got)
	}
	want := `<p id="settings-delete-rule">` + sentence + `</p>`
	at := strings.Index(page, want)
	if at == -1 {
		t.Fatalf("the Option A sentence is not alone in one p element with id settings-delete-rule")
	}
	before := page[:at]
	comment := `{{/* Q2 owner confirmed 2026-10-18: Option A. Option B is the one sentence "Reports you posted and votes you made are removed, and reports other people confirmed with you may change." */}}`
	if !strings.HasSuffix(strings.TrimRight(before, " \t\n"), comment) {
		t.Errorf("the Q2 template comment does not come just before the Option A sentence")
	}
}

// TestSettingsAlertsScriptContract pins T-03.2-08, T-03.2-22 and D-02 on
// settings-alerts.js: it reaches the routes it needs, writes text only through
// textContent, never parses markup or asks for the device position, navigates
// only to the two fixed sign in addresses, and binds no click handler on a
// window element, so a click on the backdrop closes nothing. settings.js is not
// edited by this plan, so TestSettingsScriptContract stays as it is.
func TestSettingsAlertsScriptContract(t *testing.T) {
	raw := p421ReadStatic(t, "js/settings-alerts.js")
	js := stripJSComments(raw)
	for _, want := range []string{
		"/api/me/muted-reports", "/unmute", "/api/me/logout-all", "/api/me/delete",
		"JSON.stringify({ confirm: true })", "/login?done=all-devices", "/login?done=deleted",
		"textContent", "'Escape'", "encodeURIComponent", "window.PinalertSettingsAlerts = ",
		"createElement",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("settings-alerts.js lacks %q", want)
		}
	}
	for _, banned := range []string{
		"geolocation", "watchPosition", "getCurrentPosition",
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "DOMParser", "createContextualFragment", "eval(", "new Function",
		"localStorage", "sessionStorage",
	} {
		if strings.Contains(js, banned) {
			t.Errorf("settings-alerts.js uses %q: a markup parsing sink or the device position has no place here", banned)
		}
	}
	// Navigation goes to the two fixed addresses and nowhere else.
	for _, m := range regexp.MustCompile(`location\.(assign|replace|href)[^;]*;`).FindAllString(js, -1) {
		if !strings.Contains(m, "LOGGED_OUT_PAGE") && !strings.Contains(m, "DELETED_PAGE") {
			t.Errorf("settings-alerts.js navigates with %q, want only the two fixed sign in addresses", m)
		}
	}
	// A window is never given a click handler of its own (D-02).
	for _, banned := range []string{"mutedWin.addEventListener('click'", "logoutWin.addEventListener('click'", "deleteWin.addEventListener('click'", "win.addEventListener('click'"} {
		if strings.Contains(js, banned) {
			t.Errorf("settings-alerts.js binds %q: a window closes with the X and Escape only", banned)
		}
	}
	// The Delete rule sentence lives in the template only.
	if strings.Contains(js, "no name or link to you") {
		t.Errorf("settings-alerts.js holds the Delete rule sentence; it lives in the template only")
	}
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(raw, dash) {
			t.Errorf("settings-alerts.js holds the dash character U+%04X", dash)
		}
	}
}

// Phase 3.2 plan 15: the Notifications section of Settings (NOTIF-05, D-23,
// D-26, LAYOUT-06, UI-SPEC Surface E1, owner decision Q4 of 2026-10-18).

// TestPhoneNotificationsSection pins the section's title and line, the two
// rows' exact words (no second line under the switch), the four distances
// built from the view model, the native switch and the Q4 template comment.
func TestPhoneNotificationsSection(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-phone-title")
	for _, want := range []string{
		`id="settings-phone-title">Notifications</h2>`,
		`<p class="place-section__line">Hear about Reliable reporters near your home area.</p>`,
		`<span class="place-row__label">A Reliable reporter posts near me</span>`,
		`<span class="place-row__label">How far from my home area</span>`,
		`<input class="place-switch" type="checkbox" role="switch" id="settings-reliable-switch"`,
		`{{if .AlertsOn}} checked{{end}}`,
		`<label class="place-row place-row--switch" id="settings-reliable-row" for="settings-reliable-switch">`,
		`<label class="place-row__text" for="settings-radius-select">`,
		`<select class="place-select" id="settings-radius-select"`,
		`{{- range .AlertRadii}}`,
		`{{if eq . $.AlertRadiusKm}} selected{{end}}>{{.}} km</option>`,
		`<p class="inline-error" id="settings-alerts-error" role="alert" hidden></p>`,
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the Notifications section lacks %q", want)
		}
	}
	if !strings.Contains(page, `<section class="place-section" id="settings-alerts-section" aria-labelledby="settings-phone-title">`) {
		t.Errorf("the Notifications section lacks id settings-alerts-section")
	}
	// The distances come from the closed list the server sends, never typed
	// into the template, so the page and PATCH /api/me/alerts agree.
	for _, banned := range []string{">1 km<", ">2 km<", ">5 km<", ">10 km<"} {
		if strings.Contains(region, banned) {
			t.Errorf("the Notifications section types %q into the template; the options come from .AlertRadii", banned)
		}
	}
	// The switch row has no second line: a simple option carries no hint text.
	switchRow := region[strings.Index(region, `id="settings-reliable-row"`):strings.Index(region, `id="settings-radius-row"`)]
	if strings.Contains(switchRow, "place-row__sub") {
		t.Errorf("the switch row carries a second line")
	}
	// Owner decision Q4 (2026-10-18): the name, and no line about the app being open.
	const comment = `{{/* Owner 2026-10-18 (Q4): the section is called Notifications, with no extra line about the app being open; Phase 3.3 adds its phone rows to this same section. */}}`
	if !strings.Contains(page, comment) {
		t.Errorf("settings.html.tmpl lacks the Q4 template comment")
	}
	// The comment text itself is allowed; the rendered words are not.
	rendered := strings.Replace(page, comment, "", 1)
	for _, banned := range []string{"Phone notifications", "Shows in Updates while Pinalert is open", "buzz", "vibrat"} {
		if strings.Contains(strings.ToLower(rendered), strings.ToLower(banned)) {
			t.Errorf("settings.html.tmpl holds %q", banned)
		}
	}
	// The section sits after About you and before People and reports.
	p421InOrder(t, "section order", page, `id="settings-you-title"`, `id="settings-phone-title"`, `id="settings-people-title"`)
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(page, dash) {
			t.Errorf("settings.html.tmpl holds the dash character U+%04X", dash)
		}
	}
}

// TestPhoneNotificationsRowsNeedAHomeArea pins UI-SPEC E1 and orchestrator
// decision 2: without a home area the alert row reads Add a home area first and
// opens the Home area window, and the distance row is not drawn. The hidden
// attribute sits on the boxes, never on a .place-row (which declares display).
func TestPhoneNotificationsRowsNeedAHomeArea(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-phone-title")
	if !strings.Contains(region, `<div class="place-box" id="settings-alerts-on"{{if not .HasHomeArea}} hidden{{end}}>`) {
		t.Errorf("the with home box is not hidden exactly when there is no home area")
	}
	if !strings.Contains(region, `<div class="place-box" id="settings-alerts-off"{{if .HasHomeArea}} hidden{{end}}>`) {
		t.Errorf("the no home box is not hidden exactly when there is a home area")
	}
	on := region[strings.Index(region, `id="settings-alerts-on"`):strings.Index(region, `id="settings-alerts-off"`)]
	off := region[strings.Index(region, `id="settings-alerts-off"`):]
	if !strings.Contains(on, `id="settings-radius-row"`) || strings.Contains(off, `id="settings-radius-row"`) {
		t.Errorf("the distance row must sit only inside the with home box")
	}
	if !strings.Contains(on, `id="settings-reliable-row"`) || strings.Contains(off, `id="settings-reliable-row"`) {
		t.Errorf("the switch row must sit only inside the with home box")
	}
	for _, want := range []string{
		`<button type="button" class="place-row" id="settings-reliable-nohome" aria-haspopup="dialog">`,
		`<span class="place-row__label">A Reliable reporter posts near me</span>`,
		`<span class="place-row__sub">Add a home area first</span>`,
		"app-icon--chevron-right",
	} {
		if !strings.Contains(off, want) {
			t.Errorf("the no home box lacks %q", want)
		}
	}
	if got := strings.Count(page, "Add a home area first"); got != 1 {
		t.Errorf("Add a home area first appears %d times, want 1", got)
	}
	// No .place-row carries the hidden attribute itself.
	if regexp.MustCompile(`class="place-row[^"]*"[^>]*\shidden`).MatchString(region) {
		t.Errorf("a .place-row carries the hidden attribute; .place-row declares display flex, so hide the box")
	}
	// The alerts note sits in the Home area window, before its actions, and
	// is hidden unless the alert is on.
	win := settingsWindowRegion(t, page, "settings-home-window")
	note := `<p class="app-window__help" id="settings-home-alerts-note"{{if not .AlertsOn}} hidden{{end}}>Removing it also turns off Reliable reporter alerts.</p>`
	at := strings.Index(win, note)
	if at == -1 {
		t.Fatalf("the Home area window lacks the alerts note")
	}
	if at > strings.Index(win, `class="app-window__actions"`) {
		t.Errorf("the alerts note must come before the actions of the Home area window")
	}
}
