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

Order of checks inside `CastVote`: input validation, report lookup, the block on content votes for your own report, expiry (an expired report is refused as expired even from far away), the refusal of a vote that carries no coordinates (excused for the reporter's own resolve and reopen and for the take back), distance, for a take back the standing resolve check, then for a take back or the poster's own reopen the undo limit check (all before any write), then for a content confirm one read of the current votes that decides whether the report is still live enough to extend, then the write. The write appends the vote and, for a confirmation from a new spot on a live report, moves the fade time in the same statement.

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

The window is the report's normal lifetime, `ExpiryDuration` for its severity, divided by `CurrencyWindowDivisor` (4). That is 2 hours for an 8 hour report and 6 hours for a 24 hour report (D-05). A confirmation from a new spot can push the fade time past the normal lifetime (see "A confirmation from a new spot keeps a report alive"), and an extension never widens the window: the lifetime it divides is capped at `ExpiryDuration`. A report seeded with a shorter life keeps its own shorter window. The window itself is exact. The window length sent to the browser is rounded to the nearest whole minute.

`ExpiryDuration` depends on severity only. A low severity rescue needed report therefore lives 8 hours (before any confirmation from a new spot) and gets a 2 hour window, even though the critical bypass treats it as critical elsewhere.

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
- It decays one point per started 1/25 of a window and reaches 0 over the report's normal lifetime. That is the fast decaying half. The reporter half is slow.
- It is null below 2 places, and Phase 6 treats null as unknown.
- The feed row shows only the word. The popup shows the score as the score line of the still current row, "Score {SCORE} out of 100." (D-09, A50, A92).

### What it costs to fake

One account re-tapping Confirm moves only its own place's time, so it cannot refresh the signal without a second, genuinely different place (D-05). Faking Up to date costs 2 verified accounts and 2 claimed locations, the same as forging Live. A spoofed location defeats it exactly as it defeats the place rule. See "Honest limits" below.

The tests that pin this section are `TestCurrencyScoreBoundaries`, `TestCurrencyNotYetCorroborated`, `TestCurrencyRetapCannotRefresh`, `TestCurrencyDisputes`, `TestCurrencyWindowFollowsLifetime`, `TestCurrencyMonotone`, `TestFadeStageMapping` and `TestTrustCopyCurrencyIsPlainText`, rerun with `go test ./internal/service/ -run 'TestCurrencyScoreBoundaries|TestCurrencyNotYetCorroborated|TestCurrencyRetapCannotRefresh|TestCurrencyDisputes|TestCurrencyWindowFollowsLifetime|TestCurrencyMonotone|TestFadeStageMapping|TestTrustCopyCurrencyIsPlainText' -count=1`.

## A confirmation from a new spot keeps a report alive

A report fades at the time it was given when it was posted, unless someone at a new spot confirms it (backlog 999.12, decided by the owner on 2026-10-18). This is what makes the product words "reports auto-fade unless re-confirmed" true. The rule is one pure function, `ExtendedExpiry` in `internal/service/expiry.go`, and one SQL statement, `InsertVoteExtending` in `internal/store/queries/extend.sql`, that applies it in the same statement that writes the vote.

The rule, in seven parts:

1. Only a content confirm can extend. A dispute never extends. A resolve, a reopen, Mark resolved and a take back are resolution votes and never extend.
2. A new spot means the voter's precision 7 cell has no earlier confirm on this report in the whole append only vote log, from any account other than the reporter. The log is the reference, not the standing votes, so a confirm, dispute, confirm toggle can never extend twice from one cell. The reporter's own rows never use up a spot, so a stranger standing where the reporter once confirmed still counts as a new voice.
3. One person extends at most once: the voter must have no earlier confirm on this report. This keeps the promise made under "Honest limits" that walking around cannot add places, and it is true for the fade time too. Each account holds one current vote in the count of "confirmed by N nearby", and it now moves the fade time at most once as well.
4. The new fade time is the confirmation's own time (the database clock, the same instant the vote row is stamped with) plus `ExpiryDuration` of the report's severity (`defaultExpiry` 8 hours, `criticalExpiry` 24 hours). It is never earlier than the current fade time, and never later than `ExpiryCap` after posting (`defaultExpiryCap` 24 hours, `criticalExpiryCap` 48 hours). Severity alone decides the numbers, the same as `ExpiryDuration` always has.
5. A report that has already faded is never revived.
6. Nothing ever shortens a fade time. Switching a confirm to a dispute is the only way to take a confirmation back, and it leaves the fade time as it is.
7. A report that is finished in any way, a resolved one included, is never extended. A finished report has no life left to extend, and extending it would delay the reliability judging, which waits for expiry. `CastVote` decides this in Go: before the write, a content confirm reads the current votes once, runs `Resolve`, and passes Extend false unless the report is live. The SQL holds no visibility rule, so Go stays the one visibility authority. A resolved report stays resolved and shows its closed line, and Mark resolved still ends a report at once. One limit is accepted: a confirm cast while the report is resolved is still written to the log, so after the reporter reopens it, that spot and that person count as having confirmed already and cannot extend the report (a new spot and a new person still can). The log is the reference, and `TestConfirmWhileResolvedUsesUpSpotAndPerson` pins it.

Why the cap exists. Without a cap, a group of accounts at different spots could keep a false report alive for ever by confirming it in turn. With the cap, the most any group can buy is 24 hours from posting, or 48 hours for a Critical report. The price is the same as for raising the count: verified accounts and claimed locations (see "Honest limits"). The cap bounds the damage and does not remove the price.

A dispute never extends, and taking a confirmation back never shortens. A dispute is a reason to doubt a report, so it must never be a way to keep one alive. A take back must not shorten the life, or one person could cut a report short by confirming and then switching.

Written with the vote, atomically. The vote and the move of the fade time are one statement, so they commit together or not at all. The only row the statement updates is the report's, under the lock any update takes, and `GREATEST` lets the fade time only move later, so concurrent confirmations never lose an extension. `TestConfirmExtensionUnderConcurrentVotes` fires eight confirmations from eight spots at once and checks the result is the latest vote time plus the lifetime. The vote answer is built from the fade time the write returned, never the one read before it, so the answer equals the feed (`TestCastVoteTrustMatchesFeedAfterExtension`). The votes table stays append only with no conflict clause.

The accepted races. Two confirms from one new spot in flight at the same instant can both count as new, because neither sees the other. The result is the later of their two times plus the lifetime, so the overshoot is at most the gap between them and never passes the cap. A resolve that lands between the pre write read and the write can still let that one confirm extend. The report stays resolved either way.

What moves with the one column. Every reader reads the one stored `expires_at`, so nothing else needed a rule of its own. The feed's read time expiry, the Past tab and Ended nearby, reliability judging (which waits for the new time before it counts the report), the phone alert sweep and its stale skip, the report page history, profiles and the lived for text all treat an extended report as live until its new time and as finished after it. No migration is needed. `TestExtendedExpiryOneColumnEveryReader` checks these readers on a real database against one extended report.

The still current window stays the normal lifetime over 4 (`ExpiryDuration` of the severity divided by `CurrencyWindowDivisor`: 2 hours, or 6 for Critical), so an extension never widens it and never loosens Up to date. `TestCurrencyWindowKeepsNormalLifetime` pins it. The browser's time fade measures the last quarter of the whole life, from posting to the current fade time, so a report confirmed again looks fresh again. The same sum has an effect in absolute time that is intended: a long extended report starts to look old sooner before its fade time. A Critical report whose fade time the cap sets at 48 hours after posting is shown so that the aging look starts with 12 hours left and the stale look with 6 hours left, where an unextended Critical report (a 24 hour life) goes stale only in its last 3 hours. The words on the card ("Fades in ...") always give the real time left.

Fading notes. A confirmation from a new spot on a report about to fade cancels that fade, so no fading note follows it. That is intended. When the report does enter its last hour again, the bell does not add another row: it keeps one fades in about an hour row for each person and report and refreshes it for the new fade time (it moves to the top and shows as unread again). The phone note goes out at most once for each person and report however many times the report is extended, because the change note is keyed on that one bell row and the refreshed row keeps its id (`TestFadingRefreshedInPlaceNoSecondPush`). A phone note is also sent only while the report still fades within the hour: a queued note about a report someone confirmed again since is skipped at send time. See "Phone alerts".

The popup's promise. The map popup's fade line ends with "unless confirmed again" only while the server's `confirm_extends` is true. `ConfirmExtends` in `internal/service/expiry.go` computes it from the report's own times and visibility: true while the report is live and before its cap, false for a finished report and once the report has reached the longest life it can have. It is display only and `Resolve` never reads it. It names no voter, account, cell or count.

The tests that pin this section are `TestExtendedExpiryRule`, `TestConfirmExtendsFlag`, `TestCastVoteExtendsOnlyOnConfirm`, `TestConfirmFromNewSpotExtendsExpiry`, `TestConfirmExtensionUnderConcurrentVotes`, `TestMemQuerierExtendsLikeSQL`, `TestExtendedExpiryOneColumnEveryReader`, `TestCastVoteTrustMatchesFeedAfterExtension`, `TestCurrencyWindowKeepsNormalLifetime` and `TestFeedAndFocusCarryConfirmExtends`. The Demo world applies the same rule through its in memory copy of the vote query, and `TestDemoParityExtensionWithSQL` checks it against the SQL on a report old enough for an extension to show (`TestDemoParityWithSQL` runs the same script on a fresh report, where an extension adds only milliseconds, so it cannot tell one from none).

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

So the boundaries 70, 35 and 25 are exact (6 corroborated and 2 contradicted is exactly 70, which is 7 / 10), equal ages give equal weights, and the order of reports never changes the result. `ScoreReliability` is the one function that does this.

### Tags and sentences

Tags are cut from the rounded integer. 70 or more (`ReliableMinScore`) is Reliable reporter, 35 to 69 (`MixedMinScore`) is Mixed record, and 34 or less is Unreliable reporter. Fewer than 5 judged is New reporter with no number.

Changed 2026-10-29 by Phase 9 (D-05): the Reliable reporter bar moved from 75 to 70.

Stars are the score divided by 20, floored to one decimal from integers (`StarsTenths` and `StarsText` in `internal/service/stars.go`, never a float format), so 69 reads 3.4 and 70 reads 3.5. The bar is 3.5 stars, by the owner decision of 2026-10-17 (D-05), chosen over the advice of 4 stars. The two lowest cut points read the same way: 35 points (`MixedMinScore`) is 1.75 stars and shows as 1.7, so 34 and 35 both read 1.7 because the Mixed record floor falls between two tenths. A test walks every score from 0 to 100 and checks that the stars text is 3.5 or more exactly when the tag is Reliable reporter, so the number and the word cannot disagree. The tag is still cut in exactly one place, `reporterTagForScore`.

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
| 4 corroborated, 1 unconfirmed (5) | 0.85 | 75 (5.25 / 7, no longer a boundary) | Reliable |
| 4 corroborated, 1 contradicted (5) | 0.80 | 71 | Reliable |
| 6 corroborated, 2 contradicted (8) | 0.75 | 70 (exact boundary, 7 / 10) | Reliable |
| 9 corroborated, 2 contradicted, 2 unconfirmed (13) | 0.731 | 70 (exact boundary, 10.5 / 15) | Reliable |
| 4 corroborated, 2 unconfirmed (6) | 0.75 | 69 (5.5 / 8 is 68.75, rounded up) | Mixed |
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
| 8 corroborated, 2 contradicted (10) | 0.80 | 75 (exact, no longer a boundary) | Reliable |
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

Amended 2026-10-30 by Phase 9: the tag now also gates two opt in things for the person who chooses them. Only a Reliable reporter can be followed, and a follow note (a quiet bell and phone note when a followed person posts) is produced and sent only while the tag still reads Reliable reporter. Neither touches what anyone else sees: no report is hidden, dimmed or reordered by the tag. See "Reporter profiles" and "Follows, mute, block and follow notes". From five raters on, tapped stars also feed the points the tag is cut from, see "Stars and tapped ratings".

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

What never leaves the server: the account id, the email, the voter's place, the session id and any raw vote total. `TestTrustResponseOmitsIdentifiers` checks the live response, and `TestSwaggerSpecCoversRoutes` checks the documented field names against the same rule. Phase 8 adds two more responses to the same rule: the report history (`TestTimelineResponseOmitsIdentifiers`) and the comment list (`TestCommentResponseOmitsIdentifiers`) carry no cell, no account id and no email. The comment list names a person only by a username that passed its screen.

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

The popup's fade line follows the same rule. It says "Fades in 7 hrs unless confirmed again." only while the feed's `confirm_extends` is true, and "Fades in 7 hrs." otherwise. The browser holds no lifetime, cap or rule of its own, and the server's word is the whole decision (`TestFeedAndFocusCarryConfirmExtends`).

## Show menu: Confirmed

The Show menu on the Nearby list has a Confirmed choice. Which reports are in it is decided by the server and sent as the field `confirmed_by_people` on every report in the feed. The browser reads that field and shows it. It never counts confirmations and it is never sent a threshold.

A report is in Confirmed when all of these are true:

- It is active: live, not retracted, not expired, not disputed, not held and not removed.
- People at `IndependentAgreementThreshold` different spots, 2, have confirmed it. This is the same spot count the trust object shows, so Confirmed and "Confirmed by N people" cannot disagree.
- The disputes are not a majority. A majority means at least 2 spots disputed it and more spots disputed it than confirmed it.

The Critical case is the one to read twice. A Critical report is shown at once, with no waiting for confirmation, and disputes cannot hide it. That is the critical bypass in `Resolve`, and it decides showing, not being confirmed. So an unconfirmed Critical report is in the All list but is not in Confirmed, and its `confirmed_by_people` is false. A Critical report that people have confirmed is in Confirmed, unless the disputes are a majority; then it stays visible but is not listed as confirmed. Reports stored with the older rescue needed category follow the same rule. The choice that a confirmed Critical report belongs in Confirmed is a default waiting for the owner's answer; changing it is a small edit to `ConfirmedByPeople` and the Critical rows of `TestConfirmedByPeople`.

For a report the bypass does not touch, Confirmed is the same thing as the resolver's live because confirmed answer. `TestConfirmedByPeopleAgreesWithResolveForNonCritical` checks that over a grid of confirm and dispute counts, and `TestConfirmedByPeople` has a row for every rung of `Resolve` and for the Critical rows. `TestFeedViewAll` shows the field on the wire. `ConfirmedByPeople` is never read by `Resolve`, `ViewOf` or `BuildTrust`, so it changes no count and no visibility.

The All choice is a separate read, `view=all`, which returns the active and the disputed reports together, each with its own view word, so the list can put disputed ones last. It never returns a retracted, expired, held or removed report (`TestWithheldReportsInNoFeed` and `TestFeedViewAll`).

## Show menu: Only Reliable reporters

The Show menu also has an Only Reliable reporters tick, for a reader who wants the Nearby list and the alerts to come only from people with a good record. Which reports the tick keeps is decided by the server and sent as the field `kept_by_reliable_filter` on every report in the feed and on a report opened from a link. `KeptByReliableFilter` writes it. The browser reads that field and never holds a tag rule or a threshold.

A report is kept when either of these is true:

- Its reporter's tag is Reliable reporter. The tag is the one on the report's own trust object, so the tick and the reporter line on the card cannot disagree.
- The report is serious. That is `ReportMeta.Serious`, the one critical rule: a Critical severity, or a report stored with the older rescue needed category. There is no second critical rule in this field, so a Critical report from a brand new reporter stays when the tick is on, and an old rescue needed report stays too.

What the field is not. The tick belongs to the reader's own choice, is not remembered between visits, and nothing reads the field by default. No default view hides, reorders or changes a report because of a reporter tag, so the display only rule for the tag (Phase 3, D-14) holds for every default view. A reader who turns the tick on is choosing to let the tag shape their own list, and only their own. `Resolve` never reads the field and the server still lists every report; the field filters nothing on the server.

Limits worth stating:

- The tag is read for at most `MaxReporterAccountsPerRead` reporter accounts in one read, 200. In a very dense read a far reporter past that count reads none, so a non serious report from a Reliable reporter may be left out of the tick. It errs toward showing fewer reports, never toward calling an unrated reporter reliable.
- The tag has a price, the same one "Honest limits" gives: extra verified accounts that confirm each other, plus a claimed location for each. A reader who turns the tick on takes that risk with it, which is why the tick is opt in and why Critical reports are kept whatever the tag says.

`TestKeptByReliableFilterGrid` pins the rule over every tag, severity and category. `TestNearbyCarriesKeptByReliableFilter` checks the field on a read, and `TestReliableFilterOnTheWire` checks it on the wire with a seeded Reliable reporter, a new reporter and an old rescue needed report, and that all of them are still listed.

## Reliable reporter alerts

A quiet note appears in the app's bell when a Reliable reporter posts near a reader's home area. This section says who gets one, through which gate, how often, in what words, and what has to exist before the rule is allowed to reach a lock screen or an inbox. It is the Reliable reporter tag acting on other people's screens, so it has to be written down more carefully than the tag on a card.

Who gets one. Since the owner decision of 2026-11-03 (quick 261103-jal) every signed in reader who has a home area saved gets the row, with no switch. It used to be opt in. The distance is the reader's own choice of 1, 2, 5 or 10 km, 5 km until they choose (`DefaultAlertRadiusKm`), measured from the middle of the home area's stored cell and not from the feed or from the device. The start point is the later of the stored time the old switch went on, when the account has one, and now minus `ReliableBellLookBack` (one hour), so a report older than an hour never raises the row and the bell is never back filled with old reports. The row never reaches a phone: it is shown in the bell only. The reporter never hears about their own report. A reader does not hear about a report they follow (that reader already hears about it through the ordinary updates) or one they muted.

The gate. One predicate, `MayNotifyAboutReport`, called with the Reliable alert kind over the resolver's answer for the report. Since the owner amendment of 2026-10-18 a report is eligible from the moment it is posted while it is listable, which is Live or Provisional, critical or not. It is never eligible while Hidden, Withheld, Retracted or expired. This replaced the earlier rule that a non critical report waited until it was Live, and it is safe only because a Provisional report already shows in the feed, dimmed and labelled, so the note never shows a reader more than the feed shows them. The reporter's tag must be Reliable reporter on the report's own trust object, the same tag the Show menu tick reads. The note is a pointer to the report and carries no position, no voter, no account and no place name; the browser sends no position for it either.

The caps. One alert per report per reader, ever (a repeat is a no-op in the database). `ReliableAlertsPerReporterPerHour` alert per reporter per reader per hour, so one reporter posting five reports in a minute gives each reader one note, and the oldest report is the one announced. And the ceiling of `UpdatesHourlyCeiling` updates an hour per person, over which the rest roll up into the "And N more changes" row. The Reliable row is outside that ceiling: it has its own cap of `ReliableHourlyCeiling` rows an hour, so a busy hour of them never stops a follow note or rolls a change row up. The hour is the server's, never the browser's.

The words. The event line is "Reported nearby by a Reliable reporter." The trust line is not stored. It is built from the report's current trust object every time the list is read, so it reads "Not confirmed yet." while people have not confirmed the report and reads the confirmed sentence once they have. The same row changes its words; a confirmation never adds a second row.

The price. The tag is farmable at the price "Honest limits" gives: extra verified accounts that confirm each other, plus a claimed location for each. A Reliable tag therefore grants reach to anyone willing to pay that price, which is why the alert is capped per reporter and worded as unconfirmed until people confirm it. It first reached only the in app bell; Phase 3.3 also pushed it to the phone under the caps in "Phone alerts". On 2026-11-03 the owner reversed both halves: the row is now shown to every account with a home area, without a switch, and it no longer reaches a phone at all. The widening is softened by the same per reporter cap, the hourly ceiling, the one hour look back and the Not confirmed yet wording.

The open dependency. Burst detection (TRUSTX-03) is not built. It was a hard dependency before this rule reached push or email, and it is met for the phone by not pushing this kind at all: the per reporter cap and the "Not confirmed yet." words soften the rule for the bell only, and do not stop several coordinated Reliable accounts from each reaching the same reader's bell. Because the row is now shown to every account with a home area, that bell exposure is wider than it was; nothing here is evidence that the rule is safe to widen further, and the dependency stays open for any change that would push this kind or send it by email (ROADMAP Phase 3.2 scope note 2).

`TestReliableAlerts` pins the gate and the exclusions over a fake store, `TestReliableAlertsOnRealHistory` runs the rule on a real database with a seeded Reliable reporter, and `TestReliableAlertOverTheBell` shows it over HTTP through the bell's own route, with the owner's at posting case in `TestReliableAlertAtPostingWhenNotCritical`.

## Phone alerts

Phase 3.3 lets a note reach a phone's lock screen. This section says who gets which note, through which gate, how often, in what words, what is never sent, and what is still missing. It is the trust rules of the feed and the bell acting on someone's pocket, so it follows the same two rules: a note is decided from the facts the feed itself uses, and it never says more than the feed would.

The kinds. Since the owner decision of 2026-11-03 (quick 261103-jal) Settings, under Notifications, holds two rows, Alerts on this device (the one switch) and How far from my home area. A device whose switch is on gets only what is really important, and the device switch is the only way to stop it. The stored per kind columns are kept in the database and no longer decide delivery.

Reaches the phone:

- Critical reports near my home area, within the chosen distance. It needs a home area.
- Severe official alerts for my area. Built by Phase 5, and it needs a home area. It is not a crowd report, so it does not go through `MayNotifyAboutReport` or the report outbox. See "Official alerts on the phone" below.
- A person I follow posts. Built by Phase 9. It has no switch in Settings: the Notify me bell on that person's profile is the switch, one person at a time, and Alerts on this device still governs whether any phone note arrives (D-11). It goes through the same `MayNotifyAboutReport` rule as the Reliable kind, with its own rows and its own words. See "Follows, mute, block and follow notes".

Stays in the bell, never on a phone:

- A Reliable reporter posts near me. This is the rule of "Reliable reporter alerts" above, now for every account with a home area. It refreshes bells early and queues no phone note.
- Changes and comments on reports I posted, voted on or commented on (see "Comments and likes"). The dispatcher refreshes the bells of the report's people early and queues no phone note.

A sixth note, the confirmed update, is not a kind of its own to the person. It replaces a Critical or follow note already on the screen. The confirmed update of a Reliable note sent before 2026-11-03 is skipped at send time.

What near means. Within the distance the person chose in Settings, 1, 2, 5 or 10 km, 5 km until they choose, measured from the middle of the stored home area cell, as for the Reliable alert. The picked point is never stored and the report's exact position is never sent to the phone.

The gate. A Critical or follow note is decided twice, once when it is queued and again just before it is sent, and both times from the report read fresh through `loadPushFacts`, which asks `Resolve` with the stored text check state set, exactly as the feed does. The predicate is `MayNotifyAboutReport`, called with `UpdateCriticalNearby` for a Critical note and `UpdateFollowedPosted` for a follow note. Since 2026-11-03 the worker also asks, in `kindSwitchOn`, whether a row of that kind may reach a phone at all: Critical and follow are yes, and Reliable, change and comment rows are never sent, so a row of those kinds that was queued before that date is skipped as not eligible. The worker's older checks for the change kinds (`eventStillHolds`, `reportWasConfirmed`) stay in place behind that answer as a second layer. They say what a change note once had to hold: the fades in about an hour note needed the report Live and still fading within the hour, the now confirmed note needed `ConfirmedByPeople`, the now disputed note needed the resolver to give Hidden and people nearby to have confirmed the report before, and the was marked resolved note needed Retracted. `TestReportWasConfirmed` pins the replay of the vote history and `TestWorkerHiddenNeedsEarlierConfirmation` shows on a real database that both of the old rows are now skipped and the bell keeps its row. `TestWorkerSkipsWhenNoLongerEligible` pins each skip. At send time the worker reads again the mute, the home area and the distance. So a report that was hidden, removed, retracted or has expired in between is skipped, and nothing is ever sent as a "sorry" about a note already shown. The author never gets a note about their own report, a report the person muted gives none, and a report with no reporter account on record is never pushed, because the per reporter caps below cannot count it.

A Critical report that most nearby people dispute. Disputes cannot hide a Critical report, so it stays Live and in the feed. Its phone note follows the feed: it is sent while the report is fresh, it still reads Not confirmed yet, and it never gets the confirmed update, because `ConfirmedByPeople` is false while the dispute majority stands. Push code has no dispute rule of its own and reads none (review note N3). `TestPushDispatchResolver` shows the resolver is the only judge.

A Critical report still waiting for its text check. It is pushed. The lock screen carries none of the reporter's words, only the pick and a coarse place, so there is nothing for the text check to catch, and a Critical report is published while its check retries. A report the check removes is skipped at send time like any other removed report.

The words. `BuildPushText` writes every word and has no input for the reporter's text, a name, a number, an address or a position.

- Since 2026-11-03 only the Critical, follow and official words reach a phone. `BuildPushText` keeps its Reliable, change and comment words for rows that were queued earlier; the worker skips those rows before it words them.
- Title: the pick, such as Water rising, then near and a coarse area name cut to 28 characters. A report with no pick uses its category and Critical. A Reliable note was titled with the category.
- Body: a distance in whole kilometres, or Under 1 km, measured from the home area and never from the reader's device, then from your home area. It ends with Not confirmed yet. until `ConfirmedByPeople` is true. A Reliable note opens with From a Reliable reporter.
- Once people nearby confirm, the same note is replaced in place, through its tag, and reads Confirmed by people nearby. at the start.
- A change note reads Your report, or A report you voted on, then is now confirmed, is now disputed, was marked resolved or fades in about an hour.
- A comment note is titled with the category and the area, such as Flood report near Benz Circle has a new comment, or has a new reply. It is fixed words only: no comment text, no username, no count. It follows the fading note's rule, so it is sent only while the report is active and Live (`MayNotifyAboutReport`) and does not also need the report to be Confirmed. A comment on a Provisional, disputed, resolved, expired or withheld report stays in the bell. Because a disputed Critical report stays Live, it can still send a comment note.

What is never sent: the description, the reporter's name or email, any number or street, any coordinate, the vote counts, the Reliable tag's score, the home area's name, or the voters. The area name is looked up once after the report is saved, from a coarse layer only, is kept for the lock screen alone, and appears in no feed, bell or vote response. Phase 8 amends 03.3 D-19 and D-20 for two surfaces only: the same stored name now also appears in the Share window title and on the public report page (see "Sharing"). No other response carries it. The site asks every note to make no sound and no vibration, and no text here or in the app promises either.

The confirmed update. It is queued once for each person who was sent the first note saying Not confirmed yet, and it is checked against the same home area and distance as the note it replaces. It goes to that person's devices except Apple ones, because iPhone and Safari do not replace a shown note by its tag and a second note would only stack. This is part of one notification per report on every device: an Apple endpoint gets only the first push about a report, so a later change note about it is not sent there either and stays in the bell, while other browsers replace the note in place by its tag. `deviceSkipsRepeatPush` is the one check, per account and report, and it only ever means fewer notes. `TestWorkerAppleFirstPushOnly` pins it.

The caps, in plain words. A person gets at most 4 notes in a rolling hour from the follow kind (and from any older Reliable or change row, which is skipped before it reaches the cap). A Critical alert near the home area always gets through, because it is never stored in the bell and a dropped one would show nowhere, but it counts toward the 4, so the follow kind is held back first. `heldByHourlyCap` is the one place that decides this, and `TestWorkerCriticalNeverHeldButCounted` pins it with follow rows. Confirmed updates are not counted and never held. One reporter's reports reach people for at most 3 Critical reports in a rolling hour across everyone, and the early bell refresh of a Reliable report keeps its own cap of 2 Reliable reports in an hour. A report counts once across both kinds and is never refreshed as Reliable once it was capped as Critical. A report is pushed to at most 2000 people, nearest first. A report that reached nobody does not use up one of its reporter's slots. When a note is dropped by a cap nothing extra is shown: no summary note, no and N more. The bell still lists everything it stores, which is every kind but the Critical alert. `TestPushCaps` pins the reporter caps, and `TestPhoneAlertCapShowsNothingExtra` shows five Critical reports near one person giving five notes, then the next push, a follow note, held back, with nothing extra shown.

The open gap. Burst detection (TRUSTX-03) is not built. This is the dependency "Reliable reporter alerts" named. Since 2026-11-03 the Reliable kind no longer pushes, so the gap is met for that kind by not pushing it at all. The follow kind still pushes on the strength of the tag, so the dependency stays for it: several coordinated Reliable accounts can each be followed by the same person, and the Reliable tag is farmable at the price given in "Honest limits": extra verified accounts that confirm each other, plus a claimed location for each. The per reporter caps and the Not confirmed yet words stand in for burst detection, nothing more. Phase 9 lowered the Reliable bar to 3.5 stars (70 points), which makes the tag cheaper still, and let tapped ratings feed the points, which makes it cheaper again. It then added the follow kind, which pushes on the strength of the tag, with the same caps. Burst detection therefore matters more, not less, and stays a dependency before either cap is raised or any further kind is allowed to push on the strength of the tag.

`TestPushDispatchResolver` pins the resolver as the only judge, `TestPushReliable` the early bell refresh of the Reliable path through 3.2's own rows (and that no phone row is queued), `TestPhoneAlertCriticalEndToEnd` the whole route from a post to a replaced note, with the far reader, the author and the person who muted getting none.

### Official alerts on the phone

Phase 5 added the one kind that is not about a crowd report: a Severe or Extreme alert from NDMA SACHET that covers the person's stored home area. It is read only. It has no vote, no confirmation count and no reporter, so nothing in this document's counters or tags applies to it, and the crowd rules above neither feed it nor are fed by it. [OFFICIAL-ALERTS.md](./OFFICIAL-ALERTS.md) is the full account; this is the part that touches trust.

- Who is told. The owner's rule (D-16) is that a note fires only for a Severe or Extreme alert that names the person's district, or for an Extreme alert that covers their whole state. A state wide Severe alert never queues anyone, because it would be on nearly all the time. A district placed alert matches by district only, and the home area cell is tested at its centre and four corners, which leans toward telling. `TestOfficialQualifies` pins the grid and `TestOfficialPushEnqueue` the queue.
- Its own lane. A separate outbox and worker, so nothing in 3.3's report outbox changed. Official notes never count toward the hourly limit of 4 above. Their own limit is 3 per account in a rolling 6 hours, at most 2000 people per alert, and a life of at most 2 hours. They are silent, like every note, and a tap opens Official alerts.
- Words from closed tables. The title names a closed event word, a closed level word and a district name from Pinalert's own boundary data, and the body names a closed issuer, for example "Cyclone warning for NTR district" and "Extreme. Issued by IMD." No feed text reaches the lock screen.
- Series and the Apple choice. SACHET sends an Update under a new guid that names the earlier message, so one alert and its updates are one series. The Topic and Tag come from the series' first alert, so an escalation replaces the earlier note where the platform honours the tag. An Apple endpoint does not, so it gets only the first note about a series. The rule is per account and series, and it only ever means fewer notes, in line with the owner's few notifications rule (3.3 check 8).
- Checked again at send time, from the stored facts: the alert is still in force and not within 10 minutes of its end, it was seen or the feed was good in the last 30 minutes, the home area still matches (the stored official switch is no longer read) and the 6 hour limit has room. Nothing is sent from a feed that is not fresh. `TestOfficialWorkerRechecksAtSendTime` pins it and `TestPhase5OfficialJourney` walks the whole route.

No part of this claims that an official alert is correct. Pinalert copies SACHET and says so; the list of what can go wrong is under "Limits and open questions" in OFFICIAL-ALERTS.md.

## Signing in

Phase 7.1 replaced the emailed link with an email and a password. This section says how sign in works, what it limits, and where it is weaker than it could be. Everything here sits under the crowd rules above: signing in decides who an account is, and the vote, place and score rules are untouched. The cost of one more account in "Honest limits" is still one more email address that can receive a code.

The rules.

- An account is an email and a password. The email is proved once, at sign up, by a 6 digit code typed in from the email. After that, sign in is the email and the password only. There is no emailed link anywhere.
- A forgotten password is reset with the same kind of code. A reset ends every session of the account, on every device, and clears the wrong password lock, so the owner of an address can always get back in. The new password, the spent code and the ended sessions are written in one transaction, and the account's password epoch moves with them: a session is let in only while its epoch is the account's, and a sign in that checked the old password while the reset ran binds nothing (`TestResetEndsSignInInFlight`). Create account on an account that already existed (one made with the old emailed link) ends its sessions the same way.
- A password is stored only as an argon2id hash (19456 KiB of memory, 2 passes, 1 lane), with a random salt, and a 3 slot limit on hashes running at once. The password itself is never stored or logged. A sign in for an address with no account does the same work against a dummy hash, so time does not give the answer away.
- A code is stored only as a keyed hash (HMAC with a key made from `SESSION_SECRET`). It lasts 10 minutes, allows 5 tries, works once, and belongs to the browser that asked for it. Each try is counted before the code is compared, so a parallel guess cannot beat the limit. Asking for a new code replaces the old one.
- Every answer is the same whether or not an address has an account: the page, the status, the headers and the words. When an account already exists, Create account emails its owner to say so instead of sending a code. When none exists, Forgot password emails to say so. `TestEnumerationSignInIdentical` and its neighbours compare the raw bytes.
- A new session id is made at every successful sign in, so a planted cookie never becomes a signed in one (`TestFixationFreshSessionID`).

The limits.

- Wrong passwords: 5 free tries per address, then a lock of 5 minutes that doubles on each further failure up to 60 minutes. The count is kept in the database under a keyed hash of the address, and is forgotten after 30 minutes with no failure. Each try is counted before the password is checked, in one guarded statement, so many guesses sent at once get no more checks than the lock allows (`TestSignInLockCountsBeforeVerify`). A try that is answered "busy" because every hash slot was taken checked no password, so it is given back, the same for every address, and the lock still punishes wrong passwords only (`TestSignInBusyGivesTryBack`, `TestEnumerationBusyHasherIdentical`). A right password clears it. `TestLimitsLockScheduleReachesCap` pins the schedule.
- Per device: each browser has its own buckets for sign in, for sending codes and for trying codes, so one noisy phone is held back without touching anyone else.
- Per network address: a bucket by IP exists only when the host's client IP header is confirmed and named in `CLIENT_IP_HEADER`. Outside `ENV=development` the server will not start without it; the README canary check is how the header is picked. The per device buckets never key on the address, so a shared carrier address (CGNAT) is never counted as one person by them, which is the point (`TestLimitsDeviceBucketCGNAT`).
- Code emails: one per address every 45 seconds, 5 an hour for each address (Create account and Forgot password share the count), and one shared cap for every request that arrives without a valid cookie (none, a malformed one or a wrongly signed one), so a made up cookie never buys a fresh per device bucket. Sign in has the same shared cap.
- Email ceiling: at most 90 sends a day in total, so Resend's free 100 cannot be used up. 20 of those are kept for the Forgot password emails (`TestMailerCeilingReservesResetSends`). The reset code and the "no account" note count as one class, and so do the sign up code and the "account exists" note, so no refusal says whether an account exists. Sends are also paced: at most 15 in any 10 minutes, 5 of them kept for Forgot password, so a burst cannot spend the day and a person refused in a busy window gets in a few minutes later (`TestMailCeilingPacesBurst`). Once the other emails of the day are spent, one sender may take at most 3 of the kept slots a day. A busy 10 minutes never counts toward that share, so a shared carrier address (CGNAT) is not refused Forgot password on a busy but normal day (`TestMailerCeilingShareOnlyOnDayReserve`). A sender is a client network when `CLIENT_IP_HEADER` is set, and every request without a valid cookie counts as one sender, so one network that floods Forgot password for made up addresses cannot spend the reserve (`TestMailReserveSurvivesOneNetwork`). Every one of these limits answers a known and an unknown address alike (`TestEnumerationCeilingWindowIdentical`, `TestEnumerationCeilingShareIdentical`).

Why the password minimum is 8. The owner chose an easy sign up (D-13): 8 to 128 characters, with a list of the most common passwords refused. NIST SP 800-63B-4 asks for 15 characters when a password is the only sign in factor, and that is recorded here as the stricter option. The slow hash, the per address lock and the common password list are what stand in for the extra length. If this site is ever promoted widely, raising the minimum is a change to `PasswordMinRunes` in `internal/auth/policy.go` plus the words on the sign up card that name the number.

Honest limits of sign in.

- Credential stuffing is bounded weakly when no client IP header is set. A script that drops cookies and shares one network address is held back only by the per address lock and the cost of each hash. That is why the server will not start without `CLIENT_IP_HEADER` outside `ENV=development`, and the README canary check picks the header. The accepted gap is T-07.1-03.
- A targeted lockout is accepted. Anyone who knows an address can keep it locked by failing its password on purpose. The owner of the address gets back in through Forgot password, because a reset clears the lock, and the flow ends as drawn, with "Password set. Sign in to continue.". This was chosen over a lock that cannot be triggered from outside, which would let a guesser try forever.
- A typed code can be read over a shoulder or from a shared inbox. The code is bound to one browser and works once, but anyone who sees both the email and the page in time can use it.
- Email alias tricks (a plus sign, dots) and disposable addresses are not blocked. See "Out of scope for this model".
- The daily ceiling protects the Resend quota, not the inbox of a person. A hostile sender can still make an address receive up to 5 codes an hour.
- The reset reserve is not proof against everyone, and this is accepted as T-07.1-09. Several networks working together can each take their 3 kept slots. In development, where `CLIENT_IP_HEADER` may be unset, a script that fetches a fresh cookie before every request counts as a new sender each time, so only the pacing slows it; outside development the server will not start without the header. An IPv6 holder of many /64 prefixes counts as many networks. The reserve cannot tell a reset code from a "no account" note without telling an attacker whether an account exists, so these gaps are closed by per sender limits or not at all.
- The send counts are kept in the server's memory, and this is accepted as part of T-07.1-09. When the server restarts, or a free host puts it to sleep after a quiet spell and wakes it on the next visit, the day's counts start again from nothing. The 90 a day cap then no longer keeps Resend's own 100 a day out of reach, and once Resend's limit is spent it refuses every email until its day ends, Forgot password emails included. Counting the day's sends in the database would close this; it is not done yet.
- Usernames are public. They are checked as they are typed, screened like other free text (see MODERATION.md), and a name that could not be judged is saved as pending. Phase 8 and Phase 9 show only names whose screen word is clear, and re-check pending ones first.

Upgrading: an existing database needs `make migrate` once after this phase merges, for migrations 00014, 00015 and 00016 (run the migration first, then start the new server). 00016 adds the password epoch and keeps every session of an account that already has a password signed in.

## Report history

Phase 8 gave each report a history, "What happened so far", at `GET /api/reports/{id}/timeline`. It is read only and sits under the trust block: it adds no number of its own.

How it is built. `BuildTimeline` replays the append only vote log of one report, row by row, through the same `BuildVoteTally` and `Resolve` that make the trust block, so a history line and the trust block cannot disagree about how many spots had confirmed at a moment. The log is read once, in id order, capped at `TimelineLogCap` (2000 rows). Every sentence comes from closed copy and `strconv.Itoa`, so no reporter words, voter, account, cell or spot can reach the wire. The kinds are posted, confirmed, disputed, resolved, reopened, faded and yours.

Why it shows only upward steps. A line is written when the count first reaches a milestone (`TimelineMilestones`: 1, 2, 3, 4, 5, 10, 25 and 50 spots), when the report first becomes Live, when it enters or leaves the Disputed view, when it is resolved or reopened, and when it fades. A milestone already reached is never written twice, and no downward step is ever written (the decision of plan 08-01). A history that listed every vote would let a reader count individual voters, and one that listed every drop would turn a take back into a visible event. The take back of a mistaken resolve writes no reopen line, because a take back is not a reopen (03.1 D-28).

What that means for a reader. Each line shows the count at the moment it was written. After a vote is taken back, the trust block can read lower than the last history line. The trust block is the current truth and the history is the record of how it got there (review B2).

Who may see it. The same people who may see the report: a held, removed, unknown or too old report answers one byte equal 404, so the route confirms nothing about a hidden report. It needs a signed in session and shows the viewer's own steps as "You ...", with no other person's name.

Tests. `TestBuildTimeline` covers the table, `TestTimelineMatchesTrust` replays 60 seeded random logs with take backs and checks each count against `BuildVoteTally` at the writing row, `TestTimelineWording` pins the sentences, `TestTimelineResponseOmitsIdentifiers` scans the live response, and `TestTimelineWithheldIs404` pins the equal 404 bodies.

## Comments and likes

Phase 8 added comments on a report, one level of replies, a thumbs up like on each comment and a Flag on each comment. The rule that keeps the trust mechanic clean is short: comments and likes never feed any trust number.

- No count, tag or score reads them. A like is not a confirmation, a comment is not a vote, and neither moves `BuildTrust`, `Resolve`, the still current word, the reporter tag or any ordering. `TestLikesNeverFeedTrust` scans the trust, visibility, reliability and currency files for any use of the comment tables, and compares the trust object and the vote rows before and after ten likes and a flag. It would fail if a later change wired one in.
- Anyone signed in with a username may comment, from anywhere. There is no 1 km rule for a comment, because a comment cannot change what the report says about itself. It is safe for trust for that reason: a faraway person can say something, and cannot add one spot to a count. Voting keeps its 1 km rule, unchanged.
- Words are screened with the abuse screen, not the gibberish rule. Abusive text is refused, borderline text is posted and listed for moderators, and a comment whose check cannot finish is held, visible to its author only, until it passes (D-12). See MODERATION.md, "Comments". The screen judges tone, not truth: a calm false comment passes it. That is why a comment is never allowed to move a trust number, and why a Flag exists.
- A flag never hides a comment. Only a moderator's Remove does (D-14). Comments close with the report: they stay open only while it is active or disputed (D-15, `TestCommentsClosed`).
- A like is one per account and comment (`TestLikeOnePerAccount`) and notifies nobody. A comment on your report, or a reply to your comment, reaches your bell as one unread row per report and kind, and no phone since 2026-11-03 (see "Phone alerts").
- Names. A comment shows a username only if it passed its screen. A name that could not be judged reads New member, and a deleted account reads Deleted account. Deleting an account blanks its comments and removes the likes and flags it gave (`TestDeleteAccountComments`).
- The Reporter tag. Others see the word Reporter beside the name of the person who posted the report, and the author never sees it on their own comments. That ties a public username to a report: anyone who reads the page can tell which username posted that report. This linkability is recorded as an accepted risk under D-02, because showing who wrote the report is what makes a reply readable. The email is never shown, only the username.
- Comment text is never put on the page through a markup parsing sink (D-12), and the comment list response carries no account id, email or cell (`TestCommentResponseOmitsIdentifiers`).

What it does not stop: a comment can still carry a false claim or a pasted forward, and a group of accounts can like each other's comments. Neither moves a number. The cost is read time for a person, not a wrong count.

## Sharing

Phase 8 added a Share button and a public page for one report, `GET /r/{token}`, so a forward in WhatsApp can lead to a page that is true at the moment it is opened (D-04, D-05, D-06).

- The link carries a random token, never the report id. The token is 22 characters from 16 bytes of `crypto/rand` (128 bits), made the first time anyone taps Share, with the first write winning. A guess is not practical. A token for a held or too old report answers like an unknown one; a removed one says it was removed (D-06). The link is rooted in `BASE_URL`, never in the request's Host header (`TestShareLinkIgnoresHostHeader`).
- The preview carries no count and no claim. The share title is the category word, then report, then near and a coarse area when one is known, and the line under it is the fixed words "Open to see if this is still true right now". Open Graph tags carry the title, those words, the link and the app icon, nothing else (`TestShareOpenGraph`), so a forward cannot carry a stale number or the word verified.
- The public page shows only the category, the coarse area, a neutral status word (Active, Disputed, Resolved or Ended), the same sentence the trust block gives for that state drawn when the page is opened, the severity, the What is happening pick for a Critical report, and when it was posted and ended. It has no field for the reporter's words, a coordinate, a distance, a username, a comment, or the trust details beyond the one sentence (for a confirmed report the sentence names how many people nearby confirmed it; the cells, the votes and the scores are never shown) (`TestSharePageShowsOnlyAllowedFields`). A report a moderator removed says so; a held, unknown or too old one reads as unavailable, so a moderation state is never revealed.
- Area name amended. Phase 3.3 (D-19, D-20) kept the looked up area name out of every page response. For these two surfaces only, the Share title and this public page, the same stored name now appears, as a coarse area. It is looked up once per report, by the call that made the token, and the public page never calls the lookup.
- No account, script or cookie is involved. The page sets no cookie and writes no session row, even for a first visit (`TestSharePageSetsNoCookie`), carries `noindex` and `no-store`, and sends `Referrer-Policy: no-referrer`.
- Limits. Share spends its own per account bucket. The public page has a per address limit only when `CLIENT_IP_HEADER` is set; without it the page has no limit at all, and only the 128 bit token protects it (review B1), so the README canary check should be done before any wide sharing.

## Reporter profiles

Phase 9 lets a reporter be followed. A profile is a page for one person, at `/u/{username}`, and `/me` opens your own. It is opt in and it adds no number to the trust mechanic: it is a place to read the tag, the counts and the stars that every report already carries, plus a person's recent reports.

Opt in. The switch is Let others follow me, in Settings, People and reports. It is off for every account until the person switches it on, and every account that existed before Phase 9 starts off. Anyone with a clear username and an accepted Name can switch it on (Q2, D-18). Only following needs the Reliable reporter tag, so a New reporter can have a profile and cannot be followed until the tag is earned. While it is on the page shows the username, the Name, the home area, the stars with both parts, the tag with its counts, Member since, the Followers and Following buttons and Recent reports. While it is off the page does not exist for anyone else.

The Name is screened. When the switch goes on, and on every later edit of the Name while it is on, the Name goes through the reserved word check (`NameReserved`) and then the abuse screen, and fails closed: a Name that is borderline, rejected or could not be checked is refused. The reserved list is written in ASCII, so a Name in another script is screened for abuse by the provider and is not compared with the reserved words. That is a known limit (G1). A pending username is screened again when the switch goes on, and a username shows on a profile only while its screen word is clear.

One not found. A profile that does not exist, is private, is pending, belongs to a deleted account, belongs to someone who blocked you, or is malformed answers the same bytes, so the route confirms nothing about a person (`TestProfileNotFoundIsUniform`). The one exception is a person you blocked yourself: your own profile read says Blocked by you, and their lists and reports answer the uniform not found.

Public lists. Followers and Following are public to every signed in account (D-03). A person with a public profile of their own is named and links to it; everyone else is only counted, in a plain line such as "and 3 others" that is never a link. A list holds `PeopleListPageSize` named people a page. The paging cursor is sealed by the server (AES-GCM under a random per process key, with a random nonce each time), so only a cursor it issued is accepted, nobody can craft one to learn when a person followed, and the follow time inside it cannot be read (`TestPeopleCursorIsSigned`, `TestPeopleCursorHidesTheFollowTime`). It also carries the list's owner and direction, so a cursor issued on one list is refused on any other with the same calm sentence (`TestPeopleCursorIsBoundToItsList`, `TestPeopleCursorOpensOnlyOnItsOwnList`). A cursor from before a restart is refused too: the key is new, so the old cursor no longer opens. When Show more is answered with that refusal, the list window reads the list again from the top (an empty cursor) and shows the server's sentence once; it never sends the dead cursor a second time, and if the read from the top also fails it shows the list's error state with Try again, which starts from the top as well (`web/people_list_test.mjs`, "a Show more cursor the server refuses starts the list again from the top"). Your Blocked and Muted lists are yours alone.

Recent reports (D-07, D-20). A profile lists the person's reports of the last 30 days (`ProfileReportsWindow`), newest first, at most `ProfileReportsCap`, through the same read path as the feed, so a held or removed report is never listed and every state word is the feed's own. The two kinds of row are different on purpose.

- A live report (Active, or Disputed while live) is shown as on the map: its words, its exact point and a link that opens the report. A reader's distance to it is measured in the browser from the kept position, and the server learns no reader location.
- A finished report (Resolved or Expired) is shown at a rough area only. The point is moved to the centre of a precision 5 cell (`ProfileCoarsePrecision`, about 5 km across), a finished Rescue report is blurred the same way and shows no text (it takes the precision 5 cell too, because the precision 6 cell is exactly the point the anonymous Past tab shows, and the most sensitive category must not get the least blur), and the distance is measured to the blurred point. The row carries the end date and never a time, no report id, no description and no link, and the page draws it without an `a` element at all (`TestPersonPageFinishedRowsAreCoarse`, `TestCoarsenFinishedForProfile`). The reason is linkability: a public username beside an exact finished point and a time would match the anonymous row of the Past tab at that point.

The profile link in the trust panel. A live report carries `reporter_profile_path` when its reporter has a public profile with a clear username, is not the reader, and has not blocked the reader. The trust panel shows it as See this reporter, the last row. It sits beside the trust object and never inside it. A finished report never carries it, on Focus, Past, Activity or the feed, for the same linkability reason (D-20, `TestFinishedViewsCarryNoProfilePath`).

Evaluate on touch. A profile whose person no longer holds the Reliable reporter tag ends the follows to that person when it is next read, before the counts are read (`TestProfileEvaluateOnTouch`). The sweep does the same for people nobody opens. See "Follows, mute, block and follow notes".

Deleting an account removes its follows, blocks, ratings given and received, mutes and Notify me choices in the one transaction, and switches the profile off (`TestDeleteAccountEndsFollowsAndBlocks`, `TestDeleteAccountRemovesRatings`, `TestDeleteAccountRemovesPersonMutes`).

## Stars and tapped ratings

Stars are the reporter score divided by 20. `StarsTenths` and `StarsText` floor it to one decimal from integers, never a float format, so 69 reads 3.4 and 70 reads 3.5, and 34 and 35 both read 1.7 because the Mixed record floor falls between two tenths (`TestStarsTextWalkMatchesTag` walks every score and checks the number never contradicts the tag). The tag is still cut in exactly one place, `reporterTagForScore`.

Two parts. Below `MinRatersForStars` counted raters, the points are the report outcome score alone, exactly as before Phase 9. From five counted raters on, the points are half the outcome score and half the tapped average, where a tapped star is worth `RatingPointsPerStar` points. In integers, with raters the counted raters and tapSum the sum of their stars:

- num = outcome x raters + 20 x tapSum, and den = 2 x raters;
- points = (2 num + den) / (2 den), which rounds half up, then clamped to 0 to 100.

`CombinePoints` is the one function that does it (`TestCombinePoints`, `TestBelowFiveRatersOutcomesOnly`, `TestCombinedTagCut`). The profile shows the total with both parts beside it, such as "3.0 from reports" and "5.0 from ratings by 5 people". A reporter with no score, a New reporter, is never rated, whatever is tapped (`TestRatingsNeverRateANewReporter`).

The tapped part is blurred (review W5). It is shown rounded to half stars (`TappedHalfStars`), so 22 stars from 5 raters reads 4.5 and never 4.4. The limit is honest: the blur hides one person's exact stars only roughly. The exact total, the outcome part to a tenth and the rater count are all shown, so a careful reader can estimate the tapped average closely, to about 0.15 stars at 5 raters. It is a blur and not a guarantee that one rater's stars stay hidden.

Who may rate (D-02, D-23). A signed in account that has cast a confirm, dispute, resolve or reopen vote on one of that reporter's reports. An unresolve alone does not qualify, because it only undoes one's own resolve. A witness who voted at the scene, from the report's own cell, can rate (`TestWitnessAtTheSceneCanRate`). Nobody rates themselves, a rater removed by a moderator cannot rate again (`TestRemovedRaterCannotRateAgain`), and the reporter has to be a visible public profile. A reporter who blocked the rater is the uniform not found, and a reporter the rater blocked is a refusal. A block never deletes or hides a rating (`TestBlockKeepsRatings`).

Raters count by distinct place (D-21). Each rating takes the place of the rater's most recent qualifying vote, and one rating is counted per place, the most recently written. This is the same rule as confirmations, with the same cell (`voterGeohashPrecision`, so places about 150 m apart). Five accounts in one cell count once (`TestRatersCountedByDistinctCell`), and so do a reporter's own second accounts, which usually sit in one place. A repeat tap of the same stars neither moves the time of the rating nor takes a place from another rater (`TestRepeatTapDoesNotStealTheCell`).

Rules on changing a rating. One rating per rater and reporter, changeable no more than once in `RatingChangeGap`, 24 hours (`TestRatingUpsertAndDailyLimit`). The Rate bucket is per account and never per network address: a burst of 10, then one more every 30 seconds (`rateBurst`, `rateEvery`, `TestRatingRoutesRateLimit`). Remove my rating follows the same 24 hour rule.

The private min rule (D-22, review B3). While Let others follow me is off, ratings can only lower the points: they are the smaller of the outcome points and the combined points. So going private never lifts a tag and never wipes critics (`TestPrivateProfileRatingsOnlyLower`, `TestSwitchingOffNeverRaisesTag`).

What five raters can do (review B4). Five eligible raters in five different places can move a reporter's points by up to 40 either way. That is enough to drop any reporter below Reliable reporter, and to lift any outcome score of 39 or more to it. Two worked examples, from the formula above. An outcome score of 39 with five five star raters gives (39 + 100) / 2 = 69.5, which rounds half up to 70, so a reporter at Mixed record becomes Reliable reporter. An outcome score of 100 with five one star raters gives (100 + 20) / 2 = 60, so a reporter with a perfect record reads Mixed record. The move is half the gap between the tapped average, in points, and the outcome score, so a reporter below 20 points can be lifted a little further than the round figure.

What answers it, and what does not:

- The place rule. Raters count by distinct place, so the five have to be in five different cells, and a reporter's second accounts are held back exactly as for confirmations.
- Soft ending. A follow that is lost because the tag fell is kept with an end time and the reason tag_lost, not deleted. When a moderator removes abusive ratings and the tag comes back, those follows are restored (`TestModeratorRemovalRestoresFollows`). The restore takes the tag_lost follows ended since the earliest removed rating was first written (its creation, not its last change, so changing the stars cannot shrink the window, `TestModeratorRemovalRestoresAfterTheRatingWasChanged`), and a moderator's retry after a failed removal still re-evaluates (`TestModeratorRemovalRetryFinishesTheRestore`), so it can also bring back a follow lost to ordinary decay in the same window. That is accepted, and only tag_lost rows come back, and only while the tag is Reliable.
- Moderator removal, one rating or every rating by one rater, with an audit stamp (`removed_by`, `removed_at`). Removing a rater is permanent: their ratings stop counting and they cannot rate again. See MODERATION.md.
- The trust panel line "Includes ratings by N people." whenever tapped ratings are part of the tag, so the tag never hides that part of it rests on ratings (`TestReporterFactSaysRatingsIncluded`).
- What does not answer it: five accounts in five different places. Five places are enough, and nothing stops one person who controls accounts in five places, which costs what "Honest limits" already says an extra place costs (T-09-103). The lower 3.5 bar makes the tag cheaper to reach and cheaper to lose, so burst detection (TRUSTX-03) is a named dependency that matters more under the 3.5 bar (D-14). It is not built.

Deleting an account removes the ratings it gave and the ratings it received (G6, `TestDeleteAccountRemovesRatings`). That can move another reporter's stars: a reporter loses a rater's stars when that rater leaves, and the points are recomputed on the next read like every other score.

One loader reads the tag for the feed, Focus, Past, Activity, a vote answer, the dispatcher, the push worker and the follow sweep, each with the ratings source supplied once (`LoadReporterSignals`, `TestRatingsSourceWiredEverywhere`). So a rated tag reads the same everywhere (`TestRatedReporterSameEverywhere`).

## Follows, mute, block and follow notes

Following (D-01, D-12). A signed in account can follow a person whose profile is on, whose username is clear, who holds the Reliable reporter tag now, and who has not blocked them. Nobody follows themselves, and an account follows at most `FollowingCap` people. The refusal sentence is one for every reason, so it never says which rule refused. A follow is a row with an end time and a reason, never a delete: the reasons are unfollowed, removed (Remove follower), blocked, profile_off and tag_lost. Every ending goes through two helpers, `endFollowsOf` and `endFollowPair`, and no query ends a follow by another route (`TestFollowEndHelpers`).

"At once" (D-24, SC5). The words in the roadmap are met by the computed tag. From the moment the tag falls away, no follow note is produced or sent and the person cannot be followed. The stored follows end at the next sweep (`FollowSweepInterval`, 5 minutes, `TestFollowSweepEndsOnDecay`) or at the next touch of the profile, whichever is first. A note already written to the bell reads as no longer available.

Block (D-12). A block ends the follows in both directions, stops that person following you, hides your profile from them as a uniform not found, and hides their comments from you in Phase 8's lists and counts and never notifies you of them. It does not hide their reports anywhere, and it never touches a rating. A block is private: the other person is not told, and sees only the same not found a private profile gives. A Follow that lands in the same instant as a Block can, rarely, leave an active follow beside the block; the follow sweep finds every such pair, in either direction, and ends it with reason blocked (`TestFollowSweepEndsAFollowBesideABlock`).

Mute (D-12). A mute is private to the person who made it and is never shown to the muted person. It stops the notes about that person's posts, both the Reliable reporter alert and the follow note, and turns their Notify me bell off. It stops nothing else. Their reports still show in every feed, map, list and profile, because hiding emergency reports is never safe (`TestMuteNeverHidesReports`), and it does not touch Critical notes, nor changes on reports you posted or voted on (`TestCriticalStillArrivesFromMutedPerson`). Unmute leaves the bell off.

The Notify me bell (D-08, D-09, D-11). A follower can switch the bell on for one followed person at a time. It is off by default, off again after every ending of the follow, and off again after a mute. At most `NotifyFollowsCap` people can have the bell on. The bell replaces the general Someone I follow posts switch that Phase 3.3 had planned, so there is no such switch in Settings (D-11). The bell stores the choice and the time it was switched on, and only reports posted after that time get a note.

The follow note, under the one rule. A follow note is a bell row of its own kind, written by the refresh of the follower's bell, and the phone note is queued from exactly the rows the bell step wrote, so one row feeds two channels. It is decided by `MayNotifyAboutReport`, which gives the follow kind the Reliable kind's allowance: the report may be Live or Provisional, never Hidden, Withheld, Retracted or expired. A Provisional report reads Not confirmed yet until people nearby confirm it. A follow note is written only while the follow is active, the bell is on, the person is followable, the reader has not muted them, and their tag is Reliable reporter now, from the same rated tag as everywhere else. The reader never gets a follow note about their own report.

One first alert per person and report. The Reliable alert and the follow note share one unique index, so a person who follows a Reliable reporter gets the follow note and never both (`TestFirstAlertIndexSharedByBothKinds`). The follow note counts toward the ceiling of `UpdatesHourlyCeiling` updates an hour (the Reliable rows do not, they have their own cap), and the bell reads at most `FollowedPostsCap` followed reports a refresh.

Order, caps and words on the phone. The dispatcher runs Critical, then Follow, then the confirmed update (its Reliable and change branches only refresh bells, since 2026-11-03), so a follower who was already queued as a Critical note is not queued again as a follow note, and a follower is never queued a Reliable phone note (`TestFollowPushOrderCriticalFollowReliable`). One phone note goes to a follower per report. The pass for one report refreshes the bells of at most `FollowFanoutCap` followers. That bounds the work of one pass and is not a promise that a follower past it gets no phone note: if their own bell poll has already written the row, the phone note is still queued for it. The lock screen reads "From someone you follow." with the area, the distance if the follower has a home area, and Not confirmed yet until confirmed. It carries no username, no report words and no coordinates. See PUSH.md.

Checked again at send time (D-24, T-09-89). Just before a follow note is sent the worker reads the follow, the bell, whether the person is still followable, any block or mute, and the reporter's tag with the ratings, and it stops at the first thing that no longer holds (`TestFollowPushSendTimeRecheckDecay`). The Reliable reporter note and the confirmed update that replaces one get the block and mute part of the same check, a queued comment note stops once either person blocked the other, and a confirmed update whose first row was pruned is skipped, never re-based on another kind (`TestQueuedReliablePushStopsAfterMuteOrBlock`, `TestConfirmedUpdateWithoutItsFirstRowIsSkipped`, `TestCommentPushStillHoldsAfterABlock`). A note still waiting is deleted at once whenever a follow or the bell ends, by the same two helpers, so nothing waits for a sweep (`TestFollowCleanupOnEveryEndPath`). A follow note ignores distance and home area for the decision, because the follow is the reason the person gets it (`TestFollowPushNoHomeAreaStillSent`).

What pending means (Q9). "Pending notes" in the roadmap criterion means notes not yet written and phone notes still queued. A bell row that is already written stays as history and reads "This report is no longer available", with no name, report or place, as soon as the report finishes, the follow ends, or the person goes private, is deleted or blocks the reader (`TestFollowBellEndedFollowReadsUnavailable`, `TestFollowBellFinishedReportDropsName`).

What this does not do. The tag is farmable at the price "Honest limits" gives, and a follow note is a way to reach a person on the strength of the tag, with the same caps as the Reliable kind. Burst detection is still the dependency.

## Where the roadmap words are met

| Roadmap words | What ships | Where in code | Test |
|---------------|------------|---------------|------|
| "confirmed by N nearby" | The counter `N confirmed`: N distinct places, every vote cast from within 1 km | `BuildTrust`, `CastVote` | `TestBuildTrustCountsMatchTally`, `TestCastVoteRejectsTooFar` |
| deliberately chosen, documented geohash precision | Precision 7, with the reasoning in "What counts as a place" | `voterGeohashPrecision` | `TestGeohashPrecisionWitnesses` |
| not the raw number of votes | Distinct places, so repeat votes and a crowd in one place count once | `BuildVoteTally` | `TestTrustCountsDistinctPlacesNotVotes` |
| two distinct trust signals side by side | The still current word and the reporter tag, both shown at once on every report without opening anything: beside the state label on rows and the Activity page (A75), and as the state word and the last row of the popup card (A91, A92) | `BuildTrust`, `createStateLine` | `TestFeedSignalsMatchCastVoteSignals`, `TestTrustBlockMountedOnBothSurfaces`, `TestTrustStateLineMarkup` |
| reports auto-fade unless re-confirmed | A confirmation from a new spot pushes the fade time back, up to a cap, in the same statement as the vote | `ExtendedExpiry`, `InsertVoteExtending` | `TestConfirmFromNewSpotExtendsExpiry` |
| fast decaying still current confidence score | `currency.score` | `buildCurrency` | `TestCurrencyScoreBoundaries` |
| slow decaying source reliable score | `reporter.score` | `ScoreReliability` | `TestReliabilityTable`, `TestReliabilityDecayAndWindow` |

How the word score is met (D-08, TRUST-07), for both halves: each score is an integer from 0 to 100 (`currency.score` and `reporter.score`) from a documented formula over real votes. It is recomputed on every read and never stored. The band or tag is cut from the same integer, so the word and the number cannot disagree. A score is null when there is no basis for one: fewer than 2 confirming places over the life of the report, or `not_applicable`, for the still current score, and New reporter or none for the reporter score. The numbers show on screen in the map popup only (D-09), and Phase 6 can sort on them server side. The two subsections called "How the roadmap word score is met" in the still current and reporter sections give the detail, so it is not repeated here.

The visible form of "confirmed by N nearby" is the counter `N confirmed`, with the confirmations row of the trust card, titled "Confirmed by {N} people" with the detail "At different spots nearby.", naming who is counted. The older sentence "Confirmed by {N} people at different spots." stays on the wire as `confirmed_sentence`. That is the form CONTEXT records for the first roadmap criterion.

## Hand offs to later phases

- Phase 03.1 reuses `ClassifyOutcome` as a pure function, so history labels cannot disagree with the rating. What it built, and what later phases reuse: `ViewOf` is the one tab partition and `ClosureOf` the one closure replay, both pure functions of values with no I/O; `CoarsenFinishedRescue` is the one blur of a finished Rescue report; and `unresolve` with `UndoLimit` is the one take back. A new read path calls these and does not rewrite them.
- Phase 03.2 builds update wording from the same trust object. It should treat the Reliable reporter tag as farmable at the price stated in "Honest limits": extra verified accounts plus a claimed location. It opens a report by id from a link, so it reuses the single report read of Phase 3.1, which already applies `CoarsenFinishedRescue`, and never builds a second path that could forget it.
- Phase 4 owns vote rate limiting, and its moderator view of a report names the tab with `ViewOf` and adds its own withheld check around it. The take back is a fifth vote route, so a Phase 4 limit written for four vote routes has to name it too.
- Phase 5 is delivered: the official feed, the Demo and the official phone lane. Official items are read only and live in tables of their own. They are never counted in any crowd number or reporter tag, and they ignore the partition `ViewOf` makes. The Demo handles far away visitors with no exemption: it gives each visit one fixed pretend position and seeds about half its reports within 1 km of it, so a vote on a farther report is refused with the real `TooFarMessage()` words. Its pretend neighbours vote through the real `CastVote` from places in different cells, and the real resolver runs on an in memory copy of the vote queries that a differential test checks against the SQL. See [OFFICIAL-ALERTS.md](./OFFICIAL-ALERTS.md).
- Phase 6 may sort on the integer scores server side. It must never use them to bury a critical or rescue needed report (D-14), and must never bake a count or a rating into a shareable card image.
- Phase 8 is delivered: the report page, the history, comments and likes, and sharing (see "Report history", "Comments and likes" and "Sharing"). Its report page reuses the trust block unchanged and places the reporter words with the same helper as the other surfaces, `createStateLine`. It shows finished reports through the shared read, so it calls `CoarsenFinishedRescue`. Phase 9's followable profile does not use that function: it moves every finished report on a profile, a Rescue report included, to the centre of a precision 5 cell through `CoarsenFinishedForProfile`, and neither copies a report's coordinates or description by another route.
- Phase 4's Low data view should read `facts` for the rows of its trust block, not the older sentence fields. The server already writes every word of a row.
- Phase 3.3 built the phone channel on `MayNotifyAboutReport`. Phase 5 added the official alert kind and its Settings row beside that pipeline rather than on it, in an outbox and worker of its own, because an alert has no report, no vote and no resolver verdict to ask. Phase 9 is delivered and added the follow kind through `MayNotifyAboutReport`, driven by its Notify me bell. The function gives the follow kind the Reliable kind's allowance (Live or Provisional) in one condition, in that one place and with its test (`TestMayNotifyFollowedPosted`), and the two kinds share one first alert index so a person and report get one first alert between them. Each of the kinds added a migration of its own and a row to the closed lists, and none writes a second visibility rule.
- Phase 7.1 gave every account a unique, public username. Phases 8 and 9 read names through the one batched read `AccountService.UsernamesFor`, show a name only when its screen word is clear, and check for a username themselves in any route that needs one, because the gate passes every `/api/` path for every account. See "Signing in".
- Phase 9 is delivered: opt in reporter profiles, tapped star ratings, follows, mute, block and the follow note (see "Reporter profiles", "Stars and tapped ratings" and "Follows, mute, block and follow notes"). It reads every reporter tag through the one loader `LoadReporterSignals` with the ratings source, so a rated tag reads the same on every surface, and it cuts the tag only in `reporterTagForScore`. Whatever comes next reads the tag the same way and builds no second cut. Burst detection (TRUSTX-03) is the dependency that matters most before any of it is promoted widely.

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
| `ReliableMinScore` | `internal/service/reliability.go` | 70 |
| `MixedMinScore` | `internal/service/reliability.go` | 35 |
| `MaxReporterAccountsPerRead` | `internal/service/report.go` | 200 |
| `UndoLimit` | `internal/service/takeback.go` | 2 |
| `ApproxGeohashPrecision` | `internal/service/coarsen.go` | 6 |
| `ProfileCoarsePrecision` | `internal/service/coarsen.go` | 5 |
| `MinRatersForStars` | `internal/service/stars.go` | 5 |
| `RatingPointsPerStar` | `internal/service/stars.go` | 20 |
| `RatingChangeGap` | `internal/service/ratings.go` | 24 hours |
| `ModeratorRatingsCap` | `internal/service/ratings.go` | 200 |
| `FollowingCap` | `internal/service/people.go` | 200 |
| `PeopleListPageSize` | `internal/service/people.go` | 50 |
| `BlockedListCap` | `internal/service/people.go` | 500 |
| `MutedPeopleListCap` | `internal/service/people_mute.go` | 500 |
| `NotifyFollowsCap` | `internal/service/updates.go` | 50 |
| `FollowedPostsCap` | `internal/service/updatesfollow.go` | 200 |
| `FollowSweepInterval` | `internal/service/followsweep.go` | 5 minutes |
| `ProfileReportsWindow` | `internal/service/profilereports.go` | 30 days |
| `ProfileReportsCap` | `internal/service/profilereports.go` | 100 |
| `FollowFanoutCap` | `internal/service/pushdispatch.go` | 500 |
| `defaultExpiry` | `internal/service/report.go` | 8 hours |
| `criticalExpiry` | `internal/service/report.go` | 24 hours |
| `defaultExpiryCap` | `internal/service/report.go` | 24 hours |
| `criticalExpiryCap` | `internal/service/report.go` | 48 hours |

When you change one of these, change the table in the same commit. The test fails with the row name when they differ.

Generated code: run `make sqlc` after editing a `.sql` query and `make swag` after changing a handler annotation or a response struct, and commit the output. `git status --porcelain` should then stay empty for `internal/store/sqlc` and for `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml`.

Databases: tests that need Postgres run against a database created for the run and dropped after it, never the one a dev server uses, because `testutil.NewTestDB` truncates every table.

Upgrading: an existing database needs `make migrate` once after this phase merges, to add `idx_reports_session_expires` from migration 00005. Votes cast before the 1 km rule keep counting. See "Votes cast before the 1 km rule".

Upgrading for backlog 999.12 (a confirmation from a new spot keeps a report alive): no migration is needed. The fade time is the existing `reports.expires_at` column, and the new statement `InsertVoteExtending` only needs `make sqlc` output committed.

Upgrading for Phase 9: an existing database needs `make migrate` once for migrations 00020 (profile switch, follows, blocks), 00021 (tapped ratings and the rater removal columns), 00022 (person mutes), 00023 (the Notify me flag and the follow note kind) and 00024 (the follow kind in the phone outbox). Run the migration first, then start the new server. Nothing signs anyone out, and every profile starts off.

The rating bucket constants `rateBurst` (10) and `rateEvery` (30 seconds) live in `cmd/server/main.go`, in package main, so the constants table above cannot guard them. They are described under "Stars and tapped ratings".

## Honest limits

- Two accounts a few metres apart on either side of a cell edge count as two places. The subtest "cell edge pair counts as two places" shows it: points 1 m apart across a precision 7 cell edge count as two places through `independentCellCount`.
- Several people standing in one cell count as one place, so N is a lower bound on people, not a headcount.
- Each account holds one current vote per report and kind (the latest vote wins in `CurrentVotesForReports`), so walking around cannot add places.
- The real cost of adding a place is one more verified email account plus a claimed location for it.
- A group of accounts at different spots can keep a false report alive up to its cap and never past it: 24 hours after posting, or 48 for a Critical report. Each account extends at most once, from a spot nobody confirmed from before, and a dispute never extends. Two confirms from one new spot in flight at the same instant can both count as new, and a resolve that lands between the pre write read and the write can still let that one confirm extend (the report stays resolved either way). Both overshoots are bounded by the cap. See "A confirmation from a new spot keeps a report alive".
- Client coordinates cannot be verified. A spoofed location defeats both the cell rule and the 1 km rule.
- The server does not read GPS accuracy and does not look for mock locations. The browser reads accuracy only to decide whether a fix may be cached.
- Phone GPS error of tens of metres can put an honest voter standing near the 1 km line on the wrong side of it. Accuracy is not sent with a vote. The browser asks for a high accuracy fix. If that times out or no position is available it asks once more at the default accuracy, and only a refusal of permission ends the request at once. If both attempts fail the vote is still turned away with the location sentence, because no substitute position is ever used. A fix whose reported accuracy is worse than 300 m (`VOTER_LOCATION_MAX_CACHE_ACCURACY_M`) is used for that one vote but never cached, so it cannot be reused for later votes.
- A coarse location fix, hundreds of metres to over a kilometre (for example a laptop on Wi-Fi), is sent as it is. One physical spot can then look like several places, and a voter can be pushed across the 1 km line. This is why the place count is a rough measure and not proof of independent people. The system does not detect or block this.
- Counters, bands and fades update on the 30 second poll, so a screen can be one poll behind the server.
- Report age and the fade use the server's clock, taken from the Date header of each poll response, so a wrong clock on the phone does not change them. The header has one second resolution.
- The too far hint in the browser is advisory. Only the server decides whether a vote counts.
- A finished Rescue report is blurred, and the blur has a limit. Once a Rescue needed report is closed or has faded, `CoarsenFinishedRescue` replaces its latitude, longitude and geohash with the centre of the precision 6 cell (`ApproxGeohashPrecision`, about 1 km across) and empties its description and its distance, all four together in the one function, because blurring only some of them gives the spot back. The blur is the same for every viewer, so one reader cannot see more than another (the distance is the only value that depends on the viewer, and it is measured to the blurred point, never to the exact one). Flood, Shelter and Power reports and live Rescue reports stay exact. The Activity page applies the same blur to a voted card of someone else's finished Rescue report, and the reader's own posts stay exact there (D-27, a recommended default the owner may veto at UAT). A person's profile does not use this blur: it moves every finished report on the profile, a Rescue report included, to the centre of a precision 5 cell through `CoarsenFinishedForProfile` (`TestCoarsenFinishedForProfile`). The limit: anyone who read the report while it was live already saw the exact point and the text, and the blur takes back nothing from them. It only stops a finished report from handing the spot to the next person who looks. `TestFinishedRescueNeverLeaksOnPastOrLink` checks the raw response from two centres, and `TestFinishedRescueCoarsenedOnEveryPath` checks the service paths.
- Rescue needed is no longer offered for a new report (2026-10-16, owner decision): someone who needs rescue and has a phone calls for help, and Traffic took its place in the form. A post that still sends it gets 400 with `RescueNeededRetiredMessage`, checked by `TestSubmitRefusesRetiredRescueCategory`. Reports already stored as rescue needed keep every rule this document gives them until they fade: the critical bypass, no fade stage, the neutral outcome when unconfirmed, and the finished blur through `CoarsenFinishedRescue`. Traffic follows the ordinary rules.
- A phone note near a person can show a coarse place and a distance from their home area. It narrows where they live to roughly a few kilometres, to anyone who can see their lock screen. The device switch Alerts on this device in Settings is the control, and a person with no home area gets neither a Critical nor an official note.
- The Reporter tag on a comment links a public username to a report, which is an accepted risk (D-02). A comment's words are screened for abuse, not for truth, so a false comment can be posted from anywhere; it moves no trust number, and a Flag and a moderator's Remove are the answer. The public share page has a per address limit only when `CLIENT_IP_HEADER` is set; without it only the 128 bit token protects the page.
- Official alerts are copied, not checked. Pinalert shows what NDMA SACHET published, with the issuer, and cannot tell a wrong alert from a right one. They are read only: no one can confirm or dispute them, they are never counted in any crowd number or reporter tag, and they have no rating. The feed is honest about its own age (fresh, stale or unavailable, from the last good check), and Pinalert says it is not an official channel. NDMA's terms for relaying are unconfirmed, and a district is found with a boundary file that can be wrong within about a kilometre of an edge. OFFICIAL-ALERTS.md lists the rest.
- The Demo runs the real resolver and the real vote rules on pretend data, with no exemption from the 1 km rule. It proves the mechanic works, not that any real report is true. Its pretend neighbours are a script, not people, and the Demo's reports and alerts are invented and marked.
- A host that sleeps sends nothing. The dispatcher and the worker run inside the one server process, so on a free host that spins down after a quiet spell no note leaves, the fades in about an hour change note included, until some request wakes it. After it wakes, a report older than an hour is no longer pushed as a new report, and a queued note past its expiry is dropped.
- Delivery is at least once. A crash between a push service accepting a note and the server marking it can send the same note again; the second one replaces the first through its tag on Android and desktop and may show twice on an iPhone.
- Parallel take backs from one account can pass the two undo limit. The undo count is read before the write and the votes log has no lock by design, so every request that is in flight while the count is still below the limit passes the check and writes its own row. The overshoot is therefore the number of extra parallel requests the caller manages to send, not just one: N parallel requests can leave N rows where two were allowed. Interleaved `resolve` and `unresolve` requests can also take a resolve back more than twice, because a resolve is never limited and a late take back that passed its checks earlier still writes. The cost stays small. No rating moves, because an `unresolve` row enters no bucket and `ClassifyOutcome` clears all four resolution fields before it classifies, and no other person's vote is touched. The effect is that one account can flip its own closed mark on and off more often than the limit says, and any later take back is refused once the count shows the limit is used. This was accepted over adding a lock to an append only log (T-31-09). If it ever matters, the fix is either a `pg_advisory_xact_lock` on the account and report before the standing resolve check, or one conditional insert that counts the spent undos inside the same statement as the write and maps zero rows to `ErrUndoLimitReached`.
- Tapped ratings are as hard to fake as confirmations and no harder. Five accounts in five places are enough to move a reporter's points a long way, and nothing stops one person who controls accounts in five places. The full account, and what answers it, is in "Stars and tapped ratings" (T-09-103).
- A public profile links one username to its reports. This is opt in. A finished report on a profile shows a rough area and a date and no time, and carries no link to the report or to the profile, so it cannot be matched to the anonymous Past row at its exact point (D-20). A live report is exact, as on the map. See "Reporter profiles".
- A follow note on a phone can show a distance from the follower's home area, like the other kinds, and nobody's name. A follower with no home area gets the note without the distance words.

## Out of scope for this model

- A dampening curve for the displayed count.
- Sending GPS accuracy with a vote, radii per category or severity, and mock location detection.
- Burst and brigading detection, including a cap on how much one confirmer pair can add to a reporter's record (TRUSTX-03), and confirmer accuracy scoring (TRUSTX-01).
- Vote rate limiting (Phase 4).
- Email alias normalisation and disposable email blocking.
- A dispute extending a report, and any report living past its cap. A confirmation from a new spot does extend a report, up to the cap (see "A confirmation from a new spot keeps a report alive").
- Letting a comment or a like count toward any trust number (Phase 8 built comments and likes and keeps them out of every number, see "Comments and likes").
- Demoting stale non critical reports to Provisional, or dropping them before they expire.
- Phone notifications beyond the kinds "Phone alerts" lists, such as a note when a count crosses a threshold or when a confirmed report goes quiet. Marked resolved and is now disputed reach only the bell of the person who posted or voted on the report, and no phone since 2026-11-03. The Notify me bell was built by Phase 9, one person at a time, and has no general switch in Settings.
- Profile pictures, a profile for someone who has not switched Let others follow me on, or badge tiers beyond the four tags. The opt in reporter profile page is built (see "Reporter profiles").
- Burst and brigading detection for tapped ratings. It is the named dependency of the rating section, and it is not built.
- A stored or materialised reliability score refreshed by a job. Both scores are recomputed on every read.
- Triage ordering by these signals (Phase 6).
- A Demo that bends a rule to look better. The Demo keeps every rule and has no exemption (Phase 5).
- Rating, voting on or counting official alerts. They stay read only.
