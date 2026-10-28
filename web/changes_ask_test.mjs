// changes_ask_test.mjs - runs the real web/static/js/changes-ask.js in a Node
// sandbox and checks the pop-up that asks "Tell you when this report changes?"
// the first time a person posts or votes (Phase 3.3 plan 20, the 03.3-UAT gap
// for test 4, owner 2026-10-27: ask in the flow, Settings only for changing
// your mind):
//
//   - when it opens (a device where alerts can work, a push key and an account
//     id on the page, no Mac Dock app, no other open dialog, once per account);
//   - Allow calls enable first, then turns Changes on with one request,
//     whatever enable answered; Not now and Escape send nothing;
//   - exactly one answer, however many taps;
//   - settled() lets a later box wait for it.
//
// Run it with: node --test web/changes_ask_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// Every case loads the script through load(), so a missing script fails each
// case by itself. The fakes cover only what changes-ask.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const scriptPath = path.join(here, 'static', 'js', 'changes-ask.js');

const settle = () => new Promise((resolve) => setImmediate(resolve));

function fakeEl(id, extra = {}) {
  return {
    id,
    attrs: {},
    listeners: [],
    focused: 0,
    disabled: false,
    isConnected: true,
    addEventListener(type, fn) { this.listeners.push({ type, fn }); },
    setAttribute(name, value) { this.attrs[name] = String(value); },
    getAttribute(name) { return Object.hasOwn(this.attrs, name) ? this.attrs[name] : null; },
    removeAttribute(name) { delete this.attrs[name]; },
    focus() { this.focused += 1; this.doc.activeElement = this; this.log.push('focus:' + this.id); },
    click() { if (!this.disabled) { this.fire('click'); } },
    fire(type, e = {}) {
      const event = { type, prevented: 0, preventDefault() { this.prevented += 1; }, ...e };
      this.listeners.filter((l) => l.type === type).forEach((l) => l.fn(event));
      return event;
    },
    ...extra
  };
}

