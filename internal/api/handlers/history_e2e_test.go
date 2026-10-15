// history_e2e_test.go proves plan 03.1-10's wire behaviour of GET /api/reports
// end to end over the real router and a real Postgres: the strict view,
// category and report parameters, the Past view, the 24 hour counts for an
// empty Active tab and the focus object of a report link. It reuses
// newE2EServer, newVerifiedClient, postVote and the report helpers from the
// older e2e files and declares its own decode shapes with new names so it
// asserts the JSON contract and not the server's internal types.
package handlers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/testutil"
)

// h10Report is one report of a list as a browser decodes it.
type h10Report struct {
	ID             int64    `json:"id"`
	Category       string   `json:"category"`
	Description    string   `json:"description"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	Geohash        string   `json:"geohash"`
	DistanceKm     *float64 `json:"distance_km"`
	Visibility     string   `json:"visibility"`
	View           string   `json:"view"`
	Approximate    bool     `json:"approximate"`
	YourResolution *string  `json:"your_resolution"`
	UndosLeft      int      `json:"undos_left"`
	History        *struct {
		State        string `json:"state"`
		At           string `json:"at"`
		ClosedBy     string `json:"closed_by"`
		ClosedText   string `json:"closed_text"`
		Outcome      string `json:"outcome"`
		OutcomeLabel string `json:"outcome_label"`
		LifetimeText string `json:"lifetime_text"`
	} `json:"history"`
}

// h10List is the whole GET /api/reports body.
type h10List struct {
	View    string      `json:"view"`
	Reports []h10Report `json:"reports"`
	Recent  *struct {
		Resolved24h int `json:"resolved_24h"`
		Expired24h  int `json:"expired_24h"`
	} `json:"recent"`
	Cut *struct {
		Resolved        bool `json:"resolved"`
		Expired         bool `json:"expired"`
		ResolvedPartial bool `json:"resolved_partial"`
		ExpiredPartial  bool `json:"expired_partial"`
	} `json:"cut"`
	Focus *struct {
		Status string     `json:"status"`
		Report *h10Report `json:"report"`
	} `json:"focus"`
}

// h10Centre is where the fixture reports are posted and read from.
const h10Lat, h10Lon = 12.9716, 77.5946

// h10URL builds a GET /api/reports URL; extra is appended as is and may be "".
func h10URL(baseURL string, lat, lon float64, extra string) string {
	u := baseURL + "/api/reports?lat=" + strconv.FormatFloat(lat, 'f', -1, 64) +
		"&lon=" + strconv.FormatFloat(lon, 'f', -1, 64) + "&radius_km=5"
	if extra != "" {
		u += "&" + extra
	}
	return u
}

// h10Get issues the request and decodes a 200 body, returning the raw bytes
// too so a test can assert on text a struct does not name.
func h10Get(t *testing.T, client *http.Client, url string) (h10List, []byte) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d: %s", url, resp.StatusCode, body)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("GET %s: Cache-Control = %q, want no-store", url, cc)
	}
	var out h10List
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding %s: %v (%s)", url, err, body)
	}
	return out, body
}

// h10GetRefused issues the request and returns the 400 field and message.
func h10GetRefused(t *testing.T, client *http.Client, url string) (field, message string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("GET %s: status %d, want 400: %s", url, resp.StatusCode, b)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("GET %s: 400 Cache-Control = %q, want no-store", url, cc)
	}
	return decodeErrorResponse(t, resp)
}

// h10Find returns the report with the id, or nil.
func h10Find(reports []h10Report, id int64) *h10Report {
	for i := range reports {
		if reports[i].ID == id {
			return &reports[i]
		}
	}
	return nil
}

// h10Post submits a report body and returns its id.
func h10Post(t *testing.T, client *http.Client, baseURL string, body map[string]any) int64 {
	t.Helper()
	return submitReportGetID(t, client, baseURL, body)
}

// h10Resolve has the poster resolve their own report, which closes it at once.
func h10Resolve(t *testing.T, client *http.Client, baseURL string, id int64, lat, lon float64) {
	t.Helper()
	vis, reason := decodeVoteResponse(t, postVote(t, client, baseURL, id, "resolve", map[string]any{"latitude": lat, "longitude": lon}))
	if vis != "retracted" || reason != "resolved" {
		t.Fatalf("resolve of report %d: visibility=%q reason=%q, want retracted/resolved", id, vis, reason)
	}
}

// h10MovePast moves a report into the past by SQL: it was created createdAgo
// seconds ago and expired expiredAgo seconds ago. Never by sleeping.
func h10MovePast(t *testing.T, pool *pgxpool.Pool, id int64, createdAgo, expiredAgo float64) {
	t.Helper()
	testutil.MustExec(t, pool,
		`UPDATE reports SET created_at = now() - make_interval(secs => $2), expires_at = now() - make_interval(secs => $3) WHERE id = $1`,
		id, createdAgo, expiredAgo)
}

// h10Haversine is the great circle distance in km with the radius the feed's
// SQL and the service use.
func h10Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dPhi := (lat2 - lat1) * rad
	dLambda := (lon2 - lon1) * rad
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return 2 * 6371.0 * math.Asin(math.Sqrt(math.Min(1, math.Max(0, a))))
}

func h10PowerBody(lat, lon float64) map[string]any {
	return map[string]any{
		"category":    "power_outage",
		"severity":    "low",
		"description": "Whole street without power since the evening",
		"latitude":    lat,
		"longitude":   lon,
	}
}

// TestFeedViewStrictAndAlias: an unknown value is a 400 naming the parameter,
// resolved is answered as past, show_disputed=true is the disputed view and
// disagrees with another explicit view (D-04, D-05, D-17, D-28).
func TestFeedViewStrictAndAlias(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "h10-strict@example.com")

	refused := []struct{ extra, field string }{
		{"view=bogus", "view"},
		{"view=Past", "view"},
		{"show_disputed=true&view=past", "view"},
		{"show_disputed=true&view=active", "view"},
		{"category=water", "category"},
		{"category=Flood", "category"},
		{"report=abc", "report"},
		{"report=0", "report"},
		{"report=-4", "report"},
		{"show_disputed=yes", "show_disputed"},
	}
	for _, tc := range refused {
		field, message := h10GetRefused(t, client, h10URL(srv.URL, h10Lat, h10Lon, tc.extra))
		if field != tc.field {
			t.Errorf("%s: error.field = %q, want %q (message %q)", tc.extra, field, tc.field, message)
		}
	}

	wantView := []struct{ extra, view string }{
		{"", "active"},
		{"view=active", "active"},
		{"view=disputed", "disputed"},
		{"view=past", "past"},
		{"view=resolved", "past"},
		{"show_disputed=true", "disputed"},
		{"show_disputed=true&view=disputed", "disputed"},
		{"show_disputed=false", "active"},
	}
	for _, tc := range wantView {
		list, body := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, tc.extra))
		if list.View != tc.view {
			t.Errorf("%q: view = %q, want %q", tc.extra, list.View, tc.view)
		}
		if list.Reports == nil || !strings.Contains(string(body), `"reports":[]`) {
			t.Errorf("%q: reports is not an empty array: %s", tc.extra, body)
		}
	}
}

// TestPastViewListsRetractedAndExpired: a resolved report and one that ran out
// its time are both in the past view, the resolved group first, with the
// history facts of each.
func TestPastViewListsRetractedAndExpired(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "h10-past-poster@example.com")
	other := newVerifiedClient(t, srv, mailer, "h10-past-other@example.com")

	resolvedID := h10Post(t, poster, srv.URL, floodReportBody(h10Lat, h10Lon))
	h10Resolve(t, poster, srv.URL, resolvedID, h10Lat, h10Lon)

	expiredID := h10Post(t, poster, srv.URL, h10PowerBody(h10Lat, h10Lon))
	h10MovePast(t, pool, expiredID, 3*3600, 3600)

	posterList, _ := h10Get(t, poster, h10URL(srv.URL, h10Lat, h10Lon, "view=past"))
	otherList, _ := h10Get(t, other, h10URL(srv.URL, h10Lat, h10Lon, "view=past"))

	for name, list := range map[string]h10List{"poster": posterList, "other": otherList} {
		if list.View != "past" {
			t.Fatalf("%s: view = %q, want past", name, list.View)
		}
		if len(list.Reports) != 2 {
			t.Fatalf("%s: %d reports in the past view, want 2: %+v", name, len(list.Reports), list.Reports)
		}
		if list.Reports[0].ID != resolvedID || list.Reports[1].ID != expiredID {
			t.Fatalf("%s: order = %d, %d, want the resolved report %d before the expired one %d",
				name, list.Reports[0].ID, list.Reports[1].ID, resolvedID, expiredID)
		}
		if list.Cut == nil || list.Cut.Resolved || list.Cut.Expired || list.Cut.ResolvedPartial || list.Cut.ExpiredPartial {
			t.Errorf("%s: cut = %+v, want all four flags false", name, list.Cut)
		}
		if list.Recent != nil {
			t.Errorf("%s: recent present on the past view", name)
		}
		res, exp := list.Reports[0], list.Reports[1]
		if res.View != "resolved" || res.History == nil || res.History.State != "resolved" {
			t.Errorf("%s: resolved report view=%q history=%+v", name, res.View, res.History)
		}
		if exp.View != "past" || exp.History == nil || exp.History.State != "expired" {
			t.Errorf("%s: expired report view=%q history=%+v", name, exp.View, exp.History)
		}
		if exp.History != nil {
			if exp.History.Outcome == "" || exp.History.OutcomeLabel == "" || exp.History.LifetimeText == "" {
				t.Errorf("%s: expired history lacks outcome, label or lifetime: %+v", name, exp.History)
			}
			if exp.History.ClosedBy != "" || exp.History.ClosedText != "" {
				t.Errorf("%s: expired history carries a closer: %+v", name, exp.History)
			}
		}
		if res.History != nil && (res.History.Outcome != "" || res.History.LifetimeText != "") {
			t.Errorf("%s: resolved history carries expired keys: %+v", name, res.History)
		}
	}
	if got := posterList.Reports[0].History.ClosedBy; got != "you" {
		t.Errorf("closed_by for the poster = %q, want you", got)
	}
	if got := otherList.Reports[0].History.ClosedBy; got != "poster" {
		t.Errorf("closed_by for another viewer = %q, want poster", got)
	}
	if got := posterList.Reports[0].YourResolution; got == nil || *got != "resolve" {
		t.Errorf("poster your_resolution = %v, want resolve", got)
	}
	if got := otherList.Reports[0].YourResolution; got != nil {
		t.Errorf("another viewer your_resolution = %v, want null", *got)
	}
}

// TestPastViewNeverLeaksIntoActive: a retracted report is in Past only, and
// its presence there never changes an Active count (D-03, D-25).
func TestPastViewNeverLeaksIntoActive(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "h10-leak-active@example.com")

	controlID := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	before, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	if len(before.Reports) != 1 || before.Reports[0].ID != controlID {
		t.Fatalf("before: active = %+v, want the one control report", before.Reports)
	}

	retractedID := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	h10Resolve(t, client, srv.URL, retractedID, h10Lat, h10Lon)

	for _, extra := range []string{"", "view=active", "view=disputed", "show_disputed=true"} {
		list, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, extra))
		if r := h10Find(list.Reports, retractedID); r != nil {
			t.Errorf("%q: the retracted report %d is listed in %s", extra, retractedID, list.View)
		}
	}
	after, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	if len(after.Reports) != len(before.Reports) {
		t.Errorf("active count changed from %d to %d once a report was resolved", len(before.Reports), len(after.Reports))
	}
	past, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "view=past"))
	if h10Find(past.Reports, retractedID) == nil {
		t.Errorf("the retracted report %d is missing from the past view", retractedID)
	}
	if h10Find(past.Reports, controlID) != nil {
		t.Errorf("the live control report %d is listed in the past view", controlID)
	}
}

// TestCategoryFilterEndToEnd: a category keeps only that category on the
// active view and on the past view (D-17).
func TestCategoryFilterEndToEnd(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "h10-category@example.com")

	liveFlood := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	livePower := h10Post(t, client, srv.URL, h10PowerBody(h10Lat, h10Lon))
	doneFlood := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	donePower := h10Post(t, client, srv.URL, h10PowerBody(h10Lat, h10Lon))
	h10Resolve(t, client, srv.URL, doneFlood, h10Lat, h10Lon)
	h10Resolve(t, client, srv.URL, donePower, h10Lat, h10Lon)

	active, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "category=flood"))
	if len(active.Reports) != 1 || active.Reports[0].ID != liveFlood || active.Reports[0].Category != "flood" {
		t.Errorf("active category=flood = %+v, want only the live flood %d", active.Reports, liveFlood)
	}
	all, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	if h10Find(all.Reports, livePower) == nil || len(all.Reports) != 2 {
		t.Errorf("unfiltered active = %+v, want both live reports", all.Reports)
	}

	past, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "view=past&category=flood"))
	if len(past.Reports) != 1 || past.Reports[0].ID != doneFlood || past.Reports[0].Category != "flood" {
		t.Errorf("past category=flood = %+v, want only the resolved flood %d", past.Reports, doneFlood)
	}
	pastPower, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "view=past&category=power_outage"))
	if len(pastPower.Reports) != 1 || pastPower.Reports[0].ID != donePower {
		t.Errorf("past category=power_outage = %+v, want only %d", pastPower.Reports, donePower)
	}
}

// TestRecentCountsOnlyWhenActiveEmpty: recent is answered only when the Active
// view is unfiltered and empty (D-12).
func TestRecentCountsOnlyWhenActiveEmpty(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "h10-recent@example.com")

	resolvedID := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	h10Resolve(t, client, srv.URL, resolvedID, h10Lat, h10Lon)

	list, body := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	if len(list.Reports) != 0 {
		t.Fatalf("active = %+v, want empty", list.Reports)
	}
	if list.Recent == nil || list.Recent.Resolved24h != 1 || list.Recent.Expired24h != 0 {
		t.Fatalf("recent = %+v, want resolved_24h 1 and expired_24h 0 (%s)", list.Recent, body)
	}
	if list.Cut != nil || list.Focus != nil {
		t.Errorf("cut or focus present on the active view: %s", body)
	}

	expiredID := h10Post(t, client, srv.URL, h10PowerBody(h10Lat, h10Lon))
	h10MovePast(t, pool, expiredID, 3*3600, 3600)
	list, _ = h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	if list.Recent == nil || list.Recent.Resolved24h != 1 || list.Recent.Expired24h != 1 {
		t.Fatalf("recent = %+v, want 1 and 1", list.Recent)
	}

	// A category was sent, or the view is not active: no recent object.
	for _, extra := range []string{"category=flood", "view=past", "view=disputed"} {
		list, body := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, extra))
		if list.Recent != nil || strings.Contains(string(body), `"recent"`) {
			t.Errorf("%q: recent is present: %s", extra, body)
		}
	}

	// A live report is present: no recent object.
	h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	list, body = h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	if len(list.Reports) != 1 {
		t.Fatalf("active = %+v, want the one live report", list.Reports)
	}
	if list.Recent != nil || strings.Contains(string(body), `"recent"`) {
		t.Errorf("recent is present beside a live report: %s", body)
	}
}

// TestFocusOutsideRadiusAndWindow: a report link opens a live report far
// outside the radius and a report resolved outside the reader's area, while
// reports stays the reader's own area (D-11).
func TestFocusOutsideRadiusAndWindow(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	poster := newVerifiedClient(t, srv, mailer, "h10-focus-poster@example.com")
	reader := newVerifiedClient(t, srv, mailer, "h10-focus-reader@example.com")

	// Chennai is about 290 km from Bengaluru.
	const farLat, farLon = 13.0827, 80.2707
	farLive := h10Post(t, poster, srv.URL, floodReportBody(farLat, farLon))
	farDone := h10Post(t, poster, srv.URL, floodReportBody(farLat, farLon))
	h10Resolve(t, poster, srv.URL, farDone, farLat, farLon)
	nearLive := h10Post(t, poster, srv.URL, floodReportBody(h10Lat, h10Lon))

	list, body := h10Get(t, reader, h10URL(srv.URL, h10Lat, h10Lon, "report="+strconv.FormatInt(farLive, 10)))
	if list.Focus == nil || list.Focus.Status != "ok" || list.Focus.Report == nil || list.Focus.Report.ID != farLive {
		t.Fatalf("focus = %+v, want ok with report %d (%s)", list.Focus, farLive, body)
	}
	if list.Focus.Report.DistanceKm == nil || *list.Focus.Report.DistanceKm < 200 {
		t.Errorf("focus distance = %v, want about 290 km", list.Focus.Report.DistanceKm)
	}
	if len(list.Reports) != 1 || list.Reports[0].ID != nearLive {
		t.Errorf("reports = %+v, want only the reader's own area (%d)", list.Reports, nearLive)
	}
	if h10Find(list.Reports, farLive) != nil {
		t.Errorf("the far report %d leaked into the reader's reports", farLive)
	}

	list, _ = h10Get(t, reader, h10URL(srv.URL, h10Lat, h10Lon, "report="+strconv.FormatInt(farDone, 10)))
	if list.Focus == nil || list.Focus.Status != "ok" || list.Focus.Report == nil {
		t.Fatalf("resolved focus = %+v, want ok", list.Focus)
	}
	if list.Focus.Report.View != "resolved" || list.Focus.Report.History == nil || list.Focus.Report.History.State != "resolved" {
		t.Errorf("resolved focus report view=%q history=%+v", list.Focus.Report.View, list.Focus.Report.History)
	}
}

// TestFocusUnavailableAndTooOld: an unknown id gets the one unavailable answer
// with a null report, and a report that ended more than seven days ago is
// too_old.
func TestFocusUnavailableAndTooOld(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "h10-focus-old@example.com")

	list, body := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "report=987654"))
	if list.Focus == nil || list.Focus.Status != "unavailable" || list.Focus.Report != nil {
		t.Fatalf("unknown id focus = %+v, want unavailable with no report (%s)", list.Focus, body)
	}
	if !strings.Contains(string(body), `"report":null`) {
		t.Errorf("the focus report key is not an explicit null: %s", body)
	}

	oldID := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	h10MovePast(t, pool, oldID, 9*24*3600, 8*24*3600)
	list, _ = h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "report="+strconv.FormatInt(oldID, 10)))
	if list.Focus == nil || list.Focus.Status != "too_old" || list.Focus.Report != nil {
		t.Fatalf("old report focus = %+v, want too_old with no report", list.Focus)
	}

	// Recently expired is still shown, as past.
	recentID := h10Post(t, client, srv.URL, floodReportBody(h10Lat, h10Lon))
	h10MovePast(t, pool, recentID, 3*3600, 3600)
	list, _ = h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "report="+fmt.Sprint(recentID)))
	if list.Focus == nil || list.Focus.Status != "ok" || list.Focus.Report == nil || list.Focus.Report.View != "past" {
		t.Fatalf("recently expired focus = %+v, want ok with view past", list.Focus)
	}
}
