// people_e2e_test.go proves the Phase 9 plan 07 people routes end to end over the
// real router and a real Postgres: the opt in switch (D-01, G1), the one uniform
// 404 (research Pitfall 9, T-09-23), the Followers and Following lists (D-03),
// follow and block (D-12), the profile reports (D-20), the identifier check on
// every people body (T-09-25) and the per account limits (T-09-26). It reuses
// the 7.1 sign in helper (newVerifiedClient) by name and the 09-02 seed helpers.
// Every package level identifier added here carries the p907 prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// p907Server is the e2e server with both people buckets opened wide, so a case
// that does not study a limit never meets one.
func p907Server(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	return newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
	})
}

// p907Do sends one request and returns the status, the body and the headers.
func p907Do(t *testing.T, client *http.Client, method, url, body string) (int, string, http.Header) {
	t.Helper()
	resp := meSend(t, client, method, url, body)
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("reading %s %s: %v", method, url, err)
	}
	return resp.StatusCode, string(raw), resp.Header
}

// p907Get is a GET that must answer want and returns the decoded object.
func p907Get(t *testing.T, client *http.Client, srv *httptest.Server, path string, want int) (string, map[string]any) {
	t.Helper()
	status, body, _ := p907Do(t, client, http.MethodGet, srv.URL+path, "")
	if status != want {
		t.Fatalf("GET %s: status = %d, want %d, body %s", path, status, want, body)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		t.Fatalf("GET %s: body is not a JSON object: %v: %s", path, err, body)
	}
	return body, obj
}

// p907Post is a body free POST that must answer want and returns the body.
func p907Post(t *testing.T, client *http.Client, srv *httptest.Server, path string, want int) string {
	t.Helper()
	status, body, _ := p907Do(t, client, http.MethodPost, srv.URL+path, "")
	if status != want {
		t.Fatalf("POST %s: status = %d, want %d, body %s", path, status, want, body)
	}
	return body
}

// p907ErrorOf decodes the field and message of an error body.
func p907ErrorOf(t *testing.T, body string) (field, message string) {
	t.Helper()
	var e struct {
		Error struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("not an error body: %v: %s", err, body)
	}
	return e.Error.Field, e.Error.Message
}

// p907Person seeds an account with a clear username and signs a client in as it.
func p907Person(t *testing.T, srv *httptest.Server, mailer *recordingMailer, pool *pgxpool.Pool, email, username string, public bool) (*http.Client, int64) {
	t.Helper()
	id := testutil.SeedPerson(t, pool, email, username, public)
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET display_name = $2 WHERE id = $1", id, "Person "+username); err != nil {
		t.Fatalf("naming account %d: %v", id, err)
	}
	return newVerifiedClient(t, srv, mailer, email), id
}

// p907Reporter seeds a public Reliable reporter and signs a client in as it.
func p907Reporter(t *testing.T, srv *httptest.Server, mailer *recordingMailer, pool *pgxpool.Pool, email, username string) (*http.Client, int64) {
	t.Helper()
	id := testutil.SeedPublicReporter(t, pool, email, username, "sess-"+username, time.Now().UTC())
	return newVerifiedClient(t, srv, mailer, email), id
}

// p907ActiveFollows counts the active follows of a followee.
func p907ActiveFollows(t *testing.T, pool *pgxpool.Pool, followeeID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM account_follows WHERE followee_id = $1 AND ended_at IS NULL", followeeID).Scan(&n); err != nil {
		t.Fatalf("counting follows of %d: %v", followeeID, err)
	}
	return n
}

