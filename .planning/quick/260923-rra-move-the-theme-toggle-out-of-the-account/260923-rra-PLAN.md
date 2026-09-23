---
phase: quick-260923-rra
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/theme_contract_test.go
  - web/templates/theme_toggle.html.tmpl
  - web/templates/account_header.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/profile.html.tmpl
  - web/static/css/auth.css
autonomous: true
requirements:
  - "Phase 2 UAT round 3 follow-up: move the sun/moon theme toggle out of the account menu into an always-visible floating control stacked with the report button"
must_haves:
  truths:
    - "The theme control is visible on the main map page without opening any menu, as a 44px circle in the bottom-right corner, vertically stacked with the report button and separated from it by one space token."
    - "The same control appears at the identical bottom-right position on the Activity page, where there is no report button under it."
    - "The account menu contains exactly the email row, the Activity link and Log out, with no theme row and no empty leftover row."
    - "One tap still flips the app between light and dark, the glyph still swaps between sun and moon, and the accessible name and tooltip still update on every flip, because theme.js is not modified at all."
    - "The control's markup exists in exactly one file and is included by exactly two pages, so no page ever renders two elements carrying the same control id."
    - "The login gate and verify outcome pages, and the check inbox partial login_gate includes, render no theme control at all and keep loading theme.js unchanged, which stays inert there."
    - "The floating control never overlaps the report button, Leaflet's bottom-right attribution strip, Leaflet's top-left zoom control or the top-right account button, and it sits below the report modal's backdrop when the modal opens."
    - "The control's vertical offset is computed from the report button's own bottom token plus the shared touch-target token plus a space token, never a hardcoded pixel value."
    - "go build, go vet and go test ./... -short are all green with no database involved."
  artifacts:
    - "web/templates/theme_toggle.html.tmpl, the single shared partial holding the control's markup"
    - "One include line in web/templates/index.html.tmpl and one in web/templates/profile.html.tmpl"
    - "A theme control free web/templates/account_header.html.tmpl"
    - "A self-sufficient, fixed-position .theme-toggle rule in web/static/css/auth.css"
    - "TestThemeToggleRendersAsAFloatingControl in web/theme_contract_test.go, replacing TestAccountHeaderRendersThemeControl"
    - "TestThemeToggleFloatsClearOfTheReportButton in web/theme_contract_test.go, a computed geometry lock against .fab"
  key_links:
    - "Leaving the control in account_header.html.tmpl while also shipping the partial puts two elements with the same id on every gated page, and getElementById wires only the first, so one of the two circles becomes a dead button with a green build. The negative assertion in TestThemeToggleRendersAsAFloatingControl is the only thing that catches this."
    - ".theme-toggle currently free rides on .account-menu__item for display flex, align-items center and cursor pointer. Outside the menu it inherits none of them, so those three must be declared on the rule itself or the 24px glyph sits on the button's text baseline instead of centered in the 44px circle."
    - "The comment block above .theme-toggle in auth.css explains a source-order argument against .account-menu__item that stops being true the moment the control leaves the menu. It must be rewritten, not kept."
    - "template.ParseFS(web.TemplatesFS, \"templates/*.tmpl\") registers every .tmpl by base filename automatically, so the new partial needs no Go wiring. It must not contain an <html element, or findFullPageTemplates in theme_contract_test.go starts counting it as a fifth full page."
    - "The new file lives under templates/, so TestUserVisibleCopyUsesPlainPunctuation whole-file scans it for em dashes and en dashes from the moment it is created."
---

<objective>
Move the sun/moon theme toggle out of the account menu dropdown and into its own always visible
floating circular button in the bottom-right corner, stacked with the "+" report button on the map
page and floating at the same position on the Activity page.

Purpose: the user saw the control live immediately after quick task 260923-qwi shipped it inside the
account menu, and asked for it out of the menu and down beside the report button. A theme switch
buried behind a dropdown costs two taps for a one-tap action.

Output: one new shared template partial, two include lines, one removal, one rewritten CSS rule, and
two contract tests (one replacing an existing test, one new computed geometry lock). theme.js is not
touched at all: its wireControl already looks the control up by id and does not care where in the
document that id lives.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@./.claude/CLAUDE.md
@web/templates/account_header.html.tmpl
@web/templates/index.html.tmpl
@web/templates/profile.html.tmpl
@web/theme_contract_test.go
@web/profile_nav_contract_test.go
@web/static/css/auth.css
</context>

