# Phase 8: Report page and comments - Research

**Researched:** 2026-10-25
**Domain:** Go (chi, pgx, sqlc, goose) server rendered app with a vanilla JS client: a per report page (trust block reuse, vote log timeline, Share), a public no sign in status page, moderated comments with replies and likes, and the notification and limiter plumbing around them.
**Confidence:** HIGH on the code facts (every claim about this repo was read in the files named), MEDIUM on the WhatsApp preview behaviour (Meta's own page, but "best effort"), MEDIUM on the limiter numbers (reasoned, not measured), LOW on anything that depends on Phase 7.1 (no plans exist yet).

<user_constraints>
## User Constraints (from CONTEXT.md)

Copied verbatim from `.planning/phases/08-report-page-and-comments/08-CONTEXT.md`. Researcher's note, outside the copied text: the "Split at planning" line at the top of that file says D-07 to D-10 and the sign in half of D-11 moved to Phase 7.1 on 2026-10-25, so this phase does not build them and only consumes the Username.

### Locked Decisions

**The report page (locked)**
- **D-01 [LOCKED EARLIER, sketch 008 and ROADMAP scope note 5]:** Laptop: the report opens beside the map in place of the Nearby list (B), every pin still on the map, another pin switches the report. Phone: a full screen page whose map keeps every pin. Tapping a pin opens its report straight away on both sizes. The page uses the popup's trust block as shipped (sketch 003 H), not a table. Order: the report, the trust block, actions (Confirm, Dispute, or Mark resolved with Undo on your own; Share; quiet links Stop updates and Flag), "What happened so far" timeline from the vote log with no voter names or places, then Comments. Keep it calm ("make sure it doesnt look too busy"). Every detail is in `.planning/sketches/008-report-page/README.md`.
- **D-02 [LOCKED EARLIER, sketch 008 item 5]:** Comments: the same list on phone and laptop under a "Comments" heading with the count, Instagram like; each comment has a round picture, the username, the time, the words, Reply, then a thumbs up like with its count (one per account per comment, never on the report, never feeding any trust number, never a heart). Replies folded behind "View N replies", one level only. One box: Reply fills "@username ", shows "Replying to username" with an X. 280 letters, "N left" only in the last 40. The box is pinned to the bottom of the panel on a laptop and sits after the last comment on a phone. Your own comments show only your username, never "You", and no Reporter tag on your own report; others see Reporter beside the reporter's name.
- **D-03 [LOCKED EARLIER, ROADMAP SC4 and scope note 3]:** The popup footer shows a real comment count only, never a made up one.

**Sharing (COORD-08)**
- **D-04 [LOCKED EARLIER, sketch 008 item 6]:** Share opens a window (a bottom sheet on the phone) showing what people get in WhatsApp: the title and "Open to see if this is still true right now", with a link and no counts, so a forward can never carry a stale number or a "verified" claim.
- **D-05 [OWNER DECIDED, 2026-10-17]:** The shared link opens a **small public page for that one report, with no sign in**. It shows only: what it is and the rough area; whether it is still happening, confirmed, disputed or over; how many people nearby confirmed it; and when. It never shows comments, names or the exact spot. A "Sign in to see more" button sits at the bottom. Chosen over requiring an account.
- **D-06 [CLAUDE'S DISCRETION]:** Public page details: "what it is" is the category, the severity and, for a Critical report, the What is happening pick; the reporter's own words are not shown, matching the lock screen rule (03.3 D-14). The rough area is a coarse area name, and finished Rescue reports keep their 1 km coarsening (03.1 D-23). A report removed by a moderator shows "This report was removed". The page is `noindex` and `no-store`, rate limited per IP, and its link preview (Open Graph) carries only the title line and "Open to see if this is still true right now", never a count. The route joins the gate's exempt list in `internal/api/gate_test.go` with a note.

**Accounts, names and sign up**
- **D-07 [OWNER DECIDED, 2026-10-17]:** An account is needed to use Pinalert ("should have an account no matter what"). Only these open without one: the Demo (Phase 5), the Disclaimer and Policy pages, and the D-05 shared report page.
- **D-08 [OWNER DECIDED, 2026-10-17, chosen over the advice to make Name optional]:** Step 4 of the first-time flow (About you) requires all three: **Username** (live check), **Name** and **Home area** (a town or area, never a street). No Skip. The owner: "do have the name because ... for the profile page thing ... you know how the Instagram works it should work exactly like that". Today's step 4 says "Both are optional. They show only if you later let others follow you." and has Skip (03.4 as built); this phase changes it once. Earlier accounts fill in whatever is missing once, at their next sign in (scope note 5).
- **D-09 [LOCKED EARLIER, scope note 5]:** The Username is unique and public to everyone, Instagram style; the Name stays a profile only detail (Phase 9). The Username is editable in Settings with a live check (available, taken, wrong characters, or kept for official use such as police, ndma, imd, govt) and is screened like comments.
- **D-10 [LOCKED EARLIER, scope note 6]:** Sign in with a password. The sign in page offers Create account and Sign in. Create account takes an email and a password, emails a 6-digit code once, the person types it in (no link anywhere), then the first-time flow runs. Sign in takes the email and the password only. A forgotten password is reset with the same kind of typed code. Accounts made with the old link set a password once at their next sign in, inside D-08's one-time fill-in, so nobody is stopped twice. Engineering notes from the scope note stand (slow hash, per account and per device limits mindful of CGNAT, "Email or password is wrong", same "we sent a code" screen for a known email, short lived single use hashed codes).
- **D-11 [CLAUDE'S DISCRETION]:** Planning may split the new sign in (D-10) and the required step 4 (D-08) into their own decimal phase before the report page, because they replace the whole sign in and need their own mockup. If split, the two ship together so step 4 changes only once. The Phase 5 globe and the "Try the demo · Disclaimer · Policy" row must survive the new card (05-CONTEXT D-03, D-14).

**Comments: rules the owner was not asked about**
- **D-12 [CLAUDE'S DISCRETION]:** Screening follows Phase 4's report rule: clearly abusive text is rejected with the box kept open; borderline text posts and is listed for moderators; if the check cannot finish, the comment is held off the page until it does (comments have no critical exemption). One reusable text check with a held state, not tied to report categories, serves comments and usernames (ROADMAP scope note 4). Comment text never reaches the page through a markup parsing sink.
- **D-13 [CLAUDE'S DISCRETION]:** A per account limit on comments, replies and likes, with a message that says how long to wait, built on Phase 4's limiter; numbers set at planning.
- **D-14 [CLAUDE'S DISCRETION]:** You can delete your own comment at any time; there is no editing (as on Instagram). A deleted comment that has replies leaves "Comment deleted" so the thread still reads. The reporter cannot delete other people's comments on their report; anyone signed in can Flag a comment with Phase 4's quiet control, and moderators Keep or Remove it on the moderation screen. A flag alone never hides a comment.
- **D-15 [CLAUDE'S DISCRETION]:** Comments stay open while the report is current and become read only once it is over (resolved or expired), with a quiet "Comments are closed" line.
- **D-16 [CLAUDE'S DISCRETION]:** A reply to you and a new comment on your own report appear in your in-app updates (Phase 3.2 bell). Likes never notify. Phone notifications for these use the existing "changes to reports I posted or voted on" kind (off by default) and, like every phone notification, are silent (owner, 2026-10-18: no buzzes anywhere).
- **D-17 [CLAUDE'S DISCRETION]:** Ship order inside the phase: the page, the timeline and Share first; then comments (they need the moderation hook and limiter).

### Claude's Discretion
D-06, D-11 to D-17.

### Deferred Ideas (OUT OF SCOPE)
None new in this round. Profiles, follows and stars are Phase 9.

(Also from CONTEXT, Phase Boundary: "Not in this phase: profiles, follows and stars (Phase 9), the official alerts and the Demo (Phase 5), phone notifications (Phase 3.3).")
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PAGE-01 | A report opens into its own page with the shared trust block and a history timeline built from the vote log (posted, confirmed from N places, disputed, resolved) | "Opening a report", "Trust block reuse" and "Timeline" sections: the page is the existing popup card extended in place (phone) and mounted in a docked host (laptop); the timeline is a new replay of the append only `votes` log through `BuildVoteTally` and `Resolve`, with closed copy and no cells or accounts on the wire. |
| PAGE-02 | A signed-in user can read and add short comments, each screened by moderation and rate-limited per account; commenting makes the user follow that report | "Comments", "Limiter", "Notifications" sections: own tables, `ScreenAbuse` plus a comment specific held state, four place limiter wiring, and the follow set extended to commenters in both `FollowedOpenReports` and `FollowersWithChangesPush`. The "follow" half is in the requirement text itself, so it is not optional. |
| COORD-08 | A shareable card with only a live status link, never a baked in count or "verified" claim | "Share and the public status page": an unguessable per report token, a sessionless public page with Open Graph tags that carry no count, the Share window, and the gate and session bypass changes. |
</phase_requirements>

## Summary

Phase 8 is mostly a composition phase: the trust block, the vote log, the text check, the flag control, the limiter, the in app updates and the phone alert outbox all exist already, and each needs a small, named extension rather than a new system. The one genuinely new surface is the public status page, and its main risk is not rendering but enumeration: report ids are sequential `BIGSERIAL` values, so a public `/r/{id}` route would let anyone with no account read the status of every report, which undercuts D-07. Use a per report random share token, created when someone taps Share.

Two locked design statements are in tension with what is shipped, and the planner must surface both rather than resolve them silently. First, D-01 locks the laptop layout "B" (the report beside the map in place of the Nearby list), but on 2026-10-19 the owner said "the laptop ver looks good i dont want you to disturb that"; plan 03.4-39 dropped B from its scope on that sentence, and `web/layout_rules_contract_test.go` now pins "the laptop keeps its popup over the map unchanged". Second, SC4 and D-03 need a popup footer, but `web/popup_contract_test.go` (`TestPopupCardSurfaceAndGeometry`) forbids `__footer` in `map.js` and `trust.css` (amendment A11), so that test must be amended with a dated note. The recommended shape keeps both decisions cheap to flip: build the page sections once as a mount agnostic builder, mount them in the phone detail pane first, and give the laptop host its own plan that waits for the owner's look.

Comments cannot reuse Phase 4's held state. `moderation_checks` has its primary key on `report_id`, the worker claim joins `reports.description`, Keep and Remove lock by report id, and `report_flags` and `moderation_actions` are report keyed. What is reusable is the pure text check (`CheckText`, `ScreenAbuse`, `Decide`, `RejectMessage`). Comments need their own check columns, claim, flags table and moderator list; usernames need only the synchronous check and should refuse to save when it cannot finish. `CheckText` runs `IsGibberish` first, which rejects any text with fewer than two words of two or more letters ("Thanks", "ok"), so comments must use `ScreenAbuse`, the same choice Phase 4 made for flag notes.

**Primary recommendation:** Build in the D-17 order with one mount agnostic `report-page.js` builder (timeline, Share, then comments), a new `reports.share_token` for the public page, comment tables with their own check columns, new rows in `updates` for comment events, and four limiter buckets wired in all four places the limiter lives; hold the laptop host behind an owner viewing.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Report page body (trust block, actions, timeline, comments) | Browser (vanilla JS builder) | API (data) | The popup card is built and refreshed in place by JS today; the page is that card plus sections. The server owns every sentence and rule. |
| Timeline events and words | API / service (pure function over the vote log) | Database (new log query) | Repo rule: the browser holds no threshold or rule (`TestTrustModuleHasNoThresholdsOrSinks`); the server writes closed copy and sends timestamps. |
| Trust block for the page | API (existing `trust` object) | Browser (`trust.js` renders) | Already one JSON object on every read path; reuse, do not recompute. |
| Public status page `/r/{token}` | Frontend server (html/template, no JS) | API service (status read) | Must work for a link preview crawler and a logged out visitor: server rendered, no session, no script. |
| Share token and area name | API (authenticated POST on Share tap) | Database | Creating a capability token and calling the shared reverse geocoder budget must not happen on an unauthenticated route. |
| Comment screening and held state | API (service + background worker) | Database (check columns) | Same split as Phase 4 reports: inline attempt with a 3 s bound, retry in the background. |
| Comment ownership, delete, likes, closed state | API | Database | Server authored flags (`is_own`, `is_reporter`, `can_delete`, `comments_open`); the wire never carries an account id. |
| Rate limits | API (middleware via `RateGate`) | Config (`cmd/server/main.go`) | One gate, four places to wire. |
| Notifications for comments | API (write time `updates` insert) | Browser (bell reads), push worker | 3.2 updates are computed at read time for state changes, but a comment is an event, so it is inserted when it happens. |
| Comment count in the popup | API (extras read) | Browser (footer) | A real count only, supplied by the same batched extras query as the viewer flags. |

## Standard Stack

No new external package is needed. Everything below is already in `go.mod` or the stdlib. [VERIFIED: codebase read of go.mod, router.go, cmd/server/main.go]

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go-chi/chi/v5` | in go.mod | Routes for `/r/{token}`, `/api/reports/{id}/timeline`, comment routes | Project constraint; existing router groups |
| `jackc/pgx/v5` + `sqlc` v1.31.1 | installed locally | New tables and queries, generated code is committed | Project constraint; `make sqlc` |
| `pressly/goose` (embedded migrations) | v3.27.2 local binary | One migration for share token + comments, or two (see split) | Project constraint; "next free migration number at execution time" |
| `html/template` | stdlib | Public status page, auto escaped Open Graph attributes | Project constraint |
| `swaggo/swag` | v1.16.4 local binary (CLAUDE.md lists 1.16.6) | `make swag` regenerates `docs/` | Existing drift guard `TestSwaggerSpecCoversRoutes` |
| Node `node:test` + `vm` | Node 26.4.0 local, 22 or 24 on CI | Browser script tests run by `node --test web/*.mjs` | Existing pattern (`report_view_test.mjs`) |

### Supporting (existing internal packages to reuse, not libraries)
| Unit | Used for |
|------|----------|
| `internal/service/trustcopy.go`, `trustfacts.go`, `BuildTrust` | The page's trust block comes through the same `trust` JSON object |
| `internal/service/history.go` (`ClosureOf`, `ViewOf`), `historycopy.go` | Prefix replay pattern and resolved/expired wording for the timeline |
| `internal/service/textcheck.go` (`ScreenAbuse`, `Decide`, `RejectMessage`) | Comment screening |
| `internal/api/handlers/ratelimit_gate.go` (`RateGate`, `writeRateLimited`) | Comment and like limits |
| `internal/ratelimit.NewPerIP` | The public page's per IP limit |
| `internal/service/updates*.go`, `pushkinds.go`, `pushcopy.go`, `pushworker.go` | Comment notifications |
| `internal/api/handlers/flag.go`, `web/static/js/flag.js`, `flag_dialog.html.tmpl` | Flag a comment |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Stored random `share_token` | HMAC of the report id with a derived key | Stateless and no column, but a rotated `SESSION_SECRET` silently kills every link already forwarded in WhatsApp, and a token cannot be revoked per report. Stored token wins. |
| Comment check columns on `comments` | Generalise `moderation_checks` to a subject kind and id | One worker for both, but it rewrites the Phase 4 primary key, the claim join and the lock order that three race tests pin. Not worth the risk to shipped code. |
| `golang.org/x/text/unicode/norm` for comment text | stdlib `unicode.Cf` and control character rejection | Normalisation is nicer for lookalike text but adds a dependency; rejecting format characters covers the invisible character abuse. Do not add the package. |

**Installation:** none. **Version verification:** not applicable, no package is added.

## Package Legitimacy Audit

No external packages are recommended for installation in this phase, so the Package Legitimacy Gate has nothing to run.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| none | | | | | | |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Project Constraints (from CLAUDE.md)

- Stack is fixed: Go, `chi/v5`, `pgx/v5` + `sqlc`, goose migrations, `html/template` plus vanilla JS with no build step, Leaflet with MapLibre. No PostGIS.
- Free tiers only; moderation is OpenAI `omni-moderation-latest` (already the `TextScreener`).
- Anonymous posting was replaced by accounts (Phase 1.1); rate limiting is session/account first, IP second, with CGNAT in mind.
- GSD workflow: work starts through a GSD command; this research writes only `08-RESEARCH.md`.
- Project skill `sketch-findings-pinalert`: the popup is a summary, "Comments live on a larger report page, never inside the popup" (SKILL.md context). The footer of the d4 popup is one link to the report page with a count (`references/popup-structure.md` line 22).
- Owner rules in memory that bind plans and tests: no em or en dashes in any prose or visible copy (`TestUserVisibleCopyUsesPlainPunctuation` scans every template automatically); no images in the repo; no underlined links at rest (`TestNoUnderlineAtRest`); actions are real buttons; every phone notification is silent and no copy may promise buzzing (`TestNoNoiseWordsInUserCopy` bans buzz, vibrate, sound, ring, silent, quiet, hear for the files it lists); same features on phone and laptop, only placement differs; show layout to the owner before locking questions; never use ports 8080 or 8090; never test against `pinalert_test`.

## Architecture Patterns

### System Architecture Diagram

```
                     GET /?report=ID or pin tap or list row
                                  |
          +-----------------------+------------------------+
          |                                                |
   Pinalert.openReport(id) (app.js)                  Leaflet marker click
          |  PinalertMap.openReport / focus read           |
          +-----------------------+------------------------+
                                  v
                   buildPopupCard(report)  [map.js]  one card per report
                       |  trust block, votes, mute, flag (existing, built once, updated in place)
                       |  + PinalertReportPage.mount(card)  <-- NEW, mount agnostic builder
                       |        sections: timeline, comments, Share button
                       v
        host A: detailPane (phone, full screen sheet)     host B: docked panel (laptop, owner viewing)
                       |
      browser fetches on open and every 30 s while visible (same cadence as the feed poll)
        GET /api/reports/{id}/timeline        --> VoteLogForReport --> BuildTimeline (pure) --> closed copy
        GET /api/reports/{id}/comments        --> comments + likes + viewer flags --> server authored flags
        POST /api/reports/{id}/comments       --> RateGate --> ScreenAbuse --(clear)--> row visible
                                                              |-(borderline)--> row visible + needs_review
                                                              |-(rejected)---> 422, box stays open
                                                              '-(unfinished)--> row pending, worker retries
        POST /api/reports/{id}/share          --> RateGate --> ensure share_token + ensure area_name --> {url, title}

   Share window (browser) --> navigator.share or Copy link --> WhatsApp
                                                                  |  crawler GET /r/{token}  (no cookie, UA WhatsApp/2.x)
                                                                  v
   GET /r/{token}  (exempt from the gate, skipped by the session middleware, per IP limit)
        --> token -> report -> Focus style resolve (viewer 0) --> status words + trust.ConfirmedText + coarse area
        --> html/template page, noindex, no-store, og:title / og:description / og:url (+ optional og:image)
        --> "Sign in to see more" --> /login

   Comment event (post, reply) --> updates row for the recipient (INSERT at write time)
        --> PushDispatcher poke --> EnqueueChangedFromUpdates (kind list extended) --> push worker (silent, no text)
```

### Recommended Project Structure
```
internal/service/
├── timeline.go            # BuildTimeline (pure), event types, replay over the vote log
├── timelinecopy.go        # closed copy, same rules as trustcopy.go
├── share.go               # ShareService: EnsureToken, EnsureAreaName, PublicStatus
├── comments.go            # CommentService: Post, Reply, Delete, Like, List
├── commentscopy.go        # messages (closed, plain punctuation)
├── commentworker.go       # held check retry, mirrors modworker.go
├── commentmoderation.go   # moderator list, Keep, Remove for comments
internal/store/queries/    # votes.sql (VoteLogForReport), comments.sql, share.sql, additions to updates.sql, push.sql, reports.sql
internal/api/handlers/     # timeline.go, comments.go, share.go, public_status.go
web/templates/             # share_status.html.tmpl (public page), share_window partial in index.html.tmpl
web/static/js/             # report-page.js (builder), comments.js, share.js
web/static/css/            # report-page.css, comments.css, share.css
```

### Pattern 1: Build once, update in place (the codebase's own rule)
**What:** `buildPopupCard` creates DOM nodes once per marker, `updatePopupCard(card, report)` refreshes them on every poll, and there is no `innerHTML` anywhere. Sketch 008 requires that "a like, an open thread and a half typed comment all survive a vote". So the new sections must be separate nodes appended once and refreshed by their own fetch, never rebuilt by `updatePopupCard`.
**When to use:** every new section on the page.
**Example:**
```javascript
// Source: web/static/js/map.js buildPopupCard / updatePopupCard (read 2026-10-25)
// The card object is kept in cards[report.id]; sections hang off it as fields.
card.page = PinalertReportPage.create(report);   // builds timeline, share button, comments once
body.appendChild(card.page.el);                  // appended AFTER the quiet links, see the warning below
// in updatePopupCard(card, report):
card.page.update(report);   // touches counts and flags only; never replaces the composer or open threads
```
**Child order warning (blocks a naive copy of the snippet).** On the phone `arrangeCard` (map.js, called from `applyPopupPlacement` on every bind and every placement, including a resize across 900px) runs `lastParent.appendChild(mute.el)` and `lastParent.appendChild(flag)` with `lastParent = body`. A page node appended to `body` is therefore overtaken on the next placement: Stop updates and Flag would move to after the timeline, the comments and the composer, breaking D-01's order and the rule that the box sits after the last comment. Fix in the plan: change those appends to `body.insertBefore(mute.el, card.page.el)` and `body.insertBefore(flag, card.page.el)` (or re-append `card.page.el` as the last step of `arrangeCard`), and add a node test that builds a card, runs `arrangeCard` for phone, laptop and phone again (a resize past 900px and back), and asserts the final child order is fade line, actions, trust block, then Stop updates and Flag, then the page node.
`web/report_view_test.mjs` slices `map.js` between the plan 39 comments `// ---- The report view and its address (plan 39)` and `// ---- End of the report view (plan 39)`. Do not move or rename those two markers.

### Pattern 2: Replay the append only vote log through the same resolver
**What:** `votes` is append only. `CurrentVotesForReports` keeps one latest row per (report, account, kind), so it cannot give a history. A new query reads the whole log for one report in id order; the service replays prefixes the way `ClosureOf` does, building the standing row per (account, kind) after each row, calling `BuildVoteTally(standing, reportID, reporterAccountID)` (which already excludes the reporter and counts distinct cells) and `Resolve(meta, tally, rowTime)`.
**Emit an event only when** the confirming or disputing place count crosses a milestone upward, or the visibility changes (Provisional to Live, Live to Hidden, any to Retracted, Retracted back), or the report expires. Do not emit on every row.
**Example:**
```go
// Source: internal/service/history.go ClosureOf (prefix replay), adapted
for i, row := range log {                       // ordered by id
    standing[key{row.AccountID, row.Kind}] = row // latest wins, same rule as DISTINCT ON
    tally := BuildVoteTally(valuesOf(standing), reportID, reporterID)
    vis, _ := Resolve(meta, tally, row.CreatedAt)
    // compare tally.ConfirmCells / DisputeCells and vis with the previous step; append events
}
```

### Pattern 3: Server authored flags, never account ids
The wire carries `is_own`, `is_reporter`, `can_delete`, `viewer_liked`, `viewer_flagged`, `comments_open`, and the username. It never carries an account id, an email or a session id (same rule as `TrustResponse`, pinned by `TestTrustResponseOmitsIdentifiers`; add the equivalent for comments).

### Anti-Patterns to Avoid
- **Copying the trust block into the page.** The block is the popup's own nodes (`card.trust`); on the phone the page IS the popup card (plan 39), so there is nothing to duplicate. On the laptop host, move the same card node, do not rebuild it.
- **A second vote read for the timeline that bypasses `BuildVoteTally`.** The reporter exclusion and the per account dedup live there. A hand written count will drift from the trust block.
- **Rendering comment text with a markup sink.** `TestNoMarkupSinksAnywhere` fails the build on `innerHTML`, `outerHTML`, `insertAdjacentHTML` or `document.write` anywhere in `static/js`, comments removed first. Use `Pinalert.setText` or `document.createElement` plus `textContent`. The bold `@username` at the start of a reply is a `<strong>` element made with `createElement`, with the name set through `textContent`.
- **Letting `CheckText` screen comments.** Its gibberish prefilter refuses one word answers. Use `ScreenAbuse`.
- **Creating a share link on the public route.** The token and the area lookup belong behind the gate on the Share tap.

## Findings by research question

### 1. Opening a report: the address, the phone page, the laptop, history

Facts from `web/static/js/map.js`, `app.js`, `feed.js`, `phone.css`, `index.html.tmpl` (read this session).

- **A pin tap already opens the report on both sizes.** `upsertMarker` binds the card as a Leaflet popup and the marker `click` calls `Pinalert.select(id, 'map')`. The popup is `minWidth: 340, maxWidth: 340`. The sketch's "Phase 8 adds this" for the pin tap is therefore already true in code; no work is needed for D-01's pin sentence except the laptop host.
- **The phone "page" is the popup card in the detail pane.** `applyPopupPlacement` puts the popup in `detailPane` when `phoneDetail()` (media `(max-width: 899.98px)`); `phone.css` lays `.leaflet-detail-pane .leaflet-popup` out as a fixed full screen sheet from 160px down to the bottom bar, with `.leaflet-popup-content-wrapper` as the scroll box. The detail host `div.leaflet-container.report-view-host` is a child of `document.body`, outside the map container, so Leaflet's document level keyboard handler is not engaged by typing in a field there. [CITED: Leaflet 1.9.4 `Map.Keyboard.js`: the keydown listener is on `document`, added only on map `focus` and removed on `blur`, and `_onKeyDown` checks modifiers but not the target.]
- **The address and history are done.** `?report=ID` is written by `noteReportOpened` (push on first open from Nearby, replace on a switch), removed or `history.back()` by `settleReportClosed`, followed by `onReportPopState`, and read on load by the `feed.js` boot line `Pinalert.openReport(...)`. `Pinalert.openReport(id)` is the one public way in (kept for Phase 3.2 bell rows). Phase 8 changes none of this; the share window must not touch it.
- **The laptop is the open decision (Open Question 1).** Today the laptop shows the popup bubble over the map, 340px wide. A timeline and a comment list cannot live in that bubble. Sketch 008 B puts the report in the list column.
- **Recommended mechanism for B, if the owner confirms it.** Do not move the card out of the Leaflet popup. Reuse the phone's detail pane: make `applyPopupPlacement` choose `detailPane` for a new predicate `dockedDetail()` (laptop width) as well as `phoneDetail()`, and let CSS place `.leaflet-detail-pane .leaflet-popup` in the left column over `.pane--list` (position fixed, left 0, the list column's width, top at the app bar bottom). Leaflet then keeps owning open, close, focus return, `popupopen` and `popupclose` and every `report_view_test.mjs` rule keeps working. `phoneDetail()` is currently used for four different things (placement, Back label, strip centring, focus on heading); split it into `phoneDetail()` for the strip and top row behaviours and `detailInPane()` for placement, so the laptop gets the pane without the 160px map strip. The list column is hidden while a report is open (`html[data-popup-open]` already exists and `phone.css` line 122 hides `.pane--list` for it on the phone; add the laptop rule). "All reports" at the top of the panel is the close anchor.
- **Back path.** Close = Leaflet close anchor (Back on the phone), Escape, a tap on empty map, or browser back. All four already converge on `onPopupClose` and `settleReportClosed`. Nothing new.
- **Phone map keeps every pin** is already how the strip works (`centerPinInStrip`, plan 39 summary: "Another pin in the strip opens that report, address 2 to 7, history length stays 3").
- **Composer on the phone sits in flow after the last comment** (D-02), which also avoids the iOS fixed-position-with-keyboard problem; pinning it applies to the laptop panel only. Drive the iPhone simulator Safari for any phone check (owner memory: Playwright WebKit passed layouts iPhone Safari showed broken).

### 2. Trust block reuse

- `internal/service/trustcopy.go` writes every sentence; `trust.go` `TrustResponse` carries `confirmed_places`, `disputed_places`, `confirmed_text`, `disputed_text`, `lead_sentence`, `confirmed_sentence`, `disputed_sentence`, `currency`, `reporter` and the ordered `facts`. `trust.js` renders it (`createStateLine`, the trust block button and panel). The popup, feed rows and Activity rows already share it.
- **Reuse is by construction.** On the phone the page is the popup card, so the trust block is literally the same nodes. For the public page there is no JS: render the same server values with `html/template` (`ConfirmedText`, `ConfirmedSentence`), and do not render the currency or reporter rows (they would be a "verified" style claim and the reporter tag is a person level signal).
- **Do not add trust logic to `report-page.js`.** `TestTrustModuleHasNoThresholdsOrSinks` pins that the browser holds no thresholds; the page reads `trust.*` from the feed row and the timeline from its own endpoint.
- **Popup footer (SC4, D-03).** `TestPopupCardSurfaceAndGeometry` (`web/popup_contract_test.go`, the block ending at line 496) asserts `map.js` and `trust.css` contain none of `popup-card__footer`, `__footer`, `-footer`, `.footer`, "No footer, comments count or comments link (A11)". Phase 8 must amend that test with a dated note and replace it with: a footer exists only where a popup bubble exists, it is one link row (speech bubble, "Comments", the count, "View comments"; "No comments" with "Add a comment" at zero; "1 comment" at one), and the count text is built from a server field. The sketch skill (`popup-structure.md` line 22) holds the visual. If Open Question 1 resolves to B there is no popup bubble on the laptop and the phone page is itself the card, so the count shows as the `Comments N` heading on the page; ask the owner whether list rows should also say "3 comments" (the sketch does not).
- **Supplying the real count.** Add `comment_count` to `ReportExtrasForReports` in `internal/store/queries/reports.sql` as a correlated count of visible comments (top level and replies, `moderation_state = 'clear'`, not deleted, not removed; a held comment is not counted). `ReportView` gains `CommentCount int`, `FeedReportResponse` gains `comment_count` and a server written `comments_open` bool. This is the same batched read that carries `viewer_flagged`, so it adds no round trip. Until 8-comments ships the field is absent and the footer stays hidden (the "defined state" the skill asks for).

### 3. The timeline (SC2)

**What the log holds.** `votes(id, report_id, account_id, kind, value, geohash_cell, created_at)`, append only, no unique key. Kinds: `content` (confirm, dispute) and `resolution` (resolve, reopen, unresolve). There is no stored "resolved at" and no `report_resolutions` table; every resolve, reopen and take back is a vote row. `ClosureOf` already derives the closure moment and who closed it. [VERIFIED: migration 00004, `votes.sql`, `history.go`]

**New query (name it, do not number the migration):** `VoteLogForReport(report_id)` returning `account_id, kind, value, geohash_cell, created_at, id` ordered by `id`, `LIMIT` a safe cap (for example 2000). These columns are service internal; the handler never sees them. The viewer id is passed to the service, which uses it only to produce the viewer's own lines.

**Event vocabulary (from sketch 008 `index.html` sample data, kept):** "Posted as Critical." / "Confirmed from a second spot." / "Confirmed from 2 spots. Shown to everyone." / "Disputed by someone nearby." / "Disputed from a second spot." / "Disputed from a third spot. Marked Disputed." plus resolved and reopened lines and "Faded, no longer updating". Resolved lines reuse the existing `ClosedByFor` and `closedText` words ("Resolved by the poster", and so on). The viewer's own lines ("You confirmed it.", "You disputed it.", "You marked it resolved.") come from the viewer's own standing rows with their real vote times, and are marked `yours: true`.

**Rules to pin in tests (they mirror `trustcopy.go`):**
- A new file `timelinecopy.go` holds every string, built only from constants and `strconv.Itoa`, with the same closed character set (ASCII letters, digits, space, comma, full stop, apostrophe) and the rule that the word "place" appears in no sentence (A88: people and spots). Copy `trustwording_test.go`'s approach.
- No relative time inside a string (Pitfall 8). The wire carries `at` as RFC 3339 and the browser formats it with `Pinalert.relativeTime` or a local clock time.
- **Privacy:** the response carries counts as words and times, never `geohash_cell`, never an account id, never a raw vote total, never a voter's place (TRUST-MODEL "What never leaves the server"). The confirmed number is the distinct place count from `BuildVoteTally`, the same number the trust block shows, never a count of rows. The reporter's own votes are excluded by `BuildVoteTally` as everywhere else.
- **Take backs lower counts.** Recommendation: emit only upward milestones (first reach of 1, 2, 3, 4, 5, 10, 25, 50 confirming places, and the same for disputing places) plus visibility flips and the closure. That bounds a busy report to roughly 16 count rows plus a handful of state rows, keeps the page calm, and never shows a downward step. A reader who sees "Confirmed from 4 spots" in the timeline and "2 confirmed" in the block understands that votes were changed; the block is the current truth. State this in the plan so it is a decision, not an accident.
- **Finished Rescue reports** (stored rows only; `rescue_needed` is retired for new reports) keep the 03.1 D-23 coarsening: the timeline carries no location anyway, so only the description and the 1 km blur matter, and both are already handled by `buildFinishedView`. The timeline for a finished report is still shown.
- **A withheld report (pending or removed) gets no timeline**: answer the same 404 shape as the feed ("not available"), decided by `Resolve(meta, ...) == VisibilityWithheld`, never by reading moderation state into the response.
- Endpoint: `GET /api/reports/{id}/timeline`, inside the gated group, JSON `{ events: [{ kind, tone, text, at, yours }] }`, documented with swag annotations. Not added to every feed row (the feed polls every 30 s for many reports).

### 4. Share (COORD-08) and the public status page

**The link.** Do not use the numeric id. `reports.id` is `BIGSERIAL`; a public `/r/{id}` would let anyone crawl `1..N` and read every report's category, severity, area, status and confirmed count with no account, which contradicts D-07 ("an account is needed"). Use a random capability token: add `reports.share_token TEXT` (unique index where not null), created lazily on the first Share tap (`UPDATE reports SET share_token = $1 WHERE id = $2 AND share_token IS NULL RETURNING share_token`, then re-read on conflict), 16 bytes from `crypto/rand` as unpadded base64url (22 characters). The public route is `GET /r/{token}`. A moderator Remove already makes the page say removed; clearing the token would revoke a link outright if ever wanted.

**Share tap = one authenticated call.** `POST /api/reports/{id}/share` (gated, under the Flag-style limiter or its own small bucket) returns `{ url, title, line }`:
1. ensures the share token;
2. ensures the coarse area name (see below) with the existing non blocking pattern;
3. builds the absolute URL from a configured base URL.
There is no base URL in the router today: `BASE_URL` is read in `cmd/server/main.go` (line 189) and handed only to `AuthService`. Add an additive `Deps.PublicBaseURL` (zero value works, falls back to the request-free localhost default only in development). Never build the absolute URL from the `Host` header, or a crafted Host header poisons every forwarded link and every `og:url`.

**Area name.** `reports.area_name` exists (migration 00011) but is documented as push only, never in the feed or any JSON (03.3 D-19 and D-20), and is filled lazily, only when a push is about to fan out, so it is often NULL. D-06 wants a coarse area name on the public page, so this phase needs an explicit amendment to that rule (public page only, still never in the feed or the bell), and a source for the name: on the Share tap call the same reverse lookup the dispatcher uses (`Client.ReverseArea`), which already returns only a locality, district, neighbourhood, village, town or city, takes a limiter token only when one is free, and gives up after 2 s. Factor the dispatcher's `ensureAreaName` (it also calls `MarkAreaLookupTried` so a failing lookup is not retried on every call) into a small shared function. If the lookup yields nothing, the public page shows no area line; it never falls back to coordinates or a landmark. Never call the geocoder from the public route.

**Public page content (D-05, D-06), recommended words.** Built by a new `ShareService.PublicStatus(ctx, token)` that reuses the `Focus` machinery with viewer 0 (`loadReadContext`, `classify`, `Resolve`, `ViewOf`, `ConfirmedByPeople`):

| View | Status line (existing words) |
|------|------------------------------|
| Active, confirmed by people nearby | "Confirmed by N people at different spots." (`confirmedSentence`) |
| Active, not confirmed | `UnconfirmedLine` "Not confirmed yet." |
| Disputed | "Disputed" with `disputedSentence` |
| Resolved | "Resolved" with the `closedText` words and time |
| Past (expired) | "Ended" with the expired wording from `historycopy.go` |

Do not use "verified" anywhere and do not write "still happening" as a claim; the page is rendered live, which is what makes it true at the moment it is opened. The rest: category label, severity word, for a Critical report the What is happening pick, the coarse area, "Posted N minutes ago" (server rendered, no script). It shows no description, no distance, no coordinates, no map, no comments, no reporter tag, no names.

**Not found, held, removed.** `Focus` answers one word, unavailable, so a moderation state is never revealed (T-31-26). D-06 asks for "This report was removed". Recommendation, recorded as a deviation to keep: only a report with `moderation_state = 'removed'` shows "This report was removed" (a removed report was once public to its sharers, so the fact is not secret); held (pending), unknown token and too old all share one line, "This report is no longer available", with the same 404 status. Needs one small query that reads `moderation_state` by token.

**Title and Open Graph.** In this app the report "title" is the reporter's own words (`popup-card__title`), which D-06 forbids on this page. Build the Share window title and `og:title` from `Category label + " report"` plus, when known, `" near " + area` (for example "Flood report near Benz Circle"); for a Critical report with a What is happening pick, "Water rising reported near ..." is allowed because the pick is a closed list. `og:description` is the fixed line "Open to see if this is still true right now". Meta's current requirements [CITED: developers.facebook.com/docs/whatsapp/link-previews]: `og:title`, `og:description` and `og:url` must be present and non empty inside `<head>` within the first 300 KB; title shown bold, at most two lines, without branding; description one or two lines, about 80 characters; `og:url` the canonical URL with no session variables or counters; the crawler is an HTTP GET with user agent `WhatsApp/2.x.x.x` followed by A, I or N, sends `Accept-Language`, and runs no JavaScript; Meta says it is best effort and may fall back to a small preview. `og:image` is optional. If one is used it must be under 600 KB, at least 300 px wide, ratio no wider than 4:1. Recommendation: include `og:image` as the absolute URL of `/icons/app-512.png` (already a public exact path drawn in code by `internal/appicon`, carries only the app mark, so no count, claim or state can be baked in), after measuring its served size is under 600 KB at build time; if not, omit it and accept the small text preview. Never generate a per report image. No image file is added to the repo.

**Cache and indexing.** `Cache-Control: no-store`, `X-Robots-Tag: noindex, nofollow` and `<meta name="robots" content="noindex">`. WhatsApp does not document its preview cache lifetime, which is another reason the title and description carry no state.

**Gate and session.**
- Register `GET /r/{token}` with the other exempt routes, before the gated group, with a comment in the NewRouter doc block (dated amendment like the 3.3 one).
- `TestAccessGateExemptRoutes` (`internal/api/gate_test.go`) gets a row for a real token (200 for an unverified session) and near miss rows (`/r`, `/r/`, `/r/x/y`, upper case variants, a bad token) that must not answer 200, mirroring the existing "near miss" loop.
- **No session.** The crawler has no cookie. Today the session middleware writes a `sessions` row and sets a cookie on every first request, which is exactly why `legalCacheControl` is `private`. The page must neither set a cookie nor write a row. `skipSessionFor` currently matches exact GET paths only (`router.go` lines 767 to 790, `pwaSkipSet`). Phase 5's draft plan 05-04 generalises it into a `sessionSkip{get, post map[string]bool; prefixes []string}` rule set with `matches(r)` (empty `RawPath`, no `//`, no dot segments, exact or prefix followed by `/`). Phase 5 builds first; name that as the dependency and add `/r` as one prefix. Fallback if Phase 5 has not landed or changed shape: extend the existing map with a small prefix list in the same style and keep the near miss tests. Copy `internal/api/pwa_routes_test.go` `TestPWAAssetsSetNoCookie`: assert no `Set-Cookie` header and an unchanged `SELECT count(*) FROM sessions` after requests with and without a user agent.
- **Rate limit.** Use `ratelimit.NewPerIP` as the request-link route does, scoped with `r.With(...)` to this route only. Numbers must be loose: when no `CLIENT_IP_HEADER` is configured every visitor shares one peer address, and Meta's crawlers share a small set of addresses across every WhatsApp user, so a tight limit would refuse previews for everyone. Propose burst 120, one more per second, per IP, and a 429 with `Retry-After`; keep the CGNAT caveat in the comment. A refused preview fetch means a plain link in the chat, not an outage.
- **"Sign in to see more".** A plain link to `/login`. Note a pitfall: a logged out `GET /?report=ID` is redirected to a bare `/login` and the id is lost (`requireVerifiedAccount` always uses target `/login`), so after sign in the person lands on Nearby, not on the report. Open Question 4.

**Share window (client).** A window on the laptop and a bottom sheet on the phone, built like the existing static dialogs (`flag_dialog.html.tmpl`, `.modal-backdrop`, `window.css`; the owner's rule: boxes behave like a normal system alert, no button outlined on open, only its buttons close it, the page stays still). Content: the preview card (title and the fixed line), the link with a Copy button, and a Share button. Pitfalls: (a) `navigator.share` needs a user gesture and Safari can reject it after an awaited fetch, so open the window on the tap, fetch the link while it is open, and call `navigator.share` synchronously from the second tap inside the window; (b) `navigator.clipboard.writeText` needs a secure context, so keep a fallback that selects the link in a read only input; (c) a direct WhatsApp button can be a plain link to `https://wa.me/?text=<encoded title and url>`, which opens WhatsApp's chooser with no phone number (single source, community documentation, treat as optional). The window must not show a count or the word verified.

### 5. Comments (PAGE-02, D-02, D-12 to D-16)

**Schema (one migration, next free number at execution time).** All FKs to `reports(id)` and `accounts(id)`.
- `comments(id BIGSERIAL, report_id, account_id, parent_id NULL REFERENCES comments(id), body TEXT, created_at, deleted_at NULL, moderation_state TEXT 'clear'|'pending'|'removed', needs_review BOOL, check_attempts INT, next_attempt_at, lease_until, check_error, check_scores JSONB NULL, checked_at)`. Constraints: `char_length(body) <= 280` when not deleted; `parent_id <> id`. A reply to a reply is rewritten by the service to the thread root ("Reply on a reply joins the same thread"), so depth is one; enforce in the service and test it, optionally with a trigger. Indexes: `(report_id, created_at, id)` for the list and the count, a partial index on `next_attempt_at WHERE moderation_state = 'pending'` for the worker, a partial index on `needs_review`.
- `comment_likes(comment_id, account_id, created_at, PRIMARY KEY (comment_id, account_id))`. One like per account per comment is the primary key; self-likes allowed (as on Instagram; nothing ranks by likes).
- `comment_flags(id, comment_id, account_id, reasons TEXT[], note, created_at, dismissed_at, UNIQUE(comment_id, account_id))` with the same reasons check and note length as `report_flags` (migration 00010).
- `comment_moderation_actions(id, comment_id, moderator_account_id, action 'keep'|'remove', created_at)`.
- Store text blanked on delete: `UPDATE comments SET body = '', deleted_at = now()`. Soft delete always, so foreign keys from flags and audit rows stay valid. A deleted comment with no visible replies is simply not returned; with replies it is returned as a placeholder with `deleted: true` and the browser writes "Comment deleted". A removed comment (moderator) is treated the same way for display, except no placeholder text beyond "Comment removed" is a decision for the plan.
- Usernames are read by join at list time, never copied, so a Settings change shows on old comments at once (sketch 008, eleventh round).

**Who may comment.** Any signed in account, from anywhere: SC3 says "a signed-in user", the 1 km rule applies to votes only, and a text box needs no location. Record the trade: a remote person can post a rumour as a comment, which is why comments never feed any trust number, are screened, rate limited, flaggable, and the page keeps the trust block above them. Open Question 5 offers the stricter variant (only people who could vote), not recommended.

**Text rules.** Server authoritative, in `comments.go`: trim; reject empty; reject over 280 Unicode code points (`utf8.RuneCountInString`); reject ASCII control characters other than newline and any `unicode.Cf` format character (zero width space U+200B, joiners U+200C and U+200D, the byte order mark U+FEFF, the bidi overrides U+202A to U+202E and isolates U+2066 to U+2069), so nobody hides a spoofed `@ndma` or reverses text; collapse runs of more than two newlines; allow emoji and Indic scripts. The browser's `maxlength` counts UTF-16 units and `String.length` does too, so the "N left" counter must use `Array.from(text).length` to match the server (the same note the Flag note box carries).

**Screening (D-12) and what Phase 4 gives.**
- Reusable, unchanged: `ScreenAbuse(ctx, screener, text)` (one 3 s bounded call, never repeats, an error answers `TextUnfinished` with the provider's `RetryAfter`), `Decide(scores, false)` (reject groups hateful, harassing, sexual; borderline), `RejectMessage(reason)`. The screener is `ReportService.Screener()` (nil means pass everything in tests and development).
- **Use `ScreenAbuse`, not `CheckText`.** `IsGibberish` returns true when the text has fewer than two words of two or more letters, so "Thanks", "ok" and a lone emoji would be refused. `@ravi.k yes` passes only because "ravi" and "yes" count as two words. Optionally keep the repeated letter run test (`GibberishRunLength`) as a cheap spam rule by calling it separately.
- **Outcomes:** `TextRejected` returns 422 with `RejectMessage`, nothing stored, the box keeps its text. `TextBorderline` stores `moderation_state='clear'`, `needs_review=true` (posts, listed for moderators). `TextClear` stores clear. `TextUnfinished` stores `pending` with `next_attempt_at = now()`, the author sees their own comment marked "Waiting for a check" (the author needs to see it exist), and nobody else sees it until the check finishes. Comments have no critical exemption (D-12), unlike reports.
- **Held state is not reusable (important).** `moderation_checks.report_id` is the primary key and a foreign key to `reports`, `ClaimDueModerationChecks` joins `reports.description`, `FinishCheckAndSetState` writes `reports.moderation_state`, `LockModerationCheck`, `KeepReport`, `RemoveReport`, `ModerationStatusForReport`, `report_flags` and `moderation_actions` are all report keyed. Do not generalise them (three race tests pin the lock order). Build `CommentWorker` as a sibling of `ModerationWorker`: same constants (`ModerationBackoff`, 30 s sweep, batch 5, lease 60 s, 8 s call timeout), claim by `SKIP LOCKED` on `comments`, one screener call per row per sweep, classify errors the same three ways (transient, quota, permanent), stop retrying when the report has expired. The worker needs no report description.
- **What Phase 7.1 can share:** `ScreenAbuse`/`CheckText`, `Decide`, `RejectMessage`, the `Deps` plumbing for the screener, and a reserved word list. Usernames do not need a held state: if the check is unfinished, refuse the save with a "try again" line. Phase 7.1 should not build `comment_*` anything, and Phase 8 should not wait for 7.1 to build a held state.

**Deletion and visibility rules (D-14, D-15).**
- Delete: own comment, any time, even after the report closes; no editing endpoint exists. The reporter has no delete power over others' comments (`can_delete` is true only for the author).
- Flag: any signed in account except the author; one flag per account per comment; a flag never hides. Reuse Phase 4's control and `FlagReasons`; the dialog is report specific and needs a mode. Today `flag_dialog.html.tmpl` says "Why are you flagging this report?", has the line "If the report is just wrong, use Dispute instead." and the reason "False or a hoax"; `flag.js` keys on `data-flag-report-id` and posts to the report path. Add a target kind to the dialog: title "Why are you flagging this comment?", hide the Dispute line, keep the same reasons (a comment can be spam, abusive or dangerous; "False or a hoax" can stay). Update `flag_dialog_test.mjs` and `flag_contract_test.go`.
- Closed: `comments_open` is server written from the report's view (`active` or `disputed` and not expired and not withheld); post, reply and like return 409 with a stable field when closed; delete and flag stay allowed (safety). The line "Comments are closed" is quiet text. Existing comments remain readable. A finished Rescue report (stored rows only) should hide its comments entirely, because 03.1 D-23 removes its text; state this as an assumption.
- Moderator screen (`/moderate`): add a Comments section with three reasons per row (flagged by people, near the line, check not finished) showing the comment text, the report's category and id, and Keep and Remove buttons; new endpoints `GET /api/moderation/comments`, `POST /api/moderation/comments/{id}/keep` and `/remove`, registered with `r.Handle` inside the existing `requireModerator` group so a non moderator gets the byte equal 404. Keep clears `needs_review`, dismisses open flags and releases a pending comment; Remove sets `moderation_state='removed'`. Write the audit row in the same transaction. Render with `textContent` in `moderate.js`.

**Likes.** Thumbs up is a registered SVG mask icon, not an emoji (`TestNewIconsAreRegisteredAndUsed`, `TestPhase33IconsRegisteredAndUsed` show the registration pattern in `css_contract_test.go`; the repo bans emoji icons). Filled when liked, grey otherwise, never a severity colour, 44px target (`TestNewControlsMeetTouchTarget`). Add a Go test, in the style of `TestTrustModuleHasNoThresholdsOrSinks`, that no file in `internal/service` that builds trust, currency, reliability, visibility or confirmed membership references `comment_likes` or the comments tables, and that the likes SQL file is not referenced by `CurrentVotesForReports` readers.

**Reporter tag and linkability.** Showing "Reporter" beside a public username ties that username to a report, the linkability concern in ROADMAP Phase 9 scope note 1. It is locked (D-02), so record it as an accepted risk in the plan and in `docs/TRUST-MODEL.md`.

**Account deletion.** `AccountDeletionService.DeleteAccount` (Option A, anonymise) must be extended, inside its transaction, to blank the account's comment bodies and mark them deleted, delete the likes the account gave, delete its comment flags, and rely on Phase 7.1 clearing or freeing the username so the display reads "Deleted account" if a join still finds the row. Add the new queries to `AccountDeleteQuerier` and a test next to `accountdelete_test.go`. Add the new tables to the explicit list in `testutil.Truncate`.

### 6. Notifications (D-16)

**What exists.** 3.2 `updates` rows hold only a kind and a report id. The sentence is written at read time (`UpdatesService.List` calls `EventLine(kind)` and `UpdateTrustLine`). Rows are written by `Refresh` (state diffs, with the 1 hour per report gap `UpdateReportGap` and the 10 per hour ceiling `UpdatesHourlyCeiling` with a rolled up "more" row) and by `InsertReliableAlert` (a direct write, unique once per account and report). `updates_kind_valid` (migration 00009) is a closed `CHECK` list and `updates_shape` demands `report_id NOT NULL` for every non `more` kind. The `UpdateKinds`, `UpdateEventKinds` lists and their tests are closed sets.

**What to add.**
1. A migration extending `updates_kind_valid` with two kinds, for example `comment_on_yours` and `reply_to_yours`, and the Go constants, `UpdateKinds`, `UpdateEventKinds`, `Valid`, `IsEvent`, `EventLine` ("New comment on your report", "New reply to your comment") and a test update. `UpdateTrustLine` needs a case (return the confirmed or lead sentence like `UpdateResolved`, or an empty string; the doc comment on `UpdateTrustLine` says "for every event kind the line is non empty" and its tests likely pin it, so pick one deliberately and check). The wire enumeration in `handlers/updates.go` swag comments and `docs/` change. `updates.js` only special cases `more`, so new kinds flow through. Rows carry no comment text and no username; the lock screen rule (03.3 D-14) forbids the reporter's words and a bell row follows the same no free text rule (D-15 of 3.2).
2. **Write time insert,** like `InsertReliableAlert`, not a cursor transition: on a successful post, insert one row for each recipient. A top level comment notifies the report's author (account behind `reports.session_id`; a report with no account notifies nobody); a reply notifies the parent comment's author; neither notifies the commenter themself; a recipient who muted the report gets nothing (check `report_mutes`); a deleted account gets nothing. A partial unique index `(account_id, report_id, kind) WHERE read_at IS NULL` plus `ON CONFLICT DO NOTHING` keeps 30 comments from becoming 30 bell rows. Decide in the plan whether these rows count toward the hourly ceiling; recommended: they do not roll up into "more" but are capped by the unique unread index.
3. **Follow set (PAGE-02 says commenting makes the user follow the report).** `FollowedOpenReports` (updates.sql) unions "reports I posted" and "reports I have a vote row on"; add a third arm for reports the account commented on. `FollowersWithChangesPush` (push.sql) has the same two arms; add the third. Without this the sentence in the requirement is false.
4. **Phone alerts.** D-16 says use the existing Changes kind. `EnqueueChangedFromUpdates` hard codes `u.kind IN ('reached_live','hidden','resolved','fading')`; add the two comment kinds. `changeEventWords` (pushcopy.go line 291) needs "has a new comment" and "has a new reply" style words, and `PushWorker.eventStillHolds` (pushworker.go line 479) needs cases that return true while the comment still exists and the report is not withheld. The comment text and the username must not appear in the push (03.3 D-14, `PushTitleMaxRunes` 64). The dispatcher is poked after a post with `ReportChanged(reportID, actorAccountID)` (plan 11 pattern); `changesBranch` refreshes every follower, which is more work than a single recipient needs, so either accept it or add a narrow `RecipientChanged(accountID)` poke. Likes call neither path and need a test that proves no `updates` row and no `push_outbox` row appear.
5. **Silent.** All phone notifications are already silent. Do not write any copy near these features that mentions sound or vibration.
6. **Settings text.** The switch reads "Changes to reports I posted or voted on"; once comments count, it should read "Changes and comments on reports I posted, voted on or commented on". It is in `settings.html.tmpl` and pinned by `settings_rows_contract_test.go` and `noise_words_contract_test.go`; a copy change needs the owner's eye (Phase 3.3 plan 14 is still being built; coordinate).

### 7. Rate limits (D-13)

Phase 4's gate is `handlers.RateGate` over `ratelimit.PerKey`, keyed `acct:<id>`, plus a network backstop keyed on the client IP only when `CLIENT_IP_HEADER` names a trusted header. A new bucket must be added in four places or the config silently falls back: the `RateLimits` struct and `NewRateGate` in `ratelimit_gate.go`; the constants block in `cmd/server/main.go` and its wiring in `Deps.RateLimits`; `api.RateLimitsDefault`; and `resolveRateLimits` in `router.go`. Every existing e2e tweak (`p20Tweak` and its siblings) opens all buckets with `p20Open`, so add the new fields to those helpers or frozen clock tests fail; and check `internal/api/ip_backstop_test.go` for a test that pins main.go against the defaults.

Proposed numbers, with the same "100 phones behind one carrier address" backstop rule main.go documents:

| Bucket | Applies to | Per account | Network backstop | Reasoning |
|--------|-----------|-------------|------------------|-----------|
| `Comment` | post comment and reply (one bucket; every attempt spends, no refund, because each attempt costs a screener call) | burst 6, one more every 20 s | burst 600, one every 200 ms | A real thread is a few lines in a minute; sustained 3 per minute is generous and a spammer is slowed. The screener call is the scarce resource, as for reports (`SubmitAttempt`). |
| `CommentAction` | like, unlike, delete | burst 30, one more every 3 s | burst 3000, one every 30 ms | Cheap writes, toggles; mirrors `Vote` (20 per 15 s) with a little more room. |
| flags of comments | reuse the existing `Flag` bucket (burst 10, 2 min) | shared | reuse `IPBackstopFlag` | Same abuse concern; one flag per account per comment is also a unique key. |
| public page | `GET /r/{token}` | n/a | per IP burst 120, one per second | Loose on purpose (see Share). |

Messages (in `ratecopy.go`, built with `WaitPhrase`): "You're commenting very quickly. You can comment again in {wait}." with 429 field `rate_comment`; "You're reacting very quickly. You can try again in {wait}." field `rate_like`; the existing `NetworkLimitMessage` for the backstop. The browser keeps the typed text on a 429 (the report form's rule). Add a per account per report cap (for example 60 comments) only if the planner wants a hard stop; not required.

### 8. OpenAPI and contract tests

**Swagger.** Annotate each new `/api/*` handler (`@Router /reports/{id}/timeline [get]`, comments, likes, flags, share, moderation) in swag style, add types to the generated model section, and run `make swag` (`swag init -g internal/api/router.go -o docs`); the local binary is v1.16.4 while CLAUDE.md lists v1.16.6, so regenerate with one version and commit the whole `docs/` diff. `TestSwaggerSpecCoversRoutes` (internal/api/handlers/swagger_test.go) asserts specific paths, 401/403/409/429 responses and parameters; extend it with the new routes, each declaring 401, 404, 409 and 429 where they apply. The HTML pages (`/r/{token}`, `/login`, `/disclaimer`) are not in the spec today; leave `/r/{token}` out and say so. `TestTrustResponseOmitsIdentifiers` style tests must be repeated for the timeline and comment responses (no `account_id`, `email`, `session`, `geohash`, `cell` keys).

**Contract tests that new templates, JS and CSS must satisfy** (all under `web/` unless noted):
- `js_sinks_contract_test.go` `TestNoMarkupSinksAnywhere`: no `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write` in any `static/js` file, comments stripped first. It also fails if its comment scanner ends inside an unterminated string, so keep regular expressions that hold quote characters out of the new files or write them with a character class.
- `design_rules_contract_test.go`: `TestUserVisibleCopyUsesPlainPunctuation` scans every embedded template (new ones included automatically) for em and en dashes; `TestUserVisibleCopyUsesPlainPunctuationAuthJS`; `TestNoPillShapedControls` (the composer's rounded field must not be a 999px pill button; the Post button inside it is the risk, check the rule).
- `layout_rules_contract_test.go`: `TestNewControlsMeetTouchTarget` (every control named in `touchControls` needs 44px; add the new controls: Reply, thumbs up, View replies, Post, Share, Copy link), `TestNoOpacityText`, `TestNoDarkBorderOnChosenItems`, `TestSizeOnlyElementsAreNamed` (any element hidden on one size needs an entry with a reason; the new laptop docked rules will need entries, and the existing `.report-view__title` and `.report-gone` entries quote the owner's 2026-10-19 sentence, which a B build must amend), `TestNewScriptsUsePlainPunctuation` (add the new script names to `phaseScripts` or rely on the template scan; comments count).
- `no_underline_contract_test.go`: no underline at rest; actions such as Reply, View replies, Delete, Copy link are buttons, not links.
- `css_contract_test.go`: `TestModalBackdropHiddenGuard` and the `[hidden]` guard (any `display` on `.modal-backdrop`, the share window or the composer needs `:not([hidden])`), `TestPhase34DesignTokensDeclaredOnce` (do not redeclare tokens), the icon registration tests (thumbs up, share, speech bubble masks).
- `popup_contract_test.go` `TestPopupCardSurfaceAndGeometry`: the footer ban, to be amended with a dated note (see question 2). Other tests in that file pin popup order and geometry; the page sections go after the existing card children so none of them moves.
- `phone_layout_contract_test.go`, `sheet_contract_test.go`, `view_strip_contract_test.go`, `report_view_contract_test.go` (changed in plan 39): they pin the detail pane's geometry; the laptop host plan must amend them with dated notes, and `report_view_test.mjs` slices map.js by section markers.
- `noise_words_contract_test.go`: bans buzz, vibrate, sound, ring, silent, quiet, hear, and variants in the files it lists (Settings, welcome, notify, sw). The word "quiet" is also used informally in these docs for the Flag control; it is only banned in the listed files, but do not put it in `settings.html.tmpl`.
- `flag_contract_test.go`, `flag_dialog_test.mjs`, `moderate_contract_test.go`, `activity_moderation_contract_test.go`: touched by the flag and moderator changes.
- `template_contract_test.go` `TestVendorMapScriptsLoadInDependencyOrder`: the script order in `index.html.tmpl`; new scripts load after `trust.js` and `votes.js` and before `map.js` if `map.js` calls them at build time, so declare the order and update this test.
- `internal/api/gate_test.go`, `internal/api/pwa_routes_test.go`: the exempt route and the no session proof.
- `internal/service/trust_model_doc_test.go` `TestTrustModelDocMatchesCode`: `docs/TRUST-MODEL.md` must stay true and must cite named tests. Add a section on the timeline, comments and likes ("comments and likes never feed any trust number") and add the new test names to `p20MustCite` only if the doc cites them.

### 9. Phase 7.1 outputs this phase depends on (name them so the planner can check)

7.1 has no plans yet. Phase 8 needs exactly these, and the planner should write them as preconditions with a re-grep step like Phase 5 plan 05-04 does:
1. `accounts.username`: unique, case insensitive, public, non null for every account that can reach the app (7.1's one time fill-in guarantees this at sign in).
2. A service call to read usernames for a batch of account ids (comment lists are batched), and one to read the viewer's own.
3. Account deletion clears or frees the username (so "Deleted account" can be shown).
4. The reserved word list (police, ndma, imd, govt, official, pinalert and similar) as a reusable function; Phase 8 uses it only for `@mention` highlighting rules (do not highlight a reserved name as a person), not for validation.
5. `requireOnboarded` still passes `/api/` and treats a signed in account with a username as onboarded, so comment routes are reachable.
6. The gate's exempt list after 7.1 (new sign up, code and reset screens), onto which `/r/{token}` is added.
7. Optional: a safe same origin return path (an opaque share token, never a URL) so "Sign in to see more" can land on the report; otherwise the button is a plain `/login` link.
Everything else in 7.1 (passwords, codes, step 4 UI) is out of scope here.

### 10. Suggested plans and waves

Ship order from D-17: page, timeline, Share, then comments. Several plans regenerate `internal/store/sqlc/*.go` and edit `router.go`; plans that add queries or routes in the same wave will conflict on generated files, so either serialise them or make the merge step re-run `sqlc generate`. Migrations take "the next free number at execution time" and never a hard coded one. Each plan gets its own scratch database (owner recipe).

| Wave | Plan | Content | Depends on |
|------|------|---------|------------|
| 1 | 08-01 | Timeline backend: `VoteLogForReport`, `BuildTimeline`, `timelinecopy.go`, `GET /api/reports/{id}/timeline`, swag, tests. | none |
| 1 | 08-02 | Share backend: migration (`reports.share_token`), `ShareService` (token, area name, public status), `POST /api/reports/{id}/share`, `GET /r/{token}` page and template, `Deps.PublicBaseURL`, exempt route and session bypass, per IP limit, gate and no session tests. | Phase 5 `sessionSkip` (or the fallback); runs after 08-01 if both regenerate sqlc |
| 2 | 08-03 | Page body client: `report-page.js` builder (timeline section, action row with Share, comment mount point), mounted in the phone detail pane; CSS; node tests; contract test amendments (popup footer ban lifted and replaced, size-only entries). | 08-01 |
| 2 | 08-04 | Share window and sheet (client), navigator.share, Copy link, preview card; node and contract tests. | 08-02, 08-03 |
| 3 | 08-05 | Laptop host (docked panel via the detail pane). A checkpoint: show the owner a test copy first; if the owner keeps the popup, the fallback is a wider popup with the same sections (Open Question 1). | 08-03, owner's look |
| 3 | 08-06 | Comments data and service: migration, queries, `CommentService`, `ScreenAbuse` hook, text rules, delete, likes, closed state, follow arms in `FollowedOpenReports` and `FollowersWithChangesPush`, `comment_count` in the extras read, account deletion hooks, `testutil.Truncate`. | Phase 7.1 outputs |
| 4 | 08-07 | Comments HTTP: handlers, `RateLimits` in all four places, messages, swag, e2e tests with the p20 style helpers. | 08-06 |
| 4 | 08-08 | Comment worker (held checks) and the moderator screen section. | 08-06 |
| 5 | 08-09 | Comments client: list, folded replies, composer and counters, thumbs up, flag dialog mode, popup footer count, CSS, icons, node and contract tests. | 08-03, 08-07 |
| 5 | 08-10 | Notifications: migration for the update kinds, write time insert, push cases, poke, copy, Settings text, tests (including likes produce nothing). | 08-06, Phase 3.3 plan 14 merged |
| 6 | 08-11 | Docs and proof: `docs/TRUST-MODEL.md` and `MODERATION.md`, README, swagger regeneration, full suite, a Linux CI check recipe run, viewing session with the owner (phone simulator and laptop), STATE and ROADMAP edits by hand. | all |

If time runs short, the ROADMAP cut order says the comments half of Phase 8 is the next thing to drop after 3.2's tail; waves 1 to 3 alone satisfy PAGE-01 and COORD-08.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Distinct place counts in the timeline | A separate counter over the log | `BuildVoteTally` on a standing row prefix | It already excludes the reporter, dedups by account and counts distinct cells; a second count would disagree with the trust block. |
| Resolved by whom and when | New timestamps or a resolution table | `ClosureOf`, `ClosedByFor`, `closedText` | The closure moment and actor are already derived from the same rows. |
| Text screening | A new classifier or word list | `ScreenAbuse`, `Decide`, `RejectMessage` | One policy for reports, flag notes, comments and usernames. |
| Held check retry | A new queue or cron | A `CommentWorker` copy of `ModerationWorker` (same claim, backoff, lease) | The pattern is race tested; only the table differs. |
| Rate limiting | A new limiter | `ratelimit.PerKey` through `RateGate`, `ratelimit.NewPerIP` for the public page | Includes clocks for tests, refund rules and the backstop. |
| Absolute URLs and tokens | Host header URLs; sequential ids in public links; an HMAC with the session secret | `Deps.PublicBaseURL` from `BASE_URL`; `crypto/rand` token column | Host poisoning and link death on secret rotation. |
| Relative times | Strings with "5 minutes ago" from the server for the app page | RFC 3339 plus `Pinalert.relativeTime` | Pitfall 8: equal reads must produce equal strings. (The public page is server rendered with no script and no equality contract.) |
| Share mechanics | A custom social share SDK | `navigator.share` when present, a Copy link button, optional `wa.me` link | Nothing to install, no tracking. |
| Dialog behaviour | A new modal system | The existing `.modal-backdrop` pattern, `window.css`, the `.logout-confirm` conventions | The owner's rules about alert boxes are already encoded there. |

**Key insight:** every hard part of this phase (independence counting, screening, rate limiting, silent notifications, privacy rules) is already solved once in this repo. The risk is not missing machinery but quietly re-deriving it in a second place that then disagrees with the first.

## Common Pitfalls

### Pitfall 1: Sequential ids in a public link
**What goes wrong:** `/r/{id}` becomes a no sign in mirror of the whole feed.
**Why it happens:** `reports.id` is `BIGSERIAL` and the page is meant to be reachable without an account.
**How to avoid:** the stored random `share_token`; the route never accepts a numeric id.
**Warning signs:** a test that fetches `/r/1` and gets 200.

### Pitfall 2: The preview crawler writes a session row and a cookie
**What goes wrong:** every WhatsApp preview fetch inserts a `sessions` row and sets `pinalert_session`; a shared cache can then hand one visitor's cookie to another (the reason `legalCacheControl` is `private`).
**How to avoid:** add `/r` to the session bypass prefix list; assert no `Set-Cookie` and an unchanged `sessions` count.

### Pitfall 3: `skipSessionFor` is exact path only today
**What goes wrong:** adding `/r/{token}` to a map of exact paths does nothing for a parametric route.
**How to avoid:** depend on Phase 5's `sessionSkip` prefix support or add the same shape; keep the near miss rows (`//`, dot segments, encoded paths, other methods).

### Pitfall 4: A tight per IP limit blinds the preview
**What goes wrong:** Meta's crawler addresses are shared by every WhatsApp user, and without `CLIENT_IP_HEADER` every visitor shares one peer address; a tight limit makes previews fail for everyone.
**How to avoid:** loose numbers (burst 120, 1 per second) and a comment that says why.

### Pitfall 5: `CheckText` refuses short replies
**What goes wrong:** "Thanks", "ok" or an emoji reply is rejected as gibberish.
**How to avoid:** `ScreenAbuse`; add table tests with short Hindi, Telugu and English replies and a lone emoji.

### Pitfall 5b: `arrangeCard` re-appends Flag and Stop updates after the page node
**What goes wrong:** the page sections end up above Flag and Stop updates on the phone after the next placement.
**Why it happens:** `arrangeCard` appends them to `body` on every call.
**How to avoid:** insert them before `card.page.el`; node test of the child order across a resize (see Pattern 1).

### Pitfall 6: Rebuilding the page on every poll
**What goes wrong:** an open reply thread, a liked state or half typed text vanishes when the 30 second poll or a vote calls `updatePopupCard`.
**How to avoid:** the page sections are separate nodes with their own `update`; a node test types into the composer, fires an update and asserts the value, the open thread and the focus survive.

### Pitfall 7: UTF-16 versus code points
**What goes wrong:** the counter, the `maxlength` and the server disagree for emoji and Indic text.
**How to avoid:** server counts runes; the client counter uses `Array.from`; a Go and a node test use a 4 byte emoji and a Telugu word.

### Pitfall 8: `navigator.share` after an await
**What goes wrong:** Safari throws `NotAllowedError` because the user gesture was spent by the fetch.
**How to avoid:** open the window on tap, fetch the link while it is open, call `navigator.share` synchronously from the Share button inside the window.

### Pitfall 9: Typing inside a Leaflet popup
**What goes wrong (laptop popup variant only):** if the field lives inside the map container, Leaflet's keyboard handler or `onMapKeydown` in map.js (Escape closes the popup) can act on keys typed in the box.
**How to avoid:** the detail pane host is outside the map container, which is why the docked panel via `detailPane` is the recommended B mechanism; if a wider popup is used instead, stop `keydown` propagation on the composer and test arrow keys, plus and minus.

### Pitfall 10: A new bucket added in three of four places
**What goes wrong:** the default fills a zero field silently or a frozen clock test finds an unopened bucket.
**How to avoid:** the four place checklist in "Rate limits", and a test that `RateLimitsDefault` equals the main.go constants.

### Pitfall 11: Generated code merge conflicts
**What goes wrong:** two plans in one wave both regenerate `internal/store/sqlc/*.go`.
**How to avoid:** serialise DB plans, or re-run `sqlc generate` at merge. CI needs no sqlc binary because the output is committed.

### Pitfall 12: The follow set stays unchanged
**What goes wrong:** PAGE-02 says commenting makes the user follow the report; with the two arms of `FollowedOpenReports` and `FollowersWithChangesPush` untouched, a commenter hears nothing about state changes.
**How to avoid:** add the third arm to both and test it.

### Pitfall 13: `docs/TRUST-MODEL.md` is test guarded
**What goes wrong:** adding or removing a cited test name breaks `TestTrustModelDocMatchesCode`.
**How to avoid:** edit the doc and `p20MustCite` together.

## Code Examples

### Timeline replay skeleton (pure, no I/O)
```go
// Source: modelled on internal/service/history.go ClosureOf; names are proposals
type VoteLogRow struct {
    ID        int64
    AccountID int64
    Kind      VoteKind
    Value     VoteValue
    Cell      string // service internal, never serialised
    At        time.Time
}

type TimelineEvent struct {
    Kind string    // posted, confirmed, disputed, live, hidden, resolved, reopened, expired, yours
    Tone string    // good, neutral, bad, as FactResponse
    Text string    // from timelinecopy.go only
    At   time.Time
    Yours bool
}

func BuildTimeline(meta ReportMeta, reportID int64, reporter *int64, createdAt, expiresAt, now time.Time,
    log []VoteLogRow, viewer int64) []TimelineEvent
```

### Public page template fragment (escaping is automatic)
```html
{{/* Source: html/template contextual escaping; area and title come from fixed lists and a geocoder */}}
<meta property="og:title" content="{{.OGTitle}}">
<meta property="og:description" content="Open to see if this is still true right now">
<meta property="og:url" content="{{.CanonicalURL}}">
<meta name="robots" content="noindex, nofollow">
```

### Text rules (server)
```go
// Source: new, uses stdlib only
func cleanCommentBody(s string) (string, error) {
    s = strings.TrimSpace(s)
    if s == "" { return "", ErrCommentEmpty }
    for _, r := range s {
        if r == '\n' { continue }
        if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) { return "", ErrCommentChars }
    }
    if utf8.RuneCountInString(s) > CommentMaxRunes { return "", ErrCommentTooLong }
    return s, nil
}
```

### Rendering a leading mention without a markup sink
```javascript
// Source: pattern from flag.js / Pinalert.setText; no innerHTML anywhere
function writeCommentText(el, text) {
  var m = /^@([A-Za-z0-9._]{3,20})\s/.exec(text);
  if (!m) { Pinalert.setText(el, text); return; }
  var strong = document.createElement('strong');
  Pinalert.setText(strong, '@' + m[1]);
  el.appendChild(strong);
  el.appendChild(document.createTextNode(text.slice(m[0].length - 1)));
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Link preview image required | `og:image` optional; head within first 300 KB; image under 600 KB, at least 300 px wide, ratio at most 4:1 | Meta's current WhatsApp doc | A text only preview is allowed; the app mark can be used safely. [CITED: developers.facebook.com/docs/whatsapp/link-previews] |
| Report view on the laptop as popup over the map | Sketch 008 B (beside the map) locked 2026-10-17; laptop popup kept 2026-10-19 | 3.4 plan 39 | Open Question 1. |
| Held check keyed on reports | Held check on comments needs its own columns | this phase | Do not generalise Phase 4's tables. |

**Deprecated/outdated:** `check_inbox`/`verify_outcome` link pages (Phase 7.1 removes them); the "popup has no footer" rule A11 (this phase amends it).

## Runtime State Inventory

Not a rename, refactor or migration phase. Nothing stored under an old name changes. The only existing stored data touched is additive: `reports.share_token` (nullable, filled lazily), `updates_kind_valid` widened, and the follow arms. Existing reports have no share token until someone taps Share; existing `updates` rows are unaffected.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The laptop docked panel can reuse the Leaflet detail pane with a CSS only geometry change and a split of `phoneDetail()` | 1 | The change in `map.js` is larger than described; the plan 08-05 estimate grows. Verify in a spike before committing the plan. |
| A2 | `wa.me/?text=` opens WhatsApp's chooser with no number | 4 | Optional button misbehaves; drop it. Single community source only. |
| A3 | `/icons/app-512.png` is under 600 KB as served | 4 | Preview drops to small; measure at build time and omit `og:image` if not. |
| A4 | The limiter numbers (6 per 20 s, 30 per 3 s, public page 120 and 1 per second) fit real use | 7 | Too tight blocks a lively thread; too loose invites spam. Tune at the owner's viewing; all are constants in one block. |
| A5 | Comments from anywhere (no 1 km rule) is acceptable | 5 | The owner may want only nearby people to comment; changes the service and the closed state copy. Open Question 5. |
| A6 | A finished Rescue report should hide its comments | 5 | Minor: comments shown on a blurred report. Stored rows only, since Rescue is retired for new posts. |
| A7 | Upward milestones only is the right timeline density | 3 | The owner may prefer every step; the replay supports either, only the emit rule changes. |
| A8 | `Phase 5` will land the prefix aware `sessionSkip` as drafted | 4 | Fallback in the plan (extend the map with a prefix list). |
| A9 | The Settings switch wording should widen to include comments | 6 | Owner may keep the old words; cosmetic. |
| A10 | `EnqueueChangedFromUpdates` poke volume from `changesBranch` is acceptable | 6 | Extra refreshes per comment on a busy report; add the narrow poke if load shows. |

## Open Questions

1. **Laptop layout: B (beside the map) or the popup the owner asked not to disturb?**
   - What we know: D-01 and sketch 008 lock B (2026-10-17). On 2026-10-19 the owner said "the laptop ver looks good i dont want you to disturb that"; plan 03.4-39 dropped B and the sentence is quoted in `layout_rules_contract_test.go`.
   - What is unclear: whether that was a scope cut for 3.4 or a reversal of B.
   - Recommendation: build the page sections mount agnostic, give the laptop host its own plan (08-05) behind an owner viewing on a test copy (owner memory: judge layout by looking, batch the question). Show B and the fallback (a wider popup with the same sections). Do not ask in words.
1b. **A second later owner statement against the lock: the order of fade line, actions and trust block.** D-01 orders the page as the report, the trust block, then the actions. The shipped phone view deliberately puts the fade line and the actions above the trust block (owner, 2026-10-21, "make sure this confirm and dispute buttons are more visible", recorded in the `arrangeCard` comment in map.js, after the 2026-10-17 lock), and puts Stop updates and Flag below the trust details. Recommendation: keep the shipped phone order, record it in the plans as a deviation from D-01, and do not "fix" it back (owner rule: fix only what is asked). The laptop popup keeps trust, fade, actions as built.
2. **Where does SC4's footer live if B wins?** With no popup bubble on the laptop and the phone page being the card itself, the footer count shows as the `Comments N` heading; should list rows also say "N comments"? Recommend heading only unless the owner asks.
3. **Removed report wording.** D-06 says "This report was removed"; `Focus` hides moderation state by design. Recommendation: only `removed` says so; held, unknown and too old share one line. Confirm at the viewing.
4. **Return to the report after "Sign in to see more".** A plain `/login` link loses the report. A safe return needs Phase 7.1's login to accept an opaque share token. Ask 7.1's planner; default to plain `/login` for v1.
5. **Who may comment.** Anyone signed in (recommended, matches SC3) or only people within the vote radius (stricter, needs location for a text box).
6. **Low data view.** `/low` has report rows and a Flag control but no report page; comments, timeline and Share are not planned there. Recommend out of scope and say so in the plan.
7. **Moderation wording for removed comments.** "Comment deleted" is fixed by D-14 for own deletion; a moderator removal could read "Comment removed". Confirm.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build and tests | yes | go1.26.4 (go.mod says `go 1.26.4`; CI uses 1.25 setup) | none needed |
| Node | `node --test web/*.mjs` | yes | v26.4.0 (CI runs 22 or 24) | none needed |
| sqlc | regenerate queries | yes | v1.31.1 | none |
| goose | migrations | yes | v3.27.2 (local binary; tests apply embedded migrations themselves) | none |
| swag | regenerate `docs/` | yes | v1.16.4 | none; commit one version's output |
| Postgres client tools | scratch DB for tests | `psql`, `pg_isready` present | not probed against any database | create `r08_scratch`, refuse if it exists, drop after; never `pinalert_test` or `pinalert` |
| Docker | Linux CI recipe check | yes | not probed | skip |
| OpenAI Moderation key | live screening | not needed for tests | n/a | `moderation.PassScreener` and stubs in tests |
| WhatsApp / iPhone simulator | preview and phone checks | not probed | n/a | Meta's debug flow: compose a message with the link and wait up to 10 s for the preview (Meta doc); iPhone simulator Safari for layout |

**Missing dependencies with no fallback:** none. This research ran no database command and no test.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) with a real Postgres for DB tests; Node `node:test` with `vm` sandboxes for browser scripts; Go contract tests that read the embedded `web/static` and `web/templates` trees |
| Config file | none; Postgres comes from `DATABASE_URL` (tests skip without it); migrations are applied by `testutil.NewTestDB` |
| Quick run command | `go test ./internal/service ./web -short` and `node --test web/<new>.mjs` |
| Full suite command | `make test` (`go test ./... -v -p 1`, scratch `DATABASE_URL` only) and `node --test web/*.mjs` |

### Phase Requirements to Test Map
| Req / SC | Behavior | Test Type | Automated Command | File Exists? |
|----------|----------|-----------|-------------------|-------------|
| PAGE-01, SC1 | A report opens to its page with the same trust block and a way back | node + contract | `node --test web/report_view_test.mjs web/report_page_test.mjs`; `go test ./web -run 'TestReportPage|TestPopupCardSurfaceAndGeometry'` | partial: report_view_test.mjs exists; report_page_test.mjs and TestReportPage* are Wave 0 |
| PAGE-01, SC1 | Trust block on the page is the popup's own nodes, not a copy | contract | `go test ./web -run TestReportPageReusesTrustBlock` | Wave 0 |
| PAGE-01, SC2 | Timeline replays the log, one event per milestone, no names or cells | unit | `go test ./internal/service -run 'TestBuildTimeline'` | Wave 0 |
| SC2 | Timeline wording is closed copy, no "place", plain punctuation | unit | `go test ./internal/service -run TestTimelineWording` | Wave 0 |
| SC2 | Timeline response omits account, session, cell keys; withheld report is 404 | e2e (DB) | `go test ./internal/api/handlers -run 'TestTimeline'` | Wave 0 |
| SC2 | Timeline counts equal the trust block counts at every step | unit | `go test ./internal/service -run TestTimelineMatchesTrust` | Wave 0 |
| COORD-08, SC5 | Share returns a token link; public page shows only allowed fields; no comments, names, description, coordinates | e2e (DB) | `go test ./internal/api/handlers -run 'TestSharePage'` | Wave 0 |
| SC5 | Public page needs no account: exempt row 200 and near misses not 200 | e2e (DB) | `go test ./internal/api -run TestAccessGateExemptRoutes` | exists, extend |
| SC5 | Public page sets no cookie and writes no sessions row | e2e (DB) | `go test ./internal/api -run TestSharePageSetsNoCookie` | Wave 0 (copy `TestPWAAssetsSetNoCookie`) |
| SC5 | Open Graph tags carry no count or "verified"; og:url absolute from config, not Host | unit/e2e | `go test ./internal/api/handlers -run TestShareOpenGraph` | Wave 0 |
| SC5 | Numeric id is not a valid public route; unknown token 404; removed shows removed; held shows unavailable | e2e (DB) | `go test ./internal/api/handlers -run TestSharePageStates` | Wave 0 |
| COORD-08 | Share window shows title + fixed line, no count; Copy link; share called from the second tap | node | `node --test web/share_window_test.mjs` | Wave 0 |
| PAGE-02, SC3 | Comment passes the check, borderline posts and is listed, rejected keeps the box, unfinished is held | unit + e2e | `go test ./internal/service -run TestComment`; `go test ./internal/api/handlers -run TestCommentScreening` | Wave 0 |
| SC3 | Short replies ("Thanks", "ok", lone emoji) are accepted (no gibberish rule) | unit | `go test ./internal/service -run TestCommentShortReplies` | Wave 0 |
| SC3 | 280 rune limit, control and format characters refused | unit | `go test ./internal/service -run TestCleanCommentBody` | Wave 0 |
| SC3 | Per account limits and messages for comment and like; 429 shape | e2e (DB) | `go test ./internal/api/handlers -run 'TestCommentRateLimit|TestLikeRateLimit'` | Wave 0 |
| SC3 | No markup sink anywhere in the new scripts | contract | `go test ./web -run TestNoMarkupSinksAnywhere` | exists, covers new files automatically |
| SC3 | Comment text never rendered through a sink; mention is a strong element | node | `node --test web/comments_test.mjs` | Wave 0 |
| PAGE-02, D-14 | Delete own; placeholder when replies exist; reporter cannot delete others; flag never hides | unit + e2e | `go test ./internal/service ./internal/api/handlers -run 'TestCommentDelete|TestCommentFlag'` | Wave 0 |
| D-15 | Closed after resolve or expiry; delete and flag still allowed | unit | `go test ./internal/service -run TestCommentsClosed` | Wave 0 |
| D-02 | One like per account; likes never touch trust, currency, reliability, visibility | unit | `go test ./internal/service -run 'TestLikeOnePerAccount|TestLikesNeverFeedTrust'` | Wave 0 |
| D-02 | Page state survives a vote/poll: open thread, liked state, composer text | node | `node --test web/report_page_test.mjs` | Wave 0 |
| D-12 | Comment worker retries, backoff, stops at report expiry, never double screens | unit | `go test ./internal/service -run TestCommentWorker` | Wave 0 (mirror `modworker_test.go`) |
| D-14 | Moderator Comments list, Keep and Remove, non moderator gets the byte equal 404 | e2e (DB) | `go test ./internal/api/handlers -run TestModerateComments` | Wave 0 |
| D-16 | Comment and reply write an update for the right recipient only; mute, self, deleted respected; likes write none | unit + e2e | `go test ./internal/service -run 'TestCommentUpdates|TestLikesNotify'` | Wave 0 |
| D-16 | Commenter is in the follow set (both queries) | e2e (DB) | `go test ./internal/service -run TestCommenterFollowsReport` | Wave 0 |
| D-16 | Push words for comment kinds carry no comment text or username; still holds check | unit | `go test ./internal/service -run 'TestPushCommentWords|TestEventStillHoldsComment'` | Wave 0 |
| SC4, D-03 | `comment_count` is a real count (visible comments only), absent before comments ship | unit + e2e | `go test ./internal/service -run TestCommentCountExtras` | Wave 0 |
| SC4 | Popup footer renders the server count and the zero and one forms | node + contract | `node --test web/popup_footer_test.mjs`; `go test ./web -run TestPopupFooter` | Wave 0 |
| API docs | New routes documented with 401/404/409/429 | e2e | `go test ./internal/api/handlers -run TestSwaggerSpecCoversRoutes` | exists, extend |
| Account deletion | Comments blanked, likes and flags removed | e2e (DB) | `go test ./internal/service -run TestDeleteAccountComments` | Wave 0 |
| Docs | TRUST-MODEL.md true | unit | `go test ./internal/service -run TestTrustModelDocMatchesCode` | exists, extend |

### Sampling Rate
- **Per task commit:** `go test ./internal/service ./web -short` plus the single `node --test` file the task touches.
- **Per wave merge:** `node --test web/*.mjs` and `go test ./... -p 1` against the plan's own scratch database (`r08_scratch` style, never `pinalert_test` or `pinalert`).
- **Phase gate:** full suite green on a fresh database, then the Linux CI check recipe from owner memory, then the owner's viewing (phone in the iPhone simulator, laptop in Mac Safari only, no iPad) before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/service/timeline_test.go`, `timelinecopy_test.go`: events, milestones, take backs, closed copy.
- [ ] `internal/service/comments_test.go`, `commentworker_test.go`, `commentupdates_test.go`, `commentlikes_test.go`.
- [ ] `internal/api/handlers/timeline_e2e_test.go`, `share_e2e_test.go`, `comments_e2e_test.go`, `moderate_comments_e2e_test.go`.
- [ ] `internal/api/share_routes_test.go`: no cookie, no sessions row, near misses; extend `gate_test.go`.
- [ ] `web/report_page_test.mjs`, `web/comments_test.mjs`, `web/share_window_test.mjs`, `web/popup_footer_test.mjs`.
- [ ] `web/report_page_contract_test.go`, `web/comments_contract_test.go`, `web/share_contract_test.go`; amended entries in `popup_contract_test.go`, `layout_rules_contract_test.go`, `flag_contract_test.go`, `template_contract_test.go`.
- [ ] A test helper to seed comments, likes and a vote log in `internal/testutil`.
- [ ] Framework install: none.

## Security Domain

`security_enforcement` is enabled in `.planning/config.json` (ASVS level 1, block on high).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no (7.1 owns it) | this phase consumes the gate; public page is the only exempt route added |
| V3 Session Management | yes | the public page and its crawler fetches create no session and no cookie |
| V4 Access Control | yes | gated routes by construction (new routes go inside the group); comment delete only by author; moderator routes inside `requireModerator` with the byte equal 404; share token as capability, not id |
| V5 Input Validation | yes | server side rune count, control and format character refusal, closed enums for flag reasons, numeric path ids parsed and bounded, token shape check before any query |
| V6 Cryptography | yes | `crypto/rand` for tokens, never `math/rand`; no hand rolled signing |
| V7 Error Handling and Logging | yes | logs carry ids and bucket names only, never comment text (T-04-04 pattern) |
| V8 Data Protection | yes | public page minimal fields; no cell, account id, email, exact location, description; comment text blanked on delete |
| V11 Business Logic | yes | rate limits, closed state, one like per account, one flag per account |
| V13 API | yes | swag documented; consistent error envelope; no account ids on the wire |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Enumerating the public page by id | Information disclosure | Random stored share token; numeric ids never routable on `/r/` |
| Stored XSS through comment text, usernames, area names | Tampering | `textContent` and `createElement` only (`TestNoMarkupSinksAnywhere`); `html/template` escaping in the public page, including attributes |
| Open Graph / Host header injection | Tampering | `og:url` and share URL from `Deps.PublicBaseURL`, never `Host` |
| Crawler writes sessions / cookie cache poisoning | Elevation / Info disclosure | Session bypass prefix, `no-store`, no `Set-Cookie` test |
| Impersonation by invisible or bidi characters | Spoofing | Refuse `unicode.Cf` and control characters; reserved usernames (7.1) |
| Abuse and spam in comments | Denial of service / Tampering | `ScreenAbuse`, per account bucket, IP backstop, flag and moderator Keep/Remove |
| Screener exhaustion | Denial of service | Every attempt spends the comment bucket, no refund; worker backoff; 3 s inline bound |
| CSRF on POST routes | Tampering | SameSite=Lax session cookie, JSON bodies (DEC-L); the public page has no state changing route |
| Linking a public username to a report | Information disclosure | Locked (D-02 Reporter tag); recorded as accepted risk; no profile before Phase 9 |
| Lock screen leak of comment text or names | Information disclosure | Push words fixed, no text, no username (03.3 D-14) |
| Timing and vote time exposure in the timeline | Information disclosure | Counts and times only; no cell, no account; milestone only; accepted residual |
| Open redirect via a return path after sign in | Tampering | If added, an opaque share token resolved server side, never a URL parameter |
| Moderation state disclosure on the public page | Information disclosure | One word for held, unknown and too old; only `removed` is named (recorded deviation) |

## Sources

### Primary (HIGH confidence)
- This repository, read 2026-10-25: `internal/api/router.go`, `gate.go`, `gate_test.go`, `pwa_routes_test.go`; `internal/service/trustcopy.go`, `trust.go`, `history.go`, `historyread.go`, `feed.go`, `textcheck.go`, `screening.go`, `modworker.go`, `moderation.go`, `flag.go`, `updates.go`, `updatesservice.go`, `updatescopy.go`, `pushkinds.go`, `pushdispatch.go`, `pushworker.go`, `pushcopy.go`, `accountdelete.go`; `internal/store/migrations/00001`, `00004`, `00007`, `00008`, `00009`, `00010`, `00011`; `internal/store/queries/votes.sql`, `moderation.sql`, `updates.sql`, `push.sql`, `reports.sql`; `internal/api/handlers/ratelimit_gate.go`, `reports.go`, `trust.go`; `cmd/server/main.go`; `web/static/js/map.js`, `app.js`, `flag.js`, `updates.js`; `web/static/css/phone.css`, `trust.css`; `web/templates/index.html.tmpl`, `moderate.html.tmpl`, `flag_dialog.html.tmpl`; `web/*_contract_test.go` named in the text; `docs/TRUST-MODEL.md`; the 03.4-39 SUMMARY; the 08, 07.1 and 03.3 CONTEXT files; the scratchpad seams file; sketches 008 and the sketch-findings skill; the 05-04 plan draft (read only).
- [Meta: WhatsApp link previews](https://developers.facebook.com/docs/whatsapp/link-previews): required tags, head within 300 KB, image under 600 KB at least 300 px, user agent, best effort. [CITED]
- [Leaflet 1.9.4 Map.Keyboard.js](https://raw.githubusercontent.com/Leaflet/Leaflet/v1.9.4/src/map/handler/Map.Keyboard.js): document level keydown, hooks only while the map has focus, no target check. [CITED]

### Secondary (MEDIUM confidence)
- Web search results on `wa.me/?text=` (vendor guides, not WhatsApp's own FAQ): the chooser form without a number. Treated as optional (A2).

### Tertiary (LOW confidence)
- Phase 7.1 outputs: inferred from `07.1-CONTEXT.md` and the ROADMAP section; no plans exist.
- Phase 5 `sessionSkip` shape: from an uncommitted draft plan under review by another session.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new packages; every unit named was read in the repo.
- Architecture: HIGH for server side and phone; MEDIUM for the laptop host mechanism (A1) and LOW for the final laptop layout until the owner looks.
- Pitfalls: HIGH, each traces to a file and line read this session.
- Limiter numbers and timeline density: MEDIUM, reasoned, tunable constants.

**Research date:** 2026-10-25
**Valid until:** 2026-11-08 for anything touching Phase 3.3 plans 14 and 15, Phase 5's `sessionSkip` and Phase 7.1 (all in flight); about 30 days for the rest.
