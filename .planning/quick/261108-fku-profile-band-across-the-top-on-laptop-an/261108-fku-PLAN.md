---
phase: quick-261108-fku
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/templates/person.html.tmpl
  - web/static/css/person.css
  - web/static/js/person.js
  - web/person_band_contract_test.go
  - web/person_contract_test.go
  - web/person_test.mjs
  - internal/api/handlers/person_page_e2e_test.go
  - internal/service/username.go
  - internal/service/account_username.go
  - internal/service/account_username_test.go
  - internal/api/handlers/settings.go
  - internal/api/handlers/settings_username_e2e_test.go
  - web/templates/settings.html.tmpl
  - web/static/js/settings.js
  - web/static/js/username-check.js
  - web/settings_username_test.mjs
  - web/username_check_test.mjs
  - web/settings_rows_contract_test.go
autonomous: true
requirements:
  - QUICK-261108-fku
must_haves:
  truths:
    - "D2: at 900px and up the profile page is one full width column: the band, then (own page only) the state line, then Recent reports at full width; no side column and no centred narrow column (03.4 R6)"
    - "D3: the band is one box (1px var(--color-border), radius 12px, padding var(--space-lg)); its top part holds the letter disc, the username h1 and the name on the left and the Followers and Following counts over the action row, right aligned, on the right; a 1px line runs across it; its lower part holds the stars when present, the tag chip with its sentence beside it (the sentence wraps under itself), and Member since with the home area on one line; on someone else's page the Your rating control and the action error line sit in the lower part after the details"
    - "D3: the More menu opens fully on screen and is not clipped by the band at 900, 1280 and 1440"
    - "D4: inside the band the Followers and Following buttons draw no visible border, keep the semibold number, aria-haspopup dialog, at least --touch-target-min tall, the --color-surface hover fill and the focus ring; Edit in Settings stays the outlined .btn"
    - "D5: on your own page the role status state line sits directly under the band and outside it"
    - "D6: on the laptop the Recent reports empty state and error state sit in a 1px var(--color-border) box with a 12px radius at the top of the section, text centred; the rows box, the not found, load failed and You blocked states are unchanged"
    - "D1, D7: below 900px nothing changes: before and after full page screenshots (the top chrome and the bottom bar masked the same way in both runs) at 320 and 390 of your own page with no reports, your own page with reports and someone else's page with stars and the action row, and of the not found page, are byte identical, and the laptop not found and You blocked pages are byte identical at 1280"
    - "D7: the DOM order stays the phone's reading order and no rule uses the CSS order property; on the laptop Tab runs top to bottom: Followers, Following, the action row, then the rating control"
    - "D8: tokens only, no new custom property; no horizontal scroll at 320, 390, 900, 1280 and 1440 in light, dark and both data-theme overrides"
    - "D9: for an account inside the 7 day rename gap the Settings page carries the server's wait sentence; the Username window then opens with the current username read only (readonly and aria-readonly true), the sentence in the status line with the wait look, the help line, and one OK button (btn--primary); Save username is not shown and no live check is sent"
    - "D10: OK, Enter in the form, Escape and the X all close the locked window the way the X does (focus back to the Username row) and nothing is saved"
    - "D11: a wait answer from the live check while the window is in its normal state switches it to the same locked state with the server's sentence"
    - "D12: outside the wait the window works exactly as before (the box, the live check, Save username); step 4 of sign up (welcome.html.tmpl, welcome.js) is untouched"
    - "C9: after a username change saved in this page (a name that replaced an earlier one) the next opening of the window is locked with the server's full gap sentence; a first username saved from Not set starts no wait and locks nothing"
  artifacts:
    - path: "web/templates/person.html.tmpl"
      provides: "The band wrapper and the record wrapper inside #person-header, the own page state line after the band"
      contains: "id=\"person-band\" class=\"person-band\""
    - path: "web/static/css/person.css"
      provides: "The phone neutral wrappers in the base and the band layout inside the min-width 900px block"
      contains: ".person-band"
    - path: "web/static/js/person.js"
      provides: "The rating host is inserted beside the action error line in whatever box holds it"
      contains: "els.error.parentNode.insertBefore"
    - path: "web/person_band_contract_test.go"
      provides: "The band pins: laptop rules inside the 900px block, counts without a border in the band, the boxed empty state, the phone base rules unchanged, no order property"
      contains: "func TestPersonProfileBand("
    - path: "internal/api/handlers/person_page_e2e_test.go"
      provides: "The rendered band holds the identity, the record, the counts, the actions and the action error; the own page state line is after it"
      contains: "func TestPersonPageBand("
    - path: "internal/service/account_username.go"
      provides: "The wait sentence for the caller's own account on the service clock, empty outside the gap"
      contains: "func (s *AccountService) UsernameWaitLine("
    - path: "internal/service/username.go"
      provides: "The full gap sentence used after a change saved in the page"
      contains: "func UsernameWaitAfterChangeMessage() string"
    - path: "internal/api/handlers/settings_username_e2e_test.go"
      provides: "The Settings page renders the wait sentence inside the gap and none outside it"
      contains: "func TestSettingsUsernameWaitLine("
    - path: "web/templates/settings.html.tmpl"
      provides: "The OK button and the two data attributes on the Username window"
      contains: "id=\"settings-username-ok\""
    - path: "web/static/js/username-check.js"
      provides: "The read only guard and the hold method of the controller"
      contains: "hold: function"
    - path: "web/settings_username_test.mjs"
      provides: "The locked window node tests"
      contains: "opens locked"
  key_links:
    - from: "internal/api/handlers/settings.go"
      to: "AccountService.UsernameWaitLine"
      via: "SettingsPage fills UsernameWait from the profile it already read"
      pattern: "accounts\\.UsernameWaitLine\\(profile\\)"
    - from: "web/static/js/settings.js"
      to: "the data-wait-message attribute of #settings-username-window"
      via: "read once at load; a non empty value opens the window locked"
      pattern: "data-wait-message"
    - from: "web/static/js/settings.js"
      to: "PinalertUsernameCheck controller hold"
      via: "lockUsername writes the sentence through the controller so the status line classes stay owned by username-check.js"
      pattern: "\\.hold\\("
    - from: "web/static/js/person.js"
      to: "#person-action-error"
      via: "a rating host built after a refetch is inserted into the band before the error line"
      pattern: "els\\.error\\.parentNode\\.insertBefore\\(host, els\\.error\\)"
---

<objective>
Two owner fixes from 2026-11-08, testing on the laptop, each its own commit.

A. The profile page (/u/{username}) on the laptop. The owner said their own profile "looks really off", "all the tabs all over the place with no proper difference", "how they are arranged looks really misplaced". At 900px and up the header becomes one band across the top, drawn by the owner (below), and Recent reports run full width under it. The phone stays exactly as it is.

B. The Settings Username window during the 7 day rename wait. The owner: after changing the username, opening the window again it "is saying i can only change for 7 days but we have to click (x) to continue and we only have save username". Inside the wait the window opens read only with the server's wait sentence and one OK button.

Purpose: information and buttons look different on the profile header, and the rename wait never leaves the person with a dead Save button and only the X.

