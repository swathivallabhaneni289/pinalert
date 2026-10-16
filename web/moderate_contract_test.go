package web

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
)

// This file locks the client half of the moderator screen (plan 04-26, D-08):
// moderate.js, the page template and moderate.css. Every test reads the shipped
// files out of the embedded filesystems, never a copy of their text. The
// honest limit shared by all of them: static inspection cannot run the script
// in a browser, so the human check in the plan (the V-24 script run in 04-27)
// is what proves the screen works on a phone.

// p426ReadJS returns the embedded moderate.js with its comments removed.
func p426ReadJS(t *testing.T) string {
	t.Helper()
	raw, err := StaticFS.ReadFile("static/js/moderate.js")
	if err != nil {
		t.Fatalf("reading embedded static/js/moderate.js: %v", err)
	}
	return stripJSComments(string(raw))
}

// p426ReadTemplate returns the raw moderator page template.
func p426ReadTemplate(t *testing.T) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/moderate.html.tmpl")
	if err != nil {
		t.Fatalf("reading embedded templates/moderate.html.tmpl: %v", err)
	}
	return string(raw)
}

// p426ReadCSS returns the embedded moderate.css with its comments removed.
func p426ReadCSS(t *testing.T) string {
	t.Helper()
	raw, err := StaticFS.ReadFile("static/css/moderate.css")
	if err != nil {
		t.Fatalf("reading embedded static/css/moderate.css: %v", err)
	}
	return stripCSSComments(string(raw))
}

// p426CountQuoted counts s wrapped in single quotes plus s wrapped in double
// quotes.
func p426CountQuoted(js, s string) int {
	return strings.Count(js, "'"+s+"'") + strings.Count(js, `"`+s+`"`)
}

// p426CountTemplateText counts s as the whole text of an element: a greater
// than sign, optional whitespace, s, optional whitespace and a less than sign.
func p426CountTemplateText(tmpl, s string) int {
	re := regexp.MustCompile(`>\s*` + regexp.QuoteMeta(s) + `\s*<`)
	return len(re.FindAllStringIndex(tmpl, -1))
}

// p426WantAll fails for every needle that body does not contain.
func p426WantAll(t *testing.T, where, body string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(body, n) {
			t.Errorf("%s: missing %q", where, n)
		}
	}
}

// p426WantNone fails for every needle that body contains.
func p426WantNone(t *testing.T, where, body string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(body, n) {
			t.Errorf("%s: must not contain %q", where, n)
		}
	}
}

// p426InOrder fails unless the first occurrence of each anchor sits after the
// first occurrence of the one before it.
func p426InOrder(t *testing.T, where, body string, anchors ...string) {
	t.Helper()
	last := -1
	for _, a := range anchors {
		i := strings.Index(body, a)
		if i < 0 {
			t.Errorf("%s: missing %q", where, a)
			return
		}
		if i <= last {
			t.Errorf("%s: %q is out of order", where, a)
		}
		last = i
	}
}

// TestModerateUsesSetTextOnly (T-04-01, D-05): every string reaches the page
// through Pinalert.setText, the one href comes from the closed view set, and
// the words other modules own are never typed here.
//
// Honest limit of this test's claim: static inspection proves no markup
// parsing sink is present and that each writer line goes through setText; it
// cannot run the script in a browser, which the human check in Task 2 does.
func TestModerateUsesSetTextOnly(t *testing.T) {
	js := p426ReadJS(t)

	sinks := []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"}
	p426WantNone(t, "sinks", js, sinks...)
	p426WantNone(t, "other writers", js, "textContent", "innerText", ".style.", "setAttribute('on",
		"setAttribute('style'", "eval(", "new Function")

	if n := strings.Count(js, "Pinalert.setText("); n < 20 {
		t.Errorf("Pinalert.setText( occurs %d times, want at least 20", n)
	} else {
		t.Logf("measured Pinalert.setText( count: %d", n)
	}

	for i, line := range strings.Split(js, "\n") {
		if strings.Contains(line, ".description") && !strings.Contains(line, "Pinalert.setText(") {
			t.Errorf("line %d reads .description without Pinalert.setText: %q", i+1, strings.TrimSpace(line))
		}
		if strings.Contains(line, ".visibility_word") || strings.Contains(line, ".label") {
			if !strings.Contains(line, "Pinalert.setText(") && !strings.Contains(line, "typeof ") &&
				!strings.Contains(line, "var ") {
				t.Errorf("line %d reads a server word without setText, a guard or a local: %q", i+1, strings.TrimSpace(line))
			}
		}
	}

	p426WantNone(t, "builder owned classes", js, "severity-label", "visibility-tag")

	if n := strings.Count(js, "'href'"); n != 1 {
		t.Errorf("'href' occurs %d times, want exactly 1", n)
	}
	row := jsFunctionBody(t, js, "buildRow(report, listKey, lists)")
	p426WantAll(t, "buildRow", row, "'href'", "mainPageHref(report)")
	href := jsFunctionBody(t, js, "mainPageHref(report)")
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-13, D-14,
	// D-18): the Past tab left Nearby, so the link of a report that is active or
	// disputed is /?report=ID (All shows a disputed report in its last group)
	// and the link of a resolved or past report is /profile?part=ended&report=ID.
	// Both come from two closed lists of view words and a numeric id. The Phase
	// 3.1 and Phase 4 hands-on tests are walked on this layout after Phase 3.4
	// is built.
	p426WantAll(t, "mainPageHref", href, "NEARBY_VIEWS.indexOf(report.view)", "ENDED_VIEWS.indexOf(report.view)",
		"'/?report='", "'/profile?part=ended&report='", "isReportId(report.id)", "moderationWord(report)")
	p426WantNone(t, "mainPageHref", href, "'/?view='", "MAIN_PAGE_VIEWS")

	flagGo, err := os.ReadFile("../internal/service/flag.go")
	if err != nil {
		t.Fatalf("reading flag.go: %v", err)
	}
	labels := []string{"Spam or gibberish", "Abusive or hateful", "False or a hoax", "Dangerous or illegal", "Something else"}
	for _, l := range labels {
		if !strings.Contains(string(flagGo), l) {
			t.Errorf("flag.go no longer holds the label %q", l)
		}
		if p426CountQuoted(js, l) != 0 {
			t.Errorf("moderate.js types the flag label %q; labels come from the JSON", l)
		}
	}
	words := []string{"Showing to everyone", "Disputed", "Resolved", "Expired", "No longer in any view"}
	for _, w := range words {
		if p426CountQuoted(js, w) != 0 {
			t.Errorf("moderate.js types the visibility word %q; the server sends it", w)
		}
	}
}

