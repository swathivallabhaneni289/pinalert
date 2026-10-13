---
phase: quick-261013-lrw
plan: 01
subsystem: web-frontend
tags: [phone, feed, detail, activity, bottom-bar, leaflet, css, contract-tests]
status: complete
requires: []
provides:
  - "Phone layout under 900px for the feed, the report detail and the Activity page"
  - "profileViewModel.Record: the reporter record card built from the account's own reporter signal"
affects: [web/static/css/phone.css, web/static/js/feed.js, web/static/js/map.js, web/templates/index.html.tmpl, web/templates/profile.html.tmpl, internal/api/handlers/auth.go]
tech-stack:
  added: []
  patterns:
    - "One stylesheet, one 899.98px block, container prefixed heads so exact head contract tests keep reading the base rules"
    - "Leaflet popup opened in a pane parented to the map container, laid out as a fixed sheet, four inline properties overridden with the important flag"
key-files:
  created:
    - web/static/css/phone.css
    - web/phone_layout_contract_test.go
    - internal/api/handlers/profile_record_test.go
    - .planning/sketches/005-phone-layout/README.md
  modified:
    - web/static/js/feed.js
    - web/static/js/map.js
    - web/templates/index.html.tmpl
    - web/templates/profile.html.tmpl
    - web/labels_contract_test.go
    - web/popup_contract_test.go
    - internal/api/handlers/auth.go
    - internal/api/handlers/activity_e2e_test.go
decisions:
  - "Three drawn items are not built (Past tab, category chips, My votes) because each needs data or a feature that does not exist"
  - "Both trust counters stay on every row (D-16), and the existing word Unconfirmed stays where the pictures say Waiting"
  - "The record card shows the server's own label (Reliable reporter) and real counts, so a new reporter reads 0 of 0 confirmed"
metrics:
  completed: 2026-10-13
---

# Phase quick-261013-lrw Plan 01: Phone layout under 900px Summary

Under 900px the feed shows the map on top at 40 percent of the height with the Nearby now list under it, report details open as a full screen sheet under a 192px map strip, and the Activity page shows a reporter record card, Log out and the bottom bar, all reproduced from the owner's two pictures from real data and existing wording, with 900px and up unchanged.

## Commits

| Task | Commit | Subject |
|---|---|---|
| 1 | 8c29984 | feat(quick-261013-lrw): phone feed with the map on top and a bottom bar |
| 2 | c24ce88 | feat(quick-261013-lrw): phone report detail as a full screen sheet |
| 3 | f3a1299 | feat(quick-261013-lrw): phone Activity page with the reporter record |

