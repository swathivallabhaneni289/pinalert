package web

// Phase 3.4 plan 39: the phone report view and the report address.
//
// On a phone an opened report fills the screen inside Nearby: a top row with
// Back, the title Report, the bell and the gear, a short map strip that keeps
// every pin, then the report, scrolling to its last control above the bottom
// bar. The laptop keeps its popup over the map exactly as it was (owner,
// 2026-10-19: "the laptop ver looks good i dont want you to disturb that").
// On both sizes an open report has its own address, /?report=ID, and
// Pinalert.openReport(id) is the one public way in. These tests read the
// embedded bytes, like the other contract tests.

import (
	"strings"
	"testing"
)

const p39PopupHead = ".leaflet-container .leaflet-detail-pane .leaflet-popup"

func p39Rules(t *testing.T, media, head string) []p16CSSRule {
	t.Helper()
	var found []p16CSSRule
	for _, r := range phoneRules(t) {
		if r.Head == head && r.Media == media {
			found = append(found, r)
		}
	}
	return found
}

func p39Decl(t *testing.T, media, head, prop string) string {
	t.Helper()
	rules := p39Rules(t, media, head)
	if len(rules) == 0 {
		t.Fatalf("phone.css has no rule %q in %q", head, media)
	}
	for _, r := range rules {
		if v, ok := r.Decls[prop]; ok {
			return v
		}
	}
	return ""
}

// TestReportViewTemplate pins the not available view: its own Back with the
// name VoiceOver reads, the heading Report that takes focus, and the line.
func TestReportViewTemplate(t *testing.T) {
	raw, err := TemplatesFS.ReadFile("templates/index.html.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	p13Ordered(t, page, "index.html.tmpl report gone",
		`<section class="report-gone" id="report-gone" aria-labelledby="report-gone-title" hidden>`,
		`<button type="button" class="report-gone__back" id="report-gone-back" aria-label="Back">`,
		`<h2 class="report-gone__title" id="report-gone-title" tabindex="-1">Report</h2>`,
		`<p class="report-gone__line">This report is no longer available</p>`)
	shell := strings.Index(page, `<div class="app-shell" id="app-shell"`)
	gone := strings.Index(page, `id="report-gone"`)
	bottom := strings.Index(page, `{{template "app_bottombar" .}}`)
	if !(shell >= 0 && shell < gone && gone < bottom) {
		t.Errorf("the not available view must sit inside the app shell, before the bottom bar")
	}
}

