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

// load runs position.js on a page. account is the id the page carries in its
// data-account-id attribute ('7' by default); null is a page with no such
// attribute (the first time flow).
function load(storage, account = '7') {
  const sandbox = { window: null, Date, JSON, isFinite, Math };
  sandbox.window = sandbox;
  sandbox.localStorage = storage;
  sandbox.document = {
    querySelector(selector) {
      if (selector !== '[data-account-id]' || account === null) { return null; }
      return { getAttribute: (name) => (name === 'data-account-id' ? account : null) };
    }
  };
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

// Amended 2026-10-17 by Phase 3.4 plan 25 (code review part 2 WR-01): a stored
// value belongs to the account that saved it. The shape under the shared key
// is the same ({lat, lon, savedAt}) with one more field, account.
test('a stored value in the shared shape that this account saved reads back', () => {
  const position = load(memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5, savedAt: 1760000000000, account: '7' }) }));
  assert.deepEqual({ ...position.read() }, { lat: 17.4, lon: 78.5, savedAt: 1760000000000 });
});

test('WR-01: save keeps the account of the page next to the fix', () => {
  const storage = memoryStorage();
  load(storage, '7').save(12.97, 77.59);
  assert.equal(JSON.parse(storage.data.get(KEY)).account, '7');
});

test('WR-01: a fix another account saved is not read, and it is removed', () => {
  const storage = memoryStorage();
  load(storage, '7').save(12.97, 77.59);
  assert.equal(load(storage, '8').read(), null, 'the next account on this browser does not get it');
  assert.equal(storage.data.has(KEY), false, 'and the earlier account\'s place is gone from the browser');
});

test('WR-01: the same account still reads its own fix after a log out and back in', () => {
  const storage = memoryStorage();
  load(storage, '7').save(12.97, 77.59);
  assert.equal(load(storage, '7').read().lat, 12.97);
});

test('WR-01: a value with no owner (the Low data view, the first time flow, an old browser) is taken over by the first account that reads it', () => {
  const storage = memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5, savedAt: 1760000000000 }) });
  const first = load(storage, '7').read();
  assert.deepEqual({ ...first }, { lat: 17.4, lon: 78.5, savedAt: 1760000000000 });
  const kept = JSON.parse(storage.data.get(KEY));
  assert.equal(kept.account, '7', 'it now belongs to the account that read it');
  assert.equal(kept.savedAt, 1760000000000, 'with its own time, not a new one');
  assert.equal(load(storage, '8').read(), null, 'a second account never gets it from the first');
  assert.equal(storage.data.has(KEY), false);

  const flow = memoryStorage();
  assert.equal(load(flow, null).save(12.97, 77.59), true, 'the first time flow can still save');
  assert.equal(JSON.parse(flow.data.get(KEY)).account, '', 'but it cannot say whose fix it is');
  assert.equal(load(flow, '7').read().lat, 12.97, 'the account that then reads it takes it over');
  assert.equal(JSON.parse(flow.data.get(KEY)).account, '7');
  assert.equal(load(flow, '8').read(), null);
});

test('WR-01: taking over an ownerless value that has no time keeps it without one, and a blocked store still reads it', () => {
  const storage = memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5 }) });
  assert.equal(load(storage, '7').read().savedAt, null);
  assert.equal(JSON.parse(storage.data.get(KEY)).account, '7');

  const blocked = memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5, savedAt: 5 }) });
  blocked.setItem = () => { throw new Error('blocked'); };
  assert.equal(load(blocked, '7').read().lat, 17.4);
});

test('WR-01: a page that does not say which account it is for reads nothing and removes nothing', () => {
  const storage = memoryStorage();
  load(storage, '7').save(12.97, 77.59);
  assert.equal(load(storage, null).read(), null);
  assert.equal(storage.data.has(KEY), true);
  for (const odd of ['', '0', '-3', 'abc', '1.5', '7 ']) {
    assert.equal(load(storage, odd).read(), null, 'account "' + odd + '" is not an id');
    assert.equal(storage.data.has(KEY), true);
  }
});

test('WR-01: only the position key is ever removed; the queue and the theme choice stay', () => {
  const storage = memoryStorage({ 'pinalert.theme': 'dark', 'pinalert.queue.7': '[{"x":1}]' });
  const removed = [];
  const original = storage.removeItem;
  storage.removeItem = (k) => { removed.push(k); original(k); };
  load(storage, '7').save(12.97, 77.59);
  load(storage, '8').read();
  assert.deepEqual(removed, [KEY]);
  assert.equal(storage.data.get('pinalert.theme'), 'dark');
  assert.equal(storage.data.get('pinalert.queue.7'), '[{"x":1}]');
});

test('WR-01: no age limit is invented, an old fix of this account still reads', () => {
  const storage = memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5, savedAt: 1, account: '7' }) });
  assert.equal(load(storage, '7').read().lat, 17.4);
});

test('WR-01: a storage that throws on removal still reads null', () => {
  const storage = memoryStorage({ [KEY]: JSON.stringify({ lat: 17.4, lon: 78.5, savedAt: 1, account: '7' }) });
  storage.removeItem = () => { throw new Error('blocked'); };
  assert.equal(load(storage, '8').read(), null);
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
