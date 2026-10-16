package web

import (
	"strings"
	"testing"
)

// TestReportMapTapMovesPinInEveryState pins quick task 261016-tap (backlog
// 999.6, F1 in 02-UAT.md, IN-W10 in 04-REVIEW-WEB.md): the report form's map
// has exactly one click handler, added when the map is built, so a tap moves
// the pin whether location is on, off or denied and a reopen never adds a
// second one. web/report_map_tap_test.mjs runs the real modal.js through each
// location state with a fake Leaflet.
func TestReportMapTapMovesPinInEveryState(t *testing.T) {
	code := p422Code(p422Static(t, "static/js/modal.js"))

	if n := p422Count(code, ".on('click'"); n != 1 {
		t.Fatalf("modal.js adds %d click handlers, want exactly 1", n)
	}
	if n := p422Count(code, "modalMap.on('click', placeMarkerAtTap);"); n != 1 {
		t.Fatalf("modal.js must add placeMarkerAtTap as the map's click handler exactly once, found %d", n)
	}

	// The handler is added inside the one time map build, not on each open.
	build := p422Window(t, code, "if (!modalMap) {", "locationNotice.hidden = true;")
	if !strings.Contains(build, "modalMap = L.map(") {
		t.Fatalf("the map build window does not hold the map construction")
	}
	if !strings.Contains(build, "modalMap.on('click', placeMarkerAtTap);") {
		t.Errorf("the click handler must be added inside initLocation's one time map build")
	}

	// No location branch adds or gates the handler.
	denied := jsFunctionBody(t, code, "showLocationDenied(")
	if strings.Contains(denied, "'click'") {
		t.Errorf("showLocationDenied must not add a click handler; a tap works in every state")
	}
	for _, name := range []string{"startLocation(", "clearMarker(", "chooseSearchResult("} {
		if strings.Contains(jsFunctionBody(t, code, name), "'click'") {
			t.Errorf("%s must not touch the click handler", name)
		}
	}

	// The tap does what a drag does: the form counts it as an edit and the
	// pin and readout move.
	tap := jsFunctionBody(t, code, "placeMarkerAtTap(")
	p422Need(t, "placeMarkerAtTap", tap, "markFormTouched();", "placeMarker(e.latlng.lat, e.latlng.lng);")
	drag := p422Window(t, code, "modalMarker.on('dragend'", "});")
	p422Need(t, "dragend handler", drag, "markFormTouched();", "updateCoordReadout(")
}
