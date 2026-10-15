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
	back := jsFunctionBody(t, code, "onBackPress()")
	p13Ordered(t, back, "onBackPress", "Pinalert.clearPlace()", "PinalertMap.recenterOnReader()", "locationLost = !")
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
