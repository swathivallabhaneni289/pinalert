---
phase: 05-official-feed-demo-mode
plan: 11
subsystem: official-alerts
tags: [push, outbox, d-16, home-cell, closed-words, migration]
status: complete

requires:
  - phase: 05-05
    provides: official.Refresher, VersionHook, SavedVersion, official_alerts and official_alert_areas
  - phase: 05-08
    provides: OfficialEventWords, OfficialEventWord, OfficialLevelWord, official.NewService
  - phase: 03.3
    provides: PushKind, PushTitleMaxRunes, AreaWords, the lease pattern, push_subscriptions, DeleteAccountPushOutbox
provides:
  - migration 00013_add_official_push.sql (official_push_outbox and account_alert_settings.official_push_on)
  - queries/official_push.sql and its sqlc output (switch, candidates, enqueue, lease conditional queue, counts, alert facts)
  - service.PushKindOfficial as a built kind, the official caps and DefaultOfficialPushOn
  - service.BuildOfficialPushText, OfficialPushInput, OfficialIssuer, OfficialStateWords, OfficialTopic, OfficialTag
  - official.PushEnqueueHook, wired in official.NewService
affects: [05-14, 05-16]

key-files:
  created:
    - internal/store/migrations/00013_add_official_push.sql
    - internal/store/queries/official_push.sql
    - internal/store/sqlc/official_push.sql.go
    - internal/store/official_push_test.go
    - internal/service/pushcopy_official.go
    - internal/service/pushcopy_official_test.go
    - internal/service/pushkinds_official_test.go
    - internal/official/pushenqueue.go
    - internal/official/pushenqueue_test.go
    - internal/official/pushenqueue_db_test.go
  modified:
    - internal/store/migrations_test.go
    - internal/store/sqlc/models.go
    - internal/testutil/db.go
    - internal/service/pushkinds.go
    - internal/service/pushkinds_test.go
    - internal/service/accountdelete.go
    - internal/service/accountdelete_test.go
    - internal/official/service.go

key-decisions:
  - "Migration number 00013 (00012 was the newest). The Down drops official_push_outbox first and the official_push_on column second."
  - "The hook returns nil for everything that is not a database error (does not qualify, issuer outside the closed table, nobody to tell, alert withdrawn), because an error would roll the version back and the refresh would count it as failed."
  - "Tier decides the match: a district placed alert matches by district only (its state rows do not widen it to the state); only a state placed Extreme alert matches by state. Unplaced never matches."
  - "SavedVersion has no cancelled or superseded field, so those flags are read inside the transaction with OfficialAlertForPush; qualifies stays pure and sees the Cancel message type."
  - "An Update under a new guid is judged against the stored alerts its references name (new query OfficialPredecessorsForPush, merged into one version with the highest severity and the union of areas). Without it a wording only Update under a new guid, the common real shape, would have queued everyone again."
  - "An alert stored after the Update or Cancel that replaces it (the refresher stores newest first) is refused through OfficialAlertWithdrawnByStored, so a cold start cannot push a withdrawn alert."
  - "Candidates need a live device in the Phase 3.3 sense (a subscription joined to a session that is not revoked), not any push_subscriptions row."
  - "BuildOfficialPushText also refuses a state level Severe input (AreaIsState with Level severe). D-16 never queues one, so this is a second guard in the one place that writes the words."
  - "titleStateName reads a comma in a boundary state name as a space and an ampersand as and, so every state's SDMA passes the closed issuer check (the Dadra and Nagar Haveli and Daman and Diu name carries commas)."

requirements-completed: []

duration: about 1h 50m
completed: 2026-10-29
---

# Phase 5 Plan 11: Who is told on their phone, and the closed words Summary

Severe and Extreme official alerts for a person's stored home area are now queued, once, inside the transaction that stores each alert version, into their own leased `official_push_outbox`, with lock screen words built only from closed tables and the D-16 rule (a state level Severe alert queues nobody) enforced in three places.

## Performance

- **Duration:** about 1h 50m
- **Completed:** 2026-10-29
- **Tasks:** 3 of 3
- **Files modified:** 18 (10 created, 8 modified)

