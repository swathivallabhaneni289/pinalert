---
phase: quick-261013-lrw
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/static/css/phone.css
  - web/templates/index.html.tmpl
  - web/templates/profile.html.tmpl
  - web/static/js/feed.js
  - web/static/js/map.js
  - web/phone_layout_contract_test.go
  - web/popup_contract_test.go
  - web/trust_contract_test.go
  - internal/api/handlers/auth.go
  - internal/api/handlers/activity_e2e_test.go
  - internal/api/handlers/profile_record_test.go
  - .planning/sketches/005-phone-layout/README.md
autonomous: true
requirements: [QUICK-261013-LRW]

must_haves:
  truths:
    - "At 390 by 844 the feed shows the map on top at about 40 percent of the height and the Nearby now list under it at the same time, with no map or list switch button and no sideways scroll"
    - "At 1280 by 800 every probed computed style equals the baseline captured before the first edit, so the desktop and iPad two column layout is unchanged"
    - "On a phone, tapping a row opens the full screen report detail with a short map strip on top showing the pin; tapping a pin opens the same detail and scrolls only the list pane to that row, never the page; the round back arrow closes the detail and focus returns to the opener"
    - "The phone detail shows the real severity pill, title, meta line, white trust card with tick rows, fade line, Confirm and Dispute side by side, Mark resolved as a centred green link and the existing too far sentence, all from the server's data"
    - "The phone Activity page shows the Activity title with Log out, the reporter record card built from the server's real label and counts, every own report with severity word, state word, both counters, meta line, trust panel and Reopen, and the bottom bar with Activity current"
    - "Every departure from the pictures and from the house rules, including the three drawn items that are not built (Past tab, category chips, My votes), is written in .planning/sketches/005-phone-layout/README.md"
    - "The full Go suite passes on a disposable database with 0 skipped tests"
  artifacts:
    - path: "web/static/css/phone.css"
      provides: "Every phone rule, inside one max-width 899.98px block, plus base rules that hide the phone only elements at 900px and up"
    - path: "web/phone_layout_contract_test.go"
      provides: "Contract tests for the phone stylesheet, the bottom bar, the pane switch in map.js, the phone list scroll and the Activity phone markup"
    - path: ".planning/sketches/005-phone-layout/README.md"
      provides: "Departures from the two pictures and from the house rules, text only"
    - path: "internal/api/handlers/auth.go"
      provides: "profileViewModel.Record built from the account's own reporter signal"
  key_links:
    - from: "web/templates/index.html.tmpl and web/templates/profile.html.tmpl"
      to: "web/static/css/phone.css"
      via: "a stylesheet link placed after every other app stylesheet, with the same AssetVersion query"
      pattern: "phone.css\\?v=\\{\\{.AssetVersion\\}\\}"
    - from: "web/static/js/map.js"
      to: "the detailPane created inside the #map container"
      via: "popup options pane and autoPan written per breakpoint before a popup opens"
      pattern: "detailPane"
    - from: "web/static/js/feed.js render"
      to: "#feed-count"
      via: "count of rendered reports and Pinalert.config.defaultRadiusKm"
      pattern: "feed-count"
    - from: "internal/api/handlers/auth.go Profile"
      to: "web/templates/profile.html.tmpl"
      via: "Record field read by the record card"
      pattern: "\\.Record"
---

<objective>
Build the phone layout (under 900px wide, the existing 899.98px breakpoint) for the feed, the report detail and the
Activity page, reproduced from the owner's two pasted pictures, while the layout at 900px and up stays exactly as it is.

Purpose: the owner said on 2026-10-10 that the site is mostly for phones. The pictures are the look they want. Owner rule:
copy the picture, record every conflict with the house rules in the docs, do not adapt.

Output: one new stylesheet (`web/static/css/phone.css`), small markup additions to the two page templates, the minimum
JavaScript (the live count line, a row distance line, a phone only list scroll, a phone only popup pane switch), a
reporter record on the Activity page from the server's own signal, a new contract test file, and a text only departures
document at `.planning/sketches/005-phone-layout/README.md`.

Scope decisions made while planning, recorded here so no executor re-decides them:

1. Three drawn items are NOT built, because each needs data or a feature that does not exist and a dead control is not
   allowed (sketch 004 README already rules dead controls out):
   - the Past tab in the bottom bar (no history view exists; the feed query drops expired reports; it is Phase 3.1, which
     still has an open owner question about phone history tabs),
   - the category chips over the map (category filtering is Phase 3.1 HIST scope, with an open owner question on whether
     a filter may hide critical or rescue reports),
   - the My reports and My votes segmented control (no read of votes by account exists anywhere; the built laptop
     Activity page lists only the account's own reports, and `TestProfileNoLongerPromisesVotingHistory` guards against
     promising a voting history).
   Each is written in the departures document as "not built, needs X". The bottom bar therefore has two tabs, Feed and
   Activity. The handback to the owner asks about all three.
2. The pictures are words, not measurements. Sizes, spacing and corners use the existing tokens (type 14, 16, 18, 24;
   weights 400 and 600; spacing 4 to 64; corners up to 8px; 44px targets). The document records this.
3. Wording: every drawn string maps to an existing app string where one exists, and each mismatch is recorded. The two
   pictures contradict each other on the provisional word ("Unconfirmed" in picture A, "Waiting" in picture B), so the
   existing "Unconfirmed" stays. Both trust counters stay on every row (D-16).
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.claude/skills/sketch-findings-pinalert/SKILL.md
@.planning/sketches/004-activity-page/README.md
@web/templates/index.html.tmpl
@web/templates/profile.html.tmpl
@web/templates/account_header.html.tmpl
@web/templates/theme_toggle.html.tmpl
@web/static/css/main.css
@web/static/css/feed.css
@web/static/css/auth.css
@web/static/js/feed.js
@web/static/js/map.js
@web/static/js/activity.js
@web/static/js/trust.js
@web/static/js/app.js
@internal/api/handlers/auth.go

<owner_pictures_in_words>
The project keeps no images. These are the owner's two pictures as words, the only description executors get.

PICTURE A, screen "02 Feed, map + list":
- Map on top, about 40 percent of the height, always visible. Category chips float over the top of the map in one row
  that scrolls sideways (All selected dark with white text, Flood, Rescue, Shelter, Power, more clipped). NOT BUILT, see
  the objective.
- Map pins are round dots with a white ring (red critical, green low, grey unconfirmed).
- Under the map, a list headed "Nearby now" (bold, left) with "5 active · 1 km" in grey on the right.
- Rows are rounded cards. Critical has a soft red tint, Low a soft green tint, unconfirmed is white. First line small: a
  coloured dot, the bold severity word ("Critical", "Low", "Low · Unconfirmed"), then category, minutes, distance in
  grey ("Flood · 51 min · 0.3 km"). Then the report text in bold, up to 2 lines. Then one trust line: tinted rows
  "1 confirmed · Too early to tell"; the green row "2 confirmed · Up to date · Reliable reporter" in green; the white row
  "Waiting for a second confirmation" in grey.
