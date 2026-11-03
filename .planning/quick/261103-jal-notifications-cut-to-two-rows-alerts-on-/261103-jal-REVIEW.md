---
phase: quick-261103-jal
reviewed: 2026-11-03T00:00:00Z
depth: deep
files_reviewed: 41
files_reviewed_list:
  - internal/service/pushdispatch.go
  - internal/service/pushworker.go
  - internal/service/pushworker_official.go
  - internal/service/pushkinds.go
  - internal/service/pushrecipients.go
  - internal/service/updatesalerts.go
  - internal/service/updates.go
  - internal/service/alertsettings.go
  - internal/service/follow.go
  - internal/service/profile.go
  - internal/store/queries/push.sql
  - internal/store/queries/official_push.sql
  - internal/store/queries/updates.sql
  - internal/store/sqlc/push.sql.go
  - internal/store/sqlc/official_push.sql.go
  - internal/store/sqlc/updates.sql.go
  - internal/api/handlers/me.go
  - internal/api/handlers/settings.go
  - internal/api/handlers/page.go
  - internal/api/handlers/low.go
  - internal/api/router.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - docs/PUSH.md
  - docs/OFFICIAL-ALERTS.md
  - docs/TRUST-MODEL.md
  - docs/MODERATION.md
  - README.md
  - web/templates/settings.html.tmpl
  - web/templates/welcome.html.tmpl
  - web/templates/installed_alerts_ask.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/low.html.tmpl
  - web/static/js/settings-alerts.js
  - web/static/js/welcome.js
  - web/static/js/installed-ask.js
  - web/static/js/votes.js
  - web/static/js/report-form.js
  - web/static/css/auth.css
  - web/static/css/welcome.css
findings:
  critical: 0
  warning: 4
  info: 7
  total: 11
status: issues_found
---

# Quick 261103-jal: Code Review Report

**Reviewed:** 2026-11-03
**Depth:** deep (cross file: dispatcher, worker, bell refresh, SQL, handler, pages, JS)
**Files Reviewed:** 41 source and doc files, plus the changed tests in the diff `c1bd86be..e34dff41`
**Status:** issues_found

## Summary

The delivery rules hold. I found no path where a Reliable, changed or comment note gets queued or sent to a phone:

- `EnqueueReliableFromUpdates` and `EnqueueChangedFromUpdates` have no production caller left. `cmd/` has no enqueue path either.
- `kindSwitchOn` answers false for reliable and changed rows, and for a confirmed update whose base is reliable.
- The unit table in `TestWorkerSkipsWhenNoLongerEligible` sets every stored flag on, so the not_eligible answers there do come from `kindSwitchOn` and not from a switch.

No production code reads a stored flag that could stop a Critical or official note. `criticalBranch` passes a nil filter, `OfficialPushCandidates` lost its `official_push_on` predicate, and the official worker no longer reads the setting. The PATCH handler, the narrowed `AlertSettings`, the pass through of the stored `reliable_alerts_on` (which keeps `enabled_at`) and the regenerated swagger all match C4, C7, C8 and C10. The removed pop-up and tick box leave no script that reads a missing node. `go build ./...` and `go vet ./...` are clean, and the five changed `.mjs` suites pass (328 tests). No long dash appears in any added line.

The weak spot is a consequence of D4 that nobody analysed. Every account with a home area now gets `reliable_nearby` rows. Those rows count toward the hourly bell ceiling of 10, and two other paths are gated by the same ceiling: the change rows and the follow note rows. The follow push is queued only from a written `followed_posted` row. So in a busy hour, the widened Reliable row can push a follow note off the phone (D2) and roll change rows into "And N more" for people who never opted in (D4 says those stay "exactly as today"). A second path lets a Reliable row take the shared first alert slot when the follow step fails. Two smaller warnings cover the swagger text and the dispatcher's bigger early refresh loop.

## Warnings

### WR-01: Reliable bell rows for every account now eat the shared hourly ceiling, which can stop follow phone notes and roll up change rows

