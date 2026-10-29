// installed_ask_test.mjs - runs the real web/static/js/installed-ask.js in a Node
// sandbox and checks the box that asks about alerts once inside the installed
// iPhone or iPad app (Phase 7.1 plan 13c, D-17, D-18, owner decision 2026-10-28):
//
//   - it opens only when the device is an installed Apple mobile app, notify.js
//     base state is maybe, the device state resolves to ready, the page has the
//     dialog, no other dialog is open, the page root carries no data-modal-open
//     and this device has not asked this account before;
//   - the account id is added to the list in pinalert.alertsAsked just before
//     the dialog opens, keeping the last 8, and not at all when it skips;
//   - Allow calls enable first, then saves the Reliable switch only when the box
//     is ticked; Not now and Escape ask the browser nothing and send nothing;
//   - it runs once per page load and builds no markup.
//
// Run it with: node --test web/installed_ask_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// Every case loads the script through load(), so a missing script fails each case
// by itself. The fakes cover only what installed-ask.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const scriptPath = path.join(here, 'static', 'js', 'installed-ask.js');
const source = fs.readFileSync(scriptPath, 'utf8');

const KEY = 'pinalert.alertsAsked';
const settle = () => new Promise((resolve) => setImmediate(resolve));

function fakeEl(id, doc, log, extra = {}) {
  return {
    id,
    doc,
    attrs: {},
    listeners: [],
    focused: 0,
    disabled: false,
    isConnected: true,
    addEventListener(type, fn) { this.listeners.push({ type, fn }); },
    setAttribute(name, value) { this.attrs[name] = String(value); },
    getAttribute(name) { return Object.hasOwn(this.attrs, name) ? this.attrs[name] : null; },
    removeAttribute(name) { delete this.attrs[name]; },
    focus() { this.focused += 1; doc.activeElement = this; log.push('focus:' + this.id); },
    click() { if (!this.disabled) { this.fire('click'); } },
    fire(type, e = {}) {
      const event = { type, prevented: 0, preventDefault() { this.prevented += 1; }, ...e };
      this.listeners.filter((l) => l.type === type).forEach((l) => l.fn(event));
      return event;
    },
    ...extra
  };
}

