-- name: InsertMagicLinkToken :one
INSERT INTO magic_link_tokens (
    token_hash, email, expires_at
) VALUES (
    sqlc.arg(token_hash), sqlc.arg(email), sqlc.arg(expires_at)
)
RETURNING id, created_at, expires_at;
