package service_test

// profilereports_test.go proves ProfileReports on a real database (plan 09-05,
// PROF-02, D-06, D-07, T-09-16, T-09-18): the 30 day window and its order, a
// withheld report never listed, finished reports blurred with no id, no words
// and no time, the Rescue rule kept, the distance rule and the cut at 100.
// Helpers carry the prefix prp so they never collide with other files.

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

const (
	prpLat = 12.9716
	prpLon = 77.5946
)

type prpRig struct {
	pool     *pgxpool.Pool
	svc      *service.ReportService
	now      time.Time
	session  string
	reporter int64
	voters   []int64
}

func newPrpRig(t *testing.T) *prpRig {
	t.Helper()
	pool := testutil.NewTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	r := &prpRig{pool: pool, now: now, session: "prp-reporter-session"}
	r.reporter = testutil.SeedReporterAccount(t, pool, "prp-reporter@example.com", r.session)
	for _, email := range []string{"prp-v1@example.com", "prp-v2@example.com", "prp-v3@example.com"} {
		r.voters = append(r.voters, testutil.SeedReporterAccount(t, pool, email))
	}
	r.svc = service.NewReportService(sqlcgen.New(pool), service.WithReportClock(func() time.Time { return now }))
	return r
}

type prpReport struct {
	category    string
	severity    string
	description string
	lat, lon    float64
	created     time.Time
	expires     time.Time
	area        *string
	moderation  string
}

// add inserts one report from the rig's session and returns its id.
func (r *prpRig) add(t *testing.T, p prpReport) int64 {
	t.Helper()
	if p.category == "" {
		p.category = "flood"
	}
	if p.severity == "" {
		p.severity = "low"
	}
	if p.lat == 0 {
		p.lat, p.lon = prpLat, prpLon
	}
	if p.moderation == "" {
		p.moderation = "clear"
	}
	if p.description == "" {
		p.description = "water at the junction"
	}
	var id int64
	err := r.pool.QueryRow(context.Background(),
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 created_at, expires_at, area_name, moderation_state)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		r.session, p.category, p.severity, p.description, p.lat, p.lon,
		geohash.EncodeWithPrecision(p.lat, p.lon, 7), p.created, p.expires, p.area, p.moderation,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding report: %v", err)
	}
	return id
}

