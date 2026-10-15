package web

import (
	"io/fs"
	"strings"
	"testing"
)

// p413ReadTemplate returns an embedded template file as written.
func p413ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("reading embedded template %s: %v", name, err)
	}
	return string(raw)
}

// This file pins plan 04-13: the main page keeps its list through a failed or
// stalled poll (D-20, P4-A19), the toast takes an optional duration and a root
// flag (P4-A12), and the stale line announces itself through one live region.
//
// Honest limit of these tests' claim: they inspect source text. They cannot
// prove that a browser abandons a stalled request after 10 seconds, that a
// screen reader speaks an announcement, or that rows stay on screen. That is
// the job of the Node sandbox runs recorded in the plan summary and of the
// human check in plan 04-27.

// p413ReadStatic returns an embedded static file with its comments removed
// (JavaScript through the shared scanner, CSS through the block comment
// stripper).
func p413ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := StaticFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", path, err)
	}
	if strings.HasSuffix(path, ".css") {
		return stripCSSComments(string(raw))
	}
	return stripJSComments(string(raw))
}

// p413MustContain fails once per missing needle.
func p413MustContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s must contain %q", label, n)
		}
	}
}

// p413MustNotContain fails once per needle that is present.
func p413MustNotContain(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(text, n) {
			t.Errorf("%s must not contain %q", label, n)
		}
	}
}

// p413Ordered requires the needles to appear in this order, each after the
// one before it.
func p413Ordered(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	from := 0
	for _, n := range needles {
		idx := strings.Index(text[from:], n)
		if idx == -1 {
			t.Errorf("%s must hold %q after the previous needle", label, n)
			return
		}
		from += idx + len(n)
	}
}

// TestFetchReportsBackgroundRefreshAndTimeout pins P4-A19 and rules C1 to C3,
// C6 and C7 of plan 04-13: every call after the first success is a background
// refresh, a request is abandoned after 10 seconds as a failed poll, the first
// request is flagged on the root element, and the success write records the
// load. Honest limit: this reads source shapes. The Node sandbox runs in the
// summary and the human check show the behaviour.
func TestFetchReportsBackgroundRefreshAndTimeout(t *testing.T) {
	app := p413ReadStatic(t, "static/js/app.js")
	b := jsFunctionBody(t, app, "fetchReports(options)")

	phase3 := "var isBackgroundRefresh = state.status === 'ok' || state.reports.length > 0;"
	widen := "if (state.hasLoaded === true) {\n      isBackgroundRefresh = true;\n    }"
	p413Ordered(t, "fetchReports", b, phase3, widen, "if (!isBackgroundRefresh) {")

	p413MustContain(t, "fetchReports", b,
		"AbortSignal.timeout(10000)", "new AbortController()", "controller.abort()",
		"signal: signal", "url = new Request(url, { signal: signal });",
		"pending = fetch(url)", "setAttribute('data-feed-requested', 'true')")
	flagAt := strings.Index(b, "data-feed-requested")
	requestAt := strings.Index(b, "url = new Request(")
	fetchAt := strings.Index(b, "pending = fetch(url)")
	if !(flagAt != -1 && requestAt != -1 && fetchAt != -1 && flagAt < fetchAt && requestAt < fetchAt) {
		t.Errorf("the flag and the Request must come before the request call (flag %d, request %d, call %d)",
			flagAt, requestAt, fetchAt)
	}
	if n := strings.Count(b, "10000"); n < 2 {
		t.Errorf("the 10000 ms limit must appear for the native signal and the fallback timer, found %d", n)
	}

	success, ok, _ := windowAfter(b, ".then(function (body) {", ".catch(")
	if !ok {
		t.Fatal("could not locate the success handler of fetchReports")
	}
	p413Ordered(t, "success handler", success,
		"window.clearTimeout(abortTimer)", "if (isStale()) {", "lastAppliedSeq = mySeq;",
		"state.hasLoaded = true;", "state.lastUpdatedAt = Date.now();", "notify();")

	failure, ok, _ := windowAfter(b, ".catch(function (err) {", "});")
	if !ok {
		t.Fatal("could not locate the failure handler of fetchReports")
	}
	p413Ordered(t, "failure handler", failure,
		"window.clearTimeout(abortTimer)", "if (isStale()) {", "state.status = 'error'")
	if n := strings.Count(failure, "return"); n != 1 {
		t.Errorf("the failure handler must hold exactly one return (the stale guard), found %d", n)
	}
	p413MustNotContain(t, "failure handler", failure,
		".name", "lastAppliedSeq", "writeFloor", "hasLoaded", "lastUpdatedAt")

	if n := strings.Count(app, "state.hasLoaded = true;"); n != 1 {
		t.Errorf("state.hasLoaded = true; must be written once, found %d", n)
	}
	if n := strings.Count(app, "state.lastUpdatedAt = Date.now();"); n != 1 {
		t.Errorf("state.lastUpdatedAt = Date.now(); must be written once, found %d", n)
	}
	literal, ok, _ := windowAfter(app, "var state = {", "};")
	if !ok {
		t.Fatal("could not locate the state literal")
	}
	p413MustContain(t, "the state literal", literal, "hasLoaded: false", "lastUpdatedAt: null")
}

