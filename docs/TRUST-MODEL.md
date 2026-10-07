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

The sentence is written by the function `TooFarMessage` in `internal/service/geo.go`, from `MaxVoteDistanceKm`, so the number in the sentence and the number in the rule cannot differ.

The tests are `TestHaversine`, `TestWithinVoteRadiusBoundary`, `TestCastVoteRejectsTooFar`, `TestCastVoteCheckOrder` and `TestCastVoteReporterExemption`.

## Votes cast before the 1 km rule

Votes written before this phase were never distance checked. They cannot be checked now, because the votes table stores only the voter's cell and never the coordinates. They are grandfathered: they keep counting toward N, the resolver gate and reporter ratings.

A database created before this phase may therefore hold votes cast from far away. The distance tests always run on a fresh disposable database for this reason.

## The two counters

Every report shows two counters, `N confirmed` and `M disputed`. Both always show, also at 0.

- The confirmed count N is the number of distinct confirming places, `ConfirmCells` from `BuildVoteTally`. The disputed count M is `DisputeCells`.
- Both are built by `filterVoteRows`, which skips the reporter's own content votes, keeps one current row per account, keeps confirm and dispute rows only, and uses the voter's place as the server computed it at precision 7.
- The same N gates Live, drives the display, the still current signal and the reporter classifier. There is one counting rule, in one function, and nothing recounts it.
- The surface numerals are written by the server as `confirmed_text` and `disputed_text`: "0" to "99", then "99+". The exact counts stay in `confirmed_places` and `disputed_places`, and the panel sentences always carry the real number.
- N is a lower bound on people, not a headcount. See "Honest limits" below.

The panel sentences are written by the server. T is `IndependentAgreementThreshold`.

| Line | Case | Text |
|------|------|------|
| Lead | Provisional | `Needs {T} confirmations before this counts as confirmed.` |
| Lead | expired, not retracted (Activity page) | `This report has expired. It can no longer be confirmed.` |
| Confirmed | N is 0 | `No one has confirmed this yet. Counts separate places, not votes.` |
| Confirmed | N is 1 | `1 person in 1 place confirmed this. Counts separate places, not votes.` |
| Confirmed | N is 2 or more | `{N} people in {N} different places confirmed this. Counts separate places, not votes.` |
| Disputed | M is 0 | `No one nearby has disputed this. Counts separate places, not votes.` |
| Disputed | M is 1 | `1 person in 1 place disputed this. Counts separate places, not votes.` |
| Disputed | M is 2 or more | `{M} people in {M} different places disputed this. Counts separate places, not votes.` |

The tests that pin this section are `TestBuildTrustCountsMatchTally`, `TestTrustCountLabel`, `TestTrustCountsDistinctPlacesNotVotes` and `TestTrustCountUnderConcurrentVotes`. The last one casts votes from many places at once and needs a disposable database.

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

The tests that pin this section are `TestCurrencyScoreBoundaries`, `TestCurrencyNotYetCorroborated`, `TestCurrencyRetapCannotRefresh`, `TestCurrencyDisputes`, `TestCurrencyWindowFollowsLifetime`, `TestCurrencyMonotone`, `TestFadeStageMapping` and `TestTrustCopyCurrencyIsPlainText`, rerun with `go test ./internal/service/ -run 'TestCurrencyScoreBoundaries|TestCurrencyNotYetCorroborated|TestCurrencyRetapCannotRefresh|TestCurrencyDisputes|TestCurrencyWindowFollowsLifetime|TestCurrencyMonotone|TestFadeStageMapping|TestTrustCopyCurrencyIsPlainText' -count=1`.

## Reporter reliability

Every report also carries a slow signal beside the still current one: is the person who posted it a reliable source? It is built from how that person's earlier reports turned out. The function that composes it is `ReporterSignals` in `internal/service/reporter.go`. It decides no rule itself. `ClassifyOutcome` and `ScoreReliability` in `internal/service/reliability.go` own every rule, and `internal/service/trustcopy.go` owns every word.

### Who is rated

The rating is public and person level. It is keyed to the verified account through `sessions.account_id`, across every session the account has used, and never to a session. Someone who signs in on a second phone keeps one record.

The tag and the counts behind it are visible to everyone on every report, and the popup also shows the number. The account id and the email are never shown. The signal carries only the tag, the score, the two counts and server written sentences.

