// review_hide_e2e_test.go proves quick 261117-vvb over the real router and a
// real Postgres: the flag route takes an optional position and refuses a bad
// one, flags from three places hide a report for everyone, the author's bell
// and full updates list say so in plain rows, a moderator's Keep brings it back
// for good, a Remove points the author to the Policy, and the place cell is in
// no answer.
package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api"
	"pinalert/internal/service"
)

const vvbLat, vvbLon = 12.95, 77.61

// vvbBody is a report body at the shared point.
func vvbBody(description string) map[string]any {
	return map[string]any{
		"category":    "flood",
		"severity":    "low",
		"description": description,
		"latitude":    vvbLat,
		"longitude":   vvbLon,
	}
}

// vvbPlace is a flagger position k steps of about 330 metres north of the
// report point; each k is its own place cell.
func vvbPlace(k int) (float64, float64) {
	return vvbLat + float64(k)*0.003, vvbLon
}

// vvbFlag posts a raw flag body to the report's flag route.
func vvbFlag(t *testing.T, client *http.Client, baseURL string, reportID int64, body string) *http.Response {
	t.Helper()
	return p20PostFlagRaw(t, client, baseURL+"/api/reports/"+strconv.FormatInt(reportID, 10)+"/flag", body, "")
}

// vvbFlagAt flags the report from place k and requires 201.
func vvbFlagAt(t *testing.T, client *http.Client, baseURL string, reportID int64, k int) {
	t.Helper()
	lat, lon := vvbPlace(k)
	body := `{"reasons":["false"],"latitude":` + strconv.FormatFloat(lat, 'f', -1, 64) + `,"longitude":` + strconv.FormatFloat(lon, 'f', -1, 64) + `}`
	raw := p20Status(t, "flag from place "+strconv.Itoa(k), vvbFlag(t, client, baseURL, reportID, body), http.StatusCreated)
	if strings.TrimSpace(string(raw)) != `{"flagged":true}` {
		t.Errorf("flag answer = %s, want only flagged true", raw)
	}
}

// vvbUpdate is one row of the bell answer with the link field.
type vvbUpdate struct {
	Kind       string `json:"kind"`
	ReportID   *int64 `json:"report_id"`
	Link       string `json:"link"`
	Available  bool   `json:"available"`
	Event      string `json:"event"`
	ReportLine string `json:"report_line"`
	Category   string `json:"category"`
	Trust      string `json:"trust"`
}

