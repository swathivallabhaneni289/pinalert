// updatesfollow_test.go proves the follow note (plan 09-22, PROF-03, D-09, D-12,
// SC5): the producer step of Refresh and the words of its rows in the list. The
// database tests run the real report and updates services on the pushDBRig
// clock, with the rating source wired the way the router wires it, so a
// reporter's tag is the rated tag. They skip without DATABASE_URL. The fake
// tests prove the order of the two first alert steps and the two bounds.
//
// Every person here is made up and uses example.com.

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// followRig is the push rig with the report service wired to the rating source,
// so a reporter whose raters pulled the points down reads below Reliable
// reporter exactly as in the app.
type followRig struct {
	*pushDBRig
	reports *ReportService
	seq2    int
}

func newFollowRig(t *testing.T) *followRig {
	t.Helper()
	base := newPushDBRig(t)
	r := &followRig{pushDBRig: base}
	clock := func() time.Time { return r.cur }
	r.reports = NewReportService(r.q, WithReportClock(clock), WithReportSignals(NewRatingService(r.q)))
	r.updates = NewUpdatesService(r.q, r.reports, WithUpdatesClock(clock))
	return r
}

// person makes a public reporter who reads Reliable reporter, with a clear
// username, and returns the account id and its session id.
func (r *followRig) person(username string) (int64, string) {
	r.t.Helper()
	r.seq2++
	session := fmt.Sprintf("p9-22-sess-%s-%d", username, r.seq2)
	id := testutil.SeedPublicReporter(r.t, r.pool, fmt.Sprintf("p9-22-%s-%d@example.com", username, r.seq2), username, session, r.base)
	return id, session
}

// follower makes a reader account with no settings row, so no other alert is on.
func (r *followRig) follower(name string) int64 {
	r.t.Helper()
	r.seq2++
	return testutil.SeedReporterAccount(r.t, r.pool, fmt.Sprintf("p9-22-%s-%d@example.com", name, r.seq2))
}

// bellOn makes an active follow from the start of the rig and turns the bell on
// at the given time.
func (r *followRig) bellOn(follower, followee int64, at time.Time) {
	r.t.Helper()
	testutil.SeedFollow(r.t, r.pool, follower, followee, r.base)
	r.setBell(follower, followee, true, at)
}

func (r *followRig) setBell(follower, followee int64, on bool, at time.Time) {
	r.t.Helper()
	n, err := r.q.SetFollowNotify(context.Background(), sqlcgen.SetFollowNotifyParams{Notify: on, Now: at, FollowerID: follower, FolloweeID: followee})
	if err != nil || n != 1 {
		r.t.Fatalf("SetFollowNotify(%d, %d, %v) = %d, %v; want 1 row", follower, followee, on, n, err)
	}
}

// notes counts the follow notes a reader holds about a report.
func (r *followRig) notes(reader, report int64) int {
	r.t.Helper()
	return r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'followed_posted'`, reader, report)
}

// firstAlerts counts both first alert kinds a reader holds about a report.
func (r *followRig) firstAlerts(reader, report int64) int {
	r.t.Helper()
	return r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND report_id = $2 AND kind IN ('reliable_nearby', 'followed_posted')`, reader, report)
}

func (r *followRig) allNotes(reader int64) int {
	r.t.Helper()
	return r.count(`SELECT COUNT(*) FROM updates WHERE account_id = $1 AND kind = 'followed_posted'`, reader)
}

// ratedDown gives a person five one star raters, so the combined points fall
// below the Reliable reporter line and the tag reads below it now.
func (r *followRig) ratedDown(person int64, session string) {
	r.t.Helper()
	rated := testutil.SeedHistoryReport(r.t, r.pool, session, "low", "flood", r.base.Add(-time.Hour))
	testutil.SeedRaters(r.t, r.pool, person, rated, []int{1, 1, 1, 1, 1})
}

