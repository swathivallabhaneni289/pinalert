-- name: IssueAuthCode :one
-- Phase 7.1 (D-02, D-05): writes the one live code row of a request. The CTE
-- first marks every unused row of the same session and purpose as used, then
-- the new row is inserted, so there is one live code per session and purpose
-- and resending never multiplies the try budget. The address lives in the row,
-- never in a URL. The code is stored only as a keyed hash, and the password
-- typed at Create account (already hashed) rides along until the code is
-- confirmed. delivery says which email this row stands for.
WITH retired AS (
    UPDATE auth_codes
    SET used_at = now()
    WHERE session_id = sqlc.arg(session_id)
      AND purpose = sqlc.arg(purpose)
      AND used_at IS NULL
)
INSERT INTO auth_codes (session_id, purpose, delivery, email, code_hash, password_hash, expires_at)
VALUES (
    sqlc.arg(session_id), sqlc.arg(purpose), sqlc.arg(delivery), sqlc.arg(email),
    sqlc.arg(code_hash), sqlc.narg(password_hash), sqlc.arg(expires_at)
)
RETURNING id;

-- name: BumpAuthCodeAttempt :one
-- Every guess, the right one included, is counted before the Go side compares
-- (D-02): one atomic UPDATE of the newest unused row of this session and
-- purpose, guarded by used_at IS NULL, expires_at > now() and attempts below
-- the budget, so parallel guessing cannot beat it. No rows (pgx.ErrNoRows)
-- means no live row or the budget is spent: one answer for wrong, expired, used
-- and spent.
UPDATE auth_codes
SET attempts = attempts + 1
WHERE id = (SELECT c.id FROM auth_codes c
            WHERE c.session_id = sqlc.arg(session_id)
              AND c.purpose = sqlc.arg(purpose)
              AND c.used_at IS NULL
            ORDER BY c.id DESC
            LIMIT 1)
  AND used_at IS NULL
  AND expires_at > now()
  AND attempts < sqlc.arg(max_attempts)::int
RETURNING id, delivery, email, code_hash, password_hash, attempts;

-- name: ConsumeAuthCode :execrows
-- Spends a code exactly once (the ConsumeToken rule). 1 row means this request
-- won; 0 means a parallel submit did, or the code expired in between.
-- Phase 7.1 review fix round 3 (skeptic SK2-01): a spent code's life ends when
-- it is spent, so expires_at is set to the same now() as used_at. That equal
-- pair is written by this statement only (a retired row keeps its later expiry),
-- and it is how the sign in page tells a browser whose confirm answer was lost
-- that its own code was confirmed. The password hash typed at Create account has
-- done its job and is cleared, so the kept row holds none.
UPDATE auth_codes
SET used_at = now(), expires_at = now(), password_hash = NULL
WHERE id = sqlc.arg(id)
  AND used_at IS NULL
  AND expires_at > now();

-- name: MarkAuthCodePasswordSet :exec
-- Phase 7.1 review fix round 4 (skeptic SK3-03): a Create account confirm that
-- only set a password on an account the old emailed link had made did not create
-- anything, so the kept row is marked as a password set (purpose reset) and the
-- sign in page after a lost answer says "Password set", not "Account created".
-- Only the confirm writes it, on the row it just spent; the row holds no hash,
-- so the purpose constraints still hold. The page never asks whether an
-- address has an account; it reads this browser's own row.
UPDATE auth_codes
SET purpose = 'reset'
WHERE id = sqlc.arg(id)
  AND purpose = 'signup'
  AND used_at IS NOT NULL
  AND password_hash IS NULL;

-- name: MarkAuthCodeVerified :execrows
-- Quick 261113-r5w, step one of Forgot password: this browser typed the right
-- code. Only a live reset row that was emailed a code can be marked, and only
-- while it is unused and unexpired, so 0 rows means the row cannot be verified
-- (the service answers ErrWrongCode). verified_at is the service clock's moment
-- of the first right code and is never moved by a second one (COALESCE), so a
-- right code typed twice, or by two parallel requests, is still one verify.
UPDATE auth_codes
SET verified_at = COALESCE(verified_at, sqlc.arg(verified_at)::timestamptz)
WHERE id = sqlc.arg(id)
  AND used_at IS NULL
  AND expires_at > now()
  AND purpose = 'reset'
  AND delivery = 'code';

