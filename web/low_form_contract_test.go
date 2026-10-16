package web

// Contract tests for plan 04-19: voting and posting from the Low data view,
// the wait memory for a rate limited vote, and the page lock behind the report
// form. They read the shipped scripts, template and stylesheets out of the
// embedded filesystems, so they inspect what the binary serves. Helpers start
// with p419 so they never collide with another file in this package.

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"time"

	"pinalert/internal/service"
)

// p419JS returns an embedded script with its comments removed.
func p419JS(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, "static/js/"+name)
	if err != nil {
		t.Fatalf("reading embedded static/js/%s: %v", name, err)
	}
	return stripJSComments(string(raw))
}

// p419CSS returns an embedded stylesheet with its comments removed.
func p419CSS(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, "static/css/"+name)
	if err != nil {
		t.Fatalf("reading embedded static/css/%s: %v", name, err)
	}
	return stripCSSComments(string(raw))
}

// p419Template returns an embedded template file as it is written.
func p419Template(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(TemplatesFS, "templates/"+name)
	if err != nil {
		t.Fatalf("reading embedded templates/%s: %v", name, err)
	}
	return string(raw)
}

func p419Has(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Errorf("%s does not contain %q", label, needle)
		}
	}
}

func p419Lacks(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			t.Errorf("%s contains %q", label, needle)
		}
	}
}

func p419Count(t *testing.T, label, body, needle string, want int) {
	t.Helper()
	if n := strings.Count(body, needle); n != want {
		t.Errorf("%s holds %q %d times, want %d", label, needle, n, want)
	}
}

// p419Before fails unless first appears in body and ends before second starts.
func p419Before(t *testing.T, label, body, first, second string) {
	t.Helper()
	a := strings.Index(body, first)
	b := strings.Index(body, second)
	if a == -1 || b == -1 {
		t.Errorf("%s: %q (at %d) and %q (at %d) must both be present", label, first, a, second, b)
		return
	}
	if a >= b {
		t.Errorf("%s: %q must come before %q", label, first, second)
	}
}

// TestLowDataVoteBlockOmitsResolve proves a Low data row mounts the shared
// vote block without Mark resolved, while the one argument call on the main
// page keeps it (D-15, P4-A15, R-09).
func TestLowDataVoteBlockOmitsResolve(t *testing.T) {
	votes := p419JS(t, "votes.js")
	low := p419JS(t, "low.js")

	p419Count(t, "votes.js", votes, "function createVoteBlock(reportId, options)", 1)
	p419Count(t, "votes.js", votes, "function createVoteBlock(reportId)", 0)

	create := jsFunctionBody(t, votes, "createVoteBlock(reportId, options)")
	p419Has(t, "createVoteBlock", create, "options.resolve === false",
		"vote-btn--resolve", "resolve-confirm", "vote-btn--confirm-resolve", "vote-btn--cancel-resolve")
	p419Count(t, "createVoteBlock", create, "controls.appendChild(resolveBtn)", 1)
	p419Count(t, "createVoteBlock", create, "controls.appendChild(confirmBlock)", 1)
	p419Count(t, "createVoteBlock", create, "addEventListener", 2)

	for _, anchor := range []string{"applyOwnReportRule(block, report)", "updateVoteBlock(block, report)"} {
		body := jsFunctionBody(t, votes, anchor)
		p419Lacks(t, anchor, body, "disabled", "options")
	}

	cancel := jsFunctionBody(t, votes, "cancelResolveConfirm(container)")
	p419Has(t, "cancelResolveConfirm", cancel, "return false")
	p419Before(t, "cancelResolveConfirm", cancel, "if (!open)", "open.closest(")

	p419Count(t, "low.js", low, "PinalertVotes.createVoteBlock(report.id, { resolve: false })", 1)
	update := jsFunctionBody(t, low, "updateLowRow(row, report)")
	p419Before(t, "updateLowRow", update,
		"PinalertTrust.updateTrustBlock(row.trust, report.trust)",
		"PinalertVotes.updateVoteBlock(row.voteBlock, report)")

	create2 := jsFunctionBody(t, low, "createLowRow(report)")
	p419Has(t, "createLowRow", create2, "voteBlock: voteBlock")
	p419Before(t, "createLowRow", create2, "PinalertTrust.createTrustBlock(", "PinalertVotes.createVoteBlock(")
	p419Before(t, "createLowRow", create2, "PinalertVotes.createVoteBlock(", "appendChild(voteBlock.controls)")
	p419Before(t, "createLowRow", create2, "appendChild(voteBlock.controls)", "appendChild(voteBlock.note)")
	p419Before(t, "createLowRow", create2, "appendChild(voteBlock.note)", "appendChild(voteBlock.error)")

	p419Lacks(t, "low.js", low, "vote-btn", "vote-controls", "vote-error", "your_vote",
		"is_own_report", "resolve-confirm", "Mark resolved")

	entries, err := fs.ReadDir(StaticFS, "static/js")
	if err != nil {
		t.Fatalf("listing static/js: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "low.js" {
			continue
		}
		if strings.Contains(p419JS(t, entry.Name()), "resolve: false") {
			t.Errorf("%s passes the resolve option; only low.js may", entry.Name())
		}
	}

	oneArg := regexp.MustCompile(`PinalertVotes\.createVoteBlock\([^)]*,`)
	for _, name := range []string{"feed.js", "map.js"} {
		if oneArg.MatchString(p419JS(t, name)) {
			t.Errorf("%s calls createVoteBlock with more than one argument", name)
		}
	}
}

