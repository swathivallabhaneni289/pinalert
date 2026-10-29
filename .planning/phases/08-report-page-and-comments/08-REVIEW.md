---
phase: 08
reviewed_commit: 67370b92
parts: [server, browser, templates]
status: issues_found
findings:
  critical: 0
  warning: 15
  info: 19
  total: 34
---

# Phase 8 code review: index and triage

Three read only reviewers (Opus) read the Phase 8 changes on main (the 08 commits after 88cbe8ad) in parallel: the server, the browser scripts, and the templates, stylesheets, contract tests and docs.

| Part | Critical | Warning | Info | File |
|------|----------|---------|------|------|
| Server | 0 | 3 | 8 | 08-REVIEW-server.md |
| Browser | 0 | 5 | 7 | 08-REVIEW-browser.md |
| Templates | 0 | 7 | 4 | 08-REVIEW-templates.md |

## Triage

Fix round 1 runs three fixers in worktrees off the commit that adds these files, one per part.

**Server fixer** (internal, cmd, router.go, handlers):
- Fix WR-S01, WR-S02 and WR-S03.
- Server half of WR-T02: /favicon.ico answers without minting a session.
- IN-T04: the public page's Retry-After matches its words.
- Fix where cheap and safe: the IN-S items for tab paste, the 500 path headers and the queued push after removal. Record the rest with a reason.

**Browser fixer** (web/static/js, web/*.mjs):
- Fix WR-B01 to WR-B05.
- Fix the cheap IN-B items. IN-B01 stays as built: UI-SPEC allows the 409 closed answer to clear the box.

**Templates fixer** (web/templates, web/static/css, web contract tests, docs, README):
- Fix WR-T01, WR-T02 (the icon link on the public page), WR-T03, WR-T04, WR-T05, WR-T06 and WR-T07.
- Fix IN-T02 and IN-T03.

**Accepted:** IN-T01. The Policy date and CurrentTermsVersion stay 2026-10-29. On deploy day the owner moves both together (README step 9), which now also covers the Phase 8 Policy lines.

After the fixes: merge, the gate, a read only skeptic, then the verifier. The phase stays In Progress until the owner says yes.
