-- +goose Up
-- Plan 04-31 (owner, 2026-10-22, 04-UAT test 10): the Flag window asks why the
-- report is a problem, lets a person tick every reason that applies and say
-- more in an optional note of at most 300 characters. A flag therefore holds a
-- set of reasons and a note instead of one reason.
--
-- The old single reason column is migrated into the set in this migration and
-- then dropped, so a flag has exactly one place that says why it was sent and
-- a stored flag can never disagree with itself. Every existing flag keeps its
-- reason as a one element set. One flag per account per report stays
-- (report_flags_one_per_account is untouched).
--
-- The note is free text written by the flagger and read only by moderators. It
-- is trimmed by the service, and an empty note is stored as NULL, which the
-- length check enforces. It never holds report text (T-04-04).

-- No reason may appear twice in one flag. A CHECK cannot hold a subquery, so
-- the rule is this small immutable function.
-- +goose StatementBegin
CREATE FUNCTION report_flag_reasons_distinct(r TEXT[]) RETURNS BOOLEAN
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT count(DISTINCT x) = count(*) FROM unnest(r) AS x
$$;
-- +goose StatementEnd

ALTER TABLE report_flags ADD COLUMN reasons TEXT[];
ALTER TABLE report_flags ADD COLUMN note TEXT;

UPDATE report_flags SET reasons = ARRAY[reason];

ALTER TABLE report_flags ALTER COLUMN reasons SET NOT NULL;

ALTER TABLE report_flags
    ADD CONSTRAINT report_flags_reasons_valid CHECK (
        cardinality(reasons) >= 1
        AND array_ndims(reasons) = 1
        AND array_position(reasons, NULL) IS NULL
        AND reasons <@ ARRAY['spam', 'abusive', 'false', 'dangerous', 'other']::TEXT[]
        AND report_flag_reasons_distinct(reasons)
    ),
    ADD CONSTRAINT report_flags_note_length CHECK (
        note IS NULL OR char_length(note) BETWEEN 1 AND 300
    );

ALTER TABLE report_flags DROP CONSTRAINT report_flags_reason_valid;
ALTER TABLE report_flags DROP COLUMN reason;

-- +goose Down
-- The single reason comes back as the first reason of each set, which is the
-- first in the dialog's order because the service stores the set in that
-- order. A flag that held more than one reason keeps only that first one, and
-- every note is lost: the old schema had nowhere to keep either.
ALTER TABLE report_flags ADD COLUMN reason TEXT;
UPDATE report_flags SET reason = reasons[1];
ALTER TABLE report_flags ALTER COLUMN reason SET NOT NULL;
ALTER TABLE report_flags
    ADD CONSTRAINT report_flags_reason_valid CHECK (reason IN ('spam', 'abusive', 'false', 'dangerous', 'other'));
ALTER TABLE report_flags DROP CONSTRAINT report_flags_note_length;
ALTER TABLE report_flags DROP CONSTRAINT report_flags_reasons_valid;
ALTER TABLE report_flags DROP COLUMN note;
ALTER TABLE report_flags DROP COLUMN reasons;
DROP FUNCTION report_flag_reasons_distinct(TEXT[]);
