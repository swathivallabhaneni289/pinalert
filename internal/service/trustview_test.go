// trustview_test.go declares package service (the internal test package) for
// the same reason trust_test.go does: it reaches the unexported
// filterVoteRows and reuses trust_test.go's acctPtr, testVoteContext and
// fakeVotingQuerier. Every helper added here carries the p05 prefix so it
// cannot collide with another plan's helper in this package.

package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// p05Now is the fixed clock every test in this file reads.
var p05Now = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

// p05VoteAt builds a vote row with every field set, including CreatedAt.
func p05VoteAt(reportID, accountID int64, kind VoteKind, value VoteValue, cell string, at time.Time) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   accountID,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
		CreatedAt:   at,
	}
}

// p05Input builds a TrustInput for report 1 at the fixed clock.
func p05Input(rows []sqlcgen.CurrentVotesForReportsRow, reporter *int64, vis Visibility, expiresAt time.Time) TrustInput {
	return TrustInput{
		Rows:              rows,
		ReportID:          1,
		ReporterAccountID: reporter,
		Meta:              ReportMeta{Severity: SeverityLow, Category: CategoryFlood},
		Visibility:        vis,
		CreatedAt:         expiresAt.Add(-ExpiryDuration(SeverityLow)),
		ExpiresAt:         expiresAt,
		Now:               p05Now,
	}
}

