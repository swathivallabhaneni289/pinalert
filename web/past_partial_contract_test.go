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

const ppLine = "This area has more reports than this list can hold, so some are not shown."

// TestPastPartialLineWords pins the one sentence and its single owner.
func TestPastPartialLineWords(t *testing.T) {
	feed := pt12Feed(t)

	if n := strings.Count(feed, ppLine); n != 1 {
		t.Errorf("feed.js holds the partial line %d times, want 1 (one constant for both groups)", n)
	}
	p13MustContain(t, "feed.js", feed, "var PARTIAL_LINE_TEXT = '"+ppLine+"';")
	for _, dash := range []string{"\u2014", "\u2013"} {
		if strings.Contains(ppLine, dash) {
			t.Errorf("the partial line holds a dash character %q", dash)
		}
	}

	// The cap line stays the cap line: it is built from a count only and knows
	// nothing of a partial flag, so a partial flag can never print it.
	cut := jsFunctionBody(t, feed, "cutLineText(count)")
	// The helper reads to the next function, which here includes the constant
	// declared right after cutLineText, so stop at that declaration.
	cut, _, _ = strings.Cut(cut, "\n  var ")
	p13MustNotContain(t, "cutLineText", cut, "partial", "Partial", "PARTIAL")

	line := jsFunctionBody(t, feed, "groupLineText(count, wasCut, wasPartial)")
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

// TestPastPartialGroupShown pins that a partial group keeps its label with no
// cards, shows the line through setText only, and that each group gets its own
// flag.
func TestPastPartialGroupShown(t *testing.T) {
	feed := pt12Feed(t)

	place := jsFunctionBody(t, feed, "placePastGroup(groupEl, listEl, cutEl, reports, wasCut, wasPartial)")
	p13MustContain(t, "placePastGroup", place,
		"var lineText = groupLineText(reports.length, wasCut, wasPartial);",
		"setHidden(groupEl, reports.length === 0 && !wasPartial);",
		"setHidden(cutEl, lineText === '');",
		"Pinalert.setText(cutEl, lineText);")
	p13MustNotContain(t, "placePastGroup", place, "innerHTML", "textContent", "insertAdjacentHTML", "createTextNode(")

	groups := jsFunctionBody(t, feed, "renderPastGroups(state)")
	p13Ordered(t, groups, "renderPastGroups",
		"pastResolvedEl, pastResolvedListEl, pastResolvedCutEl, resolved,",
		"cut.resolved === true, cut.resolved_partial === true);",
		"pastExpiredEl, pastExpiredListEl, pastExpiredCutEl, expired,",
		"cut.expired === true, cut.expired_partial === true);")
	// The two booleans are read as exactly true, so a missing field is false.
	p13MustNotContain(t, "renderPastGroups", groups, "cut.resolved_partial)", "cut.expired_partial)")
}

// TestPastPartialNeverShowsAnEmptyBlock pins that no empty block is drawn while
// either partial flag is true, for the no reports case and the no cards case,
// including a chip on (the filter block would also say nothing is there).
func TestPastPartialNeverShowsAnEmptyBlock(t *testing.T) {
	feed := pt12Feed(t)

	tab := jsFunctionBody(t, feed, "renderPastTab(state)")
	p13MustContain(t, "renderPastTab", tab,
		"var partialCut = state.cut || {};",
		"var partial = partialCut.resolved_partial === true || partialCut.expired_partial === true;",
		"if (state.reports.length === 0 && !partial) {",
		"if (renderPastGroups(state) === 0 && !partial) {")
	p13Ordered(t, tab, "renderPastTab",
		"var partial = ", "if (state.reports.length === 0 && !partial) {", "setEmptyState('none');",
		"setHidden(pastGroupsEl, false);", "if (renderPastGroups(state) === 0 && !partial) {")

	// setEmptyState('past') and setEmptyState('filter') are reached only inside
	// the two guarded branches above.
	if n := strings.Count(tab, "!partial"); n != 2 {
		t.Errorf("renderPastTab guards its empty blocks with !partial %d times, want 2", n)
	}
	if n := strings.Count(tab, "setEmptyState("); n != 3 {
		t.Errorf("renderPastTab calls setEmptyState %d times, want 3 (two guarded empties and the none)", n)
	}
}
