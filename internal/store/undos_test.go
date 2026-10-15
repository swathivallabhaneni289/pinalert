package store_test

import (
	"context"
	"testing"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// TestResolutionUndoCounts pins the batched undo count of D-22 and D-28
// against a real Postgres: it counts the caller's own unresolve and reopen
// rows per report from the append only log, ignores other accounts and
// content rows, and returns no row for a report the account never touched.
func TestResolutionUndoCounts(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)

	reportA := seedReportForVotes(t, pool, "TestResolutionUndoCounts report A")
	reportB := seedReportForVotes(t, pool, "TestResolutionUndoCounts report B")
	reportC := seedReportForVotes(t, pool, "TestResolutionUndoCounts report C")
	account := seedAccountForVotes(t, pool, "undo-counts-me@example.com")
	other := seedAccountForVotes(t, pool, "undo-counts-other@example.com")

	insert := func(reportID, accountID int64, kind, value string) {
		t.Helper()
		if err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{
			ReportID:    reportID,
			AccountID:   accountID,
			Kind:        kind,
			Value:       value,
			GeohashCell: "tdr1qg0",
		}); err != nil {
			t.Fatalf("InsertVote %s/%s: %v", kind, value, err)
		}
	}

	// Report A: resolve, unresolve, resolve, unresolve, reopen by the account.
	insert(reportA, account, "resolution", "resolve")
	insert(reportA, account, "resolution", "unresolve")
	insert(reportA, account, "resolution", "resolve")
	insert(reportA, account, "resolution", "unresolve")
	insert(reportA, account, "resolution", "reopen")
	// Another account's unresolve on report A is not this account's budget.
	insert(reportA, other, "resolution", "unresolve")
	// Content rows are never counted, even with a value that matches.
	insert(reportA, account, "content", "unresolve")
	// Report B: a single resolve.
	insert(reportB, account, "resolution", "resolve")

	rows, err := q.ResolutionUndoCounts(ctx, sqlcgen.ResolutionUndoCountsParams{
		UnresolveValue: "unresolve",
		ReopenValue:    "reopen",
		AccountID:      account,
		Kind:           "resolution",
		ReportIds:      []int64{reportA, reportB, reportC},
	})
	if err != nil {
		t.Fatalf("ResolutionUndoCounts: %v", err)
	}

	got := map[int64]sqlcgen.ResolutionUndoCountsRow{}
	for _, r := range rows {
		got[r.ReportID] = r
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (report C has no rows of this account): %+v", len(rows), rows)
	}
	if a := got[reportA]; a.Unresolves != 2 || a.Reopens != 1 {
		t.Errorf("report A = %+v, want unresolves 2 and reopens 1", a)
	}
	if b, ok := got[reportB]; !ok || b.Unresolves != 0 || b.Reopens != 0 {
		t.Errorf("report B = %+v (present %v), want a row with unresolves 0 and reopens 0", b, ok)
	}
	if _, ok := got[reportC]; ok {
		t.Errorf("report C returned a row, want none")
	}

	// The other account sees only its own single unresolve on report A.
	otherRows, err := q.ResolutionUndoCounts(ctx, sqlcgen.ResolutionUndoCountsParams{
		UnresolveValue: "unresolve",
		ReopenValue:    "reopen",
		AccountID:      other,
		Kind:           "resolution",
		ReportIds:      []int64{reportA},
	})
	if err != nil {
		t.Fatalf("ResolutionUndoCounts other: %v", err)
	}
	if len(otherRows) != 1 || otherRows[0].Unresolves != 1 || otherRows[0].Reopens != 0 {
		t.Errorf("other account rows = %+v, want one row with unresolves 1 and reopens 0", otherRows)
	}
}
