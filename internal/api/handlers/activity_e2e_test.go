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
	"net/url"
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
		start := regexp.MustCompile(`<div class="report-row__body">\s*<span class="activity-card__tag">Your post</span>\s*<p class="report-row__title">`)
		if !start.MatchString(row) {
			t.Errorf("%s row does not start its body with the tag and then the title: %s", c.level, row)
		}
		chip := regexp.MustCompile(`<span class="activity-chip__word activity-chip__word--severity">` + c.word + `</span>`)
		if !chip.MatchString(row) {
			t.Errorf("%s row's chip does not hold the severity word %q: %s", c.level, c.word, row)
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

	for _, want := range []string{
		`class="profile-back-link"`, "activity-record@example.com", "<h2>Activity</h2>",
		`data-report-id="` + strconv.FormatInt(reportID, 10) + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/profile lost %q: %s", want, body)
		}
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
	all := regexp.MustCompile(`<div class="[^"]*"\s+data-tab-empty="all"([^>]*)>`).FindStringSubmatch(body)
	if all == nil || strings.Contains(all[1], "hidden") {
		t.Errorf("/profile's All empty state is missing or hidden: %s", body)
	}
	if !strings.Contains(body, "No activity yet") {
		t.Errorf("/profile lost the All empty state words: %s", body)
	}
	posts := regexp.MustCompile(`<div class="[^"]*"\s+data-tab-empty="posts"([^>]*)>`).FindStringSubmatch(body)
	if posts == nil || !strings.Contains(posts[1], "hidden") {
		t.Errorf("/profile's My posts empty state is missing or not hidden: %s", body)
	}
	if !strings.Contains(body, "No reports yet") {
		t.Errorf("/profile lost the My posts empty state words: %s", body)
	}
}

// getProfileTab GETs /profile?tab=<raw> using client and returns the body.
func getProfileTab(t *testing.T, client *http.Client, baseURL, raw string) string {
	t.Helper()
	resp, err := client.Get(baseURL + "/profile?tab=" + url.QueryEscape(raw))
	if err != nil {
		t.Fatalf("GET /profile?tab=%s: %v", raw, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /profile?tab=%s body: %v", raw, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /profile?tab=%s status = %d, want 200: %s", raw, resp.StatusCode, b)
	}
	return string(b)
}

