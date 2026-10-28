---
phase: 05-official-feed-demo-mode
plan: 05
subsystem: official-alerts
tags: [sachet, cap, refresher, migration, sqlc, placement]
status: complete

requires:
  - phase: 05-01
    provides: internal/areas (ByCode, StateForSender, TrailingState)
  - phase: 05-02
    provides: SachetClient (FetchList, FetchCAPs), ParseCAP, Version
provides:
  - official_alerts, official_alert_areas, official_feed_status, official_hazards (migration 00012)
  - official.sql queries and generated sqlc code
  - official.Place, Tier, Placement
  - official.Refresher with RefreshIfDue, WithSachet, WithVersionHook, Status
  - testutil.SeedOfficialAlert and SetOfficialFeedStatus
affects: [05-08, 05-11, 05-12, 05-21]

key-files:
  created:
    - internal/store/migrations/00012_add_official_alerts.sql
    - internal/store/queries/official.sql
    - internal/store/sqlc/official.sql.go
    - internal/store/official_test.go
    - internal/testutil/official.go
    - internal/official/place.go
    - internal/official/store.go
    - internal/official/refresher.go
    - internal/official/place_test.go
    - internal/official/refresher_test.go
    - internal/official/refresher_db_test.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/sqlc/models.go
    - internal/testutil/db.go

key-decisions:
  - "Migration number 00012 (00011 was the newest)."
  - "Area rows hold one code each: a district row has district_lgd only, a state row state_lgd only. A district tier alert has district rows plus state rows for their states; a state tier alert has state rows only. OfficialAlertsInForce gates the state match on placement_tier = 'state'."
  - "MarkFeedAttempt is an atomic claim (upsert with a cutoff, returns rows changed), so two instances cannot both start a run inside the gap; the in-process check and single flight sit in front of it."
  - "The RSS validators (ETag, Last-Modified) are stored only when a run finished all its work. A run with leftover items (over 40) or failed items keeps the old validators, so a 304 can never hide unfetched items."
  - "A message that parsed but is not kept (Exercise, Test) is remembered in memory by guid and pubDate so it is not refetched every run; lost on restart, harmless."
  - "Items with an invalid guid are left out of selection, not counted as failures, so they cannot use up the 40 CAP budget or fail a run."
  - "Truncate puts the two official_feed_status rows back after clearing, and MarkFeedAttempt creates a missing row, so a test or a fresh database never needs a seed."

requirements-completed: [ALERT-01, ALERT-02, ALERT-04]
duration: about 1h 15m
completed: 2026-10-29
---

# Phase 5 Plan 05: Official alert storage and the idempotent SACHET refresh Summary

One guarded function, `RefreshIfDue(ctx, feed)`, fetches SACHET, places each alert on districts or a state, stores it in its own tables, handles Update, Cancel and expiry, and keeps a feed status that only moves `last_success_at` on a real success.

## Migration number

00012 (`internal/store/migrations/00012_add_official_alerts.sql`). Not applied to the owner's dev database: that needs `make migrate` after the owner says yes.

## Commits

- ae7d5686 feat(05-05): add the official alert tables, queries and test helpers
- 86b87efb feat(05-05): place official alerts and refresh SACHET with one guarded function
- bc0154bf test(05-05): refresh a fake SACHET end to end on a real database

## What was built

**Task 1, tables and queries.** Four tables with named CHECK lists, no ON DELETE, no CONCURRENTLY, a Down in reverse order, both feed rows seeded. `official.sql` takes the time as an argument everywhere (no `now()`). `OfficialAlertsInForce` returns alerts not expired, cancelled or superseded that match any passed district code (district tier), any passed state code (state tier) or are unplaced, with each alert's own district and state code arrays. `Truncate` lists the four tables and re-inserts the two feed rows.

**Task 2, place and refresh.** `Place` resolves LGD codes through `ByCode` (unknown codes dropped), else a `<State>-SDMA` sender, else a trailing state name, else unplaced. `saveVersion` runs in the caller's transaction: reads the previous version of the guid (`FOR UPDATE`), upserts by guid, replaces areas, marks referenced alerts cancelled (Cancel) or superseded (Update, other guids only). `RefreshIfDue` has single flight per feed (a waiting caller shares the running call's result), a 4 minute gap per feed, the atomic claim, the 40 CAP bound, newest first, and the anomaly rule (zero items after a success that saw 10 or more is a failure with the word `anomaly`). A 304 is a success that keeps `items_seen`. Version hooks run inside each version's transaction with the transaction-bound queries.

