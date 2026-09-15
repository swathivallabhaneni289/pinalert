// visibility.js — window.PinalertVisibility: the display half of the trust
// mechanic, turning the resolver's answer into a chip and a state class,
// serving the feed row and the map pin from one builder — the same
// one-builder-two-surfaces arrangement 02-05 made structural for the vote
// block.
//
// This module computes nothing. It reads two server-authored fields
// (visibility, visibility_reason) and renders them. There is no threshold,
// no tally and no derivation anywhere in this file, and adding one would be
// the resolver bypass T-02-04 names — the whole reason 02-01 wrote exactly
// one Resolve function.
//
// SECURITY (T-01-03): every string reaching the DOM goes through
// Pinalert.setText, a text-node assignment, never a markup-parsing sink.
//
// The fallback rule, stated in full because the direction it leans matters:
// an unrecognised, missing or non-string visibility state renders as
// not-yet-trusted, never as trusted. A client that guesses "trusted" for a
// value it could not validate overstates corroboration, which is the exact
// failure this product exists to counter.
(function () {
  'use strict';

  // The four values of 02-01's shared type, in the resolver's own order —
  // the allowlist every server-supplied value is validated against
  // (T-01-17). "retracted" is listed even though 02-04 filters it out of
  // this response in both views (D-12): this allowlist describes the type
  // rather than this plan's two surfaces, and 02-07 renders it on the
  // Activity page without editing a validation control.
  var VISIBILITY_STATES = ['live', 'provisional', 'hidden', 'retracted'];

  // The five reason slugs 02-04's wire-contract table declares — the same
  // closed set 02-03b's vote response delivers under its own key (`reason`
  // there, `visibility_reason` here). This list exists so an unrecognised
  // slug is dropped rather than reflected into an attribute (T-01-17). The
  // reason-to-copy surface is 02-07's, where the Copywriting Contract
  // supplies strings — this plan exposes the slug as a hook and renders no
  // prose from it.
  var VISIBILITY_REASONS = [
    'resolved',
    'critical_bypasses_gates',
    'disputed',
    'awaiting_second_independent_confirmation',
    'confirmed'
  ];

  // The row/badge state-class prefix (main.css's .sev-*/.age-* pattern,
  // continued) and the chip's own modifier-class prefix. Both are safe to
  // strip-and-replace because no existing class on either host element
  // begins with either one.
  var VISIBILITY_CLASS_PREFIX = 'vis-';
  var VISIBILITY_TAG_CLASS_PREFIX = 'visibility-tag--';

  // The Copywriting Contract's three chip strings, verbatim. `live` maps to
  // the empty string: a Live report gets no chip at all — no empty chip and
  // no "Confirmed" chip, that number being Phase 3 / TRUST-05's job. The
  // Retracted entry is declared for 02-07's Activity page and is
  // unreachable from this plan's two surfaces (D-12 removes Retracted
  // reports from both before this client ever sees them).
  var VISIBILITY_TAG_LABELS = {
    live: '',
    provisional: 'Unconfirmed',
    hidden: 'Disputed',
    retracted: 'Resolved'
  };

  // replacePrefixedClass is a deliberate second copy of feed.js's private
  // helper of the same name and behaviour — not an extraction, because
  // feed.js's copy is private to its own IIFE and map.js has none, both
  // surfaces need it, and 02-PATTERNS.md names this helper by name as the
  // pattern for swapping these classes.
  function replacePrefixedClass(el, prefix, newClass) {
    var kept = [];
    var current = el.className ? el.className.split(/\s+/) : [];
    for (var i = 0; i < current.length; i++) {
      if (current[i] && current[i].indexOf(prefix) !== 0) {
        kept.push(current[i]);
      }
    }
    kept.push(newClass);
    el.className = kept.join(' ');
  }

  // visibilityState reads report.visibility, validates it against
  // VISIBILITY_STATES (T-01-17), and returns the provisional slug for
  // anything unrecognised, missing, null or non-string. Falling back to the
  // live slug would let a malformed or unexpected value render at full
  // saturation with no chip — a report that might be uncorroborated reading
  // as trusted. This direction is also the loud one: a mis-wired field name
  // makes every row read as unconfirmed, which is immediately visible
  // rather than silently wrong.
  function visibilityState(report) {
    var value = report && report.visibility;
    if (VISIBILITY_STATES.indexOf(value) === -1) {
      return 'provisional';
    }
    return value;
  }

  // visibilityReason is the same shape against VISIBILITY_REASONS,
  // returning the empty string when unrecognised so an unvalidated slug is
  // never reflected into the chip's data attribute.
  function visibilityReason(report) {
    var value = report && report.visibility_reason;
    if (VISIBILITY_REASONS.indexOf(value) === -1) {
      return '';
    }
    return value;
  }

  // applyVisibilityClass replaces any vis-prefixed class on el with
  // vis-<state>. The caller supplies the element because the two surfaces
  // place it differently: feed.js puts it on the row element (so it
  // inherits down to that row's badge), map.js puts it on the pin badge
  // itself (there being no row) — the same split main.css's .icon-badge
  // comment already documents for the age ramp.
  function applyVisibilityClass(el, report) {
    replacePrefixedClass(el, VISIBILITY_CLASS_PREFIX, VISIBILITY_CLASS_PREFIX + visibilityState(report));
  }

  // createVisibilityTag returns a span built with document.createElement,
  // carrying only the base chip class and the hidden attribute. It applies
  // no state — updateVisibilityTag is always called immediately after,
  // exactly as votes.js's createVoteBlock applies no state and is always
  // followed by updateVoteBlock.
  function createVisibilityTag() {
    var tag = document.createElement('span');
    tag.className = 'visibility-tag';
    tag.setAttribute('hidden', '');
    return tag;
  }

  // updateVisibilityTag is the one place state reaches the chip. It is
  // idempotent because it runs again on every 30-second poll: the modifier
  // class is written even though no rule styles it today (main.css and
  // trust.css style the row/badge, not this chip's own modifier), because
  // 02-UI-SPEC.md declares it in the markup contract and 02-07 extends by
  // that exact name, so the hook ships now rather than being retrofitted
  // later. Guards tag for null so a caller that has not built one yet is a
  // no-op rather than a thrown error mid-render.
  function updateVisibilityTag(tag, report) {
    if (!tag) {
      return;
    }

    var state = visibilityState(report);
    var label = VISIBILITY_TAG_LABELS[state];

    if (!label) {
      Pinalert.setText(tag, '');
      replacePrefixedClass(tag, VISIBILITY_TAG_CLASS_PREFIX, '');
      tag.removeAttribute('data-visibility-reason');
      tag.setAttribute('hidden', '');
      return;
    }

    Pinalert.setText(tag, label);
    replacePrefixedClass(tag, VISIBILITY_TAG_CLASS_PREFIX, VISIBILITY_TAG_CLASS_PREFIX + state);
    var reason = visibilityReason(report);
    if (reason) {
      tag.setAttribute('data-visibility-reason', reason);
    } else {
      tag.removeAttribute('data-visibility-reason');
    }
    tag.removeAttribute('hidden');
  }

  window.PinalertVisibility = {
    VISIBILITY_STATES: VISIBILITY_STATES,
    VISIBILITY_TAG_LABELS: VISIBILITY_TAG_LABELS,
    visibilityState: visibilityState,
    applyVisibilityClass: applyVisibilityClass,
    createVisibilityTag: createVisibilityTag,
    updateVisibilityTag: updateVisibilityTag
  };
}());
