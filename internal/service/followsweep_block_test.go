package service

// followsweep_block_test.go pins IN-S01 and IN-E01 of the Phase 9 code review
// (2026-10-30): a Follow that raced a Block can leave an active follow beside the
// block, and the follow sweep ends it with reason blocked, in either direction.
// The test skips without DATABASE_URL.

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

func TestFollowSweepEndsAFollowBesideABlock(t *testing.T) {
	pool, q, t0, followee := sweepRig(t, "blockedpair")

	// The two followers of the rig: one blocked the followee, the followee
	// blocked the other. The follow rows are active, as a lost race leaves them.
	var followers []int64
	rows, err := pool.Query(context.Background(), `SELECT follower_id FROM account_follows WHERE followee_id = $1 AND ended_at IS NULL ORDER BY follower_id`, followee)
	if err != nil {
		t.Fatalf("reading the followers: %v", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning a follower: %v", err)
		}
		followers = append(followers, id)
	}
	rows.Close()
	if len(followers) != 2 {
		t.Fatalf("followers = %d, want 2", len(followers))
	}
	testutil.SeedBlock(t, pool, followers[0], followee)
	testutil.SeedBlock(t, pool, followee, followers[1])

	sweep := NewFollowSweep(q, WithFollowSweepClock(func() time.Time { return t0 }))
	ended, err := sweep.SweepOnce(context.Background())
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if ended != 2 {
		t.Errorf("ended = %d, want 2", ended)
	}
	var blocked, active int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FILTER (WHERE end_reason = 'blocked'), count(*) FILTER (WHERE ended_at IS NULL)
		 FROM account_follows WHERE followee_id = $1`, followee).Scan(&blocked, &active); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if blocked != 2 || active != 0 {
		t.Errorf("blocked ends = %d, active = %d; want 2 and 0", blocked, active)
	}
}
