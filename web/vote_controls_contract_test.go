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

// TestVoteButtonsCarryIconAndLabelSpans pins amendments A12 and A36. Confirm,
// Dispute and Mark resolved are built by one helper that gives each button an
// aria-hidden icon span and a label span holding the visible word, and every
// later label write goes to the label span, because the text setter replaces
// every child and would delete the icon. Yes and Cancel carry no icon and
// keep plain text writes. Static inspection proves the construction and the
// absence of direct writes. It cannot show the icon, which plan 03-11 draws
// with CSS masks, so the look is left to that plan and the human check.
func TestVoteButtonsCarryIconAndLabelSpans(t *testing.T) {
	text := p09ReadJS(t, "static/js/votes.js")

	buttonBody := p09FuncBody(t, text, "createVoteButton(className, action, iconClass, labelText)")
	p09MustContain(t, "createVoteButton", buttonBody,
		"document.createElement('button')", "button.type = 'button'", "'vote-btn__icon '",
		"setAttribute('aria-hidden', 'true')", "'vote-btn__label'", "setVoteButtonLabel(button, labelText)")
	p09MustNotContain(t, "createVoteButton", buttonBody, "Pinalert.setText(button")

	labelBody := p09FuncBody(t, text, "setVoteButtonLabel(button, text)")
	p09MustContain(t, "setVoteButtonLabel", labelBody, "'.vote-btn__label'", "Pinalert.setText(")

	blockBody := p09FuncBody(t, text, "createVoteBlock(reportId)")
	if n := strings.Count(blockBody, "createVoteButton("); n != 3 {
		t.Errorf("createVoteBlock must build exactly three buttons through createVoteButton, found %d", n)
	}
	p09MustContain(t, "createVoteBlock", blockBody,
		"createVoteButton('vote-btn vote-btn--confirm', 'confirm', 'vote-btn__icon--check', 'Confirm')",
		"createVoteButton('vote-btn vote-btn--dispute', 'dispute', 'vote-btn__icon--ban', 'Dispute')",
		"createVoteButton('vote-btn vote-btn--resolve', 'resolve', 'vote-btn__icon--shield-check', RESOLVE_BUTTON_LABEL)",
		"Pinalert.setText(affirmBtn, RESOLVE_CONFIRM_AFFIRM_LABEL)",
		"Pinalert.setText(cancelBtn, RESOLVE_CONFIRM_CANCEL_LABEL)")

	direct := regexp.MustCompile(`Pinalert\.setText\(\s*(confirmBtn|disputeBtn|resolveBtn|reopenBtn)\b`)
	if loc := direct.FindStringIndex(text); loc != nil {
		t.Errorf("votes.js writes text straight onto a vote button at offset %d, which would wipe its icon span", loc[0])
	}
	p09MustContain(t, "onControlsClick", p09FuncBody(t, text, "onControlsClick(block, button)"),
		"Pinalert.setText(affirmBtn, RESOLVE_BUTTON_LABEL_IN_FLIGHT)")
}

// TestVoteBlockReturnsNoteAndConsumersAppendInOrder pins amendments A66 and
// A8 for the note node. createVoteBlock returns controls, note and error; the
// note is a hidden, empty paragraph whose id comes from the same counter as
// the question id; the feed row and the popup append controls, then note,
// then error, and neither types the note class or touches its hidden state.
// Static inspection proves the order and the absences. It cannot show the
// sentence, which plan 03-18 writes, nor the spacing, which plan 03-11 styles.
func TestVoteBlockReturnsNoteAndConsumersAppendInOrder(t *testing.T) {
	text := p09ReadJS(t, "static/js/votes.js")
	body := p09FuncBody(t, text, "createVoteBlock(reportId)")

	seqMatch := regexp.MustCompile(`(\w+)\s*=\s*\+\+voteBlockSeq`).FindStringSubmatch(body)
	if seqMatch == nil {
		t.Fatalf("createVoteBlock does not assign ++voteBlockSeq to a local")
	}
	noteID := regexp.MustCompile(`note\.id\s*=\s*'pinalert-vote-note-'\s*\+\s*(\w+);`).FindStringSubmatch(body)
	if noteID == nil || noteID[1] != seqMatch[1] {
		t.Errorf("note.id must be 'pinalert-vote-note-' plus the counter local %q, got %v", seqMatch[1], noteID)
	}
	p09MustContain(t, "createVoteBlock", body,
		"'vote-note'", "note.hidden = true", "var block = { controls: controls, note: note, error: error };")

	for _, path := range []string{"static/js/feed.js", "static/js/map.js"} {
		consumer := p09ReadJS(t, path)
		first := regexp.MustCompile(`appendChild\((\w+)\.controls\)`).FindStringSubmatch(consumer)
		if first == nil {
			t.Errorf("%s does not append block.controls", path)
			continue
		}
		p09Ordered(t, consumer, path+" vote block mount order",
			first[0], "appendChild("+first[1]+".note)", "appendChild("+first[1]+".error)")
		p09MustNotContain(t, path, consumer, "vote-note", "note.hidden")
	}
}

