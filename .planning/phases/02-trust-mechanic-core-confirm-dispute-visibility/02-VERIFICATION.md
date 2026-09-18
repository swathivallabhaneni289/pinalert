---
phase: 02-trust-mechanic-core-confirm-dispute-visibility
verified: 2026-09-18T20:00:00Z
status: human_needed
score: 8/13 must-haves verified
behavior_unverified: 5
overrides_applied: 0
mode_guard:
  phase_mode: mvp
  goal_is_user_story: false
  note: >
    Carried forward from the prior 02-VERIFICATION.md (2026-09-16), unresolved. ROADMAP.md still
    marks this phase Mode: mvp, but the phase's Goal field ("A user can confirm or dispute a
    report, and the resulting Hidden/Provisional/Live/Retracted visibility is computed by one
    shared, concurrency-safe resolver everywhere it's shown.") is not in the required
    "As a <role>, I want to <capability>, so that <outcome>." format the MVP Mode Verification
    guard requires. Per that guard, full MVP-mode narrowing (the User Flow Coverage table) is
    refused and NOT produced; this report again verifies directly against ROADMAP's 5 explicit
    Success Criteria plus the gap-closure plans' own must_haves. A human should decide whether to
    fix the Goal field's wording or clear the Mode: mvp flag for Phase 2 so a future verification
    pass doesn't hit the same guard again.
re_verification:
  previous_status: human_needed
  previous_score: "5/5 (ROADMAP SCs only; no gaps: block existed to parse — see Method)"
  gaps_closed:
    - "D-15: Mark Resolved button row now actually disappears (replaced) instead of rendering beside the Yes/Cancel confirmation, on both feed row and map popup (trust.css .vote-btn:not([hidden]) guard)"
    - "Map popup 'Mark this report resolved?' heading legibility: Leaflet popup surface is now theme-aware (background/color override) instead of a hard-coded white Leaflet box"
    - "Stale feed after Reopen + Back navigation: feed.js now refetches on pageshow when event.persisted is true; GET /api/reports now sets Cache-Control: no-store on every exit path"
    - "Activity page had no way back to the map: profile.html.tmpl now has a 'Back to map' link as the first element in <main>"
    - "Show disputed filter reset on reload (user-requested reversal): filter state now round-trips through the page URL (history.replaceState, show_disputed param), restored before the first fetch"
    - "No in-app light/dark toggle (user-requested new feature): theme.js + account-menu Theme control (System/Light/Dark), applied pre-paint via a non-deferred head script on all 4 full-page templates, with color-scheme added to all 4 theme sources in main.css"
  gaps_remaining: []
  regressions: []
gaps: []
deferred:
  - truth: "A report's visibility state is identical on the triage view and the shareable card, in addition to feed/map/Activity"
    addressed_in: "Phase 6"
    evidence: "REQUIREMENTS.md Traceability maps COORD-04 (triage list) and COORD-08 (shareable card) to Phase 6; neither surface exists yet in the codebase."
