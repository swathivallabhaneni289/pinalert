---
phase: quick-261027-jhd
plan: 01
type: execute
wave: 1
depends_on: []
quick_id: 261027-jhd
created: 2026-10-27
source: owner 2026-10-27 during the Phase 3.3 owner checks, "i want the diclaimer and policy page to work exactlty lke anyother policy page which asks for confirmation", "have the agree and disagree button pop up after the user reads the disclaimer and policy page", and "yes make it gentler"
status: planned
files_modified:
  - web/welcome_flow_test.mjs
  - web/welcome_alerts_test.mjs
  - web/welcome_contract_test.go
  - internal/api/handlers/welcome_e2e_test.go
  - web/templates/welcome.html.tmpl
  - web/static/js/welcome.js
  - web/static/css/welcome.css
  - web/templates/device_guide.html.tmpl
  - web/static/css/device-guide.css
  - .planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md
autonomous: true
requirements:
  - QUICK-261027-jhd
must_haves:
  truths:
    - "On step 2, Agree and Disagree cannot be seen or reached with Tab until the end of the Disclaimer and policy text has been in view inside #welcome-scrollbox. Then they appear in the same place, and nothing on the page moves"
    - "If the whole text fits in the box without scrolling, Agree and Disagree show at once"
    - "Where IntersectionObserver is missing or throws, where the end marker is missing, and for an account that has already agreed, Agree and Disagree show at once"
    - "A VoiceOver user who reads the text to its last line reveals the buttons the same way a sighted user does, because the end marker sits right after the last line and is watched with the scrollbox as its root"
    - "Once shown, Agree and Disagree stay shown for the rest of the visit. Read again, Disagree, Back, a failed Agree and coming back to step 2 never hide them again. The Disagree box, Read again and Log out behave exactly as before"
    - "On an iPhone Safari tab (device state needs-install), step 5 shows the title Alerts, the one line You can turn on alerts later in Settings., Continue and the Back control, and no install guide"
    - "The ready, blocked and unsupported variants of step 5 are unchanged. The Settings install guide window and the device_guide partial in it are unchanged. Step 5 is still the only place first-run-done is sent"
    - "Phone and laptop behave the same; only placement differs"
  artifacts:
    - path: web/templates/welcome.html.tmpl
      provides: "the end marker as the last child of #welcome-scrollbox, an id on the Agree and Disagree row, and the one needs-install line in step 5. The template no longer includes the install guide or links device-guide.css"
      contains: "id=\"welcome-scrollbox-end\""
    - path: web/static/js/welcome.js
      provides: "watchForEnd and revealConsent (an IntersectionObserver whose root is the scrollbox), the focus trap skipping a held back row, and the needs-install title Alerts"
      contains: "IntersectionObserver"
    - path: web/static/css/welcome.css
      provides: "the held back row keeps its place (visibility only, no motion) and the 1px end marker"
      contains: ".welcome__actions--pair[data-unread]"
    - path: web/welcome_flow_test.mjs
      provides: "a fake IntersectionObserver and the step 2 reveal cases"
      contains: "IntersectionObserver"
    - path: .planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md
      provides: "the dated owner amendment for the needs-install copy"
      contains: "Owner amendment, 2026-10-27"
  key_links:
    - from: web/static/js/welcome.js
      to: "#welcome-scrollbox-end inside #welcome-scrollbox"
      via: "new window.IntersectionObserver(callback, { root: scrollbox, rootMargin: END_MARGIN, threshold: 0 }) observing the marker"
      pattern: "root: scrollbox"
    - from: web/static/js/welcome.js
      to: web/static/css/welcome.css
      via: "the script sets and then removes the data-unread attribute on #welcome-consent-actions, and the CSS hides that row with visibility only"
      pattern: "data-unread"
    - from: "welcome.js isShown(node)"
      to: "focusables() and the Tab trap"
      via: "a control inside an element that carries data-unread counts as not shown, so Tab never walks out of the dialog"
      pattern: "hasAttribute("
    - from: "welcome.js drawAlerts(state)"
      to: "#welcome-alerts-install (now one line of copy) and the step 5 title"
      via: "TITLE_LATER for the needs-install variant"
      pattern: "TITLE_LATER"
---

<objective>
Two owner requests from the Phase 3.3 owner checks on 2026-10-27, both in the first-time flow at /welcome.

