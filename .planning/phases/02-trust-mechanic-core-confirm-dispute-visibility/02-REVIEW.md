---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
reviewed: 2026-09-18T08:56:15Z
depth: standard
files_reviewed: 55
files_reviewed_list:
  - .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md
  - cmd/server/main.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
  - internal/api/handlers/activity_e2e_test.go
  - internal/api/handlers/auth.go
  - internal/api/handlers/feed_visibility_e2e_test.go
  - internal/api/handlers/reports.go
  - internal/api/handlers/reports_e2e_test.go
  - internal/api/handlers/swagger_test.go
  - internal/api/handlers/votes.go
  - internal/api/handlers/votes_e2e_test.go
  - internal/api/router.go
  - internal/service/auth.go
  - internal/service/auth_test.go
  - internal/service/feed.go
  - internal/service/feed_test.go
  - internal/service/report.go
  - internal/service/trust.go
  - internal/service/trust_test.go
  - internal/service/visibility.go
  - internal/service/visibility_test.go
  - internal/store/migrations/00004_create_votes.sql
  - internal/store/queries/reports.sql
  - internal/store/queries/votes.sql
  - internal/store/reports_test.go
  - internal/store/sqlc/models.go
  - internal/store/sqlc/reports.sql.go
  - internal/store/sqlc/votes.sql.go
  - internal/store/votes_test.go
  - internal/testutil/db.go
  - web/account_menu_contract_test.go
  - web/feed_freshness_contract_test.go
  - web/profile_nav_contract_test.go
  - web/static/css/auth.css
  - web/static/css/main.css
  - web/static/css/modal.css
  - web/static/css/trust.css
  - web/static/js/activity.js
  - web/static/js/app.js
  - web/static/js/feed.js
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/static/js/theme.js
  - web/static/js/visibility.js
  - web/static/js/votes.js
  - web/template_contract_test.go
  - web/templates/account_header.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/login_gate.html.tmpl
  - web/templates/profile.html.tmpl
  - web/templates/verify_outcome.html.tmpl
  - web/theme_contract_test.go
  - web/votes_contract_test.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-09-18T08:56:15Z
**Depth:** standard
**Files Reviewed:** 55
**Status:** issues_found

## Summary

This phase implements the confirm/dispute/resolve/reopen trust mechanic: a single
`Resolve` function (`internal/service/visibility.go`) that is the sole authority for a
report's visibility, fed by `BuildVoteTally` (`internal/service/trust.go`), exposed
through four vote endpoints (`internal/api/handlers/votes.go`) and a read path
(`GET /api/reports`, `GET /profile`) that both defer to the same resolver. The Go-side
architecture is unusually disciplined: `Resolve` is pure and exhaustively table-tested
(including a ~27k-combination totality/determinism matrix), the append-only `votes`
table is proven lock-free and race-safe against a real Postgres, session-id/account-id
leak controls are asserted by reflection and e2e string-search, and the client JS
(`votes.js`, `visibility.js`) is covered by static contract tests for the XSS-sink
discipline, the D-18 "no GPS fallback for voting" rule, and CSS specificity traps.

The one place this discipline does not reach is the thing the phase's stated Core
Value is actually about: whether the confirm/dispute mechanic is "resistant to trivial
gaming." As implemented, the independence predicate can be satisfied by a small,
easily scripted number of freshly-verified email accounts supplying self-reported,
unverified coordinates, with no rate limiting on the vote-casting endpoints themselves
to raise the cost of doing so. That is this review's one Critical finding; the
remaining findings are narrower robustness and code-quality items.

## Critical Issues

### CR-01: The independence predicate is trivially gameable — no proof-of-location, no per-account/per-session throttling on vote endpoints

**File:** `internal/service/trust.go:290-363` (`VotingService.CastVote`), `internal/api/router.go:161-203` (gated route group), `internal/api/handlers/votes.go:103-184` (`CastVote` handler)

**Issue:**
`CastVote` computes the voter's geohash cell directly from `CastVoteInput.Latitude`/`Longitude` (trust.go:337), which is copied verbatim from the client-supplied JSON body (`votes.go:136-137`, `web/static/js/votes.js` `castVote()`). There is no server-side check that the coordinates are plausible for the caller (no IP-geolocation cross-check, no device attestation, no proof the browser's own geolocation API — rather than a hand-crafted `fetch` call — produced the value). `IndependentAgreementThreshold` is 2 (`visibility.go:45`), so exactly **two** accounts voting from two fabricated coordinate pairs ≥153m apart (`voterGeohashPrecision = 7`, `trust.go:31`) are sufficient to:
- flip any report to `live`/`confirmed` (two fake confirms),
- retract any report (two fake resolves, or one if the attacker is also the reporter — `D-13` instant path),
- hide any report behind a dispute (two fake disputes, since `DisputeCells > ConfirmCells` is satisfied at 2-vs-0).

