---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 31
subsystem: trust-card
tags: [facts, trust-card, tick-rows, popup, feed-row, activity, owner-feedback, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the plain people words (plan 03-30), the state line and toggle face (plans 03-22 and 03-23), the trust object of D-21
provides:
  - "trust.facts: an ordered, server authored list of tick rows on the feed, the vote response and the Activity row"
  - The popup card as the owner's picture draws it: always open, counters left, state word right, tick rows, fade line, distance in the meta line
  - The same tick rows as the body of the panel on feed rows and Activity rows
  - Amendment rows A89 to A96, a Facts table and the amended contract lines in 03-UI-SPEC.md
  - docs/TRUST-MODEL.md, the swagger and the Phase 4 notes in the facts words
affects: [Phase 4 plans 04-12, 04-16, 04-18 and 04-UI-SPEC (the Low data view reads facts), 03-UAT test 10]
tech-stack:
  added: []
  patterns:
    - "One component, one data source: the browser holds no wording for a fact, a missing or invalid facts array hides the block"
    - "The fact rows reuse the signals' own sentences, so nothing is recomputed and nothing can drift"
key-files:
  created:
    - internal/service/trustfacts.go
    - internal/service/trustfacts_test.go
    - web/trust_facts_contract_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-31-PLAN.md
  modified:
    - internal/service/trustview.go
    - internal/service/feed_test.go
    - internal/service/reporter_test.go
    - internal/api/handlers/trust.go
    - internal/api/handlers/trust_e2e_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - docs/TRUST-MODEL.md
    - web/static/js/trust.js
    - web/static/js/map.js
    - web/static/js/app.js
    - web/static/css/trust.css
    - web/static/css/main.css
    - web/trust_contract_test.go
    - web/trust_face_contract_test.go
    - web/trust_topzone_contract_test.go
    - web/popup_contract_test.go
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
    - .planning/phases/04-robustness-real-world-resilience/04-12-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-16-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-18-PLAN.md
    - .planning/phases/04-robustness-real-world-resilience/04-UI-SPEC.md
key-decisions:
  - "A89 to A96: the panel is a checklist of server authored facts on all three surfaces; the popup card is always showing; distance and a fade line join the popup; tone glyphs are four shapes"
  - "The fade line reads Fades in 7 hrs., Fades in 1 hr. and Fades in 45 min., and never says unless confirmed again, because confirming does not extend a report's life"
requirements-completed: []
status: complete
---

# Phase 3 Plan 31: The tick mark trust card Summary

## Why

On 2026-10-10 the owner pasted the look they want for the trust card ("yes use this", picture `2026-10-10-checklist-FINAL-use-this.png`): a white card of tick rows, each a round tone icon, a bold title and a muted detail line, with the counters on the left of its header and the state word on the right. They test on a phone first, and three or four sentences was too long. A checklist shows the same four facts (confirmations, disputes, still current, reporter) as rows that can be scanned.

## What changed

| Surface | Before | After |
|---------|--------|-------|
| Trust object | counters, five sentence fields, two signals | the same, plus `facts` (additive, never null) |
| Map popup | a toggle that opened a panel of sentences, the reporter words and two numbers on the state line | a white card that is always showing: counters left, the server's state word right, four tick rows, a fade line under it, the distance in the meta line; no toggle, no chevron |
| Feed row and Activity row | compact toggle, panel of sentences | compact toggle unchanged; the panel shows the same tick rows (never the score lines) in a white bordered card |
| Escape in the popup | panel first, then the popup | one Escape closes the popup (rows keep A70) |

Not changed: the vote buttons and Mark resolved and their states, the severity badge and label, the title, the map, the page layout and colours, the popup surface (neutral, A10), every existing field and sentence of the trust object, `trustcopy.go`.

## The facts

Built in the new `internal/service/trustfacts.go`, called from `BuildTrust` with one added line and one added struct field. Kinds in order: `status` (only an expired report that is not resolved), `confirmations`, `disputes`, `currency` (exactly when the existing still current label is filled), `reporter` (not for none). Tones: good, neutral, caution, bad.

| Kind | Case | Tone | Title | Detail |
|------|------|------|-------|--------|
| status | expired, not resolved | neutral | Expired | It can no longer be confirmed. |
| confirmations | 0 | neutral | Not confirmed yet | Needs 2 people at different spots. |
| confirmations | 1 | neutral | Confirmed by 1 person | Needs 1 more person at a different spot. |
| confirmations | 2 or more | good | Confirmed by N people | At different spots nearby. |
| disputes | 0 | good | No disputes | No one nearby has disputed this. |
| disputes | 1 | bad | Disputed by 1 person | Someone nearby says this is wrong. |
| disputes | 2 or more | bad | Disputed by M people | At different spots nearby. |
| currency | up to date | good | Still happening | R confirmations in the last W. |
| currency | getting old | caution | Getting old | Fewer than 2 confirmations in the last W. |
| currency | needs re-confirming | bad | Gone quiet | People nearby can confirm it again. |
| currency | a dispute raised the bar | caution | Needs more confirmations | Disputed nearby, so it needs K confirmations in the last W. |
| currency | too early to tell | neutral | Too early to tell | Fewer than 2 people have confirmed this yet. |
| reporter | reliable, mixed, unreliable | good, caution, bad | Reliable reporter, Mixed record, Unreliable reporter | C of their last J reports were confirmed. |
| reporter | new | neutral | New reporter | Not rated yet. It needs 5 reports with a clear result. |

`score_detail` is "Score S out of 100." on the currency and reporter rows when a score exists, and only the popup shows it (A50, D-09). Currency and reporter rows carry the signals' own sentences; the one exception is Gone quiet, whose detail drops the words the title already says. A raised bar is read from the signal itself (its sentence is not the plain sentence of the band), so nothing is recomputed. The "Needs ..." detail of a confirmations row is empty for Hidden, Retracted and expired reports. Real numbers appear above 99 in titles (only the counter text is capped).

## The look against the picture

Rendered with the picture's own data (a Low power outage, 50 minutes old, 0.4 km away, 2 people at different spots, no disputes, Reliable reporter 5 of 5, up to date) at a 340px popup in Chromium and WebKit, light and dark, and compared by eye and by computed style.

Fixed in this plan: the card bottom air (the last line was about 5px above the edge, the picture has about 16px; now 13px line box, 15 to 16px to the ink); the tone glyph sits on the first title line also at 200 percent text; a counter pair breaks between the two counters and never inside one.

Left on purpose or left for the owner (each one is a difference you can see):

| What | Picture | Ours | Why |
|------|---------|------|-----|
| Popup surface | tinted with the severity colour, no bar | neutral white with the 4px severity bar, and a 1px border on the white card | A10 (owner Q4) pins a neutral popup; the owner decides whether the picture's tint replaces it |
| Header row | counters left, "Up to date" right on one line | the state word wraps under the counters, right aligned | the server label "Report up to date" (A86) is 117px, the counters 178px, 271px is available; the sample "Up to date" would fit |
| Card width | 319px, side padding 10px | 305px | the 4px bar plus the 16px body padding are pinned |
| Row text | about 15px title, 13px detail | 16px and 14px | the type scale has four sizes |
| Score lines | none | "Score 92 out of 100." under the currency and reporter rows (39px) | A50 and the decision to keep score_detail in the popup |
| Row rhythm | about 54.5px rows | about 52.6px rows | less height on a popup that already scrolls |
| Disc colour | lighter green, about 3.3:1 | the tone token, 5.1:1 | the state word shares the token and must clear 4.5:1 |
| Radius | about 16px | 12px | the 12px radius is pinned on the card, the panel and the popup wrapper |
| Fade line | darker text, flush with the card, no rule under it | muted, inset 4px, the existing hairline of the actions below it | the actions block is unchanged by this plan |
| Meta line | no glyph | the existing category glyph in front | A24, earlier plan |
| Confirm and Mark resolved | black Confirm, white Dispute, green link | the existing grey buttons | explicitly out of this plan |
| Fade words | "Fades in 7 h unless confirmed again." | "Fades in 7 hrs." | the owner's later wording; the second half would be false |

## Measured

Heights come from the real browser run on the final build, light theme, Chromium and WebKit gave the same numbers.

| Viewport | Popup content / visible | Opened feed row panel | Opened Activity row panel |
|----------|-------------------------|-----------------------|---------------------------|
| 320x640 | worst case 689 / 460, picture case 732 / 460 | 274px | 385px worst, 337px picture case |
| 360x740 | 689 / 560, 669 / 560 | 274px worst, 235px picture case | 274px |
| 390x844 | 650 / 650 (fits) | 235px | 274px |

Nothing clips and nothing scrolls sideways at 320, 360 and 390 in both engines (card and panel `scrollWidth` equals `clientWidth`, document `scrollWidth` equals `clientWidth`). At 200 percent text (320x640 and 390x844) nothing clips and nothing scrolls sideways, the popup scrolls inside itself, and the glyph stays on the first line. Reduced motion: every transition and animation on the card, the glyph and the close button is 0s. Forced colours: Chromium recolours fully (1px CanvasText card edge, black dividers, black discs with Canvas marks, all four shapes visible); WebKit only emulates the media query.

Popup height is the one thing to decide (see Open product questions): on a 360x740 phone about 109px of the popup, and on a 320x640 phone 229 to 272px, sit below the fold of the scrolling popup, so Confirm and Dispute need a scroll inside the popup.

## Tokens and contrast

Three tone tokens in all four theme blocks of `main.css`: `--color-tone-good` (light #3A7B4D, dark #34D399), `--color-tone-caution` (light #946200, dark #E5A93B), `--color-tone-bad` (is `--color-severity-critical`). The neutral glyph and the detail text use `--color-text-muted`. Rendered contrast on the card: light text 16.91, detail 4.83, good 5.10, caution 5.24, bad 5.44; dark text 17.68, detail 7.66, good 10.12, caution 9.33, bad 6.03. All text is at least 4.5:1 and every glyph pair at least 3:1, asserted for all four theme sources by `TestToneTokensClearTheContrastFloors`.

## How the front end knows the reader's own position

`app.js` keeps `state.center = {lat, lon, own}`. `setCenter(lat, lon, own)` stores `own === true`; the initial centre is not own. In `map.js`, a geolocation success boots the map with `own` true; denied, timeout and unsupported boot the fallback centre with `own` false. The address search lives in the report form and never sets the feed centre. `Pinalert.feedIsAroundReader()` reads the flag, and the popup meta line appends the distance only when it is true and `formatDistance(report.distance_km)` is not empty. This is the smallest flag that was needed: the centre source was not known before.

`formatDistance`: under 0.1 km "under 100 m away"; 0.1 to 9.95 one decimal ("0.4 km away"); 9.95 and up a whole number ("10 km away"). `formatFade` (from `report.expires_at`, which the feed carries): expired or missing gives nothing; one hour or more is whole hours rounded down ("Fades in 7 hrs.", exactly "Fades in 1 hr."); under an hour is minutes rounded up ("Fades in 45 min.").

## Proof

RED before any code: the Go build failed on the missing builder (`undefined: FactKind`, `undefined: TrustFact`, `undefined: FactKinds` and the tone names) with `trustfacts_test.go` written first. The web contract tests went in red first too: 8 new tests failed (`TestFactsListIsTheOnlySourceOfRows`, `TestToneTokensClearTheContrastFloors`, `TestToneGlyphsAreFourShapes`, `TestTrustCardIsAlwaysOpen`, `TestRowPanelHoldsTheFactsList`, `TestPopupBuildsTheCardAndTheFadeLine`, `TestSharedDistanceAndFadeFormatters`, `TestFactsCSS`) and all 142 old ones passed.

- Whole Go suite on the scratch database `p3_ticks`: FINAL_GO_COUNTS
- `go vet ./...` clean, `gofmt -l web internal cmd` lists nothing.
- The tool guard refused `env -i`, so the suite ran as `DATABASE_URL=... go test ./... -p 1 -count=1 -v`.
- Swagger: `make swag` changed 50, 50 and 46 lines, all of them the new `FactResponse` type and the new `facts` property.
- Browser harness: HARNESS_PENDING

## Pins changed on purpose

Go: `reporter_test.go` ("the signal changes nothing else" now drops the reporter row with the reporter signal, and the Trust field shape pin ends with Facts), `feed_test.go` (`p12WithoutReporter` also drops the reporter row), `trust_e2e_test.go` (the exact key set of the trust object gains `facts`).

Web (each one is explained in the Follow on cell of an amendment row, A89 and A91 to A94):

| Test | Old | New | Why |
|------|-----|-----|-----|
| TestTrustBlockMountedOnBothSurfaces | `map.js` builds `createTrustBlock({showScores: true})`; the block takes options; the panel is built from PANEL_LINES and the sentence fields | `map.js` builds `createTrustCard()` and holds no `createTrustBlock(`; the block takes no options; the panel calls `renderFacts` and reads no sentence field | A89, A91 |
| TestTrustModuleHasNoThresholdsOrSinks | numeric literals other than 0, 1, -1 banned on every code line | the same, except inside the one `TONE_GLYPHS` table | the SVG geometry (A93) |
| TestTrustDisclosure | the word `title` banned | `setAttribute('title'` and `.title =` banned | a fact has a title field |
| TestTrustDisclosure | Escape closes the row panel, the confirmation, then the popup | no panel level in the popup; `collapsePanelsIn` must not exist | A91 |
| TestTrustCSS | muted text and the severity tokens banned in trust rules; the panel is a four pixel gap grid of lines | muted text only on the detail, score and neutral glyph; the tone tokens only on the glyph and state word rules; the panel is a bordered white card | A93, A96 |
| TestTrustStateLineMarkup | `createReporterWords(showScores)` with a hidden ", score" prefix and a number | returns the element and the word only | A92 |
| TestPopupCardBuiltOnceUpdatedInPlace | `updatePopupCard` must not contain `distance_km` (A31) | must call `Pinalert.feedIsAroundReader()` and `Pinalert.formatDistance(report.distance_km)` | A94 |
| TestPopupDialogFocusAndOpener | `onPopupClose` calls `collapsePanelsIn(` | must not | A91 |
| trust_face_contract_test.go | `createTrustBlock(options)` | `createTrustBlock()` | no options |
| `.trust-block__panel` padding pin | `0 var(--space-sm)` | `0 var(--space-sm) var(--space-xs)` | card bottom air (proof task) |

The loosened colour pin of `TestTrustCSS` has a replacement guarantee, `TestToneTokensClearTheContrastFloors`.

## Documents that quote the old panel structure and were not edited

- `04-08-PLAN.md` line 178, `04-24-PLAN.md` line 352 and `04-27-PLAN.md` line 298 name "the trust toggle" in the popup Tab order. Phase 4 plans outside the brief's list; the Phase 4 UI contract carries the labelled correction (P4-A6).
- `03-VALIDATION.md` (the V-42 manual rows that say hover, tap and reasons) and `03-CONTEXT.md` (D-17 to D-20, locked): amended through the UI contract rows, not rewritten.
- `.claude/skills/sketch-findings-pinalert/references/trust-block.md` and `.planning/sketches/` quote the old toggle hint and the reporter words: forbidden for this plan, the skill needs an owner refresh to the new look.
- `README.md`: lines 17 to 19 say the popup shows 0 to 100 scores, which is still true as score lines, so it was not edited.
- `docs/swagger.*` examples of the old sentence fields stay: those fields are still on the wire.

## Open product questions

1. **Popup height on a phone.** The always open card makes the popup 650 to 732px tall. On 360x740 about 109px, and on 320x640 about 229 to 272px, is below the fold of the scrolling popup, so Confirm and Dispute need a scroll inside the popup. Options: a sticky actions bar (it touches the vote area, which this plan left unchanged), dropping the two score lines (39px, one rule), a shorter label, or a smaller autopan reserve.
2. **The fade line.** Does a fresh confirmation extend a report's life? Today nothing updates `expires_at`, so "unless confirmed again" would be false; the line says only when it fades. A trust model change for Phase 4 if the owner wants it.
3. **The popup tint.** The picture tints the whole popup with the severity colour; the app keeps a neutral popup (A10, owner Q4). Reversing it is one rule plus its pins.
4. **The state word.** "Report up to date" does not fit beside the counters at 340px, so it wraps under them. Dropping the word "Report" in the card would put it on one line, as drawn (a one line server change, but A86 chose the longer words).
5. **Score lines.** Keep "Score 92 out of 100." in the popup (D-09, A50) or drop them to save 39px.
6. **A lighter disc.** The picture's disc is lighter than our token because the state word shares it; a separate disc-only token would match the picture at 3:1.
7. **Server gap noticed, not in this plan.** `POST /api/reports` accepts a null latitude and longitude and files the report at 0,0 (the vote endpoints say "latitude and longitude are required").

## Deviations

- The fade line refreshes on the 30 second poll path with the popup's relative age text (`updatePopupCard`), not on the 60 second timer, which touches rows only and rows have no fade line.
- Words: "hrs" and "hr" (an owner change relayed during the run); "needs K confirmations" keeps the word confirmations (the brief wrote "needs K").
- For Hidden, Retracted and expired reports the confirmations row has no "Needs ..." detail for a count of 1 as well as 0 (the brief named only 0), because the line would be false.
- A resolved report that is also past its expiry shows no Expired status row (Retracted wins, the same precedence as the lead sentence and A44).
- The Gone quiet row drops "Gone quiet." from its detail, because the title already says it.
- The popup surface stays neutral and the white card gets a 1px border (see the table above).
- The builder agents were delegated parts of this plan (front end, documents, visual proof, harness); the orchestrator reviewed their reports and the diff.
