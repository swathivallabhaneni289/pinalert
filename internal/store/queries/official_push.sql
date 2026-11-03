-- Phase 5 queries (plan 05-11): the official push switch, the recipient
-- candidate read, and the leased queue of official pushes. The official_push_outbox
-- table holds ids only: no words, no position and no endpoint. The lock screen
-- words are built at send time from closed tables, and recipients are read by
-- their stored home cell, never by a position.
--
-- A live device, wherever a query says so, is a push_subscriptions row joined to
-- sessions with sessions.revoked_at IS NULL, for an account whose
-- accounts.deleted_at IS NULL (the same rule as Phase 3.3).
--
-- Privacy rules the queries keep: the candidate read returns an account id and
-- its home cell to server code only, and the cell is never returned by a route
-- or written to a log. OfficialAlertForPush returns the alert's own facts and
-- its area codes and reads no person. The marks follow Phase 3.3's lease
-- pattern: a write that needs a claim carries the exact lease it holds and
-- reports how many rows it changed.
--
-- Every write that carries a time takes it as an argument, so tests can pin the
-- clock. The claim and the lease use the database clock, like Phase 3.3's.

-- name: GetOfficialPushSetting :one
-- The switch "Severe official alerts for my area" (D-11). An account with no
-- settings row reads it as on: the same value as the Go constant
-- DefaultOfficialPushOn, pinned by a test. Always returns a row for an existing
-- account. Owner decision 2026-11-03, quick 261103-jal: the column is kept and
-- no longer decides delivery; production code does not call this any more.
SELECT COALESCE(s.official_push_on, true)::bool AS official_push_on
FROM accounts a
LEFT JOIN account_alert_settings s ON s.account_id = a.id
WHERE a.id = sqlc.arg(account_id)::bigint;

-- name: UpsertOfficialPushSetting :one
-- Saves the switch. The other settings columns keep their stored values, or
-- their defaults on insert. Owner decision 2026-11-03, quick 261103-jal: kept
-- for tests that seed a value stored before this change (C6).
INSERT INTO account_alert_settings (account_id, official_push_on, updated_at)
VALUES (
    sqlc.arg(account_id)::bigint,
    sqlc.arg(official_push_on)::boolean,
    sqlc.arg(now)::timestamptz
)
ON CONFLICT (account_id) DO UPDATE
SET official_push_on = EXCLUDED.official_push_on,
    updated_at = EXCLUDED.updated_at
RETURNING COALESCE(official_push_on, true)::bool AS official_push_on;

-- name: OfficialPushCandidates :many
-- Who might be told: accounts with a home cell, not deleted and with at least
-- one live device, whose home cell lies under one of the given geohash
-- prefixes (the caller passes the prefixes that cover the alert's districts, so
-- the row cap applies only after the area filter: WR-S05 of the Phase 5
-- review). The stored official switch is not read: the device switch is the only
-- way to stop these notes (owner decision 2026-11-03, quick 261103-jal). Ordered
-- by account id and bounded by row_cap; the caller turns each home cell into
-- districts and keeps the accounts whose districts meet the alert's. It
-- selects by cell, never by a coordinate.
SELECT a.id AS account_id,
       COALESCE(a.home_area_cell, '')::text AS home_area_cell
FROM accounts a
WHERE a.deleted_at IS NULL
  AND a.home_area_cell IS NOT NULL
  AND left(a.home_area_cell, sqlc.arg(prefix_len)::int) = ANY(sqlc.arg(prefixes)::text[])
  AND EXISTS (
      SELECT 1
      FROM push_subscriptions p
      JOIN sessions ss ON ss.session_id = p.session_id
      WHERE p.account_id = a.id
        AND ss.revoked_at IS NULL
  )
ORDER BY a.id
LIMIT sqlc.arg(row_cap)::int;

