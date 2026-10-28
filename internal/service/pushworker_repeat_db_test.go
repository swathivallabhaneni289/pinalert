// pushworker_repeat_db_test.go proves, on a real database and the real outbox,
// the two rules that came out of test 8 of 03.3-UAT (plan 03.3-16): a Critical
// alert near the home area is never stopped by the hourly cap but still counts
// toward it, and an Apple endpoint gets only the first push about a report while
// other endpoints keep getting later ones. Both run the real dispatcher rows and
// the real worker with a fake push service.
//
// The file needs DATABASE_URL: without it testutil.NewTestDB skips each test.

package service

import (
	"context"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// pwQueueChanged writes a 3.2 "reached_live" update row for the account and a
// queued changed push that points at it, the way the dispatcher does, and
// returns the outbox row id. The row is due at once and lives well past the
// rig's clock.
func pwQueueChanged(t *testing.T, r *pushDBRig, account, reportID int64) int64 {
	t.Helper()
	ctx := context.Background()
	var updateID int64
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'reached_live', $3) RETURNING id`,
		account, reportID, r.cur).Scan(&updateID); err != nil {
		t.Fatalf("writing the update row: %v", err)
	}
	var id int64
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO push_outbox (account_id, report_id, kind, update_id, expires_at)
		 VALUES ($1, $2, 'changed', $3, $4) RETURNING id`,
		account, reportID, updateID, r.cur.Add(2*time.Hour)).Scan(&id); err != nil {
		t.Fatalf("queueing the changed push: %v", err)
	}
	return id
}

// countParams are the arguments of the hourly count for one account.
func countParams(account int64, since time.Time) sqlcgen.CountAccountPushesInWindowParams {
	return sqlcgen.CountAccountPushesInWindowParams{AccountID: account, Since: since}
}

// pwConfirmReport adds two confirmations from two cells, so the report is
// confirmed by people nearby and a "reached_live" row still holds.
func pwConfirmReport(r *pushDBRig, reportID, a, b int64) {
	r.t.Helper()
	r.vote(reportID, a, "content", "confirm", "tdr1v8y", r.cur)
	r.vote(reportID, b, "content", "confirm", "tdr1vc3", r.cur)
}

// TestWorkerCriticalNeverHeldButCounted: with four pushes already sent to a
// person in the rolling hour, a new Critical row is still sent; it counts, so
// the next push of another kind in the same hour is held back as
// cap_account_hour.
func TestWorkerCriticalNeverHeldButCounted(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const reporterSession = "p33-16-cap-reporter"
	reporter := testutil.SeedReporterAccount(t, r.pool, r.email("reporter"), reporterSession)
	// Critical and Changes on, radius 5 km, one Google device.
	reader, _ := r.reader("reader", 1, 5, true, false, true)
	v1 := testutil.SeedReporterAccount(t, r.pool, r.email("v1"), "p33-16-cap-v1")
	v2 := testutil.SeedReporterAccount(t, r.pool, r.email("v2"), "p33-16-cap-v2")

	r.cur = r.base.Add(10 * time.Minute)

	// Four pushes already sent to the reader five minutes ago, each about its own
	// report.
	for i := 0; i < PushPerAccountPerHour; i++ {
		other := r.report(reporterSession, SeverityLow, r.base.Add(time.Duration(i+1)*time.Minute), "clear")
		testutil.MustExec(t, r.pool,
			`INSERT INTO push_outbox (account_id, report_id, kind, state, sent_at, expires_at)
			 VALUES ($1, $2, 'reliable', 'sent', $3, $4)`,
			reader, other, r.cur.Add(-5*time.Minute), r.cur.Add(time.Hour))
	}
	since := r.cur.Add(-time.Hour)
	if n, err := r.q.CountAccountPushesInWindow(ctx, countParams(reader, since)); err != nil || n != PushPerAccountPerHour {
		t.Fatalf("pushes counted before the test = %d (err %v), want %d", n, err, PushPerAccountPerHour)
	}

	reportID := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	testutil.MustExec(t, r.pool, `UPDATE reports SET happening = 'fire' WHERE id = $1`, reportID)
	r.eval(reportID, 0)
	if got := r.outbox(reportID); len(got[reader]) != 1 || got[reader][0] != "critical" || len(got[reporter]) != 0 {
		t.Fatalf("outbox = %v, want one critical row for the reader", got)
	}

	sender := &pwSender{answers: map[string]pwAnswer{}}
	worker := pwDBWorker(r, sender)

	// 1. The Critical row goes out although the hour already holds four pushes.
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if n := len(sender.messages()); n != 1 {
		t.Fatalf("messages after the first sweep = %d, want 1 (a Critical alert is never held by the cap)", n)
	}
	if state, skip, _ := pwOutboxRow(t, r, reader, reportID, "critical"); state != "sent" {
		t.Fatalf("the critical row is %q (%q), want sent", state, skip)
	}

	// 2. It counts: the hour now holds five.
	n, err := r.q.CountAccountPushesInWindow(ctx, countParams(reader, since))
	if err != nil {
		t.Fatal(err)
	}
	if n != PushPerAccountPerHour+1 {
		t.Fatalf("pushes counted after the Critical push = %d, want %d (it counts toward the limit)", n, PushPerAccountPerHour+1)
	}

	// 3. So the next push of another kind in the same hour is held back.
	pwConfirmReport(r, reportID, v1, v2)
	pwQueueChanged(t, r, reader, reportID)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if state, skip, _ := pwOutboxRow(t, r, reader, reportID, "changed"); state != "skipped" || skip != "cap_account_hour" {
		t.Errorf("the changed row is %q (%q), want skipped as cap_account_hour", state, skip)
	}
	if n := len(sender.messages()); n != 1 {
		t.Errorf("messages after the second sweep = %d, want still 1 (nothing extra shown)", n)
	}
}

