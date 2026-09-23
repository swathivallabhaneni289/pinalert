---
phase: quick-260923-qwi
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/theme_contract_test.go
  - web/css_contract_test.go
  - web/static/icons/sun.svg
  - web/static/icons/moon.svg
  - web/static/css/auth.css
  - web/static/js/theme.js
  - web/templates/account_header.html.tmpl
autonomous: true
requirements:
  - "Phase 2 UAT round 3 follow-up: replace the wordy Theme text label with a two-state icon-only toggle"
must_haves:
  truths:
    - "The account menu's theme control shows a sun glyph when the app is currently in light mode and a moon glyph when it is currently in dark mode, with no text label at all."
    - "One tap flips the app straight to the other mode. There is no third state and no follow-the-OS option anywhere in the shipped control."
    - "The control's accessible name and tooltip describe the action the tap performs (Switch to light mode / Switch to dark mode), and both update on every flip."
    - "A first-ever visitor with no stored preference gets a starting mode derived from the operating system preference exactly once, and that value is written to storage immediately, so no later load ever consults the operating system again."
    - "A corrupt or unrecognised stored value is never reflected into the root theme attribute; it falls back to the one-time derived value instead."
    - "Storage access stays wrapped in try/catch on both the read and the write, so a browser in a privacy mode never throws out of this module."
    - "theme.js still contains no app-shell reference, so it keeps loading unchanged on the login gate and verify-outcome pages."
    - "go build, go vet and go test ./... -short are all green with no database involved."
  artifacts:
    - "web/static/icons/sun.svg"
    - "web/static/icons/moon.svg"
    - ".auth-icon--sun and .auth-icon--moon mask rules in web/static/css/auth.css"
    - ".theme-toggle circular control rule plus hover, active and reduced-motion rules in web/static/css/auth.css"
    - "Two-mode theme.js in web/static/js/theme.js"
    - "Icon-only theme button in web/templates/account_header.html.tmpl"
    - "Updated TestThemeModuleValidatesStoredModeBeforeReflectingIt and TestAccountHeaderRendersThemeControl, plus a new TestThemeToggleIconAssetsExist, in web/theme_contract_test.go"
    - "sun.svg and moon.svg entries in css_contract_test.go's nonCategoryIcons map"
  key_links:
    - "css_contract_test.go's nonCategoryIcons map must list both new icon files, or TestCategoryGlyphMaskRulesCoverEveryCategory fails on an icon file no .icon-glyph rule claims. This is the non-obvious blast radius of adding any file to web/static/icons."
    - "The .theme-toggle rule must be placed AFTER the .account-menu__item rule in auth.css. Both selectors are single-class (equal specificity), so source order alone decides which width, padding, border, border-radius and background win."
    - "theme.js's ICON_CLASSES values must match the .auth-icon-- modifier names declared in auth.css, or the button renders an empty 44px circle with a green build."
    - "The control keeps id=theme-toggle and the glyph span keeps a stable id, because wireControl looks both up by id and returns inert if either is absent (the login gate and verify-outcome pages render no header)."
---

<objective>
Replace the account menu's wordy "Theme: Dark" / "Theme: Light" / "Theme: System" text control with a
two-state icon-only toggle: a 44px circular button showing a sun glyph in light mode and a moon glyph
in dark mode, tapping it flips straight to the other mode. The System (follow-OS) mode is dropped
entirely.

Purpose: the user called the current control wordy during Phase 2 UAT round 3, and explicitly chose a
true two-state switch over a three-way cycle. The operating system preference still seeds the very
first visit, but only once, after which the control behaves like an ordinary light switch.

Output: two new Lucide icon files, four new CSS rule groups in auth.css, a rewritten theme.js state
machine, one replaced button in the shared account header partial, and three updated or new contract
tests plus one allowlist entry.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@./.claude/CLAUDE.md
@web/static/js/theme.js
@web/theme_contract_test.go
@web/templates/account_header.html.tmpl
@web/static/css/auth.css
@web/static/icons/arrow-left.svg
</context>

<decisions>
Recorded so a later reviewer does not "fix" them back:

D-A. **The control stays inside #account-menu.** It keeps role="menuitem" and the account-menu__item
class, and is styled into a circle by a sibling modifier class. The user's "same visual family as
.account-trigger" wording constrains the styling tokens, not the DOM location, and this task's stated
file ownership does not include .app-header's layout. Moving it beside .account-trigger would turn the
header into a flex row, shift the #account-menu absolute-positioning origin, alter the geometry
profile_nav_contract_test.go's lock reasons about, and overturn the recorded DEC-S note in auth.css
that the header carries no extra slot content. Whether an in-menu circle reads well is a live-look
question for the human check below, not something to pre-solve by expanding scope here.

