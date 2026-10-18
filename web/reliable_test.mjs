// reliable_test.mjs - runs the real web/static/js/app.js and
// web/static/js/reliable.js together in a Node sandbox and checks the Only
// Reliable reporters tick (Phase 3.2 plan 14, NOTIF-04, D-26, 03.4 D-13,
// TRUST-02):
//
//   - the store keeps a report under the tick only when the server's own
//     field kept_by_reliable_filter is true; with the tick off every report
//     passes; the tick composes with All, Confirmed and Disputed by AND;
//   - setReliable only assigns (no request, no redraw) and says whether the
//     value changed;
//   - the address word reliable=1 is read with an exact comparison, written
//     with replaceState (never pushState) keeping the other parameters, the
//     hash and history.state, and removed alone when the tick goes off;
//   - a change drops a held report link, draws the filter line with its Show
//     everyone button (or hides it) and asks the store to redraw, with no new
//     request;
//   - a page opened on reliable=1 starts with the tick on before the first draw.
//
// Run it with: node --test web/reliable_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the two scripts call.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');
const reliableSource = fs.readFileSync(path.join(here, 'static', 'js', 'reliable.js'), 'utf8');

class FakeEl {
  constructor(doc, tag, id) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.hidden = false;
    this.className = '';
    this.type = '';
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) { this.children.push(node); node.parentNode = this; return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.doc.activeElement = this; }
  fire(type, evt = {}) {
    const event = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() {}, ...evt };
    (this.handlers[type] || []).slice().forEach((fn) => fn(event));
    return event;
  }
}

// load evaluates app.js and then reliable.js against a page with the Show
// button, the Show menu, the filter line and the empty state's button. href is
// the page address; the fake history rewrites it on replaceState and records
// every call.
function load({ href = '/' } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener() {},
    removeEventListener() {}
  };
  doc.documentElement = new FakeEl(doc, 'html');

  const button = new FakeEl(doc, 'button', 'show-button');
  const menu = new FakeEl(doc, 'div', 'show-menu');
  const line = new FakeEl(doc, 'p', 'reliable-line');
  line.hidden = true;
  const emptyClear = new FakeEl(doc, 'button', 'reliable-empty-clear');
  doc.byId = { 'show-button': button, 'show-menu': menu, 'reliable-line': line, 'reliable-empty-clear': emptyClear };

  const fetches = [];
  const calls = { replace: [], push: [] };
  const location = {};
  const setAddress = (target) => {
    const parsed = new URL(target, 'http://localhost');
    location.pathname = parsed.pathname;
    location.search = parsed.search;
    location.hash = parsed.hash;
  };
  setAddress(href);
  const history = {
    state: { tab: 'nearby' },
    replaceState(state, title, target) { calls.replace.push({ state, title, target }); setAddress(target); },
    pushState(state, title, target) { calls.push.push({ state, title, target }); setAddress(target); }
  };
  const window = {
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    location,
    history
  };
  const sandbox = {
    window,
    document: doc,
    navigator: {},
    fetch: (target) => new Promise((resolve, reject) => { fetches.push({ url: target && target.url, resolve, reject }); }),
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    URLSearchParams,
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent
  };
  vm.createContext(sandbox);
  vm.runInContext(appSource, sandbox);
  sandbox.Pinalert = window.Pinalert;
  vm.runInContext(reliableSource, sandbox);
  return { Pinalert: window.Pinalert, Reliable: window.PinalertReliable, doc, button, menu, line, emptyClear, fetches, calls, location, history };
}

const plain = (value) => JSON.parse(JSON.stringify(value));
const report = (id, view, extra = {}) => ({ id, view, severity: 'low', created_at: '2026-10-16T10:00:00Z', ...extra });

test('reportKeptByReliable: off keeps every report, on keeps only the server field being true', () => {
  const m = load();
  const kept = report(1, 'active', { kept_by_reliable_filter: true });
  const notKept = report(2, 'active', { kept_by_reliable_filter: false });
  const missing = report(3, 'active');
  const wrongType = report(4, 'active', { kept_by_reliable_filter: 'true' });
  for (const r of [kept, notKept, missing, wrongType]) {
    assert.equal(m.Pinalert.reportKeptByReliable(r, false), true);
  }
  assert.equal(m.Pinalert.reportKeptByReliable(kept, true), true);
  assert.equal(m.Pinalert.reportKeptByReliable(notKept, true), false);
  assert.equal(m.Pinalert.reportKeptByReliable(missing, true), false);
  assert.equal(m.Pinalert.reportKeptByReliable(wrongType, true), false);
  assert.equal(m.Pinalert.reportKeptByReliable(null, true), false);
  assert.equal(m.Pinalert.reportKeptByReliable(undefined, false), true);
});

