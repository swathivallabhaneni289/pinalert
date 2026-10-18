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

// UpToMessage is the API message for a mark read call whose up_to is not a
// positive id. It is for the API only and is never shown on a screen.
const UpToMessage = "Send the id of the newest update shown."

// UpdateItem is one row of an updates list. It has no field for a description
// beyond ReportLine, a coordinate, a session id, a voter cell or an account id,
// so a row cannot leak what the feed withholds (T-03.2-01). Every word is built
// when the list is read and none is stored.
type UpdateItem struct {
	ID   int64
	Kind UpdateKind
	// ReportID is nil for the rolled up row and for an unavailable report, so a
	// row never links to a report the reader may not open.
	ReportID  *int64
	Available bool
	Unread    bool
	CreatedAt time.Time
	// Event is line 1: the event words, the unavailable line or the rolled up
	// line.
	Event string
	// ReportLine is the first line of the report's description, empty for a
	// finished Rescue report, an unavailable report and the rolled up row.
	ReportLine string
	Category   Category
	Severity   Severity
	// Trust is the trust sentence chosen from the live trust object.
	Trust     string
	MoreCount int
}

// UpdatesPage is one list read: the newest rows, the unread count capped at
// UnreadCountCap and the number of rows kept.
type UpdatesPage struct {
	Items  []UpdateItem
	Unread int
	Total  int
}

