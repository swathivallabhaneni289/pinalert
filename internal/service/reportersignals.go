// reportersignals.go is the one way a production path reads reporter signals
// (Phase 9, plan 09-03, research Pitfall 2). Every surface that shows a
// reporter tag (the Nearby feed, the vote response, Past and Focus, Activity,
// the push dispatcher, and every later caller) goes through
// reporterSignalsWith or LoadReporterSignals, so one SignalSource plugged in
// here reaches all of them and no surface can show a different tag. A guard
// test fails on any other production call of ReporterSignals.

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
)

// ErrTooManyReporterAccounts is returned by LoadReporterSignals when the caller
// asks for more than MaxReporterAccountsPerRead accounts (T-09-09).
var ErrTooManyReporterAccounts = errors.New("service: too many reporter accounts in one read")

// SignalSource supplies the inputs that are not in the history or vote rows.
// A nil source means outcomes only, exactly the tag every surface showed before
// this plan. The source is called once per read with the same account ids
// ReporterSignals receives, and the options it returns are applied to that one
// call. Plan 09-13 supplies the first real source (tapped ratings).
type SignalSource interface {
	SignalOptions(ctx context.Context, accountIDs []int64, now time.Time) ([]SignalOption, error)
}

// ReporterTagQuerier is the two reads a standalone tag read needs.
type ReporterTagQuerier interface {
	ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)
	CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
}

// reporterSignalsWith is the one production caller of ReporterSignals. With a
// nil source it returns exactly what ReporterSignals returns for the same
// inputs. With a source it asks once for the options and passes them on; a
// source error fails the read, wrapped, with a nil map.
func reporterSignalsWith(ctx context.Context, src SignalSource, accountIDs []int64, history []sqlcgen.ReporterHistoryReportsRow, votes []sqlcgen.CurrentVotesForReportsRow, now time.Time) (map[int64]ReporterSignal, error) {
	if src == nil {
		return ReporterSignals(accountIDs, history, votes, now), nil
	}
	opts, err := src.SignalOptions(ctx, accountIDs, now)
	if err != nil {
		return nil, fmt.Errorf("service: reporter signal source: %w", err)
	}
	return ReporterSignals(accountIDs, history, votes, now, opts...), nil
}

// LoadReporterSignals reads the history and the votes for the accounts and
// rates them through reporterSignalsWith: the same read sequence as the feed
// (history, then votes over voteReadIDs(nil, history)). No ids gives an empty
// map without a query, and more than MaxReporterAccountsPerRead ids is
// refused. An account with no history still gets its entry, so it reads as the
// none tag, as it does on the feed.
func LoadReporterSignals(ctx context.Context, q ReporterTagQuerier, src SignalSource, accountIDs []int64, now time.Time) (map[int64]ReporterSignal, error) {
	if len(accountIDs) == 0 {
		return map[int64]ReporterSignal{}, nil
	}
	if len(accountIDs) > MaxReporterAccountsPerRead {
		return nil, ErrTooManyReporterAccounts
	}
	history, err := q.ReporterHistoryReports(ctx, reporterHistoryParams(accountIDs, now))
	if err != nil {
		return nil, fmt.Errorf("service: reporter history: %w", err)
	}
	var votes []sqlcgen.CurrentVotesForReportsRow
	if ids := voteReadIDs(nil, history); len(ids) > 0 {
		votes, err = q.CurrentVotesForReports(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("service: reporter history votes: %w", err)
		}
	}
	return reporterSignalsWith(ctx, src, accountIDs, history, votes, now)
}
