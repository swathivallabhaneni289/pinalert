-- +goose Up
-- Phase 9: tapped star ratings of a reporter (plan 09-13). It serves D-02 (one
-- tapped rating per rater and reporter, shown only from five counted raters),
-- D-14 (a moderator can remove a rating, and ratings from a deleted account stop
-- counting), D-21 (raters count by distinct voter cell, so the table holds every
-- rating and the cell is read from the rater's qualifying vote, never stored
-- here), D-22, D-23, G2 and W4 (a moderator removes one rating or every rating by
-- one rater; the second also stamps the rater, who can never rate again).
--
-- One row per (rater, reporter). A rating a rater clears keeps its row with
-- cleared_at set, so the once a day change rule can see the last change. A rating
-- a moderator removes keeps its row with removed_at and removed_by set, so the
-- removal is an audit trail and the same rater cannot write it back. Every read of
-- the tally ignores rows with either set.
CREATE TABLE profile_ratings (
    rater_id    BIGINT NOT NULL REFERENCES accounts(id),
    reporter_id BIGINT NOT NULL REFERENCES accounts(id),
    stars       SMALLINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_at  TIMESTAMPTZ,
    removed_at  TIMESTAMPTZ,
    removed_by  BIGINT REFERENCES accounts(id),
    PRIMARY KEY (rater_id, reporter_id),
    CONSTRAINT profile_ratings_not_self CHECK (rater_id <> reporter_id),
    CONSTRAINT profile_ratings_stars_range CHECK (stars BETWEEN 1 AND 5),
    CONSTRAINT profile_ratings_removed_shape CHECK ((removed_at IS NULL) = (removed_by IS NULL))
);

-- Serves the tally read: only rows that can count are indexed.
CREATE INDEX idx_profile_ratings_reporter
    ON profile_ratings (reporter_id)
    WHERE removed_at IS NULL AND cleared_at IS NULL;

-- Serves the moderator's remove all by one rater and the account deletion.
CREATE INDEX idx_profile_ratings_rater ON profile_ratings (rater_id);

-- A rater a moderator removed (W4). RaterMayRate refuses such an account, so the
-- removal cannot be undone by rating again, and the tally ignores its ratings.
ALTER TABLE accounts
    ADD COLUMN rater_removed_at TIMESTAMPTZ,
    ADD COLUMN rater_removed_by BIGINT REFERENCES accounts(id);

ALTER TABLE accounts
    ADD CONSTRAINT accounts_rater_removed_shape
    CHECK ((rater_removed_at IS NULL) = (rater_removed_by IS NULL));

-- +goose Down
DROP TABLE profile_ratings;
ALTER TABLE accounts
    DROP CONSTRAINT accounts_rater_removed_shape,
    DROP COLUMN rater_removed_by,
    DROP COLUMN rater_removed_at;
