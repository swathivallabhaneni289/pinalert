package web

import (
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"testing"
	"unicode"
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
				if path != "static/css/main.css" {
					continue
				}
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

// extractCategoryList returns every single-quoted string found between the
// first "CATEGORIES = [" in js and the next "]" that follows it. Used by
// TestCategoryGlyphMaskRulesCoverEveryCategory to read app.js's declared
// category list without hand-maintaining a second copy of it in this test.
// Fails loudly (via t.Fatalf) if the array's own brackets cannot be located
// at all — a parse that silently finds nothing would make every assertion
// downstream of it vacuously true.
func extractCategoryList(t *testing.T, js string) []string {
	t.Helper()

	const anchor = "CATEGORIES = ["
	idx := strings.Index(js, anchor)
	if idx == -1 {
		t.Fatalf("could not find %q in app.js — cannot locate the category allowlist to check against", anchor)
	}
	rest := js[idx+len(anchor):]
	end := strings.Index(rest, "]")
	if end == -1 {
		t.Fatalf("found %q in app.js but no closing bracket for the CATEGORIES array", anchor)
	}
	body := rest[:end]

	var categories []string
	for {
		start := strings.Index(body, "'")
		if start == -1 {
			break
		}
		body = body[start+1:]
		closeIdx := strings.Index(body, "'")
		if closeIdx == -1 {
			break
		}
		categories = append(categories, body[:closeIdx])
		body = body[closeIdx+1:]
	}
	return categories
}

// takeIdentRun returns the leading run of CSS-identifier characters
// (letters, digits, underscore) in s. Used to read a category suffix off a
// selector immediately following the ".icon-glyph--" prefix — this
// project's categories (e.g. "storm_cyclone", "road_blocked") use
// underscores, so the identifier character set must include it.
func takeIdentRun(s string) string {
	for i, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return s[:i]
		}
	}
	return s
}

// extractMaskURLPath returns the path inside a CSS value of the form
// `url("/static/icons/flood.svg")` (quotes optional), or "" if the value is
// not a url(...) expression at all.
func extractMaskURLPath(value string) string {
	const open = "url("
	start := strings.Index(value, open)
	if start == -1 {
		return ""
	}
	rest := value[start+len(open):]
	end := strings.Index(rest, ")")
	if end == -1 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(rest[:end]), `"'`)
}

// glyphMaskRule records one parsed ".icon-glyph--{category}" rule: which
// stylesheet it came from, the category suffix on its selector, and the
// resolved path from its unprefixed mask-image declaration.
type glyphMaskRule struct {
	path       string
	category   string
	maskSource string
}

// collectGlyphMaskRules walks every shipped stylesheet and returns one
// glyphMaskRule per rule whose selector head contains the per-category
// glyph class prefix ".icon-glyph--". Reused by
// TestCategoryGlyphMaskRulesCoverEveryCategory; kept separate so the walk
// itself stays easy to read apart from the assertions built on top of it.
func collectGlyphMaskRules(t *testing.T) []glyphMaskRule {
	t.Helper()

	const glyphPrefix = ".icon-glyph--"
	var rules []glyphMaskRule

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
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := chunk[:lastOpen]
			declBody := chunk[lastOpen+1:]

			idx := strings.Index(selectorHead, glyphPrefix)
			if idx == -1 {
				continue
			}
			category := takeIdentRun(selectorHead[idx+len(glyphPrefix):])
			if category == "" {
				continue
			}

			var maskSource string
			for _, decl := range strings.Split(declBody, ";") {
				name, value, hasColon := strings.Cut(decl, ":")
				if !hasColon {
					continue
				}
				// Compare the trimmed property NAME for equality, not
				// substring containment: "mask-image" is a substring of
				// "-webkit-mask-image", so a containment check would
				// conflate the prefixed and unprefixed declarations.
				if strings.TrimSpace(name) == "mask-image" {
					maskSource = extractMaskURLPath(strings.TrimSpace(value))
				}
			}

			rules = append(rules, glyphMaskRule{path: path, category: category, maskSource: maskSource})
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	return rules
}

