---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "04"
subsystem: api
tags: [go, sqlc, chi, resolver, visibility, feed, trust-mechanic]

requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility (02-03a)
    provides: service.BuildVoteTally, service.VoteTally, service.VoteKind/VoteValue, the reportID batching contract
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility (02-03b)
    provides: the four vote routes, CastVoteResponse's visibility/reason json tags and enum set, the votes_e2e_test.go helper set (newE2EServer, verifySession, postReport, newVerifiedClient, postVote, decodeVoteResponse, decodeErrorResponse)
provides:
  - "internal/store/queries/reports.sql's ReporterAccountsForReports batched query (reports LEFT JOIN sessions), NearbyReports/InsertReport/ReportsByAccount byte-for-byte unchanged"
  - "internal/service/feed.go: ReportView, (Visibility) ListableInFeed, ViewerContentVote"
  - "internal/service/report.go: Querier extended with CurrentVotesForReports/ReporterAccountsForReports, NearbyQuery gains ViewerAccountID/IncludeDisputed, ReportService.Nearby now returns []ReportView"
  - "internal/api/handlers/reports.go: FeedReportResponse (the GET /api/reports element shape), reportViewToResponse, NearbyReports reads the caller's account and the show_disputed parameter"
  - "GET /api/reports wire contract: visibility, visibility_reason, your_vote, is_own_report, plus the show_disputed query parameter"
affects: [02-05, 02-06, 02-07]

tech-stack:
  added: []
  patterns:
    - "Read-path resolver call: exactly one Resolve(meta, tally, now) call site on the read path (internal/service/report.go), gated by grep in <verify> — the same discipline 02-03a established for the write path"
    - "Batched sidecar query pattern: ReporterAccountsForReports is a separate LEFT JOIN query keyed on ids NearbyReports already returned, rather than a join added to the pinned NearbyReports query — symmetric with CurrentVotesForReports"
    - "Two hand-declared response structs (ReportResponse for POST, FeedReportResponse for GET) that never alias each other or a store row, per reports.go's package doc comment"

key-files:
  created:
    - internal/service/feed.go
    - internal/service/feed_test.go
    - internal/api/handlers/feed_visibility_e2e_test.go
  modified:
    - internal/store/queries/reports.sql
    - internal/store/sqlc/reports.sql.go
    - internal/service/report.go
    - internal/api/handlers/reports.go
    - internal/store/reports_test.go
    - internal/api/handlers/swagger_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml

key-decisions:
  - "ReporterAccountsForReports is a separate batched query, not a join added to NearbyReports — keeps the Phase 1 bounding-box/Haversine query byte-for-byte unchanged, which is itself the Anti-Pattern-1 evidence that the feed query gained no visibility/vote predicate (per plan's explicit instruction, not a new decision)"
  - "your_vote serialises as a JSON key that is always present, explicitly null (not omitted) when the caller has no standing content vote — deliberate: 02-05's JS branches on null-vs-string, not on key absence"
  - "TestNearbyReportsExcludesSessionID's context is wrapped with account.WithAccount rather than weakened; its body check additionally guards against leaking the caller's own account email"

patterns-established:
  - "A read path that needs a caller identity from the gate reads it via account.FromContext at the top of the handler and 500s if absent, mirroring votes.go's CastVote — a missing account this deep means a route was mounted outside the gated group, not a client condition"

requirements-completed: [TRUST-02, TRUST-04, TRUST-06, TRUST-08]

