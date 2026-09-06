package web

import (
	"io/fs"
	"strings"
	"testing"
)

// stripCSSComments removes every /* ... */ block comment from a CSS source
// string. It works by repeatedly locating the next comment-open delimiter
// and the matching comment-close delimiter that follows it, then dropping
// that span. If a comment is left unterminated (no closing "*/"), the
// remainder of the string from the comment-open delimiter onward is
// dropped, since anything after an unterminated comment is unparseable
// anyway.
func stripCSSComments(css string) string {
	var b strings.Builder
	rest := css
	for {
		before, afterOpen, foundOpen := strings.Cut(rest, "/*")
		b.WriteString(before)
		if !foundOpen {
			break
		}
		_, afterClose, foundClose := strings.Cut(afterOpen, "*/")
		if !foundClose {
			// Unterminated comment — drop the remainder entirely.
			break
		}
		rest = afterClose
	}
	return b.String()
}

// TestModalBackdropHiddenGuard guards the UAT Test 1 blocker: an unguarded
// `display` declaration on the full-viewport `.modal-backdrop` scrim makes
// the entire primary map UI unreachable on load, because the browser's
// native `[hidden] { display: none }` rule is user-agent-origin and loses to
// an author-origin `display` declaration at equal specificity. This test
// fails the build if any shipped stylesheet ever sets `display` on the
// modal backdrop selector without a `:not([hidden])` guard.
func TestModalBackdropHiddenGuard(t *testing.T) {
	const backdropSelector = ".modal-backdrop"
	const guard = ":not([hidden])"

	var sawGuardedBackdropInMainCSS bool

	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}

		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}

		// Strip comments first so a rule's own explanatory comment can
		// mention the selector name without tripping the check below.
		text := stripCSSComments(string(raw))

		// Split on the closing-brace character to get one chunk per rule.
		for chunk := range strings.SplitSeq(text, "}") {
			// Take everything after the LAST opening-brace character as the
			// declaration body, and everything before it as the selector
			// head. Splitting on the last brace (not the first) keeps a
			// rule nested inside an at-rule (e.g. a media query) parsing
			// correctly instead of mistaking the at-rule preamble for the
			// selector.
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := chunk[:lastOpen]
			declBody := chunk[lastOpen+1:]

			if !strings.Contains(selectorHead, backdropSelector) {
				continue
			}
			if !strings.Contains(declBody, "display") {
				continue
			}

			if strings.Contains(selectorHead, guard) {
				if path == "static/css/main.css" {
					sawGuardedBackdropInMainCSS = true
				}
				continue
			}

			t.Errorf(
				"%s: found a rule setting `display` on the modal backdrop selector without a %q guard — selector head: %q",
				path, guard, strings.TrimSpace(selectorHead),
			)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	if !sawGuardedBackdropInMainCSS {
		t.Fatalf("expected static/css/main.css to contain at least one guarded %s%s selector — found none (the rule may have been deleted outright)", backdropSelector, guard)
	}
}

// TestPrimaryMapHasResolvedHeight guards the UAT Test 1 retest blocker: the
// primary Leaflet map container (the div index.html.tmpl gives id="map")
// had no height rule anywhere in the shipped CSS, so it collapsed to 0px and
// Leaflet painted no tiles at all. This test fails the build if the shipped
// CSS ever loses a height-establishing rule on that container.
//
// Honest limit of this test's claim: static inspection of the shipped CSS
// can prove a height-establishing rule exists on the right selector, but it
// cannot prove the resulting computed height is non-zero in a real
// browser's layout — a percentage-based ancestor chain, an unrelated
// overriding rule loaded later, or a browser bug could all still produce a
// zero computed height despite this test passing. That gap is exactly what
// this plan's human-check verify step covers; the two are complementary,
// not redundant. This test also intentionally does not universally forbid a
// parent-relative height: a percentage height is perfectly legitimate when
// every ancestor up to a definitely-sized box has a definite height. It
// encodes this fix's chosen strategy — a height the container resolves by
// itself, in a viewport unit — because that is the specific property this
// plan is locking in. A developer who deliberately builds a definite
// percentage chain instead should update this test alongside it, not fight
// it.
func TestPrimaryMapHasResolvedHeight(t *testing.T) {
	const primaryMapIDSelector = "#map"

	var matchingRulesInMainCSS int
	var selfResolvingValueFound bool

	err := fs.WalkDir(StaticFS, "static/css", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".css") {
			return nil
		}

		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}

		text := stripCSSComments(string(raw))

		for chunk := range strings.SplitSeq(text, "}") {
			// Split on the LAST opening brace (not the first) so a rule
			// nested inside a media query still parses correctly instead of
			// mistaking the at-rule preamble for the selector.
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := chunk[:lastOpen]
			declBody := chunk[lastOpen+1:]

			// A selector head can list multiple comma-separated selectors;
			// treat the rule as targeting the primary map container only if
			// one of those selectors' LAST whitespace-separated field
			// equals the primary map id exactly. Matching the last field
			// (rather than a plain substring search) lets a descendant-
			// qualified form like ".pane--map #map" still match, while
			// preventing a false positive on "#modal-map" — a different
			// element, inside the report modal, that this test must not
			// police.
			var targetsPrimaryMap bool
			for _, sel := range strings.Split(selectorHead, ",") {
				fields := strings.Fields(sel)
				if len(fields) == 0 {
					continue
				}
				if fields[len(fields)-1] == primaryMapIDSelector {
					targetsPrimaryMap = true
					break
				}
			}
			if !targetsPrimaryMap {
				continue
			}

			if path == "static/css/main.css" {
				matchingRulesInMainCSS++
			}

			for _, decl := range strings.Split(declBody, ";") {
				name, value, hasColon := strings.Cut(decl, ":")
				if !hasColon {
					continue
				}
				name = strings.TrimSpace(name)
				value = strings.TrimSpace(value)
				if name != "height" && name != "min-height" {
					continue
				}

				// A viewport-height unit suffix ("vh") also covers the
				// dynamic ("dvh"), small ("svh") and large ("lvh")
				// variants, since each of those strings contains "vh" as a
				// substring — no need to enumerate all four separately. An
				// absolute length (px, cm, mm, in, pt, pc, q) resolves
				// without depending on any ancestor either, so it counts as
				// self-resolving too. A bare percentage or a keyword like
				// "auto" does not.
				if strings.Contains(value, "vh") ||
					strings.HasSuffix(value, "px") ||
					strings.HasSuffix(value, "cm") ||
					strings.HasSuffix(value, "mm") ||
					strings.HasSuffix(value, "in") ||
					strings.HasSuffix(value, "pt") ||
					strings.HasSuffix(value, "pc") ||
					strings.HasSuffix(value, "q") {
					selfResolvingValueFound = true
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	if matchingRulesInMainCSS == 0 {
		t.Fatalf(
			"static/css/main.css has no rule targeting the primary map container (%s) — "+
				"this is exactly the UAT Test 1 blocker: Leaflet's L.map(container) in map.js's "+
				"init() measures a zero-height container and paints no tiles at all",
			primaryMapIDSelector,
		)
	}

	if !selfResolvingValueFound {
		t.Fatalf(
			"the primary map container (%s) has a height/min-height declaration, but none of "+
				"them carry a viewport-height or absolute-length unit — this test locks in this "+
				"fix's specific strategy (a height the container resolves by itself), not an "+
				"absolute rule of CSS: a parent-relative height is legitimate when every ancestor "+
				"up to a definitely-sized box has a definite height, which is not guaranteed here; "+
				"if you deliberately built that chain, update this test alongside it",
			primaryMapIDSelector,
		)
	}
}
