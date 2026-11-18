-- Phase 4 moderation queries: the text check record, the background worker
-- claim, the three moderator lists with their counts, Keep and Remove as one
-- statement each, flags, and the audit row.
--
-- Three list predicates, written once here and repeated word for word in each
-- list, in ModerationQueueCounts and in ModerationStatusForReport so a heading
-- count and a list can never drift apart:
--   review:  c.needs_review AND r.moderation_state <> 'removed'
--   flagged: an open flag exists AND r.moderation_state <> 'removed'
--   pending: c.status = 'pending' AND r.expires_at > now()
--            AND r.moderation_state <> 'removed' (an expired report drops out)
--
-- No query here selects a session id, an email or an account id, and no table
-- it writes has a column that holds report text (T-04-04). The flag_notes column
-- of the three lists carries each open flag's reasons and note, the flagger's
-- own words for moderators only, and never who wrote them (plan 04-31). A
-- reason is counted once per flag it appears in (unnest over the set).

-- name: InsertModerationCheck :exec
-- One statement serves both writers: the pending row that the background
-- worker will pick up (a check that did not finish at submit) and the done
-- borderline row written inline when the check finished at submit and wants a
-- human look (D-04). Scores are stored without report text.
INSERT INTO moderation_checks (
    report_id, status, next_attempt_at, verdict, scores, model, checked_at, needs_review
) VALUES (
    sqlc.arg(report_id), sqlc.arg(status), sqlc.arg(next_attempt_at),
    sqlc.narg(verdict), sqlc.narg(scores)::jsonb, sqlc.narg(model), sqlc.narg(checked_at),
    sqlc.arg(needs_review)
);

-- name: ClaimDueModerationChecks :many
-- Hands due checks to one claimant each. The claimed set is fixed once in a
-- MATERIALIZED CTE and then updated, rather than written as an IN (subquery),
-- because the planner may rescan an IN subquery that carries LIMIT and SKIP
-- LOCKED and so lock and return more rows than the batch limit. SKIP LOCKED
-- lets two workers claim different rows without waiting on each other, and the
-- lease makes a crashed claimant's row claimable again once it lapses; attempts
-- counts every claim. A check is never claimed when its report has expired,
-- was removed or is hidden for review (quick 261117-vvb), when it is done, or
-- when it is not yet due. The lease and the
-- batch size come from the worker's Go constants (60 seconds, 5 rows).
-- Postgres does not promise the order of rows an UPDATE returns.
WITH due AS MATERIALIZED (
    SELECT mc.report_id
    FROM moderation_checks mc
    JOIN reports r2 ON r2.id = mc.report_id
    WHERE mc.status = 'pending'
      AND mc.next_attempt_at <= now()
      AND (mc.lease_until IS NULL OR mc.lease_until < now())
      AND r2.expires_at > now()
      AND r2.moderation_state NOT IN ('removed', 'review')
    ORDER BY mc.next_attempt_at, mc.report_id
    LIMIT sqlc.arg(batch_limit)::int
    FOR UPDATE OF mc SKIP LOCKED
)
UPDATE moderation_checks c
SET lease_until = now() + sqlc.arg(lease_seconds)::int * interval '1 second',
    attempts = c.attempts + 1
FROM due, reports r
WHERE c.report_id = due.report_id
  AND r.id = c.report_id
RETURNING c.report_id, c.attempts, r.description, r.severity, r.category,
          r.moderation_state, r.expires_at;

