---
phase: 2
plan: "02-03b"
type: execute
wave: 3
depends_on: ["02-03a"]
files_modified:
  - internal/api/handlers/votes.go
  - internal/api/handlers/votes_e2e_test.go
  - internal/api/handlers/reports_e2e_test.go
  - internal/api/handlers/swagger_test.go
  - internal/api/router.go
  - cmd/server/main.go
  - docs/docs.go
  - docs/swagger.json
  - docs/swagger.yaml
autonomous: true
requirements: [TRUST-01, TRUST-03, TRUST-08]

must_haves:
  truths:
    - "A verified account can POST to /api/reports/{id}/confirm, /dispute, /resolve and /reopen and receive 200 with the freshly recomputed {visibility, reason} the server just resolved — proven end to end over a real router and a real Postgres (TRUST-01, HTTP half)."
    - "All four routes are produced by ONE handlers.CastVote(svc, kind, value) factory mounted inside the existing gated r.Group, so the JSON contract, the error mapping and the account resolution exist exactly once."
    - "An unverified caller receives 401 with the gate's standard ErrorResponse envelope and no vote is recorded — the four routes inherit requireVerifiedAccount by construction rather than re-checking auth themselves."
    - "The reporter POSTing /confirm on their own report receives 403 with 'You can't vote on your own report.' (D-03, T-02-05), while the same reporter POSTing /resolve receives 200 and a retracted visibility (D-13, TRUST-08)."
    - "Two confirms from two verified accounts at coordinates inside ONE geohash cell leave the report provisional; moving the second voter to a distinct cell returns live — TRUST-03's independence predicate proven through the full HTTP stack, not only at the service layer."
    - "The request body accepts latitude and longitude only: a body carrying a geohash field is rejected 400 by DisallowUnknownFields, so a client-computed cell can never reach the store (D-17, T-02-01)."
    - "docs/swagger.json documents all four vote paths with their 200/400/401/403/404/409 responses, so Phase 1's OPS-01 API reference stays a truthful description of the API."
  artifacts:
    - internal/api/handlers/votes.go
    - internal/api/handlers/votes_e2e_test.go
    - docs/swagger.json
  key_links:
    - "handlers.CastVote(deps.Votes, service.VoteKindContent, service.VoteConfirm) and its three siblings are registered INSIDE router.go's single gated r.Group — a route added outside it is visibly unprotected by construction (the router's own doc comment, threat T-01-70)."
    - "Deps.Votes *service.VotingService is constructed in cmd/server/main.go from the same sqlcgen.Queries handle Reports and AuthService already use, mirroring Reports' existing wiring exactly."
    - "The handler maps service.ErrCannotVoteOwnReport to 403, service.ErrReportExpired to 409 and service.ErrReportNotFound to 404 via errors.Is — the same sentinel-error-to-field-level-4xx convention reports.go already uses with errors.As on ValidationError."
    - "newE2EServer in reports_e2e_test.go gains a Votes dependency and returns the pool, so every e2e test in the package drives one fully-wired router — one helper, not a second parallel harness."
  prohibitions:
    - "No auth check inside votes.go — identity comes from account.FromContext, populated by the gate."
    - "No visibility, tally or independence logic in the handler or the router; votes.go decodes, calls service.VotingService.CastVote, and encodes."
    - "No r.Route/r.Mount subrouter for the vote paths — router.go's own comment records that every /api/* path in this router is registered flat."
    - "No geohash, visibility, kind or value field on CastVoteRequest — the client sends coordinates, the server decides everything else."
---

## Phase Goal

**As a** person near an ongoing incident, **I want to** confirm or dispute another user's report **so that** reports lacking independent nearby corroboration lose visibility while well-corroborated ones stay trusted — and as a reporter or nearby confirmer, I want to mark a report resolved once it's no longer true, so the feed reflects what's actually happening right now.

<objective>
**This plan is one slice of that story: the four HTTP routes that make the decision layer
reachable.**

02-03a built `VotingService.CastVote` and proved every branch of it against a fake. This plan gives
it a wire protocol: one handler factory, four `POST` routes inside the existing verified-account
gate, the dependency wiring that constructs the service at boot, an end-to-end proof over a real
router and a real Postgres, and a regenerated OpenAPI spec so the published API reference does not
start lying about what this service does.

**Why 02-03 was split.** The original plan spanned store query, service, handlers, routes,
`main.go`, e2e tests and swagger regeneration — around ten files across three tiers. Split at the
tier boundary, each half fits the phase's context budget and ends at an independently provable
guarantee. This half is the one a person can actually `curl`.

The decisions this plan carries to the wire, each cited by ID:

- **D-03** — the reporter cannot vote on their own report. 02-03a rejects it in the service; this
  plan maps that rejection to `403` with the Copywriting Contract's exact message and proves it end
  to end with a real verified reporter driving a real request (T-02-05). 02-05 will additionally
  hide the buttons; that is a courtesy on top of this control, never a replacement for it.
- **D-13** — the reporter *can* resolve their own report instantly. The same e2e file proves the
  403 and the 200 side by side, because a block accidentally widened past content votes would
  break D-13 with no other test noticing.
- **D-17** — the voter's location arrives as raw `{latitude, longitude}` from the browser's
  once-per-session GPS capture. `CastVoteRequest` declares those two fields and nothing else, and
  the decoder runs with `DisallowUnknownFields`, so a body carrying a `geohash` is a loud `400`
  rather than a silently ignored field — the same treatment `SubmitReportRequest` already gives an
  attempt to set a server-computed field.

Purpose: TRUST-01's requirement is that a user can confirm or dispute another user's report. Until
there is a route, that is a library, not a feature. This plan is the last server-side step before
02-05 makes it clickable.

Output: `internal/api/handlers/votes.go`, four routes in `internal/api/router.go`, a `Votes`
dependency wired in `cmd/server/main.go`, `internal/api/handlers/votes_e2e_test.go`, and a
regenerated `docs/`.

