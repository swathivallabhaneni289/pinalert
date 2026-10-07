package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// This file holds the static contract tests of plan 03-13, the shared trust
// block. Every helper carries the p13 prefix so a later plan can add its own
// without a name clash. Nothing here needs a database or a server: the tests
// read the embedded static and template trees and inspect the shipped text.

// p13ReadStatic returns one embedded static file as text.
func p13ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p13ReadTemplate returns one embedded template as text.
func p13ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("failed to read embedded templates/%s: %v", name, err)
	}
	return string(raw)
}

// p13CodeLines returns text without the lines whose trimmed form starts with
// a double slash, so a comment naming a call cannot change an order or a
// count.
func p13CodeLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// p13Ordered requires every needle to be present in text and the first
// occurrences to be strictly increasing.
func p13Ordered(t *testing.T, text, label string, needles ...string) {
	t.Helper()
	prev := -1
	prevNeedle := ""
	for _, needle := range needles {
		idx := strings.Index(text, needle)
		if idx == -1 {
			t.Errorf("%s: %q is missing", label, needle)
			return
		}
		if idx <= prev {
			t.Errorf("%s: %q must come after %q", label, needle, prevNeedle)
			return
		}
		prev = idx
		prevNeedle = needle
	}
}

// p13MustContain fails once per missing needle.
func p13MustContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Errorf("%s: missing %q", label, needle)
		}
	}
}

// p13MustNotContain fails once per present needle.
func p13MustNotContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Errorf("%s: must not contain %q", label, needle)
		}
	}
}

// p13MustMatch fails when the pattern does not match.
func p13MustMatch(t *testing.T, label, text, pattern string) {
	t.Helper()
	if !regexp.MustCompile(pattern).MatchString(text) {
		t.Errorf("%s: no match for %s", label, pattern)
	}
}

// TestTrustModuleLoadsBeforeItsConsumers pins the script order (V-38): the
// trust module sits after the visibility module and before the map, modal
// and feed modules that call it, deferred and versioned like its siblings.
func TestTrustModuleLoadsBeforeItsConsumers(t *testing.T) {
	html := p13ReadTemplate(t, "index.html.tmpl")

	const src = "/static/js/trust.js"
	if n := strings.Count(html, src); n != 1 {
		t.Fatalf("index.html.tmpl references %s %d times, want exactly 1", src, n)
	}
	window := findTagWindow(t, html, src)
	p13MustContain(t, "trust.js tag", window, " defer", "?v={{.AssetVersion}}")

	p13Ordered(t, html, "app module script order",
		"/static/js/app.js",
		"/static/js/votes.js",
		"/static/js/visibility.js",
		"/static/js/trust.js",
		"/static/js/map.js",
		"/static/js/modal.js",
		"/static/js/feed.js",
		"/static/js/account-menu.js",
	)

	lastVendor := vendorScriptSources[len(vendorScriptSources)-1]
	vendorIdx := strings.Index(html, lastVendor)
	if vendorIdx == -1 {
		t.Fatalf("could not locate the last vendor script %q", lastVendor)
	}
	if strings.Index(html, src) <= vendorIdx {
		t.Errorf("trust.js must load after the vendor bridge %q", lastVendor)
	}
}

