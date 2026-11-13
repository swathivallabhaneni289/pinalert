// profile_edit_panel_test.mjs - runs the real web/static/js/profile-edit.js in a
// Node sandbox and checks what quick 261113-t4y (owner decision 2026-11-13, the
// compact profile) added to it: the Edit profile button, the edit panel with its
// three rows and Done, and the header that shows what a window saved without a
// reload.
//
//   - Edit profile opens the panel, marks the header as editing, sets
//     aria-expanded and puts focus on the first row;
//   - Done and Escape inside the panel hide it again and return focus to Edit
//     profile; an Escape that closes a window leaves the panel open;
//   - a saved name, home area and username are drawn at once: the name line and
//     the details line show them, the heading, the phone line, the letter, the
//     page's data-username, the address and the title follow a new username, and
//     the username-changed event tells person.js;
//   - every value reaches the page through textContent.
//
// Run it with: node --test web/profile_edit_panel_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'profile-edit.js'), 'utf8');

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
    this.value = '';
    this.disabled = false;
    this.inert = false;
    const classes = new Set();
    this.classList = {
      add: (c) => { classes.add(c); },
      remove: (c) => { classes.delete(c); },
      contains: (c) => classes.has(c)
    };
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
  querySelectorAll() {
    const found = [];
    const walk = (node) => {
      node.children.forEach((c) => {
        if (c.tagName === 'BUTTON' || c.tagName === 'INPUT') { found.push(c); }
        walk(c);
      });
    };
    walk(this);
    return found;
  }
  focus() { this.doc.activeElement = this; }
}

// The markup of person.html.tmpl (the header, the panel) and
// profile_edit_windows.html.tmpl (the three windows), as the ids profile-edit.js reads.
function buildPage() {
  const doc = {
    activeElement: null,
    title: 'Pinalert: ravi.k',
    byId: {},
    phoneLine: null,
    events: [],
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector(selector) { return selector === '.person-username--phone' ? doc.phoneLine : null; },
    querySelectorAll() { return []; },
    dispatchEvent(evt) { doc.events.push(evt); return true; },
    addEventListener() {},
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
  const main = make('main', 'person-page', doc.body);
  main.setAttribute('data-username', 'ravi.k');
  const header = make('header', 'person-header', main);
  make('span', 'person-disc', header).textContent = 'R';
  make('h1', 'person-username', header).textContent = 'ravi.k';
  doc.phoneLine = new FakeEl(doc, 'p');
  doc.phoneLine.textContent = 'ravi.k';
  header.appendChild(doc.phoneLine);
  make('p', 'person-name', header).textContent = 'Ravi Kumar';
  const panel = make('div', 'person-edit-panel', header);
  panel.hidden = true;
  const usernameRow = make('button', 'settings-username-row', panel);
  usernameRow.setAttribute('data-value', 'ravi.k');
  make('span', 'settings-username-value', usernameRow).textContent = 'ravi.k';
  const nameRow = make('button', 'settings-name-row', panel);
  nameRow.setAttribute('data-value', 'Ravi Kumar');
  make('span', 'settings-name-value', nameRow).textContent = 'Ravi Kumar';
  const homeRow = make('button', 'settings-home-row', panel);
  homeRow.setAttribute('data-value', 'Benz Circle');
  make('span', 'settings-home-value', homeRow).textContent = 'Benz Circle';
  make('button', 'person-edit-done', panel);
  make('div', 'person-stats', header);
  const facts = make('p', 'person-facts', header);
  make('span', 'person-member-since', facts).textContent = 'Member since October 2026';
  make('span', 'person-facts-dot', facts).textContent = ' · ';
  make('span', 'person-home-area', facts).textContent = 'Benz Circle';
  const actions = make('div', 'person-actions', header);
  make('button', 'person-edit', actions);

  const usernameWin = make('div', 'settings-username-window', doc.body);
  usernameWin.hidden = true;
  const usernamePanel = make('div', '', usernameWin);
  make('button', 'settings-username-close', usernamePanel);
  const usernameForm = make('form', 'settings-username-form', usernamePanel);
  make('input', 'settings-username-input', usernameForm);
  make('p', 'settings-username-status', usernameForm);
  make('p', 'settings-username-error', usernameForm).hidden = true;
  make('button', 'settings-username-save', usernameForm);
  make('button', 'settings-username-ok', usernameForm).hidden = true;

  const nameWin = make('div', 'settings-name-window', doc.body);
  nameWin.hidden = true;
  const namePanel = make('div', '', nameWin);
  make('button', 'settings-name-close', namePanel);
  const nameForm = make('form', 'settings-name-form', namePanel);
  make('input', 'settings-name-input', nameForm);
  make('p', 'settings-name-error', nameForm).hidden = true;
  make('button', 'settings-name-save', nameForm);

  const homeWin = make('div', 'settings-home-window', doc.body);
  homeWin.hidden = true;
  const homePanel = make('div', '', homeWin);
  make('button', 'settings-home-close', homePanel);
  const homeForm = make('form', 'settings-home-form', homePanel);
  make('input', 'settings-home-input', homeForm);
  make('p', 'settings-home-status', homePanel).hidden = true;
  make('ul', 'settings-home-results', homePanel).hidden = true;
  make('p', 'settings-home-error', homePanel).hidden = true;
  make('button', 'settings-home-save', homePanel);

  return { doc, header, panel };
}

function load() {
  const page = buildPage();
  const requests = [];
  const replaced = [];
  class FakeAbortController {
    constructor() { this.signal = { aborted: false }; }
    abort() { this.signal.aborted = true; }
  }
  // The live check is its own script; a stub that always says the name is good
  // and reads the value of the field is enough for these tests.
  const check = {
    attach(input) {
      return {
        isGood: () => true,
        name: () => input.value,
        state: () => 'available',
        refresh() {}, cancel() {}, hold() {}, showSaveError() {}
      };
    }
  };
  const sandbox = {
    console,
    document: page.doc,
    PinalertUsernameCheck: check,
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    history: { state: null, replaceState(state, title, address) { replaced.push(address); } },
    fetch(target, init) {
      return new Promise((resolve, reject) => { requests.push({ url: String(target), init: init || {}, resolve, reject }); });
    },
    setTimeout(fn) { return 1; },
    clearTimeout() {},
    AbortController: FakeAbortController,
    navigator: { get geolocation() { throw new Error('profile-edit.js asked for the device position'); } },
    location: {}
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'profile-edit.js' });
  return { ...page, api: sandbox.PinalertProfileEdit, requests, replaced, sandbox };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };
