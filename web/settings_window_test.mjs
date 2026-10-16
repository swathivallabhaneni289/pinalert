// settings_window_test.mjs - runs the real web/static/js/settings.js in a Node
// sandbox and checks the Name and Home area edit windows of the Settings page
// (Phase 3.4, plan 06, D-24b, D-02):
//
//   - a window opens with the stored value, moves focus inside, keeps Tab
//     inside, and closes with the X or Escape only, returning focus to its row;
//   - Save name sends PATCH /api/me/profile with only display_name; an emptied
//     name is sent as an empty string; a 400 keeps the window open with the
//     server's sentence;
//   - the Home area window offers only places from the place search (one GET,
//     no map centre, no device position), Save home area sends only the picked
//     place, Remove home area sends null, and a failed or slow search never
//     blocks the window;
//   - every string reaches the page through textContent.
//
// Run it with: node --test web/settings_window_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what settings.js calls: a minimal element model, fake
// timers the test moves by hand and a fetch the test answers by hand.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'settings.js'), 'utf8');

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
  // Every button and input below this element, which is all the focus trap asks for.
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

// The markup of settings.html.tmpl, as the ids and nesting settings.js reads.
function buildPage({ name = '', home = '' } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelectorAll() { return []; },
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
  const bar = make('div', 'top-chrome', doc.body);
  const main = make('div', 'app-content', doc.body);
  const bottom = make('nav', 'bottom-bar', doc.body);

  const nameRow = make('button', 'settings-name-row', main);
  nameRow.setAttribute('data-value', name);
  make('span', 'settings-name-value', nameRow).textContent = name || 'Not set';
  const homeRow = make('button', 'settings-home-row', main);
  homeRow.setAttribute('data-value', home);
  make('span', 'settings-home-value', homeRow).textContent = home || 'Not set';

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
  const homeSave = make('button', 'settings-home-save', homePanel);
  homeSave.disabled = true;
  const remove = make('button', 'settings-home-remove', homePanel);
  remove.hidden = !home;

  return { doc, bar, main, bottom, nameWin, homeWin };
}

// loadSettings evaluates settings.js against a page. fetch keeps every call
// for the test to answer. Any use of the device position throws.
function loadSettings(options) {
  const page = buildPage(options);
  const requests = [];
  let now = 0;
  let nextId = 1;
  const timers = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) { break; }
      const [id, timer] = due;
      now = timer.at;
      timers.delete(id);
      timer.fn();
    }
    now = end;
  };

  class FakeAbortController {
    constructor() { this.signal = { aborted: false }; }
    abort() { this.signal.aborted = true; }
  }

  const sandbox = {
    console,
    document: page.doc,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimeout(id) { timers.delete(id); },
    AbortController: FakeAbortController,
    navigator: {
      get geolocation() { throw new Error('settings.js asked for the device position'); }
    },
    location: {}
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'settings.js' });
  return { ...page, api: sandbox.PinalertSettings, requests, advance, sandbox };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: () => Promise.resolve(body)
});

const byId = (page, id) => page.doc.byId[id];
const fire = (el, type, extra) => {
  const event = { type, target: el, key: '', shiftKey: false, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};

const pick = (page, index) => {
  const list = byId(page, 'settings-home-results');
  const option = list.children[index].children[0];
  fire(option, 'click');
  return option;
};

test('opening the Name window fills the stored value, locks the page, and the X returns focus to the row', () => {
  const page = loadSettings({ name: 'Ravi Kumar' });
  fire(byId(page, 'settings-name-row'), 'click');

  assert.equal(page.nameWin.hidden, false);
  assert.equal(byId(page, 'settings-name-input').value, 'Ravi Kumar');
  assert.equal(page.doc.activeElement, byId(page, 'settings-name-input'));
  assert.equal(page.main.inert, true);
  assert.equal(page.bar.inert, true);
  assert.equal(page.bottom.inert, true);
  assert.equal(page.nameWin.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), 'true');

  fire(byId(page, 'settings-name-close'), 'click');
  assert.equal(page.nameWin.hidden, true);
  assert.equal(page.main.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), null);
  assert.equal(page.doc.activeElement, byId(page, 'settings-name-row'));
});

