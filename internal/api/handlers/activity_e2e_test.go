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
	const expiryLine = "Expired, so it can no longer be confirmed."

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

// TestProfileRendersSeverityLabel proves A26 and A58: every card carries the
// server written severity word.
//
// Departure (quick task 261013-wm5, A26 and A58): the picture's card moves the
// severity word from the start of the body into the chip on the right. The body
// now starts with the tag and then the title, and the chip's severity span holds
// the word. The sev class check is kept.
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
		// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to
		// D-21): the Report cell starts with the title, and the severity word is
		// kept as a visually hidden word in the Status cell next to the chip
		// (the tint alone must not carry the severity). The Phase 3.1 and Phase 4
		// hands-on tests are walked on this layout after Phase 3.4 is built.
		start := regexp.MustCompile(`<div class="activity-card__report">\s*<p class="report-row__title">`)
		if !start.MatchString(row) {
			t.Errorf("%s row does not start its Report cell with the title: %s", c.level, row)
		}
		severity := regexp.MustCompile(`<span class="visually-hidden">` + c.word + ` severity\. </span><span class="activity-chip"`)
		if !severity.MatchString(row) {
			t.Errorf("%s row's Status cell does not hold the severity word %q: %s", c.level, c.word, row)
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

// --- Quick task 261013-wm5: the reports the account voted on ---

// TestProfileListsReportsTheAccountVotedOn proves My votes and My disputes read
// the account's own votes: account A confirmed one of B's reports and disputed
// another, and each of A's cards carries exactly the trust the vote response
// sent, no identifier, and no Reopen hook. A third account's vote on a third
// report never reaches A's page.
func TestProfileListsReportsTheAccountVotedOn(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	aEmail := "activity-wm5-voter-a@example.com"
	bEmail := "activity-wm5-poster-b@example.com"
	a := newVerifiedClient(t, srv, mailer, aEmail)
	b := newVerifiedClient(t, srv, mailer, bEmail)
	c := newVerifiedClient(t, srv, mailer, "activity-wm5-third-c@example.com")

	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}
	confirmedID := submitReportGetID(t, b, srv.URL, floodReportBody(12.9716, 77.5946))
	disputedID := submitReportGetID(t, b, srv.URL, floodReportBody(12.9716, 77.5946))
	thirdID := submitReportGetID(t, b, srv.URL, floodReportBody(12.9716, 77.5946))

	confirmTrust := p15VoteTrust(t, postVote(t, a, srv.URL, confirmedID, "confirm", loc))
	disputeTrust := p15VoteTrust(t, postVote(t, a, srv.URL, disputedID, "dispute", loc))
	p15VoteTrust(t, postVote(t, c, srv.URL, thirdID, "confirm", loc))

	resp, err := a.Get(srv.URL + "/profile")
	if err != nil {
		t.Fatalf("GET /profile: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("/profile Cache-Control = %q, want no-store", got)
	}
	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)

	for _, tc := range []struct {
		id    int64
		trust map[string]any
	}{{confirmedID, confirmTrust}, {disputedID, disputeTrust}} {
		row := p15ProfileRow(t, body, tc.id)
		rowTrust := p15ProfileTrust(t, row)
		if !reflect.DeepEqual(rowTrust, tc.trust) {
			t.Errorf("report %d: data-trust differs from the vote response trust:\nrow  = %v\nvote = %v", tc.id, rowTrust, tc.trust)
		}
		p15WalkKeys(rowTrust, func(key string) {
			if p15IdentifierKey.MatchString(key) {
				t.Errorf("report %d: data-trust carries an identifier-like key %q", tc.id, key)
			}
		})
		attr := p15Attr(t, row, "data-trust")
		for _, forbidden := range []string{"@", aEmail, bEmail} {
			if strings.Contains(attr, forbidden) {
				t.Errorf("report %d: data-trust contains %q", tc.id, forbidden)
			}
		}
		if p15HasAttr(row, "data-can-reopen") {
			t.Errorf("report %d: a voted row carries data-can-reopen", tc.id)
		}
	}
	if strings.Contains(body, `data-report-id="`+strconv.FormatInt(thirdID, 10)+`"`) {
		t.Errorf("A's page lists report %d, which only a third account voted on", thirdID)
	}

	// B's own row for the confirmed report carries the same trust A saw.
	bRow := p15ProfileRow(t, getProfile(t, b, srv.URL), confirmedID)
	if got := p15ProfileTrust(t, bRow); !reflect.DeepEqual(got, confirmTrust) {
		t.Errorf("B's own row trust differs from the confirm response trust:\nrow  = %v\nvote = %v", got, confirmTrust)
	}
}