// TestBuildTrustCountsMatchTally is V-11: for any row set, N and M equal the
// resolver's own ConfirmCells and DisputeCells, because both read the same
// filterVoteRows buckets.
func TestBuildTrustCountsMatchTally(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 3))
	cells := []string{"cellA", "cellB", "cellC", "cellD", "cellE", "cellF"}
	kinds := []VoteKind{VoteKindContent, VoteKindResolution, VoteKind("bogus")}
	values := []VoteValue{VoteConfirm, VoteDispute, VoteResolve, VoteReopen, VoteValue("bogus")}
	expires := p05Now.Add(time.Hour)

	for i := 0; i < 500; i++ {
		n := rng.IntN(41)
		rows := make([]sqlcgen.CurrentVotesForReportsRow, 0, n)
		for j := 0; j < n; j++ {
			rows = append(rows, p05VoteAt(
				int64(1+rng.IntN(2)),
				int64(1+rng.IntN(12)),
				kinds[rng.IntN(len(kinds))],
				values[rng.IntN(len(values))],
				cells[rng.IntN(len(cells))],
				p05Now.Add(-time.Duration(rng.IntN(600))*time.Minute),
			))
		}
		var reporter *int64
		if rng.IntN(2) == 0 {
			reporter = acctPtr(int64(1 + rng.IntN(12)))
		}

		tally := BuildVoteTally(rows, 1, reporter)
		got := BuildTrust(p05Input(rows, reporter, VisibilityLive, expires))
		if got.ConfirmedPlaces != tally.ConfirmCells {
			t.Fatalf("iteration %d: ConfirmedPlaces = %d, ConfirmCells = %d", i, got.ConfirmedPlaces, tally.ConfirmCells)
		}
		if got.DisputedPlaces != tally.DisputeCells {
			t.Fatalf("iteration %d: DisputedPlaces = %d, DisputeCells = %d", i, got.DisputedPlaces, tally.DisputeCells)
		}

		shuffled := append([]sqlcgen.CurrentVotesForReportsRow(nil), rows...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		if again := BuildTrust(p05Input(shuffled, reporter, VisibilityLive, expires)); !reflect.DeepEqual(got, again) {
			t.Fatalf("iteration %d: shuffled rows changed the trust object:\n%+v\n%+v", i, got, again)
		}

		// A confirm from a new account in a cell that already confirms
		// changes nothing.
		if confirms := filterVoteRows(rows, 1, reporter).Confirms; len(confirms) > 0 {
			extra := append(append([]sqlcgen.CurrentVotesForReportsRow(nil), rows...),
				p05VoteAt(1, int64(1000+i), VoteKindContent, VoteConfirm, confirms[0].Cell, p05Now))
			if again := BuildTrust(p05Input(extra, reporter, VisibilityLive, expires)); again.ConfirmedPlaces != got.ConfirmedPlaces {
				t.Fatalf("iteration %d: a repeat cell moved N from %d to %d", i, got.ConfirmedPlaces, again.ConfirmedPlaces)
			}
		}

		// The reporter's own content votes in a fresh cell change nothing.
		if reporter != nil {
			own := append(append([]sqlcgen.CurrentVotesForReportsRow(nil), rows...),
				p05VoteAt(1, *reporter, VoteKindContent, VoteConfirm, "p05-fresh", p05Now),
				p05VoteAt(1, *reporter, VoteKindContent, VoteDispute, "p05-fresh", p05Now))
			again := BuildTrust(p05Input(own, reporter, VisibilityLive, expires))
			if again.ConfirmedPlaces != got.ConfirmedPlaces || again.DisputedPlaces != got.DisputedPlaces {
				t.Fatalf("iteration %d: the reporter's own content votes moved the counters", i)
			}
		}

		// A confirm on another report in a fresh cell changes nothing.
		other := append(append([]sqlcgen.CurrentVotesForReportsRow(nil), rows...),
			p05VoteAt(3, 500, VoteKindContent, VoteConfirm, "p05-fresh", p05Now))
		if again := BuildTrust(p05Input(other, reporter, VisibilityLive, expires)); again.ConfirmedPlaces != got.ConfirmedPlaces {
			t.Fatalf("iteration %d: a vote on report 3 moved N", i)
		}
	}

	t.Run("filterVoteRows routing", func(t *testing.T) {
		t1 := p05Now.Add(-4 * time.Minute)
		t2 := p05Now.Add(-3 * time.Minute)
		t3 := p05Now.Add(-2 * time.Minute)
		t4 := p05Now.Add(-1 * time.Minute)
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p05VoteAt(1, 11, VoteKindContent, VoteConfirm, "cellA", t1),
			p05VoteAt(1, 12, VoteKindContent, VoteDispute, "cellB", t2),
			p05VoteAt(1, 7, VoteKindResolution, VoteResolve, "cellR", t3),
			p05VoteAt(1, 13, VoteKindResolution, VoteReopen, "cellC", t4),
			p05VoteAt(1, 7, VoteKindContent, VoteConfirm, "cellD", p05Now),
		}
		got := filterVoteRows(rows, 1, acctPtr(7))
		if want := []placeVote{{Cell: "cellA", CreatedAt: t1}}; !reflect.DeepEqual(got.Confirms, want) {
			t.Errorf("Confirms = %+v, want %+v", got.Confirms, want)
		}
		if want := []placeVote{{Cell: "cellB", CreatedAt: t2}}; !reflect.DeepEqual(got.Disputes, want) {
			t.Errorf("Disputes = %+v, want %+v", got.Disputes, want)
		}
		if len(got.Resolves) != 0 {
			t.Errorf("Resolves = %+v, want empty", got.Resolves)
		}
		if want := []placeVote{{Cell: "cellC", CreatedAt: t4}}; !reflect.DeepEqual(got.Reopens, want) {
			t.Errorf("Reopens = %+v, want %+v", got.Reopens, want)
		}
		if !got.ReporterResolved || got.ReporterReopened {
			t.Errorf("reporter flags = resolved %v reopened %v, want true false", got.ReporterResolved, got.ReporterReopened)
		}
	})
}

