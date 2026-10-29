// commentmoderation.go is the moderator side of comments (Phase 8 plan 13,
// D-12, D-14): the list of comments that need a human decision, and Keep and
// Remove with their audit rows. It is a sibling of moderation.go, not a part of
// it, because the report side is keyed and locked by report while a comment
// carries its own check columns.

package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "pinalert/internal/store/sqlc"
)

// The reason words of the moderator list, one per way a comment gets there. A
// comment can carry several.
const (
	// CommentReasonFlagged marks a comment that people flagged.
	CommentReasonFlagged = "Flagged by people"
	// CommentReasonNearLine marks a comment the text check called near the line.
	CommentReasonNearLine = "Near the line"
	// CommentReasonCheckPending marks a held comment whose check did not finish.
	CommentReasonCheckPending = "Check not finished"
)

// The two server messages of the comment moderation API. They stay inside the
// closed character set the server copy test checks.
const (
	// ModCommentAlreadyHandledMessage answers a Keep on a comment that is in no
	// list any more.
	ModCommentAlreadyHandledMessage = "This comment was already handled. Refresh the lists."
	// ModCommentNotFoundMessage answers an id that is not a live comment.
	ModCommentNotFoundMessage = "No comment found with that id. Check the id and try again."
)

// CommentModerationQueueLimit bounds the moderator list of comments.
const CommentModerationQueueLimit = 200

// CommentModerationQuerier is the slice of the generated queries the comment
// moderation service uses, declared here in the consumer.
type CommentModerationQuerier interface {
	CommentModerationQueue(ctx context.Context, rowLimit int32) ([]sqlcgen.CommentModerationQueueRow, error)
	LockCommentForModeration(ctx context.Context, commentID int64) (sqlcgen.LockCommentForModerationRow, error)
	KeepComment(ctx context.Context, commentID int64) (int64, error)
	RemoveComment(ctx context.Context, commentID int64) (int64, error)
	DismissCommentFlags(ctx context.Context, commentID int64) (int64, error)
	InsertCommentModerationAction(ctx context.Context, arg sqlcgen.InsertCommentModerationActionParams) error
}

var _ CommentModerationQuerier = (*sqlcgen.Queries)(nil)

// CommentModerationTxFunc runs fn inside one database transaction with a
// querier bound to it, commits when fn returns nil and rolls back otherwise.
type CommentModerationTxFunc func(ctx context.Context, fn func(q CommentModerationQuerier) error) error

// PgxCommentModerationTx builds the production CommentModerationTxFunc over a
// pool.
func PgxCommentModerationTx(db TxBeginner) CommentModerationTxFunc {
	return func(ctx context.Context, fn func(q CommentModerationQuerier) error) error {
		return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
			return fn(sqlcgen.New(tx))
		})
	}
}

// CommentModerationItem is one comment as the moderator list shows it. It holds
// no author, flagger, email or account (T-08-65): a moderator reads the words
// and the report they sit on, nothing about who wrote them.
type CommentModerationItem struct {
	ID       int64
	ReportID int64
	// Category is the category slug of the comment's report.
	Category string
	// Words is the comment text as stored.
	Words string
	At    time.Time
	// Reasons are the CommentReason words that put the comment in the list, in
	// the order flagged, near the line, check not finished. Never nil.
	Reasons []string
	// FlagReasons are the dialog labels of the distinct reasons people gave,
	// in the dialog's order. Never nil, empty when nobody flagged.
	FlagReasons []string
	// Held is true for a comment whose check has not finished.
	Held bool
	// NeedsReview is true for a comment the check called near the line.
	NeedsReview bool
	// Flags is the number of open flags.
	Flags int
}

// CommentModerationService is the moderator side of comments (D-12, D-14).
//
// Steps of Queue: read the list once (CommentModerationQueue, bounded by
// CommentModerationQueueLimit) and name the reasons of each row.
//
// Steps of Keep and Remove, all inside one transaction (the tx func):
//  1. LockCommentForModeration takes the comment row lock and reads its state.
//     No row, a deleted comment or a removed one is ErrCommentGone. Keep of a
//     comment that is clear, not marked and not flagged has nothing to keep and
//     is ErrAlreadyHandled, so a double tap writes one audit row.
//  2. The change: KeepComment clears the review mark and releases a held
//     comment, RemoveComment withdraws the comment.
//  3. DismissCommentFlags closes the open flags.
//  4. InsertCommentModerationAction writes the audit row. A failure anywhere
//     rolls every write of the act back.
//
// After the commit, Keep of a comment that was held tells its recipient once
// through the notifier (the bell row of D-16; a second call is a no-op). A
// failure of that is logged with the comment id only and does not undo the Keep.
//
// Without a tx func (tests only) the same steps run on the plain querier.
type CommentModerationService struct {
	q        CommentModerationQuerier
	tx       CommentModerationTxFunc
	notifier CommentVisibleNotifier
}

