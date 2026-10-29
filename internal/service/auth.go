package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"pinalert/internal/mailer"
	sqlcgen "pinalert/internal/store/sqlc"
)

// ResendCooldown is the minimum time a caller must wait between successive
// emails to the same address (D-03: the midpoint of the 30-60 second range
// CONTEXT.md leaves to Claude's discretion). Exported so handlers and
// templates share one source of truth rather than each hard-coding the
// number separately. CredentialService reads it as its default send cooldown.
const ResendCooldown = 45 * time.Second

// AuthQuerier is the subset of the sqlc-generated Queries type AuthService
// needs (the Activity page reads), small enough to fake in a test without a
// real Postgres. Amended 2026-10-29 by Phase 7.1 plan 14b: the emailed link
// methods are gone with the link.
type AuthQuerier interface {
	ReportsByAccount(ctx context.Context, accountID *int64) ([]sqlcgen.ReportsByAccountRow, error)
	// CurrentVotesForReports backs ActivityForAccount (02-07). Signature
	// copied verbatim from trust.go's VotingQuerier — the same generated
	// method, named a second time by a second consumer-defined interface
	// (idiomatic Go: each interface stays small enough to fake without a
	// real Postgres, deliberately not deduplicated into a shared one).
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	// ReporterHistoryReports gives the Activity page the account's bounded
	// history, so every row carries the account's reporter signal.
	ReporterHistoryReports(ctx context.Context, arg sqlcgen.ReporterHistoryReportsParams) ([]sqlcgen.ReporterHistoryReportsRow, error)
	// VotedReportsByAccount gives the Activity page the account's current
	// content vote per report (quick task 261013-wm5). A second consumer
	// defined interface naming one generated method, as the two above do.
	VotedReportsByAccount(ctx context.Context, arg sqlcgen.VotedReportsByAccountParams) ([]sqlcgen.VotedReportsByAccountRow, error)
	// ReporterAccountsForReports gives the Activity page the reporter of each
	// voted report, so its trust carries that reporter's signal and not the
	// viewer's. A second consumer defined interface naming one generated
	// method, with the signature ReportQuerier names.
	ReporterAccountsForReports(ctx context.Context, reportIds []int64) ([]sqlcgen.ReporterAccountsForReportsRow, error)
	// ResolutionUndoCounts gives the Activity page how many take backs and
	// reopens the account has written on its own closed posts, so the page can
	// stop offering a Reopen the server would refuse (plan 03.1-07, D-22). A
	// consumer defined interface naming the generated method VotingQuerier names.
	ResolutionUndoCounts(ctx context.Context, arg sqlcgen.ResolutionUndoCountsParams) ([]sqlcgen.ResolutionUndoCountsRow, error)
	// ReportExtrasForReports gives ActivityPageForAccount (which
	// ActivityForAccount wraps) the moderation state and the display only
	// written time of the page's reports, in one batched read (R-01).
	ReportExtrasForReports(ctx context.Context, arg sqlcgen.ReportExtrasForReportsParams) ([]sqlcgen.ReportExtrasForReportsRow, error)
	// PublicProfilePathsForViewer gives the voted rows of the Activity page the
	// profile path of an opted in reporter who has not blocked this account
	// (plan 09-09, D-04). One batched read, skipped when no voted report has a
	// reporter. A consumer defined interface naming the generated method
	// ReportQuerier names.
	PublicProfilePathsForViewer(ctx context.Context, arg sqlcgen.PublicProfilePathsForViewerParams) ([]sqlcgen.PublicProfilePathsForViewerRow, error)
}

// ErrRateLimited is returned by CredentialService when a second email request
// for the same normalised address arrives inside the send cooldown, or the
// hourly count is spent (D-03/D-04, IDENT-04). It is a distinct sentinel,
// never a ValidationError, so the handler maps it to 429 (errors.Is), not
// 400. Declared via errors.New rather than as a typed error since no
// additional data ever needs to travel with it.
var ErrRateLimited = errors.New("service: rate limited")

