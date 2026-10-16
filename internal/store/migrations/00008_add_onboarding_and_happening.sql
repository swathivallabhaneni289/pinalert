-- +goose Up
-- Phase 3.4 (D-24b): the first-time flow record, the optional profile rows and
-- the What is happening column on reports.

-- Consent is stored as the version of the text that was agreed to, with the
-- time it was agreed, so a later change of the Disclaimer or Policy text can
-- ask again (D-27). first_run_done_at records that the first-time flow was
-- finished. display_name is the optional name. The home area is the picked
-- place's display name plus a geohash cell of precision 5 (about 5 km); the
-- raw coordinates are never stored. No defaults and no backfill: existing
-- accounts have no consent stored and see the first-time flow once.
ALTER TABLE accounts
    ADD COLUMN terms_agreed_at    TIMESTAMPTZ,
    ADD COLUMN terms_version      TEXT,
    ADD COLUMN first_run_done_at  TIMESTAMPTZ,
    ADD COLUMN display_name       TEXT,
    ADD COLUMN home_area_name     TEXT,
    ADD COLUMN home_area_cell     TEXT;

-- A version without a time, or a time without a version, would make the
-- consent record ambiguous.
ALTER TABLE accounts
    ADD CONSTRAINT accounts_terms_pair
    CHECK ((terms_agreed_at IS NULL) = (terms_version IS NULL));

ALTER TABLE accounts
    ADD CONSTRAINT accounts_display_name_len
    CHECK (display_name IS NULL OR char_length(display_name) BETWEEN 1 AND 60);

ALTER TABLE accounts
    ADD CONSTRAINT accounts_home_area_len
    CHECK (home_area_name IS NULL OR char_length(home_area_name) BETWEEN 1 AND 120);

-- Name and cell are set and cleared together.
ALTER TABLE accounts
    ADD CONSTRAINT accounts_home_area_pair
    CHECK ((home_area_name IS NULL) = (home_area_cell IS NULL));

-- The geohash alphabet has no a, i, l or o.
ALTER TABLE accounts
    ADD CONSTRAINT accounts_home_area_cell_shape
    CHECK (home_area_cell IS NULL OR home_area_cell ~ '^[0-9b-hjkmnp-z]{5}$');

-- What is happening, chosen on the report form. Nullable: older reports and
-- categories that do not ask for it have none.
ALTER TABLE reports
    ADD COLUMN happening TEXT;

ALTER TABLE reports
    ADD CONSTRAINT reports_happening_valid
    CHECK (happening IS NULL OR happening IN (
        'water_rising', 'people_trapped', 'fire', 'building_collapsed',
        'landslide', 'medical_emergency', 'other'));

-- +goose Down
ALTER TABLE reports
    DROP CONSTRAINT reports_happening_valid,
    DROP COLUMN happening;
ALTER TABLE accounts
    DROP CONSTRAINT accounts_home_area_cell_shape,
    DROP CONSTRAINT accounts_home_area_pair,
    DROP CONSTRAINT accounts_home_area_len,
    DROP CONSTRAINT accounts_display_name_len,
    DROP CONSTRAINT accounts_terms_pair,
    DROP COLUMN home_area_cell,
    DROP COLUMN home_area_name,
    DROP COLUMN display_name,
    DROP COLUMN first_run_done_at,
    DROP COLUMN terms_version,
    DROP COLUMN terms_agreed_at;