<decisions>
Recorded so a later reviewer does not "fix" them back:

**D-A. The floating control stacks ABOVE the report button, at a larger `bottom` value, not below it.**

This overrides the literal reading in the task constraints, and the reasoning is load bearing, so it
is recorded here and must also be written into a code comment in auth.css.

Three independent grounds:

1. *The source utterance is ambiguous, not clearly "below".* The raw quote is "put it over the below
   the uh, the plus option." It contains both "over" and "below", spoken mid-correction. Neither word
   can be treated as the settled instruction.

2. *A lower placement is geometrically impossible inside this task's scope.* `.fab` is
   `position: fixed; bottom: var(--space-lg)` with `height: var(--touch-target-min)`, which resolves
   to a band from 24px to 68px measured up from the viewport bottom edge. A 44px control placed under
   it would need its own top edge at or below 24px, putting its `bottom` at `-20px` even with a zero
   gap. It would be off screen. The only way to make room is to move `.fab` itself, which the task
   scope explicitly forbids and which would also relocate the primary action a user already has
   muscle memory for.

3. *A lower placement would land on Leaflet's attribution strip.* No stylesheet in this repo
   overrides Leaflet's control positions, so the attribution control sits at the default bottom-right,
   flush with the bottom edge. The report button already clears it by sitting at 24px. A control
   placed under the report button would cover it, which is both ugly and an attribution problem.

The chosen value is `bottom: calc(var(--space-lg) + var(--touch-target-min) + var(--space-sm))`,
which resolves to 24 + 44 + 8 = 76px, giving a toggle band of 76px to 120px and a clean 8px gap above
the report button's 68px top edge. This is also the standard floating-action-button stack convention:
secondary actions stack upward from the primary one.

If the user genuinely meant physically lower on the screen, the follow-up is a swap (toggle at
`var(--space-lg)`, report button moved up to the 76px slot), which is a one-line change to each rule
but touches `.fab`, and so is deliberately out of this task's scope. The human check below asks this
question directly.

**D-B. The partial is named `theme_toggle.html.tmpl`, not `theme_toggle_fab.html.tmpl`.**

`.fab` is main.css's class for the report button specifically. Naming the file after it would imply
the control uses that class, which it does not: it keeps its own `.theme-toggle` class and its own
distinct visual treatment (bordered circle on `--color-bg`, versus the report button's solid inverted
`--color-text` fill). Snake case with a `.html.tmpl` suffix matches `account_header.html.tmpl`.

**D-C. One rule, one `bottom` value, both pages. No page-scoped override for the Activity page.**

The user asked for "the same fixed bottom-right position" on the Activity page. Reading that plainly
gives one rule with no page qualifier, which also means the control does not jump position when the
user navigates between the map and Activity. The cost is that on the Activity page it floats 76px up
with nothing under it. Whether that reads as stranded is a live-look question, raised in the human
check, not something to pre-solve with a second magic value.

**D-D. theme.js is not modified.** `wireControl` resolves `#theme-toggle` and `#theme-toggle-icon`
through `getElementById`, which is document scoped and indifferent to where the ids live. Both pages
already load the script in their head. Nothing in the module needs to change, and touching it would
put the shipped two-state machine at risk for no gain.

**D-E. The removal from `account_header.html.tmpl` is a plain deletion.** No comment, no placeholder,
no empty row is left behind, per the user's "do not leave a dead/empty row". The menu returns exactly
to its pre-qwi shape: email row, Activity link, Log out form.
</decisions>

<tasks>

<task type="auto">
  <name>Task 1: Rewrite the theme contract tests to the floating-control shape</name>
  <files>web/theme_contract_test.go</files>
  <action>
