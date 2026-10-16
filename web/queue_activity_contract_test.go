package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file pins plan 04-25: the saved reports strip on the Activity page
// (D-20) and the Edit and resend link that goes to the page with a form
// (D-17, T-04-01, T-04-13, T-04-14).

// p25ReadTemplate returns the raw text of an embedded template.
func p25ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("reading embedded template %s: %v", name, err)
	}
	return string(raw)
}

// p25ReadStatic returns the raw text of an embedded static file.
func p25ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := StaticFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", path, err)
	}
	return string(raw)
}

// p25OpeningTag returns the tag that holds marker: from the nearest opening
// bracket at or before the marker's first byte to the first closing bracket at
// or after it. It fails the test when either end is missing.
func p25OpeningTag(t *testing.T, text, marker string) string {
	t.Helper()
	idx := strings.Index(text, marker)
	if idx == -1 {
		t.Fatalf("could not locate %q", marker)
	}
	start := strings.LastIndex(text[:idx+1], "<")
	if start == -1 {
		t.Fatalf("no opening bracket at or before %q", marker)
	}
	end := strings.Index(text[idx:], ">")
	if end == -1 {
		t.Fatalf("no closing bracket after %q", marker)
	}
	return text[start : idx+end+1]
}

// p25MountArgs returns the argument object of the PinalertQueue.mount call:
// from "PinalertQueue.mount({" to the first "});" after it.
func p25MountArgs(t *testing.T, text string) string {
	t.Helper()
	const anchor = "PinalertQueue.mount({"
	idx := strings.Index(text, anchor)
	if idx == -1 {
		t.Fatalf("could not locate %q", anchor)
	}
	rest := text[idx:]
	end := strings.Index(rest, "});")
	if end == -1 {
		t.Fatalf("could not locate the end of the mount call")
	}
	return rest[:end]
}

// TestQueueActivityStrip proves the Activity page carries the strip container,
// the announcer, the account id hook and the script order, and that
// activity.js mounts the strip once before its list guard with no edit
// callback.
//
// Honest limit: a source test proves wiring, not runtime behaviour. The
// optional Node test and the end of phase human check cover behaviour.
func TestQueueActivityStrip(t *testing.T) {
	profile := p25ReadTemplate(t, "profile.html.tmpl")
	index := p25ReadTemplate(t, "index.html.tmpl")

	if n := strings.Count(profile, `id="queue-strip"`); n != 1 {
		t.Fatalf(`profile.html.tmpl has id="queue-strip" %d times, want 1`, n)
	}
	stripTag := p25OpeningTag(t, profile, `id="queue-strip"`)
	if !strings.Contains(stripTag, " hidden") {
		t.Errorf("the strip container %q is not hidden", stripTag)
	}
	if mainTag := p25OpeningTag(t, index, `id="queue-strip"`); mainTag != stripTag {
		t.Errorf("the strip container differs from the main page one:\n  activity: %s\n  main:     %s", stripTag, mainTag)
	}
	stripAt := strings.Index(profile, `id="queue-strip"`)
	if closeAt := strings.Index(profile[stripAt:], "</div>"); closeAt == -1 || strings.Contains(profile[stripAt:stripAt+closeAt], "<h2") {
		t.Errorf("the strip container holds a hard coded h2 or is not closed")
	}
	headingAt := strings.Index(profile, "<h2>Activity</h2>")
	reportsAt := strings.Index(profile, "{{if .Reports}}")
	if headingAt == -1 || reportsAt == -1 || !(headingAt < stripAt && stripAt < reportsAt) {
		t.Errorf("the strip must sit after <h2>Activity</h2> and before {{if .Reports}}")
	}

	const live = `<div id="notice-live" class="visually-hidden" role="status" aria-live="polite"></div>`
	if n := strings.Count(profile, live); n != 1 {
		t.Fatalf("the #notice-live element appears %d times, want 1", n)
	}
	liveAt := strings.Index(profile, live)
	chromeAt := strings.Index(profile, `class="top-chrome"`)
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-08): the
	// light and dark button moved into the laptop top bar, so the page no
	// longer includes it; the point the live region follows is the top bar that
	// holds it. The Phase 3.1 and Phase 4 hands-on tests are walked on this
	// layout after Phase 3.4 is built.
	toggleAt := strings.Index(profile, `{{template "app_topbar" .}}`)
	mainAt := strings.Index(profile, `<main class="profile-page"`)
	if chromeAt == -1 || toggleAt == -1 || mainAt == -1 || !(chromeAt < toggleAt && toggleAt < liveAt && liveAt < mainAt) {
		t.Errorf("#notice-live must sit after .top-chrome and the theme toggle and before main")
	}

	shell := p25OpeningTag(t, index, `id="app-shell"`)
	field := regexp.MustCompile(`data-account-id="(\{\{\.[A-Za-z]+\}\})"`).FindStringSubmatch(shell)
	if field == nil {
		t.Fatalf("the main page shell tag %q has no data-account-id field", shell)
	}
	pageTag := p25OpeningTag(t, profile, `<main class="profile-page"`)
	if !strings.Contains(pageTag, `data-account-id="`+field[1]+`"`) {
		t.Errorf(".profile-page tag %q must carry data-account-id=%q like the main page shell", pageTag, field[1])
	}

	order := []string{"app.js", "votes.js", "visibility.js", "trust.js", "queue.js", "activity.js", "account-menu.js"}
	last := -1
	for _, name := range order {
		src := "/static/js/" + name + "?v="
		if n := strings.Count(profile, src); n != 1 {
			t.Fatalf("profile.html.tmpl loads %s %d times, want 1", name, n)
		}
		at := strings.Index(profile, src)
		if at < last {
			t.Errorf("%s loads out of order", name)
		}
		last = at
	}
	window := findTagWindow(t, profile, "/static/js/queue.js")
	if !strings.Contains(window, " defer") || !strings.Contains(window, "?v={{.AssetVersion}}") {
		t.Errorf("the queue.js tag %q needs defer and the asset version", window)
	}
	for _, banned := range []string{"report-form.js", "flag.js", "/feed.js", "/low.js"} {
		if strings.Contains(profile, banned) {
			t.Errorf("the Activity page must not load %s", banned)
		}
	}

	rawActivity := p25ReadStatic(t, "static/js/activity.js")
	activity := stripJSComments(rawActivity)
	mountAt := strings.Index(activity, "PinalertQueue.mount(")
	guardAt := strings.Index(activity, "var list = document.querySelector('.profile-report-list');")
	if n := strings.Count(activity, "PinalertQueue.mount("); n != 1 {
		t.Fatalf("activity.js has %d mount calls, want 1", n)
	}
	if guardAt == -1 || mountAt > guardAt {
		t.Errorf("the mount call must come before the list guard")
	}
	args := p25MountArgs(t, activity)
	for _, want := range []string{"headingLevel: 3", "accountId:", "resendHref: resendHref", "focusFallback: queueFocusFallback"} {
		if !strings.Contains(args, want) {
			t.Errorf("the mount options lack %q", want)
		}
	}
	if strings.Contains(args, "onEdit") {
		t.Errorf("the Activity mount must pass no edit callback")
	}
	for _, want := range []string{"getElementById('queue-strip')", "dataset.accountId", "window.PinalertQueue"} {
		if !strings.Contains(activity, want) {
			t.Errorf("activity.js lacks %q", want)
		}
	}
	fallback := jsFunctionBody(t, activity, "queueFocusFallback()")
	for _, want := range []string{".profile-report-list", "closest('[hidden]')", ".profile-back-link"} {
		if !strings.Contains(fallback, want) {
			t.Errorf("queueFocusFallback lacks %q", want)
		}
	}

	counts := []struct {
		needle string
		want   int
	}{
		{"accountID", 0}, {"addEventListener", 1}, {"Pinalert.showToast(", 1}, {"fetch(", 0},
		{"indexedDB", 0}, {"localStorage", 0}, {"PinalertReportForm", 0},
		{"location.search", 0}, {"URLSearchParams", 0}, {"PinalertTrust.focusToggle(", 1},
	}
	for _, c := range counts {
		if got := strings.Count(rawActivity, c.needle); got != c.want {
			t.Errorf("activity.js has %q %d times, want %d", c.needle, got, c.want)
		}
	}

	// 04-22's focus helper already takes a link as a row control ("button, a"),
	// so this plan widened nothing and makes no queue.js focus assertion.
}

