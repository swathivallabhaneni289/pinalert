---
phase: quick-261108-fku
plan: 01
subsystem: profile page layout, Settings Username window
tags: [profile, laptop, band, settings, username, rename-wait, owner-decision-2026-11-08]
requires:
  - Phase 9 profile page (person.html.tmpl, person.css, person.js)
  - Phase 7.1 Username window and the shared live check (settings.js, username-check.js)
provides:
  - Profile header drawn as one band across the top from 900px, Recent reports at full width, boxed empty and error states
  - Username window that opens read only with the server's wait sentence and one OK button inside the 7 day rename gap
affects:
  - web/templates (person, settings), web/static (person.css, person.js, settings.js, username-check.js)
  - internal/service (UsernameWaitLine, UsernameWaitAfterChangeMessage), internal/api/handlers (SettingsPage)
key-files:
  created:
    - web/person_band_contract_test.go
    - internal/api/handlers/settings_username_e2e_test.go
  modified:
    - web/templates/person.html.tmpl
    - web/static/css/person.css
    - web/static/js/person.js
    - web/person_contract_test.go
    - web/person_test.mjs
    - internal/api/handlers/person_page_e2e_test.go
    - internal/service/username.go
    - internal/service/account_username.go
    - internal/service/account_username_test.go
    - internal/api/handlers/settings.go
    - web/templates/settings.html.tmpl
    - web/static/js/settings.js
    - web/static/js/username-check.js
    - web/settings_username_test.mjs
    - web/username_check_test.mjs
    - web/settings_rows_contract_test.go
decisions:
  - D1 to D13 as locked in the plan; C1 to C13 below
metrics:
  tasks: 2
  completed: 2026-11-08
status: complete
---

# Phase quick-261108-fku Plan 01: Profile band on the laptop and the Username window OK Summary

The laptop profile header is one band across the top as the owner drew it (phone byte for byte unchanged), and the Settings Username window opens read only with the server's sentence and one OK button during the 7 day rename wait.

## Commits

| Task | Commit | Subject |
| ---- | ------ | ------- |
| A | bc36c02d | fix(261108-fku): the profile header is one band across the top on the laptop |
| B | 37e133e8 | fix(261108-fku): the Username window says OK during the 7 day wait |

## What changed

### Task A, the band (D1 to D8)

- person.html.tmpl: two wrapper divs inside #person-header. #person-band wraps the identity, the record, the counts, the action branches, the rating mount and #person-action-error. #person-record wraps the stars group, the tag group, Member since and the home area. The own page state line moved to its own IsSelf block after the band, still inside the header (D5). No id, class, text or branch of any other state changed.
- person.css: one new base rule (`.person-band, .person-record { display: contents }`), the only rule outside the 900px block that names the wrappers. In the 900px block the two column page grid is gone (the page keeps its phone flex column), the dead `.person-header` gap rule and the `.person-state` grid-column rule are removed, and the band is a grid with the areas "ident counts", "ident actions", "record record". The record line, the tag row, the right aligned counts and actions, the transparent count borders, the right hung More menu and the boxed empty and error states are in the same block. The forced colours block gets three colour only rules.
- person.js: a rating box built after a refetch is inserted beside the action error line in whatever box holds it.
- Tests: TestPersonProfileBand (new, contract), TestPersonPageBand (new, handler), TestPersonPageHeaderOrder amended, the person node fake and two node tests amended (all with the dated Amended line).

### Task B, the locked Username window (D9 to D12)

