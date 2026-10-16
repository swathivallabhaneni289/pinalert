// sheet_glue_test.mjs - runs the real web/static/js/sheet.js against a small
// element model and checks the glue that sheet_logic_test.mjs cannot reach
// (Phase 3.4, D-16): the positions the sheet takes on a box, the handle's words,
// the keyboard, a drag, a tap, a touch gesture, remember and restore, the
// motion rules and the idle laptop.
//
// Run it with: node --test web/sheet_glue_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the script calls: elements with attributes, event
// listeners and a style that keeps custom properties, a document whose
// listeners the test fires by hand, timers the test runs by hand, and a map that
// records the sheet tops it was told about.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'sheet.js'), 'utf8');

class FakeEl {
  constructor(id) {
    this.id = id || '';
    this.attrs = {};
    this.listeners = {};
    this.props = {};
    this.style = {
      setProperty: (name, value) => { this.props[name] = value; },
      removeProperty: (name) => { delete this.props[name]; }
    };
    this.children = {};
    this.clientHeight = 719;
    this.scrollTop = 0;
  }
  setAttribute(name, value) { this.attrs[name] = String(value); }
  removeAttribute(name) { delete this.attrs[name]; }
  getAttribute(name) { return Object.hasOwn(this.attrs, name) ? this.attrs[name] : null; }
  hasAttribute(name) { return Object.hasOwn(this.attrs, name); }
  addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
  removeEventListener(type, fn) {
    this.listeners[type] = (this.listeners[type] || []).filter((f) => f !== fn);
  }
  querySelector(sel) { return this.children[sel] || null; }
  get offsetHeight() { return 1; }
  fire(type, evt) { (this.listeners[type] || []).slice().forEach((fn) => fn(evt)); }
}

function setup({ phone = true, reduced = false, height = 719 } = {}) {
  const box = new FakeEl('app-shell');
  box.clientHeight = height;
  const sheet = new FakeEl('report-sheet');
  const handle = new FakeEl('sheet-handle');
  const row = new FakeEl('');
  const head = new FakeEl('');
  const showButton = new FakeEl('show-button');
  sheet.children['.sheet__handle-row'] = row;
  sheet.children['.list-head'] = head;
  Object.defineProperty(sheet, 'offsetTop', {
    get: () => parseFloat(box.props['--sheet-top'] || '0')
  });
  const byId = { 'app-shell': box, 'report-sheet': sheet, 'sheet-handle': handle, 'show-button': showButton };
  const timers = [];
  const resized = [];
  const clock = { t: 1000 };
  const docListeners = new FakeEl('document');
  const queries = { phone, reduced };
  const document = {
    readyState: 'complete',
    getElementById: (id) => byId[id] || null,
    addEventListener: (t, f) => docListeners.addEventListener(t, f),
    removeEventListener: (t, f) => docListeners.removeEventListener(t, f)
  };
  const mq = (matches) => ({ matches, addEventListener() {} });
  const window = {
    matchMedia: (q) => mq(q.includes('prefers-reduced-motion') ? queries.reduced : queries.phone),
    performance: { now: () => clock.t },
    requestAnimationFrame: (fn) => { fn(); return 1; },
    setTimeout: (fn) => { timers.push(fn); return timers.length; },
    clearTimeout: (id) => { if (id > 0) { timers[id - 1] = null; } },
    addEventListener() {},
    PinalertMap: { resizeToSheet: (sheetTop) => resized.push(sheetTop) }
  };
  const context = vm.createContext({ window, document, Date });
  vm.runInContext(source, context, { filename: 'sheet.js' });
  const fireTimers = () => {
    const pending = timers.splice(0).filter(Boolean);
    pending.forEach((fn) => fn());
  };
  return { box, sheet, handle, row, head, showButton, api: window.PinalertSheet, resized, fireTimers, docListeners, clock };
}

const top = (env) => env.box.props['--sheet-top'];
const evt = (extra = {}) => ({
  target: { closest: () => null },
  cancelable: true,
  defaultPrevented: false,
  preventDefault() { this.defaultPrevented = true; },
  stopPropagation() { this.stopped = true; },
  ...extra
});

test('on a phone the sheet starts at half with the handle named to show more', () => {
  const env = setup();
  assert.equal(top(env), '324px');
  assert.equal(env.sheet.getAttribute('data-snap'), 'half');
  assert.equal(env.handle.getAttribute('aria-expanded'), 'false');
  assert.equal(env.handle.getAttribute('aria-label'), 'Show more of the list');
  assert.deepEqual(env.resized, [324], 'the map was told the sheet settled');
});

