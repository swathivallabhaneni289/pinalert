---
created: 2026-10-16T17:10:00.000Z
title: The person menu looks unfinished; design it properly in Phase 3.4
area: ui
files:
  - web/templates/account_header.html.tmpl (the menu items)
  - web/static/css/auth.css and web/static/css/chrome.css (the trigger and the menu)
  - web/static/js/account-menu.js
  - .planning/phases/03.4-app-layout/03.4-CONTEXT.md (laptop person menu: My profile, Settings, Log out only; D-25 Settings page)
---

## Problem

Owner, 2026-10-16, while checking the Phase 4 test copy on a laptop: the person menu "looks like it can be
displayed better". What the screenshot showed: a grey panel under the round person button, the signed in email
in grey at the top with nothing separating it from the items, then Activity, Low data view (with "Text only, no
map" under it), Disclaimer, Policy and Log out as one flat list. The gaps between items are large and uneven, the
Low data view title and its second line sit tight together so that row looks different from the rest, only Log out
has an icon, and the info pages, the view switch and the account action are not grouped.

## Solution

Phase 3.4 already changes this menu (03.4-CONTEXT: the laptop person menu holds only My profile, Settings and
Log out; Disclaimer, Policy and Low data view move to the Settings page under About Pinalert). Build that menu with
care: a white surface with the house corner and shadow, the email as a muted 14px line with a divider under it,
44px rows with even 16px side padding, icons on every row or on none, Log out set apart at the bottom, clear hover
and keyboard focus, and the same look in light and dark. Judge it at 320, 360 and 390 px first, then laptop.
