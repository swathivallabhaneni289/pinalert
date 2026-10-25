-- Phase 3.3 queries: phone alert subscriptions, the recipient candidate read,
-- the send queue (outbox) and the per report fan out bookkeeping. SQL never
-- decides visibility (TRUST-02): the Go resolver does, and these queries only
-- select candidates and hand the worker raw facts.
--
-- A live subscription, wherever a query says so, is a push_subscriptions row
-- joined to sessions on session_id with sessions.revoked_at IS NULL, for an
-- account whose accounts.deleted_at IS NULL.
--
-- Privacy rules the queries keep: the endpoint is a capability URL and is
-- returned by exactly one query (PushSubscriptionsForAccount, the send path);
-- the recipient candidate read selects accounts by their home cell and returns
-- no position of any kind; only PushReportFacts, which is read by server code
-- alone, returns a report's position, for the distance. No query here reads a
-- voter cell.
--
-- Every write that carries a time takes it as an argument, so tests can pin the
-- clock. The claim and the lease marks use the database clock for the lease,
-- like the moderation worker does.

-- name: UpsertPushSubscription :one
-- Registers a device (D-13). The endpoint is unique, so the same endpoint
-- registered by another account moves to the caller: a shared phone must stop
-- delivering the previous account's alerts, and the failure count starts over.
INSERT INTO push_subscriptions (
    account_id, session_id, endpoint, endpoint_host, p256dh, auth, created_at
) VALUES (
    sqlc.arg(account_id)::bigint,
    sqlc.arg(session_id)::text,
    sqlc.arg(endpoint)::text,
    sqlc.arg(endpoint_host)::text,
    sqlc.arg(p256dh)::text,
    sqlc.arg(auth)::text,
    sqlc.arg(now)::timestamptz
)
ON CONFLICT (endpoint) DO UPDATE
SET account_id = EXCLUDED.account_id,
    session_id = EXCLUDED.session_id,
    endpoint_host = EXCLUDED.endpoint_host,
    p256dh = EXCLUDED.p256dh,
    auth = EXCLUDED.auth,
    created_at = EXCLUDED.created_at,
    last_ok_at = NULL,
    failures = 0
RETURNING id;

-- name: TrimPushSubscriptions :execrows
-- Keeps the newest keep devices of one account and deletes the rest (at most 8
-- devices per account, drop the oldest). Never touches another account.
DELETE FROM push_subscriptions
WHERE account_id = sqlc.arg(account_id)::bigint
  AND id NOT IN (
      SELECT p.id
      FROM push_subscriptions p
      WHERE p.account_id = sqlc.arg(account_id)::bigint
      ORDER BY p.created_at DESC, p.id DESC
      LIMIT sqlc.arg(keep)::int
  );

-- name: PushSubscriptionRegistered :one
-- Whether this account has this endpoint registered, so the Settings switch can
-- say the truth about this device. The endpoint arrives in a request body and
-- is compared in SQL only; it is not returned.
SELECT EXISTS (
    SELECT 1
    FROM push_subscriptions
    WHERE account_id = sqlc.arg(account_id)::bigint
      AND endpoint = sqlc.arg(endpoint)::text
)::bool AS registered;

-- name: DeletePushSubscriptionForAccount :execrows
-- Turning alerts off on this device: deletes the caller's own row only, so an
-- endpoint that belongs to another account is never touched.
DELETE FROM push_subscriptions
WHERE account_id = sqlc.arg(account_id)::bigint
  AND endpoint = sqlc.arg(endpoint)::text;

-- name: DeletePushSubscriptionsForSession :execrows
-- Plain Log out: the devices registered by this session stop receiving alerts.
DELETE FROM push_subscriptions
WHERE session_id = sqlc.arg(session_id)::text;

-- name: DeleteAccountPushSubscriptions :execrows
-- Log out of all devices and Delete my account: every device of the account.
DELETE FROM push_subscriptions
WHERE account_id = sqlc.arg(account_id)::bigint;

