---
created: 2026-10-27T13:45:00Z
title: Sign in page, with Low data view, Disclaimer and Policy, designed more neatly
area: ui
files:
  - web/templates/login_gate.html.tmpl
  - web/static/css/auth.css
---

## Problem

The owner, during the Phase 3.3 owner checks on 2026-10-27, looking at the sign in page in the iPhone simulator, said:
"also in the starting page where its sign in and low data view mode - should be designed more neatly".
Today the page shows the "Verify your email to continue" card. Under it is a second grey card holding a Low data view button with the sub line "Text only, no map", then Disclaimer and Policy buttons. The owner reads this second card as untidy.

## Solution

The sign in page will be rebuilt anyway:
- Phase 7.1 adds password sign in and the typed 6 digit code.
- Phase 5 adds "Try the demo · Disclaimer · Policy" to this screen (05-CONTEXT D-03).

So fold "designed more neatly" into that rebuild rather than redesigning the screen twice:
- one clean card for signing in;
- the secondary links (Try the demo, Low data view, Disclaimer, Policy) as one tidy row of real buttons, not a second boxed card;
- even spacing, phone first.

Follow the site design rules: no pill buttons, no underlined links, no hint text under simple options. Show the result to the owner on a test copy.
