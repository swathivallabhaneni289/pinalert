---
phase: 01-foundation-report-map
plan: 01
subsystem: infra
tags: [go, chi, pgx, sqlc, goose, postgres, github-actions, ci]

# Dependency graph
requires: []
provides:
  - Go module `pinalert` (go 1.26.4) with the full directory skeleton from 01-RESEARCH.md
  - First goose migration: `reports` + `sessions` tables, TIMESTAMPTZ everywhere, three indexes
  - Embedded migrations (`store.MigrationsFS`) and a standalone `cmd/migrate` runner
  - `internal/testutil` shared test-database helper (`NewTestDB`, `Truncate`, `MustExec`,
    `SeedReports`, `SeedExpiringReport`)
  - `sqlc.yaml` wired to pgx/v5 with timestamptz/nullable overrides pre-resolved
  - GitHub Actions CI (build/vet/test against a postgres:16 service container)
  - `Makefile` and a documented full-stack local run in README.md
affects: [01-02, 01-03, 01-04, 01-05, 01-06, 01-07]

# Tech tracking
tech-stack:
  added:
    - github.com/go-chi/chi/v5 v5.3.2 (pinned, not yet imported — plan 01-03)
    - github.com/jackc/pgx/v5 v5.10.0
    - github.com/mmcloughlin/geohash v0.10.0 (imported by testutil/seed.go)
    - github.com/pressly/goose/v3 v3.28.0
    - github.com/swaggo/http-swagger/v2 v2.0.2 (pinned, not yet imported — plan 01-07)
    - sqlc v1.31.1 CLI (dev tool, `go install`, not a module dependency)
    - swag v1.16.6 CLI (dev tool, `go install`, not a module dependency)
  patterns:
    - Migrations embedded via `//go:embed` and applied only by cmd/migrate or
      internal/testutil — never on application boot (T-01-10)
    - TIMESTAMPTZ for every time column, expires_at materialized at write time
      (Pitfall 5 / FOUND-05)
    - internal/store/sqlc/ output committed to git — CI needs no sqlc binary

key-files:
  created:
    - go.mod
    - go.sum
    - sqlc.yaml
    - Makefile
    - .github/workflows/ci.yml
    - cmd/migrate/main.go
    - internal/store/migrations/00001_create_reports.sql
    - internal/store/migrations.go
    - internal/store/migrations_test.go
    - internal/testutil/db.go
    - internal/testutil/seed.go
    - internal/testutil/db_test.go
  modified:
    - .gitignore
    - README.md

key-decisions:
  - "go.mod's go directive left at the toolchain's installed version (1.26.4) rather than
    pinned literally to 1.25 — 01-RESEARCH.md explicitly recommends building on whatever the
    current stable release is, and 1.26.4 satisfies the plan's '1.25+' constraint"
  - "chi, geohash, and http-swagger/v2 pinned in go.mod via `go get` even though only geohash
    is imported by any code in this plan — prevents a future plan's `go get`/`go mod tidy` from
    silently resolving a different (possibly newer, unaudited) version when the import finally
    lands, preserving the T-01-SC supply-chain pin"
  - "geohash manually promoted from the indirect to the direct require block in go.mod after
    testutil/seed.go started importing it, without running `go mod tidy` (which would have
    stripped the still-unimported chi/http-swagger/v2 pins again)"

patterns-established:
  - "Standalone `cmd/migrate` binary is the only thing that ever runs goose.Up/-Down/-Status —
    the server and testutil both consume the same embedded store.MigrationsFS but never invoke
    a migration binary themselves at boot"
  - "internal/testutil.NewTestDB(t) is the one call every future store-layer test uses to get a
    migrated, truncated Postgres connection or a clean t.Skip with no DATABASE_URL"

requirements-completed: [OPS-02, FOUND-05, FOUND-06, FOUND-03]

