// laptop_list_scroll_test.mjs - runs the real web/static/js/feed.js (with the
// real app.js, visibility.js and trust.js) in a Node sandbox and checks where a
// pin choice scrolls the list (Phase 3.4 plan 20, D-17, UI-SPEC 2a and 2e item 2):
//
//   - on a laptop only the cards list scrolls (the list head is outside it), so
//     choosing a pin whose row is below the fold calls scrollTo on #report-list
//     with the row's offset in it, and never on #report-sheet or the window;
//   - a row that is already whole in view causes no scroll at all;
//   - a list that does not overflow causes no scroll;
//   - on the narrow query the sheet (#report-sheet) is still the scroller and
//     the row ends up under the sticky list head with a small gap;
//   - reduced motion scrolls at once instead of smoothly.
//
// Run it with: node --test web/laptop_list_scroll_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the scripts call: a small element model with
// measurements that each test sets.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const read = (name) => fs.readFileSync(path.join(here, 'static', 'js', name), 'utf8');

class FakeEl {
  constructor(doc, tag, id) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.nodeType = 1;
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.className = '';
    this.dataset = {};
    this.hidden = false;
    this.disabled = false;
    // Measurements a test sets.
    this.rect = { top: 0, bottom: 0 };
    this.scrollHeight = 0;
    this.clientHeight = 0;
    this.scrollTop = 0;
    this.scrolls = [];
    this.headNode = null;
  }
  get classList() {
    const el = this;
    const names = () => (el.className ? el.className.split(/\s+/).filter(Boolean) : []);
    return {
      add: (c) => { if (!names().includes(c)) { el.className = names().concat(c).join(' '); } },
      remove: (c) => { el.className = names().filter((n) => n !== c).join(' '); },
      contains: (c) => names().includes(c),
      toggle(c, force) {
        const on = force === undefined ? !names().includes(c) : !!force;
        if (on) { this.add(c); } else { this.remove(c); }
        return on;
      }
    };
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  setAttribute(n, v) { if (n === 'hidden') { this.hidden = true; } this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return n === 'hidden' ? this.hidden === true : Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { if (n === 'hidden') { this.hidden = false; } delete this.attrs[n]; }
  detach(node) { if (node.parentNode) { node.parentNode.removeChild(node); } }
  appendChild(node) { this.detach(node); this.children.push(node); node.parentNode = this; return node; }
  insertBefore(node, ref) {
    this.detach(node);
    const at = ref ? this.children.indexOf(ref) : -1;
    if (at === -1) { this.children.push(node); } else { this.children.splice(at, 0, node); }
    node.parentNode = this;
    return node;
  }
  removeChild(node) {
    const at = this.children.indexOf(node);
    if (at !== -1) { this.children.splice(at, 1); }
    node.parentNode = null;
    return node;
  }
  remove() { if (this.parentNode) { this.parentNode.removeChild(this); } }
  get firstChild() { return this.children[0] || null; }
  get nextElementSibling() {
    const sibs = this.parentNode ? this.parentNode.children : [];
    return sibs[sibs.indexOf(this) + 1] || null;
  }
  get previousElementSibling() {
    const sibs = this.parentNode ? this.parentNode.children : [];
    return sibs[sibs.indexOf(this) - 1] || null;
  }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  closest() { return null; }
  getClientRects() { return []; }
  getBoundingClientRect() { return this.rect; }
  scrollTo(opts) { this.scrolls.push(opts); }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener() {}
  querySelector(sel) { return sel === '.list-head' ? this.headNode : null; }
  querySelectorAll() { return []; }
  focus() { this.doc.activeElement = this; }
}

const PAGE_IDS = [
  'report-list', 'report-sheet', 'feed-skeleton', 'feed-empty', 'disputed-empty', 'confirmed-empty', 'feed-error', 'feed-retry',
  'view-toggle', 'queue-strip', 'show-button', 'feed-count', 'feed-status', 'feed-empty-history',
  'filter-empty', 'filter-empty-clear', 'feed-empty-history-body', 'feed-stale', 'feed-stale-text',
  'feed-stale-retry', 'notice-live', 'notice-card', 'notice-slow', 'notice-stale', 'notice-stale-text',
  'notice-queue', 'notice-queue-text', 'notice-queue-show'
];

// load evaluates the scripts against a Nearby page. layout is 'laptop' or
// 'phone': it answers the narrow query. calm answers the reduced motion query.
function load({ layout = 'laptop', calm = false } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    createElementNS(ns, tag) { return new FakeEl(doc, tag); },
    createTextNode(text) { return { nodeType: 3, textContent: String(text), parentNode: null }; },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener() {},
    removeEventListener() {},
    body: null
  };
  doc.documentElement = new FakeEl(doc, 'html');
  doc.body = new FakeEl(doc, 'body');
  PAGE_IDS.forEach((id) => { doc.byId[id] = new FakeEl(doc, id === 'report-list' ? 'ol' : 'div', id); });
  ['feed-empty', 'disputed-empty', 'confirmed-empty', 'feed-empty-history', 'filter-empty', 'feed-error', 'feed-skeleton'].forEach((id) => {
    doc.byId[id].setAttribute('hidden', '');
  });

  const windowScrolls = [];
  const window = {
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    scrollTo(...args) { windowScrolls.push(args); },
    scroll(...args) { windowScrolls.push(args); },
    getComputedStyle: () => ({ getPropertyValue: (name) => (name === '--space-sm' ? '8px' : '') }),
    location: { search: '', replace() {} },
    matchMedia: (query) => {
      if (query.includes('prefers-reduced-motion')) {
        return { matches: calm, addEventListener() {} };
      }
      return { matches: layout === 'phone' && query.includes('max-width'), addEventListener() {} };
    }
  };
  const sandbox = {
    window,
    document: doc,
    navigator: {},
    fetch: () => new Promise(() => {}),
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    URLSearchParams,
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent, Event: class Event {}
  };
  vm.createContext(sandbox);
  vm.runInContext(read('app.js'), sandbox);
  sandbox.Pinalert = window.Pinalert;
  vm.runInContext(read('visibility.js'), sandbox);
  sandbox.PinalertVisibility = window.PinalertVisibility;
  vm.runInContext(read('trust.js'), sandbox);
  sandbox.PinalertTrust = window.PinalertTrust;
  window.PinalertVotes = { farFromCachedFix: () => false };
  sandbox.PinalertVotes = window.PinalertVotes;
  vm.runInContext(read('show-menu.js'), sandbox);
  sandbox.PinalertShowMenu = window.PinalertShowMenu;
  vm.runInContext(read('feed.js'), sandbox);
  return { Pinalert: window.Pinalert, doc, windowScrolls, el: (id) => doc.byId[id] };
}

