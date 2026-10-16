package web

import (
	"strings"
	"testing"
)

// TestReviewFixA3ToastUndoRelinks pins the first follow up of fixer A3: a
// distant linked report that a Mark resolved moved off the map comes back
// when its toast Undo reopens it. The toast keeps the link and the set it was
// built in, the Undo puts the report id back before its own refetch only when
// the take back reopened the report and the reader is still on that set, and
// map.js clears its once per link mark when the link ends, so the popup opens
// again from the server's answer.
func TestReviewFixA3ToastUndoRelinks(t *testing.T) {
	votes := p19Code(t, "static/js/votes.js")

	action := jsFunctionBody(t, votes, "resolveToastAction(reportId, result)")
	p13Ordered(t, action, "resolveToastAction",
		"Pinalert.state.focusId === String(reportId) ? linkSetKey() : null",
		"takeBackFromToast(reportId, wasClosed, link)")

	relink := jsFunctionBody(t, votes, "relinkAfterToastUndo(reportId, wasClosed, link, result)")
	p13Ordered(t, relink, "relinkAfterToastUndo",
		"result.visibility === RETRACTED_VISIBILITY_SLUG",
		"link === null || !wasClosed || stillClosed || Pinalert.state.focusId !== null || link !== linkSetKey()",
		"return;", "Pinalert.setFocusId(reportId);")

	key := jsFunctionBody(t, votes, "linkSetKey()")
	p13MustContain(t, "linkSetKey", key, "s.view", "s.category", "s.center.lat", "s.center.lon")

	toast := jsFunctionBody(t, votes, "takeBackFromToast(reportId, wasClosed, link)")
	failAt := strings.Index(toast, "}, function (err) {")
	if failAt == -1 {
		t.Fatalf("takeBackFromToast lost its failure branch")
	}
	p13Ordered(t, toast[:failAt], "takeBackFromToast success",
		"castVote(reportId, 'unresolve')",
		"relinkAfterToastUndo(reportId, wasClosed, link, result);",
		"Pinalert.fetchReports({ afterWrite: true })")
	p13MustNotContain(t, "takeBackFromToast refusal", toast[failAt:], "relinkAfterToastUndo", "setFocusId(")

	if n := strings.Count(votes, "Pinalert.setFocusId("); n != 1 {
		t.Errorf("votes.js calls Pinalert.setFocusId %d times, want 1", n)
	}

	mapJS := p19Code(t, "static/js/map.js")
	end := jsFunctionBody(t, mapJS, "endLinkOnClose()")
	p13Ordered(t, end, "endLinkOnClose", "openedFocusId = null;", "Pinalert.dropFocus();",
		"window.setTimeout(redrawAfterLinkEnded, 0);")
}

// TestReviewFixA3LatestPlaceActionWins pins the second half of IN-07: a
// place search that goes out, or a picked result, cancels a Back to my
// location still waiting, and the position that arrives after that changes
// nothing (no place clear, no centre, no move, no list request). No new
// words are shown.
func TestReviewFixA3LatestPlaceActionWins(t *testing.T) {
	mapJS := p19Code(t, "static/js/map.js")
	p13MustContain(t, "map.js", mapJS, "var recenterSeq = 0;", "cancelRecenter: cancelRecenter")
	back := jsFunctionBody(t, mapJS, "recenterOnReader()")
	p13Ordered(t, back, "recenterOnReader",
		"recenterSeq += 1;", "var mine = recenterSeq;",
		"if (mine !== recenterSeq) {", "resolve({ cancelled: true });", "return;",
		"Pinalert.clearPlace();", "Pinalert.setCenter(lat, lon, own);")
	cancel := jsFunctionBody(t, mapJS, "cancelRecenter()")
	p13MustContain(t, "cancelRecenter", cancel, "recenterSeq += 1;")

	tools := p19Code(t, "static/js/map-tools.js")
	search := jsFunctionBody(t, tools, "search(query, typed)")
	p13MustNotContain(t, "search", search, "backPending")
	// Since quick 261016-sfu a newer search ends the older one rather than
	// being ignored, and only Enter or the button (not typing) cancels Back.
	p13Ordered(t, search, "search",
		"Array.from(q).length < MIN_QUERY_RUNES", "if (!quiet) {\n      cancelBack();", "inFlight = true;")
	pick := jsFunctionBody(t, tools, "pickResult(i)")
	p13Ordered(t, pick, "pickResult", "cancelBack();", "Pinalert.setCenter(pick.lat, pick.lon, false);")
	cb := jsFunctionBody(t, tools, "cancelBack()")
	p13Ordered(t, cb, "cancelBack", "if (!backPending) {", "backPending = false;", "PinalertMap.cancelRecenter();")
	press := jsFunctionBody(t, tools, "onBackPress()")
	p13Ordered(t, press, "onBackPress",
		"PinalertMap.recenterOnReader()", "if (result && result.cancelled === true) {",
		"locationLost = !(result && result.own === true);", "focusSearch();")
}

// TestReviewFixA3FocusFallsBackToMap pins the second follow up: closing a
// popup while the loading mark hides the rows and the marker pane leaves
// focus on the map, never on the page body. A row or a pin counts only when
// focus really landed on it.
func TestReviewFixA3FocusFallsBackToMap(t *testing.T) {
	mapJS := p19Code(t, "static/js/map.js")
	ret := jsFunctionBody(t, mapJS, "returnFocusToOpener(opener)")
	p13Ordered(t, ret, "returnFocusToOpener",
		"rows[i].focus();", "if (document.activeElement === rows[i]) {",
		"pin.focus({ preventScroll: true });", "if (document.activeElement === pin) {",
		"map.getContainer().focus({ preventScroll: true });")
}
