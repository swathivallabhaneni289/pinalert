---
created: 2026-10-16T09:20:00.000Z
title: Official alert headline strip at the top of the map, like a TV news headline
area: ui
files:
  - .planning/ROADMAP.md (Phase 5 success criterion 3 and scope note 4 place the banner in the list panel)
  - web/templates/index.html.tmpl (map pane, map tools)
  - web/static/css/phone.css (phone map strip, 176px)
---

## Problem

Owner (voice, 2026-10-16): when the government announces an emergency, the app should show it the
way television shows headlines, as one strip "where the news is going on", on top of the map. Phase 5
success criterion 3 already asks for a banner when a Severe or Extreme official alert covers the
reader's area, but scope note 4 puts the banner in the list panel beside Nearby, not over the map.

## Solution

Settle in the Phase 5 discussion, with the owner looking at a test copy on a phone. Points to bring:
- Placement: a strip across the top of the map (owner's wish) rather than the list panel; on a phone
  it sits between the unofficial bar and the 176px map strip, so measure what is left for the list.
- Still or moving: a TV style crawl is moving text, which is hard to read on a phone, needs a pause
  control for accessibility, and breaks the house rule against motion. Suggest a still headline that
  shows the newest Severe or Extreme alert, "and N more", and opens the full alert on tap.
- It shows only for alerts covering the reader's area, carries the authority badge, issuer and
  expiry (criterion 2), and never looks current when the feed is stale (criterion 3).
- It must not look like the unofficial bar; the two say opposite things (official versus unofficial).
Related: the pending todo on feed tabs and official alerts (2026-10-16).
