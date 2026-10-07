package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// p09ReadJS reads one embedded static file and returns it after
// stripCSSComments, the same reading the other contract tests use. Line
// comments in JavaScript survive that strip, so every assertion below also
// scans comments: a forbidden word in a comment counts.
func p09ReadJS(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return stripCSSComments(string(raw))
}

// p09FuncBody returns the text from "function " + anchor through the first
// module level closing brace after it. It is deliberately tighter than
// jsFunctionBody, which runs on to the next function declaration and so
// swallows that neighbour's leading comment.
func p09FuncBody(t *testing.T, text, anchor string) string {
	t.Helper()
	const closing = "\n  }\n"
	start := strings.Index(text, "function "+anchor)
	if start == -1 {
		t.Fatalf("could not locate %q in the text", "function "+anchor)
	}
	rest := text[start:]
	end := strings.Index(rest, closing)
	if end == -1 {
		t.Fatalf("could not find the module level closing brace after %q", "function "+anchor)
	}
	return rest[:end+len(closing)]
}

// p09Ordered fails naming the first needle that is missing or that does not
// appear after the previous needle.
func p09Ordered(t *testing.T, text, label string, needles ...string) {
	t.Helper()
	pos := 0
	for _, needle := range needles {
		idx := strings.Index(text[pos:], needle)
		if idx == -1 {
			t.Errorf("%s: %q is missing or out of order", label, needle)
			return
		}
		pos += idx + len(needle)
	}
}

// p09MustContain reports every needle that body does not contain.
func p09MustContain(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Errorf("%s does not contain %q", label, needle)
		}
	}
}

// p09MustNotContain reports every needle that body contains.
func p09MustNotContain(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			t.Errorf("%s must not contain %q", label, needle)
		}
	}
}

// TestResolveConfirmIsANamedGroupWithCancelFirst pins amendments A16 and A63
// plus the Cancel half of A52. The inline Mark resolved confirmation is a
// group named by its question through aria-labelledby (the id comes from one
// module counter, never from report data), its DOM order is question, Cancel,
// Yes so a screen reader and the Tab order meet the safe choice first, and
// opening it focuses Cancel while closing it never moves focus by itself.
// Static inspection proves these shapes and their order. It cannot prove how
// a real browser or screen reader announces the group or where focus lands;
// the human check in plan 03-20 covers that.
func TestResolveConfirmIsANamedGroupWithCancelFirst(t *testing.T) {
	text := p09ReadJS(t, "static/js/votes.js")

	if n := strings.Count(text, "var voteBlockSeq = 0;"); n != 1 {
		t.Errorf("votes.js must declare var voteBlockSeq = 0; exactly once, found %d", n)
	}

	body := p09FuncBody(t, text, "createVoteBlock(reportId)")
	if n := strings.Count(body, "++voteBlockSeq"); n != 1 {
		t.Errorf("createVoteBlock must take the counter with ++voteBlockSeq exactly once, found %d", n)
	}
	seqMatch := regexp.MustCompile(`(\w+)\s*=\s*\+\+voteBlockSeq`).FindStringSubmatch(body)
	if seqMatch == nil {
		t.Fatalf("createVoteBlock does not assign ++voteBlockSeq to a local")
	}
	seqLocal := seqMatch[1]

	idMatch := regexp.MustCompile(`(\w+)\s*=\s*'pinalert-resolve-q-'\s*\+\s*(\w+)`).FindStringSubmatch(body)
	if idMatch == nil {
		t.Fatalf("createVoteBlock does not assign the question id from 'pinalert-resolve-q-' plus the counter local")
	}
	idLocal := idMatch[1]
	if idMatch[2] != seqLocal {
		t.Errorf("the question id is built from %q, not from the counter local %q", idMatch[2], seqLocal)
	}

	headingID := regexp.MustCompile(`confirmHeading\.id\s*=\s*(\w+);`).FindStringSubmatch(body)
	if headingID == nil || headingID[1] != idLocal {
		t.Errorf("confirmHeading.id must be set from the id local %q, got %v", idLocal, headingID)
	}
	labelled := regexp.MustCompile(`confirmBlock\.setAttribute\('aria-labelledby',\s*(\w+)\)`).FindStringSubmatch(body)
	if labelled == nil || labelled[1] != idLocal {
		t.Errorf("confirmBlock must set aria-labelledby from the id local %q, got %v", idLocal, labelled)
	}
	p09MustContain(t, "createVoteBlock", body, "confirmBlock.setAttribute('role', 'group')")
	p09MustNotContain(t, "createVoteBlock", body, "aria-describedby")

	p09Ordered(t, body, "createVoteBlock confirmation order",
		"confirmBlock.appendChild(confirmHeading)",
		"confirmBlock.appendChild(cancelBtn)",
		"confirmBlock.appendChild(affirmBtn)",
		"controls.appendChild(confirmBlock)")

	openBody := p09FuncBody(t, text, "openResolveConfirm(block)")
	p09MustContain(t, "openResolveConfirm", openBody, "vote-btn--cancel-resolve", ".focus()")
	p09MustNotContain(t, "openResolveConfirm", openBody, "vote-btn--confirm-resolve")

	closeBody := p09FuncBody(t, text, "closeResolveConfirm(block)")
	p09MustNotContain(t, "closeResolveConfirm", closeBody, ".focus(")

	clickBody := p09FuncBody(t, text, "onControlsClick(block, button)")
	p09Ordered(t, clickBody, "onControlsClick cancel branch",
		"if (action === 'cancel-resolve') {",
		"closeResolveConfirm(block)",
		"focusResolveButton(block)",
		"castVote(")

	focusBody := p09FuncBody(t, text, "focusResolveButton(block)")
	p09MustContain(t, "focusResolveButton", focusBody, "'.vote-btn--resolve'", "canTakeFocus(", ".focus()")
}

