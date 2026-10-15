package web

import (
	"regexp"
	"strings"
	"testing"
)

// This file pins plan 04-22: the device queue for reports that could not be
// sent (D-18, D-19, D-20), the shared form's queue path, Send after the wait
// and the held toast (D-12, D-02, P4-A12, P4-A13), and the account binding of
// every saved row (T-04-14).
//
// Honest limit of these tests: they inspect the shipped scripts, templates and
// stylesheet as text. IndexedDB, timing, two tabs and real networks are covered
// by the end of phase human check (plan 04-27) and, for the replay of a repeated
// request id, by the server side tests of plan 04-17.

// p422Static returns the raw text of an embedded static file.
func p422Static(t *testing.T, path string) string {
	t.Helper()
	raw, err := StaticFS.ReadFile(path)
	if err != nil {
		t.Fatalf("reading embedded %s: %v", path, err)
	}
	return string(raw)
}

// p422Template returns the raw text of an embedded template.
func p422Template(t *testing.T, name string) string {
	t.Helper()
	raw, err := TemplatesFS.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("reading embedded template %s: %v", name, err)
	}
	return string(raw)
}

// p422Code returns a script with its comments removed.
func p422Code(raw string) string {
	return stripJSComments(raw)
}

// p422Window returns the text from anchor up to (not including) terminator and
// fails the test when either end is missing.
func p422Window(t *testing.T, text, anchor, terminator string) string {
	t.Helper()
	body, anchorFound, terminatorFound := windowAfter(text, anchor, terminator)
	if !anchorFound {
		t.Fatalf("could not locate the anchor %q", anchor)
	}
	if !terminatorFound {
		t.Fatalf("could not locate the terminator %q after %q", terminator, anchor)
	}
	return body
}

// p422Count counts the occurrences of sub in text.
func p422Count(text, sub string) int {
	return strings.Count(text, sub)
}

// p422PlainCopy fails the test when s holds a character outside the plain set
// of the queue copy: ASCII letters, digits, space, comma, period, straight
// apostrophe, question mark, colon, parentheses and equals sign.
func p422PlainCopy(t *testing.T, s string) {
	t.Helper()
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune(" ,.'?:()=", r):
		default:
			t.Errorf("copy %q holds the character %q outside the plain set", s, r)
			return
		}
	}
}

// p422Words splits text into words on every non letter and on each lower case
// to upper case boundary, lower cased.
func p422Words(text string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	var prev rune
	for _, r := range text {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !isLetter {
			flush()
			prev = r
			continue
		}
		if r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z' {
			flush()
		}
		cur = append(cur, r)
		prev = r
	}
	flush()
	return words
}

// p422Tag returns the opening tag of the first element that holds marker:
// from the last "<" before marker to the next ">" after it.
func p422Tag(t *testing.T, text, marker string) string {
	t.Helper()
	at := strings.Index(text, marker)
	if at == -1 {
		t.Fatalf("could not locate %q", marker)
	}
	start := strings.LastIndex(text[:at], "<")
	end := strings.Index(text[at:], ">")
	if start == -1 || end == -1 {
		t.Fatalf("no opening tag around %q", marker)
	}
	return text[start : at+end+1]
}

// p422Ordered fails the test unless every needle occurs in text in the given
// order.
func p422Ordered(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	last := -1
	for _, n := range needles {
		at := strings.Index(text, n)
		if at == -1 {
			t.Errorf("%s: %q is missing", label, n)
			return
		}
		if at <= last {
			t.Errorf("%s: %q is out of order", label, n)
			return
		}
		last = at
	}
}

// p422Need fails once per missing needle.
func p422Need(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s is missing %q", label, n)
		}
	}
}

// p422Lack fails once per forbidden needle that is present.
func p422Lack(t *testing.T, label, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(text, n) {
			t.Errorf("%s must not contain %q", label, n)
		}
	}
}