// TestProfileOptIn is V8 (D-01, G1): a Name that cannot be public is refused
// with 422 and the sentence, the switch stays off, an ordinary Name turns it
// on and makes the profile readable, and turning it off ends the followers.
func TestProfileOptIn(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	me, meID := p907Person(t, srv, mailer, pool, "p907-optin@example.com", "optin_one", false)
	other, _ := p907Person(t, srv, mailer, pool, "p907-optin-reader@example.com", "optin_reader", false)
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET display_name = 'NDMA Control Room' WHERE id = $1", meID); err != nil {
		t.Fatalf("setting the name: %v", err)
	}

	// A body without the key, and one with an unknown key, are 400.
	for _, bad := range []string{`{}`, `{"profile_public":true,"extra":1}`, `not json`} {
		status, body, _ := p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", bad)
		if status != http.StatusBadRequest {
			t.Fatalf("PATCH /api/me/visibility %q: status = %d, want 400, body %s", bad, status, body)
		}
	}

	// A refused Name is a 422 with the UI-SPEC sentence, and the switch stays off.
	status, body, _ := p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":true}`)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("a refused Name: status = %d, want 422, body %s", status, body)
	}
	field, message := p907ErrorOf(t, body)
	if field != "profile_public" || message != "Your name can't be shown on a public profile. Change it in About you." {
		t.Fatalf("a refused Name: field %q message %q", field, message)
	}
	var public bool
	if err := pool.QueryRow(context.Background(), "SELECT profile_public FROM accounts WHERE id = $1", meID).Scan(&public); err != nil || public {
		t.Fatalf("the switch must stay off after a refusal: %v, %v", public, err)
	}
	p907Get(t, other, srv, "/api/people/optin_one", http.StatusNotFound)

	// An ordinary Name turns it on.
	meWant(t, "name change", meSend(t, me, http.MethodPatch, srv.URL+"/api/me/profile", `{"display_name":"Asha Rao"}`), http.StatusOK)
	status, body, _ = p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":true}`)
	if status != http.StatusOK || !strings.Contains(body, `"profile_public":true`) {
		t.Fatalf("switching on: status = %d, body %s", status, body)
	}
	_, profile := p907Get(t, other, srv, "/api/people/optin_one", http.StatusOK)
	if profile["username"] != "optin_one" || profile["name"] != "Asha Rao" {
		t.Fatalf("the public profile: %v", profile)
	}

	// Turning it off ends the follower and hides the profile again.
	followerID := testutil.SeedPerson(t, pool, "p907-optin-follower@example.com", "optin_fan", true)
	testutil.SeedFollow(t, pool, followerID, meID, time.Now().UTC().Add(-time.Hour))
	if got := p907ActiveFollows(t, pool, meID); got != 1 {
		t.Fatalf("active follows before switching off = %d, want 1", got)
	}
	status, body, _ = p907Do(t, me, http.MethodPatch, srv.URL+"/api/me/visibility", `{"profile_public":false}`)
	if status != http.StatusOK || !strings.Contains(body, `"profile_public":false`) {
		t.Fatalf("switching off: status = %d, body %s", status, body)
	}
	if got := p907ActiveFollows(t, pool, meID); got != 0 {
		t.Fatalf("active follows after switching off = %d, want 0", got)
	}
	p907Get(t, other, srv, "/api/people/optin_one", http.StatusNotFound)
}

// TestProfileNotFoundIsUniform is V9 (T-09-23): an unknown username, a private
// person, a pending username, a deleted account and a person who blocked the
// caller all answer 404 with byte equal bodies on every route that reads a person.
func TestProfileNotFoundIsUniform(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, viewerID := p907Person(t, srv, mailer, pool, "p907-uni-viewer@example.com", "uni_viewer", false)

	testutil.SeedPerson(t, pool, "p907-uni-private@example.com", "uni_private", false)
	pendingID := testutil.SeedPerson(t, pool, "p907-uni-pending@example.com", "uni_pending", true)
	testutil.SeedUsername(t, pool, pendingID, "uni_pending", "pending")
	deletedID := testutil.SeedPerson(t, pool, "p907-uni-deleted@example.com", "uni_deleted", true)
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET deleted_at = now() WHERE id = $1", deletedID); err != nil {
		t.Fatalf("deleting the account: %v", err)
	}
	blockerID := testutil.SeedPerson(t, pool, "p907-uni-blocker@example.com", "uni_blocker", true)
	testutil.SeedBlock(t, pool, blockerID, viewerID)

	reads := []string{"", "/followers", "/following", "/reports"}
	want := ""
	for _, name := range []string{"uni_nobody", "uni_private", "uni_pending", "uni_deleted", "uni_blocker"} {
		for _, suffix := range reads {
			status, body, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/api/people/"+name+suffix, "")
			if status != http.StatusNotFound {
				t.Fatalf("GET /api/people/%s%s: status = %d, want 404, body %s", name, suffix, status, body)
			}
			if want == "" {
				want = body
			}
			if body != want {
				t.Fatalf("GET /api/people/%s%s: body differs from the uniform 404:\n got %q\nwant %q", name, suffix, body, want)
			}
		}
		status, body, _ := p907Do(t, viewer, http.MethodPost, srv.URL+"/api/people/"+name+"/follow", "")
		if status != http.StatusNotFound || body != want {
			t.Fatalf("POST /api/people/%s/follow: status = %d, body %q, want the uniform 404 %q", name, status, body, want)
		}
	}
	if field, message := p907ErrorOf(t, want); field != "username" || message != service.ProfileNotFoundMessage {
		t.Fatalf("the uniform 404 says field %q message %q", field, message)
	}
}

