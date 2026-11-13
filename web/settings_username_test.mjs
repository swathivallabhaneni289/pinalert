// settings_username_test.mjs - runs the real web/static/js/username-check.js and
// web/static/js/profile-edit.js in a Node sandbox and checks the Username window of
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
const settingsSource = fs.readFileSync(path.join(here, 'static', 'js', 'profile-edit.js'), 'utf8');
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

// The markup of settings.html.tmpl, as the ids and nesting profile-edit.js reads.
// Amended 2026-11-08 by quick 261108-fku (owner decision 2026-11-08): the page
// can carry the moment the rename wait ends on the Username window, and draws the
// hidden OK button after Save username. Amended again 2026-11-08 by the review fix
// WR-02: the page option is waitIn (milliseconds from the fake clock's start to the
// end of the wait; a negative number is a wait that is over; null draws no
// attribute, as outside the gap), written as data-wait-until in RFC 3339 UTC, or
// waitRaw for any other attribute text. It replaces waitMessage and waitAfterChange.
function buildPage({ username = '', name = '', home = '', waitIn = null, waitRaw = null, startMs = 0, withOk = true } = {}) {
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
  if (waitIn !== null) { usernameWin.setAttribute('data-wait-until', new Date(startMs + waitIn).toISOString()); }
  if (waitRaw !== null) { usernameWin.setAttribute('data-wait-until', waitRaw); }
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
  if (withOk) {
    const ok = make('button', 'settings-username-ok', usernameForm);
    ok.className = 'btn btn--primary';
    ok.hidden = true;
  }

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

// loadSettings evaluates username-check.js and then profile-edit.js against a page.
// One fetch keeps every call (the live check and the save) for the test to
// answer; one set of fake timers is moved by hand.
function loadSettings(options) {
  // Amended 2026-11-08 (review fix WR-02): the page reads the clock, so the sandbox
  // gets a Date whose now() is the hand moved clock.ms. It starts at a fixed time.
  const clock = { ms: Date.UTC(2026, 10, 8, 10, 0, 0) };
  const page = buildPage({ ...options, startMs: clock.ms });
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
    Date: class FakeDate extends Date { static now() { return clock.ms; } },
    navigator: {
      get geolocation() { throw new Error('profile-edit.js asked for the device position'); }
    },
    location: {}
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(checkSource, sandbox, { filename: 'username-check.js' });
  vm.runInContext(settingsSource, sandbox, { filename: 'profile-edit.js' });
  const checks = () => requests.filter((r) => r.url.startsWith('/api/username/check'));
  const saves = () => requests.filter((r) => r.url === '/api/me/profile');
  return { ...page, api: sandbox.PinalertSettings, requests, checks, saves, advance, sandbox, clock };
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

// Phase 7.1 review fix (browser), IN-B05: shutting the window stops a check that
// is still due, so a shut window spends none of the account's allowance.
test('IN-B05: a check still due when the window shuts is not sent', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  page.advance(300);
  assert.equal(page.checks().length, 1);
  input(page).value = 'ravi.kumar';
  fire(input(page), 'input');
  page.advance(100);
  fire(byId(page, 'settings-username-close'), 'click');
  page.advance(1000);
  assert.equal(page.checks().length, 1, 'no check after the window shut');

  // Escape shuts it the same way.
  const esc = loadSettings({ username: 'ravi.k' });
  open(esc);
  input(esc).value = 'ravi.kumar';
  fire(input(esc), 'input');
  fire(byId(esc, 'settings-username-window'), 'keydown', { key: 'Escape' });
  esc.advance(1000);
  assert.equal(esc.checks().length, 0);
});

// Phase 7.1 review fix (browser), WR-B03: a check that failed leaves Save username
// off, and a press on it then asks again instead of doing nothing.
test('WR-B03: a press on Save username after a failed check asks again and saves nothing', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  page.advance(300);
  answer(page.checks()[0], 200, { state: 'yours', message: 'This is your username now.' });
  await flush();
  input(page).value = 'ravi.kumar';
  fire(input(page), 'input');
  page.advance(300);
  page.checks()[1].reject(new TypeError('Failed to fetch'));
  await flush();
  assert.equal(isOff(page), true);
  fire(byId(page, 'settings-username-form'), 'submit');
  await flush();
  page.advance(300);
  assert.equal(page.checks().length, 3, 'a new check is sent');
  assert.ok(page.checks()[2].url.endsWith('ravi.kumar'));
  assert.equal(page.saves().length, 0);
  answer(page.checks()[2], 200, { state: 'available', message: 'ravi.kumar is available.' });
  await flush();
  assert.equal(isOn(page), true);
});

