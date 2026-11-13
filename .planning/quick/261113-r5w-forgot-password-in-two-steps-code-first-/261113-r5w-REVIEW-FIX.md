---
phase: quick-261113-r5w
fixed_at: 2026-11-13T20:45:00Z
review_path: .planning/quick/261113-r5w-forgot-password-in-two-steps-code-first-/261113-r5w-REVIEW.md
iteration: 1
findings_in_scope: 10
fixed: 9
skipped: 1
status: partial
---

# Quick 261113-r5w: Code Review Fix Report

**Fixed at:** 2026-11-13
**Source review:** `.planning/quick/261113-r5w-forgot-password-in-two-steps-code-first-/261113-r5w-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 10 (WR-01 to WR-03, IN-01 to IN-07; the owner's decisions put every one in scope, IN-06 as an accepted skip)
- Fixed: 9
- Skipped: 1 (IN-06, accepted by decision)
- One extra change was needed that the review did not list (see "Found while fixing").

Fixes were made in the isolated worktree the harness gave this run (branch `worktree-agent-a84671e0b250e8cb8`, based on `0a9b9568`). No second worktree, recovery sentinel or fast-forward tail was created: the caller merges this branch. All gates ran in that worktree, against the scratch database `q_r5w_b` (created fresh for the gate and dropped at the end), so the numbers are reproducible from this branch, not from the main checkout. No migration was added (everything is an UPDATE on existing columns); 00025 is untouched.

## Fixed Issues

### WR-01: A confirmed reset was a 10 minute capability, and Change email did not disarm it

**Status:** fixed: requires human verification (logic)
**Files modified:** `internal/service/credentials.go`, `internal/service/credentials_signin.go`, `internal/store/queries/auth_codes.sql`, `internal/store/sqlc/auth_codes.sql.go`, `internal/api/handlers/credentials.go`, `internal/api/router.go`, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`, `docs/TRUST-MODEL.md`, `web/static/js/auth.js`, tests
**Commits:** `02dba9a4` (a, the 5 minute window), `e658b1d2` (b, the cancel), `cf4d4ec0` (follow-up, below)
**Applied fix:**
- (a) `service.ResetConfirmWindow = 5 * time.Minute`, one exported constant beside the other credential limits, used by `ConfirmReset`, the spend, `LiveCode`, the swagger text and TRUST-MODEL. A test pins 4m59s (accepted), 5m and 6m (refused).
- (b) `POST /api/auth/reset/cancel` (body `{}`, same `cop.Handler`, exempt set and route lists as its siblings: router, gate test row, `authRoutePaths`, swagger test, the two web contract tests). `ClearAuthCodeVerified` clears `verified_at` on this session's newest unused reset row. The answer is `{"cancelled":true}` with the same bytes and headers for a browser with a confirm, a pending code, no code and a no account row (compared in `TestResetCancelAnswersTheSameEverywhere`). `auth.js` calls it from Change email when the card is on reset step two, does not wait for the answer, and leaves at once. A reload afterwards draws step one, and step two is refused until the code is typed again. Swagger regenerated with `make swag`; the license line is kept and only the reset paths and two new definitions changed.
- The cancel takes no code try token. It sends nothing, hashes nothing and changes only the caller's own session row.

### WR-02: A planted session cookie became an account takeover through step two

