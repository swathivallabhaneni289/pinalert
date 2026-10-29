-- +goose Up
-- Phase 9 plan 21: the data behind the follow note (PROF-03, D-08, D-09,
-- research Pitfall 5).
--
-- 1. A follow carries one on and off flag, Notify me (D-08), off by default,
--    with the time it was turned on. Notes are only for reports posted after
--    that time. The two columns move together: notify is true exactly when
--    notify_since is set.
-- 2. A new update kind, followed_posted, says a person you follow with Notify me
--    on posted a report. The row holds the kind and the report id only; the
--    sentence is written at read time (D-15 of Phase 3.2). The kind list below
--    is the list of 00019 with the new kind appended, in that order. A later
--    phase that adds a kind copies the list from the newest migration that
--    changed it.
-- 3. One note per account and report across both first alert kinds (D-09,
--    Pitfall 5): the Reliable reporter alert's own unique index is replaced by
--    one partial index over both kinds, so a person who gets the Reliable
--    reporter alert for a report cannot also get the follow note for it, in
--    either order. Every ON CONFLICT clause that writes either kind names this
--    same predicate, word for word. Phase 8's unread index for the two comment
--    kinds is not touched here.
ALTER TABLE account_follows
    ADD COLUMN notify       BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN notify_since TIMESTAMPTZ;

ALTER TABLE account_follows
    ADD CONSTRAINT account_follows_notify_since CHECK (notify = (notify_since IS NOT NULL));

-- Serve the two note reads: the bells one account has on, and the followers of
-- one person with the bell on. Only rows with Notify me on are indexed.
CREATE INDEX idx_account_follows_notify_follower
    ON account_follows (follower_id)
    WHERE notify;

CREATE INDEX idx_account_follows_notify_followee
    ON account_follows (followee_id)
    WHERE notify;

ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours', 'followed_posted'));

DROP INDEX uq_updates_reliable_once;
CREATE UNIQUE INDEX uq_updates_first_alert
    ON updates (account_id, report_id)
    WHERE kind IN ('reliable_nearby', 'followed_posted');

-- +goose Down
DELETE FROM updates WHERE kind = 'followed_posted';
DROP INDEX uq_updates_first_alert;
CREATE UNIQUE INDEX uq_updates_reliable_once
    ON updates (account_id, report_id)
    WHERE kind = 'reliable_nearby';
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours'));

DROP INDEX idx_account_follows_notify_followee;
DROP INDEX idx_account_follows_notify_follower;
ALTER TABLE account_follows
    DROP CONSTRAINT account_follows_notify_since;
ALTER TABLE account_follows
    DROP COLUMN notify_since,
    DROP COLUMN notify;
