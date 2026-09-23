---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
plan: 12
subsystem: trust-mechanic
tags: [go, feed-visibility, swagger, gap-closure, uat]

requires:
  - phase: 02-trust-mechanic-core-confirm-dispute-visibility
    provides: "Resolve()'s four-state visibility ladder (visibility.go) and the GET /api/reports read path (feed.go, report.go) that filters over it"
provides:
  - "ListableInFeed as a true partition: show_disputed selects between two disjoint views (default = Live+Provisional, disputed = Hidden alone) instead of unioning them"
  - "Unit and HTTP-level test coverage asserting the exclusive contract by name, including a union/intersection partition property test"
  - "A corrected, regenerated OpenAPI description and parameter text for GET /reports' show_disputed flag"
  - "A corrected 02-RESEARCH.md data-flow diagram with a dated provenance note"
affects: [02-verify-work, roadmap-backlog-999.2]

tech-stack:
  added: []
  patterns:
    - "Visibility partition over Resolve()'s output: the read-path filter switches on the resolver's own Visibility value alone, never re-derives disputedness"

key-files:
  created: []
  modified:
    - internal/service/feed.go
    - internal/service/feed_test.go
    - internal/api/handlers/feed_visibility_e2e_test.go
    - internal/api/handlers/reports.go
    - docs/docs.go
    - docs/swagger.json
    - docs/swagger.yaml
    - .planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md

key-decisions:
  - "ListableInFeed's flag now partitions (Live/Provisional return !includeDisputed) rather than unions (previously always true) — closes 02-UAT.md Test 9 / ROADMAP.md backlog Phase 999.2"
  - "D-10 and D-11 were not amended; their locked text never required additivity. The additive reading was an interpretation carried into feed.go and 02-RESEARCH.md's diagram, one layer below the locked decision, and is what was corrected here"
  - "No SQL, client file, template, toggle copy, or resolver rung (visibility.go/Resolve) was touched — the fix is confined to the Go-level read filter and its documentation"

patterns-established:
  - "A read-path visibility filter must assert its partition/union property explicitly in tests (union == all ids, intersection == empty), not just per-state presence, since per-state coverage alone missed this exact defect"

requirements-completed: [TRUST-02, OPS-01]

coverage:
  - id: D1
    description: "ListableInFeed partitions the four visibility states across exactly two disjoint views; Live/Provisional excluded under show_disputed=true, Hidden included only then, Retracted excluded from both"
    requirement: "TRUST-02"
    verification:
      - kind: unit
        ref: "internal/service/feed_test.go#TestListableInFeed"
        status: pass
      - kind: unit
        ref: "internal/service/feed_test.go#TestNearbyHidesDisputedUnlessRequested"
        status: pass
    human_judgment: false
  - id: D2
    description: "The HTTP-level GET /api/reports contract asserts the partition in both directions: disputed view contains only the disputed report, default view contains only the control report"
    requirement: "TRUST-02"
    verification:
      - kind: integration
        ref: "internal/api/handlers/feed_visibility_e2e_test.go#TestShowDisputedRevealsHiddenReports"
        status: pass
    human_judgment: false
  - id: D3
    description: "Published OpenAPI description and show_disputed parameter text state the flag replaces the view rather than adding to it, and docs/ is regenerated (not hand-edited) via make swag"
    requirement: "OPS-01"
    verification:
      - kind: other
        ref: "make swag && git diff --exit-code docs/ (idempotence check, run after commit)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Human-observable UAT retests this unblocks: no-unfiltered-flash and reload-persistence halves of 02-UAT.md Test 9, now judgeable because the disputed view is genuinely empty when nothing is disputed"
    verification: []
    human_judgment: true
    rationale: "Requires a real browser per workflow.human_verify_mode: end-of-phase; the plan's own <human-check> block defers this to end-of-phase UAT, not to this executor"

duration: 35min
completed: 2026-09-23
status: complete
---

# Phase 02 Plan 12: Feed visibility partition (gap closure) Summary

