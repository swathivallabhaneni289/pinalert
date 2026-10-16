package web

import (
	"strings"
	"testing"
)

// The Phase 4 browser harness (run outside the repo on 2026-10-16) measured
// these problems in Chromium and WebKit. Each test pins the fix of one finding
// so the source cannot drift back while the harness is not run. F-P4-5 is
// pinned in TestPhoneDetailSheetStylesheet (phone_layout_contract_test.go).

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
		"if (clearingTools ||", "phoneDetail()", "map.dragging.moving()",
		"refreshPanPadding();", "applyPanPadding(popup);",
		"getBoundingClientRect()", "panPad.topLeft[1]",
		"if (dy >= 0) {", "toolsTried.count >= TOOLS_TRIES", "toolsTried.count += 1;",
		"clearingTools = true;", "map.panBy([0, dy], { animate: false });", "} finally {", "clearingTools = false;")
	// The pan with no animation fires moveend before it returns; the nested
	// check must stand down, or it sees the card clear, resets the count and
	// the cap against the map's bounds is never reached.
	if n := strings.Count(code, "clearingTools = true;"); n != 1 {
		t.Errorf("map.js sets clearingTools %d times, want 1 (around the one pan)", n)
	}
	p13MustContain(t, "map.js", code, "var clearingTools = false;")
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

// TestHarnessFindingP4_10MenuScrollStaysInTheMenu pins F-P4-10: in WebKit a
// wheel over the open account menu scrolled the page behind it. The menu keeps
// overscroll-behavior: contain, and account-menu.js adds a wheel and a
// touchmove guard (not passive) while the menu is open and removes them on
// close. web/account_menu_scroll_test.mjs runs the guard itself.
func TestHarnessFindingP4_10MenuScrollStaysInTheMenu(t *testing.T) {
	p16MustDecl(t, "account menu", p31mRule(t, "static/css/auth.css", "", "#account-menu:not([hidden])"),
		map[string]string{"overflow-y": "auto", "overscroll-behavior": "contain"})

	code := p19Code(t, "static/js/account-menu.js")
	set := jsFunctionBody(t, code, "setOpen(open)")
	p13Ordered(t, set, "setOpen",
		"if (open) {",
		"panel.addEventListener('wheel', onPanelWheel, { passive: false });",
		"panel.addEventListener('touchstart', onPanelTouchStart, { passive: true });",
		"panel.addEventListener('touchmove', onPanelTouchMove, { passive: false });",
		"} else if (outsideClickBound) {",
		"panel.removeEventListener('wheel', onPanelWheel);",
		"panel.removeEventListener('touchstart', onPanelTouchStart);",
		"panel.removeEventListener('touchmove', onPanelTouchMove);")
	p13MustContain(t, "onPanelWheel", jsFunctionBody(t, code, "onPanelWheel(e)"),
		"e.ctrlKey", "panelCannotScroll(e.deltaY)", "e.preventDefault();")
	p13MustContain(t, "onPanelTouchMove", jsFunctionBody(t, code, "onPanelTouchMove(e)"),
		"panelCannotScroll(deltaY)", "e.preventDefault();")
	p13MustContain(t, "panelCannotScroll", jsFunctionBody(t, code, "panelCannotScroll(deltaY)"),
		"panel.scrollHeight - panel.clientHeight", "panel.scrollTop")
}