// TestShowToastDuration pins P4-A12: the optional third argument, the root
// flag that every hide removes, and the three width declarations on #toast.
// Honest limit: source shapes only; the human check runs a six second toast at
// 320 px.
func TestShowToastDuration(t *testing.T) {
	app := p413ReadStatic(t, "static/js/app.js")
	s := jsFunctionBody(t, app, "showToast(message, action, durationMs)")
	p413Ordered(t, "showToast", s,
		"var plainMs = (typeof durationMs === 'number' && durationMs > 0) ? durationMs : TOAST_MS;",
		"startToastTimer(toastEl, hasAction ? TOAST_ACTION_MS : plainMs);")
	p413Ordered(t, "showToast", s, "setAttribute('data-toast-open', 'true')",
		"startToastTimer(toastEl, hasAction ? TOAST_ACTION_MS : plainMs);")

	timer := jsFunctionBody(t, app, "startToastTimer(toastEl, ms)")
	p413Ordered(t, "startToastTimer", timer, "toastEl.hidden = true;", "removeAttribute('data-toast-open')")

	hides := strings.Count(app, "toastEl.hidden = true;")
	drops := strings.Count(app, "removeAttribute('data-toast-open')")
	if hides != drops || hides == 0 {
		t.Errorf("every place the toast hides must drop the flag: %d hides, %d removals", hides, drops)
	}
	if n := strings.Count(app, "var TOAST_MS = 3000;"); n != 1 {
		t.Errorf("TOAST_MS must be declared once, found %d", n)
	}
	if strings.Contains(app, "TOAST_DEFAULT_MS") {
		t.Error("the default toast length is TOAST_MS; no TOAST_DEFAULT_MS")
	}
	if n := strings.Count(app, "function showToast("); n != 1 {
		t.Errorf("showToast must be declared once, found %d", n)
	}
	if n := strings.Count(app, "getElementById('toast')"); n != 1 {
		t.Errorf("the toast element must be looked up once, found %d", n)
	}
	p413MustContain(t, "app.js", app, "showToast: showToast")

	main := p413ReadStatic(t, "static/css/main.css")
	rule, ok := ruleBySelector(parseCSSRules(main), "#toast")
	if !ok {
		t.Fatal("main.css has no #toast rule")
	}
	decls := declsOf(rule.declBody)
	want := map[string]string{
		"width":         "max-content",
		"max-width":     "calc(100vw - 2 * var(--space-md))",
		"text-align":    "center",
		"left":          "50%",
		"transform":     "translateX(-50%)",
		"border-radius": "8px",
		"z-index":       "1200",
	}
	for prop, value := range want {
		if decls[prop] != value {
			t.Errorf("#toast %s = %q, want %q", prop, decls[prop], value)
		}
	}
	if n := strings.Count(main, "@media (prefers-reduced-motion: reduce)"); n != 1 {
		t.Errorf("main.css must keep exactly one reduced motion block, found %d", n)
	}
}

// TestUpdatedAgoPhrases pins the phrases of the stale line's age and that
// relativeTime is untouched. Honest limit: the sandbox runs in the summary
// compute real values; this test reads the source words.
func TestUpdatedAgoPhrases(t *testing.T) {
	app := p413ReadStatic(t, "static/js/app.js")
	u := jsFunctionBody(t, app, "updatedAgo(timestampMs)")
	p413MustContain(t, "updatedAgo", u,
		"Math.floor", "'just now'", "'1 minute ago'", "' minutes ago'", "'1 hour ago'", "' hours ago'")
	p413MustContain(t, "app.js", app, "updatedAgo: updatedAgo")

	r := jsFunctionBody(t, app, "relativeTime(iso)")
	p413MustContain(t, "relativeTime", r, "' min ago'")
	p413MustNotContain(t, "relativeTime", r, "Math.floor", "Date.now()")
}