// TestWorkerAppleGetsFirstPushOnly: a reader with one Apple and one Google
// device gets the first push about a report on both; a later push about the same
// report goes to the Google device only, and the bell row stays; a push about
// another report that was not pushed before reaches both.
func TestWorkerAppleGetsFirstPushOnly(t *testing.T) {
	r := newPushDBRig(t)
	ctx := context.Background()

	const reporterSession = "p33-16-apple-reporter"
	testutil.SeedReporterAccount(t, r.pool, r.email("reporter"), reporterSession)
	// Critical and Changes on, radius 5 km, a Google device from the helper.
	reader, _ := r.reader("reader", 1, 5, true, false, true)
	const appleEndpoint = "https://web.push.apple.com/p33-16-apple-token"
	testutil.SeedPushSubscription(t, r.pool, reader, "p33-16-apple-device", appleEndpoint)
	var googleEndpoint string
	if err := r.pool.QueryRow(ctx,
		`SELECT endpoint FROM push_subscriptions WHERE account_id = $1 AND endpoint <> $2`,
		reader, appleEndpoint).Scan(&googleEndpoint); err != nil {
		t.Fatalf("reading the Google endpoint: %v", err)
	}
	v1 := testutil.SeedReporterAccount(t, r.pool, r.email("v1"), "p33-16-apple-v1")
	v2 := testutil.SeedReporterAccount(t, r.pool, r.email("v2"), "p33-16-apple-v2")

	r.cur = r.base.Add(10 * time.Minute)
	reportR := r.report(reporterSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
	testutil.MustExec(t, r.pool, `UPDATE reports SET happening = 'fire' WHERE id = $1`, reportR)
	r.eval(reportR, 0)

	sender := &pwSender{answers: map[string]pwAnswer{}}
	worker := pwDBWorker(r, sender)
	endpointsOf := func(from int) map[string]bool {
		out := map[string]bool{}
		for _, m := range sender.messages()[from:] {
			out[m.sub.Endpoint] = true
		}
		return out
	}

	// 1. The first alert about R reaches both devices.
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if got := endpointsOf(0); len(sender.messages()) != 2 || !got[appleEndpoint] || !got[googleEndpoint] {
		t.Fatalf("first alert went to %v in %d messages, want both devices", got, len(sender.messages()))
	}

	// 2. A later change about R reaches the Google device only.
	pwConfirmReport(r, reportR, v1, v2)
	pwQueueChanged(t, r, reader, reportR)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if got := endpointsOf(2); len(sender.messages()) != 3 || !got[googleEndpoint] || got[appleEndpoint] {
		t.Fatalf("change about R went to %v in %d messages in total, want the Google device only", got, len(sender.messages()))
	}
	if state, skip, _ := pwOutboxRow(t, r, reader, reportR, "changed"); state != "sent" {
		t.Errorf("the changed row is %q (%q), want sent (to the Google device)", state, skip)
	}
	if n := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reached_live'`, reader, reportR); n != 1 {
		t.Errorf("the reader's bell row for R is gone: %d rows, want 1", n)
	}

	// 3. A change about another report S, never pushed before, reaches both.
	reportS := r.report(reporterSession, SeverityLow, r.base.Add(6*time.Minute), "clear")
	pwConfirmReport(r, reportS, v1, v2)
	pwQueueChanged(t, r, reader, reportS)
	if err := worker.SweepOnce(ctx); err != nil {
		t.Fatalf("third sweep: %v", err)
	}
	if got := endpointsOf(3); len(sender.messages()) != 5 || !got[appleEndpoint] || !got[googleEndpoint] {
		t.Fatalf("change about S went to %v in %d messages in total, want both devices", got, len(sender.messages()))
	}
}
