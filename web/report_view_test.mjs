// report_view_test.mjs - checks the address of an opened report and the phone
// report view's open and close rules (Phase 3.4, plan 39).
//
// map.js builds a whole Leaflet map on load and keeps its steps private, so this
// test cuts the report view section out of the real file text (it starts and
// ends with the plan 39 section comments) and runs it with a fake map, a fake
// shared store, a fake history and a fake document. Opening and closing a
// report are simulated the way Leaflet drives map.js: a close of the open
// report, then the open of the next one, in the same turn. A rename in map.js
// fails here loudly instead of passing on stale code.
//
// Run it with: node --test web/report_view_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const mapSource = fs.readFileSync(path.join(here, 'static', 'js', 'map.js'), 'utf8');
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');
const feedSource = fs.readFileSync(path.join(here, 'static', 'js', 'feed.js'), 'utf8');

function between(source, from, to) {
  const start = source.indexOf(from);
  const end = source.indexOf(to, start);
  assert.ok(start >= 0 && end > start, 'the source no longer has the text between "' + from + '" and "' + to + '"');
  return source.slice(start, end);
}

const section = between(mapSource, '  // ---- The report view and its address (plan 39)', '  // ---- End of the report view (plan 39)');
// The store's half of the public way in, Pinalert.openReport, cut from app.js.
const appOpenReport = between(appSource, '  function openReport(id)', '\n  }\n') + '\n  }\n';

// fakeHistory keeps a list of entries and an index. back() does not move at
// once: the browser answers with popstate from its own task, so the test
// delivers it with deliverPop().
function fakeHistory(start) {
  const entries = [{ state: null, url: start }];
  let index = 0;
  let pendingPop = false;
  const calls = [];
  return {
    calls,
    get url() { return entries[index].url; },
    get length() { return entries.length; },
    get index() { return index; },
    api: {
      get state() { return entries[index].state; },
      pushState(state, title, address) {
        calls.push('push ' + address);
        entries.splice(index + 1);
        entries.push({ state, url: address });
        index += 1;
      },
      replaceState(state, title, address) {
        calls.push('replace ' + address);
        entries[index] = { state, url: address };
      },
      back() {
        calls.push('back');
        if (index > 0) { index -= 1; pendingPop = true; }
      },
      forward() {
        if (index < entries.length - 1) { index += 1; pendingPop = true; }
      }
    },
    takePop() { const had = pendingPop; pendingPop = false; return had; },
    // userBack is the browser's own back button, which the page did not call.
    userBack() { if (index > 0) { index -= 1; pendingPop = true; } }
  };
}

function el(id) {
  return {
    id,
    hidden: true,
    focused: 0,
    attrs: {},
    textContent: '',
    focus() { this.focused += 1; env.active = this; },
    setAttribute(k, v) { this.attrs[k] = String(v); },
    removeAttribute(k) { delete this.attrs[k]; },
    hasAttribute(k) { return Object.prototype.hasOwnProperty.call(this.attrs, k); },
    contains(node) { return node === this; }
  };
}

let env = null;

