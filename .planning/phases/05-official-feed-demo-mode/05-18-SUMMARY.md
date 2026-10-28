---
phase: 05-official-feed-demo-mode
plan: 18
subsystem: first-time-flow
tags: [welcome, demo, onboarding, buttons, phone, laptop]
status: complete

requires:
  - phase: 05-13
    provides: the Demo page, demoExitHref (Exit goes to /welcome only for from=welcome)
  - phase: 05-14
    provides: the four step flow with the alerts pop-up, untouched here
provides:
  - Try the demo beside Continue on step 1 of the first-time flow
  - Try the demo as the third control of the step 2 Disagree box, with the restored second sentence
affects: [05-19]

key-files:
  created:
    - web/demo_entry_contract_test.go
  modified:
    - web/templates/welcome.html.tmpl
    - web/static/css/welcome.css
    - web/welcome_contract_test.go
    - internal/api/handlers/welcome_e2e_test.go

key-decisions:
  - "Both controls are plain a elements styled by .btn, going to /demo?from=welcome; welcome.js is not changed and does not know about them."
  - "The Disagree row is a 2 column grid on the phone with the third control spanning both columns (#welcome-try-demo-2 grid-column 1 / -1), and repeat(3, max-content) on the laptop."

requirements-completed: []
duration: about 1 hour
completed: 2026-10-29
---

# Phase 5 Plan 18: The Demo inside the first-time flow Summary

A person in the first-time flow can open the Demo from step 1 (Try the demo, outlined, beside the filled Continue) or from the step 2 Disagree box (the restored sentence plus a third Try the demo button), and the Demo's Exit brings them back to /welcome. No step was added and welcome.js is untouched.

## What was built

- `web/templates/welcome.html.tmpl`: step 1's actions are now `div.welcome__actions.welcome__actions--pair.welcome__actions--demo` holding `a.btn#welcome-try-demo-1` (href `/demo?from=welcome`) and then the built `#welcome-continue-1`. The Disagree lead reads "Sorry, you need to agree before you can continue. You can still look around in the Demo without agreeing." and `.welcome__notice-actions` gains `a.btn#welcome-try-demo-2` after the Log out form. No hint text under either control.
- `web/static/css/welcome.css`: `.welcome__actions--demo .btn` takes padding `0 var(--space-md)` and `white-space: nowrap`; below 340px (`@media (max-width: 339.98px)`) the pair stacks in source order (Try the demo above, Continue last, no reordering); `#welcome-try-demo-2` spans the box on the phone and sits in the one row of three on the laptop.
- Tests: new `web/demo_entry_contract_test.go` (TestWelcomeStepOneDemoPair, TestDisagreeBoxPointsToDemo, TestWelcomeDemoLinksAreButtons, and TestWelcomeScriptIsUnchangedByTheDemoLinks); new `TestWelcomeOffersTheDemoForANewAccount` in `internal/api/handlers/welcome_e2e_test.go`.

## Amended tests (never weakened)

- `web/welcome_contract_test.go` `TestWelcomeFourStepsInOrder`: "Demo", "look around" and "looking around" left the banned list ("Amended 2026-10-29 by plan 05-18 (D-03)"). "Severe official alerts" stays banned for good and the comment above it now says official alerts are a Settings switch only (owner 2026-10-27, quick 261027-t5w).
- `web/welcome_contract_test.go` `TestWelcomeStep2ReadsTheSharedText`: the Disagree lead pin now holds the first sentence plus the second sentence (the first sentence is still counted exactly once in the page); the "<a " ban in the Disagree box became "exactly one anchor, and it is the Demo link", while the bans on 112, tel:, CallHref and Numbers stay. Dated note in place.
- `internal/api/handlers/welcome_e2e_test.go` `TestOnboardingFlowEndToEnd`: the rendered page banned the word "Demo" (a case the plan did not list; found by the full run). "Demo" left that list with a dated note; every other banned case stays.

