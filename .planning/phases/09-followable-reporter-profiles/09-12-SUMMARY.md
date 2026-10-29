---
phase: 09-followable-reporter-profiles
plan: 12
subsystem: settings
tags: [people, profile, settings, switch, lists, menu, html, js, css]
requires:
  - phase: 09-04
    provides: PeopleService.Counts and the people counts
  - phase: 09-07
    provides: PATCH /api/me/visibility, the lists routes, GET /api/me/blocked
  - phase: 09-11
    provides: the people_list_window partial and PinalertPeopleList.open
provides:
  - Settings, People and reports with the Let others follow me switch, Following, Followers, Muted reports and Blocked people rows
  - the turn off confirm box and the refusal and failure lines of the switch
  - My profile as the first item of the laptop person menu and View my profile as a phone row of About you
affects: [09-13, 09-14, 09-15]
tech-stack:
  patterns:
    - "the switch row copies the Critical reports switch row: a label around a native checkbox with role switch"
    - "one list window for the three rows, opened with own true and the username the server wrote into the page"
key-files:
  created:
    - web/static/js/settings-people.js
    - web/settings_people_test.mjs
    - internal/api/handlers/settings_people_e2e_test.go
  modified:
    - internal/api/handlers/settings.go
    - internal/api/router.go
    - internal/service/personprofile.go
    - web/templates/settings.html.tmpl
    - web/templates/account_header.html.tmpl
    - web/static/css/places.css
    - web/places_contract_test.go
    - web/settings_rows_contract_test.go
    - web/layout_rules_contract_test.go
    - web/noise_words_contract_test.go
    - web/ways_in_contract_test.go
    - internal/api/handlers/places_e2e_test.go
    - internal/api/handlers/ways_in_e2e_test.go
key-decisions:
  - "PeopleService.MyProfileState (one read of the caller's own username and switch state) is the accessor the plan left open. It treats an account with no username as an empty name, not an error."
  - "The username for the list windows is data-username on a wrapper of the People and reports box, written by html/template."
  - "A person with no username gets no Followers or Following window (nothing to read); Blocked people still opens."
requirements-completed: [PROF-01, PROF-03]
status: complete
completed: 2026-10-29
---

# Phase 9 Plan 12: Settings people rows and My profile Summary

People and reports in Settings now holds, in the owner's order and on both sizes, Let others follow me (a native switch, off by default, saved at once), Following, Followers, Muted reports (unchanged) and Blocked people; the three list rows open the shared list window with their counts, My profile is the first item of the laptop person menu, and View my profile is a phone only row of About you.

## What was built

- **settings.go and router.go**: `SettingsPage` takes the `PeopleService`. The view model gains `ProfilePublic`, `FollowingCount`, `FollowersCount`, `BlockedCount` and `MyUsername`. A failed read is a logged 500 with ids only, like the muted count.
- **personprofile.go**: `OwnProfileState` and `PeopleService.MyProfileState` read the caller's username and switch state in one query.
- **settings.html.tmpl**: the section line is "Your public profile and the lists that go with it."; rows `#settings-profile-switch` (label row, checkbox, role switch), `#settings-following-row`, `#settings-followers-row`, the built Muted reports row, `#settings-blocked-row`, each count in `place-row__value`; `p#settings-people-error` under the box; About you gets `a#settings-view-profile-row` to /me; `dialog#settings-profile-off-confirm` built like the Log out box; the `people_list_window` partial at body level; person.css, people-list.js and settings-people.js linked. No hint text under any row, and no Muted people row (a later plan).
- **account_header.html.tmpl**: My profile (`href="/me"`, `auth-icon--user`) is the first menu item, so the menu has three `role="menuitem"` entries.
- **settings-people.js** (`window.PinalertSettingsPeople`, textContent only): the switch sends PATCH /api/me/visibility once; 200 says "Your profile is on. People can follow you while you hold the Reliable reporter tag." or "Your profile is off." (and sets Followers to 0); 422 or 429 restores the old value and shows the server's sentence; any other failure restores and shows "Couldn't save that. Try again."; offline sends nothing and shows the built offline line. Turning it off while Followers is above 0 opens the box first, Cancel and Escape leave it on, Turn off saves, a failed save puts it back on. The three rows call `PinalertPeopleList.open` with kind following, followers or blocked, `own: true`, the username from the page and an `onChange` that reads the counts again.
- **places.css**: `.place-row.place-row--phone-only` is hidden at 900px and up.

