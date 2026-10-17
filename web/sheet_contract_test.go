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
	// The base rule hides the row everywhere, so the phone rule must show it.
	if got := row.Decls["display"]; got != "block" {
		t.Errorf("the handle row display is %q on a phone, want block (the base rule hides it)", got)
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
	dragging, _ := p13Rule(t, rules, sh13Phone, ".app-shell[data-dragging] > .pane--list, .app-shell[data-dragging] .pane--map #map, .app-shell[data-dragging] .pane--map > .map-label")
	if got := dragging.Decls["transition"]; got != "none" {
		t.Errorf("no transition may run while dragging, got %q", got)
	}
	label, _ := p13Rule(t, rules, sh13Phone, ".pane--map > .map-label")
	if label.Decls["top"] != "calc(var(--sheet-top, 45%) - var(--space-2xl))" || label.Decls["bottom"] != "auto" {
		t.Errorf("the map label must follow the sheet's edge, not the bottom of the box where the sheet covers it: %v", label.Decls)
	}
	if got := label.Decls["transition"]; got != "top 0.3s cubic-bezier(.2, .8, .2, 1)" {
		t.Errorf("the map label transition is %q, want it to ease with the sheet", got)
	}
	calm, _ := p13Rule(t, rules, "(prefers-reduced-motion: reduce)", ".app-shell > .pane--list, .pane--map #map, .pane--map > .map-label")
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

	// The wheel is the mouse form of the same rule (UI-SPEC 2b: finger up,
	// wheel down). Its listener is not passive so the sheet can keep a turn it
	// owns from the list, and it decides through gestureOwner, the one copy of
	// the rule, with the wheel turned into the finger's direction.
	p13MustContain(t, "sheet.js", script,
		"sheetEl.addEventListener('wheel', onWheel, { passive: false });",
		"gestureOwner(snap, sheetEl.scrollTop, -evt.deltaY)")
	wheel := jsFunctionBody(t, script, "onWheel(evt)")
	p13Ordered(t, wheel, "onWheel", "!active", "gestureOwner(", "evt.preventDefault();", "setSnap(")
	if strings.Count(script, "gestureOwner(") != 3 {
		t.Errorf("gestureOwner must be defined once and asked by the touch and the wheel paths only, found %d mentions", strings.Count(script, "gestureOwner("))
	}

	// The one scroll the list does is on the sheet's own element, found by its
	// id, and only a row that is not already whole under the head is moved.
	feed := p19Code(t, "static/js/feed.js")
	reveal := jsFunctionBody(t, feed, "revealRowInListPane(id)")
	p13MustContain(t, "revealRowInListPane", reveal,
		"document.getElementById('report-sheet')", "sheet.scrollHeight <= sheet.clientHeight", "sheet.scrollTo(",
		"behavior: calm ? 'auto' : 'smooth'")
}

// sh13SelectionRules returns every parsed rule of the stylesheets that style a
// chosen row or a pin, with the file it came from.
type sh13Rule struct {
	File string
	p13CSSRule
}

func sh13SelectionRules(t *testing.T) []sh13Rule {
	t.Helper()
	var all []sh13Rule
	for _, name := range []string{"static/css/main.css", "static/css/feed.css", "static/css/phone.css", "static/css/trust.css"} {
		for _, r := range p13ParseCSS(p13ReadStatic(t, name)) {
			all = append(all, sh13Rule{File: name, p13CSSRule: r})
		}
	}
	return all
}

