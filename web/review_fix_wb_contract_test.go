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
