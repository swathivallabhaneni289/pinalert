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
