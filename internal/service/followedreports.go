// followedreports.go is the read behind the follow note (plan 09-22, D-09): the
// listable reports of the people a reader has Notify me on for, through the same
// loadReadContext, classify and baseView as Nearby, Focus and the profile list.
// There is no second visibility rule here and no SQL predicate on moderation or
// view: a withheld report fails closed in classify and is never returned.

package service

import (
	"context"
	"errors"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// ErrFollowedReadUnavailable is returned when the service's querier cannot read
// followed candidates. The real querier always can.
var ErrFollowedReadUnavailable = errors.New("service: followed reports are not available on this querier")

// followedCandidateReader is the one read FollowedLiveReports adds. It is its
// own small interface, like profileCandidateReader, so the many fakes of Querier
// need no new method; the generated *sqlcgen.Queries satisfies it.
type followedCandidateReader interface {
	FollowedLiveCandidates(ctx context.Context, arg sqlcgen.FollowedLiveCandidatesParams) ([]sqlcgen.FollowedLiveCandidatesRow, error)
}

// FollowedLiveQuery is one read. Since maps each followed account to the time
// its bell was turned on; a report is kept only when it was created after its
// own reporter's time. An account that is not a key of Since is never read.
// Now is the instant to judge against (zero means the service clock).
type FollowedLiveQuery struct {
	ViewerAccountID int64
	Since           map[int64]time.Time
	Now             time.Time
}

// FollowedLiveReports returns the live reports (Active or Disputed view) of the
// accounts in q.Since that were created after each account's own time, oldest
// first, at most FollowedPostsCap of the newest ones. Whether a report may be
// announced is not decided here: the caller asks MayNotifyAboutReport with the
// view's visibility. The views are the ones Nearby builds, so the reporter's
// tag in Trust.Reporter is the rated tag the Show menu tick reads.
func (s *ReportService) FollowedLiveReports(ctx context.Context, q FollowedLiveQuery) ([]ReportView, error) {
	if len(q.Since) == 0 {
		return []ReportView{}, nil
	}
	reader, ok := s.q.(followedCandidateReader)
	if !ok {
		return nil, ErrFollowedReadUnavailable
	}
	now := q.Now
	if now.IsZero() {
		now = s.now()
	}
	now = now.UTC()

	accounts := make([]int64, 0, len(q.Since))
	var earliest time.Time
	for id, at := range q.Since {
		accounts = append(accounts, id)
		if earliest.IsZero() || at.Before(earliest) {
			earliest = at
		}
	}
	rows, err := reader.FollowedLiveCandidates(ctx, sqlcgen.FollowedLiveCandidatesParams{
		AccountIds: accounts,
		Now:        now,
		Since:      earliest,
		RowLimit:   FollowedPostsCap,
	})
	if err != nil {
		return nil, err
	}

	// Fail closed on the per reporter time: a row whose reporter is not in Since,
	// or that is not newer than that reporter's time, is dropped here.
	cands := make([]historyCandidate, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		since, ok := q.Since[row.ReporterAccountID]
		if !ok || !row.CreatedAt.After(since) {
			continue
		}
		r := reportFromNearbyRow(sqlcgen.NearbyReportsRow{
			ID:                    row.ID,
			Category:              row.Category,
			Severity:              row.Severity,
			Description:           row.Description,
			Latitude:              row.Latitude,
			Longitude:             row.Longitude,
			Geohash:               row.Geohash,
			ShelterCapacityStatus: row.ShelterCapacityStatus,
			ShelterHeadcount:      row.ShelterHeadcount,
			CreatedAt:             row.CreatedAt,
			ExpiresAt:             row.ExpiresAt,
		})
		// The candidate read measured no distance, and a note needs none.
		r.DistanceKm = nil
		cands = append(cands, historyCandidate{report: r, expired: !r.ExpiresAt.After(now)})
		ids = append(ids, r.ID)
	}
	out := []ReportView{}
	if len(cands) == 0 {
		return out, nil
	}

	rc, err := s.loadReadContext(ctx, ids, now, true, q.ViewerAccountID)
	if err != nil {
		return nil, err
	}
	// The candidate read is newest first; the answer is oldest first so the first
	// post is the first note when the ceiling stops the rest.
	for i := len(cands) - 1; i >= 0; i-- {
		c := cands[i]
		cl := classify(c, rc, now)
		switch cl.view {
		case FeedViewActive, FeedViewDisputed:
			out = append(out, baseView(viewInput{
				Report:          c.report,
				Class:           cl,
				Signals:         rc.signals,
				ViewerAccountID: q.ViewerAccountID,
				Now:             now,
			}))
		default:
			// FeedViewNone, Resolved, Past and anything a later view adds is never
			// returned: only a live report is news.
		}
	}
	return out, nil
}
