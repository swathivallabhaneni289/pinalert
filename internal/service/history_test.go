// history_test.go pins the history view classifier, the closure replay and
// the server written history strings (plan 03.1-01). It is package service
// (the internal test package) because it reaches unexported helpers of the
// trust tests. Every helper added here starts with p31 so it cannot collide
// with a name from another plan. No database is needed.

package service

import (
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// p31Now is the fixed clock of every test in this file.
var p31Now = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// p31Row builds a vote row on report 1.
func p31Row(account int64, kind VoteKind, value VoteValue, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    1,
		AccountID:   account,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   at,
	}
}

func p31Acct(id int64) *int64 { return &id }

func TestViewOf(t *testing.T) {
	future := p31Now.Add(time.Hour)
	past := p31Now.Add(-time.Hour)

	cases := []struct {
		name      string
		v         Visibility
		expiresAt time.Time
		want      FeedView
	}{
		{"live future is active", VisibilityLive, future, FeedViewActive},
		{"provisional future is active", VisibilityProvisional, future, FeedViewActive},
		{"hidden future is disputed", VisibilityHidden, future, FeedViewDisputed},
		{"retracted future is resolved", VisibilityRetracted, future, FeedViewResolved},
		{"retracted past is resolved", VisibilityRetracted, past, FeedViewResolved},
		{"retracted equal is resolved", VisibilityRetracted, p31Now, FeedViewResolved},
		{"live past is past", VisibilityLive, past, FeedViewPast},
		{"provisional past is past", VisibilityProvisional, past, FeedViewPast},
		{"hidden past is past", VisibilityHidden, past, FeedViewPast},
		{"live equal counts as expired", VisibilityLive, p31Now, FeedViewPast},
		{"provisional equal counts as expired", VisibilityProvisional, p31Now, FeedViewPast},
		{"hidden equal counts as expired", VisibilityHidden, p31Now, FeedViewPast},
		{"unknown future is none", Visibility("bogus"), future, FeedViewNone},
		{"unknown past is none", Visibility("bogus"), past, FeedViewNone},
		{"empty is none", Visibility(""), future, FeedViewNone},
		{"wrong case is none", Visibility("LIVE"), future, FeedViewNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ViewOf(tc.v, tc.expiresAt, p31Now); got != tc.want {
				t.Errorf("ViewOf(%q, %v, now) = %q, want %q", tc.v, tc.expiresAt, got, tc.want)
			}
		})
	}

	t.Run("total and disjoint", func(t *testing.T) {
		for _, v := range Visibilities {
			for _, exp := range []time.Time{future, p31Now, past} {
				got := ViewOf(v, exp, p31Now)
				if got == FeedViewNone {
					t.Errorf("ViewOf(%q, %v) = none, want one of the four views", v, exp)
				}
				if !got.Valid() {
					t.Errorf("ViewOf(%q, %v) = %q, which is not a valid FeedView", v, exp, got)
				}
			}
		}
	})
}

func TestFeedViewValid(t *testing.T) {
	want := []FeedView{"active", "disputed", "resolved", "past"}
	if len(FeedViews) != len(want) {
		t.Fatalf("FeedViews has %d entries, want %d", len(FeedViews), len(want))
	}
	for i, w := range want {
		if FeedViews[i] != w {
			t.Errorf("FeedViews[%d] = %q, want %q", i, FeedViews[i], w)
		}
		if !w.Valid() {
			t.Errorf("%q should be valid", w)
		}
	}
	for _, bad := range []FeedView{FeedViewNone, "Active", "PAST", " active", "expired", "closed"} {
		if bad.Valid() {
			t.Errorf("%q should not be valid", bad)
		}
	}
}

