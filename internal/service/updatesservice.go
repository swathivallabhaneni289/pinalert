// updatesservice.go is UpdatesService (plan 03.2-03): the lazy producer of in
// app updates (Refresh), the reader that writes every word at read time (List)
// and the read marks (MarkRead). It owns no rule of its own. Whether a report is
// Live, Hidden or withheld is the resolver's answer, and whether that answer is
// news is DecideUpdate's. Every call here is keyed by the account id passed in
// and nothing reads a request context, so Phase 3.3's write time trigger can
// call the same code (D-29).

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// updatesCeilingWindow is the window the hourly ceiling counts over (D-16).
const updatesCeilingWindow = time.Hour

// UpdatesQuerier is the store surface of UpdatesService. Small enough to fake
// without a database. The last four methods belong to the Reliable alert step
// that plan 13 adds to Refresh; they are listed now so the fake and the
// assertion below do not change then.
type UpdatesQuerier interface {
	FollowedOpenReports(ctx context.Context, arg sqlcgen.FollowedOpenReportsParams) ([]sqlcgen.FollowedOpenReportsRow, error)
	UpdateCursorsForReports(ctx context.Context, arg sqlcgen.UpdateCursorsForReportsParams) ([]sqlcgen.UpdateCursorsForReportsRow, error)
	InsertBaselineCursor(ctx context.Context, arg sqlcgen.InsertBaselineCursorParams) error
	AdoptCursorState(ctx context.Context, arg sqlcgen.AdoptCursorStateParams) (int64, error)
	AdvanceCursorAndInsertUpdate(ctx context.Context, arg sqlcgen.AdvanceCursorAndInsertUpdateParams) (int64, error)
	MarkFadingAndInsertUpdate(ctx context.Context, arg sqlcgen.MarkFadingAndInsertUpdateParams) (int64, error)
	PruneUpdates(ctx context.Context, arg sqlcgen.PruneUpdatesParams) error
	PruneCursors(ctx context.Context, arg sqlcgen.PruneCursorsParams) error
	UpdatesForAccount(ctx context.Context, arg sqlcgen.UpdatesForAccountParams) ([]sqlcgen.UpdatesForAccountRow, error)
	CountUpdatesForAccount(ctx context.Context, arg sqlcgen.CountUpdatesForAccountParams) (int64, error)
	UnreadUpdateCount(ctx context.Context, arg sqlcgen.UnreadUpdateCountParams) (int64, error)
	MarkUpdatesRead(ctx context.Context, arg sqlcgen.MarkUpdatesReadParams) (int64, error)
	ReportsForUpdates(ctx context.Context, reportIds []int64) ([]sqlcgen.ReportsForUpdatesRow, error)
	CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReporterAccountsForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error)
	GetAccountHomeCell(ctx context.Context, id int64) (*string, error)
	GetAlertSettings(ctx context.Context, accountID int64) (sqlcgen.GetAlertSettingsRow, error)
	AlertsFromReporterInWindow(ctx context.Context, arg sqlcgen.AlertsFromReporterInWindowParams) (int64, error)
	InsertReliableAlert(ctx context.Context, arg sqlcgen.InsertReliableAlertParams) (int64, error)
}

var _ UpdatesQuerier = (*sqlcgen.Queries)(nil)

// NearbyReader is the one method of ReportService that the Reliable alert step
// of plan 13 needs: the resolved reports around a point. UpdatesService holds
// it now so the constructor does not change then.
type NearbyReader interface {
	Nearby(ctx context.Context, q NearbyQuery) ([]ReportView, error)
}

// UpdatesService produces and reads in app updates.
type UpdatesService struct {
	q       UpdatesQuerier
	reports NearbyReader
	now     func() time.Time
}

// UpdatesOption configures an UpdatesService at construction time.
type UpdatesOption func(*UpdatesService)

// WithUpdatesClock overrides the clock. Tests use it to pin time; production
// code never needs it.
func WithUpdatesClock(fn func() time.Time) UpdatesOption {
	return func(s *UpdatesService) {
		s.now = fn
	}
}

