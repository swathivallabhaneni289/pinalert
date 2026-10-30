package service

import (
	"context"
	"errors"
)

// BlockableAuthor answers whose account a Block button on this comment would
// block, for the reader viewerID (quick 261030-u78, WR-E04). Every refusal is
// ErrCommentGone, so the answer cannot be used to probe a report, a comment, a
// private profile or who has blocked whom.
//
// The order is the specification:
//
//  1. actOnComment: no such comment, or any failure of the report check (an
//     unknown, withheld or too old report, or one listed in no view), is
//     ErrCommentGone. ErrReportNotFound is mapped to the same value so the
//     caller sees one refusal. A database error stays a wrapped error.
//  2. A finished Rescue report (rep.hidden), or a comment that is deleted,
//     removed or held (even the reader's own held one), is ErrCommentGone.
//  3. The reader's own comment is ErrCommentGone.
//  4. A comment whose author account is deleted is ErrCommentGone: deleting an
//     account removes its blocks, so a block written later could never be
//     listed or taken back.
//  5. A comment whose author has no username, or one whose screen is not clear,
//     is ErrCommentGone (review CR-01, 2026-10-30). The reader sees such an
//     author as New member, so the refusal reveals nothing new, and Block is
//     offered only where the name is shown: Settings, Blocked people lists
//     only a clear name, so a block on any other could not be seen or taken
//     back there, and the follow totals would tell the blocker whether an
//     unnamed person followed them.
//  6. Otherwise the author's account id.
//
// It does not look at rep.open, because Block stays available on a report whose
// comments are closed, as Flag does (a safety action). It does not look at
// blocks either, so a comment of a person who blocked the reader is answered
// like any other and reveals nothing.
func (s *CommentService) BlockableAuthor(ctx context.Context, commentID, viewerID int64) (int64, error) {
	target, rep, err := s.actOnComment(ctx, commentID)
	if err != nil {
		if errors.Is(err, ErrReportNotFound) {
			return 0, ErrCommentGone
		}
		return 0, err
	}
	if rep.hidden || commentTargetGone(target) {
		return 0, ErrCommentGone
	}
	if target.AccountID == viewerID {
		return 0, ErrCommentGone
	}
	if target.AuthorDeleted {
		return 0, ErrCommentGone
	}
	if target.AuthorUsername == nil || target.AuthorUsernameScreen == nil ||
		*target.AuthorUsernameScreen != UsernameScreenClear {
		return 0, ErrCommentGone
	}
	return target.AccountID, nil
}
