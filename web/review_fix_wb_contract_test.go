package web

import (
	"strings"
	"testing"
)

// This file pins the Phase 4 web review fixes for the report form, the store
// and the Low data view (04-REVIEW-WEB.md WR-W03, WR-W04 and WR-W08).
//
// Honest limit: these tests read the shipped scripts as text. The behaviour of
// the store's notify is run for real by web/notify_isolation_test.mjs; a real
// browser pass checks the form and the location block.

// TestReviewFixWBListenersIsolated pins WR-W04: one store listener that throws
// cannot stop the others, and the Low data row is drawn without the Flag
// control when flag.js did not load.
func TestReviewFixWBListenersIsolated(t *testing.T) {
	app := p424ReadJS(t, "static/js/app.js")
	notify := jsFunctionBody(t, app, "notify()")
	p424Ordered(t, "notify", notify, "for (", "try {", "listeners[i](state);", "} catch (e) {", "rethrowLater(e);")
	p424Contains(t, "rethrowLater", jsFunctionBody(t, app, "rethrowLater(e)"), "window.setTimeout(", "throw e;")

	low := p424ReadJS(t, "static/js/low.js")
	create := jsFunctionBody(t, low, "createLowRow(report)")
	p424Ordered(t, "createLowRow", create,
		"var flagControl = window.PinalertFlag ? PinalertFlag.createFlagControl(report) : null;",
		"if (flagControl) {", "body.appendChild(flagControl.el);")
	update := jsFunctionBody(t, low, "updateLowRow(row, report)")
	p424Ordered(t, "updateLowRow", update,
		"if (row.flag && window.PinalertFlag) {", "PinalertFlag.updateFlagControl(row.flag, report);")
	p424NoDashes(t, "low.js", p424ReadRaw(t, "static/js/low.js"))
	if strings.Count(low, "PinalertFlag.") != 2 {
		t.Errorf("low.js must reach PinalertFlag only in the two guarded calls, found %d uses", strings.Count(low, "PinalertFlag."))
	}
}

