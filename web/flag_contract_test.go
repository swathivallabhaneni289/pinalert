package web

import (
	"regexp"
	"strings"
	"testing"

	"pinalert/internal/service"
)

// This file pins plan 04-24: the one Flag control, mounted in the opened
// report (the popup card) and on the Low data row, and the one Flag dialog
// (D-09, P4-A5, P4-A6, P4-A7, P4-A16, P4-A26).
//
// Honest limit of these tests: they inspect the shipped scripts, templates and
// stylesheet as text. Static inspection cannot prove that a browser routes a
// click or a key press the way the listeners are written, that focus really
// lands where the return chain says, or that the page behind the dialog is
// unreachable. The V-24 keyboard and touch steps of plan 04-27 are the runtime
// check.

// p424ReadJS returns an embedded script with its comments removed.
func p424ReadJS(t *testing.T, path string) string {
	t.Helper()
	raw, err := StaticFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", path, err)
	}
	return stripJSComments(string(raw))
}

// p424ReadRaw returns an embedded static file exactly as shipped.
func p424ReadRaw(t *testing.T, path string) string {
	t.Helper()
	raw, err := StaticFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", path, err)
	}
	return string(raw)
}

// p424ReadCSS returns an embedded stylesheet with its comments removed.
func p424ReadCSS(t *testing.T, path string) string {
	t.Helper()
	return stripCSSComments(p424ReadRaw(t, path))
}

// p424ReadTemplate returns an embedded template exactly as shipped.
func p424ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("reading embedded template %s: %v", name, err)
	}
	return string(raw)
}

// p424Ordered requires every needle to be found, each one after the end of the
// previous match, and names the first needle that is not.
func p424Ordered(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	from := 0
	for _, needle := range needles {
		i := strings.Index(text[from:], needle)
		if i == -1 {
			t.Errorf("%s: %q not found after offset %d", label, needle, from)
			return
		}
		from += i + len(needle)
	}
}

// p424Contains fails for each needle that text does not hold.
func p424Contains(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(text, needle) {
			t.Errorf("%s does not contain %q", label, needle)
		}
	}
}

// p424Lacks fails for each needle that text holds.
func p424Lacks(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			t.Errorf("%s contains %q", label, needle)
		}
	}
}

// p424NoDashes fails on an em dash, an en dash or an ellipsis character.
func p424NoDashes(t *testing.T, label, raw string) {
	t.Helper()
	for _, bad := range []string{string(rune(0x2014)), string(rune(0x2013)), string(rune(0x2026))} {
		if strings.Contains(raw, bad) {
			t.Errorf("%s contains the character %q", label, bad)
		}
	}
}

// TestFlagControlClickDoesNotActivateItsRow pins P4-A7. The name is kept
// although no main page row holds the control any more (03.1 UI-SPEC R9): the
// two propagation stops keep the one builder safe on any surface it is mounted
// on, and the row half proves that no list row or Past card builds a control.
func TestFlagControlClickDoesNotActivateItsRow(t *testing.T) {
	flag := p424ReadJS(t, "static/js/flag.js")
	b := jsFunctionBody(t, flag, "createFlagControl(report)")
	if n := strings.Count(b, "stopPropagation"); n < 2 {
		t.Errorf("createFlagControl stops propagation %d times, want at least 2", n)
	}
	if n := strings.Count(b, "addEventListener("); n != 2 {
		t.Errorf("createFlagControl registers %d listeners, want exactly 2", n)
	}
	p424Contains(t, "createFlagControl", b,
		"'click'", "'keydown'", "'Enter'", "' '", "'Spacebar'", "createElement('button')",
		"'flag-control'", "dataset.flagReportId", "return { el: el }")
	p424Lacks(t, "createFlagControl", b, "prevent"+"Default")

	feed := p424ReadJS(t, "static/js/feed.js")
	p424Lacks(t, "feed.js", feed, "PinalertFlag")
}

