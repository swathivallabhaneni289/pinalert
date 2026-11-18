package service

// push_flag_check_db_test.go proves quick 261118-saj on a real Postgres: while
// flags have the Being checked mark on a Critical report, nothing about it is
// queued for a phone (the Critical first alert, the follow note and the
// confirmed update), and after a moderator's Keep it alerts like any shown
// Critical report, to the readers who were waiting and to readers who arrive
// later, with nobody getting the same alert twice. A report kept after a review
// hide, and a restored one, stay silent as before (TestPushReviewHideAndKeep,
// TestPushRestoredReportNeverPushes).

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// markForCheck puts three open flags from three place cells on a report and
// the mark, the state the flag service leaves a flagged serious report in.
func (r *pushDBRig) markForCheck(id int64) {
	r.t.Helper()
	for k := 1; k <= 3; k++ {
		flagger := testutil.SeedReporterAccount(r.t, r.pool, r.email("saj-flagger"))
		testutil.MustExec(r.t, r.pool,
			`INSERT INTO report_flags (report_id, account_id, reasons, flagger_cell) VALUES ($1, $2, ARRAY['false'], $3)`,
			id, flagger, "tdr1v8"+string(rune('0'+k)))
	}
	testutil.MustExec(r.t, r.pool, `UPDATE reports SET flag_check_at = now() WHERE id = $1`, id)
}

func TestPushFlagCheckPausesUntilKeep(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const authorSession = "saj-push-author"
	testutil.SeedReporterAccount(t, r.pool, r.email("saj-author"), authorSession)
	mod := testutil.SeedReporterAccount(t, r.pool, r.email("saj-mod"))
	mods := NewModerationService(r.q, WithModerationTx(PgxModerationTx(r.pool)))
	first, _ := r.reader("saj-first", 0, 5, true, false, false)

	r.cur = r.base.Add(10 * time.Minute)
	at := r.base.Add(5 * time.Minute)
	control := r.report(authorSession, SeverityCritical, at, "clear")
	marked := r.report(authorSession, SeverityCritical, at, "clear")
	r.markForCheck(marked)

	facts, ok, err := loadPushFacts(ctx, r.q, marked, r.cur)
	if err != nil || !ok || !facts.FlagCheck || facts.Kept || facts.Vis != VisibilityLive {
		t.Fatalf("marked facts: FlagCheck %v Kept %v Vis %q (ok %v, err %v)", facts.FlagCheck, facts.Kept, facts.Vis, ok, err)
	}
	for i := 0; i < 2; i++ {
		r.eval(control, 0)
		r.eval(marked, 0)
	}
	if !outboxKindsEqual(r.outbox(control)[first], "critical") {
		t.Fatalf("the control did not push: %v", r.outbox(control)[first])
	}
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1`, marked); n != 0 {
		t.Errorf("%d phone rows queued for a marked report, want 0", n)
	}
	if critical, _ := r.progress(marked); critical != "" {
		t.Errorf("critical state of a marked report = %q, want none, so a run after Keep serves it", critical)
	}

	// Keep from the mark: the report alerts again, to the reader who was
	// waiting and to one who arrives later.
	if _, err := mods.Keep(ctx, marked, mod); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	facts, _, _ = loadPushFacts(ctx, r.q, marked, r.cur)
	if facts.FlagCheck || facts.Kept {
		t.Fatalf("after Keep from the mark: FlagCheck %v Kept %v, want both false", facts.FlagCheck, facts.Kept)
	}
	late, _ := r.reader("saj-late", 0, 5, true, false, false)
	for i := 0; i < 3; i++ {
		r.eval(control, 0)
		r.eval(marked, 0)
	}
	for name, reader := range map[string]int64{"the first reader": first, "a late reader": late} {
		if got := r.outbox(marked)[reader]; !outboxKindsEqual(got, "critical") {
			t.Errorf("after Keep, %s holds %v for the kept report, want one critical row", name, got)
		}
		if got := r.outbox(control)[reader]; !outboxKindsEqual(got, "critical") {
			t.Errorf("%s holds %v for the control, want one critical row", name, got)
		}
	}
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1 AND kind = 'critical'`, marked); n != 2 {
		t.Errorf("%d critical rows for the kept report, want 2: one per reader, never a repeat", n)
	}
	if n := r.count(`SELECT count(*) FROM updates WHERE report_id = $1 AND kind LIKE 'own_%'`, marked); n != 0 {
		t.Errorf("%d own bell rows for a report kept from the mark, want 0", n)
	}
}

