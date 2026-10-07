---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 14
subsystem: api
tags: [trust, wire-format, openapi, swag, e2e]
requires:
  - phase: 03-07
    provides: TrustResponse, trustToResponse, newTrustE2EServer and the p07 test helpers
  - phase: 03-08
    provides: Trust.Currency (the still current signal)
  - phase: 03-12
    provides: Trust.Reporter loaded on every read path
provides:
  - trust.currency and trust.reporter on the feed and every vote response, built by one mapper
  - handlers.CurrencyResponse and handlers.ReporterResponse in the OpenAPI spec
  - TestFeedSignalsMatchCastVoteSignals and TestReporterSignalFromSeededHistory
affects: [03-15, 03-16, 03-20]
tech-stack:
  added: []
  patterns:
    - "Field by field mapper with no computation (D-21)"
    - "Swagger enums asserted element by element against the service lists"
key-files:
  created: []
  modified:
    - internal/api/handlers/trust.go
    - internal/api/handlers/trust_e2e_test.go
    - internal/api/handlers/swagger_test.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
key-decisions:
  - "The bare extensions:\"x-nullable\" tag is used, which swag 1.16.4 emits as the boolean true"
requirements-completed: [TRUST-07, D-08, D-21]
status: complete
duration: 25min
completed: 2026-10-08
---

# Phase 3 Plan 14: Both signals on the wire Summary

The feed and every vote response now carry `trust.currency` (8 keys) and `trust.reporter` (7 keys) through the one mapper `trustToResponse`, documented in OpenAPI with enums tied to the service lists, and proven equal between feed and vote response on a real Postgres.

## Tasks

| Task | Commit | What |
| ---- | ------ | ---- |
| 1 | c062695 | `CurrencyResponse`, `ReporterResponse`, `currencyToResponse`, `reporterToResponse`, `fadeStageToResponse`; regenerated docs; extended `TestSwaggerSpecCoversRoutes`; `p07TrustWireKeys` gains `currency` and `reporter` |
| 2 | a667d45 | `TestFeedSignalsMatchCastVoteSignals` and `TestReporterSignalFromSeededHistory` with the p14 helpers |

## Task 1 detail

- 03-07's `TrustResponse` type check typed every key of `p07TrustWireKeys` by elimination (the two place counts `integer`, everything else `string`). That breaks once the list holds the two object keys, so it was rewritten to name each expectation: two `integer` counts, five `string` texts and sentences, and `currency` and `reporter` checked as references with the descriptions `still current signal` and `reporter signal`. It also asserts the property count equals the key list.
- New assertions: exact key sets of both definitions, scalar types, the three enums equal to `service.CurrencyBands`, `service.FadeStages` and `service.ReporterTags` element by element, and `x-nullable` as the boolean `true` on exactly `CurrencyResponse.score`, `CurrencyResponse.fade_stage` and `ReporterResponse.score` and on no other property of any walked definition. `p07GuardedSwaggerRoots` was appended to (five roots), and the visited-roots check now loops over that list.
- `make swag` diff: purely additive. Two new definitions (`handlers.CurrencyResponse`, `handlers.ReporterResponse`), two new properties on `handlers.TrustResponse` (`currency`, `reporter`, each an `allOf` wrapper with a description), three boolean `x-nullable` keys. A fresh `swag init` into a scratch directory is byte identical to the committed `docs.go`, `swagger.json` and `swagger.yaml`.
- Long dash counts equal HEAD: swagger.json 7, docs.go 7, swagger.yaml 8, swagger_test.go 3; none in trust.go or trust_e2e_test.go.
- Gate counts: non comment `json:"` tags in trust.go = 24, `extensions:"x-nullable"` = 3.

### RED and GREEN runs

