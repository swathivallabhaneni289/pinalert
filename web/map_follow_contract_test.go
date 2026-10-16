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
// names the set only while Show Disputed is chosen, it is written through the
// text sink, and it follows every store notification.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18): the Past tab left Nearby, so the label has one word left, for Show
// Disputed, and the past words are pinned absent. The Phase 3.1 and Phase 4
// hands-on tests are walked on this layout after Phase 3.4 is built.
func TestMapFollowsLabel(t *testing.T) {
	raw := p13ReadStatic(t, "static/js/map.js")
	code := p31fMapCode(t)

	label := jsFunctionBody(t, code, "updateMapLabel(state)")
	p13MustContain(t, "updateMapLabel", label,
		"document.getElementById('map-label')",
		"'Showing disputed reports'",
		"state.view === 'disputed'",
		"Pinalert.setText(",
		".hidden = ")
	p13MustNotContain(t, "updateMapLabel", label,
		"innerHTML", "insertAdjacentHTML", "fetchReports", "Pinalert.setView(",
		"'Showing past reports, last 7 days'", "state.view === 'past'")

	// The word exists once in the whole file, comments included, so a second
	// copy cannot drift from the first, and the retired past word is gone.
	if n := strings.Count(raw, "Showing disputed reports"); n != 1 {
		t.Errorf("map.js names %q %d times, want exactly 1", "Showing disputed reports", n)
	}
	if strings.Contains(raw, "Showing past reports") {
		t.Errorf("map.js still names the retired Past label")
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

// TestMapFollowsFinishedPopup pins Surface 6 (D-09, D-11): a finished report
// opens in the same popup card, with the closed line in the slot of the fade
// line and the fade sentence not shown. The nodes are built once and written
// in place on every poll, and the vote block stays mounted exactly as the
// vote tests pin it (plan 14 decides what its actions area shows).
func TestMapFollowsFinishedPopup(t *testing.T) {
	code := p31fMapCode(t)

	closed := jsFunctionBody(t, code, "closedLineText(report)")
	p13MustContain(t, "closedLineText", closed,
		"report.history", "history.state === 'resolved'", "history.state === 'expired'",
		"history.closed_text", "history.lifetime_text", "Pinalert.relativeTime(history.at)",
		"'Faded '", "'Resolved'")
	p13MustNotContain(t, "closedLineText", closed,
		"new Date", "Date.now", "getTime", "expires_at", "created_at", "innerHTML")

	// The card is built once: the fade node and the privacy node are made in
	// buildPopupCard, and updatePopupCard builds nothing.
	build := jsFunctionBody(t, code, "buildPopupCard(report)")
	p13MustContain(t, "buildPopupCard", build,
		"fade.className = 'popup-card__fade'", "fade: fade", "PinalertVotes.createVoteBlock(report.id)",
		"actions.appendChild(block.controls)", "actions.appendChild(block.note)", "actions.appendChild(block.error)")

	update := jsFunctionBody(t, code, "updatePopupCard(card, report)")
	p13MustContain(t, "updatePopupCard", update,
		"closedLineText(report)", "Pinalert.formatFade(report)", "Pinalert.setText(card.fade, ",
		"card.fade.hidden", "card.fade.classList.toggle('popup-card__fade--closed'",
		"PinalertVotes.updateVoteBlock(card.votes, report)", "PinalertTrust.updateTrustCard(card.trust, report.trust)")
	// The closed line wins: the fade sentence is asked for only when there is
	// no closed line.
	p13Ordered(t, update, "updatePopupCard", "closedLineText(report)", "Pinalert.formatFade(report)", "card.fade.hidden")
	p13MustNotContain(t, "updatePopupCard", update,
		"createElement(", "appendChild(", "innerHTML", "new Date", "Date.now", "getTime", "expires_at", ".focus(")

	// The closed line is the normal text colour on a laptop popup (the fade
	// sentence stays muted, which TestPopupBuildsTheCardAndTheFadeLine pins).
	closedRule := p31mRule(t, "static/css/trust.css", "",
		".leaflet-container .popup-card .popup-card__fade--closed:not([hidden])")
	p16MustDecl(t, "closed line", closedRule, map[string]string{"color": "var(--color-text)"})
}

// TestMapFollowsApproximate pins Surface 7 (D-23, T-31-58): a finished Rescue
// report opens titled Rescue needed with the label and the privacy line, its
// pin sits at the server's coarse point with no circle drawn, the map never
// goes closer than zoom 14 for it, and the pin name says it is approximate.
func TestMapFollowsApproximate(t *testing.T) {
	raw := p13ReadStatic(t, "static/js/map.js")
	code := p31fMapCode(t)

	// One ceiling, one copy of each word, all fixed strings.
	for _, want := range []string{
		"var MAX_APPROX_ZOOM = 14;",
		"Details of finished rescue requests are hidden to protect privacy.",
	} {
		if n := strings.Count(raw, want); n != 1 {
			t.Errorf("map.js names %q %d times, want exactly 1", want, n)
		}
	}
	p13MustContain(t, "map.js constants", code,
		"var FINISHED_RESCUE_TITLE = 'Rescue needed';", "var APPROX_LABEL = 'Approximate location';")

	build := jsFunctionBody(t, code, "buildPopupCard(report)")
	p13MustContain(t, "buildPopupCard", build,
		"privacyLine.className = 'popup-card__privacy'", "privacyLine.hidden = true",
		"metaLines.appendChild(privacyLine)", "privacy: privacyLine")
	p13Ordered(t, build, "buildPopupCard",
		"metaLines.appendChild(shelterLine)", "metaLines.appendChild(privacyLine)")

	update := jsFunctionBody(t, code, "updatePopupCard(card, report)")
	p13MustContain(t, "updatePopupCard", update,
		"report.approximate === true", "FINISHED_RESCUE_TITLE", "APPROX_LABEL", "PRIVACY_LINE",
		"Pinalert.setText(card.privacy, ", "card.privacy.hidden", "card.approximate = ",
		"Pinalert.formatShelterLine(report)", "Pinalert.distanceText(report)")

	// The zoom ceiling applies to every code driven zoom for an approximate
	// pin and leaves a live pin at 16.
	zoom := jsFunctionBody(t, code, "zoomFor(id)")
	p13MustContain(t, "zoomFor", zoom, "card.approximate === true", "MAX_APPROX_ZOOM", "16")
	fly := jsFunctionBody(t, code, "flyTo(id, opts)")
	p13MustContain(t, "flyTo", fly, "var zoom = zoomFor(id);", "map.setView(marker.getLatLng(), zoom, { animate: false });",
		"map.flyTo(marker.getLatLng(), zoom);")
	p13MustNotContain(t, "flyTo", fly, "), 16", ", 16)")
	strip := jsFunctionBody(t, code, "centerPinInStrip(popup, popupEl)")
	p13MustContain(t, "centerPinInStrip", strip,
		"approximate ? MAX_APPROX_ZOOM : Math.max(map.getZoom(), 16)", "map.setView(popup.getLatLng(), zoom, { animate: false });")
	p13MustNotContain(t, "centerPinInStrip", strip, "Math.min(")

	// No circle and no radius layer is drawn anywhere: a drawn radius would
	// promise a precision the coarsening does not give.
	p13MustNotContain(t, "map.js", code, "L.circle", "L.Circle", "circleMarker", "L.polygon", "L.rectangle")

	// The pin name ends with the approximate location words, after the state.
	name := jsFunctionBody(t, code, "pinName(report)")
	p13MustContain(t, "pinName", name, "report.approximate === true", "'approximate location'")
	p13Ordered(t, name, "pinName", "PinalertVisibility.stateWord(report)", "'approximate location'", "parts.join(', ')")

	// The privacy line is a popup owned paragraph with the ancestor field and
	// the normal text colour.
	privacy := p31mRule(t, "static/css/trust.css", "",
		".leaflet-container .popup-card .popup-card__privacy:not([hidden])")
	p16MustDecl(t, "privacy line", privacy, map[string]string{
		"display": "block", "font-size": "var(--font-size-label)",
		"line-height": "var(--line-height-label)", "color": "var(--color-text)",
	})
}

// TestMapFollowsFocusOnce pins D-11 and D-13: a report link keeps its pin on
// the map when the report is outside the reader's area or the 7 day window,
// only under the Show choice and the chip that fit it, and opens it once. The
// list is never changed from here and the store is only read.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18): the tab test is now a Show test. A linked report is drawn only when it
// is active or disputed (a finished report is no longer drawn on Nearby, its
// link lands on My activity) and when Pinalert.reportInShow, which reads the
// server's view and confirmed_by_people words, lets it into the Show choice.
// "Already in the list" now means in the drawn set (visibleReports). The Phase
// 3.1 and Phase 4 hands-on tests are walked on this layout after Phase 3.4 is
// built.
func TestMapFollowsFocusOnce(t *testing.T) {
	code := p31fMapCode(t)

	pin := jsFunctionBody(t, code, "focusPinReport(state)")
	p13MustContain(t, "focusPinReport", pin,
		"state.focus", "focus.status !== 'ok'", "focus.report", "Pinalert.visibleReports()", "drawn[",
		"belongsToShow(report, state.show)",
		"state.category !== ''", "report.category !== state.category", "return null;", "return report;")
	p13MustNotContain(t, "focusPinReport", pin, "new Date", "Date.now", "getTime", "expires_at", "created_at")

	show := jsFunctionBody(t, code, "belongsToShow(report, show)")
	p13MustContain(t, "belongsToShow", show, "report.view", "'active'", "'disputed'", "Pinalert.reportInShow(report, show)")
	p13MustNotContain(t, "belongsToShow", show, "new Date", "Date.now", "getTime", "'resolved'", "'past'")

	render := jsFunctionBody(t, code, "render(state)")
	p13Ordered(t, render, "render", "focusPinReport(state)", "upsertMarker(", "removeLayer(", "openFocusOnce(state)")

	once := jsFunctionBody(t, code, "openFocusOnce(state)")
	p13MustContain(t, "openFocusOnce", once,
		"focus.status !== 'ok'", "openedFocusId === id", "openedFocusId = id;", "markers[id]", "flyTo(id)",
		"Pinalert.select(id, 'map')")
	p13Ordered(t, once, "openFocusOnce", "openedFocusId === id", "markers[id]", "openedFocusId = id;", "flyTo(id)")
	p13MustNotContain(t, "openFocusOnce", once, "fetchReports", "setView(", "state.reports =", "state.focus =")

	// map.js reads the store and never writes the list or the link directly.
	// The one way it ends a link is Pinalert.dropFocus() when the linked
	// popup closes, whether the reader closed it or a redraw removed its pin;
	// only a close made by a resize across 900px keeps the link (03.1 review
	// fixes CR-01 and A2, pinned in TestReviewFixALinkedReportRefreshes and
	// TestReviewFixA2LinkEndsOnRedrawClose).
	p13MustNotContain(t, "map.js", code, "state.focus =", "state.reports =", "state.focusId", "setFocusId(")
	if n := strings.Count(code, "var openedFocusId = null;"); n != 1 {
		t.Errorf("map.js declares openedFocusId %d times, want 1", n)
	}
	if n := strings.Count(p13ReadStatic(t, "static/js/map.js"), "fetch("); n != 0 {
		t.Errorf("map.js holds %d fetch( occurrences, want 0", n)
	}
}

// TestMapFollowsPastCardOpener pins D-11 for the keyboard: closing a popup
// opened from a Past card returns focus to that card, the same way a row gets
// it, and a pin is found from either kind of opener by its report id.
func TestMapFollowsPastCardOpener(t *testing.T) {
	code := p31fMapCode(t)

	ret := jsFunctionBody(t, code, "returnFocusToOpener(opener)")
	p13MustContain(t, "returnFocusToOpener", ret,
		"document.querySelectorAll('.report-row, .past-card')", "dataset.reportId === String(opener.id)",
		"isConnected", "offsetParent")

	find := jsFunctionBody(t, code, "markerForElement(el)")
	p13MustContain(t, "markerForElement", find,
		".past-card[data-report-id]", ".report-row[data-report-id]", "dataset.reportId", "markers[")
	// A report id is looked up as a key and never put into a selector.
	p13MustNotContain(t, "map.js", code, "querySelector('[data-report-id=", `querySelector('.past-card[data-report-id="`)

	// The Past card is the list's own element, so this script never builds one.
	p13MustNotContain(t, "map.js", code, "createElement('li')", "className = 'past-card")
}
