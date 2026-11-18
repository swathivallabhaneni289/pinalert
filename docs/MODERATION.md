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

A report has one of four `moderation_state` values: `clear`, `pending`, `removed` and, since
quick 261117-vvb (2026-11-17), `review` (hidden for review by flags, see Flag this report).
`Resolve` puts removed and review above every other rung, even a critical report, and pending
and not serious next. All three give `withheld`, which is listable in no view (Active, Disputed,
Resolved or Past).

- A vote or a flag on a withheld report answers 404.
- Only the author sees it, on the Activity page, as "Waiting for a text check" or "Removed after
  review", with no trust block, counters or votes. Since 2026-11-17 the author is also told in
  the bell when moderation shows, hides or removes the report (see Flag this report).
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
  review". Since 2026-11-17 the same statement writes the author one bell row: "Your report was
  removed after review." for a reject, and "Your report passed the text check and is now
  showing." when a held report is released. A pending serious report was already showing, so its
  pass writes nothing.
- A report hidden for review is never claimed and never moved by the worker.
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
- reports with open flags, with a count per reason and the notes people wrote;
- reports waiting for a text check.

Since 2026-11-18 (quick 261118-rz6) two more lists frame them: Review asks comes first and Removed
comes after the three (see Removal reasons, asks for review and Restore below).

The heading counts come from one count query and never from the length of a list. Find a report
by id opens any report, including a held or removed one. Keep and a confirmed Remove each write
one `moderation_actions` row, which holds no report text. No flagger identity is ever shown.
Since 2026-11-18 Remove is confirmed in a "Why remove?" box with a reason, not in the row.

Since 2026-11-17 (quick 261117-vvb) a report hidden for review comes first in the flagged list,
marked "Hidden by flags", with the usual reasons, notes and counts. Keep shows it again, dismisses
its open flags and stamps `review_kept_at`, so flags can never hide it again; later flags still
reach this list. Remove works as for any report. The place cells of the flags are never shown.
Since 2026-11-18 (quick 261118-saj) a Critical report with the Being checked mark comes first
with them, tagged "Still showing"; Keep takes the mark off for good and it alerts again.

## Flag this report

A signed in person can flag a report they did not write, ticking one or more of five reasons:
Spam or gibberish, Abusive or hateful, False or a hoax, Dangerous or illegal, or Something else
(the labels come from `FlagReasonLabel`). Since plan 04-31 (owner, 2026-10-22) the window asks
"Why are you flagging this report?", points to Dispute right under the title for a report that
is just wrong, and has an optional "Tell the moderators more" box of up to 300 characters. Rules:

- One flag per account and report. The reasons are stored as a set (migration 00010), each at
  most once; the older single `reason` field of the API is still accepted on its own as a one
  reason flag, for a page cached before the change.
- The note is trimmed and goes through the abuse screen of a report description's text check
  (`ScreenAbuse`, the screener half of `CheckText`), but not the gibberish rule: a one word note
  such as "fake" or "duplicate" is a good flag note (owner, 2026-10-23). The 300 character limit
  still applies. A note refused as hateful, harassing or sexual answers 422 and nothing is stored. A
  check that does not finish still stores the note, because only moderators read it and a
  provider outage must not stop a flag.
- The moderator screen counts each reason once per flag it appears in and lists each open
  flag's note with its reasons, newest first, at most 20 per report, as text. The note is never
  shown to the author or any reader, and nothing names who wrote it.
- ~~A flag never hides or changes a report. Only a moderator's Remove does.~~ Amended
  2026-11-17 (quick 261117-vvb, owner, 7.1 UAT test 7): one flag still never hides a report, but
  flags from three places do, until a moderator looks. The rule follows below. A flag on a comment
  still never hides it.
- Flagging your own report answers 403, a missing or withheld report 404 and a repeat 409.
- Flags have their own allowance, listed under Limits.
- Police guidance stays plain text on the legal pages. The Flag control sends nothing to anyone
  outside Pinalert.

### Hidden for review (since 2026-11-17)

The owner asked for an action when several people complain about one report, the way community
apps hide a post after several independent complaints until a person looks. Big apps never remove
by flag count alone, and neither does Pinalert: a moderator still decides.

