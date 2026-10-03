// sketch-shared.js: sample data, DOM builders and helpers shared by the Pinalert sketches.
// Mockup only. Every string reaches the DOM through textContent, the same rule the app follows.
window.Sketch = (function () {
  'use strict';

  // Category glyph paths copied from web/static/icons/*.svg. The app masks these through
  // url("/static/icons/x.svg"), a root relative path that cannot resolve when a sketch is opened as a
  // file, so the sketch re-declares the same masks as data URIs. Nothing else about the glyph changes.
  var GLYPHS = {
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
  var AUTH_GLYPHS = {
    'arrow-left': '<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>',
    user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>'
  };

  var CATEGORY_LABELS = {
    flood: 'Flood',
    earthquake: 'Earthquake',
    fire: 'Fire',
    storm_cyclone: 'Storm/Cyclone damage',
    road_blocked: 'Road blocked',
    power_outage: 'Power outage',
    shelter_open: 'Shelter open',
    rescue_needed: 'Rescue needed',
    other: 'Other'
  };

  // Copy constants mirror web/static/js/votes.js.
  var COPY = {
    confirm: 'Confirm',
    dispute: 'Dispute',
    resolve: 'Mark resolved',
    resolveHeading: 'Mark this report resolved?',
    resolveAffirm: 'Yes, mark resolved',
    resolveCancel: 'Cancel',
    tooFar: 'Too far to vote. You need to be within 1\u00A0km of this report.'
  };

  // The order the owner specified for the popup, top to bottom. The rules check asserts it.
  var EXPECTED_ORDER = [
    'Fire near Beach Road junction',
    'Fire · 4 min ago · 0.6 km',
    '12 confirmed',
    '0 disputed',
    'Up to date 82',
    'Reliable reporter 78',
    'Confirm',
    'Dispute',
    'Mark resolved',
    'Comments',
    '2 comments',
    'View comments'
  ];

  // Sample reports. "critical" is the owner's spec verbatim. The others come from the sample table in
  // 03-DESIGN-BRIEF.md. Scores other than 82 and 78 are placeholders so the layout can be judged.
  // Nothing here is measured data.
  var THREAD_FIRE = [
    { reporter: true, timeText: '3 min ago', body: 'Fire brigade has reached the junction. Both lanes towards the lighthouse are closed.' },
    { reporter: false, timeText: '1 min ago', body: 'Smoke is drifting towards the market. If you live on the first two lanes, keep windows shut.' }
  ];

  var critical = {
    id: 42, category: 'fire', severity: 'critical', visibility: 'live', age: 'fresh',
    title: 'Fire near Beach Road junction', timeText: '4 min ago', distanceText: '0.6 km', shelter: null,
    confirmed: 12, disputed: 0,
    current: { label: 'Up to date', score: 82 },
    reporter: { label: 'Reliable reporter', score: 78 },
    chip: null, own: false, tooFar: false, thread: THREAD_FIRE
  };

  var STATES = {
    critical: critical,
    early: {
      id: 43, category: 'rescue_needed', severity: 'critical', visibility: 'live', age: 'fresh',
      title: 'Rescue needed on the terrace of a flooded house', timeText: '2 min ago', distanceText: '0.4 km', shelter: null,
      confirmed: 0, disputed: 0,
      current: { label: 'Too early to tell', score: null },
      reporter: { label: 'New reporter', score: null },
      chip: null, own: false, tooFar: false, thread: []
    },
    quiet: {
      id: 44, category: 'road_blocked', severity: 'low', visibility: 'live', age: 'stale',
      title: 'Road blocked by fallen tree', timeText: '6 hours ago', distanceText: '1.8 km', shelter: null,
      confirmed: 2, disputed: 0,
      current: { label: 'Needs re-confirming', score: 38 },
      reporter: { label: 'New reporter', score: null },
      chip: null, own: false, tooFar: false,
      thread: [{ reporter: false, timeText: '5 hours ago', body: 'The tree has been cut and moved to the side. One lane is open now.' }]
    },
    unconfirmed: {
      id: 45, category: 'shelter_open', severity: 'low', visibility: 'provisional', age: 'fresh',
      title: 'Shelter open at Government School', timeText: '35 min ago', distanceText: '0.9 km',
      shelter: { status: 'Limited', headcount: '150 people' },
      confirmed: 1, disputed: 0,
      current: null,
      reporter: { label: 'New reporter', score: null },
      chip: 'Unconfirmed', own: false, tooFar: false, thread: []
    },
    contested: {
      id: 46, category: 'power_outage', severity: 'medium', visibility: 'live', age: 'aging',
      title: 'Power outage in the whole street', timeText: '3 hours ago', distanceText: '1.6 km', shelter: null,
      confirmed: 5, disputed: 4,
      current: { label: 'Getting old', score: 52 },
      reporter: { label: 'Unreliable reporter', score: 23 },
      chip: null, own: false, tooFar: false,
      thread: [
        { reporter: false, timeText: '2 hours ago', body: 'Our lane has power again since about six.' },
        { reporter: false, timeText: '1 hour ago', body: 'Still no power on the north side.' },
        { reporter: true, timeText: '40 min ago', body: 'A repair crew is working on the north side now.' }
      ]
    },
    storm: {
      id: 48, category: 'storm_cyclone', severity: 'medium', visibility: 'live', age: 'fresh',
      title: 'Roof sheets blown off the community hall', timeText: '3 hours ago', distanceText: '1.6 km', shelter: null,
      confirmed: 3, disputed: 0,
      current: { label: 'Up to date', score: 71 },
      reporter: { label: 'Mixed record', score: 55 },
      chip: null, own: false, tooFar: false, thread: []
    },
    own: Object.assign({}, critical, { own: true }),
    far: Object.assign({}, critical, { tooFar: true }),
    long: Object.assign({}, critical, {
      id: 47, category: 'flood',
      title: 'Flooding on the service road behind the old railway station has cut off all three lanes and water is entering the ground floor shops, please avoid the whole stretch until the pumps arrive',
      confirmed: '99+', disputed: '99+',
      current: { label: 'Needs re-confirming', score: 41 },
      reporter: { label: 'Unreliable reporter', score: 12 }
    })
  };

  function getState(name) {
    return STATES[name] || STATES.critical;
  }

  function params() {
    var out = {};
    new URLSearchParams(location.search).forEach(function (value, key) { out[key] = value; });
    return out;
  }

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) { node.className = className; }
    if (text !== undefined && text !== null) { node.textContent = text; }
    return node;
  }

  function maskUrl(inner) {
    var svg = "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' " +
      "stroke-width='2' stroke-linecap='round' stroke-linejoin='round'>" + inner + '</svg>';
    return 'url("data:image/svg+xml,' + encodeURIComponent(svg) + '")';
  }

  function injectGlyphMasks() {
    var css = '';
    Object.keys(GLYPHS).forEach(function (key) {
      var u = maskUrl(GLYPHS[key]);
      css += '.icon-glyph--' + key + '{-webkit-mask-image:' + u + ';mask-image:' + u + ';}\n';
    });
    Object.keys(AUTH_GLYPHS).forEach(function (key) {
      var u = maskUrl(AUTH_GLYPHS[key]);
      css += '.auth-icon--' + key + '{-webkit-mask-image:' + u + ';mask-image:' + u + ';}\n';
    });
    var style = document.createElement('style');
    style.id = 'sketch-glyph-masks';
    style.textContent = css;
    document.head.appendChild(style);
  }

  function stateClasses(r) {
    return 'sev-' + r.severity + ' age-' + r.age + ' vis-' + r.visibility;
  }

  // Line one is category, time and distance. A shelter report puts its capacity and headcount on a second
  // line, so the first line stays short enough to sit on one row.
  function metaLines(r) {
    var lines = [[CATEGORY_LABELS[r.category], r.timeText, r.distanceText].join(' · ')];
    if (r.shelter) { lines.push([r.shelter.status, r.shelter.headcount].join(' · ')); }
    return lines;
  }

  function metaText(r) {
    return metaLines(r)[0];
  }

  // The same lines as segments. Each segment after the first carries its own separator dot, positioned in the
  // gap before it and clipped by the line, so a segment that wraps to the start of a line shows no stray dot.
  // The typed text is still exactly "Fire · 4 min ago · 0.6 km": the dot is real text, marked aria-hidden.
  function metaParts(r) {
    var parts = [[CATEGORY_LABELS[r.category], r.timeText, r.distanceText]];
    if (r.shelter) { parts.push([r.shelter.status, r.shelter.headcount]); }
    return parts;
  }

  function buildMetaLine(parts, className) {
    var line = el('p', 'meta-line ' + className);
    parts.forEach(function (part, i) {
      if (i > 0) { line.appendChild(document.createTextNode(' ')); }
      var seg = el('span', 'meta-line__seg');
      if (i > 0) {
        var dot = el('span', 'meta-line__dot', '\u00B7');
        dot.setAttribute('aria-hidden', 'true');
        seg.appendChild(dot);
        seg.appendChild(document.createTextNode(' '));
      }
      seg.appendChild(document.createTextNode(part));
      line.appendChild(seg);
    });
    return line;
  }

  function commentsText(n) {
    return n === 1 ? '1 comment' : n + ' comments';
  }

  // size: 'sm' (feed row and popup) or 'pin' (map marker). The state classes go on the badge for the
  // pin, exactly as map.js buildBadgeElement does, because a marker has no row ancestor.
  function buildBadge(r, size) {
    var badge = el('span', 'icon-badge icon-badge--' + size);
    if (size === 'pin') { badge.className += ' ' + stateClasses(r); }
    var glyph = el('span', 'icon-glyph icon-glyph--' + r.category);
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);
    return badge;
  }

  function buildTitle(r, className, tag) {
    var title = el(tag || 'p', className);
    if (r.severity === 'critical') {
      // Severity is never carried by colour alone, so a screen reader hears it.
      title.appendChild(el('span', 'visually-hidden', 'Critical. '));
    }
    title.appendChild(document.createTextNode(r.title));
    return title;
  }

  function countItem(value, label) {
    var item = el('span', 'trust-block__count');
    item.appendChild(el('span', 'trust-block__num', String(value)));
    item.appendChild(document.createTextNode(' ' + label));
    return item;
  }

  function signalItem(signal, showScore) {
    var item = el('span', 'trust-block__signal');
    item.appendChild(document.createTextNode(signal.label));
    if (showScore && signal.score !== null && signal.score !== undefined) {
      item.appendChild(document.createTextNode(' '));
      item.appendChild(el('span', 'trust-block__num', String(signal.score)));
    }
    return item;
  }

  // The trust block: two lines of plain text, no chips, bars, icons or colour. opts.scores adds the two
  // small numbers the popup shows after each signal word.
  function buildTrustBlock(r, opts) {
    var block = el('div', 'trust-block');

    var counts = el('p', 'trust-block__counts');
    var confirmedItem = countItem(r.confirmed, 'confirmed');
    var disputedItem = countItem(r.disputed, 'disputed');
    counts.appendChild(confirmedItem);
    counts.appendChild(disputedItem);
    block.appendChild(counts);

    var signals = el('p', 'trust-block__signals');
    if (r.current) { signals.appendChild(signalItem(r.current, opts && opts.scores)); }
    if (r.reporter) { signals.appendChild(signalItem(r.reporter, opts && opts.scores)); }
    if (signals.childNodes.length) { block.appendChild(signals); }

    block.setCounts = function (confirmed, disputed) {
      confirmedItem.firstChild.textContent = String(confirmed);
      disputedItem.firstChild.textContent = String(disputed);
    };
    return block;
  }

  function actionButton(className, label) {
    var btn = el('button', className, label);
    btn.type = 'button';
    return btn;
  }

  // The same three controls votes.js builds, with the same class names and copy. Returns the controls
  // element and a note element for the too far explanation. onCounts keeps the plain text counters in step
  // with a tap, purely so the sketch feels alive.
  function buildActions(r, trustBlock) {
    var controls = el('div', 'vote-controls');
    var note = el('p', 'vote-note');
    note.id = 'vote-note-' + r.id;
    note.setAttribute('role', 'status');
    note.hidden = true;

    var confirmBtn = null;
    var disputeBtn = null;
    var resolveBtn = actionButton('vote-btn vote-btn--resolve', COPY.resolve);
    var confirmed = r.confirmed;
    var disputed = r.disputed;

    function sync() {
      if (trustBlock && typeof confirmed === 'number' && typeof disputed === 'number') {
        trustBlock.setCounts(confirmed, disputed);
      }
    }

    if (!r.own) {
      confirmBtn = actionButton('vote-btn vote-btn--confirm', COPY.confirm);
      confirmBtn.setAttribute('aria-pressed', 'false');
      disputeBtn = actionButton('vote-btn vote-btn--dispute', COPY.dispute);
      disputeBtn.setAttribute('aria-pressed', 'false');
      controls.appendChild(confirmBtn);
      controls.appendChild(disputeBtn);

      confirmBtn.addEventListener('click', function () {
        var pressed = confirmBtn.getAttribute('aria-pressed') === 'true';
        confirmBtn.setAttribute('aria-pressed', pressed ? 'false' : 'true');
        if (typeof confirmed === 'number') { confirmed += pressed ? -1 : 1; }
        if (!pressed && disputeBtn.getAttribute('aria-pressed') === 'true') {
          disputeBtn.setAttribute('aria-pressed', 'false');
          if (typeof disputed === 'number') { disputed -= 1; }
        }
        sync();
      });
      disputeBtn.addEventListener('click', function () {
        var pressed = disputeBtn.getAttribute('aria-pressed') === 'true';
        disputeBtn.setAttribute('aria-pressed', pressed ? 'false' : 'true');
        if (typeof disputed === 'number') { disputed += pressed ? -1 : 1; }
        if (!pressed && confirmBtn.getAttribute('aria-pressed') === 'true') {
          confirmBtn.setAttribute('aria-pressed', 'false');
          if (typeof confirmed === 'number') { confirmed -= 1; }
        }
        sync();
      });
    }
    controls.appendChild(resolveBtn);

    var confirmBlock = el('div', 'resolve-confirm');
    confirmBlock.hidden = true;
    confirmBlock.appendChild(el('p', null, COPY.resolveHeading));
    var affirm = actionButton('vote-btn vote-btn--confirm-resolve', COPY.resolveAffirm);
    var cancel = actionButton('vote-btn vote-btn--cancel-resolve', COPY.resolveCancel);
    confirmBlock.appendChild(affirm);
    confirmBlock.appendChild(cancel);
    controls.appendChild(confirmBlock);

    function showButtons(show) {
      [confirmBtn, disputeBtn, resolveBtn].forEach(function (b) { if (b) { b.hidden = !show; } });
      confirmBlock.hidden = show;
    }
    resolveBtn.addEventListener('click', function () { showButtons(false); });
    cancel.addEventListener('click', function () { showButtons(true); resolveBtn.focus(); });
    affirm.addEventListener('click', function () {
      showButtons(true);
      [confirmBtn, disputeBtn, resolveBtn].forEach(function (b) { if (b) { b.hidden = true; } });
      note.textContent = 'Marked resolved.';
      note.hidden = false;
    });

    if (r.tooFar && !r.own) {
      [confirmBtn, disputeBtn, resolveBtn].forEach(function (b) {
        if (b) { b.disabled = true; b.setAttribute('aria-describedby', note.id); }
      });
      note.textContent = COPY.tooFar;
      note.hidden = false;
    }
    return { controls: controls, note: note };
  }

  function commentsHref(q) {
    var next = new URLSearchParams();
    ['theme', 'state', 'width', 'resolve', 'comments'].forEach(function (k) { if (q[k]) { next.set(k, q[k]); } });
    next.set('variant', 'a');
    return '../002-full-report-view-and-thread/stage.html?' + next.toString();
  }

  // The bottom row of the popup: a navigation row into the full report, never the thread itself.
  function buildCommentsRow(r, href) {
    var link = el('a', 'map-popup__comments');
    link.href = href;
    var text = el('span', 'map-popup__comments-text');
    var hideLabel = document.documentElement.getAttribute('data-variant') === 'c';
    text.appendChild(el('span', 'map-popup__comments-label' + (hideLabel ? ' visually-hidden' : ''), 'Comments'));
    text.appendChild(el('span', 'map-popup__comments-count', commentsText(r.thread.length)));
    link.appendChild(text);
    link.appendChild(el('span', 'map-popup__comments-action', 'View comments'));
    return link;
  }

  function post(meta, result) {
    window.__rules = result;
    window.__ready = true;
    try { window.parent.postMessage({ type: 'rules', meta: meta, result: result }, '*'); } catch (e) { /* standalone */ }
  }

  return {
    STATES: STATES,
    EXPECTED_ORDER: EXPECTED_ORDER,
    COPY: COPY,
    getState: getState,
    params: params,
    el: el,
    injectGlyphMasks: injectGlyphMasks,
    stateClasses: stateClasses,
    metaText: metaText,
    metaLines: metaLines,
    metaParts: metaParts,
    buildMetaLine: buildMetaLine,
    commentsText: commentsText,
    buildBadge: buildBadge,
    buildTitle: buildTitle,
    buildTrustBlock: buildTrustBlock,
    buildActions: buildActions,
    buildCommentsRow: buildCommentsRow,
    commentsHref: commentsHref,
    post: post
  };
}());