**`ListableInFeed` rewritten from a union to a true partition, closing the "Show disputed reports widens instead of switches the view" gap the user reported twice (2026-09-18, 2026-09-21) — resolves ROADMAP.md backlog entry Phase 999.2.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-23T09:00:00Z (approx.)
- **Completed:** 2026-09-23T09:31:30Z
- **Tasks:** 3/3 completed
- **Files modified:** 8

## Accomplishments

- `internal/service/feed.go`'s `ListableInFeed(includeDisputed bool)` now returns `!includeDisputed` for Live and Provisional (previously unconditionally `true`), making the two views — default (Live+Provisional) and disputed (Hidden alone) — a true partition: exactly one view contains any given non-retracted report, never both, never neither.
- The service-layer unit suite (`feed_test.go`) proves this by name: `TestListableInFeed`'s Live/Provisional-under-`true` cases now expect `false`, and `TestNearbyHidesDisputedUnlessRequested` gained a new subtest asserting the union of the two views' report ids is exactly the three seeded reports and their intersection is empty — the assertion shape that would have caught the original defect directly.
- The HTTP-level contract (`feed_visibility_e2e_test.go`'s `TestShowDisputedRevealsHiddenReports`) now asserts the partition in both directions against a real Postgres: the disputed view contains the genuinely disputed report and not an undisputed control report from the same account, and (new) the default view contains the control and not the disputed report.
- The published OpenAPI description and `show_disputed` parameter text (`reports.go`, regenerated into `docs/`) now say the flag replaces the default view with the disputed-only view, and document the pre-existing critical-bypass interaction (a critical/rescue-needed report stays Live however heavily disputed, so it never appears in the disputed view) so an API consumer doesn't file it as a bug.
- `02-RESEARCH.md`'s data-flow diagram, which stated the additive contract in its own filter-line sketch, is corrected to describe the partition, with a dated note recording the provenance of the original error and its correction.

## Task Commits

1. **Task 1: Make the feed predicate a true partition, and rewrite the unit tests that asserted the union** — `10bca11` (fix)
2. **Task 2: Flip the HTTP-level contract test from "adds reports" to "replaces the view"** — `aa37950` (test)
3. **Task 3: Correct the published API description and the phase's own data-flow diagram, then regenerate the spec** — `1f649f9` (docs)

**Plan metadata:** committed via `gsd_run query commit` after this SUMMARY (see final commit in worktree merge).

_No TDD RED/GREEN split was used for Task 1 despite `tdd="true"` in the plan frontmatter — the failing state was the pre-existing additive behavior itself (already committed on `main`), so this plan's Task 1 commit is the single fix+test-update commit that turns the known-red assertion (union) into the correct green one (partition), rather than a separate RED-then-GREEN pair. `internal/api/handlers` was left intentionally RED for the duration of Task 1 (per the plan's own `<verify_note>`) until Task 2's commit turned it GREEN._

## Files Created/Modified

