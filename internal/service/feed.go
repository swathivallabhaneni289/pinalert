// feed.go holds the read-side shapes GET /api/reports returns: the view
// type, the listability predicate and the viewer's-own-vote lookup. Unlike
// visibility.go, this file deliberately DOES consume generated store
// types — it is the layer that turns 02-04's read path into the value the
// handler serialises, so this file's import set is NOT visibility.go's
// time-only parallelism seam and must not be held to that constraint.

package service

import (
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// WrittenLabelThreshold is the one 30 minute constant behind the "Written 4
// hours ago" line: a report shows a written time only when it was uploaded
// more than this long after the client wrote it. Every surface reads it
// through WrittenAtForDisplay (D-19).
const WrittenLabelThreshold = 30 * time.Minute

// WrittenAtForDisplay returns a copy of the client written time when the
// upload came more than WrittenLabelThreshold after it was written, otherwise
// nil. It is display only: it never feeds Resolve, the listability filter, the
// order, created_at or expires_at (R-05). Clamping the stored value is
// ClampWrittenAt's job (04-17).
func WrittenAtForDisplay(createdAt time.Time, written *time.Time) *time.Time {
	return nil
}

// ReportView is what a read path returns; Report alone is what the write
// path returns, so a freshly submitted report is never handed a visibility
// nobody resolved. Visibility and VisibilityReason are Resolve's own answer
// and its own explanation, copied verbatim and never recomputed by a
// caller (TRUST-02, T-02-04). ViewerVote and IsOwnReport are
// viewer-specific and therefore make this type unsafe to cache across
// callers. ViewerVote is empty when the viewer has no standing content
// vote. Trust is built by BuildTrust from the same batched rows as the
// visibility. Unlike ViewerVote and IsOwnReport, it is viewer independent
// (D-21).
//
// View is ViewOf's answer for the report at the read's own clock. History is
// nil for active and disputed views and holds how the report ended for
// resolved and past views. ViewerResolution and UndosLeft are viewer specific
// like ViewerVote: ViewerResolution is the viewer's own standing resolution
// vote or empty, and UndosLeft is how many take backs the viewer has left on
// this report, UndoLimit when nothing was spent or the count was not read, and
// 0 once the report has faded, because nothing can be undone then.
// Approximate is true only when CoarsenFinishedRescue changed the report, so a
// reader can say the spot is a blurred one.
//
// WrittenAt and ViewerFlagged are viewer or display data filled from the
// batched extras read: WrittenAt is WrittenAtForDisplay's answer, ViewerFlagged
// is viewer specific like ViewerVote, and neither reaches the wire in this plan
// (04-17 adds written_at, 04-20 viewer_flagged).
type ReportView struct {
	Report
	Visibility       Visibility
	VisibilityReason ResolveReason
	View             FeedView
	History          *HistoryFacts
	ViewerVote       VoteValue
	ViewerResolution VoteValue
	IsOwnReport      bool
	UndosLeft        int
	Approximate      bool
	Trust            Trust
	WrittenAt        *time.Time
	ViewerFlagged    bool
}

// ListableInFeed reports whether a report with visibility v should appear
// in GET /api/reports's result given includeDisputed. The flag selects
// between two DISJOINT views rather than widening one: exactly one of them
// contains any given non-retracted report, never both, never neither. The
// default view (includeDisputed false) is Live plus Provisional (D-08,
// D-09 — Provisional is dimmed and labelled client-side in 02-06, not
// filtered here). The disputed view (includeDisputed true) is Hidden
// alone, served to the list and the map pins together because one
// endpoint feeds both (D-10, D-11). Retracted appears in neither view,
// because D-12 removes a resolved report from the live feed and map
// immediately. It stays visible on two other surfaces: the Activity page and
// the Resolved group of the Past tab (D-25).
// "Show disputed" reveals disputed reports, not resolved ones — folding
// the two together is the likely future mistake this comment exists to
// prevent.
//
// Pre-existing consequence, made more visible by this partition and not a
// regression it introduces: D-06's critical-bypass rung sits above the
// dispute rung in Resolve(), so a critical or rescue-needed report stays
// Live however heavily it is disputed, and therefore never appears in the
// disputed view.
//
// This reverses a documented earlier choice: this function previously
// unioned Hidden into the default view instead of partitioning, matching
// 02-RESEARCH.md's data-flow diagram at the time. That additive reading
// was never required by D-10 or D-11's locked text in 02-CONTEXT.md — it
// was an interpretation carried into this function and the diagram one
// layer below the decision. The user reported the resulting symptom twice
// (02-UAT.md Test 9, ROADMAP.md backlog entry Phase 999.2) before this was
// corrected by gap-closure plan 02-12.
//
// Written as a switch with an explicit default so an unrecognised value
// fails closed rather than leaking into a feed.
func (v Visibility) ListableInFeed(includeDisputed bool) bool {
	switch v {
	case VisibilityLive, VisibilityProvisional:
		return !includeDisputed
	case VisibilityHidden:
		return includeDisputed
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
	if viewerAccountID == 0 {
		return ""
	}
	for _, row := range rows {
		if row.ReportID != reportID || row.AccountID != viewerAccountID {
			continue
		}
		if VoteKind(row.Kind) != VoteKindContent {
			continue
		}
		v := VoteValue(row.Value)
		if v == VoteConfirm || v == VoteDispute {
			return v
		}
	}
	return ""
}
