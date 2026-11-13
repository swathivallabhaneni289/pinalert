---
phase: quick-261113-r5w
reviewed: 2026-11-13T00:00:00Z
depth: deep
files_reviewed: 22
files_reviewed_list:
  - internal/store/migrations/00025_add_auth_code_verified.sql
  - internal/store/queries/auth_codes.sql
  - internal/store/sqlc/auth_codes.sql.go
  - internal/store/sqlc/models.go
  - internal/service/credentials.go
  - internal/service/credentials_signin.go
  - internal/service/credentials_test.go
  - internal/service/credentials_reset_steps_db_test.go
  - internal/service/credentials_lock_db_test.go
  - internal/service/credentials_spent_db_test.go
  - internal/api/handlers/credentials.go
  - internal/api/handlers/auth.go
  - internal/api/handlers/reset_steps_e2e_test.go
  - internal/api/handlers/credentials_security_e2e_test.go
  - internal/api/handlers/auth_helpers_e2e_test.go
  - internal/api/router.go
  - internal/api/gate_test.go
  - docs/swagger.yaml
  - docs/TRUST-MODEL.md
  - web/templates/login_gate.html.tmpl
  - web/static/js/auth.js
  - web/static/css/auth.css
findings:
  critical: 0
  warning: 3
  info: 7
  total: 10
  necessity: 1
status: issues_found
---

# Quick 261113-r5w: Code Review Report

**Reviewed:** 2026-11-13
**Depth:** deep (diff `768c8cca..f9daa79e`, read only, no tests or database run)
**Files Reviewed:** 22
**Status:** issues_found

## Summary

The server side of the split is careful. I traced every path into step two and found no way for a party that does not hold this browser's session id to reach it:

- `ConfirmReset` (internal/service/credentials_signin.go:444) reads the newest row of the session and demands purpose reset, delivery code, unused and `verified_at` inside `CodeTTL`. A sign up row, an `account_exists` row, a `no_account` row (the CHECK `auth_codes_verified_shape` also forbids the mark there), an expired confirm, a replaced row (Resend or Forgot password again retire it through `IssueAuthCode`, and it is no longer newest) and another browser all get `ErrWrongCode`.
- The spend is `ConsumeVerifiedResetCode`, conditional on `used_at IS NULL` and the window, inside the same transaction as the address lock, the new epoch, the delete of other rows, the throttle clear and `LogOutAllDevices`. Two parallel step two calls give one winner (tested on the database). `SpentCode` still works because the spend writes the equal `used_at`/`expires_at` pair.
- Step one keeps the try counted before the compare (`proveCode`), and the mark is guarded by `used_at IS NULL AND expires_at > now()`, so a Resend between the compare and the mark makes the mark write 0 rows.
- Answers for a known and an unknown address are byte identical at step one (same bump, same keyed hash compare, same 400) and at step two (an unknown address can never be verified). The e2e test `TestResetStepsEnumerationIdentical` compares both.
- The migration's CHECK and Down are correct. The new route has `cop.Handler`, the strict body, `no-store`, the 503 without a service, swagger, and it is in `authRoutePaths`, so the cross site and content type tests cover it.

The weaknesses are not in the queries. They come from what the split creates: a confirmed reset that lasts up to 10 minutes and is held only by the browser's long lived session cookie. Anyone who holds that session id during the window can set the password without the code. Before this change, that browser needed the emailed code in the same request. Two findings below (WR-01 shared device, WR-02 planted cookie) describe the two ways this happens. One fix closes both: rotate the session id at step one. Missing tests are WR-03. The rest is Info.

The necessity reference (`~/.claude/gsd-core/references/necessity-review.md`) was read. One necessity finding (IN-03).

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: A confirmed reset is a 10 minute capability anyone using the same browser can spend, and Change email does not disarm it

**File:** `internal/service/credentials_signin.go:623`, `internal/api/handlers/auth.go:252-256`, `web/static/js/auth.js:880-892`, `internal/store/queries/auth_codes.sql` (MarkAuthCodeVerified)
**Issue:** The scenario on a shared phone, which the project already treats as in scope (`spentCodeMemory`, review SK3-02):
1. Person A starts Forgot password, types the right code at step one, then leaves the phone or presses Change email.
2. Within 10 minutes of the confirm, person B opens or reloads `/login` in the same browser.
3. `LiveCode.Verified` is true, so the page draws step two ("Code confirmed. Choose a new password.") for A's address.
4. B posts `/api/auth/reset/password` with a password of their choice. A's password is replaced and every session and push subscription of A's account ends.

B never needed the code. Before this change, the same reload showed the code box, and setting a password needed the emailed code in that request.

Two choices in the code make this worse than the plan required:
- Change email at step two (`changeEmail`, auth.js:880) only switches the panel on the client. `verified_at` stays set on the server, so a reload lands back on step two. The plan called Change email "the way out", but on the server it is not one.
- The window counts from the confirm, and the mark is allowed until `expires_at`. A code confirmed at minute 9:59 after the send keeps step two open until about minute 20.

