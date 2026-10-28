-- Phase 5 queries: official alerts (SACHET), their areas, the feed status and
-- the in force read. These tables hold only official feed text and codes. They
-- hold no reader location and are never joined to reports or votes (D-15).
--
-- Business queries take the time as an argument and never call now(), so tests
-- can pin the clock (the push.sql rule).
--
-- An alert is in force while expires_at is after the given time and neither
-- cancelled_at nor superseded_at is set. It never leaves because it is missing
-- from the feed: the feed is a sliding 24 hour window.

-- name: GetFeedStatus :one
SELECT feed, last_attempt_at, last_success_at, last_error, consecutive_failures,
       last_etag, last_modified, items_seen
FROM official_feed_status
WHERE feed = sqlc.arg(feed)::text;

-- name: MarkFeedAttempt :execrows
-- Claims one refresh run. The row is made if it is missing, and the claim
-- succeeds only when the last attempt is not newer than the cutoff, so two
-- instances can never both start a run inside the minimum gap. It returns the
-- rows changed: 1 when the run is claimed, 0 when another attempt is too recent.
INSERT INTO official_feed_status (feed, last_attempt_at)
VALUES (sqlc.arg(feed)::text, sqlc.arg(now)::timestamptz)
ON CONFLICT (feed) DO UPDATE
SET last_attempt_at = EXCLUDED.last_attempt_at
WHERE official_feed_status.last_attempt_at IS NULL
   OR official_feed_status.last_attempt_at <= sqlc.arg(cutoff)::timestamptz;

-- name: MarkFeedSuccess :exec
-- The only query that moves last_success_at.
UPDATE official_feed_status
SET last_success_at = sqlc.arg(now)::timestamptz,
    last_error = NULL,
    consecutive_failures = 0,
    last_etag = sqlc.arg(etag)::text,
    last_modified = sqlc.arg(last_modified)::text,
    items_seen = sqlc.arg(items_seen)::int
WHERE feed = sqlc.arg(feed)::text;

-- name: MarkFeedFailure :exec
-- Leaves last_success_at, the validators and items_seen alone.
UPDATE official_feed_status
SET last_error = sqlc.arg(error_class)::text,
    consecutive_failures = consecutive_failures + 1
WHERE feed = sqlc.arg(feed)::text;

-- name: StoredPubDates :many
-- The pubDate each stored series was fetched at, to tell a new or changed item
-- from one already stored.
SELECT guid, source_pubdate
FROM official_alerts
WHERE guid = ANY(sqlc.arg(guids)::text[]);

-- name: OfficialAlertForUpdate :one
-- The stored series of one guid, locked for the rest of the transaction.
SELECT * FROM official_alerts
WHERE guid = sqlc.arg(guid)::text
FOR UPDATE;

-- name: UpsertOfficialAlert :one
-- Stores one version of a series by its guid. A later version replaces the row
-- in place and keeps first_seen_at. A new Alert or Update version clears a
-- cancelled or superseded mark because that version is the series' latest word;
-- a Cancel passes cancelled_at so the row never shows.
INSERT INTO official_alerts (
    guid, cap_identifier, source_pubdate, sender, issuer, msg_type, references_text,
    event, severity, urgency, certainty, sent_at, effective_at, onset_at, expires_at,
    expires_inferred, headline_en, headline_original, original_lang, description_en,
    description_original, instruction, area_desc, placement_tier, cancelled_at,
    first_seen_at, last_seen_at
) VALUES (
    sqlc.arg(guid)::text, sqlc.arg(cap_identifier)::text, sqlc.narg(source_pubdate)::timestamptz,
    sqlc.arg(sender)::text, sqlc.arg(issuer)::text, sqlc.arg(msg_type)::text,
    sqlc.arg(references_text)::text, sqlc.arg(event)::text, sqlc.arg(severity)::text,
    sqlc.arg(urgency)::text, sqlc.arg(certainty)::text, sqlc.arg(sent_at)::timestamptz,
    sqlc.narg(effective_at)::timestamptz, sqlc.narg(onset_at)::timestamptz,
    sqlc.arg(expires_at)::timestamptz, sqlc.arg(expires_inferred)::boolean,
    sqlc.arg(headline_en)::text, sqlc.arg(headline_original)::text, sqlc.arg(original_lang)::text,
    sqlc.arg(description_en)::text, sqlc.arg(description_original)::text,
    sqlc.arg(instruction)::text, sqlc.arg(area_desc)::text, sqlc.arg(placement_tier)::text,
    sqlc.narg(cancelled_at)::timestamptz, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz
)
ON CONFLICT (guid) DO UPDATE
SET cap_identifier = EXCLUDED.cap_identifier,
    source_pubdate = EXCLUDED.source_pubdate,
    sender = EXCLUDED.sender,
    issuer = EXCLUDED.issuer,
    msg_type = EXCLUDED.msg_type,
    references_text = EXCLUDED.references_text,
    event = EXCLUDED.event,
    severity = EXCLUDED.severity,
    urgency = EXCLUDED.urgency,
    certainty = EXCLUDED.certainty,
    sent_at = EXCLUDED.sent_at,
    effective_at = EXCLUDED.effective_at,
    onset_at = EXCLUDED.onset_at,
    expires_at = EXCLUDED.expires_at,
    expires_inferred = EXCLUDED.expires_inferred,
    headline_en = EXCLUDED.headline_en,
    headline_original = EXCLUDED.headline_original,
    original_lang = EXCLUDED.original_lang,
    description_en = EXCLUDED.description_en,
    description_original = EXCLUDED.description_original,
    instruction = EXCLUDED.instruction,
    area_desc = EXCLUDED.area_desc,
    placement_tier = EXCLUDED.placement_tier,
    cancelled_at = EXCLUDED.cancelled_at,
    superseded_at = NULL,
    last_seen_at = EXCLUDED.last_seen_at
