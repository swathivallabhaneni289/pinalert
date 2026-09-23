---
phase: 2
plan: "02-14"
subsystem: design-rules-gap-closure
tags: [copy, css, design-rules, gap-closure, wave-9]
dependency-graph:
  requires: ["02-10"]
  provides:
    - "TestUserVisibleCopyUsesPlainPunctuation (web/design_rules_contract_test.go): standing gate, no em/en dash in any embedded template"
    - "TestNoPillShapedControls (web/design_rules_contract_test.go): standing gate, fully-rounded radius allowed only on the severity slider selectors"
    - "agreedRateLimitSentence wording, identical across four files"
  affects:
    - "web/templates/profile.html.tmpl, verify_outcome.html.tmpl, login_gate.html.tmpl, check_inbox.html.tmpl, index.html.tmpl"
    - "web/static/js/auth.js"
    - "internal/mailer/resend.go, internal/ratelimit/perip.go, internal/api/handlers/auth.go"
    - "web/static/css/main.css (.view-toggle responsive rule, #toast)"
tech-stack:
  added: []
  patterns:
    - "Derive a contract test's subject set from an embedded FS (TemplatesFS/StaticFS) rather than hardcoding a file list, per this package's established house style"
    - "Reuse web/css_contract_test.go's parseCSSRules/declsOf/ruleBySelector rather than a second CSS parser"
key-files:
  created:
    - web/design_rules_contract_test.go
  modified:
    - web/templates/profile.html.tmpl
    - web/templates/verify_outcome.html.tmpl
    - web/templates/login_gate.html.tmpl
    - web/templates/check_inbox.html.tmpl
    - web/templates/index.html.tmpl
    - web/static/js/auth.js
    - internal/mailer/resend.go
    - internal/mailer/resend_test.go
    - internal/ratelimit/perip.go
    - internal/ratelimit/perip_test.go
    - internal/api/handlers/auth.go
    - web/static/css/main.css
decisions:
  - "Page-title separator: colon (\"Pinalert: Activity\"), applied consistently to all three retitled templates"
  - "Rate-limit sentence, agreed once and reused character-for-character in all four copies: \"Too many requests. Try again in a minute.\""
  - "Interim de-pill radius: 8px, matching the category tile and the account menu panel rather than inventing a third value"
  - "The three rate-limit sentence copies were NOT deduplicated into a shared constant; left as a documented follow-up (see below)"
metrics:
  duration: "~1.5 hours"
  completed: "2026-09-23"
status: complete
---

# Phase 2 Plan 14: Design-rules gap closure, dashes and pill radii Summary

Rewrote ten em-dash occurrences across nine files into plain punctuation and de-pilled two
fully-rounded controls (the view toggle, the toast) to an 8px interim radius, closing the two
live, mechanically-fixable violations of `site-design-rules.md` found during Phase 2 UAT gap
closure round 2. Both fixes are now gated by standing tests
(`TestUserVisibleCopyUsesPlainPunctuation`, `TestNoPillShapedControls`) so the cleanup cannot
silently erode.

## What Was Built

**Task 1 (client-side copy + template gate):** Rewrote the three page titles (colon separator,
"Pinalert: Activity" / "Pinalert: Verify your email"), the login gate's one-time-link sentence,
the check-inbox device sentence, the location-off notice, and auth.js's 429 rate-limit error,
all in ordinary punctuation with meaning preserved. Added `web/design_rules_contract_test.go`
with `TestUserVisibleCopyUsesPlainPunctuation` (whole-file dash scan derived from `TemplatesFS`,
fatals if fewer than 5 `.tmpl` files are found) and a second, narrowly targeted positive
assertion for `auth.js` (read from `StaticFS`, since that file's own SECURITY comments
legitimately keep dashes and a whole-file negative scan would be wrong there).

**Task 2 (server-side copy, three independent copies of one sentence):** Rewrote the mailer's
verification-email disclaimer, the rate-limiter's 429 message constant, and the auth handler's
own inline copy of the same rate-limit sentence, all to the wording agreed in Task 1. Updated the
two existing tests (`resend_test.go`, `perip_test.go`) that assert the old strings verbatim.

**Task 3 (de-pill two controls, gate by selector not by count):** Changed the view toggle's
responsive rule and the toast from `border-radius: 999px` to `8px`, with a two-line comment on
each recording why 8px (matches two existing controls) and that the deferred UI phase may
re-tokenise it. Added `TestNoPillShapedControls`, which walks every rule in `main.css` and allows
the fully-rounded radius only on selectors naming the severity slider tracks (a slider track is
not a pill-shaped button), rather than banning the value outright, which would have wrongly
failed on the slider's own correct CSS.

