---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
reviewed: 2026-09-16T00:00:00Z
depth: standard
files_reviewed: 45
files_reviewed_list:
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
  - web/static/css/main.css
  - web/static/css/modal.css
  - web/static/css/trust.css
  - web/static/js/activity.js
  - web/static/js/app.js
  - web/static/js/feed.js
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/static/js/visibility.js
  - web/static/js/votes.js
  - web/template_contract_test.go
  - web/templates/index.html.tmpl
  - web/templates/profile.html.tmpl
  - web/votes_contract_test.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 2: Code Review Report

**Reviewed:** 2026-09-16
**Depth:** standard
**Files Reviewed:** 45
**Status:** issues_found

## Summary

This phase implements the confirm/dispute/resolve/reopen trust mechanic: the shared
`service.Resolve` visibility resolver (`internal/service/visibility.go`), the independence
predicate and vote tally builder (`internal/service/trust.go`), the vote-casting HTTP surface
(`internal/api/handlers/votes.go`), and the three read paths that consume the resolver
(`ReportService.Nearby` in `internal/service/report.go:450`, `AuthService.ActivityForAccount` in
`internal/service/auth.go:378`, and `VotingService.CastVote` in `internal/service/trust.go:361`).

**The core trust logic holds up under adversarial reading.** I traced the single-resolver-
everywhere invariant (TRUST-02/T-02-04) across all three read/write paths and found no
divergent visibility computation anywhere — no read path re-derives visibility in SQL, in a
handler, or in JavaScript; every one of them calls `service.Resolve` with a tally built by the
one `BuildVoteTally` function. I traced the independence predicate (distinct account, guaranteed
by `CurrentVotesForReports`' `DISTINCT ON (report_id, account_id, kind)`, combined with distinct
geohash cell via `independentCellCount`) through `Resolve`'s five-rung ladder, the D-13/D-16
reporter-instant resolve/reopen symmetry, and the D-08 non-latching reversibility property, and
confirmed the implementation matches every documented decision (D-05 through D-16) with no
off-by-one or ordering defect. The `votes` table is deliberately unique-key-free and the append-
only design is exercised by a real concurrent-write test (`TestCastVoteConcurrentSameAccountKeepsEveryRow`)
that would fail under an upsert reintroduction — a stronger-than-typical proof for TRUST-09.
`internal/service/visibility_test.go`'s `TestResolve_IsTotalAndDeterministic` exhaustively checks
~27.6k tally combinations for totality, determinism, and the exact retraction predicate, which
gives high confidence the resolver itself is correct. The D-16 reporter-instant-reopen path is
enforced server-side only (`BuildVoteTally`'s identity check against `ReportVoteContext`'s
`reporter_account_id`); `web/static/js/activity.js` renders the reopen control from a
server-rendered `data-can-reopen` hook alone and asserts no client-side identity or threshold
logic, matching the server.

I found no BLOCKER-level defects. One WARNING is worth fixing before this ships more broadly:
the vote-casting endpoints carry no rate or velocity limiting, which is a real gap given the
`votes` table's deliberately-unbounded append-only design and the fact that the independence
predicate's cell diversity is rooted in a client-supplied GPS coordinate the server cannot
verify. Two INFO items are minor consistency/robustness notes.

## Warnings

### WR-01: Vote-casting endpoints carry no rate or velocity limit

**File:** `internal/api/router.go:154-191`
**Issue:** The router's only rate limiter (`ratelimit.NewPerIP`, DEC-I: burst 5, one token per
60s) is scoped via `r.With(requestLinkLimiter.Middleware())` to `POST /api/auth/request-link`
alone (line 159). The four vote routes registered immediately below it — `POST
/api/reports/{id}/confirm|dispute|resolve|reopen` (lines 188-191) — carry no limiter of any
kind, only the `requireVerifiedAccount` gate.

This matters more here than it would for an ordinary write endpoint because of two properties
this phase deliberately built:
1. `votes` has no unique key beyond its `id` primary key (migration `00004_create_votes.sql`,
   proven at the database level by `TestVotesHaveNoUniqueKeyBeyondPrimaryKey`) — every POST to
   a vote route appends a new row regardless of whether it duplicates the caller's own standing
   vote. A single verified account can grow this table without bound against a single report at
   essentially no cost (3 DB round trips per call: `ReportVoteContext`, `InsertVote`,
   `CurrentVotesForReports` — see `internal/service/trust.go:290-363`).
