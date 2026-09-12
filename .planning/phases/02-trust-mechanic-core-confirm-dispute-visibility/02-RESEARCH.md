# Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility - Research

**Researched:** 2026-09-12
**Domain:** Concurrency-safe crowd-voting trust mechanic on an existing Go/pgx/sqlc/chi codebase (extension, not greenfield)
**Confidence:** HIGH

## Summary

This phase extends an already-shipped Phase 1/1.1 codebase, not a blank slate. The correct
approach is almost entirely dictated by two things already locked before this research started:
the `VisibilityResolver` pure-function architecture in `.planning/research/ARCHITECTURE.md`, and
18 specific implementation decisions in `02-CONTEXT.md`. No new third-party package is required —
everything needed (pgx/v5, sqlc, chi, mmcloughlin/geohash) is already a dependency, already pinned,
already used correctly elsewhere in this repo.

The single most consequential design call this research makes is **not** to reuse Phase 1.1's
`INSERT ... ON CONFLICT ... DO UPDATE` atomic-upsert pattern for votes, even though the phase
description explicitly asked this research to check whether it fits. It doesn't, and the reason
matters: `email_cooldowns` had a genuine single-mutable-row race (one row, contended reads-then-
writes) that needed an atomic conditional UPDATE to serialize. Votes have a structurally different
shape — an append-only history table with no shared mutable counter at all. Combined with D-08's
explicit mandate that visibility is "recomputed live from current vote tallies on every read,"
there is no read-modify-write race to guard against in the vote-write path: concurrent `INSERT`s
into an unconstrained log table are something Postgres already handles correctly with zero
application-level locking. TRUST-09's concurrency test proves this holds (no vote silently
dropped), not that a lost-update race was averted by a special SQL shape.

