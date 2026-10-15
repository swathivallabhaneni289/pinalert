-- name: ExpiredReportsInWindow :many
-- The report history reads behind the Past tab and a report link
-- (03.1-04-PLAN.md). Three statements share this block. Six choices a later
-- reader would otherwise "correct":
--
-- (a) What each read feeds. ExpiredReportsInWindow feeds both Past groups
-- (a report that was closed and then expired stays Resolved, D-03).
-- OpenReportsInArea feeds only the Resolved group: a report closed by its
-- reporter or by independent resolve votes that has not yet expired.
-- ReportForFocus feeds a /?report=ID link, which must open a report outside
-- the reader's radius and outside the 7 day window (D-11).
--
-- (b) Why two candidate sets with separate caps. One read ordered by expiry
-- would let a disaster's many live reports fill the cap and empty the Past
-- tab exactly when it is needed. The expired set (cap 500) and the open set
-- (cap 1000) are read apart, so live reports can never crowd expired ones
-- out (RESEARCH Pitfall 2).
--
-- (c) Every bound is a parameter and none of the three statements holds a
-- database clock call. The caller passes the instant it judges against and
-- the window start, so an injected clock reaches the queries (backlog 999.5).
-- NearbyReports is left alone on purpose: it filters on the database clock
-- and on unexpired rows, so it cannot serve Past (D-28).
--
-- (d) No vote, visibility or votes table predicate. Whether a report is
-- Resolved, Expired or still open is decided in Go (T-02-04). The prefilter
-- is a safe superset: CastVote refuses every vote on an expired report, so a
-- closure always happens before expiry and an unexpired row is the only place
-- a closure can be found.
--
-- (e) No session id, voter cell or account in any output (T-01-02). The
-- twelve columns are the ones NearbyReports returns, in the same order, and
-- distance_km is measured from the lat and lon parameters.
--
-- (f) Indexes the predicates can use: idx_reports_expires_at for the expiry
-- bounds and idx_reports_lat_lon for the bounding box. No new index and no
-- migration is added (RESEARCH A8).
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
    WHERE expires_at <= sqlc.arg(now)::timestamptz
      AND expires_at > sqlc.arg(window_start)::timestamptz
      AND latitude  BETWEEN sqlc.arg(lat_min)::float8 AND sqlc.arg(lat_max)::float8
      AND longitude BETWEEN sqlc.arg(lon_min)::float8 AND sqlc.arg(lon_max)::float8
) AS candidates
WHERE distance_km <= sqlc.arg(radius_km)::float8
ORDER BY expires_at DESC, id DESC
LIMIT sqlc.arg(row_cap)::int;

-- name: OpenReportsInArea :many
-- Unexpired reports inside the reader's area: the candidates that may have
-- been closed. See the comment block on ExpiredReportsInWindow, points (a)
-- to (f). The expiry edge is strict (expires_at after the instant), the
-- mirror of the expired set's inclusive upper edge, so a report is in exactly
-- one of the two sets. Nearest first, so a capped read keeps the reports
-- closest to the reader.
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
    WHERE expires_at > sqlc.arg(now)::timestamptz
      AND latitude  BETWEEN sqlc.arg(lat_min)::float8 AND sqlc.arg(lat_max)::float8
      AND longitude BETWEEN sqlc.arg(lon_min)::float8 AND sqlc.arg(lon_max)::float8
) AS candidates
WHERE distance_km <= sqlc.arg(radius_km)::float8
ORDER BY distance_km ASC, id ASC
LIMIT sqlc.arg(row_cap)::int;

-- name: ReportForFocus :one
-- One report by id for a /?report=ID link. See the comment block on
-- ExpiredReportsInWindow, points (a) to (f). There is deliberately no expiry,
-- area or radius predicate: a link must open a report outside the reader's
-- radius and outside the 7 day window (D-11). The service decides "too old"
-- and answers one way for a report that is missing or not shown, so moderation
-- state is never revealed (T-31-19). distance_km is measured from the lat and
-- lon parameters.
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
WHERE id = sqlc.arg(report_id)::bigint;