// TestFlagControlAbsentOnOwnReport pins the own report rule (D-09): the control
// is hidden, never disabled, and the Activity page never carries it.
func TestFlagControlAbsentOnOwnReport(t *testing.T) {
	flag := p424ReadJS(t, "static/js/flag.js")
	u := jsFunctionBody(t, flag, "updateFlagControl(control, report)")
	p424Contains(t, "updateFlagControl", u,
		"report.is_own_report === true", ".hidden = ", "isFlagged(report)", "FLAGGED_LABEL", "FLAG_LABEL",
		"'aria-label'", "accessibleName(", "setAttribute('aria-disabled', 'true')",
		"removeAttribute('aria-disabled')", "Pinalert.setText(")
	p424Lacks(t, "updateFlagControl", u, ".disabled")

	p424Contains(t, "isFlagged", jsFunctionBody(t, flag, "isFlagged(report)"),
		"report.viewer_flagged === true", "flaggedHere[")
	p424Contains(t, "accessibleName", jsFunctionBody(t, flag, "accessibleName(prefix, report)"),
		"Pinalert.CATEGORY_LABELS[", "Pinalert.relativeTime(report.created_at)", "', '")

	activity := p424ReadJS(t, "static/js/activity.js")
	p424Lacks(t, "activity.js", activity, "PinalertFlag")
}

// TestFlagControlBuiltOnceUpdatedInPlace pins P4-A6: the control is built once
// per surface and refreshed in place on every poll.
func TestFlagControlBuiltOnceUpdatedInPlace(t *testing.T) {
	flag := p424ReadJS(t, "static/js/flag.js")
	u := jsFunctionBody(t, flag, "updateFlagControl(control, report)")
	p424Lacks(t, "updateFlagControl", u, "createElement(", ".focus(", "getElementById(", "openFlagDialog(", "refs")

	m := jsFunctionBody(t, flag, "markFlaggedEverywhere(reportId)")
	p424Contains(t, "markFlaggedEverywhere", m,
		"flaggedHere[", "querySelectorAll('.flag-control')", "dataset.flagReportId", "FLAGGED_LABEL", "'aria-disabled'")
	p424Lacks(t, "markFlaggedEverywhere", m, "[data-flag-report-id=")

	p424Contains(t, "flag.js", flag, "createFlagControl: createFlagControl", "updateFlagControl: updateFlagControl")
	if n := strings.Count(flag, "window.PinalertFlag = "); n != 1 {
		t.Errorf("flag.js assigns window.PinalertFlag %d times, want 1", n)
	}

	// The opened report (P4-A6): built once after the vote error, refreshed
	// right after the vote block, never created by the update.
	mapjs := p424ReadJS(t, "static/js/map.js")
	p424Ordered(t, "buildPopupCard", jsFunctionBody(t, mapjs, "buildPopupCard(report)"),
		"block.error", "PinalertFlag.createFlagControl(report)", ".appendChild(flagControl.el)",
		"flag: flagControl", "updatePopupCard(card, report)")
	upd := jsFunctionBody(t, mapjs, "updatePopupCard(card, report)")
	flagAt := strings.Index(upd, "PinalertFlag.updateFlagControl(card.flag, report)")
	voteAt := strings.Index(upd, "PinalertVotes.updateVoteBlock(card.votes, report)")
	if flagAt == -1 || voteAt == -1 || flagAt < voteAt {
		t.Errorf("updatePopupCard must refresh the Flag control after the vote block (flag at %d, vote at %d)", flagAt, voteAt)
	}
	p424Lacks(t, "updatePopupCard", upd, "createFlagControl(")
	if n := strings.Count(mapjs, "PinalertFlag.createFlagControl("); n != 1 {
		t.Errorf("map.js builds the Flag control %d times, want 1", n)
	}

	// The marker seam used by the focus return chain.
	p424Contains(t, "map.js", mapjs, "focusMarker: focusMarker")
	p424Contains(t, "focusMarker", jsFunctionBody(t, mapjs, "focusMarker(id)"),
		"markers[", "getElement()", "isConnected", ".focus(", "return true", "return false")

	// The Low data row: built once after the vote block error, kept as the
	// row's flag part, refreshed right after the vote block update. The row is
	// a plain object literal returned by createLowRow, so the control joins it
	// as the flag key (amended from the assignment form, which this base lacks).
	low := p424ReadJS(t, "static/js/low.js")
	create := jsFunctionBody(t, low, "createLowRow(report)")
	p424Ordered(t, "createLowRow", create,
		"PinalertVotes.createVoteBlock(report.id, { resolve: false })", "PinalertFlag.createFlagControl(report)",
		".appendChild(flagControl.el)", "flag: flagControl")
	p424Lacks(t, "createLowRow", create, "addEventListener(", "tabindex")
	lowUpd := jsFunctionBody(t, low, "updateLowRow(row, report)")
	lowFlagAt := strings.Index(lowUpd, "PinalertFlag.updateFlagControl(row.flag, report)")
	lowVoteAt := strings.Index(lowUpd, "PinalertVotes.updateVoteBlock(row.voteBlock, report)")
	if lowFlagAt == -1 || lowVoteAt == -1 || lowFlagAt < lowVoteAt {
		t.Errorf("updateLowRow must refresh the Flag control after the vote block (flag at %d, vote at %d)", lowFlagAt, lowVoteAt)
	}
	p424Lacks(t, "updateLowRow", lowUpd, "createFlagControl(")
	if n := strings.Count(low, "PinalertFlag.createFlagControl("); n != 1 {
		t.Errorf("low.js builds the Flag control %d times, want 1", n)
	}
}

