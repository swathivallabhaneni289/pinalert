---
phase: 04-robustness-real-world-resilience
verified: 2026-10-16T00:00:00Z
status: gaps_found
score: 4/7 roadmap success criteria verified
behavior_unverified: 2
overrides_applied: 0
re_verification: false
gaps:
  - truth: "SC7: A public Disclaimer page and a public Policy page ... and both are linked from the bar on every page."
    status: failed
    reason: "The bar links only the Disclaimer page. The Policy page is not linked from the bar on any page. It is reachable one step away (bar, then the Disclaimer page's Policy link), from the account menu on signed in pages and from the login page panel, but not from the bar as the criterion says. Plan 04-06 deliberately made Disclaimer 'the only link in the bar' to keep the owner approved D-22 bar sentence verbatim, so the roadmap criterion and the owner approved wording disagree."
    artifacts:
      - path: "web/templates/disclaimer_bar.html.tmpl"
        issue: "One link only: <a href=\"/disclaimer\">Disclaimer</a>. No /policy link."
    missing:
      - "Either add a Policy link to the bar (this changes the owner approved D-22 sentence, so the owner must agree, and TestDisclaimerBar* pins in web/chrome_contract_test.go and the bar sentence test must move with it), or accept the deviation with an override entry (suggested text in the Gaps Summary)."
      - "With the shipped default (CONTACT_EMAIL unset) neither legal page states how to complain or ask for removal; set CONTACT_EMAIL before any public deploy (D-24, README launch checklist item 3), or name this in the same override."
deferred: []
behavior_unverified_items:
  - truth: "SC1: On a degraded connection a user can switch to the Low data view (no map, no photos) and post a report and confirm or dispute from it."
    test: "Open /low on Chromium and WebKit at 320, 360 and 390 px, light and dark, with network throttled to a slow 3G profile and geolocation granted (mocked). Check the list shows from the stored position, post a report through the map free location block, then Confirm and Dispute a report from a row. Also open the main page with unpkg blocked and wait 6 seconds for 'Taking long? Open the low data view.'"
    expected: "No request to unpkg, tile hosts or any image; the report posts (201) and appears; Confirm and Dispute work and a row never offers Mark resolved; the slow notice appears and links to /low."
    why_human: "Server side is tested (route, gate, cookie redirects, asset policy, byte budget). The posting and voting flow lives in low.js and report-form.js, which no test executes; the Go tests only read their source text."
  - truth: "SC2: A report submitted while offline is queued on the device (IndexedDB) and automatically sent once connectivity returns."
    test: "With the browser context set offline, post from the main form and from the Low data form; then set it online and fire the online event; also reload the page with a saved row and wait for the 30 second timer."
    expected: "Form closes with 'No signal. Your report is saved on this device and will send when you're back online.'; a Waiting to send strip shows the row; on reconnect the row sends once, shows 'Report sent.' and disappears; the database holds exactly one row for that request id; a second account in the same browser never sees or sends it."
    why_human: "The server retry safety is tested (request id idempotency, concurrent and per account, under -race), and node web/queue_logic_test.mjs runs the real queue.js for ordering, no age discard and account isolation, but its window stub ignores addEventListener, so the online event trigger, the IndexedDB path in a real browser and the form to queue hand off are never executed."
---

# Phase 4: Robustness, Real-World Resilience Verification Report

**Phase Goal:** As a person reading or reporting during an emergency, often on a weak phone connection, I want to keep reading, posting and confirming reports when the signal is bad, knowing abusive posts are screened out and that the site says plainly it is unofficial, so that the reporting loop keeps working and I am not misled about what Pinalert is.
**Mode:** mvp (user story validated by `user-story.validate`: valid true, no errors)
**Verified:** 2026-10-16
**Status:** gaps_found (one criterion fails as worded; see Gaps Summary. Everything else is either verified or waiting on a human or browser check.)
**Re-verification:** No, initial verification

