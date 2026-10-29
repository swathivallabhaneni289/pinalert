---
phase: 05
part: round3
base: 7b41842c
fixed: 1
not_reproduced: 0
recorded_only: 0
---

# Phase 5 review fix, round 3

| Finding | Outcome | Commit | Test |
|---------|---------|--------|------|
| NP2-1 | fixed | 827f11c7 | web/official_alerts_test.mjs: the amended "NP-1: an idle Demo Official alerts page makes no request and never restarts on its own" (after 31 idle ticks: one request, no hand over, no "In force now", error block shown, focus on the error title) and the new "NP2-1: on an idle Demo page the claim is dropped once, and Try again with an ended visit hands over once" |

In the minute tick of web/static/js/official-alerts.js, a Demo page whose answer is too old now runs takePlace(), renderError(), keepPlace(place) with no request. renderError sets current to null, so it runs once. All NP-1 assertions are kept (no automatic request, no navigation without a press); only the old last line (the sample answer stays drawn after 121 idle minutes) was amended, with a dated note in the test file.

Checks: `node --test web/*.mjs` 1162 pass, 0 fail; `DATABASE_URL= go test ./web/` ok.
