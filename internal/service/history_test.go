// history_test.go pins the history view classifier, the closure replay and
// the server written history strings (plan 03.1-01). It is package service
// (the internal test package) because it reaches unexported helpers of the
// trust tests. Every helper added here starts with p31 so it cannot collide
// with a name from another plan. No database is needed.

package service

import (
	"strings"
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

func TestHistoryOutcomeLabels(t *testing.T) {
	t.Run("one case per outcome", func(t *testing.T) {
		cases := []struct {
			outcome   Outcome
			confirmed int
			want      string
		}{
			{OutcomeCorroborated, 2, "Confirmed by 2 people nearby"},
			{OutcomeCorroborated, 1, "Confirmed by 1 person nearby"},
			{OutcomeCorroborated, 12, "Confirmed by 12 people nearby"},
			{OutcomeContradicted, 0, "Disputed"},
			{OutcomeUnconfirmed, 0, "Expired unconfirmed"},
			{OutcomeNeutral, 0, "Expired"},
			{Outcome("bogus"), 3, "Expired"},
			{Outcome(""), 0, "Expired"},
		}
		for _, tc := range cases {
			if got := outcomeLabelText(tc.outcome, tc.confirmed); got != tc.want {
				t.Errorf("outcomeLabelText(%q, %d) = %q, want %q", tc.outcome, tc.confirmed, got, tc.want)
			}
		}
	})

	// A critical report that two independent places disputed stays Live on the
	// real read path, but its finished outcome is contradicted, so the history
	// label reads Disputed (D-10).
	t.Run("critical report that was out disputed", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p31Row(31, VoteKindContent, VoteDispute, "dx", p31Now.Add(-3*time.Hour)),
			p31Row(32, VoteKindContent, VoteDispute, "dy", p31Now.Add(-2*time.Hour)),
		}
		meta := ReportMeta{Severity: SeverityCritical, Category: CategoryFlood}
		tally := BuildVoteTally(rows, 1, p31Acct(10))
		got := ClassifyOutcome(meta, tally, p31Now.Add(-time.Hour), p31Now)
		if got != OutcomeContradicted {
			t.Fatalf("ClassifyOutcome = %q, want %q", got, OutcomeContradicted)
		}
		if label := outcomeLabelText(got, tally.ConfirmCells); label != "Disputed" {
			t.Errorf("label = %q, want Disputed", label)
		}
	})

	t.Run("classified end to end", func(t *testing.T) {
		meta := ReportMeta{Severity: SeverityLow, Category: CategoryFlood}
		expired := p31Now.Add(-time.Hour)
		confirms := []sqlcgen.CurrentVotesForReportsRow{
			p31Row(31, VoteKindContent, VoteConfirm, "cx", p31Now.Add(-5*time.Hour)),
			p31Row(32, VoteKindContent, VoteConfirm, "cy", p31Now.Add(-4*time.Hour)),
		}
		tally := BuildVoteTally(confirms, 1, p31Acct(10))
		got := ClassifyOutcome(meta, tally, expired, p31Now)
		if label := outcomeLabelText(got, tally.ConfirmCells); label != "Confirmed by 2 people nearby" {
			t.Errorf("corroborated label = %q", label)
		}
		none := BuildVoteTally(nil, 1, p31Acct(10))
		got = ClassifyOutcome(meta, none, expired, p31Now)
		if label := outcomeLabelText(got, none.ConfirmCells); label != "Expired unconfirmed" {
			t.Errorf("unconfirmed label = %q", label)
		}
		rescue := ReportMeta{Severity: SeverityLow, Category: CategoryRescueNeeded}
		got = ClassifyOutcome(rescue, none, expired, p31Now)
		if label := outcomeLabelText(got, none.ConfirmCells); label != "Expired" {
			t.Errorf("neutral label = %q", label)
		}
	})
}

func TestHistoryCopyIsPlain(t *testing.T) {
	var all []string
	for _, c := range []ClosedBy{"", ClosedByYou, ClosedByPoster, ClosedByPeople, ClosedByPosterAndPeople, ClosedByYouAndPeople} {
		all = append(all, closedText(c))
	}
	for _, o := range append([]Outcome{Outcome("bogus")}, Outcomes...) {
		for n := -1; n <= 120; n++ {
			all = append(all, outcomeLabelText(o, n))
		}
	}
	spans := []time.Duration{-time.Hour, 0, time.Second, 30 * time.Second, time.Minute, 30 * time.Minute, 59 * time.Minute, time.Hour,
		90 * time.Minute, 8 * time.Hour, 24 * time.Hour, 48 * time.Hour}
	for _, d := range spans {
		all = append(all, lifetimeText(p31Now, p31Now.Add(d)))
	}
	for _, s := range all {
		assertPlainCopy(t, s)
		lower := strings.ToLower(s)
		for _, b := range p30Banned {
			if strings.Contains(lower, b) {
				t.Errorf("history copy %q holds the banned word %q", s, b)
			}
		}
	}
}

