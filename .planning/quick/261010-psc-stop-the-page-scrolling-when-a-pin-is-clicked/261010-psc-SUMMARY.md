---
quick_id: 261010-psc
type: quick
status: complete
description: Stop the page scrolling when a pin is clicked on a wide screen
completed: 2026-10-10
commits: 4cf8367 (red test), 69470d6 (fix), and the docs commit that holds this file
---

# Quick task 261010-psc summary

## Done

- A selection that starts from the map now applies the row highlight (`report-row--selected`) and nothing else.
  In `web/static/js/feed.js`, `applySelectionHighlight(id)` lost its `scroll` parameter and the call that asked
  the browser to bring the row into view. The `source === 'map'` guard stays, the call in `activateRow` follows
  the new signature, and both doc comments were rewritten. No `scrollIntoView` is left in the file. A selection
  that starts from a list row runs the same steps in the same order.
- No CSS, markup, `map.js`, focus, keyboard, A74 flyTo, autopan, vote or trust code changed.
- `TestMapSelectionNeverScrollsThePage` in `web/popup_contract_test.go`, next to
  `TestListRowActivationOpensThePopupOnce`. It pins: no `scrollIntoView(` anywhere in the raw `feed.js` text,
  `applySelectionHighlight(id)` with one parameter that adds and removes `report-row--selected` and holds no
  `scroll` word, the single `Pinalert.onSelect` subscriber applies `applySelectionHighlight(id);` after the
  `source === 'map'` guard, and `activateRow` still highlights.
- Scripted check `T9.18` in the browser harness (`~/code/pinalert-browser-checks`, outside the repo):
  `checks/test9.js` gained a section at the end, and `lib/selectors.js` gained one appended block (new keys
  only: `S.rowSelected(id)` and `S.anyRowSelected`, because the page marks the selected row with that class and
  nothing else).
- Amendment A87 in `03-UI-SPEC.md` (new row after A86, and the row count sentence now says 87 rows, A1 to A87).

## How it was checked

Red first, the Go test on the unfixed `feed.js`:

```
popup_contract_test.go:684: feed.js: must not contain "scrollIntoView("
popup_contract_test.go:689: applySelectionHighlight: missing "function applySelectionHighlight(id) {"
popup_contract_test.go:692: applySelectionHighlight: must not contain "scroll"
popup_contract_test.go:702: onSelect subscriber: "applySelectionHighlight(id);" is missing
popup_contract_test.go:706: activateRow: missing "applySelectionHighlight(id);"
--- FAIL: TestMapSelectionNeverScrollsThePage (0.00s)
```

Go suite on the fix, scratch database `p3_scroll_fix` (dropped at the end), `-p 1 -count=1 -v`: 385 PASS, 0 FAIL,
0 SKIP at top level (the recorded baseline is 384, plus this test). `go vet ./...` clean, `gofmt -l web internal
cmd` lists nothing.

Harness, the check is 1200x700, 7 reports so the page is 2200px tall in a 700px window, a real mouse click on
the pin of the report whose row starts below the fold, popup open, `window.scrollY` unchanged, row highlighted.
It asserts the page is taller than the window before it clicks, so it cannot pass vacuously.

Before the fix (HEAD 4cf8367, `--tests 9`, Chromium and WebKit, 76 PASS and 2 FAIL for test 9):

```
[FAIL] T9 chromium | T9.18 ... | page 2200px tall in a 700px window; row of report 17 (A) starts at 1900px, below the fold; pin clicked at 292,93; popup open true (title "T9 pin click keeps the page A", is that report true); scrollY before 0, after 1500 (map top at -1500px); that row carries the selected mark true, rows with the mark 1
[FAIL] T9 webkit   | T9.18 ... | page 2200px tall in a 700px window; row of report 40 (A) starts at 1900px, below the fold; pin clicked at 292,93; popup open true (title "T9 pin click keeps the page A", is that report true); scrollY before 0, after 1500 (map top at -1500px); that row carries the selected mark true, rows with the mark 1
```

After the fix (HEAD 69470d6, `--tests 2,5,9,11`, Chromium and WebKit, exit code 0, no FAIL anywhere):