// TestFlagScriptLoadOrder pins where flag.js and the dialog partial load:
// after queue.js and before the form, map, feed and Low data scripts, and on
// the main page and the Low data view only.
func TestFlagScriptLoadOrder(t *testing.T) {
	const tag = `<script src="/static/js/flag.js?v={{.AssetVersion}}" defer></script>`
	index := p424ReadTemplate(t, "index.html.tmpl")
	if n := strings.Count(index, tag); n != 1 {
		t.Errorf("index.html.tmpl loads flag.js %d times, want 1", n)
	}
	p424Ordered(t, "index.html.tmpl", index,
		"/static/js/queue.js", "/static/js/flag.js", "/static/js/report-form.js")
	p424Ordered(t, "index.html.tmpl", index,
		"/static/js/flag.js", "/static/js/map.js", "/static/js/feed.js")

	low := p424ReadTemplate(t, "low.html.tmpl")
	if n := strings.Count(low, tag); n != 1 {
		t.Errorf("low.html.tmpl loads flag.js %d times, want 1", n)
	}
	p424Ordered(t, "low.html.tmpl", low,
		"/static/js/queue.js", "/static/js/flag.js", "/static/js/report-form.js", "/static/js/low.js")

	for name, text := range findFullPageTemplates(t) {
		page := strings.TrimPrefix(name, "templates/")
		wantsFlag := page == "index.html.tmpl" || page == "low.html.tmpl"
		if got := strings.Contains(text, "/static/js/flag.js"); got != wantsFlag {
			t.Errorf("%s: loads flag.js = %v, want %v", page, got, wantsFlag)
		}
		if got := strings.Contains(text, "flag_dialog.html.tmpl"); got != wantsFlag {
			t.Errorf("%s: calls the Flag dialog partial = %v, want %v", page, got, wantsFlag)
		}
	}
}

// p424RadioPattern finds the radio inputs of the Flag dialog partial.
var p424RadioPattern = regexp.MustCompile(`<input[^>]*type="radio"[^>]*>`)

// p424ValuePattern reads the value attribute of a tag.
var p424ValuePattern = regexp.MustCompile(`value="([^"]*)"`)

// p424LabelPattern finds the label text spans of the Flag dialog partial.
var p424LabelPattern = regexp.MustCompile(`<span class="flag-dialog__label">([^<]*)</span>`)

