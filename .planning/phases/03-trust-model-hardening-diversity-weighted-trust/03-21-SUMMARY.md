---
phase: 03-trust-model-hardening-diversity-weighted-trust
plan: 21
subsystem: ui
tags: [ui-spec, amendments, trust-block, sketch-003, variant-h, docs, gap-closure]
requires:
  - phase: 03-trust-model-hardening-diversity-weighted-trust
    provides: the A1 to A74 contract and the plan 03-26 focus fixes
provides:
  - Amendments A75 to A84 in 03-UI-SPEC.md, the locked record of sketch 003 variant H
  - Owner decision Q6 and the gap closure round 4 Hand Off paragraph
  - Open item 2 (panel discoverability) marked resolved by A78
  - An amendment tag on each of the 28 body lines variant H makes false
  - A superseded note at the top of the sketch-findings trust-block reference
affects: [03-22, 03-23, 03-24, 03-UAT Gaps 1 to 3]
tech-stack:
  added: []
  patterns:
    - "Append only amendment tags: old text kept, tag added at the end of the line or last table cell"
key-files:
  created: []
  modified:
    - .planning/phases/03-trust-model-hardening-diversity-weighted-trust/03-UI-SPEC.md
    - .claude/skills/sketch-findings-pinalert/references/trust-block.md
key-decisions:
  - "Q6: owner chose sketch 003 variant H, recorded as amendments A75 to A84; Q1 to Q5 untouched"
  - "A84: wording stays deferred, UAT Gap 3 stays open"
requirements-completed: []
status: complete
---

# Phase 3 Plan 21: Record variant H in the UI contract Summary

The locked UI contract now carries sketch 003 variant H as amendments A75 to A84, owner decision Q6 and tags on every body line it makes false, with no earlier row or unnamed line changed.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| 1 | e5e9e68 | docs(03-21): record owner choice H as amendments A75 to A84 |
| 2 | c8552b0 | docs(03-21): tag the body lines variant H amends |

`git diff --numstat` of each commit:

- e5e9e68: `2 0` on `.claude/skills/sketch-findings-pinalert/references/trust-block.md`, `23 2` on `03-UI-SPEC.md`. The two deleted lines are the two lines the plan names (the "One row per change" intro and the Open item 2 line), each kept in full with text appended.
- c8552b0: `34 28` on `03-UI-SPEC.md` only. Every deleted line is an anchored line that reappears with its full old text plus a tag; the added lines are those 28 lines plus the four inserted pieces.

Each commit lists only its planned paths (checked with `git show --stat --format= HEAD`), and `git status --short` was clean afterwards.

## What changed

- **Task 1.** Intro line now says 84 rows and adds the "unchanged, deferred" resolution. Ten rows A75 to A84 follow A74, five cells each, with the locked item each one amends: D-18 (A75), A4 and Q3 (A76), A48 (A77), Open item 2 (A78), A22 (A79), A21 (A80), A26 (A81), D-09 and A50 (A82), the `:has()` floor (A83), D-19 (A84). Heading `### Q6 (owner choice 2026-10-09, sketch 003 variant H, A75)` was added after the Q5 factual note, with the owner's words copied from sketch 003 Design Question items 1 to 3. The round 4 Hand Off paragraph sits between the round 3 tests paragraph and "Docs to update". Open item 2 gained "Resolved 2026-10-09 (A78)". `trust-block.md` gained the superseded paragraph under its title; nothing else in it changed.
- **Task 2.** Tags on the 28 anchored lines (A75 to A84 ids, appended inside the last cell of table rows). Inserted: the Round 4 markup line, the Round 4 structure line, and the forced colours rows "Chevron (A78)" and "Trust toggle hover and press (A77)" after the "All mask icons" row.

## Verification

The plan's two verify blocks call `git` inside a shell construct, which the worktree sandbox refused to run. I ran git free copies instead, saved in the session scratchpad (`v1.sh` and `v2.sh`): the same awk and grep checks, reading the HEAD version of the spec from a file written by `git show HEAD:...` and the diff from a file written by `git diff -U0 HEAD`. Both checks were run before their commit.

- Task 1 copy: exit 0, all 13 checks PASS (only the two named deletions, A1 to A74 identical, Copywriting Contract region identical, Owner decisions region identical up to the Q6 heading, ten rows with five cells and the required locked items and tokens, intro, Open item 2, one Q6 heading, Hand Off paragraph, no em or en dash in either file, `trust-block.md` title kept and numstat `2 0`, note present).
- Task 2 copy: exit 0, all 10 checks PASS (only anchored deletions, old text kept as a prefix of an added line, each of the 28 anchors and 4 inserted lines found exactly once and tagged with an A75 to A84 id, all A rows identical, Copywriting Contract and Owner decisions regions identical, D-20 line and Trust toggle line content, no em or en dash).
- No Go test, database, server, browser or port was used. No test reads these files.

## Anchors

All anchors were found. No anchor was missing and none matched twice. The line numbers in the plan were not used; each anchor was re-grepped by its leading words.

## Deviations from Plan

None to the content. The only departure is procedural: the verify blocks were run as git free copies, as described above, because the sandbox refuses shell constructs that call git. No existing test assertion was changed (no test was touched).

## Owner item for phase close

ROADMAP Success Criterion 2 says "two distinct trust signals side by side"; TRUST-07 does not. A75 argues both still hold (both signals show on every report at once, as two separate pieces of plain text), but the owner should review that wording by hand at phase close.

## Live proof still owed (orchestrator, after merge)

Once 03-23 is merged, the browser harness measures variant H against A75 to A83 in Chromium and WebKit. This plan makes no code change.

## Known Stubs

None.

## Threat Flags

None. Docs only; T-03-21-01 to T-03-21-03 are mitigated by the wording of A75, A83 and by the deletion checks of the verifies.

## Self-Check: PASSED

- 03-UI-SPEC.md contains `| A84 |`, one Q6 heading and both Round 4 lines; trust-block.md contains the superseded note.
- Commits e5e9e68 and c8552b0 exist on the worktree branch.
- STATE.md, ROADMAP.md, REQUIREMENTS.md and 03-UAT.md are untouched.
