-- +goose Up
-- Phase 8 plan 11 (D-16, PAGE-02, A-P8-3): two more things the bell can
-- announce. comment_on_yours says someone commented on your report;
-- reply_to_yours says someone replied to your comment.
--
-- A comment row holds a kind and a report id only, like every other update. No
-- comment words and no username are ever stored here: the sentence is written
-- at read time from the kind (D-15 of Phase 3.2, T-08-49).
--
-- The unread index makes thirty comments on one report one unread row per kind:
-- a second comment while the first row is still unread conflicts and writes
-- nothing. A read row does not conflict, so the next comment after the person
-- looked gives a new row. The index is scoped to the two comment kinds, so
-- Phase 9's index on its own first alert kind never collides with it, and it is
-- the only cap these rows have (they never roll up into "more" and do not count
-- toward the hourly ceiling).
--
-- The kind list below is the list of 00009 with the two new kinds appended, in
-- that order. A later phase that adds a kind copies the list from the newest
-- migration that changed it.
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours'));

CREATE UNIQUE INDEX uq_updates_comment_unread
    ON updates (account_id, report_id, kind)
    WHERE read_at IS NULL AND kind IN ('comment_on_yours', 'reply_to_yours');

-- +goose Down
DROP INDEX uq_updates_comment_unread;
DELETE FROM updates WHERE kind IN ('comment_on_yours', 'reply_to_yours');
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more'));
