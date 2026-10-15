package web

// Plan 03.1-11 contract tests: the tools over the map (place search and
// category chips) and the map side support for them. Like the other contract
// files these are static inspection of the embedded bytes; nothing here needs
// a database or a server, and none can prove how a browser lays a control out
// (the width pass at 320, 360, 390 and 1280 px is the orchestrator's browser
// check). Every helper and test name carries the p31m prefix where a helper
// is needed, so a later change can add its own without a clash.

import (
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
}
