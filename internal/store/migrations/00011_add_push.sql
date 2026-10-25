-- +goose Up
-- Phase 3.3: phone alerts. One migration so the generated models change once.
-- It serves D-03 (the subscription and the 404 and 410 handling), D-13 (one
-- subscription per device, the endpoint is unique), D-16 and D-19 (the coarse
-- area name), decision 6 (no rolled up kind in the outbox) and decision 12
-- (Critical alerts are on by default, Changes are off).

-- One row per device that agreed to alerts. The endpoint is a capability URL
-- and a secret: whoever holds it can send to the device. It is never logged,
-- never put in a URL and never returned by a route; endpoint_host is the only
-- part of it that may appear in a log line. session_id is the session that
-- registered the device, so plain Log out can delete the rows of that session.
-- The endpoint is unique, so registering an endpoint from another account moves
-- it to that account (a shared phone must stop delivering the previous
-- account's alerts).
CREATE TABLE push_subscriptions (
    id            BIGSERIAL PRIMARY KEY,
    account_id    BIGINT NOT NULL REFERENCES accounts(id),
    session_id    TEXT NOT NULL REFERENCES sessions(session_id),
    endpoint      TEXT NOT NULL UNIQUE,
    endpoint_host TEXT NOT NULL,
    p256dh        TEXT NOT NULL,
    auth          TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_ok_at    TIMESTAMPTZ,
    failures      INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT push_subscriptions_endpoint_shape CHECK (
        char_length(endpoint) BETWEEN 9 AND 2048 AND endpoint LIKE 'https://%'),
    CONSTRAINT push_subscriptions_key_len CHECK (
        char_length(p256dh) BETWEEN 1 AND 128 AND char_length(auth) BETWEEN 1 AND 64)
);

CREATE INDEX idx_push_subscriptions_account ON push_subscriptions (account_id);
CREATE INDEX idx_push_subscriptions_session ON push_subscriptions (session_id);

-- The work queue the sender claims from. The table stores no words, no payload,
-- no coordinate and no endpoint: the words are built at send time from the
-- report's current facts. The kind list has no rolled up kind (decision 6);
-- Phase 5 adds official and Phase 9 adds follow with their own migrations.
-- update_id is the 3.2 updates row a changed push carries; it deliberately has
-- no foreign key, because 3.2 prunes updates after 7 days.
-- sent_unconfirmed is true when the words sent said Not confirmed yet, so only
-- those get the in place confirmed update.
CREATE TABLE push_outbox (
    id               BIGSERIAL PRIMARY KEY,
    account_id       BIGINT NOT NULL REFERENCES accounts(id),
    report_id        BIGINT NOT NULL REFERENCES reports(id),
    kind             TEXT NOT NULL,
    update_id        BIGINT,
    state            TEXT NOT NULL DEFAULT 'queued',
    skip_reason      TEXT,
    attempts         INTEGER NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_until      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    sent_at          TIMESTAMPTZ,
    sent_unconfirmed BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT push_outbox_kind_valid CHECK (
        kind IN ('critical', 'reliable', 'changed', 'confirmed_update')),
    CONSTRAINT push_outbox_state_valid CHECK (
        state IN ('queued', 'sent', 'skipped', 'failed')),
    CONSTRAINT push_outbox_skip_reason_valid CHECK (
        skip_reason IS NULL OR skip_reason IN (
            'cap_account_hour', 'not_eligible', 'no_home_area', 'out_of_range',
            'no_device', 'stale')),
    CONSTRAINT push_outbox_update_shape CHECK ((kind = 'changed') = (update_id IS NOT NULL))
);

-- One initial alert per account and report, whatever its kind; the dispatcher
-- enqueues critical before reliable, so critical wins.
CREATE UNIQUE INDEX uq_push_outbox_first
    ON push_outbox (account_id, report_id)
    WHERE kind IN ('critical', 'reliable');

-- At most one in place confirmed update per account and report.
CREATE UNIQUE INDEX uq_push_outbox_confirmed
    ON push_outbox (account_id, report_id)
    WHERE kind = 'confirmed_update';

-- At most one changed push per account and 3.2 update row.
CREATE UNIQUE INDEX uq_push_outbox_changed
    ON push_outbox (account_id, update_id)
    WHERE kind = 'changed';

-- Serves the claim: only queued rows are indexed.
CREATE INDEX idx_push_outbox_due
    ON push_outbox (next_attempt_at)
    WHERE state = 'queued';

-- Serves the per account hourly count of sent pushes.
CREATE INDEX idx_push_outbox_account_sent
    ON push_outbox (account_id, sent_at)
    WHERE state = 'sent';

-- Where each report stands in the fan out. A capped report stays capped: the
-- per reporter fan out cap is sticky, so a report over the cap is never fanned
-- out late. area_lookup_at records that the reverse lookup of the area name was
-- tried; evaluated_at that the sweep looked at the report.
CREATE TABLE push_report_progress (
    report_id      BIGINT PRIMARY KEY REFERENCES reports(id),
    critical_state TEXT,
    critical_at    TIMESTAMPTZ,
    reliable_state TEXT,
    reliable_at    TIMESTAMPTZ,
    area_lookup_at TIMESTAMPTZ,
    evaluated_at   TIMESTAMPTZ,
    CONSTRAINT push_report_progress_state_valid CHECK (
        (critical_state IS NULL OR critical_state IN ('fanned', 'capped'))
        AND (reliable_state IS NULL OR reliable_state IN ('fanned', 'capped')))
);

-- The coarse area or landmark name from the reverse lookup. Read only by push
-- code: never by the feed, never in a bell row and never in any JSON response
-- (D-19, D-20).
ALTER TABLE reports
    ADD COLUMN area_name TEXT;

ALTER TABLE reports
    ADD CONSTRAINT reports_area_name_len
    CHECK (area_name IS NULL OR char_length(area_name) BETWEEN 1 AND 120);

-- Two more switches on the shared alert settings row. An account with no row
-- reads Critical as on and Changes as off through COALESCE in every read, the
-- same value as the Go constant DefaultCriticalPushOn. Phase 5 will add
-- official_push_on in its own migration (named here, not added).
ALTER TABLE account_alert_settings
    ADD COLUMN critical_push_on BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN changes_push_on BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN changes_enabled_at TIMESTAMPTZ;

-- The inverse lookup: which accounts have a home cell near a report.
CREATE INDEX idx_accounts_home_area_cell
    ON accounts (home_area_cell)
    WHERE home_area_cell IS NOT NULL;

-- +goose Down
DROP INDEX idx_accounts_home_area_cell;
ALTER TABLE account_alert_settings
    DROP COLUMN changes_enabled_at,
    DROP COLUMN changes_push_on,
    DROP COLUMN critical_push_on;
ALTER TABLE reports
    DROP CONSTRAINT reports_area_name_len,
    DROP COLUMN area_name;
DROP TABLE push_report_progress;
DROP TABLE push_outbox;
DROP TABLE push_subscriptions;