// TestProfileTabsRenderFromTheAddress proves the server renders the tab named
// in the address (quick task 261013-wm5): the selected link, the hidden flags
// of the cards and the counts. Account one holds two posts (one resolved) and
// votes on two of account two's reports (one confirm, one dispute), so the
// counts are All 4, My posts 2, My votes 2, My disputes 1, Resolved 1 and Not
// resolved 3.
func TestProfileTabsRenderFromTheAddress(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	one := newVerifiedClient(t, srv, mailer, "activity-wm5-tabs-one@example.com")
	two := newVerifiedClient(t, srv, mailer, "activity-wm5-tabs-two@example.com")
	loc := map[string]any{"latitude": 12.9716, "longitude": 77.5946}

	livePost := submitReportGetID(t, one, srv.URL, floodReportBody(12.9716, 77.5946))
	resolvedPost := submitReportGetID(t, one, srv.URL, floodReportBody(12.9716, 77.5946))
	p15VoteTrust(t, postVote(t, one, srv.URL, resolvedPost, "resolve", loc))
	confirmed := submitReportGetID(t, two, srv.URL, floodReportBody(12.9716, 77.5946))
	disputed := submitReportGetID(t, two, srv.URL, floodReportBody(12.9716, 77.5946))
	p15VoteTrust(t, postVote(t, one, srv.URL, confirmed, "confirm", loc))
	p15VoteTrust(t, postVote(t, one, srv.URL, disputed, "dispute", loc))

	type tabWant struct {
		key   string
		count int
		ids   []int64
	}
	tabs := []tabWant{
		{"all", 4, []int64{livePost, resolvedPost, confirmed, disputed}},
		{"posts", 2, []int64{livePost, resolvedPost}},
		{"votes", 2, []int64{confirmed, disputed}},
		{"disputes", 1, []int64{disputed}},
		{"resolved", 1, []int64{resolvedPost}},
		{"not-resolved", 3, []int64{livePost, confirmed, disputed}},
	}
	hrefs := map[string]string{
		"all": "/profile", "posts": "/profile?tab=posts", "votes": "/profile?tab=votes",
		"disputes": "/profile?tab=disputes", "resolved": "/profile?tab=resolved",
		"not-resolved": "/profile?tab=not-resolved",
	}
	allIDs := []int64{livePost, resolvedPost, confirmed, disputed}

	check := func(t *testing.T, body, selected string, want tabWant) {
		t.Helper()
		links := regexp.MustCompile(`<a class="activity-tab" href="([^"]*)" data-tab="([^"]*)" data-count="(\d+)"([^>]*)>`).FindAllStringSubmatch(body, -1)
		if len(links) != 6 {
			t.Fatalf("got %d tab links, want 6: %s", len(links), body)
		}
		var order []string
		for _, l := range links {
			order = append(order, l[2])
			if l[1] != hrefs[l[2]] {
				t.Errorf("tab %q href = %q, want %q", l[2], l[1], hrefs[l[2]])
			}
			isCurrent := strings.Contains(l[4], `aria-current="page"`)
			if isCurrent != (l[2] == selected) {
				t.Errorf("tab %q aria-current = %v, selected tab is %q", l[2], isCurrent, selected)
			}
		}
		if got := strings.Join(order, ","); got != "all,posts,votes,disputes,resolved,not-resolved" {
			t.Errorf("tab order = %s", got)
		}
		for _, w := range tabs {
			for _, l := range links {
				if l[2] == w.key && l[3] != strconv.Itoa(w.count) {
					t.Errorf("tab %q count = %s, want %d", w.key, l[3], w.count)
				}
			}
		}
		visible := 0
		for _, id := range allIDs {
			row := p15ProfileRow(t, body, id)
			opening := row[:strings.Index(row, ">")+1]
			isHidden := p15HasAttr(opening, "hidden")
			shouldShow := false
			for _, w := range want.ids {
				if w == id {
					shouldShow = true
				}
			}
			if isHidden == shouldShow {
				t.Errorf("tab %q: report %d hidden = %v, want %v", selected, id, isHidden, !shouldShow)
			}
			if !isHidden {
				visible++
			}
		}
		if visible != want.count {
			t.Errorf("tab %q shows %d cards, want %d", selected, visible, want.count)
		}
	}

	for _, w := range tabs {
		t.Run(w.key, func(t *testing.T) {
			check(t, getProfileTab(t, one, srv.URL, w.key), w.key, w)
		})
	}
	t.Run("unknown key renders All", func(t *testing.T) {
		check(t, getProfileTab(t, one, srv.URL, "nonsense"), "all", tabs[0])
	})
	t.Run("upper case key renders All", func(t *testing.T) {
		check(t, getProfileTab(t, one, srv.URL, "POSTS"), "all", tabs[0])
	})
	t.Run("no key renders All", func(t *testing.T) {
		check(t, getProfile(t, one, srv.URL), "all", tabs[0])
	})

	// Tags, action phrases and the chip words per kind of card, and each
	// card's tab tokens.
	body := getProfile(t, one, srv.URL)
	cases := []struct {
		id                   int64
		tag, action, tabs, v string
	}{
		{livePost, "Your post", "You posted this report", "posts not-resolved", "vis-provisional"},
		{resolvedPost, "Your post", "You posted this report", "posts resolved", "vis-retracted"},
		{confirmed, "Your vote", "You confirmed this report", "votes not-resolved", "vis-provisional"},
		{disputed, "Your dispute", "You disputed this report", "votes disputes not-resolved", "vis-provisional"},
	}
	for _, c := range cases {
		row := p15ProfileRow(t, body, c.id)
		if !strings.Contains(row, `<span class="activity-card__tag">`+c.tag+`</span>`) {
			t.Errorf("report %d: tag %q missing: %s", c.id, c.tag, row)
		}
		if !strings.Contains(row, `<span class="activity-card__action">`+c.action) {
			t.Errorf("report %d: action %q missing: %s", c.id, c.action, row)
		}
		if got := p15Attr(t, row, "data-tabs"); got != c.tabs {
			t.Errorf("report %d: data-tabs = %q, want %q", c.id, got, c.tabs)
		}
		hasClass := false
		for _, tok := range p15ClassTokens(t, row) {
			if tok == c.v {
				hasClass = true
			}
		}
		if !hasClass {
			t.Errorf("report %d: missing class %s", c.id, c.v)
		}
		for _, word := range []string{">Resolved</span>", ">Not resolved</span>"} {
			if !strings.Contains(row, word) {
				t.Errorf("report %d: chip word %q missing", c.id, word)
			}
		}
	}
	if p15HasAttr(p15ProfileRow(t, body, resolvedPost), "data-can-reopen") == false {
		t.Errorf("the own resolved post lost its Reopen hook")
	}
	for _, id := range []int64{confirmed, disputed, livePost} {
		if p15HasAttr(p15ProfileRow(t, body, id), "data-can-reopen") {
			t.Errorf("report %d carries a Reopen hook, only the own resolved post may", id)
		}
	}
}

// TestProfileRendersLogoutAndBottomNav proves the phone page frame: a Log out
// form that posts to the existing logout route from inside the Activity
// section, and the bottom bar with Activity the current tab.
func TestProfileRendersLogoutAndBottomNav(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "activity-frame@example.com")

	submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))
	body := getProfile(t, client, srv.URL)

	form := regexp.MustCompile(`(?s)<form class="profile-logout" method="post"\s+action="/auth/logout">(.*?)</form>`).FindStringSubmatch(body)
	if form == nil {
		t.Fatalf("/profile has no Log out form posting to /auth/logout: %s", body)
	}
	if !strings.Contains(form[1], `type="submit"`) || !strings.Contains(form[1], "Log out") {
		t.Errorf("Log out form has no submit button: %s", form[1])
	}
	if n := strings.Count(form[1], "<input"); n != 0 {
		t.Errorf("Log out form carries %d input fields, want none", n)
	}
	activity := strings.Index(body, `class="profile-activity"`)
	logout := strings.Index(body, `class="profile-logout"`)
	list := strings.Index(body, `class="profile-report-list"`)
	if activity < 0 || logout < activity || list < logout {
		t.Errorf("Log out form is not inside the Activity section before the list (section %d, form %d, list %d)", activity, logout, list)
	}

	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-10): the
	// two tab Feed and Activity bar is replaced by the four item app bar. My
	// activity is the current place, the Report link opens the report window,
	// and the Log out form above stays until plan 07 rebuilds this page. The
	// Phase 3.1 and Phase 4 hands-on tests are walked on this layout after
	// Phase 3.4 is built.
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
