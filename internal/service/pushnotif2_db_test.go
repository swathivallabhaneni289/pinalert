// pushnotif2_db_test.go proves, on a real database, the delivery rules of the two
// row notifications box (quick 261103-jal, owner decision 2026-11-03, D2, D3 and
// D9): the stored reliable_alerts and changes_push flags never put a Reliable or
// a change note on a phone, at enqueue time or at send time; the stored
// critical_push flag set to off still leaves the Critical note on the phone; and
// a follow note reaches the phone with every stored flag off. The device switch
// is the only way to stop them. Each test needs DATABASE_URL and skips without
// it, so the quick run stays quick.

package service

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestStoredReliableAndChangesFlagsNeverReachThePhone: a reader with a home
// area, a device, and the stored reliable_alerts and changes_push flags on gets
// no Reliable and no change note queued, and the bell still holds the rows
// (the early refresh ran). Rows of those kinds inserted as queued by SQL, the way
// rows queued before the change sit in the outbox, are skipped at send time.
func TestStoredReliableAndChangesFlagsNeverReachThePhone(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const reporterSession = "n2-reliable-reporter"
	testutil.SeedReliableReporter(t, r.pool, r.email("reliable"), reporterSession, r.base)
	// Stored flags: Critical, Reliable and Changes all on.
	reader, readerSession := r.reader("rd", 0, 5, true, true, true)
	v1 := testutil.SeedReporterAccount(t, r.pool, r.email("v1"), "n2-voter-1")
	v2 := testutil.SeedReporterAccount(t, r.pool, r.email("v2"), "n2-voter-2")

	// 1. Enqueue side, Reliable: a plain report by a Reliable reporter near the
	// reader's home area.
	r.cur = r.base.Add(10 * time.Minute)
	relReport := r.report(reporterSession, SeverityLow, r.base.Add(5*time.Minute), "clear")
	r.eval(relReport, 0)
	if rows := r.outbox(relReport)[reader]; len(rows) != 0 {
		t.Fatalf("the reader holds %v for a Reliable report, want no phone row", rows)
	}
	if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reliable_nearby'`, reader, relReport); n != 1 {
		t.Fatalf("reliable_nearby bell rows = %d, want 1 (the early refresh still runs)", n)
	}

	// 2. Enqueue side, Changes: a vote changes a report the reader posted, then
	// the changes sweep runs.
	own := r.report(readerSession, SeverityLow, r.base, "clear")
	r.cur = r.base.Add(1 * time.Minute)
	if err := r.updates.Refresh(ctx, reader); err != nil {
		t.Fatalf("baseline refresh: %v", err)
	}
	r.vote(own, v1, "content", "confirm", "tdr1v8y", r.base.Add(3*time.Minute))
	r.vote(own, v2, "content", "confirm", "tdr1vc3", r.base.Add(4*time.Minute))
	r.cur = r.base.Add(12 * time.Minute)
	r.eval(own, v1)
	if err := r.d.ChangesSweepOnce(ctx); err != nil {
		t.Fatalf("ChangesSweepOnce: %v", err)
	}
	if n := r.count(`SELECT COUNT(*) FROM push_outbox WHERE account_id = $1 AND kind = 'changed'`, reader); n != 0 {
		t.Fatalf("changed phone rows = %d, want none", n)
	}
	if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reached_live'`, reader, own); n != 1 {
		t.Fatalf("reached_live bell rows = %d, want 1 (the bell keeps the change)", n)
	}

	// 3. Send side: rows of kind reliable, changed and confirmed_update (whose
	// base is a Reliable row) inserted as queued are all skipped as not_eligible,
	// and nothing is sent. Report relReport is confirmed so its rows still hold.
	r.vote(relReport, v1, "content", "confirm", "tdr1v8y", r.cur)
	r.vote(relReport, v2, "content", "confirm", "tdr1vc3", r.cur)
	if _, err := r.q.EnqueueReliableFromUpdates(ctx, sqlcgen.EnqueueReliableFromUpdatesParams{
		ExpiresAt: r.cur.Add(PushOtherTTL), Now: r.cur, ReportID: relReport,
	}); err != nil {
		t.Fatalf("seeding a queued reliable row: %v", err)
	}
	if _, err := r.q.EnqueueChangedFromUpdates(ctx, sqlcgen.EnqueueChangedFromUpdatesParams{
		Now: r.cur, AccountID: reader, Since: r.base,
	}); err != nil {
		t.Fatalf("seeding a queued changed row: %v", err)
	}
	testutil.MustExec(t, r.pool,
		`INSERT INTO push_outbox (account_id, report_id, kind, expires_at) VALUES ($1, $2, 'confirmed_update', $3)`,
		reader, relReport, r.cur.Add(time.Hour))
	if got := r.outbox(relReport)[reader]; !outboxKindsEqual(got, "confirmed_update", "reliable") {
		t.Fatalf("the Reliable report holds %v for the reader, want a reliable row and a confirmed update", got)
	}
	if n := r.count(`SELECT COUNT(*) FROM push_outbox WHERE account_id = $1 AND kind = 'changed'`, reader); n != 1 {
		t.Fatalf("seeded changed rows = %d, want 1", n)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwDBWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("worker sweep: %v", err)
	}
	for _, c := range []struct {
		kind   string
		report int64
	}{{"reliable", relReport}, {"confirmed_update", relReport}, {"changed", own}} {
		if state, skip, _ := pwOutboxRow(t, r, reader, c.report, c.kind); state != "skipped" || skip != "not_eligible" {
			t.Errorf("%s row is %q (%q), want skipped as not_eligible", c.kind, state, skip)
		}
	}
	if n := len(sender.messages()); n != 0 {
		t.Errorf("messages sent = %d, want none: a Reliable or change note never reaches a phone", n)
	}
}

