package web

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

// Anchors shared by all three tests in this file. vectorAnchor and
// rasterAnchor mark the start of each basemap branch's own construction
// call; constructionEnd is the ".addTo(" call that always immediately
// follows either one in this codebase, giving a bounded search window per
// construction rather than an unbounded run to end-of-file. leafletMapAnchor
// marks the Leaflet map constructor itself — its own open parenthesis is
// what keeps it from also matching the vector bridge constructor's longer
// name below. leafletMapConstructionEnd is the closing-paren-plus-semicolon
// that always immediately follows a map construction's options object in
// this codebase.
const (
	vectorAnchor              = "L.maplibreGL("
	rasterAnchor              = "L.tileLayer("
	constructionEnd           = ".addTo("
	leafletMapAnchor          = "L.map("
	leafletMapConstructionEnd = ");"
)

// The vector style URL and the raster tile URL template this app has
// reviewed and chosen. A silent re-point of either would otherwise start
// streaming visitor viewport coordinates — an approximation of where a
// person physically is during an emergency — to a host nobody evaluated.
const (
	vectorStyleURL        = "https://tiles.openfreemap.org/styles/liberty"
	rasterTileURLTemplate = "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
)

// readOptionValue returns the trimmed text of the value assigned to key
// within body — the text after key's first following colon, up to the next
// comma or closing brace, whichever comes first — and whether key was found
// at all. Tolerant of arbitrary whitespace around the key, colon and value.
// Shared by all three tests in this file so this parsing behaviour can only
// drift once, not three times.
func readOptionValue(body, key string) (value string, found bool) {
	optIdx := strings.Index(body, key)
	if optIdx == -1 {
		return "", false
	}
	afterKey := body[optIdx+len(key):]
	colonIdx := strings.Index(afterKey, ":")
	if colonIdx == -1 {
		return "", false
	}
	afterColon := afterKey[colonIdx+1:]
	end := len(afterColon)
	if i := strings.IndexAny(afterColon, ",}"); i != -1 {
		end = i
	}
	return strings.TrimSpace(afterColon[:end]), true
}

// windowAfter bounds a search window from the first occurrence of anchor in
// text to the first occurrence of terminator that follows it (exclusive of
// the terminator itself). It never searches unbounded to end-of-file, which
// would let an option matched anywhere later in an unrelated call "pass" a
// test that must be scoped to one specific construction. Returns the window
// body, whether the anchor was found, and whether the terminator was found
// once the anchor was located.
func windowAfter(text, anchor, terminator string) (body string, anchorFound, terminatorFound bool) {
	anchorIdx := strings.Index(text, anchor)
	if anchorIdx == -1 {
		return "", false, false
	}
	rest := text[anchorIdx:]
	endIdx := strings.Index(rest, terminator)
	if endIdx == -1 {
		return "", true, false
	}
	return rest[:endIdx], true, true
}