// TestTrustBlockMountedOnBothSurfaces pins the one builder, the mount order
// and the discipline of the consumers (V-38 feed and popup, D-20, D-09,
// D-14, A33).
func TestTrustBlockMountedOnBothSurfaces(t *testing.T) {
	feed := stripCSSComments(p13ReadStatic(t, "static/js/feed.js"))
	mapJS := stripCSSComments(p13ReadStatic(t, "static/js/map.js"))
	trust := p13ReadStatic(t, "static/js/trust.js")

	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS} {
		p13MustContain(t, label, text,
			"PinalertTrust.createTrustBlock(", "PinalertTrust.updateTrustBlock(")
	}

	p13MustContain(t, "feed.js", feed, "PinalertTrust.createTrustBlock()")
	p13MustNotContain(t, "feed.js", feed, "showScores")
	p13MustMatch(t, "map.js", mapJS,
		`PinalertTrust\.createTrustBlock\(\s*\{\s*showScores:\s*true\s*\}\s*\)`)

	createRow := jsFunctionBody(t, feed, "createRow(id)")
	p13Ordered(t, createRow, "feed.js createRow",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustBlock(",
		"PinalertVotes.createVoteBlock(",
	)
	p13Ordered(t, p13CodeLines(mapJS), "map.js popup builder",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustBlock(",
		"PinalertVotes.createVoteBlock(",
	)

	updateRow := jsFunctionBody(t, feed, "updateRow(row, report)")
	p13MustContain(t, "feed.js updateRow", updateRow,
		"PinalertTrust.updateTrustBlock(row.trust, report.trust)")

	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS} {
		p13MustNotContain(t, label, text,
			"'trust-", "\"trust-",
			"trust-block", "trust-num", "trust-signal", "trust-icon",
			"confirmed_places", "disputed_places", "confirmed_text", "disputed_text",
			"lead_sentence", "fade_stage", "score_sentence", ".currency", ".reporter",
		)
	}

	sortBody := jsFunctionBody(t, feed, "sortReports(reports)")
	if strings.Contains(sortBody, "trust") {
		t.Errorf("sortReports reads trust, but the block never reorders a report (D-14)")
	}

	p13MustContain(t, "trust.js exports", trust,
		"window.PinalertTrust = {",
		"createTrustBlock: createTrustBlock",
		"updateTrustBlock: updateTrustBlock",
		"fadeStage: fadeStage",
		"focusToggle: focusToggle",
	)

	create := jsFunctionBody(t, trust, "createTrustBlock(options)")
	p13MustContain(t, "createTrustBlock", create, "'trust-panel-'")
	if regexp.MustCompile(`\breport\b`).MatchString(p13CodeLines(create)) {
		t.Errorf("createTrustBlock names a report, so a panel id could come from the report id")
	}

	p13MustContain(t, "trust.js", trust,
		". Shows what these mean.",
		", score",
		"visually-hidden",
		"aria-hidden",
		"trust-icon trust-icon--clock",
		"trust-icon trust-icon--user",
		"trust-num",
		"trust-block__count--second",
		"trust-signal--second",
		"trust-block__panel",
		"trust-block__line",
	)
	p13MustMatch(t, "trust.js PANEL_LINES", trust,
		`PANEL_LINES\s*=\s*\[\s*'lead',\s*'confirmed',\s*'disputed',\s*'currency',\s*'currency_score',\s*'reporter',\s*'reporter_score'\s*\]`)

	panel := jsFunctionBody(t, trust, "renderTrustPanel(block, trust)")
	p13MustContain(t, "renderTrustPanel", panel,
		"showScores", "lead_sentence", "confirmed_sentence", "disputed_sentence", "score_sentence")

	update := jsFunctionBody(t, trust, "updateTrustBlock(block, trust)")
	p13MustContain(t, "updateTrustBlock", update, "validTrust(", "hidden")

	// The open state is never reset by an update, so a panel stays open
	// across every poll.
	for _, name := range []string{
		"updateTrustBlock(block, trust)",
		"renderTrustCounts(block, trust)",
		"renderTrustSignal(signal, shown, label, score)",
		"renderTrustPanel(block, trust)",
	} {
		body := jsFunctionBody(t, trust, name)
		p13MustNotContain(t, name, body, "aria-expanded", "setPanelOpen(")
	}

	counts := jsFunctionBody(t, trust, "renderTrustCounts(block, trust)")
	p13MustContain(t, "renderTrustCounts", counts, "confirmed_text", "disputed_text")
	p13MustNotContain(t, "renderTrustCounts", counts, "=== 0", "!== 0", "> 0")

	if regexp.MustCompile(`trust\.[a-z_]+\s*=[^=]`).MatchString(trust) {
		t.Errorf("trust.js assigns to a trust field, but the browser never changes a count (A33)")
	}
}

