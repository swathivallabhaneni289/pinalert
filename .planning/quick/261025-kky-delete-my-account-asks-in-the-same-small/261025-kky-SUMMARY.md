---
phase: quick-261025-kky
plan: 01
subsystem: web (Settings page)
tags: [settings, delete-account, confirm-box, dialog]
requires: [Log out confirm box (dialog#logout-confirm, .logout-confirm classes)]
provides: [Delete my account asks in a native dialog like Log out]
affects: [web/templates/settings.html.tmpl, web/static/css/auth.css, web/static/js/settings-alerts.js]
key-files:
  modified:
    - web/settings_alerts_test.mjs
    - web/settings_rows_contract_test.go
    - web/templates/settings.html.tmpl
    - web/static/css/auth.css
    - web/static/js/settings-alerts.js
    - .planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-UI-SPEC.md
decisions:
  - "Delete box is dialog#settings-delete-confirm.logout-confirm placed after main and before the Name window, so settingsWindowRegion never swallows it"
  - "Button label is Delete (not Delete my account) so two half width buttons fit at 320px"
  - "Focus is put back on the question after a failure, because showModal's own focusing can land on Cancel"
metrics:
  tasks: 3
  commits: 3
status: complete
completed: 2026-10-25
---

# Phase quick-261025-kky Plan 01: Delete my account asks like Log out Summary

Delete my account on Settings now opens the same small native box as Log out (question "Delete your account?", the three sentences, Cancel beside a red Delete), with focus on the question, no handler on the dim page, Escape refused only while the delete is sent, and failures shown inside the open box.

## Commits (branch worktree-agent-aa45ed9ed277184f3)

- 2126d04 test(261025-kky): the Delete box asks like the Log out box, tests first (RED)
- 9b6fe0c feat(261025-kky): Delete my account asks in the same small box as Log out (GREEN)
- 69256f8 docs(261025-kky): UI-SPEC E3 note, Delete my account asks like Log out

## What changed

- settings.html.tmpl: the old X only Delete app-window is gone; the dialog sits just before the Name window. The Q2 comment and the settings-delete-rule paragraph moved word for word.
- auth.css: .logout-confirm gets a viewport based max-height with inner scroll; the actions row wraps; a dated block styles the box text, title gap, in-box error line and the space above the buttons. The Log out box is unchanged at normal text.
- settings-alerts.js: openDeleteConfirm, native dialog wiring (cancel event refused while deleting, close event restores focus to the row, no click handler on the dialog), window.confirm fallback with window.alert on failure, in-box failure line with re-open if the browser shut the box.
- Tests: Node fake dialog (showModal, close, open, cancel and close events) and 12 Delete tests; the X and Tab tests now run on Log out of all devices; Go TestDeleteAccountConfirmBox added and TestAccountConfirmWindows narrowed to the two remaining windows.
- UI-SPEC E3: dated amendment note.

## Results

- go build ./... and go vet ./...: clean
- go test ./web/ -count=1: ok; 502 top level PASS (529 with subtests), 0 FAIL, 0 SKIP
- node --test web/*.mjs: 654 tests, 654 pass, 0 fail, 0 skipped
- go test ./internal/api/handlers/ -count=1 -v on scratch database p32_kky_1 (dropped afterwards): ok; 280 top level PASS (637 with subtests), 0 FAIL, 0 SKIP
- RED state seen before GREEN: Node file 14 of 15 tests failing, only TestDeleteAccountConfirmBox failing in Go

## Deviations from Plan

None in substance. Small choices inside the plan's wording:

- In the window.confirm fallback failure path, focus is not moved to the question (the dialog is not shown), only the alert is raised; the plan's "finally focuses deleteTitle" is applied to the box path.
- The handlers tests were run with a plain `DATABASE_URL=... go test` rather than `env -i`, because the tool sandbox refused the `env -i` form; the inherited environment held no database variables (checked), so the effect is the same.

## Known Stubs

None.

## Self-Check: PASSED

- Commits 2126d04, 9b6fe0c, 69256f8 exist on the worktree branch; each touched only its intended paths.
- web/places_contract_test.go, account-menu.js, account_header.html.tmpl and server code are unchanged.

## Live check (orchestrator, 2026-10-25)

On a scratch copy (port 8793, database p32_kky_uat, both gone afterwards), built from the branch:

- iPhone 17 Pro simulator, iOS 26.5 Safari, driven with WebDriver touch taps:
  - The Delete my account row opens the box with focus on the question and no ring, and the page stays still.
  - A tap on the dim page or on the question leaves it open. Cancel closes it, sends nothing and puts focus back on the row.
  - It looks the same as the Log out box (360px, 18px semibold question, Cancel beside a red button), in light and dark.
  - At 200 percent text the box scrolls inside itself, and Cancel and Delete sit side by side above the Safari toolbar.
  - Delete went to /login?done=deleted, showing "Your account has been deleted.", and the account row was anonymised.
- WebKit at 1280 by 800, light and dark: the box is centred at 360px, a click on the page leaves it open, Escape closes it with focus back on the row, and Delete works.