// TestPeopleLists is V11 (D-03): the named followers, the others count and the
// sentence for it, and the 50 per page keyset paging.
func TestPeopleLists(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, viewerID := p907Person(t, srv, mailer, pool, "p907-list-viewer@example.com", "list_viewer", false)
	now := time.Now().UTC()

	reporter := testutil.SeedPublicReporter(t, pool, "p907-list-rep@example.com", "list_rep", "sess-list-rep", now)
	pub := testutil.SeedPerson(t, pool, "p907-list-pub@example.com", "list_pub", true)
	priv := testutil.SeedPerson(t, pool, "p907-list-priv@example.com", "list_priv", false)
	blocked := testutil.SeedPerson(t, pool, "p907-list-blocked@example.com", "list_blocked", true)
	testutil.SeedBlock(t, pool, viewerID, blocked)
	for i, id := range []int64{pub, priv, blocked} {
		testutil.SeedFollow(t, pool, id, reporter, now.Add(-time.Duration(i+1)*time.Hour))
	}

	_, list := p907Get(t, viewer, srv, "/api/people/list_rep/followers", http.StatusOK)
	entries, _ := list["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["username"] != "list_pub" {
		t.Fatalf("followers entries = %v, want only list_pub", list["entries"])
	}
	if list["others_count"] != float64(2) || list["others_text"] != "and 2 others" {
		t.Fatalf("others = %v %v, want 2 and %q", list["others_count"], list["others_text"], "and 2 others")
	}

	// Paging: 55 public followers come as 50 then 5.
	big := testutil.SeedPublicReporter(t, pool, "p907-list-big@example.com", "list_big", "sess-list-big", now)
	for i := 0; i < 55; i++ {
		id := testutil.SeedPerson(t, pool, fmt.Sprintf("p907-fan-%02d@example.com", i), fmt.Sprintf("fan_%02d", i), true)
		testutil.SeedFollow(t, pool, id, big, now.Add(-time.Duration(i+1)*time.Minute))
	}
	_, first := p907Get(t, viewer, srv, "/api/people/list_big/followers", http.StatusOK)
	firstEntries, _ := first["entries"].([]any)
	next, _ := first["next"].(string)
	if len(firstEntries) != 50 || next == "" {
		t.Fatalf("first page: %d entries, next %q, want 50 and a cursor", len(firstEntries), next)
	}
	_, second := p907Get(t, viewer, srv, "/api/people/list_big/followers?after="+url.QueryEscape(next), http.StatusOK)
	secondEntries, _ := second["entries"].([]any)
	if len(secondEntries) != 5 || second["next"] != "" {
		t.Fatalf("second page: %d entries, next %v, want 5 and no cursor", len(secondEntries), second["next"])
	}
	seen := map[string]bool{}
	for _, page := range [][]any{firstEntries, secondEntries} {
		for _, e := range page {
			seen[e.(map[string]any)["username"].(string)] = true
		}
	}
	if len(seen) != 55 {
		t.Fatalf("the two pages hold %d distinct people, want 55", len(seen))
	}

	// A cursor that does not decode is a 400 on the field after.
	status, body, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/api/people/list_big/followers?after=%25%25", "")
	if status != http.StatusBadRequest {
		t.Fatalf("a bad cursor: status = %d, want 400, body %s", status, body)
	}
	if field, _ := p907ErrorOf(t, body); field != "after" {
		t.Fatalf("a bad cursor: field %q, want after", field)
	}

	// Following reads the same way: the viewer follows nobody.
	_, following := p907Get(t, viewer, srv, "/api/people/list_viewer/following", http.StatusOK)
	if e, _ := following["entries"].([]any); len(e) != 0 {
		t.Fatalf("following entries = %v, want none", following["entries"])
	}
}

