---
phase: quick-261027-t5w
plan: 01
subsystem: first-time flow (welcome)
tags: [welcome, alerts, popup, dialog, onboarding]
requires:
  - quick 261027-jhd (step 2 Agree after reading, kept as is)
  - Phase 3.3 notify.js (baseState, enable)
provides:
  - four step first-time flow ("Step n of 4", four step names, four progress stops)
  - alerts pop-up after step 4 (native dialog, Log out box look)
affects:
  - Phase 5 plans that build on the step 1 markup and the welcome contract tests
  - Phase 7.1 (step 4 required): the flow ends with step 4, then the pop-up or finish
tech-stack:
  added: []
  patterns:
    - native modal dialog reusing .logout-confirm from auth.css
    - enable called synchronously in the Allow click, before anything is awaited
key-files:
  created: []
  modified:
    - web/templates/welcome.html.tmpl
    - web/static/js/welcome.js
    - web/static/css/welcome.css
    - internal/api/handlers/welcome.go
    - web/welcome_alerts_test.mjs
    - web/welcome_flow_test.mjs
    - web/welcome_contract_test.go
    - web/review33_templates_fix_contract_test.go
    - web/noise_words_contract_test.go
    - internal/api/handlers/welcome_e2e_test.go
    - .planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md
    - .planning/phases/03.4-app-layout/03.4-UI-SPEC.md
decisions:
  - "D-A no supporting line under the pop-up title"
  - "D-B the pop-up decision reads baseState only, no deviceState request"
  - "D-C no dialog support means no pop-up and a direct first-run-done, no window.confirm fallback"
  - "D-D the pop-up stays open, both buttons disabled and aria-busy, while enable and first-run-done run; a 204 leaves the page with it open"
  - "D-E Escape is handled on the dialog cancel event; nothing is sent from a close event"
  - "D-F after the pop-up was answered once, a later Continue or Skip sends first-run-done directly"
  - "D-G welcome.go drops HasHomeArea and AlertExample; service.AlertExample stays in Go for pushcopy_test.go"
status: complete
metrics:
  tasks: 3
  commits: 4 plus this summary
  completed: 2026-10-27
---

# Phase quick-261027-t5w Plan 01: Alerts ask becomes a pop-up after step 4 Summary

The first-time flow is four steps everywhere, and the alerts question is a small Log out style pop-up that opens after step 4 only on devices that can turn alerts on; Allow calls `enable` inside the tap and either answer finishes sign up.

## What was built

- **Template** (`web/templates/welcome.html.tmpl`): steps 1 to 4 read "Step n of 4"; the fifth section is gone; `auth.css` is linked between `main.css` and `chrome.css`; the pop-up is a `<dialog id="welcome-alerts-ask" class="logout-confirm">` that is a direct child of body after the `#welcome` root, with the title "Get alerts for critical reports near you?" (tabindex -1, no autofocus), then Not now and Allow, and no supporting line.
- **Script** (`web/static/js/welcome.js`): `canAsk`, `openAsk`, `endFlow`, `onAskAllow`, `onAskLater`, `onAskCancel`, `closeAsk`, `setAskDisabled`, `doneFailed`, `refocusOpener`. `finish` ends through `endFlow` at both places it used to move to step 5. `postDone` is still the only sender of first-run-done. All step 5 code is removed. Step 2's Agree after reading code is untouched.
- **Styles** (`web/static/css/welcome.css`): progress stops 25, 50, 75 and 100 percent; the step 5 rules (status line clip, alerts part, example card, `.btn[hidden]`) are removed; nothing added for the pop-up.
- **Handler** (`internal/api/handlers/welcome.go`): `welcomeSteps` has four names; `HasHomeArea` and `AlertExample` are gone from the view model.
- **Docs**: dated owner amendment in `03.3-UI-SPEC.md` with superseded markers and the new copy row P33-C22; one pointer sentence in `03.4-UI-SPEC.md`.

## Commits

| Commit | Message |
|--------|---------|
| a37ecaa | test(quick-261027-t5w): failing tests for the alerts pop-up after step 4 (RED) |
| 20e4d51 | test(quick-261027-t5w): fix two test bugs found while building the pop-up |
| e7c2362 | feat(quick-261027-t5w): the alerts ask is a pop-up after step 4 and the flow has four steps (GREEN) |
| 3b81de0 | docs(quick-261027-t5w): owner amendment 2026-10-27, the alerts ask is a pop-up after step 4 |

## RED run

