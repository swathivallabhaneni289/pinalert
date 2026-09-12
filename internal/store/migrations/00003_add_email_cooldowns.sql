-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
-- The primary key on email is not decoration — it is the unique index
-- Postgres blocks on to serialise concurrent claims for the same address
-- (DEC-W, T-01-93), which is what makes ClaimEmailCooldown's single
-- INSERT ... ON CONFLICT statement atomic with no advisory lock or explicit
-- transaction needed.
CREATE TABLE email_cooldowns (
    email TEXT PRIMARY KEY,
    last_requested_at TIMESTAMPTZ NOT NULL
);

-- 00002's idx_magic_link_tokens_email_created index is deliberately left in
-- place even though the recency lookup it was added for (the most recent
-- token requested for an email, ordered by recency) no longer runs — this
-- table's own primary key now serves that purpose. Dropping the old index
-- here would be scope creep against a gap-closure plan.

-- +goose Down
DROP TABLE email_cooldowns;