// TestFollowRoutes is V12 (D-01, D-12): follow, the refusal sentence, the cap,
// unfollow, remove follower and remove all, and the quiet answers for names that
// match nobody.
func TestFollowRoutes(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, _ := p907Person(t, srv, mailer, pool, "p907-fol-viewer@example.com", "fol_viewer", false)
	repClient, repID := p907Reporter(t, srv, mailer, pool, "p907-fol-rep@example.com", "fol_rep")
	testutil.SeedPerson(t, pool, "p907-fol-plain@example.com", "fol_plain", true)

	// Follow a Reliable reporter: 204, and again 204.
	p907Post(t, viewer, srv, "/api/people/fol_rep/follow", http.StatusNoContent)
	p907Post(t, viewer, srv, "/api/people/fol_rep/follow", http.StatusNoContent)
	_, profile := p907Get(t, viewer, srv, "/api/people/fol_rep", http.StatusOK)
	if profile["following"] != true || profile["followers_count"] != float64(1) || profile["can_follow"] != false {
		t.Fatalf("after following: %v", profile)
	}
	if got := p907ActiveFollows(t, pool, repID); got != 1 {
		t.Fatalf("active follows = %d, want 1", got)
	}

	// A public person without the tag cannot be followed.
	body := p907Post(t, viewer, srv, "/api/people/fol_plain/follow", http.StatusConflict)
	if field, message := p907ErrorOf(t, body); field != "follow" || message != "You can't follow this person right now." {
		t.Fatalf("a refused follow: field %q message %q", field, message)
	}

	// Unfollow: 204 and the follow is gone.
	p907Post(t, viewer, srv, "/api/people/fol_rep/unfollow", http.StatusNoContent)
	if got := p907ActiveFollows(t, pool, repID); got != 0 {
		t.Fatalf("active follows after unfollow = %d, want 0", got)
	}

	// Remove follower and remove all, as the reporter.
	_, f1ID := p907Person(t, srv, mailer, pool, "p907-fol-f1@example.com", "fol_f1", false)
	_, f2ID := p907Person(t, srv, mailer, pool, "p907-fol-f2@example.com", "fol_f2", false)
	now := time.Now().UTC()
	testutil.SeedFollow(t, pool, f1ID, repID, now.Add(-time.Hour))
	testutil.SeedFollow(t, pool, f2ID, repID, now.Add(-2*time.Hour))
	p907Post(t, repClient, srv, "/api/people/fol_f1/remove-follower", http.StatusNoContent)
	if got := p907ActiveFollows(t, pool, repID); got != 1 {
		t.Fatalf("active follows after remove follower = %d, want 1", got)
	}
	p907Post(t, repClient, srv, "/api/me/followers/remove-all", http.StatusNoContent)
	_, own := p907Get(t, repClient, srv, "/api/people/fol_rep", http.StatusOK)
	if own["followers_count"] != float64(0) || own["is_self"] != true {
		t.Fatalf("own page after remove all: %v", own)
	}

	// A well formed username that matches nobody changes nothing and answers 204.
	for _, path := range []string{"/unfollow", "/unblock", "/remove-follower"} {
		p907Post(t, viewer, srv, "/api/people/fol_nobody_at_all"+path, http.StatusNoContent)
	}
	// A malformed one answers the uniform 404.
	_, uniform, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/api/people/fol_nobody_at_all", "")
	for _, path := range []string{"/unfollow", "/unblock", "/remove-follower"} {
		got := p907Post(t, viewer, srv, "/api/people/bad%21name"+path, http.StatusNotFound)
		if got != uniform {
			t.Fatalf("malformed username on %s: body %q, want the uniform %q", path, got, uniform)
		}
	}

	// The cap: a person who follows 200 people cannot follow a 201st.
	capClient, capID := p907Person(t, srv, mailer, pool, "p907-fol-cap@example.com", "fol_cap", false)
	for i := 0; i < service.FollowingCap; i++ {
		id := testutil.SeedPerson(t, pool, fmt.Sprintf("p907-fol-filler-%03d@example.com", i), fmt.Sprintf("fol_fill_%03d", i), false)
		testutil.SeedFollow(t, pool, capID, id, now.Add(-time.Duration(i+1)*time.Minute))
	}
	body = p907Post(t, capClient, srv, "/api/people/fol_rep/follow", http.StatusConflict)
	if field, message := p907ErrorOf(t, body); field != "follow" || message != "You can follow up to 200 people." {
		t.Fatalf("the cap: field %q message %q", field, message)
	}
}