The only friction on minting a new "independent" voter is `POST /api/auth/request-link`: email-only verification (no CAPTCHA, no phone/SMS), a 45s per-address cooldown, and a per-IP token bucket of burst 5 + 1 refill/60s (`router.go:154-159`, `cmd/server/main.go:37-47`). That budget alone permits roughly one new verified account per minute per source IP (~60/hour), far more than the 2 accounts needed to control any single report's state, and is trivially multiplied with disposable-inbox services or multiple egress IPs — neither of which requires any sophistication.

Compounding this, **none of the four vote endpoints (`/confirm`, `/dispute`, `/resolve`, `/reopen`) or `POST /api/reports` carry any rate limiting at all** — `router.go`'s gated `r.Group` (lines 163-203) applies only `requireVerifiedAccount`; the per-IP limiter (`ratelimit.NewPerIP`) is wired exclusively to `/api/auth/request-link` (line 158-159). Once an attacker holds two verified sessions, they can cast unlimited confirm/dispute/resolve/reopen requests per second against any report id (ids are sequential and guessable) with no throttle at all.

Taken together: the mechanic whose entire stated purpose is to let a reader trust "confirmed by N independent nearby confirmations" can be fully manufactured, for any report, by an unauthenticated script that only needs to (a) receive two emails and click two links, and (b) send two POST bodies with fabricated lat/lon. Nothing in this phase raises that cost above "trivial."

This is flagged as Critical because it is exactly the property `CLAUDE.md` names as this project's Core Value ("the confirm/dispute trust mechanic ... must work correctly and be resistant to trivial gaming"), and the code as shipped in this phase does not meet that bar — not as an edge case, but as the default, unmitigated path for every report in the system. (The project's own longer-term plan defers weighted/diversity scoring to Phase 3 — `TRUST-05`, `TRUST-07` — which is a reasonable place to land the *full* fix; the finding here is that Phase 2 ships with *zero* mitigating friction in the meantime, not merely an "unfinished" weighting scheme.)

