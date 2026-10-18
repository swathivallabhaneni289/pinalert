// updates_e2e_test.go is plan 03.2-07: the bell's poll, the read mark, mute and
// unmute and the muted list, proven over the real router and a real Postgres.
// It runs only on a disposable database (p32_07), through newE2EServerWith like
// every e2e in this package.
//
// The updates service reads the wall clock and the router gives the test no way
// to move it, so "an hour later" is made by moving the cursor row's
// last_update_at two hours back (p32uAgeCursors). Nothing else in the cursor is
// touched, so the self caused rule (adopted_at) keeps its meaning.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/testutil"
)

// p32uCells are four points a few hundred metres apart, each in its own voter
// cell (the same points the trust e2e uses), and all within voting range of the
// report point.
var p32uCells = [][2]float64{
	{12.9720, 77.5950},
	{12.9686, 77.5946},
	{12.9746, 77.5946},
	{12.9716, 77.5986},
}

// p32uRow is one row of GET /api/updates as a client reads it.
type p32uRow struct {
	ID            int64  `json:"id"`
	Kind          string `json:"kind"`
	ReportID      *int64 `json:"report_id"`
	Available     bool   `json:"available"`
	Unread        bool   `json:"unread"`
	Event         string `json:"event"`
	ReportLine    string `json:"report_line"`
	Category      string `json:"category"`
	Severity      string `json:"severity"`
	SeverityLevel string `json:"severity_level"`
	Trust         string `json:"trust"`
	When          string `json:"when"`
	Label         string `json:"label"`
}

type p32uWire struct {
	Unread  int       `json:"unread"`
	Updates []p32uRow `json:"updates"`
}

// p32uPoll is the bell poll for client. It requires 200 and no-store.
func p32uPoll(t *testing.T, label string, client *http.Client, baseURL string) (p32uWire, []byte) {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/updates")
	if err != nil {
		t.Fatalf("%s: GET /api/updates: %v", label, err)
	}
	raw := p20Status(t, label+": GET /api/updates", resp, http.StatusOK)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("%s: Cache-Control = %q, want no-store (T-03.2-20)", label, cc)
	}
	var out p32uWire
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: decoding %s: %v", label, raw, err)
	}
	return out, raw
}

