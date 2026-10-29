// demo_page_test.mjs - runs the real web/static/js/api-base.js, app.js, votes.js
// and demo.js in a Node sandbox and checks the two seams that let the Demo run
// the real scripts unchanged (plan 05-13, D-05, TRUST-02):
//
//   - the report API base: with no data-api-base on the page every report call
//     goes to /api/reports as always; with a Demo base the feed read and the
//     vote post go to that base, and the page announces the centre it loaded;
//   - the fixed pretend position: with data-fixed-lat and data-fixed-lon on the
//     shell the vote path takes the position from the page, never asks the
//     browser, writes no browser storage, and the too far hint judges reports
//     from it; and
//   - the Demo strip: demo.js writes the strip's measured height into
//     --disclaimer-bar-height after the page script has written its own 0px.
//
// The map and the report window read the same attributes (web/demo_contract_test.go
// pins their source; the browser walk of the plan covers them running).
//
// Run it with: node --test web/demo_page_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what these scripts call at load and in these cases.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const read = (name) => fs.readFileSync(path.join(here, 'static', 'js', name), 'utf8');

const DEMO_BASE = '/demo/api/v/abcDEF123_-xyz';
const PRETEND = { lat: '16.5424', lon: '80.6386' };

const settle = async () => { for (let i = 0; i < 12; i += 1) { await Promise.resolve(); } };

class FakeEl {
  constructor(attrs = {}) {
    this.attrs = { ...attrs };
    this.style = { props: {}, setProperty(n, v) { this.props[n] = v; } };
    this.height = 0;
    this.isConnected = true;
  }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  get offsetHeight() { return this.height; }
  getBoundingClientRect() { return { height: this.height }; }
}

// load builds a page: shellAttrs are the attributes of #app-shell (null means
// the page has no shell), strip is an optional .demo-strip element.
// Amended 2026-10-29 by the Phase 5 review fix (CR-B02): mainAttrs are the
// attributes of a main page root (Demo My activity and Demo Official alerts
// carry the base there), found by querySelector('main[<attr>]') or
// querySelector('main[data-demo="1"]'); without it the page has none, as before.
function load({ shellAttrs = {}, strip = null, scripts = ['app.js'], mainAttrs = null } = {}) {
  const fetches = [];
  const events = [];
  const storage = { reads: 0, writes: 0 };
  const geo = { calls: 0 };
  const rootStyle = new FakeEl().style;
  const frames = [];
  const listeners = {};
  const shell = shellAttrs === null ? null : new FakeEl(shellAttrs);
  const main = mainAttrs === null ? null : new FakeEl(mainAttrs);
  const mainMatches = (sel) => {
    const m = /^main\[([a-z-]+)(?:="([^"]*)")?\]$/.exec(sel);
    return !!main && !!m && main.hasAttribute(m[1]) && (m[2] === undefined || main.getAttribute(m[1]) === m[2]);
  };
  const doc = {
    readyState: 'complete',
    documentElement: Object.assign(new FakeEl(), { style: rootStyle }),
    getElementById(id) { return id === 'app-shell' ? shell : null; },
    querySelector(sel) { return sel === '.demo-strip' ? strip : (mainMatches(sel) ? main : null); },
    querySelectorAll() { return []; },
    createElement() { return new FakeEl(); },
    addEventListener(type, fn) { (listeners[type] = listeners[type] || []).push(fn); },
    removeEventListener() {},
    dispatchEvent(ev) { events.push(ev); return true; }
  };
  const win = {
    setInterval() { return 1; },
    setTimeout() { return 1; },
    clearTimeout() {},
    clearInterval() {},
    requestAnimationFrame(fn) { frames.push(fn); return frames.length; },
    addEventListener(type, fn) { (listeners['window:' + type] = listeners['window:' + type] || []).push(fn); },
    removeEventListener() {},
    location: { href: 'http://127.0.0.1/demo', pathname: '/demo', search: '' },
    history: { state: null, replaceState() {} },
    matchMedia() { return { matches: false, addEventListener() {}, addListener() {} }; },
    getComputedStyle() { return { paddingTop: '0', paddingBottom: '0' }; }
  };
  const sandbox = {
    window: win,
    document: doc,
    navigator: {
      geolocation: {
        getCurrentPosition() { geo.calls += 1; },
        watchPosition() { geo.calls += 1; return 0; }
      },
      permissions: { query() { geo.calls += 1; return Promise.resolve({ state: 'prompt' }); } }
    },
    sessionStorage: {
      getItem() { storage.reads += 1; return null; },
      setItem() { storage.writes += 1; },
      removeItem() { storage.writes += 1; }
    },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    AbortController: class { constructor() { this.signal = { aborted: false, addEventListener() {} }; } abort() {} },
    ResizeObserver: undefined,
    URL,
    URLSearchParams,
    Date,
    Promise,
    Math,
    JSON,
    isFinite,
    parseFloat,
    encodeURIComponent
  };
  sandbox.fetch = (target, init) => new Promise((resolve) => {
    const req = { url: typeof target === 'string' ? target : target.url, init: init || {}, resolve };
    fetches.push(req);
  });
  vm.createContext(sandbox);
  scripts.forEach((name) => vm.runInContext(read(name), sandbox, { filename: name }));
  const answer = (req, body, status = 200) => req.resolve({
    ok: status >= 200 && status < 300,
    status,
    headers: { get: () => null },
    json: () => Promise.resolve(body)
  });
  return { sandbox, win, doc, shell, fetches, events, storage, geo, rootStyle, frames, listeners, answer };
}

