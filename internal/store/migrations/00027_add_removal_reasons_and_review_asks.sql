-- +goose Up
-- Quick 261118-rz6 (owner, 2026-11-18, 7.1 UAT): a removed report shows its
-- author why it was removed, the author can ask for a review from the bell up
-- to 3 times per report, and a moderator can restore a removed report or keep
-- it removed. This amends the rule of 00007 that removal is terminal: only a
-- moderator's Restore brings a removed report back.
--
-- removed_reason is the reason of the current removal: one of the five a
-- moderator picks, or text_check when the late text check removed it.
-- removed_at is when the current removal happened, for the moderator's
-- Removed list. Both are set together on every move to removed and cleared
-- together by Restore, and neither is set on a report that is not removed.
-- Reports removed before this migration have no reason; their removal time is
-- taken from the latest Remove action, else from the rejecting check, else
-- from when the report was made.
ALTER TABLE reports
    ADD COLUMN removed_reason TEXT,
    ADD COLUMN removed_at TIMESTAMPTZ;

UPDATE reports r
SET removed_at = COALESCE(
        (SELECT max(a.created_at) FROM moderation_actions a
          WHERE a.report_id = r.id AND a.action = 'remove'),
        (SELECT c.checked_at FROM moderation_checks c
          WHERE c.report_id = r.id AND c.verdict = 'reject'),
        r.created_at)
WHERE r.moderation_state = 'removed';

ALTER TABLE reports
    ADD CONSTRAINT reports_removed_reason_valid CHECK (
        removed_reason IS NULL
        OR removed_reason IN ('spam', 'abusive', 'false', 'dangerous', 'other', 'text_check'));
ALTER TABLE reports
    ADD CONSTRAINT reports_removed_fields_shape CHECK (
        moderation_state = 'removed' OR (removed_reason IS NULL AND removed_at IS NULL));

-- The moderator's audit row gains the reason of a Remove and two actions:
-- restore (a removed report shown again) and keep_removed (an ask for review
-- answered no). Only a Remove carries a reason; Remove rows written before
-- this migration have none.
ALTER TABLE moderation_actions
    ADD COLUMN reason TEXT;
ALTER TABLE moderation_actions
    ADD CONSTRAINT moderation_actions_reason_valid CHECK (
        reason IS NULL OR (action = 'remove' AND reason IN ('spam', 'abusive', 'false', 'dangerous', 'other')));
ALTER TABLE moderation_actions
    DROP CONSTRAINT moderation_actions_action_valid;
ALTER TABLE moderation_actions
    ADD CONSTRAINT moderation_actions_action_valid
    CHECK (action IN ('keep', 'remove', 'restore', 'keep_removed'));

-- One ask for review by the author of a removed report. The note is the
-- author's own plain text for moderators only, at most 200 characters; it is
-- never sent to the text check, never logged and never in a public answer.
-- An ask is open until a moderator answers it: restored or kept_removed. At
-- most one open ask per report (the partial unique index); at most 3 asks per
-- report in total, which the service decides under the report row lock in the
-- same transaction as the insert.
CREATE TABLE report_review_asks (
    id                     BIGSERIAL PRIMARY KEY,
    report_id              BIGINT NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    account_id             BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    note                   TEXT,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    answered_at            TIMESTAMPTZ,
    answer                 TEXT,
    answered_by_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    CONSTRAINT report_review_asks_note_length CHECK (note IS NULL OR char_length(note) <= 200),
    CONSTRAINT report_review_asks_answer_valid CHECK (answer IS NULL OR answer IN ('restored', 'kept_removed')),
    CONSTRAINT report_review_asks_answer_shape CHECK ((answered_at IS NULL) = (answer IS NULL))
);

-- At most one open ask per report. It also serves the moderator's Review asks
-- list and its count.
CREATE UNIQUE INDEX uq_report_review_asks_open
    ON report_review_asks (report_id)
    WHERE answered_at IS NULL;

-- Serves the count of asks per report.
CREATE INDEX idx_report_review_asks_report
    ON report_review_asks (report_id);

-- Serves the Removed list: removed reports, newest removal first.
CREATE INDEX idx_reports_removed_at
    ON reports (removed_at DESC)
    WHERE moderation_state = 'removed';

-- Two more bell rows written straight to the author: a moderator restored the
-- report (own_restored) or answered an ask with keep removed
-- (own_stays_removed). The kind list is the list of 00026 with the two
-- appended.
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours', 'followed_posted', 'own_shown', 'own_review', 'own_kept', 'own_removed', 'own_restored', 'own_stays_removed'));

-- Still one row per account, report and kind for every own kind, now six. An
-- own event can happen again (a second removal after a Restore, a second keep
-- removed), and the writers then move the one row's time to now and make it
-- unread instead of adding a second row, so the bell never piles up.
DROP INDEX uq_updates_own_report;
CREATE UNIQUE INDEX uq_updates_own_report
    ON updates (account_id, report_id, kind)
    WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed', 'own_restored', 'own_stays_removed');

-- +goose Down
-- The two new bell rows and the restore and keep_removed audit rows are
-- deleted, because the old checks do not allow them. A report that was
-- restored stays clear. Nothing else moves.
DROP INDEX uq_updates_own_report;
DELETE FROM updates WHERE kind IN ('own_restored', 'own_stays_removed');
ALTER TABLE updates
    DROP CONSTRAINT updates_kind_valid;
ALTER TABLE updates
    ADD CONSTRAINT updates_kind_valid CHECK (kind IN ('reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more', 'comment_on_yours', 'reply_to_yours', 'followed_posted', 'own_shown', 'own_review', 'own_kept', 'own_removed'));
CREATE UNIQUE INDEX uq_updates_own_report
    ON updates (account_id, report_id, kind)
    WHERE kind IN ('own_shown', 'own_review', 'own_kept', 'own_removed');

DROP INDEX idx_reports_removed_at;
DROP TABLE report_review_asks;

DELETE FROM moderation_actions WHERE action IN ('restore', 'keep_removed');
ALTER TABLE moderation_actions
    DROP CONSTRAINT moderation_actions_action_valid;
ALTER TABLE moderation_actions
    ADD CONSTRAINT moderation_actions_action_valid CHECK (action IN ('keep', 'remove'));
ALTER TABLE moderation_actions
    DROP CONSTRAINT moderation_actions_reason_valid;
ALTER TABLE moderation_actions
    DROP COLUMN reason;

ALTER TABLE reports
    DROP CONSTRAINT reports_removed_fields_shape;
ALTER TABLE reports
    DROP CONSTRAINT reports_removed_reason_valid;
ALTER TABLE reports
    DROP COLUMN removed_at,
    DROP COLUMN removed_reason;
