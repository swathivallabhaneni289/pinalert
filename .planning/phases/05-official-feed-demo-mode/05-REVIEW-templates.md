---
phase: 05
part: templates
reviewed_at: 2026-10-29T12:00:00Z
depth: standard
status: issues_found
findings:
  critical: 0
  warning: 7
  info: 9
  total: 16
files_reviewed: 23
files_reviewed_list:
  - web/static/css/auth.css
  - web/static/css/demo.css
  - web/static/css/feed.css
  - web/static/css/login-globe.css
  - web/static/css/official-pins.css
  - web/static/css/official.css
  - web/static/css/phone.css
  - web/static/css/welcome.css
  - web/templates/alerts.html.tmpl
  - web/templates/app_bar.html.tmpl
  - web/templates/demo.html.tmpl
  - web/templates/demo_activity.html.tmpl
  - web/templates/demo_alerts.html.tmpl
  - web/templates/demo_strip.html.tmpl
  - web/templates/index.html.tmpl
  - web/templates/legal_body.html.tmpl
  - web/templates/login_gate.html.tmpl
  - web/templates/login_globe.html.tmpl
  - web/templates/login_ways_links.html.tmpl
  - web/templates/report_form.html.tmpl
  - web/templates/settings.html.tmpl
  - web/templates/verify_outcome.html.tmpl
  - web/templates/welcome.html.tmpl
---

# Phase 5: Code Review, templates and styles half

**Reviewed:** 2026-10-29, main at 1f44177d
**Depth:** standard (read every file in scope, read the Phase 5 diffs, traced the cascade into main.css, trust.css and the scripts that fill these templates; no browser was run)
**Status:** issues_found

## Summary

No Critical issue. The Demo safety gate holds (`{{if not .Demo}}` wraps the Call 112 `tel:` box and the What is happening picks in `report_form.html.tmpl:66-80`, and all three pages that include the form carry a `Demo` field). No Phase 5 template uses `template.HTML`. Feed text reaches the page through `textContent`, and the GDACS link is prefix checked with `rel="noopener noreferrer"`.

Seven Warnings, four of them the orchestrator's items:

- (a) The banner area is clipped at 320px.
- (b) "11:20 pm" can break at the space.
- (c) The GDACS card's paragraph spacing is zeroed by a trust.css reset.
- (d) The sign in error line has never been red. The probe was right and the 47j expectation was wrong.

Three Warnings were found outside the list:

- The stale notice is a live region that changes every minute.
- A muted line sits on the hover tint at 4.34 to 1.
- The Exit demo name does not contain its visible label.

Geometry figures marked "estimate" are worked out from the 05-15 and 05-19 measured tables, not measured again.

## Critical Issues

None.

## Warnings

### WR-T01: Nearby banner area clipped to about one letter at 320px (orchestrator item a)

**File:** `web/static/css/feed.css:108-205`, `web/static/css/phone.css:221-228`, `web/templates/index.html.tmpl:67-80`, `web/templates/demo.html.tmpl:99-111`

**Issue:** The row is `white-space: nowrap`, and every part except `.alert-banner__area` is `flex: none`. At 320px the area box is 8px with "Thunderstorm warning" and "+2" (05-15 table), and 8px with the Demo tag and "Flood warning +1" (05-19 table). On screen it reads "Thunderstorm warning · N +2" or "Demo Flood warning · N +1". A sighted reader on a small phone sees a warning but not where it applies, and "where" is the reason the banner exists. The full words are only in the accessible name. At 360px with the Demo tag the area gets 48px, about 6 characters, which is acceptable.

**Fix (smallest, CSS only, two lines below 360px):** add this to phone.css inside the existing `max-width: 899.98px` block, after the banner rule:

```css
@media (max-width: 359.98px) {
  .pane--list > .alert-banner:not([hidden]) {
    flex-wrap: wrap;
    align-content: center;
  }
  .pane--list > .alert-banner:not([hidden]) .alert-banner__sep {
    display: none;
  }
  .pane--list > .alert-banner:not([hidden]) .alert-banner__area {
    order: 1;
    flex: 1 0 100%;
    padding-left: calc(20px + var(--space-sm));
  }
}
```

How the layout works:

