---
phase: 02
slug: trust-mechanic-core-confirm-dispute-visibility
mode: mvp
granularity: standard
plans: 8
waves: 7
created: 2026-09-15
updated: 2026-09-15
---

# Phase 2 — Plan Outline

> **Updated post-planning.** 02-03 self-split into 02-03a (service layer) + 02-03b (HTTP layer)
> during the detailed per-plan pass — it was too large for one plan at full deep_work_rules
> detail. That shifted every downstream wave by one (7 waves total, not 6). The table below
> reflects what was actually planned and committed; see "Cross-plan contracts" for what changed
> from the original assumptions below it.

| Plan ID | Objective | Wave | Depends On | Requirements |
|---------|-----------|------|------------|--------------|
| 02-01 | A report's Hidden/Provisional/Live/Retracted state is decided by exactly one pure `Resolve()` function — critical/rescue-needed bypasses both the Provisional gate and the Hidden-by-dispute trigger, Hidden is fully reversible from live tallies, Retracted outranks everything except an agreed reopen — proven by table-driven tests over fabricated tallies (D-05..D-09, D-14, D-16) | 1 | none | TRUST-02, TRUST-04, TRUST-06 |
| 02-02 | A vote is durably appended to a log that has no shared mutable counter to race on: N concurrent casts from N distinct accounts on one report all land (TRUST-09 proof), the current effective vote per (report, account, kind) reads back as the latest row, and content vs. resolution votes never interfere (D-02) | 1 | none | TRUST-01, TRUST-09 |
| 02-03a | Service layer: `VotingService.CastVote` — voter geohash cell computed server-side from raw lat/lon, the reporter blocked from voting on their own report (content votes only), expired reports rejected, independence counted once by one shared helper reused at every gate (D-03, D-13, D-17) | 2 | 02-01, 02-02 | TRUST-01, TRUST-03, TRUST-08, TRUST-09 |
| 02-03b | HTTP layer: the four vote routes (confirm/dispute/resolve/reopen) mounted from one `handlers.CastVote` factory inside the existing gated `r.Group`, `main.go` wiring, e2e tests, swagger regen | 3 | 02-03a | TRUST-01, TRUST-03, TRUST-08 |
| 02-04 | Feed and map reads return the identical resolver-computed visibility for the same report, plus the viewer's own current vote and an is-own-report flag so the client can honour D-03 — Retracted never appears, Hidden appears only under `?show_disputed=true`, and severity still drives triage order only (D-03, D-08, D-10, D-11, D-12) | 4 | 02-03b | TRUST-02, TRUST-04, TRUST-06, TRUST-08 |
| 02-05 | First user-clickable increment: a visitor taps Confirm or Dispute on a feed row or a map pin popup, the browser captures GPS once per session and hard-blocks the vote on denial, the UI waits for the server before changing anything, and the tapped state reflects the server's answer (D-01, D-02, D-04, D-17, D-18) | 5 | 02-04 | TRUST-01, TRUST-03 |
| 02-06 | Trust state is legible without reading code: Provisional rows/pins render dimmed *and* labelled "Unconfirmed", "Show disputed reports" reveals outline-treated Hidden rows *and* Hidden map pins from the same shared query param, and an empty result explains itself (D-09, D-10, D-11) | 6 | 02-05 | TRUST-02, TRUST-04 |
| 02-07 | A report can be taken out of the live feed and put back: "Mark resolved" with an inline destructive confirmation on both surfaces (instant for the reporter, threshold-gated proposal for a confirmer, outcome copy driven by the server's answer), plus the Activity page's real trust state and "Reopen · not actually resolved" (D-12, D-13, D-15, D-16) | 7 | 02-06 | TRUST-08 |

## MVP framing

This phase is **one vertical slice** — "a user can confirm, dispute, and resolve reports, and see
the trust state that results" — not seven independent features. The seven plans are sequential
steps through that single slice; the first user-clickable increment lands at **02-05**, and every
plan before it exists to make that click provably correct rather than to "lay a layer."

Wave 1's two plans are deliberately parallel and non-overlapping: `visibility.go` is a pure,
DB-free function with zero dependency on generated types, and the vote log is a schema + query +
concurrency proof with zero dependency on the resolver. Neither is a "build the schema" / "build
the service" layer task — each ends at a provable, independently-testable guarantee.

The schema → service → HTTP → read-path → browser chain is a real dependency chain in this
codebase; 7 waves (after the 02-03 split) is the honest depth, not under-parallelised planning.

## Cross-plan contracts (pinned here so per-plan calls do not drift)

- **`VoteTally` / `BuildVoteTally` split.** `Visibility`, `ReportMeta`, `VoteTally`,
  `IndependentAgreementThreshold` and `Resolve()` live in `internal/service/visibility.go`
  (02-01) and must stay free of any `sqlcgen` import — that is what lets 02-01 run parallel to
  02-02. `BuildVoteTally()`, `independentCellCount()`, `voterGeohashPrecision = 7`, the vote
  kind/value enums and `VotingService` live in `internal/service/trust.go` (02-03a), which does
  consume `sqlcgen` rows.
- **sqlc regeneration side effects.** 02-02's `files_modified` must include
  `internal/store/sqlc/votes.sql.go`, `internal/store/sqlc/models.go` and
  `internal/store/sqlc/db.go` — `sqlc generate` rewrites all three.
- **Route mounting.** All four vote routes mount in **02-03b** (not 02-03 — split into 02-03a
  service layer / 02-03b HTTP layer during detailed planning) from one
  `handlers.CastVote(svc, kind, value)` factory, inside the existing gated `r.Group` in
  `internal/api/router.go`. 02-07 adds no new *vote* routes.
- **Corrected: 02-07 did NOT need a new store query.** This outline originally assumed the
  Activity page would need a bespoke retracted-reports query because 02-04 excludes Retracted from
  the feed by design (D-12). That reasoning didn't transfer: `ReportsByAccount` (the existing
  "your reports" query from Phase 1.1) never excluded anything by visibility — its own SQL comment
  reads "No expiry predicate: a person's own history does not vanish from their own profile." A
  bespoke query would also have had to encode retraction *in SQL*, which is `ARCHITECTURE.md`
  Anti-Pattern 2 / T-02-04 — the exact control 02-04 locked. 02-07 reused the existing query and
  computed visibility client-side of the DB, through the same `Resolve()` call every other read
  path uses.