-- name: PushSubscriptionsForAccount :many
-- The send path (T-03.3-02, T-03.3-11): the live devices of one account, with
-- the keys the encryption needs. The only query that returns an endpoint.
SELECT p.id, p.endpoint, p.endpoint_host, p.p256dh, p.auth
FROM push_subscriptions p
JOIN sessions ss ON ss.session_id = p.session_id
JOIN accounts a ON a.id = p.account_id
WHERE p.account_id = sqlc.arg(account_id)::bigint
  AND ss.revoked_at IS NULL
  AND a.deleted_at IS NULL
ORDER BY p.id;

-- name: MarkPushSubscriptionOK :exec
-- The push service accepted a send: the device is healthy again.
UPDATE push_subscriptions
SET last_ok_at = sqlc.arg(now)::timestamptz,
    failures = 0
WHERE id = sqlc.arg(id)::bigint;

-- name: DeletePushSubscriptionByID :exec
-- The push service said the device is gone (404 or 410), or the worker gave up
-- on it.
DELETE FROM push_subscriptions
WHERE id = sqlc.arg(id)::bigint;

-- name: BumpPushSubscriptionFailure :one
-- One more failure that points at the device. The worker deletes the device
-- only when the returned count reaches its named limit AND last_ok_at is NULL
-- or older than the stale window, and it calls this only for answers that
-- point at the device (review notes N5 and W6).
UPDATE push_subscriptions
SET failures = failures + 1
WHERE id = sqlc.arg(id)::bigint
RETURNING failures, last_ok_at;

-- name: GetPushKindSettings :one
-- The Critical and Changes switches (decision 12). An account with no settings
-- row reads Critical as on and Changes as off: the same value as the Go
-- constant DefaultCriticalPushOn. Always returns a row for an existing account.
SELECT COALESCE(s.critical_push_on, true)::bool AS critical_push_on,
       COALESCE(s.changes_push_on, false)::bool AS changes_push_on,
       s.changes_enabled_at
FROM accounts a
LEFT JOIN account_alert_settings s ON s.account_id = a.id
WHERE a.id = sqlc.arg(account_id)::bigint;

-- name: UpsertPushKindSettings :one
-- Saves the two switches in one statement. changes_enabled_at becomes now when
-- Changes goes from off (or no row) to on, stays while it stays on, and becomes
-- NULL when it is off. A change push is only ever sent for an event after it.
-- The 3.2 columns keep their stored values, or their defaults on insert.
INSERT INTO account_alert_settings (
    account_id, critical_push_on, changes_push_on, changes_enabled_at, updated_at
) VALUES (
    sqlc.arg(account_id)::bigint,
    sqlc.arg(critical_push_on)::boolean,
    sqlc.arg(changes_push_on)::boolean,
    CASE WHEN sqlc.arg(changes_push_on)::boolean THEN sqlc.arg(now)::timestamptz ELSE NULL END,
    sqlc.arg(now)::timestamptz
)
ON CONFLICT (account_id) DO UPDATE
SET critical_push_on = EXCLUDED.critical_push_on,
    changes_push_on = EXCLUDED.changes_push_on,
    changes_enabled_at = CASE
        WHEN NOT EXCLUDED.changes_push_on THEN NULL
        WHEN account_alert_settings.changes_push_on THEN COALESCE(account_alert_settings.changes_enabled_at, EXCLUDED.changes_enabled_at)
        ELSE EXCLUDED.changes_enabled_at
    END,
    updated_at = EXCLUDED.updated_at
RETURNING critical_push_on, changes_push_on, changes_enabled_at;

