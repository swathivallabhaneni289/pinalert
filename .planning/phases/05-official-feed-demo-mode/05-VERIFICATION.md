---
phase: 05-official-feed-demo-mode
verified: 2026-10-29T08:10:00Z
verified_commit: 1a72f388
status: human_needed
score: 8/8 must-haves verified in code and tests
behavior_unverified: 0
overrides_applied: 0
gaps: []
human_verification:
  - test: "Official alerts page on a real phone and on Mac Safari"
    expected: "In force now with a count and districts, Last checked line, authority badge, issuer, Issued and Until on every row, state wide alerts under Across <state>, an honest unavailable notice when the feed is down"
    why_human: "Look and feel on real devices; Playwright WebKit has passed pages that real iPhone Safari showed broken"
  - test: "Nearby banner on a real phone and on Mac Safari"
    expected: "One slim still line (icon, 'Cyclone warning, Krishna district', chevron), only for Severe or Extreme naming the reader's district or Extreme for the whole state, whole row opens Official alerts, hidden when the feed is stale"
    why_human: "Visual fit, 44px tap height, and the 320 to 359px two line case"
  - test: "GDACS pins on Nearby on a real phone and on Mac Safari"
    expected: "Flat rectangle pin with the word GDACS, told apart from a crowd dot and a Demo dot with a dashed ring; tapping opens a read only card with Open GDACS page; no vote controls"
    why_human: "Pin kinds told apart on a real iPhone (05-22 left this open); GDACS often draws nothing near India, so a seeded or live event is needed"
  - test: "One silent official notification on a real installed phone after hosting"
    expected: "A Severe or Extreme alert at the home district reaches the lock screen with no sound and no vibration, and tapping it opens Official alerts"
    why_human: "Needs the https host, the installed app, VAPID keys, the cron and a real alert (PUSH.md owner check 6). iPhone's own settings decide the sound"
  - test: "Demo from the sign in page and from step 1 Welcome of the first time flow"
    expected: "Try the demo opens the Vijayawada flood practice copy with no login, the Demo strip on every screen, a practice report that pretend neighbours confirm until Confirmed, Exit returning to the page it came from; nothing saved, reload starts fresh"
    why_human: "End to end feel and Exit return path on a real device"
  - test: "New sign in globe on a real iPhone (including the SE size) and on Mac Safari"
    expected: "Whole Earth above the card on a phone, not cut off, card on the first screen with Send reachable, glass boxes readable, still under reduce motion, nothing downloaded in Low data view"
    why_human: "No iPhone SE exists on the build Mac (iPhone 17 simulator only); the SE 548px visible height and Mac Safari were never checked in a real browser"
  - test: "Settings switch 'Severe official alerts for my area'"
    expected: "On by default when a home area exists, saves, and turning it off stops official notifications"
    why_human: "Visual and flow check on a real device"
  - test: "Approve or rewrite the three Policy sentences (sign in page hosts, NASA acknowledgement, source credits)"
    expected: "Owner picks the final words. Any rewrite moves the Last updated date (legal.go policyLastUpdated) and service.CurrentTermsVersion together to the real date of that day"
    why_human: "Owner's words. Note: the draft already moved both to 29 October 2026, so every account is sent through the agree step again after deploy"
  - test: "NDMA terms question"
    expected: "Read https://sachet.ndma.gov.in/WebsitePolicy and email NDMA's SACHET contact asking in writing whether a free, unofficial, attributed relay is welcome and what request rate is acceptable. Required before wide promotion, not before the first deploy"
    why_human: "Cannot be done by code. Nothing in the repo says NDMA has agreed. Also confirm the district boundary credit wording in the legal or mentor review"
  - test: "make migrate for 00012 and 00013 on the owner's dev database, after the owner says yes"
    expected: "Migrations 00012_add_official_alerts and 00013_add_official_push apply, goose version 13"
    why_human: "Never run on the owner's database by an agent. (They did apply cleanly on a disposable database in this verification.)"
  - test: "GitHub secret OFFICIAL_REFRESH_TOKEN, variable PINALERT_BASE_URL, and the same token on the host"
    expected: "Token at least 32 characters, same value in the GitHub secret and the host variable OFFICIAL_REFRESH_TOKEN; PINALERT_BASE_URL is the https address with no trailing path; the workflow stays skipped while the variable is unset. Choose a 10 minute or 30 minute cadence"
    why_human: "Owner credentials and hosting; the workflow does nothing without them"
  - test: "Banner blink while panning"
    expected: "The banner hides on every move to a new place and returns when that place's answer arrives. A fixer noted it may blink while panning. Owner decides if it is acceptable"
    why_human: "Feel of the interaction; a design judgment"
