---
phase: quick-261025-kky
plan: 01
type: execute
wave: 1
depends_on: []
quick_id: 261025-kky
created: 2026-10-25
source: owner 2026-10-25, "yes exactly how the logout option is looking and it should work like any delete my account thing work"
status: planned
files_modified:
  - web/settings_alerts_test.mjs
  - web/settings_rows_contract_test.go
  - web/templates/settings.html.tmpl
  - web/static/css/auth.css
  - web/static/js/settings-alerts.js
  - .planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-UI-SPEC.md
autonomous: true
requirements:
  - QUICK-261025-kky
must_haves:
  truths:
    - "Tapping the Delete my account row on Settings opens a small centred box with the same look as the Log out box: the question Delete your account?, the three sentences, and Cancel beside a red Delete button sharing the row equally"
    - "When the box opens, focus is on the question, no button looks chosen, nothing is sent, and the page behind stays still"
    - "Cancel closes the box, sends nothing and puts focus back on the Delete my account row; Escape does the same while nothing is being sent; a tap on the dim page does nothing"
    - "Delete posts once to /api/me/delete with {confirm:true}, the button reads Deleting… and Cancel is disabled while it waits, Escape cannot close the box while it waits, and success goes to /login?done=deleted"
    - "A failed or offline delete shows Couldn't delete your account. Try again. inside the open box and gives back both buttons with the label Delete"
    - "The Delete my account row, the Log out of all devices window, the Log out box and every other window look and behave exactly as before"
    - "At 320px wide with 200 percent text the box fits: its buttons stack instead of overflowing and the box scrolls inside itself"
  artifacts:
    - path: web/templates/settings.html.tmpl
      provides: "dialog#settings-delete-confirm.logout-confirm placed just before the Name window; the old Delete app-window removed"
      contains: "<dialog id=\"settings-delete-confirm\" class=\"logout-confirm\""
    - path: web/static/js/settings-alerts.js
      provides: "openDeleteConfirm, the native dialog wiring, the window.confirm fallback, the in-box failure line"
      contains: "openDeleteConfirm"
    - path: web/static/css/auth.css
      provides: "dated rules for the box text, title gap, error, action spacing, wrap and inner scroll"
      contains: ".logout-confirm__text"
    - path: web/settings_rows_contract_test.go
      provides: "TestDeleteAccountConfirmBox"
      contains: "func TestDeleteAccountConfirmBox"
    - path: web/settings_alerts_test.mjs
      provides: "fake dialog (showModal, close, open attribute, cancel and close events) and the rewritten Delete tests"
      contains: "showModal"
  key_links:
    - from: web/static/js/settings-alerts.js
      to: web/templates/settings.html.tmpl
      via: "getElementById of settings-delete-confirm, settings-delete-title, settings-delete-cancel, settings-delete-go, settings-delete-error"
      pattern: "settings-delete-confirm"
    - from: web/static/js/settings-alerts.js
      to: "POST /api/me/delete"
      via: "deleteAccount with JSON.stringify({ confirm: true }), then window.location.assign(DELETED_PAGE)"
      pattern: "DELETED_PAGE"
    - from: web/templates/settings.html.tmpl
      to: web/static/css/auth.css
      via: "the shared .logout-confirm classes"
      pattern: "logout-confirm__actions"
---

<objective>
Replace the X only Delete my account window on Settings with a small native confirm box that looks and works like the Log out box (owner, 2026-10-25: "yes exactly how the logout option is looking and it should work like any delete my account thing work").

Purpose: the owner wants Delete my account to ask the way any normal app asks, in the same small box as Log out, with Cancel and a red Delete. The server route, the row, and every other window stay as they are.