// TestModerateRemoveIsConfirmed (D-08, P4-A17, P4-A5): Remove report opens a
// named confirmation first and only the confirm button sends a remove.
//
// Honest limit of this test's claim: static inspection proves the call graph
// and the DOM order; it cannot prove focus lands where the code says in a real
// browser, which the human check does.
func TestModerateRemoveIsConfirmed(t *testing.T) {
	js := p426ReadJS(t)

	for _, s := range []string{"'/remove'", "'/keep'"} {
		if n := strings.Count(js, s); n != 1 {
			t.Errorf("%s occurs %d times, want exactly 1", s, n)
		}
	}
	send := jsFunctionBody(t, js, "sendAction(entry, action)")
	p426WantAll(t, "sendAction", send, "'/remove'", "'/keep'")
	if n := strings.Count(js, "sendAction(entry, 'remove')"); n != 1 {
		t.Errorf("sendAction(entry, 'remove') occurs %d times, want 1", n)
	}
	if n := strings.Count(js, "sendAction(entry, 'keep')"); n != 1 {
		t.Errorf("sendAction(entry, 'keep') occurs %d times, want 1", n)
	}

	click := jsFunctionBody(t, js, "onRootClick(evt)")
	w, okA, okT := windowAfter(click, "action === 'remove'", "}")
	if !okA || !okT {
		t.Fatalf("the remove branch of onRootClick was not found (anchor %v, terminator %v)", okA, okT)
	}
	p426WantAll(t, "remove branch", w, "openRemoveConfirm(entry)")
	p426WantNone(t, "remove branch", w, "sendAction(")
	w, okA, okT = windowAfter(click, "action === 'confirm-remove'", "}")
	if !okA || !okT {
		t.Fatalf("the confirm-remove branch of onRootClick was not found (anchor %v, terminator %v)", okA, okT)
	}
	p426WantAll(t, "confirm-remove branch", w, "sendAction(entry, 'remove')")

	c := jsFunctionBody(t, js, "buildRemoveConfirm(entry)")
	p426WantAll(t, "buildRemoveConfirm", c, "setAttribute('role', 'group')", "'aria-labelledby'", "REMOVE_QUESTION",
		"GO_BACK_LABEL", "CONFIRM_REMOVE_LABEL", "'btn btn--destructive'", "'confirm-remove'", "'back'")
	back, confirm := strings.Index(c, "appendChild(goBack)"), strings.Index(c, "appendChild(confirmBtn)")
	if back < 0 || confirm < 0 || back >= confirm {
		t.Errorf("Go back must be appended before Yes, remove report (indexes %d and %d)", back, confirm)
	}

	p426WantAll(t, "openRemoveConfirm", jsFunctionBody(t, js, "openRemoveConfirm(entry)"), "entry.goBack.focus()")
	p426WantAll(t, "closeRemoveConfirm", jsFunctionBody(t, js, "closeRemoveConfirm(entry, returnFocus)"),
		"entry.removeBtn.focus()")
	p426WantAll(t, "onRootKeydown", jsFunctionBody(t, js, "onRootKeydown(evt)"), "'Escape'", "closeRemoveConfirm(", "entry.busy")

	const question = `var REMOVE_QUESTION = 'Remove this report? The author is told it was removed after review.';`
	if n := strings.Count(js, question); n != 1 {
		t.Errorf("the REMOVE_QUESTION line occurs %d times, want exactly 1", n)
	}
}

