// settings_username_test.mjs - runs the real web/static/js/username-check.js and
// web/static/js/settings.js in a Node sandbox and checks the Username window of
// the Settings page (Phase 7.1, plan 15, IDENT-07, D-07, UI-SPEC Surface 8):
//
//   - the Username row opens the built edit window with the stored name filled
//     in and selected, focus in the field, the page behind locked, and the X or
//     Escape as the only ways out;
//   - Save username starts switched off (aria-disabled true, the btn--off look,
//     never the disabled attribute) and switches on only when the shared live
//     check says available; a press while it is off sends nothing;
//   - a save sends only {username}; on 200 the window closes and the row takes
//     the stored name; a 400 or 422 with field username shows the server's
//     sentence under the Username and keeps the window open; a 429, a 401 and
//     a network failure show a sentence in the general line;
//   - every string reaches the page through textContent.
//
// Run it with: node --test web/settings_username_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const settingsSource = fs.readFileSync(path.join(here, 'static', 'js', 'settings.js'), 'utf8');
const checkSource = fs.readFileSync(path.join(here, 'static', 'js', 'username-check.js'), 'utf8');

class FakeEl {
  constructor(doc, tag, id) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.className = '';
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.value = '';
    this.disabled = false;
    this.inert = false;
    this.selected = false;
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
  select() { this.selected = true; }
}

// The markup of settings.html.tmpl, as the ids and nesting settings.js reads.
function buildPage({ username = '', name = '', home = '' } = {}) {
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

  const usernameRow = make('button', 'settings-username-row', main);
  usernameRow.setAttribute('data-value', username);
  make('span', 'settings-username-value', usernameRow).textContent = username || 'Not set';
  const nameRow = make('button', 'settings-name-row', main);
  nameRow.setAttribute('data-value', name);
  make('span', 'settings-name-value', nameRow).textContent = name || 'Not set';
  const homeRow = make('button', 'settings-home-row', main);
  homeRow.setAttribute('data-value', home);
  make('span', 'settings-home-value', homeRow).textContent = home || 'Not set';

  const usernameWin = make('div', 'settings-username-window', doc.body);
  usernameWin.hidden = true;
  const usernamePanel = make('div', '', usernameWin);
  make('button', 'settings-username-close', usernamePanel);
  const usernameForm = make('form', 'settings-username-form', usernamePanel);
  const box = make('div', '', usernameForm);
  box.className = 'username-field';
  make('input', 'settings-username-input', box);
  make('p', 'settings-username-status', usernameForm).className = 'username-status';
  make('p', 'settings-username-help', usernameForm);
  make('p', 'settings-username-error', usernameForm).hidden = true;
  const save = make('button', 'settings-username-save', usernameForm);
  save.className = 'btn btn--off';
  save.setAttribute('aria-disabled', 'true');

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

  return { doc, bar, main, bottom, usernameWin };
}

// loadSettings evaluates username-check.js and then settings.js against a page.
// One fetch keeps every call (the live check and the save) for the test to
// answer; one set of fake timers is moved by hand.
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
  vm.runInContext(checkSource, sandbox, { filename: 'username-check.js' });
  vm.runInContext(settingsSource, sandbox, { filename: 'settings.js' });
  const checks = () => requests.filter((r) => r.url.startsWith('/api/username/check'));
  const saves = () => requests.filter((r) => r.url === '/api/me/profile');
  return { ...page, api: sandbox.PinalertSettings, requests, checks, saves, advance, sandbox };
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

const input = (page) => byId(page, 'settings-username-input');
const line = (page) => byId(page, 'settings-username-status');
const save = (page) => byId(page, 'settings-username-save');
const errorLine = (page) => byId(page, 'settings-username-error');
const isOff = (page) => save(page).getAttribute('aria-disabled') === 'true' && save(page).className.split(/\s+/).includes('btn--off');
const isOn = (page) => save(page).getAttribute('aria-disabled') === null && save(page).className.split(/\s+/).includes('btn--primary') && !save(page).className.split(/\s+/).includes('btn--off');

const open = (page) => fire(byId(page, 'settings-username-row'), 'click');

