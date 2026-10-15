package web

// Plan 03.1-11 contract tests: the tools over the map (place search and
// category chips) and the map side support for them. Like the other contract
// files these are static inspection of the embedded bytes; nothing here needs
// a database or a server, and none can prove how a browser lays a control out
// (the width pass at 320, 360, 390 and 1280 px is the orchestrator's browser
// check). Every helper and test name carries the p31m prefix where a helper
// is needed, so a later change can add its own without a clash.

import (
	"regexp"
	"strings"
	"testing"
)

// TestPlaceSearchScript pins the map side of the place search (HIST-05, D-14):
// the map can move to a searched place and back to the reader, the popup
// reserve is measured from the tools layer, and no map move or zoom ever
// reads the list again. The script side of the same test (one request, no
// type ahead) is added with map-tools.js.
func TestPlaceSearchScript(t *testing.T) {
	mapJS := p19Code(t, "static/js/map.js")

	for _, name := range []string{"moveTo(lat, lon, zoom)", "recenterOnReader()", "getCenter()", "locateReader(onFix, onFail)", "toolsReserve()"} {
		jsFunctionBody(t, mapJS, name)
	}
	p13MustContain(t, "map.js exports", mapJS,
		"moveTo: moveTo", "recenterOnReader: recenterOnReader", "getCenter: getCenter")

	// moveTo puts the target below the tools layer: the target is shifted by
	// half the measured reserve, never a literal, and it never reads the list.
	move := jsFunctionBody(t, mapJS, "moveTo(lat, lon, zoom)")
	p13MustContain(t, "moveTo", move, "toolsReserve()", "map.project(", "map.unproject(", "map.setView(")
	p13MustNotContain(t, "moveTo", move, "fetchReports", "Pinalert.setCenter", "startPolling")

	// recenterOnReader reruns the geolocation step, stores the centre with
	// the own flag, moves the map, reads the list once and resolves { own }.
	back := jsFunctionBody(t, mapJS, "recenterOnReader()")
	p13Ordered(t, back, "recenterOnReader",
		"Pinalert.setCenter(lat, lon, own)", "moveTo(lat, lon, zoom)",
		"Pinalert.fetchReports()", "resolve({ own: own })", "locateReader(")
	p13MustContain(t, "recenterOnReader", back, "new Promise(", "Pinalert.config.fallbackLat", "Pinalert.config.fallbackLon")
	// A second polling loop would double every poll.
	p13MustNotContain(t, "recenterOnReader", back, "startPolling", "boot(")

	// getCenter wraps the longitude (the map is unbounded sideways and the
	// server refuses a longitude outside 180) and answers null with no map.
	centre := jsFunctionBody(t, mapJS, "getCenter()")
	p13MustContain(t, "getCenter", centre, "map.getCenter().wrap()", "return null", "lat:", "lon:")

	// The one geolocation step is shared by the first centring and the return.
	locate := jsFunctionBody(t, mapJS, "locateReader(onFix, onFail)")
	p13MustContain(t, "locateReader", locate,
		"navigator.geolocation", "getCurrentPosition(onFix, onFail, { timeout: 8000 })", "onFail()")
	visitor := jsFunctionBody(t, mapJS, "centerOnVisitor()")
	p13MustContain(t, "centerOnVisitor", visitor, "locateReader(")
	if n := strings.Count(mapJS, "navigator.geolocation.getCurrentPosition("); n != 1 {
		t.Errorf("map.js has %d getCurrentPosition calls, want exactly 1 (locateReader)", n)
	}

	// Dragging or zooming the map never reloads the list (D-14). The only
	// reads are the first centring and the return to the reader.
	if n := strings.Count(mapJS, "Pinalert.fetchReports()"); n != 2 {
		t.Errorf("map.js calls Pinalert.fetchReports() %d times, want 2 (the first centring and the return to the reader)", n)
	}
	for _, name := range []string{"onMapMoveEnd()", "onMapResize()"} {
		p13MustNotContain(t, name, jsFunctionBody(t, mapJS, name), "fetchReports", "setCenter(")
	}
	if strings.Contains(mapJS, "'zoomend'") || strings.Contains(mapJS, "'dragend'") {
		t.Errorf("map.js listens for zoomend or dragend, a map move must never read the list (D-14)")
	}
	if n := strings.Count(mapJS, "map.on('moveend'"); n != 1 {
		t.Errorf("map.js has %d moveend listeners, want exactly 1 (the popup keep in view pan)", n)
	}

	// The popup top reserve is the larger of the existing derivation and the
	// measured bottom of the tools layer plus one small space.
	refresh := jsFunctionBody(t, mapJS, "refreshPanPadding()")
	p13MustContain(t, "refreshPanPadding", refresh, "Math.max(top, toolsReserve())")
	reserve := jsFunctionBody(t, mapJS, "toolsReserve()")
	p13MustContain(t, "toolsReserve", reserve,
		"document.querySelector('.map-tools')", "getBoundingClientRect()", "readSpaceToken('--space-sm')", "return 0;")
	p13MustNotContain(t, "toolsReserve", reserve, "112", "104")

	// The popup distance is the store's sentence (C20, C21).
	update := jsFunctionBody(t, mapJS, "updatePopupCard(card, report)")
	p13MustContain(t, "updatePopupCard", update, "Pinalert.distanceText(report)")
	p13MustNotContain(t, "updatePopupCard", update, "Pinalert.formatDistance(", "Pinalert.feedIsAroundReader()")

	// The script side of the place search is pinned by TestPlaceSearchToolsScript.
	p31mSearchScript(t)
}

