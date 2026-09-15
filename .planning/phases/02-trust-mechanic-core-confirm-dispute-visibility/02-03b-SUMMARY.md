---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: "03b"
subsystem: api
tags: [chi, http-handlers, swagger, e2e-testing, trust-mechanic, voting]

# Dependency graph
requires:
  - phase: 02-03a
    provides: "VotingService.CastVote, CastVoteInput/CastVoteResult, the three sentinel errors (ErrCannotVoteOwnReport, ErrReportExpired, ErrReportNotFound), VoteKind/VoteValue vocabulary"
provides:
  - "Four live HTTP routes: POST /api/reports/{id}/confirm, /dispute, /resolve, /reopen — all inside the single gated r.Group"
  - "handlers.CastVote(svc, kind, value) factory — one handler produces all four endpoints"
  - "handlers.CastVoteRequest{Latitude, Longitude} / CastVoteResponse{Visibility, Reason} wire contract"
  - "api.Deps.Votes *service.VotingService, wired in cmd/server/main.go from the same queries handle Reports/AuthService share"
  - "internal/api/handlers.newE2EServer now returns (*httptest.Server, *recordingMailer, *pgxpool.Pool) — the three-value form 02-04/02-07 must call"
  - "docs/swagger.json documents all four vote paths, their 401/400/403/404/409 responses, and the four visibility enum values"
