# Phase 07: Address Search Box for Report Location - Context

**Gathered:** 2026-09-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Add a text address search box to the report-submission modal, alongside the existing map
interactions (GPS auto-fill, tap-to-place, drag-to-adjust), so a person reporting an emergency can
type a place name or address instead of relying only on map gestures. The search box is additive:
it must never replace or remove the existing GPS/tap/drag ways of setting a location. Geocoding
uses OSM Nominatim (free, no API key). This phase does not touch the map's basemap style, the
report list, or any other page; it is scoped to the location step of the submission flow only.

</domain>

<decisions>
## Implementation Decisions

### Search trigger behavior
- **D-01:** Live suggestions as the person types (debounced), not search-on-submit-only. A
  dropdown of matching places appears; tapping one selects it. This is the familiar
  "Google Maps"-style pattern the user expects.
- **D-02:** The debounce must respect Nominatim's usage policy of roughly 1 request per second
  (free tier, no API key). Do not fire a request on every keystroke.

### What a match does to the pin
- **D-03:** Tapping a suggestion auto-places the pin at that location and centers the map there,
  exactly matching how the existing GPS auto-fill behaves today. The user can still drag the pin
  afterward to fine-tune, same as they can after GPS placement. Do not require a second
  tap-to-confirm step after picking a suggestion.

### No match or service unavailable
- **D-04:** A failed or empty search shows a plain inline message near the search box (e.g. "No
  matches found" or "Search unavailable, try tapping the map instead"). It must never block or
  gate report submission — GPS, tap, and drag continue to work the entire time regardless of the
  search box's state. This matches the project's access-model priority: nothing about reporting
  during an emergency should ever be blocked by a secondary convenience feature failing.

### Claude's Discretion
- Exact visual placement of the search box within the modal (above/below the map), its styling
  (no existing text-input style exists in this codebase yet, per the code scout below), the
  debounce interval in milliseconds, dropdown result count/formatting, and whether results are
  cached client-side to reduce duplicate requests for the same query. Follow site-design-rules.md
  (see canonical refs) for anything visual: no purple gradients, no pill-shaped buttons, no emoji
  icons, no em or en dashes in any copy, no over-the-top animation.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Design constraints
- `/Users/swathivallabhaneni/.claude/projects/-Users-swathivallabhaneni-code-pinalert/memory/site-design-rules.md` — hard visual/copy rules from the user (no gradients, no pill buttons, no fake metrics, no hero text, no emoji icons, no em/en dashes, no over-the-top animation). This is a memory file outside the repo; describe its rules inline in any comment, never cite its path as if it were a repo file.

### External service
- Nominatim usage policy (https://operations.osmfoundation.org/policies/nominatim/) — the free-tier
  rate limit (roughly 1 request/second), required attribution, and the prohibition on bulk
  geocoding. The researcher should fetch and verify this directly rather than relying on this
  note, since usage policies can change.

### Backlog origin
- `.planning/ROADMAP.md` Phase 999.1 entry (now marked "PROMOTED to Phase 7") — the original
  backlog capture and its surfacing history (first raised 2026-09-18, repeated by the user
  2026-09-23: "we don't know if it's where we're going... it's like randomly picking up a place").

[No ADR or SPEC.md exists for this phase yet — requirements are TBD per ROADMAP.md and captured
in the decisions above, not locked by a separate spec document.]

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/static/js/modal.js`'s `initLocation()` — the existing GPS-auto-fill, tap-to-place, and
  drag-to-adjust pin logic for the report-submission map. The search box's "auto-place pin, still
  draggable after" behavior (D-03) should mirror exactly how this function already updates the pin
  on a successful GPS fix, not invent a second, different placement mechanism.
- `#coord-readout` and `#location-notice` elements (modal.js lines ~24-25) — the existing
  human-readable coordinate display and the GPS-denied notice. A new inline no-match/error message
  (D-04) should follow this same pattern (a small text element near the location controls), not a
  toast or a blocking modal.

### Established Patterns
- No text `<input type="text">` exists anywhere in the app's forms today (checked
  `web/templates/index.html.tmpl` and `web/static/css/modal.css`) — only a checkbox, a range
  slider, and a number input. There is no existing `.text-input`-style CSS class to reuse; styling
  for the new search box is new ground, not a drop-in reuse. Follow the existing design-token
  system (`--color-*`, `--space-*`, `--font-*` custom properties in `main.css`) rather than
  introducing new hardcoded values.
- The project's rate-limiting/offline conventions (see `internal/ratelimit/`, the client-side
  debounce/retry patterns already in `web/static/js/votes.js` and `auth.js`) are the precedent to
  follow for debouncing and handling a slow/failed external request gracefully, consistent with
  D-04's "never block submission" requirement.

### Integration Points
- The search box lives inside the report-submission modal's location step, alongside the existing
  map (`#modal-map`), not on the main feed/map page. No other page needs this control per the
  phase boundary above.
- Geocoding calls Nominatim directly (no existing backend proxy for this). The planner/researcher
  should confirm whether a client-side-only call is acceptable (Nominatim supports CORS for
  reasonable use) or whether a thin server-side proxy is warranted for rate-limiting/attribution
  control; this was not decided in discussion and is open for research to inform.

</code_context>

<specifics>
## Specific Ideas

No specific visual reference or exact wording was given beyond the decisions above. The user's
own framing of the problem: "we don't know if it's where we're going where that issue is, it's
like randomly picking up a place depending on the location where I'm in, but we need to have that
description for the location also mentioned properly when the person is making a report" — the
core want is a readable, typed description of location that removes ambiguity about where a
report actually is, not just a raw pin.

</specifics>

<deferred>
## Deferred Ideas

None raised during this discussion that fell outside phase scope.

### Reviewed Todos (not folded)
- `2026-09-16-real-3d-webgl-globe-on-login-screen.md` — matched this phase at a low relevance
  score (0.3, generic "ui" area tag only). Reviewed and judged unrelated to address search; not
  folded in.

[No other deferred ideas — discussion stayed within phase scope.]

</deferred>

---

*Phase: 07-address-search-box-for-report-location*
*Context gathered: 2026-09-23*
