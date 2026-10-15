// moderation.go holds the moderator side of D-08: the three lists, Find, Keep,
// Remove and the server list of moderator emails. Moderators and the author are
// the only people who see a held or removed report (R-01).

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

// The two server messages of the moderator API. They are the UI-SPEC copy and
// stay inside the closed character set the server copy test checks.
const (
	// ModAlreadyHandledMessage answers a Keep or Remove on a report that is no
	// longer where the moderator saw it.
	ModAlreadyHandledMessage = "This report was already handled. Refresh the lists."
	// ModNotFoundMessage answers an id that is not a report.
	ModNotFoundMessage = "No report found with that id. Check the id and try again."
)

// The two actions written to moderation_actions.
const (
	ModerationActionKeep   = "keep"
	ModerationActionRemove = "remove"
)

// ErrAlreadyHandled is returned by Keep and Remove for a report that is in no
// list (Keep) or already removed (Remove). The handler maps it to 409.
var ErrAlreadyHandled = errors.New("service: report already handled")

// The view slugs a ModerationRow carries. The first four are the Phase 3.1
// FeedView values; ModViewNone is a report that is in no view at all (held,
// removed, or a visibility the partition does not list).
const (
	ModViewActive   = "active"
	ModViewDisputed = "disputed"
	ModViewResolved = "resolved"
	ModViewPast     = "past"
	ModViewNone     = "none"
)

// The visibility words of the moderation row, the UI-SPEC "State labels".
const (
	ModWordShowing  = "Showing to everyone"
	ModWordDisputed = "Disputed"
	ModWordResolved = "Resolved"
	ModWordExpired  = "Expired"
	ModWordNoView   = "No longer in any view"
)

// The slugs of the three lists, as Find reports which lists a report is in.
const (
	ModListReview  = "review"
	ModListFlagged = "flagged"
	ModListPending = "pending"
)

// ModerationQuerier is the slice of the generated queries the moderator service
// uses. It is declared here, in the consumer, the way VotingQuerier is.
// CurrentVotesForReports and ReporterAccountsForReports are also on Querier and
// VotingQuerier, deliberately: each consumer defines the interface it needs
// (the existing style in report.go).
type ModerationQuerier interface {
	ReviewQueue(ctx context.Context) ([]sqlcgen.ReviewQueueRow, error)
	FlagQueue(ctx context.Context) ([]sqlcgen.FlagQueueRow, error)
	PendingQueue(ctx context.Context) ([]sqlcgen.PendingQueueRow, error)
	ModerationQueueCounts(ctx context.Context) (sqlcgen.ModerationQueueCountsRow, error)
	ReportForModeration(ctx context.Context, reportID int64) (sqlcgen.ReportForModerationRow, error)
	ModerationStatusForReport(ctx context.Context, reportID int64) (sqlcgen.ModerationStatusForReportRow, error)
	KeepReport(ctx context.Context, reportID int64) error
	RemoveReport(ctx context.Context, reportID int64) error
	InsertModerationAction(ctx context.Context, arg sqlcgen.InsertModerationActionParams) error
	CurrentVotesForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReporterAccountsForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error)
}

var _ ModerationQuerier = (*sqlcgen.Queries)(nil)

// ModerationFlagCount is one reason with the number of open flags that chose
// it. Label is the dialog's own label for the reason (FlagReasonLabel).
type ModerationFlagCount struct {
	Reason FlagReason
	Label  string
	Count  int64
}

// ModerationRow is one report as the moderator screen shows it. It holds no
// author, flagger, email, account or session (T-04-04).
type ModerationRow struct {
	ID          int64
	Category    Category
	Severity    Severity
	Description string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	// WrittenAt is set only past the 30 minute gap (WrittenAtForDisplay).
	WrittenAt  *time.Time
	Serious    bool
	Moderation ModerationState
	// View is one of the ModView constants and VisibilityWord the matching
	// ModWord, empty for a report that is in no view.
	View           string
	VisibilityWord string
	// Scores and Model are set only while the check is done and needs review.
	Scores      map[string]float64
	Model       string
	FlagTotal   int64
	FlagReasons []ModerationFlagCount
}

// ModerationQueues is the answer of Queues: the three lists in their SQL order
// and the heading counts, which come from ModerationQueueCounts only.
type ModerationQueues struct {
	Review  []ModerationRow
	Flagged []ModerationRow
	Pending []ModerationRow
	Counts  struct{ Review, Flagged, Pending int64 }
}

