# Moderation, limits and resilience

Phase 4 added a text check on every new report, a held and removed state for reports that fail
it, a background retry for checks that could not finish, a moderator screen, a Flag this report
control, per account limits on posting, voting and flagging, and a size budget for the Low data
view. This file is the design record: each section says what the code does and names the tests
that prove it, so a reviewer can check the words against the code. Where the code and this file
ever differ, the code is right and this file is stale. The reporter rating is written up in
[docs/TRUST-MODEL.md](./TRUST-MODEL.md) and is not repeated here.

## Text check

Every new report description goes through the same steps, in this order.

1. The gibberish rule runs first. It counts characters (runes), not bytes, and it is the only
   spam rule. A text is gibberish when one letter repeats ten or more times in a row
   (`GibberishRunLength`), or when it holds fewer than two words of two or more letters. Digits,
   punctuation, emoji and phone numbers never make a run, and the rule never looks at capitals.
   A gibberish post is refused and the person is asked to say what is happening.
2. One call goes to OpenAI's moderation endpoint with only the trimmed description. It runs
   inside a 3 second deadline (`InlineScreenTimeout`) and is never repeated inline, whatever the
   error. The model is pinned to the snapshot `omni-moderation-2024-09-26` (`DefaultModel` in
   `internal/moderation/client.go`), and `MODERATION_MODEL` can override it.
3. `Decide` turns the category scores into a verdict. Only hateful, harassing and sexual scores
   reject, and they reject at every severity: choosing Critical never skips the check, because
   severity is self declared (`TestDecideIgnoresSeverity`). Violence, self harm and illicit
   scores reach at most borderline, because genuine disaster reports use those words. A
   borderline report posts and is listed on the moderator screen, which shows the report text
   from the report itself.

What is stored and what is not:

- The stored check result holds the verdict, the category scores and the model name. It holds no
  copy of the text. A row is written inline only for a borderline result and for an unfinished
  check. When the background retry later finishes a check, it fills that row with the verdict and
  scores whatever the verdict is, so a late pass or a late reject also keeps its scores. A clear
  or rejected result at submit stores no row.
- A rejected post stores nothing and tells the person what the problem was.
- No account, session, network address or coordinates go to the provider, only the description.
  OpenAI documents no retention and no training use for that endpoint, and the Policy page says
  so in its own words.
- With no key set and `ENV=development`, every check passes and the server logs a WARNING at
  startup. Outside development a missing `OPENAI_API_KEY` stops startup.

An unfinished check (a timeout, a provider error, a quota answer) does not lose the report. A
report that is not serious is held off every feed. A serious report (critical, or rescue needed)
is published straight away, because a delay there can cost more than a wrong word. The background
retry decides later, as described under Background retry.

Both thresholds are compared at or above. They are starting values and are tuned only on scores
recorded from the pinned snapshot, never by guessing.

Thresholds in use: RejectScoreThreshold 0.85, BorderlineScoreThreshold 0.40

Tests: `TestDecide`, `TestDecideIgnoresSeverity`, `TestDecideThresholdsOrdered`, the gibberish
tests in `screening_test.go`, and `TestSubmitScreening`, `TestSubmitScreeningHeldNotInFeed` and
`TestSubmitScreeningPendingSeriousPublished` in `internal/api/handlers`.

## How accurate the text check is

The fixture lives in `internal/service/testdata/screening/`. It holds 200 genuine reports in
English, Hindi, Telugu, Tamil and Hinglish (40 each) with urgent wording, all capitals,
addresses and phone numbers, and 40 abusive examples. `TestScreeningFixtureShape` checks the
shape of both files, and `TestScreeningFixture` checks the recorded scores against the policy:
no genuine report rejected, no serious report withheld, every abusive example rejected.

When the recorded run rejects none of the 200 genuine reports, that shows only that the true
rate of wrongly rejected reports is probably under about 3 in 200 (about 1.5 percent), not that
it is 0.1 percent. The 0.1 percent figure in PROJECT-NOTES.md is a design target, not a result.
The fixture text was written for this test and is not real traffic. Real calibration needs real
traffic and the decisions made on the moderator screen.

OpenAI names Telugu, Bengali and Marathi in its language claims but not Hindi, Tamil or
Hinglish, so support for those is best effort until the recorded fixture shows otherwise.

## Recording the fixture scores

The scores are recorded once, by the owner, on their own machine, never in CI:

```bash
read -rs OPENAI_API_KEY
MODERATION_LIVE=1 OPENAI_API_KEY="$OPENAI_API_KEY" go run ./cmd/modrecord
unset OPENAI_API_KEY
```

The command, run from the repository root:

- refuses to run without both `MODERATION_LIVE=1` and `OPENAI_API_KEY`;
- always asks for the pinned snapshot and ignores `MODERATION_MODEL`;
- paces itself under 150 requests and 6,000 characters a minute;
- writes only `model` and `scores` into the two fixture files;
- skips entries already recorded with the pinned model, so a stopped run resumes;
- stops at once with a line naming `insufficient_quota` when the account has no usable quota,
  which also answers whether a fresh zero balance key is accepted.

