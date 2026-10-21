// low_page_test.mjs - runs the real web/static/js/app.js and
// web/static/js/low.js together in a Node sandbox and checks the Low data
// view's behaviour the Go source tests cannot:
//
//   - a failed place search on this page shows the page's own sentences,
//     picked by the answer's status, never the server's text (F-P4-12);
//   - a list request whose promise never settles, as WebKit can leave an
//     aborted one, still fails at the 10 second limit: the stale line shows,
//     the call's promise resolves and the next list request goes out (the
//     WebKit stale line finding). feed.js draws its stale line from the same
//     store state, but it needs the map and most of the main page, so it is
//     not loaded here.
//
// Run it with: node --test web/low_page_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the two scripts call: a minimal element model,
// fake timers the test moves by hand, a fetch the test answers by hand and a
// fake shared report form that keeps the location object low.js gives it.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');
const lowSource = fs.readFileSync(path.join(here, 'static', 'js', 'low.js'), 'utf8');

class FakeEl {
  constructor(doc, tag, id) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.className = '';
    this.disabled = false;
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  get hidden() { return Object.hasOwn(this.attrs, 'hidden'); }
  set hidden(v) { if (v) { this.attrs.hidden = ''; } else { delete this.attrs.hidden; } }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    this.children.push(node);
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
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  querySelector() { return null; }
  querySelectorAll() { return []; }
  focus() { this.doc.activeElement = this; }
}

// The ids low.js and app.js look up on the Low data page.
const PAGE_IDS = [
  'low-list', 'low-basis', 'low-reload', 'low-stale', 'low-stale-text',
  'low-skeleton', 'low-empty', 'low-empty-title', 'low-empty-text', 'low-error', 'low-full',
  'notice-live', 'report-modal', 'low-report', 'low-location-status'
];

// loadLowPage evaluates app.js and then low.js against a Low data page. fetch
// keeps every call for the test to answer; abortRejects says whether an
// abort ends the call's promise, as Chromium does, or leaves it unsettled, as
// WebKit sometimes does. nativeTimeout swaps AbortController for a native
// AbortSignal.timeout that never fires. stored is the remembered position and
// geolocation the fake position service, both absent unless given.
const POSITION_KEY = 'pinalert.lowLastPosition';