// TestFeedRenderKeepsRowsOnError pins P4-A19, D-20 and rule C4: once a list
// has loaded, render takes the list path for every status, so a failed poll
// keeps the rows and the pins; the Couldn't load reports panel belongs to
// renderFirstLoad alone. Honest limit: source shapes only; the Node sandbox
// runs and the human check show that rows stay on a real screen.
func TestFeedRenderKeepsRowsOnError(t *testing.T) {
	feed := p413ReadStatic(t, "static/js/feed.js")
	r := jsFunctionBody(t, feed, "render(state)")

	p413Ordered(t, "render", r,
		"var listShown = state.hasLoaded === true || state.reports.length > 0;",
		"if (!listShown) {", "renderFirstLoad(state);", "return;", "}",
		"renderStale(state)", "if (state.status === 'error') {")
	if n := strings.Count(r, "renderFirstLoad(state)"); n != 1 {
		t.Errorf("render must call renderFirstLoad once, found %d", n)
	}
	if first, rows := strings.Index(r, "renderFirstLoad(state)"), strings.Index(r, "renderRows("); rows != -1 && first > rows {
		t.Errorf("renderFirstLoad must come before renderRows (%d, %d)", first, rows)
	}

	start := strings.Index(r, "if (state.status === 'error') {")
	errBlock, _, _ := strings.Cut(r[start:], "return;")
	p413MustContain(t, "the error block", errBlock,
		"var hasRows = state.reports.length > 0;", "setHidden(reportListEl, !hasRows);",
		"if (!hasRows) {\n        clearRows();\n      }", "setEmptyState('none');")
	p413MustNotContain(t, "the error block", errBlock, "setHidden(errorEl, false)", "renderRows(")

	if n := strings.Count(r, "setHidden(reportListEl, true)"); n != 2 {
		t.Errorf("render must hide the list in exactly two places (the Past hide and the empty branch), found %d", n)
	}
	if n := strings.Count(feed, "setHidden(errorEl, false)"); n != 1 {
		t.Errorf("feed.js must show the error panel once, found %d", n)
	}
	first := jsFunctionBody(t, feed, "renderFirstLoad(state)")
	p413MustContain(t, "renderFirstLoad", first, "setHidden(errorEl, false)")

	renderAt := strings.Index(feed, "function render(state)")
	for _, name := range []string{"function renderFirstLoad(state)", "function renderStale(state)"} {
		at := strings.Index(feed, name)
		if at == -1 || at > renderAt {
			t.Errorf("%s must be declared before render (%d, %d)", name, at, renderAt)
		}
	}
}

// TestFeedRenderLoadingKeepsRows pins P4-A19: only renderFirstLoad shows the
// skeleton, and render itself never branches on idle or loading. Honest limit:
// source shapes only.
func TestFeedRenderLoadingKeepsRows(t *testing.T) {
	feed := p413ReadStatic(t, "static/js/feed.js")
	if n := strings.Count(feed, "setHidden(skeletonEl, false)"); n != 1 {
		t.Fatalf("feed.js must show the skeleton once, found %d", n)
	}
	first := jsFunctionBody(t, feed, "renderFirstLoad(state)")
	p413MustContain(t, "renderFirstLoad", first, "setHidden(skeletonEl, false)")

	r := jsFunctionBody(t, feed, "render(state)")
	p413Ordered(t, "render", r, "setHidden(skeletonEl, true)", "renderStale(state)")
	p413MustNotContain(t, "render", r, "state.status === 'loading'", "state.status === 'idle'")
}