- **Place cells.** A report flag may carry the flagger's position, the same cached fix a vote on
  that page reuses without asking (`PinalertVotes.knownPosition`). There is no new location
  prompt; with no known position the flag is sent without one, as before. The server turns the
  position into a place cell at the voter precision (`voterGeohashPrecision`, 7, about 150 m,
  the cells confirmations count by) and stores only the cell, in `report_flags.flagger_cell`
  (migration 00026). The position itself is never stored. The cell is read only by the hide
  decision: it is in no JSON answer, not on the moderator screen and in no log. Half a position
  answers 400 on the missing field, and a latitude or longitude outside the map answers 400, as
  on the vote route.
- **The rule.** A report is hidden for review when its open (not dismissed) flags come from at
  least `FlagHideThreshold` (3) different place cells. One flag per account means that is at
  least 3 accounts too. A flag with no cell never counts, but still reaches the moderator. Only a
  shown report (`moderation_state = 'clear'`) that no moderator kept after a review hide can be
  hidden; a held or removed report never moves to review, and a pending serious report stays
  pending. Amended 2026-11-18 (quick 261118-saj, owner): a serious report (critical, or a stored
  Rescue needed report, `ReportMeta.Serious`) is never hidden this way. It gets the Being checked
  mark instead; see the next section. The rule above is unchanged for every other report.
- **Exactly once.** The flag, the decision and the author's bell row run in one transaction that
  first takes the report's row lock (`LockReportModeration`, `FOR NO KEY UPDATE`). Two flags on
  one report queue on that lock, so each sees the other's committed flag: concurrent flags can
  neither skip the hide nor make it twice (`TestFlagReviewConcurrent`).
- **Withheld everywhere.** `review` is a fourth `moderation_state` and `Withheld` treats it like
  `removed` at every severity, so every read that already fails closed leaves it out: feeds, map,
  report page and focus, Past, profile, search, comments, share, votes and flags (404), and push.
  The text check worker never claims a report hidden for review and its verdict write never moves
  one; the comment worker does not check held comments on it either. The report's time keeps
  running while it is hidden. A hidden report sends no push of any kind.
- **Only a moderator ends it.** See Moderator screen. Keep stamps `review_kept_at`, so a report
  is hidden this way at most once; a kept report is never pushed again
  (`TestPushReviewHideAndKeep`). Remove works as today.
