// moderator_ratings_e2e_test.go proves the moderator ratings tool of Phase 9 plan
// 15 end to end over the real router and a real Postgres (D-14, research G2 and
// G3, V7, T-09-56, T-09-57): a moderator lists one reporter's tapped ratings by
// rater username, removes one, or removes everything one rater gave; the next
// read of the reporter's stars ignores them; and nobody who is not a moderator
// reaches the routes. Every package level identifier added here carries the
// p915 prefix.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const p915ModEmail = "mod@example.com"

// p915Server builds the harness with the moderator list and open people buckets.
func p915Server(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	return newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "Mod@Example.com")
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
	})
}

// p915Rating mirrors one rating of the moderator list.
type p915Rating struct {
	Rater     string `json:"rater"`
	Stars     int    `json:"stars"`
	UpdatedAt string `json:"updated_at"`
	Removed   bool   `json:"removed"`
	Cleared   bool   `json:"cleared"`
}

// p915Seed is one seeded reporter with raters who each have a clear username.
type p915Seed struct {
	reporterID int64
	username   string
	report     int64
	raters     []int64
	names      []string
}

// p915SeedReporter seeds a public reporter, a live report of theirs and one
// rater per entry of stars, each in a different cell, named prefix.r1 and so on.
func p915SeedReporter(t *testing.T, pool *pgxpool.Pool, name string, stars []int) p915Seed {
	t.Helper()
	now := time.Now().UTC()
	id := testutil.SeedPublicReporter(t, pool, "p915-"+name+"@example.com", name, "p915-"+name+"-sess", now)
	report := testutil.SeedHistoryReport(t, pool, "p915-"+name+"-sess", "low", "flood", now.Add(time.Hour))
	raters := testutil.SeedRaters(t, pool, id, report, stars)
	names := make([]string, len(raters))
	for i, rid := range raters {
		names[i] = name + ".r" + string(rune('1'+i))
		testutil.SeedUsername(t, pool, rid, names[i], "clear")
	}
	return p915Seed{reporterID: id, username: name, report: report, raters: raters, names: names}
}

// p915ModeratorClient signs the moderator in.
func p915ModeratorClient(t *testing.T, srv *httptest.Server, mailer *recordingMailer) *http.Client {
	t.Helper()
	return newVerifiedClient(t, srv, mailer, p915ModEmail)
}

// p915List reads the moderator list of a reporter and requires 200.
func p915List(t *testing.T, mod *http.Client, srv *httptest.Server, reporter string) []p915Rating {
	t.Helper()
	status, body, _ := p907Do(t, mod, http.MethodGet, srv.URL+"/api/moderation/people/"+reporter+"/ratings", "")
	if status != http.StatusOK {
		t.Fatalf("list ratings of %s: status %d, body %s", reporter, status, body)
	}
	return p915Decode(t, body)
}

