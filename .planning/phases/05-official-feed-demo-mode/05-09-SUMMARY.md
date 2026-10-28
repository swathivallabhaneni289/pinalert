---
phase: 05-official-feed-demo-mode
plan: 09
subsystem: ui
tags: [official-alerts, sachet, honesty, disclosure-rows, level-chips, vanilla-js, css-grid]
status: complete

requires:
  - phase: 05-08
    provides: GET /api/official/alerts (the server authored reader view)
  - phase: 03.4
    provides: the Official alerts place (D-22), place-section, place-box, empty-state, app bars
provides:
  - the filled Official alerts page: status line, stale notice, groups, rows, details, attribution, noscript line
  - web/static/js/official-alerts.js, a source agnostic renderer (API base, fixed point and account read from the page)
  - web/static/css/official.css (laptop grid, phone card, level chips, authority badge, Demo tag, details panel)
  - octagon-alert.svg and triangle-alert.svg, registered in nonCategoryIcons
  - window.PinalertOfficialAlerts = { render, renderStatus, areaSource, relativeIssued, untilText }
affects: [05-15, 05-19, 05-21]

key-files:
  created:
    - web/static/js/official-alerts.js
    - web/static/css/official.css
    - web/static/icons/octagon-alert.svg
    - web/static/icons/triangle-alert.svg
    - web/official_alerts_test.mjs
    - web/official_alerts_contract_test.go
    - web/official_contrast_test.go
  modified:
    - web/templates/alerts.html.tmpl
    - internal/api/handlers/alerts.go
    - web/css_contract_test.go
    - web/places_contract_test.go
    - internal/api/handlers/places_e2e_test.go

key-decisions:
  - "One precedence for the page, pinned by node cases: load error, then unavailable, then area unknown, then stale or fresh. The claim sentence is written in exactly one place, behind fresh AND area_known AND zero rows drawn."
  - "A body that is not shaped like the route's (state not fresh, stale or unavailable, or groups not an array) is a load error, never an empty list."
  - "Rows are counted from the groups actually drawn, not from the server's count field."
  - "Time words come from the server clock: now is last_success_at plus age_seconds, so Issued 6 min ago is right on a phone whose own clock is wrong. Only the elapsed time since the answer is local."
  - "Clock times are built from Intl formatToParts (hour, minute, lower cased dayPeriod) so a narrow no break space or an upper case am never reaches the page."
  - "The laptop only labels (column heads, the Issued by, Issued and Until words) are clipped, not display none, so TestSizeOnlyElementsAreNamed (not in this plan's file list) needed no new entry and screen readers keep full names."
  - "Phone card: the 12px radius border sits on the item (button plus panel), so an open panel stays inside the card's border."
  - "Stale with no rows (no UI-SPEC copy): the notice plus the heading Last known alerts and As of <time>, no box and no reassuring sentence."

patterns-established:
  - "Server authored state, page chooses blocks: every state sentence lives in the script, the template carries none (TestOfficialAlertsNoClaimInTemplate)."
  - "id scoped display none for the blocks that share a display rule (.empty-state flex, .place-section grid), the low.css precedent."

requirements-completed: [ALERT-01, ALERT-02, ALERT-04, ROBUST-06]

duration: about 2h 20m
completed: 2026-10-29
---

# Phase 5 Plan 09: The Official alerts page Summary

The Official alerts place now lists the real alerts for where the reader is and their home area, as groups of disclosure rows with colour plus shape plus word level chips and the Official badge, and it tells the plain truth about every state of the feed (fresh, stale, unavailable, never checked, area unknown, load error) from one source agnostic renderer.

## Commits

- 082d67b8 test(05-09): pin the official alerts page states, honesty grid and markup
- 44a84ea5 feat(05-09): fill the Official alerts page with every honest state
- ab9eaf96 test(05-09): amend the Phase 3.4 alerts pins for the filled page
- 41d83ed5 fix(05-09): capitalise Just now and About in the laptop columns
- the metadata commit holding this file

## What was built

- **Task 1 (RED).** `web/official_alerts_test.mjs` (36 cases), `web/official_alerts_contract_test.go` (TestOfficialAlertsShell, NoClaimInTemplate, ScriptSinks, LaptopGridAndPhoneCard, Copy) and `web/official_contrast_test.go` (TestOfficialLevelContrast, six pairs, four token sources). Run before Task 2 the node file died on the missing script and the five contract tests failed (RED). TestOfficialLevelContrast passed at once, because it pins existing tokens, not new code.
- **Task 2.** The template (3.4 shell kept, every state block starts hidden, no claim words), the renderer, the stylesheet, the two icons, `TestPhase5IconsRegisteredAndUsed` and the two `nonCategoryIcons` entries in css_contract_test.go, and the rewritten AlertsPage doc comment.
- **Task 3.** The two Phase 3.4 pins amended, the browser check, the full suite.

## Amended tests (list for the record)