behavior_unverified_items:
  - truth: "Tapping 'Mark resolved' REPLACES the Confirm/Dispute/Mark-resolved button row in place with the inline Yes/Cancel confirmation, on both the feed row and the map pin popup (02-08 gap closure, D-15)"
    test: "In a real browser (both themes), tap Mark resolved on someone else's report on the feed row and on a map pin popup. Confirm the three primary buttons visually disappear the instant the confirmation appears, rather than rendering beside it."
    why_human: "The fix is a CSS selector-head guard (`:not([hidden])`) proven present by static parsing and by the same two touch-target contract tests that previously failed to catch its absence. Static CSS parsing cannot prove a real browser's rendered layout actually stops painting the three buttons — that is exactly the class of claim this repository has now shipped wrong once already (the original UAT gap) from a test suite that was green throughout."
  - truth: "The map popup's 'Mark this report resolved?' heading is legible in dark mode, because the popup surface now resolves to app tokens instead of Leaflet's hard-coded white box (02-08 gap closure)"
    test: "Open a map pin popup in dark mode and tap Mark resolved. Confirm the heading text is clearly legible against the popup's own background, at contrast comparable to the buttons beside it. Repeat in light mode and confirm nothing regressed (the fix's own falsifiable prediction is that the pre-fix bug never reproduced in light mode)."
    why_human: "02-08's new tests (`TestMapPopupSurfaceIsThemeAware`) prove the override's declarations exist, resolve through the correct CSS custom properties, and that the token pair clears 4.5:1 WCAG contrast arithmetically — they do not composite the page in a real browser. The plan's own SUMMARY explicitly leaves the light-mode falsifiability check and the live dark-mode legibility check to this human-check step, not yet run."
  - truth: "Navigating Back to the feed after tapping Reopen on the Activity page shows the reopened report live again with no manual reload, specifically on Safari (02-09 gap closure)"
    test: "Resolve a report, open Activity, tap Reopen, press the browser's Back button (Safari first, then one Chromium browser). Confirm the feed shows the reopened report live immediately with no manual reload, and that the Network panel shows a fresh GET /api/reports fired on the restore."
    why_human: "The fix (a `pageshow` listener guarded on `event.persisted`, firing before any refetch) is proven wired and correctly ordered by static inspection of the embedded JS source only. 02-09-SUMMARY states directly that the pre-fix Safari bfcache symptom this fix targets was never independently reproduced live in the original UAT session — the diagnosis is the most likely explanation, not a confirmed one, and only a live Safari Back navigation can confirm the fix actually closes it."
  - truth: "Reloading the page while 'Show disputed reports' is checked keeps the same filtered view, restored before the first render with no flash of the unfiltered feed (02-09 gap closure, user-requested reversal)"
    test: "Check 'Show disputed reports', reload the page. Confirm the box is still checked and the disputed view shows from the very first paint, with no visible flash of the default feed first. Copy the URL with the parameter set, open in a new tab, confirm it loads the disputed view directly."
    why_human: "The 'restored before the first fetch' guarantee rests on an argument about JS module/script execution order (deferred module bodies run to completion before map.js's async geolocation callback fires the first fetch) that 02-09-SUMMARY itself flags as an argument, not a proof — and explicitly names what would invalidate it (a future change to script order). Static tests can confirm the code shape; they cannot observe whether a real browser ever paints an unfiltered frame first."
  - truth: "The chosen theme is applied before the page paints (no flash of the wrong palette on reload), and native form controls/scrollbars follow the chosen theme too, across every full page including post-logout (02-10 gap closure)"
    test: "Set the theme to Dark, reload — confirm no flash of light first. Set Light, log out, confirm the login gate renders light rather than snapping to a dark OS default. Toggle Dark and confirm native checkboxes and the scrollbar also go dark. As 02-10's human-check step 6 requires: walk all of Phase 2's UI once in light mode (never done — every UAT screenshot was dark) and report anything illegible."
    why_human: "Presence of a non-deferred head script with no `defer`/`async` and a `color-scheme` CSS declaration is provable by static inspection (done, both pass); a real paint-flash timing effect and real rendered legibility across every surface in a theme that has never once been visually inspected in this project cannot be. 02-10-SUMMARY states explicitly this walkthrough 'has not been performed as part of this plan's execution.'"
human_verification:
  - test: "D-18 GPS-denial hard block — DevTools no-request check, sessionStorage same-tab/new-tab caching, and map-popup parity on the deny path specifically"
    expected: "Denying the location prompt sends zero network requests (verified in the Network panel, not just by visible outcome) and shows the exact GPS-denial copy; a granted prompt is cached per session (not re-prompted same-tab, re-prompted new-tab); the map popup behaves identically to the feed row."
    why_human: "02-UAT.md Test 1 recorded this as 'pass' but with an explicit caveat: the DevTools Network-tab check, the sessionStorage caching behavior, and map-popup deny-path parity were never independently re-confirmed live after the root-cause (OS Location Services being off) was fixed. 'No defect found in anything actually observed; these remain untested rather than failed' — carried forward verbatim from 02-UAT.md, not resolved by any of the three gap-closure plans (none of which touch votes.js's GPS-transport code)."
  - test: "Provisional dimming + 'Unconfirmed' chip visual treatment, and the 'No disputed reports nearby' empty-state copy, in a real browser"
    expected: "A fresh non-critical report shows both a desaturated badge/border and an explicit 'Unconfirmed' chip, on both the feed row and the map pin popup. An empty disputed-view result shows the specific 'No disputed reports nearby' copy, not a blank list."
    why_human: "02-UAT.md Test 2 recorded these two items as 'never visually confirmed either way (only checked via API response, not a screenshot)' when the rest of Test 2 was marked 'issue' (now resolved by 02-09). Neither item was re-tested by any of the three gap-closure plans, none of which touch visibility.js or the badge/chip rendering path."
  - test: "The five items in behavior_unverified_items above (Mark Resolved button replacement in both themes; map popup heading legibility in both themes; Safari Back-after-Reopen refetch; disputed-filter reload persistence with no unfiltered flash; theme toggle no-flash + native-control theming + full light-mode UI walkthrough)"
    expected: "See each item's own test/expected text above."
    why_human: "Every one of these gaps was a browser-rendering or browser-caching bug the static/unit test suite was green throughout while the bug shipped. The gap-closure plans' own SUMMARYs and human-check sections explicitly defer the actual confirmation to this end-of-phase human pass and have not yet been run against a live browser."