RETURNING *;

-- name: DeleteOfficialAlertAreas :exec
DELETE FROM official_alert_areas
WHERE alert_id = sqlc.arg(alert_id)::bigint;

-- name: InsertOfficialAlertArea :exec
INSERT INTO official_alert_areas (alert_id, district_lgd, state_lgd)
VALUES (sqlc.arg(alert_id)::bigint, sqlc.narg(district_lgd)::int, sqlc.narg(state_lgd)::int)
ON CONFLICT DO NOTHING;

-- name: CancelOfficialAlertsByIdentifier :execrows
-- A Cancel names the full identifiers it withdraws. The rows stay; they are
-- marked and never shown. The first mark is kept.
UPDATE official_alerts
SET cancelled_at = COALESCE(cancelled_at, sqlc.arg(now)::timestamptz)
WHERE cap_identifier = ANY(sqlc.arg(identifiers)::text[])
  AND cancelled_at IS NULL;

-- name: SupersedeOfficialAlertsByIdentifier :execrows
-- An Update names the full identifiers it replaces. The series of the update's
-- own guid is replaced in place by the upsert, so it is left out here.
UPDATE official_alerts
SET superseded_at = COALESCE(superseded_at, sqlc.arg(now)::timestamptz)
WHERE cap_identifier = ANY(sqlc.arg(identifiers)::text[])
  AND guid <> sqlc.arg(guid)::text
  AND superseded_at IS NULL;

-- name: OfficialAlertAreas :many
SELECT district_lgd, state_lgd
FROM official_alert_areas
WHERE alert_id = sqlc.arg(alert_id)::bigint
ORDER BY district_lgd NULLS LAST, state_lgd NULLS LAST;

-- name: OfficialAlertsInForce :many
-- Alerts in force at the given time that matter to a reader whose district
-- codes and state codes are passed: a district tier alert that names any of the
-- district codes, a state tier alert that names any of the state codes, and
-- every unplaced alert (those are listed for everyone, never in a banner or a
-- push). Each row carries the alert's own district and state codes.
SELECT a.*,
       COALESCE(array_agg(DISTINCT ar.district_lgd) FILTER (WHERE ar.district_lgd IS NOT NULL), '{}')::int[] AS district_codes,
       COALESCE(array_agg(DISTINCT ar.state_lgd) FILTER (WHERE ar.state_lgd IS NOT NULL), '{}')::int[] AS state_codes
FROM official_alerts a
LEFT JOIN official_alert_areas ar ON ar.alert_id = a.id
WHERE a.expires_at > sqlc.arg(now)::timestamptz
  AND a.cancelled_at IS NULL
  AND a.superseded_at IS NULL
  AND (
        a.placement_tier = 'unplaced'
     OR (a.placement_tier = 'district' AND EXISTS (
            SELECT 1 FROM official_alert_areas d
            WHERE d.alert_id = a.id AND d.district_lgd = ANY(sqlc.arg(district_codes)::int[])))
     OR (a.placement_tier = 'state' AND EXISTS (
            SELECT 1 FROM official_alert_areas s
            WHERE s.alert_id = a.id AND s.state_lgd = ANY(sqlc.arg(state_codes)::int[])))
  )
GROUP BY a.id
ORDER BY a.sent_at DESC, a.id DESC;