// TestFlagDialogMatchesServerReasons proves that the five reasons in the
// dialog markup are the server's FlagReasons, in order, with the server's
// labels (D-09), so the words live once in Go and once in the partial.
func TestFlagDialogMatchesServerReasons(t *testing.T) {
	partial := p424ReadTemplate(t, "flag_dialog.html.tmpl")
	radios := p424RadioPattern.FindAllStringIndex(partial, -1)
	labels := p424LabelPattern.FindAllStringSubmatch(partial, -1)
	if len(radios) != len(service.FlagReasons) || len(labels) != len(service.FlagReasons) {
		t.Fatalf("partial holds %d radios and %d labels, want %d of each", len(radios), len(labels), len(service.FlagReasons))
	}
	for i, reason := range service.FlagReasons {
		tag := partial[radios[i][0]:radios[i][1]]
		if !strings.Contains(tag, `name="flag-reason"`) {
			t.Errorf("radio %d has no name flag-reason: %s", i, tag)
		}
		if strings.Contains(tag, "checked") {
			t.Errorf("radio %d starts checked: %s", i, tag)
		}
		value := p424ValuePattern.FindStringSubmatch(tag)
		if value == nil || value[1] != string(reason) {
			t.Errorf("radio %d value = %v, want %q", i, value, string(reason))
		}
		if labels[i][1] != service.FlagReasonLabel(reason) {
			t.Errorf("label %d = %q, want %q", i, labels[i][1], service.FlagReasonLabel(reason))
		}
		if reason == service.FlagFalse {
			if !strings.Contains(tag, `aria-describedby="flag-dialog-hint"`) {
				t.Errorf("the %q radio does not point at the Dispute hint: %s", reason, tag)
			}
			labelEnd := strings.Index(partial[radios[i][0]:], "</label>")
			hintAt := strings.Index(partial, `<p id="flag-dialog-hint"`)
			next := len(partial)
			if i+1 < len(radios) {
				next = radios[i+1][0]
			}
			if labelEnd == -1 || hintAt < radios[i][0]+labelEnd || hintAt > next {
				t.Errorf("the Dispute hint must sit after the %q label and before the next radio", reason)
			}
		}
	}
}

// TestFlagDialogCopy pins the exact words of the dialog (D-09) and the things
// it must not hold: no free text, no outside reporting line, no number.
func TestFlagDialogCopy(t *testing.T) {
	partial := p424ReadTemplate(t, "flag_dialog.html.tmpl")
	p424Contains(t, "flag_dialog.html.tmpl", partial,
		"What is wrong with this report?",
		"If you only disagree with the facts, use Dispute instead.",
		"A flag never hides a post by itself. A moderator reads it. The author isn't told who flagged it.",
		"Send to moderators", "Go back", "Choose a reason to continue.")

	rawJS := p424ReadRaw(t, "static/js/flag.js")
	flag := stripJSComments(rawJS)
	p424Contains(t, "flag.js", flag,
		"'Flag this report'", "'Flagged'", "'Sending'", "'Send to moderators'",
		"'Flag sent. Moderators will see it.'",
		"'You already flagged this report. Moderators will see it.'",
		`"You're offline, so this wasn't sent. Flags are never saved for later. Try again when you have a signal."`,
		"'This report is no longer available.'", "'Something went wrong. Try again.'")

	for label, raw := range map[string]string{"flag.js": rawJS, "flag_dialog.html.tmpl": partial} {
		p424NoDashes(t, label, raw)
		lower := strings.ToLower(raw)
		p424Lacks(t, label, lower, "police", "tel:", "112", "strike", "abuse report")
	}
	p424Lacks(t, "flag_dialog.html.tmpl", partial, "<textarea", `type="text"`, "<html")

	for _, reason := range service.FlagReasons {
		p424Lacks(t, "flag.js", flag, service.FlagReasonLabel(reason), "'"+string(reason)+"'", `"`+string(reason)+`"`)
	}
}

