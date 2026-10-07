---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 18
subsystem: ui
tags: [votes-js, too-far-hint, aria-disabled, page-config, contract-tests]
requires:
  - phase: 03-09
    provides: createVoteBlock returns controls, note and error; note slot and setBlockBusy slot
  - phase: 03-11
    provides: trust.css styles for aria-disabled vote buttons and the visible vote note
  - phase: 03-13
    provides: script wiring on the feed page
provides:
  - PageConfig.VoteRadiusKm rendered as data-vote-radius-km on the feed page shell, sourced from service.MaxVoteDistanceKm
  - votes.js too far hint (aria-disabled, data-too-far, aria-describedby, visible sentence) on Confirm, Dispute and Mark resolved
  - click guard that ignores a button marked too far
affects: [03-17, 03-19, 03-20]
tech-stack:
  added: []
  patterns:
    - "Hint is decided on every updateVoteBlock render and fails open; the server stays the gatekeeper"
key-files:
  created:
    - web/vote_hint_contract_test.go
  modified:
    - internal/api/handlers/page.go
    - internal/api/handlers/page_test.go
    - cmd/server/main.go
    - web/templates/index.html.tmpl
    - web/static/js/votes.js
key-decisions:
  - "Strict greater than: a distance equal to the radius is not marked, matching the server's inclusive check"
  - "A non finite distance clears the hint (fail open); the server still refuses"
  - "VOTER_LOCATION_HINT_MS = 120000 (UI-SPEC open item 6) is a client timer, not a trust threshold"
  - "The hint never writes or clears the vote error; a forced vote still shows the server sentence there"
  - "Yes inside an open resolve confirmation is never marked; a press goes to the server"
requirements-completed: [TRUST-05, D-22]
duration: 25min
completed: 2026-10-08
status: complete
---

# Phase 3 Plan 18: Too far hint Summary

The feed page shell now carries the server's vote radius from `service.MaxVoteDistanceKm`, and `votes.js` marks Confirm, Dispute and Mark resolved with `aria-disabled`, `data-too-far` and `aria-describedby` plus a visible sentence when a fresh cached fix puts the viewer strictly farther than that radius.

## Tasks

| Task | Commit | Subject |
|------|--------|---------|
| 1 | 28127bd | feat(03-18): carry the server vote radius on the page shell |
| 2 | c29dd47 | feat(03-18): mark far reports too far with a visible reason |

## RED runs

- Task 1 RED: `go test ./internal/api/handlers/ -run TestPageShell` failed to compile: `unknown field VoteRadiusKm in struct literal of type PageConfig`.
- Task 2 RED: `TestVoteHint`, `TestVoteHintSentenceMatchesServer` and `TestVoteRadiusReadWithoutConfig` failed on the missing anchors (`could not locate "function applyDistanceHint(block, report)"`, `could not locate "function readVoteRadiusKm()"`, the four constants occurring 0 times).

## Final PASS lines

- `--- PASS: TestPageShellCarriesVoteRadius` (with the four existing TestPageShell tests, `ok pinalert/internal/api/handlers`)
- `--- PASS: TestVoteHint`, `--- PASS: TestVoteHintSentenceMatchesServer`, `--- PASS: TestVoteRadiusReadWithoutConfig`; `go test ./web/ -count=1` gives `ok pinalert/web`
- `node --check web/static/js/votes.js` exits 0; `go vet ./cmd/server/ ./internal/api/handlers/ ./web/` clean; gofmt clean on the plan's Go files.
- `go test ./internal/service/ ./web/ -short -count=1`: both `ok`.

## Names added to votes.js (module private, do not reuse in 03-17 and 03-19)

Vars: `VOTER_LOCATION_HINT_MS`, `HINT_BUTTON_SELECTORS`, `TOO_FAR_LEAD`, `TOO_FAR_TAIL`.
Functions: `peekVoterLocation`, `readVoteRadiusKm`, `greatCircleKm`, `clearDistanceHint`, `applyDistanceHint`.
No export was added to `window.PinalertVotes`. The `web` tests now import `pinalert/internal/service` (no import cycle).

Go test helpers: `p18AppShellTag` (handlers package), `p18ReadStatic`, `p18ReadTemplate`, `p18TightBody`, `p18Before`, `p18Contains`, `p18Excludes`, `p18Once` (web package).

## Decisions

- Strict greater than comparison on the distance; fail open on a non finite distance, a missing or zero radius, a missing or stale fix.
- The 120 second hint bound comes from UI-SPEC open item 6; it is a client timer.
- The hint never writes or clears the vote error. Yes inside an open confirmation stays live.
- The reporter's own report always clears the hint; the Activity page template carries no radius attribute, so no hint there.

## Known limit for 03-20's V-42 walk

A far hint can hold for up to the 120 second bound plus one 30 second poll. While it holds, a tap cannot trigger D-02's silent refresh, so a viewer who walked closer waits for the bound to pass before voting.

## Deviations from Plan

1. [Rule 3 - Blocked command] The verify commands start with `env -u DATABASE_URL go test ...`. `echo "DATABASE_URL=${DATABASE_URL:-unset}"` printed `unset`, so I ran the same `go test` without the env prefix, and split the compound verify one-liner into separate commands (go test, go vet, gofmt, node --check).
2. [Tooling] The editor tools turned the ` ` escape in `votes.js` and the Go escapes in the new test file into literal characters. I restored the escapes with perl and re-checked that no added line holds U+00A0, U+2014 or U+2013 as a literal.
3. Tests for each task were committed with the task rather than as separate RED and GREEN commits (the plan defines one commit per task).

Human check (browser two-profile hint walk with PORT=8792 and p3_0318_uat) was deferred to plan 03-20's V-42 script. No database was created or touched; p3_0318 was never created.

## Known Stubs

None.

## Threat Flags

None. The new surface is a server rendered numeric attribute and a read of the sessionStorage cache; both are covered by T-03-06 and T-03-09.

## Self-Check: PASSED

- web/vote_hint_contract_test.go, web/static/js/votes.js, internal/api/handlers/page.go, cmd/server/main.go, web/templates/index.html.tmpl exist with the changes.
- Commits 28127bd and c29dd47 exist; each lists only files from files_modified.
