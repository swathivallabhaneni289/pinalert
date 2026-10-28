// demo_practice_test.mjs - runs the real web/static/js/demo.js (and app.js for the
// event and the ended visit) in a Node sandbox and checks the Demo's practice
// flow (plan 05-17, D-01, D-05, UI-SPEC 4d, T-05-92, T-05-94):
//
//   - a practice post shows the toast, opens the new report on its trust block
//     and starts a poll every 3 seconds;
//   - the page follows the SERVER's words. The trust sentences in these cases
//     are fed in as server data in the shape app.js holds in state; the page
//     under test never holds them;
//   - while the report is below Confirmed the poll continues and nothing is
//     announced; when the server's state says confirmed, the polite status gets
//     "Practice report confirmed. " plus the server's sentence once, and the poll
//     stops;
//   - a second post starts a new watch; a real page does none of this; and
//   - an ended visit (a 404 naming the field visit) sends the page back to /demo.
//
// Run it with: node --test web/demo_practice_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const read = (name) => fs.readFileSync(path.join(here, 'static', 'js', name), 'utf8');

const settle = async () => { for (let i = 0; i < 12; i += 1) { await Promise.resolve(); } };

// What the server says, as server data. The page under test must repeat these
// from the state it is handed and never from its own text.
const SERVER_LEAD_0 = 'server lead zero';
const SERVER_LEAD_1 = 'server lead one';
const SERVER_CONFIRMED = 'server confirmed sentence for the people at different spots';

function report(id, { confirmed = false, lead = SERVER_LEAD_0, sentence = 'server not yet' } = {}) {
  return {
    id,
    confirmed_by_people: confirmed,
    trust: { lead_sentence: lead, confirmed_sentence: sentence }
  };
}

// load builds a Demo page around demo.js with a fake Pinalert (the feed state,
// subscribe, fetchReports, setText, showToast), a fake map and fake timers.
function load({ demoPage = true, search = '' } = {}) {
  const calls = { toast: [], open: [], fetch: 0, intervals: [], cleared: [], replaced: [] };
  const listeners = [];
  const state = { reports: [] };
  const subscribers = [];
  const nodes = { 'feed-status': { textContent: '' } };
  const timeouts = [];
  const events = {};
  const pinalert = {
    state,
    subscribe(fn) {
      subscribers.push(fn);
      return () => { const i = subscribers.indexOf(fn); if (i !== -1) { subscribers.splice(i, 1); } };
    },
    fetchReports() { calls.fetch += 1; return Promise.resolve(); },
    setText(el, text) { el.textContent = text; },
    showToast(text) { calls.toast.push(text); },
    openReport(id) { calls.open.push(['Pinalert', id]); }
  };
  const win = {
    Pinalert: pinalert,
    PinalertMap: { openReport(id) { calls.open.push(['PinalertMap', id]); return true; } },
    PinalertDemoPage: demoPage ? true : undefined,
    setInterval(fn, ms) { calls.intervals.push({ fn, ms, id: calls.intervals.length + 1 }); return calls.intervals.length; },
    clearInterval(id) { calls.cleared.push(id); },
    setTimeout(fn) { timeouts.push(fn); return timeouts.length; },
    clearTimeout() {},
    requestAnimationFrame() { return 1; },
    addEventListener() {},
    location: { pathname: '/demo', search, replace(target) { calls.replaced.push(target); } }
  };
  const doc = {
    readyState: 'complete',
    documentElement: { style: { setProperty() {} } },
    getElementById(id) { return nodes[id] || null; },
    querySelector() { return null; },
    addEventListener(type, fn) { listeners.push({ type, fn }); }
  };
  const sandbox = {
    window: win, document: doc, ResizeObserver: undefined,
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } }
  };
  vm.createContext(sandbox);
  vm.runInContext(read('demo.js'), sandbox, { filename: 'demo.js' });
  return {
    win, calls, state, nodes, timeouts,
    notify() { subscribers.slice().forEach((fn) => fn(state)); },
    flush() { while (timeouts.length) { timeouts.shift()(); } },
    post(rep) {
      listeners.filter((l) => l.type === 'pinalert:report-posted').forEach((l) => l.fn({ type: 'pinalert:report-posted', detail: { report: rep } }));
    },
    tick(n = 1) { const poll = calls.intervals[calls.intervals.length - 1]; for (let i = 0; i < n; i += 1) { poll.fn(); } },
    subscribers
  };
}

