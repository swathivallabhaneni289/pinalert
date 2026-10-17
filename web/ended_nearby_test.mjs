// ended_nearby_test.mjs - runs the real web/static/js/ended-cards.js and
// ended-nearby.js (with the real app.js, position.js and trust.js) in a Node
// sandbox and checks My activity, part Ended nearby (Phase 3.4, plan 14, D-18
// to D-21, orchestrator decision 6):
//
//   - with no kept position nothing is requested and the location state shows;
//   - with a position one read of the Past answer is sent, with the position,
//     the page's radius and, for a link to a finished report, the report id;
//   - the answer is drawn in two groups, Resolved then Faded out, in the
//     server's order, with the cut and partial lines and the empty states;
//   - a partial flag never leaves an empty block (WR-02, handed over by plan 11);
//   - a failed read shows the error state and Try again sends one new read;
//   - a finished Rescue row shows the coarse words and never the description;
//   - the Distance cells of the other parts are filled from the kept position;
//   - a link to a finished report chooses its row and scrolls the page's own
//     region, never the window.
//
// Run it with: node --test web/ended_nearby_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the scripts call: a small element model with a
// class list over the class name and a class selector over the children.

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
    this.rectTop = 0;
    this.scrollTop = 0;
    this.focused = null;
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
  get firstChild() { return this.children[0] || null; }
  getBoundingClientRect() { return { top: this.rectTop }; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  click() { (this.handlers.click || []).forEach((fn) => fn({ type: 'click', target: this })); }
  querySelectorAll(selector) {
    const found = [];
    const klass = selector.startsWith('.') ? selector.slice(1) : null;
    const walk = (node) => {
      node.children.forEach((child) => {
        if (child.nodeType === 1) {
          if (klass && child.classList.contains(klass)) { found.push(child); }
          walk(child);
        }
      });
    };
    walk(this);
    return found;
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  focus(options) { this.doc.activeElement = this; this.focused = options || {}; }
  // Plan 19: the take back asks for its nearest popup ancestor and whether a
  // node holds focus; neither matters in these sandboxes.
  closest() { return null; }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
}

const IDS = {
  'ended-skeleton': 'div', 'ended-cols': 'div',
  'ended-resolved': 'div', 'ended-resolved-list': 'ol', 'ended-resolved-cut': 'p',
  'ended-faded': 'div', 'ended-faded-list': 'ol', 'ended-faded-cut': 'p',
  'ended-empty': 'div', 'ended-location': 'div', 'ended-error': 'div', 'ended-retry': 'button',
  'ended-hub-count': 'span', 'ended-title-count': 'span'
};

// load evaluates the scripts against a My activity page. position is what the
// browser has kept ('none' for nothing). answers is a list of functions or
// objects, one per read: an object is the JSON answer, an Error rejects.
function load({ search = '', position = null, radius = '12', answers = [], cells = [], pageAccount = '7', savedBy = '7' } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    createElementNS(ns, tag) { return new FakeEl(doc, tag); },
    createTextNode(text) { return { nodeType: 3, textContent: String(text), parentNode: null }; },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector(sel) { return sel === '.profile-page' || sel === '[data-account-id]' ? doc.page : null; },
    querySelectorAll(sel) { return sel === '.activity-card__distance' ? cells : []; },
    addEventListener() {},
    removeEventListener() {},
    body: null
  };
  doc.documentElement = new FakeEl(doc, 'html');
  doc.body = new FakeEl(doc, 'body');
  doc.page = new FakeEl(doc, 'main');
  doc.page.setAttribute('data-default-radius-km', radius);
  if (pageAccount !== null) { doc.page.setAttribute('data-account-id', pageAccount); }
  const region = new FakeEl(doc, 'div');
  region.overflowY = 'auto';
  region.scrollHeight = 2000;
  region.clientHeight = 600;
  const root = new FakeEl(doc, 'div', 'ended-nearby');
  doc.byId['ended-nearby'] = root;
  region.appendChild(root);
  Object.entries(IDS).forEach(([id, tag]) => { doc.byId[id] = new FakeEl(doc, tag, id); });
  const by = (id) => doc.byId[id];
  ['ended-cols', 'ended-resolved', 'ended-faded', 'ended-empty', 'ended-location', 'ended-error',
    'ended-resolved-cut', 'ended-faded-cut', 'ended-hub-count', 'ended-title-count'].forEach((id) => by(id).setAttribute('hidden', ''));
  by('ended-resolved').appendChild(by('ended-resolved-list'));
  by('ended-resolved').appendChild(by('ended-resolved-cut'));
  by('ended-faded').appendChild(by('ended-faded-list'));
  by('ended-faded').appendChild(by('ended-faded-cut'));
  ['ended-skeleton', 'ended-cols', 'ended-resolved', 'ended-faded', 'ended-empty', 'ended-location', 'ended-error']
    .forEach((id) => root.appendChild(by(id)));

  const stored = new Map();
  if (position && position !== 'none') {
    stored.set('pinalert.lowLastPosition', JSON.stringify({ lat: position[0], lon: position[1], savedAt: 1, account: savedBy }));
  }
  const writes = [];
  const fetches = [];
  const toasts = [];
  const sandbox = {
    document: doc,
    navigator: {},
    localStorage: {
      getItem: (k) => (stored.has(k) ? stored.get(k) : null),
      setItem: (k, v) => { writes.push([k, v]); stored.set(k, String(v)); },
      removeItem: () => {}
    },
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    matchMedia: () => ({ matches: false, addEventListener() {} }),
    getComputedStyle: (node) => ({ overflowY: node.overflowY || 'visible' }),
    location: { search },
    fetch: (target, init) => {
      fetches.push({ url: String(target), init });
      const next = answers.shift();
      if (next instanceof Error) { return Promise.reject(next); }
      const body = typeof next === 'function' ? next() : next;
      // Plan 19: { refuse: status, error: {...} } is a refused write.
      if (body && body.refuse) {
        return Promise.resolve({
          ok: false, status: body.refuse, headers: { get: () => null },
          json: () => Promise.resolve({ error: body.error })
        });
      }
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(body) });
    },
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    AbortController,
    URLSearchParams,
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent,
    Event: class Event {}
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(read('app.js'), sandbox);
  vm.runInContext(read('trust.js'), sandbox);
  vm.runInContext(read('position.js'), sandbox);
  vm.runInContext(read('votes.js'), sandbox);
  vm.runInContext(read('ended-cards.js'), sandbox);
  sandbox.Pinalert.showToast = (message) => toasts.push(message);
  return {
    sandbox, doc, region, writes, fetches, toasts, by,
    start() {
      vm.runInContext(read('ended-nearby.js'), sandbox);
      return sandbox.PinalertEndedNearby;
    }
  };
}