// TestFadeUsesCombinedStage proves the browser holds no fade threshold of its
// own: every fade site asks the trust module for the worse of the time stage
// and the server stage (V-37, D-07, R-02).
func TestFadeUsesCombinedStage(t *testing.T) {
	feed := stripCSSComments(p13ReadStatic(t, "static/js/feed.js"))
	mapJS := stripCSSComments(p13ReadStatic(t, "static/js/map.js"))
	trust := p13ReadStatic(t, "static/js/trust.js")

	for label, text := range map[string]string{"feed.js": feed, "map.js": mapJS} {
		if strings.Contains(text, "Pinalert.ageStage(") {
			t.Errorf("%s calls the time only stage directly", label)
		}
	}

	sites := []struct{ label, text, fn string }{
		{"feed.js updateRow", feed, "updateRow(row, report)"},
		{"feed.js refreshAgeAndTime", feed, "refreshAgeAndTime()"},
		{"map.js buildBadgeElement", mapJS, "buildBadgeElement(report)"},
		{"map.js popupStateClasses", mapJS, "popupStateClasses(report)"},
		{"map.js syncPopupState", mapJS, "syncPopupState(marker, report)"},
	}
	for _, s := range sites {
		body := jsFunctionBody(t, s.text, s.fn)
		if !strings.Contains(body, "PinalertTrust.fadeStage(report)") {
			t.Errorf("%s does not call PinalertTrust.fadeStage(report)", s.label)
		}
	}

	code := p13CodeLines(trust)
	if n := strings.Count(code, "Pinalert.ageStage("); n != 1 {
		t.Errorf("trust.js calls Pinalert.ageStage( %d times on code lines, want exactly 1", n)
	}
	fade := jsFunctionBody(t, code, "fadeStage(report)")
	p13MustContain(t, "fadeStage", fade, "Pinalert.ageStage(", "fade_stage", "FADE_STAGES", "indexOf")
	p13MustMatch(t, "trust.js FADE_STAGES", trust, `FADE_STAGES\s*=\s*\[\s*'fresh',\s*'aging',\s*'stale'\s*\]`)
}

// TestTrustModuleHasNoThresholdsOrSinks keeps the module a pure renderer
// (R-02, T-03-06, A69): no markup sink, no network, no date arithmetic, no
// server copy, no threshold and no long dash.
func TestTrustModuleHasNoThresholdsOrSinks(t *testing.T) {
	trust := p13ReadStatic(t, "static/js/trust.js")

	// The two long dash code points are written as escapes so this test file
	// stays dash free.
	if strings.ContainsRune(trust, '\u2014') || strings.ContainsRune(trust, '\u2013') {
		t.Errorf("trust.js contains a long dash (A69)")
	}

	p13MustNotContain(t, "trust.js sinks", trust,
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write",
		"DOMParser", "createContextualFragment")
	p13MustNotContain(t, "trust.js network and dates", trust,
		"fetch(", "XMLHttpRequest", "Date.now", "new Date", "getTime", "created_at", "expires_at")
	p13MustNotContain(t, "trust.js server copy", trust,
		"Up to date", "Getting old", "re-confirming", "Too early", "Reliable reporter",
		"Mixed record", "Unreliable reporter", "New reporter", "Counts separate places",
		"out of 100", "Unconfirmed", "confirmations before")
	p13MustContain(t, "trust.js", trust, "Pinalert.setText(", "'use strict'")
	p13MustMatch(t, "trust.js REPORTER_TAGS", trust,
		`REPORTER_TAGS\s*=\s*\[\s*'reliable',\s*'mixed',\s*'unreliable',\s*'new',\s*'none'\s*\]`)
	p13MustMatch(t, "trust.js CURRENCY_BANDS", trust,
		`CURRENCY_BANDS\s*=\s*\[\s*'up_to_date',\s*'getting_old',\s*'needs_reconfirming',\s*'not_yet_corroborated',\s*'not_applicable'\s*\]`)
	p13MustNotContain(t, "trust.js row state classes", trust, "'vis-'", "'sev-'", "'age-'")

	code := p13CodeLines(trust)
	if regexp.MustCompile(`(?i)threshold|cutoff|min_?score|half_?life|window_minutes|recent_places|judged_reports|corroborated_reports`).MatchString(code) {
		t.Errorf("trust.js names a threshold or a server fact field on a code line")
	}
	if loc := regexp.MustCompile(`\b(?:[2-9]|[1-9][0-9]+)\b`).FindString(code); loc != "" {
		t.Errorf("trust.js has the numeric literal %q on a code line, only 0, 1 and -1 may appear", loc)
	}
}

