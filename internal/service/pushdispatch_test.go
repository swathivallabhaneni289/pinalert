// pushdispatch_test.go proves the phone alert dispatcher with fakes (plan
// 03.3-09, task 2): which accounts are queued for which kind, the caps, the one
// fan out per report, and that nothing is decided from the poke or sent here.
// The same rules run on a real database in pushdispatch_db_test.go.

package service

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

const dispLat, dispLon = 12.9716, 77.5946

// dispatchFake is a PushDispatchQuerier that records every call in order and
// keeps the two rules of the real queries a test depends on: a repeat initial
// alert answers pgx.ErrNoRows, and a fan out mark only lands while the stored
// state is NULL.
type dispatchFake struct {
	mu    sync.Mutex
	calls []string

	facts   map[int64]sqlcgen.PushReportFactsRow
	votes   []sqlcgen.CurrentVotesForReportsRow
	history []sqlcgen.ReporterHistoryReportsRow

	progress map[int64]*sqlcgen.PushReportProgress

	counts     map[string]int64 // answer of CountReporterFanouts by kind
	countCalls []sqlcgen.CountReporterFanoutsParams

	candidates     []sqlcgen.PushHomeCandidatesRow
	candidateCalls []sqlcgen.PushHomeCandidatesParams

	queued       map[[2]int64]bool
	initialCalls []sqlcgen.EnqueueInitialPushParams
	enqueueErr   error

	reliableCalls  []sqlcgen.EnqueueReliableFromUpdatesParams
	changedCalls   []sqlcgen.EnqueueChangedFromUpdatesParams
	confirmedCalls []sqlcgen.EnqueueConfirmedUpdatesParams

	criticalMarks []sqlcgen.MarkCriticalFanoutParams
	areaSets      []sqlcgen.SetReportAreaNameParams
	areaTried     int

	followers     []int64
	bellFollowers []int64 // answer of NotifyFollowersOf (Phase 9 plan 25)
	followerCalls []int64
	recent        []int64
	recentCalls   []sqlcgen.RecentReportsForPushParams
	settings      map[int64]sqlcgen.GetPushKindSettingsRow
}

func newDispatchFake() *dispatchFake {
	return &dispatchFake{
		facts:    map[int64]sqlcgen.PushReportFactsRow{},
		progress: map[int64]*sqlcgen.PushReportProgress{},
		counts:   map[string]int64{},
		queued:   map[[2]int64]bool{},
		settings: map[int64]sqlcgen.GetPushKindSettingsRow{},
	}
}

// dispatchReport is the default fixture: a Critical report 10 minutes old,
// posted by account 900, expiring in 5 hours.
func dispatchReport(severity, moderation string) sqlcgen.PushReportFactsRow {
	row := pushFactsRow(severity, "flood", moderation)
	row.Latitude, row.Longitude = dispLat, dispLon
	return row
}

func (f *dispatchFake) rec(name string) { f.calls = append(f.calls, name) }

func (f *dispatchFake) names(name string) int {
	n := 0
	for _, c := range f.calls {
		if c == name {
			n++
		}
	}
	return n
}

func (f *dispatchFake) indexOf(name string) int {
	for i, c := range f.calls {
		if c == name {
			return i
		}
	}
	return -1
}

func (f *dispatchFake) PushReportFacts(_ context.Context, id int64) (sqlcgen.PushReportFactsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("PushReportFacts")
	row, ok := f.facts[id]
	if !ok {
		return sqlcgen.PushReportFactsRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *dispatchFake) CurrentVotesForReports(_ context.Context, _ []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("CurrentVotesForReports")
	return f.votes, nil
}

func (f *dispatchFake) ReporterHistoryReports(_ context.Context, _ sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("ReporterHistoryReports")
	return f.history, nil
}

func (f *dispatchFake) GetPushReportProgress(_ context.Context, id int64) (sqlcgen.PushReportProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("GetPushReportProgress")
	p, ok := f.progress[id]
	if !ok {
		return sqlcgen.PushReportProgress{}, pgx.ErrNoRows
	}
	return *p, nil
}

func (f *dispatchFake) prog(id int64) *sqlcgen.PushReportProgress {
	p, ok := f.progress[id]
	if !ok {
		p = &sqlcgen.PushReportProgress{ReportID: id}
		f.progress[id] = p
	}
	return p
}

func (f *dispatchFake) MarkReportEvaluated(_ context.Context, a sqlcgen.MarkReportEvaluatedParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("MarkReportEvaluated")
	f.prog(a.ReportID).EvaluatedAt = pgtype.Timestamptz{Time: a.Now, Valid: true}
	return nil
}

func (f *dispatchFake) MarkCriticalFanout(_ context.Context, a sqlcgen.MarkCriticalFanoutParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("MarkCriticalFanout")
	f.criticalMarks = append(f.criticalMarks, a)
	p := f.prog(a.ReportID)
	if p.CriticalState == nil {
		s := a.State
		p.CriticalState = &s
		p.CriticalAt = pgtype.Timestamptz{Time: a.Now, Valid: true}
	}
	return nil
}

// MarkFollowFanout, EnqueueFollowFromUpdates and NotifyFollowersOf were added by
// Phase 9 plan 25 (D-09) when PushDispatchQuerier gained the follow pass. They
// record their calls like the others and change no existing behavior: the mark
// is sticky like the other two, the enqueue queues nothing, and no follower has
// the bell on unless a test sets bellFollowers.
func (f *dispatchFake) MarkFollowFanout(_ context.Context, a sqlcgen.MarkFollowFanoutParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("MarkFollowFanout")
	p := f.prog(a.ReportID)
	if p.FollowState == nil {
		s := a.State
		p.FollowState = &s
		p.FollowAt = pgtype.Timestamptz{Time: a.Now, Valid: true}
	}
	return nil
}

func (f *dispatchFake) EnqueueFollowFromUpdates(_ context.Context, _ sqlcgen.EnqueueFollowFromUpdatesParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("EnqueueFollowFromUpdates")
	return 0, nil
}

func (f *dispatchFake) NotifyFollowersOf(_ context.Context, _ sqlcgen.NotifyFollowersOfParams) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("NotifyFollowersOf")
	return f.bellFollowers, nil
}