// TestReviewFixWBLatePostLeavesNewForm pins WR-W03: every post keeps the form
// opening it was sent under, an answer for a form that has since closed never
// reaches the open form, and the outcome of that earlier post is told in plain
// words. web/report_form_session_test.mjs runs the real script through each
// outcome.
func TestReviewFixWBLatePostLeavesNewForm(t *testing.T) {
	raw := p424ReadRaw(t, "static/js/report-form.js")
	form := p424ReadJS(t, "static/js/report-form.js")
	p424NoDashes(t, "report-form.js comments and copy", strings.ReplaceAll(raw, "Posting"+string(rune(0x2026)), ""))

	for _, c := range []struct{ name, literal string }{
		{"EARLIER_POSTED_TOAST", `'Your earlier report was posted.'`},
		{"EARLIER_HELD_TOAST", `"Your earlier report was received. It isn't shown yet. A text check has to finish first."`},
		{"EARLIER_SAVE_TOAST", `"No signal. Your earlier report is saved on this device and will send when you're back online."`},
		{"EARLIER_SAVE_TOAST_MEMORY", `'No signal. Your earlier report is kept on this page only. Keep the page open until it sends.'`},
		{"EARLIER_NOT_SAVED_TOAST", `'Your earlier report could not be saved on this device, so it was not sent.'`},
		{"EARLIER_REFUSED_PREFIX", `'Your earlier report was not posted. '`},
		{"EARLIER_REFUSED_FALLBACK", `'Something went wrong.'`},
	} {
		if n := strings.Count(form, "var "+c.name+" = "+c.literal+";"); n != 1 {
			t.Errorf("%s is declared %d times with the exact copy, want 1", c.name, n)
		}
		p422PlainCopy(t, c.literal[1:len(c.literal)-1])
	}

	if n := strings.Count(form, "formSession += 1;"); n != 1 {
		t.Errorf("the form opening number must be raised in one place, found %d", n)
	}
	p424Ordered(t, "closeForm", jsFunctionBody(t, form, "closeForm()"), "formSession += 1;", "resetForm();")
	p424Ordered(t, "resetForm", jsFunctionBody(t, form, "resetForm()"),
		"submitButton.disabled = false;", "Pinalert.setText(submitButton, 'Post report');")

	submit := jsFunctionBody(t, form, "handleSubmit()")
	p424Ordered(t, "handleSubmit", submit,
		"session: formSession,", "resendRequestId: resendRequestId,", "writtenAt: resendWrittenAt || firstSubmitAt",
		"Pinalert.submitReport(payload).then(",
		"PinalertQueue.discard(sent.resendRequestId)",
		"if (sent.session !== formSession) {", "EARLIER_HELD_TOAST : EARLIER_POSTED_TOAST", "return;",
		"submitButton.disabled = false;", "closeForm();",
		".catch(function (err) {",
		"if (sent.session !== formSession) {", "reportEarlierFailure(payload, err, sent);", "return;",
		"submitButton.disabled = false;",
		"saveToQueue(payload, err.status >= 502 ? 'no_answer' : 'waiting', sent);")
	p424Lacks(t, "handleSubmit", submit, "discard(resendRequestId)")

	queued := jsFunctionBody(t, form, "saveToQueue(payload, status, sent)")
	p424Contains(t, "saveToQueue", queued, "request_id: payload.request_id,", "var writtenAt = sent.writtenAt;",
		"EARLIER_NOT_SAVED_TOAST", "EARLIER_SAVE_TOAST : EARLIER_SAVE_TOAST_MEMORY")
	p424Lacks(t, "saveToQueue", queued, "currentRequestId", "firstSubmitAt", "resendWrittenAt")
	p424Ordered(t, "saveToQueue", queued, "if (sent.session !== formSession) {", "return;", "closeForm();")

	earlier := jsFunctionBody(t, form, "reportEarlierFailure(payload, err, sent)")
	p424Contains(t, "reportEarlierFailure", earlier,
		"err.queueable === true", "saveToQueue(", "EARLIER_REFUSED_PREFIX", "EARLIER_REFUSED_FALLBACK",
		"err.serious === true", "EMERGENCY_CALL_PREFIX + err.emergencyNumber + EMERGENCY_CALL_SUFFIX",
		"Pinalert.showToast(text, null, LONG_TOAST_MS)")
	p424Lacks(t, "reportEarlierFailure", earlier, "formError", "focusField(", "submitButton", "closeForm(", "showLimitMessage(")

	wait := jsFunctionBody(t, form, "saveAfterWait()")
	p424Ordered(t, "saveAfterWait", wait, "var session = formSession;", "if (session === formSession) {", "closeForm();",
		"if (session !== formSession) {", "EARLIER_NOT_SAVED_TOAST")
}

// TestReviewFixWBLowFormLocationRetry pins WR-W08: the Low data form asks for a
// high accuracy fix first and, when that times out or the position is
// unavailable, asks once more at the default accuracy, as votes.js does. A
// refusal ends the request at once, and a request that a close or a Retry made
// old starts no second attempt.
func TestReviewFixWBLowFormLocationRetry(t *testing.T) {
	low := p424ReadJS(t, "static/js/low.js")
	request := jsFunctionBody(t, low, "requestFix()")
	p424Ordered(t, "requestFix", request,
		"positionSeq += 1;",
		"var seq = positionSeq;",
		"function attempt(highAccuracy) {",
		"navigator.geolocation.getCurrentPosition(",
		"onFix(seq, pos);",
		"if (highAccuracy && seq === positionSeq && !(err && err.code === err.PERMISSION_DENIED)) {",
		"attempt(false);",
		"return;",
		"onFail(seq, err);",
		"{ enableHighAccuracy: true, timeout: LOCATION_TIMEOUT_MS, maximumAge: 0 }",
		"{ enableHighAccuracy: false, timeout: LOCATION_TIMEOUT_MS, maximumAge: 0 }",
		"attempt(true);")
	for needle, want := range map[string]int{
		"attempt(false)":      1,
		"attempt(true)":       1,
		"getCurrentPosition(": 1,
	} {
		if n := strings.Count(request, needle); n != want {
			t.Errorf("requestFix holds %q %d times, want %d", needle, n, want)
		}
	}
	// The list's own position request keeps its single default accuracy ask.
	if n := strings.Count(low, "getCurrentPosition("); n != 2 {
		t.Errorf("low.js asks for a position from %d places, want 2 (the list and the form)", n)
	}
}