// TestModerateFindParsesReportParam (D-08): Find takes a bare id or any pasted
// link carrying report=ID, and only digits ever reach the request path.
//
// Honest limit of this test's claim: the pattern is compiled with Go's regexp
// engine, which agrees with the JavaScript engine on this simple pattern; the
// parse function itself is checked by text, not run.
func TestModerateFindParsesReportParam(t *testing.T) {
	js := p426ReadJS(t)

	for _, line := range []string{
		`var REPORT_ID_PATTERN = /^[0-9]+$/;`,
		`var REPORT_PARAM_PATTERN = /(?:^|[?&#])report=([0-9]+)/;`,
		`var REPORT_ID_MAX_DIGITS = 15;`,
	} {
		if n := strings.Count(js, line); n != 1 {
			t.Errorf("%q occurs %d times, want exactly 1", line, n)
		}
	}
	p426WantAll(t, "parseReportId", jsFunctionBody(t, js, "parseReportId(text)"), "REPORT_ID_PATTERN.test(",
		"REPORT_PARAM_PATTERN.exec(", ".replace(/^0+/, '')", "REPORT_ID_MAX_DIGITS", "return null")

	const open, shut = "var REPORT_PARAM_PATTERN = /", "/;"
	start := strings.Index(js, open)
	if start < 0 {
		t.Fatal("REPORT_PARAM_PATTERN not found")
	}
	rest := js[start+len(open):]
	end := strings.Index(rest, shut)
	if end < 0 {
		t.Fatal("REPORT_PARAM_PATTERN has no closing slash")
	}
	param := regexp.MustCompile(rest[:end])
	table := []struct{ in, want string }{
		{"/?view=resolved&report=128", "128"},
		{"https://example.org/?report=42&view=past", "42"},
		{"report=7", "7"},
		{"#report=9", "9"},
		{"/?report=0042", "0042"},
		{"/?myreport=5", ""},
		{"/?report=", ""},
		{"abc", ""},
	}
	for _, c := range table {
		got := ""
		if m := param.FindStringSubmatch(c.in); m != nil {
			got = m[1]
		}
		if got != c.want {
			t.Errorf("param pattern on %q gave %q, want %q", c.in, got, c.want)
		}
	}
	id := regexp.MustCompile(`^[0-9]+$`)
	if !id.MatchString("128") || id.MatchString("12a") {
		t.Error("the id pattern must match 128 and reject 12a")
	}

	f := jsFunctionBody(t, js, "onFindSubmit(evt)")
	p426WantAll(t, "onFindSubmit", f, "evt.preventDefault()", "parseReportId(", "FIND_EMPTY_MESSAGE",
		"'/api/moderation/reports/' + id", "notFoundMessage(id)", "status === 404", "FIND_FAILED_MESSAGE",
		"FIND_OFFLINE_MESSAGE", "REQUEST_TIMEOUT_MS")
	p426InOrder(t, "onFindSubmit", f, "parseReportId(", "apiRequest(")
}

// TestModerateHeadingCountsFromServer (D-08, open item 12): the numbers in the
// three headings come from the server's counts object and are lowered only by
// dropReport, never counted from what is on screen.
//
// Honest limit of this test's claim: static inspection proves where the
// numbers are read from; it cannot prove the headings update on screen.
func TestModerateHeadingCountsFromServer(t *testing.T) {
	js := p426ReadJS(t)

	p426WantAll(t, "applyQueue", jsFunctionBody(t, js, "applyQueue(data)"), "data.counts.review", "data.counts.flagged",
		"data.counts.pending", "toCount(")
	r := jsFunctionBody(t, js, "renderCounts()")
	p426WantAll(t, "renderCounts", r, "counts[", "Pinalert.setText(", "'('")
	p426WantNone(t, "renderCounts", r, ".length", "children", "childElementCount", "querySelectorAll")
	d := jsFunctionBody(t, js, "dropReport(id, extraKeys)")
	p426WantAll(t, "dropReport", d, "Math.max(0, counts[", "seen[", "renderCounts()", "renderEmpties()")

	for i, line := range strings.Split(js, "\n") {
		if strings.Contains(line, "counts") && strings.Contains(line, ".length") {
			t.Errorf("line %d mixes counts and .length: %q", i+1, strings.TrimSpace(line))
		}
	}
}