test('demo.js: a practice post shows the toast, opens the report and polls every 3 seconds', () => {
  const m = load();
  m.post(report(41));
  // The toast and the open wait one tick, so the post window has closed by then.
  assert.equal(m.calls.open.length, 0);
  assert.equal(m.calls.toast.length, 0);
  m.flush();
  assert.deepEqual(m.calls.toast, ['Practice report posted. Pretend neighbours will confirm it in a few seconds.']);
  assert.deepEqual(m.calls.open, [['PinalertMap', 41]]);
  assert.equal(m.calls.intervals.length, 1);
  assert.equal(m.calls.intervals[0].ms, 3000);
  assert.equal(m.subscribers.length, 1, 'it follows the feed state');
});

test('demo.js: a report the map cannot open is handed to the app', () => {
  const m = load();
  m.win.PinalertMap.openReport = (id) => { m.calls.open.push(['PinalertMap', id]); return false; };
  m.post(report(41));
  m.flush();
  assert.deepEqual(m.calls.open, [['PinalertMap', 41], ['Pinalert', 41]]);
});

test('demo.js: while the report is below Confirmed the poll continues and nothing is announced', () => {
  const m = load();
  m.post(report(41));
  m.state.reports = [report(41, { lead: SERVER_LEAD_0 })];
  m.notify();
  m.tick(2);
  assert.equal(m.calls.fetch, 2, 'one reload per tick');
  m.state.reports = [report(41, { lead: SERVER_LEAD_1 })];
  m.notify();
  m.tick();
  assert.equal(m.calls.fetch, 3);
  assert.equal(m.nodes['feed-status'].textContent, '', 'the climb is silent step by step');
  assert.equal(m.calls.cleared.length, 0, 'the poll is still running');
});

test('demo.js: at Confirmed the server sentence is announced once and the poll stops', () => {
  const m = load();
  m.post(report(41));
  m.state.reports = [report(41, { confirmed: true, sentence: SERVER_CONFIRMED })];
  m.notify();
  assert.equal(m.nodes['feed-status'].textContent, 'Practice report confirmed. ' + SERVER_CONFIRMED);
  assert.equal(m.calls.cleared.length, 1, 'the poll stopped');
  assert.equal(m.subscribers.length, 0, 'it stopped following the state');
  // Further state changes announce nothing more.
  m.nodes['feed-status'].textContent = '';
  m.notify();
  m.state.reports = [report(41, { confirmed: true, sentence: SERVER_CONFIRMED })];
  m.notify();
  assert.equal(m.nodes['feed-status'].textContent, '', 'announced once');
  const before = m.calls.fetch;
  m.calls.intervals[0].fn();
  assert.equal(m.calls.fetch, before, 'a late tick reloads nothing');
});

test('demo.js: a second practice post starts a new watch and ends the old one', () => {
  const m = load();
  m.post(report(41));
  m.post(report(42));
  assert.equal(m.calls.intervals.length, 2);
  assert.deepEqual(m.calls.cleared, [1], 'the first poll was stopped');
  assert.equal(m.subscribers.length, 1);
  // Report 41 confirmed now says nothing: only the report being watched counts.
  m.state.reports = [report(41, { confirmed: true, sentence: SERVER_CONFIRMED }), report(42)];
  m.notify();
  assert.equal(m.nodes['feed-status'].textContent, '');
  m.state.reports = [report(42, { confirmed: true, sentence: SERVER_CONFIRMED })];
  m.notify();
  assert.equal(m.nodes['feed-status'].textContent, 'Practice report confirmed. ' + SERVER_CONFIRMED);
});

test('demo.js: a poll that never sees the report gives up after a while', () => {
  const m = load();
  m.post(report(41));
  m.tick(200);
  assert.ok(m.calls.fetch < 200, 'the poll is bounded');
  assert.equal(m.calls.cleared.length, 1);
});

test('demo.js: on a real page a posted report does nothing', () => {
  const m = load({ demoPage: false });
  m.post(report(41));
  m.flush();
  assert.deepEqual(m.calls.toast, []);
  assert.deepEqual(m.calls.open, []);
  assert.equal(m.calls.intervals.length, 0);
  assert.equal(m.subscribers.length, 0);
});

test('demo.js: an ended visit sends the page back to the Demo, keeping from=welcome', () => {
  const plain = load({ search: '' });
  plain.win.PinalertDemoEnded();
  assert.deepEqual(plain.calls.replaced, ['/demo']);

  const welcome = load({ search: '?from=welcome' });
  welcome.win.PinalertDemoEnded();
  assert.deepEqual(welcome.calls.replaced, ['/demo?from=welcome']);

  const other = load({ search: '?from=https://evil.example' });
  other.win.PinalertDemoEnded();
  assert.deepEqual(other.calls.replaced, ['/demo'], 'only the one allowed word is carried');
});

// The app side: the event after a post and the ended visit.

