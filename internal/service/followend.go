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

// followEnder is the slice of the generated queries that ends follows. It is
// declared here, in the consumer.
type followEnder interface {
	EndFollowsOf(ctx context.Context, arg sqlcgen.EndFollowsOfParams) (int64, error)
	EndFollowPair(ctx context.Context, arg sqlcgen.EndFollowPairParams) (int64, error)
}

var _ followEnder = (*sqlcgen.Queries)(nil)

// endFollowsOf ends every active follow of followeeID with reason at now and
// returns how many it ended. The rows stay (D-21), so the follows can be
// restored later.
//
// Every path that ends follows must call this or endFollowPair and never the
// queries directly (review W1): plan 09-26 adds the cleanup of queued phone
// notes here, once, instead of in each caller.
func endFollowsOf(ctx context.Context, q followEnder, followeeID int64, reason FollowEndReason, now time.Time) (int64, error) {
	return q.EndFollowsOf(ctx, sqlcgen.EndFollowsOfParams{
		Now:        now,
		Reason:     string(reason),
		FolloweeID: followeeID,
	})
}

// endFollowPair ends the one active follow of followeeID by followerID with
// reason at now and returns how many rows it ended (0 or 1). The row stays.
//
// Every path that ends follows must call this or endFollowsOf and never the
// queries directly (review W1); plan 09-26 adds the queued phone note cleanup
// here.
func endFollowPair(ctx context.Context, q followEnder, followerID, followeeID int64, reason FollowEndReason, now time.Time) (int64, error) {
	return q.EndFollowPair(ctx, sqlcgen.EndFollowPairParams{
		Now:        now,
		Reason:     string(reason),
		FollowerID: followerID,
		FolloweeID: followeeID,
	})
}
