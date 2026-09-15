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