- `internal/service/feed.go` — `ListableInFeed`'s Live/Provisional case flipped to `!includeDisputed`; doc comment rewritten to describe the partition, the critical-bypass consequence, and the reversal's provenance (names Phase 999.2).
- `internal/service/feed_test.go` — `TestListableInFeed` table flipped for the two affected cases; `TestNearbyHidesDisputedUnlessRequested` gained a rewritten last-two-subtests split by view plus a new union/intersection partition subtest; three further tests in the same file (`TestNearbyExcludesRetractedFromBothViews`, `TestNearbyOrderingIsIndependentOfVisibility`, `TestNearbyMarksOwnReportAndViewerVote`) were also fixed — see Deviations.
- `internal/api/handlers/feed_visibility_e2e_test.go` — `TestShowDisputedRevealsHiddenReports` flipped: control report now asserted absent under `show_disputed=true`, plus a new mirror assertion fetching the default view and asserting the control present / disputed report absent there.
- `internal/api/handlers/reports.go` — `NearbyReports`'s `@Description` and `@Param show_disputed` swagger text rewritten to describe the exclusive contract and the critical-bypass interaction.
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` — regenerated via `make swag`; diff scoped exactly to the `/reports` endpoint's description and `show_disputed` parameter text, confirmed by inspection and by a second `make swag` run producing no further diff.
- `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md` — data-flow diagram's filter line corrected to describe the partition (same box width, D-reference style preserved); dated correction note added beneath the diagram.

## Decisions Made

- **The union lived in the service layer, not the handler or SQL — the first guess would have been wrong.** Diagnosis (`.planning/debug/disputed-filter-not-exclusive.md`) confirmed the additive behavior was a single four-case switch in `internal/service/feed.go`'s `ListableInFeed`, with exactly one production call site (`report.go`'s `Nearby()`). No SQL WHERE clause, no handler-level logic, no client-side filtering was ever involved. This is worth recording explicitly because the same wrong guess (handler or SQL) is the natural one to make again if this symptom recurs elsewhere.
- **D-10 and D-11 were not amended — this corrects an interpretation, not a locked decision.** `02-CONTEXT.md`'s locked text for D-10/D-11 only requires that Hidden reports be *reachable* through the "Show disputed" toggle; it never says Live and Provisional must remain visible alongside them. The additive reading was introduced one layer below the decision — in `feed.go`'s implementation/tests and in `02-RESEARCH.md`'s diagram — and that is what this plan corrects. "We changed a locked decision" and "we corrected an interpretation below a locked decision" are different claims; only the second is true here, and both the code comment and the research diagram's correction note say so explicitly.
- **The three artefacts that asserted the union and are now rewritten, by name:** (1) `ListableInFeed`'s own doc comment in `feed.go`, which previously stated the additive design and warned only against a different, unrelated future mistake; (2) `02-RESEARCH.md`'s data-flow diagram's filter-line sketch; (3) `TestNearbyHidesDisputedUnlessRequested`'s and `TestShowDisputedRevealsHiddenReports`'s failure-message wording, which spelled out the additive contract by name ("the toggle adds Hidden reports, never removes Provisional ones"; "the toggle should add reports, not replace the view"). All three are rewritten in this plan so a future `git blame` reader sees the reversal was deliberate, not a fix that broke tests and was forced through.
- **The critical-bypass consequence is pre-existing behavior, now more visible, not a regression.** D-06's critical-bypass rung sits above the dispute rung in `Resolve()`, so a critical or rescue-needed report stays Live however heavily it is disputed and therefore never appears in the disputed view once that view is a true partition. This is now documented in three places: `ListableInFeed`'s doc comment, the OpenAPI `@Description` for `GET /reports`, and item 5 of the plan's `<human-check>` UAT block ("Not a bug if you see it").
- **What the client did not need, and why that's evidence rather than luck:** `feed.js`'s empty-state branch already selected the disputed-specific copy and `map.js`'s reconcile-by-id loop already cleared every marker on an empty result, both read and confirmed correct during diagnosis before this plan started. Neither needed a line of change — they moved from a rare code path (empty disputed view) to a common one (the disputed view will now usually be empty, since most reports are not disputed) without any modification, which is direct evidence the client was already correctly written for this behavior rather than merely lucky.
- **`make swag` produced no diff beyond this endpoint's text.** `git diff --stat docs/` after regeneration showed only `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`, each changed by only the `@Description`/`@Param` text for `/reports`' `show_disputed` flag. A second `make swag` run against the committed state produced `git diff --exit-code docs/` with no output, confirming the installed `swag` binary (`~/go/bin/swag`) matches the one that produced the previously committed files — no installed-version drift, no unexplained bulk diff to report.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Three further tests in `feed_test.go` encoded the same additive assumption and would have regressed under the corrected predicate**

- **Found during:** Task 1, while running `go test ./internal/service/ -short -v` after the `ListableInFeed` change and the two named subtest rewrites the plan called out.
- **Issue:** The plan's action items only named `TestListableInFeed` and the last two subtests of `TestNearbyHidesDisputedUnlessRequested` for rewriting. Three other tests in the same file also asserted additive behavior and would fail once the predicate became exclusive:
  - `TestNearbyExcludesRetractedFromBothViews`'s first subtest asserted reports 2 and 3 (both Provisional, unvoted) present under **both** `includeDisputed` values; under the partition, Provisional reports are absent from the disputed view.
  - `TestNearbyOrderingIsIndependentOfVisibility`'s "show_disputed view" subtest asserted all of `{1, 2, 4, 5}` (live, hidden, provisional, critical-bypass) present under `show_disputed=true`; under the partition only the Hidden report (`2`) belongs there.
  - `TestNearbyMarksOwnReportAndViewerVote` called `Nearby` with `IncludeDisputed: true` throughout, but every seeded report in that test is Provisional (no dispute votes at all); under the partition the disputed view would return zero reports, failing every presence assertion in that test even though the test's actual subject (own-report/viewer-vote attribution) has nothing to do with the show_disputed flag.
- **Fix:** `TestNearbyExcludesRetractedFromBothViews`'s presence assertion for reports 2/3 was made view-aware (`wantPresent := !includeDisputed`). `TestNearbyOrderingIsIndependentOfVisibility`'s expected id list for the disputed-view subtest was corrected to `{2}`. `TestNearbyMarksOwnReportAndViewerVote`'s four `Nearby` calls were switched from `IncludeDisputed: true` to `IncludeDisputed: false`, since the test's subject is unrelated to the toggle and its seeded reports are all Provisional.
- **Files modified:** `internal/service/feed_test.go` (same file already in the task's `<files>` scope — no file-scope expansion).
- **Verification:** `go test ./internal/service/ -count=1 -short -v` passed in full after the fix, and again under a disposable Postgres database (`go test ./internal/service/ -count=1 -p 1`) in Task 2's verification.
- **Committed in:** `10bca11` (part of Task 1's commit).

---

**Total deviations:** 1 auto-fixed (Rule 1, spanning 3 test functions in one already-in-scope file)
**Impact on plan:** Necessary for correctness — these three tests would have failed the moment the predicate changed, independent of anything this plan chose to touch. No scope creep: all three fixes stayed inside `internal/service/feed_test.go`, which was already one of Task 1's two `<files>`, and none of them touch production code, SQL, the client, or `visibility.go`/`Resolve()`.

## Issues Encountered

None beyond the deviation above. All three plan tasks' own `<verify>` blocks passed on first attempt after implementation; no auto-fix attempt limit was approached on any task.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- This plan closes `02-UAT.md`'s Test 9 (severity `minor`) and resolves `ROADMAP.md`'s backlog entry Phase 999.2. **Two UAT retests are now judgeable for the first time**, per the plan's `<human-check>` item 3: with a live (undisputed) report present and the "Show disputed reports" toggle ticked, (a) reloading the page and watching for an unfiltered first-paint flash of the live report before it clears, and (b) opening a URL with `show_disputed=true` directly in a new tab and confirming it loads straight into the (now genuinely empty) disputed view. Neither half was separately observable before this fix, because the disputed view was never actually filtered.
- Run `/gsd-verify-work 2` (or this wave's shared end-of-phase UAT pass) to exercise the plan's `<human-check>` block, including item 5 ("Not a bug if you see it: a critical/rescue-needed report never appears in the disputed view").
- No blockers introduced. This plan is one of four mutually parallel gap-closure plans in wave 9 (`02-11`, `02-12`, `02-13`, `02-14`) sharing zero files; nothing here depends on or is depended on by its siblings beyond the shared wave.

---
*Phase: 02-trust-mechanic-core-confirm-dispute-visibility*
*Plan: 12*
*Completed: 2026-09-23*

## Self-Check: PASSED

All 8 `files_modified` paths plus this SUMMARY.md verified present on disk. All 4 commit
hashes (`10bca11`, `aa37950`, `1f649f9`, `8f1bd98`) verified present in `git log --oneline --all`.