// TestFeedStaleLine pins D-20: the stale line markup, its words, its one
// announcement on each change, the silent minute refresh and the Retry now
// wiring. Honest limit: a screen reader is not run here; the human check
// listens to the announcements.
func TestFeedStaleLine(t *testing.T) {
	tmpl := p413ReadTemplate(t, "index.html.tmpl")
	p413MustContain(t, "index.html.tmpl", tmpl,
		`<div class="feed-stale" id="feed-stale" hidden>`,
		`<p class="inline-note feed-stale__text" id="feed-stale-text"></p>`,
		`<button type="button" class="btn" id="feed-stale-retry">Retry now</button>`)
	staleAt := strings.Index(tmpl, `<div class="feed-stale" id="feed-stale" hidden>`)
	filterAt := strings.Index(tmpl, `id="filter-line"`)
	listAt := strings.Index(tmpl, `<ol id="report-list">`)
	if !(filterAt != -1 && staleAt > filterAt && listAt > staleAt) {
		t.Fatalf("the stale block must sit between the filter line and the list (%d, %d, %d)", filterAt, staleAt, listAt)
	}
	block, after, _ := strings.Cut(tmpl[staleAt:listAt], "</button>")
	p413MustNotContain(t, "the stale block", block, "role=", "aria-live")
	if strings.TrimSpace(after) != "</div>" {
		t.Errorf("the stale block must be the last element before the list, found %q after its button", after)
	}

	live := `<div id="notice-live" class="visually-hidden" role="status" aria-live="polite"></div>`
	if n := strings.Count(tmpl, live); n != 1 {
		t.Fatalf("notice-live must appear once, found %d", n)
	}
	liveAt := strings.Index(tmpl, live)
	toggleAt := strings.Index(tmpl, `{{template "theme_toggle.html.tmpl" .}}`)
	shellAt := strings.Index(tmpl, `id="app-shell"`)
	if !(toggleAt != -1 && liveAt > toggleAt && liveAt < shellAt) {
		t.Errorf("notice-live must sit after the theme toggle and before the shell (%d, %d, %d)", toggleAt, liveAt, shellAt)
	}

	feed := p413ReadStatic(t, "static/js/feed.js")
	p413MustContain(t, "feed.js", feed,
		"'Last updated '", "'. Trying again.'",
		"STALE_ENTER_PREFIX = 'Connection lost. Showing reports from '",
		"STALE_LEAVE_MESSAGE = 'Back online. The list is up to date.'",
		"window.setInterval(refreshStaleLine, 60000)")
	stale := jsFunctionBody(t, feed, "renderStale(state)")
	p413MustContain(t, "renderStale", stale,
		"state.status === 'error'", "announce(STALE_ENTER_PREFIX", "announce(STALE_LEAVE_MESSAGE)",
		"staleRetryBtn.disabled = false")
	p413MustContain(t, "announce", jsFunctionBody(t, feed, "announce(text)"), "Pinalert.setText(noticeLiveEl")
	p413MustNotContain(t, "refreshStaleLine", jsFunctionBody(t, feed, "refreshStaleLine()"), "announce(")

	listenerAt := strings.Index(feed, "staleRetryBtn.addEventListener('click'")
	pageshowAt := strings.Index(feed, "addEventListener('pageshow'")
	if listenerAt == -1 || pageshowAt == -1 || listenerAt > pageshowAt {
		t.Fatalf("the Retry now listener must come before the page restore listener (%d, %d)", listenerAt, pageshowAt)
	}
	p413Ordered(t, "the Retry now listener", feed[listenerAt:pageshowAt],
		"staleRetryBtn.disabled = true", "Pinalert.fetchReports()")
	raw, err := StaticFS.ReadFile("static/js/feed.js")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), "'pageshow'"); n != 1 {
		t.Errorf("feed.js must keep exactly one quoted page restore event name, found %d", n)
	}

	chrome := p413ReadStatic(t, "static/css/chrome.css")
	rules := parseCSSRules(chrome)
	line, ok := ruleBySelector(rules, ".feed-stale:not([hidden])")
	if !ok {
		t.Fatal("chrome.css has no .feed-stale:not([hidden]) rule")
	}
	d := declsOf(line.declBody)
	for prop, want := range map[string]string{
		"display": "flex", "flex-wrap": "wrap", "align-items": "center", "gap": "var(--space-sm)",
	} {
		if d[prop] != want {
			t.Errorf(".feed-stale %s = %q, want %q", prop, d[prop], want)
		}
	}
	text, ok := ruleBySelector(rules, ".feed-stale .feed-stale__text")
	if !ok {
		t.Fatal("chrome.css has no .feed-stale .feed-stale__text rule")
	}
	if declsOf(text.declBody)["margin"] != "0" {
		t.Errorf(".feed-stale__text must set margin 0")
	}
	if strings.Index(chrome, ".feed-stale .feed-stale__text") < strings.Index(chrome, ".inline-note:not([hidden])") {
		t.Error("the stale text rule must come after the inline note rule so the zero margin wins")
	}

	// The hidden guard walk: TestPhase4HiddenGuard does not list .feed-stale.
	inspected := 0
	err = fs.WalkDir(StaticFS, "static/css", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".css") {
			return walkErr
		}
		css, readErr := StaticFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, r := range parseCSSRules(string(css)) {
			head := r.selectorHead
			if i := strings.LastIndex(head, "{"); i != -1 {
				head = head[i+1:]
			}
			display := declsOf(r.declBody)["display"]
			for sel := range strings.SplitSeq(head, ",") {
				sel = strings.TrimSpace(sel)
				if !strings.Contains(sel, ".feed-stale") || display == "" || display == "none" {
					continue
				}
				inspected++
				if !strings.Contains(sel, ":not([hidden])") {
					t.Errorf("%s: %q sets display %q without :not([hidden])", path, sel, display)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inspected == 0 {
		t.Error("the hidden guard walk inspected no .feed-stale selector")
	}
}
