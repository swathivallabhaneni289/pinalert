package web

import (
	"strings"
	"testing"
)

// Quick 261118-srq (owner, 7.1 UAT test 8, 2026-11-18): the first row of About
// Pinalert in Settings is Add to Dock on Safari in a Mac tab, which opens a
// small window with two steps, or Install app where the browser offers a one
// click install. Nothing shows without the scripts. The behaviour is pinned in
// web/settings_app_row_test.mjs and web/settings_alerts_test.mjs; the markup,
// the words and the script loading are pinned here.

// settingsDockWindowRegion returns the markup of the Add to Dock window.
func settingsDockWindowRegion(t *testing.T, page string) string {
	t.Helper()
	at := strings.Index(page, `id="settings-dock-window"`)
	if at == -1 {
		t.Fatalf("settings.html.tmpl has no settings-dock-window")
	}
	end := strings.Index(page[at:], `{{template "reader_dialog.html.tmpl"`)
	if end == -1 {
		t.Fatalf("settings-dock-window is not followed by the reader dialog")
	}
	return page[at : at+end]
}

// TestSettingsAppRowPlaceAndWords pins the row: first in the About Pinalert
// box, above Disclaimer, hidden by default with both variants hidden, the exact
// labels, a chevron on Add to Dock only, no sub line.
func TestSettingsAppRowPlaceAndWords(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	about := settingsSectionRegion(t, page, "settings-about-title")
	box := about[strings.Index(about, `<div class="place-box">`):]
	p421InOrder(t, "the row comes first in About Pinalert", box,
		`<div class="place-box">`, `<div id="settings-app-row" hidden>`,
		`id="settings-dock-row"`, `id="settings-install-row"`, `href="/disclaimer"`, `href="/policy#policy"`, `href="/low"`)
	wrapAt := strings.Index(box, `<div id="settings-app-row" hidden>`)
	wrapEnd := strings.Index(box, `href="/disclaimer"`)
	wrap := box[wrapAt:wrapEnd]
	for _, want := range []string{
		`<button type="button" class="place-row" id="settings-dock-row" aria-haspopup="dialog" hidden>`,
		`<span class="place-row__label">Add to Dock</span>`,
		`<button type="button" class="place-row" id="settings-install-row" hidden>`,
		`<span class="place-row__label">Install app</span>`,
	} {
		if !strings.Contains(wrap, want) {
			t.Errorf("the About Pinalert app row lacks %q", want)
		}
	}
	if strings.Contains(wrap, "place-row__sub") {
		t.Errorf("the app row has a sub line; the plan has none")
	}
	dock := wrap[strings.Index(wrap, `id="settings-dock-row"`):strings.Index(wrap, `id="settings-install-row"`)]
	install := wrap[strings.Index(wrap, `id="settings-install-row"`):]
	if strings.Count(dock, "app-icon--chevron-right") != 1 {
		t.Errorf("Add to Dock opens a window and needs one chevron")
	}
	if strings.Contains(install, "app-icon--chevron-right") || strings.Contains(install, "aria-haspopup") {
		t.Errorf("Install app is a plain button: no chevron and no window")
	}
}

