---
phase: 3
slug: trust-model-hardening-diversity-weighted-trust
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-30
---

# Phase 3: Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Source of truth for the
> behavior to test map is `03-RESEARCH.md`, section "Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` (go 1.26.4); the web contract tests are ordinary Go tests that read the embedded JS, CSS and templates |
| **Config file** | none (Makefile targets `test`, `test-short`, `vet`, `sqlc`, `swag`) |
| **Quick run command** | `go test ./internal/service/ ./web/ -count=1` (no database needed, about 2 seconds) |
| **Full suite command** | `DATABASE_URL="postgres://localhost:5432/<disposable>?sslmode=disable" go test ./... -p 1 -count=1` |
| **Disposable database** | `createdb p3_verify` before the run and `dropdb p3_verify` after it, with the migrations applied to it. **Never `pinalert_test`** (the owner's dev server database; `testutil.NewTestDB` truncates it) and never `pinalert`. |
| **Estimated runtime** | about 15 seconds for the full suite |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/service/ ./web/ -count=1`, plus the package the task touched
- **After every plan wave:** the full suite command above, against the disposable database
- **Phase gate:** full suite green; `go vet ./...`; `sqlc generate` then `git diff --exit-code internal/store/sqlc`; `swag init -g internal/api/router.go -o docs` then `git diff --exit-code docs`; the manual UAT items below
- **Before `/gsd-verify-work`:** full suite must be green
- **Max feedback latency:** 30 seconds for the quick run

---

## Per-Task Verification Map

Task ids are assigned by the planner. Each row below is a required test from the research; every plan
task must cite at least one row (or a Wave 0 dependency) in its `<verify>` block. Rows V-01 to V-42
map TRUST-05, TRUST-07, decisions D-01 to D-22 and derived rules R-01 to R-08 to named tests.

| Req / Decision | Behavior | Test type | Automated command | File exists? |
|---|---|---|---|---|
| V-01 D-01 | Haversine correctness, symmetry, NaN and Inf refused, India latitude table | unit | `go test ./internal/service/ -run 'TestHaversine|TestWithinVoteRadius' -count=1` | Wave 0 (geo_test.go) |
| V-02 D-01 | Voter inside 0.9896 km accepted, 1.0008 km refused (report 12.9716,77.5946) | unit | same package `-run TestWithinVoteRadiusBoundary` | Wave 0 |
| V-03 D-01 | `BoundingBox` contains every point within R (oracle for helper) | property | `-run TestHaversineInsideBoundingBox` | Wave 0 |
| V-04 D-01 | `CastVote` refuses all four values from far, zero inserts | unit | `go test ./internal/service/ -run TestCastVoteRejectsTooFar -count=1` | Wave 0 (trust_test.go) |
| V-05 D-01 | Order: own report before expiry before distance | unit | `-run TestCastVoteCheckOrder` | Wave 0 |
| V-06 D-01 | Reporter resolve or reopen exempt from anywhere, non reporter not, accountless exempts nobody | unit | `-run TestCastVoteReporterExemption` | Wave 0 |
| V-07 D-01 | Handler returns 403 `field: location`, fixed copy, no distance in body | e2e (DB) | `go test ./internal/api/handlers/ -run TestCastVoteTooFar -count=1` | Wave 0 |
| V-08 D-01 | Existing vote e2e still pass with C = 12.9686,77.5946 | e2e (DB) | `go test ./internal/api/handlers/ -count=1` | edit existing |
| V-09 D-01 | `ReportVoteContext` returns report coordinates | store (DB) | `go test ./internal/store/ -run TestReportVoteContextReturnsCoordinates` | Wave 0 |
| V-10 D-03 | Cell width and witness test (at most 24 distinct cells from 200 witnesses in 300 m at five latitudes) | unit | `go test ./internal/service/ -run TestGeohashPrecisionWitnesses` | Wave 0 |
| V-11 D-04 TRUST-05 | `ConfirmedPlaces == ConfirmCells`, dedupe, permutation, reporter excluded | property | `-run TestBuildTrustCountsMatchTally` | Wave 0 |
| V-12 D-16 R-04 TRUST-05 | Counter labels for confirmed and disputed (0, 1, 2, 99, 100, both always shown, zeros included) | unit | `-run TestTrustCountLabel` | Wave 0 |
| V-13 D-05 R-01 TRUST-07 | Currency boundary table (W-1ns, W, W+1ns, 2W, 2W+1ns, 4W, skew) for 8 h and 24 h | unit | `-run TestCurrencyScoreBoundaries` | Wave 0 |
| V-14 R-01 | N below 2 gives null score and `not_yet_corroborated`, also for critical | unit | `-run TestCurrencyNotYetCorroborated` | Wave 0 |
| V-15 D-05 | One account re-tapping cannot refresh; two places can | unit | `-run TestCurrencyRetapCannotRefresh` | Wave 0 |
| V-16 lean | Dispute raises the bar by one place, never refreshes; contested gives 0 | unit | `-run TestCurrencyDisputes` | Wave 0 |
| V-17 D-05 | Window is a quarter of lifetime (8 h and 24 h, rescue needed low severity is 8 h) | unit | `-run TestCurrencyWindowFollowsLifetime` | Wave 0 |
| V-18 | Currency monotone in time and places | property | `-run TestCurrencyMonotone` | Wave 0 |
| V-19 D-07 R-02 R-03 | Fade stage mapping, none for critical, rescue needed, Hidden, not applicable | unit | `-run TestFadeStageMapping` | Wave 0 |
| V-20 D-11 D-12 R-05 R-06 | Outcome classification table, equals `Resolve` on neutral inputs | unit and property | `-run TestClassifyOutcome` | Wave 0 |
| V-21 D-13 | Reliability worked table, boundaries (n 4 vs 5, 30 cap, exact 75 and 35) | unit | `-run TestReliabilityTable` | Wave 0 |
| V-22 D-13 D-15 | Invariants I1 to I4 (spotless always Reliable, unconfirmed only with n of at least 5 always Unreliable per D-23, monotone, bounded) | property | `-run TestReliabilityInvariants` | Wave 0 |
| V-23 D-13 | Half life weights and 90 day window edge | unit | `-run TestReliabilityDecayAndWindow` | Wave 0 |
| V-24 D-13 | Index named `idx_reports_session_expires` used, no Seq Scan on reports | store (DB) | `go test ./internal/store/ -run TestReporterHistoryUsesSessionIndex` | Wave 0 |
| V-25 D-13 | History bounds: window, cap 30, expired only, multi session, accountless | store (DB) | `-run TestReporterHistoryBounds` | Wave 0 |
| V-26 D-13 | Query source drift guard | store | `-run TestReporterHistoryQuerySourceHasExpectedShape` | Wave 0 |
| V-27 D-13 | Migration 00005 embedded and index exists | store (DB) | `go test ./internal/store/ -run 'TestMigrations|TestSessionExpiresIndexExists'` | edit existing |
| V-28 D-10 | Reliability keyed by account across sessions | store and service | `-run TestReporterHistoryUsesAccountNotSession` | Wave 0 |
| V-29 D-21 | Feed and vote response carry equal `trust`; batching keeps one vote read; account cap | service and e2e | `-run 'TestNearbyBatches|TestFeedVisibilityMatchesCastVoteResponse'` | edit existing |
| V-30 D-21 | Swagger has the new definitions, enums, no `cell`, `account`, `session`, `email` property names | unit | `go test ./internal/api/handlers/ -run TestSwaggerSpecCoversRoutes` | edit existing |
| V-31 D-21 | Response bodies contain no `account_id`, `geohash_cell`, email | e2e (DB) | `-run TestTrustResponseOmitsIdentifiers` | Wave 0 |
| V-32 D-16 to D-19 | Every server string is dash free and from the closed charset | unit | `go test ./internal/service/ -run TestTrustCopy` | Wave 0 |
| V-33 D-20 R-08 | Activity rows carry `data-trust`; block updates from `CastVoteResponse.trust` after Reopen | e2e and contract | `go test ./internal/api/handlers/ ./web/ -run 'TestProfile|TestActivity'` | Wave 0 |
| V-34 D-22 | `data-vote-radius-km` equals `service.MaxVoteDistanceKm`; read without `Pinalert.config` | unit and contract | `go test ./internal/api/handlers/ ./web/ -run 'TestPageShell|TestVoteRadius'` | Wave 0 |
| V-35 D-02 | `votes.js` one prompt, one fetch, cache with timestamp, retry once path present | contract | `go test ./web/ -run TestVoteTransport -count=1` | edit existing |
| V-36 D-22 | Hint uses `aria-disabled`, never `disabled` in `updateVoteBlock` | contract | `go test ./web/ -run TestVoteHint` | Wave 0 |
| V-37 D-07 R-02 | No `Pinalert.ageStage(` in `feed.js` or `map.js`; both use `PinalertTrust.fadeStage` | contract | `go test ./web/ -run TestFadeUsesCombinedStage` | Wave 0 |
| V-38 D-18 D-19 | Same block mounted on both surfaces and Activity; no banned literals in consumers; load order | contract | `go test ./web/ -run TestTrust` | Wave 0 |
| V-39 D-19 | Tooltip: Escape, focus, hover, click present; no `aria-live`; no markup sink | contract | `go test ./web/ -run TestTrustTooltip` | Wave 0 |
| V-40 D-18 | trust CSS tokens only, `:not([hidden])` guards, no `999px`, no long dash | contract | `go test ./web/ -run 'TestTrustCSS|TestNoPillShapedControls|TestUserVisibleCopy'` | Wave 0 |
| V-41 TRUST-09 spirit | Concurrent votes then N equals distinct places | concurrency (DB) | `go test ./internal/api/handlers/ ./internal/store/ -race -run TestTrustCountUnderConcurrentVotes` | Wave 0 |
| V-42 all | Hover, tap, keyboard focus on a touch device and desktop; screen reader reads reasons; too far hint with DevTools Sensors location override; Reopen refresh on Activity; 320 px width wrapping | manual UAT | script in the phase UAT file | manual |

*Status for every row starts as pending and is updated by the executor and verifier.*

---

## Wave 0 Requirements

- [ ] Disposable database procedure written into the plans (never `pinalert_test`)
- [ ] `voteRowAt` helper (vote row with an explicit `created_at`) in the service tests, plus a helper that builds many confirming places quickly
- [ ] History fixture seeder (accounts, bound sessions, reports with explicit `expires_at`, votes with explicit `created_at`), likely in `internal/testutil`
- [ ] Clock options on `ReportService` and `VotingService` for deterministic end to end tests; `ActivityForAccount` uses the injected clock
- [ ] The three existing fakes gain `ReporterHistoryReports`; the `ReportVoteContextRow` fixtures gain report coordinates
- [ ] End to end helpers `decodeTrust` and a swagger property name walker
- [ ] New contract test file `web/trust_contract_test.go` and the updated `votes.js` anchors

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Hover, tap and keyboard focus each show the reason for every word and tag; a screen reader reads the reasons; Escape dismisses | TRUST-05, TRUST-07 (D-19) | Real pointer, touch and assistive technology behavior cannot be asserted from Go | Use a fresh disposable database and fresh accounts. Open a report row and a map popup on desktop and on a phone sized viewport, and on a touch device if available. |
| The too far hint disables Confirm, Dispute and Mark resolved with a reason, and the server still refuses a forced vote | TRUST-05 (D-01, D-22) | Needs a browser location override | Use DevTools Sensors to set a location more than 1 km from a report; check the disabled state and hover reason, then try to vote anyway. |
| Reopen on the Activity page refreshes the block; the block wraps cleanly at 320 px width | TRUST-07 (D-20, R-08) | Layout and live refresh | Reopen a resolved report of your own and watch the block; resize to 320 px. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency under 30 seconds for the quick run
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
