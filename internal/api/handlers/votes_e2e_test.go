// Package handlers_test also covers the confirm/dispute/resolve/reopen
// mechanic's HTTP surface (02-03b). It drives the real api.NewRouter over a
// real Postgres, the same real-router-plus-real-Postgres shape
// reports_e2e_test.go and auth_e2e_test.go already established, and reuses
// their verifySession/postReport/recordingMailer helpers rather than
// standing up a second parallel harness.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/testutil"
)

// voterGeohashPrecision mirrors internal/service/trust.go's own constant of
// the same name, used here only to compute the same precision-7 cell a
// test's chosen coordinates will land in, so distinctness (or sameness) can
// be asserted up front rather than assumed.
const voterGeohashPrecision = 7

// newVerifiedClient constructs a fresh cookie-jar-backed client and drives
// the real request-link-then-verify flow for email against srv — one client
// equals one browser equals one account. Every test below that needs N
// independent voters calls this N times with N distinct emails.
func newVerifiedClient(t *testing.T, srv *httptest.Server, mailer *recordingMailer, email string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	verifySession(t, client, srv, mailer, email)
	return client
}

// postVote POSTs body (marshalled as JSON) to
// baseURL/api/reports/{reportID}/{action} using client.
func postVote(t *testing.T, client *http.Client, baseURL string, reportID int64, action string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling vote body: %v", err)
	}
	url := baseURL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/" + action
	resp, err := client.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// decodeVoteResponse decodes a successful CastVoteResponse as a client sees
// it — a locally declared anonymous struct, never handlers.CastVoteResponse
// itself, so this test asserts the JSON contract rather than the server's
// internal shape.
func decodeVoteResponse(t *testing.T, resp *http.Response) (visibility, reason string) {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		Visibility string `json:"visibility"`
		Reason     string `json:"reason"`
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding vote response: %v", err)
	}
	return out.Visibility, out.Reason
}

// decodeErrorResponse decodes an ErrorResponse envelope as a client sees
// it, matching decodeVoteResponse's locally-declared-struct idiom.
func decodeErrorResponse(t *testing.T, resp *http.Response) (field, message string) {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	return out.Error.Field, out.Error.Message
}

// submitReportGetID drives postReport (reports_e2e_test.go) and decodes the
// created report's id out of the 201 body — every vote test needs the id,
// and reports_e2e_test.go's own tests discard it.
func submitReportGetID(t *testing.T, client *http.Client, baseURL string, body map[string]any) int64 {
	t.Helper()
	resp := postReport(t, client, baseURL, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 from POST /api/reports, got %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Report struct {
			ID int64 `json:"id"`
		} `json:"report"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding submit report response: %v", err)
	}
	return out.Report.ID
}

// countVotesForReport reads the raw votes row count for reportID directly,
// as internal/store/votes_test.go's tests do — the behavioural proof that a
// refused vote wrote nothing, not just that the HTTP status looked right.
func countVotesForReport(t *testing.T, pool *pgxpool.Pool, reportID int64) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM votes WHERE report_id = $1`, reportID).Scan(&count); err != nil {
		t.Fatalf("counting votes for report %d: %v", reportID, err)
	}
	return count
}

