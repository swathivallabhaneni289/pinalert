---
phase: quick-261116-isd
plan: 01
subsystem: auth
tags: [rate-limit, code-email, messages]
requires: []
provides:
  - "service.SendWaitError and service.SendWaitMessage: the per address 45 second wait names the real seconds left"
affects: [internal/service/credentials.go, internal/api/handlers/credentials.go]
key-files:
  modified:
    - internal/service/credentials.go
    - internal/api/handlers/credentials.go
    - web/auth_flow_test.mjs
    - docs/TRUST-MODEL.md
    - README.md
  created:
    - internal/service/credentials_sendwait_test.go
    - internal/api/handlers/credentials_sendwait_e2e_test.go
decisions:
  - "SendWaitError matches errors.Is(ErrRateLimited) through an Is method, so any caller that only knows the plain sentinel behaves as before"
status: complete
commits: 1
plan_head_before: c9d082052a36d7c3d64572ea2ff928c1b77e3126
plan_head_after: a4c3132210de714feaf1ed3f558cde1f242ef392
completed: 2026-11-16
---

# Quick 261116-isd: the 45 second email wait says how many seconds are left

A second code email to one address inside the 45 second wait is now refused with "A code was just sent to this email. Try again in N seconds." where N is the real time left (rounded up, at least 1, "1 second" when it is one). Retry-After and retry_after_seconds carry the same N, so the Resend button counts down from the real time left.

## What changed

- `internal/service/credentials.go`: new `SendWaitError{Wait}` (with an `Is` method so `errors.Is(err, ErrRateLimited)` stays true) and `SendWaitMessage(wait)`, next to `SignInLockedError`. `claimRefused` now returns it when the claim was refused inside the cooldown, with Wait = `last_requested_at + SendCooldown - now`, at least 1 second. The no row case keeps plain `ErrRateLimited`; the hourly case keeps `ErrTooManyCodes` (checked first, so a spent hourly count inside the cooldown still says the hour sentence). No query, cooldown, count or limit changed.
- `internal/api/handlers/credentials.go`: `writeCredentialError` maps `SendWaitError` to `writeAuthRateLimited(w, SendWaitMessage(wait), wait)` before the plain `ErrRateLimited` case. Per device and per network buckets still answer `authRateLimitedMessage` ("Too many requests. Try again in a minute.").
- Tests: service (`credentials_sendwait_test.go`), handler e2e on a movable clock (`credentials_sendwait_e2e_test.go`), and two node tests in `web/auth_flow_test.mjs` (the old WR-B02 early Resend case now uses the new sentence and 20 s; a new create panel case).
- Docs: one clause each in `docs/TRUST-MODEL.md` and `README.md`.

## Routes that now say the seconds left

All three routes that take the per address claim:

- `POST /api/auth/sign-up` (Create account, also what "Email me a code" sends again after Change email)
- `POST /api/auth/code/resend` (Resend)
- `POST /api/auth/reset` (Forgot password)

## Enumeration check

The claim (`claimSendAt` or `claimSend`) runs before `lookupAccount` on SignUp, RequestReset and ResendCode, and the claim row is keyed by the normalised address only, so the refusal and its N do not depend on whether an account exists. Proved three ways: service tests (sign up, forgot password, Resend: known with password vs unknown, same message and Wait), handler e2e (`TestSendWaitSaysTheSecondsLeft/the_same_answer_for_an_address_with_an_account_and_one_without`: known and unknown sign up, Resend, and Forgot password compared with the existing `compareAuthAnswers`, which checks status, body bytes, Set-Cookie presence, Retry-After, Content-Length, Content-Type and Cache-Control), and the existing `TestEnumeration*` tests still pass. No path needed to keep the old words.

## auth.js

No change. `messageFor` already shows the server's sentence for a 429, `fieldFor` returns no field for a 429 so the create panel shows it on its own line without marking a field, and the Resend click restarts its countdown from `retry_after_seconds` when it is no longer than the code's life (20 s fits). Node tests pin both.

## Swagger

No 429 example or description for these routes names the old words, so no change and `make swag` was not run.

## Gate

- `go build ./...` and `go vet ./...`: clean.
- `go test ./... -p 1 -count=1 -v` on a fresh database q_isd_a: exit 0, 22 packages ok, top level 2495 PASS, 0 FAIL, 3 SKIP (TestGeocodeLive, TestOfficialLive, TestScreeningFixture).
- `node --test web/*.mjs`: 1644 pass, 0 fail.
- `gofmt -l` on the changed Go files: nothing. (`internal/api/handlers/place_test.go` and `places_e2e_test.go` are already unformatted on this base and are not part of this change.)
- Wording check (`git grep ... | grep -v LinkedInOrder`): nothing.
- Dash check on the diff: none.

## Look at it

Server on port 8794 against q_isd_a with the log mailer (ENV=development, no RESEND_API_KEY). Made up address isd.tester at example.com (with a number added per run so a repeat is not itself inside the wait): Create account, Change email, Email me a code again at once. The create panel line read "A code was just sent to this email. Try again in 45 seconds." (45 because the second press came under a second after the send, N is rounded up). Pictures, outside the repo:

- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/isd-exec/laptop-1440x900.png
- /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/isd-exec/phone-390x844.png

The server was stopped, q_isd_a dropped and the server log deleted afterwards.

## Deviations from Plan

- The plan file list named `web/*_test.go`; no Go test under `web/` needed a change (the contract test pins the client side fallback sentence, which is unchanged).
- Playwright WebKit pictures were taken with a 0.8 s settle wait after the line appeared; the first attempt caught the button mid transition and was retaken.
- The server on 8794 also made its normal outbound fetches of the official GDACS and Sachet feeds at start (no mail server and no Resend call). Not a change to the plan, noted for completeness.
- I used a small start script in the isd-exec folder instead of `env -i` because the sandbox refused that form; the script unsets RESEND_API_KEY and sets only ENV, PORT, BASE_URL and DATABASE_URL.
- No ledger file was created for commit counting; `commits:` above is measured with `git rev-list --count c9d08205..HEAD` (the plan commit is the base).

## Known Stubs

None.

## Self-Check: PASSED

Commit a4c31322 is an ancestor of HEAD; the two new test files and the summary exist.
