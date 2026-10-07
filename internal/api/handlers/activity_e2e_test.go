// activity_e2e_test.go — 02-07: the Activity page's real trust state
// (Task 2) and its Reopen control (Task 3), driven against the real
// api.NewRouter over a real Postgres. Reuses reports_e2e_test.go's
// newE2EServer (already wires deps.Votes) and auth_e2e_test.go's/
// votes_e2e_test.go's session/report/vote helpers (newVerifiedClient,
// submitReport, submitReportGetID, postVote, decodeVoteResponse,
// floodReportBody) rather than standing up a second harness.
package handlers_test

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getProfile GETs /profile using client and returns the raw body, failing
// the test on anything but 200.
func getProfile(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	resp, err := client.Get(baseURL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /profile status = %d, want 200: %s", resp.StatusCode, b)
	}
	return string(b)
}

// expireReport sets reportID's expires_at to an hour in the past, directly
// against the pool. Applied only AFTER a report has already been resolved
// through the real vote route — CastVote's own expiry rejection (02-03a
// step 4) would refuse the resolve outright if the report were seeded
// expired from the start, so the order (resolve, then expire) is
// load-bearing here, not incidental.
func expireReport(t *testing.T, pool *pgxpool.Pool, reportID int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE reports SET expires_at = $1 WHERE id = $2`,
		time.Now().Add(-1*time.Hour), reportID,
	); err != nil {
		t.Fatalf("expiring report %d: %v", reportID, err)
	}
}

// --- Task 2: the Activity page's real trust state ---

// TestProfileShowsResolvedStateForAResolvedReport proves the Activity
// page's visibility comes from the same service.Resolve every other read
// path calls: a report the account resolved renders the retracted state
// class and the "Resolved" visibility-tag label.
func TestProfileShowsResolvedStateForAResolvedReport(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-resolved@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	resp := postVote(t, client, srv.URL, reportID, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	visibility, _ := decodeVoteResponse(t, resp)
	if visibility != "retracted" {
		t.Fatalf("resolve response visibility = %q, want %q", visibility, "retracted")
	}

	// Honest limit of this test's claim: GET /profile returns the
	// server-rendered HTML only — the "Resolved" chip text itself is built
	// by activity.js/visibility.js client-side from these attributes, which
	// a Go HTTP test cannot execute. This asserts the server-rendered
	// hooks that make that chip build correctly; the rendered pixels are
	// the end-of-phase <human-check>'s claim.
	body := getProfile(t, client, srv.URL)
	if !strings.Contains(body, "vis-retracted") {
		t.Errorf("/profile does not carry the vis-retracted state class for the resolved report: %s", body)
	}
	if !strings.Contains(body, `data-visibility="retracted"`) {
		t.Errorf("/profile does not carry data-visibility=\"retracted\" for the resolved report: %s", body)
	}
	if !strings.Contains(body, `data-visibility-reason="resolved"`) {
		t.Errorf("/profile does not carry data-visibility-reason=\"resolved\" for the resolved report: %s", body)
	}
}

// TestProfileShowsLiveReportsWithoutARetractedMarker proves the marker is
// a real signal, not a constant rendered on every row: an unresolved
// report's row carries a non-retracted state.
func TestProfileShowsLiveReportsWithoutARetractedMarker(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-live@example.com")

	submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))

	body := getProfile(t, client, srv.URL)
	if strings.Contains(body, "vis-retracted") {
		t.Errorf("/profile carries vis-retracted for a report nobody resolved: %s", body)
	}
	if !strings.Contains(body, "vis-provisional") {
		t.Errorf("/profile does not carry vis-provisional for a freshly-submitted, unconfirmed report: %s", body)
	}
}

// TestProfileStillListsEveryReportTheAccountFiled proves the existing
// multi-device listing behaviour (01.1-07) is unchanged: a resolved
// report is ADDED to what the page shows, never substituted for the
// still-live one.
func TestProfileStillListsEveryReportTheAccountFiled(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-multi@example.com")

	submitReport(t, client, srv, "flood", "still-live report")
	resolvedID := submitReportGetID(t, client, srv.URL, floodReportBody(13.05, 77.65))
	postVote(t, client, srv.URL, resolvedID, "resolve", map[string]any{"latitude": 13.05, "longitude": 77.65})

	body := getProfile(t, client, srv.URL)
	if !strings.Contains(body, "still-live report") {
		t.Errorf("/profile is missing the unresolved report: %s", body)
	}
	if !strings.Contains(body, "Water rising near the bridge") {
		t.Errorf("/profile is missing the resolved report's own description: %s", body)
	}
}

// TestProfileNoLongerPromisesVotingHistory proves the removed placeholder
// sentence ("...once voting launches.") is absent from the rendered page.
func TestProfileNoLongerPromisesVotingHistory(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-placeholder@example.com")

	body := getProfile(t, client, srv.URL)
	if strings.Contains(body, "once voting launches") {
		t.Errorf("/profile still contains the now-false voting-history placeholder sentence: %s", body)
	}
}

// --- Task 3: Reopen ---

// TestProfileOffersReopenOnARetractedReport proves a retracted, unexpired
// report's row carries the reopen hook.
func TestProfileOffersReopenOnARetractedReport(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-reopen-hook@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	postVote(t, client, srv.URL, reportID, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})

	body := getProfile(t, client, srv.URL)
	if !strings.Contains(body, "data-can-reopen") {
		t.Errorf("/profile does not carry the reopen hook for a retracted, unexpired report: %s", body)
	}
}

// TestProfileWithholdsReopenOnAnExpiredRetractedReport is the
// discriminating test: a retracted report past its own expires_at still
// carries the retracted state and its chip, but withholds the reopen
// hook — a button rendered past expiry could only ever 409 (02-03a's
// expiry rejection is not scoped by vote kind), and even a successful
// reopen would put nothing back into the feed (NearbyReports filters on
// expiry regardless of visibility).
func TestProfileWithholdsReopenOnAnExpiredRetractedReport(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-reopen-expired@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	postVote(t, client, srv.URL, reportID, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	expireReport(t, pool, reportID)

	body := getProfile(t, client, srv.URL)
	if !strings.Contains(body, "vis-retracted") {
		t.Errorf("/profile no longer carries the retracted state for the expired report: %s", body)
	}
	if strings.Contains(body, "data-can-reopen") {
		t.Errorf("/profile still carries the reopen hook for a retracted, EXPIRED report — a "+
			"reopen there would 409 and, if it somehow succeeded, restore nothing to the feed: %s", body)
	}
}

// TestProfileWithholdsReopenOnANonRetractedReport proves a live report
// carries no reopen hook.
func TestProfileWithholdsReopenOnANonRetractedReport(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-reopen-live@example.com")

	submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))

	body := getProfile(t, client, srv.URL)
	if strings.Contains(body, "data-can-reopen") {
		t.Errorf("/profile carries the reopen hook for a report nobody resolved: %s", body)
	}
}

// TestProfileReopenIsInstantForTheReporter (D-16 amended 2026-09-15) is
// the test the Activity page's fixed reopen-succeeded copy rests on: a
// SINGLE reopen POST from the reporter, with no second account and no
// threshold met, lifts the retraction. See 02-CONTEXT.md D-16's amendment
// history — a reader who finds an older document describing a
// no-carve-out reading must not "restore" a threshold here; that reading
// was reversed by the user on 2026-09-15 and is locked.
func TestProfileReopenIsInstantForTheReporter(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-reopen-instant@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	postVote(t, client, srv.URL, reportID, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})

	bodyBefore := getProfile(t, client, srv.URL)
	if !strings.Contains(bodyBefore, "data-can-reopen") {
		t.Fatalf("expected the reopen hook before reopening: %s", bodyBefore)
	}

	resp := postVote(t, client, srv.URL, reportID, "reopen", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	visibility, _ := decodeVoteResponse(t, resp)
	if visibility == "retracted" {
		t.Fatalf("after the reporter's single reopen POST: visibility = %q, want it to no longer "+
			"be retracted — no second account voted and no threshold was met (D-16 amended 2026-09-15)",
			visibility)
	}

	bodyAfter := getProfile(t, client, srv.URL)
	if strings.Contains(bodyAfter, "data-can-reopen") {
		t.Errorf("/profile still carries the reopen hook after the report was reopened: %s", bodyAfter)
	}
}

// --- Plan 03-15: the trust object, the expired flag and the severity word ---

// p15IdentifierKey matches any JSON key that could name an identifier.
// The Activity trust attribute must carry none (T-03-04).
var p15IdentifierKey = regexp.MustCompile("(?i)cell|account|session|email|voter")

// p15ProfileRow returns the markup of the li that carries reportID, from its
// opening tag to the closing tag, failing the test when the row is absent.
func p15ProfileRow(t *testing.T, body string, reportID int64) string {
	t.Helper()
	marker := `data-report-id="` + strconv.FormatInt(reportID, 10) + `"`
	idx := strings.Index(body, marker)
	if idx == -1 {
		t.Fatalf("/profile has no row for report %d: %s", reportID, body)
	}
	start := strings.LastIndex(body[:idx], "<li ")
	if start == -1 {
		t.Fatalf("no <li before %s", marker)
	}
	end := strings.Index(body[idx:], "</li>")
	if end == -1 {
		t.Fatalf("no </li> after %s", marker)
	}
	return body[start : idx+end+len("</li>")]
}

// p15Attr returns the unescaped value of the first attribute called name in
// row, failing the test when it is absent.
func p15Attr(t *testing.T, row, name string) string {
	t.Helper()
	re := regexp.MustCompile(`\s` + regexp.QuoteMeta(name) + `="([^"]*)"`)
	m := re.FindStringSubmatch(row)
	if m == nil {
		t.Fatalf("row has no %s attribute: %s", name, row)
	}
	return html.UnescapeString(m[1])
}

