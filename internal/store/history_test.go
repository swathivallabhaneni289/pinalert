// Package store_test tests the report history reads behind the Past tab and
// report links (03.1-04-PLAN.md): ExpiredReportsInWindow, OpenReportsInArea
// and ReportForFocus. They take the instant they judge against and the window
// start as parameters, keep expired and unexpired candidates in separate
// capped sets, and return no session id.
package store_test

import (
	"context"
	"errors"
	"math"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// Fixture values for this file. The instant and the window are fixed values,
// never the wall clock, so the tests prove an injected clock reaches the
// statements.
var (
	hvNow         = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	hvWindowStart = hvNow.Add(-7 * 24 * time.Hour)
)

const (
	hvCentreLat = 12.9716
	hvCentreLon = 77.5946
	hvRadiusKm  = 10.0
	hvGeohash   = "tdr1qgz"
)

// hvBox returns the bounding box of a radius around the centre, the same
// degrees per kilometre arithmetic the service uses for NearbyReports.
func hvBox() (latMin, latMax, lonMin, lonMax float64) {
	dLat := hvRadiusKm / 111.19
	dLon := dLat / math.Cos(hvCentreLat*math.Pi/180)
	return hvCentreLat - dLat, hvCentreLat + dLat, hvCentreLon - dLon, hvCentreLon + dLon
}

func hvExpiredArg(rowCap int32) sqlcgen.ExpiredReportsInWindowParams {
	latMin, latMax, lonMin, lonMax := hvBox()
	return sqlcgen.ExpiredReportsInWindowParams{
		Lat: hvCentreLat, Lon: hvCentreLon,
		LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax,
		RadiusKm: hvRadiusKm, Now: hvNow, WindowStart: hvWindowStart, RowCap: rowCap,
	}
}

func hvOpenArg(rowCap int32) sqlcgen.OpenReportsInAreaParams {
	latMin, latMax, lonMin, lonMax := hvBox()
	return sqlcgen.OpenReportsInAreaParams{
		Lat: hvCentreLat, Lon: hvCentreLon,
		LatMin: latMin, LatMax: latMax, LonMin: lonMin, LonMax: lonMax,
		RadiusKm: hvRadiusKm, Now: hvNow, RowCap: rowCap,
	}
}

// hvSeed inserts one report at the given place, expiring at expiresAt, and
// returns its id.
func hvSeed(t *testing.T, pool *pgxpool.Pool, lat, lon float64, expiresAt time.Time) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 shelter_capacity_status, shelter_headcount, created_at, expires_at)
		 VALUES ('hv-session', 'flood', 'medium', 'history read fixture', $1, $2, $3, NULL, NULL, $4, $5)
		 RETURNING id`,
		lat, lon, hvGeohash, expiresAt.Add(-8*time.Hour), expiresAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding history fixture report: %v", err)
	}
	return id
}

func hvExpiredIDs(rows []sqlcgen.ExpiredReportsInWindowRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

func hvOpenIDs(rows []sqlcgen.OpenReportsInAreaRow) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// hvClockCall matches a database clock call: a clock function with an
// argument list, or one of the clock keywords that need no parentheses.
var hvClockCall = regexp.MustCompile(
	`(?i)\b(now|clock_timestamp|statement_timestamp|transaction_timestamp|timeofday)\s*\(` +
		`|\b(current_timestamp|current_date|current_time|localtimestamp|localtime)\b`)

// TestHistoryQuerySourceHasNoClockCall reads the query file and fails if any
// statement calls the database clock. It needs no database.
func TestHistoryQuerySourceHasNoClockCall(t *testing.T) {
	data, err := os.ReadFile("queries/history.sql")
	if err != nil {
		t.Fatalf("reading queries/history.sql: %v", err)
	}
	src := string(data)
	for _, name := range []string{
		"-- name: ExpiredReportsInWindow :many",
		"-- name: OpenReportsInArea :many",
		"-- name: ReportForFocus :one",
	} {
		if !strings.Contains(src, name) {
			t.Errorf("history.sql is missing %q", name)
		}
	}
	// The comment blocks are held to the same rule: they describe the clock
	// in words and never quote a clock function.
	if m := hvClockCall.FindString(src); m != "" {
		t.Fatalf("history.sql holds a database clock call %q; every bound must be a parameter", m)
	}
}

func TestHistoryCandidatesExpiredWindow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	atNow := hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow)
	hvSeed(t, pool, hvCentreLat, hvCentreLon, hvWindowStart)
	hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow.Add(time.Hour))
	tieA := hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow.Add(-time.Hour))
	tieB := hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow.Add(-time.Hour))
	justInside := hvSeed(t, pool, hvCentreLat, hvCentreLon, hvWindowStart.Add(time.Minute))
	// One degree north is far outside the bounding box.
	hvSeed(t, pool, hvCentreLat+1.0, hvCentreLon, hvNow.Add(-2*time.Hour))
	// Inside the bounding box but past the radius: a corner, about 13 km out.
	hvSeed(t, pool, hvCentreLat+0.085, hvCentreLon+0.087, hvNow.Add(-3*time.Hour))

	rows, err := q.ExpiredReportsInWindow(ctx, hvExpiredArg(500))
	if err != nil {
		t.Fatalf("ExpiredReportsInWindow: %v", err)
	}
	// Expiry descending, then id descending: the tie pair comes back with the
	// higher id first. The edge at now is in, the edge at the window start is
	// out, and an unexpired, out of box and out of radius rows are out.
	want := []int64{atNow, tieB, tieA, justInside}
	got := hvExpiredIDs(rows)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpiredReportsInWindow ids = %v, want %v", got, want)
	}
	for _, r := range rows {
		if r.DistanceKm > hvRadiusKm {
			t.Errorf("report %d distance %.3f km is past the %.0f km radius", r.ID, r.DistanceKm, hvRadiusKm)
		}
	}

	capped, err := q.ExpiredReportsInWindow(ctx, hvExpiredArg(2))
	if err != nil {
		t.Fatalf("ExpiredReportsInWindow capped: %v", err)
	}
	if got := hvExpiredIDs(capped); !reflect.DeepEqual(got, []int64{atNow, tieB}) {
		t.Fatalf("capped ids = %v, want %v", got, []int64{atNow, tieB})
	}
}

func TestHistoryCandidatesNotFilledByLiveReports(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	// 1001 live reports in the area, one more than the open cap, spread north
	// of the centre so each has a different distance. The farthest one is the
	// one the capped read must leave out.
	testutil.MustExec(t, pool,
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 shelter_capacity_status, shelter_headcount, created_at, expires_at)
		 SELECT 'hv-live', 'flood', 'low', 'live fixture',
		        $1::float8 + g * 0.00005, $2::float8, $3,
		        NULL, NULL, $4::timestamptz - interval '1 hour', $4::timestamptz + interval '6 hours'
		 FROM generate_series(1, 1001) AS g`,
		hvCentreLat, hvCentreLon, hvGeohash, hvNow)

	var expired []int64
	for i := 1; i <= 3; i++ {
		expired = append(expired, hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow.Add(-time.Duration(i)*time.Hour)))
	}

	rows, err := q.ExpiredReportsInWindow(ctx, hvExpiredArg(500))
	if err != nil {
		t.Fatalf("ExpiredReportsInWindow: %v", err)
	}
	if got := hvExpiredIDs(rows); !reflect.DeepEqual(got, expired) {
		t.Fatalf("expired set = %v, want exactly the 3 expired reports %v", got, expired)
	}

	open, err := q.OpenReportsInArea(ctx, hvOpenArg(1000))
	if err != nil {
		t.Fatalf("OpenReportsInArea: %v", err)
	}
	if len(open) != 1000 {
		t.Fatalf("open set has %d rows, want exactly 1000", len(open))
	}
	for i, r := range open {
		if !r.ExpiresAt.After(hvNow) {
			t.Fatalf("open row %d (report %d) expires at %v, not after the instant", i, r.ID, r.ExpiresAt)
		}
		if i > 0 && r.DistanceKm < open[i-1].DistanceKm {
			t.Fatalf("open rows are not nearest first at index %d", i)
		}
	}
	var farthest int64
	if err := pool.QueryRow(ctx,
		"SELECT id FROM reports WHERE session_id = 'hv-live' ORDER BY latitude DESC LIMIT 1").Scan(&farthest); err != nil {
		t.Fatalf("finding the farthest live report: %v", err)
	}
	for _, id := range hvOpenIDs(open) {
		if id == farthest {
			t.Fatalf("the farthest live report %d is in the capped open set", farthest)
		}
	}
}