const NOW = new Date(Date.now() - 3600 * 1000).toISOString();
const TRUST = { confirmed_places: 3, disputed_places: 1, confirmed_text: '3', disputed_text: '1', facts: [] };
const resolvedReport = (id, extra = {}) => ({
  id, view: 'resolved', severity: 'medium', category: 'flood', description: 'Resolved report ' + id,
  distance_km: 2.46, approximate: false, trust: TRUST,
  history: { state: 'resolved', at: NOW, closed_by: 'poster', closed_text: 'Resolved by the poster' },
  ...extra
});
const expiredReport = (id, extra = {}) => ({
  id, view: 'past', severity: 'low', category: 'power_outage', description: 'Faded report ' + id,
  distance_km: 14.2, approximate: false, trust: TRUST,
  history: { state: 'expired', at: NOW, outcome: 'unconfirmed', outcome_label: 'Expired unconfirmed', lifetime_text: 'after 6 hrs' },
  ...extra
});
const answer = (reports, extra = {}) => ({ reports, cut: {}, ...extra });

const settle = () => new Promise((resolve) => setImmediate(resolve));
const shown = (el) => el.hidden === false;
const rowsOf = (m, id) => m.by(id).children;
const cell = (m, row, klass) => row.querySelectorAll('.' + klass)[0];

test('with no kept position nothing is requested and the location state shows', async () => {
  const cells = [new FakeEl(null, 'p'), new FakeEl(null, 'p')];
  cells.forEach((c) => { c.setAttribute('data-lat', '12.9'); c.setAttribute('data-lon', '77.5'); c.own = 'old'; });
  const m = load({ cells });
  m.start();
  await settle();
  assert.equal(m.fetches.length, 0);
  assert.equal(shown(m.by('ended-location')), true);
  assert.equal(shown(m.by('ended-skeleton')), false);
  assert.equal(shown(m.by('ended-error')), false);
  assert.equal(shown(m.by('ended-empty')), false);
  assert.equal(m.by('ended-hub-count').hidden, true, 'the hub count stays empty');
  cells.forEach((c) => assert.equal(c.textContent, 'Unknown'));
});

test('with a position one read is sent with the position, the page radius and view=past, and the skeleton shows meanwhile', async () => {
  const m = load({ position: [12.97, 77.59], answers: [answer([])] });
  m.start();
  assert.equal(m.fetches.length, 1);
  const sent = new URL(m.fetches[0].url, 'http://x');
  assert.equal(sent.pathname, '/api/reports');
  assert.equal(sent.searchParams.get('view'), 'past');
  assert.equal(sent.searchParams.get('lat'), '12.97');
  assert.equal(sent.searchParams.get('lon'), '77.59');
  assert.equal(sent.searchParams.get('radius_km'), '12');
  assert.equal(sent.searchParams.has('report'), false);
  assert.equal(shown(m.by('ended-skeleton')), true, 'four skeleton rows show while the read is out');
  assert.equal(m.fetches[0].init.method, undefined, 'a plain read, never a post');
  assert.equal(m.fetches[0].init.body, undefined);
  await settle();
  assert.equal(shown(m.by('ended-skeleton')), false);
  assert.equal(m.writes.length, 0, 'the position is read, never written');
});

