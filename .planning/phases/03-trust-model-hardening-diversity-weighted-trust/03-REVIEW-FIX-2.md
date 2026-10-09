---
phase: 03-trust-model-hardening-diversity-weighted-trust
fixed_at: 2026-10-08T00:00:00Z
review_path: .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-FIX-VERIFICATION-1.md
iteration: 2
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 3: Code Review Fix Report, round 2

**Source:** `03-FIX-VERIFICATION-1.md` (independent skeptics on the round 1 fixes)
**Base:** `ef2376e`. Five commits plus this report. WR-02, WR-04 and WR-06 were judged sound and are untouched. WR-05 is out of scope by instruction.

## Fixed

### 1. WR-01 (V1, V2, V3): drop only responses older than the last applied one

**Commit:** 0fdbc1e `fix(03): WR-01 follow up drop only responses older than applied`
**Files:** `web/static/js/app.js`, `web/votes_contract_test.go`
**What changed:** `fetchReports` keeps `lastAppliedSeq` (set with the success write). A response is dropped only when `mySeq < lastAppliedSeq` or it was sent with another Show disputed flag. A request merely starting later no longer drops anything, so a link slower than the 30 s poll still renders and still raises the error. A dropped call resolves at once when newer data is already applied; when it is dropped for the flag it resolves with the newest call's promise (`newestFetch`), so the vote busy release in `votes.js` and `submitReport` never treat the store as fresh before the newest data lands. Failures do not move `lastAppliedSeq`.
**Test:** new `TestFetchReportsDropsOnlyResponsesOlderThanTheLastApplied` (exact guard expression, no `mySeq !== fetchSeq`, both paths drop through `whenFresh()`, `lastAppliedSeq` set once with the success write, `newestFetch = pending`).
**Node proof** (fake `fetch` with controllable responses, run against the real `app.js`; the same script against `ef2376e` fails 4 checks and then crashes on the last):
```
PASS older response applied when no newer response has landed
PASS older response after newer applied is dropped
PASS slow link keeps updating to newest
PASS failure of an older call is shown when nothing newer applied
PASS dropped call has NOT resolved while newest is in flight
PASS dropped call resolves after newest lands, store holds newest
PASS older call resolves at once when newer data is applied
ALL PASS
```

### 2. WR-03 (V1, V2): the tap uses the fix it just took

**Commit:** 53d7f1a `fix(03): WR-03 follow up tap uses the fresh fix it just took`
**Files:** `web/static/js/votes.js`, `docs/TRUST-MODEL.md`, `web/votes_contract_test.go`, `web/vote_hint_contract_test.go`, `web/vote_controls_contract_test.go`
**What changed:** `refreshHintThenRetry` takes the coordinates `getVoterLocation({ fresh: true })` resolves with, recomputes the too far mark from them (`applyDistanceHint(block, report, justTaken)`), and if the button is no longer marked re-enters `onControlsClick(block, button, taken)`, which passes `location: taken` to `castVote`. `castVote` accepts that option only as a finite number pair and sends it with no second GPS fix. A coarse fix (not cached, WR-07) therefore serves its tap. The producer `block.lastReport = report` is pinned. TRUST-MODEL.md line on the too far hint updated.
**Node proof** (fake DOM and geolocation, real `votes.js`; the same script on `ef2376e` fails the three vote checks):
```
PASS button starts marked too far from the cached far fix
PASS one GPS fix taken for the tap (no second fix)
PASS vote was sent
PASS vote carries the coordinates the fresh fix resolved with
PASS coarse fix was still not cached
PASS button no longer marked
PASS still too far: nothing sent
PASS no fix: nothing sent, mark stays
PASS block.lastReport is the report passed to updateVoteBlock
ALL PASS
```
**Tests changed on purpose:** the anchors `onControlsClick(block, button)` (5 tests in `vote_controls_contract_test.go`, `vote_hint_contract_test.go`, `votes_contract_test.go`) became `onControlsClick(block, button, taken)` and `applyDistanceHint(block, report)` became `applyDistanceHint(block, report, justTaken)` because the signatures grew an argument; the body assertions after them are unchanged. The refresh pin `applyDistanceHint(block, block.lastReport)` became `applyDistanceHint(block, block.lastReport, taken)` and gained exclusions of `peekVoterLocation(` and `sessionStorage` in that function. The pin `{ ownReport: block.isOwnReport === true }` became `{ ownReport: block.isOwnReport === true, location: taken }`.
**Residual (cosmetic):** after a vote sent from an uncached coarse fix, the next render re-reads the older far fix from the cache and may mark the buttons again until that fix is 120 s old. The vote itself is not affected.

