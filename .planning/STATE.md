---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 01.1
current_phase_name: Identity & Login — Mandatory Email Verification
status: executing
stopped_at: Phase 1.1 gap closure planned — 01.1-08 (wave 7) created to re-close SC4/IDENT-04
last_updated: "2026-09-12T19:30:00.000Z"
last_activity: 2026-09-12
last_activity_desc: Phase 01.1 gap-closure plan 01.1-08 created
progress:
  total_phases: 7
  completed_phases: 1
  total_plans: 23
  completed_plans: 15
  percent: 14
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-10)

**Core value:** A report showing "confirmed by N nearby" must be verifiably backed by N
independent nearby confirmations, resistant to trivial gaming.
**Current focus:** Phase 01.1 — Identity & Login — Mandatory Email Verification
across 6 waves, ready to execute; inserted before Phase 2, whose own discussion is paused
mid-way, see Pending Todos)

## Current Position

Phase: 01.1 (Identity & Login — Mandatory Email Verification) — EXECUTING (gap closure)
Plan: 8 of 8 (plans 01-07 executed; 01.1-08 planned, not yet executed)
Status: Phase 01.1 verified with one gap — gap-closure plan 01.1-08 created, awaiting execution
Last activity: 2026-09-12 — Phase 01.1 gap-closure plan 01.1-08 created
The D-12 account menu is now a shared `account_header.html.tmpl` partial and moved to wave 5 as
01.1-06; the profile page (D-13 merged Activity section) includes that partial and moved to wave 6
as 01.1-07. The swap is required, not cosmetic: `html/template` errors at execution time on a
`{{template}}` call naming a template a later wave has not written yet.

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**

- Total plans completed: 15
- Average duration: N/A
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 15 | - | - |

**Recent Trend:**

- Last 5 plans: N/A
- Trend: N/A

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Roadmap: Trust mechanic split into two phases (Phase 2 core, Phase 3 hardening) rather than
  merged, because the geohash-precision and confirmer-location-capture decisions Phase 3 resolves
  are load-bearing design choices, not incidental polish (see research SUMMARY.md Phase Ordering
  Rationale).

- Roadmap: Phase 6 (Coordination) depends on Phase 3, not just Phase 2 — triage ordering
  (COORD-04) and field-verified priority (COORD-06) consume the diversity-weighted trust signals
  built in Phase 3.

### Pending Todos

- **Phase 2 discuss-phase session is paused mid-way** (checkpoint file:
  `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-DISCUSS-CHECKPOINT.json`).
  1 of 4 selected gray areas complete (Confirm/Dispute interaction); "Hidden vs Retracted
  triggers" is partway through (3 of ~4 questions answered: dispute-ratio threshold, critical
  exemption, Retracted=resolved-marking — "can Hidden revert back to Live" was interrupted before
  being answered). "Visibility-state visual treatment" and "Resolved-marking flow" areas not yet
  started. Resume with `/gsd-discuss-phase 2` once Phase 1.1 is discussed/planned/executed.

- ~~Phase 1.1 (Identity & Login) needs its own `/gsd-discuss-phase 1.1` session~~ — **done
  2026-09-10**, see `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md`.

