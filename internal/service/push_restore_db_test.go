package service

// push_restore_db_test.go proves on a real Postgres that a report a moderator
// restored after a removal (quick 261118-rz6) never sends a phone alert: the
// removal kept it from pushing while it was withheld, and Restore stamps
// review_kept_at, the same gate that keeps a report kept after a review hide
// from ever pushing (quick 261117-vvb). A report that was never removed is the
// control. It also checks that none of the new bell rows reaches a phone.

import (
	"context"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

func TestPushRestoredReportNeverPushes(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const authorSession = "rz6-push-author"
	testutil.SeedReporterAccount(t, r.pool, r.email("rz6-author"), authorSession)
	mod := testutil.SeedReporterAccount(t, r.pool, r.email("rz6-mod"))
	first, _ := r.reader("rz6-first", 0, 5, true, false, false)
	mods := NewModerationService(r.q, WithModerationTx(PgxModerationTx(r.pool)))

	r.cur = r.base.Add(10 * time.Minute)
	at := r.base.Add(5 * time.Minute)
	control := r.report(authorSession, SeverityCritical, at, "clear")
	removed := r.report(authorSession, SeverityCritical, at, "clear")
	if _, err := mods.Remove(ctx, removed, mod, RemovalFalse); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	for _, id := range []int64{control, removed} {
		r.eval(id, 0)
	}
	if !outboxKindsEqual(r.outbox(control)[first], "critical") {
		t.Fatalf("the control did not push: %v", r.outbox(control)[first])
	}
	if got := r.outbox(removed)[first]; len(got) != 0 {
		t.Errorf("a removed report queued %v", got)
	}

	if _, err := mods.Restore(ctx, removed, mod); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	late, _ := r.reader("rz6-late", 0, 5, true, false, false)
	for _, id := range []int64{control, removed} {
		r.eval(id, 0)
	}
	if !outboxKindsEqual(r.outbox(control)[late], "critical") {
		t.Errorf("the control did not serve the late reader: %v", r.outbox(control)[late])
	}
	for name, reader := range map[string]int64{"the first reader": first, "a late reader": late} {
		if got := r.outbox(removed)[reader]; len(got) != 0 {
			t.Errorf("a restored report was pushed to %s: %v", name, got)
		}
	}
	if n := r.count(`SELECT count(*) FROM updates WHERE report_id = $1 AND kind IN ('own_removed', 'own_restored')`, removed); n != 2 {
		t.Errorf("%d own rows, want own_removed and own_restored", n)
	}
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1`, removed); n != 0 {
		t.Errorf("%d phone rows for the restored report, want 0", n)
	}
}
