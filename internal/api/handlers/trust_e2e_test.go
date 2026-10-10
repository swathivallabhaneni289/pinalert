// trust_e2e_test.go proves D-21 and D-04 end to end over the real router and a
// real Postgres, reusing the existing e2e helpers (newVerifiedClient,
// submitReportGetID, floodReportBody, countVotesForReport and the cell
// assertions) rather than standing up a second harness. The feed and every
// vote response must carry the same trust object, and its counters must count
// distinct places, not votes. Every package level identifier added here
// carries the p07 prefix, except newTrustE2EServer, whose name the phase
// outline pins.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// p07ReportLat and p07ReportLon are the report point every test in this file
// submits at.
const p07ReportLat, p07ReportLon = 12.9716, 77.5946

// newTrustE2EServer builds the same real router newE2EServer does, with two
// differences. First, when clock is non nil it is passed to the report
// service through service.WithReportClock and to the voting service through
// service.WithVotingClock, so the time dependent trust fields agree between
// the feed and the vote response; when clock is nil no option is passed at
// all. Second, the request-link limiter is opened to
// api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond}, the
// precedent newAuthE2EServerWithRateLimit sets: the default limiter allows 5
// request-link calls a minute from 127.0.0.1, and these tests verify up to 25
// accounts. The AuthService keeps the wall clock because magic link expiry
// and the resend cooldown read it. Every test in this file uses this helper.
func newTrustE2EServer(t *testing.T, clock func() time.Time) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.NewTestDB(t)

	mgr, err := session.NewManager([]byte("trust-e2e-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	queries := sqlcgen.New(pool)
	fm := &recordingMailer{}

	var reportOpts []service.ReportOption
	var votingOpts []service.VotingOption
	if clock != nil {
		reportOpts = append(reportOpts, service.WithReportClock(clock))
		votingOpts = append(votingOpts, service.WithVotingClock(clock))
	}
	deps := api.Deps{
		Session:              mgr,
		Sessions:             queries,
		Reports:              service.NewReportService(queries, reportOpts...),
		Votes:                service.NewVotingService(queries, votingOpts...),
		AuthService:          service.NewAuthService(queries, fm, "https://pinalert.example"),
		Auth:                 handlers.AuthConfig{AssetVersion: "test"},
		Template:             tmpl,
		RequestLinkRateLimit: api.RequestLinkRateLimit{Burst: 100, Every: time.Millisecond},
	}
	router := api.NewRouter(deps)

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, fm, pool
}

// p07FixedClock returns a closure over one instant captured once, one minute
// after the test starts, so it stays inside every report's lifetime and after
// every vote the test writes. The closure only reads that value, so it is
// race free.
func p07FixedClock() func() time.Time {
	fixed := time.Now().UTC().Add(time.Minute)
	return func() time.Time { return fixed }
}

// p07PostVoteRaw POSTs one vote and returns the status and raw body. It takes
// no *testing.T, so goroutines may call it.
func p07PostVoteRaw(client *http.Client, baseURL string, reportID int64, action string, lat, lon float64) (int, []byte, error) {
	buf, err := json.Marshal(map[string]float64{"latitude": lat, "longitude": lon})
	if err != nil {
		return 0, nil, err
	}
	url := baseURL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/" + action
	resp, err := client.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// p07VoteTrust casts one vote, requires 200, and returns the visibility and
// the raw trust member of the response. It fails when trust is missing or
// JSON null.
func p07VoteTrust(t *testing.T, client *http.Client, baseURL string, reportID int64, action string, lat, lon float64) (string, json.RawMessage) {
	t.Helper()
	status, body, err := p07PostVoteRaw(client, baseURL, reportID, action, lat, lon)
	if err != nil {
		t.Fatalf("POST %s vote on report %d: %v", action, reportID, err)
	}
	if status != http.StatusOK {
		t.Fatalf("POST %s vote on report %d: status %d, body %s", action, reportID, status, body)
	}
	var out struct {
		Visibility string          `json:"visibility"`
		Trust      json.RawMessage `json:"trust"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding %s vote response: %v (body %s)", action, err, body)
	}
	if len(out.Trust) == 0 || string(out.Trust) == "null" {
		t.Fatalf("%s vote response carries no trust object: %s", action, body)
	}
	return out.Visibility, out.Trust
}

// p07GetFeedRaw reads the feed at the report point and returns the raw body,
// failing unless the status is 200.
func p07GetFeedRaw(t *testing.T, client *http.Client, baseURL string, showDisputed bool) []byte {
	t.Helper()
	resp := getNearby(t, client, baseURL, p07ReportLat, p07ReportLon, showDisputed)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading feed body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/reports: status %d, body %s", resp.StatusCode, body)
	}
	return body
}

// p07FeedTrust reads the feed and returns the raw trust of reportID and
// whether that report was listed. It fails if any element's trust is missing
// or null.
func p07FeedTrust(t *testing.T, client *http.Client, baseURL string, reportID int64, showDisputed bool) (json.RawMessage, bool) {
	t.Helper()
	body := p07GetFeedRaw(t, client, baseURL, showDisputed)
	var out struct {
		Reports []struct {
			ID    int64           `json:"id"`
			Trust json.RawMessage `json:"trust"`
		} `json:"reports"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding feed body: %v (body %s)", err, body)
	}
	var found json.RawMessage
	var ok bool
	for _, r := range out.Reports {
		if len(r.Trust) == 0 || string(r.Trust) == "null" {
			t.Fatalf("feed element %d carries no trust object: %s", r.ID, body)
		}
		if r.ID == reportID {
			found, ok = r.Trust, true
		}
	}
	return found, ok
}

// p07TrustWire is the client side view of the trust object. It tolerates
// extra keys, because plan 03-14 adds two.
type p07TrustWire struct {
	ConfirmedPlaces   int    `json:"confirmed_places"`
	DisputedPlaces    int    `json:"disputed_places"`
	ConfirmedText     string `json:"confirmed_text"`
	DisputedText      string `json:"disputed_text"`
	LeadSentence      string `json:"lead_sentence"`
	ConfirmedSentence string `json:"confirmed_sentence"`
	DisputedSentence  string `json:"disputed_sentence"`
}

func p07DecodeTrust(t *testing.T, raw json.RawMessage) p07TrustWire {
	t.Helper()
	var w p07TrustWire
	if err := json.Unmarshal(raw, &w); err != nil {
		t.Fatalf("decoding trust object: %v (raw %s)", err, raw)
	}
	return w
}

// p07TrustWireKeys is the exact key set of the trust object. Plan 03-14
// appended currency and reporter, the two signal objects, to this one list, so
// it stays a single package level slice.
var p07TrustWireKeys = []string{
	"confirmed_places", "disputed_places", "confirmed_text", "disputed_text",
	"lead_sentence", "confirmed_sentence", "disputed_sentence",
	"currency", "reporter",
}

// p14CurrencyWireKeys is the exact key set of trust.currency, the eight rows
// of the phase outline's "The trust wire object" table. The spec test and the
// runtime tests both read this one list.
var p14CurrencyWireKeys = []string{"band", "score", "label", "sentence", "score_sentence", "recent_places", "window_minutes", "fade_stage"}

// p14ReporterWireKeys is the exact key set of trust.reporter, the seven rows
// of the same outline table.
var p14ReporterWireKeys = []string{"tag", "score", "label", "sentence", "score_sentence", "judged_reports", "corroborated_reports"}

// p07AssertTrustKeys fails unless the object's key set equals
// p07TrustWireKeys exactly.
func p07AssertTrustKeys(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("trust object is not a JSON object: %v (raw %s)", err, raw)
	}
	want := map[string]bool{}
	for _, k := range p07TrustWireKeys {
		want[k] = true
		if _, ok := m[k]; !ok {
			t.Errorf("trust object is missing key %q: %s", k, raw)
		}
	}
	for k := range m {
		if !want[k] {
			t.Errorf("trust object has unexpected key %q: %s", k, raw)
		}
	}
}

// p07AssertSameTrust fails unless a and b decode to deeply equal objects.
func p07AssertSameTrust(t *testing.T, label string, a, b json.RawMessage) {
	t.Helper()
	var ma, mb map[string]any
	if err := json.Unmarshal(a, &ma); err != nil {
		t.Fatalf("%s: decoding first trust: %v (raw %s)", label, err, a)
	}
	if err := json.Unmarshal(b, &mb); err != nil {
		t.Fatalf("%s: decoding second trust: %v (raw %s)", label, err, b)
	}
	if !reflect.DeepEqual(ma, mb) {
		t.Fatalf("%s: trust objects differ\n  first:  %s\n  second: %s", label, a, b)
	}
}

// p07WantTrust is the expectation for one step: both counters, their capped
// numerals (derived from the counts, all small here) and the three sentences.
type p07WantTrust struct {
	confirmed, disputed                  int
	lead, confirmedSentence, disputedSen string
}

func p07AssertTrust(t *testing.T, label string, raw json.RawMessage, want p07WantTrust) {
	t.Helper()
	p07AssertTrustKeys(t, raw)
	got := p07DecodeTrust(t, raw)
	exp := p07TrustWire{
		ConfirmedPlaces:   want.confirmed,
		DisputedPlaces:    want.disputed,
		ConfirmedText:     strconv.Itoa(want.confirmed),
		DisputedText:      strconv.Itoa(want.disputed),
		LeadSentence:      want.lead,
		ConfirmedSentence: want.confirmedSentence,
		DisputedSentence:  want.disputedSen,
	}
	if got != exp {
		t.Fatalf("%s: trust = %+v, want %+v", label, got, exp)
	}
}

const (
	p07Lead           = "Needs 2 confirmations to count as confirmed."
	p07NoDispute      = "No one nearby has disputed this."
	p07Confirmed1     = "1 place confirmed this. Counts places, not votes."
	p07Confirmed2     = "2 different places confirmed this. Counts places, not votes."
	p07Confirmed3     = "3 different places confirmed this. Counts places, not votes."
	p07Disputed1      = "1 place disputed this."
	p07Disputed2      = "2 different places disputed this."
	p07FeedCompareTag = "vote response vs feed"
)

// p07CompareWithFeed requires the report to be listed for each given reader
// and its feed trust to deeply equal the vote response's trust.
func p07CompareWithFeed(t *testing.T, step string, voteTrust json.RawMessage, baseURL string, reportID int64, readers map[string]*http.Client) {
	t.Helper()
	for name, c := range readers {
		feed, ok := p07FeedTrust(t, c, baseURL, reportID, false)
		if !ok {
			t.Fatalf("%s: report %d not listed for %s", step, reportID, name)
		}
		p07AssertSameTrust(t, step+" "+p07FeedCompareTag+" read by "+name, voteTrust, feed)
	}
}

// TestFeedTrustMatchesCastVoteTrust is V-29 and D-21: the feed and every vote
// response, on all four routes, carry the same trust object for the same data
// and clock, whichever verified account reads the feed.
func TestFeedTrustMatchesCastVoteTrust(t *testing.T) {
	srv, mailer, _ := newTrustE2EServer(t, p07FixedClock())

	a := newVerifiedClient(t, srv, mailer, "p07-match-a@example.com")
	b := newVerifiedClient(t, srv, mailer, "p07-match-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p07-match-c@example.com")
	d := newVerifiedClient(t, srv, mailer, "p07-match-d@example.com")
	e := newVerifiedClient(t, srv, mailer, "p07-match-e@example.com")

	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	p2Lat, p2Lon := 12.9686, 77.5946
	p3Lat, p3Lon := 12.9746, 77.5946
	p4Lat, p4Lon := 12.9716, 77.5986
	p5Lat, p5Lon := 12.9716, 77.5906

	// Step 1: B confirms from P3. Provisional, one confirming place.
	vis, tr := p07VoteTrust(t, b, srv.URL, reportID, "confirm", p3Lat, p3Lon)
	if vis != "provisional" {
		t.Fatalf("step 1: visibility = %q, want provisional", vis)
	}
	p07AssertTrust(t, "step 1", tr, p07WantTrust{1, 0, p07Lead, p07Confirmed1, p07NoDispute})
	p07CompareWithFeed(t, "step 1", tr, srv.URL, reportID, map[string]*http.Client{"B": b, "A": a, "C": c})

	// Step 2: C confirms from P2. Live, two confirming places.
	vis, tr = p07VoteTrust(t, c, srv.URL, reportID, "confirm", p2Lat, p2Lon)
	if vis != "live" {
		t.Fatalf("step 2: visibility = %q, want live", vis)
	}
	p07AssertTrust(t, "step 2", tr, p07WantTrust{2, 0, "", p07Confirmed2, p07NoDispute})
	p07CompareWithFeed(t, "step 2", tr, srv.URL, reportID, map[string]*http.Client{"C": c, "A": a, "B": b})

	// Step 3: D disputes from P4. Still live, 2 confirmed and 1 disputed.
	vis, tr = p07VoteTrust(t, d, srv.URL, reportID, "dispute", p4Lat, p4Lon)
	if vis != "live" {
		t.Fatalf("step 3: visibility = %q, want live", vis)
	}
	p07AssertTrust(t, "step 3", tr, p07WantTrust{2, 1, "", p07Confirmed2, p07Disputed1})
	p07CompareWithFeed(t, "step 3", tr, srv.URL, reportID, map[string]*http.Client{"D": d, "A": a, "B": b})

	// Step 4: E resolves from P5. A resolution vote is never a confirm or a
	// dispute, so the counts do not move.
	vis, tr = p07VoteTrust(t, e, srv.URL, reportID, "resolve", p5Lat, p5Lon)
	if vis != "live" {
		t.Fatalf("step 4: visibility = %q, want live", vis)
	}
	p07AssertTrust(t, "step 4", tr, p07WantTrust{2, 1, "", p07Confirmed2, p07Disputed1})
	p07CompareWithFeed(t, "step 4", tr, srv.URL, reportID, map[string]*http.Client{"E": e, "A": a, "B": b})

	// Step 5: A resolves their own report. Retracted wins the lead line, the
	// counts stay, and the report is absent from both views (D-12), so there
	// is no feed comparison here.
	vis, tr = p07VoteTrust(t, a, srv.URL, reportID, "resolve", p07ReportLat, p07ReportLon)
	if vis != "retracted" {
		t.Fatalf("step 5: visibility = %q, want retracted", vis)
	}
	p07AssertTrust(t, "step 5", tr, p07WantTrust{2, 1, "", p07Confirmed2, p07Disputed1})
	for _, show := range []bool{false, true} {
		if _, ok := p07FeedTrust(t, a, srv.URL, reportID, show); ok {
			t.Fatalf("step 5: retracted report listed with show_disputed=%v", show)
		}
	}

	// Step 6: A reopens their own report. Live again, and the trust matches
	// the feed for both A and B.
	vis, tr = p07VoteTrust(t, a, srv.URL, reportID, "reopen", p07ReportLat, p07ReportLon)
	if vis != "live" {
		t.Fatalf("step 6: visibility = %q, want live", vis)
	}
	p07AssertTrustKeys(t, tr)
	p07CompareWithFeed(t, "step 6", tr, srv.URL, reportID, map[string]*http.Client{"A": a, "B": b})
}

// TestTrustCountsDistinctPlacesNotVotes is D-04 on the wire: N and M count
// distinct places, so two accounts in one place count once, a re-tap counts
// once, and an account that switches to dispute moves its place.
func TestTrustCountsDistinctPlacesNotVotes(t *testing.T) {
	srv, mailer, pool := newTrustE2EServer(t, p07FixedClock())

	a := newVerifiedClient(t, srv, mailer, "p07-places-a@example.com")
	b1 := newVerifiedClient(t, srv, mailer, "p07-places-b1@example.com")
	b2 := newVerifiedClient(t, srv, mailer, "p07-places-b2@example.com")
	c := newVerifiedClient(t, srv, mailer, "p07-places-c@example.com")
	d := newVerifiedClient(t, srv, mailer, "p07-places-d@example.com")
	e := newVerifiedClient(t, srv, mailer, "p07-places-e@example.com")

	p1Lat, p1Lon := 12.9720, 77.5950
	p2Lat, p2Lon := 12.9686, 77.5946
	p3Lat, p3Lon := 12.9746, 77.5946
	p4Lat, p4Lon := 12.9716, 77.5986

	// The table below relies on these cell facts, so check them at runtime.
	assertSameCell(t, p1Lat, p1Lon, p1Lat, p1Lon)
	pts := [][2]float64{{p1Lat, p1Lon}, {p2Lat, p2Lon}, {p3Lat, p3Lon}, {p4Lat, p4Lon}}
	for i := 0; i < len(pts); i++ {
		for j := i + 1; j < len(pts); j++ {
			assertDistinctCells(t, pts[i][0], pts[i][1], pts[j][0], pts[j][1])
		}
	}

	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	steps := []struct {
		name     string
		client   *http.Client
		action   string
		lat, lon float64
		want     p07WantTrust
		rows     int
	}{
		{"1 B1 confirm P1", b1, "confirm", p1Lat, p1Lon, p07WantTrust{1, 0, p07Lead, p07Confirmed1, p07NoDispute}, 1},
		{"2 B2 confirm P1", b2, "confirm", p1Lat, p1Lon, p07WantTrust{1, 0, p07Lead, p07Confirmed1, p07NoDispute}, 2},
		{"3 B1 re-tap P1", b1, "confirm", p1Lat, p1Lon, p07WantTrust{1, 0, p07Lead, p07Confirmed1, p07NoDispute}, 3},
		{"4 C confirm P2", c, "confirm", p2Lat, p2Lon, p07WantTrust{2, 0, "", p07Confirmed2, p07NoDispute}, 4},
		{"5 D confirm P3", d, "confirm", p3Lat, p3Lon, p07WantTrust{3, 0, "", p07Confirmed3, p07NoDispute}, 5},
		{"6 E dispute P4", e, "dispute", p4Lat, p4Lon, p07WantTrust{3, 1, "", p07Confirmed3, p07Disputed1}, 6},
		{"7 C switch to dispute P2", c, "dispute", p2Lat, p2Lon, p07WantTrust{2, 2, "", p07Confirmed2, p07Disputed2}, 7},
	}
	for i, s := range steps {
		vis, tr := p07VoteTrust(t, s.client, srv.URL, reportID, s.action, s.lat, s.lon)
		p07AssertTrust(t, "step "+s.name, tr, s.want)
		feed, ok := p07FeedTrust(t, s.client, srv.URL, reportID, false)
		if !ok {
			t.Fatalf("step %s: report not listed for the voter", s.name)
		}
		p07AssertSameTrust(t, "step "+s.name+" "+p07FeedCompareTag, tr, feed)
		if got := countVotesForReport(t, pool, reportID); got != s.rows {
			t.Fatalf("step %s: %d raw vote rows, want %d", s.name, got, s.rows)
		}
		if i == len(steps)-1 && vis != "live" {
			t.Fatalf("step %s: visibility = %q, want live", s.name, vis)
		}
	}

	// 2 confirming plus 2 disputing places came from 7 rows cast by 5 accounts.
	t.Logf("2 confirmed places and 2 disputed places came from 7 vote rows cast by 5 accounts")
}

// p07WireIdentifierKey is the key name pattern no feed or vote body may carry
// at any depth.
var p07WireIdentifierKey = regexp.MustCompile(`(?i)cell|account|session|email|voter`)

// p07WalkJSONKeys fails on any object key matching p07WireIdentifierKey,
// recursing through maps and slices.
func p07WalkJSONKeys(t *testing.T, label string, v any) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if p07WireIdentifierKey.MatchString(k) {
				t.Errorf("%s: JSON key %q looks like an identifier", label, k)
			}
			p07WalkJSONKeys(t, label, val)
		}
	case []any:
		for _, val := range x {
			p07WalkJSONKeys(t, label, val)
		}
	}
}

