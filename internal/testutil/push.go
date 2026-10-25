// This file seeds the Phase 3.3 phone alert rows for store and service tests: a
// push subscription for an account and its session, and a home area on an
// account.
package testutil

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pushSeedCounter makes every default endpoint unique across a test binary.
var pushSeedCounter atomic.Int64

// SeedPushSubscription inserts a push_subscriptions row for the account and
// the session and returns its id. When endpoint is empty a unique fake https
// endpoint on fcm.googleapis.com is used. The keys are fixed placeholder
// strings, not real key material. The session row is created and bound to the
// account when it does not exist yet, because the subscription references it.
func SeedPushSubscription(t *testing.T, pool *pgxpool.Pool, accountID int64, sessionID, endpoint string) int64 {
	t.Helper()

	ctx := context.Background()
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://fcm.googleapis.com/fcm/send/seed-%d", pushSeedCounter.Add(1))
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO sessions (session_id, account_id) VALUES ($1, $2) ON CONFLICT (session_id) DO NOTHING",
		sessionID, accountID,
	); err != nil {
		t.Fatalf("seeding session %q for account %d: %v", sessionID, accountID, err)
	}
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO push_subscriptions (account_id, session_id, endpoint, endpoint_host, p256dh, auth)
		 VALUES ($1, $2, $3, 'fcm.googleapis.com', 'seed-p256dh', 'seed-auth')
		 RETURNING id`,
		accountID, sessionID, endpoint,
	).Scan(&id); err != nil {
		t.Fatalf("seeding push subscription for account %d: %v", accountID, err)
	}
	return id
}

// SetHomeCell sets the home area name and cell of an account together, so the
// pair check of migration 00008 holds. The cell must be a precision 5 geohash.
func SetHomeCell(t *testing.T, pool *pgxpool.Pool, accountID int64, name, cell string) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		"UPDATE accounts SET home_area_name = $2, home_area_cell = $3 WHERE id = $1",
		accountID, name, cell,
	); err != nil {
		t.Fatalf("setting home cell %q for account %d: %v", cell, accountID, err)
	}
}