// MarkAreaLookupTried mirrors the conditional claim of quick 261119-sps: 1 only
// when the lookup was never tried, 0 after that. areaTried counts the calls.
func (f *dispatchFake) MarkAreaLookupTried(_ context.Context, a sqlcgen.MarkAreaLookupTriedParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("MarkAreaLookupTried")
	f.areaTried++
	p := f.prog(a.ReportID)
	if p.AreaLookupAt.Valid {
		return 0, nil
	}
	p.AreaLookupAt = pgtype.Timestamptz{Time: a.Now, Valid: true}
	return 1, nil
}

func (f *dispatchFake) SetReportAreaName(_ context.Context, a sqlcgen.SetReportAreaNameParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("SetReportAreaName")
	f.areaSets = append(f.areaSets, a)
	if row, ok := f.facts[a.ReportID]; ok {
		name := a.AreaName
		row.AreaName = &name
		f.facts[a.ReportID] = row
	}
	return nil
}

func (f *dispatchFake) CountReporterFanouts(_ context.Context, a sqlcgen.CountReporterFanoutsParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("CountReporterFanouts")
	f.countCalls = append(f.countCalls, a)
	return f.counts[a.Kind], nil
}

func (f *dispatchFake) PushHomeCandidates(_ context.Context, a sqlcgen.PushHomeCandidatesParams) ([]sqlcgen.PushHomeCandidatesRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("PushHomeCandidates")
	f.candidateCalls = append(f.candidateCalls, a)
	return f.candidates, nil
}

func (f *dispatchFake) EnqueueInitialPush(_ context.Context, a sqlcgen.EnqueueInitialPushParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("EnqueueInitialPush")
	f.initialCalls = append(f.initialCalls, a)
	if f.enqueueErr != nil {
		return 0, f.enqueueErr
	}
	key := [2]int64{a.AccountID, a.ReportID}
	if f.queued[key] {
		return 0, pgx.ErrNoRows
	}
	f.queued[key] = true
	return int64(len(f.queued)), nil
}

func (f *dispatchFake) EnqueueReliableFromUpdates(_ context.Context, a sqlcgen.EnqueueReliableFromUpdatesParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("EnqueueReliableFromUpdates")
	f.reliableCalls = append(f.reliableCalls, a)
	return 0, nil
}

func (f *dispatchFake) EnqueueChangedFromUpdates(_ context.Context, a sqlcgen.EnqueueChangedFromUpdatesParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("EnqueueChangedFromUpdates")
	f.changedCalls = append(f.changedCalls, a)
	return 0, nil
}

func (f *dispatchFake) EnqueueConfirmedUpdates(_ context.Context, a sqlcgen.EnqueueConfirmedUpdatesParams) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("EnqueueConfirmedUpdates")
	f.confirmedCalls = append(f.confirmedCalls, a)
	return 0, nil
}

func (f *dispatchFake) FollowersForBellRefresh(_ context.Context, reportID int64) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("FollowersForBellRefresh")
	f.followerCalls = append(f.followerCalls, reportID)
	return f.followers, nil
}

func (f *dispatchFake) RecentReportsForPush(_ context.Context, a sqlcgen.RecentReportsForPushParams) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("RecentReportsForPush")
	f.recentCalls = append(f.recentCalls, a)
	return f.recent, nil
}

func (f *dispatchFake) GetPushKindSettings(_ context.Context, id int64) (sqlcgen.GetPushKindSettingsRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rec("GetPushKindSettings")
	s, ok := f.settings[id]
	if !ok {
		return sqlcgen.GetPushKindSettingsRow{CriticalPushOn: true}, nil
	}
	return s, nil
}

// addReliableHistory gives the account six expired reports, five confirmed from
// two distinct cells: the same shape testutil.SeedReliableReporter writes, which
// reads Reliable reporter.
func (f *dispatchFake) addReliableHistory(account int64) {
	expires := pushFactsNow.Add(-time.Hour)
	for i := int64(0); i < 6; i++ {
		id := 5000 + i
		f.history = append(f.history, sqlcgen.ReporterHistoryReportsRow{
			AccountID: account, ReportID: id, Severity: "low", Category: "flood",
			ExpiresAt: expires, ModerationState: "clear",
		})
		if i == 5 {
			continue
		}
		for _, v := range []struct {
			acct int64
			cell string
		}{{7001, "tdr1v8y"}, {7002, "tdr1vc3"}} {
			f.votes = append(f.votes, sqlcgen.CurrentVotesForReportsRow{
				ReportID: id, AccountID: v.acct, Kind: "content", Value: "confirm",
				GeohashCell: v.cell, CreatedAt: expires.Add(-2 * time.Hour),
			})
		}
	}
}

func dispatchStrPtr(s string) *string { return &s }

