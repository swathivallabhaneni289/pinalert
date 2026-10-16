// queue_logic_test.mjs - runs the real web/static/js/queue.js in a Node sandbox
// and checks four things the Go source tests cannot: saved reports send oldest
// first, a saved report is never discarded for age or failure, another
// account's report is never sent, drawn or counted, and Edit and resend is
// drawn as a link only for a single slash path. The last cases run the real
// app.js too, with a post that never settles, as WebKit can leave an aborted
// one: the post fails at its 10 second limit as no answer, the send pass ends
// and the row stays saved, and a body that stalls after a 201 is never taken
// for no answer.
//
// Run it with: node --test web/queue_logic_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs in
// .github/workflows/ci.yml). It is not embedded in the server.
// The fakes below cover only what queue.js calls: a small in memory IndexedDB,
// a minimal element model, timers that never fire on their own, and a fake
// Pinalert whose submitReport answers from a script per case, or the real
// Pinalert of app.js with a fetch the case answers by hand.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'queue.js'), 'utf8');
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');

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
  hasAttribute(name) { return Object.hasOwn(this.attrs, name); }
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
    getElementById(id) { return doc.byId[id] || null; },
    querySelector() { return null; }
  };
  doc.documentElement = doc.createElement('html');
  return doc;
}

// ----- the real app.js, for the hung post cases -----

class FakeSignal {
  constructor() { this.aborted = false; this.listeners = []; }
  addEventListener(type, fn) { if (type === 'abort') { this.listeners.push(fn); } }
  fire() {
    if (this.aborted) { return; }
    this.aborted = true;
    this.listeners.forEach((fn) => fn());
  }
}

class FakeAbortController {
  constructor() { this.signal = new FakeSignal(); }
  abort() { this.signal.fire(); }
}