- Line 1 keeps the icon, the Demo tag, the event, "+2" and the chevron. "+2" still pushes right with `margin-left: auto`.
- Line 2 is the area, which still shortens with its own ellipsis but now has about 225px.
- `order` is visual only, so the accessible name is unchanged. The dot is already `aria-hidden`. The hidden ", " and ". " spans are absolutely positioned and take no part in the rows.
- Two 14px lines at line height 1.4 are about 40 to 44px, so the row stays about 44px and may grow by a few pixels. It never clips.

Estimate, unverified: the first line with the Demo tag, "Thunderstorm warning" and "+2" together is about 288px, over the roughly 255px available, so the event would overflow. That pairing cannot happen today because the Demo sample is "Flood warning". Add `min-width: 0; overflow: hidden; text-overflow: ellipsis` to `.alert-banner__event` inside the same block as a guard.

This departs from UI-SPEC 2a and R3 ("one line, never wraps"), so it needs a dated 2a amendment. The owner has delegated polish, so this is a recommendation, not a question.

**One line alternative:** below 360px, clip the word " warning" with the visually hidden technique (never `display: none`, which would drop it from the name). This needs " warning" in its own span, which is a change to official-banner.js.

**Test:** amend `TestBannerOneLineAndTarget` (`web/official_banner_contract_test.go:107`) to allow the wrap only inside a `max-width: 359.98px` block. In a browser check at 320x568, for "Thunderstorm warning +2" and for the Demo sample, assert all three:

- the `#alert-banner-area` client width is at least 80px;
- the banner height is 48px or less;
- `scrollWidth` equals `innerWidth`.

### WR-T02: "11:20 pm" can break between the time and "pm" (orchestrator item b)

**File:** `web/static/js/official-alerts.js:118` and `:366`, `web/static/js/official-pins.js:141` (outside the file list; they write text into `alerts.html.tmpl:31-34` and the GDACS card)

**Issue:** Both `clockText` functions build the time from parts and join minute and period with an ASCII space: `hour + ':' + minute + ' ' + period`. The comment says this keeps the narrow no break space from Intl out, but it also removes the no break. The stale notice is one text node in a box about 234px wide at 320px: "We could not check for alerts since Sun 11:20 pm (2 hours ago)." The line can end after "11:20" and leave "pm" at the start of the next line. This also happens at larger text sizes, and on the phone row's narrow Until cell ("Until about Sun 11:20 pm"). CSS cannot fix it, because the notice is a single text node.

**Fix:** in both files, use `hour + ':' + minute + ' ' + period`. Optionally use `' '` between the weekday and the clock in `clockWithDay` too. `TestUserVisibleCopyUsesPlainPunctuation` bans only the long and short dash characters, so U+00A0 passes it.

**Test:** update the `official_alerts_test.mjs` assertions that expect "2:05 pm" with a plain space (lines 392, 394, 406, 520 and the pins tests), and add one assertion that the character before "pm" is U+00A0.

### WR-T03: GDACS card lines and honesty line lose their spacing on phone and laptop (orchestrator item c)

**File:** `web/static/css/official-pins.css:235-256`; the overriding rule is `web/static/css/trust.css:1361`

**Issue:** The card is set as Leaflet popup content on both sizes (`map.js` `openCard`). `.leaflet-container .leaflet-popup-content p { margin: 0 }` has specificity 0,2,1. Leaflet's own `.leaflet-popup-content p` (0,1,1) also outranks the card rules. Both beat the single class rules `.official-card__line`, `.official-card__detail`, `.official-card__honesty` and `.official-card__credit` (0,1,0). The effects:

- The 4px gap under each line is dead.
- The 12px space that sets the honesty line ("Automatic estimate. Follow your state disaster management authority.") apart from the facts above it is dead.

On a phone sheet and in the 340px laptop popup, the card reads as one block of seven unspaced 14px lines. The trust.css comment at line 1359 says popup spacing rules need at least three classes. The 05-22 browser table checked content, not spacing.

The title (an `h3`) and the link (an `a`) are not `p`, so they keep their margins.

**Fix:** prefix the four rules with the container:

```css
.leaflet-container .official-card .official-card__line,
.leaflet-container .official-card .official-card__detail,
.leaflet-container .official-card .official-card__honesty,
.leaflet-container .official-card .official-card__credit { margin: 0 0 var(--space-xs); }
.leaflet-container .official-card .official-card__honesty { margin-top: var(--space-12); }
```

