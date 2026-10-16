// reader_test.mjs - runs the real web/static/js/reader.js in a Node sandbox and
// checks the reader of the Disclaimer and the Policy (Phase 3.4, plan 08, D-31):
//
//   - in the app (the window), an opener with data-open-reader disclaimer opens
//     the window titled Disclaimer with the box at the top, and policy opens it
//     titled Policy with the Policy heading at the top of the box;
//   - the X, Escape and a click on the backdrop close it, focus moves to the
//     title on open and returns to the opener on close, Tab stays inside, and
//     the page behind is locked while it is open and unlocked after;
//   - a modified click (new tab) is left to the browser, so the plain link works;
//   - on the standalone page, the X goes back only when the referrer is this
//     origin, /policy opens with the Policy heading at the top of the box, and a
//     part link scrolls the box and moves the current mark;
//   - nothing is built through a markup parsing sink.
//
// Run it with: node --test web/reader_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what reader.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'reader.js'), 'utf8');

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
    this.inert = false;
    this.scrollTop = 0;
    this.top = 0;
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML must never be used: ' + v); }
  get hidden() { return Object.hasOwn(this.attrs, 'hidden'); }
  set hidden(v) { if (v) { this.attrs.hidden = ''; } else { delete this.attrs.hidden; } }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
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
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  // The box sits at the top of its window; a heading's top is its offset inside
  // the box content, minus how far the box has scrolled.
  getBoundingClientRect() { return { top: this.top - (this.scrollsWith ? this.scrollsWith.scrollTop : 0) }; }
  focus() { this.doc.activeElement = this; }
}

// The markup of reader_dialog.html.tmpl inside the app page, as the ids and
// nesting reader.js reads.
function buildApp() {
  const doc = {
    activeElement: null,
    byId: {},
    handlers: {},
    referrer: '',
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelectorAll() { return []; },
    addEventListener(type, fn) { (doc.handlers[type] = doc.handlers[type] || []).push(fn); },
    removeEventListener() {}
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  doc.documentElement = new FakeEl(doc, 'html');
  doc.body = make('body');
  const bar = make('div', 'top-chrome', doc.body);
  const main = make('div', 'app-content', doc.body);
  const opener = make('a', 'strip-link', main);
  opener.setAttribute('data-open-reader', 'disclaimer');
  const policyOpener = make('a', 'policy-link', main);
  policyOpener.setAttribute('data-open-reader', 'policy');
  const inner = make('span', 'policy-link-inner', policyOpener);

  const win = make('div', 'reader-window', doc.body);
  win.hidden = true;
  const panel = make('div', '', win);
  make('h2', 'reader-title', panel).textContent = 'Disclaimer';
  const close = make('button', 'reader-close', panel);
  const box = make('div', 'reader-box', panel);
  const disclaimerHead = make('h2', 'disclaimer', box);
  disclaimerHead.top = 0;
  const policyHead = make('h2', 'policy', box);
  policyHead.top = 900;
  policyHead.scrollsWith = box;
  const bottom = make('nav', 'bottom-bar', doc.body);
  return { doc, bar, main, bottom, opener, policyOpener, inner, win, close, box, policyHead };
}

function loadReader(options = {}) {
  const page = buildApp();
  const calls = [];
  const sandbox = {
    console,
    document: page.doc,
    location: { origin: 'https://pinalert.example', hash: '' },
    history: { length: 2, back() { calls.push('back'); } },
    URL
  };
  if (options.withApp) {
    sandbox.Pinalert = {
      lockPage(el) { calls.push('lock:' + el.id); },
      unlockPage() { calls.push('unlock'); }
    };
  }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'reader.js' });
  return { ...page, api: sandbox.PinalertReader, calls, sandbox };
}