func (f *dispatchFake) setProgress(id int64, critical, reliable string) {
	p := f.prog(id)
	if critical != "" {
		p.CriticalState = dispatchStrPtr(critical)
		p.CriticalAt = pgtype.Timestamptz{Time: pushFactsNow.Add(-5 * time.Minute), Valid: true}
	}
	if reliable != "" {
		p.ReliableState = dispatchStrPtr(reliable)
		p.ReliableAt = pgtype.Timestamptz{Time: pushFactsNow.Add(-5 * time.Minute), Valid: true}
	}
}

func (f *dispatchFake) initialAccounts() []int64 {
	var out []int64
	for _, c := range f.initialCalls {
		out = append(out, c.AccountID)
	}
	return out
}

// fakeUpdatesRefresher records which accounts the 3.2 rule was asked to refresh.
type fakeUpdatesRefresher struct {
	mu        sync.Mutex
	log       func(string)
	refreshed []int64
	errFor    map[int64]error
}

func (r *fakeUpdatesRefresher) Refresh(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.log != nil {
		r.log("Refresh")
	}
	r.refreshed = append(r.refreshed, id)
	return r.errFor[id]
}

// fakeAreaNamer is the reverse area lookup.
type fakeAreaNamer struct {
	mu       sync.Mutex
	name     string
	err      error
	calls    int
	deadline []time.Duration
}

func (n *fakeAreaNamer) ReverseArea(ctx context.Context, lat, lon float64) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
	if dl, ok := ctx.Deadline(); ok {
		n.deadline = append(n.deadline, time.Until(dl))
	}
	return n.name, n.err
}

type dispatchRig struct {
	f *dispatchFake
	u *fakeUpdatesRefresher
	n *fakeAreaNamer
	d *PushDispatcher
}

func newDispatchRig(opts ...DispatchOption) *dispatchRig {
	f := newDispatchFake()
	u := &fakeUpdatesRefresher{errFor: map[int64]error{}}
	u.log = func(s string) { f.mu.Lock(); f.rec(s); f.mu.Unlock() }
	n := &fakeAreaNamer{name: "Central Market"}
	all := append([]DispatchOption{WithDispatchClock(func() time.Time { return pushFactsNow })}, opts...)
	return &dispatchRig{f: f, u: u, n: n, d: NewPushDispatcher(f, u, n, all...)}
}

// critical42 puts the default Critical report 42 and two in range readers in
// place: accounts 11 and 12 with Critical on.
func (r *dispatchRig) critical42() {
	r.f.facts[42] = dispatchReport("critical", "clear")
	r.f.candidates = []sqlcgen.PushHomeCandidatesRow{
		homeRowAt(11, dispLat, dispLon, 0, 5, true, false),
		homeRowAt(12, dispLat, dispLon, 0, 5, true, false),
	}
}

func (r *dispatchRig) eval(t *testing.T, id, actor int64) {
	t.Helper()
	if err := r.d.EvaluateReport(context.Background(), id, actor); err != nil {
		t.Fatalf("EvaluateReport: %v", err)
	}
}

