---
sketch: 002
name: full-report-view-and-thread
question: "What does View comments open, and how does a plain chronological comment thread look next to the same report block?"
winner: null
tags: [report-view, comments, thread, layout]
---

# Sketch 002: Full report view and thread

## Design Question

The popup only carries a navigation row into the comments. This sketch answers where that row goes: a
larger report view that shows the same report block as the popup and the row, and below it a simple
chronological thread. No avatars, no cards, no feed look.

## How to View

```
open .planning/sketches/002-full-report-view-and-thread/index.html
```

Or tap View comments inside a popup in sketch 001, which opens this view inside the frame, and use the
back arrow to return.

## Variants

- **A: Page.** A dedicated page in the same frame as the Activity page: back arrow, the report block,
  then the thread. One URL per report, server rendered, shareable, works on every screen. Leaving the map
  means the map reloads when you go back.
- **B: Split with map.** Identical to A on a phone. On a wide screen the report opens as a 440px panel
  beside the map, which stays visible with the pin. Keeps location context, but needs client side state to
  open and close the panel, and a phone still needs a full screen sheet.

## What to Look For

- Is it obvious this is the same report you tapped, only larger: same badge, same trust block, same actions?
- Does the thread read as a plain list, oldest first, with nothing that looks like a social feed?
- Post a comment at the bottom. It joins the end and the count updates.
- The Reporter label marks the author's own replies in words. No identities, emails or avatars appear.

## Notes

- The composer is not in the brief. It is one label, one box and one button, included so the thread is
  complete. Remove it if comments will be read only at first. Its box edge uses `--color-text-muted`
  because `--color-border` is only 1.28:1 against the page.
- The comment thread text, timestamps and counts are sample data.
- The score numbers show here as they do in the popup. Phase 3 decision D-09 limits the numbers to the
  popup, so showing them on the larger view is an open decision.
- The page reuses `.profile-page` and `.profile-back-link` from `auth.css`, so only the report block and
  the thread are new CSS.

## Verified

- 88 renders of this sketch pass the in-browser rules check in Chromium and WebKit: all eight states, both
  variants, both themes, at 320, 360 and 1100 wide. axe-core found zero violations, the page has one h1, one
  labelled Comments region and an ordered list for the thread, and the layout survives 320px width and
  the WCAG text spacing values.

## Open Decisions

1. Page (A) or side panel (B)?
2. Numbers on the report view: show them (as built) or keep to the popup only (D-09)?
3. Composer now, or read only until the comments phase is scoped?
4. Should commenters carry the reporter tag (Reliable reporter and so on) later? Not designed here.
