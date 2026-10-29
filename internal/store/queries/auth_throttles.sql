-- name: GetAuthThrottle :one
-- The stored sign in failures of one address (as a keyed hash). No row means
-- the address has no failures on record.
SELECT failures, last_failure_at, locked_until
FROM auth_throttles
WHERE scope = sqlc.arg(scope) AND key_hash = sqlc.arg(key_hash);

-- name: RecordAuthFailure :one
-- Counts one failed sign in in the database, so the throttle survives a free
-- host sleep (D-02, IDENT-04 as amended by IDENT-06). The count is forgotten
-- (restarts at 1) only when both the last failure and the end of the last lock
-- are at or before forget_cutoff; a failure made the moment a lock ends keeps
-- counting, so a 40 minute lock followed by a failure is failure 10 and locks
-- for the 60 minute cap, never a reset to 1. Once the failures pass
-- free_failures the lock is base_lock_seconds doubled for each failure beyond
-- the first locked one, capped at max_lock_seconds: with 5 free, 300 s base and
-- 3600 s cap the waits are 5, 10, 20, 40, 60, 60 minutes. The service refuses a
-- locked attempt before this runs, so a refused attempt never extends the wait.
INSERT INTO auth_throttles AS t (scope, key_hash, failures, last_failure_at, locked_until)
VALUES (
    sqlc.arg(scope), sqlc.arg(key_hash), 1, sqlc.arg(now)::timestamptz,
    CASE WHEN 1 > sqlc.arg(free_failures)::int
         THEN sqlc.arg(now)::timestamptz
              + make_interval(secs => LEAST(sqlc.arg(max_lock_seconds)::int::float8, sqlc.arg(base_lock_seconds)::int::float8))
         ELSE NULL END
)
ON CONFLICT (scope, key_hash) DO UPDATE
SET failures = CASE
        WHEN t.last_failure_at <= sqlc.arg(forget_cutoff)::timestamptz
         AND (t.locked_until IS NULL OR t.locked_until <= sqlc.arg(forget_cutoff)::timestamptz)
            THEN 1
        ELSE t.failures + 1
    END,
    last_failure_at = sqlc.arg(now)::timestamptz,
    locked_until = CASE
        WHEN (CASE
                WHEN t.last_failure_at <= sqlc.arg(forget_cutoff)::timestamptz
                 AND (t.locked_until IS NULL OR t.locked_until <= sqlc.arg(forget_cutoff)::timestamptz)
                    THEN 1
                ELSE t.failures + 1
              END) > sqlc.arg(free_failures)::int
            THEN sqlc.arg(now)::timestamptz
                 + make_interval(secs => LEAST(
                       sqlc.arg(max_lock_seconds)::int::float8,
                       sqlc.arg(base_lock_seconds)::int::float8 * power(2::float8, LEAST(30, (CASE
                           WHEN t.last_failure_at <= sqlc.arg(forget_cutoff)::timestamptz
                            AND (t.locked_until IS NULL OR t.locked_until <= sqlc.arg(forget_cutoff)::timestamptz)
                               THEN 1
                           ELSE t.failures + 1
                         END) - sqlc.arg(free_failures)::int - 1)::float8)))
        ELSE NULL
    END
RETURNING failures, locked_until;

-- name: ClearAuthThrottle :exec
-- Forgets an address: after a successful sign in, after a password reset, and
-- on Delete my account.
DELETE FROM auth_throttles WHERE scope = sqlc.arg(scope) AND key_hash = sqlc.arg(key_hash);

-- name: PruneAuthThrottles :execrows
-- Called beside PruneAuthCodes with the forget cutoff, so the table holds only
-- rows that still count: a row goes when its last failure and the end of its
-- lock are both at or before the cutoff.
DELETE FROM auth_throttles
WHERE last_failure_at <= sqlc.arg(cutoff)::timestamptz
  AND (locked_until IS NULL OR locked_until <= sqlc.arg(cutoff)::timestamptz);
