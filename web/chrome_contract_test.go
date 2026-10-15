package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

const (
	chromeBarCall    = `{{template "disclaimer_bar.html.tmpl" .}}`
	chromeHeaderCall = `{{template "account_header.html.tmpl" .}}`
	chromeToggleCall = `{{template "theme_toggle.html.tmpl" .}}`
	chromeSheetLink  = `href="/static/css/chrome.css?v={{.AssetVersion}}"`
)

// chromeTemplateActionPattern matches one template action.
var chromeTemplateActionPattern = regexp.MustCompile(`\{\{[^}]*\}\}`)

// TestDisclaimerBarOnEveryFullPage derives the set of full pages from the
// embedded templates (R-08), so a future full page without the bar fails the
// build. Each page must open its body with the .top-chrome wrapper holding the
// bar partial call first, on pages with an account the account header call
// right after it, and link chrome.css once in the head. The partial itself
// reads only Chrome data (R-07).
func TestDisclaimerBarOnEveryFullPage(t *testing.T) {
	pages := findFullPageTemplates(t)

	bodyOpening := regexp.MustCompile(`(?s)<body[^>]*>\s*<div class="top-chrome">\s*` + regexp.QuoteMeta(chromeBarCall))

	for path, text := range pages {
		t.Run(path, func(t *testing.T) {
			if !bodyOpening.MatchString(text) {
				t.Errorf("%s: <body> must be followed by <div class=\"top-chrome\"> and then the bar partial call, with only whitespace between", path)
			}
			if got := strings.Count(text, chromeBarCall); got != 1 {
				t.Errorf("%s: bar partial call appears %d times, want exactly 1", path, got)
			}

			barAt := strings.Index(text, chromeBarCall)
			headerAt := strings.Index(text, chromeHeaderCall)
			toggleAt := strings.Index(text, chromeToggleCall)
			hasAccount := strings.HasSuffix(path, "index.html.tmpl") || strings.HasSuffix(path, "profile.html.tmpl") || strings.HasSuffix(path, "low.html.tmpl")

			if hasAccount {
				if headerAt < 0 {
					t.Fatalf("%s: expected the account header call", path)
				}
				between := text[barAt+len(chromeBarCall) : headerAt]
				if strings.TrimSpace(between) != "" {
					t.Errorf("%s: only whitespace may sit between the bar call and the account header call, found %q", path, between)
				}
				if toggleAt < headerAt {
					t.Errorf("%s: the theme toggle call must come after the account header call", path)
				}
			} else if headerAt >= 0 {
				t.Errorf("%s: this page has no account and must not call the account header", path)
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

	raw, err := fs.ReadFile(TemplatesFS, "templates/disclaimer_bar.html.tmpl")
	if err != nil {
		t.Fatalf("reading the bar partial: %v", err)
	}
	partial := string(raw)
	t.Run("partial", func(t *testing.T) {
		if strings.Contains(partial, "<html") {
			t.Error("the partial must never contain the html element opening text (it would be classed as a full page)")
		}
		if !strings.Contains(partial, `<aside class="disclaimer-bar" aria-label="Pinalert notice">`) {
			t.Error("the partial must open with the labelled aside")
		}
		if got := strings.Count(partial, "<p>"); got != 1 {
			t.Errorf("the partial has %d paragraphs, want exactly 1", got)
		}
		if got := strings.Count(partial, "<a "); got != 1 {
			t.Errorf("the partial has %d links, want exactly 1", got)
		}
		for _, action := range chromeTemplateActionPattern.FindAllString(partial, -1) {
			if action != "{{end}}" && !strings.Contains(action, ".Chrome.") {
				t.Errorf("template action %q reads something other than Chrome data (R-07)", action)
			}
		}
	})
}

// TestDisclaimerBarStylesheet checks the shared chrome rules with contains
// checks on declarations, so a later plan can add its own height and scroll
// padding rules without editing this test.
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

	want(".top-chrome", map[string]string{
		"position": "sticky",
		"top":      "0",
		"z-index":  "1050",
	})
	want(".disclaimer-bar", map[string]string{
		"display":       "flex",
		"align-items":   "center",
		"box-sizing":    "border-box",
		"background":    "var(--color-surface)",
		"border-bottom": "1px solid var(--color-border)",
		"color":         "var(--color-text)",
		"font-size":     "var(--font-size-label)",
		"line-height":   "var(--line-height-label)",
	})
	want(".disclaimer-bar p", map[string]string{"margin": "0"})
	want(".disclaimer-bar a", map[string]string{
		"font-weight":     "var(--font-weight-semibold)",
		"white-space":     "nowrap",
		"text-decoration": "underline",
	})

	forced := regexp.MustCompile(`(?s)@media \(forced-colors: active\)\s*\{.*`).FindString(css)
	if forced == "" {
		t.Fatal("no forced-colors block")
	}
	forcedRules := parseCSSRules(forced)
	// parseCSSRules keeps the at-rule preamble in the head of the first rule
	// inside it, so find the bar rule by suffix.
	var barRule cssRule
	ok := false
	for _, r := range forcedRules {
		if strings.HasSuffix(r.selectorHead, ".disclaimer-bar") {
			barRule, ok = r, true
		}
	}
	if !ok {
		t.Fatal("forced-colors block has no .disclaimer-bar rule")
	}
	barDecls := declsOf(barRule.declBody)
	if barDecls["border-bottom"] != "1px solid CanvasText" || barDecls["color"] != "CanvasText" {
		t.Errorf("forced-colors bar = %v, want a CanvasText border and text", barDecls)
	}
	linkFound := false
	for _, r := range forcedRules {
		if strings.HasSuffix(r.selectorHead, ".disclaimer-bar a") && declsOf(r.declBody)["color"] == "LinkText" {
			linkFound = true
		}
	}
	if !linkFound {
		t.Error("forced-colors block must give .disclaimer-bar a the LinkText colour")
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
