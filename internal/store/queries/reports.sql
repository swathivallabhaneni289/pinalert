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
