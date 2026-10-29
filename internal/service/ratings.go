// ratings.go is the service behind tapped star ratings of a reporter (Phase 9,
// plan 09-13, D-02, D-14, D-21, D-22, D-23, W1, W4). It decides who may rate,
// writes and clears a rating under the once a day rule, supplies the counted
// tallies to every reporter tag read as a SignalSource, and lets a moderator
// remove ratings and restore the follows they cost.
//
// Raters count by distinct voter cell of their qualifying vote, one counted
// rating per cell, the same independence rule confirmations follow (D-21): the
// counting lives in RatingTalliesForReporters, and the arithmetic lives in
// stars.go and reporter.go. Every call is keyed by the account id the verified
// gate passes in, never by a request value. Follows end only through
// endFollowsOf (review W1), softly, so a removal can bring them back.

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// RatingChangeGap is how long a rater waits before changing or removing a
// rating they wrote (UI-SPEC 3a): a first rating is free, any later change,
// including Remove my rating, needs this long since the rater's last change.
const RatingChangeGap = 24 * time.Hour

// ModeratorRatingsCap bounds the moderator's list of one reporter's ratings.
const ModeratorRatingsCap = 200

// ErrRatingNotAllowed is the one answer for a rating that cannot be written:
// yourself, a reporter you blocked, a rater who never voted on one of the
// reporter's reports, a rater a moderator removed, or a rating a moderator
// removed. It never says which (P9-C28), so it is not an oracle about who voted
// on whom.
var ErrRatingNotAllowed = errors.New("service: rating not allowed")

// ErrRatingTooSoon is returned when a rating is changed or removed again inside
// RatingChangeGap.
var ErrRatingTooSoon = errors.New("service: rating changed too recently")

// The three sentences of P9-C28.
const (
	RatingNotAllowedMessage = "You can't rate this person right now."
	RatingTooSoonMessage    = "You can change this rating again tomorrow."
	RatingStarsMessage      = "Choose 1 to 5 stars."
)

// RatingQuerier is the narrow store surface of RatingService: the rating
// queries, the person reads that decide visibility, and the two reads and the
// follow end a tag evaluation needs.
type RatingQuerier interface {
	RaterMayRate(ctx context.Context, arg sqlcgen.RaterMayRateParams) (bool, error)
	GetRating(ctx context.Context, arg sqlcgen.GetRatingParams) (sqlcgen.GetRatingRow, error)
	UpsertRating(ctx context.Context, arg sqlcgen.UpsertRatingParams) (int64, error)
	ClearRating(ctx context.Context, arg sqlcgen.ClearRatingParams) (int64, error)
	RatingTalliesForReporters(ctx context.Context, accountIds []int64) ([]sqlcgen.RatingTalliesForReportersRow, error)
	ModeratorRatingsForReporter(ctx context.Context, arg sqlcgen.ModeratorRatingsForReporterParams) ([]sqlcgen.ModeratorRatingsForReporterRow, error)
	RemoveRating(ctx context.Context, arg sqlcgen.RemoveRatingParams) (time.Time, error)
	RemoveRatingsByRater(ctx context.Context, arg sqlcgen.RemoveRatingsByRaterParams) ([]sqlcgen.RemoveRatingsByRaterRow, error)
	RemovedRatingsByRater(ctx context.Context, raterID int64) ([]sqlcgen.RemovedRatingsByRaterRow, error)
	MarkRaterRemoved(ctx context.Context, arg sqlcgen.MarkRaterRemovedParams) (int64, error)
	RestoreTagLostFollows(ctx context.Context, arg sqlcgen.RestoreTagLostFollowsParams) (int64, error)
	GetPersonByUsername(ctx context.Context, username string) (sqlcgen.GetPersonByUsernameRow, error)
	GetPersonByID(ctx context.Context, id int64) (sqlcgen.GetPersonByIDRow, error)
	GetAccountIDByUsername(ctx context.Context, username string) (int64, error)
	PersonRelation(ctx context.Context, arg sqlcgen.PersonRelationParams) (sqlcgen.PersonRelationRow, error)
	ReporterTagQuerier
	followEnder
}

