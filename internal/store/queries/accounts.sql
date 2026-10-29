-- name: InsertAccount :exec
-- Lazily creates an account only once a magic-link token for the address has
-- actually been consumed (RESEARCH.md Pattern 1) — never at "request link"
-- time. ON CONFLICT DO NOTHING means a returning-visitor's second
-- InsertAccount call is a no-op rather than an error; the row is then read
-- back by GetAccountByEmail, since RETURNING would come back empty on the
-- conflict path that matters most.
INSERT INTO accounts (email) VALUES (sqlc.arg(email))
ON CONFLICT (email) DO NOTHING;

-- name: GetAccountByEmail :one
SELECT id, email, created_at FROM accounts WHERE email = sqlc.arg(email);

-- name: AcceptTerms :exec
-- Records agreement to one version of the terms text (D-27). When the stored
-- version already equals the argument the first agreed time is kept, so a
-- repeat call is harmless; a different or missing version stamps the time now.
UPDATE accounts
SET terms_agreed_at = CASE
        WHEN terms_version IS NOT DISTINCT FROM sqlc.arg(terms_version)::text AND terms_agreed_at IS NOT NULL
            THEN terms_agreed_at
        ELSE now()
    END,
    terms_version = sqlc.arg(terms_version)::text
WHERE id = sqlc.arg(id);

-- name: MarkFirstRunDone :exec
-- Marks the first-time flow finished. COALESCE keeps the first time, so a
-- repeat call changes nothing.
UPDATE accounts
SET first_run_done_at = COALESCE(first_run_done_at, now())
WHERE id = sqlc.arg(id);

-- name: UpdateAccountProfile :exec
-- Saves the optional name and the home area in one statement, so a failure
-- leaves neither changed. set_name and set_home_area say which part to write;
-- a part that is not set keeps its stored value. With set_name a NULL
-- display_name clears the name. With set_home_area the name and the precision
-- 5 cell are written together (accounts_home_area_pair); both NULL clears it.
-- The raw point is never an argument here.
UPDATE accounts
SET display_name   = CASE WHEN sqlc.arg(set_name)::boolean THEN sqlc.narg(display_name)::text ELSE display_name END,
    home_area_name = CASE WHEN sqlc.arg(set_home_area)::boolean THEN sqlc.narg(home_area_name)::text ELSE home_area_name END,
    home_area_cell = CASE WHEN sqlc.arg(set_home_area)::boolean THEN sqlc.narg(home_area_cell)::text ELSE home_area_cell END
WHERE id = sqlc.arg(id);

-- name: GetAccountHomeCell :one
-- The stored home area cell (precision 5, about 5 km), the anchor for the
-- trusted reporter alerts. Read only by server code; it is never written into a
-- page or a response. NULL when no home area is set.
SELECT home_area_cell
FROM accounts
WHERE id = sqlc.arg(id);

-- name: GetAccountProfile :one
-- The caller's own profile rows and onboarding state.
SELECT display_name, home_area_name, home_area_cell, terms_version, first_run_done_at,
       username, username_screen, username_changed_at
FROM accounts
WHERE id = sqlc.arg(id);

-- name: AccountEmailForDelete :one
-- Delete my account step 1: reads the real address and the deleted stamp
-- before anything is scrubbed, because the resend cooldown, the typed code
-- rows and the throttle row of that address are removed by it, and a repeat
-- call must find the account already deleted. Option A, anonymise (owner
-- confirmed 2026-10-18). Amended 2026-10-29 by Phase 7.1 plan 16: the sign in
-- link tokens are gone with their table.
SELECT email, deleted_at
FROM accounts
WHERE id = sqlc.arg(id);

-- name: DeleteEmailCooldownForEmail :exec
-- Delete my account: the resend cooldown of the real address, so the same
-- address can sign up again at once as a brand new account.
DELETE FROM email_cooldowns WHERE email = sqlc.arg(email);

-- name: DeleteAccountUpdates :exec
-- Delete my account: the person's bell rows. They hold only a kind and a report
-- id and are read by nobody else.
DELETE FROM updates WHERE account_id = sqlc.arg(account_id);

-- name: DeleteAccountCursors :exec
-- Delete my account: the bookkeeping behind the person's bell.
DELETE FROM update_cursors WHERE account_id = sqlc.arg(account_id);

-- name: DeleteAccountMutes :exec
-- Delete my account: the person's muted reports.
DELETE FROM report_mutes WHERE account_id = sqlc.arg(account_id);

-- name: DeleteAccountAlertSettings :exec
-- Delete my account: the person's alert switch and distance.
DELETE FROM account_alert_settings WHERE account_id = sqlc.arg(account_id);