// List returns the account's newest updates, newest first, with the unread
// count and the kept total. The words are built here from the report's current
// facts (D-15): the rows hold a kind and a report id only. A report that is
// held, removed or missing reads as unavailable with no detail (D-05), decided
// by the same Resolve the feed uses, with the moderation state set in this
// function body.
func (s *UpdatesService) List(ctx context.Context, accountID int64, limit int) (UpdatesPage, error) {
	now := s.now().UTC()
	since := now.Add(-UpdatesRetention)
	if limit <= 0 {
		limit = BellListSize
	}
	if limit > ActivityUpdatesListSize {
		limit = ActivityUpdatesListSize
	}

	rows, err := s.q.UpdatesForAccount(ctx, sqlcgen.UpdatesForAccountParams{AccountID: accountID, Since: since, RowLimit: int32(limit)})
	if err != nil {
		return UpdatesPage{}, fmt.Errorf("service: List: reading updates: %w", err)
	}
	unread, err := s.q.UnreadUpdateCount(ctx, sqlcgen.UnreadUpdateCountParams{AccountID: accountID, Since: since, Cap: UnreadCountCap})
	if err != nil {
		return UpdatesPage{}, fmt.Errorf("service: List: counting unread: %w", err)
	}
	total, err := s.q.CountUpdatesForAccount(ctx, sqlcgen.CountUpdatesForAccountParams{AccountID: accountID, Since: since})
	if err != nil {
		return UpdatesPage{}, fmt.Errorf("service: List: counting updates: %w", err)
	}
	page := UpdatesPage{Items: make([]UpdateItem, 0, len(rows)), Unread: int(unread), Total: int(total)}

	var ids []int64
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if row.ReportID == nil || UpdateKind(row.Kind) == UpdateMore {
			continue
		}
		if _, dup := seen[*row.ReportID]; !dup {
			seen[*row.ReportID] = struct{}{}
			ids = append(ids, *row.ReportID)
		}
	}

	reportByID := make(map[int64]sqlcgen.ReportsForUpdatesRow, len(ids))
	var votesByReport map[int64][]sqlcgen.CurrentVotesForReportsRow
	reporterByReport := make(map[int64]*int64, len(ids))
	if len(ids) > 0 {
		reportRows, err := s.q.ReportsForUpdates(ctx, ids)
		if err != nil {
			return UpdatesPage{}, fmt.Errorf("service: List: reading reports: %w", err)
		}
		for _, r := range reportRows {
			reportByID[r.ID] = r
		}
		voteRows, err := s.q.CurrentVotesForReports(ctx, ids)
		if err != nil {
			return UpdatesPage{}, fmt.Errorf("service: List: reading votes: %w", err)
		}
		votesByReport = voteRowsByReport(voteRows)
		reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
		if err != nil {
			return UpdatesPage{}, fmt.Errorf("service: List: reading reporters: %w", err)
		}
		for _, rr := range reporterRows {
			reporterByReport[rr.ReportID] = rr.ReporterAccountID
		}
	}

	for _, row := range rows {
		kind := UpdateKind(row.Kind)
		item := UpdateItem{
			ID:        row.ID,
			Kind:      kind,
			Unread:    !row.ReadAt.Valid,
			CreatedAt: row.CreatedAt,
		}

		if kind == UpdateMore {
			item.Available = true
			item.MoreCount = int(row.MoreCount)
			item.Event = MoreLine(int(row.MoreCount))
			page.Items = append(page.Items, item)
			continue
		}
		if row.ReportID == nil {
			// An event row always names a report; one that does not reads as gone.
			item.Event = UnavailableLine
			page.Items = append(page.Items, item)
			continue
		}

		rep, ok := reportByID[*row.ReportID]
		unavailable := !ok
		var (
			vis     Visibility
			view    FeedView
			tally   VoteTally
			reportV []sqlcgen.CurrentVotesForReportsRow
			meta    ReportMeta
		)
		if ok {
			reportV = votesByReport[rep.ID]
			meta = ReportMeta{Severity: Severity(rep.Severity), Category: Category(rep.Category), Moderation: ModerationState(rep.ModerationState)}
			tally = BuildVoteTally(reportV, rep.ID, reporterByReport[rep.ID])
			vis, _ = Resolve(meta, tally, now)
			view = ViewOf(vis, rep.ExpiresAt, now)
			unavailable = vis == VisibilityWithheld
		}
		if unavailable {
			// Held, removed or gone: no report id, no category, no trust line, no
			// text. The row still says something happened and is no longer there.
			item.Event = UnavailableLine
			page.Items = append(page.Items, item)
			continue
		}

		trust := BuildTrust(TrustInput{
			Rows:              reportV,
			ReportID:          rep.ID,
			ReporterAccountID: reporterByReport[rep.ID],
			Meta:              meta,
			Visibility:        vis,
			CreatedAt:         rep.CreatedAt,
			ExpiresAt:         rep.ExpiresAt,
			Now:               now,
			Reporter:          nil,
		})
		confirmed := ConfirmedByPeople(vis, view, tally)

		finished := view == FeedViewResolved || view == FeedViewPast
		item.Available = true
		item.ReportID = row.ReportID
		item.Category = meta.Category
		item.Severity = meta.Severity
		item.Event = EventLine(kind)
		item.Trust = UpdateTrustLine(kind, trust, view, confirmed)
		// Q1 owner confirmed 2026-10-18: rows show the report's first line. If the
		// owner says no, delete this one assignment.
		item.ReportLine = ReportFirstLine(Report{
			ID:          rep.ID,
			Category:    meta.Category,
			Severity:    meta.Severity,
			Description: rep.Description,
			CreatedAt:   rep.CreatedAt,
			ExpiresAt:   rep.ExpiresAt,
		}, finished)
		page.Items = append(page.Items, item)
	}
	return page, nil
}

// MarkRead marks the account's own unread updates up to and including upTo as
// read and returns how many it changed. The id comes from the newest update the
// reader was shown, so an update that arrived after the list was drawn stays
// unread.
func (s *UpdatesService) MarkRead(ctx context.Context, accountID, upTo int64) (int64, error) {
	if upTo <= 0 {
		return 0, ValidationError{Field: "up_to", Message: UpToMessage}
	}
	n, err := s.q.MarkUpdatesRead(ctx, sqlcgen.MarkUpdatesReadParams{Now: s.now().UTC(), AccountID: accountID, UpTo: upTo})
	if err != nil {
		return 0, fmt.Errorf("service: MarkRead: %w", err)
	}
	return n, nil
}
