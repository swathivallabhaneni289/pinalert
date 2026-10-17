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
        requests.push({ url: String(target), init: init || {}, resolve, reject, at: now });
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
  assert.equal(page.requests[0].url, '/api/geocode/place?kind=area&q=Indira');
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

// ---- Phase 3.4 plan 25 (code review part 2, WR-03): an answer belongs to the
// window session that sent it, and each window has its own save flag.

const HOME_PLACE = { name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru, Karnataka, India', lat: 12.9784, lon: 77.6408 };

// startHomeSave opens the Home area window, searches, picks the one place and
// presses Save. It returns the request of the save (the last one sent).
async function startHomeSave(page) {
  fire(byId(page, 'settings-home-row'), 'click');
  byId(page, 'settings-home-input').value = 'Indira';
  fire(byId(page, 'settings-home-form'), 'submit');
  answer(page.requests[page.requests.length - 1], 200, { results: [HOME_PLACE] });
  await flush();
  pick(page, 0);
  fire(byId(page, 'settings-home-save'), 'click');
  return page.requests[page.requests.length - 1];
}

test('WR-03: a name save that answers after the window was closed and opened again does not close it or touch what was typed', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.equal(page.requests.length, 1);

  fire(page.nameWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-name-row'), 'click');
  assert.equal(byId(page, 'settings-name-input').value, 'Old', 'the row is not updated yet, so the field shows the old name');
  byId(page, 'settings-name-input').value = 'Typing a new one';

  answer(page.requests[0], 200, { display_name: 'Ravi', home_area_name: null });
  await flush();

  assert.equal(page.nameWin.hidden, false, 'the window the person is typing in stays open');
  assert.equal(byId(page, 'settings-name-input').value, 'Typing a new one');
  assert.equal(byId(page, 'settings-name-value').textContent, 'Ravi', 'the row still takes the saved name');
  assert.equal(byId(page, 'settings-name-row').getAttribute('data-value'), 'Ravi');
  assert.equal(byId(page, 'settings-name-save').disabled, false, 'and its Save works again');
  assert.equal(page.main.inert, true, 'the page behind stays locked');
});

test('WR-03: pressing Save again in a reopened Name window while the first save is out sends nothing', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-name-row'), 'click');
  assert.equal(byId(page, 'settings-name-save').disabled, true, 'a save is still out');
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.equal(page.requests.length, 1, 'one PATCH only');

  answer(page.requests[0], 200, { display_name: 'Ravi', home_area_name: null });
  await flush();
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.equal(page.requests.length, 2, 'once it answered, a save goes out');
});

test('WR-03: a late refusal of an older name save shows nothing in the reopened window', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-name-row'), 'click');
  answer(page.requests[0], 400, { error: { field: 'display_name', message: 'Use letters, numbers and spaces in your name.' } });
  await flush();
  assert.equal(page.nameWin.hidden, false);
  assert.equal(byId(page, 'settings-name-error').hidden, true);
  assert.equal(byId(page, 'settings-name-save').disabled, false);
});

test('WR-03: a home save that answers after its window was reopened does not close it', async () => {
  const page = loadSettings();
  const save = await startHomeSave(page);
  fire(page.homeWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-home-row'), 'click');
  byId(page, 'settings-home-input').value = 'Something else';

  answer(save, 200, { display_name: null, home_area_name: 'Indiranagar' });
  await flush();

  assert.equal(page.homeWin.hidden, false, 'the reopened window stays open');
  assert.equal(byId(page, 'settings-home-input').value, 'Something else');
  assert.equal(byId(page, 'settings-home-value').textContent, 'Indiranagar', 'the row takes the saved place');
  assert.equal(byId(page, 'settings-home-remove').hidden, false);
  assert.equal(byId(page, 'settings-home-save').disabled, true, 'nothing is picked in the reopened window');
});

test('WR-03: the two windows do not share one save flag', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  const nameSave = page.requests[0];

  // The Name window is closed with its save still out, and the Home area
  // window saves a place and gets its answer first.
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  const homeSave = await startHomeSave(page);
  answer(homeSave, 200, { display_name: null, home_area_name: 'Indiranagar' });
  await flush();
  assert.equal(page.homeWin.hidden, true);

  // The name save has not answered yet: the Name window must still refuse a second save.
  const before = page.requests.length;
  fire(byId(page, 'settings-name-row'), 'click');
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.equal(page.requests.length, before, 'the home answer must not clear the name save flag');

  answer(nameSave, 200, { display_name: 'Ravi', home_area_name: 'Indiranagar' });
  await flush();
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.equal(page.requests.length, before + 1);
});

