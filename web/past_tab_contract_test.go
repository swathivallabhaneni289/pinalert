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

// TestPastCardMarkup pins D-11: a card is one tab stop, holds no button and no
// link, opens the report through the caller's onOpen on a click, Enter or
// Space, and ends with the visually hidden Open report cue.
func TestPastCardMarkup(t *testing.T) {
	ended := pt12Ended(t)
	body := jsFunctionBody(t, ended, "createPastCard(id, onOpen)")

	p13MustContain(t, "createPastCard", body,
		"createElement('li')", "'past-card'", "setAttribute('tabindex', '0')", "dataset.reportId = String(id)",
		"'past-card__title'", "'past-card__meta'", "'past-card__facts'", "'past-card__open-cue visually-hidden'",
		"Pinalert.setText(cue, 'Open report');",
		"addEventListener('click'", "addEventListener('keydown'",
		"evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar'", "onOpen(id);")
	p13MustNotContain(t, "createPastCard", body,
		"createElement('button')", "createElement('a')", "createElement('input')", "innerHTML", "href")
	p13Ordered(t, body, "createPastCard",
		"li.appendChild(title);", "li.appendChild(meta);", "li.appendChild(facts);", "li.appendChild(cue);")

	update := jsFunctionBody(t, ended, "updatePastCard(card, report)")
	p13MustNotContain(t, "updatePastCard", update, "appendChild(card.cue", "card.el.appendChild(")
	p13MustContain(t, "updatePastCard", update, "past-card--selected")
}

// TestPastCardWords pins D-08 to D-10: the meta and facts lines are the
// server's words joined by the app's middle dot, every string goes through
// setText, and the browser computes no count and no outcome.
func TestPastCardWords(t *testing.T) {
	ended := pt12Ended(t)

	meta := jsFunctionBody(t, ended, "buildPastMeta(report)")
	p13MustContain(t, "buildPastMeta", meta,
		"history.closed_text", "Pinalert.relativeTime(history.at)", "'Faded '", "history.lifetime_text",
		"history.state === 'resolved'", "SEPARATOR")
	p13MustNotContain(t, "buildPastMeta", meta, "innerHTML", "Math.", "parseInt(")

	// The facts line is the one trust line builder: the standing pin of
	// TestTrustBlockMountedOnBothSurfaces keeps every trust field name out of
	// the scripts, so the numerals and words come from PinalertTrust.
	create := jsFunctionBody(t, ended, "createPastCard(id, onOpen)")
	p13MustContain(t, "createPastCard", create,
		"PinalertTrust.createTrustLine();", "facts.classList.add('past-card__facts');",
		"'past-card__outcome'", "querySelectorAll('.report-row__trustline-count')",
		"classList.add('past-card__count')", "classList.add('past-card__count', 'past-card__count--second')")

	update := jsFunctionBody(t, ended, "updatePastCard(card, report)")
	p13MustContain(t, "updatePastCard", update,
		"Pinalert.setText(card.title,", "Pinalert.setText(card.meta, buildPastMeta(report));",
		"PinalertTrust.updateTrustLine(card.trust, report.trust);", "card.trust.word.hidden = true;",
		"history.outcome_label", "history.outcome === 'corroborated'",
		"Pinalert.setText(card.outcome,", "card.firstCount.hidden", "card.secondCount.hidden")
	p13MustNotContain(t, "updatePastCard", update,
		"innerHTML", "insertAdjacentHTML", "Math.", "parseInt(", "+ 1", "_text", "createTextNode(")
	if strings.Contains(ended, "confirmed_text") || strings.Contains(ended, "disputed_text") {
		t.Errorf("ended-cards.js names a trust text field, a Past card must use PinalertTrust's trust line")
	}

	// The label is written first, and the corroborated outcome leaves the
	// confirm count to the label while no other outcome hides it.
	p13Ordered(t, update, "updatePastCard facts",
		"history.outcome_label", "card.firstCount.hidden", "history.outcome === 'corroborated'",
		"card.secondCount.hidden")

	// The module is exported under one name for My activity.
	p13MustContain(t, "ended-cards.js", ended, "window.PinalertEnded = (function () {",
		"createPastCard: createPastCard,", "updatePastCard: updatePastCard,", "buildPastMeta: buildPastMeta,",
		"groupLineText: groupLineText,", "cutLineText: cutLineText,")
	p13MustNotContain(t, "ended-cards.js", ended, "fetch(", "scrollIntoView", "innerHTML", "insertAdjacentHTML")
}

// TestPastCardApproximate pins D-23 and T-31-55: a finished Rescue card is
// titled Rescue needed, its meta begins Approximate location and the
// description is read only when the report is not approximate.
func TestPastCardApproximate(t *testing.T) {
	ended := pt12Ended(t)

	p13MustContain(t, "ended-cards.js", ended,
		"var FINISHED_RESCUE_TITLE = 'Rescue needed';", "var APPROXIMATE_LABEL = 'Approximate location';")

	update := jsFunctionBody(t, ended, "updatePastCard(card, report)")
	p13MustContain(t, "updatePastCard", update,
		"var approximate = report.approximate === true;",
		"approximate ? FINISHED_RESCUE_TITLE : report.description")
	if n := strings.Count(update, "report.description"); n != 1 {
		t.Errorf("updatePastCard reads report.description %d times, want exactly the one guarded by approximate", n)
	}
	if strings.Contains(update, "description") && !strings.Contains(update, "approximate ? FINISHED_RESCUE_TITLE : report.description") {
		t.Errorf("updatePastCard reads the description outside the approximate guard")
	}

	meta := jsFunctionBody(t, ended, "buildPastMeta(report)")
	p13MustContain(t, "buildPastMeta", meta,
		"report.approximate === true", "APPROXIMATE_LABEL : Pinalert.categoryLabel(report.category)")
	p13MustNotContain(t, "buildPastMeta", meta, "description")
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
