---
phase: 08-report-page-and-comments
verified: 2026-10-29T21:00:00Z
status: human_needed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification: false
gaps: []
deferred: []
human_verification:
  - test: "Phone report page in the iPhone simulator (never the iPad), widths 320, 360, 390, light and dark"
    expected: "History, Share, comments, thumbs up, replies, Delete, Flag, closed, removed, deleted and held comments all read well; nothing jumps on a poll"
    why_human: "Look and feel on a real phone shell; Playwright WebKit has passed things iPhone Safari showed broken"
  - test: "Laptop panel in Mac Safari at 1280 and 1440"
    expected: "Pin tap, Open report, switch pins, Back, Nearby row tap all behave; arrows, plus, minus and Escape in the comment box do not move the map"
    why_human: "Visual layout and keyboard feel"
  - test: "The pinned comment box (laptop sticky box, phone box under the soft keyboard)"
    expected: "Box stays in reach and is not hidden by the keyboard"
    why_human: "Needs a real keyboard and viewport"
  - test: "Share window, the public page /r/{token} with scripts blocked, and the WhatsApp link preview"
    expected: "No sign in needed, no count and no verified claim in the preview; Sign in to see more goes to /login; preview shows within about 10 seconds"
    why_human: "The preview needs a public https copy; looks need eyes"
  - test: "Moderator Comments screen: Keep and Remove"
    expected: "Held and flagged comments listed; Keep shows it, Remove hides it and the public reads say removed"
    why_human: "Visual and moderator flow"
  - test: "A real phone comment note (after Phase 3.3 is live, Changes on)"
    expected: "Another account replies; the bell shows New reply to your comment; the phone note is silent and one per report per device"
    why_human: "Needs a real device and push service"
  - test: "Dev database migrations 00012 to 00019 (the owner runs make migrate on the dev database after saying yes)"
    expected: "Applied cleanly in order"
    why_human: "Owner's database; the owner must say yes first. Not run by this verification."
  - test: "D-18 default A for the laptop host"
    expected: "Keep A (bubble stays, Open report row opens the panel with Back). Option B is plan 08-07 alone. The build recommends keeping A."
    why_human: "Owner decision, asked twice, not answered"
---

# Phase 8: Report page and comments, verification report