// The generated Queries value satisfies the narrow querier.
var _ RatingQuerier = (*sqlcgen.Queries)(nil)

// RatingService applies the rating rules. It is the SignalSource every tag read
// plugs in, so a tapped rating reaches the feed, the vote response, Past and
// Focus, Activity, the dispatcher and the profile through the one seam of
// plan 09-03.
type RatingService struct {
	q   RatingQuerier
	now func() time.Time
}

// RatingOption configures a RatingService.
type RatingOption func(*RatingService)

// WithRatingClock replaces the clock, for tests.
func WithRatingClock(now func() time.Time) RatingOption {
	return func(s *RatingService) {
		if now != nil {
			s.now = now
		}
	}
}

// NewRatingService stores the querier. It never dereferences it, so a router
// built with a nil store still constructs.
func NewRatingService(q RatingQuerier, opts ...RatingOption) *RatingService {
	s := &RatingService{q: q, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

// The service is a SignalSource.
var _ SignalSource = (*RatingService)(nil)

// SignalOptions reads the counted tallies of the accounts and returns them as
// WithRatings (D-02, D-21, D-22). The tallies come back whether the reporter's
// profile is on or off, with the flag, because while it is off the ratings can
// only lower the points. No ids means no query and no options.
func (s *RatingService) SignalOptions(ctx context.Context, accountIDs []int64, now time.Time) ([]SignalOption, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	rows, err := s.q.RatingTalliesForReporters(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("service: SignalOptions: reading the rating tallies: %w", err)
	}
	tallies := make(map[int64]RatingTally, len(rows))
	for _, r := range rows {
		tallies[r.ReporterID] = RatingTally{Raters: int(r.Raters), TapSum: int(r.TapSum), ProfilePublic: r.ProfilePublic}
	}
	return []SignalOption{WithRatings(tallies)}, nil
}

// MayRate says whether the rater may rate the reporter now (D-23): not yourself;
// the reporter is opted in with a clear username and not deleted; no block in
// either direction; the rater was not removed by a moderator; and the rater cast
// a confirm, dispute, resolve or reopen vote on one of the reporter's reports,
// from any cell (a witness at the scene qualifies). An unresolve alone does not.
func (s *RatingService) MayRate(ctx context.Context, raterID, reporterID int64) (bool, error) {
	if raterID == reporterID {
		return false, nil
	}
	person, err := s.q.GetPersonByID(ctx, reporterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("service: MayRate: reading the reporter: %w", err)
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: raterID, OtherID: reporterID})
	if err != nil {
		return false, fmt.Errorf("service: MayRate: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) || rel.ViewerBlocked {
		return false, nil
	}
	ok, err := s.q.RaterMayRate(ctx, sqlcgen.RaterMayRateParams{RaterID: raterID, ReporterID: reporterID})
	if err != nil {
		return false, fmt.Errorf("service: MayRate: checking the votes: %w", err)
	}
	return ok, nil
}

// YourRating returns the rater's own live rating of the reporter and true, or
// 0 and false when there is none (never written, cleared, or removed by a
// moderator). It is the rater's own row only.
func (s *RatingService) YourRating(ctx context.Context, raterID, reporterID int64) (int, bool, error) {
	row, err := s.q.GetRating(ctx, sqlcgen.GetRatingParams{RaterID: raterID, ReporterID: reporterID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("service: YourRating: reading the rating: %w", err)
	}
	if row.ClearedAt.Valid || row.RemovedAt.Valid {
		return 0, false, nil
	}
	return int(row.Stars), true, nil
}

// Rate writes the rater's rating of the person with this username (D-02).
//
// Order of the answers: stars outside 1 to 5 is a ValidationError on stars with
// RatingStarsMessage; a malformed, unknown, private, pending or deleted person,
// or one who blocked the rater, is ErrPersonNotFound (the same answer as
// Profile); yourself, a person you blocked, a rater who has not voted on one of
// their reports, a rater a moderator removed, or a rating a moderator removed is
// ErrRatingNotAllowed; a different value inside RatingChangeGap of the last
// change is ErrRatingTooSoon. The same value again succeeds and changes
// nothing, including its time. After a write the reporter's tag is read again,
// and when it is no longer Reliable reporter every follow of the reporter ends
// softly with reason tag_lost (D-21, review W1).
func (s *RatingService) Rate(ctx context.Context, raterID int64, username string, stars int) error {
	if stars < 1 || stars > 5 {
		return ValidationError{Field: "stars", Message: RatingStarsMessage}
	}
	name, ok := canonicalUsername(username)
	if !ok {
		return ErrPersonNotFound
	}
	person, err := s.q.GetPersonByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPersonNotFound
		}
		return fmt.Errorf("service: Rate: reading the person: %w", err)
	}
	if person.ID == raterID {
		return ErrRatingNotAllowed
	}
	rel, err := s.q.PersonRelation(ctx, sqlcgen.PersonRelationParams{ViewerID: raterID, OtherID: person.ID})
	if err != nil {
		return fmt.Errorf("service: Rate: reading the relation: %w", err)
	}
	if !publicFace(person.Username, person.UsernameScreen, person.ProfilePublic, rel.BlockedViewer) {
		return ErrPersonNotFound
	}
	if rel.ViewerBlocked {
		return ErrRatingNotAllowed
	}
	eligible, err := s.q.RaterMayRate(ctx, sqlcgen.RaterMayRateParams{RaterID: raterID, ReporterID: person.ID})
	if err != nil {
		return fmt.Errorf("service: Rate: checking the votes: %w", err)
	}
	if !eligible {
		return ErrRatingNotAllowed
	}

	now := s.now().UTC()
	rows, err := s.q.UpsertRating(ctx, sqlcgen.UpsertRatingParams{
		RaterID:      raterID,
		ReporterID:   person.ID,
		Stars:        int16(stars),
		Now:          now,
		ChangeCutoff: now.Add(-RatingChangeGap),
	})
	if err != nil {
		return fmt.Errorf("service: Rate: writing the rating: %w", err)
	}
	if rows == 0 {
		cur, err := s.q.GetRating(ctx, sqlcgen.GetRatingParams{RaterID: raterID, ReporterID: person.ID})
		if err != nil {
			return fmt.Errorf("service: Rate: reading the refused rating: %w", err)
		}
		if cur.RemovedAt.Valid {
			return ErrRatingNotAllowed
		}
		return ErrRatingTooSoon
	}
	return s.reevaluate(ctx, person.ID, now)
}

// Clear removes the rater's own rating of the person with this username (Remove
// my rating). It works whatever the person's profile switch says, so a rater can
// always take a rating back; a well formed username that matches nobody, a
// rating that does not exist, one already cleared and one a moderator removed
// all succeed and change nothing, and only a malformed username is
// ErrPersonNotFound. It follows the same once a day rule as a change: inside
// RatingChangeGap of the last change it is ErrRatingTooSoon. The reporter's tag
// is read again afterwards, exactly as after Rate.
func (s *RatingService) Clear(ctx context.Context, raterID int64, username string) error {
	name, ok := canonicalUsername(username)
	if !ok {
		return ErrPersonNotFound
	}
	reporterID, err := s.q.GetAccountIDByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("service: Clear: resolving the username: %w", err)
	}

	now := s.now().UTC()
	rows, err := s.q.ClearRating(ctx, sqlcgen.ClearRatingParams{
		RaterID:      raterID,
		ReporterID:   reporterID,
		Now:          now,
		ChangeCutoff: now.Add(-RatingChangeGap),
	})
	if err != nil {
		return fmt.Errorf("service: Clear: clearing the rating: %w", err)
	}
	if rows == 0 {
		cur, err := s.q.GetRating(ctx, sqlcgen.GetRatingParams{RaterID: raterID, ReporterID: reporterID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("service: Clear: reading the refused clear: %w", err)
		}
		if cur.RemovedAt.Valid || cur.ClearedAt.Valid {
			return nil
		}
		return ErrRatingTooSoon
	}
	return s.reevaluate(ctx, reporterID, now)
}

// reporterTag reads the reporter's tag now, through the one loader with this
// service as the source, so the answer is the one every surface shows.
func (s *RatingService) reporterTag(ctx context.Context, reporterID int64, now time.Time) (ReporterTag, error) {
	signals, err := LoadReporterSignals(ctx, s.q, s, []int64{reporterID}, now)
	if err != nil {
		return "", fmt.Errorf("service: reading the tag: %w", err)
	}
	return signals[reporterID].Tag, nil
}

// reevaluate ends the reporter's follows softly with reason tag_lost when their
// tag is not Reliable reporter after a rating write (D-21, review W1).
func (s *RatingService) reevaluate(ctx context.Context, reporterID int64, now time.Time) error {
	tag, err := s.reporterTag(ctx, reporterID, now)
	if err != nil {
		return err
	}
	if tag == ReporterReliable {
		return nil
	}
	ended, err := endFollowsOf(ctx, s.q, reporterID, FollowEndTagLost, now)
	if err != nil {
		return fmt.Errorf("service: ending the follows of account %d after a rating: %w", reporterID, err)
	}
	logTagLost(reporterID, ended)
	return nil
}

// reevaluateAfterRemoval reads the reporter's tag after a moderator removed
// ratings. When it is Reliable reporter, the follows that ended with reason
// tag_lost at or after since (the time the earliest removed rating was first written, WR-S01)
// come back, up to each follower's following cap; otherwise the follows end, as
// after any rating write (D-21, review W1).
func (s *RatingService) reevaluateAfterRemoval(ctx context.Context, reporterID int64, since, now time.Time) error {
	tag, err := s.reporterTag(ctx, reporterID, now)
	if err != nil {
		return err
	}
	if tag != ReporterReliable {
		ended, err := endFollowsOf(ctx, s.q, reporterID, FollowEndTagLost, now)
		if err != nil {
			return fmt.Errorf("service: ending the follows of account %d after a removal: %w", reporterID, err)
		}
		logTagLost(reporterID, ended)
		return nil
	}
	restored, err := s.q.RestoreTagLostFollows(ctx, sqlcgen.RestoreTagLostFollowsParams{
		FolloweeID:   reporterID,
		Since:        since,
		FollowingCap: FollowingCap,
	})
	if err != nil {
		return fmt.Errorf("service: restoring the follows of account %d: %w", reporterID, err)
	}
	if restored > 0 {
		log.Printf("service: ratings: restored %d follows of account %d after a moderator removal", restored, reporterID)
	}
	return nil
}

// ModeratorRating is one row of the moderator's read of a reporter's ratings.
// It is the one place a rater's name is read; no public page carries it.
type ModeratorRating struct {
	RaterUsername string
	Stars         int
	UpdatedAt     time.Time
	Removed       bool
	Cleared       bool
}

// accountByUsername resolves a well formed username to a live account id,
// whether or not its profile is on, for the moderator reads and removals. A
// malformed or unknown username is ErrPersonNotFound.
func (s *RatingService) accountByUsername(ctx context.Context, username string) (int64, error) {
	name, ok := canonicalUsername(username)
	if !ok {
		return 0, ErrPersonNotFound
	}
	id, err := s.q.GetAccountIDByUsername(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrPersonNotFound
		}
		return 0, fmt.Errorf("service: resolving a username: %w", err)
	}
	return id, nil
}