// TestModerateRowStructure (D-08, P4-A8, R-01): the row has the parts of the
// moderation row in order, the why blocks say their reasons in words, and the
// busy and error states carry the right attributes.
//
// Honest limit of this test's claim: static inspection proves the parts and
// their order in the source; it cannot prove how the row looks, which the
// human check does at 320 and 360 px.
func TestModerateRowStructure(t *testing.T) {
	js := p426ReadJS(t)

	row := jsFunctionBody(t, js, "buildRow(report, listKey, lists)")
	p426InOrder(t, "buildRow", row, "buildIdLine(entry, report)", "Pinalert.relativeTime(report.created_at)",
		"Pinalert.writtenLine(report)", "report.description", "buildWhy(", "KEEP_LABEL", "buildRemoveConfirm(entry)",
		"'mod-row__does'", "MAIN_PAGE_LINK_TEXT")
	p426WantAll(t, "buildRow", row, "'aria-labelledby'", "'Keep report ' + ", "'Remove report ' + ", "'data-mod-key'",
		"REMOVED_SENTENCE")

	p426WantAll(t, "buildIdLine", jsFunctionBody(t, js, "buildIdLine(entry, report)"), "'Report ' + ",
		"PinalertTrust.createSeverityLabel()", "PinalertTrust.updateSeverityLabel(", "report.visibility_word",
		"moderationWord(report)")
	p426WantAll(t, "moderationWord", jsFunctionBody(t, js, "moderationWord(report)"), "'removed'", "'pending'",
		"report.serious !== true", "HELD_WORD", "REMOVED_WORD")
	p426WantAll(t, "buildScores", jsFunctionBody(t, js, "buildScores(report)"), "scoreLine(", "TOP_SCORES",
		"SCORED_INTRO", "SHOW_ALL_SCORES", "createElement('details')", "createElement('summary')", "sortedScores(")
	p426WantAll(t, "scoreLine", jsFunctionBody(t, js, "scoreLine(item)"), ".toFixed(2)")
	p426WantAll(t, "buildFlags", jsFunctionBody(t, js, "buildFlags(report)"), "flaggedLine(", ".label", ".count", "': '")
	p426WantAll(t, "flaggedLine", jsFunctionBody(t, js, "flaggedLine(total)"), "'1 person flagged this.'",
		"' people flagged this.'")
	p426WantAll(t, "buildUnfinished", jsFunctionBody(t, js, "buildUnfinished(report)"), "UNFINISHED_INTRO",
		"postedLine(report)", "HELD_LINE", "SERIOUS_PENDING_LINE")
	p426WantAll(t, "setBusy", jsFunctionBody(t, js, "setBusy(entry, busy, action)"), "'aria-busy'", "'aria-disabled'",
		"SENDING_LABEL", "'Sending, remove report ' + ", "'Yes, remove report ' + ")
	p426WantAll(t, "showRowError", jsFunctionBody(t, js, "showRowError(entry, message)"), "'inline-error mod-row__error'",
		"setAttribute('role', 'alert')")
	msg := jsFunctionBody(t, js, "showFindMessage(text, look)")
	p426WantAll(t, "showFindMessage", msg, "'inline-note'", "'inline-error'")
	p426WantNone(t, "showFindMessage", msg, "'role'")

	pairs := []string{
		`'hate': 'Hate'`, `'hate/threatening': 'Hate with threats'`, `'harassment': 'Harassment'`,
		`'harassment/threatening': 'Harassment with threats'`, `'illicit': 'Illicit'`,
		`'illicit/violent': 'Illicit and violent'`, `'self-harm': 'Self harm'`, `'self-harm/intent': 'Self harm intent'`,
		`'self-harm/instructions': 'Self harm instructions'`, `'sexual': 'Sexual'`,
		`'sexual/minors': 'Sexual involving minors'`, `'violence': 'Violence'`, `'violence/graphic': 'Graphic violence'`,
	}
	p426WantAll(t, "SCORE_WORDS", js, pairs...)
}

// TestModerateTransportAndNoPolling (D-08, D-18, open item 12): one request
// call, no timer that repeats, nothing stored for later, three listeners.
//
// Honest limit of this test's claim: static inspection proves the shapes are
// absent or single; it cannot prove the network behaviour, which the offline
// step of the human check does.
func TestModerateTransportAndNoPolling(t *testing.T) {
	js := p426ReadJS(t)

	if n := strings.Count(js, "fetch("); n != 1 {
		t.Errorf("fetch( occurs %d times, want exactly 1", n)
	}
	api := jsFunctionBody(t, js, "apiRequest(method, path, timeoutMs)")
	p426WantAll(t, "apiRequest", api, "fetch(", "credentials: 'same-origin'", "cache: 'no-store'", "timeoutMs > 0",
		"Pinalert.requestTimeout(timeoutMs, ", "Promise.race([promise, limit.expired])",
		"withinLimit(fetch(path, options))", "withinLimit(res.json())", "res.ok", "noAnswer: true")
	p426WantNone(t, "whole file", js, "setInterval", "indexedDB", "localStorage", "sessionStorage",
		"document.addEventListener", "window.addEventListener")
	if n := strings.Count(js, "addEventListener("); n != 3 {
		t.Errorf("addEventListener( occurs %d times, want exactly 3", n)
	}
	send := jsFunctionBody(t, js, "sendAction(entry, action)")
	p426WantAll(t, "sendAction", send, "apiRequest('POST', ", ", 0)")
	p426WantAll(t, "loadLists", jsFunctionBody(t, js, "loadLists(manual)"),
		"apiRequest('GET', '/api/moderation/queue', REQUEST_TIMEOUT_MS)", "'aria-busy'", "REFRESHED_TOAST", "TOAST_MS")
}