// TestTrustLeadSentence covers the lead line rule through BuildTrust.
func TestTrustLeadSentence(t *testing.T) {
	const expiredLine = "This report has expired. It can no longer be confirmed."
	needs := "Needs " + strconv.Itoa(IndependentAgreementThreshold) + " confirmations before this counts as confirmed."
	if needs != "Needs 2 confirmations before this counts as confirmed." {
		t.Fatalf("threshold sentence = %q", needs)
	}
	future := p05Now.Add(time.Hour)

	cases := []struct {
		name    string
		vis     Visibility
		expires time.Time
		want    string
	}{
		{"provisional unexpired", VisibilityProvisional, future, needs},
		{"live unexpired", VisibilityLive, future, ""},
		{"hidden unexpired", VisibilityHidden, future, ""},
		{"retracted unexpired", VisibilityRetracted, future, ""},
		{"retracted wins over expired", VisibilityRetracted, p05Now.Add(-time.Hour), ""},
		{"provisional expiring exactly now", VisibilityProvisional, p05Now, expiredLine},
		{"live one nanosecond before now", VisibilityLive, p05Now.Add(-time.Nanosecond), expiredLine},
		{"live one nanosecond after now", VisibilityLive, p05Now.Add(time.Nanosecond), ""},
		{"hidden expired", VisibilityHidden, p05Now.Add(-time.Hour), expiredLine},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildTrust(p05Input(nil, nil, tc.vis, tc.expires)).LeadSentence
			if got != tc.want {
				t.Errorf("LeadSentence = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTrustCarriesNoIdentifiers walks the Trust type and fails on any field
// named like an identifier or a raw total (T-03-04).
func TestTrustCarriesNoIdentifiers(t *testing.T) {
	banned := regexp.MustCompile(`(?i)cell|account|session|email|vote|total`)
	var walk func(path string, typ reflect.Type)
	walk = func(path string, typ reflect.Type) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if banned.MatchString(f.Name) {
				t.Errorf("field %s.%s matches the identifier ban", path, f.Name)
			}
			walk(path+"."+f.Name, f.Type)
		}
	}
	walk("Trust", reflect.TypeOf(Trust{}))
}

// TestCastVoteResultCarriesTrust proves a vote caller gets the trust object
// built from the post vote rows and the resolved visibility (D-21).
func TestCastVoteResultCarriesTrust(t *testing.T) {
	expires := p05Now.Add(7 * time.Hour)
	clock := func() time.Time { return p05Now }
	meta := ReportMeta{Severity: SeverityLow, Category: CategoryFlood}
	cast := func(q *fakeVotingQuerier, account int64, kind VoteKind, value VoteValue, lat, lon float64, now func() time.Time) (CastVoteResult, error) {
		svc := NewVotingService(q, WithVotingClock(now))
		return svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: account, Kind: kind, Value: value, Latitude: lat, Longitude: lon,
		})
	}
	newQ := func(rows ...sqlcgen.CurrentVotesForReportsRow) *fakeVotingQuerier {
		return &fakeVotingQuerier{
			voteContextRow:   testVoteContext(expires, acctPtr(7)),
			currentVotesRows: rows,
		}
	}

	t.Run("live with three confirming places and one disputing", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			p05VoteAt(1, 9, VoteKindContent, VoteConfirm, "tdr1v9a", p05Now),
			p05VoteAt(1, 10, VoteKindContent, VoteConfirm, "tdr1v9b", p05Now),
			p05VoteAt(1, 11, VoteKindContent, VoteConfirm, "tdr1v9c", p05Now),
			p05VoteAt(1, 12, VoteKindContent, VoteConfirm, "tdr1v9a", p05Now),
			p05VoteAt(1, 13, VoteKindContent, VoteDispute, "tdr1v9d", p05Now),
			p05VoteAt(1, 7, VoteKindContent, VoteConfirm, "tdr1v9e", p05Now),
		}
		q := newQ(rows...)
		got, err := cast(q, 9, VoteKindContent, VoteConfirm, testReportLat, testReportLon, clock)
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		if got.Visibility != VisibilityLive {
			t.Fatalf("Visibility = %q, want live", got.Visibility)
		}
		tr := got.Trust
		if tr.ConfirmedPlaces != 3 || tr.DisputedPlaces != 1 || tr.ConfirmedText != "3" || tr.DisputedText != "1" || tr.LeadSentence != "" {
			t.Errorf("Trust = %+v", tr)
		}
		if want := "3 people in 3 different places confirmed this. Counts separate places, not votes."; tr.ConfirmedSentence != want {
			t.Errorf("ConfirmedSentence = %q, want %q", tr.ConfirmedSentence, want)
		}
		if want := "1 person in 1 place disputed this. Counts separate places, not votes."; tr.DisputedSentence != want {
			t.Errorf("DisputedSentence = %q, want %q", tr.DisputedSentence, want)
		}
		rc := q.voteContextRow
		want := BuildTrust(TrustInput{
			Rows: rows, ReportID: 1, ReporterAccountID: rc.ReporterAccountID, Meta: meta,
			Visibility: VisibilityLive, CreatedAt: rc.CreatedAt, ExpiresAt: rc.ExpiresAt, Now: p05Now,
		})
		if !reflect.DeepEqual(tr, want) {
			t.Errorf("result.Trust differs from BuildTrust over the same rows:\n%+v\n%+v", tr, want)
		}
	})

	t.Run("provisional with one place", func(t *testing.T) {
		q := newQ(p05VoteAt(1, 9, VoteKindContent, VoteConfirm, "tdr1v9a", p05Now))
		got, err := cast(q, 9, VoteKindContent, VoteConfirm, testReportLat, testReportLon, clock)
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		if got.Visibility != VisibilityProvisional {
			t.Fatalf("Visibility = %q, want provisional", got.Visibility)
		}
		tr := got.Trust
		if tr.LeadSentence != "Needs 2 confirmations before this counts as confirmed." || tr.ConfirmedText != "1" || tr.DisputedText != "0" {
			t.Errorf("Trust = %+v", tr)
		}
		if want := "No one nearby has disputed this. Counts separate places, not votes."; tr.DisputedSentence != want {
			t.Errorf("DisputedSentence = %q, want %q", tr.DisputedSentence, want)
		}
	})

	t.Run("the reporter's own resolve retracts and clears the lead", func(t *testing.T) {
		q := newQ(
			p05VoteAt(1, 7, VoteKindResolution, VoteResolve, "tdr1v9e", p05Now),
			p05VoteAt(1, 9, VoteKindContent, VoteConfirm, "tdr1v9a", p05Now),
		)
		got, err := cast(q, 7, VoteKindResolution, VoteResolve, testReportLat, testReportLon, clock)
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		if got.Visibility != VisibilityRetracted {
			t.Fatalf("Visibility = %q, want retracted", got.Visibility)
		}
		if got.Trust.LeadSentence != "" || got.Trust.ConfirmedPlaces != 1 {
			t.Errorf("Trust = %+v", got.Trust)
		}
	})

	t.Run("a refused vote carries no trust", func(t *testing.T) {
		q := newQ(p05VoteAt(1, 9, VoteKindContent, VoteConfirm, "tdr1v9a", p05Now))
		got, err := cast(q, 9, VoteKindContent, VoteConfirm, 13.0827, 80.2707, clock)
		if !errors.Is(err, ErrVoterTooFar) {
			t.Fatalf("err = %v, want ErrVoterTooFar", err)
		}
		if !reflect.DeepEqual(got, CastVoteResult{}) {
			t.Errorf("result = %+v, want the zero CastVoteResult", got)
		}
	})

	t.Run("the injected clock decides expiry", func(t *testing.T) {
		build := func() *fakeVotingQuerier {
			q := newQ(p05VoteAt(1, 9, VoteKindContent, VoteConfirm, "tdr1v9a", p05Now))
			q.voteContextRow = testVoteContext(p05Now.Add(time.Minute), acctPtr(7))
			return q
		}
		if _, err := cast(build(), 9, VoteKindContent, VoteConfirm, testReportLat, testReportLon, clock); err != nil {
			t.Errorf("clock at now: err = %v, want nil", err)
		}
		late := func() time.Time { return p05Now.Add(2 * time.Minute) }
		if _, err := cast(build(), 9, VoteKindContent, VoteConfirm, testReportLat, testReportLon, late); !errors.Is(err, ErrReportExpired) {
			t.Errorf("clock past expiry: err = %v, want ErrReportExpired", err)
		}
	})
}