**Deviation fix (style, Rule 1):** After Task 3's commit, found that the new CSS comments and the
new test file's own doc comments and failure-message strings used em dashes, which a plan whose
entire purpose is removing em dashes should not introduce in its own new code. Fixed in a small
follow-up commit; the two data literals the test defines as its forbidden character set were
left untouched (they are the character being tested for, not prose), and no pre-existing dash in
an untouched comment elsewhere in `main.css` or the rest of the codebase was touched (those are
correctly out of scope, per Task 2's own instruction to leave doc comments alone).

## The Tenth Occurrence

The debug session's own sweep found nine em-dash occurrences across eight files. Verifying this
plan against the live source before writing Task 2 found a tenth: `internal/api/handlers/auth.go`
carries a **third, independent copy** of the rate-limit sentence, written out inline rather than
referencing the rate-limiter package's constant. It has no test asserting it and was not in the
debug session's own list. All three copies (rate-limiter package, auth handler, client script)
now read identically. **Deduplicating the three copies into a shared constant was deliberately
NOT done in this plan** — it would mean exporting the rate-limiter's message constant and
creating a package dependency from `internal/api/handlers` to `internal/ratelimit` purely to fix
a copy-paste duplication, which is a small refactor with its own review surface. A comment beside
the auth handler's literal records the duplication so the next editor who touches one copy knows
to check the other two. Worth a small standalone follow-up plan.

## Copywriting Contract Divergence

The location-off notice's exact string ("Location access is off — drag the map to your area, or
tap the map to drop a pin.") is pinned in Phase 1's `01-UI-SPEC.md` Copywriting Contract and
quoted in `02-UI-SPEC.md`. This plan's design-rule instruction (a later, overriding user
instruction, `site-design-rules.md`) required rewriting its punctuation
("Location access is off. Drag the map to your area, or tap the map to drop a pin."). The spec
files themselves were **not** edited by this plan — only the shipped code. The pinned string and
the shipped string now knowingly diverge on one character (an em dash replaced by a full stop).
This is recorded here rather than left as a silent drift; the developer should decide whether to
amend `01-UI-SPEC.md`/`02-UI-SPEC.md` to match the new punctuation, or leave the spec as
historical record of the original decision with this plan's override noted alongside it.

## What This Plan Will Not Change

The gap this plan partially closes was UAT's "the app looks like a beginner made it" finding.
**Fixing two mechanical rule violations does not answer that impression, and this is the
expected outcome, not a shortfall.** The structural root cause behind most of the remaining gap:
`02-UI-SPEC.md` never mentions hover, transition, motion or radius anywhere, so of roughly eleven
button-family selectors built against it, exactly one has any interaction feedback at all.
Deferred to `/gsd-ui-phase`, per the debug session's own recommendation
(`.planning/debug/ui-visual-polish.md`) and this plan's own scope decision:

- A hover, active and transition motion system across the whole button family, generalising the
  one working precedent (the Activity page's back control, 150ms ease with its own
  reduced-motion override) rather than inventing a second pattern.
- A radius, shadow and transition-duration token scale alongside the existing spacing and colour
  tokens. The 8px chosen in this plan is an interim value matching two existing controls; the UI
  phase may re-tokenise it into a proper scale.
- A background and surface-elevation treatment, including whether the report modal's panel and
  the primary buttons should sit on the surface tone rather than the page background.
- The theme control's redesign from a wordy text label to an icon, using the existing
  masked-SVG icon system. The research the user asked for has already been delivered; the
  redesign itself needs a contract.
- The Reopen button's wording, once the user confirms it is a control they meant, through the
  project's existing Copywriting Contract.
- An explicit ruling on the login gate's page heading, recording a considered "not hero text"
  rather than an unstated assumption.

## Five of Seven Design Rules Already Clean (Verified Negative)

`site-design-rules.md` lists seven hard rules. A full sweep of the live code before this plan
found exactly two live violations (the two this plan fixes) and cleanly cleared the other five.
Recording the negatives matters: "we checked and it was clean" and "we never checked" are
different states, and only one is evidence.

- No purple gradients or gradient-heavy backgrounds. `main.css`'s only `linear-gradient` is a
  flat single-colour fill trick on the severity slider (two identical colour stops), not a
  gradient in the visual sense the rule forbids.
- No fake reviews, testimonials, or fake metrics/stats. Confirmed by direct inspection; the app
  shows only real, live-computed values (confirm counts, report data).
