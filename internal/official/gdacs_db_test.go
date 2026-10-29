package official

// These tests run the GDACS feed against a real database and a fake GDACS: an
// httptest server that serves the recorded SEARCH fixtures through the client's
// test seam (plan 05-21). No test here reaches the real GDACS host. Every test
// takes its clock from gdacsNow, 2026-10-28 10:00 UTC, the day the fixtures were
// recorded, so the 48 hour keep decides what is current and not the wall clock.

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/areas"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

type gdacsEnv struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
	srv  *fakeGdacs
	clk  *clock
	r    *Refresher
	svc  *Service
}

func newGdacsEnv(t *testing.T) *gdacsEnv {
	t.Helper()
	pool := testutil.NewTestDB(t)
	srv := newFakeGdacs(t)
	clk := &clock{t: gdacsNow}
	q := sqlcgen.New(pool)
	r := NewRefresher(pool, q, WithRefresherClock(clk.now), WithGdacs(testGdacsClient(srv.srv.URL, ClientConfig{}, 0)))
	svc := newService(r, q, areas.Default())
	svc.now = clk.now
	return &gdacsEnv{pool: pool, q: q, srv: srv, clk: clk, r: r, svc: svc}
}

// refresh moves the clock past the minimum gap and runs one GDACS refresh.
func (e *gdacsEnv) refresh(t *testing.T) (Result, error) {
	t.Helper()
	e.clk.set(e.clk.now().Add(GdacsMinGap))
	return e.r.RefreshIfDue(context.Background(), FeedGdacs)
}

