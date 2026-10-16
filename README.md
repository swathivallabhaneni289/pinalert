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
   export BASE_URL="http://localhost:8080"        # must be the externally reachable origin — see below
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
   # export CLIENT_IP_HEADER="Header-Your-Host-Sets" # optional, unset skips the network address backstop; use the header your host sets, only after the host canary in the checklist below
   # export MODERATOR_EMAILS="you@example.com"       # optional, comma separated verified emails; unset means no moderator; a malformed entry stops startup
   ```
   The server refuses to start without `SESSION_SECRET` unless `ENV=development` is also set —
   never let a missing secret silently fall back to an auto-generated one, which would invalidate
   every session on every restart. `RESEND_API_KEY` follows the identical rule: with
   `ENV=development` and no key set, the server prints each verification link to its own log
   instead of sending it; with any other `ENV` (including unset), a missing `RESEND_API_KEY` makes
   the process exit at startup rather than boot into a state where nobody can receive their
   verification email. `BASE_URL` roots the absolute link every verification email carries
   (`{BASE_URL}/auth/verify?token=...`) — a wrong or unset-in-production value produces emailed
   links that resolve to the wrong host, so set it explicitly to your real deployed origin outside
   local development. The address search box in the report modal geocodes through OSM Nominatim,
   whose usage policy requires an identifying contact in the outbound User-Agent rather than a
   stock HTTP library agent string; setting `NOMINATIM_CONTACT_EMAIL` puts your address in that
   header and in Nominatim's optional `email` parameter. There is no signup and no API key for
   this service. Leaving it unset is safe and the server still starts, falling back to the project
   repository URL as the identifier, so unlike `SESSION_SECRET` and `RESEND_API_KEY` this variable
   never blocks boot. The text check, the limits, the moderator screen and the numbers settings are
   described in [docs/MODERATION.md](./docs/MODERATION.md). A missing `OPENAI_API_KEY` outside
   `ENV=development`, a malformed `CONTACT_EMAIL`, a `CLIENT_IP_HEADER` that is not a plain header
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

Pinalert sends verification-magic-link email through [Resend](https://resend.com)'s free tier.
`internal/mailer/loader.go` selects the implementation at startup: `RESEND_API_KEY` unset with
`ENV=development` prints the link to the server log (a build-and-test convenience, not a
substitute for real delivery — a green test suite proves nothing about whether real mail arrives);
`RESEND_API_KEY` set sends through Resend's official Go SDK; `RESEND_API_KEY` unset with any other
`ENV` refuses to start.

**Pre-launch requirement — read before sharing this app with anyone but yourself.** Resend's
sandbox sender `onboarding@resend.dev` (the default when `RESEND_FROM` is unset) delivers **only**
to the Resend account owner's own signup address. Until a custom domain is added and DNS-verified,
every other visitor's verification email is silently undeliverable — and because login is
mandatory to view anything at all (see PROJECT.md's Access Model), an undeliverable verification
email means an unusable app for that person, not a degraded experience.

Before sharing a deployed instance with real visitors:

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
5. Send one canary request through the host to learn which header carries the visitor address
   before setting `CLIENT_IP_HEADER` (unset means no network limit on the new limiters).
   Until `CLIENT_IP_HEADER` is set on a host behind a proxy, the per address limits on place search
   and login link requests see one shared address for every visitor, so set it only after the
   canary shows which header carries the visitor address.
6. Set `MODERATOR_EMAILS` to the verified emails of your moderators and bookmark `/moderate`.
7. Set `OPENAI_API_KEY` and check that a fresh zero balance key is accepted by running the
   recorder once (see [docs/MODERATION.md](./docs/MODERATION.md)).
8. Have a lawyer or mentor read the Disclaimer and Policy pages before any wide promotion.

Resend's free tier is also quota-limited (roughly 100 sends/day at the time this was written —
re-check the current figure in your own Resend dashboard rather than trusting this document). The
per-email resend cooldown and per-IP rate limiter shipping in plan `01.1-05` exist specifically to
protect that quota from being exhausted by repeated requests.

## Account access and logout

Pinalert has **no account-recovery path in v1** (D-11), by deliberate decision, not oversight.
Losing access to the inbox that verified an address means verifying a new address and starting
fresh — the previous account's reports stay attached to the old, now-unreachable address, with no
way to reclaim them. There is no password to reset and no support flow that hands an account back.

Logging out is a different, much safer action: it is fully **reversible**. Clicking "Log out" from
the account menu clears the session cookie on that device and returns to the login gate, but the
account itself, and everything it has ever reported, is untouched — re-verifying the same email
address (on the same device or a new one) restores the full report history on `/profile`. Losing
the inbox is the only unrecoverable case; logging out is never one.

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