// load builds a page and evaluates installed-ask.js on it. The options:
//   appleMobile, installed   what PinalertNotify answers
//   base                     what baseState answers
//   device                   what deviceState resolves to ('reject' rejects)
//   enable                   'resolve' | 'refused' | 'reject' | 'throw' | 'pending'
//   patch                    200 | 500 | 'reject'
//   noDialog                 true for a page without the partial
//   showModal                false for a dialog with no showModal
//   otherDialog, modalOpen   another open dialog, data-modal-open on the root
//   stored                   what pinalert.alertsAsked holds before
//   readThrows, writeThrows  a storage that fails; noStorage for none at all
//   pushKey, account         what the dialog carries
//   globals                  true to put document, window.PinalertNotify and
//                            localStorage on the sandbox, so the script runs at load
//   locationAsking           Amended 2026-10-29 by the Phase 7.1 review fix
//                            (browser), WR-B05: what PinalertPosition.askingThisLoad
//                            answers (false: the browser's location question is not
//                            asked on this load; true: it is; null: not decided
//                            yet). A function is called on every look.
//   position                 a PinalertPosition of the case's own, or null for
//                            a page without one
//   timers                   true to run the script's own waits on a fake clock
//                            the case moves with m.advance(ms)
function load(o = {}) {
  const opts = {
    appleMobile: true, installed: true, base: 'maybe', device: 'ready', enable: 'resolve', patch: 200,
    noDialog: false, showModal: true, otherDialog: false, modalOpen: false, stored: null,
    readThrows: false, writeThrows: false, noStorage: false, pushKey: 'BPUSHKEY', account: '42',
    globals: false, notify: true, locationAsking: false, timers: false, ...o
  };
  const looks = [];
  const position = o.position !== undefined ? o.position : {
    askingThisLoad() {
      const answer = typeof opts.locationAsking === 'function' ? opts.locationAsking() : opts.locationAsking;
      looks.push(answer);
      return answer;
    }
  };
  let now = 0;
  let nextId = 1;
  const fakeTimers = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...fakeTimers.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) { break; }
      const [id, timer] = due;
      now = timer.at;
      fakeTimers.delete(id);
      timer.fn();
    }
    now = end;
  };
  const setTimer = (fn, ms) => { const id = nextId++; fakeTimers.set(id, { fn, at: now + ms }); return id; };
  const log = [];
  const doc = { activeElement: null };
  doc.documentElement = {
    attrs: opts.modalOpen ? { 'data-modal-open': 'true' } : {},
    hasAttribute(name) { return Object.hasOwn(this.attrs, name); }
  };
  const make = (id, extra) => fakeEl(id, doc, log, extra);

  const dialog = make('installed-alerts-ask', {
    open: false,
    showCalls: 0,
    closeCalls: 0,
    close() { if (this.open) { this.open = false; this.closeCalls += 1; log.push('close'); this.fire('close'); } }
  });
  if (opts.pushKey !== undefined) { dialog.attrs['data-push-key'] = opts.pushKey; }
  if (opts.account !== undefined) { dialog.attrs['data-account-id'] = opts.account; }
  if (opts.showModal) {
    dialog.showModal = function () { this.open = true; this.showCalls += 1; log.push('showModal'); };
  }
  const title = make('installed-alerts-ask-title');
  const later = make('installed-alerts-ask-later');
  const allow = make('installed-alerts-ask-allow');
  let ticked = !!opts.ticked;
  const reliable = make('installed-alerts-ask-reliable');
  Object.defineProperty(reliable, 'checked', {
    get() { log.push('read-checked'); return ticked; },
    set(v) { ticked = !!v; }
  });
  const els = opts.noDialog ? {} : {
    'installed-alerts-ask': dialog,
    'installed-alerts-ask-title': title,
    'installed-alerts-ask-later': later,
    'installed-alerts-ask-allow': allow,
    'installed-alerts-ask-reliable': reliable
  };
  doc.getElementById = (id) => els[id] || null;
  doc.querySelector = (sel) => {
    if (sel === 'dialog[open]') { return opts.otherDialog ? { id: 'other' } : (dialog.open ? dialog : null); }
    return null;
  };

  const store = new Map();
  if (opts.stored !== null) { store.set(KEY, String(opts.stored)); }
  const localStorage = {
    getItem(k) {
      if (opts.readThrows) { throw new Error('storage blocked'); }
      return store.has(k) ? store.get(k) : null;
    },
    setItem(k, v) {
      if (opts.writeThrows) { throw new Error('storage full'); }
      log.push('write:' + k + '=' + v);
      store.set(k, String(v));
    }
  };

  const enables = [];
  const deviceCalls = [];
  let releaseEnable = null;
  const notify = {
    isAppleMobile: () => opts.appleMobile,
    isInstalled: () => opts.installed,
    baseState: () => opts.base,
    deviceState(key) {
      deviceCalls.push(key);
      log.push('deviceState');
      if (opts.device === 'reject') { return Promise.reject(new Error('offline')); }
      return Promise.resolve(opts.device);
    },
    enable(key) {
      enables.push(key);
      log.push('enable');
      if (opts.enable === 'throw') { throw new Error('enable broke'); }
      if (opts.enable === 'reject') { return Promise.reject(new Error('no')); }
      if (opts.enable === 'refused') { return Promise.resolve({ ok: false, reason: 'refused' }); }
      if (opts.enable === 'pending') { return new Promise((resolve) => { releaseEnable = resolve; }); }
      return Promise.resolve({ ok: true });
    }
  };

  const fetches = [];
  const fetchFn = (target, init) => {
    log.push('fetch');
    fetches.push({
      url: String(target),
      method: init && init.method,
      credentials: init && init.credentials,
      headers: init && init.headers,
      body: init && typeof init.body === 'string' ? JSON.parse(init.body) : init && init.body
    });
    if (opts.patch === 'reject') { return Promise.reject(new Error('offline')); }
    return Promise.resolve({ ok: opts.patch < 400, status: opts.patch });
  };

  const sandbox = { JSON, AbortController, setTimeout, clearTimeout };
  sandbox.window = sandbox;
  if (opts.globals) {
    sandbox.document = doc;
    sandbox.fetch = fetchFn;
    if (opts.notify) { sandbox.PinalertNotify = notify; }
    if (!opts.noStorage) { sandbox.localStorage = localStorage; }
    if (position !== null) { sandbox.PinalertPosition = position; }
  }
  vm.runInNewContext(source, sandbox);
  const options = { doc, notify: opts.notify ? notify : undefined, storage: opts.noStorage ? null : localStorage, fetchFn, position };
  if (opts.timers) { options.setTimer = setTimer; }
  return {
    ask: sandbox.PinalertInstalledAsk,
    options,
    looks,
    advance,
    fakeTimers,
    dialog, title, later, allow, reliable, doc, log, fetches, enables, deviceCalls, store, make,
    releaseEnable: (value) => releaseEnable(value),
    escape: () => dialog.fire('cancel'),
    tick: (on) => { ticked = on; },
    // go runs the ask with the page above and waits for its decision.
    go: () => sandbox.PinalertInstalledAsk.run(options)
  };
}

