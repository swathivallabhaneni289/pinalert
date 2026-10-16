# Sketch 007: App layout (approved 2026-10-16)

**Design question:** Where does everything live, and how does each screen look on a phone and on a laptop, so that the
two offer exactly the same features, nothing is repeated, hidden or covered, and reporting from a phone is the
quickest path?

**Winner:** this file, as it stands. The owner reviewed it round by round on 2026-10-16 and approved it ("this looks
fine"). The decisions, the rules and the conflicts with built code are in
`.planning/phases/03.4-app-layout/03.4-CONTEXT.md`.

## How to open it

Open `index.html` in a browser (a file URL works; all icons are inside the file). The links at the top switch between
**Laptop view** and **Phone view** (a phone sized frame on a wide screen) and **Show the first time screens** replays
the five step first-time flow. Everything on it is sample data from the 2024 Vijayawada floods, marked as a mockup.

Handy address parameters: `?view=phone` or `?view=laptop`, `screen=alerts|activity|settings`, `open=report`,
`sev=low|medium|critical`, `open=welcome&step=1..5`, `pop=bell|show|person`.

## What it shows

- Main places: Nearby, Official alerts, My activity; phone bottom bar with a fourth Report item; laptop top bar with
  Post a report, search, the bell, the light and dark button and the person menu (My profile, Settings, Log out).
- Nearby: one Show menu (All, Confirmed, Disputed, Only Reliable reporters); disputed reports labelled at the bottom of
  All; the phone list as a sheet over the map (short, half, full; scrolling up grows it); tap a report to see its pin,
  tap a pin to see its report.
- My activity: phone opens on a short list of five parts, each opening its own screen; laptop shows the parts stacked
  with aligned columns.
- Official alerts, Settings, Post a report (phone full screen, laptop two pane window), the Disclaimer and Policy
  reader, and the first-time flow (Welcome, Disclaimer and policy with Agree, Location, About you, Alerts).

## Rounds that were rejected on the way (do not bring them back)

- Tab strips on My activity (laptop and phone), side by side card grids, centred laptop pages.
- A notice strip, an amber strip and an "Emergency? Call 112" box on the phone main screens.
- A floating round Report button that covered reports; a filled dark Report block in the bottom bar.
- Radio circles and hint lines on How serious; a black outline around a chosen option.
- Icon tiles and a two row word block for My activity's parts.

## Checks run

A 23 step scripted click through (navigation, Settings, Disclaimer and Policy, bell, Show menu, Post a report, the
first-time flow) passes at 390 px and 320 px phone width, at laptop width and in dark mode, with no script errors.
Pictures were taken in the session scratchpad only, never saved here.

## Departures from house rules and earlier decisions

Listed in `03.4-CONTEXT.md` under "Conflicts and risks": the phone carries no disclaimer strip (ROBUST-08), the phone
has no light and dark button (UX-01), Phase 3.1's tab strip and Past tab are replaced, and today's six tab Activity
page layout is replaced while keeping every feature.