func dispatchInt64sEqual(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDispatchCriticalThroughResolve(t *testing.T) {
	rig := newDispatchRig()
	rig.critical42()
	// Account 11 in range, 12 in range, 13 too far, 14 is the author, 15 has the
	// stored switch off.
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
	// the stored Critical switch no longer decides, so 15 is queued too.
	rig.f.candidates = []sqlcgen.PushHomeCandidatesRow{
		homeRowAt(11, dispLat, dispLon, 0, 5, true, false),
		homeRowAt(12, dispLat, dispLon, 0, 5, true, false),
		homeRowAt(13, dispLat, dispLon, 3, 5, true, false),
		homeRowAt(900, dispLat, dispLon, 0, 5, true, false),
		homeRowAt(15, dispLat, dispLon, 0, 5, false, false),
	}
	rig.eval(t, 42, 0)

	if got := rig.f.initialAccounts(); !dispatchInt64sEqual(got, []int64{11, 12, 15}) {
		t.Fatalf("queued accounts = %v, want [11 12 15]", got)
	}
	for _, c := range rig.f.initialCalls {
		if c.Kind != string(PushKindCritical) || c.ReportID != 42 {
			t.Errorf("queued %+v, want kind critical for report 42", c)
		}
		if want := pushFactsNow.Add(PushInitialWindow); !c.ExpiresAt.Equal(want) {
			t.Errorf("expires_at = %v, want %v", c.ExpiresAt, want)
		}
		if !c.Now.Equal(pushFactsNow) {
			t.Errorf("now = %v, want the dispatcher clock", c.Now)
		}
	}
	if len(rig.f.criticalMarks) != 1 || rig.f.criticalMarks[0].State != "fanned" {
		t.Errorf("critical marks = %+v, want one fanned", rig.f.criticalMarks)
	}
	if len(rig.f.candidateCalls) != 1 {
		t.Fatalf("candidate reads = %d, want 1", len(rig.f.candidateCalls))
	}
	cc := rig.f.candidateCalls[0]
	if cc.ReportID != 42 || cc.RowCap != PushRecipientsPerReport*2 || len(cc.Cells) == 0 {
		t.Errorf("candidate read = report %d cap %d cells %d", cc.ReportID, cc.RowCap, len(cc.Cells))
	}
	if want := candidateCells(dispLat, dispLon, PushCandidateMaxRadiusKm); cc.Cells[0] != want[0] {
		t.Errorf("first candidate cell = %q, want the report's own cell %q", cc.Cells[0], want[0])
	}
	if rig.f.indexOf("PushReportFacts") != 0 {
		t.Errorf("the first call was %q, want PushReportFacts: every decision starts from fresh facts", rig.f.calls[0])
	}

	t.Run("a report that expires within the window keeps its own expiry", func(t *testing.T) {
		r := newDispatchRig()
		r.critical42()
		row := r.f.facts[42]
		row.ExpiresAt = pushFactsNow.Add(20 * time.Minute)
		r.f.facts[42] = row
		r.eval(t, 42, 0)
		if len(r.f.initialCalls) == 0 || !r.f.initialCalls[0].ExpiresAt.Equal(row.ExpiresAt) {
			t.Errorf("initial calls = %+v, want expires_at at the report's expiry", r.f.initialCalls)
		}
	})

	// Every case below must queue nothing and mark nothing.
	silent := []struct {
		name  string
		setup func(r *dispatchRig)
	}{
		{"removed", func(r *dispatchRig) {
			r.f.facts[42] = dispatchReport("critical", "removed")
		}},
		{"pending and not serious", func(r *dispatchRig) {
			r.f.facts[42] = dispatchReport("low", "pending")
		}},
		{"retracted by its reporter", func(r *dispatchRig) {
			r.f.votes = append(r.f.votes, pushVote(42, 900, "resolution", "resolve", "tdr1v8y"))
		}},
		{"expired", func(r *dispatchRig) {
			row := r.f.facts[42]
			row.ExpiresAt = pushFactsNow.Add(-time.Minute)
			r.f.facts[42] = row
		}},
		{"older than the initial window", func(r *dispatchRig) {
			row := r.f.facts[42]
			row.CreatedAt = pushFactsNow.Add(-PushInitialWindow - time.Minute)
			r.f.facts[42] = row
		}},
		{"already capped for good", func(r *dispatchRig) {
			r.f.setProgress(42, "capped", "")
		}},
		{"not serious and the reporter is not reliable", func(r *dispatchRig) {
			r.f.facts[42] = dispatchReport("low", "clear")
		}},
		{"posted without an account", func(r *dispatchRig) {
			row := r.f.facts[42]
			row.ReporterAccountID = nil
			r.f.facts[42] = row
		}},
		{"does not exist", func(r *dispatchRig) {
			delete(r.f.facts, 42)
		}},
	}
	for _, c := range silent {
		t.Run("queues nothing when "+c.name, func(t *testing.T) {
			r := newDispatchRig()
			r.critical42()
			c.setup(r)
			r.eval(t, 42, 0)
			if len(r.f.initialCalls) != 0 {
				t.Errorf("queued %v", r.f.initialAccounts())
			}
			if len(r.f.criticalMarks) != 0 {
				t.Errorf("critical marks = %+v", r.f.criticalMarks)
			}
			if r.n.calls != 0 {
				t.Error("the area lookup ran for a report that is not pushed")
			}
		})
	}
}

func TestDispatchCriticalPendingTextCheck(t *testing.T) {
	rig := newDispatchRig()
	rig.critical42()
	rig.f.facts[42] = dispatchReport("critical", "pending")
	rig.eval(t, 42, 0)
	if got := rig.f.initialAccounts(); !dispatchInt64sEqual(got, []int64{11, 12}) {
		t.Fatalf("a pending critical report queued %v, want [11 12]: the lock screen carries none of the reporter's words", got)
	}
	if rig.f.initialCalls[0].Kind != string(PushKindCritical) {
		t.Errorf("kind = %q", rig.f.initialCalls[0].Kind)
	}

	// A pending report that is not serious is held, so nothing is queued.
	held := newDispatchRig()
	held.critical42()
	held.f.facts[42] = dispatchReport("medium", "pending")
	held.eval(t, 42, 0)
	if len(held.f.initialCalls) != 0 {
		t.Errorf("a held report queued %v", held.f.initialAccounts())
	}
}

func TestDispatchCriticalCapPerReporter(t *testing.T) {
	t.Run("the fourth report in an hour is capped for good", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.f.counts["critical"] = PushCriticalReportsPerReporterPerHour
		rig.eval(t, 42, 0)
		if len(rig.f.initialCalls) != 0 {
			t.Errorf("a capped report queued %v", rig.f.initialAccounts())
		}
		if len(rig.f.criticalMarks) != 1 || rig.f.criticalMarks[0].State != "capped" {
			t.Fatalf("critical marks = %+v, want one capped", rig.f.criticalMarks)
		}
		if len(rig.f.countCalls) != 1 {
			t.Fatalf("count calls = %d, want 1", len(rig.f.countCalls))
		}
		cnt := rig.f.countCalls[0]
		if cnt.ExcludeReportID != 42 || cnt.ReporterAccountID != 900 || cnt.Kind != "critical" {
			t.Errorf("count call = %+v: it must leave the report itself out (N2)", cnt)
		}
		if want := pushFactsNow.Add(-time.Hour); !cnt.Since.Equal(want) {
			t.Errorf("since = %v, want %v", cnt.Since, want)
		}

		// Evaluated again, the stored capped state ends it before any count.
		before := len(rig.f.countCalls)
		rig.eval(t, 42, 0)
		if len(rig.f.countCalls) != before {
			t.Error("a report already capped was counted again")
		}
		if len(rig.f.initialCalls) != 0 {
			t.Error("a capped report queued on a later run")
		}
	})

	t.Run("two other reports leave room", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.f.counts["critical"] = PushCriticalReportsPerReporterPerHour - 1
		rig.eval(t, 42, 0)
		if len(rig.f.initialCalls) != 2 {
			t.Errorf("queued %v, want both readers", rig.f.initialAccounts())
		}
	})

	t.Run("N2 a re run serves a new reader and never turns fanned into capped", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.eval(t, 42, 0)
		if len(rig.f.initialCalls) != 2 {
			t.Fatalf("first run queued %v", rig.f.initialAccounts())
		}
		// A reader subscribed since, and the reporter has two other fanned reports.
		rig.f.counts["critical"] = 2
		rig.f.candidates = append(rig.f.candidates, homeRowAt(13, dispLat, dispLon, 0, 5, true, false))
		rig.eval(t, 42, 0)
		if rig.f.areaTried > 1 {
			t.Errorf("the area lookup was tried %d times", rig.f.areaTried)
		}
		newlyQueued := 0
		for acct := range map[int64]bool{11: true, 12: true, 13: true} {
			if rig.f.queued[[2]int64{acct, 42}] {
				newlyQueued++
			}
		}
		if newlyQueued != 3 {
			t.Errorf("queued %d of 3 readers after the re run", newlyQueued)
		}
		for _, m := range rig.f.criticalMarks {
			if m.State == "capped" {
				t.Fatalf("a re run called MarkCriticalFanout with capped: %+v", m)
			}
		}
		if *rig.f.progress[42].CriticalState != "fanned" {
			t.Errorf("critical_state = %q, want fanned", *rig.f.progress[42].CriticalState)
		}

		// With three other fanned reports the re run queues nothing more, and the
		// state stays fanned.
		rig.f.counts["critical"] = PushCriticalReportsPerReporterPerHour
		rig.f.candidates = append(rig.f.candidates, homeRowAt(14, dispLat, dispLon, 0, 5, true, false))
		calls := len(rig.f.initialCalls)
		rig.eval(t, 42, 0)
		if len(rig.f.initialCalls) != calls {
			t.Errorf("a re run blocked by the reporter's other reports still queued %v", rig.f.initialAccounts()[calls:])
		}
		if *rig.f.progress[42].CriticalState != "fanned" {
			t.Errorf("critical_state = %q after a blocked re run, want fanned", *rig.f.progress[42].CriticalState)
		}
		for _, c := range rig.f.countCalls {
			if c.ExcludeReportID != 42 {
				t.Errorf("a count call did not exclude the report: %+v", c)
			}
		}
	})

	t.Run("no readers in range does not use up a slot", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.f.candidates = nil
		rig.eval(t, 42, 0)
		if len(rig.f.criticalMarks) != 0 {
			t.Errorf("critical marks = %+v, want none when nobody was reached", rig.f.criticalMarks)
		}
		if rig.n.calls != 0 {
			t.Error("the area lookup ran with nobody to tell")
		}
	})
}

