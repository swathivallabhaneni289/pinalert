package service

import (
	"context"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// FollowEndReason says why a follow ended (D-21). The reason is stored in
// account_follows.end_reason, and the database refuses any other value.
type FollowEndReason string

const (
	// FollowEndUnfollowed: the follower unfollowed.
	FollowEndUnfollowed FollowEndReason = "unfollowed"
	// FollowEndRemoved: the followee used Remove follower or Remove all followers.
	FollowEndRemoved FollowEndReason = "removed"
	// FollowEndBlocked: one of the two blocked the other.
	FollowEndBlocked FollowEndReason = "blocked"
	// FollowEndProfileOff: the followee switched the profile off, went pending or
	// was found not followable.
	FollowEndProfileOff FollowEndReason = "profile_off"
	// FollowEndTagLost: the computed tag fell below Reliable reporter. These are
	// the only follows ever restored, after a moderator removes ratings.
	FollowEndTagLost FollowEndReason = "tag_lost"
)

// followEnder is the slice of the generated queries that ends follows and
// deletes the phone notes still waiting for them. It is declared here, in the
// consumer. Plan 09-26 added the two delete methods, so every querier that is
// handed to the helpers below (PeopleQuerier, FollowSweepQuerier, RatingQuerier)
// must carry them.
type followEnder interface {
	EndFollowsOf(ctx context.Context, arg sqlcgen.EndFollowsOfParams) (int64, error)
	EndFollowPair(ctx context.Context, arg sqlcgen.EndFollowPairParams) (int64, error)
	DeleteQueuedFollowPushesForFollowee(ctx context.Context, followeeID int64) (int64, error)
	DeleteQueuedFollowPush(ctx context.Context, arg sqlcgen.DeleteQueuedFollowPushParams) (int64, error)
}

var _ followEnder = (*sqlcgen.Queries)(nil)

// endFollowsOf ends every active follow of followeeID with reason at now and
// returns how many it ended. The rows stay (D-21), so the follows can be
// restored later. Right after the end it deletes every follow push still
// waiting in the outbox about a report the followee posted, for all followers.
//
// Every path that ends follows must call this or endFollowPair and never the
// queries directly (review W1): this is the one place the queued note cleanup
// lives (plan 09-26, SC5 pending alerts, research Q9, D-24). A note already sent
// and the bell rows stay, as history; a row a worker holds is not touched, and
// that worker checks the follow again at send time. The delete runs even when
// no follow was active, because a note can be left behind by one that ended
// earlier and the call is cheap.
func endFollowsOf(ctx context.Context, q followEnder, followeeID int64, reason FollowEndReason, now time.Time) (int64, error) {
	n, err := q.EndFollowsOf(ctx, sqlcgen.EndFollowsOfParams{
		Now:        now,
		Reason:     string(reason),
		FolloweeID: followeeID,
	})
	if err != nil {
		return n, err
	}
	if _, err := q.DeleteQueuedFollowPushesForFollowee(ctx, followeeID); err != nil {
		return n, err
	}
	return n, nil
}

// endFollowPair ends the one active follow of followeeID by followerID with
// reason at now and returns how many rows it ended (0 or 1). The row stays.
// Right after the end it deletes the follow pushes still waiting for this
// follower about reports of this followee.
//
// Every path that ends follows must call this or endFollowsOf and never the
// queries directly (review W1); the queued phone note cleanup lives here
// (plan 09-26, SC5, research Q9, D-24). Block ends both of its follows through
// this helper, so it is covered too.
func endFollowPair(ctx context.Context, q followEnder, followerID, followeeID int64, reason FollowEndReason, now time.Time) (int64, error) {
	n, err := q.EndFollowPair(ctx, sqlcgen.EndFollowPairParams{
		Now:        now,
		Reason:     string(reason),
		FollowerID: followerID,
		FolloweeID: followeeID,
	})
	if err != nil {
		return n, err
	}
	if _, err := q.DeleteQueuedFollowPush(ctx, sqlcgen.DeleteQueuedFollowPushParams{FollowerID: followerID, FolloweeID: followeeID}); err != nil {
		return n, err
	}
	return n, nil
}
