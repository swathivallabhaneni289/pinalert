package service

// pushworker_fix_test.go pins the send time rechecks of the Phase 9 code review
// (2026-10-30): WR-S03, K3 and K6. A queued Reliable reporter note and its
// confirmed update stop once the reader muted the reporter or either blocked the
// other; a confirmed update whose first row is gone is skipped, never re-based;
// a queued comment note stops once either side blocked the commenter. The
// database tests use the pushDBRig and skip without DATABASE_URL.

import (
	"context"
	"fmt"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// reliableScene is a Reliable reporter, four readers who each have a device, and
// one low report whose Reliable notes are queued.
type reliableScene struct {
	r        *pushDBRig
	reporter int64
	report   int64
	muter    int64
	blocker  int64 // the reader who blocked the reporter
	blocked  int64 // the reader the reporter blocked
	control  int64
}

// newReliableScene builds the scene. With withRules the mute and the two blocks
// are stored before any bell poll, so the Reliable bell row is written under
// them (review IN-03, quick 261103-jal): a mute keeps the row away from its
// reader, and the control reader's row proves the fixture does alert. Without
// them all four readers hold a bell row and a queued Reliable note.
func newReliableScene(t *testing.T, withRules bool) *reliableScene {
	t.Helper()
	r := newPushDBRig(t)
	const session = "p9-fix-reliable-reporter"
	s := &reliableScene{r: r}
	s.reporter = testutil.SeedReliableReporter(t, r.pool, "p9-fix-reliable-reporter@example.com", session, r.base)
	s.muter, _ = r.reader("muter", 0, 5, false, true, false)
	s.blocker, _ = r.reader("blocker", 0, 5, false, true, false)
	s.blocked, _ = r.reader("blocked", 0, 5, false, true, false)
	s.control, _ = r.reader("control", 0, 5, false, true, false)
	r.cur = r.base.Add(10 * time.Minute)
	s.report = r.report(session, SeverityLow, r.base.Add(5*time.Minute), "clear")
	r.eval(s.report, 0)
	if withRules {
		testutil.SeedMute(t, r.pool, s.muter, s.reporter)
		testutil.SeedBlock(t, r.pool, s.blocker, s.reporter)
		testutil.SeedBlock(t, r.pool, s.reporter, s.blocked)
	}
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box,
	// D3): the dispatcher no longer queues a Reliable note, so the rows are
	// queued by the query that is kept for them, the way a row queued before this
	// change sits in the outbox. The bell rows they read come from each reader's
	// own bell poll (review WR-04: the dispatcher no longer writes them early).
	for _, id := range []int64{s.muter, s.blocker, s.blocked, s.control} {
		r.refresh(id, r.cur)
	}
	if _, err := r.q.EnqueueReliableFromUpdates(context.Background(), sqlcgen.EnqueueReliableFromUpdatesParams{
		ExpiresAt: r.cur.Add(PushOtherTTL), Now: r.cur, ReportID: s.report,
	}); err != nil {
		t.Fatalf("seeding the reliable rows: %v", err)
	}
	for name, id := range map[string]int64{"muter": s.muter, "blocker": s.blocker, "blocked": s.blocked, "control": s.control} {
		if withRules && id == s.muter {
			continue
		}
		if !outboxKindsEqual(r.outbox(s.report)[id], "reliable") {
			t.Fatalf("%s: outbox = %v, want a queued reliable row (the fixture must alert)", name, r.outbox(s.report)[id])
		}
	}
	return s
}

// bellRows counts the Reliable bell rows a reader holds about the scene's report.
func (s *reliableScene) bellRows(id int64) int {
	s.r.t.Helper()
	return s.r.reliableNotes(id, s.report)
}

// wantPersonRules asks the worker's person check directly for each reader. It is
// the defensive layer for a Reliable note queued before 2026-11-03: it answers no
// for the muter and for either side of a block, and yes for the control reader,
// so it fails when the mute or a block is absent, which a skip that every row
// gets (kindSwitchOn) cannot tell.
func (s *reliableScene) wantPersonRules(t *testing.T) {
	t.Helper()
	w := pwDBWorker(s.r, &pwSender{answers: map[string]pwAnswer{}})
	for name, c := range map[string]struct {
		id   int64
		want bool
	}{
		"muter":   {s.muter, false},
		"blocker": {s.blocker, false},
		"blocked": {s.blocked, false},
		"control": {s.control, true},
	} {
		run := &pushRowRun{
			claim: sqlcgen.ClaimDuePushesRow{AccountID: c.id},
			facts: pushReportFacts{ReporterAccountID: &s.reporter},
		}
		got, err := w.personNotMutedOrBlocked(context.Background(), run)
		if err != nil {
			t.Fatalf("%s: personNotMutedOrBlocked: %v", name, err)
		}
		if got != c.want {
			t.Errorf("%s: personNotMutedOrBlocked = %v, want %v", name, got, c.want)
		}
	}
}

func (s *reliableScene) sweep(t *testing.T, sender *pwSender) {
	t.Helper()
	if err := pwDBWorker(s.r, sender).SweepOnce(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
}

// TestQueuedReliablePushStopsAfterMuteOrBlock (WR-S03, D-12): a Reliable
// reporter note queued before the reader muted the reporter, or before either
// blocked the other, is skipped as not eligible at send time.
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D3):
// a Reliable note is never sent now, so every queued row is skipped as not
// eligible, the control reader's too, and that alone cannot tell a mute or a
// block from the blanket skip. Review IN-03 therefore has the test check the
// two places the mute and the block still act: the bell row (a muted reporter
// raises none for its muter, the control reader does get one) and the worker's
// person check, asked directly. It fails when the mute or a block is absent.
func TestQueuedReliablePushStopsAfterMuteOrBlock(t *testing.T) {
	s := newReliableScene(t, true)
	if n := s.bellRows(s.muter); n != 0 {
		t.Errorf("muter: Reliable bell rows = %d, want 0 (a mute keeps the row away)", n)
	}
	if n := s.bellRows(s.control); n != 1 {
		t.Errorf("control: Reliable bell rows = %d, want 1 (the fixture must alert)", n)
	}
	s.wantPersonRules(t)

	sender := &pwSender{answers: map[string]pwAnswer{}}
	s.sweep(t, sender)
	for name, id := range map[string]int64{"blocker": s.blocker, "blocked": s.blocked, "control": s.control} {
		if state, skip, _ := pwOutboxRow(t, s.r, id, s.report, "reliable"); state != "skipped" || skip != "not_eligible" {
			t.Errorf("%s: reliable row is %q (%q), want skipped as not_eligible", name, state, skip)
		}
	}
	if got := len(sender.messages()); got != 0 {
		t.Errorf("messages = %d, want 0 (a Reliable note never reaches a phone)", got)
	}
}

// markReliableSentBeforeTheChange marks every queued reliable row sent with the
// words Not confirmed yet, the way a note sent before 2026-11-03 sits in the
// outbox, so the confirmed update of a Reliable note can be queued.
func (s *reliableScene) markReliableSentBeforeTheChange(t *testing.T) {
	t.Helper()
	testutil.MustExec(t, s.r.pool, `UPDATE push_outbox SET state = 'sent', sent_at = $2, sent_unconfirmed = true
		WHERE report_id = $1 AND kind = 'reliable'`, s.report, s.r.cur)
}

// confirm adds two confirming votes from two cells and runs the dispatcher, so
// each reader holds a confirmed_update row beside the sent Reliable row.
func (s *reliableScene) confirm(t *testing.T) {
	t.Helper()
	r := s.r
	v1 := testutil.SeedReporterAccount(t, r.pool, "p9-fix-voter-1@example.com", "p9-fix-voter-1")
	v2 := testutil.SeedReporterAccount(t, r.pool, "p9-fix-voter-2@example.com", "p9-fix-voter-2")
	r.vote(s.report, v1, "content", "confirm", "tdr1v8y", r.cur)
	r.vote(s.report, v2, "content", "confirm", "tdr1vc3", r.cur)
	r.eval(s.report, v1)
	r.eval(s.report, v2)
	r.eval(s.report, 0)
}

// TestConfirmedUpdateOfReliablePushStopsAfterMuteOrBlock (WR-S03): hours later
// the confirmed update of a sent Reliable note goes through the same person
// check, so a person muted or blocked since then gets nothing.
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D3):
// the confirmed update of a Reliable note is never sent now, so every update is
// skipped as not eligible, with or without a mute or a block. The Reliable notes
// are marked sent by SQL, the way one sent before this change sits in the outbox.
// Review IN-03: the mute and the blocks are stored first, so the muter raised no
// Reliable row and so no confirmed update, and the worker's person check, asked
// directly, still says no for each of them and yes for the control reader.
func TestConfirmedUpdateOfReliablePushStopsAfterMuteOrBlock(t *testing.T) {
	s := newReliableScene(t, true)
	sender := &pwSender{answers: map[string]pwAnswer{}}
	s.markReliableSentBeforeTheChange(t)

	s.confirm(t)
	if got := s.r.outbox(s.report)[s.muter]; len(got) != 0 {
		t.Errorf("muter: outbox = %v, want nothing (a mute keeps the Reliable row, and so its confirmed update, away)", got)
	}
	for name, id := range map[string]int64{"blocker": s.blocker, "blocked": s.blocked, "control": s.control} {
		if !outboxKindsEqual(s.r.outbox(s.report)[id], "confirmed_update", "reliable") {
			t.Fatalf("%s: outbox = %v, want a reliable row and a confirmed update", name, s.r.outbox(s.report)[id])
		}
	}
	s.wantPersonRules(t)

	s.sweep(t, sender)
	for name, id := range map[string]int64{"blocker": s.blocker, "blocked": s.blocked, "control": s.control} {
		if state, skip, _ := pwOutboxRow(t, s.r, id, s.report, "confirmed_update"); state != "skipped" || skip != "not_eligible" {
			t.Errorf("%s: confirmed update is %q (%q), want skipped as not_eligible", name, state, skip)
		}
	}
	if got := len(sender.messages()); got != 0 {
		t.Errorf("messages = %d, want 0 (the confirmed update of a Reliable note never reaches a phone)", got)
	}
}