// TestReportViewPhoneStylesheet pins the phone layout: the report starts under
// the top row and a 160px map strip, stops at the bottom bar, the Back control
// and the title Report sit in the top row, and the not available view fills
// the same room. Nothing new is drawn at 900px and up.
func TestReportViewPhoneStylesheet(t *testing.T) {
	// Amended 2026-10-21 by plan 40: the strip starts at the top of the page
	// under the floating controls; top was
	// calc(var(--app-topbar-height) + var(--space-3xl) * 2.5).
	want := map[string]string{
		"position":  "fixed",
		"top":       "calc(var(--space-3xl) * 2.5)",
		"right":     "0",
		"overflow":  "visible",
		"left":      "0 !important",
		"bottom":    "calc(var(--app-bottombar-height) + env(safe-area-inset-bottom)) !important",
		"transform": "none !important",
	}
	for prop, value := range want {
		if got := p39Decl(t, phoneBlockMedia, p39PopupHead, prop); got != value {
			t.Errorf("phone report view %s is %q, want %q", prop, got, value)
		}
	}
	// Amended 2026-10-21: the wrapper is the scroller, so Safari on the iPhone
	// does not clip Back and Report, which sit outside it in the top row.
	for prop, value := range map[string]string{"height": "100%", "overflow-y": "auto", "overscroll-behavior": "contain", "touch-action": "pan-y"} {
		if got := p39Decl(t, phoneBlockMedia, ".leaflet-container .leaflet-detail-pane .leaflet-popup-content-wrapper", prop); got != value {
			t.Errorf("phone report view wrapper %s is %q, want %q", prop, got, value)
		}
	}

	short := p39Rules(t, "@media (max-width: 899.98px) and (max-height: 359px)", p39PopupHead)
	if len(short) != 1 || short[0].Decls["top"] != "calc(var(--disclaimer-bar-height) + var(--app-topbar-height))" {
		t.Errorf("a short landscape phone drops the strip and starts the report under the top row, got %v", short)
	}

	// Amended 2026-10-21 by plan 40: Back floats over the strip as a round
	// control with the page colour and the menu shadow, 12px in from the left,
	// where it was a see through square at 4px (left var(--space-xs),
	// background transparent, box-shadow none) in a white top row.
	back := p39PopupHead + " a.leaflet-popup-close-button:not([hidden])"
	for prop, value := range map[string]string{
		"position": "fixed", "top": "var(--space-sm)", "left": "var(--space-12)",
		"border-radius": "50%", "background": "var(--color-bg)", "box-shadow": "var(--shadow-menu)",
	} {
		if got := p39Decl(t, phoneBlockMedia, back, prop); got != value {
			t.Errorf("phone Back %s is %q, want %q", prop, got, value)
		}
	}

	// Amended 2026-10-21 by plan 40: the title Report is no longer drawn as a
	// row beside Back; it stays in the popup for screen readers, clipped like
	// .visually-hidden, and still takes focus on open (display block, outline
	// none). It was drawn fixed at the top row: top var(--space-sm), left
	// calc(var(--space-xs) + var(--touch-target-min) + var(--space-xs)),
	// height and line-height var(--touch-target-min), the heading font size
	// and weight.
	title := p39PopupHead + " .report-view__title"
	for prop, value := range map[string]string{
		"display": "block", "position": "absolute",
		"width":    "1px",
		"height":   "1px",
		"overflow": "hidden",
		"clip":     "rect(0, 0, 0, 0)",
		"outline":  "none",
	} {
		if got := p39Decl(t, phoneBlockMedia, title, prop); got != value {
			t.Errorf("phone title %s is %q, want %q", prop, got, value)
		}
	}

	gone := ".app-shell > .report-gone:not([hidden])"
	for prop, value := range map[string]string{
		"display": "block", "position": "fixed", "top": "var(--app-topbar-height)",
		"bottom":  "calc(var(--app-bottombar-height) + env(safe-area-inset-bottom))",
		"z-index": "1080",
	} {
		if got := p39Decl(t, phoneBlockMedia, gone, prop); got != value {
			t.Errorf("phone not available view %s is %q, want %q", prop, got, value)
		}
	}

	// The two new parts are hidden outside the phone block, so the laptop has
	// nothing new in it.
	var titleHidden, goneHidden bool
	for _, r := range phoneRules(t) {
		if r.Media != "" {
			continue
		}
		if strings.Contains(r.Head, ".report-view__title") && r.Decls["display"] == "none" {
			titleHidden = true
		}
		if strings.Contains(r.Head, ".report-gone") && r.Decls["display"] == "none" {
			goneHidden = true
		}
	}
	if !titleHidden || !goneHidden {
		t.Errorf("the title Report and the not available view must be display none outside the phone block (title %v, gone %v)", titleHidden, goneHidden)
	}

	// While the view or the not available line is open the Nearby search steps
	// aside, so the top row reads Back, Report, the bell and the gear.
	var searchAside, goneSearchAside bool
	for _, r := range phoneRules(t) {
		if r.Media != phoneBlockMedia || r.Decls["visibility"] != "hidden" {
			continue
		}
		if strings.Contains(r.Head, `body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open="true"]) .app-topbar > .map-search`) {
			searchAside = true
		}
		if strings.Contains(r.Head, `html[data-report-gone] .app-topbar > .map-search`) {
			goneSearchAside = true
		}
	}
	if !searchAside || !goneSearchAside {
		t.Errorf("the search must step aside under the report view (%v) and the not available view (%v)", searchAside, goneSearchAside)
	}

	// The bell's list hangs over the view, so the top chrome stacks above the
	// view (1080) and below the dialogs (1100) while either view is open, and
	// lets presses through to Back and Report, which share its row.
	chrome := `body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open="true"]) > .top-chrome, html[data-report-gone] body > .top-chrome`
	if got := p39Decl(t, phoneBlockMedia, chrome, "z-index"); got != "1090" {
		t.Errorf("the top chrome over an open view has z-index %q, want 1090", got)
	}
	if got := p39Decl(t, phoneBlockMedia, chrome, "pointer-events"); got != "none" {
		t.Errorf("the top chrome over an open view has pointer-events %q, want none", got)
	}
	var bellTakesPresses bool
	for _, r := range phoneRules(t) {
		if r.Media == phoneBlockMedia && r.Decls["pointer-events"] == "auto" &&
			strings.Contains(r.Head, "> .top-chrome .app-topbar__bell") && strings.Contains(r.Head, "> .top-chrome .bell-list") &&
			strings.Contains(r.Head, "> .top-chrome .app-topbar__gear") {
			bellTakesPresses = true
		}
	}
	if !bellTakesPresses {
		t.Errorf("the bell, the gear and the bell list must take presses over an open view")
	}
}

