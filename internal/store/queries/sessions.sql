-- name: UpsertSession :exec
INSERT INTO sessions (session_id)
VALUES (sqlc.arg(session_id))
ON CONFLICT (session_id) DO NOTHING;

-- name: BindSessionAccount :execrows
-- Must be an upsert, never a bare UPDATE: internal/session/cookie.go logs
-- and continues when its persist callback fails, so a browser can hold a
-- valid signed cookie with no sessions row at all (RESEARCH.md Pitfall 2). A
-- bare UPDATE would silently affect zero rows while the token was already
-- consumed: a dead end the visitor cannot recover from without requesting
-- a new link.
-- The upsert never moves a bound row to a different account (Phase 3.2,
-- Pattern 10): the WHERE lets it run only when the row is live (never revoked)
-- and is unbound or already bound to this same account. It never clears
-- revoked_at and never binds a revoked row (W1, 2026-10-19): reviving a revoked
-- id would bring back every copy of the old cookie, which Log out of all
-- devices exists to end. Zero rows affected means the row is bound to a
-- different account or is revoked, and the caller must use a fresh session id;
-- a sign in must never rewrite whose history a session carries.
INSERT INTO sessions (session_id, account_id)
VALUES (sqlc.arg(session_id), sqlc.arg(account_id))
ON CONFLICT (session_id) DO UPDATE
   SET account_id = EXCLUDED.account_id
 WHERE sessions.revoked_at IS NULL
   AND (sessions.account_id IS NULL OR sessions.account_id = EXCLUDED.account_id);

-- name: GetAccountBySessionID :one
-- A session with a NULL account_id yields no row (pgx.ErrNoRows), which
-- callers must treat as "unverified", never as an error. A revoked session
-- (Log out of all devices) and a session of a deleted account (Delete my
-- account) also yield no row and read as unverified: requireVerifiedAccount
-- and requireModerator both read this one query, so one filter covers both
-- gates. Neither filter touches sessions.account_id, so authorship joins are
-- unchanged.
-- Phase 7.1 adds four presence fields so the gate judges the account without a
-- second read. has_password (W1): the gate treats a session of an account with
-- no password as signed out, so a session the old binary made after the
-- migration still has to go through the typed code. has_username, has_name and
-- has_home_area say whether the three required profile steps are done.
SELECT accounts.id, accounts.email, accounts.terms_version, accounts.first_run_done_at,
       (accounts.password_hash IS NOT NULL)::bool AS has_password,
       (accounts.username IS NOT NULL)::bool AS has_username,
       (accounts.display_name IS NOT NULL)::bool AS has_name,
       (accounts.home_area_name IS NOT NULL)::bool AS has_home_area
FROM sessions
JOIN accounts ON sessions.account_id = accounts.id
WHERE sessions.session_id = sqlc.arg(session_id)
  AND sessions.revoked_at IS NULL AND accounts.deleted_at IS NULL;

-- name: GetSessionBinding :one
-- The sign in path's raw read of a session row. It does NOT filter on
-- revoked_at or deleted_at, so a revoked session or a deleted account still
-- shows the account the row is bound to. No row means the session is not
-- stored yet; a NULL account_id means it is stored but unbound. The sign in
-- path reads this before it consumes the link, to decide whether to bind this
-- session id or draw a fresh one. Phase 7.1: the password sign in (plan 05)
-- reads it to clean the push subscriptions of a replaced session, so it
-- outlives the link flow.
SELECT s.account_id,
       (s.revoked_at IS NOT NULL)::bool AS revoked,
       COALESCE(a.deleted_at IS NOT NULL, false)::bool AS account_deleted,
       a.email AS account_email
FROM sessions s
LEFT JOIN accounts a ON a.id = s.account_id
WHERE s.session_id = sqlc.arg(session_id);

-- name: RevokeAccountSessions :execrows
-- Log out of all devices and Delete my account: marks every live session of
-- one account revoked in one statement, so a stolen or copied cookie stops
-- working at the gate. It never clears sessions.account_id: the binding is how
-- posts reach their author, and ending a session must not move authorship.
UPDATE sessions
SET revoked_at = sqlc.arg(now)
WHERE account_id = sqlc.arg(account_id)
  AND revoked_at IS NULL;

-- name: SessionEndedByPasswordSwitch :one
-- UI-SPEC Surface 6 route 2: true when this session is bound to a not deleted
-- account that has no password yet, whether or not the session is revoked (the
-- migration revoked the old ones; a session made after it is refused by the
-- gate all the same, W1). It tells a browser only about its own ended session.
SELECT EXISTS (
    SELECT 1
    FROM sessions s
    JOIN accounts a ON a.id = s.account_id
    WHERE s.session_id = sqlc.arg(session_id)
      AND a.password_hash IS NULL
      AND a.deleted_at IS NULL
)::bool AS ended;

-- name: RevokeSession :execrows
-- When a browser bound to one account signs in as another, its old session is
-- revoked (W8), never unbound, so authorship joins stay. It never clears
-- sessions.account_id and changes nothing on a session already revoked.
UPDATE sessions
SET revoked_at = sqlc.arg(now)::timestamptz
WHERE session_id = sqlc.arg(session_id)
  AND revoked_at IS NULL;
