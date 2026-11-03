// updatesfollow.go is the follow note step of Refresh (plan 09-22, PROF-03,
// D-09, D-12): when someone the reader follows with Notify me on posts a report,
// the reader's bell gets one note about it. It is the sibling of the Reliable
// reporter step in updatesalerts.go and writes only through the one predicate
// and the one insert, so a hidden, withheld, retracted or expired report is
// never announced.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// FollowedPostsCap is the most candidate reports one Refresh reads for the
// follow note (D-09, T-09-78): the newest ones, then oldest first. With
// NotifyFollowsCap it bounds the step on every 30 second poll.
const FollowedPostsCap = 200

// followedLiveReader is the one method of ReportService the follow note step
// needs. UpdatesService reaches it by asserting it on its reports reader, the
// way ProfileReports reaches its read on the querier, so NearbyReader and its
// fakes stay as they are; a reader without it ends the step quietly.
type followedLiveReader interface {
	FollowedLiveReports(ctx context.Context, q FollowedLiveQuery) ([]ReportView, error)
}

// followStep is what the follow note step hands to the Reliable reporter step
// that follows it (review WR-02, quick 261103-jal). The follow note and the
// Reliable row share one first alert slot per report, and the follow phone note
// is queued only from a written follow row, so the Reliable step must never take
// the slot of a report the follow step is responsible for.
type followStep struct {
	// notifying is each person the reader follows with Notify me on, whom the
	// step can write for, with the time that bell went on. The Reliable step
	// skips their reports posted after that time.
	notifying map[int64]time.Time
	// truncated is true when the read hit FollowedPostsCap, so some report of
	// theirs may not have been seen this time.
	truncated bool
}

// refreshFollowedPosts writes the follow notes of one account and returns whom
// it covers, so the Reliable reporter step that follows can leave those people's
// reports to it (review W3, WR-02). It does nothing, with no
// error, when the reader has no Notify me follow or the service has no reports
// reader of the right kind. It never stops the other updates: Refresh has
// already written those (D-28).
//
// The order is part of the rule:
//
//  1. Read the account's Notify me follows (active follows only), at most
//     NotifyFollowsCap, with the time each bell was turned on.
//  2. Drop the people the reader muted (D-12) and the people who are not
//     followable now: private, a pending username or deleted (D-01).
//  3. Read their listable live reports through the shared pipeline, only those
//     posted after the bell was turned on, at most FollowedPostsCap.
//  4. Oldest report first, skip the reader's own, skip unless the one predicate
//     allows an announcement, and skip unless the reporter still reads Reliable
//     reporter now (the tag the trust object carries, ratings included, D-12).
//  5. Write the note. pgx.ErrNoRows is the normal answer when the account
//     already holds a first alert for the report or is over the hourly ceiling;
//     the report stays eligible for a later poll while it lasts.
//
// A note counts toward the hourly ceiling and seeds the reader's cursor in the
// same statement, exactly as the Reliable reporter note does.
func (s *UpdatesService) refreshFollowedPosts(ctx context.Context, accountID int64, now time.Time) (followStep, error) {
	reader, ok := s.reports.(followedLiveReader)
	if !ok {
		return followStep{}, nil
	}

	follows, err := s.q.NotifyFollowsFor(ctx, sqlcgen.NotifyFollowsForParams{FollowerID: accountID, RowLimit: NotifyFollowsCap})
	if err != nil {
		return followStep{}, fmt.Errorf("reading Notify me follows: %w", err)
	}
	if len(follows) == 0 {
		return followStep{}, nil
	}

	mutedIDs, err := s.q.MutedPeopleIDs(ctx, accountID)
	if err != nil {
		return followStep{}, fmt.Errorf("reading muted people: %w", err)
	}
	muted := make(map[int64]bool, len(mutedIDs))
	for _, id := range mutedIDs {
		muted[id] = true
	}

	ids := make([]int64, 0, len(follows))
	for _, f := range follows {
		if !muted[f.FolloweeID] {
			ids = append(ids, f.FolloweeID)
		}
	}
	if len(ids) == 0 {
		return followStep{}, nil
	}
	standing, err := s.q.FolloweeStanding(ctx, ids)
	if err != nil {
		return followStep{}, fmt.Errorf("reading followee standing: %w", err)
	}
	followable := make(map[int64]bool, len(standing))
	for _, row := range standing {
		followable[row.ID] = row.Followable
	}
	since := make(map[int64]time.Time, len(follows))
	for _, f := range follows {
		if muted[f.FolloweeID] || !followable[f.FolloweeID] {
			continue
		}
		since[f.FolloweeID] = f.NotifySince
	}
	if len(since) == 0 {
		return followStep{}, nil
	}

	views, err := reader.FollowedLiveReports(ctx, FollowedLiveQuery{ViewerAccountID: accountID, Since: since, Now: now})
	if err != nil {
		return followStep{}, fmt.Errorf("reading followed reports: %w", err)
	}

	windowStart := now.Add(-updatesCeilingWindow)
	for _, v := range views {
		if v.IsOwnReport {
			continue
		}
		if _, ok := since[v.ReporterAccount()]; !ok {
			continue
		}
		if !MayNotifyAboutReport(UpdateFollowedPosted, v.Visibility, v.ExpiresAt, now) {
			continue
		}
		if v.Trust.Reporter.Tag != ReporterReliable {
			continue
		}
		_, err := s.q.InsertFollowedPostedAlert(ctx, sqlcgen.InsertFollowedPostedAlertParams{
			AccountID: accountID,
			ReportID:  v.ID,
			Now:       now,
			HourEdge:  windowStart,
			Ceiling:   FollowHourlyCeiling,
			State:     string(v.Visibility),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return followStep{notifying: since}, fmt.Errorf("writing follow note for report %d: %w", v.ID, err)
		}
	}
	return followStep{notifying: since, truncated: len(views) >= FollowedPostsCap}, nil
}
