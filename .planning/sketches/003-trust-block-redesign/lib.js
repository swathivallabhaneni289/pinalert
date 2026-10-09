// lib.js: the DOM builders for sketch 003. Mockup only, classic script, no network, no markup sinks:
// every string reaches the DOM through textContent or a text node, the same rule the app follows.
//
// Two designs are built from one report object:
//   current   mirrors the shipped markup exactly (trust.js createTrustBlock, map.js buildPopupCard, feed.js createRow,
//             profile.html.tmpl plus activity.js), so the baseline is today's design and not a redrawing of it.
//   redesign  the three new variants. The reporter words leave the button and join the top of the report, the button
//             keeps the two counts and the still current words, and a chevron marks it as a disclosure.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';

  function el(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) { node.className = cls; }
    if (text !== undefined && text !== null) { node.textContent = text; }
    return node;
  }
  function txt(s) { return document.createTextNode(s); }
  S3.el = el;
  S3.txt = txt;

  // ---- Icon masks ------------------------------------------------------------------------------------------
  // The app masks its icons through root relative urls such as /static/icons/clock.svg. That path does not resolve
  // when this page is opened from a repo root server or from a file, so the sketch redeclares the same masks as data
  // URIs, after the app stylesheets, with the same selector heads so they win on order. The Lucide paths are copied
  // from web/static/icons. The chevron is new: the app has no chevron-down.svg yet.
  var CATEGORY = {
    fire: '<path d="M12 3q1 4 4 6.5t3 5.5a1 1 0 0 1-14 0 5 5 0 0 1 1-3 1 1 0 0 0 5 0c0-2-1.5-3-1.5-5q0-2 2.5-4"/>',
    flood: '<path d="M12 22a7 7 0 0 0 7-7c0-2-1-3.9-3-5.5s-3.5-4-4-6.5c-.5 2.5-2 4.9-4 6.5C6 11.1 5 13 5 15a7 7 0 0 0 7 7z"/>',
    road_blocked: '<rect x="2" y="6" width="20" height="8" rx="1"/><path d="M17 14v7"/><path d="M7 14v7"/><path d="M17 3v3"/><path d="M7 3v3"/><path d="M10 14 2.3 6.3"/><path d="m14 6 7.7 7.7"/><path d="m8 6 8 8"/>',
    shelter_open: '<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
    rescue_needed: '<circle cx="12" cy="12" r="10"/><path d="m4.93 4.93 4.24 4.24"/><path d="m14.83 9.17 4.24-4.24"/><path d="m14.83 14.83 4.24 4.24"/><path d="m9.17 14.83-4.24 4.24"/><circle cx="12" cy="12" r="4"/>',
    power_outage: '<path d="M10.768 5.111 13.44 2.44a1.5 1.5 0 012.474 1.561l-1.633 4.625"/><path d="m18.889 13.232.672-.672A1.5 1.5 0 0018.5 10h-2.844"/><path d="m2 2 20 20"/><path d="m7.94 7.94-3.5 3.499A1.5 1.5 0 005.5 14h4.002a.5.5 0 01.471.666L8.086 20a1.5 1.5 0 002.475 1.56l5.5-5.5"/>',
    storm_cyclone: '<path d="M12.8 19.6A2 2 0 1 0 14 16H2"/><path d="M17.5 8a2.5 2.5 0 1 1 2 4H2"/><path d="M9.8 4.4A2 2 0 1 1 11 8H2"/>',
    earthquake: '<path d="M22 12h-2.48a2 2 0 0 0-1.93 1.46l-2.35 8.36a.25.25 0 0 1-.48 0L9.24 2.18a.25.25 0 0 0-.48 0l-2.35 8.36A2 2 0 0 1 4.49 12H2"/>',
    other: '<path d="M4 22V4a1 1 0 0 1 .4-.8A6 6 0 0 1 8 2c3 0 5 2 7.333 2q2 0 3.067-.8A1 1 0 0 1 20 4v10a1 1 0 0 1-.4.8A6 6 0 0 1 16 16c-3 0-5-2-8-2a6 6 0 0 0-4 1.528"/>'
  };
  var ICON = {
    clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
    user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    ban: '<circle cx="12" cy="12" r="10"/><path d="M4.929 4.929 19.07 19.071"/>',
    shield: '<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/><path d="m9 12 2 2 4-4"/>',
    reopen: '<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>',
    x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    alert: '<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>',
    chevron: '<path d="m6 9 6 6 6-6"/>'
  };

  function maskUrl(inner) {
    var svg = "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' " +
      "stroke-width='2' stroke-linecap='round' stroke-linejoin='round'>" + inner + '</svg>';
    return 'url("data:image/svg+xml,' + encodeURIComponent(svg) + '")';
  }
  function maskRule(selector, inner) {
    var u = maskUrl(inner);
    return selector + '{-webkit-mask-image:' + u + ';mask-image:' + u + ';}\n';
  }

  S3.injectMasks = function () {
    if (document.getElementById('s3-masks')) { return; }
    var css = '';
    Object.keys(CATEGORY).forEach(function (key) { css += maskRule('.icon-glyph--' + key, CATEGORY[key]); });
    css += maskRule('.trust-icon--clock', ICON.clock);
    css += maskRule('.trust-icon--user', ICON.user);
    css += maskRule('.vote-btn__icon--check', ICON.check);
    css += maskRule('.vote-btn__icon--ban', ICON.ban);
    css += maskRule('.vote-btn__icon--shield-check', ICON.shield);
    css += maskRule('.vote-btn__icon--rotate-ccw', ICON.reopen);
    css += maskRule('.leaflet-container .leaflet-popup a.leaflet-popup-close-button:not([hidden])::before', ICON.x);
    css += maskRule('.vote-error:not([hidden])::before', ICON.alert);
    css += maskRule('.trust-block__chevron', ICON.chevron);
    var style = document.createElement('style');
    style.id = 's3-masks';
    style.textContent = css;
    document.head.appendChild(style);
  };

  // ---- Small shared pieces ----------------------------------------------------------------------------------
  var panelSeq = 0;
  function stateClasses(state) {
    return 'sev-' + state.severity + ' age-' + state.age + ' vis-' + state.visibility;
  }
  S3.stateClasses = stateClasses;

  function buildBadge(state) {
    var badge = el('span', 'icon-badge icon-badge--sm');
    var glyph = el('span', 'icon-glyph icon-glyph--' + state.category);
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);
    return badge;
  }
  function buildSeverityLabel(state) {
    return el('span', 'severity-label severity-label--' + state.severity, S3.SEVERITY_WORDS[state.severity]);
  }
  // The state label is built only while a word shows. The app builds it hidden and empty on a Live report, which
  // paints nothing, so leaving it out is the same picture.
  function buildStateTag(state) {
    var word = S3.STATE_WORDS[state.visibility];
    if (!word) { return null; }
    return el('span', 'visibility-tag visibility-tag--' + state.visibility, word);
  }

  function setOpen(toggle, panel, open) {
    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
    panel.hidden = !open;
  }

  // One panel for both designs: plain sentences in normal flow, one per line, no animation.
  function buildPanel(state, ctx, panelId) {
    var panel = el('div', 'trust-block__panel');
    panel.id = panelId;
    panel.hidden = true;
    S3.panelLines(state, ctx.wording, ctx.scores).forEach(function (line) {
      panel.appendChild(el('p', 'trust-block__line', line.text));
    });
    return panel;
  }

  function buildCounts(state) {
    var counts = el('span', 'trust-block__counts');
    var first = el('span', 'trust-block__count');
    first.appendChild(el('span', 'trust-num', state.confirmedText));
    first.appendChild(txt(' confirmed'));
    var second = el('span', 'trust-block__count trust-block__count--second');
    second.appendChild(el('span', 'trust-num', state.disputedText));
    second.appendChild(txt(' disputed'));
    counts.appendChild(first);
    counts.appendChild(txt(' '));
    counts.appendChild(second);
    return counts;
  }

  // One still current or reporter signal, as trust.js createSignal builds it: icon, word, and on the popup the number
  // behind a visually hidden ", score" prefix.
  function buildSignal(iconClass, extraClass, label, score, scores) {
    var sig = el('span', extraClass ? 'trust-signal ' + extraClass : 'trust-signal');
    var icon = el('span', iconClass);
    icon.setAttribute('aria-hidden', 'true');
    sig.appendChild(icon);
    var text = el('span', 'trust-signal__text');
    text.appendChild(el('span', null, label));
    if (scores && score !== null && score !== undefined) {
      var sc = el('span');
      sc.appendChild(el('span', 'visually-hidden', ', score'));
      sc.appendChild(txt(' '));
      sc.appendChild(el('span', 'trust-num', String(score)));
      text.appendChild(sc);
    }
    sig.appendChild(text);
    return sig;
  }

  function wireToggle(block, toggle, panel) {
    toggle.addEventListener('click', function () {
      setOpen(toggle, panel, toggle.getAttribute('aria-expanded') !== 'true');
    });
    // The block stops the click and the Enter and Space keys, so the row and the map never see them.
    block.addEventListener('click', function (evt) { evt.stopPropagation(); });
    block.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') { evt.stopPropagation(); }
    });
  }

  // ---- The current block, copied from trust.js createTrustBlock --------------------------------------------
  function buildCurrentBlock(state, ctx) {
    var block = el('div', 'trust-block');
    panelSeq += 1;
    var panelId = 'trust-panel-' + panelSeq;

    var toggle = el('button', 'trust-block__toggle');
    toggle.type = 'button';
    toggle.setAttribute('aria-expanded', 'false');
    toggle.setAttribute('aria-controls', panelId);

    var signals = el('span', 'trust-block__signals');
    var any = false;
    signals.appendChild(txt(' '));
    if (state.currency) {
      signals.appendChild(buildSignal('trust-icon trust-icon--clock', '', state.currency.label, state.currency.score, ctx.scores));
      any = true;
    }
    signals.appendChild(txt(' '));
    if (state.reporter) {
      signals.appendChild(buildSignal('trust-icon trust-icon--user', 'trust-signal--second', state.reporter.label, state.reporter.score, ctx.scores));
      any = true;
    }
    signals.hidden = !any;

    toggle.appendChild(buildCounts(state));
    toggle.appendChild(signals);
    toggle.appendChild(el('span', 'visually-hidden', '. Shows what these mean.'));

    var panel = buildPanel(state, ctx, panelId);
    block.appendChild(toggle);
    block.appendChild(panel);
    wireToggle(block, toggle, panel);
    return block;
  }

  // ---- The redesigned block -------------------------------------------------------------------------------
  // The reporter words are not in the button any more. The button holds the two counts, the still current words when
  // there are any, and a chevron that says "this opens". The face wrapper lets the compact line variant flow the
  // counts and the still current words as one wrapping line, and the two row variant stack them, from CSS alone.
  function buildRedesignBlock(state, ctx) {
    var block = el('div', 'trust-block');
    panelSeq += 1;
    var panelId = 'trust-panel-' + panelSeq;

    var toggle = el('button', 'trust-block__toggle');
    toggle.type = 'button';
    toggle.setAttribute('aria-expanded', 'false');
    toggle.setAttribute('aria-controls', panelId);

    var face = el('span', 'trust-block__face');
    face.appendChild(buildCounts(state));
    if (state.currency) {
      var signals = el('span', 'trust-block__signals');
      signals.appendChild(txt(' '));
      signals.appendChild(buildSignal('trust-icon trust-icon--clock', '', state.currency.label, state.currency.score, ctx.scores));
      face.appendChild(txt(' '));
      face.appendChild(signals);
    }
    toggle.appendChild(face);

    var chevron = el('span', 'trust-block__chevron');
    chevron.setAttribute('aria-hidden', 'true');
    toggle.appendChild(chevron);
    toggle.appendChild(el('span', 'visually-hidden', '. Shows what these mean.'));

    var panel = buildPanel(state, ctx, panelId);
    block.appendChild(toggle);
    block.appendChild(panel);
    wireToggle(block, toggle, panel);
    return block;
  }

  // The reporter words: plain text, regular weight, no icon, no chip. The popup adds the number behind the same
  // visually hidden ", score" prefix the button used, as plain text in the same weight.
  function buildReporter(state, ctx) {
    if (!state.reporter) { return null; }
    var wrap = el('span', 'trust-reporter');
    var text = el('span', 'trust-reporter__text');
    text.appendChild(txt(state.reporter.label));
    if (ctx.scores && state.reporter.score !== null) {
      text.appendChild(el('span', 'visually-hidden', ', score'));
      text.appendChild(txt(' ' + state.reporter.score));
    }
    wrap.appendChild(text);
    return wrap;
  }

  // ---- The vote area ---------------------------------------------------------------------------------------
  var voteSeq = 0;
  function voteButton(cls, action, iconCls, label) {
    var btn = el('button', cls);
    btn.type = 'button';
    btn.dataset.action = action;
    var icon = el('span', 'vote-btn__icon ' + iconCls);
    icon.setAttribute('aria-hidden', 'true');
    btn.appendChild(icon);
    btn.appendChild(el('span', 'vote-btn__label', label));
    return btn;
  }

  // The three buttons and the inline confirmation, with the app's class names. They are here so the block is judged
  // in context. Confirm and Dispute only flip aria-pressed, and no count ever changes: the app never predicts a vote.
  function buildVoteBlock() {
    voteSeq += 1;
    var controls = el('div', 'vote-controls');
    var confirmBtn = voteButton('vote-btn vote-btn--confirm', 'confirm', 'vote-btn__icon--check', 'Confirm');
    confirmBtn.setAttribute('aria-pressed', 'false');
    var disputeBtn = voteButton('vote-btn vote-btn--dispute', 'dispute', 'vote-btn__icon--ban', 'Dispute');
    disputeBtn.setAttribute('aria-pressed', 'false');
    var resolveBtn = voteButton('vote-btn vote-btn--resolve', 'resolve', 'vote-btn__icon--shield-check', 'Mark resolved');
    controls.appendChild(confirmBtn);
    controls.appendChild(disputeBtn);
    controls.appendChild(resolveBtn);

    var questionId = 'pinalert-resolve-q-' + voteSeq;
    var confirmBlock = el('div', 'resolve-confirm');
    confirmBlock.hidden = true;
    confirmBlock.setAttribute('role', 'group');
    confirmBlock.setAttribute('aria-labelledby', questionId);
    var heading = el('p', null, 'Mark this report resolved?');
    heading.id = questionId;
    var cancelBtn = el('button', 'vote-btn vote-btn--cancel-resolve', 'Cancel');
    cancelBtn.type = 'button';
    var affirmBtn = el('button', 'vote-btn vote-btn--confirm-resolve', 'Yes, mark resolved');
    affirmBtn.type = 'button';
    confirmBlock.appendChild(heading);
    confirmBlock.appendChild(cancelBtn);
    confirmBlock.appendChild(affirmBtn);
    controls.appendChild(confirmBlock);

    var note = el('p', 'vote-note');
    note.hidden = true;
    var error = el('p', 'vote-error');
    error.setAttribute('role', 'alert');
    error.hidden = true;

    function press(btn, other) {
      var on = btn.getAttribute('aria-pressed') === 'true';
      btn.setAttribute('aria-pressed', on ? 'false' : 'true');
      if (!on) { other.setAttribute('aria-pressed', 'false'); }
    }
    confirmBtn.addEventListener('click', function () { press(confirmBtn, disputeBtn); });
    disputeBtn.addEventListener('click', function () { press(disputeBtn, confirmBtn); });
    function showButtons(show) {
      [confirmBtn, disputeBtn, resolveBtn].forEach(function (b) { b.hidden = !show; });
      confirmBlock.hidden = show;
    }
    resolveBtn.addEventListener('click', function () { showButtons(false); cancelBtn.focus(); });
    cancelBtn.addEventListener('click', function () { showButtons(true); resolveBtn.focus(); });
    affirmBtn.addEventListener('click', function () {
      showButtons(true);
      note.textContent = 'Report marked resolved.';
      note.hidden = false;
    });
    controls.addEventListener('click', function (evt) { evt.stopPropagation(); });
    return { controls: controls, note: note, error: error };
  }

  function buildReopen() {
    var controls = el('div', 'vote-controls');
    controls.appendChild(voteButton('vote-btn vote-btn--reopen', 'reopen', 'vote-btn__icon--rotate-ccw', 'Reopen · not actually resolved'));
    var error = el('p', 'vote-error');
    error.setAttribute('role', 'alert');
    error.hidden = true;
    return { controls: controls, error: error };
  }

  // ---- Surfaces --------------------------------------------------------------------------------------------
  // ctx: { variant: spec, scores: bool, wording: 'proposed'|'current', statepos: 'today'|'top' }
  function buildBlock(state, ctx) {
    return ctx.variant.redesign ? buildRedesignBlock(state, ctx) : buildCurrentBlock(state, ctx);
  }

  // The popup card, from map.js buildPopupCard. Redesign: the header holds a pills group and the reporter words.
  function buildPopupCard(state, ctx) {
    var redesign = ctx.variant.redesign;
    var card = el('div', 'popup-card');
    card.setAttribute('tabindex', '-1');
    var body = el('div', 'popup-card__body');

    var header = el('div', 'popup-card__header');
    var tag = buildStateTag(state);
    if (redesign) {
      var besideLabel = ctx.variant.header === 'beside';
      var pills = el('div', 'popup-card__pills');
      pills.appendChild(buildBadge(state));
      pills.appendChild(buildSeverityLabel(state));
      if (tag && !besideLabel) { pills.appendChild(tag); }
      header.appendChild(pills);
      var reporter = buildReporter(state, ctx);
      if (besideLabel) {
        // H: the state label and the reporter words share a line of their own, under the badge row, so the words are
        // always right beside the label and the line can use the room the close button needs only on the first line.
        if (tag || reporter) {
          var sline = el('div', 'popup-card__stateline');
          if (tag) { sline.appendChild(tag); }
          if (reporter) { sline.appendChild(reporter); }
          header.appendChild(sline);
        }
      } else if (reporter) { header.appendChild(reporter); }
    } else {
      header.appendChild(buildBadge(state));
      header.appendChild(buildSeverityLabel(state));
      if (tag) { header.appendChild(tag); }
    }

    var title = el('p', 'popup-card__title', state.description);
    var meta = el('div', 'popup-card__meta');
    var metaGlyph = el('span', 'icon-glyph icon-glyph--' + state.category);
    metaGlyph.setAttribute('aria-hidden', 'true');
    var metaLines = el('div', 'popup-card__meta-lines');
    S3.metaLines(state).forEach(function (line) { metaLines.appendChild(el('p', 'popup-card__meta-line', line)); });
    meta.appendChild(metaGlyph);
    meta.appendChild(metaLines);

    body.appendChild(header);
    body.appendChild(title);
    body.appendChild(meta);
    body.appendChild(buildBlock(state, ctx));
    if (!state.activityOnly) {
      var actions = el('div', 'popup-card__actions');
      var votes = buildVoteBlock();
      actions.appendChild(votes.controls);
      actions.appendChild(votes.note);
      actions.appendChild(votes.error);
      body.appendChild(actions);
    }
    card.appendChild(body);
    return card;
  }

  // The map stand in: a flat surface, a pin, and the Leaflet popup chain the app's CSS expects. The close button is the
  // first child of the popup, as in A20. Leaflet itself is not loaded because the sketch uses no network.
  function buildPopup(state, ctx) {
    var map = el('div', 'leaflet-container map-mock');
    var popup = el('div', 'leaflet-popup ' + stateClasses(state));
    popup.setAttribute('data-popup-open', 'true');
    popup.setAttribute('role', 'dialog');
    popup.setAttribute('aria-modal', 'false');

    var close = el('a', 'leaflet-popup-close-button');
    close.href = '#close';
    close.setAttribute('role', 'button');
    close.setAttribute('aria-label', 'Close popup');
    var times = el('span', null, '×');
    times.setAttribute('aria-hidden', 'true');
    close.appendChild(times);
    close.addEventListener('click', function (evt) { evt.preventDefault(); setPopupShown(false); });

    var wrapper = el('div', 'leaflet-popup-content-wrapper');
    var content = el('div', 'leaflet-popup-content');
    content.appendChild(buildPopupCard(state, ctx));
    wrapper.appendChild(content);

    var tipContainer = el('div', 'leaflet-popup-tip-container');
    tipContainer.appendChild(el('div', 'leaflet-popup-tip'));

    popup.appendChild(close);
    popup.appendChild(wrapper);
    popup.appendChild(tipContainer);
    map.appendChild(popup);

    // The pin is a button, as every Leaflet marker is in the app, with the accessible name 03-UI-SPEC gives it. Tapping it
    // opens the popup again after the close button has shut it.
    var holder = el('div', 'map-mock__pin');
    var pin = el('button', 'map-mock__pin-button');
    pin.type = 'button';
    var named = S3.CATEGORY_LABELS[state.category] + ', ' + S3.SEVERITY_WORDS[state.severity] + ', ' + state.timeText;
    var stateWord = S3.STATE_WORDS[state.visibility];
    pin.setAttribute('aria-label', stateWord ? named + ', ' + stateWord : named);
    var pinBadge = el('span', 'icon-badge icon-badge--pin ' + stateClasses(state));
    var pinGlyph = el('span', 'icon-glyph icon-glyph--' + state.category);
    pinGlyph.setAttribute('aria-hidden', 'true');
    pinBadge.appendChild(pinGlyph);
    pin.appendChild(pinBadge);
    holder.appendChild(pin);
    map.appendChild(holder);

    function setPopupShown(show) {
      popup.hidden = !show;
      pin.setAttribute('aria-expanded', show ? 'true' : 'false');
      if (!show) { pin.focus(); }
    }
    pin.setAttribute('aria-expanded', 'true');
    pin.addEventListener('click', function () { setPopupShown(popup.hidden); });
    return { root: map, scope: wrapper, popup: popup, pin: pin, setPopupShown: setPopupShown, card: popup.querySelector('.popup-card') };
  }

  // The row body, shared by the feed row and the Activity row. Order today: severity label, title, meta, state label,
  // trust block, then the vote area. Redesign: a top zone (pills group plus the reporter words) comes first. The state
  // label stays under the meta by default, or joins the severity label when statepos is "top".
  function buildRowBody(state, ctx, surface) {
    var redesign = ctx.variant.redesign;
    var body = el('div', 'report-row__body');
    var tag = buildStateTag(state);
    var tagUp = redesign && ctx.statepos === 'top';

    if (redesign) {
      var top = el('div', 'report-row__top');
      var pills = el('div', 'report-row__pills');
      pills.appendChild(buildSeverityLabel(state));
      if (tag && tagUp) { pills.appendChild(tag); }
      top.appendChild(pills);
      var reporter = buildReporter(state, ctx);
      var beside = ctx.variant.header === 'beside';
      if (reporter && !beside) { top.appendChild(reporter); }
      if (reporter && beside && tagUp) { pills.appendChild(reporter); }
      body.appendChild(top);
    } else {
      body.appendChild(buildSeverityLabel(state));
    }
    body.appendChild(el('p', 'report-row__title', state.description));
    body.appendChild(el('p', 'report-row__meta', S3.metaText(state)));
    if (redesign && beside && !tagUp && (tag || reporter)) {
      var stateLine = el('div', 'report-row__stateline');
      if (tag) { stateLine.appendChild(tag); }
      if (reporter) { stateLine.appendChild(reporter); }
      body.appendChild(stateLine);
    } else if (tag && !tagUp) { body.appendChild(tag); }
    body.appendChild(buildBlock(state, ctx));

    if (surface === 'activity') {
      if (state.canReopen) {
        var reopen = buildReopen();
        body.appendChild(reopen.controls);
        body.appendChild(reopen.error);
      }
    } else if (!state.activityOnly) {
      var votes = buildVoteBlock();
      body.appendChild(votes.controls);
      body.appendChild(votes.note);
      body.appendChild(votes.error);
    }
    return body;
  }

  function buildRowItem(state, ctx, surface) {
    var li = el('li', 'report-row ' + stateClasses(state));
    if (surface === 'row') {
      li.setAttribute('tabindex', '0');
      li.setAttribute('role', 'listitem');
    }
    li.appendChild(buildBadge(state));
    li.appendChild(buildRowBody(state, ctx, surface));
    return li;
  }

  function buildRow(state, ctx) {
    var list = el('ol');
    list.id = 'report-list';
    var li = buildRowItem(state, ctx, 'row');
    list.appendChild(li);
    return { root: list, scope: li };
  }

  function buildActivity(state, ctx) {
    var page = el('main', 'profile-page');
    var section = el('section', 'profile-activity');
    section.appendChild(el('h2', null, 'Activity'));
    var list = el('ol', 'profile-report-list');
    var li = buildRowItem(state, ctx, 'activity');
    list.appendChild(li);
    section.appendChild(list);
    page.appendChild(section);
    return { root: page, scope: li };
  }

  S3.buildSurface = function (surface, state, ctx) {
    if (surface === 'row') { return buildRow(state, ctx); }
    if (surface === 'activity') { return buildActivity(state, ctx); }
    return buildPopup(state, ctx);
  };

  S3.setOpen = setOpen;

  // Variants register themselves here (variants/*.js).
  S3.variants = S3.variants || {};
}(window.S3));