test('an answer of two resolved and three expired reports draws the groups, newest first, with the server words', async () => {
  const reports = [resolvedReport(11), expiredReport(21), resolvedReport(12), expiredReport(22), expiredReport(23)];
  const m = load({ position: [12.97, 77.59], answers: [answer(reports)] });
  m.start();
  await settle();

  assert.equal(shown(m.by('ended-resolved')), true);
  assert.equal(shown(m.by('ended-faded')), true);
  assert.equal(shown(m.by('ended-cols')), true);
  assert.equal(shown(m.by('ended-empty')), false);
  assert.deepEqual(rowsOf(m, 'ended-resolved-list').map((r) => r.dataset.reportId), ['11', '12']);
  assert.deepEqual(rowsOf(m, 'ended-faded-list').map((r) => r.dataset.reportId), ['21', '22', '23']);

  const first = rowsOf(m, 'ended-resolved-list')[0];
  assert.equal(cell(m, first, 'report-row__title').textContent, 'Resolved report 11');
  assert.equal(cell(m, first, 'activity-chip').textContent, 'Resolved');
  assert.equal(cell(m, first, 'activity-card__happened').textContent, 'Resolved by the poster');
  assert.match(cell(m, first, 'activity-card__when').textContent, /^Resolved /);
  assert.equal(cell(m, first, 'activity-card__distance').textContent, '2.5 km');
  assert.equal(cell(m, first, 'report-row__meta').textContent, 'Flood');
  assert.ok(first.classList.contains('vis-retracted'));
  assert.ok(first.classList.contains('sev-medium'));

  const faded = rowsOf(m, 'ended-faded-list')[0];
  assert.equal(cell(m, faded, 'activity-chip').textContent, 'Expired');
  assert.equal(cell(m, faded, 'activity-card__happened').textContent, 'Expired unconfirmed');
  assert.match(cell(m, faded, 'activity-card__when').textContent, /^Faded .*, after 6 hrs$/);
  assert.equal(cell(m, faded, 'activity-card__distance').textContent, '14 km');
  assert.ok(faded.classList.contains('vis-expired'));

  assert.equal(m.by('ended-hub-count').textContent, '5', 'the hub count equals the rows shown');
  assert.equal(m.by('ended-hub-count').hidden, false);
  assert.equal(m.by('ended-title-count').textContent, '5');
});

test('a cut group says Showing the newest {N}. with the count it shows, and a group with no rows is not drawn', async () => {
  const m = load({
    position: [12.97, 77.59],
    answers: [answer([resolvedReport(1), resolvedReport(2)], { cut: { resolved: true } })]
  });
  m.start();
  await settle();
  assert.equal(shown(m.by('ended-resolved')), true);
  assert.equal(shown(m.by('ended-resolved-cut')), true);
  assert.equal(m.by('ended-resolved-cut').textContent, 'Showing the newest 2.');
  assert.equal(shown(m.by('ended-faded')), false, 'a group with no rows is not drawn');
  assert.equal(shown(m.by('ended-faded-cut')), false);
});

test('zero reports with no partial flag shows the empty state and a hub count of 0', async () => {
  const m = load({ position: [12.97, 77.59], answers: [answer([])] });
  m.start();
  await settle();
  assert.equal(shown(m.by('ended-empty')), true);
  assert.equal(shown(m.by('ended-resolved')), false);
  assert.equal(shown(m.by('ended-faded')), false);
  assert.equal(shown(m.by('ended-cols')), false);
  assert.equal(m.by('ended-hub-count').textContent, '0');
});

test('WR-02: a partial flag never leaves an empty block, and the partial line is its own sentence', async () => {
  const line = 'This area has more reports than this list can hold, so some are not shown.';

  // No reports at all, but the server could not read every one of the faded out
  // group: the empty state must not be claimed.
  let m = load({ position: [12.97, 77.59], answers: [answer([], { cut: { expired_partial: true } })] });
  m.start();
  await settle();
  assert.equal(shown(m.by('ended-empty')), false, 'no empty block while a partial flag is set');
  assert.equal(shown(m.by('ended-faded')), true, 'the partial group stays on screen');
  assert.equal(m.by('ended-faded-cut').textContent, line);
  assert.equal(shown(m.by('ended-faded-cut')), true);
  assert.equal(shown(m.by('ended-resolved')), false, 'the group without the flag and without rows is not drawn');

  // Cards in one group and a partial flag on the other: both are shown.
  m = load({
    position: [12.97, 77.59],
    answers: [answer([resolvedReport(1)], { cut: { resolved: false, expired_partial: true } })]
  });
  m.start();
  await settle();
  assert.equal(shown(m.by('ended-resolved')), true);
  assert.equal(shown(m.by('ended-resolved-cut')), false);
  assert.equal(shown(m.by('ended-faded')), true);
  assert.equal(m.by('ended-faded-cut').textContent, line);

  // A group that is cut and partial says both sentences, each for its own flag.
  m = load({
    position: [12.97, 77.59],
    answers: [answer([resolvedReport(1), resolvedReport(2)], { cut: { resolved: true, resolved_partial: true } })]
  });
  m.start();
  await settle();
  assert.equal(m.by('ended-resolved-cut').textContent, 'Showing the newest 2. ' + line);

  // A partial flag alone never prints the cap line.
  m = load({ position: [12.97, 77.59], answers: [answer([], { cut: { resolved_partial: true } })] });
  m.start();
  await settle();
  assert.equal(m.by('ended-resolved-cut').textContent, line);
  assert.equal(m.by('ended-resolved-cut').textContent.includes('Showing the newest'), false);
});

