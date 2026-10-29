package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// Phase 8 plan 04 (D-05, D-06, SC5): the public status page a shared link opens.
// These contract tests read the embedded template and stylesheet only; the
// served page is proved end to end in internal/api/handlers.

const (
	publicPageTemplate = "templates/share_status.html.tmpl"
	publicPageCSS      = "static/css/public.css"
)

var publicTemplateComment = regexp.MustCompile(`(?s)\{\{/\*.*?\*/\}\}`)

// publicPageMarkup returns the template text with its leading comment removed,
// so the checks below read markup and actions only.
func publicPageMarkup(t *testing.T) string {
	t.Helper()
	raw, err := fs.ReadFile(TemplatesFS, publicPageTemplate)
	if err != nil {
		t.Fatalf("reading %s: %v", publicPageTemplate, err)
	}
	return publicTemplateComment.ReplaceAllString(string(raw), "")
}

// TestPublicPageHasNoScript pins that the page runs no script at all and does
// not call app_head, which would register the service worker for a reader who
// has no account (UI-SPEC 6a).
func TestPublicPageHasNoScript(t *testing.T) {
	markup := publicPageMarkup(t)
	if !strings.Contains(markup, `{{define "share_status"}}`) {
		t.Errorf("%s does not define share_status", publicPageTemplate)
	}
	for _, banned := range []string{"<script", "app_head", "theme.js", "onclick=", "onload=", "javascript:"} {
		if strings.Contains(strings.ToLower(markup), banned) {
			t.Errorf("%s holds %q: the public page carries no script", publicPageTemplate, banned)
		}
	}
	// The preview tags: the title is the h1 text, the description the fixed line.
	for _, want := range []string{
		`<meta name="robots" content="noindex, nofollow">`,
		`<meta property="og:title" content="{{.Title}}">`,
		`<meta property="og:description" content="{{.OGDescription}}">`,
		`<meta property="og:url" content="{{.CanonicalURL}}">`,
		`<meta property="og:type" content="website">`,
		`<meta property="og:image" content="{{.OGImageURL}}">`,
		`<h1 class="public-title">{{.Title}}</h1>`,
		`<title>{{.Title}} · Pinalert</title>`,
		`href="/login">Sign in to see more</a>`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("%s does not contain %q", publicPageTemplate, want)
		}
	}
	// The page names no field that could hold the reporter's words or a place.
	for _, banned := range []string{".Description", ".Latitude", ".Longitude", ".Distance", ".Username", ".Email", ".Comments"} {
		if strings.Contains(markup, banned) {
			t.Errorf("%s reads %q: the public page shows no such field", publicPageTemplate, banned)
		}
	}
}

// TestPublicPageControlsMeetTouchTarget pins that the three buttons of the page
// (Sign in to see more, Disclaimer, Policy) are at least the 44px token high.
func TestPublicPageControlsMeetTouchTarget(t *testing.T) {
	raw, err := fs.ReadFile(StaticFS, publicPageCSS)
	if err != nil {
		t.Fatalf("reading %s: %v", publicPageCSS, err)
	}
	rules := parseCSSRules(string(raw))
	rule, ok := ruleBySelector(rules, ".public-page .btn")
	if !ok {
		t.Fatalf("%s has no .public-page .btn rule", publicPageCSS)
	}
	if got := declsOf(rule.declBody)["min-height"]; got != "var(--touch-target-min)" {
		t.Errorf(".public-page .btn min-height = %q, want var(--touch-target-min)", got)
	}
	markup := publicPageMarkup(t)
	for _, want := range []string{
		`class="btn btn--primary public-signin" href="/login"`,
		`class="btn" href="/disclaimer"`,
		`class="btn" href="/policy"`,
	} {
		if !strings.Contains(markup, want) {
			t.Errorf("%s does not draw %q as a .btn", publicPageTemplate, want)
		}
	}
}
