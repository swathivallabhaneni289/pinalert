package web

import (
	"strings"
	"testing"
)

// This file holds the static contract tests of plan 03.1-12, the Past tab's
// cards and groups, the empty Active tab's recent counts and the tab choice
// for a report link. Like the other contract files nothing here needs a
// database or a server: the tests read the embedded script and stylesheet and
// inspect the shipped text. Every helper and test name carries the pt12 or
// Past prefix so no other plan's file can clash with it.

// pt12Feed returns feed.js with its whole line comments removed.
func pt12Feed(t *testing.T) string {
	t.Helper()
	return p19Code(t, "static/js/feed.js")
}

// TestPastGroupsOrderAndCap pins D-08: the groups are split by the server's
// history state and keep the server's order (no sort), a group with no card is
// hidden with its header, and the cap line says how many cards are shown only
// when the server cut that group.
func TestPastGroupsOrderAndCap(t *testing.T) {
	feed := pt12Feed(t)

	groups := jsFunctionBody(t, feed, "renderPastGroups(state)")
	p13MustContain(t, "renderPastGroups", groups,
		".history.state", "'resolved'", "'expired'", "cut.resolved === true", "cut.expired === true",
		"pastResolvedListEl", "pastExpiredListEl", "removeDepartedPastCards(seen);", "return resolved.length + expired.length;")
	p13Ordered(t, groups, "renderPastGroups",
		"pastResolvedListEl", "pastExpiredListEl")
	p13MustNotContain(t, "renderPastGroups", groups, "sortReports(", ".sort(", "innerHTML", "slice(")

	place := jsFunctionBody(t, feed, "placePastGroup(groupEl, listEl, cutEl, reports, wasCut)")
	p13MustContain(t, "placePastGroup", place,
		"setHidden(groupEl, reports.length === 0);", "setHidden(cutEl, !(wasCut && reports.length > 0));",
		"Pinalert.setText(cutEl, cutLineText(reports.length));")

	cut := jsFunctionBody(t, feed, "cutLineText(count)")
	p13MustContain(t, "cutLineText", cut, "'Showing the newest ' + count + '.'")
	if n := strings.Count(feed, "Showing the newest "); n != 1 {
		t.Errorf("feed.js holds the cap line words %d times, want 1 (one helper for both groups)", n)
	}

	tab := jsFunctionBody(t, feed, "renderPastTab(state)")
	p13Ordered(t, tab, "renderPastTab",
		"setHidden(reportListEl, true);", "clearRows();", "state.reports.length === 0",
		"setHidden(pastGroupsEl, false);", "renderPastGroups(state)")

	// The Past cards are cleared when the live list is drawn, so a card of
	// another tab never lingers in a hidden list.
	render := jsFunctionBody(t, feed, "render(state)")
	p13MustContain(t, "render", render, "clearPastCards();")
}

// TestPastCardMarkup pins D-11: a card is one tab stop, holds no button and no
// link, opens the report through activateRow on a click, Enter or Space, and
// ends with the visually hidden Open report cue.
func TestPastCardMarkup(t *testing.T) {
	feed := pt12Feed(t)
	body := jsFunctionBody(t, feed, "createPastCard(id)")

	p13MustContain(t, "createPastCard", body,
		"createElement('li')", "'past-card'", "setAttribute('tabindex', '0')", "dataset.reportId = String(id)",
		"'past-card__title'", "'past-card__meta'", "'past-card__facts'", "'past-card__open-cue visually-hidden'",
		"Pinalert.setText(cue, 'Open report');",
		"addEventListener('click'", "addEventListener('keydown'",
		"evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar'", "activateRow(id);")
	p13MustNotContain(t, "createPastCard", body,
		"createElement('button')", "createElement('a')", "createElement('input')", "innerHTML", "href")
	p13Ordered(t, body, "createPastCard",
		"li.appendChild(title);", "li.appendChild(meta);", "li.appendChild(facts);", "li.appendChild(cue);")

	update := jsFunctionBody(t, feed, "updatePastCard(card, report)")
	p13MustNotContain(t, "updatePastCard", update, "appendChild(card.cue", "card.el.appendChild(")
	p13MustContain(t, "updatePastCard", update, "past-card--selected")
}

