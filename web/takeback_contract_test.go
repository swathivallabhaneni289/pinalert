package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Plan 03.1-14 (HIST-09, D-22): the take back in the opened report and the
// short Undo after Yes, mark resolved. These tests read the shipped source
// text. They cannot run a browser, so where focus really lands and how the
// block looks at 320, 360 and 390 px is the orchestrator's browser pass.

// tbVotes returns votes.js after the shared comment strip.
func tbVotes(t *testing.T) string {
	t.Helper()
	return p09ReadJS(t, "static/js/votes.js")
}

// TestTakeBackNeedsNoLocation proves D-22's "needs no location": the closed
// action list names unresolve, castVote leaves for the network before any
// location step for it and sends the empty object, and the click flow that
// calls it holds no location read. The other four actions keep their
// location rules byte for byte (the existing pins of TestVoteTransportHasNoLocationFallback
// and TestOwnReportResolutionSkipsLocation are unedited).
func TestTakeBackNeedsNoLocation(t *testing.T) {
	text := tbVotes(t)

	p09MustContain(t, "votes.js", text,
		"var VOTE_ACTIONS = ['confirm', 'dispute', 'resolve', 'reopen', 'unresolve'];")

	cast := jsFunctionBody(t, text, "castVote(reportId, action, opts)")
	p09Ordered(t, cast, "castVote take back branch",
		"VOTE_ACTIONS.indexOf(action)",
		"if (action === 'unresolve') {\n      return postVote(null);\n    }",
		"var skipLocation = !!(opts && opts.ownReport) && (action === 'resolve' || action === 'reopen');",
		"getVoterLocation({ fresh: false })")
	p09MustContain(t, "castVote", cast, "body: coords === null ? '{}' : JSON.stringify({ latitude: coords.latitude, longitude: coords.longitude })")

	click := p09FuncBody(t, text, "onTakeBackClick(block, button)")
	p09MustContain(t, "onTakeBackClick", click, "castVote(origin.reportId, action, ")
	p09MustNotContain(t, "onTakeBackClick", click,
		"getVoterLocation(", "ownReportLocation(", "navigator.geolocation", "latitude", "longitude", "peekVoterLocation(")

	// The own report skip and the take back skip are different things: the
	// reporter's resolve and reopen send a placeholder, the take back sends
	// no coordinates at all.
	if n := strings.Count(cast, "'unresolve'"); n != 1 {
		t.Errorf("castVote must name 'unresolve' exactly once (its early branch), found %d", n)
	}
}