**Files:**
- `internal/store/queries/updates.sql:215-221` (InsertReliableAlert ceiling)
- `internal/store/queries/updates.sql:99-104` (AdvanceCursorAndInsertUpdate `recent`)
- the same count in InsertFollowedPostedAlert (around line 247)
- `internal/service/pushdispatch.go:574-603` (followBranch)
- `internal/service/updatesalerts.go:80-95`

**Issue:** All three inserts count the account's updates in the last hour, except `more`, `comment_on_yours` and `reply_to_yours`, and compare the count with `UpdatesHourlyCeiling` (10). Before this change only opted in accounts held `reliable_nearby` rows. Now every account with a home area gets one per nearby Reliable reporter per hour (ReliableAlertsPerReporterPerHour = 1). In a real flood, ten Reliable reporters inside 5 km in one hour is plausible, and that fills the ceiling with Reliable rows alone. Then:

- **D2.** `InsertFollowedPostedAlert` returns ErrNoRows over the ceiling, so no `followed_posted` row is written. `followBranch` queues a follow push only from those rows (`EnqueueFollowFromUpdates`), and it marks the report fanned after one full pass whether or not a row was queued (line 600). The follow phone note for that report is then lost for good. A later poll writes the bell row once the hour frees, but the phone never gets it. The "follow before Reliable" order protects only one Refresh, not earlier polls.
- **D4.** For an account that never turned the old switch on, a change on its own report (`AdvanceCursorAndInsertUpdate`) now rolls into the "And N more changes" row instead of a normal row. The plan says changes "stay in the bell exactly as today".

No test pins this interaction. The new tests check one report at a time.

**Fix:** Leave the Reliable rows out of the ceiling count used by the other kinds, and give them their own cap. No migration is needed, only SQL plus `make sqlc`. For example, in AdvanceCursorAndInsertUpdate, the refresh variant and InsertFollowedPostedAlert:

```sql
AND u.kind NOT IN ('more', 'comment_on_yours', 'reply_to_yours', 'reliable_nearby')
```

and in InsertReliableAlert count all rows as today, or count `reliable_nearby` only against a smaller named cap. Add a DB test: ten Reliable rows in the hour, then a followed person posts, and the follow row is still written and queued. As a second guard, `followBranch` could skip `MarkFollowFanout` while a Notify me follower got no row because of the ceiling, so the sweep tries again inside `PushInitialWindow`.

### WR-02: A failed or truncated follow step lets the Reliable row take the shared first alert slot, so a follow note never reaches the phone; widened to every account

**Files:** `internal/service/updatesservice.go:151-158`, `internal/service/updatesalerts.go:162-170`

**Issue:** Refresh runs `refreshFollowedPosts` first, then `refreshReliableAlerts` with the ids the follow step wrote. If the follow step returns an error, `followed` is nil and the Reliable step still runs (by design: "A failure here does not stop the Reliable step"). The same happens if the report falls outside `FollowedPostsCap`. The Reliable step then writes `reliable_nearby` for a report by a person the reader follows with Notify me on. The shared unique index `uq_updates_first_alert` covers `reliable_nearby` and `followed_posted`, so the follow row can never be written for that report, and `EnqueueFollowFromUpdates` never queues the follow phone note. Before the change this traded one phone note for another, and only for opted in accounts. Now it turns a phone note (D2) into a bell only row, and it can happen to every account with a home area.

**Fix:** In `refreshReliableAlerts`, skip a candidate whose reporter is one of the reader's active Notify me followees. The follow step already reads that list through `NotifyFollowsFor`; pass the followee set into the Reliable step next to `followNoted`. A simpler option is to skip the Reliable step when `followErr != nil`:

```go
followed, followErr := s.refreshFollowedPosts(ctx, accountID, now)
var reliableErr error
if followErr == nil {
    reliableErr = s.refreshReliableAlerts(ctx, accountID, now, followed)
}
```

### WR-03: The API reference contradicts the handler about bodies without alert_radius_km

**File:** `internal/api/handlers/me.go:252-254`, regenerated into `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml`