// p07SessionID returns the id part of the client's pinalert_session cookie,
// read the way sessionIDFromJar and TestNearbyResponseOmitsSessionID read it.
// It fails when the cookie or the id is missing.
func p07SessionID(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	id := sessionIDFromJar(t, client, baseURL)
	if id == "" {
		t.Fatalf("empty session id for %s", baseURL)
	}
	return id
}

// TestTrustResponseOmitsIdentifiers is V-31 and T-03-04: no account id,
// email, voter cell or session id appears in a feed or vote response, either
// as a key at any depth or as a value anywhere in the raw text.
func TestTrustResponseOmitsIdentifiers(t *testing.T) {
	srv, mailer, _ := newTrustE2EServer(t, p07FixedClock())

	const reporterEmail = "p07-omit-reporter@example.com"
	const voterEmail = "p07-omit-voter@example.com"
	a := newVerifiedClient(t, srv, mailer, reporterEmail)
	b := newVerifiedClient(t, srv, mailer, voterEmail)

	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	p3Lat, p3Lon := 12.9746, 77.5946
	voterCell := geohash.EncodeWithPrecision(p3Lat, p3Lon, voterGeohashPrecision)
	reportCell := geohash.EncodeWithPrecision(p07ReportLat, p07ReportLon, 8)
	if strings.Contains(reportCell, voterCell) {
		t.Fatalf("the voter cell %q is a substring of the report's own geohash %q, so the check could match by accident", voterCell, reportCell)
	}

	status, voteBody, err := p07PostVoteRaw(b, srv.URL, reportID, "confirm", p3Lat, p3Lon)
	if err != nil || status != http.StatusOK {
		t.Fatalf("confirm vote: status %d, err %v, body %s", status, err, voteBody)
	}
	feedAsB := p07GetFeedRaw(t, b, srv.URL, false)
	feedAsA := p07GetFeedRaw(t, a, srv.URL, false)

	sessionA := p07SessionID(t, a, srv.URL)
	sessionB := p07SessionID(t, b, srv.URL)

	bodies := []struct {
		label string
		raw   []byte
	}{
		{"vote response", voteBody},
		{"feed read by the voter", feedAsB},
		{"feed read by the reporter", feedAsA},
	}
	for _, bd := range bodies {
		var decoded any
		if err := json.Unmarshal(bd.raw, &decoded); err != nil {
			t.Fatalf("%s: not JSON: %v (body %s)", bd.label, err, bd.raw)
		}
		p07WalkJSONKeys(t, bd.label, decoded)

		text := string(bd.raw)
		for _, banned := range []string{reporterEmail, voterEmail, "@", voterCell, "account_id", "geohash_cell", sessionA, sessionB} {
			if strings.Contains(text, banned) {
				t.Errorf("%s contains %q: %s", bd.label, banned, bd.raw)
			}
		}
	}

	// Each trust object carries exactly the pinned keys.
	var vote struct {
		Trust json.RawMessage `json:"trust"`
	}
	if err := json.Unmarshal(voteBody, &vote); err != nil {
		t.Fatalf("decoding vote body: %v", err)
	}
	p07AssertTrustKeys(t, vote.Trust)
	for name, c := range map[string]*http.Client{"voter": b, "reporter": a} {
		tr, ok := p07FeedTrust(t, c, srv.URL, reportID, false)
		if !ok {
			t.Fatalf("report %d not listed for the %s", reportID, name)
		}
		p07AssertTrustKeys(t, tr)
	}
}

