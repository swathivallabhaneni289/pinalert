// trust.js: window.PinalertTrust, the shared trust block. The feed row and the
// Activity row get a compact toggle (two counters that are always shown, an
// optional "still current" word) whose panel holds the facts list. The map
// popup gets the trust card, which is always showing and holds the same facts
// list under a header of counters and the state word (A90, A91). One list
// builder serves both (D-20), so a report reads the same everywhere. The
// reporter words of a row sit outside the button, and each row consumer places
// them with the state label through createStateLine (A75).
//
// This module computes nothing the server did not send (T-02-04). The two
// counter numerals, the state word and every fact row (title, detail and
// score detail) are authored by the server in trust.facts and rendered here
// as they arrive. There is no cap, no count, no score cut, no wording of its
// own and no date arithmetic in this file. A vote changes a number only
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

  // The kinds of fact row in the server's order, which is also the order of
  // the slots the list builds, and the tones a row can carry (A89).
  var FACT_KINDS = ['status', 'confirmations', 'disputes', 'currency', 'reporter'];
  var FACT_TONES = ['good', 'neutral', 'caution', 'bad'];

  // The tone glyphs (A92): a disc and one mark per tone, drawn in a square
  // box. The marks are four different shapes so a tone is never colour alone.
  // This table is the only place in this file that holds numbers other than 0
  // and 1, and they are geometry, not thresholds.
  var TONE_GLYPHS = {
    ns: 'http://www.w3.org/2000/svg',
    viewBox: '0 0 24 24',
    disc: { cx: '12', cy: '12', r: '10.5' },
    marks: {
      good: 'M7.5 12.3l3 3.2 6-6.6',
      neutral: 'M8 12h8',
      caution: 'M12 7v6M12 16.5h.01',
      bad: 'M8.5 8.5l7 7M15.5 8.5l-7 7'
    }
  };

  // A module counter gives every panel its id, never the report id, because
  // one report renders in both the list and the popup.
  var trustPanelSeq = 0;

  function isNonEmptyString(v) {
    return typeof v === 'string' && v.length > 0;
  }

  // validFacts is true only for an ordered array the list can draw without
  // guessing: an object per fact, its kind and tone from the closed lists,
  // kinds in the fixed order with none repeated, a title and both detail
  // lines as strings (an empty detail is allowed).
  function validFacts(facts) {
    if (!Array.isArray(facts)) {
      return false;
    }
    var last = -1;
    for (var i = 0; i < facts.length; i++) {
      var fact = facts[i];
      if (!fact || typeof fact !== 'object') {
        return false;
      }
      var at = FACT_KINDS.indexOf(fact.kind);
      if (at === -1 || at <= last || FACT_TONES.indexOf(fact.tone) === -1) {
        return false;
      }
      last = at;
      if (!isNonEmptyString(fact.title) || typeof fact.detail !== 'string' ||
          typeof fact.score_detail !== 'string') {
        return false;
      }
    }
    return true;
  }

  // validTrust is true only for an object carrying the fields the block
  // cannot render without: the two counters and a valid facts array. A
  // missing currency or reporter is not a failure: the block simply shows no
  // second line, and a facts array without those kinds shows no such row.
  function validTrust(trust) {
    if (!trust || typeof trust !== 'object') {
      return false;
    }
    return Number.isInteger(trust.confirmed_places) && trust.confirmed_places >= 0 &&
      Number.isInteger(trust.disputed_places) && trust.disputed_places >= 0 &&
      isNonEmptyString(trust.confirmed_text) &&
      isNonEmptyString(trust.disputed_text) &&
      validFacts(trust.facts);
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

  // createSignal builds one signal: an icon, then the word. The score numbers
  // are gone from the signals (A91): score_detail rows of the card carry them.
  // Everything starts hidden.
  function createSignal(iconClass, extraClass) {
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
    el.appendChild(text);

    return { el: el, word: word };
  }

  // createReporterWords builds the reporter words as plain text outside the
  // button (A75), with no icon (A76) and no number (A91). Everything starts
  // hidden.
  function createReporterWords() {
    var el = document.createElement('span');
    el.className = 'trust-reporter';
    el.hidden = true;

    var word = document.createElement('span');
    el.appendChild(word);

    return { el: el, word: word };
  }

  // createSeverityLabel builds the read only severity word shared by the feed
  // row, the Activity row and the popup (A26, A45). It starts hidden and
  // empty, carries no aria attribute and takes no focus, and
  // updateSeverityLabel is always called after it.
  function createSeverityLabel() {
    var label = document.createElement('span');
    label.className = 'severity-label';
    label.setAttribute('hidden', '');
    return label;
  }

  // updateSeverityLabel writes the class and the word only. The word comes
  // from the fixed table in app.js and the value reaches the class name only
  // when it is one of that table's own keys, so an unexpected value never
  // reads as a severity (T-03-06).
  function updateSeverityLabel(label, report) {
    if (!label) {
      return;
    }
    var severity = report && report.severity;
    if (Object.prototype.hasOwnProperty.call(Pinalert.SEVERITY_WORDS, severity)) {
      label.className = 'severity-label severity-label--' + severity;
      Pinalert.setText(label, Pinalert.SEVERITY_WORDS[severity]);
      label.removeAttribute('hidden');
      return;
    }
    label.className = 'severity-label';
    Pinalert.setText(label, '');
    label.setAttribute('hidden', '');
  }

  // createTrustBlock is the row builder, for the feed row and the Activity row
  // (D-20). It shows no score numbers (D-09). The block starts hidden and
  // empty, and updateTrustBlock is always called after it.
  function createTrustBlock() {
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
    var currency = createSignal('trust-icon trust-icon--clock', '');
    var reporter = createReporterWords();
    signals.appendChild(document.createTextNode(' '));
    signals.appendChild(currency.el);

    var hint = document.createElement('span');
    hint.className = 'visually-hidden';
    Pinalert.setText(hint, '. Shows what these mean.');

    // The face holds the counts and the signals so the chevron can move under
    // the text at a large text size (A79). The chevron is the visual cue that
    // the block opens; it is hidden from screen readers because the hint
    // already says it (A78).
    var face = document.createElement('span');
    face.className = 'trust-block__face';
    face.appendChild(counts);
    face.appendChild(signals);

    var chevron = document.createElement('span');
    chevron.className = 'trust-block__chevron';
    chevron.setAttribute('aria-hidden', 'true');

    toggle.appendChild(face);
    toggle.appendChild(chevron);
    toggle.appendChild(hint);

    var panel = document.createElement('div');
    panel.className = 'trust-block__panel';
    panel.id = panelId;
    panel.hidden = true;
    var facts = createFactsList(false);
    panel.appendChild(facts.el);

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
      facts: facts
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

  // createStateLine is the one placement helper for the line that holds the
  // state label and the reporter words (D-20, A75, A80). Each consumer passes
  // a fixed class literal it owns. The space keeps label and words two words
  // for a screen reader and is not rendered between flex items. Script never
  // hides the line: the stylesheet removes it when every child is hidden (A83).
  function createStateLine(className, tag, block) {
    var line = document.createElement('div');
    line.className = className;
    if (tag) {
      line.appendChild(tag);
    }
    line.appendChild(document.createTextNode(' '));
    if (block && block.reporter) {
      line.appendChild(block.reporter.el);
    }
    return line;
  }

  // setPanelOpen is the only writer of aria-expanded and of the panel's
  // hidden attribute, and it opens or closes the panel at once, with no
  // animation.
  function setPanelOpen(block, open) {
    block.toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    block.panel.hidden = !open;
  }

  // blockFromToggle rebuilds the two parts setPanelOpen needs from a toggle
  // found in the DOM, using the panel id the toggle already points at.
  function blockFromToggle(toggle) {
    return { toggle: toggle, panel: document.getElementById(toggle.getAttribute('aria-controls')) };
  }

  // closeRowPanel closes the open panel of one displayed row, puts focus back
  // on its toggle and returns true. It returns false when the row has no open
  // panel.
  function closeRowPanel(row) {
    var toggle = row.querySelector('.trust-block__toggle');
    if (!toggle || toggle.getAttribute('aria-expanded') !== 'true') {
      return false;
    }
    setPanelOpen(blockFromToggle(toggle), false);
    toggle.focus();
    return true;
  }

  // openPopupEl returns the open popup container while it is shown, or null.
  // map.js writes the data attribute on open and removes it on close.
  function openPopupEl() {
    var el = document.querySelector('.leaflet-popup[data-popup-open="true"]');
    return surfaceIsDisplayed(el) ? el : null;
  }

  // isPopupMarker is true for the pin whose popup is open. It accepts a
  // missing element, because the active element can be absent.
  function isPopupMarker(el) {
    return !!el && !!el.classList && el.classList.contains('leaflet-marker-icon') &&
      el.getAttribute('data-popup-open') === 'true';
  }

  // closeOpenPopup presses the popup's own close button, so Leaflet closes it
  // and map.js returns focus to the opener. It says whether a button was found.
  function closeOpenPopup(popupEl) {
    var button = popupEl.querySelector('.leaflet-popup-close-button');
    if (!button) {
      return false;
    }
    button.click();
    return true;
  }

  // closePopupLevel closes the popup itself, and only when focus is in the
  // popup or on its pin (A23, A70). The popup holds no panel, so one Escape
  // closes it. Focus on the page body closes nothing here.
  function closePopupLevel(popupEl, mayClose) {
    if (!mayClose) {
      return false;
    }
    return closeOpenPopup(popupEl);
  }

  // closeOneLevel resolves one Escape level and returns true only when it
  // acted (A23, A70). The report modal and the account menu keep their own
  // Escape, so this stays out of their way while either is open. The owner is
  // the popup or row holding focus, or the open popup when focus is on its pin
  // or on the page body. An open confirmation goes first, then the owner's
  // open panel, then (for focus inside the popup or on its pin) the popup.
  function closeOneLevel() {
    var modal = document.getElementById('report-modal');
    if (modal && !modal.hidden) {
      return false;
    }
    var menu = document.getElementById('account-menu');
    if (menu && !menu.hidden) {
      return false;
    }

    var active = document.activeElement;
    var popupEl = openPopupEl();
    var owner = null;
    if (active && active !== document.body) {
      owner = active.closest('.report-row, .leaflet-popup');
    }
    if (!owner && popupEl && (!active || active === document.body || isPopupMarker(active))) {
      owner = popupEl;
    }
    if (!owner) {
      return false;
    }
    var focusInside = !!active && owner.contains(active);

    // An open Mark resolved confirmation goes first, by its Cancel path.
    if (focusInside && window.PinalertVotes && typeof PinalertVotes.cancelResolveConfirm === 'function' &&
        PinalertVotes.cancelResolveConfirm(owner)) {
      return true;
    }

    if (owner.classList.contains('leaflet-popup')) {
      return closePopupLevel(owner, focusInside || isPopupMarker(active));
    }

    // A row owns the panel only while its list pane is displayed, so focus
    // never moves into a hidden pane.
    if (owner.classList.contains('report-row') && surfaceIsDisplayed(owner)) {
      return closeRowPanel(owner);
    }
    return false;
  }

  // onDocumentEscape consumes the key only when a level closed.
  function onDocumentEscape(evt) {
    if (evt.key !== 'Escape') {
      return;
    }
    var acted = closeOneLevel();
    if (!acted) {
      return;
    }
    evt.stopPropagation();
    evt.preventDefault();
  }

  // renderTrustCounts writes the server's two numerals, both always, zeros
  // included (D-16, A27).
  function renderTrustCounts(block, trust) {
    Pinalert.setText(block.confirmedNum, trust.confirmed_text);
    Pinalert.setText(block.disputedNum, trust.disputed_text);
  }

  // renderTrustSignal shows or hides one signal and writes its word.
  function renderTrustSignal(signal, shown, label) {
    signal.el.hidden = !shown;
    if (shown) {
      Pinalert.setText(signal.word, label);
    }
  }

  // createToneGlyph builds one tone glyph with the DOM API: a disc and the
  // mark of the tone, aria-hidden because the row text carries the meaning
  // (A92). The colours come from classes in trust.css, never from here.
  function createToneGlyph(tone) {
    var svg = document.createElementNS(TONE_GLYPHS.ns, 'svg');
    svg.setAttribute('class', 'trust-fact__glyph trust-fact__glyph--' + tone);
    svg.setAttribute('viewBox', TONE_GLYPHS.viewBox);
    svg.setAttribute('aria-hidden', 'true');
    svg.setAttribute('focusable', 'false');

    var disc = document.createElementNS(TONE_GLYPHS.ns, 'circle');
    disc.setAttribute('class', 'trust-fact__disc');
    disc.setAttribute('cx', TONE_GLYPHS.disc.cx);
    disc.setAttribute('cy', TONE_GLYPHS.disc.cy);
    disc.setAttribute('r', TONE_GLYPHS.disc.r);
    svg.appendChild(disc);

    var mark = document.createElementNS(TONE_GLYPHS.ns, 'path');
    mark.setAttribute('class', 'trust-fact__mark');
    mark.setAttribute('d', TONE_GLYPHS.marks[tone]);
    svg.appendChild(mark);
    return svg;
  }

  // createFactsList builds the one list of tick rows (A89): a slot per kind
  // in the fixed order, all hidden and empty, filled by renderFacts. The
  // score line exists only when the caller asks for it (the card, A50). Fact
  // rows are divs inside list items so Leaflet's paragraph rules never apply.
  function createFactsList(showScoreDetail) {
    var list = document.createElement('ul');
    list.className = 'trust-facts';
    list.setAttribute('role', 'list');

    var slots = {};
    for (var i = 0; i < FACT_KINDS.length; i++) {
      var item = document.createElement('li');
      item.className = 'trust-fact trust-fact--' + FACT_KINDS[i];
      item.hidden = true;

      var text = document.createElement('div');
      text.className = 'trust-fact__text';
      var title = document.createElement('div');
      title.className = 'trust-fact__title';
      var detail = document.createElement('div');
      detail.className = 'trust-fact__detail';
      detail.hidden = true;
      text.appendChild(title);
      text.appendChild(detail);

      var score = null;
      if (showScoreDetail) {
        score = document.createElement('div');
        score.className = 'trust-fact__score';
        score.hidden = true;
        text.appendChild(score);
      }
      item.appendChild(text);
      list.appendChild(item);
      slots[FACT_KINDS[i]] = { el: item, text: text, glyph: null, tone: '', title: title, detail: detail, score: score };
    }
    return { el: list, slots: slots };
  }

  // fillFactLine writes one muted line of a row and hides it when the server
  // sent an empty string for it.
  function fillFactLine(line, value) {
    line.hidden = value.length === 0;
    Pinalert.setText(line, value);
  }

  // renderFacts fills the slots from a facts array that validFacts already
  // passed. A kind the array does not carry stays hidden. The glyph is swapped
  // only when the tone changed, so a poll rewrites text and nothing else.
  function renderFacts(list, facts) {
    var shown = {};
    for (var i = 0; i < facts.length; i++) {
      var fact = facts[i];
      var slot = list.slots[fact.kind];
      shown[fact.kind] = true;
      if (slot.tone !== fact.tone) {
        var glyph = createToneGlyph(fact.tone);
        if (slot.glyph) {
          slot.el.replaceChild(glyph, slot.glyph);
        } else {
          slot.el.insertBefore(glyph, slot.text);
        }
        slot.glyph = glyph;
        slot.tone = fact.tone;
      }
      Pinalert.setText(slot.title, fact.title);
      fillFactLine(slot.detail, fact.detail);
      if (slot.score) {
        fillFactLine(slot.score, fact.score_detail);
      }
      slot.el.hidden = false;
    }
    for (var k = 0; k < FACT_KINDS.length; k++) {
      if (!shown[FACT_KINDS[k]]) {
        list.slots[FACT_KINDS[k]].el.hidden = true;
      }
    }
  }

  // renderTrustPanel fills the row panel with the facts list (A91). The old
  // sentences are not read any more.
  function renderTrustPanel(block, trust) {
    renderFacts(block.facts, trust.facts);
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
      // The words live outside the block, so an invalid trust object hides
      // them too.
      block.reporter.el.hidden = true;
      return;
    }
    var state = signalState(trust);
    renderTrustCounts(block, trust);
    renderTrustSignal(block.currency, state.currency,
      state.currency ? trust.currency.label : '');
    renderTrustSignal(block.reporter, state.reporter,
      state.reporter ? trust.reporter.label : '');
    block.signals.hidden = !state.currency;
    renderTrustPanel(block, trust);
    block.el.hidden = false;
  }

  // currencyTone reads the tone of the currency fact, which colours the state
  // word of the card. No currency fact means the neutral look (A92).
  function currencyTone(facts) {
    for (var i = 0; i < facts.length; i++) {
      var fact = facts[i];
      if (fact.kind === 'currency') {
        return fact.tone;
      }
    }
    return '';
  }

  // createTrustCard is the popup builder (A90): one white card that is always
  // showing, a header of the two counters and the state word, then the facts
  // list with its score lines. It has no toggle, no chevron, no panel and no
  // expanded state, and nothing in it takes focus. The card starts hidden and
  // empty, and updateTrustCard is always called after it.
  function createTrustCard() {
    var el = document.createElement('div');
    el.className = 'trust-card';
    el.hidden = true;

    var header = document.createElement('div');
    header.className = 'trust-card__header';

    var counts = document.createElement('span');
    counts.className = 'trust-card__counts';
    var first = document.createElement('span');
    var confirmedNum = document.createElement('span');
    confirmedNum.className = 'trust-num';
    first.appendChild(confirmedNum);
    first.appendChild(document.createTextNode(' confirmed'));
    var dot = document.createElement('span');
    dot.className = 'trust-card__dot';
    dot.setAttribute('aria-hidden', 'true');
    Pinalert.setText(dot, '\u00b7');
    var second = document.createElement('span');
    var disputedNum = document.createElement('span');
    disputedNum.className = 'trust-num';
    second.appendChild(disputedNum);
    second.appendChild(document.createTextNode(' disputed'));
    // The spaces are real text nodes outside the hidden dot, so a screen
    // reader still hears two separate counters.
    counts.appendChild(first);
    counts.appendChild(document.createTextNode(' '));
    counts.appendChild(dot);
    counts.appendChild(document.createTextNode(' '));
    counts.appendChild(second);

    var state = document.createElement('span');
    state.className = 'trust-card__state';
    state.hidden = true;

    header.appendChild(counts);
    header.appendChild(state);

    var facts = createFactsList(true);
    el.appendChild(header);
    el.appendChild(facts.el);

    return { el: el, confirmedNum: confirmedNum, disputedNum: disputedNum, state: state, facts: facts };
  }

  // updateTrustCard is the one place server state reaches the card. The state
  // word is the server's still current label and its colour class comes from
  // the currency fact's tone, both through closed lists. It runs again on
  // every poll and only writes text and classes.
  function updateTrustCard(card, trust) {
    if (!card) {
      return;
    }
    if (!validTrust(trust)) {
      card.el.hidden = true;
      return;
    }
    renderTrustCounts(card, trust);

    var word = signalState(trust).currency ? trust.currency.label : '';
    var tone = currencyTone(trust.facts);
    card.state.hidden = word.length === 0;
    card.state.className = tone ? 'trust-card__state trust-card__state--' + tone : 'trust-card__state';
    Pinalert.setText(card.state, word);

    renderFacts(card.facts, trust.facts);
    card.el.hidden = false;
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

  // The one capture phase key listener for the whole page (A23).
  document.addEventListener('keydown', onDocumentEscape, true);

  window.PinalertTrust = {
    createSeverityLabel: createSeverityLabel,
    updateSeverityLabel: updateSeverityLabel,
    createTrustBlock: createTrustBlock,
    createTrustCard: createTrustCard,
    createStateLine: createStateLine,
    updateTrustBlock: updateTrustBlock,
    updateTrustCard: updateTrustCard,
    fadeStage: fadeStage,
    focusToggle: focusToggle
  };
}());