**Issue:** The description says the old keys "are accepted and ignored. A body with no alert_radius_km is refused with the field body." The handler at line 289 answers 200 with the stored settings for a body that has no `alert_radius_km` and at least one non null old key, for example `{"critical_push":false}`. That case is pinned in `TestAlertSettingsRoute`. A client that follows the reference expects a 400 that never comes.

**Fix:** Reword and run `make swag`, for example: "A body that carries only those older keys changes nothing and answers with the stored settings. A body with neither alert_radius_km nor an older key, such as {}, is refused with the field body."

### WR-04: The Reliable early refresh now walks up to 2000 neighbours on the single dispatcher goroutine for a bell only gain, and pokes are dropped while it runs

**Files:** `internal/service/pushdispatch.go:676-698` (reliableBranch Refresh loop), `internal/service/pushdispatch.go:257-262` (ReportChanged drops a poke when the 64 slot buffer is full), `internal/service/pushdispatch.go:338-346` (ChangesSweepOnce, now every account with a device, up to 500)

**Issue:** C2 dropped the stored flag from the recipient filter. Each non serious report by a Reliable reporter now runs `UpdatesService.Refresh` for every subscribed neighbour in range, up to `PushRecipientsPerReport` (2000). Each Refresh does a full Nearby read. Before, only accounts that opted in got this loop. Pokes, the 30 second sweep and the changes sweep all run in one goroutine (`Run`), and `ReportChanged` drops a poke when the buffer is full. So a long Reliable refresh pass can delay a Critical report's fan out until a later sweep, and that is the one kind that has to arrive fast. The loop buys no phone delivery any more: the bell poll (GET /api/updates) writes the same row when the person looks.

**Fix:** Remove the Refresh loop from `reliableBranch`, keeping the tag check, the caps and `MarkReliableFanout`, or bound it to a small named constant. Do the same for `ChangesSweepOnce`'s 500 account pass, or lengthen its interval, since neither feeds a phone note now. Update the C2 and C3 comments and `TestPushReliable` to match.

## Info

### IN-01: The new installed-ask.js comment says the star box makes the same check; it does not

**File:** `web/static/js/installed-ask.js:30-34`
**Issue:** The new comment says the star box checks for an open dialog, "so the two boxes never stack". `stars.js` and `mountRateBox` in `votes.js` read no `data-modal-open` and no open dialog. In practice nothing stacks, because the ask is a `showModal` dialog and makes the page inert, but the comment points to a check that does not exist.
**Fix:** Say instead that the ask is modal, so no resolve, and so no star box, can happen while it is open.

### IN-02: rateGeneration is written and never read

**File:** `web/static/js/votes.js:2001-2004` and `:2027`
**Issue:** Deviation 4 kept the counter "as the place a later wait would take its number from". It is dead state.
**Fix:** Remove it and its increment in `takeRateBoxBack`. The one remaining SK-I04 test does not depend on it.

### IN-03: The Reliable mute and block tests are now vacuous, and TRUST-MODEL still cites them as proof of that check

**Files:**
- `internal/service/pushworker_fix_test.go:76-93` and `:125-150`
- `docs/TRUST-MODEL.md:786`
- `internal/service/pushworker.go:490`

**Issue:** Both tests now expect the control reader to be skipped too, so they cannot tell a mute or block from the blanket `kindSwitchOn` skip. `personNotMutedOrBlocked` (line 490) still runs for reliable bases but cannot change any outcome. TRUST-MODEL.md line 786 still says "The Reliable reporter note and the confirmed update that replaces one get the block and mute part of the same check" and cites `TestQueuedReliablePushStopsAfterMuteOrBlock`. PUSH.md was updated, but this sentence was not.
**Fix:** Reword line 786 the way PUSH.md line 94 now reads, so the test names stay cited for the trust model doc test. Optionally rename the two test bodies' intent in their comments to "a queued Reliable row is never sent".

### IN-04: reliableBranch reads candidates for a serious report, then leaves without marking it