## Accomplishments

- **Storage (task 1).** Migration 00013 adds `official_push_outbox` (ids and closed words only, the 3.3 lease columns, `UNIQUE (account_id, alert_id, cap_identifier)`, an index on `(state, next_attempt_at)` and a partial index for the sent count) and `account_alert_settings.official_push_on BOOLEAN NOT NULL DEFAULT true`. The skip_reason CHECK list holds `already_notified`. `queries/official_push.sql` has the switch read and upsert (both COALESCE to true), the candidate read, the once only enqueue, the claim, lease extension and the four lease conditional marks, the 6 hour sent count, `OfficialPushSentForAlert`, the stale skip, the prune, `OfficialAlertForPush` and `DeleteAccountOfficialPushOutbox`. Truncate lists the new table and account deletion removes the rows in its own transaction. 3.3's `push_outbox`, its migration and its queries are untouched.
- **Kind, caps and words (task 2).** `PushKindOfficial` joined `PushKinds`; `ReservedPushKinds` is `[PushKindFollow]`. The seven constants are named and pinned. `BuildOfficialPushText` takes the five closed fields and builds "Cyclone warning for Krishna district" / "Extreme. Issued by IMD." style words, never feed text. `OfficialIssuer` maps IMD, CWC and `<State>-SDMA` senders to the closed issuer table. `OfficialTopic` is 22 characters of base64url of the first 16 bytes of sha256("official:" + guid) and `OfficialTag` is "official-" plus it.
- **The D-16 rule and the queue (task 3).** `PushEnqueueHook` is wired in `NewService` before the caller's options. `qualifies` is the D-16 grid; recipients come from `OfficialPushCandidates` by stored home cell decoded once per distinct cell through `ix.CellDistricts` (centre and four corners); at most 2000 per version; later versions queue only on escalation.

## Task Commits

| Task | Phase | Commit | Message |
|------|-------|--------|---------|
| 1 | RED | 81b99c5e | test(05-11): add failing tests for the official push outbox and its queries |
| 1 | GREEN | bf4b1d30 | feat(05-11): add the official push outbox, the Settings column and their queries |
| 2 | RED | 42ff6d7a | test(05-11): add failing tests for the official push kind, caps and lock screen words |
| 2 | GREEN | 61a9f93f | feat(05-11): add the official push kind, its caps and the closed lock screen words |
| fix | | 0bf9a492 | fix(05-11): write the dash guard of the official words with escapes, not dash characters |
| 3 | RED | 63e7a2c2 | test(05-11): add failing tests for the D-16 rule, the home match and queuing in the refresh |
| 3 | GREEN | 7fd9e946 | feat(05-11): queue official phone alerts inside the refresh transaction by the D-16 rule |

## Tests amended (each keeps every old case)

- `TestPushKindsClosed` (internal/service/pushkinds_test.go), note "Amended 2026-10-29 by plan 05-11 (D-11)": five built kinds with official last, `ReservedPushKinds` is `[follow]`, official is Valid. The four 3.3 kinds, their order, the wire values and the refusal of "more", "" and the other unknowns are unchanged.
- `wantDeleteOrder` and the fake querier in internal/service/accountdelete_test.go, note "Amended 2026-10-29 by plan 05-11 (T-05-62)": twelve steps, `DeleteAccountOfficialPushOutbox` right after `DeleteAccountPushOutbox`. This moves `TestDeleteAccountOrder`, `TestDeleteAccountRunsInOneTx` and `TestDeleteAccountTouchesNoOtherPersonsData`, which all read the list.
- `internal/service/pushcopy_test.go` was not edited. `TestPushCopyHasNoNoiseWords` ranges over `PushKinds` plus `ReservedPushKinds`; official now sits in `PushKinds`, `BuildPushText` still returns ok false for it (its words come from `BuildOfficialPushText`), and the "official is reserved" refusal row still holds.

## Deviations from Plan

### Auto-fixed and added (Rules 1 to 3)