coverage:
  - id: D1
    description: "ReporterAccountsForReports batched (report_id, nullable reporter_account_id) query, appended to reports.sql without touching NearbyReports/InsertReport/ReportsByAccount"
    requirement: "TRUST-02"
    verification:
      - kind: integration
        ref: "internal/store/reports_test.go#TestNearbyReportsUsesIndex"
        status: pass
      - kind: integration
        ref: "internal/store/reports_test.go#TestNearbyReportsExcludesSessionID"
        status: pass
    human_judgment: false
  - id: D2
    description: "ReportService.Nearby loads a page's votes and reporter identities in two batched calls (skipped on an empty page), calls Resolve once per report, and filters on ListableInFeed — retracted excluded from both views, hidden excluded unless includeDisputed, ordering unchanged"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestListableInFeed"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestViewerContentVote"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyExcludesRetractedFromBothViews"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyHidesDisputedUnlessRequested"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyBatchesVoteReadsOnce"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyOrderingIsIndependentOfVisibility"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbySeverityDoesNotChangeGatingAmongNonCritical"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyMarksOwnReportAndViewerVote"
        status: pass
    human_judgment: false
  - id: D3
    description: "GET /api/reports carries visibility, visibility_reason, your_vote, is_own_report on every report, and accepts show_disputed (closed default, 400 on a non-boolean value); the feed's visibility for a report equals the vote endpoint's answer for that same report"
    requirement: "TRUST-04"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestFeedExcludesHiddenByDefault"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestShowDisputedRevealsHiddenReports"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestFeedNeverReturnsRetractedReports"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestFeedCarriesViewerVoteAndOwnReportFlag"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestFeedVisibilityMatchesCastVoteResponse"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestShowDisputedRejectsNonBooleanValue"
        status: pass
    human_judgment: false
  - id: D4
    description: "docs/swagger.json documents show_disputed, FeedReportResponse, and the four new fields, and never documents reporter_account_id/session_id"
    requirement: "TRUST-08"
    verification:
      - kind: unit
        ref: "internal/api/handlers/swagger_test.go#TestSwaggerSpecCoversRoutes"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-09-15
status: complete
---

# Phase 02 Plan 04: The Resolver-Backed Feed Read Path Summary

**GET /api/reports now resolves visibility live via `service.Resolve` over a batched vote/reporter read, carries `your_vote`/`is_own_report`, and accepts `show_disputed` — the feed and the vote endpoint are proven to agree on a report's visibility, over a real router and a real Postgres.**

## Performance

