# Phase 8: plan review, round 1 (2026-10-27)

Four read only reviews ran on the first 20 plans: three plan checkers (haiku, plans 01 to 07, 08 to 14, 15 to 20) and one security review (opus) of plans 01 to 05, 07, 08, 09, 11 to 14, 16, 17. Every item below goes to the planner as revision round 1 with the orchestrator's decision.

## Blockers

| Id | Plans | Finding | Decision and fix |
|----|-------|---------|------------------|
| B1 | 04 | The per IP limit on `/r/{token}` falls back to the peer address, so without `CLIENT_IP_HEADER` every visitor and WhatsApp's crawler share the host proxy's one bucket: a link forwarded into a busy group locks real readers out in the emergency the page exists for (docs/MODERATION.md: address keyed limits run only when the header is set). | Build the limiter only when `deps.ClientIPHeader` is non empty, like the network backstops; without it admit every request (the 128 bit token already stops walking). Add `TestSharePageNoLimitWithoutClientIPHeader`; set the header in `TestSharePageRateLimited`; fix the router comment. |
| B2 | 01 | `TestTimelineMatchesTrust` says the newest confirmed sentence never exceeds the tally for the prefix, which contradicts "upward steps only" after a take back. | Restate: each milestone's number equals `BuildVoteTally`'s confirm (or dispute) cells at the log row where it was emitted, and is at most the highest tally so far; keep take backs in the random generator. Document in the 08-19 TRUST-MODEL text that a timeline line shows the count when it was written, so after a take back the trust block can read lower than the last line. |
| B3 | 17, 18 | 08-17's Flag control on comments needs `PinalertFlag.openForComment` from 08-18 (same wave, no dependency); run first, the D-14 flag path is silently absent and tests stay green. | 08-17 depends on 08-18; re-wave 08-17, 08-19 and 08-20. |

## Warnings

| Id | Plans | Finding | Fix |
|----|-------|---------|-----|
| W1 | 09 | `CleanCommentBody` refuses U+200C and U+200D, which Indic text (Malayalam, Hindi typed on Gboard) and joined emoji need. | Allow U+200C and U+200D; keep refusing U+200B, U+FEFF, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069 and other format and control characters; add table rows for an Indic word with a non joiner and a joined emoji. |
| W2 | 16 | The textarea `maxlength` is unspecified and would count UTF-16 units. | No `maxlength`, or at least 1120 with a comment; the server counts code points. |
| W3 | 16 | A leading "@word" is drawn bold on every comment, so "@ndma Evacuate now" looks like a mention; Reply on a placeholder fills "@New member ". | Bold the mention only when the comment is a reply and the token equals the replied to author's username, sent by the server as `reply_to_author` only when that username's screen is clear; insert no mention for a placeholder author. |
| W4 | 08, 09, 17 | Placeholder rows still carry `is_reporter`, likes, viewer_liked, viewer_flagged and their time; "Comment deleted" tagged Reporter leaks who deleted it. | In the placeholder branch select is_reporter false, likes 0, viewer_liked and viewer_flagged false; force IsReporter false in the service for deleted or removed views; no Like or Flag on placeholders (08-17); add the cases to `TestCommentsForReportVisibility`. |
| W5 | 09, 12 | Comments decide access differently from the timeline and Share: a report past HistoryWindow is 404 on the timeline but 200 on the comments list and still flaggable. | Use Focus's status (or the same HistoryWindow test) in CommentService; add a "too old answers 404" row to `TestCommentWithheld` and the e2e test. |
| W6 | 14 | The worker notifies on a clear outcome even when a moderator's Keep already cleared and notified it (FinishCommentCheck affected 0 rows), making a second bell row and push. | Notify only when FinishCommentCheck affected exactly one row; test it. |
| W7 | 08, 14 | Deleted held comments stay pending and the worker screens an empty body. | `deleted_at IS NULL` in the claim; optionally clear `next_attempt_at` on delete. |
| W8 | 11, 15 | A refused, removed or deleted comment still makes its author follow the report. | Filter the comment follow arms with `c.deleted_at IS NULL AND c.moderation_state = 'clear'`; test it. |
| W9 | 03 | `BASE_URL` silently defaults to localhost, so share links and og:url would point there once 7.1 removes the emailed link. | In production exit at start (or log an ERROR) when BASE_URL is empty or not https; test the production check. |
| W10 | 07, 04 | Under default A, 08-07 also changes the laptop list row tap to open the panel, beyond the Open report row. `TestSharePageNearMisses` words POST as "does not skip the session middleware", inviting an edit to Phase 5's matcher. | Decision: under default A a Nearby row tap keeps opening the bubble as built (UI-SPEC item 5 and default 9 updated 2026-10-27); "row opens the panel" moves into the "If the owner picks B" section. Reword the near miss as "POST /r/<token> answers 405 with no Set-Cookie"; assert a HEAD request leaves no stray sessions row, or accept it explicitly. |
| W11 | 15, 19, 20 | 08-15 still says the push decision is pending. | The owner decided on 2026-10-27 to keep phone notifications: 08-15 stands; reword its objective and rule 1 and the cut branches in 08-19 and 08-20. |
| W12 | 18 | The moderator empty line "No comments need a look." is invented copy. | Reuse the existing Phase 4 moderation empty state sentence from `moderate.html.tmpl`, quoted exactly. |
| W13 | 19 | `docs/docs.go` and `docs/swagger.yaml` lack the re-read note. | Add "re-read after Phases 5 and 7.1 land", and regenerate from the merged spec. |
| W14 | 07, 09, 14 | 08-07 names no scratch database; 08-09 has no explicit re-read line; 08-14 names `cmd/server/main_test.go` outside files_modified. | One line each: 08-07 "no database; if ever needed, p8_07_a only"; 08-09 "files are new, nothing to re-read"; 08-14 add the file to files_modified or drop it from the note. |
| W15 | 02, 04 | 08-02 cites line numbers in pushdispatch.go; 08-04 cites D-07 for the exempt route. | Use symbol names (`ensureAreaName`, `AreaNamer`); cite "D-05, and the shared page exception in 7.1 D-07". |