**1. [Rule 2 - Missing critical functionality] Cross guid Updates and late arrivals**
- **Found during:** Task 3
- **Issue:** `prev` is the earlier version under the same guid only, but the real feed also sends an Update under a new guid that names the older identifier in `references` (the 05-05 refresher test does exactly that). Judged with `prev == nil` it is "new", so a wording only Update would queue the same people again, which breaks the plan's "a wording only Update queues nothing" truth. The refresher also stores the newest message first, so a Cancel or Update can be stored before the alert it withdraws, leaving that alert un-marked when its own hook runs.
- **Fix:** two small queries, `OfficialPredecessorsForPush` and `OfficialAlertWithdrawnByStored`; `predecessorOf` merges the referenced alerts into one `prev`; the hook refuses an alert a stored message names in its references. Covered by `TestOfficialPushEnqueueUpdates` and a store subtest.
- **Files:** internal/store/queries/official_push.sql, internal/store/sqlc/official_push.sql.go, internal/official/pushenqueue.go
- **Commit:** 7fd9e946

**2. [Rule 3 - Blocking] cancelled and superseded flags**
- **Issue:** the plan's `qualifies(saved, prev, now)` cannot see `cancelled_at` or `superseded_at` because `SavedVersion` carries neither. 
- **Fix:** `qualifies` refuses a Cancel message type (grid row), and the hook reads `OfficialAlertForPush` inside the transaction and queues nothing for a cancelled or superseded row. The superseded and withdrawn cases are proved in the database test, not the pure grid.
- **Commit:** 7fd9e946

**3. [Rule 1 - Bug] State names with commas**
- **Found during:** Task 2, by the test that passes every state's `<State>-SDMA` through `OfficialIssuer`.
- **Issue:** the boundary name "DADRA,NAGAR HAVELI,DAMAN & DIU" failed the letters and spaces issuer check, so that union territory's authority would have been dropped.
- **Fix:** `titleStateName` reads a comma as a space and "&" as "and" ("Dadra Nagar Haveli Daman and Diu SDMA", 37 runes). Exported `OfficialStateWords` is the same helper for the caller that builds a state level `Area`.
- **Commit:** 61a9f93f

**4. [Rule 1 - Bug] Dash characters in my own files**
- **Issue:** the Write tool turned my unicode escapes for the en dash and the em dash into literal dash characters in `pushcopy_official.go` and its test, against the no dash rule.
- **Fix:** replaced with escapes in a follow up commit (no history rewrite). The perl check is clean on every file this plan wrote.
- **Commit:** 0bf9a492

### Small choices beyond the plan text

- `BuildOfficialPushText` refuses `AreaIsState` with Level severe (D-16 second guard). The plan's refusal list had four items; this is a fifth and is tested.
- `OfficialPushCandidates` joins `sessions` and requires `revoked_at IS NULL` (a live device, as 3.3 defines it), and the migration adds a partial index `(account_id, sent_at) WHERE state = 'sent'` for the window count.
- `ClaimDueOfficialPushes` orders escalated rows first, then by id, and returns the row directly from the update (no joins are needed here).
- The version hook does no feed freshness check: that belongs to the plan 05-16 worker, and `last_success_at` has not moved yet while the hook runs.

**Total deviations:** 4 auto-fixed, none architectural. No Rule 4 stop.

## Notes for plan 05-16 and 05-14

