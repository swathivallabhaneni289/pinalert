package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

const (
	chromeHeaderCall = `{{template "account_header.html.tmpl" .}}`
	chromeToggleCall = `{{template "theme_toggle.html.tmpl" .}}`
	// chromeTopbarCall is the shared top bar of the main places (Phase 3.4);
	// its block in app_bar.html.tmpl calls the account header.
	chromeTopbarCall = `{{template "app_topbar" .}}`
	// chromeStripCall is the laptop notice strip (Phase 3.4, D-29). Only the
	// Nearby page makes it.
	chromeStripCall = `{{template "notice_strip.html.tmpl" .}}`
	chromeSheetLink = `href="/static/css/chrome.css?v={{.AssetVersion}}"`
)

// chromeTemplateActionPattern matches one template action.
var chromeTemplateActionPattern = regexp.MustCompile(`\{\{[^}]*\}\}`)

// TestDisclaimerBarOnEveryFullPage derives the set of full pages from the
// embedded templates (R-08), so a future full page fails the build until it is
// given a place in the table below. Every page links chrome.css once in the
// head; the pages that carry a bar open their body in the order the table
// names; no page calls the Phase 4 bar partial any more.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
func TestDisclaimerBarOnEveryFullPage(t *testing.T) {
	pages := findFullPageTemplates(t)

	// What each page opens its body with, in order. "strip+top" is the notice
	// strip then the .top-chrome wrapper holding the top bar, "top" that
	// wrapper alone, "header" the wrapper holding only the account header,
	// "none" a body with no wrapper.
	opening := map[string]string{
		"templates/index.html.tmpl":          "strip+top",
		"templates/profile.html.tmpl":        "top",
		"templates/alerts.html.tmpl":         "top",
		"templates/settings.html.tmpl":       "top",
		"templates/low.html.tmpl":            "header",
		"templates/moderate.html.tmpl":       "header",
		"templates/login_gate.html.tmpl":     "none",
		"templates/verify_outcome.html.tmpl": "none",
		"templates/reader.html.tmpl":         "none",
		// Amended 2026-10-17 by Phase 3.4 plan 10 (D-27): the first-time flow is
		// a page of its own with no bar, no strip and no top wrapper.
		"templates/welcome.html.tmpl": "none",
	}

	wrapped := func(call string) *regexp.Regexp {
		return regexp.MustCompile(`(?s)<body[^>]*>\s*<div class="top-chrome">\s*` + regexp.QuoteMeta(call))
	}

	for path, text := range pages {
		t.Run(path, func(t *testing.T) {
			want, known := opening[path]
			if !known {
				t.Fatalf("%s: a full page with no entry in this table; add it with the bars it carries", path)
			}
			if strings.Contains(text, `{{template "disclaimer_bar.html.tmpl" .}}`) {
				t.Errorf("%s: the Phase 4 bar partial is gone and must not be called", path)
			}

			switch want {
			case "strip+top":
				stripFirst := regexp.MustCompile(`(?s)<body[^>]*>\s*` + regexp.QuoteMeta(chromeStripCall) + `\s*<div class="top-chrome">\s*` + regexp.QuoteMeta(chromeTopbarCall))
				if !stripFirst.MatchString(text) {
					t.Errorf("%s: <body> must open with the notice strip, then <div class=\"top-chrome\"> and the top bar call", path)
				}
			case "top":
				if !wrapped(chromeTopbarCall).MatchString(text) {
					t.Errorf("%s: <body> must be followed by <div class=\"top-chrome\"> and then the top bar call", path)
				}
			case "header":
				if !wrapped(chromeHeaderCall).MatchString(text) {
					t.Errorf("%s: <body> must be followed by <div class=\"top-chrome\"> and then the account header call", path)
				}
			case "none":
				if strings.Contains(text, `class="top-chrome"`) {
					t.Errorf("%s: this page carries no bar and no wrapper", path)
				}
				if strings.Contains(text, chromeHeaderCall) || strings.Contains(text, chromeTopbarCall) {
					t.Errorf("%s: this page has no account and must not call a bar", path)
				}
			}

			if want != "strip+top" && strings.Contains(text, chromeStripCall) {
				t.Errorf("%s: the notice strip is for the Nearby page only", path)
			}

			// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT
			// D-10): on the main places the account header is drawn by the
			// shared top bar, whose block in app_bar.html.tmpl calls it.
			if want == "strip+top" || want == "top" {
				bar, err := fs.ReadFile(TemplatesFS, "templates/app_bar.html.tmpl")
				if err != nil {
					t.Fatalf("reading the app bar partial: %v", err)
				}
				topbar, _, _ := strings.Cut(string(bar), `{{define "app_bottombar"}}`)
				if !strings.Contains(topbar, chromeHeaderCall) {
					t.Errorf("%s: the app_topbar block must call the account header", path)
				}
				if strings.Contains(text, chromeHeaderCall) {
					t.Errorf("%s: the account header is drawn by the top bar, not called again", path)
				}
			}

			// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-08):
			// the light and dark button is in the laptop top bar on the pages
			// that carry it (app_bar.html.tmpl includes it), so only the Low
			// data view and the moderator screen still call it themselves, and
			// there it still follows the account header. The Phase 3.1 and
			// Phase 4 hands-on tests are walked on this layout after Phase 3.4
			// is built.
			headerAt := strings.Index(text, chromeHeaderCall)
			if topAt := strings.Index(text, chromeTopbarCall); topAt >= 0 {
				headerAt = topAt
			}
			if toggleAt := strings.Index(text, chromeToggleCall); toggleAt >= 0 && toggleAt < headerAt {
				t.Errorf("%s: the theme toggle call must come after the account header call", path)
			}

			head, _, found := strings.Cut(text, "</head>")
			if !found {
				t.Fatalf("%s: no closing head tag", path)
			}
			if got := strings.Count(head, chromeSheetLink); got != 1 {
				t.Errorf("%s: chrome.css linked %d times in the head, want exactly 1", path, got)
			}
		})
	}

	raw, err := fs.ReadFile(TemplatesFS, "templates/notice_strip.html.tmpl")
	if err != nil {
		t.Fatalf("reading the strip partial: %v", err)
	}
	partial := string(raw)
	t.Run("partial", func(t *testing.T) {
		if strings.Contains(partial, "<html") {
			t.Error("the partial must never contain the html element opening text (it would be classed as a full page)")
		}
		if !strings.Contains(partial, `<aside class="notice-strip" role="note" aria-label="Pinalert notice">`) {
			t.Error("the partial must open with the labelled aside")
		}
		if got := strings.Count(partial, "<p "); got != 1 {
			t.Errorf("the partial has %d paragraphs, want exactly 1", got)
		}
		if got := strings.Count(partial, "<a "); got != 1 {
			t.Errorf("the partial has %d links, want exactly 1", got)
		}
		for _, action := range chromeTemplateActionPattern.FindAllString(partial, -1) {
			if !strings.Contains(action, ".Chrome.") {
				t.Errorf("template action %q reads something other than Chrome data (R-07)", action)
			}
		}
	})
}

