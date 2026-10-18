// show_menu_test.mjs - runs the real web/static/js/app.js and
// web/static/js/show-menu.js together in a Node sandbox and checks the Show
// menu and the grouping rules the Go source tests cannot (Phase 3.4, D-13,
// D-14, TRUST-02):
//
//   - the menu opens with the chosen item focused, arrow keys move, Enter
//     chooses and closes and returns focus to the button, Escape closes
//     without choosing, and an outside click closes;
//   - the old /?view=disputed link (and the older show_disputed=true
//     bookmark) opens Show Disputed, and any other address opens Show All;
//   - choosing Disputed asks for a new read, choosing Confirmed against All
//     does not (it only draws the rows in the store again);
//   - All draws the not disputed reports first and the disputed ones last,
//     Disputed draws only disputed reports, and Confirmed draws only reports
//     whose server field confirmed_by_people is true, whatever their
//     visibility, and never a disputed one.
//
// Where the grouping helper lives: the pure helpers are in app.js
// (Pinalert.reportInShow, visibleReports and disputedLast), which the Node
// tests already load; feed.js is too DOM heavy for a sandbox and only calls
// them. Run it with: node --test web/show_menu_test.mjs
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
const menuSource = fs.readFileSync(path.join(here, 'static', 'js', 'show-menu.js'), 'utf8');
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
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) { this.children.push(node); node.parentNode = this; return node; }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  querySelector() { return null; }
  // Amended 2026-10-17 by Phase 3.2 plan 14 (D-26): the menu query is now the
  // three radios and the Only Reliable reporters checkbox in one selector, and
  // the fake answers them in document order, like a browser.
  querySelectorAll(selector) {
    return selector === '[role="menuitemradio"], [role="menuitemcheckbox"]'
      ? this.children.filter((c) => ['menuitemradio', 'menuitemcheckbox'].includes(c.getAttribute('role')))
      : [];
  }
  focus() { this.doc.activeElement = this; }
  // fire runs the handlers of one event type on this element, then bubbles it
  // to the parent the way the browser does, so a delegated listener sees it.
  fire(type, evt = {}) {
    const event = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...evt };
    for (let node = this; node && !event.stopped; node = node.parentNode) {
      (node.handlers[type] || []).slice().forEach((fn) => fn(event));
    }
    return event;
  }
}

// load evaluates app.js and then show-menu.js against a list head with the
// button, its label and the three item menu. search is the page address'
// query string. fetches keeps every list request.
function load({ search = '' } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    handlers: {},
    // captured says which listeners were added for the capture phase. A
    // listener is told apart by its function and its capture flag, as in a
    // browser: removeEventListener with the other flag removes nothing.
    captured: new Map(),
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener(type, fn, options) {
      (doc.handlers[type] = doc.handlers[type] || []).push(fn);
      doc.captured.set(fn, options === true || (!!options && options.capture === true));
    },
    removeEventListener(type, fn, options) {
      const capture = options === true || (!!options && options.capture === true);
      if (doc.captured.get(fn) !== capture) { return; }
      doc.handlers[type] = (doc.handlers[type] || []).filter((h) => h !== fn);
    }
  };
  doc.documentElement = new FakeEl(doc, 'html');

  const button = new FakeEl(doc, 'button', 'show-button');
  button.setAttribute('aria-expanded', 'false');
  const label = new FakeEl(doc, 'span', 'show-button-label');
  label.textContent = 'Show: All';
  button.appendChild(label);
  const menu = new FakeEl(doc, 'div', 'show-menu');
  menu.hidden = true;
  const items = {};
  [['all', 'true'], ['confirmed', 'false'], ['disputed', 'false']].forEach(([show, checked]) => {
    const item = new FakeEl(doc, 'button');
    item.setAttribute('role', 'menuitemradio');
    item.setAttribute('aria-checked', checked);
    item.setAttribute('data-show', show);
    menu.appendChild(item);
    items[show] = item;
  });
  // The hairline and the Only Reliable reporters checkbox item (Phase 3.2 plan
  // 14). The separator is not an item, so the arrow keys skip it.
  const sep = new FakeEl(doc, 'div');
  sep.setAttribute('role', 'separator');
  menu.appendChild(sep);
  const tick = new FakeEl(doc, 'button');
  tick.setAttribute('role', 'menuitemcheckbox');
  tick.setAttribute('aria-checked', 'false');
  tick.setAttribute('data-reliable', '');
  menu.appendChild(tick);
  items.reliable = tick;
  doc.byId = { 'show-button': button, 'show-button-label': label, 'show-menu': menu };

  const fetches = [];
  const replaced = [];
  const window = {
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    location: { search, pathname: '/', hash: '' },
    history: { state: null, replaceState(state, title, target) { replaced.push(target); }, pushState() { throw new Error('pushState must not be used'); } }
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
  // reliable.js loads between app.js and show-menu.js, as in the page, so the
  // tick item has its real owner to call.
  vm.runInContext(reliableSource, sandbox);
  vm.runInContext(menuSource, sandbox);
  return { Pinalert: window.Pinalert, Menu: window.PinalertShowMenu, doc, button, label, menu, items, fetches, replaced };
}

