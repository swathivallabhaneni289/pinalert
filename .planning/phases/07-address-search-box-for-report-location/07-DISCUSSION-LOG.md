# Phase 07: Address Search Box for Report Location - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-23
**Phase:** 07-address-search-box-for-report-location
**Areas discussed:** Search trigger behavior, What a match does to the pin, No match or service down

---

## Search trigger behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Live suggestions | Dropdown of matches appears as they type, debounced to respect the free geocoder's 1-request-per-second limit | Yes |
| Search on submit only | Type full address, press Enter/Search, jump to best match | |
| You decide | Leave to Claude's judgment | |

**User's choice:** Live suggestions.
**Notes:** None beyond the selection.

---

## What a match does to the pin

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-place the pin | Pin jumps to the address, map centers there, still draggable after, matching how GPS auto-fill already works | Yes |
| Pan only, no pin placed | Map moves there but pin stays put until a manual tap/drag | |
| You decide | Leave to Claude's judgment | |

**User's choice:** Auto-place the pin.
**Notes:** None beyond the selection.

---

## No match or service down

| Option | Description | Selected |
|--------|-------------|----------|
| Plain inline message | Small note under the search box; GPS/tap/drag keep working the whole time | Yes |
| Block submission until resolved | Treat as an error state that must clear before submitting | |
| You decide | Leave to Claude's judgment | |

**User's choice:** Plain inline message.
**Notes:** None beyond the selection.

---

## Claude's Discretion

- Exact visual placement of the search box within the modal.
- Styling of the new text input (no existing `.text-input` style to reuse).
- Debounce interval in milliseconds.
- Dropdown result count and formatting.
- Whether results are cached client-side to reduce duplicate Nominatim requests.
- Whether geocoding calls Nominatim directly from the client or through a thin server-side proxy
  (not decided in discussion; left for research to inform).

## Deferred Ideas

None raised during this discussion that fell outside phase scope. One pending todo
(`2026-09-16-real-3d-webgl-globe-on-login-screen.md`) matched this phase at a low relevance score
(generic "ui" area tag only) and was reviewed but judged unrelated; not folded in.
