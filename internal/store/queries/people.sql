-- Phase 9 plan 02: the core people queries. Every read of current follows
-- filters ended_at IS NULL (D-21, T-09-95); TestFollowReadsFilterEnded scans
-- this directory and fails on a query that touches the follows table without
-- naming that column. The only exception is the hard delete with an account.

-- name: GetPersonByUsername :one
-- A person by the canonical (lowercase) username, for the profile page. The
-- service normalises the typed name first. A deleted account returns no row.
-- Callers show a name only when username_screen is clear (07.1 D-16), and show
-- the profile only when profile_public is true (D-01).
SELECT id, username, username_screen, display_name, home_area_name, created_at, profile_public
FROM accounts
WHERE username = sqlc.arg(username)::text
  AND deleted_at IS NULL;

-- name: GetPersonByID :one
-- The same person columns by id. A deleted account returns no row.
SELECT id, username, username_screen, display_name, home_area_name, created_at, profile_public
FROM accounts
WHERE id = sqlc.arg(id)::bigint
  AND deleted_at IS NULL;

-- name: SetProfilePublic :execrows
-- "Let others follow me" (D-01). profile_public_at is the time the profile was
-- last switched on: it moves only on a real off to on change, and a switch off
-- leaves it (the accounts_profile_public_at_set check needs it set only while
-- the profile is public). 0 rows means the account is deleted.
UPDATE accounts
SET profile_public = sqlc.arg(is_public)::boolean,
    profile_public_at = CASE
        WHEN sqlc.arg(is_public)::boolean AND NOT profile_public THEN sqlc.arg(now)::timestamptz
        ELSE profile_public_at
    END
WHERE id = sqlc.arg(id)::bigint
  AND deleted_at IS NULL;

-- name: SetProfilePublicOn :execrows
-- Switches the profile on only while the stored Name is still the Name that was
-- screened (WR-S04, review 2026-10-30). The Name screen is a slow network call;
-- a Name edit that commits while it runs would otherwise go public unscreened.
-- 0 rows means the account is deleted or the Name changed since the screen.
-- profile_public_at moves only on a real off to on change, as in
-- SetProfilePublic.
UPDATE accounts
SET profile_public = true,
    profile_public_at = CASE
        WHEN NOT profile_public THEN sqlc.arg(now)::timestamptz
        ELSE profile_public_at
    END
WHERE id = sqlc.arg(id)::bigint
  AND deleted_at IS NULL
  AND display_name = sqlc.arg(screened_name)::text;

-- name: MarkUsernameScreenClear :execrows
-- The result of a re-screen of a pending name (07.1 D-16). Race safe: it writes
-- only while the stored name still equals the screened name and the state is
-- still pending, so a rename between the screen and the write changes nothing.
UPDATE accounts
SET username_screen = 'clear'
WHERE id = sqlc.arg(id)::bigint
  AND username = sqlc.arg(username)::text
  AND username_screen = 'pending'
  AND deleted_at IS NULL;

-- name: FollowAccount :execrows
-- Follow a person (D-01, D-12, D-21). One statement checks every precondition:
-- the followee is opted in, not deleted, has a username whose screen is clear,
-- and no block exists between the two in either direction. A new pair inserts a
-- row; an ended pair is reactivated with a fresh created_at and no end. Zero
-- rows means already following or refused; the service tells them apart with
-- PersonRelation. A follow of yourself fails with SQLSTATE 23514.
INSERT INTO account_follows (follower_id, followee_id, created_at)
SELECT sqlc.arg(follower_id)::bigint, a.id, sqlc.arg(now)::timestamptz
FROM accounts a
WHERE a.id = sqlc.arg(followee_id)::bigint
  AND a.profile_public
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
  AND a.username_screen = 'clear'
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE (b.blocker_id = sqlc.arg(follower_id)::bigint AND b.blocked_id = a.id)
         OR (b.blocker_id = a.id AND b.blocked_id = sqlc.arg(follower_id)::bigint))
ON CONFLICT (follower_id, followee_id) DO UPDATE
SET created_at = EXCLUDED.created_at,
    ended_at = NULL,
    end_reason = NULL,
    notify = false,
    notify_since = NULL
WHERE account_follows.ended_at IS NOT NULL;