// TestConfirmedUpdateWithoutItsFirstRowIsSkipped (K3): when the first row of the
// pair was pruned, the confirmed update is skipped as not eligible. It is never
// re-based on Critical or Reliable, which would skip the rechecks of the kind
// it replaced.
func TestConfirmedUpdateWithoutItsFirstRowIsSkipped(t *testing.T) {
	s := newReliableScene(t, false)
	sender := &pwSender{answers: map[string]pwAnswer{}}
	s.markReliableSentBeforeTheChange(t)
	s.confirm(t)

	// The control keeps its first row; the muter's first row was pruned, and the
	// muter muted the reporter since.
	testutil.SeedMute(t, s.r.pool, s.muter, s.reporter)
	testutil.MustExec(t, s.r.pool, `DELETE FROM push_outbox WHERE account_id = $1 AND report_id = $2 AND kind = 'reliable'`, s.muter, s.report)

	s.sweep(t, sender)
	if state, skip, _ := pwOutboxRow(t, s.r, s.muter, s.report, "confirmed_update"); state != "skipped" || skip != "not_eligible" {
		t.Errorf("confirmed update without its first row is %q (%q), want skipped as not_eligible", state, skip)
	}
	// A reader with no mute is skipped too, because the base kind is unknown.
	testutil.MustExec(t, s.r.pool, `DELETE FROM push_outbox WHERE account_id = $1 AND report_id = $2 AND kind = 'reliable'`, s.control, s.report)
	testutil.MustExec(t, s.r.pool, `UPDATE push_outbox SET state = 'queued', skip_reason = NULL, next_attempt_at = now() - interval '1 minute' WHERE account_id = $1 AND report_id = $2 AND kind = 'confirmed_update'`, s.control, s.report)
	s.sweep(t, sender)
	if state, skip, _ := pwOutboxRow(t, s.r, s.control, s.report, "confirmed_update"); state != "skipped" || skip != "not_eligible" {
		t.Errorf("control confirmed update without its first row is %q (%q), want skipped as not_eligible", state, skip)
	}
}