// run builds the sandbox. phone picks the phone layout, start is the first
// address, ids the reports that have a pin, hasLoaded whether the first list
// answer has landed.
function run({ phone = true, start = '/', ids = ['5', '6', '7'], hasLoaded = true } = {}) {
  const hist = fakeHistory(start);
  const timers = [];
  const log = [];
  const root = el('html');
  const gone = el('report-gone');
  const goneTitle = el('report-gone-title');
  const mapBox = el('map');
  let view = { lat: 12, lng: 77, zoom: 13 };
  const size = { x: 390, y: 340 };
  env = { active: null };
  const markers = {};
  ids.forEach((id) => { markers[id] = { id }; });

  const map = {
    getCenter() { return { lat: view.lat, lng: view.lng }; },
    getZoom() { return view.zoom; },
    // A flat projection, 2 to the zoom pixels per degree, enough to check that
    // the top left corner comes back. The map box is size, which a test may change.
    project(ll, z) { return { x: ll.lng * 2 ** z, y: -ll.lat * 2 ** z }; },
    unproject(p, z) { const x = Array.isArray(p) ? p[0] : p.x; const y = Array.isArray(p) ? p[1] : p.y; return { lat: -y / 2 ** z, lng: x / 2 ** z }; },
    getSize() { return { x: size.x, y: size.y }; },
    containerPointToLatLng(p) {
      const c = map.project({ lat: view.lat, lng: view.lng }, view.zoom);
      return map.unproject([c.x - size.x / 2 + p[0], c.y - size.y / 2 + p[1]], view.zoom);
    },
    setView(c, z) { log.push('setView ' + (Array.isArray(c) ? c.join(',') : c.lat + ',' + c.lng) + ' z' + z); view = { lat: Array.isArray(c) ? c[0] : c.lat, lng: Array.isArray(c) ? c[1] : c.lng, zoom: z }; },
    closePopup() { log.push('closePopup'); closeOpen(); },
    getContainer() { return mapBox; }
  };

  const store = {
    state: { hasLoaded, focus: null, focusId: null },
    select(id, source) { log.push('select ' + id + ' ' + source); },
    setFocusId(id) { log.push('setFocusId ' + id); store.state.focusId = id; },
    fetchReports() { log.push('fetchReports'); },
    setText(node, text) { node.textContent = text; }
  };

  const windowObj = {
    get location() {
      const u = new URL(hist.url, 'http://x');
      return { pathname: u.pathname, search: u.search, hash: u.hash };
    },
    history: hist.api,
    setTimeout(fn) { timers.push(fn); return timers.length; },
    clearTimeout(n) { if (n > 0) { timers[n - 1] = null; } }
  };

  const sandbox = {
    window: windowObj,
    URLSearchParams,
    document: {
      documentElement: root,
      get activeElement() { return env.active; },
      getElementById(id) { return id === 'report-gone' ? gone : id === 'report-gone-title' ? goneTitle : null; }
    },
    Pinalert: store,
    env: { map, markers, phone, log },
    api: null
  };

  const wrapped =
    'var map = env.map; var markers = env.markers; var activePopup = null; var openedFocusId = null;\n' +
    'function reportIdForPopup(p) { return p ? p.id : null; }\n' +
    'function phoneDetail() { return env.phone; }\n' +
    'function flyTo(id) { env.log.push("flyTo " + id); env.open(String(id)); return true; }\n' +
    section +
    '\napi = { openReport: openReport, noteReportOpened: noteReportOpened, noteReportClosed: noteReportClosed,\n' +
    '  onReportPopState: onReportPopState, noteFocusAnswer: noteFocusAnswer, closeReportGone: closeReportGone,\n' +
    '  hideReportGone: hideReportGone,\n' +
    '  setActive: function (p) { activePopup = p; }, getActive: function () { return activePopup; },\n' +
    '  setOpenedFocusId: function (v) { openedFocusId = v; }, getOpenedFocusId: function () { return openedFocusId; } };\n' +
    // The real app.js openReport, with the store's own setFocusId and fetchReports.
    '(function () { var state = Pinalert.state;\n' +
    '  function setFocusId(id) { Pinalert.setFocusId(id); }\n' +
    '  function fetchReports() { Pinalert.fetchReports(); }\n' +
    appOpenReport +
    '  window.PinalertMap = { openReport: api.openReport };\n' +
    '  Pinalert.openReport = openReport; }());\n';

  // Leaflet's order: opening a report while another is open closes that one
  // first, then the new one opens, all in one turn.
  function closeOpen() {
    if (sandbox.api.getActive()) {
      sandbox.api.setActive(null);
      sandbox.api.noteReportClosed();
    }
  }
  sandbox.env.open = function (id) {
    closeOpen();
    sandbox.api.setActive({ id });
    sandbox.api.noteReportOpened(id);
  };

  vm.createContext(sandbox);
  vm.runInContext(wrapped, sandbox);
  const api = sandbox.api;

  return {
    api,
    hist,
    log,
    root,
    gone,
    goneTitle,
    store,
    view: () => view,
    setView(v) { view = v; },
    setSize(x, y) { size.x = x; size.y = y; },
    corner() { return map.containerPointToLatLng([0, 0]); },
    tap(id) { sandbox.env.open(String(id)); },
    close() { closeOpen(); },
    // flush runs the timers a close started, as the next turn would.
    flush() {
      while (timers.some((t) => t)) {
        const i = timers.findIndex((t) => t);
        const fn = timers[i];
        timers[i] = null;
        fn();
      }
    },
    // browserBack is the browser's back button: the entry moves and popstate fires.
    browserBack() { hist.userBack(); hist.takePop(); api.onReportPopState(); },
    deliverPop() { if (hist.takePop()) { api.onReportPopState(); } }
  };
}

