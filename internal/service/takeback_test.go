package service_test

import (
	"reflect"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestViewerResolution pins the helper that tells a read path what the viewer's
// own standing resolution vote is, so a screen can offer the take back only to
// someone who has a resolve to take back (D-22).
func TestViewerResolution(t *testing.T) {
	t.Run("returns the viewer's resolve for that report", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindResolution, service.VoteResolve, "cellA")}
		if got := service.ViewerResolution(rows, 1, 9); got != service.VoteResolve {
			t.Errorf("got %q, want %q", got, service.VoteResolve)
		}
	})

	t.Run("returns the viewer's reopen for that report", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindResolution, service.VoteReopen, "cellA")}
		if got := service.ViewerResolution(rows, 1, 9); got != service.VoteReopen {
			t.Errorf("got %q, want %q", got, service.VoteReopen)
		}
	})

	t.Run("returns empty for another report", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(2, 9, service.VoteKindResolution, service.VoteResolve, "cellA")}
		if got := service.ViewerResolution(rows, 1, 9); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns empty for another account", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 7, service.VoteKindResolution, service.VoteResolve, "cellA")}
		if got := service.ViewerResolution(rows, 1, 9); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("ignores content rows", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindContent, service.VoteConfirm, "cellA")}
		if got := service.ViewerResolution(rows, 1, 9); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("viewer id 0 matches nothing, even a row whose account id is 0", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 0, service.VoteKindResolution, service.VoteResolve, "cellA")}
		if got := service.ViewerResolution(rows, 1, 0); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("an unknown value on a resolution row is not surfaced", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindResolution, service.VoteValue("bogus"), "cellA")}
		if got := service.ViewerResolution(rows, 1, 9); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// TestUndosUsedCountsPosterReopenOnly pins D-22 and D-28: the budget is the
// unresolves of any account plus the reopens of the poster alone, and a
// non poster's earlier reopen rows never spend it.
func TestUndosUsedCountsPosterReopenOnly(t *testing.T) {
	row := sqlcgen.ResolutionUndoCountsRow{ReportID: 1, Unresolves: 1, Reopens: 1}

	if got := service.UndosUsed(row, true); got != 2 {
		t.Errorf("poster used = %d, want 2", got)
	}
	if got := service.UndosUsed(row, false); got != 1 {
		t.Errorf("non poster used = %d, want 1", got)
	}
	if got := service.UndosUsed(sqlcgen.ResolutionUndoCountsRow{}, true); got != 0 {
		t.Errorf("empty row used = %d, want 0", got)
	}

	cases := []struct{ used, want int }{
		{0, service.UndoLimit},
		{1, service.UndoLimit - 1},
		{service.UndoLimit, 0},
		{service.UndoLimit + 3, 0},
		{-1, service.UndoLimit},
	}
	for _, tc := range cases {
		if got := service.UndosLeft(tc.used); got != tc.want {
			t.Errorf("UndosLeft(%d) = %d, want %d", tc.used, got, tc.want)
		}
	}
	if service.UndoLimit != 2 {
		t.Errorf("UndoLimit = %d, want 2 (D-22)", service.UndoLimit)
	}
}

// TestViewerResolutionSeesTheTakeBack covers the neutral value once it exists:
// a viewer whose latest resolution row is unresolve reads unresolve, so a
// screen can tell "nothing standing to take back" from "resolved".
func TestViewerResolutionSeesTheTakeBack(t *testing.T) {
	rows := []sqlcgen.CurrentVotesForReportsRow{voteRow(1, 9, service.VoteKindResolution, service.VoteUnresolve, "cellA")}
	if got := service.ViewerResolution(rows, 1, 9); got != service.VoteUnresolve {
		t.Errorf("got %q, want %q", got, service.VoteUnresolve)
	}
}