---

# Phase 2: Trust Mechanic Core — Confirm/Dispute & Visibility Verification Report

**Phase Goal:** A user can confirm or dispute a report, and the resulting Hidden/Provisional/Live/Retracted visibility is computed by one shared, concurrency-safe resolver everywhere it's shown.
**Verified:** 2026-09-18
**Status:** human_needed
**Re-verification:** Yes — after gap closure (02-08, 02-09, 02-10) and a code review (02-REVIEW.md)

## Method

The prior `02-VERIFICATION.md` (2026-09-16) recorded `status: human_needed` with no `gaps:` block
(all 5 ROADMAP Success Criteria were VERIFIED at that time; only planner-deferred human-check items
were outstanding). A human UAT walkthrough (`02-UAT.md`, `status: diagnosed`) subsequently ran those
deferred checks and found **6 gaps** (3 major, 3 minor — 4 defects, 2 user-requested scope
changes/new features). Three gap-closure plans (`02-08`, `02-09`, `02-10`, all `wave: 8`, mutually
parallel, zero shared files) closed all 6. A code review (`02-REVIEW.md`) then ran and found 1
Critical, 3 Warning, 2 Info findings — advisory-only per this project's workflow, not a phase
must-have gate (see "Advisory Item" section below).

This re-verification:

1. Re-checked all 5 ROADMAP Success Criteria against the current codebase (regression check — no
   code in this area changed since the prior pass, confirmed by `git log` on the relevant files).
