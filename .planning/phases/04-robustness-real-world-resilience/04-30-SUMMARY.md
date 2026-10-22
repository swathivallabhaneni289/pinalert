---
phase: 04-robustness-real-world-resilience
plan: 30
subsystem: web (list pane, bottom bar, map credit, opened report, Activity, Disclaimer)
tags: [gap-closure, design-fixes, accessibility, large-text]
status: complete
requires: []
provides:
  - context lines inset and spaced under the list head
  - Expired in place of Faded out in Ended nearby
  - bottom bar labels that wrap between words at large text
  - map credit capped at 12px
  - Disclaimer 102 note
  - Flag and Stop updates as calm rows at the bottom of every opened report
affects: [03.1 retest, Phase 4 retest, plan 04-31 (no shared files)]
tech-stack:
  added: []
  patterns: [calm full width row for report actions, sibling selector to join two rows]
key-files:
  created:
    - web/static/icons/bell-off.svg
  modified:
    - internal/service/moderationcopy.go
    - web/static/css/feed.css
    - web/static/css/main.css
    - web/static/css/phone.css
    - web/static/css/trust.css
    - web/static/css/updates.css
    - web/static/js/map.js
    - web/static/js/mute.js
    - web/static/js/ended-cards.js
    - web/static/js/ended-nearby.js
    - web/templates/profile.html.tmpl