**Primary recommendation:** One `votes` table (report_id, account_id, kind, value, geohash_cell,
created_at), pure INSERT-only, no unique constraint. "Current effective vote" per account is a
`SELECT DISTINCT ON (report_id, account_id, kind) ... ORDER BY created_at DESC, id DESC` read.
Independence counting is `COUNT(DISTINCT geohash_cell)` over that already-per-account-deduped set —
one small Go helper function, reused at all four independence-gated call sites (Hidden trigger,
Provisional→Live gate, confirmer-resolve, reopen). `VisibilityResolver` stays a pure function with
no DB calls, computed fresh on every read per D-08, with no cached `reports.status` column in this
phase (that optimization is explicitly deferred, not needed at this scale, and would reintroduce
exactly the lost-update risk this design avoids).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| GPS location capture for the independence predicate | Browser/Client | — | `navigator.geolocation` is a client-only capability; the server never trusts a client-computed geohash, only raw lat/lon (extends the existing "server decides, client never claims" pattern) |
| Confirm/dispute/resolve/reopen vote casting | API/Backend | Browser/Client | Server is sole authority for vote validity, the independence predicate, and resulting visibility; the browser only collects GPS coords, POSTs, and waits for server confirmation (D-04) before updating anything |
| Independence predicate (distinct account + distinct geohash cell) | API/Backend | — | Pure computation over server-held vote rows only; a client can never influence this by claiming diversity |
| `VisibilityResolver` (Hidden/Provisional/Live/Retracted) | API/Backend | — | Single pure function per `ARCHITECTURE.md`, called by every read path (feed, map, triage, share-card — the last two are not built until later phases, but the resolver's contract must already support them) |
| Feed/map rendering of visibility state (dimmed/labeled/hidden) | Browser/Client | API/Backend | The API computes and serializes visibility + tally counts as JSON; the browser only applies CSS classes and copy (D-09/D-10/D-11) — no client-side visibility logic exists anywhere |
| "Show disputed" filter toggle | Browser/Client | API/Backend | The toggle is a UI control that adds a query param; the actual Hidden-inclusion decision is made server-side by the same resolver-backed query, never by hiding/showing already-fetched rows client-side |
| Vote/visibility persistence | Database/Storage | — | The append-only `votes` table is the sole source of truth; there is deliberately no cached counter/status column this phase (see Summary) |

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TRUST-01 | User can confirm or dispute another user's report | `votes` schema, `POST /api/reports/{id}/confirm`\|`dispute` endpoint design, reporter-exclusion check (D-03) |
| TRUST-02 | One shared resolver computes visibility identically everywhere it's shown | `internal/service/visibility.go` pure-function design, `Resolve()` signature, no read-path re-derivation |
| TRUST-03 | A vote counts toward independence only if distinct account AND distinct geohash cell | `independentCellCount()` helper, geohash precision-7 voter cell, GPS capture contract |
| TRUST-04 | Non-critical reports gated Provisional until 2nd independent confirm; critical publishes instantly | `Resolve()` ordering (critical bypass before the gate check), `IndependentAgreementThreshold = 2` |
| TRUST-06 | Severity affects triage order only, never bypasses the gate | `Resolve()` explicitly keys the bypass on `Severity == Critical \|\| Category == RescueNeeded`, never on triage sort weight — same field, two independent consumers |
| TRUST-08 | Reporter or nearby confirmer can mark a report resolved, removing it from the feed | `resolve`/`reopen` vote kind design, reporter-instant-resolve vs. independent-agreement-gated confirmer-resolve (D-13/D-14) |
| TRUST-09 | Concurrent votes never silently lose an update, verified by an automated concurrency test | Append-only INSERT-only schema (no read-modify-write surface), concurrency test design mirroring `TestClaimEmailCooldownConcurrent` |
</phase_requirements>

## Standard Stack

### Core

No new core dependency. This phase is built entirely on what `go.mod` already pins:

| Library | Version (pinned in repo) | Purpose | Why Standard (already validated in Phase 1/1.1) |
|---------|---------|---------|--------------|
| `github.com/jackc/pgx/v5` | v5.10.0 | Postgres driver + pool | Already the store-layer driver; votes use the same `pgxpool.Pool` |
| `sqlc` (v1.31.1, dev tool) | v1.31.1 | SQL → typed Go | Same `sql_package: "pgx/v5"` config in `sqlc.yaml`; add `votes.sql` alongside `reports.sql` |
| `github.com/go-chi/chi/v5` | v5.3.2 | Routing | New routes mount inside the existing gated `r.Group` in `internal/api/router.go` |
| `github.com/mmcloughlin/geohash` | v0.10.0 | Geohash encode | Already used at `internal/service/report.go`'s `geohashPrecision = 8`; this phase adds a **second, coarser** precision constant for the voter's own independence-predicate cell (see Don't Hand-Roll) |
| `github.com/pressly/goose/v3` | v3.28.0 | Migrations | New migration `00004_create_votes.sql`, following the exact `-- +goose Up`/`-- +goose Down` convention of `00001`–`00003` |

### Supporting

Nothing new. `golang.org/x/time/rate` (already a dependency for `internal/ratelimit`) is **not**
needed here — vote endpoints are gated by the existing verified-account middleware, and rate-
limiting a verified account's voting frequency is out of this phase's scope (not named in any of
TRUST-01..09, and CONTEXT.md's Deferred Ideas section is empty, meaning it wasn't raised).

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Append-only `votes` log + live-recompute resolver | Cached `reports.confirm_count`/`status` column updated via `SELECT ... FOR UPDATE` per vote | `ARCHITECTURE.md` Pattern 2 offers this as a *later* optimization once per-request aggregation becomes a measured bottleneck (thousands of votes on one report). At this phase's scale (a portfolio demo, tens of votes per report) it is premature, and D-08 explicitly chose the always-recompute path ("no special-case unhide logic needed") — building the cache now would mean building the exact lost-update race Pitfall 2 warns about, for no present benefit. Revisit only if `EXPLAIN ANALYZE` on the feed query ever shows the vote-aggregation join as the bottleneck. |
| One `votes` table with a `kind` discriminator column (content vs. resolution) | Two separate tables (`confirmations` and `resolutions`) | A single table keeps the "current effective vote per (report, account, kind)" query, the independence-counting helper, and the concurrency test identical in shape for both vote families — directly satisfying CONTEXT.md's "one independence rule everywhere" ask. Two tables would duplicate that logic twice for no isolation benefit (both are the same append-only-log pattern). |
| `INSERT`-only, no unique constraint, latest-row-wins | `INSERT ... ON CONFLICT (report_id, account_id, kind) DO UPDATE` (Phase 1.1's proven atomic-upsert shape) | This is the alternative the phase description explicitly asked to evaluate. Rejected: Phase 1.1's pattern exists to serialize a *single mutable row* under contention (one email, one cooldown timestamp). A vote has no equivalent single-row contention point — "current vote" is a derived read, not a stored mutable field — so there is nothing to make atomic via `ON CONFLICT`. Using `ON CONFLICT DO UPDATE` here would also destroy the vote history Phase 3 needs for confidence/reliability decay scoring (an `UPDATE` overwrites the row in place; an `INSERT`-only log preserves every vote-change event). |

**Installation:** none — no `go get` needed. Regenerate sqlc output after adding `votes.sql`:
```bash
sqlc generate   # or: make sqlc
```

**Version verification:** all four libraries above are already installed at the versions shown in
`go.mod` (verified by direct file read, not npm/registry lookup — this is an existing Go module,
not a new install). `[VERIFIED: go.mod]`.

## Package Legitimacy Audit

**No new external packages are introduced by this phase.** Every dependency needed already exists
in `go.mod` (`pgx/v5`, `sqlc` as a dev tool, `chi/v5`, `mmcloughlin/geohash`, `goose/v3`), each
already verified during Phase 1/1.1's own research and audited by this project's existing CI
(`go build`, `go vet`, `go test`). The Package Legitimacy Gate protocol (registry check,
postinstall-script scan) is not applicable — there is nothing to check. If a future planning pass
for this phase decides GPS-accuracy sanity-checking or additional client tooling is needed, run the
gate at that time.

**Packages removed due to [SLOP] verdict:** none (none proposed).
**Packages flagged as suspicious [SUS]:** none (none proposed).

## Architecture Patterns

### System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│ BROWSER (vanilla JS, no build step)                                 │
│                                                                       │
│  feed row / map pin popup                                            │
│   [Confirm] [Dispute] [Mark Resolved] / [Reopen]  (D-01, D-15)       │
│         │                                                             │
│         ▼                                                             │
│  getVoterLocation()  ── sessionStorage cache (once per session, D-17) │
│         │                     │                                       │
│         │        (granted)    │        (denied → D-18: BLOCK)         │
│         ▼                     ▼                                       │
│  {lat, lon} ready        inline error, vote button stays disabled,    │
│         │                 no request ever sent                        │
│         ▼                                                             │
│  POST /api/reports/{id}/{confirm|dispute|resolve|reopen}              │
│  body: {latitude, longitude}   (raw coords only — no client geohash) │
│  UI shows "submitting…" and WAITS for the response (D-04, no          │
│  optimistic update)                                                    │
└──────────────────────────┬────────────────────────────────────────────┘
                           │ JSON over HTTP
┌──────────────────────────▼────────────────────────────────────────────┐
│ API LAYER (chi, gated r.Group — requireVerifiedAccount already exists) │
│  handlers/votes.go: decode body, resolve caller account from context, │
│  call VotingService, encode response                                  │
└──────────────────────────┬────────────────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────────────────┐
│ SERVICE LAYER (internal/service)                                      │
│                                                                         │
│  VotingService.CastVote(ctx, in)                                       │
│   1. look up report's reporter account (join through sessions)         │
│   2. D-03: content vote (confirm/dispute) from the reporter → reject   │
│   3. compute geohash_cell = geohash.EncodeWithPrecision(lat,lon,7)      │
│   4. q.InsertVote(...)  — plain INSERT, no ON CONFLICT, no lock         │
│   5. q.CurrentVotesForReports([]int64{reportID}) — DISTINCT ON read    │
│   6. BuildVoteTally(rows, reporterAccountID) → VoteTally                │
│   7. visibility.Resolve(reportMeta, tally, now) → (Visibility, reason) │
│   8. return {visibility, tally, reason} to the handler                 │
│                                                                         │
│  visibility.go: Resolve(meta, tally, now) — PURE, no DB calls,          │
│  called identically by CastVote above AND by every read path below    │
└──────────────────────────┬────────────────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────────────────┐
│ READ PATH (GET /api/reports — existing endpoint, extended)             │
│  1. NearbyReports bbox+Haversine (existing, unchanged)                 │
│  2. CurrentVotesForReports(ANY(ids)) — ONE batched query, no N+1        │
│  3. BuildVoteTally per report → visibility.Resolve() per report        │
│  4. filter: default {Provisional, Live}; ?show_disputed=true adds       │
│     {Hidden} (D-10/D-11); Retracted is never returned by this feed      │
│     query (D-12 — visible only via the existing profile/Activity path) │
└─────────────────────────────────────────────────────────────────────────┘
                           │
┌──────────────────────────▼────────────────────────────────────────────┐
│ STORE LAYER (Postgres)                                                 │
│  votes (append-only, INSERT-only, no unique constraint)                 │
│  reports (unchanged this phase — no new columns needed)                 │
└─────────────────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure

```
internal/
├── service/
│   ├── report.go        # unchanged
│   ├── visibility.go     # NEW — pure Resolve() function, heavily unit-tested
│   └── trust.go          # NEW — VotingService, CastVote, BuildVoteTally, independentCellCount
├── api/
│   ├── handlers/
│   │   └── votes.go      # NEW — CastVoteHandler(kind, value) factory, one handler reused ×4
│   └── router.go         # EXTENDED — mount /api/reports/{id}/{confirm,dispute,resolve,reopen}
├── store/
│   ├── migrations/
│   │   └── 00004_create_votes.sql   # NEW
│   └── queries/
│       └── votes.sql     # NEW — InsertVote, CurrentVotesForReports
web/
└── static/js/
    └── votes.js           # NEW — getVoterLocation(), castVote(id, action), button wiring
```

### Pattern 1: Append-only votes, no cached counter (extends `ARCHITECTURE.md` Pattern 1)

**What:** `votes(id, report_id, account_id, kind, value, geohash_cell, created_at)`. Every cast —
including a changed vote (D-02) — is a new `INSERT`, never an `UPDATE`. "Current effective vote"
is always a read: `DISTINCT ON (report_id, account_id, kind) ... ORDER BY created_at DESC, id DESC`.

**When to use:** From this phase's first migration — not backfillable later without losing the
full vote-change history Phase 3's confidence/reliability decay scoring needs (per
`ARCHITECTURE.md`'s own warning and `PITFALLS.md` §405-406's geohash-capture-from-day-one guidance).

**Trade-offs:** A double-click or client retry produces two rows for the same (account, kind)
instead of one — harmless (the `DISTINCT ON` read still collapses to exactly one current vote per
account), but grows the table faster than a single-row-per-voter design. Acceptable per
`PITFALLS.md`'s own "Unbounded table growth" entry — a portfolio project's scale, not a concern
requiring an archive job in this phase.

**Example (schema):**
```sql
-- internal/store/migrations/00004_create_votes.sql
-- +goose Up
CREATE TABLE votes (
    id            BIGSERIAL PRIMARY KEY,
    report_id     BIGINT NOT NULL REFERENCES reports(id),
    account_id    BIGINT NOT NULL REFERENCES accounts(id),
    kind          TEXT NOT NULL,   -- 'content' | 'resolution'
    value         TEXT NOT NULL,   -- content: 'confirm'|'dispute'; resolution: 'resolve'|'reopen'
    geohash_cell  TEXT NOT NULL,   -- voter's OWN location, precision 7 — NOT reports.geohash
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Serves the DISTINCT ON current-vote read, ordered exactly as that query needs.
CREATE INDEX idx_votes_report_account_kind_created
    ON votes (report_id, account_id, kind, created_at DESC, id DESC);

-- +goose Down
DROP TABLE votes;
```

### Pattern 2: One `Resolve()` pure function, called by every read path (reuses `ARCHITECTURE.md` Pattern 2, adapted to CONTEXT.md's locked decisions)

**What:** `Resolve(meta ReportMeta, tally VoteTally, now time.Time) (Visibility, string)`. No DB
calls inside it. Expiry is **not** one of its four states — expiry stays the existing Phase 1
read-time predicate (`expires_at > now()`), applied as a separate filter alongside this resolver's
output, exactly as it is today. This resolver's four states are purely about the trust mechanic
(D-07: "Retracted is a distinct trigger... not folded into ordinary expiry").

**When to use:** Every place a report's state is decided — the extended `GET /api/reports` feed
query and the vote-cast response itself (so the caller sees the fresh state immediately, per D-04).
Future phases (map popup detail, triage view, share cards) call the exact same function — this is
the whole point of the architecture.

**Example:**
```go
// internal/service/visibility.go
package service

import "time"

type Visibility string

const (
	VisibilityHidden      Visibility = "hidden"
	VisibilityProvisional Visibility = "provisional"
	VisibilityLive        Visibility = "live"
	VisibilityRetracted   Visibility = "retracted"
)

// IndependentAgreementThreshold is the one shared number CONTEXT.md's D-14
// requires reusing across the Hidden trigger's floor, the Provisional→Live
// gate, confirmer-driven resolving, and reopening. See "Concrete Threshold
// Recommendation" below for the reasoning.
const IndependentAgreementThreshold = 2

// ReportMeta is the subset of a report's fields Resolve needs — deliberately
// not the full store row, so this file has zero DB-shaped dependencies.
type ReportMeta struct {
	Severity Severity
	Category Category
}

func (m ReportMeta) criticalBypass() bool {
	return m.Severity == SeverityCritical || m.Category == CategoryRescueNeeded
}

// VoteTally is every already-aggregated count Resolve needs. Building this
// from raw vote rows is trust.go's job (BuildVoteTally), not this file's —
// Resolve itself never sees a []Vote or touches the database.
type VoteTally struct {
	ConfirmCells     int  // independent (distinct geohash cell) confirm count
	DisputeCells     int  // independent dispute count
	ResolveCells     int  // independent resolve count, EXCLUDING the reporter's own vote
	ReopenCells      int  // independent reopen count, EXCLUDING the reporter's own vote
	ReporterResolved bool // reporter's own current resolution vote is "resolve" (D-13 instant path)
}

func (t VoteTally) isRetracted() bool {
	resolved := t.ReporterResolved || t.ResolveCells >= IndependentAgreementThreshold
	reopened := t.ReopenCells >= IndependentAgreementThreshold
	return resolved && !reopened
}

// Resolve is THE single authority for a report's visibility (TRUST-02). No
// DB calls, no HTTP, no side effects — pure function over already-loaded
// data, exactly as ARCHITECTURE.md's Pattern 2 specifies, adapted to this
// phase's locked decisions: Hidden is dispute-triggered and fully reversible
// (D-05/D-08), Retracted is resolve-triggered only (D-07) and reopenable
// (D-16), and critical/rescue-needed bypasses BOTH the Hidden-by-dispute
// trigger (D-06) and the Provisional gate (TRUST-04) — but never bypasses
// Retracted, since marking a report resolved must work regardless of
// severity.
func Resolve(meta ReportMeta, tally VoteTally, now time.Time) (Visibility, string) {
	if tally.isRetracted() {
		return VisibilityRetracted, "resolved"
	}
	if meta.criticalBypass() {
		return VisibilityLive, "critical_bypasses_gates"
	}
	if tally.DisputeCells >= IndependentAgreementThreshold && tally.DisputeCells > tally.ConfirmCells {
		return VisibilityHidden, "disputed"
	}
	if tally.ConfirmCells < IndependentAgreementThreshold {
		return VisibilityProvisional, "awaiting_second_independent_confirmation"
	}
	return VisibilityLive, "confirmed"
}
```

### Pattern 3: One independence-counting helper, four call sites (directly satisfies CONTEXT.md's "one independence rule everywhere")

**What:** Because "current effective vote" already collapses to exactly one row per account (via
the `DISTINCT ON` read), the "distinct account AND distinct geohash cell" predicate (TRUST-03)
simplifies to just `COUNT(DISTINCT geohash_cell)` over that already-per-account-deduped set — the
distinct-account half is automatically satisfied by construction, so only the geohash-cell
dimension needs an explicit count.

**Example:**
```go
// internal/service/trust.go
package service

import (
	"context"

	"github.com/mmcloughlin/geohash"

	sqlcgen "pinalert/internal/store/sqlc"
)

// voterGeohashPrecision is deliberately DIFFERENT from report.go's
// geohashPrecision = 8 (which encodes a REPORT's own location for the
// bbox/Haversine feed query). This constant encodes a VOTER's own location
// for the independence predicate specifically — precision 7 is ~153m x
// 153m at the equator, chosen to roughly match PITFALLS.md §383-421's
// "realistic incident radius" guidance (~100-300m for a flooded road
// segment) so that several genuinely distinct nearby witnesses (different
// households on the same short street) are NOT accidentally collapsed into
// one cell — see "Concrete Threshold Recommendation" below. Phase 3 revisits
// this with a benchmarked value; this is a deliberate, documented interim
// choice, not a library default (the mistake PITFALLS.md Pitfall 10 warns
// against).
const voterGeohashPrecision = 7

// independentCellCount returns the number of distinct geohash cells among
// rows — the ONE implementation TRUST-03's independence predicate reduces
// to, reused by the Hidden trigger, the Provisional→Live gate, confirmer-
// driven resolving, and reopening (CONTEXT.md's explicit "one independence
// rule everywhere" ask). Callers must pass an already-per-account-deduped
// slice (one row per account) — this function does not deduplicate by
// account itself.
func independentCellCount(cells []string) int {
	seen := make(map[string]struct{}, len(cells))
	for _, c := range cells {
		seen[c] = struct{}{}
	}
	return len(seen)
}

// VotingService casts votes and derives the tally Resolve needs.
type VotingService struct {
	q VotingQuerier
}

type VotingQuerier interface {
	InsertVote(ctx context.Context, arg sqlcgen.InsertVoteParams) error
	CurrentVotesForReports(ctx context.Context, reportIDs []int64) ([]sqlcgen.CurrentVotesForReportsRow, error)
	ReporterAccountID(ctx context.Context, reportID int64) (int64, error)
}

func NewVotingService(q VotingQuerier) *VotingService {
	return &VotingService{q: q}
}

// CastVote validates the caller, computes the voter's own geohash cell
// server-side from raw lat/lon (never trusting a client-supplied geohash —
// extends the existing "server decides" pattern from report.go's Submit),
// inserts the vote, and returns the freshly recomputed visibility so the
// caller can respond before any client-side optimistic update (D-04).
func (s *VotingService) CastVote(ctx context.Context, in CastVoteInput) (Visibility, string, error) {
	reporterID, err := s.q.ReporterAccountID(ctx, in.ReportID)
	if err != nil {
		return "", "", err
	}
	if in.Kind == VoteKindContent && in.AccountID == reporterID {
		return "", "", ErrCannotVoteOwnReport // D-03
	}

	cell := geohash.EncodeWithPrecision(in.Latitude, in.Longitude, voterGeohashPrecision)
	if err := s.q.InsertVote(ctx, sqlcgen.InsertVoteParams{
		ReportID:    in.ReportID,
		AccountID:   in.AccountID,
		Kind:        string(in.Kind),
		Value:       string(in.Value),
		GeohashCell: cell,
	}); err != nil {
		return "", "", err
	}

	rows, err := s.q.CurrentVotesForReports(ctx, []int64{in.ReportID})
	if err != nil {
		return "", "", err
	}
	tally := BuildVoteTally(rows, reporterID)
	// meta (severity/category) is loaded by the caller from the reports
	// table and passed in alongside — omitted here for brevity.
	vis, reason := Resolve(meta, tally, time.Now())
	return vis, reason, nil
}
```

### Anti-Patterns to Avoid

- **Re-deriving visibility in the feed query's SQL `WHERE` clause** (`ARCHITECTURE.md` Anti-Pattern
  1) — the extended `GET /api/reports` query must fetch candidate reports + their current votes,
  then call `Resolve()` in Go for each, never encode "what counts as visible" as a second WHERE
  clause.
- **Building a diversity-weighting dampening curve this phase** (`ARCHITECTURE.md` Anti-Pattern 3)
  — Phase 2 needs only the boolean/count independence gate; the "confirmed by N nearby" *display*
  number (diminishing returns per cell) is explicitly Phase 3 (TRUST-05), per CONTEXT.md's Phase
  Boundary section.
- **Caching a `reports.status` column this phase** — tempting because it mirrors
  `ARCHITECTURE.md`'s general advice, but D-08 explicitly chose live recomputation, and adding a
  cache would reopen the exact lost-update race this design's `INSERT`-only shape was chosen to
  avoid. Revisit only if profiling later shows the vote-aggregation join is the bottleneck.
- **Reusing the fallback-permissive geolocation pattern from `modal.js`/`map.js`** — those two
  existing call sites deliberately fall back silently to a default center when geolocation is
  denied (correct for their purpose: report submission must never be blocked by a GPS prompt).
  D-18 requires the *opposite* behavior for voting: a denied GPS prompt must **block** the vote,
  not silently proceed with a fallback location. Copy-pasting `initLocation()`'s fallback branch
  into the vote-casting flow would silently violate D-18.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Vote de-duplication per account | A manual "check if this account already voted, then update" flow in application code | The `DISTINCT ON (report_id, account_id, kind) ORDER BY created_at DESC, id DESC` read | Postgres's own `DISTINCT ON` is exactly the tool for "latest row per key" and needs no explicit locking; a hand-rolled check-then-branch reintroduces a TOCTOU race between the check and the insert |
| Diversity/independence counting | A join-heavy aggregate query mixing distinct-account and distinct-cell logic differently at each of the four call sites | The single `independentCellCount()` Go helper, called identically everywhere | This is the literal ask in CONTEXT.md's "one independence rule everywhere" — three or four bespoke implementations is the exact anti-pattern the discussion converged against |
| GPS permission state tracking | A hand-rolled polling loop against `navigator.permissions` | `navigator.geolocation.getCurrentPosition()`'s own native prompt/grant/deny flow, with a `sessionStorage` cache of the last successful capture | The Permissions API's `'geolocation'` name has inconsistent cross-browser support (notably Safari); `getCurrentPosition` alone already handles prompt-once-then-grant/deny natively in every evergreen browser without needing the Permissions API at all |
| Concurrency safety for the vote tally | Any `SELECT ... FOR UPDATE` lock or `UPDATE ... SET count = count + 1` on a shared row | Plain unconstrained `INSERT` into the append-only log | There is no shared row to lock — this is the core insight of Pattern 1 above; adding a lock here would be solving a problem this schema doesn't have |

**Key insight:** Every "don't hand-roll" item above traces back to the same root cause as
`ARCHITECTURE.md`'s own Anti-Pattern 1 — the temptation to give each caller (a vote-cast handler,
a feed query, a future triage view) its own slightly-different implementation of "what does this
vote data mean right now." The fix in every case is the same: push the derivation into one shared,
pure function or one shared, correctly-indexed query, and make every caller go through it.