func TestDispatchCriticalBeforeReliable(t *testing.T) {
	rig := newDispatchRig()
	rig.f.addReliableHistory(900)
	rig.f.facts[42] = dispatchReport("critical", "clear")
	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
	// the stored switches no longer decide, so every reader in range gets the
	// Critical push, and the Reliable branch refreshes nobody for a serious report
	// because those neighbours already got the Critical push.
	rig.f.candidates = []sqlcgen.PushHomeCandidatesRow{
		homeRowAt(11, dispLat, dispLon, 0, 5, true, true),   // Critical and Reliable on: one critical push
		homeRowAt(12, dispLat, dispLon, 0, 5, false, true),  // stored Critical off: still one critical push
		homeRowAt(13, dispLat, dispLon, 0, 5, false, false), // both off: still one critical push
		homeRowAt(900, dispLat, dispLon, 0, 5, false, true), // the author
	}
	rig.eval(t, 42, 0)

	if got := rig.f.initialAccounts(); !dispatchInt64sEqual(got, []int64{11, 12, 13}) {
		t.Fatalf("critical rows = %v, want [11 12 13]", got)
	}
	for _, c := range rig.f.initialCalls {
		if c.Kind != "critical" {
			t.Errorf("kind = %q", c.Kind)
		}
	}
	if len(rig.u.refreshed) != 0 {
		t.Errorf("refreshed = %v, want nobody: a serious report is not a Reliable bell refresh", rig.u.refreshed)
	}
	if n := rig.f.names("EnqueueReliableFromUpdates"); n != 0 {
		t.Errorf("a reliable enqueue ran %d times, want none (D3)", n)
	}
	// N4: a report fanned under Critical holds its one slot and takes no Reliable slot.
	for _, c := range rig.f.countCalls {
		if c.Kind == "reliable" {
			t.Errorf("a Reliable cap count ran for a report fanned under Critical: %+v", c)
		}
	}
}

