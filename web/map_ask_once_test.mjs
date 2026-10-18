// map_ask_once_test.mjs - checks that Nearby asks the browser for the device
// position on its own only once per account in a browser (Phase 3.4, plan 36).
//
// map.js builds a whole Leaflet map on load and keeps its steps private, so this
// test cuts the two pieces it needs out of the real file text (the first
// centring, centerOnVisitor and the helpers next to it, and the return to the
// reader, recenterOnReader) and runs them with the real position.js, a fake
// shared store, a fake map, a fake navigator.geolocation and navigator.permissions
// and a memory storage. The cuts are found by their comments and function names,
// so a rename in map.js fails here loudly instead of passing on stale code.
//
// Run it with: node --test web/map_ask_once_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const mapSource = fs.readFileSync(path.join(here, 'static', 'js', 'map.js'), 'utf8');
const positionSource = fs.readFileSync(path.join(here, 'static', 'js', 'position.js'), 'utf8');

// same compares values made inside the sandbox, whose objects have another realm's prototypes.
function same(actual, expected) {
  assert.deepStrictEqual(JSON.parse(JSON.stringify(actual)), expected);
}

function between(from, to) {
  const start = mapSource.indexOf(from);
  const end = mapSource.indexOf(to, start);
  assert.ok(start >= 0 && end > start, 'map.js no longer has the text between "' + from + '" and "' + to + '"');
  return mapSource.slice(start, end);
}

const firstCentring = between('  // centerOnVisitor centres', '  // moveTo puts the map on a place');
const returnToReader = between('  // recenterOnReader is Back to my location', '  // getCenter is the visible centre');

const KEY = 'pinalert.lowLastPosition';
const ASKED = 'pinalert.locationAsked.7';

const FALLBACK = { lat: 17.385, lon: 78.4867, zoom: 11 };
const HOME = { lat: 17.4, lon: 78.45, zoom: 13 };

// run builds the sandbox. permission is 'granted', 'prompt', 'denied' or null
// (no Permissions API). storage holds the starting local storage values.
function run({ permission = 'prompt', storage = {}, fallback = FALLBACK, geolocation = true, throwingStorage = false, account = '7' } = {}) {
  const data = new Map(Object.entries(storage));
  const local = throwingStorage
    ? { getItem() { throw new Error('blocked'); }, setItem() { throw new Error('blocked'); }, removeItem() { throw new Error('blocked'); } }
    : {
      getItem: (k) => (data.has(k) ? data.get(k) : null),
      setItem: (k, v) => { data.set(k, String(v)); },
      removeItem: (k) => { data.delete(k); }
    };

  const asks = [];
  const views = [];
  const centres = [];
  const calls = { fetch: 0, poll: 0, clearPlace: 0 };
  const moves = [];
  const navigator = {};
  if (geolocation) {
    navigator.geolocation = { getCurrentPosition(ok, fail, opts) { asks.push({ ok, fail, opts }); } };
  }
  if (permission !== null) {
    navigator.permissions = { query: () => Promise.resolve({ state: permission }) };
  }

  const sandbox = {
    window: null,
    navigator,
    Promise,
    Date,
    JSON,
    isFinite,
    Math,
    document: {
      querySelector(selector) {
        if (selector !== '[data-account-id]' || account === null) { return null; }
        return { getAttribute: (name) => (name === 'data-account-id' ? account : null) };
      }
    },
    Pinalert: {
      config: { fallbackLat: fallback.lat, fallbackLon: fallback.lon, fallbackZoom: fallback.zoom },
      setCenter(lat, lon, own) { centres.push({ lat, lon, own }); },
      fetchReports() { calls.fetch += 1; },
      startPolling() { calls.poll += 1; },
      clearPlace() { calls.clearPlace += 1; }
    }
  };
  sandbox.window = sandbox;
  sandbox.localStorage = local;
  vm.createContext(sandbox);
  vm.runInContext(positionSource, sandbox, { filename: 'position.js' });

  const glue = `
    var map = { setView: function (c, z) { __views.push({ lat: c[0], lon: c[1], zoom: z }); } };
    var recenterSeq = 0;
    function moveTo(lat, lon, zoom) { __moves.push({ lat: lat, lon: lon, zoom: zoom }); }
    ${firstCentring}
    ${returnToReader}
    this.__start = centerOnVisitor;
    this.__back = recenterOnReader;
  `;
  sandbox.__views = views;
  sandbox.__moves = moves;
  vm.runInContext('(function () {' + glue + '}).call(this);', sandbox, { filename: 'map-cut.js' });

  const settle = async () => {
    for (let i = 0; i < 5; i++) { await new Promise((resolve) => setImmediate(resolve)); }
  };
  return { sandbox, data, asks, views, centres, calls, moves, settle, start: sandbox.__start, back: sandbox.__back };
}

const keptFix = (account = '7') => JSON.stringify({ lat: 12.97, lon: 77.59, savedAt: 1700000000000, account });