function loadLowPage({ abortRejects = true, nativeTimeout = false, stored = null, geolocation = null, account = '7', storedRaw = null, extraStorage = {}, permissions = null, throwingStorage = false } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    // Amended 2026-10-17 by Phase 3.4 plan 27 (skeptic S-2): the page carries
    // data-account-id, as low.html.tmpl does, and low.js reads it from there.
    querySelector(selector) {
      if (selector !== '[data-account-id]' || account === null) { return null; }
      return { getAttribute: (name) => (name === 'data-account-id' ? account : null) };
    },
    querySelectorAll() { return []; },
    addEventListener() {},
    removeEventListener() {}
  };
  doc.documentElement = new FakeEl(doc, 'html');
  PAGE_IDS.forEach((id) => { doc.byId[id] = new FakeEl(doc, id === 'low-list' ? 'ul' : 'div', id); });

  // Fake timers: the test moves the clock. Intervals repeat.
  let now = 0;
  let nextId = 1;
  const timers = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) { break; }
      const [id, timer] = due;
      now = timer.at;
      if (timer.every) { timer.at += timer.every; } else { timers.delete(id); }
      timer.fn();
    }
    now = end;
  };

  // Every list request is kept with its abort state for the test to answer.
  const requests = [];
  const fetch = (target) => new Promise((resolve, reject) => {
    const signal = target && target.signal;
    const req = { url: target && target.url, resolve, reject, aborted: false };
    if (signal && typeof signal.addEventListener === 'function') {
      signal.addEventListener('abort', () => {
        req.aborted = true;
        if (abortRejects) {
          const err = new Error('aborted');
          err.name = 'AbortError';
          reject(err);
        }
      });
    }
    requests.push(req);
  });
  const answer = (req, body) => req.resolve({
    ok: true,
    status: 200,
    headers: { get: () => null },
    json: () => Promise.resolve(body)
  });

  class FakeSignal {
    constructor() { this.aborted = false; this.listeners = []; }
    addEventListener(type, fn) { if (type === 'abort') { this.listeners.push(fn); } }
    fire() {
      if (this.aborted) { return; }
      this.aborted = true;
      this.listeners.forEach((fn) => fn());
    }
  }
  class AbortController {
    constructor() { this.signal = new FakeSignal(); }
    abort() { this.signal.fire(); }
  }

  // A small memory store that keeps every removal for the test to read.
  const data = new Map(Object.entries(extraStorage));
  if (storedRaw !== null) { data.set(POSITION_KEY, storedRaw); }
  if (stored) { data.set(POSITION_KEY, JSON.stringify(stored)); }
  const removed = [];
  const storage = {
    data,
    removed,
    getItem: (k) => (data.has(k) ? data.get(k) : null),
    setItem: (k, v) => { data.set(k, String(v)); },
    removeItem: (k) => { removed.push(k); data.delete(k); }
  };

  if (throwingStorage) {
    storage.getItem = () => { throw new Error('blocked'); };
    storage.setItem = () => { throw new Error('blocked'); };
  }
  const mounted = [];
  const window = {
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { at: now + (ms || 0), fn }); return id; },
    clearTimeout(id) { timers.delete(id); },
    setInterval(fn, ms) { const id = nextId++; timers.set(id, { at: now + ms, fn, every: ms }); return id; },
    clearInterval(id) { timers.delete(id); },
    addEventListener() {},
    localStorage: storage,
    PinalertReportForm: { mount(options) { mounted.push(options); }, open() {} }
  };
  const sandbox = {
    window,
    document: doc,
    navigator: Object.assign({}, geolocation ? { geolocation } : {}, permissions ? { permissions } : {}),
    fetch,
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    AbortController,
    Promise,
    Date,
    Math,
    JSON,
    Array,
    Object,
    String,
    Number,
    isFinite,
    isNaN,
    parseFloat,
    parseInt,
    encodeURIComponent
  };
  if (nativeTimeout) {
    // A browser with no AbortController but a native timeout signal, one
    // that never ends the request: only the page's own timer is left.
    delete sandbox.AbortController;
    sandbox.AbortSignal = { timeout: () => new FakeSignal() };
  }
  sandbox.PinalertReportForm = window.PinalertReportForm;
  vm.createContext(sandbox);
  vm.runInContext(appSource, sandbox);
  sandbox.Pinalert = window.Pinalert;
  vm.runInContext(lowSource, sandbox);
  return { Pinalert: window.Pinalert, el: (id) => doc.byId[id], requests, answer, advance, mounted, storage, retryClick: () => (doc.byId['low-reload'].handlers.click || []).forEach((fn) => fn({})) };
}

const settle = async () => {
  for (let i = 0; i < 10; i++) {
    await new Promise((resolve) => setImmediate(resolve));
  }
};

const STALE_LINE = 'Last updated just now. Trying again.';