// TestModerateScreenCopy (D-08): every moderator string of the UI-SPEC appears
// exactly once across the template text and the quoted strings of the script.
//
// Honest limit of this test's claim: it counts text in the source; it cannot
// prove the strings are shown in the right place, which the human check does.
func TestModerateScreenCopy(t *testing.T) {
	tmpl := p426ReadTemplate(t)
	js := p426ReadJS(t)

	once := []string{
		"Moderator screen", "Only moderators can open this page.", "Refresh lists",
		"Couldn't refresh the lists. Try again.", "Find a report",
		"Type the report id, or paste a link that includes report=", "Find report",
		"Flagged by the automatic check", "Flagged by people", "Text check not finished",
		"Posted normally, but the text scored near the line. Newest first.",
		"People used Flag this report. Most flags first.",
		"The check couldn't finish. Serious reports in this list are already published. Waiting longest first.",
		"No reports flagged by the check.", "No reports flagged by people.", "No unfinished checks.",
		"Nothing needs a decision here. Use Find a report to look up a post by id.",
		"Enter a report id or a link that includes report=.", "Couldn't look that up. Try again.",
		"No signal. Try again when you have a signal.", "Keep report", "Remove report", "Go back",
		"Yes, remove report", "Sending", "Show all scores", "View on the main page", "Held, shown to no one",
		"Removed after review", "Removed after review.", "The check scored this text:",
		"The text check hasn't finished.",
		"Held. It isn't shown to anyone until the check finishes or you keep it.",
		"Serious report, already published while the check waits.", "Couldn't save that. Try again.",
		"No signal. Nothing changed. Try again.", "This report was already handled. Refresh the lists.",
		"Lists refreshed.", "Remove this report? The author is told it was removed after review.",
		"Keep report clears the flag. The report stays. Remove report hides it from everyone and tells the author.",
		"Keep report clears the flags on it. The report stays. Remove report hides it from everyone and tells the author.",
		"Keep report accepts the text as it is and lets the report through the normal rules. Remove report hides it from everyone and tells the author.",
		"Remove report hides it from everyone and tells the author.",
	}
	for _, s := range once {
		if n := p426CountTemplateText(tmpl, s) + p426CountQuoted(js, s); n != 1 {
			t.Errorf("%q appears %d times across the template and the script, want exactly 1", s, n)
		}
	}

	p426WantAll(t, "composed fragments", js, `'No report found with id '`, `'. Check the id and try again.'`,
		`'Found report '`, `' kept.'`, `' removed.'`, `'Report '`, `'1 person flagged this.'`,
		`' people flagged this.'`, `'Posted '`)

	if n := strings.Count(tmpl, "<title>Pinalert: Moderator screen</title>"); n != 1 {
		t.Errorf("the title occurs %d times, want 1", n)
	}
	if !strings.Contains(tmpl, `includes report=</p>`) {
		t.Error("the find help text must be followed directly by </p> with no full stop")
	}

	self, err := os.ReadFile("moderate_contract_test.go")
	if err != nil {
		t.Fatalf("reading this test file: %v", err)
	}
	for name, text := range map[string]string{"template": tmpl, "moderate.js": js, "this test file": string(self)} {
		for _, r := range []rune{0x2014, 0x2013, 0x2026} {
			if strings.ContainsRune(text, r) {
				t.Errorf("%s contains %U", name, r)
			}
		}
	}
}

