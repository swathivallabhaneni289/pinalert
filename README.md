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
   local development.
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

### CI

Every push and pull request runs `go build`, `go vet`, and `go test` against a `postgres:16`
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

Resend's free tier is also quota-limited (roughly 100 sends/day at the time this was written —
re-check the current figure in your own Resend dashboard rather than trusting this document). The
per-email resend cooldown and per-IP rate limiter shipping in plan `01.1-05` exist specifically to
protect that quota from being exhausted by repeated requests.

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