- **Ratings.** A review hide never moves the author's outcome score, stars or reporter tag,
  either way: while it is hidden the report is judged by its votes exactly as if it were shown
  (amended 2026-11-18, review fix; it was neutral, which let three accounts take a good report
  out of someone's rating or a poor one out of their own). After Keep nothing changes; after
  Remove it counts the way a removal counts (neutral). `TestReviewHideLeavesTheRating`.

### Being checked (since 2026-11-18)

Disputes never hide a Critical report (the critical bypass), so flags must not either: a hidden
Critical report would never alert people who arrive later. The owner wanted a fake Critical report
to get a moderator's attention without hiding a real one. Quick 261118-saj:

- **The mark.** When the open flags of a shown serious report reach the same threshold as above
  (3 place cells, decided in the same transaction and under the same row lock), it is not hidden.
  `MarkReportForFlagCheck` stamps `reports.flag_check_at` (migration 00028) and the report stays
  `clear`. A pending serious report, which is shown while its check retries, gets the mark at once;
  the late check passing it keeps the mark, removing it clears the mark. No bell row is written:
  the report was never hidden.
- **One settled marker.** The mark is on while `flag_check_at IS NOT NULL AND review_kept_at IS
  NULL`. Keep stamps `review_kept_at`, the settled marker of the hide, which takes the mark off and
  stops flags from marking or hiding the report again. `flag_check_at` stays set after Keep: that
  is how push tells a report kept from the mark (alerts again) from one kept after a hide or
  restored (silent, as before). Remove, a late text check removal and Restore clear
  `flag_check_at`.
- **What readers see.** Everything a clear Critical report shows, plus a small flag button next to
  its severity tag (Nearby rows, the map popup, the report page and focus, the Low data rows). A
  mouse resting on it, a tap, Enter or Space show one line, "Being checked."; a second press,
  Escape or a press elsewhere close it. The answer carries only `being_checked` (a boolean, the
  same for every reader), never a count, a cell or a flagger. Resolve never reads it.
- **Alerts pause.** While the mark is on nothing is queued for a phone (the Critical first alert,
  the follow note, the confirmed update, changed notes) and nothing already queued is sent (each
  kind is skipped at send time). The Critical and follow passes are not marked done while the
  mark is on, so after Keep the next evaluation serves every reader in range, the ones who waited
  and the ones who arrived later. The one first alert per person and report index means nobody
  gets the same alert twice; a first alert that was queued before the mark and skipped while it
  was on is not queued again for that person. `TestPushFlagCheckPausesUntilKeep`.
- **The moderator.** A marked report comes first in the flagged list together with the hidden
  ones, tagged "Still showing" in place of the showing word, with the same Keep and Remove. A
  repeat answers 409, as for any handled report.
- **The rating** never moves because of the mark.

The author's bell gets one row per event, written straight to their updates whatever their
follows or mutes, one row per report and kind (`uq_updates_own_report`). They are bell only:
never a phone notification and never an email. They show in the bell and in the full updates list
(`/profile?part=updates`) on phone and laptop, unread like other rows.

Amended 2026-11-18 (quick 261118-rz6): an own event can now happen again (a second Remove after a
Restore, a second Keep removed). The writers then move that one row's time to now and make it
unread (an upsert on the same key, `read_at` cleared), for every own kind, so it comes back as the
newest unread row and the bell never piles up. It also takes a fresh id: the bell's read mark
clears every row up to the largest id shown, so a row that kept its old id would be marked read by
a list drawn before it came back (review fix, same day).

| Event | The row says | Tapping it opens |
|---|---|---|
| A held report becomes shown, by the late check or a moderator Keep (`own_shown`) | Your report passed the text check and is now showing. | The report |
| Hidden for review by flags (`own_review`) | Your report is hidden while it's reviewed. | Nothing (a plain row) |
| A moderator keeps it after a review hide (`own_kept`) | Your report is showing again. Sorry for the wait. | The report |
| Removed, by a moderator's Remove from any state or the late text check (`own_removed`) | Your report was removed after review. | ~~The Policy page~~ The author's box while it is still removed (since 2026-11-18), else nothing |
| A moderator restored it (`own_restored`, since 2026-11-18) | Your report is back. | The report, while it is available |
| A moderator answered an ask with Keep removed (`own_stays_removed`, since 2026-11-18) | Your report stays removed. | The author's box while it is still removed, else nothing |

No row is written while a report waits for the text check: the message at posting already says
so. A row about the author's own withheld report never turns into "This report is no longer
available" and never shows the report's words. The shown and kept rows open the report only while
it is not withheld. The existing follow path (`DecideUpdate`) still writes nothing for a withheld
report. The gaming analysis is in [docs/TRUST-MODEL.md](./TRUST-MODEL.md).

Tests: `TestFlagReviewHide`, `TestFlagReviewConcurrent`, `TestReviewUntouchedByTextCheck`,
`TestOwnShownAndRemovedRows`, `TestReviewIsWithheldEverywhere` and `TestPushReviewHideAndKeep` in
`internal/service`, `TestReviewHideFlow` and `TestWithheldReportsInNoFeed` in
`internal/api/handlers`, and `TestMigration00026` in `internal/store`.

### Removal reasons, asks for review and Restore (since 2026-11-18)

The owner asked (7.1 UAT, 2026-11-17 and 18) that a removed report says why, that its author can
ask for a second look, and that the moderator can let people see it again, all in few words.
Before this a removal was final; now only a moderator's Restore undoes it. Reports only: comment
moderation is unchanged.

- **Why.** Every move to removed records one reason on the report (`reports.removed_reason`,
  migration 00027): Spam, Abusive, False, Dangerous or Other from the moderator, or `text_check`
  when the late text check removed it. The moderator's audit row keeps the same five (never
  `text_check`). `reports.removed_at` keeps when, for the Removed list. Both are cleared by
  Restore and set on no report that is not removed. Reports removed before 00027 have no reason
  and show no Why line; their time is taken from the latest Remove action, else the rejecting
  check, else when the report was made.
- **Remove needs a reason.** Remove report, from any list or Find, opens a "Why remove?" box built
  on the Log out box: the five reasons as one radio group of real buttons, then Cancel and Remove.
  Remove stays off until a reason is picked. The API answers 400 on `reason` without one of the
  five (`POST /api/moderation/reports/{id}/remove` with `{"reason":"false"}`). Keep is unchanged.
- **The author's box.** The bell rows "Your report was removed after review." and "Your report
  stays removed." open a box, in the bell and in the full updates list, rendered as a child of the
  page body so nothing clips it on iPhone Safari. Title Removed, the line "Why: False" (none
  without a recorded reason), then one state: a "Note (optional)" box of at most 200 characters
  with Close and Ask for review; "Asked." while an ask is open; "No asks left." after three. It
  never shows the report's words, flag counts, who flagged, places or the moderator.
