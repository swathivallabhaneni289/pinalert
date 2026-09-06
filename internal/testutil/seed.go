package testutil

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
)

// DefaultSeedCount is the row count callers should default to when seeding
// data for the FOUND-03 index-scan assertion: Postgres deliberately
// sequential-scans small tables because that's genuinely cheaper, so the
// planner only prefers an index once the table is large enough.
const DefaultSeedCount = 50_000

// geohashPrecision matches 01-RESEARCH.md's Open Questions recommendation:
// finer than Phase 3's eventual diversity-weighting cell size is always
// safely truncatable later, coarser would require a backfill.
const geohashPrecision = 8

var seedCategories = []string{
	"flood",
	"earthquake",
	"fire",
	"storm_cyclone_damage",
	"road_blocked",
	"power_outage",
	"shelter_open",
	"rescue_needed",
	"other",
}

var seedSeverities = []string{"low", "medium", "critical"}

// SeedReports bulk-inserts n synthetic report rows via pgx.CopyFrom (not n
// individual INSERTs — a 50,000-row loop of round-trips would dominate test
// runtime), spreading latitude/longitude pseudo-randomly within roughly ±20
// degrees of (centreLat, centreLon) from a fixed seed so runs are
// reproducible. Rows cycle through every category and severity value and
// expire comfortably in the future.
func SeedReports(t *testing.T, pool *pgxpool.Pool, n int, centreLat, centreLon float64) {
	t.Helper()

	rng := rand.New(rand.NewSource(42)) // fixed seed: reproducible across runs
	now := time.Now().UTC()

	rows := make([][]any, 0, n)
	for i := 0; i < n; i++ {
		lat := clampLat(centreLat + (rng.Float64()*40 - 20)) // ±20 degrees
		lon := clampLon(centreLon + (rng.Float64()*40 - 20))
		category := seedCategories[i%len(seedCategories)]
		severity := seedSeverities[i%len(seedSeverities)]
		gh := geohash.EncodeWithPrecision(lat, lon, geohashPrecision)

		rows = append(rows, []any{
			"seed-session-" + severity, // session_id (not a real per-visitor identity, fine for seed data)
			category,
			severity,
			"seeded test report",
			lat,
			lon,
			gh,
			nil, // shelter_capacity_status
			nil, // shelter_headcount
			now,
			now.Add(24 * time.Hour), // expires comfortably in the future
		})
	}

	copyCount, err := pool.CopyFrom(
		context.Background(),
		pgx.Identifier{"reports"},
		[]string{
			"session_id", "category", "severity", "description",
			"latitude", "longitude", "geohash",
			"shelter_capacity_status", "shelter_headcount",
			"created_at", "expires_at",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		t.Fatalf("seeding %d reports via CopyFrom: %v", n, err)
	}
	if int(copyCount) != n {
		t.Fatalf("expected CopyFrom to insert %d rows, inserted %d", n, copyCount)
	}
}

// SeedExpiringReport inserts a single report whose expires_at is
// secondsUntilExpiry seconds in the future and returns its id. Used by the
// FOUND-05 read-time-expiry test, which needs a report that will still be
// live at insert time but expired a few seconds later.
func SeedExpiringReport(t *testing.T, pool *pgxpool.Pool, secondsUntilExpiry int) int64 {
	t.Helper()

	now := time.Now().UTC()
	lat, lon := 12.9716, 77.5946 // Bengaluru, an arbitrary but realistic default
	gh := geohash.EncodeWithPrecision(lat, lon, geohashPrecision)

	var id int64
	err := pool.QueryRow(
		context.Background(),
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 shelter_capacity_status, shelter_headcount, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9)
		 RETURNING id`,
		"seed-session-expiring",
		"flood",
		"low",
		"seeded expiring test report",
		lat,
		lon,
		gh,
		now,
		now.Add(time.Duration(secondsUntilExpiry)*time.Second),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding expiring report: %v", err)
	}
	return id
}

func clampLat(lat float64) float64 {
	if lat < -90 {
		return -90
	}
	if lat > 90 {
		return 90
	}
	return lat
}

func clampLon(lon float64) float64 {
	if lon < -180 {
		return -180
	}
	if lon > 180 {
		return 180
	}
	return lon
}