coverage:
  - id: D1
    description: "Go module compiles from scratch with no application code beyond the scaffold"
    requirement: null
    verification:
      - kind: unit
        ref: "go build ./... && go vet ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "First migration creates reports+sessions with TIMESTAMPTZ columns and the three FOUND-03/FOUND-05 indexes; idempotent on re-run"
    requirement: "FOUND-03"
    verification:
      - kind: unit
        ref: "internal/store/migrations_test.go#TestMigrationsEmbedded"
        status: pass
      - kind: integration
        ref: "go run ./cmd/migrate (run twice against an empty pinalert_test database — second run is a no-op)"
        status: pass
    human_judgment: false
  - id: D3
    description: "TIMESTAMPTZ used for every time column, no bare TIMESTAMP declarations"
    requirement: "FOUND-05"
    verification:
      - kind: unit
        ref: "grep -c TIMESTAMPTZ internal/store/migrations/00001_create_reports.sql (>=3, 0 bare TIMESTAMP)"
        status: pass
    human_judgment: false
  - id: D4
    description: "shelter_capacity_status and shelter_headcount exist on reports and are nullable"
    requirement: "FOUND-06"
    verification:
      - kind: integration
        ref: "psql -d pinalert_test -c '\\d reports' (both columns present, Nullable column blank)"
        status: pass
    human_judgment: false
  - id: D5
    description: "internal/testutil provides a migrated, truncated, optionally-seeded Postgres connection to any store-layer test in later plans"
    requirement: null
    verification:
      - kind: integration
        ref: "internal/testutil/db_test.go#TestNewTestDB"
        status: pass
      - kind: integration
        ref: "internal/testutil/db_test.go#TestSeedExpiringReport"
        status: pass
    human_judgment: false
  - id: D6
    description: "CI runs build, vet, and test against a real postgres:16 service container on every push/PR"
    requirement: "OPS-02"
    verification:
      - kind: other
        ref: "python3 yaml-parse check on .github/workflows/ci.yml confirming go build/go vet/go test steps and a postgres service block"
        status: pass
    human_judgment: true
    rationale: "CI's actual green run can only be confirmed once this branch is pushed to GitHub and the workflow executes there; local verification confirms the YAML is well-formed and the same commands pass locally, but the live Actions run itself is not observable from this worktree."

duration: 40min
completed: 2026-09-06
status: complete
---

# Phase 1 Plan 1: Foundation Scaffold Summary

**Greenfield Go 1.26 module (chi/pgx/sqlc/goose) with an embedded-migration schema, a shared
migrated-Postgres test helper, and postgres:16-backed GitHub Actions CI — zero application code.**

## Performance

- **Duration:** 40 min
- **Started:** 2026-09-06T07:12Z (approx, first tool call)
- **Completed:** 2026-09-06T07:52:50Z
- **Tasks:** 3
- **Files modified:** 14 (11 created, 2 modified: `.gitignore`, `README.md`; plus `go.mod`/`go.sum`
  generated by tooling)

## Accomplishments
- Stood up the `pinalert` Go module with the full `cmd/`, `internal/`, `web/`, `docs/` directory
  skeleton from 01-RESEARCH.md, with all five direct dependencies pinned at their proxy-verified
  exact versions (no `@latest` resolution anywhere)
- Wrote the first goose migration (`reports` + `sessions`, all four time columns TIMESTAMPTZ, the
  nullable shelter-capacity pair, and the three FOUND-03/FOUND-05 indexes), embedded it via
  `store.MigrationsFS`, and built `cmd/migrate` as the sole thing that ever applies it
- Built `internal/testutil` (`NewTestDB`, `Truncate`, `MustExec`, `SeedReports` via
  `pgx.CopyFrom`, `SeedExpiringReport`) so every later plan's store-layer test gets a migrated,
  truncated Postgres connection with zero external CLI tools, or a clean skip with no
  `DATABASE_URL`
- Wired `sqlc.yaml` to `pgx/v5` with the timestamptz/nullable-text/nullable-integer overrides
  pre-resolved (Assumption A6), so plan 01-03 hits no surprise compile error
- Added `.github/workflows/ci.yml` (build/vet/test against a `postgres:16` service container),
  a `Makefile`, and a documented full-stack local run in `README.md`

## Task Commits

Each task was committed atomically:

1. **Task 1: Go module scaffold, schema migration, and the explicit migration runner** - `fb045df` (feat)
2. **Task 2: Shared test-database helper and bulk seeder** - `b999934` (feat)
3. **Task 3: GitHub Actions CI and the documented full-stack local run** - `8438691` (feat)

**Plan metadata:** committed as part of this SUMMARY (worktree mode — orchestrator handles the
final docs commit after merge; STATE.md/ROADMAP.md are not touched by this agent per its
instructions).

## Files Created/Modified
- `go.mod`, `go.sum` - module `pinalert`, five pinned direct deps, dev tools installed separately
- `sqlc.yaml` - pgx/v5, package `sqlcgen`, timestamptz/nullable overrides
- `internal/store/migrations/00001_create_reports.sql` - reports+sessions schema, three indexes
- `internal/store/migrations.go` - `store.MigrationsFS` embed.FS
- `internal/store/migrations_test.go` - asserts the embed contains the migration and both markers
- `cmd/migrate/main.go` - standalone up/down/status runner, fails fast without `DATABASE_URL`
- `internal/testutil/db.go` - `NewTestDB`, `Truncate`, `MustExec`
- `internal/testutil/seed.go` - `SeedReports` (CopyFrom), `SeedExpiringReport`
- `internal/testutil/db_test.go` - exercises both helpers against a real Postgres
- `.github/workflows/ci.yml` - build/vet/test, postgres:16 service container
- `Makefile` - migrate/run/test/test-short/vet/sqlc/swag/check targets
- `.gitignore` - compiled binaries, `.env`/`.env.local`; `internal/store/sqlc/` deliberately kept
- `README.md` - "Run locally" section (Postgres provisioning, env vars, `make migrate && make run`)