test('the Username window is not a login field: no autocomplete value, no password manager hook', () => {
  const settings = settingsSource;
  assert.equal(/autocomplete/i.test(settings), false);
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML/.test(settings), false);
});

// ---- Quick 261108-fku (owner decision 2026-11-08): the window during the 7 day
// rename wait. It opens read only with the server's sentence in the wait look
// and one OK button instead of Save username. ------------------------------------
//
// Amended 2026-11-08 by the review fix WR-02: the page now carries the moment the
// wait ends (data-wait-until, drawn here by the waitIn option) instead of a
// sentence, and every opening works out the days left from it against the clock
// the sandbox gives profile-edit.js (page.clock.ms). The tests below that used
// waitMessage and waitAfterChange use waitIn: 7 * DAY, or nothing, to the same
// effect. The D11 test keeps its first half; its reopen half changed (a live wait
// answer locks only the window open now, see the Amended line on that test).

const DAY = 24 * 60 * 60 * 1000;
const SEVEN = 'You can change your username again in 7 days.';
const okButton = (page) => byId(page, 'settings-username-ok');
const hasClass = (el, name) => el.className.split(/\s+/).includes(name);
const isReadOnly = (page) => input(page).getAttribute('readonly') !== null && input(page).getAttribute('aria-readonly') === 'true';

function assertLocked(page, sentence, stored) {
  assert.equal(page.usernameWin.hidden, false);
  assert.equal(input(page).value, stored, 'the field holds the stored name');
  assert.equal(isReadOnly(page), true, 'readonly and aria-readonly true');
  assert.equal(line(page).textContent, sentence);
  assert.equal(hasClass(line(page), 'username-status--warn'), true, 'the wait look');
  assert.equal(save(page).hidden, true, 'Save username is not shown');
  assert.equal(okButton(page).hidden, false, 'OK is shown');
  assert.equal(hasClass(okButton(page), 'btn--primary'), true);
  assert.equal(page.doc.activeElement, okButton(page), 'focus is on OK');
  assert.equal(errorLine(page).hidden, true);
}

test('inside the wait the window opens locked: read only name, the server sentence, OK, no Save, no check sent', async () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
  open(page);
  assertLocked(page, SEVEN, 'ravi.k');
  assert.equal(page.main.inert, true, 'the page behind is still locked');
  page.advance(2000);
  await flush();
  assert.equal(page.checks().length, 0, 'no live check is sent');
  assert.equal(byId(page, 'settings-username-help').getAttribute('hidden'), null, 'the help line stays');
});

test('OK closes the locked window like the X, sends nothing and saves nothing', async () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
  open(page);
  fire(okButton(page), 'click');
  assert.equal(page.usernameWin.hidden, true);
  assert.equal(page.main.inert, false);
  assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'));
  page.advance(2000);
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(byId(page, 'settings-username-value').textContent, 'ravi.k');
});

test('Enter in the locked window, a submit and Escape all close it and save nothing', async () => {
  for (const how of ['enter', 'submit', 'escape']) {
    const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
    open(page);
    let event = null;
    if (how === 'enter') { event = fire(page.usernameWin, 'keydown', { key: 'Enter', target: input(page) }); }
    if (how === 'submit') { event = fire(byId(page, 'settings-username-form'), 'submit'); }
    if (how === 'escape') { event = fire(page.usernameWin, 'keydown', { key: 'Escape' }); }
    if (how !== 'escape') { assert.equal(event.prevented, true, how + ' is prevented'); }
    assert.equal(page.usernameWin.hidden, true, how + ' closes the window');
    assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'), how + ' returns focus to the row');
    await flush();
    assert.equal(page.requests.length, 0, how + ' sends nothing');
  }
});