// assertBaseGlyphRule locates the exact ".icon-glyph" base rule (not a
// per-category ".icon-glyph--{category}" rule) in the shipped CSS and
// asserts it declares `background-color: currentColor` — the entire color
// mechanism the whole fix depends on — plus the mask-size, mask-repeat and
// mask-position longhands in BOTH -webkit- prefixed and unprefixed form.
// The cross-engine guarantee is the whole reason the "which engine
// implements the unprefixed spelling" question did not need resolving, so
// it must be enforced here, not assumed.
func assertBaseGlyphRule(t *testing.T) {
	t.Helper()

	var foundRule bool
	var hasCurrentColorBg bool
	var hasWebkitMaskSize, hasMaskSize bool
	var hasWebkitMaskRepeat, hasMaskRepeat bool
	var hasWebkitMaskPosition, hasMaskPosition bool

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
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := strings.TrimSpace(chunk[:lastOpen])
			if selectorHead != ".icon-glyph" {
				continue
			}
			foundRule = true
			declBody := chunk[lastOpen+1:]

			for _, decl := range strings.Split(declBody, ";") {
				name, value, hasColon := strings.Cut(decl, ":")
				if !hasColon {
					continue
				}
				name = strings.TrimSpace(name)
				value = strings.TrimSpace(value)
				switch name {
				case "background-color":
					if value == "currentColor" {
						hasCurrentColorBg = true
					}
				case "-webkit-mask-size":
					hasWebkitMaskSize = true
				case "mask-size":
					hasMaskSize = true
				case "-webkit-mask-repeat":
					hasWebkitMaskRepeat = true
				case "mask-repeat":
					hasMaskRepeat = true
				case "-webkit-mask-position":
					hasWebkitMaskPosition = true
				case "mask-position":
					hasMaskPosition = true
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css while looking for the base .icon-glyph rule: %v", err)
	}

	if !foundRule {
		t.Fatalf("no exact %q selector found in any shipped stylesheet — the base glyph rule may have been deleted or renamed", ".icon-glyph")
	}
	if !hasCurrentColorBg {
		t.Errorf(".icon-glyph base rule is missing `background-color: currentColor` — this is the entire color mechanism every one of the six theme/context combinations resolves from")
	}
	if !hasWebkitMaskSize || !hasMaskSize {
		t.Errorf(".icon-glyph base rule must declare mask-size in BOTH prefixed and unprefixed form (found -webkit-mask-size=%v, mask-size=%v)", hasWebkitMaskSize, hasMaskSize)
	}
	if !hasWebkitMaskRepeat || !hasMaskRepeat {
		t.Errorf(".icon-glyph base rule must declare mask-repeat in BOTH prefixed and unprefixed form (found -webkit-mask-repeat=%v, mask-repeat=%v) — omitting either lets that engine's mask tile, producing repeated ghost glyphs", hasWebkitMaskRepeat, hasMaskRepeat)
	}
	if !hasWebkitMaskPosition || !hasMaskPosition {
		t.Errorf(".icon-glyph base rule must declare mask-position in BOTH prefixed and unprefixed form (found -webkit-mask-position=%v, mask-position=%v)", hasWebkitMaskPosition, hasMaskPosition)
	}
}

// assertGlyphSizingAndNoOrphanedImageSelectors asserts two complementary
// things about each of the three shipped stylesheets that touch an icon:
// (a) each contains at least one rule whose selector head names the glyph
// class and whose body declares a width — Task 1's per-file sizing
// checklist, made mechanical; and (b) no rule anywhere in any shipped
// stylesheet still targets a REPLACED element (<img>) or a BARE VECTOR
// element (<svg>) nested inside the badge class or the category-tile
// class. (b) exists because an executor who appends the glyph selector to
// an existing image or vector-element selector, instead of replacing it,
// would still pass (a) while leaving dead CSS behind that documents the
// removed mechanism just as misleadingly as a stale comment would. The
// bare-vector-element half of this check (the trailing "svg" field) closes
// 01-REVIEW.md's WR-01: main.css's badge-child sizing rule shipped from
// 01-13 with a second, comma-separated ".icon-badge svg" selector that this
// function's earlier "img"-only check never caught, because every renderer
// appends a glyph <span> and no <svg> element ever actually lived inside
// the badge — the selector was dead from day one, and only a check that
// also looks for a trailing bare "svg" field can flag that shape.
func assertGlyphSizingAndNoOrphanedImageSelectors(t *testing.T) {
	t.Helper()

	sizingSeen := map[string]bool{
		"static/css/main.css":  false,
		"static/css/feed.css":  false,
		"static/css/modal.css": false,
	}

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
			lastOpen := strings.LastIndex(chunk, "{")
			if lastOpen == -1 {
				continue
			}
			selectorHead := chunk[:lastOpen]
			declBody := chunk[lastOpen+1:]

			if _, tracked := sizingSeen[path]; tracked {
				if strings.Contains(selectorHead, "icon-glyph") && strings.Contains(declBody, "width") {
					sizingSeen[path] = true
				}
			}

			// No rule may target an <img> (replaced-element) or a bare
			// <svg> (vector-element) selector nested inside .icon-badge or
			// .category-tile. Matched on whitespace-separated selector
			// fields — the last field is the element type, an earlier
			// field names the badge/tile class — rather than a raw
			// substring search, so an unrelated image or svg rule
			// elsewhere in the sheet (e.g. .view-toggle's own vector icon)
			// is not policed by this check.
			for _, sel := range strings.Split(selectorHead, ",") {
				fields := strings.Fields(sel)
				if len(fields) < 2 {
					continue
				}
				trailing := fields[len(fields)-1]
				if trailing != "img" && trailing != "svg" {
					continue
				}
				for _, field := range fields[:len(fields)-1] {
					if strings.Contains(field, ".icon-badge") || strings.Contains(field, ".category-tile") {
						t.Errorf(
							"%s: found an orphaned %s selector %q — Task 1 requires each of the "+
								"three icon-sizing selectors to REPLACE its image/vector-element half, "+
								"not keep it alongside the new glyph selector",
							path, trailing, strings.TrimSpace(sel),
						)
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/css: %v", err)
	}

	for path, ok := range sizingSeen {
		if !ok {
			t.Errorf("%s: expected at least one rule targeting the glyph class with a width declaration — this is Task 1's per-file sizing checklist", path)
		}
	}
}

// TestCategoryGlyphMaskRulesCoverEveryCategory guards the 01-13 fix's own
// core claim: every category in Pinalert.CATEGORIES has exactly one CSS
// mask rule pointing at a real, committed icon file. Nine hand-written
// mask-source paths mean a single typo produces exactly one silently
// invisible glyph on the report modal's 3x3 grid — a defect a human
// visually scanning that grid would plausibly miss, and a test that only
// asserted "a rule exists per category" would pass right through it. This
// test resolves every mask source against the embedded icon tree and
// asserts set equality in both directions at every step, so neither a
// missing rule, an extra rule for a deleted category, an orphaned icon
// file, nor a rule pointing at a mistyped filename can pass silently.
//
// Honest limit of this test's claim, in the same spirit as this package's
// other CSS contract tests: static inspection proves the rules and files
// line up, but cannot prove a real browser paints a legible glyph in any of
// the six context/theme combinations this fix's objective describes — that
// is exactly what this plan's human-check verify step covers, and the two
// are complementary, not redundant.
func TestCategoryGlyphMaskRulesCoverEveryCategory(t *testing.T) {
	appJS, err := fs.ReadFile(StaticFS, "static/js/app.js")
	if err != nil {
		t.Fatalf("failed to read embedded static/js/app.js: %v", err)
	}

	categories := extractCategoryList(t, string(appJS))
	if len(categories) == 0 {
		t.Fatalf("extracted zero categories from app.js's CATEGORIES array — a parse that finds " +
			"nothing would make every assertion below vacuously true, which is the classic way a " +
			"contract test rots into decoration")
	}

	rules := collectGlyphMaskRules(t)

	// Set equality between the category list and the rule suffixes, in
	// BOTH directions: every category has a rule, and every rule names a
	// real category. A one-directional check would pass with a rule for a
	// category that no longer exists.
	categorySet := map[string]bool{}
	for _, c := range categories {
		categorySet[c] = true
	}
	ruleCategorySet := map[string]bool{}
	for _, r := range rules {
		ruleCategorySet[r.category] = true
	}
	for c := range categorySet {
		if !ruleCategorySet[c] {
			t.Errorf("category %q is declared in app.js's CATEGORIES array but has no "+
				".icon-glyph--%s mask rule in any shipped stylesheet", c, c)
		}
	}
	for c := range ruleCategorySet {
		if !categorySet[c] {
			t.Errorf("found a .icon-glyph--%s mask rule for a category that does not exist in "+
				"app.js's CATEGORIES array — a rule for a deleted category would otherwise ship as "+
				"dead weight forever", c)
		}
	}

	// Resolve every rule's mask source against the embedded icon tree, in
	// BOTH directions: every rule resolves to a file, and every icon file
	// in the tree is claimed by some rule. This is the assertion that
	// earns the test its keep — nine hand-written file paths means a
	// single typo produces exactly one silently invisible glyph.
	claimedIcons := map[string]bool{}
	for _, r := range rules {
		if r.maskSource == "" {
			t.Errorf("%s: .icon-glyph--%s rule has no unprefixed mask-image declaration to resolve",
				r.path, r.category)
			continue
		}
		iconPath := strings.TrimPrefix(r.maskSource, "/")
		if _, statErr := fs.Stat(StaticFS, iconPath); statErr != nil {
			t.Errorf("%s: .icon-glyph--%s rule's mask source %q does not resolve to a real file in "+
				"the embedded static tree — this is exactly how a single mistyped path yields one "+
				"silently invisible glyph on the 3x3 grid, which a human doing the visual check would "+
				"plausibly miss", r.path, r.category, r.maskSource)
			continue
		}
		claimedIcons[iconPath] = true
	}

	err = fs.WalkDir(StaticFS, "static/icons", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !claimedIcons[path] {
			t.Errorf("%s: icon file exists in the embedded tree but is not claimed by any "+
				".icon-glyph--{category} mask rule", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/icons: %v", err)
	}

	assertBaseGlyphRule(t)
	assertGlyphSizingAndNoOrphanedImageSelectors(t)
}

// ---------------------------------------------------------------------------
// TestCategoryGlyphInkCoverageAcrossRenderContexts (01-14 Task 1) and its
// parsing helpers. See the test's own doc comment below for what it proves.
// ---------------------------------------------------------------------------

// inkFloorStrokeWidth and inkCeilingStrokeWidth are the hard bounds 01-14's
// plan derived, both measured (not guessed): the floor is set above the
// pre-fix artwork's stroke-width of 2, so this gate provably fails on the
// artwork shipped before this fix; the ceiling is set below the point at
// which a stroke centred on rescue_needed.svg's outer r=10 circle (or
// earthquake.svg's x=2..22 path span) would clip the 24-unit viewBox.
const (
	inkFloorStrokeWidth   = 2.5
	inkCeilingStrokeWidth = 3.5
	glyphViewBoxUnits     = 24.0
)

// extractAttr returns the value of an XML/SVG attribute of the form
// `name="value"` in raw, or ok=false if that exact attribute is not present.
func extractAttr(raw, attr string) (string, bool) {
	anchor := attr + `="`
	idx := strings.Index(raw, anchor)
	if idx == -1 {
		return "", false
	}
	rest := raw[idx+len(anchor):]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return "", false
	}
	return rest[:end], true
}

// findDeclValue reads the named embedded stylesheet and returns the trimmed
// value of declName from the rule whose selector head, once trimmed, equals
// exactSelector exactly. t.Fatalf on a missing file, a selector that cannot
// be found, or a selector found but never declaring declName — a silent ""
// would let every literal-sizing or token assertion built on this helper
// pass vacuously.
func findDeclValue(t *testing.T, path, exactSelector, declName string) string {
	t.Helper()

	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	text := stripCSSComments(string(raw))

	for chunk := range strings.SplitSeq(text, "}") {
		lastOpen := strings.LastIndex(chunk, "{")
		if lastOpen == -1 {
			continue
		}
		selectorHead := strings.TrimSpace(chunk[:lastOpen])
		if selectorHead != exactSelector {
			continue
		}
		declBody := chunk[lastOpen+1:]
		for _, decl := range strings.Split(declBody, ";") {
			name, value, hasColon := strings.Cut(decl, ":")
			if !hasColon {
				continue
			}
			if strings.TrimSpace(name) == declName {
				return strings.TrimSpace(value)
			}
		}
	}

	t.Fatalf("%s: no rule with selector head %q declaring %q found — a Task 1/Task 2 literal this "+
		"gate depends on may have been renamed or removed", path, exactSelector, declName)
	return ""
}

func parsePx(t *testing.T, label, value string) float64 {
	t.Helper()
	if !strings.HasSuffix(value, "px") {
		t.Fatalf("%s: value %q does not end in px", label, value)
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(value, "px"), 64)
	if err != nil {
		t.Fatalf("%s: value %q is not a valid px number: %v", label, value, err)
	}
	return n
}

func parsePercent(t *testing.T, label, value string) float64 {
	t.Helper()
	if !strings.HasSuffix(value, "%") {
		t.Fatalf("%s: value %q does not end in %%", label, value)
	}
	n, err := strconv.ParseFloat(strings.TrimSuffix(value, "%"), 64)
	if err != nil {
		t.Fatalf("%s: value %q is not a valid percentage: %v", label, value, err)
	}
	return n
}

// TestCategoryGlyphInkCoverageAcrossRenderContexts closes 01-14 Mechanism 1:
// the diagnosed stroke-only source artwork that capped a masked glyph's ink
// at a 1.47-2.02px hairline in all three render contexts regardless of
// color token (see .planning/debug/category-glyph-legibility.md). It has
// two factors and one free variable:
//
//  1. Artwork factor — every icon under static/icons must declare an
//     identical stroke-width, within [inkFloorStrokeWidth,
//     inkCeilingStrokeWidth], on the 24-unit viewBox the sizing arithmetic
//     below assumes.
//  2. Render-box factor — the feed row (55% of .icon-badge--sm's own 32px)
//     and the modal tile (a flat 24px) are read as LITERALS from one rule
//     each, not resolved through a cascade, and asserted to be at least
//     their current value — a monotonicity guard, so this gate can never be
//     satisfied by quietly shrinking a glyph.
//
// The map pin badge (55% of a >=44px badge = >=24.2px) is deliberately NOT
// resolved as a third asserted context: it is strictly larger than the feed
// row's 17.6px, so the feed-row assertion already dominates it by
// arithmetic. Chasing .icon-badge's width through var(--touch-target-min)
// into :root would require a miniature cascade resolver — exactly the kind
// of test that gets weakened the first time it breaks. Instead this test
// guards the domination argument itself with two cheap literal checks: that
// :root's --touch-target-min is still >=44px, and that .icon-badge's own
// width declaration still references it by name. If either check's shape
// has changed, this test fails loudly rather than silently keep assuming
// pin-badge dominance that may no longer hold.
//
// Honest limit of this test's claim, in the same spirit as this package's
// other CSS contract tests: this gate proves the diagnosed root cause
// (ink-starved geometry) was actually addressed and cannot silently
// regress, but it cannot prove a human can tell two glyphs apart at a
// glance — that is 01-14 Task 3's human-check, and the two are
// complementary, not redundant.
func TestCategoryGlyphInkCoverageAcrossRenderContexts(t *testing.T) {
	// --- Artwork factor ---
	strokeWidths := map[string]float64{}

	err := fs.WalkDir(StaticFS, "static/icons", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := fs.ReadFile(StaticFS, path)
		if err != nil {
			return err
		}
		text := string(raw)

		swStr, ok := extractAttr(text, "stroke-width")
		if !ok {
			t.Errorf("%s: no stroke-width attribute found", path)
			return nil
		}
		sw, parseErr := strconv.ParseFloat(swStr, 64)
		if parseErr != nil {
			t.Errorf("%s: stroke-width %q is not a valid number: %v", path, swStr, parseErr)
			return nil
		}
		strokeWidths[path] = sw

		vb, ok := extractAttr(text, "viewBox")
		if !ok {
			t.Errorf("%s: no viewBox attribute found", path)
			return nil
		}
		if strings.TrimSpace(vb) != "0 0 24 24" {
			t.Errorf("%s: viewBox is %q, expected the 24-unit box %q the sizing arithmetic assumes",
				path, vb, "0 0 24 24")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded static/icons: %v", err)
	}

	if len(strokeWidths) == 0 {
		t.Fatalf("found zero icon files under static/icons — a walk that finds nothing would make " +
			"every assertion below vacuously true")
	}

	var uniform float64
	first := true
	for path, sw := range strokeWidths {
		if first {
			uniform = sw
			first = false
			continue
		}
		if sw != uniform {
			t.Errorf("%s: stroke-width %v does not match the value %v declared by other icons — "+
				"Task 1 requires one identical value across all nine, applied identically", path, sw, uniform)
		}
	}
	if uniform < inkFloorStrokeWidth || uniform > inkCeilingStrokeWidth {
		t.Errorf("uniform stroke-width %v is outside the required bound [%v, %v]",
			uniform, inkFloorStrokeWidth, inkCeilingStrokeWidth)
	}

	// --- Render-box factor: literal values only ---
	feedPct := parsePercent(t, "static/css/feed.css .report-row .icon-badge .icon-glyph width",
		findDeclValue(t, "static/css/feed.css", ".report-row .icon-badge .icon-glyph", "width"))
	if feedPct < 55.0 {
		t.Errorf("feed row glyph width %v%% is below the current 55%% baseline — a shrink would "+
			"defeat this gate's monotonicity guarantee", feedPct)
	}

	smPx := parsePx(t, "static/css/main.css .icon-badge--sm width",
		findDeclValue(t, "static/css/main.css", ".icon-badge--sm", "width"))
	if smPx < 32.0 {
		t.Errorf(".icon-badge--sm width %vpx is below the current 32px baseline", smPx)
	}

	modalPx := parsePx(t, "static/css/modal.css #category-grid .category-tile .icon-glyph width",
		findDeclValue(t, "static/css/modal.css", "#category-grid .category-tile .icon-glyph", "width"))
	if modalPx < 24.0 {
		t.Errorf("modal tile glyph width %vpx is below the current 24px baseline", modalPx)
	}

	feedRenderBoxPx := feedPct / 100.0 * smPx
	modalRenderBoxPx := modalPx

	// Pin-badge domination guard — two literal checks, no cascade resolver.
	touchTargetPx := parsePx(t, ":root --touch-target-min",
		findDeclValue(t, "static/css/main.css", ":root", "--touch-target-min"))
	if touchTargetPx < 44.0 {
		t.Fatalf("--touch-target-min is %vpx, below 44px — the pin-badge domination argument "+
			"(55%% of a >=44px badge always exceeds 55%% of the 32px feed badge) no longer holds; "+
			"re-derive it", touchTargetPx)
	}
	iconBadgeWidthVal := findDeclValue(t, "static/css/main.css", ".icon-badge", "width")
	if !strings.Contains(iconBadgeWidthVal, "var(--touch-target-min)") {
		t.Fatalf(".icon-badge's width declaration is %q, no longer references var(--touch-target-min) "+
			"by name — re-derive the pin-badge domination argument", iconBadgeWidthVal)
	}
	pinRenderBoxPx := 0.55 * touchTargetPx // derived for the SUMMARY table only; not asserted below.

	// --- Compute and assert physical stroke width per icon per asserted context ---
	floorFeed := inkFloorStrokeWidth * feedRenderBoxPx / glyphViewBoxUnits
	floorModal := inkFloorStrokeWidth * modalRenderBoxPx / glyphViewBoxUnits

	for path, sw := range strokeWidths {
		feedPhysical := sw * feedRenderBoxPx / glyphViewBoxUnits
		modalPhysical := sw * modalRenderBoxPx / glyphViewBoxUnits
		pinPhysical := sw * pinRenderBoxPx / glyphViewBoxUnits

		if feedPhysical < floorFeed {
			t.Errorf("%s: feed-row physical stroke width %.3fpx is below the floor's implied %.3fpx",
				path, feedPhysical, floorFeed)
		}
		if modalPhysical < floorModal {
			t.Errorf("%s: modal-tile physical stroke width %.3fpx is below the floor's implied %.3fpx",
				path, modalPhysical, floorModal)
		}

		t.Logf("%s: stroke-width=%.2f -> pin(%.1fpx box)=%.3fpx tile(%.1fpx box)=%.3fpx feedRow(%.1fpx box)=%.3fpx",
			path, sw, pinRenderBoxPx, pinPhysical, modalRenderBoxPx, modalPhysical, feedRenderBoxPx, feedPhysical)
	}
}

// ---------------------------------------------------------------------------
// TestBadgeGlyphContrastAcrossAgeStagesAndThemes (01-14 Task 2) and its
// resolver helpers.
// ---------------------------------------------------------------------------

// bareVarName parses a CSS value of the exact shape `var(--name)` and
// returns "--name". Returns ok=false for any other shape, including a
// var() with a fallback (see parseColorVarShape for that two-argument
// shape, used only by the glyph-foreground parser).
func bareVarName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "var(") || !strings.HasSuffix(value, ")") {
		return "", false
	}
	inner := strings.TrimSpace(value[len("var(") : len(value)-1])
	if !strings.HasPrefix(inner, "--") {
		return "", false
	}
	return inner, true
}

// parseColorVarShape parses a `.icon-badge` `color` property value and
// accepts exactly the two shapes 01-14 Task 2 allows: a bare `var(--token)`
// (overrideName returned as ""), or `var(--override, var(--token))`. Any
// other shape returns ok=false. The override name is read out of the CSS
// here rather than hardcoded, so this test tracks a rename of the property
// instead of silently falling back to a stale name.
func parseColorVarShape(value string) (overrideName, baseTokenName string, ok bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "var(") || !strings.HasSuffix(value, ")") {
		return "", "", false
	}
	inner := value[len("var(") : len(value)-1]

	commaIdx := strings.Index(inner, ",")
	if commaIdx == -1 {
		name := strings.TrimSpace(inner)
		if !strings.HasPrefix(name, "--") {
			return "", "", false
		}
		return "", name, true
	}

	overrideName = strings.TrimSpace(inner[:commaIdx])
	if !strings.HasPrefix(overrideName, "--") {
		return "", "", false
	}
	fallback := strings.TrimSpace(inner[commaIdx+1:])
	if !strings.HasPrefix(fallback, "var(") || !strings.HasSuffix(fallback, ")") {
		return "", "", false
	}
	baseTokenName = strings.TrimSpace(fallback[len("var(") : len(fallback)-1])
	if !strings.HasPrefix(baseTokenName, "--") {
		return "", "", false
	}
	return overrideName, baseTokenName, true
}

// splitColorStop splits one `color-mix()` argument of the form
// " var(--x) 50%" into ("var(--x)", 50.0, true).
func splitColorStop(s string) (string, float64, bool) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return "", 0, false
	}
	if !strings.HasSuffix(fields[1], "%") {
		return "", 0, false
	}
	pct, err := strconv.ParseFloat(strings.TrimSuffix(fields[1], "%"), 64)
	if err != nil {
		return "", 0, false
	}
	return fields[0], pct, true
}

