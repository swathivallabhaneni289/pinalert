---
phase: 05-official-feed-demo-mode
plan: 02
subsystem: official-feed
tags: [sachet, cap, rss, encoding-xml, go, httptest]

requires:
  - phase: 03.3-installable-app-and-phone-alerts
    provides: merged to main, the precondition for Phase 5
provides:
  - "internal/official: ParseRSS, ValidGUID, IssuerLabel (rss.go)"
  - "internal/official: ParseCAP, Version, Keep, ParseSeverity, CleanText and the expiry rule (cap.go)"
  - "internal/official: SachetClient with FetchList, FetchCAPs, UserAgent, ErrorClass (sachet.go)"
  - "Seven recorded or synthetic XML fixtures and an opt in live test"
affects: [05-05 poller and storage, 05-09 and 05-13 rendering of official alerts]

tech-stack:
  added: []
  patterns:
    - "Constant base URL plus an unexported seam constructor for tests (same shape as internal/geocode)"
    - "Closed error words (timeout, status, parse, network) with error values that never hold a URL"
    - "Bounded body reads through io.LimitReader with limit plus one byte"

key-files:
  created:
    - internal/official/rss.go
    - internal/official/cap.go
    - internal/official/sachet.go
    - internal/official/cap_test.go
    - internal/official/sachet_test.go
    - internal/official/live_test.go
    - internal/official/testdata/rss_head.xml
    - internal/official/testdata/cap_ap_sdma_lightning.xml
    - internal/official/testdata/cap_imd_chennai.xml
    - internal/official/testdata/cap_maharashtra_update.xml
    - internal/official/testdata/cap_cwc_flood.xml
    - internal/official/testdata/cap_cancel_synthetic.xml
    - internal/official/testdata/cap_exercise_synthetic.xml
  modified: []

key-decisions:
  - "ParseRSS keeps an item whose guid is invalid; FetchCAPs refuses it with class parse and makes no request, so a bad item is counted, not silently lost."
  - "Version.IssuerLabel falls back to the CAP sender when the RSS author has no bracketed label, so a row never shows a blank issuer."
  - "District codes are the union over every language block of the message, sorted and unique, 1 to 99999 only."
  - "CleanText turns a tab into a space, drops bidi override and isolate characters and the byte order mark, and keeps zero width joiners (Indic scripts need them)."
  - "A 304 from FetchList hands back the validators that were sent, so the caller can store them unchanged."

requirements-completed: []  # ALERT-01 and ALERT-02 are only parsed here; storing and showing them is plans 05-05 onward, so nothing is marked per plan

duration: 45min
completed: 2026-10-28
status: complete
---

# Phase 5 Plan 02: SACHET RSS and CAP reading Summary

**A bounded SACHET client that reads the India RSS list as pointers and each CAP message into one normalised, text-safe alert version with English and original headline, sorted LGD district codes, full references and a guaranteed end time.**

## Performance

- **Duration:** about 45 min
- **Tasks:** 2 of 2
- **Files created:** 13 (3 Go sources, 3 Go test files, 7 fixtures)

## Accomplishments

- RSS item treated as a pointer only (guid, title, link, issuer label, pubDate). The CAP address is built from a guid that matches `^[0-9]{10,20}$` on the fixed host and nothing else in an item is ever requested.
- `Version` carries status, scope, msgType, full referenced identifiers, event, closed severity, urgency, certainty, effective, onset, expiry (with `ExpiresInferred`), English and original headline and description, instruction, area text and district codes.
- Expiry rule: a valid `expires` (between sent and sent plus 7 days, both ends allowed) is used; anything else ends at sent plus 6 hours with `ExpiresInferred` true.
- `Keep()` passes only Actual plus Public plus Alert, Update or Cancel.
- Client bounds: RSS 1 MB, CAP 256 KB, 15 s and 8 s timeouts, 3 CAP requests at once, one retry on timeout, network error or 5xx, redirects refused, conditional GET, a Pinalert User-Agent with the contact from configuration.

## Task Commits

1. **Task 1, fixtures:** `12ff9af` test(05-02): add SACHET RSS and CAP fixtures
2. **Task 1, RED:** `3e754cb` test(05-02): add failing tests for the SACHET RSS and CAP parsers
3. **Task 1, GREEN:** `5114777` feat(05-02): parse SACHET RSS pointers and CAP messages into normalised versions
4. **Task 2, RED:** `32a1d8e` test(05-02): add failing tests for the bounded SACHET client
5. **Task 2, GREEN:** `a012860` feat(05-02): add the bounded SACHET client and the opt in live test

