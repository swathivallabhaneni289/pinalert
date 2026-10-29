// This file seeds the Phase 9 people rows for store and service tests: a person
// with a username, a reporter who reads Reliable and has a public profile, a
// follow and a block. The rows are written with plain SQL, not through the
// follow service, so a test can set up states the service would not produce.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