2. The independence predicate's "distinct geohash cell" half is computed server-side from a
   client-supplied `latitude`/`longitude` pair (`CastVoteInput`, `internal/service/trust.go:337`)
   that the server has no way to corroborate against the account's actual location. A caller who
   controls several verified accounts (each requiring only an email address, not a payment or
   phone verification) can supply an arbitrary distinct coordinate per account and defeat the
   independence predicate's "resistant to trivial gaming" goal (this Core Value is the phase's
   own named design target) without needing to physically be anywhere — nothing rate-limits how
   many distinct-cell votes one IP or one burst of accounts can cast against one report.

Neither of these is a correctness bug in the resolver itself — `Resolve` and `BuildVoteTally`
do exactly what they are specified to do given their inputs. But the phase's stated goal is
resistance to *trivial* gaming, and right now the only friction on scaling either attack is the
magic-link email flow's own cooldown (`ResendCooldown`, 45s per address) — there is nothing that
throttles vote volume once an account is verified.

**Fix:** Add a per-account (and/or per-IP) token-bucket limiter scoped to the four vote routes,
mirroring the existing `r.With(...)` pattern used for `/api/auth/request-link`:
```go
voteLimiter := ratelimit.NewPerIP(1*time.Second, 10) // or per-account, keyed on acc.ID
r.With(voteLimiter.Middleware()).Post("/api/reports/{id}/confirm", handlers.CastVote(...))
// ...repeat for dispute/resolve/reopen, or wrap the whole sub-group
```
At minimum, consider capping the number of *distinct geohash cells* one account can contribute
across recent votes in a short window, since that is the specific signal the independence
predicate depends on and the specific one a rate limiter alone does not fully address.

## Info

### IN-01: `ActivityForAccount` and `Profile` read the wall clock twice for one page render

**File:** `internal/service/auth.go:361`, `internal/api/handlers/auth.go:421`
**Issue:** `ActivityForAccount` captures `now := time.Now().UTC()` to pass into `Resolve` (whose
`now` parameter is currently unused — see `visibility.go`'s own doc comment), and the `Profile`
handler separately captures `now := time.Now()` a few instructions later to compute
`CanReopen`'s `row.ExpiresAt.After(now)` check (`internal/api/handlers/auth.go:376`). These are
two independent live-clock reads for what is conceptually one render. Go's `time.Time`
comparisons are correct regardless of location, so this is not a live bug today (the drift
between the two calls is microseconds, and `Resolve` ignores its `now` argument entirely in
Phase 2), but the moment `now` becomes load-bearing in `Resolve` for Phase 3's decay scoring
(TRUST-07, flagged in `visibility.go`'s own comment as the reason the parameter exists), a
`CanReopen` gate computed against a *different* clock read than the one `Resolve` used for
`Visibility` becomes a real (if narrow) source of disagreement between the two.
**Fix:** Thread a single `now time.Time` from the `Profile` handler down into
`ActivityForAccount` (or have `ActivityForAccount` return the `now` it used alongside its
`[]ActivityReport`), so the visibility decision and the reopen-eligibility decision are provably
computed against the same instant. Low priority for Phase 2; worth doing before Phase 3 wires up
time-based decay.

### IN-02: `newProfileReport`'s `CanReopen` duplicates an expiry rule that lives nowhere else, with no shared constant or test spanning both call sites

**File:** `internal/api/handlers/auth.go:376`
**Issue:** `canReopen := visibility == service.VisibilityRetracted && row.ExpiresAt.After(now)`
encodes a real, load-bearing business rule (a reopen on an expired report 409s at
`VotingService.CastVote`'s step 4, `internal/service/trust.go:326-332`) purely in a handler-
layer boolean, duplicating the *meaning* of that check without sharing any code or constant with
it. The two are proven consistent today only by two separate, non-adjacent tests
(`TestProfileWithholdsReopenOnAnExpiredRetractedReport` in `activity_e2e_test.go` and
`TestCastVoteOnExpiredReportIsRejected` in `votes_e2e_test.go`), not by a shared implementation.
This is not a bug — the current behavior is correct and the doc comment above `newProfileReport`
explains the reasoning clearly — but it is exactly the kind of two-copies-of-one-decision
pattern that this phase's own commentary elsewhere (e.g. `criticalBypass`'s doc comment:
"deliberately one method rather than two copies that could drift apart") identifies as a risk
worth naming when it appears.
**Fix:** Consider exposing a small `service` helper (e.g. `func CanVoteResolution(expiresAt,
now time.Time) bool`) that both `VotingService.CastVote`'s expiry check and
`newProfileReport`'s `CanReopen` computation call, so a future change to the expiry rule (e.g.
scoping it differently per vote kind, which `trust.go`'s own comment flags as a currently-true
but non-obvious constraint) cannot update one call site and silently miss the other.

---

_Reviewed: 2026-09-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