// load builds a page and evaluates changes-ask.js on it. The options:
//   key, account       what the page carries (undefined means no such element)
//   base               what PinalertNotify.baseState answers
//   macDock            what isMacDockApp answers
//   notify             false for no window.PinalertNotify at all
//   enable             'resolve' | 'refused' | 'reject' | 'throw' | 'pending'
//   patch              200 | 400 | 500 | 'reject' | 'hang'
//   showModal          false for a dialog with no showModal
//   otherDialog        true for another open dialog on the page
//   modalOpen          true for a page locked by another box (data-modal-open
//                      on the root, set by Pinalert.lockPage for the report
//                      form, the Flag box and the reader window)
//   inert              true for a dialog the page lock made inert
//   stored             what pinalert.changesAsked holds before
//   readThrows, writeThrows   a storage that fails
//   noDialog           true for a page without the partial
function load(o = {}) {
  const opts = {
    key: 'BPUSHKEY', account: '7', base: 'maybe', macDock: false, notify: true, enable: 'resolve',
    patch: 200, showModal: true, otherDialog: false, stored: null, readThrows: false, writeThrows: false,
    noDialog: false, modalOpen: false, inert: false, ...o
  };
  const source = fs.readFileSync(scriptPath, 'utf8');
  const log = [];
  const doc = { activeElement: null };
  doc.documentElement = {
    attrs: opts.modalOpen ? { 'data-modal-open': 'true' } : {},
    hasAttribute(name) { return Object.hasOwn(this.attrs, name); },
    removeAttribute(name) { delete this.attrs[name]; }
  };
  const make = (id, extra) => fakeEl(id, { doc, log, ...extra });

  const dialog = make('changes-ask', {
    open: false,
    showCalls: 0,
    closeCalls: 0,
    close() { if (this.open) { this.open = false; this.closeCalls += 1; log.push('close'); this.fire('close'); } },
    closest(sel) { return sel === '[inert]' && opts.inert ? { id: 'locked' } : null; }
  });
  if (opts.showModal) {
    dialog.showModal = function () { this.open = true; this.showCalls += 1; log.push('showModal'); };
  }
  const title = make('changes-ask-title');
  const later = make('changes-ask-later');
  const allow = make('changes-ask-allow');
  const els = opts.noDialog ? {} : {
    'changes-ask': dialog, 'changes-ask-title': title, 'changes-ask-later': later, 'changes-ask-allow': allow
  };

  const nodeWith = (name, value) => ({ getAttribute: (n) => (n === name ? value : null) });
  doc.getElementById = (id) => els[id] || null;
  doc.querySelector = (sel) => {
    if (sel === '[data-push-key]') { return opts.key === undefined ? null : nodeWith('data-push-key', opts.key); }
    if (sel === '[data-account-id]') { return opts.account === undefined ? null : nodeWith('data-account-id', opts.account); }
    if (sel === 'dialog[open]') { return opts.otherDialog ? { id: 'other' } : (dialog.open ? dialog : null); }
    return null;
  };

  const store = new Map();
  if (opts.stored !== null) { store.set('pinalert.changesAsked', String(opts.stored)); }
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
  let releaseEnable = null;
  const notify = {
    baseState: () => opts.base,
    isMacDockApp: () => opts.macDock,
    enable(key) {
      enables.push(key);
      log.push('enable');
      if (opts.enable === 'throw') { throw new Error('enable broke'); }
      if (opts.enable === 'reject') { return Promise.reject(new Error('no')); }
      if (opts.enable === 'refused') { return Promise.resolve({ ok: false, reason: 'blocked' }); }
      if (opts.enable === 'pending') { return new Promise((resolve) => { releaseEnable = resolve; }); }
      return Promise.resolve({ ok: true });
    }
  };

  const fetches = [];
  const timers = [];
  const cleared = [];
  const sandbox = {
    document: doc,
    localStorage,
    AbortController,
    JSON,
    setTimeout(fn, ms) { timers.push({ fn, ms }); return timers.length; },
    clearTimeout(id) { cleared.push(id); },
    fetch(target, init) {
      log.push('fetch');
      fetches.push({
        url: String(target),
        method: init && init.method,
        credentials: init && init.credentials,
        headers: init && init.headers,
        body: init && typeof init.body === 'string' ? JSON.parse(init.body) : init && init.body,
        signal: init && init.signal
      });
      if (opts.patch === 'reject') { return Promise.reject(new Error('offline')); }
      if (opts.patch === 'hang') {
        return new Promise((resolve, reject) => {
          init.signal.addEventListener('abort', () => reject(new Error('aborted')));
        });
      }
      return Promise.resolve({ ok: opts.patch < 400, status: opts.patch, json: () => Promise.resolve({}) });
    }
  };
  sandbox.window = sandbox;
  if (opts.notify) { sandbox.PinalertNotify = notify; }
  vm.runInNewContext(source, sandbox);
  return {
    ask: sandbox.PinalertChangesAsk,
    dialog, title, later, allow, doc, log, fetches, enables, timers, cleared, store, make,
    releaseEnable: (value) => releaseEnable(value),
    // escape sends the cancel event a dialog gets from the Escape key.
    escape: () => dialog.fire('cancel')
  };
}

const NAME = 'changes ask: ';

// ---- when it opens -------------------------------------------------------

test(NAME + 'opens on the first offer: key written first, title focused, no button focused, no request', async () => {
  const m = load();
  const pending = m.ask.offer();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.dialog.open, true);
  assert.equal(m.store.get('pinalert.changesAsked'), '7');
  assert.deepEqual(m.log, ['write:pinalert.changesAsked=7', 'showModal', 'focus:changes-ask-title']);
  assert.equal(m.title.focused, 1);
  assert.equal(m.allow.focused, 0);
  assert.equal(m.later.focused, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.enables.length, 0);
  let done = false;
  pending.then(() => { done = true; });
  await settle();
  assert.equal(done, false, 'the promise stays pending until the answer');
});

