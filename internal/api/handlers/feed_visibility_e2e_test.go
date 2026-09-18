// feed_visibility_e2e_test.go covers 02-04's read side: GET /api/reports'
// resolver-backed visibility, show_disputed, your_vote and is_own_report,
// proven end to end over the real router (internal/api.NewRouter) and a
// real Postgres. It reuses newE2EServer, verifySession, postReport (from
// reports_e2e_test.go) and newVerifiedClient, postVote, decodeVoteResponse,
// decodeErrorResponse (from votes_e2e_test.go) rather than standing up a
// second parallel harness.
package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"
)

// feedReport is the client-side view of one FeedReportResponse element,
// decoded locally rather than by importing handlers.FeedReportResponse, so
// these tests assert the JSON contract rather than the server's internal
// shape. YourVote is *string so the null-vs-"confirm" distinction is real.
type feedReport struct {
	ID               int64   `json:"id"`
	Visibility       string  `json:"visibility"`
	VisibilityReason string  `json:"visibility_reason"`
	YourVote         *string `json:"your_vote"`
	IsOwnReport      bool    `json:"is_own_report"`
}

// getNearby builds and issues a GET /api/reports request, appending
// &show_disputed=true only when asked.
func getNearby(t *testing.T, client *http.Client, baseURL string, lat, lon float64, showDisputed bool) *http.Response {
	t.Helper()
	url := baseURL + "/api/reports?lat=" + strconv.FormatFloat(lat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(lon, 'f', -1, 64) + "&radius_km=5"
	if showDisputed {
		url += "&show_disputed=true"
	}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// decodeFeedResponse decodes a successful GET /api/reports response into
// the client-side feedReport shape.
func decodeFeedResponse(t *testing.T, resp *http.Response) []feedReport {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Reports []feedReport `json:"reports"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding feed response: %v", err)
	}
	return out.Reports
}

func findFeedReport(reports []feedReport, id int64) (feedReport, bool) {
	for _, r := range reports {
		if r.ID == id {
			return r, true
		}
	}
	return feedReport{}, false
}

// TestFeedExcludesHiddenByDefault is D-10's core claim: a report disputed
// past the threshold is absent from the default feed, while an unvoted
// control report stays present as provisional.
func TestFeedExcludesHiddenByDefault(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "feed-hidden-default-a@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "feed-hidden-default-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "feed-hidden-default-c@example.com")

	const lat, lon = 12.9716, 77.5946
	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(lat, lon))
	controlID := submitReportGetID(t, clientA, srv.URL, floodReportBody(lat, lon))

	bLat, bLon := 12.9746, 77.5946
	cLat, cLon := 13.0500, 77.6500
	assertDistinctCells(t, bLat, bLon, cLat, cLon)

	decodeVoteResponse(t, postVote(t, clientB, srv.URL, reportID, "dispute", map[string]any{"latitude": bLat, "longitude": bLon}))
	decodeVoteResponse(t, postVote(t, clientC, srv.URL, reportID, "dispute", map[string]any{"latitude": cLat, "longitude": cLon}))

	reports := decodeFeedResponse(t, getNearby(t, clientA, srv.URL, lat, lon, false))
	if r, ok := findFeedReport(reports, reportID); ok {
		t.Fatalf("hidden report %d present in the default view: %+v", reportID, r)
	}
	control, ok := findFeedReport(reports, controlID)
	if !ok {
		t.Fatalf("control report %d absent from the default view", controlID)
	}
	if control.Visibility != "provisional" {
		t.Fatalf("control report visibility = %q, want %q", control.Visibility, "provisional")
	}
}

// TestShowDisputedRevealsHiddenReports is D-10/D-11/TRUST-02: show_disputed
// additionally returns the hidden report — and because GET /api/reports is
// the one endpoint serving both the map and the list (reports.go's own
// package/handler doc comments), this single response satisfies D-11's "the
// toggle also reveals Hidden pins on the map" without a second code path
// existing to drift.
func TestShowDisputedRevealsHiddenReports(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "feed-show-disputed-a@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "feed-show-disputed-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "feed-show-disputed-c@example.com")

	const lat, lon = 12.9716, 77.5946
	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(lat, lon))
	controlID := submitReportGetID(t, clientA, srv.URL, floodReportBody(lat, lon))

	bLat, bLon := 12.9746, 77.5946
	cLat, cLon := 13.0500, 77.6500
	assertDistinctCells(t, bLat, bLon, cLat, cLon)

	decodeVoteResponse(t, postVote(t, clientB, srv.URL, reportID, "dispute", map[string]any{"latitude": bLat, "longitude": bLon}))
	decodeVoteResponse(t, postVote(t, clientC, srv.URL, reportID, "dispute", map[string]any{"latitude": cLat, "longitude": cLon}))

	reports := decodeFeedResponse(t, getNearby(t, clientA, srv.URL, lat, lon, true))
	r, ok := findFeedReport(reports, reportID)
	if !ok {
		t.Fatalf("disputed report %d absent under show_disputed=true", reportID)
	}
	if r.Visibility != "hidden" || r.VisibilityReason != "disputed" {
		t.Fatalf("got visibility=%q reason=%q, want %q/%q", r.Visibility, r.VisibilityReason, "hidden", "disputed")
	}
	if _, ok := findFeedReport(reports, controlID); !ok {
		t.Fatalf("control report %d absent under show_disputed=true — the toggle should add reports, not replace the view", controlID)
	}
}

// TestFeedNeverReturnsRetractedReports is D-12/D-07/TRUST-08: a resolved
// report is absent from both the default and the show_disputed views, and
// severity does not rescue it. A critical report Hidden BY DISPUTES is
// deliberately not tested: D-06 exempts critical/rescue_needed from the
// Hidden-by-dispute trigger entirely, so that state is unreachable. The
// reachable assertion is a critical Retracted report — D-07 puts Retracted
// above the critical bypass.
func TestFeedNeverReturnsRetractedReports(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "feed-retracted-a@example.com")

	const lat, lon = 12.9716, 77.5946

	lowID := submitReportGetID(t, client, srv.URL, floodReportBody(lat, lon))
	visLow, reasonLow := decodeVoteResponse(t, postVote(t, client, srv.URL, lowID, "resolve", map[string]any{"latitude": lat, "longitude": lon}))
	if visLow != "retracted" || reasonLow != "resolved" {
		t.Fatalf("after resolve: visibility=%q reason=%q, want %q/%q", visLow, reasonLow, "retracted", "resolved")
	}

	for _, showDisputed := range []bool{false, true} {
		reports := decodeFeedResponse(t, getNearby(t, client, srv.URL, lat, lon, showDisputed))
		if _, ok := findFeedReport(reports, lowID); ok {
			t.Fatalf("show_disputed=%v: retracted report %d present, want absent", showDisputed, lowID)
		}
	}

	criticalID := submitReportGetID(t, client, srv.URL, map[string]any{
		"category":    "rescue_needed",
		"severity":    "critical",
		"description": "Person trapped, needs rescue urgently",
		"latitude":    lat,
		"longitude":   lon,
	})
	visCrit, reasonCrit := decodeVoteResponse(t, postVote(t, client, srv.URL, criticalID, "resolve", map[string]any{"latitude": lat, "longitude": lon}))
	if visCrit != "retracted" || reasonCrit != "resolved" {
		t.Fatalf("after critical resolve: visibility=%q reason=%q, want %q/%q", visCrit, reasonCrit, "retracted", "resolved")
	}

	for _, showDisputed := range []bool{false, true} {
		reports := decodeFeedResponse(t, getNearby(t, client, srv.URL, lat, lon, showDisputed))
		if _, ok := findFeedReport(reports, criticalID); ok {
			t.Fatalf("show_disputed=%v: critical/rescue_needed retracted report %d present, want absent (D-07 outranks the critical bypass)", showDisputed, criticalID)
		}
	}
}

// TestFeedCarriesViewerVoteAndOwnReportFlag is D-03/D-02: is_own_report and
// your_vote are relative to the calling account alone and never leak
// another viewer's standing.
func TestFeedCarriesViewerVoteAndOwnReportFlag(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "feed-viewer-vote-a@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "feed-viewer-vote-b@example.com")

	const lat, lon = 12.9716, 77.5946
	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(lat, lon))

	repA, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientA, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from A's own feed", reportID)
	}
	if !repA.IsOwnReport {
		t.Fatalf("A's own report: is_own_report = false, want true")
	}
	if repA.YourVote != nil {
		t.Fatalf("A's own report: your_vote = %v, want null", *repA.YourVote)
	}

	repB, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientB, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from B's feed", reportID)
	}
	if repB.IsOwnReport {
		t.Fatalf("B's view of A's report: is_own_report = true, want false")
	}
	if repB.YourVote != nil {
		t.Fatalf("B's view before voting: your_vote = %v, want null", *repB.YourVote)
	}

	decodeVoteResponse(t, postVote(t, clientB, srv.URL, reportID, "confirm", map[string]any{"latitude": lat, "longitude": lon}))

	repB2, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientB, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from B's feed after confirming", reportID)
	}
	if repB2.YourVote == nil || *repB2.YourVote != "confirm" {
		t.Fatalf("B's view after confirm: your_vote = %v, want %q", repB2.YourVote, "confirm")
	}
	if repB2.IsOwnReport {
		t.Fatalf("B's view after confirm: is_own_report = true, want false")
	}

	repA2, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientA, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from A's feed after B confirmed", reportID)
	}
	if repA2.YourVote != nil {
		t.Fatalf("A's view after B's confirm: your_vote = %v, want null — one viewer's vote must never leak into another viewer's response", *repA2.YourVote)
	}

	decodeVoteResponse(t, postVote(t, clientB, srv.URL, reportID, "dispute", map[string]any{"latitude": lat, "longitude": lon}))

	repB3, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientB, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from B's feed after disputing", reportID)
	}
	if repB3.YourVote == nil || *repB3.YourVote != "dispute" {
		t.Fatalf("B's view after dispute: your_vote = %v, want %q", repB3.YourVote, "dispute")
	}
}

// TestFeedVisibilityMatchesCastVoteResponse is TRUST-02/T-02-04's headline
// case: the feed's answer for a report must equal the vote endpoint's
// answer for that same report moments earlier. This — not any grep — is the
// real proof that the write response and the read path share one resolver;
// it would fail the instant a cached status column, a SQL-level visibility
// filter, or a handler-side re-derivation were introduced.
func TestFeedVisibilityMatchesCastVoteResponse(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	clientA := newVerifiedClient(t, srv, mailer, "feed-matches-vote-a@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "feed-matches-vote-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "feed-matches-vote-c@example.com")

	const lat, lon = 12.9716, 77.5946
	reportID := submitReportGetID(t, clientA, srv.URL, floodReportBody(lat, lon))

	bLat, bLon := 12.9746, 77.5946
	cLat, cLon := 13.0500, 77.6500
	assertDistinctCells(t, bLat, bLon, cLat, cLon)

	voteVisB, voteReasonB := decodeVoteResponse(t, postVote(t, clientB, srv.URL, reportID, "confirm", map[string]any{"latitude": bLat, "longitude": bLon}))
	feedRepB, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientB, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from B's feed after B's own confirm", reportID)
	}
	if feedRepB.Visibility != voteVisB {
		t.Fatalf("after B's confirm: feed visibility %q != vote response visibility %q", feedRepB.Visibility, voteVisB)
	}
	if feedRepB.VisibilityReason != voteReasonB {
		t.Fatalf("after B's confirm: feed visibility_reason %q != vote response reason %q", feedRepB.VisibilityReason, voteReasonB)
	}
	if voteVisB != "provisional" {
		t.Fatalf("after B's confirm: visibility = %q, want %q", voteVisB, "provisional")
	}

	voteVisC, voteReasonC := decodeVoteResponse(t, postVote(t, clientC, srv.URL, reportID, "confirm", map[string]any{"latitude": cLat, "longitude": cLon}))
	feedRepC, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, clientC, srv.URL, lat, lon, false)), reportID)
	if !ok {
		t.Fatalf("report %d absent from C's feed after C's own confirm", reportID)
	}
	if feedRepC.Visibility != voteVisC {
		t.Fatalf("after C's confirm: feed visibility %q != vote response visibility %q", feedRepC.Visibility, voteVisC)
	}
	if feedRepC.VisibilityReason != voteReasonC {
		t.Fatalf("after C's confirm: feed visibility_reason %q != vote response reason %q", feedRepC.VisibilityReason, voteReasonC)
	}
	if voteVisC != "live" {
		t.Fatalf("after C's confirm: visibility = %q, want %q", voteVisC, "live")
	}
}

// TestShowDisputedRejectsNonBooleanValue proves the parameter's default is
// closed and its rejection is loud: an unparseable value is a 400 naming
// the field, while an absent parameter and an explicit "false" both fall
// through to the default (Hidden-excluded) result.
func TestShowDisputedRejectsNonBooleanValue(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "feed-show-disputed-reject@example.com")

	const lat, lon = 12.9716, 77.5946
	baseURL := srv.URL + "/api/reports?lat=" + strconv.FormatFloat(lat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(lon, 'f', -1, 64) + "&radius_km=5"

	badResp, err := client.Get(baseURL + "&show_disputed=yes")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if badResp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(badResp.Body)
		badResp.Body.Close()
		t.Fatalf("expected 400, got %d: %s", badResp.StatusCode, b)
	}
	field, _ := decodeErrorResponse(t, badResp)
	if field != "show_disputed" {
		t.Fatalf("error.field = %q, want %q", field, "show_disputed")
	}

	for _, suffix := range []string{"", "&show_disputed=false"} {
		resp, err := client.Get(baseURL + suffix)
		if err != nil {
			t.Fatalf("GET (suffix %q): %v", suffix, err)
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("suffix %q: expected 200, got %d: %s", suffix, resp.StatusCode, b)
		}
		resp.Body.Close()
	}
}

// TestFeedResponseIsNotCached proves the second, lower-priority hole the
// UAT gap-3 diagnosis surfaced (02-UAT.md): the feed endpoint set no
// Cache-Control header at all, so an uncached-but-cacheable JSON response
// was eligible for a browser's own heuristic caching even on a page that
// DID re-run its JavaScript after a Back navigation. Both a 200 and a 400
// from this handler are checked so the header is proven set early — before
// any query-parameter validation runs — rather than only on the success
// path. page.go's Page handler applies the same DEC-Q reasoning to the
// document; this closes the equivalent gap for the JSON that document's own
// JavaScript fetches.
func TestFeedResponseIsNotCached(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "feed-no-store@example.com")

	const lat, lon = 12.9716, 77.5946

	resp := getNearby(t, client, srv.URL, lat, lon, false)
	decodeFeedResponse(t, resp)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("200 response Cache-Control = %q, want %q", cc, "no-store")
	}

	badURL := srv.URL + "/api/reports?lat=" + strconv.FormatFloat(lat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(lon, 'f', -1, 64) + "&radius_km=5&show_disputed=yes"
	badResp, err := client.Get(badURL)
	if err != nil {
		t.Fatalf("GET %s: %v", badURL, err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", badResp.StatusCode)
	}
	if cc := badResp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("400 response Cache-Control = %q, want %q — the header must be set before "+
			"query-parameter validation runs, not only on the success path", cc, "no-store")
	}
}