**Fix (incremental, does not require Phase 3's full scoring model):**
```go
// 1. Rate-limit the vote endpoints per verified account, mirroring the
//    existing per-IP limiter already wired to request-link:
requestLinkLimiter := ratelimit.NewPerIP(requestLinkLimit.Every, requestLinkLimit.Burst)
voteLimiter := ratelimit.NewPerAccount(5*time.Second, 3) // e.g. burst 3, 1/5s refill

r.Group(func(r chi.Router) {
    r.Use(requireVerifiedAccount(deps.Sessions))
    r.With(voteLimiter.Middleware()).Post("/api/reports/{id}/confirm", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteConfirm))
    // ...same for dispute/resolve/reopen
})

// 2. Raise the cost of minting a new "independent" voter: add a CAPTCHA
//    (e.g. free Cloudflare Turnstile) to POST /api/auth/request-link, and/or
//    tighten the per-IP budget materially below 60/hour.

// 3. Treat a voter's geohash cell as a weaker signal than the current binary
//    "counts as 1" model: e.g. discount cells from accounts verified in the
//    last N minutes, or require the CONFIRMED report to have survived past
//    some minimum account age — pulling a slice of Phase 3's planned
//    reliability decay (TRUST-07) forward as a cheap interim control rather
//    than shipping this phase with no control at all.
```

## Warnings

### WR-01: `POST /api/reports` (report submission) also carries no rate limiting

**File:** `internal/api/router.go:161-203`, `internal/api/handlers/reports.go:241-289`

**Issue:** Like the vote endpoints, report submission is gated only by `requireVerifiedAccount`; no per-account or per-IP limiter is applied. A single verified session can flood the feed with reports (each up to 64 KiB, `maxSubmitBodyBytes`) at line rate, which is both a resource-exhaustion vector and, since triage ordering and the empty-state UX depend on the feed being a small, meaningful list, a direct usability/DoS risk during exactly the disaster scenario this app is built for (an attacker or a malfunctioning client script drowning out real reports).

**Fix:** Apply a per-account (or per-session) token-bucket limiter to `POST /api/reports`, analogous to the existing per-IP limiter on `/api/auth/request-link`. A generous budget (e.g. 1 report every few seconds, burst 3) would not meaningfully impede a genuine reporter while blocking scripted flooding.

### WR-02: `CastVote`'s voter-location cache has no staleness or plausibility check beyond the initial GPS read

**File:** `web/static/js/votes.js:91-149` (`getVoterLocation`)

**Issue:** Per D-17, the voter's coordinates are read from the browser's geolocation API once per browser session and then cached in `sessionStorage` for every subsequent vote in that tab. This is a deliberate UX choice (avoid re-prompting), but it means a single genuine GPS read early in a session is reused indefinitely for votes cast much later and potentially from a different physical location (the voter has moved), silently feeding a stale cell into the independence predicate for the rest of the session. This is not a security bug in the adversarial sense (a legitimate voter isn't "gaming" anything), but it does mean the predicate's accuracy degrades over a session with no visible signal to the voter or the product that this has happened.

**Fix:** Consider re-validating the cached coordinate's age (e.g. re-prompt after N minutes, or on next `getCurrentPosition` opportunistically refresh the cache) rather than caching for the lifetime of the tab unconditionally. Low priority relative to CR-01/WR-01, but worth tracking alongside the Phase 3 reliability work since it affects the same independence signal.

### WR-03: Dead/unreachable outcome branches ship in the vote-response copy layer with no caller that can exercise them

**File:** `web/static/js/votes.js:404-421` (`resolutionOutcomeMessage`), `61-82` (`REOPEN_PENDING_TOAST`, `REOPEN_BUTTON_LABEL`/`_IN_FLIGHT` for the reopen half)

**Issue:** `resolutionOutcomeMessage`'s `reopen` branch and `REOPEN_PENDING_TOAST` are, by the module's own doc comments, unreachable from any UI this phase ships — the only caller of `castVote('reopen', ...)` is `activity.js`, which hard-codes its own toast and never calls `resolutionOutcomeMessage`. The code is deliberately retained "for API-consistency," which is a defensible call, but it means this file now carries logic with no test or runtime path that can ever select it (a future non-reporter reopen UI would need to be added and wired through before this branch does anything). This is a maintainability/quality note rather than a functional bug: an untested, unreachable branch is exactly the kind of code that silently rots (e.g., a future edit to the resolve-half logic could break the reopen-half symmetry with no red test to catch it, since nothing calls it).

**Fix:** Either add a unit test that calls `PinalertVotes.resolutionOutcomeMessage('reopen', ...)` directly (cheap, and it already exists as an exported function — `window.PinalertVotes.resolutionOutcomeMessage`) so the symmetry is at least pinned by a test even though no UI reaches it yet, or drop the unreachable branch until a non-reporter-facing reopen surface actually ships.

## Info

### IN-01: `activity.js` never releases `reopenBlocks` map entries after a successful reopen

**File:** `web/static/js/activity.js:26-30, 145-159`

**Issue:** `onReopenClick`'s success path removes the `.vote-controls`/`.vote-error` DOM nodes (`block.controls.remove()`) once the report is no longer retracted, but never deletes the corresponding entry from the module-level `reopenBlocks` map (`reopenBlocks[row.dataset.reportId]`). The stale reference is harmless in practice (the delegated click listener re-checks `row.dataset.reportId` and the button that would trigger it is gone from the DOM), but it's an avoidable minor leak/inconsistency between the map's keys and what's actually still interactive.

**Fix:**
```js
if (updated.visibility !== PinalertVotes.RETRACTED_VISIBILITY_SLUG) {
  block.controls.remove();
  block.error.remove();
  delete reopenBlocks[row.dataset.reportId];
}
```

### IN-02: `newSwaggerTestRouter`'s `api.Deps` omits `Votes`, which is fine today but relies on an implicit "never invoked" contract with no compile-time guard

**File:** `internal/api/handlers/swagger_test.go:66-72`

**Issue:** `api.Deps{Reports: service.NewReportService(nil), ...}` leaves `Votes` as its zero value (`nil *service.VotingService`). `handlers.CastVote(deps.Votes, ...)` captures that `nil` pointer in a closure at router-construction time; the test never exercises `/api/reports/{id}/confirm` etc., so this is currently safe, but it is safe only because no test in this file happens to hit those routes — there is no assertion (and none is easy to add) that would fail loudly if a future test in this file *did* accidentally drive a vote route and hit a nil-pointer dereference inside `VotingService.CastVote`. This is the same pattern `router.go`'s own doc comment calls out as intentionally permissive ("a `Deps{}` literal that omits it still compiles"), so this is a note for awareness rather than a defect to fix now.

**Fix:** No action required; flagging only so a future contributor extending `swagger_test.go` to cover the vote routes knows to also construct a real (or fake-backed) `VotingService` first.

---

_Reviewed: 2026-09-18T08:56:15Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
