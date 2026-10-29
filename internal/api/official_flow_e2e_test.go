package api

// TestPhase5OfficialJourney walks the whole official path of Phase 5 on a real
// database through the real router: a fake SACHET, the protected refresh call,
// the reader's alerts and banner, a phone push queued for a home in the same
// district and sent by the worker to a fake push service, and then the feed
// ageing into stale and unavailable. It crosses plans 05-05 (storage and
// refresh), 05-08 (the reader's view), 05-11 (the push queue), 05-12 (the
// refresh door) and 05-16 (the worker), so a seam that drifts between two of
// them fails here even when each plan's own tests still pass.
//
// No real SACHET host and no real push service is contacted. The SACHET client
// has no exported seam for a test upstream, so the test points the process wide
// default HTTP transport at an in process fake for the one host name and
// restores it when the test ends. Nothing else in this package runs in parallel
// with it.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api/handlers"
	"pinalert/internal/areas"
	"pinalert/internal/official"
	"pinalert/internal/push"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	journeyToken    = "journey-test-refresh-token-0123456789abcdef"
	journeyGUID     = "1792915039199001"
	journeyEndpoint = "https://fcm.googleapis.com/fcm/send/journey-device-1"
	// Vijayawada, inside district 749 in the boundary file.
	journeyLat = 16.5062
	journeyLon = 80.6480
)

// journeyUpstream is the fake SACHET: an RSS list with one item and the CAP
// message it points to, both built from the clock so the alert is recent.
type journeyUpstream struct {
	mu   sync.Mutex
	hits int
	rss  string
	cap  string
}

func newJourneyUpstream(now time.Time) *journeyUpstream {
	sent := now.Add(-5 * time.Minute).UTC()
	expires := now.Add(3 * time.Hour).UTC()
	stamp := func(t time.Time) string { return t.Format("2006-01-02T15:04:05+00:00") }
	cap := `<cap:alert xmlns:cap="urn:oasis:names:tc:emergency:cap:1.2">
<cap:identifier>IN-` + journeyGUID + `_1</cap:identifier>
<cap:sender>IMD-Chennai</cap:sender>
<cap:sent>` + stamp(sent) + `</cap:sent>
<cap:status>Actual</cap:status>
<cap:msgType>Alert</cap:msgType>
<cap:scope>Public</cap:scope>
<cap:info>
<cap:language>en-IN</cap:language>
<cap:category>Met</cap:category>
<cap:event>Severe Cyclonic Storm</cap:event>
<cap:urgency>Expected</cap:urgency>
<cap:severity>Extreme</cap:severity>
<cap:certainty>Likely</cap:certainty>
<cap:effective>` + stamp(sent) + `</cap:effective>
<cap:onset>` + stamp(sent) + `</cap:onset>
<cap:expires>` + stamp(expires) + `</cap:expires>
<cap:headline>Severe cyclonic storm expected to cross the coast near Krishna district in the next 3 hours.</cap:headline>
<cap:description/>
<cap:instruction>Please follow SDMA guidelines.</cap:instruction>
<cap:area>
<cap:areaDesc>Ntr district of Andhra Pradesh</cap:areaDesc>
<cap:geocode>
<cap:valueName>LGD District Code</cap:valueName>
<cap:value>749</cap:value>
</cap:geocode>
</cap:area>
</cap:info>
</cap:alert>`
	rss := `<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>t</title>` +
		`<item><title>x</title><link>https://example.invalid/x</link>` +
		`<author>controlroom@ndma.gov.in (IMD Chennai)</author>` +
		`<guid isPermaLink="false">` + journeyGUID + `</guid>` +
		`<pubDate>` + sent.Format("Mon, 02 Jan 2006 15:04:05") + ` GMT</pubDate></item>` +
		`</channel></rss>`
	return &journeyUpstream{rss: rss, cap: cap}
}

func (u *journeyUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u.mu.Lock()
	u.hits++
	u.mu.Unlock()
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	switch {
	case strings.HasSuffix(r.URL.Path, "rss_india.xml"):
		_, _ = io.WriteString(w, u.rss)
	case strings.HasSuffix(r.URL.Path, "FetchXMLFile") && r.URL.Query().Get("identifier") == journeyGUID:
		_, _ = io.WriteString(w, u.cap)
	default:
		http.NotFound(w, r)
	}
}

func (u *journeyUpstream) total() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits
}