Against the unchanged production files, `go test ./web/` failed TestWelcomeFourStepsInOrder, TestWelcomeStepsAreFreelyNavigable, TestWelcomeScriptsAndStyles, TestWelcomeAlertsAsk and TestWelcomeAlertsScript. The Node files failed 100 of 100 cases, because welcome.js throws at load against a four step page (the old script wires step 5 buttons that no longer exist), so the step 2 cases failed for that reason too rather than staying green. Handlers e2e tests were not run at RED (they need the database; they ran in Task 3).

## Deviations from Plan

### Auto-fixed issues

**1. [Rule 1 - Test bug] Fake page returned undefined, not null, for a missing id**
- **Found during:** Task 2 (making Task 1 pass)
- **Issue:** `welcome_alerts_test.mjs` asserted `byId(...) === null` for ids that no longer exist; the fake `doc.byId` map yields `undefined`.
- **Fix:** assert `undefined`.
- **Commit:** 20e4d51

**2. [Rule 1 - Test pins] Script pins named a parameter the build does not use**
- **Found during:** Task 2
- **Issue:** the RED contract pins named `endFlow(opener)` and `openAsk(opener)` and expected `setFinishing(false);` inside `postDone`. The build passes the pressed button through a click-time variable (so the existing `finish(save)` pin in TestWelcomeStepsAreFreelyNavigable keeps its signature), and `postDone` hands failures to `doneFailed()`.
- **Fix:** pins now read `endFlow()`, `openAsk()` and `doneFailed();`. Same commit as fix 1.
- **Commit:** 20e4d51

**3. [Rule 3 - Blocking] Card count pin**
- **Found during:** Task 1
- **Issue:** `TestWelcomeFiveStepsInOrder` pinned seven `welcome__card` items (step 5 held one); removing step 5 makes it six. The plan did not list this number.
- **Fix:** pin is six, with a dated comment.
- **Commit:** a37ecaa

**4. [Rule 3 - Blocking] Unused import in welcome_e2e_test.go**
- **Issue:** removing the step 5 e2e test left `pinalert/internal/service` unused in that file.
- **Fix:** import removed. **Commit:** a37ecaa

### Plan items and how they were met

- The banned word list near line 70 of `web/welcome_contract_test.go` ("Demo", "look around", "looking around") is unchanged.
- `#welcome-continue-1` and `.welcome__actions--pair` markup are unchanged. Settings files, `device_guide.html.tmpl` and `device-guide.css` are untouched.
- `internal/api/handlers/place_test.go` shows in `gofmt -l`; it was already unformatted and is out of scope.

## Verification

- `go build ./...` and `go vet ./...`: clean.
- `node --test web/*.mjs`: 791 tests, 791 pass, 0 fail (the alerts file has 30, the flow file 70).
- `go test ./web/ -count=1`: ok.
- `go test ./internal/api/handlers/ -p 1 -count=1` on a fresh `p_quick_t5w_a`: ok (72.8 s). The database was dropped afterwards.
- No em or en dashes in any added line. No server was started and no image was saved.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path or file access. The pop-up copy is static template text and welcome.js only toggles `disabled`, `aria-busy` and calls `showModal`, `close` and `focus`.

## Notes for the owner and the orchestrator (human check after the merge)

- **iPhone simulator:** the pop-up appears after step 4 only in the installed app (Home Screen). In a Safari tab, step 4 goes straight to Nearby with no pop-up, by design.
- **Mac Safari:** the pop-up should look like the Log out box, no button looks chosen on open, Escape lands on Nearby, a click on the backdrop does nothing, and the page behind does not scroll. Also confirm that steps 1 to 4 look the same as before the `auth.css` link was added (the plan's grep check found only auth scoped selectors; a computed style comparison was not run because no server was started).
- **Phase 7.1** (planned, not built) makes step 4 required; it already knows the flow ends with step 4, then the pop-up or finish.
- **Untracked `pinalert-browser-checks/`** was not edited. It still mentions step 5 or "of 5" in `run.js`, `README.md` and `checks/test18.js`, so those checks will need updating before they are run against the new flow.
- **Phase 5 plans** that build on `#welcome-continue-1`, `.welcome__actions--pair` and the banned word list: all three are unchanged.

## Self-Check: PASSED

- web/templates/welcome.html.tmpl, web/static/js/welcome.js, web/static/css/welcome.css, internal/api/handlers/welcome.go: modified, committed in e7c2362.
- Commits a37ecaa, 20e4d51, e7c2362, 3b81de0 exist in `git log`.
