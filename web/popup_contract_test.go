package web

import (
	"io/fs"
	"strings"
	"testing"
)

// Plan 03-17 pins the map popup card. Like the rest of this package these
// tests are static inspection of the embedded bytes: they prove the wiring,
// the order and the stylesheet declarations exist, and they cannot prove a
// browser rendered the card or kept a panel open across a poll. The browser
// half belongs to the V-42 owner script in plan 03-20.

// p17ReadStatic returns the embedded static file with block comments
// stripped. Line comments stay, so a comment in a script is scanned by every
// check below and must not name a banned literal.
func p17ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return stripCSSComments(string(raw))
}

// p17Ordered requires every needle to be present in body and the first
// indexes to strictly increase in the order given.
func p17Ordered(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	prev := -1
	prevNeedle := ""
	for _, needle := range needles {
		idx := strings.Index(body, needle)
		if idx == -1 {
			t.Errorf("%s: missing %q", label, needle)
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

// p17MustContain reports each needle missing from body.
func p17MustContain(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Errorf("%s: missing %q", label, needle)
		}
	}
}

// p17MustNotContain reports each needle present in body.
func p17MustNotContain(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			t.Errorf("%s: must not contain %q", label, needle)
		}
	}
}

// TestPopupCardBuiltOnceUpdatedInPlace pins A19, A21, A24, A30, A31, A32,
// D-09 and D-20 on the popup: the card is built once per marker in the A21
// order, every poll writes text, classes and attributes on the existing
// nodes, Leaflet's content setter is gone, and a removed report drops its
// observer.
func TestPopupCardBuiltOnceUpdatedInPlace(t *testing.T) {
	text := p17ReadStatic(t, "static/js/map.js")

	// The old rebuild path is gone. The two literals are split so this file
	// does not name them whole.
	setter := "set" + "PopupContent("
	oldBuilder := "build" + "PopupContent"
	p17MustNotContain(t, "map.js", text, oldBuilder, setter, "unbindPopup(")

	build := jsFunctionBody(t, text, "buildPopupCard(report)")
	p17MustContain(t, "buildPopupCard", build,
		"'popup-card'",
		"setAttribute('tabindex', '-1')",
		"'pinalert-popup-title-'",
		"++popupTitleSeq",
		"PinalertTrust.createSeverityLabel(",
		"updatePopupCard(card, report)",
	)
	p17Ordered(t, "buildPopupCard", build,
		"PinalertTrust.createSeverityLabel(",
		"PinalertVisibility.createVisibilityTag(",
		"Pinalert.setText(titleEl, report.description)",
		"PinalertTrust.createTrustBlock({ showScores: true })",
		"PinalertVotes.createVoteBlock(",
		"actions.appendChild(block.controls)",
		"actions.appendChild(block.note)",
		"actions.appendChild(block.error)",
	)
	if open := strings.Index(build, "{"); open == -1 {
		t.Errorf("buildPopupCard: no opening brace")
	} else if vote := strings.Index(build, "PinalertVotes.createVoteBlock("); vote > open {
		if strings.Contains(build[open:vote], "function") {
			t.Errorf("buildPopupCard declares a nested function before the vote block call (A21)")
		}
	}

	update := jsFunctionBody(t, text, "updatePopupCard(card, report)")
	p17MustContain(t, "updatePopupCard", update,
		"PinalertTrust.updateSeverityLabel(",
		"PinalertVisibility.updateVisibilityTag(",
		"PinalertTrust.updateTrustBlock(",
		"PinalertVotes.updateVoteBlock(",
		"Pinalert.formatShelterLine(report)",
		"Pinalert.relativeTime(",
		"Pinalert.iconClass(",
	)
	p17MustNotContain(t, "updatePopupCard", update,
		"createElement(", "appendChild(",
		"createVisibilityTag(", "createTrustBlock(", "createVoteBlock(", "createSeverityLabel(",
		"innerHTML", "SEVERITY_LABELS", "distance_km",
		"setBlockBusy(", "disabled", ".focus(", "update()",
	)

	bind, anchorFound, termFound := windowAfter(text, "bindPopup(", "marker.on(")
	if !anchorFound || !termFound {
		t.Fatalf("map.js: could not bound the bind call window (anchor=%v terminator=%v)", anchorFound, termFound)
	}
	if v, ok := readOptionValue(bind, "className"); !ok || v != "popupStateClasses(report)" {
		t.Errorf("bind call className must be popupStateClasses(report), found %q (present=%v)", v, ok)
	}
	p17MustContain(t, "bind call", bind, "card.el", "minWidth: 340", "maxWidth: 340", "keepInView: true")
	p17MustNotContain(t, "bind call", bind, "closeOnClick", "maxHeight", "autoPan")

	upsert := jsFunctionBody(t, text, "upsertMarker(report)")
	p17MustContain(t, "upsertMarker", upsert,
		"popupAnchor: [0, -22]",
		"updatePopupCard(cards[report.id], report)",
		"cards[report.id] = card",
		"observePopupCard(marker, card)",
	)

	render := jsFunctionBody(t, text, "render(state)")
	p17MustContain(t, "render", render, "disconnect()", "delete cards[")

	observe := jsFunctionBody(t, text, "observePopupCard(marker, card)")
	p17MustContain(t, "observePopupCard", observe, "ResizeObserver", "requestAnimationFrame(")

	relayout := jsFunctionBody(t, text, "relayoutOpenPopup(marker, card)")
	p17MustContain(t, "relayoutOpenPopup", relayout, "setLatLng(", "getLatLng()")
	// The observer repositions only. The popup update call would re-append
	// the content node and drop focus (A32).
	p17MustNotContain(t, "relayoutOpenPopup", relayout, "update()")

	votes := p17ReadStatic(t, "static/js/votes.js")
	p17MustNotContain(t, "votes.js", votes, "rebuilds its popup", "detached node")
}
