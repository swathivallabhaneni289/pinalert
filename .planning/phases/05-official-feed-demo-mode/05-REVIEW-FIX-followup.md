---
phase: 05
part: followup
base: e2a20bdd
follow_up_base: f6ca25ba
fixed: 6
not_reproduced: 0
recorded: 0
---

# Phase 5 review fix, follow-up part

Six small items left between the part fixers' file boundaries. Fixed 2026-10-29.

| Item | Outcome | Commit | Test that proves it |
|------|---------|--------|---------------------|
| WR-T07 Exit demo accessible name | fixed | 3bc67461 | web TestDemoStripCopyAndExit (failed before the template change; now bans `aria-label="Exit the demo"` and requires `>Exit demo</a>`); handlers TestDemoPageRendersWhatItPromises (amended, same ban) |
| Swagger regeneration, `ended` field | fixed | 51f370ac | handlers TestSwaggerOfficialAlerts pins `ended` on OfficialRowResponse (failed before `swag init -g internal/api/router.go -o docs`, the Makefile swag target) |
| Docs examples after the display table (WR-S07) | fixed | 8a64ce79 | docs only; no test. TRUST-MODEL.md example now "NTR"; OFFICIAL-ALERTS.md line 84 no longer claimed names are shown as the file spells them |
| Report form history comment (IN-S01) | fixed | a7a1e801 | comment only; web report form contract tests still pass |
| Demo `#official-notice` role="status" (WR-B06) | fixed | 491da717 | web TestOfficialAlertsShell now also checks demo_alerts.html.tmpl (failed before the template change) |
| UI-SPEC new on screen words | fixed | d74bcf56 | documentation only |

Notes:

- Item 1 also amended UI-SPEC lines 315, 479 (P5-C15) and 511 with dated notes (in commit 3bc67461).
- Item 3: only two docs named a raw district name. The `Ntr` strings in web/official_alerts_test.mjs are fake fixture data, not an example shown to readers, and were left alone.
- Item 6 words: "Hazard" (GDACS map card heading, map.js), "Recently ended" and "Hazard events from GDACS" (group titles, official-alerts.js), "Ended" (row tag).
- Swagger diff was only the new field (20 added lines across the four files).

Full suite on p5_fix_f: `go test ./... -p 1 -count=1 -v` 1865 PASS, 0 `--- FAIL`, 0 `^FAIL`, 3 skips (TestGeocodeLive, TestOfficialLive, TestScreeningFixture). `node --test web/*.mjs` 1145 pass, 0 fail. No flake reruns were needed.