// installRealApp evaluates app.js in the sandbox and makes its Pinalert the
// one queue.js uses. Every request goes to the returned list for the case to
// answer by hand. abortRejects says whether an abort ends the request's
// promise, as Chromium does, or leaves it unsettled, as WebKit sometimes
// does. nativeTimeout takes AbortController away and gives a native timeout
// signal that never fires, so only the page's own timer is left.
function installRealApp(sandbox, { abortRejects = false, nativeTimeout = false } = {}) {
  const requests = [];
  sandbox.fetch = (target, init) => new Promise((resolve, reject) => {
    const signal = (init && init.signal) || (target && target.signal);
    const req = {
      url: typeof target === 'string' ? target : target.url,
      method: (init && init.method) || 'GET',
      body: init && init.body,
      resolve,
      reject,
      aborted: false
    };
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
  sandbox.Request = function Request(target, init) { this.url = target; this.signal = init && init.signal; };
  if (nativeTimeout) {
    sandbox.AbortSignal = { timeout: () => new FakeSignal() };
  } else {
    sandbox.AbortController = FakeAbortController;
  }
  vm.runInContext(appSource, sandbox);
  sandbox.Pinalert = sandbox.window.Pinalert;
  return requests;
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

function loadQueue({ records = [], answers = [], indexedDBThrows = false, lockHeldElsewhere = false, withChannel = false, realApp = null }) {
  const doc = makeDocument();
  const idb = makeIndexedDB(records);
  const submitted = [];
  const toasts = [];
  // Timers never fire on their own; a case fires one by calling its fn.
  const timers = [];
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
    setTimeout(fn, delay) { timers.push({ fn, delay, cleared: false }); return timers.length; },
    clearTimeout(handle) { if (timers[handle - 1]) { timers[handle - 1].cleared = true; } },
    location: { href: 'http://127.0.0.1/profile' },
    history: { state: null, replaceState() {} }
  };
  const sandbox = {
    window: win,
    document: doc,
    Pinalert: pinalert,
    indexedDB: idb.indexedDB,
    IDBKeyRange: { only: (value) => ({ only: value }) },
    // lockHeldElsewhere stands for another tab holding the flush lock, so no
    // pass of this page ever runs.
    navigator: lockHeldElsewhere
      ? { locks: { request: (name, options, fn) => Promise.resolve().then(() => fn(null)) } }
      : {},
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    URL,
    URLSearchParams
  };
  // withChannel gives the page a BroadcastChannel. channels holds every one
  // the page opened and posted keeps what it posted; a case delivers another
  // tab's message by calling a channel's onmessage.
  const channels = [];
  const posted = [];
  if (withChannel) {
    sandbox.BroadcastChannel = class {
      constructor(name) { this.name = name; this.onmessage = null; channels.push(this); }
      postMessage(data) { posted.push(structuredClone(data)); }
    };
  }
  vm.createContext(sandbox);
  // realApp swaps the fake Pinalert for the real one of app.js.
  const requests = realApp ? installRealApp(sandbox, realApp) : [];
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
  // otherTab delivers another tab's channel message to this page.
  const otherTab = (accountId) => channels.forEach((c) => { if (c.onmessage) { c.onmessage({ data: { account_id: accountId } }); } });
  return { queue: win.PinalertQueue, strip, store: idb.data, idb: idb.control, submitted, toasts, doc, timers, live, posted, otherTab, requests, Pinalert: sandbox.Pinalert };
}

// wakeTimers returns the wake timers still set, made from index from on. The
// announcement timer (50 ms) is left out.
function wakeTimers(env, from = 0) {
  return env.timers.slice(from).filter((t) => !t.cleared && t.delay >= 1000);
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

await test('names the saved account on each send and holds the rows that send by themselves when the server refuses it (WR-W09, WR-05)', async () => {
  const first = 'req-nnnnnnnnnnnnnnnn-14';
  const second = 'req-oooooooooooooooo-15';
  const otherAccount = () => Promise.reject({
    status: 409,
    field: 'expected_account_id',
    fieldMessage: 'This report was saved under a different account than the one signed in now. Sign in to that account to send it.'
  });
  const notSent = 'req-pppppppppppppppp-16';
  const later = Date.now() + 10 * 60 * 1000;
  const env = loadQueue({
    records: [
      record(first, '7', 'first saved report', '2026-10-01T00:00:00.000Z'),
      record(second, '7', 'second saved report', '2026-10-02T00:00:00.000Z', {
        status: 'blocked', next_try_at: later
      }),
      record(notSent, '7', 'not sent saved report', '2026-10-03T00:00:00.000Z', {
        status: 'rejected', last_error: 'Say what is happening.'
      })
    ],
    answers: [otherAccount]
  });
  // onEdit is given, so a Not sent row draws Edit and resend.
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3, onEdit: () => {} });
  await settle();

  assert.equal(env.submitted.length, 1, 'the pass stops at the refusal');
  assert.equal(env.submitted[0].expected_account_id, '7', 'the post names the saved account, as digits');
  assert.equal(env.submitted[0].request_id, first);
  assert.equal(Object.hasOwn(env.store.get(first).payload, 'expected_account_id'), false,
    'the saved payload never holds it');

  // Only the rows that would send by themselves are held (WR-05); the Not
  // sent row keeps its status and its reason.
  for (const id of [first, second]) {
    assert.ok(env.store.has(id), 'the report stays saved');
    assert.equal(env.store.get(id).status, 'other_account');
  }
  assert.equal(env.store.get(notSent).status, 'rejected');
  assert.equal(env.store.get(notSent).last_error, 'Say what is happening.');
  const statusLines = env.strip.descendants().filter((el) => el.className.split(/\s+/).includes('queue-strip__status'));
  assert.deepEqual(statusLines.map((el) => el.textContent), [
    'This report was saved under a different account than the one signed in now. ' +
      'Sign in to that account to send it. It stays saved on this device.',
    'This report was saved under a different account than the one signed in now. ' +
      'Sign in to that account to send it. It stays saved on this device.',
    'Not sent. Say what is happening.'
  ]);
  const discards = env.strip.descendants().filter((el) => el.className.split(/\s+/).includes('queue-strip__discard'));
  assert.equal(discards.length, 3, 'Discard report stays on each row');
  assert.equal(env.strip.descendants().some((el) => el.textContent === 'Edit and resend'), false,
    'while another account is signed in no row offers Edit and resend');

  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 1, 'nothing more is sent from this page');

  // A page that mounts for the saved account again proves its session, so
  // each held row goes back to the status it had: the waiting one is sent,
  // the blocked one keeps its wait and the Not sent one waits for the person.
  // A held row saved by older code, with no earlier status, becomes Not sent
  // rather than sending by itself.
  const legacy = 'req-yyyyyyyyyyyyyyyy-25';
  const again = loadQueue({
    records: [...env.store.values(),
      record(legacy, '7', 'held by older code', '2026-10-04T00:00:00.000Z', { status: 'other_account' })]
  });
  again.queue.mount({ strip: again.strip, accountId: '7', headingLevel: 3, onEdit: () => {} });
  await settle();
  assert.deepEqual(again.submitted.map((p) => p.description), ['first saved report']);
  assert.equal(again.store.has(first), false);
  assert.equal(again.store.get(second).status, 'blocked');
  assert.equal(again.store.get(second).next_try_at, later);
  assert.equal(Object.hasOwn(again.store.get(second), 'held_from'), false);
  assert.equal(again.store.get(notSent).status, 'rejected');
  assert.equal(again.store.get(legacy).status, 'rejected');
  assert.equal(again.store.get(legacy).last_error, 'Something went wrong. Try again.');
  assert.ok(again.strip.descendants().some((el) => el.textContent === 'Edit and resend'),
    'Edit and resend is back for the right account');
});