// TestReportViewMapScript pins map.js: the address helpers live in one
// section, the popup and the close tell it about every open and close, the
// phone gets the title and the Back name, the browser's back and forward are
// heard, and openReport is public.
func TestReportViewMapScript(t *testing.T) {
	code := p19Code(t, "static/js/map.js")

	open := jsFunctionBody(t, code, "onPopupOpen(evt)")
	p13Ordered(t, open, "onPopupOpen",
		"close.setAttribute('aria-label', phoneDetail() ? BACK_LABEL : CLOSE_LABEL);",
		"activePopup = popup;",
		"noteReportOpened(id);",
		"if (phoneDetail()) {",
		"centerPinInStrip(popup, container);",
		"focusReportHeading(container);")

	closeBody := jsFunctionBody(t, code, "onPopupClose(evt)")
	p13MustContain(t, "onPopupClose", closeBody, "noteReportClosed();")

	render := jsFunctionBody(t, code, "render(state)")
	p13Ordered(t, render, "render", "openFocusOnce(state);", "noteFocusAnswer(state);")

	initBody := jsFunctionBody(t, code, "init()")
	p13MustContain(t, "init", initBody,
		"window.addEventListener('popstate', onReportPopState);",
		"wireReportGone();")

	opener := jsFunctionBody(t, code, "openReport(id)")
	p13Ordered(t, opener, "openReport", "REPORT_ID_TEXT.test(key)", "markers[key]", "return false;", "flyTo(key);",
		"Pinalert.select(key, 'map');")
	pop := jsFunctionBody(t, code, "onReportPopState()")
	p13MustContain(t, "onReportPopState", pop, "Pinalert.openReport(key);", "closeFromHistory = true;", "map.closePopup();")
	p13MustContain(t, "map.js", code, "openReport: openReport", "var BACK_LABEL = 'Back';", "var REPORT_TITLE = 'Report';")

	// The laptop paths that must stay as they were.
	p13MustContain(t, "applyPopupPlacement", jsFunctionBody(t, code, "applyPopupPlacement(popup, card)"),
		"popup.options.pane = phone ? 'detailPane' : 'popupPane'")
	p13MustNotContain(t, "onPopupOpen", open, "pushState", "replaceState")
}

// TestReportViewStoreEntry pins the one public way in: Pinalert.openReport in
// app.js, which an address on load uses too.
func TestReportViewStoreEntry(t *testing.T) {
	app := p19Code(t, "static/js/app.js")
	body := jsFunctionBody(t, app, "openReport(id)")
	p13Ordered(t, body, "Pinalert.openReport", "window.PinalertMap.openReport(text)", "setFocusId(text);",
		"if (state.hasLoaded || fetchSeq > 0) {", "fetchReports();")
	p13MustContain(t, "app.js", app, "openReport: openReport")

	feed := p19Code(t, "static/js/feed.js")
	p13MustContain(t, "feed.js", feed, "Pinalert.openReport(new URLSearchParams(window.location.search).get(REPORT_PARAM));")
}

