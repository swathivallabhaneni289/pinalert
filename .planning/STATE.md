---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
current_phase: 3
status: not_started
stopped_at: "Sketch 001 closed (d4 wins, 124d34c). Next: sketch wrap-up, report page and comments phase, then /gsd-ui-phase 3"
last_updated: "2026-10-03T06:48:05.560Z"
progress:
  total_phases: 8
  completed_phases: 4
  total_plans: 42
  completed_plans: 42
  percent: 50
current_phase_name: trust-model-hardening-diversity-weighted-trust
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-09-10)

**Core value:** A report showing "confirmed by N nearby" must be verifiably backed by N
independent nearby confirmations, resistant to trivial gaming.
**Current focus:** Phase 3 (Trust-Model Hardening: Diversity-Weighted Trust). The popup design is
chosen (sketch 001, variant d4), so the next step is the UI contract (`/gsd-ui-phase 3`), then
`/gsd-plan-phase 3`. Open items from earlier phases that do not block planning: Phase 2's live retests
(`02-UAT.md`, wanted before Phase 3 is executed) and Phase 1.1's live Resend delivery check
(`01.1-UAT.md`, SC2/IDENT-02). See Blockers/Concerns below.

## Current Position

Phase: 3 — Trust-Model Hardening: Diversity-Weighted Trust (not started)
Status: Context, research, validation draft, design brief and the popup sketch are done. Ready for the UI
contract (`/gsd-ui-phase 3`); `/gsd-plan-phase 3` is gated on it. Phase 07 (address search box)
completed 2026-09-29: 4/4 plans across 3
waves, full regression suite green, 1 code-review Critical (CR-01 staleness guard) found and
fixed, 6/6 UAT tests passed (including a live layout bug found and fixed mid-session — see
`07-UAT.md`), security review closed with threats_open: 0 (`07-SECURITY.md`).

Progress: [██████████████░░░░░░] 4/8 phases complete (1, 1.1, 2, 07)

## Performance Metrics

**Velocity:**

- Total plans completed: 19
- Average duration: N/A
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 | 15 | - | - |
| 07 | 4 | - | - |

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

- **[2026-09-16] Real 3D WebGL globe on login screen** — user wants the Phase 1.1 login page's
  flat CSS-masked rotating SVG globe replaced with a genuine 3D render, raised live during Phase 2
  UAT. Not scoped/planned — needs a design pass (new WebGL dependency vs. this project's
  no-build-step stack). See `.planning/todos/pending/2026-09-16-real-3d-webgl-globe-on-login-screen.md`.

- ~~Phase 2 discuss-phase session is paused mid-way~~ — **done 2026-09-12**: resumed and
  completed all 4 originally-selected areas (Confirm/Dispute interaction, Hidden vs Retracted
  triggers, Visibility-state visual treatment, Resolved-marking flow) plus a 5th area
  (Confirmer location-capture method, resolving the open item PROJECT.md flagged for TRUST-03).
  See `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-CONTEXT.md`.

- ~~Phase 2 needs `/gsd-ui-phase 2`~~ — **done 2026-09-15**: `02-UI-SPEC.md` approved, 6/6
  dimensions passed, 0 recommendations.

- ~~Phase 2 needs `/gsd-plan-phase 2`~~ — **done 2026-09-15**: research complete
  (`02-RESEARCH.md`), pattern map complete (`02-PATTERNS.md`), 8 plans across 7 waves committed
  (`02-01`, `02-02`, `02-03a`, `02-03b`, `02-04`, `02-05`, `02-06`, `02-07`). The initial
  single-shot planner call stalled after 600s with nothing written; recovered by switching to
  chunked mode (one plan per agent call, committed individually) — see `02-PLAN-OUTLINE.md` for
  the full plan breakdown and cross-plan contracts. **Verification: done 2026-09-15** — 7 parallel
  gsd-plan-checker passes (one per wave) over all 8 plans, all passed (1 non-blocking warning
  found and fixed same session). Next: `/gsd-execute-phase 2`.