const NAME = 'installed ask: ';

test(NAME + 'opens when every condition holds: key written first, title focused, deviceState given the dialog key, no request', async () => {
  const m = load();
  const result = await m.go();
  assert.equal(result, 'opened');
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.dialog.open, true);
  assert.deepEqual(m.deviceCalls, ['BPUSHKEY']);
  assert.equal(m.store.get(KEY), '["42"]');
  assert.deepEqual(m.log, ['deviceState', 'write:' + KEY + '=["42"]', 'showModal', 'focus:installed-alerts-ask-title']);
  assert.equal(m.title.focused, 1);
  assert.equal(m.allow.focused, 0);
  assert.equal(m.later.focused, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.enables.length, 0);
});

test(NAME + 'the exposed key is pinalert.alertsAsked', () => {
  const m = load({ noDialog: true });
  assert.equal(m.ask.ASKED_KEY, 'pinalert.alertsAsked');
});

// Each condition turned off in turn: the dialog never opens and enable is never
// called.
const OFF = [
  ['not Apple mobile', { appleMobile: false }],
  ['not installed', { installed: false }],
  ['a Mac Dock app (installed, not Apple mobile, base maybe)', { appleMobile: false, installed: true, base: 'maybe' }],
  ['base state needs-install', { base: 'needs-install' }],
  ['base state blocked', { base: 'blocked' }],
  ['base state unsupported', { base: 'unsupported' }],
  ['base state ready', { base: 'ready' }],
  ['device state on', { device: 'on' }],
  ['device state blocked', { device: 'blocked' }],
  ['device state unsupported', { device: 'unsupported' }],
  ['device state needs-install', { device: 'needs-install' }],
  ['a device state that rejects', { device: 'reject' }],
  ['no dialog on the page', { noDialog: true }],
  ['a dialog without showModal', { showModal: false }],
  ['another dialog open', { otherDialog: true }],
  ['data-modal-open on the page root', { modalOpen: true }],
  ['this account already in the list', { stored: JSON.stringify(['7', '42']) }],
  ['this account as an older single id', { stored: '42' }],
  ['no push key on the dialog', { pushKey: '' }],
  ['no account id on the dialog', { account: undefined }],
  ['an account id that is not an id', { account: 'abc' }],
  ['no notify.js', { notify: false }],
  ['no storage', { noStorage: true }]
];
for (const [label, extra] of OFF) {
  test(NAME + 'does not open with ' + label, async () => {
    const m = load(extra);
    const result = await m.go();
    assert.equal(result, 'skipped');
    assert.equal(m.dialog.showCalls, 0);
    assert.equal(m.enables.length, 0);
    assert.equal(m.fetches.length, 0);
  });
}