// parseColorMixStops parses a `color-mix(in srgb, A P%, B Q%)` value into
// its two (colorRef, percent) stops. Returns ok=false for any other shape —
// the caller must t.Fatalf per this test's "any fourth form must fail
// loudly" contract, rather than silently deleting the pairing.
func parseColorMixStops(value string) (ref1 string, pct1 float64, ref2 string, pct2 float64, ok bool) {
	value = strings.TrimSpace(value)
	const prefix = "color-mix(in srgb,"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ")") {
		return "", 0, "", 0, false
	}
	inner := value[len(prefix) : len(value)-1]
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return "", 0, "", 0, false
	}
	ref1, pct1, ok1 := splitColorStop(strings.TrimSpace(parts[0]))
	ref2, pct2, ok2 := splitColorStop(strings.TrimSpace(parts[1]))
	if !ok1 || !ok2 {
		return "", 0, "", 0, false
	}
	return ref1, pct1, ref2, pct2, true
}

func parseHexRGB(t *testing.T, hex, context string) (int, int, int) {
	t.Helper()
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		t.Fatalf("%s: color value %q is not a 6-digit hex color", context, hex)
	}
	r, err1 := strconv.ParseInt(hex[0:2], 16, 32)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 32)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 32)
	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("%s: color value %q could not be parsed as hex RGB", context, hex)
	}
	return int(r), int(g), int(b)
}