test('the first load asks the browser once and sets the marker', async () => {
  const page = run({ permission: 'prompt' });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 1, 'the first load asks');
  assert.equal(page.data.get(ASKED), '1');
  page.asks[0].ok({ coords: { latitude: 12.9, longitude: 77.5 } });
  same(page.views[0], { lat: 12.9, lon: 77.5, zoom: 13 });
  assert.equal(page.centres[0].own, true);
  assert.equal(JSON.parse(page.data.get(KEY)).lat, 12.9, 'the fix is kept');
  assert.equal(page.calls.poll, 1);
});

test('a second load with the marker and a prompt state does not ask and starts at the kept fix', async () => {
  const page = run({ permission: 'prompt', storage: { [ASKED]: '1', [KEY]: keptFix() } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 0, 'the browser is not asked again');
  same(page.views[0], { lat: 12.97, lon: 77.59, zoom: 13 });
  same(page.centres[0], { lat: 12.97, lon: 77.59, own: true });
  assert.equal(JSON.parse(page.data.get(KEY)).savedAt, 1700000000000, 'reusing the fix does not renew its time');
  assert.equal(page.calls.fetch, 1);
  assert.equal(page.calls.poll, 1);
});

test('a denied state with the marker does not ask either', async () => {
  const page = run({ permission: 'denied', storage: { [ASKED]: '1', [KEY]: keptFix() } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 0);
  assert.equal(page.views[0].lat, 12.97);
});

test('a browser with no Permissions API (Safari) and the marker does not ask', async () => {
  const page = run({ permission: null, storage: { [ASKED]: '1', [KEY]: keptFix() } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 0);
  assert.equal(page.views[0].lat, 12.97);
});

test('a granted state reads a fresh fix silently on every load', async () => {
  const page = run({ permission: 'granted', storage: { [ASKED]: '1', [KEY]: keptFix() } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 1, 'a fresh fix is read');
  page.asks[0].ok({ coords: { latitude: 13.0, longitude: 77.6 } });
  same(page.views[0], { lat: 13.0, lon: 77.6, zoom: 13 });
  assert.equal(JSON.parse(page.data.get(KEY)).lat, 13.0, 'and the kept fix is renewed');
});

test('with the marker and no kept fix the map starts at the home area centre, then the fallback, and does not ask', async () => {
  const home = run({ permission: 'prompt', fallback: HOME, storage: { [ASKED]: '1' } });
  home.start();
  await home.settle();
  assert.equal(home.asks.length, 0);
  same(home.views[0], HOME, 'the home area centre at its zoom');
  assert.equal(home.centres[0].own, false, 'a fallback centre is never the reader\'s own position');

  const city = run({ permission: 'prompt', storage: { [ASKED]: '1' } });
  city.start();
  await city.settle();
  assert.equal(city.asks.length, 0);
  same(city.views[0], FALLBACK);
  assert.equal(city.centres[0].own, false);
});

test('a kept fix from another account is not used', async () => {
  const page = run({ permission: 'prompt', storage: { [ASKED]: '1', [KEY]: keptFix('8') } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 0);
  same(page.views[0], FALLBACK);
});

test('another account is asked for itself even when this one was asked', async () => {
  const page = run({ permission: 'prompt', account: '8', storage: { [ASKED]: '1' } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 1);
});

test('Back to my location asks on the tap and updates the kept fix', async () => {
  const page = run({ permission: 'prompt', storage: { [ASKED]: '1', [KEY]: keptFix() } });
  page.start();
  await page.settle();
  assert.equal(page.asks.length, 0);

  const done = page.back();
  assert.equal(page.asks.length, 1, 'the tap asks');
  page.asks[0].ok({ coords: { latitude: 13.5, longitude: 77.7 } });
  const result = await done;
  same(result, { own: true });
  same(page.moves[0], { lat: 13.5, lon: 77.7, zoom: 13 });
  const kept = JSON.parse(page.data.get(KEY));
  assert.equal(kept.lat, 13.5);
  assert.notEqual(kept.savedAt, 1700000000000, 'the kept fix is renewed');
});

test('Back to my location counts as the one question', async () => {
  const page = run({ permission: 'prompt' });
  page.back();
  assert.equal(page.data.get(ASKED), '1');
});

test('a storage that throws keeps today behaviour: it asks on every load and the map still starts', async () => {
  for (let i = 0; i < 2; i++) {
    const page = run({ permission: 'prompt', throwingStorage: true });
    page.start();
    await page.settle();
    assert.equal(page.asks.length, 1);
    page.asks[0].fail({ code: 1 });
    same(page.views[0], FALLBACK, 'a refusal starts at the fallback');
    assert.equal(page.calls.poll, 1);
  }
});

test('a browser with no location API starts at the fallback without throwing', async () => {
  const page = run({ permission: null, geolocation: false });
  page.start();
  await page.settle();
  same(page.views[0], FALLBACK);
  assert.equal(page.centres[0].own, false);
});
