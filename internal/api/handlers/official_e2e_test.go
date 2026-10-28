// official_e2e_test.go proves GET /api/official/alerts end to end over a real
// Postgres and the real router: the reader's groups, banner and honest feed
// state, the lazy refresh, and that a position is never written to a log. No
// test here reaches NDMA: every service is given a fake SACHET fetcher that
// counts its runs, and the tests that must not refresh keep the last attempt
// recent.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/api"
	"pinalert/internal/areas"
	"pinalert/internal/official"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// fakeSachet stands in for the SACHET fetcher. It counts runs and can take its
// time, so a test can tell that a read did not wait for it.
type fakeSachet struct {
	calls atomic.Int32
	delay time.Duration
}

func (f *fakeSachet) Fetch(_ context.Context, _ official.FeedStatus) (official.Result, error) {
	f.calls.Add(1)
	time.Sleep(f.delay)
	return official.Result{ItemsSeen: 3}, nil
}

// lockedBuffer is a bytes.Buffer safe to write from the background refresh
// while a test reads it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// officialJSON is the wire shape as a client sees it, declared here so the test
// asserts the JSON contract and not the server's own types.
type officialJSON struct {
	State         string  `json:"state"`
	AgeSeconds    *int64  `json:"age_seconds"`
	LastSuccessAt *string `json:"last_success_at"`
	AreaKnown     bool    `json:"area_known"`
	Count         int     `json:"count"`
	DistrictsLine string  `json:"districts_line"`
	Groups        []struct {
		Kind  string `json:"kind"`
		Label string `json:"label"`
		Rows  []struct {
			ID           int64  `json:"id"`
			Source       string `json:"source"`
			Level        string `json:"level"`
			LevelWord    string `json:"level_word"`
			Badge        string `json:"badge"`
			Demo         bool   `json:"demo"`
			Headline     string `json:"headline"`
			Issuer       string `json:"issuer"`
			IssuedAt     string `json:"issued_at"`
			ExpiresAt    string `json:"expires_at"`
			GroupedCount int    `json:"grouped_count"`
			Details      struct {
				SourceLine string `json:"source_line"`
			} `json:"details"`
		} `json:"rows"`
	} `json:"groups"`
	Banner *struct {
		Level       string `json:"level"`
		LevelWord   string `json:"level_word"`
		EventPhrase string `json:"event_phrase"`
		Area        string `json:"area"`
		More        int    `json:"more"`
		AlertID     int64  `json:"alert_id"`
	} `json:"banner"`
}

// newOfficialServer builds the e2e server with an official.Service that has the
// given fake SACHET fetcher registered. tweak runs after, for the request log.
func newOfficialServer(t *testing.T, f official.Fetcher, tweak func(d *api.Deps)) (*httptest.Server, *recordingMailer, *pgxpool.Pool) {
	t.Helper()
	return newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		// The fake never saves a version, so the service needs no transaction
		// source; the refresher's status, claim and success go through q.
		d.Official = official.NewService(nil, q, official.Config{}, func(r *official.Refresher) {
			r.Register(official.FeedSachet, f, 0)
		})
		if tweak != nil {
			tweak(d)
		}
	})
}

func getOfficial(t *testing.T, client *http.Client, baseURL, query string) (int, officialJSON, string) {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/official/alerts" + query)
	if err != nil {
		t.Fatalf("GET /api/official/alerts: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}
	var out officialJSON
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decoding the body: %v (%s)", err, raw)
		}
	}
	return resp.StatusCode, out, string(raw)
}

func officialFeed(t *testing.T, pool *pgxpool.Pool, attemptAgo, successAgo time.Duration, haveSuccess bool) {
	t.Helper()
	now := time.Now().UTC()
	attempt := now.Add(-attemptAgo)
	var success *time.Time
	if haveSuccess {
		s := now.Add(-successAgo)
		success = &s
	}
	testutil.SetOfficialFeedStatus(t, pool, "sachet", &attempt, success, 20)
}

