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
-- it writes has a column that holds report text (T-04-04).

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
-- counts every claim. A check is never claimed when its report has expired or
-- was removed, when it is done, or when it is not yet due. The lease and the
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
      AND r2.moderation_state <> 'removed'
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

-- name: FinishModerationCheck :exec
-- Records the worker's verdict. Only a check that is still pending is
-- written, so a check already closed by Keep or Remove is never reopened or
-- overwritten by a late worker pass.
UPDATE moderation_checks
SET status = 'done',
    verdict = sqlc.arg(verdict)::text,
    scores = sqlc.narg(scores)::jsonb,
    model = sqlc.narg(model),
    needs_review = sqlc.arg(needs_review)::bool,
    checked_at = now(),
    lease_until = NULL
WHERE report_id = sqlc.arg(report_id)
  AND status = 'pending';

-- name: RescheduleModerationCheck :exec
-- The provider did not answer: try again later. last_error holds an error
-- class such as a status code, never report text. Only a pending check moves.
UPDATE moderation_checks
SET next_attempt_at = sqlc.arg(next_attempt_at),
    lease_until = NULL,
    last_error = sqlc.narg(last_error)
WHERE report_id = sqlc.arg(report_id)
  AND status = 'pending';

-- name: SetReportModerationState :exec
-- The worker's write to the report row after a verdict. Removal is terminal,
-- so a late worker pass can never undo a moderator Remove (R-03). A check
-- closed by Keep or Remove can no longer change the state: the worker is the
-- only caller and always writes while the check is still pending, so a late
-- worker reject cannot withdraw a report a moderator kept.
UPDATE reports
SET moderation_state = sqlc.arg(moderation_state)
WHERE id = sqlc.arg(report_id)
  AND moderation_state <> 'removed'
  AND EXISTS (
      SELECT 1 FROM moderation_checks mc
      WHERE mc.report_id = reports.id AND mc.status = 'pending'
  );

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
                   FROM (SELECT f.reason, count(*) AS n FROM report_flags f
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL
                          GROUP BY f.reason) x), '{}'::jsonb)::jsonb AS flag_counts
FROM reports r
JOIN moderation_checks c ON c.report_id = r.id
WHERE c.needs_review AND r.moderation_state <> 'removed'
ORDER BY r.created_at DESC, r.id DESC
LIMIT 100;

-- name: FlagQueue :many
-- List (b): posts with at least one open flag, most flags first and then
-- newest, at most 100 rows. LEFT JOIN because a flagged report may have no
-- check row (seeded and older reports).
SELECT r.id, r.category, r.severity, r.description, r.created_at, r.expires_at,
       r.client_written_at, r.moderation_state,
       c.status AS check_status, c.verdict, c.scores, c.model,
       COALESCE(c.needs_review, false)::bool AS needs_review,
       (SELECT count(*) FROM report_flags f
         WHERE f.report_id = r.id AND f.dismissed_at IS NULL)::bigint AS open_flag_total,
       COALESCE((SELECT jsonb_object_agg(x.reason, x.n)
                   FROM (SELECT f.reason, count(*) AS n FROM report_flags f
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL
                          GROUP BY f.reason) x), '{}'::jsonb)::jsonb AS flag_counts
FROM reports r
LEFT JOIN moderation_checks c ON c.report_id = r.id
WHERE EXISTS (SELECT 1 FROM report_flags f WHERE f.report_id = r.id AND f.dismissed_at IS NULL)
  AND r.moderation_state <> 'removed'
ORDER BY open_flag_total DESC, r.created_at DESC, r.id DESC
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
                   FROM (SELECT f.reason, count(*) AS n FROM report_flags f
                          WHERE f.report_id = r.id AND f.dismissed_at IS NULL
                          GROUP BY f.reason) x), '{}'::jsonb)::jsonb AS flag_counts
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
        AND r.moderation_state <> 'removed')::bigint AS pending_count;

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

-- name: KeepReport :exec
-- Keep as one statement. Two data modifying CTEs close the check and dismiss
-- the open flags; the main statement releases a pending report to clear. Keep
-- accepts a pending post as it is and never touches a removed one, so removal
-- stays terminal (D-08, R-03).
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
SET moderation_state = 'clear'
WHERE reports.id = sqlc.arg(report_id) AND reports.moderation_state = 'pending';

-- name: RemoveReport :exec
-- Remove as one statement, with the same two CTEs as Keep. The main statement
-- sets removed on any report, listed or not, and removal is terminal (D-08,
-- R-03). The check is closed so the worker never claims it again.
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
SET moderation_state = 'removed'
WHERE reports.id = sqlc.arg(report_id);

-- name: InsertModerationAction :exec
-- One audit row per Keep or Remove. It holds the moderator's account id and
-- the action, never report text (T-04-15).
INSERT INTO moderation_actions (report_id, moderator_account_id, action)
VALUES (sqlc.arg(report_id), sqlc.arg(moderator_account_id), sqlc.arg(action));

-- name: InsertReportFlag :one
-- One flag per account per report. No row back means this account already
-- flagged this report (409). A flag is stored only here; it never changes a
-- column of reports (D-09). The flag upsert lives in this file because
-- votes.sql must stay free of conflict clauses and row locks.
INSERT INTO report_flags (report_id, account_id, reason)
VALUES (sqlc.arg(report_id), sqlc.arg(account_id), sqlc.arg(reason))
ON CONFLICT (report_id, account_id) DO NOTHING RETURNING id;