Not taken: one checker asked for 08-20 to be `autonomous: false`; the project's end of phase human verify mode keeps the owner checks in the last plan, as in Phases 7.1 and 9.

W12 decision: a new line "No comments to review." was added to the UI-SPEC as P8-C19 rather than reusing a Phase 4 sentence that would misdescribe the list.

# Round 2 (2026-10-27, opus, focused)

Every round 1 item CLOSED with a named test; structure clean across 20 plans in 12 waves. One warning and one note, both fixed in 08-09:

| Id | Plans | Finding | Fix |
|----|-------|---------|-----|
| R2-W1 | 09 | Allowing U+200C and U+200D let a comment made only of joiners pass the empty check and show blank. | Trim joiners with whitespace from both ends, require at least one letter, mark, number, punctuation or symbol, refuse a joiner not after a letter, mark or symbol; four new TestCleanCommentBody rows. |
| R2-I1 | 09 | IsReporter false on your own comments had no stated reason. | UI-SPEC 7a: your own comments show only your username. |

# Execution notes added after planning

- **2026-10-28, Nearby gains an alerts ask in 7.1** (plan 13c, owner decision 07.1 D-17). index.html.tmpl includes `{{template "installed_alerts_ask" .}}` under a condition on `.PushPublicKey` and `.HomeAreaName`, and loads installed-ask.js. page.go gains `PageConfig.Profiles`, `pageViewModel.HomeAreaName` and `pageViewModel.PushPublicKey`.
  - Plans that edit index.html.tmpl or page.go (08-05, 08-06, 08-07, 08-16, 08-17) keep all of these.
  - The ask opens only inside the installed iPhone app and only when no other dialog is open, so no Phase 8 box needs to change for it.
- **2026-10-29, the server's write limit** (from pinalert-3b, Phase 5 quick task 261029-6wb): the production server's WriteTimeout is 10 seconds (cmd/server/main.go, about line 509). Any handler of this phase that can take longer (an outside call, a large export) needs its own write deadline (http.ResponseController), or a bounded context, and a test.