- ~~Phase 1.1 needs `/gsd-plan-phase 1.1`~~ — **done 2026-09-11**: 7 plans across 6 waves,
  gsd-plan-checker VERIFICATION PASSED after 4 revision iterations (D-12/D-13 account-menu and
  merged-Activity-section rework, the "full header everywhere" wave-5/6 swap, and the provisional
  wordmark's removal). Ready for `/gsd-execute-phase 1.1`.

### Blockers/Concerns

- **[2026-09-12] Phase 1.1 verification found one gap: SC4/IDENT-04 abuse resistance does not hold**
  (`01.1-VERIFICATION.md`, matching `01.1-REVIEW.md`'s CR-01/WR-01). Both layers are defeatable:
  the per-IP limiter's key comes from a client-supplied proxy header via chi's deprecated
  `middleware.RealIP`, and the per-address cooldown is an unserialised check-then-insert (TOCTOU).
  **Gap-closure plan `01.1-08` (wave 7) is written and validated** — run `/gsd-execute-phase 1.1`
  to execute it. It supersedes two of `01.1-05`'s declared must-haves (the `LatestTokenForEmail`
  key link and the `ORDER BY created_at DESC` artifact string); the supersession table is in
  `01.1-08-PLAN.md` so a re-verify reads it rather than flagging a regression.

- **[2026-09-12] SC2/IDENT-02 (real Resend delivery) needs one human check, not a code fix** — no
  test ever makes a live call to Resend by explicit design, so delivery has never been exercised.
  Tracked in `01.1-VERIFICATION.md`'s `human_verification` section; not something a plan can close.

- **[2026-09-10] Resend custom-domain DNS verification is a pre-launch requirement**: Resend's
  sandbox sender (`onboarding@resend.dev`) only delivers to the account owner's own signup email
  until a custom domain is DNS-verified — until then, no real visitor can receive a verification
  link, and since login is mandatory to view anything (D-05), that means an unusable app for
  everyone but the developer. Plan 01.1-03 documents this in README.md as a required pre-launch
  checklist item (add + DNS-verify a domain, then set `RESEND_FROM`), per the user's explicit
  decision to plan for it now rather than defer it. Same category of blocker as the DLT
  registration item below — a real account/DNS-configuration step, not something code can work
  around.

- **[2026-09-10] Access model reversed mid-Phase-2-discussion**: anonymous no-signup posting
  (Phase 1's FOUND-01, shipped and verified) is superseded by mandatory email verification via
  magic link, inserted as new Phase 1.1 before Phase 2. Phone OTP was explicitly rejected — no
  free tier at any real SMS volume, and the cheap route needs India DLT sender registration
  (business paperwork, not just an API key). See PROJECT.md Key Decisions for the full tradeoff
  record.

- **Confirmer location-capture method unresolved** (GPS prompt vs. IP-derived coarse geohash) —
  materially changes what "diversity-weighted" measures; PROJECT.md flags this as an explicit
  open decision to resolve during Phase 2/3 planning, not deferred further.

- **Geohash cell-size precision has no benchmarked value** — needs a documented, reasoned choice
  against a realistic incident radius (~100-300m for a flooded road segment) during Phase 3
  planning.

- **India IT Rules 2021 intermediary-liability applicability is LOW confidence** — get an actual
  legal/mentor review before any wide public promotion; not required before initial deploy
  (Phase 4 ships the disclaimer, not a legal clearance).

- **IMD/CWC bulletin integration is unverified** — GDACS (Phase 5) is the safer first official-feed
  integration; treat IMD/CWC as a follow-up spike, not assumed available in Phase 5.

- **REQUIREMENTS.md coverage count corrected**: the file's own Coverage block stated "27 total"
  v1 requirements, but 33 unique requirement IDs are actually enumerated (FOUND 6, TRUST 9,
  COORD 8, ROBUST 8, OPS 2). All 33 are mapped across the 6 phases above; the stale "27" count
  has been corrected in REQUIREMENTS.md.

- **[Phase 1] Non-blocking security warnings from `01-SECURITY.md`**: three plans (01-03/01-04/
  01-06) claimed a "grep gate on markup-parsing sinks" that was never actually committed as a
  test — the live no-`innerHTML` property currently holds (verified by direct inspection), but
  nothing guards the regression, so a future `el.innerHTML = report.description` would ship
  stored XSS with a green build. `TestVendorMapScriptsLoadInDependencyOrder` also only asserts the
  SRI `integrity="sha256-` prefix, not the actual hash value. Worth a small follow-up plan; not
  urgent since threats_open: 0 at the current ASVS L1 gate.

- **[Phase 1] CI Go-version pin is looser than go.mod**: `.github/workflows/ci.yml` pins
  `go-version: '1.25'` while `go.mod` requires `go 1.26.4`. `GOTOOLCHAIN=auto` downloads the right
  toolchain so CI still runs correctly today, but there's no explicit `toolchain` directive or
  `GOTOOLCHAIN` override — worth tightening so this isn't silently relying on default behavior.

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-09-11T13:30:00.000Z
Stopped at: Phase 1.1 fully planned and verified — 7 plans across 6 waves, gsd-plan-checker
VERIFICATION PASSED. Reached via 4 revision iterations continuing a prior session's half-finished
UI amendment: 01.1-06 was stale against D-12 (account menu)/D-13 (merged Activity section), so it
was split into a header plan and a profile plan; the user then chose "full header everywhere" over
a minimal back-link, which required swapping wave order (01.1-06 is now the shared
`account_header.html.tmpl` partial, wave 5; 01.1-07 is now the profile page that includes it, wave
6) and building/removing a provisional home-link wordmark (user declined it — `/profile` has no
route back to the map beyond browser-back, an accepted, documented trade-off). Ready for
`/gsd-execute-phase 1.1`. Phase 2's own discussion remains checkpointed and paused (see Pending
Todos) — resume it after Phase 1.1 ships.
Resume file: .planning/phases/01.1-identity-login-mandatory-email-verification/01.1-07-PLAN.md
