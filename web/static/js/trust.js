// trust.js: window.PinalertTrust, the shared trust block. One builder serves
// the feed row and the map popup (D-20), so a report reads the same on both
// surfaces: two counters that are always shown, an optional "still current"
// word and an optional reporter tag, and a panel of plain sentences that
// explains them.
//
// This module computes nothing the server did not send (T-02-04). The two
// counter numerals, every word and every panel sentence are authored by the
// server and rendered here as they arrive. There is no cap, no count, no
// score cut and no date arithmetic in this file. A vote changes a number only
// through the refetch votes.js already runs (A33).
//
// SECURITY (T-03-06): every string reaching the DOM goes through
// Pinalert.setText, a text node assignment, never a markup parsing sink. No
// report text enters the block. Class names are fixed literals and never come
// from the payload, and the closed lists below gate every enum the server
// sends.
//
// The fallback direction: a trust object that fails validation hides the
// whole block. It never invents a "0 confirmed", because a made up zero would
// state a fact nobody checked. An unknown word or tag renders nothing, and an
// unknown fade stage falls back to the time stage.

(function () {
  'use strict';

  // The closed lists the server's values are checked against. They equal the
  // server enums, in the outline's order.
  var FADE_STAGES = ['fresh', 'aging', 'stale'];
  var CURRENCY_BANDS = [
    'up_to_date',
    'getting_old',
    'needs_reconfirming',
    'not_yet_corroborated',
    'not_applicable'
  ];
  var REPORTER_TAGS = ['reliable', 'mixed', 'unreliable', 'new', 'none'];

  // The panel lines in their fixed order: the lead sentence, the confirmed
  // sentence, the disputed sentence, the still current sentence and its
  // score sentence, then the reporter sentence and its score sentence.
  var PANEL_LINES = ['lead', 'confirmed', 'disputed', 'currency', 'currency_score', 'reporter', 'reporter_score'];

  // A module counter gives every panel its id, never the report id, because
  // one report renders in both the list and the popup.
  var trustPanelSeq = 0;

  function isNonEmptyString(v) {
    return typeof v === 'string' && v.length > 0;
  }

  // validTrust is true only for an object carrying the fields the block
  // cannot render without. A missing currency or reporter is not a failure:
  // the block simply shows no second line.
  function validTrust(trust) {
    if (!trust || typeof trust !== 'object') {
      return false;
    }
    return Number.isInteger(trust.confirmed_places) && trust.confirmed_places >= 0 &&
      Number.isInteger(trust.disputed_places) && trust.disputed_places >= 0 &&
      isNonEmptyString(trust.confirmed_text) &&
      isNonEmptyString(trust.disputed_text) &&
      typeof trust.lead_sentence === 'string' &&
      typeof trust.confirmed_sentence === 'string' &&
      typeof trust.disputed_sentence === 'string';
  }

  // signalState says which of the two signals shows. The server decides the
  // words, so a Provisional or Hidden row gets the reporter tag only (R-03,
  // D-18), and a reporter tag of none draws nothing (A47).
  function signalState(trust) {
    var cur = trust.currency;
    var rep = trust.reporter;
    var currencyShown = !!(cur && typeof cur === 'object' &&
      CURRENCY_BANDS.indexOf(cur.band) !== -1 &&
      isNonEmptyString(cur.label));
    var reporterShown = !!(rep && typeof rep === 'object' &&
      REPORTER_TAGS.indexOf(rep.tag) !== -1 &&
      rep.tag !== 'none' &&
      isNonEmptyString(rep.label));
    return { currency: currencyShown, reporter: reporterShown };
  }

  // surfaceIsDisplayed is true when the element takes part in layout, so
  // focus is never sent into a hidden pane.
  function surfaceIsDisplayed(el) {
    return !!el && el.getClientRects().length > 0;
  }

  // createSignal builds one signal: an icon, then the word, then (popup only)
  // the number behind a visually hidden ", score" prefix. The number sits
  // after its word with no icon of its own (R-07). Everything starts hidden.
  function createSignal(iconClass, extraClass, showScores) {
    var el = document.createElement('span');
    el.className = extraClass ? 'trust-signal ' + extraClass : 'trust-signal';
    el.hidden = true;

    var icon = document.createElement('span');
    icon.className = iconClass;
    icon.setAttribute('aria-hidden', 'true');
    el.appendChild(icon);

    var text = document.createElement('span');
    text.className = 'trust-signal__text';
    var word = document.createElement('span');
    text.appendChild(word);

    var score = null;
    var num = null;
    if (showScores) {
      score = document.createElement('span');
      score.hidden = true;
      var prefix = document.createElement('span');
      prefix.className = 'visually-hidden';
      Pinalert.setText(prefix, ', score');
      num = document.createElement('span');
      num.className = 'trust-num';
      score.appendChild(prefix);
      score.appendChild(document.createTextNode(' '));
      score.appendChild(num);
      text.appendChild(score);
    }
    el.appendChild(text);

    return { el: el, word: word, score: score, num: num };
  }

  // createTrustBlock is the one builder for every surface (D-20). Only the
  // popup asks for scores (D-09). The block starts hidden and empty, and
  // updateTrustBlock is always called after it.
  function createTrustBlock(options) {
    var showScores = !!(options && options.showScores === true);

    var block = document.createElement('div');
    block.className = 'trust-block';
    block.hidden = true;

    trustPanelSeq += 1;
    var panelId = 'trust-panel-' + trustPanelSeq;

    var toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'trust-block__toggle';
    toggle.setAttribute('aria-expanded', 'false');
    toggle.setAttribute('aria-controls', panelId);

    var counts = document.createElement('span');
    counts.className = 'trust-block__counts';
    var first = document.createElement('span');
    first.className = 'trust-block__count';
    var confirmedNum = document.createElement('span');
    confirmedNum.className = 'trust-num';
    first.appendChild(confirmedNum);
    first.appendChild(document.createTextNode(' confirmed'));
    var second = document.createElement('span');
    second.className = 'trust-block__count trust-block__count--second';
    var disputedNum = document.createElement('span');
    disputedNum.className = 'trust-num';
    second.appendChild(disputedNum);
    second.appendChild(document.createTextNode(' disputed'));
    counts.appendChild(first);
    counts.appendChild(document.createTextNode(' '));
    counts.appendChild(second);

    var signals = document.createElement('span');
    signals.className = 'trust-block__signals';
    signals.hidden = true;
    var currency = createSignal('trust-icon trust-icon--clock', '', showScores);
    var reporter = createSignal('trust-icon trust-icon--user', 'trust-signal--second', showScores);
    signals.appendChild(document.createTextNode(' '));
    signals.appendChild(currency.el);
    signals.appendChild(document.createTextNode(' '));
    signals.appendChild(reporter.el);

    var hint = document.createElement('span');
    hint.className = 'visually-hidden';
    Pinalert.setText(hint, '. Shows what these mean.');

    toggle.appendChild(counts);
    toggle.appendChild(signals);
    toggle.appendChild(hint);

    var panel = document.createElement('div');
    panel.className = 'trust-block__panel';
    panel.id = panelId;
    panel.hidden = true;
    var lines = {};
    for (var i = 0; i < PANEL_LINES.length; i++) {
      var line = document.createElement('p');
      line.className = 'trust-block__line';
      line.hidden = true;
      panel.appendChild(line);
      lines[PANEL_LINES[i]] = line;
    }

    block.appendChild(toggle);
    block.appendChild(panel);

    var instance = {
      el: block,
      toggle: toggle,
      panel: panel,
      confirmedNum: confirmedNum,
      disputedNum: disputedNum,
      signals: signals,
      currency: currency,
      reporter: reporter,
      lines: lines,
      showScores: showScores
    };

    toggle.addEventListener('click', function () {
      setPanelOpen(instance, toggle.getAttribute('aria-expanded') !== 'true');
    });
    block.addEventListener('click', function (evt) {
      evt.stopPropagation();
    });
    // Enter and Space reach the native button, which then fires its click.
    // The root stops them so the row's own key handler and the map never see
    // them. It never prevents the default, or the click would not fire.
    block.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.stopPropagation();
      }
    });

    return instance;
  }

  // setPanelOpen is the only writer of aria-expanded and of the panel's
  // hidden attribute, and it opens or closes the panel at once, with no
  // animation.
  function setPanelOpen(block, open) {
    block.toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    block.panel.hidden = !open;
  }

  // renderTrustCounts writes the server's two numerals, both always, zeros
  // included (D-16, A27).
  function renderTrustCounts(block, trust) {
    Pinalert.setText(block.confirmedNum, trust.confirmed_text);
    Pinalert.setText(block.disputedNum, trust.disputed_text);
  }

  // renderTrustSignal shows or hides one signal and writes its word. On the
  // popup it also shows the number, but only for an integer score.
  function renderTrustSignal(signal, shown, label, score) {
    signal.el.hidden = !shown;
    if (shown) {
      Pinalert.setText(signal.word, label);
    }
    if (signal.score) {
      var showNum = shown && Number.isInteger(score) && score >= 0;
      signal.score.hidden = !showNum;
      if (showNum) {
        Pinalert.setText(signal.num, String(score));
      }
    }
  }

  // fillLine writes one panel line and hides it when the server sent no
  // sentence for it.
  function fillLine(block, key, value, allowed) {
    var line = block.lines[key];
    var show = allowed && isNonEmptyString(value);
    line.hidden = !show;
    Pinalert.setText(line, show ? value : '');
  }

  // renderTrustPanel fills the panel from the server's sentences, in the
  // fixed order (D-19 as amended by A5, A6, A50). A sentence for a signal
  // that does not show stays hidden, and a score sentence shows only where
  // the score does, which is the popup alone.
  function renderTrustPanel(block, trust) {
    var state = signalState(trust);
    var cur = state.currency ? trust.currency : null;
    var rep = state.reporter ? trust.reporter : null;

    fillLine(block, 'lead', trust.lead_sentence, true);
    fillLine(block, 'confirmed', trust.confirmed_sentence, true);
    fillLine(block, 'disputed', trust.disputed_sentence, true);
    fillLine(block, 'currency', cur && cur.sentence, !!cur);
    fillLine(block, 'currency_score', cur && cur.score_sentence,
      !!cur && block.showScores && Number.isInteger(cur.score));
    fillLine(block, 'reporter', rep && rep.sentence, !!rep);
    fillLine(block, 'reporter_score', rep && rep.score_sentence,
      !!rep && block.showScores && Number.isInteger(rep.score));
  }

  // updateTrustBlock is the one place server state reaches the block. It
  // runs again on every poll, so it never touches the open state of the
  // panel, and an open panel stays open.
  function updateTrustBlock(block, trust) {
    if (!block) {
      return;
    }
    if (!validTrust(trust)) {
      block.el.hidden = true;
      return;
    }
    var state = signalState(trust);
    renderTrustCounts(block, trust);
    renderTrustSignal(block.currency, state.currency,
      state.currency ? trust.currency.label : '',
      state.currency ? trust.currency.score : null);
    renderTrustSignal(block.reporter, state.reporter,
      state.reporter ? trust.reporter.label : '',
      state.reporter ? trust.reporter.score : null);
    block.signals.hidden = !(state.currency || state.reporter);
    renderTrustPanel(block, trust);
    block.el.hidden = false;
  }

  // fadeStage returns the worse of the time stage and the server's stage, in
  // the order fresh, aging, stale (D-07, R-02). The server sends no stage for
  // a critical or rescue needed row, and the time stage then stands alone.
  function fadeStage(report) {
    var timeStage = Pinalert.ageStage(report);
    var trust = report && report.trust;
    var currency = trust && trust.currency;
    var server = currency && currency.fade_stage;
    var serverIndex = FADE_STAGES.indexOf(server);
    if (serverIndex === -1) {
      return timeStage;
    }
    var timeIndex = FADE_STAGES.indexOf(timeStage);
    return serverIndex > timeIndex ? FADE_STAGES[serverIndex] : timeStage;
  }

  // focusToggle focuses a block's toggle when it is on screen and says
  // whether it did. It accepts a block instance or an element holding one.
  function focusToggle(blockOrRow) {
    var toggle = null;
    if (blockOrRow && blockOrRow.toggle) {
      toggle = blockOrRow.toggle;
    } else if (blockOrRow && typeof blockOrRow.querySelector === 'function') {
      toggle = blockOrRow.querySelector('.trust-block__toggle');
    }
    if (!toggle || !surfaceIsDisplayed(toggle)) {
      return false;
    }
    toggle.focus();
    return true;
  }

  window.PinalertTrust = {
    createTrustBlock: createTrustBlock,
    updateTrustBlock: updateTrustBlock,
    fadeStage: fadeStage,
    focusToggle: focusToggle
  };
}());