---

# Phase 5: Official Feed and Demo Mode Verification Report

**Phase Goal:** A first-time visitor lands on a populated, working map immediately, with real official alerts and hazard pins plus a working demo, instead of an empty product.
**Verified:** 2026-10-29 at 1a72f388
**Status:** human_needed
**Re-verification:** No, initial verification

I started from the view that the goal was missed and tried to falsify the summaries against the code. I found no blocker. Every code level check passes. The phase cannot pass without the owner's own checks (real devices, hosting, owner words and credentials), so the status is human_needed.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Map shows GDACS official pins beside crowd reports; Alerts view lists SACHET alerts covering the reader's district; official items are unaffected by votes (SC1, ROBUST-05, ALERT-01) | VERIFIED | Own tables (migration 00012: official_alerts, official_alert_areas, official_feed_status, official_hazards, official_push_outbox). Routes `GET /api/official/alerts` and `GET /api/official/pins` and page `/alerts` in internal/api/router.go lines 768 to 780, behind the gate. `official-pins.js` is fed only by `/api/official/pins`. Tests pass: TestOfficialPinsHiddenWhenStale, TestOfficialPinsRoute family, web official_pins_test.mjs |
| 2 | Official items carry a distinct authority badge, issuer, issued and expires; expired alerts disappear (SC2, ROBUST-06, ALERT-02) | VERIFIED | Pin is a flat rectangle with the word GDACS, never round (docs/OFFICIAL-ALERTS.md; contract tests TestOfficialPinDistinctFromCrowdAndDemo, TestOfficialPinCard). Expiry handled in the store query and view; internal/official tests pass (TestBuildView family, TestRefreshUpdateAndCancel) |
| 3 | Banner only for Severe or Extreme covering the reader's area; "last checked" shown; unavailable feed said plainly; stale never looks current (SC3, ALERT-03, ALERT-04) | VERIFIED | TestBuildViewBannerRule and TestBuildViewStaleAndUnavailable pass. D-16 rule (district named, or Extreme for the whole state) implemented in view.go. Pins route answers nothing unless GDACS is fresh. Browser scripts clear on stale (official_banner, official_pins, official_alerts node tests 213 of 213 pass) |
| 4 | First time visitor sees a seeded past event and can try confirm and dispute (SC4, ROBUST-07) | VERIFIED | internal/demo has seed.go (Vijayawada reports), memquerier.go implementing service.Querier and VotingQuerier (so the real resolver runs), neighbours.go. TestNeighboursClimbToConfirmed, TestPracticeVotesUseVisitorAccount, TestNearSeedReportCanBeConfirmedByTheVisitor pass. `/demo` mounted in router.go line 571 and exempt from the gate; per IP visit limit fails closed (TestDemoVisitLimitFailsClosed). Sandbox in memory, writes nothing to the database |
| 5 | Sign in page shows a real 3D Earth behind the card; form works first and never moves; still under reduce motion; no download in Low data or without WebGL (SC5) | VERIFIED in code, device look is human | login-globe.js (362 lines), login_globe.html.tmpl pins maplibre-gl 5.24.0, tile routes via handlers.GlobeTilePaths, login row "Try the demo, Disclaimer, Policy". Node login_globe tests pass. Quick 261029-47j browser runs at five sizes in two engines recorded in its SUMMARY (iPhone 17 simulator only) |
| 6 | Demo is offered on the sign in screen and beside Continue on Welcome step 1, and only Demo, Disclaimer, Policy open without an account (D-03, D-04) | VERIFIED | login_ways_links.html.tmpl `#login-try-demo`; welcome.html.tmpl `#welcome-try-demo-1` href `/demo?from=welcome`; router.go exempt list (lines 418 to 430, 485, 957 to 966) and gate_test pin, tests ok |
| 7 | Severe or Extreme official alerts reach the home area's phone silently, with a Settings switch (D-11, PWA-06) | VERIFIED in code, real phone is human | settings.html.tmpl `#settings-official-switch`; pushenqueue.go, service.NewOfficialPushWorker wired in cmd/server/main.go line 500; migration 00013 outbox. internal/official and internal/service tests pass |
| 8 | Feed stays fresh while the free host sleeps; failure is visible (scope notes 2a, 3) | VERIFIED in code, hosting is human | `POST /internal/official/refresh` bearer door (router.go 641), `.github/workflows/official-refresh.yml` every 10 minutes, skipped while PINALERT_BASE_URL is unset, ticker `officialSvc.Run` in main.go line 468. Quick 261029-6wb made GDACS requests run together at 40 s each |