// TestFlagDialogLocksPage pins the dialog shell, the page lock and the focus
// return (P4-A5, P4-A26).
func TestFlagDialogLocksPage(t *testing.T) {
	partial := p424ReadTemplate(t, "flag_dialog.html.tmpl")
	p424Contains(t, "flag_dialog.html.tmpl", partial,
		`id="flag-dialog"`, `class="modal-backdrop flag-dialog"`, `class="modal-panel flag-dialog__panel"`,
		`role="dialog"`, `aria-modal="true"`, `aria-labelledby="flag-dialog-title"`)
	backdropTag := regexp.MustCompile(`<div[^>]*id="flag-dialog"[^>]*>`).FindString(partial)
	if !strings.Contains(backdropTag, " hidden") {
		t.Errorf("the dialog backdrop must start hidden: %q", backdropTag)
	}
	titleTag := regexp.MustCompile(`<h2[^>]*id="flag-dialog-title"[^>]*>`).FindString(partial)
	p424Contains(t, "dialog title tag", titleTag, `id="flag-dialog-title"`, `class="flag-dialog__title"`, `tabindex="-1"`)

	flag := p424ReadJS(t, "static/js/flag.js")
	p424Ordered(t, "openFlagDialog", jsFunctionBody(t, flag, "openFlagDialog(opener)"),
		"dialogRefs()", ".hidden = false", "Pinalert.lockPage(", ".focus(")
	p424Ordered(t, "closeFlagDialog", jsFunctionBody(t, flag, "closeFlagDialog()"),
		"if (sending)", ".hidden = true", "Pinalert.unlockPage(", "returnFocus(")
	p424Contains(t, "onDialogKeydown", jsFunctionBody(t, flag, "onDialogKeydown(evt)"),
		"'Escape'", "stopPropagation", "closeFlagDialog(", "'Tab'", "sending", "focusableInPanel(")
	p424Contains(t, "dialogRefs", jsFunctionBody(t, flag, "dialogRefs()"), "evt.target === ")
	p424Contains(t, "returnFocus", jsFunctionBody(t, flag, "returnFocus(state)"),
		"isShown(", "surfaceOf(", "'.report-row, .past-card'", "rowNext", "rowPrev", "getElementById('map')",
		"PinalertMap.focusMarker(", "getElementById('low-report')")

	trust := p424ReadJS(t, "static/js/trust.js")
	p424Contains(t, "trust.js", trust, "data-modal-open")

	p424CheckFlagPartialPlacement(t, "index.html.tmpl")
	p424CheckFlagPartialPlacement(t, "low.html.tmpl")
}

// p424CheckFlagPartialPlacement runs the three placement checks on a page:
// the partial call occurs once, only whitespace sits between it and the toast
// element, and it sits outside the report dialog backdrop.
func p424CheckFlagPartialPlacement(t *testing.T, page string) {
	t.Helper()
	text := p424ReadTemplate(t, page)
	const call = `{{template "flag_dialog.html.tmpl" .}}`
	if n := strings.Count(text, call); n != 1 {
		t.Errorf("%s calls the Flag dialog partial %d times, want 1", page, n)
		return
	}
	at := strings.Index(text, call)
	after := strings.TrimLeft(text[at+len(call):], " \t\r\n")
	if !strings.HasPrefix(after, `<div id="toast"`) {
		t.Errorf("%s: the Flag dialog partial must sit directly before the toast element", page)
	}
	modalAt := strings.Index(text, `id="report-modal"`)
	if modalAt == -1 {
		t.Errorf("%s has no report dialog", page)
		return
	}
	open := strings.LastIndex(text[:modalAt], "<div")
	if open == -1 || open > at {
		t.Errorf("%s: could not find the tag that opens the report dialog", page)
		return
	}
	slice := text[open:at]
	if opens, closes := strings.Count(slice, "<div"), strings.Count(slice, "</div>"); opens != closes {
		t.Errorf("%s: the Flag dialog partial sits inside the report dialog (%d opening and %d closing div tags before it)", page, opens, closes)
	}
}