-- name: FinishCheckAndSetState :exec
-- The worker's one write after a verdict: it closes the check and moves the
-- report in a single statement, and the report moves only when this statement
-- itself closed a pending check. The main UPDATE reads fin, so the check row
-- is locked first and the report row second, the order Keep and Remove also
-- take (LockModerationCheck). When Keep or Remove holds the check row, the
-- check UPDATE waits, then rechecks status = 'pending' against the committed
-- row, finds it done and changes nothing, so fin is empty and the report is
-- untouched: a late worker reject cannot withdraw a report a moderator kept,
-- and a check closed by Keep or Remove is never reopened or overwritten.
-- Removal is terminal, so a worker pass never undoes a moderator Remove
-- (R-03). One statement also leaves no crash window between the two writes.
--
-- Quick 261117-vvb: a report hidden for review is never moved, like a removed
-- one. When the report moved and own_kind is set, the same statement writes
-- that one bell row for the author (own_shown when a held report is now
-- showing, own_removed when the late check removed it). The worker decides the
-- kind in Go from the state and the claimed report; SQL decides nothing about
-- visibility. A report whose author has no account, or a deleted one, gets no
-- row.
--
-- Quick 261118-rz6: removed_reason is text_check when the worker removes the
-- report and NULL otherwise, and removed_at is stamped with it, so the
-- author's box and the moderator's Removed list read why and when. The bell
-- row is an upsert: a row of the same kind that is already there moves to now,
-- becomes unread again and takes a fresh id, so it is never a second row and a
-- read mark sent for a list drawn before it cannot mark it read.
WITH fin AS (
    UPDATE moderation_checks
    SET status = 'done',
        verdict = sqlc.arg(verdict)::text,
        scores = sqlc.narg(scores)::jsonb,
        model = sqlc.narg(model),
        needs_review = sqlc.arg(needs_review)::bool,
        checked_at = now(),
        lease_until = NULL
    WHERE moderation_checks.report_id = sqlc.arg(report_id)
      AND moderation_checks.status = 'pending'
    RETURNING moderation_checks.report_id
), moved AS (
    UPDATE reports r
    SET moderation_state = sqlc.arg(moderation_state),
        removed_reason = sqlc.narg(removed_reason)::text,
        removed_at = CASE WHEN sqlc.narg(removed_reason)::text IS NULL THEN NULL ELSE now() END
    FROM fin
    WHERE r.id = fin.report_id
      AND r.moderation_state NOT IN ('removed', 'review')
    RETURNING r.id, r.session_id
)
INSERT INTO updates (account_id, report_id, kind)
SELECT s.account_id, m.id, sqlc.narg(own_kind)::text
FROM moved m
JOIN sessions s ON s.session_id = m.session_id
JOIN accounts a ON a.id = s.account_id
WHERE sqlc.narg(own_kind)::text IS NOT NULL
  AND a.deleted_at IS NULL
ON CONFLICT (account_id, report_id, kind) WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed', 'own_restored', 'own_stays_removed')
DO UPDATE SET id = nextval(pg_get_serial_sequence('updates', 'id')), created_at = now(), read_at = NULL;

-- name: RescheduleModerationCheck :exec
-- The provider did not answer: try again later. last_error holds an error
-- class such as a status code, never report text. Only a pending check moves.
UPDATE moderation_checks
SET next_attempt_at = sqlc.arg(next_attempt_at),
    lease_until = NULL,
    last_error = sqlc.narg(last_error)
WHERE report_id = sqlc.arg(report_id)
  AND status = 'pending';

-- name: ReviewQueue :many
-- List (a): posts the text check flagged as borderline and a person has not
-- decided, newest first, at most 100 rows. The count in ModerationQueueCounts
-- is not this list's length.
SELECT r.id, r.category, r.severity, r.description, r.created_at, r.expires_at,
       r.client_written_at, r.moderation_state,
       c.status AS check_status, c.verdict, c.scores, c.model,
       COALESCE(c.needs_review, false)::bool AS needs_review,
       (SELECT count(*) FROM report_flags f
         WHERE f.report_id = r.id AND f.dismissed_at IS NULL)::bigint AS open_flag_total,
       COALESCE((SELECT jsonb_object_agg(x.reason, x.n)
                   FROM (SELECT fr.reason, count(*) AS n
                           FROM report_flags f, unnest(f.reasons) AS fr(reason)
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL
                          GROUP BY fr.reason) x), '{}'::jsonb)::jsonb AS flag_counts,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('reasons', n.reasons, 'note', n.note)
                                  ORDER BY n.created_at DESC, n.id DESC)
                   FROM (SELECT f.id, f.reasons, f.note, f.created_at FROM report_flags f
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL AND f.note IS NOT NULL
                          ORDER BY f.created_at DESC, f.id DESC
                          LIMIT 20) n), '[]'::jsonb)::jsonb AS flag_notes
