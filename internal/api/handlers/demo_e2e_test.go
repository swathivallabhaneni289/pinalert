// demo_e2e_test.go proves the Demo API end to end over the real router and a
// real Postgres (plan 05-10): a whole Demo run writes nothing to any table and
// sets no cookie, the Demo answers in the same JSON shapes as the real routes,
// and the Demo keeps the real vote rules and refusals. The Demo store is created
// in the test (there is no page that starts a visit yet), and its clock is moved
// by hand so the pretend neighbours' votes fall due without waiting.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/demo"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// demoClock is the Demo store's clock: the wall clock plus an offset the test
// moves.
type demoClock struct{ off atomic.Int64 }

func (c *demoClock) Now() time.Time          { return time.Now().Add(time.Duration(c.off.Load())) }
func (c *demoClock) Advance(d time.Duration) { c.off.Add(int64(d)) }

// demoE2E is one real server with one Demo visit on it.
type demoE2E struct {
	t      *testing.T
	srv    *httptest.Server
	mailer *recordingMailer
	pool   *pgxpool.Pool
	clock  *demoClock
	visit  string
	client *http.Client // no cookie jar: a Demo visitor has no session
}

func newDemoE2E(t *testing.T) *demoE2E {
	t.Helper()
	clock := &demoClock{}
	store := demo.NewStore(demo.WithStoreClock(clock.Now), demo.WithWorldSetup(demo.SeedSetup))
	w, err := store.Create()
	if err != nil {
		t.Fatalf("creating a Demo world: %v", err)
	}
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.Demo = store
	})
	return &demoE2E{t: t, srv: srv, mailer: mailer, pool: pool, clock: clock, visit: w.ID, client: &http.Client{}}
}

// do sends one Demo request and returns the status and body. It fails the test
// when the answer sets a cookie.
func (d *demoE2E) do(method, path string, body any) (int, []byte) {
	d.t.Helper()
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			d.t.Fatalf("marshalling the body: %v", err)
		}
		rd = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, d.srv.URL+"/demo/api/v/"+d.visit+path, rd)
	if err != nil {
		d.t.Fatalf("building the request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := d.client.Do(req)
	if err != nil {
		d.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
		d.t.Fatalf("%s %s set a cookie: %v", method, path, got)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		d.t.Fatalf("reading the answer: %v", err)
	}
	return resp.StatusCode, raw
}

const demoFeedQuery = "/reports?lat=16.5424&lon=80.6386&radius_km=10"

// feed reads the active view at the pretend position and requires 200.
func (d *demoE2E) feed() ([]map[string]any, map[string]any) {
	d.t.Helper()
	status, raw := d.do(http.MethodGet, demoFeedQuery, nil)
	if status != http.StatusOK {
		d.t.Fatalf("GET Demo reports = %d, want 200: %s", status, raw)
	}
	var whole map[string]any
	if err := json.Unmarshal(raw, &whole); err != nil {
		d.t.Fatalf("decoding the Demo feed: %v (%s)", err, raw)
	}
	list, _ := whole["reports"].([]any)
	items := make([]map[string]any, 0, len(list))
	for _, v := range list {
		items = append(items, v.(map[string]any))
	}
	return items, whole
}

// vote sends one vote on the Demo with the pretend position as the body.
func (d *demoE2E) vote(reportID int64, action string) (int, []byte) {
	d.t.Helper()
	return d.do(http.MethodPost, "/reports/"+strconv.FormatInt(reportID, 10)+"/"+action,
		map[string]float64{"latitude": demo.PretendLat, "longitude": demo.PretendLon})
}

// postPractice posts a practice report at the pretend position and returns its
// id and the raw answer.
func (d *demoE2E) postPractice() (int64, []byte) {
	d.t.Helper()
	status, raw := d.do(http.MethodPost, "/reports", map[string]any{
		"category":    "flood",
		"severity":    "low",
		"description": "Water rising near the bridge",
		"latitude":    demo.PretendLat,
		"longitude":   demo.PretendLon,
	})
	if status != http.StatusCreated {
		d.t.Fatalf("POST Demo practice report = %d, want 201: %s", status, raw)
	}
	var out struct {
		Report struct {
			ID int64 `json:"id"`
		} `json:"report"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Report.ID == 0 {
		d.t.Fatalf("decoding the practice report answer %s: %v", raw, err)
	}
	return out.Report.ID, raw
}

// itemByID finds a feed item by id.
func itemByID(items []map[string]any, id int64) map[string]any {
	for _, it := range items {
		if n, ok := it["id"].(float64); ok && int64(n) == id {
			return it
		}
	}
	return nil
}

// nearSeedReports are the feed items that are not the visitor's own and lie
// within 900 metres of the pretend position, so a vote from it is inside the 1 km
// rule.
func nearSeedReports(items []map[string]any) []map[string]any {
	var out []map[string]any
	for _, it := range items {
		dist, ok := it["distance_km"].(float64)
		own, _ := it["is_own_report"].(bool)
		if ok && dist <= 0.9 && !own {
			out = append(out, it)
		}
	}
	return out
}

func reportIDOf(it map[string]any) int64 { return int64(it["id"].(float64)) }

// tableCounts counts the rows of every table in the public schema. The table
// list is read from pg_tables, so a table added later is covered automatically.
func tableCounts(t *testing.T, pool *pgxpool.Pool) map[string]int {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		t.Fatalf("listing the tables: %v", err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scanning a table name: %v", err)
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the table names: %v", err)
	}
	counts := make(map[string]int, len(names))
	for _, n := range names {
		var c int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM "`+n+`"`).Scan(&c); err != nil {
			t.Fatalf("counting %s: %v", n, err)
		}
		counts[n] = c
	}
	return counts
}