// p31mTools returns map-tools.js with whole line comments removed, so a
// comment naming a call cannot change a count.
func p31mTools(t *testing.T) string {
	t.Helper()
	return p19Code(t, "static/js/map-tools.js")
}

// p31mSearchScript pins the script side of the place search (HIST-05, D-14,
// D-16): one request, nothing sent while typing, a three letter minimum, a
// stale response guard, every string through the text sink, and the pick path
// that moves the map and reads the list once.
func p31mSearchScript(t *testing.T) {
	t.Helper()
	raw := p13ReadStatic(t, "static/js/map-tools.js")
	code := p31mTools(t)

	// Exactly one request, to the reader lane route. Both counts read the raw
	// file, comments included, so no comment may name either.
	if n := strings.Count(raw, "fetch("); n != 1 {
		t.Errorf("map-tools.js has %d fetch( occurrences, want exactly 1", n)
	}
	if n := strings.Count(raw, "/api/geocode/place"); n != 1 {
		t.Errorf("map-tools.js names /api/geocode/place %d times, want exactly 1", n)
	}
	if n := strings.Count(code, "new AbortController("); n != 1 {
		t.Errorf("map-tools.js has %d AbortController constructions, want 1", n)
	}

	search := jsFunctionBody(t, code, "search(query)")
	p13MustContain(t, "search", search,
		"encodeURIComponent(q)", "PinalertMap.getCenter()", "'&lat='", "'&lon='",
		"Array.from(q).length < MIN_QUERY_RUNES", "if (inFlight", "controller.signal", "mySeq !== searchSeq")
	p13Ordered(t, search, "search",
		"if (inFlight", "Array.from(q).length < MIN_QUERY_RUNES", "inFlight = true;", "fetch(")

	// The three letter minimum, the status words (C27 to C32) and the ellipsis
	// written as an escape, the style app.js uses.
	p13MustContain(t, "map-tools.js", code,
		"var MIN_QUERY_RUNES = 3;",
		"'Type at least 3 letters.'",
		"'Searching\\u2026'",
		"'No matches found. Try a nearby town or landmark.'",
		"'Search unavailable. Try again in a moment.'",
		"'Too many searches. Wait a few seconds and try again.'",
		"' results.'", "'1 result.'",
		"var MAX_RESULTS = 5;", "var RESULT_NAME_MAX = 32;", "var RESULT_ZOOM = 13;")

	// No type ahead: the box is read on submit only.
	p13MustNotContain(t, "map-tools.js", code,
		"addEventListener('input'", "addEventListener('keyup'", "addEventListener('keypress'", "setTimeout(")
	p13MustContain(t, "map-tools.js", code, "addEventListener('submit'", "evt.preventDefault()")

	// An aborted request is not an error line, and a late answer is dropped.
	p13MustContain(t, "map-tools.js", code, "'AbortError'", "controller.abort()")
	// Every dismissal cancels the request in flight (Escape, Close and a press
	// outside the search), so a late answer never reopens the list (T-31-53).
	for _, dismissal := range []string{
		"if (!resultsEl.hidden || !statusEl.hidden) {\n      abortSearch();\n      closeResults();",
		"closeBtn.addEventListener('click', function () {\n      abortSearch();\n      closeResults();",
		"!formEl.contains(evt.target)) {\n        abortSearch();\n        closeResults();",
	} {
		if !strings.Contains(code, dismissal) {
			t.Errorf("map-tools.js lacks a dismissal that cancels the request in flight: %q", dismissal)
		}
	}

	// A refusal or a failure shows the server's sentence or the fixed one.
	p13MustContain(t, "map-tools.js", code, "body.error.message", "MESSAGE_UNAVAILABLE")

	// Every string reaches the page through the text sink, and no markup
	// parsing sink exists (T-31-49).
	if n := strings.Count(code, "Pinalert.setText("); n < 6 {
		t.Errorf("map-tools.js calls Pinalert.setText( %d times, want at least 6", n)
	}
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment"} {
		if strings.Contains(code, sink) {
			t.Errorf("map-tools.js holds the markup parsing sink %q (T-31-49)", sink)
		}
	}
	for _, m := range regexp.MustCompile(`\.textContent\s*=\s*([^;]*);`).FindAllStringSubmatch(code, -1) {
		if strings.TrimSpace(m[1]) != "''" {
			t.Errorf("map-tools.js assigns textContent %q, only the empty string is allowed (use Pinalert.setText)", m[1])
		}
	}

	// A result is at most five buttons; the name is cut and the full name is
	// a second line.
	render := jsFunctionBody(t, code, "renderResults(list)")
	p13MustContain(t, "renderResults", render,
		"MAX_RESULTS", "document.createElement('button')", "'map-search__result'",
		"map-search__result-name", "map-search__result-full", "cutName(")

	// A pick: the centre is marked as not the reader's, the place is stored,
	// the map moves to it at zoom 13 and the list is read once. No second
	// network call and no write to the address.
	pick := jsFunctionBody(t, code, "pickResult(i)")
	p13Ordered(t, pick, "pickResult",
		"Pinalert.setCenter(pick.lat, pick.lon, false);",
		"Pinalert.setPlace(",
		"PinalertMap.moveTo(pick.lat, pick.lon, RESULT_ZOOM);",
		"Pinalert.fetchReports();",
		"closeResults();")
	p13MustContain(t, "pickResult", pick, "focusSearch()")

	// Dismissal: Escape closes the results, then the expanded phone row.
	p13MustContain(t, "map-tools.js", code, "evt.key !== 'Escape'", "'(max-width: 899.98px)'")
	closeBody := jsFunctionBody(t, code, "closeResults()")
	p13MustContain(t, "closeResults", closeBody, "resultsEl.hidden = true", "lastResults = []")

	// Nothing about the reader's area outlives the visit (D-18, T-31-52).
	for _, sink := range []string{"localStorage", "sessionStorage", "history.", "window.location", "URLSearchParams", "document.cookie"} {
		if strings.Contains(code, sink) {
			t.Errorf("map-tools.js touches %q, the place and the chip are never stored or written to the address", sink)
		}
	}

	p13MustContain(t, "map-tools.js exports", code,
		"window.PinalertMapTools = (function () {",
		"init: init", "search: search", "pickResult: pickResult", "closeResults: closeResults",
		"renderChips: renderChips", "renderPlaceLine: renderPlaceLine", "renderFilterLine: renderFilterLine")

	// app.js keeps exactly two network calls and feed.js none.
	if n := strings.Count(p13ReadStatic(t, "static/js/app.js"), "fetch("); n != 2 {
		t.Errorf("app.js has %d fetch( occurrences, want 2", n)
	}
	if n := strings.Count(p13ReadStatic(t, "static/js/feed.js"), "fetch("); n != 0 {
		t.Errorf("feed.js has %d fetch( occurrences, want 0", n)
	}
}

