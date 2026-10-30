---
phase: quick-261030-v5f
plan: 01
subsystem: web demo
tags: [demo, map-pin, css, contract-tests]
requires: []
provides:
  - "A Demo map pin drawn exactly like a crowd pin, with no ring"
affects:
  - web/static/css/demo.css
  - web/static/js/map.js
tech-stack:
  added: []
  patterns: ["absence assertions over every embedded stylesheet"]
key-files:
  modified:
    - web/demo_contract_test.go
    - web/official_pins_contract_test.go
    - web/static/js/map.js
    - web/static/css/demo.css
    - web/static/css/official-pins.css
    - web/static/js/official-pins.js
    - docs/OFFICIAL-ALERTS.md
decisions:
  - "Owner decision 2026-10-30: make the Demo pins like how they look in the real website"
metrics:
  tasks: 2
  files: 7
  completed: 2026-10-30
status: complete
---

# Quick 261030-v5f: Demo pins look like the real site Summary

A Demo map pin is now the plain crowd pin: the dashed ring rule in demo.css and the Demo class that map.js added to the badge are gone, while the Demo strip, the Demo tag and the pin name that starts with Demo. still mark the Demo.

## Why

Owner decision 2026-10-30 while trying the Demo: "make them like how they look in the real website". In dark mode the ring was messy where pins overlap. This supersedes the Phase 5 plan 05-13 ring (D-02, UI-SPEC 4b); those planning documents are not edited.

## What changed

- web/static/js/map.js: buildBadgeElement lost its five line Demo block. pinName still prefixes Demo. on the Demo page.
- web/static/css/demo.css: the whole "The Demo pin" section and its copy inside the forced colours block removed; header reworded (strip, Exit demo, Demo tag, alert banner Demo word, a Demo pin drawn like a crowd pin). No template or load order change.
- web/static/css/official-pins.css, web/static/js/official-pins.js, docs/OFFICIAL-ALERTS.md: wording only, the GDACS pin is told apart from the crowd and Demo pin by shape and word.
- Tests (names kept): TestDemoMarksInFeedAndMap now forbids any icon-badge rule, outline property or dashed value in demo.css, forbids the Demo class and the PinalertDemoPage branch in map.js and requires the Demo. pin name. TestOfficialPinDistinctFromCrowdAndDemo now walks every embedded stylesheet for a Demo pin rule head, forbids the Demo branch in buildBadgeElement and requires pinIcon to write the word GDACS.

## RED then GREEN

RED (commit 33d1838c): both tests failed, every failure line a new absence check: the Demo class and PinalertDemoPage in buildBadgeElement, icon-badge--demo in map.js, the demo.css pin rule head (top level and forced colours copy), its outline, outline-offset, outline-color and the dashed value. GREEN (commit c3be249d): both pass, go test ./web/ ok.

## Commits

- 33d1838c test(261030-v5f): pin that a Demo pin has no ring and is the crowd pin
- c3be249d feat(261030-v5f): Demo pins look like the real site, no ring
- ae5161b1 docs(261030-v5f): the Demo is marked by the strip, the tag and the pin name
- this summary, committed alone last

Base: 5c58f1b68d45bef87bbc5ae0a324ddb71c6020fc.

## Verification

- Scratch database q_dring_a (created, dropped, confirmed gone): go test ./... -p 1 -count=1 -v: 6836 PASS lines, 0 FAIL, 0 panic, 21 packages ok, only skips TestGeocodeLive, TestOfficialLive, TestScreeningFixture.
- node --test web/*.mjs: 1665 tests, 1665 pass, 0 fail.
- Postcondition grep for icon-badge--demo or "dashed ring" under web/static and docs: exactly one line, web/static/js/map.js:1019, the real site's Expired pin ring comment, kept on purpose. Repository wide (outside .planning) the only other hits are the two contract test files (absence assertions and the official pin's own no dashed ring checks). No node test references the class.
- gofmt clean on the two test files; no em or en dash on any added line; neutral wording grep empty; no migration; no server started; STATE.md, ROADMAP.md, REQUIREMENTS.md never staged.

## Deviations from Plan

None. The plan executed as written. Only the sandbox forced one mechanical difference: test commands ran with an inline DATABASE_URL assignment (after confirming DATABASE_URL and PGDATABASE were empty) and output redirected to a file instead of tee, because the sandbox refused the compound forms.

## Accepted residual

T-v5f-01: on the map alone a Demo pin is now indistinguishable from a crowd pin; the Demo strip on every Demo screen, the Demo tag on rows and popups and the pin name starting with Demo. remain the marks, and the real page loads no Demo file.

## Known Stubs

None.

## Self-Check: PASSED

Commits 33d1838c, c3be249d and ae5161b1 exist in git log; the three changed code files and docs are in the committed diff.