const SAMPLE = [
  report(1, 'active', { confirmed_by_people: true, kept_by_reliable_filter: true }),
  report(2, 'active', { confirmed_by_people: true, kept_by_reliable_filter: false }),
  report(3, 'disputed', { confirmed_by_people: false, kept_by_reliable_filter: true }),
  report(4, 'disputed', { confirmed_by_people: false, kept_by_reliable_filter: false }),
  report(5, 'active', { confirmed_by_people: false, kept_by_reliable_filter: true, severity: 'critical' }),
  report(6, 'active', { confirmed_by_people: false, kept_by_reliable_filter: false })
];

test('the tick composes with All, Confirmed and Disputed by AND', () => {
  const m = load();
  m.Pinalert.state.reports = SAMPLE;
  const ids = () => plain(m.Pinalert.visibleReports().map((r) => r.id));

  assert.deepEqual(ids(), [1, 2, 3, 4, 5, 6], 'tick off, Show All: everything');
  m.Pinalert.setReliable(true);
  assert.deepEqual(ids(), [1, 3, 5], 'tick on, Show All: every kept report');
  m.Pinalert.setShow('confirmed');
  assert.deepEqual(ids(), [1], 'tick on, Show Confirmed: confirmed and kept');
  m.Pinalert.setShow('disputed');
  assert.deepEqual(ids(), [3], 'tick on, Show Disputed: disputed and kept');
  m.Pinalert.setReliable(false);
  assert.deepEqual(ids(), [3, 4], 'tick off again, Show Disputed: all disputed');
});

test('with the tick on and Show All the kept reports keep the disputed last order', () => {
  const m = load();
  m.Pinalert.state.reports = [SAMPLE[2], SAMPLE[0], SAMPLE[4]];
  m.Pinalert.setReliable(true);
  const ordered = m.Pinalert.disputedLast(m.Pinalert.visibleReports()).map((r) => r.id);
  assert.deepEqual(plain(ordered), [1, 5, 3]);
});

test('setReliable only assigns: no request, no redraw, and it says whether the value changed', () => {
  const m = load();
  let redraws = 0;
  m.Pinalert.subscribe(() => { redraws += 1; });
  assert.equal(m.Pinalert.state.reliable, false);
  assert.equal(m.Pinalert.setReliable(true), true);
  assert.equal(m.Pinalert.state.reliable, true);
  assert.equal(m.Pinalert.setReliable(true), false, 'the same value is no change');
  assert.equal(m.Pinalert.setReliable(false), true);
  assert.equal(m.Pinalert.state.reliable, false);
  assert.equal(redraws, 0);
  assert.equal(m.fetches.length, 0);
  // Anything but a true boolean is off.
  assert.equal(m.Pinalert.setReliable('yes'), false);
  assert.equal(m.Pinalert.state.reliable, false);
});

test('readFromAddress: only the exact word 1 turns the tick on', () => {
  const m = load();
  assert.equal(m.Reliable.readFromAddress('?reliable=1'), true);
  assert.equal(m.Reliable.readFromAddress('?view=disputed&reliable=1'), true);
  for (const search of ['?reliable=true', '?reliable=0', '?reliable=', '?reliable=11', '?reliable=%31x', '?Reliable=1', '?reliable', '', '?report=12', undefined, null, 5]) {
    assert.equal(m.Reliable.readFromAddress(search), false, `address ${String(search)} leaves the tick off`);
  }
});

test('set(true) replaces the address, keeping the other parameters, the hash and history.state', () => {
  const m = load({ href: '/?report=12&post=1#top' });
  const state = m.history.state;
  m.Reliable.set(true);
  assert.equal(m.calls.replace.length, 1);
  assert.equal(m.calls.push.length, 0, 'the address is replaced, never pushed');
  assert.equal(m.calls.replace[0].state, state);
  assert.equal(m.calls.replace[0].target, '/?report=12&post=1&reliable=1#top');
  assert.equal(m.Pinalert.state.reliable, true);
});

