---
phase: quick-261029-6wb
plan: 01
subsystem: official-feed
tags: [gdacs, concurrency, refresh, cron-door, go]
status: complete
quick_id: 261029-6wb
completed: 2026-10-29
requires: [05-21 GDACS hazards plan]
provides:
  - "GDACS request A and B run at the same time, 40 s per request, first failure wins"
  - "Service.RefreshAll runs every feed at the same time with a recover per feed"
  - "The cron door answers even when a refresh outlives the server write timeout"
key-files:
  modified:
    - internal/official/gdacs.go
    - internal/official/gdacs_test.go
    - internal/official/service.go
    - internal/official/service_test.go
    - internal/api/handlers/official_refresh.go
    - internal/api/handlers/official_refresh_test.go
    - .planning/phases/05-official-feed-demo-mode/05-21-PLAN.md
commits:
  - f0126069 test (RED, official package)
  - 4bc7448e fix (GREEN, official package)
  - f8b98697 test (RED, cron door)
  - 83010c8c fix (GREEN, cron door)
  - bbc94227 docs (05-21 dated line)
---

# Quick 261029-6wb: GDACS refresh succeeds against the slow live service

GDACS requests A and B now run at the same time with a 40 s limit each, the official feeds refresh at the same time inside the 55 s door budget, and the cron door answers even past the server's 10 s write timeout. One live POST on a disposable database stored 9 GDACS hazards with no error in 8.5 s.

## Probe (Task 1, nothing committed)

Request A as sent today, encoded (`%3B`) versus raw semicolons, from this process, User-Agent `Pinalert/1.0 (+http://localhost:8795)`, no contact address. Encoded went first, then raw, one at a time. GDACS requests sent by the probe: 2.

| variant | RawQuery sent | ms | status | bytes | raw features | kept by ParseGdacs | types | levels |
|---|---|---|---|---|---|---|---|---|
| encoded | `alertlevel=orange%3Bred&eventlist=TC%3BFL%3BEQ%3BVO%3BWF&fromdate=2026-10-14&pagenumber=1&pagesize=100&todate=2026-10-28` | 5458 | 200 | 5847 | 4 | 4 | EQ TC | orange red |
| raw | `alertlevel=orange;red&eventlist=TC;FL;EQ;VO;WF&fromdate=2026-10-14&pagenumber=1&pagesize=100&todate=2026-10-28` | 1431 | 200 | 5847 | 4 | 4 | EQ TC | orange red |

Decision: `decision=KEEP`. Both answers were 200, identical in size and content, and both CORRECT under the fixed rule. Clause (a) and (b) did not apply. Clause (c) did not apply either: encoded took 5458 ms, well under 40 s (the gap to raw looks like a cold start, since encoded went first). So url.Values with `%3B` stays, the existing subtest "the query is built with url.Values, not pasted in" is the encoding pin, and none of the RAW work (hand parser helper, TestGdacsQueryKeepsSemicolons, query helper) was done. No confirming pair was needed.

## What was done

- `internal/official/gdacs.go`: GdacsTimeout is 40 s (comment records the 2026-10-29 raise after the live 20.9 s request). FetchHazards builds the same two SearchRequest values and runs them in two goroutines under a derived cancel context. The first failure is recorded with sync.Once and cancels the sibling; both goroutines are always waited for with a WaitGroup; the merged hazards and Seen are exactly what the sequential code returned. Header and doc comments updated.
- `internal/official/service.go`: RefreshAll starts one goroutine per feed, each writing only `out[i]`, each with a recover (see P-2), each calling RefreshIfDue with the caller's whole context. Results come back in the fixed order. Comments on kickTimeout and sweep corrected. Kick and kickFeed unchanged (verified: kickFeed already starts one goroutine per feed with its own 55 s context; sweep inherits the concurrency through RefreshAll).
- `internal/api/handlers/official_refresh.go`: after the token check and the nil service check, the door calls `http.NewResponseController(w).SetWriteDeadline(now + deadline + 5 s)`. `http.ErrNotSupported` is ignored; any other error logs the class word network. Unauthorised callers and every other route keep the server default. main.go is unchanged.
- `.planning/phases/05-official-feed-demo-mode/05-21-PLAN.md`: the dated note inside the T-05-108 row, one line, table intact.