func (r *prpRig) vote(t *testing.T, reportID, account int64, kind, value, cell string, at time.Time) {
	t.Helper()
	testutil.MustExec(t, r.pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`, reportID, account, kind, value, cell, at)
}

func (r *prpRig) read(t *testing.T, centre *service.GeoPoint) service.ProfileReportsPage {
	t.Helper()
	page, err := r.svc.ProfileReports(context.Background(), service.ProfileQuery{
		AccountID: r.reporter, ViewerAccountID: 0, Centre: centre, Now: r.now,
	})
	if err != nil {
		t.Fatalf("ProfileReports: %v", err)
	}
	return page
}

func prpKm(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	a := math.Sin((lat2-lat1)*rad/2)*math.Sin((lat2-lat1)*rad/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin((lon2-lon1)*rad/2)*math.Sin((lon2-lon1)*rad/2)
	return 6371 * 2 * math.Asin(math.Sqrt(a))
}

func TestProfileReportsWindowAndOrder(t *testing.T) {
	r := newPrpRig(t)
	day := 24 * time.Hour
	old := r.add(t, prpReport{created: r.now.Add(-40 * day), expires: r.now.Add(-40*day + 8*time.Hour)})
	edge := r.add(t, prpReport{created: r.now.Add(-29 * day), expires: r.now.Add(-29*day + 8*time.Hour)})
	mid := r.add(t, prpReport{created: r.now.Add(-2 * day), expires: r.now.Add(-2*day + 8*time.Hour)})
	live := r.add(t, prpReport{created: r.now.Add(-time.Hour), expires: r.now.Add(7 * time.Hour)})
	_ = old

	page := r.read(t, nil)
	if len(page.Rows) != 3 {
		t.Fatalf("rows = %d, want 3 (the 40 day old report is out of the window)", len(page.Rows))
	}
	if page.Cut {
		t.Errorf("Cut = true for 3 rows")
	}
	if !page.Rows[0].Live || page.Rows[0].View.ID != live {
		t.Errorf("row 0 = live %v id %d, want the live report %d first", page.Rows[0].Live, page.Rows[0].View.ID, live)
	}
	if page.Rows[1].Live || page.Rows[2].Live {
		t.Errorf("rows 1 and 2 should be finished")
	}
	// Finished rows carry no id, so order is checked by their end dates instead.
	wantMid := r.now.Add(-2*day + 8*time.Hour).In(time.FixedZone("IST", 5*3600+1800)).Format("2 January 2006")
	wantEdge := r.now.Add(-29*day + 8*time.Hour).In(time.FixedZone("IST", 5*3600+1800)).Format("2 January 2006")
	if page.Rows[1].EndedOn != wantMid || page.Rows[2].EndedOn != wantEdge {
		t.Errorf("end dates = %q, %q, want %q, %q (newest first)", page.Rows[1].EndedOn, page.Rows[2].EndedOn, wantMid, wantEdge)
	}
	_, _ = edge, mid
}

func TestProfileReportsNeverListsWithheld(t *testing.T) {
	r := newPrpRig(t)
	for _, state := range []string{"pending", "removed"} {
		r.add(t, prpReport{created: r.now.Add(-time.Hour), expires: r.now.Add(7 * time.Hour), moderation: state})
		r.add(t, prpReport{created: r.now.Add(-20 * time.Hour), expires: r.now.Add(-12 * time.Hour), moderation: state})
	}
	clear := r.add(t, prpReport{created: r.now.Add(-2 * time.Hour), expires: r.now.Add(6 * time.Hour)})

	page := r.read(t, nil)
	if len(page.Rows) != 1 || page.Rows[0].View.ID != clear {
		t.Fatalf("rows = %+v, want only the clear report %d", page.Rows, clear)
	}
}

func TestProfileReportsFinishedAreCoarse(t *testing.T) {
	r := newPrpRig(t)
	area := "Indiranagar, Bengaluru"
	liveID := r.add(t, prpReport{description: "knee deep water here", created: r.now.Add(-time.Hour), expires: r.now.Add(7 * time.Hour), area: &area})
	r.add(t, prpReport{description: "call me on 9876543210", created: r.now.Add(-30 * time.Hour), expires: r.now.Add(-22 * time.Hour), area: &area})
	resolvedID := r.add(t, prpReport{description: "road blocked by a tree", created: r.now.Add(-10 * time.Hour), expires: r.now.Add(-2 * time.Hour), area: &area})
	closedAt := r.now.Add(-5 * time.Hour)
	r.vote(t, resolvedID, r.reporter, "resolution", "resolve", "tdr1v8y", closedAt)

	page := r.read(t, nil)
	if len(page.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(page.Rows))
	}

	live := page.Rows[0]
	if !live.Live || live.View.ID != liveID || live.View.Description != "knee deep water here" {
		t.Errorf("live row lost its id or words: %+v", live.View.Report)
	}
	if live.View.Latitude != prpLat || live.View.Longitude != prpLon {
		t.Errorf("live row point = (%v, %v), want the exact point", live.View.Latitude, live.View.Longitude)
	}

	wantLat, wantLon, wantCell := service.CoarsePointAt(prpLat, prpLon, 5)
	zone := time.FixedZone("IST", 5*3600+1800)
	wantEnds := map[string]string{
		service.ProfileStatusResolved: closedAt.In(zone).Format("2 January 2006"),
		service.ProfileStatusExpired:  r.now.Add(-22 * time.Hour).In(zone).Format("2 January 2006"),
	}
	for _, row := range page.Rows[1:] {
		if row.Live {
			t.Fatalf("a finished report listed as live: %+v", row)
		}
		v := row.View
		if v.ID != 0 || v.Description != "" {
			t.Errorf("finished row leaks id %d or text %q", v.ID, v.Description)
		}
		if v.Latitude != wantLat || v.Longitude != wantLon || v.Geohash != wantCell || len(v.Geohash) != 5 {
			t.Errorf("finished point = (%v, %v, %q), want the precision 5 cell (%v, %v, %q)", v.Latitude, v.Longitude, v.Geohash, wantLat, wantLon, wantCell)
		}
		if !v.CreatedAt.IsZero() || !v.ExpiresAt.IsZero() || (v.History != nil && !v.History.At.IsZero()) {
			t.Errorf("finished row carries a time: created %v expires %v history %+v", v.CreatedAt, v.ExpiresAt, v.History)
		}
		if !v.Approximate {
			t.Errorf("finished row is not marked approximate")
		}
		if row.FinishedTitle != "Flood, Low" {
			t.Errorf("title = %q, want %q", row.FinishedTitle, "Flood, Low")
		}
		if row.AreaLine != "Near "+area {
			t.Errorf("area line = %q", row.AreaLine)
		}
		if want := wantEnds[row.Status]; want == "" || row.EndedOn != want {
			t.Errorf("status %q ended on %q, want %q", row.Status, row.EndedOn, want)
		}
		if row.DistanceText != "" || v.DistanceKm != nil {
			t.Errorf("no centre was given but a distance is shown: %q", row.DistanceText)
		}
	}
	if page.Rows[1].Status != service.ProfileStatusResolved && page.Rows[2].Status != service.ProfileStatusResolved {
		t.Errorf("no resolved row: %q, %q", page.Rows[1].Status, page.Rows[2].Status)
	}
}

func TestProfileReportsRescueKeepsD23(t *testing.T) {
	r := newPrpRig(t)
	r.add(t, prpReport{category: string(service.CategoryRescueNeeded), severity: "critical", description: "family on the roof",
		created: r.now.Add(-30 * time.Hour), expires: r.now.Add(-6 * time.Hour)})

	page := r.read(t, nil)
	if len(page.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(page.Rows))
	}
	v := page.Rows[0].View
	_, _, want6 := service.CoarsePointAt(prpLat, prpLon, 6)
	if v.Geohash != want6 || len(v.Geohash) != 6 {
		t.Errorf("rescue geohash = %q, want the precision 6 cell %q", v.Geohash, want6)
	}
	if v.Description != "" || v.ID != 0 {
		t.Errorf("rescue row leaks text %q or id %d", v.Description, v.ID)
	}
	if page.Rows[0].FinishedTitle != "Rescue needed, Critical" {
		t.Errorf("title = %q", page.Rows[0].FinishedTitle)
	}
}

func TestProfileReportsDistance(t *testing.T) {
	r := newPrpRig(t)
	r.add(t, prpReport{created: r.now.Add(-time.Hour), expires: r.now.Add(7 * time.Hour)})
	r.add(t, prpReport{created: r.now.Add(-30 * time.Hour), expires: r.now.Add(-22 * time.Hour)})

	// A centre about 2.2 km north of the exact point: the exact distance and the
	// distance to the coarse cell centre differ, so measuring to the wrong point
	// shows.
	centre := &service.GeoPoint{Lat: prpLat + 0.02, Lon: prpLon}
	page := r.read(t, centre)
	if len(page.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(page.Rows))
	}
	exact := prpKm(centre.Lat, centre.Lon, prpLat, prpLon)
	cLat, cLon, _ := service.CoarsePointAt(prpLat, prpLon, 5)
	coarse := prpKm(centre.Lat, centre.Lon, cLat, cLon)
	if math.Abs(exact-coarse) < 0.05 {
		t.Fatalf("test setup: exact %.3f and coarse %.3f km are too close to tell apart", exact, coarse)
	}

	live, done := page.Rows[0], page.Rows[1]
	if live.View.DistanceKm == nil || math.Abs(*live.View.DistanceKm-exact) > 1e-6 {
		t.Errorf("live distance = %v, want the exact %.3f", live.View.DistanceKm, exact)
	}
	if want := service.DistanceWords(exact); live.DistanceText != want {
		t.Errorf("live distance text = %q, want %q", live.DistanceText, want)
	}
	if done.View.DistanceKm == nil || math.Abs(*done.View.DistanceKm-coarse) > 1e-6 {
		t.Errorf("finished distance = %v, want the distance to the coarse point %.3f", done.View.DistanceKm, coarse)
	}
	n := int(math.Round(coarse))
	if n < 1 {
		n = 1
	}
	if want := "About " + strconv.Itoa(n) + " km"; done.DistanceText != want {
		t.Errorf("finished distance text = %q, want %q", done.DistanceText, want)
	}

	none := r.read(t, nil)
	for _, row := range none.Rows {
		if row.DistanceText != "" || row.View.DistanceKm != nil {
			t.Errorf("a distance with no centre: %q", row.DistanceText)
		}
	}
}

func TestProfileReportsStatusWords(t *testing.T) {
	r := newPrpRig(t)
	cells := []string{"tdr1v8y", "tdr1vc3", "tdr1vd2"}
	at := r.now.Add(-30 * time.Minute)

	plain := r.add(t, prpReport{created: r.now.Add(-time.Hour), expires: r.now.Add(7 * time.Hour)})
	confirmed := r.add(t, prpReport{created: r.now.Add(-2 * time.Hour), expires: r.now.Add(6 * time.Hour)})
	r.vote(t, confirmed, r.voters[0], "content", "confirm", cells[0], at)
	r.vote(t, confirmed, r.voters[1], "content", "confirm", cells[1], at)
	disputed := r.add(t, prpReport{created: r.now.Add(-3 * time.Hour), expires: r.now.Add(5 * time.Hour)})
	for i := 0; i < 3; i++ {
		r.vote(t, disputed, r.voters[i], "content", "dispute", cells[i], at)
	}
	_ = plain

	page := r.read(t, nil)
	if len(page.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(page.Rows))
	}
	got := []string{page.Rows[0].Status, page.Rows[1].Status, page.Rows[2].Status}
	want := []string{service.ProfileStatusNotConfirmed, service.ProfileStatusOngoing, service.ProfileStatusDisputed}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d status = %q, want %q", i, got[i], want[i])
		}
	}
	words := []string{"Not confirmed yet", "Ongoing", "Disputed"}
	for i := range words {
		if page.Rows[i].StatusWord != words[i] {
			t.Errorf("row %d word = %q, want %q", i, page.Rows[i].StatusWord, words[i])
		}
	}
	if page.Rows[2].DisputedLine != "Disputed: may not be true" || page.Rows[0].DisputedLine != "" {
		t.Errorf("disputed lines = %q, %q", page.Rows[2].DisputedLine, page.Rows[0].DisputedLine)
	}
}

func TestProfileReportsCut(t *testing.T) {
	r := newPrpRig(t)
	testutil.MustExec(t, r.pool,
		`INSERT INTO reports (session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		 SELECT $1, 'flood', 'low', 'bulk report ' || g, $2::float8, $3::float8, 'tdr1v8y',
		        $4::timestamptz - (g || ' minutes')::interval, $4::timestamptz + interval '6 hours'
		 FROM generate_series(1, 120) AS g`,
		r.session, prpLat, prpLon, r.now)

	page := r.read(t, nil)
	if len(page.Rows) != service.ProfileReportsCap {
		t.Fatalf("rows = %d, want %d", len(page.Rows), service.ProfileReportsCap)
	}
	if !page.Cut || page.CutText != "Showing the newest 100." {
		t.Errorf("Cut = %v, CutText = %q", page.Cut, page.CutText)
	}
	if page.Rows[0].View.Description != "bulk report 1" {
		t.Errorf("first row = %q, want the newest report", page.Rows[0].View.Description)
	}
}
