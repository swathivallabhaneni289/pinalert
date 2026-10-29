---
phase: 09-followable-reporter-profiles
plan: 29
subsystem: service
tags: [trust, ratings, copy, go]
requires:
  - phase: 09-13
    provides: ReporterSignal Raters and TappedHalves, MinRatersForStars
  - phase: 09-14
    provides: ratings wired into every tag reader
provides:
  - RatingsIncludedSentence, the P9-C29 line
  - the reporter fact detail ending with that line when ratings applied
affects: [09-26]
tech-stack:
  added: []
  patterns:
    - "one server written trust object, so every surface shows the same words"
key-files:
  created:
    - internal/service/trustfacts_ratings_test.go
  modified:
    - internal/service/trustcopy.go
    - internal/service/trustfacts.go
key-decisions:
  - "The line shows exactly when TappedHalves is set and Raters is at least MinRatersForStars, which includes a private profile whose ratings only lowered the points (D-22)."
  - "The signal's own Sentence is unchanged; only the reporter fact detail carries the line."
metrics:
  tasks: 1
  files: 3
completed: 2026-10-29
status: complete
---

# Phase 9 Plan 29: Ratings line in the trust panel Summary

The trust panel's reporter row now ends with "Includes ratings by N people." whenever tapped ratings are part of the reporter's tag, so a Reliable reporter tag never hides that part of it rests on ratings (review W6, UI-SPEC P9-C29).

## What was built

- `RatingsIncludedSentence(raters int)` in `internal/service/trustcopy.go`, built from the existing `peoplePhrase` helper ("Includes ratings by 12 people.").
- `reporterFact` in `internal/service/trustfacts.go` appends the line, after one space, to the row's Detail when `r.TappedHalves != nil && r.Raters >= MinRatersForStars`. A small `joinSentences` helper keeps a row from starting with a space. Title, Tone and ScoreDetail are untouched.
- The line is part of the one server built `Trust.Facts`, so the feed, the vote answer, Past, Focus, Activity and the report page all show it, and `trust.js` computes nothing and holds no new numeral.

## Task and commit

1. 4cc97308 feat(09-29): say in the trust panel when ratings are part of a reporter's tag

## Tests

- New, each seen with `-v`: `TestReporterFactSaysRatingsIncluded` (5, 12 and 150 raters; the exact fact; the closed character set check), `TestReporterFactWithoutRatingsUnchanged` (ratings not applied, 4 raters, a stray tap count under the minimum, a New reporter, a private profile whose ratings applied shows the line, and the Title, Tone and ScoreDetail stay equal with and without the line).
- Existing `TestTrustFacts*` pass unchanged. There is no test named `TestTrustWording`; the wording checks run through `p30Check` and `p31Check`, which the new test also calls.
- Full gate on scratch database `p9_29_a`: `go test ./... -p 1 -count=1` is ok for every package, including `TestRatedReporterSameEverywhere`, `TestFeedTrustMatchesCastVoteTrust`, `TestTrustResponseOmitsIdentifiers`, `TestFeedSignalsMatchCastVoteSignals` and `TestTrustModuleHasNoThresholdsOrSinks`. `go build ./...` and `go vet ./internal/service/` are clean. The scratch database was dropped.

## Deviations from Plan

None. The plan executed as written. The extra `joinSentences` helper is a small safety addition inside the same task.

## Known Stubs

None.

## Threat Flags

None. T-09-102 is mitigated by the new line.

## Notes

- The push worker (09-26) is the one reader still to receive ratings; once it does, any trust object it builds carries the same line with no further change here.
- No STATE.md, ROADMAP.md or REQUIREMENTS.md change was made, by instruction.

## Self-Check: PASSED

- internal/service/trustfacts_ratings_test.go found; commit 4cc97308 found.
