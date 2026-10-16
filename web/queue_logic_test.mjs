// queue_logic_test.mjs - runs the real web/static/js/queue.js in a Node sandbox
// and checks four things the Go source tests cannot: saved reports send oldest
// first, a saved report is never discarded for age or failure, another
// account's report is never sent, drawn or counted, and Edit and resend is
// drawn as a link only for a single slash path.
//
// Run it with: node --test web/queue_logic_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs in
// .github/workflows/ci.yml). It is not embedded in the server.
// The fakes below cover only what queue.js calls: a small in memory IndexedDB,
// a minimal element model, timers that never fire on their own, and a fake
// Pinalert whose submitReport answers from a script per case.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'queue.js'), 'utf8');

const DAY_MS = 24 * 60 * 60 * 1000;

// ----- fake element model -----

class FakeEl {
  constructor(tag) {
    this.tag = tag;
    this.attrs = {};
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.hidden = false;
    this.className = '';
    this.id = '';
    this.type = '';
    this.handlers = {};
    this.focused = false;
  }
  get textContent() {
    return this.own + this.children.map((c) => c.textContent).join('');
  }
  set textContent(value) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(value);
  }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  getAttribute(name) { return Object.hasOwn(this.attrs, name) ? this.attrs[name] : null; }
  removeAttribute(name) { delete this.attrs[name]; }
  appendChild(node) { return this.insertBefore(node, null); }
  insertBefore(node, ref) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
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
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  descendants() {
    const out = [];
    this.children.forEach((c) => { out.push(c, ...c.descendants()); });
    return out;
  }
  matches(selector) {
    if (selector.startsWith('.')) { return this.className.split(/\s+/).includes(selector.slice(1)); }
    return this.tag === selector;
  }
  querySelector(selector) {
    const parts = selector.split(',').map((s) => s.trim());
    return this.descendants().find((el) => parts.some((p) => el.matches(p))) || null;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  dispatchEvent() { return true; }
  focus() { this.focused = true; this.ownerDocument.activeElement = this; }
}

function makeDocument() {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { const el = new FakeEl(tag); el.ownerDocument = doc; return el; },
    createTextNode(text) { const el = new FakeEl('#text'); el.ownerDocument = doc; el.own = String(text); return el; },
    getElementById(id) { return doc.byId[id] || null; }
  };
  return doc;
}

// ----- fake IndexedDB -----

