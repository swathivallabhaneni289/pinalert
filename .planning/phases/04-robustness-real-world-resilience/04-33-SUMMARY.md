---
phase: 04-robustness-real-world-resilience
plan: 33
subsystem: web (phone bottom bar, theme.js token writer, phone message)
tags: [gap-closure, accessibility, large-text]
status: complete
requires: [04-30]
provides:
  - phone bottom bar labels that fit a 320px screen at 200 percent text
  - --app-bottombar-height written from the bar's measured height
  - phone message (toast) that never sits under the bottom bar
affects: [Phase 4 retest, plan 04-32 (no shared files)]
tech-stack:
  added: []
  patterns: [measured length token written by theme.js, rAF scheduled ResizeObserver]
key-files:
  created: []
  modified:
    - web/static/css/phone.css
    - web/static/js/theme.js
    - web/app_bar_contract_test.go
    - web/bar_height_contract_test.go
    - web/harness_p4_findings_contract_test.go
    - web/phone_layout_contract_test.go
decisions:
  - At 380px and under the bar labels stop at 20px (min(var(--font-size-caption), 20px)); the largest round size that fits 320px with room for wider fonts
  - theme.js writes the bar's measured height into --app-bottombar-height, leaving the home indicator inset out because every reader adds it again
  - The phone message keeps its 68px place as a floor and never sits lower than the bar's top
metrics:
  completed: 2026-10-23
  tasks: 2
  commits: 2
---

# Phase 4 Plan 33: The phone bottom bar at 200 percent text Summary

The two limits plan 04-30 left open are closed. At 320px wide and 200 percent
text the four bar labels stop at 20px, so no page scrolls sideways. The bar's
real height is now measured by theme.js and written into
--app-bottombar-height, so the opened report, the not available view, the
search results and the message all end on the bar's top at every text size.
At 100 percent text nothing moves: the bar is 65px and every label is on one
line.

## Commits

| Commit | What |
| ------ | ---- |
| 450fd47 | Label cap at 380px and under, theme.js bar measurement, message floor, one new pin and three amended pins |
| (this file) | Summary |

## What I chose

**320px labels:** inside the existing `(max-width: 380px)` rule the item's
font size is `min(var(--font-size-caption), 20px)`. At 24px (200 percent) the
four longest words plus paddings need about 348px; at 20px they need about
304px, so 320 fits with 16px to spare (room for a wider system font such as
Roboto). By arithmetic from the 24px and 20px runs (not measured), 21px would
fit with only about 9px to spare. The cap only bites above 166 percent text, so 100 and 150 percent are
unchanged. At 360 the 200 percent labels also drop from 24px to 20px, because
a 340px phone would overflow at 24px and the 380px tier is one rule. The line
height is the unitless 1.3, so it follows the capped size.

**Bar height:** measured, not removed. theme.js gains measureAppBar,
scheduleAppBarMeasure and watchAppBar beside the strip's three functions:
measure once on readystatechange, again from a ResizeObserver whose callback
only schedules the write for the next animation frame (the plan 21 loop
lesson), and on window resize. The value written is the bar's height less
(padding-bottom minus padding-top), that is less the home indicator inset,
because the bar puts the inset in its own bottom padding and every reader adds
`env(safe-area-inset-bottom)` again. A hidden bar (the laptop) writes 0px,
which is the 900px tier's value; a page with no bar writes nothing.

**Message (toast):** `bottom: max(calc(var(--touch-target-min) + var(--space-sm) + var(--space-md)), calc(var(--app-bottombar-height) + env(safe-area-inset-bottom)))`.
68px at the normal text size as before; above the bar when the bar is taller,
where it sits directly on the bar's top edge with no gap (a gap would have
moved it up at least 1px at the normal size). The message with an action
(Undo, `#toast.toast--action`) declares no `bottom` of its own, so it takes
the same place. No other fixed phone element has a literal bottom offset: the
report view and the not available view already read the token.

## Measurements, before and after

Playwright, Chromium and WebKit gave identical numbers. Signed in as a second
account viewing another account's report, so Flag and Stop updates both show.
Pages: Nearby, Official alerts, My activity, Settings, and Nearby with the
report open (`/?report=1`, scrolled to its Flag row). 150 and 200 percent are
`html { font-size }`.

**Sideways scroll (document scrollWidth)**

| Width | 100% | 150% | 200% before | 200% after |
| ----- | ---- | ---- | ----------- | ---------- |
| 320 | 320 | 320 | 348 Nearby, 349 Official alerts, 350 My activity, 344 Settings | 320 on every page |
| 360 | 360 | 360 | 360 | 360 |
| 390 | 390 | 390 | 390 | 390 |

**Labels at 200 percent** ([left, right], lines)

| Width | Before | After |
| ----- | ------ | ----- |
| 320 | 24px: Nearby [8,87] 1, Official alerts [99,172] 2, My activity [184,258] 2, Report [270,344] 1 (past the edge) | 20px: Nearby [8,75] 1, Official alerts [87,163] 2, My activity [175,238] 2, Report [250,312] 1 |
| 360 | 24px, Report [278,352] | 20px, Report [290,352] |
| 390 | 24px, unchanged | 24px, unchanged (the cap is 380px and under) |

No label overlaps another and none breaks inside a word at any of the nine
sizes. At 100 percent every label is one line at 12px with the same positions
as before (390: Nearby [29,72], Official alerts [109,184], My activity
[213,274], Report [320,360]).

**Bar height and the written token**

