package web

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// themeScriptSource is the versioned static-asset path the theme module
// ships under. themeJSPath and accountHeaderPath name the two embedded
// files most of this file's tests inspect. themeTogglePath names the
// shared floating-control partial this plan extracts the control into.
const (
	themeScriptSource = "/static/js/theme.js"
	themeJSPath       = "static/js/theme.js"
	accountHeaderPath = "templates/account_header.html.tmpl"
	themeTogglePath   = "templates/theme_toggle.html.tmpl"
)

// findFullPageTemplates walks every file under templates/*.tmpl in
// TemplatesFS and treats a file as a full page if its text contains an
// "<html" element — a partial included by another template (e.g.
// check_inbox.html.tmpl, account_header.html.tmpl) never opens its own
// <html> element and is excluded by this check. Deriving the set from the
// embedded filesystem, rather than hardcoding four filenames, is what lets
// this test fail the build the day a future fifth full page ships without
// the theme script — the whole reason this helper exists.
func findFullPageTemplates(t *testing.T) map[string]string {
	t.Helper()

	pages := map[string]string{}
	err := fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		raw, readErr := fs.ReadFile(TemplatesFS, path)
		if readErr != nil {
			return readErr
		}
		text := string(raw)
		if strings.Contains(text, "<html") {
			pages[path] = text
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk embedded templates: %v", err)
	}

	if len(pages) < 4 {
		t.Fatalf("expected at least 4 full-page templates (files containing an <html element), "+
			"found %d — this test's premise (a fixed, non-trivial set of full pages exists to check) "+
			"may have silently evaporated", len(pages))
	}

	return pages
}

// TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage is this plan's central
// gate: every full page the app serves must load the theme module in its
// head, asset-versioned, with neither `defer` nor `async` — a deferred or
// async-loaded copy reintroduces the flash of the wrong palette this
// feature exists to remove. The full-page set is derived from the embedded
// filesystem (findFullPageTemplates) rather than hardcoded, so a future
// fifth full page added without the script tag fails this test rather than
// shipping silently.
//
// Honest limit of this test's claim: static inspection of the shipped
// templates proves the tag is present, unmodified by defer/async, and
// positioned before the head's closing tag. It cannot prove a real browser
// actually executes the script before paint, or that the resulting theme
// visibly applies with no flash — that is exactly this plan's human-check
// verify step's job (step 4), and the two are complementary, not redundant.
func TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage(t *testing.T) {
	pages := findFullPageTemplates(t)

	for path, text := range pages {
		if got := strings.Count(text, themeScriptSource); got != 1 {
			t.Errorf("%s: expected %q to appear exactly once, found %d", path, themeScriptSource, got)
			continue
		}

		window := findTagWindow(t, text, themeScriptSource)
		if strings.Contains(window, " defer") {
			t.Errorf("%s: theme script tag carries a defer attribute — this is exactly the change "+
				"that reintroduces the flash of the wrong palette on every load — window: %q", path, window)
		}
		if strings.Contains(window, " async") {
			t.Errorf("%s: theme script tag carries an async attribute — an async-loaded copy can "+
				"execute after first paint just as a deferred one can — window: %q", path, window)
		}
		if !strings.Contains(window, "?v={{.AssetVersion}}") {
			t.Errorf("%s: theme script tag is missing the ?v={{.AssetVersion}} cache-busting suffix "+
				"every other first-party local asset carries — window: %q", path, window)
		}

		tagIdx := strings.Index(text, themeScriptSource)
		headCloseIdx := strings.Index(text, "</head>")
		if headCloseIdx == -1 {
			t.Errorf("%s: could not locate a </head> closing tag", path)
			continue
		}
		if tagIdx > headCloseIdx {
			t.Errorf("%s: theme script tag (at %d) appears after </head> (at %d) — it must load in "+
				"the document head, while it is still parsing, not after", path, tagIdx, headCloseIdx)
		}
	}
}