2. Verified, against the actual codebase (not the SUMMARYs' narration), that each of the 6 UAT gaps
   has a real corresponding fix: grepped for the specific fix artifacts, then read the surrounding
   code directly.
3. Ran the **full test suite once against a real local Postgres 16 instance** (a scratch
   `pinalert_verify_test2` database, created and dropped by this verifier), plus the 13 new/extended
   gap-closure tests individually by name, so their pass status is this verifier's own observation,
   not SUMMARY.md's claim:

```
go test -p 1 ./...
ok  	pinalert/internal/api            0.835s
ok  	pinalert/internal/api/handlers   1.269s
ok  	pinalert/internal/auth           (cached)
ok  	pinalert/internal/mailer         (cached)
ok  	pinalert/internal/ratelimit      (cached)
ok  	pinalert/internal/service        (cached)
ok  	pinalert/internal/session        (cached)
ok  	pinalert/internal/store          3.913s
ok  	pinalert/internal/testutil       0.576s
ok  	pinalert/web                     (cached)
```

All 13 gap-closure tests (`TestVoteBlockHiddenGuard`, `TestMapPopupSurfaceIsThemeAware`,
`TestResolveConfirmPaintsItsOwnSurface`, `TestFeedRefetchesOnBackForwardCacheRestore`,
`TestDisputedFilterIsCarriedInThePageURL`, `TestFeedResponseIsNotCached`,
`TestActivityPageLinksBackToTheMap`, `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage`,
`TestThemeModuleHasNoAppShellDependency`, `TestThemeModuleUsesNoMarkupParsingSink`,
`TestThemeModuleValidatesStoredModeBeforeReflectingIt`, `TestThemeModuleGuardsStorageAccess`,
`TestAccountHeaderRendersThemeControl`, `TestThemeOverrideBlocksExistForBothModes`) were also run
individually by name with `-v` and every one printed `--- PASS`. `go build ./...` and `go vet ./...`
are clean.

4. Applied the treatment Step 3 of this workflow requires for behavior-dependent truths: every gap
   this round closed is a **rendered-visibility or browser-caching invariant** (does a button
   disappear on screen; is text legible in a real compositor; does a real Safari bfcache restore
   refetch; does a real reload avoid an unfiltered flash; does a real paint avoid a palette flash).
   Grep/static-parse evidence proves the code is present, wired, and — critically — that the
   contract tests which failed to catch each original bug now encode the specific invariant that
   was missing. It does **not** prove the browser-rendered behavior itself, which is exactly the
   class of claim this same repository shipped wrong once already while its test suite stayed green
   throughout. Each of these 5 items is therefore marked `PRESENT_BEHAVIOR_UNVERIFIED` below, not
   `VERIFIED`, per this workflow's Step 3/Step 9 behavior-dependent-truth rule, and routed to human
   verification rather than counted toward the score.

## Goal Achievement

### Observable Truths — ROADMAP Success Criteria (regression check, unchanged since 2026-09-16)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A user can confirm or dispute another user's report, and concurrent votes never silently lose an update, verified by an automated concurrency test | ✓ VERIFIED | Unchanged since the prior pass. `internal/store/votes_test.go`'s `TestCastVoteConcurrent` and `TestCastVoteConcurrentSameAccountKeepsEveryRow` re-ran against a real Postgres in this pass's full-suite invocation and passed. No file in this area was touched by any 02-08/09/10 gap-closure plan (confirmed: none of their `files_modified` lists include `internal/store/` or `internal/service/trust.go`). |
| 2 | A newly submitted non-critical report displays as "provisional" until a second independent confirmation; a critical report publishes at full visibility immediately | ✓ VERIFIED | Unchanged. `internal/service/visibility.go`'s `Resolve()`/`criticalBypass()` untouched by any gap-closure plan; `TestResolve_ProvisionalGate` and `TestIndependentConfirmsFlipProvisionalToLive` re-ran and passed. |
| 3 | Only a distinct verified account AND a distinct geohash cell counts toward independence; severity affects triage order only | ✓ VERIFIED | Unchanged. `internal/service/trust.go`'s `independentCellCount` untouched; `TestResolve_SeverityNeverBypassesGateAlone` re-ran and passed. |
| 4 | A report's visibility state is identical everywhere it's shown — feed, map, triage view, shareable card — via one shared resolver | ✓ VERIFIED (for surfaces that exist; see `deferred`) | Unchanged. `grep -rn "Resolve("` across `internal/service` still finds exactly the 3 call sites recorded in the prior pass. `TestFeedVisibilityMatchesCastVoteResponse` re-ran and passed. Triage view/shareable card still correctly deferred to Phase 6. |
| 5 | A user (reporter or nearby confirmer) can mark a report resolved, removing it from the live feed | ✓ VERIFIED | Server-side resolve/reopen logic (`VoteKindResolution`, `IndependentAgreementThreshold` gate, `ListableInFeed`) is unchanged and re-verified — `TestFeedNeverReturnsRetractedReports`, `TestReporterCanResolveOwnReportInstantly` re-ran and passed. **The client-side click-through UX this truth also depends on** (the button-replacement, popup legibility, and post-Reopen feed freshness) is the exact surface the 6 UAT gaps hit and this round's gap-closure plans targeted — see the gap-closure items below, several of which remain `PRESENT_BEHAVIOR_UNVERIFIED` rather than fully closed. |

### Observable Truths — 02-08/09/10 Gap-Closure Must-Haves

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 6 | 02-08: trust.css's base vote-button rule carries `:not([hidden])`; the two exact-head test lookups were moved, not relaxed | ✓ VERIFIED | `grep -n "^\.vote-btn" web/static/css/trust.css` shows `.vote-btn:not([hidden]) {` at line 36; the unguarded head (`grep -cE '^\.vote-btn[[:space:]]*\{'`) returns 0. `ruleBySelector` in `web/css_contract_test.go` is byte-identical (not weakened to substring matching) — confirmed by reading the file; the two callers were realigned. |
| 7 | 02-08: the Leaflet popup surface is theme-aware (background/color resolve through app tokens, ancestor-field-guarded against the vendor-load-order trap, no raw color/no `!important`) | ✓ VERIFIED (structurally — see item 2 in `behavior_unverified_items` for the rendered claim) | `web/static/css/trust.css:380-381` declares `.leaflet-container .leaflet-popup-content-wrapper, .leaflet-container .leaflet-popup-tip`. `TestMapPopupSurfaceIsThemeAware` (asserts the ancestor field, the WCAG 4.5:1 pair, and that the vendor stylesheet is still linked) ran and passed. |
| 8 | 02-09: `feed.js` refetches on a `pageshow` event when `event.persisted` is true, no `unload`/`beforeunload` listener added | ✓ VERIFIED (structurally — see item 3 in `behavior_unverified_items` for the rendered/live-Safari claim) | `web/static/js/feed.js:625` registers exactly one `pageshow` listener; `grep -cE "'(unload|beforeunload)'"` returns 0. `TestFeedRefetchesOnBackForwardCacheRestore` ran and passed. |
| 9 | 02-09: `GET /api/reports` sets `Cache-Control: no-store` on every exit path, set inside `NearbyReports` and not inside the shared `writeJSON` helper | ✓ VERIFIED | Read directly: `internal/api/handlers/reports.go` line 352 sets the header inside `NearbyReports`, before any query-parameter parsing — confirmed by reading the surrounding code, not merely grepping for the string. `TestFeedResponseIsNotCached` (asserts the header on both a 200 and a 400 from the same handler) ran and passed as its own named test in this pass. |
| 10 | 02-09: the disputed filter's state round-trips through the page URL (`history.replaceState`, never `pushState`, no `localStorage`), restored before the first fetch, kept in step with the address bar | ✓ VERIFIED (structurally — see item 4 in `behavior_unverified_items` for the rendered/no-flash claim) | `web/static/js/feed.js`: `DISPUTED_PARAM = 'show_disputed'` (line 46), `history.replaceState` call present, `grep -c 'pushState'`/`grep -c 'localStorage'` both return 0. `TestDisputedFilterIsCarriedInThePageURL` ran and passed. The pre-existing `TestShowDisputedUsesOneSharedQueryParam` guard (independently re-read by this verifier, not just trusted from SUMMARY narration) still enforces exactly one `show_disputed` occurrence in `app.js` and exactly two `fetch(` calls total — the "no second report query, no client-side filtering of `state.reports`" invariant was narrowed to the real invariant, not loosened; confirmed by reading the test body directly. `grep -n "state.reports\.filter\|\.filter("` on `feed.js` returns no matches. |
| 11 | 02-09: the reversal of Phase 2's original non-persistence scoping is recorded in `02-UI-SPEC.md` as a dated, locked amendment | ✓ VERIFIED | `02-UI-SPEC.md` line 254: "Amendment history (added 2026-09-17, plan 02-09 — locked, not open)" — read directly, present and dated as claimed. |
| 12 | 02-10: the Activity page has a "Back to map" link as the first element in `<main>`, in the page body (not the shared header, per DEC-S) | ✓ VERIFIED | `web/templates/profile.html.tmpl:17`: `<a href="/" class="profile-back-link">Back to map</a>`. `TestActivityPageLinksBackToTheMap` ran and passed. `git diff web/templates/account_header.html.tmpl` for this concern is empty (the link was not added to the shared header). |
| 13 | 02-10: an in-app System/Light/Dark theme toggle exists in the account menu, applies before first paint via a non-deferred head script on every full page, validates the stored value against a fixed list before reflecting it, and `color-scheme` is declared on all four theme sources | ✓ VERIFIED (structurally — see item 5 in `behavior_unverified_items` for the rendered no-flash/native-control claim) | `web/static/js/theme.js` exists; `web/templates/account_header.html.tmpl:11-12` renders the `#theme-toggle`/`#theme-toggle-label` control. All 7 of 02-10's named tests (`TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage`, `TestThemeModuleHasNoAppShellDependency`, `TestThemeModuleUsesNoMarkupParsingSink`, `TestThemeModuleValidatesStoredModeBeforeReflectingIt`, `TestThemeModuleGuardsStorageAccess`, `TestAccountHeaderRendersThemeControl`, `TestThemeOverrideBlocksExistForBothModes`) ran individually and passed. |

**Score:** 8/13 truths verified (5 ROADMAP SCs + 3 gap-closure truths with no rendered-behavior
component). **5 present-but-behavior-unverified** (items 7, 8, 10, 13 partially, and 6's rendered
half via item 5's truth) — every one is a browser-composited or browser-caching claim the gap-closure
plans' own authors flag as needing the human-check that has not yet run. No truth FAILED.

### Required Artifacts (gap-closure round)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/static/css/trust.css` | Guarded vote-button rule + Leaflet popup surface override | ✓ VERIFIED | Both present, read directly; no raw hex/`rgb()`/`rgba()`/`hsl()`/`!important` introduced (confirmed by grep). |
| `web/votes_contract_test.go` | Widened `TestVoteBlockHiddenGuard` targets, 2 new tests, narrowed (not loosened) `TestShowDisputedUsesOneSharedQueryParam` | ✓ VERIFIED | All present and read; ran and passed individually. |
| `web/static/js/feed.js` | `pageshow` listener, `DISPUTED_PARAM`, URL restore/sync | ✓ VERIFIED | Present, read directly; no `unload`/`beforeunload`/`localStorage`/`pushState`. |
| `internal/api/handlers/reports.go` | `Cache-Control: no-store` in `NearbyReports`, not `writeJSON` | ✓ VERIFIED | Confirmed by reading the handler body, not just grepping the string. |
| `web/feed_freshness_contract_test.go` | New file, 2 tests | ✓ VERIFIED | Exists, both tests ran and passed. |
| `internal/api/handlers/feed_visibility_e2e_test.go` | `TestFeedResponseIsNotCached` | ✓ VERIFIED | Ran and passed against a real e2e harness. |
| `.planning/.../02-UI-SPEC.md` | Dated amendment on D-10/D-11 section | ✓ VERIFIED | Present, read in full. |
| `web/templates/profile.html.tmpl` | "Back to map" link | ✓ VERIFIED | Present as first element in `<main>`. |
| `web/static/js/theme.js` | Theme module, IIFE house style, no app-shell dependency | ✓ VERIFIED | Exists; contract tests confirm no `Pinalert` reference, no markup-parsing sink, guarded storage access. |
| `web/templates/account_header.html.tmpl`, `index.html.tmpl`, `login_gate.html.tmpl`, `verify_outcome.html.tmpl` | Theme control / script tag wiring | ✓ VERIFIED | `TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage` (filesystem-derived, not hardcoded) ran and passed. |
| `web/static/css/main.css` | `color-scheme` on all 4 theme sources, no token drift | ✓ VERIFIED | `TestThemeOverrideBlocksExistForBothModes` and the pre-existing 18-pairing WCAG matrix (`TestBadgeGlyphContrastAcrossAgeStagesAndThemes`) both ran and passed, confirming no token was disturbed. |
| `web/profile_nav_contract_test.go`, `web/theme_contract_test.go` | New test files | ✓ VERIFIED | Both exist; all tests ran and passed individually. |

### Key Link Verification (gap-closure round)

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `votes.js`'s `openResolveConfirm` | `trust.css`'s guarded `.vote-btn` rule | `hidden` attribute + CSS guard | ✓ WIRED | Confirmed: `votes.js` unedited by 02-08 (per its prohibition list, verified by `git diff`-equivalent read), CSS now respects the attribute. |
| `feed.js`'s `pageshow` handler | `app.js`'s shared fetch | direct call, `persisted`-guarded | ✓ WIRED | Confirmed by reading `feed.js`; `TestFeedRefetchesOnBackForwardCacheRestore` asserts `persisted` check precedes the fetch call textually. |
| `feed.js`'s URL reader/writer | `internal/api/handlers/reports.go`'s query parser | shared `show_disputed` param name | ✓ WIRED | Confirmed identical literal on both sides by reading both files directly (not merely trusting the SUMMARY's claim). |
| `account_header.html.tmpl`'s theme control | `theme.js`'s click handler | shared element ids | ✓ WIRED | `TestAccountHeaderRendersThemeControl` reads both embedded files and asserts the module addresses both ids — ran and passed. |
| `theme.js` | `main.css`'s `data-theme` override blocks | `data-theme` attribute | ✓ WIRED | `TestThemeOverrideBlocksExistForBothModes` confirms both override blocks still exist with `color-scheme` added; `theme.js` is the sole writer of the attribute (exactly one `setAttribute`/`removeAttribute` pair, confirmed by grep count of 1/1). |

### Advisory Item Carried Forward from Code Review (not a phase must-have — informational only)

`02-REVIEW.md` (2026-09-18, 0 Blocker findings in this workflow's own review taxonomy / 1 Critical +
3 Warning + 2 Info in the reviewer's own severity scale) flags **CR-01**: the four vote-casting
routes and report submission carry no rate limiting, and the independence predicate accepts
client-supplied lat/lon with no proof-of-location — meaning the trust mechanic's core
gaming-resistance property (the project's own stated Core Value) can be defeated by a small number
of scripted, freshly-verified accounts. This is correctly scoped by the review itself as advisory
rather than a phase must-have: no plan's `must_haves.truths`/`must_haves.artifacts` names rate
limiting or proof-of-location, and the review's own text notes the fuller weighted/diversity fix is
explicitly Phase 3 scope (`TRUST-05`, `TRUST-07`). Per this workflow's own gate taxonomy, review
findings are advisory and do not block phase verification; per Step 9b, this would also be a
legitimate deferral to Phase 3/4 if it were treated as a gap (Phase 4's `ROBUST-04` already covers
session-based rate limiting). **Flagged here for visibility, carried forward from the prior
verification report, not counted as a gap.** The orchestrator should still surface CR-01 to the
developer directly, per this workflow's escalation instructions for code-review Critical findings.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|--------------|--------|----------|
| TRUST-01 | 02-02, 02-03a, 02-03b, 02-05 | User can confirm or dispute another user's report | ✓ SATISFIED | Unchanged from prior pass; regression-confirmed. |
| TRUST-02 | 02-01, 02-04, 02-06, 02-08, 02-09 | One shared resolver, identical everywhere shown | ✓ SATISFIED | Unchanged resolver; gap-closure plans (correctly) touched only client-rendering/caching of the resolver's output, not the resolver itself. |
| TRUST-03 | 02-03a, 02-03b, 02-05 | Distinct account AND distinct geohash cell for independence | ✓ SATISFIED | Unchanged; regression-confirmed. |
| TRUST-04 | 02-01, 02-04, 02-06 | Provisional gate; critical bypass with no gate | ✓ SATISFIED | Unchanged; regression-confirmed. |
| TRUST-06 | 02-01, 02-04 | Severity is triage-sort-only, never a gate | ✓ SATISFIED | Unchanged; regression-confirmed. |
| TRUST-08 | 02-03a, 02-03b, 02-04, 02-07, 02-08, 02-09 | Reporter or nearby confirmer can mark resolved | ✓ SATISFIED (server-side); client-side click-through UX **partially human_needed** | Server logic unchanged and re-verified. The button-replacement and post-Reopen feed-freshness gap-closure fixes are structurally verified but their rendered/live behavior is not yet human-confirmed (see `behavior_unverified_items`). |
| TRUST-09 | 02-02, 02-03a | Concurrent votes never silently lost, automated concurrency test | ✓ SATISFIED | Concurrency tests re-ran against a real Postgres in this pass and passed. `.planning/REQUIREMENTS.md`'s Traceability table now correctly shows TRUST-09 as "Complete" — the prior pass's stale-row note is resolved. |
| IDENT-03 | 02-10 | Verified user can view their own profile page, listing submitted reports | ✓ SATISFIED | Already "Complete" in REQUIREMENTS.md prior to this gap-closure round (per 02-10-SUMMARY, this ID's presence in 02-10's `requirements:` field covers the Activity-page back-link addition only); the back-link itself is verified above. |
| UX-01 | 02-10 | A user can choose Light, Dark, or follow-the-OS appearance from inside the app | ✓ SATISFIED in code, ⚠️ stale traceability row | The requirement's own checkbox in `.planning/REQUIREMENTS.md` is `[x]` and the requirement text records it was added 2026-09-18 from this phase's UAT. **However, the Traceability table's own row (line 266) still reads "UX-01 | Phase 2 | Pending (gap-closure plan 02-10)"** despite 02-10-SUMMARY's claim that `gsd-tools query requirements.mark-complete IDENT-03 UX-01` was run. This is a documentation-bookkeeping gap (the same class of issue the prior verification flagged for TRUST-09, which is now itself resolved) — not a code gap. Recommend running the mark-complete step (or a manual edit) so the Traceability table's "Pending" row does not contradict the requirement's own "Complete"-implying checkbox and 02-10-SUMMARY's claim. |

No orphaned requirements: all IDs REQUIREMENTS.md maps to Phase 2 (TRUST-01/02/03/04/06/08/09,
IDENT-03, UX-01) appear in at least one plan's `requirements:` frontmatter field, including the two
requirements UX-01 and IDENT-03 that 02-10 (a gap-closure plan) newly claims.

### Anti-Patterns Found

None. Scanned every file touched by 02-08/02-09/02-10 for `TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` —
zero matches. `go vet ./...` is clean. `git status --porcelain` on the working tree is clean and all
gap-closure commits (`2705743`, `bdfa5a3`, `63b1790`, `5909c39`, `6173ada`, `3a3fedc`, `3e5257c`,
`1fe424b`, `b3dee5b`, `bae7fcc`, `738c850`) are present in `git log`.

### Behavioral Spot-Checks (run by this verifier)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full workspace suite (single run, real Postgres) | `go test -p 1 ./...` | All packages PASS, 0 failures | ✓ PASS |
| Build / static check | `go build ./...`, `go vet ./...` | Clean | ✓ PASS |
| All 13 gap-closure tests, run individually by name | `go test ./web/... ./internal/api/handlers/... -run '<13 names>' -v` | 13/13 `--- PASS` | ✓ PASS |
| `TestShowDisputedUsesOneSharedQueryParam` (narrowed guard, independently re-read, not just trusted from SUMMARY) | read `web/votes_contract_test.go:919-964` directly | Still enforces exactly 1 `show_disputed` occurrence in `app.js` and exactly 2 `fetch(` calls total | ✓ PASS |
| `feed.js` performs no client-side filtering of `state.reports` | `grep -n "state.reports\.filter\|\.filter(" web/static/js/feed.js` | No matches | ✓ PASS |
| `Cache-Control: no-store` placement | read `internal/api/handlers/reports.go:320-352` directly | Confirmed inside `NearbyReports`, before query parsing, not inside `writeJSON` | ✓ PASS |

### Human Verification Required

Eight items — three carried forward unresolved from `02-UAT.md` even where marked "pass" or already
addressed, plus the five newly-introduced gap-closure fixes that are themselves rendered/live-browser
claims (see frontmatter `behavior_unverified_items` for full detail on each):

1. **D-18 GPS-denial** — the DevTools no-request check, sessionStorage caching behavior, and map-popup deny-path parity were never independently re-confirmed live (02-UAT.md Test 1's own caveat, unresolved by any gap-closure plan).
2. **Provisional dimming/"Unconfirmed" chip and the empty-disputed-state copy** — never visually confirmed with a screenshot (02-UAT.md Test 2's own caveat, unresolved by any gap-closure plan).
3. **02-08's button-replacement fix**, live, both themes, both surfaces (feed row + map popup).
4. **02-08's map-popup heading-legibility fix**, live, both themes — including checking the fix's own falsifiable prediction (no light-mode reproduction).
5. **02-09's Safari Back-after-Reopen refetch fix**, live in Safari specifically, then one Chromium browser — the pre-fix symptom itself was never confirmed reproducible live, only diagnosed from source.
6. **02-09's disputed-filter reload persistence**, live — including whether the first paint genuinely shows no unfiltered flash.
7. **02-10's theme toggle**, live — no-flash-on-reload, native form-control/scrollbar theming, post-logout persistence, and (novel) the **first-ever light-mode walkthrough of all of Phase 2's UI**, which 02-10-SUMMARY states explicitly has not yet happened.
8. **02-10's Activity "Back to map" link**, live click-through (structurally verified; a live tap-through was not separately re-run by this verifier).

Recommend running these via `/gsd-verify-work 2` (a second UAT round against `02-UAT.md`'s
re-verification instructions and each gap-closure plan's own `<human-check>` block) before treating
Phase 2 as fully closed. None of these are code gaps — every fix has a corresponding automated
regression gate that passed — but every one is a rendered-browser claim this workflow's own rules
require a human, not a grep, to confirm.

### Gaps Summary

No code gaps found in this re-verification pass. All 6 gaps `02-UAT.md` recorded as `status: failed`
now have a corresponding, independently-verified codebase fix (grepped and read directly by this
verifier, not merely accepted from SUMMARY.md narration), and all 13 new/extended regression tests
those fixes shipped with pass individually against a real Postgres instance this verifier created
and dropped for the purpose. The full workspace test suite is green with zero failures. No
`TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER` debt marker exists in any file this gap-closure round touched.

The report's status is `human_needed` rather than `passed` for two independent reasons: (1) every
one of the 6 fixes this round shipped is a rendered-visibility or browser-caching invariant that
static/unit tests can prove is wired correctly but cannot prove actually composites/behaves correctly
in a real browser — exactly the class of claim this same phase's test suite was green throughout
while the original bug shipped, so presence-and-wiring evidence alone is treated as
`PRESENT_BEHAVIOR_UNVERIFIED`, not `VERIFIED`, per this workflow's behavior-dependent-truth rule; and
(2) three items from the original `02-UAT.md` walkthrough remain genuinely untested (not merely
unverified by this pass) even where the UAT's own summary marked the surrounding test "pass" — the
GPS-denial DevTools/sessionStorage checks, and the Provisional-chip/empty-state visual confirmation.

Two non-blocking notes carried forward or newly found: the `mode_guard` escalation (ROADMAP's
`Mode: mvp` tag vs. a non-user-story Goal field) is unresolved and carried forward unchanged from the
prior verification pass; and `UX-01`'s Traceability-table row still reads "Pending" despite the
requirement's own checkbox and 02-10-SUMMARY's claim that it was marked complete — a one-row
documentation staleness, the mirror of the now-resolved TRUST-09 staleness the prior pass flagged.
The advisory code-review Critical finding (CR-01, vote-endpoint rate limiting / proof-of-location) is
carried forward for visibility per the prior report's framing; it is correctly out of scope for this
phase's must-haves and is Phase 3/4 territory by the project's own roadmap.

---

_Verified: 2026-09-18_
_Verifier: Claude (gsd-verifier)_