// sachetHostSwap sends requests for the SACHET host name to the fake upstream
// and everything else to the transport that was in place.
//
// Amended 2026-10-29 by plan 05-21: the real service now registers the GDACS
// feed too, so the refresh door calls www.gdacs.org as well. That host is
// answered here, in process, with an empty feature collection and counted in
// gdacsHits, so the journey never reaches the real GDACS either. Every older
// line of the swap is unchanged.
type sachetHostSwap struct {
	target    *url.URL
	next      http.RoundTripper
	gdacsHits atomic.Int32
}

func (s *sachetHostSwap) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "www.gdacs.org" {
		s.gdacsHits.Add(1)
		body := `{"type":"FeatureCollection","features":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Proto:      "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header:        http.Header{"Content-Type": []string{"application/json"}},
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
			Request:       req,
		}, nil
	}
	if req.URL.Host != "sachet.ndma.gov.in" {
		return s.next.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = s.target.Host
	clone.Host = s.target.Host
	return s.next.RoundTrip(clone)
}

// journeySender is a push.Sender that records every message and answers 201.
type journeySender struct {
	mu   sync.Mutex
	sent []journeySent
}

type journeySent struct {
	endpoint string
	msg      push.Message
}

func (s *journeySender) Send(_ context.Context, sub push.Subscription, msg push.Message) (push.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, journeySent{endpoint: sub.Endpoint, msg: msg})
	return push.Result{Status: 201}, nil
}

func (s *journeySender) messages() []journeySent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]journeySent(nil), s.sent...)
}

// journeyAlerts is the part of GET /api/official/alerts the journey reads.
type journeyAlerts struct {
	State  string `json:"state"`
	Count  int    `json:"count"`
	Groups []struct {
		Kind string `json:"kind"`
		Rows []struct {
			Level     string `json:"level"`
			LevelWord string `json:"level_word"`
			Badge     string `json:"badge"`
			Demo      bool   `json:"demo"`
			Issuer    string `json:"issuer"`
			IssuedAt  string `json:"issued_at"`
			ExpiresAt string `json:"expires_at"`
		} `json:"rows"`
	} `json:"groups"`
	Banner *struct {
		Level       string `json:"level"`
		EventPhrase string `json:"event_phrase"`
		Area        string `json:"area"`
	} `json:"banner"`
}

func journeyGet(t *testing.T, client *http.Client, base string) journeyAlerts {
	t.Helper()
	resp, err := client.Get(fmt.Sprintf("%s/api/official/alerts?lat=%v&lon=%v", base, journeyLat, journeyLon))
	if err != nil {
		t.Fatalf("GET /api/official/alerts: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/official/alerts = %d, want 200: %s", resp.StatusCode, raw)
	}
	var out journeyAlerts
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decoding the answer: %v (%s)", err, raw)
	}
	return out
}

func journeyRows(a journeyAlerts) int {
	n := 0
	for _, g := range a.Groups {
		n += len(g.Rows)
	}
	return n
}

// journeyCell finds a precision 5 cell that lies inside exactly one district,
// so a home area in it is unambiguously in that district.
func journeyCell(t *testing.T, ix *areas.Index, code int) string {
	t.Helper()
	d, ok := ix.ByCode(code)
	if !ok {
		t.Fatalf("district %d is not in the boundary file", code)
	}
	for lat := d.BBox[1]; lat <= d.BBox[3]; lat += 0.01 {
		for lon := d.BBox[0]; lon <= d.BBox[2]; lon += 0.01 {
			cell := geohash.EncodeWithPrecision(lat, lon, 5)
			if ds := ix.CellDistricts(cell); len(ds) == 1 && ds[0].Code == code {
				return cell
			}
		}
	}
	t.Fatalf("no interior cell found for district %d", code)
	return ""
}

func journeyCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

func TestPhase5OfficialJourney(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ix := areas.Default()
	now := time.Now().UTC()

	// The fake SACHET, reached only through the swapped transport.
	upstream := newJourneyUpstream(now)
	fake := httptest.NewServer(upstream)
	t.Cleanup(fake.Close)
	target, err := url.Parse(fake.URL)
	if err != nil {
		t.Fatalf("parsing the fake upstream address: %v", err)
	}
	original := http.DefaultTransport
	swap := &sachetHostSwap{target: target, next: original}
	http.DefaultTransport = swap
	t.Cleanup(func() { http.DefaultTransport = original })

	// The real router, with the real official service and the refresh door open.
	mgr, err := session.NewManager([]byte("journey-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page template: %v", err)
	}
	queries := sqlcgen.New(pool)
	mailer := &gateFakeMailer{}
	router := NewRouter(Deps{
		Session:     mgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, mailer, "https://pinalert.example"),
		Auth:        handlers.AuthConfig{AssetVersion: "test"},
		Template:    tmpl,
		RateLimits:  gateGenerousRateLimits,
		// Amended 2026-10-29 by Phase 7.1 plan 08: the reader signs in with the
		// password flow through gateVerifySession.
		Credentials:          newGateTestCredentials(queries, mailer),
		AuthLimits:           gateGenerousAuthLimits,
		Official:             official.NewService(pool, queries, official.Config{}),
		OfficialRefreshToken: journeyToken,
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// The people, all seeded before the refresh, because the push hook queues
	// inside the refresh's own transaction. The reader signs in through the real
	// link flow. The person to be told has a home area inside district 749 and one
	// device on a Google endpoint (the Apple rule cannot interfere).
	reader := newGateJarClient(t)
	gateVerifySession(t, reader, srv, mailer, "journey-reader@example.com")
	account := testutil.SeedReporterAccount(t, pool, "journey-home@example.com", "journey-home-sess")
	testutil.SeedPushSubscription(t, pool, account, "journey-home-sess", journeyEndpoint)
	testutil.SetHomeCell(t, pool, account, "Home", journeyCell(t, ix, 749))

	reports := journeyCount(t, pool, `SELECT count(*) FROM reports`)
	votes := journeyCount(t, pool, `SELECT count(*) FROM votes`)
	sessions := journeyCount(t, pool, `SELECT count(*) FROM sessions`)

	// 1. The protected refresh call stores the alert.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/internal/official/refresh", nil)
	if err != nil {
		t.Fatalf("building the refresh request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+journeyToken)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("POST /internal/official/refresh: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh = %d, want 200: %s", resp.StatusCode, body)
	}
	var summary struct {
		Feeds []struct {
			Feed    string `json:"feed"`
			Skipped bool   `json:"skipped"`
			Stored  int    `json:"stored"`
			Error   string `json:"error"`
		} `json:"feeds"`
	}
	if err := json.Unmarshal(body, &summary); err != nil {
		t.Fatalf("decoding the refresh summary: %v (%s)", err, body)
	}
	// Amended 2026-10-29 by plan 05-21: the summary lists SACHET and then GDACS,
	// the service's two registered feeds. The SACHET check is the one written
	// before, unchanged; the GDACS entry must be a run (not skipped) with no
	// error, stored nothing (the fake GDACS holds no event) and made exactly its
	// two SEARCH requests.
	if len(summary.Feeds) != 2 {
		t.Fatalf("refresh summary = %s, want two feeds, sachet then gdacs", body)
	}
	if summary.Feeds[0].Feed != "sachet" || summary.Feeds[0].Skipped ||
		summary.Feeds[0].Error != "" || summary.Feeds[0].Stored < 1 {
		t.Fatalf("refresh summary = %s, want sachet run, not skipped, no error, at least 1 stored", body)
	}
	if g := summary.Feeds[1]; g.Feed != "gdacs" || g.Skipped || g.Error != "" || g.Stored != 0 {
		t.Fatalf("refresh summary = %s, want gdacs run, not skipped, no error, nothing stored", body)
	}
	if n := swap.gdacsHits.Load(); n != 2 {
		t.Fatalf("%d GDACS requests for one refresh, want 2", n)
	}
	if upstream.total() == 0 {
		t.Fatal("the fake upstream was never reached: the swap did not take")
	}

	// 2. The reader's alerts and banner for district 749.
	fresh := journeyGet(t, reader, srv.URL)
	if fresh.State != "fresh" {
		t.Fatalf("state after the refresh = %q, want fresh", fresh.State)
	}
	if len(fresh.Groups) == 0 || fresh.Groups[0].Kind != "reader" || len(fresh.Groups[0].Rows) != 1 {
		t.Fatalf("groups = %+v, want one reader group with the one alert", fresh.Groups)
	}
	row := fresh.Groups[0].Rows[0]
	if row.Level != "extreme" || row.Badge != "Official" || row.Demo {
		t.Errorf("row = %+v, want an Extreme alert with the Official badge and no Demo flag", row)
	}
	if row.Issuer == "" || row.IssuedAt == "" || row.ExpiresAt == "" {
		t.Errorf("row = %+v, want the issuer, the issued time and the end time", row)
	}
	if fresh.Banner == nil || fresh.Banner.Level != "extreme" || fresh.Banner.EventPhrase != "Cyclone warning" {
		t.Fatalf("banner = %+v, want an Extreme Cyclone warning", fresh.Banner)
	}

	// 3. Exactly one phone push is queued for the home in 749.
	if n := journeyCount(t, pool,
		`SELECT count(*) FROM official_push_outbox WHERE account_id = $1 AND state = 'queued'`, account); n != 1 {
		t.Fatalf("%d queued official pushes for the home in 749, want 1", n)
	}

	// 4. The worker sends it, silently, to the fake push service.
	sender := &journeySender{}
	worker := service.NewOfficialPushWorker(queries, sender, ix)
	if err := worker.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	msgs := sender.messages()
	if len(msgs) != 1 || msgs[0].endpoint != journeyEndpoint {
		t.Fatalf("messages = %+v, want one to the person's device", msgs)
	}
	p := msgs[0].msg.Payload
	district, _ := ix.ByCode(749)
	if p.Kind != "official" || p.URL != "/alerts" || p.Tag != service.OfficialTag(journeyGUID) {
		t.Errorf("payload = %+v, want kind official, url /alerts and the alert's tag", p)
	}
	if want := "Cyclone warning for " + district.Name + " district"; p.Title != want {
		t.Errorf("title = %q, want the closed words %q", p.Title, want)
	}
	if p.Body != "Extreme. Issued by IMD." {
		t.Errorf("body = %q, want the closed words Extreme. Issued by IMD.", p.Body)
	}
	if msgs[0].msg.Urgency != push.UrgencyHigh || msgs[0].msg.Topic != service.OfficialTopic(journeyGUID) {
		t.Errorf("urgency %v topic %q, want high and the alert's topic", msgs[0].msg.Urgency, msgs[0].msg.Topic)
	}
	wire, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("encoding the payload: %v", err)
	}
	for _, loud := range []string{"sound", "vibrate"} {
		if strings.Contains(strings.ToLower(string(wire)), loud) {
			t.Errorf("payload %s names %q: the sound and vibration choice belongs to the service worker alone", wire, loud)
		}
	}
	if err := worker.SweepOnce(context.Background()); err != nil {
		t.Fatalf("second SweepOnce: %v", err)
	}
	if n := len(sender.messages()); n != 1 {
		t.Errorf("%d pushes after a second sweep, want still 1", n)
	}

	// 5. The feed ages. The last attempt stays at this moment so the read's lazy
	// refresh never starts; only the last good check moves back.
	age := func(ago time.Duration) {
		t.Helper()
		attempt := time.Now().UTC()
		success := attempt.Add(-ago)
		testutil.SetOfficialFeedStatus(t, pool, "sachet", &attempt, &success, 1)
	}
	hitsBefore := upstream.total()

	age(2 * time.Hour)
	stale := journeyGet(t, reader, srv.URL)
	if stale.State != "stale" {
		t.Fatalf("state with the last good check 2 hours back = %q, want stale", stale.State)
	}
	if journeyRows(stale) != 1 {
		t.Errorf("stale view holds %d rows, want the one last known alert kept", journeyRows(stale))
	}
	if stale.Banner != nil {
		t.Errorf("stale banner = %+v, want none: the feed cannot vouch for it", stale.Banner)
	}

	age(7 * time.Hour)
	gone := journeyGet(t, reader, srv.URL)
	if gone.State != "unavailable" {
		t.Fatalf("state with the last good check 7 hours back = %q, want unavailable", gone.State)
	}
	if journeyRows(gone) != 0 || gone.Count != 0 || gone.Banner != nil {
		t.Errorf("unavailable view = %+v, want no rows, no count and no banner", gone)
	}
	if upstream.total() != hitsBefore {
		t.Errorf("a read reached the fake upstream %d times: the ageing steps must not refresh", upstream.total()-hitsBefore)
	}

	// 6. The whole official path touched no report, vote or session.
	if got := journeyCount(t, pool, `SELECT count(*) FROM reports`); got != reports {
		t.Errorf("reports went from %d to %d", reports, got)
	}
	if got := journeyCount(t, pool, `SELECT count(*) FROM votes`); got != votes {
		t.Errorf("votes went from %d to %d", votes, got)
	}
	if got := journeyCount(t, pool, `SELECT count(*) FROM sessions`); got != sessions {
		t.Errorf("sessions went from %d to %d", sessions, got)
	}
}
