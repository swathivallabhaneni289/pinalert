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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
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
// appends currency and reporter to this one list, so it stays a single
// package level slice.
var p07TrustWireKeys = []string{
	"confirmed_places", "disputed_places", "confirmed_text", "disputed_text",
	"lead_sentence", "confirmed_sentence", "disputed_sentence",
}

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
	p07Lead           = "Needs 2 confirmations before this counts as confirmed."
	p07NoDispute      = "No one nearby has disputed this. Counts separate places, not votes."
	p07Confirmed1     = "1 person in 1 place confirmed this. Counts separate places, not votes."
	p07Confirmed2     = "2 people in 2 different places confirmed this. Counts separate places, not votes."
	p07Confirmed3     = "3 people in 3 different places confirmed this. Counts separate places, not votes."
	p07Disputed1      = "1 person in 1 place disputed this. Counts separate places, not votes."
	p07Disputed2      = "2 people in 2 different places disputed this. Counts separate places, not votes."
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