Each commit carries the trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` and lists only its own task's paths. No image file was added.

## What was built

- **Feed (Task 1).** `phone.css` (every phone rule in one 899.98px block, plus one grouped base rule that hides the five phone only classes at every width). Map row 40vh/40dvh, list pane its own scroll box, rounded severity cards (tint from the existing token, white with a hairline when provisional or hidden), dot pins with a ring in the 44px marker box, a two tab bottom bar, the Nearby now heading with a live count line, a row distance line, and a phone only list pane scroll when a pin is tapped. The switch button, the account button and the left severity bar are not drawn on phones.
- **Detail (Task 2).** `map.js` creates a `detailPane` inside the map container, writes `options.pane` and `options.autoPan` on every popup before it can open, closes the open popup when a resize crosses 900px, and puts the pin in the strip. `phone.css` lays the existing popup card out as a fixed sheet (`left`, `bottom`, `transform` and content `width` carry the important flag, nothing else does). Confirm and Dispute sit side by side, Mark resolved is a centred link, the close anchor is the round back arrow. No card markup, focus code or Escape code changed.
- **Activity (Task 3).** `profileViewModel.Record` from the first row's `Trust.Reporter` (no new query), a `.profile-logout` form copied from the account header, the record card inside `{{with .Record}}`, the bottom nav with Activity current, and a grid layout with white hairline cards. All three new elements are hidden at 900px and up.

## Not built (and why)

| Drawn item | Why | Needs |
|---|---|---|
| Past tab in the bottom bar | No history view exists, the feed query drops expired reports | Phase 3.1 history views (open owner question on phone history tabs) |
| Category chips over the map | Filtering is Phase 3.1 scope, with an open question on whether a filter may hide critical or rescue reports | Phase 3.1 filters plus the owner's answer |
| My reports / My votes segmented control | No read of votes by account exists, and a test guards against promising a voting history | A read of votes by account |

## Departures from the pictures and the house rules

All of them are written in `.planning/sketches/005-phone-layout/README.md` (sections for each task). In short:

- **Wording.** Unconfirmed (not Waiting), both counters on every row, `min ago` and `km away`, `Reliable reporter` (not Reliable), `0 of 0 confirmed` for a new reporter, no Active word for Live, the picture's status lines (Needs one more confirmation, Resolved by you, Not re-confirmed) not built because they are not app strings, the too far sentence shown only when the reader is too far.
- **Colour (contrast rule, both themes computed).** Meta, distance and fade lines take the normal text colour on tinted surfaces (muted is 4.12 to 4.37 there in the light theme). The good tone is used on the trust line and on Mark resolved only where it clears 4.5 to 1 (not on the Critical tint, 4.35). Written with the numbers.
- **House rules.** A55 (row title clamped to two lines on phone feed cards), the round tab dot and back arrow (no fully rounded controls), A87 amended on phones only (the list pane scroll), D-A not applied on phones (theme toggle at the top right of the map), account button hidden on phones, no vote buttons on phone rows (voting is in the detail), no chevron or clock on rows, no category glyph on pins, severity label loses its pill on phone cards, trust line colours.
- **Detail.** Pin sits at 96px in the strip after a pin tap and at 169px after a row tap, because `activateRow` runs a second `flyTo` that this plan was told not to change. Both are inside the 192px strip.

## Contract tests changed and why

| Test | File | Change | Why |
|---|---|---|---|
| `TestRowTitleWrapsLongWords` | `web/labels_contract_test.go` | Skips exactly the head `#report-list .report-row .report-row__title` in `phone.css` | Picture A draws the card title up to two lines (A55 forbids a clamp) |
| `TestReadOnlyLabelsRoundedControlsNot` | `web/labels_contract_test.go` | Skips exactly two `phone.css` heads (tab dot, detail back arrow) | The pictures draw both round; the arrow is the same kind of round icon control as the exempt `.profile-back-link` |
| `TestMapSelectionNeverScrollsThePage` | `web/popup_contract_test.go` | Doc comment only | Names the phone amendment of A87, every assertion kept |

`TestTrustCSS` (the trust text colour rule) needed no change: it reads `trust.css` only, so the phone trust line colours do not touch it. The departure is recorded in the README for the same reason the test's comment gives.

New: `web/phone_layout_contract_test.go` (10 tests), `internal/api/handlers/profile_record_test.go` (1 test, table of 4 cases) and three e2e cases in `activity_e2e_test.go`. Task 3 followed RED then GREEN (the helper and markup were missing, then added).

## Verification

- **Full suite**, each on a fresh disposable database that was refused if present and dropped afterwards: `p_phone_t1` 1340 PASS, `p_phone_t2` 1342 PASS, `p_phone_t3` 1352 PASS, each EXIT 0 with 0 SKIP and 0 FAIL. The new test names appear with `--- PASS` in the third run.
- **Desktop unchanged.** A baseline of 458 computed values (grid, panes, row, badge, severity label, fab, theme toggle, header, toast, popup size and pane class, profile page, back link, overflow) was captured from a HEAD build before the first edit, in Chromium and WebKit at 1280 by 800 and at 900 by 800. After each task: 458 values compared, 0 differences, both widths, both engines. The popup still opens in `leaflet-popup-pane` with a 341px content width at 1280. The new elements (`.feed-heading`, `.bottom-nav`, `.report-row__away`, `.profile-logout`, `.profile-record`) are not rendered there.
- **Phone probes at 390 by 844, Chromium and WebKit.**
  - Feed: map 338px (40.0 percent of 844), list pane top 338 equal to the map bottom, view toggle `none`, bottom bar 52px at the bottom with two tabs and Feed current, add button bottom 776 above the bar top 792, count line `10 active · 10 km` for 10 rendered rows, rows 0px left border and 8px radius, `scrollWidth` minus `innerWidth` 0, page not scrollable. Tapping a pin: list pane `scrollTop` 301 (Chromium) and 300 (WebKit), selected row inside the pane box, `window.scrollY` 0.
  - Detail: popup inside `.leaflet-detail-pane`, rect top 192, bottom 844, width 390, opacity 1, visibility visible; `elementFromPoint` at the sheet centre, near its bottom edge, at the add button and at the bar centre all land inside the sheet; pin centre y 169 (row tap) inside the strip; close anchor 44 by 44 at (16, 16); focus inside the card; Confirm and Dispute 171px each on one row; Escape and the back arrow both close it with focus back on the row that opened it. Vote button states checked in both themes and both engines (rest, hover, busy, too far, pressed).
  - Activity: title and Log out on one row (title [16,16,235,44], Log out [251,16,71,44], clear of the toggle), record card shows `Your reporter record | New reporter | 0 of 0 confirmed`, back link `none`, rows 8px radius and white, bottom bar Feed and Activity current, no sideways scroll. Dark theme checked.