**Score:** 8/8 truths verified in code and tests. Nothing is left as present but behavior unverified, because the state transitions (cancel and update, stale to unavailable, neighbours climbing to Confirmed, fail closed Demo) each have a passing named test.

### Required Artifacts and Wiring

| Artifact | Status | Details |
|----------|--------|---------|
| internal/official (cap, rss, sachet, gdacs, refresher, store, view, place, pushenqueue) | VERIFIED | Wired into main.go (NewService, Run) and router Deps.Official |
| internal/areas (district file, names table) | VERIFIED | Locate, CellDistricts used by the service |
| internal/demo (seed, memquerier, neighbours, store, world) | VERIFIED | Wired through handlers.DemoRoutes in router.go |
| Migrations 00012, 00013 | VERIFIED | Applied cleanly to disposable p5_verify via `go run ./cmd/migrate`, version 13, all five official tables present; database dropped afterwards |
| web/static/js official-alerts, official-banner, official-pins, demo, login-globe, welcome | VERIFIED | Present, loaded by their templates; node tests pass |
| web/templates alerts, demo_alerts, demo_strip, login_globe, login_ways_links, welcome, settings | VERIFIED | Present and carrying the controls named above |
| .github/workflows/official-refresh.yml | VERIFIED | Present; needs owner secret and variable |

Data flow: the pins and alerts endpoints read stored rows written by the refresher; no hollow props or hardcoded empties found. The Demo reads in memory seed data by design.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet | `go build ./...`, `go vet` on official, demo, api | clean | PASS |
| Official, demo, areas packages (with DB) | `go test ./internal/official/ ./internal/demo/ ./internal/areas/ -p 1` on p5_verify | ok x3 | PASS |
| Handlers and router, Official/Demo/Alerts/Globe/Gate | `go test ./internal/api/handlers/ ./internal/api/ -run ...` | ok x2 | PASS |
| Service and web contract tests | `go test ./internal/service/ ./web/ -run ...` | ok x2 | PASS |
| Named transition tests | TestBuildViewBannerRule, TestBuildViewStaleAndUnavailable, TestRefreshUpdateAndCancel, TestNeighboursClimbToConfirmed, TestDemoVisitLimitFailsClosed, TestOfficialPinsHiddenWhenStale | all PASS | PASS |
| Node, six Phase 5 files | `node --test web/official_alerts_test.mjs official_pins_test.mjs official_banner_test.mjs demo_page_test.mjs login_globe_test.mjs` | 213 of 213 | PASS |

The full suite was not rerun, as instructed (reported green at this commit: 1866 Go top level passes, three expected skips, node 1161 of 1161). No live SACHET or GDACS call was made. Only the disposable database p5_verify was used.

### Probe Execution

No `probe-*.sh` scripts are declared by Phase 5. Skipped.

### Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| ROBUST-05 GDACS pins | SATISFIED in code | Truth 1 |
| ROBUST-06 distinct authority badge | SATISFIED in code | Truth 2 |
| ROBUST-07 populated Demo with simulated report | SATISFIED in code | Truths 4 and 6 |
| ALERT-01 Alerts view for reader's district | SATISFIED in code | Truth 1 |
| ALERT-02 issuer, issued, expires, disappears | SATISFIED in code | Truth 2 |
| ALERT-03 banner only Severe or Extreme | SATISFIED in code | Truth 3 |
| ALERT-04 last checked and plain unavailable | SATISFIED in code | Truth 3 |

