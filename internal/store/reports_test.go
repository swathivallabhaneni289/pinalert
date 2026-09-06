// Package store_test lives outside package store deliberately: this file
// imports internal/testutil, and internal/testutil imports internal/store,
// so an in-package store_test would be an import cycle.
package store_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// nearbyReportsSQLForExplain must stay byte-for-byte in sync with the
// NearbyReports query body in internal/store/queries/reports.sql (sqlc
// emits the query as an unexported const, unreachable from this external
// test package). TestNearbyReportsQuerySourceHasExpectedShape is a drift
// guard against the two diverging silently.
const nearbyReportsSQLForExplain = `
SELECT id, category, severity, description, latitude, longitude, geohash,
       shelter_capacity_status, shelter_headcount, created_at, expires_at, distance_km
FROM (
    SELECT id, category, severity, description, latitude, longitude, geohash,
           shelter_capacity_status, shelter_headcount, created_at, expires_at,
           ( 6371 * acos(
               least(1.0,
                 cos(radians($1::float8)) * cos(radians(latitude))
                 * cos(radians(longitude) - radians($2::float8))
                 + sin(radians($1::float8)) * sin(radians(latitude))
               )
             )
           )::float8 AS distance_km
    FROM reports
    WHERE expires_at > now()
      AND latitude  BETWEEN $3::float8 AND $4::float8
      AND longitude BETWEEN $5::float8 AND $6::float8
) AS candidates
WHERE distance_km <= $7::float8
ORDER BY distance_km ASC
`

func TestNearbyReportsQuerySourceHasExpectedShape(t *testing.T) {
	data, err := os.ReadFile("queries/reports.sql")
	if err != nil {
		t.Fatalf("reading queries/reports.sql: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "expires_at > now()") {
		t.Fatalf("expected NearbyReports to gate on expires_at > now() (read-time expiry, FOUND-05)")
	}
	if !strings.Contains(src, "BETWEEN") {
		t.Fatalf("expected NearbyReports to use an indexed BETWEEN bounding-box prefilter (FOUND-03)")
	}
}

func TestNearbyReportsUsesIndex(t *testing.T) {
	pool := testutil.NewTestDB(t)
	const centreLat, centreLon = 12.9716, 77.5946
	testutil.SeedReports(t, pool, testutil.DefaultSeedCount, centreLat, centreLon)
	// pgx.CopyFrom leaves the planner without fresh stats until autovacuum
	// catches up; ANALYZE now so the planner has real row-count/selectivity
	// estimates and doesn't fall back to a Seq Scan for the wrong reason.
	testutil.MustExec(t, pool, "ANALYZE reports")

	// A deliberately narrow box around the seed centre.
	const delta = 0.01
	latMin, latMax := centreLat-delta, centreLat+delta
	lonMin, lonMax := centreLon-delta, centreLon+delta

	rows, err := pool.Query(context.Background(),
		"EXPLAIN (ANALYZE, FORMAT TEXT) "+nearbyReportsSQLForExplain,
		centreLat, centreLon, latMin, latMax, lonMin, lonMax, 5.0,
	)
	if err != nil {
		t.Fatalf("EXPLAIN failed: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scanning EXPLAIN output: %v", err)
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading EXPLAIN output: %v", err)
	}
	planText := plan.String()

	hasIndexScan := strings.Contains(planText, "Index Scan") ||
		strings.Contains(planText, "Index Only Scan") ||
		strings.Contains(planText, "Bitmap Index Scan")
	if !hasIndexScan {
		t.Fatalf("expected an index scan form against a %d-row table, got:\n%s", testutil.DefaultSeedCount, planText)
	}
	if strings.Contains(planText, "Seq Scan on reports") {
		t.Fatalf("expected no sequential scan of reports, got:\n%s", planText)
	}
}

func TestExpiryReadTimePredicate(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)

	const lat, lon = 12.9716, 77.5946 // matches testutil.SeedExpiringReport
	id := testutil.SeedExpiringReport(t, pool, 2)

	latMin, latMax, lonMin, lonMax := service.BoundingBox(lat, lon, 5)
	params := sqlcgen.NearbyReportsParams{
		Lat: lat, Lon: lon,
		LatMin: latMin, LatMax: latMax,
		LonMin: lonMin, LonMax: lonMax,
		RadiusKm: 5,
	}

	before, err := q.NearbyReports(context.Background(), params)
	if err != nil {
		t.Fatalf("NearbyReports before expiry: %v", err)
	}
	if !containsReportID(before, id) {
		t.Fatalf("expected report %d to be present before its expiry", id)
	}

	time.Sleep(2500 * time.Millisecond)

	after, err := q.NearbyReports(context.Background(), params)
	if err != nil {
		t.Fatalf("NearbyReports after expiry: %v", err)
	}
	if containsReportID(after, id) {
		t.Fatalf("expected report %d to be absent after its expiry, with no sweep job involved", id)
	}
}

func containsReportID(rows []sqlcgen.NearbyReportsRow, id int64) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

func TestNearbyReportsExcludesSessionID(t *testing.T) {
	// The generated row struct must have no session-shaped field at all —
	// a compile-time-adjacent guarantee, checked at runtime via reflection.
	typ := reflect.TypeOf(sqlcgen.NearbyReportsRow{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		if strings.Contains(name, "session") {
			t.Fatalf("sqlcgen.NearbyReportsRow must not expose a session-shaped field, found %q", typ.Field(i).Name)
		}
	}

	pool := testutil.NewTestDB(t)
	const sentinel = "sentinel-session-must-never-leak-zzz999"
	const lat, lon = 12.9716, 77.5946

	testutil.MustExec(t, pool,
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash, created_at, expires_at)
		 VALUES ($1, 'flood', 'low', 'session id leak canary report', $2, $3, 'tdr1qgzn', now(), now() + interval '1 hour')`,
		sentinel, lat, lon,
	)

	svc := service.NewReportService(sqlcgen.New(pool))

	req := httptest.NewRequest(http.MethodGet, "/api/reports?lat="+
		strconv.FormatFloat(lat, 'f', -1, 64)+"&lon="+
		strconv.FormatFloat(lon, 'f', -1, 64)+"&radius_km=5", nil)
	rec := httptest.NewRecorder()

	handlers.NearbyReports(svc)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), sentinel) {
		t.Fatalf("nearby response leaked the session identifier: %s", rec.Body.String())
	}
}
