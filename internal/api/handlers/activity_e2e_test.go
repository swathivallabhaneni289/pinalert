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
	"io"
	"net/http"
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

	body := getProfile(t, client, srv.URL)
	if !strings.Contains(body, "vis-retracted") {
		t.Errorf("/profile does not carry the vis-retracted state class for the resolved report: %s", body)
	}
	if !strings.Contains(body, "Resolved") {
		t.Errorf("/profile does not render the \"Resolved\" visibility-tag label: %s", body)
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