const answer = (req, status, body) => req.resolve({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
const byId = (page, id) => page.doc.byId[id];
const fire = (el, type, extra) => {
  const event = { type, target: el, key: '', shiftKey: false, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};

test('Edit profile opens the panel, marks the header and focuses the first row', () => {
  const page = load();
  assert.equal(byId(page, 'person-edit-panel').hidden, true);
  fire(byId(page, 'person-edit'), 'click');
  assert.equal(byId(page, 'person-edit-panel').hidden, false);
  assert.equal(page.header.classList.contains('person-header--editing'), true);
  assert.equal(byId(page, 'person-edit').getAttribute('aria-expanded'), 'true');
  assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'));
});

test('Done hides the panel and returns focus to Edit profile', () => {
  const page = load();
  fire(byId(page, 'person-edit'), 'click');
  fire(byId(page, 'person-edit-done'), 'click');
  assert.equal(byId(page, 'person-edit-panel').hidden, true);
  assert.equal(page.header.classList.contains('person-header--editing'), false);
  assert.equal(byId(page, 'person-edit').getAttribute('aria-expanded'), 'false');
  assert.equal(page.doc.activeElement, byId(page, 'person-edit'));
});

test('Escape inside the panel does what Done does', () => {
  const page = load();
  fire(byId(page, 'person-edit'), 'click');
  const evt = fire(page.panel, 'keydown', { key: 'Escape' });
  assert.equal(evt.prevented, true);
  assert.equal(page.panel.hidden, true);
  assert.equal(page.doc.activeElement, byId(page, 'person-edit'));
});

test('a window opened from a row leaves the panel open when it closes, focus back on its row', () => {
  const page = load();
  fire(byId(page, 'person-edit'), 'click');
  fire(byId(page, 'settings-name-row'), 'click');
  assert.equal(byId(page, 'settings-name-window').hidden, false);
  assert.equal(byId(page, 'settings-name-input').value, 'Ravi Kumar');
  fire(byId(page, 'settings-name-window'), 'keydown', { key: 'Escape' });
  assert.equal(byId(page, 'settings-name-window').hidden, true);
  assert.equal(page.panel.hidden, false, 'the panel stays open');
  assert.equal(page.doc.activeElement, byId(page, 'settings-name-row'));
});

test('a saved name is drawn under the username at once', async () => {
  const page = load();
  fire(byId(page, 'person-edit'), 'click');
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi K Kumar';
  fire(byId(page, 'settings-name-form'), 'submit');
  await flush();
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.body, '{"display_name":"Ravi K Kumar"}');
  answer(page.requests[0], 200, { display_name: 'Ravi K Kumar' });
  await flush();
  assert.equal(byId(page, 'person-name').textContent, 'Ravi K Kumar');
  assert.equal(byId(page, 'person-name').hidden, false);
  assert.equal(byId(page, 'settings-name-value').textContent, 'Ravi K Kumar');
});

test('a saved home area is drawn on the details line with its dot', async () => {
  const page = load();
  byId(page, 'person-home-area').hidden = true;
  byId(page, 'person-facts-dot').hidden = true;
  fire(byId(page, 'person-edit'), 'click');
  fire(byId(page, 'settings-home-row'), 'click');
  byId(page, 'settings-home-input').value = 'Gayathri';
  fire(byId(page, 'settings-home-form'), 'submit');
  await flush();
  assert.equal(page.requests.length, 1, 'the place search');
  answer(page.requests[0], 200, { results: [{ name: 'Gayathri Nagar', display_name: 'Gayathri Nagar, Vijayawada', lat: 16.52, lon: 80.62 }] });
  await flush();
  const option = byId(page, 'settings-home-results').children[0].children[0];
  fire(option, 'click');
  fire(byId(page, 'settings-home-save'), 'click');
  await flush();
  assert.equal(page.requests.length, 2, 'the save');
  answer(page.requests[1], 200, { home_area_name: 'Gayathri Nagar' });
  await flush();
  assert.equal(byId(page, 'person-home-area').textContent, 'Gayathri Nagar');
  assert.equal(byId(page, 'person-home-area').hidden, false);
  assert.equal(byId(page, 'person-facts-dot').hidden, false);
  assert.equal(byId(page, 'settings-home-value').textContent, 'Gayathri Nagar');
});

test('a saved username redraws the heading, the phone line, the letter, the address and the title', async () => {
  const page = load();
  fire(byId(page, 'person-edit'), 'click');
  fire(byId(page, 'settings-username-row'), 'click');
  byId(page, 'settings-username-input').value = '3d.kumar';
  fire(byId(page, 'settings-username-form'), 'submit');
  await flush();
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.body, '{"username":"3d.kumar"}');
  answer(page.requests[0], 200, { username: '3d.kumar' });
  await flush();
  assert.equal(byId(page, 'person-username').textContent, '3d.kumar');
  assert.equal(page.doc.phoneLine.textContent, '3d.kumar');
  assert.equal(byId(page, 'person-disc').textContent, 'D', 'the first ASCII letter, upper case');
  assert.equal(byId(page, 'person-page').getAttribute('data-username'), '3d.kumar');
  assert.equal(page.doc.title, 'Pinalert: 3d.kumar');
  assert.deepEqual(page.replaced, ['/u/3d.kumar']);
  assert.equal(page.doc.events.length, 1);
  assert.equal(page.doc.events[0].type, 'pinalert:username-changed');
  assert.equal(page.doc.events[0].detail.username, '3d.kumar');
  assert.equal(byId(page, 'settings-username-value').textContent, '3d.kumar');
});

test('a refused save draws nothing on the header', async () => {
  const page = load();
  fire(byId(page, 'person-edit'), 'click');
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Someone Else';
  fire(byId(page, 'settings-name-form'), 'submit');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(byId(page, 'person-name').textContent, 'Ravi Kumar');
  assert.equal(page.replaced.length, 0);
});

test('the script writes no markup', () => {
  assert.equal(/\.innerHTML\s*=|insertAdjacentHTML|outerHTML\s*=|document\.write|DOMParser/.test(source), false);
});
