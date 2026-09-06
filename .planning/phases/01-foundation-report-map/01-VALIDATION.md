---
phase: 1
slug: foundation-report-map
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-09-06
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `go test` (no third-party test runner — `net/http/httptest` + table-driven tests are sufficient at this scope) |
| **Config file** | none — Wave 0 installs |
| **Quick run command** | `go test ./... -short` |
| **Full suite command** | `go test ./... -v -p 1` (requires `DATABASE_URL` pointing at a real Postgres for store-layer tests; `-p 1` is required — packages share one physical test DB and each calls `testutil.NewTestDB`'s `TRUNCATE ... RESTART IDENTITY` at test start, so concurrent package execution lets one package's truncate delete/recycle another's in-flight rows, discovered as a real flake in Wave 1 post-merge testing) |
| **Estimated runtime** | ~30 seconds (quick), ~90 seconds (full, with real Postgres) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -short`
- **After every plan wave:** Run `go test ./... -v` (full suite, real Postgres via `DATABASE_URL`)
- **Before `/gsd-verify-work`:** Full suite must be green, plus the manual UAT items below
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

Task IDs are finalized when `/gsd-plan-phase` creates PLAN.md; the requirement-level mapping
below (from `01-RESEARCH.md`'s Phase Requirements → Test Map) is authoritative until then.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | — | — | FOUND-01 | Session forgery/replay | HMAC-signed cookie; first request with no cookie gets `Set-Cookie`, subsequent request reuses same session id | unit/integration | `go test ./internal/session/... -run TestSessionIssuance -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | FOUND-02 | Input validation (V5) | Rejects invalid category/severity enum values; accepts valid combinations | unit | `go test ./internal/service/... -run TestValidateSubmitInput -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | FOUND-03 | — | `EXPLAIN ANALYZE` on the nearby-reports query shows an index scan, not `Seq Scan`, against a seeded large dataset | integration | `go test ./internal/store/... -run TestNearbyReportsUsesIndex -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | FOUND-04 | — | (Leaflet rendering is client-side; no automated Go test) | manual-only | — | — | ⬜ pending |
| TBD | — | — | FOUND-05 | — | Report with `expires_at` ~2s in the future disappears from `GET /api/reports` after that instant | integration | `go test ./internal/store/... -run TestExpiryReadTimePredicate -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | FOUND-06 | — | `shelter_capacity_status` required/validated when category=shelter_open; rejected otherwise | unit | `go test ./internal/service/... -run TestShelterCapacityValidation -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | OPS-01 | — | `GET /swagger/doc.json` returns 200 with valid JSON | smoke | `go test ./internal/api/... -run TestSwaggerDocServed -v` | ❌ W0 | ⬜ pending |
| TBD | — | — | OPS-02 | — | CI runs test/vet/build on every push | n/a (CI config) | `.github/workflows/ci.yml` present and green | ❌ W0 | ⬜ pending |
| TBD | — | — | Report reads (all) | Info Disclosure — `session_id` leak | `GET /api/reports` response never includes `session_id`; queries enumerate columns explicitly, never `SELECT *` | unit/integration | `go test ./internal/store/... -run TestNearbyReportsExcludesSessionID -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `go.mod` / module init — nothing exists yet (greenfield)
- [ ] `internal/testutil/db.go` — spin up/truncate a test Postgres connection, shared by all
      store-layer tests (FOUND-03, FOUND-05, session_id-leak test)
- [ ] `.github/workflows/ci.yml` — with a `postgres:16` service container
- [ ] `sqlc.yaml` + first goose migration — needed before any store-layer test can run
- [ ] Seed-data script/fixture for the FOUND-03 large-dataset index-scan test (a few thousand
      fake rows spread across a wide lat/lon range)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|--------------------|
| Leaflet map renders pins, drag-marker location picker works, map↔list sync/toggle (D-06/D-07) | FOUND-04 | Client-side map interaction isn't meaningfully unit-testable in Go; no JS test framework in scope for this phase | Open the app, submit a report via the modal (drag the GPS-prefilled marker), confirm the pin appears on the map and the corresponding row appears in the list; resize to mobile width and confirm the map/list toggle works both directions |
| Severity slider accessibility (D-05, UI-SPEC) | FOUND-02 | Requires manual keyboard/screen-reader check, not automatable in this phase's test scope | Tab to the severity slider, confirm arrow keys change the value, confirm the numeric+label readout updates, confirm a screen reader announces the value via `aria-valuetext` |
| Expiry visual fade / age-ramp desaturation (D-15, D-17) | FOUND-05 | Visual/timing behavior on the client, not a backend-testable assertion | Seed or wait for a report nearing expiry and confirm it visually desaturates before disappearing, matching the theme reference's age-ramp |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending — finalize task IDs once PLAN.md exists