| Test | File | Amendment |
|------|------|-----------|
| TestAlertsNeverSaysNoAlerts | web/places_contract_test.go | Amended 2026-10-29 by plan 05-09 (ALERT-04). The two "not connected" wants are replaced by the noscript line and the attribution line. Every claim ban is kept: `"no" + " alerts"`, `"no official" + " alerts"`, `"no" + " warnings"`, `"all" + " clear"`, `"nothing in" + " force"`, `"in" + " force"`, `"last" + " checked"`, `"0" + " alerts"`, and the list, count and notice strip bans. |
| TestAlertsPageRenders | internal/api/handlers/places_e2e_test.go | Amended 2026-10-29 by plan 05-09 (ALERT-04). The two "not connected" wants are replaced by the noscript line, the attribution line and the shell ids (official-status, official-list, official-alerts.js). Title, both headings, no-store, the claim bans, the places.css then phone.css order and the T-34-19 redirect are kept; a new check puts official.css between places.css and phone.css. |
| nonCategoryIcons | web/css_contract_test.go | two entries (octagon-alert, triangle-alert) with a search.svg style comment, plus the new TestPhase5IconsRegisteredAndUsed that proves the files and the literal mask rules in official.css. |

## Browser check

Port 8792, database p5_09, WebKit and Chromium through Playwright. Seeded on district 749 (shown by the boundary data as "Ntr"): an Extreme with an English and a Telugu block, a Severe with an inferred end, 45 near identical state level Severe lightning alerts for Andhra Pradesh, one unplaced alert. Signed in with the link from the server log, consent and first run done by the API, a device fix near Vijayawada (16.5062, 80.6480) stored by visiting Nearby once with the geolocation set. The feed status was set to fresh (4 minutes), then 2 hours, then 7 hours, with the last attempt set to now each time so the server never reached NDMA (the log holds no SACHET line). Screenshots stayed in the scratch folder.

| Size | Theme | Engine | Groups and order | Details | Stale (2 h) | Unavailable (7 h) | Horizontal scroll | Smallest row |
|------|-------|--------|------------------|---------|-------------|-------------------|-------------------|--------------|
| 390x844 | light | WebKit | Where you are: Ntr district, Across Andhra Pradesh, Could not be placed on a map; count 4; "45 alerts like this" | Original text (Telugu), lang te, row expands, chevron turns | notice, Last known alerts 4, As of; no In force now | could not be checked, body with since time; no list | none (390 of 390) | 201 px |
| 390x844 | dark | WebKit | same | same | same | same | none | 201 px |
| 390x844 | light | Chromium | same | same | same | same | none | 201 px |
| 390x844 | dark | Chromium | same | same | same | same | none | 201 px |
| 1470x830 | light | WebKit | same, column head row drawn (Level, Alert, Issued by, Issued, Until) | same | same | same | none | 80 px |
| 1470x830 | dark | WebKit | same | same | same | same | none | 80 px |
| 1470x830 | light | Chromium | same | same | same | same | none | 80 px |
| 1470x830 | dark | Chromium | same | same | same | same | none | 80 px |
| 320x568 | light and dark | WebKit and Chromium | same, fresh only | not opened | not run | not run | none (320 of 320) | 201 px |

Also seen: the chips draw octagon-alert (Extreme), triangle-alert (Severe) and circle-alert (Moderate) at 16 px; the phone clips the column head row to 1 by 1 px and the laptop draws it 1120 by 32 px; fresh with every alert removed shows "No official alerts in force for your area" in all eight combinations; no page script error after /alerts loaded in any run. The stale and unavailable times read "Wed 11:20 pm" and "Wed 6:20 pm" because the clock was just past midnight and the weekday is added whenever the time is not today.

Not driven in a browser (covered by node cases only): area unknown, load error and Try again, the never checked words. The iPhone simulator check is the orchestrator's.

## Verification