test('a failed read shows the error state and Try again sends one new read', async () => {
  const m = load({
    position: [12.97, 77.59],
    answers: [new Error('network down'), answer([resolvedReport(7)])]
  });
  m.start();
  await settle();
  assert.equal(m.fetches.length, 1);
  assert.equal(shown(m.by('ended-error')), true);
  assert.equal(shown(m.by('ended-skeleton')), false);
  assert.equal(shown(m.by('ended-resolved')), false);

  m.by('ended-retry').click();
  assert.equal(m.fetches.length, 2, 'Try again sends exactly one new read');
  assert.equal(shown(m.by('ended-error')), false);
  assert.equal(shown(m.by('ended-skeleton')), true);
  await settle();
  assert.equal(shown(m.by('ended-resolved')), true);
  assert.equal(shown(m.by('ended-error')), false);
});

test('a read that answers with an error status shows the error state', async () => {
  const m = load({ position: [12.97, 77.59] });
  m.sandbox.fetch = () => Promise.resolve({ ok: false, status: 500, json: () => Promise.resolve({}) });
  m.start();
  await settle();
  assert.equal(shown(m.by('ended-error')), true);
});

test('a finished Rescue row shows the coarse words and never reads the description (D-23)', async () => {
  const rescue = resolvedReport(31, {
    category: 'rescue_needed', severity: 'critical', approximate: true, distance_km: 0.4,
    description: 'SECRET street address', latitude: 12.5, longitude: 77.5
  });
  const m = load({ position: [12.97, 77.59], answers: [answer([rescue])] });
  m.start();
  await settle();
  const row = rowsOf(m, 'ended-resolved-list')[0];
  assert.equal(cell(m, row, 'report-row__title').textContent, 'Rescue needed');
  assert.equal(cell(m, row, 'report-row__meta').textContent, 'Approximate location');
  assert.equal(cell(m, row, 'activity-card__distance').textContent, 'about 1 km');
  assert.equal(row.textContent.includes('SECRET'), false, 'the description never reaches the row');
});

test('the corroborated outcome leaves the confirm count to the label', async () => {
  const corroborated = expiredReport(41, {
    history: { state: 'expired', at: NOW, outcome: 'corroborated', outcome_label: 'Confirmed by 3 people nearby' }
  });
  const plain = expiredReport(42);
  const m = load({ position: [12.97, 77.59], answers: [answer([corroborated, plain])] });
  m.start();
  await settle();
  const [a, b] = rowsOf(m, 'ended-faded-list');
  const counts = (row) => row.querySelectorAll('.report-row__trustline-count');
  assert.equal(counts(a)[0].hidden, true);
  assert.equal(counts(a)[1].hidden, false);
  assert.equal(counts(b)[0].hidden, false);
});

test('fillDistances writes km words from the kept position and Unknown for a cell with no coordinates', async () => {
  const mk = (lat, lon) => {
    const c = new FakeEl(null, 'p');
    c.own = 'Unknown';
    if (lat !== null) { c.setAttribute('data-lat', lat); }
    if (lon !== null) { c.setAttribute('data-lon', lon); }
    return c;
  };
  const near = mk('12.9716', '77.5946');
  const far = mk('13.0827', '80.2707');
  const same = mk('12.9716', '77.5946');
  const blank = mk('', '');
  const ended = mk(null, null);
  ended.own = '3.0 km';
  const m = load({ position: [12.9716, 77.5946], answers: [answer([])], cells: [near, far, same, blank, ended] });
  const api = m.start();
  await settle();
  assert.equal(near.textContent, 'under 100 m');
  assert.equal(same.textContent, 'under 100 m');
  assert.match(far.textContent, /^2\d\d km$/, 'Bengaluru to Chennai is a whole number of km');
  assert.equal(blank.textContent, 'Unknown');
  assert.equal(ended.textContent, '3.0 km', 'a cell with no coordinates attribute at all is left to the row builder');

  api.fillDistances(null);
  assert.equal(far.textContent, 'Unknown');
  assert.equal(near.textContent, 'Unknown');

  assert.ok(Math.abs(api.greatCircleKm(0, 0, 0, 1) - 111.19) < 0.1);
});

test('the distance words round like formatDistance: one decimal under 10 km, whole from 10 km', () => {
  const m = load();
  const words = m.sandbox.PinalertEnded.kmWords;
  assert.equal(words(0.05), 'under 100 m');
  assert.equal(words(0.1), '0.1 km');
  assert.equal(words(9.94), '9.9 km');
  assert.equal(words(9.95), '10 km');
  assert.equal(words(42.4), '42 km');
  assert.equal(words(NaN), 'Unknown');
  assert.equal(words(-1), 'Unknown');
});

test('a link to a finished report sends the id, chooses its row and scrolls the page region, not the window', async () => {
  const reports = [resolvedReport(1), resolvedReport(2), expiredReport(3)];
  const m = load({
    search: '?part=ended&report=3',
    position: [12.97, 77.59],
    answers: [answer(reports, { focus: { status: 'ok', report: expiredReport(3) } })]
  });
  m.region.rectTop = 100;
  m.start();
  assert.equal(new URL(m.fetches[0].url, 'http://x').searchParams.get('report'), '3');
  // The row is 400px below the region's top once drawn.
  const original = FakeEl.prototype.getBoundingClientRect;
  FakeEl.prototype.getBoundingClientRect = function () {
    return this === m.region ? { top: 100 } : { top: this.dataset && this.dataset.reportId === '3' ? 500 : 0 };
  };
  await settle();
  FakeEl.prototype.getBoundingClientRect = original;

  const rows = rowsOf(m, 'ended-faded-list');
  assert.equal(rows.length, 1);
  assert.ok(rows[0].classList.contains('activity-card--chosen'));
  assert.equal(rowsOf(m, 'ended-resolved-list').some((r) => r.classList.contains('activity-card--chosen')), false);
  assert.equal(m.region.scrollTop, 400 - 16, 'the page region scrolls by the row offset');
  assert.equal(rows[0].focused.preventScroll, true);
});

