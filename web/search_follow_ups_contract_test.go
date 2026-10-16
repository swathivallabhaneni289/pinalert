package web

// Quick task 261016-sfu contract tests: the report form searches after a
// 300 ms pause, and the map's place search suggests while the reader types,
// after the same pause. These read the embedded scripts; the behaviour of the
// typing path is run for real in place_search_typing_test.mjs.

import (
	"strings"
	"testing"
)

// TestReportSearchPauseIs300 pins the report form's pause before it searches.
func TestReportSearchPauseIs300(t *testing.T) {
	form := p19Code(t, "static/js/report-form.js")
	p13MustContain(t, "report-form.js", form, "var SEARCH_DEBOUNCE_MS = 300;")
	if n := strings.Count(form, "var SEARCH_DEBOUNCE_MS ="); n != 1 {
		t.Errorf("report-form.js defines SEARCH_DEBOUNCE_MS %d times, want 1", n)
	}
	input := jsFunctionBody(t, form, "onSearchInput()")
	p13MustContain(t, "onSearchInput", input, "window.clearTimeout(searchDebounceTimer)", "SEARCH_DEBOUNCE_MS")
}

// TestPlaceSearchSuggestsWhileTyping pins the typing path of the map's place
// search: one input listener, a 300 ms pause, at least 3 letters, the same
// search marked as typed, and every dismissal and pick ending a pending pause.
func TestPlaceSearchSuggestsWhileTyping(t *testing.T) {
	code := p31mTools(t)

	p13MustContain(t, "map-tools.js", code,
		"var TYPING_PAUSE_MS = 300;",
		"inputEl.addEventListener('input', onInput);")
	if n := strings.Count(code, "addEventListener('input'"); n != 1 {
		t.Errorf("map-tools.js has %d input listeners, want 1", n)
	}
	// The one timer is the typing pause.
	if n := strings.Count(code, "setTimeout("); n != 1 {
		t.Errorf("map-tools.js has %d setTimeout( calls, want 1 (the typing pause)", n)
	}

	onInput := jsFunctionBody(t, code, "onInput()")
	p13Ordered(t, onInput, "onInput",
		"cancelTyping();", "Array.from(q).length < MIN_QUERY_RUNES", "window.setTimeout(", "search(q, true);", "TYPING_PAUSE_MS")
	// Under 3 letters while typing closes quietly: no "Type at least 3 letters."
	p13MustNotContain(t, "onInput", onInput, "MESSAGE_TOO_SHORT", "showStatus(")

	cancel := jsFunctionBody(t, code, "cancelTyping()")
	p13MustContain(t, "cancelTyping", cancel, "window.clearTimeout(typingTimer)", "typingTimer = null")

	// Every way a search ends also ends a pending pause, so a late timer
	// never reopens the list after Escape, Close, a press outside or a pick.
	abort := jsFunctionBody(t, code, "abortSearch()")
	p13MustContain(t, "abortSearch", abort, "cancelTyping();", "controller.abort()")
	pick := jsFunctionBody(t, code, "pickResult(i)")
	p13Ordered(t, pick, "pickResult", "abortSearch();", "Pinalert.setCenter(pick.lat, pick.lon, false);")

	// The typed search is quiet: it never shows Searching, the too short line
	// or a failure line; Enter and the button show all three. Every such line
	// sits right inside an if (!quiet...) or if (!searchQuiet) guard.
	search := jsFunctionBody(t, code, "search(query, typed)")
	p13MustContain(t, "search", search, "var quiet = typed === true;", "cancelTyping();", "searchQuiet = quiet;")
	for _, loud := range []string{"showStatus(MESSAGE_SEARCHING, false);", "showStatus(messageFor(res.status, body), false);", "showStatus(MESSAGE_UNAVAILABLE, false);", "showStatus(MESSAGE_TOO_SHORT, false);"} {
		rest, seen := search, 0
		for {
			at := strings.Index(rest, loud)
			if at == -1 {
				break
			}
			seen++
			guard := strings.LastIndex(rest[:at], "if (!")
			cond := ""
			if guard != -1 {
				cond = rest[guard:at]
				if end := strings.Index(cond, "{"); end != -1 {
					cond = cond[:end]
				}
			}
			if guard == -1 || !strings.Contains(cond, "quiet") && !strings.Contains(cond, "Quiet") || strings.Contains(rest[guard:at], "}") {
				t.Errorf("search shows %q without a quiet guard right around it", loud)
			}
			rest = rest[at+len(loud):]
		}
		if seen == 0 {
			t.Errorf("search lacks %q", loud)
		}
	}
	// Enter and the button keep their path.
	p13MustContain(t, "map-tools.js", code, "evt.preventDefault();\n      search();")
}