// TestFollowBellWritesNote: with the bell on, one note per report the person
// posts after the bell was turned on, for a Provisional and a Live report, and
// none for a report posted before it.
func TestFollowBellWritesNote(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.writer")
	reader := r.follower("reader")

	before := r.report(session, SeverityLow, r.base.Add(5*time.Minute), "clear")
	r.bellOn(reader, person, r.base.Add(10*time.Minute))
	provisional := r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	live := r.report(session, SeverityCritical, r.base.Add(21*time.Minute), "clear")

	r.refresh(reader, r.base.Add(30*time.Minute))

	if got := r.notes(reader, provisional); got != 1 {
		t.Errorf("notes for the Provisional report = %d, want 1", got)
	}
	if got := r.notes(reader, live); got != 1 {
		t.Errorf("notes for the Live report = %d, want 1", got)
	}
	if got := r.notes(reader, before); got != 0 {
		t.Errorf("notes for a report posted before the bell went on = %d, want 0", got)
	}

	// The two visibilities really differ, so both are covered.
	views, err := r.reports.FollowedLiveReports(context.Background(), FollowedLiveQuery{
		ViewerAccountID: reader,
		Since:           map[int64]time.Time{person: r.base.Add(10 * time.Minute)},
		Now:             r.cur,
	})
	if err != nil {
		t.Fatalf("FollowedLiveReports: %v", err)
	}
	seen := map[int64]Visibility{}
	for _, v := range views {
		seen[v.ID] = v.Visibility
	}
	if seen[provisional] != VisibilityProvisional || seen[live] != VisibilityLive {
		t.Errorf("visibilities = %v, want the low report Provisional and the critical one Live", seen)
	}
}

// TestFollowBellListableOnly: no note while a report is Withheld, removed,
// Hidden, Resolved or expired, and a report that becomes listable later gets its
// note on the next refresh.
func TestFollowBellListableOnly(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.listable")
	reader := r.follower("reader")
	r.bellOn(reader, person, r.base.Add(10*time.Minute))
	at := r.base.Add(20 * time.Minute)

	pending := r.report(session, SeverityLow, at, "pending")
	removed := r.report(session, SeverityLow, at, "removed")
	expired := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
		SessionID: session, Category: "flood", Severity: "low", Description: "p9-22 short lived",
		Latitude: r.lat, Longitude: r.lon, CreatedAt: at, ExpiresAt: at.Add(20 * time.Minute), ModerationState: "clear",
	})
	hidden := r.report(session, SeverityLow, at, "clear")
	for i, cell := range []string{"tdr1w01", "tdr1w02"} {
		voter := testutil.SeedReporterAccount(t, r.pool, fmt.Sprintf("p9-22-disputer-%d@example.com", i), fmt.Sprintf("p9-22-disputer-sess-%d", i))
		r.vote(hidden, voter, "content", "dispute", cell, at.Add(time.Minute))
	}
	resolved := r.report(session, SeverityLow, at, "clear")
	r.vote(resolved, person, "resolution", "resolve", "tdr1v8y", at.Add(2*time.Minute))

	// The fixture is what the names say: the read returns the Hidden report as a
	// live view (it is the one predicate that keeps it out of the bell) and
	// returns none of the Withheld, removed, Resolved or expired ones.
	views, err := r.reports.FollowedLiveReports(context.Background(), FollowedLiveQuery{
		ViewerAccountID: reader,
		Since:           map[int64]time.Time{person: r.base.Add(10 * time.Minute)},
		Now:             at.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("FollowedLiveReports: %v", err)
	}
	if len(views) != 1 || views[0].ID != hidden || views[0].Visibility != VisibilityHidden {
		t.Fatalf("live views = %+v, want only the Hidden report %d", views, hidden)
	}

	r.refresh(reader, at.Add(30*time.Minute))
	for name, id := range map[string]int64{"pending": pending, "removed": removed, "expired": expired, "hidden": hidden, "resolved": resolved} {
		if got := r.notes(reader, id); got != 0 {
			t.Errorf("notes for the %s report = %d, want 0", name, got)
		}
	}
	if got := r.allNotes(reader); got != 0 {
		t.Errorf("follow notes in all = %d, want 0", got)
	}

	// The text check passes later: the held report is Provisional now and gets its
	// note on the next refresh.
	testutil.MustExec(t, r.pool, `UPDATE reports SET moderation_state = 'clear' WHERE id = $1`, pending)
	r.refresh(reader, at.Add(31*time.Minute))
	if got := r.notes(reader, pending); got != 1 {
		t.Errorf("notes for the report that became listable = %d, want 1", got)
	}
	if got := r.allNotes(reader); got != 1 {
		t.Errorf("follow notes in all after it = %d, want 1", got)
	}
}

