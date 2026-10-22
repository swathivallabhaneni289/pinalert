package web

import (
	"strings"
	"testing"
)

// Owner, 2026-10-22, on the Low data view: "align them neatly. Why do we have
// to stack them up ... the reload button is placed in one corner, and report
// option is in one corner", and on the phone "the reports ... they're too
// big". These tests pin the laptop top bar and the denser phone card in
// low.css. The template is unchanged; the phone blocks give their children
// to the bar through display contents, so the phone page above the list is
// unchanged.

const lowPhone = "@media (max-width: 899.98px)"

// TestLowLaptopTopBar: from 900px one sticky bar on the page colour with a
// muted hairline under it, Back and Reload in its left corner behind a muted
// divider, the title first, Report an emergency pushed to the right corner at the
// phone button's size, and the short lines wrapped under the title.
func TestLowLaptopTopBar(t *testing.T) {
	const css = "static/css/low.css"
	p16MustDecl(t, "bar", p31mRule(t, css, p45Laptop, ".low-side:not([hidden])"), map[string]string{
		"position":      "sticky",
		"top":           "0",
		"z-index":       "10",
		"display":       "flex",
		"align-items":   "flex-start",
		"padding":       "var(--space-12) 0",
		"border-bottom": "1px solid var(--color-text-muted)",
		"background":    "var(--color-bg)",
	})
	p16MustDecl(t, "left corner", p31mRule(t, css, p45Laptop, ".low-topbar:not([hidden])"), map[string]string{
		"flex":         "none",
		"padding":      "0 var(--space-md) 0 0",
		"border-right": "1px solid var(--color-text-muted)",
	})
	p16MustDecl(t, "bar body", p31mRule(t, css, p45Laptop, ".low-page__head:not([hidden])"), map[string]string{
		"display":   "flex",
		"flex":      "1 1 auto",
		"flex-wrap": "wrap",
		"min-width": "0",
	})
	p16MustDecl(t, "blocks open", p31mRule(t, css, p45Laptop, ".low-block:not([hidden])"), map[string]string{
		"display": "contents",
	})
	p16MustDecl(t, "status open", p31mRule(t, css, p45Laptop, ".low-status:not([hidden])"), map[string]string{
		"display": "contents",
	})
	p16MustDecl(t, "title first", p31mRule(t, css, p45Laptop, ".low-page__head h1"), map[string]string{
		"order":         "1",
		"padding-right": "0",
	})
	p16MustDecl(t, "right corner", p31mRule(t, css, p45Laptop, ".low-page__actions:not([hidden])"), map[string]string{
		"order":  "2",
		"margin": "0 0 0 auto",
	})
	p16MustDecl(t, "line break", p31mRule(t, css, p45Laptop, ".low-page__head::before"), map[string]string{
		"order":      "3",
		"flex-basis": "100%",
	})
	p16MustDecl(t, "left corner centred", p31mRule(t, css, p45Laptop, ".low-topbar:not([hidden])"), map[string]string{
		"margin-top": "var(--space-xs)",
	})
	// The phone's button size (owner, 2026-10-22: "I like the phone version"):
	// the base rule's 52px height stays, about the phone button's width.
	p16MustDecl(t, "button size", p31mRule(t, css, p45Laptop, ".low-page__actions .low-page__report"), map[string]string{
		"flex":      "none",
		"min-width": "320px",
	})
	p16MustDecl(t, "focus clears the bar", p31mRule(t, css, p45Laptop, "html"), map[string]string{
		"scroll-padding-top": "8rem",
	})
}

// TestLowPhoneCard: under 900px each card is a grid whose first line holds
// the icon, the severity label and, in the right corner, Flag without a box
// or hairline; every other part spans the whole card.
func TestLowPhoneCard(t *testing.T) {
	const css = "static/css/low.css"
	p16MustDecl(t, "card", p31mRule(t, css, lowPhone, ".report-row.report-row--low:not([hidden])"), map[string]string{
		"display":               "grid",
		"grid-template-columns": "auto auto minmax(0, 1fr) auto",
		"gap":                   "0 var(--space-sm)",
		"padding":               "var(--space-12) var(--space-12) var(--space-12) var(--space-md)",
	})
	p16MustDecl(t, "body open", p31mRule(t, css, lowPhone, ".report-row--low > .report-row__body:not([hidden])"), map[string]string{
		"display": "contents",
	})
	p16MustDecl(t, "icon", p31mRule(t, css, lowPhone, ".report-row--low > .icon-badge"), map[string]string{
		"grid-row":    "1",
		"grid-column": "1",
	})
	p16MustDecl(t, "label", p31mRule(t, css, lowPhone, ".report-row--low .report-row__body > .severity-label"), map[string]string{
		"grid-row":    "1",
		"grid-column": "2",
	})
	p16MustDecl(t, "full width parts", p31mRule(t, css, lowPhone, ".report-row--low .report-row__body > *"), map[string]string{
		"grid-column": "1 / -1",
	})
	p16MustDecl(t, "flag corner", p31mRule(t, css, lowPhone, ".report-row--low .report-row__body > .flag-control:not([hidden])"), map[string]string{
		"grid-row":    "1",
		"grid-column": "4",
		"min-height":  "var(--touch-target-min)",
		"border-top":  "0",
	})
}

// TestLowReloadIcon: Reload draws the app's anticlockwise stroke glyph
// through a mask (owner, 2026-10-22: the text arrow "looks odd ... it should
// be the other way around"), not a text character.
func TestLowReloadIcon(t *testing.T) {
	p16MustDecl(t, "reload glyph", p31mRule(t, "static/css/low.css", "", ".low-topbar__icon--reload"), map[string]string{
		"width":            "20px",
		"height":           "20px",
		"background-color": "currentColor",
		"mask":             `url("/static/icons/rotate-ccw.svg") center / contain no-repeat`,
	})
	html := p418Template(t)
	at := strings.Index(html, `id="low-reload"`)
	end := strings.Index(html[max(at, 0):], `</button>`)
	if at < 0 || end < 0 || !strings.Contains(html[at:at+end], `<span class="low-topbar__icon low-topbar__icon--reload" aria-hidden="true"></span>`) {
		t.Errorf("the Reload button must hold the empty masked glyph span")
	}
	if strings.Contains(html, "&#8635;") {
		t.Errorf("the text arrow &#8635; must be gone from low.html.tmpl")
	}
}