// p422CopyConstants is the copy table of queue.js, name and exact quoted value.
var p422CopyConstants = []struct{ name, literal string }{
	{"QUEUE_HEADING", `'Waiting to send'`},
	{"SEND_NOW_LABEL", `'Send now'`},
	{"SENDING_LABEL", `'Sending'`},
	{"COUNT_ONE_SUFFIX", `' report saved on this device.'`},
	{"COUNT_MANY_SUFFIX", `' reports saved on this device.'`},
	{"HELP_LINE", `'Reports send by themselves when you have a signal and this page is open.'`},
	{"HELP_LINE_MEMORY", `"This browser can't keep reports for later. Keep this page open until they send."`},
	{"STATUS_TEXT_WAITING", `'Waiting for a signal.'`},
	{"STATUS_TEXT_SENDING", `'Sending.'`},
	{"STATUS_TEXT_NO_ANSWER", `"Pinalert didn't answer. Trying again."`},
	{"NOT_SENT_PREFIX", `'Not sent. '`},
	{"BLOCKED_PREFIX", `'Waiting to send. It will send by itself in '`},
	{"EMERGENCY_CALL_PREFIX", `'In an emergency, call '`},
	{"EMERGENCY_CALL_SUFFIX", `' now.'`},
	{"CALL_LABEL_PREFIX", `'Call '`},
	{"STATUS_TEXT_SIGNED_OUT", `'Sign in again to send this report. It stays saved on this device.'`},
	{"SIGN_IN_LABEL", `'Sign in again'`},
	{"EDIT_LABEL", `'Edit and resend'`},
	{"DISCARD_LABEL", `'Discard report'`},
	{"KEEP_LABEL", `'Keep saved report'`},
	{"CONFIRM_DISCARD_LABEL", `'Yes, discard report'`},
	{"DISCARD_QUESTION", `"Discard this saved report? It won't be sent."`},
	{"SENT_TOAST", `'Report sent.'`},
	{"HELD_TOAST_MESSAGE", `"Report received. It isn't shown yet. A text check has to finish first."`},
	{"DISCARDED_TOAST", `'Saved report discarded.'`},
	{"GENERIC_REASON", `'Something went wrong. Try again.'`},
	{"ANNOUNCE_ONE_SUFFIX", `' report waiting to send.'`},
	{"ANNOUNCE_MANY_SUFFIX", `' reports waiting to send.'`},
	{"ANNOUNCE_NOT_SENT_PREFIX", `'Report not sent. '`},
	{"ANNOUNCE_BLOCKED_PREFIX", `'Report waiting to send. It will send by itself in '`},
	{"ANNOUNCE_SIGNED_OUT", `'Report waiting. Sign in again to send it.'`},
}

func TestQueueNoMarkupSinksAndNoAgeDiscard(t *testing.T) {
	code := p422Code(p422Static(t, "static/js/queue.js"))
	for _, sink := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(code, sink) {
			t.Errorf("queue.js uses the markup parsing sink %s", sink)
		}
	}
	// Provenance: the heading, the count line, the help line, the description
	// and the status line each set text through it, so fewer means a text path
	// bypassed it.
	if n := p422Count(code, "Pinalert.setText("); n < 5 {
		t.Errorf("queue.js writes text through Pinalert.setText %d times, want at least 5", n)
	}
	lower := strings.ToLower(code)
	for _, banned := range []string{"expir", "maxage", "max_age"} {
		if strings.Contains(lower, banned) {
			t.Errorf("queue.js holds the age vocabulary %q", banned)
		}
	}
	for _, w := range p422Words(code) {
		if w == "ttl" || w == "age" {
			t.Errorf("queue.js holds the age word %q", w)
		}
	}

	if n := p422Count(code, ".delete("); n != 1 {
		t.Errorf("queue.js has %d store delete calls, want exactly 1", n)
	}
	remove := jsFunctionBody(t, code, "removeRow(requestId)")
	if !strings.Contains(remove, ".delete(") {
		t.Errorf("the one store delete call must sit inside removeRow")
	}
	if n := p422Count(code, "removeRow("); n != 3 {
		t.Errorf("removeRow( occurs %d times, want 3 (the definition, the sent branch, discard)", n)
	}
	for _, name := range []string{"sendRow(record)", "discard(requestId)"} {
		if !strings.Contains(jsFunctionBody(t, code, name), "removeRow(") {
			t.Errorf("%s must call removeRow", name)
		}
	}
}