// TestTakeBackBlockStates pins takeBackState: it reads report fields only and
// computes no limit, a faded report always gives none, and the words are the
// copy contract's (C46 to C52). It also pins the three heads in main.css that
// lay the block out inside the vote controls grid.
func TestTakeBackBlockStates(t *testing.T) {
	text := tbVotes(t)

	state := p09FuncBody(t, text, "takeBackState(report)")
	p09Ordered(t, state, "takeBackState",
		"isReportFaded(report)", "typeof report.undos_left !== 'number'", "return 'none';",
		"report.your_resolution === 'resolve'", "return finished ? 'undo' : 'undo-waiting';", "return 'used';",
		"report.is_own_report === true && report.view === 'resolved'",
		"return left > 0 ? 'reopen' : 'reopen-used';", "return 'none';")
	p09MustContain(t, "takeBackState", state,
		"var left = report.undos_left;",
		"var finished = report.view === 'resolved' || report.view === 'past';")
	// Report fields only: no store, no clock, no arithmetic on the count.
	p09MustNotContain(t, "takeBackState", state,
		"Pinalert.", "Date.", "Math.", "sessionStorage", "left - ", "left + ", "UNDO_LIMIT", "= 2")

	faded := p09FuncBody(t, text, "isReportFaded(report)")
	p09MustContain(t, "isReportFaded", faded,
		"report.view === 'past'", "new Date(report.expires_at).getTime()", "expires <= Pinalert.serverNow()")
	p09MustNotContain(t, "isReportFaded", faded, "Date.now(")

	// The words, verbatim (C46 to C52, C57 and the button).
	p09MustContain(t, "votes.js", text,
		"'You marked this resolved.'",
		"'You marked this resolved. It closes when other people nearby agree.'",
		"'People nearby marked this resolved.'",
		"var TAKE_BACK_BUTTON_LABEL = 'Undo';",
		"var TAKE_BACK_BUTTON_NAME = 'Undo my resolved mark';",
		"'Undoing…'",
		"'You have used both undos on this report.'",
		"'1 undo left on this report.'",
		"' undos left on this report.'")
	if n := strings.Count(text, "Undo my resolved mark"); n != 1 {
		t.Errorf("the take back accessible name must be written once, found %d", n)
	}

	// The block's markup is built by one helper outside createVoteBlock, so
	// createVoteBlock keeps exactly its three createVoteButton calls.
	build := p09FuncBody(t, text, "createTakeBackBlock(reportId)")
	p09MustContain(t, "createTakeBackBlock", build,
		"el.className = 'vote-takeback'",
		"'vote-note vote-takeback__statement'",
		"createVoteButton('vote-btn vote-btn--takeback', 'unresolve', 'vote-btn__icon--rotate-ccw', TAKE_BACK_BUTTON_LABEL)",
		"undoBtn.setAttribute('aria-label', TAKE_BACK_BUTTON_NAME)",
		"createReopenControl(reportId).button",
		"'vote-note vote-takeback__count'")
	p09MustNotContain(t, "createTakeBackBlock", build, "addEventListener", ".controls", "'vote-controls'")

	// The grid heads in main.css.
	css := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))
	p16MustDecl(t, "take back block head",
		p16Rule(t, css, "", ".vote-controls .vote-takeback:not([hidden])"),
		map[string]string{"grid-column": "1 / -1", "display": "flex", "flex-direction": "column", "gap": "var(--space-xs)"})
	p16MustDecl(t, "closed note head",
		p16Rule(t, css, "", ".vote-controls .vote-closed-note:not([hidden])"),
		map[string]string{"grid-column": "1 / -1"})
	p16MustDecl(t, "take back button head",
		p16Rule(t, css, "", ".vote-controls .vote-btn--takeback:not([hidden])"),
		map[string]string{"width": "100%"})
}

