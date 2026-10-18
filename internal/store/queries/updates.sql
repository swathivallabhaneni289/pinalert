-- Phase 3.2 queries: in app updates, mutes and the trusted reporter alert
-- settings. SQL never decides visibility (TRUST-02): the Go resolver does, and
-- these queries only hand it the raw facts. No query here selects a session id,
-- a voter cell or a coordinate (T-01-02); a session id appears only inside a
-- JOIN condition. Every read and write is scoped by the account id argument,
-- which later plans pass from the sign-in gate alone.

-- name: FollowedOpenReports :many
-- The followed set (D-09, D-10), derived and never stored: unexpired reports the
-- account posted through any of its sessions (revoked sessions included, so
-- authorship never moves) plus unexpired reports it has any vote row on, of any
-- kind including unresolve. Bounded by report lifetime and by row_cap; a heavy
-- account loses the oldest. muted comes from the account's own mute row, which
-- the primary key on report_mutes allows at most once, so no report repeats.
WITH followed AS (
    SELECT r.id AS report_id
    FROM reports r
    JOIN sessions s ON s.session_id = r.session_id
    WHERE s.account_id = sqlc.arg(account_id)::bigint
      AND r.expires_at > sqlc.arg(now)::timestamptz
    UNION
    SELECT v.report_id
    FROM votes v
    WHERE v.account_id = sqlc.arg(account_id)::bigint
)
SELECT r.id, r.severity, r.category, r.created_at, r.expires_at, r.moderation_state,
       (m.account_id IS NOT NULL)::bool AS muted
FROM followed f
JOIN reports r ON r.id = f.report_id
LEFT JOIN report_mutes m ON m.report_id = r.id AND m.account_id = sqlc.arg(account_id)::bigint
WHERE r.expires_at > sqlc.arg(now)::timestamptz
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(row_cap)::int;

-- name: UpdateCursorsForReports :many
-- The account's cursor rows for a batch of reports (D-12, D-13). A report with
-- no row here has not been seen by a refresh yet and gets a silent baseline.
SELECT report_id, last_state, last_update_at, adopted_at, fading_for
FROM update_cursors
WHERE account_id = sqlc.arg(account_id)::bigint
  AND report_id = ANY(sqlc.arg(report_ids)::bigint[]);

-- name: InsertBaselineCursor :exec
-- The silent first look at a report (D-13): the current visibility becomes the
-- baseline and nothing is announced. A concurrent insert for the same pair is a
-- no-op.
INSERT INTO update_cursors (account_id, report_id, last_state, adopted_at)
VALUES (sqlc.arg(account_id)::bigint, sqlc.arg(report_id)::bigint, sqlc.arg(state)::text, sqlc.arg(now)::timestamptz)
ON CONFLICT (account_id, report_id) DO NOTHING;

-- name: AdoptCursorState :execrows
-- Silently adopts a new visibility (the account's own vote caused it, so it is
-- not news). Compare and swap on the old state; adopted_at always moves, so an
-- own vote settles after one poll.
UPDATE update_cursors
SET last_state = sqlc.arg(new_state)::text,
    adopted_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id)::bigint
  AND report_id = sqlc.arg(report_id)::bigint
  AND last_state = sqlc.arg(old_state)::text;

