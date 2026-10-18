-- +goose Up
-- Phase 3.2: in app updates (D-09, D-11, D-12, D-16), trusted reporter alerts
-- (D-26) and the two account lifecycle columns that Log out of all devices and
-- Delete my account need. One migration so the generated models change once.

-- One row per thing the bell announces. The row holds the kind of event and the
-- report it is about, never words, counts (other than more_count), a location
-- or a description: the sentence is written at read time from the report's
-- current facts, so a number can never be stale. The rolled up kind 'more'
-- stands for several blocked events in one clock hour (D-16) and has no
-- report.
CREATE TABLE updates (
    id           BIGSERIAL PRIMARY KEY,
    account_id   BIGINT NOT NULL REFERENCES accounts(id),
    report_id    BIGINT REFERENCES reports(id),
    kind         TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at      TIMESTAMPTZ,
    bucket_start TIMESTAMPTZ,
    more_count   INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT updates_kind_valid CHECK (kind IN (
        'reached_live', 'hidden', 'resolved', 'fading', 'reliable_nearby', 'more')),
    CONSTRAINT updates_more_shape CHECK (
        (kind = 'more' AND report_id IS NULL AND bucket_start IS NOT NULL AND more_count >= 1)
        OR
        (kind <> 'more' AND report_id IS NOT NULL AND bucket_start IS NULL AND more_count = 0))
);

-- Serves the list: newest first for one account.
CREATE INDEX idx_updates_account_created
    ON updates (account_id, created_at DESC, id DESC);

-- Serves the unread count: only unread rows are indexed.
CREATE INDEX idx_updates_account_unread
    ON updates (account_id)
    WHERE read_at IS NULL;

-- A Reliable alert is announced once per account per report, however many
-- polls see the same report.
CREATE UNIQUE INDEX uq_updates_reliable_once
    ON updates (account_id, report_id)
    WHERE kind = 'reliable_nearby';

-- The rolled up row exists once per account per clock hour; later blocked
-- events raise its count instead of adding rows.
CREATE UNIQUE INDEX uq_updates_more_bucket
    ON updates (account_id, bucket_start)
    WHERE kind = 'more';

-- The per pair bookkeeping behind exactly once announcements. last_state is
-- the last announced or silently adopted visibility; last_update_at is the time
-- of the last update written for this pair (the one per report per hour rule);
-- fading_for is the expiry that was announced, so a changed expiry arms the
-- fading event again. The followed set itself is derived, never stored (D-09).
CREATE TABLE update_cursors (
    account_id     BIGINT NOT NULL REFERENCES accounts(id),
    report_id      BIGINT NOT NULL REFERENCES reports(id),
    last_state     TEXT NOT NULL,
    last_update_at TIMESTAMPTZ,
    adopted_at     TIMESTAMPTZ NOT NULL,
    fading_for     TIMESTAMPTZ,
    PRIMARY KEY (account_id, report_id),
    CONSTRAINT update_cursors_state_valid CHECK (last_state IN (
        'hidden', 'provisional', 'live', 'retracted'))
);

-- Mute this report (D-11). A mute may exist for a report the account never
-- followed.
CREATE TABLE report_mutes (
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    report_id  BIGINT NOT NULL REFERENCES reports(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, report_id)
);

-- Serves the Muted reports list, newest mute first.
CREATE INDEX idx_report_mutes_account_created
    ON report_mutes (account_id, created_at DESC);

-- The alert switch and distance (D-26). alert_radius_km is the shared How far
-- from my home area row that Phase 3.3 reuses for every kind of alert; it is a
-- distance, never a location. The switch defaults to off.
CREATE TABLE account_alert_settings (
    account_id         BIGINT PRIMARY KEY REFERENCES accounts(id),
    reliable_alerts_on BOOLEAN NOT NULL DEFAULT false,
    alert_radius_km    SMALLINT NOT NULL DEFAULT 5,
    enabled_at         TIMESTAMPTZ,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT account_alert_settings_radius_valid CHECK (alert_radius_km IN (1, 2, 5, 10))
);

-- Log out of all devices: the gate query refuses a revoked session, while
-- authorship joins never read this column, so a revoked session still links
-- its reports to their author.
ALTER TABLE sessions
    ADD COLUMN revoked_at TIMESTAMPTZ;

-- Delete my account, anonymise (owner confirmed 2026-10-18): the row stays so
-- reports and votes keep their weight, the personal columns are cleared and
-- this stamps when.
ALTER TABLE accounts
    ADD COLUMN deleted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE accounts
    DROP COLUMN deleted_at;
ALTER TABLE sessions
    DROP COLUMN revoked_at;
DROP TABLE account_alert_settings;
DROP TABLE report_mutes;
DROP TABLE update_cursors;
DROP TABLE updates;
