---
phase: 05
part: review-fix-skeptic-2
base: 2417b4ac
head: 1a72f388
checked_at: 2026-10-29
counts:
  closes: 4
  does_not_close: 0
  new_problems:
    warning: 1
    info: 0
---

# Phase 5 review fix round 2: skeptic pass

Read only. Scope: the source and test changes between 2417b4ac and 1a72f388 (commits 48313882, 0675ab22, 7a6b8a2c, 00ea2da3, 5b7a00e1). No database, no browser, no Python.

What I ran:
- `node --test web/official_alerts_test.mjs web/official_pins_test.mjs web/official_card_close_test.mjs`: 119 of 119 pass.
- `DATABASE_URL= go test -short ./internal/api/handlers/ ./internal/demo/`: both ok. TestDemoVisitLimitFailsClosed runs under -short (no database) and passes.

## Verdicts

| Id | Closes? | New problem? | Evidence |
|----|---------|--------------|----------|
| NP-1 | closes | yes, NP2-1 (warning) | official-alerts.js:327 sets demoPage from `data-demo` on `#official-page` (demo_alerts.html.tmpl:46-47) or PinalertDemoPage. tick (:1014) and askIfOld (:1020) make no request on a Demo page, so no visit token is spent without a press. The handover at :910 now also needs `byReader === true`. Is the load error honest? Yes: it claims nothing. Can Try again recover? Yes: renderError enables Try again, and load(true) on a 404 with field visit calls PinalertDemoEnded. That path is close to dead code, though: PinalertDemoEnded exists only on Demo pages (demo.js:248), and those pages never ask on their own now. So the third NP-1 test (an automatic ask meets an ended visit) stubs PinalertDemoEnded on a page that is not a Demo page, a pairing that cannot happen in production. It still pins the byReader guard, so it is not vacuous (info only). Demo /api lens: this round adds no request path, and apiBase() still returns null on a page with `data-demo="1"` and no base (:319, unchanged), so a Demo page cannot reach the real /api. The cost is that the Demo page keeps its CR-B01 claim forever (see NP2-1). |
| NP-2 | closes | none found | official-pins.js:417-421, :454-457 and :465-468 take the marker off before closeOpenCard. map.js:2418 also counts focus that is inside the card when the close runs, so on a phone the opener is found disconnected and focus goes to the map container (:2442-2449). No new listener or timer. The `lost` check is unchanged, so focus the reader moved is still left alone. The new test runs the real closeOfficialPopup text, cut from map.js, so a rename fails it. |
| NP-3 | closes | none found | demo_page.go:130 `!ok \|\| !charge()`. The only caller that can create a world is demoVisitFor (:148). All three pages are wrapped (demo.go:137). The test serves the three pages with no limiter and asserts 429 and an empty store for a missing, unknown or malformed `v`. It also asserts that a move within a living visit still answers 200. |
| NP-4 | closes | none found | data-row-key is kind:source:id (:534). kind and id are real JSON fields (handlers/official.go:72, :80), so keys stay stable across rebuilds and cannot collide across groups. takePlace and keepPlace (:643-701) reopen the open rows. They put focus back only when it is on body, on nothing, or on a discarded row button (focusLost), so focus the reader moved is never pulled back. The fallback title gets tabindex -1. The panels hold no focusable control (buildDetails :488 makes no link or button), so "focus inside an open panel" cannot be lost. CR-B01 holds: over a fresh answer the page still blanks first (:879-882), and quiet mode (no blanking) applies only over stale, which reads "Last known alerts" (:843), or unavailable, which reads "could not be checked" (:811). Neither shows "In force now" or "No official alerts in force". No new listener or timer, and rowButtons is reset in clearList. The hung ask that still loses focus is recorded by the fixer as a residual and is older than this round. |

## New problems

### NP2-1 (warning): an idle Demo Official alerts page keeps "In force now" up forever, even after its rows pass their Until time and after the visit has ended

**File:** `web/static/js/official-alerts.js:1014` (tick: `!demoPage && answerTooOld(...)`), with `:327` (demoPage), `:852` (setTitle 'In force now'), `internal/demo/seed.go:327` (shortest sample EndsIn 6 h), `internal/demo/store.go` (an idle world is swept at 30 min).

**Scenario (by reading; the existing test pins it):**
1. Open /demo/alerts and leave it.
2. The NP-1 fix turns off every automatic ask on a Demo page, including the fresh limit check and the "a shown row has ended" check (rowEndedSinceDrawn).
3. At 30 minutes the world behind the page is swept, but the page still says "In force now" over rows from a visit that no longer exists.
4. After 6 hours the shortest sample row has passed its Until time. It is still listed and counted under "In force now", and the status line climbs ("Last checked 6 hours ago").

This is exactly what CR-B01 forbids. The rule (comment at :966-969) is that the page never keeps "In force now" up past the 30 minute fresh limit or after a shown row ends. Before this round the Demo page did drop the claim (badly, by restarting itself). Now it never does.

The new test makes it worse by locking in the regression. web/official_alerts_test.mjs:1333 asserts `hasInForce(page)` after 121 idle minutes ("the sample answer stays drawn").

The fixer recorded this as a residual ("An idle Demo Official alerts page ages in place"). The Demo strip lowers the harm, but the page itself still makes a false claim, and this phase exists to make that claim honest.

**Smallest fix:** in tick, on a Demo page, drop the claim without asking:

```js
var elapsed = Date.now() - renderedAt;
if (answerTooOld(current, elapsed)) {
  if (demoPage) {
    var place = takePlace();
    renderError(); // no request: the visit is over or the sample has aged; Try again restarts it
    keepPlace(place); // focus goes to the error title (tabindex -1), never the body (NP-4)
    return;
  }
  load(false);
  return;
}
```

This sends no request, so no token is spent and the page does not move. renderError sets `current = null`, so later ticks stop at `!current` and this runs once, not on every tick. takePlace and keepPlace keep the NP-4 rule: a reader on a row lands on the error title, not the body. "In force now" goes away at the 30 minute limit, which is also when the world is swept. Try again then runs load(true): it gets the 404 with field visit and hands over to PinalertDemoEnded once, which is the WR-B08 path the reader asked for. Optionally, show the server's own sentence ("This Demo has ended. Reload the page to start again.") in the error body on a Demo page.

**Test:** change the NP-1 idle case in official_alerts_test.mjs. After 31 one-minute ticks:
- `page.requests.length === 1`
- `ended === 0`
- `hasInForce(page) === false`
- `shown(page, 'official-error')`

Also focus the first row before the ticks, and assert afterwards that `document.activeElement` is the error title, not the body.

Then press Try again, answer it with 404 `{error:{field:'visit'}}`, and assert `ended === 1`.

---

_Checked: 2026-10-29. Skeptic, read only. No repo file changed except this one._
