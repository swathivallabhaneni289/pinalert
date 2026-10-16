# Pinalert trust model

This document explains how Pinalert decides that a confirmation is independent and nearby. "N confirmed" on a report means N different places, not N votes. The panel is written for people, so it calls a place a spot and says "people at different spots", never the word place. Every number in it comes from a named test that anyone can rerun, and the limits of the model are written down next to the rules.

## What counts as a place

The server computes each vote's cell as the geohash of the voter's coordinates at precision 7 (`voterGeohashPrecision` in `internal/service/trust.go`). It never accepts a cell from the client. The confirmed count N is the number of distinct cells among the current confirm votes of verified accounts other than the reporter (`ConfirmCells` from `BuildVoteTally`, counted by `independentCellCount`). The disputed count M is `DisputeCells` under the same rule.

People at the same place count once. Several people standing in one cell are one place, so N is a lower bound on people and never a headcount, and a crowd cannot raise the number. The panel does not carry a caveat for this. Its sentences name who is counted, for example "Confirmed by 3 people at different spots.", and the rule is explained here (amendment A88 in `03-UI-SPEC.md`).

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

The vote vocabulary has five values in two kinds: confirm and dispute (the content kind), and resolve, reopen and `unresolve` (the resolution kind). The distance rule covers the four values that make a claim about the report, confirm, dispute, resolve and reopen. There are two exemptions. The first is the reporter's own resolve and reopen, because those votes are uncounted and instant (Phase 2 D-13 and amended D-16). The server decides it by comparing the reporter account stored with the report to the signed in account, never from anything in the request. The second is the take back, described next, which makes no claim about the place.

The take back: Phase 3.1 added a fifth value, `unresolve`, for a person who tapped Mark resolved by mistake (D-22). It is valid for the resolution kind only, and the route is `POST /api/reports/{id}/unresolve`. It drops the caller's own standing resolve and says nothing else.

- It is neutral. The latest row per account and kind wins, so an `unresolve` row replaces the caller's `resolve` row, and `filterVoteRows` puts it in no bucket. It never counts toward the reopen cells, which is why it is a value of its own and is never sent as `reopen`: a non poster's `reopen` is a claim that the report is still open and counts toward lifting a closed report. `TestTakeBackNeverEntersReopenCells` pins this. A poster's `unresolve` sets both reporter flags false, so it is not an instant reopen either.
- It needs no location. It claims nothing about the place and only withdraws the caller's own mark, so the 1 km check and the missing coordinates check do not apply to it. It has its own predicate in `CastVote` and the reporter exemption is not widened to cover it. A take back sent with no coordinates stores a cell computed from the point 0,0, as the reporter's own location free resolve already does, and that cell is never counted because the row enters no bucket.
- It is refused when the caller has no standing resolve on the report (`ErrNothingToTakeBack`, HTTP 409), and when the report has faded, because the expiry check comes first, so a faded report cannot change.
- It is limited to two per report per account (`UndoLimit` in `internal/service/takeback.go`, D-22). The count comes from the append only votes log and needs no new table: the account's `unresolve` rows, plus its `reopen` rows when the account is the poster, because only the poster's reopen is a take back of their own resolve. A non poster's earlier `reopen` rows never spend the budget (D-28). Past the limit the take back and the poster's reopen are refused (`ErrUndoLimitReached`, HTTP 409). A resolve is never refused for the limit. The limit holds for requests that arrive one after another; parallel requests from one account can exceed it by the number of extra parallel requests (see the accepted race under Honest limits, T-31-09).
- It can open a report again, and that is the intended effect. A report reads from the standing counts on every read (Phase 2 D-08), so when the caller's resolve was one of the two that held a report closed, taking it back means the agreement to close no longer stands. It never adds a reopen claim of its own.

`TestCastVoteTakeBack` pins the refusals, the limit and the poster's budget.

