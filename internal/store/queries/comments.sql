-- name: InsertComment :one
-- Phase 8 plan 08 (PAGE-02, D-12): writes one comment or reply and returns the
-- new id and time. moderation_state is decided by the service from the text
-- check: clear for clear and borderline text (borderline also sets
-- needs_review and keeps the check scores), pending for text the check could
-- not finish (next_attempt_at must then be set, comments_pending_due). The
-- check columns hold scores only, never comment text. parent_id is always the
-- thread root and reply_to_id the comment actually answered; both are NULL for
-- a top level comment (comments_thread_shape).
INSERT INTO comments (
    report_id, account_id, parent_id, reply_to_id, body,
    moderation_state, needs_review, next_attempt_at, verdict, scores, model, checked_at
) VALUES (
    sqlc.arg(report_id), sqlc.arg(account_id), sqlc.narg(parent_id), sqlc.narg(reply_to_id), sqlc.arg(body),
    sqlc.arg(moderation_state), sqlc.arg(needs_review), sqlc.narg(next_attempt_at),
    sqlc.narg(verdict), sqlc.narg(scores)::jsonb, sqlc.narg(model), sqlc.narg(checked_at)
)
RETURNING id, created_at;

-- name: CommentTarget :one
-- One comment as the service needs it before it acts on it (reply, like, flag,
-- delete): which report it belongs to, who wrote it, where it sits in its
-- thread and whether it is still there, plus its author's username, username
-- screen and deleted flag so a reply can name the person answered (review W3).
-- account_id and the author fields stay inside the service: the service shows
-- the username only under the rule of CommentsForReport (screen clear, account
-- not deleted). No row means no such comment.
SELECT c.id, c.report_id, c.account_id, c.parent_id, c.reply_to_id,
       (c.deleted_at IS NOT NULL)::bool AS deleted,
       c.moderation_state,
       a.username AS author_username,
       a.username_screen AS author_username_screen,
       (a.deleted_at IS NOT NULL)::bool AS author_deleted
FROM comments c
JOIN accounts a ON a.id = c.account_id
WHERE c.id = sqlc.arg(comment_id);

-- name: CommentsForReport :many
-- The one place that decides, for one viewer, which comments of a report are
-- listed and what is shown about them (A-P8-2, D-12, D-14). Phase 9 plan 09-10
-- adds its block filter to this query and to comment_count in
-- ReportExtrasForReports; that filter must also null reply_to_author when the
-- answered author is blocked, so the name then shows only as plain typed text,
-- never as a bold name.
--
-- Visibility, written once in the CTE below:
--   shown       a comment is shown to the viewer when it is not deleted and it
--               is clear, or it is pending and the viewer wrote it (a held
--               comment is visible to its author only, T-08-33).
--   root        a thread root is listed when it is shown, or when it is deleted
--               or removed and at least one reply of it is shown. Such a row is
--               a placeholder: it carries no words, no name, no likes and no
--               viewer flags, so nothing tells who wrote it or what it said
--               (D-14, review W4).
--   reply       a reply is listed when it is shown and its root is listed.
--               Threads are one level deep, so a deleted reply is not listed.
--
-- Parameters: after_id lists roots with a larger id only (paging); root_cap
-- limits the number of roots, in id order; reply_cap is a total bound on the
-- reply rows returned for those roots, in id order (the service passes
-- CommentReplyCap as a safety bound, not a per thread page size).
--
-- Columns, in this order:
--   id, parent_id, reply_to_id   the thread position; parent_id is the root id
--                                for a reply and NULL for a root.
--   account_id                   service internal, used for IsOwn and CanDelete;
--                                it is 0 on a placeholder and never reaches a
--                                response.
--   body                         empty on a placeholder.
--   created_at                   when it was written.
--   gone                         '' for a shown comment, 'deleted' or 'removed'
--                                for a placeholder.
--   held                         true on the viewer's own pending comment.
--   author_username              NULL unless the author is the viewer, or the
--                                username screen is clear and the account is
--                                not deleted (A-P8-10, T-08-34).
--   author_kind                  'user', 'new_member', 'deleted_account', or ''
--                                for a placeholder.
--   reply_to_author              for a reply only: the username of the author of
--                                the comment it answered, and only when that
--                                comment is clear and not deleted, and its
--                                author's screen is clear and the account not
--                                deleted; NULL otherwise (review W3).
--   is_reporter                  the comment's account is the report's reporter
--                                account (reports.session_id joins
--                                sessions.session_id, as ReportVoteContext
--                                does); false on a placeholder.
--   likes, viewer_liked, viewer_flagged   0, false, false on a placeholder.
--
-- The result is ordered by thread: each root, then its replies, oldest first.
-- A viewer id of 0 matches no account, so viewer 0 sees exactly the clear,
-- not deleted comments (TestCommentCountMatchesList pins this against
-- comment_count in ReportExtrasForReports).
WITH visible AS (
    SELECT c.id, c.parent_id, c.reply_to_id, c.account_id, c.body, c.created_at,
           c.deleted_at, c.moderation_state,
           (c.deleted_at IS NULL
            AND (c.moderation_state = 'clear'
                 OR (c.moderation_state = 'pending' AND c.account_id = sqlc.arg(viewer_account_id)::bigint))) AS shown
    FROM comments c
    WHERE c.report_id = sqlc.arg(report_id)
),
roots AS (
    SELECT v.*
    FROM visible v
    WHERE v.parent_id IS NULL
      AND v.id > sqlc.arg(after_id)::bigint
      AND (v.shown
           OR ((v.deleted_at IS NOT NULL OR v.moderation_state = 'removed')
               AND EXISTS (SELECT 1 FROM visible r WHERE r.parent_id = v.id AND r.shown)))
    ORDER BY v.id
    LIMIT sqlc.arg(root_cap)::int
),
replies AS (
    SELECT v.*
    FROM visible v
    JOIN roots ON v.parent_id = roots.id
    WHERE v.shown
    ORDER BY v.id
    LIMIT sqlc.arg(reply_cap)::int
),
listed AS (
    SELECT * FROM roots
    UNION ALL
    SELECT * FROM replies
)
SELECT l.id,
       l.parent_id,
       l.reply_to_id,
       (CASE WHEN l.shown THEN l.account_id ELSE 0 END)::bigint AS account_id,
       (CASE WHEN l.shown THEN l.body ELSE '' END)::text AS body,
       l.created_at,
       (CASE WHEN l.shown THEN ''
             WHEN l.deleted_at IS NOT NULL THEN 'deleted'
             ELSE 'removed' END)::text AS gone,
       (l.shown AND l.moderation_state = 'pending')::bool AS held,
       au.username AS author_username,
       (CASE WHEN NOT l.shown THEN ''
             WHEN a.deleted_at IS NOT NULL THEN 'deleted_account'
             WHEN l.account_id = sqlc.arg(viewer_account_id)::bigint AND a.username IS NOT NULL THEN 'user'
             WHEN a.username_screen = 'clear' THEN 'user'
             ELSE 'new_member' END)::text AS author_kind,
       rn.username AS reply_to_author,
       (l.shown AND s.account_id IS NOT NULL AND s.account_id = l.account_id)::bool AS is_reporter,
       (CASE WHEN l.shown THEN (SELECT count(*) FROM comment_likes k WHERE k.comment_id = l.id) ELSE 0 END)::int AS likes,
       (l.shown AND EXISTS (SELECT 1 FROM comment_likes k
                             WHERE k.comment_id = l.id AND k.account_id = sqlc.arg(viewer_account_id)::bigint))::bool AS viewer_liked,
       (l.shown AND EXISTS (SELECT 1 FROM comment_flags f
                             WHERE f.comment_id = l.id AND f.account_id = sqlc.arg(viewer_account_id)::bigint))::bool AS viewer_flagged