function makeIndexedDB(initial) {
  const data = new Map();
  initial.forEach((record) => data.set(record.request_id, structuredClone(record)));
  let created = initial.length > 0;
  const later = (fn) => queueMicrotask(fn);
  // control lets a case break the store: failReads makes that many reads fail,
  // failOpen makes every open fail, failDeletes makes that many delete
  // transactions abort, deadOnWrite makes that many write transactions find
  // their connection silently dead (it throws on use), and closeAll closes
  // every open connection the way a browser does, firing onclose.
  const control = { failReads: 0, failOpen: false, failDeletes: 0, deadOnWrite: 0, opens: 0, connections: [] };
  control.closeAll = (silently = false) => {
    control.connections.forEach((database) => {
      database.closed = true;
      if (!silently && database.onclose) { database.onclose(); }
    });
  };
  const store = {
    createIndex() {},
    index() {
      return {
        getAll(range) {
          const request = { result: undefined, onsuccess: null, onerror: null };
          if (control.failReads > 0) {
            control.failReads -= 1;
            later(() => { if (request.onerror) { request.onerror(); } });
            return request;
          }
          later(() => {
            // The fake ignores the key range on purpose and returns every row,
            // so the account check inside queue.js is what gets tested.
            request.result = [...data.values()].map((r) => structuredClone(r));
            if (request.onsuccess) { request.onsuccess(); }
          });
          return request;
        }
      };
    }
  };
  const tx = (mode) => {
    const t = { oncomplete: null, onerror: null, onabort: null };
    t.objectStore = () => ({
      put(record) { data.set(record.request_id, structuredClone(record)); later(() => t.oncomplete && t.oncomplete()); },
      delete(id) {
        if (t.failing || control.failDeletes > 0) {
          if (!t.failing) {
            control.failDeletes -= 1;
            t.failing = true;
            later(() => t.onabort && t.onabort());
          }
          return;
        }
        data.delete(id);
        later(() => t.oncomplete && t.oncomplete());
      },
      index: store.index
    });
    return t;
  };
  return {
    data,
    control,
    indexedDB: {
      open() {
        const request = { result: null, onupgradeneeded: null, onsuccess: null, onerror: null, onblocked: null };
        control.opens += 1;
        if (control.failOpen) {
          later(() => { if (request.onerror) { request.onerror(); } });
          return request;
        }
        const database = {
          closed: false,
          onclose: null,
          objectStoreNames: { contains: () => created },
          createObjectStore() { created = true; return store; },
          transaction(name, mode) {
            if (mode === 'readwrite' && control.deadOnWrite > 0) {
              control.deadOnWrite -= 1;
              database.closed = true;
            }
            if (database.closed) { throw new Error('InvalidStateError'); }
            return tx(mode);
          },
          close() { database.closed = true; }
        };
        control.connections.push(database);
        request.result = database;
        later(() => {
          if (!created && request.onupgradeneeded) { request.onupgradeneeded(); }
          if (request.onsuccess) { request.onsuccess(); }
        });
        return request;
      }
    }
  };
}

// ----- one sandbox per case -----

function loadQueue({ records = [], answers = [], indexedDBThrows = false }) {
  const doc = makeDocument();
  const idb = makeIndexedDB(records);
  const submitted = [];
  const toasts = [];
  const pinalert = {
    CATEGORY_LABELS: { flood: 'Flood', other: 'Other' },
    setText(el, text) { el.textContent = text; },
    showToast(message) { toasts.push(message); },
    formatWait() { return '1 minute'; },
    submitReport(payload) {
      submitted.push(payload);
      const answer = answers.shift();
      return answer ? answer(payload) : Promise.resolve({ id: submitted.length });
    }
  };
  const win = {
    addEventListener() {},
    setInterval() { return 1; },
    setTimeout() { return 1; },
    clearTimeout() {},
    location: { href: 'http://127.0.0.1/profile' },
    history: { state: null, replaceState() {} }
  };
  const sandbox = {
    window: win,
    document: doc,
    Pinalert: pinalert,
    indexedDB: idb.indexedDB,
    IDBKeyRange: { only: (value) => ({ only: value }) },
    navigator: {},
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    URL,
    URLSearchParams
  };
  vm.createContext(sandbox);
  if (indexedDBThrows) {
    // Some frames throw on a plain read of the indexedDB global. A vm global
    // never runs a getter, so the read goes through a with scope instead.
    sandbox.throwingScope = Object.defineProperty({}, 'indexedDB', {
      get() { throw new Error('SecurityError'); }
    });
    vm.runInContext('with (throwingScope) {\n' + source + '\n}', sandbox);
  } else {
    vm.runInContext(source, sandbox);
  }
  const strip = doc.createElement('div');
  const live = doc.createElement('div');
  doc.byId['notice-live'] = live;
  return { queue: win.PinalertQueue, strip, store: idb.data, idb: idb.control, submitted, toasts, doc };
}

async function settle() {
  for (let i = 0; i < 30; i++) {
    await new Promise((resolve) => setImmediate(resolve));
  }
}

function record(id, account, description, writtenAt, extra = {}) {
  return {
    request_id: id,
    account_id: account,
    payload: { category: 'flood', severity: 'low', description, latitude: 12.97, longitude: 77.59 },
    written_at: writtenAt,
    status: 'waiting',
    attempts: 0,
    next_try_at: 0,
    last_error: '',
    serious: false,
    emergency_number: '',
    sending_until: 0,
    ...extra
  };
}

