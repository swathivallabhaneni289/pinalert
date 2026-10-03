/*
 * d4: the popup built to the owner's reference (image 3). Mockup code, same rules as the other sketches:
 * every string reaches the DOM through textContent, never innerHTML.
 *
 * Plugs into stage.html through window.PopupVariant. Looks come from d4.css, behaviour lives here:
 *   - the whole trust block (counts row and signals row) is ONE disclosure button that opens an inline panel
 *     of four plain sentences (aria-expanded, a second tap or Escape closes it)
 *   - Confirm and Dispute are the real toggles from Sketch.buildActions, with icons added
 *   - the whole comments footer is one link
 */
(function () {
  'use strict';

  // Lucide icons (lucide-static 1.41.0, ISC licence), the same family the repo already uses. They are drawn
  // as CSS masks so they take the text colour, exactly like the category glyphs in main.css.
  var ICONS = {
    clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
    user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    ban: '<circle cx="12" cy="12" r="10"/><path d="M4.929 4.929 19.07 19.071"/>',
    'shield-check': '<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/><path d="m9 12 2 2 4-4"/>',
    'message-square': '<path d="M22 17a2 2 0 0 1-2 2H6.828a2 2 0 0 0-1.414.586l-2.202 2.202A.71.71 0 0 1 2 21.286V5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2z"/>',
    'chevron-right': '<path d="m9 18 6-6-6-6"/>',
    x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>'
  };

  // The nine category glyphs, copied unchanged from web/static/icons/*.svg and drawn here as masks for the meta line.
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

  var SEVERITY_WORD = { critical: 'Critical', medium: 'Medium', low: 'Low' };

  function maskUrl(inner, strokeWidth) {
    var svg = "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='black' " +
      "stroke-width='" + strokeWidth + "' stroke-linecap='round' stroke-linejoin='round'>" + inner + '</svg>';
    return 'url("data:image/svg+xml,' + encodeURIComponent(svg) + '")';
  }

  function injectMasks() {
    if (document.getElementById('d4-icon-masks')) { return; }
    var css = ':root[data-variant="d4"]{--d4-x:' + maskUrl(ICONS.x, 2) + ';}\n';
    Object.keys(ICONS).forEach(function (key) {
      var u = maskUrl(ICONS[key], 2);
      css += ':root[data-variant="d4"] .d4-icon--' + key + '{-webkit-mask-image:' + u + ';mask-image:' + u + ';}\n';
    });
    Object.keys(CATEGORY).forEach(function (key) {
      var u = maskUrl(CATEGORY[key], 2);
      css += ':root[data-variant="d4"] .d4-icon--cat-' + key + '{-webkit-mask-image:' + u + ';mask-image:' + u + ';}\n';
    });
    var style = document.createElement('style');
    style.id = 'd4-icon-masks';
    style.textContent = css;
    document.head.appendChild(style);
  }

  function icon(el, name, small) {
    var node = el('span', 'd4-icon d4-icon--' + name + (small ? ' d4-icon--sm' : ''));
    node.setAttribute('aria-hidden', 'true');
    return node;
  }

  // ---- The plain sentences behind the trust lines -------------------------------------------------------
  // Sample wording for the sketch. Where the report carries a number the sentence uses it. The two signal
  // sentences quote a window and a record that the backend does not supply yet (Phase 3), so their figures are
  // derived from the score or fixed, and the owner should read them as placeholders.
  function many(n) { return n === '99+' ? '99 or more' : String(n); }

  function confirmedSentence(n) {
    var tail = ' Counted by place, not by vote.';
    if (n === 0) { return 'No one has confirmed this yet.' + tail; }
    if (n === 1) { return '1 person in 1 place confirmed this.' + tail; }
    return many(n) + ' people in ' + many(n) + ' different places confirmed this.' + tail;
  }

  function disputedSentence(n) {
    if (n === 0) { return 'No one nearby has disputed this.'; }
    if (n === 1) { return '1 person nearby disputed this.'; }
    return many(n) + ' people nearby disputed this.';
  }

  function currentSentence(r, confirmed) {
    var label = r.current ? r.current.label : '';
    if (label === 'Up to date') {
      var k = confirmed === 1 ? 1 : 2;
      return k === 1 ? '1 place confirmed this in the last 2 hours.' : k + ' different places confirmed this in the last 2 hours.';
    }
    if (label === 'Too early to tell') { return 'Too few people nearby have confirmed this yet to judge.'; }
    if (label === 'Getting old') { return 'Few recent confirmations. It fades unless someone confirms it again.'; }
    return 'No one has confirmed this lately. It fades unless someone confirms it again.';
  }

  function reporterSentence(r) {
    var sig = r.reporter;
    if (!sig || sig.score === null || sig.score === undefined) {
      return 'This is their first report, so there is no record yet.';
    }
    var of = 9;
    var good = Math.round((sig.score / 100) * of);
    return good + ' of ' + of + ' earlier reports were confirmed by people nearby.';
  }

  function commentsCopy(n) {
    if (n === 0) { return { count: 'No comments', action: 'Add a comment', name: 'Add a comment, no comments yet' }; }
    if (n === 1) { return { count: '1 comment', action: 'View comment', name: 'View comment, 1 comment' }; }
    return { count: n + ' comments', action: 'View comments', name: 'View comments, ' + n + ' comments' };
  }

  function build(r, ctx) {
    var el = ctx.el;
    injectMasks();

    var root = el('div', 'd4-card');
    var body = el('div', 'd4-body');
    root.appendChild(body);

    // ---- Header: badge, severity chip (a pill), optional Unconfirmed chip. The close button is Leaflet's own,
    // restyled in d4.css and placed on this row.
    var header = el('div', 'd4-header');
    header.appendChild(ctx.Sketch.buildBadge(r, 'sm'));
    var sevChip = el('span', 'd4-chip d4-chip--severity d4-chip--' + (r.severity === 'critical' ? 'critical' : 'dotted'));
    if (r.severity !== 'critical') {
      // Low and medium text in the severity hue fails 4.5:1, so those two keep the hue on the tint and this dot.
      var dot = el('span', 'd4-chip__dot');
      dot.setAttribute('aria-hidden', 'true');
      sevChip.appendChild(dot);
    }
    sevChip.appendChild(document.createTextNode(SEVERITY_WORD[r.severity] || 'Low'));
    header.appendChild(sevChip);
    if (r.chip) { header.appendChild(el('span', 'd4-chip d4-chip--neutral', r.chip)); }
    body.appendChild(header);

    body.appendChild(el('p', 'd4-title', r.title));

    var meta = el('div', 'd4-meta');
    meta.appendChild(icon(el, 'cat-' + r.category, true));
    var metaLines = el('div', 'd4-meta__lines');
    ctx.Sketch.metaParts(r).forEach(function (parts) {
      metaLines.appendChild(ctx.Sketch.buildMetaLine(parts, 'd4-meta__line'));
    });
    meta.appendChild(metaLines);
    body.appendChild(meta);

    // ---- Trust block: ONE disclosure button over both rows, and the panel it opens --------------------------
    var counts = { confirmed: r.confirmed, disputed: r.disputed };
    var panelId = 'd4-explain-' + r.id;
    var open = false;

    var trust = el('div', 'd4-trust');
    var toggle = el('button', 'd4-trust__toggle');
    toggle.type = 'button';
    toggle.setAttribute('aria-expanded', 'false');
    toggle.setAttribute('aria-controls', panelId);

    // Rows are spans, because a button may only hold phrasing content.
    var countsRow = el('span', 'd4-row d4-row--counts');
    var confirmedItem = el('span', 'd4-count');
    var confirmedNum = el('span', 'd4-num', String(counts.confirmed));
    confirmedItem.appendChild(confirmedNum);
    confirmedItem.appendChild(document.createTextNode(' confirmed'));
    var disputedCell = el('span', 'd4-count-cell d4-count-cell--second');
    var disputedItem = el('span', 'd4-count');
    var disputedNum = el('span', 'd4-num', String(counts.disputed));
    disputedItem.appendChild(disputedNum);
    disputedItem.appendChild(document.createTextNode(' disputed'));
    disputedCell.appendChild(disputedItem);
    var confirmedCell = el('span', 'd4-count-cell');
    confirmedCell.appendChild(confirmedItem);
    countsRow.appendChild(confirmedCell);
    countsRow.appendChild(disputedCell);
    toggle.appendChild(countsRow);

    function signalItem(iconName, signal, second) {
      var item = el('span', 'd4-signal' + (second ? ' d4-signal--second' : ''));
      item.appendChild(icon(el, iconName, true));
      var text = el('span', 'd4-signal__text');
      text.appendChild(document.createTextNode(signal.label));
      if (signal.score !== null && signal.score !== undefined) {
        text.appendChild(document.createTextNode(' '));
        text.appendChild(el('span', 'd4-num', String(signal.score)));
      }
      item.appendChild(text);
      return item;
    }

    var signalsRow = el('span', 'd4-row d4-row--signals');
    if (r.current) { signalsRow.appendChild(signalItem('clock', r.current, false)); }
    if (r.reporter) { signalsRow.appendChild(signalItem('user', r.reporter, !!r.current)); }
    if (signalsRow.childNodes.length) { toggle.appendChild(signalsRow); }

    var hint = el('span', 'visually-hidden', '. Shows what these mean.');
    toggle.appendChild(hint);
    trust.appendChild(toggle);

    // The panel: the four short sentences as plain text lines. It stays out of the page until opened.
    var panel = el('div', 'd4-explain');
    panel.id = panelId;
    panel.hidden = true;
    var lines = {
      confirmed: el('p', 'd4-explain__line'),
      disputed: el('p', 'd4-explain__line')
    };
    if (r.current) { lines.current = el('p', 'd4-explain__line'); }
    lines.reporter = el('p', 'd4-explain__line');
    ['confirmed', 'disputed', 'current', 'reporter'].forEach(function (k) { if (lines[k]) { panel.appendChild(lines[k]); } });

    function fillPanel() {
      lines.confirmed.textContent = confirmedSentence(counts.confirmed);
      lines.disputed.textContent = disputedSentence(counts.disputed);
      if (lines.current) { lines.current.textContent = currentSentence(r, counts.confirmed); }
      lines.reporter.textContent = reporterSentence(r);
    }

    function setOpen(next) {
      open = next;
      toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
      panel.hidden = !open;
      if (open) { fillPanel(); }
    }

    toggle.addEventListener('click', function () { setOpen(!open); });
    trust.appendChild(panel);
    body.appendChild(trust);

    // Escape closes the panel first. With nothing open it closes the popup. A document level listener does
    // this because Safari does not focus a button on click, so the key would otherwise never reach the card, and
    // Leaflet itself only handles Escape while the map has focus.
    function onEscape(e) {
      if (e.key !== 'Escape') { return; }
      if (!root.isConnected) { return; }
      if (open) {
        setOpen(false);
        toggle.focus();
        e.stopPropagation();
      } else if (root.contains(document.activeElement)) {
        var closer = root.closest('.leaflet-popup');
        closer = closer && closer.querySelector('.leaflet-popup-close-button');
        if (closer) { closer.click(); }
      }
    }
    document.addEventListener('keydown', onEscape, true);

    // The vote buttons keep the sketch's own behaviour. This facade only keeps the plain count text and an
    // open panel in step with a tap.
    var facade = {
      setCounts: function (c, d) {
        counts.confirmed = c;
        counts.disputed = d;
        confirmedNum.textContent = String(c);
        disputedNum.textContent = String(d);
        if (open) { fillPanel(); }
      }
    };

    // ---- Actions ----------------------------------------------------------------------------------------
    var actions = ctx.Sketch.buildActions(r, facade);
    var actionSection = el('div', 'd4-actions');
    actionSection.appendChild(actions.controls);
    actionSection.appendChild(actions.note);
    body.appendChild(actionSection);

    function addIcon(selector, name) {
      var btn = actions.controls.querySelector(selector);
      if (btn) { btn.insertBefore(icon(el, name), btn.firstChild); }
    }
    addIcon('.vote-btn--confirm', 'check');
    addIcon('.vote-btn--dispute', 'ban');
    addIcon('.vote-btn--resolve', 'shield-check');

    // Focus follows the inline confirmation, so a keyboard user is never left on a button that just vanished.
    var resolveBtn = actions.controls.querySelector('.vote-btn--resolve');
    var affirm = actions.controls.querySelector('.vote-btn--confirm-resolve');
    var cancel = actions.controls.querySelector('.vote-btn--cancel-resolve');
    // Cancel comes first in the page and on screen, the destructive Yes after it.
    if (affirm && cancel && affirm.parentNode) { affirm.parentNode.insertBefore(cancel, affirm); }
    if (resolveBtn && cancel) { resolveBtn.addEventListener('click', function () { cancel.focus(); }); }
    if (affirm) {
      affirm.addEventListener('click', function () {
        actions.note.tabIndex = -1;
        actions.note.focus();
      });
    }

    // ---- Footer: the whole row is one link --------------------------------------------------------------
    var copy = commentsCopy(r.thread.length);
    var footer = el('a', 'd4-footer');
    footer.href = ctx.href;
    footer.setAttribute('aria-label', copy.name);
    footer.appendChild(icon(el, 'message-square'));
    var ftext = el('span', 'd4-footer__text');
    ftext.appendChild(el('span', 'd4-footer__title', 'Comments'));
    ftext.appendChild(el('span', 'd4-footer__count', copy.count));
    footer.appendChild(ftext);
    var faction = el('span', 'd4-footer__action');
    faction.appendChild(el('span', 'd4-footer__label', copy.action));
    faction.appendChild(icon(el, 'chevron-right'));
    footer.appendChild(faction);
    root.appendChild(footer);

    return root;
  }

  window.PopupVariant = {
    build: build,
    // About 340 so the two signals fit on one row. The stage clamps this to the viewport minus 32 on narrow phones.
    width: 340,
    maxRadius: 12,
    radiusExempt: '.d4-chip',
    expectedOrder: [
      'Critical',
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
    ]
  };
}());
