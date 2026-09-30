# Phase 3: Trust-Model Hardening, Diversity-Weighted Trust - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md; this log preserves the alternatives considered.

**Date:** 2026-09-30
**Phase:** 3-trust-model-hardening-diversity-weighted-trust
**Areas discussed:** Counting rule for N, Still current signal, Source reliable signal, How N and signals look, Voting from too far away, Your own rating

**Preparation:** the four gray areas were researched in parallel before the first question (four read-only
researchers plus a reconciler, run `wf_1d4c62dd-e20`). The options below came from those briefs. The
reconciler trimmed 16 candidate questions to 12, pre-filled decisions that follow from locked Phase 2
decisions (N equals the distinct place count, both signals display only, one shared trust block), and
corrected several scouting assumptions (for example the wire key is `your_vote`, there is no Go Haversine
helper, and one account cannot add cells by walking).

**Post draft review:** after CONTEXT.md was drafted, three independent read-only reviewers (decision
fidelity, factual accuracy against the repo, and consistency with planner usability; run
`wf_a6cf31b3-794`) returned no blockers and about 30 findings. They were applied: stale wording removed,
code facts corrected (the 60 second fade timer in `feed.js`, the Activity page having no Mark resolved
control, votes written before Phase 3 being grandfathered), and eight derived rules (R-01 to R-08) added for
states the discussion had not spelled out. The derived rules are flagged in CONTEXT.md as not separately
reviewed by the owner. R-06 (an out disputed critical report counts against the reporter's rating) is a
default the owner should confirm.

---

## Counting rule for N

### Nearby rule

| Option | Description | Selected |
|--------|-------------|----------|
| Reject beyond 2 km (Recommended) | Server refuses the vote with a clear message. Covers all four vote kinds, reporter's own instant resolve or reopen exempt. No migration. Cost: faraway viewers cannot vote, Phase 5 demo mode needs an exemption or simulated reports near the visitor, a spoofer only needs a spot within 2 km. | |
| Reject beyond 1 km | Same rule, tighter. Strongest meaning of nearby, but rejects witnesses of a long flooded stretch and anyone with a stale or coarse location. | ✓ |
| No distance rule | Keep today's behavior, document that nearby only means the voter shared a location. Cost: the word nearby becomes untrue. | |

**User's choice:** Reject beyond 1 km
**Notes:** Orchestrator added: the client refreshes location silently when permission is already granted and
retries once before showing the error, never prompting per vote.

### Precision

| Option | Description | Selected |
|--------|-------------|----------|
| Keep precision 7 (Recommended) | Cells about 132 to 153 m across. Document a derivation table, add a witness simulation test, state the boundary effect and the multi account plus spoofed location limit honestly. No migration. | ✓ |
| Coarser precision 6 | Cells about 1.1 km by 0.6 km, derived by trimming the stored cell. Harder cell farming, but neighbors on a street share a cell and few distinct cells fit inside a 1 km radius. | |

**User's choice:** Keep precision 7
**Notes:** N equals the distinct place count the resolver already gates on, with no dampening curve
(announced as derived from Phase 2 D-14 and TRUST-02; the user did not object).

---

## Still current signal

### Refresh rule

| Option | Description | Selected |
|--------|-------------|----------|
| 2 places in window (Recommended) | Recent only when at least 2 distinct confirming places confirmed inside a window of one quarter of the report's lifetime. One account re-tapping cannot refresh it. | ✓ |
| Any latest confirm | The newest confirm from any counted voter resets the clock. Gameable at zero cost by re-tapping. | |

**User's choice:** 2 places in window

### Fade clock

| Option | Description | Selected |
|--------|-------------|----------|
| Worst of two, critical exempt (Recommended) | Non critical rows fade by the worse of the time stage and the still current stage. Critical and rescue needed rows keep the time fade only. | ✓ |
| Text cue only | Leave the fade purely time based, add still current only as text. The two cues can disagree. | |

**User's choice:** Worst of two, critical exempt

### Wire form

| Option | Description | Selected |
|--------|-------------|----------|
| Bands plus facts (Recommended) | Band plus real facts on the wire, any numeric score stays inside the server. | |
| Bands plus 0 to 100 score | Same, plus an integer 0 to 100 per signal on the wire as the Phase 6 sort key. | ✓ |

**User's choice:** Bands plus 0 to 100 score (the non-recommended option)

### Show the number

| Option | Description | Selected |
|--------|-------------|----------|
| Number in the map popup only (Recommended) | Row shows band and facts, popup also shows the number with a one line meaning. | ✓ |
| Number beside the band everywhere | Row and popup both show it. | |
| Band and facts only | Number stays in the API. | |

**User's choice:** Number in the map popup only
**Notes:** The user was also told that re-confirmation does not extend `expires_at` and that this is listed as
a deferred idea; they did not ask to include it.

---

## Source reliable signal

### Outcomes

| Option | Description | Selected |
|--------|-------------|----------|
| Corroboration only (Recommended) | Corroborated counts for, contradicted counts against, never confirmed and in flight neutral. | |
| Also penalise unconfirmed | Same, plus a report that expires never confirmed counts as a mild negative. | ✓ |

**User's choice:** Also penalise unconfirmed (the non-recommended option)

### Critical edge

| Option | Description | Selected |
|--------|-------------|----------|
| Exempt critical and rescue needed (Recommended) | Their unconfirmed expiry stays neutral; corroborated or contradicted outcomes still count. | ✓ |
| Apply to all reports equally | One rule, but a lone real rescue report that nobody confirms lowers the record. | |

**User's choice:** Exempt critical and rescue needed

### Cold start

| Option | Description | Selected |
|--------|-------------|----------|
| 3 judged, 30 day half-life (Recommended) | Signal appears on sparse data; a newcomer could show a low record after three quiet reports. | |
| 5 judged, 30 day half-life | Harder to mark down a newcomer; on a small deployment most sources stay at no record. | ✓ |
| 5 judged, 90 day half-life | Strictest and slowest; blurs the fast versus slow contrast. | |

**User's choice:** 5 judged, 30 day half-life (accepted that the signal will look absent on a small deployment)

### Guardrails

| Option | Description | Selected |
|--------|-------------|----------|
| Show everywhere, inert (Recommended) | Same neutral text on every report including critical, never affects visibility or blocks anything. | ✓ |
| Hide on critical and rescue needed | Never render it on those reports. | |

**User's choice:** Show everywhere, inert

### Public data (options declined, freeform answer)

| Option | Description | Selected |
|--------|-------------|----------|
| Band and counts, score stays server side (Recommended) | Row shows the band, popup adds counts, exact score never leaves the server. | |
| Band, counts and score all public | Everything visible. | |
| Band only | Only the band leaves the server. | |

**User's choice:** Declined the options to clarify, then answered in their own words: each person should have a
rating based on how their reports turned out and whether people confirmed them, visible to others, with a tag
so readers know a poster is not reliable.
**Notes:** Recorded as a public, person level rating: rating, tag and counts visible on every report, account id
and email never shown, linking a person's reports is intended. Orchestrator listed two limits to document
(a liar can start over with a fresh email; two colluding accounts could dispute an honest reporter). The user
confirmed the reading and asked only for different tag words.

### Tag words (four rounds)

| Option | Description | Selected |
|--------|-------------|----------|
| Trusted reporter, Mixed record, Low reliability, No track record yet | First suggestion. | |
| Trusted, Mixed, Unreliable, New reporter / Reliable, Inconsistent, Not reliable, No history / Often confirmed, Mixed, Often disputed, No record yet | Second round; the user declined the options to clarify. | |
| Four long plain sentences ("Their past reports: usually confirmed by others" and similar) | Third round; rejected as too long. | |
| Reliable reporter, Mixed record, Unreliable reporter, New reporter | Fourth round, short and plain. | ✓ |

**User's choice:** Reliable reporter, Mixed record, Unreliable reporter, New reporter
**Notes:** The user said one word style tags were not clear enough for a new user, then that the sentence style
was too long. During the final round they added that hovering the tag should show the reason.

---

## How N and signals look

### Count copy

| Option | Description | Selected |
|--------|-------------|----------|
| From 2 up, plain line below (Recommended) | Confirmed by N nearby from 2 up, a plain line below 2. | |
| Show the count from 1 | Confirmed by N nearby for any N of 1 or more, nothing at zero. | ✓ |

**User's choice:** Show the count from 1 (the non-recommended option, after being told it clashes with the
Unconfirmed chip at exactly 1)

