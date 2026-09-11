-- name: UpsertSession :exec
INSERT INTO sessions (session_id)
VALUES (sqlc.arg(session_id))
ON CONFLICT (session_id) DO NOTHING;

-- name: BindSessionAccount :exec
-- Must be an upsert, never a bare UPDATE: internal/session/cookie.go logs
-- and continues when its persist callback fails, so a browser can hold a
-- valid signed cookie with no sessions row at all (RESEARCH.md Pitfall 2). A
-- bare UPDATE would silently affect zero rows while the token was already
-- consumed — a dead end the visitor cannot recover from without requesting
-- a new link.
INSERT INTO sessions (session_id, account_id)
VALUES (sqlc.arg(session_id), sqlc.arg(account_id))
ON CONFLICT (session_id) DO UPDATE SET account_id = EXCLUDED.account_id;

-- name: GetAccountBySessionID :one
-- A session with a NULL account_id yields no row (pgx.ErrNoRows), which
-- callers must treat as "unverified", never as an error.
SELECT accounts.id, accounts.email
FROM sessions
JOIN accounts ON sessions.account_id = accounts.id
WHERE sessions.session_id = sqlc.arg(session_id);