// TestFlagSendOutcomes pins the one request and what each answer does (D-09,
// D-12): exactly the reason slug, a 10 second bound, nothing saved for later.
func TestFlagSendOutcomes(t *testing.T) {
	flag := p424ReadJS(t, "static/js/flag.js")
	if n := strings.Count(flag, "fetch("); n != 1 {
		t.Errorf("flag.js makes %d request calls, want exactly 1", n)
	}
	if n := strings.Count(flag, "var FLAG_TIMEOUT_MS = 10000;"); n != 1 {
		t.Errorf("flag.js declares the timeout %d times, want 1", n)
	}
	s := jsFunctionBody(t, flag, "sendFlag(reason)")
	p424Contains(t, "sendFlag", s,
		"'/api/reports/' + encodeURIComponent(", "'/flag'", "method: 'POST'", "'Content-Type': 'application/json'",
		"JSON.stringify({ reason: reason })", "handleAnswer(", "setSending(true)")
	// The shared time limit of app.js, raced against the post and its body
	// read, so a post whose aborted request never ends (WebKit) still shows
	// the offline line at the limit.
	p424Ordered(t, "sendFlag time limit", s,
		"var limit = Pinalert.requestTimeout(FLAG_TIMEOUT_MS, ", "Promise.race([fetch(",
		"signal: limit.signal", "}), limit.expired]).then(function (res) {",
		"Promise.race([res.json(), limit.expired]).catch(function () {", "limit.clear();", "handleAnswer(",
		"limit.clear();", "showError(FLAG_OFFLINE_MESSAGE)")
	h := jsFunctionBody(t, flag, "handleAnswer(status, body, headers)")
	p424Contains(t, "handleAnswer", h,
		"setSending(false)", "201", "409", "FLAG_SENT_TOAST", "FLAG_ALREADY_TOAST", "FLAG_ERROR_MESSAGE")
	p424CheckAnswersAndWaits(t)
	p424Ordered(t, "finishFlagged", jsFunctionBody(t, flag, "finishFlagged(toast)"),
		"markFlaggedEverywhere(", "closeFlagDialog(", "Pinalert.showToast(toast)")
	p424Contains(t, "setSending", jsFunctionBody(t, flag, "setSending(on)"), "sending = on", ".disabled = on")
	p424Contains(t, "showError", jsFunctionBody(t, flag, "showError(message)"), "setAttribute('role', 'alert')", "Pinalert.setText(")
	p424Contains(t, "hideError", jsFunctionBody(t, flag, "hideError()"), "removeAttribute('role')")
	p424Contains(t, "showNote", jsFunctionBody(t, flag, "showNote()"), "'aria-describedby'", "announce(", ".focus(")
	p424Contains(t, "announce", jsFunctionBody(t, flag, "announce(text)"), "getElementById('notice-live')", "Pinalert.setText(")
	p424Lacks(t, "flag.js", flag, "indexedDB", "localStorage", "sessionStorage", "PinalertQueue")
}