// TestMapChips pins the five category chips and the two context lines
// (HIST-06, D-17, D-18, C33 to C38).
func TestMapChips(t *testing.T) {
	code := p31mTools(t)
	tmpl := p13ReadTemplate(t, "index.html.tmpl")

	// Five buttons in the drawn order, All pressed by default, the other four
	// carrying the four chip categories.
	chips := regexp.MustCompile(`<button type="button" class="map-chip" data-category="([a-z_]*)" aria-pressed="(true|false)">([A-Za-z]+)</button>`)
	found := chips.FindAllStringSubmatch(tmpl, -1)
	want := [][3]string{
		{"", "true", "All"}, {"flood", "false", "Flood"}, {"rescue_needed", "false", "Rescue"},
		{"shelter_open", "false", "Shelter"}, {"power_outage", "false", "Power"},
	}
	if len(found) != len(want) {
		t.Fatalf("template holds %d chip buttons, want %d", len(found), len(want))
	}
	for i, w := range want {
		if found[i][1] != w[0] || found[i][2] != w[1] || found[i][3] != w[2] {
			t.Errorf("chip %d is %v, want slug %q pressed %q word %q", i, found[i][1:], w[0], w[1], w[2])
		}
	}
	if strings.Contains(strings.ToLower(tmpl), "severity filter") {
		t.Errorf("template names a severity filter, there is none (D-17)")
	}

	// The chip words come from the store's table, so the line and the chips
	// cannot disagree.
	app := p13ReadStatic(t, "static/js/app.js")
	for _, slug := range []string{"flood", "rescue_needed", "shelter_open", "power_outage"} {
		if !strings.Contains(app, slug+": '") {
			t.Errorf("app.js CHIP_LABELS lacks %q", slug)
		}
	}

	tap := jsFunctionBody(t, code, "onChipPress(evt)")
	p13Ordered(t, tap, "onChipPress",
		"Pinalert.CHIP_LABELS", "Pinalert.setCategory(", "renderChips(Pinalert.state)", "Pinalert.fetchReports()")
	p13MustContain(t, "onChipPress", tap, "=== Pinalert.state.category", "''")

	render := jsFunctionBody(t, code, "renderChips(state)")
	p13MustContain(t, "renderChips", render, "setAttribute('aria-pressed'", "state.category")

	// Rendered from the store on every notification, so the filter empty
	// block's Clear filter resets the chips too.
	p13MustContain(t, "map-tools.js", code,
		"Pinalert.subscribe(", "renderChips(state)", "renderPlaceLine(state)", "renderFilterLine(state)",
		"document.getElementById('filter-empty-clear')")

	// The two lines.
	place := jsFunctionBody(t, code, "renderPlaceLine(state)")
	p13MustContain(t, "renderPlaceLine", place,
		"'Showing reports near '", "'Back to my location'", "state.place", "locationLost", "MESSAGE_LOCATION_LOST")
	p13MustContain(t, "map-tools.js", code,
		"var MESSAGE_LOCATION_LOST = \"Your location isn't available. Showing the default area.\";")
	// Test change on purpose (03.1 review fix IN-07): the order was pinned as
	// "Pinalert.clearPlace()", "PinalertMap.recenterOnReader()", "locationLost = !".
	// The place now stays until the position (or the default area) arrives,
	// so recenterOnReader drops it in the same step as the centre, and the
	// press itself no longer clears it.
	back := jsFunctionBody(t, code, "onBackPress()")
	p13Ordered(t, back, "onBackPress", "PinalertMap.recenterOnReader()", "locationLost = !")
	p13MustNotContain(t, "onBackPress", back, "Pinalert.clearPlace()")
	recenter := jsFunctionBody(t, p19Code(t, "static/js/map.js"), "recenterOnReader()")
	p13Ordered(t, recenter, "recenterOnReader",
		"Pinalert.clearPlace();", "Pinalert.setCenter(lat, lon, own);", "Pinalert.fetchReports();", "resolve({ own: own });")
	filter := jsFunctionBody(t, code, "renderFilterLine(state)")
	p13MustContain(t, "renderFilterLine", filter,
		"'Filtered by '", "Pinalert.CHIP_LABELS", "'Clear'", "'Clear filter'", "state.category")

	// No severity filter, no rescue or critical special case.
	p13MustNotContain(t, "map-tools.js", strings.ToLower(code), "severity", "critical")
}