- RED for the spec test was real: after the test edits and before `make swag`, `TestSwaggerSpecCoversRoutes` failed with `docs/swagger.json is missing the guarded definition "handlers.CurrencyResponse"`.
- RED for `TestFeedTrustMatchesCastVoteTrust` on the nine key list was NOT run before the mapper: I wrote `trust.go` before editing the tests, so that failure was not captured (see Deviations).
- GREEN on p3_0314: `--- PASS` for `TestSwaggerSpecCoversRoutes`, `TestFeedTrustMatchesCastVoteTrust`, `TestTrustCountsDistinctPlacesNotVotes`, `TestTrustResponseOmitsIdentifiers`, `TestTrustCountUnderConcurrentVotes`; `ok pinalert/internal/api/handlers 1.126s`. Without a database, `TestSwaggerSpecCoversRoutes` and `TestSwaggerDocServed` pass.

## Task 2 detail

Task 2 had no RED step by design. Both pinned tests passed on their first run on p3_0314: `--- PASS: TestFeedSignalsMatchCastVoteSignals (0.06s)` and `--- PASS: TestReporterSignalFromSeededHistory (0.05s)`, with `TestFeedTrustMatchesCastVoteTrust` and `TestTrustResponseOmitsIdentifiers` still green and no `--- SKIP`.

- `TestFeedSignalsMatchCastVoteSignals`: six vote steps on a low report and a critical report, one pinned clock for both paths. Covers `not_yet_corroborated` with null score and `fresh`, `Up to date` (score 75 to 100 via `p14UpToDate`), the standing dispute case (`Needs re-confirming`, score 0, `stale`), recovery to `Up to date` at 3 places, and the critical `Too early to tell` with a null fade stage and window 360. The reporter stays `new` throughout. Every step compares the vote response with the feed read by the voter and by the reporter.
- `TestReporterSignalFromSeededHistory`: one account, two sessions, six expired reports (five corroborated) read as Reliable reporter 78, judged 6, corroborated 5, `5 of 6 earlier reports were confirmed by people nearby.`; fresh account reads New reporter; accountless report reads none; the 91 day row is not counted; RA stays `provisional` after one confirming place (D-14); the raw vote and feed bodies pass the identifier key walk and contain no email, `@`, session id, seeded cell, `account_id` or `geohash_cell`.

## Verification

- `dropdb --if-exists p3_0314; createdb p3_0314`, then `go test ./... -count=1 -p 1` with DATABASE_URL pointed at p3_0314: every package `ok` (handlers 2.891s, service 1.086s, store 7.552s, web 0.692s).
- `go test ./internal/api/handlers/ -run TestTrustCountUnderConcurrentVotes -race -count=1 -p 1` on p3_0314: `ok pinalert/internal/api/handlers 2.241s`.
- `go build ./...` and `go vet ./...` clean; `gofmt -l` empty for the three Go files this plan edits.
- p3_0314 was dropped afterwards. No other database, no port, no server was used.

## Deviations from Plan

1. **[Process] Task 1 RED order.** I wrote `trust.go` before the test edits, so the `TestFeedTrustMatchesCastVoteTrust` failure on the nine key list was not captured. The spec test RED was captured. The final GREEN state is verified.
2. **[Sandbox] Split verify commands.** The sandbox refused the long compound verify one-liners and the `rc=$?` database form. I ran the same checks as separate commands: gofmt, vet and build; the swag temporary directory diff (to the session scratchpad); the long dash counts per file; the json and x-nullable tag counts; then `dropdb --if-exists`, `createdb`, the `go test` runs with DATABASE_URL set inline, and `dropdb` as separate calls. DATABASE_URL was unset in the shell, so no `env -u` prefix was needed.
3. **[Scope]** The spec test also asserts `len(trustProps) == len(p07TrustWireKeys)`, a small addition beyond the plan.

No auth gates. No files outside `files_modified` were touched.

## Notes for later plans

- 03-15: `trustToResponse` now carries `currency` and `reporter`, so the Activity `data-trust` JSON gains both through the mapper.
- 03-20: V-42 can check the still current word and reporter tag side by side on real data.

## Known Stubs

None.

## Threat Flags

None. No new endpoints, auth paths or identifier-like property names; the new properties are counts, scores, enum words and server sentences.

## Self-Check: PASSED

- FOUND: internal/api/handlers/trust.go, trust_e2e_test.go, swagger_test.go, docs/docs.go, docs/swagger.json, docs/swagger.yaml
- FOUND commits: c062695, a667d45
