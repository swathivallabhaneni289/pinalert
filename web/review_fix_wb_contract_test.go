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
		{"EARLIER_NOT_SAVED_TOAST", `"Your earlier report couldn't be saved on this device, so it wasn't sent."`},
		{"EARLIER_REFUSED_PREFIX", `"Your earlier report wasn't posted. "`},
		{"EARLIER_REFUSED_FALLBACK", `'Something went wrong.'`},
		{"EARLIER_WAIT_SAVED_PREFIX", `"Your earlier report has to wait before it can be posted. It's saved on this device and will send by itself in "`},
		{"EARLIER_WAIT_MEMORY_PREFIX", `"Your earlier report has to wait before it can be posted. It's kept on this page only. Keep the page open and it'll send by itself in "`},
		{"EARLIER_REFUSED_SAVED", `"It's saved on this device under Waiting to send, so you can edit it and send it again."`},
		{"EARLIER_REFUSED_MEMORY", `"It's kept on this page only, under Waiting to send. Keep the page open to edit it and send it again."`},
		{"EARLIER_REFUSED_NOT_SAVED", `"It couldn't be saved on this device."`},
		{"EARLIER_SIGNED_OUT_SAVED", `"Your earlier report wasn't posted because you're signed out. It's saved on this device. Sign in again to send it."`},
		{"EARLIER_SIGNED_OUT_MEMORY", `"Your earlier report wasn't posted because you're signed out. It's kept on this page only. Sign in again in another tab, then tap Send now on this page."`},
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
		// Post stays off while a report with no answer is being saved (WR-06).
		"saveToQueue(payload, err.status >= 502 ? 'no_answer' : 'waiting', sent);", "return;",
		"submitButton.disabled = false;")
	p424Lacks(t, "handleSubmit", submit, "discard(resendRequestId)")

	queued := jsFunctionBody(t, form, "saveToQueue(payload, status, sent)")
	p424Contains(t, "saveToQueue", queued, "request_id: payload.request_id,", "var writtenAt = sent.writtenAt;",
		"EARLIER_NOT_SAVED_TOAST", "EARLIER_SAVE_TOAST : EARLIER_SAVE_TOAST_MEMORY")
	p424Lacks(t, "saveToQueue", queued, "currentRequestId", "firstSubmitAt", "resendWrittenAt")
	p424Ordered(t, "saveToQueue", queued, "if (sent.session !== formSession) {", "return;", "closeForm();")

	// CR-01, WR-01: a refused earlier report is saved on the device with the
	// status its answer calls for, and the toast says where it is in the
	// form's own words, never with the server's 429 sentence.
	earlier := jsFunctionBody(t, form, "reportEarlierFailure(payload, err, sent)")
	p424Contains(t, "reportEarlierFailure", earlier,
		"err.queueable === true", "saveToQueue(", "earlierRecord(payload, err, sent)", "window.PinalertQueue.save(record)",
		"earlierToastText(record, err, persisted)",
		"err.serious === true", "EMERGENCY_CALL_PREFIX + err.emergencyNumber + EMERGENCY_CALL_SUFFIX",
		"action = callAction(err.emergencyNumber);", "Pinalert.showToast(text, action, LONG_TOAST_MS)")
	p424Lacks(t, "reportEarlierFailure", earlier, "formError", "focusField(", "submitButton", "closeForm(", "showLimitMessage(")
	record := jsFunctionBody(t, form, "earlierRecord(payload, err, sent)")
	p424Ordered(t, "earlierRecord", record,
		"payload.written_at = sent.writtenAt;", "request_id: payload.request_id,", "written_at: sent.writtenAt",
		"err.status === 429", "record.status = 'blocked';", "Math.max(MIN_WAIT_SECONDS, seconds)",
		"record.serious = err.serious === true;", "record.emergency_number = err.emergencyNumber || '';",
		"err.status === 401", "record.status = 'signed_out';",
		"record.status = 'rejected';", "record.last_error = err.fieldMessage || 'Something went wrong. Try again.';")
	p424Lacks(t, "earlierRecord", record, "currentRequestId", "firstSubmitAt", "resendWrittenAt")
	toast := jsFunctionBody(t, form, "earlierToastText(record, err, persisted)")
	p424Contains(t, "earlierToastText", toast, "EARLIER_NOT_SAVED_TOAST", "EARLIER_WAIT_SAVED_PREFIX : EARLIER_WAIT_MEMORY_PREFIX",
		"Pinalert.formatWait(", "EARLIER_SIGNED_OUT_SAVED : EARLIER_SIGNED_OUT_MEMORY", "EARLIER_REFUSED_NOT_SAVED",
		"EARLIER_REFUSED_SAVED : EARLIER_REFUSED_MEMORY", "EARLIER_REFUSED_PREFIX", "EARLIER_REFUSED_FALLBACK")
	// Only the non 429 refusals ever show the server's reason.
	p424Ordered(t, "earlierToastText", toast, "record.status === 'blocked'", "return (persisted", "err.fieldMessage")
	// The serious toast's Call button dials the number the way the form's tap to
	// call link does, under the same accessible name.
	p424Contains(t, "callAction", jsFunctionBody(t, form, "callAction(number)"),
		"label: CALL_LABEL_PREFIX + number,", "ariaLabel: CALL_LABEL_PREFIX + number,",
		"window.location.href = 'tel:' + number;")
	if n := strings.Count(form, "var CALL_LABEL_PREFIX = 'Call ';"); n != 1 {
		t.Errorf("CALL_LABEL_PREFIX is declared %d times as 'Call ', want 1", n)
	}

	wait := jsFunctionBody(t, form, "saveAfterWait()")
	p424Ordered(t, "saveAfterWait", wait, "var session = formSession;", "if (session === formSession) {", "closeForm();",
		"if (session !== formSession) {", "EARLIER_NOT_SAVED_TOAST")
}

