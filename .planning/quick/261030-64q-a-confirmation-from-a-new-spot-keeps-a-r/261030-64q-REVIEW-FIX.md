---
phase: quick-261030-64q
fixed_at: 2026-10-30T00:00:00Z
review_path: .planning/quick/261030-64q-a-confirmation-from-a-new-spot-keeps-a-r/261030-64q-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 6
skipped: 3
status: partial
---

# Quick task 261030-64q: Code Review Fix Report

**Source review:** `.planning/quick/261030-64q-a-confirmation-from-a-new-spot-keeps-a-r/261030-64q-REVIEW.md`
**Base:** 4e443e6c. Scratch database p_q64q_fix_a (created, used, dropped).

**Summary:**
- Findings in scope: 9 (3 Warning, 6 Info)
- Fixed: 6 (WR-01, WR-02, WR-03, IN-01 as a stated effect, IN-03 as a stated limit, IN-06)
- Not fixed: 3 (IN-02, IN-04, IN-05), each with its reason below

## Fixed Issues

### WR-03: One report can send several "fades in about an hour" phone notes

**Commit:** c651078f
**Files:** `internal/store/queries/updates.sql`, `internal/store/sqlc/updates.sql.go`, `internal/store/updates_fading_refresh_test.go` (new), `internal/store/updates_test.go`, `internal/service/pushworker.go` (comment), `docs/TRUST-MODEL.md`, `docs/PUSH.md`
**Applied fix:** `MarkFadingAndInsertUpdate` now keeps ONE fading bell row per account and report. When one exists, the event refreshes it in place (same id, `created_at` moved to now, `read_at` cleared) instead of inserting another. The hour gap and the soft ceiling still apply to a refresh (over the ceiling it rolls into the "more" row and the fading row is left as it was). The phone note needed no new rule: `EnqueueChangedFromUpdates` is already unique on `(account_id, update_id)`, and a refreshed row keeps its id, so a second extension into the last hour queues no second note. That holds for every device, Apple or not, and the 3.3 names (`deviceSkipsRepeatPush`, `heldByHourlyCap`, `AccountPushSentAboutReport`) are untouched, so there is no second copy of any rule. No migration (00025 not used).
**Tests:** `TestFadingRefreshedInPlaceNoSecondPush` failed first (second event returned a new id, 2 fading rows, 1 extra phone note) and passes. `TestFadingRefreshRespectsCeiling` pins the ceiling path. `TestFadingOncePerExpiry` was amended with a dated note: it pinned 2 fading rows after a changed expiry, now 1 (the event still re-arms; it refreshes the row).
**Docs:** TRUST-MODEL.md (two places) and PUSH.md no longer say "one more" or "a fresh" fading row; they say the bell refreshes its one row and the phone gets the note at most once per person and report.
**Status:** fixed: requires human verification (the refresh makes the bell row unread again and moves it to the top, which is the "latest state" reading of the owner's rule; say so if you would rather it stay read).

### WR-01: Fading negative push cases passed on the time check alone

**Commit:** 3ae0af4d
**Files:** `internal/service/pushworker_test.go`
**Applied fix:** the provisional, hidden, retracted and withheld fading cases in `TestWorkerSkipsWhenNoLongerEligible` now use `fadesSoon`. Mutation run: with `MayNotifyAboutReport` dropped from the `UpdateFading` case of `eventStillHolds`, the provisional, hidden and retracted cases fail with "mark = sent, want skipped with not_eligible". The withheld case still passes under that mutation because `eventStillHolds` rejects Withheld before the switch, which is its own visibility reason. Mutation reverted (`git checkout`), suite green.

### WR-02: Demo parity test could not see an extension

**Commit:** 9575e6af
**Files:** `internal/demo/parity_db_test.go`, `docs/TRUST-MODEL.md`
**Applied fix:** `viewSnap` gains `ConfirmExtends`. The scenario takes an optional post hook, and `TestDemoParityExtensionWithSQL` runs the same script on a report posted 7 hours ago (SQL side through `testutil.InsertStoredReport`, world side through `SubmitChecked` with the world clock moved back 7 hours), so the first confirm moves the fade time by hours and the lifetime reads about 15 hours, with an explicit spot check on that. Proof: with the twin's `InsertVoteExtending` forced to never extend, the old `TestDemoParityWithSQL` still passed and the new test failed on every step from "A confirms" on (database 54000 s, world 28800 s). Mutation reverted. TRUST-MODEL.md now names `TestDemoParityExtensionWithSQL` and explains why the fresh-report test cannot tell an extension from none.

### IN-06: p20MustCite missed tests TRUST-MODEL cites

**Commit:** 80443ad1
**Files:** `internal/service/trust_model_doc_test.go`
**Applied fix:** added `TestExtendedExpiryRule`, `TestConfirmExtendsFlag`, `TestCastVoteExtendsOnlyOnConfirm`, `TestMemQuerierExtendsLikeSQL`, `TestCurrencyWindowKeepsNormalLifetime`, `TestFeedAndFocusCarryConfirmExtends`, `TestDemoParityExtensionWithSQL` and `TestFadingRefreshedInPlaceNoSecondPush`. Red shown by renaming a cited test in the doc: the doc test failed on both the missing and the unknown name; reverted.

### IN-03: A confirm cast while resolved uses up that spot and person

**Commit:** ccb688bb
**Files:** `internal/service/expiry_resolved_spot_db_test.go` (new), `internal/service/trust_model_doc_test.go`, `docs/TRUST-MODEL.md`
**Applied fix:** accepted as a limit, as the review allowed. TRUST-MODEL rule 7 now says it, and `TestConfirmWhileResolvedUsesUpSpotAndPerson` pins it on a real database (same person, other spot: no extension; other person, used spot: no extension; new person, new spot: extends). The doc pin failed first (test not cited), then passed.

### IN-01: Browser time fade divides by the whole extended life

**Commit:** 44d0e413
**Files:** `internal/service/trust_model_doc_test.go`, `docs/TRUST-MODEL.md`
**Applied fix:** documented, not changed in code. A code fix means the server sends a second life number on the feed and focus JSON (and the regenerated API reference) just for a look that the card words already correct; the review also says the design is intended. TRUST-MODEL now states the absolute effect (Critical capped at 48 hours: aging look with 12 hours left, stale look with 6, against the last 3 hours for an unextended Critical report), pinned by a new doc test subtest that failed first.

## Not fixed

### IN-02: Returned fade time comes from the statement snapshot when no extension runs
**File:** `internal/store/queries/extend.sql:79`
**Reason:** the review says none needed. The client refetches after every vote and the window is capped; a comment line would have no behavior to pin.

### IN-04: A queued Critical or Reliable push keeps the expiry copied before an extension
**Files:** `internal/service/pushdispatch.go:509`, `internal/service/pushworker.go:442`, `internal/store/queries/push.sql:719`
**Reason:** the review says none needed at this scale. Changing the stale rule in `SkipStalePushes` and the worker would touch the Phase 3.3 push paths for a case that needs a long failing retry, against the rule of fixing only what is asked.

### IN-05: ROADMAP 999.12 still shows `- [ ] TBD`
**Reason:** left alone on purpose, as instructed (entries marked DONE keep it, as in 999.7).

## Final checks

- Full suite once, last: `go test ./... -p 1 -count=1 -v` on p_q64q_fix_a: 0 `--- FAIL`, 0 package `FAIL`, 0 lines starting `panic:`, 2439 top level `--- PASS`. Only skips: `TestGeocodeLive`, `TestOfficialLive`, `TestScreeningFixture`. No "DATABASE_URL not set".
- `node --test web/*.mjs`: 1654 pass, 0 fail.
- Scratch database dropped. No migration, no server, no push, no image.

---

_Fixed: 2026-10-30_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
