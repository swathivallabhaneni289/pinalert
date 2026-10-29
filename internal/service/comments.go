package service

// comments.go is the rule of a comment on a report page (Phase 8 plan 09,
// PAGE-02, D-02, D-12 to D-16). CommentService posts, lists, deletes, likes and
// flags comments. It has no route (plan 08-12), sends no notification (plan
// 08-11 uses the notifier stored here) and runs no background check (plan
// 08-14 settles the held comments).
//
// Who may comment: any signed in account that has a username, from anywhere.
// There is deliberately no distance check, unlike a vote. A remote person can
// therefore post a rumour as a comment, which is why comments never feed a
// trust number, are screened, are rate limited and can be flagged, and why the
// page keeps the trust block above them. Likes never touch a trust number
// either (D-02), which TestLikesNeverFeedTrust pins.
//
// The ordered checks of Post are the specification:
//
//  1. CleanCommentBody: the text rule (empty, characters, 280 code points).
//  2. The account has a username. requireOnboarded lets every /api/ path
//     through, so the route cannot rely on it (Phase 7.1 seam).
//  3. The report context is read once; no row is ErrReportNotFound.
//  4. A withheld report, or one that ended at or before now minus
//     HistoryWindow, is ErrReportNotFound (the rule of Focus, review W5).
//  5. A report that is not active or disputed is ErrCommentsClosed.
//  6. A reply target must be on the same report, clear and not deleted, else
//     ErrCommentGone. The thread root is the target's own root, so a reply to a
//     reply joins the root's thread (D-02).
//  7. ScreenAbuse with the stored screener, never the gibberish prefilter. A
//     refused text is CommentRejectedError and nothing is stored. Comments have
//     no critical exemption (D-12).
//  8. InsertComment with the state the outcome gives: clear for clear, clear
//     with needs_review for borderline, pending for an unfinished check.
//
// Withheld comes before closed and before screening, so a hidden report costs
// no provider call and reveals nothing. Like, Unlike and Flag read the comment
// first and then run the same report check on the comment's own report. Delete
// skips the report check on purpose: you can delete your own words at any time
// (D-14).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "pinalert/internal/store/sqlc"
)

const (
	// CommentPageSize is how many top level comments one List returns.
	CommentPageSize = 50
	// CommentReplyCap is a total bound on the reply rows one List returns.
	CommentReplyCap = 500
)

var (
	// ErrCommentsClosed is returned by Post, Like and Unlike when the report is
	// resolved or has faded (D-15).
	ErrCommentsClosed = errors.New("service: comments are closed")
	// ErrCommentNeedsUsername is returned when the account has no username.
	ErrCommentNeedsUsername = errors.New("service: comment needs a username")
	// ErrCommentGone is returned for an action on a comment that is deleted,
	// removed, held, unknown or on another report.
	ErrCommentGone = errors.New("service: comment is gone")
	// ErrCommentNotYours is returned when someone deletes another person's
	// comment, the reporter included.
	ErrCommentNotYours = errors.New("service: comment belongs to someone else")
	// ErrCommentOwnFlag is returned when an account flags its own comment.
	ErrCommentOwnFlag = errors.New("service: cannot flag your own comment")
	// ErrCommentRejected is the sentinel a refused comment wraps. Nothing was
	// stored.
	ErrCommentRejected = errors.New("service: comment rejected by the text check")
)

// CommentRejectedError is returned by Post when the text check refused the
// comment. RejectMessage(Reason) is the message to show.
type CommentRejectedError struct {
	Reason RejectReason
}

func (e CommentRejectedError) Error() string {
	return fmt.Sprintf("service: comment rejected (%s)", e.Reason)
}

// Unwrap lets errors.Is(err, ErrCommentRejected) match.
func (e CommentRejectedError) Unwrap() error {
	return ErrCommentRejected
}