test('opening a report from Nearby adds one address entry with the report id', () => {
  const t = run();
  t.tap(5);
  assert.equal(t.hist.url, '/?report=5');
  assert.equal(t.hist.length, 2);
  assert.ok(t.hist.api.state && t.hist.api.state.pinalertReport === '5');
});

test('another pin while a report is open switches the report in the same entry', () => {
  const t = run();
  t.tap(5);
  t.tap(6);
  t.flush();
  assert.equal(t.hist.url, '/?report=6');
  assert.equal(t.hist.length, 2, 'a switch replaces the entry, it never adds one');
  assert.ok(!t.hist.calls.includes('back'));
});

test('closing a report this script opened goes back one entry, to Nearby', () => {
  const t = run();
  t.tap(5);
  t.close();
  t.flush();
  assert.ok(t.hist.calls.includes('back'));
  t.deliverPop();
  assert.equal(t.hist.url, '/');
  assert.equal(t.api.getActive(), null);
  assert.equal(t.log.filter((l) => l === 'closePopup').length, 0, 'the popstate of that back closes nothing more');
});

test('the browser back button closes the open report and moves no further', () => {
  const t = run();
  t.tap(5);
  t.browserBack();
  assert.equal(t.api.getActive(), null);
  t.flush();
  assert.ok(!t.hist.calls.includes('back'), 'the close made by the browser does not go back a second time');
  assert.equal(t.hist.url, '/');
});

test('forward after back opens the same report again without a new entry', () => {
  const t = run();
  t.tap(5);
  t.browserBack();
  t.flush();
  t.hist.api.forward();
  t.hist.takePop();
  t.api.onReportPopState();
  assert.equal(t.api.getActive().id, '5');
  assert.equal(t.hist.length, 2);
  assert.equal(t.hist.url, '/?report=5');
});

test('a page that arrived with ?report=ID adds no entry, and closing strips the id', () => {
  const t = run({ start: '/?report=7&post=1' });
  t.tap(7);
  assert.equal(t.hist.length, 1);
  t.close();
  t.flush();
  assert.ok(!t.hist.calls.includes('back'), 'there is no entry of ours to go back to');
  assert.equal(t.hist.url, '/?post=1');
});

test('Pinalert.openReport opens a report that has a pin, and refuses an id that is not a number', () => {
  const t = run();
  assert.equal(t.store.openReport('6'), true);
  assert.equal(t.api.getActive().id, '6');
  assert.ok(t.log.includes('flyTo 6'));
  assert.ok(t.log.includes('select 6 map'));
  assert.ok(t.log.indexOf('flyTo 6') < t.log.indexOf('select 6 map'), 'the report opens before the selection, so the phone sheet is not raised under it');
  assert.ok(!t.log.some((l) => l.indexOf('setFocusId') === 0), 'a report with a pin needs no report link');
  for (const bad of ['abc', '0', null, undefined, '12a', '-3']) {
    assert.equal(t.store.openReport(bad), false);
  }
  assert.equal(t.api.openReport('abc'), false);
});

test('Pinalert.openReport on the report already open changes nothing', () => {
  const t = run();
  t.store.openReport('6');
  const before = t.log.length;
  assert.equal(t.store.openReport('6'), true);
  assert.equal(t.log.length, before);
  assert.equal(t.hist.length, 2);
});

test('Pinalert.openReport asks the server for a report without a pin, as an address on load does', () => {
  const early = run({ hasLoaded: false, ids: [] });
  assert.equal(early.store.openReport('42'), true);
  assert.deepEqual(early.log, ['setFocusId 42'], 'before the first answer the first list request carries it');
  const later = run({ ids: [] });
  assert.equal(later.api.openReport('42'), false, 'the map opens only a report it has a pin for');
  assert.deepEqual(later.log, [], 'the map never writes the link or reads the list itself');
  later.store.openReport('42');
  assert.deepEqual(later.log, ['setFocusId 42', 'fetchReports']);
});