func (e *gdacsEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

func (e *gdacsEnv) hazard(t *testing.T, eventType string, eventID int64) sqlcgen.OfficialHazard {
	t.Helper()
	var h sqlcgen.OfficialHazard
	err := e.pool.QueryRow(context.Background(),
		`SELECT id, event_type, event_id, episode_id, name, alert_level, country, latitude, longitude,
		        from_date, to_date, is_current, severity_text, report_url, date_modified, first_seen_at, last_seen_at
		 FROM official_hazards WHERE event_type = $1 AND event_id = $2`, eventType, eventID).
		Scan(&h.ID, &h.EventType, &h.EventID, &h.EpisodeID, &h.Name, &h.AlertLevel, &h.Country, &h.Latitude, &h.Longitude,
			&h.FromDate, &h.ToDate, &h.IsCurrent, &h.SeverityText, &h.ReportUrl, &h.DateModified, &h.FirstSeenAt, &h.LastSeenAt)
	if err != nil {
		t.Fatalf("reading hazard %s %d: %v", eventType, eventID, err)
	}
	return h
}

func TestGdacsRefreshStores(t *testing.T) {
	e := newGdacsEnv(t)

	res, err := e.refresh(t)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if res.Skipped || res.ItemsSeen != 13 || res.Fetched != 13 || res.Stored != 5 {
		t.Fatalf("result = %+v, want 13 seen and 5 stored", res)
	}
	if e.srv.hits() != 2 {
		t.Fatalf("%d requests for one refresh, want 2", e.srv.hits())
	}
	if n := e.count(t, `SELECT count(*) FROM official_hazards`); n != 5 {
		t.Fatalf("%d hazards stored, want 5 (current cyclones from A, the in box points from B, once each)", n)
	}
	for _, dropped := range []struct {
		t  string
		id int64
	}{{"DR", 1000999}, {"DR", 1000998}, {"EQ", 1573141}, {"TC", 1001335}, {"FL", 1104239}, {"FL", 1104216}} {
		if n := e.count(t, `SELECT count(*) FROM official_hazards WHERE event_type = $1 AND event_id = $2`, dropped.t, dropped.id); n != 0 {
			t.Errorf("%s %d was stored", dropped.t, dropped.id)
		}
	}

	tc := e.hazard(t, "TC", 1001345)
	if tc.Name != "Tropical Cyclone ARNAB-26" || tc.AlertLevel != "Red" || tc.Country != "India" || !tc.IsCurrent ||
		tc.Latitude != 16.4 || tc.Longitude != 81.8 || tc.EpisodeID == nil || *tc.EpisodeID != 10 {
		t.Errorf("stored cyclone = %+v", tc)
	}
	if tc.ReportUrl != "https://www.gdacs.org/report.aspx?eventid=1001345&episodeid=10&eventtype=TC" {
		t.Errorf("stored report url = %q", tc.ReportUrl)
	}
	if !tc.ToDate.Valid || !tc.ToDate.Time.Equal(time.Date(2026, 10, 28, 6, 0, 0, 0, time.UTC)) {
		t.Errorf("stored to_date = %v", tc.ToDate)
	}
	firstSeen := tc.FirstSeenAt
	if !firstSeen.Equal(e.clk.now()) {
		t.Errorf("first_seen_at = %v, want the refresher clock %v", firstSeen, e.clk.now())
	}

	st, err := e.r.Status(context.Background(), FeedGdacs)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.LastSuccessAt == nil || !st.LastSuccessAt.Equal(e.clk.now()) || st.ItemsSeen != 13 || st.LastError != "" || st.ConsecutiveFailures != 0 {
		t.Errorf("gdacs status = %+v, want a success now with 13 items", st)
	}
	// SACHET's row is untouched by a GDACS refresh.
	if s, _ := e.r.Status(context.Background(), FeedSachet); s.LastAttemptAt != nil || s.LastSuccessAt != nil {
		t.Errorf("a GDACS refresh touched the sachet row: %+v", s)
	}

	// Inside the gap nothing is requested.
	e.clk.set(e.clk.now().Add(GdacsMinGap - time.Minute))
	if res, err := e.r.RefreshIfDue(context.Background(), FeedGdacs); err != nil || !res.Skipped {
		t.Fatalf("a call inside the 15 minute gap: %+v, %v; want Skipped", res, err)
	}
	if e.srv.hits() != 2 {
		t.Errorf("%d requests after a skipped call, want still 2", e.srv.hits())
	}

	// Past the gap the same events are updated in place.
	e.srv.mu.Lock()
	e.srv.a = bytes.ReplaceAll(e.srv.a, []byte("Tropical Cyclone ARNAB-26"), []byte("Tropical Cyclone ARNAB-26 (updated)"))
	e.srv.b = bytes.ReplaceAll(e.srv.b, []byte("Tropical Cyclone ARNAB-26"), []byte("Tropical Cyclone ARNAB-26 (updated)"))
	e.srv.mu.Unlock()
	res, err = e.refresh(t)
	if err != nil || res.Skipped || res.Stored != 5 {
		t.Fatalf("second refresh: %+v, %v", res, err)
	}
	if n := e.count(t, `SELECT count(*) FROM official_hazards`); n != 5 {
		t.Fatalf("%d hazards after a second refresh, want still 5", n)
	}
	again := e.hazard(t, "TC", 1001345)
	if again.ID != tc.ID || again.Name != "Tropical Cyclone ARNAB-26 (updated)" {
		t.Errorf("second refresh row = id %d name %q, want the same row renamed", again.ID, again.Name)
	}
	if !again.FirstSeenAt.Equal(firstSeen) || !again.LastSeenAt.Equal(e.clk.now()) {
		t.Errorf("first_seen_at %v last_seen_at %v, want %v kept and the new time", again.FirstSeenAt, again.LastSeenAt, firstSeen)
	}

	t.Run("what a pin may show", func(t *testing.T) {
		ctx := context.Background()
		now := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
		rows, err := e.q.CurrentOfficialHazards(ctx, now.Add(-GdacsEndedKeep))
		if err != nil {
			t.Fatalf("CurrentOfficialHazards: %v", err)
		}
		var got []string
		for _, r := range rows {
			got = append(got, r.EventType+":"+strconv.FormatInt(r.EventID, 10))
		}
		// Ended more than 48 hours before: TC 1001338 (25 Oct 18:00) and EQ 1573845 (25 Oct 22:01).
		if want := "TC:1001345,EQ:1574196,EQ:1574148"; strings.Join(got, ",") != want {
			t.Errorf("current hazards = %s, want %s", strings.Join(got, ","), want)
		}

		// A box around Vijayawada and the Godavari delta holds the cyclone's point.
		box, err := e.q.OfficialHazardsInBox(ctx, sqlcgen.OfficialHazardsInBoxParams{
			EndedAfter: now.Add(-GdacsEndedKeep), MinLat: 15.5, MaxLat: 17.5, MinLon: 80, MaxLon: 82})
		if err != nil || len(box) != 1 || box[0].EventID != 1001345 {
			t.Errorf("hazards in the box = %+v, %v; want only the cyclone", box, err)
		}
		none, err := e.q.OfficialHazardsInBox(ctx, sqlcgen.OfficialHazardsInBoxParams{
			EndedAfter: now.Add(-GdacsEndedKeep), MinLat: 10, MaxLat: 12, MinLon: 76, MaxLon: 78})
		if err != nil || len(none) != 0 {
			t.Errorf("hazards in an empty box = %+v, %v", none, err)
		}
	})

	t.Run("a hazard GDACS stopped calling current leaves", func(t *testing.T) {
		e.srv.mu.Lock()
		e.srv.b = bytes.Replace(e.srv.b, []byte(`"iscurrent": "true"`), []byte(`"iscurrent": "false"`), -1)
		e.srv.mu.Unlock()
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		rows, err := e.q.CurrentOfficialHazards(context.Background(), e.clk.now().Add(-GdacsEndedKeep))
		if err != nil {
			t.Fatalf("CurrentOfficialHazards: %v", err)
		}
		for _, r := range rows {
			if r.EventType == "EQ" {
				t.Errorf("EQ %d is still current after GDACS marked it not current", r.EventID)
			}
		}
	})

	// WR-S03 of the Phase 5 review: an event only request A returns (the
	// cyclone 1001338 lies outside the South Asia box) leaves the map when A
	// marks it not current.
	t.Run("a request A only hazard GDACS stopped calling current leaves", func(t *testing.T) {
		current := func() bool {
			t.Helper()
			rows, err := e.q.CurrentOfficialHazards(context.Background(), e.clk.now().Add(-GdacsEndedKeep))
			if err != nil {
				t.Fatalf("CurrentOfficialHazards: %v", err)
			}
			for _, r := range rows {
				if r.EventType == "TC" && r.EventID == 1001338 {
					return true
				}
			}
			return false
		}
		// First it runs on, so the row is current.
		e.srv.mu.Lock()
		e.srv.a = bytes.Replace(e.srv.a, []byte(`"todate": "2026-10-25T18:00:00"`), []byte(`"todate": "2026-10-30T18:00:00"`), 1)
		e.srv.mu.Unlock()
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		if !current() {
			t.Fatalf("TC 1001338 is not current after request A extended it; the case needs it current first")
		}
		e.srv.mu.Lock()
		e.srv.a = bytes.Replace(e.srv.a, []byte(`"iscurrent": "true"`), []byte(`"iscurrent": "false"`), -1)
		e.srv.mu.Unlock()
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		if current() {
			t.Errorf("TC 1001338 is still current after request A marked it not current")
		}
	})
}

// TestGdacsHazardWithoutEndDate: a current hazard with no end date counts as
// ended when it was last seen more than 48 hours ago, so a row GDACS stopped
// mentioning cannot stay on the map for ever.
func TestGdacsHazardWithoutEndDate(t *testing.T) {
	e := newGdacsEnv(t)
	insert := func(id int64, lastSeen time.Time) {
		testutil.MustExec(t, e.pool,
			`INSERT INTO official_hazards (event_type, event_id, name, alert_level, latitude, longitude, is_current, first_seen_at, last_seen_at)
			 VALUES ('EQ', $1, 'no end date', 'Green', 20, 78, true, $2, $2)`, id, lastSeen)
	}
	insert(1, gdacsNow.Add(-3*time.Hour))
	insert(2, gdacsNow.Add(-GdacsEndedKeep-time.Hour))
	rows, err := e.q.CurrentOfficialHazards(context.Background(), gdacsNow.Add(-GdacsEndedKeep))
	if err != nil || len(rows) != 1 || rows[0].EventID != 1 {
		t.Fatalf("current hazards = %+v, %v; want only the one seen 3 hours ago", rows, err)
	}
}

// TestGdacsLeavesReportsAlone: a GDACS refresh writes official_hazards and the
// feed status row and nothing else. Crowd reports and votes are the trust
// mechanic's data; an official event is never one of them (D-15, T-05-110).
func TestGdacsLeavesReportsAlone(t *testing.T) {
	e := newGdacsEnv(t)
	testutil.SeedReports(t, e.pool, 6, 16.5062, 80.6480)
	testutil.SeedOfficialAlert(t, e.pool, testutil.OfficialAlertSeed{Tier: "state", StateCodes: []int{28}})

	snap := func(table string) string {
		var s *string
		if err := e.pool.QueryRow(context.Background(),
			`SELECT md5(coalesce(string_agg(t::text, '|' ORDER BY t::text), '')) FROM `+table+` t`).Scan(&s); err != nil {
			t.Fatalf("snapshot of %s: %v", table, err)
		}
		return *s
	}
	tables := []string{"reports", "votes", "sessions", "accounts", "official_alerts", "official_alert_areas", "official_push_outbox", "push_outbox", "updates"}
	before := map[string]string{}
	for _, tb := range tables {
		before[tb] = snap(tb)
	}
	nReports := e.count(t, `SELECT count(*) FROM reports`)
	if nReports != 6 {
		t.Fatalf("test setup: %d reports", nReports)
	}

	if res, err := e.refresh(t); err != nil || res.Stored != 5 {
		t.Fatalf("refresh: %+v, %v", res, err)
	}
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("second refresh: %v", err)
	}

	for _, tb := range tables {
		if got := snap(tb); got != before[tb] {
			t.Errorf("a GDACS refresh changed the %s table", tb)
		}
	}
	if e.count(t, `SELECT count(*) FROM official_hazards`) != 5 {
		t.Error("the hazards were not stored")
	}
}