// p424CheckAnswersAndWaits runs inside TestFlagSendOutcomes and pins the answers beyond a sent flag (D-09, D-12): a
// 429 keeps the server's message and remembers the wait, a known wait sends
// nothing, a gone report keeps Go back only, and a rejection says the flag was
// not saved for later.
func p424CheckAnswersAndWaits(t *testing.T) {
	t.Helper()
	flag := p424ReadJS(t, "static/js/flag.js")
	p424Contains(t, "sendFlag", jsFunctionBody(t, flag, "sendFlag(reason)"), "FLAG_OFFLINE_MESSAGE")
	p424Contains(t, "handleAnswer", jsFunctionBody(t, flag, "handleAnswer(status, body, headers)"),
		"429", "404", "410", "rememberFlagWait(", "FLAG_GONE_MESSAGE", ".hidden = true",
		"showError(rememberFlagWait(body, headers))")
	p424Ordered(t, "onSendClick", jsFunctionBody(t, flag, "onSendClick()"),
		"if (sending)", "checkedReason(", "showNote(", "flagWait", "waitMessageNow(", "return", "sendFlag(")
	p424Contains(t, "waitMessageNow", jsFunctionBody(t, flag, "waitMessageNow()"),
		"Math.ceil(", "Pinalert.formatWait(", "flagWait.phrase")
	p424Contains(t, "rememberFlagWait", jsFunctionBody(t, flag, "rememberFlagWait(body, headers)"),
		"retry_after_seconds", "'Retry-After'", "body.error", "Pinalert.formatWait(", "FLAG_ERROR_MESSAGE", "return message")
	p424Contains(t, "setSending", jsFunctionBody(t, flag, "setSending(on)"),
		"SENDING_LABEL", "SEND_LABEL", "'aria-busy'", "removeAttribute('aria-busy')")
}

// p424MediaRules returns the rules inside the first media block of chrome.css
// that holds the selector, keyed by the query text.
func p424MediaRules(t *testing.T, css, query string) []cssRule {
	t.Helper()
	var all []cssRule
	for _, span := range p408MediaSpans(css, query) {
		all = append(all, parseCSSRules(css[span[1]:span[2]-1])...)
	}
	return all
}

// p424WantDecls fails for each declaration the rule lacks, comparing with
// whitespace squashed.
func p424WantDecls(t *testing.T, label string, rules []cssRule, selector string, want map[string]string) {
	t.Helper()
	rule, ok := ruleBySelector(rules, selector)
	if !ok {
		t.Errorf("%s: no rule %q", label, selector)
		return
	}
	decls := declsOf(rule.declBody)
	for prop, value := range want {
		if p408Squash(decls[prop]) != p408Squash(value) {
			t.Errorf("%s %q: %s = %q, want %q", label, selector, prop, decls[prop], value)
		}
	}
}

// TestFlagControlGeometry pins the look of the control (Element table,
// Motion): a quiet underlined text button with a 44px target, a hover fill, an
// inset press cue, a Flagged look with no underline, and the two media rules.
func TestFlagControlGeometry(t *testing.T) {
	css := p424ReadCSS(t, "static/css/chrome.css")
	rules := parseCSSRules(css)
	p424WantDecls(t, "chrome.css", rules, ".flag-control:not([hidden])", map[string]string{
		"display": "inline-flex", "align-items": "center", "min-height": "var(--touch-target-min)",
		"padding": "0 var(--space-xs)", "margin-top": "var(--space-xs)", "border": "0",
		"border-radius": "8px", "background": "none", "font-family": "var(--font-family)",
		"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
		"font-weight": "var(--font-weight-regular)", "color": "var(--color-text)",
		"text-decoration": "underline", "cursor": "pointer",
	})
	rule, _ := ruleBySelector(rules, ".flag-control:not([hidden])")
	transition := declsOf(rule.declBody)["transition"]
	p424Contains(t, "flag-control transition", transition, "background-color", "border-color", "color", "150ms ease")
	p424WantDecls(t, "chrome.css", rules, `.flag-control:not([aria-disabled="true"]):hover`,
		map[string]string{"background": "var(--color-border)"})
	p424WantDecls(t, "chrome.css", rules, `.flag-control:not([aria-disabled="true"]):active`,
		map[string]string{"background": "var(--color-border)", "box-shadow": "inset 0 0 0 1px var(--color-text-muted)"})
	p424WantDecls(t, "chrome.css", rules, `.flag-control[aria-disabled="true"]`,
		map[string]string{"text-decoration": "none", "cursor": "default", "color": "var(--color-text)"})

	for _, r := range rules {
		for _, sel := range p408Selectors(r.selectorHead) {
			if !strings.Contains(sel, ".flag-control") {
				continue
			}
			decls := declsOf(r.declBody)
			if _, has := decls["outline"]; has {
				t.Errorf("%q declares outline", sel)
			}
			if _, has := decls["display"]; has && !strings.Contains(sel, ":not([hidden])") {
				t.Errorf("%q declares display without :not([hidden])", sel)
			}
		}
	}

	p424WantDecls(t, "reduced motion", p424MediaRules(t, css, "(prefers-reduced-motion: reduce)"),
		".flag-control:not([hidden])", map[string]string{"transition": "none"})
	forced := p424MediaRules(t, css, "(forced-colors: active)")
	p424WantDecls(t, "forced colors", forced, ".flag-control:not([hidden])", map[string]string{"text-decoration": "underline"})
	p424WantDecls(t, "forced colors", forced, `.flag-control[aria-disabled="true"]`, map[string]string{"text-decoration": "none"})
}