// Amended 2026-10-29 by the Phase 7.1 review fix (browser), IN-B01: a device that
// is already on now writes the key (the case below), so { device: 'on' } left this
// list; the device states that are not ready and not on stay.
test(NAME + 'does not write the key when it skips for another dialog or a locked page, or for a device that is not ready', async () => {
  for (const extra of [{ otherDialog: true }, { modalOpen: true }, { device: 'blocked' }, { device: 'reject' }, { base: 'needs-install' }, { locationAsking: true }]) {
    const m = load({ stored: JSON.stringify(['7']), ...extra });
    await m.go();
    assert.equal(m.dialog.showCalls, 0);
    assert.equal(m.store.get(KEY), '["7"]', JSON.stringify(extra));
    assert.equal(m.log.some((l) => l.startsWith('write:')), false, JSON.stringify(extra));
  }
});

test(NAME + 'a box opened between the checks and the open stops the ask without writing the key', async () => {
  // The page check is made after deviceState resolves, so a box that opened while
  // the device state was being asked is seen.
  const m = load();
  const original = m.options.notify.deviceState;
  m.options.notify.deviceState = (key) => original(key).then((state) => { m.doc.documentElement.attrs['data-modal-open'] = 'true'; return state; });
  const result = await m.go();
  assert.equal(result, 'skipped');
  assert.equal(m.dialog.showCalls, 0);
  assert.equal(m.store.size, 0);
});

test(NAME + 'a list holding only other account ids does not stop the ask, and those ids stay', async () => {
  const m = load({ stored: JSON.stringify(['7', '9']) });
  await m.go();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.store.get(KEY), '["7","9","42"]');
});

test(NAME + 'an older single other id is kept as one entry and this account is added', async () => {
  const m = load({ stored: 8 });
  await m.go();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.store.get(KEY), '["8","42"]');
});

test(NAME + 'the list keeps the last 8 ids, a ninth drops the oldest', async () => {
  const eight = ['1', '2', '3', '4', '5', '6', '7', '8'];
  const m = load({ stored: JSON.stringify(eight) });
  await m.go();
  assert.equal(m.dialog.showCalls, 1);
  assert.deepEqual(JSON.parse(m.store.get(KEY)), ['2', '3', '4', '5', '6', '7', '8', '42']);
});

test(NAME + 'a stored value that is neither an id nor a list means no ask', async () => {
  for (const junk of ['{"a":1}', '"42"', 'true', 'not json', '[', '42x']) {
    const m = load({ stored: junk });
    const result = await m.go();
    assert.equal(result, 'skipped', junk);
    assert.equal(m.dialog.showCalls, 0, junk);
    assert.equal(m.store.get(KEY), junk, 'the junk is left as it was');
  }
});

test(NAME + 'ids in a list that are not ids are ignored, and this account is still asked', async () => {
  const m = load({ stored: JSON.stringify(['7', 9, '0', 'x', '12']) });
  await m.go();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.store.get(KEY), '["7","12","42"]');
});

test(NAME + 'a storage that cannot be read or written means nothing opens and nothing throws', async () => {
  for (const extra of [{ readThrows: true }, { writeThrows: true }]) {
    const m = load(extra);
    const result = await m.go();
    assert.equal(result, 'skipped', JSON.stringify(extra));
    assert.equal(m.dialog.showCalls, 0, JSON.stringify(extra));
  }
});

test(NAME + 'runs once per page load: a second run returns the first and never opens the dialog twice', async () => {
  const m = load();
  const first = m.ask.run(m.options);
  const second = m.ask.run(m.options);
  assert.equal(first, second);
  await first;
  await m.ask.run();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.deviceCalls.length, 1);
});