Read the whole file before editing. Only one existing test changes. Leave
TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage, TestThemeModuleHasNoAppShellDependency,
TestThemeModuleUsesNoMarkupParsingSink, TestThemeModuleValidatesStoredModeBeforeReflectingIt,
TestThemeModuleGuardsStorageAccess, TestThemeToggleIconAssetsExist and
TestThemeOverrideBlocksExistForBothModes exactly as they are. In particular do not weaken
TestThemeToggleIconAssetsExist: its two locks (the control's height must equal the account button's
height, and the control's border-radius must be fifty percent) both survive this change untouched,
because the control keeps its circular shape and only moves.

Add one path constant next to the existing themeJSPath and accountHeaderPath constants, naming the
new partial at templates/theme_toggle.html.tmpl. Keep accountHeaderPath: it is still needed, for the
negative assertion below.

Change 1. Replace TestAccountHeaderRendersThemeControl with TestThemeToggleRendersAsAFloatingControl.
Same purpose, new location. It must assert all of the following, in this order, because a later
assertion passing vacuously after an earlier one fails is the failure mode this decomposition exists
to prevent. Use t.Fatalf for the existence checks and t.Errorf for the content checks, matching the
file's existing style.

  a. The partial reads out of TemplatesFS without error.
  b. The control id attribute string appears exactly once in the partial, and the glyph span id
     attribute string appears exactly once. Counts, not presence, so a duplicated block inside the
     partial itself is caught.
  c. Slice the control's opening tag with the existing findTagWindow helper (defined in
     template_contract_test.go, reuse it, do not write a second scanner) and assert the window carries
     a type attribute set to button, an aria-label attribute, and a title attribute. An icon-only
     control has no visible text, so those two attributes carry the whole accessible name.
  d. The partial contains the base icon class and the moon modifier class, which is the markup-side
     initial glyph theme.js overwrites on load.
  e. The partial does NOT contain the account menu item class. Outside the menu that class styles
     nothing and its presence would mean the migration was only half done.
  f. THE CRITICAL NEGATIVE ASSERTION: the shared account header partial no longer contains the
     control id attribute string at all. Without this, a copy-paste that adds the new partial while
     leaving the old button in place ships two elements with the same id on every gated page,
     getElementById wires only the first, and one of the two circles becomes a dead button with a
     fully green build. Write the failure message so it says exactly that.
  g. The include action for the new partial appears exactly once in templates/index.html.tmpl and
     exactly once in templates/profile.html.tmpl. Build the expected include string from the path
     constant rather than typing the filename a second time, so a rename can only fail the build and
     never let the two sides drift.
  h. That same include action appears zero times in templates/login_gate.html.tmpl,
     templates/check_inbox.html.tmpl and templates/verify_outcome.html.tmpl. Note the exact shapes
     here so you target the right files: login_gate and verify_outcome are full pages, while
     check_inbox is a partial that login_gate itself includes, so it reaches a reader only through
     that page and opens no html element of its own. All three render no header and no theme control
     today, and this assertion is what keeps that true. Read each with TemplatesFS.ReadFile and fail
     loudly if any of the three cannot be read, so the zero-count check can never pass merely because
     the file was not found.
  i. Keep the existing cross-file drift loop unchanged: theme.js must still reference the control id,
     the glyph span id, and both icon modifier class names. This is what catches a rename on one side
     shipping a button that renders an empty circle.

Write a doc comment that states what this test proves and what it cannot. It proves the markup lives
in exactly one file, is included by exactly the two pages that should have it, and is addressed by
the same identifiers theme.js uses. It cannot prove that a browser composes the two templates into a
single document with one unique id, because ParseFS composition is not rendered here, which is why
the per-file counts plus the negative assertion together stand in for that.

Change 2. Add TestThemeToggleFloatsClearOfTheReportButton, a computed geometry lock modelled on the
two locks already in profile_nav_contract_test.go. Parse main.css and auth.css with this package's
existing parseCSSRules, ruleBySelector and declsOf helpers. Do not write a new parser.

First build a token table: fetch the exact :root rule from main.css and read every declaration whose
name begins with the space-token prefix plus the shared touch-target token, parsing each value as an
integer number of pixels by trimming the px suffix. Fail loudly if the touch-target token or the
large space token cannot be resolved, because every later assertion depends on them.

Then write one small helper local to this test that resolves a length expression to an integer:
regexp-find every var reference in the expression, look each name up in the token table, and sum
them. Reject the expression and fail the test if it resolves to zero var references, or if it
contains any px literal, or if it contains a minus, asterisk or slash character. That rejection is
the no-magic-number gate the task constraints ask for: a hardcoded seventy-six pixel value has no var
references and fails on the spot, and a value built with arithmetic other than addition is refused
rather than silently mis-summed. This same helper resolves the report button's plain single-token
bottom value and the control's calc expression, so both sides go through identical code.

