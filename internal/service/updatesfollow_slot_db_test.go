// updatesfollow_slot_db_test.go proves review WR-02 of quick 261103-jal on a real
// database: a Reliable row never takes the one first alert slot of a report by a
// person the reader follows with Notify me on. The follow phone note is queued
// only from a written follow row, so a Reliable row in the slot would lose it
// for good. The test skips without DATABASE_URL.
//
// Every person here is made up and uses example.com.

package service

import (
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// TestReliableRowNeverTakesTheFollowSlot: the reader is at the follow cap of the
// hour (FollowHourlyCeiling follow rows about other reports) when a followed
// Reliable person posts, so the follow row cannot be written yet. No Reliable row
// appears in its place. Once the hour frees a place the follow row is written,
// and the report never holds a Reliable row. Amended 2026-11-03 by quick
// 261103-jal (follow up F2): the cap counts follow rows only, so the fixture
// fills it with follow rows, not change rows.
func TestReliableRowNeverTakesTheFollowSlot(t *testing.T) {
	r := newFollowRig(t)
	person, personSession := r.person("wr02.followed")
	reader, _ := r.reader("wr02", 0, 5, false, false, false)
	r.bellOn(reader, person, r.base.Add(10*time.Minute))

	// Fill the follow cap with follow rows about other reports.
	const filler = "wr02-filler-sess"
	testutil.SeedReporterAccount(t, r.pool, "wr02-filler@example.com", filler)
	at := r.base.Add(30 * time.Minute)
	for i := 0; i < FollowHourlyCeiling; i++ {
		report := r.report(filler, SeverityLow, at.Add(-time.Duration(i+1)*time.Minute), "clear")
		testutil.MustExec(t, r.pool,
			`INSERT INTO updates (account_id, report_id, kind, created_at) VALUES ($1, $2, 'followed_posted', $3)`,
			reader, report, at.Add(-time.Duration(i)*time.Second))
	}
	followed := r.report(personSession, SeverityLow, at, "clear")

	r.refresh(reader, at.Add(time.Minute))
	if got := r.notes(reader, followed); got != 0 {
		t.Fatalf("follow rows over the ceiling = %d, want 0 (the fixture must be over the ceiling)", got)
	}
	if got := r.reliableNotes(reader, followed); got != 0 {
		t.Fatalf("Reliable rows for a followed person's report = %d, want 0: it would take the follow slot", got)
	}

	// The cap frees: the follow row is written, the slot was still free.
	testutil.MustExec(t, r.pool, `DELETE FROM updates WHERE account_id = $1 AND kind = 'followed_posted'`, reader)
	r.refresh(reader, at.Add(2*time.Minute))
	if got := r.notes(reader, followed); got != 1 {
		t.Errorf("follow rows after the ceiling freed = %d, want 1", got)
	}
	if got := r.reliableNotes(reader, followed); got != 0 {
		t.Errorf("Reliable rows after the ceiling freed = %d, want 0", got)
	}
}