**Fix:** Pick one or more:
- Disarm on exit. Have Change email on step two call a small endpoint, for example `POST /api/auth/code/cancel`, that sets `used_at = now()` on the browser's newest unused reset row. Then a reload after Change email shows the empty start panel.
- Shorten the confirm window to a dedicated value well under `CodeTTL` (for example 5 minutes), and cap it at `expires_at` plus a small grace. In `ConsumeVerifiedResetCode` and `verifiedResetRow`, use `LEAST(verified_at + window, expires_at + grace)`.
- The stronger fix in WR-02 (a fresh session id at step one) does not help with this case, because B uses the same browser. So disarming on exit is needed in any case.
- At minimum, record this trade in docs/TRUST-MODEL.md (see IN-02).

### WR-02: A planted session cookie now becomes an account takeover through step two

**File:** `internal/service/credentials_signin.go:386-416` (CheckResetCode), `internal/api/handlers/credentials.go:497-519` (ConfirmResetCode), `internal/api/handlers/reset_steps_e2e_test.go:46`
**Issue:** The project defends against planted session cookies. Sign in always binds a freshly drawn id (`rotate`, credentials_signin.go:546; TRUST-MODEL "a planted cookie never becomes a signed in one", `TestFixationFreshSessionID`). The reset flow deliberately keeps the request's session id. Before this change that was harmless: the victim's own request carried the code and the new password together, so a party holding the same id gained nothing.

Now the confirm is server state keyed only by the session id:
1. The attacker holds a valid signed id S and plants it in the victim's browser.
2. The attacker calls `POST /api/auth/reset` on S with the victim's address. The victim's `/login` now shows "Reset your password, we emailed <victim>", or the victim starts the reset on S themselves.
3. The victim types the emailed code at step one. The row is marked verified on S.
4. The attacker polls `GET /login` with S until it shows `data-code-step="password"` (no bucket on that page), then posts `/api/auth/reset/password` with their own password. The polling of step two itself is limited only to one try per 30 s after a burst of 10, which is far inside the 10 minute window.
5. The victim's step two then gets the wrong code answer and is sent back to step one.

**Precondition:** planting the cookie means beating a signed, HttpOnly, SameSite=Lax cookie (Secure in production). In practice that takes a sibling subdomain or an HTTP man in the middle on a first visit with no existing cookie. That is why this is a Warning and not Critical. But it breaks an invariant the trust model states, and the impact is a full account takeover.

**Fix:** On a step one success, move the confirm to a freshly drawn session id that only the browser receiving the answer holds:
- In `CheckResetCode`, inside one statement, update the verified row's `session_id` to `newID()`.
- Return the new id. Have `ConfirmResetCode` call `cfg.Session.SetCookie(w, newID)`, the same helper sign in uses.
- An attacker still holding S then has no row, and the reload path (`LiveCode` on the new cookie) keeps working.
- The e2e assertion at reset_steps_e2e_test.go:46 ("step one set a cookie: it must sign nobody in") must flip to expect a fresh, unbound cookie.
- Add a test: after step one, step two on the old id answers the wrong code error.

### WR-03: Missing tests for the new route's limits and for the races the doc comments promise

**File:** `internal/api/handlers/credentials.go:545-575`, `internal/service/credentials_signin.go:438-443`, `internal/service/credentials_reset_steps_db_test.go`
**Issue:** These behaviours are claimed in code or comments, but no test exercises them:
1. **Bucket on `/api/auth/reset/password`.** Nothing checks that it answers 429 when the code try bucket is spent. Nothing checks that a refused password gives the token back (`isValidation`, line 563), or that a wrong code answer at step two keeps it. A regression that drops the refund, or that drops the bucket entirely, would pass the suite. `authlimits_test.go` only tests `ReserveCodeTry` in isolation.
2. **Resend between step two's read and its transaction.** The doc comment says this "rolls everything back". There is no database test with a held transaction, the way `TestConfirmLockOrder` uses a hold for the address lock.
3. **Parallel step one with the right code.** The `COALESCE` claim ("one verify") is tested only sequentially on the fake.
4. **Step one racing a Resend.** Expected answer: the mark writes 0 rows and step one returns the wrong code error.
5. **Change email then reload while verified.** This would pin whatever is decided for WR-01.

**Fix:** Add an e2e test that sets `CodeTryDevice` to burst 1 and checks, on `/api/auth/reset/password`:
- one refused password, then a second call, is not 429 (the token came back);
- one wrong code answer, then a second call, is 429.

Add `t.Run` cases to `TestResetStepsOnDatabase` for items 2 to 4, using a querier hook to hold step two between `GetLatestAuthCode` and `LockCredentialAddress`.

## Info

### IN-01: The ConfirmReset comment about a Resend rolling everything back is only half true