// TestGdacsStatusSeparate: a failing GDACS fetch marks only the gdacs row with
// its error word and leaves SACHET's status as it was, and the view tells the
// two feeds' freshness apart (ALERT-04, D-15).
func TestGdacsStatusSeparate(t *testing.T) {
	e := newGdacsEnv(t)
	ctx := context.Background()

	sAt := gdacsNow.Add(-5 * time.Minute)
	testutil.SetOfficialFeedStatus(t, e.pool, "sachet", &sAt, &sAt, 20)
	sachetBefore, err := e.r.Status(ctx, FeedSachet)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if _, err := e.refresh(t); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	firstOK := e.clk.now()

	e.srv.setMode("failB")
	res, err := e.refresh(t)
	if err == nil || res.ErrorClass != "status" {
		t.Fatalf("failing refresh: %+v, %v; want the status word", res, err)
	}
	g, err := e.r.Status(ctx, FeedGdacs)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if g.LastError != "status" || g.ConsecutiveFailures != 1 || g.LastSuccessAt == nil || !g.LastSuccessAt.Equal(firstOK) {
		t.Errorf("gdacs status = %+v, want error status, one failure and the first success kept", g)
	}
	if n := e.count(t, `SELECT count(*) FROM official_hazards`); n != 5 {
		t.Errorf("%d hazards after a failed run, want the 5 from the good run kept", n)
	}
	sachetAfter, err := e.r.Status(ctx, FeedSachet)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if sachetAfter.LastError != "" || sachetAfter.ConsecutiveFailures != 0 || sachetAfter.ItemsSeen != sachetBefore.ItemsSeen ||
		sachetAfter.LastAttemptAt == nil || !sachetAfter.LastAttemptAt.Equal(*sachetBefore.LastAttemptAt) ||
		sachetAfter.LastSuccessAt == nil || !sachetAfter.LastSuccessAt.Equal(*sachetBefore.LastSuccessAt) {
		t.Errorf("sachet status changed: before %+v after %+v", sachetBefore, sachetAfter)
	}

	// Two hours on, SACHET was checked a few minutes ago and GDACS last
	// succeeded two hours ago: the view says fresh for one and stale for the other.
	e.clk.set(firstOK.Add(2 * time.Hour))
	recent := e.clk.now().Add(-3 * time.Minute)
	testutil.SetOfficialFeedStatus(t, e.pool, "sachet", &recent, &recent, 20)
	lat, lon := 16.5062, 80.6480
	v, err := e.svc.Alerts(ctx, 0, &lat, &lon)
	if err != nil {
		t.Fatalf("Alerts: %v", err)
	}
	if v.State != StateFresh || v.HazardState != StateStale {
		t.Fatalf("state %q hazard state %q, want fresh and stale", v.State, v.HazardState)
	}
	if v.HazardAgeSeconds == nil || *v.HazardAgeSeconds != 7200 || v.HazardLastSuccessAt == nil || !v.HazardLastSuccessAt.Equal(firstOK) {
		t.Errorf("hazard age %v last success %v, want 7200 seconds since %v", v.HazardAgeSeconds, v.HazardLastSuccessAt, firstOK)
	}
	if v.AgeSeconds == nil || *v.AgeSeconds != 180 {
		t.Errorf("alerts age = %v, want 180: SACHET's own", v.AgeSeconds)
	}
}