// TestVoteRateLimitClient proves the 429 wait memory in votes.js and the wait
// wording in app.js (D-12, D-18).
func TestVoteRateLimitClient(t *testing.T) {
	votes := p419JS(t, "votes.js")
	app := p419JS(t, "app.js")

	p419Has(t, "votes.js", votes, "var voteWaits = {};", "function voteWaitKind(action)",
		"function rememberVoteWait(reportId, action, seconds, message, field)",
		"function knownVoteWait(reportId, action)")
	p419Lacks(t, "votes.js", votes, "indexedDB", "localStorage")

	p419Has(t, "voteWaitKind", jsFunctionBody(t, votes, "voteWaitKind(action)"),
		"'resolve'", "'reopen'", "'unresolve'", "'resolution'", "'content'")

	cast := jsFunctionBody(t, votes, "castVote(reportId, action, opts)")
	p419Count(t, "castVote", cast, "knownVoteWait(reportId, action)", 1)
	p419Before(t, "castVote", cast, "knownVoteWait(reportId, action)", "getVoterLocation(")
	p419Has(t, "castVote", cast, "Promise.reject(", "res.status === 429", "retry_after_seconds",
		"Retry-After", "rememberVoteWait(", "err.retryAfterSeconds")
	p419Lacks(t, "castVote", cast, "while (", "for (")

	// A tap on a button marked too far must not read a position during a wait.
	hint := jsFunctionBody(t, votes, "refreshHintThenRetry(block, button)")
	p419Before(t, "refreshHintThenRetry", hint, "knownVoteWait(", "getVoterLocation(")

	remember := jsFunctionBody(t, votes, "rememberVoteWait(reportId, action, seconds, message, field)")
	known := jsFunctionBody(t, votes, "knownVoteWait(reportId, action)")
	p419Has(t, "knownVoteWait", known, "Date.now()", "Pinalert.formatWait(", ".indexOf(", "=== -1",
		"fieldMessage", "429")
	for label, body := range map[string]string{"rememberVoteWait": remember, "knownVoteWait": known} {
		p419Lacks(t, label, body, "disabled", "setBlockBusy(", "Storage", "indexedDB")
	}

	p419Count(t, "app.js", app, "function formatWait(seconds)", 1)
	p419Count(t, "app.js", app, "formatWait: formatWait", 1)
	p419Has(t, "formatWait", jsFunctionBody(t, app, "formatWait(seconds)"),
		"Math.ceil(", "< 60", "< 3600", "' second'", "' seconds'", "' minute'", "' minutes'",
		"' hour'", "' hours'")

	// The client finds the wait phrase in the stored sentence only because
	// the server builds the sentence around WaitPhrase.
	phrases := []struct {
		wait time.Duration
		want string
	}{
		{1 * time.Second, "1 second"},
		{30 * time.Second, "30 seconds"},
		{59 * time.Second, "59 seconds"},
		{60 * time.Second, "1 minute"},
		{61 * time.Second, "2 minutes"},
		{15 * time.Minute, "15 minutes"},
		{time.Hour, "1 hour"},
		{3601 * time.Second, "2 hours"},
	}
	for _, c := range phrases {
		if got := service.WaitPhrase(c.wait); got != c.want {
			t.Errorf("WaitPhrase(%v) = %q, want %q", c.wait, got, c.want)
		}
	}
	if m := service.VoteLimitMessage(15 * time.Second); !strings.Contains(m, service.WaitPhrase(15*time.Second)) {
		t.Errorf("VoteLimitMessage does not carry the wait phrase: %q", m)
	}
	if m := service.VoteChangeLimitMessage(30 * time.Second); !strings.Contains(m, service.WaitPhrase(30*time.Second)) {
		t.Errorf("VoteChangeLimitMessage does not carry the wait phrase: %q", m)
	}
}