// TestCommentPushStillHoldsAfterABlock (K6): a queued comment note stops once
// the recipient blocked the commenter or the commenter blocked the recipient.
func TestCommentPushStillHoldsAfterABlock(t *testing.T) {
	pool := testutil.NewTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	now := time.Now()
	reporter := testutil.SeedReporterAccount(t, pool, "k6-reporter@example.com", "k6-reporter")
	report := testutil.InsertStoredReport(t, pool, testutil.StoredReport{
		SessionID: "k6-reporter", Category: "flood", Severity: "low", Description: "water",
		Latitude: 16.5062, Longitude: 80.6480, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	})
	holds := func() bool {
		t.Helper()
		got, err := q.CommentPushStillHolds(ctx, sqlcgen.CommentPushStillHoldsParams{
			ReportID: report, RecipientAccountID: reporter, Kind: string(UpdateCommentOnYours),
		})
		if err != nil {
			t.Fatalf("CommentPushStillHolds: %v", err)
		}
		return got
	}
	for i, tc := range []struct {
		name  string
		block func(commenter int64)
	}{
		{"recipient blocked the commenter", func(c int64) { testutil.SeedBlock(t, pool, reporter, c) }},
		{"commenter blocked the recipient", func(c int64) { testutil.SeedBlock(t, pool, c, reporter) }},
	} {
		commenter := testutil.SeedReporterAccount(t, pool, fmt.Sprintf("k6-commenter-%d@example.com", i))
		testutil.SeedComment(t, pool, testutil.CommentSeed{ReportID: report, AccountID: commenter})
		if !holds() {
			t.Fatalf("%s: the note does not hold before the block", tc.name)
		}
		tc.block(commenter)
		if holds() {
			t.Errorf("%s: the note still holds after the block", tc.name)
		}
		testutil.MustExec(t, pool, `DELETE FROM comments WHERE account_id = $1`, commenter)
	}
}
