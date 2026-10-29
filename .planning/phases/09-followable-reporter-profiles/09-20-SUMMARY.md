---
phase: 09-followable-reporter-profiles
plan: 20
subsystem: web
tags: [mute, profile, settings, lists, html, js]
requires:
  - phase: 09-08
    provides: person.html.tmpl, person.js, the More menu, the status line and the profile refetch
  - phase: 09-11
    provides: the shared list window (people-list.js) and PinalertPeopleList.open
  - phase: 09-12
    provides: Settings People and reports rows and settings-people.js
  - phase: 09-18
    provides: PeopleService.MutedPeopleCount
  - phase: 09-19
    provides: POST /api/people/{username}/mute and /unmute, GET /api/me/muted-people, muted on the profile JSON
provides:
  - Mute or Unmute as the first item of the profile More menu
  - the muted kind of the people list window
  - the Muted people row of Settings with its count
affects: [09-24, 09-27, 09-29]
tech-stack:
  patterns:
    - "one undo row for Blocked and Muted people: a table (path, button word, status sentence, failure line) drives the shared row and focus code"
    - "the menu item label and data-muted are drawn by the server and redrawn only from the refetched profile"
key-files:
  created:
    - web/person_mute_test.mjs
    - internal/api/handlers/person_mute_page_e2e_test.go
  modified:
    - web/templates/person.html.tmpl
    - web/static/js/person.js
    - web/static/js/people-list.js
    - web/static/js/settings-people.js
    - web/templates/settings.html.tmpl
    - internal/api/handlers/settings.go
    - web/people_list_test.mjs
    - web/settings_people_test.mjs
    - web/settings_rows_contract_test.go
    - web/places_contract_test.go
    - web/layout_rules_contract_test.go
    - internal/api/handlers/places_e2e_test.go
    - internal/api/handlers/settings_people_e2e_test.go
key-decisions:
  - "The Muted people count in Settings goes down by one for each Unmute done in the window instead of being read again from the list. The count can exceed the list by the people whose new username is pending, so reading the list length would drop them."
  - "The Muted people row is optional in settings-people.js (a page drawn without it still wires the other rows), unlike the three older rows."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 20: Mute screens Summary

A person can be muted and unmuted from the first item of their profile's More menu, and Settings lists everyone muted in a Muted people row that opens the shared list window with an Unmute button on every row, whether or not the person still has a public profile.

## What was built

- **person.html.tmpl**: `#person-mute-item` is the first item of `#person-more-menu`, always drawn on another person's page, labelled Mute or Unmute and carrying `data-muted`, both from the server's `Profile.Muted`.
- **person.js**: choosing the item posts `/mute` or `/unmute` at once (no box), sets the label for the action, refetches the profile and lets the server's `muted` flag overrule it, says "{username} is muted. Their reports still show." or "{username} is unmuted.", and puts focus on More. A failure shows "Couldn't mute. Try again." or "Couldn't unmute. Try again."; a 404 reloads so the server draws Not found; offline sends nothing and shows the built line. The item is disabled with `aria-busy` while the call is out. `drawMute` runs from `draw`, so any later refetch keeps the label right.
- **people-list.js**: kind `muted` (title "Muted people", GET `/api/me/muted-people`, no subtitle, empty state "No muted people" with "People you mute show here, with a way to unmute them."). The Blocked row became `undoRow`, driven by an `UNDO` table, so Muted and Blocked share one row and one focus routine. Unmute posts `/api/people/{username}/unmute`, removes the row, says "{username} is unmuted.", moves focus to the next row's button or the empty heading, and shows "Couldn't unmute. Try again." above the list on failure.
- **settings.html.tmpl, settings.go**: the `#settings-muted-people-row` button row (chevron, count in `#settings-muted-people-value`, no second line) sits between Muted reports and Blocked people. The view model gains `MutedPeopleCount`, read from `PeopleService.MutedPeopleCount` in the same logged-500 block as the other people counts.
- **settings-people.js**: the row opens `PinalertPeopleList.open({kind: 'muted', own: true, username: '', opener})`; `onChange` takes the shown count down by one (never below zero).

