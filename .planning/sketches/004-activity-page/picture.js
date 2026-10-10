// picture.js: the page for sketch 004 variant B, the Activity page copied from the owner's picture. Mockup only, classic
// script, no network, no markup sinks: every string from the data reaches the DOM through textContent, and the fixed
// icon drawings are parsed as SVG, never assigned as markup.
//
// Sample data. The seven rows and the six tab counts are the ones in the picture. The counts are the picture's own and do
// not add up to the seven rows, and a tab filters those seven rows with the meaning the picture's panel gives it.
(function () {
  'use strict';

  var NS = 'http://www.w3.org/2000/svg';

  // Lucide drawings (24 x 24, stroke). The flame is the shipped fire glyph.
  var DRAW = {
    map: '<path d="M14.106 5.553a2 2 0 0 0 1.788 0l3.659-1.83A1 1 0 0 1 21 4.619v12.764a1 1 0 0 1-.553.894l-4.553 2.277a2 2 0 0 1-1.788 0l-4.212-2.106a2 2 0 0 0-1.788 0l-3.659 1.83A1 1 0 0 1 3 19.381V6.618a1 1 0 0 1 .553-.894l4.553-2.277a2 2 0 0 1 1.788 0z"/><path d="M15 5.764v15"/><path d="M9 3.236v15"/>',
    reports: '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>',
    bell: '<path d="M10.268 21a2 2 0 0 0 3.464 0"/><path d="M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326"/>',
    zap: '<path d="M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z"/>',
    user: '<circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/>',
    reload: '<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/>',
    flame: '<path d="M8.5 14.5A2.5 2.5 0 0 0 11 12c0-1.38-.5-2-1-3-1.072-2.143-.224-4.054 2-6 .5 2.5 2 4.9 4 6.5 2 1.6 3 3.5 3 5.5a7 7 0 1 1-14 0c0-1.153.433-2.294 1-3a2.5 2.5 0 0 0 2.5 2.5z"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    message: '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>',
    home: '<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-6a2 2 0 0 1 2.582 0l7 6A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
    ban: '<circle cx="12" cy="12" r="10"/><path d="m4.929 4.929 14.142 14.142"/>',
    clock: '<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>',
    link: '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>',
    chevron: '<path d="m9 18 6-6-6-6"/>',
    arrowLeft: '<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>',
    logo: '<rect x="8" y="5" width="14" height="12" rx="4"/><path d="M12 17v6"/>'
  };

  function svg(name) {
    var doc = new DOMParser().parseFromString(
      '<svg xmlns="' + NS + '" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' + DRAW[name] + '</svg>', 'image/svg+xml');
    return document.importNode(doc.documentElement, true);
  }

  function icon(name, extra) {
    var span = document.createElement('span');
    span.className = 'ic' + (extra ? ' ' + extra : '');
    span.appendChild(svg(name));
    return span;
  }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) { n.className = cls; }
    if (text !== undefined) { n.textContent = text; }
    return n;
  }

  // The static icons in the markup: <span data-icon="map"></span>.
  Array.prototype.forEach.call(document.querySelectorAll('[data-icon]'), function (host) {
    var name = host.getAttribute('data-icon');
    host.classList.add('ic');
    if (host.hasAttribute('data-solid')) { host.classList.add('ic--solid'); }
    host.appendChild(svg(name));
  });

  // ---- The seven rows from the picture ------------------------------------------------------------------------
  var ROWS = [
    { tag: 'Your post', kind: 'post', title: 'Fire near Beach Road', act: 'You posted this report', time: '4 min ago', dist: '0.6 km', chip: ['Critical', 'critical'], icon: 'flame', tone: '#D15744', resolved: false },
    { tag: 'Your vote', kind: 'vote', title: 'Water over the underpass', act: 'You confirmed this report', time: '1 h ago', dist: '1.2 km', chip: ['Medium', 'medium'], icon: 'check', tone: '#459471', resolved: false },
    { tag: 'Your dispute', kind: 'dispute', title: 'Power outage near Market Road', act: 'You disputed this report', time: '3 h ago', dist: '2.4 km', chip: ['Low', 'low'], icon: 'message', tone: '#788089', resolved: false },
    { tag: 'System update', kind: 'system', title: 'New shelter open at Gandhi Nagar', act: 'Marked as resolved', time: '5 h ago', dist: '3.1 km', chip: ['Resolved', 'resolved'], icon: 'home', tone: '#4D82E1', resolved: true },
    { tag: 'System update', kind: 'system', title: 'Flood near Station Road', act: 'Moved to hidden', time: '6 h ago', dist: '4.8 km', chip: ['Not resolved', 'notresolved'], icon: 'ban', tone: '#7A818C', resolved: false },
    { tag: 'System update', kind: 'system', title: 'Power outage near Old Bridge', act: 'Expires in 1 hour', time: '8 h ago', dist: '5.2 km', chip: ['Medium', 'medium'], icon: 'clock', tone: '#D29C44', resolved: false },
    { tag: 'System update', kind: 'system', title: 'Road blocked near Lake View', act: 'Comment added', time: '10 h ago', dist: '6.7 km', chip: ['Low', 'low'], icon: 'link', tone: '#7B47B8', resolved: false }
  ];

  // The picture's tab counts, as drawn. What each tab shows is the picture's own panel text.
  var TABS = [
    { key: 'all', label: 'All', count: null },
    { key: 'posts', label: 'My posts', count: 3 },
    { key: 'votes', label: 'My votes', count: 5 },
    { key: 'disputes', label: 'My disputes', count: 2 },
    { key: 'resolved', label: 'Resolved', count: 8 },
    { key: 'unresolved', label: 'Not resolved', count: 4 }
  ];

  var FILTER = {
    all: function () { return true; },
    posts: function (r) { return r.kind === 'post'; },
    votes: function (r) { return r.kind === 'vote' || r.kind === 'dispute'; },
    disputes: function (r) { return r.kind === 'dispute'; },
    resolved: function (r) { return r.resolved; },
    unresolved: function (r) { return !r.resolved; }
  };

  var q = new URLSearchParams(location.search);
  var state = { tab: TABS.some(function (t) { return t.key === q.get('tab'); }) ? q.get('tab') : 'all' };

  var tablist = document.getElementById('tabs');
  var list = document.getElementById('rows');
  var tabButtons = {};

  function buildRow(r) {
    var li = el('li', 'row' + (r.kind === 'system' ? '' : ' row--personal'));
    li.style.setProperty('--tone', r.tone);
    li.style.setProperty('--bar', r.tone);

    var badge = el('span', 'row__icon');
    badge.appendChild(icon(r.icon, r.icon === 'flame' ? 'ic--solid' : ''));
    li.appendChild(badge);

    var body = el('div', 'row__body');
    body.appendChild(el('span', 'row__tag', r.tag));
    body.appendChild(el('p', 'row__title', r.title));
    var sub = el('p', 'row__sub');
    sub.appendChild(el('span', 'act', r.act));
    sub.appendChild(el('span', 'sep act-sep', '·'));
    sub.appendChild(el('span', 'when', r.time));
    sub.appendChild(el('span', 'sep', '·'));
    sub.appendChild(el('span', 'dist', r.dist));
    body.appendChild(sub);
    li.appendChild(body);

    li.appendChild(el('span', 'row__chip row__chip--' + r.chip[1], r.chip[0]));

    var go = el('span', 'row__go ic');
    go.appendChild(svg('chevron'));
    li.appendChild(go);

    // The whole card opens the report on the map. The sketch only says so.
    li.addEventListener('click', function () { toast('Opens "' + r.title + '" on the map.'); });
    return li;
  }

  var toastTimer = 0;
  function toast(message) {
    var t = document.getElementById('toast');
    if (!t) { return; }
    t.textContent = message;
    t.hidden = false;
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(function () { t.hidden = true; }, 2200);
  }

  function render() {
    TABS.forEach(function (t) {
      var on = t.key === state.tab;
      tabButtons[t.key].setAttribute('aria-selected', on ? 'true' : 'false');
      tabButtons[t.key].tabIndex = on ? 0 : -1;
    });
    list.textContent = '';
    ROWS.filter(FILTER[state.tab]).forEach(function (r) { list.appendChild(buildRow(r)); });
    list.setAttribute('aria-labelledby', 'tab-' + state.tab);
    window.__ready = true;
  }

  function select(key, focus) {
    state.tab = key;
    render();
    if (focus) { tabButtons[key].focus(); }
  }

  TABS.forEach(function (t) {
    var b = el('button', 'tab');
    b.type = 'button';
    b.id = 'tab-' + t.key;
    b.setAttribute('role', 'tab');
    b.setAttribute('aria-controls', 'rows');
    b.appendChild(el('span', 'tab__label', t.label));
    if (t.count !== null) { b.appendChild(el('span', 'tab__count', String(t.count))); }
    b.addEventListener('click', function () { select(t.key, false); });
    tabButtons[t.key] = b;
    tablist.appendChild(b);
  });

  tablist.addEventListener('keydown', function (evt) {
    var order = TABS.map(function (t) { return t.key; });
    var i = order.indexOf(state.tab);
    var next = null;
    if (evt.key === 'ArrowRight') { next = order[(i + 1) % order.length]; }
    else if (evt.key === 'ArrowLeft') { next = order[(i + order.length - 1) % order.length]; }
    else if (evt.key === 'Home') { next = order[0]; }
    else if (evt.key === 'End') { next = order[order.length - 1]; }
    if (next) { evt.preventDefault(); select(next, true); }
  });

  // Links in the sketch never leave it.
  document.addEventListener('click', function (evt) {
    var a = evt.target.closest('a[href]');
    if (a) { evt.preventDefault(); toast('Not part of the sketch.'); }
  });

  render();
}());
