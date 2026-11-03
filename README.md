# Pinalert

A local, verified information feed for emergencies like floods or cyclones — a live,
trustworthy map of what's actually happening on the ground, instead of unverified rumors and
WhatsApp forwards. People near an affected area post short, location-tagged updates; nearby
people confirm or dispute each one, so a report showing "confirmed by 12 people nearby" can be
trusted while an unconfirmed one gets flagged.

Status: planning — see [PROJECT-NOTES.md](./PROJECT-NOTES.md) for the full feature plan, tech
plan, and data model.

Built with Go, PostgreSQL, and Leaflet.js. Deployed as a mobile-responsive website first, PWA
next.

How confirmations are counted, why a place is a precision 7 geohash cell, the 1 km voting rule and the honest limits of all of it are written up in [docs/TRUST-MODEL.md](./docs/TRUST-MODEL.md).

- The two counters, confirmed and disputed, count people at different spots within 1 km of the report, not votes. Several people at the same spot count once, so a crowd in one place cannot raise the number.
- The still current word says whether people at different spots confirmed the report recently, and the map popup shows a 0 to 100 score for it.
- The reporter tag rates how the reporter's earlier reports turned out, and the map popup shows a 0 to 100 score for it as well.

## Run locally

1. **Provision Postgres.** Either use a local Postgres 16/17 instance (Homebrew's `postgresql@16`
   works fine for development), or create a free project on
   [Neon](https://neon.tech) or [Supabase](https://supabase.com) — genuinely persistent free
   Postgres, unlike Render's free tier (expires after 30 days) or Railway (no indefinite free
   tier).
2. **Export required environment variables:**
   ```bash
   export DATABASE_URL="postgres://localhost:5432/pinalert?sslmode=disable"  # or your Neon/Supabase connection string
   export SESSION_SECRET="$(openssl rand -base64 32)"
   export BASE_URL="http://localhost:8080"        # the site origin; roots every share link and og:url, push still reads it; outside ENV=development it must be an https address, see below
   export ENV=development                          # unlocks the SESSION_SECRET/RESEND_API_KEY dev fallbacks below
   export RESEND_API_KEY="re_..."                   # optional when ENV=development, required otherwise — see Email delivery below
   export RESEND_FROM="Pinalert <onboarding@resend.dev>"  # optional; defaults to the Resend sandbox sender when unset
   export NOMINATIM_CONTACT_EMAIL="you@example.com"  # optional, safe to leave unset; see below
   # export OPENAI_API_KEY="sk-..."                  # optional when ENV=development (each text check passes with a WARNING), required otherwise: the server will not start without it
   # export MODERATION_MODEL="omni-moderation-2024-09-26"  # optional, that pinned snapshot is the default
   # export CONTACT_EMAIL="contact@yourdomain.com"   # optional, empty means no contact block on any page
   # export EMERGENCY_PRIMARY="112"                  # optional, default 112
   # export EMERGENCY_OLDER="100,101,108"            # optional, this is the default; set but empty drops the older numbers text
   # export EMERGENCY_STATE_LINES="1070,1077"        # optional, this is the default; set but empty drops the state helpline row
   # export CLIENT_IP_HEADER="Header-Your-Host-Sets" # optional when ENV=development, required otherwise: the server will not start without it; pick it with the host canary check in the checklist below
   # export MODERATOR_EMAILS="you@example.com"       # optional, comma separated verified emails; unset means no moderator; a malformed entry stops startup
   # export VAPID_PUBLIC_KEY="..."                   # optional, phone notifications; make the pair once with: go run ./cmd/vapidkeys, and keep it as host secrets
   # export VAPID_PRIVATE_KEY="..."                  # optional, set both or neither; one alone stops startup
   # export VAPID_SUBJECT="you@example.com"          # optional, an email or https address for the push services; falls back to CONTACT_EMAIL, then an https BASE_URL
   # export OFFICIAL_REFRESH_TOKEN="..."             # optional, at least 32 characters; switches on POST /internal/official/refresh for the outside cron; unset means that route answers 404
   ```
   The server refuses to start without `SESSION_SECRET` unless `ENV=development` is also set —
   never let a missing secret silently fall back to an auto-generated one, which would invalidate
   every session on every restart. `RESEND_API_KEY` follows the identical rule: with
   `ENV=development` and no key set, the server prints each 6 digit sign in code to its own log
   instead of sending it (a line with `code=`; there is never a link to print); with any other
   `ENV` (including unset), a missing `RESEND_API_KEY` makes the process exit at startup rather
   than boot into a state where nobody can receive their code email. `BASE_URL` is no longer used
   by sign in, because no email carries a link any more. Amended 2026-10-29 by Phase 8: it now roots
   every share link and the `og:url` of the public report page, and phone notifications still read
   it as a fallback contact. Outside `ENV=development` the server refuses to start unless `BASE_URL`
   is an `https://` address, so a share link can never leave with `localhost` in it. Both place searches (the report form's and the map's) ask Photon
   (photon.komoot.io, OpenStreetMap data, by komoot) first, because it matches close spellings and
   half typed words, and fall back to OSM Nominatim only when Photon fails, takes over 2 seconds
   and then over 2.5 seconds on one retry, or finds nothing. Nominatim's usage policy requires an identifying contact in the outbound
   User-Agent rather than a stock HTTP library agent string; setting `NOMINATIM_CONTACT_EMAIL` puts
   your address in that header, which both services receive, and in Nominatim's optional `email`
   parameter. There is no signup and no API key for
   either service. Leaving it unset is safe and the server still starts, falling back to the project
   repository URL as the identifier, so unlike `SESSION_SECRET` and `RESEND_API_KEY` this variable
   never blocks boot. The text check, the limits, the moderator screen and the numbers settings are
   described in [docs/MODERATION.md](./docs/MODERATION.md). A missing `OPENAI_API_KEY` outside
   `ENV=development`, a malformed `CONTACT_EMAIL`, a missing `CLIENT_IP_HEADER` outside
   `ENV=development`, a `CLIENT_IP_HEADER` that is not a plain header
   name and a malformed `MODERATOR_EMAILS` entry each stop startup with a message that names the
   setting.