// TestTakeBackFinishedReportIsReadOnly pins the finished report rule: a
// resolved or past report shows the closed note and no Confirm, Dispute or
// Mark resolved, and a live report whose mark is waiting has the block
// replace Mark resolved while Confirm and Dispute stay. Controls that do not
// apply are hidden, never disabled.
func TestTakeBackFinishedReportIsReadOnly(t *testing.T) {
	text := tbVotes(t)

	apply := p09FuncBody(t, text, "applyTakeBackBlock(block, report)")
	p09MustContain(t, "applyTakeBackBlock", apply,
		"var finished = report.view === 'resolved' || report.view === 'past';",
		"var hideResolve = finished || state === 'undo-waiting' || state === 'used';",
		"confirmBtn.hidden = finished;",
		"disputeBtn.hidden = finished;",
		"resolveBtn.hidden = hideResolve;",
		"faded ? CLOSED_NOTE_EXPIRED : CLOSED_NOTE_RESOLVED")
	p09MustNotContain(t, "applyTakeBackBlock", apply, ".disabled", "aria-disabled", "createElement", "innerHTML")
	// A poll must not unhide the primary buttons under an open confirmation.
	p09Ordered(t, apply, "applyTakeBackBlock confirmation guard",
		"var confirmOpen = !!confirmBlock && !confirmBlock.hidden;",
		"if (!confirmOpen) {", "confirmBtn.hidden = finished;")

	p09MustContain(t, "votes.js", text,
		"var CLOSED_NOTE_RESOLVED = 'This report was marked resolved, so voting is off.';",
		"var CLOSED_NOTE_EXPIRED = 'This report has faded, so voting is off.';")

	update := p09FuncBody(t, text, "updateVoteBlock(block, report)")
	p09Ordered(t, update, "updateVoteBlock",
		"applyOwnReportRule(block, report);", "applyTakeBackBlock(block, report);",
		"clearDistanceHint(block);", "applyDistanceHint(block, report);")
	p09MustContain(t, "updateVoteBlock", update, "report.view === 'resolved' || report.view === 'past'")

	// The mount order inside the controls: the three buttons, the
	// confirmation, the closed note, the take back block.
	create := p09FuncBody(t, text, "createVoteBlock(reportId, options)")
	p09Ordered(t, create, "createVoteBlock mount order",
		"controls.appendChild(resolveBtn)", "controls.appendChild(confirmBlock)",
		"controls.appendChild(closedNote)", "controls.appendChild(takeBack.el)",
		"var block = { controls: controls, note: note, error: error };",
		"block.closedNote = closedNote;", "block.takeBack = takeBack;")
	p09MustContain(t, "createVoteBlock", create,
		"closedNote.className = 'vote-note vote-closed-note'", "createTakeBackBlock(reportId)")
	if n := strings.Count(create, "createVoteButton("); n != 3 {
		t.Errorf("createVoteBlock must keep exactly three createVoteButton calls, found %d", n)
	}
	if n := strings.Count(create, "addEventListener"); n != 2 {
		t.Errorf("createVoteBlock must keep exactly two listeners, found %d", n)
	}

	// The two take back buttons leave the shared click flow before it can
	// post a vote, so the poster's Reopen and the Undo have their own busy
	// label and error line.
	click := p09FuncBody(t, text, "onControlsClick(block, button, taken)")
	p09Ordered(t, click, "onControlsClick take back branch",
		"if (action === 'cancel-resolve') {",
		"if (action === 'unresolve' || action === 'reopen') {\n      onTakeBackClick(block, button);\n      return;\n    }",
		"clearVoteError(block);", "castVote(")
	p09MustContain(t, "onControlsClick", click, "{ ownReport: block.isOwnReport === true, location: taken }")

	// The success steps of the take back: refetch with afterWrite, restore
	// the label, release the block, then the toast and the focus steps.
	back := p09FuncBody(t, text, "onTakeBackClick(block, button)")
	p09Ordered(t, back, "onTakeBackClick",
		"setBlockBusy(block, true);", "castVote(", "Pinalert.fetchReports({ afterWrite: true })",
		"setBlockBusy(block, false);", "Pinalert.showToast(message);", "focusAfterTakeBack(origin);",
		"showVoteError(block, takeBackErrorMessage(err));")
	p09MustContain(t, "onTakeBackClick", back,
		"TAKE_BACK_BUTTON_LABEL_IN_FLIGHT", "REOPEN_BUTTON_LABEL_IN_FLIGHT", "REOPEN_SUCCEEDED_TOAST",
		"takeBackState(block.lastReport) === 'undo'", "isReopen && block.isOwnReport === true")
	p09MustNotContain(t, "onTakeBackClick", back, "aria-pressed")

	// The focus helper ends on the current tab link and never on the body.
	focus := p09FuncBody(t, text, "focusAfterTakeBack(origin)")
	p09Ordered(t, focus, "focusAfterTakeBack",
		"focusIsLost(origin.controls)", "findFeedRow(origin.reportId)", "currentTabLink()")
}

