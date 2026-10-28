---
phase: 05-official-feed-demo-mode
plan: 20
subsystem: verification-and-docs
tags: [e2e, journey-test, docs, policy, credits, browser-pass, official-alerts, demo]
status: complete

requires:
  - phase: 05-05
    provides: the refresher, official storage and the stored feed status
  - phase: 05-07
    provides: the sign in page, the globe and the Try the demo row
  - phase: 05-08
    provides: GET /api/official/alerts and the honesty states
  - phase: 05-11
    provides: the official push queue and the D-16 rule
  - phase: 05-12
    provides: POST /internal/official/refresh and the ticker
  - phase: 05-14
    provides: the Settings switch
  - phase: 05-15
    provides: the Nearby banner
  - phase: 05-16
    provides: the official push worker
  - phase: 05-18
    provides: Try the demo in the first-time flow
  - phase: 05-19
    provides: the Demo's Official alerts, banner and My activity
provides:
  - TestPhase5OfficialJourney, one test across plans 05-05 to 05-16 on a real database
  - docs/OFFICIAL-ALERTS.md, the written design of the official feed and the Demo
  - TRUST-MODEL.md and PUSH.md brought up to what Phase 5 built, and a README section
  - three Policy sentences (sign in page hosts, NASA's GIBS acknowledgement, source credits) and their pin
  - the final browser pass of every Phase 5 surface except GDACS
affects: [05-21, 05-22, phase 8 sign in card, owner viewing session]

key-files:
  created:
    - internal/api/official_flow_e2e_test.go
    - docs/OFFICIAL-ALERTS.md
  modified:
    - docs/TRUST-MODEL.md
    - docs/PUSH.md
    - README.md
    - web/templates/legal_body.html.tmpl
    - web/legal_contract_test.go
    - internal/api/handlers/legal.go
    - internal/service/profile.go

key-decisions:
  - "The journey test reaches its fake SACHET by swapping http.DefaultTransport for the one host name and restoring it at cleanup. The SACHET client has no exported test seam, and this keeps production code untouched."
  - "Changing the Policy text moved its Last updated date and CurrentTermsVersion to 29 October 2026 together, as legal.go and the Policy itself require. Everyone will be asked to agree again."
  - "GDACS is described as planned and not built; the Policy credit sentence still names it, verbatim from the plan."

requirements-completed: []
duration: about 2 hours
completed: 2026-10-29
---

# Phase 5 Plan 20: Verification, documentation, Policy credits and the final browser pass Summary

**One journey test proves the official path from a fake SACHET to a silent phone note and through every honesty state, docs/OFFICIAL-ALERTS.md writes down how it works and what is still open, the Policy now says which hosts the sign in page contacts and credits the sources, and all 176 browser checks across both engines, both sizes and both themes passed.**

## Task commits

| Task | Commit | What |
|------|--------|------|
| 1 | 172196ed | test(05-20): TestPhase5OfficialJourney |
| 2a | 6b4d9ab0 | feat(05-20): the three Policy sentences, their pin, and the Policy date and terms version |
| 2b | f663aa56 | docs(05-20): OFFICIAL-ALERTS.md, TRUST-MODEL.md, PUSH.md, README.md |
| 3 | none | browser pass and full suite only, no file changed (the scratch server test was deleted before any commit) |
| meta | the commit holding this file | SUMMARY.md |

## What was built

**Task 1, the journey.** `TestPhase5OfficialJourney` in `internal/api/official_flow_e2e_test.go` builds the real router over a real database with the real `official.NewService` and the refresh door open, and walks: (1) `POST /internal/official/refresh` with the token answers 200 with sachet stored at least 1; (2) a verified reader at 16.5062, 80.6480 gets `fresh`, one alert in the reader group with the Official badge, issuer, issued and end times, and an Extreme "Cyclone warning" banner; (3) one `official_push_outbox` row is queued for an account whose home cell lies in district 749; (4) `OfficialPushWorker.SweepOnce` sends one payload (kind `official`, url `/alerts`, the closed title and body, urgency high, the series Topic and Tag), nothing more on a second sweep, and the payload names no sound or vibration; (5) with the last good check moved to 2 hours back the read says `stale`, keeps the row and shows no banner, and at 7 hours back says `unavailable` with no rows and no banner, with the upstream never reached by a read; (6) the reports, votes and sessions counts equal those taken after the reader signed in. The alert times are built from the clock, so the hook's recency rules are really exercised. It passed at once; no plan's seam was broken and no production file needed a fix for it.

**Task 2, docs and Policy.**
- `docs/OFFICIAL-ALERTS.md` has the eight sections the plan names. It says the sources and credits, the two step fetch and its bounds, the three refresh layers and their limits (a sleeping host, the 744 of 750 free hours, GitHub's delays and 60 day rule, the 30 minute alternative), the three honesty states and the one rule for the "no alert in force" sentence, district placement and the about 1 km edge limit, the D-16 rule for the banner and the phone, Update and Cancel and the 6 hour and 7 day expiry rule, the push lane (silent, own outbox, 3 per 6 hours, 2000 per alert, series Topic and Tag, the Apple choice), the Demo (in memory world on the real services, the differential test, nothing written, the 1 km rule kept, neighbours in different cells, the climb timing) and the open questions. Every test it names was checked to exist. It holds no long dash and none of the three banned phrases.
- `docs/TRUST-MODEL.md`: the Phone alerts kinds line now describes the built official kind, a new subsection "Official alerts on the phone" covers who is told, the separate lane, closed words, the series and Apple choice and the send time recheck, the Phase 5 hand off paragraph and the Phase 3.3 hand off sentence describe what was built (the old text proposed exempting demo reports; what was built keeps every rule), two Honest limits and the Out of scope list are updated. The heading order test and `TestTrustModelDocMatchesCode` pass.
- `docs/PUSH.md` was re-read whole first (3.3's sections are untouched). It gains the official switch in the list, an "Official alerts" section, the official Topic and Urgency in "What the push service can see", migrations 00012 and 00013 in check 0, and owner check 6.
- `README.md`: a section "Official alerts and the Demo", and `OFFICIAL_REFRESH_TOKEN` in the environment list.
- `web/templates/legal_body.html.tmpl`: the three P5-C20 drafts, verbatim, inside "Other services Pinalert uses" next to the map sentence. `TestPolicyNamesSignInHostsAndCredits` in `web/legal_contract_test.go` pins each once, inside that section, and that the map sentence is still there.

**Task 3, the browser pass and the suite.** See the table and the results below.

## Final browser pass (Playwright, WebKit and Chromium, 390x844 and 1470x830, light and dark)

Method: the real `api.NewRouter` over a throwaway database, served on 127.0.0.1:8792 by a throwaway Go test (literal throwaway session secret, no mail key, a SACHET fetcher that panics if reached; it never ran). Seeded: a fresh feed status, two district alerts and five state wide Severe lightning alerts and one unplaced alert for Vijayawada (district 749), accounts with a home area, and one without. Signed in with signed session cookies the test made. Computed style and DOM probes only; no screenshot was saved. The feed state was moved through psql on the scratch database. Each cell below is 8 of 8 (2 engines x 2 sizes x 2 themes) unless stated, 176 checks in all, every one passed.

| Surface | Result | What was measured |
|---------|--------|-------------------|
| Sign in page | pass 8 of 8 | Try the demo, Disclaimer, Policy in that order, each at least 44px, form visible, no horizontal overflow, the NASA credit present, the globe reached its ready state in every run, no page error |
| Policy page | pass 8 of 8 | the host sentence, NASA's GIBS acknowledgement, the source credit and "Last updated 29 October 2026" all present, no overflow |
| Demo Nearby | pass 8 of 8 | strip "Demo. Nothing here is real." first, Exit 44px to /welcome, 18 rows each with the Demo tag, no cookie set, no overflow, no page error |
| Demo banner | pass 8 of 8 | 44px, Demo tag shown, "Flood warning", "Ntr district", "+1", links to /demo/alerts with the visit |
| Demo Too far to vote | pass (separate probe, 4 of 4) | 8 of 18 rows say Too far to vote in both engines at both sizes |
| Demo practice report | pass 8 of 8 | the first step "Needs 2 people" at 0.3 to 0.8 s, Confirmed at 9.2 to 9.3 s, announced once as "Practice report confirmed. Confirmed by 2 people at different spots."; the toast "Practice report posted. Pretend neighbours will confirm it in a few seconds." showed in the separate probe, 4 of 4 |
| Demo Official alerts | pass 8 of 8 | 3 sample rows, "Where you are" group, the made up line, no NDMA, SACHET or GDACS word, no Official badge, no overflow |
| Demo My activity | pass 8 of 8 | Posted part, Demo tag, no overflow |
| Demo Exit | pass 8 of 8 | lands on /login for a visitor with no account (Exit target /welcome redirects there) |
| Demo storage | pass 8 of 8 | zero geolocation calls, no session cookie; the only localStorage key is `pinalert_theme`, at laptop width, the known one |
| First-time flow, step 1 pair | pass 16 of 16 | Try the demo (outlined) left of Continue, same row, 48px, no underline |
| First-time flow, Disagree box | pass 16 of 16 | the restored sentence, Try the demo 48px to /demo?from=welcome |
| First-time flow, four steps and the alerts pop-up | pass 8 of 8 | the flow ran step 1 to 4 and posted consent, profile and first run; the pop-up text has no official word. WebKit drew the pop-up (Get alerts for critical reports near Vijayawada?). Headless Chromium reported the notification state as blocked, so it did not draw it and its markup was read instead; see the first finding below |
| Nearby banner (fresh) | pass 8 of 8 | 44px, "Cyclone warning", "Ntr district", "+1", href /alerts, no underline |
| Official alerts (fresh) | pass 8 of 8 | 4 rows, groups Where you are, Across Andhra Pradesh, Could not be placed on a map, "alerts like this" grouping, no Demo tag, details open, no overflow |
| Official alerts (stale) | pass 8 of 8 | the could not check notice, the 4 last known rows kept, no "No official alerts in force", Nearby banner hidden |
| Official alerts (unavailable) | pass 8 of 8 | 0 rows, "Official alerts could not be checked", no reassuring sentence |
| Settings official row | pass 8 of 8 | "Severe official alerts for my area", switch role, no sound or vibration word |
| Page errors on the signed in pages | pass 8 of 8 | none |

Not done here, by design: GDACS (plans 05-21 and 05-22), the iPhone simulator and Mac Safari (the orchestrator's), a 320 pixel width (plans 05-15 and 05-19 measured it), the area unknown and load error states of Official alerts (node cases only, as in plan 05-09).

## For the viewing session

Built from UI-SPEC "Defaults to confirm"; each stands unless the owner says otherwise. The owner looks at a test copy.

1. Severe "orange" is the app's amber, not a new orange. Where: a Severe row chip on Official alerts and a Severe Nearby banner.
2. The sign in row: three outlined 44px links with no drawn dots, two lines at 360 to 390 px. Where: /login on a phone.
3. Step 1 pair order: Try the demo (outlined) left, Continue (filled) right, stacked below 340 px. Where: /welcome step 1.
4. The Disagree box has a third button, Try the demo. Where: /welcome step 2, Disagree.
5. The Demo shows one strip only, no laptop "Unofficial" strip. Where: /demo at laptop width.
6. The Demo has no Search a place, bell, gear, person menu, Flag, Mute, Call 112 box or What is happening picks, and My activity has four parts. Where: /demo and /demo/activity.
7. Demo pins carry a dashed ring. (The GDACS pin look waits for plans 05-21 and 05-22.) Where: the Demo map.
8. Times follow the reader's own time zone, 12 hour, lowercase am and pm; Issued is relative, Until is a clock time ("About 8:35 pm" when inferred). Where: Official alerts rows.
9. Grouped rows count once in "In force now" and the panel shows the newest alert in full. Where: the "alerts like this" row.
10. The Low data view has no banner, Official alerts or globe, and the Demo from a Low data device opens the full app. Where: /low and /demo?low.
11. Official alerts lists Moderate, Minor and Unknown alerts with a neutral chip. Where: the unplaced row.
12. The globe credit link style. Where: the sign in page corner (laptop) or under the Earth (phone).
13. The Policy words (three drafts). Where: /policy, Other services Pinalert uses.
14. Two things the earlier plans left for this session: the Nearby banner area text gets 8 px at 320 px wide with the longest phrase and the Demo tag (the row stays one line and the accessible name is complete; decide whether the dot and area should drop out under about 5 characters), and the phone climb timing in the Demo (3 s poll against the 4 s neighbours gives the second step at about 6 s and Confirmed at about 9 s; either constant can be shortened).

## Owner steps (user_setup)

Nothing below was done by this plan.

1. **NDMA SACHET terms.** Open https://sachet.ndma.gov.in/WebsitePolicy in a browser, read it, and email NDMA's SACHET contact asking in writing whether a free, unofficial, attributed relay is welcome and what request rate is acceptable. Required before any wide promotion, not before the portfolio deploy. Nothing in the docs says NDMA has agreed.
2. **District boundary licence wording.** Before the first public share, read the Policy credit and the credit section of docs/OFFICIAL-ALERTS.md and confirm or change the wording; include it in the legal or mentor review CLAUDE.md asks for.
3. **Policy wording.** Approve or rewrite the three Policy sentences at the viewing session. Any rewrite changes the Policy text, so the Last updated date and `CurrentTermsVersion` need the same day again (see the first deviation).
4. **Real phone.** With the site on its https host, the phone installed, the cron set up and VAPID keys set, confirm one silent notification for a real Severe or Extreme alert at the home district that opens Official alerts (PUSH.md owner check 6).
5. **Migrations.** `make migrate` on the owner's development database, after the owner says yes, to add 00012 and 00013. An executor never runs it.
6. **Cron.** Add the GitHub secret `OFFICIAL_REFRESH_TOKEN` and the repository variable `PINALERT_BASE_URL`, set the same token (at least 32 characters) on the host, and choose a 10 or 30 minute cadence. The workflow does nothing while the variable is unset.
7. **Orchestrator.** The iPhone simulator and Mac Safari passes of the sign in page, the Demo and Official alerts.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] The Policy text changed, so its date and terms version moved with it**
- **Found during:** Task 2, reading `internal/api/handlers/legal.go`.
- **Issue:** legal.go says the Last updated date is edited together with the text and that `service.CurrentTermsVersion` moves to the same day so everyone is asked to agree to the new text (D-27). The Policy itself promises "When this text changes, everyone is asked to read it and agree again." Adding three sentences without moving them would leave a Policy that changed under an old date.
- **Fix:** `policyLastUpdated` is "29 October 2026" and `CurrentTermsVersion` is "2026-10-29". `TestTermsVersionFollowsPolicyDate`, the welcome, onboarding and legal tests pass.
- **Consequence for the owner:** after this merges, every account is sent through the agree step again. Rewriting the three drafts at the viewing session means one more same-day move of the two constants (use the real date of that day).
- **Files modified (outside the plan's list):** internal/api/handlers/legal.go, internal/service/profile.go. **Commit:** 6b4d9ab0.

**2. [Rule 3 - Blocking] No exported seam for a fake SACHET in package api**
- **Issue:** the SACHET client's test constructor is unexported and the journey must live in package api (the plan's file).
- **Fix:** the test points `http.DefaultTransport` at an in process fake for the host name `sachet.ndma.gov.in` only and restores it on cleanup. No production code changed. The test asserts the fake was reached, so a swap that did not take cannot pass.

### Other changes beyond the plan text (no rule)

- TRUST-MODEL.md: besides the sentence the plan names, two stale Phase 5 lines (the Hand offs demo paragraph and the Out of scope lines "Demo mode (Phase 5)" and "Official alerts ... wait for Phase 5") were updated, because they contradicted what was built.
- README.md also gained the `OFFICIAL_REFRESH_TOKEN` line in the environment list.
- The Policy pin is a new test function; no existing assertion was edited, so no "Amended" note was needed.

### Process notes

- The sandbox refuses a server with ENV=development, and I invented no secret. The browser pass used a throwaway Go test (`internal/api/zz_p5_20_walk_test.go`, literal throwaway session secret, a public VAPID key of a throwaway pair with its private half dropped in memory so the pop-up could be offered, no mail key) that served the real router on 127.0.0.1:8792 over p5_20. It was deleted right after the browser pass, before the full suite, and never added to git. `git status` is clean.
- No call to the real SACHET host was made by this plan.
- Two scratch server restarts were needed (a missing page configuration made the Demo feed ask for radius 0, then a missing public push key hid the pop-up); both were fixes to the scratch harness only.

## Verification

- `go build ./...` and `go vet ./...` clean. `node --test web/*.mjs`: 1048 tests, 1048 pass, 0 fail.
- Full Go suite on p5_20 (created fresh, migrated to version 13): 5198 `--- PASS` lines, SKIP lines exactly TestGeocodeLive, TestOfficialLive and TestScreeningFixture, 19 of 20 packages `ok`. One failure, the known planner statistics flake `TestVotedReportsByAccountUsesTheAccountIndex` in `internal/store` (this plan changes nothing under internal/store). As the project notes say, p5_20 was dropped and recreated and `internal/store` rerun on it: `ok`. No other FAIL and no panic.
- `TestPhase5OfficialJourney`, `TestPolicyNamesSignInHostsAndCredits`, `TestTermsVersionFollowsPolicyDate`, `TestTrustModelDocMatchesCode`, `TestUserVisibleCopyUsesPlainPunctuation` and the legal, welcome and onboarding tests pass.
- Acceptance greps: `func TestPhase5OfficialJourney` 1; `grep -c '^## ' docs/OFFICIAL-ALERTS.md` 8 and `## How alerts stay honest` 1; `Global Imagery Browse Services` 1 and `Datameet and ramSeraph` 1 in the Policy template; no em or en dash in OFFICIAL-ALERTS.md or in any added line of the other files; zero matches for "cannot be gamed", "approved by NDMA" and "buzz" in OFFICIAL-ALERTS.md.
- p5_20 dropped and confirmed gone with `psql -lqt`; the 8792 server stopped and the port free.

## Findings for the orchestrator

1. **The first-time alerts pop-up could not be drawn by headless Chromium here.** `PinalertNotify.baseState()` returned `blocked` even with the notifications permission granted to the context, so `canAsk()` is false and the pop-up does not open; WebKit opened it. Its markup was read in Chromium and holds no official word. The owner's viewing session on a real browser covers it.
2. **The welcome page now contains the word "official"** in the Policy scroll text (the new credit sentence). The pop-up and steps do not. If any later check bans the word on /welcome as a whole it will trip on the Policy text.
3. **Plan 05-18's focus trap finding is still open**: `FOCUSABLE_SELECTOR` in welcome.js does not list links, so Tab from the Disagree box's Try the demo link leaves the dialog instead of wrapping. A one token fix (add `a[href]`) needs a decision because it touches welcome.js.
4. **The Policy sentence names GDACS** though GDACS is not built yet (plans 05-21 and 05-22). The credit is in place for when it is.
5. **The terms version bump** (first deviation) reaches every existing account, including test accounts on the owner's copies, when this merges.

## Known Stubs

None. The docs say plainly that GDACS is not built.

## Threat Flags

None beyond the plan's register. T-05-103 (no claim of NDMA agreement or of an unbeatable trust model: the negative greps and the open terms question stated), T-05-104 (the Policy names unpkg.com and Pinalert's NASA copy before consent: the Policy pin), T-05-105 (credits in the Policy and docs, the licence check left to the owner) and T-05-106 (one journey test, the full suite and the browser pass) are mitigated as planned.

## Self-Check: PASSED

- Files exist: internal/api/official_flow_e2e_test.go, docs/OFFICIAL-ALERTS.md, and the modified docs, README, Policy template, legal test, legal.go and profile.go.
- Commits 172196ed, 6b4d9ab0 and f663aa56 are on branch worktree-agent-a312515d2ef9cafa0.
- p5_20 is dropped, 8792 is free, no scratch file is in the worktree.
