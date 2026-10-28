package demo

// visitor.go reads back what the visitor did in this world, for the Demo's My
// activity page (plan 05-19, D-05). World.Activity says which reports were
// posted, confirmed or disputed; the times of the votes are only in the vote log,
// which this file reads through the same twin of the store the real services use.

import (
	"context"
	"time"

	"pinalert/internal/service"
)

// VisitorVote is the visitor's standing content vote on one report: confirm or
// dispute, and when it was cast on the world clock.
type VisitorVote struct {
	Value service.VoteValue
	At    time.Time
}

// VisitorVotes answers, for each of the given reports the visitor has a standing
// content vote on, that vote. A report the visitor has not voted on, and a vote
// of another account or another kind (a resolution vote), is left out. The
// newest vote wins, as in the real read, so a visitor who confirmed and then
// disputed a report has the dispute.
func (w *World) VisitorVotes(ctx context.Context, ids []int64) (map[int64]VisitorVote, error) {
	out := make(map[int64]VisitorVote, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := w.q.CurrentVotesForReports(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.AccountID != VisitorAccountID || r.Kind != string(service.VoteKindContent) {
			continue
		}
		out[r.ReportID] = VisitorVote{Value: service.VoteValue(r.Value), At: r.CreatedAt}
	}
	return out, nil
}