Accountless reports, and official pins in a later phase, have no rating. They carry the tag `none` and show nothing (A47). A feed read rates at most 200 reporter accounts, taken from the nearest reports first, and reports by accounts past that cap also carry none.

The history is read in one query per request for all reporters on the page, backed by the index `idx_reports_session_expires`, with one batch of current votes beside it. `ReporterSignals` groups those votes by report once, so the work grows with the history rows plus the vote rows and not with their product.

### Outcomes

Each finished report gets one outcome from `ClassifyOutcome`. It asks `Resolve` itself, with neutral metadata (low severity, other category) and the tally's resolve and reopen fields zeroed. Live maps to corroborated, Hidden to contradicted and Provisional to unconfirmed. No threshold is copied, so the rating can never disagree with the visibility ladder.

For each history report, the reporter is the account being scored, passed to `BuildVoteTally`. So the reporter's own confirmations never count, and their own resolve or reopen cannot erase a record. A report its reporter marked resolved is still judged by the content votes of other people.

A critical or rescue needed report that expires unconfirmed is neutral, because those reports skip the Provisional gate and an unconfirmed one says nothing about the reporter (D-12). A critical or rescue needed report that independent places out dispute counts as contradicted for the rating only. On the read path it is still never Hidden (R-06).

### What counts as judged

A report is judged once its expiry time is at or before now, whether or not it was resolved (R-05). A report still in flight is neutral. Neutral outcomes are not judged.

At least 5 judged reports (`ReliabilityMinJudged`) are needed, counted without weights, inside the window and the cap. Below that the reporter is a New reporter with no number and never a default score. The real judged count is still reported, so the sentence can say how far along they are.

### Window, half life and the 30 report cap

Only reports that expired within the last 90 days count (`ReliabilityWindow`). A report exactly 90 days old is out, and one that expires exactly now is in. Age is measured from the expiry time, and each outcome's weight halves every 30 days (`ReliabilityHalfLife`).

The read keeps the latest 30 expired reports per account (`ReliabilityRowCap`) across all of its sessions, newest expiry first. The cap counts the latest 30 expired reports in the window, not 30 judged reports.

That is a decision, and the reasoning is this:

- The decision's own wording says 30 judged reports. SQL cannot know which reports are judged without their votes, so the read caps expired reports and Go classifies them.
- The two readings differ only for a reporter with many critical or rescue needed reports that expired unconfirmed, which are neutral.
- The exact alternative, reading 60 and keeping the first 30 judged in Go, doubles the read cost and was not taken.
- `ScoreReliability` applies the same cap to whatever rows it receives, neutral rows included, so the result is the same whether or not the read was capped.
- The test case that shows 30 newer neutral reports pushing 5 corroborated ones out is in `TestReporterSignalsNewAndNone`, beside the case with 29 neutral reports where one corroborated report survives.

The owner may still choose the exact reading.

### Reporter score formula

Credits are 1 for corroborated, 0 for contradicted and 0.25 for unconfirmed (`UnconfirmedCredit`). Let p be the recency weighted mean credit, with weight 2 to the power of minus the age over 30 days. The score is round(100 (n p + 1) / (n + 2)) with n the judged count, so a short record is pulled toward 50.

The code computes this as integer arithmetic and not as a floating point formula:

- each weight is computed once with `math.Exp2`, scaled by 2 to the 30th power and rounded to an integer;
- credits are whole hundredths (100, 0 and 25);
- the sums are 64 bit integers;
- the score is an integer division that rounds half up.

So the boundaries 75, 35 and 25 are exact (4 corroborated and 1 unconfirmed is exactly 5.25 / 7, which is 75), equal ages give equal weights, and the order of reports never changes the result. `ScoreReliability` is the one function that does this.

### Tags and sentences

Tags are cut from the rounded integer. 75 or more (`ReliableMinScore`) is Reliable reporter, 35 to 74 (`MixedMinScore`) is Mixed record, and 34 or less is Unreliable reporter. Fewer than 5 judged is New reporter with no number.

New reporter never looks like a good tag. It is the same plain text as the others, with no check, star or colour. The feed row and the Activity page show the tag. The panel shows the reporter sentence. The popup also shows the number and the score sentence.

The templates, in the server's exact words:

