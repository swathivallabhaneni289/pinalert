package web

// Plan 03.1-13 contract tests: the map follows the tab (HIST-01, HIST-02,
// HIST-03, D-09, D-11, D-13, D-23). Like the other contract files these are
// static inspection of the embedded bytes. Nothing here needs a database or a
// server, and none can prove how a browser lays a pin out (the 320, 360, 390
// and 1280 px pass is the orchestrator's browser check). Helpers carry the
// p31f prefix.

import (
	"strings"
	"testing"
)

const (
	p31fPhoneMedia  = "@media (max-width: 899.98px)"
	p31fForcedMedia = "@media (forced-colors: active)"
)

// p31fMapCode returns map.js with block comments and whole line comments
// removed, so a count, an order or a body sees code lines only.
func p31fMapCode(t *testing.T) string {
	t.Helper()
	return p19Code(t, "static/js/map.js")
}

// TestMapFollowsLabel pins the one line label on the map (C41, D-13): it
// names the tab only while Disputed or Past is selected, it is written
// through the text sink, and it follows every store notification.
func TestMapFollowsLabel(t *testing.T) {
	raw := p13ReadStatic(t, "static/js/map.js")
	code := p31fMapCode(t)

	label := jsFunctionBody(t, code, "updateMapLabel(state)")
	p13MustContain(t, "updateMapLabel", label,
		"document.getElementById('map-label')",
		"'Showing disputed reports'",
		"'Showing past reports, last 7 days'",
		"state.view === 'disputed'",
		"state.view === 'past'",
		"Pinalert.setText(",
		".hidden = ")
	p13MustNotContain(t, "updateMapLabel", label,
		"innerHTML", "insertAdjacentHTML", "fetchReports", "Pinalert.setView(")

	// Each word exists once in the whole file, comments included, so a second
	// copy cannot drift from the first.
	for _, want := range []string{"Showing disputed reports", "Showing past reports, last 7 days"} {
		if n := strings.Count(raw, want); n != 1 {
			t.Errorf("map.js names %q %d times, want exactly 1", want, n)
		}
	}

	// render runs on every store notification and calls the label first, so a
	// tab change updates the label with the same notification as the pins.
	render := jsFunctionBody(t, code, "render(state)")
	p13MustContain(t, "render", render, "updateMapLabel(state)")
	p13Ordered(t, code, "map.js subscription", "Pinalert.subscribe(render)", "function render(state)")
}