test(NAME + 'opens when the base state is ready as well as maybe', () => {
  for (const base of ['maybe', 'ready']) {
    const m = load({ base });
    m.ask.offer();
    assert.equal(m.dialog.showCalls, 1, base);
  }
});

test(NAME + 'asks once per account: a second offer after the answer does not open', async () => {
  const m = load();
  m.ask.offer();
  m.later.click();
  await settle();
  m.ask.offer();
  assert.equal(m.dialog.showCalls, 1);
});

test(NAME + 'storage holding another account id opens for this one and writes this one', () => {
  const m = load({ stored: 8 });
  m.ask.offer();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.store.get('pinalert.changesAsked'), '7');
});

test(NAME + 'storage holding this account id does not open', () => {
  const m = load({ stored: 7 });
  m.ask.offer();
  assert.equal(m.dialog.showCalls, 0);
});

test(NAME + 'storage that cannot be read or written does not open', () => {
  for (const o of [{ readThrows: true }, { writeThrows: true }]) {
    const m = load(o);
    m.ask.offer();
    assert.equal(m.dialog.showCalls, 0, JSON.stringify(o));
  }
});

test(NAME + 'never opens where alerts cannot work: needs-install, blocked, unsupported or unknown', () => {
  for (const base of ['needs-install', 'blocked', 'unsupported', 'surprise']) {
    const m = load({ base });
    m.ask.offer();
    assert.equal(m.dialog.showCalls, 0, base);
    assert.equal(m.store.has('pinalert.changesAsked'), false, base + ' writes no key');
  }
});

test(NAME + 'never opens without a push key or an account id on the page', () => {
  for (const o of [{ key: '' }, { key: undefined }, { account: undefined }, { account: '' }, { account: 'abc' }, { account: '0' }]) {
    const m = load(o);
    m.ask.offer();
    assert.equal(m.dialog.showCalls, 0, JSON.stringify(o));
  }
});

test(NAME + 'never opens without notify.js or inside a Mac Dock app', () => {
  const none = load({ notify: false });
  none.ask.offer();
  assert.equal(none.dialog.showCalls, 0);
  const dock = load({ macDock: true });
  dock.ask.offer();
  assert.equal(dock.dialog.showCalls, 0);
  assert.equal(dock.store.has('pinalert.changesAsked'), false);
});

test(NAME + 'never opens on a dialog with no showModal or a page without the partial', async () => {
  const noShow = load({ showModal: false });
  noShow.ask.offer();
  assert.equal(noShow.dialog.open, false);
  assert.equal(noShow.store.has('pinalert.changesAsked'), false);
  const none = load({ noDialog: true });
  assert.equal(typeof none.ask.offer, 'function');
  assert.equal(typeof none.ask.settled, 'function');
  await none.ask.offer();
  await none.ask.settled();
});

test(NAME + 'does not open while another dialog is open, and then writes no key', () => {
  const m = load({ otherDialog: true });
  m.ask.offer();
  assert.equal(m.dialog.showCalls, 0);
  assert.equal(m.store.has('pinalert.changesAsked'), false);
});

// CR-01 (03.3-REVIEW): the report form, the Flag box and the reader window are
// not native dialogs. They lock the page with data-modal-open on the root and
// make the rest of it inert, this dialog included, so a pop-up opened then
// could not be answered.
test(NAME + 'does not open while the page is locked by another box (data-modal-open), writes no key, and offers again later', async () => {
  const m = load({ modalOpen: true });
  let done = false;
  m.ask.offer().then(() => { done = true; });
  await settle();
  assert.equal(done, true, 'offer resolves at once');
  assert.equal(m.dialog.showCalls, 0);
  assert.equal(m.dialog.open, false);
  assert.equal(m.store.has('pinalert.changesAsked'), false, 'no key, so the next post or vote can ask');
  let quiet = false;
  m.ask.settled().then(() => { quiet = true; });
  await settle();
  assert.equal(quiet, true, 'settled resolves');
  // The other box closes: the next offer opens.
  m.doc.documentElement.removeAttribute('data-modal-open');
  m.ask.offer();
  assert.equal(m.dialog.showCalls, 1);
  assert.equal(m.store.has('pinalert.changesAsked'), true);
});

