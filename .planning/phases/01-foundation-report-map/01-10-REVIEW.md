---
phase: 01-foundation-report-map
reviewed: 2026-09-06T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - web/static/js/map.js
  - web/static/js/modal.js
  - web/js_contract_test.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
  fixed: 2
status: issues_found_and_fixed
---

# Phase 01 Plan 10: Code Review Report

**Reviewed:** 2026-09-06T00:00:00Z
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

The production change itself is correct and provably minimal. `git show 2b17e1a` confirms a pure
`+1/-0` insertion in each of `web/static/js/map.js` (6-space indent) and `web/static/js/modal.js`
(8-space indent, matching its one-block-deeper nesting) — `detectRetina: true,` inserted exactly
between the existing `maxZoom: 19` and `attribution` lines in both files, with the tile URL
template, `maxZoom` value and attribution string byte-identical to before in both. `node --check`,
`go build ./...`, `go vet ./...` and `go test ./...` all pass. I independently reproduced the
plan's bidirectional bite-check: removing `detectRetina: true,` from `modal.js` alone made
`TestTileLayersRequestRetinaTiles` fail naming `static/js/modal.js` verbatim as the SUMMARY
claims, and restoring the line left the file byte-identical (`git diff --stat` clean) with the
test passing again.

I also specifically checked `01-10-PLAN.md` and `01-10-SUMMARY.md` for the orphaned-tool-tag
contamination class flagged as having occurred twice before in this phase (in `01-09-PLAN.md`):
`grep -nE "</(content|invoke|parameter|function_calls)>|<invoke|<parameter"` against both files
returns **no matches**. Both documents are clean.

The defect surface is entirely in the new regression test's search-window scoping, not in the
production JS. `TestTileLayersRequestRetinaTiles` locates the tile-layer anchor
(`L.tileLayer(`) and then searches from that anchor to **end of file** for `detectRetina` and its
value, rather than bounding the search to the tile-layer constructor call itself. I confirmed by
direct mutation-testing that this makes the test **pass even when the real tile layer is missing
the option**, as long as the identifier `detectRetina: true` appears anywhere later in the same
file (see WR-01). The same unbounded window means the plan's stated primary long-term risk — a
future edit silently zeroing or removing `maxZoom`, which Leaflet 1.9.4 uses to gate the entire
retina branch — has zero durable enforcement despite the plan explicitly rating that threat
(T-01-33) "mitigate" on the strength of a gate that only ran once, in the executor's `<verify>`
block, and left no trace in committed code (see WR-02).

No security issues, no hardcoded secrets, no dangerous functions, no debug artifacts, and no dead
code were found in any of the three files. No BLOCKER-tier finding: the two-line production
change ships correctly regardless of the test's gap. The findings below are about whether this
new regression gate will actually catch the regression it was built for.

## Warnings

### WR-01: `TestTileLayersRequestRetinaTiles`'s search window is unbounded, so it can pass with the tile layer missing the option entirely

**File:** `web/js_contract_test.go:80-116`

**Issue:** After locating the anchor, the test builds its search window as everything from the
anchor to the end of the file:

```go
anchorIdx := strings.Index(text, tileLayerAnchor)
body := text[anchorIdx:]

optIdx := strings.Index(body, retinaOption)
```

The plan correctly forbids brace-span scoping (`{s}/{z}/{x}/{y}` puts the tile URL's own closing
brace-adjacent characters inside a string, so a naive `{`...`}` span would never reach the real
options object) — but the fix that shipped went to the opposite extreme: no upper bound at all.
I confirmed this concretely by mutation: with `detectRetina: true,` removed from the actual tile
layer construction in `map.js` and the identical string inserted instead into the unrelated
`L.divIcon({...})` call fourteen lines later in the same file, `node --check` still passes (valid
JS) and `TestTileLayersRequestRetinaTiles` **still passes** — the exact regression this test
exists to catch (a tile layer silently losing `detectRetina`) goes undetected as long as the
string shows up anywhere later in the file. The file was restored byte-identical afterward.

This is not a hypothetical: `map.js` alone has three more object-literal constructions after the
tile layer (`L.divIcon`, `L.marker`, the popup builder), and `modal.js` has several more `L.map`/
`L.marker` calls — plenty of real places a future refactor could introduce or move a stray
`detectRetina`-shaped string without anyone noticing this test stopped meaning what its name says.

**Fix:** Bound the window at the point the tile-layer constructor call actually ends. Both shipped
files terminate the expression with `).addTo(`, which appears nowhere inside the URL template or
the options object:

