// This file seeds reporter history for the D-10 and D-13 store tests: an
// account bound to one or more sessions, and reports filed from those
// sessions with an exact expiry the test chooses.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
)

// SeedReporterAccount inserts an account with the given email, binds each of
// sessionIDs to it through sessions.account_id, and returns the account id.
func SeedReporterAccount(t *testing.T, pool *pgxpool.Pool, email string, sessionIDs ...string) int64 {
	t.Helper()

	ctx := context.Background()
	var accountID int64
	if err := pool.QueryRow(ctx,
		"INSERT INTO accounts (email) VALUES ($1) RETURNING id", email,
	).Scan(&accountID); err != nil {
		t.Fatalf("seeding account %q: %v", email, err)
	}
	for _, sessionID := range sessionIDs {
		if _, err := pool.Exec(ctx,
			"INSERT INTO sessions (session_id, account_id) VALUES ($1, $2)", sessionID, accountID,
		); err != nil {
			t.Fatalf("seeding session %q for account %d: %v", sessionID, accountID, err)
		}
	}
	return accountID
}

// SeedHistoryReport inserts one report filed from sessionID that expires at
// expiresAt and returns its id. The location is Bengaluru, created_at is
// eight hours before expiresAt, and the shelter fields are null.
func SeedHistoryReport(t *testing.T, pool *pgxpool.Pool, sessionID, severity, category string, expiresAt time.Time) int64 {
	t.Helper()

	lat, lon := 12.9716, 77.5946
	gh := geohash.EncodeWithPrecision(lat, lon, geohashPrecision)

	var id int64
	err := pool.QueryRow(
		context.Background(),
		`INSERT INTO reports
			(session_id, category, severity, description, latitude, longitude, geohash,
			 shelter_capacity_status, shelter_headcount, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULL, NULL, $8, $9)
		 RETURNING id`,
		sessionID, category, severity, "seeded history report",
		lat, lon, gh,
		expiresAt.Add(-8*time.Hour), expiresAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding history report for session %q: %v", sessionID, err)
	}
	return id
}
