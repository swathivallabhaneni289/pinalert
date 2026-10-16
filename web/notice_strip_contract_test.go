package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of Phase 3.4 plan 05, the laptop
// notice strip (D-29, D-30, UI-SPEC Surface 8a). The strip replaces the Phase 4
// bar that sat on every page: it is drawn on the laptop Nearby page only (and
// so above the Post a report window, which opens over Nearby), never on My
// activity, Settings, Official alerts, the login screen, the verify page, the
// legal pages, the Low data view or the moderator screen, and never on a phone.
//
// Phase 4 SC7 (both legal pages linked from the bar on every page) is retired
// as worded by this phase; the bar contract tests carry the dated note naming
// the places that replace it. Nothing here needs a database or a server.

// stripTemplateCall is the one include of the strip.
const stripTemplateCall = `{{template "notice_strip.html.tmpl" .}}`

func stripReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(TemplatesFS, "templates/"+name)
	if err != nil {
		t.Fatalf("reading templates/%s: %v", name, err)
	}
	return string(raw)
}

// TestStripOnlyOnNearby pins mockup override 1: of all templates only
// index.html.tmpl includes the strip, and it is the first element of the body,
// above the top bar.
func TestStripOnlyOnNearby(t *testing.T) {
	var including []string
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
		if strings.Contains(string(raw), "notice_strip.html.tmpl") {
			including = append(including, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the templates: %v", err)
	}
	if len(including) != 1 || including[0] != "templates/index.html.tmpl" {
		t.Errorf("templates including the strip = %v, want only templates/index.html.tmpl", including)
	}

	// The pages the owner named, one by one, so a rename of the file cannot
	// hide a missing check.
	for _, name := range []string{
		"profile.html.tmpl", "alerts.html.tmpl", "settings.html.tmpl",
		"low.html.tmpl", "moderate.html.tmpl", "login_gate.html.tmpl",
		"verify_outcome.html.tmpl", "reader.html.tmpl", "legal_body.html.tmpl",
	} {
		if strings.Contains(stripReadTemplate(t, name), "notice_strip") {
			t.Errorf("%s includes the strip; it is drawn on Nearby only (D-29)", name)
		}
	}

	index := stripReadTemplate(t, "index.html.tmpl")
	if strings.Count(index, stripTemplateCall) != 1 {
		t.Errorf("index.html.tmpl must include the strip exactly once")
	}
	opening := regexp.MustCompile(`(?s)<body[^>]*>\s*` + regexp.QuoteMeta(stripTemplateCall) + `\s*<div class="top-chrome">`)
	if !opening.MatchString(index) {
		t.Error("the strip must be the first element of the Nearby body, above the top chrome (not sticky, in normal flow)")
	}

	if _, err := fs.ReadFile(TemplatesFS, "templates/disclaimer_bar.html.tmpl"); err == nil {
		t.Error("templates/disclaimer_bar.html.tmpl must be gone")
	}
}

// TestStripSentence pins the exact copy of P34-C24 in the partial: a bold lead,
// the sentences, the configured number in bold, no telephone link, no button.
func TestStripSentence(t *testing.T) {
	src := stripReadTemplate(t, "notice_strip.html.tmpl")

	for _, want := range []string{
		`<aside class="notice-strip" role="note" aria-label="Pinalert notice">`,
		`<strong class="notice-strip__lead">Unofficial, not a government service.</strong>`,
		` Pinalert cannot send help, it shows what people near you are seeing. Reports can sometimes be wrong. In danger, call `,
		`<strong class="notice-strip__number">{{.Chrome.Numbers.Primary}}</strong>.</p>`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("notice_strip.html.tmpl is missing %q", want)
		}
	}
	if got := strings.Count(src, "Unofficial, not a government service."); got != 1 {
		t.Errorf("the lead appears %d times, want 1", got)
	}
	if got := strings.Count(src, `class="notice-strip__icon"`); got != 1 {
		t.Errorf("the icon span appears %d times, want 1", got)
	}

	// The number comes from the Chrome settings, never a typed 112.
	visible := chromeTemplateActionPattern.ReplaceAllString(src, "")
	if regexp.MustCompile(`\b[0-9]{3,}\b`).MatchString(visible) {
		t.Errorf("notice_strip.html.tmpl types a number: %q", visible)
	}
	for _, action := range chromeTemplateActionPattern.FindAllString(src, -1) {
		if action != "{{.Chrome.Numbers.Primary}}" {
			t.Errorf("template action %q: the strip reads only the primary number", action)
		}
	}

	// A laptop cannot dial: no tel link and no call button. The older numbers
	// parenthetical of the Phase 4 bar is not repeated.
	for _, banned := range []string{"tel:", "TelHref", "<button", "HasOlder", "OlderList", "Emergency?", "Call 1"} {
		if strings.Contains(src, banned) {
			t.Errorf("notice_strip.html.tmpl holds %q", banned)
		}
	}

	// No dash of any kind in a string the person reads.
	if strings.ContainsAny(src, string(rune(0x2013))+string(rune(0x2014))) {
		t.Error("notice_strip.html.tmpl holds an en or em dash")
	}
}

