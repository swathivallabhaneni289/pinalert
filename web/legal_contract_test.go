package web

import (
	"regexp"
	"strings"
	"testing"
)

var (
	legalActionPattern = regexp.MustCompile(`(?s)\{\{.*?\}\}`)
	legalNumberPattern = regexp.MustCompile(`\b(112|100|101|108|1070|1077)\b`)
	legalWordSplitter  = regexp.MustCompile(`[^A-Za-z]+`)
	legalNamePattern   = regexp.MustCompile(`(?s)class="[^"]*"|\{\{/\*.*?\*/\}\}|<!--.*?-->`)
)

var legalTemplateNames = []string{"templates/disclaimer.html.tmpl", "templates/policy.html.tmpl"}

// legalBannedWord reports a banned word in text, splitting identifiers on hyphens,
// underscores and every other non letter.
func legalBannedWord(text string) string {
	for _, w := range legalWordSplitter.Split(strings.ToLower(text), -1) {
		if w == "light" || w == "lite" {
			return w
		}
	}
	return ""
}

func TestLegalPagesTemplateSource(t *testing.T) {
	pages := findFullPageTemplates(t)
	for _, name := range legalTemplateNames {
		src, ok := pages[name]
		if !ok {
			t.Errorf("findFullPageTemplates does not return %s", name)
			continue
		}
		for _, banned := range []string{"account_header.html.tmpl", "theme_toggle.html.tmpl", ".Email", "logout", "data-account-id"} {
			if strings.Contains(src, banned) {
				t.Errorf("%s holds %q (R-07)", name, banned)
			}
		}
		visible := legalActionPattern.ReplaceAllString(src, "")
		if m := legalNumberPattern.FindString(visible); m != "" {
			t.Errorf("%s types the emergency number %s (D-22)", name, m)
		}
		if strings.Contains(visible, "tel:") {
			t.Errorf("%s types a tel link", name)
		}
		// The words are banned in class names and comments. The visible
		// copy names the Light and Dark choice, so the page text is not checked.
		var names []string
		for _, m := range legalNamePattern.FindAllStringSubmatch(src, -1) {
			names = append(names, m[0])
		}
		if w := legalBannedWord(strings.Join(names, " ")); w != "" {
			t.Errorf("%s holds the word %q in a class or comment", name, w)
		}
		if strings.Count(src, `{{template "disclaimer_bar.html.tmpl" .}}`) != 1 {
			t.Errorf("%s must call the bar partial once", name)
		}
		main := strings.Index(src, "/static/css/main.css")
		chrome := strings.Index(src, "/static/css/chrome.css")
		legal := strings.Index(src, "/static/css/legal.css")
		theme := strings.Index(src, "/static/js/theme.js")
		if main < 0 || !(main < chrome && chrome < legal && legal < theme) {
			t.Errorf("%s head must link main.css, chrome.css, legal.css, then theme.js", name)
		}
		if strings.Count(src, "<script") != 1 {
			t.Errorf("%s loads a script other than theme.js", name)
		}
		bar := strings.Index(src, `<body>`)
		if bar < 0 || !strings.HasPrefix(strings.TrimSpace(src[bar+len("<body>"):]), `<div class="top-chrome">`) {
			t.Errorf("%s body must open with div.top-chrome", name)
		}
	}
	if pol, ok := pages["templates/policy.html.tmpl"]; ok && strings.Contains(pol, "CallHref") {
		t.Errorf("policy.html.tmpl uses CallHref, the Policy page has no tap to call link")
	}
}

func TestLegalPagesCSS(t *testing.T) {
	raw := p408ReadStatic(t, "static/css/legal.css")
	css := stripCSSComments(raw)

	for _, banned := range []string{"rgb(", "rgba(", "hsl(", "!important", "999px"} {
		if strings.Contains(css, banned) {
			t.Errorf("legal.css holds %q", banned)
		}
	}
	if regexp.MustCompile(`#[0-9A-Fa-f]{3,8}\b`).MatchString(css) {
		t.Errorf("legal.css holds a hex colour")
	}
	if w := legalBannedWord(raw); w != "" {
		t.Errorf("legal.css holds the word %q", w)
	}

	sizes := map[string]bool{
		"var(--font-size-label)": true, "var(--font-size-body)": true,
		"var(--font-size-heading)": true, "var(--font-size-display)": true,
	}
	weights := map[string]bool{"var(--font-weight-regular)": true, "var(--font-weight-semibold)": true}

	rules := parseCSSRules(raw)
	for _, r := range rules {
		decls := declsOf(r.declBody)
		for name := range decls {
			if strings.HasPrefix(name, "--") {
				t.Errorf("%q declares the custom property %s", r.selectorHead, name)
			}
		}
		if v, ok := decls["font-size"]; ok && !sizes[v] {
			t.Errorf("%q font-size %q is not a size token", r.selectorHead, v)
		}
		if v, ok := decls["font-weight"]; ok && !weights[v] {
			t.Errorf("%q font-weight %q is not a weight token", r.selectorHead, v)
		}
		if strings.Contains(r.declBody, "--color-text-muted") {
			for _, sel := range p408Selectors(r.selectorHead) {
				if sel != ".legal-updated" && sel != ".legal-subtitle" {
					t.Errorf("%q uses --color-text-muted, allowed only on .legal-updated and .legal-subtitle", sel)
				}
			}
		}
		if d, ok := decls["display"]; ok && d != "none" {
			for _, sel := range p408Selectors(r.selectorHead) {
				if strings.Contains(sel, ".legal-") && !strings.Contains(sel, ":not([hidden])") {
					t.Errorf("%q sets display: %s without :not([hidden])", sel, d)
				}
			}
		}
	}

	page, ok := ruleBySelector(rules, ".legal-page")
	if !ok || declsOf(page.declBody)["max-width"] != "640px" {
		t.Errorf(".legal-page must declare max-width: 640px")
	}
	numbers, ok := ruleBySelector(rules, ".legal-numbers")
	if !ok {
		t.Fatalf("legal.css has no .legal-numbers rule")
	}
	nd := declsOf(numbers.declBody)
	bg := nd["background"]
	if bg == "" {
		bg = nd["background-color"]
	}
	if bg != "var(--color-surface)" {
		t.Errorf(".legal-numbers background = %q, want var(--color-surface)", bg)
	}
	if nd["color"] != "var(--color-text)" {
		t.Errorf(".legal-numbers color = %q", nd["color"])
	}
	if nd["border"] != "1px solid var(--color-border)" {
		t.Errorf(".legal-numbers border = %q", nd["border"])
	}
	if nd["border-radius"] != "8px" {
		t.Errorf(".legal-numbers border-radius = %q", nd["border-radius"])
	}
	forced := regexp.MustCompile(`@media \(forced-colors: active\)\s*\{\s*\.legal-numbers\s*\{[^}]*border:\s*1px solid CanvasText`)
	if !forced.MatchString(css) {
		t.Errorf("legal.css lacks the forced-colors CanvasText border on .legal-numbers")
	}
}
