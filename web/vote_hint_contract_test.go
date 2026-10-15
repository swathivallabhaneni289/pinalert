package web

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/service"
)

// Plan 03-18 contract tests: the too far hint on Confirm, Dispute and Mark
// resolved (D-22 as amended by A8 and RR-3). The server stays the gatekeeper
// (D-01); these tests pin the shape of the browser half so a later edit
// cannot turn the hint into a native disabled state, a hover tooltip, or a
// second source of the radius.

// p18ReadStatic reads one embedded static file and strips block comments.
func p18ReadStatic(t *testing.T, path string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, path)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", path, err)
	}
	return stripCSSComments(string(raw))
}

// p18ReadTemplate reads one embedded template file.
func p18ReadTemplate(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(TemplatesFS, name)
	if err != nil {
		t.Fatalf("failed to read embedded %s: %v", name, err)
	}
	return string(raw)
}

// p18TightBody returns the span from "function "+anchor through the first
// closing brace at module level indentation, so it holds one function and
// none of the next function's leading comment.
func p18TightBody(t *testing.T, text, anchor string) string {
	t.Helper()
	start := strings.Index(text, "function "+anchor)
	if start == -1 {
		t.Fatalf("could not locate %q in votes.js", "function "+anchor)
	}
	rest := text[start:]
	end := strings.Index(rest, "\n  }\n")
	if end == -1 {
		t.Fatalf("could not find the end of %q", "function "+anchor)
	}
	return rest[:end+len("\n  }\n")]
}

// p18Before fails when either needle is missing from text or when first does
// not come before second.
func p18Before(t *testing.T, text, label, first, second string) {
	t.Helper()
	a := strings.Index(text, first)
	b := strings.Index(text, second)
	if a == -1 || b == -1 {
		t.Errorf("%s: missing %q (index %d) or %q (index %d)", label, first, a, second, b)
		return
	}
	if a >= b {
		t.Errorf("%s: %q must come before %q", label, first, second)
	}
}

func p18Contains(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(body, n) {
			t.Errorf("%s must contain %q", label, n)
		}
	}
}

func p18Excludes(t *testing.T, label, body string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(body, n) {
			t.Errorf("%s must not contain %q", label, n)
		}
	}
}

func p18Once(t *testing.T, label, text, needle string) {
	t.Helper()
	if n := strings.Count(text, needle); n != 1 {
		t.Errorf("%s: %q occurs %d times, want exactly 1", label, needle, n)
	}
}