// NewCommentModerationService builds the service. tx may be nil (tests only) and
// n may be nil when nobody needs telling.
func NewCommentModerationService(q CommentModerationQuerier, tx CommentModerationTxFunc, n CommentVisibleNotifier) *CommentModerationService {
	return &CommentModerationService{q: q, tx: tx, notifier: n}
}

// commentModerationReasons names why a row is in the list.
func commentModerationReasons(r sqlcgen.CommentModerationQueueRow) []string {
	out := make([]string, 0, 3)
	if r.OpenFlags > 0 {
		out = append(out, CommentReasonFlagged)
	}
	if r.NeedsReview {
		out = append(out, CommentReasonNearLine)
	}
	if r.Pending {
		out = append(out, CommentReasonCheckPending)
	}
	return out
}

// commentFlagLabels turns the stored flag reasons into dialog labels, in the
// FlagReasons order. A reason the dialog does not know is skipped.
func commentFlagLabels(stored []string) []string {
	have := make(map[string]struct{}, len(stored))
	for _, s := range stored {
		have[s] = struct{}{}
	}
	out := make([]string, 0, len(stored))
	for _, fr := range FlagReasons {
		if _, ok := have[string(fr)]; ok {
			out = append(out, FlagReasonLabel(fr))
		}
	}
	return out
}

// Queue returns the comments that need a decision, newest first. The slice is
// never nil.
func (s *CommentModerationService) Queue(ctx context.Context) ([]CommentModerationItem, error) {
	rows, err := s.q.CommentModerationQueue(ctx, CommentModerationQueueLimit)
	if err != nil {
		return nil, err
	}
	out := make([]CommentModerationItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, CommentModerationItem{
			ID:          r.ID,
			ReportID:    r.ReportID,
			Category:    r.Category,
			Words:       r.Body,
			At:          r.CreatedAt,
			Reasons:     commentModerationReasons(r),
			FlagReasons: commentFlagLabels(r.FlagReasons),
			Held:        r.Pending,
			NeedsReview: r.NeedsReview,
			Flags:       int(r.OpenFlags),
		})
	}
	return out, nil
}

// inTx runs fn inside the configured transaction, or on the plain querier when
// there is none.
func (s *CommentModerationService) inTx(ctx context.Context, fn func(q CommentModerationQuerier) error) error {
	if s.tx == nil {
		return fn(s.q)
	}
	return s.tx(ctx, fn)
}

// act is the shared body of Keep and Remove. It reports whether the comment was
// held when the act ran.
func (s *CommentModerationService) act(ctx context.Context, moderatorID, commentID int64, action string) (wasHeld bool, err error) {
	err = s.inTx(ctx, func(q CommentModerationQuerier) error {
		wasHeld = false
		st, err := q.LockCommentForModeration(ctx, commentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrCommentGone
			}
			return err
		}
		if st.Deleted || st.ModerationState == "removed" {
			return ErrCommentGone
		}
		if action == ModerationActionKeep && st.ModerationState == "clear" && !st.NeedsReview && st.OpenFlags == 0 {
			return ErrAlreadyHandled
		}
		wasHeld = st.ModerationState == "pending"

		var changed int64
		if action == ModerationActionKeep {
			changed, err = q.KeepComment(ctx, commentID)
		} else {
			changed, err = q.RemoveComment(ctx, commentID)
		}
		if err != nil {
			return err
		}
		if changed == 0 {
			return ErrCommentGone
		}
		if _, err := q.DismissCommentFlags(ctx, commentID); err != nil {
			return err
		}
		return q.InsertCommentModerationAction(ctx, sqlcgen.InsertCommentModerationActionParams{
			CommentID: commentID, ModeratorAccountID: moderatorID, Action: action,
		})
	})
	if err != nil {
		return false, err
	}
	return wasHeld, nil
}

// Keep clears the review mark of a comment, dismisses its open flags and
// releases a held comment into the normal rules. See the steps on
// CommentModerationService. ErrCommentGone is an unknown, deleted or removed
// comment; ErrAlreadyHandled is a comment that needs no decision.
func (s *CommentModerationService) Keep(ctx context.Context, moderatorID, commentID int64) error {
	wasHeld, err := s.act(ctx, moderatorID, commentID, ModerationActionKeep)
	if err != nil {
		return err
	}
	if wasHeld && s.notifier != nil {
		if err := s.notifier.NotifyCommentVisible(ctx, commentID); err != nil {
			log.Printf("service: CommentModerationService.Keep: notifying for comment %d: %v", commentID, err)
		}
	}
	return nil
}

// Remove withdraws a comment for everyone and dismisses its open flags. See the
// steps on CommentModerationService. ErrCommentGone is an unknown, deleted or
// already removed comment.
func (s *CommentModerationService) Remove(ctx context.Context, moderatorID, commentID int64) error {
	_, err := s.act(ctx, moderatorID, commentID, ModerationActionRemove)
	return err
}