- **Asks.** `GET` and `POST /api/reports/{id}/review-ask`. Only the report's author, only while it
  is removed: a stranger, a shown, held or hidden report and an unknown id all answer the same
  404. One open ask at a time and at most `ReviewAskLimit` (3) per report in total, counted across
  Restores and later removals; a fourth or a second open ask answers 409 with a plain line. The
  ask locks the report row first and decides in the same transaction, and a partial unique index
  (`uq_report_review_asks_open`) backs the one open ask. The note is trimmed plain text for
  moderators only: never sent to the text check, never logged, never in a public answer.
- **Review asks list.** First on the moderator screen: each open ask, oldest first, at most 100,
  with the report as other rows show it, "Why: X" and "Note:" with the author's note when given,
  and Restore and Keep removed.
- **Removed list.** Reports removed in the last 30 days, newest removal first, at most 100, each
  with "Why: X" and Restore.
- **Restore** (either list): removed to clear, the reason and time cleared, open flags dismissed,
  `review_kept_at` stamped (so flags can never hide it again and it never sends a phone alert
  again), an open ask closed as restored, a `restore` audit row and the author's `own_restored`
  row. The time it lasts is not extended: an expired report comes back into Past only.
- **Keep removed** (only with an open ask): the ask closed as kept_removed, a `keep_removed` audit
  row and `own_stays_removed`, which opens the box again so the author may ask while fewer than 3
  asks are used. Both answers take the check row lock and then the report row lock, like Keep and
  Remove; a repeated or stale answer answers 409 and writes nothing.
- **A later Remove** of a restored report works as before, needs a reason and brings
  `own_removed` back unread.
- **Ratings.** A restored report counts by its votes like any shown report; a removal counts as a
  removal always has (neutral). Asks change nothing (`TestRestoredReportCountsInTheRating`).

Tests: `TestRemoveNeedsAReason`, `TestLateTextCheckRecordsTextCheck`, `TestReviewAskRules`,
`TestReviewAskConcurrent`, `TestRestoreAndKeepRemoved`, `TestRestoreUnderTheRowLock`,
`TestRestoredReportCountsInTheRating`, `TestUpdatesListBoxRows` and
`TestPushRestoredReportNeverPushes` in `internal/service`, `TestReviewAskFlow` in
`internal/api/handlers`, `TestMigration00027` in `internal/store`, and
`web/moderate_remove_test.mjs` and `web/updates_test.mjs` for the two boxes.

## Comments

Phase 8 added comments on a report, one level of replies, a thumbs up like and a Flag on each
comment. Anyone signed in with a username may write one. The rules (D-12 to D-15):

- The text check is `ScreenAbuse`, the same abuse screen a Flag note and a username use, and not
  the gibberish rule. A short reply such as "ok" or "same here" is gibberish by that rule and is a
  good comment (`TestCommentShortReplies` shows it). The words are cleaned and bounded first
  (`CleanCommentBody`, 1 to 280 characters, no control characters), with no provider call, so a
  refused text, a closed report or a reply to a gone comment costs no check.
- Three outcomes. Rejected (hateful, harassing or sexual): 422 on field `words`, the box keeps
  its text, nothing is stored. Borderline: the comment is posted, shows to everyone, and is listed
  for moderators. Unfinished (the provider cannot answer): the comment is held. Only its author
  sees it, marked as held, and nobody is told about it, until the check passes. Comments have no
  Critical exemption (`TestCommentScreeningOutcomes`).
- The comment worker rechecks held comments every 30 seconds and once at start, with the same
  claim, lease, backoff and error classes as the report worker (`TestCommentWorkerBacksOff`). A
  pass makes the comment visible and sends its bell row. A borderline result makes it visible and
  lists it. A rejection removes it with no notice. A comment whose report expired, or that its
  author deleted meanwhile, is left alone (`TestCommentWorkerStopsAtExpiry`,
  `TestCommentWorkerSkipsDeleted`). A moderator's Keep or Remove and the worker never both act on
  one comment (`TestCommentWorkerNoDoubleNotify`).
- A flag never hides a comment. Anyone signed in can flag a comment they did not write, with the
  same reasons, the same optional note and the same flag allowance as a report flag. One flag per
  account and comment, a repeat answers 409, flagging your own comment answers 409. Only
  a moderator's Remove takes a comment down. Your own delete is allowed any time, there is no
  editing, and a deleted comment that has replies leaves the line Comment deleted so the thread
  still reads. The reporter cannot delete other people's comments on their report (D-14).
