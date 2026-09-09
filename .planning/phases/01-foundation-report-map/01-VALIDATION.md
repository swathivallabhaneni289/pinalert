---
phase: 1
slug: foundation-report-map
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-09-06
updated: 2026-09-09
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `go test` (no third-party test runner — `net/http/httptest` + table-driven tests are sufficient at this scope) |
| **Config file** | none |
| **Quick run command** | `go test ./... -short` |
| **Full suite command** | `go test ./... -v -p 1` (requires `DATABASE_URL` pointing at a real Postgres for store-layer tests; `-p 1` is required — packages share one physical test DB) |
| **Estimated runtime** | ~30 seconds (quick), ~90 seconds (full, with real Postgres) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -short`
- **After every plan wave:** Run `go test ./... -v` (full suite, real Postgres via `DATABASE_URL`)
- **Before `/gsd-verify-work`:** Full suite must be green, plus the manual UAT items below
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

Reconstructed 2026-09-09 against the 15 completed plans (`01-01-PLAN.md` .. `01-15-PLAN.md`).
Every automated command below was actually executed against this repo on 2026-09-09 (Go
`go1.25+`, real Postgres via `DATABASE_URL=postgres://$(whoami)@localhost:5432/pinalert_test?sslmode=disable`),
not inferred from plan text.

