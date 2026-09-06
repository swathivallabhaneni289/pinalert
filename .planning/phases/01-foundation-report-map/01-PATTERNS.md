# Phase 1: Foundation — Report & Map - Pattern Map

**Mapped:** 2026-09-06
**Files analyzed:** 24 (new files, per RESEARCH.md's Recommended Project Structure)
**Analogs found:** 0 / 24

## Codebase State

This is a **greenfield project**. The repository at `/Users/swathivallabhaneni/code/pinalert`
contains only planning artifacts:

```
README.md
PROJECT-NOTES.md
.gitignore
.claude/CLAUDE.md
.planning/**  (REQUIREMENTS.md, PROJECT.md, STATE.md, ROADMAP.md, research/*, phases/*)
```

There is no `go.mod`, no `internal/`, no `web/`, no `cmd/`, and no prior commit history to search
(`git` is not even initialized in this directory). A repo-wide search confirms:

- No `.go`, `.sql`, `.html`, `.js`, or `.css` application source files exist anywhere.
- No `controllers`, `services`, `models`, `middleware`, `routes`, `components`, or `store`
  directories exist.
- No existing tests, migrations, or CI config exist.

**Conclusion: there are zero existing code analogs in this codebase for any file this phase will
create.** Every file in Phase 1 is a first-of-its-kind for this project. Searching further
(additional Glob/Grep passes) would not change this conclusion — the absence is total, not partial.

## What the Planner Should Use Instead

Because no in-repo analogs exist, **`.planning/phases/01-foundation-report-map/01-RESEARCH.md`'s
"Code Examples" section is the actual implementation reference for this phase**, not this
PATTERNS.md. Specifically:

| File/Area to Build | Reference in RESEARCH.md |
|---|---|
| `sqlc.yaml` | Code Examples → `sqlc.yaml` (pgx/v5) |
| `internal/store/queries/reports.sql` | Code Examples → Bounding-box + Haversine query |
| `internal/store/migrations/00001_create_reports.sql` | Code Examples → goose migration |
| `internal/session/cookie.go`, `internal/service/session.go` | Code Examples → Anonymous session cookie (HMAC-signed) |
| `web/static/js/map.js` | Code Examples → Leaflet draggable marker (GPS-prefilled) |
| `web/static/js/modal.js` (severity slider) | Code Examples → Accessible severity slider (D-05) |
| `internal/api/router.go` (chi + swagger) | Code Examples → chi + swaggo/http-swagger/v2 wiring |
| `.github/workflows/ci.yml` | Code Examples → GitHub Actions CI |
| `internal/service/report.go` (expires_at, validation) | Architecture Patterns → Pattern 1, Pattern 2 |

These are drawn from verified external sources (Go module proxy docs, official library examples,
W3C ARIA practices, GitHub Actions docs) and this project's own locked `ARCHITECTURE.md`/
`STACK.md`/`PITFALLS.md` — see RESEARCH.md's Sources section for provenance and confidence levels
per snippet.

## File Classification (for planner's reference — no analog column populated)

| New File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `cmd/server/main.go` | config/bootstrap | request-response | none | no analog |
| `internal/api/router.go` | route | request-response | none | no analog |
| `internal/api/handlers/reports.go` | controller | CRUD | none | no analog |
| `internal/api/handlers/page.go` | controller | request-response | none | no analog |
| `internal/service/report.go` | service | CRUD | none | no analog |
| `internal/service/session.go` | service | request-response | none | no analog |
| `internal/session/cookie.go` | middleware | request-response | none | no analog |
| `internal/store/db.go` | config | CRUD | none | no analog |
| `internal/store/queries/reports.sql` | model (SQL) | CRUD | none | no analog |
| `internal/store/migrations/00001_create_reports.sql` | migration | batch | none | no analog |
| `internal/store/sqlc/*` (generated) | model | CRUD | none | no analog (generated code) |
| `internal/testutil/db.go` | test utility | CRUD | none | no analog |
| `web/templates/index.html.tmpl` | component | request-response | none | no analog |
| `web/static/js/map.js` | component | event-driven | none | no analog |
| `web/static/js/modal.js` | component | event-driven | none | no analog |
| `web/static/js/feed.js` | component | streaming (polling) | none | no analog |
| `web/static/css/main.css` | config (styling) | n/a | none | no analog |
| `web/static/icons/*.svg` | asset | n/a | none | no analog (downloaded, not authored) |
| `sqlc.yaml` | config | n/a | none | no analog |
| `.github/workflows/ci.yml` | config | batch | none | no analog |
| `internal/service/report_test.go` | test | CRUD | none | no analog |
| `internal/session/cookie_test.go` | test | request-response | none | no analog |
| `internal/store/reports_test.go` | test | CRUD | none | no analog |
| `internal/api/handlers/reports_test.go` | test | request-response | none | no analog |

## Pattern Assignments

Not applicable — no in-repo analog exists for any file above. See "What the Planner Should Use
Instead" for the concrete external/research-sourced patterns to copy from per file.

## Shared Patterns

No shared in-repo patterns exist yet (no auth middleware, no error-handling wrapper, no logging
convention has been established in code). RESEARCH.md's Architecture Patterns section establishes
the following as the patterns Phase 1 itself should establish as project conventions (for later
phases to then treat as real analogs):

- **Error handling / validation:** hand-written Go validation functions returning typed
  `ErrValidation` errors (RESEARCH.md Pattern 2) — no third-party validation library.
- **Expiry-as-data pattern:** compute and materialize `expires_at` at write time; every read query
  filters `WHERE expires_at > now()` (RESEARCH.md Pattern 1).
- **Single-endpoint feed pattern:** one `GET /api/reports` serves both map and list consumers
  (RESEARCH.md Pattern 3) — later phases adding new read views should follow this rather than
  spawning parallel endpoints.
- **Session security:** HMAC-signed cookies via stdlib `crypto/hmac`/`crypto/sha256`, constant-time
  compare — this becomes the analog for any future session-touching code (e.g., Phase 2's
  reputation lookups).

## No Analog Found

All 24 files listed above have no analog — this is expected and correct for a Phase 1 greenfield
build. This is not a gap to remediate; it is the phase's designed starting condition.

## Metadata

**Analog search scope:** entire repository root (`/Users/swathivallabhaneni/code/pinalert`),
confirmed via directory listing and extension search (`.go`, `.sql`, `.js`, `.html`, `.css`).
**Files scanned:** 17 (all existing files in repo, all planning docs — zero application source
files found)
**Pattern extraction date:** 2026-09-06
