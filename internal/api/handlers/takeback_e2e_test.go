// takeback_e2e_test.go drives the take back of a mistaken Mark resolved over
// HTTP (HIST-09, D-22): POST /api/reports/{id}/unresolve, its two refusals,
// the limit of two undos per report that the poster's reopen shares, and the
// undos_left number every resolution vote answers with. The router and the
// verified clients are the same real-router-plus-real-Postgres harness the
// vote tests use.
package handlers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"pinalert/internal/testutil"
)

const (
	noResolveMessage = "You have no resolved mark on this report to take back."
	undoLimitMessage = "You have used both undos on this report."
)

// Two voter positions inside 1 km of the report at 12.9716, 77.5946, in two
// different precision 7 cells (the existing independent vote tests use the
// same pair).
const (
	tbReportLat, tbReportLon = 12.9716, 77.5946
	tbBLat, tbBLon           = 12.9746, 77.5946
	tbCLat, tbCLon           = 12.9686, 77.5946
)

// decodedVote is a vote response as the browser sees it. UndosLeft is nil when
// the key is absent, so a content vote and a resolution vote can be told apart.
type decodedVote struct {
	Visibility string
	Reason     string
	UndosLeft  *int
	HasUndos   bool
}

// decodeVote decodes a 200 vote response and keeps whether undos_left was
// present at all.
func decodeVote(t *testing.T, resp *http.Response) decodedVote {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading vote response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("decoding vote response: %v (%s)", err, b)
	}
	var out decodedVote
	if err := json.Unmarshal(raw["visibility"], &out.Visibility); err != nil {
		t.Fatalf("decoding visibility: %v (%s)", err, b)
	}
	if err := json.Unmarshal(raw["reason"], &out.Reason); err != nil {
		t.Fatalf("decoding reason: %v (%s)", err, b)
	}
	if v, ok := raw["undos_left"]; ok {
		out.HasUndos = true
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			t.Fatalf("undos_left is not an integer: %v (%s)", err, b)
		}
		out.UndosLeft = &n
	}
	return out
}

// expectRefusal asserts a 409 with the given field and sentence.
func expectRefusal(t *testing.T, resp *http.Response, field, message string) {
	t.Helper()
	if resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, b)
	}
	gotField, gotMessage := decodeErrorResponse(t, resp)
	if gotField != field || gotMessage != message {
		t.Fatalf("error = (%q, %q), want (%q, %q)", gotField, gotMessage, field, message)
	}
}

func atPlace(lat, lon float64) map[string]any {
	return map[string]any{"latitude": lat, "longitude": lon}
}

