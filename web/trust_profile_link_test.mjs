// trust_profile_link_test.mjs - runs the real web/static/js/trust.js (with the
// real app.js) in a Node sandbox and checks PinalertTrust.setProfileLink, the
// "See this reporter" row of the trust block's explanation panel (plan 09-09,
// D-04, UI-SPEC 7d, T-09-35):
//
//   - a good path draws exactly one row as the last child of the panel, with
//     the visible words and the username as a hidden span;
//   - a second call with another path replaces the row, never adds one;
//   - an empty, missing or malformed path draws nothing and removes an earlier
//     row;
//   - the face of the block (the toggle) is unchanged;
//   - the popup card takes the row as its last child too;
//   - the module draws it with text nodes only.
//
// Run it with: node --test web/trust_profile_link_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

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
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  setAttribute(n, v) { if (n === 'hidden') { this.hidden = true; } this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
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
  remove() { if (this.parentNode) { this.parentNode.removeChild(this); } }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener() {}
  focus() {}
  hasClass(c) { return this.className.split(/\s+/).includes(c); }
}

function load() {
  const doc = {
    createElement(tag) { return new FakeEl(doc, tag); },
    createElementNS(ns, tag) { return new FakeEl(doc, tag); },
    createTextNode(text) { return { nodeType: 3, textContent: String(text), parentNode: null }; },
    getElementById() { return null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener() {},
    removeEventListener() {},
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
  return window.PinalertTrust;
}

const rows = (host) => host.children.filter((c) => c.tagName === 'A' && c.hasClass('trust-panel__profile'));
const last = (host) => host.children[host.children.length - 1];

test('a good path draws one row as the last line of the panel', () => {
  const trust = load();
  const block = trust.createTrustBlock();
  const faceBefore = block.toggle.children.length;
  trust.setProfileLink(block, '/u/ravi.k');

  assert.equal(rows(block.panel).length, 1);
  const row = last(block.panel);
  assert.equal(row.tagName, 'A');
  assert.equal(row.hasClass('trust-panel__profile'), true);
  assert.equal(row.getAttribute('href'), '/u/ravi.k');
  const hidden = row.children.find((c) => c.hasClass('visually-hidden'));
  assert.ok(hidden, 'a visually hidden username span');
  assert.equal(hidden.textContent, ' ravi.k');
  assert.equal(row.textContent, 'See this reporter ravi.k');
  assert.equal(row.children[0].textContent, 'See this reporter');
  assert.equal(row.children[row.children.length - 1].getAttribute('aria-hidden'), 'true');

  // Nothing was added to the face of the block.
  assert.equal(block.toggle.children.length, faceBefore);
  assert.equal(block.el.children.length, 2);
});

test('a second call replaces the row', () => {
  const trust = load();
  const block = trust.createTrustBlock();
  trust.setProfileLink(block, '/u/ravi.k');
  trust.setProfileLink(block, '/u/asha_7');

  assert.equal(rows(block.panel).length, 1);
  assert.equal(last(block.panel).getAttribute('href'), '/u/asha_7');
  assert.equal(last(block.panel).textContent, 'See this reporter asha_7');
  assert.equal(last(block.panel), block.profileLink);
});

test('an empty or malformed path draws nothing and removes an earlier row', () => {
  const trust = load();
  const bad = ['', null, undefined, 42, '/u/../x', 'https://x', '/u/A', '/u/ab', '/u/..', '/u/a b', '/u/x/y', '//u/abc', '/u/abc?x=1', 'javascript:alert(1)'];
  for (const p of bad) {
    const block = trust.createTrustBlock();
    trust.setProfileLink(block, p);
    assert.equal(rows(block.panel).length, 0, 'drew a row for ' + String(p));

    trust.setProfileLink(block, '/u/ravi.k');
    assert.equal(rows(block.panel).length, 1);
    trust.setProfileLink(block, p);
    assert.equal(rows(block.panel).length, 0, 'kept the earlier row for ' + String(p));
  }
});

test('a missing target is ignored', () => {
  const trust = load();
  assert.doesNotThrow(() => trust.setProfileLink(null, '/u/ravi.k'));
  assert.doesNotThrow(() => trust.setProfileLink(undefined, ''));
});

test('the popup card takes the row as its last child', () => {
  const trust = load();
  const card = trust.createTrustCard();
  trust.setProfileLink(card, '/u/ravi.k');
  assert.equal(rows(card.el).length, 1);
  assert.equal(last(card.el).getAttribute('href'), '/u/ravi.k');
  trust.setProfileLink(card, '');
  assert.equal(rows(card.el).length, 0);
});

test('the module uses no markup sink', () => {
  const src = read('trust.js');
  for (const sink of ['innerHTML', 'outerHTML', 'insertAdjacentHTML']) {
    assert.equal(src.includes(sink), false, sink);
  }
});