// TestStripLinkIsReadTheDisclaimer pins the one link: Read the disclaimer, to
// /disclaimer, no new tab. Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): the link
// also carries data-open-reader="disclaimer", so a click opens the in app reader
// and a visitor without script still follows the plain href.
func TestStripLinkIsReadTheDisclaimer(t *testing.T) {
	src := stripReadTemplate(t, "notice_strip.html.tmpl")
	if got := strings.Count(src, "<a "); got != 1 {
		t.Fatalf("the strip holds %d links, want exactly 1", got)
	}
	want := `<a class="notice-strip__link" href="/disclaimer" data-open-reader="disclaimer">Read the disclaimer</a>`
	if !strings.Contains(src, want) {
		t.Errorf("the strip link is not %q", want)
	}
	if strings.Contains(src, "target=") {
		t.Error("the strip link must not open a new tab")
	}
}

// TestStripHiddenOnPhone pins D-30: the phone shows no strip. phone.css hides
// it inside its one narrow block, and nothing outside that block reveals it
// below 900px. The laptop rules are in chrome.css.
func TestStripHiddenOnPhone(t *testing.T) {
	rules := phoneRules(t)
	hidden := false
	for _, r := range rules {
		if strings.TrimSpace(r.Head) != "body > .notice-strip" {
			continue
		}
		if r.Media != phoneBlockMedia {
			t.Errorf("the strip hide rule sits in %q, want the narrow block %q", r.Media, phoneBlockMedia)
		}
		if r.Decls["display"] == "none" {
			hidden = true
		}
	}
	if !hidden {
		t.Error("phone.css must hide body > .notice-strip with display: none in the narrow block")
	}

	// The laptop rules are in chrome.css and say nothing about a width.
	chrome := stripCSSComments(p16ReadStatic(t, "static/css/chrome.css"))
	rule, ok := ruleBySelector(parseCSSRules(chrome), ".notice-strip")
	if !ok {
		t.Fatal("chrome.css has no .notice-strip rule")
	}
	if declsOf(rule.declBody)["display"] != "flex" {
		t.Error(".notice-strip must be a flex row at the laptop width")
	}
	for _, r := range p16ParseCSS("static/css/chrome.css", p16ReadStatic(t, "static/css/chrome.css")) {
		if strings.Contains(r.Head, ".notice-strip") && strings.Contains(r.Media, "max-width") {
			t.Errorf("chrome.css holds a phone rule for the strip (%q in %q); phone rules live in phone.css", r.Head, r.Media)
		}
	}
}

// TestBarTokenZeroWithoutStrip pins the height token (T-34-24). It is 0px in
// the :root rule of main.css, no width tier restates it, and theme.js writes
// 0px when the strip is absent or not displayed, so the shells that subtract
// the token keep working on every page without a per page edit.
//
// Dated 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
// D-31): the bar on every page is replaced by the laptop strip on Nearby and
// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
// every page) is retired as worded; Disclaimer and Policy are reached from the
// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
// first-time flow, the login screen and the one reader. The Phase 3.1 and
// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
func TestBarTokenZeroWithoutStrip(t *testing.T) {
	const token = "--disclaimer-bar-height"
	main := p16ReadStatic(t, "static/css/main.css")

	root, ok := ruleBySelector(parseCSSRules(main), ":root")
	if !ok {
		t.Fatal("main.css has no exact :root rule")
	}
	if got := declsOf(root.declBody)[token]; got != "0px" {
		t.Errorf(":root %s = %q, want 0px", token, got)
	}

	// No width tier restates the token.
	for _, r := range p16ParseCSS("static/css/main.css", main) {
		if _, has := r.Decls[token]; has && r.Media != "" {
			t.Errorf("%s is restated in %q; no width tier remains", token, r.Media)
		}
	}

	// theme.js writes the strip's height or 0px, and never looks for the old bar.
	raw, err := StaticFS.ReadFile(themeJSPath)
	if err != nil {
		t.Fatalf("reading %s: %v", themeJSPath, err)
	}
	code := stripJSComments(string(raw))
	if !strings.Contains(code, "querySelector('.notice-strip')") {
		t.Error("theme.js must measure .notice-strip")
	}
	if strings.Contains(code, "'.disclaimer-bar'") {
		t.Error("theme.js still looks for the Phase 4 bar")
	}
	measure := jsFunctionBody(t, code, "measureBar()")
	if !strings.Contains(measure, "setProperty(BAR_PROPERTY, '0px')") {
		t.Errorf("measureBar must write 0px when the strip is absent or not displayed: %q", measure)
	}
	watch := jsFunctionBody(t, code, "watchBar()")
	measureAt := strings.Index(watch, "measureBar();")
	returnAt := strings.Index(watch, "if (!barEl)")
	if measureAt < 0 || returnAt < 0 || measureAt > returnAt {
		t.Errorf("watchBar must measure before it returns for a page with no strip, so the token is written as 0px there: %q", watch)
	}
}