Output: the band (template wrappers, laptop CSS, one person.js line), the locked Username window (a service method, two view model fields, the template, settings.js, username-check.js), their tests, byte identical phone screenshots, a full green gate on q_fku_a, and the SUMMARY.
</objective>

## Why one plan with two ordered tasks

The orchestrator asked for one quick plan with two tasks, A then B, each its own path limited commit. They share no source file. A runs first because its before screenshots must be taken from a binary built before any edit. B reuses A's view server for its browser check. The full gate and the SUMMARY come at the end of B. If context runs low, finish the current task, commit it, and stop with a SUMMARY note naming what is left; never leave a task half committed.

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@web/templates/person.html.tmpl
@web/static/css/person.css
@web/static/js/person.js
@web/templates/settings.html.tmpl
@web/static/js/settings.js
@web/static/js/username-check.js
@internal/api/handlers/settings.go
@internal/service/account_username.go
@internal/service/username.go

## The owner's drawing for A (reproduce as drawn)

```
+-------------------------------------------------------------+
| (T)  testing.2                    0 Followers   0 Following |
|      testing.1                          [ Edit in Settings ]|
|-------------------------------------------------------------|
| [New reporter]  Not rated yet. It needs 5 reports with a    |
|                 clear result.                               |
| Member since October 2026    (pin) Benz Circle, Vijayawada  |
+-------------------------------------------------------------+
  Only you can see this page. Turn on Let others follow me
  in Settings.

  Recent reports
  Reports from the last 30 days, newest first.
+-------------------------------------------------------------+
|              No reports in the last 30 days                 |
|              Reports you post will show here.               |
+-------------------------------------------------------------+
```

Project rule: when the owner picks a drawn layout, copy it; record any conflict with a house rule in the SUMMARY, do not adapt the drawing.

## Locked decisions (from the orchestrator, owner 2026-11-08; plan exactly these)

