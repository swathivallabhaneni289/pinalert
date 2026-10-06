# Planner brief for Phase 4 (shared by the outline run and every per-plan run)

You are a gsd-planner subagent in a chunked planning run for Phase 4 of Pinalert. Read this whole file
first, then do the task named in your prompt. Where this brief and your prompt disagree, your prompt wins.
Your prompt says whether you are the OUTLINE run (writes only the outline) or a SINGLE-PLAN run (writes
exactly one PLAN.md).

<planning_context>
**Phase:** 04 (robustness-real-world-resilience)
**Phase directory:** .planning/phases/04-robustness-real-world-resilience
**Plan files:** `04-NN-PLAN.md` in that directory (NN is two digits). Outline file: `04-PLAN-OUTLINE.md`.
**Mode:** standard, chunked
**MVP_MODE:** true. Read `$HOME/.claude/gsd-core/references/planner-mvp-mode.md` and follow its vertical slice rules.
**WALKING_SKELETON:** false (not a new project; do not write SKELETON.md)
**Granularity:** standard
**TDD mode:** off (`workflow.tdd_mode` is false). Use `type: tdd` only where a pure logic plan clearly fits.
**Human verify mode:** end of phase. No `checkpoint:human-verify` tasks. Manual checks go in `<verify><human-check>`.
**Security:** enforcement is on, ASVS level 1, block on high. Every plan carries a `<threat_model>` block.

**Phase goal (verbatim from ROADMAP.md; every plan restates it under `## Phase Goal` with the three keywords in bold):**
Verbatim: As a person reading or reporting during an emergency, often on a weak phone connection, I want to keep reading, posting and confirming reports when the signal is bad, knowing abusive posts are screened out and that the site says plainly it is unofficial, so that the reporting loop keeps working and I am not misled about what Pinalert is.

Plan form (use this under `## Phase Goal` in every plan): **As a** person reading or reporting during an emergency, often on a weak phone connection, **I want to** keep reading, posting and confirming reports when the signal is bad, knowing abusive posts are screened out and that the site says plainly it is unofficial, **so that** the reporting loop keeps working and I am not misled about what Pinalert is.