// TestThemeModuleHasNoAppShellDependency mirrors
// TestAccountMenuJSHasNoAppShellDependency's own check: theme.js must never
// reference the app shell's global client store, because it loads on the
// login gate and verify-outcome pages, where that store does not exist.
func TestThemeModuleHasNoAppShellDependency(t *testing.T) {
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeJSPath, err)
	}
	text := string(raw)

	if strings.Contains(text, "Pinalert") {
		t.Errorf("%s: found a reference to %q — this script must run unchanged on the login gate "+
			"and verify-outcome pages, which load none of app.js/map.js/modal.js/feed.js",
			themeJSPath, "Pinalert")
	}
}

// TestThemeModuleUsesNoMarkupParsingSink guards T-01-03: theme.js only
// toggles one attribute and writes one text node, so it must never assemble
// a string into markup for the browser to parse.
func TestThemeModuleUsesNoMarkupParsingSink(t *testing.T) {
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeJSPath, err)
	}
	text := string(raw)

	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(text, sink) {
			t.Errorf("%s: found forbidden markup-parsing sink %q (T-01-03) — every string reaching "+
				"the DOM in this file must be inserted as text content or an attribute value, never "+
				"assembled into markup", themeJSPath, sink)
		}
	}
}

// TestThemeModuleValidatesStoredModeBeforeReflectingIt guards T-01-17: the
// stored mode is validated against a fixed two-element list (light, dark)
// before it is ever reflected into the root theme attribute. There must be
// exactly one place a value can reach the DOM, one setAttribute call site
// for the root theme attribute, and it must be downstream of an indexOf
// membership check against that fixed list. The single setAttribute call is
// the only write path. Since review finding WR-W01 (Phase 4) there is one
// removeAttribute call, inside applyMode, for the phone width, where the
// owner decided (2026-10-15) the page follows the device setting with no
// toggle; TestPhoneFollowsTheDeviceTheme pins it.
//
// This test also positively asserts theme.js references matchMedia at all,
// the one-time operating-system read that seeds a first-ever visit's
// initial mode (D-C). Presence only, deliberately never a count: the
// correct guarded form of this lookup (an existence check on the window
// property, then the call) legitimately names matchMedia twice, and the
// media-feature string legitimately appears again in the file's own header
// comment, so any exact count would force either a red build or a
// bound-reference hack that throws in a real browser. What this presence
// assertion does NOT prove: that the operating system is read exactly once
// ever. That property is structural, not greppable, because the
// initial-mode function runs exactly once during module evaluation and
// only reaches matchMedia when the storage read returned null (D-C) —
// carried by this task's done criteria and the human check, not by this
// test.
func TestThemeModuleValidatesStoredModeBeforeReflectingIt(t *testing.T) {
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeJSPath, err)
	}
	text := string(raw)

	if !strings.Contains(text, "MODES") {
		t.Errorf("%s: expected a declared fixed list of modes (found no reference to a MODES-shaped "+
			"identifier)", themeJSPath)
	}
	if !strings.Contains(text, ".indexOf(") {
		t.Errorf("%s: expected an indexOf membership check validating a stored value against the "+
			"fixed mode list before it is ever used", themeJSPath)
	}

	if got := strings.Count(text, "setAttribute('data-theme'"); got != 1 {
		t.Errorf("%s: expected exactly one setAttribute('data-theme', ...) call site, found %d — "+
			"more than one place a value can reach the DOM defeats the point of validating it in "+
			"exactly one place", themeJSPath, got)
	}

	if !strings.Contains(text, "matchMedia") {
		t.Errorf("%s: expected a reference to matchMedia — the one-time operating-system read that "+
			"seeds a first-ever visit's initial mode (D-C)", themeJSPath)
	}
}

// TestThemeModuleGuardsStorageAccess asserts theme.js references the
// storage API and guards it with at least two try blocks (the read and the
// write), citing votes.js's getVoterLocation precedent: a browser in a
// privacy mode can throw on storage access.
func TestThemeModuleGuardsStorageAccess(t *testing.T) {
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeJSPath, err)
	}
	text := string(raw)

	if !strings.Contains(text, "localStorage") {
		t.Errorf("%s: expected a reference to the localStorage API", themeJSPath)
	}
	if got := strings.Count(text, "try {"); got < 2 {
		t.Errorf("%s: found %d try blocks, want at least 2 — votes.js's getVoterLocation guards "+
			"both its storage read and its storage write in try/catch for the same privacy-mode "+
			"reason, and this module must guard both of its own storage accesses identically",
			themeJSPath, got)
	}
}