func TestQueueBoundToAccount(t *testing.T) {
	code := p422Code(p422Static(t, "static/js/queue.js"))
	p422Need(t, "queue.js", code,
		"indexedDB.open(DB_NAME, DB_VERSION)",
		"DB_NAME = 'pinalert-queue'",
		"DB_VERSION = 1",
		"STORE_NAME = 'reports'",
		"keyPath: 'request_id'",
		"createIndex(ACCOUNT_INDEX, 'account_id'",
		"IDBKeyRange.only(accountId)",
	)
	if !strings.Contains(jsFunctionBody(t, code, "belongsToAccount(record)"), "record.account_id === accountId") {
		t.Errorf("belongsToAccount must compare record.account_id === accountId")
	}
	for _, name := range []string{"loadRows()", "get(requestId)", "sendRow(record)", "renderStrip()", "openResendFromAddress()"} {
		if !strings.Contains(jsFunctionBody(t, code, name), "belongsToAccount(") {
			t.Errorf("%s must re-check belongsToAccount", name)
		}
	}
	save := jsFunctionBody(t, code, "save(record)")
	if !strings.Contains(save, "account_id = accountId") && !strings.Contains(save, "account_id: accountId") {
		t.Errorf("save must stamp the mounted account id on the row")
	}
	mount := jsFunctionBody(t, code, "mount(options)")
	p422Need(t, "mount(options)", mount, "/^[0-9]+$/", "String(")
	if strings.Contains(strings.ToLower(code), "email") {
		t.Errorf("queue.js must never mention the email")
	}
}

func TestQueueSendsThroughSubmitReport(t *testing.T) {
	code := p422Code(p422Static(t, "static/js/queue.js"))
	p422Lack(t, "queue.js", code, "fetch(", "XMLHttpRequest", "sendBeacon", "/api/", "PinalertVotes", "castVote")
	if n := p422Count(code, "Pinalert.submitReport("); n != 1 {
		t.Errorf("queue.js calls Pinalert.submitReport %d times, want exactly 1", n)
	}
	if !strings.Contains(jsFunctionBody(t, code, "sendRow(record)"), "Pinalert.submitReport(") {
		t.Errorf("the one Pinalert.submitReport call must sit inside sendRow")
	}
	// Provenance: the pinned count of plan 04-13, the list request and the post.
	if n := p422Count(p422Static(t, "static/js/app.js"), "fetch("); n != 2 {
		t.Errorf("app.js has %d fetch( occurrences, want exactly 2", n)
	}
}

func TestQueueSubmitReportClassifiesQueueable(t *testing.T) {
	raw := p422Static(t, "static/js/app.js")
	app := p422Code(raw)
	b := jsFunctionBody(t, app, "submitReport(payload)")
	p422Need(t, "submitReport(payload)", b,
		"requestTimeout(SUBMIT_TIMEOUT_MS)",
		"signal: timeout.signal",
		"err.status = res.status",
		"QUEUEABLE_STATUSES.indexOf(res.status) !== -1",
		"retry_after_seconds",
		"'Retry-After'",
		"err.retryAfterSeconds",
		"body.serious === true",
		"err.serious",
		"emergency_number",
		"err.emergencyNumber",
		"err.queueable = true",
	)
	// The res.json fallback is the only .catch, so the no answer handler cannot
	// be a catch after the answer handler (that form would turn every thrown
	// 400, 422, 429 or 500 into a queueable no answer error).
	if n := p422Count(b, ".catch("); n != 1 {
		t.Errorf("submitReport has %d .catch( calls, want exactly 1", n)
	}
	first := strings.Index(b, "err.queueable = QUEUEABLE_STATUSES.indexOf(res.status) !== -1")
	second := strings.Index(b, "err.queueable = true")
	if first == -1 || second == -1 || second < first {
		t.Errorf("the no answer handler must come after the answer handler (%d, %d)", first, second)
	}
	if n := p422Count(app, "var QUEUEABLE_STATUSES = [502, 503, 504];"); n != 1 {
		t.Errorf("QUEUEABLE_STATUSES declared %d times, want 1", n)
	}
	if n := p422Count(app, "var SUBMIT_TIMEOUT_MS = 10000;"); n != 1 {
		t.Errorf("SUBMIT_TIMEOUT_MS declared %d times, want 1", n)
	}
	if n := p422Count(app, "function submitReport("); n != 1 {
		t.Errorf("function submitReport( occurs %d times, want 1", n)
	}
	if n := p422Count(app, "submitReport: submitReport"); n != 1 {
		t.Errorf("submitReport is exported %d times, want 1", n)
	}
	tail := p422Window(t, b, "fetchReports(", "\n  }")
	if n := p422Count(tail+"\n", "return report;"); n < 2 {
		t.Errorf("both refetch handlers must resolve the created report, found %d returns", n)
	}
}