// TestVoteErrorGetsNoChildNodes pins amendment A73 on the JavaScript side.
// The vote error paragraph only ever receives text through Pinalert.setText,
// so its alert glyph can live in CSS as a pseudo element. Static inspection
// proves no code appends a child to a node named error and that no file names
// the glyph. It cannot see a child added through some other variable name,
// which review covers.
func TestVoteErrorGetsNoChildNodes(t *testing.T) {
	appendRe := regexp.MustCompile(`\berror\.(appendChild|append|prepend|insertBefore|replaceChildren|insertAdjacentElement)\(`)
	for _, path := range []string{
		"static/js/votes.js", "static/js/activity.js", "static/js/feed.js", "static/js/map.js",
	} {
		text := p09ReadJS(t, path)
		if loc := appendRe.FindStringIndex(text); loc != nil {
			t.Errorf("%s adds a child node to the error paragraph at offset %d", path, loc[0])
		}
		p09MustNotContain(t, path, text, "circle-alert")
	}

	text := p09ReadJS(t, "static/js/votes.js")
	for _, anchor := range []string{"showVoteError(block, message)", "clearVoteError(block)"} {
		body := p09FuncBody(t, text, anchor)
		p09MustContain(t, anchor, body, "Pinalert.setText(block.error")
		p09MustNotContain(t, anchor, body, "createElement")
	}
}

// TestRetapResendsTheSameVote pins amendment A34, the Phase 2 re-tap
// contract. Re-tapping the pressed Confirm or Dispute sends the same vote
// again: there is no take back, no toggle off, and no unvote request. The
// click handler never reads the pressed state, and the transport action is
// still derived only from the clicked action. Static inspection proves the
// absence of such a branch. It cannot prove the server's reply to a repeated
// vote, which the Go service tests cover.
func TestRetapResendsTheSameVote(t *testing.T) {
	text := p09ReadJS(t, "static/js/votes.js")
	body := p09FuncBody(t, text, "onControlsClick(block, button)")

	p09MustNotContain(t, "onControlsClick", body, "aria-pressed")
	p09MustContain(t, "onControlsClick", body, "var transportAction = isResolveAffirm ? 'resolve' : action;")
	p09MustNotContain(t, "votes.js", text, "'DELETE'", "unvote")
}

// TestReopenControlIsSharedFromVotes pins amendments A64, A28 and A36 and
// decision D-20. The Activity page builds Reopen through the one builder in
// votes.js, which returns controls, button and error with no note and no
// listener, and writes every Reopen label through the exported label helper.
// activity.js builds no other vote control. Static inspection proves the
// builder's shape and the absence of a second builder or a direct label
// write. It cannot prove what the page looks like, which plan 03-11 styles,
// nor the click behaviour, which the existing Reopen contract tests and the
// human check cover.
func TestReopenControlIsSharedFromVotes(t *testing.T) {
	votes := p09ReadJS(t, "static/js/votes.js")

	body := p09FuncBody(t, votes, "createReopenControl(reportId)")
	p09MustContain(t, "createReopenControl", body,
		"'vote-controls'",
		"controls.dataset.reportId = String(reportId)",
		"createVoteButton('vote-btn vote-btn--reopen', 'reopen', 'vote-btn__icon--rotate-ccw', REOPEN_BUTTON_LABEL)",
		"controls.appendChild(reopenBtn)",
		"'vote-error'",
		"setAttribute('role', 'alert')",
		"error.hidden = true",
		"return { controls: controls, button: reopenBtn, error: error };")
	p09MustNotContain(t, "createReopenControl", body, "vote-note", "addEventListener")

	p09MustContain(t, "the export object", votes,
		"createReopenControl: createReopenControl", "setReopenLabel: setVoteButtonLabel")

	activity := p09ReadJS(t, "static/js/activity.js")
	p09MustContain(t, "activity.js", activity,
		"PinalertVotes.createReopenControl(row.dataset.reportId)",
		"block.controls.remove()", "block.error.remove()")
	if n := strings.Count(activity, "PinalertVotes.setReopenLabel("); n < 3 {
		t.Errorf("activity.js must write Reopen labels through PinalertVotes.setReopenLabel at least 3 times, found %d", n)
	}
	p09MustNotContain(t, "activity.js", activity,
		"function buildReopenControl", "createElement('button')", "'vote-controls'", "'vote-error'",
		"PinalertVotes.createVoteBlock(")
	for _, pattern := range []string{
		`Pinalert\.setText\(\s*reopenBtn`,
		`Pinalert\.setText\([^;]*REOPEN_BUTTON_LABEL`,
	} {
		if loc := regexp.MustCompile(pattern).FindStringIndex(activity); loc != nil {
			t.Errorf("activity.js writes a Reopen label directly at offset %d (pattern %s)", loc[0], pattern)
		}
	}
}
