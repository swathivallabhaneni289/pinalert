---
created: 2026-10-27T13:45:00Z
title: Agree and Disagree appear after the person reads the disclaimer and policy
area: ui
files:
  - web/templates/welcome.html.tmpl (step 2, #welcome-scrollbox, #welcome-disagree, #welcome-agree)
  - web/static/js/welcome.js
  - web/static/css/welcome.css
---

## Problem

The owner, during the Phase 3.3 owner checks on 2026-10-27 (iPhone simulator, first time flow step 2), said:
"i think we should have the agree and disagree button pop up after the user reads the disclaimer and policy page. like how any other app deals with it."
Today the Agree and Disagree buttons show at once under the scrolling disclaimer and policy text.

## Solution

Agreed direction (my recommendation, owner delegates design polish):
- Agree and Disagree appear once the person has scrolled to the end of the disclaimer and policy text, the way terms screens in other apps work.
- If the whole text fits on the screen, they show at once.
- Keyboard and screen reader users reach the end of the text the same way, by scrolling the text box.
- The same on phone and laptop.
- Keep Read again and Log out as they are.

Do it as one small quick task after the 3.3 owner checks, then check it in the iPhone simulator and Mac Safari.