func TestQueueFlushTriggers(t *testing.T) {
	code := p422Code(p422Static(t, "static/js/queue.js"))
	p422Need(t, "queue.js", code,
		"QUEUE_FLUSH_MS = 30000",
		"WAIT_TICK_MS = 60000",
		"LOCK_NAME = 'pinalert-queue-flush'",
		"navigator.locks",
		"ifAvailable: true",
		"SENDING_LEASE_MS",
		"sending_until",
	)
	if n := p422Count(code, "addEventListener('online'"); n != 1 {
		t.Errorf("the online listener is registered %d times, want 1", n)
	}
	if !regexp.MustCompile(`setInterval\([^;]*QUEUE_FLUSH_MS\)`).MatchString(code) {
		t.Errorf("the flush must run on a QUEUE_FLUSH_MS interval")
	}
	if !regexp.MustCompile(`setInterval\([^;]*WAIT_TICK_MS\)`).MatchString(code) {
		t.Errorf("the wait refresh must run on a WAIT_TICK_MS interval")
	}
	if !strings.Contains(jsFunctionBody(t, code, "mount(options)"), "flush()") {
		t.Errorf("mount must flush once (the page load trigger)")
	}
	click := p422Window(t, code, "sendBtn.addEventListener('click'", "headEl.appendChild(sendBtn)")
	if !strings.Contains(click, "runFlush(true)") {
		t.Errorf("the Send now click must call runFlush(true)")
	}
	load := jsFunctionBody(t, code, "loadRows()")
	p422Need(t, "loadRows()", load, ".sort(", "written_at")
	run := jsFunctionBody(t, code, "runFlush(manual)")
	p422Need(t, "runFlush(manual)", run, "STATUS_REJECTED", "STATUS_SIGNED_OUT", "manual")
	render := jsFunctionBody(t, code, "renderStrip()")
	if n := p422Count(render, "dispatchEvent("); n != 1 {
		t.Errorf("renderStrip dispatches %d events, want exactly 1", n)
	}
	hide := strings.Index(render, "stripEl.hidden = true")
	event := strings.Index(render, "new CustomEvent(COUNT_EVENT")
	if hide == -1 || event == -1 || event < hide {
		t.Errorf("the count event must fire on every render, the empty one included (%d, %d)", hide, event)
	}
	// The queue decides from real request outcomes only (D-18).
	if strings.Contains(code, "navigator."+"onLine") {
		t.Errorf("queue.js must never read the browser's online flag")
	}
}