test('Escape closes a window, Tab stays inside, and a click on the backdrop does nothing', () => {
  const page = loadSettings();
  fire(byId(page, 'settings-name-row'), 'click');

  const close = byId(page, 'settings-name-close');
  const save = byId(page, 'settings-name-save');
  save.focus();
  const tab = fire(page.nameWin, 'keydown', { key: 'Tab' });
  assert.equal(tab.prevented, true);
  assert.equal(page.doc.activeElement, close);
  close.focus();
  const back = fire(page.nameWin, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.equal(page.doc.activeElement, save);

  fire(page.nameWin, 'click', { target: page.nameWin });
  assert.equal(page.nameWin.hidden, false, 'a click on the backdrop must not close the window (D-02)');

  fire(page.nameWin, 'keydown', { key: 'Escape' });
  assert.equal(page.nameWin.hidden, true);
  assert.equal(page.doc.activeElement, byId(page, 'settings-name-row'));
});

test('Save name sends only display_name, trimmed, and an emptied name is an empty string', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = '  Ravi Kumar  ';
  fire(byId(page, 'settings-name-form'), 'submit');

  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/me/profile');
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.equal(page.requests[0].init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(page.requests[0].init.body), { display_name: 'Ravi Kumar' });
  assert.equal(byId(page, 'settings-name-save').disabled, true, 'a second tap while saving must not send again');
  answer(page.requests[0], 200, { display_name: 'Ravi Kumar', home_area_name: null });
  await flush();

  assert.equal(byId(page, 'settings-name-value').textContent, 'Ravi Kumar');
  assert.equal(byId(page, 'settings-name-row').getAttribute('data-value'), 'Ravi Kumar');
  assert.equal(page.nameWin.hidden, true);
  assert.equal(byId(page, 'settings-name-save').disabled, false);

  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = '   ';
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.deepEqual(JSON.parse(page.requests[1].init.body), { display_name: '' });
  answer(page.requests[1], 200, { display_name: null, home_area_name: null });
  await flush();
  assert.equal(byId(page, 'settings-name-value').textContent, 'Not set');
  assert.equal(byId(page, 'settings-name-row').getAttribute('data-value'), '');
});

test('a refused name keeps the window open with the server sentence, and a network failure says so plainly', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  answer(page.requests[0], 400, { error: { field: 'display_name', message: 'Use letters, numbers and spaces in your name.' } });
  await flush();

  const error = byId(page, 'settings-name-error');
  assert.equal(page.nameWin.hidden, false);
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, 'Use letters, numbers and spaces in your name.');
  assert.equal(byId(page, 'settings-name-save').disabled, false);

  fire(byId(page, 'settings-name-form'), 'submit');
  page.requests[1].reject(new Error('offline'));
  await flush();
  assert.equal(page.nameWin.hidden, false);
  assert.equal(error.textContent, 'Could not save. Try again in a moment.');
  assert.equal(byId(page, 'settings-name-save').disabled, false);
});

test('the Home area search sends only q, shows up to 5 places, and a pick enables Save home area', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-home-row'), 'click');
  const input = byId(page, 'settings-home-input');
  const save = byId(page, 'settings-home-save');
  assert.equal(save.disabled, true);
  assert.equal(byId(page, 'settings-home-remove').hidden, true);

  input.value = 'Indira';
  fire(byId(page, 'settings-home-form'), 'submit');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/geocode/place?q=Indira');
  assert.equal(byId(page, 'settings-home-status').textContent, 'Searching…');

  const results = [];
  for (let i = 0; i < 7; i++) {
    results.push({ name: 'Place ' + i, display_name: 'Place ' + i + ', District, Karnataka, India', lat: 12.9 + i / 100, lon: 77.6 });
  }
  answer(page.requests[0], 200, { results });
  await flush();

  const list = byId(page, 'settings-home-results');
  assert.equal(list.hidden, false);
  assert.equal(list.children.length, 5);
  assert.equal(list.children[0].children[0].textContent.startsWith('Place 0'), true);
  assert.equal(save.disabled, true, 'nothing is picked yet');

  pick(page, 2);
  assert.equal(save.disabled, false);
  assert.equal(input.value, 'Place 2');
});

test('Save home area sends only the picked place, the row takes the stored name, and Remove appears', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-home-row'), 'click');
  byId(page, 'settings-home-input').value = 'Indira';
  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[0], 200, { results: [{ name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru, Karnataka, India', lat: 12.9784, lon: 77.6408 }] });
  await flush();
  pick(page, 0);

  fire(byId(page, 'settings-home-save'), 'click');
  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].url, '/api/me/profile');
  assert.equal(page.requests[1].init.method, 'PATCH');
  assert.deepEqual(JSON.parse(page.requests[1].init.body), { home_area: { name: 'Indiranagar', lat: 12.9784, lon: 77.6408 } });
  answer(page.requests[1], 200, { display_name: null, home_area_name: 'Indiranagar' });
  await flush();

  assert.equal(byId(page, 'settings-home-value').textContent, 'Indiranagar');
  assert.equal(byId(page, 'settings-home-row').getAttribute('data-value'), 'Indiranagar');
  assert.equal(byId(page, 'settings-home-remove').hidden, false);
  assert.equal(page.homeWin.hidden, true);
});

