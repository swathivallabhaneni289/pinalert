package web

import (
	"strings"
	"testing"
)

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
