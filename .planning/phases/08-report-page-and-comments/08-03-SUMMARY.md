---
phase: 08-report-page-and-comments
plan: 03
subsystem: share
tags: [share, rate-limit, base-url, handler]
requires:
  - phase: 08-02
    provides: ShareService.EnsureLink, WithShareAreaNamer, ShareFixedLine, ShareTitle
  - phase: 08-01
    provides: TimelineNotFoundMessage (the one 404 body)
provides:
  - POST /api/reports/{id}/share (gated, per account Share bucket and network backstop)
  - handlers.ShareReport, ShareResponse, PublicShareURL, DefaultPublicBaseURL
  - RateLimits.Share, RateLimits.IPBackstopShare, RateGate.ShareMiddleware
  - service.ShareLimitMessage
  - Deps.PublicBaseURL, Deps.Shares
  - cmd/server publicBaseURL (https only outside ENV=development)
affects: [08-04, 08-06, 08-19]
tech-stack:
  added: []
  patterns: [link rooted in configuration, never in the Host header]
key-files:
  created:
    - internal/api/handlers/share.go
    - internal/api/handlers/share_e2e_test.go
  modified:
    - internal/api/handlers/ratelimit_gate.go
    - internal/api/handlers/flag_e2e_test.go
    - internal/service/ratecopy.go
    - internal/service/ratecopy_test.go
    - internal/api/router.go
    - cmd/server/main.go
    - cmd/server/main_test.go
key-decisions:
  - "Share defaults: account bucket burst 10 then one per 30 s; network backstop burst 1000 then one per 1.2 s (the flag family's shape)."
  - "The 404 body reuses the timeline's field report and TimelineNotFoundMessage, so every unavailable report reads the same."
  - "No write deadline extension: the only slow step is the area lookup, already bounded by PushAreaLookupTimeout (2 s)."
requirements-completed: [COORD-08]
status: complete
duration: about 1 hour
completed: 2026-10-29
---

# Phase 8 Plan 03: Share link route and Share bucket Summary

A signed in reader taps Share and POST /api/reports/{id}/share answers one absolute link rooted in BASE_URL, the title and the fixed line, behind a per account Share bucket wired in every place the limiter lives.

## Commits

- 67a7fa1a feat(08-03): add the Share bucket and its network backstop
- ee8e170f feat(08-03): add POST /api/reports/{id}/share and the configured base URL

## What was built

**Share bucket (task 1)**
- `RateLimits.Share` and `RateLimits.IPBackstopShare` (BucketLimit) in `internal/api/handlers/ratelimit_gate.go`, built in `NewRateGate` as `share` and `ipBackstopShare`. Constants `shareBucketName = "share"`, `shareRateField = "rate_share"`, `ipBackstopShareBucket = "ip_backstop_share"`. `ShareMiddleware` copies `FlagMiddleware`: network backstop first, then the account bucket, every attempt spends, no number typed in the file.
- `service.ShareLimitMessage(wait)`: "You've shared several reports in a short time. You can share again in {WaitPhrase(wait)}."
- `RateLimitsDefault.Share = {30 s, 10}` and `IPBackstopShare = {1200 ms, 1000}` plus the two fill lines in `resolveRateLimits` (router.go); the same numbers as `shareEvery`, `shareBurst`, `ipBackstopShareEvery`, `ipBackstopShareBurst` in the D-11 block of `cmd/server/main.go` and in `deps.RateLimits`.
- `p20Tweak` opens both new buckets (dated note, rule 9).

**Route and base URL (task 2)**
- `handlers.ShareReport(svc, base)`: account from context, `parseReportID`, `EnsureLink`; `ErrReportNotFound` answers 404 field `report` with `TimelineNotFoundMessage`; other errors log the report id only and answer 500; 200 `ShareResponse{url, title, line}` with `Cache-Control: no-store`. Reads nothing from `r.Host` or a forwarded header. Swag block written; `swag init` to a scratch directory held `/reports/{id}/share` with a `post` operation; `docs/` untouched.
- `PublicShareURL(base, token)` trims a trailing slash and appends `/r/` and the token. `DefaultPublicBaseURL = "http://localhost"`.
- `Deps.PublicBaseURL` and `Deps.Shares` in router.go; nil `Shares` builds `NewShareService(deps.Sessions, deps.Reports)` with no area namer; empty base takes the default. Route registered flat in the gated group after the timeline route: `r.With(rateGate.ShareMiddleware()).Post("/api/reports/{id}/share", ...)`.
- `publicBaseURL(env, raw, port)` in main.go: development answers raw or `http://localhost:<port>`; any other ENV errors unless raw starts with `https://` and has an address after it. `main` stops with `log.Fatalf("BASE_URL: %v")`; the message never prints the value. The old localhost fallback is gone for non development. `baseURL` (now from `publicBaseURL`) still feeds `NewAuthService` and the new `Deps.PublicBaseURL`; `official` and the globe user agent still read `os.Getenv("BASE_URL")` directly, unchanged.
- `Deps.Shares` in main.go is `NewShareService(queries, reportService, WithShareAreaNamer(geocodeClient))`, the one geocode client.

## Tests

- Without a database: `TestShareLimitMessage` (service), `TestPublicBaseURLNeedsHTTPSOutsideDevelopment` (cmd/server, 8 cases plus a check that main reads the base through `publicBaseURL(env, os.Getenv("BASE_URL"), port)`), PASS.
- With the scratch database p8_03_a: `TestShareLinkReturnsTokenURL`, `TestShareLinkIgnoresHostHeader`, `TestShareLinkWithheldIs404`, `TestShareRateLimit` (4 subtests: defaults, burst and exact wait, refused report still spends, address backstop), all PASS.
- Full run `go test ./... -p 1 -count=1` (own DATABASE_URL): every package ok; the only SKIP lines are TestGeocodeLive, TestOfficialLive, TestScreeningFixture; zero FAIL. `go build ./...` and `go vet ./...` clean.

## Deviations from Plan

None to the plan's design. Notes:
- Swagger check: the scratch output directory was made inside the worktree (`p803-swag-scratch`) and deleted, because the project rule forbids creating files outside the worktree; nothing from it was committed.
- The expired-report case in `TestShareLinkWithheldIs404` asserts a report past `expires_at` but inside the history window still gets a link (the service's Focus allows it); only held, removed and unknown reports 404.
- `gofmt` realigned the `RateGate` struct fields and the const blocks in the files this plan touched; no other formatting change.

## Notes for later plans

- 08-04: use `handlers.PublicShareURL(base, token)` with the same configured base for og:url. The router resolves the base as `deps.PublicBaseURL` or `handlers.DefaultPublicBaseURL`; the public page handler needs both `shares` and `shareBase` (local variables in `NewRouter`).
- 08-06: a 429 from the Share route has field `rate_share` and the `ShareLimitMessage` sentence, with `Retry-After` and `retry_after_seconds`; a 404 has field `report`.
- 08-19: regenerate `docs/` so the Share route appears.
- Go's `http.Client` looks cookies up under `req.Host`, so a test that forges a Host header must attach the session cookie by hand (see `p803Share`).
- The new route is gated, so `TestAccessGateExemptRoutes` needed no new row.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-08-10 to T-08-13, T-08-86), all mitigated as written.

## Self-Check: PASSED

- internal/api/handlers/share.go and share_e2e_test.go exist; commits 67a7fa1a and ee8e170f exist on the worktree branch.
- No U+2013 or U+2014 in this plan's added lines; scratch database p8_03_a dropped.