**Test:** in a browser check with an open card, assert that the computed `margin-top` of `.official-card__honesty` is 12px. A contract test can pin that each card spacing rule's selector carries at least three classes.

Also checked for item (c):

- The header band leaves 52px for the laptop close button.
- The link is a 44px outlined button.
- Dark theme pins are a light rectangle with dark text.
- The forced colours rules keep the pin fill.
- There are no vote, trust, Flag or Mute parts.
- The popup background is `--color-bg` on both sizes (no `sev-*` class reaches the official popup), so the muted credit is 4.83 to 1 in light.

### WR-T04: The sign in error line is not red. The probe was right (orchestrator item d)

**File:** `web/static/css/auth.css:46-56` against `:163-169`; `web/static/css/login-globe.css:185-192`; `web/login_globe_contract_test.go:716-718`

**Issue:** `#auth-error` is a `<p>` inside `.login-card` (`login_gate.html.tmpl:29`). `.login-card p` (0,1,1) beats `.auth-error` (0,1,0), so the line is drawn in `--color-text` at 16px with body line height, not in `--color-severity-critical` at 14px. The 47j probe's 15.20 (light) and 16.15 (dark) are exactly #1A1D21 on #F1F3F4 and #F3F4F6 on #16181C, the text colour on the surface.

So:

- The colours on screen pass contrast with room to spare.
- The 47j comment ("on the solid surface 4.87 light and 5.46 dark") and the summary's "harness parse" explanation are wrong.
- `TestLoginGlassTextContrast` pins a red on surface pair the cascade never paints.

`#link-expiry-notice` loses the same way to `#check-inbox p`. `.login-done` already uses the `.login-card` prefix workaround for exactly this, but `.auth-error` never got it. The cause dates from 01.1-01 (2026-09-11) and is not a Phase 5 regression. Phase 5 wrote the false expectation.

The solid backing itself is correct: it is present, `:not(:empty)` keeps an empty line free of it, and it stays solid in every fallback.

**Fix (choose one):**

1. Keep the text colour. This fits the rule that colour is reserved for severity. Drop the dead `color` and size from `.auth-error`, correct the login-globe.css comment, and change the test to check `--color-text` on the surface. If you want, also drop the backing, because text on the glass already measures at least 4.99.
2. Make it red, as auth.css intended: `.login-card .auth-error, #check-inbox .auth-error { color: ...; font-size: ...; line-height: ...; }`. This is a visible change the owner has never seen: red text, 14px, and a different margin.

Recommend option 1.

**Test:** a browser check that the computed `color` of a non-empty `#auth-error` equals the chosen token, plus a contract test that resolves the winning rule by specificity instead of reading `.auth-error` alone.

### WR-T05: The stale notice is a live region that changes every minute

**File:** `web/templates/alerts.html.tmpl:31`, `web/templates/demo_alerts.html.tmpl:56`; the writer is `official-alerts.js:366` on the 60 second tick at `:694`

**Issue:** `#official-notice` carries `role="status"`. The stale state starts at 30 minutes, and `relativeAge` reads "N minutes ago" up to 59. The minute tick therefore rewrites the notice text every minute for up to 30 minutes, and a screen reader reads "We could not check for alerts since 2:05 pm (34 minutes ago)." again and again. The UI-SPEC made the status line deliberately not live ("no chatter"), but the notice repeats the same changing age.

**Fix:** remove `role="status"` from `#official-notice` in both templates. If you want an announcement, send one sentence to `#notice-live` only when the state changes into stale.

**Test:** a node test that drives two minute ticks in the stale state and asserts that no element with `role="status"` or `aria-live` changed text. A contract test can assert that `#official-notice` has no `role`.

### WR-T06: Muted text sits on the hover tint at 4.34 to 1 on laptop rows

**File:** `web/static/css/official.css:177-183` and `:229-233`

**Issue:** `.official-row:hover` fills the row with `--color-surface`, while `.official-row__area` and `.official-row__grouped` stay `--color-text-muted`. In the light theme, #6B7280 on #F1F3F4 is 4.34 to 1, under 4.5. auth.css cites the same figure, and the official.css header says muted text appears "on --color-bg only". A mouse user who points at a row to read its area ("6 mandals", "45 alerts like this") gets the text at its lowest contrast. The dark theme is about 7 to 1 and is fine.