FROM reports r
JOIN moderation_checks c ON c.report_id = r.id
WHERE c.needs_review AND r.moderation_state <> 'removed'
ORDER BY r.created_at DESC, r.id DESC
LIMIT 100;

-- name: FlagQueue :many
-- List (b): posts with at least one open flag, most flags first and then
-- newest, at most 100 rows. Quick 261117-vvb: a report hidden for review by
-- flags comes before every other row. LEFT JOIN because a flagged report may have no
-- check row (seeded and older reports).
SELECT r.id, r.category, r.severity, r.description, r.created_at, r.expires_at,
       r.client_written_at, r.moderation_state,
       c.status AS check_status, c.verdict, c.scores, c.model,
       COALESCE(c.needs_review, false)::bool AS needs_review,
       (SELECT count(*) FROM report_flags f
         WHERE f.report_id = r.id AND f.dismissed_at IS NULL)::bigint AS open_flag_total,
       COALESCE((SELECT jsonb_object_agg(x.reason, x.n)
                   FROM (SELECT fr.reason, count(*) AS n
                           FROM report_flags f, unnest(f.reasons) AS fr(reason)
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL
                          GROUP BY fr.reason) x), '{}'::jsonb)::jsonb AS flag_counts,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('reasons', n.reasons, 'note', n.note)
                                  ORDER BY n.created_at DESC, n.id DESC)
                   FROM (SELECT f.id, f.reasons, f.note, f.created_at FROM report_flags f
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL AND f.note IS NOT NULL
                          ORDER BY f.created_at DESC, f.id DESC
                          LIMIT 20) n), '[]'::jsonb)::jsonb AS flag_notes
FROM reports r
LEFT JOIN moderation_checks c ON c.report_id = r.id
WHERE EXISTS (SELECT 1 FROM report_flags f WHERE f.report_id = r.id AND f.dismissed_at IS NULL)
  AND r.moderation_state <> 'removed'
ORDER BY (r.moderation_state = 'review') DESC, open_flag_total DESC, r.created_at DESC, r.id DESC
LIMIT 100;

-- name: PendingQueue :many
-- List (c): posts whose text check has not finished and that have not
-- expired, waiting longest first, at most 100 rows. Serious pending reports
-- are included (D-08).
SELECT r.id, r.category, r.severity, r.description, r.created_at, r.expires_at,
       r.client_written_at, r.moderation_state,
       c.status AS check_status, c.verdict, c.scores, c.model,
       COALESCE(c.needs_review, false)::bool AS needs_review,
       (SELECT count(*) FROM report_flags f
         WHERE f.report_id = r.id AND f.dismissed_at IS NULL)::bigint AS open_flag_total,
       COALESCE((SELECT jsonb_object_agg(x.reason, x.n)
                   FROM (SELECT fr.reason, count(*) AS n
                           FROM report_flags f, unnest(f.reasons) AS fr(reason)
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL
                          GROUP BY fr.reason) x), '{}'::jsonb)::jsonb AS flag_counts,
       COALESCE((SELECT jsonb_agg(jsonb_build_object('reasons', n.reasons, 'note', n.note)
                                  ORDER BY n.created_at DESC, n.id DESC)
                   FROM (SELECT f.id, f.reasons, f.note, f.created_at FROM report_flags f
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL AND f.note IS NOT NULL
                          ORDER BY f.created_at DESC, f.id DESC
                          LIMIT 20) n), '[]'::jsonb)::jsonb AS flag_notes
FROM reports r
JOIN moderation_checks c ON c.report_id = r.id
WHERE c.status = 'pending' AND r.expires_at > now() AND r.moderation_state <> 'removed'
ORDER BY r.created_at ASC, r.id ASC
LIMIT 100;