// The WebKit stale line finding: an aborted request whose promise never
// settles must not keep the stale line, or anything else that waits on the
// list, from happening. Each case runs with both ways the page can set its
// time limit: its own controller, and a native timeout signal.
for (const nativeTimeout of [false, true]) {
  const how = nativeTimeout ? ' (native timeout signal)' : ' (controller)';

  test('a poll whose request never settles shows the stale line at 10 seconds' + how, async () => {
    const page = loadLowPage({ abortRejects: false, nativeTimeout });
    await settle();
    assert.equal(page.requests.length, 1, 'the first list request is out');
    page.answer(page.requests[0], { reports: [] });
    await settle();
    assert.equal(page.Pinalert.state.status, 'ok');
    assert.equal(page.el('low-stale').hidden, true);

    // The poll 30 seconds later gets no answer, ever, and its abort does not
    // end its promise.
    page.advance(30000);
    assert.equal(page.requests.length, 2, 'the poll went out');
    page.advance(9999);
    await settle();
    assert.equal(page.el('low-stale').hidden, true, 'nothing before the limit');
    page.advance(1);
    await settle();
    assert.equal(page.requests[1].aborted, !nativeTimeout, 'the page aborts its own controller at the limit');
    assert.equal(page.Pinalert.state.status, 'error', 'the poll failed at its limit');
    assert.equal(page.el('low-stale').hidden, false, 'the stale line shows');
    assert.equal(page.el('low-stale-text').textContent, STALE_LINE);
    assert.equal(page.el('notice-live').textContent, 'Connection lost. Showing reports from just now.');

    // The next poll answers, and the line goes.
    page.advance(20000);
    assert.equal(page.requests.length, 3);
    page.answer(page.requests[2], { reports: [] });
    await settle();
    assert.equal(page.Pinalert.state.status, 'ok');
    assert.equal(page.el('low-stale').hidden, true);
  });

  test('the promise of a list request that never settles resolves at the limit' + how, async () => {
    const page = loadLowPage({ abortRejects: false, nativeTimeout });
    await settle();
    page.answer(page.requests[0], { reports: [] });
    await settle();
    let done = false;
    page.Pinalert.fetchReports({ afterWrite: true }).then(() => { done = true; });
    page.advance(9999);
    await settle();
    assert.equal(done, false);
    page.advance(1);
    await settle();
    assert.equal(done, true, 'a refetch after a write is not left waiting for good');
    assert.equal(page.Pinalert.state.status, 'error');
  });

  test('a stored position list that never settles does not hold back the fresh one' + how, async () => {
    let found = null;
    const geolocation = { getCurrentPosition(onFound) { found = onFound; } };
    const page = loadLowPage({ abortRejects: false, nativeTimeout, stored: { lat: 17.385, lon: 78.4867 }, geolocation });
    await settle();
    assert.equal(page.requests.length, 1, 'the list for the stored position is out');
    found({ coords: { latitude: 17.4, longitude: 78.45 } });
    await settle();
    assert.equal(page.requests.length, 1, 'the fresh list waits for the stored one, in order');
    page.advance(10000);
    await settle();
    assert.equal(page.requests.length, 2, 'the fresh list goes out once the stored one hit its limit');
    assert.match(page.requests[1].url, /lat=17\.4&lon=78\.45/);
    page.answer(page.requests[1], { reports: [] });
    await settle();
    assert.equal(page.Pinalert.state.status, 'ok');
    assert.equal(page.el('low-basis').textContent, 'Near your position.');
  });
}

test('an answer that lands before the limit clears the limit' , async () => {
  const page = loadLowPage({ abortRejects: false });
  await settle();
  page.answer(page.requests[0], { reports: [] });
  await settle();
  page.advance(10000);
  await settle();
  assert.equal(page.requests[0].aborted, false, 'an answered request is never aborted');
  assert.equal(page.Pinalert.state.status, 'ok');
  assert.equal(page.el('low-stale').hidden, true);
});

test('a refused search on the Low data page shows the main form words, any other failure the Low sentence (F-P4-12)', () => {
  const page = loadLowPage();
  assert.equal(page.mounted.length, 1, 'low.js mounts the shared report form');
  const location = page.mounted[0].location;
  assert.equal(page.mounted[0].variant, 'low');

  const tooMany = 'Too many searches. Wait a few seconds and try again.';
  const unavailable = 'Search unavailable. Try your position again, or open the full version.';
  // The form passes '' as the server's text on this page, and the status.
  assert.equal(location.searchFailureMessage('', 429), tooMany);
  assert.equal(location.searchFailureMessage('', 503), unavailable);
  assert.equal(location.searchFailureMessage('', undefined), unavailable, 'a timeout or no answer');
  assert.equal(location.searchFailureMessage('', 500), unavailable);
  // The server's text is never shown, whatever it says.
  assert.equal(location.searchFailureMessage('Search unavailable, try tapping the map instead.', 503), unavailable);
  assert.equal(location.searchFailureMessage('Anything at all', 429), tooMany);
});