test('WR-03: a name save in flight does not stop a home save, and the other way round', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  const save = await startHomeSave(page);
  assert.equal(save.url, '/api/me/profile');
  assert.equal(save.init.method, 'PATCH');
  assert.deepEqual(JSON.parse(save.init.body).home_area.name, 'Indiranagar');
});

// ---- Phase 3.4 plan 27 (skeptic S-3): when a save from an earlier opening of a
// window answers after the window was closed and opened again, a reopened field
// that still holds the value it was filled with takes the newly saved value.

test('S-3: an earlier name save that lands in a reopened window refills the untouched field', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-name-row'), 'click');
  assert.equal(byId(page, 'settings-name-input').value, 'Old');
  assert.equal(byId(page, 'settings-name-save').disabled, true, 'a save is still out');

  answer(page.requests[0], 200, { display_name: 'Ravi', home_area_name: null });
  await flush();

  assert.equal(page.nameWin.hidden, false, 'the reopened window stays open');
  assert.equal(byId(page, 'settings-name-input').value, 'Ravi', 'the field shows the saved name, not the old one');
  assert.equal(byId(page, 'settings-name-save').disabled, false, 'nothing is out, so Save is worked out as available');
  // Pressing Save now sends the new name, never the old one back.
  fire(byId(page, 'settings-name-form'), 'submit');
  assert.equal(JSON.parse(page.requests[1].init.body).display_name, 'Ravi');
});

test('S-3: a name field the person already changed is left alone', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Typing a new one';

  answer(page.requests[0], 200, { display_name: 'Ravi', home_area_name: null });
  await flush();

  assert.equal(byId(page, 'settings-name-input').value, 'Typing a new one');
  assert.equal(byId(page, 'settings-name-save').disabled, false);
});