// ModerationService is the moderator side of D-08. Steps of the reads:
//  1. Queues reads the three lists and the counts. The counts come only from
//     ModerationQueueCounts, never from the length of a capped list.
//  2. All rows of the three lists go through one builder that makes one batched
//     vote read and one batched reporter read for the whole response, never one
//     per list and never one per row. An empty response reads nothing.
//  3. The builder asks Resolve for each report with the report's real
//     moderation state, so a held or removed report resolves to withheld. A
//     withheld report is in no view and has no visibility word; any other
//     report takes its view and word from ModerationViewOf.
//  4. Find reads one report by id (ErrReportNotFound for an unknown id), then
//     the lists it is in.
//
// Steps of the writes:
//  5. Keep reads ModerationStatusForReport (ErrReportNotFound for an unknown
//     id). When the report is in none of the three lists, which includes a
//     removed report, it returns ErrAlreadyHandled. Otherwise it writes the
//     audit row (InsertModerationAction, action keep), then KeepReport, then
//     re-reads the report and builds its row. KeepReport clears the automatic
//     flag, dismisses the open people flags and releases a pending report into
//     the normal rules, so a held report enters the feed with no separate
//     Release action. The re-read row carries no scores because the statement
//     clears needs_review.
//  6. Remove does the same lookup. A report whose state is removed returns
//     ErrAlreadyHandled. Otherwise it writes the audit row (action remove),
//     then RemoveReport, which withholds any report, listed or not, serious or
//     not, from every feed, map, vote and history view, then re-reads. A
//     moderation Remove is the authority R-03 names, so it withdraws even a
//     serious report that was published while its check was pending. A
//     moderator may Keep or Remove their own report: no special rule.
//
// Two Claude's Discretion readings the owner can veto. First, the audit row is
// written before the state change, because the store uses no transaction and a
// recorded attempt that did not apply is safer for repudiation than an applied
// change with no record; a failed state change answers 500 and the moderator
// retries. Second, two moderators acting on one report at the same instant may
// both pass the 409 check and write two audit rows; that is accepted for an
// owner run screen.
type ModerationService struct {
	q   ModerationQuerier
	now func() time.Time
}

// ModerationOption configures a ModerationService.
type ModerationOption func(*ModerationService)

// WithModerationClock replaces the clock, for tests.
func WithModerationClock(now func() time.Time) ModerationOption {
	return func(s *ModerationService) { s.now = now }
}