**Fix:** add `.official-row:hover .official-row__area, .official-row:hover .official-row__grouped { color: var(--color-text); }` inside the hover media query, or hover with a 1px inset border instead of a fill.

**Test:** add the muted on surface pair to `TestPhase4ContrastPairs` (it fails at 4.34), or add a contract test that no rule with `:hover` sets `--color-surface` on an element whose children are muted.

### WR-T07: The Exit demo accessible name does not contain its visible label

**File:** `web/templates/demo_strip.html.tmpl:12`

**Issue:** The visible text is "Exit demo" and `aria-label` is "Exit the demo". WCAG 2.5.3 (Label in Name, level A) asks for the name to contain the visible text, and "Exit the demo" does not contain the string "Exit demo". A Voice Control or Dragon user who says "tap Exit demo" can miss the control that leaves the sandbox. The UI-SPEC copy (P5-C15) asked for this, so the defect is in the spec.

**Fix:** remove the `aria-label` (the name becomes "Exit demo"), or change the visible text to "Exit the demo". Amend P5-C15.

**Test:** a contract test that every `aria-label` on a link or button with visible text contains that text, case insensitive.

## Info

### IN-T01: On a phone the GDACS card focuses a hidden heading named "Report"

**File:** `web/static/js/map.js:2234-2252` and `:2369` (outside the file list)

The phone detail sheet gives every popup the clipped `h2` "Report", and `openOfficialPopup` focuses it. The 05-22 summary table confirms "heading Report focused". A screen reader user hears "Report, heading" for a GDACS hazard, which UI-SPEC 3b says must be "plainly a different thing". Fix: in `openOfficialPopup`, set that heading's text to "Official hazard", or focus `#official-card-title`. Test: node test on the phone path that the focused element's text is not "Report".

### IN-T02: The Exit demo label is 14px, but the spec says Body 16px

**File:** `web/static/css/demo.css:53`

The UI-SPEC Typography table lists "buttons (Exit demo, Try the demo)" at 16px. demo.css sets `--font-size-label`. Either drop the line, so `.btn` gives 16px, or record the departure. The strip text wraps at 320px either way, which is allowed.

### IN-T03: The class name `.official-card` means two different things

**File:** `web/static/css/official.css:417-475` (the phone alert card, `official-row official-card`) and `web/static/css/official-pins.css:115-127` (the GDACS card)

No page loads both files today: index loads only official-pins.css, and alerts and the Demo pages load only official.css. A future page that loads both would put `display: block` on the alert row grid and remove its focus ring (`outline: none`). Rename one, for example to `.official-row--card`.

### IN-T04: The Green chip vanishes into the GDACS header band

**File:** `web/static/css/official-pins.css:129-136` and `:199-202`

The neutral chip fill and the band are both `--color-surface`, and the border is #E1E4E8 on #F1F3F4, about 1.1 to 1, so the chip's edge cannot be seen. The word and the icon still carry the level, so nothing depends on colour alone. Consider giving `--color-bg` to the chip fill inside the band.

### IN-T05: The gap under the first status line differs by size

**File:** `web/static/css/official.css:43-45` and `:482-484`