func TestDemoWritesNothing(t *testing.T) {
	d := newDemoE2E(t)
	before := tableCounts(t, d.pool)
	for _, must := range []string{"sessions", "reports", "votes", "accounts"} {
		if _, ok := before[must]; !ok {
			t.Fatalf("the table list %v lacks %s: the count would prove nothing", before, must)
		}
	}

	items, _ := d.feed()
	if len(items) == 0 {
		t.Fatal("the Demo feed is empty: the seed did not run")
	}
	practiceID, _ := d.postPractice()

	near := nearSeedReports(items)
	if len(near) < 2 {
		t.Fatalf("only %d seed reports lie within 900 m of the pretend position, want at least 2", len(near))
	}
	if status, raw := d.vote(reportIDOf(near[0]), "confirm"); status != http.StatusOK {
		t.Fatalf("Demo confirm = %d, want 200: %s", status, raw)
	}
	if status, raw := d.vote(reportIDOf(near[1]), "dispute"); status != http.StatusOK {
		t.Fatalf("Demo dispute = %d, want 200: %s", status, raw)
	}

	// The pretend neighbours fall due; the next read casts their votes through the
	// real CastVote, in memory.
	d.clock.Advance(20 * time.Second)
	after, _ := d.feed()
	mine := itemByID(after, practiceID)
	if mine == nil {
		t.Fatalf("the practice report %d is not in the Demo feed", practiceID)
	}
	trust, _ := mine["trust"].(map[string]any)
	places, _ := trust["confirmed_places"].(float64)
	if int(places) < service.IndependentAgreementThreshold {
		t.Fatalf("practice report confirmed_places = %v after 20 s, want at least %d: the neighbours did not vote", places, service.IndependentAgreementThreshold)
	}
	if confirmed, _ := mine["confirmed_by_people"].(bool); !confirmed {
		t.Fatalf("practice report confirmed_by_people = false after the neighbours confirmed it")
	}

	got := tableCounts(t, d.pool)
	if !reflect.DeepEqual(before, got) {
		t.Fatalf("a Demo run changed the table row counts\nbefore: %v\nafter:  %v", before, got)
	}
}

// demoShape reduces decoded JSON to its shape: an object keeps its keys, an
// array is the shape of its first element, a null is "null" and every other
// value is "value". Two answers have the same shape when their keys agree at
// every depth.
func demoShape(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = demoShape(val)
		}
		return out
	case []any:
		if len(x) == 0 {
			return []any{}
		}
		return []any{demoShape(x[0])}
	case nil:
		return "null"
	default:
		return "value"
	}
}

func mustDecodeAny(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	return out
}

