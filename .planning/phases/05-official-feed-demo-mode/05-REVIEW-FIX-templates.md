---
phase: 05
part: templates
base: e2a20bdd
fixed_at: 2026-10-29
review_path: .planning/phases/05-official-feed-demo-mode/05-REVIEW-templates.md
iteration: 1
findings_in_scope: 12
fixed: 7
not_reproduced: 1
blocked: 1
recorded_only: 4
status: partial
---

# Phase 5 code review fix: templates and styles

**Fixed:** 2026-10-29, on base e2a20bdd, worktree branch worktree-agent-a9efceb1c3fcd3651.
**Source review:** 05-REVIEW-templates.md, under the triage and decisions in 05-REVIEW.md.
**Not in this part:** the browser fixer owns WR-T02, WR-T05 and IN-T01.

## Findings

| Id | Outcome | Commit | Proof |
|----|---------|--------|-------|
| WR-T01 | fixed | 014ce08d, 3d682fd9 | TestBannerOneLineAndTarget (amended, dated) pins the 359.98px block and that no other rule wraps the banner. In the browser at 320px, the area is 256px wide and the row is 46px tall in every case, with no sideways scroll. |
| WR-T03 | fixed | 88881719 | TestOfficialCardSpacingBeatsPopupReset. In the browser the honesty line has a 12px top margin and the other lines have 4px, while a plain p in the same popup still gets 0px. |
| WR-T04 | fixed, no visible change | 13cc2824 | TestLoginGlassTextContrast (amended, dated). In the browser the colour is the text colour at 16px on the solid surface. |
| WR-T06 | fixed | 35fb887d | TestOfficialRowHoverKeepsMutedTextReadable. In the browser a hovered row's area is 15.20 to 1 in light and 16.15 to 1 in dark. |
| WR-T07 | blocked, not changed | none | Needs a line in internal/ (see below). |
| IN-T02 | fixed | 3717c3ea | TestDemoStripCopyAndExit (amended, dated). In the browser the label is 16px with a 24px line height, the button is 44px tall, and the strip does not overflow at 320px. |
| IN-T05 | fixed | 9b89c228 | TestOfficialHazardLineSitsCloseOnBothSizes |
| IN-T08 | not reproduced | none | Nearby has had no Past view since Phase 3.4 (`app.js` VIEWS is active, all and disputed), so the Demo map draws only live reports and an Expired pin never meets the Demo ring. |
| IN-T09 | fixed | e0e308f3 | The Official alerts contract test (amended, dated) checks the tag in demo.css, that no other stylesheet defines `.demo-tag`, and that every Demo page loads demo.css. |
| IN-T03 | recorded | none | As triaged: no page loads both official.css and official-pins.css today. |
| IN-T04 | recorded | none | As triaged: the word and the icon carry the level. |
| IN-T06 | recorded | none | As triaged: tell the owner about the draft Policy sentences before the next deploy. |
| IN-T07 | recorded | none | As triaged: a platform note; the text still measures at least 4.99 to 1. |

## Notes per fix

### WR-T01: the banner below 360px

There is a new top level block, `@media (max-width: 899.98px) and (max-width: 359.98px)`, in phone.css. It follows the repo's pattern, and the CSS parser in the tests reads it.

How the narrow banner lays out:
- The row wraps.
- The area takes the second line with `order: 1` and the full width, indented under the event. It still shortens with its own ellipsis.
- The dot is hidden.
- The event has `flex: 1 1 0`, so the first line can never wrap.

That last point comes from the browser check. The review's `flex: 0 1 auto` guard let the first line wrap when the Demo tag, "Thunderstorm warning" and "+2" met, and the row grew to 70px. The follow-up commit 3d682fd9 fixed it.

UI-SPEC 2a (the Height row and the "reports it rather than wrapping" paragraph) and R3 carry dated amendments. At 360px and wider nothing changed.

Measured in WebKit and Chromium, light and dark:

| Width | Demo sample ("Demo Flood warning +1") | "Thunderstorm warning +2", no tag | Demo tag with "Thunderstorm warning +2" |
|-------|-----------------------------------------|-----------------------------------|------------------------------------------|
| 320 | area 256px, height 46px | area 256px, height 46px | area 256px, height 46px; the event ellipses (113 of 154px) |
| 360 | area 48px, height 44px | area 48px, height 44px | area 0px |
| 390 | area 69px, height 44px | area 78px, height 44px | area 17px |

There was no sideways scroll at any width.