const OTHER_ACCOUNT_LINE = 'This report was saved under a different account than the one signed in now. ' +
  'Sign in to that account to send it. It stays saved on this device.';

function statusLines(env) {
  return env.strip.descendants()
    .filter((el) => el.className.split(/\s+/).includes('queue-strip__status'))
    .map((el) => el.textContent);
}

// announcements fires the announcement timers not fired yet and returns what
// each one said.
function announcements(env) {
  return env.timers.filter((t) => t.delay === 50 && !t.fired).map((t) => {
    t.fired = true;
    t.fn();
    return env.live.textContent;
  });
}

await test("another tab's account holds the rows exactly as the 409 does, and this account's tab puts them back (F-P4-8)", async () => {
  const waiting = 'req-zzzzzzzzzzzzzzzz-30';
  const blocked = 'req-zzzzzzzzzzzzzzzz-31';
  const notSent = 'req-zzzzzzzzzzzzzzzz-32';
  const noAnswer = 'req-zzzzzzzzzzzzzzzz-33';
  const later = Date.now() + 10 * 60 * 1000;
  // The post of the waiting row is blocked, as in the harness: it never
  // answers while the other account signs in.
  const env = loadQueue({
    withChannel: true,
    lockHeldElsewhere: true,
    records: [
      record(waiting, '7', 'waiting saved report', '2026-10-01T00:00:00.000Z'),
      record(blocked, '7', 'blocked saved report', '2026-10-02T00:00:00.000Z', { status: 'blocked', next_try_at: later }),
      record(notSent, '7', 'not sent saved report', '2026-10-03T00:00:00.000Z', {
        status: 'rejected', last_error: 'Say what is happening.'
      }),
      record(noAnswer, '7', 'no answer saved report', '2026-10-04T00:00:00.000Z', { status: 'no_answer' })
    ]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3, onEdit: () => {} });
  await settle();
  assert.deepEqual(env.posted, [{ account_id: '7' }], 'the page tells the other tabs its account');
  assert.equal(env.submitted.length, 0, 'another tab holds the flush lock, so nothing is sent here');
  assert.deepEqual(statusLines(env), [
    'Waiting for a signal.',
    'Waiting to send. It will send by itself in 1 minute.',
    'Not sent. Say what is happening.',
    "Pinalert didn't answer. Trying again."
  ]);
  assert.ok(env.strip.descendants().some((el) => el.textContent === 'Edit and resend'));
  announcements(env);

  // Another tab opens for account 8.
  env.otherTab('8');
  await settle();
  assert.deepEqual(statusLines(env), [OTHER_ACCOUNT_LINE, OTHER_ACCOUNT_LINE, 'Not sent. Say what is happening.',
    OTHER_ACCOUNT_LINE], 'each row that sends by itself reads as the 409 makes it read');
  for (const [id, was] of [[waiting, 'waiting'], [blocked, 'blocked'], [noAnswer, 'no_answer']]) {
    assert.equal(env.store.get(id).status, 'other_account', id + ' is held in the store');
    assert.equal(env.store.get(id).held_from, was, id + ' keeps its earlier status');
  }
  assert.equal(env.store.get(notSent).status, 'rejected');
  assert.equal(env.store.get(notSent).last_error, 'Say what is happening.');
  assert.equal(env.strip.descendants().some((el) => el.textContent === 'Edit and resend'), false,
    'no row offers Edit and resend');
  const discards = env.strip.descendants().filter((el) => el.className.split(/\s+/).includes('queue-strip__discard'));
  assert.equal(discards.length, 4, 'Discard report stays on each row');
  const sendBtn = env.strip.descendants().find((el) => el.id === 'queue-send-now');
  assert.equal(sendBtn.getAttribute('aria-disabled'), 'true', 'Send now does nothing');
  assert.deepEqual(announcements(env), ['Report not sent. It was saved under a different account.']);

  // A second message changes nothing and says nothing again.
  env.otherTab('8');
  await settle();
  assert.equal(env.store.get(waiting).held_from, 'waiting');
  assert.deepEqual(announcements(env), []);

  // A tab opens for account 7 again: every row is back exactly.
  env.otherTab('7');
  await settle();
  assert.equal(env.store.get(waiting).status, 'waiting');
  assert.equal(env.store.get(blocked).status, 'blocked');
  assert.equal(env.store.get(blocked).next_try_at, later);
  assert.equal(env.store.get(noAnswer).status, 'no_answer');
  assert.equal(env.store.get(notSent).status, 'rejected');
  for (const id of [waiting, blocked, noAnswer]) {
    assert.equal(Object.hasOwn(env.store.get(id), 'held_from'), false, id + ' has no held_from left');
  }
  assert.deepEqual(statusLines(env), [
    'Waiting for a signal.',
    'Waiting to send. It will send by itself in 1 minute.',
    'Not sent. Say what is happening.',
    "Pinalert didn't answer. Trying again."
  ]);
  assert.ok(env.strip.descendants().some((el) => el.textContent === 'Edit and resend'),
    'Edit and resend is back for the right account');
  assert.equal(sendBtn.getAttribute('aria-disabled'), null, 'Send now works again');
});

