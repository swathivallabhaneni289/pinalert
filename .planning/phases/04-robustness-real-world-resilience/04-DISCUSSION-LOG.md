# Phase 4: Robustness, Real-World Resilience - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md. This log preserves the alternatives considered.

**Date:** 2026-10-04
**Phase:** 4-Robustness, Real-World Resilience
**Areas discussed:** Moderation outcomes, Rate limit rules, Text-only view (renamed Low data view), Offline queue, Disclaimer and legal pages (with a Report button)

**How the session went:** the owner chose all four offered areas. From the second moderation batch onward the owner repeatedly chose
"clarify" on pick-a-button prompts (six times) and answered well in plain typed sentences, often through an Instagram comparison.
From the rate limit area onward the questions were asked in plain text, one at a time, with a recommendation. Two background
fact-check workflows ran (outside facts for the options, then a second check of India's emergency numbers after the owner challenged 112).

---

## Moderation outcomes

| Question | Options presented | Selected |
|----------|-------------------|----------|
| Who sees a non-critical report while the text check cannot finish? | Hold off every feed (Recommended); Show with text withheld; Show as Provisional (roadmap wording) | Hold off every feed |
| Clearly abusive text, and does severity change it? | Reject all, reword (Recommended); Reject non-critical only; Fail open for critical | Reject all, reword |
| Middle band (borderline text): what does "flag" do? | Post and log (Recommended); Post and add a review page | Post and add a review page |
| What counts as spam? (multi select) | Links; Same text again within an hour; Gibberish or repeated runs; Phone numbers | Gibberish or repeated runs only |
| Build the text check once so comments and nicknames reuse it? | Yes, build it once (Recommended); Reports only for now | Yes, build it once |
| Who can open the moderator screen? | Emails listed in settings (Recommended); A moderator flag on accounts | Not answered; default applied |
| How to state the false positive target? | Test on realistic posts (Recommended); State 0.1% as a goal | Not answered; default applied |

**Notes:** the owner first read "owner-only review page" as a page for the person who wrote a report. Claude explained it is a private screen for
the person running Pinalert, and that the owner's Instagram idea (open another person's report, comment, react) is Phase 8. The owner chose to
clarify on the moderator access and accuracy questions twice, then moved on, so the recommended defaults were applied and stated. Claude
raised that OpenAI's free moderation allows about 5,000 checks a day, and that India's takedown windows were reportedly shortened in
February 2026, so Remove on the moderator screen works on any post by link.

## Rate limit rules

| Question | Options presented | Selected |
|----------|-------------------|----------|
| Which actions are limited, keyed on what? | Not asked (settled by Phase 2 WR-01, Phase 3 deferral, Phase 1.1 D-10) | Reports, vote routes, re-cast cooldown; per verified account |
| Should the limit ever stop a rescue request? | Separate looser limit; Same limit for everything; Never limit rescue (shown as pick-a-button, set aside) | Separate, roomy allowance (after the owner asked how we would know we block a real emergency) |
| Do other serious events get the bigger allowance? | Free text | Yes: anything marked critical in any category, plus rescue always |
| How strict at the start? | Generous; Moderate; Strict (set aside) | Serious 5 then 1 per 5 min; others 8 then 1 per 15 min; votes 20 then 1 per 15 s; re-cast once per 30 s |

**Notes:** owner's words: "Another person can put up a request on a different situation in an hour or two. How would we know if we are
blocking a real emergency? Maybe we should let them do it anyway." Then: "the other situations or events might also be as serious as the rescue
thing, we should have the allowances for these also", and "low and medium should also have it, but for a long time, we can make the time longer."
Claude's reasoning: critical and rescue reports cannot be hidden by disputes, so the limit is the only automatic brake; a refilling allowance
protects a different situation an hour or two later.

## Low data view (the roadmap's text-only view)

