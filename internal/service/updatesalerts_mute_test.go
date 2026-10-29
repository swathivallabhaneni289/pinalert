// updatesalerts_mute_test.go proves that person mute (plan 09-18, D-12, T-09-65)
// stops the Reliable reporter alert for the muted person and nothing else: the
// Critical phone path and the changes rows for a report the reader voted on keep
// arriving from a muted person, because they are about the place and the
// reader's own reports, not about the person. The tests run on a real database
// with the real services (the pushDBRig of pushdispatch_db_test.go) and skip
// without DATABASE_URL.

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"pinalert/internal/testutil"
)

// reliableNotes counts the Reliable reporter notes a reader holds about a report.
func (r *pushDBRig) reliableNotes(reader, report int64) int {
	r.t.Helper()
	return r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reliable_nearby'`, reader, report)
}

func (r *pushDBRig) refresh(reader int64, at time.Time) {
	r.t.Helper()
	r.cur = at
	if err := r.updates.Refresh(context.Background(), reader); err != nil {
		r.t.Fatalf("Refresh at %s: %v", at.Format(time.RFC3339), err)
	}
}

// TestReliableAlertSkipsMutedPerson: with the Reliable alert on, a muted
// Reliable reporter's reports (critical or not) raise no reliable_nearby row,
// an unmuted Reliable reporter's report still does, and the muted person's
// report alerts again once the mute is taken back, which shows the mute was the
// cause.
func TestReliableAlertSkipsMutedPerson(t *testing.T) {
	r := newPushDBRig(t)

	const sessionA, sessionB = "p9-18-reliable-a", "p9-18-reliable-b"
	reporterA := testutil.SeedReliableReporter(t, r.pool, "p9-18-reliable-a@example.com", sessionA, r.base)
	testutil.SeedReliableReporter(t, r.pool, "p9-18-reliable-b@example.com", sessionB, r.base)
	reader, _ := r.reader("reader", 0, 5, true, true, false)
	testutil.SeedMute(t, r.pool, reader, reporterA)

	lowA := r.report(sessionA, SeverityLow, r.base.Add(5*time.Minute), "clear")
	lowB := r.report(sessionB, SeverityLow, r.base.Add(6*time.Minute), "clear")
	criticalA := r.report(sessionA, SeverityCritical, r.base.Add(7*time.Minute), "clear")

	for i := 0; i < 2; i++ {
		r.refresh(reader, r.base.Add(10*time.Minute+time.Duration(i)*time.Minute))
	}
	if got := r.reliableNotes(reader, lowB); got != 1 {
		t.Fatalf("notes about the unmuted Reliable reporter's report = %d, want 1 (the fixture must alert)", got)
	}
	if got := r.reliableNotes(reader, lowA); got != 0 {
		t.Errorf("notes about the muted person's report = %d, want 0", got)
	}
	if got := r.reliableNotes(reader, criticalA); got != 0 {
		t.Errorf("notes about the muted person's critical report = %d, want 0", got)
	}

	// Take the mute back: the same report alerts, so the mute was the only cause.
	testutil.MustExec(t, r.pool, "DELETE FROM person_mutes WHERE muter_id = $1 AND muted_id = $2", reader, reporterA)
	r.refresh(reader, r.base.Add(20*time.Minute))
	if got := r.reliableNotes(reader, lowA); got != 1 {
		t.Errorf("notes about the person's report after Unmute = %d, want 1", got)
	}

	// Someone else's mute does not silence this reader.
	other, _ := r.reader("other", 0, 5, true, true, false)
	testutil.SeedMute(t, r.pool, other, reporterA)
	r.refresh(reader, r.base.Add(25*time.Minute))
	if got := r.reliableNotes(reader, lowA); got != 1 {
		t.Errorf("notes about the report for the reader after another person's mute = %d, want 1", got)
	}
}

// TestCriticalStillArrivesFromMutedPerson: a Critical report by a muted person
// is still queued for the reader's phone by the dispatcher, and a report the
// reader voted on still gets its changes row (D-12: hiding emergency
// information is never safe).
func TestCriticalStillArrivesFromMutedPerson(t *testing.T) {
	r := newPushDBRig(t)

	const authorSession = "p9-18-muted-author"
	author := testutil.SeedReporterAccount(t, r.pool, "p9-18-muted-author@example.com", authorSession)
	reader, _ := r.reader("reader", 0, 5, true, false, false)
	testutil.SeedMute(t, r.pool, reader, author)

	t.Run("the critical phone path", func(t *testing.T) {
		r.cur = r.base.Add(10 * time.Minute)
		critical := r.report(authorSession, SeverityCritical, r.base.Add(5*time.Minute), "clear")
		r.eval(critical, 0)
		if got := r.outbox(critical)[reader]; !outboxKindsEqual(got, "critical") {
			t.Errorf("outbox of the reader for a muted person's critical report = %v, want [critical]", got)
		}
	})

	t.Run("a changes row for a report the reader voted on", func(t *testing.T) {
		low := r.report(authorSession, SeverityLow, r.base.Add(11*time.Minute), "clear")
		r.vote(low, reader, "content", "confirm", "tdr1v8y", r.base.Add(12*time.Minute))
		r.refresh(reader, r.base.Add(13*time.Minute)) // adopts the report at Provisional

		voterA := testutil.SeedReporterAccount(t, r.pool, "p9-18-voter-a@example.com")
		voterB := testutil.SeedReporterAccount(t, r.pool, "p9-18-voter-b@example.com")
		r.vote(low, voterA, "content", "confirm", "tdr1vc3", r.base.Add(15*time.Minute))
		r.vote(low, voterB, "content", "confirm", "tdr1vcb", r.base.Add(16*time.Minute))
		// Past the one update per report per hour gap after the adoption, so the
		// row is not held back by D-12 for an unrelated reason.
		r.refresh(reader, r.base.Add(13*time.Minute+UpdateReportGap+time.Minute))

		if got := r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'reached_live'`, reader, low); got != 1 {
			t.Errorf("reached_live rows for a report by a muted person the reader voted on = %d, want 1", got)
		}
	})
}

// mutedErrFake makes the muted people read fail, to pin that the Reliable alert
// step returns the error like its other reads instead of alerting through it.
type mutedErrFake struct {
	*alertFake
	err error
}

func (f *mutedErrFake) MutedPeopleIDs(_ context.Context, _ int64) ([]int64, error) {
	return nil, f.err
}

// TestReliableAlertMutedReadFailure: a failed read of the muted set stops the
// step with an error and writes no note (fail closed on the person level note).
func TestReliableAlertMutedReadFailure(t *testing.T) {
	boom := errors.New("boom")
	f := &mutedErrFake{
		alertFake: &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}, settings: alertOn(5), homeCell: cellPtr("tdr1v")},
		err:       boom,
	}
	n := &fakeNearby{views: []ReportView{alertView(1, VisibilityLive, ReporterReliable, 30*time.Minute)}}
	svc := NewUpdatesService(f, n, WithUpdatesClock(func() time.Time { return updatesNow }))
	err := svc.refreshReliableAlerts(context.Background(), 5, updatesNow)
	if !errors.Is(err, boom) {
		t.Fatalf("refreshReliableAlerts = %v, want the read error", err)
	}
	if len(f.inserted) != 0 {
		t.Errorf("%d notes written after a failed muted read, want none", len(f.inserted))
	}
}

var _ UpdatesQuerier = (*mutedErrFake)(nil)