// p15HasAttr reports whether row carries an attribute called name.
func p15HasAttr(row, name string) bool {
	return regexp.MustCompile(`\s` + regexp.QuoteMeta(name) + `(=|\s|>)`).MatchString(row)
}

// p15ClassTokens returns the class tokens of the row's li element.
func p15ClassTokens(t *testing.T, row string) []string {
	t.Helper()
	return strings.Fields(p15Attr(t, row, "class"))
}

// p15ProfileTrust decodes the row's data-trust attribute.
func p15ProfileTrust(t *testing.T, row string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(p15Attr(t, row, "data-trust")), &out); err != nil {
		t.Fatalf("data-trust is not a JSON object: %v", err)
	}
	return out
}

// p15VoteTrust decodes the trust object of a successful vote response.
func p15VoteTrust(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("vote status = %d, want 200: %s", resp.StatusCode, b)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decoding vote response: %v", err)
	}
	trust, ok := out["trust"].(map[string]any)
	if !ok {
		t.Fatalf("vote response has no trust object: %s", b)
	}
	return trust
}

// p15WalkKeys calls visit with every object key at any depth in v.
func p15WalkKeys(v any, visit func(key string)) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			visit(k)
			p15WalkKeys(child, visit)
		}
	case []any:
		for _, child := range x {
			p15WalkKeys(child, visit)
		}
	}
}