// CommentQuerier is the store surface of CommentService: the report reads that
// decide who may see a report and whether it is open, plus the comment queries.
type CommentQuerier interface {
	ReportVoteContext(ctx context.Context, reportID int64) (sqlcgen.ReportVoteContextRow, error)
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReportExtrasForReports(ctx context.Context, arg sqlcgen.ReportExtrasForReportsParams) ([]sqlcgen.ReportExtrasForReportsRow, error)
	InsertComment(ctx context.Context, arg sqlcgen.InsertCommentParams) (sqlcgen.InsertCommentRow, error)
	CommentTarget(ctx context.Context, commentID int64) (sqlcgen.CommentTargetRow, error)
	CommentsForReport(ctx context.Context, arg sqlcgen.CommentsForReportParams) ([]sqlcgen.CommentsForReportRow, error)
	MarkCommentDeleted(ctx context.Context, arg sqlcgen.MarkCommentDeletedParams) (int64, error)
	LikeComment(ctx context.Context, arg sqlcgen.LikeCommentParams) (int64, error)
	UnlikeComment(ctx context.Context, arg sqlcgen.UnlikeCommentParams) (int64, error)
	CommentLikeState(ctx context.Context, arg sqlcgen.CommentLikeStateParams) (sqlcgen.CommentLikeStateRow, error)
	InsertCommentFlag(ctx context.Context, arg sqlcgen.InsertCommentFlagParams) (int64, error)
}

var _ CommentQuerier = (*sqlcgen.Queries)(nil)

// CommentService applies the comment rules.
type CommentService struct {
	q        CommentQuerier
	screener TextScreener
	now      func() time.Time
	notifier ReportChangeNotifier
}

// CommentOption configures a CommentService.
type CommentOption func(*CommentService)

// WithCommentScreener sets the text screener every comment and comment flag
// note goes through. It is the same screener the report description goes
// through. Without one, every text passes, as it does for a report.
func WithCommentScreener(s TextScreener) CommentOption {
	return func(cs *CommentService) { cs.screener = s }
}

// WithCommentClock overrides the clock. Tests only.
func WithCommentClock(fn func() time.Time) CommentOption {
	return func(cs *CommentService) {
		if fn != nil {
			cs.now = fn
		}
	}
}

// WithCommentChangeNotifier stores the notifier plan 08-11 calls after a
// comment is posted. Like, Unlike and Flag never call it (D-16).
func WithCommentChangeNotifier(n ReportChangeNotifier) CommentOption {
	return func(cs *CommentService) { cs.notifier = n }
}