// TestDispatchMakesNoReliableBellRefresh: a report by a Reliable reporter makes
// the dispatcher refresh nobody's bell, read no reporter history and write no
// Reliable fan out state, whatever the readers in range look like and whether it
// is a new report, a poke from a vote or a serious report. The bell poll writes
// the reliable_nearby row. Amended 2026-11-03 by quick 261103-jal (review
// WR-04): this replaces TestDispatchReliableUsesRefreshRows, whose early refresh
// loop fed no phone note and kept the single dispatcher goroutine busy while a
// Critical fan out could wait behind it.
func TestDispatchMakesNoReliableBellRefresh(t *testing.T) {
	for _, c := range []struct {
		name     string
		severity string
		actor    int64
	}{
		{"a new report", "low", 0},
		{"a poke from a vote", "low", 77},
		{"a serious report", "critical", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			rig := newDispatchRig()
			rig.f.addReliableHistory(900)
			rig.f.facts[42] = dispatchReport(c.severity, "clear")
			rig.f.candidates = []sqlcgen.PushHomeCandidatesRow{
				homeRowAt(11, dispLat, dispLon, 0, 5, true, true),
				homeRowAt(12, dispLat, dispLon, 1, 10, true, true),
				homeRowAt(13, dispLat, dispLon, 0, 5, true, false),
				homeRowAt(900, dispLat, dispLon, 0, 5, true, true), // the author
			}
			rig.eval(t, 42, c.actor)

			if len(rig.u.refreshed) != 0 {
				t.Errorf("refreshed = %v, want nobody: the bell poll writes the Reliable row", rig.u.refreshed)
			}
			if rig.f.names("ReporterHistoryReports") != 0 {
				t.Error("the reporter history was read for a Reliable path that no longer exists")
			}
			if n := rig.f.names("EnqueueReliableFromUpdates"); n != 0 {
				t.Errorf("reliable enqueues = %d, want none (D3)", n)
			}
			for _, cc := range rig.f.countCalls {
				if cc.Kind == "reliable" {
					t.Errorf("a Reliable cap count ran: %+v", cc)
				}
			}
			if c.severity != "critical" && rig.f.names("PushHomeCandidates") != 0 {
				t.Error("home candidates were read for a non serious report")
			}
		})
	}
}

func TestDispatchChangesSkipsVoter(t *testing.T) {
	rig := newDispatchRig()
	rig.f.facts[42] = dispatchReport("low", "clear")
	rig.f.followers = []int64{77, 78, 79, 80}

	// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box):
	// change notes no longer reach a phone, so the branch refreshes the bells of
	// the report's people and queues nothing, whatever any stored switch says.
	rig.eval(t, 42, 77)

	if !dispatchInt64sEqual(rig.f.followerCalls, []int64{42}) {
		t.Fatalf("follower reads = %v", rig.f.followerCalls)
	}
	if !dispatchInt64sEqual(rig.u.refreshed, []int64{78, 79, 80}) {
		t.Errorf("refreshed = %v, want every follower except the voter", rig.u.refreshed)
	}
	if n := rig.f.names("EnqueueChangedFromUpdates"); n != 0 {
		t.Fatalf("changed enqueues = %d, want none (D3)", n)
	}
	if n := rig.f.names("GetPushKindSettings"); n != 0 {
		t.Errorf("the stored switches were read %d times, want none", n)
	}

	t.Run("a sweep poke with no actor reads no followers", func(t *testing.T) {
		r := newDispatchRig()
		r.f.facts[42] = dispatchReport("low", "clear")
		r.f.followers = []int64{78}
		r.eval(t, 42, 0)
		if len(r.f.followerCalls) != 0 || len(r.f.changedCalls) != 0 {
			t.Errorf("follower reads %v, changed enqueues %d", r.f.followerCalls, len(r.f.changedCalls))
		}
	})

	t.Run("a withheld report queues no change", func(t *testing.T) {
		r := newDispatchRig()
		r.f.facts[42] = dispatchReport("critical", "removed")
		r.f.followers = []int64{78}
		r.eval(t, 42, 5)
		if len(r.f.changedCalls) != 0 {
			t.Errorf("changed enqueues = %d for a removed report", len(r.f.changedCalls))
		}
	})

	t.Run("a refresh error for one follower does not stop the others", func(t *testing.T) {
		r := newDispatchRig()
		r.f.facts[42] = dispatchReport("low", "clear")
		r.f.followers = []int64{78, 79}
		r.u.errFor[78] = errors.New("boom")
		r.eval(t, 42, 5)
		if !dispatchInt64sEqual(r.u.refreshed, []int64{78, 79}) {
			t.Errorf("refreshed = %v", r.u.refreshed)
		}
		if len(r.u.refreshed) != 2 {
			t.Error("the follower after the failed one was not refreshed")
		}
	})
}

func TestDispatchConfirmedUpdate(t *testing.T) {
	confirmVotes := []sqlcgen.CurrentVotesForReportsRow{
		pushVote(42, 101, "content", "confirm", "tdr1v8y"),
		pushVote(42, 102, "content", "confirm", "tdr1vc3"),
	}

	t.Run("queued once the report is confirmed by people", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.f.votes = confirmVotes
		rig.eval(t, 42, 0)
		if len(rig.f.confirmedCalls) != 1 {
			t.Fatalf("confirmed enqueues = %d, want 1", len(rig.f.confirmedCalls))
		}
		c := rig.f.confirmedCalls[0]
		if c.ReportID != 42 || !c.Now.Equal(pushFactsNow) || !c.ExpiresAt.Equal(pushFactsNow.Add(PushOtherTTL)) {
			t.Errorf("confirmed enqueue = %+v", c)
		}
	})

	t.Run("not queued while the report is unconfirmed", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.f.votes = confirmVotes[:1]
		rig.eval(t, 42, 0)
		if len(rig.f.confirmedCalls) != 0 {
			t.Errorf("confirmed enqueues = %d, want 0", len(rig.f.confirmedCalls))
		}
	})

	t.Run("still queued long after the first hour", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		row := rig.f.facts[42]
		row.CreatedAt = pushFactsNow.Add(-3 * time.Hour)
		rig.f.facts[42] = row
		rig.f.votes = confirmVotes
		rig.eval(t, 42, 9)
		if len(rig.f.confirmedCalls) != 1 {
			t.Errorf("confirmed enqueues = %d, want 1 for an older report confirmed by a vote", len(rig.f.confirmedCalls))
		}
		if len(rig.f.initialCalls) != 0 {
			t.Error("an old report was pushed as new")
		}
	})

	t.Run("never for a report that is not shown", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		rig.f.facts[42] = dispatchReport("critical", "removed")
		rig.f.votes = confirmVotes
		rig.eval(t, 42, 0)
		if len(rig.f.confirmedCalls) != 0 {
			t.Error("a confirmed update was queued for a removed report")
		}
	})
}