test('an input event on the read only field sends no check and leaves the sentence', async () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
  open(page);
  input(page).value = 'ravi.kumar';
  fire(input(page), 'input');
  page.advance(2000);
  await flush();
  assert.equal(page.checks().length, 0);
  assert.equal(line(page).textContent, SEVEN);
});

// Amended 2026-11-08 (review fix WR-02, second commit): a live wait answer locks
// the window open now and sets no deadline in the page, because its day count is
// rounded up and a lock on it could outlast the real wait with nothing to correct
// it. The reopen half used to expect a locked window; it now expects the usual
// window, whose live check answers wait again and locks with a fresh count.
test('D11: a wait answer to a typed name locks the window with the server sentence and puts the stored name back', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  assert.equal(isReadOnly(page), false);
  const sentence = 'You can change your username again in 4 days.';
  await typeName(page, 'ravi.kumar', 'wait', sentence);
  assertLocked(page, sentence, 'ravi.k');
  fire(okButton(page), 'click');
  open(page);
  assertNormal(page);
  assert.equal(input(page).value, 'ravi.k');
  const checksBefore = page.checks().length;
  page.advance(300);
  assert.equal(page.checks().length, checksBefore + 1, 'the live check judges the stored name on the reopening');
  await typeName(page, 'ravi.kumar', 'wait', 'You can change your username again in 3 days.');
  assertLocked(page, 'You can change your username again in 3 days.', 'ravi.k');
});

test('C9: a change saved in the page locks the next opening with the whole gap sentence, and sends no check', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 200, { username: 'ravi.kumar', display_name: null, home_area_name: null });
  await flush();
  assert.equal(page.usernameWin.hidden, true);
  const before = page.checks().length;
  open(page);
  assertLocked(page, SEVEN, 'ravi.kumar');
  page.advance(2000);
  await flush();
  assert.equal(page.checks().length, before, 'no check on the locked opening');
});

test('C9: a first username saved from Not set starts no wait, so the next opening is normal', async () => {
  const page = loadSettings({});
  open(page);
  await typeName(page, 'ravi.k');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 200, { username: 'ravi.k', display_name: null, home_area_name: null });
  await flush();
  assert.equal(page.usernameWin.hidden, true);
  open(page);
  assert.equal(isReadOnly(page), false);
  assert.equal(input(page).getAttribute('readonly'), null);
  assert.equal(save(page).hidden, false);
  assert.equal(okButton(page).hidden, true);
  const before = page.checks().length;
  page.advance(300);
  assert.equal(page.checks().length, before + 1, 'the live check judges the stored name as before');
});

test('D12: with no wait on the page the window opens exactly as before', () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  assert.equal(input(page).getAttribute('readonly'), null);
  assert.equal(input(page).getAttribute('aria-readonly'), null);
  assert.equal(save(page).hidden, false);
  assert.equal(okButton(page).hidden, true);
  assert.equal(page.doc.activeElement, input(page));
  assert.equal(input(page).selected, true);
  page.advance(300);
  assert.equal(page.checks().length, 1, 'a check is sent');
});

test('C12: a page drawn without the OK button still locks: read only, the sentence, Save hidden', () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY, withOk: false });
  open(page);
  assert.equal(isReadOnly(page), true);
  assert.equal(line(page).textContent, SEVEN);
  assert.equal(save(page).hidden, true);
  assert.equal(page.doc.activeElement, input(page));
});

// ---- Review fix WR-02: the lock follows the real deadline ---------------------

function assertNormal(page) {
  assert.equal(page.usernameWin.hidden, false);
  assert.equal(input(page).getAttribute('readonly'), null, 'the field is editable');
  assert.equal(input(page).getAttribute('aria-readonly'), null);
  assert.equal(save(page).hidden, false, 'Save username is shown');
  assert.equal(okButton(page).hidden, true, 'OK is hidden');
  assert.equal(page.doc.activeElement, input(page));
}

