---
status: diagnosed
trigger: "UI visual polish: buttons/backgrounds read as amateurish/beginner-made; wants smooth restrained transitions; two odd controls named (Activity page Reopen label, account menu Theme text toggle)"
created: 2026-09-22T00:00:00.000Z
updated: 2026-09-22T00:35:00.000Z
---

## Current Focus

hypothesis: CONFIRMED, complete sweep — structural root cause is 02-UI-SPEC.md never specifying hover/transition/motion/radius, so 10 of 11 button-family controls ship flat/instant (only .profile-back-link has hover/active/transition, added ad hoc in a later plan). A second independent driver: one flat --color-bg reused almost everywhere with no surface-elevation system, matching the user's separate "background" complaint. Plus 9 confirmed em-dash violations (7 template/JS + 2 previously-unrecorded Go server-side) and 2 pill-radius violations; gradient/emoji/fake-metric/scroll-animation all verified clean (negative).
test: Full 7-constraint site-design-rules.md sweep (dashes incl. internal/ Go strings, gradients, pill radii, emoji, hero text, fake metrics, scroll animation) across CSS/templates/JS/Go; full button-family radius/shadow/transition/hover/active inventory; design-token layer check; 02-UI-SPEC.md grepped for motion-spec coverage; background/surface treatment inventory; both named odd controls located exactly.
expecting: n/a — investigation complete for find_root_cause_only mode.
next_action: Investigation complete. Returned via SubagentHandback to caller with return_diagnosis-formatted report, including bundle-now-vs-defer split for the gap-closure round vs. /gsd-ui-phase.

## Symptoms

expected: Buttons, surfaces and backgrounds look intentionally designed; interactions have smooth, restrained transitions; nothing reads as a beginner's or AI-generated first project.
actual: User reported (2026-09-21): "the buttons and like the way the website is designed right now looks like a star beginner doing to start... I don't want it to look like that I want it to be like looking smooth transitions very smooth process." Also named two odd labels/controls: the Activity page's "Reopen · not actually resolved" button (web/static/js/votes.js REOPEN_BUTTON_LABEL) and the account menu's "Theme: Dark"/"Theme: Light"/"Theme: System" text toggle (web/static/js/theme.js), the latter confirmed 2026-09-22 as "it looks really wordy and I don't like it that way," with a request to research how other apps present a theme toggle.
errors: None; this is a design/perception issue, not a functional error.
reproduction: General use of the app (feed, map, vote buttons, Activity page, account menu), reported across Tests 3, 8 and 10 in UAT (.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UAT.md).
started: Raised 2026-09-21/22 during Phase 2 UAT round 3.

## Eliminated

(none yet)

## Evidence

- timestamp: 2026-09-22T00:05:00.000Z
  checked: site-design-rules.md memory file (hard constraints)
  found: 7 rules — no purple gradients, no pill buttons (rounded-rect with modest radius OK), no fake reviews/metrics, no hero text, no emoji icons, no em/en dashes anywhere user-visible, no over-the-top scroll animation. Memory already names known violations: em dashes in <title> tags, login_gate/check_inbox/index.html.tmpl body copy, auth.js rate-limit error; 999px pill radius on .view-toggle (~line 595) and #toast (~line 642).
  implication: Memory's claimed violations are the starting hypothesis list to re-verify against live code, not assumed-true.