**Phase goal:** A reader can open any report into its own page with the trust block and a history timeline, share it as a live status link, and read and add moderated comments.
**Verified:** 2026-10-29 at main e14f24d6
**Status:** human_needed (no gaps; only the owner's own checks remain)
**Re-verification:** No, initial verification

## Evidence I ran myself (not taken from SUMMARY files)

| Check | Result |
|-------|--------|
| `go build ./...` and `go vet ./...` | clean |
| `TZ=UTC go test ./... -p 1 -count=1` on my own database p8_verify | every package ok, 0 FAIL |
| `node --test web/*.mjs` | 1452 tests, 1452 pass, 0 fail, 0 skipped |
| p8_verify | did not exist, was created, dropped at the end (count after: 0). pinalert_test and owner databases not touched. No server started. |
| Debt markers (TBD, FIXME, XXX) in the 85 non test files changed since 88cbe8ad | none |
| Git state | no change made by me (STATE.md was already modified before I began) |

Not run: the Linux CI check (Docker, owed at push time, as 08-20 said).

## Observable truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Tapping a report in the feed or the popup opens its own page with the same trust block and a way back to the map | VERIFIED | `web/static/js/map.js:1159` builds the page with `PinalertReportPage.create`; the same card appends `trust.el` and the page node (map.js:1186 onward); Open report row `buildOpenReportRow`; Back and "Back to the list" anchors (map.js:293, 1382, 2285); feed deep link `Pinalert.openReport` (feed.js:1162, app.js:554). report-page.js holds no trust rule (header comment, A-P8-7). |
| 2 | A timeline of the report's trust built from the vote log, with no names or locations of voters | VERIFIED | `TimelineVoteLog` in `internal/store/queries/votes.sql:68` replays the whole log; `internal/api/handlers/timeline.go:23-36` response has only kind, tone, text, at, yours. Route `GET /api/reports/{id}/timeline` in router.go. Account and cell stay inside the service. Tests green in full run. |
| 3 | A signed in user can read and add short comments; each passes moderation and a per account rate limit; comment text never reaches the page through a markup parsing sink | VERIFIED | `CommentService.Post` (`internal/service/comments.go:293`) cleans the words, needs a username, checks open state, runs `ScreenAbuse` (reject, borderline listed for moderators, unfinished held and retried by the worker). Routes with `CommentMiddleware`, `LikeMiddleware`, `FlagMiddleware` (router.go ~961-966). No innerHTML, outerHTML, insertAdjacentHTML or document.write in comments.js, report-page.js, share.js or map.js; `TestNoMarkupSinksAnywhere` (web/js_sinks_contract_test.go:195) passes. Commenting follows the report: `TestCommenterFollowsReportInFeed`, `...InUpdates`, `...ForPush` pass. Moderator Keep and Remove routes exist (router.go:1117-1119). |
| 4 | The popup footer shows a real comment count only, never a count the backend cannot supply | VERIFIED | `comment_count` is computed in `internal/store/queries/reports.sql:170` (clear, not deleted); `openReportWords` (map.js:318) shows words only for a whole number from the server, else nothing; `TestPopupFooterOpenReportRow` passes. |
| 5 | Anyone can share a report as a link to its live status, never a baked in count or verified claim | VERIFIED | `POST /api/reports/{id}/share` (router.go) makes a 22 character token from 16 `crypto/rand` bytes (`internal/service/share.go:30-84`); `GET /r/{token}` public page (router.go:782) sets no cookie (`TestSharePageSetsNoCookie`), access log redacts the token (router.go:241-251). `web/templates/share_status.html.tmpl`: link preview title is the heading, description a fixed line, no count and no claim; "Sign in to see more" links to /login (line 66). |

**Score:** 5/5 truths verified, 0 present but behavior unverified. The behavior sensitive parts (poll merge, tail reread, cancellation of a list read on post) are covered by named node tests that pass (1452 of 1452, including the three SK2-01 tests).

## Requirements coverage

| Requirement | Plans | Status | Evidence |
|-------------|-------|--------|----------|
| PAGE-01 (report page, shared trust block, history timeline) | 08-01, 05, 07 | SATISFIED | Truths 1 and 2 |
| PAGE-02 (screened, rate limited comments; commenting follows the report) | 08-08 to 08-18 | SATISFIED | Truth 3 |
| COORD-08 (shareable card, live status link only) | 08-02, 03, 04, 06 | SATISFIED | Truth 5 |

No orphaned requirement. REQUIREMENTS.md still shows these three as Pending and unticked; that is the orchestrator's edit, not mine.

## Review and fix trail

08-REVIEW found 34 (0 critical, 15 warning, 19 info). All warnings were fixed across the server, browser and templates fixers, the skeptic (SK-01, SK-02, SK-04, SK-07 fixed) and rounds 2 and 3 (SK2-01, SK2-03, SK2-04 fixed). I confirmed the late fixes exist in code (`rereadTails` in comments.js, `reply_after` in handlers/comments.go, the favicon route in router.go) and that their tests are in the green runs.

### Accepted and recorded items (not gaps)

- IN-T01 accepted: the Policy date and CurrentTermsVersion stay 2026-10-29; on deploy day the owner moves both together (README step 9).
- Server fixer, recorded: IN-S03 (comment ids probed through 403 and 404 answers; sequential ids, no content leaks), IN-S04 (timeline keeps the oldest 2000 vote rows), IN-S06 (account deletion can race a comment in flight, millisecond window), IN-S07 (non development start needs an https BASE_URL, documented), IN-S08 (moderator comment list scans every comment, fine at this scale).
- Browser fixer, left: IN-B01 (a 409 closed answer clears the typed words, UI-SPEC allows it), IN-B03 (live status lines may not repeat; needs a VoiceOver check), IN-B04 (focus after a poll removes a flagged or deleted comment), IN-B05 (moderator Comments list rebuilt under an action in flight; the missing rejection handler after `.then(loadComments)` predates Phase 8).
- Round 2, recorded: SK-03 (send time comment check can skip a push in a narrow race; errs toward silence), SK-05 (middle pages not refreshed after Show more twice), SK-06 (TestPhoneSheetHasNoMutedText matches overrides by last selector only).
- Round 3, recorded: SK2-02 (cannot happen: a post restarts the list read).
- Doc notes for the next planning pass: 08-VALIDATION V22 names the wrong package for `TestCommentCountExtras` (it lives in `internal/store`); V9 says "from the second tap" but the Share window never calls `navigator.share` (the wa.me link is used, as UI-SPEC set).
- Rate gate note: ratelimit keys and the Linux check are as built; nothing here blocks the phase.

## Anti-patterns

None found: no debt markers, no markup sinks, no stub handlers, no hollow data (the count and timeline come from real queries).

## Gaps

None.

## Owner check list (in short words)

Phone checks use the iPhone simulator, never the iPad. Laptop checks use Mac Safari. Show each one at a time, with a recommendation.

1. Phone report page: open a report, look at the history, Share, comments, thumbs up, replies, Delete, Flag, closed, removed, deleted and held comments. Check 320, 360 and 390 wide, light and dark, and turning the phone. Nothing should jump while it refreshes. The long Settings switch label should wrap.
2. Laptop in Mac Safari at 1280 and 1440: tap a pin, press Open report, switch pins, press Back, tap a Nearby row. Type in the comment box and use the arrow keys, plus, minus and Escape; the map must not move.
3. The comment box that stays in place: on the laptop, and on the phone with the keyboard up.
4. The Share window, then the public page for a shared link with scripts blocked. It must need no sign in and show no count and no verified claim. The WhatsApp preview needs a public https copy; give it up to 10 seconds.
5. The moderator Comments screen: try Keep and Remove. Remove has no confirm step (08-18); say if you want one.
6. A real phone comment note, once Phase 3.3 is live and Changes is on: another account replies, the bell says "New reply to your comment", and the note is silent, one per report per device.
7. The dev database: say yes, then run `make migrate` for 00012 to 00019 on the dev database only.
8. D-18: keep default A (the build recommends it, since you said the laptop looks good as it is) or choose B (a pin tap opens the report at once).
9. Other defaults to confirm in the same sitting: phone order as shipped; comment count only in the Comments heading and the Open report row; public page says "removed" only for a moderator removal; Sign in to see more is a plain link; anyone signed in may comment; no report page in the Low data view; "Comment removed" versus "Comment deleted"; a Nearby row tap on the laptop opens the bubble.
10. Small looks the builders asked about: after a tap on empty map focus stays on the map; a pin near the left edge may leave view when the panel opens; the comment trust line on a disputed report; Escape in the comment box closes an open trust panel first.
11. Before the push: run the Linux check in Docker. On deploy day: move the Policy date and CurrentTermsVersion together.

---

_Verified: 2026-10-29_
_Verifier: Claude (gsd-verifier)_