// TestThemeToggleRendersAsAFloatingControl replaces
// TestAccountHeaderRendersThemeControl now that the theme control has moved
// out of the account menu dropdown and into its own always-visible floating
// partial (quick task 260923-rra). It proves the markup lives in exactly one
// file, is included by exactly the two pages that should carry it, is
// entirely absent from the account header it used to live in and from the
// three headerless pages, and is still addressed by the same identifiers
// theme.js uses.
//
// Honest limit of this test's claim: static per-file inspection cannot prove
// a browser composes the templates into a single document with one unique
// id — html/template's ParseFS composition is not rendered here. The
// per-file exactly-once counts plus the critical negative assertion below
// stand in for that: if the control existed in two files at once, at least
// one of those counts would read two instead of one, or the negative
// assertion would fail.
func TestThemeToggleRendersAsAFloatingControl(t *testing.T) {
	const controlID = `id="theme-toggle"`
	const iconID = `id="theme-toggle-icon"`

	// a. The partial reads out of TemplatesFS without error.
	tmplRaw, err := TemplatesFS.ReadFile(themeTogglePath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeTogglePath, err)
	}
	tmplText := string(tmplRaw)

	// b. The control id and glyph span id each appear exactly once —
	// counts, not presence, so a duplicated block inside the partial itself
	// is caught.
	if got := strings.Count(tmplText, controlID); got != 1 {
		t.Fatalf("%s: expected %s to appear exactly once, found %d", themeTogglePath, controlID, got)
	}
	if got := strings.Count(tmplText, iconID); got != 1 {
		t.Fatalf("%s: expected %s to appear exactly once, found %d", themeTogglePath, iconID, got)
	}

	// c. The control's opening tag carries type=button, aria-label and
	// title — an icon-only control has no visible text, so those two
	// attributes carry the whole accessible name.
	window := findTagWindow(t, tmplText, controlID)
	if !strings.Contains(window, `type="button"`) {
		t.Errorf("%s: theme control tag is missing type=\"button\" — window: %q", themeTogglePath, window)
	}
	if !strings.Contains(window, "aria-label=") {
		t.Errorf("%s: theme control tag is missing an aria-label attribute — an icon-only control has "+
			"no visible text, so this attribute carries the whole accessible name — window: %q",
			themeTogglePath, window)
	}
	if !strings.Contains(window, "title=") {
		t.Errorf("%s: theme control tag is missing a title attribute — window: %q", themeTogglePath, window)
	}

	// d. The partial contains the base icon class and the moon modifier
	// class, the markup-side initial glyph theme.js overwrites on load.
	if !strings.Contains(tmplText, "auth-icon") {
		t.Errorf("%s: expected the base %q icon class", themeTogglePath, "auth-icon")
	}
	if !strings.Contains(tmplText, "auth-icon--moon") {
		t.Errorf("%s: expected the %q modifier class as the markup-side initial glyph", themeTogglePath, "auth-icon--moon")
	}

	// e. The partial does NOT contain the account menu item class. Outside
	// the menu that class styles nothing, and its presence would mean the
	// migration was only half done.
	if strings.Contains(tmplText, "account-menu__item") {
		t.Errorf("%s: found the account-menu__item class in the extracted partial — outside the menu "+
			"that class styles nothing, and its presence here means the migration into a standalone "+
			"floating control was only half done", themeTogglePath)
	}

	// f. THE CRITICAL NEGATIVE ASSERTION. Without this, a copy-paste that
	// adds the new partial while leaving the old button in place ships two
	// elements carrying the same control id on every gated page.
	// getElementById wires only the first, leaving a second, permanently
	// dead circle with a fully green build.
	headerRaw, err := TemplatesFS.ReadFile(accountHeaderPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountHeaderPath, err)
	}
	if strings.Contains(string(headerRaw), controlID) {
		t.Errorf("%s: still contains %s — the control must be a plain deletion from the account menu "+
			"now that it lives in %s; leaving both in place puts two elements with the same id on every "+
			"gated page, getElementById wires only the first, and one of the two circles becomes a "+
			"permanently dead button with a fully green build", accountHeaderPath, controlID, themeTogglePath)
	}

	// g. The include action for the new partial appears exactly once in
	// each of the two pages that should carry it. The expected string is
	// built from the path constant, not typed a second time, so a rename
	// can only fail the build and never let the two sides drift.
	includeFilename := strings.TrimPrefix(themeTogglePath, "templates/")
	includeAction := `{{template "` + includeFilename + `" .}}`

	//
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-08, UX-01
	// as amended): the light and dark button sits in the laptop top bar, so the
	// include lives once in app_bar.html.tmpl (every page that carries the bar:
	// Nearby, My activity, Official alerts, Settings) and the map and Activity
	// pages no longer include it themselves. The Low data view and the
	// moderator screen have no top bar and keep the floating include, once
	// each. The phone has none in either place and follows the device setting.
	// The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
	for page, want := range map[string]int{
		"templates/app_bar.html.tmpl":  1,
		"templates/low.html.tmpl":      1,
		"templates/moderate.html.tmpl": 1,
		"templates/index.html.tmpl":    0,
		"templates/profile.html.tmpl":  0,
		"templates/alerts.html.tmpl":   0,
		"templates/settings.html.tmpl": 0,
	} {
		raw, err := TemplatesFS.ReadFile(page)
		if err != nil {
			t.Fatalf("reading embedded %s: %v", page, err)
		}
		if got := strings.Count(string(raw), includeAction); got != want {
			t.Errorf("%s: expected the include action %q to appear %d times, found %d", page, includeAction, want, got)
		}
	}

	// The bar case: the button is an ordinary child of the laptop bar, and the
	// phone hides both arrangements.
	barCSS := stripCSSComments(p16ReadStatic(t, "static/css/app-bar.css"))
	barRule, ok := ruleBySelector(parseCSSRules(barCSS), ".app-topbar .theme-toggle")
	if !ok {
		t.Fatal("app-bar.css has no .app-topbar .theme-toggle rule")
	}
	barDecls := declsOf(barRule.declBody)
	for prop, want := range map[string]string{
		"position":      "static",
		"border":        "1px solid var(--color-border)",
		"border-radius": "8px",
		"background":    "var(--color-bg)",
		"box-shadow":    "none",
	} {
		if barDecls[prop] != want {
			t.Errorf(".app-topbar .theme-toggle %s = %q, want %q", prop, barDecls[prop], want)
		}
	}
	// The floating arrangement stays on the plain class for the two pages with
	// no bar (auth.css) and is hidden on a phone by chrome.css.
	floating, ok := ruleBySelector(parseCSSRules(stripCSSComments(p16ReadStatic(t, "static/css/auth.css"))), ".theme-toggle")
	if !ok || declsOf(floating.declBody)["position"] != "fixed" {
		t.Error("auth.css .theme-toggle must keep the floating position: fixed for the Low data view and the moderator screen")
	}
	hiddenOnPhone := false
	// The head is one of several in a shared hide list, so split the list.
	for _, r := range phoneRules(t) {
		for _, head := range p16HeadParts(r.Head) {
			if strings.TrimSpace(head) == ".app-topbar > .theme-toggle" && r.Media == phoneBlockMedia && r.Decls["display"] == "none" {
				hiddenOnPhone = true
			}
		}
	}
	if !hiddenOnPhone {
		t.Error("phone.css must hide .app-topbar > .theme-toggle inside its narrow block")
	}

	// h. That same include action appears zero times on the three
	// headerless pages. login_gate and verify_outcome are full pages;
	// check_inbox is a partial login_gate itself includes and reaches a
	// reader only through that page. All three render no header and no
	// theme control today, and this assertion is what keeps that true.
	for _, page := range []string{
		"templates/login_gate.html.tmpl",
		"templates/check_inbox.html.tmpl",
		"templates/verify_outcome.html.tmpl",
	} {
		raw, err := TemplatesFS.ReadFile(page)
		if err != nil {
			t.Fatalf("reading embedded %s: %v", page, err)
		}
		if got := strings.Count(string(raw), includeAction); got != 0 {
			t.Errorf("%s: expected the include action %q to appear zero times, found %d — this page "+
				"renders no header and must keep rendering no theme control", page, includeAction, got)
		}
	}

	// i. The existing cross-file drift loop, unchanged: theme.js must still
	// reference the control id, the glyph span id, and both icon modifier
	// class names. This is what catches a rename on one side shipping a
	// button that renders an empty circle.
	jsRaw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeJSPath, err)
	}
	jsText := string(jsRaw)

	for _, ref := range []string{"theme-toggle", "theme-toggle-icon", "auth-icon--sun", "auth-icon--moon"} {
		if !strings.Contains(jsText, ref) {
			t.Errorf("%s: expected a reference to %q — theme.js must address every id and class name "+
				"the template and CSS declare, or a rename on one side ships a button that renders an "+
				"empty circle", themeJSPath, ref)
		}
	}
}

