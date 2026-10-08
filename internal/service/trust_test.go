// trust_test.go declares package service (the internal test package, not
// service_test) because independentCellCount is unexported and
// 02-VALIDATION.md names TestIndependentCellCount as a required Wave 0
// test — the test must live inside the package to reach it. Go permits a
// service and a service_test test package side by side in one directory;
// report_test.go and auth_test.go remain package service_test. Because
// this file is internal, every symbol is referenced unqualified
// (VoteConfirm, not service.VoteConfirm).

package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// voteRow builds a sqlcgen.CurrentVotesForReportsRow from its component
// values so the test tables below stay readable.
func voteRow(reportID, accountID int64, kind VoteKind, value VoteValue, cell string) sqlcgen.CurrentVotesForReportsRow {
	return sqlcgen.CurrentVotesForReportsRow{
		ReportID:    reportID,
		AccountID:   accountID,
		Kind:        string(kind),
		Value:       string(value),
		GeohashCell: cell,
	}
}

// testReportLat and testReportLon are the report point every CastVote
// fixture shares, so a vote cast from the same point is always inside the
// D-01 vote radius.
const (
	testReportLat = 12.9716
	testReportLon = 77.5946
)

// testVoteContext builds the ReportVoteContextRow the fake querier returns:
// a low severity flood report at (testReportLat, testReportLon) that expires
// at expiresAt, with the given reporter (nil means no reporter account).
// CreatedAt is derived from the expiry so the pair stays consistent.
func testVoteContext(expiresAt time.Time, reporter *int64) sqlcgen.ReportVoteContextRow {
	return sqlcgen.ReportVoteContextRow{
		Severity:          string(SeverityLow),
		Category:          string(CategoryFlood),
		ExpiresAt:         expiresAt,
		ReporterAccountID: reporter,
		Latitude:          testReportLat,
		Longitude:         testReportLon,
		CreatedAt:         expiresAt.Add(-ExpiryDuration(SeverityLow)),
	}
}

// acctPtr returns a pointer to id, for populating the reporterAccountID
// argument BuildVoteTally and CastVote both take as *int64.
func acctPtr(id int64) *int64 { return &id }

func TestVoteKindAndValueValidation(t *testing.T) {
	if !VoteKindContent.Valid() {
		t.Error("VoteKindContent should be valid")
	}
	if !VoteKindResolution.Valid() {
		t.Error("VoteKindResolution should be valid")
	}
	if VoteKind("").Valid() {
		t.Error("empty VoteKind should be invalid")
	}
	if VoteKind("Content").Valid() {
		t.Error(`VoteKind("Content") should be invalid — exact comparison, no case coercion`)
	}

	if !VoteConfirm.ValidFor(VoteKindContent) {
		t.Error("VoteConfirm should be valid for VoteKindContent")
	}
	if !VoteDispute.ValidFor(VoteKindContent) {
		t.Error("VoteDispute should be valid for VoteKindContent")
	}
	if VoteResolve.ValidFor(VoteKindContent) {
		t.Error("VoteResolve should not be valid for VoteKindContent")
	}
	if VoteReopen.ValidFor(VoteKindContent) {
		t.Error("VoteReopen should not be valid for VoteKindContent")
	}

	if !VoteResolve.ValidFor(VoteKindResolution) {
		t.Error("VoteResolve should be valid for VoteKindResolution")
	}
	if !VoteReopen.ValidFor(VoteKindResolution) {
		t.Error("VoteReopen should be valid for VoteKindResolution")
	}
	if VoteConfirm.ValidFor(VoteKindResolution) {
		t.Error("VoteConfirm should not be valid for VoteKindResolution")
	}
	if VoteDispute.ValidFor(VoteKindResolution) {
		t.Error("VoteDispute should not be valid for VoteKindResolution")
	}

	if VoteValue("confirm ").ValidFor(VoteKindContent) || VoteValue("confirm ").ValidFor(VoteKindResolution) {
		t.Error(`VoteValue("confirm ") should be valid for no kind`)
	}
	if VoteValue("CONFIRM").ValidFor(VoteKindContent) || VoteValue("CONFIRM").ValidFor(VoteKindResolution) {
		t.Error(`VoteValue("CONFIRM") should be valid for no kind`)
	}
}

func TestIndependentCellCount(t *testing.T) {
	cases := []struct {
		name  string
		cells []string
		want  int
	}{
		{"nil slice counts as zero", nil, 0},
		{"empty slice counts as zero", []string{}, 0},
		{"three entries all equal to tdr1qg0 count as one — the TRUST-03 headline: three accounts standing together count once", []string{"tdr1qg0", "tdr1qg0", "tdr1qg0"}, 1},
		{"three distinct cells count as three", []string{"tdr1qg0", "tdr1qg1", "tdr1qg2"}, 3},
		{"mixed cells count only the distinct values", []string{"tdr1qg0", "tdr1qg1", "tdr1qg0", "tdr1qg1", "tdr1qg2"}, 3},
		{"two empty strings count as one — an empty cell is impossible in practice because votes.geohash_cell is NOT NULL and CastVote always computes a 7-character value; asserted anyway so the helper's behaviour is defined rather than accidental", []string{"", ""}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := independentCellCount(tc.cells)
			if got != tc.want {
				t.Errorf("independentCellCount(%v) = %d, want %d", tc.cells, got, tc.want)
			}
		})
	}
}

