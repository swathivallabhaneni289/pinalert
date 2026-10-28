---
phase: 05-official-feed-demo-mode
plan: 14
subsystem: settings
tags: [settings, notifications, official-alerts, push, go, vanilla-js]
requires:
  - phase: 05-11
    provides: official_push_on column, GetOfficialPushSetting and UpsertOfficialPushSetting, DefaultOfficialPushOn
  - phase: 05-09
    provides: the Official alerts page
  - phase: 03.3
    provides: the Notifications box, saveKind, redrawAlerts, the Home area line
provides:
  - AlertSettings.OfficialPushOn and AlertSettingsUpdate.OfficialPush
  - official_push on PATCH /api/me/alerts (request and response)
  - settingsViewModel.OfficialPushOn
  - the Settings row "Severe official alerts for my area" with its no home variant
  - the Home area line W1 naming official warnings
affects: [05-16, 05-15]
tech-stack:
  added: []
  patterns:
    - "official switch follows the Critical pattern: validate every part first, write each stored row only when one of its own fields is set"
key-files:
  created:
    - internal/api/handlers/official_settings_e2e_test.go
  modified:
    - internal/service/alertsettings.go
    - internal/service/alertsettings_test.go
    - internal/service/profile.go
    - internal/api/handlers/me.go
    - internal/api/handlers/settings.go
    - internal/api/handlers/swagger_test.go
    - internal/api/handlers/places_e2e_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/templates/settings.html.tmpl
    - web/static/js/settings-alerts.js
    - web/settings_alerts_test.mjs
    - web/places_contract_test.go
    - web/settings_rows_contract_test.go
key-decisions:
  - "Official on without a home area is refused on field official_push with Add a home area first.; off never needs an area; clearing the area keeps the stored value (same as Critical)."
  - "Removing the home area while only the official switch is on shows the existing W2 status, because that switch is on by default and the row does stop alerting without an area."
  - "TestNotificationsNoFollowOrOfficialRow keeps its name (other plans and the acceptance grep refer to it) and keeps the follow absence; the new TestNotificationsOfficialRow pins the row."
patterns-established:
  - "The node page fixture takes official: false by default so every older case keeps its meaning; new cases pass official: true."
requirements-completed: []
duration: about 70 min
completed: 2026-10-29
status: complete
---

# Phase 5 Plan 14: The Severe official alerts switch Summary

**A per account switch for silent official alerts: the service field and PATCH key, the Settings row between Critical and Changes with its Add a home area first variant, and the Home area line naming official warnings. On by default; the first-time flow is untouched.**

## Tasks and commits

| Task | Commit | What |
|------|--------|------|
| 1 RED | 8aa0c9bc | TestAlertSettingsOfficialDefault, TestUpdateAlertSettingsOfficial, TestClearingHomeAreaKeepsOfficial, TestOfficialPushSettingRoute, swagger pin |
| 1 GREEN | ce58c4f1 | AlertSettings.OfficialPushOn, AlertSettingsUpdate.OfficialPush, ProfileQuerier methods, PATCH key, Settings model field, regenerated docs |
| 2 RED | 8005da8d | TestNotificationsOfficialRow, node cases, amended 3.3 pins |
| 2 GREEN | 603adf49 | template row, settings-alerts.js, W1 sentence; test slice fix in TestNotificationsOfficialRow |
| 2 follow-up | acad9a24 | TestSettingsRows (handlers) lists the new no home button |
| 3 | none | browser check and the full suite only |

## What was built

- Service: `AlertSettings` reads `GetOfficialPushSetting` (default `DefaultOfficialPushOn`, true). `UpdateAlertSettings` validates `official_push` on without a home area as a `ValidationError` on field `official_push` before any write, and calls `UpsertOfficialPushSetting` only when the field is set. No migration (05-11 owns storage).
- API: `AlertSettingsRequest.OfficialPush *bool` and `AlertSettingsResponse.OfficialPush bool` (`official_push`), swag text updated, docs regenerated with the repo's swag target (swag v1.16.4 installed locally; the diff is 29 lines, only this change).
- Settings page: `settingsViewModel.OfficialPushOn`, boolean only; the home cell never enters the page (asserted in `TestOfficialPushSettingRoute`).
- Row: `div#settings-official-row` with `label#settings-official-on` (native checkbox, `role="switch"`, no sub line) and `button#settings-official-nohome` (sub line "Add a home area first", chevron), placed between Critical and Changes, same on phone and laptop.
- Script: `settings-alerts.js` saves the switch through `saveKind('official_push', ...)`, restores the old value with "Couldn't save that. Try again." on failure, redraws the two variants with the home area, and counts the switch in the Home area line and in the W2 status.
- W1: "Removing it also turns off alerts about reports and official warnings for your home area." shown while Critical, Reliable or Severe official alerts is on.
- Welcome files: not touched (no 05-14 commit changes a file with "welcome" in its path; "Severe official alerts" stays in the welcome banned list).

## Amended tests (every moved 3.3 pin)

All carry "Amended 2026-10-29 by plan 05-14 (D-11)" in the file:

