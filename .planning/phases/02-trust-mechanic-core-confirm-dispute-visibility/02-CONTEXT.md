# Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility - Context

**Gathered:** 2026-09-12 (session started 2026-09-10, paused mid-way for Phase 1.1's insertion, resumed and completed 2026-09-12)
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase delivers the core trust mechanic: a verified user can confirm or dispute another
user's report, and the resulting visibility state (Hidden/Provisional/Live/Retracted) is computed
by one shared, concurrency-safe resolver everywhere it's shown — feed, map, triage view, and
shareable card. Covers TRUST-01, TRUST-02, TRUST-03, TRUST-04, TRUST-06, TRUST-08, TRUST-09.

Diversity-weighted display counts ("confirmed by N nearby" as a dampened, cell-weighted number)
and the confidence/reliability score split are **Phase 3** scope, not this phase — Phase 2 only
needs the independence predicate (distinct session + distinct geohash cell) as a boolean gate for
the provisional→live transition, not the full weighting curve.

Per Phase 1.1 (now executed), "distinct session" throughout this phase's independence predicate
means "distinct verified account" — anonymous voting no longer exists.

</domain>

<decisions>
## Implementation Decisions

### Confirm/Dispute interaction
- **D-01:** Confirm/dispute controls appear in **both the feed row and the map pin popup** — not
  restricted to one surface.
- **D-02:** A user **can change their vote** after casting it (not a one-shot permanent choice).
- **D-03:** The reporter **cannot vote on their own report** — the confirm/dispute button is
  hidden/disabled for the reporter, not silently discarded.
- **D-04:** After casting a vote, the UI **waits for server confirmation** before updating —
  explicitly not an optimistic instant-update pattern.

### Hidden vs Retracted triggers
- **D-05:** A report goes **Hidden** when disputes outnumber confirms past a threshold (not a
  single dispute, not a bare majority with no floor — exact threshold value is planning/research's
  call, informed by Pitfall research).
- **D-06:** **Critical/rescue-needed reports can never be Hidden by disputes** — same exemption
  pattern as the Provisional gate (TRUST-04), for the same reason (never suppress a
  possibly-real emergency behind a soft trust signal).
- **D-07:** **Retracted** is a distinct trigger from Hidden — it fires only when the reporter or an
  eligible confirmer explicitly marks the report resolved (see Resolved-marking flow below), not
  from dispute volume and not folded into ordinary expiry.
- **D-08:** **Hidden is fully reversible** — visibility is recomputed live from current vote
  tallies on every read (consistent with the locked `VisibilityResolver` architecture: one
  stateless function, called by every read path). If confirms later outweigh disputes again, the
  report returns to Live/Provisional automatically, no special-case "unhide" logic needed.

### Visibility-state visual treatment
- **D-09:** **Provisional** reports get **both** a dimmed/desaturated treatment (reusing Phase 1's
  D-17 expiry-fade desaturation pattern, so "not yet trusted" reads consistently with "going
  stale") **and** an explicit text label (e.g., "Unconfirmed") — not one or the other alone.
- **D-10:** **Hidden** reports are **removed from the default feed and map** for all viewers, but
  browsable via a **"Show disputed" filter toggle on the existing feed** (same page, same
  resolver-backed query, no new route/page) — not a fully separate "disputed" page, and not
  restricted to only the reporter's own view.
- **D-11:** The same "Show disputed" filter **also reveals Hidden pins on the map** (styled
  distinctly, e.g. grayed/outline), not feed-list-only — keeps feed and map showing identical
  resolver state per TRUST-02.
- **D-12:** **Retracted** reports are **removed from the live feed/map immediately** on
  resolution — no grace-period/strikethrough lingering state. (They remain visible in the
  reporter/confirmer's own Activity history per Phase 1.1's merged Activity section.)

### Resolved-marking flow
- **D-13:** The **reporter can mark their own report resolved instantly**, no threshold. **Any
  nearby confirmer can also propose "resolved,"** but it only takes effect once a small number of
  **independent** confirmers agree — mirrors the confirm/dispute independence predicate rather
  than trusting a single non-reporter vote to silently close someone else's report.
- **D-14:** The threshold for confirmer-driven resolving reuses **the same independence
  predicate and threshold already used for the Hidden trigger** — one independence rule, applied
  consistently everywhere the resolver needs "independent agreement," not a second bespoke number
  to keep in sync.
- **D-15:** "Mark Resolved" is placed at **the same surface as Confirm/Dispute** (feed row & map
  popup) — not tucked into a report-detail-only view, despite being a higher-consequence action
  than a vote.
- **D-16 (amended 2026-09-15 — see below):** A Retracted report **can be reopened** — not a
  one-way terminal state. Reopening uses **a dedicated "Reopen / not actually resolved" action**,
  distinct from ordinary confirm/dispute votes on the report's underlying content — keeps "is this
  report still true" and "is this resolved" as two separately-tracked signals rather than
  overloading one vote type. **The reporter can reopen their own report instantly, no threshold —
  symmetric with D-13's instant resolve.** A non-reporter confirmer's reopen still requires the
  same independent-agreement threshold as D-14 to flip the report back to Live/Provisional; the
  reporter's own reopen action always succeeds immediately, regardless of how the report came to
  be Retracted (their own instant resolve, or independent confirmers' agreement).

  **Amendment history:** the original discuss-phase session (2026-09-12) locked reopening as
  requiring the independent-agreement threshold for every account including the reporter, with no
  carve-out. `02-RESEARCH.md`'s Open Question 1 flagged this specific asymmetry (instant resolve
  but no instant reopen) as needing an explicit user confirmation before plan-checker sign-off
  rather than a silent default, since it materially changes reporter power over their own report.
  All 8 Phase 2 plans were built against the literal no-carve-out reading; `gsd-plan-checker`'s
  2026-09-15 verification pass caught the missing confirmation and surfaced the question directly.
  The user's answer, given then, reversed the original reading to the symmetric-with-D-13 form
  above. `02-01`, `02-03a`, and `02-07` were re-planned accordingly the same day.

### Confirmer location-capture method (resolves the open item flagged in PROJECT.md)
- **D-17:** The voter's own location for the independence predicate's "distinct geohash cell"
  check is captured via a **GPS permission prompt, asked once per session/device and cached** —
  not re-prompted on every single vote, and explicitly **not** IP-derived geohashing (this
  project's own CGNAT finding — many real Indian phones sharing one carrier IP — would let
  genuinely independent nearby voters collapse into the same "cell," undermining exactly the
  independence check the Core Value depends on) and **not** a self-declared/unverified "I'm near
  this" claim (trivially gameable, directly against the "resistant to trivial gaming" Core Value
  requirement).
- **D-18:** If a voter **denies the GPS permission prompt, their vote is blocked** (not silently
  accepted-but-uncounted) — every vote that counts at all is meaningfully location-backed; this
  keeps the independence predicate load-bearing rather than an optional nicety with silent
  exceptions.

### Claude's Discretion
- Exact numeric threshold for "disputes outnumber confirms past a threshold" (D-05) and for the
  independent-agreement count used identically by D-06/D-14/D-16 — informed by
  `.planning/research/PITFALLS.md`'s concurrency/race findings and the Phase 3 geohash-precision
  work, but the specific number is an implementation/research call, not re-litigated here.
- Exact UI copy/wording for the "Unconfirmed"/"Show disputed"/"Reopen" labels — the *behavior* is
  locked above, the *copy* is Claude's call within Phase 1's established neutral-utility visual
  direction.
- Exact mechanism for GPS-location caching (session storage vs. a short-lived cookie vs. in-memory
  per page load) — D-17 locks the *behavior* (prompt once per session, not every vote), not the
  storage mechanism.
- Whether the append-only vote log stores one row per (report, account) with an update-in-place
  for "change your vote" (D-02) or an append-only log where the latest row per (report, account)
  wins — architecture research (`ARCHITECTURE.md`) already specifies append-only as the pattern;
  exact schema is planning's call.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project-level context and locked decisions
- `.planning/PROJECT.md` — Core Value, Active requirements ("Base reporting loop" and
  "Trust-model hardening" sections), Key Decisions table (`VisibilityResolver` architecture,
  independence-predicate-before-diversity-weighting sequencing, mandatory-login access model)
- `.planning/REQUIREMENTS.md` — TRUST-01 through TRUST-09 (this phase covers all except TRUST-05
  and TRUST-07, which are Phase 3)
- `.planning/ROADMAP.md` §"Phase 2: Trust Mechanic Core" — Goal and the 5 Success Criteria this
  phase's plans must satisfy

### Architecture and pitfalls (from ecosystem research) — load-bearing for this phase specifically
- `.planning/research/ARCHITECTURE.md` — **the `VisibilityResolver` pattern** (`Resolve(report,
  votes, moderationState, now) → {Hidden, Provisional, Live, Retracted}`, a pure function called
  by every read path, no exceptions); the two-layer independence-predicate/diversity-weighting
  split (this phase builds Layer A only); Anti-Pattern 2 ("bypassing VisibilityResolver from any
  read path") and Anti-Pattern 3 ("building the diversity-weighting curve before the independence
  predicate") both apply directly — this phase must not repeat either
- `.planning/research/PITFALLS.md` **Pitfall 2** ("Race condition in the confirm/dispute tally —
  the Core Value silently breaks under concurrency") — directly maps to TRUST-09's required
  concurrency test; the confirm/dispute tally must be updated atomically in the same transaction
  as the vote insert, never read-then-write
- `.planning/research/PITFALLS.md` §405-406 — geohash precision must be a deliberate, documented
  choice against a realistic incident radius; full precision selection is Phase 3 scope, but this
  phase's vote-write path must capture `geohash_cell` from day one per Layer A (not backfillable
  later)

### Prior-phase context this phase depends on
- `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md` — why and
  how anonymous voting was replaced with verified-account sessions; "distinct session" in this
  phase's independence predicate now means "distinct verified account"
- `.planning/phases/01-foundation-report-map/01-CONTEXT.md` — D-17 (expiry-fade desaturation
  pattern reused by D-09 above), D-14/D-15 (severity-driven expiry tiers this phase's Retracted
  state interacts with), the theme reference image (see `<specifics>` below)

### Existing schema and code to extend (not replace)
- `internal/store/migrations/00001_create_reports.sql` — `reports` table (id, session_id,
  category, severity, geohash, expires_at, etc.) and `sessions` table this phase's votes/
  visibility state build on top of
- `internal/store/migrations/00002_add_accounts_and_verification.sql` — `accounts` table and
  `sessions.account_id` link; votes should key off the verified account per Phase 1.1/D-05 above
- `internal/service/report.go` — existing report-service code with forward-looking comments
  already flagging "arrive once confirm/dispute exists in Phase 2/3" (line ~29) and
  "trust-weighted tuning arrive once confirm/dispute exists" (line ~230)
- `internal/api/handlers/reports.go` — existing comment already noting severity affects triage
  ordering only and never bypasses the provisional-visibility gate (line ~43) — this phase
  implements the gate that comment refers to

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/session/cookie.go`'s `Manager` — signed-cookie session mechanism, now gated behind
  verified accounts (Phase 1.1); votes key off this session/account, not a new identity mechanism.
- Phase 1's severity-slider UI, traffic-light color system (green/amber/red), and category-badge
  glyph treatment — the Provisional dimmed+labeled treatment (D-09) and Hidden filter-toggle
  styling (D-10/D-11) should extend this existing visual system, not invent a parallel one.
- Phase 1's expiry-fade desaturation CSS pattern (D-17 in `01-CONTEXT.md`) — directly reused by
  this phase's D-09 (Provisional dimming).

### Established Patterns
- Server-computed, client-untrusted fields (Phase 1.1's pattern: server decides verification
  state, client can never claim it) — extends naturally to vote/visibility state: a client can
  never claim "this report is Live," only cast a vote; the server-side `VisibilityResolver` alone
  decides the resulting state.
- Append-only log + atomic cache update pattern, already named as a requirement in PROJECT.md's
  "Base reporting loop" section — this phase's `confirmations` table is the first application of
  it.

### Integration Points
- `internal/api/router.go` — existing route/middleware structure where new
  `POST /api/reports/{id}/confirm`, `.../dispute`, `.../resolve`, `.../reopen` endpoints (exact
  route shape is planning's call) will mount, behind the existing verified-session middleware.
- `internal/service/report.go` — the two comments cited above mark where this phase's logic slots
  into the existing service layer rather than requiring a new parallel structure.

</code_context>

<specifics>
## Specific Ideas

- **Theme reference image** (`.planning/phases/01-foundation-report-map/assets/01-theme-reference.png`)
  already mockups "Confirmed by N nearby" and "Unconfirmed · expires soon" states — useful visual
  direction for D-09's Provisional treatment, though the literal "confirmed by N nearby" *number*
  itself is Phase 3 (diversity-weighted count), not this phase. This phase's Provisional/Hidden
  states can borrow the visual language without needing the final weighted count wired up yet.
- **One independence rule everywhere — with one now-explicit reporter carve-out.** The discussion
  repeatedly converged on reusing the *same* distinct-account-AND-distinct-geohash-cell predicate
  and the *same* small threshold across three separate mechanics — the Hidden trigger (D-05/D-06),
  confirmer-driven resolving (D-13/D-14), and non-reporter reopening (D-16). Treat "independent
  agreement" as one reusable concept for all three, not three separate implementations. **The one
  asymmetry, confirmed 2026-09-15:** the reporter gets an instant, threshold-free path on both
  halves of the resolve/reopen pair (D-13 resolve, amended D-16 reopen) — mirroring each other —
  while the Hidden trigger (D-05/D-06) has no equivalent reporter carve-out at all (a reporter
  cannot single-handedly un-hide their own disputed report; only new independent confirms can).
- **GPS-based location capture (D-17) was chosen specifically because of this project's own CGNAT
  finding** — the same shared-carrier-IP risk that ruled out pure IP-based *rate limiting*
  elsewhere in the project (see PROJECT.md Constraints) was raised again here as the reason to
  reject IP-derived geohashing for the independence predicate specifically. This is the same
  underlying real-world constraint (India carrier NAT) showing up in a second, different design
  decision — worth keeping consistent if it comes up again in Phase 3 or Phase 4.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within Phase 2 scope. (The diversity-weighted "confirmed by N" display
count and the confidence/reliability score split are already correctly scoped to Phase 3 per
ROADMAP.md, not raised as new scope here; the "notify a reporter when someone
confirms/disputes their report" idea was already deferred during Phase 1.1's discussion, pending
push infrastructure, and wasn't re-raised.)

</deferred>

---

*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Context gathered: 2026-09-12*
