---
phase: 09-followable-reporter-profiles
plan: 24
subsystem: web
tags: [notify-me, bell, profile, html, js, css]
requires:
  - phase: 09-23
    provides: POST /api/people/{username}/notify, the notify key of the profile JSON
  - phase: 09-20
    provides: the Mute item, muted on the profile, the refetch and redraw
  - phase: 09-08
    provides: person.html.tmpl, person.js, #person-actions
provides:
  - the Notify me bell button on another person's profile
  - the bell toggle in person.js (post, pressed state, status lines, errors)
affects: [09-25, 09-26]
tech-stack:
  patterns:
    - "the bell is always drawn on another person's page and shown or hidden from the server's following and muted flags, like Follow and Following; the icon follows aria-pressed in CSS, so one attribute is the whole state"
key-files:
  created:
    - web/person_bell_test.mjs
  modified:
    - web/templates/person.html.tmpl
    - web/static/js/person.js
    - web/static/css/person.css
    - web/person_contract_test.go
    - web/layout_rules_contract_test.go
key-decisions:
  - "The bell button is always in the markup of another person's page and carries hidden unless you follow and have not muted. The plan said 'rendered when Following and not Muted', but person.js never builds a button, so the hidden flag is the way the bell appears after Follow and returns after Unmute."
  - "The pressed state is set from the server's saved answer ({notify}) and, after any refetch, from the profile's notify key, never from the tap."
requirements-completed: [PROF-03]
status: complete
completed: 2026-10-30
---

# Phase 9 Plan 24: The Notify me bell Summary

A follower can switch Notify me on and off from a small 44 by 44 bell between Following and More; it appears the moment Follow succeeds (off), disappears on Unfollow and on Mute, and comes back off after Unmute.

## What was built

- **person.html.tmpl**: `button#person-bell.person-bell` after Following, before More, only in the other-person branch. `aria-pressed` comes from `.Profile.Notify`, `hidden` unless `Following` and not `Muted`, the name is the visually hidden text "Notify me when {username} posts" (html/template escaped), the glyph is an `aria-hidden` span.
- **person.js**: `toggleBell` posts `{"on": <opposite of aria-pressed>}` to `/api/people/{username}/notify`, disables the bell with `aria-busy` while out, sets `aria-pressed` from the answer's `notify`, says "You will be notified when {username} posts." or "You will not be notified when {username} posts.", and puts focus back on the bell. A 409 shows the server's sentence (the 50 person cap or "Couldn't change this. Try again.") and leaves the state; other failures and network failures show "Couldn't change this. Try again."; offline sends nothing and shows the built offline line; a 404 reloads. `drawBell` runs from `draw()`: it shows the bell when `following === true` and `muted !== true` and takes `aria-pressed` from `notify`, so Follow, Unfollow, Mute and Unmute (all refetch) redraw it. Unfollow focus still lands on Follow (existing code).
- **person.css**: `.person-bell` 44 by 44, 1px `--color-border`, radius 8px, 24px mask glyph (`bell-off.svg`, `bell.svg` while `aria-pressed="true"`), `--color-surface` fill as the chosen look, `border: 1px solid Highlight` under forced colours, `:not([hidden])` guard, no animation, no `!important`.

## Tests

- `node --test web/person_bell_test.mjs web/person_test.mjs web/person_mute_test.mjs`: 55 pass (14 new in person_bell_test.mjs: absent before follow and off after, one post with `{"on":true}` and the exact sentence, second tap off, pressed follows the server answer, busy state and no double send, both 409 sentences, 500 and network and offline lines, 404 reload, Mute and Unmute, Unfollow with focus on Follow and a fresh off bell after following again, hidden bell sends nothing, template order and name, no markup sink and no sound wording).
- `node --test web/*.mjs`: 1604 pass, 0 fail.
- `TestPersonBellMarkup` (new) passes with `-v`, together with `TestNoDarkBorderOnChosenItems`, `TestNoNoiseWordsInUserCopy`, `TestNotificationsNoFollowOrOfficialRow`, `TestNewControlsMeetTouchTarget` and `TestNoMarkupSinksAnywhere`. `.person-bell` added to `touchControls`.
- `go vet ./...` clean. Full `go test ./... -p 1 -count=1` on the scratch database `p9_24_a` (created fresh, dropped at the end): every package ok, exit 0; `TestPersonPage*` (the real page through the template) pass with the bell in the markup.

## Commits

- 5816fafe: feat(09-24): add the Notify me bell to the profile
- ad4fb39a: test(09-24): pin the Notify me bell in the contract tests

## Deviations from Plan

**1. [Interpretation] The bell is always in the markup and hidden, not left out of it.** See key-decisions. Visible behaviour matches the plan and UI-SPEC 1d: it is not drawn for a stranger, a muted person or after Unfollow, and hidden controls leave the accessibility tree and the tab order.

**2. [Plan wording] No `.person-bell` text under the button.** Nothing was added under the bell; the status line is the only announcement (UI-SPEC Defaults 11).

No auth gates.

## Notes

- 09-25 and 09-26 read nothing from this plan; the bell sends nothing but the notify route call.
- Settings has no Notify me switch (D-11); none added.

## Known Stubs

None.

## Threat Flags

None. T-09-83: the username reaches the bell's name through html/template escaping and the status lines through `textContent` (`TestNoMarkupSinksAnywhere` and the no markup sink test in person_bell_test.mjs pass). T-09-84: no sound or vibration word in the bell's markup, rules or script (`TestPersonBellMarkup`, `TestNoNoiseWordsInUserCopy`).

## Self-Check: PASSED

web/person_bell_test.mjs exists; commits 5816fafe and ad4fb39a are in `git log`; no non-ASCII characters or dashes in any touched file.