func TestHistoryCandidatesOpenSet(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	// Nearest first: 0 km, about 1.1 km, about 2.2 km.
	far := hvSeed(t, pool, hvCentreLat+0.02, hvCentreLon, hvNow.Add(5*time.Hour))
	near := hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow.Add(2*time.Hour))
	mid := hvSeed(t, pool, hvCentreLat+0.01, hvCentreLon, hvNow.Add(time.Hour))
	// Excluded: expired, expiring exactly at the instant (the edge is strict),
	// out of the box and in the box but past the radius.
	hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow.Add(-time.Hour))
	hvSeed(t, pool, hvCentreLat, hvCentreLon, hvNow)
	hvSeed(t, pool, hvCentreLat+1.0, hvCentreLon, hvNow.Add(time.Hour))
	hvSeed(t, pool, hvCentreLat+0.085, hvCentreLon+0.087, hvNow.Add(time.Hour))

	rows, err := q.OpenReportsInArea(ctx, hvOpenArg(1000))
	if err != nil {
		t.Fatalf("OpenReportsInArea: %v", err)
	}
	want := []int64{near, mid, far}
	if got := hvOpenIDs(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenReportsInArea ids = %v, want %v", got, want)
	}
	for _, r := range rows {
		if r.DistanceKm > hvRadiusKm {
			t.Errorf("report %d distance %.3f km is past the %.0f km radius", r.ID, r.DistanceKm, hvRadiusKm)
		}
	}
}

