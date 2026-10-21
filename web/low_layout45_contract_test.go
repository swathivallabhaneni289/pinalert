package web

import (
	"strings"
	"testing"
)

// Plan 03.4-45 (owner, 2026-10-21): the Low data view uses the whole laptop
// screen, its Report an emergency button is the app's one filled button, and
// Flag ends each card as a calm full width row instead of a third box under
// Confirm. These tests pin the template grouping and the low.css rules that
// carry those three asks; the phone keeps plan 44's one column.

const p45Laptop = "@media (min-width: 900px)"

// TestLowLayout45Grouping: the top row and the blocks sit in one left column
// wrapper, and the Active reports heading, the list and its three states sit
// in one reports wrapper, in that order.
func TestLowLayout45Grouping(t *testing.T) {
	html := p418Template(t)
	at := func(s string) int {
		t.Helper()
		i := strings.Index(html, s)
		if i < 0 {
			t.Fatalf("the template is missing %q", s)
		}
		return i
	}
	order := []string{
		`<div class="low-side">`,
		`id="low-full"`,
		`id="low-reload"`,
		`class="low-page__head"`,
		`id="low-report"`,
		`id="queue-strip"`,
		`id="low-stale"`,
		`<div class="low-reports">`,
		`<h2 id="low-list-heading">Active reports</h2>`,
		`id="low-list"`,
		`id="low-skeleton"`,
		`id="low-empty"`,
		`id="low-error"`,
		`</main>`,
	}
	for i := 1; i < len(order); i++ {
		if at(order[i-1]) >= at(order[i]) {
			t.Errorf("%q must come before %q", order[i-1], order[i])
		}
	}
	if n := strings.Count(html, `class="low-side"`); n != 1 {
		t.Errorf("low-side appears %d times, want 1", n)
	}
	if n := strings.Count(html, `class="low-reports"`); n != 1 {
		t.Errorf("low-reports appears %d times, want 1", n)
	}
}

// TestLowLayout45Laptop: from 900px a 360px sticky left column and a reports
// grid of cards at least 360px wide (scaled with the text), with a right inset
// that clears the fixed person and light and dark buttons.
func TestLowLayout45Laptop(t *testing.T) {
	const css = "static/css/low.css"
	p16MustDecl(t, "laptop page", p31mRule(t, css, p45Laptop, ".low-page:not([hidden])"), map[string]string{
		"display":               "grid",
		"grid-template-columns": "360px minmax(0, 1fr)",
		"column-gap":            "var(--space-xl)",
		"align-items":           "start",
		"max-width":             "none",
		"margin":                "0",
		"padding":               "0 calc(var(--touch-target-min) + var(--space-lg) + var(--space-md)) var(--space-xl) var(--space-xl)",
	})
	p16MustDecl(t, "left column", p31mRule(t, css, p45Laptop, ".low-side"), map[string]string{
		"position":   "sticky",
		"top":        "0",
		"max-height": "100vh",
		"overflow-y": "auto",
	})
	p16MustDecl(t, "reports grid", p31mRule(t, css, p45Laptop, ".low-list:not([hidden])"), map[string]string{
		"display":               "grid",
		"grid-template-columns": "repeat(auto-fill, minmax(min(100%, 22.5rem), 1fr))",
	})
	p16MustDecl(t, "card body", p31mRule(t, css, p45Laptop, ".report-row--low > .report-row__body:not([hidden])"), map[string]string{
		"display":        "flex",
		"flex-direction": "column",
		"align-self":     "stretch",
	})
	p16MustDecl(t, "action rows at the bottom", p31mRule(t, css, p45Laptop, ".report-row--low .report-row__body > .trust-block"), map[string]string{
		"margin-bottom": "auto",
	})
}

// TestLowLayout45ReportButton: Report an emergency is the filled button of
// Post a report: full width, at least 52px, 8px corners, the plus glyph.
func TestLowLayout45ReportButton(t *testing.T) {
	const css = "static/css/low.css"
	p16MustDecl(t, "report button", p31mRule(t, css, "", ".low-page__actions .low-page__report"), map[string]string{
		"flex":          "1 1 100%",
		"min-height":    "52px",
		"border-radius": "8px",
	})
	p16MustDecl(t, "report glyph", p31mRule(t, css, "", ".low-page__report::before"), map[string]string{
		"width":            "20px",
		"height":           "20px",
		"background-color": "currentColor",
		"mask":             `url("/static/icons/plus.svg") center / contain no-repeat`,
	})
	button := findTagWindow(t, p418Template(t), `id="low-report"`)
	for _, want := range []string{`class="btn btn--primary low-page__report"`, `type="button"`} {
		if !strings.Contains(button, want) {
			t.Errorf("the Report button is missing %s: %s", want, button)
		}
	}
}

// TestLowLayout45FlagRow: Flag is a full width row with a muted hairline
// above, the alert glyph, no box and no underline, a hover fill and glyphs
// that stay visible under forced colours.
func TestLowLayout45FlagRow(t *testing.T) {
	const css = "static/css/low.css"
	const head = ".report-row--low .flag-control:not([hidden])"
	p16MustDecl(t, "flag row", p31mRule(t, css, "", head), map[string]string{
		"display":         "flex",
		"width":           "100%",
		"min-height":      "48px",
		"border":          "0",
		"border-top":      "1px solid var(--color-text-muted)",
		"border-radius":   "0",
		"background":      "transparent",
		"color":           "var(--color-text)",
		"font-weight":     "var(--font-weight-semibold)",
		"text-align":      "left",
		"text-decoration": "none",
	})
	p16MustDecl(t, "flag glyph", p31mRule(t, css, "", head+"::before"), map[string]string{
		"width":  "20px",
		"height": "20px",
		"mask":   `url("/static/icons/circle-alert.svg") center / contain no-repeat`,
	})
	p16MustDecl(t, "flag hover", p31mRule(t, css, "", head+`:not([aria-disabled="true"]):hover`), map[string]string{
		"background": "var(--color-border)",
	})
	for _, glyph := range []string{".low-page__report::before", head + "::before"} {
		p16MustDecl(t, "forced colours "+glyph, p31mRule(t, css, "@media (forced-colors: active)", glyph), map[string]string{
			"forced-color-adjust": "none",
			"background-color":    "ButtonText",
		})
	}
}
