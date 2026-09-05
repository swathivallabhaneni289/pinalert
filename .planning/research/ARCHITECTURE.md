# Architecture Research

**Domain:** Crowd-verified local emergency information feed (Go / PostgreSQL / Leaflet, anonymous posting, trust-scoring core)
**Researched:** 2026-09-05
**Confidence:** HIGH (component boundaries and data flow are derived directly from Pinalert's own stated requirements and constraints; MEDIUM on two externally-verified specifics — GDACS event-ID stability and Render/Railway free-tier sleep behavior — both corroborated via web search)

## Standard Architecture

### System Overview

```
┌───────────────────────────────────────────────────────────────────────┐
│                         CLIENTS (interchangeable)                     │
│  ┌────────────────┐   ┌────────────────┐   ┌────────────────┐        │
│  │ Server-rendered │   │  PWA (phase 2) │   │ Native app      │        │
│  │ HTML+JS (v1)    │   │  same JSON API │   │ (later, no      │        │
│  │ website         │   │  + SW/offline  │   │  backend rewrite)│       │
│  └────────┬────────┘   └────────┬────────┘   └────────┬────────┘      │
└───────────┼─────────────────────┼─────────────────────┼───────────────┘
            │                     │                     │
            └─────────────────────┴─────────────────────┘
                                  │  JSON over HTTP (the only contract)
┌─────────────────────────────────▼───────────────────────────────────┐
│                          API LAYER (Go, chi/mux)                     │
│  handlers/  — thin: decode request → call service → encode response │
│  html handlers and JSON handlers both sit here, side by side,       │
│  and BOTH depend only on service/, never on store/ directly         │
├───────────────────────────────────────────────────────────────────────┤
│                        SERVICE LAYER (domain logic)                  │
│  ┌──────────────┐ ┌──────────────────┐ ┌─────────────────────────┐ │
│  │ ReportService │ │ VisibilityResolver│ │ TrustScoringService     │ │
│  │ (submit,      │ │ (THE single       │ │ (confirm/dispute write, │ │
│  │  receipt code)│ │  function: state→ │ │  diversity weighting,   │ │
│  │               │ │  Hidden/Provisional│ │  confidence/reliability │ │
│  │               │ │  /Live/Retracted) │ │  decay)                 │ │
│  └──────────────┘ └──────────────────┘ └─────────────────────────┘ │
│  ┌──────────────┐ ┌──────────────────┐ ┌─────────────────────────┐ │
│  │ ModerationSvc │ │ FeedService       │ │ ShareCardService        │ │
│  │ (cascade,     │ │ (proximity query, │ │ (resolves live status,  │ │
│  │  async worker)│ │  triage/gap views)│ │  view/confirm log)      │ │
│  └──────────────┘ └──────────────────┘ └─────────────────────────┘ │
├───────────────────────────────────────────────────────────────────────┤
│                    BACKGROUND JOBS (triggered, not just ticker)      │
│  ┌────────────────────┐   ┌────────────────────────────────────┐   │
│  │ Official feed poller│   │ Expiry sweep (status-transition    │   │
│  │ (GDACS/IMD/CWC,     │   │ only — expiry itself is a read-time │   │
│  │ idempotent upsert)  │   │ predicate, sweep is just cleanup)   │   │
│  └────────────────────┘   └────────────────────────────────────┘   │
├───────────────────────────────────────────────────────────────────────┤
│                          STORE LAYER (Postgres)                      │
│  ┌──────────┐ ┌──────────────┐ ┌──────────┐ ┌────────────────────┐ │
│  │ reports  │ │ confirmations │ │ sessions │ │ report_view_log     │ │
│  │ (mutable │ │ (append-only, │ │ (anon,   │ │ (who saw/confirmed  │ │
│  │  status  │ │  vote+geohash │ │  reputa- │ │  a report while live,│ │
│  │  cache)  │ │  +session_id) │ │  tion)   │ │  needed for retract)│ │
│  └──────────┘ └──────────────┘ └──────────┘ └────────────────────┘ │
└───────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| API handlers | Decode HTTP/JSON, call one service method, encode response — no business logic, no direct SQL | `net/http` + `chi`, one handler file per resource |
| ReportService | Create report, assign receipt code, enqueue moderation, apply proximity filters | plain Go struct with a `store.Querier` dependency |
| VisibilityResolver | **The** function: `Resolve(report, votes, moderationState, now) → {Hidden, Provisional, Live, Retracted}` | pure function, no DB calls inside it — takes already-loaded data, returns an enum + reason |
| TrustScoringService | Write votes (append-only), compute diversity-weighted confirm count, compute confidence (fast decay) and reliability (slow decay) scores | pure functions over `[]Vote` + timestamps, called by VisibilityResolver and by read paths |
| ModerationService | Run confidence-cascade (toxicity/image APIs) async after submit, write `moderation_state` | worker consuming a queue table (`moderation_jobs`) or a goroutine pool with retry |
| FeedService | Proximity-filtered queries (map, list, triage, gap-layer) — all call VisibilityResolver, none re-implement the gate logic | SQL bbox prefilter + Go/SQL haversine, `WHERE status IN (...)` narrowed by resolver output |
| ShareCardService | Resolve a report's *current* live status for a card URL; reads `report_view_log` to know who to push a retraction correction to | thin read-only service, no independent visibility logic — calls VisibilityResolver |
| Official feed poller | Poll GDACS/IMD/CWC on a schedule, idempotent upsert on `(source, external_id)`, bypasses trust pipeline entirely | goroutine ticker for local dev; on Render/Railway free tier, triggered lazily on-request or via external cron hitting a protected endpoint (see Pitfall below) |
| Expiry | `expires_at < now()` filtered at every read (authoritative); a sweep job just transitions DB status for archival/admin views | SQL predicate in every query; sweep is a correctness *optimization*, not the source of truth |

## Recommended Project Structure

```
pinalert/
├── cmd/
│   └── server/
│       └── main.go            # wiring: config, DB pool, router, start poller
├── internal/
│   ├── api/
│   │   ├── handlers/           # JSON + HTML handlers, one file per resource
│   │   │   ├── reports.go
│   │   │   ├── confirmations.go
│   │   │   ├── feed.go
│   │   │   └── sharecards.go
│   │   └── router.go
│   ├── service/                # ALL business logic lives here
│   │   ├── report.go
│   │   ├── visibility.go        # VisibilityResolver — pure, heavily unit-tested
│   │   ├── trust.go             # diversity weighting, confidence/reliability decay
│   │   ├── moderation.go
│   │   └── sharecard.go
│   ├── jobs/
│   │   ├── poller.go            # official feed ingestion
│   │   └── expirysweep.go
│   ├── store/                   # SQL only, no business rules
│   │   ├── reports.go
│   │   ├── confirmations.go
│   │   ├── sessions.go
│   │   └── migrations/
│   └── session/                 # anonymous cookie/session-id issuance
├── web/
│   ├── templates/                # html/template files
│   └── static/                   # JS (map, confirm/dispute buttons), CSS
├── openapi.yaml
└── go.mod
```

### Structure Rationale

- **`internal/service/` is the API-first boundary**: both HTML handlers and JSON handlers call into it, never into `store/` directly. This is what makes "PWA next, native app later, no backend rewrite" actually true instead of aspirational — if a template handler ever queries the DB directly, that guarantee is already broken.
- **`visibility.go` is isolated and pure** on purpose: no DB, no HTTP, just `Resolve(input) → output`. That makes the Core Value table-testable (feed it a matrix of vote combinations, moderation states, and timestamps, assert the resulting visibility) without spinning up Postgres.
- **`jobs/` is separate from `service/`** because it has different triggering semantics (schedule/on-request vs HTTP-request-triggered) but calls the same service methods — a poller inserting an official report should go through `ReportService`/no-op through the trust pipeline, not a parallel code path.
- **`store/` has zero business logic** — it's query functions and struct scans only. Business rules (what counts as "confirmed," what expires) never leak into SQL WHERE clauses scattered across multiple files; they live once in `service/`.

## Architectural Patterns

### Pattern 1: Votes are the append-only source of truth; counters are derived

**What:** `confirmations` (votes) is an insert-only log: `report_id, session_id, vote, geohash_cell, created_at`. Nothing is ever updated or deleted from it. "Confirmed by N nearby," confidence score, and visibility state are all *computed* from this log (optionally cached on `reports` as a denormalized column with a recompute path), never hand-incremented in a handler.
**When to use:** From day one — this is not a refactor to defer, because retrofitting an append-only log onto a system that started with `UPDATE reports SET confirm_count = confirm_count + 1` means losing the ability to ever compute diversity weighting or reliability decay for historical data.
**Trade-offs:** Slightly more query work per feed render (aggregate over votes) vs. a naive counter column. Mitigate with a cached/denormalized `confirm_count_cached` column updated by the same service call that writes the vote, with a periodic reconciliation job as a safety net — cache as optimization, log as truth.

**Example:**
```go
// service/trust.go
func RecordVote(ctx context.Context, q Querier, reportID string, v Vote) error {
    if err := q.InsertVote(ctx, reportID, v); err != nil { // append-only insert
        return err
    }
    cached, err := computeConfirmCount(ctx, q, reportID) // recomputed from votes
    if err != nil { return err }
    return q.UpdateCachedCount(ctx, reportID, cached) // cache write, not truth
}
```

### Pattern 2: One visibility resolver, called by every read path

**What:** A single function takes a report's moderation state, vote log, category/severity, source, and current time, and returns exactly one of `{Hidden, Provisional, Live, Retracted}` plus the reason. The feed query, map query, triage query, and share-card resolver all call this same function — none of them re-derive visibility with their own WHERE-clause logic.
**When to use:** Always, starting from the first phase that has more than one read path (feed + map already qualifies).
**Trade-offs:** Requires loading a report's vote log to render it (vs. a single flat `status` column check), which is more DB work per item. Mitigate by resolving visibility at write-time on every vote/moderation-state change and storing the *result* as `reports.status`, with the resolver as the only writer of that column — reads stay a flat filter, only the write path pays the resolver cost.

**Example:**
```go
type Visibility int
const (
    Hidden Visibility = iota
    Provisional
    Live
    Retracted
)

func Resolve(r Report, votes []Vote, mod ModerationState, now time.Time) (Visibility, string) {
    if mod == ModRejected { return Hidden, "moderation_rejected" }
    if now.After(r.ExpiresAt) { return Retracted, "expired" } // expiry is a predicate, not just a sweep
    if disputeRatio(votes) > disputeThreshold { return Retracted, "disputed" }
    if r.Severity == Critical || r.Category == RescueNeeded {
        return Live, "critical_bypasses_gate" // fail-open for critical even if moderation still pending
    }
    if mod == ModPending { return Provisional, "moderation_pending" }
    if independentConfirmCount(votes) < 2 { return Provisional, "awaiting_second_independent_confirmation" }
    return Live, "confirmed"
}
```

### Pattern 3: Independence predicate before diversity weighting

**What:** Two layers, not one. Layer A (independence predicate): a vote only counts toward the provisional gate's "second confirmation" if it comes from a distinct `session_id` AND distinct `geohash_cell` from the reporter and from each other. Layer B (diversity weighting curve): once the gate is satisfied, the displayed "confirmed by N" number is a dampened function across cells (e.g. diminishing returns per additional vote from an already-represented cell), not a raw count.
**When to use:** Layer A must exist before the provisional gate is meaningful at all — a gate that accepts two votes from one device is worse than no gate, because it displays false assurance. Layer B is a refinement that can land after the base gate ships.
**Trade-offs:** Requires `geohash_cell` and `session_id` to be captured on every vote from the very first version, even before the weighting curve is built — this is not backfillable (see Pitfalls).

### Pattern 4: Confidence-cascade moderation is async from submission

**What:** `POST /api/reports` writes the row with `moderation_state = pending`, returns the receipt code immediately, and enqueues a moderation job. The confidence-cascade (auto-hide above a high-confidence threshold / flag for review in the middle band / no action below) runs out-of-band and only ever *tightens* visibility (via the resolver), never blocks the initial write.
**When to use:** Always — moderation depends on a third-party API (toxicity/image-safety), and coupling submission latency/availability to that vendor is exactly the failure mode to avoid during a surge, which is when the product matters most.
**Trade-offs:** A window exists between submission and moderation completing. Resolve it via the fail-open/fail-closed policy already encoded in the resolver: critical/rescue-needed publishes live while moderation is pending (fail-open, because suppressing a real rescue request is worse than a rare abusive one slipping through briefly); non-critical reports sit in Provisional until moderation clears (fail-closed), which conveniently reuses the same gate the confirm-count mechanic already needs.

## Data Flow

### Trust-Scoring Pipeline (the core flow)

```
[Anonymous submit, no signup]
      ↓  ReportService.Submit()
[reports row: status=pending, moderation_state=pending, receipt_code issued]
      ↓ (async enqueue)                              ↓ (sync, returned to client)
[ModerationService worker]                    [Receipt code shown to reporter]
      ↓ confidence-cascade result
[moderation_state: cleared | flagged | rejected]
      ↓
[VisibilityResolver.Resolve(report, votes=[], moderation_state, now)]
      ↓
   ┌──────────────┬───────────────────────┬─────────────┐
   │ critical/     │ non-critical,          │ rejected    │
   │ rescue-needed │ moderation cleared     │             │
   │ → Live now    │ → Provisional          │ → Hidden    │
   └──────┬────────┴───────────┬────────────┴─────────────┘
          │                    │
          │         [nearby users see dimmed pin, vote confirm/dispute]
          │                    ↓
          │         TrustScoringService.RecordVote()
          │         → append vote (session_id, geohash_cell, vote, ts)
          │         → independence predicate check
          │         → if 2nd independent confirm → Resolver re-runs → Live
          ↓                    ↓
   [Both paths converge: report is now Live, visible on feed/map/triage]
          ↓
   [Ongoing: every new vote → RecordVote → diversity-weighted count
    recomputed → confidence score decays fast without reconfirmation,
    reliability score on session decays slowly → Resolver re-run]
          ↓
   [expires_at < now() reached, OR dispute ratio crosses threshold]
          ↓
   VisibilityResolver → Retracted
          ↓
   [ShareCardService: any card URL for this report now resolves to
    "Retracted" instead of frozen stale count]
          ↓
   [Retraction push: report_view_log gives the list of sessions that
    viewed/confirmed while Live → push correction to those sessions]
```

### Official Feed Ingestion Flow (parallel, independent)

```
[Poller: triggered by ticker (local dev) OR lazily on next feed request
 OR external cron hitting a protected /internal/poll endpoint (production,
 see Pitfall on free-tier sleep)]
      ↓
[Fetch GDACS/IMD/CWC feed]
      ↓
[Upsert on (source='official', external_id=gdacs:eventid) — idempotent]
      ↓
[Insert directly as status=Live, source=official — BYPASSES VisibilityResolver
 and the confirm/dispute pipeline entirely; crowd votes cannot hide an
 official pin — this is a deliberate abuse-surface decision, not an oversight]
      ↓
[Rendered on map/feed alongside crowd reports, distinguished by source field]
```

### Read Path (feed / map / triage / gap-layer)

```
[Client requests feed, lat/lon + radius]
      ↓
FeedService.Query()
      ↓
[SQL: bbox prefilter on lat/lon columns] → [status IN (Live, Provisional
 depending on view) AND expires_at > now()] → [haversine distance filter
 in Go or SQL]
      ↓
[Results already carry resolver-computed status — no re-derivation here]
      ↓
[JSON response / html/template render]
```

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0–1k reports / low concurrent users (portfolio demo, initial real usage) | Single Postgres instance, single Go process, in-process goroutine poller acceptable for local/dev; plain lat/lon bbox + haversine is fast enough with an index on `(status, expires_at)` and a bbox-friendly index on `(latitude, longitude)` |
| 1k–100k reports / a real localized surge event | Add a composite index covering the feed's actual WHERE clause; consider caching the diversity-weighted count as a column updated on write (already recommended above) rather than aggregating votes per request; move the poller/sweep off the request-serving process's ticker onto an external cron hit against a protected endpoint (needed regardless of scale once deployed to Render/Railway free tier — see Pitfalls) |
| 100k+ reports (beyond this project's realistic scope) | Only then consider PostGIS for spatial indexing (explicitly deferred per project constraints — plain lat/lon is sufficient at target scale) or splitting moderation/poller into a separate deployed service from the request-serving API |

### Scaling Priorities

1. **First bottleneck: the proximity feed query.** A full-table haversine scan without a bbox prefilter degrades fast even at a few thousand active reports. Fix: index `(status, expires_at)` for the resolver-state filter, plus a simple lat/lon range prefilter before computing exact distance, before ever reaching for PostGIS.
2. **Second bottleneck: per-report vote aggregation on every feed render.** If "confirmed by N nearby" is computed by aggregating the full `confirmations` log on each read, this degrades as votes accumulate on popular reports. Fix: the cached-counter-with-recompute-path pattern from Pattern 1 — write-time aggregation, not read-time.

## Anti-Patterns

### Anti-Pattern 1: Visibility logic scattered across query WHERE clauses

**What people do:** Each read path (feed query, map query, triage query, share-card resolver) independently encodes its own version of "what counts as visible" — e.g. `WHERE status = 'active' AND NOT hidden` in one place, `WHERE moderation_state != 'rejected' AND confirm_count > 0` in another.
**Why it's wrong:** The four conditions drift independently over time. This project's own stated pain point — "shared cards resolve live instead of freezing a stale confirmation count" — is precisely a symptom of this anti-pattern: the share-card path doesn't share logic with the live feed path, so it goes stale.
**Do this instead:** One `VisibilityResolver.Resolve()` function, called (directly or via its cached output column) by every single read path without exception, including the share-card status endpoint.

### Anti-Pattern 2: Trusting an in-process ticker as the only clock on free-tier hosting

**What people do:** Spin up a `time.NewTicker` goroutine in `main.go` for the official-feed poller and the expiry sweep, and treat expiry/ingestion as reliably scheduled.
**Why it's wrong:** Verified via web search — Render's free tier spins a service down after ~15 minutes of inactivity and cold-starts (~1 min) on the next incoming request; there's no officially supported way to keep a free instance always-on (MEDIUM confidence, corroborated across Render community discussion and multiple independent write-ups). Railway's free/hobby tier has similar sleep/usage-cap behavior. A ticker goroutine simply doesn't fire while the process is asleep — a sweep outage during exactly that window would show stale/expired reports as still live, a correctness failure in the product's core claim.
**Do this instead:** Make expiry a read-time predicate (`expires_at < now()` filtered in every query) so correctness never depends on the sweep having run recently; trigger the poller and sweep lazily (e.g., "if more than N minutes since last poll, poll now" checked on an incoming feed request) or via an external cron hitting a protected internal endpoint — not solely an in-process ticker. Keep the ticker for local development convenience only.

### Anti-Pattern 3: Building the diversity-weighting curve before the independence predicate

**What people do:** Jump straight to a dampening formula ("10 confirms from one building read as ~1-2") without first establishing what makes a vote "independent" at all.
**Why it's wrong:** Without a hard independence predicate (distinct `session_id` + distinct `geohash_cell`), the provisional gate's "second independent confirmation" requirement is meaningless — two votes from the same device satisfy a naive count-based gate and the pin publishes on false assurance, which is worse than never gating.
**Do this instead:** Ship the independence predicate as part of the vote-write path from day one (it's cheap — a comparison on two already-captured columns), gate on it, and treat the fuller diminishing-returns weighting curve as a later refinement on the same underlying data.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| GDACS GeoJSON API (`gdacsapi/api/events/geteventlist`) | Poll periodically, upsert on `(source='official', external_id)` | Each event carries a stable `gdacs:eventid` (MEDIUM confidence, corroborated across official docs + two independent client libraries) — safe idempotency key. API caps at 100 records/call, paginated; a "most recent 100 events in last 4 days" endpoint fits a periodic-poll pattern well |
| IMD / CWC bulletins | Same poller pattern if/when a stable feed format is confirmed | Not yet verified in this research pass — treat as a follow-up spike before building that specific ingestor; GDACS is the safer first integration |
| Toxicity/text-moderation API (hosted, vendor TBD) | Async job, called from ModerationService worker, never inline on submit | Calibrate threshold to an explicit false-positive budget (per project constraint), not a flat score cutoff |
| Image-safety API (hosted, vendor TBD) | Same async pattern, deferred until photo attachment ships (currently out of scope for core build) | — |
| Railway / Render (hosting) | Deploy as a single Go binary; do not rely on in-process scheduling for correctness-critical jobs | Free tier sleeps after inactivity (verified, MEDIUM confidence) — use external cron or lazy on-request triggering for poller/sweep in production |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| API handlers ↔ service layer | Direct Go function calls, in-process | Handlers never construct SQL or call `store/` directly — this is the enforceable rule behind "API-first, no backend rewrite for native app later" |
| Service layer ↔ store layer | Direct Go function calls via a `Querier` interface | Keeps `service/` unit-testable against a fake/in-memory store, independent of Postgres |
| Service layer ↔ background jobs | Jobs call the same service methods (e.g. poller calls `ReportService.InsertOfficial`) rather than duplicating insert logic | Prevents the poller from silently diverging from the crowd-report code path on validation/schema rules |
| VisibilityResolver ↔ everything else | Pure function, called synchronously from TrustScoringService (on every vote), ModerationService (on state change), and read paths (via cached output) | The one component every other component depends on; it depends on nothing else in the system |
| Website / PWA / native app ↔ backend | JSON over HTTP only, versioned via OpenAPI spec | The only contract that must remain stable across clients; html/template rendering is a presentation concern layered on top of the same JSON-producing service calls, not a separate logic path |

## Sources

- Project's own prior research baked into `PROJECT-NOTES.md` (Waze confidence/reliability split, X/Twitter Community Notes diversity/bridging-based ranking, Wikipedia Pending Changes and ClueBot NG confidence-cascade moderation, Ushahidi provisional-gating precedent, Boston Marathon misinformation-correction-reach study motivating retraction push) — treated as HIGH confidence, already vetted by the project owner before this research pass.
- [GDACS API quick start (v1)](https://www.gdacs.org/Documents/2025/GDACS_API_quickstart_v1.pdf) and [v2](https://www.gdacs.org/Documents/2025/GDACS_API_quickstart_v2.pdf) — official GDACS API documentation confirming `gdacs:eventid` and the GeoJSON `geteventlist` endpoint. MEDIUM confidence (web search synthesis, cross-corroborated).
- [gdacs-api on PyPI](https://pypi.org/project/gdacs-api/) and [python-aio-georss-gdacs](https://github.com/exxamalte/python-aio-georss-gdacs) — independent third-party client libraries confirming the same stable event-ID field, used as corroboration.
- [Render: service goes to sleep after inactivity (community discussion)](https://github.com/orgs/community/discussions/197645) and multiple independent write-ups on Render free-tier spin-down (~15 min inactivity, ~1 min cold start, no officially supported always-on free option) — MEDIUM confidence, corroborated across sources, directly informs the expiry-as-predicate and lazy-trigger recommendations in this document.

---
*Architecture research for: crowd-verified local emergency information feed (Pinalert)*
*Researched: 2026-09-05*
