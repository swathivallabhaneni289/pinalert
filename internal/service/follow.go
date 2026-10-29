package service

import (
	sqlcgen "pinalert/internal/store/sqlc"
)

// ViewerFollows reports whether the viewer follows reportID: they posted it,
// hold a vote row of any kind and value on it (confirm, dispute, resolve,
// reopen or unresolve), or have a comment on it. It is the same rule as plan
// 01's FollowedOpenReports query (posted through any session of the account, or
// any vote row of any kind, D-09, D-10), computed from rows the read already
// holds so the feed adds no round trip. isOwnReport is the caller's own
// posted-it answer, the expression IsOwnReport uses.
//
// Phase 8, PAGE-02: commenting makes the user follow the report. commented is
// the viewer_commented column of the one batched extras read, true only for a
// clear comment that is not deleted (review W8), the same third arm as
// FollowedOpenReports after plan 08-11 and FollowersWithChangesPush after plan
// 08-15.
//
// It is viewer specific and never says who else follows. A viewerAccountID of 0
// is no identified viewer and follows nothing, because accounts.id starts at 1.
func ViewerFollows(rows []sqlcgen.CurrentVotesForReportsRow, reportID, viewerAccountID int64, isOwnReport, commented bool) bool {
	if viewerAccountID == 0 {
		return false
	}
	if isOwnReport || commented {
		return true
	}
	for _, row := range rows {
		if row.ReportID == reportID && row.AccountID == viewerAccountID {
			return true
		}
	}
	return false
}