test('the handle toggles half and full, and aria-expanded is true only at full', () => {
  const env = setup();
  env.handle.fire('click', evt());
  assert.equal(top(env), '0px');
  assert.equal(env.sheet.getAttribute('data-snap'), 'full');
  assert.equal(env.handle.getAttribute('aria-expanded'), 'true');
  assert.equal(env.handle.getAttribute('aria-label'), 'Show the map again');
  env.handle.fire('click', evt());
  assert.equal(top(env), '324px');
  assert.equal(env.handle.getAttribute('aria-expanded'), 'false');
  assert.equal(env.handle.getAttribute('aria-label'), 'Show more of the list');
});

test('a short sheet opens to full from the handle, and the arrow keys step through the three positions', () => {
  const env = setup();
  const key = (k) => { const e = evt({ key: k }); env.handle.fire('keydown', e); return e; };
  assert.equal(key('ArrowDown').defaultPrevented, true);
  assert.equal(top(env), '511px');
  assert.equal(env.sheet.getAttribute('data-snap'), 'short');
  env.handle.fire('click', evt());
  assert.equal(top(env), '0px', 'a tap on a short sheet goes to full');
  key('ArrowDown');
  assert.equal(top(env), '324px');
  key('ArrowUp');
  key('ArrowUp');
  assert.equal(top(env), '0px', 'ArrowUp stops at full');
  assert.equal(key('Tab').defaultPrevented, false, 'other keys are left alone');
});

test('the move eases and the map is measured when it settles; reduced motion settles at once', () => {
  const env = setup();
  env.resized.length = 0;
  env.api.setSnap('short');
  assert.equal(env.resized.length, 0, 'the map waits for the ease to end');
  assert.equal(env.box.hasAttribute('data-dragging'), false, 'the ease is on');
  env.fireTimers();
  assert.deepEqual(env.resized, [511]);

  const calm = setup({ reduced: true });
  calm.resized.length = 0;
  calm.api.setSnap('short');
  assert.deepEqual(calm.resized, [511], 'no ease, no wait');
  assert.equal(calm.box.hasAttribute('data-dragging'), false, 'the no-ease mark is taken off again');
});

test('remember and restore bring the sheet back after the detail closes, keeping the first position', () => {
  const env = setup();
  env.api.remember();
  env.api.setSnap('short');
  env.api.remember();
  assert.equal(top(env), '511px');
  env.api.restore();
  assert.equal(top(env), '324px', 'the first remembered position, not short');
  env.api.restore();
  assert.equal(top(env), '324px', 'a second restore with nothing remembered does nothing');

  env.api.setSnap('full');
  env.api.remember();
  env.api.setSnap('short');
  env.api.restore();
  assert.equal(env.api.getSnap(), 'full');
});

test('on a laptop the sheet is idle: no top is written and nothing moves', () => {
  const env = setup({ phone: false });
  assert.equal(top(env), undefined);
  assert.equal(env.api.setSnap('full'), 'half');
  env.api.remember();
  env.api.setSnap('short');
  env.api.restore();
  assert.equal(top(env), undefined);
  assert.deepEqual(env.resized, []);
  env.handle.fire('click', evt());
  assert.equal(top(env), undefined);
});

test('a drag on the handle row follows the finger and lets go on the nearest position', () => {
  const env = setup();
  env.row.fire('pointerdown', evt({ clientY: 300, pointerId: 1, isPrimary: true, button: 0 }));
  env.docListeners.fire('pointermove', evt({ clientY: 298, pointerId: 1 }));
  assert.equal(top(env), '324px', 'under 6px is still a tap');
  assert.equal(env.box.hasAttribute('data-dragging'), false);
  const m = evt({ clientY: 250, pointerId: 1 });
  env.docListeners.fire('pointermove', m);
  assert.equal(m.defaultPrevented, true);
  assert.equal(top(env), '274px');
  assert.equal(env.box.hasAttribute('data-dragging'), true, 'nothing eases while a finger drags');
  env.docListeners.fire('pointermove', evt({ clientY: -900, pointerId: 1 }));
  assert.equal(top(env), '0px', 'the sheet stops at the top of the box');
  env.docListeners.fire('pointerup', evt({ pointerId: 1 }));
  assert.equal(env.box.hasAttribute('data-dragging'), false);
  assert.equal(env.sheet.getAttribute('data-snap'), 'full');
  assert.equal(top(env), '0px');
});