-- name: PushHomeCandidates :many
-- The inverse lookup: accounts whose home cell is in the caller's list, with a
-- live subscription and no mute on the report. The caller lists the report's
-- own cell first and then the cells around it by distance, and the result is
-- ordered by the position of the home cell in that list before LIMIT, so under
-- the row cap the nearest accounts are kept and the farthest dropped (review
-- note N1). Selects by cell, never by a coordinate; the caller decides the
-- distance and the rest of the rules in Go (D-20). The defaults of an account
-- with no settings row are 5 km, Critical on, Reliable off.
SELECT a.id AS account_id,
       COALESCE(a.home_area_cell, '')::text AS home_area_cell,
       COALESCE(s.alert_radius_km, 5)::smallint AS alert_radius_km,
       COALESCE(s.critical_push_on, true)::bool AS critical_push_on,
       COALESCE(s.reliable_alerts_on, false)::bool AS reliable_alerts_on
FROM accounts a
LEFT JOIN account_alert_settings s ON s.account_id = a.id
WHERE a.deleted_at IS NULL
  AND a.home_area_cell = ANY(sqlc.arg(cells)::text[])
  AND EXISTS (
      SELECT 1
      FROM push_subscriptions p
      JOIN sessions ss ON ss.session_id = p.session_id
      WHERE p.account_id = a.id
        AND ss.revoked_at IS NULL
  )
  AND NOT EXISTS (
      SELECT 1
      FROM report_mutes m
      WHERE m.account_id = a.id
        AND m.report_id = sqlc.arg(report_id)::bigint
  )
ORDER BY array_position(sqlc.arg(cells)::text[], a.home_area_cell), a.id
LIMIT sqlc.arg(row_cap)::int;

-- name: FollowersWithChangesPush :many
-- Who may hear a change about a report (PWA-06): the accounts that posted it
-- (through any of their sessions) or have any vote row on it, with Changes
-- switched on in an explicit settings row (the default is off), not deleted,
-- not muted, with a live subscription.
WITH people AS (
    SELECT ss.account_id
    FROM reports r
    JOIN sessions ss ON ss.session_id = r.session_id
    WHERE r.id = sqlc.arg(report_id)::bigint
      AND ss.account_id IS NOT NULL
    UNION
    SELECT v.account_id
    FROM votes v
    WHERE v.report_id = sqlc.arg(report_id)::bigint
)
SELECT a.id AS account_id
FROM people pe
JOIN accounts a ON a.id = pe.account_id
JOIN account_alert_settings st ON st.account_id = a.id AND st.changes_push_on
WHERE a.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM report_mutes m
      WHERE m.account_id = a.id
        AND m.report_id = sqlc.arg(report_id)::bigint
  )
  AND EXISTS (
      SELECT 1
      FROM push_subscriptions p
      JOIN sessions ps ON ps.session_id = p.session_id
      WHERE p.account_id = a.id
        AND ps.revoked_at IS NULL
  )
ORDER BY a.id;

-- name: AccountsWithChangesPush :many
-- Every account that wants change pushes and has a live subscription, for the
-- sweep that turns 3.2 updates rows into outbox rows. Bounded by row_cap.
SELECT a.id AS account_id
FROM accounts a
JOIN account_alert_settings st ON st.account_id = a.id AND st.changes_push_on
WHERE a.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM push_subscriptions p
      JOIN sessions ps ON ps.session_id = p.session_id
      WHERE p.account_id = a.id
        AND ps.revoked_at IS NULL
  )
ORDER BY a.id
LIMIT sqlc.arg(row_cap)::int;

-- name: PushReportFacts :one
-- The facts the worker needs to decide and to write the words. Read only by
-- server code: the position never leaves the server (D-20, T-03.2-14) and the
-- area name is read by no feed query. reporter_account_id comes from the
-- session binding and is NULL for a report posted without an account.
SELECT r.id, r.severity, r.category, r.happening, r.area_name,
       r.latitude, r.longitude,
       r.created_at, r.expires_at, r.moderation_state,
       ss.account_id AS reporter_account_id
FROM reports r
LEFT JOIN sessions ss ON ss.session_id = r.session_id
WHERE r.id = sqlc.arg(report_id)::bigint;