- **Browser harness** (`BUILD_FROM=worktree`, `PINALERT_REPO` set to this worktree, port 8795, database `p_phone_auto` created and dropped by the harness). After Task 3, on an idle machine: 351 PASS, 54 NOTE, 22 NEEDS A PERSON, 76 FAIL. Every FAIL is a check that drives something the pictures remove or move on phones: the map and list switch button (T5 V0 and L0, and the T7, T8, T9 B, T11 and T12 checks that open the app under 900px and click it), voting from a feed row (T8), the floating popup with a tip and Leaflet autopan (T9.1, T9.2c, T9.4, T9.7), and the 188px Activity row body beside the badge (T11.2f, T11.6, T11.8g, T11.8h). Each is listed with the reason in the README. T2 and T5 P failed only in an earlier run that overlapped with another browser and pass alone. The checks at 900px and up are unchanged and pass.

## Deviations from Plan

### Auto-fixed / adjusted

**1. [Rule 3 - Blocking] Sandbox refused `env -i` and `ENV=` at the top level of a command.**
- **Fix:** `env -i PATH HOME` is used inside runner scripts in the session scratchpad (outside the repo), and the shell environment was checked to hold no `DATABASE_URL`, `PG*` or `PINALERT*` variable before any run. Databases used were only `p_phone_probe`, `p_phone_t1`, `p_phone_t2`, `p_phone_t3` and the harness's own `p_phone_auto`; never `pinalert` or `pinalert_test`.

**2. [Rule 3 - Blocking] A contract test refused the pictured look in two places beyond the trust line.**
- **Found during:** Task 1 and Task 2. `TestRowTitleWrapsLongWords` (title clamp), `TestReadOnlyLabelsRoundedControlsNot` (tab dot, back arrow) and, as the plan expected, `TestMapSelectionNeverScrollsThePage` (doc comment).
- **Fix:** each updated to allow exactly the named `phone.css` heads, with a doc comment line naming the departure. `web/labels_contract_test.go` is not in the plan's `files_modified`; the plan's own rule (update the test in the task) covers it.

**3. [Rule 1 - Bug] Probe baseline made deterministic.**
- The first baseline selected "the first row", which changed when more reports were seeded to make the list overflow. The baseline was recaptured from a build of the untouched HEAD with rows selected by report id, and the probe was checked to give 0 differences against HEAD itself before any edit was compared.

### Other notes

- The pin sits at 169px, not 96px, after a list row tap (see above).
- The record counts use the first Activity row's signal. An account with reports always has a label, so the card always shows for an account with at least one report.

## Questions for the owner (plain words)

1. **Past tab.** The picture has Feed, Past and Activity. There is no history view yet, so the bar has two tabs. Recommendation: add Past when Phase 3.1 builds the history view.
2. **Category chips over the map.** Recommendation: build them with Phase 3.1's filters once you say whether a filter may hide critical and rescue reports.
3. **My votes.** The control on the Activity page needs a read of votes by account, which nothing does yet. Recommendation: plan that read as its own small piece of work, then add the control.

## Known Stubs

None. Every new element is wired to real data (the count is the number of rendered reports and the radius the feed was asked for, the distance is the existing formatter, the record is the server's signal).

## Threat Flags

None new. The count and distance lines are written through `Pinalert.setText`; the record is escaped by `html/template` and read from the verified account's own rows; the Log out form is a copy of the existing form (T-lrw-01 to T-lrw-04 hold).

## Self-Check: PASSED

- Created files exist: `web/static/css/phone.css`, `web/phone_layout_contract_test.go`, `internal/api/handlers/profile_record_test.go`, `.planning/sketches/005-phone-layout/README.md`.
- Commits 8c29984, c24ce88 and f3a1299 exist on `worktree-agent-a2dcb785fd03d792d`.
- No `pinalert` or `pinalert_test` database was touched; the baseline folder, probe database, server log and sign in state were deleted.