func TestBuildVoteTally(t *testing.T) {
	cases := []struct {
		name              string
		reportID          int64
		reporterAccountID *int64
		rows              []sqlcgen.CurrentVotesForReportsRow
		want              VoteTally
	}{
		{
			name:     "two confirm rows from distinct accounts in distinct cells count as two",
			reportID: 1,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg0"),
				voteRow(1, 11, VoteKindContent, VoteConfirm, "tdr1qg1"),
			},
			want: VoteTally{ConfirmCells: 2},
		},
		{
			name:     "two confirm rows from distinct accounts in the SAME cell count as one — the discriminating TRUST-03 case: a tally counting rows instead of cells would wrongly read 2",
			reportID: 1,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg0"),
				voteRow(1, 11, VoteKindContent, VoteConfirm, "tdr1qg0"),
			},
			want: VoteTally{ConfirmCells: 1},
		},
		{
			name:     "two dispute rows in distinct cells count as two disputes and zero confirms",
			reportID: 1,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 10, VoteKindContent, VoteDispute, "tdr1qg0"),
				voteRow(1, 11, VoteKindContent, VoteDispute, "tdr1qg1"),
			},
			want: VoteTally{DisputeCells: 2, ConfirmCells: 0},
		},
		{
			name:     "rows for a different report id are ignored entirely, even when they would otherwise dominate the counts — proves the filter 02-04's batched read depends on",
			reportID: 1,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg0"),
				voteRow(2, 20, VoteKindContent, VoteConfirm, "tdr1qg9"),
				voteRow(2, 21, VoteKindContent, VoteConfirm, "tdr1qg8"),
				voteRow(2, 22, VoteKindContent, VoteConfirm, "tdr1qg7"),
			},
			want: VoteTally{ConfirmCells: 1},
		},
		{
			name:              "reporter's own resolve row sets ReporterResolved and excludes it from ResolveCells",
			reportID:          1,
			reporterAccountID: acctPtr(7),
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 7, VoteKindResolution, VoteResolve, "tdr1qg0"),
			},
			want: VoteTally{ReporterResolved: true, ReporterReopened: false, ResolveCells: 0},
		},
		{
			name:              "reporter's own reopen row sets ReporterReopened and excludes it from ReopenCells — the amended-D-16 case, symmetric with the resolve case above",
			reportID:          1,
			reporterAccountID: acctPtr(7),
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 7, VoteKindResolution, VoteReopen, "tdr1qg0"),
			},
			want: VoteTally{ReporterReopened: true, ReporterResolved: false, ReopenCells: 0},
		},
		{
			name:              "reporter's own reopen row still sets only ReporterReopened even with other accounts' resolve/reopen rows also present — the flag is a function of the reporter's own row alone, never of other accounts' volume",
			reportID:          1,
			reporterAccountID: acctPtr(7),
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 7, VoteKindResolution, VoteReopen, "tdr1qg0"),
				voteRow(1, 8, VoteKindResolution, VoteResolve, "tdr1qg1"),
				voteRow(1, 9, VoteKindResolution, VoteResolve, "tdr1qg2"),
				voteRow(1, 10, VoteKindResolution, VoteReopen, "tdr1qg3"),
			},
			want: VoteTally{ReporterReopened: true, ReporterResolved: false, ResolveCells: 2, ReopenCells: 1},
		},
		{
			name:              "two resolve rows from two NON-reporter accounts in distinct cells count toward ResolveCells with both reporter flags false",
			reportID:          1,
			reporterAccountID: acctPtr(999),
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 8, VoteKindResolution, VoteResolve, "tdr1qg0"),
				voteRow(1, 9, VoteKindResolution, VoteResolve, "tdr1qg1"),
			},
			want: VoteTally{ResolveCells: 2, ReporterResolved: false, ReporterReopened: false},
		},
		{
			name:     "a nil reporterAccountID treats no row as the reporter's own — resolve and reopen rows count normally with both flags false",
			reportID: 1,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 8, VoteKindResolution, VoteResolve, "tdr1qg0"),
				voteRow(1, 9, VoteKindResolution, VoteReopen, "tdr1qg1"),
			},
			want: VoteTally{ResolveCells: 1, ReopenCells: 1, ReporterResolved: false, ReporterReopened: false},
		},
		{
			name:     "rows with an unrecognised kind or value are ignored and change no count — the vote vocabulary can grow in Phase 3 without a migration",
			reportID: 1,
			rows: []sqlcgen.CurrentVotesForReportsRow{
				voteRow(1, 8, VoteKind("telemetry"), VoteConfirm, "tdr1qg0"),
				voteRow(1, 9, VoteKindContent, VoteValue("maybe"), "tdr1qg1"),
				voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg2"),
			},
			want: VoteTally{ConfirmCells: 1},
		},
	}

	// The two reporter flags are never both true in any case in this
	// table, and no case asserts that both-true is rejected —
	// CurrentVotesForReports' DISTINCT ON (report_id, account_id, kind)
	// makes it unproducible, and 02-01 explicitly forbids adding a
	// defensive invariant check for it in this plan.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildVoteTally(tc.rows, tc.reportID, tc.reporterAccountID)
			if got != tc.want {
				t.Errorf("BuildVoteTally() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestBuildVoteTallyExcludesReporterOwnContentVote(t *testing.T) {
	rows := []sqlcgen.CurrentVotesForReportsRow{
		voteRow(1, 7, VoteKindContent, VoteConfirm, "tdr1qg0"),
		voteRow(1, 8, VoteKindContent, VoteConfirm, "tdr1qg1"),
	}
	got := BuildVoteTally(rows, 1, acctPtr(7))
	if got.ConfirmCells != 1 {
		t.Errorf("ConfirmCells = %d, want 1 — D-03 stops these being written in the first place; this is the read-side second layer that also covers any row predating the check (T-02-05)", got.ConfirmCells)
	}
}

// TestBuildVoteTallyOnlyReporterSetsReporterReopened is the
// security-critical counterpart to
// TestBuildVoteTallyExcludesReporterOwnContentVote, and the obligation
// 02-01's T-02-03 row hands to this plan by name. Before D-16's amendment,
// the ABSENCE of a reporter-reopen field was itself the enforcement —
// violating D-16 required visibly adding a struct field. That structural
// guarantee is deliberately gone now that ReporterReopened exists as a
// field; Resolve() is a pure function that structurally cannot replace it,
// so this read-side identity check is now the WHOLE of the control. A
// caller able to set ReporterReopened for a non-reporter could un-retract
// any report in the system.
func TestBuildVoteTallyOnlyReporterSetsReporterReopened(t *testing.T) {
	t.Run("a reopen row from a non-reporter account leaves ReporterReopened false and counts as one ordinary cell", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindResolution, VoteReopen, "tdr1qg0"),
		}
		got := BuildVoteTally(rows, 1, acctPtr(7))
		if got.ReporterReopened {
			t.Error("ReporterReopened should be false for a non-reporter's reopen row")
		}
		if got.ReopenCells != 1 {
			t.Errorf("ReopenCells = %d, want 1", got.ReopenCells)
		}
	})

	t.Run("a nil reporter must never compare equal to any account", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindResolution, VoteReopen, "tdr1qg0"),
		}
		got := BuildVoteTally(rows, 1, nil)
		if got.ReporterReopened {
			t.Error("ReporterReopened should be false when reporterAccountID is nil")
		}
		if got.ReopenCells != 1 {
			t.Errorf("ReopenCells = %d, want 1", got.ReopenCells)
		}
	})

	t.Run("no volume of non-reporter reopens can ever promote itself into the reporter's instant flag", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindResolution, VoteReopen, "tdr1qg0"),
			voteRow(1, 10, VoteKindResolution, VoteReopen, "tdr1qg1"),
			voteRow(1, 11, VoteKindResolution, VoteReopen, "tdr1qg2"),
		}
		got := BuildVoteTally(rows, 1, acctPtr(7))
		if got.ReporterReopened {
			t.Error("ReporterReopened should still be false regardless of non-reporter reopen volume — it reaches IndependentAgreementThreshold through ReopenCells or not at all (D-14)")
		}
		if got.ReopenCells != 3 {
			t.Errorf("ReopenCells = %d, want 3", got.ReopenCells)
		}
	})

	t.Run("the mirror assertion for the resolve half — a resolve row from a non-reporter account leaves ReporterResolved false", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindResolution, VoteResolve, "tdr1qg0"),
		}
		got := BuildVoteTally(rows, 1, acctPtr(7))
		if got.ReporterResolved {
			t.Error("ReporterResolved should be false for a non-reporter's resolve row")
		}
		if got.ResolveCells != 1 {
			t.Errorf("ResolveCells = %d, want 1", got.ResolveCells)
		}
	})

	t.Run("the reporter's own reopen row routes to the flag while a non-reporter's reopen row reaches the count — the discriminating case", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 7, VoteKindResolution, VoteReopen, "tdr1qg0"),
			voteRow(1, 9, VoteKindResolution, VoteReopen, "tdr1qg1"),
		}
		got := BuildVoteTally(rows, 1, acctPtr(7))
		if !got.ReporterReopened {
			t.Error("ReporterReopened should be true for the reporter's own reopen row")
		}
		if got.ReopenCells != 1 {
			t.Errorf("ReopenCells = %d, want 1 — only the non-reporter's row should reach the count", got.ReopenCells)
		}
	})
}

