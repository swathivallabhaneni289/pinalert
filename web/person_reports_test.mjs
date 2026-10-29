// person_reports_test.mjs - runs the real web/static/js/person-reports.js in a
// Node sandbox and checks the Distance cells of a profile's Recent reports
// (Phase 9 plan 11, D-06, D-07, UI-SPEC Surface 5): a live cell reads "{d} km",
// a finished cell reads "About {d} km" with at least 1, every cell reads
// Unknown without a position, and a cell with no coordinates is left alone.
//
// Run it with: node --test web/person_reports_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'person-reports.js'), 'utf8');

function cell(lat, lon, approx, text = 'Unknown') {
  const attrs = {};
  if (lat !== null) { attrs['data-lat'] = String(lat); }
  if (lon !== null) { attrs['data-lon'] = String(lon); }
  if (approx !== null) { attrs['data-approx'] = String(approx); }
  return {
    attrs,
    textContent: text,
    getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; }
  };
}

// load runs the script on a page of these cells. position is what the kept
// position reads as: an object, or null for none.
function load(cells, position) {
  const document = {
    readyState: 'complete',
    querySelector: () => (cells.length > 0 ? cells[0] : null),
    querySelectorAll: () => cells,
    addEventListener() {}
  };
  const window = { PinalertPosition: { read: () => position } };
  vm.runInNewContext(source, { document, window, Math, Number, isFinite });
  return window.PinalertPersonReports;
}

const HERE = { lat: 12.9716, lon: 77.5946 };

test('with a position a live cell reads "N km" and a finished cell reads "About N km"', () => {
  const live = cell(13.0616, 77.5946, false);
  const farLive = cell(13.2716, 77.5946, false);
  const finished = cell(13.2716, 77.5946, true);
  load([live, farLive, finished], HERE);
  assert.equal(live.textContent, '10 km');
  assert.equal(farLive.textContent, '33 km');
  assert.equal(finished.textContent, 'About 33 km');
});

test('a close live cell reads one decimal and a close finished cell reads at least 1 km', () => {
  const live = cell(12.9816, 77.5946, false);
  const finished = cell(12.9726, 77.5946, true);
  load([live, finished], HERE);
  assert.equal(live.textContent, '1.1 km');
  assert.equal(finished.textContent, 'About 1 km');
});

test('with no position every cell reads Unknown', () => {
  const live = cell(13.0616, 77.5946, false, 'stale words');
  const finished = cell(13.0616, 77.5946, true, 'stale words');
  load([live, finished], null);
  assert.equal(live.textContent, 'Unknown');
  assert.equal(finished.textContent, 'Unknown');
});

test('a cell without coordinates is left as it is and a cell with bad numbers reads Unknown', () => {
  const bare = cell(null, null, null, 'untouched');
  const bad = cell('abc', 77.5, false, 'untouched');
  const api = load([bare, bad], HERE);
  assert.equal(bare.textContent, 'untouched');
  assert.equal(bad.textContent, 'Unknown');
  assert.equal(typeof api.fillDistances, 'function');
});

test('fillDistances can be called again with a new position', () => {
  const live = cell(13.0616, 77.5946, false);
  const api = load([live], null);
  assert.equal(live.textContent, 'Unknown');
  api.fillDistances(HERE);
  assert.equal(live.textContent, '10 km');
});

test('the script reaches the page only through textContent', () => {
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write/.test(source), false);
});