- **Swagger drift guard (Phase 1 gate — will fail otherwise).**
  `internal/api/handlers/swagger_test.go` and `docs/docs.go` already exist and guard the OpenAPI
  spec against drift (OPS-01, shipped by plan 01-07). 02-03b adds four routes and 02-04 changes
  the `reports.go` response shape, so **both** plans must add swag doc-comment annotations
  (`@Summary`/`@Param`/`@Success`/`@Failure`/`@Router`, per `02-PATTERNS.md`), re-run `swag init`,
  and name `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` in `files_modified`. Skipping
  this makes an existing Phase 1 test fail for reasons the executor has no context for.
- **Threat IDs.** Reuse `02-VALIDATION.md`'s existing `T-02-01..T-02-05` numbering (mapped in
  order to `02-RESEARCH.md`'s Security Domain rows: Sybil/multi-account → T-02-01, concurrency
  race → T-02-02, unauthorised resolve/reopen → T-02-03, resolver bypass → T-02-04, reporter
  self-confirm → T-02-05). Do not mint fresh per-plan IDs. `security_enforcement: true` at ASVS
  L1 means every plan carries a `<threat_model>` block.
- **Frontend automated gates.** 02-05 / 02-06 / 02-07 must use the existing
  `web/js_contract_test.go`, `web/css_contract_test.go` and `web/template_contract_test.go`
  harnesses for their `<automated>` verify — these are the repo's established gates for
  no-build-step frontend work, and without them those plans would trip the Nyquist rule.
- **Human verification.** `workflow.human_verify_mode: end-of-phase` — `02-VALIDATION.md`'s two
  manual-only items (GPS-denial hard block in a real browser; Provisional/Hidden visual treatment
  and the "Show disputed" toggle) belong in `<verify><human-check>` on 02-05 / 02-06, never as a
  `checkpoint:human-verify` task.
- **Locked from RESEARCH open questions.** A vote on an already-expired report is rejected with an
  explicit 4xx ("This report has expired." — `02-UI-SPEC.md` Copywriting Contract ships the copy),
  assigned to 02-03. See Open Questions below for the one that is *not* closed.

## Open questions — resolved during planning

1. **Reporter-instant reopen (RESEARCH A1 / Open Question 1, `02-UI-SPEC.md` line 237) —
   RESOLVED: no reporter-instant reopen.** D-13 grants the reporter an instant, threshold-free
   *resolve*; D-16 gives reopening no equivalent carve-out. Every plan (01, 03a, 07) implements and
   asserts the literal reading — reopen always requires the independent-agreement threshold, for
   every account including the reporter. Confirmed consistent across `Resolve()` (02-01),
   `CastVote` (02-03a), and the Activity-page UI (02-07).
2. **ROADMAP Goal line is not in user-story form — RESOLVED, left as-is deliberately.** Rather than
   editing ROADMAP.md's Goal line (a capability statement) into `As a … I want to … so that …`
   form, every one of the 8 PLAN.md files was handed the same locked user story verbatim by the
   orchestrator: "As a person near an ongoing incident, I want to confirm or dispute another
   user's report so that reports lacking independent nearby corroboration lose visibility while
   well-corroborated ones stay trusted — and as a reporter or nearby confirmer, I want to mark a
   report resolved once it's no longer true, so the feed reflects what's actually happening right
   now." This satisfies MVP mode's requirement where it's actually consumed (each plan's
   `<objective>`) without rewriting a project-level doc other tooling reads. ROADMAP.md's Goal line
   is intentionally left as the capability statement.
3. **`02-VALIDATION.md` Per-Task Verification Map — RESOLVED.** Backfilled with real plan/task IDs
   once all 8 PLAN.md files existed; see `02-VALIDATION.md` directly.
