package service

import (
	sqlcgen "pinalert/internal/store/sqlc"
)

// ViewerFollows reports whether the viewer follows reportID: they posted it, or
// hold a vote row of any kind and value on it (confirm, dispute, resolve,
// reopen or unresolve). It is the same rule as plan 01's FollowedOpenReports
// query (posted through any session of the account, or any vote row of any
// kind, D-09, D-10), computed from rows the read already holds so the feed adds
// no round trip. isOwnReport is the caller's own posted-it answer, the
// expression IsOwnReport uses.
//
// It is viewer specific and never says who else follows. A viewerAccountID of 0
// is no identified viewer and follows nothing, because accounts.id starts at 1.
func ViewerFollows(rows []sqlcgen.CurrentVotesForReportsRow, reportID, viewerAccountID int64, isOwnReport bool) bool {
	if viewerAccountID == 0 {
		return false
	}
	if isOwnReport {
		return true
	}
	for _, row := range rows {
		if row.ReportID == reportID && row.AccountID == viewerAccountID {
			return true
		}
	}
	return false
}