func TestDemoAPIShapesMatchReal(t *testing.T) {
	d := newDemoE2E(t)

	// The real side: one account posts, another confirms from a different spot
	// inside 1 km.
	reporter := newVerifiedClient(t, d.srv, d.mailer, "shape-reporter@example.com")
	voter := newVerifiedClient(t, d.srv, d.mailer, "shape-voter@example.com")
	realPost := postReport(t, reporter, d.srv.URL, map[string]any{
		"category":    "flood",
		"severity":    "low",
		"description": "Water rising near the bridge",
		"latitude":    demo.PretendLat,
		"longitude":   demo.PretendLon,
	})
	realPostRaw, _ := io.ReadAll(realPost.Body)
	realPost.Body.Close()
	if realPost.StatusCode != http.StatusCreated {
		t.Fatalf("real POST /api/reports = %d: %s", realPost.StatusCode, realPostRaw)
	}
	realID := int64(mustDecodeAny(t, realPostRaw)["report"].(map[string]any)["id"].(float64))

	realFeed, err := reporter.Get(d.srv.URL + "/api/reports?lat=16.5424&lon=80.6386&radius_km=10")
	if err != nil {
		t.Fatalf("real GET /api/reports: %v", err)
	}
	realFeedRaw, _ := io.ReadAll(realFeed.Body)
	realFeed.Body.Close()
	if realFeed.StatusCode != http.StatusOK {
		t.Fatalf("real GET /api/reports = %d: %s", realFeed.StatusCode, realFeedRaw)
	}
	realWhole := mustDecodeAny(t, realFeedRaw)
	var realItems []map[string]any
	for _, v := range realWhole["reports"].([]any) {
		realItems = append(realItems, v.(map[string]any))
	}
	// The real reporter's own fresh report, as the real reporter sees it, to
	// compare with the Demo visitor's own fresh report. Both are read before any
	// vote, so the same states are compared.
	realItem := itemByID(realItems, realID)
	if realItem == nil {
		t.Fatalf("the real report %d is not in the real feed", realID)
	}

	realVote := postVote(t, voter, d.srv.URL, realID, "confirm", map[string]float64{"latitude": demo.PretendLat + 0.002, "longitude": demo.PretendLon})
	realVoteRaw, _ := io.ReadAll(realVote.Body)
	realVote.Body.Close()
	if realVote.StatusCode != http.StatusOK {
		t.Fatalf("real confirm = %d: %s", realVote.StatusCode, realVoteRaw)
	}

	// The Demo side.
	practiceID, practiceRaw := d.postPractice()
	demoItems, demoWhole := d.feed()
	demoItem := itemByID(demoItems, practiceID)
	if demoItem == nil {
		t.Fatalf("the practice report %d is not in the Demo feed", practiceID)
	}
	near := nearSeedReports(demoItems)
	if len(near) == 0 {
		t.Fatal("no seed report within 900 m to confirm")
	}
	status, demoVoteRaw := d.vote(reportIDOf(near[0]), "confirm")
	if status != http.StatusOK {
		t.Fatalf("Demo confirm = %d: %s", status, demoVoteRaw)
	}

	check := func(name string, real, demoV any) {
		t.Helper()
		rs, ds := demoShape(real), demoShape(demoV)
		if !reflect.DeepEqual(rs, ds) {
			rj, _ := json.Marshal(rs)
			dj, _ := json.Marshal(ds)
			t.Errorf("%s: the Demo answer has a different shape from the real one\nreal: %s\ndemo: %s", name, rj, dj)
		}
	}
	check("GET reports item", realItem, demoItem)
	check("POST reports answer", mustDecodeAny(t, realPostRaw), mustDecodeAny(t, practiceRaw))
	check("confirm answer", mustDecodeAny(t, realVoteRaw), mustDecodeAny(t, demoVoteRaw))

	// The list envelope, apart from the reports array whose elements are compared
	// above.
	realEnv, demoEnv := map[string]any{}, map[string]any{}
	for k, v := range realWhole {
		if k != "reports" {
			realEnv[k] = v
		}
	}
	for k, v := range demoWhole {
		if k != "reports" {
			demoEnv[k] = v
		}
	}
	check("GET reports envelope", realEnv, demoEnv)
}