// TestTrustCountUnderConcurrentVotes is TRUST-05 criterion 1 under load
// (D-04, V-41): many concurrent votes, then one read, equals the distinct
// places of the committed rows. 24 verified accounts confirm at the same
// moment from 12 distinct places, two accounts per place.
func TestTrustCountUnderConcurrentVotes(t *testing.T) {
	srv, mailer, pool := newTrustE2EServer(t, p07FixedClock())

	const (
		places   = 12
		accounts = 24
	)
	a := newVerifiedClient(t, srv, mailer, "p07-concurrent-reporter@example.com")

	// A 3 by 4 grid. The 0.002 degree spacing exceeds the precision 7 cell
	// size of 0.001373 degrees in both axes, so no two points share a cell,
	// and the farthest point is about 0.39 km from the report, inside the
	// 1 km rule.
	var pts [][2]float64
	for _, lat := range []float64{12.9696, 12.9716, 12.9736} {
		for _, lon := range []float64{77.5916, 77.5936, 77.5956, 77.5976} {
			pts = append(pts, [2]float64{lat, lon})
		}
	}
	cells := map[string]bool{}
	for _, p := range pts {
		cells[geohash.EncodeWithPrecision(p[0], p[1], voterGeohashPrecision)] = true
	}
	if len(pts) != places || len(cells) != places {
		t.Fatalf("grid has %d points in %d distinct precision 7 cells, want %d of each", len(pts), len(cells), places)
	}

	reportID := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))

	clients := make([]*http.Client, accounts)
	for i := range clients {
		email := "p07-concurrent-" + strconv.Itoa(i/10) + strconv.Itoa(i%10) + "@example.com"
		clients[i] = newVerifiedClient(t, srv, mailer, email)
	}

	type result struct {
		status int
		body   []byte
		err    error
	}
	results := make([]result, accounts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < accounts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			p := pts[i%places]
			status, body, err := p07PostVoteRaw(clients[i], srv.URL, reportID, "confirm", p[0], p[1])
			results[i] = result{status, body, err}
		}(i)
	}
	close(start)
	wg.Wait()

	maxConfirmed := 0
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("voter %d: %v", i, r.err)
		}
		if r.status != http.StatusOK {
			t.Fatalf("voter %d: status %d, body %s", i, r.status, r.body)
		}
		var out struct {
			Trust json.RawMessage `json:"trust"`
		}
		if err := json.Unmarshal(r.body, &out); err != nil || len(out.Trust) == 0 {
			t.Fatalf("voter %d: no trust in %s (%v)", i, r.body, err)
		}
		tr := p07DecodeTrust(t, out.Trust)
		if tr.ConfirmedPlaces < 1 || tr.ConfirmedPlaces > places || tr.DisputedPlaces != 0 {
			t.Fatalf("voter %d: confirmed_places %d, disputed_places %d, want 1 to %d and 0", i, tr.ConfirmedPlaces, tr.DisputedPlaces, places)
		}
		if tr.ConfirmedPlaces > maxConfirmed {
			maxConfirmed = tr.ConfirmedPlaces
		}
	}
	// CastVote runs InsertVote and the read back as separate autocommit
	// statements with no transaction (its own doc comment says so), so the
	// vote whose insert committed last reads all 24 rows. If CastVote ever
	// wraps them in one transaction, this one assertion is what changes.
	if maxConfirmed != places {
		t.Fatalf("largest confirmed_places across the responses = %d, want %d", maxConfirmed, places)
	}

	feed, ok := p07FeedTrust(t, a, srv.URL, reportID, false)
	if !ok {
		t.Fatalf("report %d not listed for the reporter", reportID)
	}
	p07AssertTrustKeys(t, feed)
	p07AssertTrust(t, "final feed", feed, p07WantTrust{
		confirmed:         12,
		disputed:          0,
		lead:              "",
		confirmedSentence: "12 different places confirmed this. Counts places, not votes.",
		disputedSen:       p07NoDispute,
	})
	if got := countVotesForReport(t, pool, reportID); got != accounts {
		t.Fatalf("%d vote rows stored, want %d", got, accounts)
	}
}