// NewModerationService builds the service over q.
func NewModerationService(q ModerationQuerier, opts ...ModerationOption) *ModerationService {
	s := &ModerationService{q: q, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ParseModeratorEmails turns the MODERATOR_EMAILS setting into the set of
// addresses allowed to open the moderator screen (D-08, T-04-07).
//
// Steps:
//  1. Split raw on commas.
//  2. Trim each item and skip it when empty.
//  3. Normalise it with NormalizeEmail, the rule sign in uses, so a mixed case
//     entry still matches the stored address of the account.
//  4. Collect into a set, which drops duplicates.
//
// A bad entry returns a nil map and an error that names the entry by its 1
// based position among the items as written, empty ones included, and never by
// its value, so a mistyped address cannot reach a log (T-04-07). An empty or
// all blank setting returns an empty, non nil set and no error: nobody is a
// moderator.
func ParseModeratorEmails(raw string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	for i, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		email, err := NormalizeEmail(item)
		if err != nil {
			return nil, fmt.Errorf("entry %d is not a bare email address", i+1)
		}
		set[email] = struct{}{}
	}
	return set, nil
}

// ModerationViewOf names the view a report is in and the word the moderator
// screen shows for it. A withheld report is in no view (Phase 4's own wrap,
// since ViewOf knows nothing about moderation). Every other visibility goes
// through ViewOf once and its FeedView is mapped to a slug and a word;
// FeedViewNone, which is also what ViewOf answers for a visibility it does not
// list, maps to ModViewNone with ModWordNoView so the JSON never carries an
// empty view. No window length or membership rule is restated here.
func ModerationViewOf(vis Visibility, expiresAt, now time.Time) (view, word string) {
	if vis == VisibilityWithheld {
		return ModViewNone, ""
	}
	switch ViewOf(vis, expiresAt, now) {
	case FeedViewActive:
		return ModViewActive, ModWordShowing
	case FeedViewDisputed:
		return ModViewDisputed, ModWordDisputed
	case FeedViewResolved:
		return ModViewResolved, ModWordResolved
	case FeedViewPast:
		return ModViewPast, ModWordExpired
	}
	return ModViewNone, ModWordNoView
}

// moderationRecord holds the columns the four generated moderator row types
// share, so one builder serves all of them.
type moderationRecord struct {
	ID              int64
	Category        string
	Severity        string
	Description     string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	ClientWrittenAt pgtype.Timestamptz
	ModerationState string
	CheckStatus     string
	Scores          []byte
	Model           string
	NeedsReview     bool
	OpenFlagTotal   int64
	FlagCounts      []byte
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func recordFromReview(r sqlcgen.ReviewQueueRow) moderationRecord {
	return moderationRecord{r.ID, r.Category, r.Severity, r.Description, r.CreatedAt, r.ExpiresAt, r.ClientWrittenAt,
		r.ModerationState, r.CheckStatus, r.Scores, derefString(r.Model), r.NeedsReview, r.OpenFlagTotal, r.FlagCounts}
}

func recordFromFlag(r sqlcgen.FlagQueueRow) moderationRecord {
	return moderationRecord{r.ID, r.Category, r.Severity, r.Description, r.CreatedAt, r.ExpiresAt, r.ClientWrittenAt,
		r.ModerationState, derefString(r.CheckStatus), r.Scores, derefString(r.Model), r.NeedsReview, r.OpenFlagTotal, r.FlagCounts}
}

func recordFromPending(r sqlcgen.PendingQueueRow) moderationRecord {
	return moderationRecord{r.ID, r.Category, r.Severity, r.Description, r.CreatedAt, r.ExpiresAt, r.ClientWrittenAt,
		r.ModerationState, r.CheckStatus, r.Scores, derefString(r.Model), r.NeedsReview, r.OpenFlagTotal, r.FlagCounts}
}

func recordFromDetail(r sqlcgen.ReportForModerationRow) moderationRecord {
	return moderationRecord{r.ID, r.Category, r.Severity, r.Description, r.CreatedAt, r.ExpiresAt, r.ClientWrittenAt,
		r.ModerationState, derefString(r.CheckStatus), r.Scores, derefString(r.Model), r.NeedsReview, r.OpenFlagTotal, r.FlagCounts}
}

// buildRows turns records into ModerationRows. An empty slice reads nothing.
// Otherwise it makes one CurrentVotesForReports and one
// ReporterAccountsForReports call over the distinct ids in ascending order,
// reads the clock once, and per record resolves the report the way Nearby
// does, with the real moderation state.
func (s *ModerationService) buildRows(ctx context.Context, recs []moderationRecord) ([]ModerationRow, error) {
	rows := make([]ModerationRow, 0, len(recs))
	if len(recs) == 0 {
		return rows, nil
	}

	seen := make(map[int64]struct{}, len(recs))
	ids := make([]int64, 0, len(recs))
	for _, rec := range recs {
		if _, dup := seen[rec.ID]; dup {
			continue
		}
		seen[rec.ID] = struct{}{}
		ids = append(ids, rec.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	reporterRows, err := s.q.ReporterAccountsForReports(ctx, ids)
	if err != nil {
		return nil, err
	}
	reporterByReport := make(map[int64]*int64, len(reporterRows))
	for _, rr := range reporterRows {
		reporterByReport[rr.ReportID] = rr.ReporterAccountID
	}
	voteRows, err := s.q.CurrentVotesForReports(ctx, ids)
	if err != nil {
		return nil, err
	}
	votesByReport := voteRowsByReport(voteRows)
	now := s.now().UTC()

	for _, rec := range recs {
		meta := ReportMeta{
			Severity:   Severity(rec.Severity),
			Category:   Category(rec.Category),
			Moderation: ModerationState(rec.ModerationState),
		}
		tally := BuildVoteTally(votesByReport[rec.ID], rec.ID, reporterByReport[rec.ID])
		vis, _ := Resolve(meta, tally, now)

		row := ModerationRow{
			ID:          rec.ID,
			Category:    meta.Category,
			Severity:    meta.Severity,
			Description: rec.Description,
			CreatedAt:   rec.CreatedAt,
			ExpiresAt:   rec.ExpiresAt,
			WrittenAt:   WrittenAtForDisplay(rec.CreatedAt, writtenAtPtr(rec.ClientWrittenAt)),
			Serious:     meta.Serious(),
			Moderation:  meta.Moderation,
			FlagTotal:   rec.OpenFlagTotal,
			FlagReasons: []ModerationFlagCount{},
		}
		if meta.Withheld() {
			row.View = ModViewNone
		} else {
			row.View, row.VisibilityWord = ModerationViewOf(vis, rec.ExpiresAt, now)
		}

		if rec.CheckStatus == "done" && rec.NeedsReview {
			row.Model = rec.Model
			if len(rec.Scores) > 0 {
				if err := json.Unmarshal(rec.Scores, &row.Scores); err != nil {
					return nil, fmt.Errorf("service: decoding check scores of report %d: %w", rec.ID, err)
				}
			}
		}

		var counts map[string]int64
		if len(rec.FlagCounts) > 0 {
			if err := json.Unmarshal(rec.FlagCounts, &counts); err != nil {
				return nil, fmt.Errorf("service: decoding flag counts of report %d: %w", rec.ID, err)
			}
		}
		for _, reason := range FlagReasons {
			if n := counts[string(reason)]; n >= 1 {
				row.FlagReasons = append(row.FlagReasons, ModerationFlagCount{Reason: reason, Label: FlagReasonLabel(reason), Count: n})
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// Queues reads the three lists and their counts. The three lists share one
// batched vote read and one batched reporter read.
func (s *ModerationService) Queues(ctx context.Context) (ModerationQueues, error) {
	var out ModerationQueues

	review, err := s.q.ReviewQueue(ctx)
	if err != nil {
		return out, err
	}
	flagged, err := s.q.FlagQueue(ctx)
	if err != nil {
		return out, err
	}
	pending, err := s.q.PendingQueue(ctx)
	if err != nil {
		return out, err
	}
	counts, err := s.q.ModerationQueueCounts(ctx)
	if err != nil {
		return out, err
	}

	recs := make([]moderationRecord, 0, len(review)+len(flagged)+len(pending))
	for _, r := range review {
		recs = append(recs, recordFromReview(r))
	}
	for _, r := range flagged {
		recs = append(recs, recordFromFlag(r))
	}
	for _, r := range pending {
		recs = append(recs, recordFromPending(r))
	}
	rows, err := s.buildRows(ctx, recs)
	if err != nil {
		return out, err
	}

	out.Review = rows[:len(review)]
	out.Flagged = rows[len(review) : len(review)+len(flagged)]
	out.Pending = rows[len(review)+len(flagged):]
	out.Counts.Review = counts.ReviewCount
	out.Counts.Flagged = counts.FlaggedCount
	out.Counts.Pending = counts.PendingCount
	return out, nil
}

// act is the shared body of Keep and Remove: the status lookup, the decision,
// the audit row, the state change and the re-read, in that order.
func (s *ModerationService) act(ctx context.Context, id, moderatorID int64, action string) (ModerationRow, error) {
	status, err := s.q.ModerationStatusForReport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModerationRow{}, ErrReportNotFound
		}
		return ModerationRow{}, err
	}
	switch action {
	case ModerationActionKeep:
		if !status.InReview && !status.InFlagged && !status.InPending {
			return ModerationRow{}, ErrAlreadyHandled
		}
	case ModerationActionRemove:
		if ModerationState(status.ModerationState) == ModerationRemoved {
			return ModerationRow{}, ErrAlreadyHandled
		}
	}

	if err := s.q.InsertModerationAction(ctx, sqlcgen.InsertModerationActionParams{
		ReportID: id, ModeratorAccountID: moderatorID, Action: action,
	}); err != nil {
		return ModerationRow{}, err
	}
	if action == ModerationActionKeep {
		err = s.q.KeepReport(ctx, id)
	} else {
		err = s.q.RemoveReport(ctx, id)
	}
	if err != nil {
		return ModerationRow{}, err
	}

	row, err := s.q.ReportForModeration(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModerationRow{}, ErrReportNotFound
		}
		return ModerationRow{}, err
	}
	built, err := s.buildRows(ctx, []moderationRecord{recordFromDetail(row)})
	if err != nil {
		return ModerationRow{}, err
	}
	return built[0], nil
}

// Keep clears the automatic flag, dismisses the open people flags and releases
// a pending report into the normal rules. See the steps on ModerationService.
func (s *ModerationService) Keep(ctx context.Context, id, moderatorID int64) (ModerationRow, error) {
	return s.act(ctx, id, moderatorID, ModerationActionKeep)
}

// Remove withholds any report from every feed, map, vote and history view. See
// the steps on ModerationService.
func (s *ModerationService) Remove(ctx context.Context, id, moderatorID int64) (ModerationRow, error) {
	return s.act(ctx, id, moderatorID, ModerationActionRemove)
}

// Find reads one report by id and the lists it is in, in the order review,
// flagged, pending. The list slice is never nil.
func (s *ModerationService) Find(ctx context.Context, id int64) (ModerationRow, []string, error) {
	row, err := s.q.ReportForModeration(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModerationRow{}, nil, ErrReportNotFound
		}
		return ModerationRow{}, nil, err
	}
	status, err := s.q.ModerationStatusForReport(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ModerationRow{}, nil, ErrReportNotFound
		}
		return ModerationRow{}, nil, err
	}
	built, err := s.buildRows(ctx, []moderationRecord{recordFromDetail(row)})
	if err != nil {
		return ModerationRow{}, nil, err
	}
	lists := []string{}
	if status.InReview {
		lists = append(lists, ModListReview)
	}
	if status.InFlagged {
		lists = append(lists, ModListFlagged)
	}
	if status.InPending {
		lists = append(lists, ModListPending)
	}
	return built[0], lists, nil
}