func TestBuildVoteTallyFeedsResolveEndToEnd(t *testing.T) {
	meta := ReportMeta{Severity: SeverityLow, Category: CategoryFlood}

	t.Run("two confirms in distinct cells resolve to Live/Confirmed", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg0"),
			voteRow(1, 11, VoteKindContent, VoteConfirm, "tdr1qg1"),
		}
		tally := BuildVoteTally(rows, 1, nil)
		vis, reason := Resolve(meta, tally, time.Now())
		if vis != VisibilityLive || reason != ReasonConfirmed {
			t.Errorf("Resolve() = %v/%v, want %v/%v", vis, reason, VisibilityLive, ReasonConfirmed)
		}
	})

	t.Run("the same two confirms in ONE cell resolve to Provisional/AwaitingConfirmation", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg0"),
			voteRow(1, 11, VoteKindContent, VoteConfirm, "tdr1qg0"),
		}
		tally := BuildVoteTally(rows, 1, nil)
		vis, reason := Resolve(meta, tally, time.Now())
		if vis != VisibilityProvisional || reason != ReasonAwaitingConfirmation {
			t.Errorf("Resolve() = %v/%v, want %v/%v", vis, reason, VisibilityProvisional, ReasonAwaitingConfirmation)
		}
	})
}

// fakeVotingQuerier is the recording fake CastVote's tests drive — the
// exact shape auth_test.go's fakeAuthQuerier established: one programmable
// return value (plus error) per method, every InsertVoteParams it
// received recorded verbatim, and an ordered call log every method
// appends to, which is what makes
// TestCastVoteReturnsFreshlyResolvedVisibility's ordering assertion and
// every "zero calls recorded" assertion mechanical rather than inferred.
type fakeVotingQuerier struct {
	calls []string

	voteContextRow sqlcgen.ReportVoteContextRow
	voteContextErr error

	currentVotesRows []sqlcgen.CurrentVotesForReportsRow
	currentVotesErr  error

	insertVoteErr error
	insertedVotes []sqlcgen.InsertVoteParams

	// currentVotesIDs is the id slice the last CurrentVotesForReports call
	// received.
	currentVotesIDs []int64

	// historyRows/historyErr/historyArgs drive ReporterHistoryReports (plan
	// 03-12). The fake returns only the rows of the requested accounts, like
	// the query, and an empty result when nothing is configured.
	historyRows []sqlcgen.ReporterHistoryReportsRow
	historyErr  error
	historyArgs []sqlcgen.ReporterHistoryReportsParams
}

func (f *fakeVotingQuerier) ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error) {
	f.calls = append(f.calls, "ReportVoteContext")
	if f.voteContextErr != nil {
		return sqlcgen.ReportVoteContextRow{}, f.voteContextErr
	}
	return f.voteContextRow, nil
}

func (f *fakeVotingQuerier) InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error {
	f.calls = append(f.calls, "InsertVote")
	f.insertedVotes = append(f.insertedVotes, arg)
	return f.insertVoteErr
}

func (f *fakeVotingQuerier) CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error) {
	f.calls = append(f.calls, "CurrentVotesForReports")
	f.currentVotesIDs = reportIDs
	if f.currentVotesErr != nil {
		return nil, f.currentVotesErr
	}
	return f.currentVotesRows, nil
}