func TestQueueStatusCopy(t *testing.T) {
	raw := p422Static(t, "static/js/queue.js")
	code := p422Code(raw)
	for _, c := range p422CopyConstants {
		decl := "var " + c.name + " = " + c.literal + ";"
		if n := p422Count(code, decl); n != 1 {
			t.Errorf("%s: %q declared %d times, want exactly 1", c.name, decl, n)
		}
		p422PlainCopy(t, c.literal[1:len(c.literal)-1])
	}
	for _, dash := range []string{string(rune(0x2014)), string(rune(0x2013))} {
		if strings.Contains(raw, dash) {
			t.Errorf("queue.js contains the dash character %q", dash)
		}
	}
	for _, w := range p422Words(raw) {
		if w == "li"+"ght" || w == "li"+"te" {
			t.Errorf("queue.js holds the forbidden word %q", w)
		}
	}

	callBody := jsFunctionBody(t, code, "appendEmergencyCall(el, number)")
	announceBody := jsFunctionBody(t, code, "blockedAnnouncement(record)")
	rest := strings.Replace(code, callBody, "", 1)
	rest = strings.Replace(rest, announceBody, "", 1)
	if n := p422Count(rest, "EMERGENCY_CALL_PREFIX"); n != 1 {
		t.Errorf("EMERGENCY_CALL_PREFIX occurs %d times outside the two builders, want only its declaration", n)
	}
	p422Need(t, "blockedAnnouncement(record)", announceBody, "record.serious === true", "record.emergency_number")
	p422Need(t, "appendEmergencyCall(el, number)", callBody,
		"document.createElement('a')",
		"setAttribute('href', 'tel:' + number)",
		"setAttribute('aria-label', CALL_LABEL_PREFIX + number)",
		"Pinalert.setText(",
	)
	calls := 0
	for from := 0; ; {
		at := strings.Index(code[from:], "appendEmergencyCall(")
		if at == -1 {
			break
		}
		at += from
		from = at + len("appendEmergencyCall(")
		if strings.HasSuffix(code[:at], "function ") {
			continue
		}
		calls++
		tail := code[at:]
		if end := strings.Index(tail, ";"); end != -1 {
			tail = tail[:end]
		}
		if !strings.Contains(tail, "record.emergency_number") {
			t.Errorf("an appendEmergencyCall call must pass record.emergency_number: %q", tail)
		}
		start := at - 300
		if start < 0 {
			start = 0
		}
		if !strings.Contains(code[start:at], "record.serious === true") {
			t.Errorf("an appendEmergencyCall call must sit under a record.serious === true check")
		}
	}
	if calls == 0 {
		t.Errorf("no call of appendEmergencyCall was found")
	}
	if strings.Contains(code, "'112'") || strings.Contains(code, `"112"`) {
		t.Errorf("queue.js must never type the emergency number")
	}
}

func TestQueueStripCSS(t *testing.T) {
	raw := p422Static(t, "static/css/chrome.css")
	rules := parseCSSRules(stripCSSComments(raw))
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	byHead := func(head string) (map[string]string, bool) {
		for _, r := range rules {
			if norm(r.selectorHead) == head {
				return declsOf(r.declBody), true
			}
		}
		return nil, false
	}
	need := func(head string, want map[string]string) {
		t.Helper()
		d, ok := byHead(head)
		if !ok {
			t.Errorf("chrome.css has no rule %q", head)
			return
		}
		for k, v := range want {
			if d[k] != v {
				t.Errorf("%q: %s is %q, want %q", head, k, d[k], v)
			}
		}
	}
	need(".queue-strip", map[string]string{
		"background":    "var(--color-surface)",
		"border":        "1px solid var(--color-border)",
		"border-radius": "8px",
		"padding":       "var(--space-md)",
	})
	need(".app-shell .queue-strip", map[string]string{"margin": "var(--space-md)"})
	need(".queue-strip__text:not([hidden])", map[string]string{
		"-webkit-line-clamp": "3",
		"overflow-wrap":      "anywhere",
	})
	need(".queue-strip__call, .form-error__call", map[string]string{
		"color":           "var(--color-text)",
		"text-decoration": "underline",
	})
	need(".low-page__head .queue-strip__heading", map[string]string{"margin": "0"})
	need(".low-page__head .queue-strip .btn", map[string]string{"margin": "0"})

	forced := false
	for _, r := range rules {
		head := norm(r.selectorHead)
		if strings.Contains(head, ".queue-strip") {
			d := declsOf(r.declBody)
			if v, has := d["display"]; has && v != "none" && !strings.Contains(head, ":not([hidden])") {
				t.Errorf("%q declares display %q without :not([hidden])", head, v)
			}
			if strings.Contains(r.declBody, "var(--color-text-muted)") {
				t.Errorf("%q uses the muted text colour", head)
			}
		}
		if strings.Contains(head, "(forced-colors: active)") && strings.HasSuffix(head, "{ .queue-strip") &&
			strings.Contains(r.declBody, "CanvasText") {
			forced = true
		}
	}
	if !forced {
		t.Errorf("chrome.css has no forced colors .queue-strip border with CanvasText")
	}
	spans := p408MediaSpans(stripCSSComments(raw), "(forced-colors: active)")
	if len(spans) < 3 {
		t.Errorf("chrome.css holds %d forced colors blocks, want at least 3", len(spans))
	}
}

