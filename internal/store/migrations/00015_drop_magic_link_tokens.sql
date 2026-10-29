-- Phase 7.1 plan 16 (D-12): the password sign in of IDENT-06 replaced the
-- emailed sign in link, so the table that held the link tokens has no reader
-- or writer left. Dropping it also drops its index. Unused links lived at most
-- five minutes, so nothing of value is lost (T-07.1-22).
-- +goose Up
DROP TABLE magic_link_tokens;

-- +goose Down
-- Recreates the table and its index exactly as 00002 made them, so the drop is
-- reversible. Any rows that were in the table are not restored.
CREATE TABLE magic_link_tokens (
    id          BIGSERIAL PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,
    email       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);

CREATE INDEX idx_magic_link_tokens_email_created ON magic_link_tokens (email, created_at DESC);