- Comments close with the report (D-15). They stay open only while a report is active or disputed,
  and become read only, with the line Comments are closed, once it is resolved, faded or withheld.
- The moderator Comments list is `GET /api/moderation/comments`, behind the same plain 404 gate as
  the report routes and left out of the public API reference on purpose. It lists, newest first and
  capped at 200, each comment that is not deleted or removed and has an open flag, is marked for
  review, or is held. A row shows the words, the category, the reasons (Flagged by people, Near the
  line, Check not finished), the flag reasons and the count. It carries no account, email or
  username. Keep and Remove run in one transaction: change the comment, dismiss its open flags and
  write one `comment_moderation_actions` row (comment, moderator, keep or remove, time) that holds
  no comment text. Keep of a comment that needs nothing answers 409 and a repeat Remove answers
  404 (`TestCommentModerationAuditInTx`, `TestCommentRemove`, `TestModerateComments`).
- A comment's author is shown only by a username whose screen word is clear. A name that could not
  be judged reads New member. A deleted account reads Deleted account, and deleting an account
  blanks its comments and removes the likes and flags it gave (`TestDeleteAccountComments`).
- Every comment by someone else has a Block button beside Flag, but Block is offered only where the
  name is shown: a New member comment (a username that is pending or missing) has none.
  `POST /api/comments/{id}/block-author` blocks the commenter whether or not their profile is
  public, and also when they blocked you, so the answer reveals neither. Because the name is clear,
  every block made from a comment is listed in Settings, Blocked people and can be taken back there,
  and a block cannot tell you whether a person whose name is not shown follows you. It is the same
  block as from a profile: the block row is written
  first, the follows end both ways with reason blocked, the queued follow notes of the pair are
  deleted, comment and Reliable reporter notes about that person stop at send time, and their
  comments are hidden from you. It answers 204, also when you already blocked them. Your own
  comment, an unknown one, one you cannot see (deleted, removed, held, on a report that is not
  available, or on a finished Rescue report), one by a deleted account and one by a New member all
  answer the same 404, the same bytes as a comment that is no longer available. Like Flag it stays
  available after the comments close. It spends the people action bucket in the Limits table below.
- Comment limits are per account and are in the Limits table below. A comment post is never
  given back, because each one may be a provider call. A refused 429 says how long to wait and
  that the words are still in the box.

Tests: `TestCommentScreening`, `TestCommentFlagRoutes`, `TestCommentRateLimit`,
`TestLikeRateLimit`, `TestBlockCommentAuthorBlocksAnyCommenter`, `TestBlockCommentAuthorUniformRefusal`,
`TestBlockCommentAuthorStopsCommentNotes`, `TestBlockAccountSkipsADeletedAccount`,
`TestCommentBlockAuthorRoute`, `TestCommentBlockAuthorUniformNotFound`, `TestCommentBlockAuthorRateLimit`.

## Usernames

Phase 7.1 gave every account a public username. It is screened like other short free text:

- The check is `ScreenAbuse`, the same abuse screen a Flag note uses, and not the gibberish
  rule, because a username is one word and the gibberish rule would refuse it. The text sent to
  the provider is the name only, with underscores and full stops read as spaces
  (`UsernameScreenText`). No account, address or location goes with it.
- The shape and the reserved words are checked first, with no provider call: 3 to 20 letters,
  digits, underscores and full stops, at least one letter, and no name that looks like an official
  or emergency word (look alike digits are read as letters). The live check as you type uses only
  these rules and the uniqueness lookup. It never calls the provider.
- A name the screen rejects as hateful, harassing or sexual is refused (422, field `username`)
  and nothing is stored.
- A check that cannot finish, or a borderline score, keeps the name as pending (D-16). The
  person is not told, and can use the site as normal. Phase 8 and Phase 9 show only a name whose
  screen word is clear, and re-check a pending one first.
- A name can be changed once every 7 days. Saves and live checks have their own per account
  limits, and a refused guess still uses a token.

Tests: `TestUsernameSaveScreening`, `TestUsernameSaveRules`, `TestCheckUsernameStates`.

## Names on a public profile

