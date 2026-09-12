# Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-10 (started) — 2026-09-12 (resumed and completed)
**Phase:** 02-trust-mechanic-core-confirm-dispute-visibility
**Areas discussed:** Confirm/Dispute interaction, Hidden vs Retracted triggers, Visibility-state
visual treatment, Resolved-marking flow, Confirmer location-capture method

---

## Confirm/Dispute interaction

| Option | Description | Selected |
|--------|-------------|----------|
| Both feed row & map popup | Controls appear on both surfaces | ✓ |
| Feed row only | — | |
| Map pin popup only | — | |

**User's choice:** Both feed row & map popup

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, vote can be changed | — | ✓ |
| No, vote is permanent once cast | — | |

**User's choice:** Yes, vote can be changed

| Option | Description | Selected |
|--------|-------------|----------|
| Button hidden/disabled for the reporter | — | ✓ |
| Button visible but vote silently doesn't count | — | |

**User's choice:** Button hidden/disabled for the reporter

| Option | Description | Selected |
|--------|-------------|----------|
| Optimistic instant update | — | |
| Wait for server confirmation | — | ✓ |

**User's choice:** Wait for server confirmation (against the recommended optimistic-update option)

---

## Hidden vs Retracted triggers

| Option | Description | Selected |
|--------|-------------|----------|
| Disputes outnumber confirms past a threshold | — | ✓ |
| A single independent dispute is enough | — | |
| Dispute count relative to confirms, plus a minimum dispute floor | — | |

**User's choice:** Disputes outnumber confirms past a threshold

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, critical/rescue-needed can never be Hidden by disputes | — | ✓ |
| No, disputes can hide even critical reports | — | |

**User's choice:** Yes, critical/rescue-needed exempt

| Option | Description | Selected |
|--------|-------------|----------|
| Retracted = reporter/confirmer marked it resolved | — | ✓ |
| Retracted = expired naturally (folded into existing expiry) | — | |

**User's choice:** Retracted = reporter/confirmer marked it resolved

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, fully reversible | Visibility recomputed live from current vote tallies every read | ✓ |
| Reversible with hysteresis | Requires clearing threshold by a margin | |
| No, one-way — once Hidden, stays Hidden | Permanent terminal state, needs a sticky flag | |

**User's choice:** Yes, fully reversible
**Notes:** Chosen for consistency with the locked `VisibilityResolver` architecture (a pure,
stateless function called by every read path) — no special-case "unhide" logic needed.

---

## Visibility-state visual treatment

| Option | Description | Selected |
|--------|-------------|----------|
| Dimmed treatment | Reuses Phase 1's expiry-fade desaturation pattern | |
| Badge/pill label only | Text tag, full color/opacity otherwise | |
| Both dimmed + labeled | Combines both | ✓ |

**User's choice:** Both dimmed + labeled

| Option | Description | Selected |
|--------|-------------|----------|
| Removed entirely | Hidden reports don't come back from any query | |
| Shown grayed-out with a note | Stays visible, visually collapsed | |
| Shown only in a separate 'disputed' view | Excluded from main feed, browsable in a dedicated filter | ✓ |

**User's choice:** Shown only in a separate 'disputed' view
**Notes:** Follow-up asked how this fits within phase scope (no brand-new page):

| Option | Description | Selected |
|--------|-------------|----------|
| Filter toggle on the existing feed | Same page, same resolver-backed query, opt-in filter value | ✓ |
| Section within the reporter's own Activity page | Only the reporter sees their own Hidden report | |

**User's choice:** Filter toggle on the existing feed

| Option | Description | Selected |
|--------|-------------|----------|
| Map pin too, filter-gated | Same filter also reveals Hidden pins on the map | ✓ |
| Feed list only, not on the map | Map never shows Hidden reports | |

**User's choice:** Map pin too, filter-gated

| Option | Description | Selected |
|--------|-------------|----------|
| Removed from live feed/map immediately | Disappears right away, stays in reporter's Activity history | ✓ |
| Brief grace-period badge before disappearing | Stays visible briefly with a 'Resolved' badge | |

**User's choice:** Removed from live feed/map immediately

---

## Resolved-marking flow

| Option | Description | Selected |
|--------|-------------|----------|
| Reporter only, always instant | Simplest, zero new abuse surface | |
| Reporter instant; any confirmer needs N marks | Mirrors the confirm/dispute independence predicate | ✓ |
| Any nearby confirmer, single mark, instant | Matches TRUST-08 literally, but single-actor gameable | |

**User's choice:** Reporter instant; any confirmer needs N marks

| Option | Description | Selected |
|--------|-------------|----------|
| Same threshold as Hidden's independence predicate | One rule, reused everywhere | ✓ |
| A different (higher) fixed number for resolving | Resolving is more consequential | |

**User's choice:** Same threshold as Hidden's independence predicate

| Option | Description | Selected |
|--------|-------------|----------|
| Report detail/expanded view only | Natural speed bump against accidental resolving | |
| Same placement as confirm/dispute (feed row & map popup) | Fastest to use | ✓ |

**User's choice:** Same placement as confirm/dispute (feed row & map popup)

| Option | Description | Selected |
|--------|-------------|----------|
| Final — not reopenable; post a new report instead | Terminal state, same simplicity as expiry | |
| Reopenable the same way Hidden is reversible | Symmetric with Hidden-reversible decision | ✓ |

**User's choice:** Reopenable the same way Hidden is reversible
**Notes:** Follow-up asked what mechanism actually reopens a Retracted report, since a
resolved-mark isn't a confirm/dispute vote:

| Option | Description | Selected |
|--------|-------------|----------|
| A dedicated 'Reopen / not actually resolved' action | Separate from confirm/dispute on report content | ✓ |
| Posting a new confirm/dispute vote on it reopens it | Reuses the existing vote mechanism | |

**User's choice:** A dedicated 'Reopen / not actually resolved' action

---

## Confirmer location-capture method

> Raised as an additional gray area beyond the original 4 — flagged in PROJECT.md as an explicit
> open decision needed for TRUST-03 (this phase's requirement), not a new capability.

| Option | Description | Selected |
|--------|-------------|----------|
| GPS prompt, cached per session | Accurate geohash, matches Phase 1's GPS-based report submission | ✓ |
| IP-derived coarse geohash | Zero-friction, but CGNAT risk collapses distinct voters into one cell | |
| Self-declared — no location capture at all | Weakest signal, trivially gameable | |

**User's choice:** GPS prompt, cached per session

| Option | Description | Selected |
|--------|-------------|----------|
| Vote blocked until location is granted | Every counted vote is meaningfully location-backed | ✓ |
| Vote still recorded, but excluded from the independence count | Lets people vote without GPS, at reduced effect | |

**User's choice:** Vote blocked until location is granted

---

## Claude's Discretion

- Exact numeric threshold for "disputes outnumber confirms past a threshold" and the shared
  independent-agreement count reused by the Hidden trigger, confirmer-driven resolving, and
  reopening.
- Exact UI copy/wording for "Unconfirmed"/"Show disputed"/"Reopen" labels.
- Exact mechanism for GPS-location caching (session storage vs. cookie vs. in-memory per page
  load).
- Exact vote-log schema shape (update-in-place per (report, account) vs. append-only with
  latest-row-wins) — architecture research already specifies append-only as the pattern; precise
  schema is planning's call.

## Deferred Ideas

None raised during this session. (Diversity-weighted "confirmed by N" display and the
confidence/reliability score split remain correctly scoped to Phase 3, not re-litigated here.)
