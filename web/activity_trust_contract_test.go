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
// writes the trust JSON and the expired flag into each row, writes the server's
// severity word into the chip, and keeps the older hooks (A58, A44, V-33).
//
// Departure (quick task 261013-wm5, A26 and A58): the picture's card moves the
// severity word out of the body start and into the chip on the right. The body
// now starts with the tag, then the title, and the chip's severity span holds
// {{.SeverityWord}} once. Trust JSON, expired and hook checks are unchanged.
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

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to
	// D-21): the Report cell starts with the title paragraph (the tag span went
	// with the six tab card), the Status chip holds the server's {{.StatusWord}}
	// once, and the severity word is kept for screen readers as a visually
	// hidden word in the Status cell, because the tint alone must not carry the
	// severity. The Phase 3.1 and Phase 4 hands-on tests are walked on this
	// layout after Phase 3.4 is built.
	reportCell := regexp.MustCompile(`<div class="activity-card__report">\s*<p class="report-row__title">\{\{\.Description\}\}</p>`)
	if !reportCell.MatchString(tmpl) {
		t.Errorf("the Report cell does not start with the title paragraph")
	}
	chipWord := regexp.MustCompile(`<span class="activity-chip"[^>]*>\{\{\.StatusWord\}\}</span>`)
	if n := len(chipWord.FindAllString(tmpl, -1)); n != 1 {
		t.Errorf("the status chip holds {{.StatusWord}} %d times, want exactly 1", n)
	}
	hiddenSeverity := regexp.MustCompile(`<span class="visually-hidden">\{\{\.SeverityWord\}\} severity\. </span>`)
	if n := len(hiddenSeverity.FindAllString(tmpl, -1)); n != 1 {
		t.Errorf("the hidden severity word appears %d times, want exactly 1", n)
	}
	if n := strings.Count(tmpl, ".SeverityWord"); n != 1 {
		t.Errorf(".SeverityWord occurs %d times in the template, want exactly 1", n)
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

// TestDisplayStateDecidesExpired proves one rule decides the state word and
// row class: Retracted wins, then the server's expired flag, then the
// resolver's own state, with the four resolver states left alone (A57).
func TestDisplayStateDecidesExpired(t *testing.T) {
	code := p15CodeLines(p15ReadStatic(t, "static/js/visibility.js"))

	if !strings.Contains(code, "function displayState(report)") || !strings.Contains(code, "function stateWord(report)") {
		t.Fatalf("visibility.js must declare displayState(report) and stateWord(report)")
	}

	display := jsFunctionBody(t, code, "displayState(report)")
	// Plan 03.1-13 (D-13): a finished report of the Past tab carries its own
	// history state, and the server's word decides. The old literals were the
	// first four only; report.history.state joins them, after Resolved.
	for _, want := range []string{"visibilityState(report)", "'retracted'", "report.expired === true", "'expired'",
		"report.history.state"} {
		if !strings.Contains(display, want) {
			t.Errorf("displayState does not contain %q", want)
		}
	}
	p15InOrder(t, "displayState", display, "'retracted'", "report.expired === true")
	p15InOrder(t, "displayState", display, "'retracted'", "report.history.state")
	for _, banned := range []string{"new Date", "Date.now", "getTime", "created_at", "expires_at"} {
		if strings.Contains(display, banned) {
			t.Errorf("displayState contains %q: the browser decides no state from a date (T-31-61)", banned)
		}
	}

	for _, name := range []string{"applyVisibilityClass(el, report)", "updateVisibilityTag(tag, report)"} {
		body := jsFunctionBody(t, code, name)
		if !strings.Contains(body, "displayState(report)") {
			t.Errorf("%s does not read displayState(report)", name)
		}
		if strings.Contains(body, "visibilityState(") {
			t.Errorf("%s still reads visibilityState directly", name)
		}
	}
	if word := jsFunctionBody(t, code, "stateWord(report)"); !strings.Contains(word, "VISIBILITY_TAG_LABELS[displayState(report)]") {
		t.Errorf("stateWord does not read the label table through displayState")
	}

	if !strings.Contains(code, "expired: 'Expired'") {
		t.Errorf("VISIBILITY_TAG_LABELS has no expired label")
	}
	if !strings.Contains(code, "var VISIBILITY_STATES = ['live', 'provisional', 'hidden', 'retracted'];") {
		t.Errorf("VISIBILITY_STATES must stay at exactly the four resolver states")
	}
	if strings.Contains(jsFunctionBody(t, code, "visibilityState(report)"), "expired") {
		t.Errorf("visibilityState must not know about expired")
	}

	for _, want := range []string{"displayState: displayState", "stateWord: stateWord", "visibilityState: visibilityState"} {
		if !strings.Contains(code, want) {
			t.Errorf("the export object does not contain %q", want)
		}
	}
}

// TestActivityReadsExpiredFromTheServer proves the Activity page reads the
// server's expiry decision from the row attribute and compares no dates
// (A44, A57, V-33).
func TestActivityReadsExpiredFromTheServer(t *testing.T) {
	raw := p15ReadStatic(t, "static/js/activity.js")
	code := p15CodeLines(raw)

	const read = "expired: row.dataset.expired === 'true'"
	if n := strings.Count(code, read); n != 1 {
		t.Fatalf("activity.js contains %q %d times, want 1", read, n)
	}
	p15InOrder(t, "activity.js report object", code,
		"visibility_reason: row.dataset.visibilityReason",
		read,
		"PinalertVisibility.updateVisibilityTag(tag, report)")
	if strings.Contains(raw, "vis-"+"expired") {
		t.Errorf("activity.js types the expired row class, applyVisibilityClass writes it through displayState")
	}
	if regexp.MustCompile(`\bDate\b|getTime\(|expires_?[aA]t|ExpiresAt`).MatchString(code) {
		t.Errorf("activity.js compares dates, the server decides expiry")
	}
	if strings.Contains(jsFunctionBody(t, code, "onReopenClick(row, block)"), "expired") {
		t.Errorf("onReopenClick must not carry expired, Reopen exists only on unexpired rows")
	}
	for _, call := range []string{"PinalertVisibility.updateVisibilityTag(tag, report)", "PinalertVisibility.applyVisibilityClass(row, report)"} {
		if !strings.Contains(code, call) {
			t.Errorf("activity.js no longer calls %s", call)
		}
	}
}