**Status:** fixed: requires human verification (logic)
**Files modified:** `internal/store/queries/auth_codes.sql`, `internal/store/sqlc/auth_codes.sql.go`, `internal/service/credentials_signin.go`, `internal/api/handlers/credentials.go`, `internal/api/router.go`, swagger files, tests
**Commit:** `23af7aac`
**Applied fix:** `CheckResetCode` now takes `newID func() string` (as `SignIn` does) and returns the fresh id. `MarkAuthCodeVerified` marks the row and re-keys it to the new id in one statement guarded by `session_id = old id`. In the same transaction the old id is ended the way `rotate` ends a replaced one (push subscriptions deleted only when the id was bound to an account, then `RevokeSession`). `ConfirmResetCode` takes the session manager and sets the new cookie on the step one answer. `rotate` itself was not reused: it binds an account, which would sign the browser in.
Tests: `TestResetStepTwoRefusesAPlantedSession` (the attacker on the old id gets no step two page and the wrong code answer, the victim on the fresh id completes the reset); the check at `reset_steps_e2e_test.go:46` flipped (cookie set, id changed, new id unbound, `GET /api/reports` 401); `TestResetStepOneCookieBehaviourIsTheSame` (a wrong code sets no cookie and moves nothing for an address with and without an account, answers compared byte for byte, and the known address's right code typed in the no account browser is a plain wrong code); service tests that a wrong code returns no id. The claim in the decision holds by construction: the only path to a new cookie is a right code on a `delivery = 'code'` row, and a no account row never held a code.
Two older tests were adapted because the browser now holds a different id after step one (`TestResetEndsEverySession`, and the "lost answer" case of `TestLoginGateSpentCode`, where the lost answer is now step two's).

### WR-03: Missing tests

**Status:** fixed
**Files modified:** new `internal/api/handlers/reset_steps_limits_e2e_test.go`, new `internal/service/credentials_reset_races_more_db_test.go` (helpers are in `credentials_reset_races_db_test.go`, added with IN-07)
**Commit:** `d33e11b6` (item 5 is in the WR-01 cancel commit `e658b1d2`, as `TestResetCancelTurnsTheConfirmOff`)
**Applied fix:**
1. `TestResetStepsTryBucket` on a bucket of one token: step one answers 429 after a wrong code; step two keeps the token for a wrong code answer (second call 429); step two gives the token back for a refused password (three refusals, then success) and on success. Mutation checked: dropping the refund makes it fail.
2. `TestResetStepTwoWithResendDuringIt`: a Resend landing between step two's read and its transaction gives the wrong code answer, the password epoch does not move, the old password still signs in, and the Resend's code is live and not confirmed. The held step is a hasher that waits (`holdHasher`).
3. `TestResetStepOneRaces/two right codes at once...`: both count their try before either marks; exactly one winner, one `ErrWrongCode` with no id, one verified row on the winner's id, nothing on the other ids.
4. `TestResetStepOneRaces/a Resend between the count and the mark...`: the mark writes nothing, no id is returned, nothing rotated.
5. Change email then reload: in `e658b1d2`.

### IN-01: ConfirmReset comment about a Resend was half true

**Status:** fixed
**Files modified:** `internal/service/credentials_signin.go`
**Commit:** `c80f9ea0`
**Applied fix:** The comment now says a Resend that commits before the spend makes it write 0 rows and rolls everything back, and one that commits after leaves a fresh code for the address; it names the test that pins the first order and the fresh code afterwards.

### IN-02: TRUST-MODEL overstated why step two cannot skip the code

**Status:** fixed
**Files modified:** `docs/TRUST-MODEL.md`
**Commit:** `8700bdb4` (the renewal clause was added in `cf4d4ec0`)
**Applied fix:** The sentence now says it holds for anyone who does not hold the browser's session, names the 5 minute window, the fresh session id at step one, the cancel on Change email and the in-transaction newest row check, and states what is left: a shared device used by someone else inside the window without Change email pressed.

### IN-03: Already defended elsewhere: `.login-actions .btn[hidden]`

**Status:** fixed (verified first, then removed)
**Files modified:** `web/static/css/auth.css`, `web/reset_two_steps_contract_test.go`
**Commit:** `d42f1912`
**Applied fix:** Verified before removing: `web/static/css/chrome.css:169` has `.btn[hidden] { display: none; }` at specificity (0,2,0), which beats `main.css` `.btn { display: inline-flex; }` at (0,1,0); `login_gate.html.tmpl` links main.css, auth.css, then chrome.css; and no rule in auth.css sets a display on `.login-actions .btn` (only `width: 100%`). The rule is removed and its comment says why. `TestResetResendHiddenGuard` now pins the three facts (chrome.css rule, load order, no display in auth.css) instead of the removed rule. The SUMMARY's stated reason for the rule (the shared `.btn` display beating the attribute) was not edited; the review already notes it was wrong.

### IN-04: Going back to step one left the clocks and the try count out of step

**Status:** fixed: requires human verification (client logic)
**Files modified:** `web/static/js/auth.js`, `web/auth_flow_test.mjs`
**Commit:** `dbbb68a0`
**Applied fix:** `enterPasswordStep` adds one to the page's try count (the right code is a counted try on the server) instead of zeroing it. `backToCodeStep` switches Confirm code off with the too many tries sentence and focuses Resend when the budget is spent. The expiry notice restarts the way openCode starts it, but from the code's own deadline kept in `state.expiresAt` (also set when the page is reloaded on step two from `data-code-seconds-left`), not from a fresh full lifetime: a code that ran out says so, a live one is timed. Three node tests cover the carried count, a spent budget and both expiry cases.

### IN-05: Step two was not announced to screen readers

**Status:** fixed
**Files modified:** `web/static/js/auth.js`, `web/auth_flow_test.mjs`
**Commit:** `5ab5e55d`
**Applied fix:** `Code confirmed. Choose a new password.` is written into `#code-ready-live` (outside the wrappers that step two hides) by `enterPasswordStep` after the timers stop, and by a reload that draws step two. Both paths are asserted in the node tests.

### IN-07: Step two did not re-check that the verified row is still the newest when it spends

**Status:** fixed: requires human verification (logic)
**Files modified:** `internal/store/queries/auth_codes.sql`, `internal/store/sqlc/auth_codes.sql.go`, `internal/service/credentials_signin.go`, `internal/service/credentials_test.go` (fake), new `internal/service/credentials_reset_races_db_test.go`
**Commit:** `188643e2`
**Applied fix:** `ConsumeVerifiedResetCode` takes the session id and requires `id = (SELECT max(id) FROM auth_codes WHERE session_id = ...)` inside the spend's own transaction; 0 rows is the wrong code answer. The database test holds step two in the hasher, writes a Create account row on the same session, and checks nothing changed. Mutation checked: removing the clause makes it fail.

## Skipped Issues

### IN-06: A page open during the deploy cannot reset until it is reloaded

**File:** `internal/api/handlers/credentials.go:497-519`, `cmd/server/main.go:380`
**Reason:** skipped by decision: an old page gets a generic 400 until it is reloaded, and this is accepted. `TestResetStepOneRefusesTheOldBody` keeps the refusal deliberate. No code changed.
**Original issue:** An old auth.js open across the deploy posts `{code, password}` to `/api/auth/reset/code`, the strict decoder refuses the unknown field, and the person sees the generic failure until they reload.

## Found while fixing (not in the review)

**WR-01 follow-up, commit `cf4d4ec0`:** with the window cut to 5 minutes and the code living 10, a confirm that ran out while the code was still live could not be renewed. `MarkAuthCodeVerified` used `COALESCE(verified_at, ...)`, so typing the right code again kept the old, expired moment and step two stayed refused until a Resend. Before WR-01 the window was never shorter than the code's life, so this could not happen. The mark now sets `verified_at` to the moment of the latest right code. The person holds the code each time, and two parallel right codes still give one winner (the session guard from WR-02). Test: a confirm 6 minutes old is refused, typing the live code again renews it. The one older assertion that pinned "the first moment must stay" was replaced.

## Things to know

- **Lost step one answer.** Because the browser moves to a fresh id in the step one answer, an answer lost in transit leaves the browser on the revoked old id while the confirmed row is on the new one. The person sees the sign in card and asks for Forgot password again. A lost step two answer is unchanged (the browser holds the fresh id, so the spent code page still works).
- **Parallel right codes.** Two right codes at once give one winner; the other request answers the wrong code error although its code was right. The page disables its button while a request runs, so this needs a script or a double tap that beats the disable.
- **The fresh id has no `sessions` row** until something binds it (the middleware stores only ids it mints, and persist is best effort by design, see `Pitfall 2`). Nothing reads a row for an unbound browser; sign in binds a new id and creates the row.
- **Device try bucket.** The fresh id also has its own code try bucket, so the bucket restarts at step one. A right code is needed to get there, so this gives nobody tries they could not already have.
- **A stray command.** During the run I issued one `python3 --version` call by mistake against the no Python rule. The permission system denied it and nothing ran. No Python was used or written.

## Verification

Run in the isolated worktree against the fresh database `q_r5w_b` (dropped afterwards). `DATABASE_URL` and `PGDATABASE` were empty before the export in the same call.

- `go build ./...`: clean. `go vet ./...`: clean.
- `go test ./... -p 1 -count=1 -v`: exit 0. 22 packages ok, 5 with no test files, no FAIL package line. PASS lines: 6901 counting subtests (2487 top level), FAIL 0, SKIP 3: `TestGeocodeLive`, `TestOfficialLive`, `TestScreeningFixture` (the only skips allowed). The executor's run was 6882 PASS lines, so this adds 19 counting subtests.
- `node --test web/*.mjs`: tests 1636, pass 1636, fail 0, cancelled 0, skipped 0 (the executor's run was 1632).
- `sqlc generate` leaves no diff; `make swag` leaves no diff after the last commit and the license line `Unlicensed (all rights reserved)` is kept.
- Wording check `git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder` prints nothing. No em or en dash in any added line or commit message. Every commit carries the `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` trailer. STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched. Nothing was pushed.

---

_Fixed: 2026-11-13_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
