package web

import (
	"io/fs"
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
// stylesheet still targets an <img> element nested inside the badge class
// or the category-tile class. (b) exists because an executor who appends
// the glyph selector to the existing image selector, instead of replacing
// it, would still pass (a) while leaving dead CSS behind that documents the
// removed mechanism just as misleadingly as a stale comment would.
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

			// No rule may target an <img> nested inside .icon-badge or
			// .category-tile. Matched on whitespace-separated selector
			// fields — the last field is the element type, an earlier
			// field names the badge/tile class — rather than a raw
			// substring search, so an unrelated image rule elsewhere in
			// the sheet is not policed by this check.
			for _, sel := range strings.Split(selectorHead, ",") {
				fields := strings.Fields(sel)
				if len(fields) < 2 || fields[len(fields)-1] != "img" {
					continue
				}
				for _, field := range fields[:len(fields)-1] {
					if strings.Contains(field, ".icon-badge") || strings.Contains(field, ".category-tile") {
						t.Errorf(
							"%s: found an orphaned image selector %q — Task 1 requires each of the "+
								"three icon-sizing selectors to REPLACE its image half, not keep it "+
								"alongside the new glyph selector",
							path, strings.TrimSpace(sel),
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
