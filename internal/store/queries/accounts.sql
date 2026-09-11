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