Output: tests first (RED commit), then the dialog markup, the CSS for its text and the JS wiring (GREEN commit), then a dated UI-SPEC note and the full gate (docs commit).
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@web/templates/settings.html.tmpl
@web/templates/account_header.html.tmpl
@web/static/css/auth.css
@web/static/js/account-menu.js
@web/static/js/settings-alerts.js
@web/settings_alerts_test.mjs
@web/settings_rows_contract_test.go
@web/places_contract_test.go
@web/logout_confirm_test.mjs
@.planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-UI-SPEC.md

Facts the planner checked, so the executor does not need to re-derive them:

- The Log out box to copy is dialog#logout-confirm in account_header.html.tmpl (lines 21 to 29) and its rules in auth.css (lines 564 to 623: .logout-confirm, ::backdrop, html:has(.logout-confirm[open]) page lock, __body, __title with :focus outline none, __actions:not([hidden]) flex with gap, __actions .btn flex 1 1 0, forced-colors border). Its script is account-menu.js lines 223 to 307. Native Escape closes it through the default cancel event; it has no click handler on the dialog.
- web/places_contract_test.go settingsWindowRegion (line 249) cuts a window's region at the next `<div class="app-window ` or the app_bottombar include. That is why the new dialog must sit BEFORE the Name window: placed after the Log out of all devices window it would fall inside that window's region and trip its banned word check. places_contract_test.go must NOT be edited.
- TestSettingsAlertsScriptContract (settings_rows_contract_test.go line 208) still requires the literal 'Escape' (the Muted and Log out of all devices windows keep onWindowKeydown), requires every location.assign to go through LOGGED_OUT_PAGE or DELETED_PAGE, and bans deleteWin.addEventListener('click'.
- internal/api/handlers/places_e2e_test.go TestSettingsRows matches only buttons with class place-row, so the new dialog buttons do not change it. No other Go or JS test names settings-delete-window, settings-delete-close or settings-delete-go (grep confirmed).
- .inline-error:not([hidden]) in chrome.css has specificity (0,2,0) and chrome.css loads after auth.css, so an override inside the box needs a (0,3,0) selector such as .logout-confirm .inline-error:not([hidden]).
- internal/testutil NewTestDB applies the embedded migrations itself, so a fresh empty database is enough for the handlers tests.

Coordination: another session (pinalert-60) is building Phase 3.3 into main and shares settings.html.tmpl and settings_rows_contract_test.go. Work in the worktree, do NOT merge to main, and keep the diff in those two files to the regions named below.
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED, the Delete box tests in the Node suite and the Go contract</name>
  <files>web/settings_alerts_test.mjs, web/settings_rows_contract_test.go</files>
  <behavior>
    - The Delete my account row opens the box: the dialog has the open attribute, focus is on settings-delete-title, no request is sent, the page is not inert and data-modal-open is not set (the box uses the native modal, not lockPage)
    - Cancel closes it (open attribute gone), focus goes back on settings-delete-row, no request is sent
    - Escape while idle: the cancel event is not prevented, the box closes, focus goes back on the row
    - While deleting: Escape's cancel event is prevented and the box stays open; Cancel and Delete are both disabled
    - Delete clicked twice posts once to /api/me/delete, method POST, body {confirm:true}, the button reads Deleting…, then a 204 assigns /login?done=deleted
    - A 500 and a rejected fetch each show Couldn't delete your account. Try again. in settings-delete-error inside the still open box, Delete reads Delete again and is enabled, Cancel is enabled, focus is on settings-delete-title, nothing is assigned; Escape after the failure closes the box
    - A failure that lands after the browser closed the box anyway (the test calls close() on the dialog while the request waits, which moves focus to the row) opens the box again with the sentence showing and focus on settings-delete-title
    - The dialog has no click handler, and a click fired on it leaves it open
    - With no showModal on the dialog: a confirm that returns false sends nothing and the question asked is Delete your account?; a confirm that returns true sends once and a 204 assigns /login?done=deleted; a failure in that path calls alert with Couldn't delete your account. Try again.
    - The X close test and the Tab trap test now run on the Log out of all devices window
    - The handles test also expects openDeleteConfirm
    - Go: TestDeleteAccountConfirmBox pins the dialog markup and its position; TestAccountConfirmWindows covers the Muted and Log out of all devices windows only
  </behavior>
  <action>
Write the tests only. Do not touch the template, the CSS or the JS in this task. Commit the two files on their own as the RED commit.

settings_alerts_test.mjs:

1. Header comment: change the Delete bullet and the window bullet so they say Log out of all devices is a window that closes with the X or Escape, and Delete my account is a small native box like Log out (owner, 2026-10-25) answered by Cancel, Delete or Escape, where a tap on the dim page does nothing and Escape is refused while the delete is being sent.

2. FakeEl: add a getter named open that returns whether the open attribute is set. Do not add showModal or close to every element.

3. buildPage takes a new option dialog (default true). Replace the whole settings-delete-window block with a dialog element, id settings-delete-confirm, appended to doc.body, holding in order: a div, inside it an h2 settings-delete-title whose textContent is Delete your account?, a div settings-delete-text, a p settings-delete-error with hidden true, and a div holding a button settings-delete-cancel (text Cancel) then a button settings-delete-go (text Delete). When dialog is true, give only this element a showModal method that sets the open attribute to the empty string, and a close method that does nothing when the open attribute is absent and otherwise removes it and then calls every handler in handlers.close with an event object of type close. When dialog is false, the element has neither method. Keep returning it as del.

4. loadAlerts: default options to an empty object. Add to the sandbox a confirm function that records its question into an asked array and returns options.confirmAnswer, and an alert function that records its message into an alerted array. Return asked and alerted with the rest.

5. Add a helper pressEscape(page) that fires a cancel event on page.del with cancelable true through the existing fire helper, then calls page.del.close() only if the event was not prevented, and returns the event. This is the order a browser follows.

6. Delete the five old Delete tests (the confirm true test, the failed test, the network failure test, the X test that used settings-delete-close, and the Tab test that used settings-delete-close and settings-delete-go). Write the tests listed in the behavior block in their place, one test each, with plain names. Use the exact strings Delete your account?, Deleting…, Delete, Couldn't delete your account. Try again., /login?done=deleted.

7. Rewrite the X close test against the Log out of all devices window: row settings-logout-all-row, close settings-logout-all-close, page.logout hidden after, main not inert, data-modal-open null, focus on the row. Rewrite the Tab test the same way: focus settings-logout-all-go, Tab moves to settings-logout-all-close, Shift Tab moves back to settings-logout-all-go. Keep the existing Escape window test as it is (it already checks page.del has no click handlers, which stays true).

8. Add openDeleteConfirm to the name list in the handles test.

settings_rows_contract_test.go (keep the diff to these spots, pinalert-60 shares this file):

a. The file's top comment says the three windows; make it the two windows and the Delete box.

b. TestAccountConfirmWindows: remove the settings-delete-window table entry AND the trailing block that starts with the comment about the dark filled button and calls settingsWindowRegion for settings-delete-window with the critical or danger check (that block would t.Fatalf once the window is gone). Keep the dash check and the script order check after it. Fix its comment: it pins E2 and E3 for the two windows, Muted reports and Log out of all devices, and the Delete box is pinned by TestDeleteAccountConfirmBox.

c. Add TestDeleteAccountConfirmBox right after TestAccountConfirmWindows. Above it, a dated comment: Amended 2026-10-25, the owner said "yes exactly how the logout option is looking and it should work like any delete my account thing work", so Delete my account asks in a native dialog with the Log out box classes, Cancel and a red Delete; this replaces UI-SPEC E3's X only window and its dark filled button. The test reads the page with p421ReadTemplate and:
   - finds the exact opening tag `<dialog id="settings-delete-confirm" class="logout-confirm" aria-labelledby="settings-delete-title" aria-describedby="settings-delete-text">` and t.Fatalf if absent. Matching the whole tag also pins that the dialog carries neither hidden nor open, which in a real browser would keep a showModal dialog invisible or open at load;
   - takes the region from that tag to the first `</dialog>` after it;
   - requires in the region: class="logout-confirm__body"; the title `<h2 class="logout-confirm__title" id="settings-delete-title" tabindex="-1">Delete your account?</h2>`; `<div class="logout-confirm__text" id="settings-delete-text">`; `<p>This cannot be undone.</p>`; `<p>Your email, name, home area, updates, muted reports and settings are deleted, and you are logged out on every device.</p>`; the rule p with id settings-delete-rule; `<p class="inline-error" id="settings-delete-error" role="alert" hidden></p>`; `<div class="logout-confirm__actions">`; `<button type="button" class="btn" id="settings-delete-cancel">Cancel</button>`; `<button type="button" class="btn btn--destructive" id="settings-delete-go">Delete</button>`;
   - checks with p421InOrder that the region runs: This cannot be undone., then Your email, then settings-delete-rule, then settings-delete-error, then settings-delete-cancel, then settings-delete-go;
   - errors if the region holds aria-label="Close", app-icon--x, autofocus or btn--primary;
   - errors if the whole page still holds settings-delete-window or settings-delete-close (the old window is gone);
   - errors unless the opening tag comes after `</main>` and before `id="settings-name-window"` (a direct child of the body, never inside a hidden ancestor, and outside every window region).
Leave TestDeleteRuleSentenceIsOneString, TestAccountSectionOnBothSizes and TestSettingsAlertsScriptContract unchanged. Run gofmt on the file.

RED is expected to look like this: the whole Node file fails except the leaves quietly test, because the current script's early return still looks for the old window and bails on the new fixture. Do not change the script to make RED smaller. In Go, only TestDeleteAccountConfirmBox fails.

Commit, path limited: git add -- web/settings_alerts_test.mjs web/settings_rows_contract_test.go, then git commit with the message test(261025-kky): the Delete box asks like the Log out box, tests first, a blank line, and the trailer Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>, followed by -- and the same two paths.
  </action>
  <verify>
    <automated>! node --test web/settings_alerts_test.mjs && ! go test ./web/ -run '^TestDeleteAccountConfirmBox$' -count=1 && go test ./web/ -run '^(TestAccountConfirmWindows|TestDeleteRuleSentenceIsOneString|TestSettingsAlertsScriptContract|TestSettingsWindowsCloseWithXOnly)$' -count=1 && test -z "$(gofmt -l web/settings_rows_contract_test.go)"</automated>
  </verify>
  <done>The RED commit holds only the two test files. node --test web/settings_alerts_test.mjs exits non-zero; TestDeleteAccountConfirmBox fails; TestAccountConfirmWindows, TestDeleteRuleSentenceIsOneString, TestSettingsAlertsScriptContract and TestSettingsWindowsCloseWithXOnly pass.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: GREEN, the dialog markup, its CSS and the script</name>
  <files>web/templates/settings.html.tmpl, web/static/css/auth.css, web/static/js/settings-alerts.js</files>
  <behavior>
    - Every test written in Task 1 passes, and web/logout_confirm_test.mjs still passes unchanged
  </behavior>
  <action>
settings.html.tmpl (touch only these two spots; pinalert-60 shares this file):

1. Delete the whole `<div class="app-window app-window--edit" id="settings-delete-window" hidden>` block, from that line to its closing div, and the blank line after it.

2. Just BEFORE the line `<div class="app-window app-window--edit" id="settings-name-window" hidden>`, add a template comment that quotes the owner's words with the date: Owner 2026-10-25, "yes exactly how the logout option is looking and it should work like any delete my account thing work", so Delete my account asks in the same small box as Log out. The comment must not name the old window's ids, because TestDeleteAccountConfirmBox bans them page wide. Then add the dialog, indented like the windows: the exact opening tag pinned by the test (no hidden, no open, no autofocus), a div.logout-confirm__body holding: the h2 title exactly as pinned; the div.logout-confirm__text with id settings-delete-text holding p This cannot be undone., then the p Your email ... every device., then the existing Q2 template comment moved word for word from the old window, immediately followed by the existing settings-delete-rule paragraph moved word for word (TestDeleteRuleSentenceIsOneString needs the comment to end just before it, so put nothing but whitespace between them); after the text div, the error p exactly as pinned; then the div.logout-confirm__actions with the Cancel button and then the Delete button exactly as pinned. Then a blank line before the Name window.

auth.css (only around the .logout-confirm rules, lines 564 to 623; do not change any other rule):

3. In the existing .logout-confirm rule add max-height: calc(100% - 2 * var(--space-md)) and overflow-y: auto, with a dated comment (2026-10-25) saying the Delete box carries three paragraphs, so at 320px with 200 percent text it scrolls inside itself; the percentage is measured against the viewport because a modal dialog sits in the top layer, and a vh unit is avoided because the iPhone toolbar would cover the buttons. The Log out box is short, so it never reaches the cap.

4. In the existing .logout-confirm__actions:not([hidden]) rule add flex-wrap: wrap, with a comment that at normal text both buttons fit on one row so neither box changes, and at 200 percent text the two buttons stack instead of running out of the box.

5. After the .logout-confirm__actions .btn rule and before the forced-colors block, add one dated block (comment quoting the owner 2026-10-25 and saying these rules serve the Delete box's sentences and leave the Log out box as it is):
   - .logout-confirm__title:has(+ .logout-confirm__text) gets margin-bottom var(--space-sm), so the question sits close to its sentences (the page already relies on :has for the page lock);
   - .logout-confirm__text p gets margin 0 0 var(--space-sm), color var(--color-text), font-size var(--font-size-body), line-height var(--line-height-body); .logout-confirm__text p:last-child gets margin-bottom 0;
   - .logout-confirm .inline-error:not([hidden]) gets margin var(--space-md) 0 0 (this selector outranks chrome.css's .inline-error:not([hidden]), which loads later);
   - .logout-confirm__text ~ .logout-confirm__actions gets margin-top var(--space-lg), so only a box with text gets the extra space above its buttons; the Log out box keeps the title's own margin.
   No !important, no 999px, no new colour values, no dashes.

settings-alerts.js:

6. Header comment: line 1 and 2 say the Muted reports window, the Log out of all devices window and the Delete my account box. Rewrite the bullet that says a window only closes through the X or Escape so it keeps that rule for the windows and adds the exception: the Delete box (owner, 2026-10-25: "yes exactly how the logout option is looking and it should work like any delete my account thing work") is a native dialog like Log out, answered by its Cancel and Delete buttons, with Escape the same as Cancel except while the delete is being sent; it still has no handler on the dialog itself, so a tap on the dim page does nothing. Change the sections comment above logOutAll from the two confirm windows to the Log out of all devices window and the Delete box.

7. Replace deleteWin with deleteBox = getElementById('settings-delete-confirm'), and make the early return check mutedWin, logoutWin and deleteBox. Add deleteTitle (settings-delete-title) and deleteCancel (settings-delete-cancel) next to deleteRow, deleteError and deleteGo. Change LABEL_DELETE to Delete; LABEL_DELETE_BUSY stays Deleting…. Add canAskInBox, true when deleteBox.showModal is a function, and DELETE_QUESTION, the trimmed textContent of deleteTitle, or Delete your account? when that is empty, the same way account-menu.js builds LOGOUT_QUESTION.

8. Remove del from the windows map, remove the del branch from openWindow, and remove the del close and keydown wiring lines. Keep 'Escape' in onWindowKeydown for the two windows.

9. Add openDeleteConfirm: return at once while deleting. Without canAskInBox, ask window.confirm(DELETE_QUESTION) and call deleteAccount only when it returns true, then return. Otherwise close any open window first with closeWindow(current, false) when current is not null, hide the error line, call deleteBox.showModal(), then focus deleteTitle. Do not call lockPage: the native modal makes the page inert and the html:has rule in auth.css stops the scroll.

10. deleteAccount: as now it returns at once while deleting, sets deleting, hides the error line, disables deleteGo, sets the busy label, and posts with JSON.stringify({ confirm: true }). Also disable deleteCancel. Keep window.location.assign(DELETED_PAGE) on success; never write the address as a literal there, because TestSettingsAlertsScriptContract only accepts the two named constants. Its fail function clears deleting, enables both buttons, puts the label Delete back, then: without canAskInBox it calls window.alert(MESSAGE_DELETE_FAILED); with it, it first calls deleteBox.showModal() again when deleteBox.open is false (a browser may force the box shut on a second Escape even though cancel was refused) and then shows MESSAGE_DELETE_FAILED in the error line, and finally focuses deleteTitle. That last step matters in a real browser: showModal runs the browser's own focusing steps, which with no autofocus can land on Cancel (the owner rejected a box that looks pre selected), and in the plain failure path focus fell to the body when Delete was disabled under it. The error line has role alert, so it is still announced.

11. Wiring: the Delete row's click calls openDeleteConfirm. deleteCancel's click calls deleteBox.close() when not deleting. deleteGo's click calls deleteAccount. deleteBox gets a cancel listener that calls preventDefault on the event while deleting and otherwise leaves it alone, so the browser closes the box. deleteBox gets a close listener that puts focus back on deleteRow. Do not add a click listener to deleteBox.

12. Add openDeleteConfirm to window.PinalertSettingsAlerts; keep every existing handle.

Commit, path limited: git add -- the three paths, then git commit with the message feat(261025-kky): Delete my account asks in the same small box as Log out, a blank line, and the trailer Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>, followed by -- and the three paths.
  </action>
  <verify>
    <automated>node --test web/settings_alerts_test.mjs web/logout_confirm_test.mjs && go test ./web/ -count=1</automated>
  </verify>
  <done>The GREEN commit holds only the template, auth.css and settings-alerts.js. Both Node files pass, and go test ./web/ passes in full, including TestDeleteAccountConfirmBox, TestDeleteRuleSentenceIsOneString, TestSettingsAlertsScriptContract, TestSettingsWindowsCloseWithXOnly and every places contract test.</done>
</task>

<task type="auto">
  <name>Task 3: the dated UI-SPEC note and the whole suite</name>
  <files>.planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-UI-SPEC.md</files>
  <action>
1. In 03.2-UI-SPEC.md section E3, right after the Delete my account table (before the --- line that ends Surface E), add a blockquote that starts with Amended 2026-10-25 in bold, quotes the owner ("yes exactly how the logout option is looking and it should work like any delete my account thing work") and says: Delete my account no longer opens an app-window. It asks in a native dialog with the Log out box's look (the .logout-confirm classes): the question Delete your account?, the same three paragraphs as above, then Cancel and a red Delete button sharing the row; Delete reads Deleting… while it works and Cancel is disabled; a failure shows Couldn't delete your account. Try again. inside the box; focus starts on the question; Escape is the same as Cancel except while the delete is being sent; a tap on the dim page does nothing; success still lands on the sign in page. The button says Delete, not Delete my account, because at 320px each half width button is about 116px wide. This replaces, for this box only, the X only rule, the No Cancel rule (R2) and the dark filled button above. The row, the server route and the Log out of all devices window are unchanged. No dashes in the note.

2. Run the whole gate from the worktree root:
   - go build ./... and go vet ./...
   - go test ./web/ -count=1 (no database needed)
   - node --test web/*.mjs
   - the handlers tests on a disposable database you create and drop yourself: pick a number n, run createdb p32_kky_n, then env -i HOME="$HOME" PATH="$PATH" DATABASE_URL='postgres://localhost:5432/p32_kky_n?sslmode=disable' go test ./internal/api/handlers/ -count=1, then dropdb p32_kky_n even if the tests failed. Never point any test at pinalert_test or pinalert.
   If anything fails, fix the cause in the file that owns it (never in web/places_contract_test.go) and commit that fix path limited with the same trailer.

3. Commit the UI-SPEC alone, path limited: git add -- the UI-SPEC path, then git commit with the message docs(261025-kky): UI-SPEC E3 note, Delete my account asks like Log out, a blank line, the trailer Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>, then -- and the path.

Do not commit .planning/STATE.md. Do not push. Do not merge to main; the orchestrator merges after telling pinalert-60. Any server you start for a look uses an 87NN port, never 8080 or 8090.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./web/ -count=1 && node --test web/*.mjs && grep -c 'Amended 2026-10-25' .planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-UI-SPEC.md</automated>
  </verify>
  <done>The UI-SPEC carries the dated note in E3. go build, go vet, go test ./web/, node --test web/*.mjs and the handlers package on a disposable database all pass, and that database is dropped. Three commits (RED, GREEN, docs) and any fix commits sit on the worktree branch, not pushed and not merged.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser to POST /api/me/delete | a signed in person's request that removes their account; the server route and its checks are unchanged |
| owner's tap to the delete request | the confirm box is the last step before an action that cannot be undone |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-kky-01 | Denial of service (self, accidental deletion) | openDeleteConfirm, the dialog | high | mitigate | Focus starts on the question, never on Delete; a tap on the dim page and on the question does nothing (no click handler on the dialog); Escape and Cancel never send; pinned by the Node tests for focus, Cancel, Escape and the no click handler test |
| T-kky-02 | Tampering (double submit) | deleteAccount | low | mitigate | The deleting flag returns early and both buttons are disabled while the request waits; the test clicks Delete twice and expects one request |
| T-kky-03 | Spoofing (open redirect) | deleteAccount success | medium | mitigate | Navigation goes only through the DELETED_PAGE constant; TestSettingsAlertsScriptContract rejects any other location.assign argument |
| T-kky-04 | Tampering (markup injection) | the error line | low | mitigate | The failure sentence is a fixed constant written through textContent; innerHTML and outerHTML throw in the fake element and are banned by the script contract |
| T-kky-05 | Repudiation (box shut by the browser mid request) | fail in deleteAccount | low | mitigate | A failure that lands after the box was forced shut opens it again before showing the sentence, so the person always learns the account was not deleted; pinned by its own Node test |
</threat_model>

<verification>
- go build ./... and go vet ./... pass
- go test ./web/ -count=1 passes, with TestDeleteAccountConfirmBox new and TestAccountConfirmWindows covering two windows
- node --test web/*.mjs passes, including web/logout_confirm_test.mjs unchanged
- go test ./internal/api/handlers/ passes against a disposable database p32_kky_n that is then dropped
- web/places_contract_test.go, account-menu.js, account_header.html.tmpl and the server code are unchanged
- git log on the worktree branch shows a RED test commit before the GREEN feat commit, each path limited with the exact trailer
</verification>

<success_criteria>
- Delete my account opens a small box that matches the Log out box: Delete your account?, the three sentences, Cancel and a red Delete
- It works like any normal delete account confirm: Cancel or Escape backs out, Delete sends once and shows Deleting…, a failure says so inside the box, success lands on the sign in page with the deleted status
- Nothing else on Settings changes, and the diff in the two shared files stays in the regions named above
</success_criteria>

<output>
Create `.planning/quick/261025-kky-delete-my-account-asks-in-the-same-small/261025-kky-SUMMARY.md` when done. Commit it path limited with the same trailer, or leave it for the orchestrator if it asks to commit the summary itself.
</output>