// TestBlockRoutes is V12 (D-12): a block ends follows both ways, the blocked
// person reads 404 on the blocker, the blocker sees blocked_by_you, the Blocked
// list names the person and unblock restores the normal profile.
func TestBlockRoutes(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	blocker, blockerID := p907Reporter(t, srv, mailer, pool, "p907-blk-a@example.com", "blk_a")
	blocked, blockedID := p907Person(t, srv, mailer, pool, "p907-blk-b@example.com", "blk_b", true)
	now := time.Now().UTC()
	testutil.SeedFollow(t, pool, blockedID, blockerID, now.Add(-time.Hour))
	testutil.SeedFollow(t, pool, blockerID, blockedID, now.Add(-2*time.Hour))

	p907Post(t, blocker, srv, "/api/people/blk_b/block", http.StatusNoContent)
	var live int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM account_follows WHERE ended_at IS NULL AND follower_id IN ($1, $2) AND followee_id IN ($1, $2)",
		blockerID, blockedID).Scan(&live); err != nil || live != 0 {
		t.Fatalf("follows left after a block: %d, %v", live, err)
	}

	// The blocked person gets the uniform 404 on the blocker.
	_, uniform, _ := p907Do(t, blocked, http.MethodGet, srv.URL+"/api/people/blk_nobody_here", "")
	status, body, _ := p907Do(t, blocked, http.MethodGet, srv.URL+"/api/people/blk_a", "")
	if status != http.StatusNotFound || body != uniform {
		t.Fatalf("blocked person reads the blocker: status %d body %q, want the uniform 404", status, body)
	}

	// The blocker sees the Blocked by you face and the Blocked list.
	_, face := p907Get(t, blocker, srv, "/api/people/blk_b", http.StatusOK)
	if face["blocked_by_you"] != true || face["username"] != "blk_b" {
		t.Fatalf("the blocked face: %v", face)
	}
	_, list := p907Get(t, blocker, srv, "/api/me/blocked", http.StatusOK)
	entries, _ := list["entries"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["username"] != "blk_b" {
		t.Fatalf("blocked list = %v", list)
	}
	// The blocker's reports and lists for the blocked person are the uniform 404.
	for _, suffix := range []string{"/reports", "/followers", "/following"} {
		status, body, _ := p907Do(t, blocker, http.MethodGet, srv.URL+"/api/people/blk_b"+suffix, "")
		if status != http.StatusNotFound || body != uniform {
			t.Fatalf("blocker reads blk_b%s: status %d body %q", suffix, status, body)
		}
	}

	// Unblock restores the normal profile both ways.
	p907Post(t, blocker, srv, "/api/people/blk_b/unblock", http.StatusNoContent)
	_, normal := p907Get(t, blocker, srv, "/api/people/blk_b", http.StatusOK)
	if normal["blocked_by_you"] != false || normal["name"] != "Person blk_b" {
		t.Fatalf("the profile after unblock: %v", normal)
	}
	p907Get(t, blocked, srv, "/api/people/blk_a", http.StatusOK)
	_, empty := p907Get(t, blocker, srv, "/api/me/blocked", http.StatusOK)
	if e, _ := empty["entries"].([]any); len(e) != 0 {
		t.Fatalf("blocked list after unblock = %v", empty)
	}

	// Blocking yourself or a private person is the uniform 404.
	p907Post(t, blocker, srv, "/api/people/blk_a/block", http.StatusNotFound)
}

