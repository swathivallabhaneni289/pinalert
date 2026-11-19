---
quick_id: 261119-sps
slug: update-rows-show-the-report-area-name
date: 2026-11-19
type: quick
---

# Update rows show the report's area name

## Why

3.2 owner check, 2026-11-19, looking at the bell on the laptop: "i think in those updates it should
display the area name". Shown the row as it is ("Road blocked · Low") and the proposal ("Road
blocked · Low · Benz Circle", the same short area name phone alerts already use, and the row stays
as now when no name can be found). Owner: "yes build it".

This is an owner decision that amends Phase 3.3 D-19 and D-20 (both Claude's discretion), which kept
the looked up area name out of bell rows and the feed. Bell rows carry no distance, so the reason
given there (a home area plus a distance plus a landmark narrows the spot) does not apply to them.
The name stays the coarse one (no road, house number or postcode), cut by `AreaWords`.

## What changes

### 1. The row carries the area

- `UpdateWire` (internal/api/handlers/updates.go) gains one optional field, `area` (string,
  omitempty): the report's stored `reports.area_name` passed through `service.AreaWords`. Swagger
  annotation and regenerated docs/swagger.* in sync (keep the swag `@license.name` line exactly as it
  is; the regenerated files must hold no em or en dash).
- The area is absent whenever the row hides the report's own words: the report is unavailable
  (held, removed, gone), a finished Rescue report (the same rule that drops `report_line`), and the
  rolled up row. It is also absent on rows that are not about one report. Every other row kind that
  is about one report carries it when a name is stored, the reader's own report rows included.
- The row's accessible name (`label`, built whole on the server) gains the area after the category
  and severity, in the same style as the rest of the label (for example "Road blocked, Low, Benz
  Circle."). No area, no change to the label.

### 2. Where it shows (phone and laptop alike)

- The bell list (web/static/js/updates.js): the type line becomes "{category} · {severity} ·
  {area}" with the same middle dot. With no area the line is exactly as now. With a category but no
  severity it is "{category} · {area}". No area is drawn on a row with no type line.
- The Updates screen (See all updates, `/profile?part=updates`, filled by `fillUpdatesPart` in
  internal/api/handlers/activity.go and its template): the same line and the same label rule.
- No other surface changes: not Nearby, the map, the report page, Ended, the phone notifications or
  the Share title.

### 3. Getting a name for reports that have none

Today a name is looked up only when a phone alert or a Share link needs one
(`service.LookUpReportAreaName`, once per report, recorded before the outside call, bounded by
`PushAreaLookupTimeout`, through the shared 1 request per second geocoder).

- When the bell list (GET /api/updates) or the Updates screen is read by a signed in person and a
  row that would carry an area is about a report with no stored name whose lookup was never tried,
  the server starts that same lookup **in the background, after the answer is written**. The answer
  never waits for it. The name shows on the next refresh (the bell polls every 30 seconds).
- At most 3 such lookups start per read. A report is looked up at most once ever, also when two
  reads (or a read and a phone alert or a Share) ask at the same moment: claim the attempt with one
  conditional write (only when not yet tried) and only the caller that won the claim calls out.
  Keep the existing callers working through the same rule.
- The background work uses its own context with the existing timeout, never the request's, and
  logs only the report id on failure (as areaname.go does now).
- Never from an unauthenticated route (the rule at the top of areaname.go stays true; extend its
  comment to name the two new callers). The Demo never looks anything up.
- Only the report's own coordinates are sent, never the reader's.

### 4. Docs and Policy

- docs/PUSH.md (line about the area name being "in no feed, bell or page response") and
  docs/TRUST-MODEL.md (the "What is never sent" paragraph and the "Area name amended" bullet): say
  the same stored coarse name now also shows on update rows in the bell and the Updates screen, on
  the owner's word of 2026-11-19, and is looked up once per report when a row first needs it. Few
  words.
- Policy (web/templates/legal_body.html.tmpl): find the sentence that says when a report's location
  is sent to the place search service to name its area. If it lists only phone alerts and Share,
  add the bell in a few words; touch no other Policy line. Do not change the Policy date or
  CurrentTermsVersion (they move on deploy day). If no such sentence exists, change nothing there and
  say so in the summary.
- Amend .planning/phases/03.3-installable-app-and-phone-alerts/03.3-CONTEXT.md D-19 and D-20 with a
  dated note pointing at this quick task. Do not edit any UAT file.

## Tests

- Handler and service: a row about a report with a stored name carries `area` (cut by AreaWords)
  and the label holds it; no stored name, an unavailable report, a finished Rescue report, the
  rolled up row and a row not about one report carry no `area`; the Updates screen shows the same.
- The background lookup: a read with rows missing names starts at most 3 lookups and answers before
  any of them finishes (a fake namer that blocks proves the answer does not wait); a report whose
  lookup was tried is never looked up again; two reads at once look a report up once; a failing
  namer stores nothing and is not retried; the name appears on the next read. A test that an
  unauthenticated route never calls the namer (extend the existing D-06 guard).
- Node (web/updates_test.mjs or the bell's existing test file): the line with category, severity
  and area; category and area; no area keeps the line exactly as now; no type line, no area.
- Swagger in sync test, the no dash contract tests, and the existing bell and Updates tests, amended
  with dated notes where they pinned the old row.
- Run `node --test web/*.mjs`, `go build ./...`, `go vet ./...`, and `go test ./...` with your own
  scratch database `p9_sps` (createdb, migrate the way the repo's tests do, drop it after).
- Live look: your own server on port 8789 with a scratch database, ENV=development and the log
  mailer, made up example.com accounts only. Make bell rows (for example a Reliable reporter row or
  a confirm on someone's report), open the bell: the first read shows the row without an area, the
  next refresh shows it with one (the real geocoder is fine to call; if it is unreachable, say so and
  prove the path with the fake instead). Playwright WebKit
  (NODE_PATH=/Users/swathivallabhaneni/.npm/_npx/e41f203b7505f1fb/node_modules) at 390 (touch) and
  1280, light and dark: the bell list and the Updates screen, the line fits or wraps cleanly, nothing
  scrolls sideways. Screenshots only in your own scratchpad subfolder, never in the repo. Do not
  drive the iPhone simulator.

## Rules

- Fix only this. Do not change any other surface, the row's other words, the bell's timing, the
  phone notification text, Share, moderation, flags or migrations, unless the claim in part 3 needs
  a small query change; no new migration unless there is truly no way without one (say why in the
  summary if so).
- No em or en dashes in code comments, copy or commits. Never run python or python3, not even
  --version.
- Keep all wording neutral, as the owner's wording rule asks.
- Path limited commits (`git commit -- <paths>`), each ending with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md,
  ROADMAP.md or REQUIREMENTS.md. Do not edit UAT files.
- Never use ports 8080, 8090, 8776, 8778, 8781, 8782 or 8797. Never touch the databases
  pinalert_test, pinalert, p31_4_uat, p34_uat, p3_view, p4_uat or p9_view, and never run a
  migration on any database but your own scratch ones.
- Write 261119-sps-SUMMARY.md in this folder when done (what changed, test counts, the live look
  result with screenshot paths, any deviation) and commit it.
