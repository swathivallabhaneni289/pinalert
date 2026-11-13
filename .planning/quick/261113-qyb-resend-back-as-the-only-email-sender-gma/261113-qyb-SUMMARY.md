---
phase: quick-261113-qyb
plan: 01
subsystem: mailer
tags: [email, resend, mailcheck, docs]
status: complete
commits: 3
plan_head_before: dce9c093f077e6e22d61399081288d09f8a793a9
plan_head_after: 2a13d512dc9b705d4e339171b091b581ff4e71a2
key-decisions:
  - Resend is the one email sender, over HTTPS, so hosts that block mail ports are fine
---

# Quick 261113-qyb: Resend back as the only email sender, Gmail removed

Sign in emails go through Resend only again; the Gmail SMTP sender, its fake server and its tests are gone, and `mailcheck` now sends its one sample email through Resend.

## Commits

| Task | Commit | Subject |
| --- | --- | --- |
| R1 | 77c5983d | feat(261113-qyb): send sign in emails through Resend only, Gmail SMTP removed |
| R2 | 19c9c6bd | feat(261113-qyb): mailcheck sends its sample email through Resend |
| R3 | 2a13d512 | docs(261113-qyb): README Resend section, trust model ceiling lines and the Policy sign in email line back on Resend |

## What changed

- Restored from `e754b884`: resend.go, resend_test.go, loader.go, loader_test.go, logmailer.go, ceiling.go, handlers/credentials.go, go.mod, go.sum (resend-go v3.17.0 back, `go mod tidy` clean), the Policy line and its contract test, README and TRUST-MODEL.
- cmd/server/main.go: only the three Resend comment blocks changed back (the file is otherwise as before). internal/mailer/mailer.go: package comment names Resend again; `CodeLifetime` and its pin in internal/service kept.
- Deleted: smtp.go, its three test files, smtptest/, guard_test.go (it guarded only the SMTP test only hooks), redact_test.go, and the three loader_*_test.go files for the SMTP settings.
- Loader: new `mailer.ErrMissingAPIKey`, wrapped by the missing key error; new test `TestLoadMailerMissingKeyIsErrMissingAPIKey`.
- mailcheck: reads RESEND_API_KEY (required) and RESEND_FROM (optional); hint only for the missing key (errors.Is); tests use a recording fake mailer through the `load` seam; the half set and missing password tests are dropped.
- README: Resend section back with the owner check for today, deploy day domain steps, the HTTPS note and the key leak steps (placeholders YOUR_RESEND_SIGNUP_EMAIL and YOUR_DOMAIN). TRUST-MODEL: three lines restored plus the dated amendment note.

## Gate numbers

- `go build ./...` and `go vet ./...`: clean.
- `go test ./... -p 1 -count=1 -v` on database q_qyb_a (created and dropped by this run): 22 packages ok, 0 FAIL; 6843 PASS lines (tests and subtests), 3 SKIP: TestGeocodeLive, TestOfficialLive, TestScreeningFixture.
- `node --test web/*.mjs`: tests 1627, pass 1627, fail 0.
- `git grep -n -i smtp` and `git grep -n smtptest` (excluding .planning): nothing. `git grep resend-go`: go.mod, go.sum, resend.go and resend_test.go.
- Wording check: nothing.

## Deviations from Plan

**1. [Rule 1 - plan check cannot print nothing] The gmail grep has unavoidable matches.** `git grep -n -i gmail` also matches the letters "gMail" inside every `recordingMailer`, `countingMailer` and `logMailer` identifier (already in the handlers and mailer tests before this plan), and the common password list line `gmail1234` in internal/auth/common_passwords.txt. With those filtered out, the only hits are the TRUST-MODEL amendment note and that password list line (data, left as is).

**2. resend_test.go in the resend-go grep.** The plan says go.mod, go.sum and resend.go only; resend_test.go (restored as it was) imports the SDK too.

## Self-Check

Files and commits verified present; working tree clean apart from this summary. No STATE.md, ROADMAP.md or REQUIREMENTS.md staged or edited. Nothing pushed. No network call to Resend or a mail server was made.
