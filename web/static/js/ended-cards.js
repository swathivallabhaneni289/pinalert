// ended-cards.js - window.PinalertEnded: the row builders and the word helpers
// of a finished report (Resolved or Expired). The Past card builders moved here
// out of feed.js in Phase 3.4 when the Past tab left Nearby (D-18), and plan 14
// reshaped them into the My activity row (D-20, D-21): a disc, the Report cell,
// the Status chip, What happened, When and Distance. The words themselves are
// the Phase 3.1 Past words, unchanged.
//
// A row is read only apart from one control: the take back of a mistaken Mark
// resolved, which only the person who marked it sees (03.1 D-22, 03.4 plan 19),
// mounted by votes.js's one builder. The row holds no link, and the whole row is
// not a tab stop: nothing opens from it, like every other row of My activity.
// This file fetches nothing, reads no store rows and computes no trust figure: the
// counters, the outcome label and the closed and lifetime phrases are the
// server's, written through Pinalert.setText.
//
// SECURITY (T-01-03): report text is authored by strangers. Every value
// reaches the DOM through Pinalert.setText, a text node assignment, never a
// string a browser then parses as markup. A finished Rescue report arrives with
// an empty description and a blurred point, and its row never reads a
// description on top of that (D-23, T-34-61).
window.PinalertEnded = (function () {
  'use strict';

  // The words a finished Rescue row uses in place of the report's own text and
  // category word (C17, C18). The server sends such a report with an empty
  // description and a blurred point, and the row never reads the description of
  // an approximate report on top of that (D-23).
  var FINISHED_RESCUE_TITLE = 'Rescue needed';
  var APPROXIMATE_LABEL = 'Approximate location';

  // The two Status chip words of a finished report.
  var STATUS_RESOLVED = 'Resolved';
  var STATUS_EXPIRED = 'Expired';

  // The word of a Distance cell when nothing is known (D-20).
  var UNKNOWN_DISTANCE = 'Unknown';

  // SEPARATOR is the app's own middle dot between the parts of a line.
  var SEPARATOR = ' · ';

  // The one line of a partial group: the server could not read every report of
  // the area for this group, so some are not shown (WR-02). It is its own flag
  // and its own words, and it never borrows the cap line: "Showing the newest
  // 25." says more than 25 were kept, which a partial flag does not say.
  var PARTIAL_LINE_TEXT = 'This area has more reports than this list can hold, so some are not shown.';

  // kmWords is the Distance cell's words for a number of kilometres: one decimal
  // under 10 km, a whole number from 10 km and "under 100 m" below a tenth of a
  // kilometre, the rounding of Pinalert.formatDistance without the word away.
  // A missing or invalid value reads Unknown, so nothing is invented. It is for
  // display only and never an input to any trust figure.
  function kmWords(km) {
    if (typeof km !== 'number' || !isFinite(km) || km < 0) {
      return UNKNOWN_DISTANCE;
    }
    if (km < 0.1) {
      return 'under 100 m';
    }
    if (km < 9.95) {
      return km.toFixed(1) + ' km';
    }
    return Math.round(km) + ' km';
  }

  // distanceWords is the Distance cell of a finished report: the server's own
  // distance_km. For an approximate (finished Rescue) report the number was
  // measured to the blurred point, so it is worded "about {n} km", a whole
  // number of at least 1 (D-23), the same words the Nearby rows use.
  function distanceWords(report) {
    var km = report ? report.distance_km : null;
    if (typeof km !== 'number' || !isFinite(km) || km < 0) {
      return UNKNOWN_DISTANCE;
    }
    if (report.approximate === true) {
      return 'about ' + Math.max(1, Math.round(km)) + ' km';
    }
    return kmWords(km);
  }

  // createEndedRow builds the empty row of one finished report in the My
  // activity row shape. updateEndedRow fills it. The facts line is the trust
  // line the list rows use, from the one builder, so its numerals and words are
  // the server's and this file names none of the trust fields.
  function createEndedRow(id) {
    var li = document.createElement('li');
    li.className = 'report-row activity-card activity-card--ended';
    li.dataset.reportId = String(id);

    var badge = document.createElement('span');
    badge.className = 'icon-badge activity-card__icon';
    badge.setAttribute('aria-hidden', 'true');
    var glyph = document.createElement('span');
    glyph.className = 'icon-glyph';
    badge.appendChild(glyph);

    var report = document.createElement('div');
    report.className = 'activity-card__report';
    var title = document.createElement('p');
    title.className = 'report-row__title';
    var sub = document.createElement('p');
    sub.className = 'report-row__meta activity-card__sub';
    report.appendChild(title);
    report.appendChild(sub);

    var status = document.createElement('div');
    status.className = 'activity-card__status';
    var severity = document.createElement('span');
    severity.className = 'visually-hidden';
    var chip = document.createElement('span');
    chip.className = 'activity-chip';
    status.appendChild(severity);
    status.appendChild(chip);

    var what = document.createElement('div');
    what.className = 'activity-card__what';
    var happened = document.createElement('p');
    happened.className = 'activity-card__happened';
    var trust = PinalertTrust.createTrustLine();
    var facts = trust.el;
    facts.classList.add('activity-card__facts');
    var counts = facts.querySelectorAll('.report-row__trustline-count');
    what.appendChild(happened);
    what.appendChild(facts);

    var when = document.createElement('p');
    when.className = 'activity-card__when';
    var distance = document.createElement('p');
    distance.className = 'activity-card__distance';

    li.appendChild(badge);
    li.appendChild(report);
    li.appendChild(status);
    li.appendChild(what);
    li.appendChild(when);
    li.appendChild(distance);

    return {
      el: li, what: what, takeBack: null, glyph: glyph, title: title, sub: sub, severity: severity, chip: chip,
      happened: happened, facts: facts, trust: trust, firstCount: counts[0], secondCount: counts[1],
      when: when, distance: distance
    };
  }

  // whenWords is the When cell: when it was resolved, or when it faded and
  // after how long. The relative time is of the server's own timestamp and the
  // lifetime phrase is the server's.
  function whenWords(report) {
    var history = report.history;
    if (!history) {
      return '';
    }
    var rel = history.at ? Pinalert.relativeTime(history.at) : '';
    if (history.state === 'resolved') {
      return rel !== '' ? 'Resolved ' + rel : 'Resolved';
    }
    var text = rel !== '' ? 'Faded ' + rel : 'Faded';
    if (history.lifetime_text) {
      text += ', ' + history.lifetime_text;
    }
    return text;
  }

  // happenedWords is the What happened sentence, the server's own words: who
  // closed a resolved report, or the outcome label of one that faded out. A
  // report with neither has no sentence and its counters stand alone.
  function happenedWords(report) {
    var history = report.history;
    if (!history) {
      return '';
    }
    if (history.state === 'resolved') {
      return history.closed_text || '';
    }
    return history.outcome_label || '';
  }

  // readEndedAgain reads the Ended nearby answer again after a take back and
  // resolves true when it did, false when it could not. The read is
  // ended-nearby.js's own (this file fetches nothing); it never rejects.
  function readEndedAgain(reportId) {
    var part = window.PinalertEndedNearby;
    if (!part || typeof part.refresh !== 'function') {
      return Promise.resolve(false);
    }
    return part.refresh(reportId);
  }

  // mountTakeBack puts the take back on the last line of the What happened
  // cell for the person who marked this report resolved, and nothing for anyone
  // else (03.1 D-22, 03.4 D-21, UI-SPEC Surface 3: rows are read only apart from
  // the take back). Whether this viewer has one, and how many undos are left,
  // is the server's answer in the report (your_resolution, undos_left) read by
  // the one builder in votes.js; this file holds no limit and sends nothing.
  function mountTakeBack(card, report) {
    if (card.takeBack) {
      card.takeBack.update(report);
      return;
    }
    if (!window.PinalertVotes || typeof PinalertVotes.createTakeBackFor !== 'function') {
      return;
    }
    var made = PinalertVotes.createTakeBackFor(report, {
      readAgain: function () {
        return readEndedAgain(report.id);
      }
    });
    if (made) {
      card.takeBack = made;
      card.what.appendChild(made.el);
    }
  }

  // updateEndedRow writes a row, text only. The state classes follow the Status
  // chip: a resolved row takes the Low tint and an expired one the grey chip and
  // disc, both from activity.css, with the severity class for the disc. The
  // description of an approximate (finished Rescue) report is never read here
  // (D-23). The confirm counter is left to the label when the report was
  // corroborated, as on the old Past card.
  function updateEndedRow(card, report) {
    var history = report.history;
    var resolved = !!history && history.state === 'resolved';
    var approximate = report.approximate === true;

    card.el.className = 'report-row activity-card activity-card--ended ' +
      Pinalert.severityClass(report) + ' ' + (resolved ? 'vis-retracted' : 'vis-expired');
    card.glyph.className = Pinalert.iconClass(report.category);

    var severity = Pinalert.SEVERITY_WORDS[report.severity] || Pinalert.SEVERITY_WORDS.low;
    Pinalert.setText(card.severity, severity + ' severity. ');
    Pinalert.setText(card.chip, resolved ? STATUS_RESOLVED : STATUS_EXPIRED);

    Pinalert.setText(card.title, approximate ? FINISHED_RESCUE_TITLE : report.description);
    var lead = approximate ? APPROXIMATE_LABEL : Pinalert.categoryLabel(report.category);
    var written = Pinalert.writtenLine(report);
    Pinalert.setText(card.sub, written !== '' ? lead + SEPARATOR + written : lead);

    var sentence = happenedWords(report);
    Pinalert.setText(card.happened, sentence);
    card.happened.hidden = sentence === '';

    // The counters: the row's trust line, which hides itself when the trust
    // object is unusable (it never invents a zero). Its currency word belongs
    // to a live report and is not shown on a finished one.
    PinalertTrust.updateTrustLine(card.trust, report.trust);
    card.trust.word.hidden = true;
    var countsShown = !card.facts.hidden;
    var corroborated = !!history && history.state === 'expired' && history.outcome === 'corroborated' &&
      !!history.outcome_label;
    card.firstCount.hidden = !countsShown || corroborated;
    card.secondCount.hidden = !countsShown;
    card.facts.hidden = !countsShown;

    Pinalert.setText(card.when, whenWords(report));
    Pinalert.setText(card.distance, distanceWords(report));
    mountTakeBack(card, report);
  }

  // cutLineText is the one cap line of both groups (C13): the number is the
  // count of rows the group really shows, never a number from elsewhere.
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

  return {
    FINISHED_RESCUE_TITLE: FINISHED_RESCUE_TITLE,
    APPROXIMATE_LABEL: APPROXIMATE_LABEL,
    PARTIAL_LINE_TEXT: PARTIAL_LINE_TEXT,
    UNKNOWN_DISTANCE: UNKNOWN_DISTANCE,
    kmWords: kmWords,
    distanceWords: distanceWords,
    createEndedRow: createEndedRow,
    updateEndedRow: updateEndedRow,
    whenWords: whenWords,
    happenedWords: happenedWords,
    cutLineText: cutLineText,
    groupLineText: groupLineText
  };
}());