func seedHazardRow(t *testing.T, e *gdacsEnv, id int64, level string, lat, lon float64, to time.Time) {
	t.Helper()
	testutil.MustExec(t, e.pool,
		`INSERT INTO official_hazards (event_type, event_id, name, alert_level, country, latitude, longitude, from_date, to_date, is_current, severity_text, report_url, first_seen_at, last_seen_at)
		 VALUES ('EQ', $1, 'Earthquake in India', $2, 'India', $3, $4, $5, $6, true, 'Magnitude 5.0M', 'https://www.gdacs.org/report.aspx?eventid=1', $5, $5)`,
		id, level, lat, lon, to.Add(-6*time.Hour), to)
}

// TestAlertsListHazardsFromTheDatabase: the service reads the hazards in the
// reader's district and the home district, tests the point in Go, lists nothing
// when the hazard feed is not fresh, and gives pins only when it is fresh.
func TestAlertsListHazardsFromTheDatabase(t *testing.T) {
	e := newGdacsEnv(t)
	ctx := context.Background()
	lat, lon := 16.5062, 80.6480 // Vijayawada, district 749
	ix := areas.Default()
	d, ok := ix.Locate(lat, lon)
	if !ok || d.Code != 749 {
		t.Fatalf("test setup: Vijayawada is in %+v", d)
	}

	recent := gdacsNow.Add(-2 * time.Minute)
	testutil.SetOfficialFeedStatus(t, e.pool, "sachet", &recent, &recent, 20)
	testutil.SetOfficialFeedStatus(t, e.pool, "gdacs", &recent, &recent, 5)

	seedHazardRow(t, e, 1, "Red", lat, lon, gdacsNow.Add(-time.Hour))                                 // in the district, ended an hour ago
	seedHazardRow(t, e, 2, "Orange", 28.6, 77.2, gdacsNow.Add(time.Hour))                             // Delhi: another district
	seedHazardRow(t, e, 3, "Green", lat, lon, gdacsNow.Add(-GdacsEndedKeep-time.Hour))                // ended long ago
	seedHazardRow(t, e, 4, "Green", lat+0.0005, lon+0.0005, gdacsNow.Add(30*time.Minute))             // in the district, still going
	seedHazardRow(t, e, 5, "Red", -33.9, 151.2, gdacsNow.Add(time.Hour))                              // Sydney: no district
	testutil.MustExec(t, e.pool, `UPDATE official_hazards SET is_current = false WHERE event_id = 4`) // GDACS says not current

	v, err := e.svc.Alerts(ctx, 0, &lat, &lon)
	if err != nil {
		t.Fatalf("Alerts: %v", err)
	}
	if v.HazardState != StateFresh {
		t.Fatalf("hazard state = %q", v.HazardState)
	}
	if len(v.Groups) != 1 || v.Groups[0].Kind != GroupReader || len(v.Groups[0].Rows) != 1 || v.Groups[0].Rows[0].LevelWord != "Red" {
		t.Fatalf("groups = %+v, want one reader group holding the one Red hazard", v.Groups)
	}
	if v.Banner != nil {
		t.Errorf("banner = %+v, want none", *v.Banner)
	}

	pins, err := e.svc.Pins(ctx)
	if err != nil {
		t.Fatalf("Pins: %v", err)
	}
	if pins.State != StateFresh || len(pins.Pins) != 3 {
		t.Fatalf("pins = %+v, want 3 current hazards (1, 2 and 5; 3 ended long ago and 4 is not current)", pins)
	}
	for _, p := range pins.Pins {
		if p.ReportURL != "https://www.gdacs.org/report.aspx?eventid=1" {
			t.Errorf("pin report url = %q", p.ReportURL)
		}
	}

	// A hazard feed last checked two hours ago lists no rows and gives no pins.
	old := gdacsNow.Add(-2 * time.Hour)
	testutil.SetOfficialFeedStatus(t, e.pool, "gdacs", &recent, &old, 5)
	v, err = e.svc.Alerts(ctx, 0, &lat, &lon)
	if err != nil || v.HazardState != StateStale || v.Count != 0 {
		t.Fatalf("stale: state %q count %d err %v, want stale with no rows", v.HazardState, v.Count, err)
	}
	pins, err = e.svc.Pins(ctx)
	if err != nil || pins.State != StateStale || len(pins.Pins) != 0 || pins.AgeSeconds == nil || *pins.AgeSeconds != 7200 {
		t.Fatalf("stale pins = %+v, %v; want stale, 7200 seconds and no pins", pins, err)
	}
}