await test('the right account back in another tab sends again (F-P4-8)', async () => {
  const id = 'req-zzzzzzzzzzzzzzzz-34';
  const env = loadQueue({
    withChannel: true,
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')],
    answers: [() => Promise.reject({ queueable: true, status: 0 })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 1);
  env.otherTab('8');
  await settle();
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 1, 'nothing is sent while another account is signed in');
  env.otherTab('7');
  await settle();
  assert.equal(env.submitted.length, 2, 'the held report is sent once the right account is back');
  assert.equal(env.store.size, 0);
});

await test("a send out when another tab's account arrives keeps its row held (F-P4-8)", async () => {
  const id = 'req-zzzzzzzzzzzzzzzz-35';
  let fail = null;
  const env = loadQueue({
    withChannel: true,
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')],
    answers: [() => new Promise((resolve, reject) => { fail = reject; })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 1, 'the post is out');
  env.otherTab('8');
  await settle();
  assert.equal(env.store.get(id).status, 'other_account');
  // The blocked post then fails with no answer.
  fail({ queueable: true, status: 0 });
  await settle();
  assert.equal(env.store.get(id).status, 'other_account', 'the late failure does not undo the hold');
  assert.equal(env.store.get(id).held_from, 'waiting');
  assert.deepEqual(statusLines(env), [OTHER_ACCOUNT_LINE]);

  // A report saved now is held too.
  const fresh = 'req-zzzzzzzzzzzzzzzz-36';
  await env.queue.save(record(fresh, '7', 'second report', '2026-10-02T00:00:00.000Z'));
  await settle();
  assert.equal(env.store.get(fresh).status, 'other_account');
  assert.equal(env.store.get(fresh).held_from, 'waiting');
  assert.deepEqual(statusLines(env), [OTHER_ACCOUNT_LINE, OTHER_ACCOUNT_LINE]);
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

await test('a report another tab is sending right now is never discarded (WR-04)', async () => {
  const id = 'req-uuuuuuuuuuuuuuuu-21';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z', {
      status: 'rejected', last_error: 'Say what is happening.'
    })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const question = openDiscardQuestion(env);
  // Another tab takes the row's lease while the question is open.
  const leased = env.store.get(id);
  leased.sending_until = Date.now() + 15000;
  env.store.set(id, leased);
  question.pressYes();
  await settle();
  assert.ok(env.store.has(id), 'the report being sent stays');
  assert.deepEqual(env.toasts, ["This report is being sent right now, so it can't be discarded."]);
  assert.equal(question.rowEl.querySelector('.queue-strip__confirm'), null, 'the question closes');
  assert.equal(env.strip.querySelector('.queue-strip__status').textContent, 'Sending.');
  assert.equal(await env.queue.discard(id), false);
});

await test('no wake timer for a due row that is stale, and no reopen on every pass (WR-07)', async () => {
  const id = 'req-vvvvvvvvvvvvvvvv-22';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z', {
      status: 'blocked', next_try_at: Date.now() + 20
    })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 0, 'not due yet at mount');
  await new Promise((resolve) => setTimeout(resolve, 40));

  // The store cannot be read now, so the due row is shown from the last good
  // read and is never sent.
  env.idb.failOpen = true;
  env.idb.closeAll();
  const mark = env.timers.length;
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 0);
  assert.deepEqual(wakeTimers(env, mark), [], 'no wake timer for a row that cannot be sent');
  const opens = env.idb.opens;

  // A pass right after does not open the store again.
  await env.queue.flush();
  await env.queue.flush();
  await settle();
  assert.equal(env.idb.opens, opens, 'the reopen waits before it is tried again');
});