1. `web/places_contract_test.go`: `waitingSettingsLabels` loses "Severe official alerts for my area".
2. `web/places_contract_test.go`: `TestSettingsShowsOnlyExistingRows` chevron count 11 to 12 (the official no home row).
3. `web/settings_rows_contract_test.go`: `TestNotificationsNoFollowOrOfficialRow` keeps "Someone I follow posts" absent; the two Severe official strings left its banned list (name kept).
4. `web/settings_rows_contract_test.go`: `TestNotificationsSectionRows` row order, label, checked binding, switch and no sub line checks include the official row; W1 sentence replaced and the old sentence must be gone.
5. `web/settings_rows_contract_test.go`: `TestPhoneNotificationsRowsNeedAHomeArea` hidden-attribute pairs and wrapper list include the official row; "Add a home area first" count 2 to 3; the W1 template line now includes `.OfficialPushOn` and the new sentence.
6. `web/settings_rows_contract_test.go`: `TestSettingsAlertsSavesAtOnce` also requires `official_push` in the script.
7. `web/settings_rows_contract_test.go`: new `TestNotificationsOfficialRow`.
8. `internal/api/handlers/swagger_test.go`: `official_push` added to both AlertSettings definitions.
9. `internal/api/handlers/places_e2e_test.go` `TestSettingsRows` (not in the plan's file list, found by the full suite): button order gains `settings-official-nohome` between Critical and Reliable.
10. `web/settings_alerts_test.mjs`: fixture gained the official elements (default off so no older case changes meaning) plus 11 new cases.
11. `internal/service/alertsettings_test.go`: the fake querier gained the two official methods.

No assertion was removed or loosened; only counts and lists grew, and the follow absence is kept.

## Deviations from Plan

**1. [Rule 1 - Bug] TestSettingsRows (internal/api/handlers/places_e2e_test.go) failed after the template change**
- **Found during:** Task 3 full suite.
- **Issue:** The test pins the exact list of button rows on the Settings page; the new no home button is one more.
- **Fix:** Added the id to the expected order with a dated note; commit acad9a24.

**2. [Rule 3 - Blocking] The sandbox refused starting the server on 8796 with ENV=development**
- **Fix:** As the project rules say, no secret was invented. A throwaway Go test (deleted before commit, never committed) ran the real router and the real p5_14 database inside `go test`, serving the same handler on 127.0.0.1:8796, and issued session cookies through the real request-link flow. Playwright then drove real browsers against port 8796. Playwright used `serviceWorkers: 'block'` because the app's service worker bypasses `page.route`, which would otherwise hide the forced 500.

No other deviations. Task 3's welcome card is dropped per the 2026-10-27 amendment.

## Browser check (port 8796, p5_14, Playwright)

Engines webkit and chromium; 390x844 and 1470x830; light and dark: 8 combinations, 28 checks each, 224 checks, 0 failures. Per combination:

- Home area account: official row visible, label exact, switch on, `role="switch"`, no sub line, order Critical then official then Changes (box tops), no horizontal overflow, switch at least 24px, no buzz/ring/sound/vibration word in the box; a tap sends `{"official_push":false}` with 200 and a reload keeps it off; the W1 line is shown with Critical on, hidden with Critical, Reliable and official all off, shown with official alone on, and reads the new sentence; a forced 500 restores the switch and shows "Couldn't save that. Try again."
- No home area account: the row reads "Severe official alerts for my area / Add a home area first" with a chevron, the switch is hidden, the server refuses `official_push: true` with 400 on `official_push`, and the button opens the Home area window.
- First-time account: /welcome shows "STEP 3 OF 4" (the counter is "of 4"), no word "official" on the page, and dialog#welcome-alerts-ask ("Get alerts for critical reports near your home area? ... Not now Allow") holds none.

Not done here, by design: the iPhone simulator (the orchestrator's). No screenshots were saved; checks used computed-style and DOM probes.

## Verification

- Full Go suite on p5_14 (`go test ./... -p 1 -count=1 -v`), final run: 1739 `--- PASS`, 0 `--- FAIL` other than the known flake, 0 `^FAIL` other than the same package line, SKIP: TestGeocodeLive, TestOfficialLive, TestScreeningFixture (all allowed).
- Known flake `TestVotedReportsByAccountUsesTheAccountIndex` (internal/store) failed in both full runs and on a rerun by name against the used database (planner statistics); it passed on a freshly created p5_14, as the project notes say. An earlier full run also failed `TestSettingsRows`, fixed in acad9a24 and passing since.
- Node: `node --test web/*.mjs` 973 pass, 0 fail (the queue_logic WR-07 flake did not appear).
- `go vet ./...` clean. `gofmt -l` lists only internal/api/handlers/place_test.go, which was already listed before this plan.
- Acceptance greps: OfficialPushOn x6 and `UpsertOfficialPushSetting(` x1 in alertsettings.go; `json:"official_push` x2 in me.go; `official_push` x3 in swagger.json; `settings-official-row` and the label present in the template; the label is out of the waiting list; follow absence kept; welcome files untouched.
- p5_14 dropped and confirmed gone with `psql -lqt`; nothing listens on 8796; scratch files are in the scratchpad only.

## Known Stubs

None. The switch is stored and read; sending official pushes is plan 05-16.

## Threat Flags

None beyond the plan's register. T-05-76 (account from the gate only, strict JSON), T-05-77 (no home cell in the page, asserted), T-05-78 (noise words absent, asserted in TestNotificationsOfficialRow and the existing noise word contract) and T-05-79 (validate before write, TestUpdateAlertSettingsOfficial) are mitigated.

## Self-Check: PASSED

- Files exist: official_settings_e2e_test.go, alertsettings.go, settings.html.tmpl, settings-alerts.js (checked by the acceptance greps above).
- Commits 8aa0c9bc, ce58c4f1, 8005da8d, 603adf49, acad9a24 are in `git log`.