- No hero text of any kind, with one borderline exception now explicitly deferred for a ruling
  (the login gate's page heading, see above) rather than silently assumed clean.
- No emoji used as icons. The app uses the committed Lucide SVG icon set throughout via
  `.icon-glyph`/`.auth-icon` masked-image rules, never an emoji character.
- No over-the-top scroll animations. The only three animations in the whole app are the ambient
  login-page globe rotation, a loading-skeleton pulse, and a 200ms toast entry; all three are
  restrained, none is scroll-triggered.

## Whether the Human-Check Sentences Read Well

Not evaluated in this session; this plan's own `<human-check>` block (7 items) is deferred to
`workflow.human_verify_mode: end-of-phase`, run via `/gsd-verify-work 2` alongside the phase's
other outstanding UAT items. A test can prove the dash character is gone from each rewritten
sentence; only a person reading them in a real browser, including the one that goes to a real
inbox (the verification-email disclaimer), can confirm they still read naturally.

## Verification Performed

- `go build ./...` and `go vet ./...`: clean.
- `go test ./web/ -count=1 -run 'TestUserVisibleCopyUsesPlainPunctuation' -v`: both the template
  gate and the auth.js positive-assertion test pass.
- `go test ./web/ -count=1 -run 'TestNoPillShapedControls' -v`: passes.
- `go test ./web/ -count=1`: full `package web` suite (all existing contract tests) passes.
- `go test ./... -short`: passes across every package.
- `go test ./internal/mailer/ ./internal/ratelimit/ -count=1 -v`: passes.
- `go test ./internal/api/... -count=1 -p 1` under a disposable database (created and dropped
  around the run, never `pinalert_test`): passes, confirming all five rewritten templates still
  parse and render server-side.
- `go test ./... -p 1` under a second disposable database: full suite, single worker, real
  Postgres, passes.
- Positive grep of the agreed rate-limit sentence in all four copies (`perip.go`, `perip_test.go`,
  `auth.go`, `auth.js`): each returns exactly 1.
- Positive grep of the rewritten disclaimer in both `resend.go` and `resend_test.go`: each returns
  exactly 1.
- `grep -n 'border-radius: 999px' web/static/css/main.css`: exactly two matches remain, both on
  the severity slider's track pseudo-elements.
- `grep -c 'border-radius: 8px' web/static/css/main.css`: 4, up from a pre-task value of 2 (the
  category tile and the account menu panel), confirming exactly the view toggle and toast were
  added.
- `git status --porcelain` after all commits: clean; the union of files touched across this
  plan's four commits matches the plan frontmatter's `files_modified` list exactly (thirteen
  paths). No file under `web/static/css/trust.css`, `web/static/js/map.js`,
  `web/static/js/modal.js`, `internal/api/handlers/reports.go`, `internal/service/` or `docs/`
  was touched.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - style/consistency] Removed em dashes from this plan's own newly authored comments
and test-failure strings**
- **Found during:** self-review after Task 3's commit, before writing this SUMMARY.
- **Issue:** the two new comments added to `web/static/css/main.css` and every doc comment and
  `t.Errorf`/`t.Fatalf` message added to `web/design_rules_contract_test.go` in this plan's own
  commits used em dashes. Go and CSS comments are correctly out of scope for the mechanical gate
  this plan ships (it only scans user-visible template bytes), so this was never a test-coverage
  gap; it was a plan whose entire purpose is removing em dashes introducing new ones in its own
  supporting code, which reads as inconsistent even though nothing enforces it.
- **Fix:** rewrote the affected prose using periods, semicolons and commas instead of em dashes.
  The two literal em/en dash characters the test defines as its forbidden character set were left
  unchanged, since those are data the test compares against, not prose. No pre-existing dash in
  an untouched comment elsewhere in `main.css` (there are many, all correctly out of scope per
  Task 2's own instruction to leave existing doc comments alone) was touched.
- **Files modified:** `web/static/css/main.css`, `web/design_rules_contract_test.go`
- **Commit:** `2950408`

No other deviations. The plan executed as written otherwise.

## Self-Check: PASSED

- FOUND: web/design_rules_contract_test.go
- FOUND: web/templates/profile.html.tmpl (title rewritten, verified via grep)
- FOUND: web/templates/verify_outcome.html.tmpl
- FOUND: web/templates/login_gate.html.tmpl
- FOUND: web/templates/check_inbox.html.tmpl
- FOUND: web/templates/index.html.tmpl
- FOUND: web/static/js/auth.js
- FOUND: internal/mailer/resend.go
- FOUND: internal/mailer/resend_test.go
- FOUND: internal/ratelimit/perip.go
- FOUND: internal/ratelimit/perip_test.go
- FOUND: internal/api/handlers/auth.go
- FOUND: web/static/css/main.css
- Commit af04ca2 (Task 1): present in `git log --oneline`
- Commit 33295b5 (Task 2): present in `git log --oneline`
- Commit 686b1d0 (Task 3): present in `git log --oneline`
- Commit 2950408 (deviation fix): present in `git log --oneline`
