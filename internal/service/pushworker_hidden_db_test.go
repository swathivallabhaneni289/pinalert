// pushworker_hidden_db_test.go proves on a real database that the now disputed
// note reaches a phone only for a report people nearby had confirmed before
// (plan 03.3-18, the 03.3-UAT gap for test 8, owner 2026-10-28): the owner wants
// phone notifications few, so a report nobody confirmed that is then disputed
// stays in the bell. It runs the real updates service, the real dispatcher change
// sweep and the real worker over the real vote log, with a fake push service. It
// needs DATABASE_URL and skips without it, so the quick run stays quick.

package service

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// pwHiddenRows reads, for one account and report, how many bell rows of kind
// hidden exist, and the state and skip reason of the changed outbox row that
// carries that hidden row. Zero rows in the outbox gives an empty state.
func pwHiddenRows(t *testing.T, r *pushDBRig, account, reportID int64) (bell int, state, skip string) {
	t.Helper()
	bell = r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'hidden'`, account, reportID)
	var reason *string
	err := r.pool.QueryRow(context.Background(),
		`SELECT o.state, o.skip_reason
		   FROM push_outbox o
		   JOIN updates u ON u.id = o.update_id
		  WHERE o.account_id = $1 AND o.report_id = $2 AND o.kind = 'changed' AND u.kind = 'hidden'`,
		account, reportID).Scan(&state, &reason)
	if err != nil {
		t.Fatalf("reading the changed hidden row of report %d: %v", reportID, err)
	}
	if reason != nil {
		skip = *reason
	}
	return bell, state, skip
}

// TestWorkerHiddenNeedsEarlierConfirmation: one reporter with Changes on posted
// two low severity reports. Report A was confirmed from enough cells (Live) and
// later disputed by more cells (Hidden). Report B was only ever disputed. Both
// get a now disputed row in the bell, but only A's note is pushed; B's changed
// push is skipped as not_eligible.
func TestWorkerHiddenNeedsEarlierConfirmation(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	reporter, reporterSession := r.reader("rep", 1, 5, false, false, true)

	var voters [5]int64
	for i := range voters {
		voters[i] = testutil.SeedReporterAccount(t, r.pool, r.email("v"), "p33-18-voter-"+string(rune('a'+i)))
	}
	confirmCells := []string{"tdr1v8y", "tdr1vc3"}
	disputeCells := []string{"tdr1w01", "tdr1w02", "tdr1w03"}

	// Both reports are posted at the start; the reporter's bell learns about them
	// with a silent baseline.
	r.cur = r.base.Add(1 * time.Minute)
	reportA := r.report(reporterSession, SeverityLow, r.base, "clear")
	reportB := r.report(reporterSession, SeverityLow, r.base, "clear")
	if err := r.updates.Refresh(ctx, reporter); err != nil {
		t.Fatalf("baseline refresh: %v", err)
	}

	// A is confirmed from two cells and the bell says it is now live.
	r.vote(reportA, voters[0], "content", "confirm", confirmCells[0], r.base.Add(3*time.Minute))
	r.vote(reportA, voters[1], "content", "confirm", confirmCells[1], r.base.Add(4*time.Minute))
	r.cur = r.base.Add(5 * time.Minute)
	if err := r.updates.Refresh(ctx, reporter); err != nil {
		t.Fatalf("refresh after the confirmations: %v", err)
	}
	if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reached_live'`, reporter, reportA); n != 1 {
		t.Fatalf("reached_live rows for report A = %d, want 1 (the fixture needs A to have been live)", n)
	}

	// More than an hour later three other cells dispute both reports, so both are
	// Hidden now. The change sweep refreshes the reporter and queues the pushes.
	for i, cell := range disputeCells {
		at := r.base.Add(time.Duration(70+i) * time.Minute)
		r.vote(reportA, voters[2+i], "content", "dispute", cell, at)
		r.vote(reportB, voters[2+i], "content", "dispute", cell, at)
	}
	r.cur = r.base.Add(75 * time.Minute)
	if err := r.d.ChangesSweepOnce(ctx); err != nil {
		t.Fatalf("ChangesSweepOnce: %v", err)
	}
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
	// D3): the sweep refreshes the bell and queues no change note any more, so the
	// two rows are queued by the query that is kept for them, the way rows queued
	// before this change sit in the outbox.
	if _, err := r.q.EnqueueChangedFromUpdates(ctx, sqlcgen.EnqueueChangedFromUpdatesParams{
		Now: r.cur, AccountID: reporter, Since: r.base,
	}); err != nil {
		t.Fatalf("seeding the changed rows: %v", err)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwDBWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("worker sweep: %v", err)
	}

	// The bell keeps the now disputed row for both reports, pushed or not.
	bellA, stateA, skipA := pwHiddenRows(t, r, reporter, reportA)
	bellB, stateB, skipB := pwHiddenRows(t, r, reporter, reportB)
	if bellA != 1 || bellB != 1 {
		t.Errorf("hidden bell rows: report A %d, report B %d, want 1 and 1", bellA, bellB)
	}

	// Both notes are skipped now: a change note never reaches a phone (D3). The
	// earlier confirmation rule stays pinned by TestReportWasConfirmed.
	if stateA != "skipped" || skipA != "not_eligible" {
		t.Errorf("report A (confirmed, then disputed): changed push is %q (%q), want skipped as not_eligible", stateA, skipA)
	}
	if stateB != "skipped" || skipB != "not_eligible" {
		t.Errorf("report B (only disputed): changed push is %q (%q), want skipped as not_eligible", stateB, skipB)
	}
	if n := len(sender.messages()); n != 0 {
		t.Errorf("messages sent = %d, want none", n)
	}
}