function loadApp({ shell = {} } = {}) {
  const fetches = [];
  const events = [];
  const ended = [];
  const el = (attrs) => ({
    attrs,
    getAttribute(n) { return Object.hasOwn(attrs, n) ? attrs[n] : null; },
    hasAttribute(n) { return Object.hasOwn(attrs, n); },
    setAttribute(n, v) { attrs[n] = String(v); },
    style: { setProperty() {} }
  });
  const shellEl = el(shell);
  const doc = {
    readyState: 'complete',
    documentElement: el({}),
    getElementById(id) { return id === 'app-shell' ? shellEl : null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    createElement() { return el({}); },
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent(ev) { events.push(ev); return true; }
  };
  const win = {
    setInterval() { return 1; }, setTimeout() { return 1; }, clearTimeout() {}, clearInterval() {},
    requestAnimationFrame() { return 1; }, addEventListener() {}, removeEventListener() {},
    location: { href: 'http://127.0.0.1/demo', pathname: '/demo', search: '' },
    history: { state: null, replaceState() {} },
    matchMedia() { return { matches: false, addEventListener() {}, addListener() {} }; },
    getComputedStyle() { return { paddingTop: '0', paddingBottom: '0' }; },
    PinalertDemoEnded() { ended.push(true); }
  };
  const sandbox = {
    window: win, document: doc, navigator: {},
    sessionStorage: { getItem() { return null; }, setItem() {}, removeItem() {} },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    AbortController: class { constructor() { this.signal = { aborted: false, addEventListener() {} }; } abort() {} },
    ResizeObserver: undefined, URL, URLSearchParams, Date, Promise, Math, JSON, isFinite, parseFloat, encodeURIComponent
  };
  sandbox.fetch = (target, init) => new Promise((resolve) => {
    fetches.push({ url: typeof target === 'string' ? target : target.url, init: init || {}, resolve });
  });
  vm.createContext(sandbox);
  ['api-base.js', 'app.js'].forEach((name) => vm.runInContext(read(name), sandbox, { filename: name }));
  const answer = (req, body, status = 200) => req.resolve({
    ok: status >= 200 && status < 300, status, headers: { get: () => null }, json: () => Promise.resolve(body)
  });
  return { win, fetches, events, ended, answer };
}

const DEMO_SHELL = { 'data-api-base': '/demo/api/v/abcDEF123_-xyz', 'data-demo': '1' };

test('app.js: a created report raises pinalert:report-posted once the refetch is in', async () => {
  const m = loadApp({ shell: DEMO_SHELL });
  const done = m.win.Pinalert.submitReport({ description: 'water' });
  m.answer(m.fetches[0], { report: { id: 41 } }, 201);
  await settle();
  assert.equal(m.fetches.length, 2, 'the refetch after the write');
  assert.equal(m.events.filter((e) => e.type === 'pinalert:report-posted').length, 0, 'not before the refetch is in');
  m.answer(m.fetches[1], { reports: [] });
  const created = await done;
  assert.equal(created.id, 41);
  const posted = m.events.filter((e) => e.type === 'pinalert:report-posted');
  assert.equal(posted.length, 1);
  assert.equal(posted[0].detail.report.id, 41);
});

test('app.js: a refused post raises no posted event', async () => {
  const m = loadApp({ shell: DEMO_SHELL });
  const done = m.win.Pinalert.submitReport({ description: '' }).catch((err) => err);
  m.answer(m.fetches[0], { error: { field: 'description', message: 'Say what is happening.' } }, 400);
  await done;
  assert.equal(m.events.filter((e) => e.type === 'pinalert:report-posted').length, 0);
  assert.equal(m.ended.length, 0);
});

test('app.js: a 404 naming the field visit on the Demo calls PinalertDemoEnded', async () => {
  const m = loadApp({ shell: DEMO_SHELL });
  const done = m.win.Pinalert.submitReport({ description: 'water' }).catch((err) => err);
  m.answer(m.fetches[0], { error: { field: 'visit', message: 'This Demo has ended. Reload the page to start again.' } }, 404);
  await done;
  assert.equal(m.ended.length, 1);
});

test('app.js: a feed read that answers 404 for the visit calls PinalertDemoEnded', async () => {
  const m = loadApp({ shell: DEMO_SHELL });
  m.win.Pinalert.setCenter(16.5424, 80.6386, true);
  m.win.Pinalert.fetchReports();
  m.answer(m.fetches[0], { error: { field: 'visit', message: 'This Demo has ended. Reload the page to start again.' } }, 404);
  await settle();
  assert.equal(m.ended.length, 1);
});

test('app.js: on the real page a 404 never calls PinalertDemoEnded', async () => {
  const m = loadApp({ shell: {} });
  const done = m.win.Pinalert.submitReport({ description: 'water' }).catch((err) => err);
  m.answer(m.fetches[0], { error: { field: 'visit', message: 'x' } }, 404);
  await done;
  assert.equal(m.ended.length, 0);
});