The key is typed into the shell for that one command only and is never saved in a file. With no
scores recorded, `TestScreeningFixture` skips and says so. With some recorded it fails and lists
the missing ids. With all recorded it checks the pinned model and the conditions above. A
recording that stopped early is never committed.

Recording status: not recorded yet.

## Held and removed reports

A report has one of three `moderation_state` values: `clear`, `pending` and `removed`.
`Resolve` puts removed above every other rung, even a critical report, and pending and not
serious next. Both give `withheld`, which is listable in no view (Active, Disputed, Resolved or
Past).

- A vote or a flag on a withheld report answers 404.
- Only the author sees it, on the Activity page, as "Waiting for a text check" or "Removed after
  review", with no trust block, counters or votes.
- It never counts for or against the author's reporter rating, whatever its votes (see
  [docs/TRUST-MODEL.md](./TRUST-MODEL.md)).
- The pinned `NearbyReports` query is unchanged. Moderation arrives through one batched extras
  read, so the feed query itself did not grow.

Tests: `TestWithheldReportsInNoFeed` in `internal/api/handlers`, `TestWithheldInNoView` and
`TestCastVoteRefusesWithheld` in `internal/service`.

## Background retry

A worker sweeps once at start, because the free host cold starts, and then every
`ModerationSweepInterval` (30 seconds). Each sweep claims at most `ModerationBatchLimit` (5)
unfinished checks and reserves each for `ModerationLeaseSeconds` (60), so a second instance would
not screen the same report twice. The provider call in a sweep has its own 8 second limit.

A failed attempt backs off exponentially with jitter: 30 seconds doubled for each earlier
attempt, capped at one hour, scaled by a factor between 0.5 and 1.5, and never sooner than a
`Retry-After` the provider sent (`ModerationBackoff`).

- A late pass releases the report.
- A late borderline releases it as clear and lists it for review.
- A late reject removes it, serious reports included, and the author sees "Removed after
  review".
- An expired report is never retried.

Tests: `TestModerationWorkerBackoff`, `TestModerationWorker`,
`TestModerationWorkerLateRejectRemovesSerious`, `TestModerationWorkerLateBorderlineNeedsReview`
and `TestModerationWorkerSkipsExpired` in `internal/service`.

## Moderator screen

Moderators are the verified accounts whose emails are listed in `MODERATOR_EMAILS`, comma
separated and normalised once at startup. A malformed entry stops startup and names its
position, never the address. Unset means no moderator.

The owner bookmarks `/moderate`. No link to it exists anywhere in the interface. Everyone who is
not a listed moderator gets a 404 that is byte for byte the same as an unknown address, on every
method.

The page loads its scripts in this order: `theme.js` in the head, then `app.js`, `trust.js`,
`moderate.js` and `account-menu.js`, all deferred. Its stylesheets are `main.css`, `trust.css`,
`auth.css`, `chrome.css` and `moderate.css`. It is not part of the Low data view.

The screen has three lists, each oldest first and capped at 100 rows:

- borderline reports listed for review;
- reports with open flags, with a count per reason;
- reports waiting for a text check.

The heading counts come from one count query and never from the length of a list. Find a report
by id opens any report, including a held or removed one. Keep and a confirmed Remove each write
one `moderation_actions` row, which holds no report text. No flagger identity is ever shown.

## Flag this report

A signed in person can flag a report they did not write, with one of five reasons: Spam or
gibberish, Abusive or hateful, False or a hoax, Dangerous or illegal, or Something else (the
labels come from `FlagReasonLabel`). Rules:

- One flag per account and report.
- A flag never hides or changes a report. Only a moderator's Remove does.
- Flagging your own report answers 403, a missing or withheld report 404 and a repeat 409.
- Flags have their own allowance, listed under Limits.
- Police guidance stays plain text on the legal pages. The Flag control sends nothing to anyone
  outside Pinalert.

## Limits

Every limit is a token bucket kept in the server's memory and keyed on the verified account,
never on the session. The one exception is the network backstop, which is keyed on the client
address and runs only when `CLIENT_IP_HEADER` is set. The numbers live in two constants blocks
side by side in `cmd/server/main.go`.

| Bucket | Key | Burst | Refill | Constants |
| ------ | --- | ----- | ------ | --------- |
| Other reports | account | 8 | 1 per 15 minutes | `reportOtherBurst`, `reportOtherEvery` |
| Serious reports | account | 5 | 1 per 5 minutes | `reportSeriousBurst`, `reportSeriousEvery` |
| Votes (all five vote routes) | account | 20 | 1 per 15 seconds | `voteBurst`, `voteEvery` |
| Vote change cooldown | account, report and kind | 1 | 1 per 30 seconds | `voteChangeBurst`, `voteChangeEvery` |
| Flags | account | 10 | 1 per 2 minutes | `flagBurst`, `flagEvery` |
| Submit attempts | account | 20 | 1 per 6 seconds | `submitAttemptBurst`, `submitAttemptEvery` |
| Network backstop, reports | client address | 2000 | 1 per second | `ipBackstopReportBurst`, `ipBackstopReportEvery` |
| Network backstop, votes | client address | 2000 | 1 per 150 milliseconds | `ipBackstopVoteBurst`, `ipBackstopVoteEvery` |
| Network backstop, flags | client address | 1000 | 1 per 1.2 seconds | `ipBackstopFlagBurst`, `ipBackstopFlagEvery` |