// TestFollowBellOnePerReport: refreshing again and again writes one note per
// report, and a reader who also has the Reliable reporter alert on, with the
// reporter inside the distance, holds one first alert of one kind: the follow
// note.
func TestFollowBellOnePerReport(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.once")
	// A reader with the Reliable alert on at 5 km from the report point, so the
	// Reliable step would alert too without the follow note.
	both, _ := r.reader("both", 0, 5, false, true, false)
	plain := r.follower("plain")
	r.bellOn(both, person, r.base.Add(10*time.Minute))
	r.bellOn(plain, person, r.base.Add(10*time.Minute))
	// A reader who only has the Reliable alert on, as the control that the
	// fixture alerts.
	reliableOnly, _ := r.reader("reliable-only", 0, 5, false, true, false)

	report := r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	for i := 0; i < 4; i++ {
		r.refresh(both, r.base.Add(30*time.Minute+time.Duration(i)*time.Minute))
		r.refresh(plain, r.base.Add(30*time.Minute+time.Duration(i)*time.Minute))
		r.refresh(reliableOnly, r.base.Add(30*time.Minute+time.Duration(i)*time.Minute))
	}

	if got := r.notes(plain, report); got != 1 {
		t.Errorf("notes after four refreshes = %d, want 1", got)
	}
	if got := r.notes(both, report); got != 1 {
		t.Errorf("follow notes for the reader with both on = %d, want 1", got)
	}
	if got := r.firstAlerts(both, report); got != 1 {
		t.Errorf("first alerts for the reader with both on = %d, want 1 (follow wins the slot)", got)
	}
	if got := r.reliableNotes(reliableOnly, report); got != 1 {
		t.Fatalf("Reliable notes for the control reader = %d, want 1 (the fixture must alert)", got)
	}
}

// TestFollowBellSkipsMutedAndNotReliable: no note for a muted person, a person
// whose rated tag fell below Reliable reporter, a person who went private, a
// follow that ended, or a person who is no longer opted in; the one person with
// every check passing still gets a note.
func TestFollowBellSkipsMutedAndNotReliable(t *testing.T) {
	r := newFollowRig(t)
	reader := r.follower("reader")
	at := r.base.Add(20 * time.Minute)

	good, goodSession := r.person("bell.good")
	muted, mutedSession := r.person("bell.muted")
	lowered, loweredSession := r.person("bell.lowered")
	private, privateSession := r.person("bell.private")
	ended, endedSession := r.person("bell.ended")
	for _, p := range []int64{good, muted, lowered, private, ended} {
		r.bellOn(reader, p, r.base.Add(10*time.Minute))
	}
	// The mute arrives without the service, so the bell row is still on: the
	// step must read the mute itself.
	testutil.SeedMute(t, r.pool, reader, muted)
	r.ratedDown(lowered, loweredSession)
	testutil.MustExec(t, r.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, private)
	if _, err := endFollowPair(context.Background(), r.q, reader, ended, FollowEndUnfollowed, r.base.Add(15*time.Minute)); err != nil {
		t.Fatalf("ending the follow: %v", err)
	}

	reportOf := map[string]int64{
		"good":    r.report(goodSession, SeverityLow, at, "clear"),
		"muted":   r.report(mutedSession, SeverityLow, at, "clear"),
		"lowered": r.report(loweredSession, SeverityLow, at, "clear"),
		"private": r.report(privateSession, SeverityLow, at, "clear"),
		"ended":   r.report(endedSession, SeverityLow, at, "clear"),
	}
	r.refresh(reader, at.Add(10*time.Minute))

	if got := r.notes(reader, reportOf["good"]); got != 1 {
		t.Fatalf("notes about the person who passes every check = %d, want 1 (the fixture must alert)", got)
	}
	for _, name := range []string{"muted", "lowered", "private", "ended"} {
		if got := r.notes(reader, reportOf[name]); got != 0 {
			t.Errorf("notes about the %s person = %d, want 0", name, got)
		}
	}
	if got := r.allNotes(reader); got != 1 {
		t.Errorf("follow notes in all = %d, want 1", got)
	}
}