// p14CurrencyWire and p14ReporterWire are the client side views of the two
// signal objects, decoded from the JSON contract and never from the server's
// Go types.
type p14CurrencyWire struct {
	Band          string  `json:"band"`
	Score         *int    `json:"score"`
	Label         string  `json:"label"`
	Sentence      string  `json:"sentence"`
	ScoreSentence string  `json:"score_sentence"`
	RecentPlaces  int     `json:"recent_places"`
	WindowMinutes int     `json:"window_minutes"`
	FadeStage     *string `json:"fade_stage"`
}

type p14ReporterWire struct {
	Tag                 string `json:"tag"`
	Score               *int   `json:"score"`
	Label               string `json:"label"`
	Sentence            string `json:"sentence"`
	ScoreSentence       string `json:"score_sentence"`
	JudgedReports       int    `json:"judged_reports"`
	CorroboratedReports int    `json:"corroborated_reports"`
}

// p14Str returns a pointer to s, for expected fade stages.
func p14Str(s string) *string { return &s }

// p14Int returns a pointer to n, for expected scores.
func p14Int(n int) *int { return &n }

// p14AssertSignalKeys fails unless trust.currency and trust.reporter have
// exactly their pinned key sets, so a null score or fade stage must be present
// as null and not omitted.
func p14AssertSignalKeys(t *testing.T, label string, trust json.RawMessage) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trust, &top); err != nil {
		t.Fatalf("%s: trust is not a JSON object: %v (raw %s)", label, err, trust)
	}
	for member, want := range map[string][]string{"currency": p14CurrencyWireKeys, "reporter": p14ReporterWireKeys} {
		raw, ok := top[member]
		if !ok {
			t.Errorf("%s: trust has no %q member: %s", label, member, trust)
			continue
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("%s: trust.%s is not a JSON object: %v (raw %s)", label, member, err, raw)
			continue
		}
		wantSet := map[string]bool{}
		for _, k := range want {
			wantSet[k] = true
			if _, ok := got[k]; !ok {
				t.Errorf("%s: trust.%s is missing key %q: %s", label, member, k, raw)
			}
		}
		for k := range got {
			if !wantSet[k] {
				t.Errorf("%s: trust.%s has unexpected key %q: %s", label, member, k, raw)
			}
		}
	}
}