// TestMapToolsMarkup pins the tools layer in the template (D-26): the layer
// before the map inside the map pane, the search form with its accessible
// names, the chips, the hidden map label and the script order.
func TestMapToolsMarkup(t *testing.T) {
	tmpl := p13ReadTemplate(t, "index.html.tmpl")

	p13Ordered(t, tmpl, "index.html.tmpl",
		`<section class="pane pane--map">`, `class="map-tools"`, `<form class="map-search" role="search"`,
		`id="map-search-open"`, `id="map-search-input"`, `id="map-search-submit"`, `id="map-search-close"`,
		`id="map-search-status"`, `id="map-search-results"`, `class="map-chips"`, `id="map"`,
		`id="map-label"`, `id="fab-report"`, `<section class="pane pane--list">`)

	for _, want := range []string{
		`type="search"`, `autocomplete="off"`, `autocapitalize="off"`, `spellcheck="false"`,
		`enterkeyhint="search"`, `maxlength="200"`, `placeholder="Search for a place"`,
		`aria-label="Search for a place"`, `aria-label="Close search"`,
		`role="status" aria-live="polite"`, `aria-label="Filter by category"`, `role="group"`,
	} {
		if !strings.Contains(tmpl, want) {
			t.Errorf("index.html.tmpl lacks %s in the tools layer", want)
		}
	}
	// The map label is hidden until plan 13 fills it.
	if !strings.Contains(tmpl, `<p class="map-label" id="map-label" hidden></p>`) {
		t.Errorf("index.html.tmpl lacks the hidden map label paragraph")
	}
	// Every button inside the form is a plain button except the one submit,
	// so no press submits by accident.
	start := strings.Index(tmpl, `<form class="map-search"`)
	if start == -1 {
		t.Fatalf("index.html.tmpl has no search form")
	}
	form := tmpl[start:]
	form = form[:strings.Index(form, `</form>`)]
	for _, b := range regexp.MustCompile(`<button[^>]*>`).FindAllString(form, -1) {
		if !strings.Contains(b, `type="button"`) && !strings.Contains(b, `id="map-search-submit"`) {
			t.Errorf("a form button without type=button: %s", b)
		}
	}
	if n := strings.Count(form, `type="submit"`); n != 1 {
		t.Errorf("the search form holds %d submit buttons, want 1", n)
	}

	// map-tools.js loads deferred after map.js and feed.js.
	tag := `<script src="/static/js/map-tools.js?v={{.AssetVersion}}" defer></script>`
	if n := strings.Count(tmpl, tag); n != 1 {
		t.Fatalf("index.html.tmpl holds the map-tools.js tag %d times, want 1", n)
	}
	p13Ordered(t, tmpl, "script order",
		`/static/js/map.js?v=`, `/static/js/feed.js?v=`, `/static/js/map-tools.js?v=`)
}