// ReporterHistoryReports records the call and returns the rows of the
// requested accounts, which is the query's own account filter.
func (f *fakeVotingQuerier) ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error) {
	f.calls = append(f.calls, "ReporterHistoryReports")
	f.historyArgs = append(f.historyArgs, arg)
	if f.historyErr != nil {
		return nil, f.historyErr
	}
	var out []sqlcgen.ReporterHistoryReportsRow
	for _, row := range f.historyRows {
		if slices.Contains(arg.AccountIds, row.AccountID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// p12CastNow is the clock of the plan 03-12 vote tests.
var p12CastNow = time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)

// p12CastHistory returns n judged history reports of one account, each
// confirmed by accounts 21 and 22 in two distinct cells, with those votes.
func p12CastHistory(accountID, firstReportID int64, n int) ([]sqlcgen.ReporterHistoryReportsRow, []sqlcgen.CurrentVotesForReportsRow) {
	history := make([]sqlcgen.ReporterHistoryReportsRow, 0, n)
	var votes []sqlcgen.CurrentVotesForReportsRow
	for i := 0; i < n; i++ {
		id := firstReportID + int64(i)
		history = append(history, sqlcgen.ReporterHistoryReportsRow{
			AccountID: accountID,
			ReportID:  id,
			Severity:  string(SeverityLow),
			Category:  string(CategoryFlood),
			ExpiresAt: p12CastNow.Add(-48 * time.Hour),
		})
		for j, account := range []int64{21, 22} {
			votes = append(votes, sqlcgen.CurrentVotesForReportsRow{
				ReportID:    id,
				AccountID:   account,
				Kind:        string(VoteKindContent),
				Value:       string(VoteConfirm),
				GeohashCell: "p12cell" + string(rune('A'+j)),
				CreatedAt:   p12CastNow.Add(-72 * time.Hour),
			})
		}
	}
	return history, votes
}

// TestCastVoteReadsReporterHistoryOnce proves CastVote reads the reporter's
// history once, after every refusal and before the insert, keyed by the
// report's reporter account, and that its ids join the one read back.
func TestCastVoteReadsReporterHistoryOnce(t *testing.T) {
	clock := func() time.Time { return p12CastNow }
	confirm := func(q *fakeVotingQuerier, lat float64) (CastVoteResult, error) {
		svc := NewVotingService(q, WithVotingClock(clock))
		return svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: lat, Longitude: testReportLon,
		})
	}
	expires := p12CastNow.Add(7 * time.Hour)

	t.Run("a reporter account gives one history read between the context and the insert", func(t *testing.T) {
		history, votes := p12CastHistory(7, 101, 5)
		q := &fakeVotingQuerier{
			voteContextRow:   testVoteContext(expires, acctPtr(7)),
			currentVotesRows: votes,
			historyRows:      history,
		}
		res, err := confirm(q, testReportLat)
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		wantCalls := []string{"ReportVoteContext", "ReporterHistoryReports", "InsertVote", "CurrentVotesForReports"}
		if !slices.Equal(q.calls, wantCalls) {
			t.Fatalf("calls = %v, want %v", q.calls, wantCalls)
		}
		wantArgs := sqlcgen.ReporterHistoryReportsParams{
			Now:         p12CastNow,
			WindowStart: p12CastNow.Add(-ReliabilityWindow),
			RowCap:      int32(ReliabilityHistoryReadCap),
			AccountIds:  []int64{7},
		}
		if len(q.historyArgs) != 1 || !reflect.DeepEqual(q.historyArgs[0], wantArgs) {
			t.Errorf("historyArgs = %+v, want one entry %+v", q.historyArgs, wantArgs)
		}
		if want := []int64{1, 101, 102, 103, 104, 105}; !slices.Equal(q.currentVotesIDs, want) {
			t.Errorf("currentVotesIDs = %v, want %v", q.currentVotesIDs, want)
		}
		want := ReporterSignals([]int64{7}, history, votes, p12CastNow)[7]
		if want.Tag != ReporterReliable || !reflect.DeepEqual(res.Trust.Reporter, want) {
			t.Errorf("reporter = %+v, want reliable %+v", res.Trust.Reporter, want)
		}
	})

	t.Run("a report with no reporter account reads no history", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: testVoteContext(expires, nil)}
		res, err := confirm(q, testReportLat)
		if err != nil {
			t.Fatalf("CastVote: %v", err)
		}
		wantCalls := []string{"ReportVoteContext", "InsertVote", "CurrentVotesForReports"}
		if !slices.Equal(q.calls, wantCalls) {
			t.Fatalf("calls = %v, want %v", q.calls, wantCalls)
		}
		if want := []int64{1}; !slices.Equal(q.currentVotesIDs, want) {
			t.Errorf("currentVotesIDs = %v, want %v", q.currentVotesIDs, want)
		}
		if !reflect.DeepEqual(res.Trust.Reporter, ReporterSignal{Tag: ReporterNone}) {
			t.Errorf("reporter = %+v, want none", res.Trust.Reporter)
		}
	})

	t.Run("a failed history read writes no vote", func(t *testing.T) {
		boom := errors.New("history unavailable")
		q := &fakeVotingQuerier{voteContextRow: testVoteContext(expires, acctPtr(7)), historyErr: boom}
		res, err := confirm(q, testReportLat)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
		if len(q.insertedVotes) != 0 {
			t.Errorf("insertedVotes = %v, want none", q.insertedVotes)
		}
		wantCalls := []string{"ReportVoteContext", "ReporterHistoryReports"}
		if !slices.Equal(q.calls, wantCalls) {
			t.Errorf("calls = %v, want %v", q.calls, wantCalls)
		}
		if !reflect.DeepEqual(res, CastVoteResult{}) {
			t.Errorf("result = %+v, want the zero CastVoteResult", res)
		}
	})

	t.Run("a refused vote reads no history", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: testVoteContext(expires, acctPtr(7))}
		_, err := confirm(q, 12.9806)
		if !errors.Is(err, ErrVoterTooFar) {
			t.Fatalf("err = %v, want ErrVoterTooFar", err)
		}
		if want := []string{"ReportVoteContext"}; !slices.Equal(q.calls, want) {
			t.Errorf("calls = %v, want %v", q.calls, want)
		}
	})
}

func TestCastVoteRejectsReporterContentVote(t *testing.T) {
	reporterID := int64(7)
	for _, v := range []VoteValue{VoteConfirm, VoteDispute} {
		t.Run(string(v), func(t *testing.T) {
			q := &fakeVotingQuerier{
				voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporterID),
			}
			svc := NewVotingService(q)
			_, err := svc.CastVote(context.Background(), CastVoteInput{
				ReportID: 1, AccountID: 7, Kind: VoteKindContent, Value: v,
				Latitude: 12.9716, Longitude: 77.5946,
			})
			if !errors.Is(err, ErrCannotVoteOwnReport) {
				t.Fatalf("err = %v, want ErrCannotVoteOwnReport", err)
			}
			if len(q.insertedVotes) != 0 {
				t.Fatalf("InsertVote called %d times, want 0 — rejecting AFTER writing the row would still fail this requirement", len(q.insertedVotes))
			}
		})
	}
}

