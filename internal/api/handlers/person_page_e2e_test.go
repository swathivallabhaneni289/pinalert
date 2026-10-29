// person_page_e2e_test.go proves the Phase 9 plan 08 profile page end to end over
// the real router and a real Postgres: the header and its action row for your
// own page and for anyone else's (D-04, D-13), the one uniform not found
// (research Pitfall 9, T-09-30), /me (D-04) and the You blocked state (T-09-32).
// It reuses the 09-07 helpers by name. Every package level identifier added here
// carries the p908 prefix.
package handlers_test

import (
	"context"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// p908NoRedirect is a copy of a signed in client that reports a redirect
// instead of following it.
func p908NoRedirect(client *http.Client) *http.Client {
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// p908Tag returns the opening tag of the element with this id.
func p908Tag(body, id string) (string, bool) {
	re := regexp.MustCompile(`<[a-z0-9]+ [^>]*id="` + regexp.QuoteMeta(id) + `"[^>]*>`)
	tag := re.FindString(body)
	return tag, tag != ""
}

// p908Shown is true when the element with this id is on the page and has no
// hidden attribute.
func p908Shown(body, id string) bool {
	tag, ok := p908Tag(body, id)
	return ok && !strings.Contains(tag, " hidden")
}

// TestPersonPage is V9 for the page: another person's page shows the h1
// username, the tag word and Follow; your own page shows Edit in Settings and
// the state line and no Follow; both are no-store and noindex.
func TestPersonPage(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, _ := p907Person(t, srv, mailer, pool, "p908-page-viewer@example.com", "page_viewer", false)
	owner, _ := p907Reporter(t, srv, mailer, pool, "p908-page-owner@example.com", "page_owner")

	status, body, header := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/page_owner", "")
	if status != http.StatusOK {
		t.Fatalf("GET /u/page_owner: status = %d, want 200, body %s", status, body)
	}
	for _, want := range []string{
		`<h1 id="person-username" class="person-username">page_owner</h1>`,
		`id="person-tag"`, "Reliable reporter",
		`<meta name="robots" content="noindex">`,
		"<title>Pinalert: page_owner</title>",
		`data-username="page_owner"`, `data-self="false"`,
		"Member since", "Followers", "Following",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("another person's page lacks %q", want)
		}
	}
	if !p908Shown(body, "person-follow") {
		t.Errorf("a Reliable reporter you do not follow must show Follow")
	}
	if p908Shown(body, "person-unfollow") {
		t.Errorf("Following must be hidden before you follow")
	}
	if !p908Shown(body, "person-more") {
		t.Errorf("More must be on another person's page")
	}
	if strings.Contains(body, `id="person-edit"`) || strings.Contains(body, `id="person-state-line"`) {
		t.Errorf("another person's page must not draw the own page controls")
	}
	if got := header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := header.Get("X-Robots-Tag"); got != "noindex" {
		t.Errorf("X-Robots-Tag = %q, want noindex", got)
	}

	// After following, the same page shows Following and not Follow.
	p907Post(t, viewer, srv, "/api/people/page_owner/follow", http.StatusNoContent)
	_, followed, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/page_owner", "")
	if p908Shown(followed, "person-follow") || !p908Shown(followed, "person-unfollow") {
		t.Errorf("after following, the page must show Following and hide Follow")
	}

	// Your own page.
	status, own, ownHeader := p907Do(t, owner, http.MethodGet, srv.URL+"/u/page_owner", "")
	if status != http.StatusOK {
		t.Fatalf("GET own /u/page_owner: status = %d, want 200, body %s", status, own)
	}
	for _, want := range []string{
		"Edit in Settings", `id="person-state-line"`, `data-self="true"`,
		"People can follow you while you hold the Reliable reporter tag.",
	} {
		if !strings.Contains(own, want) {
			t.Errorf("your own page lacks %q", want)
		}
	}
	for _, banned := range []string{`id="person-follow"`, `id="person-more"`} {
		if strings.Contains(own, banned) {
			t.Errorf("your own page must not draw %s", banned)
		}
	}
	if ownHeader.Get("Cache-Control") != "no-store" || ownHeader.Get("X-Robots-Tag") != "noindex" {
		t.Errorf("your own page headers = %v, want no-store and noindex", ownHeader)
	}
}

// TestPersonPageNotFoundIsUniform is T-09-30: an unknown, private, pending,
// deleted and blocking profile each answer 404 with byte equal bodies, and the
// body carries nothing from the request.
func TestPersonPageNotFoundIsUniform(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, viewerID := p907Person(t, srv, mailer, pool, "p908-uni-viewer@example.com", "p8_viewer", false)

	testutil.SeedPerson(t, pool, "p908-uni-private@example.com", "p8_private", false)
	pendingID := testutil.SeedPerson(t, pool, "p908-uni-pending@example.com", "p8_pending", true)
	testutil.SeedUsername(t, pool, pendingID, "p8_pending", "pending")
	deletedID := testutil.SeedPerson(t, pool, "p908-uni-deleted@example.com", "p8_deleted", true)
	if _, err := pool.Exec(context.Background(), "UPDATE accounts SET deleted_at = now() WHERE id = $1", deletedID); err != nil {
		t.Fatalf("deleting the account: %v", err)
	}
	blockerID := testutil.SeedPerson(t, pool, "p908-uni-blocker@example.com", "p8_blocker", true)
	testutil.SeedBlock(t, pool, blockerID, viewerID)

	want := ""
	for _, name := range []string{"p8_nobody", "p8_private", "p8_pending", "p8_deleted", "p8_blocker", "no%20such"} {
		status, body, header := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/"+name, "")
		if status != http.StatusNotFound {
			t.Fatalf("GET /u/%s: status = %d, want 404", name, status)
		}
		if want == "" {
			want = body
		}
		if body != want {
			t.Fatalf("GET /u/%s: the body differs from the uniform 404", name)
		}
		if strings.Contains(body, name) {
			t.Errorf("the not found body echoes %q", name)
		}
		if header.Get("Cache-Control") != "no-store" || header.Get("X-Robots-Tag") != "noindex" {
			t.Errorf("GET /u/%s headers = %v, want no-store and noindex", name, header)
		}
	}
	for _, text := range []string{
		"Profile not found", "There is no public profile with this username.", "Go to Nearby",
		"<title>Pinalert: Profile not found</title>",
	} {
		if !strings.Contains(want, text) {
			t.Errorf("the not found page lacks %q", text)
		}
	}
	if strings.Contains(want, "data-username") {
		t.Errorf("the not found page must not carry data-username")
	}
}

// TestMyProfileRedirect is D-04: /me answers 303 to /u/{your username}.
func TestMyProfileRedirect(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	me, _ := p907Person(t, srv, mailer, pool, "p908-me@example.com", "me_user", false)

	resp := meSend(t, p908NoRedirect(me), http.MethodGet, srv.URL+"/me", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET /me: status = %d, want 303", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/u/me_user" {
		t.Errorf("GET /me: Location = %q, want /u/me_user", got)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Robots-Tag") != "noindex" {
		t.Errorf("GET /me headers = %v, want no-store and noindex", resp.Header)
	}
}

// TestPersonPageBlockedByYou is T-09-32: a person you blocked shows only the
// You blocked box with Unblock, and Unblock brings the profile back.
func TestPersonPageBlockedByYou(t *testing.T) {
	srv, mailer, pool := p907Server(t)
	viewer, viewerID := p907Person(t, srv, mailer, pool, "p908-blk-viewer@example.com", "blk_viewer", false)
	other := testutil.SeedPerson(t, pool, "p908-blk-other@example.com", "blk_other", true)
	testutil.SeedBlock(t, pool, viewerID, other)

	status, body, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/blk_other", "")
	if status != http.StatusOK {
		t.Fatalf("GET /u/blk_other: status = %d, want 200", status)
	}
	for _, want := range []string{
		"You blocked blk_other",
		"They can't see your profile or follow you. Their reports still show in the feed.",
		`id="person-unblock"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the blocked page lacks %q", want)
		}
	}
	for _, banned := range []string{`id="person-header"`, `id="person-follow"`, `id="person-reports-section"`} {
		if strings.Contains(body, banned) {
			t.Errorf("the blocked page must not draw %s", banned)
		}
	}
	if !p908Shown(body, "person-blocked") {
		t.Errorf("the blocked box must be shown")
	}

	p907Post(t, viewer, srv, "/api/people/blk_other/unblock", http.StatusNoContent)
	status, again, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/blk_other", "")
	if status != http.StatusOK || !strings.Contains(again, `id="person-header"`) {
		t.Errorf("after Unblock the page must draw the profile again (status %d)", status)
	}
}

// p911RowPattern matches the li elements of the Recent reports list.
var p911RowPattern = regexp.MustCompile(`(?s)<li class="person-report [^"]*">.*?</li>`)

// p911Setup seeds one public reporter (six finished reports from
// SeedPublicReporter), one live report nobody has voted on, one live report
// three people dispute, and one report 40 days old that is out of the window.
// It returns the live ids, the old id and the signed in viewer's page body.
func p911Setup(t *testing.T) (body string, liveID, disputedID, oldID int64) {
	t.Helper()
	srv, mailer, pool := p907Server(t)
	viewer, _ := p907Person(t, srv, mailer, pool, "p911-viewer@example.com", "p911_viewer", false)
	now := time.Now().UTC()
	testutil.SeedPublicReporter(t, pool, "p911-owner@example.com", "p911_owner", "sess-p911-owner", now)

	liveID = testutil.SeedHistoryReport(t, pool, "sess-p911-owner", "medium", "flood", now.Add(2*time.Hour))
	disputedID = testutil.SeedHistoryReport(t, pool, "sess-p911-owner", "low", "flood", now.Add(3*time.Hour))
	p911Dispute(t, pool, disputedID, now)
	oldID = testutil.SeedHistoryReport(t, pool, "sess-p911-owner", "low", "flood", now.Add(-40*24*time.Hour))

	status, page, _ := p907Do(t, viewer, http.MethodGet, srv.URL+"/u/p911_owner", "")
	if status != http.StatusOK {
		t.Fatalf("GET /u/p911_owner: status = %d, want 200", status)
	}
	return page, liveID, disputedID, oldID
}

// p911Dispute adds three disputes from three voters in three cells.
func p911Dispute(t *testing.T, pool *pgxpool.Pool, reportID int64, now time.Time) {
	t.Helper()
	cells := []string{"tdr1v8y", "tdr1vc3", "tdr1vd2"}
	for i, cell := range cells {
		voter := testutil.SeedReporterAccount(t, pool, "p911-disputer-"+strconv.Itoa(i)+"@example.com")
		if _, err := pool.Exec(context.Background(),
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
			 VALUES ($1, $2, 'content', 'dispute', $3, $4)`,
			reportID, voter, cell, now.Add(-30*time.Minute),
		); err != nil {
			t.Fatalf("seeding a dispute on report %d: %v", reportID, err)
		}
	}
}

// TestPersonPageRecentReports is V10 for the page: a live report is a link to
// the Phase 8 report link, a disputed one is listed with its line and its word,
// the finished ones are listed, and a report older than 30 days is absent.
func TestPersonPageRecentReports(t *testing.T) {
	body, liveID, disputedID, _ := p911Setup(t)
	rows := p911RowPattern.FindAllString(body, -1)

	var live, finished int
	var sawLive, sawDisputed bool
	for _, row := range rows {
		switch {
		case strings.Contains(row, "person-report--live"):
			live++
			if !strings.Contains(row, `<a class="person-report__body" href="/?report=`) {
				t.Errorf("a live row is not a link to the report link: %s", row)
			}
			if strings.Contains(row, `href="/?report=`+strconv.FormatInt(liveID, 10)+`"`) {
				sawLive = true
				if !strings.Contains(row, "Not confirmed yet") || !strings.Contains(row, "Posted ") {
					t.Errorf("the live row lacks its status word or its time: %s", row)
				}
			}
			if strings.Contains(row, `href="/?report=`+strconv.FormatInt(disputedID, 10)+`"`) {
				sawDisputed = true
				if !strings.Contains(row, "Disputed: may not be true") || !strings.Contains(row, `person-chip--grey">Disputed<`) {
					t.Errorf("the disputed row lacks its line or its chip: %s", row)
				}
			}
		case strings.Contains(row, "person-report--finished"):
			finished++
		default:
			t.Errorf("a row is neither live nor finished: %s", row)
		}
	}
	if !sawLive || !sawDisputed {
		t.Errorf("live row listed = %v, disputed row listed = %v, want both", sawLive, sawDisputed)
	}
	if live != 2 {
		t.Errorf("live rows = %d, want 2", live)
	}
	// SeedPublicReporter files six finished reports; the 40 day old one is not
	// among the rows, so the list holds six finished rows and no more.
	if finished != 6 {
		t.Errorf("finished rows = %d, want 6 (the report from 40 days ago must be absent)", finished)
	}
	for _, want := range []string{"Recent reports", "Reports from the last 30 days, newest first.", ">Report<", ">Status<", ">When<", ">Distance<"} {
		if !strings.Contains(body, want) {
			t.Errorf("the section lacks %q", want)
		}
	}
}

// TestPersonPageFinishedRowsAreCoarse is T-09-40 and D-20 for the page: a
// finished row is not a link and holds no tabindex, no description, no id and no
// time of day; its coordinates are the precision 5 cell centre, never the
// stored point, and its When text is the word Ended and a date.
func TestPersonPageFinishedRowsAreCoarse(t *testing.T) {
	body, _, _, _ := p911Setup(t)
	storedLat, storedLon := 12.9716, 77.5946
	wantLat, wantLon, _ := service.CoarsePointAt(storedLat, storedLon, service.ProfileCoarsePrecision)
	if wantLat == storedLat || wantLon == storedLon {
		t.Fatalf("the coarse centre %v, %v equals the stored point: the test proves nothing", wantLat, wantLon)
	}

	latRe := regexp.MustCompile(`data-lat="([^"]+)" data-lon="([^"]+)" data-approx="([^"]+)"`)
	whenRe := regexp.MustCompile(`<span class="person-report__when">([^<]*)</span>`)
	dateRe := regexp.MustCompile(`^Ended [0-9]{1,2} [A-Z][a-z]+ [0-9]{4}$`)
	seen := 0
	for _, row := range p911RowPattern.FindAllString(body, -1) {
		if !strings.Contains(row, "person-report--finished") {
			continue
		}
		seen++
		for _, banned := range []string{"<a ", "href=", "tabindex", "seeded history report", "app-icon--chevron-right", "data-report-id"} {
			if strings.Contains(row, banned) {
				t.Errorf("a finished row holds %q: %s", banned, row)
			}
		}
		for _, banned := range []string{"12.9716", "77.5946"} {
			if strings.Contains(row, banned) {
				t.Errorf("a finished row holds the stored point %q: %s", banned, row)
			}
		}
		m := latRe.FindStringSubmatch(row)
		if m == nil {
			t.Fatalf("a finished row has no coordinates: %s", row)
		}
		lat, _ := strconv.ParseFloat(m[1], 64)
		lon, _ := strconv.ParseFloat(m[2], 64)
		if math.Abs(lat-wantLat) > 1e-9 || math.Abs(lon-wantLon) > 1e-9 || m[3] != "true" {
			t.Errorf("a finished row has %v, %v approx %s; want the cell centre %v, %v and true", lat, lon, m[3], wantLat, wantLon)
		}
		w := whenRe.FindStringSubmatch(row)
		if w == nil || !dateRe.MatchString(w[1]) {
			t.Errorf("a finished row's When text = %q, want Ended and a date with no time", w)
		}
		if strings.Contains(row, " ago") || regexp.MustCompile(`[0-9]:[0-9]{2}`).MatchString(row) || strings.Contains(row, "datetime") {
			t.Errorf("a finished row carries a relative time or a time of day: %s", row)
		}
		if !strings.Contains(row, "Ended.") {
			t.Errorf("a finished row's name must start with Ended.: %s", row)
		}
	}
	if seen == 0 {
		t.Fatal("no finished row was listed, so nothing was checked")
	}
}
