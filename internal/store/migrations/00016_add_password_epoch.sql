-- +goose Up
-- Phase 7.1 review CR-S02: the password epoch. accounts.password_epoch moves by
-- one every time a password is set by a typed code (a reset, and Create account
-- on an account that already existed); a rehash after a cost change never moves
-- it. sessions.password_epoch is the account's epoch at the moment the session
-- was bound. The sign in path binds a session only while the account's epoch
-- still equals the one it read before checking the password, and the access
-- gate lets a session in only while the two are equal, so a session opened with
-- a password that has since been replaced is never signed in, even when its
-- bind raced the reset.
-- Both columns start at 0, so every session of an account that already has a
-- password stays signed in after this migration (only 00014 signed people out).
ALTER TABLE accounts
    ADD COLUMN password_epoch BIGINT NOT NULL DEFAULT 0;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_password_epoch_valid CHECK (password_epoch >= 0);

ALTER TABLE sessions
    ADD COLUMN password_epoch BIGINT NOT NULL DEFAULT 0;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_password_epoch_valid CHECK (password_epoch >= 0);

-- +goose Down
-- Dropping the columns lets every session of an account in again whatever its
-- epoch was; a session a reset already ended stays ended through revoked_at.
ALTER TABLE sessions
    DROP CONSTRAINT sessions_password_epoch_valid;
ALTER TABLE sessions
    DROP COLUMN password_epoch;
ALTER TABLE accounts
    DROP CONSTRAINT accounts_password_epoch_valid;
ALTER TABLE accounts
    DROP COLUMN password_epoch;