### 3. WR-07 (V1): retry once before denying

**Commit:** 14a2d88 `fix(03): WR-07 follow up retry once before denying on a timeout`
**Files:** `web/static/js/votes.js`, `docs/TRUST-MODEL.md`, `web/votes_contract_test.go`
**What changed:** `requestPosition` calls `attempt(true)`. On error, only `err.code === GEOLOCATION_PERMISSION_DENIED` (1) ends the request at once; a timeout or position unavailable on the high accuracy attempt gets exactly one `attempt(false)` (`enableHighAccuracy: false, timeout: 8000, maximumAge: 0`), a real fix taken now. If that also fails the vote is still turned away with the location sentence, because there is never a substitute position. The 300 m never-cache rule is untouched; V2 (desktop fix half) is the documented inherent limit and was not changed.
**Tests (T1 included):** `VOTER_LOCATION_MAX_CACHE_ACCURACY_M` must appear exactly twice (declaration and the gate), the `sessionStorage.setItem` must sit behind `typeof accuracy === 'number' && accuracy <= VOTER_LOCATION_MAX_CACHE_ACCURACY_M` and before `resolve(coords)`, and the retry shape (`attempt(true)`, one `attempt(false)`, the exact error branch, the constant) is pinned. The existing options literal pin is unchanged because the true branch keeps the literal text.
**Node proof** (same script on `ef2376e` fails 11 checks):
```
PASS [prompt] timeout then a default accuracy fix resolves
PASS [prompt] first attempt high accuracy, second default accuracy
PASS [prompt] the 300 m never cache rule still holds
PASS [prompt] PERMISSION_DENIED denies at once
PASS [prompt] two failed attempts end in the denial sentence, never a third attempt
PASS [granted] (same five checks)
ALL PASS
```

### 4. WR-08 (V1 to V4): the list never disappears, a saved vote is flagged

**Commit:** b5d261c `fix(03): WR-08 follow up keep the list and flag a saved vote`
**Files:** `web/static/js/app.js`, `web/static/js/feed.js`, `web/static/js/votes.js`, `web/votes_contract_test.go`
**What changed:**
- V1, V3: `isBackgroundRefresh = state.status === 'ok' || state.reports.length > 0`. A poll or Retry after a failure with rows on screen stays a background refresh, so `render` never reaches the loading branch and never hides the list again. (An empty ok list still polls without a skeleton flash, as before.)
- V2: the store remembers the flag its rows were fetched with (`rowsShowDisputed`). A failed refetch under another flag drops those rows, so a disputed set never sits under an unticked box.
- Focus: the error branch with no rows removes the leftover rows through `clearRows()`, which restores focus (map, view toggle, empty state) as the empty branch always did.
- V4: after the post-vote refetch, if the store is in `error`, the row shows "Your vote was saved, but the list could not refresh. The counts shown may be out of date." in its alert paragraph, and for Confirm and Dispute shows the pressed state from the vote response (also written into the store report so a redraw of stale data does not undo it). The sentence clears on the first render after a good fetch, not on a render of stale rows.
**Test changed on purpose:** `TestFeedErrorStateKeepsStaleRows` pinned the single line `setHidden(reportListEl, state.reports.length === 0);`. The branch now names the condition (`var hasRows = ...`, `setHidden(reportListEl, !hasRows);`, `clearRows()` when empty), so the pin moved to those lines. It gained pins for the background rule, the flag drop and the saved vote notice.
**Node proof** (app.js store, votes.js block and real feed.js render in a fake DOM; same script on `ef2376e` fails 7 checks):
```
transitions: loading(0) > ok(2) > error(2) > error(2) > ok(1)      (before: ... error(2) > loading(2) > error(2) > loading(2) ...)
PASS Retry from error with rows keeps status error and the rows (background refresh)
PASS an empty ok list polls without flashing the skeleton
PASS failed refetch after unticking Show disputed drops the disputed rows
PASS row says the vote was saved and the list could not refresh
PASS row shows the vote result from the vote response (Confirm pressed)
PASS a render during the failed state keeps the sentence
PASS the first render after a good fetch clears the sentence
PASS error with rows: list visible, error panel shown, rows kept
PASS error with no rows: list hidden, stale rows removed from the page
PASS focus restored to a shown control instead of the page body
ALL PASS
```
**Not changed:** the Retry panel still sits after the list in the template; the in-row sentence is what makes the saved vote visible.