test('S-3: a refused earlier name save does not refill the field', async () => {
  const page = loadSettings({ name: 'Old' });
  fire(byId(page, 'settings-name-row'), 'click');
  byId(page, 'settings-name-input').value = 'Ravi';
  fire(byId(page, 'settings-name-form'), 'submit');
  fire(page.nameWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-name-row'), 'click');
  answer(page.requests[0], 400, { error: { field: 'display_name', message: 'Use letters, numbers and spaces in your name.' } });
  await flush();
  assert.equal(byId(page, 'settings-name-input').value, 'Old');
});

test('S-3: an earlier home save that lands in a reopened window refills the untouched field and works out the buttons', async () => {
  const page = loadSettings({ home: 'Old place' });
  const save = await startHomeSave(page);
  fire(page.homeWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-home-row'), 'click');
  assert.equal(byId(page, 'settings-home-input').value, 'Old place');

  answer(save, 200, { display_name: null, home_area_name: 'Indiranagar' });
  await flush();

  assert.equal(page.homeWin.hidden, false);
  assert.equal(byId(page, 'settings-home-input').value, 'Indiranagar');
  assert.equal(byId(page, 'settings-home-save').disabled, true, 'nothing is picked in the reopened window');
  assert.equal(byId(page, 'settings-home-remove').hidden, false);
  assert.equal(byId(page, 'settings-home-remove').disabled, false);
});

test('S-3: a home field the person already changed is left alone', async () => {
  const page = loadSettings({ home: 'Old place' });
  const save = await startHomeSave(page);
  fire(page.homeWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-home-row'), 'click');
  byId(page, 'settings-home-input').value = 'Something else';

  answer(save, 200, { display_name: null, home_area_name: 'Indiranagar' });
  await flush();

  assert.equal(byId(page, 'settings-home-input').value, 'Something else');
  assert.equal(byId(page, 'settings-home-remove').hidden, false);
});

test('S-3: an earlier removal that lands in a reopened window empties the untouched field and hides Remove', async () => {
  const page = loadSettings({ home: 'Old place' });
  fire(byId(page, 'settings-home-row'), 'click');
  fire(byId(page, 'settings-home-remove'), 'click');
  const removal = page.requests[page.requests.length - 1];
  fire(page.homeWin, 'keydown', { key: 'Escape' });
  fire(byId(page, 'settings-home-row'), 'click');
  assert.equal(byId(page, 'settings-home-input').value, 'Old place');
  assert.equal(byId(page, 'settings-home-remove').disabled, true, 'a removal is still out');

  answer(removal, 200, { display_name: null, home_area_name: null });
  await flush();

  assert.equal(byId(page, 'settings-home-input').value, '');
  assert.equal(byId(page, 'settings-home-remove').hidden, true);
  assert.equal(byId(page, 'settings-home-save').disabled, true);
});

// ---- Plan 28: suggestions while typing in the Home area --------------------

const AREA_URL = '/api/geocode/place?kind=area&q=';
const AREAS = [
  { name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru, Karnataka, India', lat: 12.9784, lon: 77.6408 },
  { name: 'Indira Nagar', display_name: 'Indira Nagar, Lucknow, Uttar Pradesh, India', lat: 26.8806, lon: 80.9946 }
];
const openHome = (options) => {
  const page = loadSettings(options);
  fire(byId(page, 'settings-home-row'), 'click');
  return page;
};
const typeHome = (page, text) => {
  const input = byId(page, 'settings-home-input');
  input.value = text;
  fire(input, 'input');
};
const homeStatus = (page) => byId(page, 'settings-home-status');
const homeList = (page) => byId(page, 'settings-home-results');

test('typing in the Home area sends nothing under 3 characters and shows no line', () => {
  const page = openHome();
  typeHome(page, 'In');
  page.advance(5000);
  assert.equal(page.requests.length, 0);
  assert.equal(homeStatus(page).hidden, true, 'no too short line comes from typing');
  assert.equal(homeList(page).hidden, true);
});

test('one request goes out after the typing pause, in areas mode, with no Searching line', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(399);
  assert.equal(page.requests.length, 0, 'the pause is 400 ms');
  page.advance(1);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, AREA_URL + 'Indira');
  assert.equal(homeStatus(page).hidden, true, 'a typed search is quiet');
  answer(page.requests[0], 200, { results: AREAS });
  await flush();
  assert.equal(homeList(page).hidden, false);
  assert.equal(homeList(page).children.length, 2);
  assert.equal(byId(page, 'settings-home-save').disabled, true, 'a suggestion is not a pick until it is tapped');
  pick(page, 0);
  assert.equal(byId(page, 'settings-home-save').disabled, false);
});

test('at most 5 suggestions show', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(400);
  const many = [];
  for (let i = 0; i < 8; i++) { many.push({ name: 'Area ' + i, display_name: 'Area ' + i + ', Karnataka', lat: 12 + i / 100, lon: 77 }); }
  answer(page.requests[0], 200, { results: many });
  await flush();
  assert.equal(homeList(page).children.length, 5);
});

test('every keystroke restarts the pause', () => {
  const page = openHome();
  typeHome(page, 'Ind');
  page.advance(300);
  typeHome(page, 'Indi');
  page.advance(300);
  assert.equal(page.requests.length, 0);
  page.advance(100);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, AREA_URL + 'Indi');
});

test('the same text is never sent twice in a row', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: AREAS });
  await flush();
  typeHome(page, 'Indira');
  page.advance(5000);
  assert.equal(page.requests.length, 1);
  typeHome(page, 'Indira ');
  page.advance(5000);
  assert.equal(page.requests.length, 1, 'a trailing space is the same text');
});

test('a request still out is cancelled by a newer one and its late answer is ignored', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(400);
  const first = page.requests[0];
  assert.equal(first.init.signal.aborted, false);
  typeHome(page, 'Indiranag');
  page.advance(800);
  assert.equal(page.requests.length, 1, 'typed searches go out at least a second apart');
  page.advance(200);
  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].at - first.at, 1000);
  assert.equal(first.init.signal.aborted, true, 'the older request is cancelled');
  answer(page.requests[1], 200, { results: [AREAS[0]] });
  await flush();
  answer(first, 200, { results: AREAS });
  await flush();
  assert.equal(homeList(page).children.length, 1, 'the stale answer did not replace the newer list');
});

test('Enter searches at once, shows Searching, and the pause that was running sends nothing more', () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(100);
  fire(byId(page, 'settings-home-form'), 'submit');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, AREA_URL + 'Indira');
  assert.equal(homeStatus(page).textContent, 'Searching…');
  page.advance(5000);
  assert.equal(page.requests.length, 1);
});