// TestCastVoteAllowsReporterResolutionVote proves a block applied to both
// content AND resolution votes would silently break D-13's
// reporter-instant resolve — this test exists next to
// TestCastVoteRejectsReporterContentVote for exactly that reason.
// Structured as a two-case table over {VoteResolve, VoteReopen} so neither
// half of the reporter's resolution privilege can be dropped by a later
// edit without the table visibly shrinking: since D-16's amendment,
// reopen is a reporter privilege of exactly the same standing as resolve.
func TestCastVoteAllowsReporterResolutionVote(t *testing.T) {
	reporterID := int64(7)
	newQuerier := func() *fakeVotingQuerier {
		return &fakeVotingQuerier{
			voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporterID),
		}
	}

	for _, v := range []VoteValue{VoteResolve, VoteReopen} {
		t.Run(string(v), func(t *testing.T) {
			q := newQuerier()
			svc := NewVotingService(q)
			_, err := svc.CastVote(context.Background(), CastVoteInput{
				ReportID: 1, AccountID: 7, Kind: VoteKindResolution, Value: v,
				Latitude: 12.9716, Longitude: 77.5946,
			})
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if len(q.insertedVotes) != 1 {
				t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
			}
		})
	}

	t.Run("reporter's instant resolve resolves the report to Retracted end to end", func(t *testing.T) {
		q := newQuerier()
		q.currentVotesRows = []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 7, VoteKindResolution, VoteResolve, "tdr1qg0"),
		}
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 7, Kind: VoteKindResolution, Value: VoteResolve,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility != VisibilityRetracted || result.Reason != ReasonResolved {
			t.Errorf("result = %+v, want Visibility=%v Reason=%v", result, VisibilityRetracted, ReasonResolved)
		}
	})
}

// TestCastVoteReporterInstantReopen is the exact mirror of
// TestCastVoteAllowsReporterResolutionVote's resolve half, and the reason
// this plan was amended (D-16, 2026-09-15, TRUST-08). Reopening is
// threshold-free for the reporter and threshold-gated for everyone else —
// see 02-CONTEXT.md D-16's amendment history so a reader who finds an
// older document does not "restore" a threshold here.
func TestCastVoteReporterInstantReopen(t *testing.T) {
	reporterID := int64(7)
	baseVoteContext := func() sqlcgen.ReportVoteContextRow {
		return testVoteContext(time.Now().Add(time.Hour), &reporterID)
	}

	t.Run("reporter's own reopen lifts the retraction their own resolve caused, at zero independent reopen cells", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: baseVoteContext()}
		q.currentVotesRows = []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 7, VoteKindResolution, VoteReopen, "tdr1qg0"),
		}
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 7, Kind: VoteKindResolution, Value: VoteReopen,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility == VisibilityRetracted {
			t.Fatalf("Visibility = %v, want NOT Retracted", result.Visibility)
		}
		if result.Visibility != VisibilityProvisional || result.Reason != ReasonAwaitingConfirmation {
			t.Errorf("result = %+v, want Visibility=%v Reason=%v — the report falls back through the ordinary ladder with zero confirm cells standing", result, VisibilityProvisional, ReasonAwaitingConfirmation)
		}
		if len(q.insertedVotes) != 1 {
			t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
		}
		if q.insertedVotes[0].Kind != string(VoteKindResolution) || q.insertedVotes[0].Value != string(VoteReopen) {
			t.Errorf("inserted vote = %+v, want Kind=resolution Value=reopen — the reporter's reopen is a stored vote like any other, not a state mutation", q.insertedVotes[0])
		}
	})

	t.Run("the load-bearing case: reporter's own reopen lifts a retraction that came from independent confirmers, not their own resolve", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: baseVoteContext()}
		q.currentVotesRows = []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 8, VoteKindResolution, VoteResolve, "tdr1qg1"),
			voteRow(1, 10, VoteKindResolution, VoteResolve, "tdr1qg2"),
			voteRow(1, 7, VoteKindResolution, VoteReopen, "tdr1qg0"),
		}
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 7, Kind: VoteKindResolution, Value: VoteReopen,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility == VisibilityRetracted {
			t.Fatalf("Visibility = %v, want NOT Retracted — the reporter's reopen must lift a retraction regardless of how it arose (D-16 amended, TRUST-08)", result.Visibility)
		}
		if len(q.insertedVotes) != 1 {
			t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
		}
	})

	t.Run("the inverse: one non-reporter reopen is below threshold and cannot lift someone else's retraction — proves no threshold crept in", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextRow: baseVoteContext()}
		q.currentVotesRows = []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 8, VoteKindResolution, VoteResolve, "tdr1qg1"),
			voteRow(1, 10, VoteKindResolution, VoteResolve, "tdr1qg2"),
			voteRow(1, 9, VoteKindResolution, VoteReopen, "tdr1qg0"),
		}
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindResolution, Value: VoteReopen,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility != VisibilityRetracted {
			t.Errorf("Visibility = %v, want Retracted — one non-reporter reopen is below IndependentAgreementThreshold and cannot lift someone else's retraction (D-14)", result.Visibility)
		}
		if len(q.insertedVotes) != 1 {
			t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
		}
	})
}

