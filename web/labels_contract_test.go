package web

// Plan 03-16 contract tests: the visible severity word on every feed row, the
// shared shelter line formatter, the rounded read only labels, and the row
// meta and title legibility.
//
// Amendments pinned here: A1, A24, A26, A37, A38, A40, A42, A44, A45, A53,
// A55, RR-7 and V-40 narrowed to controls (owner Q2).
//
// Honest limit: these tests prove shipped source text and declared CSS. They
// cannot prove how a browser lays a label out, so the 200 percent text size
// check and the 320px wrapping check stay in the plan 03-20 V-42 manual
// script.

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// p16ReadStatic reads one embedded static file and fails the test on error.
func p16ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return string(raw)
}

// p16Ordered fails when a needle is missing or when the first occurrences are
// not strictly increasing, naming the label and the offending pair.
func p16Ordered(t *testing.T, text, label string, needles ...string) {
	t.Helper()
	prevIdx := -1
	prevNeedle := ""
	for _, needle := range needles {
		idx := strings.Index(text, needle)
		if idx == -1 {
			t.Errorf("%s: missing %q", label, needle)
			return
		}
		if idx <= prevIdx {
			t.Errorf("%s: %q must come after %q", label, needle, prevNeedle)
			return
		}
		prevIdx = idx
		prevNeedle = needle
	}
}

// p16MustContain fails once per missing needle.
func p16MustContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Errorf("%s: missing %q", label, needle)
		}
	}
}

// p16MustNotContain fails once per present needle.
func p16MustNotContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Errorf("%s: must not contain %q", label, needle)
		}
	}
}

// TestRowSeverityLabelMountsFirst pins A26 and A45: the severity word is the
// first child of the row body, built and refreshed through the shared trust.js
// builder, and feed.js owns neither the class name nor the word.
func TestRowSeverityLabelMountsFirst(t *testing.T) {
	feed := p16ReadStatic(t, "static/js/feed.js")
	trust := p16ReadStatic(t, "static/js/trust.js")

	create := jsFunctionBody(t, feed, "createRow(id)")
	p16Ordered(t, create, "feed.js createRow",
		"PinalertTrust.createSeverityLabel(",
		"body.appendChild(severityLabel)",
		"body.appendChild(title)",
		"body.appendChild(meta)",
		"PinalertVisibility.createVisibilityTag(",
		"PinalertTrust.createTrustBlock(",
		"PinalertVotes.createVoteBlock(",
	)
	if strings.Index(create, "body.appendChild(") != strings.Index(create, "body.appendChild(severityLabel)") {
		t.Errorf("feed.js createRow: the severity label must be the first child appended to the row body (A45)")
	}
	p16MustContain(t, "feed.js createRow", create, "severity: severityLabel")

	update := jsFunctionBody(t, feed, "updateRow(row, report)")
	p16MustContain(t, "feed.js updateRow", update, "PinalertTrust.updateSeverityLabel(row.severity, report)")

	p16MustNotContain(t, "feed.js", feed, "severity-label", "SEVERITY_WORDS")

	build := jsFunctionBody(t, trust, "createSeverityLabel()")
	p16MustContain(t, "createSeverityLabel", build,
		"document.createElement('span')", "'severity-label'", "setAttribute('hidden', '')")

	refresh := jsFunctionBody(t, trust, "updateSeverityLabel(label, report)")
	p16MustContain(t, "updateSeverityLabel", refresh,
		"Pinalert.SEVERITY_WORDS", "hasOwnProperty", "'severity-label severity-label--'",
		"Pinalert.setText(", "removeAttribute('hidden')", "setAttribute('hidden', '')")
	p16MustNotContain(t, "updateSeverityLabel", refresh,
		"innerHTML", "'Critical'", "'Medium'", "'Low'")

	p16MustContain(t, "trust.js exports", trust,
		"createSeverityLabel: createSeverityLabel",
		"updateSeverityLabel: updateSeverityLabel",
		"createTrustBlock: createTrustBlock",
		"updateTrustBlock: updateTrustBlock",
		"fadeStage: fadeStage",
		"focusToggle: focusToggle",
	)
}

// TestShelterLineFormatterIsShared pins A24: one shelter status and headcount
// formatter in app.js, called by feed.js, never containing the word Capacity.
func TestShelterLineFormatterIsShared(t *testing.T) {
	app := p16ReadStatic(t, "static/js/app.js")
	feed := p16ReadStatic(t, "static/js/feed.js")

	words := regexp.MustCompile(`SEVERITY_WORDS\s*=\s*\{\s*low:\s*'Low',\s*medium:\s*'Medium',\s*critical:\s*'Critical'\s*\}`)
	if !words.MatchString(app) {
		t.Errorf("app.js: SEVERITY_WORDS must be exactly low Low, medium Medium, critical Critical")
	}
	p16MustContain(t, "app.js exports", app,
		"SEVERITY_WORDS: SEVERITY_WORDS", "formatShelterLine: formatShelterLine")

	format := jsFunctionBody(t, app, "formatShelterLine(report)")
	p16MustContain(t, "formatShelterLine", format,
		"'shelter_open'", "CAPACITY_STATUSES.indexOf(", "Number.isInteger(", "' person'", "' people'")
	p16MustNotContain(t, "formatShelterLine", format, "Capacity")

	meta := jsFunctionBody(t, feed, "buildMetaText(report)")
	p16MustContain(t, "feed.js buildMetaText", meta, "Pinalert.formatShelterLine(report)")
	p16MustNotContain(t, "feed.js buildMetaText", meta, "' person'", "' people'", "CAPACITY_STATUSES")
	p16MustNotContain(t, "feed.js", feed, "CAPACITY_LABELS")
}