- **Duration:** ~25 min (4 commits, 20:55–21:09 IST)
- **Tasks:** 3 (Task 2 executed as RED then GREEN — 2 commits)
- **Files created:** 3 (`feed.go`, `feed_test.go`, `feed_visibility_e2e_test.go`)
- **Files modified:** 9 (`reports.sql`, `reports.sql.go`, `report.go`, `reports.go`, `reports_test.go`, `swagger_test.go`, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`)

## Accomplishments

- Appended `ReporterAccountsForReports :many` to `reports.sql` (LEFT JOIN `sessions`, batched by `ANY(report_ids)`) — `NearbyReports`, `InsertReport` and `ReportsByAccount` are byte-for-byte unchanged; `TestNearbyReportsUsesIndex`'s `EXPLAIN` guard and the `nearbyReportsSQLForExplain` drift const both pass unedited.
- `internal/service/feed.go`: `ReportView` (embeds `Report` plus `Visibility`, `VisibilityReason`, `ViewerVote`, `IsOwnReport`), `(Visibility) ListableInFeed(includeDisputed bool) bool` (Live/Provisional always, Hidden only when asked, Retracted never, unknown values fail closed), and `ViewerContentVote` (content votes only, viewer id 0 matches nothing via an explicit guard).
- `ReportService.Nearby` now returns `[]ReportView`: runs the unchanged bounding-box query, returns immediately on an empty page (zero extra round trips), otherwise loads `CurrentVotesForReports` and `ReporterAccountsForReports` exactly once each for the whole page, calls `Resolve` once per report with one captured `now`, and filters via `ListableInFeed` — preserving the bounding-box query's ascending-distance order.
- `GET /api/reports` reads the caller's verified account from the gate's request context, parses the optional `show_disputed` boolean (closed default; a non-boolean value is a 400 naming the field), and returns `FeedReportResponse` — a hand-declared sibling of `ReportResponse`, never aliasing it — carrying `visibility`, `visibility_reason`, `your_vote` and `is_own_report` on every report.
- `docs/swagger.json` documents `FeedReportResponse`, the four new fields and the `show_disputed` parameter; `TestSwaggerSpecCoversRoutes` now also fatals if `reporter_account_id` ever reaches the published spec.
- End-to-end proof (`TestFeedVisibilityMatchesCastVoteResponse`) that the feed's `visibility`/`visibility_reason` for a report equals the vote endpoint's `visibility`/`reason` for that same report seconds earlier — the real proof of TRUST-02/T-02-04, not a grep.

## Task Commits

Task 2 (TDD) is RED then GREEN, no REFACTOR commit — the GREEN implementation needed no cleanup.

1. **Task 1: ReporterAccountsForReports batched query** — `506c43b` (feat)
2. **Task 2 RED: failing tests for the resolver-backed feed read path** — `6641e23` (test)
3. **Task 2 GREEN: implement the resolver-backed feed read path** — `568d1cc` (feat)
4. **Task 3: wire show_disputed/your_vote/is_own_report onto GET /api/reports** — `95c8e9d` (feat)

_Note: `internal/store/sqlc/models.go` and `internal/store/sqlc/db.go` were not rewritten by `sqlc generate` in Task 1 — the new query needed no new params struct and no new model, so only `reports.sql.go` changed; the plan's "commit all three together" instruction was conditional on `sqlc` actually rewriting them._

## Files Created/Modified

- `internal/store/queries/reports.sql` — appended `ReporterAccountsForReports`; the three Phase 1/1.1 queries are untouched
- `internal/store/sqlc/reports.sql.go` — regenerated; `ReporterAccountsForReportsRow{ReportID int64, ReporterAccountID *int64}`
- `internal/service/feed.go` — `ReportView`, `ListableInFeed`, `ViewerContentVote` (new file)
- `internal/service/feed_test.go` — eight behaviour-spec test functions against a fake `Querier` with a call log (new file)
- `internal/service/report.go` — `Querier` gains two methods, `NearbyQuery` gains `ViewerAccountID`/`IncludeDisputed`, `Nearby` returns `[]ReportView`
- `internal/api/handlers/reports.go` — `FeedReportResponse`, `reportViewToResponse`, `ReportListResponse.Reports` retyped, `NearbyReports` reads the account and `show_disputed`
- `internal/api/handlers/feed_visibility_e2e_test.go` — six e2e tests over the real router/real Postgres (new file)
- `internal/store/reports_test.go` — `TestNearbyReportsExcludesSessionID` now supplies a gated account and additionally asserts no email leak
- `internal/api/handlers/swagger_test.go` — extended `TestSwaggerSpecCoversRoutes` for `show_disputed`, the four new fields, `FeedReportResponse`, and the `reporter_account_id` non-disclosure guard
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` — regenerated via `make swag` (idempotent, verified by re-running)

## Wire Contract — What Actually Shipped

**`GET /api/reports` query parameters** (existing three unchanged, one added):

| Parameter | Type | Behavior as shipped |
|---|---|---|
| `show_disputed` | boolean, optional | Absent, `""`, or `"false"` → default (Hidden excluded). `"true"` → Hidden reports additionally included. Any other value (e.g. `"yes"`) → `400` with `error.field = "show_disputed"`, `error.message = "show_disputed must be true or false."` |

**`FeedReportResponse` — every field, as shipped, with exact JSON type:**

| JSON key | Go type | Wire behavior |
|---|---|---|
| `id` | `int64` | as before |
| `category`, `severity`, `description`, `latitude`, `longitude`, `geohash` | as before | as before |
| `shelter_capacity_status`, `shelter_headcount` | `*string`/`*int`, `omitempty` | as before |
| `created_at`, `expires_at` | `string` (RFC3339Milli) | as before |
| `distance_km` | `*float64`, `omitempty` | as before |
| `visibility` | `string` | `"hidden" \| "provisional" \| "live"` — `"retracted"` is in the declared enum but never actually appears (D-12 removes it from both views) |
| `visibility_reason` | `string` | `"resolved" \| "critical_bypasses_gates" \| "disputed" \| "awaiting_second_independent_confirmation" \| "confirmed"` — the identical closed slug set `votes.go`'s `CastVoteResponse.Reason` uses |
| `your_vote` | `*string`, **no `omitempty`** | **Always present as a key.** `null` (not omitted) when the caller has no standing content vote; `"confirm"` or `"dispute"` otherwise. Confirmed by direct JSON inspection during the e2e run — this is what 02-05's JS must branch on (`value === null`, not `"your_vote" in obj`). |
| `is_own_report` | `bool` | `true` only when the calling verified account submitted the report; `false` for a nil reporter (pre-Phase-1.1/seeded reports) regardless of viewer, including viewer account id `0` |

## Decisions Made

- `ReporterAccountsForReports` implemented exactly as the plan specified (separate batched query, `LEFT JOIN sessions`, no third join through `accounts`) — no deviation from the plan's design here.
- No architectural changes required; all three tasks executed as written.

## Deviations from Plan

None — plan executed exactly as written. The `feed.go` import block, all gated function signatures (`ListableInFeed`, `ViewerContentVote`, `Nearby`), and every acceptance-criteria grep gate were matched verbatim on the first pass.

## Issues Encountered

None.

## `service.ReportView` — Final Field List (for 02-07)

```go
type ReportView struct {
	Report                          // embedded — ID, Category, Severity, Description, Latitude, Longitude, Geohash, ShelterCapacityStatus, ShelterHeadcount, CreatedAt, ExpiresAt, DistanceKm
	Visibility       Visibility
	VisibilityReason ResolveReason
	ViewerVote       VoteValue
	IsOwnReport      bool
}
```

**`ReportService.Nearby`'s new signature:** `func (s *ReportService) Nearby(ctx context.Context, q NearbyQuery) ([]ReportView, error)`.

**`NearbyQuery`'s new fields:** `ViewerAccountID int64` (0 = no identified viewer), `IncludeDisputed bool`.

`internal/store/queries/reports.sql`'s `NearbyReports` was **not** modified — confirmed by `git diff`, which shows only an appended `ReporterAccountsForReports` block with no hunk inside `NearbyReports`. `TestNearbyReportsUsesIndex` (the `EXPLAIN`-based index-scan guard) and `TestNearbyReportsQuerySourceHasExpectedShape` both passed **unedited**.

## `TestFeedVisibilityMatchesCastVoteResponse` — Observed Values (for 02-06)

Real values recorded from the passing e2e run, at a real router over real Postgres:

| Step | Vote response `visibility`/`reason` | Feed `visibility`/`visibility_reason` for the same report | Equal? |
|---|---|---|---|
| After B's confirm (1 independent cell) | `"provisional"` / `"awaiting_second_independent_confirmation"` | `"provisional"` / `"awaiting_second_independent_confirmation"` | Yes |
| After C's confirm (2nd distinct cell) | `"live"` / `"confirmed"` | `"live"` / `"confirmed"` | Yes |

Both steps used the same coordinate pair already proven distinct-cell in `votes_e2e_test.go`: report at `(12.9716, 77.5946)`, voters at `(12.9746, 77.5946)` and `(13.0500, 77.6500)`.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- 02-05 (client-side confirm/dispute controls) can key off `your_vote`/`is_own_report` exactly as documented above — `null` vs a string, never key absence.
- 02-06 (visibility display copy) has the real observed `visibility`/`visibility_reason` pairs above plus the full closed slug set to build its display-copy map against.
- 02-07 (Activity/reopen history) can build its own store query alongside this read path with the final `Nearby` signature and `ReportView` shape now fixed — this plan deliberately excludes Retracted reports from the feed by design (D-12), which is why 02-07 needs its own query.
- No blockers. `go test ./... -short` is green; `go test ./... -v -p 1` is green with `DATABASE_URL` set, including every Phase 1, Phase 1.1, 02-01, 02-02, 02-03a and 02-03b test.

## Self-Check: PASSED

- `internal/service/feed.go` — FOUND
- `internal/service/feed_test.go` — FOUND
- `internal/api/handlers/feed_visibility_e2e_test.go` — FOUND
- Commit `506c43b` — FOUND in `git log`
- Commit `6641e23` — FOUND in `git log`
- Commit `568d1cc` — FOUND in `git log`
- Commit `95c8e9d` — FOUND in `git log`
- All plan `<acceptance_criteria>` grep gates for Tasks 1–3 re-run and PASS
- Full `go test ./... -short` and `DATABASE_URL`-backed `go test ./... -v -p 1` both green
- `git diff` confirms `NearbyReports` untouched (single appended hunk); `git diff --name-only -- web/` empty; `router.go`, `cmd/server/main.go`, `visibility.go`, `trust.go`, migrations all untouched

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-15*
