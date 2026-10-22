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
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18): the Past tab left Nearby. The card builders and the word helpers
// moved, behaviour unchanged, from feed.js to web/static/js/ended-cards.js
// (window.PinalertEnded), where My activity, Ended nearby draws them, so the
// card word assertions below read that file. What belonged to the tab itself
// (the two group lists, the Past render, the Past hide, the card refresh and
// highlight in feed.js, the Past card stylesheet) is gone with the tab, and
// its absence is pinned instead. A report link to a finished report now leaves
// Nearby for My activity. The Phase 3.1 and Phase 4 hands-on tests are walked
// on this layout after Phase 3.4 is built.

// pt12Feed returns feed.js with its whole line comments removed.
func pt12Feed(t *testing.T) string {
	t.Helper()
	return p19Code(t, "static/js/feed.js")
}

// pt12Ended returns ended-cards.js with its whole line comments removed.
func pt12Ended(t *testing.T) string {
	t.Helper()
	return p19Code(t, "static/js/ended-cards.js")
}

// TestPastTabLeftNearby pins D-18: nothing of the Past tab is left on the
// Nearby list. feed.js builds no Past card and draws no Past group, the store
// has no past word, and the page has neither the groups nor the empty block.
func TestPastTabLeftNearby(t *testing.T) {
	feed := pt12Feed(t)
	p13MustNotContain(t, "feed.js", feed,
		"createPastCard", "updatePastCard", "buildPastMeta", "renderPastTab", "renderPastGroups", "placePastGroup",
		"pastCardsById", "pastGroupsEl", "pastEmptyEl", "clearPastCards", "removeDepartedPastCards",
		"groupLineText", "cutLineText", "PARTIAL_LINE_TEXT", "FINISHED_RESCUE_TITLE", "APPROXIMATE_LABEL",
		"past-card", "seePastBtn", "selectTabOnScreen", "switchTab(")

	index := p13ReadTemplate(t, "index.html.tmpl")
	p13MustNotContain(t, "index.html.tmpl", index,
		`id="past-groups"`, `id="past-empty"`, `id="past-resolved"`, `id="past-expired"`, `id="see-past-reports"`,
		`data-feed-view="past"`, "/?view=past")
	p13MustContain(t, "index.html.tmpl", index, `href="/profile?part=ended"`, ">See ended reports</a>")

	css := p16ReadStatic(t, "static/css/feed.css")
	p13MustNotContain(t, "feed.css", css, ".past-card", ".past-group", "#past-groups", "#past-empty")
}

// TestPastCardMarkup pins the Ended nearby row (D-11 amended by Phase 3.4 plan
// 14, D-20, D-21): a finished report is a My activity row of six cells in the
// grid order, disc, Report, Status, What happened, When, Distance. It is read
// only: it holds no button and no link, takes no tab stop and has no click or key
// handler, because nothing opens from a row of that page.
//
// Amended 2026-10-17 by Phase 3.4 plan 14: the Past card (one tab stop, an Open
// report cue, an onOpen callback) became the row, so the assertions follow the
// row's DOM. The no button, no link and no markup sink assertions are kept.
func TestPastCardMarkup(t *testing.T) {
	ended := pt12Ended(t)
	body := jsFunctionBody(t, ended, "createEndedRow(id)")

	p13MustContain(t, "createEndedRow", body,
		"createElement('li')", "'report-row activity-card activity-card--ended'", "dataset.reportId = String(id)",
		"'icon-badge activity-card__icon'", "setAttribute('aria-hidden', 'true')",
		"'activity-card__report'", "'report-row__title'", "'report-row__meta activity-card__sub'",
		"'activity-card__status'", "'visually-hidden'", "'activity-chip'",
		"'activity-card__what'", "'activity-card__happened'", "'activity-card__facts'",
		"'activity-card__when'", "'activity-card__distance'")
	p13MustNotContain(t, "createEndedRow", body,
		"createElement('button')", "createElement('a')", "createElement('input')", "innerHTML", "href",
		"tabindex", "addEventListener(", "onOpen")
	p13Ordered(t, body, "createEndedRow",
		"li.appendChild(badge);", "li.appendChild(report);", "li.appendChild(status);",
		"li.appendChild(what);", "li.appendChild(when);", "li.appendChild(distance);")

	update := jsFunctionBody(t, ended, "updateEndedRow(card, report)")
	p13MustNotContain(t, "updateEndedRow", update, "card.el.appendChild(", "addEventListener(")
	p13MustContain(t, "updateEndedRow", update, "'vis-retracted'", "'vis-expired'", "Pinalert.severityClass(report)",
		"Pinalert.iconClass(report.category)", "STATUS_RESOLVED", "STATUS_EXPIRED")
}

