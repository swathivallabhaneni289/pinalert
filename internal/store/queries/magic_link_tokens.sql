-- name: InsertMagicLinkToken :one
INSERT INTO magic_link_tokens (
    token_hash, email, expires_at
) VALUES (
    sqlc.arg(token_hash), sqlc.arg(email), sqlc.arg(expires_at)
)
RETURNING id, created_at, expires_at;

-- name: GetMagicLinkTokenByHash :one
-- Used only to choose outcome copy (already-used vs. expired vs. unknown);
-- authorisation rests entirely on ConsumeToken's atomic conditional UPDATE
-- below, never on this read.
SELECT email, created_at, expires_at, used_at
FROM magic_link_tokens
WHERE token_hash = sqlc.arg(token_hash);

-- name: ConsumeToken :one
-- A single atomic conditional UPDATE, never read-then-write: two
-- near-simultaneous clicks (a real user plus a mail-scanner prefetch) would
-- both pass a separate read before either wrote (RESEARCH.md "Don't
-- Hand-Roll" TOCTOU row). sqlc's :one returns pgx.ErrNoRows when zero rows
-- match (already-used, expired, or unknown token) — the caller distinguishes
-- which by the preceding GetMagicLinkTokenByHash read, not by parsing this
-- error.
UPDATE magic_link_tokens
SET used_at = now()
WHERE token_hash = sqlc.arg(token_hash)
  AND used_at IS NULL
  AND expires_at > now()
RETURNING email;

-- name: LatestTokenForEmail :one
-- Plan 01.1-05's resend-cooldown read (IDENT-04, D-03): the most recent
-- token requested for this address, whatever its used/expired state — the
-- cooldown clock runs from when a token was last REQUESTED, not from
-- whether it was ever consumed. Served by migration 00002's
-- (email, created_at DESC) index. sqlc's :one returns pgx.ErrNoRows when no
-- prior request exists for the address — the caller treats that as "no
-- cooldown, proceed", never as an error.
SELECT created_at
FROM magic_link_tokens
WHERE email = sqlc.arg(email)
ORDER BY created_at DESC
LIMIT 1;