func TestDemoRulesAreReal(t *testing.T) {
	d := newDemoE2E(t)
	items, _ := d.feed()

	// Too far: a seed report beyond 1 km of the pretend position. The Ended seed
	// report is not used: the service checks expiry before distance.
	var far map[string]any
	for _, it := range items {
		dist, ok := it["distance_km"].(float64)
		own, _ := it["is_own_report"].(bool)
		if ok && dist > 1.1 && dist < 4 && !own {
			far = it
			break
		}
	}
	if far == nil {
		t.Fatal("no seed report 1.1 to 4 km from the pretend position")
	}
	status, raw := d.vote(reportIDOf(far), "confirm")
	if status != http.StatusForbidden {
		t.Fatalf("confirm from beyond 1 km = %d, want 403: %s", status, raw)
	}
	var e demoErrBody
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	if e.Error.Field != "location" || e.Error.Message != service.TooFarMessage() {
		t.Fatalf("too far error = %+v, want field location and %q", e.Error, service.TooFarMessage())
	}

	// No self vote.
	practiceID, _ := d.postPractice()
	for _, action := range []string{"confirm", "dispute"} {
		status, raw = d.vote(practiceID, action)
		if status != http.StatusForbidden {
			t.Fatalf("%s on the visitor's own report = %d, want 403: %s", action, status, raw)
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("decoding %s: %v", raw, err)
		}
		if e.Error.Field != "account" || e.Error.Message != "You can't vote on your own report." {
			t.Fatalf("self vote error = %+v, want field account and the self vote sentence", e.Error)
		}
	}

	// The two undo limit: resolve then take it back, twice, and the third take
	// back is refused.
	for round := 1; round <= 2; round++ {
		if status, raw = d.vote(practiceID, "resolve"); status != http.StatusOK {
			t.Fatalf("resolve round %d = %d, want 200: %s", round, status, raw)
		}
		if status, raw = d.vote(practiceID, "unresolve"); status != http.StatusOK {
			t.Fatalf("unresolve round %d = %d, want 200: %s", round, status, raw)
		}
	}
	if status, raw = d.vote(practiceID, "resolve"); status != http.StatusOK {
		t.Fatalf("third resolve = %d, want 200: %s", status, raw)
	}
	status, raw = d.vote(practiceID, "unresolve")
	if status != http.StatusConflict {
		t.Fatalf("third undo = %d, want 409: %s", status, raw)
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	if e.Error.Message != "You have used both undos on this report." {
		t.Fatalf("third undo message = %q, want the undo limit sentence", e.Error.Message)
	}

	// A report that is not there is the real 404.
	if status, raw = d.vote(999999, "confirm"); status != http.StatusNotFound {
		t.Fatalf("vote on a missing report = %d, want 404: %s", status, raw)
	}
}

// demoStartVisit opens GET /demo on a server and returns the visit it started.
func demoStartVisit(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	resp, body := demoPageGet(t, client, base+"/demo")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /demo = %d, want 200", resp.StatusCode)
	}
	m := demoAPIBaseRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("GET /demo has no data-api-base: %.200s", body)
	}
	return strings.TrimPrefix(m[1], "/demo/api/v/")
}