test('typing after a pick clears the pick and disables Save home area again', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-home-row'), 'click');
  const input = byId(page, 'settings-home-input');
  input.value = 'Indira';
  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[0], 200, { results: [{ name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru', lat: 12.9, lon: 77.6 }] });
  await flush();
  pick(page, 0);
  assert.equal(byId(page, 'settings-home-save').disabled, false);

  input.value = 'Indiranagar 2';
  fire(input, 'input');
  assert.equal(byId(page, 'settings-home-save').disabled, true);
  fire(byId(page, 'settings-home-save'), 'click');
  assert.equal(page.requests.length, 1, 'no save may go out without a picked place');
});

test('a failed, refused or slow search shows a plain line and never blocks the window', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-home-row'), 'click');
  const input = byId(page, 'settings-home-input');
  const status = byId(page, 'settings-home-status');

  input.value = 'ab';
  fire(byId(page, 'settings-home-form'), 'submit');
  assert.equal(page.requests.length, 0, 'under 3 letters sends nothing');
  assert.equal(status.textContent, 'Type at least 3 letters.');

  input.value = 'Bengal';
  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[0], 429, { error: { message: 'Too many searches.' } });
  await flush();
  assert.equal(status.hidden, false);
  assert.equal(status.textContent, 'Too many searches.');

  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[1], 503, {});
  await flush();
  assert.equal(status.textContent, 'Search unavailable. Try again in a moment.');

  fire(byId(page, 'settings-home-form'), 'submit');
  page.requests[2].reject(new Error('offline'));
  await flush();
  assert.equal(status.textContent, 'Search unavailable. Try again in a moment.');

  fire(byId(page, 'settings-home-form'), 'submit');
  page.advance(10000);
  await flush();
  assert.equal(status.textContent, 'Search unavailable. Try again in a moment.');

  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[4], 200, { results: [] });
  await flush();
  assert.equal(status.textContent, 'No matches found. Try a nearby town or landmark.');

  assert.equal(page.homeWin.hidden, false);
  assert.equal(input.disabled, false);
  fire(page.homeWin, 'keydown', { key: 'Escape' });
  assert.equal(page.homeWin.hidden, true, 'the X and Escape still close after a failed search');
});

test('an older search answer that arrives late is dropped', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-home-row'), 'click');
  const input = byId(page, 'settings-home-input');
  input.value = 'Mumbai';
  fire(byId(page, 'settings-home-form'), 'submit');
  input.value = 'Pune';
  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[1], 200, { results: [{ name: 'Pune', display_name: 'Pune, Maharashtra', lat: 18.5, lon: 73.8 }] });
  await flush();
  answer(page.requests[0], 200, { results: [{ name: 'Mumbai', display_name: 'Mumbai, Maharashtra', lat: 19.0, lon: 72.8 }] });
  await flush();

  const list = byId(page, 'settings-home-results');
  assert.equal(list.children.length, 1);
  assert.equal(list.children[0].children[0].textContent.startsWith('Pune'), true);
});

test('Remove home area sends home_area null, the row reads Not set and the button goes away', async () => {
  const page = loadSettings({ home: 'Indiranagar' });
  fire(byId(page, 'settings-home-row'), 'click');
  assert.equal(byId(page, 'settings-home-remove').hidden, false);
  assert.equal(byId(page, 'settings-home-input').value, 'Indiranagar');
  assert.equal(byId(page, 'settings-home-save').disabled, true, 'a stored name is not a picked place');

  fire(byId(page, 'settings-home-remove'), 'click');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.deepEqual(JSON.parse(page.requests[0].init.body), { home_area: null });
  answer(page.requests[0], 200, { display_name: null, home_area_name: null });
  await flush();

  assert.equal(byId(page, 'settings-home-value').textContent, 'Not set');
  assert.equal(byId(page, 'settings-home-row').getAttribute('data-value'), '');
  assert.equal(byId(page, 'settings-home-remove').hidden, true);
  assert.equal(byId(page, 'settings-home-input').value, '');
});

test('place names and server sentences reach the page as text only', async () => {
  const page = loadSettings();
  fire(byId(page, 'settings-home-row'), 'click');
  byId(page, 'settings-home-input').value = 'markup';
  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[0], 200, { results: [{ name: '<img src=x onerror=alert(1)>', display_name: '<b>district</b>', lat: 1, lon: 2 }] });
  await flush();

  const option = byId(page, 'settings-home-results').children[0].children[0];
  assert.equal(option.textContent.includes('<img src=x onerror=alert(1)>'), true);
  assert.equal(option.textContent.includes('<b>district</b>'), true);
  // The FakeEl innerHTML and outerHTML setters throw, so reaching here proves
  // neither was used. The device position getter would have thrown as well.
});

test('the script leaves quietly on a page without the windows', () => {
  const sandbox = { console, document: { getElementById() { return null; } }, window: {} };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'settings.js' });
  assert.equal(sandbox.PinalertSettings, undefined);
});
