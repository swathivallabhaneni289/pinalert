// rated_reporter_e2e_test.go proves that a rated reporter reads the same tag, the
// same score and the same score sentence on every surface at once (Phase 9 plan
// 14, research Pitfall 2, D-02, D-05, D-22, V5, T-09-52): the Nearby feed, the
// Past view, the Focus read, the vote answer, the Activity page, the profile and
// the loader the phone alert dispatcher uses. Every package level identifier
// added here carries the p914 prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// The two voter cells the seeded history votes come from. They are distinct
// precision 7 cells, so each judged report has two independent places.
const (
	p914CellA = "tdr1v8y"
	p914CellB = "tdr1vc3"
)

// p914Reporter is one seeded reporter: its account, its live report and one of
// its finished reports.
type p914Reporter struct {
	id       int64
	username string
	live     int64
	finished int64
}

// p914SeedReporter seeds a reporter with the given count of corroborated and
// contradicted finished reports (all expired an hour ago, so equal weights) and
// one live report. public sets the profile switch. Two voter accounts confirm
// or dispute each finished report from two distinct cells.
func p914SeedReporter(t *testing.T, pool *pgxpool.Pool, name string, corroborated, contradicted int, public bool) p914Reporter {
	t.Helper()
	now := time.Now().UTC()
	session := "p914-session-" + name
	id := testutil.SeedReporterAccount(t, pool, "p914-"+name+"@example.com", session)
	testutil.SeedUsername(t, pool, id, "p914"+name, "clear")
	testutil.MustExec(t, pool,
		`UPDATE accounts SET display_name = $2, profile_public = $3,
		        profile_public_at = CASE WHEN $3 THEN now() ELSE NULL END WHERE id = $1`,
		id, "Reporter "+name, public)
	voterA := testutil.SeedReporterAccount(t, pool, "p914-voter-a-"+name+"@example.com")
	voterB := testutil.SeedReporterAccount(t, pool, "p914-voter-b-"+name+"@example.com")

	expires := now.Add(-time.Hour)
	var finished int64
	for i := 0; i < corroborated+contradicted; i++ {
		value := "confirm"
		if i >= corroborated {
			value = "dispute"
		}
		rid := testutil.SeedHistoryReport(t, pool, session, "low", "flood", expires)
		finished = rid
		for _, v := range []struct {
			account int64
			cell    string
		}{{voterA, p914CellA}, {voterB, p914CellB}} {
			testutil.MustExec(t, pool,
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
				 VALUES ($1, $2, 'content', $3, $4, $5)`,
				rid, v.account, value, v.cell, expires.Add(-2*time.Hour))
		}
	}
	live := testutil.SeedHistoryReport(t, pool, session, "low", "flood", now.Add(time.Hour))
	return p914Reporter{id: id, username: "p914" + name, live: live, finished: finished}
}

// p914Get reads a URL with client and requires 200.
func p914Get(t *testing.T, client *http.Client, url string) []byte {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s", url, resp.StatusCode, raw)
	}
	return raw
}

// p914FeedURL is the feed address at the shared report point, with extra query
// parameters appended.
func p914FeedURL(base, extra string) string {
	return base + "/api/reports?lat=" + strconv.FormatFloat(p07ReportLat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(p07ReportLon, 'f', -1, 64) + "&radius_km=5" + extra
}

// p914ReporterOf decodes the reporter member of a raw trust object.
func p914ReporterOf(t *testing.T, label string, trust json.RawMessage) p14ReporterWire {
	t.Helper()
	if len(trust) == 0 || string(trust) == "null" {
		t.Fatalf("%s: no trust object", label)
	}
	_, rep := p14DecodeSignals(t, trust)
	return rep
}

// p914ListTrust finds one report in a feed body and returns its trust.
func p914ListTrust(t *testing.T, label string, body []byte, reportID int64) json.RawMessage {
	t.Helper()
	var out struct {
		Reports []struct {
			ID    int64           `json:"id"`
			Trust json.RawMessage `json:"trust"`
		} `json:"reports"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("%s: decoding the list: %v: %s", label, err, body)
	}
	for _, r := range out.Reports {
		if r.ID == reportID {
			return r.Trust
		}
	}
	t.Fatalf("%s: report %d is not listed: %s", label, reportID, body)
	return nil
}

var p914ActivityRow = regexp.MustCompile(`data-report-id="(\d+)"[^>]*?data-trust="([^"]*)"`)

// p914ActivityTrust reads the You confirmed part of the Activity page and
// returns the trust of the card of reportID.
func p914ActivityTrust(t *testing.T, client *http.Client, base string, reportID int64) json.RawMessage {
	t.Helper()
	page := string(p914Get(t, client, base+"/profile?part=confirmed"))
	for _, m := range p914ActivityRow.FindAllStringSubmatch(page, -1) {
		if m[1] == strconv.FormatInt(reportID, 10) {
			return json.RawMessage(html.UnescapeString(m[2]))
		}
	}
	t.Fatalf("the Activity page has no card with a trust object for report %d: %s", reportID, page)
	return nil
}

// p914Surfaces reads the reporter signal of one reporter on every surface and
// returns them by name. reader is any signed in account that is not the author
// and has not voted on the live report; rater is an account that confirmed it;
// voter has not voted on the live report and confirms it from a place south of it.
func p914Surfaces(t *testing.T, srv *httptest.Server, pool *pgxpool.Pool, rep p914Reporter, reader, rater, voter *http.Client, includePast bool) map[string]p14ReporterWire {
	t.Helper()
	got := map[string]p14ReporterWire{}

	feed := p914Get(t, reader, p914FeedURL(srv.URL, ""))
	got["feed"] = p914ReporterOf(t, "feed", p914ListTrust(t, "feed", feed, rep.live))

	focus := p914Get(t, reader, p914FeedURL(srv.URL, "&report="+strconv.FormatInt(rep.live, 10)))
	var fo struct {
		Focus struct {
			Status string `json:"status"`
			Report *struct {
				Trust json.RawMessage `json:"trust"`
			} `json:"report"`
		} `json:"focus"`
	}
	if err := json.Unmarshal(focus, &fo); err != nil || fo.Focus.Report == nil {
		t.Fatalf("focus: %v: %s", err, focus)
	}
	got["focus"] = p914ReporterOf(t, "focus", fo.Focus.Report.Trust)

	if includePast {
		past := p914Get(t, reader, p914FeedURL(srv.URL, "&view=past"))
		got["past"] = p914ReporterOf(t, "past", p914ListTrust(t, "past", past, rep.finished))
	}

	_, vt := p07VoteTrust(t, voter, srv.URL, rep.live, "confirm", 12.9686, 77.5946)
	got["vote"] = p914ReporterOf(t, "vote", vt)

	got["activity"] = p914ReporterOf(t, "activity", p914ActivityTrust(t, rater, srv.URL, rep.live))

	// The dispatcher reads the tag through the same loader with the ratings
	// source (reporterIsReliable), so the loader is the dispatcher's surface.
	q := sqlcgen.New(pool)
	sigs, err := service.LoadReporterSignals(context.Background(), q, service.NewRatingService(q), []int64{rep.id}, time.Now().UTC())
	if err != nil {
		t.Fatalf("LoadReporterSignals: %v", err)
	}
	sig := sigs[rep.id]
	w := p14ReporterWire{
		Tag: string(sig.Tag), Score: sig.Score, Label: sig.Label, Sentence: sig.Sentence,
		ScoreSentence: sig.ScoreSentence, JudgedReports: sig.JudgedReports, CorroboratedReports: sig.CorroboratedReports,
	}
	got["dispatcher"] = w
	return got
}

// p914AssertAll requires every surface to carry exactly want, and the profile
// (when the reader may open it) to carry the same words and stars.
func p914AssertAll(t *testing.T, label string, got map[string]p14ReporterWire, want p14ReporterWire) {
	t.Helper()
	for name, w := range got {
		p14AssertReporter(t, label+" "+name, w, want)
	}
}

// p914AssertProfile reads GET /api/people/{username} and requires the tag word
// and the stars number to match the signal on every other surface.
func p914AssertProfile(t *testing.T, label string, reader *http.Client, base, username string, want p14ReporterWire, fromReports, fromRatings string) {
	t.Helper()
	var p struct {
		TagWord          string `json:"tag_word"`
		StarsNumber      string `json:"stars_number"`
		StarsFromReports string `json:"stars_from_reports"`
		StarsFromRatings string `json:"stars_from_ratings"`
	}
	raw := p914Get(t, reader, base+"/api/people/"+username)
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("%s: decoding the profile: %v: %s", label, err, raw)
	}
	if want.Score == nil {
		t.Fatalf("%s: no score to compare", label)
	}
	if p.TagWord != want.Label || p.StarsNumber != service.StarsText(*want.Score) {
		t.Errorf("%s: profile reads %q with %q stars, want %q with %q", label, p.TagWord, p.StarsNumber, want.Label, service.StarsText(*want.Score))
	}
	if p.StarsFromReports != fromReports || p.StarsFromRatings != fromRatings {
		t.Errorf("%s: profile parts read %q and %q, want %q and %q", label, p.StarsFromReports, p.StarsFromRatings, fromReports, fromRatings)
	}
}

// p914RaterClient signs in as the seeded rater account id.
func p914RaterClient(t *testing.T, srv *httptest.Server, mailer *recordingMailer, pool *pgxpool.Pool, id int64) *http.Client {
	t.Helper()
	var email string
	if err := pool.QueryRow(context.Background(), "SELECT email FROM accounts WHERE id = $1", id).Scan(&email); err != nil {
		t.Fatalf("reading the email of rater %d: %v", id, err)
	}
	return newVerifiedClient(t, srv, mailer, email)
}

// TestRatedReporterSameEverywhere is V5 and D-22. A public reporter whose
// outcome score is 60 (Mixed record) is rated five stars by five raters in five
// distinct places: every surface reads 80, Reliable reporter, together, then a
// moderator removes one rating and every surface falls back to 60, Mixed record,
// together. A second reporter with an outcome score of 86 and a private profile
// is rated one star by five raters: every surface reads 53, Mixed record, with
// the profile off, and still reads 53 after it is switched on (D-22).
func TestRatedReporterSameEverywhere(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	ctx := context.Background()

	// Leg one: the public reporter.
	one := p914SeedReporter(t, pool, "one", 5, 3, true)
	raters := testutil.SeedRaters(t, pool, one.id, one.live, []int{5, 5, 5, 5, 5})
	for i, id := range raters {
		testutil.SeedUsername(t, pool, id, fmt.Sprintf("p914rater%d", i), "clear")
	}
	reader := newVerifiedClient(t, srv, mailer, "p914-reader@example.com")
	rater := p914RaterClient(t, srv, mailer, pool, raters[0])

	reliable := p14ReporterWire{
		Tag: "reliable", Score: p14Int(80), Label: "Reliable reporter",
		ScoreSentence: "Score 80 out of 100.", JudgedReports: 8, CorroboratedReports: 5,
	}
	got := p914Surfaces(t, srv, pool, one, reader, rater, newVerifiedClient(t, srv, mailer, "p914-voter-1@example.com"), true)
	reliable.Sentence = got["feed"].Sentence
	p914AssertAll(t, "rated five stars", got, reliable)
	p914AssertProfile(t, "rated five stars", reader, srv.URL, one.username, reliable, "3.0 from reports", "5.0 from ratings by 5 people")

	// A moderator removes one rating: four counted raters is below the five that
	// make ratings count, so every surface falls back to the outcome only signal.
	var moderator int64
	if err := pool.QueryRow(ctx, "SELECT id FROM accounts WHERE email = $1", "p914-reader@example.com").Scan(&moderator); err != nil {
		t.Fatalf("reading the moderator account: %v", err)
	}
	if err := service.NewRatingService(sqlcgen.New(pool)).ModeratorRemove(ctx, moderator, one.username, "p914rater1"); err != nil {
		t.Fatalf("ModeratorRemove: %v", err)
	}
	mixed := reliable
	mixed.Tag, mixed.Score, mixed.Label, mixed.ScoreSentence = "mixed", p14Int(60), "Mixed record", "Score 60 out of 100."
	got = p914Surfaces(t, srv, pool, one, reader, rater, newVerifiedClient(t, srv, mailer, "p914-voter-2@example.com"), true)
	mixed.Sentence = got["feed"].Sentence
	p914AssertAll(t, "after the removal", got, mixed)
	p914AssertProfile(t, "after the removal", reader, srv.URL, one.username, mixed, "", "")

	// Leg two (D-22): a private profile cannot lift the tag, and the ratings
	// still lower it.
	two := p914SeedReporter(t, pool, "two", 5, 0, false)
	ones := testutil.SeedRaters(t, pool, two.id, two.live, []int{1, 1, 1, 1, 1})
	rater2 := p914RaterClient(t, srv, mailer, pool, ones[0])
	low := p14ReporterWire{
		Tag: "mixed", Score: p14Int(53), Label: "Mixed record",
		ScoreSentence: "Score 53 out of 100.", JudgedReports: 5, CorroboratedReports: 5,
	}
	got = p914Surfaces(t, srv, pool, two, reader, rater2, newVerifiedClient(t, srv, mailer, "p914-voter-3@example.com"), false)
	low.Sentence = got["feed"].Sentence
	p914AssertAll(t, "private profile", got, low)

	testutil.MustExec(t, pool, `UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1`, two.id)
	got = p914Surfaces(t, srv, pool, two, reader, rater2, newVerifiedClient(t, srv, mailer, "p914-voter-4@example.com"), false)
	p914AssertAll(t, "profile switched on", got, low)
	p914AssertProfile(t, "profile switched on", reader, srv.URL, two.username, low, "4.3 from reports", "1.0 from ratings by 5 people")
}