// TestBasemapBranchesPointAtCorrectHosts asserts that both basemap branches
// — the OpenFreeMap vector construction and the OpenStreetMap raster
// fallback — exist in each map module, that neither appears more than once,
// and that each points at the host this project actually reviewed and
// chose.
//
// Non-obvious dependency for a future editor: the raster window's
// terminator is the FIRST ".addTo(" call after the "L.tileLayer(" anchor,
// which is the fallback layer's own add-to-map call — so the line that
// re-derives the map's maximum zoom from that layer (map.setMaxZoom(...))
// sits OUTSIDE this window by construction, and the maximum-zoom value this
// test reads is unambiguously the layer's own. Anyone reordering the
// fallback's add-to-map call and its maxZoom re-derivation should know this
// window's bound depends on their order.
//
// Line comments are deliberately left alone by the stripper this test
// reuses (stripCSSComments strips block comments only): both the raster
// tile URL and the vector style URL contain a double slash, and a naive
// line-comment stripper would truncate either string mid-URL and destroy
// the very options object these tests read.
func TestBasemapBranchesPointAtCorrectHosts(t *testing.T) {
	seenVector := map[string]bool{}
	seenRaster := map[string]bool{}

	err := fs.WalkDir(StaticFS, "static/js", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".js") {
			return nil
		}

		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}

		text := stripCSSComments(string(raw))

		nVector := strings.Count(text, vectorAnchor)
		nRaster := strings.Count(text, rasterAnchor)
		if nVector == 0 && nRaster == 0 {
			return nil
		}
		if nVector > 1 {
			t.Fatalf("%s: found %d vector basemap constructions — this test assumes at most one per file",
				path, nVector)
		}
		if nRaster > 1 {
			t.Fatalf("%s: found %d raster fallback constructions — this test assumes at most one per file",
				path, nRaster)
		}

		if nVector == 1 {
			seenVector[path] = true
			body, _, termFound := windowAfter(text, vectorAnchor, constructionEnd)
			if !termFound {
				t.Errorf("%s: could not find %q after the vector basemap construction — cannot safely "+
					"bound the search window to just this call's own options", path, constructionEnd)
			} else {
				style, found := readOptionValue(body, "style")
				if !found {
					t.Errorf("%s: vector basemap construction is missing the style option entirely", path)
				} else if trimmed := strings.Trim(style, `'"`); trimmed != vectorStyleURL {
					t.Errorf("%s: vector basemap style must be exactly %q, found %q — a silently "+
						"re-pointed style host would stream visitor viewport coordinates to a "+
						"provider nobody evaluated", path, vectorStyleURL, trimmed)
				}

				attr, found := readOptionValue(body, "customAttribution")
				if !found || strings.TrimSpace(attr) == "" {
					t.Errorf("%s: vector basemap construction is missing a non-empty customAttribution "+
						"option", path)
				}
			}
		}

		if nRaster == 1 {
			seenRaster[path] = true
			body, _, termFound := windowAfter(text, rasterAnchor, constructionEnd)
			if !termFound {
				t.Errorf("%s: could not find %q after the raster fallback construction — cannot safely "+
					"bound the search window to just this call's own options", path, constructionEnd)
			} else {
				retina, found := readOptionValue(body, "detectRetina")
				if !found || retina != "true" {
					t.Errorf("%s: raster fallback must keep detectRetina set to the boolean true "+
						"literal, found %q (present=%v) — the fallback is meant to be exactly the "+
						"layer plan 01-10 shipped, not a stripped-down version of it", path, retina, found)
				}

				mz, found := readOptionValue(body, "maxZoom")
				if !found {
					t.Errorf("%s: raster fallback construction is missing maxZoom entirely", path)
				} else if n, convErr := strconv.Atoi(mz); convErr != nil || n <= 0 {
					t.Errorf("%s: raster fallback maxZoom must be a positive integer, found %q",
						path, mz)
				}

				if !strings.Contains(body, rasterTileURLTemplate) {
					t.Errorf("%s: raster fallback construction must use the byte-identical "+
						"OpenStreetMap tile URL template %q", path, rasterTileURLTemplate)
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/js: %v", err)
	}

	for _, module := range []string{"static/js/map.js", "static/js/modal.js"} {
		if !seenVector[module] {
			t.Fatalf("%s: expected a vector basemap construction in this file but found none — "+
				"either the primary map or the report modal's own separate Leaflet instance is "+
				"missing it, which would let this test pass vacuously if the call were ever "+
				"deleted or renamed", module)
		}
		if !seenRaster[module] {
			t.Fatalf("%s: expected a raster fallback construction in this file but found none — "+
				"same vacuity concern as the vector branch above", module)
		}
	}
}

// TestMapsDeclareOwnZoomBounds is the direct replacement for an assertion
// that used to live on the deleted raster tile layer, and it guards a real
// regression: Leaflet derives a map's maximum zoom from its zoom-bound
// layers, only grid-based layers register as one, and the vector bridge's
// layer is not one — so without an explicit maxZoom on the map's own
// options, both maps would silently get unbounded pinch-zoom.
//
// The minimum-zoom assertion guards a separate, unrelated-looking
// requirement of the bridge integration, not a stylistic preference: it
// runs the renderer one zoom level below Leaflet, so Leaflet zoom zero
// computes an out-of-range renderer zoom. A floor of at least 1 is a
// requirement of the integration, which is why a value of zero must fail
// here even though Leaflet itself would accept it.
//
// The anchor for the Leaflet map constructor includes its own open
// parenthesis, which is what keeps it from also matching the vector bridge
// constructor's longer name.
func TestMapsDeclareOwnZoomBounds(t *testing.T) {
	for _, module := range []string{"static/js/map.js", "static/js/modal.js"} {
		raw, err := fs.ReadFile(StaticFS, module)
		if err != nil {
			t.Fatalf("%s: could not read embedded file — %v", module, err)
		}
		text := stripCSSComments(string(raw))

		body, anchorFound, termFound := windowAfter(text, leafletMapAnchor, leafletMapConstructionEnd)
		if !anchorFound {
			t.Fatalf("%s: expected a %q Leaflet map construction but found none", module, leafletMapAnchor)
		}
		if !termFound {
			t.Fatalf("%s: could not find %q after the map construction — cannot safely bound the "+
				"search window to just this call's own options", module, leafletMapConstructionEnd)
		}

		mz, found := readOptionValue(body, "maxZoom")
		if !found {
			t.Errorf("%s: the map's own construction is missing maxZoom — the vector basemap layer "+
				"registers no zoom limit of its own, so without this option pinch-zoom becomes "+
				"unbounded", module)
		} else if n, convErr := strconv.Atoi(mz); convErr != nil || n <= 0 {
			t.Errorf("%s: the map's maxZoom must be a positive integer, found %q", module, mz)
		}

		mnz, found := readOptionValue(body, "minZoom")
		if !found {
			t.Errorf("%s: the map's own construction is missing minZoom — the bridge runs the "+
				"renderer one zoom level below Leaflet, so zoom zero computes an out-of-range "+
				"renderer zoom", module)
		} else if n, convErr := strconv.Atoi(mnz); convErr != nil || n <= 0 {
			t.Errorf("%s: the map's minZoom must be a positive integer (at least 1), found %q — "+
				"zero is accepted by Leaflet itself but is out of range for the bridge integration",
				module, mnz)
		}

		if !strings.Contains(body, "maxBounds") {
			t.Errorf("%s: the map's own construction is missing the maxBounds option — this is the "+
				"vector bridge maintainers' own boilerplate that prevents a documented renderer bug",
				module)
		}
	}
}

// TestCapabilityProbeGatesBasemapChoice checks that a capability probe is
// defined, that its own body actually contains the fail-safe checks it
// claims to (both vendor-global guards and the webgl2 context request —
// not just that those strings appear somewhere in the file), that the file
// requests a webgl2 rendering context somewhere, that the probe is invoked
// at least once after its own definition and before the vector
// construction, and that the vector construction appears before the raster
// fallback — the shape a true-branch-then-false-branch reads as. It also
// asserts the raster fallback's maxZoom ceiling correction
// (map.setMaxZoom(rasterLayer.options.maxZoom)) is present after the raster
// construction: Leaflet's retina branch can decrement a layer's own maxZoom
// on a high-density display, and without re-deriving the map's ceiling from
// the layer's post-construction value, a fallback visitor on such a display
// could reach a zoom at which the raster layer renders nothing.
//
// Honest limit of this test's claim, in the same spirit as this package's
// existing CSS tests' honest-limits paragraphs: static inspection can prove
// both constructions exist, that a capability probe is defined with the
// right checks in its own body, that it is invoked ahead of them, and that
// they appear in branch order — but it CANNOT prove the probe's return
// value actually selects between them at runtime, and it cannot execute
// WebGL detection at all. A file that called the probe and then
// unconditionally built both layers would pass this test. The runtime claim
// belongs to this plan's human-check; the two are complementary, not
// redundant. An `else`-token search between the two constructions was
// considered and rejected as brittle: line comments survive the stripper,
// so any prose containing that word between the branches would satisfy it
// vacuously while proving nothing.
func TestCapabilityProbeGatesBasemapChoice(t *testing.T) {
	const probeDefAnchor = "function hasVectorBasemap"
	const probeCallAnchor = "hasVectorBasemap("
	const webglContextCall = "getContext('webgl2')"
	const bridgeGlobalGuard = "typeof L.maplibreGL"
	const rendererGlobalGuard = "typeof maplibregl"
	const fallbackCeilingCorrection = "setMaxZoom(rasterLayer.options.maxZoom)"

	for _, module := range []string{"static/js/map.js", "static/js/modal.js"} {
		raw, err := fs.ReadFile(StaticFS, module)
		if err != nil {
			t.Fatalf("%s: could not read embedded file — %v", module, err)
		}
		text := stripCSSComments(string(raw))

		defIdx := strings.Index(text, probeDefAnchor)
		if defIdx == -1 {
			t.Fatalf("%s: expected a %q definition but found none", module, probeDefAnchor)
		}

		afterDef := text[defIdx+len(probeDefAnchor):]
		callIdx := strings.Index(afterDef, probeCallAnchor)
		if callIdx == -1 {
			t.Fatalf("%s: expected the capability probe to be invoked at least once after its own "+
				"definition", module)
		}
		absCallIdx := defIdx + len(probeDefAnchor) + callIdx

		// Bound the probe's own body to the span between its definition and
		// its first call site, rather than checking these guards appear
		// anywhere in the file — a fail-safe check present elsewhere but
		// absent from the probe itself would not actually gate anything.
		probeBody := afterDef[:callIdx]

		if !strings.Contains(probeBody, bridgeGlobalGuard) {
			t.Errorf("%s: the capability probe's own body must check %q — without this guard, a "+
				"browser where the bridge script failed to load (or was blocked by its integrity "+
				"check) would throw instead of falling back to the raster layer",
				module, bridgeGlobalGuard)
		}
		if !strings.Contains(probeBody, rendererGlobalGuard) {
			t.Errorf("%s: the capability probe's own body must check %q — without this guard, a "+
				"browser where the renderer script failed to load would throw instead of falling "+
				"back to the raster layer", module, rendererGlobalGuard)
		}
		if !strings.Contains(probeBody, webglContextCall) {
			t.Errorf("%s: the capability probe's own body must request a %q context — a "+
				"rendering-context request found only outside the probe would not actually gate "+
				"the basemap choice", module, webglContextCall)
		}

		vectorIdx := strings.Index(text, vectorAnchor)
		rasterIdx := strings.Index(text, rasterAnchor)
		if vectorIdx == -1 || rasterIdx == -1 {
			t.Fatalf("%s: expected both a vector and a raster basemap construction in this file",
				module)
		}

		if absCallIdx >= vectorIdx {
			t.Errorf("%s: the capability probe must be invoked before the vector basemap construction",
				module)
		}

		if vectorIdx >= rasterIdx {
			t.Errorf("%s: the vector basemap construction must appear before the raster fallback, "+
				"matching a true-branch-then-false-branch shape", module)
		}

		if !strings.Contains(text[rasterIdx:], fallbackCeilingCorrection) {
			t.Errorf("%s: expected %q after the raster fallback construction — Leaflet's retina "+
				"branch can decrement the layer's own maxZoom on a high-density display, and "+
				"without re-deriving the map's ceiling from the layer's post-construction value, a "+
				"fallback visitor on such a display could reach a zoom at which the map renders "+
				"nothing", module, fallbackCeilingCorrection)
		}
	}
}

// TestIconGlyphsAreClassDriven guards the 01-13 fix's chosen strategy: each
// of the three icon-rendering renderers resolves its category glyph
// through Pinalert.iconClass and builds a mask-styled element carrying that
// class pair, never a replaced-element <img> pointed directly at an icon
// file — the exact pattern that caused UAT Tests 2/6 (every glyph
// rendering solid black in dark mode, because an <img>'s loaded document is
// a sealed sub-document the page's CSS cannot reach into).
//
// This test encodes THIS fix's strategy, not a universal rule against ever
// creating an <img> element anywhere in the client. A contributor adding a
// genuinely unrelated image element to one of these three files later
// should update this test alongside it, not fight it.
func TestIconGlyphsAreClassDriven(t *testing.T) {
	const iconClassCall = "Pinalert.iconClass("
	const imageNodeCreation = "createElement('img')"

	for _, module := range []string{"static/js/map.js", "static/js/modal.js", "static/js/feed.js"} {
		raw, err := fs.ReadFile(StaticFS, module)
		if err != nil {
			t.Fatalf("%s: could not read embedded file — %v", module, err)
		}
		text := stripCSSComments(string(raw))

		if !strings.Contains(text, iconClassCall) {
			t.Errorf("%s: expected a %q call resolving the category glyph — this fix's whole point "+
				"is that glyph color comes from an ordinary CSS class on a real page element rather "+
				"than a path into an isolated image sub-document", module, iconClassCall)
		}
		if strings.Contains(text, imageNodeCreation) {
			t.Errorf("%s: found %q — this renderer has reverted to the sealed-sub-document icon "+
				"pattern that caused UAT Tests 2/6 (every category glyph rendering solid black in "+
				"dark mode, illegible on the report modal's category grid)", module, imageNodeCreation)
		}
	}

	appJS, err := fs.ReadFile(StaticFS, "static/js/app.js")
	if err != nil {
		t.Fatalf("static/js/app.js: could not read embedded file — %v", err)
	}
	appText := stripCSSComments(string(appJS))

	const defAnchor = "function iconClass"
	const exportAnchor = "iconClass: iconClass"
	const allowlistRef = "CATEGORIES.indexOf"

	defIdx := strings.Index(appText, defAnchor)
	if defIdx == -1 {
		t.Fatalf("static/js/app.js: expected a %q definition but found none", defAnchor)
	}
	if !strings.Contains(appText, exportAnchor) {
		t.Errorf("static/js/app.js: iconClass is defined but not exported on the returned Pinalert "+
			"API object (expected %q) — every renderer in map.js, modal.js and feed.js calls it as "+
			"Pinalert.iconClass", exportAnchor)
	}

	// Bound the allowlist-reference check to iconClass's own body — from
	// its definition to the next top-level function declaration in this
	// file, or end of file if it is the last one — rather than the whole
	// module. A CATEGORIES.indexOf reference found only elsewhere would not
	// actually guard this specific helper against T-01-13-01.
	afterDef := appText[defIdx+len(defAnchor):]
	fnBody := afterDef
	if nextFuncIdx := strings.Index(afterDef, "\n  function "); nextFuncIdx != -1 {
		fnBody = afterDef[:nextFuncIdx]
	}
	if !strings.Contains(fnBody, allowlistRef) {
		t.Errorf("static/js/app.js: iconClass's own body does not reference the CATEGORIES allowlist "+
			"(expected %q) — an unguarded rewrite of this helper would let a server-supplied category "+
			"value reach className unchecked (T-01-13-01, direct continuation of T-01-17)", allowlistRef)
	}
}

// TestMapPopupCarriesReportStateClasses closes plan 260923-mb0 Task 3: it
// proves map.js wires the same three validated state classes (severity,
// age, visibility) onto a marker's popup at the moment it is first bound,
// keeps them current on every reconcile pass afterward through the popup's
// own live container element, and never unbinds an already open popup to
// do it. render()'s own comment names dropping an open popup a reader is
// looking at as exactly the failure its reconcile-by-id loop exists to
// prevent.
//
// This file's own convention, matching TestIconGlyphsAreClassDriven above:
// prove the wiring exists in the shipped source text, not that it executes
// correctly. A real browser actually compositing a tinted popup is what the
// end-of-phase human check is for.
func TestMapPopupCarriesReportStateClasses(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/map.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/map.js: %v", err)
	}
	text := stripCSSComments(string(raw))

	// The content argument passed to bindPopup( itself contains a nested
	// call (buildPopupContent(report)), so a paren-based terminator would
	// read past the one call this window means to scope. marker.on( is the
	// statement that always immediately follows a new marker's bindPopup(
	// call in this file, so it is the deliberate terminator instead.
	const bindAnchor = "bindPopup("
	const bindTerminator = "marker.on("
	body, anchorFound, termFound := windowAfter(text, bindAnchor, bindTerminator)
	if !anchorFound {
		t.Fatalf("expected a %q call in static/js/map.js", bindAnchor)
	}
	if !termFound {
		t.Fatalf("expected a %q call following the %q call in static/js/map.js", bindTerminator, bindAnchor)
	}

	classNameValue, found := readOptionValue(body, "className")
	if !found {
		t.Fatalf("the bindPopup( call's options object does not declare a className option: %q", body)
	}
	if strings.HasPrefix(classNameValue, "'") || strings.HasPrefix(classNameValue, "\"") {
		t.Errorf("bindPopup('s className option %q is a literal string rather than a call to the "+
			"state class builder. A report's severity/age/visibility state can change on any 30 "+
			"second poll, so a literal string bound once at first open would go stale", classNameValue)
	}
	if classNameValue != "popupStateClasses(report)" {
		t.Errorf("bindPopup('s className option must call the state class builder as "+
			"popupStateClasses(report), found %q", classNameValue)
	}

	// getElement is Leaflet's own documented accessor and the only path
	// that reaches an already-opened popup's live container, which the
	// reconcile path needs in order to swap classes on an open popup in
	// place rather than rebinding it.
	const getElementAccessor = "getElement("
	if !strings.Contains(text, getElementAccessor) {
		t.Errorf("static/js/map.js does not reference Leaflet's %q accessor, the only documented "+
			"path that reaches an already-opened popup's live container", getElementAccessor)
	}

	const reconcileUpdate = "setPopupContent("
	if !strings.Contains(text, reconcileUpdate) {
		t.Errorf("static/js/map.js no longer calls %q on the reconcile path, the mechanism that "+
			"keeps an already-bound marker's popup content current on every 30 second poll",
			reconcileUpdate)
	}

	// The negative form of render()'s never-drop-an-open-popup constraint:
	// rebinding a popup to pass a fresh className would close a popup a
	// reader currently has open on the very next poll.
	const unbindMethod = "unbindPopup("
	if strings.Contains(text, unbindMethod) {
		t.Errorf("static/js/map.js calls %q. Rebinding a popup to pass a fresh className would "+
			"close a popup a reader currently has open on the next 30 second poll, which is exactly "+
			"what render()'s own reconcile-by-id loop exists to prevent", unbindMethod)
	}
}