func TestReportForFocusOutsideRadiusAndWindow(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	// About 300 km north of the centre and expired 30 days ago: outside the
	// reader's radius and outside the 7 day window.
	id := hvSeed(t, pool, hvCentreLat+2.7, hvCentreLon, hvNow.Add(-30*24*time.Hour))

	row, err := q.ReportForFocus(ctx, sqlcgen.ReportForFocusParams{
		ReportID: id, Lat: hvCentreLat, Lon: hvCentreLon,
	})
	if err != nil {
		t.Fatalf("ReportForFocus: %v", err)
	}
	if row.ID != id {
		t.Fatalf("ReportForFocus id = %d, want %d", row.ID, id)
	}
	if row.DistanceKm < 299 || row.DistanceKm > 302 {
		t.Fatalf("distance = %.2f km, want about 300", row.DistanceKm)
	}

	_, err = q.ReportForFocus(ctx, sqlcgen.ReportForFocusParams{
		ReportID: id + 1000, Lat: hvCentreLat, Lon: hvCentreLon,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown id error = %v, want pgx.ErrNoRows", err)
	}
}

// TestHistoryRowsOmitSessionID holds the identity rule on the generated row
// types: no session id, voter cell or account field in any of the three.
func TestHistoryRowsOmitSessionID(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(sqlcgen.ExpiredReportsInWindowRow{}),
		reflect.TypeOf(sqlcgen.OpenReportsInAreaRow{}),
		reflect.TypeOf(sqlcgen.ReportForFocusRow{}),
	}
	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			if strings.Contains(name, "session") || strings.Contains(name, "cell") || strings.Contains(name, "account") {
				t.Errorf("%s has identity field %q", typ.Name(), typ.Field(i).Name)
			}
		}
	}
}