// TestTakeBackLimitSentences pins the limit words: the confirmation line when
// no undo is left (C53), the used line (C52), and the error mapping (C54,
// C55, and any other server message as sent). The sentences the server sends
// are read from its handler so a drift on either side fails here.
func TestTakeBackLimitSentences(t *testing.T) {
	text := tbVotes(t)

	if n := strings.Count(text, "You have used both undos on this report, so this can't be undone."); n != 1 {
		t.Errorf("the confirmation line (C53) must be written once, found %d", n)
	}
	create := p09FuncBody(t, text, "createVoteBlock(reportId, options)")
	p09Ordered(t, create, "confirmation group order",
		"confirmBlock.appendChild(confirmHeading)",
		"confirmBlock.appendChild(confirmLimit)",
		"confirmBlock.appendChild(cancelBtn)")
	p09MustContain(t, "createVoteBlock", create,
		"confirmLimit.hidden = true;", "block.confirmLimit = confirmLimit;")
	p09MustContain(t, "applyTakeBackBlock", p09FuncBody(t, text, "applyTakeBackBlock(block, report)"),
		"block.confirmLimit.hidden = !(typeof report.undos_left === 'number' && report.undos_left === 0);")

	errBody := p09FuncBody(t, text, "takeBackErrorMessage(err)")
	p09Ordered(t, errBody, "takeBackErrorMessage",
		"if (!message || message === GENERIC_FAILURE_MESSAGE) {", "return TAKE_BACK_ERROR_MESSAGE;",
		"if (message === SERVER_EXPIRED_REFUSAL) {", "return TAKE_BACK_FADED_MESSAGE;",
		"return message;")
	p09MustContain(t, "votes.js", text,
		"var TAKE_BACK_ERROR_MESSAGE = \"Couldn't undo that. Try again.\";",
		"var TAKE_BACK_FADED_MESSAGE = \"This report has faded, so it can't be changed.\";",
		"var SERVER_EXPIRED_REFUSAL = 'This report has expired.';",
		"var TAKE_BACK_SUCCEEDED_CLOSED_TOAST = 'Taken back. The report is open again.';",
		"var TAKE_BACK_SUCCEEDED_WAITING_TOAST = 'Taken back.';")

	// The failure branch keeps focus on the button and writes the sentence
	// through the one error line.
	back := p09FuncBody(t, text, "onTakeBackClick(block, button)")
	p09Ordered(t, back, "onTakeBackClick failure", "}, function (err) {",
		"showVoteError(block, takeBackErrorMessage(err));", "button.focus();")

	// The server side words, read from the handler that sends them.
	raw, err := os.ReadFile("../internal/api/handlers/votes.go")
	if err != nil {
		t.Fatalf("failed to read the vote handler: %v", err)
	}
	handler := string(raw)
	for _, sentence := range []string{
		`"report", "This report has expired."`,
		`"report", "You have used both undos on this report."`,
		`"report", "You have no resolved mark on this report to take back."`,
	} {
		if !strings.Contains(handler, sentence) {
			t.Errorf("handlers/votes.go no longer sends %s", sentence)
		}
	}
	if !strings.Contains(text, "'You have used both undos on this report.'") {
		t.Errorf("votes.js used line must equal the server's limit sentence")
	}
}

// TestTakeBackNoDashesInNewWords keeps the owner's no long dash rule on the
// strings this plan adds to the module.
func TestTakeBackNoDashesInNewWords(t *testing.T) {
	text := tbVotes(t)
	for _, name := range []string{"TAKE_BACK_", "CLOSED_NOTE_", "SERVER_EXPIRED_REFUSAL"} {
		re := regexp.MustCompile(`(?m)^\s*var ` + name + `[^\n]*$`)
		for _, line := range re.FindAllString(text, -1) {
			if strings.ContainsRune(line, rune(0x2013)) || strings.ContainsRune(line, rune(0x2014)) {
				t.Errorf("a long dash in %q", line)
			}
		}
	}
}