func TestCastVoteAllowsNonReporter(t *testing.T) {
	reporterID := int64(7)
	q := &fakeVotingQuerier{
		voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporterID),
	}
	svc := NewVotingService(q)
	_, err := svc.CastVote(context.Background(), CastVoteInput{
		ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
		Latitude: 12.9716, Longitude: 77.5946,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(q.insertedVotes) != 1 {
		t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
	}
	got := q.insertedVotes[0]
	if got.ReportID != 1 || got.AccountID != 9 || got.Kind != string(VoteKindContent) || got.Value != string(VoteConfirm) {
		t.Errorf("inserted vote = %+v, want ReportID=1 AccountID=9 Kind=content Value=confirm", got)
	}
}

func TestCastVoteTreatsNullReporterAsNobody(t *testing.T) {
	q := &fakeVotingQuerier{
		voteContextRow: testVoteContext(time.Now().Add(time.Hour), nil),
	}
	svc := NewVotingService(q)
	_, err := svc.CastVote(context.Background(), CastVoteInput{
		ReportID: 1, AccountID: 7, Kind: VoteKindContent, Value: VoteConfirm,
		Latitude: 12.9716, Longitude: 77.5946,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil — a nil reporter must not be compared equal to any caller", err)
	}
	if len(q.insertedVotes) != 1 {
		t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
	}
}

func TestCastVoteRejectsExpiredReport(t *testing.T) {
	t.Run("expired one hour ago is rejected and writes no row", func(t *testing.T) {
		q := &fakeVotingQuerier{
			voteContextRow: testVoteContext(time.Now().Add(-time.Hour), nil),
		}
		svc := NewVotingService(q)
		_, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if !errors.Is(err, ErrReportExpired) {
			t.Fatalf("err = %v, want ErrReportExpired", err)
		}
		if len(q.insertedVotes) != 0 {
			t.Fatalf("InsertVote called %d times, want 0", len(q.insertedVotes))
		}
	})

	t.Run("expiring one hour in the future is accepted — pins the predicate's direction", func(t *testing.T) {
		q := &fakeVotingQuerier{
			voteContextRow: testVoteContext(time.Now().Add(time.Hour), nil),
		}
		svc := NewVotingService(q)
		_, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})
}

func TestCastVoteRejectsMissingReport(t *testing.T) {
	q := &fakeVotingQuerier{voteContextErr: pgx.ErrNoRows}
	svc := NewVotingService(q)
	_, err := svc.CastVote(context.Background(), CastVoteInput{
		ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
		Latitude: 12.9716, Longitude: 77.5946,
	})
	if !errors.Is(err, ErrReportNotFound) {
		t.Fatalf("err = %v, want ErrReportNotFound", err)
	}
	if len(q.insertedVotes) != 0 {
		t.Fatalf("InsertVote called %d times, want 0", len(q.insertedVotes))
	}
}

func castVoteAndGetCell(t *testing.T, lat, lon float64) string {
	t.Helper()
	q := &fakeVotingQuerier{
		voteContextRow: testVoteContext(time.Now().Add(time.Hour), nil),
	}
	svc := NewVotingService(q)
	_, err := svc.CastVote(context.Background(), CastVoteInput{
		ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
		Latitude: lat, Longitude: lon,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	return q.insertedVotes[0].GeohashCell
}

func TestCastVoteComputesGeohashCellServerSide(t *testing.T) {
	t.Run("recorded cell matches geohash.EncodeWithPrecision at voterGeohashPrecision, length 7", func(t *testing.T) {
		got := castVoteAndGetCell(t, 12.9716, 77.5946)
		want := geohash.EncodeWithPrecision(12.9716, 77.5946, voterGeohashPrecision)
		if got != want {
			t.Errorf("GeohashCell = %q, want %q", got, want)
		}
		if len(got) != 7 {
			t.Errorf("len(GeohashCell) = %d, want 7", len(got))
		}
	})

	t.Run("two callers ~40m apart produce the same cell; several hundred meters apart produce different cells", func(t *testing.T) {
		lat1, lon1 := 12.9716, 77.5946
		lat2, lon2 := 12.9716+0.0004, 77.5946
		lat3, lon3 := 12.9716+0.005, 77.5946

		if geohash.EncodeWithPrecision(lat1, lon1, voterGeohashPrecision) != geohash.EncodeWithPrecision(lat2, lon2, voterGeohashPrecision) {
			t.Fatalf("test setup invalid: expected lat1/lon1 and lat2/lon2 to fall in the same cell")
		}
		if geohash.EncodeWithPrecision(lat1, lon1, voterGeohashPrecision) == geohash.EncodeWithPrecision(lat3, lon3, voterGeohashPrecision) {
			t.Fatalf("test setup invalid: expected lat1/lon1 and lat3/lon3 to fall in different cells")
		}

		cell1 := castVoteAndGetCell(t, lat1, lon1)
		cell2 := castVoteAndGetCell(t, lat2, lon2)
		cell3 := castVoteAndGetCell(t, lat3, lon3)

		if cell1 != cell2 {
			t.Errorf("cell1 = %q, cell2 = %q, want equal (callers ~40m apart)", cell1, cell2)
		}
		if cell1 == cell3 {
			t.Errorf("cell1 = %q, cell3 = %q, want different (callers several hundred meters apart)", cell1, cell3)
		}
	})
}

func TestCastVoteValidatesInput(t *testing.T) {
	cases := []struct {
		name      string
		in        CastVoteInput
		wantField string
	}{
		{
			name:      "unknown kind",
			in:        CastVoteInput{ReportID: 1, AccountID: 9, Kind: VoteKind("bogus"), Value: VoteConfirm, Latitude: 12.9716, Longitude: 77.5946},
			wantField: "kind",
		},
		{
			name:      "value not valid for kind",
			in:        CastVoteInput{ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteResolve, Latitude: 12.9716, Longitude: 77.5946},
			wantField: "value",
		},
		{
			name:      "latitude out of range",
			in:        CastVoteInput{ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm, Latitude: 91, Longitude: 77.5946},
			wantField: "latitude",
		},
		{
			name:      "longitude out of range",
			in:        CastVoteInput{ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm, Latitude: 12.9716, Longitude: -181},
			wantField: "longitude",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &fakeVotingQuerier{}
			svc := NewVotingService(q)
			_, err := svc.CastVote(context.Background(), tc.in)
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want a ValidationError", err)
			}
			if ve.Field != tc.wantField {
				t.Errorf("ValidationError.Field = %q, want %q", ve.Field, tc.wantField)
			}
			if len(q.calls) != 0 {
				t.Errorf("store calls = %v, want none — validation must run before the store is touched at all", q.calls)
			}
		})
	}
}

func TestCastVoteReturnsFreshlyResolvedVisibility(t *testing.T) {
	newQuerier := func(rows []sqlcgen.CurrentVotesForReportsRow) *fakeVotingQuerier {
		return &fakeVotingQuerier{
			voteContextRow:   testVoteContext(time.Now().Add(time.Hour), nil),
			currentVotesRows: rows,
		}
	}

	t.Run("one confirm leaves the report Provisional", func(t *testing.T) {
		q := newQuerier([]sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindContent, VoteConfirm, "tdr1qg0"),
		})
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility != VisibilityProvisional || result.Reason != ReasonAwaitingConfirmation {
			t.Errorf("result = %+v, want Visibility=%v Reason=%v", result, VisibilityProvisional, ReasonAwaitingConfirmation)
		}
	})

	t.Run("two confirms in distinct cells flip the report to Live — TRUST-03 through the full service path", func(t *testing.T) {
		q := newQuerier([]sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindContent, VoteConfirm, "tdr1qg0"),
			voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg1"),
		})
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 10, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility != VisibilityLive || result.Reason != ReasonConfirmed {
			t.Errorf("result = %+v, want Visibility=%v Reason=%v", result, VisibilityLive, ReasonConfirmed)
		}
	})

	t.Run("two confirms in the SAME cell keep the report Provisional", func(t *testing.T) {
		q := newQuerier([]sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 9, VoteKindContent, VoteConfirm, "tdr1qg0"),
			voteRow(1, 10, VoteKindContent, VoteConfirm, "tdr1qg0"),
		})
		svc := NewVotingService(q)
		result, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 10, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if result.Visibility != VisibilityProvisional {
			t.Errorf("Visibility = %v, want %v", result.Visibility, VisibilityProvisional)
		}
	})

	t.Run("CurrentVotesForReports is called AFTER InsertVote, so the response reflects post-vote state", func(t *testing.T) {
		q := newQuerier(nil)
		svc := NewVotingService(q)
		_, err := svc.CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindContent, Value: VoteConfirm,
			Latitude: 12.9716, Longitude: 77.5946,
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		insertIdx, votesIdx := -1, -1
		for i, c := range q.calls {
			if c == "InsertVote" && insertIdx == -1 {
				insertIdx = i
			}
			if c == "CurrentVotesForReports" && votesIdx == -1 {
				votesIdx = i
			}
		}
		if insertIdx == -1 || votesIdx == -1 || votesIdx < insertIdx {
			t.Errorf("call log = %v, want InsertVote before CurrentVotesForReports", q.calls)
		}
	})
}