// TestFollowBellOnlyAfterNotifyOn: a report posted before the bell went on gets
// no note, one posted while it is on does, and a report posted while it was off
// stays without a note after the bell goes on again, because switching the bell
// off forgets the time.
func TestFollowBellOnlyAfterNotifyOn(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.since")
	reader := r.follower("reader")

	early := r.report(session, SeverityLow, r.base.Add(5*time.Minute), "clear")
	r.bellOn(reader, person, r.base.Add(10*time.Minute))
	during := r.report(session, SeverityLow, r.base.Add(15*time.Minute), "clear")
	r.refresh(reader, r.base.Add(20*time.Minute))

	r.setBell(reader, person, false, r.base.Add(25*time.Minute))
	whileOff := r.report(session, SeverityLow, r.base.Add(30*time.Minute), "clear")
	r.refresh(reader, r.base.Add(35*time.Minute))
	r.setBell(reader, person, true, r.base.Add(40*time.Minute))
	r.refresh(reader, r.base.Add(45*time.Minute))
	after := r.report(session, SeverityLow, r.base.Add(50*time.Minute), "clear")
	r.refresh(reader, r.base.Add(55*time.Minute))

	for name, want := range map[string]struct {
		id   int64
		want int
	}{
		"early":    {early, 0},
		"during":   {during, 1},
		"whileOff": {whileOff, 0},
		"after":    {after, 1},
	} {
		if got := r.notes(reader, want.id); got != want.want {
			t.Errorf("notes for the %s report = %d, want %d", name, got, want.want)
		}
	}
}

