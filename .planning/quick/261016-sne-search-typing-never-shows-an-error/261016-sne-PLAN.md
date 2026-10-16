---
quick_id: 261016-sne
created: 2026-10-16
source: owner request "make sure we have no errors on that part" (address search, 2026-10-16) and the open items of quick 261016-sfu
status: planned
---

# Search while typing never shows an error

## Why

The owner wants the address search to be quick and to show no errors while people type. Quick
261016-sfu made both searches run after a 300 ms pause, which means more searches per address typed.
Two limits now get in the way:
- `GET /api/geocode` (the report form's search) has a per IP budget of 3 searches, then 1 every 2
  seconds (`GeocodeRateLimitDefault`, `ratelimit.NewPerIP` in `internal/api/router.go`). Carrier NAT
  puts many phones behind one address, and on a host behind a proxy without CLIENT_IP_HEADER every
  visitor may share one address, so everyone would share those 3 searches. The place search was
  already moved to a per account budget for this reason (D-16).
- The per account place budget is 3 searches, then 1 every 4 seconds (`PlaceRateLimitDefault`). The map
  search then stops suggesting while someone types, and the report form shows its failure line on a
  429 during typing.

## What changes

1. `GET /api/geocode` gets a per account budget keyed by the verified account id, like
   `/api/geocode/place`, instead of the per IP middleware. Both budgets become 8 searches at once, then 1
   a second, sized for one person typing one address with 300 ms pauses. The process wide limiters for
   Photon and Nominatim inside internal/geocode stay as they are; they protect the two services.
   Record the numbers and the reason in a code comment beside the defaults.
2. Report form search while typing: a 429 or a busy 503 does not show the failure line. The current
   list stays, and the latest text is tried again once the wait passes (the answer's retry time, or 1
   second when there is none), at most 3 times. Only when those tries also fail does the existing
   failure line show. A newer text cancels a pending retry. Searches the person asks for directly keep
   showing every message as today.
3. Map search while typing: the same quiet retry of the latest text, so suggestions catch up instead
   of stopping. Enter and the button keep showing every message.
4. Nothing else in report-form.js changes (posting, closing and the saved reports were just fixed).

## Done when

Handler tests prove two accounts on one address don't share the report search budget and one account
gets 8 then 1 a second; node tests with fake timers prove the quiet retry, its cap, the cancel on newer
text and the failure line after the last try, for both scripts; contract tests pin the defaults. `go
build`, `go vet`, the full suite on a fresh database, `node --test web/*.mjs` and `go test ./web/` pass.
No em or en dash added.