```go
body := text[anchorIdx:]
if endIdx := strings.Index(body, ").addTo("); endIdx != -1 {
    body = body[:endIdx]
}
```

I verified this bound still parses both files' current tile-layer calls correctly (I hand-traced
it against the committed source) and would have caught the mutation above, since `L.divIcon(`
appears well after `).addTo(map)` closes the tile-layer statement.

**Outcome: fixed.** Bounded the window at `.addTo(` (via a `tileLayerEnd` constant) exactly as
proposed. Reproduced this review's own mutation test — moved `detectRetina: true` out of the tile
layer and into a bait comment on an unrelated function later in `map.js` — and confirmed the test
now correctly fails naming `static/js/map.js`, then restored the file byte-identical
(`git status --porcelain web/static/js` clean) and confirmed the test passes again.

### WR-02: T-01-33's claimed `maxZoom` mitigation does not survive past plan execution — no durable gate asserts it

**File:** `web/js_contract_test.go` (whole file); `.planning/phases/01-foundation-report-map/01-10-PLAN.md:91-96, 284`

**Issue:** The plan's threat register rates T-01-33 ("the load-bearing max-zoom option both tile
layers depend on") as `medium` / disposition `mitigate`, citing "a dedicated `grep -F` gate" that
"asserts that option is still present in both files" plus the new test's doc comment recording the
dependency. The `grep -F 'maxZoom: 19'` gate is real, but it lives entirely in
`01-10-PLAN.md`'s `<verify>` block — a one-time check the executor ran during this plan's
execution and which leaves no trace afterward. `web/js_contract_test.go` never asserts anything
about `maxZoom`; its doc comment only *describes* the dependency in prose (lines 24-30), including
the sentence "that is why every content-gate in this plan also asserts maxZoom is still present" —
which is true of this plan's one-time verify gates but not of anything that ships in the repo
going forward.

Net effect: the exact failure mode the plan calls out as its primary long-term risk — a future
edit removing or zeroing `maxZoom`, silently turning `detectRetina: true` into a no-op with "no
error, no test failure... just the blur quietly returning" (the plan's own words) — has no
regression coverage at all once this plan's worktree is gone. The threat register's "mitigate"
disposition is not backed by a durable artifact.

**Fix:** Extend the same bounded per-file window (see WR-01) to also assert `maxZoom` is present
with a value greater than zero, in the same walk that already inspects each tile layer's body:

```go
maxZoomIdx := strings.Index(body, "maxZoom")
if maxZoomIdx == -1 {
    t.Errorf("%s: tile layer construction is missing maxZoom entirely — Leaflet 1.9.4's "+
        "retina branch is gated on options.maxZoom > 0, so this silently disables detectRetina "+
        "with no other visible signal", path)
} else {
    // parse the numeric value the same way the retina value is parsed, and
    // t.Errorf if it is absent, zero, or negative.
}
```

This closes the actual gap T-01-33 claims is closed, and rides along in the same bounded window as
WR-01's fix at negligible additional cost.

**Outcome: fixed.** Added a `readOptionValue` helper (shared between the `detectRetina` and
`maxZoom` checks to avoid duplicating the colon/comma-or-brace parsing) and asserted `maxZoom`
parses as a positive integer within the same bounded window. Reproduced this review's own mutation
test — set `maxZoom: 0` in `modal.js` — and confirmed the test now correctly fails naming
`static/js/modal.js` with a message pointing at the retina-branch gating condition, then restored
the file byte-identical and confirmed the test passes again. `go build ./... && go vet ./... && go
test ./...` all pass with both fixes in place.

## Info

### IN-01: Tool-artifact contamination check — explicit negative result

**File:** `.planning/phases/01-foundation-report-map/01-10-PLAN.md`,
`.planning/phases/01-foundation-report-map/01-10-SUMMARY.md`

Per this review's specific instruction to verify directly (this exact defect class — orphaned
closing tags from a prior tool call — occurred twice before in this phase's planning documents, in
`01-09-PLAN.md`): `grep -nE "</(content|invoke|parameter|function_calls)>|<invoke|<parameter"`
against both `01-10-PLAN.md` and `01-10-SUMMARY.md` returns no matches. Both files read cleanly
start to end (367 and 158 lines respectively, both manually read in full). No action needed; this
entry records that the check was performed and passed, rather than silently omitting it.

---

_Reviewed: 2026-09-06T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