-- name: RecentReportsForPush :many
-- The sweep that recovers a lost poke: ids of reports created after since and
-- not yet expired at now, newest first, at most row_cap.
SELECT id
FROM reports
WHERE created_at > sqlc.arg(since)::timestamptz
  AND expires_at > sqlc.arg(now)::timestamptz
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_cap)::int;

-- name: SetReportAreaName :exec
-- Stores the coarse area name from the reverse lookup (D-19). Read only by push
-- code, never by the feed.
UPDATE reports
SET area_name = sqlc.arg(area_name)::text
WHERE id = sqlc.arg(report_id)::bigint;

-- name: MarkAreaLookupTried :exec
-- Records that the reverse lookup was tried, so a failing lookup is not retried
-- on every sweep.
INSERT INTO push_report_progress (report_id, area_lookup_at)
VALUES (sqlc.arg(report_id)::bigint, sqlc.arg(now)::timestamptz)
ON CONFLICT (report_id) DO UPDATE
SET area_lookup_at = EXCLUDED.area_lookup_at;

-- name: GetPushReportProgress :one
-- Where a report stands in the fan out. No row means it was never looked at.
SELECT report_id, critical_state, critical_at, reliable_state, reliable_at,
       area_lookup_at, evaluated_at
FROM push_report_progress
WHERE report_id = sqlc.arg(report_id)::bigint;

-- name: MarkCriticalFanout :exec
-- Records the Critical fan out as fanned or capped. Written only while the
-- stored state is NULL, so a capped report stays capped and a fanned report is
-- never overwritten (the cap is sticky).
INSERT INTO push_report_progress (report_id, critical_state, critical_at)
VALUES (sqlc.arg(report_id)::bigint, sqlc.arg(state)::text, sqlc.arg(now)::timestamptz)
ON CONFLICT (report_id) DO UPDATE
SET critical_state = EXCLUDED.critical_state,
    critical_at = EXCLUDED.critical_at
WHERE push_report_progress.critical_state IS NULL;

-- name: MarkReliableFanout :exec
-- The same for the Reliable reporter fan out.
INSERT INTO push_report_progress (report_id, reliable_state, reliable_at)
VALUES (sqlc.arg(report_id)::bigint, sqlc.arg(state)::text, sqlc.arg(now)::timestamptz)
ON CONFLICT (report_id) DO UPDATE
SET reliable_state = EXCLUDED.reliable_state,
    reliable_at = EXCLUDED.reliable_at
WHERE push_report_progress.reliable_state IS NULL;

-- name: MarkReportEvaluated :exec
-- Records that the sweep looked at the report.
INSERT INTO push_report_progress (report_id, evaluated_at)
VALUES (sqlc.arg(report_id)::bigint, sqlc.arg(now)::timestamptz)
ON CONFLICT (report_id) DO UPDATE
SET evaluated_at = EXCLUDED.evaluated_at;

-- name: CountReporterFanouts :one
-- The per reporter fan out count (PushCriticalPerReporterPerHour and the
-- Reliable equivalent) without storing who received what: how many reports
-- posted from a session bound to the reporter were fanned out under kind
-- ('critical' or 'reliable') at or after since. The report being evaluated is
-- left out, so a re run never counts itself (review note N2). For kind
-- 'reliable' a report whose Critical fan out was fanned is not counted: it
-- already holds its one slot, so a report counts once across both kinds (N4).
SELECT COUNT(*)::bigint
FROM push_report_progress p
JOIN reports r ON r.id = p.report_id
JOIN sessions ss ON ss.session_id = r.session_id
WHERE ss.account_id = sqlc.arg(reporter_account_id)::bigint
  AND p.report_id <> sqlc.arg(exclude_report_id)::bigint
  AND (
      (sqlc.arg(kind)::text = 'critical'
        AND p.critical_state = 'fanned'
        AND p.critical_at >= sqlc.arg(since)::timestamptz)
      OR
      (sqlc.arg(kind)::text = 'reliable'
        AND p.reliable_state = 'fanned'
        AND p.reliable_at >= sqlc.arg(since)::timestamptz
        AND p.critical_state IS DISTINCT FROM 'fanned')
  );