## Browser check (port 8799, p5_18, real api.NewRouter from a throwaway Go test)

New account, consent not posted so the flow shows. Chromium and WebKit, light and dark, 320x700, 390x844 and 1470x830: 12 runs, all the same result.

| Width | Step 1 pair | Try the demo box | Continue box | Label lines | Underline | Round trip (step 1) | Disagree box |
|-------|-------------|------------------|--------------|-------------|-----------|---------------------|--------------|
| 320 | stacked, Try the demo above, Continue last | 280x48 | 280x48 | 1 | none | /demo?from=welcome, Exit href /welcome, back on step 1 | lead as built plus the restored sentence; Read again and Log out in one row, Try the demo full width (48px) under them; Try the demo opens /demo?from=welcome and Exit returns to /welcome |
| 390 | side by side, Try the demo left | 171x48 | 171x48 | 1 | none | same | same as 320 |
| 1470 | side by side, natural width | 134x48 | 103x48 | 1 | none | same | all three in one row; Try the demo 48px |

No horizontal overflow in any run. Extra probe at 320x568 and 390x667 (short phones, both engines): Try the demo in the Disagree box scrolls into view and is the element hit at its centre. Theme colours come from the existing .btn rules (no new colour in this plan), so dark and light behaved the same. The orchestrator does the iPhone simulator; the owner looks at step 1 and the Disagree box at the viewing session (human-check in the plan).

## Verification

- Full suite on p5_18 (second run, after the e2e amendment): Go `--- PASS` 5090, `--- FAIL` 0, `^FAIL` 0, `--- SKIP` TestGeocodeLive, TestOfficialLive, TestScreeningFixture (all allowed). Named passes: TestWelcomeStepOneDemoPair, TestDisagreeBoxPointsToDemo, TestWelcomeDemoLinksAreButtons, TestWelcomeOffersTheDemoForANewAccount, TestWelcomeFourStepsInOrder, TestWelcomeStep2ReadsTheSharedText.
- The first full run had one FAIL, TestOnboardingFlowEndToEnd, caused by the banned word "Demo" described above; fixed and rerun green (not a flake).
- Node: `node --test web/*.mjs` 990 pass, 0 fail.
- `git diff --stat -- web/static/js/welcome.js` is empty (no change). No em or en dash in any file written. p5_18 dropped (confirmed with psql -lqt); nothing listens on 8799; the throwaway server test (internal/api/handlers/zz_scratch_serve_test.go) was deleted before any commit.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestOnboardingFlowEndToEnd banned "Demo" on the rendered page**
- **Found during:** Task 2 full suite
- **Issue:** An older e2e case from the 3.4 / 3.3 plans banned the word for this phase, so it failed once the links shipped.
- **Fix:** removed "Demo" from that list with a dated note; every other case kept.
- **Files modified:** internal/api/handlers/welcome_e2e_test.go
- **Commit:** 3b79666a

## Known Stubs

None.

## Findings for the orchestrator

- **Focus trap does not know about links.** `FOCUSABLE_SELECTOR` in welcome.js is `'button, input, [tabindex]:not([tabindex="-1"])'`, so the two Try the demo links are not in the trap's list. They are reachable by Tab in document order. Step 1 is fine (Continue is still the last item in the trap). In the Disagree box the link is the last control, but the trap thinks Log out is last, so Tab from Try the demo leaves the page's dialog instead of wrapping to the first control. The plan's acceptance criterion says welcome.js stays unchanged, so I left it. A one-token fix (add `a[href]` to the selector) would close it; it needs a decision because it touches welcome.js and possibly welcome_flow_test.mjs.
- Exit from the Demo lands on `/welcome?step=1` (welcome.js adds the step to the address); the page shows step 1 as expected.

## Self-Check: PASSED

- web/demo_entry_contract_test.go, web/templates/welcome.html.tmpl, web/static/css/welcome.css: present.
- Commits f3a07c44, 03319d65, 3b79666a present in `git log`.