Phase 9 shows a person's Name on their profile once Public profile is on (named Let others follow
me before quick 261117-lq0, amended 2026-11-17). The Name is screened when the switch goes on and on every later edit of the Name while it is on. The reserved words come
first, with no provider call, and then `ScreenAbuse`, the abuse screen again and not the gibberish
rule. It fails closed: a borderline, rejected or unfinished check refuses the Name, because a Name
is shown to strangers and nothing is held for later. A pending username is screened again when the
switch goes on. The reserved list is ASCII, so a Name in another script is screened for abuse and is
not compared with the reserved words. Only the Name goes to the provider, never an account, an
address or a location.

Tests: `TestScreenPublicName`, `TestPublicNameEditWhilePublic`.

## Ratings

Phase 9 lets a reader give a reporter one to five stars. A moderator can remove abusive ratings; this
is the only tool for it (D-14). The rules that decide what counts are in
[docs/TRUST-MODEL.md](./TRUST-MODEL.md), under "Stars and tapped ratings".

The Ratings section of the moderator screen sits below the report lists. A moderator types a
reporter's username and reads who gave how many stars and when, newest first, at most
`ModeratorRatingsCap` (200) rows, with removed and cleared ratings marked and only live ones given a
Remove button. A rater is named by username on this screen and on no other. A rater
whose username is still pending is named too, because the tool acts by name and hiding the name
would make that rating impossible to remove (a moderator only exception, decided in plan 09-15, open
for the owner to revisit). Three routes sit in the same plain 404 group as the other moderator
routes, so everyone who is not a moderator gets the byte for byte 404 of an unknown address, on every
method, and none is in the public API reference:

- `GET /api/moderation/people/{username}/ratings` lists the ratings of one reporter.
- `POST /api/moderation/people/{username}/ratings/remove` removes one rating, named by the rater in
  the body, and answers the list as it stands now.
- `POST /api/moderation/raters/{username}/remove-ratings` removes every rating one rater gave to
  anyone and answers how many it removed.

What each removal does.

- Removing one rating. The row stays, with `removed_at` and `removed_by` set to the time and the
  moderator's account. The audit stamp is the record: it holds no text. The rating stops counting on
  the next read, on every surface at once, so the stars, the tag and the "Includes ratings by N
  people." line move together (`TestRatingsIgnoreRemoved`, `TestModeratorRatings`).
- Removing every rating by one rater. This asks first, in a box titled "Remove all ratings by
  {username}?" with Cancel and Remove all and no close mark. It marks the rater first
  (`rater_removed_at`, `rater_removed_by`), so their ratings stop counting even if a later step
  fails, and then stamps each rating. The removal is permanent: that account cannot rate again
  (`TestRemovedRaterCannotRateAgain`). It affects every reporter that rater rated.
- Follows. When a removal brings a reporter back to Reliable reporter, the follows that were ended
  with the reason tag_lost since the earliest removed rating was first written are restored
  (`TestModeratorRemovalRestoresFollows`). A follow comes back only if the reporter still has a
  public profile and a clear username, neither side is deleted, no block exists in either
  direction and the follower is under the following cap. When the reporter is not Reliable again,
  nothing is restored, and follows lost for other reasons never come back.
- Nobody is told. The rater is not told a rating was removed, and the reporter is not told who
  rated them. A removal changes no report.

Not covered. There is no screen for a rater with no username or a deleted account, because the tool
acts by name, and deleting an account already removes the ratings it gave. Burst detection for a
group of accounts rating together is not built (TRUSTX-03), so a moderator finds a burst by looking.

Tests: `TestModeratorRatings`, `TestModeratorRatingsHiddenFromOthers`, `TestRatingsIgnoreRemoved`.

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
| Comments and replies | account | 6 | 1 per 20 seconds | `commentBurst`, `commentEvery` |
| Likes, unlikes, own comment deletes | account | 30 | 1 per 3 seconds | `likeBurst`, `likeEvery` |
| Share link | account | 10 | 1 per 30 seconds | `shareBurst`, `shareEvery` |
| People reads (profiles, lists) | account | 60 | 1 per second | `peopleReadBurst`, `peopleReadEvery` |
| People actions (follow, block, mute, notify and the rest) | account | 20 | 1 per 3 seconds | `peopleActionBurst`, `peopleActionEvery` |
| Star ratings and removing your own rating | account | 10 | 1 per 30 seconds | `rateBurst`, `rateEvery` |
| Visibility (turning the profile on; turning it off is never limited) | account | 6 | 1 per 20 seconds | `visibilityBurst`, `visibilityEvery` |
| Network backstop, reports | client address | 2000 | 1 per second | `ipBackstopReportBurst`, `ipBackstopReportEvery` |
| Network backstop, votes | client address | 2000 | 1 per 150 milliseconds | `ipBackstopVoteBurst`, `ipBackstopVoteEvery` |
| Network backstop, flags | client address | 1000 | 1 per 1.2 seconds | `ipBackstopFlagBurst`, `ipBackstopFlagEvery` |
| Network backstop, comments | client address | 600 | 1 per 200 milliseconds | `ipBackstopCommentBurst`, `ipBackstopCommentEvery` |
| Network backstop, likes | client address | 3000 | 1 per 30 milliseconds | `ipBackstopLikeBurst`, `ipBackstopLikeEvery` |
| Network backstop, share | client address | 1000 | 1 per 1.2 seconds | `ipBackstopShareBurst`, `ipBackstopShareEvery` |