**File:** `internal/service/pushdispatch.go:676-686`
**Issue:** For a serious report the filter is always false. So `PushHomeCandidates` and `reporterIsReliable` run on every 30 second sweep inside `PushInitialWindow`, and the report is never marked, because the empty recipients return comes before `MarkReliableFanout`.
**Fix:** Return before the history read when `f.Meta.Serious()`, or mark the report fanned in that case.

### IN-05: Deviation 6 differs from C4 for null only bodies

**File:** `internal/api/handlers/me.go:232-234`, `:289`
**Issue:** C4 says a body naming only old keys answers 200. `{"critical_push":null}` answers 400 field body, because a null pointer counts as not sent. This is defensible and pinned, but it is a behaviour the plan did not state.
**Fix:** None needed in code. Say it in the swagger wording from WR-03.

### IN-06: A Settings tab left open across the deploy shows old switches flipping off

**File:** `internal/api/handlers/me.go:305-308`
**Issue:** The old `settings-alerts.js` set each switch from `answer.body[field] === true`. The answer no longer carries those keys, so in a tab opened before the deploy, toggling a removed switch on gets a 200 and the switch flips back off with no message. Settings is `no-store` and `sw.js` holds no cache, so only an open tab is affected. The change is cosmetic, and nothing is stored.
**Fix:** None required. Optionally echo the four keys as false for one release.

### IN-07: Small leftovers

- `web/templates/settings.html.tmpl:77`: the section line "Choose what Pinalert tells you about." no longer fits a box with one switch and one distance. Raise it with the owner rather than change it (fix only what is asked).
- `internal/service/pushdispatch.go:741-749` and `:720-735`: `changesFor` now always returns nil and ignores `now`, so the `errs` collection in `changesBranch` is dead. Make `changesFor` return nothing, or inline the Refresh.
- `internal/service/follow.go:18`: the edited comment line runs well past the file's wrap width.

## Checks that came back clean

- **Point 1.** No production caller of `EnqueueReliableFromUpdates`, `EnqueueChangedFromUpdates`, `GetPushKindSettings` or `GetOfficialPushSetting`. No production read of `ReliableAlertsOn`, `CriticalPushOn`, `ChangesPushOn` or `OfficialPushOn`, except the pass through in `UpdateAlertSettings`. `PushHomeCandidates` and `PushAccountHome` still select the columns, but nothing filters on them. Confirmed updates of old sent Reliable rows are still queued by SQL and skipped by `kindSwitchOn`, which `TestStoredReliableAndChangesFlagsNeverReachThePhone` shows on a real database.
- **Point 2.** `since` is recomputed from `now` on every Refresh, so no poll can back fill past one hour. A stored stamp can only move the start later. The per reporter cap, mutes, person mutes, own and followed report exclusions and the Not confirmed yet wording are unchanged. The change and comment bell rows are untouched, apart from WR-01.
- **Point 3.** No JS or template references any removed id, key, global or partial: `settings-critical`, `settings-official`, `settings-changes`, `settings-reliable`, `welcome-alerts-reliable`, `installed-alerts-ask-reliable`, `welcome-ask__check`, `PinalertChangesAsk`, `changes_ask`, `saveKind`, or the four keys. The removed CSS was used only by the tick box.
- **Point 4.** Unknown keys are still refused. `{}` and null only bodies give 400 body. Radius validation is unchanged. A distance can still be saved without a home area, as before.
- **Point 5.** Deviation 3 is sound: `newPWRig`'s default claim is a Critical row about a Live Critical report, so `TestWorkerAppleFirstPushOnly` still exercises the Apple rule. Deviation 4 keeps the card Undo behaviour pinned. Deviation 6 is pinned, see IN-05. `TestCriticalDefaultMatchesSQL` and `TestOfficialPushDefaultMatchesSQL` remain.
- **Point 6.** No em or en dash in any added line. The follow note rule (the Notify me bell, send time recheck, no home area needed) is unchanged in code and docs.

---

_Reviewed: 2026-11-03_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
