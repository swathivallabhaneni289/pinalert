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
	"testing"
	"time"

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
