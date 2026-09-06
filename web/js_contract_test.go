package web

import (
	"io/fs"
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
func TestTileLayersRequestRetinaTiles(t *testing.T) {
	const retinaOption = "detectRetina"
	const retinaValue = "true"

	seen := map[string]bool{}

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
		body := text[anchorIdx:]

		optIdx := strings.Index(body, retinaOption)
		if optIdx == -1 {
			t.Errorf("%s: tile layer construction is missing the %q option entirely — "+
				"the tile host serves no @2x variant, so without this the browser upscales a 1x "+
				"raster and labels render blurry on a Retina display", path, retinaOption)
			return nil
		}
		if strings.Count(body, retinaOption) > 1 {
			t.Errorf("%s: expected %q exactly once in this tile layer's options, found more than one",
				path, retinaOption)
			return nil
		}

		afterOption := body[optIdx+len(retinaOption):]
		colonIdx := strings.Index(afterOption, ":")
		if colonIdx == -1 {
			t.Errorf("%s: found %q but no following colon — cannot read its value", path, retinaOption)
			return nil
		}
		afterColon := afterOption[colonIdx+1:]

		// The value runs up to the next comma or closing brace, whichever
		// comes first.
		end := len(afterColon)
		if i := strings.IndexAny(afterColon, ",}"); i != -1 {
			end = i
		}
		value := strings.TrimSpace(afterColon[:end])

		if value != retinaValue {
			t.Errorf("%s: %q must be set to the boolean %q literal, found %q — a present-but-false "+
				"option is the same blurry map with a passing presence check",
				path, retinaOption, retinaValue, value)
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
