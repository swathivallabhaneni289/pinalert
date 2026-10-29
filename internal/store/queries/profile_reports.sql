-- name: ProfileReportCandidates :many
-- The candidates behind a person's profile list (plan 09-05, D-07, RESEARCH 4.3):
-- the reports filed from any session that belongs to the account, created after
-- since, newest first, at most row_limit. The columns are those of
-- ReportForFocus plus area_name (the stored coarse area for "Near {Area}"),
-- except distance_km: no distance is computed here, because a distance to a
-- finished report's exact point would leak it. The service measures the
-- distance itself, to the exact point of a live report and to the coarse point
-- of a finished one.
--
-- No moderation, vote or view predicate (T-02-04, D-03): whether a report is
-- listed, and as what, is decided in Go by classify, which fails closed for a
-- withheld report. The join and the window use idx_sessions_account_id and
-- idx_reports_session_expires, and LIMIT keeps a long history bounded.
SELECT r.id, r.category, r.severity, r.description, r.latitude, r.longitude, r.geohash,
       r.shelter_capacity_status, r.shelter_headcount, r.created_at, r.expires_at,
       r.area_name
FROM reports r
JOIN sessions s ON s.session_id = r.session_id
WHERE s.account_id = sqlc.arg(account_id)::bigint
  AND r.created_at > sqlc.arg(since)::timestamptz
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(row_limit)::int;