Order of checks inside `CastVote`: input validation, report lookup, the block on content votes for your own report, expiry (an expired report is refused as expired even from far away), the refusal of a vote that carries no coordinates (excused for the reporter's own resolve and reopen and for the take back), distance, for a take back the standing resolve check, then for a take back or the poster's own reopen the undo limit check (all before any write), then the write.

A refused vote returns the service error `ErrVoterTooFar`, which the API answers with HTTP 403, field `location` and the sentence "Too far to vote. You need to be within 1 km of this report." The sentence is the same for all four vote values the rule covers, and the response never contains the computed distance.

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

The panel sentences are written by the server. T is `IndependentAgreementThreshold`. They say people at different spots and never the word place (A88).

| Line | Case | Text |
|------|------|------|
| Lead | Provisional, nobody has confirmed | `Needs {T} people at different spots to confirm it.` |
| Lead | Provisional, T minus N is 1 | `Needs 1 more person at a different spot to confirm it.` |
| Lead | expired, not retracted (Activity page) | `Expired, so it can no longer be confirmed.` |
| Confirmed | N is 0 | `No one has confirmed this yet.` |
| Confirmed | N is 1 | `1 person confirmed this.` |
| Confirmed | N is 2 or more | `Confirmed by {N} people at different spots.` |
| Disputed | M is 0 | `No one has disputed this.` |
| Disputed | M is 1 | `1 person nearby disputed this.` |
| Disputed | M is 2 or more | `{M} people at different spots disputed this.` |

The lead follows the confirming count. A Provisional report has fewer than T confirming places, so it needs T people, or 1 more person after one place; a larger threshold would read `Needs {K} more people at different spots to confirm it.`. The words changed in amendment A88 and no line was hidden: every sentence that was filled before is still filled in the same states. Since plan 03-31 these sentences are the wire's older fields (`lead_sentence`, `confirmed_sentence` and `disputed_sentence`). The trust card shows the same facts as the rows of `facts`, described under "One trust object on every surface" below.

The tests that pin this section are `TestBuildTrustCountsMatchTally`, `TestTrustCountLabel`, `TestTrustCountsDistinctPlacesNotVotes` and `TestTrustCountUnderConcurrentVotes`. The last one casts votes from many places at once and needs a disposable database. The wording itself is pinned by `TestTrustCounterSentences`, `TestPanelWordingEveryState`, `TestPanelWordingNamesNoPlace`, `TestPanelWordingStaysFilled` and `TestPanelWordingSingularAndPlural`.

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
| 75 or more (`UpToDateMinScore`) | Up to date | Report up to date |
| 50 to 74 (`GettingOldMinScore`) | Getting old | Report getting old |
| 49 or less | Needs re-confirming | Report needs re-confirming |

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
- The feed row shows only the word. The popup shows the score as the score line of the still current row, "Score {SCORE} out of 100." (D-09, A50, A92).

### What it costs to fake

One account re-tapping Confirm moves only its own place's time, so it cannot refresh the signal without a second, genuinely different place (D-05). Faking Up to date costs 2 verified accounts and 2 claimed locations, the same as forging Live. A spoofed location defeats it exactly as it defeats the place rule. See "Honest limits" below.

The tests that pin this section are `TestCurrencyScoreBoundaries`, `TestCurrencyNotYetCorroborated`, `TestCurrencyRetapCannotRefresh`, `TestCurrencyDisputes`, `TestCurrencyWindowFollowsLifetime`, `TestCurrencyMonotone`, `TestFadeStageMapping` and `TestTrustCopyCurrencyIsPlainText`, rerun with `go test ./internal/service/ -run 'TestCurrencyScoreBoundaries|TestCurrencyNotYetCorroborated|TestCurrencyRetapCannotRefresh|TestCurrencyDisputes|TestCurrencyWindowFollowsLifetime|TestCurrencyMonotone|TestFadeStageMapping|TestTrustCopyCurrencyIsPlainText' -count=1`.

## Reporter reliability

Every report also carries a slow signal beside the still current one: is the person who posted it a reliable source? It is built from how that person's earlier reports turned out. The function that composes it is `ReporterSignals` in `internal/service/reporter.go`. It decides no rule itself. `ClassifyOutcome` and `ScoreReliability` in `internal/service/reliability.go` own every rule, and `internal/service/trustcopy.go` owns every word.

### Who is rated

The rating is public and person level. It is keyed to the verified account through `sessions.account_id`, across every session the account has used, and never to a session. Someone who signs in on a second phone keeps one record.

The tag and the counts behind it are visible to everyone on every report, and the popup also shows the score line. The account id and the email are never shown. The signal carries only the tag, the score, the two counts and server written sentences.

Accountless reports, and official pins in a later phase, have no rating. They carry the tag `none` and show nothing (A47). A feed read rates at most 200 reporter accounts, taken from the nearest reports first, and reports by accounts past that cap also carry none.

The history is read in one query per request for all reporters on the page, backed by the index `idx_reports_session_expires`, with one batch of current votes beside it. `ReporterSignals` groups those votes by report once, so the work grows with the history rows plus the vote rows and not with their product.

### Outcomes

Each finished report gets one outcome from `ClassifyOutcome`. It asks `Resolve` itself, with neutral metadata (low severity, other category) and the tally's resolve and reopen fields zeroed. Live maps to corroborated, Hidden to contradicted and Provisional to unconfirmed. No threshold is copied, so the rating can never disagree with the visibility ladder.

For each history report, the reporter is the account being scored, passed to `BuildVoteTally`. So the reporter's own confirmations never count, and their own resolve or reopen cannot erase a record. A report its reporter marked resolved is still judged by the content votes of other people.

A critical or rescue needed report that expires unconfirmed is neutral, because those reports skip the Provisional gate and an unconfirmed one says nothing about the reporter (D-12). A critical or rescue needed report that independent places out dispute counts as contradicted for the rating only. On the read path it is still never Hidden (R-06).

Phase 3.1 reuses this classifier and adds no rule to it:

- The take back changes no rating. `ClassifyOutcome` clears all four resolution fields (the resolve and reopen counts and the two reporter flags) before it asks `Resolve`, so a report is judged by its content votes alone, whatever resolution votes it holds. A resolve, a reopen and an `unresolve` therefore cannot move an outcome or a score. `TestTakeBackLeavesRatingsUnchanged` appends take backs to a history and shows the outcomes and the score are equal.
- The Past tab's labels come from `ClassifyOutcome`, so a history label can never disagree with the rating. A corroborated report reads "Confirmed by {N} people nearby", an unconfirmed one "Expired unconfirmed", and the neutral outcome reads plain "Expired". A critical or rescue needed report that expires with no clear result is neutral, so it reads "Expired" and not as a failure. A critical or rescue needed report that people out disputed comes back contradicted, so the Past tab says "Disputed" for it (D-10, a recommended default the owner may veto at UAT, in line with R-06). `TestHistoryOutcomeLabels` pins the words.
- The partition of reports into tabs is the pure function `ViewOf`, the only place it is written. Live and Provisional reports that have not expired are Active, a Hidden report that has not expired is Disputed, a Retracted report is Resolved at any expiry, and an expired report that was not Retracted is Past. A visibility it does not know is in no tab, so a state added later cannot leak into one. This amends Phase 2 D-12, which had a Retracted report leave every view: it now stays visible as history in the Past tab, in its Resolved group, and never appears in Active or in its counts (D-25). The time a report was closed and who closed it, the poster, people nearby or both, are read by `ClosureOf`, which replays the standing resolution rows through the same `isRetracted` rule `Resolve` uses, so the closure can never disagree with the state a report reads. `TestViewOf` and `TestClosureOf` pin both.

- Under Phase 4 R-02, a held report (pending, not serious) and a removed report are neutral whatever their votes: the rating never counts a report for or against its author because of a moderation outcome, and `ClassifyOutcome` returns neutral first when `ReportMeta.Withheld()` is true.

### What counts as judged

A report is judged once its expiry time is at or before now, whether or not it was resolved (R-05). A report still in flight is neutral. Neutral outcomes are not judged.

At least 5 judged reports (`ReliabilityMinJudged`) are needed, counted without weights, inside the window and the cap. Below that the reporter is a New reporter with no number and never a default score. The real judged count is still reported in `reporter.judged_reports`. The sentence does not show it: a New reporter reads that they are not rated yet and how many reports a rating needs.

### Window, half life and the 30 report cap

Only reports that expired within the last 90 days count (`ReliabilityWindow`). A report exactly 90 days old is out, and one that expires exactly now is in. Age is measured from the expiry time, and each outcome's weight halves every 30 days (`ReliabilityHalfLife`).

The score uses the latest 30 judged reports per account (`ReliabilityRowCap`), across all of its sessions, newest expiry first. Neutral reports are dropped first and the cap of 30 applies to what is left, so a run of neutral reports cannot push a judged record out of the history. That is a decision, and the reasoning is this:

- The earlier version capped the latest 30 expired reports and dropped neutral ones afterwards. A critical or rescue needed report that expires with fewer than 2 confirmations and fewer than 2 disputes is neutral, and critical reports publish Live at once, so an account with a poor record could file 30 of them and see the poor record replaced by "New reporter" (review finding WR-05).
- SQL cannot know which reports are neutral without their votes. The read therefore returns up to 100 expired reports per account (`ReliabilityHistoryReadCap`), and Go classifies them, drops the neutral ones and keeps the latest 30 judged.
- The read cap and the Go cap are separate numbers on purpose: 100 leaves room for neutral rows, 30 is the number of judged reports the score uses.
- `ScoreReliability` applies the window, drops neutral rows and caps at 30 judged reports on whatever rows it receives, so it gives the same answer on raw rows as on the capped read unless more than 100 of the newest rows are neutral.
- The limit that remains: 100 neutral reports in a row can still push an old record out of the read, so the Phase 4 rate limits are what bound that.
- The test case that shows 30 newer neutral reports no longer pushing 5 corroborated ones out, and a poor record surviving the same run, is in `TestReporterSignalsNewAndNone`.

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

New reporter never looks like a good tag. It is the same plain text as the others, with no check, star or colour. The feed row and the Activity page show the tag as plain words beside the state label, at the top of the report and outside the trust block's button, alone on that line when the report has no state label (03-UI-SPEC.md A75). The popup shows the tag as the title of the last row of its card, with the reporter sentence as the detail and, when the score is not null, the score sentence as the score line (A91, A92). The panel of a row shows the same reporter row without the score line.

The templates, in the server's exact words:

- `{C} of their last {J} reports were confirmed.` for a rated reporter, `was confirmed` when C is 1
- `Not rated yet. It needs {MIN} reports with a clear result.` for a New reporter, whatever J is
- `Score {SCORE} out of 100.`

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

The tag appears on every report, including critical and rescue needed ones, as the same plain text. It never hides, dims, blocks, reorders or filters anything (D-14). A Hidden report in the Disputed tab shows the tag and no still current word. A later triage phase may use it as one soft input.

### How the roadmap word score is met for the reporter

The roadmap asks for a slow decaying "is this source reliable" score (TRUST-07, D-08).

- It is an integer from 0 to 100 per reporter, from the documented formula, recomputed on every read and never stored.
- It decays with a 30 day half life inside 90 days. That is the slow half beside the fast still current score.
- It is null for a New reporter, and a later phase treats null as unknown.
- The feed row shows the tag. The popup shows the score as the score line of the reporter row (D-09, A50, A92). Both signals show at once on every report without opening anything: the still current word and the reporter tag beside the state label on rows and the Activity page (A75), and the state word and the last row of the always showing card in the popup (A91, A92).

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
| `facts` | array | The ordered tick rows of the trust card, never null. Kinds in order: `status`, `confirmations`, `disputes`, `currency`, `reporter`. See "The facts list" below. |
| `facts[].kind` | string | `status`, `confirmations`, `disputes`, `currency` or `reporter`. |
| `facts[].tone` | string | `good`, `neutral`, `caution` or `bad`. It picks the row's icon. |
| `facts[].title` | string | The bold line of the row. Always set. |
| `facts[].detail` | string | The muted line under the title. Empty when the row has none. |
| `facts[].score_detail` | string | The score line, "Score {SCORE} out of 100.". Empty when there is no score. Only the map popup shows it. |

What never leaves the server: the account id, the email, the voter's place, the session id and any raw vote total. `TestTrustResponseOmitsIdentifiers` checks the live response, and `TestSwaggerSpecCoversRoutes` checks the documented field names against the same rule.

The same object reaches every surface, and these tests prove it: `TestTrustAttachedOnEveryReadPath`, `TestFeedTrustMatchesCastVoteTrust`, `TestFeedSignalsMatchCastVoteSignals`, `TestCastVoteTrustMatchesNearbyTrust` and `TestProfileRowsCarryTrustJSON`.

Cost bounds: a feed read makes one batched vote read and one history read, and rates at most `MaxReporterAccountsPerRead` reporter accounts, nearest first. Reports past the cap read `none`. `TestNearbyBatchesVoteReadsOnceWithReporters` and `TestNearbyReporterAccountCap` pin this.

Display only: neither signal hides a report, dims it beyond the time fade or reorders it. Critical and rescue needed rows keep the time fade only (D-14). `TestNearbyRatingNeverChangesVisibilityOrOrder` and `TestFadeUsesCombinedStage` pin this, and `TestTrustModuleHasNoThresholdsOrSinks` pins that the browser holds no threshold of its own.

### The facts list

`facts` is the list of rows the trust card shows. A row has a bold title, an optional muted detail line, an optional score line and a tone. The map popup always shows the rows. The feed row and the Activity row show the same rows when their panel is opened. `buildTrustFacts` in `internal/service/trustfacts.go` builds the list once, inside `BuildTrust`, so the feed, the vote response and the Activity row carry the same list for every viewer (plan 03-31, amendments A89 and A90 in `03-UI-SPEC.md`). The field is additive: every other key and every sentence above is unchanged.

The kinds, always in this order:

1. `status`, only for an expired report that is not resolved. A resolved report reads as resolved and not as expired, the same precedence as the lead sentence.
2. `confirmations`, always.
3. `disputes`, always.
4. `currency`, exactly when the still current signal shows a word. It is left out for a Hidden, Retracted or expired report, and for a non critical report that is not yet corroborated, which is the Provisional case.
5. `reporter`, for every tag except `none`.

So the list always holds the two counter rows, also at 0 confirmed and 0 disputed. A row that does not apply is left out and never sent empty.

The tone is `good`, `neutral`, `caution` or `bad`. It is the server's pick, and the browser turns it into a glyph: a check, a dash, an exclamation or a cross. Each tone has its own shape, so colour is never the only cue, and the row text carries the meaning.

The server writes every word, so the browser holds no wording, threshold or rule for a fact. The strings are built from constants and integers only, so no report text can enter them. They use the same closed character set as the other copy, and none of them contains the word place. Two rules keep the facts from drifting away from the signals:

- The currency and reporter rows carry the signals' own sentences. Their `detail` is `currency.sentence` or `reporter.sentence`, and their `score_detail` is the score sentence. Nothing is recomputed. A dispute that raised the bar is read from the signal itself, because its sentence is not the plain sentence of its band. The one exception is the Gone quiet row, whose detail drops the words its title already says.
- The count rows use the same N and M as the counters. A title carries the real number even above 99 ("Confirmed by 120 people"), while `confirmed_text` stops at "99+". The "Needs" detail of a confirmations row is left empty for a Hidden, Retracted or expired report, because nobody can act on it.

The score line follows D-09 and A50: only the popup shows it, and it is empty exactly when the score is null. The old sentence fields (`lead_sentence`, `confirmed_sentence`, `disputed_sentence` and the sentences of the two signals) stay on the wire unchanged for the API, and the Phase 4 Low data view should read `facts` instead. In this table T is `IndependentAgreementThreshold`, MIN is `ReliabilityMinJudged`, W the window phrase, R the confirming places inside the window, K the number of confirmations a standing dispute makes necessary, C the corroborated and J the judged reports.

| Kind | Case | Tone | Title | Detail |
|------|------|------|-------|--------|
| `status` | expired and not resolved | neutral | Expired | `It can no longer be confirmed.` |
| `confirmations` | N is 0 (the detail is empty for a Hidden, Retracted or expired report) | neutral | Not confirmed yet | `Needs {T} people at different spots.` |
| `confirmations` | N is 1 (the detail is empty for a Hidden, Retracted or expired report) | neutral | Confirmed by 1 person | `Needs 1 more person at a different spot.` |
| `confirmations` | N is 2 or more | good | Confirmed by {N} people | `At different spots nearby.` |
| `disputes` | M is 0 | good | No disputes | `No one nearby has disputed this.` |
| `disputes` | M is 1 | bad | Disputed by 1 person | `Someone nearby says this is wrong.` |
| `disputes` | M is 2 or more | bad | Disputed by {M} people | `At different spots nearby.` |
| `currency` | Report up to date | good | Still happening | `{R} confirmations in the last {W}.` |
| `currency` | Report getting old, no standing dispute raised the bar | caution | Getting old | `Fewer than {T} confirmations in the last {W}.` |
| `currency` | Report needs re-confirming, no standing dispute raised the bar | bad | Gone quiet | `People nearby can confirm it again.` |
| `currency` | Report getting old or needs re-confirming, and a standing dispute raised the bar | caution | Needs more confirmations | `Disputed nearby, so it needs {K} confirmations in the last {W}.` |
| `currency` | Too early to tell | neutral | Too early to tell | `Fewer than {T} people have confirmed this yet.` |
| `reporter` | Reliable | good | Reliable reporter | `{C} of their last {J} reports were confirmed.` |
| `reporter` | Mixed | caution | Mixed record | `{C} of their last {J} reports were confirmed.` |
| `reporter` | Unreliable | bad | Unreliable reporter | `{C} of their last {J} reports were confirmed.` |
| `reporter` | New | neutral | New reporter | `Not rated yet. It needs {MIN} reports with a clear result.` |

The currency and reporter score line is `Score {SCORE} out of 100.` when the score is not null. The tests that pin the list are `TestTrustFactsEveryState`, `TestTrustFactsRulesOverTheGrid`, `TestTrustFactsPlurals` and `TestTrustFactsAreDeterministic`, and `TestTrustCarriesFactsOnTheWire` shows that the feed and the vote response carry equal lists.

## Browser hints, server decisions

The server decides every vote. The browser only helps the voter get a fresh location to the server and shows a hint.

- The refusal: a vote from beyond 1 km gets HTTP 403, field `location`, and the sentence `TooFarMessage()` returns: "Too far to vote. You need to be within 1 km of this report." The body never holds a distance. The distance itself comes from `haversineKm` and the comparison from `withinVoteRadius`. `TestCastVoteTooFar` pins the response.
- The silent refresh (D-02): a cached fix younger than 60 seconds is reused. Otherwise, if location permission is already granted, the browser takes one silent fresh fix. After a refusal with field `location` it retries once with a fresh fix. It never prompts for each vote. If a new fix cannot be taken, an older cached fix is reused only while it is younger than 10 minutes (`VOTER_LOCATION_STALE_MAX_MS`); an older one, or one with no capture time, is not used and the voter sees the location sentence. `TestVoteTransportRefreshesAndRetriesOnce` pins the silent refresh. The reporter's own Mark resolved and Reopen do not ask for a location at all, because the server exempts them from the 1 km rule; the browser sends a fresh cached fix if it has one and placeholder coordinates otherwise (`TestOwnReportResolutionSkipsLocation`).
- The too far hint (D-22): the page carries `data-vote-radius-km`, which equals `MaxVoteDistanceKm` (`TestPageShellCarriesVoteRadius`). With a cached fix younger than 120 seconds, Confirm, Dispute and Mark resolved on a report farther than that radius get `aria-disabled` and the same sentence as visible text. The reporter's own Mark resolved stays enabled, and a stale fix never disables a button. A tap on a marked button sends no vote: it takes one fresh fix, recomputes the mark from the coordinates that fix resolved with, and goes on with the vote only if the button is no longer marked, sending those same coordinates and taking no second fix. A coarse fix that is not cached (`VOTER_LOCATION_MAX_CACHE_ACCURACY_M`) still serves that tap, so a voter who has walked closer is not locked out by an older far fix. `TestVoteHint` pins this.

The hint is advisory. A forced vote is still refused by the server.

## Show menu: Confirmed

The Show menu on the Nearby list has a Confirmed choice. Which reports are in it is decided by the server and sent as the field `confirmed_by_people` on every report in the feed. The browser reads that field and shows it. It never counts confirmations and it is never sent a threshold.

A report is in Confirmed when all of these are true:

- It is active: live, not retracted, not expired, not disputed, not held and not removed.
- People at `IndependentAgreementThreshold` different spots, 2, have confirmed it. This is the same spot count the trust object shows, so Confirmed and "Confirmed by N people" cannot disagree.
- The disputes are not a majority. A majority means at least 2 spots disputed it and more spots disputed it than confirmed it.

The Critical case is the one to read twice. A Critical report is shown at once, with no waiting for confirmation, and disputes cannot hide it. That is the critical bypass in `Resolve`, and it decides showing, not being confirmed. So an unconfirmed Critical report is in the All list but is not in Confirmed, and its `confirmed_by_people` is false. A Critical report that people have confirmed is in Confirmed, unless the disputes are a majority; then it stays visible but is not listed as confirmed. Reports stored with the older rescue needed category follow the same rule. The choice that a confirmed Critical report belongs in Confirmed is a default waiting for the owner's answer; changing it is the one line in `ConfirmedByPeople`.

For a report the bypass does not touch, Confirmed is the same thing as the resolver's live because confirmed answer. `TestConfirmedByPeopleAgreesWithResolveForNonCritical` checks that over a grid of confirm and dispute counts, and `TestConfirmedByPeople` has a row for every rung of `Resolve` and for the Critical rows. `TestFeedViewAll` shows the field on the wire. `ConfirmedByPeople` is never read by `Resolve`, `ViewOf` or `BuildTrust`, so it changes no count and no visibility.

The All choice is a separate read, `view=all`, which returns the active and the disputed reports together, each with its own view word, so the list can put disputed ones last. It never returns a retracted, expired, held or removed report (`TestWithheldReportsInNoFeed` and `TestFeedViewAll`).

## Where the roadmap words are met

| Roadmap words | What ships | Where in code | Test |
|---------------|------------|---------------|------|
| "confirmed by N nearby" | The counter `N confirmed`: N distinct places, every vote cast from within 1 km | `BuildTrust`, `CastVote` | `TestBuildTrustCountsMatchTally`, `TestCastVoteRejectsTooFar` |
| deliberately chosen, documented geohash precision | Precision 7, with the reasoning in "What counts as a place" | `voterGeohashPrecision` | `TestGeohashPrecisionWitnesses` |
| not the raw number of votes | Distinct places, so repeat votes and a crowd in one place count once | `BuildVoteTally` | `TestTrustCountsDistinctPlacesNotVotes` |
| two distinct trust signals side by side | The still current word and the reporter tag, both shown at once on every report without opening anything: beside the state label on rows and the Activity page (A75), and as the state word and the last row of the popup card (A91, A92) | `BuildTrust`, `createStateLine` | `TestFeedSignalsMatchCastVoteSignals`, `TestTrustBlockMountedOnBothSurfaces`, `TestTrustStateLineMarkup` |
| fast decaying still current confidence score | `currency.score` | `buildCurrency` | `TestCurrencyScoreBoundaries` |
| slow decaying source reliable score | `reporter.score` | `ScoreReliability` | `TestReliabilityTable`, `TestReliabilityDecayAndWindow` |

How the word score is met (D-08, TRUST-07), for both halves: each score is an integer from 0 to 100 (`currency.score` and `reporter.score`) from a documented formula over real votes. It is recomputed on every read and never stored. The band or tag is cut from the same integer, so the word and the number cannot disagree. A score is null when there is no basis for one: fewer than 2 confirming places over the life of the report, or `not_applicable`, for the still current score, and New reporter or none for the reporter score. The numbers show on screen in the map popup only (D-09), and Phase 6 can sort on them server side. The two subsections called "How the roadmap word score is met" in the still current and reporter sections give the detail, so it is not repeated here.

The visible form of "confirmed by N nearby" is the counter `N confirmed`, with the confirmations row of the trust card, titled "Confirmed by {N} people" with the detail "At different spots nearby.", naming who is counted. The older sentence "Confirmed by {N} people at different spots." stays on the wire as `confirmed_sentence`. That is the form CONTEXT records for the first roadmap criterion.

## Hand offs to later phases

- Phase 03.1 reuses `ClassifyOutcome` as a pure function, so history labels cannot disagree with the rating. What it built, and what later phases reuse: `ViewOf` is the one tab partition and `ClosureOf` the one closure replay, both pure functions of values with no I/O; `CoarsenFinishedRescue` is the one blur of a finished Rescue report; and `unresolve` with `UndoLimit` is the one take back. A new read path calls these and does not rewrite them.
- Phase 03.2 builds update wording from the same trust object. It should treat the Reliable reporter tag as farmable at the price stated in "Honest limits": extra verified accounts plus a claimed location. It opens a report by id from a link, so it reuses the single report read of Phase 3.1, which already applies `CoarsenFinishedRescue`, and never builds a second path that could forget it.
- Phase 4 owns vote rate limiting, and its moderator view of a report names the tab with `ViewOf` and adds its own withheld check around it. The take back is a fifth vote route, so a Phase 4 limit written for four vote routes has to name it too.
- Phase 5 demo mode has to handle far away visitors, who cannot vote on seeded reports under the 1 km rule. It should place simulated reports near the visitor or exempt demo reports. It may seed clearly labelled demo accounts to show a rated reporter. Its new tabs go beside the three of Phase 3.1, and official pins ignore the partition `ViewOf` makes.
- Phase 6 may sort on the integer scores server side. It must never use them to bury a critical or rescue needed report (D-14), and must never bake a count or a rating into a shareable card image.
- Phase 8's report page reuses the trust block unchanged and places the reporter words with the same helper as the other surfaces, `createStateLine`. It and Phase 9's followable profiles show finished reports, so they call `CoarsenFinishedRescue` through the shared read and never copy a report's coordinates or description by another route.
- Phase 4's Low data view should read `facts` for the rows of its trust block, not the older sentence fields. The server already writes every word of a row.

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
| `ReliabilityHistoryReadCap` | `internal/service/reliability.go` | 100 |
| `UnconfirmedCredit` | `internal/service/reliability.go` | 0.25 |
| `ReliableMinScore` | `internal/service/reliability.go` | 75 |
| `MixedMinScore` | `internal/service/reliability.go` | 35 |
| `MaxReporterAccountsPerRead` | `internal/service/report.go` | 200 |
| `UndoLimit` | `internal/service/takeback.go` | 2 |
| `ApproxGeohashPrecision` | `internal/service/coarsen.go` | 6 |

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
- The server does not read GPS accuracy and does not look for mock locations. The browser reads accuracy only to decide whether a fix may be cached.
- Phone GPS error of tens of metres can put an honest voter standing near the 1 km line on the wrong side of it. Accuracy is not sent with a vote. The browser asks for a high accuracy fix. If that times out or no position is available it asks once more at the default accuracy, and only a refusal of permission ends the request at once. If both attempts fail the vote is still turned away with the location sentence, because no substitute position is ever used. A fix whose reported accuracy is worse than 300 m (`VOTER_LOCATION_MAX_CACHE_ACCURACY_M`) is used for that one vote but never cached, so it cannot be reused for later votes.
- A coarse location fix, hundreds of metres to over a kilometre (for example a laptop on Wi-Fi), is sent as it is. One physical spot can then look like several places, and a voter can be pushed across the 1 km line. This is why the place count is a rough measure and not proof of independent people. The system does not detect or block this.
- Counters, bands and fades update on the 30 second poll, so a screen can be one poll behind the server.
- Report age and the fade use the server's clock, taken from the Date header of each poll response, so a wrong clock on the phone does not change them. The header has one second resolution.
- The too far hint in the browser is advisory. Only the server decides whether a vote counts.
- A finished Rescue report is blurred, and the blur has a limit. Once a Rescue needed report is closed or has faded, `CoarsenFinishedRescue` replaces its latitude, longitude and geohash with the centre of the precision 6 cell (`ApproxGeohashPrecision`, about 1 km across) and empties its description and its distance, all four together in the one function, because blurring only some of them gives the spot back. The blur is the same for every viewer, so one reader cannot see more than another (the distance is the only value that depends on the viewer, and it is measured to the blurred point, never to the exact one). Flood, Shelter and Power reports and live Rescue reports stay exact. The Activity page applies the same blur to a voted card of someone else's finished Rescue report, and the reader's own posts stay exact there (D-27, a recommended default the owner may veto at UAT). The limit: anyone who read the report while it was live already saw the exact point and the text, and the blur takes back nothing from them. It only stops a finished report from handing the spot to the next person who looks. `TestFinishedRescueNeverLeaksOnPastOrLink` checks the raw response from two centres, and `TestFinishedRescueCoarsenedOnEveryPath` checks the service paths.
- Rescue needed is no longer offered for a new report (2026-10-16, owner decision): someone who needs rescue and has a phone calls for help, and Traffic took its place in the form. A post that still sends it gets 400 with `RescueNeededRetiredMessage`, checked by `TestSubmitRefusesRetiredRescueCategory`. Reports already stored as rescue needed keep every rule this document gives them until they fade: the critical bypass, no fade stage, the neutral outcome when unconfirmed, and the finished blur through `CoarsenFinishedRescue`. Traffic follows the ordinary rules.
- Parallel take backs from one account can pass the two undo limit. The undo count is read before the write and the votes log has no lock by design, so every request that is in flight while the count is still below the limit passes the check and writes its own row. The overshoot is therefore the number of extra parallel requests the caller manages to send, not just one: N parallel requests can leave N rows where two were allowed. Interleaved `resolve` and `unresolve` requests can also take a resolve back more than twice, because a resolve is never limited and a late take back that passed its checks earlier still writes. The cost stays small. No rating moves, because an `unresolve` row enters no bucket and `ClassifyOutcome` clears all four resolution fields before it classifies, and no other person's vote is touched. The effect is that one account can flip its own closed mark on and off more often than the limit says, and any later take back is refused once the count shows the limit is used. This was accepted over adding a lock to an append only log (T-31-09). If it ever matters, the fix is either a `pg_advisory_xact_lock` on the account and report before the standing resolve check, or one conditional insert that counts the spent undos inside the same statement as the write and maps zero rows to `ErrUndoLimitReached`.

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
