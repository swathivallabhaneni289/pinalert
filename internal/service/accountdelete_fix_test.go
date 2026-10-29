package service

// accountdelete_fix_test.go pins IN-E06 of the Phase 9 code review (2026-10-30):
// deleting an account also deletes the followers' queued follow phone notes
// about that person's reports. It skips without DATABASE_URL.

import (
	"context"
	"testing"
)

func TestDeleteAccountRemovesQueuedFollowNotesAboutThem(t *testing.T) {
	s := newCleanupScene(t)
	for _, f := range s.followers {
		if n := s.r.followRowsIn(f, "queued"); n != 1 {
			t.Fatalf("seed failed: follower %d holds %d queued follow rows, want 1", f, n)
		}
	}
	svc := NewAccountDeletionService(s.r.q, PgxAccountTx(s.r.pool))
	if err := svc.DeleteAccount(context.Background(), s.person); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	for _, f := range s.followers {
		if n := s.r.followRowsIn(f, "queued"); n != 0 {
			t.Errorf("follower %d: queued follow rows after the deletion = %d, want 0", f, n)
		}
		if n := s.r.followRowsIn(f, "sent"); n != 1 {
			t.Errorf("follower %d: sent follow rows after the deletion = %d, want 1 (history stays)", f, n)
		}
	}
}
