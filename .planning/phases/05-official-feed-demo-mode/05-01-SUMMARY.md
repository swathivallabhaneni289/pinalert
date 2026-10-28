---
phase: 05-official-feed-demo-mode
plan: 01
subsystem: areas
tags: [lgd, districts, point-in-polygon, go-embed, cap-geocodes]
requires: []
provides:
  - "internal/areas: Locate, ByCode, CellDistricts, Districts, States, StateByName, StateForSender, TrailingState"
  - "cmd/buildareas offline builder"
  - "internal/areas/data/districts.json.gz (345439 bytes)"
affects: [05-05, 05-08, 05-11]
tech-stack:
  added: []
  patterns: ["embedded derived data file decoded once with sync.Once, panic on build defect"]
key-files:
  created:
    - cmd/buildareas/main.go
    - cmd/buildareas/main_test.go
    - cmd/buildareas/testdata/three.geojsonl
    - internal/areas/areas.go
    - internal/areas/areas_test.go
    - internal/areas/states.go
    - internal/areas/states_test.go
    - internal/areas/data/districts.json.gz
  modified: []
key-decisions:
  - "Features with dist_lgd 0 (two PoK areas, Mirpur and Muzaffarabad) are left out by the builder and counted on its stats line"
  - "States() returns the file's own upper case names (for example TAMIL NADU); consumers title case for display"
  - "Merged territory aliases (Dadra and Nagar Haveli, Daman and Diu) map to the file's single merged state"
requirements-completed: []
status: complete
metrics:
  completed: 2026-10-28
---

# Phase 5 Plan 01: District areas from embedded LGD boundaries Summary

Point in polygon on 783 LGD districts embedded in the binary (345 KB gzip), keyed on `dist_lgd`, with SDMA sender and trailing state name resolution, no database and no network.

## Builder stats line (real run)

`buildareas: features read 785, left out without a code 2, districts written 783, points written 77246, bytes written 345439`

Measured file size: 345,439 bytes (0.35 MB). A second run over the same input gave a clean `git status` (byte reproducible).

## What was built

- `cmd/buildareas` (stdlib only): 64 MB scanner buffer, accepts `dist_lgd` and `state_lgd` as number or numeric string, iterative Douglas-Peucker (explicit stack, closed rings split at the farthest point so they stay closed, a ring that would drop under 4 points keeps its original points), rounding with consecutive duplicate removal, merge per code, sort by code, gzip with a zero header. Malformed lines are errors naming the line number. The census code is never read.
- `internal/areas`: `Default()` (sync.Once, panics on a build defect), `Load`, `Locate` (bounding box prefilter, even odd rule with holes, code order scan), `ByCode`, `Districts`, `States`, `CellDistricts` (centre and four corners of a 5 character lower case geohash cell, deduplicated, code order, nil for anything else), `StateByName`, `StateForSender` (only `<State>-SDMA`), `TrailingState` (after the last comma or the later of the last " of ", a bare name is read whole). Package header carries the source, CC0, Datameet credit, build recipe and the roughly one kilometre edge accuracy limit.
- All eleven named plan tests pass, plus extra builder tests (`TestBuildFileRejectsBadLines`, `TestBuildFileSkipsFeaturesWithoutACode`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Two upstream features have dist_lgd 0**
- **Found during:** Task 2 (`TestEmbeddedFileParses` and `TestByCodeCapCodes` failed)
- **Issue:** The upstream file has two features (Mirpur and Muzaffarabad, areas with no LGD code) with `dist_lgd` 0. The builder merged them under code 0, producing one wrongly named district that no alert can name. This also means the "785 features, 784 codes, one code twice" note in research is really "two features without a code": there are 783 coded districts, no duplicate real code.
- **Fix:** The builder skips features with code 0 and prints how many on the stats line; added `TestBuildFileSkipsFeaturesWithoutACode`; rebuilt the data file. 783 districts is inside the plan's 780 to 800 band.
- **Files modified:** cmd/buildareas/main.go, cmd/buildareas/main_test.go, internal/areas/data/districts.json.gz
- **Commit:** 77419905

Other observations, not deviations: the point count is 77,246 against the research probe's 87,460 (the builder drops consecutive duplicates after rounding and keeps closed rings by splitting at the farthest point); the three anchor points and all eight CAP codes still resolve correctly. Upstream state names are upper case, with "ANDAMAN & NICOBAR" and one merged "DADRA,NAGAR HAVELI,DAMAN & DIU"; `stateAliases` covers the longer spellings, Orissa, Pondicherry, Uttaranchal, NCT of Delhi, New Delhi and Chattisgarh, one comment per entry.

## Amended tests

None.

## Verification

- `go test ./cmd/buildareas/ ./internal/areas/ -count=1` passes; `go vet ./...` clean.
- Full suite on disposable `p5_01` (created, migrated to version 11, dropped afterwards; count of `p5_01` in `psql -lqt` is 0): zero FAIL lines, 2 SKIP lines, TestGeocodeLive and TestScreeningFixture. TestCastVoteConcurrentSameAccountKeepsEveryRow did not flake.
- No em or en dash in the new Go files; no database imports in `internal/areas`; no `.7z` or `.geojsonl` download in the repo (the only `.geojsonl` is the hand written testdata fixture); the raw download lived in a scratch folder that is removed.

## Commits

- 8652ea3c test(05-01): failing builder tests
- 200d2796 feat(05-01): offline builder
- f25a3130 chore(05-01): first embedded data file
- 0ab0c088 test(05-01): failing lookup tests
- 77419905 fix(05-01): skip features without a code, data file rebuilt
- 38ae5eb6 feat(05-01): lookup package

## Known Stubs

None.

## Threat Flags

None. The raw download was handled per T-05-01 (scratch folder outside the repo, read as text by a stdlib builder, only the derived file committed).

## TDD Gate Compliance

RED commits (8652ea3c, 0ab0c088) precede GREEN commits (200d2796, 38ae5eb6) for both tasks. The RED state was a compile failure on undefined functions.

## Self-Check: PASSED

All created files exist and all six commits are present on the worktree branch.