var p907DateOnly = regexp.MustCompile(`^[0-9]{1,2} [A-Z][a-z]+ [0-9]{4}$`)

// TestProfileReportsRoute is V10 (D-20): a live row carries its id and
// description, a finished row carries neither and only a date, no row carries a
// coordinate, a distance appears with lat and lon, and half a pair is a 400.
func TestProfileReportsRoute(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, _ := p907Person(t, srv, mailer, pool, "p907-rep-viewer@example.com", "rep_viewer", false)
	now := time.Now().UTC()
	testutil.SeedPublicReporter(t, pool, "p907-rep-rep@example.com", "rep_rep", "sess-rep-rep", now)
	liveID := testutil.SeedHistoryReport(t, pool, "sess-rep-rep", "medium", "flood", now.Add(2*time.Hour))

	_, page := p907Get(t, viewer, srv, "/api/people/rep_rep/reports", http.StatusOK)
	rows, _ := page["rows"].([]any)
	var live, finished int
	for _, r := range rows {
		row := r.(map[string]any)
		for _, banned := range []string{"latitude", "longitude", "lat", "lon", "geohash"} {
			if _, has := row[banned]; has {
				t.Fatalf("a row carries %q: %v", banned, row)
			}
		}
		if row["live"] == true {
			live++
			if row["id"] != float64(liveID) || row["description"] != "seeded history report" || row["posted_at"] == nil {
				t.Fatalf("the live row: %v", row)
			}
			continue
		}
		finished++
		for _, banned := range []string{"id", "description", "posted_at"} {
			if _, has := row[banned]; has {
				t.Fatalf("a finished row carries %q: %v", banned, row)
			}
		}
		ended, _ := row["ended_on"].(string)
		if !p907DateOnly.MatchString(ended) {
			t.Fatalf("ended_on = %q, want a date with no time", ended)
		}
		if row["finished_title"] == "" || row["finished_title"] == nil {
			t.Fatalf("a finished row has no title: %v", row)
		}
	}
	if live != 1 || finished == 0 {
		t.Fatalf("rows: %d live, %d finished, want 1 live and some finished", live, finished)
	}

	// With lat and lon a live row says how far away it is.
	_, near := p907Get(t, viewer, srv, "/api/people/rep_rep/reports?lat=12.9716&lon=77.5946", http.StatusOK)
	for _, r := range near["rows"].([]any) {
		row := r.(map[string]any)
		if row["live"] == true && (row["distance_text"] == nil || row["distance_text"] == "") {
			t.Fatalf("a live row has no distance with lat and lon: %v", row)
		}
	}

	// Half a pair, and an out of range value, are 400.
	for _, q := range []string{"?lat=12.9", "?lon=77.5", "?lat=95&lon=77", "?lat=abc&lon=77"} {
		status, body, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/api/people/rep_rep/reports"+q, "")
		if status != http.StatusBadRequest {
			t.Fatalf("GET reports%s: status = %d, want 400, body %s", q, status, body)
		}
	}
}

var p907BannedKey = regexp.MustCompile(`(?i)cell|account|session|email|voter|geohash|latitude|longitude`)

// p907WalkKeys fails on a key that names an identifier and on a value that
// carries an @, at any depth.
func p907WalkKeys(t *testing.T, label string, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if p907BannedKey.MatchString(k) {
				t.Errorf("%s: key %q names an identifier", label, k)
			}
			p907WalkKeys(t, label, child)
		}
	case []any:
		for _, child := range x {
			p907WalkKeys(t, label, child)
		}
	case string:
		if strings.Contains(x, "@") {
			t.Errorf("%s: a value contains @: %q", label, x)
		}
	}
}