// TestNearbyTopControlsFloat pins plan 40 (owner, 2026-10-21, iPhone 17 Pro
// simulator: "there's huge white spaces on the top ... redesign it better").
// On the phone Nearby page the top chrome leaves the flow and floats over the
// map: the bar is see through and lets presses pass, the search field, the
// bell and the gear carry the page colour and the menu shadow and take their
// own presses, the chips row and the zoom control sit under them, and the
// list sheet at full stops under them. Only Nearby (the page with the app
// shell) floats; the bell button's own position is never set here (Phase 3.2
// anchors its count to it), and nothing of this is in the laptop rules.
func TestNearbyTopControlsFloat(t *testing.T) {
	const nearby = "body.has-app-bar:has(> .app-shell) > .top-chrome"
	for prop, value := range map[string]string{
		"position": "fixed", "top": "0", "right": "0", "left": "0", "pointer-events": "none",
	} {
		if got := p39Decl(t, phoneBlockMedia, nearby, prop); got != value {
			t.Errorf("Nearby top chrome %s is %q, want %q", prop, got, value)
		}
	}
	for prop, value := range map[string]string{
		"gap": "var(--space-sm)", "padding": "var(--space-sm) var(--space-12)", "background": "transparent",
	} {
		if got := p39Decl(t, phoneBlockMedia, nearby+" > .app-topbar", prop); got != value {
			t.Errorf("Nearby top bar %s is %q, want %q", prop, got, value)
		}
	}
	presses := nearby + " .map-search, " + nearby + " .app-topbar__bell, " + nearby + " .app-topbar__gear, " + nearby + " .bell-list"
	if got := p39Decl(t, phoneBlockMedia, presses, "pointer-events"); got != "auto" {
		t.Errorf("the search, the bell, the gear and the bell list must take their own presses, got %q", got)
	}
	look := nearby + " .map-search__field, " + nearby + " .app-topbar__bell, " + nearby + " .app-topbar__gear"
	for prop, value := range map[string]string{"background": "var(--color-bg)", "box-shadow": "var(--shadow-menu)"} {
		if got := p39Decl(t, phoneBlockMedia, look, prop); got != value {
			t.Errorf("floating controls %s is %q, want %q", prop, got, value)
		}
	}
	if got := p39Decl(t, phoneBlockMedia, ".pane--map > .map-tools", "top"); got != "var(--app-topbar-height)" {
		t.Errorf("the chips row must sit under the floating controls, top is %q", got)
	}
	if got := p39Decl(t, phoneBlockMedia, ".pane--map .leaflet-top", "top"); got != "var(--app-topbar-height)" {
		t.Errorf("the zoom control must sit under the floating controls, top is %q", got)
	}
	if got := p39Decl(t, phoneBlockMedia, ".app-shell > .pane--list[data-snap]", "top"); got != "max(var(--sheet-top, 45%), var(--app-topbar-height))" {
		t.Errorf("the list sheet at full must stop under the floating controls, top is %q", got)
	}
	for _, r := range phoneRules(t) {
		for _, part := range strings.Split(r.Head, ",") {
			if strings.HasSuffix(strings.TrimSpace(part), ".app-topbar__bell") {
				if _, ok := r.Decls["position"]; ok {
					t.Errorf("phone.css rule %q sets position on the bell button; Phase 3.2 anchors its count to it", r.Head)
				}
			}
		}
		if strings.Contains(r.Head, "body.has-app-bar:has(> .app-shell) > .top-chrome") && r.Media != phoneBlockMedia {
			t.Errorf("the floating top controls must live in the phone block only, found %q in %q", r.Head, r.Media)
		}
	}
}