func vvbBell(t *testing.T, client *http.Client, baseURL string) (map[string]vvbUpdate, string) {
	t.Helper()
	_, raw := p32uPoll(t, "bell", client, baseURL)
	var out struct {
		Updates []vvbUpdate `json:"updates"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the bell: %v", err)
	}
	rows := map[string]vvbUpdate{}
	for _, u := range out.Updates {
		if _, dup := rows[u.Kind]; dup {
			t.Errorf("the bell holds two %s rows", u.Kind)
		}
		rows[u.Kind] = u
	}
	return rows, string(raw)
}

// vvbRowOf returns the list item of the full updates list that holds words.
func vvbRowOf(t *testing.T, page, words string) string {
	t.Helper()
	at := strings.Index(page, words)
	if at < 0 {
		t.Fatalf("the updates list lacks %q", words)
	}
	start := strings.LastIndex(page[:at], "<li")
	end := strings.Index(page[at:], "</li>")
	if start < 0 || end < 0 {
		t.Fatalf("no list item around %q", words)
	}
	return page[start : at+end]
}

func TestReviewHideFlow(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), func(d *api.Deps) {
		d.ModeratorEmails = modAllowList(t, "vvb-mod@example.com")
	}))
	author := newVerifiedClient(t, srv, mailer, "vvb-author@example.com")
	reader := newVerifiedClient(t, srv, mailer, "vvb-reader@example.com")
	moderator := newVerifiedClient(t, srv, mailer, "vvb-mod@example.com")
	flaggers := make([]*http.Client, 7)
	for i := range flaggers {
		flaggers[i] = newVerifiedClient(t, srv, mailer, "vvb-flagger-"+strconv.Itoa(i)+"@example.com")
	}

	const descHidden = "Road under water by the bus depot"
	const descRemoved = "Shelter open at the community hall"
	hidden := submitReportGetID(t, author, srv.URL, vvbBody(descHidden))
	removed := submitReportGetID(t, author, srv.URL, vvbBody(descRemoved))

	inFeed := func(client *http.Client, id int64) bool {
		t.Helper()
		_, ok := findFeedReport(decodeFeedResponse(t, getNearby(t, client, srv.URL, vvbLat, vvbLon, false)), id)
		return ok
	}

	t.Run("the flag route refuses half a position and one outside the map", func(t *testing.T) {
		cases := []struct{ name, body, field string }{
			{"latitude alone", `{"reasons":["spam"],"latitude":12.95}`, "longitude"},
			{"longitude alone", `{"reasons":["spam"],"longitude":77.61}`, "latitude"},
			{"latitude past the pole", `{"reasons":["spam"],"latitude":91,"longitude":77.61}`, "latitude"},
			{"longitude past the date line", `{"reasons":["spam"],"latitude":12.95,"longitude":181}`, "longitude"},
		}
		for _, c := range cases {
			resp := vvbFlag(t, flaggers[0], srv.URL, hidden, c.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s: status %d, want 400", c.name, resp.StatusCode)
			}
			if field, _ := decodeErrorResponse(t, resp); field != c.field {
				t.Errorf("%s: field %q, want %q", c.name, field, c.field)
			}
		}
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM report_flags WHERE report_id = $1`, hidden).Scan(&n); err != nil {
			t.Fatalf("counting flags: %v", err)
		}
		if n != 0 {
			t.Errorf("%d flags stored by refused requests, want 0", n)
		}
	})

	t.Run("flags from three places hide the report for everyone", func(t *testing.T) {
		vvbFlagAt(t, flaggers[0], srv.URL, hidden, 1)
		vvbFlagAt(t, flaggers[1], srv.URL, hidden, 2)
		if !inFeed(reader, hidden) {
			t.Fatal("the report left the feed after two places")
		}
		vvbFlagAt(t, flaggers[2], srv.URL, hidden, 3)
		for name, client := range map[string]*http.Client{"reader": reader, "author": author} {
			if inFeed(client, hidden) {
				t.Errorf("%s still sees the hidden report on Nearby", name)
			}
			link, _ := h10Get(t, client, h10URL(srv.URL, vvbLat, vvbLon, "report="+strconv.FormatInt(hidden, 10)))
			if link.Focus == nil || link.Focus.Status != "unavailable" {
				t.Errorf("%s: link to the hidden report: focus = %+v, want unavailable", name, link.Focus)
			}
		}
		resp := vvbFlag(t, flaggers[3], srv.URL, hidden, `{"reasons":["spam"]}`)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("a flag on the hidden report: status %d, want 404", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("the author's bell says it is hidden, in a plain row with no report words", func(t *testing.T) {
		rows, raw := vvbBell(t, author, srv.URL)
		row, ok := rows["own_review"]
		if !ok {
			t.Fatalf("no own_review row in the author's bell: %s", raw)
		}
		if row.Event != "Your report is hidden while it's reviewed." || !row.Available || row.ReportID != nil || row.Link != "" {
			t.Errorf("own_review row = %+v, want the hidden line, available, no report id and no link", row)
		}
		if row.ReportLine != "" || row.Category != "" || row.Trust != "" {
			t.Errorf("own_review row shows report detail: %+v", row)
		}
		for _, banned := range []string{service.UnavailableLine, descHidden} {
			if strings.Contains(raw, banned) {
				t.Errorf("the author's bell carries %q", banned)
			}
		}
	})

	t.Run("the moderator sees it first, marked review, and never the cell", func(t *testing.T) {
		status, _, q, raw := modGetQueue(t, moderator, srv.URL)
		if status != http.StatusOK {
			t.Fatalf("queue status %d", status)
		}
		if len(q.Flagged) == 0 || q.Flagged[0]["moderation"] != "review" {
			t.Fatalf("the first flagged row is not the hidden report: %s", raw)
		}
		if id, _ := q.Flagged[0]["id"].(float64); int64(id) != hidden {
			t.Errorf("the first flagged row is report %v, want %d", q.Flagged[0]["id"], hidden)
		}
		for k := 1; k <= 3; k++ {
			lat, lon := vvbPlace(k)
			cell := geohash.EncodeWithPrecision(lat, lon, 7)
			if strings.Contains(string(raw), cell) {
				t.Errorf("the moderator queue carries the place cell %q", cell)
			}
		}
		if strings.Contains(string(raw), "cell") {
			t.Error("the moderator queue names a cell")
		}
	})

	t.Run("Keep shows it again for good and tells the author", func(t *testing.T) {
		if status, out := modAction(t, moderator, srv.URL, hidden, "keep"); status != http.StatusOK {
			t.Fatalf("keep status %d: %+v", status, out)
		}
		if !inFeed(reader, hidden) {
			t.Error("the kept report is not back on Nearby")
		}
		rows, raw := vvbBell(t, author, srv.URL)
		row, ok := rows["own_kept"]
		if !ok {
			t.Fatalf("no own_kept row: %s", raw)
		}
		if row.Event != "Your report is showing again. Sorry for the wait." || row.ReportID == nil || *row.ReportID != hidden || row.Link != "" {
			t.Errorf("own_kept row = %+v, want the showing again line linking to report %d", row, hidden)
		}
		for k := 4; k <= 6; k++ {
			vvbFlagAt(t, flaggers[k], srv.URL, hidden, k)
		}
		if !inFeed(reader, hidden) {
			t.Error("fresh flags from three places hid a kept report")
		}
	})

	t.Run("Remove points the author to the Policy", func(t *testing.T) {
		if status, out := modAction(t, moderator, srv.URL, removed, "remove"); status != http.StatusOK {
			t.Fatalf("remove status %d: %+v", status, out)
		}
		rows, raw := vvbBell(t, author, srv.URL)
		row, ok := rows["own_removed"]
		if !ok {
			t.Fatalf("no own_removed row: %s", raw)
		}
		if row.Event != "Your report was removed after review." || row.Link != "policy" || row.ReportID != nil || !row.Available {
			t.Errorf("own_removed row = %+v, want the removed line, link policy and no report id", row)
		}
		for _, banned := range []string{service.UnavailableLine, descRemoved, descHidden} {
			if strings.Contains(raw, banned) {
				t.Errorf("the author's bell carries %q", banned)
			}
		}
	})

	t.Run("the full updates list draws the same rows", func(t *testing.T) {
		page := getPageBody(t, author, srv.URL, "/profile?part=updates")
		if strings.Contains(page, service.UnavailableLine) {
			t.Errorf("the updates list says %q", service.UnavailableLine)
		}
		hiddenRow := vvbRowOf(t, page, "Your report is hidden while it&#39;s reviewed.")
		if strings.Contains(hiddenRow, "href=") {
			t.Errorf("the hidden row is a link: %s", hiddenRow)
		}
		keptRow := vvbRowOf(t, page, "Your report is showing again. Sorry for the wait.")
		if !strings.Contains(keptRow, `href="/?report=`+strconv.FormatInt(hidden, 10)+`"`) {
			t.Errorf("the kept row does not open the report: %s", keptRow)
		}
		removedRow := vvbRowOf(t, page, "Your report was removed after review.")
		if !strings.Contains(removedRow, `href="/policy"`) {
			t.Errorf("the removed row does not open the Policy: %s", removedRow)
		}
		if regexp.MustCompile(`href="/\?report=` + strconv.FormatInt(removed, 10) + `"`).MatchString(page) {
			t.Error("the updates list links to the removed report")
		}
	})

	t.Run("no answer carries a place cell", func(t *testing.T) {
		_, bell := vvbBell(t, author, srv.URL)
		feed := getNearby(t, reader, srv.URL, vvbLat, vvbLon, false)
		defer feed.Body.Close()
		var buf strings.Builder
		if _, err := io.Copy(&buf, feed.Body); err != nil {
			t.Fatalf("reading the feed: %v", err)
		}
		for k := 1; k <= 6; k++ {
			lat, lon := vvbPlace(k)
			cell := geohash.EncodeWithPrecision(lat, lon, 7)
			for name, body := range map[string]string{"bell": bell, "feed": buf.String()} {
				if strings.Contains(body, cell) {
					t.Errorf("the %s carries the place cell %q", name, cell)
				}
			}
		}
	})
}