const lastPath = (m, i = m.fetches.length - 1) => new URL(m.fetches[i].url, 'http://x');

test('api-base.js: no data-api-base leaves the base unset, a Demo base sets it and the Demo flag', () => {
  const real = load({ shellAttrs: {}, scripts: ['api-base.js'] });
  assert.equal(real.win.PinalertApiBase, undefined, 'the real page keeps the default');
  assert.ok(!real.win.PinalertDemoPage);

  const demo = load({ shellAttrs: { 'data-api-base': DEMO_BASE, 'data-demo': '1' }, scripts: ['api-base.js'] });
  assert.equal(demo.win.PinalertApiBase, DEMO_BASE);
  assert.equal(demo.win.PinalertDemoPage, true);
});

test('api-base.js: only a path on this site is taken, never an address', () => {
  for (const bad of ['https://evil.example/api', '//evil.example/api', 'javascript:alert(1)', '/demo/../x y', '', 'demo/api']) {
    const m = load({ shellAttrs: { 'data-api-base': bad, 'data-demo': '1' }, scripts: ['api-base.js'] });
    assert.equal(m.win.PinalertApiBase, undefined, 'refused: ' + JSON.stringify(bad));
  }
});

test('app.js: with no Demo base the feed read goes to /api/reports', async () => {
  const m = load({ shellAttrs: {}, scripts: ['api-base.js', 'app.js'] });
  m.win.Pinalert.setCenter(12.97, 77.59, true);
  m.win.Pinalert.fetchReports();
  assert.equal(m.fetches.length, 1);
  assert.equal(lastPath(m).pathname, '/api/reports');
  assert.equal(lastPath(m).searchParams.get('lat'), '12.97');
});

test('app.js: with a Demo base the feed read goes to the Demo and the centre is announced', async () => {
  const m = load({ shellAttrs: { 'data-api-base': DEMO_BASE, 'data-demo': '1' }, scripts: ['api-base.js', 'app.js'] });
  m.win.Pinalert.setCenter(16.5424, 80.6386, true);
  m.win.Pinalert.fetchReports();
  assert.equal(m.fetches.length, 1);
  assert.equal(lastPath(m).pathname, DEMO_BASE + '/reports');
  assert.equal(m.events.length, 0, 'nothing is announced before the answer');

  m.answer(m.fetches[0], { reports: [] });
  await settle();
  const centre = m.events.filter((e) => e.type === 'pinalert:feed-centre');
  assert.equal(centre.length, 1, 'one centre event after the load');
  // Amended 2026-10-29 by the Phase 5 review fix (WR-B09): the detail also says
  // the centre is the reader's own position (own) and not a searched place.
  assert.deepEqual({ ...centre[0].detail }, { lat: 16.5424, lon: 80.6386, own: true, place: false });
});

test('app.js: a failed read announces no centre', async () => {
  const m = load({ shellAttrs: {}, scripts: ['api-base.js', 'app.js'] });
  m.win.Pinalert.setCenter(12.97, 77.59, true);
  m.win.Pinalert.fetchReports();
  m.answer(m.fetches[0], {}, 500);
  await settle();
  assert.equal(m.events.filter((e) => e.type === 'pinalert:feed-centre').length, 0);
});