// demoPostJSON sends one JSON body to a Demo path and returns the status and body.
func demoPostJSON(t *testing.T, client *http.Client, target string, body any) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshalling the body: %v", err)
	}
	resp, err := client.Post(target, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", target, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

var (
	demoPartRe   = regexp.MustCompile(`(?s)<section class="activity-part activity-part--([a-z]+)".*?</section>`)
	demoReportRe = regexp.MustCompile(`data-report-id="(\d+)"`)
)

// demoPartsOf splits a My activity page into its part sections by key.
func demoPartsOf(page string) map[string]string {
	out := map[string]string{}
	for _, m := range demoPartRe.FindAllStringSubmatch(page, -1) {
		out[m[1]] = m[0]
	}
	return out
}

// TestDemoActivityPageParts (plan 05-19, D-05, UI-SPEC 4c): a fresh visit shows
// the four parts with their empty states and no Updates part, no queue strip and
// no reporter record. After a practice post, a confirm and a dispute in the same
// visit the cards are in the right parts, each with the Demo tag, and the part
// the address names is the one marked current. A part word the Demo has no part
// for (updates) reads as the list. The page sets no cookie and needs no account.
func TestDemoActivityPageParts(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{})
	client := &http.Client{}
	visit := demoStartVisit(t, client, srv.URL)
	api := srv.URL + "/demo/api/v/" + visit
	get := func(query string) string {
		t.Helper()
		resp, body := demoPageGet(t, client, srv.URL+"/demo/activity?v="+visit+query)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /demo/activity%s = %d, want 200: %.200s", query, resp.StatusCode, body)
		}
		if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
			t.Fatalf("GET /demo/activity set a cookie: %v", got)
		}
		return body
	}

	fresh := get("")
	parts := demoPartsOf(fresh)
	if len(parts) != 4 {
		t.Fatalf("the page has %d parts, want 4 (%v)", len(parts), parts)
	}
	for key, empty := range map[string]string{
		"posted": "No reports yet", "confirmed": "No confirmations yet",
		"disputed": "No disputes yet", "ended": "No ended reports nearby",
	} {
		if !strings.Contains(parts[key], empty) {
			t.Errorf("part %s lacks its empty state %q", key, empty)
		}
	}
	if order := []string{"Posted by you", "You confirmed", "You disputed", "Ended nearby"}; true {
		last := -1
		for _, name := range order {
			at := strings.Index(fresh, name)
			if at <= last {
				t.Errorf("part name %q is missing or out of order", name)
			}
			last = at
		}
	}
	for _, banned := range []string{"Updates", "queue-strip", "reporter record", "data-account-id", "bell-trigger", "tel:"} {
		if strings.Contains(fresh, banned) {
			t.Errorf("the fresh activity page must not contain %q", banned)
		}
	}
	if n := strings.Count(fresh, `class="report-row activity-card`); n != 0 {
		t.Errorf("a fresh visit shows %d cards, want 0", n)
	}

	// Do something in this visit: post, confirm one near report, dispute another.
	resp, body := demoPageGet(t, client, api+demoFeedQuery)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET Demo feed = %d: %.200s", resp.StatusCode, body)
	}
	var feed struct {
		Reports []map[string]any `json:"reports"`
	}
	if err := json.Unmarshal([]byte(body), &feed); err != nil {
		t.Fatalf("decoding the feed: %v", err)
	}
	near := nearSeedReports(feed.Reports)
	if len(near) < 2 {
		t.Fatalf("only %d near seed reports, want at least 2", len(near))
	}
	status, raw := demoPostJSON(t, client, api+"/reports", map[string]any{
		"category": "flood", "severity": "low", "description": "Water rising near the bridge",
		"latitude": demo.PretendLat, "longitude": demo.PretendLon,
	})
	if status != http.StatusCreated {
		t.Fatalf("practice post = %d: %s", status, raw)
	}
	var posted struct {
		Report struct {
			ID int64 `json:"id"`
		} `json:"report"`
	}
	if err := json.Unmarshal(raw, &posted); err != nil {
		t.Fatalf("decoding the practice post: %v", err)
	}
	pos := map[string]float64{"latitude": demo.PretendLat, "longitude": demo.PretendLon}
	confirmID, disputeID := reportIDOf(near[0]), reportIDOf(near[1])
	if status, raw = demoPostJSON(t, client, api+"/reports/"+strconv.FormatInt(confirmID, 10)+"/confirm", pos); status != http.StatusOK {
		t.Fatalf("confirm = %d: %s", status, raw)
	}
	if status, raw = demoPostJSON(t, client, api+"/reports/"+strconv.FormatInt(disputeID, 10)+"/dispute", pos); status != http.StatusOK {
		t.Fatalf("dispute = %d: %s", status, raw)
	}

	busy := get("")
	parts = demoPartsOf(busy)
	want := map[string]int64{"posted": posted.Report.ID, "confirmed": confirmID, "disputed": disputeID}
	for key, id := range want {
		section := parts[key]
		ids := demoReportRe.FindAllStringSubmatch(section, -1)
		if len(ids) != 1 || ids[0][1] != strconv.FormatInt(id, 10) {
			t.Errorf("part %s holds %v, want exactly report %d", key, ids, id)
		}
		if !strings.Contains(section, `class="demo-tag"`) {
			t.Errorf("part %s: the card has no Demo tag", key)
		}
		if strings.Contains(section, "No reports yet") || strings.Contains(section, "No confirmations yet") || strings.Contains(section, "No disputes yet") {
			t.Errorf("part %s still shows an empty state beside its card", key)
		}
	}
	if !strings.Contains(parts["posted"], "Posted just now") || !strings.Contains(parts["confirmed"], "You confirmed just now") ||
		!strings.Contains(parts["disputed"], "You disputed just now") {
		t.Errorf("the When cells do not read from the times of this visit: %q %q %q", parts["posted"], parts["confirmed"], parts["disputed"])
	}
	if !strings.Contains(busy, "1 still happening, 0 resolved") {
		t.Errorf("the Posted hub line does not count the practice report")
	}

	// The part in the address is the current one; updates is not a Demo part.
	if got := get("&part=confirmed"); !strings.Contains(got, `data-part="confirmed"`) {
		t.Errorf("part=confirmed is not the current part")
	}
	for _, query := range []string{"&part=updates", "&part=nonsense", "&tab=votes"} {
		if got := get(query); !strings.Contains(got, `data-part=""`) {
			t.Errorf("%s should read as the list (data-part is empty)", query)
		}
	}
}