-- name: ModerationQueueCounts :one
-- The numbers the headings show: unlimited counts over the same predicates as
-- the three lists, so 101 review rows list 100 and count 101. A report with
-- two flag reasons counts once, hence COUNT(DISTINCT).
SELECT
    (SELECT count(*) FROM reports r
       JOIN moderation_checks c ON c.report_id = r.id
      WHERE c.needs_review AND r.moderation_state <> 'removed')::bigint AS review_count,
    (SELECT COUNT(DISTINCT f.report_id) FROM report_flags f
       JOIN reports r ON r.id = f.report_id
      WHERE f.dismissed_at IS NULL AND r.moderation_state <> 'removed')::bigint AS flagged_count,
    (SELECT count(*) FROM reports r
       JOIN moderation_checks c ON c.report_id = r.id
      WHERE c.status = 'pending' AND r.expires_at > now()
        AND r.moderation_state <> 'removed')::bigint AS pending_count,
    -- Quick 261118-rz6: the Review asks and Removed headings, over the same
    -- predicates as ReviewAskQueue and RemovedQueue.
    (SELECT count(*) FROM report_review_asks a
       JOIN reports r ON r.id = a.report_id
      WHERE a.answered_at IS NULL AND r.moderation_state = 'removed')::bigint AS asks_count,
    (SELECT count(*) FROM reports r
      WHERE r.moderation_state = 'removed'
        AND r.removed_at > now() - interval '30 days')::bigint AS removed_count;

-- name: ModerationStatusForReport :one
-- Which lists a report is on right now, for the 404 and 409 decisions on Keep
-- and Remove. The three booleans use the same predicates as the lists. An
-- unknown id returns no row.
SELECT r.id, r.moderation_state, r.severity, r.category, r.expires_at,
       COALESCE(c.needs_review AND r.moderation_state <> 'removed', false)::bool AS in_review,
       (EXISTS (SELECT 1 FROM report_flags f WHERE f.report_id = r.id AND f.dismissed_at IS NULL)
         AND r.moderation_state <> 'removed')::bool AS in_flagged,
       COALESCE(c.status = 'pending' AND r.expires_at > now()
                AND r.moderation_state <> 'removed', false)::bool AS in_pending
FROM reports r
LEFT JOIN moderation_checks c ON c.report_id = r.id
WHERE r.id = sqlc.arg(report_id);

-- name: LockModerationCheck :exec
-- Takes the row lock on a report's check, the first statement of the Keep and
-- Remove transaction. Postgres does not promise the order in which KeepReport
-- and RemoveReport touch their CTE tables and the reports row, and the
-- worker's FinishCheckAndSetState locks the check row before the report row;
-- locking the check row first here gives both writers the same order, so they
-- queue on the check row and cannot deadlock. A report with no check row locks
-- nothing, and the worker never writes such a report.
SELECT report_id FROM moderation_checks
WHERE report_id = sqlc.arg(report_id)
FOR UPDATE;

-- name: KeepReport :exec
-- Keep as one statement. Two data modifying CTEs close the check and dismiss
-- the open flags; the main statement releases a pending report to clear. Keep
-- accepts a pending post as it is and never touches a removed one, so removal
-- stays terminal (D-08, R-03). The service runs it inside a transaction after
-- LockModerationCheck.
--
-- Quick 261117-vvb: a report hidden for review is released to clear too, and
-- only then is review_kept_at stamped, so flags can never hide it again. The
-- SET reads the state before this statement, which is why the stamp is a CASE.
WITH closed AS (
    UPDATE moderation_checks
    SET needs_review = false, status = 'done', lease_until = NULL
    WHERE report_id = sqlc.arg(report_id)
    RETURNING report_id
), dismissed AS (
    UPDATE report_flags
    SET dismissed_at = now()
    WHERE report_id = sqlc.arg(report_id) AND dismissed_at IS NULL
    RETURNING id
)
UPDATE reports
SET moderation_state = 'clear',
    review_kept_at = CASE WHEN reports.moderation_state = 'review' THEN now() ELSE reports.review_kept_at END
WHERE reports.id = sqlc.arg(report_id) AND reports.moderation_state IN ('pending', 'review');