**Scope boundary.** No JavaScript, no template, no CSS — 02-05 and 02-06 own the browser. No change
to `GET /api/reports`' response shape and no `?show_disputed=true` handling — that is 02-04, which
depends on this plan. No change to anything 02-03a wrote.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-CONTEXT.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-RESEARCH.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-PATTERNS.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md
@.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-VALIDATION.md
@internal/api/handlers/reports.go
@internal/api/router.go
@internal/api/gate.go
@cmd/server/main.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: One handler factory, four vote endpoints, the full sentinel-error-to-status map</name>

  <read_first>
    - `internal/api/handlers/reports.go` in full — this is the file `votes.go` is modelled on.
      Specifically: the package doc comment's rule that handlers decode/encode only; the
      `maxSubmitBodyBytes` cap; `SubmitReportRequest`'s hand-declared field-by-field shape with
      swag struct tags and its comment about `DisallowUnknownFields` making an attempt to set a
      server-computed field a loud 400; `SubmitReport`'s swag annotation block
      (`@Summary`/`@Description`/`@Tags`/`@Accept`/`@Produce`/`@Param`/`@Success`/`@Failure`/`@Router`);
      the `errors.As(err, &ve)` to `writeFieldError` mapping; and the already-defined
      `writeFieldError`, `writeJSON`, `ErrorDetail` and `ErrorResponse` helpers — same package, do
      NOT redeclare any of them.
    - `internal/api/gate.go` — `requireVerifiedAccount` populates the request context via
      `account.WithAccount`, and its comment explaining that `internal/api/handlers` must import
      `pinalert/internal/account` directly (importing `internal/api` back would be a cycle).
    - `internal/account/account.go` — `account.Account{ID, Email}` and `account.FromContext`.
    - `internal/service/trust.go` as written by 02-03a — `VoteKind`, `VoteValue`, `CastVoteInput`,
      `CastVoteResult`, `VotingService`, and the three sentinel errors with the status codes
      02-03a's artifact table pairs them with.
    - `internal/service/visibility.go` — the four `Visibility` string values and the five
      `ResolveReason` slugs, which are what this handler puts on the wire.
    - `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-UI-SPEC.md`
      Copywriting Contract — the exact user-facing strings for the self-vote refusal, the expired
      report and the generic failure. The server's messages must not contradict the client's.
    - `02-RESEARCH.md` "Code Examples" — the `CastVote` handler-factory example, written against
      this repo's conventions.
  </read_first>

  <files>internal/api/handlers/votes.go</files>

  <action>
Create `internal/api/handlers/votes.go` in the existing `package handlers`. Do not write a second
package doc comment — `reports.go` owns it. Open with a plain file-level comment (blank line before
`package handlers`) stating that this file is the HTTP surface of the confirm/dispute/resolve/reopen
mechanic, that all four endpoints are produced by one factory so the JSON contract and error
mapping exist exactly once, and that every trust decision is made in `internal/service`, never here.

Imports: `encoding/json`, `errors`, `log`, `net/http`, `strconv`, `github.com/go-chi/chi/v5`,
`pinalert/internal/account`, `pinalert/internal/service`. Group them stdlib / third-party / local
as `reports.go` does. Note that `chi` is a new import for this package — it is needed for
`chi.URLParam`, and this is the first route in the codebase with a path parameter.

Declare, in this order:

1. `const maxVoteBodyBytes = 4 * 1024`. Comment: the vote body is two floats, so the cap is far
   tighter than `reports.go`'s 64 KiB submit cap; same denial-of-service rationale, sized to the
   payload.

2. `type CastVoteRequest struct` with exactly two fields, hand-declared with swag tags exactly as
   `SubmitReportRequest` declares its own: `Latitude float64` with json tag `latitude` and an
   example, `Longitude float64` with json tag `longitude` and an example. Its doc comment must
   state that this is the one request shape all four endpoints accept; that the client sends raw
   coordinates captured by the browser's geolocation prompt (D-17) and nothing else; that the
   voter's cell, the vote's validity and the resulting visibility are all computed server-side;
   and that because the decoder runs with `DisallowUnknownFields`, a body that tries to supply a
   precomputed cell or a visibility is a loud 400 rather than a silently dropped field — the same
   protection `SubmitReportRequest` already documents for `expires_at`/`created_at`.

3. `type CastVoteResponse struct` with exactly two fields: `Visibility string` with json tag
   `visibility`, a swag `enums:"hidden,provisional,live,retracted"` tag and an example; and
   `Reason string` with json tag `reason`, a swag
   `enums:"resolved,critical_bypasses_gates,disputed,awaiting_second_independent_confirmation,confirmed"`
   tag and an example. Its doc comment must say these are the resolver's own answer and its own
   explanation, serialised verbatim from `service.CastVoteResult` — the client renders them and
   never recomputes them (D-04, TRUST-02) — and that the enum tags are the published form of the
   two closed sets `visibility.go` declares, so 02-06's display-copy mapping has a documented
   contract to key off. It deliberately carries no vote counts: the weighted "confirmed by N
   nearby" number is Phase 3 / TRUST-05.

4. `func parseReportID(w http.ResponseWriter, r *http.Request) (int64, bool)` — read
   `chi.URLParam(r, "id")`, parse with `strconv.ParseInt(raw, 10, 64)`, and on an empty, unparseable
   or non-positive value write
   `writeFieldError(w, http.StatusBadRequest, "id", "Report id must be a number.")`
   and return `ok=false`, so the caller early-returns. This mirrors `parseCoordinate`'s
   write-the-error-yourself contract in `reports.go`. Reject a non-positive id here rather than
   letting it reach the store: `reports.id` is a `BIGSERIAL` starting at 1, so `0` and negatives
   are malformed input, not missing rows.

5. `func CastVote(svc *service.VotingService, kind service.VoteKind, value service.VoteValue) http.HandlerFunc`
   returning the closure. Body, in order:
   - `acc, ok := account.FromContext(r.Context())`; if `!ok`,
     `http.Error(w, "internal server error", http.StatusInternalServerError)` and return. Comment
     that reaching this handler at all means the gate already resolved a verified account, so a
     missing one is a programming error (a route mounted outside the gated group), not a client
     condition — exactly the shape `SubmitReport` uses for a missing session id.
   - `reportID, ok := parseReportID(w, r)`; return on `!ok`.
   - `r.Body = http.MaxBytesReader(w, r.Body, maxVoteBodyBytes)`, then a `json.NewDecoder(r.Body)`
     with `dec.DisallowUnknownFields()`, decoding into a `CastVoteRequest`. On a decode error:
     `writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")`
     and return — the identical message `SubmitReport` uses, so one client-side branch handles both.
   - Call `svc.CastVote(r.Context(), service.CastVoteInput{ReportID: reportID, AccountID: acc.ID, Kind: kind, Value: value, Latitude: req.Latitude, Longitude: req.Longitude})`.
     `AccountID` comes from the gate's context and never from the body — a client cannot assert
     whose vote this is.
   - Map the error, in this order, each branch returning immediately:
     * `errors.Is(err, service.ErrCannotVoteOwnReport)` to
       `writeFieldError(w, http.StatusForbidden, "account", "You can't vote on your own report.")`.
       The message is the Copywriting Contract's verbatim string. 403 rather than 400: the request
       is well-formed, the caller is authenticated, and the server is refusing on authorisation
       grounds (D-03).
     * `errors.Is(err, service.ErrReportNotFound)` to
       `writeFieldError(w, http.StatusNotFound, "report", "Report not found.")`.
     * `errors.Is(err, service.ErrReportExpired)` to
       `writeFieldError(w, http.StatusConflict, "report", "This report has expired.")`. The message
       is the Copywriting Contract's verbatim string. Record the status choice and its reasoning in
       a comment so a later reader — and 02-05's client-side branch — does not pick a different
       code: 409 Conflict because the request conflicts with the report's current state; 410 Gone
       was rejected because the report still exists and is still retrievable through its owner's
       Activity history; 400 was rejected because nothing about the request itself is malformed.
     * `var ve service.ValidationError; errors.As(err, &ve)` to
       `writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)`, exactly as `SubmitReport`
       does.
     * default: `log.Printf("handlers: CastVote: %v", err)` then
       `http.Error(w, "internal server error", http.StatusInternalServerError)`, so a database error
       is logged server-side and never reaches a client.
     Put the `errors.Is` sentinel branches ahead of the `errors.As` validation branch and comment
     why: the sentinels are distinct error values that `errors.As` on a `ValidationError` would not
     match anyway, and ordering them first keeps the security-relevant refusals visibly at the top
     of the map rather than buried under input handling.
   - On success: `writeJSON(w, http.StatusOK, CastVoteResponse{Visibility: string(res.Visibility), Reason: string(res.Reason)})`.
     200, not 201: a vote is not a newly created addressable resource from the client's point of
     view, and D-02 means the same caller re-POSTing changes their standing vote rather than
     creating a second one.

