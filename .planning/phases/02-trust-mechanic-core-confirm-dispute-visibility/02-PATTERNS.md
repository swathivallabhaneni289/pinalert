# Phase 2: Trust Mechanic Core — Pattern Map

**Mapped:** 2026-09-15
**Files analyzed:** 12
**Analogs found:** 11 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/store/migrations/00004_create_votes.sql` | migration | CRUD (append-only) | `internal/store/migrations/00003_add_email_cooldowns.sql` | role-match (schema shape differs: no unique key here) |
| `internal/store/queries/votes.sql` | model (SQL) | CRUD | `internal/store/queries/email_cooldowns.sql` + `internal/store/queries/reports.sql` | role-match |
| `internal/store/sqlc/votes.sql.go` | model (generated) | CRUD | `internal/store/sqlc/email_cooldowns.sql.go` | exact (generated, follow same shape) |
| `internal/store/votes_test.go` | test (integration/concurrency) | CRUD | `internal/store/cooldown_test.go` | exact (concurrency-test shape reused directly) |
| `internal/service/visibility.go` | service (pure function) | transform | `internal/service/report.go` (enum/validation style) | role-match — no existing pure-resolver analog, but enum/const conventions transfer directly |
| `internal/service/visibility_test.go` | test (unit, table-driven) | transform | `internal/service/report_test.go` | role-match |
| `internal/service/trust.go` | service | CRUD + transform | `internal/service/report.go` (`ReportService`/`Querier` interface pattern) | exact (same service-construction idiom) |
| `internal/service/trust_test.go` | test (unit) | transform | `internal/service/report_test.go` | role-match |
| `internal/api/handlers/votes.go` | controller (HTTP handler factory) | request-response | `internal/api/handlers/reports.go` (`SubmitReport`) | exact |
| `internal/api/handlers/votes_e2e_test.go` | test (e2e) | request-response | `internal/api/handlers/reports_e2e_test.go` | exact |
| `internal/api/router.go` (extended) | route/middleware | request-response | itself (existing gated `r.Group`) | exact — modify in place |
| `web/static/js/votes.js` | frontend module (GPS + fetch) | request-response | `web/static/js/modal.js` (`initLocation`, contrast case) + `web/static/js/feed.js` (`createRow`/`updateRow`/event delegation) | role-match (deliberately inverse behavior from modal.js on GPS denial) |

## Pattern Assignments

### `internal/store/migrations/00004_create_votes.sql` (migration, append-only)

**Analog:** `internal/store/migrations/00003_add_email_cooldowns.sql`

**Structure to copy** (goose Up/Down convention, comment-heavy explaining *why* the schema shape is what it is):
```sql
-- +goose Up
CREATE TABLE email_cooldowns (
    email TEXT PRIMARY KEY,
    last_requested_at TIMESTAMPTZ NOT NULL
);
-- +goose Down
DROP TABLE email_cooldowns;
```

**Key difference from the analog (do not copy this part):** `email_cooldowns` has a `PRIMARY KEY (email)` because it is a single-mutable-row-per-key table requiring `ON CONFLICT` serialization. `votes` must NOT have an equivalent unique constraint — it is append-only. Copy the goose Up/Down comment convention and the "why this shape" comment style, not the PK-per-key structure. Per RESEARCH.md Pattern 1, the actual shape is:
```sql
CREATE TABLE votes (
    id            BIGSERIAL PRIMARY KEY,
    report_id     BIGINT NOT NULL REFERENCES reports(id),
    account_id    BIGINT NOT NULL REFERENCES accounts(id),
    kind          TEXT NOT NULL,
    value         TEXT NOT NULL,
    geohash_cell  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_votes_report_account_kind_created
    ON votes (report_id, account_id, kind, created_at DESC, id DESC);
```
Reference `internal/store/migrations/00001_create_reports.sql` and `00002_add_accounts_and_verification.sql` for the `reports`/`accounts` table shapes the FKs point at (both already read this session per RESEARCH.md's source list).

---

### `internal/store/queries/votes.sql` (model, CRUD)

**Analogs:** `internal/store/queries/email_cooldowns.sql` (comment-density/rationale style) and `internal/store/queries/reports.sql` (multi-row/batched-read query style — read that file for `NearbyReports`'s `sqlc.arg`/`ANY($1)` batching convention before writing `CurrentVotesForReports`).

**Imports/declaration pattern** (lines 1-16 of `email_cooldowns.sql`):
```sql
-- name: ClaimEmailCooldown :one
-- <rationale comment explaining WHY this exact SQL shape, tied to a DEC-/T- reference>
INSERT INTO email_cooldowns (email, last_requested_at)
VALUES (sqlc.arg(email), sqlc.arg(requested_at))
ON CONFLICT (email) DO UPDATE
SET last_requested_at = EXCLUDED.last_requested_at
WHERE email_cooldowns.last_requested_at <= sqlc.arg(cooldown_cutoff)
RETURNING last_requested_at;
```

**What to build instead (per RESEARCH.md Pattern 1/3), following the same `-- name:`/rationale-comment convention:**
- `InsertVote :exec` — plain `INSERT INTO votes (...) VALUES (...)`, no `ON CONFLICT`.
- `CurrentVotesForReports :many` — `SELECT DISTINCT ON (report_id, account_id, kind) * FROM votes WHERE report_id = ANY(sqlc.arg(report_ids)::bigint[]) ORDER BY report_id, account_id, kind, created_at DESC, id DESC` (mirrors `reports.sql`'s batched-read shape, not `email_cooldowns.sql`'s single-key shape).
- `ReporterAccountID :one` — joins `reports` → `sessions` → `accounts.id`, following whatever join `internal/store/queries/reports.sql` or `internal/store/queries/accounts.sql` already uses for session→account resolution (check `accounts.sql` for the existing join shape before writing a new one).

---

### `internal/store/sqlc/votes.sql.go` (generated)

**Analog:** `internal/store/sqlc/email_cooldowns.sql.go` — do not hand-write; run `sqlc generate` (or `make sqlc`) per RESEARCH.md's Installation note after `votes.sql` is written. Verify the generated `Queries` methods match the `VotingQuerier` interface shape in `trust.go`.

---

### `internal/store/votes_test.go` (integration/concurrency test)

**Analog:** `internal/store/cooldown_test.go` — copy this file's structure almost directly.

**Setup pattern** (lines 1-24):
```go
package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

func TestCastVoteConcurrent(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	q := sqlcgen.New(pool)
	...
}
```

**Concurrency pattern to copy** (lines 29-62 of `cooldown_test.go` — goroutines + `sync.WaitGroup` + `sync.Mutex`-guarded counters):
```go
var wg sync.WaitGroup
var mu sync.Mutex
successCount := 0

for i := 0; i < N; i++ {
	wg.Add(1)
	go func(i int) {
		defer wg.Done()
		err := q.InsertVote(ctx, sqlcgen.InsertVoteParams{ /* distinct account per i */ })
		mu.Lock()
		defer mu.Unlock()
		if err == nil {
			successCount++
		} else {
			t.Errorf("unexpected InsertVote error: %v", err)
		}
	}(i)
}
wg.Wait()

var count int
pool.QueryRow(ctx, `SELECT COUNT(*) FROM votes WHERE report_id = $1`, reportID).Scan(&count)
if count != N {
	t.Fatalf("count = %d, want %d", count, N)
}
```

**Difference from the analog:** `cooldown_test.go` asserts exactly ONE winner (serialized contention). `TestCastVoteConcurrent` must assert ALL N inserts succeed (no contention at all — append-only, per RESEARCH.md Pitfall 2's "stronger and simpler guarantee" framing). Do not copy the win/lose assertion shape, only the goroutine/WaitGroup harness shape.

Also model `TestCurrentVoteIsLatestOnAccountChange` and `TestVotesArePerReportPerAccountKind` (named directly in RESEARCH.md's Wave 0 Gaps) on `TestClaimEmailCooldownAllowsAfterWindow`'s sequential-then-assert style (lines 112-142).

---

### `internal/service/visibility.go` (pure resolver)

**No direct analog exists in this codebase** (this is the first pure, DB-free resolver function). Follow `internal/service/report.go`'s conventions for:

**Package doc comment style** (lines 1-7) — extend, don't replace:
```go
// Package service holds Pinalert's report business logic: ...
```

**Enum + `Valid()` pattern to copy exactly** (lines 79-100 of `report.go`):
```go
type Severity string
const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityCritical Severity = "critical"
)
var Severities = []Severity{SeverityLow, SeverityMedium, SeverityCritical}
func (s Severity) Valid() bool { ... }
```
Use this exact shape for the new `Visibility` type's constants (`VisibilityHidden`, `VisibilityProvisional`, `VisibilityLive`, `VisibilityRetracted`) — see RESEARCH.md Pattern 2's code example, which already follows this convention.

**Doc-comment-as-spec style** — `report.go`'s comments cite exact decision IDs (D-14, FOUND-03). Continue this in `visibility.go`: cite D-05/D-06/D-07/D-08/TRUST-02/TRUST-04/TRUST-06 inline exactly as RESEARCH.md's own `Resolve()` example already does (copy that example directly — it was written against this exact codebase's conventions).

---

### `internal/service/trust.go` (VotingService)

**Analog:** `internal/service/report.go`'s `ReportService`/`Querier`/`NewReportService`/`Submit` construction (lines 270-330).

**Interface + constructor pattern to copy exactly:**
```go
type Querier interface {
	InsertReport(ctx context.Context, arg sqlcgen.InsertReportParams) (sqlcgen.InsertReportRow, error)
	NearbyReports(ctx context.Context, arg sqlcgen.NearbyReportsParams) ([]sqlcgen.NearbyReportsRow, error)
}
type ReportService struct{ q Querier }
func NewReportService(q Querier) *ReportService { return &ReportService{q: q} }
```
→ becomes `VotingQuerier` / `VotingService` / `NewVotingService` exactly as RESEARCH.md's Pattern 3 code example shows (already written against this convention — copy it directly, including the `independentCellCount` helper and the `voterGeohashPrecision = 7` constant with its comment explaining why it differs from `report.go`'s `geohashPrecision = 8`).

**Server-computed-field pattern to copy** (lines 296-299 of `report.go` — never trust client input for anything server-authoritative):
```go
now := time.Now().UTC()
expiresAt := now.Add(ExpiryDuration(in.Severity))
gh := geohash.EncodeWithPrecision(in.Latitude, in.Longitude, geohashPrecision)
```
→ `trust.go`'s `CastVote` computes `geohash_cell` server-side from raw lat/lon the same way — never accept a client-supplied geohash string (RESEARCH.md's Security Domain table, "Forged/omitted geohash_cell" row).

**ValidationError pattern to reuse as-is** (lines 125-135) — `ErrCannotVoteOwnReport` should follow the same sentinel-error-plus-`errors.Is`/`errors.As` convention already used by `service.ValidationError`.

---

### `internal/api/handlers/votes.go` (controller)

**Analog:** `internal/api/handlers/reports.go`'s `SubmitReport` (lines 127-198).

**Imports pattern** (lines 1-19):
```go
import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"pinalert/internal/service"
	"pinalert/internal/session"
)
```

**Auth/identity-resolution pattern** (lines 152-156 — resolve caller identity from context, 500 if absent):
```go
sessionID, ok := session.FromContext(r.Context())
if !ok {
	http.Error(w, "internal server error", http.StatusInternalServerError)
	return
}
```
For votes, use `account.FromContext` / `api.AccountFromContext` (per `gate.go` lines 27-40) since votes need the verified `account.ID`, not the raw session id.

**Body decode + validation-error mapping pattern** (lines 158-194):
```go
r.Body = http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes)
dec := json.NewDecoder(r.Body)
dec.DisallowUnknownFields()
var req SubmitReportRequest
if err := dec.Decode(&req); err != nil {
	writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
	return
}
...
report, err := svc.Submit(r.Context(), sessionID, in)
if err != nil {
	var ve service.ValidationError
	if errors.As(err, &ve) {
		writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
		return
	}
	log.Printf("handlers: SubmitReport: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
	return
}
writeJSON(w, http.StatusCreated, SubmitReportResponse{Report: reportToResponse(report)})
```
This is exactly the shape RESEARCH.md's `CastVote` handler-factory example already follows (including the `errors.Is(err, service.ErrCannotVoteOwnReport)` → 403 branch) — copy that example directly, reusing `writeFieldError`/`writeJSON` from `reports.go` (same package, already defined, do not redeclare).

**Response-shape declaration convention** (lines 61-76, 116-125) — declare `CastVoteResponse`/`ErrorResponse` field-by-field with swag annotations, never alias a store row, exactly as `ReportResponse`/`ErrorResponse` do.

---

### `internal/api/handlers/votes_e2e_test.go`

**Analog:** `internal/api/handlers/reports_e2e_test.go` — read this file directly before writing (not yet read this session; same package/test-harness conventions as `swagger_test.go`/`page_test.go`). Reuse whatever `testutil` DB/router-bootstrap helper `reports_e2e_test.go` uses for spinning up a full `chi.Mux` + Postgres-backed request. Test the 403 (D-03 self-vote) and 401 (unverified caller, reusing the gate's existing behavior) paths named explicitly in RESEARCH.md's Wave 0 Gaps.

---

### `internal/api/router.go` (extend in place)

**Analog:** itself — the existing gated `r.Group` (lines 157-173).

**Pattern to extend exactly:**
```go
r.Group(func(r chi.Router) {
	r.Use(requireVerifiedAccount(deps.Sessions))
	r.Get("/", handlers.Page(deps.Template, deps.Page))
	r.Post("/api/reports", handlers.SubmitReport(deps.Reports))
	r.Get("/api/reports", handlers.NearbyReports(deps.Reports))
	...
})
```
Add inside this same `r.Group` (never outside it — every gated route must stay inside by construction, per the router's own doc comment on "a route added to the group is protected by construction"):
```go
r.Route("/api/reports/{id}", func(r chi.Router) {
	r.Post("/confirm", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteConfirm))
	r.Post("/dispute", handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteDispute))
	r.Post("/resolve", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteResolve))
	r.Post("/reopen", handlers.CastVote(deps.Votes, service.VoteKindResolution, service.VoteReopen))
})
```
Add a `Votes *service.VotingService` field to `Deps` (mirrors the existing `Reports *service.ReportService` field, line 32) and wire it in `cmd/server/main.go` (not read this session — check its existing `Deps{}` literal construction site before adding the field, since RESEARCH.md notes the router already tracks a `Dev`/`RequestLinkRateLimit` field-addition precedent for exactly this kind of additive `Deps` change).

---

### `web/static/js/votes.js`

**Analog for GPS-capture shape (structure only, NOT behavior on denial):** `web/static/js/modal.js`'s `initLocation()` (~line 340-410) and `web/static/js/map.js`'s `centerOnVisitor()` (~line 103-130) — both use the same `navigator.geolocation.getCurrentPosition(successCb, errorCb)` call shape.

**Critical divergence (explicitly called out in RESEARCH.md's Anti-Patterns and the UI-SPEC's Interaction Contract step 2):** `modal.js`/`map.js` fall back silently to a default location on denial (correct for report submission — must never block). `votes.js` must do the OPPOSITE: denial is a hard stop, per D-18. Do not copy the fallback branch. Use RESEARCH.md's own `votes.js` code example directly (already written against this exact divergence) — it is provided in full in `02-RESEARCH.md`'s "Code Examples" section (`getVoterLocation()`/`castVote()`), lines ~600-663.

**Analog for feed-row DOM construction/update and event delegation:** `web/static/js/feed.js`'s `createRow`/`updateRow` (lines 122-186) — element-creation-once, class/text-update-on-poll split; new `.vote-controls`/`.visibility-tag`/`.vote-error` elements should be created once in an equivalent `createRow`-style function and updated (not recreated) on each poll, exactly as `updateRow` does for `sev-`/`age-` classes via `replacePrefixedClass`. Use the same `replacePrefixedClass`-style helper for swapping `vis-provisional`/`vis-hidden` classes.

**Analog for map popup content construction:** `web/static/js/map.js`'s `buildPopupContent(report)` (~line 157) — append the same `.vote-controls` block markup here as in `feed.js`'s row, per D-01 ("both surfaces") and the UI-SPEC's explicit instruction that both surfaces share identical markup/classes so one `votes.js` event-delegation script handles both.

---

## Shared Patterns

### Server-side-only trust boundary ("server decides, client never claims")
**Source:** `internal/service/report.go` lines 137-148 (`SubmitInput` comment: "expires_at, created_at, geohash, and id are deliberately absent — computed server-side") and `internal/api/handlers/reports.go` lines 33-39 (`DisallowUnknownFields` + explicit field-by-field request struct).
**Apply to:** `votes.go`/`trust.go` — `CastVoteRequest` accepts only raw `{latitude, longitude}`; `geohash_cell`, `visibility`, and vote validity are always server-computed, never client-supplied, per RESEARCH.md's Security Domain table.

### Sentinel error + `errors.As`/`errors.Is` → field-level 4xx mapping
**Source:** `internal/service/report.go`'s `ValidationError` (lines 125-135) consumed by `internal/api/handlers/reports.go` lines 186-190 (`errors.As(err, &ve)` → `writeFieldError`).
**Apply to:** `trust.go`'s `ErrCannotVoteOwnReport` (D-03) and any "report expired" error (Open Question 2's recommended explicit 4xx) consumed identically in `votes.go`.

### Verified-account gate, never re-implemented per-route
**Source:** `internal/api/gate.go`'s `requireVerifiedAccount` + `internal/api/router.go`'s single gated `r.Group` (lines 157-173).
**Apply to:** All four new vote routes mount inside the existing `r.Group` — no new auth mechanism, no per-route auth check duplicated in `votes.go` (identity is read from context via `account.FromContext`, exactly as `handlers.Page`/other gated handlers already do).

### Append-only log + live recompute (no cached counter)
**Source:** RESEARCH.md Pattern 1, contrasted explicitly against `internal/store/queries/email_cooldowns.sql`'s `ON CONFLICT DO UPDATE` (the pattern this phase deliberately does NOT reuse, despite surface similarity).
**Apply to:** `votes.sql`, `trust.go`, `visibility.go` — no `reports.status`/`confirm_count` column, no `SELECT ... FOR UPDATE`; visibility is always recomputed from the vote log on read.

### swag/OpenAPI doc-comment annotations on every handler
**Source:** `internal/api/handlers/reports.go` lines 136-149 (`@Summary`/`@Param`/`@Success`/`@Failure`/`@Router` godoc-style annotations above each exported handler).
**Apply to:** `votes.go`'s `CastVote` factory (or the four mounted instances) — annotate consistently so `docs/swagger.json`/`docs/swagger.yaml` regeneration (`swag init`) picks up the new routes, matching OPS-01's existing requirement.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/service/visibility.go` | service (pure function) | transform | No existing pure, DB-free resolver function exists in this codebase yet — this is architecturally novel to Phase 2 (per `ARCHITECTURE.md`'s `VisibilityResolver` pattern). RESEARCH.md's own fully-worked code example (already written against this repo's exact conventions) is the correct reference instead of a codebase analog. |

## Metadata

**Analog search scope:** `internal/store/`, `internal/service/`, `internal/api/`, `web/static/js/`, `internal/api/handlers/`
**Files scanned:** ~35 (full repo file listing) + 10 read in full for pattern extraction
**Pattern extraction date:** 2026-09-15