FROM listed l
JOIN accounts a ON a.id = l.account_id
JOIN reports rp ON rp.id = sqlc.arg(report_id)
LEFT JOIN sessions s ON s.session_id = rp.session_id
LEFT JOIN comments rt ON rt.id = l.reply_to_id
-- The two names are conditional LEFT JOINs, not CASE expressions, so sqlc types
-- them as nullable text (a CASE with a NULL branch comes out as interface{}).
-- au is the author under the A-P8-10 rule: a name only for a shown comment whose
-- account is not deleted and whose screen is clear, or the viewer's own. rn is
-- the author of the answered comment under the review W3 rule: a reply only,
-- the answered comment clear and not deleted, its author's screen clear and the
-- account not deleted.
LEFT JOIN accounts au ON au.id = l.account_id
    AND l.shown
    AND au.deleted_at IS NULL
    AND (l.account_id = sqlc.arg(viewer_account_id)::bigint OR au.username_screen = 'clear')
LEFT JOIN accounts rn ON rn.id = rt.account_id
    AND l.shown AND l.parent_id IS NOT NULL
    AND rt.deleted_at IS NULL AND rt.moderation_state = 'clear'
    AND rn.deleted_at IS NULL AND rn.username_screen = 'clear'
ORDER BY COALESCE(l.parent_id, l.id), (l.parent_id IS NOT NULL), l.id;

-- name: MarkCommentDeleted :execrows
-- The author deletes their own comment (D-14): the words go, deleted_at is
-- stamped, and the row stays so likes, flags and replies keep their place. The
-- guard on account_id makes it author only, and the guard on deleted_at makes a
-- second call change nothing (zero rows), so it is idempotent. Zero rows means
-- not yours, not found or already gone; the service tells them apart with
-- CommentTarget.
UPDATE comments
SET body = '',
    deleted_at = sqlc.arg(now)
WHERE id = sqlc.arg(comment_id)
  AND account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL;

-- name: LikeComment :execrows
-- One like per account per comment, by the primary key: a second call changes
-- nothing (zero rows). A like is a personal row and is read by no trust query
-- (D-02).
INSERT INTO comment_likes (comment_id, account_id)
VALUES (sqlc.arg(comment_id), sqlc.arg(account_id))
ON CONFLICT (comment_id, account_id) DO NOTHING;

