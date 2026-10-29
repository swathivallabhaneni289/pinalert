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

-- name: ClaimAuthEmailSend :one
-- Phase 7.1 (UI-SPEC hand off note 9): one atomic claim for both the 45 second
-- cooldown and the hourly count of every auth email kind sent to one address
-- (sign up code, reset code, the account exists note, the no account note), so
-- the cap reveals nothing about which addresses have a password. The guard
-- lives on the DO UPDATE as in ClaimEmailCooldown: a refused claim writes
-- nothing and cannot extend the window that refused it, and surfaces as
-- pgx.ErrNoRows. The hourly window starts at the first send and restarts at 1
-- once it is older than window_cutoff.
INSERT INTO email_cooldowns (email, last_requested_at, window_started_at, window_sends)
VALUES (sqlc.arg(email), sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz, 1)
ON CONFLICT (email) DO UPDATE
SET last_requested_at = EXCLUDED.last_requested_at,
    window_started_at = CASE
        WHEN email_cooldowns.window_started_at IS NULL
          OR email_cooldowns.window_started_at <= sqlc.arg(window_cutoff)::timestamptz
            THEN EXCLUDED.last_requested_at
        ELSE email_cooldowns.window_started_at
    END,
    window_sends = CASE
        WHEN email_cooldowns.window_started_at IS NULL
          OR email_cooldowns.window_started_at <= sqlc.arg(window_cutoff)::timestamptz
            THEN 1
        ELSE email_cooldowns.window_sends + 1
    END
WHERE email_cooldowns.last_requested_at <= sqlc.arg(cooldown_cutoff)::timestamptz
  AND (email_cooldowns.window_started_at IS NULL
       OR email_cooldowns.window_started_at <= sqlc.arg(window_cutoff)::timestamptz
       OR email_cooldowns.window_sends < sqlc.arg(max_sends)::int)
RETURNING last_requested_at, window_sends;

-- name: GetAuthEmailSendWindow :one
-- Read only after a refused ClaimAuthEmailSend, to choose between the minute
-- sentence (still inside the cooldown) and the hour sentence (the hourly count
-- is spent).
SELECT last_requested_at, window_started_at, window_sends
FROM email_cooldowns
WHERE email = sqlc.arg(email);