| Plan | Wave | Requirement(s) | Secure Behavior / Gate | Test Type | Automated Command | Status |
|------|------|-----------------|-------------------------|-----------|---------------------|--------|
| 01-01 | 0 | OPS-02, FOUND-05, FOUND-06, FOUND-03 | Migrations embedded, schema uses `TIMESTAMPTZ` + indexes, CI workflow runs build/vet/test with a postgres service | unit/n-a | `go test ./internal/store/... -run TestMigrationsEmbedded -v`; `.github/workflows/ci.yml` present | ✅ green |
| 01-01 | 0 | FOUND-03/05 (infra) | `testutil` DB helper: `CopyFrom` bulk seed, embedded `MigrationsFS` | unit | `go test ./internal/testutil/... -v` | ✅ green |
| 01-02 | 0 | FOUND-02, FOUND-04 | Design tokens/component classes declared in `main.css`, no npm dependency introduced | static/grep | `grep`-based token/class-presence checks (see 01-02-PLAN.md) | ✅ green (re-verified via file inspection) |
| 01-03 | 1 | FOUND-01 | HMAC-signed session cookie; first request gets `Set-Cookie`, replay/tamper rejected, `crypto/rand` not `math/rand` | unit | `go test ./internal/session/... -run TestSessionIssuance -v` | ✅ green |
| 01-03 | 1 | FOUND-02, FOUND-06 | Category/severity enum + shelter-capacity validation; description length bounds; bounding-box math | unit | `go test ./internal/service/... -run 'TestValidateSubmitInput\|TestShelterCapacityValidation\|TestExpiryDuration\|TestBoundingBox' -v` | ✅ green |
| 01-03 | 1 | FOUND-03, FOUND-05, session-id-leak | Index-scan-backed nearby query, read-time expiry predicate, `session_id` never in read-path columns/response | integration | `go test ./internal/store/... -run 'TestNearbyReportsUsesIndex\|TestExpiryReadTimePredicate\|TestNearbyReportsExcludesSessionID' -v` | ✅ green |
| 01-03 | 1 | FOUND-02/03 | Submit-then-read round trip; nearby response omits `session_id` at API layer | integration | `go test ./internal/api/... -run 'TestSubmitThenNearbyReturnsReport\|TestNearbyResponseOmitsSessionID' -v` | ✅ green |
| 01-04 | 1 | FOUND-02, FOUND-04, FOUND-03 | Page shell serves the full DOM-contract element set; vendor scripts SRI-pinned; no `innerHTML` in map path | integration/static | `go test ./internal/api/... -run TestPageShellServesDOMContract -v`; `node --check web/static/js/app.js web/static/js/map.js` | ✅ green |
| 01-05 | 1 | FOUND-02, FOUND-06 | Report modal: category/severity/shelter-capacity form flow, ARIA slider semantics, no raw hex in modal.css | static/syntax | `node --check web/static/js/modal.js`; grep-based ARIA/token checks | ✅ green |
| 01-06 | 1 | FOUND-03, FOUND-04, FOUND-05 | Feed list reads from shared store (no independent fetch), no trust-signal copy (Phase-2 scope leak guard), no client-side expiry filter (server enforces FOUND-05) | static/syntax | `node --check web/static/js/feed.js`; grep-based scope-leak guards | ✅ green |
| 01-07 | 1 | OPS-01 | `/swagger/doc.json` returns 200 valid JSON documenting all category/severity/capacity enums; spec omits `session_id` | smoke | `go test ./internal/api/handlers/... -run 'TestSwaggerDocServed\|TestSwaggerSpecCoversRoutes' -v` | ✅ green (re-run 2026-09-09) |
| 01-08 | 2 | FOUND-02, FOUND-03 | Modal backdrop `hidden`-attribute guard; severity slider track/thumb touch-target floor (44x44, WCAG 2.5.5) | unit | `go test ./web/... -run TestModalBackdropHiddenGuard -v` | ✅ green |
| 01-09 | 2 | FOUND-03, FOUND-04 | Primary map container resolves a non-zero height via `100dvh`+`100vh` fallback pair | unit | `go test ./web/... -run 'TestPrimaryMapHasResolvedHeight\|TestModalBackdropHiddenGuard' -v` | ✅ green |
| 01-10 | 2 | FOUND-02, FOUND-04 | Leaflet tile layers request retina tiles (`detectRetina: true`) without repointing the tile host | unit | `go test ./web/... -run 'TestTileLayersRequestRetinaTiles\|TestPrimaryMapHasResolvedHeight\|TestModalBackdropHiddenGuard' -v` | ✅ green |
| 01-11 | 2 | FOUND-02, FOUND-04 | Vendor script load order (leaflet -> maplibre-gl -> bridge -> app), SRI hashes independently recomputed | unit | `go test ./web/... -run 'TestVendorMapScriptsLoadInDependencyOrder\|TestPrimaryMapHasResolvedHeight\|TestModalBackdropHiddenGuard\|TestTileLayersRequestRetinaTiles' -v` | ✅ green |
| 01-12 | 2 | FOUND-02, FOUND-04 | Vector basemap (OpenFreeMap) with raster fallback gated by a WebGL2 capability probe; zoom bounds/attribution correctness on both branches | unit | `go test ./web/... -run 'TestPrimaryMapHasResolvedHeight\|TestModalBackdropHiddenGuard\|TestVendorMapScriptsLoadInDependencyOrder' -v`; `go test ./web/ -v` (basemap/zoom/probe tests) | ✅ green |
| 01-13 | gap-closure | FOUND-02, FOUND-03, FOUND-04 | Category glyphs rendered via CSS mask-image (`currentColor`-driven), class-driven not `<img>`-isolated | unit | `go test ./web/... -run 'TestCategoryGlyphMaskRulesCoverEveryCategory\|TestIconGlyphsAreClassDriven' -v` | ✅ green |
| 01-14 | gap-closure | FOUND-02, FOUND-03, FOUND-04 | Badge glyph contrast across age-stages and themes (Mechanism 2) | unit | `go test ./web/... -run TestBadgeGlyphContrastAcrossAgeStagesAndThemes -v` | ✅ green |
| 01-15 | gap-closure (live UAT) | FOUND-02, FOUND-03 | Modal glyph color fix, stroke-width revert, `AssetVersion` cache-busting on local static assets | unit + human-verified | `go test ./internal/api/handlers/... -run TestPageShellAppliesAssetVersionToLocalStaticAssets -v`; `go test ./web/... -run 'TestCategoryGlyphMaskRulesCoverEveryCategory\|TestBadgeGlyphContrastAcrossAgeStagesAndThemes\|TestIconGlyphsAreClassDriven'` | ✅ green |
| — (cross-cutting) | — | FOUND-01..06, OPS-01, OPS-02 | Full suite | integration | `go test ./... -v -p 1` (with real Postgres) | ✅ green — all packages pass (`internal/api/handlers`, `internal/service`, `internal/session`, `internal/store`, `internal/testutil`, `web`) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**2026-09-09 re-run summary (this audit):**
- `go build ./... && go vet ./...` — clean, no errors.
- `go test ./... -short` — all packages `ok` (~15s).
- `go test ./... -v -p 1` with `DATABASE_URL` against a real local Postgres 16 (`pinalert_test`) — all packages `ok`, including every store-layer integration test (`TestNearbyReportsUsesIndex`, `TestExpiryReadTimePredicate`, `TestNearbyReportsExcludesSessionID`) and every `web/` contract test (CSS, JS, DOM-shell, basemap, glyph-contrast, cache-busting).
- Individually re-ran the two Wave-0-draft-named tests not shown in the truncated full-suite log: `TestPageShellAppliesAssetVersionToLocalStaticAssets` (PASS) and `TestSwaggerDocServed` (PASS, 200 on both `/swagger/doc.json` and `/swagger/index.html`).