// p14DecodeSignals decodes the two signal members of a trust object and fails
// when either is missing or JSON null.
func p14DecodeSignals(t *testing.T, trust json.RawMessage) (p14CurrencyWire, p14ReporterWire) {
	t.Helper()
	var top struct {
		Currency json.RawMessage `json:"currency"`
		Reporter json.RawMessage `json:"reporter"`
	}
	if err := json.Unmarshal(trust, &top); err != nil {
		t.Fatalf("decoding trust object: %v (raw %s)", err, trust)
	}
	for name, raw := range map[string]json.RawMessage{"currency": top.Currency, "reporter": top.Reporter} {
		if len(raw) == 0 || string(raw) == "null" {
			t.Fatalf("trust.%s is missing or null: %s", name, trust)
		}
	}
	var c p14CurrencyWire
	var r p14ReporterWire
	if err := json.Unmarshal(top.Currency, &c); err != nil {
		t.Fatalf("decoding trust.currency: %v (raw %s)", err, top.Currency)
	}
	if err := json.Unmarshal(top.Reporter, &r); err != nil {
		t.Fatalf("decoding trust.reporter: %v (raw %s)", err, top.Reporter)
	}
	return c, r
}

// p14AssertSameSignals fails unless the currency and reporter members of a
// and b are deeply equal, each compared on its own so a mismatch prints the
// two raw values.
func p14AssertSameSignals(t *testing.T, label string, a, b json.RawMessage) {
	t.Helper()
	var ma, mb map[string]json.RawMessage
	if err := json.Unmarshal(a, &ma); err != nil {
		t.Fatalf("%s: decoding first trust: %v (raw %s)", label, err, a)
	}
	if err := json.Unmarshal(b, &mb); err != nil {
		t.Fatalf("%s: decoding second trust: %v (raw %s)", label, err, b)
	}
	for _, member := range []string{"currency", "reporter"} {
		var va, vb map[string]any
		if err := json.Unmarshal(ma[member], &va); err != nil {
			t.Fatalf("%s: decoding first %s: %v (raw %s)", label, member, err, ma[member])
		}
		if err := json.Unmarshal(mb[member], &vb); err != nil {
			t.Fatalf("%s: decoding second %s: %v (raw %s)", label, member, err, mb[member])
		}
		if !reflect.DeepEqual(va, vb) {
			t.Fatalf("%s: %s signals differ\n  first:  %s\n  second: %s", label, member, ma[member], mb[member])
		}
	}
}

