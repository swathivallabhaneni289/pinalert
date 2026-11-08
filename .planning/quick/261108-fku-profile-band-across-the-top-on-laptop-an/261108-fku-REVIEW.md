---
phase: quick-261108-fku
reviewed: 2026-11-08T00:00:00Z
depth: deep
files_reviewed: 18
files_reviewed_list:
  - web/templates/person.html.tmpl
  - web/static/css/person.css
  - web/static/js/person.js
  - web/templates/settings.html.tmpl
  - web/static/js/settings.js
  - web/static/js/username-check.js
  - internal/service/username.go
  - internal/service/account_username.go
  - internal/api/handlers/settings.go
  - internal/service/account_username_test.go
  - internal/api/handlers/settings_username_e2e_test.go
  - internal/api/handlers/person_page_e2e_test.go
  - web/person_band_contract_test.go
  - web/person_contract_test.go
  - web/person_test.mjs
  - web/settings_rows_contract_test.go
  - web/settings_username_test.mjs
  - web/username_check_test.mjs
findings:
  critical: 0
  warning: 2
  info: 9
  total: 11
status: issues_found
---

# Quick 261108-fku: Code Review Report

**Reviewed:** 2026-11-08
**Depth:** deep (cross file: template, stylesheet, person.js, the Settings window script, the shared live check, the service wait helpers and the SQL they mirror)
**Files Reviewed:** 18 source and test files in the diff `2d349d2d..4bb2352d` (excluding `.planning`)
**Status:** issues_found

## Summary

Both parts do what the owner asked, and I found no correctness bug that ships wrong data.

- **Part A, the band.** The phone is safe. The two wrappers are `display: contents` in the base sheet. No selector anywhere uses a child or sibling combinator on `.person-header`, and no other stylesheet names a `person-` class. On your own page the action error line now comes before the state line in the DOM, but nothing on your own page can show that error line.
- **Part A, laptop placement.** The grid places the rating and error line in implicit rows after the record, and hidden lines take no space. The More menu hangs from the right edge of a `position: relative` action row, and nothing above it sets `overflow`.
- **Part A, the `insertBefore` change.** Correct. It inserts into the band, which is `display: contents` on the phone. A stars group rebuilt by `drawStars` still lands in the record.
- **Part B, the server.** The wait is computed from the profile SettingsPage already reads for `acc.ID`, on the service clock in UTC, through the same `usernameWaitDays` that the live check and the save path use. Exactly 7 days answers empty, which matches the SQL `username_changed_at <= wait_cutoff`.
- **Part B, the read only field.** It is never judged, never pasted into and never saved. `hold` leaves `isGood()` false, and the server still enforces the wait.
- **Part B, sign up step 4.** Unaffected. The new guards key off a `readonly` attribute that nothing in step 4 sets.

The two warnings are both about the locked window. A screen reader user is never told why the window is locked. And the lock is one way for the life of the page, so its day count goes stale and can stay up after the wait is over.

The changed `.mjs` suites pass (83 tests). `go vet` is clean on the touched packages. TestPersonProfileBand, TestPersonPageHeaderOrder, TestSettingsUsernameRow and TestUsernameWaitLine pass. I did not run the e2e tests and created no database.

## Warnings

### WR-01: On a locked opening a screen reader never hears the wait sentence; focus lands on an OK with no description

**Files:**
- `web/static/js/settings.js:410-412` (openWindow calls lockUsername while the window is still hidden)
- `web/static/js/settings.js:440-447` (the window is shown after that, then focus goes to OK)
- `web/static/js/settings.js:567-588` (lockUsername writes the sentence through `hold`)
- `web/templates/settings.html.tmpl:301` (the status line, `role="status" aria-live="polite"`)
- `web/templates/settings.html.tmpl:306` (the OK button)

**Issue:** The window explains itself only through the status line, and that line is never announced on a locked opening:

- `openWindow` calls `lockUsername(usernameLock)` before `target.win.hidden = false`. So `hold()` writes the sentence into the live region while the window is `display: none`. Screen readers do not announce live region changes inside a hidden subtree, and they do not read a live region's existing text when it appears.
- On every later opening `hold()` writes the same text again while hidden, so nothing is announced then either.
- Focus then moves to `#settings-username-ok`. OK has no `aria-describedby`, and the dialog panel has only `aria-labelledby`. VoiceOver and NVDA read "Username, dialog, OK, button" and stop.
- The sentence is reachable only by tabbing back to the read only field, whose `aria-describedby` names the status line. Nothing hints that the reason is there.

The D11 path, a wait answer while the window is open, does announce, because the line changes while it is visible. Only the opening path is silent, and that is the main path (D9, owner decision 2026-11-08: "the window says the wait sentence as soon as it opens").

**Fix:** Give OK the sentence as its description. In the template:

```html
<button type="button" class="btn btn--primary" id="settings-username-ok" aria-describedby="settings-username-status" hidden>OK</button>
```

Alternatively, set `aria-describedby="settings-username-status"` on the dialog panel inside `lockUsername` and take it off again in the normal path. Add one node test assertion that OK (or the panel) carries the description while locked.