- timestamp: 2026-09-22T00:10:00.000Z
  checked: em dash (U+2014) grep across web/templates/*.tmpl and web/static/js/*.js (en dash U+2013 also checked, zero hits anywhere)
  found: |
    Em dashes present in USER-VISIBLE copy:
    - web/templates/profile.html.tmpl:6 <title>Pinalert — Activity</title>
    - web/templates/verify_outcome.html.tmpl:6 <title>Pinalert — Verify your email</title>
    - web/templates/login_gate.html.tmpl:6 <title>Pinalert — Verify your email</title>
    - web/templates/login_gate.html.tmpl:19 body copy: "We'll send a one-time link — no password needed."
    - web/templates/check_inbox.html.tmpl:3 body copy: "...continue here — if you open it on a different device..."
    - web/templates/index.html.tmpl:71 location-off notice: "Location access is off — drag the map..."
    - web/static/js/auth.js:126 thrown error message: 'Too many requests — try again in a minute.'
    All ~40 other em-dash hits in web/static/js/*.js are inside `//` code comments (not rendered to the DOM) — correctly out of scope per the rule's own wording ("anywhere the user can read them").
  implication: Memory's dash-violation list is confirmed still accurate and current (not stale). 7 concrete user-visible dash occurrences across 6 files. Small, mechanical fix (string replacement), no design judgment needed.

- timestamp: 2026-09-22T00:15:00.000Z
  checked: gradient / pill-radius / emoji sweep across web/static/css/*.css
  found: |
    - Only linear-gradient uses: main.css ~745, ~763 (severity-slider track fill) — both stops reference the SAME var(--severity-current) value, i.e. a flat single-color fill via gradient syntax, not a visible gradient. Confirmed NOT a violation (matches memory's own assessment).
    - border-radius: 999px (pill) at main.css:595 (.view-toggle, inside a media query) and main.css:642 (#toast). Also 999px on .severity-slider track (741,759) which is a slider, and 50% on .account-trigger/.profile-back-link/.fab/icon-badge (circles, not pill buttons) — all correctly excluded by memory's own reasoning.
    - No emoji characters (U+1F000-1FFFF, U+2190-27FF, U+2B00-2BFF) found anywhere in templates/JS/CSS. Icons are all real SVG via a masked .auth-icon/.icon-glyph system (user.svg, log-out.svg, arrow-left.svg, etc.) — constraint already honored.
  implication: Exactly 2 live pill-radius violations (.view-toggle, #toast), matching memory. No gradient or emoji violations exist. These are also small, mechanical (radius-value) fixes.

- timestamp: 2026-09-22T00:20:00.000Z
  checked: design-token layer in main.css :root block
  found: |
    Tokens that DO exist: --space-xs..--space-3xl (spacing scale), --color-bg/surface/border/text/text-muted/focus-ring, --color-severity-low/medium/critical (+ -bg variants), --color-age-stale, --font-family/--font-size-*/--font-weight-*/--line-height-*, --touch-target-min.
    Tokens that do NOT exist anywhere in the codebase: no --radius-* scale, no --shadow-* scale, no --transition-* / --duration-* / --ease-* tokens. Every border-radius, box-shadow and transition value across all 5 CSS files is a hard-coded literal repeated ad hoc (radii alone: 6px, 8px, 50%, 999px all used with no naming/scale; transition durations found: 150ms ease used 3x in auth.css/modal.css, but never centralized).
  implication: A color+spacing token layer exists and is well-established (a polish pass can extend it), but there is no radius/shadow/motion token layer at all — every control invents its own values independently, which is a structural cause of the visual inconsistency, not just an aesthetic one.