**Task 3, database tests.** Seven end to end tests against an httptest fake SACHET serving the plan 05-02 fixtures.

## Verification

Run on the disposable database `p5_05` (created, used, dropped):

- Full suite `go test ./... -p 1 -count=1 -v`: 0 FAIL lines, 0 panics, every package ok. SKIP lines: exactly the three allowed (TestGeocodeLive, TestOfficialLive, TestScreeningFixture). TestCastVoteConcurrentSameAccountKeepsEveryRow passed first time.
- Named tests all PASS: TestMigration00012, TestOfficialQueries, TestMigrationsTruncateListsEveryTable, TestPlaceTiers, TestRefreshSkipsWithinGap, TestRefreshOneAtATime, TestRefreshAnomaly, TestRefreshNotModified, TestRefreshCapsCAPPerRun, TestRefreshStoresAndPlaces, TestRefreshUpdateAndCancel, TestRefreshIsIdempotent, TestRefreshFailuresKeepLastSuccess, TestRefreshLeavesReportsAlone, TestRefreshColdStartOverRuns, TestVersionHookInTransaction.
- `go vet ./...` clean; `go test ./internal/official/ -race` passes with the database.
- Unit tests pass under `-short` with an empty DATABASE_URL.
- No em or en dash in the new files; no `now()` in the queries; no ON DELETE or CONCURRENTLY in the migration; no `golang.org/x/sync`.

## Deviations from Plan

### Auto-fixed issues

**1. [Rule 1 - Bug] Validators stored on an incomplete run would hide unfetched items.**
- **Found during:** Task 2 design, pinned by TestRefreshColdStartOverRuns and TestRefreshRetriesWhatFailed.
- **Issue:** Storing the new ETag after a run that fetched only 40 of 100 items (or had failures) lets the next conditional request return 304, so the rest would never be fetched while the feed stays unchanged.
- **Fix:** The fetcher returns the stored validators unchanged unless every needed item was handled.
- **Files:** internal/official/refresher.go

**2. [Rule 2 - Missing critical] Exercise and Test messages would be refetched every run.**
- **Issue:** A parsed message that is not kept is never stored, so it would look new on every run and use CAP budget.
- **Fix:** Small in-memory memory of guid plus pubDate for messages not kept, pruned to the current feed window.

**3. [Rule 2 - Missing critical] The claim of a run is atomic in the database.**
- The plan reads the status then marks the attempt; MarkFeedAttempt is instead a conditional upsert returning rows changed, so a second instance cannot slip in between the read and the mark. Also creates a missing feed row.

### Interface notes for later plans

- `Result` has three extra exported fields beyond the plan's list: `ETag`, `LastModified`, `NotModified`. A Fetcher fills them for the validators it wants stored.
- Area rows are one code per row (see key-decisions). Plan 05-11's `OfficialAlertForPush` and 05-08's reads should take district codes from rows with `district_lgd` and state codes from rows with `state_lgd`. `OfficialAlertsInForceRow` already carries both arrays.
- `savedFromRow` rebuilds `prev` (a `SavedVersion`) from the stored row and areas, so `prev.Version.MsgType` can be `Cancel` and `prev.Version.Status`/`Scope` are the constants "Actual"/"Public".
- A Cancel under the same guid as the alert it cancels overwrites that row (cancelled_at set, Cancel text), as the plan says; a Cancel under another guid marks the referenced row and stores its own cancelled row.
- A save or hook failure is recorded with the class word `network` (the closed list has no database word); the version rolls back and is retried next run.

None other: plan executed as written.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model; mitigations T-05-23 to T-05-28 are covered by sqlc parameters and CHECK lists, the gap, single flight and 40 CAP bound, the last_success_at rule and anomaly, own tables with TestRefreshLeavesReportsAlone, the in-transaction hook with TestVersionHookInTransaction, and logs of feed name, counts and a class word only.

## User setup required

After the owner says yes, run `make migrate` against the owner's dev database (never run by an executor) so migration 00012 reaches it.

## Self-Check: PASSED

Files exist: migration, official.sql, official.sql.go, store/official_test.go, testutil/official.go, place.go, store.go, refresher.go, and the three test files. Commits ae7d5686, 86b87efb and bc0154bf exist on worktree-agent-a2ab53f16fe637810.