// TestModeratePageAssets (D-08, the recorded trust.js decision): the page
// loads the head assets and the four deferred scripts in a fixed order and
// carries every list container the script fills.
//
// Honest limit of this test's claim: it reads the template source, not the
// rendered page; 04-23's render test covers the rendered page.
func TestModeratePageAssets(t *testing.T) {
	tmpl := p426ReadTemplate(t)
	const v = "?v={{.AssetVersion}}"

	themeTag := findTagWindow(t, tmpl, "/static/js/theme.js"+v)
	if strings.Contains(themeTag, "defer") {
		t.Error("theme.js must not be deferred")
	}
	if n := strings.Count(tmpl, `<meta name="robots" content="noindex">`); n != 1 {
		t.Errorf("robots meta occurs %d times, want 1", n)
	}
	p426InOrder(t, "stylesheets", tmpl, `/static/css/main.css`+v, `/static/css/trust.css`+v, `/static/css/auth.css`+v,
		`/static/css/chrome.css`+v)
	p426WantNone(t, "template", tmpl, "feed.css", "modal.css", "modal-map.css", "low.css", "legal.css", "unpkg")

	mainEnd := strings.Index(tmpl, "</main>")
	if mainEnd < 0 {
		t.Fatal("no </main>")
	}
	scripts := []string{"app.js", "trust.js", "moderate.js", "account-menu.js"}
	last := mainEnd
	for _, s := range scripts {
		src := "/static/js/" + s + v
		if n := strings.Count(tmpl, src); n != 1 {
			t.Errorf("%s occurs %d times, want 1", src, n)
			continue
		}
		tag := findTagWindow(t, tmpl, src)
		if !strings.Contains(tag, " defer") {
			t.Errorf("%s must be deferred", s)
		}
		idx := strings.Index(tmpl, src)
		if idx <= last {
			t.Errorf("%s is out of order or before </main>", s)
		}
		last = idx
	}
	for _, s := range []string{"votes.js", "visibility.js", "feed.js", "map.js", "modal.js", "queue.js", "flag.js",
		"report-form.js", "low.js", "slow-notice.js"} {
		if strings.Contains(tmpl, "/static/js/"+s) {
			t.Errorf("the moderator page must not load %s", s)
		}
	}

	toast := findTagWindow(t, tmpl, `id="toast"`)
	p426WantAll(t, "toast", toast, `role="status"`, `aria-live="polite"`, "hidden")
	if n := strings.Count(tmpl, `id="toast"`); n != 1 {
		t.Errorf(`id="toast" occurs %d times, want 1`, n)
	}
	result := findTagWindow(t, tmpl, `id="mod-find-result"`)
	p426WantAll(t, "find result", result, `aria-live="polite"`)
	p426WantNone(t, "find result", result, "role=")
	refreshErr := findTagWindow(t, tmpl, `id="mod-refresh-error"`)
	p426WantAll(t, "refresh error", refreshErr, `class="inline-error"`, `role="alert"`, "hidden")
	input := findTagWindow(t, tmpl, `id="mod-find"`)
	p426WantAll(t, "find input", input, `type="text"`, `autocomplete="off"`, `autocapitalize="off"`,
		`spellcheck="false"`, `aria-describedby="mod-find-help"`)
	p426WantAll(t, "refresh button", findTagWindow(t, tmpl, `id="mod-refresh"`), `type="button"`)
	p426WantAll(t, "find button", findTagWindow(t, tmpl, ">Find report<"), `type="submit"`, `class="btn btn--primary"`)

	ids := []string{"mod-refresh", "mod-refresh-error", "mod-find-form", "mod-find", "mod-find-help", "mod-find-result",
		"mod-h-auto", "mod-h-people", "mod-h-pending"}
	for _, slug := range []string{"auto", "people", "pending"} {
		ids = append(ids, "mod-count-"+slug, "mod-list-"+slug, "mod-empty-"+slug)
	}
	for _, id := range ids {
		if n := strings.Count(tmpl, `id="`+id+`"`); n != 1 {
			t.Errorf(`id="%s" occurs %d times, want 1`, id, n)
		}
	}
	for _, slug := range []string{"auto", "people", "pending"} {
		list := findTagWindow(t, tmpl, `id="mod-list-`+slug+`"`)
		if !strings.HasPrefix(list, "<ol") {
			t.Errorf("mod-list-%s must be an ol, got %q", slug, list)
		}
		p426WantAll(t, "list "+slug, list, `class="mod-list"`, `role="list"`, `aria-labelledby="mod-h-`+slug+`"`)
		empty := findTagWindow(t, tmpl, `id="mod-empty-`+slug+`"`)
		p426WantAll(t, "empty "+slug, empty, `class="mod-empty"`, "hidden")
	}
	p426InOrder(t, "headings", tmpl, `id="mod-h-auto"`, `id="mod-h-people"`, `id="mod-h-pending"`)
	if n := strings.Count(tmpl, "<h1"); n != 1 {
		t.Errorf("<h1 occurs %d times, want 1", n)
	}
	if n := strings.Count(tmpl, "<h2"); n != 3 {
		t.Errorf("<h2 occurs %d times, want 3", n)
	}
	if n := strings.Count(tmpl, `class="mod-section"`); n != 3 {
		t.Errorf(`class="mod-section" occurs %d times, want 3`, n)
	}
	if n := strings.Count(tmpl, `class="mod-section__line"`); n != 3 {
		t.Errorf(`class="mod-section__line" occurs %d times, want 3`, n)
	}
}

// p426MediaBlock returns the text inside the braces of the at-rule that starts
// with anchor (for example "@media (min-width: 360px)"), counting braces. It
// returns an empty string when the anchor is absent.
func p426MediaBlock(css, anchor string) string {
	start := strings.Index(css, anchor)
	if start < 0 {
		return ""
	}
	open := strings.Index(css[start:], "{")
	if open < 0 {
		return ""
	}
	begin := start + open + 1
	depth := 1
	for i := begin; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return css[begin:i]
			}
		}
	}
	return ""
}

