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
	// InsertReliableAlert writes the note and, in the same statement and only when
	// the note row is written, seeds the reader's cursor so the note counts toward
	// the one update per report per hour (D-12, W3).
	InsertReliableAlert(ctx context.Context, arg sqlcgen.InsertReliableAlertParams) (int64, error)
	// MutedPeopleIDs is the set of people the reader muted (Phase 9 plan 18,
	// D-12). The Reliable reporter alert skips their reports; nothing else reads
	// it here.
	MutedPeopleIDs(ctx context.Context, muterID int64) ([]int64, error)
	// BlockedEitherWayIDs is the set of people with a block between them and the
	// reader in either direction (quick 261103-jal, F1). The Reliable reporter
	// alert skips their reports, the way the worker's person check does.
	BlockedEitherWayIDs(ctx context.Context, accountID int64) ([]int64, error)
	// The follow note (Phase 9 plan 22, D-09). NotifyFollowsFor lists the
	// reader's Notify me follows, FolloweeStanding says who is still followable,
	// InsertFollowedPostedAlert writes the note, and for the list
	// PublicProfilePathsForViewer names the people the reader may still see and
	// ActiveFollowsAmong says which follows are still active.
	NotifyFollowsFor(ctx context.Context, arg sqlcgen.NotifyFollowsForParams) ([]sqlcgen.NotifyFollowsForRow, error)
	FolloweeStanding(ctx context.Context, accountIds []int64) ([]sqlcgen.FolloweeStandingRow, error)
	InsertFollowedPostedAlert(ctx context.Context, arg sqlcgen.InsertFollowedPostedAlertParams) (int64, error)
	PublicProfilePathsForViewer(ctx context.Context, arg sqlcgen.PublicProfilePathsForViewerParams) ([]sqlcgen.PublicProfilePathsForViewerRow, error)
	ActiveFollowsAmong(ctx context.Context, arg sqlcgen.ActiveFollowsAmongParams) ([]int64, error)
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
	// The background area lookup of quick 261119-sps (updatesarea.go). Unset
	// leaves StartAreaLookups doing nothing.
	areaStore   UpdatesAreaStore
	areaNamer   AreaNamer
	areaSpacing time.Duration
	areaDone    func()
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
	s := &UpdatesService{q: q, reports: reports, now: time.Now, areaSpacing: updatesAreaLookupSpacing}
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
//
//  2. Read the followed open reports once (derived, never stored).
//
//  3. Read the votes, the reporters and the account's cursors once each over
//     those ids.
//
//  4. Per report resolve it exactly as the feed does (BuildVoteTally, Resolve
//     with the moderation state, ViewOf) and write only what DecideUpdate says:
//     a silent baseline, a silent adoption, one state event or one fading event.
//
//  5. Run the follow note step (refreshFollowedPosts), then the Reliable
//     reporter alert step (refreshReliableAlerts). An empty followed set ends
//     step 2 to 4 early; neither alert step depends on it.
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

	// The follow note step (PROF-03, D-09) runs before the Reliable reporter step
	// because follow wins the one first alert slot a report has per account: a
	// person who follows a Reliable reporter with the bell on gets the follow
	// note, never two. The follow phone note is queued only from a written follow
	// row, so the Reliable row must never take that slot. The step hands back the
	// people it covers, and the Reliable step skips their reports (review WR-02,
	// which replaces W3: a follow note not written this time, for example over
	// the hourly ceiling, is tried again on a later poll). When the follow step
	// fails or hits its read cap it cannot say whom it covered, so the Reliable
	// step waits for the next refresh. The first error is the one returned.
	follow, followErr := s.refreshFollowedPosts(ctx, accountID, now)

	// The Reliable reporter alert step (NOTIF-05, D-26) comes last, with the same
	// now. It measures from the account's home area and writes only through
	// MayNotifyAboutReport and InsertReliableAlert, and it ends quietly when the
	// reader has no home area, so every update above works without it (D-28).
	var reliableErr error
	if followErr == nil && !follow.truncated {
		reliableErr = s.refreshReliableAlerts(ctx, accountID, now, follow)
	}
	if followErr != nil {
		return fmt.Errorf("service: Refresh: follow notes: %w", followErr)
	}
	if reliableErr != nil {
		return fmt.Errorf("service: Refresh: reliable alerts: %w", reliableErr)
	}
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
	// ReviewReportID is set only on the removed and stays removed rows about
	// the reader's own report while that report is still removed (quick
	// 261118-rz6, which replaced vvb's Policy link): the row opens the author's
	// box for that report, never the report itself, so it is kept apart from
	// ReportID, which a row opens.
	ReviewReportID *int64
	// Area is the report's stored coarse area name cut by AreaWords (quick
	// 261119-sps, owner 2026-11-19). It is empty whenever the row hides the
	// report's own words (an unavailable report, a finished Rescue report, the
	// rolled up row), on a row not about one report, and while no name is
	// stored. The reader's own report rows carry it like any other row.
	Area string
}