// AuthService serves the Activity page reads (the account's own posts and the
// reports it votes on). Amended 2026-10-29 by Phase 7.1 plan 14b: it no
// longer mints, stores or mails a sign in link; sign in is CredentialService.
// The mailer and baseURL fields are kept only so NewAuthService's signature
// and its call sites stay unchanged, and nothing reads them.
type AuthService struct {
	q       AuthQuerier
	mailer  mailer.Mailer
	baseURL string
	now     func() time.Time

	// signals supplies extra reporter tag inputs (plan 09-13). Nil reads outcomes only.
	signals SignalSource
}

// AuthOption configures an AuthService at construction time.
type AuthOption func(*AuthService)

// WithAuthSignals hands the service the source of extra reporter tag inputs
// (plan 09-13 is the first real one), so Activity shows the same tag as the
// feed. With none it reads outcomes only.
func WithAuthSignals(src SignalSource) AuthOption {
	return func(s *AuthService) {
		s.signals = src
	}
}

// WithClock overrides the AuthService's clock. Tests use this to advance
// time without real sleeps; production code never needs to call it. The
// seam is introduced here — rather than added later alongside plan
// 01.1-05's cooldown logic — because internal/service/auth_test.go lives in
// the external service_test package (matching report_test.go) and so
// cannot reach an unexported field after the fact; adding it now means that
// later plan changes no constructor signature and breaks no call site.
func WithClock(fn func() time.Time) AuthOption {
	return func(s *AuthService) {
		s.now = fn
	}
}