## Tests

- `node --test web/person_mute_test.mjs web/person_test.mjs`: 41 pass (12 new in person_mute_test.mjs: Mute and Unmute post once and say the exact sentences, the label follows the refetched flag, a refetch with no flag keeps the tap's label, Mute is the first item and Enter chooses it, a second tap sends nothing, both failure lines, network failure, 404 reload, offline, no markup sink and the template order).
- `node --test web/*.mjs`: 1590 pass, 0 fail (new in people_list_test.mjs: muted kind and rows, Unmute once with focus and empty heading, refused Unmute, empty words; new in settings_people_test.mjs: the row opens kind muted, no username needed, the count goes down by one and stops at zero, the row is optional).
- `TestPersonPageMuteItem` (new, database): the real page draws Mute first with `data-muted="false"`, Unmute after a mute, Mute again after an unmute, no item on your own page, and no sign of the mute on the muted person's view of the muter's page. Seen with `-v`.
- Amended with dated notes, every other assertion kept: `TestPeopleAndReportsRowOrder` (order, five chevrons, "Muted people" no longer banned), `TestPeopleAndReportsSection` (five button rows), `TestSettingsShowsOnlyExistingRows` (eighteen chevrons), the Settings button list in `places_e2e_test.go`, and `TestSettingsPeopleRows` (a seeded mute, the new count, "Muted people" no longer banned). Run with `-v`: `TestPeopleAndReportsRowOrder`, `TestSettingsShowsOnlyExistingRows`, `TestNotificationsNoFollowOrOfficialRow`, `TestPeopleAndReportsSection` and `TestNewControlsMeetTouchTarget` printed their names and passed.
- Full `go test ./... -p 1 -count=1 -v` on the scratch database `p9_20_a` (created fresh, dropped at the end): exit 0, 2323 passing tests, the only skips are the three known ones (`TestGeocodeLive`, `TestOfficialLive`, `TestScreeningFixture`). `go build ./...` and `go vet ./...` clean.

## Commits

- e23dcc49 feat(09-20): add Mute and Unmute to the profile More menu
- acdecd70 feat(09-20): add Muted people to Settings and the list window

## Deviations from Plan

**1. [Rule 3 - Blocking] Tests outside the plan's file list had to change.** The new row broke `TestSettingsPeopleRows` (it banned "Muted people" and counted rows) and the Settings button list in `places_e2e_test.go`, and `TestPeopleAndReportsSection` still expected four button rows. All three are amended with dated notes. `person_mute_page_e2e_test.go` is new (the plan listed only the .mjs test for the menu item) so the server drawn label is pinned against the real router.

**2. [Plan wording] touchControls got a dated comment, not new entries.** The plan said to add the new row and the Unmute button. The row is a `.place-row`, the Unmute button is `.people-row__action` and the menu item is `.person-menu__item`, all already in the list and already held to 44px by `TestNewControlsMeetTouchTarget`, so a second entry for the same class would check nothing new. No entry was added or changed.

**3. [Design choice] The Settings count steps down by one on Unmute** instead of being read again from the list, because reading the list length would drop pending names the count includes (09-18 note). The two older list rows are unchanged.

No auth gates.

## Notes for later plans

- **09-24 (the Notify me bell)**: UI-SPEC 1e says muting turns the bell off and hides it. There is no bell on the page yet. `person.js` `draw()` already receives `profile.muted` and calls `drawMute`; 09-24 should hide the bell from the same `muted` flag in that function (its own plan text already says "hidden while muted").
- The profile status sentence and focus handling are in `toggleMute`; `PinalertPerson.toggleMute` is exported for tests.

## Known Stubs

None.

## Threat Flags

None. T-09-71: every muted username reaches the page through `textContent` (`TestNoMarkupSinksAnywhere` and the `no markup sink` tests in the three .mjs files pass).

## Self-Check: PASSED

Created files exist (`web/person_mute_test.mjs`, `internal/api/handlers/person_mute_page_e2e_test.go`, this summary); commits e23dcc49 and acdecd70 are in `git log`; no non-ASCII characters in any touched file.