**File:** `internal/service/credentials_signin.go:440-443`
**Issue:** Whether "a Resend that retired the row after it was read rolls everything back" holds depends on which statement commits first:
- If Resend's `IssueAuthCode` commits first, the consume writes 0 rows and the transaction rolls back, as the comment says.
- If step two's consume runs first, it holds the row lock. Resend's retiring UPDATE waits, then re-checks `used_at IS NULL` and skips the row. Resend then inserts and emails a fresh live code after the reset committed. That new row survives `DeleteOtherAuthCodesForEmail`, because it did not exist yet.

The outcome is harmless (the owner gets one more valid code), but the comment overstates the guarantee.
**Fix:** Change the sentence to "a Resend that commits before the spend makes it write 0 rows and rolls everything back; one that commits after leaves a fresh code for the address." Pin it with the race test in WR-03.

### IN-02: TRUST-MODEL overstates why step two cannot skip the code

**File:** `docs/TRUST-MODEL.md:643`
**Issue:** "That is why step two cannot be used to skip the code" is true for any party without the session. It leaves out that during the confirm window, whoever holds the browser's session can set the password without the code (WR-01, WR-02).
**Fix:** Add one sentence: "For up to 10 minutes after the confirm, the browser that confirmed can set the password without the code. A shared device or a planted cookie inside that window is the remaining risk." Update it again if WR-01 or WR-02 is fixed.

### IN-03: Already defended elsewhere: `.login-actions .btn[hidden]`

**File:** `web/static/css/auth.css:214-218`
**Category:** necessity
**Issue:** The SUMMARY says the shared `.btn` display beats the hidden attribute, so this rule was needed. But `web/static/css/chrome.css:169-171` already has `.btn[hidden] { display: none; }`. That rule has specificity (0,2,0), which beats `main.css:614` `.btn { display: inline-flex; }` at (0,1,0). chrome.css is loaded on this page (login_gate.html.tmpl:10), and no rule in auth.css gives `.btn` a display. So the new rule changes nothing.
**Fix:** Remove it, or keep it and correct the comment and the SUMMARY's stated reason.

### IN-04: Going back from step two to step one leaves the clocks and the try count out of step with the server

**File:** `web/static/js/auth.js:621`, `web/static/js/auth.js:631-643`
**Issue:**
- `enterPasswordStep` sets `state.tries = 0`, but the server's `attempts` on the row is unchanged.
- `backToCodeStep` calls `startResendWait(0)` and never restarts `startExpiry`. Usually step two fails because the confirm ran out, and by then the code has also expired (`verified_at < expires_at`). The page then shows "That code is wrong or has expired." with a live Confirm code button and no expiry notice.
- If the old row already had 4 counted tries, the next wrong code on the client is "try 1", but the server has spent the budget, so the fifth try switch off appears late.

These are cosmetic, and the server still enforces the budget.
**Fix:** In `backToCodeStep`, call `expireNow()` when the failure is a run out confirm. Keep the client try count instead of zeroing it in `enterPasswordStep`, or set it to `MAX_CLIENT_TRIES` in `backToCodeStep` so the person is steered to Resend.

### IN-05: Step two is not announced to screen readers

**File:** `web/static/js/auth.js:613-626`, `web/templates/login_gate.html.tmpl` (`#code-lead-confirmed`)
**Issue:** `enterPasswordStep` clears `codeReadyLive` and moves focus to the new password box. "Code confirmed. Choose a new password." is a plain `<p>` that is not referenced by the input. A screen reader user hears only "New password, edit text" and is not told the code was accepted.
**Fix:** Write the line into the existing `codeReadyLive` status region in `enterPasswordStep`. Or add `code-lead-confirmed` to the new password input's `aria-describedby`, next to `code-new-rule`.

### IN-06: A page open during the deploy cannot reset until it is reloaded

**File:** `internal/api/handlers/credentials.go:497-519`, `cmd/server/main.go:380`
**Issue:** `AssetVersion` is the server start time, so a fresh load gets the new auth.js. But a person who has the old code screen open across the deploy still runs the old script:
- It posts `{code, password}` to `/api/auth/reset/code`.
- The strict decoder refuses the unknown field `password` with the generic 400 sentence, and the person is stuck until they reload.

`TestResetStepOneRefusesTheOldBody` makes this refusal deliberate. The cost is one confusing failure at the moment someone is locked out.
**Fix:** Accept and ignore a `password` field on `/api/auth/reset/code` for one release, logging nothing. Or accept the risk and note it in the deploy notes.

### IN-07: Step two does not re-check that the verified row is still the newest when it spends

**File:** `internal/service/credentials_signin.go:444-497`
**Issue:** "Newest row of this browser" is checked once, before the hash. `ConsumeVerifiedResetCode` spends by id only. If a sign up row is written in the same session during the hash, which takes hundreds of milliseconds, the reset still completes. Afterwards `LiveCode` shows the sign up screen. This is harmless, because the confirm was real, but it is a small gap against the documented rule.
**Fix:** Add `AND id = (SELECT max(id) FROM auth_codes WHERE session_id = $session)` to `ConsumeVerifiedResetCode`, or accept the behaviour and drop "newest" from the comment's guarantees.

---

_Reviewed: 2026-11-13_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