test('app.js: the post goes to the Demo base too', () => {
  const m = load({ shellAttrs: { 'data-api-base': DEMO_BASE, 'data-demo': '1' }, scripts: ['api-base.js', 'app.js'] });
  m.win.Pinalert.submitReport({ description: 'water over the road' }).catch(() => {});
  assert.equal(m.fetches.length, 1);
  assert.equal(m.fetches[0].url, DEMO_BASE + '/reports');
  assert.equal(m.fetches[0].init.method, 'POST');
});

test('votes.js: a vote on the Demo posts to the Demo base from the page position, with no browser question and no storage', async () => {
  const m = load({
    shellAttrs: {
      'data-api-base': DEMO_BASE, 'data-demo': '1', 'data-fixed-lat': PRETEND.lat, 'data-fixed-lon': PRETEND.lon,
      'data-vote-radius-km': '1'
    },
    scripts: ['api-base.js', 'app.js', 'votes.js']
  });
  m.win.Pinalert.setCenter(16.5424, 80.6386, true);
  m.win.PinalertVotes.castVote('7', 'confirm');
  await settle();
  assert.equal(m.fetches.length, 1, 'one vote post');
  assert.equal(m.fetches[0].url, DEMO_BASE + '/reports/7/confirm');
  assert.equal(m.fetches[0].init.method, 'POST');
  assert.deepEqual(JSON.parse(m.fetches[0].init.body), { latitude: 16.5424, longitude: 80.6386 });
  assert.equal(m.geo.calls, 0, 'the browser location was never asked');
  assert.equal(m.storage.writes, 0, 'nothing was written to session storage');
  assert.equal(m.storage.reads, 0, 'no stored fix was read either');
});

test('votes.js: without a fixed position the vote path is as before: the stored fix is read and the browser is asked', async () => {
  const m = load({ shellAttrs: { 'data-vote-radius-km': '1' }, scripts: ['api-base.js', 'app.js', 'votes.js'] });
  m.win.PinalertVotes.getVoterLocation({ fresh: false });
  await settle();
  assert.ok(m.storage.reads >= 1, 'the session cache is read');
  assert.ok(m.geo.calls >= 1, 'the permission or the position was asked for');
});

test('votes.js: the too far mark is judged from the page position', () => {
  const m = load({
    shellAttrs: { 'data-fixed-lat': PRETEND.lat, 'data-fixed-lon': PRETEND.lon, 'data-vote-radius-km': '1' },
    scripts: ['api-base.js', 'app.js', 'votes.js']
  });
  const near = { id: 1, latitude: 16.5470, longitude: 80.6386 };
  const far = { id: 2, latitude: 16.5424, longitude: 80.6586 };
  const own = { id: 3, latitude: 16.60, longitude: 80.70, is_own_report: true };
  assert.equal(m.win.PinalertVotes.farFromCachedFix(near), false, 'about 0.5 km away');
  assert.equal(m.win.PinalertVotes.farFromCachedFix(far), true, 'about 2.1 km away');
  assert.equal(m.win.PinalertVotes.farFromCachedFix(own), false, 'the visitor own report is never marked');
  assert.equal(m.geo.calls, 0);
  assert.equal(m.storage.reads + m.storage.writes, 0);
});

test('votes.js: a page with a half given position takes no fixed position', async () => {
  const m = load({ shellAttrs: { 'data-fixed-lat': PRETEND.lat, 'data-vote-radius-km': '1' }, scripts: ['api-base.js', 'app.js', 'votes.js'] });
  m.win.PinalertVotes.getVoterLocation({ fresh: false });
  await settle();
  assert.ok(m.geo.calls >= 1, 'the browser is asked as on the real page');
});

test('demo.js: the strip height is written to --disclaimer-bar-height, now and on resize', () => {
  const strip = new FakeEl();
  strip.height = 52;
  const m = load({ shellAttrs: {}, strip, scripts: ['demo.js'] });
  assert.equal(m.rootStyle.props['--disclaimer-bar-height'], '52px');
  // The page script (theme.js) writes 0px on a page without the laptop strip;
  // the next measurement puts the strip's height back.
  m.rootStyle.setProperty('--disclaimer-bar-height', '0px');
  strip.height = 76;
  (m.listeners['window:resize'] || []).forEach((fn) => fn());
  assert.equal(m.rootStyle.props['--disclaimer-bar-height'], '76px');
  // A frame after load measures once more, for fonts that arrive late.
  strip.height = 80;
  m.frames.forEach((fn) => fn());
  assert.equal(m.rootStyle.props['--disclaimer-bar-height'], '80px');
});

