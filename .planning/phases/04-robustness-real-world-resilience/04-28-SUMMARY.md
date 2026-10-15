---
phase: 04-robustness-real-world-resilience
plan: 28
subsystem: testing
tags: [screening, moderation, fixture, d-07, go-test]
requires:
  - phase: 04-robustness-real-world-resilience
    provides: "04-02 Decide, IsGibberish and the two threshold constants; 04-04 recorder layout"
provides:
  - "genuine.json: 200 unrecorded genuine reports in five languages"
  - "abusive.json: 40 unrecorded abusive examples"
  - "TestScreeningFixture and three offline companion tests"
affects: [04-27, 04-29]
tech-stack:
  added: []
  patterns: ["recorded-score fixture judged offline in the inline check order"]
key-files:
  created:
    - internal/service/testdata/screening/genuine.json
    - internal/service/testdata/screening/abusive.json
    - internal/service/screening_fixture_test.go
  modified: []
key-decisions:
  - "Abusive reason lives only in the id (a-<reason>-<language>-NN), no extra JSON field"
  - "Snapshot name is a literal in the test, not imported from the provider client"
requirements-completed: []
duration: one session
completed: 2026-10-16
status: complete
---

# Phase 4 Plan 28: Screening fixture and offline test Summary

A 240 entry screening fixture (200 genuine reports and 40 abusive examples in en, hi, te, ta and hinglish) plus an offline test that judges it through the gibberish rule and `Decide`; it skips naming the recorder command until the owner records scores in 04-27.

## What was built

- `genuine.json`: 200 entries, 40 per language, 14 serious in each, every entry `expect: publish`, `model: ""`, `scores: {}`. Ids `g-<language>-NNN`.
- `abusive.json`: 40 entries: hateful 10, harassing 10, sexual 8, gibberish 12; serious per reason: hateful 5, harassing 5, sexual 3, gibberish 4 (at least 2 serious and 2 not in every reason; 5 serious hateful and 5 serious harassing entries wrap the abuse inside a disaster report). Per language entries: en 8, hinglish 8, hi 8, te 8, ta 8. Ids `a-<reason>-<language>-NN`. Unrecorded.
- `screening_fixture_test.go` (package `service_test`): `TestScreeningFixture` (skip while unrecorded, fail while part recorded, judge when recorded), `TestScreeningFixtureShape` (three subtests), `TestScreeningFixtureGibberishRule`, `TestScreeningFixtureGateCatchesFailures`.

## Step 0 results

- Task 1: `Decide`, `IsGibberish` and the seven named identifiers present in `screening.go`. `cmd/modrecord/main.go` was present and matched: tags in the order id, text, language, serious, expect, model, scores, and `SetEscapeHTML(false)`, `SetIndent("", "  ")`, `DisallowUnknownFields()` each once. The three new paths were absent.
- Task 3: the declaration grep for `screeningFixture` names printed nothing; Task 2's verify exited 0 before Part A.

## Coverage the shape test measured (genuine)

15 mobile number, 32 death or injury wording, 16 terse (25 runes or fewer), 5 long (300 runes or more), 14 all capitals, 42 address, 46 urgent, 3 link, 6 stretched letter entries. Per language: 40 entries, 14 serious, 3 phone and 2 to 4 terse entries in every language.

## Sizes for 04-27's recording time estimate

Total characters: genuine 17383, abusive 3021 (240 provider calls, one per entry).

## RED then GREEN

With the three judging helpers stubbed and the `p0428red` build constraint in place, `go test ./internal/service/ -tags p0428red -run TestScreeningFixtureGateCatchesFailures -count=1 -v` failed on three subtests ("got 0 problems, want 5", "want one problem naming syn-model-1, got []", "got 0 recorded and missing []"). The stubs were implemented, all four tests passed under the tag, the constraint was removed, and only the green state was committed.

Verify command (untagged) output, verbatim result lines:

```
--- PASS: TestScreeningFixtureShape (0.01s)
--- PASS: TestScreeningFixtureGibberishRule (0.00s)
--- PASS: TestScreeningFixtureGateCatchesFailures (0.00s)
    screening_fixture_test.go:611: none of the 240 fixture entries has recorded scores; record them once with MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord (plan 04-27)
--- SKIP: TestScreeningFixture (0.00s)
PASS
ok  	pinalert/internal/service	0.646s
```

Other runs: `go test ./internal/service/ -count=1` ok; `go test ./... -count=1` ok for every package; `go vet ./internal/service/` ok; `gofmt -l` empty; dash scan 0 in all three files. No database test was run (DATABASE_URL unset, none needed); the only SKIP is the by-design `TestScreeningFixture`.

## The nine readings applied

1. Abusive reason lives in the id. 2. "Withheld" offline means a serious `publish` entry that is rejected. 3. Every genuine text has two words and no 10 letter run; the test header names the one word and romanised Telugu and Tamil limits. 4. Byte exact layout proven by re-encode comparison. 5. Snapshot literal in the test. 6. Abusive set expected `reject` in all five languages. 7. Exact counts 40 per language and 40 abusive, floors lower. 8. Interim checks used jq (the throwaway builder lived in the session scratchpad; every text was authored by hand and ids were assigned by the builder). 9. RED observed and recorded, not committed.

## Deviations from Plan

None. The plan executed as written. Notes: `env -u DATABASE_URL` is refused by the sandbox, so the Go commands ran plain with `DATABASE_URL` confirmed unset in the shell (the plan's own owner rule 3 says the same). The abusive set has 5 serious hateful and 5 serious harassing entries instead of the target 4 (the floor is 2).

## Known limits and notes for 04-27

- Run the recorder from the repository root, then expect `TestScreeningFixture` to PASS; a failing id is a finding (a threshold change or owner decision), never an edit of the fixture.
- The Hindi, Telugu and Tamil texts were written by the model, not a fluent reader. The optional human skim in Task 2's human-check is still open. Any reword after recording must clear that entry's `model` and `scores`.
- The recorded path of `TestScreeningFixture` (all entries recorded) could not be run here because no scores exist; its judging helpers are covered by `TestScreeningFixtureGateCatchesFailures`.

## Commits

- 62b5bce test(04-28): add English and Hinglish genuine screening fixture
- f8fdc3e test(04-28): add Hindi, Telugu and Tamil genuine screening fixture
- 6662365 test(04-28): add abusive fixture and offline screening fixture test

## Self-Check: PASSED

All three files exist, the three commits are on this branch, each lists only its own paths, and the quick runs are green.