// TestLocationSearchUsesTextSinksAndExistingPinPlacement (plan 07-04) proves
// three properties of the shipped static/js/modal.js bytes: every string
// the geocoding proxy returns reaches the DOM through a text sink and never
// a markup-parsing one (T-07-03), a suggestion tap reuses the file's single
// existing pin-placement path rather than adding a second one (D-03), and
// the input listener plus the teardown function carry the contract Task 1
// and Task 2 of this plan wrote them to. It proves properties of the
// shipped bytes; it cannot prove the dropdown looks or feels right in a
// real browser, which is the human check's job.
func TestLocationSearchUsesTextSinksAndExistingPinPlacement(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/js/modal.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/modal.js: %v", err)
	}
	text := stripCSSComments(string(raw))

	// 1. The script references all three element ids the template ships.
	// Quoted forms disambiguate 'location-search' from the two ids that
	// contain it as a prefix.
	for _, id := range []string{"'location-search'", "'location-search-results'", "'location-search-status'"} {
		if !strings.Contains(text, id) {
			t.Errorf("expected modal.js to reference the element id %s, found none", id)
		}
	}

	// 2. D-03's structural proof: there is one pin-placement code path in
	// the file and the search reuses it rather than adding a second.
	const markerConstruction = "L.marker("
	if n := strings.Count(text, markerConstruction); n != 1 {
		t.Errorf("expected exactly 1 occurrence of %q in modal.js (one pin-placement code path), found %d",
			markerConstruction, n)
	}

	// 3. Bound renderResultRow's own body and assert its sink discipline
	// and its calls into the existing pin-placement path.
	const rowBuilderAnchor = "function renderResultRow("
	const nextFunctionTerminator = "\n  function "
	rowBody, rowAnchorFound, _ := windowAfter(text, rowBuilderAnchor, nextFunctionTerminator)
	if !rowAnchorFound {
		t.Fatalf("expected a %q definition in modal.js", rowBuilderAnchor)
	}
	const setTextSink = "Pinalert.setText("
	if n := strings.Count(rowBody, setTextSink); n < 2 {
		t.Errorf("renderResultRow's own body must call %q at least twice (the primary line and the "+
			"secondary line), found %d", setTextSink, n)
	}
	if !strings.Contains(rowBody, "placeMarker(") {
		t.Errorf("renderResultRow's own body must call placeMarker(, the existing pin-placement "+
			"function, rather than constructing a marker itself")
	}
	if !strings.Contains(rowBody, "modalMap.setView(") {
		t.Errorf("renderResultRow's own body must call modalMap.setView(, matching the same pair "+
			"initLocation's GPS success callback calls (D-03)")
	}

	// 4. A whole-file zero count on every markup-parsing sink property.
	// Correct here, unlike a dash scan: this file's header states its
	// text-insertion rule by concept without naming any of these
	// properties, so there is no legitimate comment for the scan to trip
	// on.
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if n := strings.Count(text, sink); n != 0 {
			t.Errorf("found %d occurrence(s) of markup-parsing sink %q in modal.js (T-07-03): every "+
				"string the geocoding proxy returns must reach the DOM through Pinalert.setText, never "+
				"a sink that lets the browser parse it as markup", n, sink)
		}
	}

	// 5. The listener is registered in the top-level wiring block, not
	// inside initLocation, which runs on every openModal call.
	const inputListenerAnchor = "locationSearchInput.addEventListener('input'"
	const resetFormAnchor = "function resetForm"
	listenerIdx := strings.Index(text, inputListenerAnchor)
	resetFormIdx := strings.Index(text, resetFormAnchor)
	if listenerIdx == -1 {
		t.Fatalf("expected %q in modal.js", inputListenerAnchor)
	}
	if resetFormIdx == -1 {
		t.Fatalf("expected %q in modal.js", resetFormAnchor)
	}
	if listenerIdx <= resetFormIdx {
		t.Errorf("expected the byte index of %q (%d) to be greater than the byte index of %q (%d), "+
			"proving the listener is registered in the top-level wiring block rather than inside "+
			"initLocation", inputListenerAnchor, listenerIdx, resetFormAnchor, resetFormIdx)
	}

	// 6. resetSearch's own body carries the full teardown contract
	// (Pitfall 4): bound on the next top-level function, not the next
	// closing brace, so a nested block inside resetSearch cannot truncate
	// the region and let this assertion pass on a fragment.
	const resetSearchAnchor = "function resetSearch("
	resetSearchBody, resetSearchAnchorFound, _ := windowAfter(text, resetSearchAnchor, nextFunctionTerminator)
	if !resetSearchAnchorFound {
		t.Fatalf("expected a %q definition in modal.js", resetSearchAnchor)
	}
	for _, want := range []string{"clearTimeout", "searchSeq", "hideDropdown(", "clearSearchStatus(", "searchCache.clear("} {
		if !strings.Contains(resetSearchBody, want) {
			t.Errorf("resetSearch's own body is missing %q, part of the six-piece teardown contract "+
				"that keeps a reopened modal from showing the previous session's search state", want)
		}
	}

	// 7. The submit-time location message names all three ways to set a
	// location, and the stale two-way sentence is gone.
	const newLocationMessage = "Set a location by searching, dragging the pin, or allowing location access."
	const oldLocationMessage = "Set a location by dragging the pin or allowing location access."
	if n := strings.Count(text, newLocationMessage); n != 1 {
		t.Errorf("expected modal.js to contain %q exactly once, found %d", newLocationMessage, n)
	}
	if n := strings.Count(text, oldLocationMessage); n != 0 {
		t.Errorf("expected modal.js to contain the stale two-way location message %q zero times, found %d",
			oldLocationMessage, n)
	}
}