func groupKindList(o officialJSON) []string {
	out := []string{}
	for _, g := range o.Groups {
		out = append(out, g.Kind)
	}
	return out
}

const (
	vijayawadaQuery = "?lat=16.5062&lon=80.6480"
	districtNtr     = 749
	stateAP         = 28
)

func districtName(t *testing.T, code int) string {
	t.Helper()
	d, ok := areas.Default().ByCode(code)
	if !ok {
		t.Fatalf("no district %d", code)
	}
	return d.Name
}

func TestOfficialAlertsRoute(t *testing.T) {
	f := &fakeSachet{}
	srv, mailer, pool := newOfficialServer(t, f, nil)
	client := newVerifiedClient(t, srv, mailer, "official-route@example.com")
	officialFeed(t, pool, time.Minute, 4*time.Minute, true)

	now := time.Now().UTC()
	extreme := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
		Tier: "district", DistrictCodes: []int{districtNtr}, StateCodes: []int{stateAP},
		Severity: "Extreme", Event: "Severe Cyclonic Storm", HeadlineEN: "Cyclone landfall expected",
		Sender: "IMD-Chennai", Issuer: "IMD Chennai", AreaDesc: "NTR",
		SentAt: now.Add(-30 * time.Minute), ExpiresAt: now.Add(3 * time.Hour),
	})
	severe := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
		Tier: "district", DistrictCodes: []int{districtNtr}, StateCodes: []int{stateAP},
		Severity: "Severe", Event: "Heavy Rain", HeadlineEN: "Heavy rain expected",
		Sender: "IMD-Chennai", Issuer: "IMD Chennai", AreaDesc: "NTR",
		SentAt: now.Add(-10 * time.Minute), ExpiresAt: now.Add(2 * time.Hour),
	})
	stateSevere := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
		Tier: "state", StateCodes: []int{stateAP},
		Severity: "Severe", Event: "Lightning", HeadlineEN: "Lightning likely",
		Sender: "APSDMA", Issuer: "Andhra Pradesh SDMA", AreaDesc: "Andhra Pradesh",
		SentAt: now.Add(-5 * time.Minute), ExpiresAt: now.Add(time.Hour),
	})
	expired := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
		Tier: "district", DistrictCodes: []int{districtNtr}, StateCodes: []int{stateAP},
		Severity: "Extreme", Event: "Flood", HeadlineEN: "An old flood warning",
		SentAt: now.Add(-5 * time.Hour), ExpiresAt: now.Add(-time.Minute),
	})

	status, got, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, raw)
	}
	if got.State != "fresh" || !got.AreaKnown {
		t.Fatalf("state %q areaKnown %v", got.State, got.AreaKnown)
	}
	if got.AgeSeconds == nil || *got.AgeSeconds < 230 || *got.AgeSeconds > 260 {
		t.Errorf("age_seconds = %v, want about 240", got.AgeSeconds)
	}
	if kinds := strings.Join(groupKindList(got), ","); kinds != "reader,state" {
		t.Fatalf("groups = %s", kinds)
	}
	wantReader := "Where you are: " + districtName(t, districtNtr) + " district"
	if got.Groups[0].Label != wantReader || got.Groups[1].Label != "Across Andhra Pradesh" {
		t.Errorf("labels = %q, %q", got.Groups[0].Label, got.Groups[1].Label)
	}
	reader := got.Groups[0].Rows
	if len(reader) != 2 || reader[0].ID != extreme || reader[1].ID != severe {
		t.Fatalf("reader rows = %+v, want the Extreme first then the Severe", reader)
	}
	if len(got.Groups[1].Rows) != 1 || got.Groups[1].Rows[0].ID != stateSevere {
		t.Errorf("state rows = %+v", got.Groups[1].Rows)
	}
	if got.Count != 3 {
		t.Errorf("count = %d, want 3", got.Count)
	}
	for _, g := range got.Groups {
		for _, r := range g.Rows {
			if r.ID == expired {
				t.Errorf("the expired alert %d is still listed", expired)
			}
			if r.Issuer == "" || r.IssuedAt == "" || r.ExpiresAt == "" || r.Badge != "Official" || r.Source != "sachet" || r.Details.SourceLine != "NDMA SACHET" {
				t.Errorf("row %d lacks a fact: %+v", r.ID, r)
			}
			if _, err := time.Parse(time.RFC3339, r.IssuedAt); err != nil {
				t.Errorf("issued_at %q: %v", r.IssuedAt, err)
			}
		}
	}
	if reader[0].Level != "extreme" || reader[0].LevelWord != "Extreme" || reader[0].Issuer != "IMD Chennai" {
		t.Errorf("first row = %+v", reader[0])
	}
	if got.Banner == nil {
		t.Fatal("no banner for an Extreme on the reader's district")
	}
	b := got.Banner
	if b.Level != "extreme" || b.EventPhrase != "Cyclone warning" || b.More != 1 || b.AlertID != extreme ||
		b.Area != districtName(t, districtNtr)+" district" {
		t.Errorf("banner = %+v", *b)
	}
	if got.DistrictsLine != districtName(t, districtNtr)+" district" {
		t.Errorf("districts_line = %q", got.DistrictsLine)
	}
	if n := f.calls.Load(); n != 0 {
		t.Errorf("the last attempt was recent, yet the fetcher ran %d times", n)
	}
}

