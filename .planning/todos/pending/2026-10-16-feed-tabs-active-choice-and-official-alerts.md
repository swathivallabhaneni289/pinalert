---
created: 2026-10-16T09:05:00.000Z
title: Feed tabs on the phone, a Confirmed or Disputed choice inside Active, room for official alerts
area: ui
files:
  - web/templates/index.html.tmpl:89-93 (view tabs Active, Disputed, Past)
  - web/static/js/feed.js (view switching, tab state, map follows the tab)
  - web/static/css/phone.css (phone tab strip)
---

## Problem

Phase 3.1 built one tab strip, Active, Disputed and Past, on laptop and phone (owner answer of
2026-10-14). On 2026-10-16 the owner said by voice that the phone top area will get crowded,
because Phase 5 adds official government alerts too. Owner idea: keep Active as a tab and, inside
it, let the reader choose confirmed ("approved") or disputed ("disapproved") reports, instead of
Disputed being its own tab. The owner also wants to see the finished version of Phase 3.1 and
Phase 4 first and may move phone things around after that.

## Solution

Decide in the Phase 5 discussion, together with where official alerts sit. One shape to show the
owner: tabs Active, Past and Official alerts, with a choice inside Active of All, Confirmed or
Disputed. Keep All as the default, so a new report that nobody has confirmed yet (a fresh rescue
request) is never hidden. Until then the Phase 3.1 tabs stay as built. Touches Phase 3.1 decisions
D-07 to D-13 (tabs, and the map following the tab) and the 3.1 UAT item 3 (D-13).