test('demo.js: a page without the strip writes nothing', () => {
  const m = load({ shellAttrs: {}, strip: null, scripts: ['demo.js'] });
  assert.equal(m.rootStyle.props['--disclaimer-bar-height'], undefined);
});

test('demo.js: it writes no storage and never asks the browser', () => {
  const strip = new FakeEl();
  strip.height = 52;
  const m = load({ shellAttrs: {}, strip, scripts: ['demo.js'] });
  assert.equal(m.storage.writes + m.storage.reads, 0);
  assert.equal(m.geo.calls, 0);
});

// ---- The Demo fails closed (Phase 5 review fix CR-B02) ----
// Added 2026-10-29. When api-base.js fails to load, window.PinalertApiBase and
// window.PinalertDemoPage are never set. The scripts then read the page's own
// data-api-base (with the same check), and a Demo page with no usable base
// sends no report call at all: never one to the real /api.

const BAD_BASES = [undefined, '', 'https://evil.example/api', '//evil.example/api', '/demo/../x y', '/'];
const demoShell = (base) => {
  const attrs = { 'data-demo': '1', 'data-fixed-lat': PRETEND.lat, 'data-fixed-lon': PRETEND.lon, 'data-vote-radius-km': '1' };
  if (base !== undefined) { attrs['data-api-base'] = base; }
  return attrs;
};
const toRealApi = (m) => m.fetches.filter((f) => /^\/api(\/|$)/.test(f.url));

test('CR-B02 app.js: with api-base.js missing the feed read and the post go to the page own Demo base', async () => {
  const m = load({ shellAttrs: demoShell(DEMO_BASE), scripts: ['app.js'] });
  assert.equal(m.win.PinalertApiBase, undefined, 'api-base.js never ran');
  assert.equal(m.win.PinalertDemoPage, true, 'the Demo marks still know this is the Demo');
  m.win.Pinalert.setCenter(16.5424, 80.6386, true);
  m.win.Pinalert.fetchReports();
  m.win.Pinalert.submitReport({ description: 'water over the road' }).catch(() => {});
  assert.equal(m.fetches.length, 2);
  assert.equal(lastPath(m, 0).pathname, DEMO_BASE + '/reports');
  assert.equal(m.fetches[1].url, DEMO_BASE + '/reports');
});

test('CR-B02 app.js: a Demo page with no usable base sends no report call at all', async () => {
  for (const bad of BAD_BASES) {
    const m = load({ shellAttrs: demoShell(bad), scripts: ['app.js'] });
    m.win.Pinalert.setCenter(16.5424, 80.6386, true);
    await m.win.Pinalert.fetchReports();
    let refused = null;
    await m.win.Pinalert.submitReport({ description: 'water over the road' }).catch((err) => { refused = err; });
    assert.equal(m.fetches.length, 0, 'no request for base ' + JSON.stringify(bad));
    assert.equal(toRealApi(m).length, 0);
    assert.ok(refused, 'the post fails');
    assert.equal(refused.queueable, false, 'and is not kept to send later');
    assert.equal(m.win.Pinalert.state.status, 'error', 'the feed shows its load error');
  }
});

test('CR-B02 app.js: the Demo My activity root carries the base on main', async () => {
  const m = load({ shellAttrs: null, mainAttrs: { 'data-demo': '1', 'data-api-base': DEMO_BASE }, scripts: ['app.js'] });
  assert.equal(m.win.PinalertDemoPage, true);
  m.win.Pinalert.submitReport({ description: 'x' }).catch(() => {});
  assert.equal(m.fetches[0].url, DEMO_BASE + '/reports');
  const closed = load({ shellAttrs: null, mainAttrs: { 'data-demo': '1' }, scripts: ['app.js'] });
  await closed.win.Pinalert.submitReport({ description: 'x' }).catch(() => {});
  assert.equal(closed.fetches.length, 0);
});

test('CR-B02 app.js: a real page with no base still uses /api and is not the Demo', () => {
  const m = load({ shellAttrs: {}, scripts: ['app.js'] });
  assert.ok(!m.win.PinalertDemoPage);
  m.win.Pinalert.submitReport({ description: 'x' }).catch(() => {});
  assert.equal(m.fetches[0].url, '/api/reports');
});

