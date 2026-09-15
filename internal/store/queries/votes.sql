-- name: InsertVote :exec
-- A bare append with no conflict resolution and no row lock. created_at is
-- deliberately omitted so the column default supplies it. Concurrency
-- safety for TRUST-09 comes from Postgres handling concurrent plain inserts
-- into an unconstrained table natively — nothing here needs serialising.
INSERT INTO votes (report_id, account_id, kind, value, geohash_cell)
VALUES (sqlc.arg(report_id), sqlc.arg(account_id), sqlc.arg(kind), sqlc.arg(value), sqlc.arg(geohash_cell));

-- name: CurrentVotesForReports :many
-- DISTINCT ON (report_id, account_id, kind) latest-row-wins read over a
-- batch of report ids. This single batched read is what makes the
-- distinct-account half of TRUST-03's independence predicate automatic —
-- the result already holds at most one row per (report, account, kind), so
-- 02-03's independence counting only has to count distinct geohash_cell
-- values over it. It takes an array of report ids so the feed path in
-- 02-04 can load votes for a whole page of reports in one query rather
-- than N+1.
SELECT DISTINCT ON (report_id, account_id, kind)
    report_id, account_id, kind, value, geohash_cell, created_at
FROM votes
WHERE report_id = ANY(sqlc.arg(report_ids)::bigint[])
ORDER BY report_id, account_id, kind, created_at DESC, id DESC;

-- name: ReportVoteContext :one
-- The single read VotingService.CastVote makes before deciding anything
-- (02-03a). Four points a later reader would otherwise "correct":
--
-- (a) Why one query instead of two. CastVote needs the reporter identity,
-- the severity/category pair Resolve needs as ReportMeta, and expires_at on
-- every single call. Reading all four in one statement removes a window in
-- which a report could expire or be deleted between a separate reporter
-- lookup and a separate meta read. This query supersedes the narrower
-- ReporterAccountID query 02-02/02-PATTERNS.md/02-RESEARCH.md anticipated;
-- that query is deliberately never created.
--
-- (b) Why the join stops at sessions. sessions.account_id IS accounts.id —
-- a foreign key onto that column. A third join through accounts could only
-- re-confirm what the foreign key already guarantees, at the cost of an
-- extra join, so it is omitted on purpose.
--
-- (c) Why LEFT JOIN and not INNER JOIN. reports.session_id carries no
-- foreign key, and sessions.account_id is nullable — both deliberate, both
-- documented in migration 00002. A report submitted before Phase 1.1 made
-- login mandatory therefore has no account-bound session. An INNER JOIN
-- would return zero rows for such a report, indistinguishable from "no
-- such report," and the report would become unvotable. The LEFT JOIN
-- returns a NULL reporter_account_id instead: nobody matches it, the D-03
-- self-vote block correctly does not fire, and the report stays votable.
-- Zero rows then means exactly one thing — the report id does not exist.
--
-- (d) What this query is for. The reporter identity for D-03's self-vote
-- block, the severity/category pair for ReportMeta, and expires_at for the
-- expired-report rejection.
SELECT r.severity, r.category, r.expires_at, s.account_id AS reporter_account_id
FROM reports r
LEFT JOIN sessions s ON r.session_id = s.session_id
WHERE r.id = sqlc.arg(report_id);
