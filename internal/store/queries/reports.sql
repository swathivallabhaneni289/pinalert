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
