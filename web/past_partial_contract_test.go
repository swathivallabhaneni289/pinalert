package web

import (
	"strings"
	"testing"
)

// This file pins the partial line of the Past tab (03.1 fixer B2, WR-02). The
// server says with cut.resolved_partial and cut.expired_partial that a candidate
// read was truncated, so reports that belong in a group may be missing even when
// fewer than 25 are shown. The screen must then show that group (its label) even
// with no cards, say so in one plain line, never reuse the cap line for it, and
// never claim "No past reports" while a partial flag is true. Like the other
// contract files nothing here needs a database or a server.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
// D-18): the Past tab left Nearby, so the words of the partial line and the cap
// line are pinned where they moved, in web/static/js/ended-cards.js. The group
// drawing (placePastGroup, renderPastGroups, renderPastTab) went with the tab;
// My activity, Ended nearby draws the groups from these helpers, and its own
// tests pin that drawing. The Phase 3.1 and Phase 4 hands-on tests are walked
// on this layout after Phase 3.4 is built.

const ppLine = "This area has more reports than this list can hold, so some are not shown."

// TestPastPartialLineWords pins the one sentence and its single owner.
func TestPastPartialLineWords(t *testing.T) {
	ended := pt12Ended(t)

	if n := strings.Count(ended, ppLine); n != 1 {
		t.Errorf("ended-cards.js holds the partial line %d times, want 1 (one constant for both groups)", n)
	}
	p13MustContain(t, "ended-cards.js", ended, "var PARTIAL_LINE_TEXT = '"+ppLine+"';")
	for _, dash := range []string{"\u2014", "\u2013"} {
		if strings.Contains(ppLine, dash) {
			t.Errorf("the partial line holds a dash character %q", dash)
		}
	}
	if strings.Contains(pt12Feed(t), ppLine) {
		t.Errorf("feed.js still holds the partial line, it belongs to ended-cards.js")
	}

	// The cap line stays the cap line: it is built from a count only and knows
	// nothing of a partial flag, so a partial flag can never print it.
	cut := jsFunctionBody(t, ended, "cutLineText(count)")
	p13MustContain(t, "cutLineText", cut, "'Showing the newest ' + count + '.'")
	if n := strings.Count(ended, "Showing the newest "); n != 1 {
		t.Errorf("ended-cards.js holds the cap line words %d times, want 1 (one helper for both groups)", n)
	}
	// The helper reads to the next function, so stop at the next declaration.
	cut, _, _ = strings.Cut(cut, "\n  var ")
	p13MustNotContain(t, "cutLineText", cut, "partial", "Partial", "PARTIAL")

	line := jsFunctionBody(t, ended, "groupLineText(count, wasCut, wasPartial)")
	p13MustContain(t, "groupLineText", line,
		"if (wasCut && count > 0) {", "parts.push(cutLineText(count));",
		"if (wasPartial) {", "parts.push(PARTIAL_LINE_TEXT);", "return parts.join(' ');")
	// The cap line sits only under the cut guard and the partial line only under
	// the partial guard, so neither flag can print the other's sentence.
	p13Ordered(t, line, "groupLineText",
		"if (wasCut && count > 0) {", "parts.push(cutLineText(count));", "if (wasPartial) {", "parts.push(PARTIAL_LINE_TEXT);")
	if n := strings.Count(line, "cutLineText("); n != 1 {
		t.Errorf("groupLineText calls cutLineText %d times, want 1 (only under the cut guard)", n)
	}
}