test(NAME + 'runs by itself at load when the page has the dialog and notify.js', async () => {
  const m = load({ globals: true });
  await settle();
  await settle();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.store.get(KEY), '["42"]');
});

test(NAME + 'leaves quietly at load on a page without the dialog', async () => {
  const m = load({ globals: true, noDialog: true });
  await settle();
  assert.equal(m.dialog.showCalls, 0);
  assert.equal(m.deviceCalls.length, 0);
});

// ---- the answers ---------------------------------------------------------

test(NAME + 'Allow: enable is the first call of the click, before the box is read and before any request', async () => {
  const m = load({ enable: 'pending' });
  await m.go();
  m.tick(true);
  m.log.length = 0;
  m.allow.click();
  assert.deepEqual(m.enables, ['BPUSHKEY']);
  assert.equal(m.log[0], 'enable', 'enable is the first statement of the tap');
  assert.equal(m.fetches.length, 0, 'no request before enable settles');
  assert.equal(m.dialog.open, true);
  assert.equal(m.allow.disabled, true);
  assert.equal(m.later.disabled, true);
  assert.equal(m.dialog.attrs['aria-busy'], 'true');
  m.releaseEnable({ ok: true });
  await settle();
  await settle();
  assert.equal(m.fetches.length, 1);
  assert.ok(m.log.indexOf('enable') < m.log.indexOf('fetch'));
  assert.equal(m.dialog.open, false);
  assert.equal(m.dialog.attrs['aria-busy'], undefined);
});

test(NAME + 'Allow with the box ticked sends the Reliable request, the same one the sign up pop-up sends', async () => {
  const m = load();
  await m.go();
  m.tick(true);
  m.allow.click();
  await settle();
  await settle();
  assert.equal(m.fetches.length, 1);
  const req = m.fetches[0];
  assert.equal(req.url, '/api/me/alerts');
  assert.equal(req.method, 'PATCH');
  assert.equal(req.credentials, 'same-origin');
  assert.equal(JSON.stringify(req.headers), '{"Content-Type":"application/json"}');
  assert.equal(JSON.stringify(req.body), '{"reliable_alerts":true}');
  assert.equal(m.dialog.open, false);
  assert.equal(m.dialog.closeCalls, 1);
});

test(NAME + 'Allow with the box unticked sends no request and closes the dialog', async () => {
  const m = load();
  await m.go();
  m.allow.click();
  await settle();
  await settle();
  assert.equal(m.enables.length, 1);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.dialog.open, false);
});

test(NAME + 'a refused, rejected or throwing enable still saves the ticked box and closes the dialog', async () => {
  for (const enable of ['refused', 'reject', 'throw']) {
    const m = load({ enable });
    await m.go();
    m.tick(true);
    m.allow.click();
    await settle();
    await settle();
    assert.equal(m.enables.length, 1, enable);
    assert.equal(m.fetches.length, 1, enable);
    assert.equal(m.dialog.open, false, enable);
  }
});

test(NAME + 'a failed Reliable request still closes the dialog', async () => {
  for (const patch of [500, 'reject']) {
    const m = load({ patch });
    await m.go();
    m.tick(true);
    m.allow.click();
    await settle();
    await settle();
    assert.equal(m.dialog.open, false, String(patch));
  }
});

test(NAME + 'a second tap on Allow while it is busy changes nothing', async () => {
  const m = load({ enable: 'pending' });
  await m.go();
  m.allow.fire('click');
  m.allow.fire('click');
  m.later.fire('click');
  assert.equal(m.enables.length, 1);
  assert.equal(m.dialog.open, true);
  m.releaseEnable({ ok: true });
  await settle();
  await settle();
  assert.equal(m.enables.length, 1);
  assert.equal(m.dialog.closeCalls, 1);
});

