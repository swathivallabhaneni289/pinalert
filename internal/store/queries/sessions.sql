-- name: UpsertSession :exec
INSERT INTO sessions (session_id)
VALUES (sqlc.arg(session_id))
ON CONFLICT (session_id) DO NOTHING;
