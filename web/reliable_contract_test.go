package web

// Phase 3.2 plan 14 contract tests: the Only Reliable reporters tick of the
// Show menu on Nearby (NOTIF-04, D-26, 03.4 D-13, TRUST-02). Like the other
// contract files these are static inspection of the embedded bytes; nothing
// here needs a database or a server. Honest limit: static inspection proves
// the words, the order, the single writer of the address word and the absence
// of any client side trust rule. It cannot prove how a browser draws the menu;
// web/reliable_test.mjs and web/show_menu_test.mjs run the real scripts
// against fakes for the behaviour, and the owner's look at the finished build
// (the Show menu with the tick at 320 and 1280, the line and the empty state
// on a deployment whose Reliable set is empty) is the layout check.

import (
	"regexp"
	"strings"
	"testing"
)

// TestReliableScriptContract pins reliable.js: the one owner of the tick's
// state change and of its address word (T-03.2-17, T-03.2-08).
func TestReliableScriptContract(t *testing.T) {
	raw := p424ReadRaw(t, "static/js/reliable.js")
	code := p424ReadJS(t, "static/js/reliable.js")

	if n := strings.Count(code, "window.PinalertReliable = "); n != 1 {
		t.Errorf("reliable.js assigns window.PinalertReliable %d times, want 1", n)
	}
	p424Contains(t, "reliable.js", code,
		"readFromAddress: readFromAddress", "set: set", "toggle: toggle", "renderLine: renderLine")

	// The address word is compared exactly, never interpolated.
	read := jsFunctionBody(t, code, "readFromAddress(search)")
	p424Contains(t, "readFromAddress", read, "params.get('reliable') === '1'")
	p424Lacks(t, "readFromAddress", read, "parseInt", "Number(", "toLowerCase", "indexOf", "test(")

	// One replace, never a push, and only the reliable key is touched.
	if n := strings.Count(code, "replaceState("); n != 1 {
		t.Errorf("reliable.js calls replaceState %d times, want 1", n)
	}
	p424Lacks(t, "reliable.js", code, "pushState", "location.assign", "location.replace(", "location.href", "localStorage", "sessionStorage", "fetch(")
	keyCall := regexp.MustCompile(`\.(set|delete)\('([^']*)'`)
	keyCalls := keyCall.FindAllStringSubmatch(code, -1)
	if len(keyCalls) != 2 {
		t.Errorf("reliable.js sets or deletes %d parameters, want 2 (set and delete of reliable)", len(keyCalls))
	}
	for _, m := range keyCalls {
		if m[2] != "reliable" {
			t.Errorf("reliable.js %ss the parameter %q, want only reliable", m[1], m[2])
		}
	}
	write := jsFunctionBody(t, code, "writeAddress(on)")
	p424Contains(t, "writeAddress", write, "window.history.state", "window.location.hash")

	// The change order: store, drop the link, address, line, redraw. No request.
	setBody := jsFunctionBody(t, code, "set(on)")
	p424Ordered(t, "set", setBody,
		"Pinalert.setReliable(", "Pinalert.dropFocus()", "writeAddress(", "renderLine()", "Pinalert.redraw()")
	p424Lacks(t, "set", setBody, "fetchReports")

	// Text goes through the text sink; the page is built with createElement.
	p424Contains(t, "reliable.js", code, "Pinalert.setText(", "document.createElement('span')", "document.createElement('button')", "'inline-text-button'")
	p424Lacks(t, "reliable.js", code, "innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(")

	// The three words of the line and its button (P32-C18), once each.
	for _, want := range []string{
		"'Showing Reliable reporters and critical reports.'",
		"'Show everyone'",
	} {
		if n := strings.Count(code, want); n != 1 {
			t.Errorf("reliable.js holds %s %d times, want 1", want, n)
		}
	}

	// The ends of a press hand focus back to the Show button.
	p424Contains(t, "reliable.js", code, "getElementById('show-button')", "getElementById('reliable-line')", "getElementById('reliable-empty-clear')")

	if !strings.Contains(raw, "window.PinalertReliable = (function () {") {
		t.Errorf("reliable.js must open window.PinalertReliable = (function () {")
	}
	p424NoDashes(t, "reliable.js", raw)
}

// TestReliableFilterIsASibling pins that the tick is a sibling of reportInShow,
// not an edit of it, and that it reads one server word and nothing else
// (TRUST-02, T-03.2-26). reportInShow itself is pinned by
// TestNoClientTrustThreshold.
func TestReliableFilterIsASibling(t *testing.T) {
	app := p19Code(t, "static/js/app.js")

	kept := jsFunctionBody(t, app, "reportKeptByReliable(report, on)")
	p13MustContain(t, "reportKeptByReliable", kept, "report.kept_by_reliable_filter === true")
	p13MustNotContain(t, "reportKeptByReliable", kept,
		"trust", "severity", "visibility", "tag", "category", "confirmed", "disputed", "_places", "reliable_reporter", ".length", "Date")
	number := regexp.MustCompile(`[<>]=?\s*[0-9]|[=!]==?\s*[0-9]|[0-9]\s*[<>=!]`)
	if m := number.FindString(kept); m != "" {
		t.Errorf("reportKeptByReliable compares with a number (%q): the browser holds no rule", m)
	}

	// visibleReports asks both functions, and the sibling is not inside the
	// Show function.
	visible := jsFunctionBody(t, app, "visibleReports()")
	p13MustContain(t, "visibleReports", visible,
		"reportInShow(state.reports[i], state.show)", "reportKeptByReliable(state.reports[i], state.reliable)")
	member := jsFunctionBody(t, app, "reportInShow(report, show)")
	p13MustNotContain(t, "reportInShow", member, "reliable", "kept_by_reliable_filter")

	// The store word and its setter exist, the setter only assigns.
	set := jsFunctionBody(t, app, "setReliable(on)")
	p13MustContain(t, "setReliable", set, "state.reliable = ")
	p13MustNotContain(t, "setReliable", set, "notify(", "fetch", "redraw(")
	p13MustContain(t, "app.js", app, "reliable: false,", "setReliable: setReliable,", "reportKeptByReliable: reportKeptByReliable,")

	// Counts other tests pin: the one Show Confirmed server field and the two
	// requests of app.js stay exactly as they were.
	if n := strings.Count(app, "confirmed_by_people"); n != 1 {
		t.Errorf("app.js names confirmed_by_people %d times, want 1", n)
	}
	if n := strings.Count(app, "fetch("); n != 2 {
		t.Errorf("app.js makes %d fetch( calls, want 2", n)
	}
	if n := strings.Count(app, "kept_by_reliable_filter"); n != 1 {
		t.Errorf("app.js names kept_by_reliable_filter %d times, want 1 (the one rule)", n)
	}
	// The tick's own functions carry no em dash, en dash or ellipsis character.
	p424NoDashes(t, "reportKeptByReliable", kept)
	p424NoDashes(t, "setReliable", set)
}
