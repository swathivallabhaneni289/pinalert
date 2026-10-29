// followsweep.go ends the stored follows of a person who is no longer
// followable (Phase 9, plan 09-06, SC5, D-12, D-21, D-24).
//
// The Reliable reporter tag is never stored (Phase 3 D-14): it is computed on
// every read from the judged history, so it can fall away by time alone, with
// no event to hook. Nothing fires when it does. The sweep below, the Reliable
// check at follow time and evaluate on touch in the profile read (plan 09-04)
// together end the stored follows softly within one interval; the produce and
// send time rechecks of the later plans stop follow notes at once (D-24). Later
// plans of this phase add queued follow notes to what a sweep clears, inside
// endFollowsOf.

package service

import (
	"context"
	"log"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// FollowSweepInterval is the pause between follow sweeps. A sweep also runs
// once when the server starts, because the free host cold starts.
const FollowSweepInterval = 5 * time.Minute

// FollowSweepQuerier is the slice of the generated queries the sweep uses: the
// two tag reads, the page of followees, their standing and the ending of
// follows.
type FollowSweepQuerier interface {
	ReporterTagQuerier
	FolloweeIDsPage(ctx context.Context, arg sqlcgen.FolloweeIDsPageParams) ([]int64, error)
	FolloweeStanding(ctx context.Context, accountIds []int64) ([]sqlcgen.FolloweeStandingRow, error)
	// BlockedActiveFollowPairs finds an active follow beside a block, which a
	// Follow racing a Block can leave (review IN-S01 and IN-E01).
	BlockedActiveFollowPairs(ctx context.Context, rowLimit int32) ([]sqlcgen.BlockedActiveFollowPairsRow, error)
	followEnder
}

var _ FollowSweepQuerier = (*sqlcgen.Queries)(nil)

// FollowSweep ends the follows of people who stopped being followable.
type FollowSweep struct {
	q       FollowSweepQuerier
	now     func() time.Time
	signals SignalSource
}

// FollowSweepOption configures a FollowSweep.
type FollowSweepOption func(*FollowSweep)

// WithFollowSweepClock replaces the clock, so a test can move time.
func WithFollowSweepClock(now func() time.Time) FollowSweepOption {
	return func(s *FollowSweep) {
		if now != nil {
			s.now = now
		}
	}
}

// WithFollowSweepSignals sets the signal source the tag read uses, the same
// SignalSource every other tag reader gets (plan 09-14 wires it).
func WithFollowSweepSignals(src SignalSource) FollowSweepOption {
	return func(s *FollowSweep) { s.signals = src }
}

// NewFollowSweep builds the sweep over the queries.
func NewFollowSweep(q FollowSweepQuerier, opts ...FollowSweepOption) *FollowSweep {
	s := &FollowSweep{q: q, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run sweeps once at start, then on every tick of FollowSweepInterval until ctx
// is done. It returns soon after the context is cancelled.
func (s *FollowSweep) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.sweepAndLog(ctx)

	ticker := time.NewTicker(FollowSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepAndLog(ctx)
		}
	}
}

func (s *FollowSweep) sweepAndLog(ctx context.Context) {
	ended, err := s.SweepOnce(ctx)
	if err != nil && ctx.Err() == nil {
		log.Printf("follow sweep: %v", err)
	}
	if ended > 0 {
		log.Printf("follow sweep: ended %d follows", ended)
	}
}

// SweepOnce pages through every person who has an active follower, in batches
// of MaxReporterAccountsPerRead, and ends the follows of each one who is not
// followable (reason profile_off: private, a pending username or deleted) or
// whose computed tag is not Reliable reporter (reason tag_lost). It returns the
// number of follow rows ended. A failure on one page is logged with the page
// bounds and the sweep moves on to the next page; only a failure to read the
// next page of followees stops it. Logs carry ids and counts only.
func (s *FollowSweep) SweepOnce(ctx context.Context) (ended int64, err error) {
	// First the follows that sit beside a block (review IN-S01 and IN-E01,
	// 2026-10-30). A failure is logged and the sweep goes on to the tag pages.
	if n, blockErr := s.endBlockedPairs(ctx); blockErr != nil {
		if ctx.Err() == nil {
			log.Printf("follow sweep: blocked pairs: %v", blockErr)
		}
	} else {
		ended += n
	}
	var afterID int64
	for {
		if err := ctx.Err(); err != nil {
			return ended, err
		}
		ids, err := s.q.FolloweeIDsPage(ctx, sqlcgen.FolloweeIDsPageParams{
			AfterID:  afterID,
			RowLimit: int32(MaxReporterAccountsPerRead),
		})
		if err != nil {
			return ended, err
		}
		if len(ids) == 0 {
			return ended, nil
		}
		n, pageErr := s.sweepPage(ctx, ids)
		ended += n
		if pageErr != nil && ctx.Err() == nil {
			log.Printf("follow sweep: page after %d to %d: %v", afterID, ids[len(ids)-1], pageErr)
		}
		afterID = ids[len(ids)-1]
		if len(ids) < MaxReporterAccountsPerRead {
			return ended, nil
		}
	}
}

// followSweepBlockedBatch is how many blocked pairs one read of
// BlockedActiveFollowPairs returns. The loop reads again until none is left.
const followSweepBlockedBatch = 200

// endBlockedPairs ends every active follow that has a block between the two
// people, in either direction, with reason blocked, through endFollowPair like
// every other end. It returns how many rows it ended. Each pass removes the pairs
// it read from the active set, so the loop stops by itself; a pass that ends
// nothing stops it too, so a pair that cannot be ended cannot spin it.
func (s *FollowSweep) endBlockedPairs(ctx context.Context) (int64, error) {
	var ended int64
	for {
		if err := ctx.Err(); err != nil {
			return ended, err
		}
		pairs, err := s.q.BlockedActiveFollowPairs(ctx, followSweepBlockedBatch)
		if err != nil {
			return ended, err
		}
		if len(pairs) == 0 {
			return ended, nil
		}
		var pass int64
		now := s.now()
		for _, p := range pairs {
			n, err := endFollowPair(ctx, s.q, p.FollowerID, p.FolloweeID, FollowEndBlocked, now)
			pass += n
			if err != nil {
				return ended + pass, err
			}
		}
		ended += pass
		if pass == 0 || len(pairs) < followSweepBlockedBatch {
			return ended, nil
		}
	}
}

// sweepPage ends the follows of the people on one page who are no longer
// followable and returns how many rows it ended.
func (s *FollowSweep) sweepPage(ctx context.Context, ids []int64) (int64, error) {
	now := s.now()
	standing, err := s.q.FolloweeStanding(ctx, ids)
	if err != nil {
		return 0, err
	}
	followable := make(map[int64]bool, len(standing))
	for _, row := range standing {
		followable[row.ID] = row.Followable
	}
	signals, err := LoadReporterSignals(ctx, s.q, s.signals, ids, now)
	if err != nil {
		return 0, err
	}

	var ended int64
	var firstErr error
	for _, id := range ids {
		reason := FollowEndReason("")
		switch {
		case !followable[id]:
			reason = FollowEndProfileOff
		case signals[id].Tag != ReporterReliable:
			reason = FollowEndTagLost
		default:
			continue
		}
		n, err := endFollowsOf(ctx, s.q, id, reason, now)
		ended += n
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return ended, firstErr
}