// TestDispatchDisputedCriticalStaysUnconfirmed is review note N3: a Critical
// report that most nearby cells dispute stays Live and listed because disputes
// cannot hide a Critical report, so its push follows the feed, and the confirmed
// update stays out because ConfirmedByPeople is false under a dispute majority.
func TestDispatchDisputedCriticalStaysUnconfirmed(t *testing.T) {
	rig := newDispatchRig()
	rig.critical42()
	rig.f.votes = []sqlcgen.CurrentVotesForReportsRow{
		pushVote(42, 101, "content", "confirm", "tdr1v8y"),
		pushVote(42, 201, "content", "dispute", "tdr1w01"),
		pushVote(42, 202, "content", "dispute", "tdr1w02"),
		pushVote(42, 203, "content", "dispute", "tdr1w03"),
	}
	rig.eval(t, 42, 0)
	if got := rig.f.initialAccounts(); !dispatchInt64sEqual(got, []int64{11, 12}) {
		t.Errorf("a disputed fresh Critical report queued %v, want [11 12]: its push follows the feed", got)
	}
	if len(rig.f.confirmedCalls) != 0 {
		t.Errorf("confirmed enqueues = %d, want 0 under a dispute majority", len(rig.f.confirmedCalls))
	}

	// Two confirming cells do not rescue it while the disputes outweigh them.
	rig2 := newDispatchRig()
	rig2.critical42()
	rig2.f.votes = append([]sqlcgen.CurrentVotesForReportsRow{
		pushVote(42, 101, "content", "confirm", "tdr1v8y"),
		pushVote(42, 102, "content", "confirm", "tdr1vc3"),
	}, rig.f.votes[1:]...)
	rig2.eval(t, 42, 0)
	if len(rig2.f.confirmedCalls) != 0 {
		t.Error("a confirmed update was queued for a Critical report the disputes outweigh")
	}
}

func TestDispatchAreaLookupOnce(t *testing.T) {
	t.Run("looked up once per report with the cap, stored and not repeated", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		row := rig.f.facts[42]
		row.AreaName = nil
		rig.f.facts[42] = row

		rig.eval(t, 42, 0)
		rig.eval(t, 42, 0)
		if rig.n.calls != 1 {
			t.Fatalf("area lookups = %d, want 1", rig.n.calls)
		}
		if len(rig.n.deadline) != 1 || rig.n.deadline[0] > PushAreaLookupTimeout || rig.n.deadline[0] <= 0 {
			t.Errorf("lookup deadline = %v, want within %v", rig.n.deadline, PushAreaLookupTimeout)
		}
		if len(rig.f.areaSets) != 1 || rig.f.areaSets[0].AreaName != "Central Market" || rig.f.areaSets[0].ReportID != 42 {
			t.Errorf("stored area = %+v", rig.f.areaSets)
		}
		if rig.f.areaTried != 1 {
			t.Errorf("area lookup marks = %d, want 1", rig.f.areaTried)
		}
	})

	t.Run("a failure leaves the name empty and the pushes still queue", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		row := rig.f.facts[42]
		row.AreaName = nil
		rig.f.facts[42] = row
		rig.n.err = errors.New("reader busy")
		rig.n.name = ""
		rig.eval(t, 42, 0)
		rig.eval(t, 42, 0)
		if rig.n.calls != 1 {
			t.Errorf("area lookups = %d, want 1: a failure is not retried on every sweep", rig.n.calls)
		}
		if len(rig.f.areaSets) != 0 {
			t.Errorf("a failed lookup stored %+v", rig.f.areaSets)
		}
		if len(rig.f.queued) != 2 {
			t.Errorf("queued %d accounts, want both readers", len(rig.f.queued))
		}
	})

	t.Run("an empty answer stores nothing", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		row := rig.f.facts[42]
		row.AreaName = nil
		rig.f.facts[42] = row
		rig.n.name = "   "
		rig.eval(t, 42, 0)
		if len(rig.f.areaSets) != 0 {
			t.Errorf("stored %+v", rig.f.areaSets)
		}
	})

	t.Run("a name already stored is never looked up again", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42() // the fixture row already has an area name
		rig.eval(t, 42, 0)
		if rig.n.calls != 0 {
			t.Errorf("area lookups = %d, want 0", rig.n.calls)
		}
	})

	t.Run("a lookup already tried is never repeated", func(t *testing.T) {
		rig := newDispatchRig()
		rig.critical42()
		row := rig.f.facts[42]
		row.AreaName = nil
		rig.f.facts[42] = row
		rig.f.prog(42).AreaLookupAt = pgtype.Timestamptz{Time: pushFactsNow.Add(-time.Minute), Valid: true}
		rig.eval(t, 42, 0)
		if rig.n.calls != 0 {
			t.Errorf("area lookups = %d, want 0", rig.n.calls)
		}
	})

	t.Run("a nil namer is a missing name", func(t *testing.T) {
		f := newDispatchFake()
		f.facts[42] = dispatchReport("critical", "clear")
		f.candidates = []sqlcgen.PushHomeCandidatesRow{homeRowAt(11, dispLat, dispLon, 0, 5, true, false)}
		d := NewPushDispatcher(f, &fakeUpdatesRefresher{}, nil, WithDispatchClock(func() time.Time { return pushFactsNow }))
		if err := d.EvaluateReport(context.Background(), 42, 0); err != nil {
			t.Fatalf("EvaluateReport: %v", err)
		}
		if len(f.initialCalls) != 1 {
			t.Errorf("queued %d, want 1", len(f.initialCalls))
		}
	})
}

