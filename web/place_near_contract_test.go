package web

// place_near_contract_test.go pins the page side of plan 04-32 (04-UAT test
// 11): the nothing near lines of both searches, the place mark on the Nearby
// map, and the report form's pick, which already moves the pin.

import (
	"os"
	"strings"
	"testing"
)

func TestNothingNearLines(t *testing.T) {
	tools := p421ReadStatic(t, "js/map-tools.js")
	form := p421ReadStatic(t, "js/report-form.js")

	p13MustContain(t, "map-tools.js", tools,
		"var MESSAGE_NO_MATCH_NEAR = 'No place by that name near you. Try another name.';")
	p13MustContain(t, "report-form.js", form,
		"var SEARCH_NO_MATCH_NEAR_MAP_MESSAGE = 'No place by that name near you. Try another name, or tap the map to put the pin there.';",
		"var SEARCH_NO_MATCH_NEAR_MESSAGE = 'No place by that name near you. Try another name.';")
	for label, src := range map[string]string{"map-tools.js": tools, "report-form.js": form} {
		for _, dash := range []string{"\u2014", "\u2013"} {
			if strings.Contains(src, "near you"+dash) || strings.Contains(src, dash+" Try another name") {
				t.Errorf("%s: a nothing near line holds a dash", label)
			}
		}
	}

	// The near line only when the search sent a point; the Low data form,
	// which has no map, never offers a tap on one.
	pick := jsFunctionBody(t, form, "noMatchMessage(near)")
	p13Ordered(t, pick, "noMatchMessage",
		"return SEARCH_NO_MATCH_MESSAGE;",
		"formVariant === 'low' ? SEARCH_NO_MATCH_NEAR_MESSAGE : SEARCH_NO_MATCH_NEAR_MAP_MESSAGE")

	// The Nearby search sends the map centre only when the map shows a local area.
	search := jsFunctionBody(t, tools, "search(query, typed)")
	p13MustContain(t, "search", search, "PinalertMap.getZoom()", "zoom >= NEAR_MIN_ZOOM", "if (near) {")

	// Both status lines stay polite live regions, so the sentence is read out.
	for _, tmpl := range []string{"templates/place_search.html.tmpl", "templates/index.html.tmpl", "templates/low.html.tmpl"} {
		raw, err := os.ReadFile(tmpl)
		if err != nil {
			t.Fatalf("reading %s: %v", tmpl, err)
		}
		src := string(raw)
		if !strings.Contains(src, `role="status" aria-live="polite"`) {
			t.Errorf("%s: the search status is no longer a polite live region", tmpl)
		}
	}
}

func TestPlaceMarkIsCalmAndTakesNoClick(t *testing.T) {
	mapJS := p421ReadStatic(t, "js/map.js")
	set := jsFunctionBody(t, mapJS, "setPlaceMark(lat, lon)")
	p13Ordered(t, set, "setPlaceMark", "clearPlaceMark();", "L.divIcon(", "L.marker(", ".addTo(map)")
	p13MustContain(t, "setPlaceMark", set,
		"interactive: false", "keyboard: false", "zIndexOffset: PLACE_MARK_Z_OFFSET",
		"'icon-glyph place-mark__glyph'", "className: 'place-mark'", "setAttribute('aria-hidden', 'true')")
	p13MustContain(t, "map.js", mapJS, "var PLACE_MARK_Z_OFFSET = -1000;",
		"setPlaceMark: setPlaceMark", "clearPlaceMark: clearPlaceMark", "getZoom: getZoom")
	// The mark goes when Back to my location lands, with the place.
	back := jsFunctionBody(t, mapJS, "recenterOnReader()")
	p13Ordered(t, back, "recenterOnReader", "Pinalert.clearPlace();", "clearPlaceMark();", "moveTo(lat, lon, zoom);")

	css := p421ReadStatic(t, "css/main.css")
	start := strings.Index(css, ".place-mark {")
	end := strings.Index(css, ".icon-glyph--flood {")
	if start < 0 || end < start {
		t.Fatal("main.css: the .place-mark rules are missing or out of place")
	}
	rules := css[start:end]
	p13MustContain(t, ".place-mark", rules, "pointer-events: none;", "color: var(--color-text);",
		`url("/static/icons/map-pin.svg")`)
	// No severity colour can be read into the mark.
	p13MustNotContain(t, ".place-mark", rules, "--color-sev", "--sev-", "#")
}

func TestReportFormPickMovesThePin(t *testing.T) {
	// Truth 5: a pick in Post a report already moves the form's pin to the
	// place, through the one pin placement path, and centres the map on it.
	modal := p421ReadStatic(t, "js/modal.js")
	choose := jsFunctionBody(t, modal, "chooseSearchResult(result)")
	p13Ordered(t, choose, "chooseSearchResult",
		"placeMarker(result.lat, result.lon, shortPlaceName(result));",
		"modalMap.setView([result.lat, result.lon], SEARCH_RESULT_ZOOM);")
	form := p421ReadStatic(t, "js/report-form.js")
	row := jsFunctionBody(t, form, "renderResultRow(result)")
	p13MustContain(t, "renderResultRow", row, "picker.choose({ lat: result.lat, lon: result.lon")
}