// TestVoteHint is V-36 with the A8 clearing assertions.
func TestVoteHint(t *testing.T) {
	text := p18ReadStatic(t, "static/js/votes.js")

	// Declarations, in order, after setBlockBusy and before showVoteError.
	anchors := []string{
		"function setBlockBusy(block, busy)",
		"function peekVoterLocation()",
		"function readVoteRadiusKm()",
		"function greatCircleKm(lat1, lon1, lat2, lon2)",
		"function clearDistanceHint(block)",
		"function applyDistanceHint(block, report, justTaken)",
		"function showVoteError(block, message)",
	}
	for i := 0; i+1 < len(anchors); i++ {
		p18Before(t, text, "declaration order", anchors[i], anchors[i+1])
	}

	// Module constants, once each.
	p18Once(t, "constants", text, "var VOTER_LOCATION_HINT_MS = 120000;")
	p18Once(t, "constants", text, "var HINT_BUTTON_SELECTORS = ['.vote-btn--confirm', '.vote-btn--dispute', '.vote-btn--resolve'];")
	p18Once(t, "constants", text, "var TOO_FAR_LEAD = 'Too far to vote. You need to be within ';")
	p18Once(t, "constants", text, "var TOO_FAR_TAIL = '\\u00a0km of this report.';")

	// updateVoteBlock decides the hint on every render and holds none of it.
	update := p18TightBody(t, text, "updateVoteBlock(block, report)")
	p18Once(t, "updateVoteBlock", update, "applyDistanceHint(block, report);")
	updateSpan := jsFunctionBody(t, text, "updateVoteBlock(block, report)")
	p18Excludes(t, "updateVoteBlock span", updateSpan, "disabled", "aria-describedby", "data-too-far")

	// applyDistanceHint.
	apply := p18TightBody(t, text, "applyDistanceHint(block, report, justTaken)")
	p18Contains(t, "applyDistanceHint", apply,
		"report.is_own_report", "readVoteRadiusKm()", "peekVoterLocation()",
		"isFreshVoterLocation(", "VOTER_LOCATION_HINT_MS", "greatCircleKm(",
		"report.latitude", "report.longitude", "clearDistanceHint(block)",
		"HINT_BUTTON_SELECTORS", "setAttribute('aria-disabled', 'true')",
		"setAttribute('data-too-far', 'true')",
		"setAttribute('aria-describedby', block.note.id)",
		"Pinalert.setText(block.note, ", "TOO_FAR_LEAD", "TOO_FAR_TAIL",
		"block.note.hidden = false", "var distanceKm = greatCircleKm(")
	if !regexp.MustCompile(`distanceKm\s*>\s*radiusKm\b`).MatchString(apply) {
		t.Errorf("applyDistanceHint must compare with a strict distanceKm > radiusKm")
	}
	if regexp.MustCompile(`>=\s*radiusKm`).MatchString(apply) || regexp.MustCompile(`<=\s*radiusKm`).MatchString(apply) {
		t.Errorf("applyDistanceHint must not use >= or <= against radiusKm")
	}
	p18Excludes(t, "applyDistanceHint", apply,
		".disabled", "'disabled'", "vote-btn--confirm-resolve", "vote-btn--cancel-resolve",
		"vote-btn--reopen", "innerHTML", "textContent", "title", "showVoteError(",
		"clearVoteError(", "getVoterLocation(", "getCurrentPosition", "navigator.permissions")

	// clearDistanceHint.
	clearBody := p18TightBody(t, text, "clearDistanceHint(block)")
	p18Contains(t, "clearDistanceHint", clearBody,
		"HINT_BUTTON_SELECTORS", "removeAttribute('aria-disabled')",
		"removeAttribute('data-too-far')", "removeAttribute('aria-describedby')",
		"Pinalert.setText(block.note, '')", "block.note.hidden = true")
	p18Excludes(t, "clearDistanceHint", clearBody,
		".disabled", "setAttribute(", "showVoteError(", "clearVoteError(")

	// peekVoterLocation is synchronous and never prompts.
	peek := p18TightBody(t, text, "peekVoterLocation()")
	p18Contains(t, "peekVoterLocation", peek,
		"sessionStorage.getItem(VOTER_LOCATION_CACHE_KEY)", "parseVoterLocation(", "catch", "return null")
	p18Excludes(t, "peekVoterLocation", peek,
		"getCurrentPosition", "navigator.permissions", "geolocationPermissionState",
		"Promise", ".then(", "setItem(", "getVoterLocation(")

	// greatCircleKm is the one geometry in JavaScript.
	circle := p18TightBody(t, text, "greatCircleKm(lat1, lon1, lat2, lon2)")
	p18Contains(t, "greatCircleKm", circle,
		"6371", "Math.PI / 180", "Math.min(1, Math.max(0, ", "Math.asin(Math.sqrt(")
	entries, err := fs.ReadDir(StaticFS, "static/js")
	if err != nil {
		t.Fatalf("read static/js: %v", err)
	}
	total := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		raw, err := fs.ReadFile(StaticFS, "static/js/"+e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		n := strings.Count(string(raw), "Math.asin(")
		total += n
		if n > 0 && e.Name() != "votes.js" {
			t.Errorf("static/js/%s uses Math.asin; the only great circle distance lives in votes.js", e.Name())
		}
	}
	if total != 1 {
		t.Errorf("Math.asin( occurs %d times across static/js, want exactly 1 (in votes.js)", total)
	}

	// onControlsClick: a marked button sends no vote and only refreshes the fix
	// and the mark (WR-03), before any action branch.
	click := p18TightBody(t, text, "onControlsClick(block, button, taken)")
	p18Contains(t, "onControlsClick", click, "getAttribute('aria-disabled') === 'true'")
	guard := "getAttribute('aria-disabled') === 'true'"
	for _, later := range []string{"openResolveConfirm(block)", "clearVoteError(block)", "setBlockBusy(block, true)", "castVote("} {
		p18Before(t, click, "onControlsClick guard order", guard, later)
	}

	// The marked tap hands over to refreshHintThenRetry, which takes one fresh
	// fix, re-applies the hint and re-enters only when the mark is gone.
	p18Contains(t, "onControlsClick", click, "refreshHintThenRetry(block, button);")
	refresh := p18TightBody(t, text, "refreshHintThenRetry(block, button)")
	// The mark is recomputed from the coordinates the fresh fix resolved with,
	// never from the session cache (a coarse fix is not cached, WR-07), and the
	// tap goes on with those same coordinates, so it takes no second fix.
	p18Contains(t, "refreshHintThenRetry", refresh,
		"getVoterLocation({ fresh: true }).then(function (coords)",
		"var taken = { latitude: coords.latitude, longitude: coords.longitude, at: Date.now() };",
		"applyDistanceHint(block, block.lastReport, taken)",
		"getAttribute('aria-disabled') !== 'true'", "onControlsClick(block, button, taken)")
	p18Excludes(t, "refreshHintThenRetry", refresh, "castVote(", "fetch(", "getCurrentPosition",
		"peekVoterLocation(", "sessionStorage")
	p18Contains(t, "applyDistanceHint", apply, "justTaken || peekVoterLocation()")
	p18Once(t, "onControlsClick", click, "location: taken")

	// The producer of the report the refresh judges: without it the mark would
	// only be recomputed by the next poll.
	own := p18TightBody(t, text, "applyOwnReportRule(block, report)")
	p18Once(t, "applyOwnReportRule", own, "block.lastReport = report;")

	// The only caller that hands castVote a location is that one tap, and
	// castVote accepts it only as a finite number pair.
	if n := strings.Count(text, "location: taken"); n != 1 {
		t.Errorf("votes.js passes a location to castVote %d times, want exactly 1 (the refreshed tap)", n)
	}
	cast := p18TightBody(t, text, "castVote(reportId, action, opts)")
	p18Contains(t, "castVote", cast,
		"var given = opts && opts.location;",
		"typeof given.latitude === 'number' && isFinite(given.latitude)",
		"typeof given.longitude === 'number' && isFinite(given.longitude)",
		"Promise.resolve({ latitude: given.latitude, longitude: given.longitude })")
	activity := p18ReadStatic(t, "static/js/activity.js")
	p18Excludes(t, "activity.js", activity, "location:")

	// createVoteBlock holds none of the hint attributes.
	create := p18TightBody(t, text, "createVoteBlock(reportId, options)")
	p18Excludes(t, "createVoteBlock", create, "aria-describedby", "data-too-far")

	// No new export.
	start := strings.Index(text, "window.PinalertVotes = {")
	if start == -1 {
		t.Fatalf("window.PinalertVotes export not found")
	}
	exports := text[start:]
	if end := strings.Index(exports, "};"); end != -1 {
		exports = exports[:end]
	}
	p18Excludes(t, "window.PinalertVotes", exports,
		"applyDistanceHint", "clearDistanceHint", "peekVoterLocation", "readVoteRadiusKm", "greatCircleKm")

	// The radius is never typed into the script.
	if regexp.MustCompile(`within\s*1\s*km`).MatchString(text) || strings.Contains(text, "'1 km") {
		t.Errorf("votes.js types the radius into the script")
	}
}