-- name: EndFollowPair :execrows
-- Ends one active follow with a reason (D-21). The row stays. Services call it
-- only through endFollowPair (followend.go), never directly (review W1). Ending
-- a follow also turns its Notify me off (D-08, D-21), so a follow restored
-- later by the sweep, or followed again, comes back with the bell off.
UPDATE account_follows
SET ended_at = sqlc.arg(now)::timestamptz,
    end_reason = sqlc.arg(reason)::text,
    notify = false,
    notify_since = NULL
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND followee_id = sqlc.arg(followee_id)::bigint
  AND ended_at IS NULL;

-- name: EndFollowsOf :execrows
-- Ends every active follow of one followee with a reason (D-21), returning the
-- number of rows ended. Services call it only through endFollowsOf. Every
-- ended follow also has its Notify me turned off (D-08, D-21).
UPDATE account_follows
SET ended_at = sqlc.arg(now)::timestamptz,
    end_reason = sqlc.arg(reason)::text,
    notify = false,
    notify_since = NULL
WHERE followee_id = sqlc.arg(followee_id)::bigint
  AND ended_at IS NULL;

-- name: SetFollowNotify :execrows
-- Turns Notify me on or off for one active follow (D-08). Turning it on keeps
-- an existing notify_since, so a repeat changes nothing and a note is only for
-- reports posted after the first time it was turned on; turning it off clears
-- the time. Zero rows means there is no active follow (D-21). The service
-- checks the relation, the mute and the cap first.
UPDATE account_follows
SET notify = sqlc.arg(notify)::boolean,
    notify_since = CASE
        WHEN sqlc.arg(notify)::boolean THEN COALESCE(notify_since, sqlc.arg(now)::timestamptz)
        ELSE NULL
    END
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND followee_id = sqlc.arg(followee_id)::bigint
  AND ended_at IS NULL;

-- name: TurnOffFollowNotify :execrows
-- Turns Notify me off for one active follow, whatever it was (D-12: a mute
-- turns the bell off). Zero rows means it was already off or there is no active
-- follow.
UPDATE account_follows
SET notify = false,
    notify_since = NULL
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND followee_id = sqlc.arg(followee_id)::bigint
  AND ended_at IS NULL
  AND notify;

-- name: CountNotifyFollows :one
-- How many Notify me bells the account has on now (active follows only), for
-- the cap of NotifyFollowsCap.
SELECT COUNT(*)::bigint
FROM account_follows
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND ended_at IS NULL
  AND notify;

-- name: NotifyFollowsFor :many
-- The people one account has the bell on for, with the time it was turned on
-- (D-08, D-21: active follows only). The producer of follow notes reads it and
-- keeps only reports posted after notify_since. Limited by the cap; ordered so
-- the answer is stable.
SELECT followee_id, notify_since::timestamptz AS notify_since
FROM account_follows
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND ended_at IS NULL
  AND notify
ORDER BY notify_since, followee_id
LIMIT sqlc.arg(row_limit)::int;

-- name: NotifyFollowersOf :many
-- The followers of one person who have the bell on (D-21: active follows
-- only), for the fan out at posting time. Limited; ordered by id so the answer
-- is stable. It names no username and no account detail.
SELECT follower_id
FROM account_follows
WHERE followee_id = sqlc.arg(followee_id)::bigint
  AND ended_at IS NULL
  AND notify
ORDER BY follower_id
LIMIT sqlc.arg(row_limit)::int;

-- name: ActiveFollowNotify :one
-- Whether an active follow with the bell on exists from the follower to the
-- followee (D-21). The push worker reads it at send time, and a profile reads
-- it for the viewer's own bell.
SELECT EXISTS (
    SELECT 1 FROM account_follows
    WHERE follower_id = sqlc.arg(follower_id)::bigint
      AND followee_id = sqlc.arg(followee_id)::bigint
      AND ended_at IS NULL
      AND notify)::bool AS notify_on;

-- name: CountFollowing :one
-- How many people the account follows now (active rows only).
SELECT COUNT(*)::bigint
FROM account_follows
WHERE follower_id = sqlc.arg(follower_id)::bigint
  AND ended_at IS NULL;