// TestQueueEditAndResendLink proves the Rejected row's Edit and resend control
// becomes a link to the page that has a form: a constant path, the encoded
// request id, built by DOM calls, and drawn only for a single slash path.
//
// Honest limit: a source test proves wiring, not runtime behaviour. The
// optional Node test and the end of phase human check cover behaviour.
func TestQueueEditAndResendLink(t *testing.T) {
	activity := stripJSComments(p25ReadStatic(t, "static/js/activity.js"))
	queue := stripJSComments(p25ReadStatic(t, "static/js/queue.js"))

	body := jsFunctionBody(t, activity, "resendHref(record)")
	for _, want := range []string{
		"querySelector('.profile-back-link')", "getAttribute('href') === '/low'", "'/low'",
		"'?resend='", "encodeURIComponent(record.request_id)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("resendHref lacks %q", want)
		}
	}
	if n := strings.Count(body, "getAttribute("); n != 1 {
		t.Errorf("resendHref calls getAttribute %d times, want 1", n)
	}
	for _, banned := range []string{"document.cookie", "view="} {
		if strings.Contains(body, banned) {
			t.Errorf("resendHref must not contain %q", banned)
		}
	}

	safe := jsFunctionBody(t, queue, "safeResendHref(value)")
	for _, want := range []string{"typeof value === 'string'", "charAt(0) === '/'", "charAt(1) !== '/'", `charAt(1) !== '\\'`, "return ''"} {
		if !strings.Contains(safe, want) {
			t.Errorf("safeResendHref lacks %q", want)
		}
	}
	link := jsFunctionBody(t, queue, "createResendLink(href)")
	for _, want := range []string{"document.createElement('a')", "setAttribute('href', href)", "Pinalert.setText("} {
		if !strings.Contains(link, want) {
			t.Errorf("createResendLink lacks %q", want)
		}
	}
	if n := strings.Count(queue, "safeResendHref("); n != 2 {
		t.Errorf("safeResendHref( appears %d times, want 2 (definition and one call)", n)
	}
	if n := strings.Count(queue, "createResendLink("); n != 2 {
		t.Errorf("createResendLink( appears %d times, want 2 (definition and one call)", n)
	}
	if n := len(regexp.MustCompile(`safeResendHref\([^;]*resendHref\(`).FindAllString(queue, -1)); n != 1 {
		t.Errorf("the link target must pass through safeResendHref(resendHref(...)) once, found %d", n)
	}
	// Measured by the 04-25 census: the label literal occurs once in queue.js
	// code, so the link reuses the existing label constant.
	if n := strings.Count(queue, "Edit and resend"); n != 1 {
		t.Errorf("the label literal Edit and resend appears %d times, want 1", n)
	}
}