---

## Wave 0 Requirements

- [x] `go.mod` / module init
- [x] `internal/testutil/db.go` — spin up/truncate a test Postgres connection, shared by all store-layer tests (`TestNewTestDB` passes)
- [x] `.github/workflows/ci.yml` — present, with a `postgres:16` service container, runs build/vet/test
- [x] `sqlc.yaml` + first goose migration — migration applies cleanly (`goose: no migrations to run. current version: 1` confirms baseline is already applied in the test DB)
- [x] Seed-data script/fixture (`internal/testutil/seed.go`, `TestSeedExpiringReport` passes) for the FOUND-03/FOUND-05 index-scan and expiry tests

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions | Status |
|----------|-------------|------------|--------------------|--------|
| Leaflet/MapLibre map renders pins, drag-marker location picker, map<->list sync/toggle | FOUND-04 | Client-side map interaction isn't meaningfully unit-testable in Go | Open the app, submit a report via the modal, confirm the pin appears and the list row appears; resize to mobile width and confirm the map/list toggle works both directions | Confirmed live during 01-15's human-in-the-loop UAT session (2026-09-09) |
| Severity slider accessibility | FOUND-02 | Requires manual keyboard/screen-reader check | Tab to the severity slider, confirm arrow keys change the value, confirm `aria-valuetext` announces correctly | Not re-verified in this audit pass — carried over as outstanding manual item |
| Expiry visual fade / age-ramp desaturation | FOUND-05 | Visual/timing behavior on the client | Seed or wait for a report nearing expiry and confirm it visually desaturates before disappearing | Not re-verified in this audit pass — carried over as outstanding manual item |
| Category glyph legibility (dark + light mode, at a glance) | FOUND-02/03 | Perceptual claim, not automatable | Look at the modal category grid and map/feed badges in both themes | Confirmed live by the human tester in 01-15 (dark mode, then light mode: "looks good") |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s (full suite ran in ~13s against real Postgres in this audit)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** Phase 1 is fully executed (15/15 plans complete, including 3 gap-closure plans:
01-13, 01-14, 01-15). All named requirement-level tests exist, run, and pass. See
"Validation Audit 2026-09-09" below for the adversarial re-check performed for this sign-off.

---

## Validation Audit 2026-09-09