// typeName types a name, waits out the pause and answers the live check.
async function typeName(page, text, state = 'available', message = 'That username is available.') {
  input(page).value = text;
  fire(input(page), 'input');
  page.advance(300);
  const check = page.checks()[page.checks().length - 1];
  answer(check, 200, { state, message });
  await flush();
  return check;
}

test('the Username row opens the window with the stored name filled in and selected, and the X closes it', () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);

  assert.equal(page.usernameWin.hidden, false);
  assert.equal(input(page).value, 'ravi.k');
  assert.equal(input(page).selected, true, 'the stored name is selected');
  assert.equal(page.doc.activeElement, input(page));
  assert.equal(page.main.inert, true);
  assert.equal(page.bar.inert, true);
  assert.equal(page.bottom.inert, true);
  assert.equal(page.usernameWin.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), 'true');

  fire(page.usernameWin, 'click', { target: page.usernameWin });
  assert.equal(page.usernameWin.hidden, false, 'a click on the backdrop must not close the window (D-02)');

  fire(byId(page, 'settings-username-close'), 'click');
  assert.equal(page.usernameWin.hidden, true);
  assert.equal(page.main.inert, false);
  assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'));
});

test('Escape closes the window and Tab stays inside it', () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  const close = byId(page, 'settings-username-close');
  save(page).focus();
  const tab = fire(page.usernameWin, 'keydown', { key: 'Tab' });
  assert.equal(tab.prevented, true);
  assert.equal(page.doc.activeElement, close);

  fire(page.usernameWin, 'keydown', { key: 'Escape' });
  assert.equal(page.usernameWin.hidden, true);
  assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'));
});

test('Save username starts switched off, with aria-disabled and the btn--off look and not the disabled attribute', () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  assert.equal(isOff(page), true);
  assert.equal(save(page).disabled, false, 'the disabled attribute is never used, so the button stays reachable');
});

test('a press on Save username while it is off sends nothing', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  fire(byId(page, 'settings-username-form'), 'submit');
  fire(save(page), 'click');
  await flush();
  assert.equal(page.saves().length, 0);
  assert.equal(errorLine(page).hidden, true);
});

test('the stored name is answered as yours and keeps Save username off', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  page.advance(300);
  assert.equal(page.checks().length, 1);
  assert.equal(page.checks()[0].url, '/api/username/check?u=ravi.k');
  answer(page.checks()[0], 200, { state: 'yours', message: 'That is your username.' });
  await flush();
  assert.equal(line(page).textContent, 'That is your username.');
  assert.equal(isOff(page), true);
});

test('Save username switches on only when the live check says available', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  input(page).value = 'ravi.kumar';
  fire(input(page), 'input');
  assert.equal(line(page).textContent, 'Checking\u2026');
  assert.equal(isOff(page), true, 'off while it checks');
  page.advance(300);
  answer(page.checks()[page.checks().length - 1], 200, { state: 'taken', message: 'That username is taken.' });
  await flush();
  assert.equal(isOff(page), true, 'off when taken');

  await typeName(page, 'ravi.k2');
  assert.equal(line(page).textContent, 'That username is available.');
  assert.equal(isOn(page), true);

  input(page).value = 'ravi.k2x';
  fire(input(page), 'input');
  assert.equal(isOff(page), true, 'typing again switches it off until the new check answers');
});

test('a save sends only the username, and on 200 the window closes and the row takes the stored name', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');

  assert.equal(page.saves().length, 1);
  const req = page.saves()[0];
  assert.equal(req.init.method, 'PATCH');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(req.init.body), { username: 'ravi.kumar' });

  fire(save(page), 'click');
  fire(byId(page, 'settings-username-form'), 'submit');
  assert.equal(page.saves().length, 1, 'a second press while the save is out sends nothing');

  answer(req, 200, { username: 'ravi.kumar', display_name: null, home_area_name: null });
  await flush();
  assert.equal(page.usernameWin.hidden, true);
  assert.equal(byId(page, 'settings-username-value').textContent, 'ravi.kumar');
  assert.equal(byId(page, 'settings-username-row').getAttribute('data-value'), 'ravi.kumar');
  assert.equal(page.main.inert, false);
});

