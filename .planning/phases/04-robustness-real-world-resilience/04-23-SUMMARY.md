---
phase: 04-robustness-real-world-resilience
plan: 23
subsystem: api
tags: [moderation, moderator-screen, blocked]
status: blocked
requirements-completed: []
completed: 2026-10-16
---

# Phase 4 Plan 23: Moderator API Summary (BLOCKED, nothing built)

**Status: blocked before any code edit.** No task was started and no source file changed. The plan needs a one line edit to a file it does not own (`web/chrome_contract_test.go`), and the executor rules forbid editing an unlisted file. This SUMMARY records the blocker and the step 0 findings so the orchestrator can fix the conflict and re-run the plan.

## BASE

`3b45dd1b1933a7e0651df891b2a0a5a644675bec`

## The blocker

`web/chrome_contract_test.go`, `TestDisclaimerBarOnEveryFullPage`, line 43:

```
hasAccount := strings.HasSuffix(path, "index.html.tmpl") || strings.HasSuffix(path, "profile.html.tmpl") || strings.HasSuffix(path, "low.html.tmpl")
```

Lines 56 to 58 then fail any other full page that calls `account_header.html.tmpl` with "this page has no account and must not call the account header". The plan's Task 1 requires `moderate.html.tmpl` to call the account header inside `.top-chrome` (must_haves truth 3, template action text) and its verify command requires `TestDisclaimerBarOnEveryFullPage` to pass. Both cannot hold while the test file is unchanged. The test file is not in this plan's `files_modified`, and 04-22 and 04-25 list it as read only.

Proposed fix (one line, for the orchestrator or a plan that owns the file): add `|| strings.HasSuffix(path, "moderate.html.tmpl")` to the `hasAccount` expression on line 43.

Other web guards that hard code a page list: a grep of `HasSuffix(path` over `web/*_test.go` found only that line for template names. `slow_notice_contract_test.go:46` skips `index.html.tmpl` only (a skip, unrelated to the header). `findFullPageTemplates` (theme_contract_test.go) is generic: it treats any template containing `<html` as a full page, so the new page is covered by the theme, bar and `chrome.css` guards from the moment it exists.

Alternatives that would also unblock: the orchestrator edits that test line itself before re-running 04-23, or amends 04-23's `files_modified` to include `web/chrome_contract_test.go`.

## Step 0 findings (all read only, all as the plan expects except the blocker)

- 04-05 generated names present in `internal/store/sqlc`: `ReviewQueue`, `FlagQueue`, `PendingQueue`, `ReportForModeration`, `ModerationQueueCounts`, `ModerationStatusForReport`, `KeepReport`, `RemoveReport`, `InsertModerationAction`, plus `CurrentVotesForReports(ctx, reportIds []int64)` and `ReporterAccountsForReports(ctx, reportIds []int64)`.
- Row field types: `ReviewQueueRow`, `FlagQueueRow`, `PendingQueueRow`, `ReportForModerationRow` share `ID int64, Category string, Severity string, Description string, CreatedAt time.Time, ExpiresAt time.Time, ClientWrittenAt pgtype.Timestamptz, ModerationState string, CheckStatus, Verdict *string, Scores []byte, Model *string, NeedsReview bool, OpenFlagTotal int64, FlagCounts []byte`. `CheckStatus` is `string` on `ReviewQueueRow` and `PendingQueueRow` and `*string` on `FlagQueueRow` and `ReportForModerationRow`. `ModerationQueueCountsRow{ReviewCount, FlaggedCount, PendingCount int64}`. `ModerationStatusForReportRow{ID int64, ModerationState, Severity, Category string, ExpiresAt time.Time, InReview, InFlagged, InPending bool}`. `InsertModerationActionParams{ReportID, ModeratorAccountID int64, Action string}`. `KeepReport` and `RemoveReport` take `reportID int64` and return `error`. `ReportForModeration` takes `reportID int64`.
- `writtenAtPtr(pgtype.Timestamptz) *time.Time` and `WrittenAtForDisplay` exist in `internal/service` (report.go, feed.go).
- Built `Nearby` tally recipe: one `ReporterAccountsForReports(ids)` and one `CurrentVotesForReports(ids)` read (plus a reporter history read and a `ReportExtrasForReports` read that the moderator service does not need), then per report `BuildVoteTally(votesByReport[id], id, reporterByReport[id])`, `meta := ReportMeta{Severity, Category, Moderation: ModerationState(...)}`, `Resolve(meta, tally, now)`. The moderator service needs only the two batched reads and the clock.
- Phase 3.1: `ViewOf(v Visibility, expiresAt, now time.Time) FeedView` and `FeedView` (`FeedViewActive`, `FeedViewDisputed`, `FeedViewResolved`, `FeedViewPast`, fail closed `FeedViewNone` = "") exist in `internal/service/history.go`, pure, no I/O, no window.
- Phase 3.2 bell: `profile.html.tmpl` has no bell markup inside `.top-chrome`. The account header partial reads only `.Email` and `.Chrome.LowDataPage`; the bar partial reads only `.Chrome...`. The theme toggle partial reads nothing.
- 04-11 tweak shape: `p11Server` in `submit_screening_e2e_test.go` calls `newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) { d.Reports = service.NewReportService(q, service.WithScreener(scr)) })`. `TextScreener` is `Screen(ctx, text) (moderation.Result, error)`.
- Feed helper for the e2e: `getNearby(t, client, baseURL, lat, lon, showDisputed)` and `decodeFeedResponse` / `findFeedReport` in `feed_visibility_e2e_test.go`. Other helpers present: `newVerifiedClient`, `postVote`, `submitReportGetID`, `floodReportBody`, `decodeErrorResponse`, `getProfile`, `expireReport`, `newE2EServerWith`, `verifySession`.
- No name collisions: none of `ModerationService`, `ModerationQuerier`, `ModerationRow`, `ModerationQueues`, `ModerationFlagCount`, `ModView*`, `ModList*`, `ModWord*`, `ErrAlreadyHandled`, `NormalizeEmail`, `ModeratorEmails`, `fakeModerationQuerier` exists yet. `moderation_view_test.go` and `moderationcopy.go` exist in `internal/service` but declare none of them.

## T-04-16 question (pending reports with no check row)

The moderator queue would NOT show them. 04-05's `PendingQueue` inner joins `moderation_checks` (`PendingQueueRow.CheckStatus` is a plain `string`), so a pending report whose check row was never written appears in no list, and `ModerationStatusForReport` reports `InPending` false for it (the `COALESCE(c.status = 'pending' ...)` is false when there is no check row). Find by id would still return its row with `lists` empty, and Remove would work on it, but Keep would answer 409 (not in any list). Listing such reports needs a new query (04-05 owns the SQL; this plan may not add one), so it is a decision for the owner.

## Tasks

None executed. No RED state to record, no commits other than this SUMMARY.

## Deviations from Plan

1. **[Blocker, Rule 1 of the executor rules]** The plan needs `web/chrome_contract_test.go` changed (one line) and the file is not in `files_modified`. Recorded and returned, as the rules require.
2. A context usage hook repeatedly reported "Usage at 88%" while the harness token counter showed about 14.8 million tokens left; the stop here is for the blocker, not for context.

## Known Stubs

None (nothing built).

## Self-Check: FAILED

The plan was not executed: an unlisted file (`web/chrome_contract_test.go`) must change before Task 1's `TestDisclaimerBarOnEveryFullPage` can pass with the account header on `moderate.html.tmpl`. Only this SUMMARY exists; no source file, template or test was touched.
