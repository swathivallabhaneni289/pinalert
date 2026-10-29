-- Phase 9 plan 13: tapped star ratings of a reporter. Raters count by distinct
-- voter cell, one counted rating per cell, as confirmations do (D-21). The cell
-- is never stored on the rating: it is read from the rater's most recent
-- qualifying vote, so a rater who moves cannot keep an old cell.

-- name: RaterMayRate :one
-- Whether the rater has earned the right to rate this reporter (D-23, W4). False
-- for yourself, for a deleted account and for a rater a moderator removed
-- (accounts.rater_removed_at), and for a pair whose rating a moderator removed
-- (the write is refused for good, so the offer would be false; WR-S05).
-- Otherwise true when the rater has cast a confirm,
-- dispute, resolve or reopen vote on one of the reporter's reports. An
-- unresolve alone does not qualify: it is only the take back of one's own
-- resolve and needs no location. The vote's cell is not compared with the
-- report's location, so a witness standing at the scene qualifies like anyone
-- else (orchestrator revision of 2026-10-27). The service adds the checks that
-- need the profile and the block rows (visible reporter, no block).
SELECT (
    sqlc.arg(rater_id)::bigint <> sqlc.arg(reporter_id)::bigint
    AND NOT EXISTS (
        SELECT 1 FROM accounts a
        WHERE a.id = sqlc.arg(rater_id)::bigint
          AND (a.rater_removed_at IS NOT NULL OR a.deleted_at IS NOT NULL))
    AND NOT EXISTS (
        SELECT 1 FROM profile_ratings pr
        WHERE pr.rater_id = sqlc.arg(rater_id)::bigint
          AND pr.reporter_id = sqlc.arg(reporter_id)::bigint
          AND pr.removed_at IS NOT NULL)
    AND EXISTS (
        SELECT 1
        FROM votes v
        JOIN reports r ON r.id = v.report_id
        JOIN sessions s ON s.session_id = r.session_id
        WHERE v.account_id = sqlc.arg(rater_id)::bigint
          AND s.account_id = sqlc.arg(reporter_id)::bigint
          AND v.value IN ('confirm', 'dispute', 'resolve', 'reopen'))
)::boolean AS eligible;

-- name: GetRating :one
-- The rater's own row for one reporter, for the daily rule and for telling a
-- removed rating from a rating that is merely too recent to change (D-02).
SELECT stars, created_at, updated_at, cleared_at, removed_at
FROM profile_ratings
WHERE rater_id = sqlc.arg(rater_id)::bigint
  AND reporter_id = sqlc.arg(reporter_id)::bigint;

-- name: UpsertRating :execrows
-- Writes the rater's rating (D-02). A first rating is free. A later change needs
-- the row's updated_at to be at or before change_cutoff (now minus 24 hours), and
-- a removed row is never written. The same stars again on a live rating is a
-- success that changes nothing: it keeps updated_at, so repeating a tap can
-- neither reset the rater's own 24 hour lock nor make this rating the most
-- recently written one of its cell (which would let a rater take the cell's slot
-- from another rater). Zero rows means too soon, or removed; the service tells
-- them apart with GetRating.
INSERT INTO profile_ratings (rater_id, reporter_id, stars, created_at, updated_at)
VALUES (sqlc.arg(rater_id)::bigint, sqlc.arg(reporter_id)::bigint, sqlc.arg(stars)::smallint,
        sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz)
ON CONFLICT (rater_id, reporter_id) DO UPDATE
SET stars = EXCLUDED.stars,
    updated_at = CASE
        WHEN profile_ratings.cleared_at IS NULL AND profile_ratings.stars = EXCLUDED.stars
            THEN profile_ratings.updated_at
        ELSE EXCLUDED.updated_at
    END,
    cleared_at = NULL
WHERE profile_ratings.removed_at IS NULL
  AND (profile_ratings.updated_at <= sqlc.arg(change_cutoff)::timestamptz
       OR (profile_ratings.cleared_at IS NULL AND profile_ratings.stars = EXCLUDED.stars));