// NewUpdatesService stores the querier and the reports reader and never
// dereferences either. reports may be nil. A caller that holds a possibly nil
// *ReportService must pass an untyped nil in that case, because a typed nil
// pointer stored in the interface is not a nil interface (the PlaceSearch
// comment in router.go says the same).
func NewUpdatesService(q UpdatesQuerier, reports NearbyReader, opts ...UpdatesOption) *UpdatesService {
	s := &UpdatesService{q: q, reports: reports, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Refresh brings one account's updates up to date. It is lazy: the caller (the
// poll of the bell) asks, and time events such as fading are judged now, with no
// ticker and no scheduler (D-12). The order is part of the specification:
//
//  1. Prune the account's updates older than the retention window and its
//     cursors of reports that expired before that edge (D-16).
//  2. Read the followed open reports once (derived, never stored).
//  3. Read the votes, the reporters and the account's cursors once each over
//     those ids.
//  4. Per report resolve it exactly as the feed does (BuildVoteTally, Resolve
//     with the moderation state, ViewOf) and write only what DecideUpdate says:
//     a silent baseline, a silent adoption, one state event or one fading event.
//
// An empty followed set ends the pass early; the Reliable alert step that plan
// 13 adds does not depend on it.
func (s *UpdatesService) Refresh(ctx context.Context, accountID int64) error {
	now := s.now().UTC()

	edge := now.Add(-UpdatesRetention)
	if err := s.q.PruneUpdates(ctx, sqlcgen.PruneUpdatesParams{AccountID: accountID, Edge: edge}); err != nil {
		return fmt.Errorf("service: Refresh: pruning updates: %w", err)
	}
	if err := s.q.PruneCursors(ctx, sqlcgen.PruneCursorsParams{AccountID: accountID, Edge: edge}); err != nil {
		return fmt.Errorf("service: Refresh: pruning cursors: %w", err)
	}

	if err := s.refreshFollowed(ctx, accountID, now); err != nil {
		return err
	}

	// Next step (plan 13, NOTIF-05): the Reliable reporter alert step goes here,
	// measured from the account's home area and written through
	// MayNotifyAboutReport and InsertReliableAlert.
	return nil
}

// refreshFollowed is step 2 to 4 of Refresh. The resolver call lives in this one
// function body together with the moderation state, which is what
// TestResolveCallersMentionModeration checks.
func (s *UpdatesService) refreshFollowed(ctx context.Context, accountID int64, now time.Time) error {
	followed, err := s.q.FollowedOpenReports(ctx, sqlcgen.FollowedOpenReportsParams{
		AccountID: accountID,
		Now:       now,
		RowCap:    FollowedReportsCap,
	})
	if err != nil {
		return fmt.Errorf("service: Refresh: reading followed reports: %w", err)
	}
	if len(followed) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(followed))
	for _, row := range followed {
		ids = append(ids, row.ID)
	}

	voteRows, err := s.q.CurrentVotesForReports(ctx, ids)
	if err != nil {
		return fmt.Errorf("service: Refresh: reading votes: %w", err)
	}
	votesByReport := voteRowsByReport(voteRows)

	reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
	if err != nil {
		return fmt.Errorf("service: Refresh: reading reporters: %w", err)
	}
	reporterByReport := make(map[int64]*int64, len(reporterRows))
	for _, rr := range reporterRows {
		reporterByReport[rr.ReportID] = rr.ReporterAccountID
	}

	cursorRows, err := s.q.UpdateCursorsForReports(ctx, sqlcgen.UpdateCursorsForReportsParams{AccountID: accountID, ReportIds: ids})
	if err != nil {
		return fmt.Errorf("service: Refresh: reading cursors: %w", err)
	}
	cursorByReport := make(map[int64]UpdateCursor, len(cursorRows))
	for _, c := range cursorRows {
		cursorByReport[c.ReportID] = UpdateCursor{
			LastState:    Visibility(c.LastState),
			LastUpdateAt: timePtr(c.LastUpdateAt),
			AdoptedAt:    c.AdoptedAt,
			FadingFor:    timePtr(c.FadingFor),
		}
	}

	for _, row := range followed {
		reportVotes := votesByReport[row.ID]
		meta := ReportMeta{Severity: Severity(row.Severity), Category: Category(row.Category), Moderation: ModerationState(row.ModerationState)}
		tally := BuildVoteTally(reportVotes, row.ID, reporterByReport[row.ID])
		vis, _ := Resolve(meta, tally, now)
		view := ViewOf(vis, row.ExpiresAt, now)

		// The account's own latest vote on this report, of any kind: the newest
		// row among those the account cast (D-13).
		var ownLatest *time.Time
		for _, v := range reportVotes {
			if v.AccountID == accountID && (ownLatest == nil || v.CreatedAt.After(*ownLatest)) {
				at := v.CreatedAt
				ownLatest = &at
			}
		}

		var cursor *UpdateCursor
		if c, ok := cursorByReport[row.ID]; ok {
			cursor = &c
		}

		d := DecideUpdate(DecideInput{
			Cursor:          cursor,
			Visibility:      vis,
			View:            view,
			ExpiresAt:       row.ExpiresAt,
			OwnLatestVoteAt: ownLatest,
			Muted:           row.Muted,
			Now:             now,
		})

		switch d.Action {
		case DecideBaseline:
			err = s.q.InsertBaselineCursor(ctx, sqlcgen.InsertBaselineCursorParams{
				AccountID: accountID,
				ReportID:  row.ID,
				State:     string(d.NewState),
				Now:       now,
			})
		case DecideAdopt:
			_, err = s.q.AdoptCursorState(ctx, sqlcgen.AdoptCursorStateParams{
				NewState:  string(d.NewState),
				Now:       now,
				AccountID: accountID,
				ReportID:  row.ID,
				OldState:  string(cursor.LastState),
			})
		case DecideEvent:
			_, err = s.q.AdvanceCursorAndInsertUpdate(ctx, sqlcgen.AdvanceCursorAndInsertUpdateParams{
				NewState:    string(d.NewState),
				Now:         now,
				AccountID:   accountID,
				ReportID:    row.ID,
				OldState:    string(cursor.LastState),
				GapEdge:     now.Add(-UpdateReportGap),
				HourEdge:    now.Add(-updatesCeilingWindow),
				Kind:        string(d.Kind),
				Ceiling:     UpdatesHourlyCeiling,
				BucketStart: HourBucket(now),
			})
		case DecideFading:
			_, err = s.q.MarkFadingAndInsertUpdate(ctx, sqlcgen.MarkFadingAndInsertUpdateParams{
				ExpiresAt:   row.ExpiresAt,
				Now:         now,
				AccountID:   accountID,
				ReportID:    row.ID,
				GapEdge:     now.Add(-UpdateReportGap),
				HourEdge:    now.Add(-updatesCeilingWindow),
				Ceiling:     UpdatesHourlyCeiling,
				BucketStart: HourBucket(now),
			})
		default:
			continue
		}
		// No row back from either insert means another request moved the cursor
		// first or the hour gap blocked it; both are the normal answer.
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("service: Refresh: writing report %d: %w", row.ID, err)
		}
	}
	return nil
}

// timePtr turns a nullable timestamp column into a pointer to a copy, nil for
// NULL. The address of the field is never taken, because a NULL column would
// then become a non nil pointer to the zero time.
func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
