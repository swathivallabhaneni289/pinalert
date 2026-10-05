# Phase 4: Robustness, Real-World Resilience - Research

**Researched:** 2026-10-05
**Domain:** Go 1.26 / chi / pgx / sqlc backend extension (text screening, rate limits, moderation state, idempotent submit, legal pages) plus vanilla JS pages (Low data view, IndexedDB queue, Flag control, moderator screen)
**Confidence:** HIGH on integration points (read from the real code), MEDIUM on provider behaviour (two OpenAI pages fetched, error codes assumed), LOW on Indic-language moderation accuracy (unmeasurable until the fixture is recorded)

Provenance tags used below: `[VERIFIED: codebase]` read in this repo today, `[CITED: url]` fetched this session, `[ASSUMED]` training knowledge or inference, needs confirming. Facts already verified by the two earlier passes (CONTEXT "Verified outside facts") are not re-verified and are referenced as `[CONTEXT]`.

## Corrections after the code check (2026-10-05)

A 32 agent read only check of this file and `04-PATTERNS.md` against the real code (314 load bearing claims, 281 confirmed, 13 errors found and each upheld by two independent skeptics) produced the list below. Every item is already corrected in place where it was stated; this list is the audit trail and the quick read. Item 1 would have shipped broken behaviour.

1. **BLOCKS, refunds never worked.** `Reservation.Cancel()` and `CancelAt(later t)` are silent no-ops for a reservation granted without delay (x/time v0.16.0 `rate.go`, pinned by its `TestCancelTooLate`). Refund with `CancelAt(the instant passed to ReserveN)` once (`sync.Once`), and advance the injected clock between Reserve and Cancel in every refund test. Fixed in Area C item 4, Code Examples, Pitfalls 1 and 2, A4.
2. **Moderator route leak.** A wrong method on a moderator path gets chi's 405 plus `Allow` before any group middleware runs. Register the moderator paths with `r.Handle` (all methods) inside the gate group, and test wrong methods too. Fixed in Area B Gate row, Pitfall 5, Code Examples, Security Domain.
3. **Idle window.** Evict a bucket only after burst times refill interval (120 minutes other reports, 25 serious, 20 Flag, 5 votes, 30 seconds cooldown). Fixed in Area C item 1.
4. **`Submit` has one caller.** No test or seed path calls it; add `SubmitChecked` for clarity, not to protect callers. Fixed in Area A entry row and R-10.
5. **`VotingQuerier` does not need `ReportExtrasForReports`.** `CastVote` reads moderation through the extended `ReportVoteContext` row. Two interfaces, two fakes. Fixed in item 7, Summary, collisions row 4.
6. **The `Resolve` caller guard test** must strip comments (`feed.go` line 47) and allowlist `reliability.go` (Phase 3 plan 03-03's one neutral call) with an assertion that it keeps exactly one call. Fixed in item 7 and collisions row 5.
7. **`fetchReports` has other callers** (`votes.js` after every vote, `feed.js`, `submitReport`); the Low data page loads `votes.js`, so `low.js` must subscribe to the store and call `setCenter` first. Fixed in Summary (4) and Area D.
8. **The all files sink test needs a `//` aware comment stripper** (`map.js` line 11 mentions `innerHTML` in a comment); `stripCSSComments` strips blocks only. Fixed in Area B landmines and the Cross cutting row.
9. **`.top-chrome` does not exist today**; it is Phase 4's own wrapper (UI-SPEC P4-A1). Fixed in collisions row 16.
10. **`TestVoteBlockHiddenGuard` covers only `.vote-*` selectors**, so Phase 4 needs its own walker test for its new selectors. Fixed in collisions row 13 and the ROBUST-01 test row.
11. **Existing tests listed as new**: `TestUserVisibleCopyUsesPlainPunctuation`, `TestSwaggerSpecCoversRoutes`, `TestNearbyBatchesVoteReadsOnce` and `TestMigrationsEmbedded` exist today and are amended. Fixed in the Validation Architecture rows and in `04-VALIDATION.md`.
12. **Phase 3 status**: all 20 plan files exist on disk now (03-16 and 03-19 were not yet committed); none of Phases 3, 3.1, 3.2 is built. Fixed in the collisions intro.
13. **Minor**: `NearbyReports` is held fixed by convention (not a byte pin); the Flag wire slugs are chosen here; the mailer loader test has five cases; the exempt route list has seven entries with the two new routes; no test reads the body of `feed.js` `render`; `votes.js` `getCurrentPosition(` is pinned only inside `getVoterLocation()`; Phase 3 edit order notes for `votes.js` and `app.js` (collisions rows 10 and 17).

<user_constraints>
## User Constraints (from CONTEXT.md)

Source of truth: `.planning/phases/04-robustness-real-world-resilience/04-CONTEXT.md`. To keep this file inside the planner's reading budget, the 24 decisions and 10 derived rules are NOT copied here. They are locked exactly as written in CONTEXT lines 49 to 308 and the planner must read that block. One-line index so every row below can cite them:

| Group | Ids | One line |
|-------|-----|----------|
| Text screening | D-01 to D-07 | Inline check, 3 s limit, text only to provider; pending holds non-critical off every feed; clear abuse (hate, harassment, sexual) rejected for all severities; borderline posts and is listed for review; spam is gibberish only; one general check reusable by Phases 8 and 9; fixture of 150 to 300 reports |
| Moderator and Report | D-08, D-09 | Moderator screen (email list in server config, plain 404 for everyone else, Keep and Remove, Remove works on any id); quiet Report control on popup, row and Low data view, never hides by itself |
| Rate limits | D-10 to D-12 | Keyed on verified account; serious vs other report buckets; vote burst 20 then 1 per 15 s; re-cast once per 30 s; Report button 10 then 1 per 2 min (discretion); exact wait message, `Retry-After`, 112 line for serious; IP is a loose backstop only |
| Low data view | D-13 to D-17 | Own page, "Text only, no map", read, post, Confirm and Dispute; device position with typed place fallback; ways in and a remembered choice |
| Offline queue | D-18 to D-20 | Website only, IndexedDB plus retry, reports never expire client side, server stamps times, "Written 4 hours ago" label, strip with Send now and Discard, bound to the account |
| Disclaimer and legal | D-21 to D-24 | Bar on every full page; numbers wording; public Disclaimer and Policy pages; contact line only when configured |
| Derived rules | R-01 to R-10 | Moderation reaches `Resolve` through `ReportMeta`; rating neutral; late reject withdraws; serious equals `criticalBypass()`; server stamps times, request id plus unique index; rune counting; legal pages carry no account data; bar on every full page; Low data is a fourth trust block surface; demo data exempt |

### Claude's Discretion (condensed here, full text in CONTEXT lines 274 to 307)
Reject and borderline thresholds tuned on the fixture (pin the dated snapshot `omni-moderation-2024-09-26`); provider client in plain `net/http` following `mailer.LoadMailer` and `internal/geocode`; zero balance key test early; how held and withheld states are modelled (R-01 must hold); limiter store (memory or database) with the consequence stated; IP backstop header handling; IndexedDB without libraries; Low data build approach and byte budget; exact copy of messages; bar and Report control geometry (UI contract); Disclaimer and Policy text drafts.

### Deferred Ideas (OUT OF SCOPE), condensed here, full text in CONTEXT lines 493 to 513
- Instagram style report page with comments and reactions (Phase 8): open any report as its own page, comment, react. Reactions already exist as Confirm and Dispute. Whether to allow plain likes is a Phase 8 question.
- The author deleting their own report. Today an author can mark a report resolved (Phase 2).
- State aware helplines: store the person's state at report time and show that state's own disaster management authority link instead of one static string.
- An installable app, a service worker and Web Push (backlog 999.3). The owner asked to keep it a website.
- Image safety screening (stretch, tied to photo attachment).
- Account level sanctions for repeat offenders (strikes, suspension).
- Email alias normalisation and disposable email blocking.
- Reviewed todo not folded: real 3D WebGL globe on the login screen.
</user_constraints>

<phase_requirements>
## Phase Requirements

Keyed by the five existing ids plus one row per added scope group (the orchestrator may assign ROBUST-09 onward).

| ID | Description | Research Support |
|----|-------------|------------------|
| ROBUST-01 | Low data view, own page, text only | Area D: client rendered rows (`low.js`), `GET /low`, byte budget, map free location path, own fetch boot |
| ROBUST-02 | Offline queue | Area E: IndexedDB, request id with partial unique index, server stamped times, display only `written_at`, account binding |
| ROBUST-03 | Text screened before publishing | Area A: `internal/moderation`, pure `Decide`, inline single attempt, pending, worker, recorded fixture |
| ROBUST-04 | Limits keyed on the verified account | Area C: `PerKey` with `Reserve` and `Cancel`, handler level gating, IP backstop that skips when unknown |
| ROBUST-08 | Persistent disclaimer | Area F: shared partial, `Chrome` embed, exempt routes, numbers setting |
| Added: held state and moderation model (D-02, R-01 to R-03) | `withheld` Visibility, `ReportMeta.Moderation`, columns on `reports`, one batched read | Architecture Patterns |
| Added: vote throttle and re-cast cooldown (D-10 to D-12, WR-01) | Wrapper on four routes, cooldown refund on non-2xx | Area C |
| Added: moderator screen (D-08) | `requireModerator`, three queue queries, Keep and Remove, audit table | Area B |
| Added: Flag control (D-09) | `POST /api/reports/{id}/flag`, unique per account and report, own limiter | Area B |
| Added: Disclaimer and Policy pages (D-21 to D-24) | Public routes, no account data, contact line by setting | Area F |
| Added: posting and voting on Low data (D-15, D-16, R-09) | `report-form.js` extraction, `createVoteBlock(id, { resolve: false })`, fourth trust block surface | Area D |

</phase_requirements>

## Summary

The backend work extends five existing seams: `handlers.SubmitReport` and `service.ReportService.Submit` (screening, limiter, idempotency, held state), `service.Resolve` (one new top rung, one new `Visibility` value), `internal/ratelimit` (a new keyed limiter beside `PerIP`, leaving it unchanged), the router (a gated group plus two new public legal routes) and the embedded `web/` trees. The frontend is five surfaces (bar, Low data, queue strip, Flag dialog, moderator screen) already specified by the approved 04-UI-SPEC; this research gives the real integration points and traps.

Five facts drive the plan. (1) `reports` has `session_id` and no account column, and `NearbyReports` is held fixed (a hand copied const plus an EXPLAIN index test and a source shape test guard it, and Phase 4 leaves the query untouched on purpose), so moderation state and `written_at` become columns on `reports` read through ONE new batched query, added to the feed and Activity read interfaces (`Querier`, `AuthQuerier`) and their two fakes; `CastVote` reads moderation through the extended `ReportVoteContext` row instead. (2) The report limiter cannot be route middleware (the serious bucket depends on the decoded body) and needs a wait time and a refund, which `PerIP.Allow` cannot give (the refund must cancel at the reservation instant: a later `Cancel()` is a silent no-op for a zero delay grant, see Area C item 4). (3) Idempotency is one `INSERT ... ON CONFLICT DO NOTHING RETURNING` on a partial unique index keyed on the caller's account and request id, with an account scoped replay. (4) On the main page the first `fetchReports` call and `startPolling` come only from `map.js` `centerOnVisitor` (the other callers, `votes.js` after every vote, `feed.js` retry and toggle, the `app.js` post submit refresh, only refresh an already booted page), so Low data must boot its own fetch loop and subscribe to the shared store, and the main list never loads when unpkg is blocked. (5) The D-07 fixture cannot call the live API in CI, so it runs on provider scores recorded once by the owner against the pinned snapshot.

**Primary recommendation:** build in this order: (1) no-behaviour-change backend foundation (migration, columns, extras read, `Visibility` value, `ReportMeta.Moderation`, Querier plumbing, `Truncate` list); (2) `ratelimit.PerKey` plus vote and report gating; (3) moderation client, policy, worker, recorded fixture; (4) idempotent submit and `written_at`; (5) legal pages, bar, numbers setting; (6) `report-form.js` extraction as a pure refactor; (7) Low data; (8) queue; (9) Flag and moderator screen. Take migration numbers at execution time (00005 is Phase 3's), run after Phases 3, 3.1 and 3.2 land, and re-read the collisions table first.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Text screening decision (reject, borderline, pass) | API / Backend (service policy) | Provider (OpenAI) | Thresholds and the gibberish rule are server authority; the browser holds none (UI-SPEC sample data rule) |
| Held and withheld visibility | API / Backend (`Resolve`) | Database (columns) | `Resolve` is the sole visibility authority (R-01); the browser renders the server answer |
| Rate limits and Retry-After | API / Backend (handler gating) | Database (none, memory) | Account and body dependent; wait seconds computed server side |
| Idempotent submit | Database (partial unique index) | API | Uniqueness must survive concurrent retries |
| Offline queue storage and flush | Browser (IndexedDB, `queue.js`) | API (replay safe) | Device holds the unsent report; server stamps all times |
| Low data page shell | Frontend Server (html/template) | Browser (`low.js` rows) | Shell and config server rendered, rows built by the same client builders as the main page (R-09) |
| Position capture on Low data | Browser | API (1 km check, cell) | Server never trusts the client for the cell |
| Disclaimer bar, legal pages | Frontend Server (templates) | none | Static, public, no account data (R-07) |
| Moderator screen and actions | API (requireModerator) | Browser (`moderate.js`) | Authorisation is server side; a plain 404 reveals nothing |
| Report (Flag) queue | Database | API | Unique per account and report, never hides a post |

## Standard Stack

### Core (nothing new to install)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `net/http`, `encoding/json`, `unicode`, `unicode/utf8`, `crypto/rand` | go 1.26.4 per go.mod [VERIFIED: codebase] | Moderation client, gibberish rule, rune counting | CONTEXT discretion: plain `net/http` and JSON following `internal/mailer/loader.go` |
| `golang.org/x/time/rate` | v0.16.0 [VERIFIED: codebase go.mod] | Keyed limiter with `Reserve`, `Delay`, `Cancel` | Already the project standard (CLAUDE.md); `Reservation.Delay`, `DelayFrom`, `Cancel` and `CancelAt` exist in v0.16.0 [VERIFIED: module source rate.go lines 139 to 204]; `Cancel` is a no-op after the reservation instant, see Area C item 4 |
| `github.com/go-chi/chi/v5` | v5.3.2 [VERIFIED: codebase] | `middleware.ClientIPFromHeader`, `middleware.NewWrapResponseWriter` (status capture for refunds), optional `middleware.Compress` | In the stack already [VERIFIED: module source client_ip.go, wrap_writer.go, compress.go] |
| `jackc/pgx/v5` + sqlc v1.31.1 | v5.10.0 | New queries (`FOR UPDATE SKIP LOCKED` claim, `ON CONFLICT`, `ANY($1::bigint[])`) | Same patterns as `votes.sql` and `reports.sql` |
| `pressly/goose/v3` | v3.28.0 | Migrations | Embedded FS applied by `testutil.NewTestDB` and `cmd/migrate` |
| Browser IndexedDB, `AbortSignal.timeout`, `navigator.storage.persist`, `crypto.randomUUID`, Web Locks | n/a | Queue, timeouts, persistence | No library, no build step (CONTEXT, CLAUDE.md) |

### Supporting
| Item | Purpose | When to Use |
|------|---------|-------------|
| `testing/httptest` | Fake provider and fake geocoder | All moderation client tests; no live calls |
| `node --test` (Node v26.4.0 present locally) | Optional pure logic tests for `queue.js` with a hand written fake IndexedDB | Only if the planner accepts one test runner beyond Go (see Open Question 6) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In memory limiter | Database backed limiter (`email_cooldowns` upsert pattern) | DB is restart proof but adds a write per request on a hot path and cannot express refunds cleanly. Memory is chosen; the consequence (a cold start or sleep resets buckets) is acceptable because the serious refill is 5 minutes and the free host sleeps only after 15 idle minutes, which already refills a full bucket [ASSUMED: host behaviour from CLAUDE.md Hosting row]. State this in the plan and the Policy-adjacent docs |
| Client rendered Low data rows | Server rendered rows | Server rendering would duplicate the trust block builder in Go and break R-09 (one block built one way). UI-SPEC open item 25 default is client rendered. Follow it |
| Always background moderation (ARCHITECTURE Pattern 4) | Inline then background | CONTEXT D-01 locked inline; the background path stays as the retry mechanism |

**Installation:** none. No new Go module, no npm package, no CDN tag. The Package Legitimacy gate is therefore not triggered.

## Package Legitimacy Audit

No external package is installed by this phase (stack above is entirely already in `go.mod` or browser built in). Nothing to audit.

| Package | Registry | Verdict | Disposition |
|---------|----------|---------|-------------|
| (none added) | n/a | n/a | n/a |

**Packages removed due to [SLOP] verdict:** none. **Packages flagged [SUS]:** none.

## Architecture Patterns

### System Architecture Diagram

```
 phone / browser
   |  GET /disclaimer, /policy (public, no account data)
   |  GET /, /low, /profile, /moderate (gated; /moderate answers 404 to everyone not on the list)
   |  POST /api/reports  (body: report + request_id + written_at)
   |  POST /api/reports/{id}/{confirm|dispute|resolve|reopen}   POST /api/reports/{id}/flag
   v
 chi router: RequestID -> ClientIP (header from setting, else remote addr) -> Recoverer -> Logger -> session cookie
   |
   +-- exempt: GET /login, POST /api/auth/request-link, GET /auth/verify, /static/*, /swagger/*, plus the two new routes GET /disclaimer and GET /policy
   |
   +-- gated group (requireVerifiedAccount -> account in context)
         |
         +-- POST /api/reports -> SubmitReport handler
         |     decode (DisallowUnknownFields) -> ValidateSubmitInput (runes)
         |       -> replay lookup (account scoped request_id) --found--> 200 with the stored report
         |       -> reserve allowance (serious or other bucket) --none left--> 429 + Retry-After + body
         |       -> local prefilter (gibberish) --bad--> cancel reservation, 422 reject, nothing stored
         |       -> provider call, ONE attempt, 3 s deadline, text only
         |            reject (hate, harassment, sexual high) -> cancel reservation, 422, nothing stored
         |            pass -> INSERT moderation_state=clear
         |            borderline -> INSERT clear + check row needs_review
         |            timeout / 429 / 5xx / provider budget spent -> INSERT moderation_state=pending + check row (retry)
         |       -> 201 (held=true only for pending non-serious)
         |
         +-- vote routes: r.With(voteGate) -> CastVote (unchanged service order) ; 429 body, refund cooldown on non-2xx
         +-- GET /api/reports -> Nearby: NearbyReports (pinned) + CurrentVotes + ReporterAccounts + ReportExtras (new, one call)
         |                       -> Resolve(meta incl. Moderation) -> withheld never listable
         +-- POST /api/reports/{id}/flag -> flag limiter -> report_flags (unique) -> moderator queue
         +-- GET /low, GET /profile (held and removed rows), GET /
   +-- moderator group (requireModerator, own lookup, 404 for everyone else; routes registered with r.Handle so every method reaches the gate)
         +-- GET /moderate, GET /api/moderation/queue, GET /api/moderation/reports/{id}, POST .../keep, POST .../remove

 background goroutine (cmd/server/main.go, time.Ticker, started with the process, one sweep at boot)
   claim due moderation_checks (FOR UPDATE SKIP LOCKED + lease) -> provider (up to 2 retries inside one sweep item is NOT used; backoff is across sweeps)
   -> pass: moderation_state=clear | borderline: clear + needs_review | reject: moderation_state=removed
```

### Recommended file additions (backend)
```
internal/moderation/         client.go (net/http, interface-free struct), loader.go (LoadScreener(env)), client_test.go (httptest)
                             testdata/genuine.json, abusive.json   (recorded scores, see Area A)
internal/service/screening.go     pure policy: Decide(scores, serious) Verdict, gibberish rule, reject reasons, copy strings
internal/service/screening_test.go, screening_fixture_test.go
internal/service/modworker.go     background sweep (claim, backoff with jitter, Retry-After)
internal/service/moderation.go    ModerationState, moderator queue reads, Keep, Remove, Flag
internal/ratelimit/perkey.go      PerKey (Reserve, Cancel, injectable clock, sweep) ; perkey_test.go
internal/api/handlers/moderation.go, flag.go, legal.go, low.go, ratelimit_gate.go (429 bodies)
internal/api/moderator.go         requireModerator
internal/store/migrations/NNNNN_*.sql   next free numbers at execution time
internal/store/queries/moderation.sql   new
web/templates/disclaimer_bar.html.tmpl, disclaimer.html.tmpl, policy.html.tmpl, low.html.tmpl, moderate.html.tmpl, report_form.html.tmpl
web/static/js/ queue.js flag.js report-form.js low.js moderate.js slow-notice.js ; css chrome.css legal.css low.css moderate.css modal-map.css
```
The exhaustive edited-file list for the frontend is the UI-SPEC "Hand Off Notes" and is not repeated here.

### Moderation state model (the decision CONTEXT left open, R-01)

Recommendation, all `[VERIFIED: codebase]` constraints noted:

1. **Columns on `reports`, not a side table, for anything the feed needs.** `NearbyReports` selects an explicit column list and is held fixed by convention: `internal/store/reports_test.go` keeps a hand copied const (`nearbyReportsSQLForExplain`) for the EXPLAIN index test (`TestNearbyReportsUsesIndex`), and `TestNearbyReportsQuerySourceHasExpectedShape` only asserts that the query source contains `expires_at > now()` and `BETWEEN` (it is not a byte for byte pin). Adding columns to the table does not touch the query, and Phase 4 leaves it untouched on purpose. Putting the state on the row makes it atomic with the insert, so there is no window where a pending report is visible, and it needs no transaction (the store has none today: `sqlcgen.Queries.WithTx` exists but no service uses it).
   - `moderation_state TEXT NOT NULL DEFAULT 'clear'` with values `clear`, `pending`, `removed`.
   - `request_id TEXT`, `request_account_id BIGINT REFERENCES accounts(id)`, partial unique index `UNIQUE (request_account_id, request_id) WHERE request_id IS NOT NULL`.
   - `client_written_at TIMESTAMPTZ NULL` (display only, R-05).
   - Existing rows and `internal/testutil/seed.go` raw inserts keep working through the defaults (R-10: seeded data is `clear`).
2. **Side tables for everything the feed does not need.**
   - `moderation_checks` (`report_id` PK FK, `status` pending or done, `attempts`, `next_attempt_at`, `lease_until`, `last_error`, `verdict`, `scores JSONB`, `model`, `needs_review BOOL`, `checked_at`). Partial index on `next_attempt_at WHERE status = 'pending'` for the sweep; partial index on `needs_review WHERE needs_review` for the moderator list. Scores are stored without report text (D-04).
   - `report_flags` (`id`, `report_id` FK, `account_id` FK, `reason`, `created_at`, `dismissed_at`, `UNIQUE (report_id, account_id)`).
   - `moderation_actions` (`id`, `report_id`, `moderator_account_id`, `action`, `created_at`): an audit trail. Cheap, and it is the evidence for the Policy page's grievance path and a portfolio signal [ASSUMED: not required by CONTEXT, recommended].
3. **A fifth `Visibility` value `withheld` with two new `ResolveReason` slugs.** Suggested slugs `removed_after_review` and `awaiting_text_check`; the planner confirms the final words. `ListableInFeed` has a fail closed `default` branch [VERIFIED: codebase feed.go], so a withheld value is excluded from every view that uses it even after Phase 3.1 rewrites the partition. `Visibilities` and `Valid()` must include it, because `newProfileReport` falls back to `provisional` for any value that fails `Valid()` [VERIFIED: codebase `internal/api/handlers/auth.go` lines 375 to 377], which would show a removed report as Unconfirmed on the Activity page.
4. **`ReportMeta` gains `Moderation ModerationState`.** Zero value is `clear`, so `visibility_test.go` and Phase 3's neutralised metadata (`ClassifyOutcome` builds `ReportMeta{Severity: SeverityLow, Category: CategoryOther}`) keep working, which is exactly R-02.
5. **Rung order in `Resolve`:** removed first (above Retracted, because a critical report is otherwise Live through the bypass), then pending-and-not-serious (withheld, reason `awaiting_text_check`), then the existing Retracted, bypass, Hidden, Provisional, Live. Pending and serious falls through to the bypass, which is the published-while-pending behaviour (D-01, R-03). "Serious" must call one predicate: add an exported wrapper such as `func (m ReportMeta) Serious() bool { return m.criticalBypass() }` rather than renaming `criticalBypass` (Phase 3 docs cite the name). The handler uses it to pick the allowance bucket (R-04).
6. **Votes and Flags on withheld reports answer 404.** `CastVote` must not accept a vote on a report the viewer cannot see. Add `r.moderation_state` to the `ReportVoteContext` select (Phase 3 plan 03-01 already extends this select) and refuse with `ErrReportNotFound` right after step 2. This amends Phase 3's pinned `CastVote` step order (`TestCastVoteCheckOrder`): list it as a test that changes. Result: `CastVoteResponse` never carries a withheld value, so its swagger enums stay at four values.
7. **Read path: one new batched query, `ReportExtrasForReports(report_ids, viewer_account_id)`** returning `report_id, moderation_state, client_written_at, viewer_flagged` (LEFT JOIN `report_flags` on the viewer). It is called once per page after the two existing batched reads, never when the page is empty. Add it to `Querier` (`report.go`) and `AuthQuerier` (`auth.go`) and to their two fakes, `fakeFeedQuerier` (`feed_test.go`) and `fakeAuthQuerier` (`auth_test.go`). `VotingQuerier` does NOT need it: `CastVote` reads `moderation_state` through the extended `ReportVoteContext` row (item 6), so only `fakeVotingQuerier.ReportVoteContext` and its `voteContextRow` literals change (`trust_test.go`). All three `Resolve` call sites must pass moderation: `report.go:450` (Nearby), `trust.go:361` (CastVote, via the context column), `auth.go:378` (ActivityForAccount); these line numbers are today's and will shift when Phase 3 lands, so cite function names. Guard against a fourth caller forgetting it with a source test in `internal/service` that reads every non-test `.go` file there and strips comments (`//` full line and trailing, and `/* */` blocks) before matching, because `feed.go` line 47 mentions `Resolve()` only inside a comment. A file is a caller when its comment stripped source contains `Resolve(` outside `func Resolve(`. Every caller must also contain the identifier `Moderation` in code. Allowlist `reliability.go` by name: Phase 3 plan 03-03 `ClassifyOutcome` makes exactly one deliberate neutral call, `Resolve(ReportMeta{Severity: SeverityLow, Category: CategoryOther}, ...)`, where the zero value `clear` is the intended behaviour (R-02) and 03-03's own verify gate pins that line, so 03-03 must not be edited; the test keeps the allowlist honest by asserting that `reliability.go` still has exactly one non comment `Resolve(` call. Phase 3.1 view queries are not allowlisted: they must pass the real moderation state and are caught by the test.
8. **Activity page.** `ReportsByAccount` returns no moderation columns; `ActivityForAccount` makes the same extras call. Held and removed rows set `data-moderation` and the server omits `data-trust` and `BuildTrust` input (UI-SPEC P4-A9, P4-A10).

## Area A: Text screening (ROBUST-03, D-01 to D-07, R-02, R-03, R-06)

### Integration points
| What | Where | Note |
|------|-------|------|
| Entry | `handlers.SubmitReport` (`reports.go`), `service.ReportService.Submit` (`report.go`) | `Submit(ctx, sessionID, in)` has exactly one caller today, `handlers.SubmitReport` (`reports.go` line 275). No test and no seed path calls it: `internal/testutil/seed.go` and the store tests insert with raw SQL, e2e tests reach it only through `POST /api/reports`, and `cmd/` has no demo seeder. Keep `Submit` as the plain validated insert (server stamping) and add a new method, for example `SubmitChecked`, for replay, screening and the idempotent insert, so the screening path stays separate; the handler switches to `SubmitChecked`. The plan decides explicitly whether `Submit` stays as a thin wrapper that `SubmitChecked` reuses or is deleted, because it will otherwise have no callers. There is no existing `Submit` test churn to budget: the handler tests (`TestSubmitScreening`, `TestSubmitIdempotent`) pin the new order |
| Validation | `ValidateSubmitInput` uses `len(desc)` for 10 and 1000 [VERIFIED: codebase report.go lines 199 to 205] | R-06: switch to `utf8.RuneCountInString`. Telugu, Hindi and Tamil letters are 3 bytes, so 4 Telugu letters pass the minimum today. Check `report_test.go` boundaries |
| Provider client | new `internal/moderation` | `Screen(ctx, text) (Scores, error)`. The interface (`service.TextScreener`) is declared in the consumer, as `handlers.GeocodeSearcher` is. `LoadScreener(env)` mirrors `mailer.LoadMailer`: key set gives the real client; unset in `ENV=development` gives a pass-everything fallback with a loud log; otherwise an error that `main` turns into `log.Fatal` |
| Env | `OPENAI_API_KEY`, optional `MODERATION_MODEL` (default the dated snapshot) | Constants block in `cmd/server/main.go` |
| Policy | pure `service.Decide(scores, serious)` | pass, borderline, or reject with one of four reasons (hateful, harassing, sexual, gibberish) |
| Worker | `ModerationWorker`, `time.Ticker` goroutine started from `main` | Run one sweep at boot (free host cold starts) |
| Deps | additive `api.Deps` fields (`Screener`, limiter config) | A nil `Screener` must mean pass-everything with no network: `swagger_test.go` builds `Deps` with `service.NewReportService(nil)`, and `gate_test.go` and `reports_e2e_test.go` build `Deps` literals that omit new fields [VERIFIED: codebase] |

### Provider facts (fetched this session)
- `POST https://api.openai.com/v1/moderations`, headers `Content-Type: application/json` and `Authorization: Bearer $OPENAI_API_KEY`, body `{"model":"...","input":"<string>"}`; response `{id, model, results:[{flagged, categories, category_scores, category_applied_input_types}]}`. Category keys exactly: `sexual`, `sexual/minors`, `harassment`, `harassment/threatening`, `hate`, `hate/threatening`, `illicit`, `illicit/violent`, `self-harm`, `self-harm/intent`, `self-harm/instructions`, `violence`, `violence/graphic` [CITED: https://developers.openai.com/api/docs/guides/moderation].
- Snapshot `omni-moderation-2024-09-26` exists; free tier 250 requests a minute, 5,000 a day, 10,000 tokens a minute [CITED: https://developers.openai.com/api/docs/models/omni-moderation-latest].
- Error bodies are not on those pages. Telling quota from rate limit by an error `code` (`insufficient_quota` versus `rate_limit_exceeded`) is `[ASSUMED]`. Classify by status first: 429, 5xx and timeout are transient; 400, 401, 403 are permanent configuration faults, handled like pending (fail closed for non-critical) and logged at error level.

### Landmines
1. **Timeout budget.** `http.Server.WriteTimeout` is 10 s [VERIFIED: codebase main.go line 173] and the UI contract's client timeout is 10 s. Inline screening is ONE attempt in `context.WithTimeout(ctx, 3s)`. The "at most 2 retries" in CONTEXT applies to the background worker only, spread across sweeps; three inline attempts would race the queue's own timeout.
2. **Tokens a minute, not requests, bind for Indic text.** A 1000 character Telugu report may cost about a token per character [ASSUMED], so roughly 10 such reports a minute exhaust the free tier. Add a process wide provider budget (a `rate.Limiter` on estimated tokens using rune count, plus a request limiter); when refused, go to pending inline or wait in the worker.
3. **Reject before insert, nothing stored (D-03).** Provider call precedes the insert. Order in the handler is in Code Examples.
4. **Only the cleaned text leaves.** Send `strings.TrimSpace(description)`, never account, session, IP or coordinates (D-01). Test with a recording fake provider asserting the exact body.
5. **Reject set is hate, harassment, sexual only.** Map `hate`, `hate/threatening` to hateful; `harassment`, `harassment/threatening` to harassing; `sexual`, `sexual/minors` to sexual. Never use the provider's `flagged` boolean. Violence, self-harm and illicit categories only feed borderline. Starting thresholds are `[ASSUMED]` (reject 0.85, borderline 0.40), tuned on the fixture.
6. **Gibberish rule counts runes and respects Indic scripts.** A letter is `unicode.IsLetter(r) || unicode.IsMark(r)` so vowel signs (Mn, Mc) do not split words. Reject only on a run of one rune of at least a stated length (suggest 10) or fewer than two tokens of two or more letters. A vowel heuristic is safe only for Latin tokens of five or more letters. D-05 forbids looking at capitals, links, repeats across posts and phone numbers; the fixture includes all caps, addresses and phone numbers.
7. **Late verdicts (R-03).** Worker reject sets `moderation_state = removed` (published serious or held alike); pass sets `clear`; borderline sets `clear` plus `needs_review`. The worker query adds `expires_at > now()`; an expired pending report stays pending and shows in the moderator list (c).
8. **Worker claim.** One statement: `UPDATE moderation_checks SET lease_until = now() + interval '60 seconds', attempts = attempts + 1 WHERE report_id IN (SELECT report_id ... WHERE status = 'pending' AND next_attempt_at <= now() AND (lease_until IS NULL OR lease_until < now()) ORDER BY next_attempt_at LIMIT 5 FOR UPDATE SKIP LOCKED) RETURNING ...` joined to `reports` for the text. Backoff `min(30s * 2^attempts, 1h)` times jitter 0.5 to 1.5, and at least `Retry-After`.
9. **Reuse (D-06).** `Decide`, the client and the four messages carry no report vocabulary; only the layer mapping serious and pending to hold behaviour knows reports.

### D-07 fixture: what CONTEXT does not say
"The test fails if any genuine fixture is rejected" cannot call the provider in CI (tests never make live calls; quota is 5,000 a day). So:
- Fixture entries hold text, language, `expect`, `serious` and **recorded `category_scores`** captured once from the pinned snapshot by an opt-in recorder (for example `go run ./cmd/modrecord`, throttled under 10,000 tokens a minute). Tests read the recorded numbers and run `Decide`, so they are offline and stable. A live re-record needs `MODERATION_LIVE=1`, checked by the recorder, never set in CI.
- Someone must run the recorder with a key: an owner step (also the zero balance key check). Until then the fixture test covers only the gibberish rule and structure; make it an end-of-wave human check.
- Writing 150 to 300 reports in five languages is authoring work, and model written text is not real traffic; say so beside the "zero of N, under about 3 in N" sentence (D-07).

## Area B: Moderator screen and Report (Flag) control (D-08, D-09, R-01)

### Integration points
| What | Where | Note |
|------|-------|------|
| Moderator list | `MODERATOR_EMAILS` env (comma separated), parsed at startup in `cmd/server/main.go` into a set | Normalise each entry exactly as sign-in does: `normalizeEmail` is unexported in `internal/service/auth.go`; export a small wrapper or move the function, then compare against `account.Email` (already lowercased at sign in) |
| Gate | new `requireModerator` in `internal/api/moderator.go` | **It must do its own account lookup and answer a plain 404 for every request that reaches it**, including an unverified visitor and an API path. If `/moderate` sat inside the existing gated group, an unverified visitor would get a 302 to `/login` and an unverified API caller a 401, while an unknown path gives chi's 404, which reveals that the route exists. `http.NotFound` writes the same body as chi's default (`404 page not found`). **Method trap (verified against chi v5.3.2 `routeHTTP`):** chi matches path and method before group middleware runs, so a request whose path matches a moderator route but whose method is not registered (HEAD or POST on `/moderate`, GET on a POST route) never reaches the gate and gets chi's 405 with an `Allow` header, which also reveals the route. Decision for the plans (scoped, no router wide behaviour change): register `/moderate` and every `/api/moderation/...` path with `r.Handle` (all methods) inside the `requireModerator` group, never `r.Get` or `r.Post`; the gate 404s everyone who is not a moderator on every method chi knows, and a small method check inside each handler answers a real moderator who uses the wrong method with its own 405. A method chi does not know at all (for example PROPFIND) gets 405 on every path, known or unknown, so it leaks nothing. The rejected alternative is `r.MethodNotAllowed(http.NotFound)` on the root router, which would change the answer for every public route. The byte equality test is a table: anonymous, unverified and verified non moderator visitors crossed with an unknown path, each moderator path on its registered method, and each moderator path on the wrong methods (HEAD, GET on the POST routes, POST on `/moderate`, PUT), asserting status, body bytes, `Content-Type` and the absence of an `Allow` header equal those of the same method on an unknown path |
| Routes | `GET /moderate`, `GET /api/moderation/queue`, `GET /api/moderation/reports/{id}`, `POST /api/moderation/reports/{id}/keep`, `POST /api/moderation/reports/{id}/remove`, flat registration like every other route | Do NOT add swag annotations for the moderator API: it is an internal surface and the swagger drift guards forbid `session_id` strings anyway. Document the Flag route in swagger because it is a normal public JSON route |
| Queue queries | three sqlc queries with a cap of 100 | (a) `moderation_checks WHERE needs_review` joined to `reports` for text and scores, (b) open `report_flags` grouped by report with reason counts, (c) `moderation_checks WHERE status = 'pending'` including published serious posts |
| Keep | clears `needs_review`, sets open flags `dismissed_at`, and on a pending report sets `moderation_state = clear` and the check `done` | No separate Release action (D-08) |
| Remove | `moderation_state = removed`, audit row | Works on any id typed or pasted; resolves the id from `report=ID` in a pasted link on the client |
| Flag | `POST /api/reports/{id}/flag` body `{reason}`, reasons spam, abusive, false, dangerous, other | The UI-SPEC (Surface: Flag Control and Dialog) defines only the five display labels; the wire slugs `spam`, `abusive`, `false`, `dangerous`, `other` are chosen here, and the planner fixes them once in the outline's cross-plan contracts, then uses them for `report_flags.reason`, the swagger `enums:` tag and the moderator screen reason counts, with the label mapping kept in one place. Refuse own report (403), unknown or withheld report (404), repeat (409, UI-SPEC says the dialog already handles 409). Never changes `Resolve` input (a flag never hides) |
| Viewer marker | `viewer_flagged` from `ReportExtrasForReports` | UI-SPEC open item 5 default; keeps the constant round trip test |
| CSRF | SameSite=Lax session cookie (DEC-L in `router.go` comments) | Cross-site POSTs do not carry the cookie. No token needed; say so in the threat model |

### Landmines
- `report=ID` deep links: Phase 3.1 owns the `view` and `report` query names; the moderator link builder lives in `moderate.js` and only builds `/?view=...&report=ID` strings. Do not read those parameters in Phase 4 server code.
- Moderator text is full report text from strangers: `moderate.js` is the page where a markup sink matters most. Use the planned one test that scans every file under `static/js` (comments stripped) for `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`. The comment stripper must remove both `/* */` blocks and `//` line comments: the only existing helper, `stripCSSComments` (`web/css_contract_test.go`), removes block comments only, and `web/static/js/map.js` line 11 mentions `innerHTML` in a `//` comment, so a scan built on it would be red on day one. Write a small `stripJSComments` that treats `//` as a comment only at the start of a line (after whitespace) or after whitespace, never inside a string or a URL such as `https://`, and also reword that `map.js` comment (map.js is a Phase 4 edited file) so the test does not depend on comment wording. The existing raw per file `modal.js` and `theme.js` scans stay as they are; they pass today only because those files avoid the words.
- A moderator who is also an author: Keep and Remove on your own report is allowed (no special rule in CONTEXT); do not invent one.
- Do not log report text in the audit table or in request logs.

## Area C: Rate limits (ROBUST-04, D-10 to D-12, WR-01)

### What exists [VERIFIED: codebase `internal/ratelimit/perip.go`]
`PerIP.Allow(key) bool` has no wait time and no refund; `NewPerIP(every, burst)` is a documented stable signature; `Middleware()` keys on the client IP with a TCP peer fallback and writes a 429 with `"field":"email"`, the message `Too many requests. Try again in a minute.` and no `Retry-After`. That sentence is also pinned in `auth.js`, `perip_test.go` and `handlers/auth.go` (`agreedRateLimitSentence`, `web/design_rules_contract_test.go`). Leave `perip.go` and that string alone.

### Design
1. **New `ratelimit.PerKey`** beside `PerIP`: `NewPerKey(Config{Every, Burst, IdleWindow, Now})`; `Reserve(key) Decision{OK, RetryAfter, Cancel}` over `rate.Limiter.ReserveN(now, 1)`, `DelayFrom(now)` and `CancelAt(now)` with the SAME captured instant `now` (see item 4 and Code Examples: later cancels are silent no-ops); a sweep like `PerIP`'s with the idle window sized per bucket instance: an entry may only be evicted once its bucket would have refilled to full anyway, because eviction hands back a full burst, so the idle window is at least burst times refill interval for that bucket (120 minutes for the other-report bucket (8 x 15), 25 for the serious bucket (5 x 5), 20 for the Flag bucket (10 x 2), 5 for votes (20 x 15 s), 30 seconds for the re-cast cooldown, 2 minutes for the submit attempt bucket, 20 for the IP backstop); `PerIP`'s 10 minute `defaultIdleWindow` is too short for the report buckets; injectable clock (the old limiter has none). `RetryAfter` is rounded up to whole seconds.
2. **Gate in the handler, not middleware, for reports.** The bucket depends on `Serious()` of the decoded body (R-04). Add an exported wrapper `func (m ReportMeta) Serious() bool { return m.criticalBypass() }` and keep `criticalBypass` (Phase 3 docs cite it). The four vote routes key on the account only, so `r.With(voteGate)` inside the gated group works and `internal/service/trust.go` stays untouched (three Phase 3 editors).
3. **Buckets** (one named constants block in `cmd/server/main.go`, passed as `api.Deps` fields whose zero value means the default, the `RequestLinkRateLimit` contract [VERIFIED: codebase router.go]):

| Bucket | Key | Burst | Refill | Source |
|--------|-----|-------|--------|--------|
| Report, serious | `acct:<id>` | 5 | 1 per 5 min | D-11 |
| Report, other | `acct:<id>` | 8 | 1 per 15 min | D-11 |
| Vote | `acct:<id>` | 20 | 1 per 15 s | D-11 |
| Re-cast cooldown | `acct:<id>:rpt:<id>:<kind>` | 1 | 30 s | D-11 |
| Flag | `acct:<id>` | 10 | 1 per 2 min | D-11 default |
| Submit attempt (recommended) | `acct:<id>` | 20 | 1 per 6 s | provider protection, never refunded |
| IP backstop (recommended) | client IP when known | 60 | 1 per 20 s | [ASSUMED] numbers |

4. **Refund rules.** Reserve the report allowance after validation (a 400 is free). `Cancel` on text reject, prefilter reject, any non-2xx and replay (look up the replay first so it never reserves). Answer to "does a reject consume": **no**. D-03 keeps the form open for rewording and 5 serious tokens refill every 5 minutes, so charging rejects would lock a real person out. The submit-attempt bucket and provider budget protect the provider instead. The 30 s cooldown reservation is cancelled on any non-2xx, because Phase 3 plan 03-04 makes `votes.js` retry once after a 403 too far and a consumed cooldown would 429 that retry [VERIFIED: outline pinned test `TestVoteTransportRefreshesAndRetriesOnce`]. The 20 burst vote bucket is NOT refunded (each attempt costs a `ReportVoteContext` read). Capture status with `middleware.NewWrapResponseWriter(w, r.ProtoMajor)`. **Refund correctness (verified against the x/time v0.16.0 source and its own `TestCancelTooLate`):** `Reservation.Cancel()` and `CancelAt(t)` with `t` later than the reservation instant are silent no-ops for a reservation granted without delay (`CancelAt` returns early on `r.timeToAct.Before(t)`), so the refund must call `CancelAt` with the SAME instant that was passed to `ReserveN`, captured once before the call and wrapped in `sync.Once` (`CancelAt` is not idempotent). Never call `Cancel()` or `CancelAt(p.now())` at refund time. Cancelling at the earlier instant moves the limiter's last event backwards, so it can over-credit a little refill when another reservation for the same key intervened, and it returns no refund at all when a later reservation for the same key was denied in between; the refund is best effort under concurrent same-key requests, and the `-race` bound test must tolerate both. Every refund test must advance the injected clock between `Reserve` and `Cancel` (a test with a frozen clock passes against the broken form) and then assert that the next `Reserve` is admitted; add one interleaved case (A reserves, B is denied, A cancels) that documents the lost refund.
5. **429 contract.** New writer, never `writeTooManyRequests`. Header `Retry-After: <integer seconds>`. Body keeps `{ "error": {"field","message"} }` and adds `retry_after_seconds`; the report route adds `serious` and `emergency_number` (UI-SPEC P4-A13). Distinct `field` values (`rate_report`, `rate_vote`, `rate_vote_change`, `rate_flag`) select among the seven messages. Messages never name numbers or say "too many requests". Add a swagger `RateLimitResponse` and 429 entries on the four vote routes, `POST /reports` and the flag route. Log every block (bucket, numeric account id, wait; never text or email).
6. **IP backstop.** The router uses `middleware.ClientIPFromRemoteAddr` for everything [VERIFIED: codebase router.go line 153]. Add `Deps.ClientIPHeader`: empty keeps today's middleware, non-empty installs `middleware.ClientIPFromHeader(header)`; both fail closed [VERIFIED: module source]. **The new limiters skip the IP check when `middleware.GetClientIP` is empty or the setting is unset, and never fall back to `r.RemoteAddr`** (behind Render, Railway or Fly that is one shared proxy bucket). `PerIP.Middleware` keeps its own fallback, so `TestRequestLinkLimiterIgnoresSpoofedProxyHeaders` stays green. Header names per host stay a deploy canary [CONTEXT].
7. **Store: memory.** Restart or free host sleep forgets buckets. Consequence: little, because a 15 minute idle sleep already refills a normal bucket [ASSUMED] and Remove is the backstop (D-11).
8. **R-10.** Seeded and demo reports are inserted with raw SQL (`internal/testutil/seed.go`, the store tests), never through `Submit`, so they skip limiter and provider by construction. Test: a seeded report is `moderation_state = clear` with no `moderation_checks` row.

### Landmines
- Existing e2e suites build `api.Deps` without the new fields and post a few reports and votes from one account. Defaults apply, so only tests that cast two votes of one kind on one report from one account within 30 s (reporter resolve then reopen, vote changes) get a 429. Give the e2e helpers `Deps` knobs (generous budgets or an injected clock) and keep one default-budget test per bucket.
- `CancelAt(reservation instant)` restores the token; a later `Cancel()` or `CancelAt` restores nothing for a zero delay grant [VERIFIED: x/time v0.16.0 rate.go `CancelAt` and its `TestCancelTooLate`]. Concurrent same-key requests can over-credit slightly (bounded by the burst) or lose a refund; a `-race` concurrency test asserts admission never exceeds the burst plus the elapsed refill.
- Do not limit `GET /api/reports` (polls every 30 s, Low data every 60 s).

## Area D: Low data view (ROBUST-01, D-13 to D-17, R-08, R-09)

### Integration points
| What | Where | Note |
|------|-------|------|
| Route | `GET /low` in the gated group, `handlers.LowPage(tmpl, cfg)` | View model: the `pageViewModel` fields (`FallbackLat`, `FallbackLon`, `DefaultRadiusKm`, `AssetVersion`, `Email`) plus `AccountID` and the shared chrome data |
| Shell | `app.js` reads `#app-shell` data attributes with Bengaluru fallbacks [VERIFIED: codebase app.js lines 49 to 71] | The page needs an element with id `app-shell` carrying them (poll 60000) |
| Feed boot | On the main page only the first `fetchReports` call and `startPolling` come from `map.js` `centerOnVisitor` (setCenter, fetchReports, startPolling after `getCurrentPosition` resolves, fails or times out) [VERIFIED: codebase map.js lines 133 to 157]. `Pinalert.fetchReports()` is also called by `feed.js` (retry button, show disputed toggle, pageshow restore), by `votes.js` after every successful vote or resolve (inside the shared vote click handler) and by `submitReport` in `app.js` | `low.js` calls `Pinalert.setCenter`, `fetchReports`, `startPolling` itself, stored position first (D-16). The Low data page loads `votes.js` but not `feed.js`, so every vote there refetches through the shared store: `low.js` must subscribe (`Pinalert.subscribe`) and have called `Pinalert.setCenter` before the first vote can happen |
| Scripts, rows | Order in UI-SPEC Hand Off Notes; `low.js` own row function using the shared builders (`PinalertTrust.createTrustBlock`, `PinalertVotes.createVoteBlock(id, { resolve: false })`, visibility tag) | R-09; no unpkg tag, no `feed.js`, `map.js`, `modal.js` |
| Redirects | `Page`: bare `/` with cookie `pinalert_low_data=1` and no query string gets 302 to `/low`, `Cache-Control: no-store`; `/?full=1` clears the cookie. `gate.go`: a logged out `/low` redirects to `/login?low=1` (the existing test pins `GET /` to exactly `/login`; only `/low` differs) | `LoginGate` sets the cookie server side on `low=1`; not HttpOnly, SameSite=Lax; list it on the Policy page |

### The form extraction is the riskiest refactor
`modal.js` is 848 lines [VERIFIED: codebase]. Its form logic shares closure state with the Leaflet pieces: `validate()` requires `modalMarker`, `buildPayload()` reads `modalMarker.getLatLng()`, `initLocation` returns early when `L` is undefined. Contract tests pin literal text in it (single `L.marker(`, `placeMarker(`, `modalMap.setView(`, the one missing location sentence; `TestLocationSearchUsesTextSinksAndExistingPinPlacement`, `TestIconGlyphsAreClassDriven`). Recommendation: its own plan, a pure refactor with tests re-pointed per UI-SPEC "Tests that change" and no main page behaviour change, merged green BEFORE Low data uses it. Fallback if the budget forces a cut: a smaller `low-form` duplicating only the five fields and calling `Pinalert.submitReport`; it contradicts the approved UI-SPEC hand off, so it needs the owner's word.

### Size budget (D-14)
Shipped files, raw bytes (gzip -9) [VERIFIED: codebase]: `app.js` 11,915 (4,561), `votes.js` 24,045 (7,956), `visibility.js` 7,730 (3,041), `account-menu.js` 3,906 (1,618), `theme.js` 7,598 (3,157); `main.css` 30,552 (9,254), `feed.css` 2,501 (1,259), `trust.css` 24,415 (8,731), `auth.css` 18,005 (5,440), `modal.css` 11,417 (3,676). That is about 142 KB raw, 49 KB gzipped before `trust.js`, `queue.js`, `flag.js`, `report-form.js`, `low.js`, `chrome.css`, `low.css`; a realistic total is 200 to 230 KB raw [ASSUMED estimate]. **The router installs no response compression** [VERIFIED: codebase], so a gzipped budget means nothing. Recommendation: test the budget in raw bytes of every local asset the rendered page links (HTML, JS, CSS, icon files its CSS references), limit set at measured plus about 15 percent after the page exists (starting guess 256 KB), and add chi `middleware.Compress(5, ...)` for html, css, javascript, svg, json (built in, no dependency). See Open Question 2.

### Other landmines
- With unpkg blocked, the main page list never loads: `map.js` `init` returns when `typeof L === 'undefined'` and nothing else fetches. The D-17 slow message sends people to Low data; decoupling the main feed from Leaflet is the optional UI-SPEC item 26 change and touches Phase 3 pinned files, so do it last or not at all.
- `feed.js` `render` hides the list on `state.status === 'error'` [VERIFIED: codebase lines 297 to 305]; D-20 and P4-A19 change it to a stale line. No existing test reads the body of `render`; the tests that constrain edits to `feed.js` are `TestFeedExplainsAnEmptyDisputedResult` (exactly one `setHidden(emptyEl` and one `setHidden(disputedEmptyEl`, both inside `setEmptyState`), `TestShowDisputedUsesOneSharedQueryParam` (zero `fetch(`, one `show_disputed`), `TestVoteClickDoesNotActivateItsRow` (at least three `activateRow(id)`) and `TestFeedRefetchesOnBackForwardCacheRestore`, so the stale line change in `render` needs its own new tests.
- Pinned counts: `app.js` has exactly two `fetch(` and one `show_disputed` (`TestShowDisputedUsesOneSharedQueryParam`); `votes.js` has exactly one `fetch(` (file wide, `TestVoteTransportHasNoLocationFallback`) and its one `getCurrentPosition(` is pinned only as living inside `getVoterLocation()` after the `sessionStorage.getItem(` call (`TestVoterLocationIsCachedPerSession`); `feed.js` zero `fetch(` [VERIFIED: codebase]. Queue sends call `Pinalert.submitReport`; 429 handling goes inside `castVote`'s fetch; Low data voting reuses `getVoterLocation`.
- Typed place search shares the process wide geocoder limiter (1 per second) and the per-IP `GeocodeRateLimit` (burst 3, 1 per 2 s), which keys on the proxy address until `ClientIPHeader` is set; add to the launch checklist.
- Keep the two banned words of D-13 out of every new identifier and string (UI-SPEC "New tests" scan; note `aria-live="polite"` contains one as a substring).

## Area E: Offline queue (ROBUST-02, D-18 to D-20, R-05)

### Server contract (where "never inserts twice" is won)
- `SubmitReportRequest` decodes with `DisallowUnknownFields` [VERIFIED: codebase], so add `request_id` and `written_at` explicitly (swag comments, examples). `request_id` pattern: letters, digits, hyphen, 16 to 64 characters.
- **One statement insert** `InsertReportIdempotent ... ON CONFLICT (request_account_id, request_id) WHERE request_id IS NOT NULL DO NOTHING RETURNING ...` (Code Examples). No row back means a conflict: run an account scoped `ReportByRequest` and answer 200 with the stored report (UI-SPEC: a repeated id is treated as sent). The conflicting insert waits for the first transaction, so concurrent retries serialise; no transaction and no orphan row (a "NOT EXISTS then insert" CTE would race).
- **Replay lookup is scoped to the caller's account**, from `account.FromContext`, never the body; a global lookup would return another account's report for a reused id.
- **Key is the account, not the session**: one account holds several sessions (Phase 1.1 D-10) and a re-login creates a new one, so a session key would double insert a queued report. `reports.session_id` stays the identity column (Phase 3 T-03-10 keys reliability on `sessions.account_id`); `request_account_id` serves only this index.
- Check the replay BEFORE reserving any allowance or calling the provider.
- `written_at`: RFC 3339; future values clamp to now; older than a stated bound (suggest 14 days) stores NULL, never rejects. It never feeds `created_at`, `expires_at`, ordering or the age stage (R-05, D-19); `Submit` keeps server stamping [VERIFIED: codebase report.go lines 312 to 313]. The server sends `written_at` only when `created_at - client_written_at` exceeds 30 minutes (UI-SPEC item 17), through the extras read and `ActivityForAccount`.
- `ReportResponse` gains `held bool`; `FeedReportResponse` gains optional `written_at` and the viewer flagged marker. Regenerate swagger after Phase 3's last regeneration; the substring bans (`session_id`, `reporter_account_id`) stay satisfied.

### Client (`queue.js`)
- `activity.js` must keep exactly one `addEventListener`, no `fetch(` and one `Pinalert.showToast(` (`votes_contract_test.go` lines 2076 to 2114) [VERIFIED: codebase], so all strip listeners live in `queue.js`, mounted by one call.
- Plain IndexedDB in try and catch with an in memory fallback; `navigator.storage.persist()`; keep the strip visible (the stated mitigation for Safari eviction after about 7 days [CONTEXT]).
- Record `{ request_id, account_id, payload, written_at, status, attempts, next_try_at, last_error, serious }`, key path `request_id`, index on `account_id`; `account_id` from server rendered `data-account-id` (never the email); flush only the current account's rows.
- Decide by real request outcomes, not `navigator.onLine`. `AbortSignal.timeout(10000)` with a feature detect and an `AbortController` fallback; accept `TimeoutError` and `AbortError` [CONTEXT]. Queueable outcomes: UI-SPEC item 6 (no answer, timeout, 502, 503, 504).
- `crypto.randomUUID` needs a secure context [CONTEXT]: fallback id generator via `crypto.getRandomValues`, last resort `Math.random`, matching the server pattern.
- Two tabs: Web Locks around the flush where available [ASSUMED support]; the unique index is the real guard.
- "Send after the wait" honours the 429 `retry_after_seconds`; `Pinalert.submitReport` passes `held` through (P4-A13). `showToast` is fixed at 3000 ms [VERIFIED: codebase app.js line 322] while UI-SPEC wants 6 s for three toasts: add an optional duration to the single implementation (`TestToastHasExactlyOneImplementation`).

## Area F: Disclaimer bar and legal pages (ROBUST-08, D-21 to D-24, R-07, R-08)

| What | Where | Note |
|------|-------|------|
| Bar partial | `web/templates/disclaimer_bar.html.tmpl`, called by every full page | `findFullPageTemplates` treats any file containing `<html` as a full page and requires `theme.js` once in the head with `?v={{.AssetVersion}}` [VERIFIED: codebase]. A partial must not contain that text, even in a comment |
| One template set | `ParsePageTemplate` parses `templates/*.tmpl` into ONE set [VERIFIED: codebase page.go line 57] | A shared partial runs with each page's own view model and `html/template` fails at execute time (a 500) on a missing field. Embed one `Chrome` struct (asset version, numbers, contact flag) in `pageViewModel`, `loginGateViewModel`, `verifyOutcomeViewModel`, `profileViewModel` and the new models; test-render every full page. No template func (`ParsePageTemplate` has none; tests call it directly) |
| Numbers | One struct (primary, older list, state helpline pair), env or constants, defaults 112, 100 101 108, 1070 1077 | D-22: bar, Disclaimer, the 429 `emergency_number` and the `tel:` href all read it. Empty older list drops the parenthetical and the row. Never present 1070 or 1077 as national |
| Routes | `GET /disclaimer`, `GET /policy` with the exempt routes in `router.go` | Update the router doc comment ("The exempt set is exactly ...") and `TestAccessGateExemptRoutes` (five cases today) [VERIFIED: codebase] |
| Contact | `CONTACT_EMAIL` env; block renders only when set (D-24) | README launch checklist (lines 79 to 97 area): "set the contact email", "check the numbers on the live government pages". Do not invent an address |
| Caching | The session middleware wraps every route and sets `pinalert_session` plus a `sessions` row on any first visit [VERIFIED: codebase session/cookie.go lines 96 to 120] | A cookie setting response must not sit in a shared cache: `Cache-Control: private, max-age=300` (R-07 only says "may be cacheable"). Policy cookie list says visiting these pages sets the session cookie. Crawler visits make session rows already today for `/login` |
| Dash gate | `TestUserVisibleCopyUsesPlainPunctuation` scans every `.tmpl` line for U+2014 and U+2013 [VERIFIED: codebase] | The long legal templates must be fully dash free |
| Policy facts | No `DELETE` statement exists in any query or migration [VERIFIED: codebase grep] | Nothing is ever deleted: reports, votes, sessions, tokens persist, and expiry only hides reports. The Policy page must say so or Phase 4 adds a retention rule (Open Question 5). Precision 7 cell is about 153 m [VERIFIED: codebase trust.go comment] |
| Geometry | `--disclaimer-bar-height`, four fixed map controls | `TestThemeToggleFloatsClearOfTheReportButton` and the Phase 3 popup max height test change (P4-A2 to P4-A4) |
| R-07 test | Render both pages with no account | No email, no account header markup |

## Phase 3, 3.1, 3.2 collisions

All 20 Phase 3 plan files now exist on disk (03-16 and 03-19 were not yet committed when this was checked); the rows below were derived from `03-PLAN-OUTLINE.md` ("Cross-plan contracts", "File ownership", pinned tests) before plans 03-14 to 03-20 were written, so re-check each row against the written plans' `files_modified` lists and their "Artifacts this phase produces" sections. Phases 3.1 and 3.2 have empty directories and ROADMAP scope notes only. None of Phase 3, 3.1 or 3.2 is built, so re-grep the built code before executing. [VERIFIED: codebase, outline, plan files]

| # | Area | Earlier phase artifact | Phase 4 touch | Resolution |
|---|------|------------------------|---------------|------------|
| 1 | Migration numbers | `00005_add_reports_session_expires_index.sql` (03-06); 3.1 and 3.2 likely add more (3.2: updates, mutes, cap bookkeeping) | New migrations | Take the next free number at execution time (`ls internal/store/migrations`). Never write "00006" in a plan |
| 2 | sqlc output | 03-01 edits `votes.sql` (`ReportVoteContext` gains latitude, longitude, created_at); 03-06 adds `reliability.sql`; 3.1 adds history queries | Edits `reports.sql`, `votes.sql`, adds `moderation.sql` | Run `make sqlc` after those phases landed, commit generated files (CI has no sqlc). Read the committed structs, not names quoted here |
| 3 | `CastVote` order | 03-01 pins steps 1 to 7 (`TestCastVoteCheckOrder`); 03-05, 03-12 edit `trust.go` | Withheld refusal right after the context read | Add the step, amend that test, list it as a changed test. Keep limiter code out of `trust.go` |
| 4 | Querier interfaces and fakes | 03-12 adds `ReporterHistoryReports` to `Querier`, `VotingQuerier`, `AuthQuerier` and the three fakes | Adds `ReportExtrasForReports` to `Querier` and `AuthQuerier` and their two fakes only (`CastVote` reads moderation through `ReportVoteContext`) | Sequential. Add a once-per-page, zero-on-empty call count like `TestNearbyBatchesVoteReadsOnce` [VERIFIED: codebase feed_test.go lines 450 to 483] |
| 5 | `Resolve` callers | 03-03 `ClassifyOutcome` with neutralised metadata; 3.1 view queries; 3.2 "call the single resolver" | Moderation in `ReportMeta`, new rung | Zero value `clear` keeps neutral calls right (R-02); source guard test with comments stripped and `reliability.go` (03-03's one neutral call) allowlisted by name, see Moderation state model item 7; 3.1 views must drop `withheld` |
| 6 | View partition | 3.1: Active, Disputed, Resolved, Past, `show_disputed` becomes `view` | Fifth `Visibility` value | One test loops over every `Visibility` value and every view: `withheld` is in none |
| 7 | Trust object | 03-05 `BuildTrust`, `ActivityReport.Trust` on every row; 03-15 `data-trust` | Held and removed Activity rows omit trust | Skip `BuildTrust` and `TrustJSON` when moderation is set |
| 8 | `reports.go`, `votes.go` handlers | 03-07 adds `Trust` to both responses; 03-04 adds the 403 too far branch | Request fields, `held`, `written_at`, replay, 429 annotations | Edit after Phase 3; regenerate swagger once at the end |
| 9 | Swagger guards | 03-07, 03-14: name guard `(?i)cell\|account\|session\|email` over `TrustResponse` and friends | New documented fields | Never document an account field; `request_id`, `held`, `emergency_number` pass |
| 10 | `votes.js` | 03-04 retry (the fetch moves into a nested `postVote(coords)` inside `castVote`), 03-09 `createVoteBlock(reportId)` returning `{ controls, note, error }`, 03-18 hint, 03-17 last (return shape comment and the deleted limitation comment); pins one `fetch(`, one `getCurrentPosition(` | 429 handling, `createVoteBlock(reportId, options)` | Handle 429 inside the existing `castVote` fetch; move the anchor as UI-SPEC lists |
| 11 | `feed.js`, `map.js` | 03-09, 03-13, 03-16, 03-17 `buildPopupCard`, 03-19 focus work | Flag control, stale line, popup flag, autopan reserve | Build the Flag control once per card, update in place; pin the two `stopPropagation` listeners |
| 12 | `activity.js`, `visibility.js` | 03-15: one `addEventListener`, no `fetch(`, one `showToast(`; `displayState` | `dataset.moderation`, strip, held and removed states | Strip listeners live in `queue.js`; extend `displayState` with the P4-A9 precedence |
| 13 | CSS | 03-11/13/16/17 own `trust.css`; 03-16/17 edit `main.css` | Bar token, popup max height (P4-A4), new sheets | `:not([hidden])` on every new `display` rule of a hidden capable element. `TestVoteBlockHiddenGuard` (`web/votes_contract_test.go`) walks every CSS file but inspects only selector heads containing `.vote-error`, `.vote-controls` or `.vote-btn`, and `TestAccountMenuHiddenGuard` (`web/css_contract_test.go`) inspects only the account menu; Phase 3 plan 03-11 adds its own test for `.vote-note` and `.resolve-confirm`. Phase 4 must add its own walker test over its new selectors (`.flag-control`, `.queue-strip`, `.notice-card`, the Low data and moderator classes); no hex or `rgb(` |
| 14 | `index.html.tmpl`, `page.go`, `main.go` | 03-13 `trust.js` tag; 03-18 `data-vote-radius-km`, `PageConfig.VoteRadiusKm` | Head order, bar, form partial, Chrome embed, constants | Sequential; keep `trust.js` in Phase 3's script position |
| 15 | `handlers/auth.go`, `profile.html.tmpl` | 03-15 `TrustJSON`, `SeverityWord`, `Expired` | `data-moderation`, `data-account-id`, strip | `data-account-id` only in the owner's own HTML |
| 16 | Router and URL names | 3.1 `view`, `report`, new read routes; 3.2 bell next to the account icon (the account trigger lives in `.app-header` today; `.top-chrome` does not exist yet, it is Phase 4's own wrapper from UI-SPEC P4-A1, which moves `.app-header` inside it; see the UI-SPEC Hand Off cross phase notes for the bell's final position), update routes, menu items | `/low`, `/moderate`, `/api/moderation/*`, `/disclaimer`, `/policy`, query `full`, `resend` | Phase 4 never reads `view` or `report` server side; Log out stays last in the account menu |
| 17 | `app.js` `fetchReports` | 03-16 adds `SEVERITY_WORDS` and `formatShelterLine` and extends the export object; 3.1 rewrites `fetchReports` for `view`; today pins `show_disputed` once and `fetch(` twice | `hasLoaded` rule, 10 s `AbortSignal.timeout`, `data-feed-requested` | Plan against the built file; keep two `fetch(` |
| 18 | `testutil.Truncate` | 03-06 adds `testutil/history.go` | New tables | List new tables explicitly [VERIFIED: codebase db.go] |
| 19 | Docs and README | 03-02, 03-08, 03-10, 03-20 own `docs/TRUST-MODEL.md`; 03-02, 03-20 edit README | Moderation doc, launch checklist | New `docs/MODERATION.md`; append to README after 03-20 |
| 20 | 3.2 updates | "Removed after review" may become an Update | none | Keep a readable removed transition (`moderation_actions`); no 3.2 code here |
| 21 | Copy tests | 03-05 `assertPlainCopy` for trust strings | New server strings (apostrophes, the 112 sentence) | Write Phase 4's own Go copy test (UI-SPEC lists it) |

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token bucket arithmetic | A counter plus timestamp map | `golang.org/x/time/rate` `ReserveN` and `CancelAt` at the reservation instant | The project standard; refunds and wait times come for free, but only if the cancel uses the reservation instant |
| Status capture for refunds | A custom ResponseWriter wrapper | `middleware.NewWrapResponseWriter` | Handles flusher and hijacker interfaces |
| Client address | Parsing `X-Forwarded-For` | `middleware.ClientIPFromHeader` | Fail closed and last value wins; trusted header only |
| Idempotency | Check then insert | `INSERT ... ON CONFLICT DO NOTHING RETURNING` on a partial unique index | Concurrent retries race otherwise |
| Concurrent worker claims | Advisory locks or an external queue | `FOR UPDATE SKIP LOCKED` with a lease column | One statement, no new dependency |
| Moderation classification | A word list for hate or abuse | The provider scores plus the small documented gibberish rule | D-05 and D-06; word lists fail on Indian languages |
| Email list matching | Ad hoc lowercase | The same normaliser sign in uses | Same identity must match |
| Compression | Hand written gzip | `middleware.Compress` | Content type allowlist built in |
| Storage layer in the browser | A wrapper library | Plain IndexedDB with a try and catch fallback | CONTEXT: no libraries, no build step |
| Time formatting of waits | New relative time code per surface | One shared Wait Format function (UI-SPEC "Wait format") | Seven messages must agree |

**Key insight:** the hard parts here are not libraries, they are ordering (screen before store, replay before reserve, reserve after validate, refund on non-2xx) and one source of truth (`Resolve`, one numbers setting, one `Serious()` predicate).

## Common Pitfalls

| # | Pitfall | Cause | Avoid | Check |
|---|---------|-------|-------|-------|
| 1 | A rejected or invalid attempt burns the report allowance | Reserving before validation and screening, or a refund that silently does nothing (`Cancel()` and `CancelAt(later t)` are no-ops for a zero delay grant) | Reserve after validation; refund with `CancelAt(reservation instant)` on reject, prefilter reject, any non-2xx and replay; provider protected by the attempt bucket and provider budget | Test with the injected clock advanced between Reserve and Cancel: one rejected text then a valid report within the burst succeeds |
| 2 | Re-cast cooldown breaks Phase 3's silent retry after a 403 too far | The refused attempt consumed the 30 s token (or the refund was a no-op, see pitfall 1) | Cancel the cooldown reservation at the reservation instant on every non-2xx; the 20 burst bucket is not refunded | Test with the clock advanced between Reserve and Cancel: refused vote then valid vote succeeds; two successes within 30 s give 429 |
| 3 | Moderation read added to `NearbyReports` or per report | Convenience | One `ReportExtrasForReports` per page, never on an empty page | Existing drift const and round trip tests stay green |
| 4 | Pending state visible for a moment | Insert `clear`, update later | Decide state before insert; write `moderation_state` in the same `INSERT` | Test with a fake provider that times out: row is `pending` at first read |
| 5 | Moderator route discovery | Route inside the gate gives 302 or 401; a wrong method gives chi's 405 plus `Allow` before any group middleware runs | Own gate, 404 for everyone else; register the moderator paths with `r.Handle` (all methods) inside the gate group | Byte equality with an unknown path, including wrong methods (HEAD, PUT, GET on POST routes, POST on `/moderate`) |
| 6 | New visibility relabelled as Unconfirmed | `newProfileReport` falls back to `provisional` when `Valid()` fails | Add the value to `Visibilities`; Activity label from `data-moderation` | Table test over every value |
| 7 | Byte versus rune counting | `len(desc)` today | `utf8.RuneCountInString` (R-06) | Boundary fixtures at 9, 10, 1000, 1001 in English and Telugu |
| 8 | Indic text exhausts 10,000 tokens a minute | Roughly a token per character [ASSUMED] | Provider budget on estimated tokens; pending is the designed degradation | Fake provider returning 429 with Retry-After |
| 9 | Idempotency scoped wrongly | Global key leaks reports; session key double inserts after re-login | Unique on `(request_account_id, request_id)`, account from the gate only | Two accounts, same id: two rows; one account twice: one row |
| 10 | Existing e2e suites hit new limits | Vote re-cast within 30 s in old tests | `Deps` knobs (generous budgets, injected clock) in the e2e helpers; one default-budget test per bucket | Whole existing suite green |
| 11 | Shared bar partial 500s a page | A view model lacks the field; `html/template` fails at execute time | Shared `Chrome` embed plus a render test of every full page | Test derived from the embedded filesystem |
| 12 | Legal pages set cookies and get cached | Session middleware wraps every route | `Cache-Control: private`; Policy lists the cookie | Header test |
| 13 | Links in report text | D-05 allows them | Every sink `Pinalert.setText`; moderator screen shows plain text | Scan over all `static/js` |
| 14 | `AbortSignal.timeout` or `crypto.randomUUID` absent | Old Safari, plain http LAN | Feature detect with the CONTEXT fallbacks | Contract test for the detects |
| 15 | Cookie redirect loop | `/` to `/low` and back | "Open full version" is `/?full=1`, clears the cookie; any query string is never redirected | `GET /?full=1` and `GET /?view=resolved` with the cookie return 200 |

## Code Examples

### Reserve and refund (Area C)
```go
// internal/ratelimit/perkey.go (new). Source: golang.org/x/time/rate Reservation API, module source v0.16.0.
type Decision struct {
	OK         bool
	RetryAfter time.Duration // zero when OK
	Cancel     func()        // refunds the token
}

func (p *PerKey) Reserve(key string) Decision {
	now := p.now()
	r := p.limiterFor(key, now).ReserveN(now, 1)
	if !r.OK() {
		return Decision{RetryAfter: p.every}
	}
	if d := r.DelayFrom(now); d > 0 {
		r.CancelAt(now) // a reservation we will not use
		return Decision{RetryAfter: d}
	}
	// Cancel at the SAME instant as ReserveN: a later Cancel() or CancelAt(p.now()) is a silent no-op
	// for a zero delay grant (x/time rate.go CancelAt returns early on r.timeToAct.Before(t); see its TestCancelTooLate).
	// CancelAt is not idempotent, so wrap it in sync.Once.
	var once sync.Once
	return Decision{OK: true, Cancel: func() { once.Do(func() { r.CancelAt(now) }) }}
}
```

### Handler order for POST /api/reports (the order is the specification)
1. decode (`DisallowUnknownFields`, new fields `request_id`, `written_at`); 2. `ValidateSubmitInput` (runes), a 400 costs nothing; 3. replay lookup by (account, `request_id`), 200 with the stored report, no limiter, no provider; 4. submit-attempt bucket (never refunded); 5. allowance `Reserve`, serious bucket when `meta.Serious()`; 6. `SubmitChecked`: prefilter, provider (3 s, one attempt), insert with the decided `moderation_state`; reject or error cancels the allowance; 7. 201 with `held`.

### Idempotent insert and account scoped replay (Area E)
```sql
-- name: InsertReportIdempotent :one  (VALUES list as InsertReport plus moderation_state, request_id, request_account_id, client_written_at)
ON CONFLICT (request_account_id, request_id) WHERE request_id IS NOT NULL DO NOTHING
RETURNING id, category, severity, description, latitude, longitude, geohash,
          shelter_capacity_status, shelter_headcount, created_at, expires_at, moderation_state;

-- name: ReportByRequest :one  (run when the insert returned no row)
SELECT ... FROM reports WHERE request_account_id = sqlc.arg(account_id) AND request_id = sqlc.arg(request_id);
```

### Moderator gate with its own lookup (Area B)
```go
row, err := q.GetAccountBySessionID(r.Context(), sid)      // unverified or error
if err != nil { http.NotFound(w, r); return }               // same bytes as an unknown route
if _, ok := allowed[row.Email]; !ok { http.NotFound(w, r); return }
```

Register every moderator path inside the gated group with `r.Handle` (all methods), never `r.Get` or `r.Post`, so a wrong method is answered by this gate and not by chi's 405 with an `Allow` header. A real moderator who uses the wrong method gets the handler's own 405.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Quota versus rate limit 429s are told apart by an error `code` field (`insufficient_quota`) | Area A | Retry storms or a held backlog never released; mitigated by classifying on status and backoff |
| A2 | Indic text costs about one token per character, so 10,000 TPM binds near 10 long reports a minute | Area A | Budget numbers too tight or too loose; measure with the recorder |
| A3 | Starting thresholds reject 0.85, borderline 0.40 | Area A | Genuine reports rejected; the recorded fixture is the gate |
| A4 | `CancelAt(reservation instant)` restores the token; later cancels restore nothing [VERIFIED: x/time v0.16.0 rate.go and its `TestCancelTooLate`]; concurrent same-key requests can over-credit slightly or lose a refund | Area C | Over-admission by a few requests or a missed refund; the concurrency test asserts bounds in both directions |
| A5 | Free host sleeps after about 15 minutes idle, so memory limiter loss is harmless | Area C | A restart gives an attacker a fresh burst; Remove is the backstop |
| A6 | IP backstop numbers (60 burst, 1 per 20 s) | Area C | Too loose or blocks CGNAT users; tune from the block log |
| A7 | `moderation_actions` audit table is wanted | Moderation model | Extra scope; cheap to drop |
| A8 | Web Locks API and `AbortSignal.timeout` availability on target browsers | Area E | Feature detects with fallbacks cover it |
| A9 | Gibberish run length of 10 and "fewer than two tokens of two letters" | Area A | False rejects of terse genuine text such as "Help!!!! flood"; fixture covers short texts |
| A10 | `written_at` bound of 14 days | Area E | Older queued reports show no "Written" label; harmless |
| A11 | Planner or owner confirm the exact Flag reason slugs | Area B | Wire contract mismatch with UI-SPEC |
| A12 | Size estimate 200 to 230 KB raw after the build | Area D | Budget set wrongly; set the number after building |

## Open Questions

None block planning. Each has a recommendation.

1. **Does a text reject (or a 400) consume the report allowance?** No: reserve after validation, cancel on reject, protect the provider with the attempt bucket and provider budget (Area C).
2. **Compression and the Low data budget.** Router sends everything uncompressed. Add chi `middleware.Compress` (no dependency) and test the budget in raw bytes; if the owner skips compression the raw budget alone stands.
3. **Fixture recording needs the owner's key.** Build the recorder and a `Decide` test on recorded scores; make "run the recorder with a fresh zero balance key" an end-of-wave human check.
4. **Moderator reach.** Keep UI-SPEC item 20's default: no link, the owner bookmarks `/moderate`.
5. **Retention and the Policy page.** Nothing is ever deleted today. No retention job in Phase 4; the Policy page says so and offers deletion through the contact line. Needs the lawyer or mentor read before promotion.
6. **JS logic tests.** Go source tests cannot run `queue.js`. One optional `node --test` file with a hand written fake IndexedDB covers flush order, account binding and never-discard; if declined, rely on the UI-SPEC manual checks plus the Go replay test.
7. **`Mode: mvp` guard, ROADMAP and REQUIREMENTS edits** belong to the orchestrator (CONTEXT planner notes).
8. **Contact email and emergency numbers** are launch checklist items, not code blockers (D-22, D-24).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | all | yes | go1.26.4 darwin/arm64 (go.mod requires 1.26.4; CI file says 1.25) | none needed |
| PostgreSQL client tools (`psql`, `createdb`, `dropdb`) | disposable test databases | yes | 16.14 (Homebrew) | none |
| sqlc | regenerate `internal/store/sqlc` | yes | v1.31.1 (`~/go/bin/sqlc`) | none; CI has no sqlc, generated files are committed |
| swag | regenerate `docs/` | yes | v1.16.4 | none |
| goose CLI | optional | yes | present | `cmd/migrate` and `testutil.NewTestDB` apply embedded migrations without it |
| Node and npm | optional `node --test` for `queue.js` | yes | Node v26.4.0 | skip the JS logic test |
| OpenAI API key (free, zero balance) | recorder, live verification | not checked (secret) | n/a | recorder is opt in; tests never need it |
| Running Postgres server | DB backed tests | not probed (do not touch the owner's databases) | n/a | tests skip when `DATABASE_URL` is unset |
| Hosting (Render, Railway, Fly) and proxy header | IP backstop | undecided (PROJECT.md) | n/a | empty `CLIENT_IP_HEADER` means no IP limit on the new limiters |

**Missing with no fallback:** none for coding. **Human checks:** zero balance OpenAI key, live government number pages, lawyer or mentor read of the legal pages, host canary for the IP header.

## Validation Architecture

> `workflow.nyquist_validation` is true. Repo rules bind every command: never run against `pinalert_test` or `pinalert`, start no server on 8080 or 8090.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (go1.26.4). Package `web` contract tests parse embedded files (no browser). e2e tests drive `api.NewRouter` over `httptest` against real Postgres via `testutil.NewTestDB` |
| Config file | none; `sqlc.yaml` for codegen; `Makefile` targets `test`, `test-short` |
| Quick run (no database) | `env -u DATABASE_URL go test ./... -count=1` (the `env -u` prefix is mandatory: if the owner's shell exports a URL pointing at `pinalert_test`, a bare run would truncate it; every database backed test skips without `DATABASE_URL` through `testutil.NewTestDB`, and `./...` never fails on a package that does not exist yet) |
| Database form (per plan N) | `dropdb --if-exists p4_04NN; createdb p4_04NN && DATABASE_URL="postgres://localhost:5432/p4_04NN?sslmode=disable" go test <pkgs> -run '<regex>' -count=1 -p 1; status=$?; dropdb --if-exists p4_04NN; exit $status` (Phase 3's form; `NewTestDB` applies embedded migrations) |
| Race | add `-race` to the database form for limiter and idempotency tests |
| Phase gate | database form over `./...` `-p 1` on scratch database `p4_04full`, dropped afterwards, then the UI-SPEC V-42 manual script on port 8792 with `ENV=development` |

"db" in the table below means the database form with the package shown; "quick" means the quick run command. A cell written `./pkg/ tests: A, B, C` lists a package and test names (a cell with no package means the whole tree): build the flag as `-run 'A|B|C'` with a plain `|` (never `\|`, which Go's RE2 reads as a literal pipe and then runs zero tests). Example: `-run 'TestDecide|TestGibberish|TestRejectMessages'`. After any `-run`, confirm the output lists tests and does not say "no tests to run".

### Phase Requirements to Test Map
| Req | Behaviour | Type | Command | Exists |
|-----|-----------|------|---------|--------|
| ROBUST-03 | `Decide`: scores to pass, borderline, reject; violence and self harm never reject; serious still rejected for abuse; gibberish on runes; four messages | unit | quick, `tests: TestDecide, TestGibberish, TestRejectMessages` | Wave 0 |
| ROBUST-03 | Fixture on recorded scores: no genuine rejected, no serious withheld, abusive set rejected | unit | quick, `-run TestScreeningFixture` | Wave 0, scores are a human step |
| ROBUST-03 | Client: request shape, only trimmed text sent, Retry-After, 429 and 5xx transient, 4xx permanent, timeout (fake server) | unit | quick, `./internal/moderation/` | Wave 0 |
| ROBUST-03 | Submit: reject stores nothing; pass clear; borderline clear plus `needs_review`; timeout pending; pending non-critical in no feed; pending critical published; `held` in 201 | e2e | db, `./internal/api/handlers/ -run TestSubmitScreening` | Wave 0 |
| ROBUST-03 | Worker: claim, backoff, Retry-After, late reject removes, late pass releases, expired not retried | integration | db, `./internal/service/ -run TestModerationWorker` | Wave 0 |
| R-01 to R-03 | Rung order (removed above bypass), zero value neutral, classifier neutral, grep guard, `withheld` in no view | unit | quick, `tests: TestResolveModeration, TestWithheldInNoView, TestResolveCallersMentionModeration` (the caller guard strips comments and allowlists `reliability.go` by name) | Wave 0 |
| R-06 | Rune counting at 9, 10, 1000, 1001, English and Telugu | unit | quick, `-run TestValidateSubmitInputCountsRunes` | amend `report_test.go` |
| ROBUST-04 | `PerKey`: burst, refill on injected clock, key independence, refund (cancel at the reservation instant, with the clock advanced between Reserve and Cancel, plus an interleaved reserve, deny, cancel case), idle window per bucket never hands back a full burst early, concurrent bound | unit | quick with `-race`, `./internal/ratelimit/` | Wave 0 |
| ROBUST-04 | Serious and other buckets separate; 429 header and body fields; reject, 400 and replay do not consume | e2e | db, `./internal/api/handlers/ -run TestReportRateLimit` | Wave 0 |
| ROBUST-04 | IP backstop skipped without address or header setting; spoof test green | e2e | db, `./internal/api/ tests: TestRequestLinkLimiterIgnoresSpoofedProxyHeaders, TestIPBackstop` | amend |
| Vote throttle | 21st vote 429; cooldown 429 with its own message; refused vote leaves the cooldown intact; four routes; swagger 429s | e2e | db, `./internal/api/handlers/ tests: TestVoteRateLimit, TestSwaggerSpecCoversRoutes` | `TestVoteRateLimit` Wave 0; `TestSwaggerSpecCoversRoutes` exists (`internal/api/handlers/swagger_test.go`, needs no database) and is amended to require the 429 responses |
| ROBUST-02 | Replay once, 200 second time; concurrent twice one row; other account same id own row; `written_at` clamped, never in expiry; sent only past 30 min | e2e | db with `-race`, `./internal/api/handlers/ tests: TestSubmitIdempotent, TestWrittenAt` | Wave 0 |
| ROBUST-02 | `queue.js` source: no sinks, no age discard, account bound, `Pinalert.submitReport`, timeout with fallback, `online` and timer flush | contract | quick, `./web/ -run TestQueue` | Wave 0 |
| ROBUST-02 | Flush order, never discard, wrong account never sent (fake IndexedDB) | node, optional | `node --test web/queue_logic_test.mjs` | if accepted |
| ROBUST-01 | `/low` 200 verified, 302 to `/login?low=1` logged out; `/` cookie redirect, `/?full=1`, loop cases | e2e | db, `./internal/api/ -run TestLowDataRoutes` | Wave 0 |
| ROBUST-01 | Asset policy, raw byte budget, banned view words, hidden guards (a new walker test over the Phase 4 selectors, because `TestVoteBlockHiddenGuard` covers only `.vote-*` heads), two track vote grid, no row tabindex, own fetch boot | contract | quick, `./web/ tests: TestLowData, TestFeedRenderKeepsRowsOnError` | Wave 0 |
| ROBUST-08 | Bar once per full page first in `<body>` inside `.top-chrome`; D-22 sentence exact; empty lists drop segments; legal pages no email or account header; contact absent when unset; `chrome.css` linked; dash free | contract and render | quick, `./web/ tests: TestDisclaimerBar, TestLegalPages, TestUserVisibleCopyUsesPlainPunctuation` | Wave 0 for `TestDisclaimerBar` and `TestLegalPages`; `TestUserVisibleCopyUsesPlainPunctuation` exists (`web/design_rules_contract_test.go`) and walks every embedded template, so the new templates are covered automatically (its `-run` regex also runs the `...AuthJS` sibling) |
| ROBUST-08 | `/disclaimer`, `/policy` reachable logged out, `Cache-Control: private`, exempt list test and router comment | e2e | db, `./internal/api/ -run TestAccessGateExemptRoutes` | amend |
| Moderator | 404 (same status, body, `Content-Type` and no `Allow` header as an unknown route) for logged out, unverified and unlisted visitors on the registered method AND on wrong methods (HEAD, PUT, GET on the POST routes, POST on `/moderate`), page and API; listed sees queues; Keep, Remove, audit; Remove by id | e2e | db, `./internal/api/handlers/ -run TestModerator` | Wave 0 |
| Flag | One per account (409), own 403, withheld 404, limiter, never changes `Resolve`, viewer marker, constant round trips | e2e | db, `./internal/api/handlers/ tests: TestFlag`; quick, `./internal/service/ tests: TestNearbyBatchesVoteReadsOnce` (no database needed) | `TestFlag` Wave 0; `TestNearbyBatchesVoteReadsOnce` exists in `internal/service/feed_test.go` and is amended for `ReportExtrasForReports` |
| Low data post and vote | `{ resolve: false }` two tracks, shared trust block, location interface of `report-form.js` | contract | quick, `./web/ tests: TestLowDataVote, TestReportFormLocationInterface` | Wave 0 |
| Cross cutting | Go copy test for every server message (four rejections, seven rate limit messages, 429 fields), plain characters; sink scan over all `web/static/js` with a `//` aware comment stripper (`stripJSComments`, see Area B landmines) | unit, contract | quick, `tests: TestServerCopy, TestNoMarkupSinksAnywhere` | Wave 0 |
| Migrations | New files embedded, up and down on scratch db, `Truncate` lists new tables | integration | db, `./internal/store/ -run 'TestMigrationsEmbedded|TestMigrationsUpDown'` | `TestMigrationsEmbedded` exists (`internal/store/migrations_test.go`, no database, checks only migration 00001) and is amended; `TestMigrationsUpDown` and a `Truncate` table list check are new (Wave 0) |

### Sampling Rate
- Per task commit: quick run plus the one database form command for the plan.
- Per wave merge: database form over `./internal/... ./web/` with `-p 1`.
- Phase gate: full database form green, then V-42 manual script.

### Wave 0 Gaps
- [ ] `internal/ratelimit/perkey.go` and test
- [ ] Migration, sqlc queries, `testutil.Truncate`, the feed and auth fakes extended with `ReportExtrasForReports`, `fakeVotingQuerier.ReportVoteContext` updated
- [ ] `screening.go`, fixture JSON (150 to 300 entries, five languages), recorder command, recorded scores (human step)
- [ ] `internal/moderation` client and `httptest` fake
- [ ] `Deps` knobs (generous budgets, injected clock, fake screener) in `newE2EServer` and `newGateTestServer`
- [ ] Optional `web/queue_logic_test.mjs`
- [ ] One source scan over every `static/js` file for the four markup sinks

## Security Domain

`security_enforcement` is on (ASVS level 1, block on high).

| ASVS | Applies | Control |
|------|---------|---------|
| V2 Authentication | yes (moderator identity) | Verified magic link account plus a server side email list; nothing new |
| V3 Session | yes | Existing signed cookie; `pinalert_low_data` is a non sensitive preference |
| V4 Access control | yes | `requireModerator` 404 gate; account scoped replay; flag and vote refused on withheld reports; limits keyed on the verified account |
| V5 Input validation | yes | `DisallowUnknownFields`, strict `request_id` pattern, rune counted length, enum flag reason, `written_at` clamp |
| V6 Cryptography | no new | `crypto/rand`, `crypto.getRandomValues` for ids |
| V7 Logging | yes | Block logs carry no report text or email; provider errors never echo the key |
| V8 Data protection | yes | Only trimmed text to the provider; `OPENAI_API_KEY` env only; `data-account-id` only in the owner's own HTML; honest retention statement |

| Threat | STRIDE | Mitigation |
|--------|--------|------------|
| Stored XSS via report text or moderator rows | Tampering | `Pinalert.setText`, no markup sinks, scan over all JS, no auto linking |
| Skip screening by choosing critical | Tampering | Abuse rejected for every severity (D-03); pending serious published but withdrawn on a late reject (R-03) |
| Provider quota exhaustion by one account | DoS | Attempt bucket, process wide provider budget, pending degradation |
| Double insert or cross account disclosure via `request_id` | Tampering, Info | Partial unique index and account scoped lookup |
| Moderator route discovery | Info | Own gate, 404, byte equal, routes registered for all methods so a wrong method cannot reveal them through chi's 405 |
| Flag brigading, fake serious floods | Tampering, DoS | A flag never hides; unique per account; limiter; Remove backstop; extra accounts cost an inbox each (accepted, as Phase 3 T-03-03) |
| Forged client IP header | Spoofing | Header setting only where the platform overwrites it; unset means no IP limit; spoof test stays green |
| CSRF on Keep, Remove, Flag | Tampering | SameSite=Lax cookie (DEC-L) |
| Cookie bearing legal pages cached by a shared cache | Info | `Cache-Control: private` |
| Open redirect on the cookie redirect | Tampering | Target is the constant `/low` |

## Sources

### Primary (HIGH)
- This repository, read 2026-10-05: router, gate, handlers, services, `internal/ratelimit`, `internal/session`, store queries and migrations, `testutil`, `cmd/server/main.go`, `mailer/loader.go`, `geocode/client.go`, `web/` templates, JS, CSS and `*_contract_test.go`, README, CI, Makefile, `sqlc.yaml`
- Phase 3 `03-PLAN-OUTLINE.md` and plan frontmatter 03-01 to 03-13; ROADMAP Phases 3.1, 3.2, 4; STATE WR-01
- Module source: chi v5.3.2 `middleware/{client_ip,wrap_writer,compress}.go`; `golang.org/x/time@v0.16.0/rate/rate.go`

### Secondary (MEDIUM)
- https://developers.openai.com/api/docs/guides/moderation and https://developers.openai.com/api/docs/models/omni-moderation-latest (fetched 2026-10-05)

### Tertiary (LOW)
- Provider error codes, Indic tokenisation, browser feature support, host sleep behaviour: tagged `[ASSUMED]`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new dependency, all versions read from `go.mod` and module source
- Architecture and integration points: HIGH, read from the code; MEDIUM for Phase 3, 3.1, 3.2 interactions because those phases are unbuilt and Phase 3 plans 03-14 to 03-20 were written after the collisions table was derived
- Pitfalls: HIGH for those tied to code lines, MEDIUM for provider behaviour
- Moderation accuracy on Indian languages: LOW until the fixture is recorded

**Research date:** 2026-10-05
**Valid until:** about 14 days, or until Phase 3 execution changes the files in the collisions table (re-grep then)