test('CR-B02 votes.js: with api-base.js missing a vote goes to the page own Demo base', async () => {
  const m = load({ shellAttrs: demoShell(DEMO_BASE), scripts: ['app.js', 'votes.js'] });
  m.win.Pinalert.setCenter(16.5424, 80.6386, true);
  m.win.PinalertVotes.castVote('7', 'confirm').catch(() => {});
  await settle();
  assert.equal(m.fetches.length, 1);
  assert.equal(m.fetches[0].url, DEMO_BASE + '/reports/7/confirm');
});

test('CR-B02 votes.js: a Demo page with no usable base casts no vote at all', async () => {
  for (const bad of BAD_BASES) {
    const m = load({ shellAttrs: demoShell(bad), scripts: ['app.js', 'votes.js'] });
    m.win.Pinalert.setCenter(16.5424, 80.6386, true);
    let refused = null;
    await m.win.PinalertVotes.castVote('7', 'confirm').catch((err) => { refused = err; });
    await settle();
    assert.equal(m.fetches.length, 0, 'no vote for base ' + JSON.stringify(bad));
    assert.ok(refused, 'the vote fails');
    assert.equal(m.geo.calls, 0, 'the browser is still never asked');
  }
});

test('CR-B02 demo.js: with api-base.js missing an ended visit still restarts the Demo', () => {
  const shellPage = load({ shellAttrs: { 'data-demo': '1' }, scripts: ['demo.js'] });
  assert.equal(typeof shellPage.win.PinalertDemoEnded, 'function', 'Demo Nearby');
  const mainPage = load({ shellAttrs: null, mainAttrs: { 'data-demo': '1' }, scripts: ['demo.js'] });
  assert.equal(typeof mainPage.win.PinalertDemoEnded, 'function', 'Demo My activity and Demo Official alerts');
  const real = load({ shellAttrs: {}, scripts: ['demo.js'] });
  assert.equal(real.win.PinalertDemoEnded, undefined, 'never on a real page');
  assert.equal(shellPage.fetches.length + mainPage.fetches.length, 0, 'demo.js itself asks nothing');
});

// ---- Demo My activity's pretend position (Phase 5 review fix IN-B03) ----
// Added 2026-10-29. Demo My activity writes the pretend point on its main page
// root, not on #app-shell. votes.js takes it from there too, so Reopen never
// reads the real voter cache on the Demo.

test('IN-B03 votes.js: on Demo My activity the position comes from the main root, with no storage and no browser question', async () => {
  const m = load({
    shellAttrs: null,
    mainAttrs: { 'data-demo': '1', 'data-api-base': DEMO_BASE, 'data-fixed-lat': PRETEND.lat, 'data-fixed-lon': PRETEND.lon },
    scripts: ['api-base.js', 'app.js', 'votes.js']
  });
  let got = null;
  await m.win.PinalertVotes.getVoterLocation({ fresh: false }).then((fix) => { got = fix; });
  assert.equal(got.latitude, 16.5424);
  assert.equal(got.longitude, 80.6386);
  assert.equal(m.storage.reads + m.storage.writes, 0, 'the real voter cache is never read');
  assert.equal(m.geo.calls, 0);
});

// ---- The centre event says what the centre is (Phase 5 review fix WR-B09) ----
// Added 2026-10-29.

test('WR-B09 app.js: the fallback centre is announced as not own and not a place; a searched place as a place', async () => {
  const m = load({ shellAttrs: {}, scripts: ['app.js'] });
  m.win.Pinalert.setCenter(16.5, 80.6, false);
  m.win.Pinalert.fetchReports();
  m.answer(m.fetches[0], { reports: [] });
  await settle();
  const first = m.events.filter((e) => e.type === 'pinalert:feed-centre')[0];
  assert.deepEqual({ ...first.detail }, { lat: 16.5, lon: 80.6, own: false, place: false }, 'the boot fallback centre');
  m.win.Pinalert.setPlace({ name: 'Benz Circle', lat: 16.4975, lon: 80.6553 });
  m.win.Pinalert.setCenter(16.4975, 80.6553, false);
  m.win.Pinalert.fetchReports();
  m.answer(m.fetches[1], { reports: [] });
  await settle();
  const second = m.events.filter((e) => e.type === 'pinalert:feed-centre')[1];
  assert.deepEqual({ ...second.detail }, { lat: 16.4975, lon: 80.6553, own: false, place: true }, 'a searched place');
});
