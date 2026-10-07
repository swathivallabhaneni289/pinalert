---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 13
subsystem: ui
tags: [javascript, css, trust-block, accessibility, forced-colors, contract-tests]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: "03-07 trust wire object, 03-09 PinalertVotes.cancelResolveConfirm and vote block order, 03-11 phase3TrustGlyphIcons and the trust.css vote section"
provides:
  - "window.PinalertTrust with createTrustBlock, updateTrustBlock, fadeStage, focusToggle"
  - "Trust block on every feed row and map popup, both counters always shown, signals when the server sends them, popup-only scores"
  - "Combined fade stage (worse of time stage and server stage) at all five fade sites"
  - "One capture phase Escape listener resolving one level per press on rows"
  - "trust.css section, clock.svg, six pinned static contract tests"
affects: [03-14, 03-15, 03-16, 03-17, 03-19, 03-20]
tech-stack:
  added: []
  patterns:
    - "One builder for every surface, server-authored strings only, closed enum allowlists"
    - "p13 static contract helpers with a media-aware CSS parser"
key-files:
  created:
    - web/static/js/trust.js
    - web/static/icons/clock.svg
    - web/trust_contract_test.go
  modified:
    - web/static/js/feed.js
    - web/static/js/map.js
    - web/templates/index.html.tmpl
    - web/static/css/trust.css
    - web/css_contract_test.go
key-decisions:
  - "The panel sits as a sibling of the toggle button; the toggle holds only phrasing spans plus the visually hidden hint."
  - "Comments in trust.js avoid the words the contract tests ban, because jsFunctionBody includes the comment above the next function in a body."
requirements-completed: [TRUST-05, TRUST-07, D-07, D-09, D-14, D-16, D-17, D-18, D-19, D-20, R-02, R-03, R-07]
status: complete
duration: 40min
completed: 2026-10-07
---

# Phase 3 Plan 13: Shared trust block Summary

A single trust block builder now renders both counters (zeros included), the server's still current word and reporter tag, popup-only scores and a panel of server sentences on every feed row and map popup, and all five fade sites take the worse of the time stage and the server stage.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Counters and signals on every row and popup, panel by click | 99dc9b2 |
| 2 | One Escape per level for a row panel or an open confirmation | 12df9e4 |
| 3 | Trust block look, clock glyph, forced colours and reduced motion | d9ec4e6 |

## RED then GREEN

- Task 1 RED: all four tests failed. `TestTrustModuleLoadsBeforeItsConsumers`: `index.html.tmpl references /static/js/trust.js 0 times, want exactly 1`. The other three: `failed to read embedded static/js/trust.js: file does not exist`. GREEN: `go test ./web/ -run 'TestTrust|TestFadeUses'` gave `ok  pinalert/web`, plus the Phase 2 mount and order tests from the verify list, all `--- PASS`.
- Task 2 RED: `TestTrustDisclosure` failed with `trust.js code: missing "'Escape'"`, `missing "preventDefault"` and `has 0 document.addEventListener( calls on code lines, want exactly 1 (A23)`. GREEN: pass, together with `TestEscapeCancelHookIsExported` and `TestVoteClickDoesNotActivateItsRow`.
- Task 3 RED: `TestPhase3TrustGlyphMasksReferenceEveryExemptIcon` failed with `static/icons/clock.svg: no rule in trust.css declares both -webkit-mask-image and mask-image as url("/static/icons/clock.svg")`, and `TestTrustCSS` failed with `no rule with head ".trust-block:not([hidden])" in media ""`. GREEN after the stylesheet section.
- Final `go test ./web/ -count=1`: `ok  pinalert/web`. A `-v` run of the whole package shows no `--- FAIL` and no `--- SKIP` (no web test uses the database). All six pinned tests `--- PASS`: TestTrustModuleLoadsBeforeItsConsumers, TestTrustBlockMountedOnBothSurfaces, TestFadeUsesCombinedStage, TestTrustDisclosure, TestTrustCSS, TestTrustModuleHasNoThresholdsOrSinks.
- Other gates: `node --check` parses trust.js, feed.js, map.js; `go vet ./web/` clean; `gofmt -l web/trust_contract_test.go web/css_contract_test.go` prints nothing; `grep -c 'PinalertTrust.fadeStage(report)'` prints 2 for feed.js and 3 for map.js; `grep -c '/static/js/trust.js'` prints 1 in the template; `grep -c 'url("/static/icons/clock.svg")'` prints 2; `grep -c 'd="M12 6v6l4 2"'` prints 1; `grep -c '"static/icons/clock.svg": true'` prints 1; the Escape listener grep prints 1.

