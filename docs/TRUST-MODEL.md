# Pinalert trust model

This document explains how Pinalert decides that a confirmation is independent and nearby. "N confirmed" on a report means N different places, not N votes. Every number in it comes from a named test that anyone can rerun, and the limits of the model are written down next to the rules.

## What counts as a place

The server computes each vote's cell as the geohash of the voter's coordinates at precision 7 (`voterGeohashPrecision` in `internal/service/trust.go`). It never accepts a cell from the client. The confirmed count N is the number of distinct cells among the current confirm votes of verified accounts other than the reporter (`ConfirmCells` from `BuildVoteTally`, counted by `independentCellCount`). The disputed count M is `DisputeCells` under the same rule.

One precision constant and one counting rule serve the resolver gate (`IndependentAgreementThreshold` is 2), the two counters, the still current signal and the reporter rating. Reports themselves are located at precision 8 (`geohashPrecision` in `internal/service/report.go`), which answers a different question: where a report is, not how many separate places agree with it.

### Derivation table

| Precision | Bits (longitude, latitude) | Cell height | Cell width at Chennai (13.08 N) | What it does to a 300 m incident |
|-----------|----------------------------|-------------|---------------------------------|----------------------------------|
| 5 | 13, 12 | 4886.5 m | 4759.7 m | The whole incident is one place. |
| 6 | 15, 15 | 610.8 m | 1189.9 m | At most 4 places from any crowd. Two witnesses merge into one place in 295 to 319 of 500 trials. |
| 7 | 18, 17 | 152.7 m | 148.7 m | At most 23 to 26 places, depending on latitude. |
| 8 | 20, 20 | 19.1 m | 37.2 m | A 200 person crowd counts as 158.6 to 163.9 places on average. |

Cell height is 180 degrees divided by 2 to the power of the latitude bits, times 111194.9 m per degree. Cell width is 360 degrees divided by 2 to the power of the longitude bits, times 111194.9 m, times the cosine of the latitude. All of it is on a sphere of radius 6371 km, the same radius the feed distance and the 1 km rule use.

### Width by city at precision 7

| City | Latitude | Cell width | Places a 300 m circle can touch |
|------|----------|------------|---------------------------------|
| Chennai | 13.08 N | 148.7 m | 23 |
| Bengaluru | 12.97 N | 148.8 m | 23 |
| Kolkata | 22.57 N | 141.0 m | 26 |
| Delhi | 28.61 N | 134.1 m | 26 |
| Srinagar | 34.08 N | 126.5 m | 26 |

The cell height is 152.7 m everywhere. The ceiling is the most cells one 300 m circle can overlap wherever it falls on the grid.

### Witness simulation

Method: `math/rand` with `NewSource(42)`, 500 trials per city, precision and crowd size. In each trial the circle centre is shifted by up to one cell in each axis, so every alignment with the cell grid is sampled. Witnesses are placed uniformly inside a 300 m circle, and places are counted with `independentCellCount`. The five cities are Chennai, Bengaluru, Kolkata, Delhi and Srinagar. Each range below runs from the lowest to the highest value over those five cities.

| Precision | Crowd | Places (lowest to highest) | Mean places | Trials counted as two places |
|-----------|-------|----------------------------|-------------|------------------------------|
| 6 | 2 | 1 to 2 | 1.36 to 1.41 | 181 to 205 of 500 |
| 6 | 5 | 1 to 4 | 1.86 to 1.94 | |
| 6 | 50 | 1 to 4 | 2.53 to 2.71 | |
| 6 | 200 | 1 to 4 | 2.72 to 2.84 | |
| 7 | 2 | 1 to 2 | 1.93 to 1.95 | 466 to 473 of 500 |
| 7 | 5 | 2 to 5 | 4.38 to 4.47 | |
| 7 | 50 | 11 to 23 | 15.73 to 17.61 | |
| 7 | 200 | 15 to 26 | 18.74 to 21.33 | |
| 8 | 2 | 1 to 2 | 2.00 | 499 of 500 |
| 8 | 5 | 3 to 5 | 4.95 to 4.98 | |
| 8 | 50 | 41 to 50 | 47.15 to 47.61 | |
| 8 | 200 | 146 to 177 | 158.63 to 163.85 | |

