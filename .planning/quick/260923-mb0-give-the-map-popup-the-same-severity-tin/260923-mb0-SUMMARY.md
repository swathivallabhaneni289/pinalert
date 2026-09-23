---
task: 260923-mb0
subsystem: ui
tags: [css, javascript, leaflet, trust-mechanic, contrast]
key-files:
  modified:
    - web/static/css/trust.css
    - web/static/js/map.js
    - web/votes_contract_test.go
    - web/js_contract_test.go
key-decisions:
  - "Popup surface reads var(--severity-tint, var(--color-bg)) instead of a bare var(--color-bg), reusing the feed row's existing custom property rather than inventing a parallel one."
  - "Hidden/retracted popup override declares --severity-tint: var(--color-bg) only (no background property), to stay compatible with two existing tests that each require exactly one background-declaring rule per state class."
  - "Close glyph gets an explicit color override (--color-text-muted at rest, --color-text on hover/focus) because the old untinted-surface contrast figures (~2.9:1 to ~3.5:1 dark) no longer clear the 3:1 floor once the surface is severity tinted."
  - "syncPopupState mutates className plus the live popup container in place rather than ever rebinding/unbinding, preserving the existing never-drop-an-open-popup reconcile contract."
status: complete
duration: not tracked (session did not record a start timestamp)
completed: 2026-09-23
---

# Quick Task 260923-mb0: Map Popup Severity Tint Summary

**Leaflet popup background now reads the same `--severity-tint` custom property the feed row reads, with a hidden/retracted override, a themed close-button color override, and two new contract tests proving the wiring.**

## Accomplishments

- The Leaflet popup box and its pointer tail now paint with the report's severity color (soft green/amber/red), a neutral wash for Provisional, and the base surface for Hidden/Retracted, matching the feed row exactly, by routing through the shared `--severity-tint` custom property rather than a second color system.
- `map.js` now carries the same validated severity/age/visibility state classes onto the popup's outer Leaflet container that the pin badge already carries, both at first bind (`className` option) and on every reconcile poll (live container mutation via `popup.getElement()`), without ever rebinding or unbinding an open popup.
- The close (`x`) glyph now gets an explicit theme-aware color override, since the old resting contrast figures (measured against an untinted white/dark surface) no longer hold once the surface itself is severity tinted.
- Two new Go contract tests lock this in: one resolves every tint value through the shipped CSS tokens across all four theme sources and checks WCAG contrast; the other proves `map.js`'s bind-time and reconcile-time wiring exists in the shipped source and that no popup-unbind call was introduced.

## Task Commits

1. **Task 1: Make the Leaflet popup surface severity aware in trust.css** - `062aec2` (feat)
2. **Task 2: Carry the report's state classes onto Leaflet's outer popup container in map.js** - `2476085` (feat)
3. **Task 3: Lock the behaviour in with two new contract tests** - `9e5fd55` (test)

## Files Modified

- `web/static/css/trust.css` - popup surface rule's `background` now reads `var(--severity-tint, var(--color-bg))`; new hidden/retracted popup override (`--severity-tint` only, no `background`, to satisfy the two existing exactly-one-rule tests); new close-button color override pair; point 3 of the block's four-point comment rewritten to describe the new override.
- `web/static/js/map.js` - new `replacePrefixedClass` helper (third deliberate copy, matching feed.js/visibility.js); new `popupStateClasses(report)`; new `syncPopupState(marker, report)`; wired into `upsertMarker`'s existing-marker branch (`syncPopupState` call) and new-marker branch (`className` bind option); SECURITY header note extended.
- `web/votes_contract_test.go` - `TestMapPopupSurfaceIsThemeAware`'s background assertion updated to the new severity-aware value, doc comment extended; new `TestMapPopupTintTracksFeedRowAcrossStatesAndThemes` added directly after it.
- `web/js_contract_test.go` - new `TestMapPopupCarriesReportStateClasses` added at end of file.

## Decisions Made

See frontmatter `key-decisions`. All four were plan-specified, not improvised; no architectural deviation.

## Deviations from Plan

