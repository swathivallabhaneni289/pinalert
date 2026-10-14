---
phase: quick-261013-wm5
plan: 01
subsystem: activity-page
tags: [activity, tabs, votes-by-account, phone, laptop, contract-tests]
requires: []
provides:
  - "migration 00006 (idx_votes_account_kind_report_created)"
  - "VotedReportsByAccount query and AuthService.ActivityPageForAccount"
  - "six tab Activity page with real counts, laptop and phone"
affects: [internal/store, internal/service, internal/api/handlers, web]
key-files:
  created:
    - internal/store/migrations/00006_add_votes_account_kind_index.sql
    - internal/api/handlers/activity.go
    - internal/api/handlers/activity_page_test.go
    - internal/service/activity_page_test.go
    - web/static/css/activity.css
    - web/static/js/activity-tabs.js
    - web/static/icons/message-square.svg
    - web/activity_page_contract_test.go
  modified:
    - internal/store/queries/votes.sql
    - internal/store/sqlc/votes.sql.go
    - internal/service/auth.go
    - internal/api/handlers/auth.go
    - web/templates/profile.html.tmpl
    - web/static/css/phone.css
    - web/static/js/activity.js
decisions:
  - "Resolved and Not resolved include reports the account voted on (owner question 1, plan recommendation)"
  - "Mark resolved on someone else's report does not put it under My votes (owner question 2)"
  - "No side menu and no top bar (backlog 999.10) (owner question 3)"
requirements: [IDENT-03, TRUST-07]
status: complete
---

# Phase quick-261013-wm5 Plan 01: Activity page with six tabs Summary

The Activity page (/profile) now has six server rendered tabs (All, My posts, My votes, My disputes, Resolved, Not
resolved) with real counts of distinct reports, built on a new account scoped votes read with its own index, drawn as
sketch 004 variant B at 80 percent on a laptop and as picture 05 on a phone, with the tab kept in the address.

## Commits

| Task | Commit | What |
|---|---|---|
| 1 | 780f531 | Migration 00006, `VotedReportsByAccount`, `ActivityPageForAccount` (one pass, one history read, one vote read), `buildActivityPage` view model, tests at store, service, handler and e2e level |
| 2 | 8843c4c | Template with tab nav, cards, chip and one empty state per tab; `activity-tabs.js`; `activity.css`; message-square icon; contract tests |
| 3 | a04e063 | Phone strip and cards in `phone.css`, `TestPhoneActivityStylesheet` extended, both sketch READMEs |

## The three contract departures

1. **Severity word moves into the chip** (A26, A58). The card body starts with the tag and then the title, and the chip's
   severity span holds the word once. Changed `TestProfileTemplateUsesServerSeverityAndTrust` and
   `TestProfileRendersSeverityLabel`. Trust JSON, expired and hook checks kept; the `sev-` class check kept.
2. **The record card shows at every width** (W-10). `.profile-record` left phone.css's grouped base hide rule, so
   `phoneBaseHideHeads` and the wording in `TestPhoneRulesLiveInTheNarrowBlock` went from five classes to four.
3. **Phone grid gains tabs and empty rows; the state word loses its auto margin.** `TestPhoneActivityStylesheet` grid areas
   gain a tabs row and an empty row, and the state word has margin 0 because the chip holds the top right corner. Card
   hairline and Log out assertions kept; new assertions for the strip, 44px tabs, white selected tab, 48px icon, hidden
   action phrase and hidden help line.

## Tests that changed and why

| Test | Change |
|---|---|
| `TestMigrationsEmbedded` | A 00006 block like the 00005 one |
| `TestActivityForAccountReadsReporterHistoryOnce` | Call log gains `VotedReportsByAccount` second (page reads votes in the same pass) |
| `TestProfileTemplateUsesServerSeverityAndTrust`, `TestProfileRendersSeverityLabel` | Departure 1 |
| `TestProfileWithoutReportsRendersNoRecord` | The visible empty state is the All tab's, "No reports yet" stays as the hidden My posts one |
| `nonCategoryIcons` in `web/css_contract_test.go` | Registers `message-square.svg` |
| `phoneBaseHideHeads`, `TestPhoneRulesLiveInTheNarrowBlock`, `TestPhoneActivityStylesheet` | Departures 2 and 3 |

New tests: `TestVotedReportsByAccountKeepsTheLatestContentVote`, `TestVotedReportsByAccountUsesTheAccountIndex`, ten in
`internal/service/activity_page_test.go`, nine in `internal/api/handlers/activity_page_test.go`,
`TestProfileListsReportsTheAccountVotedOn`, `TestProfileVotedRowNeverOffersReopen`,
`TestProfileVoterWithoutPostsHasNoRecord`, `TestProfileTabsRenderFromTheAddress`, and four in
`web/activity_page_contract_test.go` (markup, script, stylesheet, contrast).

## Verification numbers

- Full Go suite on fresh disposable databases: p_act_t1 1393 pass, p_act_t2 1407 pass, p_act_t3 1407 pass; each EXIT 0,
  FAIL 0, SKIP 0; each database dropped. No run touched pinalert or pinalert_test. The known flaky test did not fail.
- Diff gate against BASE 6e9a898: no change to index.html.tmpl, main.css, feed.css, trust.css, modal.css, auth.css, app.js,
  feed.js, map.js, votes.js, trust.js, visibility.js. phone.css hunks are the one base hide rule and the Activity section
  only.
