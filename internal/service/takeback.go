// takeback.go holds the take back of a mistaken Mark resolved (HIST-09, D-22):
// the limit, the two refusals CastVote returns, and the small pure helpers the
// read paths and CastVote share. It touches no store method itself.

package service

import (
	"errors"

	sqlcgen "pinalert/internal/store/sqlc"
)

// UndoLimit is how many take backs one account gets per report. D-22 quotes
// the owner's words, "once or twice that's it", and the number is two. The
// poster's reopen spends the same budget (see UndosUsed), so a mistaken
// resolve cannot be flipped back and forth without end.
const UndoLimit = 2

// ErrNothingToTakeBack is returned when a take back arrives from an account
// whose standing resolution vote on the report is not resolve. It is refused
// before anything is written, so a withdraw with nothing to withdraw never
// burns budget or replaces a standing row. Plan 08 maps it to 409.
var ErrNothingToTakeBack = errors.New("service: no resolved mark to take back")

// ErrUndoLimitReached is returned when the account has already used both
// undos on this report. It covers the take back itself and the poster's
// reopen. Plan 08 maps it to 409.
var ErrUndoLimitReached = errors.New("service: both undos used on this report")

// ViewerResolution returns the viewer's own standing resolution vote on one
// report, or "" when there is none. It is the resolution twin of
// ViewerContentVote: CurrentVotesForReports is already DISTINCT ON (report_id,
// account_id, kind), so at most one resolution row per viewer per report
// exists and the first match is exact. A viewerAccountID of 0 matches nothing
// because accounts.id is a BIGSERIAL starting at 1. Only values valid for the
// resolution kind are returned, so a stray value never reaches a screen.
func ViewerResolution(rows []sqlcgen.CurrentVotesForReportsRow, reportID, viewerAccountID int64) VoteValue {
	if viewerAccountID == 0 {
		return ""
	}
	for _, row := range rows {
		if row.ReportID != reportID || row.AccountID != viewerAccountID {
			continue
		}
		if VoteKind(row.Kind) != VoteKindResolution {
			continue
		}
		if v := VoteValue(row.Value); v.ValidFor(VoteKindResolution) {
			return v
		}
	}
	return ""
}

// UndosUsed turns one ResolutionUndoCounts row into the budget spent. The
// unresolve rows of the account always count. The reopen rows count only for
// the poster, because only the poster's reopen is a take back of their own
// resolve; a non poster's reopen is an ordinary claim and never spends the
// budget (D-22, D-28). Whether the caller is the poster comes from
// ReportVoteContext against the verified account, never from the request.
func UndosUsed(row sqlcgen.ResolutionUndoCountsRow, isPoster bool) int {
	used := int(row.Unresolves)
	if isPoster {
		used += int(row.Reopens)
	}
	return used
}

// UndosLeft is what is left of the budget after used undos, never below 0.
func UndosLeft(used int) int {
	if used < 0 {
		used = 0
	}
	left := UndoLimit - used
	if left < 0 {
		return 0
	}
	return left
}