-- name: EnqueueInitialPush :one
-- Queues the first alert of an account for a report. One initial alert per
-- account and report whatever the kind: the dispatcher enqueues critical before
-- reliable, so critical wins (UI-SPEC 3c). A repeat returns no row.
INSERT INTO push_outbox (account_id, report_id, kind, expires_at, created_at, next_attempt_at)
VALUES (
    sqlc.arg(account_id)::bigint,
    sqlc.arg(report_id)::bigint,
    sqlc.arg(kind)::text,
    sqlc.arg(expires_at)::timestamptz,
    sqlc.arg(now)::timestamptz,
    sqlc.arg(now)::timestamptz
)
ON CONFLICT (account_id, report_id) WHERE kind IN ('critical', 'reliable') DO NOTHING
RETURNING id;

-- name: EnqueueReliableFromUpdates :execrows
-- One 3.2 row, two channels: for every reliable_nearby updates row of the
-- report whose account has a live subscription, queue a reliable push. The 3.2
-- rule wrote the row; push only follows it. A critical row already queued for
-- the pair wins through the shared unique index.
INSERT INTO push_outbox (account_id, report_id, kind, expires_at, created_at, next_attempt_at)
SELECT u.account_id, u.report_id, 'reliable',
       sqlc.arg(expires_at)::timestamptz,
       sqlc.arg(now)::timestamptz,
       sqlc.arg(now)::timestamptz
FROM updates u
JOIN accounts a ON a.id = u.account_id
WHERE u.report_id = sqlc.arg(report_id)::bigint
  AND u.kind = 'reliable_nearby'
  AND a.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM push_subscriptions p
      JOIN sessions ps ON ps.session_id = p.session_id
      WHERE p.account_id = u.account_id
        AND ps.revoked_at IS NULL
  )
ON CONFLICT (account_id, report_id) WHERE kind IN ('critical', 'reliable') DO NOTHING;

-- name: EnqueueChangedFromUpdates :execrows
-- Queues a changed push for each of the account's 3.2 event rows after since:
-- reached_live, hidden, resolved and fading. Rows of kind more and
-- reliable_nearby are never pushed as changes. A reached_live row is skipped
-- when the account already has a critical or reliable row for that report: the
-- in place confirmed update carries that news, so the person never gets both
-- (UI-SPEC 3c). expires_at comes from the report. Once per update row.
INSERT INTO push_outbox (account_id, report_id, kind, update_id, expires_at, created_at, next_attempt_at)
SELECT u.account_id, u.report_id, 'changed', u.id, r.expires_at,
       sqlc.arg(now)::timestamptz,
       sqlc.arg(now)::timestamptz
FROM updates u
JOIN reports r ON r.id = u.report_id
WHERE u.account_id = sqlc.arg(account_id)::bigint
  AND u.kind IN ('reached_live', 'hidden', 'resolved', 'fading')
  AND u.report_id IS NOT NULL
  AND u.created_at > sqlc.arg(since)::timestamptz
  AND NOT (
      u.kind = 'reached_live'
      AND EXISTS (
          SELECT 1
          FROM push_outbox o
          WHERE o.account_id = u.account_id
            AND o.report_id = u.report_id
            AND o.kind IN ('critical', 'reliable')
      )
  )
ON CONFLICT (account_id, update_id) WHERE kind = 'changed' DO NOTHING;

-- name: EnqueueConfirmedUpdates :execrows
-- The in place confirmed update: for each critical or reliable row of the
-- report that was sent with the words Not confirmed yet, queue one
-- confirmed_update row. A sent row whose words did not say that, and a row
-- still queued, get none. Once per account and report.
INSERT INTO push_outbox (account_id, report_id, kind, expires_at, created_at, next_attempt_at)
SELECT o.account_id, o.report_id, 'confirmed_update',
       sqlc.arg(expires_at)::timestamptz,
       sqlc.arg(now)::timestamptz,
       sqlc.arg(now)::timestamptz
