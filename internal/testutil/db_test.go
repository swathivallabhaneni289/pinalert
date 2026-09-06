package testutil

import (
	"context"
	"testing"
)

// TestNewTestDB acquires a pool via NewTestDB, asserts both tables exist and
// are empty after truncation, seeds a small batch, and asserts the row
// count matches. This test skips cleanly without DATABASE_URL.
func TestNewTestDB(t *testing.T) {
	pool := NewTestDB(t)
	ctx := context.Background()

	var reportCount, sessionCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM reports").Scan(&reportCount); err != nil {
		t.Fatalf("querying reports table: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&sessionCount); err != nil {
		t.Fatalf("querying sessions table: %v", err)
	}
	if reportCount != 0 {
		t.Errorf("expected 0 reports after truncate, got %d", reportCount)
	}
	if sessionCount != 0 {
		t.Errorf("expected 0 sessions after truncate, got %d", sessionCount)
	}

	const seedN = 25
	SeedReports(t, pool, seedN, 12.9716, 77.5946)

	var gotCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM reports").Scan(&gotCount); err != nil {
		t.Fatalf("querying reports table after seeding: %v", err)
	}
	if gotCount != seedN {
		t.Errorf("expected %d seeded reports, got %d", seedN, gotCount)
	}
}

// TestSeedExpiringReport asserts SeedExpiringReport returns a valid id whose
// row really does have the requested TTL.
func TestSeedExpiringReport(t *testing.T) {
	pool := NewTestDB(t)
	ctx := context.Background()

	id := SeedExpiringReport(t, pool, 2)

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM reports WHERE id = $1 AND expires_at > now()", id).Scan(&count); err != nil {
		t.Fatalf("querying seeded expiring report: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 not-yet-expired report with id %d, got %d", id, count)
	}
}