// p14PtrEqual reports whether two optional values are both nil or equal.
func p14PtrEqual[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func p14Show[T any](p *T) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprintf("%v", *p)
}

// p14AssertCurrency compares every field of two currency signals.
func p14AssertCurrency(t *testing.T, label string, got, want p14CurrencyWire) {
	t.Helper()
	if got.Band != want.Band || !p14PtrEqual(got.Score, want.Score) || got.Label != want.Label ||
		got.Sentence != want.Sentence || got.ScoreSentence != want.ScoreSentence ||
		got.RecentPlaces != want.RecentPlaces || got.WindowMinutes != want.WindowMinutes ||
		!p14PtrEqual(got.FadeStage, want.FadeStage) {
		t.Fatalf("%s: currency differs\n  got:  %+v score=%s fade=%s\n  want: %+v score=%s fade=%s",
			label, got, p14Show(got.Score), p14Show(got.FadeStage), want, p14Show(want.Score), p14Show(want.FadeStage))
	}
}

// p14AssertReporter compares every field of two reporter signals.
func p14AssertReporter(t *testing.T, label string, got, want p14ReporterWire) {
	t.Helper()
	if got.Tag != want.Tag || !p14PtrEqual(got.Score, want.Score) || got.Label != want.Label ||
		got.Sentence != want.Sentence || got.ScoreSentence != want.ScoreSentence ||
		got.JudgedReports != want.JudgedReports || got.CorroboratedReports != want.CorroboratedReports {
		t.Fatalf("%s: reporter differs\n  got:  %+v score=%s\n  want: %+v score=%s",
			label, got, p14Show(got.Score), want, p14Show(want.Score))
	}
}

// p14UpToDate checks an Up to date currency signal whose score depends on
// seconds of test runtime: the score only has to sit between UpToDateMinScore
// and 100, and every other field is exact.
func p14UpToDate(t *testing.T, label string, got p14CurrencyWire, places, windowMinutes int, window string, fade *string) {
	t.Helper()
	if got.Score == nil || *got.Score < service.UpToDateMinScore || *got.Score > 100 {
		t.Fatalf("%s: currency score = %s, want %d to 100", label, p14Show(got.Score), service.UpToDateMinScore)
	}
	p14AssertCurrency(t, label, got, p14CurrencyWire{
		Band:          "up_to_date",
		Score:         got.Score,
		Label:         "Report up to date",
		Sentence:      fmt.Sprintf("%d places confirmed this in the last %s.", places, window),
		ScoreSentence: fmt.Sprintf("Score %d out of 100, based on recent confirmations.", *got.Score),
		RecentPlaces:  places,
		WindowMinutes: windowMinutes,
		FadeStage:     fade,
	})
}

// p14NewReporter is the reporter signal of an account with too few judged
// reports.
func p14NewReporter() p14ReporterWire {
	return p14ReporterWire{
		Tag:      "new",
		Label:    "New reporter",
		Sentence: "A rating needs 5 reports with a clear result. They have none so far.",
	}
}

// p14NoneReporter is the reporter signal of an accountless report.
func p14NoneReporter() p14ReporterWire {
	return p14ReporterWire{Tag: "none"}
}

// p14Step is the D-21 check at one step: every trust object has the pinned
// keys, and the vote response's whole trust object and both signals equal each
// feed read.
func p14Step(t *testing.T, label string, voteTrust json.RawMessage, feedTrusts ...json.RawMessage) {
	t.Helper()
	p07AssertTrustKeys(t, voteTrust)
	p14AssertSignalKeys(t, label+" vote response", voteTrust)
	for i, ft := range feedTrusts {
		name := label + " feed " + strconv.Itoa(i)
		p07AssertTrustKeys(t, ft)
		p14AssertSignalKeys(t, name, ft)
		p07AssertSameTrust(t, name, voteTrust, ft)
		p14AssertSameSignals(t, name, voteTrust, ft)
	}
}

// p14FeedTrustOrFail reads the feed and returns the trust of reportID, failing
// when the report is not listed.
func p14FeedTrustOrFail(t *testing.T, label string, client *http.Client, baseURL string, reportID int64) json.RawMessage {
	t.Helper()
	tr, ok := p07FeedTrust(t, client, baseURL, reportID, false)
	if !ok {
		t.Fatalf("%s: report %d is not listed in the feed", label, reportID)
	}
	return tr
}