Give `CastVote` one swag annotation block documenting all four endpoints. Use `@Summary`,
a multi-line `@Description`, `@Tags reports`, `@Accept json`, `@Produce json`,
`@Param id path int true "Report id"`,
`@Param body body CastVoteRequest true "Voter's current coordinates"`,
`@Success 200 {object} CastVoteResponse`, `@Failure 400/401/403/404/409/500 {object} ErrorResponse`
(one `@Failure` line per code), and **four** `@Router` lines: `/reports/{id}/confirm [post]`,
`/reports/{id}/dispute [post]`, `/reports/{id}/resolve [post]`, `/reports/{id}/reopen [post]`.
Paths are relative to the `@BasePath /api` declared in `router.go`. Multiple `@Router` lines on one
operation are supported by the pinned generator — swag v1.16.4's `processRouterOperation` branches
explicitly on `len(operation.RouterProperties) > 1` and emits a copy of the operation per path,
verified against the module source rather than assumed. The `@Description` must state which action
each path casts (confirm/dispute are content votes; resolve/reopen are resolution votes), that a
verified session is required and an unverified caller receives 401, that the reporter receives 403
on confirm/dispute but may resolve their own report, and that the response carries the freshly
recomputed visibility so the client can render the outcome without guessing.

Write no auth check, no tally arithmetic and no visibility branching in this file. If the handler
needs to know what a reason slug means, that is 02-06's display-copy job, not this layer's.
  </action>

  <acceptance_criteria>
    - `internal/api/handlers/votes.go` exists; `gofmt -l internal/api/` prints nothing and
      `go build ./... && go vet ./...` both exit 0.
    - `grep -c 'func CastVote(svc \*service.VotingService, kind service.VoteKind, value service.VoteValue) http.HandlerFunc' internal/api/handlers/votes.go` is exactly `1` — one factory,
      not four handlers.
    - `grep -c 'DisallowUnknownFields' internal/api/handlers/votes.go` is exactly `1`.
    - `CastVoteRequest` declares exactly two fields, `Latitude` and `Longitude`: the struct body
      between `type CastVoteRequest struct {` and its closing brace contains exactly those two
      field names and no third.
    - All three sentinel mappings are present:
      `grep -c 'http.StatusForbidden' internal/api/handlers/votes.go` is `1`,
      `grep -c 'http.StatusConflict' internal/api/handlers/votes.go` is `1`,
      `grep -c 'http.StatusNotFound' internal/api/handlers/votes.go` is `1`, and
      `grep -cE 'service.ErrCannotVoteOwnReport|service.ErrReportExpired|service.ErrReportNotFound' internal/api/handlers/votes.go` is `3`.
    - `grep -c '@Router' internal/api/handlers/votes.go` is exactly `4`.
    - The two user-facing strings match `02-UI-SPEC.md` verbatim:
      `grep -cF "You can't vote on your own report." internal/api/handlers/votes.go` is `1` and
      `grep -cF "This report has expired." internal/api/handlers/votes.go` is `1`.
    - `go test ./... -short` passes — no existing test regressed by the new file.
  </acceptance_criteria>

  <verify>
    <automated>[ -z "$(gofmt -l internal/api/)" ] && go build ./... && go vet ./... && [ "$(grep -c 'func CastVote(svc \*service.VotingService, kind service.VoteKind, value service.VoteValue) http.HandlerFunc' internal/api/handlers/votes.go)" = "1" ] && [ "$(grep -c 'DisallowUnknownFields' internal/api/handlers/votes.go)" = "1" ] && [ "$(grep -c '@Router' internal/api/handlers/votes.go)" = "4" ] && [ "$(grep -c 'http.StatusForbidden' internal/api/handlers/votes.go)" = "1" ] && [ "$(grep -c 'http.StatusConflict' internal/api/handlers/votes.go)" = "1" ] && [ "$(grep -c 'http.StatusNotFound' internal/api/handlers/votes.go)" = "1" ] && [ "$(grep -cE 'service.ErrCannotVoteOwnReport|service.ErrReportExpired|service.ErrReportNotFound' internal/api/handlers/votes.go)" = "3" ] && [ "$(grep -cF "You can't vote on your own report." internal/api/handlers/votes.go)" = "1" ] && [ "$(grep -cF "This report has expired." internal/api/handlers/votes.go)" = "1" ] && go test ./... -short</automated>
  </verify>

  <done>
    One exported factory produces every vote endpoint; it reads identity from the gate's context,
    accepts coordinates only, refuses an unknown body field, maps all three service sentinels to
    403/409/404 with the Copywriting Contract's exact strings, and carries four `@Router`
    annotations ready for regeneration. Nothing is mounted yet.
  </done>
</task>

<task type="auto">
  <name>Task 2: Mount the four routes inside the gate and construct the service at boot</name>

  <read_first>
    - `internal/api/router.go` in full. Three things govern this task. (1) `Deps`' existing shape,
      including the `Dev` and `RequestLinkRateLimit` fields whose comments establish the precedent
      for additive `Deps` changes that leave every existing `Deps{}` literal compiling. (2) The
      single gated `r.Group` at lines 157-173 and `NewRouter`'s doc comment: "A route added to the
      group is protected by construction; a route added outside it is visibly, by construction,
      unprotected (threat T-01-70)." (3) The comment above the `/api/auth/request-link`
      registration, which states that every `/api/*` path is registered flat, gated or not, because
      chi does not allow a literal sibling path alongside a wildcard `r.Route` mount at the same
      prefix. That sentence decides this task's registration style — see `<action>`.
    - `cmd/server/main.go` lines 100-130 — the `queries := sqlcgen.New(pool)` handle and the
      `api.Deps{...}` literal, in particular `Reports: service.NewReportService(queries)`, the
      one-line pattern `Votes` mirrors.
    - `internal/api/handlers/swagger_test.go`'s `newSwaggerTestRouter` — it builds an `api.Deps`
      with only three fields set. Confirm your `Deps` change keeps that literal compiling (it will,
      because a keyed struct literal tolerates omitted fields), and note that a nil
      `*service.VotingService` handed to `handlers.CastVote` is never dereferenced at router
      construction — the factory only closes over it.
    - `internal/service/trust.go` as written by 02-03a — `NewVotingService`, `VoteKindContent`,
      `VoteKindResolution`, `VoteConfirm`, `VoteDispute`, `VoteResolve`, `VoteReopen`.
    - `02-PATTERNS.md`'s `internal/api/router.go` section — note that its sketch uses
      `r.Route("/api/reports/{id}", ...)`. This task deliberately does not; `<action>` says why.
  </read_first>

  <files>internal/api/router.go, cmd/server/main.go</files>

  <action>
In `internal/api/router.go`, add one field to `Deps`: `Votes *service.VotingService`. Place it
immediately after the existing `Reports *service.ReportService` field so the two service handles
read together. Give it a short comment in the established style: it backs the four vote routes
added below, it is additive exactly as `Dev` and `RequestLinkRateLimit` were, and a `Deps{}`
literal that omits it (such as `swagger_test.go`'s) still compiles and still serves every route
that does not touch voting.

Inside the existing gated `r.Group`, directly beneath the two `/api/reports` registrations, add
four route registrations. Register them **flat**, as four separate `r.Post` calls on the full
literal paths — `/api/reports/{id}/confirm`, `/api/reports/{id}/dispute`,
`/api/reports/{id}/resolve`, `/api/reports/{id}/reopen` — each calling
`handlers.CastVote(deps.Votes, ...)` with the matching kind/value pair: content+confirm,
content+dispute, resolution+resolve, resolution+reopen.

Do **not** use `r.Route("/api/reports/{id}", func(r chi.Router) {...})`, even though
`02-PATTERNS.md` sketches it that way. `router.go`'s own comment already records the rule — every
`/api/*` path in this router is registered flat — and it exists because this router has a literal
`/api/reports` leaf that a wildcard subrouter mounted at an overlapping prefix would sit awkwardly
beside. Flat registration also keeps the four new paths in the same visual list as every other
route in the group, which is exactly the property `NewRouter`'s doc comment relies on for T-01-70:
you can see at a glance that they are inside the gate. Add a comment above the block recording this
deviation from `02-PATTERNS.md` and its reason, so a later reader does not "restore" the subrouter.

The comment above the four routes must also record: that all four come from one factory so the JSON
contract and error mapping exist exactly once; that content votes and resolution votes are
distinguished by the `kind` argument rather than by four separate handlers, keeping "is this report
still true" and "is this resolved" as two separately tracked signals (D-02, D-16); and that they
are inside the gate because a vote must be attributable to a verified account, which is Phase 1.1's
entire reason for existing.

In `cmd/server/main.go`, add `Votes: service.NewVotingService(queries),` to the `api.Deps{...}`
literal, immediately after the existing `Reports:` line. It takes the same `queries` handle
`Reports` and `AuthService` already share — `*sqlcgen.Queries` satisfies `service.VotingQuerier`
structurally, because 02-03a declared that interface against the generated method signatures. No new
import, no new environment variable, no new constant. If `*sqlcgen.Queries` does not satisfy
`VotingQuerier`, the build fails here with a message naming the missing method; that is the intended
failure mode, and it must be fixed in 02-03a's interface or query, never by widening the interface
from this file.

Change nothing else in either file.
  </action>

  <acceptance_criteria>
    - `go build ./... && go vet ./...` both exit 0, and `gofmt -l internal/api/ cmd/` prints
      nothing.
    - `grep -c 'Votes \*service.VotingService' internal/api/router.go` is exactly `1`.
    - All four routes are registered flat:
      `grep -c 'r.Post("/api/reports/{id}/' internal/api/router.go` is exactly `4`, and
      `grep -c 'handlers.CastVote(deps.Votes' internal/api/router.go` is exactly `4`.
    - The four registrations sit inside the gated group, not after it. Verified by line ordering:
      the line number of the last `handlers.CastVote(deps.Votes` occurrence is greater than the
      line number of `r.Use(requireVerifiedAccount(deps.Sessions))` and less than the line number
      of `r.Post("/auth/logout"`, which is the final registration inside that group.
    - `grep -c 'Votes: *service.NewVotingService(queries)' cmd/server/main.go` is exactly `1`.
    - `go test ./... -short` passes, and with `DATABASE_URL` set `go test ./... -p 1` passes —
      `swagger_test.go`'s three-field `api.Deps` literal still compiles and `TestSwaggerDocServed`
      still drives the real router.
  </acceptance_criteria>

  <verify>
    <automated>[ -z "$(gofmt -l internal/api/ cmd/)" ] && go build ./... && go vet ./... && [ "$(grep -c 'Votes \*service.VotingService' internal/api/router.go)" = "1" ] && [ "$(grep -c 'r.Post("/api/reports/{id}/' internal/api/router.go)" = "4" ] && [ "$(grep -c 'handlers.CastVote(deps.Votes' internal/api/router.go)" = "4" ] && [ "$(grep -c 'Votes: *service.NewVotingService(queries)' cmd/server/main.go)" = "1" ] && GATE=$(grep -n 'r.Use(requireVerifiedAccount(deps.Sessions))' internal/api/router.go | cut -d: -f1) && LAST=$(grep -n 'handlers.CastVote(deps.Votes' internal/api/router.go | tail -1 | cut -d: -f1) && LOGOUT=$(grep -n 'r.Post("/auth/logout"' internal/api/router.go | cut -d: -f1) && [ "$GATE" -lt "$LAST" ] && [ "$LAST" -lt "$LOGOUT" ] && go test ./... -short && go test ./... -p 1</automated>
  </verify>

  <done>
    `Deps` carries a `Votes` service, `cmd/server/main.go` constructs it from the same queries
    handle every other service uses, and the four vote paths are registered flat inside the single
    gated group — provably between the gate's `r.Use` and the group's last route, so they are
    protected by construction rather than by a reviewer noticing.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Prove it end to end over a real router and a real Postgres, then make the API reference tell the truth</name>

  <read_first>
    - `internal/api/handlers/reports_e2e_test.go` in full — `newE2EServer`'s construction of a real
      `api.Deps` over `testutil.NewTestDB(t)`, the `verifySession` helper that drives the real
      request-link-then-verify flow, `postReport`, and the `cookiejar`-per-client idiom that makes
      one `*http.Client` equal one browser/session. You extend this file's helper and reuse
      everything else.
    - `internal/api/handlers/auth_e2e_test.go` — `recordingMailer`, `requestLink` and `getVerify`,
      which `verifySession` builds on. Do not duplicate them.
    - `internal/testutil/db.go` — `NewTestDB` (skips via `t.Skip` when `DATABASE_URL` is unset, so
      these tests are safe under `-short`), `MustExec`, and `Truncate`'s table list.
    - `internal/testutil/seed.go`'s `SeedExpiringReport(t, pool, secondsUntilExpiry) int64` — it
      inserts a report directly with a literal `session_id` of `"seed-session-expiring"` and no
      `sessions` row, and returns the id. A negative `secondsUntilExpiry` yields an
      already-expired report, and the absent `sessions` row means its `reporter_account_id` is
      NULL, so the self-vote block correctly does not fire for it.
    - `internal/api/handlers/swagger_test.go` — `TestSwaggerSpecCoversRoutes` reads the committed
      `docs/swagger.json` from disk (not the live endpoint) so it fails the moment `make swag` is
      forgotten. You are extending its assertions, not rewriting it. Note its final guard: the spec
      must never contain the strings `session_id` or `sessionId`.
    - `Makefile`'s `swag` target: `swag init -g internal/api/router.go -o docs`.
    - `go.mod` — `github.com/swaggo/swag v1.16.4` is the pinned library version; the CLI must match
      it.
    - `02-VALIDATION.md` "Wave 0 Requirements", fourth bullet — it names
      `internal/api/handlers/votes_e2e_test.go` and the 403/401 cases as required. Those are the
      floor, not the ceiling.
  </read_first>

  <files>internal/api/handlers/votes_e2e_test.go, internal/api/handlers/reports_e2e_test.go, internal/api/handlers/swagger_test.go, docs/docs.go, docs/swagger.json, docs/swagger.yaml</files>

  <behavior>
    Every test below drives the real `api.NewRouter` over `httptest` against a real Postgres and
    skips cleanly without `DATABASE_URL`, except the swagger drift assertions, which read a
    committed file and run under `-short`.

    `TestCastVoteRequiresVerifiedAccount` (the 401 case `02-VALIDATION.md` names):
    - A client with a fresh cookie jar that never verifies POSTs `/api/reports/1/confirm` with a
      valid coordinate body. Status is 401, and the body decodes to the gate's envelope with
      `error.field` equal to `"auth"`. No `votes` row exists afterwards
      (`SELECT COUNT(*) FROM votes` is 0) — the gate refuses before the handler runs, and the
      assertion proves that rather than assuming it.

    `TestCastVoteRejectsReporterSelfVote` (the 403 case `02-VALIDATION.md` names; D-03, T-02-05):
    - One verified client submits a report, then POSTs `/confirm` on its own id. Status is 403 and
      `error.message` is exactly "You can't vote on your own report."
    - The same client POSTs `/dispute` on the same report: also 403.
    - `SELECT COUNT(*) FROM votes WHERE report_id = $1` is 0 — refused before the write, not after.

    `TestReporterCanResolveOwnReportInstantly` (D-13, TRUST-08):
    - The same single verified client submits a report and POSTs `/resolve` on it. Status is 200 and
      the response `visibility` is `"retracted"` with `reason` `"resolved"`.
    - The test's doc comment must state that it exists as the paired counter-case to the 403 above:
      a self-vote block accidentally widened past content votes would break D-13's
      reporter-instant resolve and nothing else in the suite would notice.

    `TestIndependentConfirmsFlipProvisionalToLive` (TRUST-01 HTTP half, TRUST-03, TRUST-04):
    - Client A (verified) submits a low-severity `flood` report at Bengaluru's coordinates. Clients
      B and C are separate `http.Client`s with their own cookie jars, each verified under its own
      email — one client, one browser, one account.
    - B POSTs `/confirm` with coordinates a few hundred metres from A's: status 200, `visibility`
      `"provisional"`, `reason` `"awaiting_second_independent_confirmation"` — one independent cell
      is below `IndependentAgreementThreshold`.
    - C POSTs `/confirm` from a third, clearly distinct location: status 200, `visibility` `"live"`,
      `reason` `"confirmed"`.
    - Derive the three coordinate pairs by computing `geohash.EncodeWithPrecision(lat, lon, 7)` in
      the test and asserting up front that the chosen cells differ, rather than trusting that a
      hand-picked offset lands in another cell. A silently-same cell would make this test pass for
      the wrong reason later.

    `TestConfirmsFromOneCellStayProvisional` (TRUST-03's discriminating case):
    - Same setup, but B and C both POST `/confirm` with the SAME coordinates. After both 200s the
      second response's `visibility` is still `"provisional"`. Two distinct verified accounts
      standing in one cell are one independent confirmation, which is the entire claim of the Core
      Value. Assert up front that the two coordinate pairs encode to the same precision-7 cell.

    `TestCastVoteOnExpiredReportIsRejected`:
    - Seed an already-expired report with `testutil.SeedExpiringReport(t, pool, -3600)`, then have
      a verified client POST `/confirm` on it. Status is 409 and `error.message` is exactly
      "This report has expired."
    - `SELECT COUNT(*) FROM votes WHERE report_id = $1` is 0.

    `TestCastVoteRejectsUnknownBodyField` (D-17, T-02-01):
    - A verified non-reporter POSTs a body carrying `latitude`, `longitude` AND a third key naming
      a precomputed cell. Status is 400 with `error.field` equal to `"body"`, and no vote row is
      written. This is the behavioural proof that a client-supplied cell has nowhere to enter —
      stronger than any grep, because it exercises the decoder the handler actually uses.

    `TestCastVoteOnMissingReportIs404`:
    - A verified client POSTs `/confirm` on report id `999999`. Status is 404.

    `TestSwaggerSpecCoversRoutes` (extended, runs under `-short`):
    - `docs/swagger.json` contains all four paths — `/reports/{id}/confirm`, `/reports/{id}/dispute`,
      `/reports/{id}/resolve`, `/reports/{id}/reopen` — each with a `post` operation.
    - Each of those four operations declares a `401` and a `403` response.
    - The spec contains the four visibility enum values (`hidden`, `provisional`, `live`,
      `retracted`), so 02-06's display-copy mapping has a published contract.
    - The existing `session_id`/`sessionId` guard still passes over the regenerated spec.
  </behavior>

  <action>
**First, extend the shared harness.** In `internal/api/handlers/reports_e2e_test.go`, make two
changes and nothing else.

(1) Add `Votes: service.NewVotingService(queries),` to `newE2EServer`'s `api.Deps` literal,
immediately after the existing `Reports:` line, so every e2e test in this package drives one
fully-wired router rather than two divergent harnesses.

(2) Change `newE2EServer`'s signature to return the pool as a third value:
`func newE2EServer(t *testing.T) (*httptest.Server, *recordingMailer, *pgxpool.Pool)`, returning
`srv, fm, pool`. Add the `github.com/jackc/pgx/v5/pgxpool` import. Update this file's two existing
call sites to `srv, mailer, _ := newE2EServer(t)`. That is the whole change — the pool is what lets
the vote tests seed an expired report and count rows directly, and threading it through the one
shared helper is cheaper and less divergent than standing up a second constructor. Update the
helper's doc comment to say the pool is returned for tests that need to seed or assert rows
directly.

**Then create `internal/api/handlers/votes_e2e_test.go`** in `package handlers_test`, matching
`reports_e2e_test.go`'s imports plus `github.com/mmcloughlin/geohash` and
`pinalert/internal/testutil`.

Write three small unexported helpers at the top and use them everywhere, so no test repeats
plumbing:
- `newVerifiedClient(t, srv, mailer, email) *http.Client` — construct a `cookiejar.New(nil)`-backed
  client and call the existing `verifySession` on it. One client equals one browser equals one
  account; every test that needs N independent voters calls this N times with N distinct emails.
- `postVote(t, client, baseURL, reportID int64, action string, body any) *http.Response` — POST to
  `baseURL + "/api/reports/" + strconv.FormatInt(reportID, 10) + "/" + action` with
  `Content-Type: application/json`.
- `decodeVoteResponse(t, resp) (visibility, reason string)` and
  `decodeErrorResponse(t, resp) (field, message string)` — decode into locally declared anonymous
  structs, never by importing the handler types, so the tests assert the JSON contract as a client
  sees it rather than as the server declares it.

Also write a helper that submits a report through the real API and returns its id, by reusing
`postReport` and decoding the `report.id` out of the 201 body — the vote tests need the id, and
`reports_e2e_test.go`'s existing tests discard it.

Write every test named in `<behavior>`. For the two independence tests, compute the precision-7
cells for your chosen coordinate pairs with `geohash.EncodeWithPrecision(lat, lon, 7)` at the top of
the test and `t.Fatalf` immediately if the distinctness (or sameness) the test depends on does not
hold — a coordinate choice that silently drifts into the wrong cell would turn a real assertion into
a tautology. Count rows with `testutil.MustExec`'s sibling pattern: use the returned pool's
`QueryRow` with `SELECT COUNT(*) FROM votes WHERE report_id = $1` directly, as
`internal/store/votes_test.go` does.

**Then regenerate the API reference.** Run `make swag` (`swag init -g internal/api/router.go -o docs`).
If the `swag` binary is absent, install the version that matches `go.mod`'s pinned library with
`go install github.com/swaggo/swag/cmd/swag@v1.16.4` — not `@latest`, and not the `v1.16.6`
mentioned in `.claude/CLAUDE.md`, which is a general research note rather than this repo's pin;
a CLI newer than the linked library can emit a spec the vendored `docs` package will not compile
against. Commit all three regenerated files: `docs/docs.go`, `docs/swagger.json`,
`docs/swagger.yaml`.

**Finally, extend the drift guard.** In `internal/api/handlers/swagger_test.go`, extend
`TestSwaggerSpecCoversRoutes` with the four assertions in `<behavior>`, following the file's
existing idiom exactly: look each path up in the already-parsed `spec.Paths` map, check for the
`post` key, and use `strings.Contains(string(op), "\"401\"")`-style checks for the response codes,
matching how the existing 401 assertion for `/reports` is written. Add the four visibility enum
values to the existing `body`-substring loop that already checks the category and severity enums.
Do not weaken or remove any existing assertion — in particular the `session_id`/`sessionId` guard
stays exactly as it is. Add a short comment recording that these rows are 02-03b's contribution to
the same OPS-01 drift guard Phase 1 established: four routes shipped, four routes documented.
  </action>

  <acceptance_criteria>
    - `gofmt -l internal/api/ docs/` prints nothing; `go build ./... && go vet ./...` both exit 0.
    - `internal/api/handlers/votes_e2e_test.go` declares `package handlers_test` and contains
      functions named exactly `TestCastVoteRequiresVerifiedAccount`,
      `TestCastVoteRejectsReporterSelfVote`, `TestReporterCanResolveOwnReportInstantly`,
      `TestIndependentConfirmsFlipProvisionalToLive`, `TestConfirmsFromOneCellStayProvisional`,
      `TestCastVoteOnExpiredReportIsRejected`, `TestCastVoteRejectsUnknownBodyField`,
      `TestCastVoteOnMissingReportIs404`.
    - `grep -c 'func newE2EServer(t \*testing.T) (\*httptest.Server, \*recordingMailer, \*pgxpool.Pool)' internal/api/handlers/reports_e2e_test.go` is exactly `1`, and
      `grep -c 'Votes: *service.NewVotingService(queries)' internal/api/handlers/reports_e2e_test.go`
      is exactly `1`.
    - With `DATABASE_URL` set,
      `go test ./internal/api/... -p 1 -v -run 'TestCastVote|TestReporterCanResolve|TestIndependentConfirms|TestConfirmsFromOneCell'`
      reports every one of the eight as PASS and none as SKIP.
    - `go test ./... -short` exits 0 — the database-backed vote tests skip cleanly and
      `TestSwaggerSpecCoversRoutes` runs and passes.
    - `docs/swagger.json` documents all four vote paths:
      `grep -cF '/reports/{id}/confirm' docs/swagger.json`,
      `grep -cF '/reports/{id}/dispute' docs/swagger.json`,
      `grep -cF '/reports/{id}/resolve' docs/swagger.json` and
      `grep -cF '/reports/{id}/reopen' docs/swagger.json` are each at least `1`. A `0` here means
      the four `@Router` lines did not all emit and the spec is now lying about the API.
    - `docs/swagger.json` contains `CastVoteResponse` and the four visibility enum values.
    - `git status` shows `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml` all modified
      and staged — a regenerated spec that is not committed fails the drift guard on the next run.
    - With `DATABASE_URL` set, `go test ./... -v -p 1` passes in full: every Phase 1, Phase 1.1,
      02-01, 02-02 and 02-03a test still green.
  </acceptance_criteria>

  <verify>
    <automated>[ -z "$(gofmt -l internal/api/ docs/)" ] && go build ./... && go vet ./... && make swag && [ "$(grep -cF '/reports/{id}/confirm' docs/swagger.json)" -ge 1 ] && [ "$(grep -cF '/reports/{id}/dispute' docs/swagger.json)" -ge 1 ] && [ "$(grep -cF '/reports/{id}/resolve' docs/swagger.json)" -ge 1 ] && [ "$(grep -cF '/reports/{id}/reopen' docs/swagger.json)" -ge 1 ] && [ "$(grep -cF 'CastVoteResponse' docs/swagger.json)" -ge 1 ] && for f in TestCastVoteRequiresVerifiedAccount TestCastVoteRejectsReporterSelfVote TestReporterCanResolveOwnReportInstantly TestIndependentConfirmsFlipProvisionalToLive TestConfirmsFromOneCellStayProvisional TestCastVoteOnExpiredReportIsRejected TestCastVoteRejectsUnknownBodyField TestCastVoteOnMissingReportIs404; do grep -q "^func ${f}(t \*testing.T)" internal/api/handlers/votes_e2e_test.go || { echo "missing ${f}"; exit 1; }; done && go test ./... -short && go test ./internal/api/... -p 1 -v -run 'TestCastVote|TestReporterCanResolve|TestIndependentConfirms|TestConfirmsFromOneCell|TestSwagger' && go test ./... -p 1</automated>
  </verify>

  <done>
    The four routes are proven over a real router and a real Postgres: an unverified caller gets 401
    and writes nothing, the reporter gets 403 on confirm and dispute but 200 and a retracted report
    on resolve, two independent cells flip a report from provisional to live while two accounts in
    one cell do not, an expired report gets 409, an unknown body field gets 400, and a missing
    report gets 404. `docs/swagger.json` documents all four paths and the drift guard now fails if a
    future change ships a fifth route undocumented.
  </done>
</task>

</tasks>

## Artifacts this phase produces

Every symbol, route and file plan 02-03b creates or changes. 02-04, 02-05, 02-06 and 02-07 consume
these by these exact names; anything not listed here does not exist yet and must not be assumed.

**New HTTP routes** — all four registered flat inside `router.go`'s single gated `r.Group`, all four
produced by one factory

| Method | Path | Kind | Value |
|--------|------|------|-------|
| POST | `/api/reports/{id}/confirm` | `service.VoteKindContent` | `service.VoteConfirm` |
| POST | `/api/reports/{id}/dispute` | `service.VoteKindContent` | `service.VoteDispute` |
| POST | `/api/reports/{id}/resolve` | `service.VoteKindResolution` | `service.VoteResolve` |
| POST | `/api/reports/{id}/reopen` | `service.VoteKindResolution` | `service.VoteReopen` |

**Wire contract** — this is what 02-05's `votes.js` and 02-07's Activity page fetch against

| Direction | Shape |
|-----------|-------|
| request body | `{"latitude": <float>, "longitude": <float>}` — nothing else; any additional key is a 400, and `Content-Type: application/json` |
| 200 body | `{"visibility": "hidden\|provisional\|live\|retracted", "reason": "resolved\|critical_bypasses_gates\|disputed\|awaiting_second_independent_confirmation\|confirmed"}` |
| every error body | the existing `{"error": {"field": "...", "message": "..."}}` envelope |

| Status | When | `error.field` | `error.message` |
|--------|------|---------------|-----------------|
| 400 | malformed/unknown-field body | `body` | "Request body is missing or malformed." |
| 400 | unparseable or non-positive `{id}` | `id` | "Report id must be a number." |
| 400 | out-of-range coordinate | `latitude` / `longitude` | the GPS-denial copy from `02-UI-SPEC.md` |
| 401 | unverified caller (from the gate, not this handler) | `auth` | "Verify your email to continue." |
| 403 | reporter casting a content vote on their own report (D-03) | `account` | "You can't vote on your own report." |
| 404 | no such report | `report` | "Report not found." |
| 409 | report already expired | `report` | "This report has expired." |
| 500 | anything else | — | plain-text `internal server error`, detail logged server-side only |

**New exported symbols in package `handlers`** (`internal/api/handlers/votes.go`)

| Symbol | Kind | Signature / shape |
|--------|------|-------------------|
| `CastVoteRequest` | struct | `Latitude float64` (json `latitude`), `Longitude float64` (json `longitude`) — exactly two fields |
| `CastVoteResponse` | struct | `Visibility string` (json `visibility`), `Reason string` (json `reason`) |
| `CastVote` | func | `func CastVote(svc *service.VotingService, kind service.VoteKind, value service.VoteValue) http.HandlerFunc` |

**New unexported symbols in package `handlers`**

| Symbol | Signature |
|--------|-----------|
| `maxVoteBodyBytes` | untyped int `4 * 1024` |
| `parseReportID` | `func parseReportID(w http.ResponseWriter, r *http.Request) (int64, bool)` |

`parseReportID` is the accessor 02-04 should reuse if it ever adds a per-report read route — it
already writes its own 400 and returns `ok=false` in `parseCoordinate`'s established style.

**Changed** (signatures and shapes downstream plans may rely on)

| File | Change |
|------|--------|
| `internal/api/router.go` | `Deps` gains `Votes *service.VotingService`; four `r.Post` registrations inside the gated group |
| `cmd/server/main.go` | `api.Deps` literal gains `Votes: service.NewVotingService(queries)` |
| `internal/api/handlers/reports_e2e_test.go` | `newE2EServer` now returns `(*httptest.Server, *recordingMailer, *pgxpool.Pool)` and wires `Votes` — **02-04's and 02-07's e2e tests must call the three-value form** |
| `internal/api/handlers/swagger_test.go` | `TestSwaggerSpecCoversRoutes` additionally asserts the four vote paths, their 401/403 responses, and the four visibility enum values |
| `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` | regenerated by `swag init`; all four vote paths documented |

**New tests** (`internal/api/handlers/votes_e2e_test.go`, `package handlers_test`)

`TestCastVoteRequiresVerifiedAccount` · `TestCastVoteRejectsReporterSelfVote` ·
`TestReporterCanResolveOwnReportInstantly` · `TestIndependentConfirmsFlipProvisionalToLive` ·
`TestConfirmsFromOneCellStayProvisional` · `TestCastVoteOnExpiredReportIsRejected` ·
`TestCastVoteRejectsUnknownBodyField` · `TestCastVoteOnMissingReportIs404`

**Deliberately NOT produced here** (so a drift check does not flag these as missing)

- Any change to `GET /api/reports`' response shape, the `?show_disputed=true` query parameter, a
  per-report `your_vote` field or an is-own-report flag — all 02-04, which depends on this plan and
  will re-run `swag init` for its own response-shape change.
- `web/static/js/votes.js`, the `.vote-controls` markup, the GPS capture, the inline resolve
  confirmation and the toasts — 02-05, 02-06, 02-07.
- Vote counts in any response. The weighted "confirmed by N nearby" number is Phase 3 / TRUST-05.
- Rate limiting on the vote endpoints — not named in any TRUST requirement and explicitly out of
  scope per `02-RESEARCH.md`'s Supporting-libraries note.

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| browser to API | Untrusted input crosses here: the report id in the path, the JSON body, and implicitly the caller's identity claim. This plan's job is to make sure only two floats and a validated id get past it, and that identity is read from the gate rather than from the request. |
| gate to handler | `requireVerifiedAccount` has already resolved the caller's account into the request context by the time any code in `votes.go` runs. The handler consumes that and never re-derives it. |
| handler to service | The handler forwards a typed `CastVoteInput` and translates errors to status codes. It makes no trust decision of its own — every one of those was made and proven in 02-03a. |

No package-manager install occurs in this plan. `swag` is a dev-time code generator already pinned
in `go.mod` at v1.16.4 and already used by Phase 1; installing its CLI at that exact version is a
re-install of an audited dependency, not a new one, so no `T-02-SC` supply-chain row applies
(`02-RESEARCH.md`'s Package Legitimacy Audit records that this phase introduces no new external
package).

## STRIDE Threat Register

Threat IDs are reused verbatim from `02-VALIDATION.md` / `02-RESEARCH.md`'s Security Domain — this
plan mints no new IDs. ASVS level 1, block-on: high.

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-02-05 | Tampering (of a report's own trust score) | `handlers.CastVote` + the four route registrations — reporter self-confirming their own report | high | mitigate | The decision lives in `service.VotingService.CastVote` (02-03a); this plan's contribution is that it is actually reachable and actually enforced over HTTP. `errors.Is(err, service.ErrCannotVoteOwnReport)` maps to 403 with the Copywriting Contract's verbatim message, and `TestCastVoteRejectsReporterSelfVote` proves it end to end with a genuinely verified reporter driving a real request against a real Postgres — including that `votes` holds zero rows for that report afterwards, so a refuse-after-write regression cannot pass. `TestReporterCanResolveOwnReportInstantly` is its paired counter-case: it fails if the block is ever widened past content votes and silently breaks D-13. 02-05 will additionally hide the buttons; that is a courtesy layered on this control, never a substitute for it, and the 403 test is what keeps the distinction honest. |
| T-02-01 | Spoofing | `CastVoteRequest` + the JSON decoder — forged or omitted voter cell, Sybil vote stuffing | high | mitigate | `CastVoteRequest` declares exactly two fields, and the decoder runs with `DisallowUnknownFields`, so a body attempting to supply a precomputed cell or a visibility is a loud 400 rather than a silently ignored field — the same protection `SubmitReportRequest` already gives `expires_at`/`geohash`/`created_at`. `TestCastVoteRejectsUnknownBodyField` proves this behaviourally against the decoder the handler actually uses, which is stronger than a source grep. The cell itself is computed server-side in 02-03a. `TestConfirmsFromOneCellStayProvisional` proves over the full stack that two distinct verified accounts standing in one cell count as one independent confirmation. **Residual risk, accepted and documented:** GPS spoofing via a devtools location override or a mock-location app is not preventable at this project's budget (`PITFALLS.md` Pitfall 4) — no UI copy in this phase may claim the mechanic is fraud-proof. |
| T-02-03 | Elevation of Privilege | The four route registrations and their placement inside the gated `r.Group` — unauthorised resolve/reopen | high | mitigate | Two controls. (a) Authentication: all four routes are registered inside the single `r.Group` wrapped by `requireVerifiedAccount`, so an anonymous caller cannot cast any vote at all. `router.go`'s doc comment makes this structural ("a route added to the group is protected by construction"), and Task 2's acceptance criteria assert the placement by line ordering rather than trusting a reviewer to notice; `TestCastVoteRequiresVerifiedAccount` asserts the resulting 401 and that no row was written. (b) Authorisation: the handler passes `acc.ID` from the gate's context, never a body field, so a caller cannot vote as someone else — and the threshold gating on resolve/reopen is enforced by 02-01's `isRetracted` and 02-03a's `BuildVoteTally`, which this layer cannot bypass because it never computes visibility itself. **Scope note:** 02-01 mitigates the decision half and 02-03a the write/tally half; this plan closes the reachability half. The three together complete T-02-03. |

**Threats owned by other plans** (listed so the gap is explicit rather than silent): T-02-02
(concurrency race on the tally) is structurally absent from 02-02's append-only schema, and this
plan adds no transaction, lock or conflict clause that could reintroduce it. T-02-04 (resolver
bypass from a read path) is 02-01's and 02-04's; this plan's contribution is negative and
load-bearing — the handler serialises `service.CastVoteResult` verbatim and contains no branch on
visibility, so the vote-cast response cannot drift from the feed's answer.

`security_asvs_level: 1`, `security_block_on: high` — all three threats above are dispositioned
`mitigate`, never `accept`.
</threat_model>

<verification>
Run in order, from the repository root.

1. `gofmt -l internal/api/ cmd/ docs/` prints nothing; `go build ./... && go vet ./...` both exit 0.
2. `make swag` exits 0 and is idempotent — running it twice leaves `git status` clean the second
   time. If the binary is missing, install exactly `go install github.com/swaggo/swag/cmd/swag@v1.16.4`.
3. `go test ./... -short` exits 0. Every database-backed vote test skips cleanly;
   `TestSwaggerSpecCoversRoutes` runs here and must pass against the regenerated spec.
4. With `DATABASE_URL` set: `go test ./... -v -p 1` — the full suite is green, including every
   Phase 1, Phase 1.1, 02-01, 02-02 and 02-03a test. `-p 1` is mandatory (shared database, per-test
   `TRUNCATE`).
5. Confirm all eight tests in `votes_e2e_test.go` report PASS and none report SKIP in step 4's
   output.
6. `02-VALIDATION.md`'s own commands for this plan's requirements run green:
   - TRUST-01: `go test ./internal/service/... ./internal/store/... -run TestCastVote -p 1`, plus
     this plan's HTTP half via `go test ./internal/api/... -run TestCastVote -p 1`
   - TRUST-08: `go test ./internal/api/... -run TestReporterCanResolveOwnReportInstantly -p 1`
7. Confirm `git status` shows `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml` staged. A
   regenerated spec left uncommitted fails Phase 1's drift guard on the very next run, for reasons
   the next executor will have no context for.
8. Manual smoke check (optional, not a gate): with the server running and a verified session,
   `curl -b cookies.txt -X POST -H 'Content-Type: application/json' -d '{"latitude":12.97,"longitude":77.59}' localhost:8080/api/reports/1/confirm`
   returns a `{"visibility":...,"reason":...}` body, and `/swagger/index.html` lists all four vote
   operations under the `reports` tag.

Note, not a step for this executor: `02-VALIDATION.md`'s Per-Task Verification Map rows map to this
plan's tasks (`02-03b-01` … `02-03b-03`, wave 3). That table is backfilled phase-wide once all
PLAN.md files exist (`02-PLAN-OUTLINE.md` Open Question 3), not by this plan — several executors
making scoped edits to one shared table would clobber each other. `02-VALIDATION.md` is deliberately
absent from `files_modified`.
</verification>

<success_criteria>
- A verified account can POST to all four vote paths and receives 200 with the `{visibility, reason}`
  pair `service.Resolve` just computed (TRUST-01, HTTP half).
- All four routes come from one `handlers.CastVote(svc, kind, value)` factory, registered flat inside
  the single gated `r.Group`, provably between the gate's `r.Use` and the group's last route.
- An unverified caller receives 401 from the gate and writes no vote row.
- The reporter receives 403 with "You can't vote on your own report." on `/confirm` and `/dispute`,
  and 200 with a `retracted` visibility on `/resolve` (D-03 and D-13 proven together, T-02-05,
  TRUST-08).
- Two verified accounts confirming from two distinct precision-7 cells flip a low-severity report
  from `provisional` to `live`; the same two accounts confirming from one cell leave it
  `provisional` (TRUST-03, TRUST-04).
- A vote on an expired report is 409 with "This report has expired."; on a missing report, 404; with
  an unknown body field, 400 — and none of the three writes a row.
- `cmd/server/main.go` constructs `service.NewVotingService(queries)` from the same handle every
  other service uses, with no new environment variable or constant.
- `docs/swagger.json` documents all four vote paths with their 401/403 responses and the four
  visibility enum values, and `TestSwaggerSpecCoversRoutes` fails if a future change ships a vote
  route undocumented.
- `go test ./... -short` is green, and `go test ./... -v -p 1` is green with `DATABASE_URL` set.
</success_criteria>

<output>
Create `.planning/phases/02-trust-mechanic-core-confirm-dispute-visibility/02-03b-SUMMARY.md` when
done.

Record in it: the four final route paths exactly as registered; the `CastVoteRequest` /
`CastVoteResponse` JSON field names and the full status-to-message table as shipped (02-05 branches
on every one of them, and reads this table rather than the source); the new three-value
`newE2EServer` signature, because 02-04 and 02-07 will both call it; confirmation that `make swag`
emitted all four paths from the single multi-`@Router` annotation block; and the observed
visibility/reason pair at each step of `TestIndependentConfirmsFlipProvisionalToLive`, so 02-06 can
map display copy against real recorded values rather than assumed ones.
</output>

