package web

// Quick task 261016-adr contract tests: both searches show one plain line
// above the results when a shorter text found them, written as text and
// never as markup, and the report form's search sends the pin. Static
// inspection of the embedded bytes like the other contract files; the width
// pass at 320, 360 and 390 px is a browser check.

import (
	"strings"
	"testing"
)

// partialLinePrefix is the new line, before the matched text and its full
// stop. Both scripts carry it byte for byte.
const partialLinePrefix = "No exact match for the full address. Showing places for: "

func TestPartialMatchLineInBothSearches(t *testing.T) {
	tools := p19Code(t, "static/js/map-tools.js")
	form := p19Code(t, "static/js/report-form.js")

	p13MustContain(t, "map-tools.js", tools, "var MESSAGE_PARTIAL_PREFIX = '"+partialLinePrefix+"';")
	p13MustContain(t, "report-form.js", form, "var SEARCH_PARTIAL_PREFIX = '"+partialLinePrefix+"';")

	// Place search: the line is a paragraph at the top of the results, set
	// through the text sink, built before the first result button, and the
	// answer's matched_query is what feeds it.
	render := jsFunctionBody(t, tools, "renderResults(list, matched)")
	p13Ordered(t, render, "renderResults",
		"resultsEl.textContent = '';",
		"document.createElement('p')",
		"'map-search__note'",
		"Pinalert.setText(note, partial)",
		"resultsEl.appendChild(note)",
		"document.createElement('button')")
	p13MustContain(t, "renderResults", render, "MESSAGE_PARTIAL_PREFIX + matched + '.'")
	p13MustContain(t, "search", jsFunctionBody(t, tools, "search(query, typed)"), "body.matched_query")

	// Report form: the same, inside the suggestion list, and the cache keeps
	// the matched text so a repeat search shows the line again.
	dropdown := jsFunctionBody(t, form, "renderDropdown(results, matched)")
	p13Ordered(t, dropdown, "renderDropdown",
		"locationSearchResults.textContent = '';",
		"document.createElement('p')",
		"'location-search-note'",
		"Pinalert.setText(note, SEARCH_PARTIAL_PREFIX + matched + '.')",
		"locationSearchResults.appendChild(note)",
		"renderResultRow(result)")
	run := jsFunctionBody(t, form, "runSearch(query)")
	p13MustContain(t, "runSearch", run,
		"body.matched_query", "matched: body.matched_query", "renderDropdown(hit.results, hit.matched)")

	// Written as text, never as markup: no parsing sink in either script, and
	// the matched text never meets a textContent assignment of its own.
	for name, code := range map[string]string{"map-tools.js": tools, "report-form.js": form} {
		p13MustNotContain(t, name, code,
			"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "createContextualFragment",
			"textContent = matched", "textContent = partial", "textContent = SEARCH_PARTIAL_PREFIX")
	}
}

func TestReportSearchSendsThePin(t *testing.T) {
	form := p19Code(t, "static/js/report-form.js")

	pin := jsFunctionBody(t, form, "searchPin()")
	p13MustContain(t, "searchPin", pin,
		"picker.current()", "return null",
		"pos.latitude < -90", "pos.latitude > 90", "pos.longitude < -180", "pos.longitude > 180")

	run := jsFunctionBody(t, form, "runSearch(query)")
	p13Ordered(t, run, "runSearch",
		"'/api/geocode?q=' + encodeURIComponent(query)",
		"searchPin()",
		"'&lat='", "'&lon='",
		"searchCache.has(cacheKey)",
		"var limit = searchTimeLimit();",
		"Promise.race([fetch(url, { signal: limit.signal }), limit.expired])",
		"Promise.race([res.json(), limit.expired]).catch(function () {")
	// The cache key is the whole request, pin included, so an answer for one
	// area is never shown for another.
	p13MustContain(t, "runSearch", run, "var cacheKey = url.toLowerCase();")

	// A new search ends the request before it, so an old cascade stops using
	// the server's one request a second budget. The limit is the shared one of
	// app.js, raced against the request, so a search whose aborted request
	// never ends (WebKit) still fails at its limit
	// (web/report_search_retry_test.mjs runs it).
	p13MustContain(t, "searchTimeLimit", jsFunctionBody(t, form, "searchTimeLimit()"),
		"abortSearchRequest()", "Pinalert.requestTimeout(SEARCH_TIMEOUT_MS, ", "searchController = limit.controller")
	p13MustContain(t, "abortSearchRequest", jsFunctionBody(t, form, "abortSearchRequest()"),
		"searchController.abort()", "searchController = null")
	p13MustContain(t, "resetSearch", jsFunctionBody(t, form, "resetSearch()"), "abortSearchRequest()")
}

func TestPartialMatchLineStyles(t *testing.T) {
	feed := p13ReadStatic(t, "static/css/feed.css")
	modal := p13ReadStatic(t, "static/css/modal.css")
	for name, pair := range map[string][2]string{
		"feed.css":  {feed, ".map-search__note {"},
		"modal.css": {modal, ".location-search-note {"},
	} {
		css, head := pair[0], pair[1]
		at := strings.Index(css, head)
		if at < 0 {
			t.Errorf("%s has no %q rule", name, head)
			continue
		}
		body := css[at:]
		body = body[:strings.Index(body, "}")]
		p13MustContain(t, name+" "+head, body,
			"font-size: var(--font-size-label)", "color: var(--color-text-muted)", "margin: 0", "padding: var(--space-sm)")
	}
}