// themeLengthTokenPrefix marks the design-token names
// resolveThemeGeometryLength is allowed to sum: every declared spacing
// token plus the shared touch-target token.
const themeLengthTokenPrefix = "--space"

// buildThemeGeometryTokenTable reads the exact :root rule out of main.css
// and returns a name-to-pixel-integer map covering every declared spacing
// token plus the shared touch-target token. It fails loudly if the
// touch-target token or the large space token cannot be resolved, because
// every later assertion in TestThemeToggleFloatsClearOfTheReportButton
// depends on them.
func buildThemeGeometryTokenTable(t *testing.T, mainCSSRules []cssRule) map[string]int {
	t.Helper()

	rootRule, ok := ruleBySelector(mainCSSRules, ":root")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/main.css", ":root")
	}
	decls := declsOf(rootRule.declBody)

	tokens := map[string]int{}
	for name, value := range decls {
		if !strings.HasPrefix(name, themeLengthTokenPrefix) && name != "--touch-target-min" {
			continue
		}
		trimmed := strings.TrimSuffix(strings.TrimSpace(value), "px")
		n, convErr := strconv.Atoi(trimmed)
		if convErr != nil {
			continue
		}
		tokens[name] = n
	}

	if _, ok := tokens["--touch-target-min"]; !ok {
		t.Fatalf(":root: could not resolve --touch-target-min to an integer pixel value")
	}
	if _, ok := tokens["--space-lg"]; !ok {
		t.Fatalf(":root: could not resolve --space-lg to an integer pixel value")
	}

	return tokens
}