// UpdatesPage is one list read: the newest rows, the unread count capped at
// UnreadCountCap and the number of rows kept.
type UpdatesPage struct {
	Items  []UpdateItem
	Unread int
	Total  int
	// AreaMissing lists, newest row first and once each, the reports of rows
	// that would carry an area but have no stored name and whose one lookup
	// was never tried (quick 261119-sps). StartAreaLookups takes it after the
	// answer is written.
	AreaMissing []int64
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

	// The names of the follow notes (plan 09-22, D-09, D-20, research Q9). One
	// read of the reporters' public usernames and one of the reader's active
	// follows, and only when the page holds a follow note, so every other list
	// costs nothing more. A follow note names its person only while the person is
	// public with a clear username and does not block the reader, and the reader
	// still follows them; the words are built here at read time and nothing is
	// stored.
	followNames, activeFollows, err := s.followNoteNames(ctx, accountID, rows, reporterByReport)
	if err != nil {
		return UpdatesPage{}, err
	}

	// areaOf is the area a row about rep carries (quick 261119-sps): the stored
	// name cut by AreaWords, or nothing for a finished Rescue report, whose row
	// shows no report words either. A row that would carry an area while no
	// name is stored and the lookup was never tried puts its report on
	// AreaMissing, once.
	missingSeen := make(map[int64]struct{})
	areaOf := func(rep sqlcgen.ReportsForUpdatesRow, finishedRescue bool) string {
		if finishedRescue {
			return ""
		}
		if rep.AreaName != nil {
			if name := AreaWords(*rep.AreaName); name != "" {
				return name
			}
		}
		if !rep.AreaLookupTried {
			if _, dup := missingSeen[rep.ID]; !dup {
				missingSeen[rep.ID] = struct{}{}
				page.AreaMissing = append(page.AreaMissing, rep.ID)
			}
		}
		return ""
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
		finished := view == FeedViewResolved || view == FeedViewPast
		// The same rule as CoarsenFinishedRescue, which drops the report line.
		finishedRescue := finished && meta.Category == CategoryRescueNeeded
		// The rows about the reader's own report (quick 261117-vvb, quick
		// 261118-rz6) are told whatever the report's state, because the state
		// is their news: one fixed line, no report words, category or trust,
		// and never the no longer available line. The shown, kept and back
		// rows open the report only while it is not withheld (it may have been
		// removed since); the hidden row opens nothing; the removed and stays
		// removed rows open the author's box while the report is still
		// removed, and nothing once it was restored.
		if kind.IsOwnReport() {
			item.Available = true
			item.Event = OwnReportLine(kind)
			switch kind {
			case UpdateOwnShown, UpdateOwnKept, UpdateOwnRestored:
				if !unavailable {
					item.ReportID = row.ReportID
				}
			case UpdateOwnRemoved, UpdateOwnStaysRemoved:
				if ok && meta.Moderation == ModerationRemoved {
					item.ReviewReportID = row.ReportID
				}
			}
			// Quick 261119-sps: the area only while the report is not withheld
			// or gone. item.Available is true on these rows whatever the
			// state, so it cannot be the test.
			if ok && !unavailable {
				item.Area = areaOf(rep, finishedRescue)
			}
			page.Items = append(page.Items, item)
			continue
		}
		if unavailable {
			// Held, removed or gone: no report id, no category, no trust line, no
			// text. The row still says something happened and is no longer there.
			item.Event = UnavailableLine
			page.Items = append(page.Items, item)
			continue
		}

		// A follow note reads as gone, with no username, no category and no report
		// link, as soon as any of three things stops being true: the report is still
		// live (Active or Disputed, so a Resolved or expired report never carries
		// its reporter's name or a way to its exact point, D-20), the person is
		// public with a clear username, and the reader still follows them. The row
		// itself stays as history (research Q9).
		var followName string
		if kind == UpdateFollowedPosted {
			live := view == FeedViewActive || view == FeedViewDisputed
			if reporter := reporterByReport[rep.ID]; reporter != nil {
				followName = followNames[*reporter]
				live = live && activeFollows[*reporter]
			} else {
				live = false
			}
			if !live || followName == "" {
				item.Event = UnavailableLine
				page.Items = append(page.Items, item)
				continue
			}
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

		item.Available = true
		item.ReportID = row.ReportID
		item.Category = meta.Category
		item.Severity = meta.Severity
		item.Event = EventLine(kind)
		item.Trust = UpdateTrustLine(kind, trust, view, confirmed)
		item.Area = areaOf(rep, finishedRescue)
		if kind == UpdateFollowedPosted {
			// Line 1 names the person, line 2 is the category and severity words
			// and never the report's own text (UI-SPEC 2a).
			item.Event = FollowedPostedLine(followName)
			item.ReportLine = FollowedPostedDetail(CategoryWord(meta.Category), SeverityWord(meta.Severity))
			page.Items = append(page.Items, item)
			continue
		}
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

// followNoteNames reads, for the follow notes of one page, which reporters the
// reader may still see named and which of them the reader still follows. The
// first map is account id to username (public, opted in, clear username, not
// blocking the reader; PublicProfilePathsForViewer is the one place that rule
// lives), the second is the set of people with an active follow from the reader.
// Both are nil when the page has no follow note, and neither read is made then.
// Reading a map for an account that is not in it gives the empty answer, so a
// person who went private, was deleted, blocked the reader or is no longer
// followed reads as gone.
func (s *UpdatesService) followNoteNames(ctx context.Context, accountID int64, rows []sqlcgen.UpdatesForAccountRow, reporterByReport map[int64]*int64) (map[int64]string, map[int64]bool, error) {
	var people []int64
	seen := map[int64]struct{}{}
	for _, row := range rows {
		if UpdateKind(row.Kind) != UpdateFollowedPosted || row.ReportID == nil {
			continue
		}
		reporter := reporterByReport[*row.ReportID]
		if reporter == nil {
			continue
		}
		if _, dup := seen[*reporter]; dup {
			continue
		}
		seen[*reporter] = struct{}{}
		people = append(people, *reporter)
	}
	if len(people) == 0 {
		return nil, nil, nil
	}
	nameRows, err := s.q.PublicProfilePathsForViewer(ctx, sqlcgen.PublicProfilePathsForViewerParams{AccountIds: people, ViewerID: accountID})
	if err != nil {
		return nil, nil, fmt.Errorf("service: List: reading follow note names: %w", err)
	}
	names := make(map[int64]string, len(nameRows))
	for _, nr := range nameRows {
		names[nr.ID] = nr.Username
	}
	followRows, err := s.q.ActiveFollowsAmong(ctx, sqlcgen.ActiveFollowsAmongParams{FollowerID: accountID, FolloweeIds: people})
	if err != nil {
		return nil, nil, fmt.Errorf("service: List: reading follows: %w", err)
	}
	active := make(map[int64]bool, len(followRows))
	for _, id := range followRows {
		active[id] = true
	}
	return names, active, nil
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
