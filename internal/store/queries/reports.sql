-- name: InsertReport :one
INSERT INTO reports (
    session_id, category, severity, description, latitude, longitude, geohash,
    shelter_capacity_status, shelter_headcount, created_at, expires_at
) VALUES (
    sqlc.arg(session_id), sqlc.arg(category), sqlc.arg(severity), sqlc.arg(description),
    sqlc.arg(latitude), sqlc.arg(longitude), sqlc.arg(geohash),
    sqlc.arg(shelter_capacity_status), sqlc.arg(shelter_headcount),
    sqlc.arg(created_at), sqlc.arg(expires_at)
)
RETURNING id, category, severity, description, latitude, longitude, geohash,
          shelter_capacity_status, shelter_headcount, created_at, expires_at;

-- name: NearbyReports :many
SELECT id, category, severity, description, latitude, longitude, geohash,
       shelter_capacity_status, shelter_headcount, created_at, expires_at, distance_km
FROM (
    SELECT id, category, severity, description, latitude, longitude, geohash,
           shelter_capacity_status, shelter_headcount, created_at, expires_at,
           ( 6371 * acos(
               least(1.0,
                 cos(radians(sqlc.arg(lat)::float8)) * cos(radians(latitude))
                 * cos(radians(longitude) - radians(sqlc.arg(lon)::float8))
                 + sin(radians(sqlc.arg(lat)::float8)) * sin(radians(latitude))
               )
             )
           )::float8 AS distance_km
    FROM reports
    WHERE expires_at > now()
      AND latitude  BETWEEN sqlc.arg(lat_min)::float8 AND sqlc.arg(lat_max)::float8
      AND longitude BETWEEN sqlc.arg(lon_min)::float8 AND sqlc.arg(lon_max)::float8
) AS candidates
WHERE distance_km <= sqlc.arg(radius_km)::float8
ORDER BY distance_km ASC;

-- name: ReportsByAccount :many
-- Joins through the session's bound account rather than filtering on
-- r.session_id directly (01.1-07-PLAN.md's whole point): one account can
-- hold multiple verified sessions (multi-device, D-10), and this is the
-- query that must surface all of them together. A plain INNER JOIN is
-- equivalent here to a defensive LEFT JOIN — an orphaned report has no
-- session row and therefore no bound account, so the WHERE clause below
-- filters it out either way (DEC-M). No expiry predicate: a person's own
-- history does not vanish from their own profile when a report ages out of
-- the public feed. No session_id in the selected columns, continuing
-- T-01-02's control (TestNearbyReportsExcludesSessionID) — Phase 2/3's
-- independence predicate must count distinct account, with the session
-- identifier retained only as the write-path column (IDENT-05).
SELECT r.id, r.category, r.severity, r.description, r.latitude, r.longitude, r.geohash,
       r.shelter_capacity_status, r.shelter_headcount, r.created_at, r.expires_at
FROM reports r
JOIN sessions s ON r.session_id = s.session_id
WHERE s.account_id = sqlc.arg(account_id)
ORDER BY r.created_at DESC;

-- name: ReporterAccountsForReports :many
-- Batched sibling of ReportVoteContext's reporter half (internal/store/
-- queries/votes.sql), reused for a whole feed page instead of one report.
--
-- (a) Why it exists at all. service.BuildVoteTally(rows, reportID,
-- reporterAccountID) needs each report's reporter account to route the
-- reporter's own resolution vote into ReporterResolved and to exclude the
-- reporter's rows from the independent counts. Passing nil here on the read
-- path would mean ReporterResolved never fires on a read, and a report whose
-- reporter marked it resolved would keep appearing in the live feed — a
-- direct D-12/D-13 failure. The same value also decides the response's
-- is_own_report flag (D-03).
--
-- (b) Why it is batched rather than per report. The feed returns a whole
-- page. One call with the page's ids costs one round trip; a per-report
-- lookup would be an N+1 — the exact shape CurrentVotesForReports was
-- designed as a batch to avoid.
--
-- (c) Why it is a separate query instead of a join added to NearbyReports.
-- NearbyReports is pinned byte-for-byte by a drift const and its plan is
-- asserted by an EXPLAIN-based index test. Leaving it untouched keeps
-- FOUND-03's proven bounding-box/Haversine query exactly as Phase 1 shipped
-- it, and — the load-bearing part — makes it self-evident that the feed
-- query gained no visibility or vote predicate of its own
-- (.planning/research/ARCHITECTURE.md Anti-Pattern 1, threat T-02-04).
-- Filtering by visibility happens in Go, over Resolve's output, never in
-- SQL.
--
-- (d) Why LEFT JOIN and not INNER JOIN. reports.session_id carries no
-- foreign key and sessions.account_id is nullable — both deliberate, both
-- documented in migration 00002. A report submitted before Phase 1.1 made
-- login mandatory, and any report seeded by internal/testutil/seed.go
-- (which inserts a literal session_id with no matching sessions row),
-- therefore has no account-bound session. An INNER JOIN would drop those
-- reports from this result entirely, and the caller could not distinguish
-- "no reporter account" from "report not in the batch." With the LEFT JOIN
-- every requested-and-existing report comes back, carrying a NULL
-- reporter_account_id: nobody matches it, is_own_report is false for every
-- viewer, and the report stays in the feed exactly as it does today. Note
-- also that sessions.session_id is the table's PRIMARY KEY, so the join
-- matches at most one row and can never duplicate a report.
--
-- (e) Why the join stops at sessions. sessions.account_id IS accounts.id by
-- foreign key, so a third join through accounts could only re-confirm a row
-- the constraint already guarantees. Omitted on purpose, matching
-- ReportVoteContext's own reasoning.
SELECT r.id AS report_id, s.account_id AS reporter_account_id
FROM reports r
LEFT JOIN sessions s ON r.session_id = s.session_id
WHERE r.id = ANY(sqlc.arg(report_ids)::bigint[]);