// TestToastAction pins showToast(message, action, durationMs): the one argument form is
// what it always was, the action form holds a text span and one button,
// stays 8 seconds, pauses while the reader is on it and restarts at 4, never
// moves focus into the toast, and a press replaces the content with the
// success text and no action. activity.js keeps its one call with one
// argument and app.js keeps its two network calls.
func TestToastAction(t *testing.T) {
	app := p09ReadJS(t, "static/js/app.js")

	if n := strings.Count(app, "function showToast(message, action, durationMs)"); n != 1 {
		t.Fatalf("app.js must declare showToast(message, action, durationMs) once, found %d", n)
	}
	body := p09FuncBody(t, app, "showToast(message, action, durationMs)")
	p09MustContain(t, "showToast", body,
		"document.getElementById('toast')",
		"setText(toastEl, message);",
		"textEl.className = 'toast__text';",
		"actionEl.className = 'toast__action';",
		"setText(textEl, message);",
		"setText(actionEl, action.label);",
		"actionEl.setAttribute('aria-label', action.ariaLabel);",
		"startToastTimer(toastEl, hasAction ? TOAST_ACTION_MS : plainMs);",
		"toastEl.addEventListener('pointerenter', pauseToast);",
		"toastEl.addEventListener('focusin', pauseToast);",
		"toastEl.addEventListener('pointerleave', resumeToast);",
		"toastEl.addEventListener('focusout', resumeToast);",
		"if (!toastPauseWired) {",
		"toastPauseWired = true;")
	p09MustNotContain(t, "showToast", body,
		".focus(", "innerHTML", "insertAdjacentHTML", "outerHTML", "document.write", "fetch(")
	// The press hands the next sentence to the same function, with no action.
	p09Ordered(t, body, "showToast press flow",
		"actionEl.addEventListener('click', function () {",
		"actionEl.disabled = true;",
		"showToast(next);",
		"action.onClick()")

	p09MustContain(t, "app.js", app,
		"var TOAST_MS = 3000;", "var TOAST_ACTION_MS = 8000;", "var TOAST_RESUME_MS = 4000;")
	if n := strings.Count(app, "startToastTimer(toastEl, TOAST_RESUME_MS);"); n != 1 {
		t.Errorf("the resume restarts the timer at TOAST_RESUME_MS once, found %d", n)
	}
	if n := strings.Count(app, "getElementById('toast')"); n != 1 {
		t.Errorf("app.js must look the toast up once, found %d", n)
	}
	if n := strings.Count(app, "fetch("); n != 2 {
		t.Errorf("app.js must keep exactly two network calls, found %d", n)
	}

	// The template element keeps its live region contract.
	for _, name := range []string{"index.html.tmpl", "profile.html.tmpl"} {
		tmpl := p13ReadTemplate(t, name)
		p09MustContain(t, name, tmpl, `<div id="toast" role="status" aria-live="polite" hidden></div>`)
	}

	// votes.js passes the action only on the resolve outcome, only when the
	// server's count is above zero, and runs the same take back flow.
	votes := tbVotes(t)
	p09MustContain(t, "votes.js", votes,
		"Pinalert.showToast(resolutionOutcomeMessage('resolve', result), resolveToastAction(reportId, result));",
		"var TOAST_ACTION_LABEL = 'Undo';",
		"var TOAST_ACTION_NAME = 'Undo marking this report resolved';")
	action := p09FuncBody(t, votes, "resolveToastAction(reportId, result)")
	p09Ordered(t, action, "resolveToastAction",
		"typeof result.undos_left !== 'number' || result.undos_left <= 0",
		"return undefined;",
		"label: TOAST_ACTION_LABEL", "ariaLabel: TOAST_ACTION_NAME",
		"takeBackFromToast(reportId, wasClosed, link)")
	flow := p09FuncBody(t, votes, "takeBackFromToast(reportId, wasClosed, link)")
	p09Ordered(t, flow, "takeBackFromToast",
		"castVote(reportId, 'unresolve')", "Pinalert.fetchReports({ afterWrite: true })",
		"focusAfterTakeBack(origin);", "takeBackSuccessMessage(wasClosed, result)", "takeBackErrorMessage(err)")

	activity := p09ReadJS(t, "static/js/activity.js")
	if n := strings.Count(activity, "showToast("); n != 1 {
		t.Errorf("activity.js must keep exactly one showToast( call, found %d", n)
	}
	call := regexp.MustCompile(`Pinalert\.showToast\(([^;]*)\);`).FindStringSubmatch(activity)
	if call == nil || strings.Contains(call[1], ",") {
		t.Errorf("activity.js must call showToast with one argument, found %v", call)
	}
}