// TestDisclaimerBarStylesheet checks the shared chrome rules with contains
// checks on declarations, so a later plan can add its own height and scroll
// padding rules without editing this test.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
// The .disclaimer-bar rules are gone; the same properties are pinned on
// .notice-strip.
func TestDisclaimerBarStylesheet(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, "static/css/chrome.css")
	if err != nil {
		t.Fatalf("reading chrome.css: %v", err)
	}
	css := stripCSSComments(string(raw))
	rules := parseCSSRules(css)

	want := func(selector string, decls map[string]string) {
		t.Helper()
		rule, ok := ruleBySelector(rules, selector)
		if !ok {
			t.Errorf("rule %q not found", selector)
			return
		}
		got := declsOf(rule.declBody)
		for name, value := range decls {
			if got[name] != value {
				t.Errorf("%s: %s = %q, want %q", selector, name, got[name], value)
			}
		}
	}

	if strings.Contains(css, "disclaimer-bar {") || strings.Contains(css, ".disclaimer-bar ") {
		t.Error("chrome.css still carries a .disclaimer-bar rule; the bar is gone")
	}

	want(".top-chrome", map[string]string{
		"position": "sticky",
		"top":      "0",
		"z-index":  "1050",
	})
	want(".notice-strip", map[string]string{
		"display":       "flex",
		"box-sizing":    "border-box",
		"min-height":    "var(--space-2xl)",
		"padding":       "var(--space-sm) var(--space-md)",
		"background":    "var(--color-notice-bg)",
		"border-bottom": "1px solid var(--color-notice-outline)",
		"color":         "var(--color-notice-text)",
		"font-size":     "var(--font-size-label)",
		"line-height":   "var(--line-height-label)",
	})
	want(".notice-strip__text", map[string]string{
		"margin": "0",
		"color":  "var(--color-notice-sub)",
	})
	want(".notice-strip__link", map[string]string{
		"font-weight":     "var(--font-weight-semibold)",
		"white-space":     "nowrap",
		"text-decoration": "underline",
		"color":           "var(--color-notice-text)",
	})
	want(".notice-strip__icon", map[string]string{
		"width":            "20px",
		"height":           "20px",
		"background-color": "var(--color-notice-icon)",
	})

	forced := regexp.MustCompile(`(?s)@media \(forced-colors: active\)\s*\{.*`).FindString(css)
	if forced == "" {
		t.Fatal("no forced-colors block")
	}
	forcedRules := parseCSSRules(forced)
	// parseCSSRules keeps the at-rule preamble in the head of the first rule
	// inside it, so find the strip rule by suffix.
	var barRule cssRule
	ok := false
	for _, r := range forcedRules {
		if strings.HasSuffix(r.selectorHead, ".notice-strip") {
			barRule, ok = r, true
		}
	}
	if !ok {
		t.Fatal("forced-colors block has no .notice-strip rule")
	}
	barDecls := declsOf(barRule.declBody)
	if barDecls["border-bottom"] != "1px solid CanvasText" || barDecls["color"] != "CanvasText" {
		t.Errorf("forced-colors strip = %v, want a CanvasText border and text", barDecls)
	}
	linkFound := false
	for _, r := range forcedRules {
		if strings.HasSuffix(r.selectorHead, ".notice-strip__link") && declsOf(r.declBody)["color"] == "LinkText" {
			linkFound = true
		}
	}
	if !linkFound {
		t.Error("forced-colors block must give .notice-strip__link the LinkText colour")
	}

	for _, banned := range []*regexp.Regexp{
		regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`),
		regexp.MustCompile(`\b(rgb|rgba|hsl|hsla)\(`),
		regexp.MustCompile(`!important`),
		regexp.MustCompile(`border-radius:\s*(9999|999|50%|[0-9]{3,}px)`),
	} {
		if banned.MatchString(css) {
			t.Errorf("chrome.css matches %q", banned)
		}
	}
}
