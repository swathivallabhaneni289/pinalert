-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
CREATE TABLE accounts (
    id          BIGSERIAL PRIMARY KEY,
    email       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE magic_link_tokens (
    id          BIGSERIAL PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,
    email       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);

-- Supports the cooldown lookup plan 01.1-05 (IDENT-04) adds: the most recent
-- token row for a given email, ordered by recency.
CREATE INDEX idx_magic_link_tokens_email_created ON magic_link_tokens (email, created_at DESC);

-- Nullable on purpose: internal/session/cookie.go deliberately log-and-
-- continues when persistSession fails, so a valid signed cookie can exist
-- with no sessions row at all, let alone an account_id. No FK is added from
-- reports.session_id to sessions.session_id for the same reason — see this
-- plan's threat T-01-56 / accepted risk AR-27.
ALTER TABLE sessions ADD COLUMN account_id BIGINT REFERENCES accounts(id);

CREATE INDEX idx_sessions_account_id ON sessions (account_id);

-- +goose Down
DROP INDEX idx_sessions_account_id;
ALTER TABLE sessions DROP COLUMN account_id;
DROP TABLE magic_link_tokens;
DROP TABLE accounts;