-- name: ScrubDeletedAccount :execrows
-- Delete my account, the last step: scrubs the accounts row in place. The row
-- keeps its id, so every foreign key from votes, sessions, report flags and
-- moderation actions stays valid and nobody else's counts move. The email
-- becomes a non deliverable address that keeps the UNIQUE column satisfied, the
-- personal columns are cleared (both pairs together, so the accounts_terms_pair
-- and accounts_home_area_pair checks hold) and deleted_at is stamped. The
-- deleted_at IS NULL guard makes a second call change nothing. Nothing here
-- touches votes, reports or sessions.account_id (Option A, owner confirmed
-- 2026-10-18). Phase 7.1: the password hash is cleared and the username is freed
-- at once, so another account can take it; Phase 8 renders a deleted account's
-- comments without a name. Phase 9 plan 02 (research G6): the profile is
-- switched off too, so a scrubbed account can never be public or followable.
UPDATE accounts
SET email             = sqlc.arg(scrubbed_email),
    profile_public    = false,
    display_name      = NULL,
    password_hash     = NULL,
    username          = NULL,
    username_screen   = NULL,
    username_changed_at = NULL,
    home_area_name    = NULL,
    home_area_cell    = NULL,
    terms_agreed_at   = NULL,
    terms_version     = NULL,
    first_run_done_at = NULL,
    deleted_at        = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;

-- name: GetAccountForSignIn :one
-- Phase 7.1: the password sign in's one read of an account by its normalised
-- address. A deleted account yields no row (its address was scrubbed anyway).
-- password_hash is NULL for an account that has not set a password yet.
-- password_epoch (migration 00016, review CR-S02) is read before the password
-- is checked; the session bind and a rehash then hold to it.
SELECT id, password_hash, (display_name IS NOT NULL)::bool AS has_name, password_epoch
FROM accounts
WHERE email = sqlc.arg(email) AND deleted_at IS NULL;

-- name: LockAccountForPassword :one
-- Inside the confirm transaction of Create account: the account of a
-- normalised address, locked FOR UPDATE, so a parallel sign in bind waits for
-- the new password and its epoch (review CR-S02). No row means no account yet.
SELECT id, password_hash, password_epoch
FROM accounts
WHERE email = sqlc.arg(email) AND deleted_at IS NULL
FOR UPDATE;

-- name: SetAccountPassword :execrows
-- Stores an argon2id PHC string, never a password, without moving the password
-- epoch: the first password of a brand new account inside the Create account
-- transaction, and the store tests. 0 rows means the account is deleted.
UPDATE accounts
SET password_hash = sqlc.arg(password_hash)::text
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetAccountPasswordNewEpoch :one
-- A password set by a typed code on an account that already existed: a reset,
-- and Create account on an account made with the old link or one that gained a
-- password meanwhile (review CR-S02, WR-S02). The epoch moves by one, so every
-- session bound under the old epoch stops passing the gate and a sign in that
-- read the old epoch cannot bind. The UPDATE takes the account row lock for the
-- rest of the transaction. No row means the account is deleted.
UPDATE accounts
SET password_hash = sqlc.arg(password_hash)::text,
    password_epoch = password_epoch + 1
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING password_epoch;

-- name: SetAccountPasswordIfEpoch :execrows
-- The best effort rehash after a cost change, during a good sign in. It never
-- moves the epoch, and it writes only while the epoch is still the one the sign
-- in read, so a rehash of the old password can never overwrite a password a
-- reset set in the meantime (review CR-S02).
UPDATE accounts
SET password_hash = sqlc.arg(password_hash)::text
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND password_epoch = sqlc.arg(password_epoch)::bigint;

-- name: SetAccountUsername :execrows
-- The first set is free, and every later change starts the 7 day wait: the
-- stored username_changed_at moves only when the name really changes, and a
-- change is allowed only when no change has been made yet or the last one is at
-- or before wait_cutoff. Setting the same name again is always allowed. 0 rows
-- means too soon (or the account is deleted); a taken name surfaces as SQLSTATE
-- 23505 from accounts_username_unique.
UPDATE accounts
SET username = sqlc.arg(username)::text,
    username_screen = sqlc.arg(username_screen)::text,
    username_changed_at = CASE
        WHEN username IS NULL OR username = sqlc.arg(username)::text THEN username_changed_at
        ELSE sqlc.arg(now)::timestamptz
    END
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
  AND (username IS NULL
       OR username = sqlc.arg(username)::text
       OR username_changed_at IS NULL
       OR username_changed_at <= sqlc.arg(wait_cutoff)::timestamptz);

-- name: GetAccountIDByUsername :one
-- The id of a not deleted account holding this canonical username, for the
-- @username box and the uniqueness message.
SELECT id FROM accounts WHERE username = sqlc.arg(username)::text AND deleted_at IS NULL;

-- name: UsernamesByAccountIDs :many
-- The batched read Phase 8 comments and Phase 9 profiles consume. Only accounts
-- that have a username and are not deleted come back; callers show a name only
-- when username_screen is clear.
SELECT id, username, username_screen
FROM accounts
WHERE id = ANY(sqlc.arg(ids)::bigint[])
  AND username IS NOT NULL
  AND deleted_at IS NULL;