decisions:
  - The laptop popup Flag row lives in trust.css and the Mute row in updates.css; chrome.css (plan 31's file) is untouched
  - The phone popup Flag hairline moved from --color-border to --color-text-muted so Flag looks the same everywhere
  - bell-off is registered through a new laterIcons list rather than growing the ten Phase 3.4 glyphs
metrics:
  completed: 2026-10-23
  tasks: 6
  commits: 7
---

# Phase 4 Plan 30: Design fixes from the 3.1 and Phase 4 retest Summary

The owner's retest findings on the Phase 3.4 layout, except the Flag window
(plan 31): the place and filter lines sit inset and spaced under the list head,
Ended nearby says Expired, the phone bottom bar wraps its labels between words
at 200 percent text, the map credit stays 12px, the Disclaimer names 102 beside
108, and Flag this report and the new Stop updates about this report are calm
full width rows at the bottom of every opened report.

## Commits

| Item | Commit | What |
| ---- | ------ | ---- |
| 5 | 80d1664 | Disclaimer: ambulance (108, or 102 in some states) |
| 2 | 55b9f3d | Ended nearby: Expired group, "Expired {when}" line |
| 1 | 6173a61 | Context lines: top space, row gap, one space between two lines (feed.css) |
| 1, 4 | 56cdfe9 | Laptop context lines take the head's 16px inset; map credit capped at 12px (main.css) |
| 3 | b33eb50 | Bottom bar labels wrap between words; items never narrower than their longest word at 380px and under |
| 6, 7 | b5b9a15 | Flag and Stop updates rows, bell-off glyph, arrangeCard order |
| 6 | ecac372 | Flag row hover fill on both sizes (the row's transparent background had outranked chrome.css's hover rule) |

## Measurements, before and after

Playwright 1.x, Chromium and WebKit, signed in as a second account on a report
it confirmed (so both Mute and Flag show). Chromium and WebKit gave the same
numbers unless noted. 200 percent is `html { font-size: 200% }`.

**1. Place line and filter line** (head = the list head's bottom line, count = where the head's words start)

| Size | Before | After |
| ---- | ------ | ----- |
| 390x844 phone | words at 16 (count 16); place line top 456 = head bottom 456 (0px gap); filter line alone would also touch the head; wrapped button straight under the sentence | place line top 464 (8px under the head), words at 16; wrapped button 4px under its sentence (488); filter line 8px under the place line (540) |
| 1470x830 laptop | words at 0 (count 16), line top 170 = head bottom 170, button top 170 | words at 16, line top 178 (8px under the head), filter line top 230, 8px under the place line |
| 1470x830, 200% | words at 0, top = head bottom | words at 16, 8px under the head |

The Back to my location and Clear buttons stay 44px tall at every size (measured h44). The Back button's computed text-decoration is none (it is the outlined inline-text-button, unchanged).

**2. Ended nearby** (My activity, Ended nearby, a report expired in the database)

| | Before | After |
| - | ------ | ----- |
| Group label | Faded out | Expired |
| Card When line | Faded {when}, after N hrs | Expired 2 hours ago, after 8 hrs |
| Opened expired report | Faded {when}, after N hrs. | Expired {when}, after N hrs. (map.js closedLineText) |

Fades in N hrs on live reports is unchanged ("Fades in 7 hrs." in the popup shots).

**3. Bottom bar labels** (lines per label; [label left, right] in [item left, right])

| Width | Before, 200% | After, 200% | After, 100% |
| ----- | ------------ | ----------- | ----------- |
| 390 | one line each; Official alerts [79,215] spills out of [101,193] into My activity [187,299]: overlap | Nearby 1, Official alerts 2 [101,193], My activity 2 [197,290], Report 1; no overlap, every label inside its item | every label one line, same positions as before (Nearby [29,72], Official alerts [109,184], My activity [213,274], Report [320,360]) |
| 360 | overlap, labels outside their items | Official alerts 2, My activity 2, others 1; no overlap; last item ends at 356 | one line each, unchanged |
| 320 | overlap, labels outside their items | Official alerts 2, My activity 2, others 1; no overlap; but the four longest words plus the item padding need 344px, so the Report item ends at 348, 28px past the 320px screen edge | one line each, unchanged |

**4. Map credit line**

| Size | Before | After |
| ---- | ------ | ----- |
| 390, 200% | 24px text, 101px tall (3 lines) | 12px, 17px tall (1 line) |
| 360 and 320, 200% | 24px, 101px | 12px, 34px (2 lines, the same as at 100% there) |
| 1470, 200% | 24px, 34px | 12px, 17px |
| every size, 100% | 12px; 17px at 390 and 1470, 34px at 320 and 360 | unchanged |

**5. Disclaimer** before: "Older numbers for police (100), fire (101) and ambulance (108)." after: "Older numbers for police (100), fire (101) and ambulance (108, or 102 in some states)." The numbers card still reads 100, 101, 108.

**6 and 7. Flag and Mute in the opened report** (light; dark gave the same shapes with the dark muted token rgb(156, 163, 175))

| | Before | After |
| - | ------ | ----- |
| Laptop Flag | boxed: 1px border on all sides in --color-border, 8px corners, 137x44, no glyph, beside Mute | row: 305 wide (the actions width), 48px, hairline above only in --color-text-muted rgb(107, 114, 128), 0 corners, transparent, alert glyph 20x20, semibold, no underline |
| Laptop Mute | "Mute this report", boxed 143x44, beside Flag | "Stop updates about this report", the same row with the bell-off glyph 20x20, 16px above it; Flag directly under it (0 margin) |
| Phone Flag | row at the bottom, hairline in --color-border rgb(225, 228, 232) | the same row, hairline in --color-text-muted |
| Phone Mute | "Mute this report", boxed 143x44, between Mark resolved and the trust card | the row just above Flag at the bottom of the report (24px above it), Flag directly under it |
| Order in the DOM | phone: votes, mute, trust, flag | phone: votes, trust, mute, flag; laptop: trust, votes, mute, flag |
| Keyboard | Mute then Flag | Mute then Flag (Chromium Tab from Mute lands on Flag, ring 2px solid drawn inside, offset -2px). WebKit's Tab skips buttons by Safari default, so its order is proven by the DOM order. |
| Hover | boxed buttons filled on hover | both rows rest transparent and fill rgb(225, 228, 232) (--color-border) on hover, at 1470 and 390, Chromium and WebKit (after ecac372) |
| Press (laptop) | | first press: "Get updates about this report again", aria-label "Get updates about this report again, Flood, 17 min ago", data-muted true; second press: back to "Stop updates about this report", data-muted false |

The Settings list keeps its "Unmute this report" button (settings-alerts.js untouched). The Low data view is untouched: the laptop Low data row is the reference look and the phone Low data cards keep Flag in their corner.

## Tests amended (each with an "Amended 2026-10-22 by plan 04-30 (owner)" note)

- internal/service/moderationcopy_test.go TestOlderNumbersLabel: the two 108 cases carry the 102 note; one new case, 108 alone.
- internal/api/handlers/legal_e2e_test.go (numbers card text): the older numbers sentence carries the 102 note.
- web/map_follow_contract_test.go TestMapFollowsFinishedPopup: 'Faded ' became 'Expired '.
- web/past_tab_contract_test.go TestPastCardWords: 'Faded ' became 'Expired '.
- web/ended_nearby_test.mjs (the two groups test): the When line matches /^Expired .*, after 6 hrs$/.
- web/app_bar_contract_test.go TestAppBarLabelsNoWrap: white-space normal, overflow-wrap normal, word-break normal and text-align center on the item, min-width min-content on the narrow item, in place of white-space nowrap.
- web/mute_contract_test.go TestMuteControlShape: the two new labels, each exactly once, and the old labels must be gone.
- web/mute_test.mjs: every "Mute this report" and "Turn updates back on" assertion and test title moved to the new words (21 tests, all pass).
- web/css_contract_test.go TestNewIconsAreRegisteredAndUsed: also checks the new laterIcons list (bell-off) by the same rules; bell-off registered in nonCategoryIcons.

New pin: web/mute_contract_test.go TestMuteMountedBeforeFlag now also checks that arrangeCard appends Mute, its error line and then Flag last on both sizes.

## Deviations from Plan

1. **[Rule 2, consistency] The phone popup Flag hairline changed colour.** The plan said the phone report view already looked right, but its hairline was --color-border (rgb(225, 228, 232)), not the muted token of the reference row. It is --color-text-muted now, so Flag is the same on all three surfaces. phone.css, commit b5b9a15.
2. **Mute moves on the phone.** It sat between Mark resolved and the trust card; arrangeCard now puts it just above Flag at the bottom on the phone too, as item 7 asks ("just above Flag at the bottom of the opened report on both sizes"). buildPopupCard is unchanged, so TestMuteMountedBeforeFlag's build order still holds.
3. **Icon registration through a new list.** TestNewIconsAreRegisteredAndUsed fails fast unless phase34Icons holds exactly ten names, so bell-off goes into a new laterIcons list that the test checks by the same rules, rather than into the Phase 3.4 list.

## Known limits and deferred items

- **320px wide at 200 percent text:** the labels no longer overlap and none is cut inside a word, but the four longest words ("Nearby", "Official", "activity", "Report") plus each item's 4px side padding and the 4px gaps need 344px, so the bar runs 28px past a 320px screen and the Report item is partly off screen. The page then scrolls sideways: document scrollWidth 348 on Nearby and 350 on My activity at 320 wide (Chromium and WebKit). Before this plan the labels overlapped there instead, and the Report label ended at 324. 360 and 390 fit with no sideways scroll from the bar. Fixing 320 would mean a smaller label or a different bar at that size; not done without the owner. The plan's own 200 percent check is at 390, which passes.
- **Bar height at 200 percent text:** the phone detail pane and the toast are placed from the fixed --app-bottombar-height (65px). At 200 percent the bar was already taller than that before this plan (one line of 24px text plus the glyph), and two line labels make it taller still, so the bottom of the open report view can sit behind the bar's top at that size. Not changed here.
- Not part of this plan and untouched: the Flag window (plan 31), chrome.css.

## Verification

- go build ./...: exit 0
- go vet ./...: exit 0
- node --test web/*.mjs: 630 tests, 630 pass, 0 fail
- Full Go suite on p4_w30 (`env -i PATH HOME DATABASE_URL=... go test ./... -p 1 -count=1 -v`): exit 0; 1348 top level `--- PASS` lines (3901 counting subtests), 0 `--- FAIL`, 2 `--- SKIP` (TestGeocodeLive and TestScreeningFixture, the two allowed). The known flake TestCastVoteConcurrentSameAccountKeepsEveryRow passed, so no rerun was needed. The scratch server was stopped before the suite ran. These are the numbers of the final code commit ecac372: the whole gate (build, vet, node, suite) was run again after the hover fix and gave the same counts.

Browser checks: Playwright Chromium and WebKit at 390x844 and 1470x830, light and dark, plus 320, 360 and 390 wide at 100 and 200 percent text and 1470 at 200 percent, on a scratch server (port 8783, database p4_w30). Screenshots stayed in the session scratchpad, outside the repo.

## Self-Check: PASSED

- FOUND web/static/icons/bell-off.svg
- FOUND commits 80d1664, 55b9f3d, 6173a61, 56cdfe9, b33eb50, b5b9a15, ecac372 on worktree-agent-a7337803d23bf630f
- No em or en dashes in any added line; STATE.md, ROADMAP.md and REQUIREMENTS.md not touched; nothing pushed.