// TestVoteHintSentenceMatchesServer pins the two literals against the
// server's own sentence, so the two copies cannot drift (A8, D-22).
func TestVoteHintSentenceMatchesServer(t *testing.T) {
	text := p18ReadStatic(t, "static/js/votes.js")

	lead := regexp.MustCompile(`var TOO_FAR_LEAD = '([^']*)';`).FindStringSubmatch(text)
	tail := regexp.MustCompile(`var TOO_FAR_TAIL = '([^']*)';`).FindStringSubmatch(text)
	if lead == nil || tail == nil {
		t.Fatalf("TOO_FAR_LEAD or TOO_FAR_TAIL literal not found in votes.js")
	}
	const nbsp = `\u00a0`
	if !strings.HasPrefix(tail[1], nbsp+"km") {
		t.Fatalf("TOO_FAR_TAIL %q must start with the source escape for a non breaking space, then km", tail[1])
	}
	plainTail := strings.Replace(tail[1], nbsp, " ", 1)
	got := lead[1] + strconv.FormatFloat(service.MaxVoteDistanceKm, 'f', -1, 64) + plainTail
	if want := service.TooFarMessage(); got != want {
		t.Errorf("hint sentence %q does not equal service.TooFarMessage() %q", got, want)
	}
	for _, lit := range []string{lead[1], tail[1]} {
		if strings.ContainsAny(lit, "\u2014\u2013") {
			t.Errorf("literal %q holds a long dash", lit)
		}
	}
}

