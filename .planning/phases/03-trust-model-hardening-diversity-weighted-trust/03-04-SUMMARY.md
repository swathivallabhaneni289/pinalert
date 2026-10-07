---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 04
subsystem: trust
tags: [voting, http-403, geolocation, permissions-api, swagger, go, javascript]
requires:
  - "03-01: ErrVoterTooFar, TooFarMessage, MaxVoteDistanceKm in internal/service/geo.go"
provides:
  - "HTTP 403 with error field location and the fixed too far sentence on all four vote routes"
  - "OpenAPI description of the 1 km rule on the four vote routes"
  - "votes.js getVoterLocation(opts) with a timestamped sessionStorage cache, a 60 second reuse bound and a silent refresh under a granted permission"
  - "castVote with one bounded retry keyed on error field location"
  - "parseVoterLocation, isFreshVoterLocation, geolocationPermissionState helpers for plan 03-18"
affects: [03-18, 03-20]
tech-stack:
  added: []
  patterns:
    - "sentinel error mapped in the handler's errors.Is switch ahead of errors.As"
    - "client retry keyed on the server's error field, never on message text"
    - "static contract test over the embedded JavaScript, bounded by jsFunctionBody"
key-files:
  created: []
  modified:
    - internal/api/handlers/votes.go
    - internal/api/handlers/votes_e2e_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - web/static/js/votes.js
    - web/votes_contract_test.go
key-decisions:
  - "A fresh retry rejects rather than reusing the stale fix, so no second POST goes out with the same coordinates, and the original server sentence is shown."
  - "The Permissions API query lives only in getVoterLocation's helper; castVote holds no permission logic and no loop."
requirements-completed: [TRUST-05]
duration: about 40 minutes
completed: 2026-10-07
status: complete
---

# Phase 3 Plan 04: Too far is visible Summary

A vote cast from more than 1 km from the report now answers HTTP 403 with error field `location` and the fixed sentence "Too far to vote. You need to be within 1 km of this report.", and the browser refreshes an already granted location silently and retries a too far vote once, never prompting per vote.

## What was built

- **Task 1** (`0b20b3d`): `votes.go` maps `service.ErrVoterTooFar` to `writeFieldError(w, http.StatusForbidden, "location", service.TooFarMessage())`, placed in the sentinel switch ahead of the `errors.As` branch. Three `@Description` lines document the rule. `make swag` regenerated `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml`. `TestCastVoteTooFar` (V-07) covers all four routes from 2.0015 km, the exact two key error shape, no "distance" in the body, zero vote rows after the refusals, a confirm from 0.3336 km (provisional), the reporter's resolve from Chennai (290.17 km, retracted), and an expired report voted on from far answering 409.
- **Task 2** (`c660c42`): `votes.js` gained `VOTER_LOCATION_FRESH_MS = 60000`, `parseVoterLocation`, `isFreshVoterLocation`, `geolocationPermissionState` and `getVoterLocation(opts)`. `castVote` now returns `getVoterLocation({ fresh: false }).then(postVote)` with a catch that retries once, only when `err.field === 'location'`, through `getVoterLocation({ fresh: true })`. A nested `postVote(coords)` builds the body from exactly latitude and longitude. `TestVoteTransportRefreshesAndRetriesOnce` (V-35) was added and `TestVoterLocationIsCachedPerSession` was re-anchored to `getVoterLocation(opts)`.

## Verification