// TestLowDataRowWrittenLine proves the Low data row shows the optional
// Written line through the shared formatter and nothing else (P4-A8, R-09).
func TestLowDataRowWrittenLine(t *testing.T) {
	low := p419JS(t, "low.js")

	create := jsFunctionBody(t, low, "createLowRow(report)")
	if n := strings.Count(create, "'report-row__meta'"); n < 2 {
		t.Errorf("createLowRow holds 'report-row__meta' %d times, want at least 2", n)
	}
	p419Has(t, "createLowRow", create, "written.hidden = true", "written: written", "appendChild(written)")
	p419Before(t, "createLowRow", create, "'report-row__meta'", "appendChild(written)")

	p419Has(t, "updateLowRow", jsFunctionBody(t, low, "updateLowRow(row, report)"),
		"updateLowWrittenLine(row, report)")
	p419Has(t, "refreshLowTimes", jsFunctionBody(t, low, "refreshLowTimes()"), "updateLowWrittenLine(")
	p419Has(t, "updateLowWrittenLine", jsFunctionBody(t, low, "updateLowWrittenLine(row, report)"),
		"Pinalert.writtenLine(report)", "Pinalert.setText(row.written", "row.written.hidden =")
	p419Lacks(t, "low.js", low, "written_at")
}

// TestPageLockPairs proves the shared report form locks the page behind it on
// open and releases it on close, before focus returns (P4-A26).
func TestPageLockPairs(t *testing.T) {
	app := p419JS(t, "app.js")
	form := p419JS(t, "report-form.js")
	css := p419CSS(t, "modal.css")

	p419Has(t, "app.js", app, "var LOCK_EXEMPT_IDS = ['toast', 'notice-live'];", "var lockedNodes = [];",
		"function lockOutsideNode(node)", "function lockPage(backdrop)", "function unlockPage()",
		"lockPage: lockPage", "unlockPage: unlockPage")

	lock := jsFunctionBody(t, app, "lockPage(backdrop)")
	p419Has(t, "lockPage", lock, "parentElement", "document.body", "lockOutsideNode(",
		"setAttribute('data-modal-open', 'true')")
	p419Lacks(t, "lockPage", lock, "document.body.children", "getElementById('toast')")

	outside := jsFunctionBody(t, app, "lockOutsideNode(node)")
	p419Has(t, "lockOutsideNode", outside, "LOCK_EXEMPT_IDS", ".contains(", "inert", "aria-hidden",
		"lockedNodes.push(")

	unlock := jsFunctionBody(t, app, "unlockPage()")
	p419Has(t, "unlockPage", unlock, "lockedNodes", "inert", "removeAttribute('aria-hidden')",
		"removeAttribute('data-modal-open')")

	openName := regexp.MustCompile(`open:\s*(\w+)`).FindStringSubmatch(form)
	closeName := regexp.MustCompile(`close:\s*(\w+)`).FindStringSubmatch(form)
	if openName == nil || closeName == nil {
		t.Fatalf("report-form.js export does not name its open and close functions")
	}
	open := jsFunctionBody(t, form, openName[1]+"(")
	closeBody := jsFunctionBody(t, form, closeName[1]+"(")
	p419Count(t, "the open function", open, "Pinalert.lockPage(", 1)
	p419Count(t, "the close function", closeBody, "Pinalert.unlockPage()", 1)
	p419Before(t, "the close function", closeBody, "Pinalert.unlockPage()", ".focus(")
	p419Count(t, "report-form.js", form, "Pinalert.lockPage(", 1)
	p419Count(t, "report-form.js", form, "Pinalert.unlockPage()", 1)

	lockRule := regexp.MustCompile(`html\[data-modal-open\]\s*\{([^}]*)\}`).FindStringSubmatch(css)
	if lockRule == nil {
		t.Fatalf("modal.css has no rule whose head is html[data-modal-open]")
	}
	p419Has(t, "the page lock rule", lockRule[1], "overflow: hidden")
	p419Lacks(t, "the page lock rule", lockRule[1], "!important")
	found := false
	for _, m := range regexp.MustCompile(`(?m)^\.modal-panel\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1) {
		if strings.Contains(m[1], "overscroll-behavior: contain") {
			found = true
			p419Lacks(t, "the .modal-panel rule", m[1], "!important")
		}
	}
	if !found {
		t.Errorf("no .modal-panel rule declares overscroll-behavior: contain")
	}
}

// TestLowDataLocationStates proves the map free location block: its constants,
// its interface members, the accuracy wording and the dialog shell that hosts
// it (D-15, D-16).
func TestLowDataLocationStates(t *testing.T) {
	low := p419JS(t, "low.js")
	page := p419Template(t, "low.html.tmpl")

	p419Has(t, "low.js", low, "var GOOD_ACCURACY_METRES = 100;", "var LOCATION_TIMEOUT_MS = 8000;",
		"function createLowLocation()", "function accuracyPhrase(metres)")

	location := jsFunctionBody(t, low, "createLowLocation()")
	p419Has(t, "createLowLocation", location, "current:", "choose:", "isSet:", "missingMessage:",
		"searchFailureMessage: lowSearchFailureMessage", "noteSearchOutcome:", "start:", "reset:",
		"getCurrentPosition(", "enableHighAccuracy: true", "timeout: LOCATION_TIMEOUT_MS", "positionSeq",
		"PERMISSION_DENIED", "markTouched", "setAttribute('aria-disabled', 'true')",
		"removeAttribute('aria-disabled')", "'low-location-status'", "low-location-use",
		"low-location-own", "low-location-retry", "low-location-full")

	phrase := jsFunctionBody(t, low, "accuracyPhrase(metres)")
	p419Has(t, "accuracyPhrase", phrase, "Math.round(", "Math.max(10", "1000",
		"Math.round(rounded / 100) / 10", "' km'", "' metres'")

	p419Count(t, "low.js", low, "PinalertReportForm.mount(", 1)
	mount := low[strings.Index(low, "PinalertReportForm.mount("):]
	if end := strings.Index(mount, ");"); end != -1 {
		mount = mount[:end]
	}
	p419Has(t, "the mount call", mount, "variant: 'low'", "location: lowLocation")
	boot := jsFunctionBody(t, low, "boot()")
	p419Has(t, "boot", boot, "PinalertReportForm.open(", "opener:")

	modal := strings.Index(page, `id="report-modal"`)
	if strings.Count(page, `id="report-modal"`) != 1 || modal == -1 {
		t.Fatalf(`low.html.tmpl must hold id="report-modal" exactly once`)
	}
	backdropTag := page[strings.LastIndex(page[:modal], "<"):]
	backdropTag = backdropTag[:strings.Index(backdropTag, ">")]
	p419Has(t, "the backdrop tag", backdropTag, "<div", `class="modal-backdrop"`, "hidden")
	p419Before(t, "low.html.tmpl", page, "</main>", `id="report-modal"`)

	panelAt := strings.Index(page[modal:], `class="modal-panel`)
	if panelAt == -1 {
		t.Fatalf("low.html.tmpl has no modal panel")
	}
	panelTag := page[modal+panelAt:]
	panelTag = panelTag[:strings.Index(panelTag, ">")]
	p419Has(t, "the panel tag", panelTag, "report-form--low", `role="dialog"`, `aria-modal="true"`,
		`aria-labelledby="modal-title"`)
	p419Has(t, "low.html.tmpl", page, `<h2 id="modal-title">Report an emergency</h2>`)

	fieldsetAt := strings.Index(page, `<fieldset class="low-location" aria-describedby="low-location-status">`)
	fieldsetEnd := strings.Index(page, "</fieldset>")
	if fieldsetAt == -1 || fieldsetEnd == -1 || fieldsetEnd < fieldsetAt {
		t.Fatalf("low.html.tmpl has no location fieldset")
	}
	fieldset := page[fieldsetAt:fieldsetEnd]
	p419Ordered := []string{
		"<legend>Location</legend>", `id="low-location-status"`, `id="low-location-use"`,
		`id="low-location-own"`, `id="low-location-retry"`, `id="low-location-full"`,
		`Or type a place or address`, `id="location-search"`, `id="location-search-results"`,
		`id="location-search-status"`,
	}
	pos := 0
	for _, needle := range p419Ordered {
		idx := strings.Index(fieldset[pos:], needle)
		if idx == -1 {
			t.Errorf("the location fieldset is missing %q or holds it out of order", needle)
			break
		}
		pos += idx + len(needle)
	}
	statusTag := fieldset[strings.Index(fieldset, `id="low-location-status"`):]
	statusTag = statusTag[:strings.Index(statusTag, ">")]
	p419Has(t, "the status tag", statusTag, `role="status"`)
	for _, id := range []string{"low-location-use", "low-location-own", "low-location-retry"} {
		tag := fieldset[strings.Index(fieldset, `id="`+id+`"`):]
		tag = tag[:strings.Index(tag, ">")]
		p419Has(t, id, tag, "hidden")
		open := fieldset[:strings.Index(fieldset, `id="`+id+`"`)]
		if !strings.HasSuffix(strings.TrimSpace(open), "<button type=\"button\" class=\"btn\"") {
			t.Errorf("%s must be a button", id)
		}
	}
	linkTag := fieldset[strings.Index(fieldset, `id="low-location-full"`):]
	linkTag = linkTag[:strings.Index(linkTag, ">")]
	p419Has(t, "the full version link tag", linkTag, `href="/?full=1"`, "hidden")
	label := regexp.MustCompile(`<label for="location-search">Or type a place or address</label>`)
	if !label.MatchString(fieldset) {
		t.Errorf("the search label for location-search is missing")
	}
	for _, id := range []string{"location-search", "location-search-results", "location-search-status"} {
		p419Count(t, "low.html.tmpl", page, `id="`+id+`"`, 1)
	}

	const partial = `{{template "report_form.html.tmpl" .}}`
	p419Count(t, "low.html.tmpl", page, partial, 1)
	if strings.Index(page, partial) < fieldsetEnd {
		t.Errorf("the shared form partial must follow the location fieldset")
	}
	p419Lacks(t, "low.html.tmpl", page, "modal-map", "coord-readout", "location-notice")
}

// TestLowDataSearchIgnoresServerText proves a failed typed place search on the
// Low data view shows only the Low sentence (U8, D-16).
func TestLowDataSearchIgnoresServerText(t *testing.T) {
	form := p419JS(t, "report-form.js")
	low := p419JS(t, "low.js")
	page := p419Template(t, "low.html.tmpl")

	p419Has(t, "report-form.js", form, "var SEARCH_TIMEOUT_MS = 8000;",
		"typeof picker.noteSearchOutcome === 'function'")
	p419Count(t, "report-form.js", form, "fetch(", 1)

	p419Has(t, "searchTimeLimit", jsFunctionBody(t, form, "searchTimeLimit()"),
		"Pinalert.requestTimeout(SEARCH_TIMEOUT_MS, ")
	p419Has(t, "runSearch", jsFunctionBody(t, form, "runSearch(query)"), "signal: limit.signal",
		"limit.expired",
		"formVariant === 'low'", "picker.searchFailureMessage(", "'unavailable'")

	dropdown := jsFunctionBody(t, form, "renderDropdown(results, matched)")
	p419Has(t, "renderDropdown", dropdown, "'no_match'", "'match'")
	p419Before(t, "renderDropdown", dropdown, "'no_match'", "return")

	p419Has(t, "runSearch", jsFunctionBody(t, form, "runSearch(query)"),
		"picker.searchFailureMessage(serverText, err && err.status)")

	// The server's text is never read on this page; the status picks one of
	// the page's own sentences, the main form's words for a 429 (F-P4-12).
	failure := jsFunctionBody(t, low, "lowSearchFailureMessage(_serverText, status)")
	p419Has(t, "lowSearchFailureMessage", failure, "if (status === 429) {",
		"return LOW_SEARCH_RATE_LIMITED_MESSAGE", "return LOW_SEARCH_UNAVAILABLE_MESSAGE")
	if n := strings.Count(failure, "_serverText"); n != 1 {
		t.Errorf("lowSearchFailureMessage must never read the server's text, found _serverText %d times", n)
	}
	p419Has(t, "low.js", low,
		"var LOW_SEARCH_RATE_LIMITED_MESSAGE = 'Too many searches. Wait a few seconds and try again.';")

	for name, text := range map[string]string{"low.js": low, "low.html.tmpl": page} {
		p419Lacks(t, name, text, "Search unavailable, try tapping the map instead.",
			"Search for a place or address", "tap the map", "tapping the map", "Drag the map")
	}
}

// TestLowDataFormCopy proves the exact sentences, the page furniture the form
// needs and the tile label colour (D-13, D-15, D-16, U10, P4-A23, P4-A27).
func TestLowDataFormCopy(t *testing.T) {
	low := p419JS(t, "low.js")
	page := p419Template(t, "low.html.tmpl")
	css := p419CSS(t, "low.css")

	p419Has(t, "low.js", low,
		"Finding your position. Your browser may ask to share it. Allow it so this report is placed correctly.",
		"Position found. Accurate to about {ACCURACY}.",
		"Position found, but only accurate to about {ACCURACY}. You can use it, or type a place for a better match.",
		"Using this position, accurate to about {ACCURACY}.",
		"Location is off for this site. Type a place or address below, or turn on location access in your browser settings and then use Retry position.",
		"Couldn't get your position in time. Retry position, or type a place or address below.",
		"Using {PLACE}. The report will be placed there, not at your position.",
		"Posting from here needs a location. Open the full version to place your report on the map.",
		"Set a location by allowing location access or typing a place.",
		"Search unavailable. Try your position again, or open the full version.",
		"Choose to use this position, or type a place.",
		"Still finding your position. Wait a moment, or type a place.")
	p419Has(t, "low.js", low, ".split('{ACCURACY}').join(", ".split('{PLACE}').join(")
	p419Lacks(t, "low.js", low, ".replace(")

	p419Has(t, "low.html.tmpl", page, "Use this position", "Use my position instead", "Retry position",
		"Open full version")
	p419Count(t, "low.html.tmpl", page, `id="low-report"`, 1)
	button := regexp.MustCompile(`<button[^>]*id="low-report"[^>]*>Report an emergency</button>`)
	if !button.MatchString(page) {
		t.Errorf("#low-report must be a button reading Report an emergency")
	}
	p419Before(t, "low.html.tmpl", page, `id="low-report"`, `id="low-list"`)
	p419Count(t, "low.html.tmpl", page, `id="toast"`, 1)
	p419Before(t, "low.html.tmpl", page, "</main>", `id="toast"`)
	p419Count(t, "low.html.tmpl", page, "report-form.js", 1)
	scriptTag := regexp.MustCompile(`<script[^>]*src="/static/js/report-form\.js[^"]*"[^>]*defer[^>]*></script>`)
	if !scriptTag.MatchString(page) {
		t.Errorf("report-form.js must be one deferred script tag")
	}
	p419Before(t, "low.html.tmpl", page, "report-form.js", "/static/js/low.js")
	p419Count(t, "low.html.tmpl", page, "modal.css", 1)
	p419Before(t, "low.html.tmpl", page, "/static/css/low.css", "/static/css/modal.css")
	p419Lacks(t, "low.html.tmpl", page, ">Cancel<", "modal-map.css", "modal.js", "map.js")

	rules := parseCSSRules(css)
	tile, ok := ruleBySelector(rules, ".report-form--low .category-tile:not(.category-tile--selected)")
	if !ok {
		t.Errorf("low.css has no rule for the unselected tiles of the Low form")
	} else if got := declsOf(tile.declBody)["color"]; got != "var(--color-text)" {
		t.Errorf("the Low form tile colour is %q, want var(--color-text)", got)
	}
	actions, ok := ruleBySelector(rules, ".low-location__actions:not([hidden])")
	if !ok {
		t.Errorf("low.css has no rule for the visible location actions")
	} else if _, has := declsOf(actions.declBody)["display"]; !has {
		t.Errorf("the visible location actions rule declares no display")
	}
	for _, r := range rules {
		if strings.Contains(r.selectorHead, ".low-location") && strings.Contains(r.declBody, "display") &&
			!strings.Contains(r.selectorHead, ":not([hidden])") {
			t.Errorf("rule %q declares display without the hidden guard", r.selectorHead)
		}
	}

	files := map[string]string{
		"low.js": low, "report-form.js": p419JS(t, "report-form.js"), "low.css": css, "low.html.tmpl": page,
	}
	for name, text := range files {
		for _, r := range []rune{0x2014, 0x2013} {
			if strings.ContainsRune(text, r) {
				t.Errorf("%s holds the character U+%04X", name, r)
			}
		}
	}
}
