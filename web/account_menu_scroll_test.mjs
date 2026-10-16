// account_menu_scroll_test.mjs - runs the real web/static/js/account-menu.js
// in a Node sandbox and checks browser finding F-P4-10: while the account menu
// is open, a wheel turn or a finger move over it that the menu cannot take
// itself is stopped, so the page behind does not scroll (WebKit chains it to
// the page despite overscroll-behavior: contain). A move the menu can take is
// left to the menu, and the guards are added on open and removed on close.
//
// Run it with: node --test web/account_menu_scroll_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what account-menu.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'account-menu.js'), 'utf8');

function fakeEl(id) {
  return {
    id,
    hidden: true,
    attrs: {},
    listeners: [],
    style: { setProperty() {} },
    scrollTop: 0,
    scrollHeight: 100,
    clientHeight: 100,
    addEventListener(type, fn, opts) { this.listeners.push({ type, fn, opts }); },
    removeEventListener(type, fn) { this.listeners = this.listeners.filter((l) => l.type !== type || l.fn !== fn); },
    setAttribute(name, value) { this.attrs[name] = value; },
    getAttribute(name) { return this.attrs[name]; },
    contains(node) { return node === this; },
    getBoundingClientRect() { return { top: 60 }; },
    querySelectorAll() { return []; },
    focus() {}
  };
}

function load() {
  const trigger = fakeEl('account-menu-trigger');
  const panel = fakeEl('account-menu');
  const document = {
    getElementById: (id) => (id === 'account-menu-trigger' ? trigger : id === 'account-menu' ? panel : null),
    addEventListener() {},
    removeEventListener() {},
    activeElement: null
  };
  const window = { innerHeight: 568, addEventListener() {}, removeEventListener() {} };
  vm.runInNewContext(source, { document, window });
  return { trigger, panel };
}

function open(m) {
  const click = m.trigger.listeners.find((l) => l.type === 'click');
  click.fn({ stopPropagation() {} });
  assert.equal(m.panel.hidden, false, 'the menu opens on the trigger');
}

function listener(panel, type) {
  return panel.listeners.find((l) => l.type === type);
}

function wheel(panel, deltaY, ctrlKey = false) {
  let prevented = false;
  listener(panel, 'wheel').fn({ deltaY, ctrlKey, preventDefault() { prevented = true; } });
  return prevented;
}

function touch(panel, type, y) {
  let prevented = false;
  listener(panel, type).fn({ touches: [{ clientY: y }], preventDefault() { prevented = true; } });
  return prevented;
}

test('a closed menu adds no scroll guard', () => {
  const m = load();
  for (const type of ['wheel', 'touchstart', 'touchmove']) {
    assert.equal(listener(m.panel, type), undefined, type);
  }
});

test('the guards are added on open, wheel and touchmove not passive, and removed on close', () => {
  const m = load();
  open(m);
  assert.equal(listener(m.panel, 'wheel').opts.passive, false);
  assert.equal(listener(m.panel, 'touchmove').opts.passive, false);
  assert.equal(listener(m.panel, 'touchstart').opts.passive, true);
  m.trigger.listeners.find((l) => l.type === 'click').fn({ stopPropagation() {} });
  assert.equal(m.panel.hidden, true, 'the trigger closes it again');
  for (const type of ['wheel', 'touchstart', 'touchmove']) {
    assert.equal(listener(m.panel, type), undefined, type);
  }
});

test('a menu with nothing to scroll stops every wheel turn and finger move over it', () => {
  const m = load();
  open(m);
  assert.equal(wheel(m.panel, 600), true, 'down');
  assert.equal(wheel(m.panel, -600), true, 'up');
  touch(m.panel, 'touchstart', 300);
  assert.equal(touch(m.panel, 'touchmove', 200), true, 'finger up');
  assert.equal(touch(m.panel, 'touchmove', 260), true, 'finger down');
});

test('a menu that scrolls takes the move itself and only its ends are stopped', () => {
  const m = load();
  open(m);
  m.panel.scrollHeight = 400;
  m.panel.clientHeight = 200;
  m.panel.scrollTop = 0;
  assert.equal(wheel(m.panel, 100), false, 'down from the top scrolls the menu');
  assert.equal(wheel(m.panel, -100), true, 'up at the top is stopped');
  m.panel.scrollTop = 100;
  assert.equal(wheel(m.panel, 100), false, 'down in the middle');
  assert.equal(wheel(m.panel, -100), false, 'up in the middle');
  m.panel.scrollTop = 200;
  assert.equal(wheel(m.panel, 100), true, 'down at the bottom is stopped');
  touch(m.panel, 'touchstart', 300);
  assert.equal(touch(m.panel, 'touchmove', 250), true, 'finger up at the bottom is stopped');
  assert.equal(touch(m.panel, 'touchmove', 300), false, 'finger down at the bottom scrolls the menu back up');
});

test('a ctrl wheel zoom and a sideways wheel are left alone', () => {
  const m = load();
  open(m);
  assert.equal(wheel(m.panel, 100, true), false, 'ctrl wheel');
  assert.equal(wheel(m.panel, 0), false, 'no vertical part');
});