// TestProfileRowsCarryTrustJSON proves D-20 and V-33: the Activity row's
// data-trust is the same object the vote response carries, before and after
// a Reopen, and holds no identifier (T-03-04).
func TestProfileRowsCarryTrustJSON(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	reporterEmail := "activity-p15-reporter@example.com"
	confirmerEmail := "activity-p15-confirmer@example.com"
	reporter := newVerifiedClient(t, srv, mailer, reporterEmail)
	confirmer := newVerifiedClient(t, srv, mailer, confirmerEmail)

	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}
	reportID := submitReportGetID(t, reporter, srv.URL, floodReportBody(12.9716, 77.5946))
	voteTrust := p15VoteTrust(t, postVote(t, confirmer, srv.URL, reportID, "confirm", loc))

	row := p15ProfileRow(t, getProfile(t, reporter, srv.URL), reportID)
	rowTrust := p15ProfileTrust(t, row)
	if !reflect.DeepEqual(rowTrust, voteTrust) {
		t.Fatalf("Activity data-trust differs from the vote response trust:\nrow  = %v\nvote = %v", rowTrust, voteTrust)
	}
	if rowTrust["confirmed_places"] != float64(1) || rowTrust["confirmed_text"] != "1" || rowTrust["disputed_places"] != float64(0) {
		t.Errorf("unexpected counters in data-trust: %v", rowTrust)
	}
	p15WalkKeys(rowTrust, func(key string) {
		if p15IdentifierKey.MatchString(key) {
			t.Errorf("data-trust carries an identifier-like key %q", key)
		}
	})
	raw := p15Attr(t, row, "data-trust")
	for _, forbidden := range []string{"@", reporterEmail, confirmerEmail} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("data-trust contains %q: %s", forbidden, raw)
		}
	}

	// R-08: the page refresh and the Reopen response agree.
	resolveResp := postVote(t, reporter, srv.URL, reportID, "resolve", loc)
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", resolveResp.StatusCode)
	}
	resolveResp.Body.Close()
	reopenTrust := p15VoteTrust(t, postVote(t, reporter, srv.URL, reportID, "reopen", loc))
	reloaded := p15ProfileTrust(t, p15ProfileRow(t, getProfile(t, reporter, srv.URL), reportID))
	if !reflect.DeepEqual(reloaded, reopenTrust) {
		t.Fatalf("reloaded data-trust differs from the Reopen response trust:\nrow    = %v\nreopen = %v", reloaded, reopenTrust)
	}
	if reloaded["confirmed_places"] != float64(1) {
		t.Errorf("confirmed_places after reopen = %v, want 1", reloaded["confirmed_places"])
	}
}