// TestFeedSignalsMatchCastVoteSignals is V-29 (signals part), D-08 and D-21:
// on real votes the feed and every vote response carry the same still current
// and reporter signals, read by the voter and by the reporter, with one pinned
// clock for both paths.
func TestFeedSignalsMatchCastVoteSignals(t *testing.T) {
	srv, mailer, _ := newTrustE2EServer(t, p07FixedClock())

	a := newVerifiedClient(t, srv, mailer, "p14-signals-reporter@example.com")
	b := newVerifiedClient(t, srv, mailer, "p14-signals-b@example.com")
	c := newVerifiedClient(t, srv, mailer, "p14-signals-c@example.com")
	d := newVerifiedClient(t, srv, mailer, "p14-signals-d@example.com")
	e := newVerifiedClient(t, srv, mailer, "p14-signals-e@example.com")

	// Four voter places inside 1 km of the report point, each in its own
	// precision 7 cell (03-07's measured points).
	p2Lat, p2Lon := 12.9686, 77.5946
	p3Lat, p3Lon := 12.9746, 77.5946
	p4Lat, p4Lon := 12.9716, 77.5986
	p5Lat, p5Lon := 12.9716, 77.5906

	r1 := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	critical := floodReportBody(p07ReportLat, p07ReportLon)
	critical["severity"] = "critical"
	r2 := submitReportGetID(t, a, srv.URL, critical)

	notYet := func(places, window int) p14CurrencyWire {
		return p14CurrencyWire{
			Band:          "not_yet_corroborated",
			RecentPlaces:  places,
			WindowMinutes: window,
			FadeStage:     p14Str("fresh"),
		}
	}

	// Step 0: before any vote.
	tr := p14FeedTrustOrFail(t, "step 0", a, srv.URL, r1)
	p14AssertSignalKeys(t, "step 0", tr)
	cur, rep := p14DecodeSignals(t, tr)
	p14AssertCurrency(t, "step 0", cur, notYet(0, 120))
	p14AssertReporter(t, "step 0", rep, p14NewReporter())

	// Step 1: B confirms R1 from P3. Provisional, one confirming place.
	vis, vt := p07VoteTrust(t, b, srv.URL, r1, "confirm", p3Lat, p3Lon)
	if vis != "provisional" {
		t.Fatalf("step 1: visibility = %q, want provisional", vis)
	}
	p14Step(t, "step 1", vt,
		p14FeedTrustOrFail(t, "step 1 B", b, srv.URL, r1),
		p14FeedTrustOrFail(t, "step 1 A", a, srv.URL, r1))
	cur, rep = p14DecodeSignals(t, vt)
	p14AssertCurrency(t, "step 1", cur, notYet(1, 120))
	p14AssertReporter(t, "step 1", rep, p14NewReporter())

	// Step 2: C confirms R1 from P2. Live, two confirming places.
	vis, vt = p07VoteTrust(t, c, srv.URL, r1, "confirm", p2Lat, p2Lon)
	if vis != "live" {
		t.Fatalf("step 2: visibility = %q, want live", vis)
	}
	p14Step(t, "step 2", vt,
		p14FeedTrustOrFail(t, "step 2 C", c, srv.URL, r1),
		p14FeedTrustOrFail(t, "step 2 A", a, srv.URL, r1))
	cur, rep = p14DecodeSignals(t, vt)
	p14UpToDate(t, "step 2", cur, 2, 120, "2 hours", p14Str("fresh"))
	p14AssertReporter(t, "step 2", rep, p14NewReporter())

	// Step 3: D disputes R1 from P4. One disputing place cannot hide two
	// confirming ones, so it stays live. A dispute never refreshes the
	// signal: it raises the bar to 3 places.
	vis, vt = p07VoteTrust(t, d, srv.URL, r1, "dispute", p4Lat, p4Lon)
	if vis != "live" {
		t.Fatalf("step 3: visibility = %q, want live", vis)
	}
	p14Step(t, "step 3", vt,
		p14FeedTrustOrFail(t, "step 3 D", d, srv.URL, r1),
		p14FeedTrustOrFail(t, "step 3 A", a, srv.URL, r1))
	cur, rep = p14DecodeSignals(t, vt)
	p14AssertCurrency(t, "step 3", cur, p14CurrencyWire{
		Band:          "needs_reconfirming",
		Score:         p14Int(0),
		Label:         "Report needs re-confirming",
		Sentence:      "Disputed nearby, so it needs 3 places in the last 2 hours.",
		ScoreSentence: "Score 0 out of 100, based on recent confirmations.",
		RecentPlaces:  2,
		WindowMinutes: 120,
		FadeStage:     p14Str("stale"),
	})
	p14AssertReporter(t, "step 3", rep, p14NewReporter())

	// Step 4: E confirms R1 from P5. Three confirming places meet the raised
	// bar, so the report reads Report up to date again.
	vis, vt = p07VoteTrust(t, e, srv.URL, r1, "confirm", p5Lat, p5Lon)
	if vis != "live" {
		t.Fatalf("step 4: visibility = %q, want live", vis)
	}
	p14Step(t, "step 4", vt,
		p14FeedTrustOrFail(t, "step 4 E", e, srv.URL, r1),
		p14FeedTrustOrFail(t, "step 4 A", a, srv.URL, r1))
	cur, rep = p14DecodeSignals(t, vt)
	p14UpToDate(t, "step 4", cur, 3, 120, "2 hours", p14Str("fresh"))
	p14AssertReporter(t, "step 4", rep, p14NewReporter())

	// Step 5: B confirms the critical R2 from P3. Critical reports bypass the
	// provisional state, and one place is too early to tell.
	vis, vt = p07VoteTrust(t, b, srv.URL, r2, "confirm", p3Lat, p3Lon)
	if vis != "live" {
		t.Fatalf("step 5: visibility = %q, want live", vis)
	}
	p14Step(t, "step 5", vt,
		p14FeedTrustOrFail(t, "step 5 B", b, srv.URL, r2),
		p14FeedTrustOrFail(t, "step 5 A", a, srv.URL, r2))
	cur, rep = p14DecodeSignals(t, vt)
	p14AssertCurrency(t, "step 5", cur, p14CurrencyWire{
		Band:          "not_yet_corroborated",
		Label:         "Too early to tell",
		Sentence:      "Fewer than 2 places have confirmed this.",
		RecentPlaces:  1,
		WindowMinutes: 360,
	})
	p14AssertReporter(t, "step 5", rep, p14NewReporter())

	// Step 6: C confirms R2 from P2. Critical rows never fade (D-07), so the
	// fade stage stays null.
	_, vt = p07VoteTrust(t, c, srv.URL, r2, "confirm", p2Lat, p2Lon)
	p14Step(t, "step 6", vt,
		p14FeedTrustOrFail(t, "step 6 C", c, srv.URL, r2),
		p14FeedTrustOrFail(t, "step 6 A", a, srv.URL, r2))
	cur, rep = p14DecodeSignals(t, vt)
	p14UpToDate(t, "step 6", cur, 2, 360, "6 hours", nil)
	p14AssertReporter(t, "step 6", rep, p14NewReporter())
}

const (
	// p14HistoryCellA and p14HistoryCellB are the two voter cells written on
	// the seeded history votes. Neither may appear on the wire.
	p14HistoryCellA = "tdr1v8y"
	p14HistoryCellB = "tdr1vc3"
)