- `{C} of {J} earlier reports were confirmed by people nearby.`
- `Only {J} of their reports have had a clear result so far. A rating needs {MIN} reports with a clear result.` for J of 2 or more
- `Only 1 of their reports has had a clear result so far. A rating needs {MIN} reports with a clear result.` for J of 1
- `None of their reports has had a clear result yet. A rating needs {MIN} reports with a clear result.` for J of 0
- `Score {SCORE} out of 100, based on how often their reports with a clear result were confirmed.`

C is the number of corroborated reports, J the number of judged reports, MIN the judged minimum and SCORE the integer score. The score sentence exists only when the score is not null.

The copy lives in `trustcopy.go` and reaches the browser as plain strings on the trust object's `reporter` key: the tag, the score, the label, the sentence, the score sentence and the judged and corroborated counts. The browser holds no rule and no number of its own.

### Anchor table

Weights are fresh unless a row says otherwise. Uniform ageing gives the same scores, because equal ages give equal weights.

| History (n) | p | Score | Tag |
|-------------|---|-------|-----|
| 5 corroborated (any ages inside the window) | 1.00 | 86 | Reliable |
| 30 corroborated | 1.00 | 97 | Reliable |
| 4 corroborated, 1 unconfirmed (5) | 0.85 | 75 (exact boundary, 5.25 / 7) | Reliable |
| 4 corroborated, 1 contradicted (5) | 0.80 | 71 | Mixed |
| 3 corroborated, 2 unconfirmed (5) | 0.70 | 64 | Mixed |
| 3 corroborated, 2 contradicted (5) | 0.60 | 57 | Mixed |
| 5 corroborated, 5 unconfirmed (10) | 0.625 | 60 | Mixed |
| 2 corroborated, 3 contradicted (5) | 0.40 | 43 | Mixed |
| 5 unconfirmed only (5) | 0.25 | 32 | Unreliable |
| 30 unconfirmed only (30) | 0.25 | 27 | Unreliable |
| 1 contradicted, 4 unconfirmed (5) | 0.20 | 29 | Unreliable |
| 2 contradicted, 3 unconfirmed (5) | 0.15 | 25 | Unreliable |
| 1 corroborated, 4 contradicted (5) | 0.20 | 29 | Unreliable |
| 5 contradicted (5) | 0.00 | 14 | Unreliable |
| 8 corroborated, 2 contradicted (10) | 0.80 | 75 (exact) | Reliable |
| 6 corroborated, 12 contradicted (18) | 0.333 | 35 (exact) | Mixed |
| corroborated at 5, 20, 50 days, contradicted at 10 days, unconfirmed at 40 days (5) | 0.6394 | 60 | Mixed |
| 5 corroborated at 80 days, 2 contradicted at 1 day (7) | 0.287 | 33 | Unreliable |
| 5 contradicted at 80 days, 2 corroborated at 1 day (7) | 0.713 | 67 | Mixed |

`TestReliabilityTable` pins these rows against the code.

### Invariants

- I1: a history with no contradicted and no unconfirmed outcome (5 to 30 judged, any ages) scores at least 85.7 and is Reliable.
- I2, restated for a credit of 0.25: a history of only unconfirmed outcomes with at least 5 judged always reads Unreliable reporter. Its score is 100 (0.25 n + 1) / (n + 2), which is 32.14 at n = 5 and falls toward 25, whatever the weights.
- I3: replacing an outcome by a better class never lowers the score, and order never matters.
- I4: the score lies in [100 / (n + 2), 100 (n + 1) / (n + 2)].

`TestReliabilityInvariants` pins all four.

### Display only

The tag appears on every report, including critical and rescue needed ones, as the same plain text. It never hides, dims, blocks, reorders or filters anything (D-14). A Hidden report in the Show disputed view shows the tag and no still current word. A later triage phase may use it as one soft input.

### How the roadmap word score is met for the reporter

The roadmap asks for a slow decaying "is this source reliable" score (TRUST-07, D-08).

- It is an integer from 0 to 100 per reporter, from the documented formula, recomputed on every read and never stored.
- It decays with a 30 day half life inside 90 days. That is the slow half beside the fast still current score.
- It is null for a New reporter, and a later phase treats null as unknown.
- The feed row shows the tag. The popup shows the number with its score sentence (D-09, A50). The two signals sit side by side on every report.

### Known limits of the rating