const NOW = '2026-10-16T10:00:00Z';
const report = (id) => ({
  id,
  view: 'active',
  severity: 'low',
  category: 'flood',
  description: 'Report ' + id,
  created_at: NOW,
  expires_at: '2026-10-17T10:00:00Z',
  latitude: 12.9,
  longitude: 77.5,
  visibility: 'live',
  confirmed_by_people: true
});

const SAMPLE = [report(1), report(2), report(3), report(4), report(5)];

function show(m, reports) {
  const store = m.Pinalert.state;
  store.reports = reports;
  store.status = 'ok';
  store.hasLoaded = true;
  m.Pinalert.redraw();
}

const rowOf = (m, id) => m.el('report-list').children.find((li) => li.dataset.reportId === String(id));

// place gives a scroll box and its rows measurements: the box starts at boxTop,
// is boxHeight tall with content of contentHeight, and every row is rowHeight
// tall one after the other from the top of the content (less scrollTop).
function place(box, rows, { boxTop, boxHeight, contentHeight, scrollTop = 0, rowHeight }) {
  box.rect = { top: boxTop, bottom: boxTop + boxHeight };
  box.clientHeight = boxHeight;
  box.scrollHeight = contentHeight;
  box.scrollTop = scrollTop;
  rows.forEach((li, i) => {
    const top = boxTop + i * rowHeight - scrollTop;
    li.rect = { top, bottom: top + rowHeight };
  });
}

test('laptop: a pin whose row is below the fold scrolls the cards list with scrollTo', () => {
  const m = load();
  show(m, SAMPLE);
  const cards = m.el('report-list');
  const rows = cards.children;
  // The cards list starts 120px down the screen (under the top bar and the
  // list head); five rows of 100px in a 250px window.
  place(cards, rows, { boxTop: 120, boxHeight: 250, contentHeight: 500, rowHeight: 100 });

  m.Pinalert.select(5, 'map');

  assert.equal(cards.scrolls.length, 1, 'the cards list is scrolled once');
  assert.equal(cards.scrolls[0].top, 400, 'to the row offset in the cards list, with nothing taken off for the head');
  assert.equal(cards.scrolls[0].behavior, 'smooth');
  assert.equal(m.el('report-sheet').scrolls.length, 0, 'the pane is not the scroller on a laptop');
  assert.equal(m.windowScrolls.length, 0, 'the page is never scrolled');
  assert.equal(rowOf(m, 5).classList.contains('report-row--selected'), true, 'the row takes the chosen fill');
});