const fire = (el, type, extra) => {
  const event = { type, target: el, key: '', shiftKey: false, ctrlKey: false, metaKey: false, button: 0, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};
// A click anywhere reaches the document handlers, as a bubbling click does.
const click = (page, el, extra) => {
  const event = { type: 'click', target: el, ctrlKey: false, metaKey: false, shiftKey: false, button: 0, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (page.doc.handlers.click || []).forEach((h) => h(event));
  return event;
};

test('a Disclaimer opener opens the window titled Disclaimer with the box at the top', () => {
  const page = loadReader();
  page.box.scrollTop = 500;
  const event = click(page, page.opener);

  assert.equal(event.prevented, true);
  assert.equal(page.win.hidden, false);
  assert.equal(page.doc.byId['reader-title'].textContent, 'Disclaimer');
  assert.equal(page.box.scrollTop, 0);
  assert.equal(page.doc.activeElement, page.doc.byId['reader-title']);
  assert.equal(page.doc.byId['reader-title'].getAttribute('tabindex'), '-1');
});

test('a Policy opener opens the window titled Policy with the Policy heading at the top of the box', () => {
  const page = loadReader();
  // A click on a child of the link still finds the opener.
  const event = click(page, page.inner);

  assert.equal(event.prevented, true);
  assert.equal(page.win.hidden, false);
  assert.equal(page.doc.byId['reader-title'].textContent, 'Policy');
  assert.equal(page.box.scrollTop, 900, 'the box scrolls so the Policy heading is at its top');

  // Opening it again from a Disclaimer link starts at the top again.
  fire(page.close, 'click');
  click(page, page.opener);
  assert.equal(page.doc.byId['reader-title'].textContent, 'Disclaimer');
  assert.equal(page.box.scrollTop, 0);
});

test('the X closes the window, unlocks the page and returns focus to the opener', () => {
  const page = loadReader();
  click(page, page.opener);

  assert.equal(page.main.inert, true);
  assert.equal(page.bar.inert, true);
  assert.equal(page.bottom.inert, true);
  assert.equal(page.win.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), 'true');

  fire(page.close, 'click');
  assert.equal(page.win.hidden, true);
  assert.equal(page.main.inert, false);
  assert.equal(page.bottom.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), null);
  assert.equal(page.doc.activeElement, page.opener);
});

test('Escape and a click on the backdrop close it; a click inside the panel does not', () => {
  const page = loadReader();
  click(page, page.policyOpener);

  fire(page.win, 'click', { target: page.box });
  assert.equal(page.win.hidden, false, 'a click inside the window must not close it');

  fire(page.win, 'click', { target: page.win });
  assert.equal(page.win.hidden, true, 'a click on the backdrop closes the reader (D-31)');
  assert.equal(page.doc.activeElement, page.policyOpener);

  click(page, page.opener);
  const esc = fire(page.win, 'keydown', { key: 'Escape' });
  assert.equal(esc.prevented, true);
  assert.equal(page.win.hidden, true);
  assert.equal(page.doc.activeElement, page.opener);
});