// ----- the cases -----

await test('sends saved reports oldest first', async () => {
  // flush orders rows by written_at, oldest first (loadRows in queue.js).
  const env = loadQueue({
    records: [
      record('req-bbbbbbbbbbbbbbbb-2', '7', 'second report', '2026-10-02T00:00:00.000Z'),
      record('req-cccccccccccccccc-3', '7', 'third report', '2026-10-03T00:00:00.000Z'),
      record('req-aaaaaaaaaaaaaaaa-1', '7', 'first report', '2026-10-01T00:00:00.000Z')
    ]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.deepEqual(env.submitted.map((p) => p.description), ['first report', 'second report', 'third report']);
  assert.equal(env.store.size, 0);
});

await test('never discards a saved report for age or failure', async () => {
  const id = 'req-dddddddddddddddd-4';
  const old = new Date(Date.now() - 400 * DAY_MS).toISOString();
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', old)],
    answers: [
      () => Promise.reject({ queueable: true, status: 0 }),
      () => Promise.reject({ status: 422, field: 'description', fieldMessage: 'Say what is happening.' })
    ]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.store.get(id).status, 'waiting');
  assert.equal(env.store.get(id).attempts, 1);

  await env.queue.flush();
  await settle();
  assert.equal(env.store.get(id).status, 'rejected');
  assert.equal(env.store.get(id).attempts, 2);

  const sentBefore = env.submitted.length;
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, sentBefore, 'a rejected report is not sent again by itself');
  assert.ok(env.store.has(id), 'the rejected report is still saved');

  await env.queue.discard(id);
  assert.equal(env.store.has(id), false, 'only discard removes it');
});

