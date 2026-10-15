---
phase: 04-robustness-real-world-resilience
plan: 02
subsystem: service
tags: [screening, moderation-policy, gibberish, server-copy, go]
requires: []
provides:
  - "internal/service/screening.go: Verdict, RejectReason, ScreenDecision, Decide, IsGibberish, RejectMessage, the three thresholds"
  - "internal/service/servercopy_test.go: assertServerCopy and serverCopyProblem, the shared closed character set checker"
affects: [04-11, 04-14, 04-22, 04-27, 04-28, phase-8-comments, phase-9-nicknames]
tech-stack:
  added: []
  patterns: ["closed enum plus Valid() drift guard", "ordered slice instead of map so results never depend on iteration order", "go/parser vocabulary guard over a policy file"]
key-files:
  created:
    - internal/service/screening.go
    - internal/service/screening_test.go
    - internal/service/servercopy_test.go
  modified: []
key-decisions:
  - "Decide never reads serious, so choosing Critical cannot skip screening (D-03, T-04-02)"
  - "Only hate, harassment and sexual groups can reject; violence, self harm, illicit and unknown keys reach borderline at most (D-03, D-04)"
  - "Thresholds start at reject 0.85 and borderline 0.40, both at or above, to be tuned by 04-27 on the pinned snapshot omni-moderation-2024-09-26"
  - "IsGibberish is exactly two rules, counted in runes (D-05, R-06)"
requirements-completed: []
duration: about 35 minutes
completed: 2026-10-16
status: complete
---

# Phase 4 Plan 02: Text screening policy Summary

**A pure score based text policy (`Decide`), a rune counted gibberish rule (`IsGibberish`) and four plain refusal messages (`RejectMessage`), with the shared `assertServerCopy` checker every later server message test will use.**

## What was built

- `Decide(scores, serious)`: walks the ordered `rejectGroups` slice (hateful, harassing, sexual), takes each group's highest key score, and rejects with the group of the highest score when it is at or above `RejectScoreThreshold` (ties go to the `RejectReasons` order). Otherwise any score in the whole map at or above `BorderlineScoreThreshold` gives borderline, else pass. `serious` is never read.
- `IsGibberish(text)`: one pass over runes. True for a run of one letter repeated `GibberishRunLength` (10) times, or for fewer than two words of two or more letters. A letter is `unicode.IsLetter` or `unicode.IsMark`, so Indic vowel signs and viramas stay inside words.
- `RejectMessage(reason)`: the three UI-SPEC abuse sentences verbatim and the owner's gibberish sentence of 2026-10-08; the empty string for anything else.
- `assertServerCopy` / `serverCopyProblem`: closed character set (ASCII letters, digits, space and the eight characters comma, period, apostrophe, question mark, colon, parentheses, equals), plus an empty string check, proven against dashes, curly quotes, an ellipsis, a hyphen and more.
- `TestScreeningHasNoReportVocabulary`: a `go/parser` guard proving the file imports only `strings`, `unicode`, `unicode/utf8` and carries no report vocabulary in identifiers or string literals (D-06).

## Step 0 collision check

The grep for every new name over `internal/service/` (excluding the three own files) printed nothing, before any file was created.

## TDD record

| Task | RED commit | RED failure | GREEN commit |
|------|-----------|-------------|--------------|
| 1 Decide | 6e5134e | TestDecide and TestDecideIgnoresSeverity failed on assertions (stub always passes) | 4e90894 |
| 2 IsGibberish | c6d5326 | TestGibberish and TestGibberishCountsRunes failed on the true rows (stub returns false) | 18a6ab0 |
| 3 messages and copy checker | c467792 | TestRejectMessages and TestServerCopyRejections failed on empty messages; the checker self test and the vocabulary guard passed already, as the plan predicted | 43a93e9 |

## Verification

- All ten named tests pass: TestDecide, TestDecideIgnoresSeverity, TestDecideThresholdsOrdered, TestScreeningEnumsAreClosed, TestGibberish, TestGibberishCountsRunes, TestScreeningHasNoReportVocabulary, TestRejectMessages, TestServerCopyRejections, TestServerCopyCheckerRejectsOutsideCharacters.
- `go test ./... -count=1` passes with `DATABASE_URL` unset (database tests skip by design; this plan has no database test and used no database).
- `go vet ./internal/service/` clean, `gofmt -l` on the three files prints nothing, the dash scan finds nothing in the final files, all three files are free of non ASCII letters except the Indic words in screening_test.go.
- Acceptance greps on screening.go (threshold values, letter definition, message sentences, signatures) each print 1.

## The six readings this plan applied

1. Letter only runs: only letter runes form a run, so ten exclamation marks, dots, repeated emoji, long space runs and phone numbers are never a run. **First owner veto item**; it amends the outline `IsGibberish` row (04-11 and 04-28 should read "a run of one letter").
2. Reject reason by highest group score; equal scores take the `RejectReasons` order.
3. Borderline reads every key in the map, including unknown keys; rejection stays limited to the six keys.
4. Both comparisons are "at or above".
5. `assertServerCopy` also refuses the empty string.
6. Closed enum extras `Verdicts`, `Verdict.Valid()` and `RejectReason.Valid()` were added beside the pinned names; no other exported name.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Verification commands without `env -u DATABASE_URL`**
- **Found during:** all tasks
- **Issue:** the owner rules say the sandbox refuses `env -u` wrappers.
- **Fix:** ran plain `go test` with `DATABASE_URL` confirmed unset in the shell instead; the effect is the same.

**2. [Rule 1 - Bug] Escapes decoded to literal characters by the file tool**
- **Found during:** Task 3 dash scan
- **Issue:** the Write tool turned the Go escapes for U+2014, U+2013, U+2019, U+201C, U+201D, U+2026, U+00A0 and U+0C05 into the literal characters in the test files.
- **Fix:** restored the Go escapes with sed before the Task 3 GREEN commit. The final files contain no long dash or short dash. The earlier RED commits c467792 (servercopy_test.go) and c6d5326 / 6e5134e (Telugu letters in screening_test.go) still carry the literal characters in history; the owner asked for no history rewrite, so they stay. Only the final tree matters for the dash rule.

**3. Commit trailer**
- The plan text names `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`; the executor rules file names `Claude Opus 5.5`. I followed the rules file (it states it overrides). All six task commits carry the Opus 5.5 trailer, so the plan's scope check that greps for Sonnet 5.5 would print 0. The orchestrator should confirm which trailer the owner wants.

## Known Stubs

None.

## Threat Flags

None. The file performs no I/O; T-04-02 is mitigated here by `TestDecideIgnoresSeverity`, and T-04-01's share by fixed messages held to the closed character set.

## Commits

- 6e5134e test(04-02): add failing Decide tests
- 4e90894 feat(04-02): implement Decide score policy
- c6d5326 test(04-02): add failing gibberish rule tests
- 18a6ab0 feat(04-02): implement gibberish rule in runes
- c467792 test(04-02): add failing refusal message and server copy tests
- 43a93e9 feat(04-02): add refusal messages and server copy checker

## Self-Check: PASSED

All three files exist, all six task commits exist on the worktree branch, and every named test, vet, gofmt and dash scan passed on the final tree.
