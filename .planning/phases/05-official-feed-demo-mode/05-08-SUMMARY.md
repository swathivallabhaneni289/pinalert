---
phase: 05-official-feed-demo-mode
plan: 08
subsystem: official-alerts
tags: [sachet, reader-view, banner, freshness, api, lazy-refresh]
status: complete

requires:
  - phase: 05-01
    provides: internal/areas (Locate, ByCode, CellDistricts)
  - phase: 05-05
    provides: official.Refresher, OfficialAlertsInForce, feed status
provides:
  - GET /api/official/alerts (gated), the server authored reader view
  - official.View, BuildView (pure), State, FreshState and the three windows
  - official.Service (Alerts, Kick, Refresher) and official.NewService
  - service.OfficialEventWord, OfficialEventPhrase, OfficialLevelWord, OfficialEventWords
  - handlers.OfficialReader, OfficialAlerts, officialViewToResponse
affects: [05-09, 05-11, 05-12, 05-15, 05-19, 05-21]

key-files:
  created:
    - internal/service/officialwords.go
    - internal/service/officialwords_test.go
    - internal/official/fresh.go
    - internal/official/fresh_test.go
    - internal/official/view.go
    - internal/official/view_test.go
    - internal/official/service.go
    - internal/official/service_test.go
    - internal/api/handlers/official.go
    - internal/api/handlers/official_test.go
    - internal/api/handlers/official_e2e_test.go
  modified:
    - internal/api/router.go
    - internal/api/gate_test.go
    - internal/api/handlers/swagger_test.go
    - cmd/server/main.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml

key-decisions:
  - "StoredAlert is defined in view.go (plan 05-05 had no such type): plain fields, the tier, district and state codes and DistrictNames filled by the service, so BuildView stays pure and the Demo can feed it samples."
  - "Each alert lands in exactly one section, first match wins: reader, home, state, unplaced. An alert for both the reader's and the home district shows once, in the reader group."
  - "The districts line lists the reader's district, the home district when its group is drawn, then the other districts the listed district alerts name, by code. 2 names read 'A and B', 3 read 'A, B and C'."
  - "A home area in a different state than the reader's gets its own 'Across <home state>' group after the reader's; with no reader point the state group follows the home state."
  - "Boundary file state names are upper case; titleName turns them into 'Andhra Pradesh', keeping and/of lower case."
  - "SACHET is registered before the caller's options in NewService, so a test or a later plan can replace or add to it with a RefresherOption; the e2e tests register a counting fake fetcher this way and never reach NDMA."
  - "main.go passes BASE_URL as set (not the localhost fallback) to the SACHET User-Agent, the same as the globe User-Agent."

requirements-completed: [ALERT-01, ALERT-02, ALERT-03, ALERT-04]

duration: about 1h 10m
completed: 2026-10-29
---

# Phase 5 Plan 08: The reader's official alerts route Summary

A gated `GET /api/official/alerts` answers which official alerts cover the reader and whether that answer can be trusted right now, with every fact (feed state, group labels, count, districts line, level words, badge, banner pick) authored by the server, and every read quietly keeps SACHET fresh.

## Commits

- c0044d2f test(05-08): pin the closed official words, freshness and the reader view
- b560f7a3 feat(05-08): add the closed official words, freshness and the pure reader view
- 4d7c703b test(05-08): pin the official service, the route's refusals and the gate row
- eb8a28dd feat(05-08): serve the reader's official alerts and refresh SACHET lazily
- db9d0523 test(05-08): prove the official alerts route honest end to end on a database
- 431ccfa1 docs(05-08): regenerate the API docs for GET /api/official/alerts

## Response shape as built

`GET /api/official/alerts[?lat=&lon=]` (gated, `Cache-Control: no-store`, `application/json`):

```
{
  "state": "fresh" | "stale" | "unavailable",
  "age_seconds": int | null,          // server clock; null when never checked
  "last_success_at": RFC3339 | null,  // the last good check
  "area_known": bool,                 // false: no usable point and no home area
  "groups": [ {                       // [] never null; empty when unavailable or area unknown
      "kind": "reader" | "home" | "state" | "unplaced",
      "label": "Where you are: Ntr district" | "Your home area: ... district" | "Across Andhra Pradesh" | "Could not be placed on a map",
      "rows": [ {
          "id", "source": "sachet", "level": "extreme|severe|moderate|minor|unknown", "level_word",
          "badge": "Official" ("" in the Demo), "demo", "headline", "original_language_only",
          "area_text", "issuer", "issued_at", "expires_at", "expires_inferred", "grouped_count",
          "details": { "description", "instruction", "original_headline", "original_description",
                       "original_lang_label", "original_lang_code", "areas", "source_line" }
      } ] } ],
  "count": int,                       // rows, a grouped row once
  "districts_line": "Ntr district",
  "banner": { "level", "level_word", "event_phrase", "area", "more", "alert_id" } | null
}
```

Errors: 400 field error for lat without lon, a bad number or out of range; 401 from the gate; 503 "Official alerts are not available right now." when the service is nil or the read fails (never an empty list).

## What was built