// TestNoSelectionRing pins D-04 and D-17: a chosen row and a chosen or open pin
// carry no ring. No rule for the selected row, the chosen pin or the open pin
// declares an outline, a dark border or a 2px dark border; the row takes the
// soft --color-surface fill and the pin grows (scale 1.35) with a white edge.
func TestNoSelectionRing(t *testing.T) {
	selection := func(head string) bool {
		return strings.Contains(head, "report-row--selected") || strings.Contains(head, "map-pin--chosen") ||
			strings.Contains(head, "data-popup-open") || strings.Contains(head, "map-pin--highlight")
	}
	var seenRow, seenPin bool
	for _, r := range sh13SelectionRules(t) {
		if !selection(r.Head) {
			continue
		}
		if strings.Contains(r.Head, "leaflet-popup") || strings.Contains(r.Head, "leaflet-detail-pane") ||
			strings.Contains(r.Head, "html[data-popup-open]") || strings.Contains(r.Head, "body:has(") {
			continue // the popup and detail sheet, not a selection mark
		}
		if strings.Contains(r.Head, "report-row--selected") {
			seenRow = true
		}
		if strings.Contains(r.Head, "map-pin--chosen") {
			seenPin = true
		}
		for prop, value := range r.Decls {
			switch {
			case prop == "outline" || strings.HasPrefix(prop, "outline-"):
				t.Errorf("%s rule %q declares %s %q: a chosen item has no ring (D-04)", r.File, r.Head, prop, value)
			case strings.HasPrefix(prop, "border") && strings.Contains(value, "--color-text)"):
				t.Errorf("%s rule %q declares %s %q: no dark border on a chosen item", r.File, r.Head, prop, value)
			case prop == "border" && strings.HasPrefix(value, "2px"):
				t.Errorf("%s rule %q declares a 2px border %q on a chosen item", r.File, r.Head, value)
			case prop == "box-shadow" && strings.Contains(value, "--color-text)"):
				t.Errorf("%s rule %q draws a dark edge %q on a chosen item", r.File, r.Head, value)
			case prop == "opacity":
				t.Errorf("%s rule %q sets opacity %q: a chosen item is never faded (D-07)", r.File, r.Head, value)
			}
		}
	}
	if !seenRow || !seenPin {
		t.Fatalf("the scan never met a chosen row rule (%v) or a chosen pin rule (%v)", seenRow, seenPin)
	}

	// The row: the soft fill in place of its tint, in main.css and with heads
	// that outrank the white provisional and hidden cards in feed.css.
	main := p13ParseCSS(p13ReadStatic(t, "static/css/main.css"))
	row, _ := p13Rule(t, main, "", ".report-row--selected")
	if got := row.Decls["background"]; got != "var(--color-surface)" {
		t.Errorf(".report-row--selected background is %q, want var(--color-surface)", got)
	}
	feed := p13ParseCSS(p13ReadStatic(t, "static/css/feed.css"))
	chosen, _ := p13Rule(t, feed, "", "#report-list .report-row.report-row--selected, #report-list .report-row.vis-provisional.report-row--selected, #report-list .report-row.vis-hidden.report-row--selected")
	if got := chosen.Decls["background"]; got != "var(--color-surface)" {
		t.Errorf("the chosen row heads in feed.css set background %q, want var(--color-surface)", got)
	}

	// The pin grows: laptop badge in main.css, phone dot in phone.css, both
	// 1.35, an edge in the white token and the deeper shadow token.
	pin, _ := p13Rule(t, main, "", ".icon-badge--pin.map-pin--chosen")
	if got := pin.Decls["transform"]; got != "scale(1.35)" {
		t.Errorf("the chosen laptop pin transform is %q, want scale(1.35)", got)
	}
	if got := pin.Decls["box-shadow"]; got != "0 0 0 3px var(--color-pin-edge), var(--shadow-pin-chosen)" {
		t.Errorf("the chosen laptop pin shadow is %q", got)
	}
	dot, _ := p13Rule(t, sh13Rules(t), sh13Phone, "#map .icon-badge--pin.map-pin--chosen")
	if got := dot.Decls["transform"]; got != "translate(-50%, -50%) scale(1.35)" {
		t.Errorf("the chosen phone dot transform is %q, want it centred and scaled 1.35", got)
	}
	if got := dot.Decls["border-color"]; got != "var(--color-pin-edge)" {
		t.Errorf("the chosen phone dot edge is %q, want the white edge token", got)
	}
	base, _ := p13Rule(t, main, "", ".icon-badge--pin")
	if got := base.Decls["transition"]; got != "transform 0.2s ease" {
		t.Errorf("the pin transition is %q, want transform 0.2s ease", got)
	}
	// The grow is instant under reduced motion, in main.css's one block.
	calm, _ := p13Rule(t, main, "(prefers-reduced-motion: reduce)", ".icon-badge--pin")
	if calm.Decls["transition"] != "none" {
		t.Errorf("reduced motion must switch the pin growth off")
	}
	tokens := p13ReadStatic(t, "static/css/main.css")
	p13MustContain(t, "main.css", tokens, "--shadow-pin-chosen: 0 2px 10px rgb(0 0 0 / 45%);", "--color-pin-edge: #FFFFFF;")

	// The old brief ring and the 3px dark open outline are gone.
	p13MustNotContain(t, "phone.css", p13ReadStatic(t, "static/css/phone.css"), "map-pin--highlight")
	p13MustNotContain(t, "map.js", p19Code(t, "static/js/map.js"), "map-pin--highlight")
}

