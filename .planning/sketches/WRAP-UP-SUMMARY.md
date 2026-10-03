# Sketch Wrap-Up Summary

**Date:** 2026-10-03
**Sketches processed:** 1 of 2
**Design areas:** Popup structure, Trust block
**Skill output:** `./.claude/skills/sketch-findings-pinalert/`

## Included Sketches

| # | Name | Winner | Design Area |
|---|------|--------|-------------|
| 001 | map-pin-popup | D4, From your reference | Popup structure, Trust block |

## Excluded Sketches

| # | Name | Reason |
|---|------|--------|
| 002 | full-report-view-and-thread | No winner was chosen. It belongs to the report page and comments phase, which does not exist yet. A later wrap-up can include it once that phase is scoped and the sketch has a winner. |

## Design Direction

Pinalert reads as a serious emergency utility: neutral, restrained and credible, built for someone checking a situation
quickly on a phone. The popup is a neutral card with the severity colour kept to a left bar, the badge and a severity
pill. The trust block is plain text inside one button that opens a short panel of sentences. Comments live on a larger
report page, never inside the popup.

## Key Decisions

- **Layout:** a 12px card up to 340px wide (328px on a 360 phone, 288px on a 320 phone), a 4px severity bar down the full
  height, one 16px gutter, and six parts in order: badge row, title, meta, trust block, actions, comments footer.
- **Palette:** grayscale plus severity only. The Critical pill is the severity red on its tint, Medium and Low keep the
  word in the text colour with a hue dot. A pressed vote is an inverted fill, not a hue. The footer has its own surface
  in dark mode only.
- **Typography:** the existing four sizes and two weights. Title 18 semibold, counts 16 with a semibold number and a
  regular word, signals and meta 14, buttons 16 semibold.
- **Spacing and targets:** the existing scale, every target at least 44px, 16px and 20px Lucide icons drawn as masks.
- **Interaction:** the trust block is one disclosure button (expanded or collapsed), Escape closes the panel first and
  then the popup. Confirm and Dispute toggle. Mark resolved asks first. The too far reason is always visible text. A
  30 second refresh must never close an open popup, move focus or collapse an open panel.

## Open Questions

- D4 departs from nine locked Phase 3 decisions (pill label, grey signal words, icons in the block, one disclosure
  button instead of per word reasons, no Unconfirmed explanation, a different confirmed sentence, a native disabled
  too far state, a 340px neutral card, a comments footer with no backend). The Phase 3 UI contract must record each.
- How the feed row and the Activity page show the block (D-20 says it is identical everywhere).
- `REVIEWS.md` decisions 2, 3 and 7 (Mark resolved weight, title source, longest category label) and the forced
  colours part of 8. Decisions 4 to 6 belong to the report page and comments phase.
- The sample sentences and figures are placeholders until the server's trust object supplies real numbers.