- D1 (A) Laptop only. Every layout change is inside the existing min-width 900px block of person.css. Below 900px the page looks exactly as today. Same features and the same words on both sizes; nothing hidden on one size only.
- D2 (A) At 900px and up the page is one full width column: the band, then (own page only) the state line, then Recent reports at full width. Never a centred narrow column (03.4 R6 stays true).
- D3 (A) The band is one box: 1px var(--color-border), radius 12px like .person-report-box, padding var(--space-lg). Top part: on the left the letter disc, the username (the h1) and the name; on the right the Followers and Following counts on the first line and the action row under them, right aligned (own page: Edit in Settings; someone else's page: Follow or Following, the Notify bell, More with its menu, which must still open fully on screen and not be clipped by the box). A 1px var(--color-border) line across the band. Lower part: the stars group when present, then the tag chip with its sentence beside it (wrapping under itself, as drawn), then Member since and the home area on one line. On someone else's page the Your rating control (#person-rating) and #person-action-error also sit inside the band, in the lower part, after the details.
- D4 (A) Inside the band the counts read as counts, not boxes: they lose their border, keep the semibold number, stay real buttons that open the lists (aria-haspopup dialog, at least var(--touch-target-min) tall, hover fill var(--color-surface), visible focus ring). Edit in Settings stays the outlined .btn.
- D5 (A) The own page state line (#person-state-line, role status) sits directly under the band, outside it.
- D6 (A) Recent reports: the empty state (#person-reports-empty) and the error state (#person-reports-error) sit inside a box with the same border and 12px radius as the rows box, at the top of the section, centred text inside the box. The rows box with rows is unchanged. The not found, load failed and You blocked states are unchanged.
- D7 (A) DOM order stays the reading order so a screen reader and Tab order make sense; no CSS order tricks that make Tab jump around. Wrapper elements are allowed only if the phone does not change, proven by before and after screenshots at 320 and 390 (own page with no reports, own page with reports, someone else's page with stars and the Follow row) that are pixel identical.
- D8 (A) Tokens only, no new token, light and dark (prefers-color-scheme and the [data-theme] overrides the file already handles), forced colours keeps the borders, no hover only information, no horizontal scroll at 900, 1280 and 1440.
- D9 (B) When the signed in person is inside the rename gap (service.UsernameRenameGap, usernameWaitDays, service.UsernameWaitMessage(days)) the server renders that sentence into the page. On open the window shows the current username in a read only box (readonly plus aria-readonly, not typable), the wait sentence in the status line with the existing wait (warn) style, the existing help line, and ONE button labelled OK (btn--primary) in place of Save username. No Save username is shown in this state.
- D10 (B) OK closes the window the same way the X does (closeWindow('username', true)), and Enter in the form does the same.
- D11 (B) If the live check answers the wait state while the window is in its normal state (the change happened in another tab), switch to the same locked state with the server's sentence.
- D12 (B) Outside the wait nothing changes: the normal box, the live check and Save username as today. Step 4 of sign up (welcome.html.tmpl, welcome.js) is untouched. Same on phone and laptop. A page left open past the end of the wait may still show the locked state until reload; that is acceptable, the server still decides on save.
- D13 Tests: find every pin with git grep before changing anything; add a contract test pinning the band, a handler test for the wait sentence inside and outside the gap, and node tests for the locked window. Comments cite the quick id, for example "Quick 261108-fku (owner decision 2026-11-08, ...)". Docs change only where a living doc describes the old layout; .planning/phases/09-*/09-UI-SPEC.md is history and is not rewritten.

## Planner choices (record each in the SUMMARY)

- C1 (D7, the reading order). The DOM order of the header stays as it is today: identity, record (stars, tag), details (Member since, home area), counts, actions, then the rating and the action error. The orchestrator's brief names the order "identity, counts, actions, record, details", which is the laptop's visual order; making that the DOM order would change the phone (D1 and the byte identical screenshots), or need the CSS order property on the phone, which D7 forbids. So the phone keeps DOM order equal to what it shows, and on the laptop grid areas place the counts and the action row top right. Every focusable item of the header is a count, an action or the rating control, and they keep their DOM order, so Tab on the laptop runs Followers, Following, the action row, then the rating control: top right, then down into the lower part, never back up. A screen reader on the laptop reads the record and the details before the counts, as on the phone. Flag this in the SUMMARY as the one place the laptop's visual order differs from the reading order of plain text.
- C2 (D3, D7, the wrappers). Two new divs inside #person-header: #person-band (class person-band) wraps the identity, the record, the counts, the action row branches, the rating mount and #person-action-error; #person-record (class person-record) wraps the stars group, the tag group, Member since and the home area. The own page state line moves out of the IsSelf actions branch to a new IsSelf block after the band closes, still inside #person-header. The phone base gets exactly one new rule: .person-band and .person-record take display contents, so their children stay flex items of the header column and the phone is unchanged. This is the only rule outside the 900px block (a wrapper cannot be layout neutral without it); the byte identical screenshots prove it. Both wrappers are plain divs with no role, so display contents drops nothing from the accessibility tree. On your own page #person-action-error now comes before the state line in the DOM; it is hidden there and person.js only ever shows it after an action on someone else's page, so nothing visible moves.
- C3 (D3, the grid). On the laptop .person-band is a grid with columns minmax(0, 1fr) and auto and the areas "ident counts", "ident actions", "record record". The rating mount and the action error are not named in the areas: they take grid-column 1 / -1 and are auto placed after the record, so a page without them gets no empty rows and no extra gap. Target spacing: counts to the action row var(--space-sm); top part to the line var(--space-lg); line to the lower content var(--space-lg); between the lower rows var(--space-12); the rating and the error var(--space-md) under the details. The line is the record's top border run edge to edge (negative inline margins of var(--space-lg) with matching inline padding), as drawn. The record is a wrapping flex row: the stars group and the tag group take a full row each, Member since and the home area share one row with var(--space-lg) between them and wrap on a narrow window. The tag group turns into a row (align-items baseline, gap var(--space-12)) whose sentence takes the rest of the width with min-width 0, so it wraps under itself.
- C4 (D3, the menu). On the laptop the More menu hangs from the right edge of the action row (right 0, left auto), because the row is right aligned and a menu opening to the right from its left edge could leave the screen. The band never sets overflow hidden.
- C5 (D4, the counts). Inside the band the count buttons take border-color transparent, so their size and every neighbour stay put. The existing forced colours rule still paints their border in CanvasText; a button needs an edge in a forced colours theme, so this is accepted and named in the SUMMARY.
- C6 (D5, a dead rule). The laptop block's .person-header gap rule never applied, because the base rule .person-header:not([hidden]) outranks it. It goes. The space between the band and the state line is the base gap var(--space-md).
- C7 (D8, forced colours). Forced colours mode paints every border in a system colour, so the band, the line and the state box keep their edges. To follow the file's convention, the existing forced colours block also gets border-color CanvasText for .person-band and .person-reports__state and border-top-color CanvasText for .person-record; these set only colours, so they draw nothing where no border style is set (the phone).
- C8 (screenshots, the view database). cmd/seedprofiles is the repo's own made up profile seed (ravi.k is a public Reliable reporter with six raters, so stars show; voter.a only votes; everyone is <username>@example.com with one shared made up password the seed prints). It refuses any database whose name does not start with p9_. So the executor seeds a transient p9_fku_seed, copies it with createdb -T p9_fku_seed q_fku_view, and drops p9_fku_seed at once; the server runs on q_fku_view as the orchestrator set. cmd/seedprofiles is not edited.
- C9 (D9, a change made in this page). The owner's own path is change, close, open again, without a reload. The page then knows it is inside the gap, so the window is told the full gap sentence as well: data-wait-after-change on the window, always rendered, holding service.UsernameWaitAfterChangeMessage(), which is UsernameWaitMessage for the whole gap (7 days). After a 200 save that replaced an earlier name (the row's stored value at this opening was not empty and differs from the stored answer), settings.js keeps that sentence as the lock, so the next opening is locked. A first username saved from Not set starts no wait (SetAccountUsername keeps username_changed_at NULL on a first set), so it locks nothing. Flag this in the SUMMARY as the planner's reading of D9 for the no reload path.
- C10 (D9, D10, focus and Enter). A locked opening puts focus on OK, the one action. Enter on OK closes through its click. Enter anywhere else in the locked window (the read only input) is caught by the window keydown handler, which prevents the default and closes, so it works whether or not the browser submits a form from a read only field with its default button hidden; the form submit handler also closes while locked. Escape and the X close as today.
- C11 (D9, D11, the live check). username-check.js gets two small additions that step 4 never reaches: judge and onPaste do nothing while the input is read only (read through getAttribute('readonly') !== null, which both node fakes and real browsers answer), and the controller gets hold(message), which drops anything due (seq plus one, cancelPending) and shows the wait look with the given sentence, so isGood() is false and state() is wait. The status line's classes stay owned by username-check.js.
- C12 (D9, older test pages). The OK button is optional in settings.js: hasUsername keeps its current list, and when OK is missing a lock still makes the field read only, writes the sentence and hides Save. So the hand built pages in settings_window_test.mjs, settings_alerts_test.mjs and settings_people_test.mjs keep working.
- C13 (D11, a refused save). A save refused with the wait sentence (400, field username) stays the refusal line, as today: the sentence alone does not say which refusal it is, and the next typed check answers wait and locks. The existing node test for a too soon save stays as it is.

## Facts the planner already established (do not re-derive)

- person.html.tmpl today: #person-header holds .person-ident, #person-stars (when StarsNumber), .person-tag-group, #person-member-since, #person-home-area (when set), .person-counts, then the IsSelf branch (#person-actions with #person-edit, then #person-state-line) or the other branch (#person-actions with Follow, Following, the bell, More and #person-more-menu; then #person-rating when MayRate), then #person-action-error for both. The page is .person-page, a flex column on the phone and a two column grid (a fixed header column and the reports) in the laptop block.
- web/person_contract_test.go TestPersonPageHeaderOrder pins today's id order with #person-state-line right after #person-edit and #person-action-error after #person-more-menu. TestPersonBellMarkup finds the two branches with strings.Index on "{{if .Profile.IsSelf}}" plus a newline, six spaces and the opening of div id person-actions, and on "{{else}}" plus a newline, six spaces and the Follow and Following template comment. Keep every existing line of the header at its current indentation (add the wrapper lines without re-indenting the lines between them), so those two pins and the diff stay small.
- person.js around lines 282 to 286 builds a missing rating host after a refetch with els.header.insertBefore(host, els.error). Once the error line lives in the band, the header is not its parent and that call throws. The stars group is inserted with tagGroup.parentNode.insertBefore, which follows the record wrapper by itself. person.js only shows #person-action-error after an action on someone else's page. The node fake in web/person_test.mjs (load, around lines 100 to 135) appends the error line straight to the header; tests around lines 502 and 574 count the header's children.
- The base rule .person-header:not([hidden]) (specificity two classes) outranks the laptop .person-header gap rule, which has never applied.
- .person-menu:not([hidden]) is absolute at top 100% and left 0 of .person-actions, which is position relative.
- main.css .empty-state and .error-state are a centred flex column with padding var(--space-2xl) var(--space-lg); person.css .person-reports__state overrides the padding to var(--space-xl) var(--space-lg) 0 and loads later.
- The global focus ring is main.css :focus-visible, 2px solid var(--color-focus-ring), offset 2px.
- chrome.css .btn[hidden] keeps a hidden .btn hidden; the Settings page loads chrome.css and window.css, and no window.css rule sets a display on .app-window__actions .btn.
- In web/review34_part3_contract_test.go, p26Find and p26Has treat media "" as any media. A base only check must filter the parsed rules (p26Rules, type p16CSSRule) on Media == "". p16CSSRule has Media, Head and Decls (property to value); the "order" property must be matched by its property name, never by substring, because "border:" contains it.
- web/layout_rules_contract_test.go TestSizeOnlyElementsAreNamed looks only for display none inside a size block; display contents and grid do not trip it. Its touch target list names .person-count and .person-bell; both keep min-height var(--touch-target-min).
- No living doc describes the profile's column layout: docs/*.md and README.md were checked (TRUST-MODEL.md around line 714 lists what the page shows, not where). The only prose that names the old laptop grid is the person.css header comment and the comment at the top of its laptop block; both are rewritten. docs/MODERATION.md line 274 "A name can be changed once every 7 days." stays true.
- internal/service: usernameWaitDays(changedAt pgtype.Timestamptz, now time.Time) (int, bool) is unexported in profile.go; AccountProfile.UsernameChangedAt is *time.Time and Username is *string; AccountService has the clock s.now (WithAccountClock in tests); NewAccountService never dereferences its querier, so a method that does not read the store can be tested with NewAccountService(nil, WithAccountClock(...)).
- SetAccountUsername (internal/store/queries/accounts.sql) keeps username_changed_at on a first set (username IS NULL) and on the same name, and stamps it on a real change. CheckUsername answers yours for the stored name even inside the gap, which is why a fresh opening today shows "This is your username now." with Save off, and only a typed name gets the wait sentence (the owner's report).
- settings.js: openWindow('username') fills the field from the row's data-value, attaches the live check on the first opening (attach judges at once) or calls refresh on later ones, then focuses and selects the field. closeWindow(which, restoreFocus) returns focus to the row. onWindowKeydown handles Escape and the Tab trap. The submit handler checks aria-disabled on Save first. saveUsername closes the window on a same window 200.
- web/settings_username_test.mjs buildPage builds the window by hand (FakeEl has getAttribute, setAttribute, hasAttribute, removeAttribute and the hidden property; fire(el, type, extra) runs handlers with a preventDefault that records prevented). web/username_check_test.mjs FakeInput has only value, handlers, getAttribute and setAttribute.
- web/settings_rows_contract_test.go TestSettingsUsernameRow pins the window's opening tag ending in id settings-username-window then hidden and the closing angle bracket, the id list of the window, ">Save username</button>", and a comment and check saying the X is the only way to close (it bans the words cancel and done and counts one control named Close; OK is neither).
- e2e: internal/api/handlers has newE2EServer (returns the pool), newVerifiedClient, getPlacePage, meSend and meWant; places_e2e_test.go shows a username PATCH and a GET of /settings. SettingsPage carries no swag annotation, so no swagger regeneration is expected; if any annotated handler's doc comment changes after all, run make swag and keep the router line "// @license.name Unlicensed (all rights reserved)".
- Playwright: NODE_PATH works only for require, not for an ES module import, so the screenshot scripts are .cjs files that call require('playwright'). The repo's binary embeds web assets at build time, so a binary built before an edit serves the old markup and CSS.

## Executor rules (binding, carried from the orchestrator)

- Tests only on a fresh scratch database the executor creates itself, named q_fku_a (never pinalert_test, pinalert, p31_4_uat, p34_uat, p9_view, p3_view, p4_uat). In the same Bash call, confirm DATABASE_URL and PGDATABASE are empty, then set DATABASE_URL for q_fku_a. Full go test ./... -p 1 -count=1 -v plus node --test web/*.mjs; only TestGeocodeLive, TestOfficialLive and TestScreeningFixture may skip. Drop q_fku_a at the end.
- Screenshots: Playwright WebKit with NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules, against the executor's own server on port 8794 with its own database q_fku_view, seeded with made up example.com people only (C8); never a real person's details. Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797, and never touch the processes on them. Screenshots and every helper file (scripts, logs, binaries, storage states) go only in /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/fku-exec/ (called S below) and never in the repo; no images in the repo at all. Never write to any other file in the scratchpad (gate-p27.sh, uat/ and shot/ belong to the orchestrator).
- No Python in any form, not even --version. No em or en dashes in prose, copy, comments or commits; check every touched file with grep -n -e $'\xe2\x80\x94' -e $'\xe2\x80\x93' <files>, never by pasting the characters. No migration.
- Path limited commits: git add -- <paths>, then git commit -m "<subject>" -m "<body>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- <paths>. Subjects like fix(261108-fku): .... Never stage .planning/STATE.md, ROADMAP.md or REQUIREMENTS.md (another session's STATE.md edit and several untracked folders are already in the tree; leave them alone). Never push. Never rewrite history.
- The wording check must print nothing: git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder
- Keep the swag @license.name line "Unlicensed (all rights reserved)".
- Use the Edit tool for edits and the Write tool for new files. gofmt -w only the Go files this plan touches. Keep comment density and naming like the surrounding code; every new or changed comment cites "Quick 261108-fku (owner decision 2026-11-08, ...)". An existing test that changes gets a line "Amended 2026-11-08 by quick 261108-fku (owner decision 2026-11-08): ..." above the change. Fix code, never weaken or skip a test.
- Before starting a server, check the port is free with lsof -nP -iTCP:8794 -sTCP:LISTEN; if anything listens there, stop and report instead of killing it. Start servers with run_in_background, log to S, and stop only the PIDs you started. If the sandbox refuses env -i, confirm DATABASE_URL, PGDATABASE, RESEND_API_KEY and OPENAI_API_KEY are empty and set the server's variables inline instead.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task A: the profile band across the top on the laptop (D1 to D8, D13)</name>
  <files>web/templates/person.html.tmpl, web/static/css/person.css, web/static/js/person.js, web/person_band_contract_test.go, web/person_contract_test.go, web/person_test.mjs, internal/api/handlers/person_page_e2e_test.go</files>
  <behavior>
    - TestPersonProfileBand (web/person_band_contract_test.go, new, package web, header comment citing quick 261108-fku and the owner decision of 2026-11-08). Template: #person-band and #person-record exist once each, inside #person-header; the order is person-band, person-disc, the h1, person-name, person-record, person-stars, person-tag, person-tag-sentence, person-member-since, person-home-area, the two count buttons, person-actions, person-edit, person-follow, person-unfollow, person-bell, person-more, person-more-menu, person-rating, person-action-error, person-state-line, the closing header tag. CSS, laptop (rules whose Media contains min-width: 900px): .person-page declares no grid-template-columns and no display grid; .person-band declares display grid, border 1px solid var(--color-border), border-radius 12px, padding var(--space-lg), and grid-template-areas holding "ident counts", "ident actions" and "record record"; a rule for the counts inside the band sets border-color transparent; .person-record declares border-top 1px solid var(--color-border); .person-tag-group declares flex-direction row; .person-reports__state declares border 1px solid var(--color-border) and border-radius 12px; the band's menu rule declares right 0; no laptop rule for .person-band declares overflow. CSS, phone base (Media == ""): the only base rules naming .person-band or .person-record declare exactly display contents and nothing else; .person-page keeps display flex, flex-direction column, gap var(--space-lg), padding var(--space-md); .person-header:not([hidden]) keeps display flex, flex-direction column, gap var(--space-md); .person-count keeps border 1px solid var(--color-border), border-radius 8px, background var(--color-bg), min-height var(--touch-target-min); .person-tag-group keeps flex-direction column; .person-menu:not([hidden]) keeps left 0; .person-reports__state keeps padding var(--space-xl) var(--space-lg) 0; no base rule declares grid-template-areas. Whole file: no declaration whose property is order; the new band rules hold no hex colour and define no custom property.
    - TestPersonPageBand (internal/api/handlers/person_page_e2e_test.go, new, built on p907Server, p907Person and p907Reporter like TestPersonPage). On someone else's page and on your own page, the element opened by id person-band, cut out by counting nested div tags from its opening tag, holds person-disc, the username h1, person-record, person-followers-button, person-following-button, person-actions and person-action-error; on someone else's page also person-follow and person-more-menu; on your own page also person-edit, and person-state-line appears after the band's end and before the closing header tag, never inside the band.
    - web/person_test.mjs: the load fake puts the action error line inside a band node that sits in the header, as the page now draws it; the test that builds the rating box after a refetch asserts the host lands in the band right before the error line; the test that expects no rating box counts the band's children instead of the header's. Every other person node test stays green unchanged.
    - Phone: before and after full page screenshots are byte identical (the compare file lists every pair as SAME).
    - Laptop probes, in WebKit at 900, 1280 and 1440: no horizontal scroll; the band's computed border is 1px with a 12px radius; the count buttons' border colour is transparent and Edit in Settings' is not; each count is at least 44px tall and shows the focus ring when reached by Tab; the More menu, once open, lies inside the viewport and every item is the element at its own centre point; the empty state box has a 1px border and a 12px radius; Tab runs Followers, Following, the action row, then the rating control, each one at or below the previous on the page.
  </behavior>
  <action>
Step A0, set up before any edit. In one Bash call confirm DATABASE_URL and PGDATABASE are empty. mkdir -p S (the scratch subfolder named in the rules). If a database q_fku_a is left from this task, dropdb it; createdb q_fku_a. Check port 8794 is free. The before binary must be exactly HEAD: git status --porcelain -- web internal cmd go.mod go.sum must print nothing (another session shares the repo). If it prints nothing, build it from the tree with go build -o S/server-before ./cmd/server. If it prints anything, do not touch those files: add a worktree of HEAD with git worktree add --detach S/head-tree HEAD, build the before binary there into S/server-before, and remove the worktree with git worktree remove S/head-tree at the teardown. Then read every pin: git grep -n -l 'person-header\|person-band\|person-record\|person\.css\|person-reports__state\|person-state-line\|person-action-error' -- '*_test.go' '*.mjs', and read the parts of each listed test that name the header, the state line, the action error or the laptop block.

Step A1, RED. Write web/person_band_contract_test.go with TestPersonProfileBand as the behavior says (reuse p421ReadTemplate, p421InOrder, p26Rules and p16HeadParts; add a small local helper that keeps only rules whose Media is empty for the base checks, and one that keeps rules whose Media contains min-width: 900px). Add TestPersonPageBand to internal/api/handlers/person_page_e2e_test.go. Amend web/person_test.mjs as the behavior says. Run go test -count=1 ./web/ -run TestPersonProfileBand, the database form for ./internal/api/handlers/ -run TestPersonPageBand, and node --test web/person_test.mjs, logging to S; they must fail.

Step A2, GREEN, the template (D3, D5, C1, C2). In web/templates/person.html.tmpl, inside #person-header, add the opening of div id person-band class person-band right after the header's opening tag, with a template comment citing Quick 261108-fku (owner decision 2026-11-08): the band across the top on the laptop; on the phone it and the record wrapper take display contents, so the column is unchanged. Wrap the stars block, the tag group, Member since and the home area block in div id person-record class person-record. Leave the counts, the two action branches and the rating mount where they are. Remove the state line from the IsSelf branch, keep #person-action-error where it is, close the band right after it, then add a new IsSelf block holding the unchanged state line paragraph, still inside the header. Do not re-indent the existing lines (see the facts). Nothing else in the template changes: no id, class, text, attribute or branch of the not found, load failed, You blocked, confirm or reports markup.

Step A3, GREEN, the stylesheet (D1 to D8, C2 to C7). In web/static/css/person.css: rewrite the file header paragraph that describes the laptop grid so it describes the band (Quick 261108-fku, owner decision 2026-11-08, drawn by the owner). In the phone base section, after the header rule, add the one rule giving .person-band and .person-record display contents, with a comment saying why it is the only base change and that the phone screenshots before and after are byte identical. In the min-width 900px block: rewrite its opening comment; replace the .person-page grid declarations with nothing that changes the base flex column (keep padding var(--space-lg)); delete the dead .person-header gap rule (C6) and the .person-state grid-column rule, which has no grid left to span; add .person-band (display grid, the columns and areas of C3, the row gap of C3, border 1px solid var(--color-border), border-radius 12px, padding var(--space-lg), background var(--color-bg), no overflow), the placement of .person-ident (area ident, align-self center), .person-counts (area counts, justify-self end, justify-content flex-end), .person-actions (area actions, justify-self end, justify-content flex-end), .person-record (area record, display flex, flex-wrap wrap, the gaps of C3, the edge to edge top line of C3), the full rows for the stars group and the tag group inside the record, the tag group row of C3, Member since and the home area sharing a row, the rating mount and the band's error line on grid-column 1 / -1 with the margin of C3, the counts inside the band with border-color transparent (C5), the band's menu with right 0 and left auto (C4), and .person-reports__state with border 1px solid var(--color-border), border-radius 12px and padding var(--space-xl) var(--space-lg). Keep every guarded :not([hidden]) display rule guarded: a new laptop rule that sets display on something the page hides with the hidden attribute must carry the same guard. In the forced colours block add the colour only rules of C7. Tokens only, no hex, no new custom property, no use of the CSS order property, no animation.

Step A4, GREEN, the script. In web/static/js/person.js, where the rating host is built after a refetch, test els.error and its parentNode instead of els.header and insert with els.error.parentNode.insertBefore(host, els.error), with a comment citing Quick 261108-fku (the error line now sits in the band). Nothing else in person.js changes.

Step A5, the other pins. Amend TestPersonPageHeaderOrder in web/person_contract_test.go to the new order (the band and record wrappers in their places, the state line after the action error) with the dated Amended line. Run go test -count=1 ./web/, node --test web/person_test.mjs web/person_bell_test.mjs web/person_mute_test.mjs web/people_list_test.mjs, and the database form for ./internal/api/handlers/ -run 'TestPerson|TestPeople|TestSettingsPeople|TestRated', logging to S. Any other test that fails only because it pinned the moved state line or the old laptop grid is amended the same way, added to this commit and named in the SUMMARY.

Step A6, screenshots and laptop probes (D1, D7, D8, C8). Build the after binary with go build -o S/server-after ./cmd/server. Seed the view database: createdb p9_fku_seed; migrate it with go run ./cmd/migrate (DATABASE_URL inline for p9_fku_seed); go run ./cmd/seedprofiles -db for p9_fku_seed with output to S/seed.log (it prints the shared made up password); createdb -T p9_fku_seed q_fku_view; dropdb p9_fku_seed. As voter.a, block anita.r once (through the profile page's More menu or the block address the page uses), so the You blocked page exists for both runs. Write S/fku-shots-a.cjs (require('playwright'), webkit, deviceScaleFactor 1, reducedMotion reduce, a fresh context per account): it signs in through the app's own sign in page as voter.a@example.com and as ravi.k@example.com with the seeded password (save each storage state in S and reuse it), and takes full page shots into S/shots/ named by run, width and page. The two runs follow the same rules, fixed before the first shot and never changed between runs: a fresh context from the saved storage state for each account; one warm-up load of each page before the load that is shot; then a full page shot with Playwright's mask option over the top chrome (.top-chrome) and the bottom bar (the element the app_bottombar partial draws; read its class in the template), because the bell count up there moves with the bell poll (since quick 261103-jal every account with a home area gets Reliable bell rows, and the seed has live Reliable reports) and this task does not touch that chrome. The masks cover the page's own content nowhere, so the proof still covers every pixel this task can change; the horizontal scroll probe below still covers the page width. Phone pages at widths 320 and 390: voter.a at /me (your own page with no reports), ravi.k at /me (your own page with reports), voter.a at /u/ravi.k (someone else's page with stars and the action row), and voter.a at /u/nobody.fku (not found). Laptop pages at 1280: voter.a at /u/nobody.fku and voter.a at /u/anita.r (You blocked). Run the before binary on port 8794 against q_fku_view with ENV=development, PORT=8794 and BASE_URL http://127.0.0.1:8794, take the before set, stop it; run the after binary the same way at once, take the after set, stop it. Run the two sets back to back, so a relative time in the report rows cannot move between them. Compare each pair with Buffer.equals and write one line per pair to S/compare.txt: SAME then the pair name when the bytes match, or another word with the pair name when they do not. All 10 pairs must read SAME. If a pair differs, find the cause by element shots of the band and the reports, fix the CSS or the markup (never the comparison or its fixed rules), rebuild the after binary and run both sets again back to back. If the only difference turns out to be a relative time or another value that moved with the clock inside the page content, run both sets once more back to back and say so in the SUMMARY. Then, with the after binary only, write S/fku-probes-a.cjs for the laptop probes of the behavior list at 900, 1280 and 1440, in the light scheme, the dark scheme, and with data-theme set to dark on the light scheme and to light on the dark scheme (set on the html element), on voter.a /me, ravi.k /me and voter.a /u/ravi.k (open More for the menu probe), plus the no horizontal scroll probe at 320 and 390. It throws on the first probe that does not hold and writes the line ALL PROBES PASSED to S/probes-a.txt only when every probe held. Also save laptop shots at 1280 in light and dark of the three pages and of the open menu into S/shots/ for the orchestrator. Forced colours cannot be emulated in WebKit; the C7 rules are pinned by the contract test only, say so in the SUMMARY. Stop the server.

Step A7, commit. gofmt -w the touched Go files, go build ./..., go vet ./..., the dash check over every file of this task, then commit "fix(261108-fku): the profile header is one band across the top on the laptop" with a body naming D1 to D8, C1 to C8 and the byte identical phone screenshots, and the trailer, path limited to this task's files.
  </action>
  <verify>
    <automated>test -z "$DATABASE_URL" && test -z "$PGDATABASE" && S=/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/fku-exec && { DATABASE_URL='postgres://localhost:5432/q_fku_a?sslmode=disable' go test -count=1 -p 1 ./web/ ./internal/api/handlers/ -v > "$S/verify-a.log" 2>&1; true; } && test "$(grep -cE '^(--- FAIL|FAIL|panic)' "$S/verify-a.log")" -eq 0 && test "$(grep -cE -- '--- PASS: (TestPersonProfileBand|TestPersonPageBand|TestPersonPageHeaderOrder|TestPersonBellMarkup)( |$)' "$S/verify-a.log")" -eq 4 && node --test web/person_test.mjs web/person_bell_test.mjs web/person_mute_test.mjs web/people_list_test.mjs > "$S/node-a.log" 2>&1 && test "$(grep -c '^SAME ' "$S/compare.txt")" -eq 10 && test "$(wc -l < "$S/compare.txt" | tr -d ' ')" -eq 10 && grep -qx 'ALL PROBES PASSED' "$S/probes-a.txt" && echo TASKA-OK</automated>
  </verify>
  <done>The command prints TASKA-OK: the web and handlers packages pass on q_fku_a with the four named tests passing at top level, the person node tests pass, all 10 before and after screenshot pairs are byte identical, and every laptop probe held. go build and go vet are clean. One path limited commit with the trailer; no image in the repo.</done>
</task>

<task type="auto" tdd="true">
  <name>Task B: the Username window says OK during the 7 day wait (D9 to D13), then the full gate</name>
  <files>internal/service/username.go, internal/service/account_username.go, internal/service/account_username_test.go, internal/api/handlers/settings.go, internal/api/handlers/settings_username_e2e_test.go, web/templates/settings.html.tmpl, web/static/js/settings.js, web/static/js/username-check.js, web/settings_username_test.mjs, web/username_check_test.mjs, web/settings_rows_contract_test.go</files>
  <behavior>
    - TestUsernameWaitLine (internal/service/account_username_test.go, no database: NewAccountService(nil, WithAccountClock(fixed now))). A username changed 1 hour ago answers "You can change your username again in 7 days."; changed 6 days and 1 hour ago answers "You can change your username again in 1 day."; changed exactly 7 days ago and 8 days ago answer ""; no change time answers ""; no username answers "" even with a change time. UsernameWaitAfterChangeMessage() is exactly "You can change your username again in 7 days.".
    - TestSettingsUsernameWaitLine (internal/api/handlers/settings_username_e2e_test.go, new). A verified client saves a first username: GET /settings holds no data-wait-message attribute and holds data-wait-after-change with the 7 days sentence. It saves a different username: GET /settings holds data-wait-message with the 7 days sentence on #settings-username-window. With username_changed_at set by SQL to now minus 6 days and 1 hour it holds the 1 day sentence; with now minus 7 days and 1 minute it holds no data-wait-message attribute again.
    - web/settings_username_test.mjs (buildPage takes waitMessage and waitAfterChange and builds the OK button). Inside the wait the window opens locked: the field holds the stored name with readonly and aria-readonly true, the status line holds the server sentence with the username-status--warn class, Save username is hidden, OK is shown with btn--primary, focus is on OK, and no live check is sent even after the pause. OK closes the window like the X (hidden, focus on the Username row) and sends nothing. Enter in the locked window (keydown on the window with the field as target) is prevented and closes it; a submit while locked closes it; Escape closes it. An input event on the read only field sends no check and leaves the sentence. In the normal state a typed name answered wait locks the window with the answer's sentence and puts the stored name back in the field. After a 200 save that replaced an earlier name the next opening is locked with the after change sentence and sends no check; a first name saved from Not set leaves the next opening normal. With no wait attribute the window opens exactly as before (field editable, Save username shown, OK hidden, a check sent). Every existing test in the file stays green.
    - web/username_check_test.mjs: hold(message) drops a check that is due (no fetch after the pause), shows the sentence with the warn class, and leaves isGood() false and state() wait. A read only input (readonly attribute set before attach) is not judged at attach and an input event on it sends nothing. Every existing test stays green.
    - TestSettingsUsernameRow (web/settings_rows_contract_test.go, amended): the window's opening tag carries the conditional data-wait-message and the always present data-wait-after-change after hidden; the OK button is pinned as a type button, class btn btn--primary, id settings-username-ok, hidden, reading OK, once, after Save username in the actions; the closing comment and check say the X is the only way to close outside the wait and that OK closes inside it.
  </behavior>
  <action>
Step B0. git grep -n -l 'settings-username\|UsernameWait\|username-status--warn\|PinalertUsernameCheck' -- '*_test.go' '*.mjs' and read what each listed test pins about the Username window and the live check.

Step B1, RED. Write TestUsernameWaitLine in internal/service/account_username_test.go, TestSettingsUsernameWaitLine in the new internal/api/handlers/settings_username_e2e_test.go (header comment citing quick 261108-fku and the owner decision of 2026-11-08), the node tests of the behavior list in web/settings_username_test.mjs and web/username_check_test.mjs, and the TestSettingsUsernameRow amendment. Run them (the database form for the handler test) logging to S; they must fail.

Step B2, GREEN, the service (D9, C9). In internal/service/username.go add UsernameWaitAfterChangeMessage, the wait sentence for the whole UsernameRenameGap in days, with a doc comment: the sentence the Settings page shows on the next opening after a change saved in the page. In internal/service/account_username.go add (s *AccountService) UsernameWaitLine(p AccountProfile) string: empty when p.Username or p.UsernameChangedAt is nil; otherwise usernameWaitDays on the change time and s.now().UTC(), and UsernameWaitMessage(days) while the wait holds, else empty. Doc comment: the caller's own wait sentence for the Settings page, read on the service clock, no store read; the save path still decides (saveUsername). No other service code changes.

Step B3, GREEN, the handler. In internal/api/handlers/settings.go add two settingsViewModel fields with comments citing Quick 261108-fku (owner decision 2026-11-08): UsernameWait, the server's wait sentence while the account is inside the rename gap, empty outside it, and UsernameWaitAfterChange, the full gap sentence. Fill them in SettingsPage with accounts.UsernameWaitLine(profile) and service.UsernameWaitAfterChangeMessage(). No new read, no change to the error paths or the cache header.

Step B4, GREEN, the template (D9). In web/templates/settings.html.tmpl add, after hidden on #settings-username-window, the attribute data-wait-message holding UsernameWait only when it is not empty, and data-wait-after-change holding UsernameWaitAfterChange. In the window's actions, after Save username, add the OK button of the behavior list. Update the template comment above the window: inside the wait the window opens read only with the server's sentence and OK (Quick 261108-fku, owner decision 2026-11-08). Nothing else in the window changes: the label, the field, the status line, the help line and the error line stay word for word.

Step B5, GREEN, the live check (C11). In web/static/js/username-check.js add the read only guard to judge and onPaste (return at once, onPaste without preventDefault) and the controller method hold(message) of C11; add both to the header's controller list (hold: shows a wait sentence the page already has and drops anything due; the input of a read only field is never judged). Step 4 behaves exactly as before.

Step B6, GREEN, the window (D9 to D12, C9 to C13). In web/static/js/settings.js: read the OK button as an optional element (C12); read data-wait-message once at load into the lock sentence and data-wait-after-change into the after change sentence. Add lockUsername(sentence): store the sentence, put the row's stored value in the field, set readonly and aria-readonly true on it before the check is attached, attach the live check if it is not yet attached, call its hold with the sentence (when the check script is missing, write the sentence with textContent), hide Save username, show OK, hide the error line, and move focus to OK (or to the field when there is no OK). In openWindow('username'): when a lock sentence is set, call lockUsername instead of refreshing or attaching the check and skip the select; otherwise everything is as today. Wrap the live check's onChange: a wait state while no lock is set and the window is open calls lockUsername with the status line's text, the server's sentence (D11); every state still runs updateUsernameSave. In saveUsername's same window 200 branch, when the value the window was filled with at this opening is not empty and differs from the stored answer, set the lock sentence to the after change sentence before closing (C9); in the earlier opening branch, when the window is open again, lock it the same way. Wire OK's click to closeWindow('username', true); in the submit handler close while locked before any other check; in onWindowKeydown, while locked, Enter prevents the default and closes (C10). Update the comments that say the X and Escape are the only ways out (closeWindow's comment, the Username section comment) to name OK and Enter inside the wait, citing Quick 261108-fku. The Name and Home area windows do not change.

Step B7, tests and the browser check. Run go build ./..., go vet ./..., node --test web/*.mjs, go test -count=1 ./web/ and the database form for ./internal/service/ ./internal/api/handlers/ logging to S. Any other test that fails only because it pinned the window's opening tag or the X only comment is amended with the dated line, added to this commit and named in the SUMMARY. Then build S/server-b, run it on port 8794 against q_fku_view (ENV=development, PORT=8794, BASE_URL http://127.0.0.1:8794), and write S/fku-probes-b.cjs (require('playwright'), webkit), at widths 390 and 1280, signed in as voter.a: Settings, the Username row opens the normal window (editable, Save username shown, OK not displayed); a change to a new made up name (for example voter.fku) is saved; the next opening is locked (the field's readOnly is true and a typed key leaves its value, Save username's computed display is none, OK is shown and focused, the status line holds the 7 days sentence); Enter closes it and focus returns to the row; it reopens locked and OK closes it; after a reload it still opens locked (the server sentence); then by psql on q_fku_view set voter.a's username_changed_at to now minus 8 days, reload, and the window opens normal again. It writes ALL PROBES PASSED to S/probes-b.txt only when every probe held, and saves a shot of the locked window at both widths into S/shots/. Stop the server.

Step B8, commit. gofmt -w the touched Go files, the dash check over this task's files, then commit "fix(261108-fku): the Username window says OK during the 7 day wait" with a body naming D9 to D12, C9 to C13 and the trailer, path limited to this task's files.

Step B9, full gate, each logged to S: go build ./... ; go vet ./... ; in one Bash call with both variables confirmed empty, DATABASE_URL for q_fku_a with go test ./... -p 1 -count=1 -v (every skip line belongs to TestGeocodeLive, TestOfficialLive or TestScreeningFixture) ; node --test web/*.mjs ; the wording grep from the rules (prints nothing) ; the dash check over every file this plan touched (prints nothing) ; grep -c 'Unlicensed (all rights reserved)' internal/api/router.go is 1 ; git log cbe48b06..HEAD -- web/templates/welcome.html.tmpl web/static/js/welcome.js prints nothing ; git status shows nothing of this plan unstaged. Then tear down: dropdb q_fku_a and q_fku_view (and p9_fku_seed if it is somehow still there), make sure nothing listens on 8794 that you started, and delete from S the logs, seed.log, the storage states and the binaries (and remove S/head-tree with git worktree remove if Step A0 made it); keep S/shots/, S/compare.txt and the probe files for the orchestrator.

Step B10. Write .planning/quick/261108-fku-profile-band-across-the-top-on-laptop-an/261108-fku-SUMMARY.md (frontmatter like the other quick SUMMARYs): what changed per task with commit shas; D1 to D13 and how each was met; C1 to C13 as decisions, with C1 (the laptop's visual order versus the reading order of plain text), C5 (the forced colours border on the counts) and C9 (the lock after a change saved in the page) flagged for the owner; the screenshot result (10 pairs byte identical), the probe results and the absolute paths of the shots; that forced colours was pinned by the contract test only; that a page left open past the end of the wait shows the locked state until reload (D12); every test added or amended; the gate results. Commit it alone, last, with "docs(261108-fku): summary" and the trailer, path limited to that file.
  </action>
  <verify>
    <automated>test -z "$DATABASE_URL" && test -z "$PGDATABASE" && S=/private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/fku-exec && DATABASE_URL='postgres://localhost:5432/q_fku_a?sslmode=disable' go test ./... -p 1 -count=1 -v > "$S/full.log" 2>&1; test "$(grep -cE '^(--- FAIL|FAIL|panic)' "$S/full.log")" -eq 0 && test "$(grep -E -- '--- SKIP' "$S/full.log" | grep -vcE 'TestGeocodeLive|TestOfficialLive|TestScreeningFixture')" -eq 0 && test "$(grep -cE -- '--- PASS: (TestUsernameWaitLine|TestSettingsUsernameWaitLine|TestSettingsUsernameRow|TestPersonProfileBand|TestPersonPageBand)( |$)' "$S/full.log")" -eq 5 && node --test web/*.mjs > "$S/node-full.log" 2>&1 && grep -qx 'ALL PROBES PASSED' "$S/probes-b.txt" && test -z "$(git log cbe48b06..HEAD -- web/templates/welcome.html.tmpl web/static/js/welcome.js)" && test "$(git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder | wc -l | tr -d ' ')" -eq 0 && echo TASKB-OK</automated>
  </verify>
  <done>The command prints TASKB-OK (run it before the teardown drops q_fku_a): the whole suite passes on q_fku_a with no skip outside the three allowed tests and the five named tests passing at top level, every node test passes, the browser probes of the locked window held, step 4's files are untouched since cbe48b06, and the wording check is empty. Then q_fku_a and q_fku_view are dropped, the server is stopped, the helper files are gone from S apart from the shots, the compare file and the probe files, and the SUMMARY is committed alone and last. Nothing pushed.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| server to the Settings page | the caller's own rename wait state is rendered into the page |
| browser to PATCH /api/me/profile | an untrusted username change; the locked window is a page state only |
| server to the profile page | the same escaped profile values, rearranged by markup and CSS only |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-fku-01 | Tampering | the locked Username window | low | mitigate | The lock is a page state only. saveUsername still refuses a change inside UsernameRenameGap before any screening call and SetAccountUsername enforces it in SQL; TestUsernameSaveRenameGap stays as it is. A page left open past the end of the wait only shows the lock until reload (D12). |
| T-fku-02 | Information disclosure | data-wait-message and data-wait-after-change | low | mitigate | Only the caller's own sentence with a day count is rendered; the change time itself never enters the page, and nothing about another account does. Both attributes go through html/template escaping. |
| T-fku-03 | Spoofing | the status line and the profile band | low | mitigate | Every sentence reaches the page through html/template or textContent, never a markup sink; the existing node tests that fail on innerHTML and outerHTML stay in force. |
| T-fku-04 | Denial of service | the live username check | low | accept | A locked window sends no check at all, so the per account check allowance is spent less, never more. |
| T-fku-05 | Information disclosure | the screenshot database | low | mitigate | q_fku_view is filled only by cmd/seedprofiles with made up example.com people; it lives on this machine, is never the owner's or another session's database, and is dropped at the end; storage states and logs are deleted. |
</threat_model>

<verification>
- go build ./... and go vet ./... are clean.
- go test ./... -p 1 -count=1 -v on q_fku_a passes; only TestGeocodeLive, TestOfficialLive and TestScreeningFixture skip.
- node --test web/*.mjs passes.
- The 10 before and after screenshot pairs (phone 320 and 390 for your own page with no reports, your own page with reports, someone else's page with stars and the action row, the not found page; laptop 1280 not found and You blocked), taken back to back with the top chrome and the bottom bar masked the same way in both runs, are byte identical, proving the phone and the unchanged states did not move.
- The laptop probes held at 900, 1280 and 1440 in light, dark and both data-theme overrides; the locked window probes held at 390 and 1280.
- Named passing tests: TestPersonProfileBand, TestPersonPageBand, TestPersonPageHeaderOrder, TestPersonBellMarkup, TestUsernameWaitLine, TestSettingsUsernameWaitLine, TestSettingsUsernameRow.
- The wording check prints nothing; no long dash in any touched file; the swag license line is unchanged; no migration; no image in the repo.
</verification>

<success_criteria>
On the laptop the owner's profile reads as one band across the top, as drawn: who the person is on the left, the counts as plain counts with the one outlined button under them on the right, a line, then the tag, its sentence and the details, with the state line and a boxed Recent reports under it at full width; the phone is byte for byte what it was. In Settings, during the 7 day wait, the Username window shows the name read only, the server's sentence and one OK button that closes it, on phone and laptop; outside the wait nothing changed. The full suite is green, nothing is pushed.
</success_criteria>

<output>
Create `.planning/quick/261108-fku-profile-band-across-the-top-on-laptop-an/261108-fku-SUMMARY.md` when done, committed alone and last.
</output>

## Source coverage audit

| Source | Item | Covered by |
|--------|------|-----------|
| GOAL | Owner 2026-11-08: the laptop profile header looks misplaced, with no difference between information and buttons | Task A |
| GOAL | Owner 2026-11-08: during the rename wait the window has only Save username and the X | Task B |
| CONTEXT | D1 laptop only, phone unchanged | Task A steps A3 and A6 (C2, byte identical shots) |
| CONTEXT | D2 one full width column | Task A step A3 |
| CONTEXT | D3 the band, its two parts, the line, the rating and error inside, the menu on screen | Task A steps A2, A3, A4, A6 (C3, C4) |
| CONTEXT | D4 counts without a border, still buttons; Edit outlined | Task A step A3 (C5), probes |
| CONTEXT | D5 the state line under the band, outside it | Task A step A2 |
| CONTEXT | D6 the boxed empty and error states; rows box and state pages unchanged | Task A step A3, shots |
| CONTEXT | D7 reading order, no order tricks, wrappers proven neutral | Task A (C1, C2), contract test, shots |
| CONTEXT | D8 tokens, themes, forced colours, no hover only information, no horizontal scroll | Task A step A3 (C7), probes |
| CONTEXT | D9 server sentence, read only box, wait look, help line, one OK | Task B steps B2 to B6 |
| CONTEXT | D10 OK and Enter close like the X | Task B step B6 (C10) |
| CONTEXT | D11 a live wait answer locks | Task B steps B5 and B6 (C11) |
| CONTEXT | D12 nothing changes outside the wait; step 4 untouched; stale page accepted | Task B steps B6 and B9 |
| CONTEXT | D13 pins found first, the three kinds of new tests, docs only where living | Tasks A and B steps A0, B0; facts (no living doc describes the layout) |
| REQ | No requirement IDs for a quick task; QUICK-261108-fku stands in | frontmatter |
| Excluded | 09-UI-SPEC.md (history), welcome.html.tmpl and welcome.js (D12), cmd/seedprofiles (C8, not edited) | not touched |
