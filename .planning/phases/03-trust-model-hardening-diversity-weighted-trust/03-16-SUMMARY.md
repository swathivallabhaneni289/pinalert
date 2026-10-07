---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 16
subsystem: ui
tags: [javascript, css, severity-label, accessibility, forced-colors, contract-tests]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-13 trust.js module, trust.css section, p13 contract tests"
provides:
  - "Pinalert.SEVERITY_WORDS and Pinalert.formatShelterLine"
  - "PinalertTrust.createSeverityLabel and PinalertTrust.updateSeverityLabel"
  - "Visible Critical, Medium or Low word as the first child of every feed row body"
  - "Rounded 12px read only severity and state labels, vis-expired row treatment, forced colors label rules"
  - "Row meta in --color-text and a wrapping row title"
affects: [03-15, 03-17, 03-19, 03-20]
tech-stack:
  added: []
  patterns:
    - "One shared label builder in trust.js, word from one app.js table, class gated by hasOwnProperty"
    - "p16 media aware CSS parser and a four theme source contrast test"
key-files:
  created:
    - web/labels_contract_test.go
  modified:
    - web/static/js/app.js
    - web/static/js/trust.js
    - web/static/js/feed.js
    - web/static/css/trust.css
    - web/static/css/main.css
key-decisions:
  - "The two new trust.js exports sit before the four 03-13 entries; all six are present."
  - "The shelter integer check is a deliberate tightening of the old not null test, since the server sends an integer or null."
requirements-completed: [TRUST-05, TRUST-07]
status: complete
duration: 35min
completed: 2026-10-08
---

# Phase 3 Plan 16: Severity words, rounded labels and legible rows Summary

Every feed row now says Critical, Medium or Low in words as the first text of its body, the read only labels share one 12px rounded shape that no control uses, the row meta passes 4.5:1 on every tint in all four theme sources, a long unbroken title wraps, and expired Activity rows have their own inactive treatment.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 | Feed rows name severity in words, shared shelter line formatter | RED 2bc2b26, GREEN 4a64dd7 |
| 2 | Rounded read only labels, controls not, vis-expired, forced colors | RED a12c7bb, GREEN c172bd1 |
| 3 | Row meta colour, title wrap, contrast across four theme sources | RED 3175b76, GREEN cf77201 |

## RED then GREEN

- Task 1 RED: `TestRowSeverityLabelMountsFirst` failed with `feed.js createRow: missing "PinalertTrust.createSeverityLabel("`, `could not locate "function createSeverityLabel()"`; `TestShelterLineFormatterIsShared` failed with `app.js: SEVERITY_WORDS must be exactly ...` and `could not locate "function formatShelterLine(report)"`. GREEN: both pass together with the 03-13 tests (`TestTrustModuleHasNoThresholdsOrSinks`, `TestTrustBlockMountedOnBothSurfaces`, `TestFadeUsesCombinedStage`, `TestTrustDisclosure`) and every Phase 2 feed test in the verify list.
- Node formatter check over the real app.js printed `formatter ok`. `node --check` passes for app.js, trust.js and feed.js.
- Task 2 RED: all three tests failed (`no rule with head ".severity-label:not([hidden])" in media ""`, `trust.css has no .vis-expired rule (A44)`). GREEN: all three pass, plus `TestRetractedRowTreatmentMatchesHidden`, `TestVisibilityTagHiddenGuard`, `TestVisibilityCascadeOverridesAgeRamp` (still one hidden background rule), `TestTrustCSS`, `TestNoPillShapedControls` (untouched), `TestMapPopupTintTracksFeedRowAcrossStatesAndThemes` and the vote style tests.
- Task 3 RED: `TestRowTitleWrapsLongWords` failed with `row title: missing declaration "overflow-wrap"` and `row meta: color is "var(--color-text-muted)"`; `TestLabelAndMetaContrastAcrossThemes` failed on the meta text with 4.298 (sev-low), 4.372 (sev-medium), 4.123 (sev-critical) and 4.343 (vis-provisional) in both light sources. GREEN: both pass.
- Final `go test ./web/ -count=1` printed `ok  pinalert/web`. A `-v` run of the whole package gave 110 `--- PASS` and 0 `--- FAIL` or `--- SKIP`. DATABASE_URL was unset; no web test uses a database.
- Other gates: `go vet ./web/` clean, `gofmt -l web/labels_contract_test.go` prints nothing, the added lines of app.js, trust.js, feed.js, trust.css and main.css contain 0 occurrences of U+2014 or U+2013, `grep -c 'border-radius: 12px' web/static/css/trust.css` prints 2, the row label head grep prints 1, `grep -c '@media (forced-colors: active)'` prints 3.

## Notes for later plans

- 03-15: the CSS for `span.severity-label.severity-label--{level}` on `li.report-row`, `visibility-tag--expired` and `vis-expired` is in place, so the Activity template needs only the markup. The `.vis-expired` rule matches `.vis-retracted` property for property.
- 03-17: `PinalertTrust.createSeverityLabel()` and `updateSeverityLabel(label, report)` serve the popup header. The level rules already carry the popup fills (literal per level tokens), so the popup only positions the label. `Pinalert.formatShelterLine(report)` returns `''` for a non shelter report. Any new radius of half its height or more on a non exempt rule, or a severity colour in a rule that names a label, fails the 03-16 tests. 03-17 cannot edit `web/labels_contract_test.go`, so it raises a conflict instead.
- 03-19: the row label is read only text with no tabindex and no aria attribute, so the row focus order is unchanged.

## Deviations from Plan

- **Command substitution.** The sandbox refused compound git/loop one-liners, so the dash gate and the verify list were run as separate plain commands covering the same checks (`go test`, `node --check`, `node -e`, `go vet`, `gofmt -l`, and `git diff HEAD -- <files>` piped through grep and perl for the long dash count). The Go tests for Task 3 were appended with the Edit tool after a heredoc append was refused.
- **Export order.** The two new trust.js exports were placed before the four 03-13 entries rather than after; the four entries and their relative order are intact.
- No other deviations. Only the six files in files_modified changed; STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched. No database, server or port was used, and p3_0316 was never created.

## Known Stubs

None.

## Threat Flags

None. T-03-06 is mitigated: the severity value reaches `className` only after `hasOwnProperty` on the app.js word table, the word never comes from the payload, every string goes through `Pinalert.setText`, and the formatter builds the status word only from the closed status list and the headcount only from an integer.

## Human check (deferred)

The 320px wrapping check (feed body 236px, Activity body 188px), the 200 percent text size check and the severity-before-title screen reader check belong to plan 03-20's V-42 script. They were not run here.

## Self-Check: PASSED

Created and modified files exist: web/labels_contract_test.go, web/static/js/app.js, web/static/js/trust.js, web/static/js/feed.js, web/static/css/trust.css, web/static/css/main.css. Commits 2bc2b26, 4a64dd7, a12c7bb, c172bd1, 3175b76 and cf77201 exist on the worktree branch.
