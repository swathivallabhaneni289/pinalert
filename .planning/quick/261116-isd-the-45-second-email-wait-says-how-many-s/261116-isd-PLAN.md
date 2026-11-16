---
phase: quick-261116-isd
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/service/auth.go
  - internal/service/credentials.go
  - internal/service/*_test.go
  - internal/api/handlers/credentials.go
  - internal/api/handlers/*_test.go
  - web/static/js/auth.js
  - web/*_test.go
  - web/*.mjs
  - docs/TRUST-MODEL.md
  - README.md
requirements: [QUICK-261116-isd]
must_haves:
  truths:
    - "A second code email to the same address inside the 45 second wait is refused with 'A code was just sent to this email. Try again in N seconds.', where N is the real time left (at least 1; '1 second' when it is one), not 'Too many requests. Try again in a minute.'"
    - "Retry-After and retry_after_seconds carry the same N, so the Resend button counts down from the real time left"
    - "The answer is the same for an address with an account and one without (no new way to tell them apart)"
    - "Every other refusal keeps its words: the per device and per network buckets ('Too many requests. Try again in a minute.'), the hourly 'Too many codes for this email. Try again in an hour.', the sign in lock, the daily ceiling"
---

# Quick 261116-isd: the 45 second email wait says how many seconds are left

## Why

On 2026-11-16, during 7.1 test 2 on the test copy, the owner signed up tester9@example.com, tapped Change email, and pressed Email me a code again 25 seconds later. The server refused it with "Too many requests. Try again in a minute." (log: POST /api/auth/sign-up 429 at 13:19:40, 25 s after the 13:19:15 send). The owner read it as an error. The refusal is the per address wait of 45 seconds between two code emails (service.ResendCooldown), but the words say "too many" and "a minute" when about 20 seconds were left. I told the owner I would fix the message.

## Today

internal/service/credentials.go `claimRefused` returns `ErrRateLimited` for a refused claim still inside the cooldown (and also when no row is found), and `ErrTooManyCodes` when the hourly count is spent. internal/api/handlers/credentials.go `writeCredentialError` maps `ErrRateLimited` to `writeAuthRateLimited(w, authRateLimitedMessage, time.Minute)`. `GetAuthEmailSendWindow` already reads `last_requested_at`. web/static/js/auth.js `messageFor` shows the server's sentence for a 429; the Resend click restarts its countdown from `retry_after_seconds` when it is no longer than the code's life.

## Task 1 (one or two commits)

- Service: when the claim is refused because the last email to this address was less than `SendCooldown` ago, return a typed error carrying the time left, for example `SendWaitError{Wait time.Duration}` (match the style of the sign in lock error that `SignInLockedMessage` serves), with `errors.Is(err, ErrRateLimited)` still true so nothing else changes meaning. Time left = `last_requested_at + SendCooldown - now`, at least 1 second. Keep plain `ErrRateLimited` for the no row case. Do not change the claim query, the cooldown, the hourly count or any limit.
- A message function next to `SignInLockedMessage`, for example `SendWaitMessage(wait)`: "A code was just sent to this email. Try again in N seconds." (N rounded up; "Try again in 1 second." for one). No em or en dashes.
- Handler: `writeCredentialError` maps the typed error to `writeAuthRateLimited(w, service.SendWaitMessage(e.Wait), e.Wait)` before the plain `ErrRateLimited` case. Every route that can hit the claim gets it: sign up, Resend (`/api/auth/code/resend`), Forgot password. The per device and per network buckets (`gate.ReserveCodeSend` and friends) keep `authRateLimitedMessage`.
- Enumeration: check that the claim, and so the refusal and its N, happen the same way for an address with an account, one without, and a Forgot password `no_account` row. If any path would now answer differently for a known and an unknown address, keep that path on the old words and say so in the SUMMARY.
- auth.js: the server's sentence already shows; check that a create form 429 shows it on the create panel (not a field error) and that Resend's countdown uses the new real N. Change nothing else.
- Swagger: if a 429 example or description names the old words for these routes, update it and run `make swag` (swag is at ~/go/bin/swag); keep the @license.name line "Unlicensed (all rights reserved)".
- Tests: service (a refused claim at 25 s answers 20 s left; at 44.5 s answers 1 s; the hourly case still ErrTooManyCodes; known and unknown address answer alike), handler e2e (sign up twice within the wait: 429, Retry-After equals retry_after_seconds equals the N in the sentence; Resend within the wait the same), and node tests for the create panel and Resend if they pin the old sentence. Update every test that pins "Too many requests. Try again in a minute." for this per address case only.
- Docs: docs/TRUST-MODEL.md and README.md where they describe the 45 second wait: one short clause that the person is told the seconds left.

## Gate and rules

- Full gate: `go build ./...`, `go vet ./...`, `go test ./... -p 1 -count=1 -v` on a fresh database q_isd_a (only TestGeocodeLive, TestOfficialLive, TestScreeningFixture may skip), `node --test web/*.mjs`, `gofmt -l` on changed Go files prints nothing.
- Look at it: start a server on port 8794 against q_isd_a with the log mailer (ENV=development, no RESEND_API_KEY), sign up a made up person (isd.tester@example.com), tap Change email, press Email me a code again at once, and take Playwright WebKit pictures of the create panel with the message at 1440 by 900 and 390 by 844 into /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/isd-exec/ (never in the repo). Playwright is at NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules. Stop the server after.
- Databases: only q_isd_a, created and dropped by you. Never pinalert_test, pinalert, p9_view, p31_4_uat, p34_uat, p3_view, p4_uat or any database you did not create. Confirm DATABASE_URL and PGDATABASE are empty, then export DATABASE_URL in the same Bash call.
- Ports: only 8794. Never 8080, 8090, 8776, 8778, 8781, 8782 or 8797, and never touch their processes.
- No live network call to Resend or any mail server. Made up example.com people only. Helper files only in the isd-exec folder above. No images in the repo. No Python in any form, not even --version. No migration.
- Path limited commits (`git add -- <new paths>` then `git commit -- <paths>`), trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md, ROADMAP.md or REQUIREMENTS.md; skip GSD steps that update STATE.md. Never push. The wording check `git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder` prints nothing.
- SUMMARY at .planning/quick/261116-isd-the-45-second-email-wait-says-how-many-s/261116-isd-SUMMARY.md, committed alone and last, with the gate numbers, the routes that now say the seconds left, the enumeration check result, and any place you differed from this plan.