The account block holds 12 constants, a burst and a refill for each of the six account buckets,
and the backstop block beside it holds 6 more.

- The network backstop has one bucket per action family, so votes from a shared carrier address
  can never use up report posting from it. Each family is sized so that about 100 accounts behind
  one address, each at its own account limit, never meet it, while one machine flooding from one
  address is still capped. A backstop refusal says "A lot of requests are coming from your network
  right now", never that the person acted too fast.
- The vote change cooldown never refuses the take back of a Mark resolved, so the Undo on the
  message works within its 8 seconds; the limit of two undos per report bounds it instead, and a
  quick resolve again after an Undo still waits out the cooldown.

- The network backstop runs only when `CLIENT_IP_HEADER` is set. Until it is set on a host that
  sits behind a proxy, the existing per address limits (login link requests and place search)
  see one shared address for every visitor. Set it only after the host canary shows which header
  carries the visitor address.
- Every 429 carries a `Retry-After` header and a JSON body with `retry_after_seconds`,
  `serious` and, for a blocked serious report, `emergency_number`, the configured primary number.
- For a report post, a 400, a text or gibberish reject, any failed post and a replay cost nothing
  from the report allowance, because it is given back. A held post keeps its cost. The submit
  attempt bucket, which protects the provider, is never given back. A flag is different: nothing
  is given back, so a flag that ends in a 400, 404 or 409 still uses a token.
- Every block is logged with the bucket name, the numeric account id and the wait, and nothing
  else.

Memory store consequence: the buckets live in the server's memory, so every restart hands every
account a fresh allowance, and two instances would each keep their own. At this scale that is
accepted, because a moderator's Remove is the backstop. Running more than one instance needs a
shared store first (T-04-09).

Tests: `TestPerKeyConcurrentBound`, `TestPerKeyRefundAtReservationInstant`,
`TestReportRateLimitRefunds`, `TestVoteRateLimit`, `TestFlagRateLimit`,
`TestSubmitIdempotentConcurrent`, run under the race detector.

## Retention

Nothing is deleted automatically today, because there is no retention job. Kept: accounts,
reports with their location, votes as a rough precision 7 square, flags, text check results
without the report text (borderline results at submit, and every result the background retry
finishes, see Text check), and moderator actions without the report text. An expired report only
stops showing in the lists. The Policy page says the same in general and offers removal through
the contact line when `CONTACT_EMAIL` is set. One small gap: the Policy page lists the scores
"for borderline reports", which understates the retry case above. If the Policy page is edited,
this section must be edited with it.

## Seeded and demo data

Phase 5's demo and seeded reports are inserted with SQL as `moderation_state` `clear` with no
`moderation_checks` row. They never call the provider, and they are exempt from the limits by
construction, because the limits sit in the HTTP handlers (`TestSeededReportIsClear`).

## Low data view size budget

The Low data view page and everything it links are held under a byte budget so the view stays
usable on a weak connection. `TestLowDataByteBudget` in `web/low_budget_contract_test.go`
renders the real `/low` page, adds the raw bytes of every local stylesheet, script and icon file
it links (each icon counted once, and the one inert globe rule excluded), and fails when the sum
passes the budget.

Measured on 2026-10-16, after plans 04-24 and 04-25 made their last Low data edits:

- measured total: 509094 bytes raw, made of 10338 bytes of HTML, 178817 bytes of stylesheets
  (7 files), 308607 bytes of scripts (10 files) and 11332 bytes of icons (25 files);
- gzip level 5 (the level the router uses) for the same files: 153048 bytes, informational only;
- budget: 585728 bytes, which is the measured total plus 15 percent, rounded up to the next
  1024 bytes.

Raw bytes are tested although the first design described a compressed budget (orchestrator
decision 2). The router now compresses text, but the transfer size depends on the client, so raw
bytes are the conservative bound.

The first planning guess was 256 KB (262144 bytes). The measured total is above it by 246950
bytes. Nothing was dropped or split to meet a number. The five largest files are `votes.js`
(81123 bytes), `trust.css` (63693), `app.js` (45625), `report-form.js` (41375) and `main.css`
(37271). Phase 4 added four scripts to the page (`report-form.js` 41375, `queue.js` 34837,
`low.js` 33772 and `flag.js` 16433, 126417 bytes together) and two stylesheets (`chrome.css`
13814 and `low.css` 3041, 16855 bytes together).

No icon file was dropped to meet the budget. The icons are counted inside it, and dropping one
would need a `low.css` rule that plan 04-27 does not own.

To change the budget after an intended change, re-measure with
`go test ./web/ -run TestLowDataByteBudget -v`, set `lowDataMeasuredBytes` and
`lowDataByteBudget` together in one commit, and update this section.