// p32uPost posts a body (may be empty) and returns the response.
func p32uPost(t *testing.T, client *http.Client, url, body string) *http.Response {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(http.MethodPost, url, rd)
	if err != nil {
		t.Fatalf("building POST %s: %v", url, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// p32uVote casts a vote from the i-th cell and requires 200.
func p32uVote(t *testing.T, client *http.Client, baseURL string, reportID int64, action string, cell int) {
	t.Helper()
	status, body, err := p07PostVoteRaw(client, baseURL, reportID, action, p32uCells[cell][0], p32uCells[cell][1])
	if err != nil || status != http.StatusOK {
		t.Fatalf("%s on report %d from cell %d: status %d, err %v, body %s", action, reportID, cell, status, err, body)
	}
}

// p32uAgeCursors moves last_update_at of every cursor on a report two hours
// back, which is "an hour has passed" for the one update per report per hour
// rule.
func p32uAgeCursors(t *testing.T, pool *pgxpool.Pool, reportID int64) {
	t.Helper()
	testutil.MustExec(t, pool,
		`UPDATE update_cursors SET last_update_at = last_update_at - interval '2 hours' WHERE report_id = $1 AND last_update_at IS NOT NULL`,
		reportID)
}

// p32uStatus requires a status and returns the raw body.
func p32uStatus(t *testing.T, label string, resp *http.Response, want int) []byte {
	t.Helper()
	return p20Status(t, label, resp, want)
}

// p32uSetup starts a server and returns it with the pool and four verified
// clients A (the poster), B, C and D.
func p32uSetup(t *testing.T, prefix string) (srv *httptest.Server, pool *pgxpool.Pool, a, b, c, d *http.Client) {
	t.Helper()
	srv, mailer, pool := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	a = newVerifiedClient(t, srv, mailer, prefix+"-a@example.com")
	b = newVerifiedClient(t, srv, mailer, prefix+"-b@example.com")
	c = newVerifiedClient(t, srv, mailer, prefix+"-c@example.com")
	d = newVerifiedClient(t, srv, mailer, prefix+"-d@example.com")
	return srv, pool, a, b, c, d
}

// p32uLive makes the report Live from a baseline: B confirms from cell 0 and
// polls (the silent baseline), then C and D confirm from cells 1 and 2.
func p32uLive(t *testing.T, srv *httptest.Server, b, c, d *http.Client, id int64) {
	t.Helper()
	p32uVote(t, b, srv.URL, id, "confirm", 0)
	if w, _ := p32uPoll(t, "B baseline", b, srv.URL); len(w.Updates) != 0 || w.Unread != 0 {
		t.Fatalf("B baseline poll = %+v, want an empty list", w)
	}
	p32uVote(t, c, srv.URL, id, "confirm", 1)
	p32uVote(t, d, srv.URL, id, "confirm", 2)
}

// p32uBanned are the key fragments no updates answer may carry at any depth.
var p32uBanned = []string{"session", "account", "email", "latitude", "longitude", "geohash", "cell", "description", "voter"}

// p32uWalk fails on a banned key, on a string value that equals or contains a
// forbidden text, and on a number equal to a known coordinate. created_at is
// skipped for the string check because a timestamp's fraction can look like a
// coordinate.
func p32uWalk(t *testing.T, label string, v any, forbidden []string, coords []float64) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			lk := strings.ToLower(k)
			for _, bad := range p32uBanned {
				if strings.Contains(lk, bad) {
					t.Errorf("%s: key %q looks like an identifier or a coordinate", label, k)
				}
			}
			if k == "created_at" {
				continue
			}
			p32uWalk(t, label+"."+k, val, forbidden, coords)
		}
	case []any:
		for i, val := range x {
			p32uWalk(t, label+"["+strconv.Itoa(i)+"]", val, forbidden, coords)
		}
	case string:
		for _, f := range forbidden {
			if f != "" && strings.Contains(x, f) {
				t.Errorf("%s: value %q contains %q", label, x, f)
			}
		}
	case float64:
		for _, c := range coords {
			if x == c {
				t.Errorf("%s: number %v is a coordinate", label, x)
			}
		}
	}
}