await test('no wake timer while the Discard question of the due row is open (WR-07)', async () => {
  const id = 'req-wwwwwwwwwwwwwwww-23';
  const env = loadQueue({
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z', {
      status: 'blocked', next_try_at: Date.now() + 20
    })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const rowEl = env.strip.querySelector('.queue-strip__row');
  rowEl.querySelector('.queue-strip__discard').handlers.click.forEach((fn) => fn());
  const keep = rowEl.querySelector('.queue-strip__confirm').children.find((el) => el.textContent === 'Keep saved report');
  await new Promise((resolve) => setTimeout(resolve, 40));
  const mark = env.timers.length;
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.length, 0, 'not sent while the question is open');
  assert.deepEqual(wakeTimers(env, mark), [], 'no wake timer while the question is open');

  // Closing the question sets the wake again.
  const closeMark = env.timers.length;
  keep.handlers.click.forEach((fn) => fn());
  assert.equal(wakeTimers(env, closeMark).length, 1);
});

await test('a wake that sends nothing waits twice as long each time, up to a minute (WR-07)', async () => {
  const id = 'req-xxxxxxxxxxxxxxxx-24';
  const env = loadQueue({
    lockHeldElsewhere: true,
    records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z', {
      status: 'blocked', next_try_at: Date.now() - 1000
    })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const delays = [];
  for (let i = 0; i < 9; i++) {
    const live = wakeTimers(env);
    assert.equal(live.length, 1, 'one wake timer at a time');
    delays.push(live[0].delay);
    live[0].cleared = true;
    live[0].fn();
    await settle();
  }
  assert.equal(env.submitted.length, 0);
  assert.deepEqual(delays, [1000, 2000, 4000, 8000, 16000, 32000, 60000, 60000, 60000]);
});