## Fade call sites replaced

Five, one for one: feed.js `updateRow` and `refreshAgeAndTime`; map.js `buildBadgeElement`, `popupStateClasses`, `syncPopupState`. The one remaining `Pinalert.ageStage(` call outside app.js is inside `fadeStage` in trust.js.

## Long dash counts

trust.css has 35 lines with a long dash, all inside comments, the same count 03-11 left (the plan text said 37, the measured value before and after this plan is 35). My new CSS comments have none, and the stripped stylesheet has none. trust.js, trust_contract_test.go and clock.svg have 0. The feed.js header edit kept line 19 byte identical up to the phrase and added no dash.

## Notes for later plans

- 03-14: the block already renders currency and reporter whenever the wire carries a non empty label, so 03-14 needs no browser change.
- 03-15: `PinalertTrust.createTrustBlock()` with no options and `PinalertTrust.focusToggle(blockOrRow)` serve the Activity page. `updateTrustBlock(block, result.trust)` refreshes after Reopen. The Escape listener already treats an Activity `li.report-row` as a row owner.
- 03-16 (edits trust.js, cannot edit trust_contract_test.go): no quoted `'vis-'`, `'sev-'` or `'age-'` literal; no server copy string; no numeric literal above 1 on a code line; full line comments only; no long dash anywhere; functions at the IIFE top level; keep the four export entries. Also keep comments above a function free of `aria-expanded`, `setPanelOpen(` and the banned words, because `jsFunctionBody` counts the comment above the next function as part of the previous body.
- 03-17: the popup block is built and filled per render inside `buildPopupContent`; the build once card keeps a popup panel open across polls. `TestTrustBlockMountedOnBothSurfaces` anchors the popup order on first occurrences in map.js and on the `{ showScores: true }` call, never on the builder's name.
- 03-19: extend `closeOneLevel()` with popup ownership and the popup close step, keep `document.addEventListener(` at exactly one occurrence, and keep the order anchors `'report-modal'`, `'account-menu'`, `PinalertVotes.cancelResolveConfirm(`, `closeRowPanel(`.

## Deviations from Plan

- **Tooling, not code.** The Write and Edit tools turned the Go escapes for the two long dash code points in `web/trust_contract_test.go` into the literal characters twice. I restored the escapes with a perl substitution and rechecked that the file has no long dash.
- **Command substitution.** The sandbox refused a heredoc append of Task 2's test and a compound git/echo orientation command, so I appended with the Edit tool and ran the checks as separate plain commands.
- **Verify split.** I ran the Task 1 verify list as separate `go test`, `node --check`, `go vet` and `gofmt` commands that check the same things.
- No other deviations. Only the eight files in files_modified changed, and STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched. No database, server or port was used, and p3_0313 was never created.

## Known Stubs

None. The block renders currency and reporter only when the wire carries them; until 03-14 puts them on the wire the second line stays hidden, which is the specified behaviour.

## Threat Flags

None. T-03-06 is mitigated: every string goes through `Pinalert.setText`, `TestTrustModuleHasNoThresholdsOrSinks` fails on any markup sink, `validTrust` hides a malformed object, and the closed allowlists gate every enum and the fade class.

## Human check (deferred)

The visual and screen reader checks for this block (panel by click, tap, Enter, Space and Escape on a row; the hint read with the toggle name; 320px wrapping; the press fill on a Provisional row; forced colours; both themes) belong to plan 03-20's V-42 session.

## Self-Check: PASSED

Created files exist: web/static/js/trust.js, web/static/icons/clock.svg, web/trust_contract_test.go. Commits 99dc9b2, 12df9e4 and d9ec4e6 exist on the worktree branch.
