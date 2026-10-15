---
phase: 04-robustness-real-world-resilience
plan: 23
subsystem: api
tags: [moderation, moderator-screen, gate, audit]
requires:
  - phase: 04-05
    provides: ReviewQueue, FlagQueue, PendingQueue, ModerationQueueCounts, ModerationStatusForReport, ReportForModeration, KeepReport, RemoveReport, InsertModerationAction
  - phase: 04-07
    provides: WrittenAtForDisplay, writtenAtPtr, TestResolveCallersMentionModeration
  - phase: 04-14
    provides: the worker that leaves unfinished checks pending
  - phase: 04-20
    provides: FlagReasons, FlagReasonLabel
provides:
  - ParseModeratorEmails and NormalizeEmail, MODERATOR_EMAILS parsed once at startup
  - requireModerator, a gate with its own lookup that answers a byte equal 404 on every method
  - ModerationService (Queues, Find, Keep, Remove) and the four moderator JSON routes
  - /moderate page shell (moderate.html.tmpl)
affects: [04-26, 04-27]
tech-stack:
  added: []
  patterns:
    - "A moderator group outside the access gate, every path through r.Handle so the gate sees every method"
    - "Audit row written before the state change"
key-files:
  created:
    - internal/service/moderation.go
    - internal/service/moderation_test.go
    - internal/api/moderator.go
    - internal/api/handlers/moderation.go
    - internal/api/handlers/moderator_e2e_test.go
    - web/templates/moderate.html.tmpl
  modified:
    - internal/service/auth.go
    - internal/api/router.go
    - cmd/server/main.go
    - web/chrome_contract_test.go
key-decisions:
  - "Audit row before the state change; two moderators acting at the same instant may write two audit rows (both documented on ModerationService)"
  - "409 field is id, matching the pinned 404 field"
status: complete
completed: 2026-10-16
---

# Phase 4 Plan 23: Moderator API Summary

**A listed moderator can read the three queues, find any report by id, and Keep or Remove it with exactly one audit row, while logged out, unverified and unlisted visitors get a 404 byte equal to an unknown route on every method.**

## Commits

- `acbb64f` Task 1: moderator page and the byte equal 404 gate (parser, NormalizeEmail, gate, page, template, router group, MODERATOR_EMAILS)
- `696dcd0` Task 2: the three queues and Find by id
- `e52e07b` Task 3: Keep, Remove and the audit row
- `5089483` an earlier commit of this plan: the first, blocked SUMMARY (replaced by this file)

BASE: `3b45dd1b1933a7e0651df891b2a0a5a644675bec`.

## Step 0 findings

- 04-05 generated names and types as expected. `ClientWrittenAt` is `pgtype.Timestamptz`; `Scores` and `FlagCounts` are `[]byte`; `CheckStatus` is `string` on `ReviewQueueRow` and `PendingQueueRow` and `*string` on `FlagQueueRow` and `ReportForModerationRow`; `Model` and `Verdict` are `*string`; counts and status rows are as listed in 04-05. `KeepReport`, `RemoveReport` and `ReportForModeration` take `reportID int64`.
- Built `Nearby` tally: `ReporterAccountsForReports(ids)` and `CurrentVotesForReports(ids)` once, then per report `BuildVoteTally(votesByReport[id], id, reporterByReport[id])` and `Resolve(ReportMeta{Severity, Category, Moderation}, tally, now)`. `ModerationService.buildRows` does the same with only those two reads and the clock.
- Phase 3.1: `ViewOf` and `FeedView` present and pure. `ModerationViewOf` calls `ViewOf` once and restates no window. `ViewOf` knows nothing about moderation, so the withheld wrap is `ModerationViewOf` and the builder.
- Phase 3.2 bell: none in `profile.html.tmpl`; the account header reads `.Email` and `.Chrome.LowDataPage` only, so the moderator view model is `AssetVersion`, `Email`, `Chrome`.
- 04-11 tweak shape: `newE2EServerWith` with `d.Reports = service.NewReportService(q, service.WithScreener(...))`; feed helper `getNearby` with `decodeFeedResponse` and `findFeedReport`.
- No name collisions for any new identifier.

## RED state of each task

- Task 1: `go vet ./internal/service/` failed with `undefined: service.ParseModeratorEmails` before the parser existed.
- Task 2: `undefined: service.ModerationQuerier` before the service existed.
- Task 3: `undefined: service.ModAlreadyHandledMessage` before Keep, Remove and the sentinel existed.

## Verification