// ---- Review fix WR-01: a locked opening says the wait sentence out loud ------

// watchAnnounce records what the status line held the moment the window was
// shown, and every write to it with whether the window was hidden at the time.
function watchAnnounce(page) {
  const win = page.usernameWin;
  const seen = { atShow: [], writes: [] };
  const hiddenDesc = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(win), 'hidden');
  Object.defineProperty(win, 'hidden', {
    get() { return hiddenDesc.get.call(this); },
    set(v) {
      if (!v) { seen.atShow.push(line(page).textContent); }
      hiddenDesc.set.call(this, v);
    }
  });
  const status = line(page);
  const textDesc = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(status), 'textContent');
  Object.defineProperty(status, 'textContent', {
    get() { return textDesc.get.call(this); },
    set(v) { seen.writes.push({ text: String(v), winHidden: win.hidden }); textDesc.set.call(this, v); }
  });
  return seen;
}

test('WR-01: when the window opens locked the wait sentence is written after it is shown, never while it is hidden', () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
  const seen = watchAnnounce(page);
  for (let opening = 1; opening <= 3; opening++) {
    seen.atShow.length = 0;
    seen.writes.length = 0;
    open(page);
    assertLocked(page, SEVEN, 'ravi.k');
    assert.deepEqual(seen.atShow, [''], 'opening ' + opening + ': the line is empty at the moment the window is shown, so writing the sentence is a change');
    const sentenceWrites = seen.writes.filter((w) => w.text === SEVEN);
    assert.ok(sentenceWrites.length >= 1, 'opening ' + opening + ': the sentence is written');
    assert.ok(sentenceWrites.every((w) => w.winHidden === false), 'opening ' + opening + ': the sentence is never written into a hidden window');
    fire(okButton(page), 'click');
  }
});

test('WR-01: a wait answer while the window is open writes the sentence at once, and the normal opening is unchanged', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  const seen = watchAnnounce(page);
  open(page);
  assert.equal(seen.writes.some((w) => w.text === SEVEN), false);
  await typeName(page, 'ravi.kumar', 'wait', 'You can change your username again in 4 days.');
  const write = seen.writes.filter((w) => w.text === 'You can change your username again in 4 days.');
  assert.ok(write.length >= 1 && write.every((w) => w.winHidden === false));
});

// ---- Review fix IN-02: a lock with no sentence does nothing ------------------

test('IN-02: a wait answer that leaves the status line empty does not half lock the window', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  let onChange = null;
  const ctl = { isGood: () => false, state: () => 'wait', refresh() {}, cancel() {}, hold() { throw new Error('hold must not run'); }, name: () => '' };
  page.sandbox.PinalertUsernameCheck = { attach(inputEl, lineEl, options) { onChange = options.onChange; return ctl; } };
  open(page);
  assertNormal(page);
  assert.equal(line(page).textContent, '');
  onChange('wait', ctl);
  assertNormal(page);
  assert.equal(isReadOnly(page), false);
  assert.equal(isOff(page), true, 'Save username is drawn off, as for any wait answer');
  fire(page.usernameWin, 'keydown', { key: 'Enter', target: input(page) });
  assert.equal(page.usernameWin.hidden, false, 'Enter is not taken as a locked window Enter');
  fire(byId(page, 'settings-username-close'), 'click');
  open(page);
  assertNormal(page);
});

// ---- Review fix IN-03: which name a locked field shows ------------------------