-- name: AdvanceCursorAndInsertUpdate :one
-- The exactly once event write (D-12, D-16). One statement with data modifying
-- CTEs: moved is a compare and swap on the cursor (the old state must still be
-- stored and the last update for this pair must be older than gap_edge, the one
-- per report per hour rule), so of two concurrent callers the second waits for
-- the row lock, re-reads the new state and moves nothing. recent counts the
-- account's stored updates after hour_edge, not counting the rolled up kind.
-- Under the ceiling one normal row is inserted. At or over it the event is
-- rolled into one row per account per clock hour (bucket_start) whose count
-- grows and which becomes unread again, so a blocked event is never announced
-- later as a normal row. No row back means another request moved the cursor
-- first or the hour gap blocked it; both are fine and change nothing. The
-- ceiling is soft: two concurrent polls on different reports can each see
-- ceiling minus one and both insert, so one account can exceed it by one. Only
-- the per report rule is exact.
WITH moved AS (
    UPDATE update_cursors
    SET last_state = sqlc.arg(new_state)::text,
        last_update_at = sqlc.arg(now)::timestamptz
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND report_id = sqlc.arg(report_id)::bigint
      AND last_state = sqlc.arg(old_state)::text
      AND (last_update_at IS NULL OR last_update_at <= sqlc.arg(gap_edge)::timestamptz)
    RETURNING account_id, report_id
), recent AS (
    SELECT COUNT(*)::bigint AS n
    FROM updates
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND kind <> 'more'
      AND created_at > sqlc.arg(hour_edge)::timestamptz
), ins AS (
    INSERT INTO updates (account_id, report_id, kind, created_at)
    SELECT m.account_id, m.report_id, sqlc.arg(kind)::text, sqlc.arg(now)::timestamptz
    FROM moved m, recent
    WHERE recent.n < sqlc.arg(ceiling)::bigint
    RETURNING id
), more AS (
    INSERT INTO updates (account_id, report_id, kind, created_at, bucket_start, more_count)
    SELECT m.account_id, NULL::bigint, 'more', sqlc.arg(now)::timestamptz, sqlc.arg(bucket_start)::timestamptz, 1
    FROM moved m, recent
    WHERE recent.n >= sqlc.arg(ceiling)::bigint
    ON CONFLICT (account_id, bucket_start) WHERE kind = 'more' DO UPDATE
    SET more_count = updates.more_count + 1,
        created_at = EXCLUDED.created_at,
        read_at = NULL
    RETURNING id
)
SELECT id FROM ins
UNION ALL
SELECT id FROM more;

-- name: MarkFadingAndInsertUpdate :one
-- The fading event (D-14), written exactly like AdvanceCursorAndInsertUpdate
-- (same hour gap, same soft ceiling, same roll up) but keyed on the expiry that
-- was announced: fading_for must differ from expires_at, so the same expiry is
-- announced once and a changed expiry arms the event again.
WITH moved AS (
    UPDATE update_cursors
    SET fading_for = sqlc.arg(expires_at)::timestamptz,
        last_update_at = sqlc.arg(now)::timestamptz
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND report_id = sqlc.arg(report_id)::bigint
      AND fading_for IS DISTINCT FROM sqlc.arg(expires_at)::timestamptz
      AND (last_update_at IS NULL OR last_update_at <= sqlc.arg(gap_edge)::timestamptz)
    RETURNING account_id, report_id
), recent AS (
    SELECT COUNT(*)::bigint AS n
    FROM updates
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND kind <> 'more'
      AND created_at > sqlc.arg(hour_edge)::timestamptz
), ins AS (
    INSERT INTO updates (account_id, report_id, kind, created_at)
    SELECT m.account_id, m.report_id, 'fading', sqlc.arg(now)::timestamptz
    FROM moved m, recent
    WHERE recent.n < sqlc.arg(ceiling)::bigint
    RETURNING id
), more AS (
    INSERT INTO updates (account_id, report_id, kind, created_at, bucket_start, more_count)
    SELECT m.account_id, NULL::bigint, 'more', sqlc.arg(now)::timestamptz, sqlc.arg(bucket_start)::timestamptz, 1
    FROM moved m, recent
    WHERE recent.n >= sqlc.arg(ceiling)::bigint
    ON CONFLICT (account_id, bucket_start) WHERE kind = 'more' DO UPDATE
    SET more_count = updates.more_count + 1,
        created_at = EXCLUDED.created_at,
        read_at = NULL
    RETURNING id
)
SELECT id FROM ins
UNION ALL
SELECT id FROM more;

