-- name: ClaimEmailCooldown :one
-- The per-address resend cooldown (IDENT-04, D-03/D-04). A single atomic
-- claim, never a check followed by a separate insert — the second shape
-- lets concurrent requests for one address all pass the check before any of
-- them writes (T-01-93). The guard lives on the DO UPDATE, not the INSERT:
-- when it is false, Postgres updates nothing and returns nothing, so a
-- refused claim performs no write at all and cannot extend the window that
-- refused it (DEC-J, DEC-X). sqlc's :one surfaces that as pgx.ErrNoRows —
-- the same "no rows means refused" contract ConsumeToken already
-- established in this codebase.
INSERT INTO email_cooldowns (email, last_requested_at)
VALUES (sqlc.arg(email), sqlc.arg(requested_at))
ON CONFLICT (email) DO UPDATE
SET last_requested_at = EXCLUDED.last_requested_at
WHERE email_cooldowns.last_requested_at <= sqlc.arg(cooldown_cutoff)
RETURNING last_requested_at;
