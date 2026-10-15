// history.go holds the pure functions every report history surface reads
// (plan 03.1-01): the one view classifier ViewOf, the closure replay
// ClosureOf and the history facts BuildHistoryFacts. Everything here takes
// values and returns values. There is no context, no querier and no I/O, so
// the history read composes these functions and Phase 4's withheld check can
// loop over them without a database.

package service

import (
	"sort"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// FeedView is the one closed partition of the report history tabs (D-04). The
// type is named FeedView so a grep for a View string type finds it.
type FeedView string

const (
	// FeedViewNone is the fail closed answer for a visibility this package
	// does not know. It is never a tab and never a wire value.
	FeedViewNone FeedView = ""
	// FeedViewActive holds Live and Provisional reports that have not expired.
	FeedViewActive FeedView = "active"
	// FeedViewDisputed holds Hidden reports that have not expired.
	FeedViewDisputed FeedView = "disputed"
	// FeedViewResolved holds Retracted reports, expired or not.
	FeedViewResolved FeedView = "resolved"
	// FeedViewPast holds expired reports that were not Retracted.
	FeedViewPast FeedView = "past"
)

// FeedViews is the ordered, canonical list of the four tab words.
var FeedViews = []FeedView{FeedViewActive, FeedViewDisputed, FeedViewResolved, FeedViewPast}

// Valid reports whether v is one of the four tab words. Comparison is exact,
// with no case folding.
func (v FeedView) Valid() bool {
	for _, x := range FeedViews {
		if x == v {
			return true
		}
	}
	return false
}

const (
	// HistoryWindow is how far back the Resolved and Past tabs reach (D-08,
	// fixed at 7 days).
	HistoryWindow = 7 * 24 * time.Hour
	// RecentWindow is the "last 24 hours" cut of a history group (D-12).
	RecentWindow = 24 * time.Hour
	// HistoryGroupCap is how many rows one history group shows before the
	// show more control (UI-SPEC Surface 3).
	HistoryGroupCap = 25
	// HistoryExpiredCandidateCap bounds the expired candidates one history read
	// loads, so live reports cannot fill the Past read (RESEARCH A3).
	HistoryExpiredCandidateCap = 500
	// HistoryOpenCandidateCap bounds the open candidates one history read
	// loads, for the same reason (RESEARCH A3).
	HistoryOpenCandidateCap = 1000
)

// ViewOf maps a visibility, an expiry and a clock to exactly one tab (D-03,
// D-04, D-25). It is the only place this partition is written down.
//
//   - Retracted is resolved at any expiry, so a report closed and then expired
//     stays in Resolved.
//   - Live, Provisional and Hidden are past once expiresAt is not after now,
//     which matches CastVote step 5 (equal to now counts as expired).
//   - Otherwise Hidden is disputed and Live and Provisional are active.
//
// An unknown visibility returns FeedViewNone, so a state added later can never
// leak into a tab by default. ViewOf knows nothing about moderation; Phase 4
// wraps it with its own withheld predicate.
func ViewOf(v Visibility, expiresAt, now time.Time) FeedView {
	switch v {
	case VisibilityRetracted:
		return FeedViewResolved
	case VisibilityLive, VisibilityProvisional, VisibilityHidden:
		if !expiresAt.After(now) {
			return FeedViewPast
		}
		if v == VisibilityHidden {
			return FeedViewDisputed
		}
		return FeedViewActive
	default:
		return FeedViewNone
	}
}

// Closure says when the current retraction of a report began and who closed
// it. The zero Closure means the report is not retracted.
type Closure struct {
	// ByReporter is true when the poster's own standing resolution is resolve.
	ByReporter bool
	// ByPeople is true when people nearby stand at the independence
	// threshold of resolve cells.
	ByPeople bool
	// At is when the current retraction began.
	At time.Time
}

// ClosureOf replays the standing resolution rows of one report and returns
// when its current retraction began and who closed it (D-09).
//
// Prefix replay is used because the closure is the moment the second distinct
// cell stood, or the poster resolved, whichever made the report Retracted. At
// each step it asks the tally for isRetracted, the same predicate Resolve
// uses, so the closure can never disagree with the live state. Nothing
// latches (02-CONTEXT D-08): when a retraction is lifted and a later row
// closes the report again, the closure time is that later row, because the
// last false to true flip is the start of the current retraction.
//
// The rows are standing rows, one per account, as CurrentVotesForReports
// returns them. Rows of other reports and unknown kinds or values are ignored
// by BuildVoteTally.
func ClosureOf(rows []sqlcgen.CurrentVotesForReportsRow, reportID int64, reporterAccountID *int64) Closure {
	var content, resolution []sqlcgen.CurrentVotesForReportsRow
	for _, r := range rows {
		if r.ReportID != reportID {
			continue
		}
		switch VoteKind(r.Kind) {
		case VoteKindContent:
			content = append(content, r)
		case VoteKindResolution:
			resolution = append(resolution, r)
		}
	}
	sort.SliceStable(resolution, func(i, j int) bool {
		if !resolution[i].CreatedAt.Equal(resolution[j].CreatedAt) {
			return resolution[i].CreatedAt.Before(resolution[j].CreatedAt)
		}
		return resolution[i].AccountID < resolution[j].AccountID
	})

	var at time.Time
	wasRetracted := false
	var full VoteTally
	for i := 1; i <= len(resolution); i++ {
		prefix := make([]sqlcgen.CurrentVotesForReportsRow, 0, len(content)+i)
		prefix = append(prefix, content...)
		prefix = append(prefix, resolution[:i]...)
		tally := BuildVoteTally(prefix, reportID, reporterAccountID)
		retracted := tally.isRetracted()
		if retracted && !wasRetracted {
			at = resolution[i-1].CreatedAt
		}
		wasRetracted = retracted
		full = tally
	}
	if !wasRetracted {
		return Closure{}
	}
	return Closure{
		ByReporter: full.ReporterResolved,
		ByPeople:   full.ResolveCells >= IndependentAgreementThreshold,
		At:         at,
	}
}

const (
	// HistoryStateResolved is the State of a report that was closed.
	HistoryStateResolved = "resolved"
	// HistoryStateExpired is the State of a report that ran out its time.
	HistoryStateExpired = "expired"
)

// HistoryFacts is what a history row says about how a report ended (D-08,
// D-09). For a resolved report ClosedBy and ClosedText are set and the three
// expired fields are empty; for a past report Outcome, OutcomeLabel and
// LifetimeText are set and the two resolved fields are empty.
type HistoryFacts struct {
	State        string
	At           time.Time
	ClosedBy     ClosedBy
	ClosedText   string
	Outcome      Outcome
	OutcomeLabel string
	LifetimeText string
}

// BuildHistoryFacts composes the history facts of one report from values the
// caller already has: the view from ViewOf, the Closure from ClosureOf, the
// outcome from ClassifyOutcome and the confirmed count from the tally. It
// returns nil for a view that has no history line (active, disputed and none).
// This file never calls a store.
func BuildHistoryFacts(view FeedView, c Closure, viewerIsReporter bool, outcome Outcome, confirmed int, createdAt, expiresAt time.Time) *HistoryFacts {
	switch view {
	case FeedViewResolved:
		by := ClosedByFor(c, viewerIsReporter)
		return &HistoryFacts{
			State:      HistoryStateResolved,
			At:         c.At,
			ClosedBy:   by,
			ClosedText: closedText(by),
		}
	case FeedViewPast:
		return &HistoryFacts{
			State:        HistoryStateExpired,
			At:           expiresAt,
			Outcome:      outcome,
			OutcomeLabel: outcomeLabelText(outcome, confirmed),
			LifetimeText: lifetimeText(createdAt, expiresAt),
		}
	default:
		return nil
	}
}

// ClosedBy names who closed a report, from the point of view of the viewer.
type ClosedBy string

const (
	ClosedByYou             ClosedBy = "you"
	ClosedByPoster          ClosedBy = "poster"
	ClosedByPeople          ClosedBy = "people"
	ClosedByPosterAndPeople ClosedBy = "poster_and_people"
	ClosedByYouAndPeople    ClosedBy = "you_and_people"
)

// ClosedByFor says who closed a report. The poster is "you" to the poster and
// "the poster" to everyone else. The zero Closure gives the empty ClosedBy.
func ClosedByFor(c Closure, viewerIsReporter bool) ClosedBy {
	switch {
	case c.ByReporter && c.ByPeople:
		if viewerIsReporter {
			return ClosedByYouAndPeople
		}
		return ClosedByPosterAndPeople
	case c.ByReporter:
		if viewerIsReporter {
			return ClosedByYou
		}
		return ClosedByPoster
	case c.ByPeople:
		return ClosedByPeople
	default:
		return ""
	}
}
