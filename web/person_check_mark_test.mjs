// person_check_mark_test.mjs - runs the real web/static/js/person-reports.js in
// a Node sandbox and checks the Being checked mark of a profile's live row
// (quick 261118-saj): the server draws it inside the row's link, a mouse
// resting on it shows "Being checked." and leaving hides it, a tap shows it
// instead of opening the report until a second tap, Escape or a press
// elsewhere, and at most one line is open.
//
// Run it with: node --test web/person_check_mark_test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'person-reports.js'), 'utf8');

class El {
  constructor(className) {
    this.className = className;
    this.children = [];
    this.parentNode = null;
    this.attrs = {};
    this.handlers = {};
    this.hidden = false;
    this.textContent = '';
  }
  append(child) { this.children.push(child); child.parentNode = this; return child; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  querySelector(sel) {
    const cls = sel.replace(/^\./, '');
    for (const c of this.children) {
      if (c.className === cls) { return c; }
      const deep = c.querySelector(sel);
      if (deep) { return deep; }
    }
    return null;
  }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  // dispatch runs the handlers here and up to the link, which counts an open
  // unless a handler prevented the default.
  dispatch(type, init) {
    const evt = Object.assign({ type, target: this, defaultPrevented: false, stopped: false,
      preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; } }, init || {});
    for (let n = this; n; n = n.parentNode) {
      for (const fn of (n.handlers[type] || [])) { fn(evt); }
      if (evt.stopped) { break; }
    }
    return evt;
  }
}

// row draws one live row as the template does: a link holding the label line
// with the mark (its glyph, its hidden words and its line, hidden).
function row() {
  const link = new El('person-report__body');
  const label = link.append(new El('person-report__label'));
  const mark = label.append(new El('person-report__check'));
  mark.setAttribute('data-check-mark', '');
  mark.append(new El('person-report__check-glyph'));
  const words = mark.append(new El('visually-hidden'));
  words.textContent = 'Being checked.';
  const tip = mark.append(new El('person-report__check-tip'));
  tip.textContent = 'Being checked.';
  tip.hidden = true;
  tip.setAttribute('aria-hidden', 'true');
  const opens = { n: 0 };
  link.addEventListener('click', (evt) => { if (!evt.defaultPrevented) { opens.n += 1; } });
  return { link, mark, tip, opens };
}

function load(rows) {
  const docHandlers = {};
  const body = new El('body');
  rows.forEach((r) => body.append(r.link));
  const document = {
    readyState: 'complete',
    querySelector: (sel) => (sel === '.person-report__distance' ? {} : null),
    querySelectorAll: (sel) => (sel === '[data-check-mark]' ? rows.map((r) => r.mark) : []),
    addEventListener(type, fn) { (docHandlers[type] = docHandlers[type] || []).push(fn); }
  };
  const window = { PinalertPosition: { read: () => null } };
  vm.runInNewContext(source, { document, window, Math, Number, isFinite });
  const fire = (type, init) => { for (const fn of (docHandlers[type] || [])) { fn(Object.assign({ type }, init)); } };
  return { body, fire };
}

test('a mouse resting on the mark shows the line and leaving hides it', () => {
  const r = row();
  load([r]);
  r.mark.dispatch('pointerenter', { pointerType: 'mouse' });
  assert.equal(r.tip.hidden, false);
  assert.equal(r.tip.textContent, 'Being checked.');
  r.mark.dispatch('pointerleave', { pointerType: 'mouse' });
  assert.equal(r.tip.hidden, true);
});

test('a tap shows the line instead of opening the report, and a second tap closes it', () => {
  const r = row();
  load([r]);
  r.mark.dispatch('pointerenter', { pointerType: 'touch' });
  assert.equal(r.tip.hidden, true, 'a touch pointer entering shows nothing');
  r.mark.dispatch('click');
  assert.equal(r.tip.hidden, false);
  assert.equal(r.opens.n, 0, 'the link did not open');
  r.mark.dispatch('click');
  assert.equal(r.tip.hidden, true);
  assert.equal(r.opens.n, 0);
  // A press on the row outside the mark still opens the report.
  r.link.dispatch('click');
  assert.equal(r.opens.n, 1);
});

test('Escape and a press elsewhere close it; a press on the mark does not', () => {
  const r = row();
  const env = load([r]);
  r.mark.dispatch('click');
  env.fire('pointerdown', { target: r.tip });
  assert.equal(r.tip.hidden, false);
  env.fire('pointerdown', { target: env.body });
  assert.equal(r.tip.hidden, true);
  r.mark.dispatch('click');
  env.fire('keydown', { key: 'Tab' });
  assert.equal(r.tip.hidden, false);
  env.fire('keydown', { key: 'Escape' });
  assert.equal(r.tip.hidden, true);
});

test('at most one line is open, and the words stay in the link for a screen reader', () => {
  const a = row();
  const b = row();
  load([a, b]);
  a.mark.dispatch('click');
  b.mark.dispatch('click');
  assert.equal(a.tip.hidden, true);
  assert.equal(b.tip.hidden, false);
  assert.equal(a.mark.querySelector('.visually-hidden').textContent, 'Being checked.');
  assert.equal(a.tip.getAttribute('aria-hidden'), 'true', 'the line is not read a second time');
});