// plain copies a value made in the sandbox into this realm, so deepEqual does
// not trip on the sandbox's own Array.
const plain = (value) => JSON.parse(JSON.stringify(value));

const report = (id, view, extra = {}) => ({ id, view, severity: 'low', created_at: '2026-10-16T10:00:00Z', ...extra });

test('opening the menu sets aria-expanded and focuses the chosen item', () => {
  const m = load();
  assert.equal(m.menu.hidden, true);
  m.button.fire('click');
  assert.equal(m.menu.hidden, false);
  assert.equal(m.button.getAttribute('aria-expanded'), 'true');
  assert.equal(m.doc.activeElement, m.items.all, 'focus goes to the checked item');
});

test('a second press on the button closes the menu', () => {
  const m = load();
  m.button.fire('click');
  m.button.fire('click');
  assert.equal(m.menu.hidden, true);
  assert.equal(m.button.getAttribute('aria-expanded'), 'false');
});

test('ArrowDown and ArrowUp move between the items and wrap', () => {
  const m = load();
  m.button.fire('click');
  m.items.all.fire('keydown', { key: 'ArrowDown' });
  assert.equal(m.doc.activeElement, m.items.confirmed);
  m.items.confirmed.fire('keydown', { key: 'ArrowDown' });
  assert.equal(m.doc.activeElement, m.items.disputed);
  // Amended 2026-10-17 by Phase 3.2 plan 14 (D-26): the Only Reliable reporters
  // tick is the fourth item, so the wrap is at the tick, not at Disputed.
  m.items.disputed.fire('keydown', { key: 'ArrowDown' });
  assert.equal(m.doc.activeElement, m.items.reliable, 'ArrowDown reaches the tick as the fourth item');
  m.items.reliable.fire('keydown', { key: 'ArrowDown' });
  assert.equal(m.doc.activeElement, m.items.all, 'ArrowDown wraps to the first item');
  m.items.all.fire('keydown', { key: 'ArrowUp' });
  assert.equal(m.doc.activeElement, m.items.reliable, 'ArrowUp wraps to the last item');
});

test('Home reaches the first item and End reaches the Only Reliable reporters tick', () => {
  const m = load();
  m.button.fire('click');
  m.items.all.fire('keydown', { key: 'End' });
  assert.equal(m.doc.activeElement, m.items.reliable);
  m.items.reliable.fire('keydown', { key: 'Home' });
  assert.equal(m.doc.activeElement, m.items.all);
});

test('the separator is not an item: the arrow keys step over it', () => {
  const m = load();
  m.button.fire('click');
  m.items.disputed.focus();
  m.items.disputed.fire('keydown', { key: 'ArrowDown' });
  assert.equal(m.doc.activeElement.getAttribute('role'), 'menuitemcheckbox');
});

test('Enter on the tick toggles it, closes the menu and puts focus back on the Show button', () => {
  const m = load();
  m.button.fire('click');
  m.items.reliable.focus();
  const evt = m.items.reliable.fire('keydown', { key: 'Enter' });
  assert.equal(evt.defaultPrevented, true, 'the browser click is stopped so the toggle happens once');
  assert.equal(m.Pinalert.state.reliable, true);
  assert.equal(m.items.reliable.getAttribute('aria-checked'), 'true');
  assert.equal(m.menu.hidden, true);
  assert.equal(m.button.getAttribute('aria-expanded'), 'false');
  assert.equal(m.doc.activeElement, m.button);
  assert.deepEqual(plain(m.replaced), ['/?reliable=1']);
  assert.equal(m.fetches.length, 0, 'the tick sends no request');
  assert.equal(m.label.textContent, 'Show: All', 'the button words do not change');
  // A second Enter turns it off again.
  m.button.fire('click');
  m.items.reliable.fire('keydown', { key: 'Enter' });
  assert.equal(m.Pinalert.state.reliable, false);
  assert.equal(m.items.reliable.getAttribute('aria-checked'), 'false');
  assert.deepEqual(plain(m.replaced), ['/?reliable=1', '/']);
});