- **Task 1.** `OfficialEventWord` and friends in `internal/service` (keyword match in the plan's priority, `cyclon` so "Severe Cyclonic Storm" is a Cyclone; header says feed text never becomes a phrase and carries no path of the official package). `fresh.go` has the three constants in one block and `State`. `view.go` has `BuildView`, pure, no clock, no log.
- **Task 2.** `official.Service`: `Alerts` locates the point with `areas.Default().Locate`, reads the home cell and tests the centre and four corners through `CellDistricts`, queries `OfficialAlertsInForce` with the union of codes and returns `BuildView`. `Kick` takes an atomic flag first, then reads the status, then calls `RefreshIfDue` on a 55 second detached context, all inside the goroutine. The handler, the Deps field `Official`, the route inside the gated group next to `/alerts`, the `main.go` wiring and the new row in `TestAccessGateBlocksUnverified` ("Amended 2026-10-29 by plan 05-08", every old case kept).
- **Task 3.** Five end to end tests on a real database plus `TestSwaggerOfficialAlerts`; docs regenerated with `swag init -g internal/api/router.go -o docs` (additions only, 798 lines, no removals, no long dash).

## Verification

Run on the disposable database `p5_08` (created, used, dropped, confirmed gone with `psql -lqt`):

- Full suite `go test ./... -p 1 -count=1 -v`: `--- FAIL` 0, `^FAIL` 0, panics 0, every package ok. SKIP lines: TestGeocodeLive, TestOfficialLive, TestScreeningFixture (the three allowed). 4901 PASS lines. No known flake needed a rerun.
- Named tests all PASS: TestOfficialEventWords, TestOfficialLevelWord, TestState, TestBuildViewGroupsAndOrder, TestBuildViewGroupsNearIdentical, TestBuildViewHomeGroup, TestBuildViewBannerRule, TestBuildViewStaleAndUnavailable, TestBuildViewLanguage, TestHomeAreaCorners, TestKickIsNonBlocking, TestOfficialAlertsRoute, TestOfficialAlertsHonest, TestOfficialAlertsKicksRefresh, TestOfficialAlertsNeverLogsPosition, TestAccessGateBlocksUnverified, TestSwaggerOfficialAlerts, and TestPWASkipSetMatchesRoutes (still passes; the route is not in the skip set).
- `go build ./...` and `go vet ./...` clean; `go test -race` on the new tests passes.
- Acceptance greps: view.go has no `time.Now()` or `log.` on a code line; `"Weather alert"` on one line of officialwords.go and no `internal/official` in it; `"/api/official/alerts"` once in router.go, inside the verified account group; `official.NewService(` once in main.go; `@Router /official/alerts [get]` once; no log line in the handler or service mentions lat, lon or cell; no long dash in any file written.
- Node tests were not run: the plan touches no JavaScript.

## Deviations from Plan

**1. [Rule 2 - Missing] Handler unit tests with no database.** The plan lists the handler behaviours (400 field errors, 503 when nil, headers, Kick on success) under Task 2 but names only database tests. Added `internal/api/handlers/official_test.go` with a stub reader so they run under `-short`. Not a behaviour change.

**2. [Interpretation] Extra `Across <home state>` group.** The plan's four kinds do not say what happens to a state level alert for the home state when it differs from the reader's. Instead of dropping it (D-09 says never dropped), a second `state` group follows the reader's. With no reader point the state group is the home state's.

**3. [Interpretation] BaseURL passed to NewService is `os.Getenv("BASE_URL")`, not the localhost fallback `baseURL`,** so the User-Agent NDMA sees is not "+http://localhost:8080". The call and its acceptance grep are as planned.

**4. [Process] Test and feature commits were written together, then committed test first.** The failing run (RED) was not performed separately; the test commits exist before their feature commits.

No auth gates, no checkpoints, no Rule 4 items.

## Notes for later plans

- District names in the boundary data are the official LGD spellings: 749 is "Ntr" (not "NTR"), 520 "Visakhapatanam", 515 "Spsr Nellore", 504 "Y.S.R.". Labels such as "Where you are: Ntr district" use them as they are; 05-09 and 05-15 may want a display name table if the owner dislikes them. UI-SPEC's examples ("Krishna, NTR") are samples.
- `Banner` carries no Demo flag; plan 05-19 sets the Demo tag itself (Row.Demo is set for rows).
- Plan 05-11 passes its version hook, and 05-21 registers GDACS, as `RefresherOption`s to `official.NewService`; `main.go` needs no change for them. `Service.Refresher()` is there for 05-12.
- `officialViewToResponse` is unexported in `handlers` and ready for the Demo (05-19) in the same package.
- Not applied anywhere: no migration in this plan; migration 00012 from 05-05 still needs `make migrate` on the owner's dev database after the owner says yes.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-05-40 (position never stored or logged, home cell never returned) is pinned by TestOfficialAlertsNeverLogsPosition and TestOfficialAlertsHomeArea; T-05-41 by the gate row; T-05-42 by TestBuildViewStaleAndUnavailable and TestOfficialAlertsHonest; T-05-43 by the closed phrase table and JSON only output; T-05-44 by TestKickIsNonBlocking; T-05-45 by the 400 tests.

## Self-Check: PASSED

Files exist: officialwords.go and test, fresh.go and test, view.go and test, service.go and test, handlers/official.go, official_test.go, official_e2e_test.go, and this summary. Commits c0044d2f, b560f7a3, 4d7c703b, eb8a28dd, db9d0523 and 431ccfa1 exist on worktree-agent-a371129943a672833. p5_08 dropped.
