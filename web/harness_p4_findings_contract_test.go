package web

import (
	"strings"
	"testing"
)

// The Phase 4 browser harness (run outside the repo on 2026-10-16) measured
// these problems in Chromium and WebKit. Each test pins the fix of one finding
// so the source cannot drift back while the harness is not run. F-P4-5 is
// pinned in TestPhoneDetailSheetCSS (phone_layout_contract_test.go).

// TestHarnessFindingP4_6NoThemeToggleOnAnyPhonePage pins F-P4-6: the toggle
// showed on Low data and Moderate under 900px because those pages do not load
// phone.css, and a press there saved a choice over the laptop one. The hide
// rule lives in chrome.css, which every page that includes the toggle links,
// the laptop toggle keeps its look, and the click handler does nothing on a
// phone width.
func TestHarnessFindingP4_6NoThemeToggleOnAnyPhonePage(t *testing.T) {
	const include = `{{template "theme_toggle.html.tmpl" .}}`
	pages := 0
	for path, text := range findFullPageTemplates(t) {
		if !strings.Contains(text, include) {
			continue
		}
		pages++
		if !strings.Contains(text, "/static/css/chrome.css") {
			t.Errorf("%s includes the theme toggle but does not link chrome.css, which hides it under 900px", path)
		}
		// The hide rule's head is body > .theme-toggle, so the include must be
		// a direct child of body (two space indent, outside .top-chrome).
		if !strings.Contains(text, "\n  "+include+"\n") {
			t.Errorf("%s: the theme toggle include is not a direct child of body, so body > .theme-toggle matches nothing", path)
		}
	}
	if pages < 4 {
		t.Errorf("only %d pages include the theme toggle, want the map, Activity, Low data and Moderate pages", pages)
	}

	hide := p31mRule(t, "static/css/chrome.css", "@media (max-width: 899.98px)", "body > .theme-toggle")
	if hide.Decls["display"] != "none" {
		t.Errorf("chrome.css body > .theme-toggle under 900px declares display %q, want none", hide.Decls["display"])
	}
	for _, r := range p16ParseCSS("static/css/chrome.css", p16ReadStatic(t, "static/css/chrome.css")) {
		if strings.Contains(r.Head, ".theme-toggle") && r.Media != "@media (max-width: 899.98px)" {
			t.Errorf("chrome.css touches the theme toggle outside the phone block (%q in %q); the laptop toggle stays as it is", r.Head, r.Media)
		}
	}
	laptop := p31mRule(t, "static/css/auth.css", "", ".theme-toggle")
	p16MustDecl(t, "laptop theme toggle", laptop, map[string]string{
		"position": "fixed", "display": "flex", "right": "var(--space-lg)", "bottom": "var(--space-lg)",
	})

	code := p19Code(t, "static/js/theme.js")
	const head = "control.addEventListener('click', function () {"
	if n := strings.Count(code, head); n != 1 {
		t.Fatalf("theme.js has %d toggle click handlers, want 1", n)
	}
	body := code[strings.Index(code, head):]
	end := strings.Index(body, "});")
	if end == -1 {
		t.Fatal("theme.js toggle click handler has no end")
	}
	body = body[:end]
	p13Ordered(t, body, "theme toggle click handler",
		"if (isPhone()) {", "return;", "currentMode = MODES[nextIndex];", "persistMode(currentMode);")
}

// TestHarnessFindingP4_7OpenCardNeverSitsUnderTheTools pins F-P4-7: at
// 1000x900 a report opened from its row settled with its close button under
// the place search's Search button. The top reserve already covers the tools
// layer at every width (refreshPanPadding, outside the phone branch); what
// failed was the pan, because Leaflet's autopan runs inside the open and a
// list row press flies the map while it runs. keepCardClearOfTools checks the
// card after every open and every move, at any motion setting, refreshes the
// paddings first, only moves the card down, stands down on the phone sheet
// and while dragging, and gives up after a few tries so the map's bounds and
// it cannot pan back and forth.
func TestHarnessFindingP4_7OpenCardNeverSitsUnderTheTools(t *testing.T) {
	code := p19Code(t, "static/js/map.js")

	refresh := jsFunctionBody(t, code, "refreshPanPadding()")
	if !strings.Contains(refresh, "\n    top = Math.max(top, toolsReserve());") {
		t.Errorf("refreshPanPadding must take the tools reserve at every width, outside the phone branch")
	}

	clear := jsFunctionBody(t, code, "keepCardClearOfTools(popup)")
	p13Ordered(t, clear, "keepCardClearOfTools",
		"phoneDetail()", "map.dragging.moving()",
		"refreshPanPadding();", "applyPanPadding(popup);",
		"getBoundingClientRect()", "panPad.topLeft[1]",
		"if (dy >= 0) {", "toolsTried.count >= TOOLS_TRIES", "map.panBy([0, dy], { animate: false });")
	p13MustNotContain(t, "keepCardClearOfTools", clear, "reducedMotion", "panPad.bottomRight", "dx")
	p13MustContain(t, "map.js", code, "var TOOLS_TRIES = 3;")

	moveEnd := jsFunctionBody(t, code, "onMapMoveEnd()")
	p13Ordered(t, moveEnd, "onMapMoveEnd", "if (activePopup) {", "keepCardClearOfTools(activePopup);")

	open := jsFunctionBody(t, code, "onPopupOpen(evt)")
	p13Ordered(t, open, "onPopupOpen",
		"refreshPanPadding();", "applyPanPadding(popup);",
		"toolsTried = { popup: popup, count: 0 };", "keepCardClearOfTools(popup);",
		"card.focus({ preventScroll: true });")

	if n := strings.Count(code, "keepCardClearOfTools("); n != 3 {
		t.Errorf("map.js has %d keepCardClearOfTools( occurrences, want 3 (one declaration, the open and the move end)", n)
	}
}