## Decisions Made
- Left `go.mod`'s `go` directive at 1.26.4 (the installed toolchain) rather than hand-pinning to
  1.25 literally — 01-RESEARCH.md recommends building on whatever the current stable release is,
  and 1.26.4 satisfies "Go 1.25+."
- Kept `chi` and `http-swagger/v2` pinned in `go.mod` even though neither is imported by any code
  in this plan yet (they're consumed by plans 01-03/01-07). See Deviations below for why this
  required a manual fix against `go mod tidy`'s default behavior.
- Picked snake_case string values for the seed data's category/severity columns
  (`storm_cyclone_damage`, `shelter_open`, etc.) since the actual enum validation is Plan 01-03/
  01-service-layer scope, not this plan's — any later plan is free to align exact string values
  during that validation work.
- Geohash precision for seeded/test data set to 8 characters, per 01-RESEARCH.md's Open Questions
  recommendation (fine-grained now, safely truncatable later, avoids a Phase 3 backfill).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `go mod tidy` silently dropped three of the five plan-mandated pinned dependencies**
- **Found during:** Task 1 (Go module scaffold)
- **Issue:** The plan requires `go.mod` to pin all five dependencies (chi, pgx, geohash, goose,
  http-swagger/v2) at exact versions. After `go get`-ing all five and running `go mod tidy` (to
  clean up after adding `cmd/migrate`'s real imports), Go's tooling completely removed
  `go-chi/chi/v5`, `mmcloughlin/geohash`, and `swaggo/http-swagger/v2` from both `go.mod` and
  `go.sum` — because nothing in the module imported them yet, and unlike `pgx`/`goose` (used
  directly by `cmd/migrate`), they have no transitive path keeping them referenced either. This
  is standard, expected Go module behavior, not a bug in the tooling — but it conflicted with the
  plan's explicit acceptance criterion and with the T-01-SC threat mitigation (pin exact versions
  now so a later plan's `go get`/`go mod tidy` can't silently resolve a different, unaudited
  version when the import finally lands).
- **Fix:** Re-ran `go get` for the three affected packages at their exact pinned versions, then
  did **not** run `go mod tidy` again for the rest of this plan (Task 2 added a real `geohash`
  import, so I manually moved that one require line into the direct block in `go.mod` rather than
  running `tidy`, which would have stripped `chi`/`http-swagger/v2` a second time). `chi` and
  `http-swagger/v2` remain in `go.mod`'s indirect-require block, version-pinned and present in
  `go.sum`, ready for plans 01-03/01-07 to import without any resolution ambiguity.
- **Files modified:** `go.mod`, `go.sum`
- **Verification:** `go build ./...`, `go vet ./...`, and the full test suite all pass; `grep`
  confirms `github.com/go-chi/chi/v5 v5.3.2` and `github.com/swaggo/http-swagger/v2 v2.0.2` are
  present in `go.mod` at the exact pinned versions.
- **Committed in:** `fb045df` (Task 1), `b999934` (Task 2, geohash promotion to direct)

---

**Total deviations:** 1 auto-fixed (1 blocking — Go tooling vs. plan acceptance criterion)
**Impact on plan:** No scope creep; resolves a genuine tension between Go's module-pruning
behavior and the plan's supply-chain-pinning requirement without inventing any placeholder
application code to force the imports early.

## Issues Encountered
- Local Homebrew Postgres 16.14 was already running; created a `pinalert_test` database locally
  to exercise every `<automated>` verify command that needs `DATABASE_URL` (all store-layer
  tests, plus the `go run ./cmd/migrate` idempotency check) before committing each task. No
  problem — this is exactly the fallback 01-RESEARCH.md's Environment Availability table expected.

## User Setup Required

None required to execute this plan — local Homebrew Postgres was sufficient. Provisioning a
persistent Neon or Supabase database (per the plan's `user_setup` block and PROJECT.md's Key
Decision) is still needed before any live deploy, but is not a gate for Phase 1 plans, which only
require a documented local full-stack run.

## Next Phase Readiness
- `store.MigrationsFS`, `internal/testutil.NewTestDB`/`SeedReports`/`SeedExpiringReport`, and the
  `reports`/`sessions` schema are all ready for plan 01-03's `NearbyReports`/`InsertReport` sqlc
  queries and the FOUND-03 index-scan / FOUND-05 expiry-predicate tests named in
  01-VALIDATION.md's Per-Task Verification Map.
- `chi` and `http-swagger/v2` are pinned and ready to import without any version-resolution step
  once plans 01-03 (router) and 01-07 (Swagger UI) write the code that uses them.
- No blockers. The only outstanding item from PROJECT.md's Blockers/Concerns (confirmer
  location-capture method, geohash cell-size precision) is explicitly out of Phase 1 scope per
  01-SKELETON.md and remains correctly deferred to Phase 3.

---
*Phase: 01-foundation-report-map*
*Completed: 2026-09-06*