-- name: PersonRelation :one
-- The four facts between a viewer and another person (D-12): who follows whom
-- now (active rows only) and who has blocked whom.
SELECT
    EXISTS (SELECT 1 FROM account_follows f
            WHERE f.follower_id = sqlc.arg(viewer_id)::bigint
              AND f.followee_id = sqlc.arg(other_id)::bigint
              AND f.ended_at IS NULL) AS viewer_follows,
    EXISTS (SELECT 1 FROM account_follows f
            WHERE f.follower_id = sqlc.arg(other_id)::bigint
              AND f.followee_id = sqlc.arg(viewer_id)::bigint
              AND f.ended_at IS NULL) AS follows_viewer,
    EXISTS (SELECT 1 FROM person_blocks b
            WHERE b.blocker_id = sqlc.arg(viewer_id)::bigint
              AND b.blocked_id = sqlc.arg(other_id)::bigint) AS viewer_blocked,
    EXISTS (SELECT 1 FROM person_blocks b
            WHERE b.blocker_id = sqlc.arg(other_id)::bigint
              AND b.blocked_id = sqlc.arg(viewer_id)::bigint) AS blocked_viewer;

-- name: FollowCounts :one
-- The follower and following totals of a person, counting everyone with an
-- active follow (D-03: the counts include people shown only as a number).
SELECT
    (SELECT COUNT(*) FROM account_follows
     WHERE followee_id = sqlc.arg(account_id)::bigint AND ended_at IS NULL)::bigint AS followers,
    (SELECT COUNT(*) FROM account_follows
     WHERE follower_id = sqlc.arg(account_id)::bigint AND ended_at IS NULL)::bigint AS following;

-- name: FollowersPage :many
-- The named followers of a person, newest first (D-03). Only active follows of
-- people who are opted in, not deleted, with a clear username and no block in
-- either direction with the viewer are named; everyone else is only counted.
-- Keyset on (created_at, username), both descending; usernames are unique, so
-- the order is total.
SELECT a.username::text AS username, f.created_at
FROM account_follows f
JOIN accounts a ON a.id = f.follower_id
WHERE f.followee_id = sqlc.arg(account_id)::bigint
  AND f.ended_at IS NULL
  AND a.profile_public
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
  AND a.username_screen = 'clear'
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE (b.blocker_id = sqlc.arg(viewer_id)::bigint AND b.blocked_id = a.id)
         OR (b.blocker_id = a.id AND b.blocked_id = sqlc.arg(viewer_id)::bigint))
  AND (sqlc.narg(after_at)::timestamptz IS NULL
       OR (f.created_at, a.username) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_username)::text))
ORDER BY f.created_at DESC, a.username DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: FollowingPage :many
-- The named people a person follows, newest first (D-03). The same rules and
-- keyset as FollowersPage, with the other side being the followee.
SELECT a.username::text AS username, f.created_at
FROM account_follows f
JOIN accounts a ON a.id = f.followee_id
WHERE f.follower_id = sqlc.arg(account_id)::bigint
  AND f.ended_at IS NULL
  AND a.profile_public
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
  AND a.username_screen = 'clear'
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE (b.blocker_id = sqlc.arg(viewer_id)::bigint AND b.blocked_id = a.id)
         OR (b.blocker_id = a.id AND b.blocked_id = sqlc.arg(viewer_id)::bigint))
  AND (sqlc.narg(after_at)::timestamptz IS NULL
       OR (f.created_at, a.username) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_username)::text))
ORDER BY f.created_at DESC, a.username DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountNamedFollowers :one
-- How many followers FollowersPage can name for this viewer (D-03); the service
-- shows "and N others" as the total minus this count.
SELECT COUNT(*)::bigint
FROM account_follows f
JOIN accounts a ON a.id = f.follower_id
WHERE f.followee_id = sqlc.arg(account_id)::bigint
  AND f.ended_at IS NULL
  AND a.profile_public
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
  AND a.username_screen = 'clear'
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE (b.blocker_id = sqlc.arg(viewer_id)::bigint AND b.blocked_id = a.id)
         OR (b.blocker_id = a.id AND b.blocked_id = sqlc.arg(viewer_id)::bigint));