// Skeptic S-2 (Phase 3.4 plan 27): a kept position has an owner on this page
// too, the same rule position.js applies.
test('S-2: a fresh fix is stored with the account of the page', async () => {
  let found = null;
  const geolocation = { getCurrentPosition(onFound) { found = onFound; } };
  const page = loadLowPage({ geolocation });
  await settle();
  found({ coords: { latitude: 17.4, longitude: 78.45 } });
  await settle();
  const kept = JSON.parse(page.storage.data.get(POSITION_KEY));
  assert.equal(kept.lat, 17.4);
  assert.equal(kept.lon, 78.45);
  assert.equal(typeof kept.savedAt, 'number');
  assert.equal(kept.account, '7');
});

test('S-2: a position another account kept is not used and is removed', async () => {
  const geolocation = { getCurrentPosition() {} };
  const page = loadLowPage({ stored: { lat: 17.385, lon: 78.4867, savedAt: 5, account: '8' }, geolocation });
  await settle();
  assert.equal(page.storage.data.has(POSITION_KEY), false, 'the other account\'s place is gone');
  assert.deepEqual(page.storage.removed, [POSITION_KEY]);
  assert.equal(page.requests.length, 0, 'no list is asked for with the other account\'s place');
  assert.equal(page.el('low-basis').textContent, 'Finding your position.');
});

test('S-2: a position this account kept is used and stays', async () => {
  const page = loadLowPage({ stored: { lat: 17.385, lon: 78.4867, savedAt: 5, account: '7' } });
  await settle();
  assert.match(page.requests[0].url, /lat=17\.385&lon=78\.4867/);
  assert.deepEqual(page.storage.removed, []);
  assert.equal(JSON.parse(page.storage.data.get(POSITION_KEY)).account, '7');
});

test('S-2: a position with no owner is taken over by this account and used', async () => {
  for (const owner of [undefined, '']) {
    const stored = { lat: 17.385, lon: 78.4867, savedAt: 1760000000000 };
    if (owner !== undefined) { stored.account = owner; }
    const page = loadLowPage({ stored });
    await settle();
    assert.match(page.requests[0].url, /lat=17\.385&lon=78\.4867/, 'the ownerless place is used');
    const kept = JSON.parse(page.storage.data.get(POSITION_KEY));
    assert.equal(kept.account, '7', 'and now belongs to this account');
    assert.equal(kept.savedAt, 1760000000000, 'with its own time');
    assert.equal(kept.lat, 17.385);
  }
});

test('S-2: a page with no account id neither reads, removes nor writes a position', async () => {
  let found = null;
  const geolocation = { getCurrentPosition(onFound) { found = onFound; } };
  const raw = JSON.stringify({ lat: 17.385, lon: 78.4867, savedAt: 5, account: '8' });
  const page = loadLowPage({ account: null, storedRaw: raw, geolocation });
  await settle();
  assert.equal(page.requests.length, 0, 'the earlier value is not used, so no list waits on it');
  found({ coords: { latitude: 17.4, longitude: 78.45 } });
  await settle();
  assert.match(page.requests[0].url, /lat=17\.4&lon=78\.45/);
  assert.equal(page.storage.data.get(POSITION_KEY), raw, 'the earlier value is untouched and nothing new is written');
  assert.deepEqual(page.storage.removed, []);
});

test('S-2: only the position key is ever removed; the queue and the theme choice stay', async () => {
  const extraStorage = { 'pinalert.theme': 'dark', 'pinalert.queue.7': '[{"x":1}]' };
  const page = loadLowPage({ stored: { lat: 17.385, lon: 78.4867, savedAt: 5, account: '8' }, extraStorage });
  await settle();
  assert.deepEqual(page.storage.removed, [POSITION_KEY]);
  assert.equal(page.storage.data.get('pinalert.theme'), 'dark');
  assert.equal(page.storage.data.get('pinalert.queue.7'), '[{"x":1}]');
});

// ---- Plan 36: the browser's location question is asked once per account ------

