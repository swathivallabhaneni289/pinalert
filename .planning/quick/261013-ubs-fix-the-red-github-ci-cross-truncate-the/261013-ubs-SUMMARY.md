---
quick_id: 261013-ubs
type: quick
status: complete
description: Fix the red GitHub CI cross by truncating the cooldown test time to microseconds
completed: 2026-10-13
commits: 874f03e (fix), and the docs commit that holds this file
---

# Quick task 261013-ubs summary

## Done

- One change, test only: `TestClaimEmailCooldownAllowsAfterWindow` in `internal/store/cooldown_test.go` now starts from
  `time.Now().Truncate(time.Microsecond)`, with a two line comment that says why. Postgres keeps microseconds and Linux
  `time.Now()` has nanoseconds, so the value read back never equalled the value sent on GitHub's runners. No application
  code changed.
- The cause and the fix were already written down on 2026-10-07 (the test came from plan 01.1-08). The owner asked for it on
  2026-10-13 after seeing the red cross on the repository page, together with a push of `main`.

## Proof

All Linux runs used Docker with the same steps as `.github/workflows/ci.yml`: Go 1.25 with the automatic switch to the
`go.mod` version (go 1.26.4), a throwaway `postgres:16`, `go build`, `go vet`, then `go test -p 1`. Each run tested a
`git archive` export of one exact commit.

| Commit | What it is | Result on Linux |
| --- | --- | --- |
| 619b688 | the last commit before the fix | `TestClaimEmailCooldownAllowsAfterWindow` FAILS, which reproduces the red cross |
| 874f03e | the fix | build and vet OK, 403 PASS, 0 FAIL, 0 SKIP, 11 packages ok |
| dffeaf6 | main tip: the fix plus the phone layout quick task 261013-lrw that the other session merged at 21:54 | build and vet OK, 417 PASS, 0 FAIL, 0 SKIP, 11 packages ok |

- On macOS with `TZ=UTC` on a fresh scratch database the whole suite gave 403 PASS, 0 FAIL, 0 SKIP, so no test depends on
  the Mac's time zone.
- Before the push, the 371 outgoing commits were scanned for private keys, provider tokens, database passwords, secret
  assignments, login link tokens, session cookie values, the owner's own coordinates, phone numbers and added pictures:
  nothing found. Every commit uses the same author address that is already public on the repository.

## Notes

- A first Docker run mounted the live working tree, and the other session's merge landed while it ran, so that run was
  thrown away. The table above only uses exact exports.
- The Docker Go image sets `GOTOOLCHAIN=local`, which GitHub's `setup-go` does not, so the runs set `GOTOOLCHAIN=auto` to
  match CI. `ci.yml` was not changed. Its `go-version: '1.25'` against `go.mod`'s 1.26.4 works through the automatic
  toolchain download, and tightening it stays on the STATE.md concerns list.
- The last proof is the GitHub run on `main` after the push.