test('a report the server cannot show puts the not available line on the phone, once per answer', () => {
  const t = run();
  const focus = { status: 'unavailable' };
  t.api.noteFocusAnswer({ focus });
  assert.equal(t.gone.hidden, false);
  assert.equal(t.goneTitle.focused, 1, 'focus moves to the heading of the view');
  assert.equal(t.root.attrs['data-report-gone'], 'true');
  t.api.closeReportGone();
  assert.equal(t.gone.hidden, true);
  assert.ok(!('data-report-gone' in t.root.attrs));
  t.api.noteFocusAnswer({ focus });
  assert.equal(t.gone.hidden, true, 'the same answer never shows the line twice');
  t.api.noteFocusAnswer({ focus: { status: 'too_old' } });
  assert.equal(t.gone.hidden, false, 'a new answer does');
});

test('the laptop keeps its toast and shows no not available view', () => {
  const t = run({ phone: false });
  t.api.noteFocusAnswer({ focus: { status: 'unavailable' } });
  assert.equal(t.gone.hidden, true);
  t.api.noteFocusAnswer({ focus: { status: 'ok', report: { id: 5 } } });
  assert.equal(t.gone.hidden, true);
});

test('Back on the not available line strips the id from an address the page arrived with', () => {
  const t = run({ start: '/?report=99', ids: [] });
  t.api.noteFocusAnswer({ focus: { status: 'unavailable' } });
  t.api.closeReportGone();
  assert.equal(t.hist.url, '/');
});

test('the browser back hides the not available line', () => {
  const t = run({ ids: [] });
  t.api.noteFocusAnswer({ focus: { status: 'unavailable' } });
  t.api.onReportPopState();
  assert.equal(t.gone.hidden, true);
});

test('the phone map goes back to where it was before the first report opened', () => {
  const t = run();
  t.setView({ lat: 1, lng: 2, zoom: 11 });
  t.tap(5);
  t.setView({ lat: 3, lng: 4, zoom: 16 });
  t.tap(6);
  t.setView({ lat: 5, lng: 6, zoom: 16 });
  t.close();
  t.flush();
  assert.deepEqual(t.view(), { lat: 1, lng: 2, zoom: 11 });
});

test('the phone map puts back its top left corner even when the sheet resized it meanwhile', () => {
  const t = run();
  t.setView({ lat: 1, lng: 2, zoom: 11 });
  t.setSize(390, 340);
  const before = t.corner();
  t.tap(5);
  t.setSize(390, 527);
  t.setView({ lat: 3, lng: 4, zoom: 16 });
  t.close();
  t.flush();
  assert.equal(t.view().zoom, 11);
  assert.deepEqual(t.corner(), before, 'every later resize keeps this corner, so the sheet coming back leaves the map as it was');
});

test('the laptop map stays where the report left it, as before', () => {
  const t = run({ phone: false });
  t.setView({ lat: 1, lng: 2, zoom: 11 });
  t.tap(5);
  t.setView({ lat: 3, lng: 4, zoom: 16 });
  t.close();
  t.flush();
  assert.deepEqual(t.view(), { lat: 3, lng: 4, zoom: 16 });
  assert.equal(t.log.filter((l) => l.indexOf('setView') === 0).length, 0);
});

test('app.js exposes Pinalert.openReport and hands it to the map', () => {
  const fn = between(appSource, '  function openReport(id)', '\n  }\n');
  assert.match(fn, /PinalertMap\.openReport\(text\)/);
  assert.match(fn, /setFocusId\(text\);/);
  assert.match(fn, /fetchReports\(\);/);
  assert.match(appSource, /openReport: openReport/);
});

test('the address on load goes through Pinalert.openReport', () => {
  assert.match(feedSource, /Pinalert\.openReport\(new URLSearchParams\(window\.location\.search\)\.get\(REPORT_PARAM\)\);/);
  assert.doesNotMatch(feedSource, /Pinalert\.setFocusId\(new URLSearchParams/);
});