// TestReporterSignalFromSeededHistory is D-08 (reporter half) on real history:
// one account with six expired reports over two sessions reads Reliable
// reporter on the feed and on the vote response alike, a fresh account reads
// New reporter, an accountless report reads none, and a report expired 91 days
// ago is not counted.
//
// Provenance of the numbers: six equal weight outcomes (all expire at one
// instant) give p = 5.25 / 6 = 0.875 and a raw score of 78.125, rounded 78. A
// leaked 91 day row would show as judged 7 and "5 of 7". Keying history by
// session instead of account would leave the first session's four corroborated
// reports, J = 4, and read New reporter.
func TestReporterSignalFromSeededHistory(t *testing.T) {
	clock := p07FixedClock()
	srv, mailer, pool := newTrustE2EServer(t, clock)
	now := clock()
	ctx := context.Background()

	const (
		aEmail = "p14-history-reporter@example.com"
		rEmail = "p14-history-reader@example.com"
	)
	a := newVerifiedClient(t, srv, mailer, aEmail)
	f := newVerifiedClient(t, srv, mailer, "p14-history-fresh@example.com")
	r := newVerifiedClient(t, srv, mailer, rEmail)

	// Fail fast on identity: A's session must be bound to an account, then a
	// second device session joins the same account.
	aSession := p07SessionID(t, a, srv.URL)
	var accountID *int64
	if err := pool.QueryRow(ctx, "SELECT account_id FROM sessions WHERE session_id = $1", aSession).Scan(&accountID); err != nil {
		t.Fatalf("reading the account of A's session: %v", err)
	}
	if accountID == nil {
		t.Fatalf("A's session %q has no account id", aSession)
	}
	const secondDevice = "p14-history-second-device"
	testutil.MustExec(t, pool, "INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)", secondDevice, *accountID)

	seedVoterA := testutil.SeedReporterAccount(t, pool, "p14-history-voter-a@example.com")
	seedVoterB := testutil.SeedReporterAccount(t, pool, "p14-history-voter-b@example.com")

	// Six expired reports in the 90 day window, all expiring at one instant:
	// four on A's session, two on the second device. Five are corroborated by
	// the two seed voters from two cells, the sixth has no vote.
	expiresAt := now.Add(-time.Hour)
	sessions := []string{aSession, aSession, aSession, aSession, secondDevice, secondDevice}
	for i, sid := range sessions {
		id := testutil.SeedHistoryReport(t, pool, sid, "low", "flood", expiresAt)
		if i == len(sessions)-1 {
			continue
		}
		for _, v := range []struct {
			account int64
			cell    string
		}{{seedVoterA, p14HistoryCellA}, {seedVoterB, p14HistoryCellB}} {
			testutil.MustExec(t, pool,
				`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at) VALUES ($1, $2, 'content', 'confirm', $3, $4)`,
				id, v.account, v.cell, expiresAt.Add(-2*time.Hour))
		}
	}

	// One more report that expired 91 days ago, with disputes. It is outside
	// the 90 day window and must not count.
	oldExpiry := now.Add(-91 * 24 * time.Hour)
	oldID := testutil.SeedHistoryReport(t, pool, aSession, "low", "flood", oldExpiry)
	for _, v := range []struct {
		account int64
		cell    string
	}{{seedVoterA, p14HistoryCellA}, {seedVoterB, p14HistoryCellB}} {
		testutil.MustExec(t, pool,
			`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at) VALUES ($1, $2, 'content', 'dispute', $3, $4)`,
			oldID, v.account, v.cell, oldExpiry.Add(-2*time.Hour))
	}

	ra := submitReportGetID(t, a, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	rf := submitReportGetID(t, f, srv.URL, floodReportBody(p07ReportLat, p07ReportLon))
	ro := testutil.SeedHistoryReport(t, pool, "p14-orphan-session", "low", "flood", now.Add(7*time.Hour))

	reliable := p14ReporterWire{
		Tag:                 "reliable",
		Score:               p14Int(78),
		Label:               "Reliable reporter",
		Sentence:            "5 of 6 earlier reports were confirmed by people nearby.",
		ScoreSentence:       "Score 78 out of 100, based on how often their reports were confirmed.",
		JudgedReports:       6,
		CorroboratedReports: 5,
	}
	want := map[int64]p14ReporterWire{ra: reliable, rf: p14NewReporter(), ro: p14NoneReporter()}
	names := map[int64]string{ra: "RA", rf: "RF", ro: "RO"}

	// The feed read by R lists all three and carries the expected reporters.
	for id, exp := range want {
		tr := p14FeedTrustOrFail(t, names[id]+" before votes", r, srv.URL, id)
		p14AssertSignalKeys(t, names[id]+" before votes", tr)
		_, rep := p14DecodeSignals(t, tr)
		p14AssertReporter(t, names[id]+" before votes", rep, exp)
	}

	p3Lat, p3Lon := 12.9746, 77.5946
	p2Lat, p2Lon := 12.9686, 77.5946
	p4Lat, p4Lon := 12.9716, 77.5986
	votes := []struct {
		id       int64
		lat, lon float64
	}{{ra, p3Lat, p3Lon}, {rf, p2Lat, p2Lon}, {ro, p4Lat, p4Lon}}
	var raVoteBody []byte
	for _, v := range votes {
		status, body, err := p07PostVoteRaw(r, srv.URL, v.id, "confirm", v.lat, v.lon)
		if err != nil || status != http.StatusOK {
			t.Fatalf("%s confirm: status %d, err %v, body %s", names[v.id], status, err, body)
		}
		var out struct {
			Visibility string          `json:"visibility"`
			Trust      json.RawMessage `json:"trust"`
		}
		if err := json.Unmarshal(body, &out); err != nil || len(out.Trust) == 0 {
			t.Fatalf("%s confirm: no trust in %s (%v)", names[v.id], body, err)
		}
		_, rep := p14DecodeSignals(t, out.Trust)
		p14AssertReporter(t, names[v.id]+" vote response", rep, want[v.id])
		p14Step(t, names[v.id]+" vote", out.Trust,
			p14FeedTrustOrFail(t, names[v.id]+" feed R", r, srv.URL, v.id),
			p14FeedTrustOrFail(t, names[v.id]+" feed A", a, srv.URL, v.id))
		if v.id == ra {
			raVoteBody = body
			// D-14: the Reliable rating is display only. One confirming
			// place leaves the report provisional.
			if out.Visibility != "provisional" {
				t.Fatalf("RA visibility = %q after one confirming place, want provisional", out.Visibility)
			}
		}
	}

	// V-31 with a rated reporter: no identifier key at any depth and none of
	// the private values anywhere in the raw text.
	banned := []string{aEmail, rEmail, "@", aSession, secondDevice, p14HistoryCellA, p14HistoryCellB, "account_id", "geohash_cell"}
	bodies := []struct {
		label string
		raw   []byte
	}{
		{"vote response for RA", raVoteBody},
		{"feed read by the reader", p07GetFeedRaw(t, r, srv.URL, false)},
	}
	for _, bd := range bodies {
		var decoded any
		if err := json.Unmarshal(bd.raw, &decoded); err != nil {
			t.Fatalf("%s: not JSON: %v (body %s)", bd.label, err, bd.raw)
		}
		p07WalkJSONKeys(t, bd.label, decoded)
		text := string(bd.raw)
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s contains %q: %s", bd.label, b, bd.raw)
			}
		}
	}
}