- RED run, handlers package on a fresh `p3_0304`: `expected 403 on /confirm from 2 km, got 500: internal server error` (and the same for dispute, resolve, reopen), `--- FAIL: TestCastVoteTooFar`.
- GREEN run, handlers package on a fresh `p3_0304` with `-v`: `--- PASS: TestSwaggerSpecCoversRoutes`, `--- PASS: TestCastVoteTooFar` (confirm, dispute, resolve, reopen and "expired_report_from_far" subtests all PASS), `ok  	pinalert/internal/api/handlers	8.209s`. No `--- FAIL` and no `--- SKIP` lines. `p3_0304` was dropped afterwards.
- `make swag` diff: only the three new description lines in each of the four vote route descriptions (confirmed on `swagger.yaml`, 12 added lines; `docs.go` and `swagger.json` 8 changed lines each). A scratch `swag init` matched `swagger.json` and `swagger.yaml` byte for byte; `docs.go` differed only in the package line (`docs04` versus `docs`), as the plan predicted.
- `error field is location` appears 4 times in `docs/swagger.json` and 4 times in `docs/docs.go`.
- RED run for Task 2 (`go test ./web/ -run 'TestVoteTransport|TestVoterLocation'`): `could not locate "function getVoterLocation(opts)" in the text`, `expected exactly one navigator.permissions.query call in the module, found 0`. After the change: `ok  	pinalert/web	0.613s` for the whole package, and `ok pinalert/web` plus `ok pinalert/internal/service` with `-short`.
- `node --check web/static/js/votes.js` passes. `go vet ./web/` and `go vet ./internal/api/...` are clean. `gofmt -l` on `votes.go` and `votes_e2e_test.go` prints nothing; the two new or edited web test functions pass `gofmt -l` when extracted.
- Counts in `votes.js`: `function getVoterLocation(opts)` 1, `navigator.geolocation.getCurrentPosition(` 1, `fetch(` 1, `navigator.permissions.query(` 1, `var VOTER_LOCATION_FRESH_MS = 60000;` 1, `getVoterLocation({ fresh: true })` 1, `getVoterLocation(` on 3 lines, `at: Date.now()` 1.
- Long dash lines: `votes.go` 8 (limit 8), `votes_e2e_test.go` 12 (limit 12), `docs/swagger.json` 7 (limit 7), `votes.js` 23 (limit 27), `votes_contract_test.go` 142 (limit 142).

## Retry design

The first attempt calls `getVoterLocation({ fresh: false })`. If the server answers with error field `location`, castVote makes one retry with `{ fresh: true }`. A fresh request never reuses the stale cached fix and never prompts: under `granted` it takes a silent fix with `maximumAge: 0`; under `denied`, `prompt` or `unknown` it rejects. On such a rejection the original server error is rethrown, so the user sees the server's too far sentence and no second POST goes out with the same coordinates. The retry's own failure propagates unchanged, so there is at most one extra POST per tap. The vote block stays busy across both attempts because `onControlsClick` already sets busy before calling castVote.

## Known browser limits (A7 class)

- Where the Permissions API is missing, `geolocationPermissionState` resolves `unknown`, the silent refresh does not run and a cached fix is reused.
- Firefox commonly reports `prompt` for geolocation unless the user chose to remember the decision. In that state a voter who has moved keeps the cached fix and sees the too far sentence; the browser is not asked again.
- Static contract tests prove the code shapes only. The Chrome two profile check in the plan's human-check block is left for the end of phase run (plan 03-20, V-42) and was not performed here.

## For plan 03-18

- Cache format in sessionStorage key `pinalert_voter_location`: `{ latitude, longitude, at }`, with `at` in epoch milliseconds. An older entry without `at` parses with `at: null` and counts as stale.
- Module private helpers: `parseVoterLocation(raw)`, `isFreshVoterLocation(entry, maxAgeMs)`, `geolocationPermissionState()`, constant `VOTER_LOCATION_FRESH_MS`. `getVoterLocation(opts)` is still exported as `PinalertVotes.getVoterLocation`.

## Deviations from Plan

- Tooling substitutions only. The sandbox refused the plan's long compound verify one-liners and any command combining `git` with other constructs, so the Task 1 and Task 2 automated verify blocks were run as separate plain commands (dropdb, createdb, go test, grep counts, gofmt on extracted functions written to the scratchpad) that check the same things. `echo "DATABASE_URL=${DATABASE_URL:-unset}"` printed `unset`, so the no-database `go test` runs used no env prefix.
- The scratch `swag init` was written to the session scratchpad rather than `mktemp -d`; the only diff in `docs.go` is the package name line.

No other deviations. The plan executed as written.

## Known Stubs

None.

## Threat Flags

None. T-03-01 is mitigated and proven over HTTP by `TestCastVoteTooFar`; T-03-03 is accepted as the plan states.

## Self-Check: PASSED

- FOUND: internal/api/handlers/votes.go, internal/api/handlers/votes_e2e_test.go, docs/docs.go, docs/swagger.json, docs/swagger.yaml, web/static/js/votes.js, web/votes_contract_test.go
- FOUND commits: 0b20b3d, c660c42
- Disposable database p3_0304 dropped. No server started. STATE.md, ROADMAP.md and REQUIREMENTS.md untouched.