const ASKED_KEY = 'pinalert.locationAsked.7';
const permissionsOf = (state) => ({ query: () => Promise.resolve({ state }) });

function countingGeolocation() {
  const calls = [];
  return { calls, getCurrentPosition(ok, fail, opts) { calls.push({ ok, fail, opts }); } };
}

test('plan 36: the first load asks once and sets the marker for the account', async () => {
  const geo = countingGeolocation();
  const page = loadLowPage({ geolocation: geo, permissions: permissionsOf('prompt') });
  await settle();
  assert.equal(geo.calls.length, 1, 'the first load asks');
  assert.equal(page.storage.data.get(ASKED_KEY), '1');
});

test('plan 36: a later load with the marker and a prompt state does not ask and stays on the kept position', async () => {
  const geo = countingGeolocation();
  const page = loadLowPage({
    geolocation: geo,
    permissions: permissionsOf('prompt'),
    stored: { lat: 17.385, lon: 78.4867, account: '7' },
    extraStorage: { [ASKED_KEY]: '1' }
  });
  await settle();
  assert.equal(geo.calls.length, 0, 'the browser is not asked again');
  assert.equal(page.requests.length, 1, 'the list for the kept position is out');
  assert.match(page.requests[0].url, /lat=17\.385&lon=78\.4867/);
  assert.match(page.el('low-basis').textContent, /^Near your last known position\./);
  assert.equal(page.el('low-reload').hidden, false, 'Reload stays the way to refresh (Amended 2026-10-21 by plan 44 (owner): the Retry position button is the Reload control now)');
});

test('plan 36: with the marker, no kept position and the API missing, the list starts at the default area and does not ask', async () => {
  const geo = countingGeolocation();
  const page = loadLowPage({ geolocation: geo, extraStorage: { [ASKED_KEY]: '1' } });
  await settle();
  assert.equal(geo.calls.length, 0);
  assert.equal(page.requests.length, 1);
  assert.equal(page.el('low-reload').hidden, false);
});

test('plan 36: a granted browser is read silently even with the marker', async () => {
  const geo = countingGeolocation();
  loadLowPage({ geolocation: geo, permissions: permissionsOf('granted'), extraStorage: { [ASKED_KEY]: '1' } });
  await settle();
  assert.equal(geo.calls.length, 1);
});

test('plan 36: Reload on a default or kept position always asks', async () => {
  const geo = countingGeolocation();
  const page = loadLowPage({ geolocation: geo, permissions: permissionsOf('prompt'), extraStorage: { [ASKED_KEY]: '1' } });
  await settle();
  assert.equal(geo.calls.length, 0);
  page.retryClick();
  assert.equal(geo.calls.length, 1, 'the person tapped, so it asks');
});

// Amended 2026-10-21 by plan 44 (owner): the Retry now and Retry buttons are
// gone; Reload asks the store for a fresh list when the position needs no retry.
test('plan 44: Reload refreshes the list when the position needs no retry', async () => {
  const geo = countingGeolocation();
  const page = loadLowPage({ geolocation: geo, permissions: permissionsOf('granted'), stored: { lat: 17.385, lon: 78.4867, account: '7' } });
  await settle();
  const first = page.requests.length;
  page.answer(page.requests[first - 1], { reports: [] });
  await settle();
  const before = page.requests.length;
  page.retryClick();
  await settle();
  assert.ok(page.requests.length > before, 'Reload sent a fresh list request');
});

test('plan 36: another account is asked for itself', async () => {
  const geo = countingGeolocation();
  loadLowPage({ geolocation: geo, account: '8', permissions: permissionsOf('prompt'), extraStorage: { [ASKED_KEY]: '1' } });
  await settle();
  assert.equal(geo.calls.length, 1, 'the marker of account 7 does not count for account 8');
});

test('plan 36: a storage that throws keeps today behaviour (asks on every load)', async () => {
  const geo = countingGeolocation();
  loadLowPage({ geolocation: geo, permissions: permissionsOf('prompt'), throwingStorage: true });
  await settle();
  assert.equal(geo.calls.length, 1);
});
