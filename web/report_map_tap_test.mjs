// report_map_tap_test.mjs - runs the real web/static/js/modal.js in a Node
// sandbox with a fake Leaflet and checks quick task 261016-tap: a tap on the
// report form's map moves the pin whether location is on, off or denied, the
// tap does what a drag does (the form counts it as an edit, the readout moves),
// and the click handler is added once per map, never again on a reopen
// (IN-W10).
//
// Run it with: node --test web/report_map_tap_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs in
// .github/workflows/ci.yml). It is not embedded in the server.
// The fakes cover only what modal.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'modal.js'), 'utf8');

const GPS = { lat: 16.5062, lon: 80.648 };
const FALLBACK = { lat: 17.385, lon: 78.4867 };

function fakeEl(id) {
  return {
    id,
    hidden: false,
    textContent: '',
    handlers: {},
    addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); },
    querySelector() { return fakeEl('panel'); }
  };
}

// load runs modal.js once. geo is 'grant', 'deny', 'absent' or 'pending'
// (the answer waits until the test calls answerGeo); setGeo changes it
// between opens, as a person turning location on or off would.
function load(geo) {
  const els = {};
  for (const id of ['report-modal', 'fab-report', 'coord-readout', 'location-notice', 'modal-map']) {
    els[id] = fakeEl(id);
  }
  const document = {
    getElementById: (id) => els[id] || null,
    createElement: () => ({ getContext: () => null })
  };

  const maps = [];
  const markers = [];
  const L = {
    map() {
      const m = {
        handlers: {},
        layers: new Set(),
        view: null,
        on(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); return this; },
        setView(latlng, zoom) { this.view = { lat: latlng[0], lon: latlng[1], zoom }; return this; },
        getZoom() { return this.view ? this.view.zoom : undefined; },
        removeLayer(layer) { this.layers.delete(layer); return this; },
        setMaxZoom() { return this; },
        invalidateSize() { return this; }
      };
      maps.push(m);
      return m;
    },
    tileLayer(u, options) {
      return { options, addTo() { return this; } };
    },
    marker(latlng, options) {
      const mk = {
        pos: { lat: latlng[0], lng: latlng[1] },
        options,
        handlers: {},
        addTo(map) { map.layers.add(this); return this; },
        on(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); return this; },
        getLatLng() { return this.pos; }
      };
      markers.push(mk);
      return mk;
    }
  };

  let mode = geo;
  let pendingAnswer = null;
  const navigator = {};
  Object.defineProperty(navigator, 'geolocation', {
    get() {
      if (mode === 'absent') { return undefined; }
      return {
        getCurrentPosition(ok, fail) {
          const coords = { coords: { latitude: GPS.lat, longitude: GPS.lon } };
          if (mode === 'grant') { ok(coords); return; }
          if (mode === 'deny') { fail({ code: 1 }); return; }
          pendingAnswer = { ok: () => ok(coords), fail: () => fail({ code: 3 }) };
        }
      };
    }
  });

  let picker = null;
  const window = {
    setTimeout: (fn) => { fn(); return 1; },
    PinalertReportForm: {
      mount(opts) { picker = opts.location; },
      open() {}
    }
  };
  const Pinalert = {
    config: { fallbackLat: FALLBACK.lat, fallbackLon: FALLBACK.lon },
    setText: (el, v) => { el.textContent = v === null || v === undefined ? '' : String(v); }
  };

  const sandbox = { window, document, navigator, Pinalert, L };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  assert.ok(picker, 'modal.js mounted the form with its location interface');

  const touched = { count: 0 };
  const hooks = { markTouched: () => { touched.count += 1; } };
  const map = () => maps[0];
  const clickHandlers = () => (map().handlers.click || []).length;
  const pinsOnMap = () => [...map().layers].filter((l) => markers.includes(l));
  const tap = (lat, lng) => {
    for (const fn of map().handlers.click || []) { fn({ latlng: { lat, lng } }); }
  };
  return {
    picker,
    hooks,
    touched,
    maps,
    markers,
    els,
    map,
    clickHandlers,
    pinsOnMap,
    tap,
    open: () => picker.start(hooks),
    close: () => picker.reset(),
    setGeo: (m) => { mode = m; },
    answerGeo: (ok) => { const a = pendingAnswer; pendingAnswer = null; if (ok) { a.ok(); } else { a.fail(); } }
  };
}

const readout = (lat, lon) => lat.toFixed(5) + ', ' + lon.toFixed(5);
// at copies the pin position out of the sandbox, whose objects carry the
// sandbox's own Object prototype and so never deep equal a plain literal.
const at = (s) => {
  const p = s.picker.current();
  return p === null ? null : { latitude: p.latitude, longitude: p.longitude };
};