-- name: InsertReliableAlert :one
-- A trusted reporter alert (NOTIF-05, D-26), once per account per report: the
-- partial unique index uq_updates_reliable_once turns a repeat into a no-op.
-- Written only while the account is under the hourly ceiling; over it nothing is
-- written and the next poll tries again while the report is still unexpired.
INSERT INTO updates (account_id, report_id, kind, created_at)
SELECT sqlc.arg(account_id)::bigint, sqlc.arg(report_id)::bigint, 'reliable_nearby', sqlc.arg(now)::timestamptz
WHERE (
    SELECT COUNT(*)
    FROM updates u
    WHERE u.account_id = sqlc.arg(account_id)::bigint
      AND u.kind <> 'more'
      AND u.created_at > sqlc.arg(hour_edge)::timestamptz
) < sqlc.arg(ceiling)::bigint
ON CONFLICT (account_id, report_id) WHERE kind = 'reliable_nearby' DO NOTHING
RETURNING id;

-- name: AlertsFromReporterInWindow :one
-- The per reporter cap (D-26) without storing the recipient to reporter link:
-- how many trusted reporter alerts the recipient got after window_start for
-- reports filed from a session bound to reporter_id. Bounded to one recipient
-- and one window; the session id is read only inside the join.
SELECT COUNT(*)::bigint
FROM updates u
JOIN reports r ON r.id = u.report_id
JOIN sessions s ON s.session_id = r.session_id
WHERE u.account_id = sqlc.arg(recipient_id)::bigint
  AND u.kind = 'reliable_nearby'
  AND s.account_id = sqlc.arg(reporter_id)::bigint
  AND u.created_at > sqlc.arg(window_start)::timestamptz;

-- name: UpdatesForAccount :many
-- The account's updates since a time, newest first (NOTIF-01, D-06). Served by
-- idx_updates_account_created. Kind and report id only: the words are written
-- from the report's current facts at read time.
SELECT id, report_id, kind, created_at, read_at, more_count
FROM updates
WHERE account_id = sqlc.arg(account_id)::bigint
  AND created_at > sqlc.arg(since)::timestamptz
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountUpdatesForAccount :one
-- How many updates the account has since a time, for the page count.
SELECT COUNT(*)::bigint
FROM updates
WHERE account_id = sqlc.arg(account_id)::bigint
  AND created_at > sqlc.arg(since)::timestamptz;

-- name: UnreadUpdateCount :one
-- The bell number (D-06). The inner LIMIT stops the count at cap, so a very
-- large backlog costs at most cap index entries. Served by
-- idx_updates_account_unread.
SELECT COUNT(*)::bigint
FROM (
    SELECT 1
    FROM updates
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND read_at IS NULL
      AND created_at > sqlc.arg(since)::timestamptz
    LIMIT sqlc.arg(cap)::int
) AS unread;

-- name: MarkUpdatesRead :execrows
-- Marks the account's own unread updates up to and including up_to as read.
-- Scoped by account_id, so another account's rows are never touched. Returns
-- the affected count.
UPDATE updates
SET read_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id)::bigint
  AND id <= sqlc.arg(up_to)::bigint
  AND read_at IS NULL;

-- name: PruneUpdates :exec
-- Deletes the account's updates created at or before edge (the 30 day window).
DELETE FROM updates
WHERE account_id = sqlc.arg(account_id)::bigint
  AND created_at <= sqlc.arg(edge)::timestamptz;

-- name: PruneCursors :exec
-- Deletes the account's cursor rows whose report expired at or before edge; an
-- expired report can never change state again.
DELETE FROM update_cursors c
USING reports r
WHERE r.id = c.report_id
  AND c.account_id = sqlc.arg(account_id)::bigint
  AND r.expires_at <= sqlc.arg(edge)::timestamptz;

-- name: ReportsForUpdates :many
-- The reports behind a page of updates, in one batch, at any expiry. The
-- columns the sentence and the resolver need; no coordinate column.
SELECT id, severity, category, description, created_at, expires_at, moderation_state
FROM reports
WHERE id = ANY(sqlc.arg(report_ids)::bigint[]);