**Phase requirement IDs (every ID must appear in some plan's `requirements` field):** ROBUST-01, ROBUST-02, ROBUST-03, ROBUST-04, ROBUST-08, ROBUST-09, ROBUST-10

**Project instructions:** read `./.claude/CLAUDE.md`. **Project skills:** `.claude/skills/sketch-findings-pinalert/SKILL.md`
(popup structure and trust block; load it with Skill("sketch-findings-pinalert") for any plan that touches the popup, feed row,
Flag control or trust block).

<files_to_read>
Always (every run):
- .planning/phases/04-robustness-real-world-resilience/04-CONTEXT.md (locked decisions D-01 to D-24 and derived rules R-01 to R-10)
- .planning/phases/04-robustness-real-world-resilience/04-RESEARCH.md (technical research; its "Open Questions" are resolved below)
- .planning/phases/04-robustness-real-world-resilience/04-PATTERNS.md (analog files and excerpts; "Shared Patterns" and "Landmines" sections matter to every plan)
- .planning/phases/04-robustness-real-world-resilience/04-VALIDATION.md (required tests V-01 to V-24 and the canonical test commands)
- .planning/ROADMAP.md (Phase 4 entry only), .planning/REQUIREMENTS.md (ROBUST ids only), .planning/STATE.md (blockers section only)
OUTLINE run additionally: read ALL of .planning/phases/04-robustness-real-world-resilience/04-UI-SPEC.md (246 KB, approved; paged reads)
and, from the Phase 3 outline `.planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-PLAN-OUTLINE.md`, the frontmatter,
the intro, "MVP framing", "Sizing", one "Sources per plan" entry, and "Cross-plan contracts" through "Pinned test names" (grep the headings;
that file is the structural model for the Phase 4 outline and is the source for the symbols Phase 3 will create).
SINGLE-PLAN run additionally: `04-PLAN-OUTLINE.md` (the sections "Cross-plan contracts" and "Coverage map" in full, plus your own entries
in the plan table, "MVP framing", "Sizing" and "Sources per plan"), then ONLY the sources the outline lists for your plan, by heading.
Never read the 246 KB UI-SPEC whole in a single-plan run: read the sections the outline names.
</files_to_read>
</planning_context>

## Precedence when sources disagree (highest first)
1. CONTEXT D-01 to D-24 (locked) and the owner choices of 2026-10-05 recorded in UI-SPEC "Decisions carried forward".
2. UI-SPEC (approved contract) for everything visual or interactive, including its amendments P4-A1 and up.
3. RESEARCH for engineering approach, with its Open Questions resolved by the next section.
4. PATTERNS for which analog to copy.
ROADMAP.md and REQUIREMENTS.md were updated on 2026-10-06 (commit e82fde6, the owner approved) to match CONTEXT; do not edit them, STATE.md, or any file other than your own output.

## Orchestrator decisions on the research's open questions (defaults; the owner can veto, so keep each one separable)
1. A text reject, a 400 and an idempotent replay do not consume the report allowance (reserve after validation, cancel on reject).
2. Compression: add chi `middleware.Compress` (no new dependency) and test the Low data byte budget in raw bytes. Put it in its own small task.
3. The D-07 fixture runs on provider scores recorded once by the owner against the pinned snapshot `omni-moderation-2024-09-26`. Build the
   recorder command and the `Decide` fixture test on recorded scores; recording is an end of phase human check (zero balance key).
4. Moderator reach: no link anywhere in the UI; the owner bookmarks `/moderate` (UI-SPEC open item 20 default).
5. Retention: no retention job; the Policy page states the real retention honestly and offers deletion through the contact line.
6. JS logic test: include ONE `node --test` file for `queue.js` with a hand written fake IndexedDB (flush order, never discard, wrong account
   never sent), in its own task marked optional. No package, no build step. Check `.github/workflows/ci.yml` before wiring it into CI.
7. Migration numbers are never written literally in a plan. Say "the next free number at execution time" and anchor on file names by purpose.
8. UI-SPEC "Open items for the planner" defaults apply as written unless a plan has a concrete reason, which it must record.

## Verified corrections (a read only code check on 2026-10-05; RESEARCH, PATTERNS and VALIDATION were corrected in place)
Read the section "Corrections after the code check" at the top of 04-RESEARCH.md and 04-PATTERNS.md first. These change what a plan must say:
1. **Rate limit refunds.** `Reservation.Cancel()` and `CancelAt(later t)` are silent no-ops for a zero delay grant (x/time v0.16.0). `PerKey` captures `now` once, calls `ReserveN(now, 1)`, and refunds with `CancelAt(now)` wrapped in `sync.Once`. Every refund test advances the injected clock between Reserve and Cancel and asserts the next Reserve is admitted.
2. **Moderator routes** are registered with `r.Handle` (all methods) inside the `requireModerator` group, never `r.Get` or `r.Post`, so a wrong method cannot leak the route through chi's 405 plus `Allow`. The byte equality test is a table that includes wrong methods. (Orchestrator decision: the scoped fix, not a router wide `MethodNotAllowed` change.)
3. **Idle windows** are per bucket and at least burst times refill interval (other reports 120 minutes, serious 25, Flag 20, votes 5, cooldown 30 seconds, submit attempt 2, IP backstop 20). The 10 minute default of `PerIP` is wrong for the report buckets.
4. **`ReportService.Submit` has exactly one caller** (`handlers.SubmitReport`); tests and seeds insert with raw SQL. Add `SubmitChecked` for clarity; the plan decides whether `Submit` stays as a thin wrapper or is deleted.
5. **`ReportExtrasForReports`** goes on `Querier` and `AuthQuerier` and their two fakes only; `CastVote` reads moderation through the extended `ReportVoteContext` row.
6. **The `Resolve` caller guard test** strips comments (`feed.go` line 47 mentions `Resolve()` in a comment), requires `Moderation` in code for every caller, and allowlists `reliability.go` by name (Phase 3 plan 03-03 has one neutral call) while asserting it still has exactly one call.
7. **`low.js` must subscribe to the shared store** (`Pinalert.subscribe`) and call `Pinalert.setCenter` before any vote, because `votes.js` calls `Pinalert.fetchReports()` after every successful vote and the Low data page loads `votes.js` but not `feed.js`.
8. **The all files markup sink test needs a `//` aware `stripJSComments`** (`stripCSSComments` strips blocks only; `map.js` line 11 mentions `innerHTML` in a comment). Reword that comment too.
9. **`.top-chrome` does not exist today.** It is the Phase 4 wrapper (UI-SPEC P4-A1) that moves `.app-header` inside it; the Phase 3.2 bell sits next to the account icon.
10. **`TestVoteBlockHiddenGuard` covers only `.vote-*` selectors**, so Phase 4 adds its own walker test over its new selectors (`.flag-control`, `.queue-strip`, `.notice-card`, the Low data and moderator classes).
11. **Existing tests that Phase 4 amends, not creates:** `TestUserVisibleCopyUsesPlainPunctuation`, `TestSwaggerSpecCoversRoutes`, `TestNearbyBatchesVoteReadsOnce`, `TestMigrationsEmbedded`.
12. **Phase 3 status:** all 20 Phase 3 plan files are final: they were revised after two integration review rounds and the phase was closed at commit b2d71bb (2026-10-05 18:50). None of Phases 3, 3.1 or 3.2 is built. Where a Phase 3 plan file on disk differs from the RESEARCH collisions table, the plan file wins.
13. **UI contract code facts:** eleven minor points about today code are listed in the subsection UI contract code facts at the top of 04-RESEARCH.md (no 429 exists on the report or vote routes today, the README has no launch checklist heading, the shared Written formatter lives in app.js, Low data search treats any non 200 as unavailable, and so on). The approved UI-SPEC is not edited; where it differs from today code, that list wins.

## Hard rules for every plan
- **Scope is fixed.** Cover D-01 to D-24, R-01 to R-10 and the whole UI-SPEC at full fidelity. NEVER cut, defer or thin anything yourself. The cut
  order in CONTEXT is only a suggestion that needs the owner's confirmation. If the work cannot fit the sizing thresholds, say so in the outline
  and return `## PHASE SPLIT RECOMMENDED` instead of dropping scope.
- **Every locked decision id must be cited** in some plan's `must_haves.truths` as "(D-02, R-01)" at the end of the truth. The decision coverage
  gate fails the whole run otherwise. Cite R-01 to R-10 and UI-SPEC amendment ids (P4-A1 and up) the same way.
- **Phases 3, 3.1 and 3.2 are not built yet but run before this phase.** Plans must be executable after they land. Every plan that relies on a
  symbol, file or test that a Phase 3, 3.1 or 3.2 plan creates gets a `## Cross-phase prerequisites` section listing each one as "produced by
  Phase 3 plan 03-NN, not built yet" (or "Phase 3.1 or 3.2, no plan yet: only what ROADMAP and UI-SPEC "Cross phase hand offs" state"), and its
  first task re-greps the built code for each before editing and stops with a report if one is absent or renamed (never invent it). The Phase 3 plans
  are final (see verified correction 12): read them only, cite by heading and symbol, never copy line numbers.
- **Plan ids and files.** `04-NN-PLAN.md`. Plans in the same wave never share a file (the outline's File ownership table decides). Only one plan per
  wave runs `make sqlc` or `make swag`, and that plan commits the regenerated files because CI has neither tool. Never edit generated files by hand.
- **Disposable database, one per plan.** Every verify command that touches Postgres uses the plan's own database `p4_04NN` in the canonical form of
  04-VALIDATION.md (`dropdb --if-exists p4_04NN; createdb p4_04NN && DATABASE_URL=... go test ... -count=1 -p 1; rc=$?; dropdb --if-exists p4_04NN; exit $rc`).
  The quick run is `env -u DATABASE_URL go test ... -count=1`. NEVER `pinalert_test` or `pinalert` (the test suite truncates `pinalert_test`, which holds
  the owner's manual testing data). Any run or manual step uses `PORT=8792`, `BASE_URL=http://127.0.0.1:8792`, `ENV=development` and a disposable database.
  NEVER ports 8080 or 8090.
- **Verify hygiene.** No `|| true` or `2>/dev/null || echo` feeding a comparison. Any count in a grep gate is measured first and its provenance stated.
  No RESEARCH or UI-SPEC line numbers in verify commands: anchor on function names, headings and strings. After a `-run`, a command must fail if it runs zero tests.
- **No dashes** (U+2014, U+2013) in prose, docs, server strings, templates, or any NEW JS or CSS file (comments included). Existing files keep their
  existing comment dashes untouched. Hyphens in identifiers, file names, slugs and CSS class names are fine.
- **No new Go modules, no npm packages, no JS build step, no images or screenshots saved in the repo.**
- **Naming.** The Low data view is never called "light" or "lite" in any name, file, route, class or string. The author facing and reader facing names
  are the UI-SPEC Vocabulary table's (for example "Flag this report", "Waiting for a text check", "Removed after review", "Open full version").
- **Wire contracts** (response fields, enums, route names, cookie names, error bodies) are decided ONCE in the outline's cross-plan contracts, then used
  verbatim by every plan. Do not mint a second name for the same thing.
- **Tests that change.** Every existing test the UI-SPEC "Tests that change" list or the research names (for example `TestCastVoteCheckOrder` once
  Phase 3 adds it, the `createVoteBlock(reportId)` anchored tests, `TestActivityPageLinksBackToTheMap`, `TestToastHasExactlyOneImplementation`) is assigned to exactly
  one plan, which edits it in the same plan that changes the behavior it pins.
- **Final plan** holds the documentation updates (README launch checklist, a new `docs/MODERATION.md`, the UI-SPEC "Docs to update" items, the
  sketch findings popup-structure "Actions" row), regeneration checks, and the owner's UAT using the UI-SPEC manual checks script. No `checkpoint:human-verify`.

## Required elements of every PLAN.md
- Frontmatter and XML task format as the planner agent specifies (`phase`, `plan`, `type`, `wave`, `depends_on`, `files_modified`, `autonomous`, `requirements`, `must_haves`).
- `## Phase Goal` with the bold user story, then the plan's own objective as one user visible or test observable step (MVP mode: after this plan something
  can be done or proven that could not be before).
- `## Cross-phase prerequisites` (omit only if the plan relies on nothing from Phases 3, 3.1 or 3.2).
- `## Artifacts this phase produces`: every new file, route, function, struct field, constant, query, table, test name, CSS custom property and data attribute.
- `<threat_model>` at ASVS level 1 reusing the outline's threat ids (never mint ids per plan). No package installs, so no supply chain row.
- Every task has `<read_first>` (including the file being modified), `<action>` with concrete identifiers and no fenced code or full implementations, and
  `<acceptance_criteria>` that are source assertions, behavior assertions or test commands (never "looks correct").
- `must_haves`: `truths` (positive, observable, with cited ids), `artifacts`, `key_links`, and `prohibitions` for must-NOT items.

<deep_work_rules>
## Anti-Shallow Execution Rules (MANDATORY)

Every task MUST include these fields. They are NOT optional:

1. **`<read_first>`**: files the executor MUST read before touching anything. Always include the file being modified, any source of truth file from CONTEXT.md
   (reference implementations, existing patterns, config files, schemas), and any file whose patterns, signatures, types or conventions must be replicated or respected.
2. **`<acceptance_criteria>`**: verifiable conditions. Every criterion must be checkable as a source assertion, behavior assertion, test command or CLI output.
   NEVER use subjective language ("looks correct", "properly configured", "consistent with"). Include exact strings, patterns, values, command outputs or observable behavior.
3. **`<action>`**: must include CONCRETE values, not references. NEVER say "align X with Y", "match X to Y" or "update to be consistent" without the exact target state.
   Include concrete identifiers and values: config keys, function signatures, SQL table names, class names, import paths, env vars, endpoint paths. Do not include
   full file contents, fenced code blocks or complete implementations in `<action>`. The executor uses `<read_first>` files for current implementation details.
</deep_work_rules>

<quality_gate>
- [ ] PLAN.md files created in the phase directory, each with valid frontmatter
- [ ] Tasks are specific and actionable; every task has `<read_first>` and `<acceptance_criteria>`; every `<action>` has concrete identifiers and no fenced code
- [ ] Dependencies correctly identified; waves assigned for parallel execution; same wave plans share no file
- [ ] must_haves derived from the phase goal; every cited decision id present; every ROBUST id present in some plan's `requirements`
- [ ] Every PLAN.md includes "Artifacts this phase produces", `## Phase Goal`, `<threat_model>`, and `## Cross-phase prerequisites` where needed
</quality_gate>

<downstream_consumer>
Output is consumed by /gsd-execute-phase and by the gsd-plan-checker. Plans need frontmatter (wave, depends_on, files_modified, autonomous), tasks in XML format with
read_first and acceptance_criteria on every task, verification criteria, must_haves for goal backward verification, and the "Artifacts this phase produces" section
(the source grounding pass reads it to exclude newly created symbols from drift checks).
</downstream_consumer>

## Repo safety rules (bind every agent)
- Read only except for writing your own output file(s). Do not run git add, commit, stash or checkout. Do not edit ROADMAP.md, REQUIREMENTS.md, STATE.md or any source file.
- NEVER run the Go test suite and NEVER connect to `pinalert_test` or `pinalert`. Start no server. Never use ports 8080 or 8090.
- Save no images. Use no em dash or en dash in anything you write.