- `node --test web/*.mjs`: 962 pass, 0 fail (36 of them are this plan's).
- `go vet ./...` clean. `DATABASE_URL= go test ./web/` passes.
- Full suite on the disposable database p5_09 (`go test ./... -p 1 -count=1 -v`), second run on a recreated database: `--- FAIL` 0, `^FAIL` 0, panics 0, 20 packages ok, 4908 PASS lines. SKIP lines: TestGeocodeLive, TestOfficialLive, TestScreeningFixture (the three allowed). Named PASS: TestOfficialAlertsShell, TestOfficialAlertsNoClaimInTemplate, TestOfficialAlertsScriptSinks, TestOfficialAlertsLaptopGridAndPhoneCard, TestOfficialAlertsCopy, TestOfficialLevelContrast, TestAlertsNeverSaysNoAlerts, TestAlertsPageRenders, TestPhase5IconsRegisteredAndUsed, TestNewIconsAreRegisteredAndUsed.
- The first full run, on the same database after the browser check seeding, had one failure: TestVotedReportsByAccountUsesTheAccountIndex (internal/store, a query plan test). Rerun by name it failed again, and in six more runs. It passed twice on a freshly recreated p5_09 and in the second full run. This plan touches nothing under internal/store (`git diff 8f57ed4c HEAD -- internal/store` is empty), so it is the known planner dependent flake, and it depends on the statistics a used database has. TestCastVoteConcurrentSameAccountKeepsEveryRow passed. The queue_logic_test WR-07 flake did not show.
- p5_09 dropped and confirmed gone with `psql -lqt`; the server on 8792 stopped (no listener).

## Deviations from Plan

**1. [Rule 3 - Blocking] Clipped, not hidden, for laptop only labels.** The plan's phone card and laptop columns share one markup. A `display: none` on the column head row (phone) or on the Issued, Until and Issued by words (laptop) would have tripped TestSizeOnlyElementsAreNamed in layout_rules_contract_test.go, which is not in this plan's file list. Both are clipped (position absolute, 1 px, clip) like the existing top bar title, the head row is aria-hidden and the words keep screen reader names. No test weakened.

**2. [Rule 2 - Missing] The honesty grid includes area_known.** The plan's grid bullet left out the case 05-08's summary states: only fresh, area known and no rows may say nothing is in force. The renderer and the node grid cover fresh with area unknown, stale with no rows, stale with area unknown, unavailable with area unknown and never checked, and re-draw after the claim was shown.

**3. [Rule 2 - Missing] A malformed answer is a load error.** A 200 with a body that is not shaped like the route's would otherwise render as an empty list; it now shows "Couldn't load official alerts".

**4. [Interpretation] Stale with no rows.** UI-SPEC has no copy for it. It shows the stale notice plus "Last known alerts" and "As of <time>" with no box and no sentence that reassures. Pinned by a node case.

**5. [Interpretation] Phone breakpoint.** The base is the laptop row and the phone card is one `@media (max-width: 899.98px)` block, as the plan names `.official-card` rules in the phone media query. The group label on the phone is `16px 0 8px` (not `16px 16px 8px`) because the section already carries the 16px side padding; the label starts 16px from the edge as UI-SPEC draws it.

**6. [Interpretation] Lower case time words.** Row values are written lower case after their "Issued" and "Until" words ("Issued just now", "Until about 8:35 pm") and the laptop capitalises them in the column (UI-SPEC P5-C10 lists "Just now" and "About 8:35 pm" for the columns).

**7. [Rule 1 - Bug] Laptop "about" stayed lower case (found in the browser check).** My first fix, a first-letter rule on the cell, did not reach the word in WebKit. Commit 41d83ed5 makes the time span an inline block with a raised first letter and capitalises the about span, laptop only.

**8. [Process] Task 3 amendments committed after Task 2.** At 44a84ea5 the old TestAlertsNeverSaysNoAlerts (still wanting the "not connected" lines) was red for one commit, until ab9eaf96. The plan splits the two tasks that way.

**9. [Process] TestOfficialLevelContrast is not a RED test.** It pins existing tokens, so it passed in the RED run.

No auth gates, no checkpoints, no Rule 4 items.

## Notes for later plans

- Plan 05-15 writes sessionStorage `pinalert.nearbyCentre` as `{"account": "<id>", "lat": n, "lon": n}`. This page reads exactly that, compares the account as a string and ignores anything else.
- Plan 05-19 (Demo): set `window.PinalertApiBase`, `data-fixed-lat`, `data-fixed-lon` on `#official-page` (and no `data-account-id`), and write its own attribution line text into `#official-attribution`. A row with `demo: true` and an empty badge draws the Demo tag and no Official badge (pinned). The page root id is `official-page`.
- Plan 05-21 (GDACS): rows with `source: "gdacs"` and badge "GDACS" render through the same code; the chip classes follow `level` (extreme, severe, else neutral), so GDACS Red and Orange need the server to send level extreme and severe, and `level_word` carries the word.
- District names are shown as the server sends them ("Where you are: Ntr district"); no display name table was added, as instructed.
- A time such as "11:20 pm" can wrap between the time and "pm" on a 320 px phone in the stale notice; the plain space is kept because the spec and tests use it.
- `/alerts` and the service worker allow list is not part of this plan.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-05-46 (stored XSS) is pinned by TestOfficialAlertsScriptSinks, js_sinks_contract and the markup payload node case; T-05-47 (false reassurance) by the honesty grid and TestOfficialAlertsNoClaimInTemplate; T-05-48 (shared phone) by the account match cases; T-05-49 (official look) by the Demo row case; T-05-50 (refetch loops) by one `fetch(` and one `setInterval(` in the script and the tick case.

## Self-Check: PASSED

Files exist: web/static/js/official-alerts.js, web/static/css/official.css, web/static/icons/octagon-alert.svg, web/static/icons/triangle-alert.svg, web/official_alerts_test.mjs, web/official_alerts_contract_test.go, web/official_contrast_test.go, and this summary. Commits 082d67b8, 44a84ea5, ab9eaf96 and 41d83ed5 exist on worktree-agent-ac68779d6c338e102. p5_09 dropped; nothing listening on 8792.