// TestReviewFixWBCloseWhilePosting pins the honest close: while a post is out
// the form closes without the Discard question, because that report is already
// on its way and a toast tells how it ends. A refused report is back in the
// form, so closing asks again. web/report_form_session_test.mjs runs each case.
func TestReviewFixWBCloseWhilePosting(t *testing.T) {
	form := p424ReadJS(t, "static/js/report-form.js")
	p424Ordered(t, "requestClose", jsFunctionBody(t, form, "requestClose()"),
		"if (!discardConfirm.hidden) {", "return;", "if (postInFlight) {", "closeForm();", "} else if (formTouched) {",
		"discardConfirm.hidden = false;")
	p424Ordered(t, "handleSubmit", jsFunctionBody(t, form, "handleSubmit()"),
		"submitButton.disabled = true;", "postInFlight = true;", "discardConfirm.hidden = true;",
		"Pinalert.submitReport(payload).then(",
		".catch(function (err) {", "if (sent.session !== formSession) {", "return;",
		"saveToQueue(", "postInFlight = false;", "showLimitMessage(err);", "postInFlight = false;")
	p424Contains(t, "resetForm", jsFunctionBody(t, form, "resetForm()"), "postInFlight = false;")
	p424Ordered(t, "saveToQueue", jsFunctionBody(t, form, "saveToQueue(payload, status, sent)"),
		"if (sent.session !== formSession) {", "return;", "postInFlight = false;")
	p424Ordered(t, "saveAfterWait", jsFunctionBody(t, form, "saveAfterWait()"),
		"postInFlight = true;", "submitButton.disabled = true;", "sendAfterWaitBtn.disabled = true;",
		"discardConfirm.hidden = true;", "window.PinalertQueue.save(",
		"if (session !== formSession) {", "return;", "postInFlight = false;",
		"submitButton.disabled = false;", "sendAfterWaitBtn.disabled = false;")
	// WR-06: a report on its way or being saved is never posted again, and a
	// failed save gives Post back.
	p424Ordered(t, "handleSubmit", jsFunctionBody(t, form, "handleSubmit()"), "if (postInFlight) {", "return;", "validate()")
	p424Ordered(t, "saveToQueue", jsFunctionBody(t, form, "saveToQueue(payload, status, sent)"),
		"postInFlight = false;", "submitButton.disabled = false;")
	p424Contains(t, "resetForm", jsFunctionBody(t, form, "resetForm()"), "sendAfterWaitBtn.disabled = false;")
}

// TestReviewFixWBResendNamesSavedAccount pins WR-02 of the skeptic check: the
// form's Edit and resend of a saved row names the account the row was saved
// under, as a queue send does, and a report typed now names none. The saved
// payload never keeps it. web/report_form_session_test.mjs runs each case.
func TestReviewFixWBResendNamesSavedAccount(t *testing.T) {
	form := p424ReadJS(t, "static/js/report-form.js")
	p424Ordered(t, "openForm", jsFunctionBody(t, form, "openForm(options)"),
		"resendRequestId = options.requestId;", "/^[0-9]+$/.test(options.accountId)", "resendAccountId = options.accountId;")
	p424Ordered(t, "handleSubmit", jsFunctionBody(t, form, "handleSubmit()"),
		"if (resendAccountId) {", "payload.expected_account_id = resendAccountId;", "Pinalert.submitReport(payload).then(")
	p424Contains(t, "resetForm", jsFunctionBody(t, form, "resetForm()"), "resendAccountId = null;")
	p424Contains(t, "saveToQueue", jsFunctionBody(t, form, "saveToQueue(payload, status, sent)"), "delete payload.expected_account_id;")
	p424Contains(t, "earlierRecord", jsFunctionBody(t, form, "earlierRecord(payload, err, sent)"), "delete payload.expected_account_id;")
	p424Contains(t, "queue save", jsFunctionBody(t, p424ReadJS(t, "static/js/queue.js"), "save(record)"),
		"delete payload.expected_account_id;")
	for _, page := range []string{"static/js/feed.js", "static/js/low.js"} {
		p424Contains(t, page+" openSavedReport", jsFunctionBody(t, p424ReadJS(t, page), "openSavedReport(record, opener, note)"),
			"requestId: record.request_id,", "accountId: record.account_id")
	}
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