test('location on: a tap moves the GPS pin and counts as an edit', () => {
  const s = load('grant');
  s.open();
  assert.deepEqual(at(s), { latitude: GPS.lat, longitude: GPS.lon });
  assert.equal(s.touched.count, 0, 'the GPS pin itself is not an edit');

  s.tap(16.51, 80.65);
  assert.deepEqual(at(s), { latitude: 16.51, longitude: 80.65 });
  assert.equal(s.els['coord-readout'].textContent, readout(16.51, 80.65));
  assert.equal(s.touched.count, 1);
  assert.equal(s.pinsOnMap().length, 1, 'one pin on the map after the tap');
  assert.equal(s.clickHandlers(), 1);
});

test('location denied: the notice shows and a tap drops the pin', () => {
  const s = load('deny');
  s.open();
  assert.equal(s.els['location-notice'].hidden, false);
  assert.equal(s.picker.isSet(), false);
  assert.deepEqual(s.map().view, { lat: FALLBACK.lat, lon: FALLBACK.lon, zoom: 11 });

  s.tap(17.4, 78.5);
  assert.deepEqual(at(s), { latitude: 17.4, longitude: 78.5 });
  assert.equal(s.els['coord-readout'].textContent, readout(17.4, 78.5));
  assert.equal(s.touched.count, 1);
  assert.equal(s.clickHandlers(), 1);
});

test('no geolocation in this browser: a tap drops the pin', () => {
  const s = load('absent');
  s.open();
  assert.equal(s.els['location-notice'].hidden, false);
  s.tap(17.39, 78.49);
  assert.deepEqual(at(s), { latitude: 17.39, longitude: 78.49 });
  assert.equal(s.touched.count, 1);
});

test('reopening in any state keeps one map and one click handler (IN-W10)', () => {
  const s = load('deny');
  s.open();
  s.close();
  s.setGeo('grant');
  s.open();
  s.close();
  s.setGeo('absent');
  s.open();
  s.close();
  s.setGeo('deny');
  s.open();

  assert.equal(s.maps.length, 1, 'the map is built once');
  assert.equal(s.clickHandlers(), 1, 'one click handler after four opens');

  const before = s.markers.length;
  s.tap(17.41, 78.51);
  assert.equal(s.markers.length, before + 1, 'one tap builds one pin, not one per open');
  assert.equal(s.touched.count, 1, 'one tap is one edit');
  assert.equal(s.pinsOnMap().length, 1);
});

test('after a denied open, a later open with location on still moves the pin on a tap', () => {
  const s = load('deny');
  s.open();
  s.close();
  s.setGeo('grant');
  s.open();
  assert.deepEqual(at(s), { latitude: GPS.lat, longitude: GPS.lon });
  s.tap(16.52, 80.66);
  assert.deepEqual(at(s), { latitude: 16.52, longitude: 80.66 });
});

test('a tap before the position answers is kept when it arrives', () => {
  const s = load('pending');
  s.open();
  s.tap(16.6, 80.7);
  s.answerGeo(true);
  assert.deepEqual(at(s), { latitude: 16.6, longitude: 80.7 });

  const t = load('pending');
  t.open();
  t.tap(16.61, 80.71);
  t.answerGeo(false);
  assert.deepEqual(at(t), { latitude: 16.61, longitude: 80.71 });
  assert.equal(t.els['location-notice'].hidden, true, 'no location off notice once a pin is set');
});

test('dragging the pin still moves the readout and counts as an edit', () => {
  const s = load('grant');
  s.open();
  const pin = s.pinsOnMap()[0];
  assert.equal(pin.options.draggable, true);
  pin.pos = { lat: 16.5, lng: 80.64 };
  for (const fn of pin.handlers.dragend) { fn(); }
  assert.equal(s.els['coord-readout'].textContent, readout(16.5, 80.64));
  assert.equal(s.touched.count, 1);

  s.tap(16.53, 80.67);
  const moved = s.pinsOnMap()[0];
  assert.equal(moved.options.draggable, true, 'a tapped pin can still be dragged');
  moved.pos = { lat: 16.54, lng: 80.68 };
  for (const fn of moved.handlers.dragend) { fn(); }
  assert.deepEqual(at(s), { latitude: 16.54, longitude: 80.68 });
  assert.equal(s.touched.count, 3);
});

test('the missing location message names a tap on the map', () => {
  const s = load('deny');
  s.open();
  assert.equal(s.picker.isSet(), false);
  assert.equal(s.picker.missingMessage(),
    'Set a location by searching, tapping the map, or allowing location access.');
});

test('a chosen search result lands at street level zoom and a tap then moves the pin', () => {
  const s = load('deny');
  s.open();
  assert.equal(s.map().view.zoom, 11);
  s.picker.choose({ lat: 16.507, lon: 80.649 });
  assert.deepEqual(s.map().view, { lat: 16.507, lon: 80.649, zoom: 16 });
  s.tap(16.5072, 80.6493);
  assert.deepEqual(at(s), { latitude: 16.5072, longitude: 80.6493 });
  assert.equal(s.pinsOnMap().length, 1);
});
