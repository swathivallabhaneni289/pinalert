---
phase: 05
reviewed_commit: 1f44177d
reviewed_at: 2026-10-29
depth: standard
status: issues_found
parts:
  - 05-REVIEW-server.md
  - 05-REVIEW-browser.md
  - 05-REVIEW-templates.md
findings:
  critical: 3
  warning: 22
  info: 25
  total: 50
---

# Phase 5 code review (index)

Three read only reviewers (Opus) read the Phase 5 changes on main at 1f44177d in parallel, split as in Phase 3.3: the server (Go, SQL, the cron workflow), the browser scripts, and the templates and stylesheets. Each part file holds the findings with file and line, a failure scenario, the fix and the test that would prove it. This index adds the orchestrator's triage.

| Part | Critical | Warning | Info |
|------|----------|---------|------|
| Server (05-REVIEW-server.md) | 1 | 8 | 11 |
| Browser (05-REVIEW-browser.md) | 1 | 9 | 6 |
| Templates (05-REVIEW-templates.md) | 0 | 7 | 9 |

The totals in the frontmatter count after the triage below: WR-B07 is raised to critical, and two pairs are one finding each.

## Triage (pinalert-3b, 2026-10-29)

Raised to critical:
- WR-B07 becomes CR-B02. The Demo fails open to the real API when api-base.js fails to load: a signed in person would see real reports under the Demo strip and could vote and post for real from the pretend place. One failed script load is enough, and the harm reaches the trust mechanic, so it is fixed in this round.

The three criticals, all fixed in this round:
- CR-S01: a cancelled or replaced alert stays listed, and can raise the banner, when its Cancel or Update was stored first, or when it is fetched again under its own guid.
- CR-B01: Official alerts never asks again after it loads, so "No official alerts in force for your area" can stand for hours under an old Last checked line.
- CR-B02 (was WR-B07): the Demo fails open to the real API.

Duplicates:
- WR-T05 and WR-B06 are the same finding (the stale notice is a role status live region rewritten every minute). The browser fixer owns it.
- IN-T01 and WR-B03 are the same finding (a hidden "Report" heading takes focus when a GDACS card opens on a phone). The browser fixer owns it.

Decisions taken here (the owner delegates design polish):
- WR-T01: below 360px the Nearby banner may put the area on a second line; UI-SPEC 2a gets a dated amendment and TestBannerOneLineAndTarget is amended, never weakened for wider screens.
- WR-T04: the sign in error line keeps the normal text colour (colour is reserved for severity, and it has been so since Phase 1.1); the 261029-47j comment and TestLoginGlassTextContrast are corrected to the colour that is painted.
- WR-T07 and IN-T02: the Exit demo button's accessible name becomes its visible text, and its label is 16px as UI-SPEC says; P5-C15 gets a dated amendment.
- WR-S07: a display name table keyed by LGD code, applied when the areas load, for the names the review lists (Ntr becomes NTR, Spsr Nellore becomes Sri Potti Sriramulu Nellore, Y.S.R. becomes YSR Kadapa, Visakhapatanam becomes Visakhapatnam, and Delhi's bare compass names gain "Delhi"); no code compares names with feed text, so no match changes.
- WR-S08: the flaky EXPLAIN test drops the other index inside its own rolled back transaction.
- WR-B09: a stored map centre older than 30 minutes is not used as "Where you are"; the fallback centre is never used as "Where you are".
- IN-S08: Demo rows keep the realistic issuer names only if they also say made up; the fixer makes the made up line sit on every Demo row and card, or changes the issuer text to a made up name if a test or the UI-SPEC already asks for that.
- Every other warning is fixed. Info items are fixed when the fix is small and safe; IN-S06, IN-S09, IN-S10, IN-T03, IN-T04, IN-T06 and IN-T07 are recorded, not changed.

Fixer split (one gsd-code-fixer each, in its own worktree, tests first, "not reproduced" is a valid outcome):
- Server: everything in 05-REVIEW-server.md, plus Chrome.DemoPage (IN-S01).
- Browser: everything in 05-REVIEW-browser.md, plus WR-T02 (the no break space lives in official-alerts.js and official-pins.js) and WR-T05 (with alerts.html.tmpl if the live region attribute must move).
- Templates: everything else in 05-REVIEW-templates.md, plus the UI-SPEC amendments above.