test('Tab stays inside the window', () => {
  const page = loadReader();
  click(page, page.opener);

  page.box.focus();
  const forward = fire(page.win, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.equal(page.doc.activeElement, page.close);

  page.close.focus();
  const back = fire(page.win, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.equal(page.doc.activeElement, page.box);

  page.doc.byId['reader-title'].focus();
  const fromTitle = fire(page.win, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(fromTitle.prevented, true);
  assert.equal(page.doc.activeElement, page.box);
});

test('a modified click is left to the browser so the plain link still opens in a new tab', () => {
  const page = loadReader();
  for (const extra of [{ ctrlKey: true }, { metaKey: true }, { shiftKey: true }, { button: 1 }]) {
    const event = click(page, page.opener, extra);
    assert.equal(event.prevented, false);
    assert.equal(page.win.hidden, true);
  }
});

test('a click that is not on an opener does nothing', () => {
  const page = loadReader();
  const event = click(page, page.main);
  assert.equal(event.prevented, false);
  assert.equal(page.win.hidden, true);
});

test('the page lock of the app is used when present', () => {
  const page = loadReader({ withApp: true });
  click(page, page.opener);
  fire(page.close, 'click');
  assert.deepEqual(page.calls, ['lock:reader-window', 'unlock']);
  assert.equal(page.main.inert, false, 'the local lock stays out of the way when the app lock exists');
});

test('PinalertReader.open and close work from script', () => {
  const page = loadReader();
  page.api.open('policy', page.opener);
  assert.equal(page.win.hidden, false);
  assert.equal(page.doc.byId['reader-title'].textContent, 'Policy');
  page.api.close();
  assert.equal(page.win.hidden, true);
  assert.equal(page.doc.activeElement, page.opener);
});

// ---- The standalone page ------------------------------------------------------

function buildStandalone(current) {
  const doc = {
    activeElement: null,
    byId: {},
    handlers: {},
    referrer: '',
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelectorAll() { return []; },
    addEventListener(type, fn) { (doc.handlers[type] = doc.handlers[type] || []).push(fn); },
    removeEventListener() {}
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  doc.documentElement = new FakeEl(doc, 'html');
  doc.body = make('body');
  const root = make('div', 'reader-page', doc.body);
  root.setAttribute('data-reader-page', current);
  const close = make('a', 'reader-close-link', root);
  const partDisclaimer = make('a', 'reader-part-disclaimer', root);
  const partPolicy = make('a', 'reader-part-policy', root);
  (current === 'policy' ? partPolicy : partDisclaimer).setAttribute('aria-current', 'page');
  const box = make('main', 'reader-box', root);
  make('h2', 'disclaimer', box).top = 0;
  const policyHead = make('h2', 'policy', box);
  policyHead.top = 1200;
  policyHead.scrollsWith = box;
  return { doc, root, close, partDisclaimer, partPolicy, box, policyHead };
}

function loadStandalone(current, { referrer = '', hash = '', length = 2 } = {}) {
  const page = buildStandalone(current);
  page.doc.referrer = referrer;
  const calls = [];
  const sandbox = {
    console,
    document: page.doc,
    location: { origin: 'https://pinalert.example', hash },
    history: { length, back() { calls.push('back'); } },
    URL
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'reader.js' });
  return { ...page, calls, sandbox };
}

test('standalone: /policy opens with the Policy heading at the top of the box', () => {
  const page = loadStandalone('policy');
  assert.equal(page.box.scrollTop, 1200);
});

test('standalone: /disclaimer starts at the top, unless the address says #policy', () => {
  assert.equal(loadStandalone('disclaimer').box.scrollTop, 0);
  assert.equal(loadStandalone('disclaimer', { hash: '#policy' }).box.scrollTop, 1200);
});

test('standalone: the X goes back only for a referrer of this origin', () => {
  const same = loadStandalone('disclaimer', { referrer: 'https://pinalert.example/settings' });
  const event = fire(same.close, 'click');
  assert.equal(event.prevented, true);
  assert.deepEqual(same.calls, ['back']);

  const foreign = loadStandalone('disclaimer', { referrer: 'https://elsewhere.example/page' });
  const away = fire(foreign.close, 'click');
  assert.equal(away.prevented, false, 'a foreign referrer keeps the plain link to /');
  assert.deepEqual(foreign.calls, []);

  const none = loadStandalone('disclaimer', { referrer: '' });
  assert.equal(fire(none.close, 'click').prevented, false);

  const broken = loadStandalone('disclaimer', { referrer: 'not a url' });
  assert.equal(fire(broken.close, 'click').prevented, false);

  const alone = loadStandalone('disclaimer', { referrer: 'https://pinalert.example/', length: 1 });
  assert.equal(fire(alone.close, 'click').prevented, false, 'no history to go back to');
});

test('standalone: a part link scrolls the box and moves the current mark', () => {
  const page = loadStandalone('disclaimer');
  const toPolicy = fire(page.partPolicy, 'click');
  assert.equal(toPolicy.prevented, true);
  assert.equal(page.box.scrollTop, 1200);
  assert.equal(page.partPolicy.getAttribute('aria-current'), 'page');
  assert.equal(page.partDisclaimer.getAttribute('aria-current'), null);

  fire(page.partDisclaimer, 'click');
  assert.equal(page.box.scrollTop, 0);
  assert.equal(page.partDisclaimer.getAttribute('aria-current'), 'page');
  assert.equal(page.partPolicy.getAttribute('aria-current'), null);
});

test('a page with neither the window nor the standalone reader leaves quietly', () => {
  const doc = { getElementById() { return null; }, addEventListener() { throw new Error('no listener expected'); } };
  const sandbox = { console, document: doc, location: { origin: 'https://pinalert.example', hash: '' }, history: { length: 1 }, URL };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'reader.js' });
  assert.equal(sandbox.PinalertReader, undefined);
});