-- name: ClearRating :execrows
-- Remove my rating (D-02). The row stays with cleared_at set, so the daily rule
-- still sees the last change. It follows the same 24 hour rule as a change, and
-- it never touches a removed or an already cleared row. Zero rows means no
-- rating, already cleared, removed, or too soon.
UPDATE profile_ratings
SET cleared_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz
WHERE rater_id = sqlc.arg(rater_id)::bigint
  AND reporter_id = sqlc.arg(reporter_id)::bigint
  AND removed_at IS NULL
  AND cleared_at IS NULL
  AND updated_at <= sqlc.arg(change_cutoff)::timestamptz;

-- name: RatingTalliesForReporters :many
-- The counted tally of each reporter (D-02, D-21, D-22). Three steps, so one
-- person with several accounts in one place counts once:
--   live:   ratings that are not removed and not cleared, whose rater is not
--           deleted and was not removed by a moderator;
--   celled: each live rating takes the cell of the rater's most recent
--           qualifying vote on the reporter's reports, from any cell (the
--           report's own cell included, as for confirmations), so a rating
--           without a qualifying vote cannot count;
--   counted: one rating per (reporter, cell), the most recently written.
-- Then COUNT and SUM per reporter, with the reporter's profile_public flag. The
-- tallies come back whether the profile is on or off, because while it is off
-- the ratings may only lower the points (D-22). Blocks are not read here: a
-- block never deletes or hides a rating (D-14, research Pitfall 10).
WITH live AS (
    SELECT pr.reporter_id, pr.rater_id, pr.stars, pr.updated_at
    FROM profile_ratings pr
    JOIN accounts rater ON rater.id = pr.rater_id
    WHERE pr.reporter_id = ANY(sqlc.arg(account_ids)::bigint[])
      AND pr.removed_at IS NULL
      AND pr.cleared_at IS NULL
      AND rater.deleted_at IS NULL
      AND rater.rater_removed_at IS NULL
), celled AS (
    SELECT DISTINCT ON (l.reporter_id, l.rater_id)
           l.reporter_id, l.rater_id, l.stars, l.updated_at, v.geohash_cell
    FROM live l
    JOIN votes v ON v.account_id = l.rater_id
    JOIN reports r ON r.id = v.report_id
    JOIN sessions s ON s.session_id = r.session_id AND s.account_id = l.reporter_id
    WHERE v.value IN ('confirm', 'dispute', 'resolve', 'reopen')
    ORDER BY l.reporter_id, l.rater_id, v.created_at DESC, v.id DESC
), counted AS (
    SELECT DISTINCT ON (c.reporter_id, c.geohash_cell)
           c.reporter_id, c.stars
    FROM celled c
    ORDER BY c.reporter_id, c.geohash_cell, c.updated_at DESC, c.rater_id DESC
)
SELECT counted.reporter_id,
       COUNT(*)::int AS raters,
       SUM(counted.stars)::int AS tap_sum,
       reporter.profile_public
FROM counted
JOIN accounts reporter ON reporter.id = counted.reporter_id
GROUP BY counted.reporter_id, reporter.profile_public
ORDER BY counted.reporter_id;

-- name: ModeratorRatingsForReporter :many
-- Every rating a reporter holds, for the moderator read (D-14): the rater's
-- username, stars, time, and whether it is removed or cleared. Newest first.
-- Live raters with a username only, because the moderator acts by username.
-- This is the one place rater names are read; the public pages never carry them.
SELECT a.username::text AS rater_username,
       pr.stars,
       pr.updated_at,
       (pr.removed_at IS NOT NULL)::boolean AS removed,
       (pr.cleared_at IS NOT NULL)::boolean AS cleared
FROM profile_ratings pr
JOIN accounts a ON a.id = pr.rater_id
WHERE pr.reporter_id = sqlc.arg(reporter_id)::bigint
  AND a.deleted_at IS NULL
  AND a.username IS NOT NULL
