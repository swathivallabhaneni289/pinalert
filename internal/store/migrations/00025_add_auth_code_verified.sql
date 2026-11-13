-- +goose Up
-- Quick 261113-r5w: Forgot password in two steps, the code first, then the new
-- password. A reset code row gains verified_at: the moment this browser typed
-- the right code at step one. Step two (setting the password) needs a row that
-- is verified and still inside the code lifetime after that moment, so the
-- password can never be set by a request that skipped the code.
--
-- Only a reset row that was emailed a code can ever be verified: a sign up row,
-- an account_exists row and a no_account row keep verified_at NULL, which the
-- CHECK enforces, so a no_account row can never open step two.
ALTER TABLE auth_codes
    ADD COLUMN verified_at TIMESTAMPTZ;

ALTER TABLE auth_codes
    ADD CONSTRAINT auth_codes_verified_shape CHECK (
        verified_at IS NULL
        OR (purpose = 'reset' AND delivery = 'code'));

-- +goose Down
ALTER TABLE auth_codes
    DROP CONSTRAINT auth_codes_verified_shape;

ALTER TABLE auth_codes
    DROP COLUMN verified_at;