3. **Apply the schema** (a dedicated binary, never run automatically by the server):
   ```bash
   make migrate
   ```
4. **Start the server:**
   ```bash
   make run
   ```
5. Open the URL printed on startup.

### Running tests

- `make test-short` — skips any test that needs a database; safe to run with no `DATABASE_URL` set.
- `make test` — the full suite, including store-layer tests against a real Postgres; requires
  `DATABASE_URL`.
- Point `DATABASE_URL` at a throwaway database created for the run, because the suite truncates every table it touches, and drop it afterwards.
- `node --test web/*.mjs` runs the browser script tests (the shipped scripts in a Node sandbox); it needs Node only, no npm install.

### CI

Every push and pull request runs `go build`, `go vet`, the browser script tests
(`node --test web/*.mjs`, on the runner's own Node), and `go test` against a `postgres:16`
GitHub Actions service container (see `.github/workflows/ci.yml`) — no local setup needed to see
CI results on a PR.

## Email delivery (Resend)

Pinalert sends its sign in email through [Resend](https://resend.com)'s free tier. There are four
emails, all plain, all with a 6 digit code or a short notice and no link: the sign up code, the
Forgot password code, a note to the owner of an address that already has an account when someone
tries Create account with it, and a note to an address with no account when someone tries Forgot
password with it. `internal/mailer/loader.go` selects the implementation at startup:
`RESEND_API_KEY` unset with `ENV=development` prints the code to the server log (a
build-and-test convenience, not a substitute for real delivery; a green test suite proves nothing
about whether real mail arrives);
`RESEND_API_KEY` set sends through Resend's official Go SDK; `RESEND_API_KEY` unset with any other
`ENV` refuses to start.

**Pre-launch requirement — read before sharing this app with anyone but yourself.** Resend's
sandbox sender `onboarding@resend.dev` (the default when `RESEND_FROM` is unset) delivers **only**
to the Resend account owner's own signup address. Until a custom domain is added and DNS-verified,
every other visitor's code email is silently undeliverable, and because login is mandatory to
view anything at all (see PROJECT.md's Access Model), an undeliverable code email means an
unusable app for that person, not a degraded experience.

Before sharing a deployed instance with real visitors, the owner steps are these (the verified
sender is the first):

1. In the Resend dashboard, go to **Domains → Add Domain**, then add the printed DKIM/SPF records
   at your DNS registrar and wait for verification to complete.
2. Set `RESEND_FROM` to an address at that verified domain (e.g. `Pinalert <alerts@yourdomain.com>`)
   and redeploy — `onboarding@resend.dev` must not be the sender in anything but local development.
3. Set the contact email (`CONTACT_EMAIL`; empty means no contact block anywhere). The address is
   your choice, and nothing contact related is public until it is set.
4. Check the emergency numbers on the live government pages (112.gov.in, which neither earlier
   check could open, and your state disaster management authority's site). Change only
   `EMERGENCY_PRIMARY` (default 112), `EMERGENCY_OLDER` (default 100,101,108) and
   `EMERGENCY_STATE_LINES` (default 1070,1077; set but empty drops the text) if they differ. Never
   present 1070 or 1077 as national numbers.
5. Pick `CLIENT_IP_HEADER` with the host canary check before the first start outside
   `ENV=development`: the server will not start there without it. Start from the header your host
   documents (CF-Connecting-IP on Render, Fly-Client-IP on Fly.io, X-Real-IP on Railway). Then send
   one canary request through the host with that header set to a made up value, to any small page
   that shows the headers it received (a throwaway echo service on the same host is enough), and
   check that the host replaced the made up value with your real address. Set `CLIENT_IP_HEADER`
   to that header only when it did. A header the host passes through unchanged would let anyone
   choose their own address, and a wrong header makes the per address limits on place search and
   the sign in and code limits see one shared address for every visitor.
6. Set `MODERATOR_EMAILS` to the verified emails of your moderators and bookmark `/moderate`.
7. Set `OPENAI_API_KEY` and check that a fresh zero balance key is accepted by running the
   recorder once (see [docs/MODERATION.md](./docs/MODERATION.md)).
8. Have a lawyer or mentor read the Disclaimer and Policy pages before any wide promotion.
9. On the day of the first public deploy, move the Policy "Last updated" date and
   `service.CurrentTermsVersion` (in `internal/service/profile.go`) together to that day, so every
   account agrees again to the text that ships (`TestTermsVersionFollowsPolicyDate` keeps the two
   equal).

Resend's free tier is also quota-limited (roughly 100 sends/day at the time this was written;
re-check the current figure in your own Resend dashboard rather than trusting this document). The
server holds a daily ceiling of **90 sends a day** (UTC days, all four emails together) so the
free 100 is never used up, and keeps 20 of those 90 for Forgot password emails. Sends are also
paced at 15 in any 10 minutes (5 kept for Forgot password), and once the other emails are spent one
sender (a client network with `CLIENT_IP_HEADER`, or every request without a valid cookie) may take
only 3 of the kept slots a day, so one network cannot spend the reserve. Several networks working
together still can; see "Signing in" in [docs/TRUST-MODEL.md](./docs/TRUST-MODEL.md). A person is
also limited to one code email every 45 seconds and 5 an hour for each address. A send past the
ceiling shows the person "Something went wrong. Try again." and says nothing about why.

### Signing in: client address header and hashing speed

- **`CLIENT_IP_HEADER` canary.** Sign in has a per device limit and a per address lock in the
  database that need no setup. A limit by network address exists too, and `CLIENT_IP_HEADER`
  names the header your host sets for it. Find the header with the host canary in step 5 of the
  checklist above, then set `CLIENT_IP_HEADER`, which turns the sign in network buckets and the
  per network share of the Forgot password reserve on. It is **required outside
  `ENV=development`**: the server will not start without it. Without it, a script that drops
  cookies is held back only by the per address lock and the hashing cost, and a script that
  fetches a fresh cookie before each Forgot password can spend the emails kept for real resets
  (see "Signing in" in [docs/TRUST-MODEL.md](./docs/TRUST-MODEL.md)).
- **Hashing speed.** Passwords are hashed with argon2id at m=19456 (19 MiB), t=2, p=1, the OWASP
  minimum, with at most 3 hashes running at once. On a very small host, if sign in feels slow or
  the host reports out of memory, use the fallback m=12288, t=3 (OWASP equal strength) by
  editing `DefaultParams` in `internal/auth/password.go`; `TestDefaultParamsFloor` accepts it. Hashes already stored are checked with the numbers they carry and are
  re-made with the new numbers the next time the person signs in.

## Phone notifications

Pinalert can send a short, silent note to a person's lock screen when a Critical report is posted near
their home area, when a severe official alert covers it, and when a person they follow with Notify me
on posts. Changes and comments on a person's reports, and posts by Reliable reporters nearby, show in
the bell only. It is off until both
`VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` are set; with neither set the app works as before. People
switch it on with Alerts on this device in Settings, under Notifications. [docs/PUSH.md](./docs/PUSH.md) covers making the keys,
https, the push services the server accepts, what a lock screen shows, what the site cannot promise
(an iPhone follows its own sound settings, and a sleeping host sends nothing) and the steps for
checking it on real devices. [docs/TRUST-MODEL.md](./docs/TRUST-MODEL.md) says who gets which note.
Never commit the keys. Phone notifications need migration 00011, so run `make migrate` once on an
existing database after pulling.

## The report page, sharing and comments

Phase 8 gave every report a page of its own. It shows the trust block, a short history of how the
report got where it is ("What happened so far"), and the comments. People signed in with a username
can comment from anywhere, reply one level, like a comment with a thumbs up and flag a comment. A
comment, a reply or a like never changes any trust number: only people at different spots within
1 km still confirm or dispute. Comments close when the report is resolved or has faded.

Share on a report gives a link such as `https://your-site/r/<random token>`. Anyone can open it
without an account. The public page shows only the category and severity, a rough area, what is
happening for a Critical report, whether it is still active, disputed, resolved or ended with how
many people confirmed it, and when. It never shows the report's own words, an exact
spot, a name or a comment, and the link preview says "Open to see if this is still true right now"
with no count, so a forward cannot go stale. The link is built from `BASE_URL`, which must be an
https address outside `ENV=development`.

Without `CLIENT_IP_HEADER` the public page has no per address limit at all, and only the long
random token protects it. Set the header after the host canary check (see the checklist above)
before sharing links widely.

Moderators keep or remove flagged and held comments on `/moderate`. [docs/MODERATION.md](./docs/MODERATION.md)
covers the comment checks and limits, and [docs/TRUST-MODEL.md](./docs/TRUST-MODEL.md) says why
comments and likes stay out of every number. This phase needs migrations 00017, 00018 and 00019, so
run `make migrate` once on an existing database after pulling (run the migration first, then start
the new server).

## Official alerts and the Demo

Pinalert copies government alerts from NDMA SACHET onto Official alerts, a one line banner on Nearby and, for Severe and Extreme alerts at a person's home area, a silent phone note. The sign in screen offers Try the demo, a practice copy of the 2024 Vijayawada floods where nothing is saved. [docs/OFFICIAL-ALERTS.md](./docs/OFFICIAL-ALERTS.md) explains the sources and their credits, how the feed is fetched and kept honest, where an alert applies, how the Demo proves the real trust rule, and what is still open (NDMA has not been asked for written terms). Set `OFFICIAL_REFRESH_TOKEN` (at least 32 characters) on the host and add the same value as a GitHub secret, with the repository variable `PINALERT_BASE_URL`, to switch on the outside refresh. Official alerts need migrations 00012 and 00013, so run `make migrate` once on an existing database after pulling.

## Account access and logout

An account is an email and a password. The email is proved once, at sign up, by a 6 digit code
that is typed in; there is no emailed link. Amended 2026-10-29 by Phase 7.1: the older "no
account-recovery path" decision (D-11) is replaced. **Forgot password** on the sign in card emails
a new 6 digit code, and the person then chooses a new password. A reset signs the account out
everywhere, so a stolen session does not survive it. Losing access to the inbox is still the only
unrecoverable case: no support flow hands an account back, and the previous account's reports stay
attached to the unreachable address.

Logging out is a different, much safer action: it is fully **reversible**. Clicking "Log out" from
the account menu clears the session cookie on that device and returns to the login gate, but the
account itself, and everything it has ever reported, is untouched. Signing in again with the same
email and password (on the same device or a new one) restores the full report history on
`/profile`.

An account made before Phase 7.1 with the old emailed link has no password. Its sign in stops
working when migration 00014 runs. The person uses **Create account** with the same email, types
the code, chooses a password and fills in the username, name and home area once; their reports are
still theirs. Passwords, usernames and typed codes need migrations 00014, 00015 and 00016, so run
`make migrate` once on an existing database after pulling (run the migration first, then start the
new server).

## API documentation

With the server running (`make run`), the JSON API is documented at:

- **`/swagger/index.html`** — a browsable Swagger UI: expand an operation to see its parameters,
  request/response schemas, every enum value, and try a live request against your own database.
- **`/swagger/doc.json`** — the raw machine-readable spec.

The document is [Swagger 2.0](https://swagger.io/specification/v2/), generated by
[`swag`](https://github.com/swaggo/swag) from doc-comment annotations on the handlers in
`internal/api/handlers` — it is not hand-maintained, and it is not OpenAPI 3.x (`swag` doesn't
emit that format; tooling that expects OAS3 specifically should account for this).

**Whenever a handler's request or response shape changes**, run `make swag` and commit the
regenerated `docs/` output in the same change. `TestSwaggerSpecCoversRoutes`
(`internal/api/handlers/swagger_test.go`) is the backstop: it fails the build if the committed
spec ever stops covering both `/reports` operations, drops an enum value, or starts documenting a
`session_id`/`sessionId` field — so a forgotten regeneration is caught in CI, not shipped silently.