affects: ["02-04", "02-05", "02-06", "02-07"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "One handler-factory-per-route-family pattern: CastVote(svc, kind, value) closes over the kind/value pair so four routes share one JSON contract and one error map, rather than four near-duplicate handlers"
    - "Sentinel-error-to-status mapping via errors.Is ahead of errors.As(ValidationError) — mirrors reports.go's convention, security-relevant refusals ordered first"
    - "e2e test harness threading a *pgxpool.Pool third return value through newE2EServer so later plans' tests can seed/assert rows directly without a second parallel harness"

key-files:
  created:
    - internal/api/handlers/votes.go
    - internal/api/handlers/votes_e2e_test.go
  modified:
    - internal/api/router.go
    - cmd/server/main.go
    - internal/api/handlers/reports_e2e_test.go
    - internal/api/handlers/swagger_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml

key-decisions:
  - "Registered all four vote routes flat (four separate r.Post calls), not via r.Route(\"/api/reports/{id}\", ...) as 02-PATTERNS.md sketched — router.go already forbids a wildcard subrouter overlapping the literal /api/reports leaf, and flat registration keeps the routes visibly inside the gated group's line-ordered list (T-01-70)."
  - "Broke gofmt's struct-field alignment group with a blank line after the new Votes field in api.Deps so it aligns to its own type name rather than the longer AuthService/Auth/Template/Page group — purely cosmetic, no behavior change."
  - "409 (not 410 or 400) for an expired report on vote: the report still exists and is reachable via the owner's Activity history, and nothing about the request itself is malformed — matches trust.go's own reasoning."

requirements-completed: [TRUST-01, TRUST-03, TRUST-08]

coverage:
  - id: D1
    description: "A verified account can POST to all four vote paths and receives 200 with the freshly resolved {visibility, reason} pair"
    requirement: "TRUST-01"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestReporterCanResolveOwnReportInstantly"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestIndependentConfirmsFlipProvisionalToLive"
        status: pass
    human_judgment: false
  - id: D2
    description: "An unverified caller receives 401 from the gate and writes no vote row; all four routes inherit requireVerifiedAccount by construction"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestCastVoteRequiresVerifiedAccount"
        status: pass
    human_judgment: false
  - id: D3
    description: "The reporter is refused 403 on confirm/dispute but can resolve their own report instantly with a retracted visibility (D-03, D-13)"
    requirement: "TRUST-08"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestCastVoteRejectsReporterSelfVote"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestReporterCanResolveOwnReportInstantly"
        status: pass
    human_judgment: false
  - id: D4
    description: "Two confirms from two verified accounts in one geohash cell leave a report provisional; two confirms from distinct cells flip it to live (TRUST-03's independence predicate proven at the HTTP boundary)"
    requirement: "TRUST-03"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestConfirmsFromOneCellStayProvisional"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestIndependentConfirmsFlipProvisionalToLive"
        status: pass
    human_judgment: false
  - id: D5
    description: "A single non-reporter reopen leaves a retracted report retracted; a second reopen from a distinct cell un-retracts it — the only test reaching /reopen at all"
    requirement: "TRUST-08"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestReopenRequiresIndependentAgreement"
        status: pass
    human_judgment: false
  - id: D6
    description: "A vote against an expired report is 409, a missing report is 404, an unknown body field is 400 — none write a vote row"
    verification:
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestCastVoteOnExpiredReportIsRejected"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestCastVoteOnMissingReportIs404"
        status: pass
      - kind: e2e
        ref: "internal/api/handlers/votes_e2e_test.go#TestCastVoteRejectsUnknownBodyField"
        status: pass
    human_judgment: false
  - id: D7
    description: "docs/swagger.json documents all four vote paths with their 401/403 responses and the four visibility enum values; the drift guard fails on a future undocumented route"
    verification:
      - kind: unit
        ref: "internal/api/handlers/swagger_test.go#TestSwaggerSpecCoversRoutes"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-15
status: complete
---

# Phase 02 Plan 03b: Confirm/Dispute/Resolve/Reopen HTTP Routes Summary

**Four `POST /api/reports/{id}/{confirm,dispute,resolve,reopen}` routes, mounted inside the existing verified-account gate via one `CastVote(svc, kind, value)` handler factory, proven end to end over a real router and Postgres, with a regenerated OpenAPI spec.**

## Performance

- **Duration:** ~40 min
- **Started:** 2026-09-15T14:37:00Z (approx, per STATE.md session start)
- **Completed:** 2026-09-15T15:16:48Z
- **Tasks:** 3 completed
- **Files modified:** 8 (2 created, 6 modified)

## Accomplishments

- One `handlers.CastVote(svc, kind, value) http.HandlerFunc` factory produces all four vote endpoints, so the JSON contract and the sentinel-error-to-status mapping exist exactly once
- All four routes registered flat inside `router.go`'s single gated `r.Group`, provably between the gate's `r.Use(requireVerifiedAccount(...))` (line 164) and the group's last route, `POST /auth/logout` (line 202) — the four `CastVote` registrations sit at lines 187-190
- `api.Deps` gained `Votes *service.VotingService`, wired in `cmd/server/main.go` from the same `queries` handle `Reports`/`AuthService` already share, with no new environment variable
- Nine new end-to-end tests in `votes_e2e_test.go` drive the real router against a real Postgres: the 401/403/409/400/404 refusal paths and the TRUST-03 independence predicate (one cell stays provisional, two distinct cells go live), all proven with a `SELECT COUNT(*) FROM votes` assertion that a refused vote wrote nothing
- `reports_e2e_test.go`'s `newE2EServer` now returns the pool as a third value and wires `Votes`, so every e2e test in the package — current and future — drives one fully-wired router
- `docs/swagger.json`/`docs.go`/`swagger.yaml` regenerated via `make swag`; `TestSwaggerSpecCoversRoutes` extended to assert all four vote paths, their 401/403 responses, and the four visibility enum values

## Task Commits

Each task was committed atomically:

1. **Task 1: One handler factory, four vote endpoints, the full sentinel-error-to-status map** - `14b4f73` (feat)
2. **Task 2: Mount the four routes inside the gate and construct the service at boot** - `beba830` (feat)
3. **Task 3: Prove it end to end over a real router and a real Postgres, then make the API reference tell the truth** - `79800ca` (test)

_Note: Task 3 is `tdd="true"` per the plan, but is a single behavioral-proof task (e2e tests over an already-implemented handler from Tasks 1-2), not a RED/GREEN/REFACTOR cycle — there is no separate failing-test-then-implementation split here since the implementation already existed. One `test(...)` commit covers it, matching the plan's own task structure._

## Files Created/Modified

- `internal/api/handlers/votes.go` - `CastVote` factory, `CastVoteRequest`/`CastVoteResponse`, `parseReportID`, the sentinel-error-to-status map
- `internal/api/handlers/votes_e2e_test.go` - nine e2e tests over a real router + real Postgres, plus shared test helpers (`newVerifiedClient`, `postVote`, `decodeVoteResponse`, `decodeErrorResponse`, `submitReportGetID`, `countVotesForReport`, `assertDistinctCells`, `assertSameCell`)
- `internal/api/router.go` - `Deps.Votes` field, four flat `r.Post` registrations inside the gated group
- `cmd/server/main.go` - `Votes: service.NewVotingService(queries)` in the `api.Deps` literal
- `internal/api/handlers/reports_e2e_test.go` - `newE2EServer` returns `(*httptest.Server, *recordingMailer, *pgxpool.Pool)` and wires `Votes`
- `internal/api/handlers/swagger_test.go` - `TestSwaggerSpecCoversRoutes` extended for the four vote paths/responses/visibility enums
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` - regenerated by `swag init`

## The Wire Contract (for 02-05/02-06/02-07)

**Final route paths, exactly as registered** (all four inside the gated group, all four from one factory):

| Method | Path | Kind | Value |
|--------|------|------|-------|
| POST | `/api/reports/{id}/confirm` | `service.VoteKindContent` | `service.VoteConfirm` |
| POST | `/api/reports/{id}/dispute` | `service.VoteKindContent` | `service.VoteDispute` |
| POST | `/api/reports/{id}/resolve` | `service.VoteKindResolution` | `service.VoteResolve` |
| POST | `/api/reports/{id}/reopen` | `service.VoteKindResolution` | `service.VoteReopen` |

**Request body** — `CastVoteRequest`, exactly two fields, `DisallowUnknownFields` enforced:

```json
{"latitude": <float>, "longitude": <float>}
```

**Success response (200)** — `CastVoteResponse`:

```json
{"visibility": "hidden|provisional|live|retracted", "reason": "resolved|critical_bypasses_gates|disputed|awaiting_second_independent_confirmation|confirmed"}
```

**Full status-to-message table as shipped:**

| Status | When | `error.field` | `error.message` |
|--------|------|----------------|-------------------|
| 400 | malformed/unknown-field body | `body` | "Request body is missing or malformed." |
| 400 | unparseable or non-positive `{id}` | `id` | "Report id must be a number." |
| 400 | out-of-range coordinate (from the service's `ValidationError`) | `latitude`/`longitude` | the GPS-denial copy from `02-UI-SPEC.md` |
| 401 | unverified caller (from the gate, not this handler) | `auth` | "Verify your email to continue." |
| 403 | reporter casting a content vote on their own report | `account` | "You can't vote on your own report." |
| 404 | no such report | `report` | "Report not found." |
| 409 | report already expired | `report` | "This report has expired." |
| 500 | anything else | — | plain-text `internal server error`, detail logged server-side only |

## Observed visibility/reason pairs — `TestIndependentConfirmsFlipProvisionalToLive`

Recorded from an actual `-v` test run against real Postgres, so 02-06 can map display copy against real values rather than assumed ones:

1. Report submitted (low severity, flood, Bengaluru coordinates) — no votes yet.
2. Client B confirms from a location ~330m from the report → **200**, `visibility: "provisional"`, `reason: "awaiting_second_independent_confirmation"` (one independent cell, below `IndependentAgreementThreshold`).
3. Client C confirms from a location several km away (distinct precision-7 cell, asserted via `geohash.EncodeWithPrecision` before the request) → **200**, `visibility: "live"`, `reason: "confirmed"` (two distinct cells, threshold met).

## New `newE2EServer` Signature

```go
func newE2EServer(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool)
```

02-04 and 02-07's e2e tests must call the three-value form. `reports_e2e_test.go`'s own two existing call sites were updated to `srv, mailer, _ := newE2EServer(t)`.

## Decisions Made

- Registered all four vote routes flat (`r.Post` on literal paths), not via `r.Route("/api/reports/{id}", ...)` as `02-PATTERNS.md` sketched — matches `router.go`'s existing "every `/api/*` path is registered flat" rule and keeps the routes visibly inside the gated group's line-ordered list, which the acceptance criteria verify by line number (T-01-70).
- Broke gofmt's struct-field alignment group with a blank line after the new `Votes` field in `api.Deps`, so `grep -c 'Votes \*service.VotingService'` matches exactly once with a single space — a cosmetic fix with no behavior change, needed because gofmt otherwise aligns `Votes` to the longer `AuthService`/`Auth`/`Template`/`Page` group.
- Reworded a doc comment on `CastVoteRequest` to avoid a second literal occurrence of "DisallowUnknownFields" (the acceptance criteria require exactly one occurrence, matching the actual `dec.DisallowUnknownFields()` call).
- 409 (not 410 or 400) for an expired report: the report still exists and is reachable via the owner's Activity history, and the request itself isn't malformed — matches `service.ErrReportExpired`'s own documented reasoning in `trust.go`.

## Deviations from Plan

None - plan executed exactly as written. Two small self-corrections were made and verified before committing (not deviations from the plan's intent, but fixes to hit the plan's own literal acceptance-criteria greps):

1. **[Rule 1 - Bug] Duplicate literal "DisallowUnknownFields" string in votes.go**
   - **Found during:** Task 1's acceptance-criteria verification (`grep -c 'DisallowUnknownFields'` returned 2, not 1)
   - **Issue:** The doc comment on `CastVoteRequest` repeated the literal string "DisallowUnknownFields" that also appears in the actual `dec.DisallowUnknownFields()` call, failing the acceptance criterion's exact-count grep
   - **Fix:** Reworded the doc comment to describe the behavior without repeating the literal API name
   - **Files modified:** internal/api/handlers/votes.go
   - **Verification:** `grep -c 'DisallowUnknownFields' internal/api/handlers/votes.go` = 1
   - **Committed in:** 14b4f73 (Task 1 commit)

2. **[Rule 1 - Bug] gofmt struct alignment broke the Votes field's exact-match acceptance criterion**
   - **Found during:** Task 2's acceptance-criteria verification (`grep -c 'Votes \*service.VotingService'` returned 0)
   - **Issue:** gofmt column-aligns contiguous struct fields into groups; because the `Votes` field's preceding comment broke it into a new group with `AuthService`/`Auth`/`Template`/`Page` (the longest name), gofmt padded `Votes` with 7 spaces instead of 1, failing the acceptance criterion's literal single-space grep
   - **Fix:** Added a blank line after the `Votes` field declaration to isolate it into its own one-field alignment group
   - **Files modified:** internal/api/router.go
   - **Verification:** `grep -c 'Votes \*service.VotingService' internal/api/router.go` = 1
   - **Committed in:** beba830 (Task 2 commit)

---

**Total deviations:** 0 substantive deviations. 2 self-corrections to hit the plan's own literal acceptance-criteria greps, both resolved before committing.
**Impact on plan:** None on scope or behavior — both corrections were cosmetic (comment wording, whitespace) and are captured in the commits' own diffs.

## Issues Encountered

None. All three tasks' acceptance criteria and the plan-level `<verification>` steps (1-7) were re-run and passed after Task 3's commit:
- `gofmt -l internal/api/ cmd/ docs/` — clean
- `go build ./... && go vet ./...` — clean
- `make swag` — idempotent (`git status docs/` clean after a second run)
- `go test ./... -short` — green, no DATABASE_URL
- `go test ./... -v -p 1` — green with DATABASE_URL set, 155 PASS / 0 FAIL
- All nine `votes_e2e_test.go` tests: PASS, none SKIP
- TRUST-01/TRUST-08 plan-named verification commands: green

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- 02-04 (feed integration: `GET /api/reports` visibility filtering, `?show_disputed=true`) can now build directly on this plan's `newE2EServer` three-value signature and the `Votes` dependency wiring pattern.
- 02-05/02-06 (client-side vote buttons and display copy) have a proven wire contract: the exact JSON field names, the full status-to-message table, and the observed visibility/reason pairs recorded above.
- 02-07 (Activity/profile page) can reuse `newE2EServer`'s pool return and the `submitReportGetID`/`newVerifiedClient` test helpers this plan established.
- No blockers.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Completed: 2026-09-15*

## Self-Check: PASSED

All 9 claimed files found on disk; all 3 task commit hashes (`14b4f73`, `beba830`, `79800ca`) confirmed in `git log`.