| Question | Options presented | Selected |
|----------|-------------------|----------|
| What can people do on it? | 1 Read only; 2 Read and post (Claude's suggestion); 3 Read, post, Confirm or Dispute | 3 |
| Does it use the exact location? | Owner question | Yes; accuracy shown; typed place secondary; no new browser question |
| Name | Low data view (Claude's suggestion); Text only view | Low data view |
| How do people find it? | Link on login page and account menu plus bookmark; "Taking long?" message after about 6 s; remembered per device | Approved ("ok") |

**Notes:** the owner asked what a vote is and what is "going on with the vote", then whether the browser asks for location again at login, report
and vote. Answered from the code. The owner pointed out that "light" collides with the Light and Dark theme, hence the name. Claude corrected its own
overstatement that location made option 3 heavy.

## Offline queue

| Question | Options presented | Selected |
|----------|-------------------|----------|
| The limit: works only while the page is open; no service worker; votes not saved offline | Statement and question | Owner: keep it a website, not discussing the installable app |
| How old should a late report look? | A. Written time, expires from then (Claude's suggestion); B. New at arrival | B, with "Written 4 hours ago" shown; never expires |
| Defaults (Waiting to send strip, rejection handling, list stays on screen, votes not saved, bound to account) | Stated | Not objected to |

**Notes:** owner's words: "It should not expire. No matter what, because it's just got updated because of the low data or no connection."
The owner later restated: the timer starts from upload, the row says written four hours ago, and it expires like other reports from that point.

## Disclaimer and legal pages, and the Report button

| Question | Options presented | Selected |
|----------|-------------------|----------|
| Where does the bar sit on the map page? | Top (Claude's suggestion); Bottom | Top |
| Which emergency number? | 112 only (first suggestion); 112 with the familiar older numbers named | Owner: "I'm in India. I don't think we use 112 here." Two independent checks run; wording with 112 first and the familiar numbers shown |
| Separate pages? | Owner request | A public Disclaimer page and a public Policy page |
| Contact line or a Report button? | 1 Contact line only; 2 Report button on every post | Both |
| Police message inside the Report flow? | Part of Claude's Report button description | Removed by the owner: police guidance is text on the pages only |

**Notes:** the contact email shows only when an address is set (the owner has not chosen one). The owner described the Report button as
"exactly what Instagram does" so that the admin can take action. Claude told the owner it would note a suggested order for trimming the phase
if it runs long (vote buttons on the Low data view first, then the Report button, then the long Policy text) unless told otherwise; the owner
moved on without objecting, so it is recorded as Claude's suggestion, not a confirmed decision.

## Review before saving

An independent four angle review of CONTEXT.md (code accuracy, conflicts with earlier decisions, completeness against this discussion, style and
invented facts), with a skeptic re-checking each finding, found 13 real issues (13 more were rejected by the skeptic). All 13 were fixed. The most
important: the bar and Disclaimer wording said the older numbers 100, 101 and 108 "still work", which the evidence does not prove (many states
still list them, some have retired them, and no source proves forwarding to 112), so it now says "may also work". The rest: the REQUIREMENTS.md
ROBUST-03 text and the roadmap "Depends on" line now appear in the edit list; explicit amendment notes for Phase 2 D-06 and D-08 (R-01, R-03);
a Report button allowance and a fixed cross reference; what Keep does on a held post; how a moderator finds a post by id before Phase 8; the
network address backstop no longer contradicts the account key; the takedown window is called reportedly short instead of verified; and the
provider client pattern now cites the mailer (the geocoder has no development fallback).

## Claude's Discretion

Reject and borderline thresholds; provider client timeout and retries; limiter store and the network address backstop; held state modelling;
IndexedDB details; Low data view build approach and size budget; exact copy; bar and Report control geometry (UI contract); the Disclaimer and
Policy text drafts; the moderator access list (default).

## Deferred Ideas

Instagram style report page with comments and possible likes (Phase 8); author deleting their own report; state aware helplines; installable
app, service worker and Web Push (backlog 999.3); image safety screening; account level sanctions; email alias normalisation.
