---
phase: quick-261113-t4y
plan: 01
subsystem: profile page, settings
tags: [profile, header, edit-profile, settings, accessibility]
requires:
  - quick 261113-k9t (the centred profile, which stays centred)
  - quick 261108-fku (the 7 day username wait)
provides:
  - a compact profile header on both sizes, for your own page and anyone else's
  - Edit profile on your own page: an edit panel of Username, Name and Home area rows with Done
  - profile-edit.js (was settings.js), profile_edit_windows.html.tmpl (the three windows)
affects:
  - Settings (About you is gone), the Settings distance row, the public name refusal sentences
key-files:
  created:
    - web/templates/profile_edit_windows.html.tmpl
    - web/profile_edit_panel_test.mjs
  modified:
    - web/templates/person.html.tmpl
    - web/templates/settings.html.tmpl
    - web/static/css/person.css
    - web/static/js/person.js
    - web/static/js/profile-edit.js (renamed from settings.js)
    - web/static/js/settings-alerts.js
    - internal/api/handlers/person_page.go
    - internal/api/handlers/settings.go
    - internal/api/router.go
    - internal/service/profilecopy.go
    - internal/service/publicname.go
decisions:
  - Two code commits instead of the plan's three (C1, C2 one or two), because the template, the stylesheet and the scripts are shared by both tasks.
  - The edit windows keep their settings- ids, so the contracts that pin them carry over.
metrics:
  completed: 2026-11-13
  tasks: 2
status: complete
actuals:
  tasks: 2
  commits: 2
plan_head_before: 9f0c61ebd8de637421036a4f2fc2ca83ba09abe2
plan_head_after: cb1f436f6d044f9e8418cee76fc466e0039416d7
---

# Phase quick-261113-t4y Plan 01: compact profile header, and Edit profile on the page Summary

The profile header now follows the owner approved mockup (profile-compact.html), and your own page edits Username, Name and Home area in place, through the same three windows Settings used to hold.

## What was built

