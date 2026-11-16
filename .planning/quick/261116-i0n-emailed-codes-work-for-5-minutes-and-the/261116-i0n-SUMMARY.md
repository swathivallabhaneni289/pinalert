---
phase: quick-261116-i0n
plan: 01
subsystem: auth
tags: [email-code, code-lifetime, login-gate, trust-model]
requires: []
provides:
  - "mailer.CodeLifetime is 5 minutes; every reader of the code's life follows it"
  - "the code screen says 'The code works for N minutes.' under the code box"
affects: [internal/mailer, internal/service, internal/api/handlers, web/templates, docs]
key-files:
  modified:
    - internal/mailer/mailer.go
    - internal/service/credentials.go
    - internal/service/codelifetime_test.go
    - internal/service/credentials_test.go
    - cmd/mailcheck/main_test.go
    - docs/TRUST-MODEL.md
    - web/templates/legal_body.html.tmpl
    - internal/api/handlers/auth.go
    - internal/api/handlers/login_gate_e2e_test.go
    - web/templates/login_gate.html.tmpl
    - web/login_card_contract_test.go
  created:
    - web/code_life_contract_test.go
decisions:
  - "One constant: mailer.CodeLifetime = 5 minutes; no second number added"
  - "CodeMinutes is computed in the handler with a mirror of service.minutesValid (whole minutes, at least 1)"
status: complete
commits: 2
plan_head_before: 58391f2a6bc75af745bc3396cb6d092ab2e45542
plan_head_after: b716fe34937cfb65d86a63a528b34edf4dae082d
completed: 2026-11-16
---

# Phase quick-261116-i0n Plan 01: Emailed codes work for 5 minutes Summary

An emailed sign up or Forgot password code is now refused after 5 minutes (was 10), the email, Policy and trust model say 5, and the code screen shows "The code works for 5 minutes." under the code box so the 45 second Resend countdown is not mistaken for the code's life.

## Commits

- 4e98a01e fix(261116-i0n): an emailed code works for 5 minutes, not 10
- b716fe34 feat(261116-i0n): the code screen says how long the code works

(The SUMMARY itself is committed alone after these, so `commits: 2` counts the two task commits.)

## Where 10 minutes became 5

- internal/mailer/mailer.go: `CodeLifetime = 5 * time.Minute` (service CodeTTL, email "within N minutes" via minutesValid, cmd/mailcheck sample email and `data-code-ttl-seconds` all follow it)
- internal/service/credentials.go: ResetConfirmWindow comment ("its length is no more than the code's own 5 minute life")
- docs/TRUST-MODEL.md: "It lasts 5 minutes" (sign in codes) and the reset confirm sentence ("5 minutes, no longer than the code's own 5 minute life")
- web/templates/legal_body.html.tmpl: Policy "works for 5 minutes"
- Pins: internal/service/codelifetime_test.go (5m, 5 minutes), cmd/mailcheck/main_test.go (MinutesValid 5), internal/service/credentials_test.go (code life 5m, MinutesValid 5 twice, expired code advance 11 to 6 minutes, TestLiveCode seconds left 600 to 300 with the clock step 200.4 s)

Left alone as the plan said: Resend wait 45 s, ResetConfirmWindow 5 minutes, ResumeWindow, all limits, CurrentTermsVersion, the Policy date, the mailer unit tests that pass `MinutesValid: 10` as plain input, and every other "10 minutes" in the repo (official alert refresh, mail pacing "15 in any 10 minutes", location staleness), which are not the code's life.

## The line on the code screen

- web/templates/login_gate.html.tmpl: `<p class="login-rule" id="code-life">The code works for {{.CodeMinutes}} minutes.</p>` inside `#code-entry-wrap` right under `.login-field--code`; `#code-input` has `aria-describedby="code-life"`. It hides with the code box on Forgot password step two. No new CSS.
- internal/api/handlers/auth.go: `CodeMinutes` on the view model, filled once after `CodeTTLSeconds` (default and service value) through a small `codeMinutes` helper that mirrors `service.minutesValid`. All entries (sign up, reset, reload) go through LoginGate, so one place covers them.
- web/static/js/auth.js: no change. `setCodeMode` toggles the whole `#code-entry-wrap`, so the line is shown on step one and hidden on step two with the box; it never changes text.
- Tests: web/code_life_contract_test.go (place inside the wrapper, under the box, number from the view model, describedby, not in the Create account, Sign in or Forgot panels, hidden with the wrapper on step two, no dash), `p711Vm` now carries `CodeMinutes` 5 and `CodeTTLSeconds` 300, and TestLoginGateClockAttributes pins "7 minutes" from a service limit of 7 minutes.

## Gate

- `go build ./...` and `go vet ./...`: clean
- `go test ./... -p 1 -count=1 -v` on a fresh q_i0n_a: 22 packages ok, 0 FAIL. Top level tests: 2491 PASS, 0 FAIL, 3 SKIP (TestGeocodeLive, TestOfficialLive, TestScreeningFixture); 6864 PASS counting subtests.
- `node --test web/*.mjs`: 1643 tests, 1643 pass, 0 fail
- Wording check (`git grep` for the five banned words, minus LinkedInOrder): prints nothing
- No swagger annotation changed, so `make swag` was not run
- Looked at it: server on 8794 against q_i0n_a with the log mailer, made up sign up (i0n.tester1 to 4 at example.com), Playwright WebKit pictures of the code screen, outside the repo:
  - /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/i0n-exec/code-screen-laptop-light.png (1440 by 900)
  - /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/i0n-exec/code-screen-laptop-dark.png
  - /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/i0n-exec/code-screen-phone-light.png (390 by 844)
  - /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/i0n-exec/code-screen-phone-dark.png
  - Probe in all four: text "The code works for 5 minutes.", visible, aria-describedby code-life, 16 px under the box, 14 px, no horizontal scroll. Server stopped and q_i0n_a dropped afterwards.

## Deviations from Plan

**1. [Rule 1 - Bug] The reset renewal test could no longer hold with a 5 minute code**
- **Found during:** Task 1 gate run
- **Issue:** "a confirm that ran out while the code is live is renewed by typing the code again" needs the 5 minute confirm window to be shorter than the code's life. With both at 5 minutes a confirm can never run out while the code is live, so it failed (LiveCode showed 0 seconds left).
- **Fix:** the test builds its rig with `WithCredentialLimits` giving the code a 10 minute life of its own, so the renewal path stays pinned for any future limits. The production numbers did not change.
- **Files modified:** internal/service/credentials_test.go
- **Commit:** 4e98a01e

**2. [Plan wording] "no longer than the code's own life" reworded**
The window is counted from the confirm, so it can end after the code does (a confirm at 4 minutes lasts to 9). The comment and the trust model say "its length is no more than the code's own 5 minute life" and the trust model notes it can end after the code does. No behavior change.

**3. [Process] Live feed fetch on the scratch server**
The development server on 8794 fetched the public GDACS and SACHET alert feeds on start (its normal background refresh). No Resend or mail call was made (log mailer, no key).

## Known Stubs

None.

## Threat Flags

None. No new endpoint, input or trust boundary; the lifetime shrank, which only narrows the guessing window.

## Self-Check: PASSED

- 4e98a01e and b716fe34 are ancestors of HEAD; web/code_life_contract_test.go exists; the four pictures exist in the scratchpad folder and none is in the repo.