FROM push_outbox o
WHERE o.report_id = sqlc.arg(report_id)::bigint
  AND o.kind IN ('critical', 'reliable')
  AND o.state = 'sent'
  AND o.sent_unconfirmed
ON CONFLICT (account_id, report_id) WHERE kind = 'confirmed_update' DO NOTHING;

-- name: ClaimDuePushes :many
-- Hands due rows to one claimant each. The claimed set is fixed once in a
-- MATERIALIZED CTE and then updated, like ClaimDueModerationChecks, because the
-- planner may rescan an IN subquery that carries LIMIT and SKIP LOCKED. SKIP
-- LOCKED lets two workers claim different rows; the lease makes a crashed
-- claimant's row claimable again once it lapses; attempts counts every claim.
-- Order: critical first, then confirmed_update, changed, reliable, then id.
-- The row carries the lease this claim holds (lease_until), which every later
-- write must present. update_kind is the kind of the 3.2 row a changed push
-- carries (NULL once 3.2 pruned it). first_kind is, for a confirmed_update row
-- only, the kind of the same account's critical or reliable row for the same
-- report (at most one, by uq_push_outbox_first; NULL for every other kind and
-- once the first row was pruned); the worker uses it as the base kind of a
-- confirmed update. The joins run after the update because an UPDATE cannot
-- join to the table it updates in its own FROM clause.
WITH due AS MATERIALIZED (
    SELECT o.id
    FROM push_outbox o
    WHERE o.state = 'queued'
      AND o.next_attempt_at <= now()
      AND (o.lease_until IS NULL OR o.lease_until < now())
    ORDER BY CASE o.kind
                 WHEN 'critical' THEN 0
                 WHEN 'confirmed_update' THEN 1
                 WHEN 'changed' THEN 2
                 ELSE 3
             END,
             o.id
    LIMIT sqlc.arg(batch_limit)::int
    FOR UPDATE OF o SKIP LOCKED
), claimed AS (
    UPDATE push_outbox c
    SET lease_until = now() + sqlc.arg(lease_seconds)::int * interval '1 second',
        attempts = c.attempts + 1
    FROM due
    WHERE c.id = due.id
    RETURNING c.id, c.account_id, c.report_id, c.kind, c.update_id,
              c.lease_until, c.attempts, c.expires_at, c.created_at
)
SELECT cl.id, cl.account_id, cl.report_id, cl.kind, cl.update_id,
       u.kind AS update_kind,
       f.kind AS first_kind,
       COALESCE(cl.lease_until, now())::timestamptz AS lease_until,
       cl.attempts, cl.expires_at, cl.created_at
FROM claimed cl
LEFT JOIN updates u
       ON u.id = cl.update_id
      AND u.account_id = cl.account_id
LEFT JOIN push_outbox f
       ON cl.kind = 'confirmed_update'
      AND f.account_id = cl.account_id
      AND f.report_id = cl.report_id
      AND f.kind IN ('critical', 'reliable')
ORDER BY CASE cl.kind
             WHEN 'critical' THEN 0
             WHEN 'confirmed_update' THEN 1
             WHEN 'changed' THEN 2
             ELSE 3
         END,
         cl.id;

-- name: ExtendPushLease :one
-- The worker renews its lease just before it sends a row, so no second claimer
-- can take a row that is being sent (review W2, T-03.3-12). It renews only for
-- the current holder: no row comes back when another claim took the row after
-- its lease passed.
UPDATE push_outbox
SET lease_until = now() + sqlc.arg(lease_seconds)::int * interval '1 second'
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz
RETURNING COALESCE(lease_until, now())::timestamptz AS lease_until;