func TestQueuePagesWiring(t *testing.T) {
	index := p422Template(t, "index.html.tmpl")

	shell := p422Tag(t, index, `id="app-shell"`)
	if !strings.Contains(shell, `data-account-id="{{.AccountID}}"`) {
		t.Errorf("the main #app-shell must carry data-account-id from AccountID: %s", shell)
	}
	const strip = `<div class="queue-strip" id="queue-strip" hidden></div>`
	if n := p422Count(index, strip); n != 1 {
		t.Fatalf("index.html.tmpl has %d queue strips, want exactly 1", n)
	}
	stripAt := strings.Index(index, strip)
	filterAt := strings.Index(index, `id="filter-line"`)
	staleAt := strings.Index(index, `id="feed-stale"`)
	if !(filterAt != -1 && staleAt != -1 && filterAt < stripAt && stripAt < staleAt) {
		t.Errorf("the strip must sit after #filter-line and before #feed-stale (%d, %d, %d)", filterAt, stripAt, staleAt)
	}
	staleOpen := strings.LastIndex(index[:staleAt], "<")
	if gap := index[stripAt+len(strip) : staleOpen]; strings.TrimSpace(gap) != "" {
		t.Errorf("only whitespace may sit between the strip and #feed-stale, found %q", gap)
	}
	p422Ordered(t, "index.html.tmpl scripts", index,
		"/static/js/trust.js", "/static/js/queue.js", "/static/js/report-form.js")
	if n := p422Count(index, `/static/js/queue.js?v={{.AssetVersion}}" defer`); n != 1 {
		t.Errorf("the queue.js tag occurs %d times, want 1", n)
	}
	if n := p422Count(index, `id="notice-live"`); n != 1 {
		t.Errorf("#notice-live occurs %d times on the main page, want 1", n)
	}

	feed := p422Code(p422Static(t, "static/js/feed.js"))
	if n := p422Count(feed, "PinalertQueue.mount("); n != 1 {
		t.Fatalf("feed.js mounts the queue %d times, want 1", n)
	}
	call := p422Window(t, feed, "PinalertQueue.mount(", "});")
	p422Need(t, "the feed.js queue mount", call,
		"headingLevel: 2", "onEdit: openSavedReport", "focusFallback: queueFocusFallback")
	around := p422Window(t, feed, "if (window.PinalertQueue", "});")
	p422Need(t, "the feed.js queue mount window", around, "data-account-id")
}

func TestQueueTimeoutFallback(t *testing.T) {
	app := p422Code(p422Static(t, "static/js/app.js"))
	timeout := jsFunctionBody(t, app, "requestTimeout(ms)")
	p422Need(t, "requestTimeout(ms)", timeout,
		"typeof AbortSignal", "AbortSignal.timeout(ms)", "new AbortController()", "setTimeout(", "clearTimeout(")
	submit := jsFunctionBody(t, app, "submitReport(payload)")
	p422Need(t, "submitReport(payload)", submit, "err.queueable = true")
	p422Lack(t, "submitReport(payload)", submit, ".name ===", ".name==")

	form := p422Code(p422Static(t, "static/js/report-form.js"))
	id := jsFunctionBody(t, form, "newRequestId()")
	p422Need(t, "newRequestId()", id, "crypto.randomUUID", "typeof", "getRandomValues", "Math.random")
	if n := p422Count(form, "newRequestId("); n != 2 {
		t.Errorf("newRequestId( occurs %d times, want 2 (the definition and one call)", n)
	}
	if !strings.Contains(jsFunctionBody(t, form, "ensureSubmission()"), "newRequestId()") {
		t.Errorf("the one call of newRequestId must sit inside ensureSubmission")
	}
}

