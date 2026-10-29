---
phase: 09
part: skeptic-1
scope: c2014ea9..79d434a2 (fix round 1 server, browser, templates; round 2)
checked_at: 2026-10-30
findings:
  critical: 0
  warning: 3
  info: 5
  total: 8
status: issues_found
---

# Phase 9 review fixes: skeptic 1

This was a read only pass. I ran `go build ./...` and `go vet ./internal/service/ ./internal/api/...`, and both are clean. I ran `node --test web/votes_stars_test.mjs` and it passed 31 of 31. I ran no go test, used no database, started no server and edited only this file.

## Confirmed issues

### SK-W01 (Warning): the hosted star box stays indefinitely above every window and the phone report card

**File:** `web/static/css/stars.css:181` (`z-index: 1190`) and `web/static/js/votes.js:2170-2179` (the `!live` mount)

**Scenario:**
- On a phone, the reader resolves report A and the card closes. The box mounts in `#rate-host`, above the bottom bar.
- The reader ignores it and opens report B. The phone detail pane is `z-index: 1080` (phone.css:1302). The box is 360px wide and sits on top of the bottom of B's card, where the vote buttons and Share are.
- The reader then opens Followers, the share sheet or any `.app-window` (window.css:26, `z-index: 1100`). The box still floats above the window's backdrop and panel and covers the bottom of the sheet.
- The only ways out are Skip, a rating, Undo, or a 404. Nothing else removes it. There is no timer before a rating, and no cleanup when another report or window opens.

This breaks the brief's rule "must not cover controls on the phone". It also goes past UI-SPEC 3b, which says the box "stays until the report view closes"; in the hosted case the report view has already closed.

**Smallest fix:**
- Option 1: drop every hosted box (`removeRateBox` for each key whose box sits in `#rate-host`) when a report opens or a window opens.
- Option 2: give the hosted box the toast's lifetime: leave when the resolve toast hides, keeping the existing 6 second line after a rating.
- In either case, put `#rate-host` under 1080 or inside the inert region, so a window always covers it.

### SK-W02 (Warning): a cursor from before a restart loops forever; the list never starts again

**Files:**
- `web/static/js/people-list.js:421-428` (`failLoad`)
- `web/static/js/people-list.js:720-747` (Show more)
- `internal/service/people.go:786-800`

**Scenario:**
- Render's free tier spins down after 15 idle minutes, so a restart while a list window is open is ordinary.
- After the restart, the cursor the reader holds fails the HMAC and the server answers 400 "That page could not be read."
- `failLoad(false)` shows Show more again with the same `next` and says "Couldn't load this list". The server's sentence is never shown.
- Every further tap sends the same dead cursor and fails the same way. The reader has to close and reopen the window.

This is not the calm handling the brief asked for. TRUST-MODEL.md (the Public lists paragraph) also says "a cursor from before a restart is refused and the reader starts the list again". No code starts it again.

**Smallest fix:** in the Show more handler, when `load(next)` answers 400, call `load('')` and then show the server's sentence in the window's message place. Or hide Show more and offer the list's Retry, which already calls `load('')`.

### SK-W03 (Warning): IN-E02 is only half fixed; the cursor still shows the exact follow time

**File:** `internal/service/people.go:775-780` (`encodePeopleCursor`)

**Issue:** IN-E02 has two parts:
- (a) `next` carries the exact follow time;
- (b) a hand made cursor can bisect follow times.

The fix signs the payload but leaves it as plain base64url of `"<micros>:<username>"`. Any reader of a public list with more than 50 named people can still base64 decode the part before the full stop, and read the microsecond time the 50th, 100th and later named person followed.

The cursor is also not tied to a list. One issued on list X is accepted on list Y. A reader can collect real timestamps from other lists (or from their own Following, paced by their own follows) and replay them on a target list to narrow down when its people followed. The bisection is coarser than before, but it is still possible. "No screen shows follow times" is still not true at the API.

**Smallest fix:**
- Seal the payload rather than only signing it: AES-GCM with the same per process key, the nonce prefixed, base64url out. The time then never leaves the server in clear.
- Put the owner id and the direction (followers or following) in the sealed payload, and refuse the cursor on any other list.

### SK-I01 (Info): WR-S04: a lost Name race still saves the username and the home area, then says try again

**File:** `internal/service/profile.go:520-545`; `internal/store/queries/accounts.sql:46-50`

**Issue:** when the Name loses the race, the username (written first) and the home area (same UPDATE) still land. The person then gets "try again". The query comment says "A Name write that loses this race changes nothing", and the G1 comment says "a refusal changes nothing". Both are inaccurate for those two fields. The window is tiny, and the Name itself is never published unscreened, so the target behaviour of WR-S04 holds.

**Fix:** reword the two comments, or use `WHERE ... AND (name_screened OR NOT profile_public OR NOT set_name)` so the whole row is skipped.

### SK-I02 (Info): the Visibility bucket takes a token for every on request, even when the profile is already on

**File:** `internal/api/handlers/people.go` (`UpdateVisibility`, `if *req.ProfilePublic && !gate.AdmitVisibilityOn(...)`)

**Issue:** MODERATION.md (the Visibility paragraph) and the server fix report both say it takes a token "only when the profile goes from off to on". The code charges any `{"profile_public":true}`, including a no-op while already on. That request never calls the screener, because `SetProfilePublic` returns early. This is harmless, but the doc does not match the code.

**Fix:** reword the doc to "for every request that turns it on (or asks for on)".

### SK-I03 (Info): the /moderate page does not check the moderator marker

**File:** `internal/api/handlers/moderation.go:35-47`