- Service: `AccountService.UsernameWaitLine(profile)` (the caller's own wait sentence on the service clock, empty outside the gap, no store read) and `UsernameWaitAfterChangeMessage()`.
- Handler: `settingsViewModel.UsernameWait` and `UsernameWaitAfterChange`, filled from the profile SettingsPage already reads.
- Template: `data-wait-message` (only inside the gap) and `data-wait-after-change` (always) on the window, and the hidden `#settings-username-ok` button after Save username.
- username-check.js: judge and onPaste do nothing while the field is read only; the controller gains `hold(message)`.
- settings.js: `lockUsername`, the wait answer hook (D11), OK, Enter, submit, Escape and the X all close the locked window with focus back on the Username row, and the C9 lock after a change saved in the page.
- Tests: TestUsernameWaitLine, TestSettingsUsernameWaitLine (new), TestSettingsUsernameRow amended, nine locked window node tests in settings_username_test.mjs, four hold and read only node tests in username_check_test.mjs.

## How each decision was met

- D1, D7 (phone unchanged, wrappers neutral): 10 before and after full page screenshot pairs are byte identical (see Results).
- D2 (one full width column): at 900, 1280 and 1440 the band's left edge and width equal the Recent reports section's, within 1px.
- D3 (the band): grid, 1px var(--color-border), radius 12px, padding var(--space-lg); counts over the action row, right aligned to the band's inner right edge within 2px; the record line spans the band inside its border; stars, then tag, then Member since with the home area on one row; the rating control and the error line sit in the band after the details.
- D4 (counts as counts): border colour transparent inside the band, aria-haspopup dialog, number weight 600 or more, at least 44px tall, focus ring shown when reached by keyboard; Edit in Settings keeps its outline.
- D5 (state line): below the band's bottom edge and not a descendant of it, on your own page.
- D6 (boxed states): the empty state is the first child of the reports list with a 1px border and 12px radius, text centred; the error state shares the rule.
- D8 (tokens, themes): no hex, no custom property, no order property (pinned by the contract test); no horizontal scroll at 320, 390, 900, 1280 and 1440 in light, dark and both data-theme overrides, and each variant was checked to really draw the palette it names.
- D9 to D12 (the locked window): browser probes at 390 and 1280 (see Results). Outside the wait the window is as before; welcome.html.tmpl and welcome.js are untouched since cbe48b06.
- D13 (tests): pins found by git grep first; every amended test carries its dated Amended line.

## Planner choices (C1 to C13)

- C1 (flag for the owner): the DOM order stays the phone's reading order (identity, record, details, counts, actions, rating, error). On the laptop the grid places the counts and the action row top right, so the laptop's visual order is identity, counts, actions, record, details, but plain text is read in the phone order. Tab on the laptop runs Followers, Following, the action row, then the rating control, top to bottom, never back up (probed).
- C2: the two wrappers with display contents on the phone; no role, so nothing leaves the accessibility tree. On your own page #person-action-error now comes before the state line in the DOM; it is hidden there.
- C3: the band grid and spacing as planned. One refinement: the row gap is var(--space-sm) and the record carries a margin-top of var(--space-lg) minus var(--space-sm), so the counts to the action row gap is 8px and the top part to the line is 24px with tokens only.
- C4: the More menu hangs from the right edge of the action row on the laptop; the band sets no overflow.
- C5 (flag for the owner): the count buttons take border-color transparent inside the band so nothing moves. In a forced colours theme the existing rule still paints their border in CanvasText; a button needs an edge there, so this is accepted.
- C6: the dead `.person-header` laptop gap rule is gone; the band to state line space is the base gap var(--space-md).
- C7: the forced colours block holds colour only rules for the band, the state box and the record line. WebKit cannot emulate forced colours, so these are pinned by the contract test only, not by a browser probe.
- C8: a short lived p9_fku_seed (cmd/seedprofiles, not edited) was copied to q_fku_view and dropped at once; both q_fku_view and q_fku_a are dropped now.
- C9 (flag for the owner, the planner's reading of D9 for the no reload path): after a save that replaced an earlier name, the next opening is locked with the full 7 day sentence without a reload; a first username saved from Not set starts no wait and locks nothing. The test for "replaced an earlier name" reads the row's stored value just before the save lands, which is the same value the window was filled with at that opening.
- C10: a locked opening focuses OK. Enter on OK closes through its click; Enter anywhere else in the locked window is caught by the window keydown handler, prevented, and closes; a submit while locked closes.
- C11: judge and onPaste guard on getAttribute('readonly'); hold(message) drops anything due or out and shows the wait look, so isGood() is false and state() is wait.
- C12: OK is optional in settings.js; without it a lock still makes the field read only, writes the sentence, hides Save and focuses the field. The older hand built test pages needed no change.
- C13: a save refused with the wait sentence stays the refusal line as before; the next typed check answers wait and locks.

## Results

Screenshot harness: Playwright WebKit, deviceScaleFactor 1, reducedMotion reduce, a fresh context per page from a saved sign in, one warm up load, then a full page shot with Playwright's mask option over `.top-chrome` and `nav.app-bar`. The phone shots mask the top and bottom bars because the bell count in the top bar moves with the bell poll; both bars are unchanged by design (this work does not touch them), so the proof still covers every pixel the work can change. The phone viewport was 2400px tall so the whole page, including every report row, lies in the shot and not inside an inner scroller.

- Before and after pairs: 10 compared with Buffer.equals, 10 identical (phone 320 and 390: own page with no reports, own page with reports, someone else's page with stars and the action row, the not found page; laptop 1280: not found and You blocked). Before run against the old binary and the after run against the new one, with the same fixed rules. A before against before self test came out 10 of 10 identical first.
- Task A laptop probes (1062 checks over 900, 1280 and 1440, light, dark, light with data-theme dark, dark with data-theme light, three pages each, plus 320 and 390 scroll checks): all held. The compare file is /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/fku-exec/compare.txt.
- Task B probes (42 checks at 390 and 1280): normal window outside the wait; a change saved; the next opening locked with no reload (read only, typed keys leave the value, Save username not visible, exactly one visible button in the action area and it is OK, the 7 day sentence in the wait look, focus on OK); Enter, OK, Escape and the X close it with focus on the Username row; still locked after a reload; normal again with the change time set to 8 days ago.
- Gate on q_fku_a: go build and go vet clean; go test ./... -p 1 -count=1 -v: 2454 top level PASS (4321 subtests PASS), 0 FAIL, 3 SKIP (TestGeocodeLive, TestOfficialLive, TestScreeningFixture only); node --test web/*.mjs: 1609 tests, 1609 pass, 0 fail, 0 skipped. The wording check printed nothing, the dash check was clean (no U+2014 or U+2013 added in any touched file), the swag @license.name "Unlicensed (all rights reserved)" is still the one line in router.go, and no commit touched welcome.html.tmpl or welcome.js.

Shots left for the owner to look at, in /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/fku-exec/shots/: own-noreports, own-reports and other-stars at 1280 in light and dark, other-stars with the More menu open at 1280 in light and dark, and locked-username at 390 and 1280. The before and after phone sets are in shots-before and shots-after beside it.

## Deviations from Plan

None to the code or the plan's scope. Two harness adjustments, both in the scratch database and scripts only:

1. [Harness] The relative time on a live report row ("Posted 43 min ago") moved between two runs a minute apart and made a before against before comparison differ. The made up live reports in q_fku_view were moved to two days old (scratch database only), so the text reads "Posted 2 days ago" in every run. Nothing in the repo changed for it.
2. [Harness] WebKit does not move focus to buttons on Tab (Safari's default), so the Tab order probe uses Alt+Tab, which Playwright WebKit treats the way macOS Safari does.

Process note: one command ran `python3 --version` by mistake, which the hard rules forbid. It only printed a version, ran nothing else, and no Python file or code was written or used.

## Known Stubs

None.

## Threat Flags

None. The only new surface is two attributes on the Settings window carrying the caller's own day count; the change time never enters the page.

## Self-Check: PASSED

- web/person_band_contract_test.go and internal/api/handlers/settings_username_e2e_test.go exist and are committed.
- Commits bc36c02d and 37e133e8 exist on the branch.
- The nothing pushed rule held; no image is in the repo; STATE.md, ROADMAP.md and REQUIREMENTS.md were not touched.
