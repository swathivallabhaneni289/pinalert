// This file seeds the Phase 9 people rows for store and service tests: a person
// with a username, a reporter who reads Reliable and has a public profile, a
// follow and a block. The rows are written with plain SQL, not through the
// follow service, so a test can set up states the service would not produce.
package testutil

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
)

// SeedPerson inserts an account with a clear username and returns its id. When
// public is true the profile is switched on ("Let others follow me") and
// profile_public_at is stamped, as accounts_profile_public_at_set requires. The
// username must follow the username rules (lowercase, 3 to 20 characters).
func SeedPerson(t *testing.T, pool *pgxpool.Pool, email, username string, public bool) int64 {
	t.Helper()

	id := SeedReporterAccount(t, pool, email)
	SeedUsername(t, pool, id, username, "clear")
	if public {
		if _, err := pool.Exec(context.Background(),
			"UPDATE accounts SET profile_public = true, profile_public_at = now() WHERE id = $1", id,
		); err != nil {
			t.Fatalf("switching the profile of account %d on: %v", id, err)
		}
	}
	return id
}

// SeedPublicReporter makes a reporter who reads Reliable (SeedReliableReporter)
// and gives it a clear username, a display name, a home area and a public
// profile. It returns the account id. The voter emails derive from email, so
// call it once per email.
func SeedPublicReporter(t *testing.T, pool *pgxpool.Pool, email, username, sessionID string, now time.Time) int64 {
	t.Helper()

	id := SeedReliableReporter(t, pool, email, sessionID, now)
	if _, err := pool.Exec(context.Background(),
		`UPDATE accounts
		 SET username = $2, username_screen = 'clear', display_name = $2,
		     home_area_name = 'Benz Circle', home_area_cell = 'tdr1v',
		     profile_public = true, profile_public_at = $3
		 WHERE id = $1`, id, username, now,
	); err != nil {
		t.Fatalf("making account %d a public reporter: %v", id, err)
	}
	return id
}

// SeedFollow inserts an active follow created at the given time.
func SeedFollow(t *testing.T, pool *pgxpool.Pool, followerID, followeeID int64, at time.Time) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		"INSERT INTO account_follows (follower_id, followee_id, created_at) VALUES ($1, $2, $3)",
		followerID, followeeID, at,
	); err != nil {
		t.Fatalf("seeding follow %d -> %d: %v", followerID, followeeID, err)
	}
}

// SeedBlock inserts a block of blockedID by blockerID. It does not touch any
// follow between the two.
func SeedBlock(t *testing.T, pool *pgxpool.Pool, blockerID, blockedID int64) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		"INSERT INTO person_blocks (blocker_id, blocked_id) VALUES ($1, $2)",
		blockerID, blockedID,
	); err != nil {
		t.Fatalf("seeding block %d -> %d: %v", blockerID, blockedID, err)
	}
}

// SeedMute inserts a person mute of mutedID by muterID (Phase 9 plan 18, D-12).
// It writes the row with plain SQL, not through the mute service.
func SeedMute(t *testing.T, pool *pgxpool.Pool, muterID, mutedID int64) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		"INSERT INTO person_mutes (muter_id, muted_id) VALUES ($1, $2)",
		muterID, mutedID,
	); err != nil {
		t.Fatalf("seeding mute %d -> %d: %v", muterID, mutedID, err)
	}
}

// raterSeq makes the e-mail of every seeded rater unique across calls, so a test
// can call SeedRaters more than once for the same reporter and report.
var raterSeq atomic.Int64

// raterCellPrecision is the precision of a voter cell (votes.geohash_cell).
const raterCellPrecision = 7

// SeedRaters gives a reporter one rating per entry of stars (Phase 9 plan 13).
// Each rater is a new account with a confirm vote on reportID cast from its own
// distinct precision 7 cell, and a rating row with the given stars, so each one
// counts as a separate rater under the distinct cell rule (D-21). The cells are
// computed from the report's point, stepping 0.003 degrees of latitude per
// rater (about 330 m), and the helper fails the test if two cells ever collide.
// It returns the rater account ids in the order of stars. The report must be one
// filed from a session of the reporter, or the raters would not be eligible.
func SeedRaters(t *testing.T, pool *pgxpool.Pool, reporterID, reportID int64, stars []int) []int64 {
	t.Helper()

	lat, lon := reportPoint(t, pool, reportID)
	seen := map[string]bool{}
	ids := make([]int64, 0, len(stars))
	for i, s := range stars {
		cell := geohash.EncodeWithPrecision(lat+0.003*float64(i+1), lon, raterCellPrecision)
		if seen[cell] {
			t.Fatalf("SeedRaters: rater %d would share the cell %s with an earlier rater", i, cell)
		}
		seen[cell] = true
		ids = append(ids, seedRater(t, pool, reporterID, reportID, cell, s))
	}
	return ids
}

// SeedRatersInOneCell is SeedRaters with every rater voting from the same
// precision 7 cell, the first 7 characters of the report's own geohash (the
// scene). Under the distinct cell rule they count as one rater.
func SeedRatersInOneCell(t *testing.T, pool *pgxpool.Pool, reporterID, reportID int64, stars []int) []int64 {
	t.Helper()

	lat, lon := reportPoint(t, pool, reportID)
	cell := geohash.EncodeWithPrecision(lat, lon, raterCellPrecision)
	ids := make([]int64, 0, len(stars))
	for _, s := range stars {
		ids = append(ids, seedRater(t, pool, reporterID, reportID, cell, s))
	}
	return ids
}

// reportPoint reads the latitude and longitude of a report.
func reportPoint(t *testing.T, pool *pgxpool.Pool, reportID int64) (lat, lon float64) {
	t.Helper()

	if err := pool.QueryRow(context.Background(),
		"SELECT latitude, longitude FROM reports WHERE id = $1", reportID,
	).Scan(&lat, &lon); err != nil {
		t.Fatalf("reading the point of report %d: %v", reportID, err)
	}
	return lat, lon
}

// seedRater inserts one rater account, its confirm vote from cell and its rating.
func seedRater(t *testing.T, pool *pgxpool.Pool, reporterID, reportID int64, cell string, stars int) int64 {
	t.Helper()

	ctx := context.Background()
	id := SeedReporterAccount(t, pool, fmt.Sprintf("seed-rater-%d@example.com", raterSeq.Add(1)))
	if _, err := pool.Exec(ctx,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell)
		 VALUES ($1, $2, 'content', 'confirm', $3)`, reportID, id, cell,
	); err != nil {
		t.Fatalf("seeding the vote of rater %d on report %d: %v", id, reportID, err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO profile_ratings (rater_id, reporter_id, stars) VALUES ($1, $2, $3)`,
		id, reporterID, stars,
	); err != nil {
		t.Fatalf("seeding the rating of rater %d for reporter %d: %v", id, reporterID, err)
	}
	return id
}