// NewCommentService stores the querier. It never dereferences it.
func NewCommentService(q CommentQuerier, opts ...CommentOption) *CommentService {
	s := &CommentService{q: q, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CommentsOpen reports whether people may still post, reply and like on a
// report: only while its view is active or disputed. A withheld, resolved or
// faded report is closed (D-15).
func CommentsOpen(v Visibility, expiresAt, now time.Time) bool {
	switch ViewOf(v, expiresAt, now) {
	case FeedViewActive, FeedViewDisputed:
		return true
	default:
		return false
	}
}

// CommentInput is one post. AccountID is the verified account of the caller and
// nothing else. HasUsername is the caller's own fact that the account has a
// username. ReplyTo is the comment answered, or nil for a top level comment.
type CommentInput struct {
	ReportID, AccountID int64
	HasUsername         bool
	Words               string
	ReplyTo             *int64
}

// CommentView is one comment as a reader sees it. It carries no account id,
// email, session or cell: the author is a username, New member or Deleted
// account (A-P8-10). A deleted or removed comment that still holds visible
// replies is a placeholder: Deleted or Removed is set and Words, Author, likes
// and the viewer flags are empty (D-14, review W4).
type CommentView struct {
	ID                                                       int64
	Author, AuthorKind, ReplyToAuthor                        string
	IsOwn, IsReporter, CanDelete, ViewerLiked, ViewerFlagged bool
	Deleted, Removed, Held                                   bool
	Likes, ReplyCount                                        int
	At                                                       time.Time
	Words                                                    string
	Replies                                                  []CommentView
}

// CommentList is one page of a report's comments. Count is the real number of
// comments everyone can see, not the page length. NextAfter is the id of the
// last top level comment returned, for the next page.
type CommentList struct {
	Comments   []CommentView
	Count      int
	Open, More bool
	NextAfter  int64
}

// LikeState is the answer to a like or an unlike.
type LikeState struct {
	Likes       int
	ViewerLiked bool
}

// commentReport is what the report check learns about the report of a comment.
type commentReport struct {
	// open is true while people may post and like.
	open bool
	// hidden is true for a finished Rescue report, whose comments are not shown
	// at all because 03.1 D-23 removes its text.
	hidden bool
}

// reportFor runs the report check shared by Post, List, Like, Unlike and Flag:
// the report must exist, must not be withheld, must not have ended more than
// HistoryWindow ago and must still be listed in some view. These are the
// answers of Focus, which the timeline and Share follow (D-14, review W5). Every
// failure is ErrReportNotFound.
func (s *CommentService) reportFor(ctx context.Context, reportID int64, now time.Time) (commentReport, error) {
	rc, err := s.q.ReportVoteContext(ctx, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return commentReport{}, ErrReportNotFound
		}
		return commentReport{}, fmt.Errorf("service: comments: reading report %d: %w", reportID, err)
	}
	meta := ReportMeta{
		Severity:   Severity(rc.Severity),
		Category:   Category(rc.Category),
		Moderation: ModerationState(rc.ModerationState),
	}
	if meta.Withheld() {
		return commentReport{}, ErrReportNotFound
	}
	windowStart := now.Add(-HistoryWindow)
	if !rc.ExpiresAt.After(windowStart) {
		return commentReport{}, ErrReportNotFound
	}

	rows, err := s.q.CurrentVotesForReports(ctx, []int64{reportID})
	if err != nil {
		return commentReport{}, fmt.Errorf("service: comments: reading votes of report %d: %w", reportID, err)
	}
	tally := BuildVoteTally(rows, reportID, rc.ReporterAccountID)
	vis, _ := Resolve(meta, tally, now)
	view := ViewOf(vis, rc.ExpiresAt, now)
	switch view {
	case FeedViewNone:
		return commentReport{}, ErrReportNotFound
	case FeedViewResolved:
		closure := ClosureOf(rows, reportID, rc.ReporterAccountID)
		if !closure.At.After(windowStart) {
			return commentReport{}, ErrReportNotFound
		}
	}

	finished := view == FeedViewResolved || view == FeedViewPast
	_, blurred := CoarsenFinishedRescue(Report{Category: Category(rc.Category)}, finished, nil)
	return commentReport{
		open:   CommentsOpen(vis, rc.ExpiresAt, now),
		hidden: blurred,
	}, nil
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// Post stores one comment or reply and returns the author's own view of it. The
// order of the checks is in the file header.
func (s *CommentService) Post(ctx context.Context, in CommentInput) (CommentView, error) {
	body, err := CleanCommentBody(in.Words)
	if err != nil {
		return CommentView{}, err
	}
	if !in.HasUsername {
		return CommentView{}, ErrCommentNeedsUsername
	}
	now := s.now().UTC()

	rep, err := s.reportFor(ctx, in.ReportID, now)
	if err != nil {
		return CommentView{}, err
	}
	if !rep.open {
		return CommentView{}, ErrCommentsClosed
	}

	var parentID, replyToID *int64
	replyToAuthor := ""
	if in.ReplyTo != nil {
		target, err := s.q.CommentTarget(ctx, *in.ReplyTo)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CommentView{}, ErrCommentGone
			}
			return CommentView{}, fmt.Errorf("service: Post: reading comment %d: %w", *in.ReplyTo, err)
		}
		if target.ReportID != in.ReportID || commentTargetGone(target) {
			return CommentView{}, ErrCommentGone
		}
		root := target.ID
		if target.ParentID != nil {
			root = *target.ParentID
		}
		answered := target.ID
		parentID, replyToID = &root, &answered
		// The name of the person answered is shown only under the username rule
		// (review W3): a clear screen on an account that still exists.
		if target.AuthorUsername != nil && target.AuthorUsernameScreen != nil &&
			*target.AuthorUsernameScreen == "clear" && !target.AuthorDeleted {
			replyToAuthor = *target.AuthorUsername
		}
	}

	check := ScreenAbuse(ctx, s.screener, body)
	if check.Outcome == TextRejected {
		return CommentView{}, CommentRejectedError{Reason: check.Reason}
	}

	arg := sqlcgen.InsertCommentParams{
		ReportID:        in.ReportID,
		AccountID:       in.AccountID,
		ParentID:        parentID,
		ReplyToID:       replyToID,
		Body:            body,
		ModerationState: "clear",
	}
	if check.Outcome != TextUnfinished && (check.Model != "" || len(check.Scores) > 0) {
		verdict := "pass"
		if check.Outcome == TextBorderline {
			verdict = "borderline"
		}
		arg.Verdict = &verdict
		model := check.Model
		if model != "" {
			arg.Model = &model
		}
		if len(check.Scores) > 0 {
			raw, err := json.Marshal(check.Scores)
			if err != nil {
				return CommentView{}, fmt.Errorf("service: Post: encoding scores: %w", err)
			}
			arg.Scores = raw
		}
		arg.CheckedAt = timestamptz(now)
	}
	switch check.Outcome {
	case TextBorderline:
		arg.NeedsReview = true
	case TextUnfinished:
		arg.ModerationState = "pending"
		arg.NextAttemptAt = timestamptz(now)
	}

	row, err := s.q.InsertComment(ctx, arg)
	if err != nil {
		return CommentView{}, fmt.Errorf("service: Post: storing comment on report %d: %w", in.ReportID, err)
	}

	// The author's own name comes from the row just written, whatever its
	// screen: you always see your own username (A-P8-10).
	author := CommentAuthorNewMember
	own, err := s.q.CommentTarget(ctx, row.ID)
	if err != nil {
		return CommentView{}, fmt.Errorf("service: Post: reading comment %d back: %w", row.ID, err)
	}
	if own.AuthorUsername != nil {
		author = *own.AuthorUsername
	}

	return CommentView{
		ID:            row.ID,
		Author:        author,
		AuthorKind:    "user",
		ReplyToAuthor: replyToAuthor,
		IsOwn:         true,
		CanDelete:     true,
		Held:          check.Outcome == TextUnfinished,
		At:            row.CreatedAt,
		Words:         body,
	}, nil
}