// TestProfileVotedRowNeverOffersReopen proves the Reopen hook stays on the
// reporter's own card: after B resolves a report A confirmed, A's card shows
// the resolved state and no hook, and B's card carries the hook.
func TestProfileVotedRowNeverOffersReopen(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	a := newVerifiedClient(t, srv, mailer, "activity-wm5-reopen-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "activity-wm5-reopen-b@example.com")

	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}
	reportID := submitReportGetID(t, b, srv.URL, floodReportBody(12.9716, 77.5946))
	p15VoteTrust(t, postVote(t, a, srv.URL, reportID, "confirm", loc))
	p15VoteTrust(t, postVote(t, b, srv.URL, reportID, "resolve", loc))

	aRow := p15ProfileRow(t, getProfile(t, a, srv.URL), reportID)
	hasRetracted := false
	for _, tok := range p15ClassTokens(t, aRow) {
		if tok == "vis-retracted" {
			hasRetracted = true
		}
	}
	if !hasRetracted {
		t.Errorf("A's card is not retracted after the reporter resolved it: %s", aRow)
	}
	if p15HasAttr(aRow, "data-can-reopen") {
		t.Errorf("A's voted card carries data-can-reopen: %s", aRow)
	}

	bRow := p15ProfileRow(t, getProfile(t, b, srv.URL), reportID)
	if !p15HasAttr(bRow, "data-can-reopen") {
		t.Errorf("B's own resolved card carries no data-can-reopen: %s", bRow)
	}
}

// TestProfileVoterWithoutPostsHasNoRecord proves an account that only voted
// gets its cards and no reporter record, because the record is the account's
// own posting history.
func TestProfileVoterWithoutPostsHasNoRecord(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	a := newVerifiedClient(t, srv, mailer, "activity-wm5-norecord-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "activity-wm5-norecord-b@example.com")

	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}
	reportID := submitReportGetID(t, b, srv.URL, floodReportBody(12.9716, 77.5946))
	p15VoteTrust(t, postVote(t, a, srv.URL, reportID, "confirm", loc))

	body := getProfile(t, a, srv.URL)
	p15ProfileRow(t, body, reportID)
	if strings.Contains(body, `class="profile-record"`) || strings.Contains(body, "Your reporter record") {
		t.Errorf("a voter with no posts got a reporter record card: %s", body)
	}
}

// --- Quick task 261013-lrw: the phone Activity page ---