- `go build ./...`, `go vet ./...` and `gofmt -l .` clean.
- Database run on a fresh `p4_0423` (created by this run, dropped after): `DATABASE_URL=postgres://localhost:5432/p4_0423?sslmode=disable go test ./internal/api/handlers/ -run '^TestModerator' -count=1 -p 1 -v` gives exactly 7 top level `--- PASS` lines (TestModeratorPageRenders, TestModeratorRoutesIndistinguishableFrom404, TestModeratorQueues, TestModeratorKeep, TestModeratorRemove, TestModeratorRemoveById, TestModeratorAuditRow) and 0 `--- SKIP`.
- The same variable over `./internal/api/... ./internal/service/ ./web/ -count=1 -p 1` all `ok`; and over `go test ./... -count=1 -p 1` all `ok`. `--- SKIP` count for the moderator, server copy and Resolve guard tests: 0.
- Quick run with `DATABASE_URL` unset: `go test ./... -count=1` all `ok`.
- Service tests: TestParseModeratorEmails, TestNormalizeEmailMatchesSignIn, TestServerCopyModeratorAPI, TestModerationViewOf, TestModerationQueuesBatchReads, TestModerationRowMapping, TestModerationFind, TestModerationKeepRemoveDecisions, plus TestResolveCallersMentionModeration, TestDisclaimerBarOnEveryFullPage, TestTopChromeGeometry, TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage, TestUserVisibleCopyUsesPlainPunctuation, TestSwaggerSpecCoversRoutes, TestSwaggerDocServed all pass.
- No U+2014 or U+2013 in any new file or added line. `git status --porcelain -- docs internal/store` is empty. No swag directive, no migration, no query, no generated file.

## Helpers added

Service tests: `fakeModerationQuerier`, `modT0`, `modStr`, `modReviewRow`, `modFlagRow`, `modPendingRow`, `modDetailRow`, `modService`, `modCallsAmong`. E2E: `modBarSentence`, `modNoFollow`, `modDo`, `modSameAsUnknown`, `modStrip`, `modRenderPage`, `modAllowList`, `modStubScreener`, `modUnfinishedTexts`, `modFixture`, `modSetup`, `modFlag`, `modGetQueue`, `modFind`, `modAction`, `modWantAction`, `modInFeed`, `modHas`, `modIDs`, `modWalkKeys` and the wire types `modQueueWire`, `modFindWire`, `modActionWire`.

## Decisions (Claude's Discretion, owner can veto)

1. The audit row is written before the state change: the store uses no transaction, and a recorded attempt that did not apply is safer for repudiation than an applied change with no record. A failed state change answers 500 and the moderator retries.
2. Two moderators acting on one report at the same instant may both pass the 409 check and write two audit rows. Accepted for an owner run screen.
3. 409 answers field `id` (matching the pinned 404 field).

## Deviations from Plan

**1. [Scope addition, approved by the coordinator] `web/chrome_contract_test.go`.** `TestDisclaimerBarOnEveryFullPage` allows the account header only on pages whose name ends in `index.html.tmpl`, `profile.html.tmpl` or `low.html.tmpl`. The plan's moderator page calls the header, so I added `|| strings.HasSuffix(path, "moderate.html.tmpl")` to the `hasAccount` expression (one line, in the Task 1 commit `acbb64f`). The file is not in the plan's `files_modified`; the coordinator ruled that no other wave 7 plan edits it.

**2.** The plan's verify commands use `env -u DATABASE_URL`, which the sandbox refuses; quick runs used `go test` with `DATABASE_URL` unset in the shell, which is the same thing.

**3.** An earlier executor run of this plan stopped at the blocker above and committed a blocked SUMMARY (`5089483`); this file replaces it.

**4.** A context usage hook repeatedly reported "88% used" while the harness token counter showed about 14.7 million tokens left; it did not affect the work.

## Owner items

- **Pending reports with no check row (T-04-11 / T-04-16): the moderator queue does not show them.** `PendingQueue` inner joins `moderation_checks` and `ModerationStatusForReport` reports `InPending` false without a check row, so a report whose check row was never written appears in no list. Find by id still returns it (with `lists` empty), Remove works on it, Keep answers 409. Listing them needs a new query in plan 04-05's area; not added here, per the coordinator.

## Known Stubs

None. The page shell's lists are empty `<ol>` elements on purpose; 04-26 fills them from the JSON API and adds the stylesheet and script tags. The template shows no invented count.

## Threat Flags

None beyond the plan's register: the new routes are the `/moderate` and `/api/moderation/*` paths the plan names, all behind `requireModerator`.

## Notes for the orchestrator

- 04-26 needs nothing from this plan beyond the ids and classes the template carries and the JSON shape in `modRowJSON`.
- Browser harness checks (V-24 human-check in the plan) are for the owner and were not run; no server was started.

## Self-Check: PASSED

Files exist: `internal/service/moderation.go`, `internal/service/moderation_test.go`, `internal/api/moderator.go`, `internal/api/handlers/moderation.go`, `internal/api/handlers/moderator_e2e_test.go`, `web/templates/moderate.html.tmpl`. Commits `acbb64f`, `696dcd0` and `e52e07b` are on the branch. All plan verification commands ran and passed on a fresh `p4_0423`, which was dropped afterwards.