func TestOfficialAlertsHomeArea(t *testing.T) {
	srv, mailer, pool := newOfficialServer(t, &fakeSachet{}, nil)
	const email = "official-home@example.com"
	client := newVerifiedClient(t, srv, mailer, email)
	officialFeed(t, pool, time.Minute, 4*time.Minute, true)

	// A home cell in another district than the reader's.
	ix := areas.Default()
	homeCell := geohash.EncodeWithPrecision(16.3067, 80.4365, 5)
	homeDistricts := ix.CellDistricts(homeCell)
	if len(homeDistricts) == 0 || homeDistricts[0].Code == districtNtr {
		t.Fatalf("test setup: the home cell lands in %+v", homeDistricts)
	}
	testutil.SetHomeCell(t, pool, p25AccountID(t, pool, email), "Home", homeCell)

	now := time.Now().UTC()
	homeAlert := testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
		Tier: "district", DistrictCodes: []int{homeDistricts[0].Code}, StateCodes: []int{stateAP},
		Severity: "Extreme", Event: "Flood", HeadlineEN: "Flood at home",
		SentAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	})

	status, got, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, raw)
	}
	if kinds := strings.Join(groupKindList(got), ","); kinds != "home" {
		t.Fatalf("groups = %s", kinds)
	}
	if got.Groups[0].Rows[0].ID != homeAlert {
		t.Errorf("rows = %+v", got.Groups[0].Rows)
	}
	if got.Banner != nil {
		t.Errorf("an alert for the home district only must not raise the banner: %+v", *got.Banner)
	}
	if strings.Contains(raw, homeCell) {
		t.Errorf("the response holds the home cell: %s", raw)
	}

	// With no position the home area still answers.
	status, got, raw = getOfficial(t, client, srv.URL, "")
	if status != http.StatusOK || !got.AreaKnown || len(got.Groups) != 1 {
		t.Fatalf("no position: status %d %s", status, raw)
	}
}

