// This file seeds the Phase 8 comment rows for store and service tests: a
// comment or reply in any visible state, a like, a comment flag and an account
// username. The rows are written with plain SQL, not through the comment
// service, so a test can set up states the service would not produce.
package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CommentSeed describes one comments row for SeedComment. Zero values mean the
// common case: a clear, live, top level comment with a short body.
type CommentSeed struct {
	ReportID  int64
	AccountID int64
	// ParentID is the thread root of a reply, 0 for a top level comment. When it
	// is set and ReplyToID is 0, ReplyToID becomes ParentID (an answer to the
	// root), because comments_thread_shape wants both or neither.
	ParentID  int64
	ReplyToID int64
	// Body defaults to "a seeded comment". A Deleted row always holds the empty
	// string, as comments_body_shape requires.
	Body string
	// State is clear (the default), pending or removed. A pending row gets
	// next_attempt_at = now(), as comments_pending_due requires.
	State string
	// Deleted stamps deleted_at and blanks the body.
	Deleted bool
}

// SeedComment inserts one comment and returns its id.
func SeedComment(t *testing.T, pool *pgxpool.Pool, c CommentSeed) int64 {
	t.Helper()

	state := c.State
	if state == "" {
		state = "clear"
	}
	body := c.Body
	if body == "" && !c.Deleted {
		body = "a seeded comment"
	}
	if c.Deleted {
		body = ""
	}
	var parent, replyTo *int64
	if c.ParentID != 0 {
		p := c.ParentID
		parent = &p
		r := c.ReplyToID
		if r == 0 {
			r = c.ParentID
		}
		replyTo = &r
	}

	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO comments (report_id, account_id, parent_id, reply_to_id, body, moderation_state, next_attempt_at, deleted_at)
		 VALUES ($1, $2, $3, $4, $5, $6,
		         CASE WHEN $6::text = 'pending' THEN now() END,
		         CASE WHEN $7::bool THEN now() END)
		 RETURNING id`,
		c.ReportID, c.AccountID, parent, replyTo, body, state, c.Deleted,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seeding comment on report %d by account %d: %v", c.ReportID, c.AccountID, err)
	}
	return id
}

// SeedCommentLike inserts a like by accountID on commentID.
func SeedCommentLike(t *testing.T, pool *pgxpool.Pool, commentID, accountID int64) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		"INSERT INTO comment_likes (comment_id, account_id) VALUES ($1, $2)", commentID, accountID,
	); err != nil {
		t.Fatalf("seeding like on comment %d by account %d: %v", commentID, accountID, err)
	}
}

// SeedCommentFlag inserts a flag by accountID on commentID with the one reason
// spam and no note, and returns its id.
func SeedCommentFlag(t *testing.T, pool *pgxpool.Pool, commentID, accountID int64) int64 {
	t.Helper()

	var id int64
	if err := pool.QueryRow(context.Background(),
		"INSERT INTO comment_flags (comment_id, account_id, reasons) VALUES ($1, $2, ARRAY['spam']) RETURNING id",
		commentID, accountID,
	).Scan(&id); err != nil {
		t.Fatalf("seeding flag on comment %d by account %d: %v", commentID, accountID, err)
	}
	return id
}

// SeedUsername gives an account a username with the given screen state, clear
// or pending, so both columns satisfy accounts_username_screen_valid. The
// username must already follow the username rules (lowercase, 3 to 20 characters).
func SeedUsername(t *testing.T, pool *pgxpool.Pool, accountID int64, username, screen string) {
	t.Helper()

	if _, err := pool.Exec(context.Background(),
		"UPDATE accounts SET username = $2, username_screen = $3 WHERE id = $1",
		accountID, username, screen,
	); err != nil {
		t.Fatalf("seeding username %q (%s) for account %d: %v", username, screen, accountID, err)
	}
}