// TestDemoOfficialAlertsAPI (plan 05-19, D-05, ROBUST-06): the Demo's alerts
// answer has the real route's shape, built from the sample alerts through the
// real view. Every row is a Demo row with no badge, the banner carries the Demo
// flag, the position in the query changes nothing (the reader is the pretend
// position), and an unknown, malformed or ended visit is the Demo's 404 with
// field visit.
func TestDemoOfficialAlertsAPI(t *testing.T) {
	d := newDemoE2E(t)
	read := func(query string) map[string]any {
		t.Helper()
		status, raw := d.do(http.MethodGet, "/official/alerts"+query, nil)
		if status != http.StatusOK {
			t.Fatalf("GET Demo official alerts%s = %d, want 200: %s", query, status, raw)
		}
		return mustDecodeAny(t, raw)
	}
	whole := read("?lat=16.5424&lon=80.6386")
	if whole["state"] != "fresh" || whole["area_known"] != true {
		t.Fatalf("state %v area_known %v, want fresh and true", whole["state"], whole["area_known"])
	}
	groups, _ := whole["groups"].([]any)
	if len(groups) != 2 || whole["count"] != float64(3) {
		t.Fatalf("groups %d count %v, want 2 groups and 3 rows", len(groups), whole["count"])
	}
	for _, g := range groups {
		for _, r := range g.(map[string]any)["rows"].([]any) {
			row := r.(map[string]any)
			if row["demo"] != true || row["badge"] != "" {
				t.Errorf("row %v: demo %v badge %q, want a Demo row with no badge", row["id"], row["demo"], row["badge"])
			}
		}
	}
	banner, _ := whole["banner"].(map[string]any)
	if banner == nil || banner["demo"] != true || banner["level"] != "extreme" {
		t.Fatalf("banner = %v, want the Extreme sample with demo true", whole["banner"])
	}

	// The reader is the pretend position whatever the query says, or says
	// nothing. The times move with the clock, so the shapes and the group
	// labels are compared, not the whole answer.
	for _, query := range []string{"", "?lat=28.6&lon=77.2"} {
		other := read(query)
		if !reflect.DeepEqual(demoShape(whole), demoShape(other)) {
			t.Errorf("the answer for %q has another shape", query)
		}
		if other["districts_line"] != whole["districts_line"] || other["count"] != whole["count"] {
			t.Errorf("the answer for %q covers %v (%v rows), want %v (%v rows): the reader must be the pretend position",
				query, other["districts_line"], other["count"], whole["districts_line"], whole["count"])
		}
	}

	for _, visit := range []string{"nosuchvisit0123456789ab", "short", "..%2f..%2fetc"} {
		req, err := http.NewRequest(http.MethodGet, d.srv.URL+"/demo/api/v/"+visit+"/official/alerts", nil)
		if err != nil {
			t.Fatalf("building the request: %v", err)
		}
		resp, err := d.client.Do(req)
		if err != nil {
			t.Fatalf("GET official alerts for %q: %v", visit, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("visit %q = %d, want 404: %s", visit, resp.StatusCode, raw)
			continue
		}
		var e struct {
			Error struct {
				Field string `json:"field"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &e); err != nil || e.Error.Field != "visit" {
			t.Errorf("visit %q answered %s, want the error with field visit", visit, raw)
		}
	}
}