-- name: UnlikeComment :execrows
-- Takes the viewer's like back. Zero rows when there was none.
DELETE FROM comment_likes
WHERE comment_id = sqlc.arg(comment_id)
  AND account_id = sqlc.arg(account_id);

-- name: CommentLikeState :one
-- How many likes a comment has and whether the viewer gave one, for the answer
-- to a like or unlike tap. Counts every like row; the service only calls it for
-- a comment the viewer may see.
SELECT (SELECT count(*) FROM comment_likes k WHERE k.comment_id = sqlc.arg(comment_id))::int AS likes,
       EXISTS (SELECT 1 FROM comment_likes k
                WHERE k.comment_id = sqlc.arg(comment_id)
                  AND k.account_id = sqlc.arg(account_id))::bool AS viewer_liked;

-- name: InsertCommentFlag :one
-- A flag on a comment. One flag per account per comment: a second call
-- conflicts and returns no row (pgx.ErrNoRows), which the service reads as
-- already flagged. A flag hides nothing by itself. reasons is the closed list
-- of report flags and note is NULL or 1 to 300 characters, both checked by the
-- table.
INSERT INTO comment_flags (comment_id, account_id, reasons, note)
VALUES (sqlc.arg(comment_id), sqlc.arg(account_id), sqlc.arg(reasons)::text[], sqlc.narg(note))
ON CONFLICT (comment_id, account_id) DO NOTHING
RETURNING id;

-- name: BlankAccountComments :execrows
-- Delete my account (A-P8-6): every comment the account wrote loses its words
-- and gets deleted_at, so it reads as a placeholder where others replied and
-- vanishes where nobody did. The rows stay, so replies, other people's likes
-- and flags and the audit trail keep their place. A comment already deleted is
-- left alone.
UPDATE comments
SET body = '',
    deleted_at = sqlc.arg(now)
WHERE account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL;

-- name: DeleteAccountCommentLikes :exec
-- Delete my account: the likes the account gave are personal rows. The likes
-- other people gave to its comments stay with the comments.
DELETE FROM comment_likes WHERE account_id = sqlc.arg(account_id);

-- name: DeleteAccountCommentFlags :exec
-- Delete my account: the flags the account sent on comments are personal rows.
DELETE FROM comment_flags WHERE account_id = sqlc.arg(account_id);

-- name: CommentForNotify :one
-- Phase 8 plan 11 (D-16): what the notification step needs to know about one
-- comment, in one read: the comment's own report, author, thread position,
-- state and deleted flag, the author of the comment it answered (NULL for a top
-- level comment) and the report's reporter account (NULL when the report was
-- filed from a session with no account). The reporter is found the way
-- ReportVoteContext finds it: reports.session_id joins sessions.session_id.
-- These ids stay inside the service; none of them reaches a response or a log.
-- No row means no such comment.
SELECT c.id, c.report_id, c.account_id, c.parent_id, c.reply_to_id,
       c.moderation_state,
       (c.deleted_at IS NOT NULL)::bool AS deleted,
       rc.account_id AS reply_to_author_id,
       rs.account_id AS reporter_account_id
FROM comments c
JOIN reports r ON r.id = c.report_id
LEFT JOIN comments rc ON rc.id = c.reply_to_id
LEFT JOIN sessions rs ON rs.session_id = r.session_id
WHERE c.id = sqlc.arg(comment_id);

-- name: InsertCommentUpdate :execrows
-- The one place a comment notification is written (Phase 8 plan 11, D-16,
-- PAGE-02). It adds one update of kind comment_on_yours or reply_to_yours for
-- the recipient and returns how many rows it wrote (0 or 1). The row holds the
-- kind and the report id only: no comment words and no username are stored, the
-- sentence is written at read time (T-08-49).
--
-- It writes nothing when the recipient is the commenter (nobody is told about
-- their own comment), when the recipient muted the report (D-11), or when the
-- recipient's account is deleted. A recipient that does not exist writes
-- nothing either, because the rows come from accounts. One unread row per
-- account, report and kind: a second comment while the first is still unread
-- conflicts on uq_updates_comment_unread and does nothing, so thirty comments
-- are one row; once the person has read it, the next comment writes a new row.
-- The conflict predicate repeats the index predicate exactly, which is what
-- lets Postgres pick the partial index.
--
-- Comment rows never roll up into the "more" row and do not count toward the
-- hourly ceiling (the ceiling reads skip these kinds); the unread index is
-- their cap. Phase 9 plan 09-10 adds its block skip here.
INSERT INTO updates (account_id, report_id, kind)
SELECT a.id, sqlc.arg(report_id)::bigint, sqlc.arg(kind)::text
FROM accounts a
WHERE a.id = sqlc.arg(recipient_account_id)::bigint
  AND a.id <> sqlc.arg(commenter_account_id)::bigint
  AND a.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM report_mutes m
      WHERE m.account_id = a.id
        AND m.report_id = sqlc.arg(report_id)::bigint
  )
ON CONFLICT (account_id, report_id, kind)
    WHERE read_at IS NULL AND kind IN ('comment_on_yours', 'reply_to_yours')
    DO NOTHING;