// TestUpdatesEndpointLeaksNothing: a report reaches Live and the follower's
// bell shows one row with the report's first line only, and nothing in the whole
// answer names an account, a session, a cell, a coordinate, an email or the
// rest of the description (T-03.2-01).
func TestUpdatesEndpointLeaksNothing(t *testing.T) {
	srv, pool, a, b, c, d := p32uSetup(t, "p32-leak")

	body := p20Body("flood", "low")
	firstLine := "Water over the market bridge"
	secondLine := "Second private line about the neighbour gate"
	body["description"] = firstLine + "\n" + secondLine
	id := submitReportGetID(t, a, srv.URL, body)

	p32uVote(t, b, srv.URL, id, "confirm", 0)
	baseline, _ := p32uPoll(t, "B baseline", b, srv.URL)
	if len(baseline.Updates) != 0 || baseline.Unread != 0 {
		t.Fatalf("baseline = %+v, want an empty list and unread 0", baseline)
	}
	p32uVote(t, c, srv.URL, id, "confirm", 1)
	p32uVote(t, d, srv.URL, id, "confirm", 2)

	got, raw := p32uPoll(t, "B after Live", b, srv.URL)
	if got.Unread != 1 || len(got.Updates) != 1 {
		t.Fatalf("after Live = %+v, want one unread row (body %s)", got, raw)
	}
	row := got.Updates[0]
	if row.Kind != "reached_live" || row.Event != "Now confirmed" || !row.Unread || !row.Available {
		t.Errorf("row = %+v", row)
	}
	if row.ReportID == nil || *row.ReportID != id {
		t.Errorf("report_id = %v, want %d", row.ReportID, id)
	}
	if row.ReportLine != firstLine {
		t.Errorf("report_line = %q, want only the first line %q", row.ReportLine, firstLine)
	}
	if row.Category != "Flood" || row.Severity != "Low" || row.SeverityLevel != "low" {
		t.Errorf("words = %q / %q / %q", row.Category, row.Severity, row.SeverityLevel)
	}
	if !strings.HasPrefix(row.Trust, "Confirmed by 3 people") {
		t.Errorf("trust = %q, want a confirmed sentence naming 3 people", row.Trust)
	}
	if !strings.HasPrefix(row.Label, "New. Now confirmed. "+firstLine+". Flood, Low. Confirmed by 3 people") {
		t.Errorf("label = %q", row.Label)
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	coords := []float64{p20Lat, p20Lon}
	for _, cell := range p32uCells {
		coords = append(coords, cell[0], cell[1])
	}
	forbidden := []string{secondLine, "p32-leak-a@example.com", "p32-leak-b@example.com", "p32-leak-c@example.com", "p32-leak-d@example.com", "Second private"}
	p32uWalk(t, "answer", decoded, forbidden, coords)

	// A second poll is the same answer: nothing is written twice.
	again, _ := p32uPoll(t, "B again", b, srv.URL)
	if len(again.Updates) != 1 || again.Updates[0].ID != row.ID || again.Unread != 1 {
		t.Errorf("second poll = %+v, want the same single row", again)
	}
	_ = pool
}

// TestUpdatesOnePerReportPerHourOverHTTP: a report that goes Live and then
// Hidden inside an hour announces only the first; once an hour has passed the
// Hidden change is announced. A poster's own resolve tells the poster nothing
// (D-13).
func TestUpdatesOnePerReportPerHourOverHTTP(t *testing.T) {
	srv, pool, a, b, c, d := p32uSetup(t, "p32-hour")

	id := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	p32uLive(t, srv, b, c, d, id)
	first, _ := p32uPoll(t, "B Live", b, srv.URL)
	if len(first.Updates) != 1 || first.Updates[0].Kind != "reached_live" {
		t.Fatalf("after Live = %+v, want one reached_live row", first)
	}

	// C and D switch to dispute: 1 confirming place against 2 disputing ones
	// hides the report, inside the hour.
	p32uVote(t, c, srv.URL, id, "dispute", 1)
	p32uVote(t, d, srv.URL, id, "dispute", 2)
	for i := 0; i < 2; i++ {
		got, _ := p32uPoll(t, "B inside the hour", b, srv.URL)
		if len(got.Updates) != 1 || got.Updates[0].ID != first.Updates[0].ID {
			t.Fatalf("inside the hour, poll %d = %+v, want only the first row", i, got)
		}
	}

	// An hour later the change is announced, once.
	p32uAgeCursors(t, pool, id)
	for i := 0; i < 2; i++ {
		got, _ := p32uPoll(t, "B after the hour", b, srv.URL)
		if len(got.Updates) != 2 {
			t.Fatalf("after the hour, poll %d = %d rows, want 2 (%+v)", i, len(got.Updates), got)
		}
		if got.Updates[0].Kind != "hidden" || got.Updates[0].Event != "Now disputed" {
			t.Errorf("newest row = %+v, want hidden / Now disputed", got.Updates[0])
		}
		if got.Updates[1].Kind != "reached_live" {
			t.Errorf("older row = %+v, want reached_live", got.Updates[1])
		}
	}

	// A's own resolve gives A no update about it (D-13): A baselines the new
	// report first, then resolves it, then polls.
	own := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	// A has not polled yet, so A's first poll baselines both reports silently.
	if w, _ := p32uPoll(t, "A baseline", a, srv.URL); len(w.Updates) != 0 {
		t.Fatalf("A baseline = %+v, want an empty list", w)
	}
	status, body, err := p07PostVoteRaw(a, srv.URL, own, "resolve", p20Lat, p20Lon)
	if err != nil || status != http.StatusOK {
		t.Fatalf("A resolve: status %d, err %v, body %s", status, err, body)
	}
	if w, _ := p32uPoll(t, "A after own resolve", a, srv.URL); len(w.Updates) != 0 || w.Unread != 0 {
		t.Fatalf("A after own resolve = %+v, want nothing (D-13)", w)
	}
}

// TestMarkReadScopedToAccount: B's read mark clears B's count and nobody else's,
// even with an id large enough to cover another account's row.
func TestMarkReadScopedToAccount(t *testing.T) {
	srv, _, a, b, c, d := p32uSetup(t, "p32-read")

	id := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	if w, _ := p32uPoll(t, "A baseline", a, srv.URL); len(w.Updates) != 0 {
		t.Fatalf("A baseline = %+v", w)
	}
	p32uLive(t, srv, b, c, d, id)

	aWire, _ := p32uPoll(t, "A Live", a, srv.URL)
	bWire, _ := p32uPoll(t, "B Live", b, srv.URL)
	if len(aWire.Updates) != 1 || len(bWire.Updates) != 1 || aWire.Unread != 1 || bWire.Unread != 1 {
		t.Fatalf("A = %+v, B = %+v, want one unread row each", aWire, bWire)
	}
	idA, idB := aWire.Updates[0].ID, bWire.Updates[0].ID
	top := max(idA, idB)

	// Bad bodies first.
	for _, tc := range []struct {
		name, body, field string
	}{
		{"zero", `{"up_to":0}`, "up_to"},
		{"negative", `{"up_to":-5}`, "up_to"},
		{"unknown field", `{"up_to":1,"extra":1}`, "body"},
		{"not a number", `{"up_to":"7"}`, "body"},
		{"empty body", ``, "body"},
	} {
		resp := p32uPost(t, b, srv.URL+"/api/updates/read", tc.body)
		field, _ := decodeErrorResponse(t, resp)
		if resp.StatusCode != http.StatusBadRequest || field != tc.field {
			t.Errorf("%s: status %d field %q, want 400 and %q", tc.name, resp.StatusCode, field, tc.field)
		}
	}
	if w, _ := p32uPoll(t, "B after bad bodies", b, srv.URL); w.Unread != 1 {
		t.Fatalf("a refused body changed B's count: %+v", w)
	}

	resp := p32uPost(t, b, srv.URL+"/api/updates/read", `{"up_to":`+strconv.FormatInt(top, 10)+`}`)
	if raw := p32uStatus(t, "B read mark", resp, http.StatusNoContent); len(raw) != 0 {
		t.Errorf("a 204 carries a body: %s", raw)
	}

	bAfter, _ := p32uPoll(t, "B after read", b, srv.URL)
	if bAfter.Unread != 0 || len(bAfter.Updates) != 1 || bAfter.Updates[0].Unread {
		t.Errorf("B after read = %+v, want unread 0 and the row kept as read", bAfter)
	}
	if !strings.HasPrefix(bAfter.Updates[0].Label, "Now confirmed.") {
		t.Errorf("a read row label = %q, want no New. prefix", bAfter.Updates[0].Label)
	}
	aAfter, _ := p32uPoll(t, "A after B's read", a, srv.URL)
	if aAfter.Unread != 1 || len(aAfter.Updates) != 1 || !aAfter.Updates[0].Unread {
		t.Errorf("A after B's read = %+v, want A untouched", aAfter)
	}
}

// TestMuteRoutes: mute and unmute act for the caller only, answer 204, are
// idempotent, drop the unread rows of that report, keep a muted report silent
// even after the hour, and unmute baselines silently.
func TestMuteRoutes(t *testing.T) {
	srv, pool, a, b, c, d := p32uSetup(t, "p32-mute")

	id := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	p32uLive(t, srv, b, c, d, id)
	if w, _ := p32uPoll(t, "B Live", b, srv.URL); len(w.Updates) != 1 || w.Unread != 1 {
		t.Fatalf("B Live = %+v, want one unread row", w)
	}

	muteURL := srv.URL + "/api/reports/" + strconv.FormatInt(id, 10) + "/mute"
	unmuteURL := srv.URL + "/api/reports/" + strconv.FormatInt(id, 10) + "/unmute"
	for i := 0; i < 2; i++ {
		resp := p32uPost(t, b, muteURL, "")
		if raw := p32uStatus(t, "B mute", resp, http.StatusNoContent); len(raw) != 0 {
			t.Errorf("mute 204 carries a body: %s", raw)
		}
	}
	if w, _ := p32uPoll(t, "B muted", b, srv.URL); len(w.Updates) != 0 || w.Unread != 0 {
		t.Fatalf("B after mute = %+v, want the unread row gone", w)
	}

	// The wire says muted for B and not for C.
	objects, _ := p32ViewerFeed(t, "B", b, srv.URL, id)
	if !p32Bool(t, "B feed", objects[id], "viewer_muted") {
		t.Errorf("B's feed says viewer_muted false after a mute")
	}
	objectsC, _ := p32ViewerFeed(t, "C", c, srv.URL, id)
	if p32Bool(t, "C feed", objectsC[id], "viewer_muted") {
		t.Errorf("C's feed says viewer_muted true, B's mute must not reach C")
	}

	// The report is disputed, an hour passes, and B hears nothing.
	p32uVote(t, c, srv.URL, id, "dispute", 1)
	p32uVote(t, d, srv.URL, id, "dispute", 2)
	p32uAgeCursors(t, pool, id)
	if w, _ := p32uPoll(t, "B muted after the hour", b, srv.URL); len(w.Updates) != 0 || w.Unread != 0 {
		t.Fatalf("B muted after the hour = %+v, want nothing", w)
	}

	// Unmute twice, then the next poll baselines silently at the current state.
	for i := 0; i < 2; i++ {
		resp := p32uPost(t, b, unmuteURL, "")
		p32uStatus(t, "B unmute", resp, http.StatusNoContent)
	}
	if w, _ := p32uPoll(t, "B unmuted", b, srv.URL); len(w.Updates) != 0 || w.Unread != 0 {
		t.Fatalf("B after unmute = %+v, want a silent baseline", w)
	}
	var state string
	if err := pool.QueryRow(context.Background(), `SELECT last_state FROM update_cursors WHERE report_id = $1 AND account_id = (SELECT id FROM accounts WHERE email = $2)`, id, "p32-mute-b@example.com").Scan(&state); err != nil {
		t.Fatalf("reading B's cursor: %v", err)
	}
	if state != "hidden" {
		t.Errorf("B's baseline state = %q, want hidden", state)
	}
	objects, _ = p32ViewerFeed(t, "B again", b, srv.URL, id)
	if p32Bool(t, "B feed again", objects[id], "viewer_muted") {
		t.Errorf("viewer_muted still true after unmute")
	}

	// A report that is not there reads 404 for mute; unmute never refuses.
	removed := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	held := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))
	p20SetModeration(t, pool, removed, "removed")
	p20SetModeration(t, pool, held, "pending")
	for name, target := range map[string]string{
		"unknown": "999999",
		"removed": strconv.FormatInt(removed, 10),
		"held":    strconv.FormatInt(held, 10),
	} {
		resp := p32uPost(t, b, srv.URL+"/api/reports/"+target+"/mute", "")
		field, message := decodeErrorResponse(t, resp)
		if resp.StatusCode != http.StatusNotFound || field != "id" || message != "This report is no longer available." {
			t.Errorf("mute %s: status %d, error (%q, %q), want 404 id", name, resp.StatusCode, field, message)
		}
	}
	resp := p32uPost(t, b, srv.URL+"/api/reports/"+strconv.FormatInt(removed, 10)+"/unmute", "")
	p32uStatus(t, "unmute a removed report", resp, http.StatusNoContent)

	// A bad id and a bad body are 400.
	resp = p32uPost(t, b, srv.URL+"/api/reports/abc/mute", "")
	if field, _ := decodeErrorResponse(t, resp); resp.StatusCode != http.StatusBadRequest || field != "id" {
		t.Errorf("mute abc: status %d field %q, want 400 id", resp.StatusCode, field)
	}
	resp = p32uPost(t, b, muteURL, `{"x":1}`)
	if field, _ := decodeErrorResponse(t, resp); resp.StatusCode != http.StatusBadRequest || field != "body" {
		t.Errorf("mute with a body: status %d field %q, want 400 body", resp.StatusCode, field)
	}
}