## Planning decisions P-1 to P-7

- P-1 First error wins: done with sync.Once, cancel on the first failure, the sibling's later "context canceled" discarded, both goroutines waited for. Pinned by the two fast failure subtests and the deadline subtest.
- P-2 A panic in a RefreshAll feed goroutine is recovered there, the slot becomes `Result{Feed, ErrorClass: "network"}`, the log line is `official: refresh feed=<feed> class=panic`. Pinned by "a panicking feed is reported and the other still runs".
- P-3 Kick and sweep: no code change, comments only.
- P-4 Results are written by index into a slice pre-sized to len(feeds); nothing appends from goroutines.
- P-5 The probe rule was fixed before the probe ran; the outcome was KEEP.
- P-6 The 05-21 note sits inside the T-05-108 row's Mitigation cell.
- P-7 The production WriteTimeout (10 s) against the door's 55 s: addressed by Task 2b inside the door handler only (not an open follow up). Whether the deadline survives the production middleware chain was checked with a throwaway test (deleted): a handler behind chi RequestID, Recoverer, Compress and Logger on a server with WriteTimeout 300 ms still delivered its 800 ms answer (200, 78 bytes, gzip because the test set Accept-Encoding itself). The repo has no custom ResponseWriter wrapper (no WriteHeader implementation outside tests), and chi v5.3.2's basicWriter and compressResponseWriter both implement Unwrap.

## RED results (which tests failed and why)

Official package RED run (commit f0126069), all failures for the intended reasons:

- TestGdacsClientBounds/the production client is fixed to the constant host: limits showed 15s, the pin wants 40s.
- TestGdacsRequestsRunTogether/A and B are in flight...: max in flight 1 and 704 ms (the sum), wanted 2 and about 400 ms.
- ...a fast failure of B ends the call without waiting for A: took 2.003 s and A's context never ended.
- ...a fast failure of A ends the call without waiting for B: failed too, which differs from the plan. The plan expected it to pass on sequential code; it failed because my version also asserts that B's request context ended, and sequential code never sends B at all. It passes after GREEN.
- The caller's deadline subtest passed in RED (sequential code is also bounded by the caller), as the plan expected. It guards the GREEN code.
- TestRefreshAllRunsFeedsTogether: gdacs never started while sachet ran; gdacs got timeout in the shared 1 s budget; a panic escaped RefreshAll.
- The amended documented query subtest passed before and after.

Cron door RED run (commit f8b98697): the authorised refresh longer than the server write timeout got EOF; the wrong token case passed.

## Verification

- `gofmt -l` clean for the touched official and handlers files (internal/api/handlers/place_test.go is listed by gofmt but is pre-existing and untouched), `go vet` clean.
- `go test -race -count=1 ./internal/official/`: ok. The new tests also passed `-race -count=3`.
- `go test -count=1 ./internal/api/...` with DATABASE_URL unset: ok.
- Full suite on p5_gdacsfix (`go test ./... -p 1 -count=1 -v`): 20 packages ok, `^--- FAIL` 0, `^FAIL` 0, `^panic` 0, 5258 `--- PASS` lines (subtests included). Skips: TestGeocodeLive, TestOfficialLive, TestScreeningFixture only. TestPhase5OfficialJourney passed (0.36 s), which proves the door, the real NewService and the transport swap work with concurrent feeds. No flake appeared, so no rerun and no database recreate.
- `node --test web/*.mjs`: tests 1048, pass 1048, fail 0.
- go.mod and go.sum unchanged.

## Live proof (one POST, nothing rerun)

Server: 127.0.0.1:8795 (`http.Server` with no WriteTimeout, the proof server only), database p5_gdacsfix, real router, real network, log mailer, throwaway session secret, token from 32 crypto/rand bytes held in memory (only its length, 64, was printed). Both official_feed_status rows had no attempt before the POST.

User-Agent sent by both clients: `Pinalert/1.0 (+http://localhost:8795)` (ContactEmail empty; no environment variable, git config or file was read for an address).

Door answer: HTTP 200, 8.551 s.

```
door-json={"feeds":[{"feed":"sachet","skipped":false,"items":100,"stored":40,"error":""},{"feed":"gdacs","skipped":false,"items":104,"stored":9,"error":""}]}
```

official_feed_status rows after the POST:

