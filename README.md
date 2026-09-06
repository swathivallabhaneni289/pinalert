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
   ```
   The server refuses to start without `SESSION_SECRET` unless `ENV=development` is also set —
   never let a missing secret silently fall back to an auto-generated one, which would invalidate
   every session on every restart.
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