-- name: EnqueueOfficialPush :execrows
-- Queues one push for one person and one version of an alert. A repeat for the
-- same person, alert and cap_identifier changes nothing (the unique index is
-- the second guard after the version's own transaction). Returns the number of
-- rows added: 1, or 0 for a repeat.
INSERT INTO official_push_outbox (account_id, alert_id, cap_identifier, reason, expires_at, created_at, next_attempt_at)
VALUES (
    sqlc.arg(account_id)::bigint,
    sqlc.arg(alert_id)::bigint,
    sqlc.arg(cap_identifier)::text,
    sqlc.arg(reason)::text,
    sqlc.arg(expires_at)::timestamptz,
    sqlc.arg(now)::timestamptz,
    sqlc.arg(now)::timestamptz
)
ON CONFLICT (account_id, alert_id, cap_identifier) DO NOTHING;

-- name: ClaimDueOfficialPushes :many
-- Hands due rows to one claimant each. The claimed set is fixed once in a
-- MATERIALIZED CTE and then updated, like ClaimDuePushes, because the planner
-- may rescan an IN subquery that carries LIMIT and SKIP LOCKED. The lease makes
-- a crashed claimant's row claimable again once it lapses; attempts counts every
-- claim. Escalated rows go first, then by id. The row carries the lease this
-- claim holds (lease_until), which every later write must present.
WITH due AS MATERIALIZED (
    SELECT o.id
    FROM official_push_outbox o
    WHERE o.state = 'queued'
      AND o.next_attempt_at <= now()
      AND (o.lease_until IS NULL OR o.lease_until < now())
    ORDER BY CASE o.reason WHEN 'escalated' THEN 0 ELSE 1 END, o.id
    LIMIT sqlc.arg(batch_limit)::int
    FOR UPDATE OF o SKIP LOCKED
)
UPDATE official_push_outbox c
SET lease_until = now() + sqlc.arg(lease_seconds)::int * interval '1 second',
    attempts = c.attempts + 1
FROM due
WHERE c.id = due.id
RETURNING c.id, c.account_id, c.alert_id, c.cap_identifier, c.reason,
          COALESCE(c.lease_until, now())::timestamptz AS lease_until,
          c.attempts, c.expires_at, c.created_at;

-- name: ExtendOfficialPushLease :one
-- The worker renews its lease just before it sends a row. It renews only for the
-- current holder: no row comes back when another claim took the row after its
-- lease passed.
UPDATE official_push_outbox
SET lease_until = now() + sqlc.arg(lease_seconds)::int * interval '1 second'
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz
RETURNING COALESCE(lease_until, now())::timestamptz AS lease_until;

-- name: MarkOfficialPushSent :execrows
-- The push service accepted the send. This mark and the three after it are
-- written only by the holder of the row's current lease: each is conditional on
-- state queued and on the exact lease the worker holds, and reports how many
-- rows it changed. Zero rows changed means the lease was lost and the caller
-- moves on. A row already sent can never be skipped, rescheduled or failed.
UPDATE official_push_outbox
SET state = 'sent',
    sent_at = sqlc.arg(now)::timestamptz,
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: MarkOfficialPushSkipped :execrows
-- The row will not be sent, for a reason from the closed list.
UPDATE official_push_outbox
SET state = 'skipped',
    skip_reason = sqlc.arg(skip_reason)::text,
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: RescheduleOfficialPush :execrows
-- A temporary failure: try again later.
UPDATE official_push_outbox
SET next_attempt_at = sqlc.arg(next_attempt_at)::timestamptz,
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: MarkOfficialPushFailed :execrows
-- A permanent failure: no retry.
UPDATE official_push_outbox
SET state = 'failed',
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: CountAccountOfficialPushesInWindow :one
-- The per account official cap (OfficialPushPerAccountPer6Hours): sent rows
-- after since. These rows are in their own table, so they never count toward
-- Phase 3.3's hourly cap.
SELECT COUNT(*)::bigint
FROM official_push_outbox
WHERE account_id = sqlc.arg(account_id)::bigint
  AND state = 'sent'
  AND sent_at > sqlc.arg(since)::timestamptz;

-- name: OfficialPushSentForAlert :one
-- Serves plan 05-16's Apple rule (added 2026-10-28, the owner's few
-- notifications rule): whether any official push about this alert was already
-- sent to this account. Only sent rows count, so a skipped or failed row never
-- makes a later push look like a repeat.
SELECT EXISTS (
    SELECT 1
    FROM official_push_outbox
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND alert_id = sqlc.arg(alert_id)::bigint
      AND state = 'sent'
)::boolean;

-- name: SkipStaleOfficialPushes :execrows
-- On wake, queued rows whose own life is over, or whose alert has ended, been
-- cancelled or been replaced, become skipped with reason stale instead of
-- flushing a backlog. A row a worker holds (lease not yet past) is left to that
-- worker: the same holder only rule as the marks.
UPDATE official_push_outbox o
SET state = 'skipped',
    skip_reason = 'stale',
    lease_until = NULL
WHERE o.state = 'queued'
  AND (o.lease_until IS NULL OR o.lease_until <= sqlc.arg(now)::timestamptz)
  AND (
      o.expires_at <= sqlc.arg(now)::timestamptz
      OR EXISTS (
          SELECT 1
          FROM official_alerts a
          WHERE a.id = o.alert_id
            AND (a.expires_at <= sqlc.arg(now)::timestamptz
                 OR a.cancelled_at IS NOT NULL
                 OR a.superseded_at IS NOT NULL)
      )
  );

-- name: PruneOfficialPushOutbox :exec
-- Deletes finished rows (sent, skipped or failed) created at or before edge.
-- Queued rows are never pruned.
DELETE FROM official_push_outbox
WHERE created_at <= sqlc.arg(edge)::timestamptz
  AND state <> 'queued';