| Text | 320 | 360 | 390 |
| ---- | --- | --- | --- |
| 100% | 65, token 65px | 65, 65px | 65, 65px |
| 150% | 83.78, 83.78px | 83.78, 83.78px | 83.78, 83.78px |
| 200% before | 99.38, token still 65px | 99.38, 65px | 99.38, 65px |
| 200% after | 89 (20px labels), 89px | 89, 89px | 99.38, 99.375px |

At 360 and 150 percent on Settings the bar is 65px (no current item, so no
label is semibold and both two word labels fit on one line); the token follows
it (65px). On the laptop (1470x830) the token is 0px on Nearby, Official
alerts, My activity, Settings, Disclaimer and an opened report; resizing to
390 writes 65px and back to 1470 writes 0px, in both engines.

**Opened report, scrolled to its end** (Flag row top to bottom; the report view's bottom; the bar's top)

| Size | Before | After |
| ---- | ------ | ----- |
| 390x844, 200% | Flag 731 to 779, view bottom 779, bar top 744.6: the Flag row ran 34px under the bar's top | Flag 697 to 745, view bottom 744.6, bar top 744.6: fully visible |
| 390x844, 150% | Flag 730.7 to 778.7, bar top 760.2 | Flag 712.7 to 760.7, bar top 760.2 |
| 360x740, 200% | Flag 626.6 to 674.6, bar top 640.6 | Flag 602.6 to 650.6, bar top 651 |
| 320x640, 200% | Flag 526.6 to 574.6, bar top 540.6 | Flag 502.6 to 550.6, bar top 551 |
| every size, 100% | Flag ends on the bar's top (779, 675, 575) | unchanged |

(The half pixel past the view bottom at some sizes is the scroll position
rounding; the Flag row's bottom is never more than 0.5px past the bar's top.)

**Message bottom edge** (Nearby, My activity, opened report)

| Size | Before | After |
| ---- | ------ | ----- |
| 390, 100% | 776 (68px up), bar top 779 | 776, unchanged |
| 390, 150% | 776, bar top 760.2: over the bar | 760.2 |
| 390, 200% | 776, bar top 744.6: over the bar | 744.6 |
| 320, 200% | 572, bar top 540.6 | 551 |

No "ResizeObserver loop" message in either engine at any size or page.

## Tests amended (each with an "Amended 2026-10-23 by plan 04-33" note)

- web/app_bar_contract_test.go TestAppBarLabelsNoWrap: the narrow item also pins `font-size: min(var(--font-size-caption), 20px)`.
- web/phone_layout_contract_test.go TestPhoneRulesLiveInTheNarrowBlock: one exact exception to the "font-size is a type size token" rule, for that head, inside the 380px query, with that exact value. Every other phone rule still has to name a token.
- web/harness_p4_findings_contract_test.go TestHarnessFindingP4_9MessageClearsTheOpenedReport: the phone message's bottom is the max of the old 68px place and the bar token plus the inset (still exactly one declaration).

New pin: web/bar_height_contract_test.go TestBottomBarTokenFollowsTheRealBar (theme.js names the token, measureAppBar writes 0px for a hidden bar and then the measured height less the inset on the root, watchAppBar returns before writing on a page with no bar, the observer only schedules through requestAnimationFrame, the resize listener, both start calls, and the phone.css readers still add the inset).

## Deviations from Plan

1. **[Rule 1, bug] The message did not read the token.** The plan names the toast as placed from --app-bottombar-height, but its phone rule was a fixed 68px, so it sat over a taller bar at 150 and 200 percent (and over the bar on any phone with a home indicator inset, where the bar is 65px plus the inset). It now takes the larger of 68px and the bar token plus the inset. At 100 percent text on a phone with no inset it is still 68px.
2. **The cap also applies at 360 and 200 percent** (24px to 20px), because the 380px tier is one rule and a phone between 321 and 343px would overflow at 24px. 390 is untouched.

## Known limits

- Real iPhone checks (home indicator inset) were not run; the inset arithmetic is pinned by the new test and by the readers all adding the inset once. Playwright's inset is 0.
- Not touched: map-tools.js and internal/geocode (plan 04-32), chrome.css, the laptop layout.

## Verification

- go build ./...: exit 0
- go vet ./...: exit 0
- node --test web/*.mjs: 637 tests, 637 pass, 0 fail
- Full Go suite on p4_w33 (`env -i PATH HOME DATABASE_URL=... go test ./... -p 1 -count=1 -v`) at commit 450fd47: exit 0; 1357 top level `--- PASS` lines (3934 counting subtests), 0 `--- FAIL`, 2 `--- SKIP` (TestGeocodeLive and TestScreeningFixture, the two allowed). The known flake TestCastVoteConcurrentSameAccountKeepsEveryRow passed, so no rerun was needed. The scratch server was stopped before the suite ran.

Browser checks: Playwright Chromium and WebKit at 320x640, 360x740 and 390x844, at 100, 150 and 200 percent text, on five pages each including an opened report scrolled to its Flag row, plus 1470x830 and a resize across 900px, on a scratch server (port 8786, database p4_w33). Nothing was saved as an image; the numbers above are computed values.

## Self-Check: PASSED

- FOUND commit 450fd47 on worktree-agent-a6c6e878198f4eff3
- FOUND web/static/css/phone.css, web/static/js/theme.js and the four test files in that commit
- No em or en dashes in any added line or in this file; STATE.md, ROADMAP.md and REQUIREMENTS.md not touched; nothing pushed.
