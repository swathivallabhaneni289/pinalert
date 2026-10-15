-- +goose Up
-- Phase 4: moderation state, retry safe submits and the moderator tables.

-- The moderation state lives on the report row itself, so it is written by the
-- same INSERT that makes the row and a held report is never readable without
-- its state (R-01). Existing and seeded rows become 'clear' through the
-- default (R-10). NearbyReports does not name this column; the feed reads it
-- through one batched extras query instead.
ALTER TABLE reports
    ADD COLUMN moderation_state TEXT NOT NULL DEFAULT 'clear';

ALTER TABLE reports
    ADD CONSTRAINT reports_moderation_state_valid
    CHECK (moderation_state IN ('clear', 'pending', 'removed'));

-- request_id is a client generated retry key and request_account_id is the
-- account that sent it. client_written_at is the time the client says the
-- report was written (offline queue). It is display only: never used for
-- expiry, ordering or the age stage (R-05, D-19).
ALTER TABLE reports
    ADD COLUMN request_id TEXT,
    ADD COLUMN request_account_id BIGINT REFERENCES accounts(id),
    ADD COLUMN client_written_at TIMESTAMPTZ;

-- A request id without an account would make the unique index below useless,
-- because NULLs never collide in a unique index.
ALTER TABLE reports
    ADD CONSTRAINT reports_request_id_needs_account
    CHECK (request_id IS NULL OR request_account_id IS NOT NULL);

-- Keyed on the account, not the session, because one account holds several
-- sessions and a re-login creates a new one. Partial so reports sent without a
-- request id (and every older row) are never deduplicated.
CREATE UNIQUE INDEX idx_reports_request_idempotency
    ON reports (request_account_id, request_id)
    WHERE request_id IS NOT NULL;

-- One row per report that went (or is going) through the text check. Scores
-- are kept without any report text so thresholds can be tuned later (D-04).
-- Seeded and older reports have no row here, so they never reach the provider
-- or the review lists.
CREATE TABLE moderation_checks (
    report_id        BIGINT PRIMARY KEY REFERENCES reports(id),
    status           TEXT NOT NULL DEFAULT 'pending',
    attempts         INTEGER NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until      TIMESTAMPTZ,
    -- An error class such as a status code, never report text.
    last_error       TEXT,
    verdict          TEXT,
    scores           JSONB,
    model            TEXT,
    needs_review     BOOLEAN NOT NULL DEFAULT false,
    checked_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT moderation_checks_status_valid CHECK (status IN ('pending', 'done')),
    CONSTRAINT moderation_checks_attempts_valid CHECK (attempts >= 0),
    CONSTRAINT moderation_checks_verdict_valid CHECK (verdict IS NULL OR verdict IN ('pass', 'borderline', 'reject'))
);

-- Serves the worker claim: only unfinished checks, soonest due first.
CREATE INDEX idx_moderation_checks_due
    ON moderation_checks (next_attempt_at)
    WHERE status = 'pending';

-- Serves the review list: only checks a moderator still has to look at.
CREATE INDEX idx_moderation_checks_review
    ON moderation_checks (report_id)
    WHERE needs_review;

-- A flag never hides a report: nothing on reports changes when one is written
-- (D-09). One flag per account per report, with a closed list of reasons.
CREATE TABLE report_flags (
    id            BIGSERIAL PRIMARY KEY,
    report_id     BIGINT NOT NULL REFERENCES reports(id),
    account_id    BIGINT NOT NULL REFERENCES accounts(id),
    reason        TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dismissed_at  TIMESTAMPTZ,
    CONSTRAINT report_flags_reason_valid CHECK (reason IN ('spam', 'abusive', 'false', 'dangerous', 'other')),
    CONSTRAINT report_flags_one_per_account UNIQUE (report_id, account_id)
);

-- Serves the flagged list and the open flag counts.
CREATE INDEX idx_report_flags_open
    ON report_flags (report_id)
    WHERE dismissed_at IS NULL;

-- An audit trail of Keep and Remove with no report text (T-04-15, T-04-04).
CREATE TABLE moderation_actions (
    id                    BIGSERIAL PRIMARY KEY,
    report_id             BIGINT NOT NULL REFERENCES reports(id),
    moderator_account_id  BIGINT NOT NULL REFERENCES accounts(id),
    action                TEXT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT moderation_actions_action_valid CHECK (action IN ('keep', 'remove'))
);

-- +goose Down
DROP TABLE moderation_actions;
DROP TABLE report_flags;
DROP TABLE moderation_checks;
DROP INDEX idx_reports_request_idempotency;
ALTER TABLE reports
    DROP CONSTRAINT reports_request_id_needs_account,
    DROP CONSTRAINT reports_moderation_state_valid,
    DROP COLUMN client_written_at,
    DROP COLUMN request_account_id,
    DROP COLUMN request_id,
    DROP COLUMN moderation_state;