test('a linked report outside the groups is drawn at the top of its group', async () => {
  const reports = [resolvedReport(1), resolvedReport(2)];
  const m = load({
    search: '?part=ended&report=9',
    position: [12.97, 77.59],
    answers: [answer(reports, { focus: { status: 'ok', report: resolvedReport(9) } })]
  });
  m.start();
  await settle();
  assert.deepEqual(rowsOf(m, 'ended-resolved-list').map((r) => r.dataset.reportId), ['9', '1', '2']);
  assert.ok(rowsOf(m, 'ended-resolved-list')[0].classList.contains('activity-card--chosen'));
  assert.equal(m.by('ended-hub-count').textContent, '3');
});

test('a linked report that is too old or unavailable is told once and nothing is chosen', async () => {
  let m = load({
    search: '?part=ended&report=5', position: [12.97, 77.59],
    answers: [answer([resolvedReport(1)], { focus: { status: 'too_old', report: null } })]
  });
  m.start();
  await settle();
  assert.deepEqual(m.toasts, ['That report is older than 7 days and is no longer shown.']);
  assert.equal(rowsOf(m, 'ended-resolved-list').some((r) => r.classList.contains('activity-card--chosen')), false);

  m = load({
    search: '?part=ended&report=5', position: [12.97, 77.59],
    answers: [answer([resolvedReport(1)], { focus: { status: 'unavailable', report: null } })]
  });
  m.start();
  await settle();
  assert.deepEqual(m.toasts, ["That report isn't available."]);
});

test('only a plain positive whole number is sent as the linked report (T-34-63)', async () => {
  for (const bad of ['?part=ended&report=abc', '?part=ended&report=0', '?part=ended&report=-3', '?part=ended&report=1.5',
    '?part=ended&report=007', '?part=ended&report=1%3Cscript%3E', '?part=posted&report=3', '?report=3']) {
    const m = load({ search: bad, position: [12.97, 77.59], answers: [answer([])] });
    m.start();
    assert.equal(new URL(m.fetches[0].url, 'http://x').searchParams.has('report'), false, bad);
    await settle();
  }
});

test('the page is untouched on another page: no Ended nearby element, no read', () => {
  const m = load({ position: [12.97, 77.59] });
  delete m.doc.byId['ended-nearby'];
  assert.equal(m.start(), null);
  assert.equal(m.fetches.length, 0);
});

// ---- Phase 3.4 plan 19: the take back of a mistaken Mark resolved (03.1 D-22,
// 03.4 D-21). The server decides who has one and how many undos are left; the
// row only shows what votes.js's one builder makes of those fields.

const MARKED = (left, extra = {}) => ({ your_resolution: 'resolve', undos_left: left, ...extra });
const undoButton = (el) => el.querySelectorAll('.vote-btn--takeback')[0];
const usedLine = (el) => el.querySelectorAll('.vote-takeback__count')[0];
const USED = 'You have used both undos on this report.';

function builtRow(report) {
  const m = load();
  const card = m.sandbox.PinalertEnded.createEndedRow(report.id);
  m.sandbox.PinalertEnded.updateEndedRow(card, report);
  return { m, card };
}

test('take back shows Undo for the person who marked it', () => {
  const { card } = builtRow(resolvedReport(7, MARKED(2)));
  assert.ok(card.takeBack, 'the row mounts the take back');
  const undo = undoButton(card.el);
  assert.ok(undo, 'the Undo button exists');
  assert.equal(undo.hidden, false);
  assert.ok(!undo.disabled, 'and is enabled');
  assert.equal(undo.querySelector('.vote-btn__label').textContent, 'Undo');
  assert.equal(undo.dataset.action, 'unresolve');
  assert.equal(usedLine(card.el).hidden, true, 'no used sentence while undos are left');
  assert.equal(card.takeBack.el.hidden, false);
  assert.ok(card.what.children.includes(card.takeBack.el), 'it is the last line of the What happened cell');
  assert.equal(card.takeBack.el.querySelectorAll('.vote-btn--reopen').length, 0, 'no Reopen on an Ended nearby row');
});

test('take back shows the used sentence when no undos are left', () => {
  const { card } = builtRow(resolvedReport(8, MARKED(0)));
  assert.ok(card.takeBack);
  assert.equal(undoButton(card.el).hidden, true, 'no Undo button');
  assert.equal(usedLine(card.el).hidden, false);
  assert.equal(usedLine(card.el).textContent, USED);
  assert.equal(card.takeBack.el.hidden, false);
});

test('take back shows nothing for anyone else', () => {
  for (const extra of [
    { your_resolution: null, undos_left: 2 },
    { your_resolution: 'reopen', undos_left: 2 },
    { your_resolution: null, undos_left: 0 },
    { your_resolution: 'resolve' },
    { your_resolution: null, undos_left: 2, is_own_report: true }
  ]) {
    const { card } = builtRow(resolvedReport(9, extra));
    assert.equal(card.takeBack, null, JSON.stringify(extra));
    assert.equal(undoButton(card.el), undefined);
    assert.equal(card.el.querySelectorAll('.vote-takeback').length, 0);
    assert.equal(card.el.querySelectorAll('button').length, 0, 'the row stays read only');
  }
});