// TestPastCardWords pins D-08 to D-10: the When and What happened cells are the
// server's words, every string goes through setText, and the browser computes
// no count and no outcome.
//
// Amended 2026-10-17 by Phase 3.4 plan 14: the one meta line (category, closed
// text and time) became the Report, What happened and When cells. Every word
// assertion is kept at its new place: the closed text and the outcome label are
// What happened, the relative time, Expired and the lifetime phrase are When.
func TestPastCardWords(t *testing.T) {
	ended := pt12Ended(t)

	when := jsFunctionBody(t, ended, "whenWords(report)")
	p13MustContain(t, "whenWords", when,
		// Amended 2026-10-22 by plan 04-30 (owner): "Expired {when}" in place
		// of "Faded {when}" (03.1-UAT gap).
		"Pinalert.relativeTime(history.at)", "'Expired '", "history.lifetime_text",
		"history.state === 'resolved'", "'Resolved '")
	p13MustNotContain(t, "whenWords", when, "innerHTML", "Math.", "parseInt(")

	happened := jsFunctionBody(t, ended, "happenedWords(report)")
	p13MustContain(t, "happenedWords", happened,
		"history.closed_text", "history.outcome_label", "history.state === 'resolved'")
	p13MustNotContain(t, "happenedWords", happened, "innerHTML", "Math.", "parseInt(", "+ 1")

	// The facts line is the one trust line builder: the standing pin of
	// TestTrustBlockMountedOnBothSurfaces keeps every trust field name out of
	// the scripts, so the numerals and words come from PinalertTrust.
	create := jsFunctionBody(t, ended, "createEndedRow(id)")
	p13MustContain(t, "createEndedRow", create,
		"PinalertTrust.createTrustLine();", "facts.classList.add('activity-card__facts');",
		"querySelectorAll('.report-row__trustline-count')")

	update := jsFunctionBody(t, ended, "updateEndedRow(card, report)")
	p13MustContain(t, "updateEndedRow", update,
		"Pinalert.setText(card.title,", "Pinalert.setText(card.happened, sentence);",
		"Pinalert.setText(card.when, whenWords(report));",
		"PinalertTrust.updateTrustLine(card.trust, report.trust);", "card.trust.word.hidden = true;",
		"history.outcome === 'corroborated'", "card.firstCount.hidden", "card.secondCount.hidden")
	p13MustNotContain(t, "updateEndedRow", update,
		"innerHTML", "insertAdjacentHTML", "Math.", "parseInt(", "+ 1", "_text", "createTextNode(")
	if strings.Contains(ended, "confirmed_text") || strings.Contains(ended, "disputed_text") {
		t.Errorf("ended-cards.js names a trust text field, a row must use PinalertTrust's trust line")
	}

	// The corroborated outcome leaves the confirm count to the label while no
	// other outcome hides it.
	p13Ordered(t, update, "updateEndedRow facts",
		"history.outcome === 'corroborated'", "card.firstCount.hidden", "card.secondCount.hidden")

	// The module is exported under one name for My activity.
	p13MustContain(t, "ended-cards.js", ended, "window.PinalertEnded = (function () {",
		"createEndedRow: createEndedRow,", "updateEndedRow: updateEndedRow,", "whenWords: whenWords,",
		"happenedWords: happenedWords,", "cutLineText: cutLineText,", "groupLineText: groupLineText",
		"kmWords: kmWords,", "distanceWords: distanceWords,")
	p13MustNotContain(t, "ended-cards.js", ended, "fetch(", "scrollIntoView", "innerHTML", "insertAdjacentHTML",
		"createPastCard", "updatePastCard", "buildPastMeta", "past-card")
}

// TestPastCardApproximate pins D-23 and T-31-55: a finished Rescue row is
// titled Rescue needed, its Report cell line reads Approximate location, its
// distance reads "about {n} km" and the description is read only when the
// report is not approximate.
func TestPastCardApproximate(t *testing.T) {
	ended := pt12Ended(t)

	p13MustContain(t, "ended-cards.js", ended,
		"var FINISHED_RESCUE_TITLE = 'Rescue needed';", "var APPROXIMATE_LABEL = 'Approximate location';")

	update := jsFunctionBody(t, ended, "updateEndedRow(card, report)")
	p13MustContain(t, "updateEndedRow", update,
		"var approximate = report.approximate === true;",
		"approximate ? FINISHED_RESCUE_TITLE : report.description",
		"approximate ? APPROXIMATE_LABEL : Pinalert.categoryLabel(report.category)")
	if n := strings.Count(update, "report.description"); n != 1 {
		t.Errorf("updateEndedRow reads report.description %d times, want exactly the one guarded by approximate", n)
	}
	if strings.Contains(update, "description") && !strings.Contains(update, "approximate ? FINISHED_RESCUE_TITLE : report.description") {
		t.Errorf("updateEndedRow reads the description outside the approximate guard")
	}

	distance := jsFunctionBody(t, ended, "distanceWords(report)")
	p13MustContain(t, "distanceWords", distance,
		"report.approximate === true", "'about ' + Math.max(1, Math.round(km)) + ' km'")
}

