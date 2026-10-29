// rate_prompt_e2e_test.go proves the rate prompt on the resolve response over the
// real router and a real Postgres (Phase 9 plan 17, D-02, T-09-62, T-09-63): the
// resolve answer says by itself when to offer the skippable star box, names only
// a reporter whose profile the caller may open, and says nothing after the caller
// has rated. Every package level identifier added here carries the p917 prefix.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// p917Prompt is the rate_prompt member of a vote response, or nil when absent.
type p917Prompt struct {
	Username    string `json:"username"`
	ProfilePath string `json:"profile_path"`
}

// p917Vote casts one vote and returns the rate_prompt member and the raw body.
func p917Vote(t *testing.T, client *http.Client, baseURL string, report int64, action string) (*p917Prompt, string) {
	t.Helper()
	status, raw, err := p07PostVoteRaw(client, baseURL, report, action, p07ReportLat, p07ReportLon)
	if err != nil || status != http.StatusOK {
		t.Fatalf("%s: status %d, err %v, body %s", action, status, err, raw)
	}
	var out struct {
		RatePrompt *p917Prompt `json:"rate_prompt"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decoding %s: %v", action, raw, err)
	}
	return out.RatePrompt, string(raw)
}

// TestResolveResponseRatePrompt: POST /api/reports/{id}/resolve answers
// rate_prompt with the username and the /u/ path for an eligible reader, and no
// rate_prompt after the reader has rated, after a confirm, for the reporter's
// own resolve, or for a private reporter.
func TestResolveResponseRatePrompt(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, nil)

	reporter, reporterID := p907Reporter(t, srv, mailer, pool, "p917-reporter@example.com", "p917.reporter")
	report := testutil.SeedHistoryReport(t, pool, "sess-p917.reporter", "low", "flood", time.Now().UTC().Add(time.Hour))

	reader := newVerifiedClient(t, srv, mailer, "p917-reader@example.com")

	// A confirm answers no rate_prompt key at all.
	if prompt, raw := p917Vote(t, reader, srv.URL, report, "confirm"); prompt != nil {
		t.Errorf("confirm answered a rate_prompt: %s", raw)
	}

	// The resolve names the reporter and the profile page.
	prompt, raw := p917Vote(t, reader, srv.URL, report, "resolve")
	if prompt == nil {
		t.Fatalf("resolve answered no rate_prompt: %s", raw)
	}
	if prompt.Username != "p917.reporter" || prompt.ProfilePath != "/u/p917.reporter" {
		t.Errorf("rate_prompt = %+v, want p917.reporter and /u/p917.reporter (%s)", *prompt, raw)
	}

	// No account id reaches the wire.
	var generic map[string]any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	member, _ := generic["rate_prompt"].(map[string]any)
	if len(member) != 2 {
		t.Errorf("rate_prompt has members %v, want exactly username and profile_path", member)
	}

	// After the reader has rated, the same resolve answers no rate_prompt.
	status, body := p914rRate(t, reader, srv, "p917.reporter", `{"stars":4}`)
	if status != http.StatusNoContent {
		t.Fatalf("rating 4: status %d, body %s", status, body)
	}
	if prompt, raw := p917Vote(t, reader, srv.URL, report, "resolve"); prompt != nil {
		t.Errorf("resolve after a rating answered a rate_prompt: %s", raw)
	}

	// The reporter's own resolve answers none.
	if prompt, raw := p917Vote(t, reporter, srv.URL, report, "resolve"); prompt != nil {
		t.Errorf("the reporter's own resolve answered a rate_prompt: %s", raw)
	}

	// A private reporter is never named.
	testutil.MustExec(t, pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, reporterID)
	second := newVerifiedClient(t, srv, mailer, "p917-second@example.com")
	if prompt, raw := p917Vote(t, second, srv.URL, report, "resolve"); prompt != nil {
		t.Errorf("resolve of a private reporter answered a rate_prompt: %s", raw)
	}
}