D-B. **Icon shows current state, accessible name describes the action.** Sun when currently light,
moon when currently dark; aria-label and title read "Switch to dark mode" when currently light. This
pairing is unusual but the user specified both halves directly.

D-C. **The one-time OS read is persisted immediately.** On a first-ever visit the derived value is
written to storage during module evaluation, not deferred to the first tap. This is a deliberate
change from the old behaviour, which left no storage trace for a reader who never touched the control.
The user's requirement is that once a value is set, by tap or by that single initial read, nothing
ever reverts to following the operating system.

D-D. **No negative-count test gate replaces the removed removeAttribute assertion.** The old
assertion counted exactly one removeAttribute call site; with follow-the-OS gone there is no such call
site. The assertion is dropped and the test's doc comment explains the single remaining DOM write
path, rather than being flipped to a zero-count gate on an unfiltered file.
</decisions>

<tasks>

<task type="auto">
  <name>Task 1: Rewrite the theme contract tests to the two-mode icon-only shape</name>
  <files>web/theme_contract_test.go</files>
  <action>
Read every test in the file first. Only the three below change; leave
TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage, TestThemeModuleHasNoAppShellDependency,
TestThemeModuleUsesNoMarkupParsingSink, TestThemeModuleGuardsStorageAccess and
TestThemeOverrideBlocksExistForBothModes untouched. TestThemeModuleGuardsStorageAccess requires at
least two try blocks and stays green because the rewritten module keeps a guarded read and a guarded
write; do not relax it. TestThemeOverrideBlocksExistForBothModes stays green because the two
data-theme override blocks and the dark media block in main.css all stay exactly as they are; only
the mechanism that sets data-theme changes.

Change 1, TestThemeModuleValidatesStoredModeBeforeReflectingIt. Keep the MODES identifier assertion,
keep the .indexOf( membership assertion, and keep the exactly-one setAttribute for the data-theme
attribute assertion. Delete the assertion that counts removeAttribute call sites for that attribute:
with follow-the-OS gone there is no branch that clears the attribute, so exactly one write path
remains. Do not replace it with a zero-count gate. Add one positive assertion instead: the module
must reference matchMedia exactly once, which is the single one-time operating-system read described
in D-C, and more than one occurrence would mean a code path still consults the OS on later loads.
Rewrite the doc comment to describe the two-element list, the single write path, and the one-time
read, per D-D.

Change 2, TestAccountHeaderRendersThemeControl. The control id constant stays. Replace the label span
id constant with the glyph span id chosen in Task 3 (theme-toggle-icon). Keep both the menuitem role
and the account-menu__item class window assertions: per D-A the control stays in the menu, and those
two staying green is the signal that scope did not expand. Add two window assertions, that the
control tag carries an aria-label attribute and a title attribute, since an icon-only control has no
visible text and those two carry the whole accessible name. Extend the existing cross-file loop that
checks theme.js addresses the ids read out of the template so it also checks the two icon modifier
class names, auth-icon--sun and auth-icon--moon: that turns the loop into a drift guard across all
three files, so a rename on the CSS side can only fail the build, never ship a button that renders an
empty circle. Update the doc comment, which currently describes a label span.

Change 3, add TestThemeToggleIconAssetsExist. Read both new icon files out of StaticFS and fail if
either is missing. Parse auth.css with this package's existing parseCSSRules helper (profile_nav_contract_test.go
already parses auth.css this way, reuse it rather than writing a second parser) and assert an exact
rule exists for each of the two icon modifier selectors, each declaring a mask-image whose url
resolves to the matching committed file under static/icons. Then lock the geometry the user asked
for: fetch the exact .theme-toggle rule and the exact .account-trigger rule and assert their height
declarations are equal, and assert .theme-toggle declares a fifty-percent border-radius, which is
what makes it a circle in the same visual family rather than a rounded rectangle. Write an honest
limit paragraph in the doc comment in the same spirit as this package's other CSS contract tests:
static inspection proves the rules and files line up, it cannot prove the glyphs read as a sun and a
moon at a glance.

Use plain punctuation throughout. No em dashes, no en dashes, anywhere in the comments or the failure
messages you write.