// TestPushFlagCheckConfirmedUpdate: a Critical alert sent saying Not confirmed
// yet gets no confirmed update while the mark is on, and gets it once after
// Keep.
func TestPushFlagCheckConfirmedUpdate(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const authorSession = "saj-conf-author"
	testutil.SeedReporterAccount(t, r.pool, r.email("saj-conf-author"), authorSession)
	mod := testutil.SeedReporterAccount(t, r.pool, r.email("saj-conf-mod"))
	mods := NewModerationService(r.q, WithModerationTx(PgxModerationTx(r.pool)))
	reader, _ := r.reader("saj-conf-reader", 0, 5, true, false, false)

	r.cur = r.base.Add(10 * time.Minute)
	id := r.report(authorSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	r.eval(id, 0)
	if !outboxKindsEqual(r.outbox(id)[reader], "critical") {
		t.Fatalf("no first alert: %v", r.outbox(id)[reader])
	}
	testutil.MustExec(t, r.pool, `UPDATE push_outbox SET state = 'sent', sent_unconfirmed = true WHERE report_id = $1`, id)

	// Two people at two spots confirm it, so it is confirmed by people, and
	// flags then mark it.
	for i := 0; i < 2; i++ {
		voter := testutil.SeedReporterAccount(t, r.pool, r.email("saj-conf-voter"))
		r.vote(id, voter, "content", "confirm", r.cell(i)+"bc", r.base.Add(6*time.Minute))
	}
	r.markForCheck(id)
	facts, _, err := loadPushFacts(ctx, r.q, id, r.cur)
	if err != nil || !facts.Confirmed || !facts.FlagCheck {
		t.Fatalf("fixture: Confirmed %v FlagCheck %v err %v", facts.Confirmed, facts.FlagCheck, err)
	}
	r.eval(id, 0)
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1 AND kind = 'confirmed_update'`, id); n != 0 {
		t.Errorf("%d confirmed updates queued while marked, want 0", n)
	}
	if _, err := mods.Keep(ctx, id, mod); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	r.eval(id, 0)
	r.eval(id, 0)
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1 AND kind = 'confirmed_update'`, id); n != 1 {
		t.Errorf("%d confirmed updates after Keep, want 1", n)
	}
}

// TestPushFlagCheckChangedNotQueued: the changed note query, kept with its
// tests though no longer called, queues nothing for a marked report.
func TestPushFlagCheckChangedNotQueued(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const authorSession = "saj-changed-author"
	testutil.SeedReporterAccount(t, r.pool, r.email("saj-changed-author"), authorSession)
	reader, _ := r.reader("saj-changed-reader", 0, 5, true, false, true)
	r.cur = r.base.Add(10 * time.Minute)
	marked := r.report(authorSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	shown := r.report(authorSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	r.markForCheck(marked)
	for _, id := range []int64{marked, shown} {
		testutil.MustExec(t, r.pool, `INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'fading', $3)`, reader, id, r.cur)
	}
	if _, err := r.q.EnqueueChangedFromUpdates(ctx, sqlcgen.EnqueueChangedFromUpdatesParams{
		Now: r.cur, AccountID: reader, Since: r.base,
	}); err != nil {
		t.Fatalf("EnqueueChangedFromUpdates: %v", err)
	}
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1`, marked); n != 0 {
		t.Errorf("%d changed rows for a marked report, want 0", n)
	}
	if n := r.count(`SELECT count(*) FROM push_outbox WHERE report_id = $1 AND kind = 'changed'`, shown); n != 1 {
		t.Errorf("%d changed rows for the shown control, want 1", n)
	}
}
