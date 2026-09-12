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
