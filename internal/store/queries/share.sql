-- Phase 8 plan 02 (COORD-08, D-05, D-06): the share token queries. Nothing in
-- this file returns a coordinate to a response: latitude and longitude are read
-- only so the service can feed the coarse area lookup, and the service never
-- puts them in a link, a title or a page.

-- name: SetReportShareToken :one
-- Writes the share token of a report that has none. The WHERE share_token IS
-- NULL makes the first writer win: a second Share tap, even at the same moment,
-- gets no row back and re-reads the token that won. A token is never replaced,
-- so a link already forwarded keeps working.
UPDATE reports
SET share_token = sqlc.arg(token)::text
WHERE id = sqlc.arg(report_id)::bigint
  AND share_token IS NULL
RETURNING share_token;

-- name: ReportShareFacts :one
-- What Share needs from one report: the closed list fields that build the
-- title, the token and the stored area name, and whether the area lookup was
-- already tried (push_report_progress.area_lookup_at), so the lookup runs at
-- most once per report. Latitude and longitude feed that lookup only.
SELECT r.id, r.category, r.severity, r.happening, r.share_token, r.area_name,
       r.latitude, r.longitude,
       (p.area_lookup_at IS NOT NULL)::boolean AS area_lookup_tried
FROM reports r
LEFT JOIN push_report_progress p ON p.report_id = r.id
WHERE r.id = sqlc.arg(report_id)::bigint;

-- name: ReportByShareToken :one
-- The report a token names and its moderation state, so the public page can say
-- a removed report was removed. No row means an unknown token. It reads no
-- report words and no coordinates.
SELECT id, moderation_state
FROM reports
WHERE share_token = sqlc.arg(token)::text;