With that in place, fetch the exact .fab rule from main.css and the exact .theme-toggle rule from
auth.css, then assert:

  - The control declares a position of fixed.
  - The control's right value is string-equal to the report button's right value, so the two share one
    vertical line. Compare the declarations as written, not the resolved numbers, because sharing the
    same token is the intent.
  - The control's bottom value starts with calc and, once resolved through the helper, is a positive
    integer. Assert separately that its text references the shared touch-target token by name, which
    is what makes the offset derived from the report button's own height rather than coincidentally
    equal to it.
  - Both z-index values parse as integers and the control's is greater than or equal to the report
    button's. Say in the failure message that a lower value would let the report button's own
    stacking context cover the control.
  - THE BAND CHECK, which is the assertion that actually catches a wrong number: resolve the report
    button's bottom and height and the control's bottom and height into two vertical bands measured up
    from the viewport bottom edge, each running from its bottom value to its bottom value plus its
    height. Assert the two bands do not intersect. Then assert the gap between them, meaning the
    control's bottom minus the report button's band top, is strictly greater than zero and no larger
    than the resolved value of the large space token. A gap of zero means the two circles touch, and a
    gap larger than that token means they no longer read as one stack. Log both bands and the gap with
    t.Logf so a future failure shows the arithmetic rather than just a boolean.
  - THE CASCADE ORPHAN GUARD: the control's rule declares display, align-items, justify-content and
    cursor in its own body. Outside the account menu it inherits none of these from the menu item
    rule, and without the first three the 24px glyph sits on the button's text baseline instead of
    centered in the 44px circle, which is a visible defect with a green build. Name that consequence
    in the failure message.

Write an honest-limit paragraph in the doc comment in the same spirit as this package's other CSS
contract tests: static arithmetic over the shipped declarations proves the two controls cannot
overlap and that the offset is derived rather than hardcoded. It cannot prove a browser paints them
as a deliberate-looking pair, that the result reads the way the user meant, or that nothing else on
the page collides at a narrow viewport.

Use plain punctuation throughout. No em dashes and no en dashes anywhere in the comments or the
failure messages you write.

This task deliberately leaves the build red. Both new tests must fail against the current templates
and CSS before either is touched.
  </action>
  <verify>
    <automated>go vet ./web/ && ! go test ./web/ -short -run 'TestThemeToggleRendersAsAFloatingControl|TestThemeToggleFloatsClearOfTheReportButton'</automated>
  </verify>
  <done>go vet passes, so the rewritten test file compiles with no unused constant and no unused import.
Both named tests fail, and their output names the missing partial file and the missing position
declaration rather than a compile error. Commit this red state on its own as a test-prefixed commit
and continue: these two failures are the expected RED half of this plan, not a gate violation. Every
other test in the package must still be green, including TestThemeToggleIconAssetsExist and
TestAccountMenuHiddenGuard, so if anything beyond those two fails, that IS a blocker.</done>
</task>

<task type="auto">
  <name>Task 2: Extract the control into a shared partial and rewire the two pages</name>
  <files>web/templates/theme_toggle.html.tmpl, web/templates/account_header.html.tmpl, web/templates/index.html.tmpl, web/templates/profile.html.tmpl</files>
  <action>
Create web/templates/theme_toggle.html.tmpl holding exactly one button element and nothing else. Copy
the button currently sitting in account_header.html.tmpl and change three things about it: drop the
menuitem role, drop the account menu item class so only the control's own class remains, and keep
everything else byte-identical, meaning the type attribute, the control id, the aria-label and title
both reading the switch-to-light sentence, and the single child span carrying the glyph span id, the
base icon class, the moon modifier class and aria-hidden true. The markup-side initial state stays
dark-showing-the-moon; theme.js's render step overwrites the glyph class and both accessible-name
attributes on load, before the user can interact.

