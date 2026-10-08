---
phase: 03-trust-model-hardening-diversity-weighted-trust
fixed_at: 2026-10-08
review_path: .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-FIX-VERIFICATION-2.md
iteration: 3
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 3: Code Review Fix Report, round 3

**Base:** 279c580. **Source:** 03-FIX-VERIFICATION-2.md (WR-01 V1 and V2, WR-08 V1 overlap and V2, WR-07 V2).

## Fixed Issues

### Item 1: write floor (WR-01 V1 and V2, WR-08 overlap)

**Commit:** 7b50c97
**Files:** web/static/js/app.js, web/static/js/votes.js, web/votes_contract_test.go, web/vote_controls_contract_test.go
**Change:** `fetchReports(options)` takes `{ afterWrite: true }`. Such a call raises `writeFloor` to its own sequence number when it STARTS (before the request is sent), so the floor holds whether the refetch later succeeds or fails. `isStale` now also drops any response with `mySeq < writeFloor`. The vote path (`votes.js` onControlsClick) and `submitReport` pass the option. Polls, Retry, pageshow and the map call `fetchReports()` with no argument and never raise the floor, so the slow link freeze stays closed.
**Pins:** `isStale` expression, `writeFloor = mySeq;` exactly once and ordered after `++fetchSeq` and before `fetch(url)`, `submitReport` passes `afterWrite: true`, the poll never does, the votes.js call is `{ afterWrite: true }`.

### Item 2: asymmetric drop (WR-08 V2)

**Commit:** 0403760
**Files:** web/static/js/app.js, web/static/js/feed.js (comment only, it said a failure never clears rows), web/votes_contract_test.go
**Change:** the failure branch now drops rows only when `rowsShowDisputed && !state.showDisputed` (old rows may hold disputed reports, the default view must not show them). Ticking Show disputed ON keeps the old rows (harmless subset). The error panel and Retry show in both cases. A stale response (flag changed again) still changes nothing, because the newer request in flight handles it.
**Pins:** the exact asymmetric condition, the old symmetric `!==` form must not return, and `rowsShowDisputed = sentDisputed;` is pinned inside the success write between the rows assignment and `lastAppliedSeq = mySeq;` (closes the second pin gap in the verification notes).

### Item 3: doc line (WR-07 V2)

**Commit:** 914ce5c
**Files:** docs/TRUST-MODEL.md
**Change:** one new bullet in "Honest limits": a coarse location fix (hundreds of metres to over a kilometre, for example a laptop on Wi-Fi) is sent as it is, so one spot can look like several places and a voter can be pushed across the 1 km line, which is why the place count is a rough measure and not proof of independent people. It says the system does not detect or block this. `TestTrustModelDocMatchesCode` passes unchanged (no constant or pinned sentence touched).

## Proof

Node sandbox outside the repo (`.../scratchpad/round3-sandbox.js`, not committed) runs the real app.js in a fake DOM and a controllable fetch.

At the base commit (`web/static/js/app.js` at 279c580), 3 of 11 checks FAIL:

```
FAIL S1 old poll cannot erase the recorded vote after a failed post vote refetch  (your_vote=null)
FAIL S1 status stays error so the saved-but-not-refreshed sentence survives  (status=ok)
FAIL S5 flag ON then request fails: rows kept  (rows=0)
FAILED 3 of 11 checks
```

On the final app.js:

```
PASS S1 old poll cannot erase the recorded vote after a failed post vote refetch
PASS S1 status stays error so the saved-but-not-refreshed sentence survives
PASS S2 old poll cannot overwrite a good post vote refetch
PASS S3 a slow plain poll still updates the list while a newer poll is in flight
PASS S3 the newer poll then applies
PASS S4 a poll started after the write refetch applies normally
PASS S5 flag ON then request fails: rows kept
PASS S5 flag ON then request fails: error state kept
PASS S6 flag OFF then request fails: disputed rows dropped
PASS S6 flag OFF then request fails: error state kept
PASS S7 same flag, request fails: rows kept
ALL PASS (11 checks)
```

Final run on scratch database p3_fix3 (created, used, dropped): `DATABASE_URL=postgres:///p3_fix3?sslmode=disable go test ./... -p 1 -count=1 -v` exit 0, 1232 `--- PASS`, no `--- SKIP`, no `--- FAIL`, every package `ok`. `go vet ./...` clean, gofmt clean on touched Go files, `node --check` clean on app.js, votes.js, feed.js. The flaky TestCastVoteConcurrentSameAccountKeepsEveryRow passed on the first run, no rerun needed.

## Notes

- The pre-existing `gofmt -l` hit on web/js_contract_test.go is untouched by this round.
- Source pins are shape tests; the behaviour proof is the Node sandbox, which stays outside the repo as the owner asked.

_Fixer: Claude (gsd-code-fixer), iteration 3_