// TestFollowBellBounded: the step reads at most FollowedPostsCap reports, and
// the follow cap still stops the notes after FollowHourlyCeiling.
func TestFollowBellBounded(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.bounded")
	reader := r.follower("reader")
	r.bellOn(reader, person, r.base.Add(10*time.Minute))

	const total = FollowedPostsCap + 50
	first := r.base.Add(20 * time.Minute)
	ids := make([]int64, 0, total)
	for i := 0; i < total; i++ {
		ids = append(ids, r.report(session, SeverityLow, first.Add(time.Duration(i)*time.Second), "clear"))
	}
	views, err := r.reports.FollowedLiveReports(context.Background(), FollowedLiveQuery{
		ViewerAccountID: reader,
		Since:           map[int64]time.Time{person: r.base.Add(10 * time.Minute)},
		Now:             first.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("FollowedLiveReports: %v", err)
	}
	if len(views) != FollowedPostsCap {
		t.Fatalf("views read = %d, want FollowedPostsCap = %d", len(views), FollowedPostsCap)
	}
	// The newest ones are kept, oldest first.
	if views[0].ID != ids[total-FollowedPostsCap] || views[len(views)-1].ID != ids[total-1] {
		t.Errorf("kept ids %d to %d, want the newest %d from %d to %d", views[0].ID, views[len(views)-1].ID, FollowedPostsCap, ids[total-FollowedPostsCap], ids[total-1])
	}

	r.refresh(reader, first.Add(time.Hour))
	if got := r.allNotes(reader); got != FollowHourlyCeiling {
		t.Errorf("notes after one refresh = %d, want the follow cap %d", got, FollowHourlyCeiling)
	}
}

// followFake is the fake of the step tests: the follow reads and the follow
// insert are canned and recorded, on top of the Reliable alert fake.
type followFake struct {
	*alertFake

	follows   []sqlcgen.NotifyFollowsForRow
	limits    []int32
	muted     []int64
	notFollow map[int64]bool
	// noteErr is the answer of InsertFollowedPostedAlert by report id; a report
	// not in it is written.
	noteErr map[int64]error
	noted   []int64
}

func (f *followFake) NotifyFollowsFor(_ context.Context, a sqlcgen.NotifyFollowsForParams) ([]sqlcgen.NotifyFollowsForRow, error) {
	f.limits = append(f.limits, a.RowLimit)
	rows := f.follows
	if int(a.RowLimit) < len(rows) {
		rows = rows[:a.RowLimit]
	}
	return rows, nil
}

func (f *followFake) MutedPeopleIDs(_ context.Context, _ int64) ([]int64, error) {
	return f.muted, nil
}

func (f *followFake) FolloweeStanding(_ context.Context, ids []int64) ([]sqlcgen.FolloweeStandingRow, error) {
	var rows []sqlcgen.FolloweeStandingRow
	for _, id := range ids {
		rows = append(rows, sqlcgen.FolloweeStandingRow{ID: id, Followable: !f.notFollow[id]})
	}
	return rows, nil
}

func (f *followFake) InsertFollowedPostedAlert(_ context.Context, a sqlcgen.InsertFollowedPostedAlertParams) (int64, error) {
	if err := f.noteErr[a.ReportID]; err != nil {
		return 0, err
	}
	f.noted = append(f.noted, a.ReportID)
	return int64(len(f.noted)), nil
}

// fakeFollowReader answers both reads of the reports side.
type fakeFollowReader struct {
	fakeNearby
	followed []ReportView
	queries2 []FollowedLiveQuery
}

func (n *fakeFollowReader) FollowedLiveReports(_ context.Context, q FollowedLiveQuery) ([]ReportView, error) {
	n.queries2 = append(n.queries2, q)
	return n.followed, nil
}

// newFollowService builds the service over the follow fake with the clock pinned.
func newFollowService(f *followFake, n NearbyReader) *UpdatesService {
	return NewUpdatesService(f, n, WithUpdatesClock(func() time.Time { return updatesNow }))
}

// followedView is a live report by the given reporter account.
func followedView(id, reporter int64, tag ReporterTag) ReportView {
	v := alertView(id, VisibilityLive, tag, time.Hour)
	v.reporterAccountID = reporter
	return v
}

func newFollowFake(follows ...int64) *followFake {
	f := &followFake{
		alertFake: &alertFake{fakeUpdatesQuerier: &fakeUpdatesQuerier{}, settings: alertOn(5), homeCell: cellPtr("tdr1v")},
		noteErr:   map[int64]error{},
	}
	for _, id := range follows {
		f.follows = append(f.follows, sqlcgen.NotifyFollowsForRow{FolloweeID: id, NotifySince: updatesNow.Add(-2 * time.Hour)})
	}
	return f
}

// TestFollowBellFollowWinsEvenWhenTheCeilingStopsIt: the Reliable step skips
// every report by a person the reader follows with Notify me on, written or not.
// Amended 2026-11-03 by quick 261103-jal (review WR-02, which replaces W3): a
// report whose follow note was not written (the hourly ceiling answer,
// pgx.ErrNoRows) is no longer offered to the Reliable step, because the Reliable
// row would take the one first alert slot and the follow phone note, queued only
// from a written follow row, would be lost. A Reliable report by someone the
// reader does not follow is still written.
func TestFollowBellFollowWinsEvenWhenTheCeilingStopsIt(t *testing.T) {
	f := newFollowFake(7)
	f.noteErr[100] = pgx.ErrNoRows
	f.reporters = reporterRows(100, 7, 101, 7, 102, 9)
	n := &fakeFollowReader{}
	n.views = []ReportView{followedView(100, 7, ReporterReliable), followedView(101, 7, ReporterReliable), followedView(102, 9, ReporterReliable)}
	n.followed = n.views[:2]
	svc := newFollowService(f, n)

	if err := svc.Refresh(context.Background(), 5); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !sameIDs(f.noted, []int64{101}) {
		t.Errorf("follow notes written for %v, want [101]", f.noted)
	}
	if got := insertedReportIDs(f.alertFake); !sameIDs(got, []int64{102}) {
		t.Errorf("Reliable alerts attempted for %v, want [102] only: 100 and 101 are by a person followed with Notify me on", got)
	}
}

// TestFollowBellSkipsOwnAndLowerTagsAndMuted is the step with a fake: the
// reader's own report, a person who lost the Reliable tag, a muted person and a
// person who is not followable get no note.
func TestFollowBellSkipsOwnAndLowerTagsAndMuted(t *testing.T) {
	f := newFollowFake(7, 8, 9, 10)
	f.muted = []int64{9}
	f.notFollow = map[int64]bool{10: true}
	f.settings = nil // no Reliable alert, so only the follow step writes
	n := &fakeFollowReader{}
	own := followedView(100, 7, ReporterReliable)
	own.IsOwnReport = true
	n.followed = []ReportView{
		own,
		followedView(101, 8, ReporterMixed),
		followedView(102, 9, ReporterReliable),
		followedView(103, 10, ReporterReliable),
		followedView(104, 7, ReporterReliable),
	}
	svc := newFollowService(f, n)

	if err := svc.Refresh(context.Background(), 5); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	// Muted and not followable people never reach the report read, and the
	// remaining checks leave one note.
	if len(n.queries2) != 1 {
		t.Fatalf("follow reads = %d, want 1", len(n.queries2))
	}
	since := n.queries2[0].Since
	if _, ok := since[9]; ok {
		t.Error("the muted person was read")
	}
	if _, ok := since[10]; ok {
		t.Error("the person who is not followable was read")
	}
	if len(since) != 2 || n.queries2[0].ViewerAccountID != 5 {
		t.Errorf("read for %v as viewer %d, want people 7 and 8 as viewer 5", since, n.queries2[0].ViewerAccountID)
	}
	if !sameIDs(f.noted, []int64{104}) {
		t.Errorf("notes written for %v, want only [104]", f.noted)
	}
}

// TestFollowBellFakeBounds: the follows read asks for at most NotifyFollowsCap
// people, so a reader with sixty bells on reads fifty.
func TestFollowBellFakeBounds(t *testing.T) {
	ids := make([]int64, 0, NotifyFollowsCap+10)
	for i := 0; i < NotifyFollowsCap+10; i++ {
		ids = append(ids, int64(1000+i))
	}
	f := newFollowFake(ids...)
	f.settings = nil
	n := &fakeFollowReader{}
	svc := newFollowService(f, n)
	if err := svc.Refresh(context.Background(), 5); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(f.limits) != 1 || f.limits[0] != NotifyFollowsCap {
		t.Errorf("follows read limits = %v, want [%d]", f.limits, NotifyFollowsCap)
	}
	if len(n.queries2) != 1 || len(n.queries2[0].Since) != NotifyFollowsCap {
		t.Fatalf("people read = %d, want %d", len(n.queries2[0].Since), NotifyFollowsCap)
	}
}

// TestFollowBellNoReaderEndsQuietly: a service whose reports reader cannot read
// followed reports, or that has none, ends the step with no error and no query.
func TestFollowBellNoReaderEndsQuietly(t *testing.T) {
	f := newFollowFake(7)
	f.settings = nil
	svc := newFollowService(f, &fakeNearby{})
	if err := svc.Refresh(context.Background(), 5); err != nil {
		t.Fatalf("Refresh with a reader that cannot read followed reports: %v", err)
	}
	if len(f.limits) != 0 {
		t.Errorf("follows read %d times, want 0", len(f.limits))
	}
	nilSvc := newFollowService(f, nil)
	if err := nilSvc.Refresh(context.Background(), 5); err != nil {
		t.Fatalf("Refresh with no reports reader: %v", err)
	}
}

// TestFollowBellErrorSkipsReliableStep: a failure in the follow step is the error
// Refresh returns, and the Reliable step does not run in that refresh. Amended
// 2026-11-03 by quick 261103-jal (review WR-02): it used to run, which let a
// Reliable row take the first alert slot of a report the follow step had not
// finished with.
func TestFollowBellErrorSkipsReliableStep(t *testing.T) {
	f := newFollowFake(7)
	f.reporters = reporterRows(100, 8)
	f.noteErr[200] = errors.New("boom")
	n := &fakeFollowReader{}
	n.views = []ReportView{followedView(100, 8, ReporterReliable)}
	n.followed = []ReportView{followedView(200, 7, ReporterReliable)}
	svc := newFollowService(f, n)

	err := svc.Refresh(context.Background(), 5)
	if err == nil || !strings.Contains(err.Error(), "follow notes") {
		t.Fatalf("Refresh error = %v, want the follow notes error", err)
	}
	if got := insertedReportIDs(f.alertFake); len(got) != 0 {
		t.Errorf("Reliable alerts attempted for %v, want none after the follow step failed", got)
	}
}

// TestFollowBellTruncatedSkipsReliableStep: when the follow read hit
// FollowedPostsCap it may have missed a report, so the Reliable step waits for
// the next refresh (review WR-02). One report fewer and it runs.
func TestFollowBellTruncatedSkipsReliableStep(t *testing.T) {
	for _, c := range []struct {
		name      string
		followed  int
		wantTried []int64
	}{
		{"at the cap", FollowedPostsCap, nil},
		{"one under the cap", FollowedPostsCap - 1, []int64{100}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFollowFake(7)
			f.reporters = reporterRows(100, 8)
			n := &fakeFollowReader{}
			n.views = []ReportView{followedView(100, 8, ReporterReliable)}
			for i := 0; i < c.followed; i++ {
				n.followed = append(n.followed, followedView(int64(1000+i), 7, ReporterMixed))
			}
			svc := newFollowService(f, n)

			if err := svc.Refresh(context.Background(), 5); err != nil {
				t.Fatalf("Refresh: %v", err)
			}
			if got := insertedReportIDs(f.alertFake); !sameIDs(got, c.wantTried) {
				t.Errorf("Reliable alerts attempted for %v, want %v", got, c.wantTried)
			}
		})
	}
}

