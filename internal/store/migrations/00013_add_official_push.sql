-- +goose Up
-- Phase 5: official alerts reach the phone (plan 05-11, owner decisions D-11 and
-- D-16). Severe and Extreme official alerts for a person's home area are queued
-- here, on by default, and sent silently by the official push lane (plan 05-16).
--
-- Official pushes get a table of their own and never use push_outbox: that
-- table's report_id is required and its CHECK lists name report kinds only, and
-- nothing of Phase 3.3's outbox, constraints or queries changes (research G7).

-- One row per person per version of an alert. The table stores ids and closed
-- words only: no title, no body, no position and no endpoint. The lock screen
-- words are built at send time from closed tables, and recipients are chosen
-- by the stored home cell, never by a position.
--
-- cap_identifier is the identifier of the message version the row was queued
-- for. The unique index lets a person be queued once for each version, so a
-- retried transaction can neither double a push nor lose one, and a later
-- version that raises the severity can queue a new row (reason escalated).
--
-- skip_reason lists why a row was not sent. already_notified was added on
-- 2026-10-28 for plan 05-16's Apple rule: an Apple endpoint gets only the first
-- official push about an alert series.
CREATE TABLE official_push_outbox (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES accounts(id),
    alert_id        BIGINT NOT NULL REFERENCES official_alerts(id),
    cap_identifier  TEXT NOT NULL,
    reason          TEXT NOT NULL,
    state           TEXT NOT NULL DEFAULT 'queued',
    skip_reason     TEXT,
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    sent_at         TIMESTAMPTZ,
    CONSTRAINT official_push_outbox_reason_valid CHECK (
        reason IN ('new', 'escalated')),
    CONSTRAINT official_push_outbox_state_valid CHECK (
        state IN ('queued', 'sent', 'skipped', 'failed')),
    CONSTRAINT official_push_outbox_skip_reason_valid CHECK (
        skip_reason IS NULL OR skip_reason IN (
            'not_eligible', 'stale', 'no_home_area', 'out_of_area', 'no_device',
            'cap_account', 'switched_off', 'feed_not_fresh', 'already_notified'))
);

-- Once per person, alert and message version.
CREATE UNIQUE INDEX uq_official_push_once
    ON official_push_outbox (account_id, alert_id, cap_identifier);

-- Serves the claim of due rows.
CREATE INDEX idx_official_push_outbox_due
    ON official_push_outbox (state, next_attempt_at);

-- Serves the per account count of sent official pushes in a window.
CREATE INDEX idx_official_push_outbox_account_sent
    ON official_push_outbox (account_id, sent_at)
    WHERE state = 'sent';

-- The switch "Severe official alerts for my area". An account with no settings
-- row reads it as on through COALESCE in every read, the same value as the Go
-- constant DefaultOfficialPushOn (Phase 3.3's decision 12 pattern).
ALTER TABLE account_alert_settings
    ADD COLUMN official_push_on BOOLEAN NOT NULL DEFAULT true;

-- +goose Down
DROP TABLE official_push_outbox;
ALTER TABLE account_alert_settings
    DROP COLUMN official_push_on;