// TestFocusStepsAfterMarkResolved pins amendment A52 and the UI-SPEC Busy
// state. Every close path of the confirmation, and every release of a
// Confirm or Dispute vote, ends with focus on a real control. The page body
// is never the destination. The helpers read the native disabled property and
// never assign it, so setBlockBusy stays its only writer, and onControlsClick
// makes no second visibility decision: whether a report left the feed is read
// from the live DOM after the refetch. Static inspection proves the call
// order and the helpers' shapes. It cannot drive a browser, so where focus
// really lands is left to the human check in plan 03-20.
func TestFocusStepsAfterMarkResolved(t *testing.T) {
	text := p09ReadJS(t, "static/js/votes.js")

	declStart := strings.Index(text, "var RELEASE_FOCUS_SELECTORS = {")
	if declStart == -1 {
		t.Fatalf("votes.js does not declare var RELEASE_FOCUS_SELECTORS = {")
	}
	declEnd := strings.Index(text[declStart:], "};")
	if declEnd == -1 {
		t.Fatalf("RELEASE_FOCUS_SELECTORS declaration is not closed")
	}
	p09MustContain(t, "RELEASE_FOCUS_SELECTORS", text[declStart:declStart+declEnd],
		"confirm: '.vote-btn--confirm'",
		"dispute: '.vote-btn--dispute'",
		"'confirm-resolve': '.vote-btn--resolve'")

	clickBody := p09FuncBody(t, text, "onControlsClick(block, button)")
	p09Ordered(t, clickBody, "onControlsClick focus steps",
		"var origin = captureVoteOrigin(block);",
		"var removal = null;",
		"castVote(",
		"removal = captureRemovalFocus(origin);",
		"closeResolveConfirm(block)",
		"Pinalert.fetchReports()",
		"setBlockBusy(block, false)",
		"focusAfterVote(origin, action, removal)",
		".catch(function (err) {",
		"setBlockBusy(block, false)",
		"closeResolveConfirm(block)",
		"focusAfterVote(origin, action, null)")
	p09MustNotContain(t, "onControlsClick", clickBody, ".visibility", "RETRACTED_VISIBILITY_SLUG")

	p09MustContain(t, "captureVoteOrigin", p09FuncBody(t, text, "captureVoteOrigin(block)"),
		".leaflet-popup", "dataset.reportId")
	p09MustContain(t, "findLiveControls", p09FuncBody(t, text, "findLiveControls(origin)"),
		"isConnected", ".vote-controls", "dataset.reportId", ".leaflet-popup")
	p09MustContain(t, "findFeedRow", p09FuncBody(t, text, "findFeedRow(reportId)"),
		".report-row", "dataset.reportId")
	p09MustContain(t, "captureRemovalFocus", p09FuncBody(t, text, "captureRemovalFocus(origin)"),
		"nextElementSibling", "previousElementSibling", "findFeedRow(origin.reportId)", ".leaflet-container")

	afterVote := p09FuncBody(t, text, "focusAfterVote(origin, action, removal)")
	p09Ordered(t, afterVote, "focusAfterVote",
		"RELEASE_FOCUS_SELECTORS[action]",
		"findLiveControls(origin)",
		"focusAfterRetract(origin, removal)")
	p09MustContain(t, "focusAfterVote", afterVote, "focusIsLost(origin.controls)", "canTakeFocus(")

	afterRetract := p09FuncBody(t, text, "focusAfterRetract(origin, removal)")
	p09Ordered(t, afterRetract, "focusAfterRetract",
		"removal.map", "removal.next", "removal.prev", "'view-toggle'", "'feed-empty'")
	p09MustContain(t, "focusAfterRetract", afterRetract,
		"'disputed-empty'", "setAttribute('tabindex', '-1')", "'popup'", "focusIsLost(")

	p09MustContain(t, "isDisplayed", p09FuncBody(t, text, "isDisplayed(el)"), "isConnected", "getClientRects()")
	p09MustContain(t, "canTakeFocus", p09FuncBody(t, text, "canTakeFocus(el)"), "isDisplayed(", ".disabled", ".hidden")
	p09MustContain(t, "focusIsLost", p09FuncBody(t, text, "focusIsLost(scope)"),
		"document.activeElement", "document.body", ".contains(")

	busyStart := strings.Index(text, "function setBlockBusy(block, busy)")
	if busyStart == -1 {
		t.Fatalf("could not locate setBlockBusy")
	}
	busyEnd := busyStart + len(p09FuncBody(t, text, "setBlockBusy(block, busy)"))
	assigns := regexp.MustCompile(`\.disabled\s*=[^=]`).FindAllStringIndex(text, -1)
	if len(assigns) == 0 {
		t.Errorf("no assignment of the native disabled property found at all; setBlockBusy must hold one")
	}
	for _, loc := range assigns {
		if loc[0] < busyStart || loc[0] >= busyEnd {
			t.Errorf("the native disabled property is assigned at offset %d, outside setBlockBusy "+
				"(%d to %d); the focus helpers must only read it", loc[0], busyStart, busyEnd)
		}
	}
}