// TestProfileRendersTheReporterRecord proves the record card is built from the
// account's own reporter signal: a fresh account with one report gets the
// server's New reporter label and its real counts (nothing invented), and the
// page keeps the back link, the email, the Activity heading and the row with
// its data attributes.
func TestProfileRendersTheReporterRecord(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-record@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	body := getProfile(t, client, srv.URL)

	section := regexp.MustCompile(`(?s)<section class="profile-record"[^>]*>(.*?)</section>`).FindStringSubmatch(body)
	if section == nil {
		t.Fatalf("/profile has no reporter record card: %s", body)
	}
	for _, want := range []string{"Your reporter record", "New reporter", "0 of 0 confirmed"} {
		if !strings.Contains(section[1], want) {
			t.Errorf("record card does not contain %q: %s", want, section[1])
		}
	}
	if !strings.Contains(body, `aria-label="Your reporter record"`) {
		t.Errorf("record card has no accessible name: %s", body)
	}

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21,
	// T-34-32): the back link and the email line are gone from the page, the
	// heading is My activity, and the record card sits in the Posted by you
	// part. The Phase 3.1 and Phase 4 hands-on tests are walked on this layout
	// after Phase 3.4 is built.
	for _, want := range []string{
		`<h1 class="activity-heading">My activity</h1>`,
		`data-report-id="` + strconv.FormatInt(reportID, 10) + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/profile lost %q: %s", want, body)
		}
	}
	for _, gone := range []string{`class="profile-back-link"`, "activity-record@example.com", `class="profile-email"`} {
		if strings.Contains(body, gone) {
			t.Errorf("/profile still holds %q: %s", gone, body)
		}
	}
	postedAt := strings.Index(body, `<section class="activity-part activity-part--posted"`)
	confirmedAt := strings.Index(body, `<section class="activity-part activity-part--confirmed"`)
	recordAt := strings.Index(body, `<section class="profile-record"`)
	if postedAt == -1 || confirmedAt == -1 || recordAt < postedAt || recordAt > confirmedAt {
		t.Errorf("the record card is not inside the Posted by you part (posted %d, record %d, confirmed %d)", postedAt, recordAt, confirmedAt)
	}
}

// TestProfileWithoutReportsRendersNoRecord proves nothing is invented for an
// account that has filed nothing: no record card, and the empty state of the
// All tab shows.
//
// Departure (quick task 261013-wm5): the visible empty state of an empty
// account is the All tab's. "No reports yet" stays as the My posts tab's own
// empty state, present in the page and hidden.
func TestProfileWithoutReportsRendersNoRecord(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-norecord@example.com")

	body := getProfile(t, client, srv.URL)
	if strings.Contains(body, `class="profile-record"`) || strings.Contains(body, "Your reporter record") {
		t.Errorf("/profile renders a record card for an account with no reports: %s", body)
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21):
	// there is no All view now; every part of an empty account shows its own
	// empty state in its box, with the UI-SPEC 3c words. The Phase 3.1 and Phase
	// 4 hands-on tests are walked on this layout after Phase 3.4 is built.
	for _, words := range []string{
		"No updates yet", "Changes on reports you posted or voted on will show here.",
		"No reports yet", "Reports you post will appear here.",
		"No confirmations yet", "Reports you confirm will appear here.",
		"No disputes yet", "Reports you dispute will appear here.",
		"No ended reports nearby", "Nothing in this area was resolved or faded out in the last 7 days.",
	} {
		if !strings.Contains(body, words) {
			t.Errorf("/profile lost the empty state words %q: %s", words, body)
		}
	}
	// The four row parts hold a visible empty state; the Ended nearby one is
	// the shell's hidden empty state.
	visibleEmpty := regexp.MustCompile(`<div class="empty-state profile-empty-state activity-empty">`)
	if n := len(visibleEmpty.FindAllString(body, -1)); n != 4 {
		t.Errorf("/profile has %d visible empty states, want 4 (Updates, Posted by you, You confirmed, You disputed)", n)
	}
	if strings.Contains(body, `class="profile-report-list"`) {
		t.Errorf("/profile renders a report list for an account with no reports")
	}
}

// getProfileQuery GETs /profile with the given raw query string using client and
// returns the body.
func getProfileQuery(t *testing.T, client *http.Client, baseURL, rawQuery string) string {
	t.Helper()
	resp, err := client.Get(baseURL + "/profile?" + rawQuery)
	if err != nil {
		t.Fatalf("GET /profile?%s: %v", rawQuery, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile?%s body: %v", rawQuery, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /profile?%s status = %d, want 200: %s", rawQuery, resp.StatusCode, b)
	}
	return string(b)
}