### WR-02: The lock never clears in the page, so the day count goes stale and the window can refuse a change the server would take

**Files:**
- `web/static/js/settings.js:96-103` (the lock is read once at load; the comment admits a stale lock)
- `web/static/js/settings.js:567-572` (`usernameLock` is set and `readonly` is added, and neither is ever removed)
- `web/static/js/settings.js:664-668` (the C9 lock always uses the whole 7 day sentence)

**Issue:** `usernameLock` only ever goes from empty to a sentence, and `readonly` is never removed. Two cases follow:

- **A page loaded inside the wait.** Say it loads with "in 1 day". It keeps saying "1 day" and stays locked after the wait ends.
- **A change saved in the page (C9).** Every later opening says "in 7 days" for as long as the tab lives, even days later.

Settings is `no-store`, but the owner's direction is phone first and installable. A Home Screen app or a phone tab can be brought back days later without a reload, and possibly from the back/forward cache. In that state the window tells the person something false and blocks a rename the server would accept. The only way out is a manual reload, and nothing on screen says so.

The server still decides on save, so nothing wrong is stored. That is why this is a warning and not a blocker.

**Fix:** Keep a deadline, not just a sentence:

- Render the seconds left next to the sentence, for example `data-wait-seconds="{{.UsernameWaitSeconds}}"`, computed from `usernameWaitDays`'s `left`. At load, set `waitUntil = Date.now() + seconds * 1000`. After a C9 save, set `waitUntil = Date.now() + 7 days`.
- In `openWindow`, if `Date.now() >= waitUntil`, drop the lock: clear `usernameLock`, remove `readonly` and `aria-readonly`, hide OK, show Save, and take the normal path. Otherwise redraw the day count, rounding up the way `usernameWaitDays` does.

This needs only the day count in seconds, never the change time, so the threat note in the SUMMARY still holds. If the owner prefers no new attribute, a cheaper option is to ask the live check once on a locked opening, with a cheap `state` request, and unlock on anything but `wait`. Add node tests for both edges.

## Info

### IN-01: aria-readonly repeats the native readonly, and the read only field keeps the editable look

**File:** `web/static/js/settings.js:571-572`
**Issue:** ARIA in HTML says authors should not set `aria-readonly` on an element that has the `readonly` attribute. It is redundant but harmless. Separately, the read only field keeps the same border, background and focus ring as the editable one. Only the missing Save and the sentence say it cannot be changed.
**Fix:** Drop the `aria-readonly` line, and change the `isReadOnly` test helper in settings_username_test.mjs to match. Leave the look as is unless the owner asks for a cue (fix only what is asked).

### IN-02: A lock entered with an empty sentence leaves the window half locked for the rest of the page

**File:** `web/static/js/settings.js:553-557`, `:567-583`
**Issue:** `onUsernameChange` falls back to `waitAfterChange` when the status text is empty. If both are empty, as on an older hand built page with no `data-wait-after-change`, `lockUsername('')` sets `readonly`, hides Save and shows OK, but leaves `usernameLock` empty. From then on, Enter and submit take the save path, and every later opening takes the normal path. There the live check is muted by `readonly`, so Save never comes back. This cannot happen with the shipped template, which always renders the attribute, and with the live check, which never shows an empty wait line.
**Fix:** In `lockUsername`, return early when `sentence === ''`, or fall back to a fixed sentence. Pairing the `readonly` change with WR-02's unlock keeps the two in step.

### IN-03: A D11 lock caused by another tab shows the stale name as your username

**File:** `web/static/js/settings.js:567-570`
**Issue:** Suppose a second Settings tab changed the name. In this tab, `lockUsername` fills the field from `storedValue(usernameRow)`, which still holds the old name. The window then shows the old name, read only, as the account's username, and the row under it still shows the old name too. Nothing is saved wrong, and a reload fixes it.
**Fix:** None required. Optionally say "reload to see your current username" in the plan notes, or have the check's wait answer carry the current name.

### IN-04: TestPersonPageBand's "someone else's page draws the state line" looks only inside the band

**File:** `internal/api/handlers/person_page_e2e_test.go:502-504`
**Issue:** The message says the page draws the state line, but the search runs over the band block only. Since this change, the state line on your own page sits after the band, so a regression that drew it on someone else's page after the band would pass.
**Fix:** Search the whole `other` body: `if strings.Contains(other, `id="person-state-line"`)`.

### IN-05: The forced colours assertions are not scoped to the forced colours block

**File:** `web/person_band_contract_test.go:190-192`
**Issue:** `fkuWant(t, "forced band", rules, ...)` searches every rule in every media block and takes the first one that declares the property. The `forced` slice is checked only for being non-empty. A `border-color: CanvasText` that moved into the laptop block or the base would still pass. A related point: in a forced colours theme the UA forces `border-color` to a system colour anyway, so these three rules pin intent more than behaviour. The SUMMARY's C5 reasoning is also off. The counts keep an edge because forced colours overrides `transparent`, not because the older `.person-count` rule wins, since it loses on specificity to `.person-band .person-count`. The outcome C5 wants still holds.
**Fix:** Use `p26Find(rules, "forced-colors", part)` for the three lookups, and correct the C5 wording in the SUMMARY if it is reused.

