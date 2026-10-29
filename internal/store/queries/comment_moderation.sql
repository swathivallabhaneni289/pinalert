-- Comment moderation (Phase 8 plan 13, D-12, D-14). A sibling of
-- moderation.sql, not a part of it: the report side is keyed and locked by
-- report (moderation_checks), while a comment carries its own check columns,
-- so Keep and Remove lock the comment row itself.
--
-- The moderator list carries the words, because a moderator must read them to
-- decide. It names nothing about the author: no account id, no email, no
-- username (T-08-65). A flag only lists a comment; hiding one needs Remove
-- (D-14, T-08-66).
--
-- The worker of plan 08-14 and this list skip rows with deleted_at set, because
-- MarkCommentDeleted and BlankAccountComments leave needs_review and the lease
-- columns alone.

-- name: CommentModerationQueue :many
-- Every comment a moderator has yet to decide, newest first, at most
-- sqlc.arg(row_limit): a clear comment with an open flag, a borderline comment
-- (needs_review), or a held comment whose check has not finished. A deleted
-- comment and a removed one are left out, as is a clear comment whose flags are
-- all dismissed.
--
-- Columns: id, report_id, category (of the comment's report), body,
-- created_at, moderation_state, needs_review, open_flags (count of undismissed
-- flags), flag_reasons (the distinct reasons of those flags, sorted), pending
-- (the check has not finished).
SELECT c.id,
       c.report_id,
       r.category,
       c.body,
       c.created_at,
       c.moderation_state,
       c.needs_review,
       (SELECT count(*) FROM comment_flags f
         WHERE f.comment_id = c.id AND f.dismissed_at IS NULL)::int AS open_flags,
       ARRAY(SELECT DISTINCT x.reason
               FROM comment_flags f, unnest(f.reasons) AS x(reason)
              WHERE f.comment_id = c.id AND f.dismissed_at IS NULL
              ORDER BY x.reason)::text[] AS flag_reasons,
       (c.moderation_state = 'pending')::bool AS pending
FROM comments c
JOIN reports r ON r.id = c.report_id
WHERE c.deleted_at IS NULL
  AND c.moderation_state <> 'removed'
  AND (c.needs_review
       OR c.moderation_state = 'pending'
       OR EXISTS (SELECT 1 FROM comment_flags f
                   WHERE f.comment_id = c.id AND f.dismissed_at IS NULL))
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(row_limit);

-- name: LockCommentForModeration :one
-- Takes the row lock on one comment, the first statement of the Keep and Remove
-- transaction, and reads what the decision needs: whether it is deleted, its
-- state, whether it is marked for review and how many flags are open. No row
-- means an unknown comment. A moderator and the settle step of the worker queue
-- on this row.
SELECT c.id,
       c.moderation_state,
       (c.deleted_at IS NOT NULL)::bool AS deleted,
       c.needs_review,
       (SELECT count(*) FROM comment_flags f
         WHERE f.comment_id = c.id AND f.dismissed_at IS NULL)::int AS open_flags
FROM comments c
WHERE c.id = sqlc.arg(comment_id)
FOR UPDATE OF c;

-- name: KeepComment :execrows
-- Keep: the review mark is cleared and a held comment is released into the
-- normal rules (clear, with its due time and lease cleared). A removed comment
-- is never touched, so removal stays terminal, and neither is a deleted one.
-- The service runs it inside a transaction after LockCommentForModeration.
UPDATE comments
SET needs_review    = false,
    moderation_state = CASE WHEN moderation_state = 'pending' THEN 'clear' ELSE moderation_state END,
    next_attempt_at = CASE WHEN moderation_state = 'pending' THEN NULL ELSE next_attempt_at END,
    lease_until     = NULL
WHERE id = sqlc.arg(comment_id)
  AND deleted_at IS NULL
  AND moderation_state <> 'removed';

-- name: RemoveComment :execrows
-- Remove: the comment is withdrawn for everyone (the list reads it as the
-- removed placeholder when replies are shown under it). The review mark, the
-- due time and the lease are cleared so the worker never claims it again.
UPDATE comments
SET moderation_state = 'removed',
    needs_review    = false,
    next_attempt_at = NULL,
    lease_until     = NULL
WHERE id = sqlc.arg(comment_id)
  AND deleted_at IS NULL
  AND moderation_state <> 'removed';

-- name: DismissCommentFlags :execrows
-- Closes every open flag on one comment, at the moment of the decision. Keep
-- and Remove both call it, so a decided comment leaves the flagged list.
UPDATE comment_flags
SET dismissed_at = now()
WHERE comment_id = sqlc.arg(comment_id)
  AND dismissed_at IS NULL;

-- name: InsertCommentModerationAction :exec
-- One audit row per Keep or Remove. It holds the comment id, the moderator's
-- account id and the action, never the comment text.
INSERT INTO comment_moderation_actions (comment_id, moderator_account_id, action)
VALUES (sqlc.arg(comment_id), sqlc.arg(moderator_account_id), sqlc.arg(action));
