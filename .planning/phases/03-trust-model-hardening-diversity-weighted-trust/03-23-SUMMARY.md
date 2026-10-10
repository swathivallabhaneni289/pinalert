---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 23
subsystem: ui
tags: [trust-block, sketch-003, variant-h, state-line, reporter-words, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: amendments A75 to A84 from plan 03-21 and the toggle face from plan 03-22
provides:
  - Plain reporter words beside the state label on the feed row, the map popup and the Activity row (UAT Gap 2)
  - One placement helper, PinalertTrust.createStateLine(className, tag, block), called once per surface
  - A popup header of a pills row plus a state line, with 16px to the title in the popup and 8px in rows
  - A rule that removes a state line whose children are all hidden (A83)
affects: [03-UAT Gap 2]
tech-stack:
  added: []
  patterns:
    - "Fixed class literal passed to a shared placement helper, one call per consumer"
    - "CSS :has() removes an empty line instead of script writes at every update site"
key-files:
  created:
    - web/trust_topzone_contract_test.go
  modified:
    - web/trust_contract_test.go
    - web/popup_contract_test.go
    - web/labels_contract_test.go
    - web/css_contract_test.go
    - web/static/js/trust.js
    - web/static/js/feed.js
    - web/static/js/map.js
    - web/static/js/activity.js
    - web/static/css/trust.css
key-decisions:
  - "The helper is createStateLine(className, tag, block); it writes no text and refreshes nothing"
  - "An empty state line is removed by CSS (A83), not by script"
  - "UAT Gap 3 (text too long) stays open and deferred (A84); no wording changed"
requirements-completed: []
status: complete
---

# Phase 3 Plan 23: Reporter words on the state line Summary

The reporter words are now plain text built outside the trust toggle and placed on one state line with the state label on all three surfaces, so UAT Gap 2 is closed in code and both trust signals show at once without opening anything.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 (RED) | 559be96 | test(03-23): pin the reporter words on the state line and the variant H spacing |
| 2 (GREEN, script) | e6c0724 | feat(03-23): reporter words beside the state label on every surface |
| 3 (GREEN, style) | b009bc4 | style(03-23): state line, plain reporter words and the variant H heading spacing |

Each commit lists only its planned paths (checked with `git show --stat --format=` after each commit).

## RED failure lines

Before any production change the six named tests behaved as the plan stated:

- TestTrustStateLineMarkup: `trust.js: missing "function createReporterWords(showScores)"`, missing `createStateLine` function and export, `trust.js: must not contain "trust-icon--user"` and `"trust-signal--second"`, then `could not locate "function createReporterWords(showScores)" in the text`.
- TestTrustStateLineStyle: `no rule with head ".trust-reporter:not([hidden])" in media ""`.
- TestTrustCSS: `no rule with head ".trust-block__count--second::before" in media "forced-colors: active"`.
- TestPopupCardSurfaceAndGeometry: header `column-gap`, `row-gap` and `margin-bottom` empty, `must not declare padding-right`, gap order failure, and `no rule with head ".popup-card__pills:not([hidden])"`.
- TestSeverityLabelHasNoFixedHeight: `row label: margin is "0 0 var(--space-xs)", want "0 0 var(--space-sm)"`.
- TestTrustBlockMountedOnBothSurfaces: PASS. The package compiled and vetted.

## Green run

- `go vet ./web/`: pass.
- Task 2 list (sixteen tests, including TestTrustStateLineMarkup and the unedited tests that pin the edited functions): ok.
- Task 3 list (TestTrustStateLineStyle, TestTrustStateLineMarkup, TestTrustCSS, TestPopupCardSurfaceAndGeometry, TestSeverityLabelHasNoFixedHeight, TestTrustToggleFaceStyle, TestReadOnlyLabelsRoundedControlsNot, TestVoteStyleSurvivesForcedColors, TestPhase3TrustGlyphMasksReferenceEveryExemptIcon): ok.
- `go test ./web/ -count=1`: ok.
- `gofmt -l web`: prints nothing.
- `node --check` passed for trust.js, feed.js, map.js and activity.js.
- No added line in any file of the plan holds an em or en dash.
- No database, server, browser or port was used. `go test ./...` was not run.

## What changed

- **trust.js.** New `createReporterWords(showScores)` (span `trust-reporter`, hidden, word span, popup-only score span with the visually hidden ", score" prefix and a plain number span). `createTrustBlock` builds the reporter through it and no longer appends the reporter or a second space to the signals row. New exported `createStateLine(className, tag, block)` after `createTrustBlock`. `updateTrustBlock` hides the words in its invalid branch and sets `block.signals.hidden = !state.currency`. The user icon class pair and the second signal modifier are no longer emitted.
- **feed.js, activity.js.** The state label is no longer appended on its own; the row body gets `createStateLine('report-row__stateline', tag, trust)` (`trustBlock` in activity.js) just before the trust block.
- **map.js, buildPopupCard.** The header holds a `popup-card__pills` row (badge, severity label) and then the state line built after the trust block. `updatePopupCard`, `pinName`, `onPopupClose`, the pointer flag and `categoryLabel` code from plan 03-27 were not touched.
- **trust.css.** Removed the second signal separator rule, the user icon rule and the second separator from the forced colours head. Added `.trust-reporter:not([hidden])`, the row and popup state line rules, the two A83 empty line rules, the 340px row reach, the pills rule, the header column-gap and row-gap and margin-bottom (no padding-right), and the row label margin `0 0 var(--space-sm)`.

## Test changes

Only the edits the plan names were made. No assertion was weakened beyond them.

| Test | Old | New |
| ---- | --- | --- |
| TestTrustBlockMountedOnBothSurfaces, trust.js needle list | needles `"trust-icon trust-icon--user"` and `"trust-signal--second"` | both removed (A76, A75); TestTrustStateLineMarkup pins the replacements and bans the retired names |
| TestTrustCSS, table | rows `.trust-signal--second::before` and `.trust-icon--user` | both rows removed |
| TestTrustCSS, forced colours separator | head `.trust-block__count--second::before, .trust-signal--second::before`, order check against `index[".trust-signal--second::before"]` | head `.trust-block__count--second::before`, order check against `index[".trust-block__count--second::before"]` |
| TestPopupCardSurfaceAndGeometry, header | wanted `padding-right: var(--space-2xl)` | wants `column-gap var(--space-sm)`, `row-gap var(--space-xs)`, `margin-bottom var(--space-md)`, no `padding-right`, `gap` declared before both longhands; new `.popup-card__pills:not([hidden])` pin holds `flex 1 1 100%` and the `padding-right` |
| TestSeverityLabelHasNoFixedHeight, row label | margin `0 0 var(--space-xs)` | margin `0 0 var(--space-sm)` (A81) |
| css_contract_test.go, comment above the user glyph entry | said the trust block reporter signal uses the glyph | says plan 03-23 (A76) removed it from the trust block and the account menu still uses it; entry kept |

New: `web/trust_topzone_contract_test.go` with TestTrustStateLineMarkup and TestTrustStateLineStyle.

## Deviations from Plan

None. The plan was executed as written. Every anchor (`createSignal`, `createTrustBlock(options)`, `updateTrustBlock`, feed.js `createRow`, map.js `buildPopupCard`, the activity.js row loop, the trust.css header, label and forced colours rules) was found where the plan said; only line numbers differed. One wording note: I shortened the trust.css forced colours comment to name the clock glyph and the counter separator.

## UAT Gap 3

Stays open and deferred (A84). No sentence, no `internal/` file, no template and no wording test was touched.

## Live proof still owed (orchestrator, after merge)

Words beside the label 8px apart at 320 and 360 wide on the three surfaces, heading to title 16px in the popup and 8px in rows, no line and no gap for a Live report with reporter none, every state label shown (T-03-23-02), and the Activity database tests plus TestProfileRendersSeverityLabel and TestProfileRowsCarryTrustJSON on a scratch database. This plan proves only the shipped text and the declared CSS.

## Known Stubs

None.

## Threat Flags

None. T-03-06, T-03-23-01 (one quoted class literal per consumer, tested), T-03-23-02 (two :has() heads only, pinned), T-03-23-03 (invalid branch hides the words) and T-03-23-04 (static text, no tabindex) are mitigated as planned.

## Self-Check: PASSED

- web/trust_topzone_contract_test.go exists.
- Commits 559be96, e6c0724 and b009bc4 exist on the worktree branch.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched.