- Queue facts to read: `official_push_outbox` rows carry `reason` (`new` or `escalated`), `cap_identifier`, `expires_at` (the earlier of the alert's end and now plus `OfficialPushTTL`). The hook queues the row; the worker must still check the alert at send time with `OfficialAlertForPush` (cancelled, superseded, expiry, `last_seen_at`) and the switch with `GetOfficialPushSetting`.
- The state level `Area` for `OfficialPushInput` is `service.OfficialStateWords(<boundary state name>)` with `AreaIsState` true; a district `Area` is the boundary district name plus " district".
- **Known limit for the Apple rule:** `OfficialPushSentForAlert(account, alert_id)` is per alert row. An Update under a new guid is a different alert row with a different Topic and Tag, so an Apple device that was pushed for the first alert is not seen as "already notified" by that query, and the platform will not replace the earlier note. If 05-16 wants "one notification per series" on Apple it needs the series link (the `references_text` of the newer alert names the older identifier). The same-guid case works as the plan describes.
- Official rows live in their own table, so they never count toward 3.3's hourly cap. The 6 hour cap is `CountAccountOfficialPushesInWindow` against `OfficialPushPerAccountPer6Hours` (3) and is for the worker.

## Verification

- Full suite on the plan's own database `p5_11`, `go test ./... -p 1 -count=1 -v`: **4990 `--- PASS`, 0 `--- FAIL`, 0 `^FAIL` lines, 3 `--- SKIP` (TestGeocodeLive, TestOfficialLive, TestScreeningFixture, all allowed).** Every package `ok`. None of the known flakes (TestCastVoteConcurrentSameAccountKeepsEveryRow, TestVotedReportsByAccountUsesTheAccountIndex) failed, so none was rerun. The node tests were not run: no JavaScript changed.
- The named tests all passed: TestMigration00013, TestOfficialPushQueries, TestPushKindsClosed, TestOfficialPushCapsAreNamed, TestOfficialPushDefaultMatchesSQL, TestBuildOfficialPushText, TestBuildOfficialPushTextRefuses, TestOfficialPushInputHasNoFeedTextField, TestOfficialPushCopyHasNoNoiseWords, TestOfficialTopic, TestOfficialIssuer, TestOfficialQualifies, TestOfficialNewlyCovered, TestOfficialPushEnqueue, TestOfficialPushEnqueueUpdates, TestOfficialPushRecipientCap and TestDeleteAccountRemovesOfficialPushRows. A mutation of the state Extreme rule failed `TestOfficialQualifies` and `TestOfficialPushEnqueue` as it should.
- `go vet ./...` is clean. `gofmt -l internal` lists only `internal/api/handlers/place_test.go` and `internal/geocode/photon.go`, neither touched by this plan.
- Acceptance greps: `PushKindFollow`-only reserved list, `DefaultOfficialPushOn = true`, `OfficialPushPerAccountPer6Hours = 3`, `func BuildOfficialPushText(in OfficialPushInput) (PushText, bool)`, `func OfficialTopic(guid string) string`, `WithVersionHook(PushEnqueueHook(` in service.go, `func PushEnqueueHook(ix *areas.Index) VersionHook`, and zero feed text field reads (`HeadlineEN`, `HeadlineOriginal`, `DescriptionEN`, `Instruction`) outside comments in pushenqueue.go. `git diff --stat` on `00011_add_push.sql`, `queries/push.sql`, `pushcopy.go` and `pushcopy_test.go` is empty.
- Database `p5_11` was created, used, dropped, and `psql -lqt` no longer lists it. No server was started and nothing was sent.

## Issues Encountered

- The harness's commit reminder named a different model in the Co-Authored-By line than the project rules do; the project rule's exact trailer (`Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`) was used on every commit.

## User Setup Required

After the owner says yes, run `make migrate` against the owner's own dev database so `official_push_outbox` and `official_push_on` exist before that server sends official alerts. An executor never runs it.

## Known Stubs

None. Nothing in this plan renders to a screen; the send worker is plan 05-16 and the Settings row is plan 05-14.

## Threat Flags

None beyond the plan's register. T-05-57 (closed five field input, reflection test), T-05-58 (D-16 rule, windows, escalation only, 2000 cap, unique index), T-05-59 (cells read by server code only, never logged), T-05-60 (queued inside the version's transaction, `ON CONFLICT DO NOTHING`), T-05-61 (3.3 files untouched, amended tests noted) and T-05-62 (deleted with the account, tested) are mitigated.

## Self-Check: PASSED

- All 10 created files and 8 modified files exist in the worktree.
- All seven task and fix commits (81b99c5e, bf4b1d30, 42ff6d7a, 61a9f93f, 0bf9a492, 63e7a2c2, 7fd9e946) are on branch worktree-agent-ae510d93d890aa1de.
