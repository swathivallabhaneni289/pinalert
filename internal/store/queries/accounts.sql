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
SELECT display_name, home_area_name, home_area_cell, terms_version, first_run_done_at
FROM accounts
WHERE id = sqlc.arg(id);