// partSection returns the markup of one part's section, cut from its opening tag
// to the opening tag of the next section (or the end of the main element).
func partSection(t *testing.T, body, key string) string {
	t.Helper()
	marker := `<section class="activity-part activity-part--` + key + `"`
	start := strings.Index(body, marker)
	if start == -1 {
		t.Fatalf("/profile has no section for part %q: %s", key, body)
	}
	rest := body[start+len(marker):]
	end := len(rest)
	if next := strings.Index(rest, `<section class="activity-part `); next != -1 {
		end = next
	}
	if mainEnd := strings.Index(rest, `</main>`); mainEnd != -1 && mainEnd < end {
		end = mainEnd
	}
	return rest[:end]
}

// TestActivityPartsRenderFromTheAddress proves the server renders the page the
// address names (Phase 3.4, D-18 to D-21). Account one holds two posts (one
// resolved) and votes on two of account two's reports (one confirm, one
// dispute), so the counts are Posted by you 2, You confirmed 1 and You disputed
// 1. The list (/profile) has an empty data-part, the hub of five rows whose
// counts equal the rows in each section, and five sections with the ids
// part-updates, part-posted, part-confirmed, part-disputed and part-ended in that
// order. A part address renders that part as the current one, with the part name
// as the top bar title and the back arrow to /profile.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21):
// this replaces TestProfileTabsRenderFromTheAddress, which pinned the six tab
// page. The Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
// Phase 3.4 is built.
func TestActivityPartsRenderFromTheAddress(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	one := newVerifiedClient(t, srv, mailer, "activity-parts-one@example.com")
	two := newVerifiedClient(t, srv, mailer, "activity-parts-two@example.com")
	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}

	livePost := submitReportGetID(t, one, srv.URL, floodReportBody(12.9716, 77.5946))
	resolvedPost := submitReportGetID(t, one, srv.URL, floodReportBody(12.9716, 77.5946))
	p15VoteTrust(t, postVote(t, one, srv.URL, resolvedPost, "resolve", loc))
	confirmed := submitReportGetID(t, two, srv.URL, floodReportBody(12.9716, 77.5946))
	disputed := submitReportGetID(t, two, srv.URL, floodReportBody(12.9716, 77.5946))
	p15VoteTrust(t, postVote(t, one, srv.URL, confirmed, "confirm", loc))
	p15VoteTrust(t, postVote(t, one, srv.URL, disputed, "dispute", loc))

	keys := []string{"updates", "posted", "confirmed", "disputed", "ended"}
	wantIDs := map[string][]int64{
		"updates":   nil,
		"posted":    {resolvedPost, livePost},
		"confirmed": {confirmed},
		"disputed":  {disputed},
		"ended":     nil,
	}
	names := map[string]string{
		"updates": "Updates", "posted": "Posted by you", "confirmed": "You confirmed",
		"disputed": "You disputed", "ended": "Ended nearby",
	}

	t.Run("the list", func(t *testing.T) {
		body := getProfile(t, one, srv.URL)
		if !strings.Contains(body, `data-part=""`) {
			t.Errorf("/profile does not render an empty data-part: %s", body)
		}
		if !strings.Contains(body, `<h1 class="app-topbar__title">My activity</h1>`) || strings.Contains(body, `class="app-topbar__back"`) {
			t.Errorf("/profile top bar must be titled My activity with no back arrow")
		}

		// The five sections, in order.
		ids := regexp.MustCompile(`<section class="activity-part [^"]*" id="(part-[a-z]+)"`).FindAllStringSubmatch(body, -1)
		var gotIDs []string
		for _, m := range ids {
			gotIDs = append(gotIDs, m[1])
		}
		if got := strings.Join(gotIDs, ","); got != "part-updates,part-posted,part-confirmed,part-disputed,part-ended" {
			t.Errorf("section ids = %s", got)
		}
		if n := strings.Count(body, `id="part-`); n != 10 {
			// Five sections and five title ids.
			t.Errorf(`found %d id="part- attributes, want 10 (five sections and five titles)`, n)
		}
		if strings.Contains(body, " data-current") {
			t.Errorf("the list marks a part as current")
		}

		// The hub: five links, in order, with the counts of the rows in each
		// section.
		hub := regexp.MustCompile(`(?s)<nav class="activity-hub" aria-label="My activity">(.*?)</nav>`).FindStringSubmatch(body)
		if hub == nil {
			t.Fatalf("/profile has no hub: %s", body)
		}
		rows := regexp.MustCompile(`(?s)<a class="activity-hub__row" href="([^"]*)" data-part-link="([^"]*)">(.*?)</a>`).FindAllStringSubmatch(hub[1], -1)
		if len(rows) != 5 {
			t.Fatalf("the hub has %d rows, want 5: %s", len(rows), hub[1])
		}
		for i, r := range rows {
			key := keys[i]
			if r[2] != key || r[1] != "/profile?part="+key {
				t.Errorf("hub row %d is key %q href %q, want %q and /profile?part=%s", i, r[2], r[1], key, key)
			}
			if !strings.Contains(r[3], `activity-hub__name">`+names[key]+`</span>`) {
				t.Errorf("hub row %q does not carry the name %q: %s", key, names[key], r[3])
			}
			count := regexp.MustCompile(`activity-hub__count">(\d+)</span>`).FindStringSubmatch(r[3])
			rowCount := strings.Count(partSection(t, body, key), `<li class="report-row`)
			if rowCount != len(wantIDs[key]) {
				t.Errorf("part %q holds %d rows, want %d", key, rowCount, len(wantIDs[key]))
			}
			switch key {
			case "updates", "ended":
				if count != nil {
					t.Errorf("hub row %q shows a count %s, the server holds no rows for it", key, count[1])
				}
			default:
				if count == nil || count[1] != strconv.Itoa(rowCount) {
					t.Errorf("hub row %q count = %v, want the %d rows of its section", key, count, rowCount)
				}
			}
		}

		// Each report is in exactly the part its kind belongs to.
		for key, ids := range wantIDs {
			section := partSection(t, body, key)
			for _, id := range ids {
				if !strings.Contains(section, `data-report-id="`+strconv.FormatInt(id, 10)+`"`) {
					t.Errorf("part %q lacks report %d", key, id)
				}
			}
		}
		for _, id := range []int64{livePost, resolvedPost, confirmed, disputed} {
			if n := strings.Count(body, `data-report-id="`+strconv.FormatInt(id, 10)+`"`); n != 1 {
				t.Errorf("report %d is on the page %d times, want exactly once", id, n)
			}
		}
	})

	t.Run("a part screen", func(t *testing.T) {
		for _, key := range keys {
			body := getProfileQuery(t, one, srv.URL, "part="+key)
			if !strings.Contains(body, `data-part="`+key+`"`) {
				t.Errorf("part %q: data-part is not %q", key, key)
			}
			wantTitle := `<h1 class="app-topbar__title">` + names[key] + `</h1>`
			if !strings.Contains(body, wantTitle) {
				t.Errorf("part %q: the top bar title %s is missing", key, wantTitle)
			}
			if !strings.Contains(body, `<a class="app-topbar__back" href="/profile"`) {
				t.Errorf("part %q: the back arrow to /profile is missing", key)
			}
			if n := strings.Count(body, " data-current"); n != 1 {
				t.Errorf("part %q: %d sections are marked current, want 1", key, n)
			}
			if !strings.Contains(partSection(t, body, key), " data-current") {
				// The marker sits in the section's own opening tag, which
				// partSection cuts off, so check the tag itself.
				if !strings.Contains(body, `data-part-key="`+key+`" data-current`) {
					t.Errorf("part %q: its own section is not marked current", key)
				}
			}
		}
	})

	t.Run("an unknown part renders the list", func(t *testing.T) {
		for _, raw := range []string{"part=nonsense", "part=POSTED", "part=posted%20", "part="} {
			body := getProfileQuery(t, one, srv.URL, raw)
			if !strings.Contains(body, `data-part=""`) {
				t.Errorf("%s did not render the list", raw)
			}
		}
	})
}

