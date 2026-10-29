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

-- name: FollowedLiveCandidates :many
-- The candidates behind the follow note (plan 09-22, D-09): the reports filed
-- from any session of the given accounts that have not expired at now and were
-- created after since, newest first, at most row_limit. The columns are those
-- of ProfileReportCandidates plus the reporter's account id. since is the
-- earliest Notify me time of the people asked about; the service keeps a report
-- only when it is newer than its own reporter's time, and decides in Go,
-- through classify, whether it is listable, so there is no moderation, vote or
-- view predicate here. now is a parameter, never the database clock, so the
-- service's one instant is the one that judges expiry.
SELECT r.id, r.category, r.severity, r.description, r.latitude, r.longitude, r.geohash,
       r.shelter_capacity_status, r.shelter_headcount, r.created_at, r.expires_at,
       r.area_name, s.account_id::bigint AS reporter_account_id
FROM reports r
JOIN sessions s ON s.session_id = r.session_id
WHERE s.account_id = ANY(sqlc.arg(account_ids)::bigint[])
  AND r.expires_at > sqlc.arg(now)::timestamptz
  AND r.created_at > sqlc.arg(since)::timestamptz
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: ActiveFollowsAmong :many
-- Which of these people the follower still follows (active rows only, D-21).
-- The bell reads it once per list to decide whether an old follow note may
-- still name its person (plan 09-22). It does not look at the Notify me flag:
-- a follow that is still on stays readable after the bell is switched off.
SELECT followee_id
FROM account_follows
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND followee_id = ANY(sqlc.arg(followee_ids)::bigint[])
  AND ended_at IS NULL;
