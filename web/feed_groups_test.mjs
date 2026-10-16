// feed_groups_test.mjs - runs the real web/static/js/feed.js (with the real
// app.js, visibility.js and trust.js) in a Node sandbox and checks what the
// Nearby list draws under each Show choice (Phase 3.4, D-13, D-14, TRUST-02):
//
//   - Show All draws the not disputed rows first, then the heading
//     "Disputed: may not be true", then the disputed rows, in that order;
//   - a disputed row carries its cues (the Disputed class, the bold line, the
//     screen reader lead), and a provisional row starts "Not confirmed yet.";
//   - Show Disputed draws only disputed rows with no heading, Show Confirmed
//     only rows whose server field confirmed_by_people is true;
//   - the count line reads "{N} reports, {R} km" under the Show choice;
//   - each empty state is chosen by the Show choice;
//   - a link to a finished report leaves Nearby once, for My activity.
//
// Run it with: node --test web/feed_groups_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the scripts call: a small element model with a
// class list over the class name, an insertBefore that moves a node, and fake
// timers that never run.

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
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener() {}
  querySelector() { return null; }
  querySelectorAll() { return []; }
  focus() { this.doc.activeElement = this; }
}

const PAGE_IDS = [
  'report-list', 'feed-skeleton', 'feed-empty', 'disputed-empty', 'confirmed-empty', 'feed-error', 'feed-retry',
  'view-toggle', 'queue-strip', 'show-button', 'feed-count', 'feed-status', 'feed-empty-history',
  'filter-empty', 'filter-empty-clear', 'feed-empty-history-body', 'feed-stale', 'feed-stale-text',
  'feed-stale-retry', 'notice-live', 'notice-card', 'notice-slow', 'notice-stale', 'notice-stale-text',
  'notice-queue', 'notice-queue-text', 'notice-queue-show'
];