### Chip wording

| Option | Description | Selected |
|--------|-------------|----------|
| Rename to Needs 2 confirmations (Recommended) | Chip states the rule and agrees with the count at 0 and 1. | |
| Keep Unconfirmed | No change to approved copy; the two can look contradictory at exactly 1. | ✓ |

**User's choice:** Keep Unconfirmed
**Notes:** Orchestrator added a hover explanation on the chip to soften the clash.

### Disputes

| Option | Description | Selected |
|--------|-------------|----------|
| Show disputes plus a count note (Recommended) | Disputed by M nearby as plain text plus a popup note. | ✓ |
| Confirmations only | Show only the confirmation count. | |

**User's choice:** Show disputes plus a count note
**Notes:** Later simplified at the user's request for fewer words: the dispute count is folded onto the count
line and the note moved into the hover.

### Wording and layout (plain text proposals, no tool)

| Option | Description | Selected |
|--------|-------------|----------|
| Four line block with the note and prefixes | First proposal. The user asked for fewer words and details on hover. | |
| Two short lines with details on hover | Confirmed by 3 people nearby, 1 disputed / Up to date, then the reporter tag. | ✓ |

**User's choice:** Two short lines, details on hover
**Notes:** The user asked "confirm by three what?", so the count line became "Confirmed by 3 people nearby", then
approved it. Still current words proposed by the orchestrator and accepted: Up to date, Getting old, Needs
re-confirming, Too early to tell.