## Common Pitfalls

### Pitfall 1: Treating TRUST-09 as "needs an atomic upsert like Phase 1.1's email cooldown"

**What goes wrong:** Copying the `INSERT ... ON CONFLICT (email) DO UPDATE ... WHERE guard
RETURNING` shape directly onto votes, because the phase description (correctly) points at it as
the codebase's own most recent concurrency-safety precedent.
**Why it happens:** Pattern-matching on "this codebase already solved a concurrency problem near
here" without checking whether the underlying shape (single mutable row vs. append-only log) is
actually the same problem.
**How to avoid:** Recognize that `email_cooldowns` has exactly one row per email that many
requests race to read-then-write; `votes` has no equivalent row — the write is a pure append, and
the only shared state that could race is the current-vote *read*, which is a query, not a stored
value. There is nothing to serialize with `ON CONFLICT`.
**Warning signs:** If the schema design ends up with a `PRIMARY KEY (report_id, account_id, kind)`
and an `ON CONFLICT ... DO UPDATE`, the vote-change history is being silently discarded — check
this against Phase 3's stated need for full vote-change history before locking in a plan.

### Pitfall 2: Race condition in the confirm/dispute tally (PITFALLS.md Pitfall 2, directly)

**What goes wrong:** Exactly as documented in `PITFALLS.md` — a naive `UPDATE reports SET
confirm_count = confirm_count + 1` or a separate read-then-write cache update loses concurrent
updates under a real voting surge.
**How this phase avoids it structurally, not just by test:** By not having a `confirm_count` column
at all in Phase 2 (see Pattern 1/Anti-Patterns above), the entire class of bug is structurally
absent, not merely tested-for. The concurrency test (see below) still exists as the phase's
required proof, but it is proving "no insert was silently lost," not "a lock correctly prevented
a lost update" — a stronger and simpler guarantee.
**Verification:** `TestCastVoteConcurrent` (see Concurrency Test Design below) — N goroutines,
N distinct accounts, one report; assert `COUNT(*) FROM votes WHERE report_id = $1` equals N exactly.