test('Enter for the text a typed search already sent takes it over and shows the lines', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(400);
  assert.equal(homeStatus(page).hidden, true);
  fire(byId(page, 'settings-home-form'), 'submit');
  assert.equal(page.requests.length, 1, 'no second request for the same text');
  assert.equal(homeStatus(page).textContent, 'Searching…');
  answer(page.requests[0], 503, {});
  await flush();
  assert.equal(homeStatus(page).textContent, 'Search unavailable. Try again in a moment.');
});

test('a busy typed search is retried quietly and only the last failure shows its line', async () => {
  const page = openHome();
  const refused = { error: { message: 'Too many searches. Wait a few seconds and try again.' } };
  typeHome(page, 'Bengal');
  page.advance(400);
  answer(page.requests[0], 429, refused);
  await flush();
  assert.equal(homeStatus(page).hidden, true, 'the first refusal is quiet');
  page.advance(999);
  assert.equal(page.requests.length, 1);
  page.advance(1);
  assert.equal(page.requests.length, 2);
  answer(page.requests[1], 503, {});
  await flush();
  page.advance(1000);
  assert.equal(page.requests.length, 3);
  answer(page.requests[2], 503, {});
  await flush();
  page.advance(1000);
  assert.equal(page.requests.length, 4);
  assert.equal(homeStatus(page).hidden, true);
  answer(page.requests[3], 429, refused);
  await flush();
  page.advance(5000);
  assert.equal(page.requests.length, 4, 'three retries at most');
  assert.equal(homeStatus(page).textContent, 'Too many searches. Wait a few seconds and try again.');
  assert.equal(homeList(page).hidden, true);
});

test('newer typing cancels a quiet retry that was waiting', async () => {
  const page = openHome();
  typeHome(page, 'Bengal');
  page.advance(400);
  answer(page.requests[0], 503, {});
  await flush();
  typeHome(page, 'Bengalu');
  page.advance(5000);
  assert.deepEqual(page.requests.map((r) => r.url), [AREA_URL + 'Bengal', AREA_URL + 'Bengalu']);
});

test('a network failure while typing stays quiet and is not tried again', async () => {
  const page = openHome();
  typeHome(page, 'Bengal');
  page.advance(400);
  page.requests[0].reject(new Error('offline'));
  await flush();
  page.advance(5000);
  assert.equal(homeStatus(page).hidden, true);
  assert.equal(page.requests.length, 1);
});

test('typing under 3 characters again drops the list, and a closed window drops a pending search', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: AREAS });
  await flush();
  assert.equal(homeList(page).hidden, false);
  typeHome(page, 'In');
  assert.equal(homeList(page).hidden, true);
  assert.equal(homeStatus(page).hidden, true);
  typeHome(page, 'Indira');
  page.advance(1000);
  assert.equal(page.requests.length, 2, 'the text may be sent again once it was dropped');

  typeHome(page, 'Indiran');
  fire(page.homeWin, 'keydown', { key: 'Escape' });
  page.advance(5000);
  assert.equal(page.requests.length, 2, 'closing the window ends the pending pause');
});

test('a 12 character name typed in one go sends one request; typed slowly it stays under one a second', () => {
  const name = 'Banjara Hill';
  assert.equal(Array.from(name).length, 12);
  const quick = openHome();
  for (let i = 1; i <= 12; i++) {
    typeHome(quick, name.slice(0, i));
    quick.advance(100);
  }
  quick.advance(400);
  assert.equal(quick.requests.length, 1);
  assert.equal(quick.requests[0].url, AREA_URL + encodeURIComponent(name));

  const slow = openHome();
  for (let i = 1; i <= 12; i++) {
    typeHome(slow, name.slice(0, i));
    slow.advance(450);
  }
  slow.advance(2000);
  const times = slow.requests.map((r) => r.at);
  assert.ok(times.length >= 2 && times.length <= 5, 'requests sent: ' + times.length + ' at ' + times.join(','));
  for (let i = 1; i < times.length; i++) {
    assert.ok(times[i] - times[i - 1] >= 1000, 'typed searches are at least a second apart: ' + times.join(','));
  }
});

test('a pick ends the typing pause and the picked text is not searched again', async () => {
  const page = openHome();
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: AREAS });
  await flush();
  typeHome(page, 'Indiran');
  pick(page, 0);
  assert.equal(byId(page, 'settings-home-input').value, 'Indiranagar');
  page.advance(5000);
  assert.equal(page.requests.length, 1, 'the pause that was running sent nothing');
});