// TestPastCardWords pins D-08 to D-10: the meta and facts lines are the
// server's words joined by the app's middle dot, every string goes through
// setText, and the browser computes no count and no outcome.
func TestPastCardWords(t *testing.T) {
	feed := pt12Feed(t)

	meta := jsFunctionBody(t, feed, "buildPastMeta(report)")
	p13MustContain(t, "buildPastMeta", meta,
		"history.closed_text", "Pinalert.relativeTime(history.at)", "'Faded '", "history.lifetime_text",
		"history.state === 'resolved'", "SEPARATOR")
	p13MustNotContain(t, "buildPastMeta", meta, "innerHTML", "Math.", "parseInt(")

	// The facts line is the one trust line builder: the standing pin of
	// TestTrustBlockMountedOnBothSurfaces keeps every trust field name out of
	// feed.js, so the numerals and words come from PinalertTrust.
	create := jsFunctionBody(t, feed, "createPastCard(id)")
	p13MustContain(t, "createPastCard", create,
		"PinalertTrust.createTrustLine();", "facts.classList.add('past-card__facts');",
		"'past-card__outcome'", "querySelectorAll('.report-row__trustline-count')")

	update := jsFunctionBody(t, feed, "updatePastCard(card, report)")
	p13MustContain(t, "updatePastCard", update,
		"Pinalert.setText(card.title,", "Pinalert.setText(card.meta, buildPastMeta(report));",
		"PinalertTrust.updateTrustLine(card.trust, report.trust);", "card.trust.word.hidden = true;",
		"history.outcome_label", "history.outcome === 'corroborated'",
		"Pinalert.setText(card.outcome,", "card.firstCount.hidden", "card.secondCount.hidden")
	p13MustNotContain(t, "updatePastCard", update,
		"innerHTML", "insertAdjacentHTML", "Math.", "parseInt(", "+ 1", "_text", "createTextNode(")
	if strings.Contains(feed, "confirmed_text") || strings.Contains(feed, "disputed_text") {
		t.Errorf("feed.js names a trust text field, a Past card must use PinalertTrust's trust line")
	}

	// The label is written first, and the corroborated outcome leaves the
	// confirm count to the label while no other outcome hides it.
	p13Ordered(t, update, "updatePastCard facts",
		"history.outcome_label", "card.firstCount.hidden", "history.outcome === 'corroborated'",
		"card.secondCount.hidden")
}

// TestPastCardApproximate pins D-23 and T-31-55: a finished Rescue card is
// titled Rescue needed, its meta begins Approximate location and the
// description is read only when the report is not approximate.
func TestPastCardApproximate(t *testing.T) {
	feed := pt12Feed(t)

	p13MustContain(t, "feed.js", feed,
		"var FINISHED_RESCUE_TITLE = 'Rescue needed';", "var APPROXIMATE_LABEL = 'Approximate location';")

	update := jsFunctionBody(t, feed, "updatePastCard(card, report)")
	p13MustContain(t, "updatePastCard", update,
		"var approximate = report.approximate === true;",
		"approximate ? FINISHED_RESCUE_TITLE : report.description")
	if n := strings.Count(update, "report.description"); n != 1 {
		t.Errorf("updatePastCard reads report.description %d times, want exactly the one guarded by approximate", n)
	}
	if strings.Contains(update, "description") && !strings.Contains(update, "approximate ? FINISHED_RESCUE_TITLE : report.description") {
		t.Errorf("updatePastCard reads the description outside the approximate guard")
	}

	meta := jsFunctionBody(t, feed, "buildPastMeta(report)")
	p13MustContain(t, "buildPastMeta", meta,
		"report.approximate === true", "APPROXIMATE_LABEL : Pinalert.categoryLabel(report.category)")
	p13MustNotContain(t, "buildPastMeta", meta, "description")
}

// TestPastCardReachesTheSharedHelpers pins that the 60 second refresh, the
// selection highlight and the pane reveal find a Past card by its id.
func TestPastCardReachesTheSharedHelpers(t *testing.T) {
	feed := pt12Feed(t)

	refresh := jsFunctionBody(t, feed, "refreshAgeAndTime()")
	p13MustContain(t, "refreshAgeAndTime", refresh, "pastCardsById", "buildPastMeta(")

	highlight := jsFunctionBody(t, feed, "applySelectionHighlight(id)")
	p13MustContain(t, "applySelectionHighlight", highlight, "pastCardsById", "past-card--selected")

	reveal := jsFunctionBody(t, feed, "revealRowInListPane(id)")
	p13MustContain(t, "revealRowInListPane", reveal, "pastCardsById[String(id)]")

	after := jsFunctionBody(t, feed, "focusAfterRemoval(candidates)")
	p13MustContain(t, "focusAfterRemoval", after, "classList.contains('past-card')")
}