test('a click on the tick toggles it the same way (Space and touch)', () => {
  const m = load();
  m.button.fire('click');
  const inner = new FakeEl(m.doc, 'span');
  m.items.reliable.appendChild(inner);
  inner.fire('click');
  assert.equal(m.Pinalert.state.reliable, true);
  assert.equal(m.menu.hidden, true);
  assert.equal(m.doc.activeElement, m.button);
});

test('the tick leaves the radio choice alone: Show Confirmed stays chosen and checked', () => {
  const m = load();
  m.button.fire('click');
  m.items.confirmed.fire('click');
  m.button.fire('click');
  m.items.reliable.fire('click');
  assert.equal(m.Pinalert.state.show, 'confirmed');
  assert.equal(m.items.confirmed.getAttribute('aria-checked'), 'true');
  assert.equal(m.items.all.getAttribute('aria-checked'), 'false');
  assert.equal(m.items.reliable.getAttribute('aria-checked'), 'true');
});

test('opening the menu lands on the chosen radio, never on the tick, and shows the tick in step with the store', () => {
  const m = load({ search: '?reliable=1' });
  assert.equal(m.items.reliable.getAttribute('aria-checked'), 'true', 'a page opened on reliable=1 draws the tick on');
  m.button.fire('click');
  assert.equal(m.doc.activeElement, m.items.all);
  m.button.fire('click');
  // Turned off elsewhere (the Show everyone buttons), the item follows on open.
  m.Pinalert.setReliable(false);
  m.button.fire('click');
  assert.equal(m.items.reliable.getAttribute('aria-checked'), 'false');
});

test('Confirmed plus the tick draws only confirmed reports the server keeps', () => {
  const m = load();
  m.Pinalert.state.reports = [
    report(40, 'active', { confirmed_by_people: true, kept_by_reliable_filter: true }),
    report(41, 'active', { confirmed_by_people: true, kept_by_reliable_filter: false }),
    report(42, 'active', { confirmed_by_people: false, kept_by_reliable_filter: true }),
    report(43, 'disputed', { confirmed_by_people: true, kept_by_reliable_filter: true })
  ];
  m.button.fire('click');
  m.items.confirmed.fire('click');
  m.button.fire('click');
  m.items.reliable.fire('click');
  assert.deepEqual(plain(m.Pinalert.visibleReports().map((r) => r.id)), [40]);
});

test('Enter on Confirmed chooses it, closes the menu, returns focus and calls setShow', () => {
  const m = load();
  const calls = [];
  const realSetShow = m.Pinalert.setShow;
  m.Pinalert.setShow = (show) => { calls.push(show); return realSetShow(show); };
  m.button.fire('click');
  m.items.all.fire('keydown', { key: 'ArrowDown' });
  const evt = m.items.confirmed.fire('keydown', { key: 'Enter' });
  assert.equal(evt.defaultPrevented, true, 'the browser click is stopped so the choice is made once');
  assert.deepEqual(calls, ['confirmed']);
  assert.equal(m.Pinalert.state.show, 'confirmed');
  assert.equal(m.menu.hidden, true);
  assert.equal(m.button.getAttribute('aria-expanded'), 'false');
  assert.equal(m.doc.activeElement, m.button);
  assert.equal(m.label.textContent, 'Show: Confirmed');
  assert.equal(m.items.confirmed.getAttribute('aria-checked'), 'true');
  assert.equal(m.items.all.getAttribute('aria-checked'), 'false');
});

test('a click on an item chooses it the same way (Space and touch)', () => {
  const m = load();
  m.button.fire('click');
  m.items.disputed.fire('click');
  assert.equal(m.Pinalert.state.show, 'disputed');
  assert.equal(m.label.textContent, 'Show: Disputed');
  assert.equal(m.menu.hidden, true);
  assert.equal(m.doc.activeElement, m.button);
});

