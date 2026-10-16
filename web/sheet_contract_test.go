package web

// Plan 03.4-13 contract tests: the phone list sheet over the map, the way a
// report or a pin is chosen, and the grey disputed pin (Phase 3.4, D-16, D-17,
// D-04, D-07, D-14, D-03). Like the other contract files these are static
// inspection of the embedded bytes; nothing here needs a database or a server.
// The snap and handoff decisions themselves are driven by sheet_logic_test.mjs.
// Helpers carry the sh13 prefix.

import (
	"regexp"
	"strings"
	"testing"
)

// sh13Rules parses phone.css once per call.
func sh13Rules(t *testing.T) []p13CSSRule {
	t.Helper()
	return p13ParseCSS(p13ReadStatic(t, "static/css/phone.css"))
}

// sh13Phone is the media prelude of the phone block.
const sh13Phone = "@media (max-width: 899.98px)"

// TestSheetHandleIsAButton pins the sheet's markup and its accessible handle
// (UI-SPEC 2b, Interaction and Accessibility): the list pane is a labelled
// region, not a dialog, the handle is a real button 44px high with an expanded
// state and a name that changes with it, Enter and Space reach the click
// handler of the button and the arrow keys step through the positions.
func TestSheetHandleIsAButton(t *testing.T) {
	index := p13ReadTemplate(t, "index.html.tmpl")

	section := regexp.MustCompile(`(?s)<section class="pane pane--list" id="report-sheet" role="region" aria-label="Reports near you" data-snap="half">(.*?)<a class="alert-banner"`)
	m := section.FindStringSubmatch(index)
	if m == nil {
		t.Fatalf("index.html.tmpl has no list pane that is the sheet: id report-sheet, role region, aria-label Reports near you, data-snap half")
	}
	handle := regexp.MustCompile(`<div class="sheet__handle-row">\s*<button type="button" class="sheet__handle" id="sheet-handle" aria-expanded="false"\s+aria-controls="report-sheet" aria-label="Show more of the list">\s*<span class="sheet__grabber" aria-hidden="true"></span>\s*</button>\s*</div>`)
	if !handle.MatchString(m[1]) {
		t.Errorf("the first thing in the sheet must be the handle row holding one real button with aria-expanded, aria-controls and the grabber: %q", m[1])
	}
	if n := strings.Count(index, `id="sheet-handle"`); n != 1 {
		t.Errorf("index.html.tmpl holds %d sheet handles, want 1", n)
	}
	// Not modal: no dialog role, no aria-modal, no inert, no trap anywhere in
	// the list pane.
	pane := vsListPane(t)
	p13MustNotContain(t, "the sheet markup", pane, "aria-modal", "inert", `role="dialog"`)
	// The switch it replaces is gone.
	p13MustNotContain(t, "index.html.tmpl", index, "view-toggle", "data-view")

	rules := sh13Rules(t)
	button, _ := p13Rule(t, rules, sh13Phone, ".sheet__handle-row > .sheet__handle")
	for prop, want := range map[string]string{
		"width":      "100%",
		"height":     "var(--touch-target-min)",
		"min-height": "var(--touch-target-min)",
	} {
		if got := button.Decls[prop]; got != want {
			t.Errorf("the handle %s is %q, want %q (a 44px full width target)", prop, got, want)
		}
	}
	grabber, _ := p13Rule(t, rules, sh13Phone, ".sheet__handle > .sheet__grabber")
	for prop, want := range map[string]string{
		"width":            "40px",
		"height":           "4px",
		"background-color": "var(--color-age-stale)",
	} {
		if got := grabber.Decls[prop]; got != want {
			t.Errorf("the grabber %s is %q, want %q", prop, got, want)
		}
	}
	row, _ := p13Rule(t, rules, sh13Phone, ".pane--list > .sheet__handle-row")
	if got := row.Decls["position"]; got != "sticky" {
		t.Errorf("the handle row position is %q, want sticky", got)
	}
	// The row is drawn on a phone only.
	base, _ := p13Rule(t, rules, "", ".app-bar, .app-topbar__title, .app-topbar__back, .app-topbar__gear, .sheet__handle-row")
	if base.Decls["display"] != "none" {
		t.Errorf("the handle row must be hidden outside the phone block")
	}

	sheet := p19Code(t, "static/js/sheet.js")
	p13MustContain(t, "sheet.js", sheet,
		"var NAME_MORE = 'Show more of the list';", "var NAME_MAP = 'Show the map again';",
		"handleEl.setAttribute('aria-expanded', name === 'full' ? 'true' : 'false');",
		"name === 'full' ? NAME_MAP : NAME_MORE")
	click := jsFunctionBody(t, sheet, "onHandleClick()")
	p13MustContain(t, "onHandleClick", click, "setSnap(snap === 'full' ? 'half' : 'full');")
	key := jsFunctionBody(t, sheet, "onHandleKey(evt)")
	p13MustContain(t, "onHandleKey", key, "'ArrowUp'", "'ArrowDown'", "stepSnap(snap,", "evt.preventDefault();")
	// Enter and Space are the button's own click, so there is no second path.
	p13MustNotContain(t, "onHandleKey", key, "'Enter'", "' '")
	p13MustContain(t, "sheet.js", sheet,
		"handleEl.addEventListener('click', onHandleClick);", "handleEl.addEventListener('keydown', onHandleKey);")
	// A short sheet grows when focus lands on a row it clips, never on a trap.
	focus := jsFunctionBody(t, sheet, "onFocusIn(evt)")
	p13MustContain(t, "onFocusIn", focus, "snap !== 'short'", "setSnap('half')")
	p13MustNotContain(t, "sheet.js", sheet, "inert", "aria-modal", "aria-hidden", "tabindex", ".focus(")
}