ORDER BY pr.updated_at DESC, pr.rater_id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: RemoveRating :one
-- A moderator removes one rating (D-14, G2). The row stays with removed_at and
-- removed_by, so it is an audit trail and the rater cannot write it back. It
-- returns the removed rating's created_at, which the service uses as the time
-- from which the reporter's tag_lost follows may be restored. It is the first
-- time the rating existed, not the last change, so changing the stars after the
-- follows ended cannot shrink the restore window (WR-S01, review 2026-10-30).
-- No row means there was nothing left to remove.
UPDATE profile_ratings
SET removed_at = sqlc.arg(now)::timestamptz,
    removed_by = sqlc.arg(moderator_id)::bigint
WHERE rater_id = sqlc.arg(rater_id)::bigint
  AND reporter_id = sqlc.arg(reporter_id)::bigint
  AND removed_at IS NULL
RETURNING created_at;

-- name: RemoveRatingsByRater :many
-- A moderator removes every rating one rater gave (D-14, G2, W4). It returns the
-- reporter and the first written time of each removed rating (created_at,
-- WR-S01), so the service can re-evaluate each affected reporter.
UPDATE profile_ratings
SET removed_at = sqlc.arg(now)::timestamptz,
    removed_by = sqlc.arg(moderator_id)::bigint
WHERE rater_id = sqlc.arg(rater_id)::bigint
  AND removed_at IS NULL
RETURNING reporter_id, created_at;

-- name: RemovedRatingsByRater :many
-- Every rating of one rater that a moderator removed, with the time it was first
-- written. A repeat of a removal that failed half way reads this to finish the
-- re-evaluation of each reporter (WR-S02, review 2026-10-30).
SELECT reporter_id, created_at
FROM profile_ratings
WHERE rater_id = sqlc.arg(rater_id)::bigint
  AND removed_at IS NOT NULL
ORDER BY reporter_id;

-- name: MarkRaterRemoved :execrows
-- Stamps the rater as removed (W4): RaterMayRate refuses such an account for
-- good and the tally ignores its ratings. A repeat changes nothing.
UPDATE accounts
SET rater_removed_at = sqlc.arg(now)::timestamptz,
    rater_removed_by = sqlc.arg(moderator_id)::bigint
WHERE id = sqlc.arg(rater_id)::bigint
  AND rater_removed_at IS NULL
  AND deleted_at IS NULL;

-- name: RestoreTagLostFollows :execrows
-- After a moderator removes ratings and the reporter is Reliable again, brings
-- back the follows that ended with reason tag_lost at or after since (D-21,
-- review W1). Only follows ended for that reason come back. The followee must be
-- opted in with a clear username and not deleted, the follower must not be
-- deleted, no block may exist either way, and the follower's active follow count
-- must be below following_cap. Returns the number restored.
UPDATE account_follows f
SET ended_at = NULL,
    end_reason = NULL
FROM accounts followee, accounts follower
WHERE f.followee_id = sqlc.arg(followee_id)::bigint
  AND f.ended_at IS NOT NULL
  AND f.end_reason = 'tag_lost'
  AND f.ended_at >= sqlc.arg(since)::timestamptz
  AND followee.id = f.followee_id
  AND followee.profile_public
  AND followee.deleted_at IS NULL
  AND followee.username IS NOT NULL
  AND followee.username_screen = 'clear'
  AND follower.id = f.follower_id
  AND follower.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM person_blocks b
      WHERE (b.blocker_id = f.follower_id AND b.blocked_id = f.followee_id)
         OR (b.blocker_id = f.followee_id AND b.blocked_id = f.follower_id))
  AND (SELECT COUNT(*) FROM account_follows x
       WHERE x.follower_id = f.follower_id AND x.ended_at IS NULL) < sqlc.arg(following_cap)::bigint;

-- name: DeleteAccountRatings :execrows
-- Delete my account: the ratings the account gave and the ratings it received
-- are hard deleted (research G6, Pitfall 11). Ratings given by a deleted account
-- stop moving other people's stars at once, and a row naming a scrubbed account
-- would point at nobody.
DELETE FROM profile_ratings
WHERE rater_id = sqlc.arg(account_id)::bigint
   OR reporter_id = sqlc.arg(account_id)::bigint;