No orphaned requirements. REQUIREMENTS.md was not edited, and nothing is marked complete.

### Anti-Patterns Found

No TBD, FIXME, XXX, TODO or HACK in the Phase 5 Go and browser source I scanned. No blocker found. All 3 criticals from 05-REVIEW.md (CR-S01, CR-B01, CR-B02) were fixed and the skeptic pass (05-REVIEW-FIX-SKEPTIC.md) found 3 warnings and 1 info, all fixed in round 2 (NP-1 to NP-4).

### Known Gaps Recorded, Not Fixed

From 05-REVIEW.md (recorded, not changed by triage):
- IN-S06, IN-S09 (needs a captured live empty GDACS answer), IN-S10 (retention and a cap_identifier index come later), IN-T03 (no page loads official.css and official-pins.css together today), IN-T04 (word and icon carry the level), IN-T06 (draft Policy sentences, tell the owner before deploy), IN-T07 (platform contrast note, text still at least 4.99 to 1).

From the server fix report: WR-S06 left undone: IPv6 keying by /64, an entry cap on the per IP limiter, and protecting recently touched Demo worlds from eviction (changing ratelimit would touch the posting limiter and the CGNAT concern).

From round 2 (05-REVIEW-FIX-round2.md):
- A hung automatic ask on Official alerts still loses focus (giveUpIfHung does not keep the reader's place). Needs a request that never answers.
- An idle Demo Official alerts page ages in place: "Last checked 2 hours ago" under "In force now", and a sample row can pass its Until time. Made up data; any press starts a fresh visit. Fixed after this verification on 2026-10-29 (NP2-1, 827f11c7, merged in f1726afd): at the 30 minute fresh limit the page drops "In force now" once and shows the load error with Try again, with no request.

Other recorded limits:
- WebKit: the first Tab from the welcome step title (tabindex -1) on steps 1 and 3 goes to the page outside the flow, because Safari's default Tab skips links and those steps hold only links and buttons. Pre-existing; not fixed because it would change title rules older tests pin.
- Official alerts' 10 second hang limit lands on the minute tick, so it is 60 to 70 seconds in practice (documented).
- At 360px a Demo row pairing the Demo tag with "Thunderstorm warning" and "+2" pushes the chevron 13px past the padding, still inside the screen. Under 360px the banner area may take a second line (WR-T01 decision); docs also say a 320px area text can be clipped.
- Real Cancel behaviour is unverified against live traffic (OFFICIAL_LIVE=1 smoke test skipped here).
- The district boundary licence wording could not be read from the original government source.
- An iPhone follows its own sound settings, and an Apple device gets only the first push about an alert series.
- The Demo's Back button to an earlier Demo place starts a fresh Demo.
- Contrast of the red error line on the sign in glass: the browser probe read 15 to 1 instead of the expected 4.87 (light) and 5.46 (dark); the ratio is pinned by TestLoginGlassTextContrast, not by the browser.
- The old `python http.server` on port 8797 is not the project's. It is a stray process from earlier sessions and should be stopped by the owner or left alone; it is not part of Phase 5.
- Working tree notes (not Phase 5): STATE.md has uncommitted edits and three `.gitkeep` files and browser check folders are untracked. This verification touched none of them.

### Human Verification Required

See the `human_verification` list in the frontmatter (13 items). Highest value first: the silent phone notification after hosting, the globe on a real iPhone SE and Mac Safari, the Demo from both entry points, and the Policy sentences (the terms version already moved to 29 October 2026, so every account will be asked to agree again).

### Gaps Summary

No code gaps. The phase delivers its goal in the repository: official alert storage, SACHET and GDACS fetch, place matching, banner, Official alerts page, GDACS pins, push lane, cron door, a sandboxed Demo running the real trust mechanic, and the sign in globe. What is left is the owner's: real device looks, hosting and credentials, owner words, the NDMA question, and the dev database migration.

---

_Verified: 2026-10-29_
_Verifier: Claude (gsd-verifier)_
