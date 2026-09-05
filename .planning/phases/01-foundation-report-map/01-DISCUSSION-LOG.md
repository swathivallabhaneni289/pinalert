# Phase 1: Foundation — Report & Map - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-05
**Phase:** 1-foundation-report-map
**Areas discussed:** Report submission flow, Landing view, Visual tone, Default expiry durations

---

## Report submission flow

| Option | Description | Selected |
|--------|-------------|----------|
| GPS pre-fills, draggable | Auto-GPS places a marker the visitor can drag to correct | ✓ |
| Tap the map only | Visitor taps the spot themselves, no GPS permission needed | |
| GPS only, no adjustment | Simplest, but wrong if GPS is imprecise | |

**User's choice:** GPS pre-fills, draggable

| Option | Description | Selected |
|--------|-------------|----------|
| Modal over the map | Floating panel opens on top of the map | ✓ |
| Separate page | Navigates to a dedicated /report page | |

**User's choice:** Modal over the map

**Category set** — mid-discussion clarification: the user flagged that the original 5-category
list (flooding, road blocked, power outage, shelter open, rescue needed) was too flood/cyclone-
specific for a product pitched as covering disasters generally ("could be earthquake or any other
natural calamity... cover most of the disasters and help people out").

| Option | Description | Selected |
|--------|-------------|----------|
| Broad set + Other | Flood, Earthquake, Fire, Storm/Cyclone damage, Road blocked, Power outage, Shelter open, Rescue needed, Other (9 categories) | ✓ |
| Moderate set | 7 categories, drops Storm and Other | |
| Keep original 5 | Stays flood/cyclone-focused | |

**User's choice:** Broad set + Other (9 categories)

| Option | Description | Selected |
|--------|-------------|----------|
| 3x3 icon grid, single tap | 9 tappable icons in a grid | ✓ |
| Dropdown select | Standard HTML select | |
| Grid of top 5 + "More" expander | Shows 5 common categories, expandable | |

**User's choice:** 3x3 icon grid, single tap

**Severity UX** — user rejected the initial "3 color-coded buttons vs. slider" framing mid-question
to clarify their preference directly: "be the slider but... go creative with it just a little bit,
but make sure it's still accessible... make the transition and everything smooth. add numbers for
the levels so they know clearly."

| Option | Description | Selected |
|--------|-------------|----------|
| Slider (creative, accessible, numbered) | Animated slider with number + label per level, high contrast, keyboard/screen-reader operable | ✓ |
| 3 color-coded buttons | Green/yellow/red tap buttons | |

**User's choice:** Slider, with smooth animated transitions and numbered levels ("1 · Low", "2 ·
Medium", "3 · Critical"), accessibility non-negotiable

---

## Landing view

| Option | Description | Selected |
|--------|-------------|----------|
| Map-first | Live map centered on visitor's location | |
| List-first | Scrollable feed with map toggle | |
| Split view | Map + list side-by-side on wider screens | ✓ |

**User's choice:** Split view (map + list side-by-side)

| Option | Description | Selected |
|--------|-------------|----------|
| Map, with list toggle | Mobile defaults to map, one tap reveals list | ✓ |
| List, with map toggle | Mobile defaults to list, one tap reveals map | |

**User's choice:** Map, with list toggle

| Option | Description | Selected |
|--------|-------------|----------|
| Severity then recency | Critical always on top, newest-first within band | ✓ |
| Nearest first | Pure distance ordering | |
| Newest first | Pure chronological | |

**User's choice:** Severity then recency

| Option | Description | Selected |
|--------|-------------|----------|
| Floating action button | Persistent round button over the map | ✓ |
| Top bar button | Button in the header | |

**User's choice:** Floating action button

---

## Visual tone

| Option | Description | Selected |
|--------|-------------|----------|
| Calm-authoritative | Blue/white/slate, official-emergency-services feel | |
| Alarm/urgent | Red/orange, bold, high-contrast | |
| Neutral utility | Grayscale/minimal, color only for severity | ✓ |

**User's choice:** Neutral utility

| Option | Description | Selected |
|--------|-------------|----------|
| Traffic-light | Green/amber/red | ✓ |
| Blue-to-red gradient | Cool blue through amber to red | |

**User's choice:** Traffic-light

| Option | Description | Selected |
|--------|-------------|----------|
| Simple line icons | Minimal outlined icons (Lucide/Feather style) | ✓ |
| Filled/solid icons | Bolder filled icons | |
| Emoji | Native emoji | |

**User's choice:** Simple line icons

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, from Phase 1 | Light/dark via CSS variables from the start | ✓ |
| Light only for now | Dark mode deferred to later | |

**User's choice:** Yes, from Phase 1

---

## Default expiry durations

| Option | Description | Selected |
|--------|-------------|----------|
| Tiered by urgency (per-category) | Different durations per category (24h/12h/48h) | |
| Two tiers only (by severity) | Critical = 24h, Low/Medium = 8h | ✓ |

**User's choice:** Two tiers only, by severity

| Option | Description | Selected |
|--------|-------------|----------|
| Gradual visual fade | Opacity decreases in the last ~25% of lifetime | ✓ |
| Disappears instantly at cutoff | No warning, just gone | |

**User's choice:** Gradual visual fade

---

## Claude's Discretion

- Session-identity storage mechanism (cookie vs. localStorage)
- Exact icon library within "simple outlined line icons" (Lucide vs. Feather vs. Heroicons outline)
- Exact fade curve/easing for expiry and severity-slider transitions

## Deferred Ideas

None — discussion stayed within Phase 1 scope. Per-category expiry tuning and confirm/dispute-
driven fade behavior are already correctly scoped to Phase 2/3 per ROADMAP.md.