// TestTrustDisclosure pins the keyboard and click behaviour of the toggle and
// the one Escape listener (V-39, renamed per A5). Plan 03-19 later extends
// the listener with the popup branches and must keep the single document
// listener.
func TestTrustDisclosure(t *testing.T) {
	trust := p13ReadStatic(t, "static/js/trust.js")
	code := p13CodeLines(trust)

	p13MustContain(t, "trust.js code", code,
		"aria-expanded", "aria-controls", "addEventListener('click'",
		"'Enter'", "' '", "'Escape'", "stopPropagation", "preventDefault")

	create := jsFunctionBody(t, trust, "createTrustBlock(options)")
	p13MustContain(t, "createTrustBlock", create, "'button'", "stopPropagation", "'Enter'")
	p13MustNotContain(t, "createTrustBlock", create, "preventDefault")

	const listener = "document.addEventListener("
	if n := strings.Count(code, listener); n != 1 {
		t.Fatalf("trust.js has %d %s calls on code lines, want exactly 1 (A23)", n, listener)
	}
	for _, line := range strings.Split(code, "\n") {
		if strings.Contains(line, listener) {
			p13MustContain(t, "the document listener statement", line, "'keydown'", "onDocumentEscape", ", true)")
		}
	}

	escape := jsFunctionBody(t, trust, "onDocumentEscape(evt)")
	p13MustContain(t, "onDocumentEscape", escape, "'Escape'", "closeOneLevel()")
	p13Ordered(t, escape, "onDocumentEscape", "if (!acted)", "stopPropagation()")
	p13Ordered(t, escape, "onDocumentEscape", "if (!acted)", "preventDefault()")
	if n := strings.Count(escape, "stopPropagation()"); n != 1 {
		t.Errorf("onDocumentEscape calls stopPropagation() %d times, want exactly 1", n)
	}

	one := jsFunctionBody(t, trust, "closeOneLevel()")
	p13Ordered(t, one, "closeOneLevel", "'report-modal'", "'account-menu'",
		"PinalertVotes.cancelResolveConfirm(", "closeRowPanel(")
	p13MustContain(t, "closeOneLevel", one, "surfaceIsDisplayed(")

	closeRow := jsFunctionBody(t, trust, "closeRowPanel(row)")
	p13MustContain(t, "closeRowPanel", closeRow, "aria-expanded", "setPanelOpen(", ".focus()")

	displayed := jsFunctionBody(t, trust, "surfaceIsDisplayed(el)")
	p13MustContain(t, "surfaceIsDisplayed", displayed, "getClientRects")

	// A5: the hover, title and tooltip design is dropped, and there is no
	// live region because the 30 second poll would re-announce it.
	p13MustNotContain(t, "trust.js", trust,
		"aria-live", "tooltip", "mouseenter", "mouseover", "'title'", ".title")
}
