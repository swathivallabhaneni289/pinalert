package service

import "context"

// CommentAuthorSource turns a comment the reader can see into the account that
// wrote it, or ErrCommentGone. *CommentService is the implementation.
type CommentAuthorSource interface {
	BlockableAuthor(ctx context.Context, commentID, viewerID int64) (int64, error)
}

var _ CommentAuthorSource = (*CommentService)(nil)

// WithPeopleCommentAuthors sets the source BlockCommentAuthor reads the author
// of a comment from. Without one BlockCommentAuthor refuses everything.
func WithPeopleCommentAuthors(src CommentAuthorSource) PeopleOption {
	return func(s *PeopleService) {
		s.comments = src
	}
}

// BlockCommentAuthor blocks the person who wrote a comment the viewer can see
// (quick 261030-u78, WR-E04, D-12). It skips publicFace on purpose: the reader
// already sees the comment and its author name, and refusing a person who
// blocked the reader would reveal that block. Every other rule of Block holds:
// the block row is written first, the follows end both ways with reason blocked
// and the queued follow notes of the pair are deleted, all through the same
// blockPair tail. Every refusal is ErrCommentGone, so it can probe nothing; a
// source that was never set fails closed the same way.
func (s *PeopleService) BlockCommentAuthor(ctx context.Context, viewerID, commentID int64) error {
	if s.comments == nil {
		return ErrCommentGone
	}
	authorID, err := s.comments.BlockableAuthor(ctx, commentID, viewerID)
	if err != nil {
		return err
	}
	return s.blockPair(ctx, "BlockCommentAuthor", viewerID, authorID)
}