This task deliberately leaves the build red. That is the point: the three tests above must fail
against the current theme.js, auth.css and template before any of them is touched.
  </action>
  <verify>
    <automated>go vet ./web/ && ! go test ./web/ -short -run 'TestThemeModuleValidatesStoredModeBeforeReflectingIt|TestAccountHeaderRendersThemeControl|TestThemeToggleIconAssetsExist'</automated>
  </verify>
  <done>go vet passes, so the rewritten test file compiles. The three named tests fail, and their failure
output names the missing matchMedia reference, the missing glyph span id, and the missing icon files
or CSS rules rather than a compile error.</done>
</task>

<task type="auto">
  <name>Task 2: Add the sun and moon icons, their mask rules, and the circular control rule</name>
  <files>web/static/icons/sun.svg, web/static/icons/moon.svg, web/static/css/auth.css, web/css_contract_test.go</files>
  <action>
Create web/static/icons/sun.svg and web/static/icons/moon.svg. Match web/static/icons/arrow-left.svg
character for character on everything except the class name and the child elements: the same
lucide-static v1.41.0 ISC license comment as the first line, then an svg element carrying xmlns,
width of one hundred percent, height of one hundred percent, viewBox of 0 0 24 24, fill of none,
stroke of currentColor, stroke-width of 2, stroke-linecap of round, stroke-linejoin of round,
aria-hidden true and focusable false. The class attribute is "lucide lucide-sun" and "lucide
lucide-moon" respectively. These are stroke icons, not filled ones: verify that by reading
arrow-left.svg before writing, and do not invent a solid style.

The sun children are one circle at cx 12, cy 12, r 4, followed by eight single-segment paths whose d
values are, in this order: M12 2v2, then M12 20v2, then m4.93 4.93 1.41 1.41, then m17.66 17.66 1.41
1.41, then M2 12h2, then M20 12h2, then m6.34 17.66-1.41 1.41, then m19.07 4.93-1.41 1.41.

The moon is a single path whose d value is M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z.

In web/static/css/auth.css, add the two icon modifier rules immediately after the existing
.auth-icon--arrow-left rule, following that rule's exact shape: both the webkit-prefixed and the
unprefixed mask-image declaration, each pointing at the matching file under /static/icons/. The base
.auth-icon rule already supplies sizing, mask-size, mask-repeat and mask-position, so declare nothing
else.

Then add the control's own visual rule. It must be placed AFTER the .account-menu__item rule and
before the account menu form rule. This placement is load-bearing: both selectors are single-class,
so they have equal specificity and source order alone decides which declarations win. Name the class
theme-toggle. Never write it as a descendant of the panel id, because css_contract_test.go's hidden-guard
test does a broad substring check on any rule naming the panel, which is exactly why the sibling
.account-menu__item rule avoids that prefix too; there is a comment block above that rule explaining
this, read it before writing.