// p915Decode reads the ratings array of a list body and requires it is not null.
func p915Decode(t *testing.T, body string) []p915Rating {
	t.Helper()
	var out struct {
		Ratings *[]p915Rating `json:"ratings"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil || out.Ratings == nil {
		t.Fatalf("not a ratings body: %v: %s", err, body)
	}
	return *out.Ratings
}

// p915ByRater indexes a list by rater username.
func p915ByRater(rows []p915Rating) map[string]p915Rating {
	m := map[string]p915Rating{}
	for _, r := range rows {
		m[r.Rater] = r
	}
	return m
}

// p915Profile reads the public profile fields the ratings move.
func p915Profile(t *testing.T, viewer *http.Client, srv *httptest.Server, username string) (tag, number, fromRatings string) {
	t.Helper()
	_, obj := p907Get(t, viewer, srv, "/api/people/"+username, http.StatusOK)
	tag, _ = obj["tag_word"].(string)
	number, _ = obj["stars_number"].(string)
	fromRatings, _ = obj["stars_from_ratings"].(string)
	return tag, number, fromRatings
}

// TestModeratorRatings is V7 (D-14, G2): list, remove one, remove all by one rater.
func TestModeratorRatings(t *testing.T) {
	srv, mailer, pool := p915Server(t)
	ctx := context.Background()
	mod := p915ModeratorClient(t, srv, mailer)

	one := p915SeedReporter(t, pool, "p915a", []int{5, 4, 3, 2, 1})
	two := p915SeedReporter(t, pool, "p915b", []int{4})

	// The first rater of reporter one also rated reporter two (a vote and a rating).
	shared := one.raters[0]
	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', 'tdr1v8z')`,
		two.report, shared)
	testutil.MustExec(t, pool,
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, 3)`, shared, two.reporterID)

	t.Run("list shows each rater with stars", func(t *testing.T) {
		rows := p915List(t, mod, srv, one.username)
		if len(rows) != 5 {
			t.Fatalf("got %d ratings, want 5: %+v", len(rows), rows)
		}
		by := p915ByRater(rows)
		for i, name := range one.names {
			r, ok := by[name]
			if !ok {
				t.Fatalf("rater %s is not listed: %+v", name, rows)
			}
			if want := []int{5, 4, 3, 2, 1}[i]; r.Stars != want || r.Removed || r.Cleared || r.UpdatedAt == "" {
				t.Errorf("rater %s: %+v, want %d stars, live", name, r, want)
			}
		}
	})

	t.Run("remove one marks it removed and nothing else", func(t *testing.T) {
		body := `{"rater":"` + one.names[4] + `"}`
		status, raw, _ := p907Do(t, mod, http.MethodPost, srv.URL+"/api/moderation/people/"+one.username+"/ratings/remove", body)
		if status != http.StatusOK {
			t.Fatalf("remove one: status %d, body %s", status, raw)
		}
		after := p915ByRater(p915Decode(t, raw))
		if !after[one.names[4]].Removed {
			t.Errorf("the removed rating is not marked Removed: %+v", after[one.names[4]])
		}
		for _, name := range one.names[:4] {
			if after[name].Removed {
				t.Errorf("rater %s was removed too", name)
			}
		}
		// The audit trail is on the row.
		var by *int64
		var at *time.Time
		if err := pool.QueryRow(ctx,
			`SELECT removed_by, removed_at FROM profile_ratings WHERE rater_id = $1 AND reporter_id = $2`,
			one.raters[4], one.reporterID).Scan(&by, &at); err != nil || by == nil || at == nil {
			t.Errorf("removed_by and removed_at are not stamped: %v %v %v", by, at, err)
		}
		// A second removal of the same rating changes nothing and succeeds.
		if status, raw, _ := p907Do(t, mod, http.MethodPost, srv.URL+"/api/moderation/people/"+one.username+"/ratings/remove", body); status != http.StatusOK {
			t.Errorf("remove again: status %d, body %s", status, raw)
		}
	})

	t.Run("remove all by one rater", func(t *testing.T) {
		status, raw, _ := p907Do(t, mod, http.MethodPost, srv.URL+"/api/moderation/raters/"+one.names[0]+"/remove-ratings", "")
		if status != http.StatusOK {
			t.Fatalf("remove all: status %d, body %s", status, raw)
		}
		var out struct {
			Removed *int64 `json:"removed"`
		}
		if err := json.Unmarshal([]byte(raw), &out); err != nil || out.Removed == nil || *out.Removed != 2 {
			t.Fatalf("remove all answer %s (%v), want removed 2", raw, err)
		}
		for _, who := range []p915Seed{one, two} {
			by := p915ByRater(p915List(t, mod, srv, who.username))
			if !by[one.names[0]].Removed {
				t.Errorf("the shared rater's rating of %s is not Removed: %+v", who.username, by[one.names[0]])
			}
		}
		// The rater can no longer rate anyone (W4).
		var marked bool
		if err := pool.QueryRow(ctx, `SELECT rater_removed_at IS NOT NULL FROM accounts WHERE id = $1`, shared).Scan(&marked); err != nil || !marked {
			t.Errorf("the rater is not marked removed: %v %v", marked, err)
		}
		// A second call has nothing left to remove.
		_, raw2, _ := p907Do(t, mod, http.MethodPost, srv.URL+"/api/moderation/raters/"+one.names[0]+"/remove-ratings", "")
		if err := json.Unmarshal([]byte(raw2), &out); err != nil || out.Removed == nil || *out.Removed != 0 {
			t.Errorf("second remove all answer %s (%v), want removed 0", raw2, err)
		}
	})

	t.Run("an empty list is an empty array", func(t *testing.T) {
		testutil.SeedPublicReporter(t, pool, "p915-quiet@example.com", "p915quiet", "p915-quiet-sess", time.Now().UTC())
		status, raw, _ := p907Do(t, mod, http.MethodGet, srv.URL+"/api/moderation/people/p915quiet/ratings", "")
		if status != http.StatusOK || strings.TrimSpace(raw) != `{"ratings":[]}` {
			t.Errorf("no ratings: status %d body %s, want {\"ratings\":[]}", status, raw)
		}
	})

	t.Run("bad names and bodies", func(t *testing.T) {
		for _, c := range []struct {
			method, path, body string
			status             int
			field              string
		}{
			{http.MethodGet, "/api/moderation/people/nobody.here/ratings", "", http.StatusNotFound, "username"},
			{http.MethodGet, "/api/moderation/people/not%20a%20name/ratings", "", http.StatusNotFound, "username"},
			{http.MethodPost, "/api/moderation/people/" + one.username + "/ratings/remove", `{"rater":"nobody.here"}`, http.StatusNotFound, "username"},
			{http.MethodPost, "/api/moderation/people/" + one.username + "/ratings/remove", `{}`, http.StatusBadRequest, "rater"},
			{http.MethodPost, "/api/moderation/people/" + one.username + "/ratings/remove", `{"rater":"x","extra":1}`, http.StatusBadRequest, "rater"},
			{http.MethodPost, "/api/moderation/raters/nobody.here/remove-ratings", "", http.StatusNotFound, "username"},
		} {
			status, raw, _ := p907Do(t, mod, c.method, srv.URL+c.path, c.body)
			if status != c.status {
				t.Errorf("%s %s %q: status %d, want %d: %s", c.method, c.path, c.body, status, c.status, raw)
				continue
			}
			if field, _ := p907ErrorOf(t, raw); field != c.field {
				t.Errorf("%s %s %q: field %q, want %q", c.method, c.path, c.body, field, c.field)
			}
		}
		// A moderator who uses the wrong method gets the group's 405.
		for _, c := range []struct{ method, path string }{
			{http.MethodPost, "/api/moderation/people/" + one.username + "/ratings"},
			{http.MethodGet, "/api/moderation/people/" + one.username + "/ratings/remove"},
			{http.MethodGet, "/api/moderation/raters/" + one.names[1] + "/remove-ratings"},
		} {
			if status, _, _ := p907Do(t, mod, c.method, srv.URL+c.path, ""); status != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status %d, want 405", c.method, c.path, status)
			}
		}
	})
}

// p915Read sends the method to url with the client and returns the status, the
// body bytes and the headers once Date and Set-Cookie are dropped.
func p915Read(t *testing.T, client *http.Client, method, url string) (int, []byte, http.Header) {
	t.Helper()
	resp := modDo(t, client, method, url)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s %s: %v", method, url, err)
	}
	h := resp.Header.Clone()
	h.Del("Date")
	h.Del("Set-Cookie")
	return resp.StatusCode, b, h
}

// TestModeratorRatingsHiddenFromOthers is G3 and T-09-56: a signed in person who
// is not a moderator gets the moderator group's own 404 on all three routes, and
// an anonymous client gets exactly what the report Keep route answers it.
func TestModeratorRatingsHiddenFromOthers(t *testing.T) {
	srv, mailer, pool := p915Server(t)
	seed := p915SeedReporter(t, pool, "p915h", []int{1, 1, 1, 1, 1})

	noFollow := func(c *http.Client) *http.Client { c.CheckRedirect = modNoFollow; return c }
	reader := noFollow(newVerifiedClient(t, srv, mailer, "p915-reader@example.com"))
	rater, _ := p907Person(t, srv, mailer, pool, "p915-rater-client@example.com", "p915.client", false)
	noFollow(rater)
	anonymous := &http.Client{CheckRedirect: modNoFollow}

	paths := []string{
		"/api/moderation/people/" + seed.username + "/ratings",
		"/api/moderation/people/" + seed.username + "/ratings/remove",
		"/api/moderation/raters/" + seed.names[0] + "/remove-ratings",
	}
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete}
	visitors := []struct {
		name   string
		client *http.Client
	}{{"anonymous", anonymous}, {"signed_in_reader", reader}, {"signed_in_person", rater}}

	for _, v := range visitors {
		for _, p := range paths {
			for _, m := range methods {
				t.Run(v.name+"/"+m+p, func(t *testing.T) {
					gotS, gotB, gotH := p915Read(t, v.client, m, srv.URL+p)
					// Same as the report Keep route and the same as a route that does not exist.
					for _, ref := range []string{"/api/moderation/reports/1/keep", "/api/no-such-route"} {
						wantS, wantB, wantH := p915Read(t, v.client, m, srv.URL+ref)
						if gotS != http.StatusNotFound || wantS != http.StatusNotFound {
							t.Errorf("status %d against %s status %d, want both 404", gotS, ref, wantS)
						}
						if !bytes.Equal(gotB, wantB) {
							t.Errorf("body %q differs from %s body %q", gotB, ref, wantB)
						}
						if !reflect.DeepEqual(gotH, wantH) {
							t.Errorf("headers %v differ from %s headers %v", gotH, ref, wantH)
						}
					}
					for _, name := range []string{"Allow", "Location", "Www-Authenticate"} {
						if v := gotH.Values(name); len(v) > 0 {
							t.Errorf("answered a %s header %v", name, v)
						}
					}
				})
			}
		}
	}

	// Nothing was removed by any of the attempts.
	var removed int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM profile_ratings WHERE removed_at IS NOT NULL`).Scan(&removed); err != nil || removed != 0 {
		t.Errorf("ratings removed by non moderators: %d (%v)", removed, err)
	}
	// No other screen carries a rater's name: the public profile has no rater key.
	reporterView, _ := p907Get(t, reader, srv, "/api/people/"+seed.username, http.StatusOK)
	for _, name := range seed.names {
		if strings.Contains(reporterView, name) {
			t.Errorf("the public profile names rater %s: %s", name, reporterView)
		}
	}
}

