-- +goose Up
-- Phase 8 plan 02 (COORD-08, D-05, D-06): the share token. Share links name a
-- report by a random token and never by its numeric id, so nobody without an
-- account can walk through reports one id at a time (research Pitfall 1).
--
-- The token is a random capability, stored plain on purpose. It is a link that
-- anyone may forward, not a credential, so hashing it would buy nothing: the
-- public page has to look the report up by the value in the link. It is 22
-- characters of unpadded base64url (16 bytes from crypto/rand), which the shape
-- check below enforces so a hand written value cannot be short or guessable.
--
-- A NULL token means the report was never shared. The token is made lazily, on
-- the first Share tap of a signed in person, and never changes after that. Setting
-- it back to NULL would revoke every link already forwarded, which no code does
-- today but the column allows. Partial unique index: many NULLs coexist, two
-- reports can never hold the same token. No ON DELETE and no CONCURRENTLY
-- (repo convention; goose runs this in a transaction).
ALTER TABLE reports
    ADD COLUMN share_token TEXT;

ALTER TABLE reports
    ADD CONSTRAINT reports_share_token_shape
    CHECK (share_token IS NULL OR share_token ~ '^[A-Za-z0-9_-]{22}$');

CREATE UNIQUE INDEX uq_reports_share_token
    ON reports (share_token)
    WHERE share_token IS NOT NULL;

-- +goose Down
-- Every forwarded link stops working: the tokens are lost with the column.
DROP INDEX uq_reports_share_token;
ALTER TABLE reports
    DROP CONSTRAINT reports_share_token_shape,
    DROP COLUMN share_token;