// TestChosenPinScript pins how map.js marks the chosen pin: from the shared
// selection, on the badge so the next poll's new badge keeps it, and the raise
// on the marker (a z-index on the badge would not lift it), plus the sheet's
// restore on close and the resize at settle.
func TestChosenPinScript(t *testing.T) {
	code := p19Code(t, "static/js/map.js")

	badge := jsFunctionBody(t, code, "buildBadgeElement(report)")
	p13MustContain(t, "buildBadgeElement", badge,
		"isChosen(report.id)", "badge.classList.add('map-pin--chosen');",
		"report.view === 'disputed'", "badge.classList.add('icon-badge--disputed');")
	p13MustNotContain(t, "buildBadgeElement", badge, "trust", "_places", "confirmed", ">=", "<=")

	apply := jsFunctionBody(t, code, "applyChosenPin(id)")
	p13MustContain(t, "applyChosenPin", apply,
		"badge.classList.toggle('map-pin--chosen', chosen);", "marker.setZIndexOffset(chosen ? CHOSEN_Z_OFFSET : 0);")
	p13MustContain(t, "init", jsFunctionBody(t, code, "init()"), "applyChosenPin(id);")
	p13MustContain(t, "upsertMarker", jsFunctionBody(t, code, "upsertMarker(report)"),
		"zIndexOffset: isChosen(report.id) ? CHOSEN_Z_OFFSET : 0")

	// The sheet is handed back when the report closes, before anything else of
	// the close runs.
	closeBody := jsFunctionBody(t, code, "onPopupClose(evt)")
	p13Ordered(t, closeBody, "onPopupClose", "window.PinalertSheet.restore();", "var popup = evt.popup;")

	// The map re-measures when the sheet settles, brings the chosen pin in and
	// leaves an open phone detail alone.
	resize := jsFunctionBody(t, code, "resizeToSheet(sheetTop)")
	p13Ordered(t, resize, "resizeToSheet", "map.invalidateSize({ animate: false, pan: false });",
		"(phoneDetail() && activePopup)", "map.panInside(marker.getLatLng()")
	p13MustContain(t, "map.js", code, "resizeToSheet: resizeToSheet")

	// The disputed pin class is read from the same server word as the row's cues.
	feed := p19Code(t, "static/js/feed.js")
	p13MustContain(t, "isDisputed", jsFunctionBody(t, feed, "isDisputed(report)"), "report.view === 'disputed'")
}

// TestDisputedPinGrey pins D-14, D-07 and T-34-57: a disputed pin is grey so it
// matches its Disputed card. The laptop badge is --color-age-stale with a
// --color-text glyph, the phone dot is a plain --color-age-stale dot, the rule
// is keyed on the class map.js sets from the server's view word, and nothing
// fades it.
func TestDisputedPinGrey(t *testing.T) {
	trust := p13ParseCSS(p13ReadStatic(t, "static/css/trust.css"))
	laptop, _ := p13Rule(t, trust, "", "#map .icon-badge--pin.icon-badge--disputed")
	for prop, want := range map[string]string{
		"background":       "var(--color-age-stale)",
		"--badge-glyph-fg": "var(--color-text)",
		"outline":          "0",
	} {
		if got := laptop.Decls[prop]; got != want {
			t.Errorf("the disputed laptop pin %s is %q, want %q", prop, got, want)
		}
	}
	if _, faded := laptop.Decls["opacity"]; faded {
		t.Errorf("the disputed pin is never faded (D-07)")
	}
	phone, _ := p13Rule(t, sh13Rules(t), sh13Phone, "#map .icon-badge--pin.icon-badge--disputed")
	if got := phone.Decls["background"]; got != "var(--color-age-stale)" {
		t.Errorf("the disputed phone dot background is %q, want var(--color-age-stale)", got)
	}
	if _, faded := phone.Decls["opacity"]; faded {
		t.Errorf("the disputed dot is never faded (D-07)")
	}
	// The card's shared hidden badge head is left as it was: transparent with a
	// hairline, for the Activity page and the card's glyph.
	shared, _ := p13Rule(t, trust, "", ".icon-badge.vis-hidden, .vis-hidden .icon-badge")
	if got := shared.Decls["background"]; got != "transparent" {
		t.Errorf("the shared hidden badge head background is %q, it must stay transparent", got)
	}
}