// commentTargetGone reports whether a comment cannot be answered, liked or
// flagged: it is deleted, or it is not clear (held or removed). A held comment
// is visible to its author only, so even the author cannot act on it.
func commentTargetGone(t sqlcgen.CommentTargetRow) bool {
	return t.Deleted || t.ModerationState != "clear"
}

// List returns one page of the comments of a report for a viewer: top level
// comments oldest first, each with its replies. The report check is the same as
// Post's, so a withheld or too old report is ErrReportNotFound. A finished
// Rescue report lists nothing (03.1 D-23). What each viewer may see of each
// comment is decided once by CommentsForReport; this method only shapes it.
func (s *CommentService) List(ctx context.Context, reportID, viewerAccountID, after int64) (CommentList, error) {
	now := s.now().UTC()
	rep, err := s.reportFor(ctx, reportID, now)
	if err != nil {
		return CommentList{}, err
	}
	if rep.hidden {
		return CommentList{Comments: []CommentView{}}, nil
	}

	rows, err := s.q.CommentsForReport(ctx, sqlcgen.CommentsForReportParams{
		ViewerAccountID: viewerAccountID,
		ReportID:        reportID,
		AfterID:         after,
		RootCap:         CommentPageSize + 1,
		ReplyCap:        CommentReplyCap,
	})
	if err != nil {
		return CommentList{}, fmt.Errorf("service: List: reading comments of report %d: %w", reportID, err)
	}

	roots := make([]CommentView, 0, len(rows))
	rootIndex := make(map[int64]int)
	for _, row := range rows {
		v := commentViewFromRow(row, viewerAccountID)
		if row.ParentID == nil {
			rootIndex[row.ID] = len(roots)
			roots = append(roots, v)
			continue
		}
		i, ok := rootIndex[*row.ParentID]
		if !ok {
			continue
		}
		roots[i].Replies = append(roots[i].Replies, v)
		roots[i].ReplyCount++
	}

	out := CommentList{Open: rep.open, Comments: roots}
	if len(roots) > CommentPageSize {
		out.Comments = roots[:CommentPageSize]
		out.More = true
	}
	if n := len(out.Comments); n > 0 {
		out.NextAfter = out.Comments[n-1].ID
	}

	extras, err := s.q.ReportExtrasForReports(ctx, sqlcgen.ReportExtrasForReportsParams{
		ViewerAccountID: viewerAccountID,
		ReportIds:       []int64{reportID},
	})
	if err != nil {
		return CommentList{}, fmt.Errorf("service: List: reading the comment count of report %d: %w", reportID, err)
	}
	for _, e := range extras {
		if e.ReportID == reportID {
			out.Count = int(e.CommentCount)
		}
	}
	return out, nil
}