// mixHexSRGB implements `color-mix(in srgb, hex1 pct1%, hex2 pct2%)` as a
// weighted per-channel average of the gamma-encoded sRGB bytes — what
// `in srgb` interpolation means. It does NOT linearize first.
func mixHexSRGB(t *testing.T, hex1 string, pct1 float64, hex2 string, pct2 float64, context string) string {
	t.Helper()
	r1, g1, b1 := parseHexRGB(t, hex1, context)
	r2, g2, b2 := parseHexRGB(t, hex2, context)
	total := pct1 + pct2
	w1 := pct1 / total
	w2 := pct2 / total
	r := int(math.Round(float64(r1)*w1 + float64(r2)*w2))
	g := int(math.Round(float64(g1)*w1 + float64(g2)*w2))
	b := int(math.Round(float64(b1)*w1 + float64(b2)*w2))
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// themeTokens maps a custom-property name (e.g. "--color-bg") to its hex
// value string as declared for one resolved theme.
type themeTokens map[string]string

// resolveColorRefHex resolves a color-mix() stop reference (a bare
// var(--name)) against either the current severity's --severity-base hex or
// the theme's token table.
func resolveColorRefHex(t *testing.T, ref, severityBaseHex string, tokens themeTokens, context string) string {
	t.Helper()
	name, ok := bareVarName(ref)
	if !ok {
		t.Fatalf("%s: color-mix() stop %q is not a bare var() reference", context, ref)
	}
	if name == "--severity-base" {
		return severityBaseHex
	}
	if hex, found := tokens[name]; found {
		return hex
	}
	t.Fatalf("%s: color-mix() stop references unknown token %q", context, name)
	return ""
}

// resolveSeverityCurrentHex resolves one .age-{stage}'s `--severity-current`
// declaration value to a final hex, supporting exactly the three forms this
// codebase's .age-* rules use: a passthrough of --severity-base, a
// passthrough of a token (e.g. --color-age-stale), or a two-stop
// `color-mix(in srgb, A P%, B Q%)`. Any fourth form is a t.Fatalf naming the
// value, per this test's "extend the resolver, don't delete the pairing"
// contract.
func resolveSeverityCurrentHex(t *testing.T, ageDeclValue, severityBaseHex string, tokens themeTokens, context string) string {
	t.Helper()
	value := strings.TrimSpace(ageDeclValue)

	if name, ok := bareVarName(value); ok {
		if name == "--severity-base" {
			return severityBaseHex
		}
		if hex, found := tokens[name]; found {
			return hex
		}
		t.Fatalf("%s: --severity-current references unknown token %q", context, name)
	}

	if strings.HasPrefix(value, "color-mix(") {
		ref1, pct1, ref2, pct2, ok := parseColorMixStops(value)
		if !ok {
			t.Fatalf("%s: --severity-current is a color-mix() but not in the recognized "+
				"`color-mix(in srgb, A P%%, B Q%%)` shape: %q", context, value)
		}
		hex1 := resolveColorRefHex(t, ref1, severityBaseHex, tokens, context)
		hex2 := resolveColorRefHex(t, ref2, severityBaseHex, tokens, context)
		return mixHexSRGB(t, hex1, pct1, hex2, pct2, context)
	}

	t.Fatalf("%s: --severity-current value %q is not one of the three recognized forms "+
		"(passthrough of --severity-base, passthrough of a token, or a color-mix()) — extend the "+
		"resolver rather than deleting this pairing", context, value)
	return ""
}

// resolveForegroundValueHex resolves a `.age-{stage}` override declaration's
// value (or, when no override exists for that stage, the base token's own
// value) to a hex. Accepts a literal hex or a bare var() reference.
func resolveForegroundValueHex(t *testing.T, value string, tokens themeTokens, context string) string {
	t.Helper()
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return value
	}
	if name, ok := bareVarName(value); ok {
		if hex, found := tokens[name]; found {
			return hex
		}
		t.Fatalf("%s: foreground value references unknown token %q", context, name)
	}
	t.Fatalf("%s: foreground value %q is neither a hex literal nor a bare var() reference", context, value)
	return ""
}