// listedReportIDs reads the default feed around the report and returns the ids.
func listedReportIDs(t *testing.T, client *http.Client, baseURL string) map[int64]bool {
	t.Helper()
	url := fmt.Sprintf("%s/api/reports?lat=%v&lon=%v&radius_km=5", baseURL, tbReportLat, tbReportLon)
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET /api/reports: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 from GET /api/reports, got %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Reports []struct {
			ID int64 `json:"id"`
		} `json:"reports"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding reports: %v", err)
	}
	ids := make(map[int64]bool, len(out.Reports))
	for _, r := range out.Reports {
		ids[r.ID] = true
	}
	return ids
}

// TestTakeBackEndToEnd: two independent resolves retract a report, one of the
// two voters takes theirs back, and the report is listed again.
func TestTakeBackEndToEnd(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "takeback-e2e-poster@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "takeback-e2e-b@example.com")
	clientC := newVerifiedClient(t, srv, mailer, "takeback-e2e-c@example.com")
	assertDistinctCells(t, tbBLat, tbBLon, tbCLat, tbCLon)

	reportID := submitReportGetID(t, poster, srv.URL, floodReportBody(tbReportLat, tbReportLon))

	decodeVote(t, postVote(t, clientB, srv.URL, reportID, "resolve", atPlace(tbBLat, tbBLon)))
	got := decodeVote(t, postVote(t, clientC, srv.URL, reportID, "resolve", atPlace(tbCLat, tbCLon)))
	if got.Visibility != "retracted" {
		t.Fatalf("after two independent resolves: visibility = %q, want retracted", got.Visibility)
	}
	if listedReportIDs(t, poster, srv.URL)[reportID] {
		t.Fatalf("a retracted report must not be listed in the default feed")
	}

	got = decodeVote(t, postVote(t, clientC, srv.URL, reportID, "unresolve", map[string]any{}))
	if got.Visibility == "retracted" {
		t.Fatalf("after the take back: visibility = %q, want it no longer retracted", got.Visibility)
	}
	if got.UndosLeft == nil || *got.UndosLeft != 1 {
		t.Fatalf("undos_left = %v, want 1", got.UndosLeft)
	}
	if !listedReportIDs(t, poster, srv.URL)[reportID] {
		t.Fatalf("the report must be listed again after the take back")
	}
}

// TestTakeBackLimitAndPosterReopen: the third take back of a non poster is
// refused, the poster's reopen shares the same limit, and a non poster's reopen
// is never limited (D-22, D-28).
func TestTakeBackLimitAndPosterReopen(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "takeback-limit-poster@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "takeback-limit-b@example.com")

	reportID := submitReportGetID(t, poster, srv.URL, floodReportBody(tbReportLat, tbReportLon))

	// A non poster: resolve, take back, twice. The third take back is refused.
	for round, wantLeft := range []int{1, 0} {
		decodeVote(t, postVote(t, clientB, srv.URL, reportID, "resolve", atPlace(tbBLat, tbBLon)))
		got := decodeVote(t, postVote(t, clientB, srv.URL, reportID, "unresolve", map[string]any{}))
		if got.UndosLeft == nil || *got.UndosLeft != wantLeft {
			t.Fatalf("take back %d: undos_left = %v, want %d", round+1, got.UndosLeft, wantLeft)
		}
	}
	decodeVote(t, postVote(t, clientB, srv.URL, reportID, "resolve", atPlace(tbBLat, tbBLon)))
	before := countVotesForReport(t, pool, reportID)
	expectRefusal(t, postVote(t, clientB, srv.URL, reportID, "unresolve", map[string]any{}), "report", undoLimitMessage)
	if after := countVotesForReport(t, pool, reportID); after != before {
		t.Fatalf("a refused take back wrote a row: %d -> %d", before, after)
	}

	// The poster: resolve, take back (1 used), resolve, reopen (2 used), then
	// the next reopen is refused with the same sentence.
	decodeVote(t, postVote(t, poster, srv.URL, reportID, "resolve", map[string]any{}))
	got := decodeVote(t, postVote(t, poster, srv.URL, reportID, "unresolve", map[string]any{}))
	if got.UndosLeft == nil || *got.UndosLeft != 1 {
		t.Fatalf("poster take back: undos_left = %v, want 1", got.UndosLeft)
	}
	decodeVote(t, postVote(t, poster, srv.URL, reportID, "resolve", map[string]any{}))
	got = decodeVote(t, postVote(t, poster, srv.URL, reportID, "reopen", map[string]any{}))
	if got.UndosLeft == nil || *got.UndosLeft != 0 {
		t.Fatalf("poster reopen: undos_left = %v, want 0", got.UndosLeft)
	}
	expectRefusal(t, postVote(t, poster, srv.URL, reportID, "reopen", map[string]any{}), "report", undoLimitMessage)

	// A non poster's reopen is an ordinary claim and is never limited, even
	// for an account that has spent both of its own take backs: clientB used
	// both above and may still reopen as often as it likes.
	for i := 0; i < 3; i++ {
		decodeVote(t, postVote(t, clientB, srv.URL, reportID, "reopen", atPlace(tbBLat, tbBLon)))
	}
}

// TestTakeBackRefusals: nothing to take back, an expired report and an
// unverified session.
func TestTakeBackRefusals(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "takeback-refusal-poster@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "takeback-refusal-b@example.com")

	reportID := submitReportGetID(t, poster, srv.URL, floodReportBody(tbReportLat, tbReportLon))

	// No standing resolve: refused, and nothing is written.
	before := countVotesForReport(t, pool, reportID)
	expectRefusal(t, postVote(t, clientB, srv.URL, reportID, "unresolve", map[string]any{}), "report", noResolveMessage)
	if after := countVotesForReport(t, pool, reportID); after != before {
		t.Fatalf("a refused take back wrote a row: %d -> %d", before, after)
	}

	// A standing confirm is not a resolve either.
	decodeVote(t, postVote(t, clientB, srv.URL, reportID, "confirm", atPlace(tbBLat, tbBLon)))
	expectRefusal(t, postVote(t, clientB, srv.URL, reportID, "unresolve", map[string]any{}), "report", noResolveMessage)

	// An expired report answers the existing 409.
	expired := testutil.SeedExpiringReport(t, pool, -3600)
	expectRefusal(t, postVote(t, clientB, srv.URL, expired, "unresolve", map[string]any{}), "report", "This report has expired.")

	// An unverified session is refused by the gate.
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	anon := &http.Client{Jar: jar}
	resp := postVote(t, anon, srv.URL, reportID, "unresolve", map[string]any{})
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 401, got %d: %s", resp.StatusCode, b)
	}
	resp.Body.Close()
}

// TestTakeBackUndosLeftOnResolve: a resolve response says how many undos the
// caller has left, a content vote carries no such key.
func TestTakeBackUndosLeftOnResolve(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "takeback-left-poster@example.com")
	clientB := newVerifiedClient(t, srv, mailer, "takeback-left-b@example.com")

	reportID := submitReportGetID(t, poster, srv.URL, floodReportBody(tbReportLat, tbReportLon))

	got := decodeVote(t, postVote(t, clientB, srv.URL, reportID, "resolve", atPlace(tbBLat, tbBLon)))
	if got.UndosLeft == nil || *got.UndosLeft != 2 {
		t.Fatalf("resolve: undos_left = %v, want 2", got.UndosLeft)
	}

	got = decodeVote(t, postVote(t, clientB, srv.URL, reportID, "confirm", atPlace(tbBLat, tbBLon)))
	if got.HasUndos {
		t.Fatalf("confirm response must not carry undos_left")
	}
}