// listItem reads the reader's bell list and returns the row of one update id.
func (r *followRig) listItem(reader, updateID int64) UpdateItem {
	r.t.Helper()
	page, err := r.updates.List(context.Background(), reader, ActivityUpdatesListSize)
	if err != nil {
		r.t.Fatalf("List: %v", err)
	}
	for _, it := range page.Items {
		if it.ID == updateID {
			return it
		}
	}
	r.t.Fatalf("update %d is not in the list of reader %d", updateID, reader)
	return UpdateItem{}
}

// noteID is the id of the follow note a reader holds about a report.
func (r *followRig) noteID(reader, report int64) int64 {
	r.t.Helper()
	var id int64
	err := r.pool.QueryRow(context.Background(),
		`SELECT id FROM updates WHERE account_id = $1 AND report_id = $2 AND kind = 'followed_posted'`, reader, report).Scan(&id)
	if err != nil {
		r.t.Fatalf("reading the follow note of report %d: %v", report, err)
	}
	return id
}

// wantGone checks the shape of a row that reads as gone: the unavailable line
// and nothing that names a person or leads to a report.
func wantGone(t *testing.T, what string, it UpdateItem) {
	t.Helper()
	if it.Available || it.ReportID != nil || it.Event != UnavailableLine || it.ReportLine != "" || it.Trust != "" || it.Category != "" || it.Severity != "" {
		t.Errorf("%s: row = %+v, want the unavailable line with no username, no report id and no detail", what, it)
	}
	if it.Kind != UpdateFollowedPosted {
		t.Errorf("%s: kind = %q, want followed_posted (the row stays as history)", what, it.Kind)
	}
}