test('set(false) removes only the reliable word', () => {
  const m = load({ href: '/?report=12&reliable=1&post=1' });
  assert.equal(m.Pinalert.state.reliable, true, 'booted on');
  m.Reliable.set(false);
  assert.equal(m.calls.replace.length, 1);
  assert.equal(m.calls.push.length, 0);
  assert.equal(m.calls.replace[0].target, '/?report=12&post=1');
  const bare = load({ href: '/?reliable=1' });
  bare.Reliable.set(false);
  assert.equal(bare.calls.replace[0].target, '/', 'no empty question mark is left behind');
});

test('set with the value already held changes nothing: no address write, no redraw', () => {
  const m = load();
  let redraws = 0;
  m.Pinalert.subscribe(() => { redraws += 1; });
  m.Reliable.set(false);
  assert.equal(m.calls.replace.length, 0);
  assert.equal(redraws, 0);
  assert.equal(m.line.hidden, true);
});

test('a change drops a held report link, draws the line, redraws once and sends no request', () => {
  const m = load({ href: '/?report=12' });
  m.Pinalert.setFocusId('12');
  assert.equal(m.Pinalert.state.focusId, '12');
  let redraws = 0;
  m.Pinalert.subscribe(() => { redraws += 1; });

  m.Reliable.set(true);
  assert.equal(m.Pinalert.state.focusId, null, 'the report link ends like any other change of what the list shows');
  assert.equal(redraws, 1);
  assert.equal(m.fetches.length, 0, 'the tick is a filter over rows already in the store');
  assert.equal(m.line.hidden, false);
  assert.equal(m.line.textContent, 'Showing Reliable reporters and critical reports.Show everyone');
  assert.equal(m.line.children.length, 2);
  assert.equal(m.line.children[0].tagName, 'SPAN');
  assert.equal(m.line.children[0].textContent, 'Showing Reliable reporters and critical reports.');
  assert.equal(m.line.children[1].tagName, 'BUTTON');
  assert.equal(m.line.children[1].type, 'button');
  assert.equal(m.line.children[1].className, 'inline-text-button');
  assert.equal(m.line.children[1].textContent, 'Show everyone');

  m.Reliable.set(false);
  assert.equal(redraws, 2);
  assert.equal(m.line.hidden, true);
  assert.equal(m.line.textContent, '');
  assert.equal(m.fetches.length, 0);
});

test('toggle flips the tick', () => {
  const m = load();
  m.Reliable.toggle();
  assert.equal(m.Pinalert.state.reliable, true);
  assert.equal(m.location.search, '?reliable=1');
  m.Reliable.toggle();
  assert.equal(m.Pinalert.state.reliable, false);
  assert.equal(m.location.search, '');
});

test('the Show everyone button of the line turns the tick off and returns focus to the Show button', () => {
  const m = load();
  m.Reliable.set(true);
  m.line.children[1].fire('click');
  assert.equal(m.Pinalert.state.reliable, false);
  assert.equal(m.line.hidden, true);
  assert.equal(m.doc.activeElement, m.button);
  assert.equal(m.location.search, '');
});

test('the Show everyone button of the empty state does the same', () => {
  const m = load();
  m.Reliable.set(true);
  m.emptyClear.fire('click');
  assert.equal(m.Pinalert.state.reliable, false);
  assert.equal(m.line.hidden, true);
  assert.equal(m.doc.activeElement, m.button);
});

test('a page opened on reliable=1 starts with the tick on, the line drawn and no address write', () => {
  const m = load({ href: '/?reliable=1' });
  assert.equal(m.Pinalert.state.reliable, true);
  assert.equal(m.line.hidden, false);
  assert.equal(m.calls.replace.length, 0, 'reading the address writes nothing');
  assert.equal(m.calls.push.length, 0);
  assert.equal(m.fetches.length, 0);

  for (const href of ['/', '/?reliable=0', '/?reliable=true']) {
    const off = load({ href });
    assert.equal(off.Pinalert.state.reliable, false, href);
    assert.equal(off.line.hidden, true, href);
  }
});