### IN-06: Two new settings.js branches have no node test

**File:** `web/static/js/settings.js:671-674`, `:486-489`
**Issue:**
- The branch where a save from an earlier opening lands while the window is open again and locks it is not exercised. The older earlier-opening tests build the page without `waitAfterChange`, so `usernameLock` stays empty there.
- The early return for Enter on OK, which relies on the native click, is covered only by the scratch browser probe.

**Fix:** Add one node test: open, type, submit, close, reopen, answer the save 200, then assert the open window is locked and focus is on OK. Add a second that fires keydown Enter with `target: okButton` and asserts it is not prevented.

### IN-07: The phone byte identical proof lives only in the scratch folder

**File:** `web/person_band_contract_test.go:151-184`
**Issue:** The repo pins only the base declarations. The 10 screenshot pairs that prove D1 and D7 are outside the repo, as the no images rule requires. A later base rule that names a header child with a combinator could change the phone without failing a test.
**Fix:** None required. Optionally add a contract assertion that no base rule head in person.css contains `.person-header >` or `.person-header +`.

### IN-08: The laptop reading order differs from the visual order (C1, already flagged to the owner)

**File:** `web/templates/person.html.tmpl:46-105`, `web/static/css/person.css:779-833`
**Issue:** On the laptop the eye reads identity, counts, actions, then the record. A screen reader reads identity, record, counts, actions. The record holds nothing focusable, so Tab order is unaffected (WCAG 2.4.3 holds). This is a 1.3.2 judgement call that C1 already put to the owner.
**Fix:** None. It is recorded so it is not lost.

### IN-09: UsernameWaitAfterChangeMessage truncates a gap that is not whole days

**File:** `internal/service/username.go:52-54`
**Issue:** `int(UsernameRenameGap / (24 * time.Hour))` rounds down, while `usernameWaitDays` rounds up. With the 7 day constant they agree. If the gap were ever 7.5 days, the after-change sentence would say 7 while the server said 8.
**Fix:** Compute it with the same rounding, for example by reusing the ceiling expression from `usernameWaitDays`.

## Checks that came back clean

- **Wrong account or wrong clock.** `profile` comes from `accounts.Profile(r.Context(), acc.ID)` (settings.go:106). `UsernameWaitLine` reads only that profile, with `s.now().UTC()`, and `Sub` on two instants is zone free. A username with no change time, and a change time with no username, both answer empty, the same as CheckUsername and saveUsername. The e2e test covers the first set (no wait), a change (7 days), 6 days and an hour (1 day) and past the gap (gone).
- **The read only field is never checked or saved.** `judge` and `onPaste` return on `readonly`, and `attach` runs after `readonly` is set, so no check goes out at attach. `hold` bumps `seq`, so an answer still out is dropped, and it clears a 429 retry timer. `isGood()` stays false, so `saveUsername` returns early. The submit handler closes before reaching it. `hold` calls `onChange`, which re-enters `onUsernameChange`, but `usernameLock` is already set, so there is no loop.
- **Focus return.** OK, the X, Escape, Enter and submit all go through `closeWindow(..., true)`, which focuses the Username row. The Tab trap skips the hidden Save through `isShown`.
- **Sign up step 4.** Nothing in welcome or step 4 sets `readonly`, and `hold` is called only by settings.js.
- **C9 first set.** `before !== ''` matches the SQL CASE, which keeps `username_changed_at` on a first set. A first change, from a set name with a NULL change time, starts the wait in SQL and locks in the page.
- **The More menu.** `.person-actions` keeps `position: relative`. The band sets no `overflow`. The laptop rule (specificity 0,3,0) moves the anchor to `right: 0`. The menu (z-index 20) paints over the later state line and report rows, none of which set a z-index.
- **CSS leaking to the phone or other pages.** The base sheet gained only the `display: contents` rule. settings.html.tmpl also loads person.css, but it uses none of the classes this change styles (`person-band`, `person-record`, `person-tag-group`, `person-reports__state`). The forced colours rules set only colours, and on the phone they fall on `display: contents` boxes or on borders with no style.
- **Grid placement.** `.person-rating` and `.inline-error` carry only `grid-column: 1 / -1`, so sparse auto placement puts them in rows 4 and 5, after the record. A hidden error line is `display: none` and adds no row gap. The record's negative inline margins plus matching padding run the line to the band's inner border edge.
- **person.js insertBefore.** It now uses `els.error.parentNode` (the band), and the node fake models that parent. `drawStars` inserts a rebuilt stars group before the tag group in the record, where `.person-record > .person-stars` applies.
- **Forced colours focus.** The count buttons keep the global `:focus-visible` outline. A transparent border becomes a visible edge under forced colours, the C5 trade-off.
- **Dashes and wording.** No em or en dash in any added line.

---

_Reviewed: 2026-11-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