// TestToastActionStyling pins the look of the toast with an action: the exact
// #toast rule is unchanged, the action rules live under other heads, the
// button is 44 by 44 at least, transparent, outlined in --color-bg with an 8px
// corner and no pill, with its own ring and hover fill, and forced colours
// draw its border in CanvasText.
func TestToastActionStyling(t *testing.T) {
	css := p16ParseCSS("static/css/main.css", p16ReadStatic(t, "static/css/main.css"))

	toast := p16Rule(t, css, "", "#toast")
	p16MustDecl(t, "#toast", toast, map[string]string{
		"position":      "fixed",
		"left":          "50%",
		"bottom":        "var(--space-2xl)",
		"z-index":       "1200",
		"transform":     "translateX(-50%)",
		"background":    "var(--color-text)",
		"color":         "var(--color-bg)",
		"font-size":     "var(--font-size-body)",
		"padding":       "var(--space-sm) var(--space-lg)",
		"border-radius": "8px",
		"animation":     "toast-in 200ms ease",
	})
	if _, has := toast.Decls["display"]; has {
		t.Errorf("the exact #toast rule must not set display (the hidden attribute relies on it)")
	}

	row := p16Rule(t, css, "", "#toast.toast--action:not([hidden])")
	p16MustDecl(t, "toast with an action", row, map[string]string{
		"display":     "flex",
		"align-items": "center",
		"gap":         "var(--space-md)",
		"max-width":   "calc(100vw - var(--space-md) * 2)",
		"padding":     "var(--space-sm) var(--space-sm) var(--space-sm) var(--space-lg)",
		"width":       "max-content",
	})

	btn := p16Rule(t, css, "", ".toast__action")
	p16MustDecl(t, ".toast__action", btn, map[string]string{
		"min-height":    "var(--touch-target-min)",
		"min-width":     "var(--touch-target-min)",
		"padding":       "0 var(--space-md)",
		"border":        "1px solid var(--color-bg)",
		"border-radius": "8px",
		"background":    "transparent",
		"color":         "var(--color-bg)",
		"font-size":     "var(--font-size-body)",
		"font-weight":   "var(--font-weight-semibold)",
	})
	p16MustDecl(t, ".toast__action ring",
		p16Rule(t, css, "", ".toast__action:focus-visible"),
		map[string]string{"outline": "2px solid var(--color-bg)", "outline-offset": "2px"})
	p16MustDecl(t, ".toast__action hover",
		p16Rule(t, css, "", ".toast__action:hover, .toast__action:active"),
		map[string]string{"background": "color-mix(in srgb, var(--color-bg) 15%, transparent)"})
	p16MustDecl(t, ".toast__action forced colours",
		p16Rule(t, css, "(forced-colors: active)", ".toast__action"),
		map[string]string{"border-color": "CanvasText"})

	// Tokens only: no colour literal in any toast rule.
	for _, r := range css {
		if !strings.Contains(r.Head, "toast") {
			continue
		}
		for name, value := range r.Decls {
			if strings.Contains(value, "#") {
				t.Errorf("%s declares %s with a literal colour %q", r.Head, name, value)
			}
		}
	}
}

// TestToastActionContrast: the toast text, the Undo label and its outline are
// --color-bg on --color-text. That pair must reach 4.5 to 1 in both themes,
// computed from the shipped tokens in all four theme sources.
func TestToastActionContrast(t *testing.T) {
	rules := parseCSSRules(p16ReadStatic(t, "static/css/main.css"))
	needed := []string{"--color-bg", "--color-text"}
	light1, light2, dark1, dark2 := extractThemeTokens(rules, needed)
	sources := map[string]themeTokens{"light root": light1, "light override": light2, "dark media": dark1, "dark override": dark2}
	for name, tokens := range sources {
		bg, text := tokens["--color-bg"], tokens["--color-text"]
		if bg == "" || text == "" {
			t.Fatalf("%s: --color-bg or --color-text is not declared", name)
		}
		if ratio := wcagContrastRatio(t, bg, text, "toast "+name); ratio < 4.5 {
			t.Errorf("%s: toast text %s on %s is %.2f to 1, want at least 4.5", name, bg, text, ratio)
		}
	}
}
