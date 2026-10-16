// ended-cards.js - window.PinalertEnded: the card builders and the word
// helpers of a finished report (Resolved or Expired), moved out of feed.js in
// Phase 3.4 when the Past tab left Nearby (D-18). Finished reports now live in
// My activity, part Ended nearby, which draws them from these builders.
//
// The behaviour is the Phase 3.1 Past card, unchanged: a card is one tab stop
// and holds no button and no link. Opening it is the caller's job: the caller
// passes an onOpen function to createPastCard, called with the report id on a
// click, Enter or Space. This file fetches nothing, reads no store rows and
// computes no trust figure: the counters, the outcome label and the closed and
// lifetime phrases are the server's, written through Pinalert.setText.
//
// SECURITY (T-01-03): report text is authored by strangers. Every value
// reaches the DOM through Pinalert.setText, a text node assignment, never a
// markup string a browser then parses. A finished Rescue report arrives with an
// empty description and a blurred point, and its card never reads a
// description on top of that (D-23).
window.PinalertEnded = (function () {
  'use strict';

  // The words a finished Rescue card uses in place of the report's own text
  // and category word (C17, C18). The server sends such a report with an empty
  // description and a blurred point, and the card never reads the description
  // of an approximate report on top of that (D-23).
  var FINISHED_RESCUE_TITLE = 'Rescue needed';
  var APPROXIMATE_LABEL = 'Approximate location';

  // SEPARATOR is the app's own middle dot between the parts of a line.
  var SEPARATOR = ' · ';

  // The one line of a partial group: the server could not read every report of
  // the area for this group, so some are not shown (WR-02). It is its own flag
  // and its own words, and it never borrows the cap line: "Showing the newest
  // 25." says more than 25 were kept, which a partial flag does not say.
  var PARTIAL_LINE_TEXT = 'This area has more reports than this list can hold, so some are not shown.';

  // createPastCard builds the empty card of one finished report. It is one tab
  // stop and holds no button and no link: the whole card opens through the
  // caller's onOpen (when it is a function), and the visually hidden cue is its
  // last text. updatePastCard fills it.
  function createPastCard(id, onOpen) {
    var li = document.createElement('li');
    li.className = 'past-card';
    li.setAttribute('tabindex', '0');
    li.setAttribute('role', 'listitem');
    li.dataset.reportId = String(id);

    var title = document.createElement('p');
    title.className = 'past-card__title';

    var meta = document.createElement('p');
    meta.className = 'past-card__meta';

    // The facts line is the trust line the list rows use, from the one
    // builder, so its numerals and words are the server's and this file names
    // none of the trust fields. The outcome label of an Expired card is its
    // first child; updatePastCard shows or hides each part.
    var trust = PinalertTrust.createTrustLine();
    var facts = trust.el;
    facts.classList.add('past-card__facts');
    var outcome = document.createElement('span');
    outcome.className = 'past-card__outcome';
    outcome.hidden = true;
    facts.insertBefore(document.createTextNode(' '), facts.firstChild);
    facts.insertBefore(outcome, facts.firstChild);
    // The two counters get this card's own class names, so the stylesheet
    // never has to name the row's classes for the card.
    var counts = facts.querySelectorAll('.report-row__trustline-count');
    counts[0].classList.add('past-card__count');
    counts[1].classList.add('past-card__count', 'past-card__count--second');

    var cue = document.createElement('span');
    cue.className = 'past-card__open-cue visually-hidden';
    Pinalert.setText(cue, 'Open report');

    li.appendChild(title);
    li.appendChild(meta);
    li.appendChild(facts);
    li.appendChild(cue);

    li.addEventListener('click', function () {
      if (typeof onOpen === 'function') {
        onOpen(id);
      }
    });
    li.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.preventDefault();
        if (typeof onOpen === 'function') {
          onOpen(id);
        }
      }
    });

    return {
      el: li, title: title, meta: meta, facts: facts, trust: trust, outcome: outcome,
      firstCount: counts[0], secondCount: counts[1]
    };
  }

  // buildPastMeta is the grey line of a finished card: the category word (or
  // Approximate location for a finished Rescue report) and then who closed it
  // and when, or when it faded and after how long. The closed and lifetime
  // phrases are the server's; the browser only joins them to the relative
  // time of the server's own timestamp.
  function buildPastMeta(report) {
    var history = report.history;
    var lead = report.approximate === true ? APPROXIMATE_LABEL : Pinalert.categoryLabel(report.category);
    var clause = '';
    if (history) {
      var when = history.at ? Pinalert.relativeTime(history.at) : '';
      if (history.state === 'resolved') {
        clause = history.closed_text || '';
        if (when !== '') {
          clause = clause !== '' ? clause + ', ' + when : when;
        }
      } else {
        clause = when !== '' ? 'Faded ' + when : 'Faded';
        if (history.lifetime_text) {
          clause += ', ' + history.lifetime_text;
        }
      }
    }
    return clause === '' ? lead : lead + SEPARATOR + clause;
  }

  // updatePastCard writes the three lines of a card, text only. The outcome
  // label of an Expired card starts the facts line in a heavier span, then the
  // two counters follow as the server sent them: a corroborated report leaves
  // its confirm count to the label. A Resolved card has no label, only the
  // counters. The description of an approximate (finished Rescue) report is
  // never read here (D-23).
  function updatePastCard(card, report) {
    var history = report.history;
    var approximate = report.approximate === true;
    Pinalert.setText(card.title, approximate ? FINISHED_RESCUE_TITLE : report.description);
    Pinalert.setText(card.meta, buildPastMeta(report));

    // The counters: the row's trust line, which hides itself when the trust
    // object is unusable (it never invents a zero). Its currency word belongs
    // to a live report and is not shown on a finished one. The outcome label
    // keeps the line visible without the counters.
    PinalertTrust.updateTrustLine(card.trust, report.trust);
    card.trust.word.hidden = true;
    var countsShown = !card.facts.hidden;
    var hasLabel = !!history && history.state === 'expired' && !!history.outcome_label;
    Pinalert.setText(card.outcome, hasLabel ? history.outcome_label : '');
    card.outcome.hidden = !hasLabel;
    card.firstCount.hidden = !countsShown || (hasLabel && history.outcome === 'corroborated');
    card.secondCount.hidden = !countsShown;
    card.facts.hidden = !(countsShown || hasLabel);

    var selectedId = Pinalert.state.selectedId;
    var isSelected = selectedId !== null && selectedId !== undefined &&
      String(selectedId) === String(report.id);
    card.el.classList.toggle('past-card--selected', isSelected);
  }

  // cutLineText is the one cap line of both groups (C13): the number is the
  // count of cards the group really shows, never a number from elsewhere.
  function cutLineText(count) {
    return 'Showing the newest ' + count + '.';
  }

  // groupLineText is the text under a group. A cut group keeps the true cap
  // line, a partial group says it is partial, and a group that is both says
  // both, each sentence for its own fact.
  function groupLineText(count, wasCut, wasPartial) {
    var parts = [];
    if (wasCut && count > 0) {
      parts.push(cutLineText(count));
    }
    if (wasPartial) {
      parts.push(PARTIAL_LINE_TEXT);
    }
    return parts.join(' ');
  }

  // placeInList puts a card at its slot, keeping focus where it was: moveBefore
  // keeps it where the browser has the method, and otherwise the focused
  // element inside the moved card is focused again afterwards.
  function placeInList(listEl, el, expected) {
    if (expected === el) {
      return;
    }
    if (el.parentNode === listEl && typeof listEl.moveBefore === 'function') {
      listEl.moveBefore(el, expected || null);
      return;
    }
    var active = document.activeElement;
    var focused = active && el.contains(active) ? active : null;
    listEl.insertBefore(el, expected || null);
    if (focused && focused.isConnected && document.activeElement !== focused) {
      focused.focus();
    }
  }

  return {
    FINISHED_RESCUE_TITLE: FINISHED_RESCUE_TITLE,
    APPROXIMATE_LABEL: APPROXIMATE_LABEL,
    PARTIAL_LINE_TEXT: PARTIAL_LINE_TEXT,
    createPastCard: createPastCard,
    buildPastMeta: buildPastMeta,
    updatePastCard: updatePastCard,
    cutLineText: cutLineText,
    groupLineText: groupLineText,
    placeInList: placeInList
  };
}());