test('a row with no username reads Not set, and the window opens empty with the rule line', () => {
  const page = loadSettings();
  assert.equal(byId(page, 'settings-username-value').textContent, 'Not set');
  open(page);
  assert.equal(input(page).value, '');
  assert.equal(line(page).textContent, 'Use 3 to 20 letters, numbers, dots or underscores.');
  assert.equal(page.checks().length, 0);
  assert.equal(isOff(page), true);
});

test('a 400 for a taken or too soon name shows the server sentence under the Username and keeps the window open', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 400, { error: { field: 'username', message: 'You can change your username again in 5 days.' } });
  await flush();

  assert.equal(page.usernameWin.hidden, false);
  assert.equal(line(page).textContent, 'You can change your username again in 5 days.');
  assert.ok(line(page).className.split(/\s+/).includes('username-status--bad'));
  assert.equal(isOff(page), true, 'Save is off until the name is typed again');
  assert.equal(errorLine(page).hidden, true);
  assert.equal(byId(page, 'settings-username-value').textContent, 'ravi.k');
  assert.equal(page.main.inert, true);
});

test('a 422 from the screening shows its sentence under the Username too', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 422, { error: { field: 'username', message: 'Choose a different username.' } });
  await flush();
  assert.equal(page.usernameWin.hidden, false);
  assert.equal(line(page).textContent, 'Choose a different username.');
  assert.equal(isOff(page), true);
});

test('a 429 shows the server sentence in the general line and leaves the window open', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 429, { error: { field: 'username', message: 'Too many tries. Wait a little and try again.' }, retry_after_seconds: 60 });
  await flush();
  assert.equal(page.usernameWin.hidden, false);
  assert.equal(errorLine(page).hidden, false);
  assert.equal(errorLine(page).textContent, 'Too many tries. Wait a little and try again.');
  assert.equal(line(page).textContent, 'That username is available.', 'the line under the Username is not used for a rate limit');
});

test('a 401 and a network failure say so plainly in the general line', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 401, {});
  await flush();
  assert.equal(errorLine(page).textContent, 'Your session has ended. Reload the page and log in again.');

  fire(byId(page, 'settings-username-form'), 'submit');
  assert.equal(page.saves().length, 2);
  assert.equal(errorLine(page).hidden, true, 'the next save clears the earlier line');
  page.saves()[1].reject(new Error('offline'));
  await flush();
  assert.equal(page.usernameWin.hidden, false);
  assert.equal(errorLine(page).textContent, 'Could not save. Try again in a moment.');
});

test('opening the window again does not attach the live check a second time', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  fire(byId(page, 'settings-username-close'), 'click');
  open(page);
  fire(byId(page, 'settings-username-close'), 'click');
  open(page);
  const before = page.checks().length;
  input(page).value = 'other.name';
  fire(input(page), 'input');
  page.advance(300);
  assert.equal(page.checks().length, before + 1, 'one check for one pause, not one for each opening');
});

test('reopening the window shows the stored name again and clears an earlier refusal', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 429, { error: { message: 'Too many tries. Wait a little and try again.' } });
  await flush();
  fire(byId(page, 'settings-username-close'), 'click');
  open(page);
  assert.equal(input(page).value, 'ravi.k');
  assert.equal(errorLine(page).hidden, true);
  assert.equal(isOff(page), true);
});

test('server sentences and the stored name reach the page as text only', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar', 'taken', '<img src=x onerror=alert(1)>');
  assert.equal(line(page).textContent, '<img src=x onerror=alert(1)>');
  fire(byId(page, 'settings-username-form'), 'submit');
  // The FakeEl innerHTML and outerHTML setters throw, so reaching here proves
  // neither was used. The device position getter would have thrown as well.
  assert.equal(page.saves().length, 0);
});

test('the Username window is not a login field: no autocomplete value, no password manager hook', () => {
  const settings = settingsSource;
  assert.equal(/autocomplete/i.test(settings), false);
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML/.test(settings), false);
});
