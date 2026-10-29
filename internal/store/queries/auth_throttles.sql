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
-- 3600 s cap the waits are 5, 10, 20, 40, 60, 60 minutes. The sign in path
-- counts its tries with ReserveSignInTry below, the same arithmetic with the
-- lock guard on it (Phase 7.1 review CR-S01); this unguarded form is kept for
-- the store tests and for seeding a throttle row.
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

-- name: ReserveSignInTry :one
-- Counts one sign in try BEFORE the password is checked (Phase 7.1 review
-- CR-S01), the way BumpAuthCodeAttempt counts a code try before the compare.
-- It is RecordAuthFailure with one more guard on the DO UPDATE: an address
-- that is locked right now gets no update and no row back (pgx.ErrNoRows), so
-- a refused try never extends the lock. The row lock of the upsert serialises
-- a parallel burst for one address, so the burst gets exactly free_failures + 1
-- password checks before the lock is in place. A right password then clears
-- the row with ClearAuthThrottle. Every address, known or unknown, takes this
-- same statement, so the count never tells whether an account exists.
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
WHERE t.locked_until IS NULL OR t.locked_until <= sqlc.arg(now)::timestamptz
RETURNING failures, locked_until, last_failure_at;

-- name: RefundSignInTry :execrows
-- Gives back one try ReserveSignInTry counted when the password was never
-- checked because the hasher was busy (Phase 7.1 review fix skeptic SK-02). It
-- is guarded on the row the reserve returned: it changes the row only while its
-- failures and last_failure_at are still that try's, so it never undoes a later
-- try. failures goes down by one and the lock goes. Clearing the lock is the
-- state before the try: the reserve only updates a row whose lock had already
-- ended (or never existed), and an ended lock and no lock read the same to the
-- reserve's guard. The forget rule also reads the same, because last_failure_at
-- is kept at this try's time, which is never before an ended lock. Every
-- address, known or unknown, takes this same statement.
UPDATE auth_throttles
SET failures = failures - 1,
    locked_until = NULL
WHERE scope = sqlc.arg(scope) AND key_hash = sqlc.arg(key_hash)
  AND failures = sqlc.arg(failures)
  AND failures > 0
  AND last_failure_at = sqlc.arg(last_failure_at)::timestamptz;

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