// TestTakeBackNeverEntersReopenCells is T-31-05: the neutral value drops the
// caller's resolve and adds nothing, so it can never help reopen a report that
// other people closed (D-22, D-28).
func TestTakeBackNeverEntersReopenCells(t *testing.T) {
	const reportID, reporter = int64(1), int64(7)
	reporterID := reporter

	t.Run("a non poster unresolve drops their resolve and counts nowhere", func(t *testing.T) {
		before := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(reportID, 8, service.VoteKindResolution, service.VoteResolve, "tdr1qg1"),
			voteRow(reportID, 10, service.VoteKindResolution, service.VoteResolve, "tdr1qg2"),
		}
		after := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(reportID, 8, service.VoteKindResolution, service.VoteUnresolve, "tdr1qg1"),
			voteRow(reportID, 10, service.VoteKindResolution, service.VoteResolve, "tdr1qg2"),
		}
		b := service.BuildVoteTally(before, reportID, &reporterID)
		a := service.BuildVoteTally(after, reportID, &reporterID)
		if b.ResolveCells != 2 || b.ReopenCells != 0 {
			t.Fatalf("before = %+v, want 2 resolve cells and 0 reopen cells", b)
		}
		if a.ResolveCells != 1 {
			t.Errorf("ResolveCells = %d, want 1: the take back drops that account's resolve", a.ResolveCells)
		}
		if a.ReopenCells != 0 {
			t.Errorf("ReopenCells = %d, want 0: the take back must never count as a reopen", a.ReopenCells)
		}
	})

	t.Run("a reopen by the same non poster would count, which is why the neutral value exists", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(reportID, 8, service.VoteKindResolution, service.VoteReopen, "tdr1qg1"),
		}
		if got := service.BuildVoteTally(rows, reportID, &reporterID); got.ReopenCells != 1 {
			t.Fatalf("control: ReopenCells = %d, want 1", got.ReopenCells)
		}
	})

	t.Run("a reporter unresolve sets neither reporter flag", func(t *testing.T) {
		rows := []sqlcgen.CurrentVotesForReportsRow{
			voteRow(reportID, reporter, service.VoteKindResolution, service.VoteUnresolve, "tdr1qg0"),
		}
		got := service.BuildVoteTally(rows, reportID, &reporterID)
		if got.ReporterResolved || got.ReporterReopened {
			t.Errorf("tally = %+v, want ReporterResolved and ReporterReopened both false", got)
		}
		if got.ResolveCells != 0 || got.ReopenCells != 0 {
			t.Errorf("tally = %+v, want no resolve or reopen cells", got)
		}
	})
}

// TestTakeBackLeavesRatingsUnchanged is T-31-08: appending take back rows to a
// reporter's expired history changes no outcome and no rating, because
// ClassifyOutcome clears every resolution vote before it asks Resolve.
func TestTakeBackLeavesRatingsUnchanged(t *testing.T) {
	const reporter = int64(7)
	reporterID := reporter
	now := time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)

	content := func(id int64) []sqlcgen.CurrentVotesForReportsRow {
		return []sqlcgen.CurrentVotesForReportsRow{
			voteRow(id, 21, service.VoteKindContent, service.VoteConfirm, "tdr1qga"),
			voteRow(id, 22, service.VoteKindContent, service.VoteConfirm, "tdr1qgb"),
		}
	}

	var beforeOutcomes, afterOutcomes []service.JudgedOutcome
	for i := int64(1); i <= 4; i++ {
		meta := service.ReportMeta{Severity: service.SeverityLow, Category: service.CategoryFlood}
		expires := now.Add(-time.Duration(i) * 24 * time.Hour)

		before := append(content(i),
			voteRow(i, 8, service.VoteKindResolution, service.VoteResolve, "tdr1qg1"),
			voteRow(i, reporter, service.VoteKindResolution, service.VoteResolve, "tdr1qg0"),
		)
		after := append(content(i),
			voteRow(i, 8, service.VoteKindResolution, service.VoteUnresolve, "tdr1qg1"),
			voteRow(i, reporter, service.VoteKindResolution, service.VoteUnresolve, "tdr1qg0"),
		)

		ob := service.ClassifyOutcome(meta, service.BuildVoteTally(before, i, &reporterID), expires, now)
		oa := service.ClassifyOutcome(meta, service.BuildVoteTally(after, i, &reporterID), expires, now)
		if ob != oa {
			t.Errorf("report %d outcome changed from %v to %v after the take backs", i, ob, oa)
		}
		beforeOutcomes = append(beforeOutcomes, service.JudgedOutcome{Outcome: ob, ExpiresAt: expires})
		afterOutcomes = append(afterOutcomes, service.JudgedOutcome{Outcome: oa, ExpiresAt: expires})
	}

	rb := service.ScoreReliability(beforeOutcomes, now)
	ra := service.ScoreReliability(afterOutcomes, now)
	if !reflect.DeepEqual(rb, ra) {
		t.Errorf("ScoreReliability changed: before %+v, after %+v", rb, ra)
	}
}