```
sachet last_attempt_at=2026-10-29 05:20:47.646856 +0530 IST last_success_at=2026-10-29 05:20:47.646856 +0530 IST last_error="" consecutive_failures=0 items_seen=100
gdacs  last_attempt_at=2026-10-29 05:20:47.646848 +0530 IST last_success_at=2026-10-29 05:20:47.646848 +0530 IST last_error="" consecutive_failures=0 items_seen=104
```

official_hazards: 9 rows.

```
TC 1001345 "Tropical Cyclone ARNAB-26" Red   is_current=true
TC 1001338 "Tropical Cyclone CHAMPI-26" Red  is_current=true
EQ 1574196 "Earthquake in China" Green       is_current=true
EQ 1574148 "Earthquake in Afghanistan" Green is_current=true
EQ 1573845 "Earthquake in China" Green       is_current=true
EQ 1573818 "Earthquake in Afghanistan" Green is_current=true
EQ 1573814 "Earthquake in China" Green       is_current=true
EQ 1573773 "Earthquake in China" Green       is_current=true
EQ 1573775 "Earthquake in China" Green       is_current=true
```

Live requests made, honestly counted: the probe sent 2 GDACS requests. The one POST made 2 GDACS requests and one SACHET run, which was 1 RSS list request plus 40 CAP message requests (fetched=40, the CAPPerRun cap), so 41 SACHET requests. The plan allowed "one SACHET run"; the 41 count is that run's real size on a cold database. Total live requests: 4 GDACS, 41 SACHET.

Teardown: zz_liveproof_6wb_test.go and the probe file deleted, nothing listens on 8795 (lsof exit 1), p5_gdacsfix dropped and absent from `psql -lqt`, .scratch-6wb removed.

## Deviations from Plan

**1. [Rule 2 - Missing critical functionality] searchSafely recover around each Search goroutine**
- **Found during:** Task 2 GREEN
- **Issue:** RefreshAll's per-feed recover cannot see a panic raised inside the two goroutines FetchHazards now starts; such a panic would crash the process.
- **Fix:** `GdacsClient.searchSafely` wraps Search with a recover and reports a network class fetchError.
- **Files modified:** internal/official/gdacs.go
- **Commit:** 4bc7448e

**2. Commit subject form.** The five commits use the plan's `type(quick-261029-6wb):` scope, while the project rules give `type(261029-6wb):`. Not amended (no history rewrite). Every commit does end with the required trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

**3. Task 2b done as the orchestrator added it.** Two commits (RED then GREEN) as required. The throwaway chi middleware check described under P-7 was extra and left no file behind.

Auth gates: none. Known stubs: none. Threat flags: none beyond the plan's register (T-6wb-01 to T-6wb-07 were handled as written; the door's longer write deadline is granted only after the token passes).

## For the orchestrator

docs/OFFICIAL-ALERTS.md (owned by plan 05-22) now needs these facts:

- GDACS limit is 40 s per request (was 15 s), `GdacsTimeout`.
- Request A and request B run at the same time (not one after the other), so a GDACS refresh takes as long as the slower request; if either fails the call returns at that failure with the first error, the other request is cancelled, and nothing is stored.
- The official feeds (SACHET and GDACS) are refreshed at the same time by RefreshAll (cron door, ticker sweep), each with the whole 55 s budget; results come back in the fixed order sachet then gdacs; a panic in one feed is reported as the word network and logged `class=panic` without stopping the other feed. The lazy kick already ran one goroutine per feed.
- The cron door POST /internal/official/refresh now sets its own write deadline (refresh deadline plus 5 s) after the token check, so a refresh that outlives the production server's 10 s WriteTimeout is still answered. main.go still sets WriteTimeout 10 s for every other route.
- GDACS query encoding stays url.Values with `%3B` for the semicolons; the 2026-10-29 probe showed raw and encoded give identical answers.
- One live run (cold database, 2026-10-29): door 200 in 8.5 s, gdacs items 104, stored 9, error empty.

## Self-Check: PASSED

Files verified present: gdacs.go, gdacs_test.go, service.go, service_test.go, official_refresh.go, official_refresh_test.go, 05-21-PLAN.md edit. Commits verified in `git log`: f0126069, 4bc7448e, f8b98697, 83010c8c, bbc94227.