// themeGeometryVarRefRE matches one var(--token-name) reference.
var themeGeometryVarRefRE = regexp.MustCompile(`var\(--[A-Za-z0-9-]+\)`)

// resolveThemeGeometryLength resolves a CSS length expression (a plain
// var() reference or a calc() summing several) to an integer number of
// pixels, by finding every var() reference, looking each name up in tokens,
// and summing them. It is the no-magic-number gate this test exists to
// enforce: it fails the test outright if the expression resolves to zero
// var() references, contains any px literal outside those references, or
// contains a minus, asterisk or slash — arithmetic other than addition is
// refused rather than silently mis-summed.
func resolveThemeGeometryLength(t *testing.T, tokens map[string]int, expr string) int {
	t.Helper()

	matches := themeGeometryVarRefRE.FindAllString(expr, -1)
	if len(matches) == 0 {
		t.Fatalf("length expression %q contains no var() references — a hardcoded pixel value has no "+
			"var references and must be rejected by this no-magic-number gate", expr)
	}

	stripped := expr
	sum := 0
	for _, m := range matches {
		name := strings.TrimSuffix(strings.TrimPrefix(m, "var("), ")")
		val, ok := tokens[name]
		if !ok {
			t.Fatalf("length expression %q references unknown token %q — not found on :root", expr, name)
		}
		sum += val
		stripped = strings.Replace(stripped, m, "", 1)
	}

	if strings.Contains(stripped, "px") {
		t.Fatalf("length expression %q contains a raw px literal outside its var() references", expr)
	}
	for _, op := range []string{"-", "*", "/"} {
		if strings.Contains(stripped, op) {
			t.Fatalf("length expression %q contains arithmetic operator %q other than addition — "+
				"refusing to resolve it rather than silently mis-summing", expr, op)
		}
	}

	return sum
}

