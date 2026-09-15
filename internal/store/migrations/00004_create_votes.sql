-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
-- votes is an append-only log: every cast, including a changed vote per
-- D-02, is a new row. It deliberately carries no unique key over
-- (report_id, account_id, kind) — the "current" vote is a derived read, not
-- a stored mutable field, so there is no contended row for concurrent
-- writers to race on (T-02-02, TRUST-09).
--
-- 00003's single-atomic-claim upsert shape (email_cooldowns, PRIMARY KEY on
-- email, INSERT ... ON CONFLICT DO UPDATE) is deliberately NOT reused here.
-- email_cooldowns has one mutable row per email that many requests
-- read-then-write; a vote write is a pure append. Reusing that shape would
-- additionally destroy the vote-change history Phase 3's confidence/
-- reliability decay scoring needs.
--
-- geohash_cell holds the VOTER's own location at a coarser precision than
-- reports.geohash — it is written by 02-03's service layer
-- (voterGeohashPrecision = 7), never by a client, and it is service-internal
-- data that must never be serialised into any client response.
--
-- kind and value are plain TEXT with no CHECK constraint on purpose: the
-- closed enums (kind in {content, resolution}; value in {confirm, dispute}
-- for content and {resolve, reopen} for resolution) are validated
-- server-side in 02-03 following this repo's existing Category.Valid() /
-- Severity.Valid() convention, so the vote vocabulary can extend in Phase 3
-- without a schema migration.
CREATE TABLE votes (
    id            BIGSERIAL PRIMARY KEY,
    report_id     BIGINT NOT NULL REFERENCES reports(id),
    account_id    BIGINT NOT NULL REFERENCES accounts(id),
    kind          TEXT NOT NULL,
    value         TEXT NOT NULL,
    geohash_cell  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Serves CurrentVotesForReports' DISTINCT ON + ORDER BY exactly: the column
-- order and the two DESC directions mirror that query, not stylistic choices.
CREATE INDEX idx_votes_report_account_kind_created
    ON votes (report_id, account_id, kind, created_at DESC, id DESC);

-- +goose Down
DROP TABLE votes;