**Issue:** all 10 moderator API handlers go through `modGuard`, which checks `account.IsModerator`. `ModeratePage` checks only `account.FromContext`. It sits in the `requireModerator` group, so it is safe as wired. The report's line "Covers the Phase 4 and 8 moderator handlers too" is true of the API only.

**Fix:** add `if !account.IsModerator(r.Context()) { http.NotFound(w, r); return }` at the top of `ModeratePage`.

### SK-I04 (Info): a hosted star box can mount after the toast Undo took the resolve back

**File:** `web/static/js/votes.js:2208-2220` (`offerRateBox` waits for `PinalertChangesAsk.settled()`)

**Scenario:** the resolve closes the card, and the first post or vote pop-up is open. The toast is z 1200, above the pop-up's window, so its Undo can be tapped. If the reader taps Undo, `removeRateBox` finds nothing yet. When the pop-up is answered, `mountRateBox` mounts a box for a resolve that no longer stands. This was possible before for a live card; the host makes it reachable for a closed one.

**Fix:** after the wait, mount only if the resolve still stands. One way is to set `rateSkipped[key] = true` in both take back paths.

### SK-I05 (Info): the K3 skip can hold back the confirmed update of a Critical alert

**File:** `internal/service/pushworker.go:468-478`

**Issue:** this happens only when the Critical first row was pruned (`PushOutboxRetention` = 7 days) while the report is still live. That is practically unreachable. The Critical first alert itself is never held back by any of the new rechecks. Recorded so the owner knows the old "serious report falls back to Critical" path is gone on purpose (triage target K3).

## Checked and fine

- Critical is never held back by the new rechecks. `personNotMutedOrBlocked` runs only when `run.base == PushKindReliable`. A Critical base gets no person check, as PUSH.md:88 (mute scope) says.
- The K3 skip of a pruned first row matches the triage target. It is not reachable within the 7 day retention for a live report (see SK-I05).
- K6: `CommentPushStillHolds` now refuses a comment by a person either side blocked, in both directions.
- WR-S03: the Reliable base and the confirmed update that replaces it run block in both directions and the reader's mute. A Follow base keeps `followHolds`.
- The IN-E02 key is never logged. It is used only in `peopleCursorTag`, and the panic text carries the rand error only. Comparison uses `hmac.Equal`.
- WR-S04: both race orders are closed. `SetProfilePublicOn` needs `display_name = screened_name`, and the Name edit lands unscreened only while `NOT profile_public`, judged in the statement (READ COMMITTED re-evaluates on the new row version).
- WR-S04 and a pending username: a rename during the Name screen can leave the profile on with a pending username. Reads gate on a clear username (IN-E07 pins pending as the uniform 404), so nothing leaks.
- WR-E03: every one of the 10 `/api/moderation/*` handlers calls `modGuard`, which checks the marker before the method. Only `requireModerator` sets the marker.
- WR-E01: `/u/{username}` takes the PeopleRead token before the lookup, so the 429 is the same for every name and the uniform 404 bytes are untouched. The 429 is an HTML page with html/template escaping, noindex and no script.
- WR-E02: the off request never meets the Visibility bucket. The numbers in MODERATION.md are read from `cmd/server/main.go` by the round 2 test.
- WR-S01, WR-S02 and IN-E03: the restore window uses `created_at`. A retry on an already removed row re-evaluates. Remove all re-evaluates every removed reporter and joins the errors.
- WR-S05: `RaterMayRate` is false for a pair with a removed rating. The trust arithmetic is unchanged.
- IN-S01 and IN-E01: the sweep's blocked pair loop ends through `endFollowPair` with reason blocked. It stops when a pass ends nothing.
- IN-T01: `CoarsenFinishedForProfile` is precision 5 for every category. The finished row template has no description, link or time. TRUST-MODEL.md:685 and :845 and the hand off line agree.
- The IN-B03 `Pinalert-Changed` header is set before the 204. A malformed name gets the uniform 404 with no header. "Nobody holds it" and "never muted" both answer 0, so the header shows only the caller's own row.
- IN-T04: `aria-hidden` on the chip applies only in the live row block (template line 137). The finished rows (line 147) keep their Resolved or Expired chip readable.
- K2: `#rate-host` has no role or aria-live, mounting calls no `focus()`, and both Undo paths (`onTakeBackClick`, `takeBackFromToast`) call `removeRateBox`. The phone base line equals the toast's (phone.css:187).
- The amended tests are not vacuous:
  - the stars.css hex colour regex still bans colour;
  - the pushworker pruned row cases assert `not_eligible`;
  - the coarsen and profilereports cases assert precision 5 and that the point differs from the anonymous Past point.
- Docs: PUSH.md, TRUST-MODEL.md and MODERATION.md carry no stale "falls back to Critical" or "Reliable for any other" wording.
  - The restore window reads "first written" in all three places.
  - The Policy names the name check by OpenAI, the ended follow reasons (matching the `account_follows_end_reason_valid` set), the switch time, the cleared rating and the permanent bar.

## Not checked (budget)

- I ran no Go tests (no database allowed). I ran node tests only for votes_stars_test.mjs.
- I did not check the rendered placement of `#rate-host` in a real phone simulator, including whether `.app-window`'s inert handling covers it on profile.html, where the host is a body child.
- I read only the diff of `accountdelete.go` (IN-E06), WR-T01, WR-T02 (CSS contrast), IN-T02, IN-T03 and IN-T05.
- I did not check IN-E04 against every browser caller's body shape.
- I did not trace `settings-people.js` IN-B04 in depth. One small oddity: a toggle during the count read is put back to the saved value, and then `save(false)` still runs.
- I did not open the swagger diffs.

_Skeptic: Claude, read only, 2026-10-30_