- A liar can start over with a fresh email. They then read New reporter, which never looks good (D-15).
- Two colluding accounts could dispute an honest reporter. Burst and brigading detection is deferred (TRUSTX-03).
- Confirmer accuracy is not scored (TRUSTX-01).
- Votes cast before the 1 km rule are grandfathered and may inflate old records. See the section "Votes cast before the 1 km rule".
- A newcomer in a quiet area can read Unreliable reporter after five quiet reports that are not critical. The owner accepted this cost (D-23).
- On a small deployment most reporters will read New reporter (D-13).
- A public rating links one person's reports together. This is intended (D-10), and no account id or email is shown.
- A faked location defeats the place rule behind every outcome exactly as it defeats the counters. See "Honest limits" below. The rating claims no protection against it.

The tests that pin this section are `TestClassifyOutcome`, `TestReliabilityTable`, `TestReliabilityInvariants`, `TestReliabilityDecayAndWindow`, `TestReporterSignalsGroupByAccount`, `TestReporterSignalsNewAndNone` and `TestTrustCopyReporterIsPlainText`, rerun with `go test ./internal/service/ -run 'TestClassifyOutcome|TestReliabilityTable|TestReliabilityInvariants|TestReliabilityDecayAndWindow|TestReporterSignalsGroupByAccount|TestReporterSignalsNewAndNone|TestTrustCopyReporterIsPlainText' -count=1` (no database needed). The history read tests `TestReporterHistoryBounds` and `TestReporterHistoryUsesAccountNotSession` need a disposable database, and so does `TestReporterSignalFromSeededHistory`, which seeds a real history and reads the signal through the API.

## One trust object on every surface

`BuildTrust` in `internal/service/trustview.go` builds one `Trust` per report. `Nearby`, `CastVote` and `ActivityForAccount` call it with `now` captured once per request, so a feed row, a vote response and an Activity row read the same clock. The handler function `trustToResponse` maps it to the `trust` key of `GET /api/reports`, of every vote response and of the Activity page `data-trust` attribute. The object is always present.

| Key | Type | Rule |
|-----|------|------|
| `confirmed_places` | int | N, the distinct confirming places. |
| `disputed_places` | int | M, the distinct disputing places. |
| `confirmed_text` | string | "0" to "99", then "99+". |
| `disputed_text` | string | The same rule. |
| `lead_sentence` | string | Empty for a retracted report. For an expired one the expiry line. For a Provisional one the lead sentence. Otherwise empty. |
| `confirmed_sentence` | string | Always set. |
| `disputed_sentence` | string | Always set. |
| `currency.band` | string | `up_to_date`, `getting_old`, `needs_reconfirming`, `not_yet_corroborated` or `not_applicable`. |
| `currency.score` | int or null | Null for `not_yet_corroborated` and `not_applicable`. |
| `currency.label` | string | The word, or empty when no word shows. |
| `currency.sentence` | string | Empty exactly when `label` is empty. |
| `currency.score_sentence` | string | Empty exactly when the score is null. |
| `currency.recent_places` | int | Distinct confirming places inside the window. |
| `currency.window_minutes` | int | The window in whole minutes. |
| `currency.fade_stage` | string or null | `fresh`, `aging` or `stale`; null for critical, rescue needed, Hidden, Retracted and `not_applicable` reports. |
| `reporter.tag` | string | `reliable`, `mixed`, `unreliable`, `new` or `none`. |
| `reporter.score` | int or null | Null for `new` and `none`. |
| `reporter.label` | string | Empty for `none`. |
| `reporter.sentence` | string | Empty for `none`. |
| `reporter.score_sentence` | string | Empty when the score is null. |
| `reporter.judged_reports` | int | J, the judged reports. |
| `reporter.corroborated_reports` | int | C, the corroborated reports. |

What never leaves the server: the account id, the email, the voter's place, the session id and any raw vote total. `TestTrustResponseOmitsIdentifiers` checks the live response, and `TestSwaggerSpecCoversRoutes` checks the documented field names against the same rule.

The same object reaches every surface, and these tests prove it: `TestTrustAttachedOnEveryReadPath`, `TestFeedTrustMatchesCastVoteTrust`, `TestFeedSignalsMatchCastVoteSignals`, `TestCastVoteTrustMatchesNearbyTrust` and `TestProfileRowsCarryTrustJSON`.

Cost bounds: a feed read makes one batched vote read and one history read, and rates at most `MaxReporterAccountsPerRead` reporter accounts, nearest first. Reports past the cap read `none`. `TestNearbyBatchesVoteReadsOnceWithReporters` and `TestNearbyReporterAccountCap` pin this.