func TestQueueSendAfterWaitOnlyAfter429(t *testing.T) {
	partial := p422Template(t, "report_form.html.tmpl")
	const button = `<button type="button" class="btn" id="send-after-wait" hidden>Send after the wait</button>`
	if n := p422Count(partial, button); n != 1 {
		t.Fatalf("the partial holds %d Send after the wait buttons, want exactly 1", n)
	}
	p422Ordered(t, "report_form.html.tmpl", partial, `id="form-error"`, button, `id="submit-report"`)

	form := p422Code(p422Static(t, "static/js/report-form.js"))
	if n := p422Count(form, "sendAfterWaitBtn.hidden = false"); n != 1 {
		t.Errorf("Send after the wait is shown in %d places, want exactly 1", n)
	}
	limit := jsFunctionBody(t, form, "showLimitMessage(err)")
	if !strings.Contains(limit, "sendAfterWaitBtn.hidden = false") {
		t.Errorf("the one place that shows Send after the wait must be showLimitMessage")
	}
	if n := p422Count(form, "showLimitMessage("); n != 2 {
		t.Errorf("showLimitMessage( occurs %d times, want 2 (the definition and one call)", n)
	}
	submit := jsFunctionBody(t, form, "handleSubmit()")
	branch := p422Window(t, submit, "err.status === 429", "} else")
	if !strings.Contains(branch, "showLimitMessage(err)") {
		t.Errorf("the 429 branch must call showLimitMessage")
	}
	for _, name := range []string{"resetForm()", "handleSubmit()"} {
		if !strings.Contains(jsFunctionBody(t, form, name), "sendAfterWaitBtn.hidden = true") {
			t.Errorf("%s must hide Send after the wait", name)
		}
	}
	p422Need(t, "saveAfterWait()", jsFunctionBody(t, form, "saveAfterWait()"),
		"status: 'blocked'",
		"next_try_at:",
		"lastLimit.retryAfterSeconds",
		"serious: lastLimit.serious",
		"emergency_number: lastLimit.emergencyNumber",
		"PinalertQueue.save(",
		"Pinalert.showToast(WAIT_SAVE_TOAST)",
	)
	p422Need(t, "showLimitMessage(err)", limit, "Pinalert.setText(formError")
	seriousAt := strings.Index(limit, "err.serious === true")
	callAt := strings.Index(limit, "appendEmergencyCall(formError, err.emergencyNumber)")
	if seriousAt == -1 || callAt == -1 || callAt < seriousAt {
		t.Errorf("the tap to call sentence must sit under an err.serious === true check (%d, %d)", seriousAt, callAt)
	}
	p422Need(t, "appendEmergencyCall(el, number)", jsFunctionBody(t, form, "appendEmergencyCall(el, number)"),
		"document.createElement('a')",
		"'form-error__call'",
		"setAttribute('href', 'tel:' + number)",
		"setAttribute('aria-label', CALL_LABEL_PREFIX + number)",
	)
}

func TestQueueHeldSubmitToast(t *testing.T) {
	form := p422Code(p422Static(t, "static/js/report-form.js"))
	queue := p422Code(p422Static(t, "static/js/queue.js"))
	const held = `"Report received. It isn't shown yet. A text check has to finish first."`
	p422Need(t, "report-form.js", form, "LONG_TOAST_MS = 6000", "var HELD_TOAST_MESSAGE = "+held+";")
	success := p422Window(t, jsFunctionBody(t, form, "handleSubmit()"), "Pinalert.submitReport(payload).then(", ".catch(")
	p422Need(t, "the success path of handleSubmit", success,
		"report && report.held === true",
		"Pinalert.showToast(HELD_TOAST_MESSAGE, null, LONG_TOAST_MS)",
		"Pinalert.showToast('Report posted.')",
	)
	p422Need(t, "saveToQueue(payload, status)", jsFunctionBody(t, form, "saveToQueue(payload, status)"),
		"Pinalert.showToast(persisted ? SAVE_TOAST : SAVE_TOAST_MEMORY, null, LONG_TOAST_MS)")
	p422Need(t, "sendRow(record)", jsFunctionBody(t, queue, "sendRow(record)"),
		"held === true",
		"Pinalert.showToast(HELD_TOAST_MESSAGE, null, LONG_TOAST_MS)",
		"Pinalert.showToast(SENT_TOAST)",
	)
	p422Need(t, "queue.js", queue, "LONG_TOAST_MS = 6000")
	p422Lack(t, "report-form.js", form, "getElementById('toast')")
	p422Lack(t, "queue.js", queue, "getElementById('toast')")
}

