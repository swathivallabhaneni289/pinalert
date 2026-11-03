-- Phase 9 plan 18: person mute (D-12, research G4). A mute stops person level
-- notes about the muted person and nothing else: no query here, and none that
-- reads these rows, may remove a report from any feed, map, list or profile.
-- These rows are separate from report_mutes (one report, Phase 3.2).

-- name: MutePerson :exec
-- Mute a person (D-12). It writes the row only; a repeat is a no op. A mute of
-- yourself fails with SQLSTATE 23514. Who may be muted (a visible profile) is
-- the service's rule.
INSERT INTO person_mutes (muter_id, muted_id, created_at)
VALUES (sqlc.arg(muter_id)::bigint, sqlc.arg(muted_id)::bigint, sqlc.arg(now)::timestamptz)
ON CONFLICT (muter_id, muted_id) DO NOTHING;

-- name: UnmutePerson :execrows
-- Take a mute back (D-12). The muter's own row only. Zero rows means there was
-- none, which the service treats as success.
DELETE FROM person_mutes
WHERE muter_id = sqlc.arg(muter_id)::bigint
  AND muted_id = sqlc.arg(muted_id)::bigint;

-- name: PersonMuted :one
-- Whether the muter has muted this person (D-12). For the muter's own view of a
-- profile only; the muted person can never read it.
SELECT EXISTS (
    SELECT 1 FROM person_mutes m
    WHERE m.muter_id = sqlc.arg(muter_id)::bigint
      AND m.muted_id = sqlc.arg(muted_id)::bigint
)::bool AS muted;

-- name: MutedPeople :many
-- The people the account has muted, newest first (D-12). Live accounts with a
-- username only, whether or not their profile is on, so a person who went
-- private since can still be unmuted. username_screen is returned so the caller
-- shows a name only when it is clear (07.1 D-16).
SELECT a.username, a.username_screen, m.created_at
FROM person_mutes m
JOIN accounts a ON a.id = m.muted_id
WHERE m.muter_id = sqlc.arg(muter_id)::bigint
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
ORDER BY m.created_at DESC, a.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountMutedPeople :one
-- How many people the account has muted (live accounts with a username only, as
-- MutedPeople), for the Settings row (D-12).
SELECT COUNT(*)::bigint
FROM person_mutes m
JOIN accounts a ON a.id = m.muted_id
WHERE m.muter_id = sqlc.arg(muter_id)::bigint
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL;

-- name: MutedPeopleIDs :many
-- The account ids the reader has muted, for the person level notes to skip
-- (D-12). Read once per refresh. Every row counts, with no limit and no join,
-- so a mute is never missed because of a list cap or a name state.
SELECT muted_id
FROM person_mutes
WHERE muter_id = sqlc.arg(muter_id)::bigint;

-- name: BlockedEitherWayIDs :many
-- The account ids with a block between them and the reader in either direction
-- (the reader blocked them, or they blocked the reader), for the Reliable
-- reporter bell row to skip (quick 261103-jal, follow up F1). It matches the
-- worker's personNotMutedOrBlocked rule. A block never hides a report on a map,
-- list or feed; it only keeps this person level note away. Read once per
-- refresh, with no limit and no join.
SELECT blocked_id AS other_id
FROM person_blocks
WHERE blocker_id = sqlc.arg(account_id)::bigint
UNION
SELECT blocker_id AS other_id
FROM person_blocks
WHERE blocked_id = sqlc.arg(account_id)::bigint;

-- name: DeleteAccountPersonMutes :execrows
-- Delete my account: the mutes the person made and the mutes of them, so no
-- mute row survives pointing at a scrubbed account (D-12, research G6).
DELETE FROM person_mutes
WHERE muter_id = sqlc.arg(account_id)::bigint
   OR muted_id = sqlc.arg(account_id)::bigint;
