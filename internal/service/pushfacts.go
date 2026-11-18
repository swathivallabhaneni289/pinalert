// pushfacts.go is the one place push code asks the resolver about a report. The
// dispatcher and, later, the sender both read a report through loadPushFacts, so
// a phone notification is always decided by the same resolver as the feed
// (TRUST-02, D-07). SQL never decides visibility: PushReportFacts only hands
// back the raw row, and nothing here is decided from the request that poked the
// dispatcher.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// PushFactsQuerier is the store surface loadPushFacts reads.
type PushFactsQuerier interface {
	PushReportFacts(ctx context.Context, reportID int64) (sqlcgen.PushReportFactsRow, error)
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
}

// pushReportFacts is one report as push code sees it: the raw row plus the
// resolver's answer for it at one instant. Lat and Lon are read by server code
// alone, for the distance, and never leave it (D-20). AreaName is the coarse
// name the reverse lookup stored, empty until it ran.
type pushReportFacts struct {
	ID                int64
	Meta              ReportMeta
	Happening         *Happening
	AreaName          string
	Lat, Lon          float64
	CreatedAt         time.Time
	ExpiresAt         time.Time
	ReporterAccountID *int64
	Vis               Visibility
	View              FeedView
	Confirmed         bool
	// Kept is true once a moderator kept the report after a review hide
	// (quick 261117-vvb), or restored it (quick 261118-rz6). Such a report is
	// never pushed again. A report kept from the Being checked mark is not
	// Kept: it alerts like any shown report (quick 261118-saj).
	Kept bool
	// FlagCheck is true while flags have the Being checked mark on the report
	// (quick 261118-saj, migration 00028). Nothing of any kind is queued or
	// sent for it until a moderator keeps or removes it.
	FlagCheck bool
}

// loadPushFacts reads one report fresh, builds its vote tally, asks the
// resolver for its visibility with the stored moderation state set, and derives
// the feed view and the Confirmed-by-people flag exactly as the feed does
// (ViewOf and ConfirmedByPeople over the same tally). ok is false, with no
// error, when the report does not exist.
//
// A disputed Critical report stays Live here because disputes cannot hide a
// Critical report (an earlier owner decision built into the resolver); only
// ConfirmedByPeople knows the dispute majority, so Confirmed is false for it
// and no second dispute rule exists in push code (review note N3).
func loadPushFacts(ctx context.Context, q PushFactsQuerier, reportID int64, now time.Time) (pushReportFacts, bool, error) {
	row, err := q.PushReportFacts(ctx, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pushReportFacts{}, false, nil
		}
		return pushReportFacts{}, false, fmt.Errorf("service: push facts: reading report %d: %w", reportID, err)
	}
	votes, err := q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		return pushReportFacts{}, false, fmt.Errorf("service: push facts: reading votes of report %d: %w", reportID, err)
	}
	tally := BuildVoteTally(votes, reportID, row.ReporterAccountID)

	meta := ReportMeta{Severity: Severity(row.Severity), Category: Category(row.Category), Moderation: ModerationState(row.ModerationState)}
	vis, _ := Resolve(meta, tally, now)
	view := ViewOf(vis, row.ExpiresAt, now)

	facts := pushReportFacts{
		ID:                row.ID,
		Meta:              meta,
		Lat:               row.Latitude,
		Lon:               row.Longitude,
		CreatedAt:         row.CreatedAt,
		ExpiresAt:         row.ExpiresAt,
		ReporterAccountID: row.ReporterAccountID,
		Vis:               vis,
		View:              view,
		Confirmed:         ConfirmedByPeople(vis, view, tally),
		// One settled marker, review_kept_at, for the hide and the mark: a
		// settled report with flag_check_at set was kept from the mark and
		// alerts again; one without it stays silent. The mark is on while
		// flag_check_at is set and the report is not settled (migration 00028).
		Kept:      row.ReviewKeptAt.Valid && !row.FlagCheckAt.Valid,
		FlagCheck: row.FlagCheckAt.Valid && !row.ReviewKeptAt.Valid,
	}
	if row.Happening != nil {
		h := Happening(*row.Happening)
		facts.Happening = &h
	}
	if row.AreaName != nil {
		facts.AreaName = *row.AreaName
	}
	return facts, true, nil
}