// NewAuthService constructs an AuthService backed by q. The m and baseURL
// arguments are kept for call site stability and are no longer used. The
// clock defaults to time.Now() unless overridden via WithClock.
func NewAuthService(q AuthQuerier, m mailer.Mailer, baseURL string, opts ...AuthOption) *AuthService {
	s := &AuthService{q: q, mailer: m, baseURL: baseURL, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ReportsForAccount returns every report filed by any session ever bound to
// accountID, newest first — the profile page's core read (01.1-07-PLAN.md).
// Aggregation stays in the service layer rather than the handler, matching
// how ReportService owns the nearby query: sqlcgen.ReportsByAccountRow is a
// generated type callers should not need to know about directly, and a
// second caller (Phase 2's own profile-shaped read, if one ever exists)
// gets this same seam rather than a second hand-rolled query. accountID is
// always non-nil in practice — it's the ID of an account the request's
// gate has already resolved via a real database row (internal/api/gate.go)
// — but the underlying column is nullable (sessions.account_id), which is
// why sqlcgen.Queries.ReportsByAccount itself takes a pointer.
func (s *AuthService) ReportsForAccount(ctx context.Context, accountID int64) ([]sqlcgen.ReportsByAccountRow, error) {
	rows, err := s.q.ReportsByAccount(ctx, &accountID)
	if err != nil {
		return nil, fmt.Errorf("service: reading reports for account %d: %w", accountID, err)
	}
	return rows, nil
}

// ActivityReport is one Activity-page row: the ReportsByAccount row
// (unmodified) plus the VALIDATED category and severity and the resolved
// visibility and reason. Category/Severity are separate fields rather than
// an embed — embedding sqlcgen.ReportsByAccountRow would collide its own
// raw string Category/Severity fields with these validated typed ones.
type ActivityReport struct {
	Row        sqlcgen.ReportsByAccountRow
	Category   Category
	Severity   Severity
	Visibility Visibility
	Reason     ResolveReason
	// Trust comes from the same BuildTrust as the feed and the vote response
	// (D-21).
	Trust Trust
	// UndosLeft is how many undos the poster has left on their own post (D-22):
	// UndoLimit unless the post is closed and not yet faded, and then the budget
	// less the unresolve and reopen rows the poster wrote. Zero means the
	// Reopen control must not be offered. It is only meaningful for an own post.
	UndosLeft int
	// Approximate is true when Row was blurred by the finished Rescue rule
	// (D-27): the point is the coarse cell centre, the geohash is the coarse
	// cell and the description is empty. Only another person's finished Rescue
	// report on the voted list is ever approximate; own posts stay exact.
	Approximate bool
	// Moderation is the stored state, carried so 04-16 can label held and
	// removed rows. A withheld row carries Visibility withheld with reason
	// awaiting_text_check or removed_after_review, and the zero Trust.
	Moderation ModerationState
	// WrittenAt is WrittenAtForDisplay's answer: the client written time when
	// the upload came more than WrittenLabelThreshold after it, else nil. It is
	// display only (R-05).
	WrittenAt *time.Time
	// ReporterProfilePath is the "/u/username" path of the reporter of a voted
	// report, set only while the report is live (Active or Disputed) and only
	// for an opted in reporter with a clear username who has not blocked this
	// account (D-04, D-12, D-20). Empty on own posts and on every finished row.
	// It sits beside Trust, never inside it.
	ReporterProfilePath string
}

// VotedReport is one report the account currently confirms or disputes: the
// ActivityReport built exactly like a feed row (its own reporter and that
// reporter's signal), plus the account's latest content vote on it and when it
// was cast (D-02, latest wins). The embedded Row is the report's own columns,
// copied from the votes read into the ReportsByAccountRow shape so one handler
// mapper serves both lists.
type VotedReport struct {
	ActivityReport
	Vote    VoteValue
	VotedAt time.Time
}

// ActivityPage is everything the Activity page lists for one account: its own
// posts, newest first, and the reports it currently votes on, newest vote
// first. A report is on exactly one list (a post is never also a vote).
type ActivityPage struct {
	Posts []ActivityReport
	Votes []VotedReport
}

// validatedActivityMeta applies the one fallback both
// Activity lists share, so a post and a voted report can never disagree about
// an unknown value (T-01-17): other for a category, low for a severity.
func validatedActivityMeta(rawCategory, rawSeverity string) (Category, Severity) {
	category := Category(rawCategory)
	if !category.Valid() {
		category = CategoryOther
	}
	severity := Severity(rawSeverity)
	if !severity.Valid() {
		severity = SeverityLow
	}
	return category, severity
}

// ActivityForAccount returns the account's own posts, newest first. It is the
// posts half of ActivityPageForAccount, kept for callers that need only that
// list, so every caller exercises the page's real read path.
func (s *AuthService) ActivityForAccount(ctx context.Context, accountID int64) ([]ActivityReport, error) {
	page, err := s.ActivityPageForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return page.Posts, nil
}

// ActivityPageForAccount is the Activity page's one pass read (02-07, extended
// by quick task 261013-wm5): the account's own posts (ReportsForAccount, the
// existing unmodified query) and the reports it currently confirms or
// disputes (VotedReportsByAccount), then ONE ReporterHistoryReports call and
// ONE CurrentVotesForReports call over the whole page, then BuildVoteTally +
// Resolve + BuildTrust per report. The same batched shape Nearby uses keeps
// the page at a constant number of round trips (V-29), and the resolver stays
// the single authority on visibility (T-02-04).
//
// Order of reads: posts, votes, then (when voted reports remain)
// ReporterAccountsForReports once over the voted ids, then history once for
// the viewer (when it has posts) and the voted reports' reporters, then votes
// once over post ids, voted ids and history ids, then ReportExtrasForReports
// once over the post ids and the voted ids for the moderation state and the
// display only written time (R-01). A report that read did not return is
// skipped, and a voted report that is withheld is dropped from the votes. An
// account with nothing to list makes only the first two reads.
//
// Each post carries the account's own reporter signal. Each voted report
// carries ITS reporter's signal, never the viewer's, so a voted card's trust
// equals the feed's trust for the same report, clock and rows (D-21). A report
// that is the account's own post is dropped from the votes (the self vote
// block makes that impossible today), and so is any vote row whose value is
// not a content value.
//
// Category and severity are validated ONCE, here, rather than in the handler:
// the fallback this function computes is the SAME value the resolver sees, so
// the rendered class name and the resolved trust state can never disagree
// (T-01-17). The account id is only ever the verified account's own (T-wm5-01).
func (s *AuthService) ActivityPageForAccount(ctx context.Context, accountID int64) (ActivityPage, error) {
	rows, err := s.ReportsForAccount(ctx, accountID)
	if err != nil {
		return ActivityPage{}, err
	}
	votedRows, err := s.q.VotedReportsByAccount(ctx, sqlcgen.VotedReportsByAccountParams{
		AccountID: accountID,
		Kind:      string(VoteKindContent),
	})
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading voted reports for account %d: %w", accountID, err)
	}

	postIDs := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		postIDs[row.ID] = struct{}{}
	}
	voted := make([]sqlcgen.VotedReportsByAccountRow, 0, len(votedRows))
	for _, row := range votedRows {
		if _, own := postIDs[row.ID]; own {
			continue
		}
		if !VoteValue(row.VoteValue).ValidFor(VoteKindContent) {
			continue
		}
		voted = append(voted, row)
	}
	if len(rows) == 0 && len(voted) == 0 {
		return ActivityPage{Posts: []ActivityReport{}, Votes: []VotedReport{}}, nil
	}

	// now is captured once, before the batched reads, because the history read
	// takes it as a parameter and ReporterSignals must see the same one.
	now := s.now().UTC()

	ids := make([]int64, 0, len(rows)+len(voted))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	votedIDs := make([]int64, 0, len(voted))
	for _, row := range voted {
		votedIDs = append(votedIDs, row.ID)
		ids = append(ids, row.ID)
	}

	reporterByReport := make(map[int64]*int64, len(voted))
	if len(voted) > 0 {
		reporterRows, err := s.q.ReporterAccountsForReports(ctx, votedIDs)
		if err != nil {
			return ActivityPage{}, fmt.Errorf("service: reading reporters of voted reports for account %d: %w", accountID, err)
		}
		for _, rr := range reporterRows {
			reporterByReport[rr.ReportID] = rr.ReporterAccountID
		}
	}

	var accounts []int64
	if len(rows) > 0 {
		accounts = append(accounts, accountID)
	}
	for _, id := range pageReporterAccounts(votedIDs, reporterByReport) {
		if id == accountID {
			continue
		}
		accounts = append(accounts, id)
	}

	var history []sqlcgen.ReporterHistoryReportsRow
	if len(accounts) > 0 {
		history, err = s.q.ReporterHistoryReports(ctx, reporterHistoryParams(accounts, now))
		if err != nil {
			return ActivityPage{}, fmt.Errorf("service: reading reporter history for account %d: %w", accountID, err)
		}
	}

	// In production the history ids of the viewer are a subset of the post ids,
	// because both reads key on the same account, but the union keeps one rule
	// across paths.
	voteRows, err := s.q.CurrentVotesForReports(ctx, voteReadIDs(ids, history))
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading current votes for account %d's page: %w", accountID, err)
	}
	signals, err := reporterSignalsWith(ctx, s.signals, accounts, history, voteRows, now)
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading reporter signals for account %d: %w", accountID, err)
	}
	votesByReport := voteRowsByReport(voteRows)

	// One batched read of the moderation state and the display only written
	// time over the own post ids and then the voted ids, never the history ids
	// (R-01, D-19). A report it did not return is skipped below: fail closed.
	extraRows, err := s.q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{ReportIds: ids, ViewerAccountID: accountID})
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading moderation extras for account %d's reports: %w", accountID, err)
	}
	extrasByReport := make(map[int64]sqlcgen.ReportExtrasForReportsRow, len(extraRows))
	for _, ex := range extraRows {
		extrasByReport[ex.ReportID] = ex
	}

	// Which reporters of the voted reports have a profile this account may open
	// (D-04): one batched read. Own posts need none.
	profilePaths, err := loadProfilePaths(ctx, s.q, accountID, reporterByReport)
	if err != nil {
		return ActivityPage{}, fmt.Errorf("service: reading profile paths for account %d's page: %w", accountID, err)
	}

	posts := make([]ActivityReport, 0, len(rows))
	// closedOwn holds the ids of own posts that are closed and not yet faded:
	// the only posts whose Reopen the page can offer, so the only ones whose
	// undo count is read (D-28).
	var closedOwn []int64
	for _, row := range rows {
		ex, ok := extrasByReport[row.ID]
		if !ok {
			continue
		}
		category, severity := validatedActivityMeta(row.Category, row.Severity)

		meta := ReportMeta{Severity: severity, Category: category, Moderation: ModerationState(ex.ModerationState)}
		// The viewing account is always a post's reporter (D-16's whole reason
		// the Reopen path needs no identity check at all).
		tally := BuildVoteTally(voteRows, row.ID, &accountID)
		vis, reason := Resolve(meta, tally, now)
		if vis == VisibilityRetracted && row.ExpiresAt.After(now) {
			closedOwn = append(closedOwn, row.ID)
		}

		// A withheld row keeps no trust object: it is not public, so there is
		// nothing to say about who backs it. A pending serious report is public
		// and keeps its trust like any published row (D-02).
		var trust Trust
		if !meta.Withheld() {
			trust = BuildTrust(TrustInput{
				Rows:              voteRows,
				ReportID:          row.ID,
				ReporterAccountID: &accountID,
				Meta:              meta,
				Visibility:        vis,
				CreatedAt:         row.CreatedAt,
				ExpiresAt:         row.ExpiresAt,
				Now:               now,
				Reporter:          reporterSignalRef(signals, &accountID),
			})
		}

		posts = append(posts, ActivityReport{
			Row:        row,
			Category:   category,
			Severity:   severity,
			Visibility: vis,
			Reason:     reason,
			UndosLeft:  UndoLimit,
			Trust:      trust,
			Moderation: meta.Moderation,
			WrittenAt:  WrittenAtForDisplay(row.CreatedAt, writtenAtPtr(ex.ClientWrittenAt)),
		})
	}

	// One batched read of the poster's undo counts for the closed own posts
	// (D-22, D-28). The account is always the poster of its own posts, so the
	// unresolve and reopen rows both spend the budget (UndosUsed, isPoster).
	if len(closedOwn) > 0 {
		undoRows, err := s.q.ResolutionUndoCounts(ctx, undoCountParams(accountID, closedOwn))
		if err != nil {
			return ActivityPage{}, fmt.Errorf("service: reading undo counts for account %d: %w", accountID, err)
		}
		used := make(map[int64]int, len(undoRows))
		for _, ur := range undoRows {
			used[ur.ReportID] = UndosUsed(ur, true)
		}
		closed := make(map[int64]struct{}, len(closedOwn))
		for _, id := range closedOwn {
			closed[id] = struct{}{}
		}
		for i := range posts {
			if _, ok := closed[posts[i].Row.ID]; ok {
				posts[i].UndosLeft = UndosLeft(used[posts[i].Row.ID])
			}
		}
	}

	votes := make([]VotedReport, 0, len(voted))
	for _, v := range voted {
		ex, ok := extrasByReport[v.ID]
		if !ok {
			continue
		}
		category, severity := validatedActivityMeta(v.Category, v.Severity)
		reporterID := reporterByReport[v.ID]
		reportVotes := votesByReport[v.ID]

		meta := ReportMeta{Severity: severity, Category: category, Moderation: ModerationState(ex.ModerationState)}
		// Another person's withheld report is never listed here: only its
		// author's own page shows it (R-01).
		if meta.Withheld() {
			continue
		}
		tally := BuildVoteTally(reportVotes, v.ID, reporterID)
		vis, reason := Resolve(meta, tally, now)

		row := sqlcgen.ReportsByAccountRow{
			ID:                    v.ID,
			Category:              v.Category,
			Severity:              v.Severity,
			Description:           v.Description,
			Latitude:              v.Latitude,
			Longitude:             v.Longitude,
			Geohash:               v.Geohash,
			ShelterCapacityStatus: v.ShelterCapacityStatus,
			ShelterHeadcount:      v.ShelterHeadcount,
			CreatedAt:             v.CreatedAt,
			ExpiresAt:             v.ExpiresAt,
		}
		// D-27: a voted card of another person's finished Rescue report is
		// blurred the way the Past tab blurs it (viewer independent). Finished
		// is the resolved or past view, so a closed or a faded report is blurred
		// and a live one is not. Every leaking field changes together here.
		approximate := false
		if category == CategoryRescueNeeded {
			if view := ViewOf(vis, v.ExpiresAt, now); view == FeedViewResolved || view == FeedViewPast {
				row.Latitude, row.Longitude, row.Geohash = CoarsePoint(v.Latitude, v.Longitude)
				row.Description = ""
				approximate = true
			}
		}
		// The profile link only while the report is live (D-20): a finished row
		// tied to a username could be matched to its exact point.
		profilePath := ""
		if view := ViewOf(vis, v.ExpiresAt, now); (view == FeedViewActive || view == FeedViewDisputed) && reporterID != nil {
			profilePath = profilePaths[*reporterID]
		}

		votes = append(votes, VotedReport{
			ActivityReport: ActivityReport{
				Row:                 row,
				Approximate:         approximate,
				UndosLeft:           UndoLimit,
				Category:            category,
				Severity:            severity,
				Visibility:          vis,
				Reason:              reason,
				Moderation:          meta.Moderation,
				WrittenAt:           WrittenAtForDisplay(v.CreatedAt, writtenAtPtr(ex.ClientWrittenAt)),
				ReporterProfilePath: profilePath,
				Trust: BuildTrust(TrustInput{
					Rows:              reportVotes,
					ReportID:          v.ID,
					ReporterAccountID: reporterID,
					Meta:              meta,
					Visibility:        vis,
					CreatedAt:         v.CreatedAt,
					ExpiresAt:         v.ExpiresAt,
					Now:               now,
					Reporter:          reporterSignalRef(signals, reporterID),
				}),
			},
			Vote:    VoteValue(v.VoteValue),
			VotedAt: v.VotedAt,
		})
	}
	sort.SliceStable(votes, func(i, j int) bool {
		if !votes[i].VotedAt.Equal(votes[j].VotedAt) {
			return votes[i].VotedAt.After(votes[j].VotedAt)
		}
		return votes[i].Row.ID > votes[j].Row.ID
	})

	return ActivityPage{Posts: posts, Votes: votes}, nil
}

// normalizeEmail trims rawEmail, parses it with the stdlib address parser
// (RESEARCH.md "Don't Hand-Roll" — no hand-rolled regex), rejects a display-name form
// such as "Someone <a@b.com>" since only a bare address is accepted, and
// lowercases the result so one human's address is one identity regardless
// of how they typed it.
func normalizeEmail(rawEmail string) (string, error) {
	trimmed := strings.TrimSpace(rawEmail)
	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Name != "" {
		return "", ValidationError{Field: "email", Message: "Enter a valid email address."}
	}
	return strings.ToLower(addr.Address), nil
}

// NormalizeEmail is the exported name of normalizeEmail. It exists so the
// moderator list (D-08) is normalised by the same rule sign in uses, so one
// person is one identity in both places.
func NormalizeEmail(raw string) (string, error) {
	return normalizeEmail(raw)
}