- timestamp: 2026-09-22T00:25:00.000Z
  checked: button-family selector inventory (.btn/.btn--primary/.btn--destructive, .vote-btn + 5 variants, .account-trigger, .view-toggle, .profile-back-link, .fab, .category-tile, .account-menu__item) for radius/shadow/transition/hover/active across all 5 CSS files
  found: |
    Radius values in use with no shared scale: 6px (.btn, .vote-btn, .account-menu__item, .skeleton-row, .category-tile is 8px), 8px (.category-tile, #account-menu panel), 50% (.account-trigger, .profile-back-link, .fab, icon-badge), 999px (.view-toggle, #toast, slider track).
    Shadow values, also all ad hoc literals with no scale: 0 2px 8px rgb(0 0 0 / 20%) (.account-trigger, .profile-back-link), 0 4px 16px rgb(0 0 0/20%) (#account-menu panel), 0 2px 8px rgb(0 0 0/25%) (.fab), 0 1px 4px rgb(0 0 0/35%) (.icon-badge--pin), 0 1px 3px / 0 2px 5px rgb(0 0 0/18%,28%) (severity-slider thumb). .btn, .btn--primary, .btn--destructive, .vote-btn and all its variants (--confirm/--dispute/--resolve/--reopen/--confirm-resolve/--cancel-resolve) have NO box-shadow at all — flat.
    Hover/active/transition — the decisive finding: grepping :hover across ALL 5 CSS files returns exactly ONE match in the entire codebase: .profile-back-link:hover (auth.css:408). :active returns .profile-back-link:active plus 2 severity-slider-thumb-only :active rules. transition on an actual button returns exactly ONE case: .profile-back-link (auth.css:401, 150ms ease, with its own prefers-reduced-motion override at 417-421).
    Every other button-family selector — .btn, .btn--primary, .btn--destructive, .vote-btn and all 5 of its state variants, .account-trigger, .view-toggle, .fab, .category-tile, .account-menu__item — has ZERO hover state, ZERO active/press state, and ZERO transition. Clicking or hovering any of these gives no visual feedback whatsoever; state changes (e.g. .vote-btn--confirm[aria-pressed="true"]) snap instantly with no interpolation.
    auth.css's own code comment at line 383 (on .profile-back-link) states explicitly: "Unlike .account-trigger it has hover and press states, which is deliberate" — confirming this inconsistency is an intentional, isolated exception rather than an oversight in that one case, but it means .profile-back-link is the ONLY polished control in the entire button system, everything else is flat/static by design-as-built.
  implication: This is very likely the single largest concrete, demonstrable contributor to the "beginner"/"static" impression — not button color or shape, but the near-total absence of interaction feedback across the button system. A polish pass adding a shared --transition-fast token + :hover/:active states to .btn/.vote-btn/.account-trigger/.view-toggle/.fab (matching the one working precedent, .profile-back-link) would directly address "smooth transitions" per the user's own words, and is a large-but-bounded, mechanical-ish change (apply one working pattern everywhere) rather than a from-scratch redesign.

- timestamp: 2026-09-22T00:30:00.000Z
  checked: the two named odd controls' exact current source
  found: |
    1. web/static/js/votes.js:79 — var REOPEN_BUTTON_LABEL = 'Reopen · not actually resolved'; (also line 80: REOPEN_BUTTON_LABEL_IN_FLIGHT = 'Reopening…'). Uses U+00B7 MIDDLE DOT, not an em/en dash — does not itself violate the dash rule, but is separately named by the user as odd/wordy copy.
    2. web/templates/account_header.html.tmpl:11-13 — <button ... id="theme-toggle" ...>Theme: <span id="theme-toggle-label">System</span></button>, driven by web/static/js/theme.js which writes 'Dark'/'Light'/'System' into #theme-toggle-label. CONFIRMED 2026-09-22 by the user as the specific odd control ("it looks really wordy"), with an explicit ask to research how other apps present a theme toggle before redesigning (candidate direction per 02-UAT.md gap entry: icon-only sun/moon-style control cycling the same 3 modes, using the existing .auth-icon mask-based SVG icon system already used for user.svg/log-out.svg/arrow-left.svg — consistent with the no-emoji-icons constraint).
  implication: Both controls' exact locations are confirmed unchanged since the UAT/memory record. The theme toggle is the user-confirmed priority of the two; REOPEN_BUTTON_LABEL wording remains unconfirmed-but-named.

- timestamp: 2026-09-22T00:40:00.000Z
  checked: remaining 3 site-design-rules.md constraints not yet swept (hero text, fake reviews/metrics, over-the-top scroll animation), plus scroll-triggered animation code, plus a full read of login_gate.html.tmpl and index.html.tmpl
  found: |
    Hero text: index.html.tmpl (main app) has no h1/headline/tagline of any kind — goes straight from the shared header into the map/feed. login_gate.html.tmpl has one <h1>Verify your email to continue</h1> above a single sentence of instructional copy and the email field — this is a login-gate page heading, not a marketing splash (no subtext/CTA-banner shape, no separate tagline block), so it does not read as the "hero text" the rule targets (a big headline/tagline banner above the app) on plain inspection, but it is the one borderline case worth the design pass explicitly ruling on rather than assuming.
    Fake reviews/metrics: none found in any template — no testimonial copy, no hardcoded counts/stats anywhere in web/templates/*.tmpl. Verified negative.
    Scroll-triggered animation: grep for @keyframes/animation:/scroll-behavior across all CSS returns exactly 3 animations: globe-rotate (auth.css:61, 90s linear infinite ambient background rotation on the login page, not scroll-triggered), skeleton-pulse (main.css:618, 1.4s loading-state pulse), toast-in (main.css:646, 200ms toast entry). grep for IntersectionObserver/scroll-event-listeners across all JS returns zero hits. No scroll-triggered animation exists anywhere in the codebase. Verified negative — all 3 site rules not yet checked are clean.

- timestamp: 2026-09-22T00:45:00.000Z
  checked: 02-UI-SPEC.md (the phase's own approved design contract, 6/6 dimensions passed) for any mention of hover/transition/motion/press/radius as interaction-design guidance
  found: The only matches for "press" are all "pressed" as in aria-pressed state naming (color/weight token choices for the pressed *state*, e.g. line 55, 89, 274) — zero literal mentions of "hover", "transition", "motion", or "radius" anywhere in the document.
  implication: The approved UI-SPEC for this phase never specified any hover/press-transition motion language or a radius token at all — it fully specifies state *colors* (pressed = colored, not just weight) but never specifies *how* a control gets from one state to another, or a shared corner-radius scale. This is the structural root cause behind the button-system finding above: 11 controls were each built to a contract that was silent on motion, so most defaulted to instant/flat, and the one exception (.profile-back-link, added in a later plan, 02-10) picked up hover/active/transition ad hoc rather than from any shared spec.

- timestamp: 2026-09-22T00:50:00.000Z
  checked: em dash sweep extended to internal/ (Go server-side strings), since user-visible copy is not template/JS-only
  found: |
    Two more user-visible em dashes found server-side, both previously unrecorded (memory only listed templates/JS):
    - internal/mailer/resend.go:21 — verificationDisclaimer = "Didn't request this? You can safely ignore this email — no account will be created." (sent in the actual verification email body to real users)
    - internal/ratelimit/perip.go:29 — tooManyRequestsMessage = "Too many requests — try again in a minute." (the server-side rate-limit message; identical text to the one already found client-side at auth.js:126, so likely the same logical string duplicated client+server rather than a second distinct violation)
    All other em-dash hits in internal/ are in `//` Go doc comments, correctly out of scope.
  implication: Total user-visible em-dash violations: 9 occurrences across 8 files (7 template/JS + 2 Go), one logical string duplicated in two places (rate-limit message). Still a small, mechanical find-replace fix, but the fix scope is one file wider than memory recorded (mailer + ratelimit packages, not just templates/JS).

- timestamp: 2026-09-22T00:55:00.000Z
  checked: background/surface treatment across all 5 CSS files (the user's complaint named "the background" alongside buttons, not yet separately inventoried)
  found: |
    grep for `background` across all CSS shows: body sits on flat var(--color-bg) with no texture/layering. Only var(--color-surface) (the one alternate, slightly-tinted tone) is used sparingly: .login-card, .category-tile, #account-menu panel (this one also gets a box-shadow, i.e. the only true "elevated surface" in the app), .vote-btn:disabled. Everything else that could read as a "surface" — .btn, .vote-btn (default state), .account-trigger, .fab, .profile-back-link, and critically the report-submission .modal-panel itself — sits on the SAME flat --color-bg as the page background, differentiated only by a 1px border and sometimes a box-shadow, never a distinct fill tone. .report-row gets a severity-colored left-border + tint, but that's data-driven accent, not a general surface-elevation system.
  implication: There is effectively one flat background tone reused almost everywhere, with a single alternate tone (--color-surface) applied inconsistently to only some controls, and no layered elevation system (e.g. surface gets progressively lighter/darker per z-level) tying it together. This directly substantiates the "background" half of the user's complaint ("the buttons and... the background"), independent of the button-motion finding — a second, separate concrete driver.

## Resolution

root_cause: |
  Not a single defect — a design-consistency gap with several independently-confirmed concrete drivers, one of which has a clean structural explanation:

  1. STRUCTURAL ROOT CAUSE: 02-UI-SPEC.md (the phase's own approved design contract, 6/6 dimensions passed) never specifies hover/press-transition motion or a radius token anywhere in the document — it specifies pressed-*state* colors but never *how* a control transitions between states, and never a shared corner-radius scale. Consequently, of ~11 distinct button-family selectors built against that contract (.btn, .btn--primary, .btn--destructive, .vote-btn + 5 state variants, .account-trigger, .view-toggle, .fab, .category-tile, .account-menu__item), only ONE control anywhere in the codebase — .profile-back-link, added later in plan 02-10, with its own code comment noting "unlike .account-trigger it has hover and press states, which is deliberate" — has any :hover/:active/transition at all. Everything else is flat and instant. This is the most likely largest *verifiable* contributor to "looks like a beginner"/lacks "smooth transitions" (the user's own words): the app isn't poorly colored so much as static.
  2. A second, independent driver matching the other half of the user's complaint ("the buttons and... the background"): there is effectively one flat background tone (--color-bg) reused almost everywhere — including .btn, .vote-btn, .account-trigger, .fab, .profile-back-link, and the report-submission modal panel itself — with only a single alternate tone (--color-surface) applied inconsistently to a few controls (login-card, category-tile, the account-menu dropdown panel), and no layered elevation system tying surfaces together.
  3. No radius/shadow/transition design-token layer exists at all (only --space-* and --color-* tokens do), so every control's radius (6px/8px/50%/999px, no shared scale) and shadow value is an independently hand-picked literal — a structural cause of visual inconsistency, not just a symptom, and the same root gap as #1 (an incomplete design-contract token layer).
  4. Two live, mechanical site-design-rules.md violations remain, both re-confirmed present: 9 user-visible em-dash occurrences across 8 files — 7 in templates/JS (title tags in profile/verify_outcome/login_gate.html.tmpl, body copy in login_gate/check_inbox/index.html.tmpl, one thrown error string in auth.js) plus 2 previously-unrecorded Go server-side strings (internal/mailer/resend.go:21 verification-email disclaimer, internal/ratelimit/perip.go:29 rate-limit message, the latter duplicating auth.js's client-side string) — and 2 pill-radius (999px) occurrences (.view-toggle in main.css:595, #toast in main.css:642). No gradient, emoji, fake-metric, or scroll-triggered-animation violations exist anywhere (all verified negative). One borderline case not previously ruled on: login_gate.html.tmpl's <h1>Verify your email to continue</h1> — reads as a functional gate heading, not a marketing hero banner, on plain inspection, but is worth an explicit design-pass ruling rather than an assumption.
  5. Two controls named by the user as specifically odd: the account-menu theme toggle's wordy "Theme: Dark/Light/System" text label (account_header.html.tmpl:11-13 + theme.js) — CONFIRMED by the user 2026-09-22 as the one they meant, wants researched against how other apps present theme toggles before an icon-based redesign — and votes.js:79's REOPEN_BUTTON_LABEL "Reopen · not actually resolved" (uses a middle dot, not a dash; user has not confirmed this is the one they meant, possibly both).
fix: |
  Not applicable — find_root_cause_only mode, no fix applied. Suggested fix direction and bundle-now-vs-defer split are recorded below for the design-pass owner, not implemented here.

  Recommended for the CURRENT small gap-closure round (mechanical, no design judgment required):
  - Replace all 9 em-dash occurrences with plain punctuation (comma/period/rewrite): profile.html.tmpl:6, verify_outcome.html.tmpl:6, login_gate.html.tmpl:6+19, check_inbox.html.tmpl:3, index.html.tmpl:71, auth.js:126, internal/mailer/resend.go:21, internal/ratelimit/perip.go:29 (check for any contract/unit test asserting the exact current string before editing perip.go/auth.js's shared message).

  Recommended for the current round WITH a caveat (fixes a known violation now, but the replacement value is itself a design choice the later UI phase may re-tokenize):
  - Change the two 999px pill radii (.view-toggle main.css:595, #toast main.css:642) to a modest rounded-rect radius — 8px is suggested since it already matches two existing controls (.category-tile, the #account-menu dropdown panel), keeping the interim value consistent with something already in the codebase rather than inventing a third.

  Recommended to DEFER to /gsd-ui-phase (broad, contract-level design work, not a mechanical fix):
  - A hover/active/transition motion system applied consistently across the whole button family (.btn, .vote-btn + variants, .account-trigger, .view-toggle, .fab, .category-tile), generalizing the one working precedent (.profile-back-link's 150ms ease pattern) rather than inventing a new one, plus a prefers-reduced-motion story for all of it (the existing precedent already has one to follow).
  - A radius/shadow/transition-duration token scale added to main.css's :root block, alongside the existing --space-*/--color-* tokens.
  - A background/surface elevation treatment addressing the "one flat background reused everywhere" finding — including whether the modal panel and primary buttons should sit on --color-surface rather than --color-bg.
  - The theme-toggle redesign (user explicitly asked for prior research into how other apps present the control; already scoped into the 02-UAT.md gap for the wider pass).
  - REOPEN_BUTTON_LABEL wording — user has not confirmed this is the control they meant, and any copy rewording belongs with the project's existing Copywriting Contract process, not a standalone edit.
  - An explicit design-pass ruling on login_gate.html.tmpl's <h1>, to record a considered "not a violation" rather than an unstated assumption.
verification: (not applicable — diagnose-only investigation)
files_changed: []
