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
// defined, that the file requests a webgl2 rendering context somewhere, that
// the probe is invoked at least once after its own definition and before
// the vector construction, and that the vector construction appears before
// the raster fallback — the shape a true-branch-then-false-branch reads as.
//
// Honest limit of this test's claim, in the same spirit as this package's
// existing CSS tests' honest-limits paragraphs: static inspection can prove
// both constructions exist, that a capability probe is defined and invoked
// ahead of them, and that they appear in branch order — but it CANNOT prove
// the probe's return value actually selects between them, and it cannot
// execute WebGL detection at all. A file that called the probe and then
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

		if !strings.Contains(text, webglContextCall) {
			t.Errorf("%s: expected a %q rendering-context request somewhere in this file",
				module, webglContextCall)
		}

		afterDef := text[defIdx+len(probeDefAnchor):]
		callIdx := strings.Index(afterDef, probeCallAnchor)
		if callIdx == -1 {
			t.Fatalf("%s: expected the capability probe to be invoked at least once after its own "+
				"definition", module)
		}
		absCallIdx := defIdx + len(probeDefAnchor) + callIdx

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
	}
}