// TestHarnessFindingP4_9MessageClearsTheOpenedReport pins F-P4-9: at 1280 by
// 800 the message (921px wide, centred) covered the opened report's Undo. On
// the map page from 900px the message sits in the list half, between the list
// pane's left edge and the column of the report button and the theme toggle,
// so it never meets a report opened in the map half. The width is the band,
// written in tokens only; the phone rule and the other pages are unchanged.
func TestHarnessFindingP4_9MessageClearsTheOpenedReport(t *testing.T) {
	const main = "static/css/main.css"
	const head = ".app-shell > #toast:not([hidden])"
	toast := p31mRule(t, main, "@media (min-width: 900px)", head)
	// Band from 50% + 16 (half the 32px gap) + 16 to 100% - (24 + 44 + 16).
	wantLeft := "calc(75% + (var(--space-xl) / 2 - var(--space-lg) - var(--touch-target-min)) / 2)"
	wantMax := "calc(50% - var(--space-xl) / 2 - var(--space-md) * 2 - var(--space-lg) - var(--touch-target-min))"
	if p19NoSpace(toast.Decls["left"]) != p19NoSpace(wantLeft) {
		t.Errorf("laptop map page message left is %q, want the middle of the list band %q", toast.Decls["left"], wantLeft)
	}
	if p19NoSpace(toast.Decls["max-width"]) != p19NoSpace(wantMax) {
		t.Errorf("laptop map page message max-width is %q, want the list band %q", toast.Decls["max-width"], wantMax)
	}
	for _, prop := range []string{"transform", "animation", "width", "bottom"} {
		if _, ok := toast.Decls[prop]; ok {
			t.Errorf("%s sets %s; the centred transform, the animation, the width and the bottom stay the base rule's", head, prop)
		}
	}

	// The band's ends are the shell gap and the round buttons' column.
	shell := p31mRule(t, main, "@media (min-width: 900px)", ".app-shell")
	if shell.Decls["gap"] != "var(--space-xl)" || shell.Decls["grid-template-columns"] != "1fr 1fr" {
		t.Errorf("the laptop shell is %v; the message band assumes two equal halves 32px apart", shell.Decls)
	}
	p16MustDecl(t, "report button", p31mRule(t, main, "", ".fab"), map[string]string{"right": "var(--space-lg)"})
	p16MustDecl(t, "theme toggle", p31mRule(t, "static/css/auth.css", "", ".theme-toggle"), map[string]string{"right": "var(--space-lg)"})

	// The head comes after the action head of equal weight, so it caps both.
	var actionAt, bandAt = -1, -1
	for _, r := range p16ParseCSS(main, p16ReadStatic(t, main)) {
		switch r.Head {
		case "#toast.toast--action:not([hidden])":
			actionAt = r.Index
		case head:
			bandAt = r.Index
		}
	}
	if actionAt == -1 || bandAt <= actionAt {
		t.Errorf("the laptop message rule (%d) must come after the action rule (%d)", bandAt, actionAt)
	}

	// The map page's message is a direct child of the shell; the other pages'
	// messages are not, so they stay centred.
	pages := findFullPageTemplates(t)
	index := pages["templates/index.html.tmpl"]
	shellAt := strings.Index(index, `<div class="app-shell"`)
	toastAt := strings.Index(index, "\n    <div id=\"toast\"")
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the
	// phone bottom bar is the app_bottombar block, still the last child after
	// the shell. The Phase 3.1 and Phase 4 hands-on tests are walked on this
	// layout after Phase 3.4 is built.
	navAt := strings.Index(index, `{{template "app_bottombar" .}}`)
	if shellAt == -1 || toastAt < shellAt || navAt < toastAt {
		t.Errorf("index.html.tmpl: #toast is not a direct child of .app-shell, so the laptop rule matches nothing")
	}
	for _, page := range []string{"templates/low.html.tmpl", "templates/profile.html.tmpl", "templates/moderate.html.tmpl"} {
		if strings.Contains(pages[page], `class="app-shell"`) {
			t.Errorf("%s now has an .app-shell; check the laptop message rule still only moves the map page's message", page)
		}
	}

	// The phone keeps its own place above the bottom bar.
	phone := p31mRule(t, "static/css/phone.css", "@media (max-width: 899.98px)", "#toast:not([hidden])")
	if p19NoSpace(phone.Decls["bottom"]) != p19NoSpace("calc(var(--touch-target-min) + var(--space-sm) + var(--space-md))") || len(phone.Decls) != 1 {
		t.Errorf("the phone message rule changed: %v", phone.Decls)
	}
}
