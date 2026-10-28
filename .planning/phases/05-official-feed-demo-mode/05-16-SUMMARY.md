---
phase: 05-official-feed-demo-mode
plan: 16
subsystem: official-alerts
tags: [push, worker, silent, series, apple-rule, service-worker, d-11, d-16]
status: complete

requires:
  - phase: 05-11
    provides: official_push_outbox, the lease conditional queue queries, BuildOfficialPushText, OfficialTopic, OfficialTag, OfficialIssuer, OfficialStateWords, the official caps
  - phase: 05-12
    provides: officialSvc started in main.go
  - phase: 05-05
    provides: official_alerts rows with references_text, Updates stored under a new guid
  - phase: 03.3
    provides: the guarded push.Sender, the endpoint policy, judge, PushBackoff, deviceSkipsRepeatPush, anyAppleDevice, the lease discipline
provides:
  - service.OfficialPushWorker (SweepOnce, Run) and its options, OfficialPushWorkerQuerier, OfficialPushFreshWindow
  - queries OfficialAlertSeries and OfficialPushSentForSeries and their sqlc output
  - fanOutSend, deviceQuerier, handleDeviceAnswers, countDeviceFailure extracted from pushworker.go with no change of behaviour
  - sw.js safeTarget accepting the exact string /alerts
  - the official worker started with the 3.3 worker when VAPID is set
affects: [05-14, 05-20]

key-files:
  created:
    - internal/service/pushworker_official.go
    - internal/service/pushworker_official_test.go
    - internal/service/pushworker_official_db_test.go
    - internal/official/pushwindow_test.go
    - internal/official/pushsend_db_test.go
  modified:
    - internal/service/pushworker.go
    - internal/store/queries/official_push.sql
    - internal/store/sqlc/official_push.sql.go
    - internal/store/official_push_test.go
    - web/static/js/sw.js
    - web/pwa_sw_test.mjs
    - cmd/server/main.go

key-decisions:
  - "Skip word mapping: a missing alert, a missing account, a level or tier that never qualifies and an issuer outside the closed table are not_eligible; a cancelled, superseded, expired, within-10-minutes-of-its-end alert and a row past its own expires_at are stale (the same word SkipStaleOfficialPushes uses); the feed check is feed_not_fresh."
  - "Area match follows plan 05-11 by placement tier: district tier matches by district only (the state rows it carries do not widen it), state tier matches by state and only for Extreme, unplaced never matches."
  - "The series root is the member that names no other member; several take the earliest sent_at then the lowest id; a cycle (none) takes the earliest sent_at then the lowest id over all; an empty series answer is the alert alone."
  - "The official worker is a push runner in main.go (pushRunners, on runCtx) next to the 3.3 worker, not a bare go ... Run(context.Background()), so shutdown waits for its current row as 3.3's review S-05 requires."

requirements-completed: []

duration: about 2h
completed: 2026-10-29
---

# Phase 5 Plan 16: The official push worker lane Summary

Queued official alerts now reach a person's phone silently: a new `OfficialPushWorker` re-checks the alert, the feed, the switch, the home area and the 6 hour cap at send time, works per alert series (Topic and Tag from the series root's guid, one notification per series on Apple endpoints), sends through 3.3's guarded sender and shared answer rules, and a tap opens exactly `/alerts`.

## Accomplishments