func TestOfficialAlertsHonest(t *testing.T) {
	srv, mailer, pool := newOfficialServer(t, &fakeSachet{}, nil)
	client := newVerifiedClient(t, srv, mailer, "official-honest@example.com")

	now := time.Now().UTC()
	seed := func() int64 {
		return testutil.SeedOfficialAlert(t, pool, testutil.OfficialAlertSeed{
			Tier: "district", DistrictCodes: []int{districtNtr}, StateCodes: []int{stateAP},
			Severity: "Extreme", Event: "Flood", HeadlineEN: "Flood warning",
			SentAt: now.Add(-30 * time.Minute), ExpiresAt: now.Add(3 * time.Hour),
		})
	}
	seed()

	t.Run("two hours old is stale: rows stay, no banner", func(t *testing.T) {
		officialFeed(t, pool, time.Minute, 2*time.Hour, true)
		status, got, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
		if status != http.StatusOK {
			t.Fatalf("status = %d: %s", status, raw)
		}
		if got.State != "stale" || got.Banner != nil || got.Count != 1 {
			t.Fatalf("state %q banner %v count %d", got.State, got.Banner, got.Count)
		}
		if got.AgeSeconds == nil || *got.AgeSeconds < 7190 || got.LastSuccessAt == nil {
			t.Errorf("age %v last %v", got.AgeSeconds, got.LastSuccessAt)
		}
	})

	t.Run("seven hours old is unavailable: no groups", func(t *testing.T) {
		officialFeed(t, pool, time.Minute, 7*time.Hour, true)
		status, got, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
		if status != http.StatusOK {
			t.Fatalf("status = %d: %s", status, raw)
		}
		if got.State != "unavailable" || len(got.Groups) != 0 || got.Count != 0 || got.Banner != nil {
			t.Fatalf("answer = %+v", got)
		}
		if got.LastSuccessAt == nil {
			t.Errorf("the last good time must be sent")
		}
		if !strings.Contains(raw, `"groups":[]`) {
			t.Errorf("groups must be an empty list: %s", raw)
		}
	})

	t.Run("never checked is unavailable with no time", func(t *testing.T) {
		officialFeed(t, pool, time.Minute, 0, false)
		status, got, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
		if status != http.StatusOK {
			t.Fatalf("status = %d: %s", status, raw)
		}
		if got.State != "unavailable" || got.LastSuccessAt != nil || got.AgeSeconds != nil || len(got.Groups) != 0 {
			t.Fatalf("answer = %+v", got)
		}
	})

	t.Run("only a fresh answer can be an empty list", func(t *testing.T) {
		if _, err := pool.Exec(context.Background(), `DELETE FROM official_alert_areas`); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM official_alerts`); err != nil {
			t.Fatal(err)
		}
		officialFeed(t, pool, time.Minute, 3*time.Minute, true)
		status, got, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
		if status != http.StatusOK {
			t.Fatalf("status = %d: %s", status, raw)
		}
		if got.State != "fresh" || got.Count != 0 || len(got.Groups) != 0 || !got.AreaKnown {
			t.Fatalf("fresh and empty = %+v", got)
		}
		// The same empty list in the other two states says so by its state.
		for _, ago := range []time.Duration{2 * time.Hour, 8 * time.Hour} {
			officialFeed(t, pool, time.Minute, ago, true)
			_, other, _ := getOfficial(t, client, srv.URL, vijayawadaQuery)
			if other.State == "fresh" {
				t.Errorf("a feed last checked %v ago must not read as fresh", ago)
			}
		}
	})

	t.Run("no position and no home area says the area is unknown", func(t *testing.T) {
		officialFeed(t, pool, time.Minute, 3*time.Minute, true)
		status, got, raw := getOfficial(t, client, srv.URL, "")
		if status != http.StatusOK {
			t.Fatalf("status = %d: %s", status, raw)
		}
		if got.AreaKnown || got.State != "fresh" || len(got.Groups) != 0 {
			t.Fatalf("answer = %+v", got)
		}
	})

	t.Run("a bad position is refused", func(t *testing.T) {
		status, _, _ := getOfficial(t, client, srv.URL, "?lat=95&lon=80")
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", status)
		}
		status, _, _ = getOfficial(t, client, srv.URL, "?lat=16.5")
		if status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", status)
		}
	})
}

func TestOfficialAlertsKicksRefresh(t *testing.T) {
	f := &fakeSachet{delay: 800 * time.Millisecond}
	srv, mailer, pool := newOfficialServer(t, f, nil)
	client := newVerifiedClient(t, srv, mailer, "official-kick@example.com")

	// The last attempt is 11 minutes old, so a read is due to refresh.
	officialFeed(t, pool, 11*time.Minute, 11*time.Minute, true)
	before := time.Now().UTC().Add(-time.Second)

	begin := time.Now()
	status, _, raw := getOfficial(t, client, srv.URL, vijayawadaQuery)
	took := time.Since(begin)
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, raw)
	}
	if took > 600*time.Millisecond {
		t.Fatalf("the read took %v: it waited for the refresh", took)
	}

	// A second read at once starts nothing.
	if status, _, raw := getOfficial(t, client, srv.URL, vijayawadaQuery); status != http.StatusOK {
		t.Fatalf("second read status = %d: %s", status, raw)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		var attempt time.Time
		var success *time.Time
		if err := pool.QueryRow(context.Background(),
			`SELECT last_attempt_at, last_success_at FROM official_feed_status WHERE feed = 'sachet'`).Scan(&attempt, &success); err != nil {
			t.Fatalf("reading the feed status: %v", err)
		}
		if attempt.After(before) && success != nil && success.After(before) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the feed status never showed a new attempt: attempt %v success %v", attempt, success)
		}
		time.Sleep(25 * time.Millisecond)
	}

	if n := f.calls.Load(); n != 1 {
		t.Fatalf("the fetcher ran %d times, want 1", n)
	}
	// Once more after the run: the attempt is now recent, so nothing starts.
	getOfficial(t, client, srv.URL, vijayawadaQuery)
	time.Sleep(100 * time.Millisecond)
	if n := f.calls.Load(); n != 1 {
		t.Fatalf("a read after the run started another fetch: %d runs", n)
	}
}

func TestOfficialAlertsNeverLogsPosition(t *testing.T) {
	requestLog := &lockedBuffer{}
	stdLog := &lockedBuffer{}
	prev := log.Writer()
	log.SetOutput(stdLog)
	t.Cleanup(func() { log.SetOutput(prev) })

	f := &fakeSachet{}
	srv, mailer, pool := newOfficialServer(t, f, func(d *api.Deps) {
		d.RequestLog = log.New(requestLog, "", log.LstdFlags)
	})
	const email = "official-nolog@example.com"
	client := newVerifiedClient(t, srv, mailer, email)

	// A due refresh runs in the background during this test, so its log lines
	// are covered too.
	officialFeed(t, pool, 11*time.Minute, 11*time.Minute, true)
	homeCell := geohash.EncodeWithPrecision(16.3067, 80.4365, 5)
	testutil.SetHomeCell(t, pool, p25AccountID(t, pool, email), "Home", homeCell)

	status, _, raw := getOfficial(t, client, srv.URL, "?lat=16.50617&lon=80.64803")
	if status != http.StatusOK {
		t.Fatalf("status = %d: %s", status, raw)
	}
	// Let the background refresh finish and log.
	deadline := time.Now().Add(3 * time.Second)
	for f.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	if !strings.Contains(requestLog.String(), "/api/official/alerts") {
		t.Fatalf("the request was not logged at all, so this test proves nothing: %q", requestLog.String())
	}
	for name, text := range map[string]string{"request log": requestLog.String(), "standard logger": stdLog.String()} {
		for _, forbidden := range []string{"16.50617", "80.64803", "lat=", "lon=", homeCell} {
			if strings.Contains(text, forbidden) {
				t.Errorf("the %s holds %q: %s", name, forbidden, text)
			}
		}
	}
	if strings.Contains(raw, homeCell) {
		t.Errorf("the response holds the home cell")
	}
}