```
[PASS] T9 chromium | T9.18 ... | page 2200px tall in a 700px window; row of report 25 (A) starts at 1900px, below the fold; pin clicked at 292,93; popup open true (title "T9 pin click keeps the page A", is that report true); scrollY before 0, after 0 (map top at 0px); that row carries the selected mark true, rows with the mark 1
[PASS] T9 webkit   | T9.18 ... | page 2200px tall in a 700px window; row of report 60 (A) starts at 1900px, below the fold; pin clicked at 292,93; popup open true (title "T9 pin click keeps the page A", is that report true); scrollY before 0, after 0 (map top at 0px); that row carries the selected mark true, rows with the mark 1
```

Test 9 compared row by row between the two runs (84 rows each, keyed by engine and step): the only status
change is T9.18, FAIL to PASS in both engines. Counts per engine on the fix: test 2 10 PASS and 1 NEEDS A
PERSON; test 5 Chromium 35 PASS and 2 NEEDS A PERSON, WebKit 31 PASS, 1 NOTE and 2 NEEDS A PERSON; test 9
39 PASS, 2 NOTE and 1 NEEDS A PERSON; test 11 Chromium 68 PASS, 11 NOTE and 1 NEEDS A PERSON, WebKit 60 PASS,
12 NOTE and 1 NEEDS A PERSON. Whole run: 292 PASS, 29 NOTE, 10 NEEDS A PERSON, 0 FAIL.

## Documents and code comments that still quote the old behaviour (not edited)

- `.planning/phases/01-foundation-report-map/01-UI-SPEC.md` line 208 (wide screens: "Clicking a map pin opens its
  popup AND scrolls the corresponding list row into view with a brief highlight ring"). A87 supersedes this
  clause for map originated selections.
- `.planning/phases/01-foundation-report-map/01-06-PLAN.md` lines 255 to 257 (action), 286 (human check) and
  290 (acceptance criteria), and `.planning/phases/01-foundation-report-map/01-06-SUMMARY.md` line 61 ("highlights
  and scrolls the matching row into view").
- `.planning/phases/04-robustness-real-world-resilience/04-UI-SPEC.md` P4-A25 (line 129) names
  `applySelectionHighlight` (`scrollIntoView({ block: 'nearest' })`) as an existing call that the
  `scroll-padding-top` rule serves, and line 1187 and `04-08-PLAN.md` line 27 repeat "Tab focus and
  `scrollIntoView`". The rule still matters for Tab focus and any other scroll; only the citation of
  `applySelectionHighlight` is stale.
- `web/static/js/map.js` line 915 comment on `highlight` ("clicking a pin scrolls/rings the matching list row").
  `map.js` was out of scope for this task. A one line comment fix is the only follow up it needs.

The "always in sync" wording of the Phase 1 research and plans (`01-RESEARCH.md`, `01-03-PLAN.md`,
`01-04-PLAN.md`) is about the shared store feeding both panes and is still true.

## Deviations from the brief, on purpose

- The task folder files are named `261010-psc-PLAN.md` and `261010-psc-SUMMARY.md`, as in all five existing quick
  task folders, not bare `PLAN.md` and `SUMMARY.md`.
- The tool guard refused `env -i ... go test` as a command it could not verify, so the suite ran as
  `DATABASE_URL=<scratch url> go test ./... -p 1 -count=1 -v`. The shell had no database variables set
  (checked with `printenv`) and the code reads only `os.Getenv("DATABASE_URL")`, with no `.env` loader, so the
  effect is the same as the scrubbed environment.
- One harness run `--tests 2,5,9,11` on the fix commit covers both the test 9 re-run and the tests 2, 5, 11
  regression run. The run on the unfixed server was `--tests 9`.
- The harness used its own work folder (`PINALERT_CHECKS_WORK`) with the vendor cache copied from an older run,
  so nothing was downloaded and no other session's work folder was touched.
- Seven reports, not six, so the list is far taller than the window (about 300px per row at 1200px wide).
- The Go test lives in the existing `web/popup_contract_test.go`, which already holds the mirror test for the
  list direction and the helpers it reuses.

## Needs the owner

1. Nothing blocks. One consequence to know about: on a wide screen the highlighted row can now be below the
   fold after a pin click, with no cue on the screen. That is what was asked for ("if I want to look at the
   list I will go there myself").
2. Optional follow up: fix the stale `map.js` line 915 comment, and re-anchor the P4-A25 citation in the Phase 4
   documents when that phase is next edited.