// TestMapFollowsFinishedPins pins D-13: a finished pin is a ring and never a
// filled severity dot, so a finished report is never read as live and the cue
// is not colour alone. The state comes from the server's own fields only.
func TestMapFollowsFinishedPins(t *testing.T) {
	// displayState reads the server's history state after Resolved, never a
	// date (T-31-61).
	vis := p15CodeLines(p15ReadStatic(t, "static/js/visibility.js"))
	display := jsFunctionBody(t, vis, "displayState(report)")
	p13MustContain(t, "displayState", display, "report.history.state", "'expired'", "'retracted'", "report.expired === true")
	p13Ordered(t, display, "displayState", "'retracted'", "report.expired === true", "=== 'expired'")
	p13MustNotContain(t, "displayState", display, "new Date", "Date.now", "getTime", "created_at", "expires_at")

	// A pin takes its state class through the one visibility helper, and the
	// 44 by 44 marker box does not change.
	code := p31fMapCode(t)
	badge := jsFunctionBody(t, code, "buildBadgeElement(report)")
	p13MustContain(t, "buildBadgeElement", badge,
		"PinalertVisibility.applyVisibilityClass(badge, report)", "finishedPinClass(report)", "badge.classList.add(ring)")
	p13Ordered(t, badge, "buildBadgeElement", "applyVisibilityClass(badge, report)", "finishedPinClass(report)")
	ring := jsFunctionBody(t, code, "finishedPinClass(report)")
	p13MustContain(t, "finishedPinClass", ring,
		"PinalertVisibility.displayState(report)", "state === 'retracted'", "'icon-badge--resolved'",
		"state === 'expired'", "'icon-badge--expired'")
	upsert := jsFunctionBody(t, code, "upsertMarker(report)")
	p13MustContain(t, "upsertMarker", upsert, "iconSize: [44, 44]")
	// The popup container carries the same display state word as the pin.
	p13MustContain(t, "popupStateClasses", jsFunctionBody(t, code, "popupStateClasses(report)"),
		"PinalertVisibility.displayState(report)")

	// The laptop badge: --color-bg fill, 2px solid ring in --color-text-muted,
	// muted glyph, scoped to #map so no Activity page badge changes. The
	// heads name the two ring modifiers and not the vis- classes, because the
	// pinned row treatment tests read every trust.css rule that names those.
	laptop := p31mRule(t, "static/css/trust.css", "", "#map .icon-badge--pin.icon-badge--resolved")
	p16MustDecl(t, "laptop resolved pin", laptop, map[string]string{
		"background": "var(--color-bg)",
		"border":     "2px solid var(--color-text-muted)",
		"color":      "var(--color-text-muted)",
	})
	expired := p31mRule(t, "static/css/trust.css", "", "#map .icon-badge--pin.icon-badge--expired")
	p16MustDecl(t, "laptop expired pin", expired, map[string]string{
		"background":   "var(--color-bg)",
		"border-style": "dashed",
	})
	for _, r := range p16ParseCSS("static/css/trust.css", p16ReadStatic(t, "static/css/trust.css")) {
		for _, part := range p16HeadParts(r.Head) {
			if strings.Contains(part, "icon-badge--pin") && !strings.HasPrefix(part, "#map ") {
				t.Errorf("trust.css pin rule %q is not scoped to #map", part)
			}
		}
	}

	// The expired popup takes the page colour as its tint, as hidden and
	// retracted do, so the phone sheet never reveals the map behind the text.
	popup := p31mRule(t, "static/css/trust.css", "", ".leaflet-container .leaflet-popup.vis-expired")
	p16MustDecl(t, "expired popup", popup, map[string]string{"--severity-tint": "var(--color-bg)"})

	// The phone dot: the same look with a 3px ring, inside the narrow block.
	phone := p31mRule(t, "static/css/phone.css", p31fPhoneMedia, "#map .icon-badge--pin.icon-badge--resolved")
	p16MustDecl(t, "phone resolved pin", phone, map[string]string{
		"background": "var(--color-bg)",
		"border":     "3px solid var(--color-text-muted)",
	})
	phoneExpired := p31mRule(t, "static/css/phone.css", p31fPhoneMedia, "#map .icon-badge--pin.icon-badge--expired")
	p16MustDecl(t, "phone expired pin", phoneExpired, map[string]string{"border-style": "dashed"})

	// Forced colours: the ring is the system text colour, with a head heavier
	// than the phone rule so it still wins at phone width.
	for _, head := range []string{
		"#map .leaflet-marker-icon .icon-badge--pin.icon-badge--resolved",
		"#map .leaflet-marker-icon .icon-badge--pin.icon-badge--expired",
	} {
		forced := p31mRule(t, "static/css/trust.css", p31fForcedMedia, head)
		p16MustDecl(t, "forced colours "+head, forced, map[string]string{"border-color": "CanvasText"})
	}

	// No raw colour in the added rules.
	for _, file := range []string{"static/css/trust.css", "static/css/phone.css"} {
		for _, r := range p16ParseCSS(file, p16ReadStatic(t, file)) {
			if !strings.Contains(r.Head, "icon-badge--resolved") && !strings.Contains(r.Head, "icon-badge--expired") {
				continue
			}
			for prop, value := range r.Decls {
				if strings.Contains(value, "#") || strings.Contains(value, "rgb(") {
					t.Errorf("%s %q %s: %q is a colour literal, tokens only", file, r.Head, prop, value)
				}
			}
		}
	}
}

// TestMapFollowsRingContrast computes the ring against the page colour from
// the shipped tokens in all four theme sources. A ring is a graphic pair, so
// the floor is 3 to 1 (WCAG 1.4.11).
func TestMapFollowsRingContrast(t *testing.T) {
	needed := []string{"--color-bg", "--color-text-muted"}
	light1, light2, dark1, dark2 := extractThemeTokens(parseCSSRules(p13ReadStatic(t, "static/css/main.css")), needed)
	sources := []struct {
		name   string
		tokens themeTokens
	}{
		{":root", light1}, {`:root[data-theme="light"]`, light2},
		{"prefers dark :root", dark1}, {`:root[data-theme="dark"]`, dark2},
	}
	for _, s := range sources {
		fg := p31Resolve(t, s.tokens, "--color-text-muted")
		bg := p31Resolve(t, s.tokens, "--color-bg")
		if ratio := wcagContrastRatio(t, fg, bg, s.name+" finished pin ring"); ratio < 3 {
			t.Errorf("%s: the finished pin ring is %.3f:1, want at least 3:1", s.name, ratio)
		}
	}
}
