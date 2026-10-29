-- +goose Up
-- Phase 7.1: password sign in and usernames. One migration so the generated
-- models change once. It serves D-02 (a slow hash for passwords, codes stored
-- only as a keyed hash, single use, limited tries), D-05 (the address stays
-- server side, never in a URL), D-07 (the username), D-08 and D-14 (the one
-- time fill in rides a real sign in through a typed code) and IDENT-06 and
-- IDENT-07.

-- The password. Only an argon2id PHC string is ever stored, never a password.
ALTER TABLE accounts
    ADD COLUMN password_hash TEXT;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_password_hash_shape
    CHECK (password_hash IS NULL OR password_hash LIKE '$argon2id$%');

-- The username. The stored form is already the canonical lowercase ASCII form,
-- so a plain UNIQUE is case insensitive by construction and needs no extension
-- type and no lower() index. NULLs do not collide, so accounts that have not
-- reached step 4 yet are fine. 3 to 20 characters of a to z, 0 to 9, underscore
-- and full stop, at least one letter, no full stop at either end and no two in
-- a row.
ALTER TABLE accounts
    ADD COLUMN username TEXT;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_username_shape
    CHECK (username IS NULL OR (
        username ~ '^[a-z0-9_.]{3,20}$'
        AND username ~ '[a-z]'
        AND username !~ '^[.]'
        AND username !~ '[.]$'
        AND username !~ '[.][.]'));

ALTER TABLE accounts
    ADD CONSTRAINT accounts_username_unique UNIQUE (username);

-- Pending means the text screening could not judge the name (D-16). Phase 8
-- displays only clear names and re-checks pending ones.
ALTER TABLE accounts
    ADD COLUMN username_screen TEXT;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_username_screen_valid
    CHECK ((username IS NULL AND username_screen IS NULL)
        OR (username IS NOT NULL AND username_screen IS NOT NULL
            AND username_screen IN ('clear', 'pending')));

-- NULL until the first change after the first set; the rename gap counts from
-- it.
ALTER TABLE accounts
    ADD COLUMN username_changed_at TIMESTAMPTZ;

-- One row per request for a typed code. Every request kind (sign up for a new,
-- legacy or existing address; reset for a known or unknown address) writes one
-- row, so the work, a later resend and a reload of the code screen look the same
-- for every branch. delivery says which email the row stands for; only a code
-- row can ever be confirmed.
-- session_id deliberately has no foreign key: the cookie layer persists the
-- sessions row best effort (internal/session/cookie.go logs and continues), so
-- a code request can arrive before that row exists. A code can be redeemed only
-- from the session that asked for it.
-- code_hash is an HMAC-SHA256 under a server key, never the code itself.
-- password_hash is the password typed at Create account, already hashed, applied
-- only when the code is confirmed.
-- used_at is set when the row is consumed or replaced by a newer code.
CREATE TABLE auth_codes (
    id            BIGSERIAL PRIMARY KEY,
    session_id    TEXT NOT NULL,
    purpose       TEXT NOT NULL,
    delivery      TEXT NOT NULL,
    email         TEXT NOT NULL,
    code_hash     TEXT NOT NULL,
    password_hash TEXT,
    attempts      INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    used_at       TIMESTAMPTZ,
    CONSTRAINT auth_codes_purpose_valid CHECK (purpose IN ('signup', 'reset')),
    CONSTRAINT auth_codes_delivery_valid CHECK (
        delivery IN ('code', 'account_exists', 'no_account')),
    CONSTRAINT auth_codes_code_hash_shape CHECK (code_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT auth_codes_password_hash_shape CHECK (
        password_hash IS NULL
        OR (purpose = 'signup' AND password_hash LIKE '$argon2id$%')),
    CONSTRAINT auth_codes_attempts_valid CHECK (attempts >= 0)
);

CREATE INDEX idx_auth_codes_session_live
    ON auth_codes (session_id, purpose)
    WHERE used_at IS NULL;

CREATE INDEX idx_auth_codes_email ON auth_codes (email);

-- The per address sign in throttle, kept in the database so it survives a free
-- host sleep. key_hash is an HMAC of the normalised address, so a probed
-- unknown address behaves the same as a known one and is never stored in the
-- clear.
CREATE TABLE auth_throttles (
    scope           TEXT NOT NULL,
    key_hash        TEXT NOT NULL,
    failures        INTEGER NOT NULL,
    last_failure_at TIMESTAMPTZ NOT NULL,
    locked_until    TIMESTAMPTZ,
    PRIMARY KEY (scope, key_hash),
    CONSTRAINT auth_throttles_scope_valid CHECK (scope IN ('sign_in')),
    CONSTRAINT auth_throttles_key_hash_shape CHECK (key_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT auth_throttles_failures_valid CHECK (failures >= 0)
);

-- The hourly count of every auth email to one address, all four kinds, so the
-- cap never depends on which addresses already have a password (UI-SPEC hand
-- off note 9).
ALTER TABLE email_cooldowns
    ADD COLUMN window_started_at TIMESTAMPTZ,
    ADD COLUMN window_sends      INTEGER NOT NULL DEFAULT 0;

ALTER TABLE email_cooldowns
    ADD CONSTRAINT email_cooldowns_window_sends_valid
    CHECK (window_sends >= 0);

-- D-14 Option X (research section 6): no password exists yet, so a live cookie
-- must not be able to set one without a typed code. At migration time every
-- account has no password, so this ends every live session. The push send path
-- already ignores revoked sessions, and the step 5 Alerts screen at the end of
-- the fill in registers a device again for the account that turned it on.
UPDATE sessions SET revoked_at = now()
WHERE revoked_at IS NULL
  AND account_id IN (SELECT id FROM accounts WHERE password_hash IS NULL);

-- +goose Down
-- The session revoke in Up is a data step and cannot be undone; it is not
-- attempted here. Those people sign in again.
ALTER TABLE email_cooldowns
    DROP CONSTRAINT email_cooldowns_window_sends_valid;
ALTER TABLE email_cooldowns
    DROP COLUMN window_sends,
    DROP COLUMN window_started_at;
DROP TABLE auth_throttles;
DROP TABLE auth_codes;
ALTER TABLE accounts
    DROP CONSTRAINT accounts_username_screen_valid,
    DROP CONSTRAINT accounts_username_unique,
    DROP CONSTRAINT accounts_username_shape,
    DROP CONSTRAINT accounts_password_hash_shape;
ALTER TABLE accounts
    DROP COLUMN username_changed_at,
    DROP COLUMN username_screen,
    DROP COLUMN username,
    DROP COLUMN password_hash;
