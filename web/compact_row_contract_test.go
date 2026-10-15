package web

// Contract tests for plan 03.1-09: the compact list row (size A of sketch 006)
// at every width, and the plain text trust line that replaces the row's
// toggle. These are static inspections of the embedded scripts and
// stylesheets, in the same spirit as the other contract tests of this
// package: they prove the forbidden shapes are absent and the required shapes
// are present. They cannot prove how a browser lays a row out; the real width
// pass at 320, 360 and 390 px is the orchestrator's browser check (plan 16).
//
// Every helper here carries the compactRow prefix.

import (
	"strings"
	"testing"
)

// TestRowHasNoControls pins D-21: a list row holds no button and no link, no
// vote block and no trust toggle. Its one tab stop is the li itself, which
// still opens the report on click, Enter and Space.
func TestRowHasNoControls(t *testing.T) {
	code := p19Code(t, "static/js/feed.js")
	create := jsFunctionBody(t, code, "createRow(id)")
	update := jsFunctionBody(t, code, "updateRow(row, report)")

	p13MustNotContain(t, "feed.js createRow", create,
		"createElement('button')", "createElement('a')", "createElement(\"button\")",
		"createVoteBlock(", "createTrustBlock(", "createReopenControl(",
		"aria-expanded", "block.controls", "block.note", "block.error")
	p13MustNotContain(t, "feed.js", code,
		"row.votes", "PinalertVotes.createVoteBlock(", "PinalertVotes.updateVoteBlock(",
		"PinalertTrust.createTrustBlock(", "PinalertTrust.updateTrustBlock(")
	p13MustNotContain(t, "feed.js updateRow", update, "updateVoteBlock(", "updateTrustBlock(")

	p13MustContain(t, "feed.js createRow", create,
		"li.setAttribute('tabindex', '0')",
		"li.addEventListener('click'",
		"li.addEventListener('keydown'",
		"evt.key === 'Enter'", "evt.key === ' '",
		"activateRow(id)")
}

// TestCompactRowAnatomy pins the DOM order of the row body (UI-SPEC Surface 2,
// elements 1 to 8): severity label, state line (visibility tag and reporter
// words), meta, distance, far fact, title, trust line, hidden open cue. The
// badge stays in the DOM for the pins' sake and is hidden by CSS.
func TestCompactRowAnatomy(t *testing.T) {
	code := p19Code(t, "static/js/feed.js")
	create := jsFunctionBody(t, code, "createRow(id)")

	p13Ordered(t, create, "feed.js createRow",
		"PinalertTrust.createSeverityLabel()",
		"body.appendChild(severityLabel)",
		"PinalertTrust.createStateLine('report-row__stateline', tag, trust)",
		"body.appendChild(meta)",
		"body.appendChild(away)",
		"body.appendChild(far)",
		"body.appendChild(title)",
		"body.appendChild(trust.el)",
		"body.appendChild(cue)",
		"li.appendChild(badge)",
		"li.appendChild(body)",
	)
	p13MustContain(t, "feed.js createRow", create,
		"PinalertTrust.createTrustLine()",
		"'report-row__away'", "away: away",
		"'report-row__far'", "far: far",
		"'report-row__open-cue visually-hidden'",
		"'Open report'")

	update := jsFunctionBody(t, code, "updateRow(row, report)")
	p13MustContain(t, "feed.js updateRow", update,
		"Pinalert.setText(row.title, report.description)",
		"Pinalert.distanceText(report)",
		"PinalertTrust.updateTrustLine(row.trust, report.trust)")
	// The distance is the store's sentence, never the old feed rule.
	p13MustNotContain(t, "feed.js updateRow", update,
		"Pinalert.formatDistance(", "Pinalert.feedIsAroundReader()")
}

// TestCompactRowTrustLine pins the plain text trust line: a paragraph, no
// button, no aria-expanded, every string through setText, the tone read from
// the server's facts and the words from the server's trust object.
func TestCompactRowTrustLine(t *testing.T) {
	code := p13CodeLines(p13ReadStatic(t, "static/js/trust.js"))
	create := jsFunctionBody(t, code, "createTrustLine()")
	update := jsFunctionBody(t, code, "updateTrustLine(line, trust)")

	p13MustContain(t, "createTrustLine", create,
		"createElement('p')", "'report-row__trustline'", "createReporterWords()", "reporter: reporter")
	p13MustNotContain(t, "createTrustLine", create,
		"'button'", "aria-expanded", "aria-controls", "addEventListener", "createFactsList(", "createStateLine(")
	p13MustContain(t, "updateTrustLine", update,
		"validTrust(trust)", "currencyTone(trust.facts)", "'data-currency-tone'",
		"trust.confirmed_text", "trust.disputed_text", "signalState(trust)", "Pinalert.setText(")
	p13MustContain(t, "trust.js exports", code,
		"createTrustLine: createTrustLine", "updateTrustLine: updateTrustLine",
		"createTrustBlock: createTrustBlock")
}

// TestCompactRowFarFact pins the too far fact of a row (D-14): it reads only
// the cached fix and the vote radius, never asks for a position, and shows the
// exact words only when the helper says the reader is farther than the radius.
func TestCompactRowFarFact(t *testing.T) {
	votes := p13CodeLines(p13ReadStatic(t, "static/js/votes.js"))
	body := jsFunctionBody(t, votes, "farFromCachedFix(report)")

	p13MustContain(t, "farFromCachedFix", body,
		"report.is_own_report", "readVoteRadiusKm()", "peekVoterLocation()",
		"isFreshVoterLocation(fix, VOTER_LOCATION_HINT_MS)", "greatCircleKm(", "> radiusKm")
	p13MustNotContain(t, "farFromCachedFix", body,
		"getCurrentPosition", "navigator.geolocation", "getVoterLocation(", "permissions",
		"sessionStorage", "setItem", "fetch(", ">=", "<=")
	p13MustContain(t, "votes.js exports", votes, "farFromCachedFix: farFromCachedFix")
	if n := strings.Count(votes, "farFromCachedFix: farFromCachedFix"); n != 1 {
		t.Errorf("votes.js exports farFromCachedFix %d times, want 1", n)
	}

	feed := p13ReadStatic(t, "static/js/feed.js")
	if n := strings.Count(feed, "Too far to vote"); n != 1 {
		t.Errorf("feed.js holds the words Too far to vote %d times, want 1", n)
	}
	update := jsFunctionBody(t, p13CodeLines(feed), "updateRow(row, report)")
	p13MustContain(t, "feed.js updateRow", update,
		"PinalertVotes.farFromCachedFix(report)", "row.far.hidden")
}