func srgbChannelToLinear(c float64) float64 {
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func relativeLuminance(t *testing.T, hex, context string) float64 {
	t.Helper()
	r, g, b := parseHexRGB(t, hex, context)
	rl := srgbChannelToLinear(float64(r) / 255.0)
	gl := srgbChannelToLinear(float64(g) / 255.0)
	bl := srgbChannelToLinear(float64(b) / 255.0)
	return 0.2126*rl + 0.7152*gl + 0.0722*bl
}

// wcagContrastRatio computes the standard WCAG relative-luminance contrast
// ratio (lighter+0.05)/(darker+0.05) between two hex colors.
func wcagContrastRatio(t *testing.T, hexA, hexB, context string) float64 {
	t.Helper()
	la := relativeLuminance(t, hexA, context)
	lb := relativeLuminance(t, hexB, context)
	lighter, darker := la, lb
	if lb > la {
		lighter, darker = lb, la
	}
	return (lighter + 0.05) / (darker + 0.05)
}

// isDarkMediaRootHead reports whether a rule's trimmed selector head is the
// `@media (prefers-color-scheme: dark) { :root` compound produced when
// splitting CSS text on its LAST opening brace: the media block's own open
// brace and :root's open brace both precede the declaration body, so the
// head retains the full "@media ... {" preamble followed by ":root". This
// is identified by its @media PREFIX and its trailing ":root" FIELD, not by
// a substring search for the word "dark" — both `prefers-color-scheme: dark`
// and `[data-theme="dark"]` contain that substring, and so would a stripped
// comment mentioning it, so a naive Contains check would misidentify a
// rule.
func isDarkMediaRootHead(head string) bool {
	trimmed := strings.TrimSpace(head)
	if !strings.HasPrefix(trimmed, "@media") {
		return false
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return false
	}
	return fields[len(fields)-1] == ":root"
}

// extractThemeTokens walks every rule in mainCSSRules and buckets each
// needed custom property's value into one of the four theme SOURCES this
// codebase actually ships (not two): the base :root block and
// :root[data-theme="light"] both carry light values; the
// @media (prefers-color-scheme: dark) :root block and :root[data-theme="dark"]
// both carry dark values. Editing one member of a pair and not the other
// ships a fix that works for OS-preference users and not explicit-toggle
// users (or vice versa) — this is why both sources of each theme are kept
// separate here rather than merged, so the caller can assert pairwise
// equality.
func extractThemeTokens(rules []cssRule, needed []string) (light1, light2, dark1, dark2 themeTokens) {
	light1 = themeTokens{}
	light2 = themeTokens{}
	dark1 = themeTokens{}
	dark2 = themeTokens{}

	for _, r := range rules {
		var target *themeTokens
		switch {
		case r.selectorHead == ":root":
			target = &light1
		case r.selectorHead == `:root[data-theme="light"]`:
			target = &light2
		case r.selectorHead == `:root[data-theme="dark"]`:
			target = &dark2
		case isDarkMediaRootHead(r.selectorHead):
			target = &dark1
		default:
			continue
		}
		decls := declsOf(r.declBody)
		for _, name := range needed {
			if v, ok := decls[name]; ok {
				(*target)[name] = v
			}
		}
	}
	return
}

// cssRule is one parsed CSS rule: its trimmed selector head and its raw
// (untrimmed) declaration body.
type cssRule struct {
	selectorHead string
	declBody     string
}

// parseCSSRules splits raw CSS text (already comment-stripped) into one
// cssRule per rule, using this package's established split-on-last-brace
// idiom so a rule nested inside an at-rule (e.g. a media query) still
// parses with its at-rule preamble retained in the selector head rather
// than mistaken for part of the declaration body.
func parseCSSRules(raw string) []cssRule {
	text := stripCSSComments(raw)
	var rules []cssRule
	for chunk := range strings.SplitSeq(text, "}") {
		lastOpen := strings.LastIndex(chunk, "{")
		if lastOpen == -1 {
			continue
		}
		rules = append(rules, cssRule{
			selectorHead: strings.TrimSpace(chunk[:lastOpen]),
			declBody:     chunk[lastOpen+1:],
		})
	}
	return rules
}

// declsOf splits one rule's declaration body into a name->value map, keyed
// by the trimmed property name. Splitting on ";" is safe here because none
// of this package's parsed CSS values contain a literal semicolon.
func declsOf(declBody string) map[string]string {
	m := map[string]string{}
	for _, decl := range strings.Split(declBody, ";") {
		name, value, hasColon := strings.Cut(decl, ":")
		if !hasColon {
			continue
		}
		m[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return m
}

// ruleBySelector returns the first rule in rules whose trimmed selector
// head equals exactly want, or ok=false if none matches. Exact-head
// matching (never a substring search) is load-bearing for this test's
// glyph-foreground resolution: a wrongly-shaped fix that uses a compound
// selector (e.g. ".icon-badge.age-stale") or a descendant selector
// (e.g. ".report-row.age-stale .icon-badge") would satisfy a substring
// search for "age-stale" while covering only one of the two badge contexts
// (map.js:144 puts the age class on the badge itself; feed.js:169 puts it
// on the row ancestor) — with an exact-head match, those wrongly-shaped
// fixes leave the bare `.age-stale` rule undeclared, the resolver falls
// through to the base token, and the failing ratios this test exists to
// catch reappear as they should. A future reader must not "simplify" this
// into a strings.Contains call.
func ruleBySelector(rules []cssRule, want string) (cssRule, bool) {
	for _, r := range rules {
		if r.selectorHead == want {
			return r, true
		}
	}
	return cssRule{}, false
}

// TestBadgeGlyphContrastAcrossAgeStagesAndThemes closes 01-14 Mechanism 2:
// `.icon-badge` paints its glyph foreground against a `--severity-current`
// fill that the age ramp rewrites, without the foreground ever adapting —
// producing a genuine WCAG 1.4.11 point-contrast failure once a report
// desaturates (see .planning/debug/category-glyph-legibility.md). This test
// resolves both sides of every one of the 3 severities x 3 age stages x 2
// themes = 18 pairings statically from the shipped CSS and asserts each
// clears the 3.0:1 graphical-object floor.
//
// The resolution is built in five layers: (1) theme token tables, asserted
// pairwise-equal across each theme's two sources; (2) badge fill per
// severity/age stage, resolved from the shipped .sev-* and .age-* rules
// rather than hardcoding the ramp; (3) glyph foreground per age stage,
// parsed from .icon-badge's own `color` declaration and each .age-{stage}
// rule's EXACT selector head (see ruleBySelector's doc comment for why this
// must never become a substring search); (4) standard WCAG contrast;
// (5) the 18-pairing matrix itself.
//
// Honest limit of this test's claim, in the same spirit as this package's
// other CSS contract tests: point contrast is a NECESSARY condition for a
// readable glyph, not a SUFFICIENT one — it says nothing about whether the
// glyph's shape is identifiable, which is Mechanism 1's concern and is what
// TestCategoryGlyphInkCoverageAcrossRenderContexts and 01-14 Task 3's
// human-check cover instead.
func TestBadgeGlyphContrastAcrossAgeStagesAndThemes(t *testing.T) {
	mainRaw, err := fs.ReadFile(StaticFS, "static/css/main.css")
	if err != nil {
		t.Fatalf("failed to read embedded static/css/main.css: %v", err)
	}
	rules := parseCSSRules(string(mainRaw))

	// --- Layer 1: token tables ---
	neededTokens := []string{
		"--color-bg",
		"--color-text",
		"--color-severity-low",
		"--color-severity-medium",
		"--color-severity-critical",
		"--color-age-stale",
	}
	light1, light2, dark1, dark2 := extractThemeTokens(rules, neededTokens)

	for _, name := range neededTokens {
		v1, ok1 := light1[name]
		v2, ok2 := light2[name]
		if !ok1 || !ok2 {
			t.Fatalf("token %s not declared in both light sources (:root=%v present, "+
				":root[data-theme=light]=%v present)", name, ok1, ok2)
		}
		if v1 != v2 {
			t.Errorf("token %s mismatched between :root (%s) and :root[data-theme=\"light\"] (%s) — "+
				"a half-applied theme edit works for one light-mode entry path and not the other",
				name, v1, v2)
		}

		d1, dok1 := dark1[name]
		d2, dok2 := dark2[name]
		if !dok1 || !dok2 {
			t.Fatalf("token %s not declared in both dark sources (media block=%v present, "+
				":root[data-theme=dark]=%v present)", name, dok1, dok2)
		}
		if d1 != d2 {
			t.Errorf("token %s mismatched between the dark media block (%s) and "+
				":root[data-theme=\"dark\"] (%s) — a half-applied theme edit works for OS-preference "+
				"users and not explicit-toggle users, or vice versa", name, d1, d2)
		}
	}

	lightTokens := light1
	darkTokens := dark1

	// --- Layer 2 setup: --severity-base per severity ---
	severities := []string{"low", "medium", "critical"}
	severityBaseTokenName := map[string]string{}
	for _, sev := range severities {
		selector := ".sev-" + sev
		r, ok := ruleBySelector(rules, selector)
		if !ok {
			t.Fatalf("no exact %q rule found in static/css/main.css", selector)
		}
		decls := declsOf(r.declBody)
		val, ok := decls["--severity-base"]
		if !ok {
			t.Fatalf("%s: rule declares no --severity-base", selector)
		}
		name, ok := bareVarName(val)
		if !ok {
			t.Fatalf("%s: --severity-base value %q is not a bare var() reference", selector, val)
		}
		severityBaseTokenName[sev] = name
	}

	// --- Layer 3 setup: glyph foreground shape from .icon-badge ---
	badgeRule, ok := ruleBySelector(rules, ".icon-badge")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/main.css", ".icon-badge")
	}
	badgeDecls := declsOf(badgeRule.declBody)
	colorVal, ok := badgeDecls["color"]
	if !ok {
		t.Fatalf(".icon-badge declares no `color` property")
	}
	overrideName, baseTokenName, ok := parseColorVarShape(colorVal)
	if !ok {
		t.Fatalf(".icon-badge `color: %s` is not one of the two recognized shapes "+
			"(bare var(--token), or var(--override, var(--token)))", colorVal)
	}
	if overrideName == "" {
		t.Fatalf(".icon-badge `color: %s` is a bare var() with no override — Task 2 requires the "+
			"var(--override, var(--token)) shape so the age ramp can override the foreground per stage",
			colorVal)
	}

	ageStages := []string{"fresh", "aging", "stale"}
	ageSeverityCurrentValue := map[string]string{}
	ageOverrideValue := map[string]string{} // stage -> override decl value; absent if not declared
	for _, stage := range ageStages {
		selector := ".age-" + stage
		r, ok := ruleBySelector(rules, selector)
		if !ok {
			t.Fatalf("no exact %q rule found in static/css/main.css", selector)
		}
		decls := declsOf(r.declBody)
		sc, ok := decls["--severity-current"]
		if !ok {
			t.Fatalf("%s: rule declares no --severity-current", selector)
		}
		ageSeverityCurrentValue[stage] = sc

		if v, ok := decls[overrideName]; ok {
			ageOverrideValue[stage] = v
		}
		// Deliberately no else: a stage that declares no override
		// property is the intended fall-through-to-base-token case (this
		// is exactly .age-fresh's expected shape), not a parse failure.
	}

	// --- Layers 4-5: resolve and assert the 18-pairing matrix ---
	themes := []struct {
		name   string
		tokens themeTokens
	}{
		{"light", lightTokens},
		{"dark", darkTokens},
	}

	var ran int
	t.Logf("severity | stage | theme | fill | fg | ratio")
	for _, sev := range severities {
		baseTokenNameForSev := severityBaseTokenName[sev]
		for _, stage := range ageStages {
			for _, th := range themes {
				context := fmt.Sprintf("severity=%s stage=%s theme=%s", sev, stage, th.name)

				severityBaseHex, found := th.tokens[baseTokenNameForSev]
				if !found {
					t.Fatalf("%s: severity base token %q not found in %s token table",
						context, baseTokenNameForSev, th.name)
				}

				fillHex := resolveSeverityCurrentHex(t, ageSeverityCurrentValue[stage], severityBaseHex, th.tokens, context)

				var fgHex string
				if overrideVal, ok := ageOverrideValue[stage]; ok {
					fgHex = resolveForegroundValueHex(t, overrideVal, th.tokens, context)
				} else {
					baseHex, found := th.tokens[baseTokenName]
					if !found {
						t.Fatalf("%s: base foreground token %q not found in %s token table",
							context, baseTokenName, th.name)
					}
					fgHex = baseHex
				}

				ratio := wcagContrastRatio(t, fillHex, fgHex, context)
				ran++
				t.Logf("%-8s | %-6s | %-5s | %s | %s | %.2f:1", sev, stage, th.name, fillHex, fgHex, ratio)

				if ratio < 3.0 {
					t.Errorf("%s: badge glyph foreground %s against fill %s computes to %.2f:1, "+
						"below WCAG 1.4.11's 3.0:1 graphical-object floor", context, fgHex, fillHex, ratio)
				}
			}
		}
	}

	if ran != 18 {
		t.Fatalf("expected to evaluate exactly 18 severity x age-stage x theme pairings, evaluated %d — "+
			"the matrix loop's own bounds may have changed", ran)
	}
}