test('a flick from half moves one position even when it ends nearest to half', () => {
  const env = setup();
  env.head.fire('pointerdown', evt({ clientY: 300, pointerId: 2, isPrimary: true, button: 0 }));
  env.docListeners.fire('pointermove', evt({ clientY: 340, pointerId: 2 }));
  env.docListeners.fire('pointerup', evt({ pointerId: 2 }));
  assert.equal(env.sheet.getAttribute('data-snap'), 'short', 'a 40px pull down is a flick to short');
});

test('the click that follows a drag is swallowed, a plain tap is not', () => {
  const env = setup();
  env.row.fire('pointerdown', evt({ clientY: 300, pointerId: 3, isPrimary: true, button: 0 }));
  env.docListeners.fire('pointermove', evt({ clientY: 200, pointerId: 3 }));
  env.docListeners.fire('pointerup', evt({ pointerId: 3 }));
  const swallowed = evt();
  env.sheet.fire('click', swallowed);
  assert.equal(swallowed.stopped, true);
  assert.equal(swallowed.defaultPrevented, true);

  env.clock.t += 500;
  env.row.fire('pointerdown', evt({ clientY: 100, pointerId: 4, isPrimary: true, button: 0 }));
  env.docListeners.fire('pointerup', evt({ pointerId: 4 }));
  const tap = evt();
  env.sheet.fire('click', tap);
  assert.equal(tap.stopped, undefined);
  assert.equal(tap.defaultPrevented, false);
});

test('a press on an item of the Show menu never starts a drag', () => {
  const env = setup();
  const inMenu = evt({ clientY: 300, pointerId: 5, isPrimary: true, button: 0, target: { closest: (s) => (s === '.show-menu' ? {} : null) } });
  env.head.fire('pointerdown', inMenu);
  env.docListeners.fire('pointermove', evt({ clientY: 100, pointerId: 5 }));
  assert.equal(top(env), '324px');
});

const touch = (y) => ({ touches: [{ clientY: y }] });

test('a finger up on the list at half belongs to the sheet and takes the gesture from the browser', () => {
  const env = setup();
  env.sheet.fire('touchstart', evt({ ...touch(500) }));
  const m = evt({ ...touch(440) });
  env.sheet.fire('touchmove', m);
  assert.equal(m.defaultPrevented, true);
  assert.equal(top(env), '264px');
  env.sheet.fire('touchend', evt());
  assert.equal(env.sheet.getAttribute('data-snap'), 'full');
});

test('at full a finger up scrolls the list, and the sheet is not touched', () => {
  const env = setup();
  env.api.setSnap('full', { instant: true });
  env.sheet.scrollTop = 80;
  env.sheet.fire('touchstart', evt({ ...touch(500) }));
  const m = evt({ ...touch(440) });
  env.sheet.fire('touchmove', m);
  assert.equal(m.defaultPrevented, false);
  env.sheet.fire('touchend', evt());
  assert.equal(top(env), '0px');
});

test('at full and the top of the list a finger down shrinks the sheet back to half', () => {
  const env = setup();
  env.api.setSnap('full', { instant: true });
  env.sheet.scrollTop = 0;
  env.sheet.fire('touchstart', evt({ ...touch(300) }));
  const m = evt({ ...touch(350) });
  env.sheet.fire('touchmove', m);
  assert.equal(m.defaultPrevented, true);
  env.sheet.fire('touchmove', evt({ ...touch(420) }));
  env.sheet.fire('touchend', evt());
  assert.equal(env.sheet.getAttribute('data-snap'), 'half');
});

test('a touch that starts on the handle row or the list head is left to the pointer drag', () => {
  const env = setup();
  env.sheet.fire('touchstart', evt({ ...touch(500), target: { closest: () => ({}) } }));
  const m = evt({ ...touch(400) });
  env.sheet.fire('touchmove', m);
  assert.equal(m.defaultPrevented, false);
  assert.equal(top(env), '324px');
});

test('focus on a row a short sheet clips grows it to half; the handle and Show button do not', () => {
  const env = setup();
  env.api.setSnap('short', { instant: true });
  env.sheet.fire('focusin', evt({ target: env.handle }));
  assert.equal(env.sheet.getAttribute('data-snap'), 'short');
  env.sheet.fire('focusin', evt({ target: env.showButton }));
  assert.equal(env.sheet.getAttribute('data-snap'), 'short');
  env.sheet.fire('focusin', evt({ target: new FakeEl('') }));
  assert.equal(env.sheet.getAttribute('data-snap'), 'half');
});

test('the sheet never reads or writes anything of a report', () => {
  const code = source.replace(/\/\/.*$/gm, '');
  assert.equal(/Pinalert\.(state|select|fetch)|\.reports\b|innerHTML|fetch\(/.test(code), false);
});