// TestProfileMarksExpiredReports proves A44 and R-08: the server decides
// expiry, writes the flag and the first paint row class, and sends no still
// current word for an expired or Retracted report.
func TestProfileMarksExpiredReports(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-p15-expired@example.com")
	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}

	freshID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	expiredID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	resolvedID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	resolveResp := postVote(t, client, srv.URL, resolvedID, "resolve", loc)
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200", resolveResp.StatusCode)
	}
	resolveResp.Body.Close()
	expireReport(t, pool, expiredID)
	expireReport(t, pool, resolvedID)

	body := getProfile(t, client, srv.URL)
	hasToken := func(tokens []string, want string) bool {
		for _, tok := range tokens {
			if tok == want {
				return true
			}
		}
		return false
	}
	noStillCurrent := func(name string, trust map[string]any) {
		if cur, ok := trust["currency"].(map[string]any); ok && cur["label"] != "" {
			t.Errorf("%s: currency label = %v, want empty", name, cur["label"])
		}
	}
	const expiryLine = "This report has expired. It can no longer be confirmed."

	expiredRow := p15ProfileRow(t, body, expiredID)
	tokens := p15ClassTokens(t, expiredRow)
	if got := p15Attr(t, expiredRow, "data-expired"); got != "true" {
		t.Errorf("unresolved expired row: data-expired = %q, want true", got)
	}
	if !hasToken(tokens, "vis-expired") || hasToken(tokens, "vis-retracted") {
		t.Errorf("unresolved expired row class tokens = %v, want vis-expired and no vis-retracted", tokens)
	}
	if got := p15Attr(t, expiredRow, "data-visibility"); got != "provisional" {
		t.Errorf("unresolved expired row: data-visibility = %q, want provisional", got)
	}
	if p15HasAttr(expiredRow, "data-can-reopen") {
		t.Errorf("unresolved expired row carries data-can-reopen")
	}
	expiredTrust := p15ProfileTrust(t, expiredRow)
	if expiredTrust["lead_sentence"] != expiryLine {
		t.Errorf("unresolved expired row lead_sentence = %q", expiredTrust["lead_sentence"])
	}
	noStillCurrent("unresolved expired row", expiredTrust)

	resolvedRow := p15ProfileRow(t, body, resolvedID)
	tokens = p15ClassTokens(t, resolvedRow)
	if got := p15Attr(t, resolvedRow, "data-expired"); got != "true" {
		t.Errorf("resolved expired row: data-expired = %q, want true", got)
	}
	if !hasToken(tokens, "vis-retracted") || hasToken(tokens, "vis-expired") {
		t.Errorf("resolved expired row class tokens = %v, want vis-retracted and no vis-expired", tokens)
	}
	if p15HasAttr(resolvedRow, "data-can-reopen") {
		t.Errorf("resolved expired row carries data-can-reopen")
	}
	resolvedTrust := p15ProfileTrust(t, resolvedRow)
	if resolvedTrust["lead_sentence"] != "" {
		t.Errorf("resolved expired row lead_sentence = %q, want empty", resolvedTrust["lead_sentence"])
	}
	noStillCurrent("resolved expired row", resolvedTrust)

	freshRow := p15ProfileRow(t, body, freshID)
	tokens = p15ClassTokens(t, freshRow)
	if got := p15Attr(t, freshRow, "data-expired"); got != "false" {
		t.Errorf("fresh row: data-expired = %q, want false", got)
	}
	if hasToken(tokens, "vis-expired") || hasToken(tokens, "vis-retracted") {
		t.Errorf("fresh row class tokens = %v, want neither vis-expired nor vis-retracted", tokens)
	}
	if p15ProfileTrust(t, freshRow)["lead_sentence"] == expiryLine {
		t.Errorf("fresh row carries the expiry lead sentence")
	}
}

// TestProfileRendersSeverityLabel proves A26 and A58: every row starts its
// body with the server written severity word, before the title.
func TestProfileRendersSeverityLabel(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-p15-severity@example.com")

	cases := []struct{ level, word string }{
		{"low", "Low"},
		{"medium", "Medium"},
		{"critical", "Critical"},
	}
	ids := make([]int64, len(cases))
	for i, c := range cases {
		body := floodReportBody(12.9716, 77.5946)
		body["severity"] = c.level
		ids[i] = submitReportGetID(t, client, srv.URL, body)
	}

	page := getProfile(t, client, srv.URL)
	for i, c := range cases {
		row := p15ProfileRow(t, page, ids[i])
		want := regexp.MustCompile(`<div class="report-row__body">\s*<span class="severity-label severity-label--` +
			c.level + `">` + c.word + `</span>\s*<p class="report-row__title">`)
		if !want.MatchString(row) {
			t.Errorf("%s row does not start its body with the severity label: %s", c.level, row)
		}
		hasSev := false
		for _, tok := range p15ClassTokens(t, row) {
			if tok == "sev-"+c.level {
				hasSev = true
			}
		}
		if !hasSev {
			t.Errorf("%s row lost its sev-%s class", c.level, c.level)
		}
	}
}
