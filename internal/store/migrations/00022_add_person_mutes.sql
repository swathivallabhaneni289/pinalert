-- +goose Up
-- Phase 9: person mute (plan 09-18). It serves D-12 (a reader can mute a person
-- so that person level notes stop; a mute never hides any report from any feed,
-- map, list or profile, because hiding emergency information is never safe) and
-- research G4 (mute is person scoped, so it is a table of its own).
--
-- Name collision: report_mutes (Phase 3.2) mutes one report for one reader and
-- is keyed by report. person_mutes is keyed by the two accounts. The two never
-- share a row, a query or a meaning, and neither reads the other.
--
-- One row per (muter, muted). A mute is one way and quiet: the muted person is
-- never told, and nothing here is readable by them. Deleting either account
-- removes the row (DeleteAccountPersonMutes).
CREATE TABLE person_mutes (
    muter_id   BIGINT NOT NULL REFERENCES accounts(id),
    muted_id   BIGINT NOT NULL REFERENCES accounts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (muter_id, muted_id),
    CONSTRAINT person_mutes_not_self CHECK (muter_id <> muted_id)
);

-- Serves the account deletion of the rows that name the muted side (the primary
-- key already serves the muter side and the per reader reads).
CREATE INDEX idx_person_mutes_muted ON person_mutes (muted_id);

-- +goose Down
DROP TABLE person_mutes;