await test("never sends, draws or counts another account's report", async () => {
  const mine = 'req-eeeeeeeeeeeeeeee-5';
  const theirs = 'req-ffffffffffffffff-6';
  const env = loadQueue({
    records: [
      record(mine, '7', 'my own saved report', '2026-10-01T00:00:00.000Z'),
      record(theirs, '8', 'someone elses saved report', '2026-10-01T00:00:01.000Z')
    ],
    answers: [() => Promise.reject({ queueable: true, status: 0 })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.deepEqual(env.submitted.map((p) => p.description), ['my own saved report']);
  assert.equal(env.strip.hidden, false);
  assert.equal(env.strip.querySelector('.queue-strip__count').textContent, '1 report saved on this device.');
  assert.ok(!env.strip.textContent.includes('someone elses saved report'));
  assert.equal(env.store.get(theirs).status, 'waiting');
  assert.equal(env.store.get(theirs).attempts, 0);
});

await test('draws Edit and resend as a link only for a single slash path', async () => {
  const id = 'req-gggggggggggggggg-7';
  const rejected = () => record(id, '7', 'rejected report', '2026-10-01T00:00:00.000Z', {
    status: 'rejected', last_error: 'Say what is happening.'
  });
  const hasEditControl = (strip) => strip.descendants().some((el) => el.textContent === 'Edit and resend');

  const good = loadQueue({ records: [rejected()] });
  good.queue.mount({
    strip: good.strip,
    accountId: '7',
    headingLevel: 3,
    resendHref: (r) => '/low?resend=' + encodeURIComponent(r.request_id)
  });
  await settle();
  const link = good.strip.querySelector('a');
  assert.ok(link, 'a link is drawn');
  assert.equal(link.textContent, 'Edit and resend');
  assert.equal(link.getAttribute('href'), '/low?resend=' + id);

  const unsafe = ['javascript:alert(1)', '//evil.example/x', '/\\evil.example/x'];
  for (const value of unsafe) {
    const env = loadQueue({ records: [rejected()] });
    env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3, resendHref: () => value });
    await settle();
    assert.equal(hasEditControl(env.strip), false, 'no control for ' + value);
    assert.ok(env.strip.querySelector('.queue-strip__discard'), 'Discard report is still drawn for ' + value);
  }
});

await test('keeps reports in memory when reading the indexedDB global throws (IN-W04)', async () => {
  const id = 'req-hhhhhhhhhhhhhhhh-8';
  const env = loadQueue({
    indexedDBThrows: true,
    answers: [() => Promise.reject({ queueable: true, status: 0 })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const persisted = await env.queue.save(record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z'));
  await settle();
  assert.equal(persisted, false, 'the report is kept in memory');
  assert.equal(env.strip.hidden, false, 'the strip is drawn');
  assert.equal(env.strip.querySelector('.queue-strip__help').textContent,
    "This browser can't keep reports for later. Keep this page open until they send.");
});

await test('reads again on a new connection after a failed read (WR-W06)', async () => {
  const id = 'req-iiiiiiiiiiiiiiii-9';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')],
    answers: [() => Promise.reject({ queueable: true, status: 0 })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 1);
  env.idb.failReads = 1;
  await env.queue.flush();
  await settle();
  assert.equal(env.idb.opens, 2, 'the store was opened again');
  assert.equal(env.submitted.length, 2, 'the report was sent after the second read');
  assert.equal(env.store.size, 0);
});

await test('keeps saved reports on the strip and unsent while the store is unreadable (WR-W06)', async () => {
  const first = 'req-jjjjjjjjjjjjjjjj-10';
  const second = 'req-kkkkkkkkkkkkkkkk-11';
  const env = loadQueue({
    records: [
      record(first, '7', 'first saved report', '2026-10-01T00:00:00.000Z'),
      record(second, '7', 'second saved report', '2026-10-02T00:00:00.000Z')
    ],
    answers: [() => Promise.reject({ queueable: true, status: 0 })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.deepEqual(env.submitted.map((p) => p.description), ['first saved report']);

  // The browser drops the connection and it cannot be opened again for now.
  env.idb.failOpen = true;
  env.idb.closeAll();
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 1, 'nothing is sent from the last good read');
  assert.equal(env.strip.hidden, false, 'the strip stays');
  assert.equal(env.strip.querySelector('.queue-strip__count').textContent, '2 reports saved on this device.');
  assert.equal(env.strip.querySelector('.queue-strip__help').textContent,
    "This page can't read the saved reports right now. Reload the page to send or discard them.");
  const sendBtn = env.strip.descendants().find((el) => el.id === 'queue-send-now');
  assert.equal(sendBtn.getAttribute('aria-disabled'), 'true');

  // A delete could not reach the store and the row would come back on a
  // reload, so no row offers Discard and a discard is refused.
  assert.equal(env.strip.querySelector('.queue-strip__discard'), null, 'no Discard while unreadable');
  assert.equal(await env.queue.discard(first), false);
  await settle();
  assert.equal(env.strip.querySelector('.queue-strip__count').textContent, '2 reports saved on this device.');
  assert.ok(env.store.has(first), 'the report is still on the device');

  // The store opens again: Discard is back and works, and the next pass sends
  // the other report.
  env.idb.failOpen = false;
  assert.equal(await env.queue.discard(first), true);
  await settle();
  assert.equal(env.store.has(first), false);
  await env.queue.flush();
  await settle();
  assert.deepEqual(env.submitted.slice(1).map((p) => p.description), ['second saved report']);
  assert.equal(env.store.size, 0);
  assert.equal(env.strip.hidden, true);
});

await test('a save that meets a silently dead connection opens a new one (WR-W06)', async () => {
  const id = 'req-qqqqqqqqqqqqqqqq-17';
  const env = loadQueue({ answers: [() => Promise.reject({ queueable: true, status: 0 })] });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  env.idb.closeAll(true);
  const persisted = await env.queue.save(record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z'));
  await settle();
  assert.equal(persisted, true, 'the report reached the store on a new connection');
  assert.ok(env.store.has(id));
  assert.equal(env.strip.querySelector('.queue-strip__help').textContent,
    'Reports send by themselves when you have a signal and this page is open.');
});

await test('an open Discard question holds its row: no send, no rebuild, focus stays (WR-W07)', async () => {
  const id = 'req-llllllllllllllll-12';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')],
    answers: [() => Promise.reject({ queueable: true, status: 0 })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 1);

  const rowEl = env.strip.querySelector('.queue-strip__row');
  const discardBtn = rowEl.querySelector('.queue-strip__discard');
  discardBtn.handlers.click.forEach((fn) => fn());
  const group = rowEl.querySelector('.queue-strip__confirm');
  assert.ok(group, 'the question is open');
  const keep = group.children.find((el) => el.textContent === 'Keep saved report');
  assert.equal(env.doc.activeElement, keep);

  // The signal returns while the person is deciding.
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 1, 'the report is not sent while the question is open');
  assert.equal(group.parentNode, rowEl, 'the question is still on the row');
  assert.ok(rowEl.contains(keep), 'Keep saved report is still on the row');
  assert.equal(env.doc.activeElement, keep, 'focus stays on Keep saved report');

  // Keep closes the question, focus goes to the row's Discard button and the
  // next pass sends the report.
  keep.handlers.click.forEach((fn) => fn());
  assert.equal(rowEl.querySelector('.queue-strip__confirm'), null);
  assert.equal(env.doc.activeElement, rowEl.querySelector('.queue-strip__discard'));
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 2);
  assert.equal(env.store.size, 0);
});

await test('a 429 with a wait of 0 seconds waits at least 5 seconds (IN-W07)', async () => {
  const id = 'req-mmmmmmmmmmmmmmmm-13';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')],
    answers: [() => Promise.reject({ status: 429, retryAfterSeconds: 0, serious: false, emergencyNumber: '' })]
  });
  const before = Date.now();
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const row = env.store.get(id);
  assert.equal(row.status, 'blocked');
  assert.ok(row.next_try_at >= before + 5000, 'next try is at least 5 seconds away, got ' + (row.next_try_at - before));

  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 1, 'no resend before the wait is over');
});

await test('names the saved account on each send and holds every row when the server refuses it (WR-W09)', async () => {
  const first = 'req-nnnnnnnnnnnnnnnn-14';
  const second = 'req-oooooooooooooooo-15';
  const otherAccount = () => Promise.reject({
    status: 409,
    field: 'expected_account_id',
    fieldMessage: 'This report was saved under a different account than the one signed in now. Sign in to that account to send it.'
  });
  const notSent = 'req-pppppppppppppppp-16';
  const env = loadQueue({
    records: [
      record(first, '7', 'first saved report', '2026-10-01T00:00:00.000Z'),
      record(second, '7', 'second saved report', '2026-10-02T00:00:00.000Z'),
      record(notSent, '7', 'not sent saved report', '2026-10-03T00:00:00.000Z', {
        status: 'rejected', last_error: 'Say what is happening.'
      })
    ],
    answers: [otherAccount]
  });
  // onEdit is given, so a Not sent row would draw Edit and resend.
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3, onEdit: () => {} });
  await settle();

  assert.equal(env.submitted.length, 1, 'the pass stops at the refusal');
  assert.equal(env.submitted[0].expected_account_id, '7', 'the post names the saved account, as digits');
  assert.equal(env.submitted[0].request_id, first);
  assert.equal(Object.hasOwn(env.store.get(first).payload, 'expected_account_id'), false,
    'the saved payload never holds it');

  for (const id of [first, second, notSent]) {
    assert.ok(env.store.has(id), 'the report stays saved');
    assert.equal(env.store.get(id).status, 'other_account');
  }
  const statusLines = env.strip.descendants().filter((el) => el.className.split(/\s+/).includes('queue-strip__status'));
  assert.equal(statusLines.length, 3);
  statusLines.forEach((el) => {
    assert.equal(el.textContent, 'This report was saved under a different account than the one signed in now. ' +
      'Sign in to that account to send it. It stays saved on this device.');
  });
  const discards = env.strip.descendants().filter((el) => el.className.split(/\s+/).includes('queue-strip__discard'));
  assert.equal(discards.length, 3, 'Discard report stays on each row');
  assert.equal(env.strip.descendants().some((el) => el.textContent === 'Edit and resend'), false,
    'no row offers Edit and resend, which would post under the account signed in now');

  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 1, 'nothing more is sent from this page');

  // A page that mounts for the saved account again proves its session, so the
  // rows are tried again.
  const again = loadQueue({ records: [...env.store.values()] });
  again.queue.mount({ strip: again.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.deepEqual(again.submitted.map((p) => p.description),
    ['first saved report', 'second saved report', 'not sent saved report']);
  assert.equal(again.store.size, 0);
});

// openDiscardQuestion opens the Discard question on the row and returns a function
// that presses Yes, discard report.
function openDiscardQuestion(env) {
  const rowEl = env.strip.querySelector('.queue-strip__row');
  rowEl.querySelector('.queue-strip__discard').handlers.click.forEach((fn) => fn());
  const group = rowEl.querySelector('.queue-strip__confirm');
  assert.ok(group, 'the question is open');
  const yes = group.children.find((el) => el.textContent === 'Yes, discard report');
  return { rowEl, pressYes: () => yes.handlers.click.forEach((fn) => fn()) };
}

await test('a discard whose delete does not reach the store keeps the row and says so (WR-03)', async () => {
  const id = 'req-ssssssssssssssss-19';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z', {
      status: 'rejected', last_error: 'Say what is happening.'
    })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const question = openDiscardQuestion(env);
  env.idb.failDeletes = 1;
  question.pressYes();
  await settle();
  assert.ok(env.store.has(id), 'the report is still on the device');
  assert.ok(!env.toasts.includes('Saved report discarded.'), 'a failed delete must not be told as discarded');
  assert.deepEqual(env.toasts, ["This saved report couldn't be discarded. Try again."]);
  assert.equal(question.rowEl.querySelector('.queue-strip__confirm'), null, 'the question closes');
  assert.ok(env.strip.querySelector('.queue-strip__discard'), 'the row is drawn again with its Discard button');
  assert.equal(env.strip.querySelector('.queue-strip__count').textContent, '1 report saved on this device.');

  // The next try reaches the store.
  assert.equal(await env.queue.discard(id), true);
  assert.equal(env.store.has(id), false);
});

await test('a discard that meets a silently dead connection deletes on a new one (WR-03)', async () => {
  const id = 'req-tttttttttttttttt-20';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z', {
      status: 'rejected', last_error: 'Say what is happening.'
    })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  env.idb.deadOnWrite = 1;
  assert.equal(await env.queue.discard(id), true);
  assert.equal(env.store.has(id), false, 'the delete reached the store on a new connection');
  assert.equal(env.idb.opens, 2, 'the store was opened again');
});

await test('a sent report never comes back when the read after the send fails (WR-08)', async () => {
  const id = 'req-rrrrrrrrrrrrrrrr-18';
  let env = null;
  env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')],
    // Both reads after the delete fail: the first one and the one on a new
    // connection that readStored tries next.
    answers: [() => { env.idb.failReads = 2; return Promise.resolve({ id: 1 }); }]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 1);
  assert.equal(env.store.size, 0, 'the delete reached the store');
  assert.deepEqual(env.toasts, ['Report sent.']);
  assert.equal(env.strip.hidden, true, 'the sent report must not be drawn again from the last good read');
  assert.ok(!env.strip.textContent.includes('water over the road'));
});