// TestVoteRadiusReadWithoutConfig is V-34, browser half: the radius is read
// straight from the shell attribute, with no numeric fallback and no use of
// the shared store.
func TestVoteRadiusReadWithoutConfig(t *testing.T) {
	text := p18ReadStatic(t, "static/js/votes.js")

	read := p18TightBody(t, text, "readVoteRadiusKm()")
	p18Contains(t, "readVoteRadiusKm", read,
		"document.getElementById('app-shell')", "getAttribute('data-vote-radius-km')",
		"isFinite(", "> 0", "return null")
	p18Excludes(t, "readVoteRadiusKm", read, "Pinalert.", "??")
	if regexp.MustCompile(`\|\|\s*[0-9.]`).MatchString(read) {
		t.Errorf("readVoteRadiusKm must not fall back to a typed number")
	}
	p18Excludes(t, "votes.js", text, "Pinalert.config")

	entries, err := fs.ReadDir(StaticFS, "static/js")
	if err != nil {
		t.Fatalf("read static/js: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") || e.Name() == "votes.js" {
			continue
		}
		raw, err := fs.ReadFile(StaticFS, "static/js/"+e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if strings.Contains(string(raw), "vote-radius") {
			t.Errorf("static/js/%s mentions vote-radius; only votes.js reads it", e.Name())
		}
	}

	index := p18ReadTemplate(t, "templates/index.html.tmpl")
	at := strings.Index(index, `id="app-shell"`)
	if at == -1 {
		t.Fatalf(`index.html.tmpl has no id="app-shell"`)
	}
	tagStart := strings.LastIndex(index[:at], "<")
	tagEnd := strings.Index(index[at:], ">")
	if tagStart == -1 || tagEnd == -1 {
		t.Fatalf("could not bound the app-shell opening tag")
	}
	if tag := index[tagStart : at+tagEnd+1]; !strings.Contains(tag, `data-vote-radius-km="{{.VoteRadiusKm}}"`) {
		t.Errorf("app-shell tag does not carry data-vote-radius-km=\"{{.VoteRadiusKm}}\": %s", tag)
	}

	profile := p18ReadTemplate(t, "templates/profile.html.tmpl")
	if strings.Contains(profile, "data-vote-radius-km") {
		t.Errorf("profile.html.tmpl must carry no vote radius attribute (no hint on the Activity page)")
	}
}

// TestTooFarRefusalClearsWithTheHint pins amendment A8 and D-22: the error
// line that holds the server's too far refusal clears when the too far hint
// goes from showing to cleared. No UI-SPEC row gives that line a lifetime, so
// the fallback applies and it leaves together with the hint, narrowly: only a
// refusal the server marked with the location field is cleared, so the GPS
// denied, generic failure and saved-but-not-refreshed lines survive. This is a
// static check of the shapes and their order. It cannot run a browser, so the
// live proof is the scripted check T4 S1.11b, run after the merge.
func TestTooFarRefusalClearsWithTheHint(t *testing.T) {
	text := p18ReadStatic(t, "static/js/votes.js")

	click := p18TightBody(t, text, "onControlsClick(block, button, taken)")
	catchAt := strings.Index(click, ".catch(function (err) {")
	if catchAt == -1 {
		t.Fatalf("onControlsClick has no .catch(function (err) { branch")
	}
	catchPart := click[catchAt:]
	pos := 0
	for _, needle := range []string{
		"showVoteError(block,",
		"block.refusedTooFar = !!(err && err.field === 'location');",
		"focusAfterVote(origin, action, null)",
	} {
		idx := strings.Index(catchPart[pos:], needle)
		if idx == -1 {
			t.Errorf("onControlsClick catch branch: %q is missing or out of order", needle)
			break
		}
		pos += idx + len(needle)
	}

	clearErr := p18TightBody(t, text, "clearVoteError(block)")
	p18Contains(t, "clearVoteError", clearErr, "block.refusedTooFar = false;", "Pinalert.setText(block.error")

	p18Before(t, text, "clearRefusalWithHint placement",
		"function clearVoteError(block)", "function clearRefusalWithHint(block, hintWasShowing)")
	refusal := p18TightBody(t, text, "clearRefusalWithHint(block, hintWasShowing)")
	p18Contains(t, "clearRefusalWithHint", refusal,
		"hintWasShowing", "block.note", ".hidden", "block.refusedTooFar", "clearVoteError(block)")
	p18Excludes(t, "clearRefusalWithHint", refusal, "showVoteError(", "setAttribute(")

	const wasShowing = "var hintWasShowing = !!block.note && !block.note.hidden;"
	update := p18TightBody(t, text, "updateVoteBlock(block, report)")
	p18Before(t, update, "updateVoteBlock hint snapshot", wasShowing, "applyDistanceHint(block, report);")
	p18Before(t, update, "updateVoteBlock refusal clear",
		"applyDistanceHint(block, report);", "clearRefusalWithHint(block, hintWasShowing);")

	retry := p18TightBody(t, text, "refreshHintThenRetry(block, button)")
	p18Before(t, retry, "refreshHintThenRetry hint snapshot",
		wasShowing, "applyDistanceHint(block, block.lastReport, taken);")
	p18Before(t, retry, "refreshHintThenRetry refusal clear",
		"applyDistanceHint(block, block.lastReport, taken);", "clearRefusalWithHint(block, hintWasShowing);")

	locate := p18TightBody(t, text, "getVoterLocation(opts)")
	p18Contains(t, "getVoterLocation", locate, "deniedErr.fieldMessage = GPS_DENIED_MESSAGE")
	p18Excludes(t, "getVoterLocation", locate, "deniedErr.field =")
}