The account block holds a burst and a refill for each account bucket, and the backstop block
beside it does the same for each action family. Phase 8 added the comment, like and share buckets
to both (a comment Flag spends the report Flag bucket and backstop). Amended 2026-10-29 by Phase 8
plan 19.

Phase 9 added the four people rows. They are per account only and have no network backstop,
because many phones share one carrier address: the lists are public to every signed in account, so
reads are bounded to slow scraping of the follow graph, and actions are bounded to slow follow
churn. A rating also has the once in 24 hours change rule on top (`RatingChangeGap`, see
TRUST-MODEL.md). Amended 2026-10-30 by Phase 9 plan 27.

The Visibility row (review WR-E02) takes a token for every request that asks for on, including a
request that finds the profile already on, because the handler takes the token before it knows the
stored state; that request calls no screener and changes nothing, but it still spends a token.
Each real switch on can send the Name to the screener, which is why the bucket exists; turning it
off is never refused by it. Amended 2026-10-30 (skeptic SK-I02). A refusal
is a 429 with the field `rate_visibility`, the same body shape and Retry-After as
`rate_people_action`. The profile page `/u/{username}` takes its token from the People reads bucket
(review WR-E01), the same one the JSON profile and lists use, so a person cannot walk the pages
round the JSON limit. When that bucket is empty the page answers a 429 page with Retry-After, never
JSON. Amended 2026-10-30 by the Phase 9 round 2 fixes.

- The network backstop has one bucket per action family, so votes from a shared carrier address
  can never use up report posting from it. Each family is sized so that about 100 accounts behind
  one address, each at its own account limit, never meet it, while one machine flooding from one
  address is still capped. A backstop refusal says "A lot of requests are coming from your network
  right now", never that the person acted too fast.
- The vote change cooldown never refuses the take back of a Mark resolved, so the Undo on the
  message works within its 8 seconds; the limit of two undos per report bounds it instead, and a
  quick resolve again after an Undo still waits out the cooldown.

- The network backstop runs only when `CLIENT_IP_HEADER` is set. Until it is set on a host that
  sits behind a proxy, the existing per address limits (the sign in and code buckets and place search)
  see one shared address for every visitor. Amended 2026-10-29 by Phase 7.1: the login link
  requests this line once named are gone; the sign in limits are in TRUST-MODEL.md, "Signing in". Set it only after the host canary shows which header
  carries the visitor address. Amended 2026-10-29 by the Phase 7.1 review fix: outside
  `ENV=development` the server will not start without it.
- Every 429 carries a `Retry-After` header and a JSON body with `retry_after_seconds`,
  `serious` and, for a blocked serious report, `emergency_number`, the configured primary number.
- For a report post, a 400, a text or gibberish reject, any failed post and a replay cost nothing
  from the report allowance, because it is given back. A held post keeps its cost. The submit
  attempt bucket, which protects the provider, is never given back. A flag is different: nothing
  is given back, so a flag that ends in a 400, 404, 409 or 422 (a refused note) still uses a token.
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
finishes, see Text check), and moderator actions without the report text. Phase 8 adds comments
(the words, with the screen verdict and scores), the likes an account gave, comment flags with
their notes, and comment moderator actions without the comment text. Deleting an account blanks
its comments and removes its likes and flags. Phase 9 adds the profile switch with the time it was
switched on, who follows whom (each follow kept with an end time and a reason after it ends, until
the account goes), the Notify me choice with the time it was switched on, blocks, mutes, and the
star ratings an account gave (the person rated, the stars, the time, and for a removed rating the
moderator who removed it and when). A rating its rater clears with Remove my rating stays stored,
marked cleared with the time, and the moderator screen still lists it as cleared. When a moderator
removes every rating one account gave, the account is also barred from rating for good, and the
moderator and the time are kept on the account. Deleting an account removes its follows, blocks, mutes, bell choices and the ratings it gave
and received in the one transaction, and switches its profile off. Quick 261118-rz6 adds the reason
for a removed report and when it was removed, and each ask for review with the author's note, the
answer, when it was answered and the moderator who answered. Deleting an account keeps its asks
and their notes, like its reports. A report's share token stays with the report. An expired report only
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
it links (each icon counted once), and fails when the sum
passes the budget. Amended 2026-10-29 by Phase 5 plan 07: the one inert globe rule that was
excluded here left auth.css with the flat picture globe, so nothing is excluded now. The real
globe's styles live in `login-globe.css`, which the Low data page never links.