## Fixture provenance

| Fixture | Source |
|---------|--------|
| `rss_head.xml` | Copied: first 5 items of the real India feed read on 2026-10-25 (research probe, `rss_india.xml` lines 1 to 56) plus closing tags. Not re-fetched. |
| `cap_ap_sdma_lightning.xml` | Copied, real: Andhra Pradesh SDMA, Lightning, en-IN block plus a Telugu block labelled TL |
| `cap_imd_chennai.xml` | Copied, real: IMD Chennai, 23 LGD District Code geocodes (574 and 568 among them) |
| `cap_maharashtra_update.xml` | Copied, real: Maharashtra SDMA relaying IMD Mumbai, msgType Update, references `IN-1792859773702029_46`, Marathi block (MR) first then en-IN |
| `cap_cwc_flood.xml` | Copied, real: CWC flood, a single en-IN block, a description and no geocodes |
| `cap_cancel_synthetic.xml` | Synthetic, CAP 1.2 element names, says so in a comment; cancels the real Chennai message |
| `cap_exercise_synthetic.xml` | Synthetic, status Exercise, says so in a comment |

No request to SACHET was needed to build fixtures (the research probe outputs were readable), so the 6 request fixture allowance was not used.

## Live test observation

`OFFICIAL_LIVE=1 go test ./internal/official/ -run TestOfficialLive -count=1 -v` was run once, 2026-10-28, with the repository URL as the contact (CONTACT_EMAIL unset). Two requests in all, both succeeded in 0.5 s:

- list: 100 items, ETag present, Last-Modified present
- newest message: msgType=Update, severity=Moderate, languages=en (English block only), kept=true, expiresInferred=false

## Verification

- `go test ./internal/official/ -count=1 -race` passes; `go vet ./...` clean; `gofmt -l` empty.
- All 17 named tests pass (plus TestParseRSSRejectsGarbage, TestParseCAPLimits, TestParseCAPErrors, TestFetchCAPsMarksUnkeptMessages, TestFetchCAPsStopsWhenContextEnds).
- Without the switch, `TestOfficialLive` gives exactly one `--- SKIP`.
- Full suite on the disposable database `p5_02` (created, migrated to version 11, dropped afterwards): 0 FAIL lines, 3 SKIP lines, all allowed (TestGeocodeLive, TestScreeningFixture, TestOfficialLive), 4688 PASS lines, every package `ok`. TestCastVoteConcurrentSameAccountKeepsEveryRow did not flake in this run.
- Acceptance greps: all counts as specified (ParseCAP and ParseRSS signatures 1, lifetimes 1, guid pattern 1, 7 fixtures, 2 synthetic, ErrUseLastResponse 1, LimitReader 1, ValidGUID( at least 1, feed link never read, no em or en dash, no personal address in Go code).

## Deviations from Plan

None to the behaviour the plan specified. Notes on small choices the plan left open:

- Extra tests beyond the named list (garbage RSS, limits, parse errors, unkept messages, cancelled context) were added; none weakens a named test.
- The `MaxLifetime` constant sits in its own const group so the literal acceptance grep `MaxLifetime = 7 \* 24 \* time.Hour` matches (gofmt would otherwise align the equals sign).
- Go source holds no raw invisible characters: the bidi and joiner test inputs and the CleanText rune checks use hex or `\u` escapes.

## Known Stubs

None.

## Threat Flags

None. The surface (outbound GET to the fixed SACHET host, untrusted XML in) is exactly what the plan's threat model T-05-05 to T-05-10 covers, each with a test: SSRF (TestClientBuildsCAPURLFromGUIDOnly, TestClientRefusesBadGUID, TestClientRefusesRedirect), size and time bounds (TestClientErrorClasses, TestFetchCAPsConcurrencyAndRetry), stored text (TestCleanText, TestParseCAPLimits), spoofing (TestParseCAPKeep), log leakage (error text checked for the URL and query).

## Notes for plan 05-05

- `FetchCAPs` returns a result for every item, including `Kept == false` for messages that parsed but are not Actual and Public (store nothing for those) and `Err != nil` with `ErrorClass(err)` for the rest.
- A Cancel can arrive with no `info` block; `ParseCAP` accepts that and leaves the text fields empty (expiry then falls back to sent plus 6 hours).
- `References` hold full identifiers (suffix included); match them against stored `CapIdentifier`, never the guid base.
- `FetchList` makes no minimum-gap decision; the caller owns the poll interval.

## Self-Check: PASSED

All created files exist, all five commit hashes appear in `git log`, `p5_02` dropped, no stray files in the worktree.