-- name: RemoveReport :exec
-- Remove as one statement, with the same two CTEs as Keep. The main statement
-- sets removed on any report, listed or not (D-08, R-03). The check is closed
-- so the worker never claims it again. Like Keep, the service runs it inside a
-- transaction after LockModerationCheck. Quick 261118-rz6: the moderator's
-- reason and the time are stored with it, and since then only a moderator's
-- Restore (RestoreReport) brings a removed report back.
WITH closed AS (
    UPDATE moderation_checks
    SET needs_review = false, status = 'done', lease_until = NULL
    WHERE report_id = sqlc.arg(report_id)
    RETURNING report_id
), dismissed AS (
    UPDATE report_flags
    SET dismissed_at = now()
    WHERE report_id = sqlc.arg(report_id) AND dismissed_at IS NULL
    RETURNING id
)
UPDATE reports
SET moderation_state = 'removed',
    removed_reason = sqlc.arg(removed_reason)::text,
    removed_at = now()
WHERE reports.id = sqlc.arg(report_id);

-- name: InsertModerationAction :exec
-- One audit row per Keep, Remove, Restore or Keep removed. It holds the
-- moderator's account id, the action and, for a Remove, the reason picked
-- (quick 261118-rz6), never report text (T-04-15).
INSERT INTO moderation_actions (report_id, moderator_account_id, action, reason)
VALUES (sqlc.arg(report_id), sqlc.arg(moderator_account_id), sqlc.arg(action), sqlc.narg(reason));

-- name: InsertReportFlag :one
-- One flag per account per report. No row back means this account already
-- flagged this report (409). The flag itself never changes a column of
-- reports (D-09); since quick 261117-vvb HideReportForReview may, in the same
-- transaction. The flag upsert lives in this file because votes.sql must stay
-- free of conflict clauses and row locks.
-- The reasons are a set in the dialog's order and the note is trimmed text or
-- NULL (plan 04-31, migration 00010). flagger_cell is the flagger's place cell
-- computed by the server, or NULL when the flag came with no position
-- (migration 00026). It is never returned.
INSERT INTO report_flags (report_id, account_id, reasons, note, flagger_cell)
VALUES (sqlc.arg(report_id), sqlc.arg(account_id), sqlc.arg(reasons)::text[], sqlc.narg(note), sqlc.narg(flagger_cell))
ON CONFLICT (report_id, account_id) DO NOTHING RETURNING id;

-- name: LockReportModeration :one
-- Quick 261117-vvb: takes the row lock on one report and returns the facts the
-- flag hide and Keep and Remove decide from. FOR NO KEY UPDATE, because the
-- statements that follow change no key and a vote or flag insert that only
-- references the row must not wait on it. A flag takes it first, so two flags
-- on one report queue here and each later statement sees the other's committed
-- flag. Keep and Remove take it after LockModerationCheck, the check row first
-- and the report second, the order the worker takes. No row means no report.
SELECT moderation_state, severity, category
FROM reports
WHERE id = sqlc.arg(report_id)
FOR NO KEY UPDATE;

-- name: HideReportForReview :execrows
-- Quick 261117-vvb: hides a shown report for review once its open flags come
-- from at least threshold different place cells. One flag per account per
-- report means that many cells are that many accounts too. A flag with no cell
-- never counts and a dismissed one never counts. Only a clear report that no
-- moderator kept after a review hide moves, so a held, removed or already
-- hidden report never does and a hide happens at most once. One row back means
-- this call hid it. The threshold is the service's FlagHideThreshold.
UPDATE reports r
SET moderation_state = 'review'
WHERE r.id = sqlc.arg(report_id)
  AND r.moderation_state = 'clear'
  AND r.review_kept_at IS NULL
  AND (SELECT count(DISTINCT f.flagger_cell)
         FROM report_flags f
        WHERE f.report_id = r.id
          AND f.dismissed_at IS NULL
          AND f.flagger_cell IS NOT NULL) >= sqlc.arg(threshold)::bigint;

