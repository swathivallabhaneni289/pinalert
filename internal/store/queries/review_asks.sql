-- Quick 261118-rz6: the author's side of an ask for review of a removed
-- report. The moderator's side (the Review asks list, Restore and Keep
-- removed) lives in moderation.sql.

-- name: ReviewAskFacts :one
-- What the author's box decides from: the report's moderation state, the
-- reason of its current removal, its author's account (NULL when the author
-- has no account) and whether that account was deleted, whether an ask is
-- open and how many asks the report has had in total. No row means no
-- report. No note and nothing about flags or moderators is read.
SELECT r.moderation_state, r.removed_reason,
       s.account_id AS author_account_id,
       COALESCE(a.deleted_at IS NOT NULL, false)::bool AS author_deleted,
       EXISTS (SELECT 1 FROM report_review_asks q
                WHERE q.report_id = r.id AND q.answered_at IS NULL)::bool AS ask_open,
       (SELECT count(*) FROM report_review_asks q
         WHERE q.report_id = r.id)::bigint AS ask_total
FROM reports r
LEFT JOIN sessions s ON s.session_id = r.session_id
LEFT JOIN accounts a ON a.id = s.account_id
WHERE r.id = sqlc.arg(report_id);

-- name: InsertReviewAsk :exec
-- One ask for review. The service runs it after locking the report row and
-- deciding the open ask and the cap of 3 from ReviewAskFacts in the same
-- transaction; uq_report_review_asks_open still refuses a second open ask.
-- The note is trimmed plain text or NULL.
INSERT INTO report_review_asks (report_id, account_id, note)
VALUES (sqlc.arg(report_id), sqlc.arg(account_id), sqlc.narg(note));

-- name: BlankAccountReviewAskNotes :execrows
-- Delete my account (quick 261118-uoe): the person's own written words on
-- their asks for review go with the account, the way their comments do. Only
-- the note is cleared. The ask rows stay, with their report, time, answer and
-- answering moderator, so the moderator's history and the cap of 3 stay
-- correct, and an open ask stays open without its note. Account deletion is a
-- scrub, so the table's ON DELETE CASCADE never fires.
UPDATE report_review_asks
SET note = NULL
WHERE account_id = sqlc.arg(account_id)::bigint
  AND note IS NOT NULL;
