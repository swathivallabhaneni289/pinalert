package service

// commentscopy.go holds the words the comment service hands out (Phase 8 plan
// 09, PAGE-02). The route (plan 08-12) puts them in its field errors and the
// page (UI-SPEC surface 7) shows them as they are. The wording rules are the
// other server messages' own: plain letters, digits, spaces, commas, full
// stops and apostrophes, nothing about sound, and no dash. The two author words
// hold a capital and a space on purpose, so no username (lowercase, no spaces)
// can ever equal them.

const (
	// CommentAuthorNewMember is shown in place of a username that has not
	// passed its text screen yet (A-P8-10).
	CommentAuthorNewMember = "New member"
	// CommentAuthorDeletedAccount is shown in place of the username of an
	// account that was deleted.
	CommentAuthorDeletedAccount = "Deleted account"

	// CommentsClosedMessage is the quiet line shown when the report is resolved
	// or has faded (D-15). Reading, deleting your own comment and flagging stay.
	CommentsClosedMessage = "Comments are closed"
	// CommentEmptyMessage answers a comment with nothing to post.
	CommentEmptyMessage = "Write a comment first."
	// CommentTooLongMessage answers a comment over CommentMaxRunes.
	CommentTooLongMessage = "A comment can be up to 280 letters."
	// CommentCharactersMessage answers a comment with a control or invisible
	// character.
	CommentCharactersMessage = "This comment has characters that can't be posted."
	// CommentNeedsUsernameMessage answers a person who has no username yet.
	CommentNeedsUsernameMessage = "Choose a username in Settings first."
	// CommentGoneMessage answers an action on a comment that is deleted, removed
	// or held back.
	CommentGoneMessage = "This comment is no longer available."
	// CommentNotYoursMessage answers a delete of someone else's comment.
	CommentNotYoursMessage = "You can only delete your own comment."
	// CommentOwnFlagMessage answers a flag on your own comment.
	CommentOwnFlagMessage = "You can't flag your own comment."
)