-- name: InsertOwnReportUpdate :execrows
-- Quick 261117-vvb: one bell row for the author of a report about what
-- moderation did to it (own_shown, own_review, own_kept, own_removed, and
-- since quick 261118-rz6 own_restored and own_stays_removed). It is written
-- straight to the author's updates, whatever their follows or mutes, and never
-- queued for a phone. A report whose author has no account, or a deleted one,
-- gets no row. Quick 261118-rz6: an own event can happen again, so a row of
-- the same kind that is already there moves to now and becomes unread again
-- instead of adding a second row (uq_updates_own_report). It also takes a
-- fresh id: MarkUpdatesRead marks every row up to the newest id shown, so a
-- row that kept its old id would be marked read by a list drawn before it
-- came back (review fix).
INSERT INTO updates (account_id, report_id, kind)
SELECT s.account_id, r.id, sqlc.arg(kind)::text
FROM reports r
JOIN sessions s ON s.session_id = r.session_id
JOIN accounts a ON a.id = s.account_id
WHERE r.id = sqlc.arg(report_id)
  AND a.deleted_at IS NULL
ON CONFLICT (account_id, report_id, kind) WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed', 'own_restored', 'own_stays_removed')
DO UPDATE SET id = nextval(pg_get_serial_sequence('updates', 'id')), created_at = now(), read_at = NULL;

-- name: ReviewAskQueue :many
-- Quick 261118-rz6, the Review asks list: each open ask for review on a report
-- that is still removed, oldest first, at most 100 rows, in the moderator row
-- shape with the reason of the removal and the author's note. The note is the
-- author's own words for moderators only; nothing here names who asked.
SELECT r.id, r.category, r.severity, r.description, r.created_at, r.expires_at,
       r.client_written_at, r.moderation_state, r.removed_reason,
       c.status AS check_status, c.verdict, c.scores, c.model,
       COALESCE(c.needs_review, false)::bool AS needs_review,
       (SELECT count(*) FROM report_flags f
         WHERE f.report_id = r.id AND f.dismissed_at IS NULL)::bigint AS open_flag_total,
       a.note AS ask_note
FROM report_review_asks a
JOIN reports r ON r.id = a.report_id
LEFT JOIN moderation_checks c ON c.report_id = r.id
WHERE a.answered_at IS NULL AND r.moderation_state = 'removed'
ORDER BY a.created_at ASC, a.id ASC
LIMIT 100;

-- name: RemovedQueue :many
-- Quick 261118-rz6, the Removed list: reports removed in the last 30 days,
-- newest removal first, at most 100 rows, in the moderator row shape with the
-- reason of the removal.
SELECT r.id, r.category, r.severity, r.description, r.created_at, r.expires_at,
       r.client_written_at, r.moderation_state, r.removed_reason,
       c.status AS check_status, c.verdict, c.scores, c.model,
       COALESCE(c.needs_review, false)::bool AS needs_review,
       (SELECT count(*) FROM report_flags f
         WHERE f.report_id = r.id AND f.dismissed_at IS NULL)::bigint AS open_flag_total
FROM reports r
LEFT JOIN moderation_checks c ON c.report_id = r.id
WHERE r.moderation_state = 'removed'
  AND r.removed_at > now() - interval '30 days'
ORDER BY r.removed_at DESC, r.id DESC
LIMIT 100;

-- name: RestoreReport :execrows
-- Quick 261118-rz6: a moderator's Restore shows a removed report again. It
-- moves only a removed report, to clear, clears the reason and time of the
-- removal, dismisses any open flags and stamps review_kept_at, so flags can
-- never hide it again and it never sends a phone alert again. The time it
-- lasts is not changed. One row back means this call restored it. The service
-- runs it inside the Restore transaction after the two row locks.
WITH dismissed AS (
    UPDATE report_flags
    SET dismissed_at = now()
    WHERE report_id = sqlc.arg(report_id) AND dismissed_at IS NULL
    RETURNING id
)
UPDATE reports
SET moderation_state = 'clear',
    removed_reason = NULL,
    removed_at = NULL,
    review_kept_at = now()
WHERE reports.id = sqlc.arg(report_id) AND reports.moderation_state = 'removed';

-- name: AnswerOpenReviewAsk :execrows
-- Quick 261118-rz6: closes the report's open ask for review, if there is one,
-- with the answer (restored or kept_removed) and the answering moderator. One
-- row back means an open ask was answered.
UPDATE report_review_asks
SET answered_at = now(),
    answer = sqlc.arg(answer)::text,
    answered_by_account_id = sqlc.arg(moderator_account_id)::bigint
WHERE report_id = sqlc.arg(report_id) AND answered_at IS NULL;