test(NAME + 'does not open while the dialog itself sits inside an inert part of the page, and writes no key', async () => {
  const m = load({ inert: true });
  await m.ask.offer();
  assert.equal(m.dialog.showCalls, 0);
  assert.equal(m.store.has('pinalert.changesAsked'), false);
});

test(NAME + 'a dialog that throws on showModal resolves at once and sends nothing', async () => {
  const m = load();
  m.dialog.showModal = function () { throw new Error('not allowed'); };
  await m.ask.offer();
  assert.equal(m.fetches.length, 0);
  assert.equal(m.enables.length, 0);
});

// ---- Allow ---------------------------------------------------------------

test(NAME + 'Allow calls enable first in the same tick, with the page key and no request yet', async () => {
  const m = load({ enable: 'pending' });
  const pending = m.ask.offer();
  m.allow.click();
  assert.deepEqual(m.enables, ['BPUSHKEY']);
  assert.equal(m.fetches.length, 0, 'no request before enable settles');
  assert.equal(m.allow.disabled, true);
  assert.equal(m.later.disabled, true);
  assert.equal(m.dialog.getAttribute('aria-busy'), 'true');
  assert.equal(m.dialog.open, true);
  m.releaseEnable({ ok: true });
  await settle();
  assert.equal(m.fetches.length, 1);
  assert.equal(m.fetches[0].url, '/api/me/alerts');
  assert.equal(m.fetches[0].method, 'PATCH');
  assert.equal(m.fetches[0].credentials, 'same-origin');
  assert.deepEqual(m.fetches[0].body, { changes_push: true });
  assert.equal(m.fetches[0].headers['Content-Type'], 'application/json');
  assert.equal(m.dialog.open, false);
  assert.equal(m.dialog.getAttribute('aria-busy'), null);
  assert.equal(m.allow.disabled, false);
  assert.equal(m.later.disabled, false);
  await pending;
  assert.ok(m.log.indexOf('enable') < m.log.indexOf('fetch'));
});

test(NAME + 'Allow turns Changes on whatever enable answered', async () => {
  for (const enable of ['resolve', 'refused', 'reject', 'throw']) {
    const m = load({ enable });
    const pending = m.ask.offer();
    m.allow.click();
    await pending;
    assert.equal(m.enables.length, 1, enable);
    assert.equal(m.fetches.length, 1, enable);
    assert.deepEqual(m.fetches[0].body, { changes_push: true }, enable);
    assert.equal(m.dialog.open, false, enable);
  }
});

test(NAME + 'a request answered 400 or 500, or rejected, still closes and resolves', async () => {
  for (const patch of [400, 500, 'reject']) {
    const m = load({ patch });
    const pending = m.ask.offer();
    m.allow.click();
    await pending;
    assert.equal(m.fetches.length, 1, String(patch));
    assert.equal(m.dialog.open, false, String(patch));
    assert.equal(m.dialog.getAttribute('aria-busy'), null, String(patch));
  }
});

test(NAME + 'a request that never answers is ended by the timeout and the pop-up still closes', async () => {
  const m = load({ patch: 'hang' });
  const pending = m.ask.offer();
  m.allow.click();
  await settle();
  assert.equal(m.fetches.length, 1);
  assert.ok(m.fetches[0].signal, 'the request carries an abort signal');
  assert.equal(m.dialog.open, true);
  const timer = m.timers.find((t) => t.ms === 10000);
  assert.ok(timer, 'a ten second timeout is set');
  timer.fn();
  await pending;
  assert.equal(m.dialog.open, false);
});