// TestSettingsDockWindow pins the window: built like the install guide window,
// titled Add to Dock, the two steps in the guide's step list with icons from
// the set Settings loads, and no lead line, hint line or button but the X.
func TestSettingsDockWindow(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	win := settingsDockWindowRegion(t, page)
	for _, want := range []string{
		`role="dialog" aria-modal="true" aria-labelledby="settings-dock-title"`,
		`<h2 class="app-window__title" id="settings-dock-title" tabindex="-1">Add to Dock</h2>`,
		`<button type="button" class="app-window__close" id="settings-dock-close" aria-label="Close">`,
		`<ol class="device-guide__steps" role="list">`,
		// Amended 2026-11-18 (owner on the 7.1 UAT: "can we be more specific
		// about what file or like where to locate it?"): step 1 says where File is.
		`In the bar at the top of the screen, click <strong>File</strong>, then <strong>Add to Dock</strong>.`,
		`Click <strong>Add</strong>, then open Pinalert from your Dock.`,
		`app-icon--square-plus`, `app-icon--map-pin`,
	} {
		if !strings.Contains(win, want) {
			t.Errorf("settings-dock-window lacks %q", want)
		}
	}
	if !strings.Contains(page, `<div class="app-window app-window--edit" id="settings-dock-window" hidden>`) {
		t.Errorf("settings-dock-window is not an app-window app-window--edit, hidden by default")
	}
	if got := strings.Count(win, `class="device-guide__step"`); got != 2 {
		t.Errorf("settings-dock-window holds %d steps, want two", got)
	}
	if got := strings.Count(win, "<button"); got != 1 {
		t.Errorf("settings-dock-window holds %d buttons, want the X only", got)
	}
	for _, gone := range []string{"device-guide__lead", "inline-hint", "<a ", "Home Screen"} {
		if strings.Contains(win, gone) {
			t.Errorf("settings-dock-window holds %q; it has the two steps and nothing else", gone)
		}
	}
	// The window sits at the body level, after main, like the other windows.
	p421InOrder(t, "the window is outside main", page, "</main>", `id="settings-dock-window"`)
	// Both icons have their mask rule in a stylesheet Settings links.
	for file, icon := range map[string]string{"css/device-guide.css": ".app-icon--square-plus", "css/app-bar.css": ".app-icon--map-pin"} {
		if !strings.Contains(p421ReadStatic(t, file), icon) {
			t.Errorf("%s lacks %s, so the step tile would be empty", file, icon)
		}
	}
}

// TestSettingsAppRowScripts pins the loading: install-app.js in the head of
// Settings, not deferred, so the browser's offer is not missed; it alone holds
// the window listeners; notify.js answers which row; settings-alerts.js draws
// it.
func TestSettingsAppRowScripts(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	head := page[:strings.Index(page, "</head>")]
	tag := `<script src="/static/js/install-app.js?v={{.AssetVersion}}"></script>`
	if !strings.Contains(head, tag) {
		t.Errorf("the Settings head lacks %q (not deferred)", tag)
	}
	if strings.Count(page, "install-app.js") != 1 {
		t.Errorf("install-app.js must load once, in the head")
	}
	install := stripJSComments(p421ReadStatic(t, "js/install-app.js"))
	for _, want := range []string{
		"window.addEventListener('beforeinstallprompt', onOffer)",
		"window.addEventListener('appinstalled', onInstalled)",
		"kept.prompt()", "window.PinalertInstall = ",
	} {
		if !strings.Contains(install, want) {
			t.Errorf("install-app.js lacks %q", want)
		}
	}
	for _, banned := range []string{"innerHTML", "textContent", "createElement", "fetch(", "localStorage", "sessionStorage", "document.write"} {
		if strings.Contains(install, banned) {
			t.Errorf("install-app.js uses %q; it writes nothing to the page, sends nothing and stores nothing", banned)
		}
	}
	notify := stripJSComments(p421ReadStatic(t, "js/notify.js"))
	for _, want := range []string{"function installRow(", "function installRowHere(", "function isMacSafari(", "installRowHere: installRowHere"} {
		if !strings.Contains(notify, want) {
			t.Errorf("notify.js lacks %q", want)
		}
	}
	alerts := stripJSComments(p421ReadStatic(t, "js/settings-alerts.js"))
	for _, want := range []string{"el('settings-app-row')", "installRowHere(", "windows.dock = ", "api.prompt()"} {
		if !strings.Contains(alerts, want) {
			t.Errorf("settings-alerts.js lacks %q", want)
		}
	}
	if strings.Contains(alerts, "addEventListener('beforeinstallprompt'") || strings.Contains(alerts, "window.addEventListener") {
		t.Errorf("settings-alerts.js adds a window listener; the offer is kept by install-app.js only")
	}
}

// TestSettingsAppRowPlainPunctuation: no em or en dash in the new script, the
// row, the window or their comments.
func TestSettingsAppRowPlainPunctuation(t *testing.T) {
	page := p421ReadTemplate(t, "settings.html.tmpl")
	about := settingsSectionRegion(t, page, "settings-about-title")
	parts := map[string]string{
		"static/js/install-app.js":   p421ReadStatic(t, "js/install-app.js"),
		"the About Pinalert section": about,
		"settings-dock-window":       settingsDockWindowRegion(t, page),
	}
	for name, text := range parts {
		for _, dash := range []rune{0x2013, 0x2014} {
			if strings.ContainsRune(text, dash) {
				t.Errorf("%s holds the dash character U+%04X", name, dash)
			}
		}
	}
}
