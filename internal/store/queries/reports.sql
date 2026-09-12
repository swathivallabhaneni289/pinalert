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