test('take back is never offered on a report that faded out, and a later answer can take it away', () => {
  const faded = builtRow(expiredReport(10, MARKED(2)));
  assert.equal(faded.card.takeBack, null, 'a faded report cannot be changed');

  const { m, card } = builtRow(resolvedReport(11, MARKED(2)));
  m.sandbox.PinalertEnded.updateEndedRow(card, resolvedReport(11, { your_resolution: null, undos_left: 2 }));
  assert.equal(card.takeBack.el.hidden, true, 'the server stopped saying the viewer marked it');
  assert.equal(undoButton(card.el).hidden, true);
  m.sandbox.PinalertEnded.updateEndedRow(card, resolvedReport(11, MARKED(0)));
  assert.equal(usedLine(card.el).textContent, USED, 'and a later answer can show the used sentence');
  assert.equal(card.takeBack.el.hidden, false);
});

test('take back click sends unresolve for that report, reads the part again and the row leaves when the report is open again', async () => {
  const m = load({
    position: [12.97, 77.59],
    answers: [answer([resolvedReport(31, MARKED(2)), resolvedReport(32)]), { visibility: 'live', undos_left: 1 },
      answer([resolvedReport(32)])]
  });
  m.start();
  await settle();
  const undo = undoButton(rowsOf(m, 'ended-resolved-list')[0]);
  assert.ok(undo);
  assert.equal(undoButton(rowsOf(m, 'ended-resolved-list')[1]), undefined, 'the other row has none');
  undo.click();
  await settle();

  assert.equal(m.fetches.length, 3);
  const post = m.fetches[1];
  assert.equal(new URL(post.url, 'http://x').pathname, '/api/reports/31/unresolve');
  assert.equal(post.init.method, 'POST');
  assert.equal(post.init.body, '{}', 'a take back sends no coordinates');
  const reread = new URL(m.fetches[2].url, 'http://x');
  assert.equal(reread.pathname, '/api/reports');
  assert.equal(reread.searchParams.get('view'), 'past', 'the part is read again, not the Nearby list');
  assert.deepEqual(rowsOf(m, 'ended-resolved-list').map((r) => r.dataset.reportId), ['32']);
  assert.equal(m.by('ended-hub-count').textContent, '1');
  assert.deepEqual(m.toasts, [m.sandbox.PinalertVotes.TAKE_BACK_SUCCEEDED_CLOSED_TOAST]);
  assert.equal(shown(m.by('ended-skeleton')), false, 'no skeleton flashes during the quiet read');
});

test('take back refused by the server keeps the row, shows the server line and the part reads the answer', async () => {
  const refusal = 'You have used both undos on this report.';
  const m = load({
    position: [12.97, 77.59],
    answers: [answer([resolvedReport(41, MARKED(1))]), { refuse: 409, error: { field: 'undo', message: refusal } },
      answer([resolvedReport(41, MARKED(0))])]
  });
  m.start();
  await settle();
  undoButton(rowsOf(m, 'ended-resolved-list')[0]).click();
  await settle();

  assert.equal(m.fetches.length, 3, 'the refusal reads the part again');
  const row = rowsOf(m, 'ended-resolved-list')[0];
  assert.equal(row.dataset.reportId, '41', 'the row stays');
  assert.equal(undoButton(row).hidden, true, 'the new answer has no undo left');
  assert.equal(usedLine(row).textContent, USED);
  const line = row.querySelectorAll('.vote-error')[0];
  assert.equal(line.hidden, false, 'the refusal line is carried to the redrawn row');
  assert.equal(line.textContent, refusal);
  assert.deepEqual(m.toasts, []);
});

test('take back on a linked report drawn on top of its group keeps that row and its refusal line', async () => {
  const refusal = 'You have used both undos on this report.';
  const linked = resolvedReport(9, MARKED(1));
  const m = load({
    search: '?part=ended&report=9',
    position: [12.97, 77.59],
    answers: [
      answer([resolvedReport(1), resolvedReport(2)], { focus: { status: 'ok', report: linked } }),
      { refuse: 409, error: { field: 'undo', message: refusal } },
      answer([resolvedReport(1), resolvedReport(2)], { focus: { status: 'ok', report: resolvedReport(9, MARKED(0)) } })
    ]
  });
  m.start();
  await settle();
  assert.deepEqual(rowsOf(m, 'ended-resolved-list').map((r) => r.dataset.reportId), ['9', '1', '2']);
  undoButton(rowsOf(m, 'ended-resolved-list')[0]).click();
  await settle();

  assert.equal(new URL(m.fetches[2].url, 'http://x').searchParams.get('report'), '9', 'the quiet read still asks for the linked report');
  const rows = rowsOf(m, 'ended-resolved-list');
  assert.deepEqual(rows.map((r) => r.dataset.reportId), ['9', '1', '2'], 'the linked row stays');
  assert.ok(rows[0].classList.contains('activity-card--chosen'), 'and keeps its fill');
  const line = rows[0].querySelectorAll('.vote-error')[0];
  assert.equal(line.hidden, false);
  assert.equal(line.textContent, refusal);
  assert.equal(usedLine(rows[0]).textContent, USED);
  assert.deepEqual(m.toasts, [], 'nothing is told twice');
});