// ModeratorList lists the ratings a reporter holds, newest first, at most
// ModeratorRatingsCap, with removed and cleared rows marked (D-14). The caller
// has already checked that the account is a moderator.
func (s *RatingService) ModeratorList(ctx context.Context, reporterUsername string) ([]ModeratorRating, error) {
	reporterID, err := s.accountByUsername(ctx, reporterUsername)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ModeratorRatingsForReporter(ctx, sqlcgen.ModeratorRatingsForReporterParams{
		ReporterID: reporterID,
		RowLimit:   ModeratorRatingsCap,
	})
	if err != nil {
		return nil, fmt.Errorf("service: ModeratorList: listing: %w", err)
	}
	out := make([]ModeratorRating, 0, len(rows))
	for _, r := range rows {
		out = append(out, ModeratorRating{
			RaterUsername: r.RaterUsername,
			Stars:         int(r.Stars),
			UpdatedAt:     r.UpdatedAt,
			Removed:       r.Removed,
			Cleared:       r.Cleared,
		})
	}
	return out, nil
}

// ModeratorRemove removes one rating (D-14, G2): the row stays with the
// moderator's id and the time, it stops counting, and the rater cannot write it
// back. The reporter's tag is then read again: when it is Reliable reporter the
// tag_lost follows ended since the rating was written come back, otherwise the
// follows end. Removing a rating that is not there, or is already removed,
// succeeds and changes nothing. The caller has already checked that the account
// is a moderator, and passes its id.
func (s *RatingService) ModeratorRemove(ctx context.Context, moderatorID int64, reporterUsername, raterUsername string) error {
	reporterID, err := s.accountByUsername(ctx, reporterUsername)
	if err != nil {
		return err
	}
	raterID, err := s.accountByUsername(ctx, raterUsername)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	since, err := s.q.RemoveRating(ctx, sqlcgen.RemoveRatingParams{
		Now: now, ModeratorID: moderatorID, RaterID: raterID, ReporterID: reporterID,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("service: ModeratorRemove: removing the rating: %w", err)
		}
		// Nothing left to remove. When the rating is already removed, an
		// earlier call may have failed after the removal committed, so the
		// re-evaluation runs again (WR-S02, review 2026-10-30). A missing or
		// live rating changes nothing.
		row, gerr := s.q.GetRating(ctx, sqlcgen.GetRatingParams{RaterID: raterID, ReporterID: reporterID})
		if gerr != nil {
			if errors.Is(gerr, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("service: ModeratorRemove: reading the rating: %w", gerr)
		}
		if !row.RemovedAt.Valid {
			return nil
		}
		return s.reevaluateAfterRemoval(ctx, reporterID, row.CreatedAt, now)
	}
	log.Printf("service: ratings: moderator %d removed a rating of account %d", moderatorID, reporterID)
	return s.reevaluateAfterRemoval(ctx, reporterID, since, now)
}

// ModeratorRemoveRater removes every rating one rater gave and marks the rater
// removed, so they can never rate again (D-14, G2, W4). The rater is marked
// first, so their ratings stop counting at once even if a later step fails. Each
// reporter they rated is then read again, ending or restoring follows as after
// ModeratorRemove. It returns how many ratings it removed. The caller has
// already checked that the account is a moderator, and passes its id.
func (s *RatingService) ModeratorRemoveRater(ctx context.Context, moderatorID int64, raterUsername string) (int64, error) {
	raterID, err := s.accountByUsername(ctx, raterUsername)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC()
	if _, err := s.q.MarkRaterRemoved(ctx, sqlcgen.MarkRaterRemovedParams{Now: now, ModeratorID: moderatorID, RaterID: raterID}); err != nil {
		return 0, fmt.Errorf("service: ModeratorRemoveRater: marking the rater: %w", err)
	}
	removed, err := s.q.RemoveRatingsByRater(ctx, sqlcgen.RemoveRatingsByRaterParams{Now: now, ModeratorID: moderatorID, RaterID: raterID})
	if err != nil {
		return 0, fmt.Errorf("service: ModeratorRemoveRater: removing the ratings: %w", err)
	}
	log.Printf("service: ratings: moderator %d removed %d ratings of one rater", moderatorID, len(removed))
	// Re-evaluate every reporter this rater's removed ratings touch, not only the
	// ones removed just now, so a repeat after a failed run finishes the work
	// (WR-S02). A failure on one reporter does not stop the others; the errors
	// come back joined.
	all, err := s.q.RemovedRatingsByRater(ctx, raterID)
	if err != nil {
		return int64(len(removed)), fmt.Errorf("service: ModeratorRemoveRater: reading the removed ratings: %w", err)
	}
	var errs []error
	for _, r := range all {
		if err := s.reevaluateAfterRemoval(ctx, r.ReporterID, r.CreatedAt, now); err != nil {
			errs = append(errs, err)
		}
	}
	return int64(len(removed)), errors.Join(errs...)
}