test('Escape closes the menu without choosing and returns focus to the button', () => {
  const m = load();
  m.button.fire('click');
  m.items.all.fire('keydown', { key: 'ArrowDown' });
  m.items.confirmed.fire('keydown', { key: 'Escape' });
  assert.equal(m.menu.hidden, true);
  assert.equal(m.Pinalert.state.show, 'all');
  assert.equal(m.label.textContent, 'Show: All');
  assert.equal(m.doc.activeElement, m.button);
});

test('a click outside closes the menu, a click inside does not', () => {
  const m = load();
  m.button.fire('click');
  const outside = new FakeEl(m.doc, 'div');
  (m.doc.handlers.click || []).slice().forEach((fn) => fn({ target: m.menu }));
  assert.equal(m.menu.hidden, false, 'a click inside the menu leaves it open');
  (m.doc.handlers.click || []).slice().forEach((fn) => fn({ target: outside }));
  assert.equal(m.menu.hidden, true);
  assert.equal((m.doc.handlers.click || []).length, 0, 'the outside listener is removed on close');
});

// clickReachingDocument sends a click to the document the way a browser does:
// the capture listeners first, then the bubble listeners unless something on
// the way stopped the event. stopAtTarget stands for a trigger that calls
// stopPropagation, like the person menu's (account-menu.js).
function clickReachingDocument(m, target, { stopAtTarget = false } = {}) {
  const run = (capture) => (m.doc.handlers.click || []).slice()
    .filter((fn) => m.doc.captured.get(fn) === capture).forEach((fn) => fn({ target }));
  run(true);
  if (!stopAtTarget) { run(false); }
}

// Amended 2026-10-17 by Phase 3.4 plan 25 (code review part 2 WR-04): the
// outside click listener is added in the capture phase, so the person menu's
// trigger, which stops its own click from bubbling, still closes the Show menu.
test('WR-04: a click on a trigger that stops propagation, like the person menu, closes the Show menu', () => {
  const m = load();
  m.button.fire('click');
  assert.equal(m.menu.hidden, false);
  const personTrigger = new FakeEl(m.doc, 'button');
  clickReachingDocument(m, personTrigger, { stopAtTarget: true });
  assert.equal(m.menu.hidden, true, 'the Show menu must close when the person menu opens');
  assert.equal(m.button.getAttribute('aria-expanded'), 'false');
  assert.equal((m.doc.handlers.click || []).length, 0, 'and the outside listener is removed with the same capture flag');
});

test('WR-04: a click inside the Show menu or on its button still leaves it alone', () => {
  const m = load();
  m.button.fire('click');
  clickReachingDocument(m, m.menu);
  assert.equal(m.menu.hidden, false);
  clickReachingDocument(m, m.items.all);
  assert.equal(m.menu.hidden, false);
});

test('the old /?view=disputed link opens Show Disputed, any other address opens Show All', () => {
  const disputed = load({ search: '?view=disputed' });
  assert.equal(disputed.Pinalert.state.show, 'disputed');
  assert.equal(disputed.Pinalert.state.view, 'disputed');
  assert.equal(disputed.label.textContent, 'Show: Disputed');
  assert.equal(disputed.items.disputed.getAttribute('aria-checked'), 'true');

  const bookmark = load({ search: '?show_disputed=true' });
  assert.equal(bookmark.Pinalert.state.show, 'disputed');

  // A view word beats the old bookmark parameter, and nothing else opens Disputed.
  for (const search of ['', '?view=active', '?view=all', '?view=Disputed', '?view=past', '?show_disputed=1',
    '?view=active&show_disputed=true', '?report=12']) {
    const m = load({ search });
    assert.equal(m.Pinalert.state.show, 'all', `address ${search || '(none)'} opens Show All`);
    assert.equal(m.Pinalert.state.view, 'all');
    assert.equal(m.label.textContent, 'Show: All');
  }
});

test('the alias writes nothing: no history call, no storage', () => {
  assert.equal(/history\.|localStorage|sessionStorage/.test(menuSource.replace(/\/\/.*$/gm, '')), false);
});

test('choosing Disputed asks for a new read, Confirmed against All does not', () => {
  const m = load();
  m.Pinalert.setCenter(12.9, 77.5, true);
  let redraws = 0;
  m.Pinalert.subscribe(() => { redraws += 1; });

  m.button.fire('click');
  m.items.confirmed.fire('click');
  assert.equal(m.fetches.length, 0, 'Confirmed against All sends no request');
  assert.equal(redraws, 1, 'the rows already in the store are drawn again');
  assert.equal(m.Pinalert.state.view, 'all');

  m.button.fire('click');
  m.items.disputed.fire('click');
  assert.equal(m.fetches.length, 1, 'Disputed reads the disputed answer');
  assert.match(String(m.fetches[0].url), /[?&]view=disputed(&|$)/);

  m.button.fire('click');
  m.items.all.fire('click');
  assert.equal(m.fetches.length, 2);
  assert.match(String(m.fetches[1].url), /[?&]view=all(&|$)/);
});