test('take back when the part cannot be read again keeps the row and says it was saved', async () => {
  const m = load({
    position: [12.97, 77.59],
    answers: [answer([resolvedReport(51, MARKED(2))]), { visibility: 'live', undos_left: 1 }, new Error('offline')]
  });
  m.start();
  await settle();
  const before = rowsOf(m, 'ended-resolved-list')[0];
  undoButton(before).click();
  await settle();
  assert.equal(rowsOf(m, 'ended-resolved-list')[0], before, 'the rows are left as they were');
  assert.equal(shown(m.by('ended-error')), false, 'a quiet read never shows the error state');
  const line = before.querySelectorAll('.vote-error')[0];
  assert.equal(line.hidden, false);
  assert.ok(line.textContent.length > 0);
  assert.equal(undoButton(before).disabled, false, 'the block is released');
});

// ---- Phase 3.4 plan 25 (code review part 2, CR-01): a link to a finished report
// with no kept position. The server answers a report link from any centre, so
// the one read goes out from the existing fallback centre and only the linked
// row is drawn. The rest of the part keeps its location state, no distance is
// written from a centre that is nobody's position (A94), and the hub count
// stays empty because one row is not a count of what is ended nearby.

const FALLBACK = { lat: '12.9716', lon: '77.5946' };

test('CR-01: a link to a finished report with no kept position reads once from the fallback centre and draws that row', async () => {
  const m = load({
    search: '?part=ended&report=9',
    answers: [answer([resolvedReport(1), expiredReport(2)], { focus: { status: 'ok', report: resolvedReport(9) } })]
  });
  m.start();
  assert.equal(m.fetches.length, 1, 'one read goes out');
  const sent = new URL(m.fetches[0].url, 'http://x');
  assert.equal(sent.searchParams.get('lat'), FALLBACK.lat);
  assert.equal(sent.searchParams.get('lon'), FALLBACK.lon);
  assert.equal(sent.searchParams.get('report'), '9');
  assert.equal(sent.searchParams.get('view'), 'past');
  assert.equal(m.fetches[0].init.method, undefined, 'a plain read');
  await settle();

  assert.deepEqual(rowsOf(m, 'ended-resolved-list').map((r) => r.dataset.reportId), ['9'],
    'only the linked row, never the reports around the fallback centre');
  assert.equal(rowsOf(m, 'ended-faded-list').length, 0);
  assert.ok(rowsOf(m, 'ended-resolved-list')[0].classList.contains('activity-card--chosen'));
  assert.equal(shown(m.by('ended-resolved')), true);
  assert.equal(shown(m.by('ended-location')), true, 'the rest of the part keeps its location state');
  assert.equal(shown(m.by('ended-empty')), false);
  assert.equal(shown(m.by('ended-error')), false);
  assert.equal(shown(m.by('ended-skeleton')), false);
  assert.equal(m.by('ended-hub-count').hidden, true, 'one linked row is not a count');
  assert.equal(m.writes.length, 0, 'the fallback centre is never saved as a position');
});

test('CR-01: the linked row shows no distance measured from the fallback centre', async () => {
  const m = load({
    search: '?part=ended&report=9',
    answers: [answer([], { focus: { status: 'ok', report: resolvedReport(9, { distance_km: 8.4 }) } })]
  });
  m.start();
  await settle();
  const row = rowsOf(m, 'ended-resolved-list')[0];
  assert.equal(cell(m, row, 'activity-card__distance').textContent, 'Unknown');

  const rescue = load({
    search: '?part=ended&report=5',
    answers: [answer([], { focus: { status: 'ok', report: expiredReport(5, { approximate: true, distance_km: 7.2 }) } })]
  });
  rescue.start();
  await settle();
  const rescueRow = rowsOf(rescue, 'ended-faded-list')[0];
  assert.equal(cell(rescue, rescueRow, 'activity-card__distance').textContent, 'Unknown', 'no "about N km" either');
});

test('CR-01: a link that cannot be shown, with no kept position, tells it once and keeps the location state alone', async () => {
  for (const status of ['too_old', 'unavailable']) {
    const m = load({
      search: '?part=ended&report=9',
      answers: [answer([resolvedReport(1)], { focus: { status, report: null } })]
    });
    m.start();
    await settle();
    assert.equal(m.toasts.length, 1, status);
    assert.equal(shown(m.by('ended-location')), true, status);
    assert.equal(shown(m.by('ended-resolved')), false, status);
    assert.equal(shown(m.by('ended-empty')), false, status);
  }
  const live = load({
    search: '?part=ended&report=9',
    answers: [answer([], { focus: { status: 'ok', report: { id: 9, history: null } } })]
  });
  live.start();
  await settle();
  assert.equal(shown(live.by('ended-location')), true, 'a report that is not finished draws nothing');
  assert.equal(shown(live.by('ended-empty')), false);
});