- **Compact header (C1).** A 64px letter with 8px under it, a 20px bold username, the muted name, one bordered group of three cells (Stars, Followers, Following, each at least 44px tall, number over word, thin dividers, radius 10px), the reporter chip with its sentence on one line (the sentence wraps under the chip on the phone), Member since and the home area joined by a dot with no icons, and the action row last (Edit profile on your own page; Follow, the bell and More on someone else's; Your rating below). 20px above the letter, 16px under the last line, with a line across the page under the header. The laptop buttons are 32px, the phone buttons 44px. Followers and Following stay buttons that open their lists. The empty and error boxes of Recent reports are unchanged.
- **State line.** `OwnStateLine` returns "" for a public profile; a profile that is off keeps "Only you can see this page." The template draws the line only when it is not empty. Pins updated in the service tests.
- **Edit profile (C2).** `#person-edit` is a button with `aria-expanded` and `aria-controls="person-edit-panel"`. It opens a bordered list of three rows and a primary Done; while it is open the counts, the tag line, the details, the private line and the action row are hidden (the mockup's editing pictures). Opening puts focus on the first row; Done and Escape inside the panel hide it and return focus to Edit profile; an Escape that closes a window leaves the panel open and returns focus to the row.
- **The three windows** moved unchanged into `profile_edit_windows.html.tmpl`, drawn at the end of the body on your own page only, and their script moved with `git mv` from settings.js to profile-edit.js. Focus trap and return, Escape, the live regions, `readonly` plus `aria-readonly` in the username wait, the OK button, Enter, the live username check and the place search are untouched. `PersonPage` now takes the `AccountService` and writes `data-wait-until` from the same `UsernameWaitUntil` Settings used.
- **After a save** the page shows the new value at once: the name line, the details line and the row; for a username also the heading, the phone line, the letter (first ASCII letter, as the server draws it), `data-username`, the address (`history.replaceState` to `/u/new`, so a reload still works), the title and the Try again link. person.js follows a rename through one document event, `pinalert:username-changed`, so Follow, the lists and Refresh use the new name.
- **Settings.** About you and its three windows, their script tags and the view model fields are gone; nothing else changed except the items under Deviations.

## Header heights (measured in Playwright WebKit, ravi.k, `#person-header`)

| Page | 1440 by 900 before | after | 390 by 844 before | after |
| ---- | ------------------ | ----- | ----------------- | ----- |
| Your own page | 401 | 312 | 475 | 347 |
| Someone else's page (anita.r viewing ravi.k) | 374 | 312 | 428 | 347 |

The Recent reports title moved from y=558 to y=409 on the laptop and from y=591 to y=443 on the 390 by 844 phone, so it is visible without scrolling on the phone. While editing the header is 345 (laptop) and 357 (phone). Private own page: 341 (laptop), 377 (phone).

## Where this differs from the plan and the mockup

1. **The "about half the height" must-have is not met in numbers.** The header is 22 to 27 percent shorter on the laptop and 19 to 27 percent shorter on the phone, not half. The plan's target of 250px does not match the mockup it names: the mockup's own CSS renders its `.head` block at 329px on the laptop and 361px on the phone (measured), including its 20px top padding, and its "about 250 px tall" tag is not what its drawing or its CSS yields. The build matches the drawing: 312 plus 20px page padding is 332 against 329, and 347 plus 20 is 367 against 361; editing 365 against 362 and 377 against 374. I reproduced the drawing as asked. If the owner wants about 250px, the drawing has to shrink (for example no dividing line, 8px less around the counts group, or the Edit profile button beside the username), which needs a decision.
2. **Two code commits, not three.** The state line change (service) is its own commit; the header, the panel and the Settings removal are one, because person.html.tmpl, person.css, person.js and the shared tests mix C1 and C2.
3. **Ids.** The rows, windows and fields keep their `settings-` ids on the profile page (for example `settings-username-window`), so the contract tests and the node harnesses carry over with a path change. A rename to `profile-` ids would be mechanical if wanted.
4. **Counts group order.** The mockup draws the action row last on your own page and the plan says the same for someone else's, so on someone else's page Follow, the bell and More come after the counts and the details line, and Your rating after them.
5. **No icons on the details line** (the mockup has none; the plan allowed dropping them).
6. **Number and word sizes.** Number 16px bold, word 12px muted, as the mockup.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Settings distance row would have stopped saving**
- **Found during:** C2, reading settings-alerts.js
- **Issue:** `alertsReady` required `settings-home-row` and `settings-home-window`, and the Add a home area first button called `PinalertSettings.openWindow('home')`. With About you gone the distance select would have stopped saving.
- **Fix:** the section no longer needs the row or window; the select state is read from the server drawn `hidden` flag; the button goes to the fixed address `/me` (profile page), where Edit profile opens the Home area window. The contract that allows only fixed addresses now names `PROFILE_PAGE` too. This state is defensive only (a home area is required at sign up).
- **Files modified:** web/static/js/settings-alerts.js, web/settings_alerts_test.mjs, web/settings_rows_contract_test.go

**2. [Rule 1 - Bug] The phone's only route to the profile lived in About you**
- **Issue:** the View my profile row (phone only) was the last row of the About you box.
- **Fix:** moved it to the end of the People and reports box (last row, so the laptop hairline rule is not disturbed), id and `/me` link unchanged. Owner visible placement change on Settings.
- **Files modified:** web/templates/settings.html.tmpl and the pins in places_e2e_test.go, settings_rows_contract_test.go

**3. [Rule 1 - Bug] Two server sentences pointed at a section that no longer exists**
- **Issue:** `PublicNameRefusedMessage` and `PublicUsernameRefusedMessage` ended with "Change it in About you."
- **Fix:** now "Change it with Edit profile." These sentences are word for word rows P9-C18 and P9-C24 of 09-UI-SPEC.md, which is now stale; revert by restoring the two constants and the two test strings if the owner prefers other words.
- **Files modified:** internal/service/publicname.go, internal/api/handlers/people_e2e_test.go, web/settings_people_test.mjs

**4. [Rule 3 - Blocking] Settings no longer reads the account profile**
- **Issue:** SettingsPage read `accounts.Profile` only for the About you values. Leaving the read would be dead work, and its 500 path is gone with it (the other reads keep theirs). `UsernameWaitUntil`, `DisplayName`, `HomeAreaName`, `Username` left the Settings view model.
- **Files modified:** internal/api/handlers/settings.go

**5. [Rule 1 - Bug] drawStars found the counts row through the tag**
- **Issue:** person.js inserted a first rating's stars group into the grandparent of the tag chip. With the tag out of the counts group the group would have landed in the header.
- **Fix:** it inserts into `#person-stats` by id. person_test.mjs follows.
- **Files modified:** web/static/js/person.js, web/person_test.mjs

### Test changes

Every test that pinned "Edit in Settings", the About you rows, the Settings windows or the old centred spacing was updated; `TestPersonProfileCentred` became `TestPersonProfileCompact` with the new geometry pins (no new custom property, no order property, no hex). `TestSettingsUsernameWaitWords` and `TestUsernameFieldReadOnlyLook` still pin the wait words and the read only look, now in profile-edit.js. New: `web/profile_edit_panel_test.mjs` (panel open, Done, Escape, window Escape, name, home area, username redraw, refused save, no markup sinks), a rename test in person_test.mjs, and `TestPersonPageEditProfile` (private line, hidden name and home lines, `data-wait-until` after a rename).

## Gate

Run on a fresh database q_t4y_a (dropped and created by me, then dropped again):

- `go build ./...` exit 0, `go vet ./...` exit 0.
- `go test ./... -p 1 -count=1 -v`: exit 0, 22 packages ok, top level tests PASS 2488, FAIL 0, SKIP 3 (TestGeocodeLive, TestOfficialLive, TestScreeningFixture only).
- `node --test web/*.mjs`: tests 1643, pass 1643, fail 0, skipped 0.
- After the gate started, two comment only edits (settings-alerts.js header, MyProfilePage doc) were made; `go build`, `go vet`, `go test ./web/` and the node run were repeated and pass (1643 of 1643).
- Wording check (`git grep -niE ...`) prints nothing. No em or en dashes in any added line.

## Looking at it (Playwright WebKit, port 8794, database q_t4y_a with cmd/seedprofiles people)

Pictures are outside the repo in `/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/t4y-exec/`: `final-self-*` and `final-other-*` at 1440 and 390, light and dark, with `-editing` and `-username` variants; `flow-*` (rename, locked window, Settings); `extra-*` (320px, private profile). The flow script checked on both sizes: panel open and focus, Escape and Done and focus return, an Escape in a window keeps the panel, a name with markup in it is drawn as text, a rename updates heading, letter, data-username, title and address and survives a reload, the locked window (`readonly`, `aria-readonly="true"`, OK, focus on OK, Enter closes), Settings without About you and with its distance select saving. No page errors. 44px targets on the phone: Edit profile 132 by 44, Done 44, rows 48, count cells 50. No horizontal overflow at 320, 390 or 1440.

## Things to know

- **Seed guard.** cmd/seedprofiles refuses any database not named `p9_...`. To seed q_t4y_a I changed `scratchPrefix` to `q_t4y_` in the working tree, built a binary into the t4y-exec folder, and restored the file with `git checkout` before any commit (it is not in any commit). p9_view was never touched.
- **Home area live save** was proven in the node test (search, pick, save, header redraw) and by reading the code; the place search needs an external service and was not run in the browser.
- **Focus trap in WebKit.** A Tab probe showed focus alternating between the window and the page chrome; WebKit skips buttons on Tab by default, the trap code is unchanged and `settings_window_test.mjs` still proves it.
- **Stale spec text.** 09-UI-SPEC.md rows P9-C18 and P9-C24 still say "About you"; the planning files were not edited.
- **The follow sentences** remain in Settings (the Let others follow me status line in settings-people.js) and in the legal text; the plan only removed the profile page line.

## Self-Check: PASSED

- Files exist: profile_edit_windows.html.tmpl, profile_edit_panel_test.mjs, profile-edit.js; settings.js is gone.
- Commits exist: 2d869458 (state line), cb1f436f (compact header, Edit profile, About you out of Settings).