test('IN-03: a locked opening shows the name the server last confirmed, and a live wait answer keeps what the row shows', async () => {
  // At load the confirmed name is the one the page was drawn with.
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
  byId(page, 'settings-username-row').setAttribute('data-value', 'drifted.row');
  open(page);
  assert.equal(input(page).value, 'ravi.k', 'the page was drawn with ravi.k, whatever the row attribute says now');
  fire(okButton(page), 'click');

  // After a save in the page the confirmed name is the saved one.
  const second = loadSettings({ username: 'ravi.k' });
  open(second);
  await typeName(second, 'ravi.kumar');
  fire(byId(second, 'settings-username-form'), 'submit');
  answer(second.saves()[0], 200, { username: 'ravi.kumar', display_name: null, home_area_name: null });
  await flush();
  byId(second, 'settings-username-row').setAttribute('data-value', 'drifted.row');
  open(second);
  assertLocked(second, SEVEN, 'ravi.kumar');
  fire(okButton(second), 'click');

  // A lock from a live wait answer keeps what the row shows.
  const third = loadSettings({ username: 'ravi.k' });
  open(third);
  byId(third, 'settings-username-row').setAttribute('data-value', 'row.shows');
  await typeName(third, 'ravi.kumar', 'wait', 'You can change your username again in 4 days.');
  assertLocked(third, 'You can change your username again in 4 days.', 'row.shows');
});

// ---- Review fix IN-06: two branches that had no node test ----------------------

test('IN-06: a save sent from an earlier opening that lands while the window is open again locks that window', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  assert.equal(page.saves().length, 1);
  fire(byId(page, 'settings-username-close'), 'click');

  open(page);
  assertNormal(page);
  const before = page.checks().length;
  answer(page.saves()[0], 200, { username: 'ravi.kumar', display_name: null, home_area_name: null });
  await flush();
  assertLocked(page, SEVEN, 'ravi.kumar');
  assert.equal(byId(page, 'settings-username-value').textContent, 'ravi.kumar');
  page.advance(2000);
  await flush();
  assert.equal(page.checks().length, before, 'no check is sent for the locked window');
  assert.equal(page.saves().length, 1, 'nothing more is saved');

  fire(okButton(page), 'click');
  assert.equal(page.usernameWin.hidden, true);
  assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'));
});

test('IN-06: a first username saved from an earlier opening does not lock the window open now', async () => {
  const page = loadSettings({});
  open(page);
  await typeName(page, 'ravi.k');
  fire(byId(page, 'settings-username-form'), 'submit');
  fire(byId(page, 'settings-username-close'), 'click');
  open(page);
  answer(page.saves()[0], 200, { username: 'ravi.k', display_name: null, home_area_name: null });
  await flush();
  assertNormal(page);
  assert.equal(input(page).value, 'ravi.k', 'the untouched field takes the saved name');
});

test('IN-06: Enter on OK is left to the button, which closes the window by its own click', () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 7 * DAY });
  open(page);
  const event = fire(page.usernameWin, 'keydown', { key: 'Enter', target: okButton(page) });
  assert.equal(event.prevented, false, 'the key is not taken, so the browser clicks OK');
  assert.equal(page.usernameWin.hidden, false, 'the key handler itself closes nothing');
  fire(okButton(page), 'click');
  assert.equal(page.usernameWin.hidden, true);
  assert.equal(page.doc.activeElement, byId(page, 'settings-username-row'));
});

test('WR-02: the fake clock really drives Date.now() inside profile-edit.js', () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 3 * DAY });
  open(page);
  assert.equal(line(page).textContent, 'You can change your username again in 3 days.');
  page.clock.ms += DAY;
  fire(byId(page, 'settings-username-close'), 'click');
  open(page);
  assert.equal(line(page).textContent, 'You can change your username again in 2 days.');
});

test('WR-02: a deadline in the past opens the window normally and the live check runs', () => {
  for (const waitIn of [-DAY, 0, -1]) {
    const page = loadSettings({ username: 'ravi.k', waitIn });
    open(page);
    assertNormal(page);
    assert.equal(input(page).selected, true);
    page.advance(300);
    assert.equal(page.checks().length, 1, 'a check is sent for the stored name');
  }
});

test('WR-02: 1.5 days left reads 2 days, and a few seconds past a whole day rounds up too', () => {
  for (const [waitIn, text] of [
    [1.5 * DAY, 'You can change your username again in 2 days.'],
    [DAY, 'You can change your username again in 1 day.'],
    [DAY + 1000, 'You can change your username again in 2 days.'],
    [1000, 'You can change your username again in 1 day.'],
    [7 * DAY, SEVEN]
  ]) {
    const page = loadSettings({ username: 'ravi.k', waitIn });
    open(page);
    assertLocked(page, text, 'ravi.k');
  }
});

