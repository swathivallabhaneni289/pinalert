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
	// The two timers are the typing pause and the quiet retry (quick
	// 261016-sne), and both use typingTimer, so every way of ending a pause
	// also ends a pending retry.
	if n := strings.Count(code, "setTimeout("); n != 2 {
		t.Errorf("map-tools.js has %d setTimeout( calls, want 2 (the typing pause and the quiet retry)", n)
	}

	onInput := jsFunctionBody(t, code, "onInput()")
	p13Ordered(t, onInput, "onInput",
		"cancelTyping();", "Array.from(q).length < MIN_QUERY_RUNES", "window.setTimeout(", "search(q, true);", "TYPING_PAUSE_MS")
	// Under 3 letters while typing closes quietly: no "Type at least 3 letters."
	p13MustNotContain(t, "onInput", onInput, "MESSAGE_TOO_SHORT", "showStatus(")

	cancel := jsFunctionBody(t, code, "cancelTyping()")
	p13MustContain(t, "cancelTyping", cancel, "window.clearTimeout(typingTimer)", "typingTimer = null")

	// Every way a search ends also ends a pending pause, so a late timer
	// never reopens the list after Escape, a press outside or a pick. (Amended
	// 2026-10-16 by Phase 3.4, D-11: the Close button is gone with the
	// collapsed state. The Phase 3.1 and Phase 4 hands-on tests are walked on
	// this layout after Phase 3.4 is built.)
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

// TestSearchQuietRetry pins the quiet retry of both searches (quick
// 261016-sne): at most 3 tries again, after the answer's Retry-After or 1
// second, only for a 429 or a 503, and the wait held on the timer that newer
// typing already clears. The behaviour is run for real in
// report_search_retry_test.mjs and place_search_typing_test.mjs.
func TestSearchQuietRetry(t *testing.T) {
	form := p19Code(t, "static/js/report-form.js")
	p13MustContain(t, "report-form.js", form,
		"var SEARCH_QUIET_RETRIES = 3;",
		"var SEARCH_RETRY_MS = 1000;")
	// One fetch in the file: a retry goes back through runSearch.
	if n := strings.Count(form, "fetch("); n != 1 {
		t.Errorf("report-form.js has %d fetch( calls, want 1", n)
	}
	run := jsFunctionBody(t, form, "runSearch(query)")
	p13MustContain(t, "runSearch", run,
		"err.status === 429 || err.status === 503",
		"searchRetries < SEARCH_QUIET_RETRIES",
		"searchDebounceTimer = window.setTimeout(",
		"runSearch(query);")
	p13MustContain(t, "onSearchInput", jsFunctionBody(t, form, "onSearchInput()"), "searchRetries = 0;")
	p13MustContain(t, "resetSearch", jsFunctionBody(t, form, "resetSearch()"), "searchRetries = 0;")

	tools := p31mTools(t)
	p13MustContain(t, "map-tools.js", tools,
		"var QUIET_RETRIES = 3;",
		"var RETRY_WAIT_MS = 1000;")
	retry := jsFunctionBody(t, tools, "retryLater(q, wait)")
	p13MustContain(t, "retryLater", retry, "cancelTyping();", "typingTimer = window.setTimeout(", "search(q, true);")
	search := jsFunctionBody(t, tools, "search(query, typed)")
	p13MustContain(t, "search", search,
		"res.status === 429 || res.status === 503",
		"quietFailures < QUIET_RETRIES",
		"retryLater(q, retryWait(res));")
	p13MustContain(t, "onInput", jsFunctionBody(t, tools, "onInput()"), "quietFailures = 0;")
}