An earlier figure of 17 to 22 places for 200 witnesses came from a circle always centred on the city point. Letting the circle fall anywhere on the grid reaches the ceiling (Srinagar reaches 26 exactly), so this document quotes the ceiling and not the sample maximum.

### Why 7

Precision 6 is too coarse. Two honest witnesses across a 300 m incident often count as one place, and no crowd can show more than 4 places, so a real incident can never look well corroborated.

Precision 8 is too fine. A crowd is counted almost person by person, so N would track how many accounts stood in one spot rather than how many separate places agree.

Precision 7 matches the 100 to 300 m scale of a real incident. Two witnesses spread across it count as two places in at least 85 percent of trials (the asserted floor), and in the measured run 466 to 473 of 500 trials did. A crowd of any size counts as at most 23 to 26 places, so N is a lower bound on people. This closes the interim status of precision 7 (02-RESEARCH assumption A3). The numbers come from `TestGeohashPrecisionWitnesses` in `internal/service/geohash_precision_test.go`, rerun with `go test ./internal/service/ -run TestGeohashPrecisionWitnesses -v` (no database needed).

## Who can vote: the 1 km rule

The cell rule decides how many places count. The distance rule decides who may vote at all.

There is one named constant, `MaxVoteDistanceKm = 1.0` in `internal/service/geo.go`. The distance is the haversine distance on a sphere of radius 6371 km, the same radius the feed's distance uses. The comparison is written so that a non finite coordinate (not a number, or infinity) is refused, never accepted.

The rule covers all four vote values (confirm, dispute, resolve, reopen) across both vote kinds. The one exemption is the reporter's own resolve and reopen, because those votes are uncounted and instant (Phase 2 D-13 and amended D-16). The server decides the exemption by comparing the reporter account stored with the report to the signed in account, never from anything in the request.

Order of checks inside `CastVote`: input validation, report lookup, the block on content votes for your own report, expiry (an expired report is refused as expired even from far away), distance, then the write.

A refused vote returns the service error `ErrVoterTooFar`, which the API answers with HTTP 403, field `location` and the sentence "Too far to vote. You need to be within 1 km of this report." The sentence is the same for all four vote values, and the response never contains the computed distance.

1 km was chosen over 2 km and over having no distance rule. The check runs when the vote is written, so no schema change was needed.

The tests are `TestWithinVoteRadiusBoundary`, `TestCastVoteRejectsTooFar`, `TestCastVoteCheckOrder` and `TestCastVoteReporterExemption`.

## Votes cast before the 1 km rule

Votes written before this phase were never distance checked. They cannot be checked now, because the votes table stores only the voter's cell and never the coordinates. They are grandfathered: they keep counting toward N, the resolver gate and reporter ratings.

A database created before this phase may therefore hold votes cast from far away. The distance tests always run on a fresh disposable database for this reason.

## Honest limits

- Two accounts a few metres apart on either side of a cell edge count as two places. The subtest "cell edge pair counts as two places" shows it: points 1 m apart across a precision 7 cell edge count as two places through `independentCellCount`.
- Several people standing in one cell count as one place, so N is a lower bound on people, not a headcount.
- Each account holds one current vote per report and kind (the latest vote wins in `CurrentVotesForReports`), so walking around cannot add places.
- The real cost of adding a place is one more verified email account plus a claimed location for it.
- Client coordinates cannot be verified. A spoofed location defeats both the cell rule and the 1 km rule.
- Pinalert does not read GPS accuracy and does not look for mock locations.

## Out of scope for this model

- A dampening curve for the displayed count.
- Sending GPS accuracy with a vote, radii per category or severity, and mock location detection.
- Burst and brigading detection, including a cap on how much one confirmer pair can add to a reporter's record (TRUSTX-03), and confirmer accuracy scoring (TRUSTX-01).
- Vote rate limiting (Phase 4).
- Email alias normalisation and disposable email blocking.
- Re-confirmation extending a report's expiry.
