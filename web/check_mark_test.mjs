// check_mark_test.mjs - runs the real web/static/js/trust.js (with the real
// app.js) in a Node sandbox and checks the Being checked mark of quick
// 261118-saj (owner 2026-11-18):
//
//   - only being_checked true shows the mark; anything else hides it;
//   - the button is named "Being checked" and described by its one line,
//     "Being checked.", which starts hidden;
//   - a mouse resting on it shows the line and leaving hides it; a touch
//     pointer does not;
//   - a tap or click shows the line until a second tap, Escape or a press
//     anywhere else, and a press on the flag never reaches the row behind it;
//   - Enter and Space press it without opening the row; Escape closes the
//     line before anything else;
//   - at most one line is open on the page;
//   - every word is written as text.
//
// Run it with: node --test web/check_mark_test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const read = (name) => fs.readFileSync(path.join(here, 'static', 'js', name), 'utf8');

class FakeEl {
  constructor(doc, tag) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.nodeType = 1;
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.className = '';
    this.id = '';
    this.dataset = {};
    this.hidden = false;
    const el = this;
    this.classList = {
      add(c) { if (!el.className.split(/\s+/).includes(c)) { el.className = (el.className + ' ' + c).trim(); } },
      remove(c) { el.className = el.className.split(/\s+/).filter((x) => x && x !== c).join(' '); },
      contains(c) { return el.className.split(/\s+/).includes(c); }
    };
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  setAttribute(n, v) { if (n === 'hidden') { this.hidden = true; } this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { if (n === 'hidden') { this.hidden = false; } delete this.attrs[n]; }
  appendChild(node) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    this.children.push(node);
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
    for (let n = node; n; n = n.parentNode) {
      if (n === this) { return true; }
    }
    return false;
  }
  closest() { return null; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener() {}
  focus() { this.doc.activeElement = this; }
  // dispatch runs this element's handlers, then its ancestors', until one
  // stops the event. It returns whether the event got past this element.
  dispatch(type, init) {
    const evt = Object.assign({
      type, target: this, stopped: false, defaultPrevented: false,
      stopPropagation() { this.stopped = true; },
      preventDefault() { this.defaultPrevented = true; }
    }, init || {});
    for (let n = this; n; n = n.parentNode) {
      for (const fn of (n.handlers[type] || [])) { fn(evt); }
      if (evt.stopped) { break; }
    }
    return evt;
  }
}

function load() {
  const docHandlers = {};
  const doc = {
    createElement(tag) { return new FakeEl(doc, tag); },
    createElementNS(ns, tag) { return new FakeEl(doc, tag); },
    createTextNode(text) { return { nodeType: 3, textContent: String(text), parentNode: null }; },
    getElementById() { return null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener(type, fn, capture) { (docHandlers[type] = docHandlers[type] || []).push({ fn, capture }); },
    removeEventListener() {},
    documentElement: { hasAttribute() { return false; } },
    activeElement: null
  };
  doc.body = new FakeEl(doc, 'body');
  const window = {
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    location: { search: '' },
    matchMedia: () => ({ matches: false, addEventListener() {} })
  };
  const sandbox = {
    window,
    document: doc,
    navigator: {},
    fetch: () => new Promise(() => {}),
    URLSearchParams,
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent
  };
  vm.createContext(sandbox);
  vm.runInContext(read('app.js'), sandbox);
  sandbox.Pinalert = window.Pinalert;
  vm.runInContext(read('trust.js'), sandbox);
  // fire sends a document level event to the capture listeners, the way a
  // press or a key reaches the page before its target.
  const fire = (type, init) => {
    const evt = Object.assign({ type, stopped: false, defaultPrevented: false,
      stopPropagation() { this.stopped = true; }, preventDefault() { this.defaultPrevented = true; } }, init || {});
    for (const h of (docHandlers[type] || [])) { h.fn(evt); }
    return evt;
  };
  return { trust: window.PinalertTrust, doc, fire };
}

// mounted builds a mark inside a row that opens a report on click, Enter and
// Space, and returns both with a counter of row opens.
function mounted(env, report) {
  const row = new FakeEl(env.doc, 'li');
  const opens = { n: 0 };
  row.addEventListener('click', () => { opens.n += 1; });
  row.addEventListener('keydown', (evt) => {
    if (evt.key === 'Enter' || evt.key === ' ') { opens.n += 1; }
  });
  const mark = env.trust.createCheckMark();
  row.appendChild(mark.el);
  env.doc.body.appendChild(row);
  env.trust.updateCheckMark(mark, report === undefined ? { being_checked: true } : report);
  return { row, mark, opens };
}

const shown = (mark) => !mark.tip.hidden;

test('only being_checked true shows the mark', () => {
  const env = load();
  for (const [report, want] of [
    [{ being_checked: true }, true],
    [{ being_checked: false }, false],
    [{ being_checked: 'true' }, false],
    [{ being_checked: 1 }, false],
    [{}, false],
    [null, false]
  ]) {
    const { mark } = mounted(env, report);
    assert.equal(mark.el.hidden, !want, JSON.stringify(report));
  }
});

test('the button is named Being checked and described by its one line', () => {
  const { trust } = load();
  const mark = trust.createCheckMark();
  assert.equal(mark.el.hidden, true, 'a new mark starts hidden');
  assert.equal(mark.button.tagName, 'BUTTON');
  assert.equal(mark.button.type, 'button');
  assert.equal(mark.button.getAttribute('aria-label'), 'Being checked');
  assert.equal(mark.button.getAttribute('aria-describedby'), mark.tip.id);
  assert.equal(mark.button.getAttribute('aria-expanded'), 'false');
  assert.equal(mark.tip.getAttribute('role'), 'tooltip');
  assert.equal(mark.tip.textContent, 'Being checked.');
  assert.equal(mark.tip.hidden, true);
  // Text only: the line is one text node, no markup.
  assert.equal(mark.tip.children.length, 0);
  const other = trust.createCheckMark();
  assert.notEqual(other.tip.id, mark.tip.id, 'each line has its own id');
});

test('a mouse resting on the flag shows the line and leaving hides it', () => {
  const env = load();
  const { mark } = mounted(env);
  mark.button.dispatch('pointerenter', { pointerType: 'mouse' });
  assert.equal(shown(mark), true);
  assert.equal(mark.button.getAttribute('aria-expanded'), 'true');
  mark.button.dispatch('pointerleave', { pointerType: 'mouse' });
  assert.equal(shown(mark), false);
});

test('a touch pointer entering does not show the line; the tap does', () => {
  const env = load();
  const { mark, opens } = mounted(env);
  mark.button.dispatch('pointerenter', { pointerType: 'touch' });
  assert.equal(shown(mark), false);
  mark.button.dispatch('click');
  assert.equal(shown(mark), true);
  assert.equal(opens.n, 0, 'the tap never reaches the row');
  // Leaving with a touch pointer keeps it open.
  mark.button.dispatch('pointerleave', { pointerType: 'touch' });
  assert.equal(shown(mark), true);
  // A second tap closes it.
  mark.button.dispatch('click');
  assert.equal(shown(mark), false);
  assert.equal(opens.n, 0);
});

test('a click while hovering keeps the line until a second click', () => {
  const env = load();
  const { mark } = mounted(env);
  mark.button.dispatch('pointerenter', { pointerType: 'mouse' });
  mark.button.dispatch('click');
  assert.equal(shown(mark), true, 'the first click keeps it open');
  mark.button.dispatch('pointerleave', { pointerType: 'mouse' });
  assert.equal(shown(mark), true, 'a kept line stays after the mouse leaves');
  mark.button.dispatch('click');
  assert.equal(shown(mark), false);
});

test('a press anywhere else closes it, a press on the mark does not', () => {
  const env = load();
  const { mark } = mounted(env);
  mark.button.dispatch('click');
  env.fire('pointerdown', { target: mark.button });
  assert.equal(shown(mark), true);
  env.fire('pointerdown', { target: mark.tip });
  assert.equal(shown(mark), true);
  env.fire('pointerdown', { target: env.doc.body });
  assert.equal(shown(mark), false);
});

test('Escape closes the line first and is used up', () => {
  const env = load();
  const { mark } = mounted(env);
  mark.button.focus();
  mark.button.dispatch('click');
  const evt = env.fire('keydown', { key: 'Escape' });
  assert.equal(shown(mark), false);
  assert.equal(evt.stopped, true, 'the popup behind it stays open');
  // With no line open Escape is left to the others.
  const again = env.fire('keydown', { key: 'Escape' });
  assert.equal(again.stopped, false);
});

test('Enter and Space press the flag and never open the row', () => {
  const env = load();
  const { mark, opens } = mounted(env);
  for (const key of ['Enter', ' ']) {
    const evt = mark.button.dispatch('keydown', { key });
    assert.equal(evt.stopped, true, key);
    // A real button turns Enter and Space into a click.
    mark.button.dispatch('click');
    assert.equal(shown(mark), true, key + ' shows the line');
    mark.button.dispatch('click');
    assert.equal(shown(mark), false);
  }
  assert.equal(opens.n, 0);
  // Other keys pass to the row.
  const tab = mark.button.dispatch('keydown', { key: 'Tab' });
  assert.equal(tab.stopped, false);
});

test('at most one line is open on the page', () => {
  const env = load();
  const a = mounted(env).mark;
  const b = mounted(env).mark;
  a.button.dispatch('click');
  b.button.dispatch('click');
  assert.equal(shown(a), false);
  assert.equal(shown(b), true);
});

test('a refresh that drops the mark closes its line', () => {
  const env = load();
  const { mark } = mounted(env);
  mark.button.dispatch('click');
  env.trust.updateCheckMark(mark, { being_checked: false });
  assert.equal(mark.el.hidden, true);
  assert.equal(shown(mark), false);
  assert.equal(mark.button.getAttribute('aria-expanded'), 'false');
  assert.equal(env.trust.closeCheckLine(), false, 'nothing is left open');
});
