-- +goose Up
-- Quick 261117-vvb (owner, 2026-11-17, 7.1 UAT test 7): a report flagged by
-- people in three different places is hidden for review until a moderator
-- keeps or removes it, and its author is told in the bell. This amends the
-- rule of 00007 that a flag never changes a report; a flag on a comment still
-- never hides it.
--
-- flagger_cell is the place cell of the flagger at the time of the flag, at
-- the voter cell precision (7). Only the cell is stored, never the position,
-- and it is read only by the hide decision: no query hands it to a client, the
-- moderator screen or a log. A flag sent with no known position has no cell
-- and never counts toward a hide.
ALTER TABLE report_flags
    ADD COLUMN flagger_cell TEXT;

ALTER TABLE report_flags
    ADD CONSTRAINT report_flags_flagger_cell_shape CHECK (
        flagger_cell IS NULL OR char_length(flagger_cell) = 7);

-- 'review' is the fourth moderation state: hidden for review by flags. It is
-- withheld everywhere like a removed report until a moderator acts.
ALTER TABLE reports
    DROP CONSTRAINT reports_moderation_state_valid;
ALTER TABLE reports
    ADD CONSTRAINT reports_moderation_state_valid
    CHECK (moderation_state IN ('clear', 'pending', 'removed', 'review'));

-- review_kept_at marks a report a moderator kept after a review hide. Flags
-- can never hide such a report again, so one report is hidden this way at most
-- once.
ALTER TABLE reports
    ADD COLUMN review_kept_at TIMESTAMPTZ;

-- Four bell rows written straight to the author: a held report passed the
-- late text check (own_shown), it was hidden for review (own_review), a
-- moderator kept it after that (own_kept) and it was removed (own_removed).
-- The kind list is the list of 00023 with the four appended.
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours', 'followed_posted', 'own_shown', 'own_review', 'own_kept', 'own_removed'));

-- Each of the four events happens at most once per report, so one row per
-- account, report and kind is the whole cap, and a repeated write is a no-op.
CREATE UNIQUE INDEX uq_updates_own_report
    ON updates (account_id, report_id, kind)
    WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed');

-- +goose Down
-- A report still hidden for review goes back to pending, the old schema's
-- nearest state: it stays hidden unless it is serious, and its open flags keep
-- it on the moderator's flagged list, where Keep or Remove still works.
DROP INDEX uq_updates_own_report;
DELETE FROM updates WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed');
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours', 'followed_posted'));
ALTER TABLE reports
    DROP COLUMN review_kept_at;
UPDATE reports SET moderation_state = 'pending' WHERE moderation_state = 'review';
ALTER TABLE reports
    DROP CONSTRAINT reports_moderation_state_valid;
ALTER TABLE reports
    ADD CONSTRAINT reports_moderation_state_valid
    CHECK (moderation_state IN ('clear', 'pending', 'removed'));
ALTER TABLE report_flags
    DROP CONSTRAINT report_flags_flagger_cell_shape;
ALTER TABLE report_flags
    DROP COLUMN flagger_cell;