test('choosing the word that is already chosen changes nothing', () => {
  const m = load();
  m.button.fire('click');
  m.items.all.fire('click');
  assert.equal(m.fetches.length, 0);
  assert.equal(m.menu.hidden, true);
  assert.equal(m.doc.activeElement, m.button);
});

test('setShow ignores a word outside the list', () => {
  const m = load();
  assert.equal(m.Pinalert.setShow('everything'), false);
  assert.equal(m.Pinalert.setShow(undefined), false);
  assert.equal(m.Pinalert.state.show, 'all');
});

const SAMPLE = [
  report(1, 'active', { confirmed_by_people: true, visibility: 'live', severity: 'medium' }),
  report(2, 'disputed', { confirmed_by_people: false, visibility: 'hidden' }),
  report(3, 'active', { confirmed_by_people: false, visibility: 'provisional', severity: 'critical' }),
  report(4, 'disputed', { confirmed_by_people: false, visibility: 'hidden' })
];

test('All draws the not disputed reports first and the disputed ones last, each group in order', () => {
  const m = load();
  m.Pinalert.state.reports = SAMPLE;
  const ids = m.Pinalert.disputedLast(m.Pinalert.visibleReports()).map((r) => r.id);
  assert.deepEqual(plain(ids), [1, 3, 2, 4]);
  // A disputed report at the top of the input still goes last.
  const reordered = m.Pinalert.disputedLast([SAMPLE[1], SAMPLE[0], SAMPLE[3], SAMPLE[2]]).map((r) => r.id);
  assert.deepEqual(plain(reordered), [1, 3, 2, 4]);
});

test('Disputed draws only the disputed reports', () => {
  const m = load();
  m.Pinalert.state.reports = SAMPLE;
  m.Pinalert.setShow('disputed');
  assert.deepEqual(plain(m.Pinalert.visibleReports().map((r) => r.id)), [2, 4]);
});

test('Confirmed draws only reports with confirmed_by_people true, whatever their visibility', () => {
  const m = load();
  m.Pinalert.setShow('confirmed');
  m.Pinalert.state.reports = [
    report(10, 'active', { confirmed_by_people: true, visibility: 'provisional' }),
    report(11, 'active', { confirmed_by_people: false, visibility: 'live' }),
    report(12, 'active', { visibility: 'live' }),
    report(13, 'active', { confirmed_by_people: 'true', visibility: 'live' }),
    report(14, 'active', { confirmed_by_people: true, visibility: 'live' })
  ];
  assert.deepEqual(plain(m.Pinalert.visibleReports().map((r) => r.id)), [10, 14]);
});

test('a disputed report is never in Confirmed, even if the field were true', () => {
  const m = load();
  m.Pinalert.setShow('confirmed');
  m.Pinalert.state.reports = [report(20, 'disputed', { confirmed_by_people: true, visibility: 'hidden' })];
  assert.deepEqual(plain(m.Pinalert.visibleReports()), []);
  assert.equal(m.Pinalert.reportInShow(m.Pinalert.state.reports[0], 'confirmed'), false);
});

test('visibleReports returns a new array and never changes the store', () => {
  const m = load();
  m.Pinalert.state.reports = SAMPLE;
  const before = m.Pinalert.state.reports;
  const seen = m.Pinalert.visibleReports();
  assert.notEqual(seen, before);
  assert.equal(m.Pinalert.state.reports, before);
  assert.deepEqual(plain(before.map((r) => r.id)), [1, 2, 3, 4]);
});

test('the browser holds no threshold: a count never decides membership', () => {
  const m = load();
  m.Pinalert.setShow('confirmed');
  m.Pinalert.state.reports = [
    report(30, 'active', { confirmed_by_people: false, trust: { confirmed_places: 50, disputed_places: 0 } }),
    report(31, 'active', { confirmed_by_people: true, trust: { confirmed_places: 0, disputed_places: 9 } })
  ];
  assert.deepEqual(plain(m.Pinalert.visibleReports().map((r) => r.id)), [31]);
});