## User Flow Coverage (MVP mode)

| Step of the story | Expected | Evidence in the codebase | Status |
| --- | --- | --- | --- |
| Keep reading on a bad signal | A text only page with no map, and a main page that keeps its rows when a poll fails | `/low` route behind the gate (`router.go`), `low.html.tmpl` loads no map library, unpkg or image; budget test `web/low_budget_contract_test.go`; stale line in `feed.js` and `app.js` (10 s abort) | Server side verified; client flow PRESENT_BEHAVIOR_UNVERIFIED |
| Post when the signal is bad | Map free report form on `/low`; an unanswered post is saved and sent later | `report-form.js` shared form, mounted by `low.js` and `modal.js`; `err.queueable` (no answer, timeout, 502, 503, 504) goes to `PinalertQueue.save`; `queue.js` flushes on `online`, page load, a 30 s timer and Send now | PRESENT_BEHAVIOR_UNVERIFIED |
| Confirm or dispute | Confirm and Dispute on every Low data row | `low.js:299` `createVoteBlock(report.id, { resolve: false })` | PRESENT_BEHAVIOR_UNVERIFIED (client) |
| Abusive posts screened out | Abusive text refused with the form kept open; borderline listed; unfinished check holds non serious, publishes serious | `SubmitChecked` (`internal/service/report.go:596`), `CheckText` with a 3 s bound, `Decide` thresholds 0.85 and 0.40, `ModerationWorker` started in `cmd/server/main.go`; Go e2e tests pass | VERIFIED (calibration still needs the owner's key) |
| Not over throttled behind a shared IP | Per account limits with the wait named; IP only a loose backstop when a header is configured | `ratelimit/perkey.go`, `handlers/ratelimit_gate.go`, router wires all five vote routes, flag and submit | VERIFIED |
| Told plainly it is unofficial | Bar on every page; public Disclaimer and Policy pages | Bar partial in all 8 full page templates; `/disclaimer` and `/policy` exempt and public | Bar VERIFIED; Policy is not linked from the bar (FAILED as worded) |

## Goal Achievement

### Observable Truths (roadmap success criteria are the contract)

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | Low data view: text only, no map or photos, can post, confirm and dispute | PRESENT_BEHAVIOR_UNVERIFIED | `GET /low` in the gated group (`internal/api/router.go`), `handlers/low.go`; `low.html.tmpl` scripts are app, votes, visibility, trust, queue, flag, report-form, low, account-menu, with no Leaflet, MapLibre or unpkg; `#low-report` button and the shared form partial; `low.js` builds rows with the trust block, `createVoteBlock(..., { resolve: false })` and the Flag control last. Ways in: login panel, account menu, 6 s slow notice (`slow-notice.js` first in head). Tests `low_routes_test.go`, `ways_in_e2e_test.go`, `web/low_budget_contract_test.go` pass. No test executes low.js or report-form.js. |
| 2 | Offline report queued in IndexedDB and sent automatically when connectivity returns | PRESENT_BEHAVIOR_UNVERIFIED | `queue.js` opens IndexedDB with an in memory fallback, binds rows to the account id, flushes on `online`, load, 30 s and Send now, never discards by age, stops on 401, calls `navigator.storage.persist()`. `report-form.js:866` saves queueable failures; `app.js:180` `QUEUEABLE_STATUSES = [502, 503, 504]` plus network error and timeout. Server: `InsertReportIdempotent` with request id unique per account; `TestSubmitIdempotent*`, `TestInsertReportIdempotentConcurrent` pass under -race. `node web/queue_logic_test.mjs` 4 of 4 pass (ordering, no discard, account isolation, resend link). The online trigger and real IndexedDB are not exercised. |
| 3 | Text screened before publishing: abusive rejected with form kept open; borderline posts and is listed; unfinished check keeps critical or rescue visible while retrying, holds anything else off every feed | VERIFIED | `SubmitChecked` runs `CheckText` before insert; gibberish first, one provider call under `InlineScreenTimeout = 3s`; reject returns `TextRejectedError` and stores nothing; unfinished stores `moderation_state = pending` in the same insert plus a pending check row; `Resolve` top rungs use `ReportMeta.Withheld()` (removed, or pending and not serious); `ModerationWorker.Run` started with `go moderationWorker.Run(...)` in `cmd/server/main.go`, sweep at boot then every 30 s, late reject writes `removed`. Client keeps the form open on 422 (`report-form.js:871`). Tests `submit_screening_e2e_test.go`, `withheld_e2e_test.go`, `feed_visibility_e2e_test.go`, `modworker_test.go`, `visibility_test.go`, `moderation_view_test.go` pass. |
| 4 | Rapid submissions, votes, vote changes and flags throttled per verified account with a wait message, before a shared IP is penalised | VERIFIED | Constants block in `cmd/server/main.go` (serious 5 then 1 per 5 min, other 8 then 1 per 15 min, votes 20 then 1 per 15 s, change 1 per 30 s, flag 10 then 1 per 2 min, IP backstop 60 then 1 per 20 s). Router: `rateGate.VoteMiddleware` on confirm, dispute, resolve, reopen and unresolve; `FlagMiddleware` on flag; submit gate inside `SubmitReport`; `GET /api/reports` unlimited. Keys `acct:<id>`; backstop only when `CLIENT_IP_HEADER` is set and parses, never a shared bucket. 429 carries `Retry-After` and a message naming the wait. `TestReportRateLimit*`, `TestVoteRateLimit*`, `TestFlagRateLimit`, `TestIPBackstop` pass; `TestReportRateLimit*`, `TestVoteRateLimit*` and the `perkey` tests also pass under -race in this run. |
| 5 | Every page shows a persistent disclaimer: unofficial, not government, not a substitute for emergency services | VERIFIED (with a warning) | `disclaimer_bar.html.tmpl` included in all 8 full page templates (index, login_gate, verify_outcome, profile, low, disclaimer, policy, moderate), first in body inside sticky `.top-chrome`, no close control; numbers from `EMERGENCY_*` settings (`handlers/chrome.go`). `TestDisclaimerBarOnEveryFullPage` derives the page set from the embedded templates. Warning: the public API reference at `/swagger/*` (third party UI) carries no bar; see Gaps Summary. Visual check at phone widths goes to the browser pass. |
| 6 | Moderator screen (owner or server list), plain 404 for everyone else, lists automatic flags, user flags and unfinished checks, Keep or Remove any post; any signed in reader can flag with a reason from a quiet control; a flag never hides a post | VERIFIED (server); client screens to the browser pass | Moderator group outside the gate with `r.Handle` and `requireModerator` (`router.go:504-512`); `MODERATOR_EMAILS` parsed by `service.ParseModeratorEmails` with fail fast. `TestModeratorRoutesIndistinguishableFrom404`, `TestModeratorQueues`, `TestModeratorKeep`, `TestModeratorRemove`, `TestModeratorRemoveById`, `TestModeratorAuditRow`, `flag_e2e_test.go` (one per account, own report 403, withheld 404, flags never change the feed) pass. Flag control mounted in `map.js:574` (popup) and `low.js:305`. Why this is VERIFIED while SC1 is not: every invariant SC6 states (byte equal 404 for non moderators, the three lists and their counts, Keep and Remove, one flag per account, a flag never hides a post) is enforced by the server and exercised by e2e tests; only the presentation in `moderate.js` and `flag.js` is left to the browser pass. SC1, by contrast, depends on a map free posting path that exists only in client code. |
| 7 | Public Disclaimer and Policy pages, readable without login, state what Pinalert is not, the numbers, what is screened and kept, how to complain or ask for removal, and both linked from the bar on every page | FAILED | Pages exist, are public (`r.Get("/disclaimer")`, `r.Get("/policy")` in the exempt set), render no account data, `Cache-Control: private, max-age=300`; `legal_e2e_test.go` passes. But the bar links only Disclaimer (`disclaimer_bar.html.tmpl`). Also, with `CONTACT_EMAIL` unset (the shipped default, owner decision D-24) neither page says how to complain or ask for removal (the Policy retention sentence about removal and the Complaints heading are both inside the same `CONTACT_EMAIL` guard, so nothing is shown). |

**Score:** 4/7 criteria verified (2 present but behavior unverified, 1 failed)

### Plan must_haves (spot checked; 29 plans, about 300 truths)

The plan level truths were checked by sampling the wiring sensitive ones rather than one row each. All sampled truths hold:

| Plan | Truth sampled | Result |
| --- | --- | --- |
| 04-01 | `Withheld()` drives the two top `Resolve` rungs; serious pending resolves as clear | Holds (`visibility.go:149`, `:274`) |
| 04-05 | `NearbyReports` untouched; moderation reaches reads via batched `ReportExtrasForReports`; migration 00007 | Holds; migration present; suite green |
| 04-07 | Every `Resolve` caller passes moderation | Holds (`TestResolveCallersMentionModeration` passes) |
| 04-07 default | Held or removed report drops off another voter's My votes | Holds (`auth.go:627` skips `meta.Withheld()`) |
| 04-09, 04-15 default | Take back shares the resolution cooldown | Holds (`unresolve` uses `VoteKindResolution`) |
| 04-11 | Screener built once by `LoadScreener` and injected through `WithScreener` | Holds (`main.go:157`, `:240`) |
| 04-14 | Worker started | Holds (`go moderationWorker.Run(context.Background())`) |
| 04-17 | Written time display only | Holds (`TestWrittenAtNeverFeedsExpiry` passes) |
| 04-23 default | A report faded or resolved more than 7 days ago still reads Expired or Resolved on the moderator screen | Holds (`ModerationViewOf` in `moderation.go:229` maps through `ViewOf`, which has no 7 day cut: past expiry gives Past and the word Expired, retracted gives Resolved) |
| 04-24 default | Flag is the last control of a Low data row | Holds (`low.js:304-306`) |
| 04-27 | docs/MODERATION.md honesty sentence, README launch checklist items, TRUST-MODEL R-02 note | Present (MODERATION.md lines 68 to 69, README checklist items 3 to 8, TRUST-MODEL.md line 271 added by commit 8ccf537) |
| 04-28 | Fixture of 200 genuine and 40 abusive entries, scores empty until recorded | Holds; `TestScreeningFixture` skips naming 240 entries |
| 04-29 | Withheld reports neutral for the rating | Holds (`reliability.go:72`) |

### Required Artifacts (key ones)

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/service/screening.go`, `textcheck.go` | Generic text check and policy | VERIFIED | Wired into `SubmitChecked` and the worker |
| `internal/moderation/client.go`, `loader.go` | OpenAI client, dev pass fallback, fail fast | VERIFIED | `LoadScreener` called in main |
| `internal/service/modworker.go` | Background retry | VERIFIED | Started in main |
| `internal/ratelimit/perkey.go`, `handlers/ratelimit_gate.go` | Per account limiter and 429 body | VERIFIED | Wired in router |
| `internal/store/migrations/00007_add_moderation_and_idempotency.sql` | State, checks, flags, actions, request id | VERIFIED | Applied by testutil on p4_ver |
| `web/templates/disclaimer_bar.html.tmpl` | Bar | VERIFIED (bar), see SC7 | Only Disclaimer linked |
| `web/templates/disclaimer.html.tmpl`, `policy.html.tmpl`, `handlers/legal.go` | Public legal pages | VERIFIED | Exempt routes |
| `web/templates/low.html.tmpl`, `web/static/js/low.js`, `low.css` | Low data view | VERIFIED (exists, wired) | Behavior unexecuted |
| `web/static/js/report-form.js`, `report_form.html.tmpl` | Map free shared form | VERIFIED (exists, wired) | Behavior unexecuted |
| `web/static/js/queue.js` | Offline queue | VERIFIED (exists, wired on index, low, profile) | Partially executed by node test |
| `web/static/js/flag.js`, `flag_dialog.html.tmpl`, `handlers/flag.go` | Flag control and route | VERIFIED | Server e2e tested |
| `web/templates/moderate.html.tmpl`, `web/static/js/moderate.js`, `internal/api/moderator.go`, `handlers/moderation.go` | Moderator screen | VERIFIED | Server e2e tested |
| `docs/MODERATION.md` | Design record | VERIFIED | |

### Key Link Verification

| From | To | Via | Status |
| --- | --- | --- | --- |
| `POST /api/reports` | `SubmitChecked` | `handlers/reports.go:521` | WIRED |
| `SubmitChecked` | screener | `CheckText(ctx, s.screener, desc)` | WIRED |
| main | worker | `go moderationWorker.Run` | WIRED |
| `Nearby`, history, Activity, votes, flags, moderator | `Resolve` with moderation | `ReportMeta.Moderation` from batched extras | WIRED |
| five vote routes, flag route | per account limiter | `r.With(rateGate.VoteMiddleware / FlagMiddleware)` | WIRED |
| `report-form.js` | `queue.js` | `err.queueable` then `PinalertQueue.save` | WIRED |
| `queue.js` | server | `Pinalert.submitReport` with the saved `request_id` | WIRED |
| bar | `/disclaimer` | link | WIRED |
| bar | `/policy` | none | NOT_WIRED (SC7) |
| `/moderate`, `/api/moderation/*` | `requireModerator` | own group outside the gate | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
| --- | --- | --- | --- | --- |
| Low data list (`low.js`) | reports | shared store `fetchReports` to `GET /api/reports` (`Nearby` over Postgres) | Yes | FLOWING |
| Moderator lists (`moderate.js`) | queue rows and counts | `GET /api/moderation/queue` to `ReviewQueue`, `FlagQueue`, `PendingQueue`, `ModerationQueueCounts` | Yes | FLOWING |
| Activity held or removed label | moderation state | `ReportExtrasForReports` | Yes | FLOWING |
| Bar numbers | `Chrome.Numbers` | `EMERGENCY_*` env, defaults 112, 100/101/108, 1070/1077 | Yes | FLOWING |
| Fixture gate | provider scores | `genuine.json`, `abusive.json` | No, empty until the owner records | STATIC by design (owner item) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Full suite on fresh `p4_ver` | `DATABASE_URL=.../p4_ver go test ./... -p 1 -count=1 -v` | EXIT 0; 860 top level `--- PASS`, 2817 PASS lines with subtests, 0 FAIL, 0 panic, 1 SKIP (`TestScreeningFixture`: "none of the 240 fixture entries has recorded scores") | PASS |
| Limiter under race | `go test -race ./internal/ratelimit/` | ok | PASS |
| Idempotency and limits under race | `go test -race -run 'TestSubmitIdempotent|TestVoteRateLimit|TestReportRateLimit' ./internal/api/handlers/` and `-run TestInsertReportIdempotent ./internal/store/` | 15 and 2 tests PASS | PASS |
| Queue logic in Node | `node web/queue_logic_test.mjs` | 4 pass, 0 fail | PASS |
| User story format | `gsd-tools query user-story.validate` | valid true | PASS |

`p4_ver` was created fresh, used only for these runs and dropped (`psql -lqt | grep -c -w p4_ver` prints 0).

### Probe Execution

Step 7c: SKIPPED (no `scripts/*/tests/probe-*.sh` in the repository and no probe declared by any Phase 4 plan).

### Requirements Coverage

| Requirement | Source plans | Status | Evidence |
| --- | --- | --- | --- |
| ROBUST-01 | 04-10, 04-13, 04-18, 04-19, 04-21, 04-27 | NEEDS HUMAN (browser) | SC1 present and wired; flow unexecuted |
| ROBUST-02 | 04-05, 04-13, 04-17, 04-22, 04-25, 04-27 | NEEDS HUMAN (browser) | SC2 present and wired; online trigger unexecuted |
| ROBUST-03 | 04-01, 04-02, 04-04, 04-05, 04-07, 04-11, 04-14, 04-16, 04-28, 04-29, 04-27 | SATISFIED for the mechanism; NEEDS HUMAN for "calibrated to a documented false-positive target" | The target and the honesty statement are documented (docs/MODERATION.md), but no provider score has been recorded, so `TestScreeningFixture` has never run. Calibration needs the owner's key. |
| ROBUST-04 | 04-03, 04-09, 04-15, 04-20, 04-27 | SATISFIED | SC4 |
| ROBUST-08 | 04-06, 04-08, 04-12, 04-27 | SATISFIED (warning on /swagger) | SC5 |
| ROBUST-09 | 04-05, 04-16, 04-20, 04-23, 04-24, 04-26, 04-27 | SATISFIED (server); client screens to the browser pass | SC6 |
| ROBUST-10 | 04-12, 04-21, 04-27 | PARTIAL | Pages public and correct; the complaint and removal path is absent until `CONTACT_EMAIL` is set (owner decision D-24, launch checklist item 3) |

All seven IDs appear in plan frontmatter (04-27 lists all seven). No orphaned requirement: REQUIREMENTS.md maps exactly these seven to Phase 4. The unchecked boxes in REQUIREMENTS.md are expected (requirements are not marked per plan in this repo) and are not a gap.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| (Phase 4 files, 168 paths from 241 commits) | | TBD, FIXME, XXX | none found | Debt marker gate passes |
| (Phase 4 files) | | TODO, HACK, PLACEHOLDER, "not yet implemented" in non test files | none found | |
| Lines added by Phase 4 commits | | U+2014 or U+2013 | none remaining (two test inputs in c467792 were later rewritten as Go unicode escapes) | |
| `internal/service/report.go` | 674 to 682 | A pending report whose check row write fails twice is stored `pending` with no check row | WARNING | Not claimed by the worker, not in any moderator list (lists join `moderation_checks`), Keep answers 409, only Find by id plus Remove works; a non serious one stays held until expiry. Logged; recorded by 04-11, 04-23 and 04-27 as an owner decision. |
| `web/templates/policy.html.tmpl` | 43 | "The scores of the automatic check for borderline reports" | WARNING | The worker also stores scores for checks it finishes with a pass (04-27 Deviations). The legal read should correct the wording. |
| `web/templates/policy.html.tmpl`, `disclaimer.html.tmpl` | 47 to 49, 67 to 70; 62 to 65 | Every complaint and removal sentence sits inside `{{if .Chrome.ContactEmail}}` | INFO | Consistent; with the shipped default nothing about complaints is shown (see Warnings) |
| `web/low_budget_contract_test.go` | | Budget is measured size plus 15 percent: 509094 raw bytes measured, 585728 budget, about 153 KB gzip | WARNING | About twice the research's 256 KB guess for a low bandwidth page. The test guards growth, not the target. Product decision. |

### Human Verification Required

Each item says whether the parallel scripted browser pass (Playwright, Chromium and WebKit, phone widths, light and dark, offline mode, throttling) can cover it, or whether it needs a person.

**Scriptable (browser pass)**

1. **Low data view flow (SC1).** Open /low at 320, 360 and 390 px on a throttled profile with mocked geolocation. Check the list renders from the stored position, then the fresh one. Post through the map free form, including the accuracy line, "Use this position" and the typed place fallback. Confirm and Dispute from a row. Check the network log shows no unpkg, tile or image requests. Block unpkg on the main page and expect "Taking long? Open the low data view." after 6 s. Covers the 04-18 and 04-19 blocks and the 04-13 slow notice. *Browser pass.*
2. **Offline queue (SC2).** Set the context offline, post from both forms, then go back online. Check the toast, the Waiting to send strip, "Report sent.", one database row per request id, the Activity page strip, Discard with its confirmation, and Edit and resend after a 422. Check a second account in the same browser never sees the row. Run on WebKit as well, to cover its IndexedDB. Covers the 04-22 and 04-25 blocks. *Browser pass.*
3. **Stale list (D-20).** After a load, go offline. Rows stay and "Last updated N minutes ago. Trying again." appears with Retry now; going back online removes it. *Browser pass.*
4. **Limit messages in the UI (SC4).** Drain the real defaults: 9 low reports, 21 votes, the same vote twice within 30 s, 11 flags. Check each message names the wait, the 112 tel link shows only on a serious post, and "Send after the wait" saves a Blocked row. *Browser pass.*
5. **Rejected text keeps the form (SC3, gibberish path).** Under the dev pass screener only the gibberish rule can reject. Post "aaaaaaaaaaaa" and expect the owner's sentence with the form still open and the text kept. The abusive path needs item 11. *Browser pass.*
6. **Held and removed on Activity.** On a scratch database, set a non serious report to `pending` and another to `removed`. Check the Activity labels "Waiting for a text check" and "Removed after review", and that neither appears in any feed tab or on the map for anyone. *Browser pass with a scripted SQL step on the scratch database.*
7. **Bar and geometry (SC5).** The bar is first and sticky on all 8 pages. At 320x568 and 1280x800 the account trigger sits 16 px under it, and the map and login pages do not scroll, in light and dark (04-06 and 04-08 blocks). *Browser pass.*
8. **Flag control and dialog (SC6).** Check the popup and Low data row controls, the five reasons, Flagged after a 201 or 409, focus return, and that another account's view is unchanged (04-24 block). *Browser pass.*
9. **Moderator screen (SC6).** A non moderator gets a 404 byte identical to an unknown path. As a moderator, check the three lists with counts, Keep, Remove with its confirmation, Find a report by id or pasted link, Refresh, at 320 px with Telugu and Hindi text (04-23 and 04-26 blocks). *Browser pass.*
10. **Account menu and ways in.** The menu stays reachable at 320x568, 320x256, 667x375 and 844x390, and at 200 percent text. The login panel's Low data link sets the cookie and redirects after sign in. Open full version clears it (04-21 block). *Browser pass.*

**Needs a person**

11. **Record the fixture scores with the owner's OpenAI key (ROBUST-03 calibration).** Run `MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord` with a fresh zero balance key. Expect either a recording, then `TestScreeningFixture` PASS (no genuine entry rejected, no serious entry withheld, every abusive entry rejected), or an `insufficient_quota` line that answers the budget question. Also post one abusive text live to see the 422 path. *Person: the owner's key.*
12. **Fluent reader check of the Hindi, Telugu and Tamil fixture texts (04-28).** The model wrote them. *Person.*
13. **Emergency numbers on live government pages (D-22).** Check 112, 100, 101 and 108, and the 1070 and 1077 wording. *Person.*
14. **Lawyer or mentor read of the Disclaimer and Policy pages (D-23).** Include the Policy scores wording in Anti-Patterns and the missing complaint path while `CONTACT_EMAIL` is unset. *Person: a lawyer or mentor.*
15. **Host canary for `CLIENT_IP_HEADER`.** Needs a chosen host. *Person: a chosen host.*
16. **Real phone on a weak network (04-19 block at line 376).** Check the location permission prompt, the accuracy wording against real GPS, and posting and voting from the Low data view on a real phone. *Person: a real phone.*
17. **Screen reader.** Check the #notice-live announcements (Connection lost and Back online, the slow notice, queue status), the Flag dialog and the moderator actions. *Person: a screen reader.*
18. **Safari storage eviction.** A saved report on iOS Safari after days without use (the documented 7 day limit). Not scriptable in any useful time. *Person: a real device over days.*
19. **Owner decisions and vetoes.**
    - The SC7 gap: a Policy link in the bar, or an override.
    - Whether `/swagger/*` counts as a "page" for SC5.
    - Choosing `CONTACT_EMAIL`.
    - The Low data byte budget.
    - The six RECOMMENDED DEFAULTS in 04-CONTEXT. Four are confirmed in code: the Flag control is last in a Low row; the take back shares the resolution cooldown; withheld reports drop off My votes; old reports read Expired or Resolved on the moderator screen. Two are client presentation and go to the browser pass before the veto: Flag reads "Flag this report" until pressed on Past or a report link (flag.js reads `viewer_flagged` or a page session record), and the Written line sits inline on the compact row (feed.js fills it; the placement is CSS).
    - The open D-05 veto on the letters only gibberish run.
    - The 04-27 "Product decisions" list, including the stranded pending report with no check row.
    - *Person: the owner.*

### Gaps Summary

**One criterion fails as written (BLOCKER for a `passed` verdict, likely an override):**

- **SC7: Policy is not linked from the bar.** The roadmap says both legal pages are "linked from the bar on every page". The bar links only Disclaimer (`web/templates/disclaimer_bar.html.tmpl`). Plan 04-06 did this deliberately, because the owner approved the exact bar sentence in D-22 and it ends in the single word Disclaimer. CONTEXT D-23 says the pages are "Linked from the bar, the login page and the account menu". Policy is reachable from the Disclaimer page, the account menu and the login panel. The fix is a one word template change plus test pins, but it alters owner approved wording, so the owner chooses. If the owner accepts the current design, add the override below. Note that an override matches the whole SC7 sentence, so it also covers the second deviation (no complaint path while `CONTACT_EMAIL` is unset); its reason names both so neither is accepted by accident:

```yaml
overrides:
  - must_have: "A public Disclaimer page and a public Policy page ... and both are linked from the bar on every page."
    reason: "Two accepted deviations: (1) the bar keeps the owner approved D-22 sentence with one Disclaimer link, and Policy is one step away from the Disclaimer page and is linked from the account menu and the login page; (2) the complaint and removal path shows only when CONTACT_EMAIL is set (D-24), which is launch checklist item 3."
    accepted_by: "<owner>"
    accepted_at: "<ISO timestamp>"
```

**Warnings (not blocking):**

- **`/swagger/*` has no bar.** It is the public API reference, served by a third party UI, and is outside the template set R-08 derives. Owner to say whether it counts as a page under SC5.
- **Complaint and removal path depends on configuration.** With `CONTACT_EMAIL` unset (the shipped default, D-24) neither legal page says how to complain or ask for removal (every such sentence, the removal line under retention included, is inside the `CONTACT_EMAIL` guard). This is on the README launch checklist (item 3) and must be done before any public deploy.
- **Stranded pending report.** A pending report whose check row write fails twice has no automatic recovery and is not listed for moderators. Rare (two consecutive database failures right after a successful insert), logged, and already recorded as an owner decision.
- **Low data page weight.** About 497 KB raw, or about 153 KB gzip. The budget test pins growth, not a low bandwidth target.
- **Policy scores wording** is narrower than what the worker stores.

**Present but behavior unverified (not failures):** SC1 and SC2 are built and wired, but their client flows are executed by no automated test. Browser items 1 and 2 cover them.

**Regression check against Phase 3.1:** the full suite, including every Phase 3.1 test, is green on a fresh database. Phase 3.1 remains `human_needed` on its own client side items, and none of its tests regressed.

---

_Verified: 2026-10-16_
_Verifier: Claude (gsd-verifier)_