await test('a Not sent row sent by hand while signed out waits for a sign in, and the others keep Edit (WR-05)', async () => {
  const mine = 'req-abababababababab-27';
  const other = 'req-cdcdcdcdcdcdcdcd-28';
  const env = loadQueue({
    records: [
      record(mine, '7', 'first not sent report', '2026-10-01T00:00:00.000Z', { status: 'rejected', last_error: 'Say what is happening.' }),
      record(other, '7', 'second not sent report', '2026-10-02T00:00:00.000Z', { status: 'rejected', last_error: 'Say what is happening.' })
    ],
    answers: [() => Promise.reject({ status: 401, fieldMessage: 'Sign in first.' })]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3, onEdit: () => {} });
  await settle();
  const sendBtn = env.strip.descendants().find((el) => el.id === 'queue-send-now');
  sendBtn.handlers.click.forEach((fn) => fn());
  await settle();
  assert.equal(env.submitted.length, 1, 'the pass stops at the 401');
  assert.equal(env.store.get(mine).status, 'signed_out', 'the row that drew the 401 waits for a sign in');
  assert.equal(env.store.get(other).status, 'rejected', 'a Not sent row not sent keeps its status');
  assert.ok(env.strip.descendants().some((el) => el.textContent === 'Sign in again'), 'the Sign in again link is drawn');
  assert.ok(env.strip.descendants().some((el) => el.textContent === 'Edit and resend'),
    'a 401 does not take Edit and resend away from the other Not sent row');
});