// Measured voter points for the distance tests, relative to the report at
// (testReportLat, testReportLon), using R = 6371 km.
const (
	nearLat   = 12.9805 // 0.9896 km north of the report
	farLat    = 12.9806 // 1.0008 km north of the report
	farLat2   = 13.05   // with farLon2: 10.58 km away
	farLon2   = 77.65
	chennaiLa = 13.0827 // Chennai, 290.17 km away
	chennaiLo = 80.2707
)

// voteCombos lists the four vote values with their kinds.
var voteCombos = []struct {
	kind  VoteKind
	value VoteValue
}{
	{VoteKindContent, VoteConfirm},
	{VoteKindContent, VoteDispute},
	{VoteKindResolution, VoteResolve},
	{VoteKindResolution, VoteReopen},
}

// TestCastVoteRejectsTooFar proves the D-01 distance rule: a vote from more
// than 1 km is refused for all four values with nothing written (V-04).
func TestCastVoteRejectsTooFar(t *testing.T) {
	reporterID := int64(7)
	newQ := func() *fakeVotingQuerier {
		return &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(time.Hour), &reporterID)}
	}
	cast := func(q *fakeVotingQuerier, kind VoteKind, value VoteValue, lat, lon float64) error {
		_, err := NewVotingService(q).CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: kind, Value: value, Latitude: lat, Longitude: lon,
		})
		return err
	}

	for _, vc := range voteCombos {
		t.Run("1.0008 km is refused for "+string(vc.value), func(t *testing.T) {
			q := newQ()
			err := cast(q, vc.kind, vc.value, farLat, testReportLon)
			if !errors.Is(err, ErrVoterTooFar) {
				t.Fatalf("err = %v, want ErrVoterTooFar", err)
			}
			if len(q.insertedVotes) != 0 {
				t.Errorf("InsertVote called %d times, want 0", len(q.insertedVotes))
			}
			if len(q.calls) != 1 || q.calls[0] != "ReportVoteContext" {
				t.Errorf("calls = %v, want exactly [ReportVoteContext]", q.calls)
			}
		})
		t.Run("0.9896 km is accepted for "+string(vc.value), func(t *testing.T) {
			q := newQ()
			if err := cast(q, vc.kind, vc.value, nearLat, testReportLon); err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if len(q.insertedVotes) != 1 {
				t.Errorf("InsertVote called %d times, want 1", len(q.insertedVotes))
			}
		})
	}

	t.Run("10.58 km is refused", func(t *testing.T) {
		q := newQ()
		if err := cast(q, VoteKindContent, VoteConfirm, farLat2, farLon2); !errors.Is(err, ErrVoterTooFar) {
			t.Fatalf("err = %v, want ErrVoterTooFar", err)
		}
		if len(q.insertedVotes) != 0 {
			t.Errorf("InsertVote called %d times, want 0", len(q.insertedVotes))
		}
	})

	t.Run("voter NaN coordinates fail closed", func(t *testing.T) {
		for name, pt := range map[string][2]float64{
			"latitude":  {math.NaN(), testReportLon},
			"longitude": {testReportLat, math.NaN()},
		} {
			t.Run(name, func(t *testing.T) {
				q := newQ()
				err := cast(q, VoteKindContent, VoteConfirm, pt[0], pt[1])
				if !errors.Is(err, ErrVoterTooFar) {
					t.Fatalf("err = %v, want ErrVoterTooFar", err)
				}
				if len(q.calls) != 1 || q.calls[0] != "ReportVoteContext" {
					t.Errorf("calls = %v, want exactly [ReportVoteContext]", q.calls)
				}
			})
		}
	})

	t.Run("non finite report coordinates refuse a nearby voter", func(t *testing.T) {
		for name, mutate := range map[string]func(*fakeVotingQuerier){
			"report NaN latitude":  func(q *fakeVotingQuerier) { q.voteContextRow.Latitude = math.NaN() },
			"report Inf longitude": func(q *fakeVotingQuerier) { q.voteContextRow.Longitude = math.Inf(1) },
		} {
			t.Run(name, func(t *testing.T) {
				q := newQ()
				mutate(q)
				err := cast(q, VoteKindContent, VoteConfirm, testReportLat, testReportLon)
				if !errors.Is(err, ErrVoterTooFar) {
					t.Fatalf("err = %v, want ErrVoterTooFar", err)
				}
				if len(q.insertedVotes) != 0 {
					t.Errorf("InsertVote called %d times, want 0", len(q.insertedVotes))
				}
			})
		}
	})

	t.Run("voter Inf latitude is a ValidationError with zero store calls", func(t *testing.T) {
		q := newQ()
		err := cast(q, VoteKindContent, VoteConfirm, math.Inf(1), testReportLon)
		var ve ValidationError
		if !errors.As(err, &ve) || ve.Field != "latitude" {
			t.Fatalf("err = %v, want a ValidationError on latitude", err)
		}
		if len(q.calls) != 0 {
			t.Errorf("calls = %v, want none", q.calls)
		}
	})
}