**One process deviation, not a code deviation:** while drafting comments in trust.css, map.js, and the new test functions, I initially wrote a few em dash characters out of habit (matching the codebase's existing pervasive style), which violates this task's explicit no-em/en-dash constraint for anything I write. I caught this before finalizing by grepping every diff I introduced (both committed and uncommitted) for those two dash characters, found four instances (two in already-committed Task 1/2 commits, two in the not-yet-committed Task 3 draft), and rewrote each to avoid the dash. Because two of the violations were already inside git commits, fixing them cleanly (rather than leaving a stray "fix dash" commit) required a `git reset --soft` back to the pre-task base commit followed by a careful re-commit of each task's exact original file scope. This used only `git reset --soft` (never `--hard`) and `git restore --staged <file>` to correct index state; no working-tree content was discarded at any point, and the final three commits are byte-identical in scope to what Tasks 1-3 specify. The plan's own dash-bearing pre-existing sentences (in trust.css, map.js, and votes_contract_test.go) were left untouched throughout, other than reverting two lines where an incidental `gofmt -w` run had reformatted (not reflowed) their operator spacing; both were restored to their original formatting before the final commits.

No other deviations. Plan executed exactly as written otherwise.

## Issues Encountered

None beyond the dash cleanup above.

## Verified Computationally vs. Needs a Live Look

**Verified computationally** (via `go build ./... && go vet ./... && go test ./web/ -short -count=1`, all green, no database involved, plus `node --check web/static/js/map.js`):

- The popup surface rule's `background` declaration is exactly `var(--severity-tint, var(--color-bg))`, and every popup-scoped selector head in `trust.css` carries the `.leaflet-container` ancestor field (`grep -n 'leaflet-popup' web/static/css/trust.css` confirms all seven occurrences).
- The hidden/retracted popup override declares `--severity-tint: var(--color-bg)` and nothing else (verified both by the two pre-existing exactly-one-background-rule tests staying green, and by the new test resolving the token to match `body`'s own declared background).
- Every severity tint (`.sev-low`, `.sev-medium`, `.sev-critical`, `.vis-provisional`) resolves through its declared custom property to a real hex value in all four theme sources (light `:root`, light explicit toggle, dark media query, dark explicit toggle), and `--color-text` clears 4.5:1 / `--color-text-muted` clears 3.0:1 against every one of those resolved surfaces in every theme.
- `map.js`'s `bindPopup(` call passes `className: popupStateClasses(report)` (a function call, not a literal string); `map.js` references Leaflet's `getElement` accessor; `setPopupContent(` is still called on the reconcile path; `map.js` contains no call to Leaflet's popup unbind method.
- `node --check` confirms `map.js` still parses as valid JavaScript with no new globals beyond the existing `{flyTo, highlight}` export.

**Static resolution proves the declarations exist and resolve to these values from the shipped source. It cannot prove a real browser actually composited a tinted Leaflet popup, that the age class carried onto the popup has no visible side effect, or that a live 30-second poll genuinely leaves an open popup undisturbed.** Every visual and interactive claim below is in that second bucket until a human looks at it.

## Restart the Dev Server Before Looking

`web/static` and `web/templates` are embedded with `go:embed` at build time, so an already-running `go run ./cmd/server` (or built binary) keeps serving the **old** CSS and JS no matter how hard the browser is reloaded. Stop and restart the server process, then hard-reload the page, before doing the check below. Do not use port 8080 (it collides with another project on this machine); use something like `PORT=8090` instead.

## Human Check Sweep

Walk this on the running app after restarting the server:

1. Tap a **low severity** pin on the map. Its popup box (and the little pointer beneath it) should be a **soft green** wash, matching that report's feed row.
2. Tap a **medium severity** pin. Popup box should be **soft amber**, matching its row.
3. Tap a **critical severity** pin. Popup box should be **soft red**, matching its row.
4. Tap an **Unconfirmed (Provisional)** report's pin. Its popup box should be a **neutral grey** wash (not colored), matching its row.
5. Toggle **"Show disputed"** on and tap a **Disputed (Hidden)** report's pin. Its popup box should be the **plain base app surface**, with no map tiles showing through it (this is the "reproduce the row's rendered result, not just its transparent keyword" behavior Task 1 built).
6. On every one of the above, confirm the popup's **body text and the close (`x`) button** are both clearly readable against the tinted background.
7. Switch the OS to **dark mode** and repeat steps 1-6.
8. Leave a popup open and wait through one full **30-second poll**. Confirm it **stays open** (does not close itself) and its tint stays correct (it should still reflect the report's current state, even if that state changed during the poll).

---
*Task: 260923-mb0*
*Completed: 2026-09-23*