// TestThemeToggleFloatsClearOfTheReportButton is RETIRED (replaced by TestFloatingReportButtonIsGone).
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the
// floating round report button (.fab) that this computed geometry lock kept
// clear of the theme toggle is gone. The Report item of the phone bottom bar
// and the Post a report link of the laptop top bar open the report window
// now, so there is no second control to stack against and no band to keep
// apart. The Phase 3.1 and Phase 4 hands-on tests are walked on this layout
// after Phase 3.4 is built.
//
// The one property of the old test that still holds on its own, the toggle's
// own layout declarations, lives on in TestThemeToggleKeepsItsOwnLayout below.
// TestFloatingReportButtonIsGone proves the report button is truly gone from the stylesheets so a
// stray rule cannot come back unnoticed.
func TestFloatingReportButtonIsGone(t *testing.T) {
	for _, path := range []string{"static/css/main.css", "static/css/phone.css", "static/css/auth.css"} {
		for _, r := range parseCSSRules(p16ReadStatic(t, path)) {
			if strings.Contains(r.selectorHead, ".fab") {
				t.Errorf("%s still has a rule for the retired floating report button: %q", path, r.selectorHead)
			}
		}
	}
}

// TestThemeToggleKeepsItsOwnLayout keeps the two properties of the retired
// stacking lock that do not depend on the report button (quick task
// 260923-rra): the toggle floats at a fixed position at the large space token
// from the right and the bottom, and, outside the account menu, it declares
// its own display, align-items, justify-content and cursor, because it
// inherits none of them from the menu item rule it used to free ride on.
// Without the first three the 24px glyph sits on the button's text baseline
// instead of centred in the 44px circle, a visible defect with a green build.
func TestThemeToggleKeepsItsOwnLayout(t *testing.T) {
	authRaw, err := StaticFS.ReadFile("static/css/auth.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/auth.css: %v", err)
	}
	toggleRule, ok := ruleBySelector(parseCSSRules(string(authRaw)), ".theme-toggle")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/auth.css", ".theme-toggle")
	}
	toggleDecls := declsOf(toggleRule.declBody)

	if toggleDecls["position"] != "fixed" {
		t.Errorf(".theme-toggle: expected position: fixed, got %q", toggleDecls["position"])
	}
	for prop, want := range map[string]string{"right": "var(--space-lg)", "bottom": "var(--space-lg)"} {
		if got := strings.TrimSpace(toggleDecls[prop]); got != want {
			t.Errorf(".theme-toggle %s is %q, want %q", prop, got, want)
		}
	}
	for _, prop := range []string{"display", "align-items", "justify-content", "cursor"} {
		if strings.TrimSpace(toggleDecls[prop]) == "" {
			t.Errorf(".theme-toggle: missing its own %q declaration, so the glyph would sit on the "+
				"button's text baseline instead of centred in the 44px circle", prop)
		}
	}
}