-- name: CountNamedFollowing :one
-- How many followed people FollowingPage can name for this viewer (D-03).
SELECT COUNT(*)::bigint
FROM account_follows f
JOIN accounts a ON a.id = f.followee_id
WHERE f.follower_id = sqlc.arg(account_id)::bigint
  AND f.ended_at IS NULL
  AND a.profile_public
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
  AND a.username_screen = 'clear'
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE (b.blocker_id = sqlc.arg(viewer_id)::bigint AND b.blocked_id = a.id)
         OR (b.blocker_id = a.id AND b.blocked_id = sqlc.arg(viewer_id)::bigint));

-- name: BlockAccount :exec
-- Block a person (D-12). It writes the block row only; a repeat is a no op.
-- Ending the follows between the two is the service's job, through endFollowPair
-- with reason blocked, so every follow end passes through the two helpers
-- (review W1). That is safe in either order because FollowAccount refuses once
-- the block row exists. A block of yourself fails with SQLSTATE 23514.
INSERT INTO person_blocks (blocker_id, blocked_id, created_at)
VALUES (sqlc.arg(blocker_id)::bigint, sqlc.arg(blocked_id)::bigint, sqlc.arg(now)::timestamptz)
ON CONFLICT (blocker_id, blocked_id) DO NOTHING;

-- name: UnblockAccount :execrows
-- Take a block back (D-12). Follows that the block ended stay ended.
DELETE FROM person_blocks
WHERE blocker_id = sqlc.arg(blocker_id)::bigint
  AND blocked_id = sqlc.arg(blocked_id)::bigint;

-- name: BlockedPeople :many
-- The people the account has blocked, newest first (D-12). Live accounts only.
-- username_screen is returned so the caller shows a name only when it is clear
-- (07.1 D-16).
SELECT a.username, a.username_screen, b.created_at
FROM person_blocks b
JOIN accounts a ON a.id = b.blocked_id
WHERE b.blocker_id = sqlc.arg(blocker_id)::bigint
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
ORDER BY b.created_at DESC, a.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: CountBlocked :one
-- How many people the account has blocked (live accounts only, as BlockedPeople).
SELECT COUNT(*)::bigint
FROM person_blocks b
JOIN accounts a ON a.id = b.blocked_id
WHERE b.blocker_id = sqlc.arg(blocker_id)::bigint
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL;

-- name: PublicProfilePathsForViewer :many
-- Which of these accounts have a public profile the viewer may open (D-01,
-- D-12): opted in, not deleted, a clear username, and not blocking the viewer.
-- A batch read for pages that link a name to its profile.
SELECT a.id, a.username::text AS username
FROM accounts a
WHERE a.id = ANY(sqlc.arg(account_ids)::bigint[])
  AND a.profile_public
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
  AND a.username_screen = 'clear'
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE b.blocker_id = a.id AND b.blocked_id = sqlc.arg(viewer_id)::bigint);

-- name: FolloweeIDsPage :many
-- The distinct people who have at least one active follower, ascending by id,
-- for the sweep to page through (D-21). after_id is the last id of the page
-- before; 0 starts from the beginning.
SELECT DISTINCT followee_id
FROM account_follows
WHERE ended_at IS NULL
  AND followee_id > sqlc.arg(after_id)::bigint
ORDER BY followee_id
LIMIT sqlc.arg(row_limit)::int;

-- name: FolloweeStanding :many
-- Whether each account is still followable: opted in, a clear username and not
-- deleted (D-01). The sweep ends the follows of anyone who is not (reason
-- profile_off).
SELECT a.id,
       COALESCE(a.profile_public AND a.username IS NOT NULL AND a.username_screen = 'clear'
                AND a.deleted_at IS NULL, false)::bool AS followable
FROM accounts a
WHERE a.id = ANY(sqlc.arg(account_ids)::bigint[]);

-- name: DeleteAccountFollows :execrows
-- Delete my account: the person's follows in both directions, active and ended,
-- are hard deleted (research G6). A deleted person has no public profile, and a
-- row naming them in someone else's lists would point at nobody. This is the
-- one place a follow is deleted rather than ended.
DELETE FROM account_follows
WHERE follower_id = sqlc.arg(account_id)::bigint
   OR followee_id = sqlc.arg(account_id)::bigint;

-- name: DeleteAccountBlocks :execrows
-- Delete my account: the blocks the person made and the blocks against them, so
-- no block row survives pointing at a scrubbed account.
DELETE FROM person_blocks
WHERE blocker_id = sqlc.arg(account_id)::bigint
   OR blocked_id = sqlc.arg(account_id)::bigint;