// TestStoredCriticalOffStillReachesThePhone: a reader with a device and the
// stored critical_push flag off still gets the Critical note, queued at enqueue
// time and sent by the worker.
func TestStoredCriticalOffStillReachesThePhone(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const reporterSession = "n2-critical-reporter"
	testutil.SeedReporterAccount(t, r.pool, r.email("reporter"), reporterSession)
	// Stored flags: Critical, Reliable and Changes all off.
	reader, _ := r.reader("rd", 0, 5, false, false, false)

	r.cur = r.base.Add(10 * time.Minute)
	report := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	testutil.MustExec(t, r.pool, `UPDATE reports SET happening = 'fire' WHERE id = $1`, report)
	r.eval(report, 0)
	if got := r.outbox(report)[reader]; !outboxKindsEqual(got, "critical") {
		t.Fatalf("the reader holds %v, want one critical row though the stored Critical flag is off", got)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwDBWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("worker sweep: %v", err)
	}
	if state, skip, _ := pwOutboxRow(t, r, reader, report, "critical"); state != "sent" {
		t.Errorf("the critical row is %q (%q), want sent", state, skip)
	}
	if n := len(sender.messages()); n != 1 {
		t.Errorf("messages sent = %d, want 1", n)
	}
}

// TestFollowNoteReachesThePhoneWithEveryFlagOff: a follower with the Notify me
// bell on, a device, and every stored flag false (critical, changes, reliable and
// official) gets the follow note on the phone.
func TestFollowNoteReachesThePhoneWithEveryFlagOff(t *testing.T) {
	r := newFollowPushRig(t)
	ctx := context.Background()

	person, session := r.person("n2.followed")
	follower := r.follower("alloff")
	r.device(follower, "n2-sess-alloff")
	r.setSettings(follower, 5, false, false, false)
	testutil.MustExec(t, r.pool, `UPDATE account_alert_settings SET official_push_on = false WHERE account_id = $1`, follower)
	r.bellOn(follower, person, r.base.Add(10*time.Minute))

	report := r.postedByPerson(session)
	if got := r.outbox(report)[follower]; !outboxKindsEqual(got, "follow") {
		t.Fatalf("the follower holds %v, want one follow row", got)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	if err := pwFollowWorker(r, sender).SweepOnce(ctx); err != nil {
		t.Fatalf("worker sweep: %v", err)
	}
	if state, skip := r.followRowState(follower, report); state != "sent" {
		t.Errorf("the follow row is %q (%q), want sent", state, skip)
	}
	if n := len(sender.messages()); n != 1 {
		t.Errorf("messages sent = %d, want 1", n)
	}
}