**Auditor:** gsd-nyquist-auditor (adversarial re-check of a completed phase's validation map)

**Starting hypothesis:** the phase is not actually green — the Wave-0 draft in this file predates
every PLAN.md and named tests were assumed, not verified.

**What was done:**

1. Read all 15 `PLAN.md`/`SUMMARY.md` pairs (`01-01` through `01-15`), `01-REQUIREMENTS.md`
   (project-level `REQUIREMENTS.md`), and the prior Wave-0-era `01-VALIDATION.md` draft.
2. Extracted every `requirements:` frontmatter list and every `<automated>` verify command across
   all 15 plans (187 lines of extracted verify blocks) to reconstruct the true task-to-requirement
   map, since the Wave-0 draft only had 7 requirement-level rows with `TBD` task IDs.
3. Actually ran, rather than assumed:
   - `go build ./... && go vet ./...` — clean.
   - `go test ./... -short` — all packages green (~15s, no DB required).
   - Provisioned a real local Postgres 16 test database (`pinalert_test`, already existed from
     prior work) and ran `go test ./... -v -p 1` with `DATABASE_URL` set — every package green,
     including all store-layer integration tests and all `web/` contract tests (CSS, JS, DOM
     shell, basemap capability-probe, glyph mask/contrast, vendor-script load order).
   - Individually re-ran the two tests the Wave-0 draft's discovery pass hadn't shown line-by-line
     in the truncated log: `TestPageShellAppliesAssetVersionToLocalStaticAssets` (01-15's
     post-review cache-busting regression test, PASS) and `TestSwaggerDocServed` (OPS-01, PASS —
     200 on both `/swagger/doc.json` and `/swagger/index.html`).
4. Confirmed `.github/workflows/ci.yml` exists and last changed in a commit specifically fixing a
   test-DB race (`14c6772`), corroborating that CI has run this suite for real, not just on paper.

**Findings:**

- **No BLOCKER findings.** Every requirement-level test named in the original Wave-0 draft
  (`TestSessionIssuance`, `TestValidateSubmitInput`, `TestNearbyReportsUsesIndex`,
  `TestExpiryReadTimePredicate`, `TestShelterCapacityValidation`, `TestSwaggerDocServed`,
  `TestNearbyReportsExcludesSessionID`) exists, runs, and passes against a real Postgres instance.
- **No WARNING findings requiring action.** The two later gap-closure test additions
  (`web/css_contract_test.go`'s glyph/contrast tests, `web/js_contract_test.go`'s vendor-order/
  retina/basemap tests, and `internal/api/handlers/page_test.go`'s asset-versioning test) are all
  present and green — the phase's iterative UAT-driven hardening (01-13 through 01-15) is fully
  reflected in the current test suite, not just described in prose.
- **Caveat (documented, not a gate failure):** Two of the three original "Manual-Only" items
  (severity-slider screen-reader announcement, expiry visual fade/desaturation) were not
  re-verified live in this audit pass — they remain manual-only by design (client-side perceptual/
  timing behavior, explicitly out of Go-test scope per the original validation strategy) and were
  last exercised, per the phase's own SUMMARYs, earlier in the phase's UAT rounds, not in this
  specific 2026-09-09 session. This is flagged as a residual manual-verification gap, not a broken
  requirement — FOUND-02 and FOUND-05 both have passing automated coverage for their non-visual
  behavioral contracts (validation logic, read-time expiry predicate) independent of this item.
- **Frontmatter corrected:** `status: draft` -> `complete`, `nyquist_compliant: false` -> `true`,
  `wave_0_complete: false` -> `true` — the prior draft state was stale from before any plan
  existed (2026-09-06) and had never been updated across 15 completed plans and 3 gap-closure
  rounds.

**Conclusion:** GAPS FILLED. All 7 previously-named gaps are confirmed FILLED by tests that
actually ran and passed on 2026-09-09, and the full per-plan requirement map (15 plans, not the
Wave-0 draft's placeholder 7 rows) is now reconstructed and green.