-- name: ConsumeVerifiedResetCode :execrows
-- Quick 261113-r5w, step two of Forgot password: spends a verified reset code
-- exactly once. It is ConsumeAuthCode for a row whose right code was typed
-- earlier: the confirm window is the code lifetime counted from verified_at
-- (verified_after is that moment minus the lifetime), not from the send, so a
-- person who typed the code just before it would have expired still has the
-- whole window to choose a password. A newer code retires the row (used_at set),
-- so it no longer matches. The rest is ConsumeAuthCode's write: used_at and
-- expires_at set to the same now() (SpentCode's pair), the hash cleared. 0 rows
-- means a parallel call won, the confirm ran out or a newer code replaced it.
UPDATE auth_codes
SET used_at = now(), expires_at = now(), password_hash = NULL
WHERE id = sqlc.arg(id)
  AND used_at IS NULL
  AND purpose = 'reset'
  AND delivery = 'code'
  AND verified_at IS NOT NULL
  AND verified_at > sqlc.arg(verified_after)::timestamptz;

-- name: RetireAuthCode :exec
-- Marks a row used when its email did not go out (the ceiling refused it or
-- the provider failed), so a reload of the sign in page does not show a code
-- screen for an email that was never sent (Phase 7.1 review IN-S01).
UPDATE auth_codes
SET used_at = now()
WHERE id = sqlc.arg(id)
  AND used_at IS NULL;

-- name: GetLatestAuthCode :one
-- The newest row of this session, whatever its purpose, created after since.
-- Read by Resend and by the sign in page to show the code screen again after a
-- reload. The address goes into page text only, never into a URL.
SELECT id, purpose, delivery, email, password_hash, created_at, expires_at, attempts, used_at, verified_at
FROM auth_codes
WHERE session_id = sqlc.arg(session_id)
  AND created_at > sqlc.arg(since)::timestamptz
ORDER BY id DESC
LIMIT 1;

-- name: DeleteAuthCodesForEmail :exec
-- Delete my account: the code rows ever written for the real address.
DELETE FROM auth_codes WHERE email = sqlc.arg(email);

-- name: DeleteOtherAuthCodesForEmail :exec
-- A confirm: every other code row of the address goes, so a code still pending
-- in another browser can no longer be used (W9). The row just spent is kept,
-- used and with no password hash, so the browser that spent it can be told so
-- after a lost answer (Phase 7.1 review fix round 3, skeptic SK2-01). The prune
-- runs when a code is sent and removes it then, once a day or more has passed
-- since the spend (round 4, SK3-01); on a quiet site it can stay longer. Delete
-- my account removes it at once.
DELETE FROM auth_codes WHERE email = sqlc.arg(email) AND id <> sqlc.arg(keep_id);

-- name: LockCredentialAddress :exec
-- The first statement of both confirm transactions, Create account's typed
-- code and Forgot password's (Phase 7.1 review fix skeptic SK-05): one
-- transaction scoped advisory lock per normalised address, keyed by its keyed
-- throttle hash, so two confirms for one address run one after the other. Each
-- takes the account row and the address's code rows, in different orders; under
-- this lock they can no longer deadlock, and the lock also covers a brand new
-- account, which has no row yet to lock. The class 71 keeps these keys apart
-- from any other advisory lock. It ends with the transaction.
SELECT pg_advisory_xact_lock(71, hashtext(sqlc.arg(address_key)::text));

-- name: PruneAuthCodes :execrows
-- Removes rows that expired before the cutoff, so the table holds only codes
-- that could still matter.
DELETE FROM auth_codes WHERE expires_at < sqlc.arg(before)::timestamptz;