// p426WithoutBlock removes the at-rule that starts with anchor, braces and all.
func p426WithoutBlock(css, anchor string) string {
	inner := p426MediaBlock(css, anchor)
	if inner == "" {
		return css
	}
	start := strings.Index(css, anchor)
	end := start + strings.Index(css[start:], "{") + 1 + len(inner) + 1
	return css[:start] + css[end:]
}

// p426RuleWithSelector returns the first rule whose head, after dropping any
// at-rule preamble up to the last open brace and splitting on commas, has a
// trimmed part exactly equal to sel.
func p426RuleWithSelector(rules []cssRule, sel string) (cssRule, bool) {
	for _, r := range rules {
		head := r.selectorHead
		if i := strings.LastIndex(head, "{"); i >= 0 {
			head = head[i+1:]
		}
		for _, part := range strings.Split(head, ",") {
			if strings.TrimSpace(part) == sel {
				return r, true
			}
		}
	}
	return cssRule{}, false
}

// TestModerateStylesheet (P4-A22, D-08): the page links moderate.css after
// chrome.css, and the stylesheet keeps to tokens, the 640px phone frame, neutral
// rows, the hidden guard and the two action tracks.
//
// Honest limit of this test's claim: it reads declarations out of the source;
// it cannot prove how the page looks at 320 and 360 px, which the human check
// in this task does.
func TestModerateStylesheet(t *testing.T) {
	tmpl := p426ReadTemplate(t)
	const link = `<link rel="stylesheet" href="/static/css/moderate.css?v={{.AssetVersion}}">`
	if n := strings.Count(tmpl, link); n != 1 {
		t.Fatalf("the moderate.css link occurs %d times, want 1", n)
	}
	chrome := strings.Index(tmpl, `/static/css/chrome.css`)
	if at := strings.Index(tmpl, link); at < chrome || at > strings.Index(tmpl, "</head>") {
		t.Errorf("the moderate.css link must follow chrome.css inside the head")
	}

	raw, err := StaticFS.ReadFile("static/css/moderate.css")
	if err != nil {
		t.Fatalf("reading moderate.css: %v", err)
	}
	for _, r := range []rune{0x2014, 0x2013, 0x2026} {
		if strings.ContainsRune(string(raw), r) {
			t.Errorf("moderate.css contains %U", r)
		}
	}

	css := p426ReadCSS(t)
	p426WantNone(t, "moderate.css", css, "!important", "rgb(", "rgba(", "hsl(", "999px", "transition", "animation",
		"--severity", "--color-severity")

	const mediaMin, mediaForced = "@media (min-width: 360px)", "@media (forced-colors: active)"
	minBlock := p426MediaBlock(css, mediaMin)
	forcedBlock := p426MediaBlock(css, mediaForced)
	if minBlock == "" || forcedBlock == "" {
		t.Fatal("moderate.css needs the 360px block and the forced colors block")
	}
	outside := p426WithoutBlock(p426WithoutBlock(css, mediaMin), mediaForced)
	rules := parseCSSRules(outside)

	hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	fontSize := regexp.MustCompile(`^var\(--font-size-(label|body|heading|display)\)$`)
	fontWeight := regexp.MustCompile(`^var\(--font-weight-(regular|semibold)\)$`)
	system := regexp.MustCompile(`\b(CanvasText|Canvas|LinkText|GrayText|ButtonFace|ButtonText)\b`)
	for _, r := range parseCSSRules(css) {
		decls := declsOf(r.declBody)
		for name, value := range decls {
			if strings.HasPrefix(name, "--") {
				t.Errorf("%s declares the custom property %s", r.selectorHead, name)
			}
			if hex.MatchString(value) {
				t.Errorf("%s: %s holds a hex colour", r.selectorHead, name)
			}
			if name == "font-size" && !fontSize.MatchString(value) {
				t.Errorf("%s: font-size %q is not a token", r.selectorHead, value)
			}
			if name == "font-weight" && !fontWeight.MatchString(value) {
				t.Errorf("%s: font-weight %q is not a token", r.selectorHead, value)
			}
		}
		if display, ok := decls["display"]; ok {
			if strings.Contains(r.selectorHead, "summary") {
				t.Errorf("%s declares display on a summary", r.selectorHead)
			}
			if display != "none" && strings.Contains(r.selectorHead, ".mod-") && !strings.Contains(r.selectorHead, ":not([hidden])") {
				t.Errorf("%s declares display without :not([hidden])", r.selectorHead)
			}
		}
	}
	if system.MatchString(outside) || system.MatchString(minBlock) {
		t.Error("system colour keywords belong only inside the forced colors block")
	}

	need := func(sel string) map[string]string {
		t.Helper()
		r, ok := p426RuleWithSelector(rules, sel)
		if !ok {
			t.Errorf("no rule for %q", sel)
			return map[string]string{}
		}
		return declsOf(r.declBody)
	}
	want := func(sel string, pairs ...string) map[string]string {
		t.Helper()
		d := need(sel)
		for i := 0; i+1 < len(pairs); i += 2 {
			if d[pairs[i]] != pairs[i+1] {
				t.Errorf("%s: %s = %q, want %q", sel, pairs[i], d[pairs[i]], pairs[i+1])
			}
		}
		return d
	}

	page := want(".mod-page", "max-width", "640px", "margin", "0 auto", "padding", "var(--space-md)")
	for _, token := range []string{"--space-lg", "--touch-target-min", "--space-md"} {
		if !strings.Contains(page["padding-bottom"], token) {
			t.Errorf(".mod-page padding-bottom %q lacks %s", page["padding-bottom"], token)
		}
	}
	want(".mod-section", "margin-top", "var(--space-xl)")
	need(".mod-section__line")
	want(".mod-row", "background", "var(--color-bg)", "border", "1px solid var(--color-border)",
		"border-radius", "8px", "padding", "var(--space-md)")
	want(".mod-row:not([hidden])", "display", "flex", "flex-direction", "column", "gap", "var(--space-sm)")
	text := want(".mod-row__text", "overflow-wrap", "anywhere", "font-size", "var(--font-size-body)")
	for _, banned := range []string{"max-height", "overflow", "-webkit-line-clamp", "line-clamp"} {
		if _, has := text[banned]; has {
			t.Errorf(".mod-row__text declares %s, but the text is never clamped", banned)
		}
	}
	want(".mod-find input", "border-radius", "8px")
	want(".mod-row__actions:not([hidden])", "display", "grid", "grid-template-columns", "minmax(0, 1fr)")
	want(".mod-confirm__buttons:not([hidden])", "display", "grid", "grid-template-columns", "minmax(0, 1fr)")
	for _, sel := range []string{".mod-row__actions:not([hidden])", ".mod-confirm__buttons:not([hidden])"} {
		r, ok := p426RuleWithSelector(parseCSSRules(minBlock), sel)
		if !ok {
			t.Errorf("the 360px block has no rule for %q", sel)
			continue
		}
		if got := declsOf(r.declBody)["grid-template-columns"]; got != "repeat(2, minmax(0, 1fr))" {
			t.Errorf("%s at 360px: grid-template-columns = %q", sel, got)
		}
	}
	want(".mod-row__all-scores summary", "min-height", "var(--touch-target-min)")
	want(".mod-row__link:not([hidden])", "min-height", "var(--touch-target-min)")

	trustRaw, err := StaticFS.ReadFile("static/css/trust.css")
	if err != nil {
		t.Fatalf("reading trust.css: %v", err)
	}
	tag, ok := ruleBySelector(parseCSSRules(string(trustRaw)), ".visibility-tag:not([hidden])")
	if !ok || declsOf(tag.declBody)["border-radius"] == "" {
		t.Fatal("trust.css has no .visibility-tag:not([hidden]) rule with a radius")
	}
	want(".mod-row__state:not([hidden])", "border-radius", declsOf(tag.declBody)["border-radius"])

	forced := parseCSSRules(forcedBlock)
	for _, sel := range []string{".mod-row", ".mod-row__state:not([hidden])"} {
		r, ok := p426RuleWithSelector(forced, sel)
		if !ok {
			t.Errorf("the forced colors block has no rule for %q", sel)
			continue
		}
		if got := declsOf(r.declBody)["border"]; got != "1px solid CanvasText" {
			t.Errorf("%s in forced colors: border = %q", sel, got)
		}
	}
}

