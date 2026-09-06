package web

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"
)

// tileLayerAnchor is the constructor call text every shipped tile layer
// construction in this app starts with. Its presence in a JS file signals
// "this file builds a Leaflet tile layer here"; its absence means the file
// constructs no tile layer at all (app.js, feed.js).
const tileLayerAnchor = "L.tileLayer("

// TestTileLayersRequestRetinaTiles guards the cosmetic UAT gap-closure fix:
// tile.openstreetmap.org serves only standard-resolution 1x, 256px raster
// tiles with no @2x/retina variant, so without Leaflet's own detectRetina
// option a Retina/HiDPI browser upscales those 1x tiles itself, which is
// what made street names and place labels look soft. detectRetina: true
// tells Leaflet to composite four higher-zoom tiles per tile slot instead,
// entirely internally — no change to the tile URL template, no {r}
// placeholder needed.
//
// This option is silently load-bearing on a second, unrelated-looking
// option: Leaflet 1.9.4's retina branch only engages when
// options.detectRetina && Browser.retina && options.maxZoom > 0. If a
// future edit removes or zeroes maxZoom on either tile layer, this fix
// becomes a silent no-op — no error, no test failure from this option's own
// presence check, just the blur quietly returning. That is why every
// content-gate in this plan also asserts maxZoom is still present.
//
// Honest limit of this test's claim: static inspection of the shipped
// JavaScript can prove the option is present and enabled on every tile
// layer this app constructs, but it cannot prove Leaflet actually fetched
// higher-density tiles or that anything looks sharper at runtime — that
// depends on devicePixelRatio in a real browser on a real display, which is
// exactly what this plan's human-check covers. The two are complementary,
// not redundant.
//
// The search window for each match is bounded to the tile layer's own
// construction — from the "L.tileLayer(" anchor up to the ".addTo(" call
// that always immediately follows it in this codebase — rather than
// running unbounded to end-of-file. An unbounded window would still "pass"
// if detectRetina or maxZoom appeared anywhere later in the file (e.g. in
// an unrelated L.divIcon({...}) call), which defeats the point of a
// regression gate: it must fail when the actual tile layer's options lose
// the setting, not merely when the string disappears from the whole file.
func TestTileLayersRequestRetinaTiles(t *testing.T) {
	const retinaOption = "detectRetina"
	const retinaValue = "true"
	const maxZoomOption = "maxZoom"
	const tileLayerEnd = ".addTo("

	seen := map[string]bool{}

	// readOptionValue returns the trimmed text of the value assigned to
	// key within body — the text after key's first following colon, up
	// to the next comma or closing brace, whichever comes first — and
	// whether key was found at all.
	readOptionValue := func(body, key string) (value string, found bool) {
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

		// Strip block comments only, reusing the package's existing
		// stripper (JS and CSS block comments share identical /* */
		// delimiters). Line comments are deliberately left alone: the
		// tile URL template's "https://" contains a double-slash, so a
		// naive line-comment stripper would truncate that string
		// mid-URL and destroy the very options object this test reads.
		text := stripCSSComments(string(raw))

		n := strings.Count(text, tileLayerAnchor)
		if n == 0 {
			return nil
		}
		if n > 1 {
			t.Fatalf(
				"%s: found %d tile layer constructions — this test assumes at most one per file "+
					"and must be updated alongside any new call",
				path, n,
			)
		}

		seen[path] = true

		anchorIdx := strings.Index(text, tileLayerAnchor)
		rest := text[anchorIdx:]

		endIdx := strings.Index(rest, tileLayerEnd)
		if endIdx == -1 {
			t.Errorf("%s: could not find %q after the tile layer construction — cannot safely "+
				"bound the search window to just this call's own options, so no further check "+
				"is run on it", path, tileLayerEnd)
			return nil
		}
		body := rest[:endIdx]

		if strings.Count(body, retinaOption) > 1 {
			t.Errorf("%s: expected %q exactly once in this tile layer's options, found more than one",
				path, retinaOption)
			return nil
		}

		value, found := readOptionValue(body, retinaOption)
		if !found {
			t.Errorf("%s: tile layer construction is missing the %q option entirely — "+
				"the tile host serves no @2x variant, so without this the browser upscales a 1x "+
				"raster and labels render blurry on a Retina display", path, retinaOption)
		} else if value != retinaValue {
			t.Errorf("%s: %q must be set to the boolean %q literal, found %q — a present-but-false "+
				"option is the same blurry map with a passing presence check",
				path, retinaOption, retinaValue, value)
		}

		// maxZoom is the second, unrelated-looking option Leaflet's retina
		// branch is conditional on (options.detectRetina && Browser.retina
		// && options.maxZoom > 0, per this test's doc comment above). A
		// future edit that removes or zeroes it would silently disable the
		// retina fix while detectRetina's own presence check above still
		// passes — this assertion is what makes that dependency durable
		// rather than only documented in prose.
		mzValue, mzFound := readOptionValue(body, maxZoomOption)
		if !mzFound {
			t.Errorf("%s: tile layer construction is missing %q entirely — Leaflet's retina "+
				"branch only engages when this option is greater than zero, so removing it "+
				"silently disables the retina fix even with %q still present and true",
				path, maxZoomOption, retinaOption)
		} else if n, convErr := strconv.Atoi(mzValue); convErr != nil || n <= 0 {
			t.Errorf("%s: %q must be a positive integer for Leaflet's retina branch to engage "+
				"(options.maxZoom > 0), found %q", path, maxZoomOption, mzValue)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/js: %v", err)
	}

	for _, module := range []string{"static/js/map.js", "static/js/modal.js"} {
		if !seen[module] {
			t.Fatalf("%s: expected a tile layer construction in this file but found none — "+
				"either the primary map or the report modal's own separate Leaflet instance is "+
				"missing its tile layer, which would stop this test passing vacuously if the call "+
				"were ever deleted or renamed", module)
		}
	}
}