// TestThemeToggleIconAssetsExist locks the visual assets and geometry the
// two-state icon-only theme control depends on: both icon files exist and
// are embedded, both .auth-icon-- modifier rules resolve to those exact
// committed files, and the .theme-toggle control's own rule shares
// .account-trigger's height and declares a fully-rounded border-radius —
// what makes it a circle in the same visual family rather than a rounded
// rectangle (D-A).
//
// Honest limit of this test's claim, in the same spirit as this package's
// other CSS contract tests: static inspection proves the rules and files
// line up. It cannot prove the glyphs read as a sun and a moon at a glance
// — that is the plan's human-check verify step's job.
func TestThemeToggleIconAssetsExist(t *testing.T) {
	for _, path := range []string{"static/icons/sun.svg", "static/icons/moon.svg"} {
		if _, err := StaticFS.ReadFile(path); err != nil {
			t.Errorf("expected %s to exist in the embedded static filesystem: %v", path, err)
		}
	}

	authRaw, err := StaticFS.ReadFile("static/css/auth.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/auth.css: %v", err)
	}
	rules := parseCSSRules(string(authRaw))

	for selector, iconPath := range map[string]string{
		".auth-icon--sun":  "static/icons/sun.svg",
		".auth-icon--moon": "static/icons/moon.svg",
	} {
		rule, ok := ruleBySelector(rules, selector)
		if !ok {
			t.Fatalf("no exact %q rule found in static/css/auth.css", selector)
		}
		maskSrc := extractMaskURLPath(declsOf(rule.declBody)["mask-image"])
		if maskSrc == "" {
			t.Errorf("%s: no unprefixed mask-image url(...) declaration", selector)
			continue
		}
		resolved := strings.TrimPrefix(maskSrc, "/")
		if resolved != iconPath {
			t.Errorf("%s: mask-image resolves to %q, want %q", selector, resolved, iconPath)
		}
		if _, statErr := fs.Stat(StaticFS, resolved); statErr != nil {
			t.Errorf("%s: mask-image %q does not resolve to an embedded file: %v", selector, maskSrc, statErr)
		}
	}

	toggleRule, ok := ruleBySelector(rules, ".theme-toggle")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/auth.css", ".theme-toggle")
	}
	triggerRule, ok := ruleBySelector(rules, ".account-trigger")
	if !ok {
		t.Fatalf("no exact %q rule found in static/css/auth.css", ".account-trigger")
	}
	toggleDecls := declsOf(toggleRule.declBody)
	triggerDecls := declsOf(triggerRule.declBody)

	toggleHeight, hasToggleHeight := toggleDecls["height"]
	triggerHeight, hasTriggerHeight := triggerDecls["height"]
	if !hasToggleHeight || !hasTriggerHeight {
		t.Fatalf("need .theme-toggle height and .account-trigger height declared, got theme-toggle=%q account-trigger=%q",
			toggleHeight, triggerHeight)
	}
	if toggleHeight != triggerHeight {
		t.Errorf("geometry lock: .theme-toggle height (%q) must equal .account-trigger height (%q), "+
			"otherwise the two controls fall out of the same visual family (D-A)", toggleHeight, triggerHeight)
	}

	if got := toggleDecls["border-radius"]; got != "50%" {
		t.Errorf(".theme-toggle: expected border-radius: 50%%, got %q — this is what makes the control "+
			"a circle rather than a rounded rectangle", got)
	}
}

// TestThemeOverrideBlocksExistForBothModes asserts main.css still declares
// both data-theme override blocks and that all four theme sources declare
// color-scheme. Without the two override blocks, the toggle would set an
// attribute nothing reads — silently, with a green build and a control
// that appears to do nothing.
func TestThemeOverrideBlocksExistForBothModes(t *testing.T) {
	mainRaw, err := StaticFS.ReadFile("static/css/main.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/main.css: %v", err)
	}
	rules := parseCSSRules(string(mainRaw))

	for _, selector := range []string{`:root[data-theme="dark"]`, `:root[data-theme="light"]`} {
		if _, ok := ruleBySelector(rules, selector); !ok {
			t.Fatalf("no exact %q rule found in static/css/main.css", selector)
		}
	}

	sources := []struct {
		name     string
		selector string
		isDark   func(cssRule) bool
	}{
		{name: ":root (light)", selector: ":root"},
		{name: `:root[data-theme="light"]`, selector: `:root[data-theme="light"]`},
		{name: `:root[data-theme="dark"]`, selector: `:root[data-theme="dark"]`},
	}
	for _, src := range sources {
		rule, ok := ruleBySelector(rules, src.selector)
		if !ok {
			t.Fatalf("no exact %q rule found in static/css/main.css", src.selector)
		}
		decls := declsOf(rule.declBody)
		if _, ok := decls["color-scheme"]; !ok {
			t.Errorf("%s: expected a color-scheme declaration", src.name)
		}
	}

	var sawDarkMediaColorScheme bool
	for _, r := range rules {
		if isDarkMediaRootHead(r.selectorHead) {
			decls := declsOf(r.declBody)
			if _, ok := decls["color-scheme"]; ok {
				sawDarkMediaColorScheme = true
			}
		}
	}
	if !sawDarkMediaColorScheme {
		t.Errorf("the @media (prefers-color-scheme: dark) :root block is missing a color-scheme declaration")
	}
}