// TestModerateNotLinkedAnywhere (open item 20, T-04-06): no template and no
// script links to the moderator screen, and the account menu does not name it.
//
// Honest limit of this test's claim: it finds the written address forms; a link
// assembled from pieces at run time would not be seen, and none is written.
func TestModerateNotLinkedAnywhere(t *testing.T) {
	templates, scripts := 0, 0
	forbidden := []string{`href="/moderate`, `"/moderate"`, `'/moderate'`}
	err := fs.WalkDir(StaticFS, "static/js", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := StaticFS.ReadFile(path)
		if err != nil {
			return err
		}
		scripts++
		code := stripJSComments(string(raw))
		for _, f := range forbidden {
			if strings.Contains(code, f) {
				t.Errorf("%s links to the moderator screen (%s)", path, f)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking static/js: %v", err)
	}
	err = fs.WalkDir(TemplatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := TemplatesFS.ReadFile(path)
		if err != nil {
			return err
		}
		templates++
		for _, f := range forbidden {
			if strings.Contains(string(raw), f) {
				t.Errorf("%s links to the moderator screen (%s)", path, f)
			}
		}
		if strings.HasSuffix(path, "account_header.html.tmpl") && strings.Contains(string(raw), "moderat") {
			t.Errorf("%s names the moderator screen", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking templates: %v", err)
	}
	if templates < 8 || scripts < 10 {
		t.Errorf("scanned %d templates and %d scripts, want at least 8 and 10", templates, scripts)
	}
	t.Logf("scanned %d templates and %d scripts", templates, scripts)
}