- Browser probes (scratch server, Chromium and WebKit, 216 checks, 0 failed): at 1280 by 800 six tabs on one row (equal
  tops), every tab 44px tall, title 32px, icon 51.19px, chip right of the body (987.3 vs body right 974.5), record card
  present, no arrow, no top bar or side menu. At 390 by 844 and 320 by 568 `documentElement.scrollWidth` equals the
  window width, strip scrollWidth 810 over clientWidth 358 at 390, selected tab `rgb(255, 255, 255)` on the
  `rgb(241, 243, 244)` track, tabs 44px, icon 48px, cards visible. Each tab click leaves exactly its count of visible
  cards (All 5, My posts 2, My votes 3, My disputes 1, Resolved 1, Not resolved 4 in the seeded data), writes
  `/profile?tab=<key>` (`/profile` for All) and does not change scrollY. Back from `/` lands on My votes with 3 cards.
  A trust panel opened on a card hidden at load measures 632 by 215 (laptop), 268 by 274 (390), 198 by 357 (320).
  Reopen changed the chip from Resolved to Medium and showed the toast "Report reopened.". Dark theme card backgrounds
  are `rgb(22, 24, 28)` at 1280 and `rgb(11, 13, 16)` at 390, the dark tokens.
- Feed baseline, BASE build against this build on the same database, Chromium and WebKit at 1280 by 800 and 900 by 800:
  zero computed differences over 12 selectors plus `documentElement.scrollWidth`.
- Contrast (light / dark): title 19.22 / 17.68, help 5.74 / 7.66, card title 18.49 / 16.15, sub line 4.67 / 7.00,
  tab 5.27 / 7.66, tab hover 4.78 / 7.00, selected tab 19.35 / 16.15, count 7.54 / 12.57, tag 5.93 / 12.57,
  critical chip 4.54 / 5.02 (picture's 4.21 darkened to #BE433A), medium chip 4.61 / 6.51 (picture's 3.00 now the caution
  tone), low chip 4.57 / 7.00, resolved chip 5.15 / 12.57, not resolved chip 4.61 / 5.02; event icon glyph 4.07, 3.67,
  4.00.

## Deviations from Plan

### Auto-fixed and noted

**1. Port departure, breaks an explicit constraint.** The plan names ports 8796 and 8797. A Python static server (pid
44791, not mine) already holds 127.0.0.1:8797, so my BASE build could not be reached there. I ran it on 8798 instead,
against the same p_act_probe database, and left the Python process alone. 8798 is not a port the plan names. Both of my
servers are stopped and p_act_probe is dropped.

**1b. A display rule on the laptop card head has no `:not([hidden])` guard.** Plan step 3b asked for the guard on every
Activity rule that sets `display` on a card. The head `main.profile-page .profile-report-list > .activity-card` in the
900px block does not carry it. It renders correctly, because the `[hidden]` override
(`main.profile-page .profile-report-list > .report-row[hidden]`, specificity 0,4,1) beats it (0,3,1), and the click
probes confirm the visible counts. Recorded as a deviation rather than a fourth commit.

**1c. The zero count tab in the browser.** The server side of "a tab with zero cards shows its own empty state" is proven
(the `ShowEmpty` unit cases and the empty account e2e test). The script path that switches to a zero count tab
(`data-count === '0'`) is pinned only statically: every seeded tab had cards, so no browser exercised it.

**2. Extra paths in commits.** Task 2 also touched `internal/api/handlers/activity_page_test.go` (its MetaText assertion
went with the field the plan removed). Task 3 also touched `web/static/js/activity.js` (one em dash left in a comment I
re-wrapped, replaced with a comma). Both are comment or test only.

**3. The page layout of a phone card is a wrapping row, not a grid.** `.report-row` is `display: flex; flex-wrap: wrap`
with the body and state line as `display: contents`, so the icon, tag, state word and chip share the first line and
long words wrap instead of overflowing at 320px. Plan text said grid; the visible result is the same and 320px never
scrolls sideways.

## Known Stubs

None.

## Not built (recorded in the sketch 004 README)

- System update rows (Phase 3.2's updates table does not exist)
- Comment added (Phase 8)
- The arrow and the open report link (nothing handles `/?report=ID` yet; the card opens nothing)
- The side menu, top bar, address pill, bell and avatar (backlog 999.10)
- The distance on the sub line (the server stores no reader location)
- Arrow key roving between tabs

## Owner questions

1. Should the Resolved and Not resolved tabs include reports you voted on, or only your own posts? Recommendation:
   include both, as the six tab picture shows; narrowing it later is a one line change. (Built that way.)
2. When you press Mark resolved on someone else's report, should it show under My votes? Recommendation: no, the picture
   says My votes means reports you confirmed or disputed. (Built that way.)
3. The laptop picture has a side menu and a top bar. Recommendation: build them later with the app wide design pass
   (backlog 999.10), because Reports, Alerts, Account, the bell and the address pill would lead nowhere today. (Not
   built.)

## Owner action

Apply migration 00006 to your own databases (`make migrate` with your own `DATABASE_URL`). It adds one index. The page
works before you do; the votes read is a sequential scan until it exists. The external browser harness's T11 Activity
checks change by design and were not run here.

## Threat model

T-wm5-01 to T-wm5-07 mitigations are in place: the account id comes only from the gate; the query selects no
session_id or geohash_cell and the e2e test proves another account's votes never reach the page; `data-trust` is built
only by `trustToResponse` and the identifier walk runs on voted cards; the tab value is an exact allowlist; the script has
no markup sink, no fetch and no Pinalert globals (pinned by tests); `CanReopen` is forced false on voted cards; `/profile`
keeps `Cache-Control: no-store`. No new threat surface beyond the plan's register.

## Self-Check: PASSED

- Files exist: migration 00006, activity.go, activity.css, activity-tabs.js, message-square.svg, activity_page_contract_test.go.
- Commits exist: 780f531, 8843c4c, a04e063.
- Scratch databases dropped, servers stopped, probe artifacts deleted.