test('laptop: a row that is already whole in view causes no scroll', () => {
  const m = load();
  show(m, SAMPLE);
  const cards = m.el('report-list');
  place(cards, cards.children, { boxTop: 120, boxHeight: 250, contentHeight: 500, rowHeight: 100 });

  m.Pinalert.select(2, 'map');

  assert.equal(cards.scrolls.length, 0);
  assert.equal(m.windowScrolls.length, 0);
  assert.equal(rowOf(m, 2).classList.contains('report-row--selected'), true, 'the row is still chosen');
});

test('laptop: a row partly under the bottom edge is brought up, one above the top edge is brought down', () => {
  const m = load();
  show(m, SAMPLE);
  const cards = m.el('report-list');
  // Scrolled 150px in a 250px window: the window shows 150..400 of the
  // content, so row 1 (0..100) is above the top edge, row 2 (100..200) is cut
  // at the top, rows 3 and 4 are whole (200..400) and row 5 (400..500) is
  // below the bottom edge.
  place(cards, cards.children, { boxTop: 120, boxHeight: 250, contentHeight: 500, scrollTop: 150, rowHeight: 100 });

  m.Pinalert.select(1, 'map');
  assert.equal(cards.scrolls.length, 1);
  assert.equal(cards.scrolls[0].top, 0, 'row 1 goes to the top of the cards list');

  m.Pinalert.select(2, 'map');
  assert.equal(cards.scrolls.length, 2);
  assert.equal(cards.scrolls[1].top, 100, 'row 2 goes to the top edge');

  m.Pinalert.select(4, 'map');
  assert.equal(cards.scrolls.length, 2, 'row 4 ends exactly at the bottom edge, so it is whole and stays');

  m.Pinalert.select(5, 'map');
  assert.equal(cards.scrolls.length, 3);
  assert.equal(cards.scrolls[2].top, 400, 'row 5 goes to the top edge');
});

test('laptop: a list that does not overflow is never scrolled', () => {
  const m = load();
  show(m, SAMPLE.slice(0, 2));
  const cards = m.el('report-list');
  place(cards, cards.children, { boxTop: 120, boxHeight: 400, contentHeight: 400, rowHeight: 100 });

  m.Pinalert.select(2, 'map');

  assert.equal(cards.scrolls.length, 0);
});

test('phone: the sheet is still the scroller and the row ends under the sticky head with a gap', () => {
  const m = load({ layout: 'phone' });
  show(m, SAMPLE);
  const sheet = m.el('report-sheet');
  const cards = m.el('report-list');
  const head = new FakeEl(m.doc, 'div');
  head.className = 'list-head';
  sheet.appendChild(head);
  sheet.headNode = head;
  // The sheet starts 300px down, 300px tall; the head ends 56px into it. Rows
  // follow the head and are 100px tall.
  sheet.rect = { top: 300, bottom: 600 };
  sheet.clientHeight = 300;
  sheet.scrollHeight = 700;
  sheet.scrollTop = 0;
  head.rect = { top: 300, bottom: 356 };
  cards.children.forEach((li, i) => {
    li.rect = { top: 356 + i * 100, bottom: 456 + i * 100 };
  });

  m.Pinalert.select(5, 'map');

  assert.equal(sheet.scrolls.length, 1);
  // Row 5 starts 456 into the sheet's box; the head is 56 and the gap 8.
  assert.equal(sheet.scrolls[0].top, 456 - 56 - 8 + 0);
  assert.equal(cards.scrolls.length, 0, 'the cards list is not the scroller on a phone');
  assert.equal(m.windowScrolls.length, 0);
});

test('reduced motion scrolls at once', () => {
  const m = load({ calm: true });
  show(m, SAMPLE);
  const cards = m.el('report-list');
  place(cards, cards.children, { boxTop: 120, boxHeight: 250, contentHeight: 500, rowHeight: 100 });

  m.Pinalert.select(5, 'map');

  assert.equal(cards.scrolls[0].behavior, 'auto');
});

test('a list originated choice does not scroll the list', () => {
  const m = load();
  show(m, SAMPLE);
  const cards = m.el('report-list');
  place(cards, cards.children, { boxTop: 120, boxHeight: 250, contentHeight: 500, rowHeight: 100 });

  m.Pinalert.select(5, 'list');

  assert.equal(cards.scrolls.length, 0);
});

test('feed.js never calls scrollIntoView', () => {
  assert.equal(/scrollIntoView/.test(read('feed.js')), false);
});