// TestFlagDialogGeometry pins the dialog look (Element table Flag dialog,
// Typography, Spacing Scale): the title at heading size, 44px reason rows with
// 20px radios, small muted hint and reassurance lines, wrapping buttons.
func TestFlagDialogGeometry(t *testing.T) {
	css := p424ReadCSS(t, "static/css/chrome.css")
	rules := parseCSSRules(css)
	p424WantDecls(t, "chrome.css", rules, ".modal-panel .flag-dialog__title", map[string]string{
		"font-size": "var(--font-size-heading)", "line-height": "var(--line-height-heading)",
		"font-weight": "var(--font-weight-semibold)", "margin": "0 0 var(--space-md) 0",
	})
	p424WantDecls(t, "chrome.css", rules, ".flag-dialog__reasons", map[string]string{
		"border": "0", "padding": "0", "margin": "0", "min-width": "0",
	})
	p424WantDecls(t, "chrome.css", rules, ".flag-dialog__reason:not([hidden])", map[string]string{
		"display": "flex", "align-items": "center", "gap": "var(--space-sm)", "min-height": "var(--touch-target-min)",
	})
	p424WantDecls(t, "chrome.css", rules, ".flag-dialog__reason input", map[string]string{
		"width": "calc(var(--space-md) + var(--space-xs))", "height": "calc(var(--space-md) + var(--space-xs))",
		"margin": "0", "accent-color": "var(--color-text)",
	})
	// The hint and the reassurance share one rule head.
	for _, sel := range []string{".flag-dialog__hint", ".flag-dialog__reassurance"} {
		found := false
		for _, r := range rules {
			for _, s := range p408Selectors(r.selectorHead) {
				if s != sel {
					continue
				}
				found = true
				decls := declsOf(r.declBody)
				for prop, value := range map[string]string{
					"font-size": "var(--font-size-label)", "line-height": "var(--line-height-label)",
					"color": "var(--color-text-muted)", "margin-bottom": "0",
				} {
					if p408Squash(decls[prop]) != p408Squash(value) {
						t.Errorf("%q: %s = %q, want %q", sel, prop, decls[prop], value)
					}
				}
			}
		}
		if !found {
			t.Errorf("chrome.css has no rule for %q", sel)
		}
	}
	p424WantDecls(t, "chrome.css", rules, ".flag-dialog__actions:not([hidden])", map[string]string{
		"display": "flex", "flex-wrap": "wrap", "gap": "var(--space-sm)",
	})
	p424WantDecls(t, "forced colors", p424MediaRules(t, css, "(forced-colors: active)"),
		".flag-dialog__panel", map[string]string{"border": "1px solid CanvasText"})
	for _, r := range rules {
		for _, sel := range p408Selectors(r.selectorHead) {
			if !strings.Contains(sel, ".flag-dialog") {
				continue
			}
			if _, has := declsOf(r.declBody)["display"]; has && !strings.Contains(sel, ":not([hidden])") {
				t.Errorf("%q declares display without :not([hidden])", sel)
			}
		}
	}
}