---

## Voting from too far away

| Option | Description | Selected |
|--------|-------------|----------|
| Disabled with a hover reason (Recommended) | Buttons greyed out once the location is known, hover says Too far to vote. | ✓ |
| Always enabled, error on tap | People find out only after tapping. | |
| Hide the buttons when too far | No explanation for why voting is unavailable. | |

**User's choice:** Disabled with a hover reason

---

## Your own rating

| Option | Description | Selected |
|--------|-------------|----------|
| One line at the top of Activity (Recommended) | A single line with the reporter's own rating. | |
| Line at the top plus the block on each report | Also the count line and still current word per report. | |
| Not in Phase 3 | Leave Activity as it is. | |

**User's choice:** Declined the options to clarify, then answered: reporters should see it, not too wordy, and
what users see and what reporters see should be the same.
**Notes:** Recorded as one identical trust block on the feed row, the map popup and the reporter's own reports on
the Activity page, with no separate reporter only line. The user also said that clicking a report should show
all of this, and that a reporter should see everything they posted in a section (the Activity page already does).

---

## Claude's Discretion

- Score formulas and band and tag cutoffs, the dispute effect on still current, where the pure functions live,
  wire field names, whether wording is server authored, exact hover sentences and tooltip implementation, the
  Activity page mechanism and how it renders Retracted or expired reports, the weight of the mild unconfirmed
  negative, the silent location refresh mechanism, the too far rejection status and copy, the history read query
  shape, and the `reports (session_id)` index migration.

## Deferred Ideas

- Report page with comments that opens like a post on X (new capability, own phase; user wants it soon).
- Re-confirmation extending `expires_at`; demoting or dropping stale reports; a dampening curve for N.
- GPS accuracy, per category radii, spoof detection; burst and brigading detection; confirmer accuracy scoring.
- Vote rate limiting (Phase 4); email alias normalisation; push notifications; public reporter profile or badge
  tiers; a stored reliability score.
- Database hosting choice (Supabase, Neon, Render, Railway): the user asked what Supabase is and whether it can be
  used; answered in chat, not part of Phase 3.
- Reviewed but not folded: the pending todo "Real 3D WebGL globe on login screen" (keyword match only).