- ~~Phase 1.1 (Identity & Login) needs its own `/gsd-discuss-phase 1.1` session~~ — **done
  2026-09-10**, see `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md`.

- ~~Phase 1.1 needs `/gsd-plan-phase 1.1`~~ — **done 2026-09-11**: 7 plans across 6 waves,
  gsd-plan-checker VERIFICATION PASSED after 4 revision iterations (D-12/D-13 account-menu and
  merged-Activity-section rework, the "full header everywhere" wave-5/6 swap, and the provisional
  wordmark's removal). Ready for `/gsd-execute-phase 1.1`.

### Blockers/Concerns

- **[2026-09-16] Phase 2 code review WARNING: vote routes have no rate/velocity limit** —
  `02-REVIEW.md` (WR-01): `POST /api/reports/{id}/confirm|dispute|resolve|reopen` carry no
  rate-limiting of any kind, unlike `/api/auth/request-link`. The `votes` table is append-only by
  design (no unique key), and the independence predicate depends on a client-supplied,
  server-unverifiable GPS coordinate — so a single verified account can currently cast unlimited
  votes with no throttle. This bears directly on the Core Value's "resistant to trivial gaming"
  framing. Not a Phase 2 must-have (rate limiting is ROBUST-04, Phase 4's scope) and the phase's
  verifier confirmed it's correctly out of scope here — but worth prioritizing early in Phase 4,
  or as a standalone hardening plan before Phase 4 if it's used in a public demo sooner.

- **[2026-09-16] Verifier escalation: ROADMAP.md's Phase 2 `Mode: mvp` flag vs. non-user-story
  Goal wording** — `02-VERIFICATION.md`'s `mode_guard` frontmatter: the phase is tagged `Mode: mvp`
  but its Goal field is a capability statement, not `As a <role>, I want to <capability>, so that
  <outcome>.` form. The verifier's own guard refused to force the MVP User Flow Coverage table and
  instead verified directly against ROADMAP's 5 explicit numbered Success Criteria (which worked
  fine — 5/5 confirmed). A human should decide: rewrite the Goal line as a user story, or clear the
  `Mode: mvp` flag for this phase, so future verification/planning runs on Phase 2 (e.g. gap
  closure) don't hit the same guard. Non-blocking for now since the direct-Success-Criteria path
  fully substituted.

- ~~**[2026-09-12] Phase 1.1 verification found one gap: SC4/IDENT-04 abuse resistance does not
  hold**~~ — **closed 2026-09-12**: gap-closure plan `01.1-08` (wave 7) replaced chi's deprecated,
  spoofable `middleware.RealIP` with `middleware.ClientIPFromRemoteAddr`, and replaced the
  per-email cooldown's unserialised check-then-insert with a single atomic
  `INSERT ... ON CONFLICT (email) DO UPDATE ... RETURNING` claim (new `email_cooldowns` table).
  Independently re-verified against the live code by a fresh `gsd-verifier` pass (not just trusted
  from the plan's own SUMMARY) — `middleware.RealIP` and `LatestTokenForEmail` are both confirmed
  absent from the codebase, and all 7 adversarial tests (3 forged-header variants, 4 concurrency
  tests) pass. It supersedes two of `01.1-05`'s declared must-haves (the `LatestTokenForEmail` key
  link and the `ORDER BY created_at DESC` artifact string) — recorded in `01.1-08-PLAN.md`'s
  supersession table, not a silent regression.

- **[2026-09-12] SC2/IDENT-02 (real Resend delivery) needs one human check, not a code fix** — no
  test ever makes a live call to Resend by explicit design, so delivery has never been exercised.
  This is the **only remaining item before Phase 1.1 fully closes** — tracked in `01.1-UAT.md`;
  run `/gsd-verify-work 1.1` with a real `RESEND_API_KEY` and a DNS-verified `RESEND_FROM` domain
  to confirm and close it out.

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

- ~~Confirmer location-capture method unresolved~~ — **resolved 2026-09-12** during Phase 2's
  discuss-phase session: GPS prompt, asked once per session/device and cached (not IP-derived —
  this project's own CGNAT finding would collapse genuinely-independent nearby voters into one
  cell); a denied prompt blocks the vote rather than accepting it uncounted. See `02-CONTEXT.md`
  D-17/D-18.

- **Geohash cell-size precision has no benchmarked value** — still open for Phase 3's
  diversity-weighting display count (TRUST-05). Note: Phase 2's research (`02-RESEARCH.md`)
  separately picked `voterGeohashPrecision = 7` (~153m cells) for the independence-predicate gate
  only — deliberately distinct from the report's own existing precision-8 geohash column and from
  whatever precision Phase 3 benchmarks for the display curve.

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

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260921-mf2 | Activity page: replace Back to map text link with a back-arrow icon control, perfect placement | 2026-09-21 | 9719806 | [260921-mf2-activity-page-replace-back-to-map-text-l](./quick/260921-mf2-activity-page-replace-back-to-map-text-l/) |
| 260923-mb0 | Give the map popup the same severity-tinted background the feed row already has | 2026-09-23 | 5ebeda4 | [260923-mb0-give-the-map-popup-the-same-severity-tin](./quick/260923-mb0-give-the-map-popup-the-same-severity-tin/) |
| 260923-qwi | Replace the wordy Theme text label with a two-state sun/moon icon toggle, dropping System mode | 2026-09-23 | 3ace168 | [260923-qwi-replace-the-wordy-theme-text-label-with-](./quick/260923-qwi-replace-the-wordy-theme-text-label-with-/) |
| 260923-rra | Move the theme toggle out of the account menu into a floating button below the report FAB (which moved up to make room) | 2026-09-29 | 3330dda | [260923-rra-move-the-theme-toggle-out-of-the-account](./quick/260923-rra-move-the-theme-toggle-out-of-the-account/) |

## Deferred Items

Items acknowledged and carried forward from previous milestone close:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| *(none)* | | | |

## Session Continuity

Last session: 2026-10-03T06:48:05.549Z
Stopped at: Sketch 001 closed (d4 wins, 124d34c). Next: sketch wrap-up, report page and comments phase, then /gsd-ui-phase 3
Phase 3 so far: context (D-01 to D-23, R-01 to R-08), research, a draft validation plan (V-01 to V-42) and
the design brief are committed. The popup was then sketched (sketches 001 and 002 in `.planning/sketches/`,
four independent reviews) and the owner chose variant d4, built from their own reference image. Sketch 001 is
closed and committed (`124d34c`). Its README lists where d4 departs from locked decisions D-18, D-19, D-20 and
D-22 and from validation rows V-39 and V-40 (a pill severity label, grey signal words, one disclosure button
instead of per word reasons, a 340px neutral card, an always visible too far sentence, a comments footer with
no phase behind it). The UI contract has to record each of those as an explicit amendment. Sketch 002 (report
page and thread) stays open until a separate report page and comments phase exists.

Order of work from here: package the sketch findings (`/gsd-sketch --wrap-up`), add the report page and
comments phase (`/gsd-phase`), write the UI contract (`/gsd-ui-phase 3`), then plan (`/gsd-plan-phase 3`).
`/gsd-plan-phase 3` exits at the UI gate until a UI-SPEC exists, and `/gsd-ui-phase 3` only reads sketches that
the wrap-up has packaged into a findings skill.

Open items that do not block planning: Phase 2's live retests (`02-UAT.md` is still
`fixes_shipped_pending_retest`; wanted before Phase 3 is executed, because Phase 3 rewrites the popup that the
Mark resolved fix never covered) and Phase 1.1's live Resend delivery check (`01.1-UAT.md`). The `Mode: mvp`
flag on Phase 3 has a Goal that is not written as a user story, the same guard that hit Phase 2, so decide
between rewriting the goal and clearing the flag before planning. Build, review and verification history for
Phases 1.1 and 2 lives in their phase directories and in git history.

Resume file: .planning/sketches/001-map-pin-popup/README.md
