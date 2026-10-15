package service_test

import (
	"testing"

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