func TestQueueFormCopyAndPrefill(t *testing.T) {
	raw := p422Static(t, "static/js/report-form.js")
	form := p422Code(raw)
	for _, c := range []struct{ name, literal string }{
		{"SAVE_TOAST", `"No signal. Your report is saved on this device and will send when you're back online."`},
		{"SAVE_TOAST_MEMORY", `'No signal. Your report is kept on this page only. Keep the page open until it sends.'`},
		{"WAIT_SAVE_TOAST", `'Report saved on this device. It will send when the wait is over.'`},
		{"EMERGENCY_CALL_PREFIX", `'In an emergency, call '`},
		{"EMERGENCY_CALL_SUFFIX", `' now.'`},
		{"CALL_LABEL_PREFIX", `'Call '`},
	} {
		if n := p422Count(form, "var "+c.name+" = "+c.literal+";"); n != 1 {
			t.Errorf("%s is declared %d times with the exact copy, want 1", c.name, n)
		}
		p422PlainCopy(t, c.literal[1:len(c.literal)-1])
	}
	for _, dash := range []string{string(rune(0x2014)), string(rune(0x2013))} {
		if strings.Contains(raw, dash) {
			t.Errorf("report-form.js contains the dash character %q", dash)
		}
	}

	open := jsFunctionBody(t, form, "openForm(options)")
	start := strings.Index(open, "picker.start(")
	prefill := strings.Index(open, "applyPrefill(")
	if start == -1 || prefill == -1 || prefill < start {
		t.Errorf("applyPrefill must run after picker.start (%d, %d)", start, prefill)
	}
	p422Need(t, "openForm(options)", open, "options.note", "options.requestId", "announce(")
	apply := jsFunctionBody(t, form, "applyPrefill(prefill)")
	p422Need(t, "applyPrefill(prefill)", apply,
		"selectCategory(", "SEVERITY_BY_VALUE", "updateSeverityReadout()", "showShelterFields()",
		"picker.choose(", "displayName: ''")
	if !strings.HasSuffix(strings.TrimSpace(apply), "formTouched = false;\n  }") {
		t.Errorf("formTouched = false must be the last statement of applyPrefill")
	}
	p422Need(t, "resetForm()", jsFunctionBody(t, form, "resetForm()"),
		"noteEl.hidden = true", "currentRequestId = null", "resendRequestId = null", "lastLimit = null")
	submit := jsFunctionBody(t, form, "handleSubmit()")
	success := p422Window(t, submit, "Pinalert.submitReport(payload).then(", ".catch(")
	p422Need(t, "the success path of handleSubmit", success, "PinalertQueue.discard(resendRequestId)")

	at := strings.Index(submit, ".written_at =")
	if at == -1 || !strings.Contains(submit[at:at+60], "resendWrittenAt") {
		t.Errorf("handleSubmit may set written_at only from resendWrittenAt")
	}
	if n := p422Count(submit, ".written_at ="); n != 1 {
		t.Errorf("handleSubmit sets written_at %d times, want 1", n)
	}
	queued := jsFunctionBody(t, form, "saveToQueue(payload, status)")
	p422Need(t, "saveToQueue(payload, status)", queued, ".written_at =", "firstSubmitAt")
	if strings.Contains(form, "navigator."+"onLine") {
		t.Errorf("report-form.js must never read the browser's online flag")
	}
}

func TestQueueLateFixKeepsChosenPlace(t *testing.T) {
	modal := p422Code(p422Static(t, "static/js/modal.js"))
	success := p422Window(t, modal, "getCurrentPosition(", "placeMarker(lat, lon);")
	if !strings.Contains(success, "hasMarker()") {
		t.Errorf("the position success callback must return early when a place is chosen")
	}
	failure := p422Window(t, modal, "placeMarker(lat, lon);", "showLocationDenied(fallbackLat, fallbackLon);")
	if !strings.Contains(failure, "hasMarker()") {
		t.Errorf("the position error callback must return early when a place is chosen")
	}
	if n := p422Count(modal, "L.marker("); n != 1 {
		t.Errorf("modal.js constructs %d markers, want exactly 1", n)
	}
}