func TestClosureOf(t *testing.T) {
	t1 := p31Now.Add(-5 * time.Hour)
	t2 := p31Now.Add(-4 * time.Hour)
	t3 := p31Now.Add(-3 * time.Hour)
	t4 := p31Now.Add(-2 * time.Hour)
	reporter := p31Acct(10)

	cases := []struct {
		name string
		rows []sqlcgen.CurrentVotesForReportsRow
		want Closure
	}{
		{
			name: "no rows",
			rows: nil,
			want: Closure{},
		},
		{
			name: "reporter resolve only",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(10, VoteKindResolution, VoteResolve, "c1", t1),
			},
			want: Closure{ByReporter: true, At: t1},
		},
		{
			name: "two people in two cells close at the second cell",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(21, VoteKindResolution, VoteResolve, "cx", t1),
				p31Row(22, VoteKindResolution, VoteResolve, "cy", t3),
			},
			want: Closure{ByPeople: true, At: t3},
		},
		{
			name: "two resolves in one cell do not close",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(21, VoteKindResolution, VoteResolve, "cx", t1),
				p31Row(22, VoteKindResolution, VoteResolve, "cx", t2),
			},
			want: Closure{},
		},
		{
			name: "flip and re close",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(21, VoteKindResolution, VoteResolve, "cx", t1),
				p31Row(22, VoteKindResolution, VoteValue("unresolve"), "cy", t3),
				p31Row(23, VoteKindResolution, VoteResolve, "cz", t4),
			},
			want: Closure{ByPeople: true, At: t4},
		},
		{
			name: "reporter resolve then reopen stands as reopen",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(10, VoteKindResolution, VoteReopen, "c1", t3),
			},
			want: Closure{},
		},
		{
			name: "people closed first and the reporter joined later",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(21, VoteKindResolution, VoteResolve, "cx", t1),
				p31Row(22, VoteKindResolution, VoteResolve, "cy", t2),
				p31Row(10, VoteKindResolution, VoteResolve, "c1", t4),
			},
			want: Closure{ByReporter: true, ByPeople: true, At: t2},
		},
		{
			name: "reporter closed first and people joined later",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(10, VoteKindResolution, VoteResolve, "c1", t1),
				p31Row(21, VoteKindResolution, VoteResolve, "cx", t2),
				p31Row(22, VoteKindResolution, VoteResolve, "cy", t3),
			},
			want: Closure{ByReporter: true, ByPeople: true, At: t1},
		},
		{
			name: "people reopen lifts the closure",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(10, VoteKindResolution, VoteResolve, "c1", t1),
				p31Row(21, VoteKindResolution, VoteReopen, "cx", t2),
				p31Row(22, VoteKindResolution, VoteReopen, "cy", t3),
			},
			want: Closure{},
		},
		{
			name: "rows of another report are ignored",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				{ReportID: 2, AccountID: 10, Kind: string(VoteKindResolution), Value: string(VoteResolve), GeohashCell: "c1", CreatedAt: t1},
			},
			want: Closure{},
		},
		{
			name: "unknown values and kinds are ignored",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(21, VoteKindResolution, VoteValue("bogus"), "cx", t1),
				p31Row(22, VoteKind("weird"), VoteResolve, "cy", t2),
			},
			want: Closure{},
		},
		{
			name: "content votes do not close",
			rows: []sqlcgen.CurrentVotesForReportsRow{
				p31Row(21, VoteKindContent, VoteConfirm, "cx", t1),
				p31Row(22, VoteKindContent, VoteDispute, "cy", t2),
			},
			want: Closure{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClosureOf(tc.rows, 1, reporter)
			if got.ByReporter != tc.want.ByReporter || got.ByPeople != tc.want.ByPeople || !got.At.Equal(tc.want.At) {
				t.Errorf("ClosureOf = %+v, want %+v", got, tc.want)
			}
		})
	}

	t.Run("input order does not matter", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p31Row(23, VoteKindResolution, VoteResolve, "cz", t4),
			p31Row(22, VoteKindResolution, VoteValue("unresolve"), "cy", t3),
			p31Row(21, VoteKindResolution, VoteResolve, "cx", t1),
		}
		got := ClosureOf(rows, 1, reporter)
		if !got.ByPeople || !got.At.Equal(t4) {
			t.Errorf("ClosureOf = %+v, want ByPeople at %v", got, t4)
		}
	})

	t.Run("nil reporter account", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p31Row(10, VoteKindResolution, VoteResolve, "c1", t1),
		}
		got := ClosureOf(rows, 1, nil)
		if got.ByReporter || got.ByPeople {
			t.Errorf("a lone resolve with no reporter account must not close: %+v", got)
		}
	})

	t.Run("agrees with Resolve", func(t *testing.T) {
		// The closure is non zero exactly when Resolve says Retracted.
		for _, tc := range cases {
			tally := BuildVoteTally(tc.rows, 1, reporter)
			vis, _ := Resolve(ReportMeta{Severity: SeverityLow, Category: CategoryFlood}, tally, p31Now)
			got := ClosureOf(tc.rows, 1, reporter)
			closed := got.ByReporter || got.ByPeople
			if closed != (vis == VisibilityRetracted) {
				t.Errorf("%s: closure %v but Resolve gave %q", tc.name, closed, vis)
			}
		}
	})
}

func TestClosedByFor(t *testing.T) {
	cases := []struct {
		name     string
		c        Closure
		reporter bool
		want     ClosedBy
	}{
		{"zero closure", Closure{}, true, ClosedBy("")},
		{"poster only seen by the poster", Closure{ByReporter: true}, true, ClosedByYou},
		{"poster only seen by someone else", Closure{ByReporter: true}, false, ClosedByPoster},
		{"people only seen by the poster", Closure{ByPeople: true}, true, ClosedByPeople},
		{"people only seen by someone else", Closure{ByPeople: true}, false, ClosedByPeople},
		{"both seen by the poster", Closure{ByReporter: true, ByPeople: true}, true, ClosedByYouAndPeople},
		{"both seen by someone else", Closure{ByReporter: true, ByPeople: true}, false, ClosedByPosterAndPeople},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClosedByFor(tc.c, tc.reporter); got != tc.want {
				t.Errorf("ClosedByFor = %q, want %q", got, tc.want)
			}
		})
	}
}