// TestPeopleJSONOmitsIdentifiers is T-09-25: no people body carries an account
// id, an email, a session, a cell, a voter or a coordinate.
func TestPeopleJSONOmitsIdentifiers(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, viewerID := p907Person(t, srv, mailer, pool, "p907-id-viewer@example.com", "id_viewer", false)
	now := time.Now().UTC()
	repID := testutil.SeedPublicReporter(t, pool, "p907-id-rep@example.com", "id_rep", "sess-id-rep", now)
	testutil.SeedHistoryReport(t, pool, "sess-id-rep", "critical", "flood", now.Add(2*time.Hour))
	fan := testutil.SeedPerson(t, pool, "p907-id-fan@example.com", "id_fan", true)
	testutil.SeedFollow(t, pool, fan, repID, now.Add(-time.Hour))
	other := testutil.SeedPerson(t, pool, "p907-id-other@example.com", "id_other", true)
	testutil.SeedBlock(t, pool, viewerID, other)

	for _, path := range []string{
		"/api/people/id_rep",
		"/api/people/id_rep/followers",
		"/api/people/id_rep/following",
		"/api/people/id_rep/reports?lat=12.9716&lon=77.5946",
		"/api/people/id_other",
		"/api/me/blocked",
		"/api/people/id_viewer",
	} {
		body, _ := p907Get(t, viewer, srv, path, http.StatusOK)
		var decoded any
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatalf("%s: not JSON: %v", path, err)
		}
		p907WalkKeys(t, path, decoded)
		if strings.Contains(body, "id_rep@") || strings.Contains(body, "sess-id-rep") {
			t.Errorf("%s: carries an email or a session id: %s", path, body)
		}
	}
	// Even the 404 carries nothing but the field and the sentence.
	_, notFound, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/api/people/id_nobody", "")
	var decoded any
	if err := json.Unmarshal([]byte(notFound), &decoded); err != nil {
		t.Fatalf("the 404 is not JSON: %v", err)
	}
	p907WalkKeys(t, "404", decoded)
}

// TestPeopleRateLimits is T-09-26: the people buckets are per account, a refused
// request answers 429 with Retry-After and the people field, and another
// account on the same address is not touched.
func TestPeopleRateLimits(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleAction = api.BucketLimit{Every: time.Hour, Burst: 2}
		d.RateLimits.PeopleRead = api.BucketLimit{Every: time.Hour, Burst: 2}
	})
	a, _ := p907Person(t, srv, mailer, pool, "p907-rl-a@example.com", "rl_a", false)
	b, _ := p907Person(t, srv, mailer, pool, "p907-rl-b@example.com", "rl_b", false)
	testutil.SeedPublicReporter(t, pool, "p907-rl-rep@example.com", "rl_rep", "sess-rl-rep", time.Now().UTC())

	p907Post(t, a, srv, "/api/people/rl_rep/follow", http.StatusNoContent)
	p907Post(t, a, srv, "/api/people/rl_rep/follow", http.StatusNoContent)
	status, body, header := p907Do(t, a, http.MethodPost, srv.URL+"/api/people/rl_rep/follow", "")
	if status != http.StatusTooManyRequests {
		t.Fatalf("the third follow: status = %d, want 429, body %s", status, body)
	}
	if field, _ := p907ErrorOf(t, body); field != "rate_people_action" {
		t.Fatalf("the 429 field = %q, want rate_people_action", field)
	}
	if header.Get("Retry-After") == "" {
		t.Fatalf("the 429 has no Retry-After header")
	}

	// Another account is not touched, whatever the address.
	p907Post(t, b, srv, "/api/people/rl_rep/follow", http.StatusNoContent)

	// The read bucket is separate and has its own field.
	p907Get(t, a, srv, "/api/people/rl_rep", http.StatusOK)
	p907Get(t, a, srv, "/api/people/rl_rep", http.StatusOK)
	status, body, _ = p907Do(t, a, http.MethodGet, srv.URL+"/api/people/rl_rep", "")
	if status != http.StatusTooManyRequests {
		t.Fatalf("the third read: status = %d, want 429, body %s", status, body)
	}
	if field, _ := p907ErrorOf(t, body); field != "rate_people_read" {
		t.Fatalf("the 429 field = %q, want rate_people_read", field)
	}
}