test(NAME + 'Not now closes the dialog and never calls enable or sends a request, even with the box ticked', async () => {
  const m = load();
  await m.go();
  m.tick(true);
  m.later.click();
  await settle();
  assert.equal(m.enables.length, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.dialog.open, false);
});

test(NAME + 'Escape is Not now: the cancel event is prevented, nothing is asked or sent', async () => {
  const m = load();
  await m.go();
  m.tick(true);
  const event = m.escape();
  assert.equal(event.prevented, 1);
  assert.equal(m.enables.length, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.dialog.open, false);
});

test(NAME + 'Escape while Allow is busy is prevented and does nothing more', async () => {
  const m = load({ enable: 'pending' });
  await m.go();
  m.allow.click();
  const event = m.escape();
  assert.equal(event.prevented, 1);
  assert.equal(m.dialog.open, true);
  m.releaseEnable({ ok: true });
  await settle();
  await settle();
  assert.equal(m.dialog.open, false);
});

test(NAME + 'a dialog the browser closes by itself sends nothing and frees the buttons', async () => {
  const m = load();
  await m.go();
  m.dialog.open = false;
  m.dialog.fire('close');
  assert.equal(m.enables.length, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.allow.disabled, false);
});

test(NAME + 'focus goes back to the element that had it', async () => {
  const m = load();
  const before = m.make('page-button');
  m.doc.activeElement = before;
  await m.go();
  m.later.click();
  assert.equal(before.focused, 1);
});

// Phase 7.1 review fix (browser), IN-B01: a device already on is as good as asked,
// so the account is written into the list and later Nearby loads stop asking the
// server for the device state. Nothing opens and nothing else is sent.
test(NAME + 'IN-B01: a device already on writes the account, and the next load asks nothing', async () => {
  const m = load({ device: 'on', stored: JSON.stringify(['7']) });
  assert.equal(await m.go(), 'skipped');
  assert.equal(m.dialog.showCalls, 0);
  assert.deepEqual(JSON.parse(m.store.get(KEY)), ['7', '42']);
  assert.equal(m.fetches.length, 0);

  const next = load({ device: 'on', stored: m.store.get(KEY) });
  assert.equal(await next.go(), 'skipped');
  assert.deepEqual(next.deviceCalls, [], 'no device state request on the next load');

  const full = load({ device: 'on', stored: JSON.stringify(['1', '2', '3', '4', '5', '6', '7', '8']) });
  await full.go();
  assert.deepEqual(JSON.parse(full.store.get(KEY)), ['2', '3', '4', '5', '6', '7', '8', '42'], 'the last 8 are kept');

  const broken = load({ device: 'on', writeThrows: true });
  assert.equal(await broken.go(), 'skipped', 'a store that cannot be written changes nothing else');
});

// Phase 7.1 review fix (browser), IN-B06: a showModal that throws means the box
// never opened: the account is taken back out of the list (other ids stay), no
// listener is left on the box, and the run answers skipped.
test(NAME + 'IN-B06: a showModal that throws leaves no key for this account and no listeners', async () => {
  const m = load({ stored: JSON.stringify(['7']) });
  m.dialog.showModal = function () { this.showCalls += 1; throw new Error('InvalidStateError'); };
  assert.equal(await m.go(), 'skipped');
  assert.deepEqual(JSON.parse(m.store.get(KEY)), ['7']);
  for (const el of [m.dialog, m.allow, m.later]) {
    assert.equal(el.listeners.length, 0, el.id + ' has no listener');
  }
  assert.equal(m.enables.length, 0);
});

// ---- WR-B05: never together with the browser's location question ----------
//
// Phase 7.1 review fix (browser), WR-B05: an app on the Home Screen has its own
// storage, so its first Nearby load asks for the location (position.js startMode
// answers 'ask') while the alerts ask would open too. The ask waits for this
// load's location decision (PinalertPosition.askingThisLoad, which position.js
// sets when the map starts), skips without writing the key when the question is
// asked on this load or no decision comes within 5 s, and opens otherwise.