func TestReportChangedNeverBlocks(t *testing.T) {
	rig := newDispatchRig()
	done := make(chan struct{})
	go func() {
		for i := 0; i < PushPokeBuffer*3; i++ {
			rig.d.ReportChanged(int64(i+1), 5)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ReportChanged blocked with the buffer full and no runner")
	}
}

func TestDispatchSweeps(t *testing.T) {
	rig := newDispatchRig()
	rig.critical42()
	rig.f.recent = []int64{42, 43}
	rig.f.facts[43] = dispatchReport("low", "clear")

	if err := rig.d.SweepOnce(context.Background()); err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if len(rig.f.recentCalls) != 1 {
		t.Fatalf("recent reads = %d", len(rig.f.recentCalls))
	}
	rc := rig.f.recentCalls[0]
	if rc.RowCap != PushSweepReportCap || !rc.Now.Equal(pushFactsNow) || !rc.Since.Equal(pushFactsNow.Add(-PushChangedWindow)) {
		t.Errorf("recent read = %+v", rc)
	}
	if rig.f.names("PushReportFacts") != 2 {
		t.Errorf("reports evaluated = %d, want 2", rig.f.names("PushReportFacts"))
	}
	if len(rig.f.followerCalls) != 0 {
		t.Error("a sweep evaluation read followers: it carries no actor")
	}

	t.Run("one report failing does not stop the rest", func(t *testing.T) {
		r := newDispatchRig()
		r.critical42()
		r.f.recent = []int64{42, 43}
		r.f.facts[43] = dispatchReport("critical", "clear")
		r.f.enqueueErr = errors.New("boom")
		err := r.d.SweepOnce(context.Background())
		if err == nil {
			t.Error("SweepOnce returned nil with enqueue errors")
		}
		if r.f.names("PushReportFacts") != 2 {
			t.Errorf("reports evaluated = %d, want both", r.f.names("PushReportFacts"))
		}
		if len(r.f.criticalMarks) != 0 {
			t.Errorf("a report whose enqueue failed was marked %+v", r.f.criticalMarks)
		}
	})
}

func waitForDispatch(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDispatchRunSweepsAndHandlesPokes(t *testing.T) {
	rig := newDispatchRig(WithDispatchSweepInterval(10 * time.Millisecond))
	rig.critical42()
	rig.f.recent = []int64{42}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		rig.d.Run(ctx)
		close(done)
	}()

	waitForDispatch(t, "the boot sweep and a ticker sweep", func() bool {
		rig.f.mu.Lock()
		defer rig.f.mu.Unlock()
		return len(rig.f.recentCalls) >= 2
	})
	rig.f.mu.Lock()
	rig.f.followers = []int64{78}
	rig.f.mu.Unlock()
	rig.d.ReportChanged(42, 5)
	waitForDispatch(t, "the poke to be evaluated with its actor", func() bool {
		rig.f.mu.Lock()
		defer rig.f.mu.Unlock()
		return len(rig.f.followerCalls) > 0
	})
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after its context ended")
	}
}

// TestDispatchNeverLogsPlaces drives the failure paths of the dispatcher and
// checks the log output holds no coordinate, area name or description (T-03.3-06).
func TestDispatchNeverLogsPlaces(t *testing.T) {
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })

	rig := newDispatchRig(WithDispatchSweepInterval(10 * time.Millisecond))
	rig.f.addReliableHistory(900)
	rig.critical42()
	rig.f.recent = []int64{42}
	rig.f.followers = []int64{78}
	rig.f.candidates = append(rig.f.candidates, homeRowAt(13, dispLat, dispLon, 0, 5, false, true))
	row := rig.f.facts[42]
	row.AreaName = nil
	rig.f.facts[42] = row
	rig.n.name = ""
	rig.n.err = errors.New("Central Market 12.9716 77.5946 busy")
	rig.u.errFor[13] = errors.New("Central Market 12.9716 77.5946 refresh")
	rig.u.errFor[78] = errors.New("Central Market 12.9716 77.5946 refresh")
	rig.f.enqueueErr = errors.New("insert failed")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		rig.d.Run(ctx)
		close(done)
	}()
	waitForDispatch(t, "a sweep to run", func() bool {
		rig.f.mu.Lock()
		defer rig.f.mu.Unlock()
		return len(rig.f.recentCalls) >= 2
	})
	rig.d.ReportChanged(42, 5)
	waitForDispatch(t, "the poke", func() bool {
		rig.f.mu.Lock()
		defer rig.f.mu.Unlock()
		return len(rig.f.followerCalls) > 0
	})
	cancel()
	<-done

	rig.f.mu.Lock()
	rig.f.enqueueErr = nil
	rig.f.mu.Unlock()
	_ = rig.d.EvaluateReport(context.Background(), 42, 5)

	out := buf.String()
	if out == "" {
		t.Fatal("no log output at all: the failure paths should log their ids")
	}
	for _, banned := range []string{"Central Market", "12.97", "77.59", "water_rising"} {
		if strings.Contains(out, banned) {
			t.Errorf("the log holds %q:\n%s", banned, out)
		}
	}
}