// TestActivityTabAliases proves the old tab addresses of the six tab page still
// work as read only aliases (D-18): posts lands on Posted by you, votes on You
// confirmed, disputes on You disputed, and all, resolved and not-resolved on the
// list; an unknown or upper case tab lands on the list, and a valid part wins over
// a tab sent with it.
//
// Added 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21). The
// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after Phase 3.4
// is built.
func TestActivityTabAliases(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-aliases@example.com")
	submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))

	cases := []struct{ query, wantPart string }{
		{"tab=posts", "posted"},
		{"tab=votes", "confirmed"},
		{"tab=disputes", "disputed"},
		{"tab=all", ""},
		{"tab=resolved", ""},
		{"tab=not-resolved", ""},
		{"tab=nonsense", ""},
		{"tab=POSTS", ""},
		{"tab=", ""},
		{"part=ended&tab=posts", "ended"},
		{"part=nonsense&tab=votes", "confirmed"},
	}
	for _, c := range cases {
		body := getProfileQuery(t, client, srv.URL, c.query)
		if !strings.Contains(body, `data-part="`+c.wantPart+`"`) {
			t.Errorf("/profile?%s did not render data-part=%q", c.query, c.wantPart)
		}
	}
}

// TestProfileRendersLogoutAndBottomNav proves the page frame: the four item app
// bar with My activity the current place, and no Log out form on the page.
//
// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-18 to D-21): the
// Log out form is removed from the Activity page. Log out lives in Settings and in
// the person menu of the laptop top bar (account_header.html.tmpl), which is the
// only logout form the page carries. The name is kept so the history of the test
// reads on. The Phase 3.1 and Phase 4 hands-on tests are walked on this layout
// after Phase 3.4 is built.
func TestProfileRendersLogoutAndBottomNav(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-frame@example.com")

	submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	body := getProfile(t, client, srv.URL)

	if strings.Contains(body, `class="profile-logout"`) {
		t.Errorf("/profile still carries the page's own Log out form: %s", body)
	}
	// The person menu of the top bar is the one place the page logs out from.
	if n := strings.Count(body, `action="/auth/logout"`); n != 1 {
		t.Errorf("/profile has %d logout forms, want exactly the person menu's one", n)
	}
	if strings.Contains(body, "activity-frame@example.com") {
		t.Errorf("/profile prints the signed in address (T-34-32): %s", body)
	}

	if strings.Contains(body, "bottom-nav") {
		t.Errorf("/profile still carries the old two tab bottom nav")
	}
	nav := regexp.MustCompile(`(?s)<nav class="app-bar"[^>]*>(.*?)</nav>`).FindStringSubmatch(body)
	if nav == nil {
		t.Fatalf("/profile has no app bar: %s", body)
	}
	tabs := regexp.MustCompile(`<a class="app-bar__item[^"]*" href="([^"]*)"([^>]*)>.*?<span class="app-bar__label">([^<]*)</span></a>`).FindAllStringSubmatch(nav[1], -1)
	if len(tabs) != 4 {
		t.Fatalf("app bar has %d items, want 4: %s", len(tabs), nav[1])
	}
	for i, want := range []struct {
		href, label string
		current     bool
	}{{"/", "Nearby", false}, {"/alerts", "Official alerts", false}, {"/profile", "My activity", true}, {"/?post=1", "Report", false}} {
		got := tabs[i]
		if got[1] != want.href || got[3] != want.label || strings.Contains(got[2], "aria-current") != want.current {
			t.Errorf("item %d is href %q label %q attrs %q, want %q %q current=%v", i, got[1], got[3], got[2], want.href, want.label, want.current)
		}
	}
	if strings.Contains(nav[1], "Past") {
		t.Errorf("bottom nav names a Past tab")
	}
}
