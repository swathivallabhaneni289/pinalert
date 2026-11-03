---
phase: quick-261103-jal
fixed_at: 2026-11-03T00:00:00Z
review_path: .planning/quick/261103-jal-notifications-cut-to-two-rows-alerts-on-/261103-jal-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Quick 261103-jal: Code Review Fix Report

**Fixed at:** 2026-11-03
**Source review:** `.planning/quick/261103-jal-notifications-cut-to-two-rows-alerts-on-/261103-jal-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 8 (WR-01 to WR-04, IN-01, IN-02, IN-03, IN-07), plus IN-04 settled by WR-04
- Fixed: 8
- Skipped: 0
- Follow up fixes after the first report: F1 and F2 (below)
- Left as they are on instruction: IN-05, IN-06, and the Settings line "Choose what Pinalert tells you about."

Gate after the first eight fixes, on the scratch database q_notif2_fix (dropped afterwards; the gate after F1 and F2 is under "Follow up fixes"): `go build ./...` and `go vet ./...` clean; `go test ./... -p 1 -count=1 -v` gave 6811 `--- PASS` lines (subtests included), 0 `--- FAIL`, every package `ok`, and exactly three skips, TestGeocodeLive, TestOfficialLive and TestScreeningFixture; `node --test web/*.mjs` gave 1596 pass, 0 fail.

## Fixed Issues

### WR-01: Reliable bell rows ate the shared hourly ceiling

**Files modified:** `internal/store/queries/updates.sql`, `internal/store/sqlc/updates.sql.go`, `internal/service/updates.go`, `internal/service/updatesalerts.go`, `internal/service/updatesalerts_test.go`, `internal/service/updatesceiling_db_test.go` (new), `internal/store/updates_test.go`, `docs/TRUST-MODEL.md`
**Commits:** 82a6fd34, 3408577b (gofmt of the new test file)
**Applied fix:** `reliable_nearby` is left out of the hourly count in `AdvanceCursorAndInsertUpdate`, `MarkFadingAndInsertUpdate` and `InsertFollowedPostedAlert`. `InsertReliableAlert` now counts only `reliable_nearby` rows, against a new constant `ReliableHourlyCeiling = 10` (same value as `UpdatesHourlyCeiling`, next to the other caps in `updates.go`). `make sqlc` regenerated only `updates.sql.go`. No migration.
**Test:** `TestReliableRowsDoNotEatTheSharedCeiling` puts ten Reliable rows in the hour, then a followed person posts and a report the reader voted on goes Live. The follow row is written, the change row is a normal `reached_live` row, nothing rolls up, and the Reliable row stops at its own cap and writes again when a place frees. Checked against the old SQL: it failed (follow row 0, `reached_live` 0, one `more` row). The two store tests that filled the old ceiling with `hidden` rows now fill the Reliable cap with Reliable rows.

### WR-02: A follow phone note could be lost to a Reliable row

**Files modified:** `internal/service/updatesfollow.go`, `internal/service/updatesservice.go`, `internal/service/updatesalerts.go`, `internal/service/updatesalerts_mute_test.go`, `internal/service/updatesfollow_test.go`, `internal/service/updatesfollow_slot_db_test.go` (new), `docs/TRUST-MODEL.md`
**Commit:** 62c098c2
**Applied fix:** Both halves. `refreshFollowedPosts` now returns a `followStep` (the people the reader follows with Notify me on, with the time each bell went on, and whether the read hit `FollowedPostsCap`). `Refresh` skips the Reliable step for that refresh when the follow step errored or was truncated. `refreshReliableAlerts` skips any report by a Notify me followee posted after that bell went on. This replaces the old W3 rule (offer the Reliable step a report whose follow note was not written); the follow note is tried again on a later poll.
**Tests:** `TestReliableRowNeverTakesTheFollowSlot` (real database; failed before: the Reliable row took the slot while the reader was over the shared ceiling, then the follow row could never be written), `TestFollowBellFollowWinsEvenWhenTheCeilingStopsIt`, `TestFollowBellErrorSkipsReliableStep`, `TestFollowBellTruncatedSkipsReliableStep` (at the cap and one under it). The two older W3 tests were amended with a dated line.

### WR-03: API reference contradicted the handler

**Files modified:** `internal/api/handlers/me.go`, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`
**Commit:** 4bd9a770
**Applied fix:** The description now says a body with only old keys answers 200 with the stored settings, and that a body with neither `alert_radius_km` nor one of those keys set, such as `{}` or only nulls, is refused with the field `body` and "Nothing to save." Regenerated with `make swag`; the diff in `docs/` is only those lines, and the router's `@license.name Unlicensed (all rights reserved)` line is untouched and present in all three generated files. This also settles IN-05's wording request.

### WR-04: Early bell refreshes on the dispatcher goroutine

**Branch taken: REMOVAL.** The bell poll builds both kinds on its own read. `GET /api/updates` (`internal/api/handlers/updates.go:259`) calls `UpdatesService.Refresh`, and `Refresh` runs `refreshFollowed` (the change and comment rows) and `refreshReliableAlerts`. The router's instance (`internal/api/router.go:747`) is built with a non nil `NearbyReader` whenever the report service exists, and it works with push off, when no dispatcher exists at all. So the dispatcher's refreshes only made rows appear sooner and fed no phone note.
**Files modified:** `internal/service/pushdispatch.go`, `internal/service/pushkinds.go`, `cmd/server/main.go`, `internal/store/queries/push.sql`, `internal/store/sqlc/push.sql.go`, `docs/PUSH.md`, `docs/TRUST-MODEL.md`, and tests: `pushdispatch_test.go`, `pushdispatch_db_test.go`, `pushkinds_test.go`, `pushnotif2_db_test.go`, `pushworker_fix_test.go`, `pushworker_hidden_db_test.go`, `pushcomments_test.go`, `followpush_test.go`, `ratings_wiring_test.go`, `internal/api/handlers/push_flow_e2e_test.go`, `rated_reporter_e2e_test.go`
**Commit:** b213c508
**Applied fix:**
- `reliableBranch` and `reporterIsReliable` are deleted, with their call in `EvaluateReport`. Nothing outside the branch read `reliable_state` or the Reliable fan out count, and the sweep rotates on `evaluated_at`, so no state is needed to stop reselection. `reliable_state` is no longer written; the column and the `MarkReliableFanout` query stay (D8, C6), with a comment saying so.
- `ChangesSweepOnce`, its ticker, its boot call, `changesMu` and `changesAfter` are deleted, with `PushChangesSweepInterval` and `PushChangesSweepAccountCap`. The `AccountsForBellRefresh` query and its store test stay (C6), with a comment that it has no caller.
- `changesBranch` is kept (the task named only the Reliable branch and the sweep): it still runs per poke with an actor, bounded by that one report's own posters, voters and commenters. `changesFor` is gone and `changesBranch` calls `Refresh` directly, which is the IN-07 part about `changesFor` and its dead `errs`.
- `followBranch` is untouched: its refresh loop feeds `EnqueueFollowFromUpdates`, a phone path.
- Orphans removed because nothing else used them and checking found no pin: `PushReliableReportsPerReporterPerHour`, `WithDispatchSignals` and the dispatcher's `signals` field (so `main.go`, two tests and the `ratings_wiring_test.go` list and TRUST-MODEL line about the dispatcher were amended with dated lines), and `ReporterHistoryReports`, `MarkReliableFanout` and `AccountsForBellRefresh` from the `PushDispatchQuerier` interface.
- IN-04 is settled: no history read, no candidate read and no unmarked re-read of a serious report happens per sweep any more.
**Tests:** `TestDispatchMakesNoReliableBellRefresh` (counting refresher fake: a new report, a poke from a vote and a serious report by a Reliable reporter make 0 Refresh calls, no history read, no Reliable count; it replaces `TestDispatchReliableUsesRefreshRows`). `TestPushReliable` now expects no `reliable_nearby` row until each reader's own bell poll, then exactly one, and no `reliable_state`; that proves the poll is the producer. `TestPushCaps` expects no Reliable state. Every `ChangesSweepOnce` call in the DB and e2e tests became a direct `Refresh`. `TestPhoneAlertReliableAtPosting` polls `GET /api/updates` before counting rows. `TestDispatchOneFanoutPerReport` and the two sweep rotation subtests were removed with the code they pinned.
**This replaces the plan's D3 "keep the bell refresh" and C2/C3 for the Reliable branch and the sweep.** The SUMMARY notes that say the refreshes stay are therefore out of date for those two paths; the plan and SUMMARY were not edited.

### IN-01: installed-ask.js comment

**Files modified:** `web/static/js/installed-ask.js`
**Commit:** 8f256721
**Applied fix:** The comment now says the star box makes no such check, and that the ask is modal so no resolve, and no star box, can happen while it is open.

### IN-02: dead rateGeneration

**Files modified:** `web/static/js/votes.js`
**Commit:** 37a7c907
**Applied fix:** Removed the counter, its comment and its increment in `takeRateBoxBack`. No test pinned it by name; the SK-I04 case in `votes_stars_test.mjs` pins the behaviour (a card Undo removes the box, the next resolve gets one) and still passes.

### IN-03: vacuous Reliable mute and block tests, and the TRUST-MODEL sentence

**Files modified:** `internal/service/pushworker_fix_test.go`, `docs/TRUST-MODEL.md`
**Commit:** a4e7c5f9
**Applied fix:** `TestQueuedReliablePushStopsAfterMuteOrBlock` and `TestConfirmedUpdateOfReliablePushStopsAfterMuteOrBlock` now store the mute and the two blocks before the readers' bell polls. They assert the muter holds no Reliable bell row (and so no confirmed update), the control reader does, and the worker's `personNotMutedOrBlocked`, asked directly, says no for the muter and for each side of a block and yes for the control reader. Flip check: deleting the mute seed fails both tests (bell row 1, check true); deleting the block seed fails both (check true). Names kept. TRUST-MODEL line 786 now reads like PUSH.md line 94: a Reliable note queued before 2026-11-03 is skipped whatever a mute or block says, the check stays behind that as a second layer, and the mute still acts earlier because a muted reporter raises no Reliable bell row for its muter. Cited test names were kept and the doc tests pass.
**Real gap found, closed by follow up F1 below:** a block did not stop the Reliable bell row. The bell step reads `person_mutes` only, and TRUST-MODEL says a block "does not hide their reports anywhere". The block half of the old tests only ever covered the phone path, which is now skipped for every Reliable row. So the block is asserted through the worker check, not the bell. If the owner wants a block to also keep a Reliable row away, that is a behaviour change for the bell step and is not made here.

### IN-07: small leftovers

**Files modified:** `internal/service/follow.go` (and `internal/service/pushdispatch.go` under WR-04)
**Commit:** 1868fa47 (follow.go), b213c508 (`changesFor`)
**Applied fix:** `changesFor` always returned nil, so it and the dead `errs`/`errors.Join` in `changesBranch` went with WR-04. The long comment line in `follow.go` is wrapped. The first bullet of IN-07 (the Settings line) was left unchanged on instruction.

## Follow up fixes

Requested by the coordinator after the first report.

### F1: a block keeps a Reliable bell row away

**Files modified:** `internal/store/queries/person_mutes.sql`, `internal/store/sqlc/person_mutes.sql.go`, `internal/service/updatesservice.go`, `internal/service/updatesalerts.go`, `internal/service/updatesservice_test.go`, `internal/service/pushworker_fix_test.go`, `docs/TRUST-MODEL.md`
**Commit:** ae763ac8
**Applied fix:** A new query `BlockedEitherWayIDs` (the people with a block between them and the reader, either direction) is read once per refresh next to `MutedPeopleIDs`, and `refreshReliableAlerts` skips their reports, the same rule as the worker's `personNotMutedOrBlocked`. Reports still show on Nearby and everywhere else. No migration. `docs/TRUST-MODEL.md` has one dated sentence in the Block paragraph, and the IN-03 sentence in the send time paragraph now names blocks as well as the mute. No test pins either sentence.
**Tests:** the two IN-03 tests now prove the block through the bell row itself (`wantBellRows`: muter, blocker and blocked hold 0 Reliable bell rows, the control reader 1, and the three get no confirmed update either), beside the direct worker check. Flip check: deleting the reader-blocks-reporter seed fails both tests (bell row 1, outbox holds a confirmed update, check true), and so does deleting the reporter-blocks-reader seed.

### F2: change rows never hold a follow row

**Files modified:** `internal/store/queries/updates.sql`, `internal/store/sqlc/updates.sql.go`, `internal/service/updates.go`, `internal/service/updatesfollow.go`, `internal/service/updatesfollow_test.go`, `internal/service/updatesfollow_slot_db_test.go`, `internal/service/followpush_ceiling_db_test.go` (new), `internal/store/updates_test.go`, `docs/TRUST-MODEL.md`
**Commit:** 52f2436f
**Applied fix:** `InsertFollowedPostedAlert` now counts only `followed_posted` rows against its cap, a new named constant `FollowHourlyCeiling = 10` (same value as the shared ceiling). Change rows still count follow rows toward `UpdatesHourlyCeiling`, as before. This closes the leftover noted in the first report. `make sqlc` touched only `updates.sql.go`. No migration.
**Tests:** `TestChangeRowsNeverHoldAFollowRow` (an account with a full hour of change rows still gets the follow row, and its follow phone note is queued; the follow cap itself still holds after `FollowHourlyCeiling` follow rows). Checked against the old SQL: it failed (follow rows 0). Amended with dated lines: `TestReliableRowNeverTakesTheFollowSlot` (its fixture now fills the follow cap with follow rows), `TestFollowBellBounded` and the store test `TestFirstAlertIndexSharedByBothKinds` (a full hour of change rows leaves the follow note open, a full hour of follow rows holds it).

**Gate after F1 and F2**, on the scratch database q_notif2_fix2 (dropped afterwards): `go build ./...` and `go vet ./...` clean; `go test ./... -p 1 -count=1 -v` gave 6812 `--- PASS`, 0 `--- FAIL`, every package `ok`, skips only TestGeocodeLive, TestOfficialLive and TestScreeningFixture; `node --test web/*.mjs` gave 1596 pass, 0 fail.

## Skipped Issues

None.

## Noted, out of scope

- `followBranch` still marks a report fanned after one pass. A follow note held back by the follow cap (after F2 that means ten follow rows in the hour) is written on a later poll but never reaches the phone, because the phone note is queued only during the pass. Pre-existing and out of scope here.
- IN-05 and IN-06 were left as they are.

---

_Fixed: 2026-11-03_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