// TestRatingsIgnoreRemoved is V7: after the moderator removes one of five
// ratings, the reporter's profile shows no ratings part and the tag returns to
// the outcome tag; removing everything one rater gave does the same.
func TestRatingsIgnoreRemoved(t *testing.T) {
	srv, mailer, pool := p915Server(t)
	mod := p915ModeratorClient(t, srv, mailer)
	viewer, _ := p907Person(t, srv, mailer, pool, "p915-viewer@example.com", "p915.viewer", false)

	now := time.Now().UTC()
	id := testutil.SeedPublicReporter(t, pool, "p915-ign@example.com", "p915ign", "p915-ign-sess", now)
	report := testutil.SeedHistoryReport(t, pool, "p915-ign-sess", "low", "flood", now.Add(time.Hour))
	baseTag, baseNumber, baseParts := p915Profile(t, viewer, srv, "p915ign")
	if baseParts != "" {
		t.Fatalf("before any rating the ratings part is %q, want empty", baseParts)
	}

	raters := testutil.SeedRaters(t, pool, id, report, []int{1, 1, 1, 1, 1})
	names := make([]string, len(raters))
	for i, rid := range raters {
		names[i] = "p915ign.r" + string(rune('1'+i))
		testutil.SeedUsername(t, pool, rid, names[i], "clear")
	}
	tag, number, parts := p915Profile(t, viewer, srv, "p915ign")
	if parts == "" || tag == baseTag || number == baseNumber {
		t.Fatalf("five low ratings: tag %q number %q parts %q, want a lower tag and number than %q %q and a ratings part", tag, number, parts, baseTag, baseNumber)
	}

	status, raw, _ := p907Do(t, mod, http.MethodPost, srv.URL+"/api/moderation/people/p915ign/ratings/remove", `{"rater":"`+names[0]+`"}`)
	if status != http.StatusOK {
		t.Fatalf("remove one: status %d, body %s", status, raw)
	}
	tag, number, parts = p915Profile(t, viewer, srv, "p915ign")
	if parts != "" || tag != baseTag || number != baseNumber {
		t.Errorf("after one removal: tag %q number %q parts %q, want %q %q and no ratings part", tag, number, parts, baseTag, baseNumber)
	}

	// The same through Remove all by one rater, on a second reporter.
	id2 := testutil.SeedPublicReporter(t, pool, "p915-ign2@example.com", "p915ign2", "p915-ign2-sess", now)
	report2 := testutil.SeedHistoryReport(t, pool, "p915-ign2-sess", "low", "flood", now.Add(time.Hour))
	raters2 := testutil.SeedRaters(t, pool, id2, report2, []int{1, 1, 1, 1, 1})
	testutil.SeedUsername(t, pool, raters2[0], "p915ign2.r1", "clear")
	if tag, _, parts := p915Profile(t, viewer, srv, "p915ign2"); parts == "" || tag == baseTag {
		t.Fatalf("second reporter, five low ratings: tag %q parts %q, want a lower tag and a ratings part", tag, parts)
	}
	status, raw, _ = p907Do(t, mod, http.MethodPost, srv.URL+"/api/moderation/raters/p915ign2.r1/remove-ratings", "")
	if status != http.StatusOK {
		t.Fatalf("remove all: status %d, body %s", status, raw)
	}
	tag, number, parts = p915Profile(t, viewer, srv, "p915ign2")
	if parts != "" || tag != baseTag || number != baseNumber {
		t.Errorf("after Remove all by one rater: tag %q number %q parts %q, want %q %q and no ratings part", tag, number, parts, baseTag, baseNumber)
	}
}