For the owner's viewing session: the last column pairs the Demo tag with "Thunderstorm warning" and "+2". The Demo does not draw that pairing today, because its sample is "Flood warning". At 360px that pairing pushes the chevron 13px past the row's padding, still inside the screen. The binding decision keeps 360px and wider unchanged, so I left it.

### WR-T03: the GDACS card spacing

The fix adds three class rules: `.leaflet-container .official-card .official-card__line`, `__detail`, `__honesty` and `__credit` set `margin: 0 0 4px`, and the honesty line then sets a 12px top margin. The base rules are kept.

The browser check put the card markup into the map's popup pane on the Demo page, with official-pins.css loaded, at 320, 360, 390 and 1470. The fix holds on both sizes, and a plain p in the same container still gets the trust.css reset of 0px.

### WR-T04: the sign in error line

The colour painted on screen was already the normal text colour, as the decision wants, so no behaviour test could fail first. The amended test failed first on the dead declarations and on the wrong comment.

What changed:
- The dead red, 14px and label line height declarations leave `.auth-error`.
- The 261029-47j comment in login-globe.css now gives the painted pair: 15.20 to 1 light and 16.15 to 1 dark.
- The test keeps its old red on surface case. It now also pins the winning `.login-card p` and `#check-inbox p` rules, that the line is a p inside them, and the painted pair.
- The solid backing is kept.

### WR-T06: the hovered laptop row

Inside the existing `@media (hover: hover)` block, the area and grouped lines of a hovered row now use `--color-text`.

### WR-T07: blocked, needs a file outside this part

Removing the `aria-label`, so the name becomes the visible "Exit demo", breaks `internal/api/handlers/demo_page_e2e_test.go:222`. That test lists `aria-label="Exit the demo"` among the strings the Demo page must contain. internal/** belongs to the server fixer, so I made no change to the template, the web test or UI-SPEC P5-C15.

The complete fix, to apply in one commit after the merge:
- `web/templates/demo_strip.html.tmpl:12`: `<a class="btn demo-strip__exit" href="{{.ExitHref}}">Exit demo</a>`, with no aria-label.
- `web/demo_contract_test.go:209`: amend with a dated note so it requires `>Exit demo</a>` and must not contain `aria-label="Exit the demo"`.
- `internal/api/handlers/demo_page_e2e_test.go:222`: replace `aria-label="Exit the demo"` with `>Exit demo</a>`, with a dated note.
- UI-SPEC lines 315, 479 (P5-C15) and 511: dated amendment saying the accessible name is the visible "Exit demo".

### IN-T02, IN-T05 and IN-T09

- **IN-T02:** `.demo-strip__exit` now sets `--font-size-body` and `--line-height-body`. UI-SPEC line 93 already said 16px, so the spec needed no amendment.
- **IN-T05:** on the laptop, `.official-status:has(+ .official-status--hazards:not([hidden]))` drops its 8px bottom padding, so the hazard line sits 4px under the first line on both sizes. Only the real Official alerts page has a hazard line, and the browser check served only the Demo pages, so this one is proven by the contract test alone.
- **IN-T09:** `.demo-tag` leaves official.css, in its two tag rules and its forced colours list. demo.css keeps the one definition. On both Demo pages that show alert rows, demo.css loads after official.css, so it already won and nothing on screen changes.

## Checks

- **Go:** the full suite on p5_fix_t (`go test ./... -p 1 -count=1 -v`) gave 1847 PASS, 0 `--- FAIL` and 0 `^FAIL`. The only skips were TestGeocodeLive, TestOfficialLive and TestScreeningFixture. No flake reruns were needed.
- **Node:** `node --test web/*.mjs` gave 1082 pass and 0 fail.
- **Browser setup:**
  - The real `api.NewRouter` ran from a throwaway Go test on 127.0.0.1:8798, with a literal throwaway session secret, a recording mailer, the Demo store and p5_fix_t. There was no official fetcher, so no SACHET or GDACS call.
  - Playwright 1.63 drove WebKit and Chromium at 320x568, 360x640, 390x844 and 1470x830, light and dark.
  - The scratch Go file was deleted before the suite ran and before any commit. The probe scripts stay in the session scratchpad only.
- **Clean up:** the server is stopped (port 8798 answers nothing), and p5_fix_t is dropped (`psql -lqt` shows no match).
- **Dashes:** no long or short dash in any added line.

---

_Fixed: 2026-10-29_
_Fixer: Claude (gsd-code-fixer), templates part_
_Iteration: 1_
