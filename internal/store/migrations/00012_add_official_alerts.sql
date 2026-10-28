-- +goose Up
-- Phase 5: official alerts. D-15 puts them in their own tables and never in
-- reports, because reports needs a session, feeds every vote path and drives
-- the trust counts; an official alert has no votes and no reader. ALERT-02
-- (an alert leaves by its own expiry, cancel or replacement, never by absence
-- from the feed) is served by expires_at, cancelled_at and superseded_at.
-- ALERT-04 (an honest last checked time) is served by official_feed_status.
-- official_hazards holds the GDACS events of research G4 for plan 05-21.
--
-- The tables hold only official feed text and codes. No reader location is
-- ever stored here.

-- One row per feed. last_success_at moves only when a refresh really worked,
-- so the page can say when it last checked without ever claiming a check that
-- did not happen. last_error is one closed word, never a message, a URL or
-- part of a body.
CREATE TABLE official_feed_status (
    feed                 TEXT PRIMARY KEY,
    last_attempt_at      TIMESTAMPTZ,
    last_success_at      TIMESTAMPTZ,
    last_error           TEXT,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_etag            TEXT NOT NULL DEFAULT '',
    last_modified        TEXT NOT NULL DEFAULT '',
    items_seen           INTEGER NOT NULL DEFAULT 0,
    CONSTRAINT official_feed_status_feed_valid CHECK (feed IN ('sachet', 'gdacs')),
    CONSTRAINT official_feed_status_error_valid CHECK (
        last_error IS NULL OR last_error IN ('timeout', 'status', 'parse', 'anomaly', 'network')),
    CONSTRAINT official_feed_status_counts CHECK (
        consecutive_failures >= 0 AND items_seen >= 0)
);

INSERT INTO official_feed_status (feed) VALUES ('sachet'), ('gdacs')
ON CONFLICT (feed) DO NOTHING;

-- One row per SACHET series, keyed by the RSS guid. A later Update or Alert
-- version of the same guid replaces the row in place; cap_identifier is the
-- message's own identifier, the one a later message names in its references.
-- source_pubdate is the RSS pubDate of the stored version, so a refresh knows
-- whether an item changed without fetching it. All text is as stored by
-- official.CleanText and is shown only through html/template or textContent.
-- A Cancel is stored too, with cancelled_at set, so it never shows and is not
-- fetched again.
CREATE TABLE official_alerts (
    id                   BIGSERIAL PRIMARY KEY,
    guid                 TEXT NOT NULL,
    cap_identifier       TEXT NOT NULL,
    source_pubdate       TIMESTAMPTZ,
    sender               TEXT NOT NULL DEFAULT '',
    issuer               TEXT NOT NULL DEFAULT '',
    msg_type             TEXT NOT NULL,
    references_text      TEXT NOT NULL DEFAULT '',
    event                TEXT NOT NULL DEFAULT '',
    severity             TEXT NOT NULL,
    urgency              TEXT NOT NULL DEFAULT '',
    certainty            TEXT NOT NULL DEFAULT '',
    sent_at              TIMESTAMPTZ NOT NULL,
    effective_at         TIMESTAMPTZ,
    onset_at             TIMESTAMPTZ,
    expires_at           TIMESTAMPTZ NOT NULL,
    expires_inferred     BOOLEAN NOT NULL DEFAULT false,
    headline_en          TEXT NOT NULL DEFAULT '',
    headline_original    TEXT NOT NULL DEFAULT '',
    original_lang        TEXT NOT NULL DEFAULT '',
    description_en       TEXT NOT NULL DEFAULT '',
    description_original TEXT NOT NULL DEFAULT '',
    instruction          TEXT NOT NULL DEFAULT '',
    area_desc            TEXT NOT NULL DEFAULT '',
    placement_tier       TEXT NOT NULL,
    cancelled_at         TIMESTAMPTZ,
    superseded_at        TIMESTAMPTZ,
    first_seen_at        TIMESTAMPTZ NOT NULL,
    last_seen_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT official_alerts_guid_unique UNIQUE (guid),
    CONSTRAINT official_alerts_msg_type_valid CHECK (msg_type IN ('Alert', 'Update', 'Cancel')),
    CONSTRAINT official_alerts_severity_valid CHECK (
        severity IN ('Extreme', 'Severe', 'Moderate', 'Minor', 'Unknown')),
    CONSTRAINT official_alerts_placement_valid CHECK (
        placement_tier IN ('district', 'state', 'unplaced'))
);

-- Serves the in force read, which keeps only alerts that have not ended.
CREATE INDEX idx_official_alerts_expires_at ON official_alerts (expires_at);

-- Where an alert applies. A district row carries a district code alone and a
-- state row a state code alone; a district tier alert has district rows for the
-- districts it names and state rows for their states, a state tier alert has
-- state rows only. The unique index treats a missing code as 0 so a row cannot
-- be stored twice.
CREATE TABLE official_alert_areas (
    id           BIGSERIAL PRIMARY KEY,
    alert_id     BIGINT NOT NULL REFERENCES official_alerts(id),
    district_lgd INTEGER,
    state_lgd    INTEGER,
    CONSTRAINT official_alert_areas_code_present CHECK (
        district_lgd IS NOT NULL OR state_lgd IS NOT NULL)
);

CREATE UNIQUE INDEX uq_official_alert_areas
    ON official_alert_areas (alert_id, COALESCE(district_lgd, 0), COALESCE(state_lgd, 0));
CREATE INDEX idx_official_alert_areas_district ON official_alert_areas (district_lgd);
CREATE INDEX idx_official_alert_areas_state ON official_alert_areas (state_lgd);

-- GDACS hazard events (research G4), one row per event, never in reports. The
-- columns follow what the GDACS search answer carries. is_current and to_date
-- decide when a pin leaves the map.
CREATE TABLE official_hazards (
    id            BIGSERIAL PRIMARY KEY,
    event_type    TEXT NOT NULL,
    event_id      BIGINT NOT NULL,
    episode_id    BIGINT,
    name          TEXT NOT NULL DEFAULT '',
    alert_level   TEXT NOT NULL DEFAULT '',
    country       TEXT NOT NULL DEFAULT '',
    latitude      DOUBLE PRECISION NOT NULL,
    longitude     DOUBLE PRECISION NOT NULL,
    from_date     TIMESTAMPTZ,
    to_date       TIMESTAMPTZ,
    is_current    BOOLEAN NOT NULL DEFAULT false,
    severity_text TEXT NOT NULL DEFAULT '',
    report_url    TEXT NOT NULL DEFAULT '',
    date_modified TIMESTAMPTZ,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT official_hazards_event_unique UNIQUE (event_type, event_id),
    CONSTRAINT official_hazards_position_valid CHECK (
        latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
);

-- +goose Down
DROP TABLE official_hazards;
DROP TABLE official_alert_areas;
DROP TABLE official_alerts;
DROP TABLE official_feed_status;