- A round black add button with a plus, bottom right, floating above the list and above the bottom bar.
- Bottom bar: three tabs, Feed, Past, Activity. The active tab is bold black with a small dot above it, the others grey.
  Past is NOT BUILT.
- No map or list switch button on phones.
- Tapping a row highlights its pin; tapping a pin scrolls to its row.

PICTURE A, screen "03 Report detail" (phone, full screen):
- Short map strip at top with a round white back arrow. A severity chip (outlined pill with dot and word, for example
  "Low"). The title in large bold. A grey meta line "Power outage · 50 min ago · 0.4 km away". A white trust card: top
  line "2 confirmed · 0 disputed" with "Up to date" in green on the right, then tick rows each with a green round tick, a
  bold heading and a grey sub line ("Seen from 2 different places / Counts places, not votes.", "No disputes / No one
  nearby has disputed this.", "Still happening / 2 confirmations in the last 2 hours.", "Reliable reporter / 5 of 5
  earlier reports were confirmed."). Below the card: "Fades in 7 h unless confirmed again." Two big buttons side by
  side: Confirm (black fill, white text) and Dispute (white). A green centred "Mark resolved" link. A small grey centred
  hint "You need to be within 1 km to confirm or dispute." The page background is a very light green tint when the
  severity is Low.

PICTURE B, screen "05 Activity":
- Large bold "Activity" title on the left, grey "Log out" on the right.
- White rounded card: small grey "Your reporter record", bold "Reliable" under it, "5 of 5 confirmed" on the right.
- Segmented control (grey track, white selected segment): "My reports" and "My votes". NOT BUILT.
- Report cards (white, rounded): first line a coloured dot and bold severity word on the left ("Low" green, "Medium"
  amber, "Critical" red) and a status word on the right, coloured: Active (green), Waiting (amber), Resolved (grey),
  Expired (grey). Then the report text in bold. Then a grey line: "2 confirmed · 0 disputed · 50 min ago", "Needs one
  more confirmation · 5 min ago", "Resolved by you · yesterday", "Not re-confirmed · 2 days ago".
- The same bottom bar with Activity active.
- Owner: "use the same instructions we used for the laptop version, the disputes and other things should also be
  added". Everything the laptop Activity page has stays on the phone page in the picture's style.
</owner_pictures_in_words>

<phone_facts>
Facts checked while planning. Executors rely on them and do not re-derive them.

Leaflet 1.9.4 (read from the unpkg source):
- `map.createPane(name, container)` creates the pane inside `container` when one is given, else inside the map pane.
  Its class is `leaflet-pane leaflet-<name without "Pane">-pane`, so `createPane('detailPane', ...)` gets the class
  `leaflet-detail-pane`.
- `DivOverlay.onAdd` reads `this.getPane()` on every open, which reads `options.pane`. Writing `popup.options.pane`
  before an open moves where that popup opens.
- `DivOverlay.update` runs `_adjustPan` inside `onAdd`, before the `popupopen` event fires. `_adjustPan` returns at once
  when `options.autoPan` is false. So autoPan must be false BEFORE the open on a phone.
- Leaflet writes the popup container's `bottom`, `left` and `transform` and the content node's `width` (minWidth and
  maxWidth 340 here) as inline styles on every update, zoom and view reset.
- The map pane carries a transform, so a fixed position element inside it is trapped. A pane parented to the `#map`
  container itself sits outside that transform.

Contract tests that read the CSS (grep before touching a selector):
- Many tests look rules up by exact selector head inside a named file (`ruleBySelector`), for example `.fab`,
  `.theme-toggle`, `.app-header`, `.account-trigger`, `.profile-page`, `#toast`, `.icon-badge`, `.vote-btn:not([hidden])`.
  Every phone rule therefore lives in the new `phone.css` and carries a container prefix that makes its head distinct
  (`#report-list .report-row`, `.profile-report-list .report-row`, `.leaflet-container .leaflet-detail-pane ...`,
  `body:has(...)`, `.app-shell > .view-toggle`), so exact head lookups keep reading the base rules.
- Whole directory scans walk every file under `static/css` (css_contract_test.go, labels_contract_test.go,
  votes_contract_test.go, trust_contract_test.go). Known ones: a rule naming the severity label must not declare
  `height`, must guard any `display` with `:not([hidden])`, must not use the important flag or declare a custom
  property; a rule naming a trust class must not use the important flag or 999px and may not use the muted or tone
  colours outside a fixed list of heads; trust.css itself may never hold the important flag.
- `TestNoPillShapedControls` fails fatally if main.css has fewer than two `.view-toggle` rules. Do not edit main.css at
  all: hide the toggle on phones from phone.css with the head `.app-shell > .view-toggle` (feed.js reparents the button
  under `#app-shell`).
- `TestPrimaryMapHasResolvedHeight` counts only main.css. A phone `#map` height in phone.css does not disturb it.
- `TestMapSelectionNeverScrollsThePage` (popup_contract_test.go) forbids the browser's into view scroll call anywhere in
  feed.js and any scroll inside `applySelectionHighlight`. The phone list scroll is a separate function that sets the
  list pane's own scroll position.
- The bind call checks in popup_contract_test.go (around line 979) pin the bindPopup literals `autoPan: !reducedMotion`,
  both autopan paddings and `keepInView: true`. Keep those literals and write the phone values onto the popup right
  after the bind.

Z index ladder (existing, then new): Leaflet panes 400, Leaflet control corners 1000, `.fab`, `.view-toggle` and
`.theme-toggle` 1000, `.app-header` 1050, modal 1100, toast 1200. New: bottom bar 1010, detail pane 1080. The detail
pane sits in the root stacking context because `.pane--map`, `.pane` and `.leaflet-container` set no z index.

Existing strings the phone layout reuses (never retype them in new copy):
- Severity words `Low`, `Medium`, `Critical` (app.js SEVERITY_WORDS). State words `Unconfirmed`, `Disputed`, `Resolved`,
  `Expired`; Live has no word (visibility.js VISIBILITY_TAG_LABELS, live maps to the empty string by contract).
- Row meta `Flood · 51 min ago` (feed.js buildMetaText). Distance `0.3 km away` (app.js formatDistance, shown only when
  `Pinalert.feedIsAroundReader()` is true, rule A94).
- Trust face `1 confirmed`, `0 disputed`, currency word (for example `Too early to tell`, `Report up to date`), reporter
  words (for example `Reliable reporter`), all server written through trust.js.
- Popup: severity pill, state line, title, meta line `Power outage · 50 min ago · 0.4 km away`, trust card header and
  tick rows, fade line from `Pinalert.formatFade`, vote block (Confirm, Dispute, Mark resolved, the resolve
  confirmation), too far sentence `Too far to vote. You need to be within 1 km of this report.` (votes.js, shown only
  when the reader is too far).
- Reporter signal on every Activity row's trust object: `label`, `judged_reports`, `corroborated_reports`; the Go
  struct is `service.ReporterSignal` with `Label`, `JudgedReports`, `CorroboratedReports`. ActivityForAccount gives every
  row the account's own signal.
- Log out: the account header's form is `method="post" action="/auth/logout"` with one submit button and no other field.

Separator: the middle dot `·` (U+00B7), as the existing copy uses.

Reporter record for a new account (checked in `ScoreReliability` and `newReporterSignal`): with fewer judged reports
than the minimum, including none, the tag is `new`, the label is `New reporter` and both counts are real (0 and 0 for an
account whose reports have not expired yet). ActivityForAccount always asks for the viewer's own signal, so an account
with at least one report always gets a label.

Contrast (WCAG 4.5 to 1 for text, the app's own rule; computed from the tokens while planning):
- Light theme, `--color-text-muted` (#6B7280): 4.83 on `--color-bg`, but 4.34 on `--color-surface`, 4.30 on the low tint,
  4.37 on the medium tint and 4.12 on the critical tint. All four FAIL.
- Light theme, `--color-tone-good` (#3A7B4D): 5.10 on the background, 4.58 on the surface, 4.53 on the low tint, 4.61 on
  the medium tint, 4.35 on the critical tint (FAIL).
- Light theme, `--color-tone-caution` 5.24 and `--color-tone-bad` 5.44 on the background.
- Dark theme: every muted and tone pair on the background, the surface and the three tints clears 4.5, except
  `--color-tone-bad` on the low tint (4.17) and the medium tint (4.20).
- Rule for every task: compute each new text and background pair in both themes. A pair under 4.5 uses `--color-text`
  instead (a fix, not a departure), and the departures document lists each such swap, so the picture's grey or green on
  a tinted surface becomes the normal text colour there. Grey text is muted only on white surfaces.
</phone_facts>

<safety_rules>
Read before running anything. These come from the owner's standing rules.
- NEVER run the Go tests or the app against the databases `pinalert_test` or `pinalert`, and never let `DATABASE_URL`
  leak in from the shell. Every database the run uses is created fresh by name, the run refuses when the name already
  exists, and only that name is dropped afterwards.
- Run Go commands under `env -i PATH="$PATH" HOME="$HOME" ...` so nothing else leaks in. `env -i PATH HOME` without the
  values runs a command named PATH, so always write the values.
- The `web` package tests need no database. The fast loop is `env -i PATH="$PATH" HOME="$HOME" go test ./web/ -count=1`
  (no DATABASE_URL at all, so no database test can run).
- The full suite runs with `-v` so skipped tests print; count the `--- SKIP` lines (must be 0) and the `--- FAIL` lines
  (must be 0). macOS has no `timeout` binary: never wrap a command in it.
- If the sandbox refuses a long compound line, split it into the separate steps listed.
- Ports: never 8080, 8090 or 8792. Use 8794 for the probe server and 8795 for the browser harness.
- No images or screenshots saved anywhere in the repo. Visual checks are computed style and geometry probes that print
  numbers. Probe scripts and logs live in the session scratchpad, never in the repo.
- Never edit `.planning/STATE.md` or `.planning/ROADMAP.md` (another session holds an uncommitted edit to STATE.md).
- Commits are path limited: `git add -- <paths>`, then `git commit -m "<subject>" -m "<body>" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- <paths>`.
  The trailer line is exactly that text. Never use the gsd commit verb (it commits everything staged). No dashes (em or
  en) in any prose, comment, commit message or UI copy you write.
- Do not edit the browser harness at `~/code/pinalert-browser-checks`; read its `lib/` for helpers and run it as is.
</safety_rules>

<probe_recipe>
How the computed style probes run (used by all three tasks).

Server: `createdb p_phone_probe` (refuse if it exists), build with `env -i PATH="$PATH" HOME="$HOME" go build -o
<scratchpad>/pinalert-server ./cmd/server`, start it in the background with `env -i PATH="$PATH" HOME="$HOME"
ENV=development PORT=8794 BASE_URL=http://127.0.0.1:8794 DATABASE_URL=postgres://localhost:5432/p_phone_probe?sslmode=disable
<scratchpad>/pinalert-server > <scratchpad>/server.log 2>&1`. Run the migrations once first with the same env and
`go run ./cmd/migrate`. `ENV=development` selects the log mailer, so the sign in link is printed in the server log.
Sign in and seed with curl and a cookie jar as the memory recipe describes: request a link for a made up
`@example.com` address, read the link from `server.log`, GET it with the jar, then POST `/api/reports` (category,
severity, description of 10 or more characters, latitude, longitude). Seed at least: one critical flood, one low power
outage confirmed from a second account at a different spot, one low report left unconfirmed, one medium report, and one
report from the first account that the first account resolves (POST `/api/reports/{id}/resolve` with its jar). Create
the reports right before probing; low reports expire after 8 hours.

Browser: find Playwright with `node ~/code/pinalert-browser-checks/lib/find-playwright.js` and put the printed path in
`NODE_PATH`. Write the probe script in the scratchpad. Sign the probe browser in by opening a fresh link in its context.
Grant geolocation in the context at the seeded coordinates so distances show. Probe in Chromium and WebKit at 390 by
844 and at 1280 by 800, and print JSON numbers only.

Rect checks prove where boxes are, not what is on top, so every phone probe also checks paint and stacking:
- the open `.leaflet-popup` has computed `opacity` 1 and `visibility` visible (leaflet.css sets the popup to opacity 0
  under `.leaflet-fade-anim` unless it sits in the map pane, so the detail relies on Leaflet's inline opacity),
- `document.elementFromPoint` at the sheet's centre and just above its bottom edge returns a node inside the card,
- `document.elementFromPoint` at the add button's centre returns the button, at the last row's bottom inside the list
  pane (after scrolling the pane to its end) returns that row, and at the bottom bar's centre returns a bar tab.

Baseline file: the desktop baseline lives at the fixed path `$HOME/.cache/pinalert-261013-lrw/desktop-baseline.json`
(outside the repo, so a later task in another agent can read it). Task 3 deletes that folder after its comparison.

Teardown at the end of each task: stop the server by PID, `dropdb p_phone_probe`, delete `server.log`, cookie jars and
link files.
</probe_recipe>

<tasks>

<task type="auto">
  <name>Task 1: Phone feed with the map on top, the Nearby now list, row cards, dot pins and the bottom bar</name>
  <files>web/static/css/phone.css, web/templates/index.html.tmpl, web/templates/profile.html.tmpl, web/static/js/feed.js, web/phone_layout_contract_test.go, .planning/sketches/005-phone-layout/README.md, web/popup_contract_test.go, web/trust_contract_test.go</files>
  <action>
Step 0, desktop baseline BEFORE any edit. Follow the probe recipe on the current HEAD. At 1280 by 800, in Chromium and
WebKit, record into `<scratchpad>/desktop-baseline.json` the computed values of: `.app-shell` grid-template-columns,
gap, min-height; `.pane--map` and `.pane--list` display and overflow; `#map` height; `#view-toggle` display;
`.filter-toggle` display; the first `#report-list .report-row` display, padding, border-left, border-radius,
background-color and the first row badge display; `.report-row .severity-label` border and background; `.fab` position,
right, bottom; `.theme-toggle` right, bottom; `.app-header` display, top, right; `#toast` bottom; an opened popup's
`.leaflet-popup-content` width and its pane class; `.profile-page` padding and max-width on `/profile`;
`.profile-back-link` display. Also record `document.documentElement.scrollWidth` minus `innerWidth`. Write it to the
baseline path in the probe recipe. This file is the proof that 900px and up stays unchanged.

Step 0b, find the test fallout cheaply before writing rules: create `web/static/css/phone.css` holding only its head
comment, add its link to both `web/templates/index.html.tmpl` and `web/templates/profile.html.tmpl` now (after
auth.css in each, with `?v={{.AssetVersion}}`), which shows any test that lists the exact stylesheets per template or
the exact CSS file set, and run the web package. Fix what that shows
first. Then add the rules below in chunks (layout, heading and rows, pins, bottom bar), running the web package after
each chunk.

Step 1, the stylesheet. Create `web/static/css/phone.css` with a head comment (no dashes) that says it holds every phone
rule, that it loads after every other app stylesheet, that it overrides main.css's narrow data-view rules instead of
editing main.css (so the main.css contract tests read untouched rules), and that the departures live in
`.planning/sketches/005-phone-layout/README.md`. Two parts:
(a) Base rules outside any media query that hide the new phone only elements at every width: `.feed-heading`,
`.bottom-nav`, `.report-row__away`, `.profile-logout`, `.profile-record`. Nothing else outside the media query.
(b) One `@media (max-width: 899.98px)` block holding every phone rule, built only from existing tokens (no raw hex, no
font size literal, no weight other than the two tokens, corners at most 8px, every target at least 44px).
Phone feed rules in that block:
- Layout: `.app-shell` becomes two rows, the map row `40vh` then `40dvh` (legacy unit first, the same pairing main.css
  uses) and the list row taking the rest, with height `100vh` then `100dvh`; the gap is zero. Both panes always show
  whatever `data-view` says: override main.css's narrow pane rules with heads of equal or higher specificity
  (`.app-shell[data-view] .pane--map`, `.app-shell[data-view] .pane--list`). `.pane--map #map` gets height `40vh` then
  `40dvh`. `.pane--list` becomes its own scroll container (overflow-y auto, overscroll-behavior contain, min-height 0)
  with side padding of the medium space and a bottom padding that clears the bottom bar plus the add button. The page
  itself must not scroll and must not scroll sideways.
- `.app-shell > .view-toggle` display none (owner: the switch is dropped on phones).
- `.app-header` hidden on phones through a distinct head (for example `body > .app-header`); its three items stay
  reachable: Activity through the bottom bar, the email and Log out on the Activity page (Task 3). Record this.
- The theme toggle moves to the top right corner of the map at the medium space token, through a distinct head (for
  example `body > .theme-toggle`). Record that on phones the bottom right corner holds only the add button above the
  bar, so decision D-A (theme toggle below the add button) is not applied on phones.
- The add button: a distinct head (for example `.pane--map > .fab`) puts it at the right medium space and a bottom of
  bar height plus the medium space. Black round with white plus is already its look.
- `#toast:not([hidden])` sits above the bottom bar.
- `.feed-heading`: a flex row, "Nearby now" left at the heading size in the semibold weight, the count right in the
  label size in the muted colour, then the existing `.filter-toggle` under it at the label size (undrawn existing item,
  kept).
- Row cards (`#report-list .report-row`): no left bar, 8px corners, the existing severity tint as the background,
  provisional rows (`.vis-provisional`) white with a 1px border token hairline, hidden rows (`.vis-hidden`) white with
  the hairline, the existing selected outline kept. The category badge is hidden. The row body is a wrapping flex row
  ordered by `order`: severity label, then the state tag, then the meta line, then the distance line on line one; the
  title on its own line (flex-basis 100 percent, clamped to 2 lines with the line clamp pair); then the trust block and
  the reporter words on one line; the vote controls, the too far note and the vote error hidden on phone rows (voting
  happens in the detail, as drawn; record it). `.report-row__stateline:not([hidden])` becomes `display: contents` so its
  tag and reporter words join the body's flex order (keep its empty line guard working). The severity label on rows
  loses its pill (no border, transparent background, no padding), shows the 8px dot for every level including Critical
  (a phone only `::before` on `#report-list .severity-label--critical`), and keeps the semibold word. The meta and
  distance lines use the label size; they are the muted colour on white cards and the normal text colour on tinted
  cards, because muted fails 4.5 on every tint (see the contrast rule; record the swap). Middle dot separators between separate elements on line one
  come from CSS generated content with an empty alternative text, or from plain spacing if a contract test objects;
  record which.
- Trust line on rows: show the face as one line, counts then the currency word then the reporter words, with the
  chevron and the clock icon hidden (the toggle still opens the facts panel in place; record that the visible open cue
  is gone on phones). Colour, as drawn: the face text takes `--color-tone-good` when the row's currency fact is good
  (select with `:has(.trust-fact--currency .trust-fact__glyph--good)` on the row), the muted colour on provisional rows,
  the normal text colour otherwise. The green applies only where the pair clears 4.5 in both themes, so not on the
  critical tint (4.35 in light); there the line stays the normal text colour. Write the computed numbers in the
  departures document; the colouring is a departure from the trust text colour rule (see Step 4).
- Pins (`.leaflet-container .icon-badge--pin`, a head distinct from `.icon-badge`): the badge draws as a dot of about
  20px in `--severity-current` with a 3px `--color-bg` ring and the existing drop shadow, centred in the unchanged 44px
  marker box so the tap target stays 44px; the glyph is hidden. Provisional pins come out grey through the existing
  `.vis-provisional` mix. A pin whose element carries `data-popup-open="true"` or the existing `map-pin--highlight`
  class gets a wider ring in `--color-text`, which is how "tapping a row highlights its pin" shows in the detail strip.
- `.bottom-nav`: fixed to the bottom, full width, z index 1010, `--color-bg` background with a 1px border token top
  edge, two equal tabs each at least 44px tall, label size text, muted colour; the tab with `aria-current="page"` is
  the text colour in the semibold weight with a small `currentColor` dot centred above the word drawn by `::before`.
  No pill shapes.

Step 2, the markup in `web/templates/index.html.tmpl`. The phone.css link from Step 0b follows every app stylesheet
(keep the Leaflet and MapLibre vendor links, their order and their integrity attributes untouched). Inside `.pane--list`, before the
filter toggle, add `<div class="feed-heading">` holding an `h2` with the class `feed-heading__title` and the text
`Nearby now`, and a `p` with `id="feed-count"`, the class `feed-heading__count` and no text. After `#app-shell`, before
the scripts, add `<nav class="bottom-nav" aria-label="Main">` with two links of the class `bottom-nav__tab`: `href="/"`
with `aria-current="page"` and the text `Feed`, and `href="/profile"` with the text `Activity`. No Past link (not built).

Step 3, the script in `web/static/js/feed.js` (no other JS file in this task):
- Count line: inside `render`, after the branch that decides what shows, write `#feed-count` through `Pinalert.setText`:
  when the status is ready, the number of reports in `state.reports`, a space, `active` (or `disputed` when
  `Pinalert.state.showDisputed` is true), a space, the middle dot, a space, then `Pinalert.config.defaultRadiusKm`
  written as a whole number when it is whole and with one decimal otherwise, a space and `km`; an empty string while
  loading or on an error with no rows. Real counts only, never a fixed number (house rule: no fake metrics).
- Distance line: `createRow` adds a `p` with the class `report-row__away` right after the meta line and returns it on
  the row object; `updateRow` writes `Pinalert.formatDistance(report.distance_km)` into it through `Pinalert.setText`
  when `Pinalert.feedIsAroundReader()` is true and sets its `hidden` attribute when the text is empty. It is hidden at
  900px and up by the base rule, so the desktop row does not change.
- Phone list scroll: a new function (for example `revealRowInListPane(id)`) called from the existing
  `Pinalert.onSelect` handler on the `source === 'map'` branch, after `applySelectionHighlight(id)`. It does nothing
  unless `window.matchMedia('(max-width: 899.98px)')` matches and the closest `.pane--list` of `#report-list` is taller
  in content than in box (scrollHeight above clientHeight). It finds the row in `rowsById`, computes the row's top
  inside the pane from the two bounding rectangles plus the pane's current scroll top minus the small space token, and
  sets the PANE's scroll position with its own `scrollTo` (behavior smooth, or auto when reduced motion is preferred).
  It never scrolls the window or the document and never calls the browser's into view helper, and
  `applySelectionHighlight` stays free of any scroll. This amends A87 on phones only: below 900px the list is its own
  scroll box under a fixed height map, so moving the list never moves the map, which was A87's reason. Update the doc
  comment of `TestMapSelectionNeverScrollsThePage` in `web/popup_contract_test.go` with one sentence naming this phone
  amendment; keep every assertion it has.
- Do not touch `activateRow`, the view toggle code or the focus code.

Step 4, tests. Create `web/phone_layout_contract_test.go` (package web, reuse the parse helpers the other contract
tests already have, such as `parseCSSRules` and the `p13`/`p16`/`p19` helpers, rather than writing new parsers):
- phone.css is linked in index.html.tmpl after main.css, feed.css, trust.css, modal.css and auth.css, with the
  AssetVersion query.
- Every rule in phone.css is inside the `max-width: 899.98px` block, except the base hide rules for the five phone
  only classes, which declare only `display: none`.
- phone.css has no rule whose head is exactly a head another test looks up by exact head (`.fab`, `.theme-toggle`,
  `.app-header`, `.account-trigger`, `.profile-page`, `#toast`, `.icon-badge`, `.view-toggle`, `.report-row`).
- index.html.tmpl has the bottom nav with exactly two tabs, Feed current, and no Past link; it has `#feed-count` inside
  `.pane--list`.
- feed.js: the phone scroll function is called only from the map branch of the onSelect handler, contains the
  899.98px media string and scrolls the list pane element; its body has no `window.scroll` call of any kind, no
  `scrollingElement`, and no scroll call on `document.documentElement` or `document.body` (its own `window.matchMedia`
  call is allowed, so do not ban the bare word `window`).
Run the whole web package. For every existing test that now fails, choose: change phone.css so the existing contract
holds (preferred when the picture allows it), or, when the picture contradicts the contract, update that test in this
task so it allows exactly the phone.css heads involved, and add one line to the test's doc comment naming the
departure and pointing at the departures document. Never weaken an assertion about any other file or head. Expected
named updates: the trust text colour rule in `web/trust_contract_test.go` (green and muted trust line on phone rows).

Step 5, the document. Create `.planning/sketches/005-phone-layout/README.md` (text only, no images, no dashes) with a
short intro (the owner's two pictures of 2026-10-13, reproduced as drawn under 900px, desktop unchanged), the pictures
in words (copy the owner_pictures_in_words block), a section "Not built" with the three items and what each needs
(Past tab: Phase 3.1 history views; category chips: Phase 3.1 filters plus the owner's answer on hiding critical or
rescue reports; My votes: a read of votes by account, which no phase has yet), a section "Choices where the picture is
silent or contradicts itself", a section "Wording kept from the app" (row meta keeps `min ago` and `km away`; both
counters on every row per D-16; "Waiting for a second confirmation" is not an app string, the row shows the counters
and the Unconfirmed word), and a section "Departures from the house rules" listing every item from this task: tokens
instead of measured sizes, trust line colours and their contrast numbers, no category glyph on phone pins, no vote
buttons on phone rows, no chevron, no left severity bar on phone cards, A87 amended on phones, D-A not applied on
phones, the account button hidden on phones, and every contract test updated with its file and test name. Tasks 2 and 3
append to this file.

Step 6, verify (see the verify block), then commit with the subject `feat(quick-261013-lrw): phone feed with the map on
top and a bottom bar`, the paths of this task only, and the exact trailer.
  </action>
  <verify>
    <automated>env -i PATH="$PATH" HOME="$HOME" go test ./web/ -count=1</automated>
    Then the full suite on a disposable database, as separate steps:
    1. `psql -lqt | cut -d'|' -f1 | tr -d ' ' | grep -qx p_phone_t1 && echo REFUSING` (stop if it prints REFUSING)
    2. `createdb p_phone_t1`
    3. `env -i PATH="$PATH" HOME="$HOME" DATABASE_URL="postgres://localhost:5432/p_phone_t1?sslmode=disable" go test ./... -p 1 -count=1 -v > <scratchpad>/t1.log 2>&1; echo "EXIT $?" >> <scratchpad>/t1.log`
    4. `tail -1 <scratchpad>/t1.log` shows `EXIT 0`; `grep -c -- '--- SKIP' <scratchpad>/t1.log` prints 0; `grep -c -- '--- FAIL' <scratchpad>/t1.log` prints 0
    5. `dropdb p_phone_t1`
    Then the probe recipe at 390 by 844 in Chromium and WebKit: `#map` height within 2px of 40 percent of innerHeight;
    `.pane--list` displayed and its top at the map's bottom; `#view-toggle` not displayed; `.bottom-nav` displayed at
    the bottom with two tabs and the Feed tab current; `.fab` bottom edge above the bar's top edge; `#feed-count` text
    matches the number of rendered rows; rows have no left border and an 8px radius; `scrollWidth` minus `innerWidth`
    is 0; after clicking a pin the list pane's scrollTop changed, the row is inside the pane's box and `window.scrollY`
    is 0. At 1280 by 800 every value in desktop-baseline.json is unchanged.
  </verify>
  <done>
    The web package and the full suite pass (0 SKIP, 0 FAIL) on a dropped disposable database; the phone probe numbers
    match in both engines; the desktop probe equals the baseline; phone.css, the template markup, the feed.js additions,
    the new contract tests and the first version of the departures document are committed in one path limited commit
    with the exact trailer.
  </done>
</task>

<task type="auto">
  <name>Task 2: Phone report detail as a full screen sheet under a short map strip</name>
  <files>web/static/js/map.js, web/static/css/phone.css, web/phone_layout_contract_test.go, web/popup_contract_test.go, .planning/sketches/005-phone-layout/README.md</files>
  <action>
The detail is the existing popup card, opened on phones in a pane that lives outside the map pane's transform, and laid
out by phone.css as a full screen sheet under a short map strip. One card, one set of focus rules, one Escape path:
nothing in the popup's markup, focus handling or Escape handling changes.

Step 1, `web/static/js/map.js` (comments in the file's own style, no dashes):
- In `init`, right after the map is created, create the pane with `map.createPane('detailPane', container)` (the `#map`
  element, so the pane is a child of the map container and outside the map pane), and call
  `L.DomEvent.disableClickPropagation` and `L.DomEvent.disableScrollPropagation` on it, the same protection Leaflet gives
  its own popup container, so presses and wheel turns on the sheet never reach the map.
- Add a helper (for example `phoneDetail()`) that returns `mediaMatches(NARROW_QUERY)`, and a helper (for example
  `applyPopupPlacement(popup)`) that writes `popup.options.pane` (`'detailPane'` on phones, `'popupPane'` otherwise) and
  `popup.options.autoPan` (`!reducedMotion && !phoneDetail()`). Call it right after `marker.bindPopup(...)` in
  `upsertMarker` (keep the bind call's literal options exactly as they are, the popup contract test pins them) and for
  every marker's popup inside the existing loop in `onMapResize`, next to `applyPanPadding`.
- Breakpoint flip: keep a module flag holding the last narrow answer. In `onMapResize`, when the answer changed and a
  popup is open, close it with `map.closePopup()` so the existing close path returns focus to the opener; the next open
  lands in the right pane. Resizes that do not cross 900px (address bar, on screen keyboard) do nothing new.
- `panPopupIntoView` returns at once on phones (the sheet is fixed, there is nothing to pan into view).
- In `onPopupOpen`, after the existing work, on phones only: put the pin in the middle of the strip. Read the strip
  height as the popup element's bounding top minus the map container's bounding top; call
  `map.setView(popup.getLatLng(), Math.max(map.getZoom(), 16), { animate: false })` (the same street zoom flyTo uses;
  setView stops a running fly), then `map.panBy([0, Math.round(map.getSize().y / 2 - strip / 2)], { animate: false })`.
  Leave the existing focus call as it is; the card is inside a fixed popup, so it is displayed and takes focus.
- Do not change `onPopupClose`, `returnFocusToOpener`, `popupHoldsFocus`, `focusOpenPopup`, `flyTo`, `highlight` or the
  returned API.

Step 2, the detail rules in the phone block of `web/static/css/phone.css` (heads all prefixed with
`.leaflet-container .leaflet-detail-pane`, so they never collide with trust.css's popup rules or their exact head
tests, and stay out of trust.css, which may never hold the important flag):
- The pane: z index 1080.
- The popup inside it: fixed, top at the strip height (`calc(var(--space-3xl) * 3)`, 192px; under `max-height: 359px`
  the strip is `var(--space-3xl)`), right 0, and `left: 0`, `bottom: 0`, `transform: none` each with the important flag,
  because Leaflet writes those three inline on every update and zoom; margin 0; overflow-y auto; overscroll-behavior
  contain; `touch-action: pan-y`. A popup in the pane that is not marked `data-popup-open="true"` (the 200ms fade after a
  close) gets `pointer-events: none`.
- The content wrapper fills the sheet (min-height 100 percent, no corner radius, no shadow) and keeps its existing
  severity tint background, which gives the very light green page on a Low report as drawn. The tip container is hidden.
  The content node gets `width: auto` with the important flag (Leaflet writes 341px inline), `max-height: none` (beats
  trust.css's phone max height), and side margins of the medium space with bottom room of the large space.
- The card: no left severity bar, no max width. The header's category badge and the meta glyph are hidden (the picture
  shows the severity pill alone and a plain grey meta line). The severity pill keeps its outlined dot and word. The
  state line stays (undrawn existing item). The title takes the display size in the semibold weight. The meta line is
  at the label size in the normal text colour, because the sheet is tinted and muted fails 4.5 on every tint (the
  picture's grey; record the swap). The trust card keeps its white surface, header and tick rows unchanged, so its own
  muted sub lines stay on white.
- The fade line sits under the card at the label size in the normal text colour, for the same contrast reason.
- Vote block: the controls row becomes a two column grid; Confirm and Dispute sit side by side, each at least 44px tall
  and full width of its column; Confirm unpressed is the text colour fill with the background colour text, Dispute
  unpressed stays the background colour with the hairline; the button icons are hidden; the existing pressed, disabled
  and busy states still read differently from unpressed (check every state in both themes and write what changed).
  Mark resolved spans both columns as a centred text link look in `--color-tone-good` where that clears 4.5 on the
  sheet's tint in both themes (low, medium and the provisional surface) and in the normal text colour on a critical
  sheet (4.35), still a 44px tall button for touch. The resolve confirmation and Reopen keep their existing look. The
  too far note and the vote error are centred at the label size in the normal text colour.
- The close anchor becomes the round white back arrow on the strip: with a head that beats trust.css's
  `(0,4,1)` close button rule (for example adding `.leaflet-detail-pane` for `(0,5,1)`), fixed at the medium space from
  the top and left, 44px round, `--color-bg` background, the popup shadow token, and its `::before` mask switched to
  `/static/icons/arrow-left.svg`. Leaflet's accessible name `Close popup` stays (record it).
- Detail mode, `body:has(.leaflet-detail-pane .leaflet-popup[data-popup-open="true"])`: hide the Leaflet zoom control
  and the theme toggle so the strip shows only the map, the pin and the back arrow.

Step 3, tests in `web/phone_layout_contract_test.go`: map.js creates `detailPane` with the map container as the
parent; the placement helper writes both `pane` and `autoPan` and is called after `bindPopup` and inside
`onMapResize`; `panPopupIntoView` returns on phones; `onPopupOpen` calls `setView` and `panBy` only behind the phone
check; every rule that uses the important flag in phone.css has a head inside `.leaflet-detail-pane` and sets only
`left`, `bottom`, `transform` or `width`. Run the whole web package; for any existing popup test that now fails, prefer
changing map.js or phone.css so the contract holds; update a test only when the picture requires it, in this task, with
a doc comment line naming the departure.

Step 4, the browser harness, to catch popup focus regressions: `PINALERT_REPO=/Users/swathivallabhaneni/code/pinalert
PINALERT_DB=p_phone_auto PINALERT_PORT=8795 BUILD_FROM=worktree bash ~/code/pinalert-browser-checks/run.sh` (the
database name ends in `_auto` and the harness creates and drops it itself). Any FAIL that comes from the planned phone
change (for example a check that drives the removed narrow map and list switch) is listed in the departures document
with the check number and why; any other FAIL is a defect to fix in this task.

Step 5, append to the departures document: the sheet mechanism (a pane outside the map pane, the four overridden inline
properties, phone.css only), the strip height, no badge and no meta glyph in the detail, button icons hidden, Mark
resolved as a link look, the back arrow keeping the name `Close popup`, the too far sentence shown only when the reader
is too far with the app's own words instead of the picture's always shown hint, and any harness check that changed by
design.

Step 6, verify, then commit with the subject `feat(quick-261013-lrw): phone report detail as a full screen sheet`, this
task's paths only, and the exact trailer.
  </action>
  <verify>
    <automated>env -i PATH="$PATH" HOME="$HOME" go test ./web/ -count=1</automated>
    Then the full suite on the disposable database `p_phone_t2` with exactly the five steps of Task 1 (refuse if it
    exists, createdb, run with -v into `<scratchpad>/t2.log`, EXIT 0 with 0 SKIP and 0 FAIL, dropdb).
    Then the probe recipe at 390 by 844 in Chromium and WebKit: click a row, then the open `.leaflet-popup` is inside
    `.leaflet-detail-pane`, its rect top is 192 within 1px, its bottom equals innerHeight, its width equals innerWidth;
    the open pin's centre is between 0 and 192 vertically; the close anchor is 44 by 44 at the top left; the active
    element is inside the card; Confirm and Dispute have equal widths on one row; pressing Escape or the back arrow
    closes the sheet and focus lands on the row that opened it; clicking a pin opens the sheet and changes the list
    pane's scrollTop with `window.scrollY` at 0. At 1280 by 800: the opened popup's pane is `leaflet-popup-pane`, its
    content width is 341px, and every value in desktop-baseline.json is unchanged.
  </verify>
  <done>
    The phone detail opens from a row and from a pin as a full screen sheet under a 192px strip that shows the pin,
    built from the existing card with no copy changes; the desktop popup is unchanged; the web package, the full suite
    (0 SKIP, 0 FAIL) and the harness pass apart from by design changes written in the departures document; one path
    limited commit with the exact trailer.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Phone Activity page with the reporter record, Log out and the bottom bar, then the final checks</name>
  <files>internal/api/handlers/auth.go, internal/api/handlers/profile_record_test.go, internal/api/handlers/activity_e2e_test.go, web/templates/profile.html.tmpl, web/static/css/phone.css, web/phone_layout_contract_test.go, .planning/sketches/005-phone-layout/README.md</files>
  <behavior>
    - A reporter signal with label `Reliable reporter`, 5 judged and 5 corroborated maps to a record with Label `Reliable reporter` and Counts `5 of 5 confirmed`.
    - A signal with label `New reporter`, 0 judged and 0 corroborated maps to Label `New reporter` and Counts `0 of 0 confirmed` (real numbers, nothing invented).
    - A signal with an empty label (tag none) maps to no record.
    - GET /profile for an account with one fresh report renders the record card with `Your reporter record`, `New reporter` and `0 of 0 confirmed`, and still renders the existing back link, email, Activity heading and the row with its data attributes.
    - GET /profile for an account with no reports renders no record card and still shows the existing empty state.
    - GET /profile renders a Log out form with `method="post"` and `action="/auth/logout"` inside `.profile-activity`, and the bottom nav with the Activity tab current.
  </behavior>
  <action>
Step 1, RED. Create `internal/api/handlers/profile_record_test.go` with table tests for a new unexported helper (for
example `newProfileRecord(service.ReporterSignal) *profileRecord`) covering the first three behaviours, and add the
three GET /profile cases to `internal/api/handlers/activity_e2e_test.go` using its existing `newE2EServer`,
`newVerifiedClient` and `getProfile` helpers. These e2e tests need the database: run them only on the disposable
database of the verify block. Commit nothing until they fail for the right reason.

Step 2, GREEN, `internal/api/handlers/auth.go`: add `type profileRecord struct { Label string; Counts string }` and a
`Record *profileRecord` field on `profileViewModel`, with doc comments in the file's style (no dashes). The helper
returns nil when the signal's Label is empty, else Label copied as the server wrote it and Counts built with
`fmt.Sprintf("%d of %d confirmed", CorroboratedReports, JudgedReports)`. In `Profile`, set Record from the first
`ActivityReport`'s `Trust.Reporter` when there is at least one report (ActivityForAccount gives every row the same
account signal, its own doc comment says so); no report means no record. No new query, no new service call, nothing
recomputed. The bold word stays the server's `Reliable reporter` instead of the picture's `Reliable` (record it: the
app keeps one wording for the tag).

Step 3, `web/templates/profile.html.tmpl` (phone.css is already linked after auth.css by Task 1):
- Inside `section.profile-activity`, right after the `h2`, add `<form class="profile-logout" method="post"
  action="/auth/logout">` with one `<button type="submit" class="profile-logout__button">Log out</button>`, a copy of the
  account header's form with no other field (not a menu item, so no menuitem role and no icon).
- After that form, `{{with .Record}}` a `<section class="profile-record" aria-label="Your reporter record">` holding a
  `p.profile-record__kicker` with `Your reporter record`, a `p.profile-record__label` with `{{.Label}}` and a
  `p.profile-record__counts` with `{{.Counts}}`; `{{end}}`. html/template escapes both values.
- Before the toast, the same `nav.bottom-nav` as index with the Feed link plain and the Activity link carrying
  `aria-current="page"`.
- Keep the back link exactly once and before the email (TestActivityPageLinksBackToTheMap), keep the `h2`, the list, the
  rows' data attributes, the empty state and the scripts unchanged.

Step 4, the Activity rules in the phone block of `web/static/css/phone.css`:
- Page: the background stays `--color-bg` (white). A grey page would put the muted email and the grey Log out at 4.34,
  under 4.5, so the white cards get the 1px border token hairline instead and still read as white rounded cards.
  `.profile-page` through a distinct head (for example `main.profile-page`) gets the medium side space, a top space of
  the medium token and a bottom padding that clears the bottom bar.
- The back link is hidden on phones (the bottom bar replaces it; the markup stays for its test).
- `.profile-activity` becomes `display: contents` and `main.profile-page` a grid with the areas title and logout on the
  first row, email, record, list and empty state below, so "Activity" sits large and bold on the left (the display size,
  semibold) and the grey `Log out` text button (muted colour, body size, regular weight, at least 44px tall, no border,
  transparent) on the right. The theme toggle stays fixed at the top right, so the logout cell keeps a right margin of
  the touch target plus the small space. The email `.profile-email` shows under the title in the muted label size
  (undrawn existing item, kept).
- `.profile-record`: a white (`--color-bg`) card with 8px corners and the medium padding; the kicker in the muted label
  size, the label in the heading size semibold under it, the counts on the right in the label size, vertically centred.
- Report cards (`.profile-report-list .report-row`): white background for every state (picture B draws white cards),
  8px corners, no left bar, the hairline border, the badge hidden. Body as a wrapping flex row: the severity label as
  the dot and bold word on the left (the same phone treatment as feed rows, including the Critical dot), the state tag
  pushed to the right with `margin-left: auto`, coloured as drawn: Unconfirmed in `--color-tone-caution` (the picture's
  amber Waiting), Disputed in `--color-tone-bad` (undrawn, kept in the picture's style), Resolved and Expired in the muted
  colour. Live shows no word (the app gives Live no word by contract; record that the picture's green Active is not
  shown). Then the title in bold on its own line. Then one muted line: the trust face counts and currency word, then the
  meta line (category and time). The reporter words on each row are hidden on phones because the record card shows the
  same server label once (record it). The trust panel toggle still opens the facts panel in place. Reopen keeps its
  existing control and error under the line.
- Any whole directory CSS scan that objects is handled as in Task 1: change phone.css when the picture allows, else
  update the test in this task with a doc comment line naming the departure.

Step 5, tests in `web/phone_layout_contract_test.go`: profile.html.tmpl links phone.css after auth.css; the Log out
form's method and action equal the account header's; the record card is inside `{{with .Record}}`; the bottom nav has
exactly two tabs with Activity current and no Past link; the back link is still present exactly once.

Step 6, finish the departures document: the Activity choices (record card built from the server's label and counts,
`Reliable reporter` instead of `Reliable`, `0 of 0 confirmed` for a new reporter, record and Log out shown on phones only
because 900px and up must not change, the email kept under the title, no Active word for Live, Waiting kept as
Unconfirmed, the picture's status lines "Needs one more confirmation", "Resolved by you · yesterday" and "Not
re-confirmed" not built because they are not app strings, so the row shows the counters, the currency word and the meta
line, per row reporter words hidden on phones, the segmented control not built). Add a closing list "Questions for the
owner" with the three not built items, each with a recommendation in plain words: Past tab, add it when Phase 3.1
builds the Past view; category chips, build them with Phase 3.1's filters once the owner says whether a filter may hide
critical and rescue reports; My votes, plan a read of votes by account as its own small piece of work.

Step 7, final checks: the full suite on the disposable database, the browser harness as in Task 2 Step 4 (fresh
`p_phone_auto`, port 8795), and the probe recipe at 390 by 844 on `/profile` and `/`, and at 1280 by 800 against
desktop-baseline.json. Then commit with the subject `feat(quick-261013-lrw): phone Activity page with the reporter
record`, this task's paths only, and the exact trailer. Delete `$HOME/.cache/pinalert-261013-lrw` after the comparison.
  </action>
  <verify>
    <automated>env -i PATH="$PATH" HOME="$HOME" go test ./web/ ./internal/api/handlers/ -count=1 -run 'Phone|Profile|Activity|Record'</automated>
    That fast run skips the database e2e cases (no DATABASE_URL). The real proof is the full suite on the disposable
    database `p_phone_t3` with exactly the five steps of Task 1 (refuse if it exists, createdb, run with -v into
    `<scratchpad>/t3.log`, EXIT 0 with 0 SKIP and 0 FAIL, dropdb); confirm the new e2e test names appear in t3.log with
    `--- PASS`. Then the harness (fresh `p_phone_auto`, port 8795, `BUILD_FROM=worktree`). Then the probe recipe: at 390
    by 844 on `/profile` in Chromium and WebKit, the title and the Log out button share one row, `.profile-record` is
    displayed with the server's label and counts, `.profile-back-link` is not displayed, every row has an 8px radius and
    a white background, the bottom nav shows Activity current, and `scrollWidth` minus `innerWidth` is 0; at 1280 by 800
    on `/` and `/profile` every value in desktop-baseline.json is unchanged and `.profile-record`, `.profile-logout` and
    `.bottom-nav` are not displayed.
  </verify>
  <done>
    The Activity page at phone width matches picture B within the recorded departures, with every laptop item kept; the
    record comes from the server's own signal with real counts; the full suite passes with 0 SKIP and 0 FAIL on a
    dropped disposable database; the harness passes apart from recorded by design changes; desktop probes equal the
    baseline; the departures document lists every departure and the three owner questions; one path limited commit with
    the exact trailer.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| server to browser | Report text, trust words and the reporter record cross into the page; descriptions are written by strangers |
| browser to server | The phone Log out form posts to the existing logout route |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-lrw-01 | Tampering | feed.js count line and distance line | medium | mitigate | Both are written through `Pinalert.setText` from numbers and fixed words only; no markup parsing sink is added (the existing T-01-03 rule) |
| T-lrw-02 | Information disclosure | profile Record | medium | mitigate | Record is read from ActivityForAccount, which is keyed by the verified account from the gated route, so a page only ever shows its own viewer's signal; html/template escapes Label and Counts |
| T-lrw-03 | Spoofing | phone Log out form | low | accept | The form is a copy of the existing account header form (same method and action, no new field), so it adds no new request shape; logout cross site request risk is unchanged from today |
| T-lrw-04 | Tampering | report trust shown on phone rows | low | mitigate | The phone layout reorders and recolours server written trust words only; both counters stay on every row (D-16) and nothing is computed in the browser |
</threat_model>

<verification>
- `env -i PATH="$PATH" HOME="$HOME" go test ./web/ -count=1` passes after each task.
- The full suite passes after each task on its own fresh disposable database (`p_phone_t1`, `p_phone_t2`, `p_phone_t3`),
  each refused if it already exists and dropped afterwards, run with `-v`, with 0 `--- SKIP` and 0 `--- FAIL`.
- The browser harness passes after Tasks 2 and 3 apart from by design changes written in the departures document.
- Probes at 390 by 844 in Chromium and WebKit match the numbers in each verify block; probes at 1280 by 800 equal the
  baseline captured before the first edit.
- Peer sessions commit to main in bursts, so select this plan's commits by subject, never by position:
  `git log --format='%H %s' -n 50 | grep '(quick-261013-lrw)'` lists exactly three. For each of those shas,
  `git show -s --format=%B <sha>` ends with the exact trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`
  and `git show --stat --format= <sha>` lists only that task's paths.
- No image file was added: the same three `git show --stat` outputs list no png, jpg, gif or webp file and no svg
  outside the existing icons folder.
</verification>

<success_criteria>
- Under 900px the feed, the report detail and the Activity page look like the owner's pictures within the recorded
  departures, from real data and existing wording.
- At 900px and up nothing changes, proven against the baseline probe.
- Every conflict with the house rules and every drawn item not built is written in
  `.planning/sketches/005-phone-layout/README.md`, ending with the three owner questions.
- Contract tests stay green; every changed test names its departure in its doc comment.
</success_criteria>

<output>
Create `.planning/quick/261013-lrw-phone-layout-under-900px-feed-map-on-top/261013-lrw-SUMMARY.md` when done, listing the
three commits, every contract test changed and why, the harness results, the probe numbers at both widths, and the three
owner questions.
</output>