The .theme-toggle rule overrides the inherited menu-item row shape into a circle in the same visual
family as .account-trigger: center the flex content horizontally, set width, min-width and height all
to the shared touch-target token (.account-menu__item already declares that token as a min-height, so
the two agree by construction and the new test's geometry lock passes), zero the horizontal padding,
declare a one pixel border in the shared border colour, a fifty percent border-radius, the base
background token, and the same two-pixel eight-pixel twenty-percent-black box-shadow .account-trigger
and .profile-back-link both use. Add a background-color and border-color transition of 150ms ease,
matching .profile-back-link exactly.

Add three companions, all modelled on the .profile-back-link block directly below in the same file:
a descendant rule giving the hosted glyph the full text colour, because the base .auth-icon rule
defaults to the muted colour and an icon-only control needs full contrast (the same override
.account-trigger and .profile-back-link already carry); a hover rule swapping to the surface
background and the muted border colour; an active rule swapping to the border colour background. Then
add the control to a prefers-reduced-motion reduce block that sets its transition to none. Declare no
raw hex, pixel or font value that is not already a token, except the border width, radius and shadow
values the two precedent rules already use literally. Do not touch any other rule in the file.

Finally, in web/css_contract_test.go, add both new icon paths to the nonCategoryIcons map with a short
comment naming this quick task, in the same style as the arrow-left entry directly above. This is not
optional and is not cosmetic: TestCategoryGlyphMaskRulesCoverEveryCategory reverse-walks the embedded
icon tree and fails on any icon file no category mask rule claims, so adding two files to
static/icons without this entry turns that test red. It is the one piece of blast radius outside the
originally scoped file list.

Use plain punctuation in every comment you write. No em dashes, no en dashes.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./web/ -short -v -run 'TestThemeToggleIconAssetsExist|TestCategoryGlyphMaskRulesCoverEveryCategory|TestNoPillShapedControls|TestAccountMenuHiddenGuard|TestActivityPageLinksBackToTheMap'</automated>
  </verify>
  <done>Both icon files exist and are embedded. TestThemeToggleIconAssetsExist passes, including the
geometry lock against .account-trigger. TestCategoryGlyphMaskRulesCoverEveryCategory still passes,
proving the allowlist entries landed. The pill-shape, hidden-guard and back-link geometry tests are
untouched and still green.</done>
</task>

<task type="auto">
  <name>Task 3: Rewrite theme.js as a two-state toggle and replace the header markup</name>
  <files>web/static/js/theme.js, web/templates/account_header.html.tmpl</files>
  <action>
Rewrite web/static/js/theme.js into a two-value state machine. Keep the storage key string exactly as
it is, so an existing visitor's stored light or dark preference carries over untouched and a stored
third-state value simply fails validation and falls through to the one-time derived default.

The fixed list becomes a two-element list of light and dark. Add two lookup maps: one from the
current mode to its glyph modifier class name, light to the sun modifier and dark to the moon
modifier; one from the TARGET mode to its action sentence, light to "Switch to light mode" and dark
to "Switch to dark mode". Keying the sentence map by target, not by current state, is what makes the
accessible name describe the action, per D-B.

The storage read keeps its try/catch wrapper and its membership check against the fixed list, and now
returns a null signal when there is no valid stored value, instead of returning a follow-the-OS
default. A separate initial-mode function consumes that: if the read returned a valid value it is
used unchanged; otherwise, and only otherwise, the module reads the operating system dark preference
through matchMedia exactly once and derives dark or light from it. Guard the matchMedia lookup with a
plain existence check on the window property rather than a third try block. This must be the only
matchMedia reference in the file; Task 1's test counts it.

The apply function collapses to a single setAttribute call for the root theme attribute. Delete the
branch that removed the attribute: with follow-the-OS gone there is no state that wants the attribute
absent, which is exactly why Task 1 dropped that assertion. The persist function likewise collapses
to a single guarded setItem with no remove branch.

During module evaluation, still while the document head is parsing, resolve the initial mode, apply
it, and persist it immediately. Persisting here rather than waiting for the first tap is deliberate
and is D-C: it is what guarantees the operating system is consulted once ever, not once per visit.
Write a comment saying so, and say plainly that this is a change from the previous behaviour, which
left no storage trace for a reader who never touched the control.

The wire function looks up the control by its id and the glyph span by its id and returns immediately
if either is absent, keeping the existing inert-on-headerless-pages guard. Replace the label-writing
code with a small render step that removes both glyph modifier classes from the span and adds the one
for the current mode, then sets both the aria-label attribute and the title attribute on the control
to the action sentence for the mode the next tap would move to. Call the render step once at wire
time and again at the end of the click handler. The click handler advances through the fixed list by
index with a modulo wrap, which for a two-element list is exactly a flip and keeps the fixed list as
the single source of truth, then applies, persists and re-renders. Use classList and setAttribute
only; the module must still contain no markup-parsing sink.

Rewrite the file's header comment block. Keep the whole before-first-paint argument, which is still
the reason this script loads undeferred in the head, and keep the self-contained note explaining it
runs on pages where no other script exists. Update the security paragraph to say the list is now two
elements. Two hard constraints on the prose you write there: the product name must not appear
anywhere in this file in the capitalised form, because a contract test greps the shipped bytes for it
and fails the build on a hit, so refer to the app generically; and use plain punctuation with no em
dashes and no en dashes, even though no test catches dashes in this file.

Then replace the control in web/templates/account_header.html.tmpl. Delete the text node and the
label span. The button keeps its type, its id, its menuitem role, and gains the new control class
alongside the existing menu-item class, per D-A. Give it an aria-label and a title, both set to the
switch-to-light sentence, and a single child span carrying the glyph span id, the base icon class,
the moon modifier class, and aria-hidden true. Markup-side initial state is dark showing the moon,
which is self-consistent with the action sentence; the wire step overwrites all three of those values
on load anyway, and because the control lives inside a panel that ships hidden, there is no visible
flash of a wrong glyph while that happens. Plain punctuation only: the template dash scan covers this
file.
  </action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./web/ -short && go test ./... -short</automated>
    <human-check>Restart the dev server first, the CSS, JS and templates are embedded at build time, so nothing below is visible until a rebuild. Then open the account menu and check: does the glyph read as a sun and a moon at a glance, or is the moon's single stroked crescent noticeably lighter than the sun's circle-and-rays under the mask; does one tap flip the whole app cleanly with no flash; is the circle's contrast fine against the menu surface in BOTH themes; and does an unlabelled circle sitting in a menu of text rows look deliberate or look stranded (if it looks stranded, the follow-up is moving it beside the account button in the header, which D-A deliberately left out of scope).</human-check>
  </verify>
  <done>Whole suite green with no database set. The control renders as a circular icon-only button inside
the account menu, one tap flips the root theme attribute between the two values, the glyph and both
the aria-label and title update on every flip, and a reload restores the stored mode.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| localStorage to DOM | A stored string the user or another script can set crosses into a root attribute value |
| OS preference to DOM | The matchMedia result crosses into the same attribute, via a two-branch derivation |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-01-17 | Tampering | theme.js stored-mode read | low | mitigate | Existing control preserved: the stored string is membership-checked against the fixed two-element list before it can reach the single setAttribute call site, and an unrecognised value falls through to the derived default. Guarded by TestThemeModuleValidatesStoredModeBeforeReflectingIt. |
| T-01-03 | Tampering | theme.js DOM writes | low | mitigate | Every value reaching the DOM goes through classList or setAttribute, never a markup-parsing sink. Guarded by the untouched TestThemeModuleUsesNoMarkupParsingSink. |
| T-QWI-01 | Information disclosure | first-visit storage write (D-C) | low | accept | The module now writes a derived light or dark value to local storage on a first-ever visit, including on the login gate, where the old code left no trace. Accepted: the value is a two-valued display preference with no identifying content, and persisting it is precisely the user's stated requirement that the operating system is consulted once ever. |

No new dependency, no package install, no network call, no server-side change: this plan touches two
static assets, one stylesheet, one script, one template partial and two test files.
</threat_model>

<verification>
1. go build ./... and go vet ./... pass.
2. go test ./web/ -short passes with every test in theme_contract_test.go green, including the three
   rewritten or added in Task 1.
3. go test ./... -short passes, confirming no collateral damage to css_contract_test.go,
   profile_nav_contract_test.go, account_menu_contract_test.go or design_rules_contract_test.go.
4. No DATABASE_URL is set at any point and pinalert_test is never targeted; the user's own dev server
   on port 8090 owns that database.
5. Grep the two written source files for the em dash and en dash characters and confirm zero hits in
   theme.js. The template is covered automatically by TestUserVisibleCopyUsesPlainPunctuation, which
   whole-file scans every embedded template for both characters and is the automated gate for the
   plain-punctuation requirement on the markup side.
</verification>

<success_criteria>
- The account menu's theme control is an icon-only 44px circle with no text label.
- Sun glyph in light mode, moon glyph in dark mode, one tap flips between exactly two states.
- The System mode is gone from the shipped control, from the fixed mode list, and from every code
  path: nothing consults the operating system after the first-ever visit.
- aria-label and title both describe the action the tap performs and both update on every flip.
- Storage read and write both stay try/catch guarded, and an unrecognised stored value never reaches
  the root theme attribute.
- theme.js still contains no capitalised product name, so it keeps loading on the login gate and
  verify-outcome pages.
- Whole test suite green with no database.
</success_criteria>

<output>
Create `.planning/quick/260923-qwi-replace-the-wordy-theme-text-label-with-/260923-qwi-SUMMARY.md` when done.

The SUMMARY must separate what was verified computationally from what still needs a live look, per
the user's explicit instruction. Computationally verified: both icon files exist and are embedded,
the CSS mask rules resolve to real files, the geometry lock against the account button holds, theme.js
and the template and auth.css all address the same ids and class names, the JS state machine is a
validated two-element list with one DOM write path and one matchMedia read, and the whole suite is
green. Not verified by any headless Go test and genuinely needing a browser: whether the glyphs read
as a sun and a moon at a glance, whether the moon's lighter stroke weight is a problem under the mask,
whether the flip feels right, whether contrast holds in both themes, and whether an unlabelled circle
in a text menu looks deliberate (see D-A: moving it into the header was deliberately left out of
scope and is the obvious follow-up if it looks stranded).

State plainly in the SUMMARY that the dev server needs a restart before anyone can see any of this,
because the CSS, JS and templates are embedded into the binary at build time.
</output>
