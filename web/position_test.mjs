// position_test.mjs - runs the real web/static/js/position.js in a Node sandbox
// (Phase 3.4, plan 10, D-27): the small module that remembers the last known
// device position in this browser, under the key the Low data view already uses.
//
// Run it with: node --test web/position_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'position.js'), 'utf8');

const KEY = 'pinalert.lowLastPosition';

function load(storage) {
  const sandbox = { window: null, Date, JSON, isFinite, Math };
  sandbox.window = sandbox;
  sandbox.localStorage = storage;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'position.js' });
  return sandbox.PinalertPosition;
}

function memoryStorage(initial) {
  const data = new Map(Object.entries(initial || {}));
  return {
    data,
    getItem(k) { return data.has(k) ? data.get(k) : null; },
    setItem(k, v) { data.set(k, String(v)); },
    removeItem(k) { data.delete(k); }
  };
}

const throwing = {
  getItem() { throw new Error('storage is blocked'); },
  setItem() { throw new Error('storage is blocked'); }
};

test('save then read returns the same numbers with a savedAt, under the shared key', () => {
  const storage = memoryStorage();
  const position = load(storage);
  assert.equal(position.save(12.97, 77.59), true);
  assert.ok(storage.data.has(KEY), 'the Low data view key must be used');
  const stored = JSON.parse(storage.data.get(KEY));
  assert.equal(stored.lat, 12.97);
  assert.equal(stored.lon, 77.59);
  assert.equal(typeof stored.savedAt, 'number');

  const read = position.read();
  assert.equal(read.lat, 12.97);
  assert.equal(read.lon, 77.59);
  assert.equal(typeof read.savedAt, 'number');
});

test('a stored value the Low data view wrote reads back', () => {
  const position = load(memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5, savedAt: 1760000000000 }) }));
  assert.deepEqual({ ...position.read() }, { lat: 17.4, lon: 78.5, savedAt: 1760000000000 });
});

test('nothing stored reads as null', () => {
  assert.equal(load(memoryStorage()).read(), null);
});

test('a value that is not JSON, not finite, out of range or the wrong shape reads as null', () => {
  const bad = [
    'not json',
    '{"lat": 12.9',
    JSON.stringify({ lat: 91, lon: 10 }),
    JSON.stringify({ lat: 10, lon: -181 }),
    JSON.stringify({ lat: 'x', lon: 10 }),
    JSON.stringify({ lat: null, lon: null }),
    JSON.stringify([12.9, 77.5]),
    'null',
    '42'
  ];
  for (const raw of bad) {
    assert.equal(load(memoryStorage({ [KEY]: raw })).read(), null, 'should read as null: ' + raw);
  }
});

test('a pair that is not a real position is refused and stores nothing', () => {
  const storage = memoryStorage();
  const position = load(storage);
  for (const [lat, lon] of [[NaN, 10], [10, Infinity], [100, 10], [10, 200], ['12', '77'], [undefined, undefined]]) {
    assert.equal(position.save(lat, lon), false);
  }
  assert.equal(storage.data.size, 0);
});

test('a storage that throws reads null and save does not throw', () => {
  const position = load(throwing);
  assert.equal(position.read(), null);
  assert.doesNotThrow(() => position.save(12.9, 77.5));
  assert.equal(position.save(12.9, 77.5), false);
});
