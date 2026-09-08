package web

import (
	"strings"
	"testing"
)

// vendorScriptSources lists, in the exact order they must appear in
// index.html.tmpl, the three third-party <script> tags this template must
// load: Leaflet itself, the MapLibre GL vector-tile renderer, and the
// Leaflet<->MapLibre bridge plugin. The order is a hard correctness
// constraint, not a formatting preference: the bridge's UMD factory reads
// global.L and global.maplibregl at script-evaluation time
// (leaflet-maplibre-gl.js's factory call), so both other globals must
// already exist by the time the bridge's own top-level code runs. If any
// two of these three are swapped, the page ships with the bridge's entry
// point (L.maplibreGL) undefined at runtime, and the map dies on load —
// silently, with no build-time signal, unless this test exists.
//
// None of the three may carry a `defer` or `async` attribute either: the
// app's own four modules are already deferred, and a classic (non-deferred)
// script always finishes executing before any deferred script runs, which
// is what currently guarantees the vendor layer is ready before map.js
// runs. Deferring or async-loading any one of these three vendor tags would
// let the browser reorder it relative to the other two, breaking the same
// guarantee from a different angle.
var vendorScriptSources = []string{
	"https://unpkg.com/leaflet@1.9.4/dist/leaflet.js",
	"https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.js",
	"https://unpkg.com/@maplibre/maplibre-gl-leaflet@0.1.4/leaflet-maplibre-gl.js",
}

const maplibreStylesheetHref = "https://unpkg.com/maplibre-gl@5.24.0/dist/maplibre-gl.css"

// findTagWindow returns the substring of html spanning from the last
// tag-opening delimiter ("<") before the given source string up to the
// first tag-closing delimiter (">") after it — i.e. the bounds of the one
// tag that references src, and nothing else. This lets callers assert on
// that tag's own attributes (integrity, crossorigin, defer, async) without
// accidentally matching an attribute that belongs to a neighboring tag.
func findTagWindow(t *testing.T, html, src string) string {
	t.Helper()

	idx := strings.Index(html, src)
	if idx == -1 {
		t.Fatalf("could not locate %q in the template at all", src)
	}

	openIdx := strings.LastIndex(html[:idx], "<")
	if openIdx == -1 {
		t.Fatalf("could not find an opening '<' before %q", src)
	}

	closeOffset := strings.Index(html[idx:], ">")
	if closeOffset == -1 {
		t.Fatalf("could not find a closing '>' after %q", src)
	}
	closeIdx := idx + closeOffset + 1

	return html[openIdx:closeIdx]
}

// TestVendorMapScriptsLoadInDependencyOrder guards the vendor asset layer
// plan 01-12's MapLibre GL basemap depends on: it reads the shipped
// index.html.tmpl straight out of the embedded TemplatesFS (the same
// embedded-filesystem access pattern the existing CSS/JS contract tests use
// for StaticFS, so this test inspects what actually ships inside the
// compiled binary rather than a file that merely sits on disk) and asserts
// that the three vendor scripts are present exactly once each, in strict
// dependency order, each pinned with a sha256 integrity hash and
// crossorigin="anonymous", and none deferred or async-loaded — plus that
// every app module under /static/js/ loads after the bridge, and that the
// MapLibre stylesheet link is present and pinned.
//
// Honest limit of this test's claim: static inspection of the shipped
// template can prove the tags exist, are pinned, carry an integrity
// attribute, and sit in the right relative order. It CANNOT prove the
// browser actually fetched any of them, that the pinned hash matches the
// bytes the CDN serves today, or that the bridge's entry point
// (L.maplibreGL) ended up defined at runtime. The plan's verify-time
// recomputation gate covers the "hash matches served bytes" claim, and the
// plan's human-check covers the runtime claim (evaluating typeof
// L.maplibreGL in a real browser) — the three checks are complementary,
// each covering a gap the others cannot.
func TestVendorMapScriptsLoadInDependencyOrder(t *testing.T) {
	raw, err := TemplatesFS.ReadFile("templates/index.html.tmpl")
	if err != nil {
		t.Fatalf("failed to read templates/index.html.tmpl from the embedded TemplatesFS: %v", err)
	}
	html := string(raw)

	var positions []int
	for _, src := range vendorScriptSources {
		n := strings.Count(html, src)
		if n != 1 {
			t.Fatalf("expected exactly one occurrence of vendor script %q, found %d — a missing tag breaks the map, a duplicated one double-executes a megabyte of renderer code and re-registers the bridge", src, n)
		}

		pos := strings.Index(html, src)
		positions = append(positions, pos)

		window := findTagWindow(t, html, src)

		if !strings.Contains(window, `integrity="sha256-`) {
			t.Errorf("%s: tag is missing an integrity attribute beginning with sha256- — window: %q", src, window)
		}
		if !strings.Contains(window, `crossorigin="anonymous"`) {
			t.Errorf("%s: tag is missing crossorigin=\"anonymous\" — window: %q", src, window)
		}
		if strings.Contains(window, " defer") {
			t.Errorf("%s: tag carries a defer attribute — none of the three vendor tags may be deferred, because mixing deferred and non-deferred among them would break their relative execution order, which the bridge depends on at evaluation time — window: %q", src, window)
		}
		if strings.Contains(window, " async") {
			t.Errorf("%s: tag carries an async attribute — an async vendor tag can execute out of order relative to the other two, and the bridge reads both other globals at script-evaluation time — window: %q", src, window)
		}
	}

	for i := 1; i < len(positions); i++ {
		if positions[i-1] >= positions[i] {
			t.Fatalf(
				"vendor script order is wrong: %q must load strictly before %q, but found it at a later or equal position (%d vs %d) — the bridge reads both other globals at script-evaluation time, so a wrong order ships a page whose map dies on load with the bridge's entry point (L.maplibreGL) undefined, not merely a style nit",
				vendorScriptSources[i-1], vendorScriptSources[i], positions[i-1], positions[i],
			)
		}
	}

	bridgeSrc := vendorScriptSources[len(vendorScriptSources)-1]
	bridgePos := strings.Index(html, bridgeSrc)

	appModules := []string{
		"/static/js/app.js",
		"/static/js/map.js",
		"/static/js/modal.js",
		"/static/js/feed.js",
	}
	for _, mod := range appModules {
		pos := strings.Index(html, mod)
		if pos == -1 {
			t.Fatalf("expected app module %q to be present in the template, found none", mod)
		}
		if pos <= bridgePos {
			t.Fatalf(
				"app module %q loads before (or at the same position as) the bridge script %q — an app module hoisted above the vendor layer would run against undefined globals",
				mod, bridgeSrc,
			)
		}
	}

	if strings.Count(html, maplibreStylesheetHref) != 1 {
		t.Fatalf("expected exactly one MapLibre stylesheet link (%s) — it is required, not optional: it carries the positioning rules that keep the renderer's canvas correctly placed inside the layer container", maplibreStylesheetHref)
	}
	stylesheetWindow := findTagWindow(t, html, maplibreStylesheetHref)
	if !strings.Contains(stylesheetWindow, `integrity="sha256-`) {
		t.Errorf("MapLibre stylesheet link is missing an integrity attribute beginning with sha256- — window: %q", stylesheetWindow)
	}
	if !strings.Contains(stylesheetWindow, `crossorigin="anonymous"`) {
		t.Errorf("MapLibre stylesheet link is missing crossorigin=\"anonymous\" — window: %q", stylesheetWindow)
	}
}