// TestEscapeCancelHookIsExported pins the outline JS contract for
// cancelResolveConfirm. Plan 03-13 owns the one Escape listener and calls
// this hook; the hook closes an open confirmation inside a container by the
// Cancel path, sends nothing, focuses that block's Mark resolved and returns
// true, and returns false when nothing is open or a resolve is in flight.
// Static inspection proves the shape and the absences. It cannot press a
// key, so the listener's real behaviour is left to plan 03-13's own tests and
// the human check.
func TestEscapeCancelHookIsExported(t *testing.T) {
	text := p09ReadJS(t, "static/js/votes.js")

	if n := strings.Count(text, "cancelResolveConfirm: cancelResolveConfirm"); n != 1 {
		t.Errorf("the export object must hold cancelResolveConfirm: cancelResolveConfirm once, found %d", n)
	}

	body := p09FuncBody(t, text, "cancelResolveConfirm(container)")
	p09MustContain(t, "cancelResolveConfirm", body,
		"'.resolve-confirm:not([hidden])'", "closest('.vote-controls')", "aria-busy",
		"closeResolveConfirm(", "focusResolveButton(", "return true", "return false")
	p09MustNotContain(t, "cancelResolveConfirm", body,
		"castVote(", "getVoterLocation(", "setBlockBusy(", "showVoteError(", "addEventListener")

	p09MustNotContain(t, "createVoteBlock", p09FuncBody(t, text, "createVoteBlock(reportId)"), "'Escape'")
}
