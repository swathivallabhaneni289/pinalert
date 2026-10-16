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

// Amended 2026-10-17 by Phase 3.4 plan 08 (owner decision, 03.4-CONTEXT D-31,
// D-06): the Disclaimer and the Policy are one reader. Two templates hold the
// legal text and the page around it: the standalone reader page that both
// addresses render, and the body partials that the page, the in app reader
// window and the first time flow all include. The two old page templates are
// gone.
const (
	legalReaderTemplate = "templates/reader.html.tmpl"
	legalBodyTemplate   = "templates/legal_body.html.tmpl"
)

var legalTemplateNames = []string{legalReaderTemplate, legalBodyTemplate}

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
	sources := map[string]string{legalBodyTemplate: stripReadTemplate(t, "legal_body.html.tmpl")}
	reader, ok := pages[legalReaderTemplate]
	if !ok {
		t.Fatalf("findFullPageTemplates does not return %s", legalReaderTemplate)
	}
	sources[legalReaderTemplate] = reader
	for _, gone := range []string{"templates/disclaimer.html.tmpl", "templates/policy.html.tmpl"} {
		if _, still := pages[gone]; still {
			t.Errorf("%s must be gone: the reader page and the body partials replace it (D-31)", gone)
		}
	}

	for _, name := range legalTemplateNames {
		src := sources[name]
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
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
		// D-31): the bar on every page is replaced by the laptop strip on Nearby and
		// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
		// every page) is retired as worded; Disclaimer and Policy are reached from the
		// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
		// first-time flow, the login screen and the one reader. The Phase 3.1 and
		// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
		// The two legal pages carry no bar and no strip.
		if strings.Contains(src, "disclaimer_bar.html.tmpl") || strings.Contains(src, "notice_strip.html.tmpl") {
			t.Errorf("%s must call neither the old bar partial nor the strip", name)
		}
	}

	// The body partials: both are defined once and the Policy part has no tap to
	// call link, which belongs to the Disclaimer's numbers block.
	body := sources[legalBodyTemplate]
	for _, def := range []string{`{{define "legal_disclaimer_body"}}`, `{{define "legal_policy_body"}}`} {
		if strings.Count(body, def) != 1 {
			t.Errorf("legal_body.html.tmpl must define %s exactly once", def)
		}
	}
	_, policyPart, found := strings.Cut(body, `{{define "legal_policy_body"}}`)
	if !found {
		t.Fatalf("legal_body.html.tmpl has no policy part")
	}
	if strings.Contains(policyPart, "CallHref") {
		t.Errorf("the policy part uses CallHref, the Policy has no tap to call link")
	}

	// The reader page: head, one body wrapper, the two partials.
	main := strings.Index(reader, "/static/css/main.css")
	chrome := strings.Index(reader, "/static/css/chrome.css")
	appBar := strings.Index(reader, "/static/css/app-bar.css")
	legal := strings.Index(reader, "/static/css/legal.css")
	theme := strings.Index(reader, "/static/js/theme.js")
	if main < 0 || !(main < chrome && chrome < appBar && appBar < legal && legal < theme) {
		t.Errorf("reader.html.tmpl head must link main.css, chrome.css, app-bar.css (for the X glyph), legal.css, then theme.js")
	}
	if strings.Count(reader, "<script") != 1 {
		t.Errorf("reader.html.tmpl loads a script other than theme.js")
	}
	bodyAt := strings.Index(reader, `<body>`)
	if bodyAt < 0 || !strings.HasPrefix(strings.TrimSpace(reader[bodyAt+len("<body>"):]), `<div class="legal-reader"`) {
		t.Errorf("reader.html.tmpl body must open with div.legal-reader, with no bar wrapper")
	}
	if strings.Contains(reader, "Back to Pinalert") {
		t.Errorf("reader.html.tmpl keeps a Back to Pinalert link; the X replaces it (D-31)")
	}
	for _, part := range []string{`{{template "legal_disclaimer_body" .Legal}}`, `{{template "legal_policy_body" .Legal}}`} {
		if strings.Count(reader, part) != 1 {
			t.Errorf("reader.html.tmpl must include %s once", part)
		}
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
				if !strings.HasSuffix(sel, ".legal-updated") && !strings.HasSuffix(sel, ".legal-subtitle") {
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

	// Amended 2026-10-17 by Phase 3.4 plan 08 (03.4-CONTEXT D-06, D-31): the
	// standalone pages use the full laptop width, so the old 640px reading
	// column (.legal-page max-width) is gone. The page is the Settings split, a
	// 260px left column and the text in 1fr, and a line of text is capped at
	// 72ch, left aligned, inside that column.
	if _, old := ruleBySelector(rules, ".legal-page"); old {
		t.Errorf("legal.css still holds the centred .legal-page column (D-06)")
	}
	if strings.Contains(css, "640px") || strings.Contains(css, "margin: 0 auto") {
		t.Errorf("legal.css holds the old 640px column or a centring margin (D-06)")
	}
	if !regexp.MustCompile(`@media \(min-width: 900px\)\s*\{\s*\.legal-reader:not\(\[hidden\]\)\s*\{[^}]*grid-template-columns:\s*260px 1fr`).MatchString(css) {
		t.Errorf("legal.css lacks the laptop grid: .legal-reader with grid-template-columns 260px 1fr inside one min-width 900px block")
	}
	if !regexp.MustCompile(`max-width:\s*72ch`).MatchString(css) {
		t.Errorf("legal.css lacks the 72ch line length cap")
	}
	box, ok := ruleBySelector(rules, ".legal-box")
	if !ok || declsOf(box.declBody)["background"] != "var(--color-surface)" {
		t.Errorf(".legal-box must be the --color-surface reader box")
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
