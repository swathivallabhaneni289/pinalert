-- Source: github.com/pressly/goose README [CITED]
-- +goose Up
CREATE TABLE reports (
    id                       BIGSERIAL PRIMARY KEY,
    session_id               TEXT NOT NULL,
    category                 TEXT NOT NULL,
    severity                 TEXT NOT NULL,
    description              TEXT NOT NULL,
    latitude                 DOUBLE PRECISION NOT NULL,
    longitude                DOUBLE PRECISION NOT NULL,
    geohash                  TEXT NOT NULL,
    shelter_capacity_status  TEXT,
    shelter_headcount        INTEGER,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at               TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_reports_lat_lon ON reports (latitude, longitude);
CREATE INDEX idx_reports_expires_at ON reports (expires_at);
CREATE INDEX idx_reports_geohash ON reports (geohash);

CREATE TABLE sessions (
    session_id  TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE reports;
DROP TABLE sessions;
