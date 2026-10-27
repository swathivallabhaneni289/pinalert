---
phase: quick-261027-jhd
plan: 01
subsystem: web/welcome (first-time flow)
tags: [welcome, step-2, step-5, intersection-observer, accessibility, copy]
requires: []
provides:
  - "Step 2 Agree and Disagree appear only after the end of the Disclaimer and policy text has been in view"
  - "Step 5 needs-install variant: title Alerts, one line, Continue and Back, no install guide"
affects:
  - web/templates/welcome.html.tmpl
  - web/static/js/welcome.js
  - web/static/css/welcome.css
tech-stack:
  added: []
  patterns:
    - "IntersectionObserver rooted in the scroll box, with a 1px marker as the last child and a 24px bottom rootMargin"
    - "Hold applied by the script only (data-unread), visibility hidden so layout never moves"
key-files:
  created: []
  modified:
    - web/templates/welcome.html.tmpl
    - web/static/js/welcome.js
    - web/static/css/welcome.css
    - web/templates/device_guide.html.tmpl
    - web/static/css/device-guide.css
    - web/welcome_flow_test.mjs
    - web/welcome_alerts_test.mjs
    - web/welcome_contract_test.go
    - internal/api/handlers/welcome_e2e_test.go
    - .planning/phases/03.3-installable-app-and-phone-alerts/03.3-UI-SPEC.md
decisions:
  - "Step 5 needs-install title is Alerts (the step's own name, short, asks nothing); the other variants keep Get alerts when it matters"
  - "The hold is applied by welcome.js only, and only when the reveal can run (not agreed, IntersectionObserver present, row and marker present); the template never carries data-unread"
  - "The held back row uses visibility hidden, not hidden or display none, so the flex scroll box does not grow and shrink on reveal"
  - "rootMargin 0px 0px 24px 0px (about the box's 20px bottom padding) so VoiceOver scrolling the last line just into view reaches the 1px marker"
  - "An account that already agreed is never held back, since on the phone Agree is its only way forward from a Back to step 2"
  - "The welcome page no longer links device-guide.css; Settings still does"
metrics:
  tasks: 3
  commits: 3
  files: 10
completed: 2026-10-27
status: complete
---

# Phase quick-261027-jhd Plan 01: Agree after reading and a gentler iPhone step 5 Summary

Step 2 of the first-time flow now works like the terms screen of any other app (Agree and Disagree appear once the end of the text has been in view, in the same place, with no motion), and step 5 on an iPhone Safari tab is gentler (title Alerts, one line, Continue and Back, no install guide).

## What was built

**Step 2, Agree after reading**
- `welcome.html.tmpl`: a 1px `#welcome-scrollbox-end` marker (`aria-hidden`) is the last child of `#welcome-scrollbox`, right under the policy's "Last updated" line. The Agree and Disagree row got the id `welcome-consent-actions`. The template carries no hold attribute, so a page whose script fails still shows Agree.
- `welcome.js`: `watchForEnd()` runs once at init, after `agreed` is set and before the first `goTo`. It sets `data-unread` on the row and starts an `IntersectionObserver` with `root: scrollbox`, `rootMargin: '0px 0px 24px 0px'`, `threshold: 0`, watching the marker. `revealConsent()` removes the attribute and disconnects the watcher. `isShown()` now also treats an element carrying `data-unread` as not shown, so the focus trap skips the held back row and Tab never walks out of the dialog. Read again, Disagree, Back, a failed Agree and returning to step 2 never hold the row back again.
- Safety cases show the buttons at once: no `IntersectionObserver`, a constructor that throws, no marker or row on the page, an account that already agreed, and a text that fits (the browser's first report already has the marker in view).
- `welcome.css`: `.welcome__actions--pair[data-unread]` has the single declaration `visibility: hidden` (keeps its place, leaves Tab and the accessibility tree, no motion); `.welcome__scrollbox-end` is `height: 1px`. No transition or animation was added.

**Step 5, gentler needs-install**
- The install guide include is replaced by `<p class="welcome__lead" id="welcome-alerts-install" hidden>You can turn on alerts later in Settings.</p>`. `welcome.js` sets the title to `Alerts` for that variant (`TITLE_LATER`); ready, blocked and unsupported keep their title. The guide, its partial and its title "Install to get alerts" stay in the Settings window unchanged. Step 5 is still the only place `first-run-done` is sent.
- The welcome page no longer links `device-guide.css`. Comment-only edits in `device_guide.html.tmpl` and `device-guide.css` record that the guide now lives in one place.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 RED | 3ffbfcb | test(quick-261027-jhd): failing tests for Agree after reading and a gentler iPhone step 5 |
| 2 GREEN | d699ab5 | feat(quick-261027-jhd): Agree and Disagree appear after reading, and a gentler iPhone step 5 |
| 3 docs | cd30520 | docs(quick-261027-jhd): owner amendment 2026-10-27 for the step 5 needs-install copy |

## Verification

- RED confirmed before GREEN: `TestWelcomeAgreeAfterReading`, `TestWelcomeAlertsStep`, `TestWelcomeAlertsScript` and `TestWelcomeStep2ReadsTheSharedText` failed, and four Node cases failed (two step 2 reveal cases, two needs-install title cases). All older cases passed against the old script.
- `go build ./...` and `go vet ./...`: clean.
- `node --test web/*.mjs`: 786 tests, 786 pass, 0 fail.
- `go test ./web/ -short -count=1`: ok (520 passing tests reported with -v); `go test ./web/ -count=1`: ok.
- `go test ./internal/api/handlers/ -p 1 -count=1` on the disposable database `p_quick_jhd_a`: ok, 663 passing tests, 0 failures. The database was dropped afterwards.
- Settings files (`settings.html.tmpl`, `settings-alerts.js`) are untouched; the settings contract pins and `TestDeviceGuideCopy` pass unchanged.

## Deviations from Plan

None. The plan was executed as written. One small implementation detail: `isShown` guards with `typeof n.hasAttribute === 'function'` before reading the attribute, so a node without that method (none exist in the real DOM) cannot throw.

## Notes for the owner and the orchestrator

- The step 2 lead still says "Tap Agree to continue." (unchanged, as asked).
- `03.4-UI-SPEC.md` was not amended for step 2 (not asked). The todo file and this SUMMARY carry the step 2 change; the 03.3 UI-SPEC amendment mentions it in one line.
- The untracked folder `pinalert-browser-checks/` was not edited. Its `checks/test18.js` clicks `#welcome-agree` directly; it now has to scroll the text box (`#welcome-scrollbox`) to the end first, or the buttons are still hidden.
- The todo file was not moved (the orchestrator does that). No server was started and no simulator or Mac Safari check was run; the human check in the plan (iPhone simulator step 2 and step 5, Settings guide window, Mac Safari Tab order) is left to the orchestrator after the merge.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path or file access. The only new script behaviour sets and removes one attribute and reads observer entries (T-261027-03). The self lockout risk (T-261027-02) is covered by the Node tests for each safety case and the contract pin that the template holds no `data-unread`.

## Self-Check: PASSED

- Commits 3ffbfcb, d699ab5 and cd30520 exist on the worktree branch.
- All files listed under key-files exist and are committed.