Display only: neither signal hides a report, dims it beyond the time fade or reorders it. Critical and rescue needed rows keep the time fade only (D-14). `TestNearbyRatingNeverChangesVisibilityOrOrder` and `TestFadeUsesCombinedStage` pin this, and `TestTrustModuleHasNoThresholdsOrSinks` pins that the browser holds no threshold of its own.

## Browser hints, server decisions

The server decides every vote. The browser only helps the voter get a fresh location to the server and shows a hint.

- The refusal: a vote from beyond 1 km gets HTTP 403, field `location`, and the sentence `TooFarMessage()` returns: "Too far to vote. You need to be within 1 km of this report." The body never holds a distance. The distance itself comes from `haversineKm` and the comparison from `withinVoteRadius`. `TestCastVoteTooFar` pins the response.
- The silent refresh (D-02): a cached fix younger than 60 seconds is reused. Otherwise, if location permission is already granted, the browser takes one silent fresh fix. After a refusal with field `location` it retries once with a fresh fix. It never prompts for each vote. `TestVoteTransportRefreshesAndRetriesOnce` pins this.
- The too far hint (D-22): the page carries `data-vote-radius-km`, which equals `MaxVoteDistanceKm` (`TestPageShellCarriesVoteRadius`). With a cached fix younger than 120 seconds, Confirm, Dispute and Mark resolved on a report farther than that radius get `aria-disabled` and the same sentence as visible text. The reporter's own Mark resolved stays enabled, and a stale fix never disables a button. `TestVoteHint` pins this.

The hint is advisory. A forced vote is still refused by the server.

## Where the roadmap words are met

| Roadmap words | What ships | Where in code | Test |
|---------------|------------|---------------|------|
| "confirmed by N nearby" | The counter `N confirmed`: N distinct places, every vote cast from within 1 km | `BuildTrust`, `CastVote` | `TestBuildTrustCountsMatchTally`, `TestCastVoteRejectsTooFar` |
| deliberately chosen, documented geohash precision | Precision 7, with the reasoning in "What counts as a place" | `voterGeohashPrecision` | `TestGeohashPrecisionWitnesses` |
| not the raw number of votes | Distinct places, so repeat votes and a crowd in one place count once | `BuildVoteTally` | `TestTrustCountsDistinctPlacesNotVotes` |
| two distinct trust signals side by side | The still current word and the reporter tag in one signals row | `BuildTrust` | `TestFeedSignalsMatchCastVoteSignals`, `TestTrustBlockMountedOnBothSurfaces` |
| fast decaying still current confidence score | `currency.score` | `buildCurrency` | `TestCurrencyScoreBoundaries` |
| slow decaying source reliable score | `reporter.score` | `ScoreReliability` | `TestReliabilityTable`, `TestReliabilityDecayAndWindow` |

How the word score is met (D-08, TRUST-07), for both halves: each score is an integer from 0 to 100 (`currency.score` and `reporter.score`) from a documented formula over real votes. It is recomputed on every read and never stored. The band or tag is cut from the same integer, so the word and the number cannot disagree. A score is null when there is no basis for one: fewer than 2 confirming places over the life of the report, or `not_applicable`, for the still current score, and New reporter or none for the reporter score. The numbers show on screen in the map popup only (D-09), and Phase 6 can sort on them server side. The two subsections called "How the roadmap word score is met" in the still current and reporter sections give the detail, so it is not repeated here.

The visible form of "confirmed by N nearby" is the counter `N confirmed`, with the panel sentence explaining that N counts places. That is the form CONTEXT records for the first roadmap criterion.

## Hand offs to later phases

- Phase 03.1 reuses `ClassifyOutcome` as a pure function, so history labels cannot disagree with the rating.
- Phase 03.2 builds update wording from the same trust object. It should treat the Reliable reporter tag as farmable at the price stated in "Honest limits": extra verified accounts plus a claimed location.
- Phase 4 owns vote rate limiting.
- Phase 5 demo mode has to handle far away visitors, who cannot vote on seeded reports under the 1 km rule. It should place simulated reports near the visitor or exempt demo reports. It may seed clearly labelled demo accounts to show a rated reporter.
- Phase 6 may sort on the integer scores server side. It must never use them to bury a critical or rescue needed report (D-14), and must never bake a count or a rating into a shareable card image.
- Phase 8's report page reuses the trust block unchanged.

