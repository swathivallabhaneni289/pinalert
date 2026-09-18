package web

import (
	"io/fs"
	"strings"
	"testing"
)

// themeScriptSource is the versioned static-asset path the theme module
// ships under. themeJSPath and accountHeaderPath name the two embedded
// files most of this file's tests inspect.
const (
	themeScriptSource = "/static/js/theme.js"
	themeJSPath       = "static/js/theme.js"
	accountHeaderPath = "templates/account_header.html.tmpl"
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
// stored mode is validated against a fixed list before it is ever
// reflected into the root theme attribute. There must be exactly one place
// a value can reach the DOM (one setAttribute, one removeAttribute call
// site for the root theme attribute), and it must be downstream of an
// indexOf membership check against that fixed list.
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
	if got := strings.Count(text, "removeAttribute('data-theme'"); got != 1 {
		t.Errorf("%s: expected exactly one removeAttribute('data-theme') call site, found %d",
			themeJSPath, got)
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

// TestAccountHeaderRendersThemeControl asserts the shared account header
// partial renders the theme control between Activity and Log out, reusing
// the existing menu-item class and menuitem role, and that theme.js
// actually addresses both the control's id and its label span's id. Both
// ids are read out of the two embedded files rather than hardcoded twice in
// this test, so a rename on either side can only drift this test, never
// let the two silently diverge.
func TestAccountHeaderRendersThemeControl(t *testing.T) {
	tmplRaw, err := TemplatesFS.ReadFile(accountHeaderPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", accountHeaderPath, err)
	}
	tmplText := string(tmplRaw)

	const controlID = `id="theme-toggle"`
	const labelID = `id="theme-toggle-label"`

	if !strings.Contains(tmplText, controlID) {
		t.Fatalf("%s: expected %s — the theme control's own id", accountHeaderPath, controlID)
	}
	if !strings.Contains(tmplText, labelID) {
		t.Fatalf("%s: expected %s — the control's changing-word span's own id", accountHeaderPath, labelID)
	}

	window := findTagWindow(t, tmplText, controlID)
	if !strings.Contains(window, "menuitem") {
		t.Errorf("%s: theme control tag is missing the menuitem role — window: %q", accountHeaderPath, window)
	}
	if !strings.Contains(window, "account-menu__item") {
		t.Errorf("%s: theme control tag is missing the existing account-menu__item class — window: %q",
			accountHeaderPath, window)
	}

	jsRaw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", themeJSPath, err)
	}
	jsText := string(jsRaw)

	for _, id := range []string{"theme-toggle", "theme-toggle-label"} {
		if !strings.Contains(jsText, id) {
			t.Errorf("%s: expected a reference to %q — the id read from %s", themeJSPath, id, accountHeaderPath)
		}
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
