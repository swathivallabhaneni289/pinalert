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

// p15CodeLines drops every line whose trimmed start is a line comment, so a
// contract test counts code and not the prose around it.
func p15CodeLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// p15InOrder fails the test unless every needle occurs in text, each after
// the one before it.
func p15InOrder(t *testing.T, where, text string, needles ...string) {
	t.Helper()
	pos := 0
	for _, needle := range needles {
		idx := strings.Index(text[pos:], needle)
		if idx == -1 {
			t.Errorf("%s: %q is missing or out of order", where, needle)
			return
		}
		pos += idx + len(needle)
	}
}

// TestActivityMountsTheTrustBlock proves the Activity page mounts the one
// shared trust block with no options, fills it from the row's server written
// JSON, refreshes it from the Reopen response and moves focus to its toggle
// (V-38 Activity part, D-20, R-08, A29, A64).
func TestActivityMountsTheTrustBlock(t *testing.T) {
	raw := p15ReadStatic(t, "static/js/activity.js")
	code := p15CodeLines(raw)

	if n := strings.Count(code, "PinalertTrust.createTrustBlock("); n != 1 {
		t.Errorf("activity.js calls createTrustBlock %d times, want 1", n)
	}
	if !strings.Contains(code, "PinalertTrust.createTrustBlock()") {
		t.Errorf("activity.js must call createTrustBlock with no options (D-09)")
	}
	if strings.Contains(code, "showScores") {
		t.Errorf("activity.js mentions showScores, the Activity page shows no score numbers")
	}
	if n := strings.Count(code, "PinalertTrust.updateTrustBlock("); n != 2 {
		t.Errorf("activity.js calls updateTrustBlock %d times, want 2", n)
	}
	if !strings.Contains(code, "PinalertTrust.updateTrustBlock(trustBlock, readRowTrust(row))") {
		t.Errorf("activity.js does not fill the block from readRowTrust(row)")
	}

	reader := jsFunctionBody(t, code, "readRowTrust(row)")
	for _, want := range []string{"try {", "JSON.parse(row.dataset.trust)", "catch (", "return null"} {
		if !strings.Contains(reader, want) {
			t.Errorf("readRowTrust does not contain %q", want)
		}
	}

	p15InOrder(t, "activity.js row loop", code,
		"PinalertVisibility.applyVisibilityClass(row, report)",
		"PinalertTrust.createTrustBlock()",
		"PinalertVotes.createReopenControl(row.dataset.reportId)")

	reopen := jsFunctionBody(t, code, "onReopenClick(row, block)")
	if !strings.Contains(reopen, "result && result.trust") {
		t.Errorf("onReopenClick does not refresh the block from result.trust")
	}
	p15InOrder(t, "onReopenClick", reopen,
		"PinalertVisibility.applyVisibilityClass(row, updated)",
		"PinalertTrust.updateTrustBlock(",
		"block.controls.remove()",
		"block.error.remove()",
		"PinalertTrust.focusToggle(")
	if n := strings.Count(code, "PinalertTrust.focusToggle("); n != 1 {
		t.Errorf("activity.js calls focusToggle %d times, want 1", n)
	}

	for _, banned := range []string{"PinalertTrust.fadeStage(", "createSeverityLabel", "severity-label", "PinalertVotes.createVoteBlock("} {
		if strings.Contains(code, banned) {
			t.Errorf("activity.js contains %q, which the Activity page must not use", banned)
		}
	}
	if regexp.MustCompile(`['"]\.?trust-`).MatchString(code) {
		t.Errorf("activity.js carries a trust class literal")
	}
	if n := strings.Count(raw, "addEventListener"); n != 1 {
		t.Errorf("activity.js has %d addEventListener occurrences, want 1 (A29)", n)
	}
	if n := strings.Count(raw, "Pinalert.showToast("); n != 1 {
		t.Errorf("activity.js has %d Pinalert.showToast calls, want 1 (A29)", n)
	}

	tmpl := p15ReadTemplate(t, "profile.html.tmpl")
	if n := strings.Count(tmpl, "/static/js/trust.js"); n != 1 {
		t.Fatalf("profile.html.tmpl loads trust.js %d times, want 1", n)
	}
	window := findTagWindow(t, tmpl, "/static/js/trust.js")
	if !strings.Contains(window, " defer") || !strings.Contains(window, "?v={{.AssetVersion}}") {
		t.Errorf("the trust.js tag must be deferred and versioned: %s", window)
	}
	p15InOrder(t, "profile.html.tmpl scripts", tmpl,
		"/static/js/visibility.js", "/static/js/trust.js", "/static/js/activity.js")
}
