---
phase: quick-261113-r5w
plan: 01
subsystem: auth
tags: [forgot-password, reset, two-step, auth_codes, sqlc, goose]
requires: []
provides:
  - "Forgot password in two steps: POST /api/auth/reset/code (code only) then POST /api/auth/reset/password (new password only)"
  - "auth_codes.verified_at, LiveCode.Verified, CredentialService.CheckResetCode and the code free ConfirmReset"
affects: [internal/service, internal/api/handlers, web/static/js/auth.js, web/templates/login_gate.html.tmpl, docs/TRUST-MODEL.md]
tech-stack:
  added: []
  patterns: ["confirm state kept on the code row, bound to the browser's session id", "window counted from the confirm, spent by a dedicated conditional UPDATE"]
key-files:
  created:
    - internal/store/migrations/00025_add_auth_code_verified.sql
    - internal/service/credentials_reset_steps_db_test.go
    - internal/api/handlers/reset_steps_e2e_test.go
    - web/reset_two_steps_contract_test.go
  modified:
    - internal/store/queries/auth_codes.sql
    - internal/store/sqlc/auth_codes.sql.go
    - internal/store/sqlc/models.go
    - internal/service/credentials.go
    - internal/service/credentials_signin.go
    - internal/api/handlers/credentials.go
    - internal/api/handlers/auth.go
    - internal/api/router.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/templates/login_gate.html.tmpl
    - web/static/css/auth.css
    - web/static/js/auth.js
    - web/auth_flow_test.mjs
    - docs/TRUST-MODEL.md
    - .planning/phases/07.1-password-sign-in-and-usernames/07.1-UAT.md
key-decisions:
  - "Step two is spent by a new ConsumeVerifiedResetCode guarded by verified_at, not by ConsumeAuthCode"
  - "verified_at and the window cutoff come from the service clock as parameters"
status: complete
commits: 3
plan_head_before: 768c8cca564076a0f9e56777d415643f0ebce6ef
plan_head_after: 4d8621cc8bec559f5a64b6af843c869d38b22167
---

# Phase quick-261113-r5w Plan 01: Forgot password in two steps Summary