func TestClosedText(t *testing.T) {
	cases := []struct {
		by   ClosedBy
		want string
	}{
		{ClosedByYou, "Resolved by you"},
		{ClosedByPoster, "Resolved by the poster"},
		{ClosedByPeople, "Resolved by people nearby"},
		{ClosedByPosterAndPeople, "Resolved by the poster and people nearby"},
		{ClosedByYouAndPeople, "Resolved by you and people nearby"},
		{ClosedBy(""), ""},
		{ClosedBy("bogus"), ""},
	}
	for _, tc := range cases {
		if got := closedText(tc.by); got != tc.want {
			t.Errorf("closedText(%q) = %q, want %q", tc.by, got, tc.want)
		}
	}
}

func TestLifetimeText(t *testing.T) {
	cases := []struct {
		span time.Duration
		want string
	}{
		{8 * time.Hour, "after 8 hrs"},
		{24 * time.Hour, "after 24 hrs"},
		{time.Hour, "after 1 hr"},
		{90 * time.Minute, "after 1 hr"},
		{119 * time.Minute, "after 1 hr"},
		{30 * time.Minute, "after 30 min"},
		{30 * time.Second, "after 1 min"},
		{61 * time.Second, "after 2 min"},
		{0, ""},
		{-time.Hour, ""},
	}
	for _, tc := range cases {
		if got := lifetimeText(p31Now, p31Now.Add(tc.span)); got != tc.want {
			t.Errorf("lifetimeText(%v) = %q, want %q", tc.span, got, tc.want)
		}
	}
}

func TestBuildHistoryFacts(t *testing.T) {
	created := p31Now.Add(-10 * time.Hour)
	expires := p31Now.Add(-2 * time.Hour)
	closedAt := p31Now.Add(-5 * time.Hour)

	t.Run("not a history view returns nil", func(t *testing.T) {
		for _, v := range []FeedView{FeedViewActive, FeedViewDisputed, FeedViewNone} {
			if got := BuildHistoryFacts(v, Closure{}, false, OutcomeNeutral, 0, created, expires); got != nil {
				t.Errorf("BuildHistoryFacts(%q) = %+v, want nil", v, got)
			}
		}
	})

	t.Run("resolved", func(t *testing.T) {
		c := Closure{ByReporter: true, ByPeople: true, At: closedAt}
		got := BuildHistoryFacts(FeedViewResolved, c, true, OutcomeCorroborated, 3, created, expires)
		if got == nil {
			t.Fatal("got nil")
		}
		if got.State != "resolved" {
			t.Errorf("State = %q", got.State)
		}
		if !got.At.Equal(closedAt) {
			t.Errorf("At = %v, want %v", got.At, closedAt)
		}
		if got.ClosedBy != ClosedByYouAndPeople || got.ClosedText != "Resolved by you and people nearby" {
			t.Errorf("closed = %q %q", got.ClosedBy, got.ClosedText)
		}
		if got.Outcome != "" || got.OutcomeLabel != "" || got.LifetimeText != "" {
			t.Errorf("expired fields must be empty: %+v", got)
		}
	})

	t.Run("resolved seen by someone else", func(t *testing.T) {
		c := Closure{ByReporter: true, At: closedAt}
		got := BuildHistoryFacts(FeedViewResolved, c, false, OutcomeNeutral, 0, created, expires)
		if got == nil || got.ClosedBy != ClosedByPoster || got.ClosedText != "Resolved by the poster" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("past", func(t *testing.T) {
		got := BuildHistoryFacts(FeedViewPast, Closure{}, false, OutcomeCorroborated, 4, created, expires)
		if got == nil {
			t.Fatal("got nil")
		}
		if got.State != "expired" {
			t.Errorf("State = %q", got.State)
		}
		if !got.At.Equal(expires) {
			t.Errorf("At = %v, want %v", got.At, expires)
		}
		if got.Outcome != OutcomeCorroborated || got.OutcomeLabel != "Confirmed by 4 people nearby" {
			t.Errorf("outcome = %q %q", got.Outcome, got.OutcomeLabel)
		}
		if got.LifetimeText != "after 8 hrs" {
			t.Errorf("LifetimeText = %q", got.LifetimeText)
		}
		if got.ClosedBy != "" || got.ClosedText != "" {
			t.Errorf("resolved fields must be empty: %+v", got)
		}
	})
}