// TestCastVoteCheckOrder pins the CastVote step order so the distance rule
// never masks a more specific refusal (V-05).
func TestCastVoteCheckOrder(t *testing.T) {
	reporterID := int64(7)
	live := func(reporter *int64) *fakeVotingQuerier {
		return &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(time.Hour), reporter)}
	}
	expired := func(reporter *int64) *fakeVotingQuerier {
		return &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(-time.Hour), reporter)}
	}
	cast := func(q *fakeVotingQuerier, acct int64, kind VoteKind, value VoteValue) error {
		_, err := NewVotingService(q).CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: acct, Kind: kind, Value: value, Latitude: farLat, Longitude: testReportLon,
		})
		return err
	}

	t.Run("invalid kind from far is a ValidationError with zero store calls", func(t *testing.T) {
		q := live(&reporterID)
		err := cast(q, 9, VoteKind("bogus"), VoteConfirm)
		var ve ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("err = %v, want a ValidationError", err)
		}
		if len(q.calls) != 0 {
			t.Errorf("calls = %v, want none", q.calls)
		}
	})

	t.Run("missing report from far is ErrReportNotFound", func(t *testing.T) {
		q := &fakeVotingQuerier{voteContextErr: pgx.ErrNoRows}
		if err := cast(q, 9, VoteKindContent, VoteConfirm); !errors.Is(err, ErrReportNotFound) {
			t.Fatalf("err = %v, want ErrReportNotFound", err)
		}
	})

	t.Run("reporter content vote from far on an unexpired report is ErrCannotVoteOwnReport", func(t *testing.T) {
		if err := cast(live(&reporterID), 7, VoteKindContent, VoteConfirm); !errors.Is(err, ErrCannotVoteOwnReport) {
			t.Fatalf("err = %v, want ErrCannotVoteOwnReport", err)
		}
	})

	t.Run("reporter content vote from far on an expired report is ErrCannotVoteOwnReport", func(t *testing.T) {
		if err := cast(expired(&reporterID), 7, VoteKindContent, VoteDispute); !errors.Is(err, ErrCannotVoteOwnReport) {
			t.Fatalf("err = %v, want ErrCannotVoteOwnReport", err)
		}
	})

	t.Run("non reporter confirm from far on an expired report is ErrReportExpired", func(t *testing.T) {
		if err := cast(expired(&reporterID), 9, VoteKindContent, VoteConfirm); !errors.Is(err, ErrReportExpired) {
			t.Fatalf("err = %v, want ErrReportExpired", err)
		}
	})

	t.Run("reporter resolve from far on an expired report is ErrReportExpired, the exemption never skips expiry", func(t *testing.T) {
		if err := cast(expired(&reporterID), 7, VoteKindResolution, VoteResolve); !errors.Is(err, ErrReportExpired) {
			t.Fatalf("err = %v, want ErrReportExpired", err)
		}
	})

	t.Run("non reporter confirm from far on an unexpired report is ErrVoterTooFar with no cell and no writes", func(t *testing.T) {
		q := live(&reporterID)
		if err := cast(q, 9, VoteKindContent, VoteConfirm); !errors.Is(err, ErrVoterTooFar) {
			t.Fatalf("err = %v, want ErrVoterTooFar", err)
		}
		if len(q.calls) != 1 || q.calls[0] != "ReportVoteContext" {
			t.Errorf("calls = %v, want exactly [ReportVoteContext]", q.calls)
		}
	})
}

// TestCastVoteReporterExemption proves the reporter's own resolve and reopen
// skip the distance rule and nobody else does (T-03-01, V-06).
func TestCastVoteReporterExemption(t *testing.T) {
	reporterID := int64(7)
	newQ := func(reporter *int64) *fakeVotingQuerier {
		return &fakeVotingQuerier{voteContextRow: testVoteContext(time.Now().Add(time.Hour), reporter)}
	}

	for _, v := range []VoteValue{VoteResolve, VoteReopen} {
		t.Run("reporter "+string(v)+" from Chennai succeeds", func(t *testing.T) {
			q := newQ(&reporterID)
			_, err := NewVotingService(q).CastVote(context.Background(), CastVoteInput{
				ReportID: 1, AccountID: 7, Kind: VoteKindResolution, Value: v,
				Latitude: chennaiLa, Longitude: chennaiLo,
			})
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if len(q.insertedVotes) != 1 {
				t.Fatalf("InsertVote called %d times, want 1", len(q.insertedVotes))
			}
			got := q.insertedVotes[0]
			if got.Kind != string(VoteKindResolution) || got.Value != string(v) {
				t.Errorf("inserted = %+v, want Kind=resolution Value=%s", got, v)
			}
			if want := geohash.EncodeWithPrecision(chennaiLa, chennaiLo, voterGeohashPrecision); got.GeohashCell != want {
				t.Errorf("GeohashCell = %q, want %q", got.GeohashCell, want)
			}
		})

		t.Run("non reporter "+string(v)+" from 1.0008 km is refused", func(t *testing.T) {
			q := newQ(&reporterID)
			_, err := NewVotingService(q).CastVote(context.Background(), CastVoteInput{
				ReportID: 1, AccountID: 9, Kind: VoteKindResolution, Value: v,
				Latitude: farLat, Longitude: testReportLon,
			})
			if !errors.Is(err, ErrVoterTooFar) {
				t.Fatalf("err = %v, want ErrVoterTooFar", err)
			}
			if len(q.insertedVotes) != 0 {
				t.Errorf("InsertVote called %d times, want 0", len(q.insertedVotes))
			}
		})
	}

	t.Run("a report with no reporter account exempts nobody", func(t *testing.T) {
		q := newQ(nil)
		_, err := NewVotingService(q).CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 7, Kind: VoteKindResolution, Value: VoteResolve,
			Latitude: chennaiLa, Longitude: chennaiLo,
		})
		if !errors.Is(err, ErrVoterTooFar) {
			t.Fatalf("err = %v, want ErrVoterTooFar", err)
		}
		if len(q.insertedVotes) != 0 {
			t.Errorf("InsertVote called %d times, want 0", len(q.insertedVotes))
		}
	})

	t.Run("a report the reporter already resolved still refuses a far reopen from someone else", func(t *testing.T) {
		q := newQ(&reporterID)
		q.currentVotesRows = []sqlcgen.CurrentVotesForReportsRow{
			voteRow(1, 7, VoteKindResolution, VoteResolve, "tdr1v9y"),
		}
		_, err := NewVotingService(q).CastVote(context.Background(), CastVoteInput{
			ReportID: 1, AccountID: 9, Kind: VoteKindResolution, Value: VoteReopen,
			Latitude: farLat, Longitude: testReportLon,
		})
		if !errors.Is(err, ErrVoterTooFar) {
			t.Fatalf("err = %v, want ErrVoterTooFar", err)
		}
		if len(q.insertedVotes) != 0 {
			t.Errorf("InsertVote called %d times, want 0", len(q.insertedVotes))
		}
	})
}
