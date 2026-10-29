// reporter_path_e2e_test.go proves the viewer dependent reporter_profile_path on
// the wire, over the real router and a real Postgres (plan 09-09, PROF-01,
// PROF-03, D-04, D-12, T-09-33, T-09-34). It reuses the trust e2e helpers. Every
// package level identifier added here carries the rpe prefix.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/testutil"
)

// rpeMakePublic gives the account behind email a clear username and turns its
// profile on, the way an opted in person looks to the server.
func rpeMakePublic(t *testing.T, pool *pgxpool.Pool, email, username string, public bool) {
	t.Helper()
	testutil.MustExec(t, pool,
		`UPDATE accounts
		 SET username = $2, username_screen = 'clear', profile_public = $3,
		     profile_public_at = CASE WHEN $3 THEN now() ELSE NULL END
		 WHERE email = $1`, email, username, public)
}

// rpeFeedRows reads the feed body into a map from report id to the element as a
// raw map, so a missing key and an empty one can be told apart.
func rpeFeedRows(t *testing.T, raw []byte) map[int64]map[string]json.RawMessage {
	t.Helper()
	var out struct {
		Reports []map[string]json.RawMessage `json:"reports"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding feed: %v (body %s)", err, raw)
	}
	rows := make(map[int64]map[string]json.RawMessage, len(out.Reports))
	for _, r := range out.Reports {
		var id int64
		if err := json.Unmarshal(r["id"], &id); err != nil {
			t.Fatalf("decoding report id: %v", err)
		}
		rows[id] = r
	}
	return rows
}

func TestReporterPathOnFeedAndVote(t *testing.T) {
	srv, mailer, pool := newTrustE2EServer(t, p07FixedClock())

	const publicEmail = "rpe-public@example.com"
	const privateEmail = "rpe-private@example.com"
	pub := newVerifiedClient(t, srv, mailer, publicEmail)
	priv := newVerifiedClient(t, srv, mailer, privateEmail)
	viewer := newVerifiedClient(t, srv, mailer, "rpe-viewer@example.com")
	rpeMakePublic(t, pool, publicEmail, "ravi.k", true)
	rpeMakePublic(t, pool, privateEmail, "quiet.one", false)

	pubReport := submitReportGetID(t, pub, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	privReport := submitReportGetID(t, priv, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	rows := rpeFeedRows(t, p07GetFeedRaw(t, viewer, srv.URL, false))
	pubRow, ok := rows[pubReport]
	if !ok {
		t.Fatalf("public reporter's report %d is not in the feed", pubReport)
	}
	var path string
	if err := json.Unmarshal(pubRow["reporter_profile_path"], &path); err != nil || path != "/u/ravi.k" {
		t.Errorf("public reporter's reporter_profile_path = %s, want \"/u/ravi.k\"", pubRow["reporter_profile_path"])
	}
	privRow, ok := rows[privReport]
	if !ok {
		t.Fatalf("private reporter's report %d is not in the feed", privReport)
	}
	if _, present := privRow["reporter_profile_path"]; present {
		t.Errorf("private reporter's row carries reporter_profile_path %s, want the key absent", privRow["reporter_profile_path"])
	}

	// Your own report links nowhere.
	ownRows := rpeFeedRows(t, p07GetFeedRaw(t, pub, srv.URL, false))
	if _, present := ownRows[pubReport]["reporter_profile_path"]; present {
		t.Errorf("the reporter's own row carries reporter_profile_path, want the key absent")
	}

	// The path is never inside the trust object, and the vote response (which has
	// no report row) still carries a trust object equal to the feed's.
	var trustKeys map[string]json.RawMessage
	if err := json.Unmarshal(pubRow["trust"], &trustKeys); err != nil {
		t.Fatalf("decoding trust: %v", err)
	}
	if _, inside := trustKeys["reporter_profile_path"]; inside {
		t.Errorf("reporter_profile_path is inside the trust object")
	}
	status, voteBody, err := p07PostVoteRaw(viewer, srv.URL, pubReport, "confirm", 12.9746, 77.5946)
	if err != nil || status != http.StatusOK {
		t.Fatalf("confirm vote: status %d, err %v, body %s", status, err, voteBody)
	}
	if strings.Contains(string(voteBody), "reporter_profile_path") {
		t.Errorf("the vote response carries reporter_profile_path: %s", voteBody)
	}
	var vote struct {
		Trust json.RawMessage `json:"trust"`
	}
	if err := json.Unmarshal(voteBody, &vote); err != nil {
		t.Fatalf("decoding vote body: %v", err)
	}
	p07CompareWithFeed(t, "after the confirm", vote.Trust, srv.URL, pubReport, map[string]*http.Client{"viewer": viewer, "reporter": pub})
}

func TestReporterPathOmitsIdentifiers(t *testing.T) {
	srv, mailer, pool := newTrustE2EServer(t, p07FixedClock())

	const reporterEmail = "rpe-omit-reporter@example.com"
	const viewerEmail = "rpe-omit-viewer@example.com"
	rep := newVerifiedClient(t, srv, mailer, reporterEmail)
	viewer := newVerifiedClient(t, srv, mailer, viewerEmail)
	rpeMakePublic(t, pool, reporterEmail, "ravi.k", true)
	reportID := submitReportGetID(t, rep, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	raw := p07GetFeedRaw(t, viewer, srv.URL, false)
	rows := rpeFeedRows(t, raw)
	if _, ok := rows[reportID]["reporter_profile_path"]; !ok {
		t.Fatalf("precondition: the feed does not carry reporter_profile_path for report %d: %s", reportID, raw)
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("feed is not JSON: %v", err)
	}
	p07WalkJSONKeys(t, "feed with the profile path", decoded)
	text := string(raw)
	for _, banned := range []string{reporterEmail, viewerEmail, "@", "account_id", "geohash_cell", p07SessionID(t, rep, srv.URL), p07SessionID(t, viewer, srv.URL)} {
		if strings.Contains(text, banned) {
			t.Errorf("feed contains %q: %s", banned, raw)
		}
	}
}