// ---- Not now and Escape --------------------------------------------------

test(NAME + 'Not now sends nothing, never calls enable, closes and resolves', async () => {
  const m = load();
  const pending = m.ask.offer();
  m.later.click();
  await pending;
  assert.equal(m.enables.length, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.dialog.open, false);
});

test(NAME + 'Escape is Not now: the cancel is prevented, nothing is sent, the dialog closes', async () => {
  const m = load();
  const pending = m.ask.offer();
  const event = m.escape();
  assert.equal(event.prevented, 1);
  await pending;
  assert.equal(m.enables.length, 0);
  assert.equal(m.fetches.length, 0);
  assert.equal(m.dialog.open, false);
});

test(NAME + 'Escape while Allow is busy is prevented and does nothing more', async () => {
  const m = load({ enable: 'pending' });
  const pending = m.ask.offer();
  m.allow.click();
  const event = m.escape();
  assert.equal(event.prevented, 1);
  assert.equal(m.dialog.open, true, 'the busy pop-up stays open');
  m.releaseEnable({ ok: true });
  await pending;
  assert.equal(m.fetches.length, 1);
  assert.equal(m.dialog.open, false);
});

test(NAME + 'a double tap on Allow gives one enable and one request', async () => {
  const m = load({ enable: 'pending' });
  const pending = m.ask.offer();
  m.allow.fire('click');
  m.allow.fire('click');
  m.releaseEnable({ ok: true });
  await pending;
  assert.equal(m.enables.length, 1);
  assert.equal(m.fetches.length, 1);
});

test(NAME + 'Allow then Escape gives one request, and Not now while busy is ignored', async () => {
  const m = load({ enable: 'pending' });
  const pending = m.ask.offer();
  m.allow.click();
  m.escape();
  m.later.fire('click');
  m.releaseEnable({ ok: true });
  await pending;
  assert.equal(m.fetches.length, 1);
  assert.equal(m.enables.length, 1);
});

// ---- settled and offer while open ------------------------------------------

test(NAME + 'settled resolves at once when no pop-up is open', async () => {
  const m = load();
  let done = false;
  m.ask.settled().then(() => { done = true; });
  await settle();
  assert.equal(done, true);
});

test(NAME + 'settled resolves at once when the pop-up did not open', async () => {
  const m = load({ base: 'blocked' });
  m.ask.offer();
  let done = false;
  m.ask.settled().then(() => { done = true; });
  await settle();
  assert.equal(done, true);
});

test(NAME + 'settled is pending in the same tick as offer and resolves only after the answer', async () => {
  const m = load();
  m.ask.offer();
  let done = false;
  m.ask.settled().then(() => { done = true; });
  await settle();
  assert.equal(done, false, 'still waiting while the pop-up is open');
  m.later.click();
  await settle();
  assert.equal(done, true);
});

test(NAME + 'offer while the pop-up is open does not open a second one and shares the wait', async () => {
  const m = load();
  m.ask.offer();
  let done = false;
  m.ask.offer().then(() => { done = true; });
  assert.equal(m.dialog.showCalls, 1);
  await settle();
  assert.equal(done, false);
  m.later.click();
  await settle();
  assert.equal(done, true);
});

// ---- focus -----------------------------------------------------------------

test(NAME + 'focus goes back to the element that had it, when it is still on the page', async () => {
  const m = load();
  const vote = m.make('vote-confirm');
  m.doc.activeElement = vote;
  const pending = m.ask.offer();
  assert.equal(vote.focused, 0);
  m.later.click();
  await pending;
  assert.equal(vote.focused, 1);
  assert.equal(m.doc.activeElement, vote);
});

test(NAME + 'focus is not sent to an element that left the page', async () => {
  const m = load();
  const vote = m.make('vote-confirm');
  m.doc.activeElement = vote;
  const pending = m.ask.offer();
  vote.isConnected = false;
  m.later.click();
  await pending;
  assert.equal(vote.focused, 0);
});