- **Extraction in pushworker.go (no behaviour change).** `fanOut` now calls `fanOutSend(ctx, sender, devices, msg)`. The per device part of `settle` moved to `handleDeviceAnswers(ctx, q deviceQuerier, now, results, logf)` with the `deviceQuerier` interface (MarkPushSubscriptionOK, DeletePushSubscriptionByID, BumpPushSubscriptionFailure) and `countDeviceFailure`. `PushWorker.countFailure` stays as a thin wrapper and `rowLogf` adds the old `push worker: row %d ` prefix, so every log line and error string is byte-identical. `sendRow` step 7 (the confirmed update Apple skip, `anyAppleDevice`, `AccountPushSentAboutReport`, `deviceSkipsRepeatPush`) was not touched. Proof: the 184 result lines of `TestWorker|TestPushBackoff|TestSendErrorClass|TestDeviceSkips|TestHeldBy|TestReportWas` are identical before and after (`diff` of the sorted `--- PASS/SKIP` lines is empty), and `TestWorkerOnRealOutbox` and the other database 3.3 tests pass in the full suite.
- **The official lane (pushworker_official.go).** `sendOfficialRow` order: renew the lease (lost: log `lease_lost`, stop); `OfficialAlertForPush`; the level, tier and issuer checks; `OfficialAlertSeries`; currency (alert `last_seen_at` within `OfficialPushFreshWindow` first, else `GetFeedStatus("sachet")` success within it); `GetOfficialPushSetting`; `GetAccountHomeCell`; the area match; `CountAccountOfficialPushesInWindow` over `OfficialPushCapWindow` against `OfficialPushPerAccountPer6Hours`; `PushSubscriptionsForAccount` through the policy (host from `policy.Check`, never a stored column); the Apple rule through `anyAppleDevice`, `OfficialPushSentForSeries` and `deviceSkipsRepeatPush` (no inline `IsAppleHost`; none left gives `already_notified`); `BuildOfficialPushText`; `fanOutSend`; `handleDeviceAnswers`; mark sent, reschedule by `PushBackoff` before the earlier of the row's and the alert's end, or failed. Payload: V 1, kind `official`, Tag `OfficialTag(root guid)`, URL `/alerts`; urgency high for Extreme, normal for Severe; TTL the smaller of `OfficialPushTTL` and the alert's remaining life; Topic `OfficialTopic(root guid)`.
- **Series queries.** `OfficialAlertSeries` (WITH RECURSIVE over `references_text`, UNION, depth limit 20, collapsed to one row per alert, returns id, guid, sent_at and `names_member`, oldest first) and `OfficialPushSentForSeries` (EXISTS a sent row for the account and any listed alert id). `OfficialPushSentForAlert` is untouched. sqlc 1.31.1 regenerated only `official_push.sql.go`.
- **Service worker.** `safeTarget` accepts the exact string `/alerts` besides `/?report=` plus 1 to 18 digits; `/alerts/`, `/alerts?x=1`, `/alerts#a`, `//alerts`, `https://evil.example/alerts`, `/Alerts` and the rest still become `/`. The eight option keys, `silent: true` and the absence of vibrate and sound are unchanged. The push handler had no allow list on kind or tag, so `official` and the `official-...` tag need no other change.
- **Wiring.** `NewOfficialPushWorker(queries, pushSender, areas.Default(), WithOfficialPushWorkerPolicy(pushPolicy))` is built in the same `if pushOn {` block as `NewPushWorker`, from the same sender and policy values.

## Task Commits

| Task | Phase | Commit | Message |
|------|-------|--------|---------|
| 1 | RED | 07aa0583 | test(05-16): add failing tests for the official worker lane and the alert series queries |
| 1 | GREEN | fd7c9ba0 | feat(05-16): add the official push worker lane on 3.3's sender and answer rules |
| 2 | RED | 7fc82a0d | test(05-16): add failing node cases for the /alerts tap target |
| 2 | GREEN | ea3e3e81 | feat(05-16): let a tap on an official alert open /alerts |
| 3 | | 1cd0a614 | feat(05-16): start the official push worker with VAPID and prove it on a real outbox |

## Tests (all pass)

