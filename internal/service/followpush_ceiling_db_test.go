// followpush_ceiling_db_test.go proves follow up F2 of quick 261103-jal on a real
// database: the hourly cap that can hold a follow row counts follow rows only. A
// follow row held back is lost for the phone, because its note is queued only
// from a written row during the one pass. The test skips without DATABASE_URL.
//
// Every person here is made up and uses example.com.

package service

import (
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// TestChangeRowsNeverHoldAFollowRow: a follower with a full hour of change rows
// (the shared ceiling is used up) still gets the follow row for a report by the
// person they follow with Notify me on, and its phone note is queued. The cap
// stays for follow rows: with FollowHourlyCeiling follow rows already in the hour
// the next one is held.
func TestChangeRowsNeverHoldAFollowRow(t *testing.T) {
	r := newFollowPushRig(t)
	person, session := r.person("f2.followed")
	follower := r.follower("f2.reader")
	r.device(follower, "f2-sess-reader")
	r.bellOn(follower, person, r.base.Add(10*time.Minute))

	// A full hour of change rows about other reports.
	const filler = "f2-filler-sess"
	testutil.SeedReporterAccount(t, r.pool, "f2-filler@example.com", filler)
	at := r.base.Add(30 * time.Minute)
	for i := 0; i < UpdatesHourlyCeiling; i++ {
		other := r.report(filler, SeverityLow, at.Add(-time.Duration(i+1)*time.Minute), "clear")
		testutil.MustExec(t, r.pool,
			`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'reached_live', $3)`,
			follower, other, at.Add(-time.Duration(i)*time.Second))
	}

	r.cur = at.Add(time.Minute)
	report := r.report(session, SeverityLow, at, "clear")
	r.eval(report, 0)

	if n := r.notes(follower, report); n != 1 {
		t.Fatalf("follow rows with a full hour of change rows = %d, want 1", n)
	}
	if got := r.outbox(report)[follower]; !outboxKindsEqual(got, "follow") {
		t.Errorf("phone rows for the follower = %v, want one follow row", got)
	}

	// The follow cap itself still holds: FollowHourlyCeiling follow rows in the
	// hour, then one more report by the same person is held.
	for i := 0; i < FollowHourlyCeiling-1; i++ {
		other := r.report(filler, SeverityLow, at.Add(-time.Duration(40+i)*time.Second), "clear")
		testutil.MustExec(t, r.pool,
			`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'followed_posted', $3)`,
			follower, other, at.Add(-time.Duration(i)*time.Second))
	}
	held := r.report(session, SeverityLow, at.Add(10*time.Second), "clear")
	r.eval(held, 0)
	if n := r.notes(follower, held); n != 0 {
		t.Errorf("follow rows past the follow cap = %d, want 0", n)
	}
}