test(NAME + 'WR-B05: skips, writing no key, when the location question is asked on this load', async () => {
  const m = load({ locationAsking: true });
  assert.equal(await m.go(), 'skipped');
  assert.equal(m.dialog.showCalls, 0);
  assert.equal(m.store.has(KEY), false, 'the next Nearby load can ask');
});

test(NAME + 'WR-B05: opens when this load asks no location question', async () => {
  const m = load({ locationAsking: false });
  assert.equal(await m.go(), 'opened');
  assert.ok(m.looks.length >= 1, 'the location decision was read');
});

test(NAME + 'WR-B05: waits for a decision that comes after the device state, then skips', async () => {
  // position.js runs after this script, so the decision can come later than
  // deviceState: null (not decided) on the first looks, then the question.
  let answer = null;
  const m = load({ timers: true, locationAsking: () => answer });
  const run = m.go();
  await settle();
  assert.deepEqual(m.deviceCalls, ['BPUSHKEY']);
  assert.equal(m.dialog.showCalls, 0, 'nothing opens before the decision');
  m.advance(300);
  await settle();
  assert.equal(m.dialog.showCalls, 0);
  answer = true;
  m.advance(100);
  assert.equal(await run, 'skipped');
  assert.equal(m.store.has(KEY), false);
});

test(NAME + 'WR-B05: waits for a decision that comes after the device state, then opens', async () => {
  let answer = null;
  const m = load({ timers: true, locationAsking: () => answer });
  const run = m.go();
  await settle();
  m.advance(500);
  await settle();
  assert.equal(m.dialog.showCalls, 0);
  answer = false;
  m.advance(100);
  assert.equal(await run, 'opened');
  assert.equal(m.dialog.showCalls, 1);
});

test(NAME + 'WR-B05: no decision within 5 s, or no position module, skips without the key', async () => {
  const m = load({ timers: true, locationAsking: null });
  const run = m.go();
  await settle();
  m.advance(4900);
  await settle();
  assert.equal(m.dialog.showCalls, 0);
  m.advance(200);
  assert.equal(await run, 'skipped');
  assert.equal(m.store.has(KEY), false);

  const none = load({ timers: true, position: null });
  const run2 = none.go();
  await settle();
  none.advance(5200);
  assert.equal(await run2, 'skipped');
  assert.equal(none.store.has(KEY), false);
});

test(NAME + 'WR-B05: a position module that throws counts as do not open', async () => {
  const m = load({ position: { askingThisLoad() { throw new Error('broken'); } } });
  assert.equal(await m.go(), 'skipped');
  assert.equal(m.store.has(KEY), false);
});

// ---- the script itself -----------------------------------------------------

test(NAME + 'the script writes no markup, no words, and reads the Reliable box only after enable', () => {
  const code = source.split('\n').filter((line) => !line.trim().startsWith('//')).join('\n');
  assert.doesNotMatch(code, /innerHTML|outerHTML|insertAdjacentHTML|document\.write|createElement|textContent/);
  const tap = source.slice(source.indexOf('function onAllow'), source.indexOf('function onLater'));
  assert.ok(tap.indexOf('notify.enable(') !== -1);
  assert.ok(tap.indexOf('notify.enable(') < tap.indexOf('.checked'), 'enable comes before the box is read');
  assert.ok(tap.indexOf('notify.enable(') < tap.indexOf('saveReliable('), 'enable comes before the request');
});

test(NAME + 'the script touches no key of notify.js and adds no listener to the page or window', () => {
  const code = source.split('\n').filter((line) => !line.trim().startsWith('//')).join('\n');
  assert.doesNotMatch(code, /INTENT_KEY|pinalert\.changesAsked|pinalert\.installAsked/);
  assert.doesNotMatch(code, /window\.addEventListener|document\.addEventListener|beforeinstallprompt/);
});