## Tests

Database tests ran on the scratch database `p9_12_a` (created, dropped at the end), each seen with `-v`, zero SKIP in the touched tests:

- `TestSettingsPeopleRows` (order, counts 2 / 1 / 1, switch unchecked and a native role switch, View my profile link, menu text "My profile Settings Log out", no follow posts row) and `TestSettingsProfileSwitchSaves` (refusal keeps it off, on shows checked, off with a follower ends the follow and shows 0 Followers, and the own profile read the script uses answers 200 while the profile is off).
- `node --test web/settings_people_test.mjs`: 15 tests pass. `node --test web/*.mjs`: 1509 pass, 0 fail.
- `go test ./internal/... ./web/ ./cmd/... -p 1 -count=1` on `p9_12_a`: every package ok. `go build ./...` and `go vet ./...` clean.
- `TestNotificationsNoFollowOrOfficialRow` is not edited and passes.

Amended with dated notes (keeping every other assertion): `TestPersonMenuItems`, `waitingSettingsLabels` (now empty), `TestSettingsShowsOnlyExistingRows` (chevrons 13 to 17), `TestPeopleAndReportsSection` (new line, four button rows), new `TestPeopleAndReportsRowOrder`, `TestSettingsRows`, `TestSettingsBackAndNoGear`, `TestSettingsAboutYouValues`, `TestAccountMenuItems` (ways_in), `TestAccountMenuRendersPerPage`, the `sizeOnlyNamed` entry for `.place-row.place-row--phone-only`, `phaseScripts` and `noiseCheckedFiles`.

## Commits

- 80370c35 feat(09-12): give Settings the profile switch state and the people counts
- 2abb6c51 feat(09-12): add the people rows, the follow switch and My profile
- e5f31f7f test(09-12): amend the pins that waited for Phase 9

## Deviations from Plan

**1. [Rule 3 - Blocking] Files outside the plan's list were edited.** `internal/service/personprofile.go` (the accessor the plan said could be "a small accessor"), `internal/api/handlers/ways_in_e2e_test.go` (`TestAccountMenuRendersPerPage` pinned the two item menu text on the main and low pages) and the `TestPeopleAndReportsSection` and `TestAccountMenuItems` pins, which the plan did not name but which fail with the new rows. Each carries a dated note.

**2. Commit order.** The Task 1 end to end tests assert the rows the template draws, so `settings_people_e2e_test.go` is committed with Task 2 (commit 2abb6c51), where it first passes. Between commits 2abb6c51 and e5f31f7f the older pins fail; the third commit amends them. Each commit builds.

**3. touchControls gets a comment, not new entries.** The switch row, the three list rows and View my profile are all `.place-row`, already in `touchControls` with a 52px min-height, so a second entry would name no rule. A dated comment records this.

**4. [Rule 2] The server sentence is also shown for a 429.** The plan names 422 only; a rate limit answer carries its own wait sentence, and showing it is better than the generic line.

**5. Environment.** `env -i` is refused by this agent's sandbox, so database tests ran as `export DATABASE_URL=...; go test ...` after confirming DATABASE_URL and PGDATABASE were empty (the project rule's own form).

**6. Hook notice.** A context hook said usage was at 96 percent several times while the token counter showed about 14.8 million tokens left. It did not match real usage, so work continued.

## Notes for later plans

- Muted people is not drawn: it needs a kind in people-list.js (UI-SPEC 4b, 4c) and a row between Muted reports and Blocked people. `TestPeopleAndReportsRowOrder` bans the words "Muted people" in the section, so the plan that adds it must amend that test.
- The Blocked people count in Settings can exceed the rows the list shows (09-04 deviation 2, a blocked person whose new username is still being checked). After a list change the count is read again from /api/me/blocked, so it then matches the rows.
- This plan stores no new personal data, so the Policy text is unchanged. It already says the Name is shown only if you let others follow you; whether it needs a line about follows and blocks is for the phase's Policy plan.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: T-09-44 (off by default, saves only on a tap, Name and username screened by the server), T-09-45 (the confirm box before followers are ended), T-09-46 (html/template escaping and textContent only, TestNoMarkupSinksAnywhere).
