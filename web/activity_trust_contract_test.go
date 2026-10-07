// activity_trust_contract_test.go: plan 03-15's contract tests for the
// Activity page. The server writes the shared trust object, an expired flag
// and a severity word into each row, and activity.js mounts the one shared
// trust block from them. These tests read the embedded template and scripts,
// the same way votes_contract_test.go does.
package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// p15ReadStatic returns an embedded static file as text.
func p15ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p15ReadTemplate returns an embedded template as text.
func p15ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("failed to read embedded templates/%s: %v", name, err)
	}
	return string(raw)
}

// TestProfileTemplateUsesServerSeverityAndTrust proves the profile template
// writes the trust JSON and the expired flag into each row, prints the
// severity word first in the body, and keeps the older hooks (A58, A44, V-33).
func TestProfileTemplateUsesServerSeverityAndTrust(t *testing.T) {
	tmpl := p15ReadTemplate(t, "profile.html.tmpl")

	liStart := strings.Index(tmpl, `<li class="report-row`)
	if liStart == -1 {
		t.Fatalf("profile.html.tmpl has no report row li")
	}
	liEnd := strings.Index(tmpl[liStart:], ">")
	if liEnd == -1 {
		t.Fatalf("report row li has no closing angle bracket")
	}
	opening := tmpl[liStart : liStart+liEnd]
	for _, want := range []string{`data-trust="{{.TrustJSON}}"`, `data-expired="{{.Expired}}"`} {
		if !strings.Contains(opening, want) {
			t.Errorf("the report row opening tag does not contain %s", want)
		}
	}
	if n := strings.Count(tmpl, ".TrustJSON"); n != 1 {
		t.Errorf(".TrustJSON occurs %d times in the template, want exactly 1", n)
	}

	severityFirst := regexp.MustCompile(`<div class="report-row__body">\s*<span class="severity-label severity-label--\{\{\.SeverityLevel\}\}">\{\{\.SeverityWord\}\}</span>\s*<p class="report-row__title">`)
	if !severityFirst.MatchString(tmpl) {
		t.Errorf("the report row body does not start with the server rendered severity label before the title")
	}

	for _, hook := range []string{"data-report-id=", "data-visibility=", "data-visibility-reason=", "data-can-reopen"} {
		if !strings.Contains(tmpl, hook) {
			t.Errorf("profile.html.tmpl lost the %q hook", hook)
		}
	}
}