-- name: OfficialAlertForPush :one
-- The facts of one stored alert that the queue and the send worker need, with
-- its area codes: district codes from the rows that carry a district and state
-- codes from the rows that carry a state. last_seen_at lets the worker prove an
-- alert is current while the refresh that stored it is still running. It reads
-- no person.
SELECT a.id, a.guid, a.cap_identifier, a.severity, a.event, a.sender, a.placement_tier,
       a.expires_at, a.cancelled_at, a.superseded_at, a.sent_at, a.last_seen_at,
       COALESCE(array_agg(DISTINCT ar.district_lgd) FILTER (WHERE ar.district_lgd IS NOT NULL), '{}')::int[] AS district_codes,
       COALESCE(array_agg(DISTINCT ar.state_lgd) FILTER (WHERE ar.state_lgd IS NOT NULL), '{}')::int[] AS state_codes
FROM official_alerts a
LEFT JOIN official_alert_areas ar ON ar.alert_id = a.id
WHERE a.id = sqlc.arg(id)::bigint
GROUP BY a.id;

-- name: OfficialPredecessorsForPush :many
-- The stored alerts, under any other guid, whose message identifier an Update
-- names in its references. The queue judges an Update that has no earlier
-- version under its own guid against them, so a wording only Update under a new
-- guid queues nobody. Each row carries the alert's area codes, district codes
-- from the district rows and state codes from the state rows.
SELECT a.id, a.severity, a.placement_tier,
       COALESCE(array_agg(DISTINCT ar.district_lgd) FILTER (WHERE ar.district_lgd IS NOT NULL), '{}')::int[] AS district_codes,
       COALESCE(array_agg(DISTINCT ar.state_lgd) FILTER (WHERE ar.state_lgd IS NOT NULL), '{}')::int[] AS state_codes
FROM official_alerts a
LEFT JOIN official_alert_areas ar ON ar.alert_id = a.id
WHERE a.cap_identifier = ANY(sqlc.arg(identifiers)::text[])
  AND a.guid <> sqlc.arg(guid)::text
GROUP BY a.id
ORDER BY a.id;

-- name: OfficialAlertWithdrawnByStored :one
-- Whether a stored message under another guid names this alert's message
-- identifier in its references (an Update that replaces it or a Cancel that
-- withdraws it). The refresher stores the newest message first, so the replacing
-- message can be stored before the alert it replaces, and that alert is then not
-- marked at the moment it is stored. The queue refuses to push it.
SELECT EXISTS (
    SELECT 1
    FROM official_alerts n
    WHERE n.guid <> sqlc.arg(guid)::text
      AND sqlc.arg(cap_identifier)::text = ANY(string_to_array(n.references_text, ' '))
)::boolean;

-- name: DeleteAccountOfficialPushOutbox :execrows
-- Delete my account: the official push rows of the person are personal rows,
-- like the updates and the Phase 3.3 push copy.
DELETE FROM official_push_outbox
WHERE account_id = sqlc.arg(account_id)::bigint;

-- name: OfficialAlertSeries :many
-- The alert series of one stored alert (plan 05-16, amended 2026-10-29): the
-- alert itself plus every stored alert it names through references_text, and
-- every one those name in turn, followed back step by step. SACHET sends an
-- Update under a new guid that names the earlier message, and the refresher
-- stores it as a new row, so the series is the link between those rows.
-- references_text is a space separated list of full CAP identifiers, matched to
-- cap_identifier. The walk uses UNION and stops at depth 20, so a references
-- cycle ends; the final select collapses the walk to one row per alert. A row
-- reports names_member when it names another member of the series: the root is
-- the member that names none. Rows come back oldest first.
WITH RECURSIVE walk(id, depth) AS (
    SELECT a.id, 0
    FROM official_alerts a
    WHERE a.id = sqlc.arg(alert_id)::bigint
    UNION
    SELECT p.id, w.depth + 1
    FROM walk w
    JOIN official_alerts n ON n.id = w.id
    JOIN official_alerts p
      ON p.cap_identifier = ANY(string_to_array(n.references_text, ' '))
     AND p.id <> n.id
    WHERE w.depth < 20
),
members AS (
    SELECT DISTINCT id FROM walk
)
SELECT a.id,
       a.guid,
       a.sent_at,
       EXISTS (
           SELECT 1
           FROM official_alerts b
           WHERE b.id IN (SELECT id FROM members)
             AND b.id <> a.id
             AND b.cap_identifier = ANY(string_to_array(a.references_text, ' '))
       )::boolean AS names_member
FROM official_alerts a
WHERE a.id IN (SELECT id FROM members)
ORDER BY a.sent_at, a.id;

-- name: OfficialPushSentForSeries :one
-- Serves plan 05-16's Apple rule per alert series (amended 2026-10-29): whether
-- any official push about any alert of the series was already sent to this
-- account. Only sent rows count, so a skipped or failed row never makes a later
-- push look like a repeat. OfficialPushSentForAlert stays as it is.
SELECT EXISTS (
    SELECT 1
    FROM official_push_outbox
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND alert_id = ANY(sqlc.arg(alert_ids)::bigint[])
      AND state = 'sent'
)::boolean;
