---
created: 2026-09-16T14:58:43.441Z
title: Real 3D WebGL globe on login screen
area: ui
files:
  - web/static/css/auth.css:29-71 (.globe-bg, .globe-bg__mask, globe-rotate keyframes)
  - web/templates/login_gate.html.tmpl
  - web/templates/verify_outcome.html.tmpl
  - web/static/js/auth.js
---

## Problem

The Phase 1.1 login screen's "rotating globe" background is currently a flat CSS `mask-image`
applied to a static SVG (`/static/img/globe.svg`), spun with a 90s linear CSS keyframe animation
(`web/static/css/auth.css` lines 29-71). The user looked at it live during Phase 2 UAT
(2026-09-16) and said it doesn't read as a real globe — they explicitly want a genuine 3D render,
not the flat masked-icon look.

## Solution

TBD — user confirmed the direction is "genuine 3D globe" (WebGL), not just a nicer flat
illustration. Needs its own design/planning pass before implementation, since it's a real scope
and dependency decision, not a copy tweak:

- Likely needs a lightweight WebGL library (e.g. Three.js, or a minimal purpose-built globe
  renderer) — a genuinely new dependency for a project whose stack (CLAUDE.md) is otherwise
  vanilla JS + server-rendered `html/template` with no build step and no JS dependencies beyond
  Leaflet/MapLibre (which load as pinned CDN `<script>` tags, not bundled).
- Needs to weigh perf/bundle-size cost for a purely decorative, unauthenticated login-page
  background against this project's no-build-step / free-tier-only / portfolio-scope constraints.
- If pursued, follow the CDN-`<script>`-tag pattern already established for MapLibre GL (pinned
  exact version, SRI-hashed, UMD/global build only — see `01-11-PLAN.md`/`01-12-PLAN.md` for the
  precedent) rather than introducing a build step.
- Route through `/gsd-sketch` or a proper `/gsd-plan-phase` (likely a small Phase 1.1 polish
  phase, e.g. `1.2`) before touching code — this is a real feature addition, not a quick fix.