test('CR-01: a failed read of the link with no kept position shows the error state, and Try again asks the same way', async () => {
  const m = load({
    search: '?part=ended&report=9',
    answers: [new Error('down'), answer([], { focus: { status: 'ok', report: resolvedReport(9) } })]
  });
  m.start();
  await settle();
  assert.equal(shown(m.by('ended-error')), true);
  assert.equal(shown(m.by('ended-location')), false);
  m.by('ended-retry').click();
  await settle();
  assert.equal(m.fetches.length, 2);
  assert.equal(new URL(m.fetches[1].url, 'http://x').searchParams.get('report'), '9');
  assert.equal(rowsOf(m, 'ended-resolved-list').length, 1);
  assert.equal(shown(m.by('ended-error')), false);
});

test('CR-01: without a link and without a kept position nothing is asked, as before; the distance cells stay Unknown', async () => {
  const cells = [new FakeEl(null, 'p')];
  cells[0].setAttribute('data-lat', '12.9');
  cells[0].setAttribute('data-lon', '77.5');
  const m = load({ search: '?part=ended', cells });
  m.start();
  await settle();
  assert.equal(m.fetches.length, 0);
  assert.equal(shown(m.by('ended-location')), true);
  assert.equal(cells[0].textContent, 'Unknown');
});

test('CR-01: a take back on the linked row with no kept position reads the part again and the row leaves', async () => {
  const m = load({
    search: '?part=ended&report=9',
    answers: [
      answer([], { focus: { status: 'ok', report: resolvedReport(9, MARKED(2)) } }),
      { visibility: 'live', undos_left: 1 },
      answer([], { focus: { status: 'ok', report: { id: 9, view: 'active', history: null } } })
    ]
  });
  m.start();
  await settle();
  undoButton(rowsOf(m, 'ended-resolved-list')[0]).click();
  await settle();
  assert.equal(m.fetches.length, 3, 'the part is read again');
  assert.equal(new URL(m.fetches[2].url, 'http://x').searchParams.get('lat'), FALLBACK.lat);
  assert.equal(rowsOf(m, 'ended-resolved-list').length, 0, 'the report is open again, so the row leaves');
  assert.equal(shown(m.by('ended-location')), true);
  assert.equal(shown(m.by('ended-error')), false);
  assert.equal(shown(m.by('ended-skeleton')), false);
});

test('WR-01: a position another account kept on this browser is not used; the part asks for the location', async () => {
  const cells = [new FakeEl(null, 'p')];
  cells[0].setAttribute('data-lat', '12.9');
  cells[0].setAttribute('data-lon', '77.5');
  const m = load({ position: [12.97, 77.59], savedBy: '8', cells });
  m.start();
  await settle();
  assert.equal(m.fetches.length, 0, 'the earlier account\'s place is never sent');
  assert.equal(shown(m.by('ended-location')), true);
  assert.equal(cells[0].textContent, 'Unknown');

  // The same link with that stale position behaves as a link with no position.
  const linked = load({
    search: '?part=ended&report=9', position: [12.97, 77.59], savedBy: '8',
    answers: [answer([], { focus: { status: 'ok', report: resolvedReport(9) } })]
  });
  linked.start();
  assert.equal(new URL(linked.fetches[0].url, 'http://x').searchParams.get('lat'), FALLBACK.lat);
});

// ---- Phase 3.4 plan 25 (code review part 2, WR-06): Try again hides the
// button it was pressed on, so focus must be put on something that is on screen
// meanwhile (the title is not drawn on a phone part screen) and, once the read
// settles, on the result.

test('WR-06: Try again moves focus onto something shown while it reads, then onto the first row', async () => {
  const m = load({
    position: [12.97, 77.59],
    answers: [new Error('down'), answer([resolvedReport(7), resolvedReport(8)])]
  });
  m.start();
  await settle();
  m.by('ended-retry').focus();
  m.by('ended-retry').click();

  const active = m.doc.activeElement;
  assert.ok(active, 'focus is on an element');
  assert.notEqual(active, m.by('ended-retry'), 'not on the button that is now hidden');
  assert.equal(active.hidden, false, 'and the element is on screen while the read is out');
  assert.equal(active.getAttribute('tabindex'), '-1');

  await settle();
  const rows = rowsOf(m, 'ended-resolved-list');
  assert.equal(m.doc.activeElement, rows[0], 'once the part is drawn, focus lands on the first row');
});

test('WR-06: a second failure puts focus back on Try again, and an empty answer on the empty block', async () => {
  const m = load({
    position: [12.97, 77.59],
    answers: [new Error('down'), new Error('down again'), answer([])]
  });
  m.start();
  await settle();
  m.by('ended-retry').click();
  await settle();
  assert.equal(shown(m.by('ended-error')), true);
  assert.equal(m.doc.activeElement, m.by('ended-retry'));

  m.by('ended-retry').click();
  await settle();
  assert.equal(shown(m.by('ended-empty')), true);
  assert.equal(m.doc.activeElement, m.by('ended-empty'));
  assert.equal(m.by('ended-empty').getAttribute('tabindex'), '-1');
});

test('WR-06: focus the person moved elsewhere while the read was out is left alone', async () => {
  const m = load({ position: [12.97, 77.59], answers: [new Error('down'), answer([resolvedReport(7)])] });
  m.start();
  await settle();
  m.by('ended-retry').click();
  const elsewhere = new FakeEl(m.doc, 'a');
  elsewhere.focus();
  await settle();
  assert.equal(m.doc.activeElement, elsewhere);
});