// TestRecentSentence pins C39's counts sentence {A}: only the parts whose
// count is above zero, the words of the four UI-SPEC examples, the numbers
// read from the server's recent object and never invented, and the empty All
// view choosing the history block only for an unfiltered Show All.
//
// Amended 2026-10-16 by Phase 3.4 (see the top of this file): the history
// block is chosen for Show All (the store's read word is all now, not active),
// after the filter, Disputed and Confirmed blocks, its body ends with the
// Phase 3.4 sentence, and its button is the See ended reports link to My
// activity.
func TestRecentSentence(t *testing.T) {
	feed := pt12Feed(t)

	sentence := jsFunctionBody(t, feed, "recentSentence(recent)")
	p13MustContain(t, "recentSentence", sentence,
		"recent.resolved_24h", "recent.expired_24h", "Number.isInteger(", "> 0",
		"' was resolved'", "' were resolved'", "' faded out'", "' and '", "' in the last 24 hours.'")
	p13MustNotContain(t, "recentSentence", sentence, "Math.", "parseInt(", "innerHTML")

	render := jsFunctionBody(t, feed, "render(state)")
	p13Ordered(t, render, "render history kind",
		"kind = 'filter';", "kind = 'disputed';", "kind = 'confirmed';",
		"recentSentence(Pinalert.state.recent)", "kind = 'history';",
		"Pinalert.setText(historyEmptyBodyEl,", "setEmptyState(kind);")
	p13MustContain(t, "render", render, "' ' + REPORT_PROMPT")
	p13MustContain(t, "feed.js", feed, `var REPORT_PROMPT = "Post a report to share what you're seeing.";`)
}

// TestFocusLeavesNearbyForAFinishedReport pins D-04, D-11 and D-18: a report
// link is acted on once per link, the two plain toasts say what the server
// said, and a report that has finished (resolved or faded out) replaces the
// address with the constant My activity address plus the id, only when the id
// is plain digits. An active or disputed report changes nothing. Nothing in
// that path adds a history entry or touches the store.
//
// Test change on purpose (03.1 review fix CR-01): the guard was pinned as
// "focus === handledFocus" (once per answer). Every answer carries a new focus
// object while the link is held, so the guard is "handledFocus !== null" (once
// per link), which keeps the toast and the redirect from repeating.
func TestFocusLeavesNearbyForAFinishedReport(t *testing.T) {
	feed := pt12Feed(t)

	handle := jsFunctionBody(t, feed, "handleFocus(state)")
	p13Ordered(t, handle, "handleFocus",
		"handledFocus !== null", "handledFocus = focus;", "focus.status === 'too_old'", "Pinalert.showToast(",
		"focus.status === 'unavailable'", "focus.status === 'ok'",
		"focus.report.view === 'resolved' || focus.report.view === 'past'",
		"REPORT_ID_PATTERN.test(finishedId)", "window.location.replace(")
	p13MustNotContain(t, "handleFocus", handle,
		"replaceState", "pushState", "location.assign", "location.href", "searchParams", "switchTab(", "fetch(",
		"setView(", "setShow(")
	p13MustContain(t, "feed.js", feed,
		"var ENDED_NEARBY_PATH = '/profile?part=ended';", "var REPORT_ID_PATTERN = /^[1-9][0-9]{0,17}$/;")
	if n := strings.Count(feed, "window.location.replace("); n != 2 {
		t.Errorf("feed.js calls window.location.replace %d times, want 2 (with and without the id)", n)
	}

	for _, words := range []string{
		`"That report is older than 7 days and is no longer shown."`,
		`"That report isn't available."`,
	} {
		if n := strings.Count(feed, words); n != 1 {
			t.Errorf("feed.js holds %s %d times, want exactly 1", words, n)
		}
	}

	// The link is acted on after every list draw of the ok path: the live
	// list and the empty list, and never on loading or error.
	render := jsFunctionBody(t, feed, "render(state)")
	if n := strings.Count(render, "handleFocus(state);"); n != 2 {
		t.Errorf("render calls handleFocus %d times, want 2 (live list, empty list)", n)
	}
	errorBranch := render[strings.Index(render, "if (state.status === 'error') {"):strings.Index(render, "setHidden(errorEl, true);\n    updateFeedStatus(state);")]
	if strings.Contains(errorBranch, "handleFocus(") {
		t.Errorf("the error branch must not act on a link")
	}
}
