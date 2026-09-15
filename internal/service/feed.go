// feed.go holds the read-side shapes GET /api/reports returns: the view
// type, the listability predicate and the viewer's-own-vote lookup. Unlike
// visibility.go, this file deliberately DOES consume generated store
// types — it is the layer that turns 02-04's read path into the value the
// handler serialises, so this file's import set is NOT visibility.go's
// time-only parallelism seam and must not be held to that constraint.

package service

import (
	sqlcgen "pinalert/internal/store/sqlc"
)

// ReportView is what a read path returns; Report alone is what the write
// path returns, so a freshly submitted report is never handed a visibility
// nobody resolved. Visibility and VisibilityReason are Resolve's own answer
// and its own explanation, copied verbatim and never recomputed by a
// caller (TRUST-02, T-02-04). ViewerVote and IsOwnReport are
// viewer-specific and therefore make this type unsafe to cache across
// callers. ViewerVote is empty when the viewer has no standing content
// vote.
type ReportView struct {
	Report
	Visibility       Visibility
	VisibilityReason ResolveReason
	ViewerVote       VoteValue
	IsOwnReport      bool
}

// ListableInFeed reports whether a report with visibility v should appear
// in GET /api/reports's result given includeDisputed. Live and Provisional
// are always shown (D-08, D-09 — Provisional is dimmed and labelled
// client-side in 02-06, not filtered here). Hidden is shown only behind the
// "Show disputed" toggle, and shown on the map and in the list together
// because one endpoint serves both (D-10, D-11). Retracted is never shown
// in either view, because D-12 removes a resolved report from the live
// feed and map immediately and its only remaining surface is the Activity
// history 02-07 builds. "Show disputed" reveals disputed reports, not
// resolved ones — folding the two together is the likely future mistake
// this comment exists to prevent. Written as a switch with an explicit
// default so an unrecognised value fails closed rather than leaking into a
// feed.
func (v Visibility) ListableInFeed(includeDisputed bool) bool {
	switch v {
	case VisibilityLive, VisibilityProvisional:
		return false // TODO(RED): fill in during GREEN
	case VisibilityHidden:
		return false // TODO(RED): fill in during GREEN
	case VisibilityRetracted:
		return false
	default:
		return false
	}
}

// ViewerContentVote returns the viewer's own standing content vote
// (confirm or dispute) for reportID, or the empty VoteValue when there is
// none. It reads the SAME batched slice BuildVoteTally reads, so the vote a
// viewer sees echoed back and the visibility the report shows are computed
// from one read and cannot disagree. It is deliberately scoped to
// VoteKindContent, because the field answers "is this report still true"
// and not "did you try to close it" (D-02's two separately-tracked
// signals). CurrentVotesForReports is already DISTINCT ON (report_id,
// account_id, kind), so at most one content row per viewer per report
// exists and returning the first match is exact rather than a heuristic. A
// viewerAccountID of 0 matches nothing because accounts.id is a BIGSERIAL
// starting at 1, which is what makes 0 a safe "no identified viewer"
// sentinel.
func ViewerContentVote(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, viewerAccountID int64) VoteValue {
	return "" // TODO(RED): fill in during GREEN
}