// load evaluates the scripts against a Nearby page. replaced keeps the
// addresses the page was sent to with location.replace.
function load({ search = '' } = {}) {
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

  const replaced = [];
  const window = {
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    location: { search, replace: (to) => replaced.push(to) },
    matchMedia: () => ({ matches: false, addEventListener() {} })
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
  return { Pinalert: window.Pinalert, doc, replaced, el: (id) => doc.byId[id] };
}

const NOW = '2026-10-16T10:00:00Z';
const report = (id, view, extra = {}) => ({
  id,
  view,
  severity: 'low',
  category: 'flood',
  description: 'Report ' + id,
  created_at: NOW,
  expires_at: '2026-10-17T10:00:00Z',
  latitude: 12.9,
  longitude: 77.5,
  visibility: view === 'disputed' ? 'hidden' : 'live',
  ...extra
});

// show puts reports in the store as an answer that has landed and draws them.
function show(m, reports, extra = {}) {
  const store = m.Pinalert.state;
  store.reports = reports;
  store.status = 'ok';
  store.hasLoaded = true;
  Object.assign(store, extra);
  m.Pinalert.redraw();
}

const list = (m) => m.el('report-list').children;
const ids = (m) => list(m).map((li) => (li.classList.contains('report-group-head') ? 'head' : li.dataset.reportId));
const shown = (el) => !el.hasAttribute('hidden');

const SAMPLE = [
  report(1, 'active', { confirmed_by_people: true, severity: 'medium' }),
  report(2, 'disputed', { confirmed_by_people: false }),
  report(3, 'active', { confirmed_by_people: false, visibility: 'provisional', severity: 'critical' }),
  report(4, 'disputed', { confirmed_by_people: false, severity: 'critical' })
];

test('Show All draws the not disputed rows first, then the heading, then the disputed rows', () => {
  const m = load();
  show(m, SAMPLE);
  // The feed's own order is critical first: the provisional Critical report
  // leads its group, and the disputed Critical report leads the last group.
  assert.deepEqual(ids(m), ['3', '1', 'head', '4', '2']);
  const head = list(m)[2];
  assert.equal(head.textContent, 'Disputed: may not be true');
});

test('a disputed row carries its cues, a live row carries none', () => {
  const m = load();
  show(m, SAMPLE);
  const byId = Object.fromEntries(list(m).filter((li) => li.dataset.reportId).map((li) => [li.dataset.reportId, li]));
  const parts = (li) => {
    const body = li.children[1];
    return {
      lead: body.children[0],
      line: body.children.find((c) => c.className.includes('report-row__disputed-line'))
    };
  };

  assert.equal(byId['2'].classList.contains('report-row--disputed'), true);
  assert.equal(parts(byId['2']).line.hidden, false);
  assert.equal(parts(byId['2']).line.textContent, 'Disputed: may not be true');
  assert.equal(parts(byId['2']).lead.textContent, 'Disputed, may not be true.');
  assert.equal(parts(byId['2']).lead.hidden, false);

  assert.equal(byId['1'].classList.contains('report-row--disputed'), false);
  assert.equal(parts(byId['1']).line.hidden, true);
  assert.equal(parts(byId['1']).lead.hidden, true);

  // A provisional row starts "Not confirmed yet."
  assert.equal(parts(byId['3']).lead.textContent, 'Not confirmed yet.');
  assert.equal(parts(byId['3']).lead.hidden, false);
  assert.equal(byId['3'].classList.contains('report-row--disputed'), false);
});

test('no row is ever faded by script: no opacity is set anywhere', () => {
  const m = load();
  show(m, SAMPLE);
  list(m).forEach((li) => {
    assert.equal(li.attrs.style, undefined, 'no inline style on a row');
  });
  const code = read('feed.js').replace(/\/\/.*$/gm, '');
  assert.equal(/opacity|\.style\b|cssText/.test(code), false, 'feed.js sets no style at all');
});

test('Show Disputed draws only the disputed rows and no heading', () => {
  const m = load();
  m.Pinalert.setShow('disputed');
  show(m, [SAMPLE[1], SAMPLE[3]]);
  assert.deepEqual(ids(m), ['4', '2']);
});

test('Show Confirmed draws only reports whose server field is true', () => {
  const m = load();
  m.Pinalert.setShow('confirmed');
  show(m, SAMPLE);
  assert.deepEqual(ids(m), ['1']);
});

test('changing from All to Confirmed takes the heading and the disputed rows out of the list', () => {
  const m = load();
  show(m, SAMPLE);
  assert.equal(list(m).length, 5);
  m.Pinalert.setShow('confirmed');
  m.Pinalert.redraw();
  assert.deepEqual(ids(m), ['1']);
  m.Pinalert.setShow('all');
  m.Pinalert.redraw();
  assert.deepEqual(ids(m), ['3', '1', 'head', '4', '2']);
});

test('the heading goes when the last disputed row goes', () => {
  const m = load();
  show(m, SAMPLE);
  show(m, [SAMPLE[0], SAMPLE[2]]);
  assert.deepEqual(ids(m), ['3', '1']);
});

test('the heading is drawn when every report is disputed', () => {
  const m = load();
  show(m, [SAMPLE[1], SAMPLE[3]]);
  assert.deepEqual(ids(m), ['head', '4', '2']);
});

test('the count line reads {N} reports, {R} km under the Show choice', () => {
  const m = load();
  show(m, SAMPLE);
  assert.equal(m.el('feed-count').textContent, '4 reports, 10 km', 'All counts the disputed group too');
  m.Pinalert.setShow('confirmed');
  m.Pinalert.redraw();
  assert.equal(m.el('feed-count').textContent, '1 report, 10 km');
});

test('the empty state follows the Show choice, one block at a time', () => {
  const m = load();
  const shownBlocks = () => ['feed-empty', 'feed-empty-history', 'confirmed-empty', 'disputed-empty', 'filter-empty']
    .filter((id) => shown(m.el(id)));

  show(m, []);
  assert.deepEqual(shownBlocks(), ['feed-empty']);

  show(m, [], { recent: { resolved_24h: 2, expired_24h: 0 } });
  assert.deepEqual(shownBlocks(), ['feed-empty-history']);
  assert.equal(m.el('feed-empty-history-body').textContent,
    "2 were resolved in the last 24 hours. Post a report to share what you're seeing.");

  m.Pinalert.setShow('confirmed');
  show(m, [SAMPLE[1]], { recent: null });
  assert.deepEqual(shownBlocks(), ['confirmed-empty'], 'reports exist but none are confirmed');

  m.Pinalert.setShow('disputed');
  show(m, []);
  assert.deepEqual(shownBlocks(), ['disputed-empty']);

  m.Pinalert.setShow('all');
  m.Pinalert.setCategory('flood');
  show(m, []);
  assert.deepEqual(shownBlocks(), ['filter-empty'], 'a chip that emptied the list says so');
});

test('a link to a finished report leaves Nearby once, for My activity', () => {
  const m = load();
  show(m, SAMPLE, { focus: { status: 'ok', report: report(55, 'resolved') } });
  assert.deepEqual(m.replaced, ['/profile?part=ended&report=55']);
  m.Pinalert.redraw();
  assert.equal(m.replaced.length, 1, 'later answers for the same link do not leave again');
});

test('a faded report and a hostile id also leave, with no id carried when it is not plain digits', () => {
  const faded = load();
  show(faded, SAMPLE, { focus: { status: 'ok', report: report(8, 'past') } });
  assert.deepEqual(faded.replaced, ['/profile?part=ended&report=8']);

  const hostile = load();
  show(hostile, SAMPLE, { focus: { status: 'ok', report: report('7&next=https://evil.example', 'resolved') } });
  assert.deepEqual(hostile.replaced, ['/profile?part=ended']);
});

test('a link to an active or disputed report stays on Nearby', () => {
  const active = load();
  show(active, SAMPLE, { focus: { status: 'ok', report: report(1, 'active') } });
  assert.deepEqual(active.replaced, []);

  const disputed = load();
  show(disputed, SAMPLE, { focus: { status: 'ok', report: report(2, 'disputed') } });
  assert.deepEqual(disputed.replaced, []);
});