// TestSheetReducedMotion pins D-16's motion rule: the sheet and the map above
// it ease 0.3s, none while a finger drags, and none under reduced motion, where
// the script also settles at once.
func TestSheetReducedMotion(t *testing.T) {
	rules := sh13Rules(t)

	sheet, _ := p13Rule(t, rules, sh13Phone, ".app-shell > .pane--list")
	if got := sheet.Decls["transition"]; got != "top 0.3s cubic-bezier(.2, .8, .2, 1)" {
		t.Errorf("the sheet transition is %q, want top 0.3s cubic-bezier(.2, .8, .2, 1)", got)
	}
	mapBox, _ := p13Rule(t, rules, sh13Phone, ".pane--map #map")
	if got := mapBox.Decls["transition"]; got != "height 0.3s cubic-bezier(.2, .8, .2, 1)" {
		t.Errorf("the map transition is %q, want height 0.3s cubic-bezier(.2, .8, .2, 1)", got)
	}
	dragging, _ := p13Rule(t, rules, sh13Phone, ".app-shell[data-dragging] > .pane--list, .app-shell[data-dragging] .pane--map #map")
	if got := dragging.Decls["transition"]; got != "none" {
		t.Errorf("no transition may run while dragging, got %q", got)
	}
	calm, _ := p13Rule(t, rules, "(prefers-reduced-motion: reduce)", ".app-shell > .pane--list, .pane--map #map")
	if got := calm.Decls["transition"]; got != "none" {
		t.Errorf("reduced motion must switch the sheet and map transitions off, got %q", got)
	}
	if !strings.Contains(calm.Media, phoneBlockMedia) {
		t.Errorf("the reduced motion rule sits in %q, want it bounded by the phone query", calm.Media)
	}

	script := p13ReadStatic(t, "static/js/sheet.js")
	p13MustContain(t, "sheet.js", script,
		"var REDUCED_QUERY = '(prefers-reduced-motion: reduce)';", "if (reduced || (opts && opts.instant)) {")
	// The glue reads the query once and again on change.
	p13MustContain(t, "sheet.js", script, "listen(reducedQuery, function () {", "reduced = !!reducedQuery.matches;")
	// No other motion exists in the sheet script.
	p13MustNotContain(t, "sheet.js", p19Code(t, "static/js/sheet.js"), "animate(", ".animate", "behavior: 'smooth'")
}

// TestSheetNoPageScroll pins the "never trap or move the page" rule: the sheet
// is a sibling above the map, it scrolls only its own element, and none of the
// three scripts calls a scroll into view method or a window, document or body
// scroll API. Gesture code holds the page back only through the sheet's own
// scroller and overscroll behaviour.
func TestSheetNoPageScroll(t *testing.T) {
	banned := []string{
		"scrollIntoView", "scrollIntoViewIfNeeded", "window.scrollTo", "window.scrollBy", "window.scroll(",
		"window.scrollY", "window.pageYOffset", "document.body.scroll", "document.documentElement.scroll",
		"document.scrollingElement", "scrollingElement", "body.scrollTop", "documentElement.scrollTop",
	}
	for _, path := range []string{"static/js/sheet.js", "static/js/feed.js", "static/js/map.js"} {
		p13MustNotContain(t, path, p19Code(t, path), banned...)
	}

	// The sheet is a sibling of the map element, never its child.
	index := p13ReadTemplate(t, "index.html.tmpl")
	mapAt := strings.Index(index, `<div id="map"></div>`)
	paneEnd := strings.Index(index, `</section>`)
	sheetAt := strings.Index(index, `<section class="pane pane--list" id="report-sheet"`)
	if mapAt == -1 || paneEnd == -1 || sheetAt == -1 || !(mapAt < paneEnd && paneEnd < sheetAt) {
		t.Errorf("the sheet must start after the map pane closes (map at %d, map pane ends %d, sheet at %d)", mapAt, paneEnd, sheetAt)
	}

	rules := sh13Rules(t)
	sheet, _ := p13Rule(t, rules, sh13Phone, ".app-shell > .pane--list")
	for prop, want := range map[string]string{
		"overscroll-behavior": "contain",
		"overflow-y":          "auto",
		"z-index":             "1020",
		"position":            "absolute",
	} {
		if got := sheet.Decls[prop]; got != want {
			t.Errorf("the sheet %s is %q, want %q", prop, got, want)
		}
	}
	page, _ := p13Rule(t, rules, sh13Phone, "body.has-app-bar:has(> .app-shell)")
	if page.Decls["overflow"] != "hidden" || page.Decls["overscroll-behavior"] != "none" {
		t.Errorf("the Nearby page frame must not scroll as a document: %v", page.Decls)
	}
	// touch-action none belongs to the handle row and the list head only; the
	// detail sheet keeps its pan-y.
	for _, r := range rules {
		if r.Decls["touch-action"] != "none" {
			continue
		}
		if r.Head != ".pane--list > .sheet__handle-row" && r.Head != ".pane--list > .list-head" {
			t.Errorf("phone.css rule %q sets touch-action none, only the handle row and the list head may", r.Head)
		}
	}

	// The touch handler is not passive, so a gesture the sheet owns can be
	// taken from the browser, and it asks gestureOwner on the first move.
	script := p19Code(t, "static/js/sheet.js")
	p13MustContain(t, "sheet.js", script,
		"sheetEl.addEventListener('touchmove', onTouchMove, { passive: false });",
		"gestureOwner(snap, sheetEl.scrollTop, dy)")
	move := jsFunctionBody(t, script, "onTouchMove(evt)")
	p13Ordered(t, move, "onTouchMove", "touch.owner === null", "gestureOwner(", "touch.owner !== 'sheet'", "evt.preventDefault();")
}