### 5. Test audit T1 to T7

**Commit:** 35e9d6c `test(03): pin the trust fix producers and the clock arithmetic` (prefix is `test`, not `fix`, because it only adds pins and a comment fix; say if you want it reworded)
**Files:** `web/votes_contract_test.go`, `internal/testutil/history.go`
- T1: in the WR-07 commit (above).
- T2: `TestAgeAndFadeUseTheServerClock` now pins `return Date.now() + clockOffsetMs;`, the body of `noteServerClock` (`new Date(headerValue).getTime()`, `isFinite(serverMs)`, `clockOffsetMs = serverMs - Date.now();`), that `clockOffsetMs` is assigned only there, and the call `noteServerClock(res.headers && res.headers.get('Date'))` between the ok check and `res.json()`.
- T3: `TestOwnReportResolutionSkipsLocation` pins `block.isOwnReport = !!report.is_own_report;` in `applyOwnReportRule`, and that it is the only assignment.
- T4, T5: in the WR-01 and WR-03 commits.
- T6: `TestVoteTransportHasNoLocationFallback` keeps its original assertions and its docstring now names the two sanctioned exceptions to "downstream of a location promise". It pins the exact `skipLocation` expression (both conditions with `&&`), the three fix sources of `castVote` in order, one call of `ownReportLocation()` in `castVote`, and one hand built `Promise.resolve({` in `castVote`.
- T7: stale 6,000 rows comment in `internal/testutil/history.go` now says 20,000 rows at the read cap of 100.
**Node proofs** for the clock arithmetic (device clock 3 h behind: `relativeTime` gives "5 min ago", a 1 h remainder of 24 h is stale, a missing or unparsable header leaves the offset alone: ALL PASS) and the own report path (producer sets `isOwnReport`; own resolve with location denied posts with no GPS request and the 0,0 placeholder; Confirm with `ownReport: true` still rejects when location is denied: ALL PASS).

## Verification

Scratch database `p3_fix2` (created, used, dropped by this run); `DATABASE_URL=postgres:///p3_fix2?sslmode=disable`.

- `go test ./... -p 1 -count=1 -v`: exit 0, every package `ok` (api, api/handlers, auth, geocode, mailer, ratelimit, service, session, store, testutil, web), 1603 `PASS`/`RUN` lines, zero `SKIP`, zero `FAIL` anywhere in the log. The flaky `TestCastVoteConcurrentSameAccountKeepsEveryRow` did not fail this run.
- `go vet ./...`: no output.
- `gofmt -l` on every Go file touched: no output.
- `node --check` on app.js, feed.js, votes.js: ok.
- Dash check: no added line in `git diff ef2376e HEAD` contains U+2014 or U+2013.
- Node proof scripts live outside the repo (scratchpad); no image files, no server started, no port used.

## Notes

- `gofmt -l web/js_contract_test.go` lists that file; it was already unformatted at the base and is not touched here.
- `docs/TRUST-MODEL.md` was edited in the WR-03 and WR-07 commits (the too far hint and the GPS accuracy limit lines), and `TestTrustModelDocMatchesCode` passes.

---

_Fixed: 2026-10-08_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
