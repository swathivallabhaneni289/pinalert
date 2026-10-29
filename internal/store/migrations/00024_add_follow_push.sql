-- +goose Up
-- Phase 9 plan 25: the follow note reaches the phone (PROF-03, D-09, research
-- 5.2 "Phone"). Three closed lists of Phase 3.3's outbox learn the new kind, and
-- the per report fan out record gains the follow pass.
--
-- 1. push_outbox_kind_valid is the list of 00011 with follow appended. Phase 5's
--    official pushes live in official_push_outbox and never touched this list. A
--    later phase that adds a kind copies the list from the newest migration that
--    changed it.
-- 2. One first alert per account and report across all three first alert kinds
--    (critical, reliable, follow): uq_push_outbox_first is re-created over the
--    wider predicate. Every ON CONFLICT clause that writes any of the three kinds
--    names this same predicate, word for word. Follow wins the slot when it is
--    queued first; the dispatcher queues Critical, then Follow, then Reliable.
-- 3. push_report_progress records the follow pass of a report once: follow_state
--    and follow_at, with the same CHECK shape as reliable_state.
ALTER TABLE push_outbox
    DROP CONSTRAINT push_outbox_kind_valid;
ALTER TABLE push_outbox
    ADD CONSTRAINT push_outbox_kind_valid CHECK (
        kind IN ('critical', 'reliable', 'changed', 'confirmed_update', 'follow'));

DROP INDEX uq_push_outbox_first;
CREATE UNIQUE INDEX uq_push_outbox_first
    ON push_outbox (account_id, report_id)
    WHERE kind IN ('critical', 'reliable', 'follow');

ALTER TABLE push_report_progress
    ADD COLUMN follow_state TEXT,
    ADD COLUMN follow_at    TIMESTAMPTZ;

ALTER TABLE push_report_progress
    DROP CONSTRAINT push_report_progress_state_valid;
ALTER TABLE push_report_progress
    ADD CONSTRAINT push_report_progress_state_valid CHECK (
        (critical_state IS NULL OR critical_state IN ('fanned', 'capped'))
        AND (reliable_state IS NULL OR reliable_state IN ('fanned', 'capped'))
        AND (follow_state IS NULL OR follow_state IN ('fanned', 'capped')));

-- +goose Down
-- Follow rows go first: the old kind list refuses them, and the old index would
-- not know them.
DELETE FROM push_outbox WHERE kind = 'follow';

ALTER TABLE push_report_progress
    DROP CONSTRAINT push_report_progress_state_valid;
ALTER TABLE push_report_progress
    DROP COLUMN follow_at,
    DROP COLUMN follow_state;
ALTER TABLE push_report_progress
    ADD CONSTRAINT push_report_progress_state_valid CHECK (
        (critical_state IS NULL OR critical_state IN ('fanned', 'capped'))
        AND (reliable_state IS NULL OR reliable_state IN ('fanned', 'capped')));

DROP INDEX uq_push_outbox_first;
CREATE UNIQUE INDEX uq_push_outbox_first
    ON push_outbox (account_id, report_id)
    WHERE kind IN ('critical', 'reliable');

ALTER TABLE push_outbox
    DROP CONSTRAINT push_outbox_kind_valid;
ALTER TABLE push_outbox
    ADD CONSTRAINT push_outbox_kind_valid CHECK (
        kind IN ('critical', 'reliable', 'changed', 'confirmed_update'));