1. Step 2, Disclaimer and policy, works like the terms screen of any other app. Agree and Disagree stay hidden until the person reaches the end of the text in #welcome-scrollbox, then they appear in the same place. A text that fits shows them at once. VoiceOver and keyboard users reveal them the same way, by reaching the end of the text. Where IntersectionObserver is missing, they show at once. There is no animation, and nothing moves.
2. Step 5, Alerts, in the needs-install device state only (an iPhone Safari tab that has not been added to the Home Screen), is made gentler. The three-step install guide is replaced by one line, "You can turn on alerts later in Settings.", with Continue and Back. The guide stays in Settings unchanged.

Purpose: the owner decided to keep Pinalert a website, with the Home Screen install only an option for iPhone notifications and no email alerts. The first-time flow should therefore not push the install. Step 2 should behave the way people already know from other apps.

Output: one RED commit of tests, one GREEN commit of the template, script and CSS (plus two comment-only fixes), and one docs commit with the dated UI-SPEC amendment.

Decisions taken at Claude's discretion (owner delegates design polish), each stated here so the executor and the SUMMARY carry them:
- Step 5 needs-install title: **"Alerts"**. It is the step's own name in the laptop step list, it is short and calm, and it does not ask the person to do anything. The ready, blocked and unsupported variants keep "Get alerts when it matters". The title stays in welcome.js, its one source, as the install title was.
- The hold on Agree and Disagree is applied by welcome.js, never written into the template. It is applied only when the reveal can run: the account has not agreed, IntersectionObserver exists, and both the row and the marker are on the page. A page whose script fails therefore never hides Agree. The server never renders step 2 first (welcomeStartStep in internal/api/handlers/welcome.go returns 1 or 3), so the row is held back before step 2 is ever shown and nothing flashes.
- The row is held back with visibility, so it keeps its place in the layout. The hidden attribute or display none would take it out of the flow, and the flex scrollbox would grow and then shrink on reveal.
- The observer uses rootMargin '0px 0px 24px 0px' (about the box's 20px bottom padding). When VoiceOver scrolls the last line ("Last updated ...") just into view, the 1px marker right under it then still counts as reached.
- An account that has already agreed (the server started it at step 3) is never held back. It may walk Back to step 2, and on the phone Agree is its only way forward from there.
- The welcome page no longer links device-guide.css, since nothing on it uses the guide. Settings still links it.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md
@.planning/phases/03.3-installable-app-and-phone-alerts/03.3-13-SUMMARY.md
@.planning/todos/pending/2026-10-27-agree-and-disagree-appear-after-reading.md
@web/templates/welcome.html.tmpl
@web/static/js/welcome.js
@web/static/css/welcome.css
@web/welcome_flow_test.mjs
@web/welcome_alerts_test.mjs
@web/welcome_contract_test.go
@internal/api/handlers/welcome_e2e_test.go

Facts read during planning (do not re-derive):
- welcome.js isShown(node) walks parentNode up to the root and checks only the hidden property. focusables() uses it. The Tab trap in onKeydown wraps only when the active element is the first or last item of focusables(). A visibility-hidden Agree left in that list would let Tab from the scrollbox walk out of the dialog, so isShown must learn the hold.
- Neither Node harness defines IntersectionObserver. Both build Disagree and Agree straight inside step 2, with no row and no marker. welcome_alerts_test.mjs can stay as it is for step 2: with no observer there is no hold.
- The init order at the end of welcome.js is: read data-start-step, set agreed (startStep > CONSENT_STEP), then goTo(firstStep(startStep), 'replace'). The hold must be set between the last two.
- legal.css: .legal-section has a top margin only, and .legal-section > :last-child has margin-bottom 0. The policy partial ends with the line "Last updated {{.PolicyLastUpdated}}", so a marker placed after the two partials sits right under that line.
- welcome_contract_test.go pins the exact row `<div class="welcome__actions welcome__actions--pair">`, `{{template "device_guide"}}` once, the device-guide.css link once, and `'Install to get alerts'` once in welcome.js. All four pins change here, each with a dated note.
- settings_rows_contract_test.go pins the guide window, the partial once and the device-guide.css link in settings.html.tmpl. Settings is not touched, so those pins keep passing.
- noise_words_contract_test.go TestDeviceGuideCopy pins the guide's copy with template comments removed, so a comment-only edit to device_guide.html.tmpl does not affect it.
- The untracked folder pinalert-browser-checks/ (the owner's local tooling, not in the repo) has checks/test18.js, which clicks #welcome-agree directly. Do not edit it. Record in the SUMMARY that it now needs to scroll the text box to the end first.

House rules for every file touched: no em or en dashes anywhere (copy, comments, commit messages). No Python. No images saved in the repo. Text reaches the page only through textContent. Never quote a Go template action such as a template include inside a template comment; name it in words. Do not touch STATE.md, ROADMAP.md, settings.html.tmpl, settings-alerts.js, or any file another session has dirty (03.3-UAT.md, the 08 and 09 phase files).
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1 (RED): failing tests for Agree after reading and the gentler needs-install step 5</name>
  <files>web/welcome_flow_test.mjs, web/welcome_alerts_test.mjs, web/welcome_contract_test.go, internal/api/handlers/welcome_e2e_test.go</files>
  <behavior>
    - Step 2 with an observer: right after load (start 1), #welcome-consent-actions carries data-unread. Exactly one observer exists, with options.root being the scrollbox, rootMargin '0px 0px 24px 0px', and the marker as its one target.
    - While held back, Tab from the focused scrollbox is prevented and wraps to the first control (the first step list button on the laptop, welcome-back-2 on the phone). Shift+Tab from that first control lands on the scrollbox. Agree and Disagree are never in the loop.
    - An entry with isIntersecting false changes nothing. An entry with isIntersecting true removes data-unread and disconnects the observer. After that, Tab from the scrollbox is left to the browser (not prevented), and Tab from Agree wraps to the first control as before.
    - A text that fits: the first entry the browser sends already has isIntersecting true, so the row shows at once and no request is sent.
    - With no IntersectionObserver, with a constructor that throws, or with no marker in the page, the row never carries data-unread after load.
    - An account that already agreed (start 3) gets no observer, and goTo(2) shows the row.
    - Once shown, the row stays shown. Disagree shows the box with focus in it, Read again sets scrollTop to 0 and focuses the scrollbox, goTo(1) then goTo(2) and a failed Agree (500) all leave the row without data-unread. Only the one consent POST is sent.
    - Step 5 needs-install: the title is 'Alerts' (on arrival and when focus lands on it, and after the unsupported redraw from Allow alerts), #welcome-alerts-install is shown, and only Continue is shown.
    - Contract: the template holds the marker as the last child of the scrollbox, the row with its id, and no data-unread. Step 5 holds the one needs-install line. The template no longer includes the install guide or links device-guide.css. The CSS rule for the held back row sets visibility hidden only. The marker rule sets a 1px height. welcome.js uses IntersectionObserver with root: scrollbox, disconnects, has isShown read an attribute, and holds 'Alerts' once and the old install title zero times.
    - e2e: /welcome renders the row id and the marker, has no data-unread, renders the needs-install line, and contains neither device-guide nor Home Screen anywhere.
  </behavior>
  <action>
Write tests only in this task. Do not touch the template, the script or the CSS. Each amended pin gets a dated comment, "Amended 2026-10-27 by quick 261027-jhd (owner): ...", in the style of the existing amendment notes, saying in plain words why it changed.

**web/welcome_flow_test.mjs**
- In buildPage, inside the scrollbox, make a div with id welcome-scrollbox-end, class welcome__scrollbox-end and aria-hidden "true", as the scrollbox's last child.
- Also in buildPage, wrap the two step 2 buttons in a div with id welcome-consent-actions and className 'welcome__actions welcome__actions--pair', placed where the buttons were (before the Disagree box). The buttons keep their ids. buildPage takes a new option marker (default true); with false, the marker is not made.
- In load, accept an option observer. When it is true, put a FakeIO class on the sandbox as IntersectionObserver. Its constructor stores the callback and options, keeps a targets list and a disconnected flag, and pushes itself onto an observers array. observe(target) appends the target; disconnect() sets the flag.
- When observer is 'throws', the constructor throws an Error.
- Without the option, the sandbox keeps having no IntersectionObserver, so every existing test runs unchanged.
- Return observers from load. Add a helper seeEnd(page, isIntersecting) that calls the callback of every observer not disconnected with one entry per target, of the shape { target, isIntersecting }, plus the observer itself as the second argument.
- Add a section header comment "Quick 261027-jhd (owner, 2026-10-27): Agree and Disagree appear after reading". Under it, add these tests, as listed in the behavior block:
  1. Held back until the end is seen, with the laptop and phone Tab checks. For the phone, use a second load with side.hiddenByCss true.
  2. A text that fits shows them at once.
  3. Shown at once with no observer, with a throwing constructor, and with no marker.
  4. An account that already agreed is never held back.
  5. Once shown they stay shown.
- Use assert.ok for element identity, as the file already does, so a failure never prints the whole fake page.
- Leave the existing step 2 and Tab tests as they are. They load without an observer and must keep passing unchanged.

**web/welcome_alerts_test.mjs**
- Add a constant LATER_TITLE = 'Alerts' next to READY_TITLE.
- Rename the needs-install test to "needs-install: one line, the title Alerts, and one Continue with no Allow alerts".
- Its two title asserts, and the title assert in "Allow alerts on a device that turns out unsupported redraws that variant", expect LATER_TITLE. Keep every other assert, including that #welcome-alerts-install is shown.
- Update the file's header comment where it describes the needs-install part. The step 2 markup of this harness stays as it is.

**web/welcome_contract_test.go**
- TestWelcomeStep2ReadsTheSharedText:
  - Change the exact row pin to `<div class="welcome__actions welcome__actions--pair" id="welcome-consent-actions">`.
  - Add a regexp pin, (?s) mode, that the scrollbox opening tag is followed, with only whitespace between them, by the disclaimer partial, the policy partial, then exactly `<div class="welcome__scrollbox-end" id="welcome-scrollbox-end" aria-hidden="true"></div>`, then the closing div.
  - Extend the existing p13Ordered call so the marker id comes between the policy partial and id="welcome-disagree".
  - Add p13MustNotContain on the page for data-unread, with the comment "the hold is the script's, so a page whose script never runs still shows Agree".
- Add a new test, TestWelcomeAgreeAfterReading, which pins the following.
  - The CSS rules (parseCSSRules, ruleBySelector and declsOf on stripCSSComments of css/welcome.css):
    - `.welcome__actions--pair[data-unread]` has exactly one declaration, visibility hidden.
    - `.welcome__scrollbox-end` has height 1px.
    - The single-declaration pin above is the no-motion proof for the reveal. Do not add a file-wide ban on transitions in welcome.css: the house rules allow short state transitions, and Phase 7.1 edits this file later.
  - The script (stripJSComments of js/welcome.js):
    - It contains window.IntersectionObserver, root: scrollbox and .disconnect().
    - jsFunctionBody of "isShown(node)" contains hasAttribute(.
    - jsFunctionBody of "readAgain()" contains neither revealConsent nor watchForEnd. Read again only scrolls and focuses.
- TestWelcomeAlertsStep:
  - Replace the "the one install guide" entry with the exact line `<p class="welcome__lead" id="welcome-alerts-install" hidden>You can turn on alerts later in Settings.</p>`.
  - Change the two w10Count pins (the guide include and the device-guide.css link) from 1 to 0, with the note that the guide now lives only in the Settings window.
  - Count the new sentence once in the page, and add p13MustNotContain for Home Screen on the page.
  - Keep the existing check that the old install title is not in the template.
  - Keep the check that #welcome-alerts-install starts hidden. It still matches.
- TestWelcomeAlertsScript:
  - In the list of sentences the script writes, replace the old install title literal with `'Alerts'`, counted once.
  - Add a w10Count of the old install title literal (single quoted, as before) at 0. The guide title lives only in settings.html.tmpl now.
- Run gofmt on the file.

**internal/api/handlers/welcome_e2e_test.go**
- TestOnboardingFlowEndToEnd: add to the want list the row opening tag with its id, and the full marker div as written above. Assert the body does not contain data-unread.
- TestWelcomeStepFiveRendersForNewAccount: add "You can turn on alerts later in Settings." to the want list. Assert the body contains neither device-guide nor Home Screen.
- Dated comments as above. Run gofmt.

**Commit (RED)**, path limited, nothing else staged by you:
- Command: git commit -m "test(quick-261027-jhd): failing tests for Agree after reading and a gentler iPhone step 5" -m "<two or three plain lines on what the tests pin>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- web/welcome_flow_test.mjs web/welcome_alerts_test.mjs web/welcome_contract_test.go internal/api/handlers/welcome_e2e_test.go
- Never add any other trailer.
  </action>
  <verify>
    <automated>! node --test web/welcome_flow_test.mjs && ! node --test web/welcome_alerts_test.mjs && ! go test ./web/ -run '^TestWelcomeAgreeAfterReading$' -count=1 && ! go test ./web/ -run '^TestWelcomeAlertsStep$' -count=1 && ! go test ./web/ -run '^TestWelcomeAlertsScript$' -count=1 && ! go test ./web/ -run '^TestWelcomeStep2ReadsTheSharedText$' -count=1 && go test ./web/ -run '^(TestWelcomeFiveStepsInOrder|TestWelcomeDisagreeBoxStyle|TestWelcomeScriptsAndStyles|TestWelcomeAlertsExampleFromGo|TestDeviceGuideCopy|TestDeviceGuideLeadSpacing|TestWelcomeStatusIsAlwaysPresentLiveRegion)$' -count=1 && go vet ./internal/api/handlers/ && test -z "$(gofmt -l web/welcome_contract_test.go internal/api/handlers/welcome_e2e_test.go)"</automated>
  </verify>
  <done>
- The RED commit holds only the four test files.
- The two Node files fail, and only on the new step 2 cases and the needs-install title asserts. Every older case in welcome_flow_test.mjs still passes against the old script. Check with node --test and read the failing names.
- TestWelcomeAgreeAfterReading, TestWelcomeAlertsStep, TestWelcomeAlertsScript and TestWelcomeStep2ReadsTheSharedText each fail. The other welcome and device guide contract tests named in the verify pass.
- The handlers package compiles and the two Go files are gofmt clean.
  </done>
</task>

<task type="auto">
  <name>Task 2 (GREEN): hold Agree and Disagree until the end is seen, and the one-line needs-install step 5</name>
  <files>web/templates/welcome.html.tmpl, web/static/js/welcome.js, web/static/css/welcome.css, web/templates/device_guide.html.tmpl, web/static/css/device-guide.css</files>
  <action>
Make the Task 1 tests pass without changing them.

**web/templates/welcome.html.tmpl**
- Remove the stylesheet link to /static/css/device-guide.css from the head. Nothing on this page uses the guide now.
- Step 2 (per the owner's first request):
  - Inside #welcome-scrollbox, after the policy partial and as the box's last child, add exactly `<div class="welcome__scrollbox-end" id="welcome-scrollbox-end" aria-hidden="true"></div>`.
  - Give the Agree and Disagree row the id welcome-consent-actions, so its opening tag reads exactly `<div class="welcome__actions welcome__actions--pair" id="welcome-consent-actions">`.
  - Do not write any hold attribute in the template.
  - The lead, the buttons, the Disagree box, Read again, Log out and the error line stay byte for byte.
- Step 5 (per the owner's second request): replace the whole needs-install part (the div with id welcome-alerts-install and the guide include inside it) with exactly `<p class="welcome__lead" id="welcome-alerts-install" hidden>You can turn on alerts later in Settings.</p>`, in the same position. The ready part, the blocked and unsupported lines, the status and error lines and the three buttons stay byte for byte.
- In the head template comment, add one dated sentence: quick 261027-jhd (owner, 2026-10-27) makes step 2's Agree and Disagree wait until the end of the text has been seen, which welcome.js does. It also makes step 5 on an iPhone tab show one line instead of the install guide, which now lives only in Settings. Name the partial in words, never as a template action.

**web/static/js/welcome.js**
Follow the file's existing ES5 style: var, named functions, two-space indent, comments in plain words.
- Header comment: add a short paragraph on step 2. Agree and Disagree are held back until the end-of-text marker inside the text box has been in view once, watched by an IntersectionObserver whose root is the text box, so reading to the end with VoiceOver reveals them too. The hold is set by this script only, and only when the reveal can run. Once shown they stay shown. In the step 5 paragraph, say the needs-install part is now one line, with the title Alerts.
- Constants:
  - Replace the old install title constant with TITLE_LATER = 'Alerts'. Its comment says the owner, on 2026-10-27, made the needs-install part gentler: the step's own name above one line written in the template.
  - Update the comment above it so the template is named as the source of the needs-install sentence too.
  - Add UNREAD_ATTR = 'data-unread'.
  - Add END_MARGIN = '0px 0px 24px 0px', with a comment saying it is about the box's bottom padding, so the 1px marker under the last line counts as reached when VoiceOver scrolls that line just into view.
- Elements: read consentActions = el('welcome-consent-actions') and scrollEnd = el('welcome-scrollbox-end'). Add var endWatcher = null.
- isShown(node): a node counts as not shown when it, or any ancestor up to the root, has the hidden property or carries UNREAD_ATTR. Use hasAttribute. Update its comment so the focus trap skips a held back row as well.
- Add, in the Step 2 section, function revealConsent():
  - It removes UNREAD_ATTR from consentActions when consentActions exists.
  - If endWatcher is not null, it disconnects it inside try/catch and sets it to null.
  - Nothing ever sets the attribute again.
- Add function watchForEnd(). It returns at once, without setting anything, when any of these holds:
  - agreed is true;
  - consentActions or scrollEnd is null;
  - typeof window.IntersectionObserver is not 'function'.
  Otherwise:
  - It sets UNREAD_ATTR to '' on consentActions.
  - Inside try, it creates endWatcher = new window.IntersectionObserver(callback, { root: scrollbox, rootMargin: END_MARGIN, threshold: 0 }) and observes scrollEnd. The callback loops the entries and calls revealConsent() when any entry has isIntersecting true.
  - In catch, it calls revealConsent(), so a browser that cannot watch never locks anyone out.
- Init: call watchForEnd() after agreed is set and before the first goTo(firstStep(startStep), 'replace'). Do not call it anywhere else.
- Leave readAgain, disagree, agree, goTo, hideDisagreeBox and the 409 path unchanged. Focus is never moved on reveal; it stays where the person is.
- drawAlerts: the title for the needs-install variant is TITLE_LATER. Every other variant keeps alertsTitleReady.
- Nothing else in step 5 changes: enterAlerts, the buttons, postDone and its one caller.

**web/static/css/welcome.css**
- After the base `.welcome__actions--pair .btn` rule, add `.welcome__actions--pair[data-unread]` with the single declaration visibility: hidden. Its comment, dated 2026-10-27 (quick 261027-jhd, owner): until the end of the text has been seen, welcome.js holds Agree and Disagree back. Visibility keeps their place, so nothing moves when they appear, and it takes them out of Tab and the accessibility tree. No motion.
- After the scrollbox rules, add `.welcome__scrollbox-end` with height: 1px. Its comment: the end-of-text marker welcome.js watches. It is 1px tall, not zero, so every browser reports it as in view.
- Add no transition and no animation anywhere.

**web/templates/device_guide.html.tmpl and web/static/css/device-guide.css (comments only)**
- In the partial's head comment, the sentence about being shown in exactly two places becomes: shown in one place only, the Settings guide window (id device-guide-window). Step 5 of the first-time flow showed it until 2026-10-27, when the owner made that step gentler (quick 261027-jhd). The sentence about which plans link the stylesheet becomes: settings.html.tmpl links /static/css/device-guide.css.
- In device-guide.css, edit only the file's head comment the same way.
- Change no markup, no copy outside the comments, and no CSS rule or declaration. TestDeviceGuideCopy, TestDeviceGuideLeadSpacing and the settings pins must keep passing untouched.

**Commit (GREEN)**, path limited:
- Command: git commit -m "feat(quick-261027-jhd): Agree and Disagree appear after reading, and a gentler iPhone step 5" -m "<two or three plain lines>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- web/templates/welcome.html.tmpl web/static/js/welcome.js web/static/css/welcome.css web/templates/device_guide.html.tmpl web/static/css/device-guide.css
  </action>
  <verify>
    <automated>node --test web/welcome_flow_test.mjs web/welcome_alerts_test.mjs && go test ./web/ -count=1 && go vet ./web/ && git diff --quiet HEAD -- web/templates/settings.html.tmpl web/static/js/settings-alerts.js</automated>
  </verify>
  <done>
- Both Node files pass in full, including every older step 2 and Tab case.
- go test ./web/ passes in full, including the four tests that failed in Task 1, TestDeviceGuideCopy, TestDeviceGuideLeadSpacing and every settings pin.
- Settings files are untouched.
- The GREEN commit holds only the five files listed.
  </done>
</task>

<task type="auto">
  <name>Task 3: dated UI-SPEC amendment for the needs-install copy, then the full check on a disposable database</name>
  <files>.planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md</files>
  <action>
Use the Edit tool only, with scoped replacements. Never rewrite the whole file. Add dated notes and leave the original wording readable. Each note says "Amended 2026-10-27 (owner, quick 261027-jhd)".

1. In the "Superseded text" table, add a row:
   - Superseded: step 5 needs-install variant, the title "Install to get alerts" and the install guide.
   - Where it lives: 1a, 1d, Surface 4, the States Matrix, P33-C7 and P33-C15.
   - Replaced by: the owner amendment of 2026-10-27 below.
2. Right after that table, add a subsection "### Owner amendment, 2026-10-27 (quick 261027-jhd)" with:
   - the owner's words "yes make it gentler";
   - the reason: the owner keeps Pinalert a website, with the Home Screen install only an option for iPhone notifications, and no email alerts;
   - the new needs-install content: title "Alerts", the one line "You can turn on alerts later in Settings." (Body, --color-text, the same look as the blocked and unsupported lines), the one button Continue and the built Back control;
   - why "Alerts": the step's own name in the step list, short, and it asks nothing of the person;
   - what stays: the ready, blocked and unsupported variants; the install guide in the Settings window (device-guide-window) with its title "Install to get alerts"; the device_guide partial used there; step 5 as the only place first-run-done is sent;
   - a line noting that the welcome page no longer links device-guide.css.
3. Add a short dated pointer to that subsection in each of these places:
   - the 1a Focal point row;
   - the 1d needs-install row;
   - the Surface 4 opening sentence about exactly two places (now one: the Settings window);
   - the 4a Title row;
   - the States Matrix "Step 5, needs-install" row (idle is now the one line and Continue);
   - P33-C7 and P33-C15.
4. In the Copywriting Contract table, add a row P33-C21, "Step 5, needs install (owner amendment 2026-10-27)": title "Alerts"; line "You can turn on alerts later in Settings."; button "Continue"; Back control "Back".
5. No em or en dashes in anything you add.

Commit (docs), path limited:
- Command: git commit -m "docs(quick-261027-jhd): owner amendment 2026-10-27 for the step 5 needs-install copy" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" -- .planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md

Then run the full check:
- go build ./... and go vet ./...
- go test ./web/ -count=1
- node --test web/*.mjs
- The handlers package on the fresh disposable database p_quick_jhd_a. Use exactly the command in verify, which drops the database even when a test fails and keeps the exit code.
  - The package can take several minutes, so use a long timeout or run it in the background.
  - Never point any test at pinalert_test, pinalert, p33_uat or any other database.
- If anything fails, fix it in the file that owns the failure and commit the fix path limited, with the same trailer.

In the SUMMARY, record:
- the discretion choices from the objective;
- that 03.4-UI-SPEC.md was not amended for step 2 (not asked; the todo file and this SUMMARY carry the step 2 change);
- that the step 2 lead still says "Tap Agree to continue." (unchanged, as asked);
- that the untracked pinalert-browser-checks/checks/test18.js now has to scroll the text box to the end before it clicks Agree.

Do not touch STATE.md or ROADMAP.md. Do not move the todo file; the orchestrator does that.
  </action>
  <verify>
    <automated>grep -c 'Owner amendment, 2026-10-27' .planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md && go build ./... && go vet ./... && go test ./web/ -count=1 && node --test web/*.mjs && dropdb --if-exists p_quick_jhd_a && createdb p_quick_jhd_a && (env -i HOME="$HOME" PATH="$PATH" DATABASE_URL='postgres://localhost:5432/p_quick_jhd_a?sslmode=disable' go test ./internal/api/handlers/ -count=1; rc=$?; dropdb p_quick_jhd_a; exit $rc)</automated>
    <human-check>
Not done by the executor and not blocking. The owner, or the orchestrator on a scratch copy, checks it afterwards.
- Setup: follow the UAT scratch server recipe, with its own port (for example 8793, never 8080, 8090, 8776, 8778 or 8781), its own scratch database and a new account.
- iPhone simulator Safari, at phone width, step 2:
  1. Agree and Disagree are not there, and their space is kept.
  2. Scrolling the text to its last line makes them appear, with nothing moving.
  3. Read again scrolls to the top and the buttons stay.
  4. With VoiceOver, reading through to "Last updated" reveals them.
- iPhone simulator Safari, step 5 in the Safari tab: the title Alerts, the one line, Continue and Back, and no install steps.
- Settings, On this device row: it still opens the Install to get alerts window with the three steps.
- Mac Safari at laptop width: the same on step 2. Tab from the text box stays inside the flow before the buttons appear, and reaches Disagree then Agree after.
- Save no screenshots in the repo. Tear the scratch server and database down afterwards.
    </human-check>
  </verify>
  <done>
- The UI-SPEC carries the dated subsection, the Superseded row, the pointers and P33-C21.
- go build, go vet, go test ./web/, node --test web/*.mjs and the handlers package on p_quick_jhd_a all pass, and p_quick_jhd_a has been dropped.
- There are three commits (RED, GREEN, docs) plus any fix commits. Each is path limited and ends with exactly the one Co-Authored-By trailer.
- STATE.md and ROADMAP.md are untouched.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser to /api/me/consent and /api/me/first-run-done | the person's own browser runs welcome.js. Anything it does can be changed in devtools, so the server alone decides whether the terms were agreed |
| server template to page | static copy and the shared legal partials are rendered by html/template. No new value from the person reaches the page |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-261027-01 | Elevation of privilege | step 2 reveal in welcome.js | low | accept | The reveal is a reading aid, not a control. Someone can skip it in devtools or through window.PinalertWelcome.agree. Consent is still recorded only by POST /api/me/consent (unchanged), and the server still refuses first-run-done with 409 without the agreement (unchanged, pinned by the existing e2e tests) |
| T-261027-02 | Denial of service | step 2 hold (self lockout) | high | mitigate | An account is required to use the site, so an Agree that never appears blocks the whole app. The hold is set only by the script, and only when the account has not agreed and IntersectionObserver and the marker exist. A throwing constructor reveals at once. The marker is 1px with a 24px bottom rootMargin. The hold is never set again after a reveal. Accounts that already agreed are never held. The template carries no hold. Each case is pinned by the Task 1 Node tests and the contract pin that the template holds no data-unread |
| T-261027-03 | Tampering | page markup sinks | low | mitigate | No new sink. The script only sets and removes one attribute and reads entries. TestWelcomeScriptsAndStyles keeps innerHTML, outerHTML, insertAdjacentHTML, document.write, eval, new Function and DOMParser out of welcome.js |
| T-261027-04 | Information disclosure | step 5 needs-install line | low | accept | Static copy, no account data, nothing sent. Step 5 still asks the browser nothing outside the Allow alerts tap (pinned by TestWelcomeAlertsScript) |

No package installs, so no supply chain entry.
</threat_model>

<verification>
- node --test web/*.mjs passes, with the new step 2 cases in web/welcome_flow_test.mjs.
- go test ./web/ -count=1 passes, including TestWelcomeAgreeAfterReading and the amended TestWelcomeStep2ReadsTheSharedText, TestWelcomeAlertsStep and TestWelcomeAlertsScript, with the settings and device guide pins unchanged.
- go test ./internal/api/handlers/ passes on the fresh p_quick_jhd_a, which is then dropped.
- go build ./... and go vet ./... pass, and the Go test files are gofmt clean.
- git log shows three path limited commits (RED, GREEN, docs), each ending with exactly `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
</verification>

<success_criteria>
- Step 2 works like the terms screen of any other app:
  - Agree and Disagree appear only once the end of the text has been in view, the same on phone and laptop, by scrolling, keyboard or VoiceOver;
  - they show at once when the text fits, when IntersectionObserver is missing, or when the account has already agreed;
  - nothing moves and nothing animates;
  - Tab never leaves the dialog;
  - Read again, Log out and the Disagree box are unchanged.
- Step 5 on an iPhone Safari tab is gentle: the title Alerts, the line "You can turn on alerts later in Settings.", Continue and Back. Every other step 5 variant, the Settings guide window and the first-run-done rule are unchanged.
- 03.3-UI-SPEC.md carries the dated owner amendment of 2026-10-27.
</success_criteria>

<output>
Create `.planning/quick/261027-jhd-agree-after-reading-and-a-gentler-iphone/261027-jhd-SUMMARY.md` when done.
</output>