-- name: ReportForMute :one
-- One report's identity for the mute action (D-11): enough for the caller to
-- check the report exists and is not withheld. No row means no such report.
SELECT id, severity, category, moderation_state
FROM reports
WHERE id = sqlc.arg(report_id)::bigint;

-- name: MuteReport :exec
-- Mute this report (D-11). One statement: the mute is inserted (a repeat is a
-- no-op) and the account's unread updates for that report are deleted, so a
-- muted report leaves nothing behind in the bell. Read updates stay.
WITH muted AS (
    INSERT INTO report_mutes (account_id, report_id)
    VALUES (sqlc.arg(account_id)::bigint, sqlc.arg(report_id)::bigint)
    ON CONFLICT (account_id, report_id) DO NOTHING
    RETURNING report_id
)
DELETE FROM updates
WHERE account_id = sqlc.arg(account_id)::bigint
  AND report_id = sqlc.arg(report_id)::bigint
  AND read_at IS NULL;

-- name: UnmuteReport :exec
-- Unmute (D-11). One statement: the mute row and the account's cursor row for
-- that report are deleted, so the next refresh starts a fresh silent baseline
-- and nothing from the muted time is announced late.
WITH unmuted AS (
    DELETE FROM report_mutes
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND report_id = sqlc.arg(report_id)::bigint
    RETURNING report_id
)
DELETE FROM update_cursors
WHERE account_id = sqlc.arg(account_id)::bigint
  AND report_id = sqlc.arg(report_id)::bigint;

-- name: MutedReportsForAccount :many
-- The Muted reports list, newest mute first. The report's current facts come
-- from the join; no coordinate column.
SELECT m.report_id, m.created_at AS muted_at, r.severity, r.category, r.description,
       r.expires_at, r.moderation_state
FROM report_mutes m
JOIN reports r ON r.id = m.report_id
WHERE m.account_id = sqlc.arg(account_id)::bigint
ORDER BY m.created_at DESC, m.report_id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountMutedReports :one
SELECT COUNT(*)::bigint
FROM report_mutes
WHERE account_id = sqlc.arg(account_id)::bigint;

-- name: GetAlertSettings :one
-- The account's alert switch and distance (D-26). No row means the defaults:
-- off, 5 km.
SELECT reliable_alerts_on, alert_radius_km, enabled_at
FROM account_alert_settings
WHERE account_id = sqlc.arg(account_id)::bigint;

-- name: UpsertAlertSettings :one
-- Saves the switch and the distance in one statement. enabled_at becomes now
-- when the switch goes from off (or no row) to on, stays as stored while it
-- stays on, and becomes NULL when it is off. An alert is only ever raised for a
-- report filed after enabled_at.
INSERT INTO account_alert_settings (account_id, reliable_alerts_on, alert_radius_km, enabled_at, updated_at)
VALUES (
    sqlc.arg(account_id)::bigint,
    sqlc.arg(reliable_alerts_on)::boolean,
    sqlc.arg(alert_radius_km)::smallint,
    CASE WHEN sqlc.arg(reliable_alerts_on)::boolean THEN sqlc.arg(now)::timestamptz ELSE NULL END,
    sqlc.arg(now)::timestamptz
)
ON CONFLICT (account_id) DO UPDATE
SET reliable_alerts_on = EXCLUDED.reliable_alerts_on,
    alert_radius_km = EXCLUDED.alert_radius_km,
    enabled_at = CASE
        WHEN NOT EXCLUDED.reliable_alerts_on THEN NULL
        WHEN account_alert_settings.reliable_alerts_on THEN COALESCE(account_alert_settings.enabled_at, EXCLUDED.enabled_at)
        ELSE EXCLUDED.enabled_at
    END,
    updated_at = EXCLUDED.updated_at
RETURNING reliable_alerts_on, alert_radius_km, enabled_at;

-- name: DisableReliableAlerts :exec
-- Switches the account's alerts off (for example when its home area is
-- cleared) and forgets when they were enabled.
UPDATE account_alert_settings
SET reliable_alerts_on = false,
    enabled_at = NULL,
    updated_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id)::bigint;