-- name: InsertReportIdempotent :one
-- The retry safe sibling of InsertReport. One statement: a concurrent retry
-- with the same account and request id waits for the first insert to commit
-- and then gets no row back, which the caller turns into ReportByRequest. The
-- conflict target repeats the column pair and the partial predicate of
-- idx_reports_request_idempotency so Postgres infers that index. A report sent
-- without a request id (request_id NULL) never conflicts and is never
-- deduplicated. created_at and expires_at are server stamps passed in by the
-- caller; client_written_at is display only and is never read by any
-- predicate or ordering (R-05, D-19).
INSERT INTO reports (
    session_id, category, severity, description, latitude, longitude, geohash,
    shelter_capacity_status, shelter_headcount, created_at, expires_at,
    moderation_state, request_id, request_account_id, client_written_at
) VALUES (
    sqlc.arg(session_id), sqlc.arg(category), sqlc.arg(severity), sqlc.arg(description),
    sqlc.arg(latitude), sqlc.arg(longitude), sqlc.arg(geohash),
    sqlc.arg(shelter_capacity_status), sqlc.arg(shelter_headcount),
    sqlc.arg(created_at), sqlc.arg(expires_at),
    sqlc.arg(moderation_state), sqlc.narg(request_id), sqlc.narg(request_account_id),
    sqlc.narg(client_written_at)
)
ON CONFLICT (request_account_id, request_id) WHERE request_id IS NOT NULL DO NOTHING
RETURNING id, category, severity, description, latitude, longitude, geohash,
          shelter_capacity_status, shelter_headcount, created_at, expires_at, moderation_state;

-- name: ReportByRequest :one
-- The replay lookup after InsertReportIdempotent returned no row. Always
-- scoped to the caller's account together with the request id, never by the
-- request id alone, so one account can never read back another account's
-- report by guessing its request id (T-04-05). No row means no such request.
SELECT id, category, severity, description, latitude, longitude, geohash,
       shelter_capacity_status, shelter_headcount, created_at, expires_at, moderation_state
FROM reports
WHERE request_account_id = sqlc.arg(account_id)::bigint
  AND request_id = sqlc.arg(request_id)::text;

-- name: ReportExtrasForReports :many
-- The moderation state, the display only written time and the viewer's flag
-- for a whole feed page in one round trip. Batched like CurrentVotesForReports
-- and ReporterAccountsForReports, and called once per non empty page, so
-- NearbyReports stays exactly as Phase 1 pinned it and names none of these
-- columns. One row per existing requested report: the LEFT JOIN is on the
-- viewer's own flag, and report_flags_one_per_account allows at most one such
-- row, so a report is never duplicated. A dismissed flag still counts as
-- flagged because the same account can never flag that report again
-- (R-01, D-09).
SELECT r.id AS report_id, r.moderation_state, r.client_written_at,
       (f.id IS NOT NULL)::bool AS viewer_flagged
FROM reports r
LEFT JOIN report_flags f ON f.report_id = r.id AND f.account_id = sqlc.arg(viewer_account_id)::bigint
WHERE r.id = ANY(sqlc.arg(report_ids)::bigint[]);

-- name: ReportForModeration :one
-- One report in the moderator row shape (the same columns as ReviewQueue,
-- FlagQueue and PendingQueue), for the Find, Keep and Remove answers (D-08).
-- No session id, email or account id is selected. The check columns are NULL
-- for a report that never had a check (seeded and older rows). The text of the
-- report is the only free text here; the check row itself holds none.
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
WHERE r.id = sqlc.arg(report_id);
