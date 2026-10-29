-- +goose Up
-- Phase 9: followable reporter profiles. One migration for the opt in flag and
-- the two person scoped tables, so the generated models change once.
-- It serves D-01 (a profile is opt in and off by default), D-03 (named lists show
-- only people who opted in, everyone else is a count), D-12 (block) and D-21
-- (follows end softly, so ratings a moderator removes can restore the follows
-- they cost). Ratings, person mutes and the follow note columns each come with
-- their own later migration.
--
-- The two tables are person scoped on purpose: follow.go, report_mutes and
-- ViewerFollows already mean report level things (research Pitfall 7), and a
-- follow of a person is a different fact from a follow of a report.

-- "Let others follow me". Off by default (D-01). profile_public_at is the time it
-- was last switched on; it must be set whenever the profile is public.
ALTER TABLE accounts
    ADD COLUMN profile_public BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN profile_public_at TIMESTAMPTZ;

ALTER TABLE accounts
    ADD CONSTRAINT accounts_profile_public_at_set
    CHECK (NOT profile_public OR profile_public_at IS NOT NULL);

-- One row per follower and followee, never deleted except with an account
-- (D-21). Ending a follow sets ended_at and end_reason together; following again
-- reactivates the same row with a fresh created_at. The reasons are unfollowed
-- (the follower), removed (the followee took the follower away), blocked,
-- profile_off (the followee switched the profile off, went pending or was found
-- not followable) and tag_lost (the computed tag fell below Reliable reporter;
-- only these are ever restored). Every read of current follows filters
-- ended_at IS NULL.
CREATE TABLE account_follows (
    follower_id BIGINT NOT NULL REFERENCES accounts(id),
    followee_id BIGINT NOT NULL REFERENCES accounts(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    end_reason  TEXT,
    PRIMARY KEY (follower_id, followee_id),
    CONSTRAINT account_follows_not_self CHECK (follower_id <> followee_id),
    CONSTRAINT account_follows_end_shape CHECK ((ended_at IS NULL) = (end_reason IS NULL)),
    CONSTRAINT account_follows_end_reason_valid CHECK (
        end_reason IS NULL
        OR end_reason IN ('unfollowed', 'removed', 'blocked', 'profile_off', 'tag_lost'))
);

-- Serve the two list reads and the counts: only active rows are indexed.
CREATE INDEX idx_account_follows_followee
    ON account_follows (followee_id, created_at DESC, follower_id DESC)
    WHERE ended_at IS NULL;

CREATE INDEX idx_account_follows_follower
    ON account_follows (follower_id, created_at DESC, followee_id DESC)
    WHERE ended_at IS NULL;

-- Serves the restore of follows lost with the tag (D-21).
CREATE INDEX idx_account_follows_tag_lost
    ON account_follows (followee_id, ended_at)
    WHERE end_reason = 'tag_lost';

-- A person blocks a person (D-12). The block is a row here; the follows between
-- the two are ended by the service through endFollowPair, and FollowAccount
-- refuses while a block exists in either direction.
CREATE TABLE person_blocks (
    blocker_id BIGINT NOT NULL REFERENCES accounts(id),
    blocked_id BIGINT NOT NULL REFERENCES accounts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT person_blocks_not_self CHECK (blocker_id <> blocked_id)
);

CREATE INDEX idx_person_blocks_blocked ON person_blocks (blocked_id);

-- +goose Down
DROP TABLE person_blocks;
DROP TABLE account_follows;
ALTER TABLE accounts
    DROP CONSTRAINT accounts_profile_public_at_set,
    DROP COLUMN profile_public_at,
    DROP COLUMN profile_public;