// TestMutedReportsRoute: the Settings list shows the caller's mutes with words,
// shows an unavailable row with no detail but its id, and is empty as [].
func TestMutedReportsRoute(t *testing.T) {
	srv, pool, a, b, c, _ := p32uSetup(t, "p32-list")

	listMuted := func(label string, client *http.Client) (rows []struct {
		ReportID      int64  `json:"report_id"`
		Available     bool   `json:"available"`
		Text          string `json:"text"`
		Category      string `json:"category"`
		Severity      string `json:"severity"`
		SeverityLevel string `json:"severity_level"`
	}, raw []byte) {
		t.Helper()
		resp, err := client.Get(srv.URL + "/api/me/muted-reports")
		if err != nil {
			t.Fatalf("%s: GET muted-reports: %v", label, err)
		}
		raw = p20Status(t, label, resp, http.StatusOK)
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("%s: Cache-Control = %q, want no-store", label, cc)
		}
		var out struct {
			Reports []struct {
				ReportID      int64  `json:"report_id"`
				Available     bool   `json:"available"`
				Text          string `json:"text"`
				Category      string `json:"category"`
				Severity      string `json:"severity"`
				SeverityLevel string `json:"severity_level"`
			} `json:"reports"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: decoding %s: %v", label, raw, err)
		}
		return out.Reports, raw
	}

	id := submitReportGetID(t, a, srv.URL, p20Body("flood", "low"))

	if rows, raw := listMuted("B empty", b); len(rows) != 0 || !bytes.Contains(raw, []byte(`"reports":[]`)) {
		t.Fatalf("B empty list = %s, want an empty array", raw)
	}

	resp := p32uPost(t, b, srv.URL+"/api/reports/"+strconv.FormatInt(id, 10)+"/mute", "")
	p32uStatus(t, "B mute", resp, http.StatusNoContent)

	rows, _ := listMuted("B muted", b)
	if len(rows) != 1 || rows[0].ReportID != id || !rows[0].Available || rows[0].Text != p20Desc ||
		rows[0].Category != "Flood" || rows[0].Severity != "Low" || rows[0].SeverityLevel != "low" {
		t.Fatalf("B muted list = %+v", rows)
	}
	if other, _ := listMuted("C other", c); len(other) != 0 {
		t.Errorf("C sees B's mute: %+v", other)
	}

	// The report is removed after review: still listed, no detail, id kept.
	p20SetModeration(t, pool, id, "removed")
	rows, raw := listMuted("B removed", b)
	if len(rows) != 1 || rows[0].ReportID != id || rows[0].Available || rows[0].Text != "" || rows[0].Category != "" || rows[0].Severity != "" || rows[0].SeverityLevel != "" {
		t.Fatalf("B removed list = %s, want available false and no detail", raw)
	}
	if bytes.Contains(raw, []byte(p20Desc)) {
		t.Errorf("an unavailable muted row leaks the description: %s", raw)
	}

	resp = p32uPost(t, b, srv.URL+"/api/reports/"+strconv.FormatInt(id, 10)+"/unmute", "")
	p32uStatus(t, "B unmute removed", resp, http.StatusNoContent)
	if rows, _ := listMuted("B cleared", b); len(rows) != 0 {
		t.Errorf("list after unmute = %+v, want empty", rows)
	}
}

// TestUpdatesRoutesNeedAnAccount: each of the five routes answers 401 JSON to a
// session that is not verified.
func TestUpdatesRoutesNeedAnAccount(t *testing.T) {
	srv, _, _ := newE2EServerWith(t, p20Tweak(newP20Clock(), nil))
	anon := newJarClient(t)
	resp, err := anon.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()

	cases := []struct{ method, path, body string }{
		{http.MethodGet, "/api/updates", ""},
		{http.MethodPost, "/api/updates/read", `{"up_to":1}`},
		{http.MethodPost, "/api/reports/1/mute", ""},
		{http.MethodPost, "/api/reports/1/unmute", ""},
		{http.MethodGet, "/api/me/muted-reports", ""},
	}
	for _, tc := range cases {
		var rd io.Reader
		if tc.body != "" {
			rd = strings.NewReader(tc.body)
		}
		req, err := http.NewRequest(tc.method, srv.URL+tc.path, rd)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := anon.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.method, tc.path, err)
		}
		if resp.StatusCode != http.StatusUnauthorized {
			resp.Body.Close()
			t.Errorf("%s %s: status %d, want 401", tc.method, tc.path, resp.StatusCode)
			continue
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("%s %s: Content-Type %q, want JSON", tc.method, tc.path, ct)
		}
		resp.Body.Close()
	}
}
