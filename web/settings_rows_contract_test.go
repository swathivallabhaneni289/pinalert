package web

import (
	"regexp"
	"strings"
	"testing"
)

// Phase 3.2 plan 12: the People and reports and Account sections of Settings
// and the two windows and the Delete box they open (D-11, D-23, LAYOUT-06,
// UI-SPEC Surface E2 and E3). The markup is pinned here; the script is pinned in the same file below.

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
//
// Amended 2026-10-22 when Phase 3.4 met Phase 3.2: the owner said "too much
// details under the heading for each section" on the phone, then with the
// sketch 008 laptop picture "stuff like this should also be included", so the
// line is back and places.css shows it on the laptop only. The sketch's words
// name a public profile, which does not exist until Phase 9, so the line stays
// the 3.2 one.
func TestPeopleAndReportsSection(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-people-title")
	for _, want := range []string{
		`id="settings-people-title">People and reports</h2>`,
		`<p class="place-section__line">Reports you have turned updates off for.</p>`,
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
// drawn on the phone and the laptop, and both new rows exist.
//
// Amended 2026-10-21 when Phase 3.4 plan 43 met Phase 3.2: the owner put the
// Log out row on the laptop too ("I don't see a logout option on my laptop
// version, so I think we should add that"), so Log out is no longer phone
// only: no place-row-form--phone class in the page, no hide rule in the CSS.
//
// Amended 2026-10-22 when Phase 3.4 met Phase 3.2: with the sketch 008 laptop
// picture ("stuff like this should also be included") the Account line is the
// sketch's "Sign out, or remove your account and its data.", shown on the
// laptop only, in place of the 3.2 line "Log out everywhere, or delete your
// account.". The Delete window still says exactly what stays.
func TestAccountSectionOnBothSizes(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	if strings.Contains(page, "place-section--phone") {
		t.Errorf("settings.html.tmpl still carries place-section--phone: the Account section draws on both sizes")
	}
	region := settingsSectionRegion(t, page, "settings-account-title")
	for _, want := range []string{
		`<p class="place-section__line">Sign out, or remove your account and its data.</p>`,
		`<form class="place-row-form" method="post" action="/auth/logout">`,
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
	if strings.Contains(page, "place-row-form--phone") {
		t.Errorf("settings.html.tmpl still marks the Log out form phone only; Log out shows on both sizes (owner, 2026-10-21)")
	}
	css := stripCSSComments(p421ReadStatic(t, "css/places.css"))
	if strings.Contains(css, "place-row-form--phone") {
		t.Errorf("places.css still names place-row-form--phone; Log out shows on both sizes (owner, 2026-10-21)")
	}
	if strings.Contains(css, "place-section--phone") {
		t.Errorf("places.css still names place-section--phone")
	}
}

// TestAccountConfirmWindows pins UI-SPEC E2 and E3 for the two windows, Muted
// reports and Log out of all devices: each is a modal dialog with the X as the
// only way out, the exact words, and in the confirm window exactly one filled
// button. The Delete box is pinned by TestDeleteAccountConfirmBox.
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
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(page, dash) {
			t.Errorf("settings.html.tmpl holds the dash character U+%04X", dash)
		}
	}
	p421InOrder(t, "script order", page,
		`/static/js/settings.js?v={{.AssetVersion}}`, `/static/js/settings-alerts.js?v={{.AssetVersion}}`)
}

// TestDeleteAccountConfirmBox pins the Delete my account box: a native dialog
// that looks like the Log out box, with the exact words and its place in the page.
//
// Amended 2026-10-25, the owner said "yes exactly how the logout option is
// looking and it should work like any delete my account thing work", so Delete
// my account asks in a native dialog with the Log out box classes, Cancel and a
// red Delete; this replaces UI-SPEC E3's X only window and its dark filled
// button.
func TestDeleteAccountConfirmBox(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	const open = `<dialog id="settings-delete-confirm" class="logout-confirm" aria-labelledby="settings-delete-title" aria-describedby="settings-delete-text">`
	at := strings.Index(page, open)
	if at == -1 {
		t.Fatalf("settings.html.tmpl lacks the exact opening tag %s", open)
	}
	end := strings.Index(page[at:], "</dialog>")
	if end == -1 {
		t.Fatalf("the Delete box has no closing dialog tag")
	}
	region := page[at : at+end]
	for _, want := range []string{
		`class="logout-confirm__body"`,
		`<h2 class="logout-confirm__title" id="settings-delete-title" tabindex="-1">Delete your account?</h2>`,
		`<div class="logout-confirm__text" id="settings-delete-text">`,
		`<p>This cannot be undone.</p>`,
		`<p>Your email, name, home area, updates, muted reports and settings are deleted, and you are logged out on every device.</p>`,
		`<p id="settings-delete-rule">`,
		`<p class="inline-error" id="settings-delete-error" role="alert" hidden></p>`,
		`<div class="logout-confirm__actions">`,
		`<button type="button" class="btn" id="settings-delete-cancel">Cancel</button>`,
		`<button type="button" class="btn btn--destructive" id="settings-delete-go">Delete</button>`,
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the Delete box lacks %q", want)
		}
	}
	p421InOrder(t, "Delete box order", region,
		"This cannot be undone.", "Your email", `id="settings-delete-rule"`,
		`id="settings-delete-error"`, `id="settings-delete-cancel"`, `id="settings-delete-go"`)
	for _, banned := range []string{`aria-label="Close"`, "app-icon--x", "autofocus", "btn--primary"} {
		if strings.Contains(region, banned) {
			t.Errorf("the Delete box holds %q", banned)
		}
	}
	for _, old := range []string{"settings-delete-window", "settings-delete-close"} {
		if strings.Contains(page, old) {
			t.Errorf("settings.html.tmpl still holds %q: the old Delete window is gone", old)
		}
	}
	// A direct child of the body: after main, before the first window, so it is
	// never inside a hidden ancestor and never inside a window's region.
	p421InOrder(t, "Delete box position", page, "</main>", open, `id="settings-name-window"`)
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
//
// Amended 2026-10-22 when Phase 3.4 met Phase 3.2: the owner said "too much
// details under the heading for each section" on the phone, then with the
// sketch 008 laptop picture "stuff like this should also be included", so the
// line is back and places.css shows it on the laptop only. The sketch's words
// promise a buzz, which the silent notifications rule forbids, so the line
// stays the 3.2 one.
//
// Amended 2026-10-18 by Phase 3.3 plan 14 (D-12, UI-SPEC N1): the section line
// is "Choose what Pinalert tells you about." in place of 3.2's P32-C21 line
// (which must now be absent), because the section covers more than Reliable
// reporters and "hear" implies sound. The reliable row is hidden by its own
// attribute when there is no home area, so its opening tag carries the
// conditional; the two boxes became one box with per row variants, so the
// classless wrapper now holds the one box and the two error lines.
func TestPhoneNotificationsSection(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-phone-title")
	for _, want := range []string{
		`id="settings-phone-title">Notifications</h2>`,
		`<p class="place-section__line">Choose what Pinalert tells you about.</p>`,
		`<span class="place-row__label">A Reliable reporter posts near me</span>`,
		`<span class="place-row__label">How far from my home area</span>`,
		`<input class="place-switch" type="checkbox" role="switch" id="settings-reliable-switch"`,
		`{{if .AlertsOn}} checked{{end}}`,
		`<label class="place-row place-row--switch" id="settings-reliable-row" for="settings-reliable-switch"{{if not .HasHomeArea}} hidden{{end}}>`,
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
	// On the laptop .place-section is a two column grid (title, then the rows),
	// so the boxes and the error line sit in one classless wrapper: a third
	// grid item would land under the title instead of under the box.
	wrapped := regexp.MustCompile(`</div>\s*(?:\{\{/\*[^}]*\*/\}\})?\s*<div>\s*(?:\{\{/\*[^}]*\*/\}\})?\s*<div class="place-box" id="settings-alerts-box">[\s\S]*id="settings-alerts-error"[^>]*></p>\s*</div>\s*</section>`)
	if !wrapped.MatchString(page) {
		t.Errorf("the Notifications box and error lines are not inside one classless wrapper div")
	}
	if strings.Contains(page, "Hear about Reliable reporters") {
		t.Errorf("settings.html.tmpl still holds the 3.2 section line (P32-C21), which N1 replaced")
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
	switchRow := region[strings.Index(region, `id="settings-reliable-row"`):strings.Index(region, `id="settings-reliable-nohome"`)]
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
// opens the Home area window, and the distance row is not drawn.
//
// Amended 2026-10-18 by Phase 3.3 plan 14 (D-12, UI-SPEC 2b and E1): the two
// boxes became per row variants. The Critical and Reliable rows each draw a
// switch variant and a no home variant, one of them hidden by its own hidden
// attribute, and the distance row is hidden by its own attribute without a home
// area. A row may now carry the hidden attribute because places.css guards the
// .place-row display rule with :not([hidden]); that guard is pinned here in
// place of the old "never on a .place-row" ban. Add a home area first appears
// twice (Critical and Reliable). The Home area note (W1) shows only with a home
// area and Critical or Reliable on.
func TestPhoneNotificationsRowsNeedAHomeArea(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-phone-title")
	for _, w := range []struct{ id, hiddenWhen string }{
		{"settings-reliable-row", `{{if not .HasHomeArea}} hidden{{end}}`},
		{"settings-reliable-nohome", `{{if .HasHomeArea}} hidden{{end}}`},
		{"settings-critical-on", `{{if not .HasHomeArea}} hidden{{end}}`},
		{"settings-critical-nohome", `{{if .HasHomeArea}} hidden{{end}}`},
		{"settings-radius-row", `{{if not .HasHomeArea}} hidden{{end}}`},
	} {
		at := strings.Index(region, `id="`+w.id+`"`)
		if at == -1 {
			t.Fatalf("the Notifications section has no %s", w.id)
		}
		open := region[at : at+strings.Index(region[at:], ">")]
		if !strings.Contains(open, w.hiddenWhen) {
			t.Errorf("%s is not hidden by %s in its opening tag: %s", w.id, w.hiddenWhen, open)
		}
	}
	// The wrappers are never hidden themselves, so the box always draws a
	// hairline between visible rows.
	for _, id := range []string{"settings-device-row", "settings-critical-row", "settings-reliable-wrap"} {
		at := strings.Index(region, `id="`+id+`"`)
		if at == -1 {
			t.Fatalf("the Notifications section has no wrapper %s", id)
		}
		if open := region[at : at+strings.Index(region[at:], ">")]; strings.Contains(open, "hidden") {
			t.Errorf("the wrapper %s is hidden itself: %s", id, open)
		}
	}
	for _, want := range []string{
		`<button type="button" class="place-row" id="settings-reliable-nohome" aria-haspopup="dialog"{{if .HasHomeArea}} hidden{{end}}>`,
		`<button type="button" class="place-row" id="settings-critical-nohome" aria-haspopup="dialog"{{if .HasHomeArea}} hidden{{end}}>`,
		`<span class="place-row__label">A Reliable reporter posts near me</span>`,
		`<span class="place-row__label">Critical reports near my home area</span>`,
		`<span class="place-row__sub">Add a home area first</span>`,
		"app-icon--chevron-right",
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the Notifications section lacks %q", want)
		}
	}
	if got := strings.Count(page, "Add a home area first"); got != 2 {
		t.Errorf("Add a home area first appears %d times, want 2 (Critical and Reliable)", got)
	}
	// A hidden row only disappears because places.css guards the display rule.
	css := stripCSSComments(p421ReadStatic(t, "css/places.css"))
	if !strings.Contains(css, ".place-row:not([hidden])") {
		t.Errorf("places.css lacks the .place-row:not([hidden]) display rule; a hidden row would stay on screen")
	}
	if regexp.MustCompile(`(?m)^\.place-row\s*\{[^}]*display\s*:`).MatchString(css) {
		t.Errorf("places.css declares display on a bare .place-row rule; it must be guarded with :not([hidden])")
	}
	// The alerts note sits in the Home area window, before its actions, and
	// is hidden unless there is a home area and Critical or Reliable is on.
	win := settingsWindowRegion(t, page, "settings-home-window")
	note := `<p class="app-window__help" id="settings-home-alerts-note"{{if not (and .HasHomeArea (or .AlertsOn .CriticalPushOn))}} hidden{{end}}>Removing it also turns off alerts about reports near your home area.</p>`
	at := strings.Index(win, note)
	if at == -1 {
		t.Fatalf("the Home area window lacks the alerts note")
	}
	if at > strings.Index(win, `class="app-window__actions"`) {
		t.Errorf("the alerts note must come before the actions of the Home area window")
	}
}

// TestSettingsAlertsSavesAtOnce pins UI-SPEC E1 and P32-C22 and C23 on
// settings-alerts.js: each change goes to PATCH /api/me/alerts with the two
// field names the route takes, a failure puts the old value back with the fixed
// sentences, the section follows the Home area row through a MutationObserver
// (settings.js is not edited), and every string is written as text.
func TestSettingsAlertsSavesAtOnce(t *testing.T) {
	raw := p421ReadStatic(t, "js/settings-alerts.js")
	js := stripJSComments(raw)
	for _, want := range []string{
		"/api/me/alerts", "'PATCH'", "reliable_alerts", "alert_radius_km", "MutationObserver",
		"'data-value'", "attributeFilter",
		// P32-C22
		`Couldn\'t save that. Try again.`,
		`You\'re offline, so this wasn\'t saved. Try again when you have a signal.`,
		// P32-C23
		"Home area removed. Reliable reporter alerts are off.",
		"navigator.onLine === false",
		"saveAlerts: saveAlerts", "redrawAlerts: redrawAlerts",
		"openWindow('home')",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("settings-alerts.js lacks %q", want)
		}
	}
	// Nothing is stored in the browser and no coordinate is read or sent; the
	// markup sinks are banned for the whole file by TestSettingsAlertsScriptContract.
	for _, banned := range []string{"home_area_cell", "geohash", "lat", "lon"} {
		if regexp.MustCompile(`\b` + banned + `\b`).MatchString(js) {
			t.Errorf("settings-alerts.js names %q: the home cell and coordinates never reach the browser (T-03.2-14)", banned)
		}
	}
	// settings.js is not edited by this plan: it does not know the alerts route.
	settings := stripJSComments(p421ReadStatic(t, "js/settings.js"))
	if strings.Contains(settings, "/api/me/alerts") || strings.Contains(settings, "settings-reliable") {
		t.Errorf("settings.js names the alert route or rows; plan 15 leaves settings.js as built")
	}
}

// Phase 3.3 plan 14: the phone rows of the Notifications section (PWA-03,
// PWA-06, D-08, D-11, D-12, D-21, UI-SPEC Surface 2 and Surface 4b).

// openingTagOf returns the opening tag of the element with the given id.
func openingTagOf(t *testing.T, region, id string) string {
	t.Helper()
	at := strings.Index(region, `id="`+id+`"`)
	if at == -1 {
		t.Fatalf("the markup has no element with id %s", id)
	}
	start := strings.LastIndex(region[:at], "<")
	return region[start : at+strings.Index(region[at:], ">")+1]
}

// TestNotificationsSectionRows pins the five rows in the owner's order with
// their exact words, each switch a native checkbox with role switch, and the
// install guide window with one close control and the shared partial.
func TestNotificationsSectionRows(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-phone-title")
	p421InOrder(t, "the Notifications rows", region,
		`id="settings-device-row"`,
		`id="settings-critical-row"`,
		`id="settings-changes-row"`,
		`id="settings-reliable-wrap"`,
		`id="settings-radius-row"`,
	)
	for _, want := range []string{
		`<span class="place-row__label">On this device</span>`,
		`<span class="place-row__label">Critical reports near my home area</span>`,
		`<span class="place-row__label">Changes to reports I posted or voted on</span>`,
		`<span class="place-row__label">A Reliable reporter posts near me</span>`,
		`<span class="place-row__label">How far from my home area</span>`,
		`{{if .CriticalPushOn}} checked{{end}}`,
		`{{if .ChangesPushOn}} checked{{end}}`,
		`{{if .AlertsOn}} checked{{end}}`,
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the Notifications section lacks %q", want)
		}
	}
	// Each kind switch is a native checkbox with role switch, the whole row a label.
	for _, w := range []struct{ row, input, name string }{
		{"settings-device-on", "settings-device-switch", "device_alerts"},
		{"settings-critical-on", "settings-critical-switch", "critical_push"},
		{"settings-changes-row", "settings-changes-switch", "changes_push"},
		{"settings-reliable-row", "settings-reliable-switch", "reliable_alerts"},
	} {
		if tag := openingTagOf(t, region, w.row); !strings.HasPrefix(tag, `<label class="place-row place-row--switch"`) || !strings.Contains(tag, `for="`+w.input+`"`) {
			t.Errorf("%s is not a switch row label for %s: %s", w.row, w.input, tag)
		}
		tag := openingTagOf(t, region, w.input)
		for _, want := range []string{`<input class="place-switch"`, `type="checkbox"`, `role="switch"`, `name="` + w.name + `"`} {
			if !strings.Contains(tag, want) {
				t.Errorf("%s lacks %s: %s", w.input, want, tag)
			}
		}
	}
	// No hint line under a simple switch: only the no home and device variants
	// carry a second line.
	for _, id := range []string{"settings-device-on", "settings-critical-on", "settings-changes-row", "settings-reliable-row"} {
		at := strings.Index(region, `id="`+id+`"`)
		end := at + strings.Index(region[at:], "</label>")
		if strings.Contains(region[at:end], "place-row__sub") {
			t.Errorf("%s carries a second line; a simple switch has no hint text", id)
		}
	}
	// The key the script hands to notify.js, and the two error lines.
	if !strings.Contains(page, `data-push-key="{{.PushPublicKey}}"`) {
		t.Errorf("settings.html.tmpl does not render data-push-key")
	}
	for _, want := range []string{
		`<p class="inline-error" id="settings-device-error" role="alert" hidden></p>`,
		`<p class="inline-error" id="settings-alerts-error" role="alert" hidden></p>`,
	} {
		if !strings.Contains(region, want) {
			t.Errorf("the Notifications section lacks %q", want)
		}
	}
	// The guide window: one close control, the shared partial, no other button.
	if got := strings.Count(page, `id="device-guide-window"`); got != 1 {
		t.Fatalf("the page holds %d guide windows, want 1", got)
	}
	win := settingsWindowRegion(t, page, "device-guide-window")
	for _, want := range []string{
		`role="dialog"`, `aria-modal="true"`, `aria-labelledby="device-guide-title"`,
		`class="app-window__title" id="device-guide-title" tabindex="-1">Install to get alerts</h2>`,
		`{{template "device_guide"}}`,
	} {
		if !strings.Contains(win, want) {
			t.Errorf("the guide window lacks %q", want)
		}
	}
	if got := strings.Count(win, `aria-label="Close"`); got != 1 {
		t.Errorf("the guide window has %d close controls, want exactly 1 (the X)", got)
	}
	if got := strings.Count(win, "<button"); got != 1 {
		t.Errorf("the guide window holds %d buttons, want only the X", got)
	}
	if !strings.Contains(page, `id="device-guide-window" hidden>`) {
		t.Errorf("the guide window is not hidden at load")
	}
	if !strings.Contains(page, `/static/css/device-guide.css?v={{.AssetVersion}}`) {
		t.Errorf("settings.html.tmpl does not link device-guide.css")
	}
	if strings.Count(page, `template "device_guide"`) != 1 {
		t.Errorf("the guide partial is included %d times, want once", strings.Count(page, `template "device_guide"`))
	}
	// The Home area note says what removal also turns off (W1).
	if !strings.Contains(page, "Removing it also turns off alerts about reports near your home area.") {
		t.Errorf("the Home area window lacks the W1 note")
	}
	for _, dash := range []rune{0x2013, 0x2014} {
		if strings.ContainsRune(page, dash) {
			t.Errorf("settings.html.tmpl holds the dash character U+%04X", dash)
		}
	}
}

// TestNotificationsDeviceStates pins the four variants of the On this device
// row: the switch, the install button, and the two plain sentences.
func TestNotificationsDeviceStates(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	region := settingsSectionRegion(t, page, "settings-phone-title")
	// The switch variant shows by default (ready, unchecked); the script chooses.
	if tag := openingTagOf(t, region, "settings-device-on"); strings.Contains(tag, "hidden") {
		t.Errorf("the switch variant is hidden at load: %s", tag)
	}
	if tag := openingTagOf(t, region, "settings-device-switch"); strings.Contains(tag, "checked") || strings.Contains(tag, "disabled") {
		t.Errorf("the device switch starts checked or disabled: %s", tag)
	}
	install := openingTagOf(t, region, "settings-device-install")
	for _, want := range []string{`<button type="button" class="place-row"`, `aria-haspopup="dialog"`, "hidden"} {
		if !strings.Contains(install, want) {
			t.Errorf("the needs install row lacks %q: %s", want, install)
		}
	}
	for _, id := range []string{"settings-device-blocked", "settings-device-unsupported"} {
		tag := openingTagOf(t, region, id)
		if !strings.HasPrefix(tag, `<div class="place-row"`) || !strings.Contains(tag, "hidden") {
			t.Errorf("%s is not a hidden plain row: %s", id, tag)
		}
	}
	// Slice each variant out of the device wrapper.
	wrapper := region[strings.Index(region, `id="settings-device-row"`):strings.Index(region, `id="settings-critical-row"`)]
	variant := func(id, next string) string {
		from := strings.Index(wrapper, `id="`+id+`"`)
		to := len(wrapper)
		if next != "" {
			to = strings.Index(wrapper, `id="`+next+`"`)
		}
		return wrapper[from:to]
	}
	installBody := variant("settings-device-install", "settings-device-blocked")
	blockedBody := variant("settings-device-blocked", "settings-device-unsupported")
	unsupportedBody := variant("settings-device-unsupported", "")
	if !strings.Contains(installBody, `<span class="place-row__sub">Add Pinalert to your Home Screen first</span>`) || !strings.Contains(installBody, "app-icon--chevron-right") {
		t.Errorf("the needs install row lacks its sub line or its chevron")
	}
	if !strings.Contains(blockedBody, `<span class="place-row__sub">Blocked in your browser settings. Allow notifications for Pinalert there, then come back.</span>`) {
		t.Errorf("the blocked row lacks its sub line")
	}
	if !strings.Contains(unsupportedBody, `<span class="place-row__sub">This browser can't show notifications. Updates still show in the bell.</span>`) {
		t.Errorf("the unsupported row lacks its sub line")
	}
	for name, body := range map[string]string{"blocked": blockedBody, "unsupported": unsupportedBody} {
		for _, banned := range []string{"app-icon--chevron-right", "<button", "<input", "<a "} {
			if strings.Contains(body, banned) {
				t.Errorf("the %s row holds %q; it is a plain row, not a control", name, banned)
			}
		}
	}
}

// TestNotificationsNoFollowOrOfficialRow pins decision 12 (owner amendment
// 2026-10-18): no row for Severe official alerts (Phase 5's slot, not drawn)
// and none for Someone I follow posts (Phase 9 puts a Notify me bell on a
// profile). The scan covers the raw template, comments included.
func TestNotificationsNoFollowOrOfficialRow(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	for _, banned := range []string{"Someone I follow posts", "Severe official alerts for my area", "Severe official alerts"} {
		if strings.Contains(page, banned) {
			t.Errorf("settings.html.tmpl holds %q; that row does not come to Settings", banned)
		}
	}
}