await test('a saved payload never keeps the account a resend named (WR-02)', async () => {
  const id = 'req-zzzzzzzzzzzzzzzz-26';
  const env = loadQueue({ answers: [() => Promise.reject({ queueable: true, status: 0 })] });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const saved = record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z');
  saved.payload.expected_account_id = '8';
  await env.queue.save(saved);
  await settle();
  assert.equal(Object.hasOwn(env.store.get(id).payload, 'expected_account_id'), false);
  await env.queue.flush();
  await settle();
  assert.equal(env.submitted.at(-1).expected_account_id, '7', 'a send names the account the row is bound to');
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

// ----- a post that never settles (the WebKit hung post) -----

// The three ways a browser can treat the page's abort: WebKit can leave the
// aborted request's promise unsettled, Chromium rejects it, and a browser
// with only a native timeout signal leaves just the page's own timer.
const HUNG_MODES = [
  { name: 'abort leaves the request unsettled', abortRejects: false },
  { name: 'abort rejects the request', abortRejects: true },
  { name: 'native timeout signal only', nativeTimeout: true }
];

// limitTimers returns the 10 second time limits still set.
function limitTimers(env) {
  return env.timers.filter((t) => !t.cleared && t.delay === 10000);
}

const created = (body) => ({ ok: true, status: 201, headers: { get: () => null }, json: () => Promise.resolve(body) });
const listAnswer = () => ({ ok: true, status: 200, headers: { get: () => null }, json: () => Promise.resolve({ reports: [] }) });

for (const mode of HUNG_MODES) {
  const how = ' (' + mode.name + ')';

  await test('a post that never settles fails at 10 seconds as no answer' + how, async () => {
    const env = loadQueue({ realApp: mode });
    let outcome = null;
    env.Pinalert.submitReport({ description: 'water over the road' }).then(
      (report) => { outcome = { report }; },
      (err) => { outcome = { err }; });
    await settle();
    assert.equal(env.requests.length, 1);
    assert.equal(env.requests[0].method, 'POST');
    assert.equal(env.requests[0].url, '/api/reports');
    const limits = limitTimers(env);
    assert.equal(limits.length, 1, 'the post has one 10 second limit');
    assert.equal(outcome, null, 'nothing before the limit');

    limits[0].fn();
    await settle();
    assert.ok(outcome && outcome.err, 'the post failed at its limit');
    assert.equal(outcome.err.status, 0);
    assert.equal(outcome.err.queueable, true, 'the form saves the report on the device');
    assert.equal(outcome.err.field, 'body');
    assert.equal(outcome.err.fieldMessage, 'Something went wrong. Try again.');
    assert.equal(env.requests[0].aborted, !mode.nativeTimeout, 'the page aborts its own controller at the limit');
    assert.equal(env.requests.length, 1, 'no list refetch after a post that got no answer');
  });

  await test('a 201 whose body never ends is not taken for no answer' + how, async () => {
    const env = loadQueue({ realApp: mode });
    let outcome = null;
    env.Pinalert.submitReport({ description: 'water over the road' }).then(
      (report) => { outcome = { report }; },
      (err) => { outcome = { err }; });
    await settle();
    env.requests[0].resolve({ ok: true, status: 201, headers: { get: () => null }, json: () => new Promise(() => {}) });
    await settle();
    assert.equal(outcome, null, 'the body read waits up to the limit');
    limitTimers(env)[0].fn();
    await settle();
    // The body fallback, as an aborted body read always gave: the post went
    // through, so the list is read again and nothing is saved on the device.
    assert.equal(env.requests.length, 2, 'the list is read again after the post');
    assert.equal(env.requests[1].method, 'GET');
    env.requests[1].resolve(listAnswer());
    await settle();
    assert.ok(outcome && !outcome.err, 'the post resolves, it does not fail');
    assert.equal(outcome.report, null);
  });

  await test('a queued send that never settles ends the pass at the limit and keeps the row' + how, async () => {
    const id = 'req-hhhhhhhhhhhhhhhh-30';
    const env = loadQueue({ records: [record(id, '7', 'water over the road', '2026-10-01T00:00:00.000Z')], realApp: mode });
    env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
    await settle();
    assert.equal(env.requests.length, 1, 'the saved report is posted');
    assert.equal(env.requests[0].method, 'POST');
    assert.ok(env.store.get(id).sending_until > 0, 'the row is leased while its post is out');

    limitTimers(env)[0].fn();
    await settle();
    assert.ok(env.store.has(id), 'the row stays saved');
    assert.equal(env.store.get(id).status, 'waiting');
    assert.equal(env.store.get(id).sending_until, 0, 'the lease is given back');
    assert.equal(env.store.get(id).attempts, 1);

    // The pass ended: a new pass sends the row again instead of returning the
    // old pass that waits for good.
    const again = env.queue.flush();
    await settle();
    assert.equal(env.requests.length, 2, 'a later pass sends the row again');
    env.requests[1].resolve(created({ report: { id: 41 } }));
    await settle();
    assert.equal(env.requests.length, 3, 'the list is read again after the post');
    env.requests[2].resolve(listAnswer());
    await again;
    await settle();
    assert.equal(env.store.has(id), false, 'the sent report leaves the device');
  });
}

// Phase 3.4 plan 12 (D-24b, T-34-54): the happening pick of a Critical report
// is stored in the saved payload and sent unchanged; an entry saved before the
// field existed has no such key and still sends, with no key added.
await test('a saved Critical report with a happening pick is sent with the same value, and one without it is sent without the key', async () => {
  const withPick = 'req-iiiiiiiiiiiiiiii-31';
  const withoutPick = 'req-jjjjjjjjjjjjjjjj-32';
  const env = loadQueue({
    records: [
      record(withPick, '7', 'people on the roof', '2026-10-01T00:00:00.000Z', {
        payload: { category: 'flood', severity: 'critical', description: 'people on the roof', latitude: 12.97, longitude: 77.59, happening: 'people_trapped' }
      }),
      record(withoutPick, '7', 'an older saved report', '2026-10-02T00:00:00.000Z', {
        payload: { category: 'flood', severity: 'critical', description: 'an older saved report', latitude: 12.97, longitude: 77.59 }
      })
    ]
  });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  assert.equal(env.submitted.length, 2, 'both saved reports are sent');
  assert.equal(env.submitted[0].description, 'people on the roof');
  assert.equal(env.submitted[0].happening, 'people_trapped', 'the pick is resent with the same value');
  assert.equal(env.submitted[1].description, 'an older saved report');
  assert.equal(Object.hasOwn(env.submitted[1], 'happening'), false, 'an entry without a pick is sent without the key');
  assert.equal(env.store.size, 0);
});

await test('a report saved with a happening pick keeps it in the stored payload', async () => {
  const env = loadQueue({ records: [] });
  env.queue.mount({ strip: env.strip, accountId: '7', headingLevel: 3 });
  await settle();
  const id = 'req-kkkkkkkkkkkkkkkk-33';
  await env.queue.save({
    request_id: id,
    payload: { category: 'flood', severity: 'critical', description: 'water rising fast', latitude: 12.97, longitude: 77.59, happening: 'water_rising' },
    written_at: '2026-10-03T00:00:00.000Z',
    status: 'blocked',
    next_try_at: Date.now() + 60000
  });
  assert.equal(env.store.get(id).payload.happening, 'water_rising');
});