The hazard line sits 4px under the first status line on a phone, but 12px on a laptop (the first line's 8px bottom padding plus 4px). The comment says "sits close under the first". Zero the first line's bottom padding when the hazard line follows, or accept the difference.

### IN-T06: The draft Policy sentences went live with a terms version change

**File:** `web/templates/legal_body.html.tmpl:137-139`

The P5-C20 sentences are drafts the owner has yet to word. 05-20 moved `CurrentTermsVersion`, so every account agrees again now, and will agree again when the owner picks the final words. Worth telling the owner before the next deploy. The wording itself is clean (see Checked and clean).

### IN-T07: Reduce Transparency on an iPhone still gets the glass

**File:** `web/static/css/login-globe.css:344-379`

Safari does not expose `prefers-reduced-transparency`, so an iPhone with Reduce Transparency turned on still gets the 65 percent glass. The text still measures at least 4.99 to 1 (the test log), so this is a platform note, not a contrast failure.

### IN-T08: The Demo's dashed ring shares a look with the phone's Expired pin

**File:** `web/static/css/demo.css:159-162` against `web/static/css/phone.css:445-453`

On a phone, an Expired crowd pin is a white dot with a 3px dashed border. A Demo pin is a dot with a 2px dashed outer ring. The two never share a map. An Ended sample on the Demo map would carry both dashed rings. Unverified whether ended samples are drawn as pins in the Demo.

### IN-T09: `.demo-tag` is defined twice

**File:** `web/static/css/official.css:307-329` and `web/static/css/demo.css:121-136`

The values match, except for demo.css's 4px end margin. Two definitions can drift apart. Keep one, in demo.css, and load it on the Demo alerts page as that page already does.

## Checked and clean

- **Glass contrast over the full strength Earth.** `TestLoginGlassTextContrast` passes, and its log gives the lowest text ratio as 4.99 (dark surface over white). The panel buttons are a second glass layer that only moves the backing further from the text colour. Every line in the card, Check your inbox and the panel is `--color-text`. `#resend-link:disabled` is muted on the glass but exempt as disabled. The focus ring tokens (#1A1D21, #F3F4F6) clear 3 to 1 on the glass.
- **Solid fallbacks.** Three paths restore the solid boxes: `@supports not` backdrop-filter, `@supports not color-mix` (with a var free test value, which is correct), and `prefers-reduced-transparency`. Low data renders no `has-globe`, so the boxes stay solid there. The error line's backing is solid in every path.
- **Credit.** Muted links on a solid `--color-bg`: 4.83 to 1 light, about 7.5 dark. 44px targets. Hidden until ready.
- **Phone globe at 320, 375 and 390.** `--g` is bounded by `100vw - 48px`, so the box never exceeds the 24px gate padding. The vh value comes first, with svh under `@supports`. The 47j table shows no overflow at 320x568, 375x548, 375x667 and 390x844. The send button is on the first screen.
- **Template escaping.**
  - No Phase 5 handler uses `template.HTML`. The only `template.URL`s are the pre-existing `tel:` and `mailto:`.
  - The Demo `ExitHref` and `AlertsHref` are plain strings in URL attributes, and Exit is one of two constants.
  - `data-visit`, `data-api-base` and the coordinates are escaped attributes.
  - Alert, banner and GDACS text goes through `textContent`, and the GDACS link passes a prefix and character check.
- **Demo marks.**
  - The Demo tag is on banner, rows and cards. The dashed ring's id selector beats phone.css's disputed `outline: 0`.
  - There is no Official badge on samples, and the Source line reads "Made up for the Demo. Not from any issuer."
  - The attribution reads "These alerts are made up for the Demo." There is no laptop Unofficial strip and no GDACS layer in the Demo.
  - demo.js adds the visit and `from` to the wordmark and Post a report links, so they do not start a new world.
- **The GDACS pin is told apart without colour.** It is a flat 28px rectangle with the word GDACS and a glyph, with no severity hue, against round crowd dots and dashed ring Demo dots. The forced colours rules keep it filled.
- **Policy sentences.** They say "copied from NDMA SACHET" and credit GDACS, NASA GIBS and LGD via Datameet. Nothing claims NDMA approval or partnership, and line 19 still says Pinalert is not connected to any government agency.
- **Silent rule.** No "buzz", "vibrate", "sound" or "ring" appears in any visible Phase 5 string. The only hits are CSS comments about focus rings.
- **Dashes.** There is no long or short dash in any visible Phase 5 string. The hits are in CSS comments that predate Phase 5.
- **Settings row.** It mirrors row 2b. The wrapper is never hidden, so the hairline holds. The "Add a home area first" variant is in the spec. W1 is updated.
- **Welcome.**
  - The step 1 pair stacks below 340px, in source order.
  - The `data-unread` hold targets `#welcome-consent-actions` by id, so the new `--pair` class on step 1 is never hidden.
  - The Disagree box has three controls, with the third spanning the full width on a phone.
- **Sign in link row.** Three 44px outlined links, stacked below 360px, with no middle dots.
- **Shapes and type.** Radii are 4, 8 and 12px, with no pill shapes. Weights are 400 and 600. The only size outside 14, 16, 18 and 24 is the 12px caption on the Official alerts laptop column heads. The UI-SPEC allows it (03.4 closed list), but it sits outside the four size rule in the orchestrator's brief.
- **`verify_outcome.html.tmpl`.** The flat globe is removed, and `{{.CurrentEmail}}` is escaped.

---

_Reviewed: 2026-10-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