## Keeping this document true

`TestTrustModelDocMatchesCode` in `internal/service/trust_model_doc_test.go` guards this document. It checks the constants table below against the compiled values, that every named identifier is declared where it is said to be, that `CastVote`'s refusal maps to HTTP 403 with field `location` and the sentence above, that every test named here exists, the heading order and the plain text rules (no long dashes, no claim that fake locations are stopped).

| Constant | File | Value |
|----------|------|-------|
| `MaxVoteDistanceKm` | `internal/service/geo.go` | 1 km |
| `voterGeohashPrecision` | `internal/service/trust.go` | 7 |
| `IndependentAgreementThreshold` | `internal/service/visibility.go` | 2 |
| `CurrencyWindowDivisor` | `internal/service/currency.go` | 4 |
| `CurrencyPointsPerWindow` | `internal/service/currency.go` | 25 |
| `UpToDateMinScore` | `internal/service/currency.go` | 75 |
| `GettingOldMinScore` | `internal/service/currency.go` | 50 |
| `ReliabilityMinJudged` | `internal/service/reliability.go` | 5 |
| `ReliabilityHalfLife` | `internal/service/reliability.go` | 30 days |
| `ReliabilityWindow` | `internal/service/reliability.go` | 90 days |
| `ReliabilityRowCap` | `internal/service/reliability.go` | 30 |
| `UnconfirmedCredit` | `internal/service/reliability.go` | 0.25 |
| `ReliableMinScore` | `internal/service/reliability.go` | 75 |
| `MixedMinScore` | `internal/service/reliability.go` | 35 |
| `MaxReporterAccountsPerRead` | `internal/service/report.go` | 200 |

When you change one of these, change the table in the same commit. The test fails with the row name when they differ.

Generated code: run `make sqlc` after editing a `.sql` query and `make swag` after changing a handler annotation or a response struct, and commit the output. `git status --porcelain` should then stay empty for `internal/store/sqlc` and for `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml`.

Databases: tests that need Postgres run against a database created for the run and dropped after it, never the one a dev server uses, because `testutil.NewTestDB` truncates every table.

Upgrading: an existing database needs `make migrate` once after this phase merges, to add `idx_reports_session_expires` from migration 00005. Votes cast before the 1 km rule keep counting. See "Votes cast before the 1 km rule".

## Honest limits

- Two accounts a few metres apart on either side of a cell edge count as two places. The subtest "cell edge pair counts as two places" shows it: points 1 m apart across a precision 7 cell edge count as two places through `independentCellCount`.
- Several people standing in one cell count as one place, so N is a lower bound on people, not a headcount.
- Each account holds one current vote per report and kind (the latest vote wins in `CurrentVotesForReports`), so walking around cannot add places.
- The real cost of adding a place is one more verified email account plus a claimed location for it.
- Client coordinates cannot be verified. A spoofed location defeats both the cell rule and the 1 km rule.
- Pinalert does not read GPS accuracy and does not look for mock locations.
- Phone GPS error of tens of metres can put an honest voter standing near the 1 km line on the wrong side of it. Accuracy is not sent with a vote, and the silent refresh only replaces a stale fix, it does not make a fix more exact.
- Counters, bands and fades update on the 30 second poll, so a screen can be one poll behind the server.
- The too far hint in the browser is advisory. Only the server decides whether a vote counts.

## Out of scope for this model

- A dampening curve for the displayed count.
- Sending GPS accuracy with a vote, radii per category or severity, and mock location detection.
- Burst and brigading detection, including a cap on how much one confirmer pair can add to a reporter's record (TRUSTX-03), and confirmer accuracy scoring (TRUSTX-01).
- Vote rate limiting (Phase 4).
- Email alias normalisation and disposable email blocking.
- Re-confirmation extending a report's expiry.
- A report page with comments (Phase 8).
- Demoting stale non critical reports to Provisional, or dropping them before they expire.
- Push notifications when a count crosses a threshold, or when a confirmed report goes quiet or is retracted.
- A public reporter profile page, or badge tiers beyond the four tags.
- A stored or materialised reliability score refreshed by a job. Both scores are recomputed on every read.
- Triage ordering by these signals (Phase 6).
- Demo mode (Phase 5).
