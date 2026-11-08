---
phase: quick-261108-fku
fixed_at: 2026-11-08T00:00:00Z
review_path: .planning/quick/261108-fku-profile-band-across-the-top-on-laptop-an/261108-fku-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Quick 261108-fku: Code Review Fix Report

**Fixed at:** 2026-11-08
**Source review:** `.planning/quick/261108-fku-profile-band-across-the-top-on-laptop-an/261108-fku-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 8 (WR-01, WR-02, IN-02, IN-03, IN-04, IN-05, IN-06, IN-09), with IN-09 settled by WR-02
- Fixed: 8
- Skipped: 0
- Left as they are on instruction: IN-01, IN-07 and IN-08

Gate on the final tree, on the scratch database q_fku_fix (dropped afterwards): `go build ./...` and `go vet ./...` clean; `go test ./... -p 1 -count=1 -v` gave 2455 top level `--- PASS` lines and 6817 `--- PASS` lines with subtests, 0 `--- FAIL`, 21 packages `ok`, no `FAIL` line, exit 0, and exactly three skips, TestGeocodeLive, TestOfficialLive and TestScreeningFixture; `node --test web/*.mjs` gave 1624 tests, 1624 pass, 0 fail, 0 skipped (the SUMMARY's gate had 1609). The wording check printed nothing. No dash was added in any touched file.

## Fixed Issues

### WR-02: The lock never cleared in the page, so the day count went stale

**Files modified:** `internal/service/account_username.go`, `internal/service/username.go`, `internal/service/account_username_test.go`, `internal/api/handlers/settings.go`, `internal/api/handlers/settings_username_e2e_test.go`, `web/templates/settings.html.tmpl`, `web/static/js/settings.js`, `web/settings_rows_contract_test.go`, `web/settings_username_test.mjs`, `web/settings_username_wait_contract_test.go` (new)
**Commits:** 93419584, 9c4fe366
**Applied fix:**
- **Server.** `AccountService.UsernameWaitUntil(profile)` returns the last change time plus `UsernameRenameGap` as RFC 3339 UTC, rounded up to the whole second so the page is never early, and empty outside the wait (same `usernameWaitDays` rule, same service clock, no store read). The Settings page writes it as `data-wait-until` on the Username window and writes nothing outside the wait.
- **Page.** `settings.js` reads the deadline once. On each opening it works out the days left, rounded up as `usernameWaitDays` rounds (`max(1, floor((left + day - 1) / day))`). No days left opens the window in the normal state (editable, Save username shown, OK hidden, the live check judging the stored name). Otherwise it opens locked with the sentence for that number of days. A change that replaced an earlier name sets the deadline to now plus 7 days in the page. A garbage or missing attribute means no deadline.
- **Words.** The script builds the sentence from named constants (`WAIT_PREFIX`, `WAIT_ONE_DAY`, `WAIT_MANY_DAYS`). `TestSettingsUsernameWaitWords` reads them from the embedded script and compares them with `service.UsernameWaitMessage` for 1 to 7 days, and pins `WAIT_GAP_DAYS` to `service.UsernameRenameGap` and the rounding line.
- **Replaced.** `data-wait-message`, `data-wait-after-change`, `UsernameWaitLine`, `UsernameWaitAfterChangeMessage` and the two view model fields are gone. The server still decides on save.
- **Node tests.** The page gets a hand moved clock (a `Date` subclass in the sandbox, with a test that proves it drives `Date.now()` inside the script). A deadline in the past opens normally; 1.5 days left reads "2 days"; a saved change locks the next opening with "7 days" and opens normally 7 days later; a window locked at load counts down 3, 2, 1 days and then opens normally; an unlocked window can type, check and save again.
- **Go tests.** `TestUsernameWaitUntil` (replaces `TestUsernameWaitLine`) and `TestSettingsUsernameWaitUntil` (replaces `TestSettingsUsernameWaitLine`; the page value equals the stored change time plus 7 days, in UTC, and goes with the gap). The template pin in `settings_rows_contract_test.go` was amended with a dated line. The other amended tests carry dated Amended lines.
- **Second commit, 9c4fe366, corrects a line of the first.** The first commit kept a lock from a live wait answer (another tab changed the name) as a deadline of now plus that answer's day count. That count is rounded up, so the deadline could run a day past the real end, and a locked opening sends no check to correct it. A device clock a little fast reaches this case on the ordinary path. A live wait answer now locks only the window open now and sets no deadline; the next opening follows the page deadline, and the usual window's live check answers wait again with a fresh count. This changed the reopen half of the D11 node test (amended, dated), and a new node test pins the fast clock case.
**Status:** fixed: requires human verification (logic change; the lock now rests on the device clock).

### WR-01: A screen reader never heard the wait sentence on a locked opening

**Files modified:** `web/templates/settings.html.tmpl`, `web/static/js/settings.js`, `web/settings_rows_contract_test.go`, `web/settings_username_test.mjs`
**Commit:** dc048b50
**Applied fix:** OK carries `aria-describedby="settings-username-status"`, pinned in `TestSettingsUsernameRow`. On a locked opening the status line is cleared while the window is hidden, and the sentence is written after the window is shown and before focus reaches OK. The line is therefore a real change inside a visible live region on every opening, including a second locked opening with the same text. A wait answer while the window is open still writes at once. Node tests watch the order of the writes (the line is empty at the moment the window is shown, and no write of the sentence happens while the window is hidden, over three openings).

### IN-02: A lock with an empty sentence left the window half locked

**Files modified:** `web/static/js/settings.js`, `web/settings_username_test.mjs`
**Commit:** 82407f8c
**Applied fix:** `lockUsername` returns at once with no sentence, and the live wait hook sets nothing from an empty status line. The real check never shows an empty wait line, so this is a guard. The node test drives the change hook with a stand in check and an empty line, and finds the window still normal, Enter not taken as a locked Enter, and the next opening normal.

### IN-03: The locked field could show a stale name

**Files modified:** `web/static/js/settings.js`, `web/settings_username_test.mjs`
**Commit:** 4d3cabe3
**Applied fix:** A named `usernameConfirmed` holds the name the page was drawn with, then the name a save in the page came back with. A locked opening and a save from an earlier opening show it. A lock from a live wait answer keeps what the row shows. Today the two coincide, because only a server 200 changes the row, so the change makes the rule explicit and the test pins each source by moving the row attribute away from it.

### IN-04: The "no state line on someone else's page" check looked only inside the band

**Files modified:** `internal/api/handlers/person_page_e2e_test.go`
**Commit:** cf8fa831
**Applied fix:** The check searches the whole page body.

### IN-05: The forced colours checks were not scoped to the forced colours block

**Files modified:** `web/person_band_contract_test.go`, `.planning/quick/261108-fku-profile-band-across-the-top-on-laptop-an/261108-fku-SUMMARY.md`
**Commit:** 200f21b2
**Applied fix:** A new `fkuForced` helper keeps only the rules inside a forced-colors block, and the three forced colours pins search those rules only. The C5 sentence in the SUMMARY now says the counts keep an edge because forced colours overrides a transparent border colour with a system colour, not because the older `.person-count` rule wins (it loses on specificity to `.person-band .person-count`).

### IN-06: Two settings.js branches had no node test

**Files modified:** `web/settings_username_test.mjs`
**Commit:** 0da25de3
**Applied fix:** A save sent from an earlier opening that lands while the window is open again locks that window (focus on OK, the saved name, no check sent), a first username in the same situation does not lock it, and Enter on OK is not prevented and the button's own click closes the window.

### IN-09: UsernameWaitAfterChangeMessage truncated a gap that is not whole days

**Settled by WR-02, no commit of its own.** The function is deleted with the sentence attributes, so there is no second rounding left. The one rounding left is the server's `usernameWaitDays`, which the script mirrors.

## Skipped Issues

None.

## Left as they are, on instruction

- **IN-01** (`aria-readonly` repeats the native `readonly`): kept. The node helper still checks it.
- **IN-07** (the phone byte identical proof lives only in the scratch folder): no change.
- **IN-08** (laptop reading order differs from the visual order): no change.

## Notes for the owner

- **The lock follows the device clock.** The page compares the server's deadline with the device's clock. A clock set wrong shows a wrong day count, or opens the window early or late. The server still decides on save, and a refusal reads as the wait sentence, so nothing wrong is stored.
- **The page now carries the end of the caller's own wait.** From `data-wait-until` the page can tell when the account last changed its username (the end minus 7 days). It is the caller's own account on their own `no-store` page. The SUMMARY's "Threat Flags: None" line and its Task B bullets (which still name `data-wait-message`, `UsernameWaitLine` and `UsernameWaitAfterChangeMessage`) are therefore out of date. Only the C5 sentence of the SUMMARY was edited, as asked.
- **Another tab changing the name.** The window then shows the stored name this tab last knew (reload to see the new one), opens normally, and locks on the first wait answer of the live check. That is the cost of not keeping a deadline from a rounded answer.

## Deviations

- No nested worktree, recovery sentinel or fast forward tail, as the caller's instructions said to work only in the given worktree. Commits are path limited with the Opus 5.5 trailer and nothing was pushed.
- No browser check. The behaviour is covered by the node tests with a hand moved clock and by the handler test that reads the rendered attribute.
- One scratch file was briefly written in the scratchpad root by mistake and moved, then deleted. Every other helper file is in the `fku-fix` folder.
- `gofmt -l` lists three files this work did not touch (`internal/api/handlers/place_test.go`, `internal/geocode/photon.go`, `internal/store/comments_test.go`). They were already unformatted.

---

_Fixed: 2026-11-08_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
