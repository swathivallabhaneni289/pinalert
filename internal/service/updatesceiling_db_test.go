// updatesceiling_db_test.go proves review WR-01 of quick 261103-jal: the
// Reliable reporter rows have their own hourly cap and count toward no other
// kind's ceiling. Since the Reliable row went to every account with a home
// area, a busy hour of them must not stop a follow note (whose phone note is
// queued only from the written row) or roll a change row up into "And N more
// changes". The tests skip without DATABASE_URL.
//
// Every person here is made up and uses example.com.

package service

import (
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// fillReliableRows writes n reliable_nearby rows for the reader, each about a
// different report by an unrelated session, all inside the hour before at.
func (r *followRig) fillReliableRows(reader int64, n int, at time.Time) {
	r.t.Helper()
	const filler = "wr01-filler-sess"
	testutil.SeedReporterAccount(r.t, r.pool, "wr01-filler@example.com", filler)
	for i := 0; i < n; i++ {
		report := r.report(filler, SeverityLow, at.Add(-time.Duration(i+1)*time.Minute), "clear")
		testutil.MustExec(r.t, r.pool,
			`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'reliable_nearby', $3)`,
			reader, report, at.Add(-time.Duration(i)*time.Second))
	}
}

// TestReliableRowsDoNotEatTheSharedCeiling: with ReliableHourlyCeiling Reliable
// rows already in the hour, a person the reader follows with Notify me posts and
// still gets the follow row, and a change on a report the reader voted on is a
// normal row, not a roll up. The Reliable row itself stops at its own cap, and
// writes again once the hour frees a place.
func TestReliableRowsDoNotEatTheSharedCeiling(t *testing.T) {
	r := newFollowRig(t)
	person, personSession := r.person("wr01.followed")
	_, reporterSession := r.person("wr01.reliable")
	reader, _ := r.reader("wr01", 0, 5, false, false, false)
	r.bellOn(reader, person, r.base.Add(10*time.Minute))

	// A report the reader voted on, adopted at Provisional, then confirmed by two
	// more people so it reaches Live after the one hour gap.
	const author = "wr01-author-sess"
	testutil.SeedReporterAccount(t, r.pool, "wr01-author@example.com", author)
	voted := r.report(author, SeverityLow, r.base.Add(11*time.Minute), "clear")
	r.vote(voted, reader, "content", "confirm", "tdr1v8y", r.base.Add(12*time.Minute))
	r.refresh(reader, r.base.Add(13*time.Minute))
	voterA := testutil.SeedReporterAccount(t, r.pool, "wr01-voter-a@example.com")
	voterB := testutil.SeedReporterAccount(t, r.pool, "wr01-voter-b@example.com")
	r.vote(voted, voterA, "content", "confirm", "tdr1vc3", r.base.Add(15*time.Minute))
	r.vote(voted, voterB, "content", "confirm", "tdr1vcb", r.base.Add(16*time.Minute))

	final := r.base.Add(13*time.Minute + UpdateReportGap + time.Minute)
	r.fillReliableRows(reader, ReliableHourlyCeiling, final.Add(-20*time.Minute))
	followed := r.report(personSession, SeverityLow, final.Add(-15*time.Minute), "clear")
	reliable := r.report(reporterSession, SeverityLow, final.Add(-14*time.Minute), "clear")

	r.refresh(reader, final)

	if got := r.notes(reader, followed); got != 1 {
		t.Errorf("follow rows for the followed person's report = %d, want 1 (Reliable rows must not fill its ceiling)", got)
	}
	if got := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reached_live'`, reader, voted); got != 1 {
		t.Errorf("reached_live rows for the report the reader voted on = %d, want 1 (a change row must not roll up)", got)
	}
	if got := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND kind = 'more'`, reader); got != 0 {
		t.Errorf("rolled up rows = %d, want 0", got)
	}
	if got := r.reliableNotes(reader, reliable); got != 0 {
		t.Errorf("Reliable rows past the Reliable cap = %d, want 0", got)
	}

	// The cap is its own: one place freed and the same report is written.
	testutil.MustExec(t, r.pool,
		`DELETE FROM updates WHERE id = (SELECT MIN(id) FROM updates WHERE account_id = $1 AND kind = 'reliable_nearby')`, reader)
	r.refresh(reader, final.Add(time.Minute))
	if got := r.reliableNotes(reader, reliable); got != 1 {
		t.Errorf("Reliable rows after a place freed = %d, want 1", got)
	}
}