test('WR-02: no attribute, or one that is not a time, opens the window normally', () => {
  for (const options of [{}, { waitRaw: '' }, { waitRaw: 'soon' }]) {
    const page = loadSettings({ username: 'ravi.k', ...options });
    open(page);
    assertNormal(page);
  }
});

test('WR-02: a change saved in the page locks the next opening with 7 days, and 7 days later it opens normally', async () => {
  const page = loadSettings({ username: 'ravi.k' });
  open(page);
  await typeName(page, 'ravi.kumar');
  fire(byId(page, 'settings-username-form'), 'submit');
  answer(page.saves()[0], 200, { username: 'ravi.kumar', display_name: null, home_area_name: null });
  await flush();
  open(page);
  assertLocked(page, SEVEN, 'ravi.kumar');
  fire(okButton(page), 'click');

  page.clock.ms += 3.5 * DAY;
  open(page);
  assertLocked(page, 'You can change your username again in 4 days.', 'ravi.kumar');
  fire(okButton(page), 'click');

  page.clock.ms += 3.5 * DAY;
  const before = page.checks().length;
  open(page);
  assertNormal(page);
  assert.equal(input(page).value, 'ravi.kumar', 'the field holds the name saved in the page');
  page.advance(300);
  assert.equal(page.checks().length, before + 1, 'the live check judges the stored name again');
});

test('WR-02: a window locked at load unlocks on the first opening after the wait ends, and the sentence follows the days', () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: 2.5 * DAY });
  open(page);
  assertLocked(page, 'You can change your username again in 3 days.', 'ravi.k');
  fire(okButton(page), 'click');

  page.clock.ms += DAY;
  open(page);
  assertLocked(page, 'You can change your username again in 2 days.', 'ravi.k');
  fire(okButton(page), 'click');

  page.clock.ms += 0.6 * DAY;
  open(page);
  assertLocked(page, 'You can change your username again in 1 day.', 'ravi.k');
  fire(okButton(page), 'click');

  page.clock.ms += DAY;
  open(page);
  assertNormal(page);
  assert.equal(isReadOnly(page), false);
  assert.equal(line(page).textContent, 'Checking…', 'the stored name is judged again, no wait sentence left');
  assert.equal(hasClass(line(page), 'username-status--warn'), false);
});

test('WR-02: a live wait answer sets no deadline, so a device clock a little fast cannot leave a lock past the real end', async () => {
  // The page deadline is 10 minutes away; the device clock is 15 minutes ahead,
  // so the window opens as usual. The server still says 1 day (rounded up from
  // the few minutes left). That locks the window open now only.
  const page = loadSettings({ username: 'ravi.k', waitIn: 10 * 60 * 1000 });
  page.clock.ms += 15 * 60 * 1000;
  open(page);
  assertNormal(page);
  await typeName(page, 'ravi.kumar', 'wait', 'You can change your username again in 1 day.');
  assertLocked(page, 'You can change your username again in 1 day.', 'ravi.k');
  fire(okButton(page), 'click');

  // An hour on, the wait is over by any clock; the window opens as usual.
  page.clock.ms += 60 * 60 * 1000;
  open(page);
  assertNormal(page);
  const before = page.checks().length;
  page.advance(300);
  assert.equal(page.checks().length, before + 1, 'a check is sent, so the server can correct the page');
});

test('WR-02: a window unlocked after the wait can type, check and enable Save username again', async () => {
  const page = loadSettings({ username: 'ravi.k', waitIn: DAY });
  open(page);
  fire(okButton(page), 'click');
  page.clock.ms += 2 * DAY;
  open(page);
  assertNormal(page);
  await typeName(page, 'ravi.kumar');
  assert.equal(isOn(page), true);
  fire(byId(page, 'settings-username-form'), 'submit');
  assert.equal(page.saves().length, 1);
  assert.deepEqual(JSON.parse(page.saves()[0].init.body), { username: 'ravi.kumar' });
});
