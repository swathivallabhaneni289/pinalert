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

## Still current signal

Every report carries a second signal beside the counters: is this report still current? It is built by `buildCurrency` in `internal/service/currency.go` from vote times, the report's lifetime, its visibility, its category and severity, and an injected clock. It is display data only. It never hides, blocks or reorders a report.

### Window

The window is the report's lifetime divided by `CurrencyWindowDivisor` (4). That is 2 hours for an 8 hour report and 6 hours for a 24 hour report (D-05). The window itself is exact. The window length sent to the browser is rounded to the nearest whole minute.

`ExpiryDuration` depends on severity only. A low severity rescue needed report therefore lives 8 hours and gets a 2 hour window, even though the critical bypass treats it as critical elsewhere.

The places that count come from the same row filter as the confirmed counter: the reporter is excluded, each account has one current row, and only content confirms count. Resolve and reopen votes never refresh the signal.

### Score formula

N is the number of distinct confirming places over the whole life of the report. It is the same number the confirmed counter shows, from the same `independentCellCount`. A place's time is the time of its latest confirm.

D is the number of distinct dispute places inside the window, and k is `IndependentAgreementThreshold` plus D. A is the age of the k-th most recent confirming place, clamped to between 0 and four windows.

The score is 100 minus the ceiling of `CurrencyPointsPerWindow` (25) times A over the window, computed in integer nanoseconds by `currencyScore`. It is null when N is below 2, and 0 when fewer than k places exist. `buildCurrency` applies these rules in a fixed order, and its doc comment numbers them.

### Bands and words

| Score | Band | Word |
|-------|------|------|
| 75 or more (`UpToDateMinScore`) | Up to date | Up to date |
| 50 to 74 (`GettingOldMinScore`) | Getting old | Getting old |
| 49 or less | Needs re-confirming | Needs re-confirming |

Below 2 places the band is not yet corroborated. Critical and rescue needed reports then read Too early to tell. Every other report shows its Unconfirmed label and no word (D-18). The word is cut from the integer score, so the word and the number cannot disagree.

With no dispute, Up to date means exactly that at least 2 different places confirmed inside the window (R-01).

### Boundary table

A is the age of the k-th most recent place and W is the window. The rows are identical for 8 hour and 24 hour reports.

| Age of the k-th most recent place | Score | Band |
|-----------------------------------|-------|------|
| 0 | 100 | Up to date |
| 1 ns | 99 | Up to date |
| W / 25 | 99 | Up to date |
| W / 25 plus 1 ns | 98 | Up to date |
| W minus 1 ns | 75 | Up to date |
| W | 75 | Up to date |
| W plus 1 ns | 74 | Getting old |
| 1.5 W | 62 | Getting old |
| 2 W | 50 | Getting old |
| 2 W plus 1 ns | 49 | Needs re-confirming |
| 3 W | 25 | Needs re-confirming |
| 4 W (the whole lifetime) | 0 | Needs re-confirming |
| 5 s in the future (clock skew) | 100 | Up to date |

One point is lost per started 1/25 of a window: 4 minutes 48 seconds on an 8 hour report, 14 minutes 24 seconds on a 24 hour report. `TestCurrencyScoreBoundaries` pins every row.

### Worked example

An 8 hour report created at 10:00. Three different places confirm it, at 10:30, 11:00 and 13:00. The window is 2 hours.

| Now | Places | Score | Band |
|-----|--------|-------|------|
| 10:45 | 1 | null | Not yet corroborated |
| 11:00 | 2 | 93 | Up to date |
| 11:30 | 2 | 87 | Up to date |
| 13:00 | 3 | 75 | Up to date |
| 13:30 | 3 | 68 | Getting old |
| 15:00 | 3 | 50 | Getting old |
| 16:00 | 3 | 37 | Needs re-confirming |
| 17:59 | 3 | 12 | Needs re-confirming |

At 11:00 the second most recent place is the one from 10:30, which is 30 minutes old, so the score is 100 minus the ceiling of 25 times 30 over 120, which is 93.

### A standing dispute

Each distinct dispute place inside the window raises k by one, so it cancels one confirming place. A dispute never refreshes the signal.

- Confirms at 10:30 and 11:00 and a dispute at 11:30 give 0 from 11:30 to 13:30. The dispute leaves the window at 13:31, and the score is then 62.
- With a third confirm at 13:00 and the same dispute, the score at 13:00 is 68, where it would be 75 without the dispute.

Because a dispute leaving the window lowers the bar, time alone can raise a contested score. The monotone property that `TestCurrencyMonotone` pins is therefore stated for rows with no dispute: with no dispute, advancing the clock never raises the score. At a fixed clock, adding a confirm never lowers the score and adding a dispute never raises it. This differs from the unconditional wording in the research notes.

### Fade stage

Up to date and not yet corroborated give fresh, Getting old gives aging, and Needs re-confirming gives stale (R-02).

- Critical and rescue needed reports get no fade stage and keep only the time based fade. Their word shows as plain text with no extra dimming (D-07).
- Hidden, Retracted and expired reports get none.
- The browser takes the worse of this stage and its own time stage, and holds no threshold.
- As intended, a non critical report that no 2 places have reconfirmed inside the window dims earlier than the last quarter time rule.

### When no score is shown

- The score is null for not yet corroborated. It is never 0 there, and the report shows no negative word (R-01).
- It is also null for not applicable, the band of Hidden reports (R-03), Retracted reports and expired reports. Only the Activity page shows the last two (R-08).
- A lifetime that is not positive is treated as not applicable, so the function never divides by zero. `Submit` never produces one.
- The facts still carried in those cases are the window in minutes and the places inside it.

### Clocks

Vote times come from the database clock, and now comes from the service's injected clock. An age that comes out negative because of clock skew is treated as 0. The score is recomputed on every read and never stored (Phase 2 D-08). Two reads a moment apart can differ by one step of the score.

### How the roadmap word score is met

The roadmap asks for a fast decaying "is this still current" confidence score (TRUST-07, D-08).

- It is an integer from 0 to 100 per report, from a documented formula over real vote times, recomputed on every read and never stored.
- The word is cut from the integer, so the two cannot disagree.
- It decays one point per started 1/25 of a window and reaches 0 over the report's lifetime. That is the fast decaying half. The reporter half is slow.
- It is null below 2 places, and Phase 6 treats null as unknown.
- The feed row shows only the word. The popup shows the number and the score sentence (D-09, A50).

### What it costs to fake

One account re-tapping Confirm moves only its own place's time, so it cannot refresh the signal without a second, genuinely different place (D-05). Faking Up to date costs 2 verified accounts and 2 claimed locations, the same as forging Live. A spoofed location defeats it exactly as it defeats the place rule. See "Honest limits" below.

The tests that pin this section are `TestCurrencyScoreBoundaries`, `TestCurrencyNotYetCorroborated`, `TestCurrencyRetapCannotRefresh`, `TestCurrencyDisputes`, `TestCurrencyWindowFollowsLifetime`, `TestCurrencyMonotone`, `TestFadeStageMapping` and `TestTrustCopyCurrencyIsPlainText`, rerun with `go test ./internal/service/ -run 'TestCurrency|TestFadeStageMapping|TestTrustCopyCurrencyIsPlainText' -count=1`.

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