-- name: MarkPushSent :execrows
-- The push service accepted the send. sent_unconfirmed is true when the words
-- said Not confirmed yet. This mark and the three after it (MarkPushSkipped,
-- ReschedulePush, MarkPushFailed) are written only by the holder of the row's
-- current lease: each is conditional on state queued and on the exact lease the
-- worker holds, and reports how many rows it changed. Zero rows changed means
-- the lease was lost and the caller moves on. A row already sent can never be
-- skipped, rescheduled or failed afterwards.
UPDATE push_outbox
SET state = 'sent',
    sent_at = sqlc.arg(now)::timestamptz,
    sent_unconfirmed = sqlc.arg(sent_unconfirmed)::boolean,
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: MarkPushSkipped :execrows
-- The row will not be sent, for a reason from the closed list.
UPDATE push_outbox
SET state = 'skipped',
    skip_reason = sqlc.arg(skip_reason)::text,
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: ReschedulePush :execrows
-- A temporary failure: try again later.
UPDATE push_outbox
SET next_attempt_at = sqlc.arg(next_attempt_at)::timestamptz,
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: MarkPushFailed :execrows
-- A permanent failure: no retry.
UPDATE push_outbox
SET state = 'failed',
    lease_until = NULL
WHERE id = sqlc.arg(id)::bigint
  AND state = 'queued'
  AND lease_until = sqlc.arg(held_lease)::timestamptz;

-- name: CountAccountPushesInWindow :one
-- The per account hourly cap: sent rows of kinds critical, reliable and changed
-- after since. Confirmed updates do not count.
SELECT COUNT(*)::bigint
FROM push_outbox
WHERE account_id = sqlc.arg(account_id)::bigint
  AND state = 'sent'
  AND kind IN ('critical', 'reliable', 'changed')
  AND sent_at > sqlc.arg(since)::timestamptz;

-- name: SkipStalePushes :execrows
-- On wake, queued rows whose own life or whose report's life is over become
-- skipped with reason stale instead of flushing a backlog. A row a worker holds
-- (lease not yet past) is left to that worker, which checks the remaining life
-- itself: the same holder only rule as the marks (review W2).
UPDATE push_outbox o
SET state = 'skipped',
    skip_reason = 'stale',
    lease_until = NULL
WHERE o.state = 'queued'
  AND (o.lease_until IS NULL OR o.lease_until <= sqlc.arg(now)::timestamptz)
  AND (
      o.expires_at <= sqlc.arg(now)::timestamptz
      OR EXISTS (
          SELECT 1
          FROM reports r
          WHERE r.id = o.report_id
            AND r.expires_at <= sqlc.arg(now)::timestamptz
      )
  );

-- name: PrunePushOutbox :exec
-- Deletes finished rows (sent, skipped or failed) created at or before edge.
-- Queued rows are never pruned.
DELETE FROM push_outbox
WHERE created_at <= sqlc.arg(edge)::timestamptz
  AND state <> 'queued';

-- name: DeleteAccountPushOutbox :execrows
-- Delete my account (Option A): the push copy of the person's updates is a
-- personal row, like the updates themselves.
DELETE FROM push_outbox
WHERE account_id = sqlc.arg(account_id)::bigint;

-- name: PushAccountHome :one
-- The worker reads this at send time, so a switch turned off or a mute made
-- after a row was queued stops it, confirmed updates included (review W5,
-- T-03.3-09): the home cell, the distance and the three switches of the
-- account, and whether the account muted this report. Defaults of an account
-- with no settings row: 5 km, Critical on, Changes off, Reliable off.
SELECT a.home_area_cell,
       COALESCE(s.alert_radius_km, 5)::smallint AS alert_radius_km,
       COALESCE(s.critical_push_on, true)::bool AS critical_push_on,
       COALESCE(s.changes_push_on, false)::bool AS changes_push_on,
       COALESCE(s.reliable_alerts_on, false)::bool AS reliable_alerts_on,
       EXISTS (
           SELECT 1
           FROM report_mutes m
           WHERE m.account_id = a.id
             AND m.report_id = sqlc.arg(report_id)::bigint
       )::bool AS muted
FROM accounts a
LEFT JOIN account_alert_settings s ON s.account_id = a.id
WHERE a.id = sqlc.arg(account_id)::bigint;