Forgot password now asks for the emailed code first and only then for the new password: a confirmed code row (`auth_codes.verified_at`, bound to the browser's session id) is what opens step two, and step two does everything the old one step confirm did.

## What was built

Server
- Migration `00025_add_auth_code_verified.sql`: `verified_at TIMESTAMPTZ` plus `auth_codes_verified_shape`, which allows it only on `purpose = 'reset' AND delivery = 'code'` (a no_account, account_exists or sign up row can never carry it). Down drops the CHECK and the column; down then up was run on the scratch database.
- Queries: `MarkAuthCodeVerified`, `ConsumeVerifiedResetCode`, and `verified_at` on `GetLatestAuthCode`. `sqlc generate` leaves no diff.
- `CheckResetCode` (step one): latest row of the browser must be an unused reset row, then `cleanTypedCode`, `proveCode` (try counted before the compare), then the mark; every refusal is `ErrWrongCode`, so a no_account row answers exactly like a wrong code.
- `ConfirmReset(ctx, sessionID, newPassword)` (step two): newest row must be reset, delivery code, unused, `verified_at` set and inside `verified_at + CodeTTL`, else `ErrWrongCode`; then `checkPassword`, account lookup, hash, and one transaction with the address lock, new epoch, spend, throttle clear and `LogOutAllDevices` in the old order. SK-05 and CR-S02 comments kept true.
- `LiveCode.Verified`; the sign in page handler passes it as `CodeVerified` (reset rows only).
- Routes: `/api/auth/reset/code` is step one (`{code}` to `{"code_ok": true}`), new `/api/auth/reset/password` is step two (`{password}` to `{"password_set": true}`). Both take a token from the code try bucket, given back on success; step two also gives it back when the new password itself is refused. Both sit in the same exempt set, behind the same `cop.Handler`. Swagger regenerated (only the reset paths and three definitions changed; license line kept).

Page
- Step one: existing code screen, no visible password box, button "Confirm code". Step two: lead "Code confirmed. Choose a new password.", code box and Resend hidden, New password box with Show and the rule line, button "Set new password", Change email kept, focus on the password box. A reload on step two draws step two (`data-code-step`). A stale confirm (the server answers the wrong code error at step two) goes back to step one with "That code is wrong or has expired." and Resend ready, counting no try. Sign up's code screen and "Create account" are unchanged.

Docs: `docs/TRUST-MODEL.md` (two steps, confirm window, why step two cannot skip the code) and Test 3 of `07.1-UAT.md` (expected text only; no result or count touched).

## Gate (database q_r5w_a, created and dropped by the executor)

- `go build ./...` and `go vet ./...`: clean.
- `go test ./... -p 1 -count=1 -v` on a fresh q_r5w_a: exit 0, 22 packages ok, 5 with no test files, no FAIL package line. PASS lines: 6882 counting subtests (2479 top level), FAIL 0, SKIP 3: TestGeocodeLive, TestOfficialLive, TestScreeningFixture (the only skips allowed). The web package was run again after a last one line edit to `web/reset_two_steps_contract_test.go`: 679 PASS lines, ok.
- `node --test web/*.mjs`: tests 1632, pass 1632, fail 0, skipped 0.
- `sqlc generate` after the commits: no diff. Wording check prints nothing. No em or en dash in the added lines or the commit messages.

## Deviations from Plan

### Where the code made me differ from the design

1. **[Rule 1 - Bug] Step two spends through a new `ConsumeVerifiedResetCode`, not `ConsumeAuthCode`.** `ConsumeAuthCode` requires `expires_at > now()`, and `expires_at` is the send time plus 10 minutes, so a code confirmed at minute 9 and used at minute 12 would be refused although the confirm window (10 minutes from the confirm) is still open. The new statement is guarded by `used_at IS NULL`, reset, delivery code, `verified_at IS NOT NULL AND verified_at > cutoff`, and writes the same equal `used_at`/`expires_at` pair and cleared hash, so `SpentCode` works. Sign up still uses `ConsumeAuthCode`. `expires_at` is not pushed forward at confirm, so `LiveCode`'s resend wait is unchanged. `spend` now takes the row id, the email and a consume function.
2. **`verified_at` and the cutoff are parameters from the service clock, not `now()`** (the unit tests move the service clock; the mark query still guards with `expires_at > now()`).
3. **The mark uses `COALESCE(verified_at, $1)`** so a right code typed twice, or by two parallel requests, is one verify and never a spurious refusal. It still writes on a second confirm; the plan wanted none. The first moment is never moved (pinned by a test).
4. **Step two checks the confirm before `checkPassword`**, so an unconfirmed browser gets `ErrWrongCode` before anything about the password is judged or hashed (a refused password still changes nothing and keeps the confirm).
5. **Handler names** are `ConfirmResetCode` (step one) and `SetResetPassword` (step two); request types `ResetCodeRequest` (code only) and `ResetPasswordRequest`, response `CodeOKResponse`.
6. **New markup and CSS:** `#code-lead-confirmed`, `#code-entry-wrap`, `data-code-step` on the card, and `.login-actions .btn[hidden] { display: none; }` (the shared `.btn` sets a display, which beats the hidden attribute on Resend). Only the visible step's field carries `required`, since a hidden empty required field blocks a browser's submit before the submit event runs.
7. **Generated code** is in `internal/store/sqlc` (package `sqlcgen`), not a `sqlcgen` directory, as the task noted.
8. A confirm that is re-typed after it ran out of time is simply refused at the mark (`expires_at > now()`); the page then shows the existing wrong code line.

### Auto-fixed issues
None beyond item 1.

## Auth gates
None.

## Known Stubs
None.

## Threat Flags
| Flag | File | Description |
|------|------|-------------|
| threat_flag: new_route | internal/api/router.go | POST /api/auth/reset/password joins the exempt auth set on the same terms as its siblings (strict JSON body, CrossOriginProtection, code try bucket, 503 without a CredentialService). It does no work for a browser with no confirmed code before the cheap row read. |

## Self-Check: PASSED

- Created files present: migration 00025, `credentials_reset_steps_db_test.go`, `reset_steps_e2e_test.go`, `reset_two_steps_contract_test.go`.
- Commits present: bbe461f6 (server), ae3fc08b (page), 4d8621cc (docs).
- STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched or staged; nothing was pushed.