// commentViewFromRow shapes one CommentsForReport row. A placeholder carries no
// author, no words, no likes and no viewer flags whatever the row holds, so
// nothing about a deleted or removed comment can leak even if the query changes.
func commentViewFromRow(row sqlcgen.CommentsForReportRow, viewerAccountID int64) CommentView {
	v := CommentView{ID: row.ID, At: row.CreatedAt}
	switch row.Gone {
	case "deleted":
		v.Deleted = true
		return v
	case "removed":
		v.Removed = true
		return v
	}

	own := viewerAccountID != 0 && row.AccountID == viewerAccountID
	v.Words = row.Body
	v.IsOwn = own
	v.CanDelete = own
	v.IsReporter = row.IsReporter && !own
	v.Held = row.Held
	v.Likes = int(row.Likes)
	v.ViewerLiked = row.ViewerLiked
	v.ViewerFlagged = row.ViewerFlagged
	switch {
	case row.AuthorUsername != nil:
		v.Author = *row.AuthorUsername
		v.AuthorKind = "user"
	case row.AuthorKind == "deleted_account":
		v.Author = CommentAuthorDeletedAccount
		v.AuthorKind = row.AuthorKind
	default:
		v.Author = CommentAuthorNewMember
		v.AuthorKind = "new_member"
	}
	if row.ReplyToAuthor != nil && row.ParentID != nil {
		v.ReplyToAuthor = *row.ReplyToAuthor
	}
	return v
}

// Delete lets an account delete its own comment, at any time, whatever the state
// of the report (D-14). There is no edit. The reporter cannot delete other
// people's comments. A second delete of the same comment returns nil.
func (s *CommentService) Delete(ctx context.Context, commentID, accountID int64) error {
	n, err := s.q.MarkCommentDeleted(ctx, sqlcgen.MarkCommentDeletedParams{
		Now:       timestamptz(s.now().UTC()),
		CommentID: commentID,
		AccountID: accountID,
	})
	if err != nil {
		return fmt.Errorf("service: Delete: deleting comment %d: %w", commentID, err)
	}
	if n > 0 {
		return nil
	}
	target, err := s.q.CommentTarget(ctx, commentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCommentGone
		}
		return fmt.Errorf("service: Delete: reading comment %d: %w", commentID, err)
	}
	if target.AccountID != accountID {
		return ErrCommentNotYours
	}
	return nil
}

// actOnComment is the shared start of Like, Unlike and Flag. The order is part
// of the specification:
//
//  1. The comment is read. No row is ErrCommentGone.
//  2. The report of the comment gets the report check of Post, run on the
//     comment's own report, so a withheld, unknown or too old report is
//     ErrReportNotFound before anything about the comment is told, even for a
//     deleted comment.
//
// The caller then decides what a closed report and a gone comment mean for it.
func (s *CommentService) actOnComment(ctx context.Context, commentID int64) (sqlcgen.CommentTargetRow, commentReport, error) {
	target, err := s.q.CommentTarget(ctx, commentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlcgen.CommentTargetRow{}, commentReport{}, ErrCommentGone
		}
		return sqlcgen.CommentTargetRow{}, commentReport{}, fmt.Errorf("service: reading comment %d: %w", commentID, err)
	}
	rep, err := s.reportFor(ctx, target.ReportID, s.now().UTC())
	if err != nil {
		return sqlcgen.CommentTargetRow{}, commentReport{}, err
	}
	return target, rep, nil
}