### Pitfall 3: Geohash precision picked without checking it against a realistic incident radius (PITFALLS.md §383-421, Pitfall 10)

**What goes wrong:** Reusing `report.go`'s existing `geohashPrecision = 8` (a report's own fine-
grained location, ~38m × 19m cells) for the *voter's* independence-predicate cell, without
realizing this is a different measurement serving a different purpose.
**Why it happens:** "There's already a geohash precision constant in this codebase" is an easy,
wrong shortcut — that constant answers "how precisely do we store where a report happened," not
"how far apart must two voters be to count as independent."
**How to avoid:** A separate, deliberately chosen constant (`voterGeohashPrecision = 7`, ~153m ×
153m — see Pattern 3's code comment) sized against Pitfall 10's own guidance (~100-300m realistic
incident radius), not copied from the existing report-location constant.
**Warning signs:** If several distinct-looking test accounts at slightly different coordinates
within ~50m of each other all land in the same voter geohash cell and the independence gate never
opens, the precision is too coarse for this project's actual incident scale.

## Code Examples

### Vote-cast handler, mounted four times with one factory (matches D-15's "same surface" requirement)

```go
// internal/api/handlers/votes.go
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// CastVoteRequest is the one request body shape every confirm/dispute/
// resolve/reopen endpoint accepts — the client sends only raw coordinates,
// never a geohash (server computes it, per report.go's existing
// server-decides pattern).
type CastVoteRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type CastVoteResponse struct {
	Visibility string `json:"visibility"`
	Reason     string `json:"reason"`
}

// CastVote returns one handler per (kind, value) pair — four call sites in
// router.go, one small factory here, so the JSON contract, error mapping,
// and account-resolution logic exist exactly once.
func CastVote(svc *service.VotingService, kind service.VoteKind, value service.VoteValue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		reportID, ok := parseReportIDParam(w, r)
		if !ok {
			return
		}

		var req CastVoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		vis, reason, err := svc.CastVote(r.Context(), service.CastVoteInput{
			ReportID:  reportID,
			AccountID: acc.ID,
			Kind:      kind,
			Value:     value,
			Latitude:  req.Latitude,
			Longitude: req.Longitude,
		})
		if err != nil {
			if errors.Is(err, service.ErrCannotVoteOwnReport) {
				writeFieldError(w, http.StatusForbidden, "account", "You can't vote on your own report.")
				return
			}
			log.Printf("handlers: CastVote: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, CastVoteResponse{Visibility: string(vis), Reason: reason})
	}
}
```

```go
// internal/api/router.go — inside the existing gated r.Group
r.Route("/api/reports/{id}", func(r chi.Router) {
	r.Post("/confirm", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteConfirm))
	r.Post("/dispute", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteDispute))
	r.Post("/resolve", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteResolve))
	r.Post("/reopen", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteReopen))
})
```

### GPS capture with session-scoped caching and D-18's hard block on denial

```javascript
// web/static/js/votes.js — deliberately NOT reusing modal.js's initLocation()
// fallback pattern: that pattern exists so report SUBMISSION is never
// blocked by a denied GPS prompt (correct for that flow). D-18 requires the
// opposite for voting — a denied prompt blocks the vote entirely, with no
// fallback location, because every counted vote must be meaningfully
// location-backed.
(function () {
  'use strict';

  var CACHE_KEY = 'pinalert_voter_geo';

  // getVoterLocation resolves {latitude, longitude} once per browser
  // session (D-17), or rejects if the visitor has denied/lacks geolocation
  // (D-18) — callers must treat rejection as a hard stop, never a fallback.
  function getVoterLocation() {
    var cached = sessionStorage.getItem(CACHE_KEY);
    if (cached) {
      return Promise.resolve(JSON.parse(cached));
    }
    if (!navigator.geolocation) {
      return Promise.reject(new Error('geolocation_unsupported'));
    }
    return new Promise(function (resolve, reject) {
      navigator.geolocation.getCurrentPosition(
        function (pos) {
          var coords = { latitude: pos.coords.latitude, longitude: pos.coords.longitude };
          sessionStorage.setItem(CACHE_KEY, JSON.stringify(coords));
          resolve(coords);
        },
        function () {
          reject(new Error('geolocation_denied'));
        }
      );
    });
  }

  // castVote waits for the server response before resolving (D-04 — no
  // optimistic update); a denied GPS prompt rejects before any network
  // request is made at all.
  function castVote(reportId, action) {
    return getVoterLocation().then(function (coords) {
      return fetch('/api/reports/' + reportId + '/' + action, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(coords)
      }).then(function (res) {
        return res.json().then(function (body) {
          if (!res.ok) {
            var err = new Error((body && body.error && body.error.message) || 'Vote failed.');
            throw err;
          }
          return body;
        });
      });
    });
  }

  window.Pinalert = window.Pinalert || {};
  window.Pinalert.castVote = castVote;
}());
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `navigator.permissions.query({name: 'geolocation'})` as the primary permission-check mechanism | `getCurrentPosition()`'s native prompt/callback flow, `permissions.query` only as an optional pre-check | Ongoing — Safari's support for querying the `'geolocation'` permission name specifically has historically lagged Chromium (per [MDN](https://developer.mozilla.org/en-US/docs/Web/API/Permissions/query), [web.dev](https://web.dev/articles/permissions-best-practices)) `[CITED: developer.mozilla.org, web.dev]` | Do not gate the vote button's availability on a `permissions.query` result — always fall through to `getCurrentPosition`'s own grant/deny outcome, which works uniformly everywhere |

**Deprecated/outdated:** None specific to this phase's stack — `pgx/v5`, `chi/v5`, and
`mmcloughlin/geohash` are all current, already-audited choices from Phase 1's own research pass.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Reopening (D-16) has **no** reporter-instant path symmetric to D-13's reporter-instant resolve — every reopen, including the original reporter's own, requires the shared independent-agreement threshold. This is the literal reading of D-16's text ("Reopening requires the same independent-agreement threshold as D-14"), but CONTEXT.md doesn't explicitly rule out a symmetric reporter shortcut. | `Resolve()`/`VoteTally.isRetracted()`, Pattern 2 | If wrong, a reporter who mistakenly marked their own still-active report resolved has no fast way to self-correct — they'd need 2 independent confirmers to reopen it, same as anyone else. Low severity (a UX friction point, not a trust-model break) but worth confirming with the user before planning locks it in. |
| A2 | `IndependentAgreementThreshold = 2` is the correct concrete number for D-05/D-06/D-14/D-16's shared floor, derived from TRUST-04's own explicit "second independent confirmation" wording plus Pitfall 4's "cost more than the value of gaming it" framing for a low-traffic portfolio demo. | Concrete Threshold Recommendation (below), `Resolve()` | If the real deployment sees meaningfully more traffic than a portfolio-demo volume, 2 may be too low a bar against coordinated multi-account gaming — this is flagged in `PITFALLS.md` Pitfall 4 as an accepted, documented limitation, not a claim of being unbeatable. |
| A3 | `voterGeohashPrecision = 7` (~153m × 153m cells) is a reasonable interim choice against Pitfall 10's ~100-300m incident-radius guidance, pending Phase 3's benchmarked precision decision. | Pattern 3, Pitfall 3 | If too coarse, genuinely independent nearby witnesses could be suppressed into one cell (Pitfall 10's exact failure mode); if too fine, a single determined attacker moving a short distance could too easily generate "distinct" cells. Phase 3 is explicitly scoped to revisit this with real benchmarking. |
| A4 | Votes cast on an already-expired report should be rejected at the service layer (not explicitly stated in any TRUST requirement or CONTEXT.md decision, but implied by the trust mechanic existing only for live/provisional reports). | `VotingService.CastVote` (not yet coded in the example above — flagged for the planner to make an explicit task) | If unhandled, a vote on an expired report is harmless (it's never included in any resolver-driven feed since expiry is a separate filter) but wastes a write and could confuse a debugging session later. Low risk either way — the planner should make an explicit, small decision here rather than leaving it implicit. |

## Open Questions

1. **Does reopening (D-16) grant the original reporter an instant path, symmetric to D-13's
   instant resolve?**
   - What we know: D-13 explicitly grants the reporter an instant, threshold-free resolve action.
     D-16 says reopening "requires the same independent-agreement threshold as D-14" with no
     explicit reporter carve-out.
   - What's unclear: whether this asymmetry (instant resolve, but never instant reopen — even for
     the reporter correcting their own mistake) was a deliberate discussion outcome or an
     unstated gap.
   - Recommendation: implement per the literal text (no reporter-instant-reopen) as the default,
     but flag this explicitly for a one-line confirmation during planning or with the user before
     the plan-checker signs off — it's a two-line code change either way, not worth blocking on,
     but worth a deliberate yes/no rather than a silent default.

2. **Should a vote endpoint reject votes on an already-expired report, or is a vote on an expired
   report simply harmless/inert?**
   - What we know: expiry is a separate, existing read-time predicate; a vote on an expired report
     would never surface through any resolver-driven read path regardless of whether it was
     accepted.
   - What's unclear: whether accepting-but-ignoring vs. rejecting-with-a-4xx is the intended UX
     when this edge case is hit (e.g., a vote submitted just as a report crosses its expiry
     instant).
   - Recommendation: reject with a small, explicit 4xx ("This report has expired.") — cheap to
     add, avoids a confusing silent-no-op, and is consistent with this codebase's existing
     server-side-validates-everything convention (see `ValidateSubmitInput`).

## Runtime State Inventory

Not applicable — this is a greenfield feature addition (new table, new endpoints, new frontend
module), not a rename/refactor/migration phase. No existing runtime state (stored data, live
service config, OS-registered state, secrets, build artifacts) is being renamed or moved.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| PostgreSQL (already provisioned, existing `DATABASE_URL`) | `votes` table, concurrency tests | ✓ | 16/17 (existing) | — |
| Browser `navigator.geolocation` | GPS capture for the independence predicate (D-17/D-18) | ✓ (universal in evergreen browsers) | — | None by design — D-18 requires blocking, not falling back, on denial |
| Browser `navigator.permissions` (`'geolocation'` query) | Optional pre-check only, not load-bearing | Partial (inconsistent on Safari, per MDN) | — | `getCurrentPosition()`'s own native flow, used unconditionally regardless of this API's availability |

**Missing dependencies with no fallback:** none — the one "no fallback" item above
(`navigator.geolocation` denial) is an intentional design choice (D-18), not a gap.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (no third-party test framework anywhere in this repo) |
| Config file | none — `make test` / `make test-short` wrap plain `go test` invocations |
| Quick run command | `go test ./... -short` (skips every test needing `DATABASE_URL`) |
| Full suite command | `go test ./... -v -p 1` (requires `DATABASE_URL`; `-p 1` is required — see `Makefile`/CI comment: tests share one Postgres and each calls `testutil.NewTestDB`'s `TRUNCATE ... RESTART IDENTITY`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TRUST-01 | Confirm/dispute cast, stored, reflected in tally | unit + integration | `go test ./internal/service/... ./internal/store/... -run TestCastVote` | ❌ Wave 0 |
| TRUST-02 | `Resolve()` gives identical output regardless of caller | unit (table-driven) | `go test ./internal/service/... -run TestResolve` | ❌ Wave 0 |
| TRUST-03 | Only distinct-account + distinct-cell votes count | unit | `go test ./internal/service/... -run TestIndependentCellCount` | ❌ Wave 0 |
| TRUST-04 | Non-critical Provisional until 2nd independent confirm; critical instant | unit | `go test ./internal/service/... -run TestResolve_ProvisionalGate` | ❌ Wave 0 |
| TRUST-06 | Severity affects triage sort only, never bypasses the gate for non-critical | unit | `go test ./internal/service/... -run TestResolve_SeverityNeverBypassesGateAlone` | ❌ Wave 0 |
| TRUST-08 | Reporter-instant-resolve; confirmer-resolve needs independent agreement | integration | `go test ./internal/store/... -run TestResolveReopen` | ❌ Wave 0 |
| TRUST-09 | N concurrent votes on one report never silently drop one | integration/concurrency | `go test ./internal/store/... -run TestCastVoteConcurrent -p 1` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./... -short`
- **Per wave merge:** `go test ./... -v -p 1` (requires `DATABASE_URL`)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/service/visibility_test.go` — table-driven `Resolve()` tests covering: critical
      bypass over Hidden-by-dispute, critical bypass over Provisional, Hidden reversibility
      (confirms later outweighing disputes flips back to Live/Provisional in the same test via two
      calls with different tallies), Retracted taking precedence over everything except reopen,
      reopen flipping Retracted back to the live pipeline.
- [ ] `internal/service/trust_test.go` — `independentCellCount()` unit tests (empty, all-same-cell,
      all-distinct, mixed) and `BuildVoteTally()` tests against fabricated `CurrentVotesForReports`
      rows (including the reporter's own resolution vote being excluded from `ResolveCells` and
      surfaced via `ReporterResolved` instead).
- [ ] `internal/store/votes_test.go` — mirrors `internal/store/cooldown_test.go`'s structure
      exactly: `TestCastVoteConcurrent` (N goroutines, N distinct accounts, one report, assert
      `COUNT(*) = N`), `TestCurrentVoteIsLatestOnAccountChange` (same account votes confirm then
      dispute, assert exactly one current row and it's the later one), `TestVotesArePerReportPerAccountKind`
      (a content vote and a resolution vote from the same account on the same report don't
      interfere with each other's "current" read).
- [ ] `internal/api/handlers/votes_e2e_test.go` — end-to-end for the 403 on reporter self-vote
      (D-03) and the 401 on an unverified caller (reusing the existing gated-route test pattern
      from `reports_e2e_test.go`).
- Framework install: none — `go test` is already the only tool this repo uses.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (reused, not new) | Already-built Phase 1.1 verified-account gate (`requireVerifiedAccount`) — every vote route mounts inside the existing gated `r.Group`, no new auth mechanism |
| V3 Session Management | no (reused, not new) | Same signed-cookie session as every other route; nothing new to verify |
| V4 Access Control | yes | New checks this phase must add: (a) reporter cannot cast a content vote on their own report (D-03), enforced server-side in `VotingService.CastVote`, never only by hiding the button client-side; (b) resolve/reopen require either reporter identity match or the independent-agreement threshold — never a bare "any account" check |
| V5 Input Validation | yes | Vote `kind`/`value` validated against a closed enum server-side (mirrors `Category.Valid()`/`Severity.Valid()`'s existing pattern); latitude/longitude range-validated the same way `parseCoordinate` already does for `GET /api/reports` |
| V6 Cryptography | no (reused, not new) | No new cryptographic primitive — session HMAC signing is untouched by this phase |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Sybil/multi-account vote stuffing from one physical location | Spoofing | GPS-derived `geohash_cell` independence check (distinct cells required) stacked on top of Phase 1.1's mandatory email verification — raises the cost of manufacturing additional "independent" identities beyond a bare cookie-clear (the exact gap Phase 1.1 was inserted to close) |
| GPS spoofing (devtools location override, mock-location apps) | Tampering | Documented, not fully preventable at this project's budget (`PITFALLS.md` Pitfall 4) — mitigated, not solved, by requiring both a distinct verified account AND a distinct geohash cell; do not claim "fraud-proof" in any UI copy per Pitfall 4's explicit guidance |
| Reporter self-confirming their own report | Tampering (of the report's own trust score) | Server-side rejection in `VotingService.CastVote` comparing the caller's account against the report's reporter account (via the existing `sessions.account_id` join) — never a client-side-only disabled button |
| Race condition on concurrent votes (`PITFALLS.md` Pitfall 2) | Tampering / Repudiation (a vote silently not counted) | Structurally avoided by the append-only, no-cached-counter schema (Pattern 1) rather than by locking — see Pitfall 2 above |
| Unauthorized resolve/reopen by a single non-reporter account | Elevation of Privilege | `Resolve()`'s `isRetracted()` requires either exact reporter-account match or `IndependentAgreementThreshold` distinct cells — enforced entirely server-side, the resolver never trusts a client-asserted "resolved" flag |
| Forged/omitted `geohash_cell` from the client | Tampering | Server computes `geohash_cell` itself from the raw `{latitude, longitude}` POST body; any client-submitted geohash string is ignored — extends `report.go`'s existing "server decides" convention (`ExpiresAt`, `Geohash`, `CreatedAt` are all server-computed there too) |

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection: `internal/store/migrations/00001-00003*.sql`, `internal/store/queries/*.sql`, `internal/store/sqlc/*.sql.go`, `internal/service/report.go`, `internal/api/handlers/reports.go`, `internal/api/router.go`, `internal/api/gate.go`, `internal/session/cookie.go`, `internal/account/account.go`, `internal/store/cooldown_test.go`, `internal/testutil/db.go`, `web/static/js/{app,feed,map,modal}.js`, `go.mod`, `Makefile`, `.github/workflows/ci.yml`, `sqlc.yaml` — all read directly this session, not inferred `[VERIFIED: direct file read]`
- `.planning/research/ARCHITECTURE.md` — `VisibilityResolver` pattern, Pattern 1/2/3, Anti-Patterns 1-3, project structure recommendation `[CITED: project research]`
- `.planning/research/PITFALLS.md` — Pitfall 2 (race condition), Pitfall 4 (Sybil), Pitfall 9 (severity bypass), Pitfall 10 (geohash precision) `[CITED: project research]`
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-CONTEXT.md` — all 18 locked decisions (D-01 through D-18) `[CITED: project context]`

### Secondary (MEDIUM confidence)
- [MDN: Permissions.query()](https://developer.mozilla.org/en-US/docs/Web/API/Permissions/query) — Permissions API general shape `[CITED: developer.mozilla.org]`
- [web.dev: Web permissions best practices](https://web.dev/articles/permissions-best-practices) — request-after-user-action guidance, cross-browser caveats `[CITED: web.dev]`
- [MDN: Geolocation API](https://developer.mozilla.org/en-US/docs/Web/API/Geolocation_API) — `getCurrentPosition` grant/deny callback contract `[CITED: developer.mozilla.org]`

### Tertiary (LOW confidence)
- None — every claim above traces to either direct codebase inspection or a cited project research
  document; no claim in this file rests on unverified training-data recall alone (concrete
  threshold numbers are flagged `[ASSUMED]`/logged in the Assumptions table above, not presented
  as externally verified facts).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependency, every library already pinned and proven in this exact repo
- Architecture: HIGH — directly extends an already-authored, already-locked architecture document plus 18 explicit user decisions; the one genuinely novel call (rejecting the atomic-upsert pattern for votes) is derived from first-principles reasoning about the actual data-race shape, not guesswork
- Pitfalls: HIGH — Pitfall 2 (the phase's core concurrency risk) is directly named in this project's own prior research and is structurally avoided by this design, not merely mitigated

**Research date:** 2026-09-12
**Valid until:** 30 days (stable Go/Postgres stack; the two open questions/assumptions above should be confirmed during planning or with the user, not left to drift past that point)