// countAllVotes reads the raw total votes row count — used by the 401 test,
// which deliberately never creates a real report (the gate refuses before
// any report lookup runs).
func countAllVotes(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM votes`).Scan(&count); err != nil {
		t.Fatalf("counting all votes: %v", err)
	}
	return count
}

// assertDistinctCells fails the test if lat1/lon1 and lat2/lon2 encode to
// the same precision-7 geohash cell — a coordinate choice that silently
// drifts into the wrong cell would turn a real independence assertion into
// a tautology.
func assertDistinctCells(t *testing.T, lat1, lon1, lat2, lon2 float64) {
	t.Helper()
	c1 := geohash.EncodeWithPrecision(lat1, lon1, voterGeohashPrecision)
	c2 := geohash.EncodeWithPrecision(lat2, lon2, voterGeohashPrecision)
	if c1 == c2 {
		t.Fatalf("expected distinct precision-7 cells for (%v,%v) and (%v,%v), both encoded to %q", lat1, lon1, lat2, lon2, c1)
	}
}

// assertSameCell fails the test if lat1/lon1 and lat2/lon2 encode to
// different precision-7 geohash cells.
func assertSameCell(t *testing.T, lat1, lon1, lat2, lon2 float64) {
	t.Helper()
	c1 := geohash.EncodeWithPrecision(lat1, lon1, voterGeohashPrecision)
	c2 := geohash.EncodeWithPrecision(lat2, lon2, voterGeohashPrecision)
	if c1 != c2 {
		t.Fatalf("expected the same precision-7 cell for (%v,%v) and (%v,%v), got %q and %q", lat1, lon1, lat2, lon2, c1, c2)
	}
}

// floodReportBody builds a submittable low-severity flood report at
// lat/lon, reused by every test below that needs a fresh report to vote
// against.
func floodReportBody(lat, lon float64) map[string]any {
	return map[string]any{
		"category":    "flood",
		"severity":    "low",
		"description": "Water rising near the bridge",
		"latitude":    lat,
		"longitude":   lon,
	}
}

// TestCastVoteRequiresVerifiedAccount is the 401 case: an unverified caller
// is refused by the gate before the handler ever runs, and no vote row is
// written.
func TestCastVoteRequiresVerifiedAccount(t *testing.T) {
	srv, _, pool := newE2EServer(t)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	resp := postVote(t, client, srv.URL, 1, "confirm", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 401, got %d: %s", resp.StatusCode, b)
	}
	field, _ := decodeErrorResponse(t, resp)
	if field != "auth" {
		t.Fatalf("error.field = %q, want %q", field, "auth")
	}

	if got := countAllVotes(t, pool); got != 0 {
		t.Fatalf("votes row count = %d, want 0 — the gate must refuse before any handler runs", got)
	}
}

// TestCastVoteRejectsReporterSelfVote is the 403 case (D-03, T-02-05): the
// reporter cannot confirm or dispute their own report.
func TestCastVoteRejectsReporterSelfVote(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "self-vote-reporter@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))

	resp := postVote(t, client, srv.URL, reportID, "confirm", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 403 on /confirm, got %d: %s", resp.StatusCode, b)
	}
	_, message := decodeErrorResponse(t, resp)
	if message != "You can't vote on your own report." {
		t.Fatalf("error.message = %q, want %q", message, "You can't vote on your own report.")
	}

	resp2 := postVote(t, client, srv.URL, reportID, "dispute", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	if resp2.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		t.Fatalf("expected 403 on /dispute, got %d: %s", resp2.StatusCode, b)
	}
	_, message2 := decodeErrorResponse(t, resp2)
	if message2 != "You can't vote on your own report." {
		t.Fatalf("error.message = %q, want %q", message2, "You can't vote on your own report.")
	}

	if got := countVotesForReport(t, pool, reportID); got != 0 {
		t.Fatalf("votes row count for report %d = %d, want 0 — refused before the write", reportID, got)
	}
}

// TestReporterCanResolveOwnReportInstantly is D-13's core claim (TRUST-08),
// and exists as the paired counter-case to
// TestCastVoteRejectsReporterSelfVote above: a self-vote block accidentally
// widened past content votes would break this test's instant resolve and
// nothing else in the suite would notice.
func TestReporterCanResolveOwnReportInstantly(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "instant-resolve-reporter@example.com")

	reportID := submitReportGetID(t, client, srv.URL, floodReportBody(12.9716, 77.5946))

	resp := postVote(t, client, srv.URL, reportID, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	visibility, reason := decodeVoteResponse(t, resp)
	if visibility != "retracted" {
		t.Fatalf("visibility = %q, want %q", visibility, "retracted")
	}
	if reason != "resolved" {
		t.Fatalf("reason = %q, want %q", reason, "resolved")
	}
}

// TestIndependentConfirmsFlipProvisionalToLive is TRUST-01's HTTP half,
// TRUST-03, and TRUST-04: one independent confirming cell leaves a report
// Provisional, a second distinct cell flips it to Live.
func TestIndependentConfirmsFlipProvisionalToLive(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "independent-confirms-reporter@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "independent-confirms-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "independent-confirms-c@example.com")

	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(12.9716, 77.5946))

	// B votes from a few hundred metres away from A's report location; C
	// votes from a clearly distinct location several kilometres away.
	bLat, bLon := 12.9746, 77.5946 // roughly 330m north of the report
	cLat, cLon := 13.0500, 77.6500 // several km away, clearly distinct
	assertDistinctCells(t, bLat, bLon, cLat, cLon)

	respB := postVote(t, clientB, srv.URL, reportID, "confirm", map[string]any{"latitude": bLat, "longitude": bLon})
	visB, reasonB := decodeVoteResponse(t, respB)
	if visB != "provisional" {
		t.Fatalf("after B's confirm: visibility = %q, want %q", visB, "provisional")
	}
	if reasonB != "awaiting_second_independent_confirmation" {
		t.Fatalf("after B's confirm: reason = %q, want %q", reasonB, "awaiting_second_independent_confirmation")
	}

	respC := postVote(t, clientC, srv.URL, reportID, "confirm", map[string]any{"latitude": cLat, "longitude": cLon})
	visC, reasonC := decodeVoteResponse(t, respC)
	if visC != "live" {
		t.Fatalf("after C's confirm: visibility = %q, want %q", visC, "live")
	}
	if reasonC != "confirmed" {
		t.Fatalf("after C's confirm: reason = %q, want %q", reasonC, "confirmed")
	}
}

// TestConfirmsFromOneCellStayProvisional is TRUST-03's discriminating case:
// two distinct verified accounts standing in ONE geohash cell are one
// independent confirmation, not two — the entire claim of the Core Value.
func TestConfirmsFromOneCellStayProvisional(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "one-cell-reporter@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "one-cell-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "one-cell-c@example.com")

	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(12.9716, 77.5946))

	sameLat, sameLon := 12.9720, 77.5950
	assertSameCell(t, sameLat, sameLon, sameLat, sameLon)

	respB := postVote(t, clientB, srv.URL, reportID, "confirm", map[string]any{"latitude": sameLat, "longitude": sameLon})
	decodeVoteResponse(t, respB)

	respC := postVote(t, clientC, srv.URL, reportID, "confirm", map[string]any{"latitude": sameLat, "longitude": sameLon})
	visC, reasonC := decodeVoteResponse(t, respC)
	if visC != "provisional" {
		t.Fatalf("after second confirm from the SAME cell: visibility = %q, want %q (two accounts in one cell is one independent confirmation)", visC, "provisional")
	}
	if reasonC != "awaiting_second_independent_confirmation" {
		t.Fatalf("after second confirm from the SAME cell: reason = %q, want %q", reasonC, "awaiting_second_independent_confirmation")
	}
}

// TestReopenRequiresIndependentAgreement is D-16/D-14/TRUST-08 — the only
// test in the phase that reaches the /reopen route at all. /reopen is
// otherwise mounted but never driven, and a non-reporter's reopen still
// requires D-14's one shared threshold under amended D-16 (the reporter's
// own reopen is separately instant and threshold-free — proven elsewhere,
// by 02-03a's TestBuildVoteTallyOnlyReporterSetsReporterReopened and
// 02-07's TestProfileReopenIsInstantForTheReporter — not by this test,
// which uses two non-reporter accounts). A route that no test ever POSTs
// to is a route nobody has confirmed is reachable.
func TestReopenRequiresIndependentAgreement(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "reopen-reporter@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "reopen-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "reopen-c@example.com")

	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(12.9716, 77.5946))

	respResolve := postVote(t, clientA, srv.URL, reportID, "resolve", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	visResolve, _ := decodeVoteResponse(t, respResolve)
	if visResolve != "retracted" {
		t.Fatalf("after reporter's resolve: visibility = %q, want %q", visResolve, "retracted")
	}

	bLat, bLon := 12.9746, 77.5946
	cLat, cLon := 13.0500, 77.6500
	assertDistinctCells(t, bLat, bLon, cLat, cLon)

	respB := postVote(t, clientB, srv.URL, reportID, "reopen", map[string]any{"latitude": bLat, "longitude": bLon})
	visB, _ := decodeVoteResponse(t, respB)
	if visB != "retracted" {
		t.Fatalf("after B's single reopen: visibility = %q, want %q (one standing reopen cell is below the threshold)", visB, "retracted")
	}

	respC := postVote(t, clientC, srv.URL, reportID, "reopen", map[string]any{"latitude": cLat, "longitude": cLon})
	visC, _ := decodeVoteResponse(t, respC)
	if visC == "retracted" {
		t.Fatalf("after C's reopen from a distinct cell: visibility = %q, want it to no longer be retracted (the shared threshold is met)", visC)
	}
}

// TestCastVoteOnExpiredReportIsRejected proves an expired report cannot be
// voted on: RESEARCH Open Question 2's locked answer is reject, not
// accept-and-ignore.
func TestCastVoteOnExpiredReportIsRejected(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "expired-report-voter@example.com")

	reportID := testutil.SeedExpiringReport(t, pool, -3600)

	resp := postVote(t, client, srv.URL, reportID, "confirm", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	if resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, b)
	}
	_, message := decodeErrorResponse(t, resp)
	if message != "This report has expired." {
		t.Fatalf("error.message = %q, want %q", message, "This report has expired.")
	}

	if got := countVotesForReport(t, pool, reportID); got != 0 {
		t.Fatalf("votes row count for expired report %d = %d, want 0", reportID, got)
	}
}

// TestCastVoteRejectsUnknownBodyField is D-17/T-02-01's behavioural proof:
// a client-supplied cell has nowhere to enter, exercised against the
// decoder the handler actually uses rather than a source grep.
func TestCastVoteRejectsUnknownBodyField(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "unknown-field-reporter@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "unknown-field-b@example.com")

	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(12.9716, 77.5946))

	resp := postVote(t, clientB, srv.URL, reportID, "confirm", map[string]any{
		"latitude":     12.9716,
		"longitude":    77.5946,
		"geohash_cell": "tdr1qg0",
	})
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, b)
	}
	field, _ := decodeErrorResponse(t, resp)
	if field != "body" {
		t.Fatalf("error.field = %q, want %q", field, "body")
	}

	if got := countVotesForReport(t, pool, reportID); got != 0 {
		t.Fatalf("votes row count for report %d = %d, want 0", reportID, got)
	}
}

// TestCastVoteOnMissingReportIs404 proves a vote against a nonexistent
// report id is refused with 404.
func TestCastVoteOnMissingReportIs404(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "missing-report-voter@example.com")

	resp := postVote(t, client, srv.URL, 999999, "confirm", map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 404, got %d: %s", resp.StatusCode, b)
	}
	resp.Body.Close()
}