Three hard constraints on this new file:

  - It must NOT contain an html element. findFullPageTemplates in theme_contract_test.go treats any
    template containing that string as a full page and requires at least four to exist; a partial that
    accidentally looks like a page would silently join that set.
  - It lives under templates/, so TestUserVisibleCopyUsesPlainPunctuation whole-file scans it for em
    dashes and en dashes from the moment it exists. Use plain punctuation in every attribute value and
    in any comment.
  - No Go wiring is needed. internal/api/handlers/page.go calls template.ParseFS over templates/*.tmpl
    and names each parsed template after its base filename, so the file is registered by existing
    code. Do not add it to any list.

Then delete the control from web/templates/account_header.html.tmpl. Per D-E this is a plain
deletion of the whole button element: no comment, no placeholder, no empty row. The menu returns to
the email paragraph, the Activity anchor, and the logout form, in that order, which is exactly its
pre-qwi shape. Do not touch the header element, the account trigger button, the panel div, its hidden
attribute or its role.

Then add the include to both pages. In web/templates/index.html.tmpl and
web/templates/profile.html.tmpl, add a template action for the new partial passing the same dot,
placed at body level on the line immediately after the existing account header include on each page.
The two lines must be byte-identical to each other, which is what Task 1's per-page count assertions
check. Body level, not inside the map pane or the main element, for two reasons: the Activity page
has no equivalent container, so body level is the only placement that is identical on both pages, and
it keeps the fixed-position control out of any nested stacking context a pane might introduce.

Do not add the include to login_gate.html.tmpl, check_inbox.html.tmpl or verify_outcome.html.tmpl.
login_gate and verify_outcome are full pages; check_inbox is a partial login_gate itself includes.
All three render no header today and must keep rendering no control; Task 1 asserts a zero count on
each of them. The two pages continue to load theme.js unchanged, where wireControl's existing early
return on a missing element keeps the module inert, exactly as it already is today.

Do not touch web/static/js/theme.js in this task or any other. Per D-D, getElementById is document
scoped, so the module keeps working unmodified once the ids move.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./web/ -short -run 'TestThemeToggleRendersAsAFloatingControl|TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage|TestUserVisibleCopyUsesPlainPunctuation|TestAccountMenuHiddenGuard|TestActivityPageLinksBackToTheMap'</automated>
  </verify>
  <done>TestThemeToggleRendersAsAFloatingControl passes, including the negative assertion proving the control
is gone from the account header and the zero-count assertions on the three headerless pages.
TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage still passes, proving the new partial did not join
the full-page set. The plain-punctuation scan passes over the new file. The account menu hidden guard
and the Activity back-link geometry lock are untouched and still green.
TestThemeToggleFloatsClearOfTheReportButton is still expected to fail at this point, because the CSS
has not moved yet. That is the only remaining red.</done>
</task>

<task type="auto">
  <name>Task 3: Reposition the control rule into a self-sufficient floating circle</name>
  <files>web/static/css/auth.css</files>
  <action>
Rewrite the .theme-toggle rule in web/static/css/auth.css. Read the existing rule and the comment
block directly above it first, then read the .fab rule in web/static/css/main.css and the
.profile-back-link block further down auth.css, because this rule ends up sharing properties with
both.

The rule keeps its selector and its visual identity, and gains two things: fixed positioning, and
every property it used to free ride on from the account menu item rule.

Positioning, per D-A. Declare position fixed. Declare right using the same single space token the
report button's own right declaration uses, so the two controls share one vertical line. Declare
bottom as a calc summing three tokens: that same space token the report button uses for its own
bottom, plus the shared touch-target token, plus the small space token. Nothing else in that
expression: no px literal, no subtraction, no multiplication. Declare a z-index equal to the report
button's, which is one thousand, and carry over the intent of that rule's own trailing comment about
sitting above Leaflet's control panes. One thousand is also deliberately below the account header's
value and below the report modal's backdrop, so the control does not float over an open modal.

The cascade orphan fix. The old rule assumed the account menu item rule supplied display flex,
align-items center, cursor pointer, a color and a min-height. None of that reaches the control any
more. Declare all of them on the rule itself: display flex, align-items center, justify-content
center, cursor pointer, the full text color token (the control also keeps its existing descendant
rule giving the hosted glyph that same token, since the base icon rule defaults to the muted color,
so leave that descendant rule alone), and the shared touch-target token as min-height alongside the
width, min-width and height declarations it already carries. Missing the first three is not a subtle
regression: the 24px glyph would sit on the button's text baseline instead of centered in the 44px
circle, visibly off center, with a green build.

Everything visual stays exactly as shipped and must not be redesigned: zero padding, a one pixel
border in the shared border color, a fifty percent border-radius, the base background token, the same
two-pixel eight-pixel twenty-percent-black box-shadow, and the 150ms ease transition on background
color and border color. Leave the hover rule, the active rule and the reduced-motion rule untouched;
all three still apply unchanged. Do not touch the report button's own rule in main.css, and do not
touch any other rule in auth.css.

Rewrite the comment block above the rule. The current one argues at length that the rule must sit
after the account menu item rule because both selectors are single-class and source order decides the
winner. That argument dies with this change and would actively mislead the next reader. The
replacement comment must carry three things forward:

  1. That the control is now a fixed-position floating button stacked with the report button, and is
     no longer a menu row.
  2. The full D-A reasoning for stacking above rather than below, in its own words: the source
     utterance contained both "over" and "below" mid-correction; a lower placement is geometrically
     impossible because the report button's band already runs from the large space token up by one
     touch-target height, leaving a 44px control nowhere to go but off screen; and a lower placement
     would cover Leaflet's bottom-right attribution strip. State the resolved arithmetic explicitly so
     a future reader does not have to redo it.
  3. That the offset is expressed in tokens on purpose, so changing the report button's own bottom
     token moves both controls together, and that the geometry test in web/theme_contract_test.go
     fails the build if the two bands ever overlap.

You may leave the rule where it currently sits in the file or move it out of the account menu section
into its own section; ruleBySelector is order independent, so no test cares either way. If you move
it, move only this rule and its three companions together and leave a one-line pointer where it used
to be.

Use plain punctuation in every comment you write. No em dashes and no en dashes. Declare no raw hex,
pixel or font value that is not already a token, except the border width, the radius, the shadow
values and the transition duration, all of which the precedent rules in this file already use
literally.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./web/ -short && go test ./... -short</automated>
    <human-check>Restart the dev server first. The CSS, JS and templates are embedded into the binary at build time, so none of this is visible until a rebuild. Then, on the map page: (1) THE DECISION QUESTION, does the toggle sitting above the "+" read as what the user asked for, or did they mean it physically lower on the screen, in which case the follow-up is swapping the two offsets, which touches the report button's rule and was left out of scope on purpose (see D-A); (2) does the pair read as one deliberate stack, or as two unrelated floating circles; (3) at 375px phone width, does anything collide, check the map's attribution strip along the bottom edge, Leaflet's zoom control in the top-left, and the account button in the top-right; (4) is the glyph actually centered in the circle, this is the cascade-orphan failure mode and it is the first thing to look at; (5) open the report modal and confirm the toggle sits behind the backdrop rather than floating over it; (6) on the Activity page, the toggle floats at the same height with nothing under it, does that look stranded, and if so the follow-up is a page-scoped bottom override, which D-C deliberately did not pre-solve; (7) one tap still flips the whole app cleanly in both places.</human-check>
  </verify>
  <done>Whole suite green with no database set, including TestThemeToggleFloatsClearOfTheReportButton's band
check and cascade-orphan guard. The control renders as a fixed 44px circle in the bottom-right corner
of both the map page and the Activity page, its glyph is centered, one tap flips the root theme
attribute, and the account menu shows only the email, Activity and Log out.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| localStorage to DOM | A stored string the user or another script can set crosses into a root attribute value. Unchanged by this task: theme.js is not modified. |
| Template composition to DOM | Two separately parsed templates compose into one document, where an id collision would be invisible to any per-file check. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-01-17 | Tampering | theme.js stored-mode read | low | mitigate | Unchanged and unmodified by this task. The stored string is still membership checked against the fixed two-element list before reaching the single setAttribute call site. Still guarded by TestThemeModuleValidatesStoredModeBeforeReflectingIt, which this plan leaves untouched. |
| T-01-03 | Tampering | theme.js DOM writes | low | mitigate | Unchanged and unmodified. Every value reaching the DOM still goes through classList or setAttribute. Still guarded by the untouched TestThemeModuleUsesNoMarkupParsingSink. |
| T-RRA-01 | Tampering | template composition, duplicate control id | low | mitigate | Splitting the markup across a new partial while leaving the original in place would put two elements carrying the same control id into every gated page. getElementById would wire only the first, leaving a second, permanently dead circle with a fully green build. Mitigated by the negative assertion in TestThemeToggleRendersAsAFloatingControl (Task 1, item f) plus the exactly-once counts inside the partial and on each including page. |
| T-RRA-02 | Denial of service | floating control overlapping the report button | low | mitigate | A wrong offset could cover the "+" button, which is the primary action for filing an emergency report. Mitigated by the computed band check in TestThemeToggleFloatsClearOfTheReportButton, which resolves both controls' offsets and heights from the shipped tokens and fails on any intersection, and by the z-index assertion. |

No new dependency, no package install, no network call, no server-side change, no new route and no
new data flow. This plan touches one stylesheet, three templates, one new template partial and one
test file.
</threat_model>

<verification>
1. go build ./... and go vet ./... pass.
2. go test ./web/ -short passes with every test in theme_contract_test.go green, including both tests
   written in Task 1.
3. go test ./... -short passes, confirming no collateral damage to css_contract_test.go,
   profile_nav_contract_test.go, account_menu_contract_test.go, template_contract_test.go or
   design_rules_contract_test.go.
4. No DATABASE_URL is set at any point and pinalert_test is never targeted. The user's own dev server
   on port 8090 owns that database. Port 8080 is never used for this project locally.
5. The plain-punctuation requirement on the markup side is covered automatically by
   TestUserVisibleCopyUsesPlainPunctuation, which whole-file scans every embedded template, including
   the new partial. For auth.css, grep the file for the em dash and en dash characters and confirm
   zero hits, since no test covers dashes in stylesheets.
6. grep the shipped account header partial and confirm it contains no occurrence of the control id.
</verification>

<success_criteria>
- The theme control is visible on the map page and the Activity page without opening any menu.
- It is a fixed 44px circle in the bottom-right corner, sharing one vertical line with the report
  button and separated from it by one space token, with the glyph centered inside it.
- Its vertical offset is a calc over three design tokens, never a hardcoded pixel value, and a
  contract test fails the build if the two controls' bands ever overlap.
- The account menu contains only the email row, the Activity link and Log out, with no leftover row.
- The control's markup exists in exactly one file, is included by exactly two pages, and appears zero
  times in the login gate page, the verify outcome page and the check inbox partial.
- theme.js is byte-identical to what shipped in quick task 260923-qwi, and one tap still flips the app
  between exactly two modes with the glyph, aria-label and title all updating.
- Whole test suite green with no database.
</success_criteria>

<output>
Create `.planning/quick/260923-rra-move-the-theme-toggle-out-of-the-account/260923-rra-SUMMARY.md` when done.

The SUMMARY must separate what was verified computationally from what genuinely needs a live look, per
the user's explicit instruction.

Computationally verified: the markup lives in exactly one partial and is included exactly once by each
of the two intended pages and zero times by the three headerless pages; the control id is absent from
the account header; the offset is a token-derived calc with no magic number; the two controls' vertical
bands provably cannot intersect and the gap between them is one space token; the control's z-index is
at least the report button's; the rule declares its own display, alignment and cursor rather than
relying on the menu row it no longer lives in; the circle geometry still matches the account button;
and the whole suite is green with no database.

Not verified by any headless Go test and genuinely needing a browser, in priority order: whether the
toggle sitting ABOVE the "+" is what the user actually meant by "below" (D-A records why a lower
placement is geometrically impossible without moving the report button, and names the swap as the
follow-up if the answer is no); whether the glyph is truly centered in the circle; whether the pair
reads as one deliberate stack rather than two stranded circles; whether anything collides at 375px
phone width, specifically the map attribution strip, Leaflet's zoom control and the account button;
whether the control correctly sits behind the report modal's backdrop; and whether the Activity page's
lone floating circle at the same height looks intentional (D-C records why no page-scoped override was
added).

State plainly in the SUMMARY that the dev server needs a restart before anyone can see any of this,
because the CSS, JS and templates are embedded into the binary at build time.
</output>