Measured on 2026-10-16, after plans 04-24 and 04-25 made their last Low data edits:

- measured total: 509094 bytes raw, made of 10338 bytes of HTML, 178817 bytes of stylesheets
  (7 files), 308607 bytes of scripts (10 files) and 11332 bytes of icons (25 files);
- gzip level 5 (the level the router uses) for the same files: 153048 bytes, informational only;
- budget: 585728 bytes, which is the measured total plus 15 percent, rounded up to the next
  1024 bytes.

Re-measured on 2026-10-17 by Phase 3.4 plan 12. The Call 112 box and the What is happening? picks
grew `report-form.js` and `modal.css`, which the Low data view links although it shows neither
(the Low data form keeps the form it was built with). The Phase 3.4 plans before it had already
used most of the headroom above (the total was about 581000 bytes before this change). The
figures now are:

- measured total: 590992 bytes raw, made of 9891 bytes of HTML, 195049 bytes of stylesheets
  (7 files), 372643 bytes of scripts (10 files) and 13409 bytes of icons (28 files);
- gzip level 5 for the same files: 176129 bytes, informational only;
- budget: 679936 bytes, the new measured total plus 15 percent, rounded up to the next 1024
  bytes.

Re-measured on 2026-10-28 by Phase 3.3 plan 21, when the Low data view started to link
`changes-ask.js` and render the changes pop-up for an account whose Changes switch is off. Most of
the growth since the figures above did not come from that plan. Measured at the commit before the
Phase 3.3 gap fix (3bf9e2a), the total was already 673066 bytes: about 82 KB (82074 bytes, 13.9
percent) had come from earlier work since 2026-10-17 that was never re-measured or looked at
here. The gap fix itself (Phase 3.3 plans 17, 20 and 21) added about 11.5 KB (11531 bytes:
`changes-ask.js` 8595, `notify.js` about 1937, `votes.js` 473, `report-form.js` 429 and the page
97). That made 684597 bytes, 4661 over the budget of 679936, and the budget was set again from
it:

- measured total: 684597 bytes raw, made of 11838 bytes of HTML, 229635 bytes of stylesheets,
  429339 bytes of scripts and 13785 bytes of icons;
- gzip level 5 for the same files: 204179 bytes, informational only;
- budget: 787456 bytes, the new measured total plus 15 percent, rounded up to the next 1024
  bytes. Nothing was dropped or split to meet it.

Setting the budget from 684597 also gave the 15 percent headroom to the earlier 82074 bytes,
which nobody had examined. Whether that earlier growth was intended is still for the owner to
decide.

Re-measured on 2026-10-28 by the Phase 3.3 code review fixes. `changes-ask.js` now loads only
with the changes pop-up, inside the same `{{if .ChangesAsk}}` as the partial (fix WR-03), so an
account whose Changes switch is on no longer loads it. The test page has no push key, so it
renders no pop-up and no longer counts `changes-ask.js`; fixes CR-02 and WR-02 grew `notify.js`.
The figures now are:

- measured total: 676839 bytes raw, made of 11773 bytes of HTML, 229635 bytes of stylesheets,
  421646 bytes of scripts and 13785 bytes of icons;
- gzip level 5 for the same files: 201292 bytes, informational only;
- budget: 779264 bytes, the new measured total plus 15 percent, rounded up to the next 1024
  bytes. A page that shows the pop-up adds `changes-ask.js` (11108 bytes) and the partial, about
  12 KB in all, which stays inside it.

Removed on 2026-11-03 by quick 261103-jal (an owner decision): the changes pop-up, its partial
and `changes-ask.js` are gone from the Low data view, so the page only got smaller. The figures
above are history and the budget stands.

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
