# Deferred Items — Phase 07

Out-of-scope discoveries logged during plan execution, per the executor's Scope Boundary rule
(only auto-fix issues directly caused by the current task's changes; pre-existing failures in
unrelated files are logged here, not fixed).

## Plan 07-02

- **Pre-existing test flakiness in `go test ./... -short -count=1`, unrelated to this plan's
  changes.** Observed during Task 2's final full-suite verification pass. This plan's own two
  tasks touch only `web/templates/index.html.tmpl`, `internal/api/handlers/page_test.go`,
  `web/static/css/modal.css`, and `web/css_contract_test.go` — no Go backend logic in
  `internal/api`, `internal/store`, or `internal/testutil`.

  Three separate full-suite runs (two parallel, one with `-p 1` for serial execution) each
  produced a **different** set of failing tests across `pinalert/internal/api`,
  `pinalert/internal/api/handlers`, `pinalert/internal/store`, and
  `pinalert/internal/testutil` (e.g. `TestExpiryReadTimePredicate`,
  `TestAccessGateAllowsVerified`, `TestNearbyReportsUsesIndex`, `TestNewTestDB`,
  `TestTruncateClearsIdentityTables`, several `TestProfile*` tests). Every failing test passed
  when re-run in isolation (`-run <name>` against its own package). This is consistent with
  shared-state or timing contention against the local `pinalert_test` Postgres database across
  packages/subtests, not a regression introduced by this plan.

  This plan's own verification (task-level `go test ./web/...` and
  `go test ./internal/api/handlers/ -run TestPageShellServesDOMContract`, plus the full
  `go test ./web/ -count=1` suite) passed cleanly and deterministically every run. The narrower,
  task-scoped acceptance criteria in `07-02-PLAN.md` are all satisfied.

  Not fixed here per the Scope Boundary rule. Worth a follow-up look before the phase's overall
  verification gate (e.g. running with `-p 1 -parallel 1` at the phase level, or auditing
  `internal/testutil`'s DB setup/teardown for a shared-fixture race) if it recurs during
  `/gsd-execute-phase 07`'s post-wave regression gate.