// TestFollowBellRowWords: a follow note for a Provisional report reads the
// username, the category and severity words and Not confirmed yet.; once people
// nearby confirm the report the same row reads the confirmed sentence, with no
// new row written.
func TestFollowBellRowWords(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.words")
	reader := r.follower("reader")
	r.bellOn(reader, person, r.base.Add(10*time.Minute))
	report := r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")
	critical := r.report(session, SeverityCritical, r.base.Add(21*time.Minute), "clear")
	r.refresh(reader, r.base.Add(30*time.Minute))

	it := r.listItem(reader, r.noteID(reader, report))
	if !it.Available || it.ReportID == nil || *it.ReportID != report {
		t.Fatalf("row = %+v, want an available row linking report %d", it, report)
	}
	if it.Event != "bell.words posted a report." {
		t.Errorf("line 1 = %q", it.Event)
	}
	if it.ReportLine != "Flood, Low." {
		t.Errorf("line 2 = %q, want the category and severity words, never the report's text", it.ReportLine)
	}
	if it.Trust != "Not confirmed yet." {
		t.Errorf("trust line = %q, want Not confirmed yet.", it.Trust)
	}
	if got := r.listItem(reader, r.noteID(reader, critical)).ReportLine; got != "Flood, Critical." {
		t.Errorf("line 2 of the critical report = %q", got)
	}

	// Two people at different spots confirm it: the same row reads the confirmed
	// sentence and no row is added.
	for i, cell := range []string{"tdr1v8y", "tdr1vc3"} {
		voter := testutil.SeedReporterAccount(t, r.pool, fmt.Sprintf("p9-22-confirmer-%d@example.com", i), fmt.Sprintf("p9-22-confirmer-sess-%d", i))
		r.vote(report, voter, "content", "confirm", cell, r.base.Add(31*time.Minute))
	}
	r.refresh(reader, r.base.Add(32*time.Minute))
	again := r.listItem(reader, it.ID)
	if !strings.HasPrefix(again.Trust, "Confirmed by ") {
		t.Errorf("trust line after the confirmations = %q, want the confirmed sentence", again.Trust)
	}
	if got := r.notes(reader, report); got != 1 {
		t.Errorf("notes for the confirmed report = %d, want 1 (updated in place, no new row)", got)
	}
	if again.Event != it.Event || again.ReportLine != it.ReportLine {
		t.Errorf("lines 1 and 2 changed to %q and %q", again.Event, again.ReportLine)
	}
}