// p31mRules returns every rule of one stylesheet that has head as one whole
// part of its selector list, in source order.
func p31mRules(t *testing.T, file, head string) []p16CSSRule {
	t.Helper()
	var out []p16CSSRule
	for _, r := range p16ParseCSS(file, p16ReadStatic(t, file)) {
		for _, part := range p16HeadParts(r.Head) {
			if part == head {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// p31mRule returns the effective declarations of one selector in one media
// prelude (empty means top level): every rule that lists the selector as a
// whole part of its head, later declarations winning, as the cascade does for
// rules of equal weight. It fails when no rule matches.
func p31mRule(t *testing.T, file, media, head string) p16CSSRule {
	t.Helper()
	merged := p16CSSRule{File: file, Media: media, Head: head, Decls: map[string]string{}}
	matched := false
	for _, r := range p31mRules(t, file, head) {
		if r.Media != media {
			continue
		}
		matched = true
		for prop, value := range r.Decls {
			merged.Decls[prop] = value
		}
	}
	if !matched {
		t.Fatalf("%s: no rule with head %q in media %q", file, head, media)
	}
	return merged
}

// TestMapToolsCSS pins the declared geometry and look of the tools layer
// (D-26, UI-SPEC "The map pane, over the map", Surfaces 4 and 5). The tokens
// are the scale; the allowed new literals are 320px (the cap of the laptop
// search box) and 1005 (the results list), both from UI-SPEC Spacing. No
// allow list of dimension literals exists in this package, so none was added.
func TestMapToolsCSS(t *testing.T) {
	const feed = "static/css/feed.css"
	const phone = "static/css/phone.css"
	const phoneMedia = "@media (max-width: 899.98px)"

	// The layer: absolute inside the map pane, ignoring presses of its own,
	// left of the layer clears Leaflet's zoom control. It declares no z index
	// itself: a z indexed layer would be one stacking context and would hold
	// the results list under the add button that comes later in the page.
	// Each control carries 1000 and the results list 1005 (UI-SPEC).
	layer := p31mRule(t, feed, "", ".map-tools")
	p16MustDecl(t, ".map-tools", layer, map[string]string{
		"position":       "absolute",
		"top":            "0",
		"left":           "calc(var(--space-2xl) + var(--space-sm))",
		"right":          "var(--space-md)",
		"height":         "var(--map-tools-height)",
		"pointer-events": "none",
	})
	if _, has := layer.Decls["z-index"]; has {
		t.Errorf(".map-tools declares a z index; the layer must not be a stacking context")
	}

	for _, head := range []string{".map-search__field", ".map-chips"} {
		r := p31mRule(t, feed, "", head)
		p16MustDecl(t, head, r, map[string]string{"position": "absolute", "z-index": "1000", "pointer-events": "none"})
	}
	p16MustDecl(t, ".map-chips", p31mRule(t, feed, "", ".map-chips"), map[string]string{
		"top":                   "calc(var(--space-sm) + var(--touch-target-min) + var(--space-sm))",
		"overflow-x":            "auto",
		"overscroll-behavior-x": "contain",
	})
	p16MustDecl(t, ".map-search__field", p31mRule(t, feed, "", ".map-search__field"),
		map[string]string{"top": "var(--space-sm)"})

	// The laptop search box caps at 320px.
	p16MustDecl(t, ".map-search__input", p31mRule(t, feed, "", ".map-search__input"), map[string]string{
		"max-width":      "320px",
		"min-height":     "var(--touch-target-min)",
		"border":         "1px solid var(--color-border)",
		"border-radius":  "8px",
		"background":     "var(--color-bg)",
		"color":          "var(--color-text)",
		"font-size":      "var(--font-size-body)",
		"pointer-events": "auto",
	})

	// Every control is at least the touch target and has the 8px corner.
	controlHeads := []string{
		".map-search__open", ".map-search__submit", ".map-search__close", ".map-chip",
		".map-search__result",
	}
	for _, head := range controlHeads {
		r := p31mRule(t, feed, "", head)
		if got := r.Decls["min-height"]; got != "var(--touch-target-min)" {
			t.Errorf("%s min-height is %q, want the touch target token", head, got)
		}
		if head != ".map-search__result" {
			if got := r.Decls["border-radius"]; got != "8px" {
				t.Errorf("%s border-radius is %q, want 8px (no pill)", head, got)
			}
		}
	}
	for _, head := range []string{".map-search__open", ".map-search__submit", ".map-search__close"} {
		r := p31mRule(t, feed, "", head)
		for _, prop := range []string{"width", "min-width"} {
			if got := r.Decls[prop]; got != "var(--touch-target-min)" {
				t.Errorf("%s %s is %q, want the touch target token (44 by 44)", head, prop, got)
			}
		}
	}

	// A chip at rest and a pressed chip, inverted and semibold. The ring is
	// inset so the scroller does not clip it.
	chip := p31mRule(t, feed, "", ".map-chip")
	p16MustDecl(t, ".map-chip", chip, map[string]string{
		"padding": "0 var(--space-md)", "background": "var(--color-bg)", "color": "var(--color-text)",
		"font-size": "var(--font-size-label)", "pointer-events": "auto",
	})
	on := p31mRule(t, feed, "", `.map-chip[aria-pressed="true"]`)
	p16MustDecl(t, "pressed chip", on, map[string]string{
		"background": "var(--color-text)", "color": "var(--color-bg)", "font-weight": "var(--font-weight-semibold)",
	})
	p16MustDecl(t, "chip focus", p31mRule(t, feed, "", ".map-chip:focus-visible"),
		map[string]string{"outline-offset": "-2px"})
	forced := p31mRule(t, feed, "@media (forced-colors: active)", `.map-chip[aria-pressed="true"]`)
	p16MustDecl(t, "forced colours chip", forced, map[string]string{"border": "2px solid CanvasText"})

	// The results list and its status line sit above the add button and
	// below the bottom bar (1010).
	for _, head := range []string{".map-search__results", ".map-search__status:not(.visually-hidden)"} {
		r := p31mRule(t, feed, "", head)
		p16MustDecl(t, head, r, map[string]string{"z-index": "1005", "pointer-events": "auto"})
	}

	// The map label: not interactive, bottom left.
	label := p31mRule(t, feed, "", ".map-label")
	p16MustDecl(t, ".map-label", label, map[string]string{
		"position": "absolute", "left": "var(--space-md)", "bottom": "var(--space-md)",
		"background": "var(--color-bg)", "border": "1px solid var(--color-border)", "border-radius": "8px",
		"font-size": "var(--font-size-label)", "font-weight": "var(--font-weight-semibold)",
		"pointer-events": "none",
	})

	// The two context lines wrap and keep the touch target height.
	for _, head := range []string{".place-line", ".filter-line"} {
		p16MustDecl(t, head, p31mRule(t, feed, "", head), map[string]string{
			"display": "flex", "flex-wrap": "wrap", "min-height": "var(--touch-target-min)",
			"font-size": "var(--font-size-label)",
		})
	}

	// The tools height token: two rows on a laptop, one on a phone.
	main := p31mRule(t, "static/css/main.css", "@media (min-width: 900px)", ".app-shell > .pane--map")
	if got := p19NoSpace(main.Decls["--map-tools-height"]); got !=
		p19NoSpace("calc(var(--space-sm) + var(--touch-target-min) + var(--space-sm) + var(--touch-target-min))") {
		t.Errorf("laptop --map-tools-height is %q", main.Decls["--map-tools-height"])
	}
	narrow := p31mRule(t, phone, phoneMedia, ".app-shell > .pane--map")
	if got := p19NoSpace(narrow.Decls["--map-tools-height"]); got !=
		p19NoSpace("calc(var(--space-sm) + var(--touch-target-min))") {
		t.Errorf("phone --map-tools-height is %q", narrow.Decls["--map-tools-height"])
	}

	// The phone: one row, right inset clears the theme toggle, and the
	// expanded search takes the full row.
	layerNarrow := p31mRule(t, phone, phoneMedia, ".pane--map > .map-tools")
	p16MustDecl(t, "phone layer", layerNarrow, map[string]string{
		"right": "calc(var(--space-md) + var(--touch-target-min) + var(--space-sm))",
	})
	expanded := p31mRule(t, phone, phoneMedia, `.pane--map > .map-tools[data-search-open="true"]`)
	p16MustDecl(t, "expanded phone layer", expanded, map[string]string{"right": "var(--space-md)"})
	p16MustDecl(t, "phone chips", p31mRule(t, phone, phoneMedia, ".pane--map > .map-tools .map-chips"), map[string]string{
		"top":  "var(--space-sm)",
		"left": "calc(var(--touch-target-min) + var(--space-sm))",
	})
	results := p31mRule(t, phone, phoneMedia, ".pane--map > .map-tools .map-search__results")
	p16MustDecl(t, "phone results", results, map[string]string{
		"position": "fixed", "left": "var(--space-md)", "right": "var(--space-md)",
		"top": "calc(var(--disclaimer-bar-height, 0px) + var(--space-sm) + var(--touch-target-min) + var(--space-xs))",
	})
	if !strings.Contains(results.Decls["max-height"], "100dvh") {
		t.Errorf("phone results max-height %q does not stop above the bottom bar", results.Decls["max-height"])
	}

	// No colour literal anywhere in the added rules, and nothing here
	// declares a font weight or size that is not a token.
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`)
	for _, r := range p16ParseCSS(feed, p16ReadStatic(t, feed)) {
		if !strings.Contains(r.Head, "map-") && !strings.Contains(r.Head, "place-line") && !strings.Contains(r.Head, "filter-line") {
			continue
		}
		for prop, value := range r.Decls {
			if hex.MatchString(value) {
				t.Errorf("%q %s: %q is a colour literal, tokens only", r.Head, prop, value)
			}
			if prop == "font-size" && !strings.HasPrefix(value, "var(--font-size-") {
				t.Errorf("%q font-size %q is not a type size token", r.Head, value)
			}
			if prop == "font-weight" && !strings.HasPrefix(value, "var(--font-weight-") {
				t.Errorf("%q font-weight %q is not a weight token", r.Head, value)
			}
			if strings.Contains(value, "999px") || strings.Contains(value, "50%") && prop == "border-radius" {
				t.Errorf("%q %s: %q is a pill shape", r.Head, prop, value)
			}
		}
	}
}

// TestMapToolsHideUnderSheet pins that the tools, the map label and the
// results step aside while the phone detail sheet is open, through the one
// existing hide rule, and that the theme toggle also steps aside while the
// phone search is open (UI-SPEC "Hidden while the phone detail sheet is open").
func TestMapToolsHideUnderSheet(t *testing.T) {
	const phoneMedia = "@media (max-width: 899.98px)"
	const sheet = `body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open="true"])`
	var hide *p16CSSRule
	for _, r := range p16ParseCSS("static/css/phone.css", p16ReadStatic(t, "static/css/phone.css")) {
		if r.Media == phoneMedia && strings.Contains(r.Head, ".leaflet-control-zoom") {
			rule := r
			hide = &rule
		}
	}
	if hide == nil {
		t.Fatalf("phone.css has no sheet hide rule")
	}
	if hide.Decls["display"] != "none" {
		t.Errorf("the sheet hide rule declares display %q, want none", hide.Decls["display"])
	}
	parts := p16HeadParts(hide.Head)
	for _, want := range []string{
		sheet + " .leaflet-control-zoom",
		sheet + " > .theme-toggle",
		sheet + " .map-tools",
		sheet + " .map-label",
		sheet + " #map-search-results",
		`body:has(.map-tools[data-search-open="true"]) > .theme-toggle`,
	} {
		found := false
		for _, p := range parts {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("the sheet hide rule lacks the head %q", want)
		}
	}
	// One rule, not two: the existing list was extended.
	n := 0
	for _, r := range p16ParseCSS("static/css/phone.css", p16ReadStatic(t, "static/css/phone.css")) {
		if strings.Contains(r.Head, ".leaflet-control-zoom") && r.Decls["display"] == "none" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("phone.css holds %d rules hiding the zoom control, want exactly 1", n)
	}
}

// TestSearchIconIsMasked pins the one new icon (UI-SPEC Design System): the
// file exists and is a well formed stroke glyph, it is exempt from the
// category walk through nonCategoryIcons, and feed.css masks it with the
// literal url on map-search__icon, so the exemption cannot hide an orphan.
func TestSearchIconIsMasked(t *testing.T) {
	const path = "static/icons/search.svg"
	svg := p13ReadStatic(t, path)
	for _, want := range []string{`viewBox="0 0 24 24"`, `stroke="currentColor"`, `stroke-width="2"`, "<circle", "<path"} {
		if !strings.Contains(svg, want) {
			t.Errorf("%s lacks %s", path, want)
		}
	}
	if strings.Contains(svg, "<script") || strings.Contains(svg, "href") {
		t.Errorf("%s holds a script or a link, which a mask glyph never carries (T-03-06)", path)
	}
	ref := p13ReadStatic(t, "static/icons/x.svg")
	refFirst, _, _ := strings.Cut(ref, "\n")
	first, _, _ := strings.Cut(svg, "\n")
	if first != refFirst {
		t.Errorf("%s first line %q differs from the licence line of x.svg %q", path, first, refFirst)
	}
	if !nonCategoryIcons[path] {
		t.Errorf("%s is not listed in nonCategoryIcons", path)
	}

	want := `url("/static/icons/search.svg")`
	found := false
	for _, r := range p31mRules(t, "static/css/feed.css", ".map-search__icon") {
		if r.Decls["-webkit-mask-image"] == want && r.Decls["mask-image"] == want {
			found = true
		}
	}
	if !found {
		t.Errorf("feed.css has no .map-search__icon rule declaring both mask images as %s", want)
	}
	// The close glyph reuses x.svg.
	closeWant := `url("/static/icons/x.svg")`
	closeFound := false
	for _, r := range p31mRules(t, "static/css/feed.css", ".map-search__close-icon") {
		if r.Decls["-webkit-mask-image"] == closeWant && r.Decls["mask-image"] == closeWant {
			closeFound = true
		}
	}
	if !closeFound {
		t.Errorf("feed.css has no .map-search__close-icon rule masking x.svg")
	}
}

// TestMapChipsContrast computes the text pairs of the tools from the shipped
// tokens in all four theme sources: a pressed chip (--color-bg on
// --color-text), a chip, the search box and the map label (--color-text on
// --color-bg) and the muted second line of a result (--color-text-muted on
// --color-bg). All reach 4.5 to 1.
func TestMapChipsContrast(t *testing.T) {
	needed := []string{"--color-bg", "--color-text", "--color-text-muted"}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p13ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
	}
	pairs := []struct{ label, fg, bg string }{
		{"pressed chip", "--color-bg", "--color-text"},
		{"chip, search box, map label", "--color-text", "--color-bg"},
		{"result second line, place line", "--color-text-muted", "--color-bg"},
	}
	for _, s := range sources {
		for _, p := range pairs {
			fg := p31Resolve(t, s.tokens, p.fg)
			bg := p31Resolve(t, s.tokens, p.bg)
			if ratio := wcagContrastRatio(t, fg, bg, s.name+" "+p.label); ratio < 4.5 {
				t.Errorf("%s: %s is %.3f:1, want at least 4.5:1", s.name, p.label, ratio)
			}
		}
	}
}

// TestPopupClearsTheToolsOnALaptop pins the popup max height term: from 900px
// the card scrolls inside a box that is one tools layer shorter, and the short
// screen rule still comes last so it wins (UI-SPEC "Popup top reserve").
func TestPopupClearsTheToolsOnALaptop(t *testing.T) {
	const head = ".leaflet-container .leaflet-popup-content"
	laptop := p31mRule(t, "static/css/trust.css", "@media (min-width: 900px)", head)
	flat := p19NoSpace(laptop.Decls["max-height"])
	if !strings.Contains(flat, "-var(--map-tools-height)") || !strings.Contains(flat, "100dvh") {
		t.Errorf("laptop popup max-height %q does not subtract var(--map-tools-height)", laptop.Decls["max-height"])
	}
	all := p16ParseCSS("static/css/trust.css", p16ReadStatic(t, "static/css/trust.css"))
	laptopIdx, shortIdx := -1, -1
	for _, r := range all {
		if r.Head != head {
			continue
		}
		if r.Media == "@media (min-width: 900px)" {
			laptopIdx = r.Index
		}
		if r.Media == "@media (max-height: 359px)" {
			shortIdx = r.Index
		}
	}
	if laptopIdx == -1 || shortIdx == -1 || laptopIdx >= shortIdx {
		t.Errorf("the laptop popup rule (%d) must come before the short screen rule (%d)", laptopIdx, shortIdx)
	}
}
