package service_test

// ratings_fix_test.go pins the Phase 9 code review fixes in the rating service
// (2026-10-30): WR-S01, WR-S02 and WR-S05. The tests skip without DATABASE_URL.

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// TestModeratorRemovalRestoresAfterTheRatingWasChanged (WR-S01): the restore
// window starts when the rating first existed, so a rater who changed the stars
// after the follows ended cannot make the moderator's removal a no op.
func TestModeratorRemovalRestoresAfterTheRatingWasChanged(t *testing.T) {
	s := newRatingScene(t)
	ctx := context.Background()
	active, _ := s.followScene()
	testutil.SeedRaters(t, s.pool, s.reporter, s.report, []int{1, 1, 1, 1})
	fifth := s.newRater("fifth.rater", "content", "confirm", "tdr1x55")
	mod := s.newRater("the.moderator", "", "", "")

	s.at = s.at.Add(time.Hour)
	if err := s.svc.Rate(ctx, fifth, s.username, 1); err != nil {
		t.Fatalf("Rate 1: %v", err)
	}
	if n := s.count(`SELECT count(*) FROM account_follows WHERE followee_id = $1 AND end_reason = 'tag_lost'`, s.reporter); n == 0 {
		t.Fatal("the fifth low rating did not end any follow")
	}

	// A day later the rater changes the stars, so updated_at moves past the end.
	s.at = s.at.Add(25 * time.Hour)
	if err := s.svc.Rate(ctx, fifth, s.username, 2); err != nil {
		t.Fatalf("Rate 2: %v", err)
	}
	s.at = s.at.Add(time.Hour)
	if err := s.svc.ModeratorRemove(ctx, mod, s.username, "fifth.rater"); err != nil {
		t.Fatalf("ModeratorRemove: %v", err)
	}
	if sig := s.signal(s.reporter); sig.Tag != service.ReporterReliable {
		t.Fatalf("after the removal the tag = %s, want Reliable", sig.Tag)
	}
	for _, id := range active {
		if n := s.count(`SELECT count(*) FROM account_follows WHERE follower_id = $1 AND followee_id = $2 AND ended_at IS NULL`, id, s.reporter); n != 1 {
			t.Errorf("follower %d: restored rows = %d, want 1", id, n)
		}
	}
}