Unit with fakes (internal/service/pushworker_official_test.go): TestOfficialWorkerSendsSilentToAlerts, TestOfficialWorkerColdStartRace, TestOfficialWorkerRechecksAtSendTime (15 cases: alert gone, cancelled, superseded, expired, ending in 9 minutes, row life over, alert and feed both 31 minutes old, no feed row, switch off, account gone, no home, empty home, home outside the districts, issuer outside the table, severity below Severe, no device; plus the fresh feed covering an old alert), TestOfficialWorkerAreaMatch (added: district not widened to state, state Extreme, state Severe refused, unplaced refused), TestOfficialWorkerCapSixHours, TestOfficialWorkerUrgencyAndTTL, TestOfficialWorkerAnswers (201, 404, 410, 429, 500, 403, 403 fifth on a never successful device, 403 fifth on a device that worked, attempts limit, backoff past the row's end), TestOfficialWorkerStopsOnLostLease, TestOfficialWorkerNeverLogsSecrets, TestOfficialWorkerEscalationSameTopic (amended 2026-10-28 rules), TestOfficialWorkerSeriesAcrossGuids (amended 2026-10-29: A to B to C resolves to root g1, Apple left out, Apple only skipped as already_notified, cycle and several root fallbacks), TestOfficialWorkerConstants. Mutation check: returning the alert's own guid instead of the root failed four series subtests.

Database: `TestOfficialSeriesQueries` (internal/store, amended 2026-10-29: chain, two references, cycle, self or unknown reference, sent for series true or false per account), `TestOfficialWorkerOnRealOutbox` (internal/service, package service_test: real hook in a transaction, real queries and leases, silent send once, second sweep nothing, reports votes and sessions counts unchanged, the real `sachet` status row decides currency, the Apple series rule on real SQL, switch off, 3 sent in the window, soft deleted account), `TestOfficialPushSendsAfterRealRefresh` (internal/official: a real refresh of an Extreme Chennai cyclone message from the fake SACHET upstream through the refresher and the real hook to the worker and a fake push service; asserts a queued row exists before the sweep so it cannot pass vacuously), `TestOfficialPushFreshWindowMatches`.

Node: three cases added to web/pwa_sw_test.mjs, each with "Amended 2026-10-29 by plan 05-16 (D-11)"; no earlier case changed. `node --test web/*_test.mjs`: 993 tests, 993 pass, 0 fail (the WR-07 flake did not fire).

## Deviations from Plan

### Auto-fixed and added

**1. [Rule 2 - Missing critical functionality] The official worker is a push runner, not a bare goroutine**
- **Found during:** Task 3
- **Issue:** The plan says `go ... Run(context.Background())`. In main.go as built, 3.3's worker is not started that way: it is in `pushRunners`, run on `runCtx` and waited for at shutdown (3.3 review S-05), so an accepted send reaches its mark. A bare goroutine on `context.Background()` would be killed mid row on SIGTERM and resend after a redeploy.
- **Fix:** `officialPushWorker.Run` is appended to `pushRunners` in the same `if pushOn {` block. The acceptance awk ("same block") prints `same block`.
- **Commit:** 1cd0a614

**2. [Rule 3 - Blocking] The real refresh test cannot live in package service**
- **Issue:** `internal/official` imports `internal/service`, so a service test cannot reach the refresher's fake SACHET upstream (`newSachetClientWithBase` and the fixtures are unexported in the official package).
- **Fix:** `TestOfficialWorkerOnRealOutbox` is in `internal/service/pushworker_official_db_test.go` (package `service_test`) and runs the real hook and real queries on seeded alerts. The literal "real refresh" chain is `TestOfficialPushSendsAfterRealRefresh` in a new file `internal/official/pushsend_db_test.go`, outside the plan's file list (a new test file only; no existing file in that package changed except the new `pushwindow_test.go` the plan names).
- **Commit:** 1cd0a614

### Small choices beyond the plan text

- `handleDeviceAnswers` returns untagged database errors and takes a `logf` closure; each caller tags errors with its own row id and adds its own log prefix (3.3 keeps `push worker: row N`, the official lane uses `official push worker: row N`). The plan's signature had a bare `log func(...)`.
- The area check also covers an unplaced alert and a state level Severe alert at send time (`not_eligible`), as a second guard to plan 05-11's queue.
- The feed name `"sachet"` is a private constant in the service package; `TestOfficialPushSendsAfterRealRefresh` pins `string(FeedSachet) == "sachet"`.
- No guid, topic, tag, district name or alert text is logged; `TestOfficialWorkerNeverLogsSecrets` checks all of them.

**Total deviations:** 2 auto-fixed, none architectural. No Rule 4 stop.

## Seams re-grepped (rule 1)

`func judge(r pushDeviceResult)`, `func (w *PushWorker) fanOut`, `NewPushWorker(` and `func BuildOfficialPushText` each print 1. `safeTarget` in sw.js is `function safeTarget` (JavaScript; the plan's `func` grep is a known typo). 3.3's gap fix helpers `deviceSkipsRepeatPush` and `anyAppleDevice` exist at pushworker.go lines 580 and 585 and are used by the official lane unchanged; `AccountPushSentAboutReport`, `heldByHourlyCap`, `reportWasConfirmed` and `VoteLogForReport` were not touched.

## Verification

- Full suite on the plan's own database `p5_16` (freshly created), `go test ./... -p 1 -count=1 -v`: **5163 `--- PASS`, 0 `--- FAIL`, 0 `^FAIL`, 0 panics, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed)**. Every named test above has a PASS line, as do `TestWorkerResponses` and `TestWorkerOnRealOutbox`.
- Known flake, reported: `TestVotedReportsByAccountUsesTheAccountIndex` failed in the first full run and in a rerun by name on that same database (planner statistics); I dropped and recreated `p5_16`, and it passed in the full run on the fresh database. It does not touch this plan's code.
- `go build ./...` and `go vet ./...` clean. `gofmt -l internal cmd` lists only `internal/api/handlers/place_test.go` and `internal/geocode/photon.go`, neither mine.
- Acceptance greps: `var _ OfficialPushWorkerQuerier = (*sqlcgen.Queries)(nil)` 1; `handleDeviceAnswers(` and `fanOutSend(` present in the official file; no feed text field (HeadlineEN, headline_en, Description) outside comments (0); `OfficialPushFreshWindow = 30 * time.Minute` 1; `deviceSkipsRepeatPush(` and `anyAppleDevice(` present; `IsAppleHost` outside comments 0; `OfficialAlertSeries(` 2, `OfficialPushSentForSeries(` 2; `name: OfficialAlertSeries :many` 1; `WITH RECURSIVE` 1; sw.js `'/alerts'` outside comments 1, `silent: true` 1, `vibrate` outside comments 0; `Amended 2026-10-.. by plan 05-16` in the node test 4; `NewOfficialPushWorker(` in main.go 1 and the awk prints `same block`. The dash check (perl, both dash characters) is clean on every line I added.
- Database `p5_16` was created, used, dropped, and `psql -lqt` no longer lists it. No server was started, no real push service or SACHET host was contacted, no real VAPID key exists in any file.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-05-84 (open redirect: exactly one new value, near misses fall back to `/`, node cases), T-05-85 (stale alarm and lost alarm: re-check of cancel, supersede, expiry and the 10 minute margin, currency by alert row or fresh feed, cold start case tested), T-05-86 (switch, home cell and area read again, soft deleted accounts reach `no_device`), T-05-87 (6 hour cap of 3, lease, backoff), T-05-88 (closed words only, no secrets in logs), T-05-89 (3.3's guarded client and endpoint policy reused) are mitigated.

## Notes for plan 05-14 and 05-20

- docs/PUSH.md (plan 05-20) should say: official notifications are silent on every platform, an iPhone's own settings may still decide, an Apple endpoint gets only the first official push about an alert series, and a tap opens /alerts.
- The owner check named in the plan (one silent official notification on the owner's phone, a tap opens Official alerts) waits for hosting and the installed app (3.3 plan 15); it is batched into plan 05-20.
- Per alert series and account, the Apple rule only ever means fewer notes (an Apple device whose first push failed, or added later, gets no later push about the series); there is no per device send record and no migration, by design as in 3.3.

## Self-Check: PASSED

- All five created files and seven modified files exist in the worktree.
- All five task commits (07aa0583, fd7c9ba0, 7fc82a0d, ea3e3e81, 1cd0a614) are on branch worktree-agent-abec77973c0c9609f.