// TestFollowBellEndedFollowReadsUnavailable: after the follow ends, the person
// goes private or is deleted, or the person blocks the reader, the row reads as
// gone with no username and no report link. A bell switched off with the follow
// kept leaves the row readable.
func TestFollowBellEndedFollowReadsUnavailable(t *testing.T) {
	r := newFollowRig(t)
	reader := r.follower("reader")
	type fixture struct {
		person int64
		report int64
		note   int64
	}
	fix := map[string]*fixture{}
	for _, name := range []string{"unfollowed", "private", "deleted", "blocker", "bellOff", "control"} {
		person, session := r.person("bell." + strings.ToLower(name))
		r.bellOn(reader, person, r.base.Add(10*time.Minute))
		fix[name] = &fixture{person: person, report: r.report(session, SeverityLow, r.base.Add(20*time.Minute), "clear")}
	}
	r.refresh(reader, r.base.Add(30*time.Minute))
	for name, f := range fix {
		f.note = r.noteID(reader, f.report)
		if it := r.listItem(reader, f.note); !it.Available || !strings.HasSuffix(it.Event, " posted a report.") {
			t.Fatalf("%s: row before the change = %+v, want a readable row", name, it)
		}
	}

	r.cur = r.base.Add(40 * time.Minute)
	if _, err := endFollowPair(context.Background(), r.q, reader, fix["unfollowed"].person, FollowEndUnfollowed, r.cur); err != nil {
		t.Fatalf("ending the follow: %v", err)
	}
	testutil.MustExec(t, r.pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, fix["private"].person)
	testutil.MustExec(t, r.pool, `UPDATE accounts SET deleted_at = $2 WHERE id = $1`, fix["deleted"].person, r.cur)
	testutil.SeedBlock(t, r.pool, fix["blocker"].person, reader)
	r.setBell(reader, fix["bellOff"].person, false, r.cur)

	for _, name := range []string{"unfollowed", "private", "deleted", "blocker"} {
		wantGone(t, name, r.listItem(reader, fix[name].note))
	}
	for _, name := range []string{"bellOff", "control"} {
		if it := r.listItem(reader, fix[name].note); !it.Available || it.ReportID == nil {
			t.Errorf("%s: row = %+v, want it still readable (the follow lasts)", name, it)
		}
	}
}

// TestFollowBellFinishedReportDropsName: once a report is resolved or has
// expired, its follow note reads as gone with no username and no report id,
// while the follow lasts and the person stays public (D-20).
func TestFollowBellFinishedReportDropsName(t *testing.T) {
	r := newFollowRig(t)
	person, session := r.person("bell.finished")
	reader := r.follower("reader")
	r.bellOn(reader, person, r.base.Add(10*time.Minute))
	at := r.base.Add(20 * time.Minute)
	toResolve := r.report(session, SeverityLow, at, "clear")
	toExpire := testutil.InsertStoredReport(t, r.pool, testutil.StoredReport{
		SessionID: session, Category: "flood", Severity: "low", Description: "p9-22 short lived",
		Latitude: r.lat, Longitude: r.lon, CreatedAt: at, ExpiresAt: at.Add(25 * time.Minute), ModerationState: "clear",
	})
	r.refresh(reader, at.Add(10*time.Minute))
	resolveNote, expireNote := r.noteID(reader, toResolve), r.noteID(reader, toExpire)
	for _, id := range []int64{resolveNote, expireNote} {
		if it := r.listItem(reader, id); !it.Available || it.ReportID == nil {
			t.Fatalf("row %d before the report finished = %+v, want a readable row", id, it)
		}
	}

	// The person marks one resolved; the other runs out of time.
	r.vote(toResolve, person, "resolution", "resolve", "tdr1v8y", at.Add(12*time.Minute))
	r.cur = at.Add(40 * time.Minute)
	wantGone(t, "resolved", r.listItem(reader, resolveNote))
	wantGone(t, "expired", r.listItem(reader, expireNote))

	// The follow and the public profile are still there, so this is the report
	// finishing and nothing else.
	var active, public bool
	if err := r.pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM account_follows WHERE follower_id = $1 AND followee_id = $2 AND ended_at IS NULL),
		        (SELECT profile_public FROM accounts WHERE id = $2)`, reader, person).Scan(&active, &public); err != nil {
		t.Fatalf("reading the follow and the profile: %v", err)
	}
	if !active || !public {
		t.Fatalf("follow active = %v, profile public = %v, want both still true", active, public)
	}
}

// TestFollowBellListReadsOnlyForFollowNotes: a page with no follow note makes
// neither of the two extra reads.
func TestFollowBellListReadsOnlyForFollowNotes(t *testing.T) {
	f := newFollowFake()
	svc := newFollowService(f, nil)
	if _, err := svc.List(context.Background(), 5, BellListSize); err != nil {
		t.Fatalf("List: %v", err)
	}
	if n := len(f.named("PublicProfilePathsForViewer")) + len(f.named("ActiveFollowsAmong")); n != 0 {
		t.Errorf("extra reads on a page with no follow note = %d, want 0", n)
	}
}