// Like adds the account's like to a comment and returns the fresh count. One
// like per account per comment is the primary key, so a second Like changes
// nothing. Liking your own comment is allowed. A like touches no trust number
// and sends no notification (D-02, D-16). The checks run in the order of
// actOnComment, then a closed report is ErrCommentsClosed, then a comment that
// is deleted, removed or held (even your own held one) is ErrCommentGone.
func (s *CommentService) Like(ctx context.Context, commentID, accountID int64) (LikeState, error) {
	return s.setLike(ctx, commentID, accountID, true)
}

// Unlike takes the account's like back and returns the fresh count. It follows
// the same checks as Like.
func (s *CommentService) Unlike(ctx context.Context, commentID, accountID int64) (LikeState, error) {
	return s.setLike(ctx, commentID, accountID, false)
}

func (s *CommentService) setLike(ctx context.Context, commentID, accountID int64, like bool) (LikeState, error) {
	target, rep, err := s.actOnComment(ctx, commentID)
	if err != nil {
		return LikeState{}, err
	}
	if !rep.open {
		return LikeState{}, ErrCommentsClosed
	}
	if commentTargetGone(target) {
		return LikeState{}, ErrCommentGone
	}
	if like {
		_, err = s.q.LikeComment(ctx, sqlcgen.LikeCommentParams{CommentID: commentID, AccountID: accountID})
	} else {
		_, err = s.q.UnlikeComment(ctx, sqlcgen.UnlikeCommentParams{CommentID: commentID, AccountID: accountID})
	}
	if err != nil {
		return LikeState{}, fmt.Errorf("service: setting like on comment %d: %w", commentID, err)
	}
	st, err := s.q.CommentLikeState(ctx, sqlcgen.CommentLikeStateParams{CommentID: commentID, AccountID: accountID})
	if err != nil {
		return LikeState{}, fmt.Errorf("service: reading likes of comment %d: %w", commentID, err)
	}
	return LikeState{Likes: int(st.Likes), ViewerLiked: st.ViewerLiked}, nil
}

// Flag stores one flag on someone else's comment. It stays allowed on a closed
// report, because a flag is a safety action, and it never hides the comment
// (D-14). The order of the checks is the specification:
//
//  1. ValidateFlagInput: the reasons and the note length.
//  2. actOnComment: the comment, then the report check (ErrReportNotFound,
//     review W5).
//  3. A finished Rescue report shows no comments, and a comment that is
//     deleted, removed or held is not listed to anyone else: both are
//     ErrCommentGone.
//  4. Your own comment is ErrCommentOwnFlag.
//  5. A non empty note goes through ScreenAbuse; a refused note is
//     FlagNoteRejectedError and nothing is stored. A borderline or unfinished
//     check still stores the note, as a report flag does.
//  6. InsertCommentFlag; no row back is ErrAlreadyFlagged.
func (s *CommentService) Flag(ctx context.Context, commentID, accountID int64, in FlagInput) error {
	reasons, note, err := ValidateFlagInput(in)
	if err != nil {
		return err
	}
	target, rep, err := s.actOnComment(ctx, commentID)
	if err != nil {
		return err
	}
	if rep.hidden || commentTargetGone(target) {
		return ErrCommentGone
	}
	if target.AccountID == accountID {
		return ErrCommentOwnFlag
	}

	var notePtr *string
	if note != "" {
		check := ScreenAbuse(ctx, s.screener, note)
		if check.Outcome == TextRejected {
			return FlagNoteRejectedError{Reason: check.Reason}
		}
		notePtr = &note
	}

	slugs := make([]string, len(reasons))
	for i, r := range reasons {
		slugs[i] = string(r)
	}
	_, err = s.q.InsertCommentFlag(ctx, sqlcgen.InsertCommentFlagParams{
		CommentID: commentID,
		AccountID: accountID,
		Reasons:   slugs,
		Note:      notePtr,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAlreadyFlagged
		}
		return fmt.Errorf("service: Flag: storing flag on comment %d: %w", commentID, err)
	}
	return nil
}
