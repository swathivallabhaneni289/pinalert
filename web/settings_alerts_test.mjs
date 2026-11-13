// settings_alerts_test.mjs - runs the real web/static/js/settings-alerts.js in a
// Node sandbox and checks the Muted reports window, the Log out of all devices
// window and the Delete my account box of the Settings page (Phase 3.2 plan 12,
// D-11, D-23, UI-SPEC E2, E3):
//
//   - Muted reports loads GET /api/me/muted-reports, lists each report with its
//     text, category and severity or "This report is no longer available", and
//     shows the empty and error states;
//   - Unmute this report posts to the report's own unmute address, removes the
//     row at once, lowers the count, moves focus on and says so politely;
//   - Log out of all devices and Delete my account each post once, show their
//     busy word, go to their fixed sign in address on success and show their
//     sentence on failure;
//   - every window (Muted reports, Log out of all devices) opens with focus on
//     its title, locks the page, keeps Tab inside and closes with the X or
//     Escape only;
//   - Delete my account is not a window: it is a small native box like Log out
//     (owner, 2026-10-25), answered by Cancel, Delete or Escape. Focus starts on
//     the question, a tap on the dim page does nothing, and Escape is refused
//     while the delete is being sent;
//   - every string reaches the page through textContent.
//
// Run it with: node --test web/settings_alerts_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what settings-alerts.js calls: a minimal element model,
// fake timers the test moves by hand and a fetch the test answers by hand.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'settings-alerts.js'), 'utf8');

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
    this.disabled = false;
    this.inert = false;
    this.checked = false;
    this.value = '';
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML must never be used: ' + v); }
  get open() { return Object.hasOwn(this.attrs, 'open'); }
  get hidden() { return Object.hasOwn(this.attrs, 'hidden'); }
  set hidden(v) {
    if (v) { this.attrs.hidden = ''; } else { delete this.attrs.hidden; }
    this.doc.changed(this, 'hidden');
  }
  setAttribute(n, v) {
    this.attrs[n] = String(v);
    if (n === 'id') { this.id = String(v); this.doc.byId[this.id] = this; }
    this.doc.changed(this, n);
  }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; this.doc.changed(this, n); }
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
  // Every button and input below this element, in order, which is all the
  // focus trap and the list code ask for.
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

// The markup of settings.html.tmpl, as the ids and nesting settings-alerts.js
// reads. withMarkup false builds a page that has none of the new rows.
function buildPage({ count = 0, withMarkup = true, home = true, radius = 5, dialog = true, key = 'BPUBLICKEY' } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    observers: [],
    mainNode: null,
    // Phase 3.3 plan 14: the page root carries data-push-key, which the script
    // reads with one attribute selector.
    querySelector(selector) { return selector === '[data-push-key]' ? doc.mainNode : null; },
    // changed tells every fake MutationObserver watching el that an attribute
    // moved, as a microtask, the way a browser delivers the records.
    changed(el, name) {
      for (const o of doc.observers) {
        if (o.target === el && (!o.filter || o.filter.includes(name))) {
          Promise.resolve().then(() => o.cb([{ type: 'attributes', target: el, attributeName: name }], o));
        }
      }
    },
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
  make('div', 'notice-live', doc.body);
  const main = make('div', 'app-content', doc.body);
  main.setAttribute('data-push-key', key);
  doc.mainNode = main;
  const bottom = make('nav', 'bottom-bar', doc.body);
  if (!withMarkup) { return { doc, bar, main, bottom }; }

  make('button', 'settings-muted-row', main);
  make('span', 'settings-muted-value', main).textContent = String(count);
  make('button', 'settings-logout-all-row', main);
  make('button', 'settings-delete-row', main);

  const muted = make('div', 'settings-muted-window', doc.body);
  muted.hidden = true;
  const mutedPanel = make('div', '', muted);
  const mutedHead = make('div', '', mutedPanel);
  make('h2', 'settings-muted-title', mutedHead);
  make('button', 'settings-muted-close', mutedHead);
  const mutedBody = make('div', '', mutedPanel);
  make('div', 'settings-muted-loading', mutedBody).hidden = true;
  make('ul', 'settings-muted-list', mutedBody).hidden = true;
  const empty = make('div', 'settings-muted-empty', mutedBody);
  empty.hidden = true;
  make('h3', 'settings-muted-empty-title', empty);
  const error = make('div', 'settings-muted-error', mutedBody);
  error.hidden = true;
  make('button', 'settings-muted-retry', error);

  const logout = make('div', 'settings-logout-all-window', doc.body);
  logout.hidden = true;
  const logoutPanel = make('div', '', logout);
  const logoutHead = make('div', '', logoutPanel);
  make('h2', 'settings-logout-all-title', logoutHead);
  make('button', 'settings-logout-all-close', logoutHead);
  make('p', 'settings-logout-all-error', logoutPanel).hidden = true;
  make('button', 'settings-logout-all-go', logoutPanel).textContent = 'Log out of all devices';

  // The Delete box is a dialog element (owner, 2026-10-25). Only this element
  // gets showModal and close, and only when the page is built with a dialog.
  const del = make('dialog', 'settings-delete-confirm', doc.body);
  const delBody = make('div', '', del);
  make('h2', 'settings-delete-title', delBody).textContent = 'Delete your account?';
  make('div', 'settings-delete-text', delBody);
  make('p', 'settings-delete-error', delBody).hidden = true;
  const delActions = make('div', '', delBody);
  make('button', 'settings-delete-cancel', delActions).textContent = 'Cancel';
  make('button', 'settings-delete-go', delActions).textContent = 'Delete';
  if (dialog) {
    del.showModal = function showModal() { del.setAttribute('open', ''); };
    del.close = function close() {
      if (!del.open) { return; }
      del.removeAttribute('open');
      (del.handlers.close || []).forEach((h) => h({ type: 'close', target: del }));
    };
  }

  // Plan 15: the Notifications section, the Home area row and window.
  //
  // Amended 2026-10-18 by Phase 3.3 plan 14 (D-12): the two boxes
  // settings-alerts-on and settings-alerts-off are gone. The section is one box
  // of rows, and each row has its own variants. The distance row is
  // page.radiusRow, with its Add a home area first button page.radiusNoHome.
  const device = make('div', 'settings-device-row', main);
  const deviceOn = make('label', 'settings-device-on', device);
  const deviceSwitch = make('input', 'settings-device-switch', deviceOn);
  const deviceInstall = make('button', 'settings-device-install', device);
  deviceInstall.hidden = true;
  const deviceBlocked = make('div', 'settings-device-blocked', device);
  deviceBlocked.hidden = true;
  const deviceUnsupported = make('div', 'settings-device-unsupported', device);
  deviceUnsupported.hidden = true;
  // Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D1,
  // C9): the Critical, official, Changes and Reliable rows are gone. The distance
  // wrapper holds the select row and the Add a home area first button.
  const radiusWrap = make('div', 'settings-radius-wrap', main);
  const radiusRow = make('div', 'settings-radius-row', radiusWrap);
  radiusRow.hidden = !home;
  const select = make('select', 'settings-radius-select', radiusRow);
  select.value = String(radius);
  const radiusNoHome = make('button', 'settings-radius-nohome', radiusWrap);
  radiusNoHome.hidden = home;
  make('p', 'settings-device-error', main).hidden = true;
  make('p', 'settings-alerts-error', main).hidden = true;
  // Amended 2026-10-29 by Phase 7.1 plan 15: the sentence about removing the home
  // area (settings-home-alerts-note) left the page with the Remove home area
  // button, so the fixture no longer draws it.

  // Phase 3.3 plan 14: the install guide window.
  const guide = make('div', 'device-guide-window', doc.body);
  guide.hidden = true;
  const guidePanel = make('div', '', guide);
  const guideHead = make('div', '', guidePanel);
  make('h2', 'device-guide-title', guideHead);
  make('button', 'device-guide-close', guideHead);

  return {
    doc, bar, main, bottom, muted, logout, del, select,
    radiusRow, radiusNoHome, deviceOn, deviceSwitch,
    deviceInstall, deviceBlocked, deviceUnsupported, guide
  };
}

// fakeNotify stands in for notify.js (Phase 3.3 plan 06). Each of enable,
// disable and reconcile is answered by hand: enables, disables and reconciles
// hold the pending calls, and every call is kept in calls.
function fakeNotify({ base = 'maybe' } = {}) {
  const api = { calls: [], enables: [], disables: [], reconciles: [], base };
  api.baseState = () => { api.calls.push(['baseState']); return api.base; };
  api.deviceState = (key) => { api.calls.push(['deviceState', key]); return Promise.resolve('ready'); };
  api.enable = (key) => {
    api.calls.push(['enable', key]);
    return new Promise((resolve, reject) => { api.enables.push({ resolve, reject }); });
  };
  api.disable = () => {
    api.calls.push(['disable']);
    return new Promise((resolve, reject) => { api.disables.push({ resolve, reject }); });
  };
  api.reconcile = (key) => {
    api.calls.push(['reconcile', key]);
    return new Promise((resolve, reject) => { api.reconciles.push({ resolve, reject }); });
  };
  return api;
}

// loadAlerts evaluates settings-alerts.js against a page. fetch keeps every
// call for the test to answer. Any use of the device position throws.
function loadAlerts(options = {}) {
  const page = buildPage(options);
  const requests = [];
  const assigned = [];
  const asked = [];
  const alerted = [];
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

  class FakeMutationObserver {
    constructor(cb) { this.cb = cb; }
    observe(target, opts) { page.doc.observers.push({ target, cb: this.cb, filter: (opts && opts.attributeFilter) || null }); }
    disconnect() {}
  }
  const opened = [];
  // Phase 3.3 plan 14: notify.js is a separate script. By default the page has a
  // fake one that can prompt; notify: null leaves it out; an object is used as is.
  const notify = options.notify === undefined ? fakeNotify(options.notifyOptions) : options.notify;

  const sandbox = {
    console,
    document: page.doc,
    MutationObserver: FakeMutationObserver,
    PinalertNotify: notify === null ? undefined : notify,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject, at: now });
      });
    },
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimeout(id) { timers.delete(id); },
    AbortController: FakeAbortController,
    navigator: {
      onLine: true,
      get geolocation() { throw new Error('settings-alerts.js asked for the device position'); }
    },
    location: { assign(target) { assigned.push(String(target)); } },
    confirm(question) { asked.push(String(question)); return options.confirmAnswer; },
    alert(message) { alerted.push(String(message)); }
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'settings-alerts.js' });
  return { ...page, api: sandbox.PinalertSettingsAlerts, requests, assigned, advance, sandbox, opened, asked, alerted, notify };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: body === undefined ? () => Promise.reject(new Error('no body')) : () => Promise.resolve(body)
});

const byId = (page, id) => page.doc.byId[id];
const fire = (el, type, extra) => {
  const event = { type, target: el, key: '', shiftKey: false, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};

// pressEscape does what a browser does with Escape on an open modal dialog: a
// cancelable cancel event, then the close unless the page refused it.
const pressEscape = (page) => {
  const event = fire(page.del, 'cancel', { cancelable: true });
  if (!event.prevented) { page.del.close(); }
  return event;
};

const REPORTS = [
  { report_id: 17, available: true, text: 'Water over the bridge', category: 'Flood', severity: 'Critical', severity_level: 'critical' },
  { report_id: 18, available: false, text: '', category: '', severity: '', severity_level: '' },
  { report_id: 19, available: true, text: 'Road open again', category: 'Road', severity: 'Low', severity_level: 'low' }
];

const openMuted = async (page, reports) => {
  fire(byId(page, 'settings-muted-row'), 'click');
  await flush();
  assert.equal(page.requests.length, 1);
  answer(page.requests[0], 200, { reports });
  await flush();
};

const rows = (page) => byId(page, 'settings-muted-list').children;

test('the script leaves quietly on a page without the new markup', () => {
  const page = loadAlerts({ withMarkup: false });
  assert.equal(page.api, undefined);
});

test('opening Muted reports focuses the title, locks the page and asks for the list', async () => {
  const page = loadAlerts({ count: 3 });
  fire(byId(page, 'settings-muted-row'), 'click');
  await flush();

  assert.equal(page.muted.hidden, false);
  assert.equal(page.doc.activeElement, byId(page, 'settings-muted-title'));
  assert.equal(page.main.inert, true);
  assert.equal(page.bar.inert, true);
  assert.equal(page.bottom.inert, true);
  assert.equal(page.muted.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), 'true');
  assert.equal(byId(page, 'settings-muted-loading').hidden, false);
  assert.equal(page.requests.length, 1);
  assert.match(page.requests[0].url, /\/api\/me\/muted-reports$/);
  assert.equal(page.requests[0].init.method || 'GET', 'GET');
  assert.equal(page.requests[0].init.credentials, 'same-origin');
});

test('each muted report is one row with its words and an Unmute this report button', async () => {
  const page = loadAlerts({ count: 3 });
  await openMuted(page, REPORTS);

  assert.equal(byId(page, 'settings-muted-loading').hidden, true);
  assert.equal(byId(page, 'settings-muted-list').hidden, false);
  assert.equal(byId(page, 'settings-muted-empty').hidden, true);
  assert.equal(rows(page).length, 3);

  const first = rows(page)[0].textContent;
  assert.ok(first.includes('Water over the bridge'));
  assert.ok(first.includes('Flood · Critical'));
  assert.ok(first.includes('Unmute this report'));
  const gone = rows(page)[1].textContent;
  assert.ok(gone.includes('This report is no longer available'));
  assert.ok(!gone.includes('·'));
  assert.ok(gone.includes('Unmute this report'));

  for (const li of rows(page)) {
    const button = li.querySelectorAll()[0];
    assert.equal(button.textContent, 'Unmute this report');
    const target = button.getAttribute('aria-describedby');
    assert.ok(target && byId(page, target), 'aria-describedby points at a text block');
    assert.ok(li.contains(byId(page, target)));
  }
});

test('a reporter\'s words reach the page as text, never as markup', async () => {
  const page = loadAlerts({ count: 1 });
  await openMuted(page, [{ report_id: 5, available: true, text: '<img src=x onerror=alert(1)>', category: '<b>Flood</b>', severity: 'Low', severity_level: 'low' }]);
  assert.equal(rows(page).length, 1);
  assert.ok(rows(page)[0].textContent.includes('<img src=x onerror=alert(1)>'));
  assert.ok(rows(page)[0].textContent.includes('<b>Flood</b> · Low'));
});

test('an empty answer shows the empty state', async () => {
  const page = loadAlerts({ count: 0 });
  await openMuted(page, []);
  assert.equal(byId(page, 'settings-muted-empty').hidden, false);
  assert.equal(byId(page, 'settings-muted-list').hidden, true);
  assert.equal(byId(page, 'settings-muted-error').hidden, true);
});

test('a failed load shows the error and Try again asks again', async () => {
  const page = loadAlerts({ count: 2 });
  fire(byId(page, 'settings-muted-row'), 'click');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(byId(page, 'settings-muted-error').hidden, false);
  assert.equal(byId(page, 'settings-muted-loading').hidden, true);
  assert.equal(byId(page, 'settings-muted-list').hidden, true);

  fire(byId(page, 'settings-muted-retry'), 'click');
  await flush();
  assert.equal(page.requests.length, 2);
  assert.equal(byId(page, 'settings-muted-error').hidden, true);
  answer(page.requests[1], 200, { reports: REPORTS });
  await flush();
  assert.equal(rows(page).length, 3);
});

test('a load that never answers still ends in the error state', async () => {
  const page = loadAlerts({ count: 2 });
  fire(byId(page, 'settings-muted-row'), 'click');
  await flush();
  page.advance(11000);
  await flush();
  assert.equal(byId(page, 'settings-muted-error').hidden, false);
});

test('a report id that is not digits is never listed or requested', async () => {
  const page = loadAlerts({ count: 2 });
  await openMuted(page, [
    { report_id: '../x', available: true, text: 'Bad', category: 'Flood', severity: 'Low', severity_level: 'low' },
    { report_id: 0, available: true, text: 'Zero', category: 'Flood', severity: 'Low', severity_level: 'low' },
    { report_id: 42, available: true, text: 'Good', category: 'Flood', severity: 'Low', severity_level: 'low' }
  ]);
  assert.equal(rows(page).length, 1);
  assert.ok(rows(page)[0].textContent.includes('Good'));
});

test('Unmute this report posts, removes the row, lowers the count, moves focus and says so', async () => {
  const page = loadAlerts({ count: 3 });
  await openMuted(page, REPORTS);
  const nextButton = rows(page)[1].querySelectorAll()[0];

  fire(rows(page)[0].querySelectorAll()[0], 'click');
  await flush();
  assert.equal(page.requests.length, 2);
  assert.match(page.requests[1].url, /\/api\/reports\/17\/unmute$/);
  assert.equal(page.requests[1].init.method, 'POST');
  assert.equal(page.requests[1].init.credentials, 'same-origin');
  answer(page.requests[1], 204);
  await flush();

  assert.equal(rows(page).length, 2);
  assert.ok(!byId(page, 'settings-muted-list').textContent.includes('Water over the bridge'));
  assert.equal(byId(page, 'settings-muted-value').textContent, '2');
  assert.equal(page.doc.activeElement, nextButton);
  assert.equal(byId(page, 'notice-live').textContent, 'Updates are on again for that report.');
});

test('clearing the last mute shows the empty state and focuses its heading', async () => {
  const page = loadAlerts({ count: 1 });
  await openMuted(page, [REPORTS[0]]);
  fire(rows(page)[0].querySelectorAll()[0], 'click');
  await flush();
  answer(page.requests[1], 204);
  await flush();
  assert.equal(byId(page, 'settings-muted-value').textContent, '0');
  assert.equal(byId(page, 'settings-muted-empty').hidden, false);
  assert.equal(byId(page, 'settings-muted-list').hidden, true);
  assert.equal(page.doc.activeElement, byId(page, 'settings-muted-empty-title'));
});

test('a failed Unmute this report keeps the row and says so under it', async () => {
  const page = loadAlerts({ count: 3 });
  await openMuted(page, REPORTS);
  fire(rows(page)[0].querySelectorAll()[0], 'click');
  await flush();
  answer(page.requests[1], 500, {});
  await flush();
  assert.equal(rows(page).length, 3);
  assert.ok(rows(page)[0].textContent.includes('Couldn\'t turn updates back on. Try again.'));
  assert.equal(byId(page, 'settings-muted-value').textContent, '3');
  assert.equal(rows(page)[0].querySelectorAll()[0].disabled, false);
  // A second try is allowed and a success clears the row.
  fire(rows(page)[0].querySelectorAll()[0], 'click');
  await flush();
  assert.equal(page.requests.length, 3);
  answer(page.requests[2], 204);
  await flush();
  assert.equal(rows(page).length, 2);
});

test('the count never drops below zero', async () => {
  const page = loadAlerts({ count: 0 });
  await openMuted(page, [REPORTS[0]]);
  fire(rows(page)[0].querySelectorAll()[0], 'click');
  await flush();
  answer(page.requests[1], 204);
  await flush();
  assert.equal(byId(page, 'settings-muted-value').textContent, '0');
});

test('Log out of all devices shows its busy word, posts once and goes to the sign in page', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-logout-all-row'), 'click');
  await flush();
  assert.equal(page.logout.hidden, false);
  assert.equal(page.doc.activeElement, byId(page, 'settings-logout-all-title'));
  assert.equal(page.main.inert, true);

  const go = byId(page, 'settings-logout-all-go');
  fire(go, 'click');
  fire(go, 'click');
  await flush();
  assert.equal(page.requests.length, 1);
  assert.match(page.requests[0].url, /\/api\/me\/logout-all$/);
  assert.equal(page.requests[0].init.method, 'POST');
  assert.equal(go.textContent, 'Logging out…');
  assert.equal(go.disabled, true);

  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.assigned, ['/login?done=all-devices']);
});

test('a failed Log out of all devices shows its sentence and gives the button back', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-logout-all-row'), 'click');
  const go = byId(page, 'settings-logout-all-go');
  fire(go, 'click');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  const error = byId(page, 'settings-logout-all-error');
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, 'Couldn\'t log you out of all devices. Try again.');
  assert.equal(go.disabled, false);
  assert.equal(go.textContent, 'Log out of all devices');
  assert.deepEqual(page.assigned, []);
});

// ---- The Delete my account box (owner, 2026-10-25) --------------------------

const DELETE_FAILED = 'Couldn\'t delete your account. Try again.';

test('Delete my account opens the box with focus on the question and sends nothing', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  assert.equal(page.del.open, true);
  assert.equal(page.doc.activeElement, byId(page, 'settings-delete-title'));
  assert.equal(page.requests.length, 0);
  assert.equal(page.main.inert, false, 'the native modal makes the page inert, not lockPage');
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), null);
  assert.equal(byId(page, 'settings-delete-error').hidden, true);
});

test('Cancel closes the Delete box, sends nothing and puts focus back on the row', async () => {
  const page = loadAlerts();
  const row = byId(page, 'settings-delete-row');
  fire(row, 'click');
  await flush();
  fire(byId(page, 'settings-delete-cancel'), 'click');
  assert.equal(page.del.open, false);
  assert.equal(page.doc.activeElement, row);
  assert.equal(page.requests.length, 0);
});

test('Escape on the idle Delete box is not refused, closes it and puts focus back on the row', async () => {
  const page = loadAlerts();
  const row = byId(page, 'settings-delete-row');
  fire(row, 'click');
  await flush();
  const event = pressEscape(page);
  assert.equal(event.prevented, false);
  assert.equal(page.del.open, false);
  assert.equal(page.doc.activeElement, row);
  assert.equal(page.requests.length, 0);
});

test('while the delete is being sent Escape is refused and both buttons are disabled', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  fire(byId(page, 'settings-delete-go'), 'click');
  await flush();
  const event = pressEscape(page);
  assert.equal(event.prevented, true);
  assert.equal(page.del.open, true);
  assert.equal(byId(page, 'settings-delete-cancel').disabled, true);
  assert.equal(byId(page, 'settings-delete-go').disabled, true);
  // A press on Cancel that still reaches the script changes nothing.
  fire(byId(page, 'settings-delete-cancel'), 'click');
  assert.equal(page.del.open, true);
});

test('Delete my account sends confirm true once, shows Deleting and goes to the deleted sign in page', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  const go = byId(page, 'settings-delete-go');
  fire(go, 'click');
  fire(go, 'click');
  await flush();
  assert.equal(page.requests.length, 1);
  assert.match(page.requests[0].url, /\/api\/me\/delete$/);
  assert.equal(page.requests[0].init.method, 'POST');
  assert.deepEqual(JSON.parse(page.requests[0].init.body), { confirm: true });
  assert.equal(go.textContent, 'Deleting…');
  assert.equal(go.disabled, true);

  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.assigned, ['/login?done=deleted']);
});

test('a failed delete shows its sentence inside the open box and gives both buttons back', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  const go = byId(page, 'settings-delete-go');
  const cancel = byId(page, 'settings-delete-cancel');
  fire(go, 'click');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  const error = byId(page, 'settings-delete-error');
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, DELETE_FAILED);
  assert.equal(page.del.open, true);
  assert.equal(go.disabled, false);
  assert.equal(go.textContent, 'Delete');
  assert.equal(cancel.disabled, false);
  assert.equal(page.doc.activeElement, byId(page, 'settings-delete-title'));
  assert.deepEqual(page.assigned, []);

  // Escape works again once the failure is shown.
  const event = pressEscape(page);
  assert.equal(event.prevented, false);
  assert.equal(page.del.open, false);
});

test('a network failure on Delete shows the same sentence in the open box', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  fire(byId(page, 'settings-delete-go'), 'click');
  await flush();
  page.requests[0].reject(new Error('offline'));
  await flush();
  assert.equal(byId(page, 'settings-delete-error').textContent, DELETE_FAILED);
  assert.equal(byId(page, 'settings-delete-error').hidden, false);
  assert.equal(page.del.open, true);
  assert.equal(byId(page, 'settings-delete-go').disabled, false);
  assert.equal(byId(page, 'settings-delete-go').textContent, 'Delete');
  assert.deepEqual(page.assigned, []);
});

test('a failure that lands after the browser shut the box opens it again with the sentence', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  fire(byId(page, 'settings-delete-go'), 'click');
  await flush();
  page.del.close();
  assert.equal(page.del.open, false);
  assert.equal(page.doc.activeElement, byId(page, 'settings-delete-row'));
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(page.del.open, true);
  assert.equal(byId(page, 'settings-delete-error').hidden, false);
  assert.equal(byId(page, 'settings-delete-error').textContent, DELETE_FAILED);
  assert.equal(page.doc.activeElement, byId(page, 'settings-delete-title'));
});

test('the Delete box has no click handler and a click on it leaves it open', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  assert.equal((page.del.handlers.click || []).length, 0, 'no click handler on the dialog');
  fire(page.del, 'click');
  fire(byId(page, 'settings-delete-title'), 'click');
  assert.equal(page.del.open, true);
  assert.equal(page.requests.length, 0);
});

test('without dialog support a refused confirm sends nothing and asks the question', async () => {
  const page = loadAlerts({ dialog: false, confirmAnswer: false });
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  assert.deepEqual(page.asked, ['Delete your account?']);
  assert.equal(page.requests.length, 0);
});

test('without dialog support an accepted confirm sends once and goes to the deleted sign in page', async () => {
  const page = loadAlerts({ dialog: false, confirmAnswer: true });
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  assert.equal(page.requests.length, 1);
  assert.match(page.requests[0].url, /\/api\/me\/delete$/);
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.assigned, ['/login?done=deleted']);
});

test('without dialog support a failed delete is told with an alert', async () => {
  const page = loadAlerts({ dialog: false, confirmAnswer: true });
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.deepEqual(page.alerted, [DELETE_FAILED]);
  assert.deepEqual(page.assigned, []);
});

test('the X closes a window and returns focus to the row that opened it', async () => {
  const page = loadAlerts();
  const row = byId(page, 'settings-logout-all-row');
  fire(row, 'click');
  await flush();
  fire(byId(page, 'settings-logout-all-close'), 'click');
  assert.equal(page.logout.hidden, true);
  assert.equal(page.main.inert, false);
  assert.equal(page.doc.documentElement.getAttribute('data-modal-open'), null);
  assert.equal(page.doc.activeElement, row);
});

test('Escape closes a window; a click on the window itself closes nothing', async () => {
  const page = loadAlerts();
  const row = byId(page, 'settings-logout-all-row');
  fire(row, 'click');
  await flush();
  assert.equal((page.logout.handlers.click || []).length, 0, 'no click handler on a window element');
  assert.equal((page.muted.handlers.click || []).length, 0);
  assert.equal((page.del.handlers.click || []).length, 0);
  fire(page.logout, 'click');
  assert.equal(page.logout.hidden, false);
  fire(page.logout, 'keydown', { key: 'Escape' });
  assert.equal(page.logout.hidden, true);
  assert.equal(page.doc.activeElement, row);
});

test('Tab stays inside an open window', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-logout-all-row'), 'click');
  await flush();
  const close = byId(page, 'settings-logout-all-close');
  const go = byId(page, 'settings-logout-all-go');
  go.focus();
  const forward = fire(page.logout, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.equal(page.doc.activeElement, close);
  const back = fire(page.logout, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.equal(page.doc.activeElement, go);
});

test('the script never asks for the device position and exposes its handles', () => {
  const page = loadAlerts();
  // Amended 2026-10-18 by Phase 3.3 plan 14 (D-12): drawDevice is exported too.
  // Amended 2026-11-03 by quick 261103-jal (D1): saveKind left with the switches.
  for (const name of ['openWindow', 'closeWindow', 'loadMuted', 'unmute', 'logOutAll', 'openDeleteConfirm', 'deleteAccount', 'saveAlerts', 'redrawAlerts', 'drawDevice']) {
    assert.equal(typeof page.api[name], 'function', name);
  }
});

// ---- Plan 15: the Notifications section (NOTIF-05, UI-SPEC E1) --------------
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D1,
// C9): the section has two rows. The Reliable switch tests of this block became
// the same save, error, offline and next save cases on the distance select, and
// the no home tests moved to the distance row's Add a home area first button
// (settings-radius-nohome). The device row tests are below.

const SAVE_FAILED = 'Couldn\'t save that. Try again.';
const OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
// Amended 2026-10-29 by Phase 7.1 plan 15 (D-15): a home area can no longer be
// removed in Settings, so the status "Home area removed. Alerts near your home
// area are off." (3.3 plan 14, UI-SPEC W2) is gone with the Remove home area
// button. The no home button stays as a defensive state for data repair: the
// tests below that empty the Home area row's data-value expect no status line.

const ok = (km, home = true) => ({ alert_radius_km: km, has_home_area: home });
const errorLine = (page) => byId(page, 'settings-alerts-error');
const live = (page) => byId(page, 'notice-live');

// pick chooses a distance the way a tap does: the select changes, then the
// change event fires.
const pick = (page, value) => {
  page.select.value = String(value);
  fire(page.select, 'change');
};

test('picking a distance sends PATCH alert_radius_km as a number, locks the select and keeps the answer', async () => {
  const page = loadAlerts({ radius: 5 });
  pick(page, 10);
  await flush();

  assert.equal(page.requests.length, 1);
  const req = page.requests[0];
  assert.match(req.url, /\/api\/me\/alerts$/);
  assert.equal(req.init.method, 'PATCH');
  assert.equal(req.init.credentials, 'same-origin');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.equal(req.init.body, '{"alert_radius_km":10}');
  assert.equal(page.select.disabled, true, 'the sender is disabled while the save is in flight');
  assert.equal(page.deviceSwitch.disabled, false);

  answer(req, 200, ok(10));
  await flush();
  assert.equal(page.select.value, '10');
  assert.equal(page.select.disabled, false);
  assert.equal(errorLine(page).hidden, true);
});

test('a failed save puts the distance back and says so in one error line', async () => {
  const page = loadAlerts({ radius: 5 });
  pick(page, 2);
  await flush();
  answer(page.requests[0], 500, { error: 'boom' });
  await flush();
  assert.equal(page.select.value, '5');
  assert.equal(page.select.disabled, false);
  assert.equal(errorLine(page).hidden, false);
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

test('a 400 puts the distance back with the same sentence', async () => {
  const page = loadAlerts({ radius: 5 });
  pick(page, 2);
  await flush();
  answer(page.requests[0], 400, { field: 'alert_radius_km', error: 'Choose 1, 2, 5 or 10 km.' });
  await flush();
  assert.equal(page.select.value, '5');
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

test('a network failure puts the distance back with the same sentence', async () => {
  const page = loadAlerts({ radius: 10 });
  pick(page, 1);
  await flush();
  page.requests[0].reject(new Error('network down'));
  await flush();
  assert.equal(page.select.value, '10');
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

test('a save that never answers ends in the same sentence', async () => {
  const page = loadAlerts({ radius: 5 });
  pick(page, 2);
  await flush();
  page.advance(10001);
  await flush();
  assert.equal(page.select.value, '5');
  assert.equal(page.select.disabled, false);
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

test('offline, nothing is sent and the offline sentence shows', async () => {
  const page = loadAlerts({ radius: 5 });
  page.sandbox.navigator.onLine = false;
  pick(page, 2);
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(page.select.value, '5');
  assert.equal(errorLine(page).hidden, false);
  assert.equal(errorLine(page).textContent, OFFLINE);
});

test('the next save clears the earlier error line', async () => {
  const page = loadAlerts({ radius: 5 });
  pick(page, 2);
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(errorLine(page).hidden, false);
  pick(page, 2);
  await flush();
  assert.equal(errorLine(page).hidden, true);
  answer(page.requests[1], 200, ok(2));
  await flush();
  assert.equal(errorLine(page).hidden, true);
});

test('a failed distance save puts the previous option back, and a second pick goes back to the last saved one', async () => {
  const page = loadAlerts({ radius: 5 });
  pick(page, 2);
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(page.select.value, '5');
  assert.equal(errorLine(page).textContent, SAVE_FAILED);

  pick(page, 1);
  await flush();
  answer(page.requests[1], 200, ok(1));
  await flush();
  pick(page, 10);
  await flush();
  page.requests[2].reject(new Error('down'));
  await flush();
  assert.equal(page.select.value, '1');
});

test('a distance that is not a whole number is never sent', async () => {
  const page = loadAlerts({ radius: 5 });
  page.select.value = '';
  fire(page.select, 'change');
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(page.select.value, '5');
});

// Amended 2026-11-13 by quick 261113-t4y (owner decision 2026-11-13, the compact
// profile): the Home area row and window left Settings with About you and are on
// the profile page behind Edit profile. The Add a home area first button now goes
// to the profile page, and the section no longer follows a Home area row, so the
// tests that drove the window and the row are replaced by these.
test('the Add a home area first button goes to the profile page', async () => {
  const page = loadAlerts({ home: false });
  fire(page.radiusNoHome, 'click');
  assert.deepEqual(page.assigned, ['/me']);
  assert.deepEqual(page.opened, [], 'no Home area window is opened from Settings');
  assert.equal(page.requests.length, 0);
});

test('the distance select still saves with no Home area row or window on the page', async () => {
  const page = loadAlerts({ home: true, radius: 5 });
  assert.equal(page.doc.byId['settings-home-row'], undefined);
  assert.equal(page.doc.byId['settings-home-window'], undefined);
  pick(page, 10);
  await flush();
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.body, '{"alert_radius_km":10}');
  answer(page.requests[0], 200, ok(10));
  await flush();
  assert.equal(page.select.value, '10');
});

test('with no home area the page draws the button and not the select', async () => {
  const page = loadAlerts({ home: false });
  assert.equal(page.radiusRow.hidden, true);
  assert.equal(page.radiusNoHome.hidden, false);
  assert.equal(page.requests.length, 0);
});

test('a failed distance save puts the old distance back', async () => {
  const page = loadAlerts({ home: true, radius: 5 });
  pick(page, 2);
  await flush();
  page.requests[0].reject(new Error('down'));
  await flush();
  assert.equal(page.select.value, '5');
});

test('saveAlerts and redrawAlerts are the same paths the controls use', async () => {
  const page = loadAlerts({ home: false });
  page.api.redrawAlerts(true);
  assert.equal(page.radiusRow.hidden, false);
  assert.equal(page.radiusNoHome.hidden, true);
  page.api.redrawAlerts(false);
  assert.equal(page.radiusRow.hidden, true);
  assert.equal(page.radiusNoHome.hidden, false);

  const pending = page.api.saveAlerts({ alert_radius_km: 2 });
  await flush();
  assert.equal(page.requests[0].init.body, '{"alert_radius_km":2}');
  answer(page.requests[0], 200, ok(2));
  assert.equal(await pending, true);
  assert.equal(page.select.value, '2');
});

test('saveAlerts sends the distance alone and ignores any other key', async () => {
  const page = loadAlerts();
  assert.equal(await page.api.saveAlerts({ critical_push: true }), false);
  assert.equal(page.requests.length, 0);
  const pending = page.api.saveAlerts({ alert_radius_km: 1, reliable_alerts: true });
  await flush();
  assert.equal(page.requests[0].init.body, '{"alert_radius_km":1}');
  answer(page.requests[0], 200, ok(1));
  assert.equal(await pending, true);
});

test('every alert string reaches the page as text', async () => {
  const page = loadAlerts({ radius: 5 });
  // innerHTML and outerHTML throw in the fake element, so a markup sink would fail here.
  pick(page, 2);
  await flush();
  answer(page.requests[0], 500, { error: '<b>bad</b>' });
  await flush();
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

// ---- Phase 3.3 plan 14: the device row (PWA-03, PWA-06) ----
//
// The UI-SPEC strings, written out here so a change to the script's words fails.
const DEVICE_ON = 'Alerts are on for this device.';
const DEVICE_OFF = 'Alerts are off for this device.';
const TURN_ON_FAILED = 'Couldn\'t turn this on. Try again.';
const TURN_OFF_FAILED = 'Couldn\'t turn this off. Try again.';

const deviceError = (page) => byId(page, 'settings-device-error');
const shownVariant = (page) => ['deviceOn', 'deviceInstall', 'deviceBlocked', 'deviceUnsupported'].filter((n) => page[n].hidden === false);

// tapDevice flips the device switch the way a tap does: the checkbox changes,
// then the change event fires, with nothing awaited in between.
const tapDevice = (page, state) => {
  page.deviceSwitch.checked = state;
  fire(page.deviceSwitch, 'change');
};

test('a page without notify.js draws the unsupported variant', () => {
  const page = loadAlerts({ notify: null });
  assert.deepEqual(shownVariant(page), ['deviceUnsupported']);
});

test('an empty push key draws the unsupported variant and asks nothing', () => {
  const page = loadAlerts({ key: '' });
  assert.deepEqual(shownVariant(page), ['deviceUnsupported']);
  assert.equal(page.notify.calls.filter((c) => c[0] === 'reconcile').length, 0);
});

for (const [base, variant] of [['needs-install', 'deviceInstall'], ['blocked', 'deviceBlocked'], ['unsupported', 'deviceUnsupported']]) {
  test('the base state ' + base + ' draws its variant at once and asks the server nothing', async () => {
    const page = loadAlerts({ notifyOptions: { base } });
    assert.deepEqual(shownVariant(page), [variant]);
    await flush();
    assert.equal(page.notify.calls.filter((c) => c[0] === 'reconcile' || c[0] === 'enable').length, 0);
    assert.equal(page.requests.length, 0);
  });
}

test('a device that may work shows the unchecked switch at once, then follows the server', async () => {
  const page = loadAlerts();
  assert.deepEqual(shownVariant(page), ['deviceOn']);
  assert.equal(page.deviceSwitch.checked, false);
  assert.deepEqual(page.notify.calls.filter((c) => c[0] === 'reconcile'), [['reconcile', 'BPUBLICKEY']]);
  page.notify.reconciles[0].resolve('on');
  await flush();
  assert.equal(page.deviceSwitch.checked, true);
  assert.deepEqual(shownVariant(page), ['deviceOn']);
});

test('a server that does not know this endpoint leaves the switch off', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].resolve('ready');
  await flush();
  assert.equal(page.deviceSwitch.checked, false);
});

test('the server word blocked on load draws the blocked row', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].resolve('blocked');
  await flush();
  assert.deepEqual(shownVariant(page), ['deviceBlocked']);
});

test('a failed check on load leaves the unchecked switch and shows no error', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].reject(new Error('down'));
  await flush();
  assert.deepEqual(shownVariant(page), ['deviceOn']);
  assert.equal(page.deviceSwitch.checked, false);
  assert.equal(deviceError(page).hidden, true);
});

test('a late answer on load never undoes a tap on the switch', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.reconciles[0].resolve('ready');
  await flush();
  assert.equal(page.deviceSwitch.checked, true, 'the tap stands while it works');
  assert.equal(page.deviceSwitch.disabled, true);
});

test('turning On this device on calls enable from the tap itself and holds the switch busy', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  // No await between the tap and the check: the browser prompt needs the tap.
  assert.deepEqual(page.notify.calls.filter((c) => c[0] === 'enable'), [['enable', 'BPUBLICKEY']]);
  assert.equal(page.deviceSwitch.disabled, true);
  assert.equal(page.deviceSwitch.getAttribute('aria-busy'), 'true');
  page.notify.enables[0].resolve({ ok: true, state: 'on' });
  await flush();
  assert.equal(page.deviceSwitch.checked, true);
  assert.equal(page.deviceSwitch.disabled, false);
  assert.equal(page.deviceSwitch.getAttribute('aria-busy'), null);
  assert.equal(live(page).textContent, DEVICE_ON);
  assert.equal(deviceError(page).hidden, true);
});

test('a second tap while the first works does nothing', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  tapDevice(page, false);
  assert.equal(page.notify.enables.length, 1);
  assert.equal(page.notify.disables.length, 0);
});

test('a refused prompt that blocks the browser draws the blocked row and says off', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].resolve({ ok: false, state: 'blocked', reason: 'refused' });
  await flush();
  assert.deepEqual(shownVariant(page), ['deviceBlocked']);
  assert.equal(live(page).textContent, DEVICE_OFF);
  assert.equal(deviceError(page).hidden, true);
});

test('a dismissed prompt keeps the switch, unchecked, and says off', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].resolve({ ok: false, state: 'ready', reason: 'refused' });
  await flush();
  assert.deepEqual(shownVariant(page), ['deviceOn']);
  assert.equal(page.deviceSwitch.checked, false);
  assert.equal(page.deviceSwitch.disabled, false);
  assert.equal(live(page).textContent, DEVICE_OFF);
});

test('turning on offline unchecks the switch and shows the offline sentence', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].resolve({ ok: false, state: 'ready', reason: 'offline' });
  await flush();
  assert.equal(page.deviceSwitch.checked, false);
  assert.equal(page.deviceSwitch.disabled, false);
  assert.equal(deviceError(page).hidden, false);
  assert.equal(deviceError(page).textContent, OFFLINE);
});

for (const reason of ['failed', 'session']) {
  test('turning on that ends in ' + reason + ' unchecks the switch and says it could not', async () => {
    const page = loadAlerts();
    tapDevice(page, true);
    page.notify.enables[0].resolve({ ok: false, state: 'ready', reason });
    await flush();
    assert.equal(page.deviceSwitch.checked, false);
    assert.equal(deviceError(page).textContent, TURN_ON_FAILED);
    assert.equal(deviceError(page).hidden, false);
  });
}

test('turning on that throws unchecks the switch and says it could not', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].reject(new Error('boom'));
  await flush();
  assert.equal(page.deviceSwitch.checked, false);
  assert.equal(page.deviceSwitch.disabled, false);
  assert.equal(deviceError(page).textContent, TURN_ON_FAILED);
});

test('a device the answer calls unsupported or needing install draws that variant', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].resolve({ ok: false, state: 'needs-install', reason: 'unsupported' });
  await flush();
  assert.deepEqual(shownVariant(page), ['deviceInstall']);
});

test('turning On this device off calls disable, says off and keeps every other row', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].resolve('on');
  await flush();
  tapDevice(page, false);
  assert.equal(page.notify.disables.length, 1);
  assert.equal(page.deviceSwitch.disabled, true);
  page.notify.disables[0].resolve({ ok: true, state: 'ready' });
  await flush();
  assert.equal(page.deviceSwitch.checked, false);
  assert.equal(page.deviceSwitch.disabled, false);
  assert.equal(live(page).textContent, DEVICE_OFF);
  // Nothing is hidden or greyed when the device is off (ROADMAP criterion 5).
  // Amended 2026-11-03 by quick 261103-jal (D1): the distance row is the only
  // other row.
  assert.equal(page.radiusRow.hidden, false);
  assert.equal(page.select.disabled, false);
});

test('a failed turn off puts the switch back on with its sentence', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].resolve('on');
  await flush();
  tapDevice(page, false);
  page.notify.disables[0].resolve({ ok: false, state: 'on', reason: 'failed' });
  await flush();
  assert.equal(page.deviceSwitch.checked, true);
  assert.equal(deviceError(page).textContent, TURN_OFF_FAILED);
});

test('a turn off that throws puts the switch back on with its sentence', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].resolve('on');
  await flush();
  tapDevice(page, false);
  page.notify.disables[0].reject(new Error('boom'));
  await flush();
  assert.equal(page.deviceSwitch.checked, true);
  assert.equal(deviceError(page).textContent, TURN_OFF_FAILED);
});

test('turning off offline puts the switch back on with the offline sentence, not the turn off one', async () => {
  const page = loadAlerts();
  page.notify.reconciles[0].resolve('on');
  await flush();
  tapDevice(page, false);
  page.notify.disables[0].resolve({ ok: false, state: 'on', reason: 'offline' });
  await flush();
  assert.equal(page.deviceSwitch.checked, true);
  assert.equal(deviceError(page).textContent, OFFLINE);
});

test('the next device action clears the earlier device error', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].resolve({ ok: false, state: 'ready', reason: 'failed' });
  await flush();
  assert.equal(deviceError(page).hidden, false);
  tapDevice(page, true);
  assert.equal(deviceError(page).hidden, true);
});

test('the needs install row opens the guide window, focus on its title, and the X returns focus to the row', async () => {
  const page = loadAlerts({ notifyOptions: { base: 'needs-install' } });
  fire(page.deviceInstall, 'click');
  assert.equal(page.guide.hidden, false);
  assert.equal(page.doc.activeElement, byId(page, 'device-guide-title'));
  assert.equal(page.main.inert, true);
  fire(byId(page, 'device-guide-close'), 'click');
  assert.equal(page.guide.hidden, true);
  assert.equal(page.main.inert, false);
  assert.equal(page.doc.activeElement, page.deviceInstall);
});

test('Escape closes the guide window and a click on the window itself closes nothing', async () => {
  const page = loadAlerts({ notifyOptions: { base: 'needs-install' } });
  fire(page.deviceInstall, 'click');
  assert.equal((page.guide.handlers.click || []).length, 0, 'no click handler on the window');
  fire(page.guide, 'keydown', { key: 'Escape' });
  assert.equal(page.guide.hidden, true);
  assert.equal(page.doc.activeElement, page.deviceInstall);
});

// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D1):
// the Critical, Changes and official switch tests (the save, the failed save, the
// 400, the offline save, saveKind, the no home rows and the home area redraws of
// each) are gone with the switches. The distance row's own cases are above; the
// device row cases stay.

test('drawDevice draws each variant', () => {
  const page = loadAlerts();
  for (const [state, variant] of [['needs-install', 'deviceInstall'], ['blocked', 'deviceBlocked'], ['unsupported', 'deviceUnsupported'], ['ready', 'deviceOn'], ['on', 'deviceOn']]) {
    page.api.drawDevice(state);
    assert.deepEqual(shownVariant(page), [variant], state);
    assert.equal(page.deviceSwitch.checked, state === 'on', state);
  }
});

test('the device switch is the only switch the script wires: no kind switch path is left', async () => {
  const page = loadAlerts();
  assert.equal(page.api.saveKind, undefined);
  // A change event on the device switch never reaches the alerts route.
  tapDevice(page, true);
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(page.notify.enables.length, 1);
});

test('every device string reaches the page as text', async () => {
  const page = loadAlerts();
  tapDevice(page, true);
  page.notify.enables[0].resolve({ ok: false, state: 'ready', reason: 'failed', message: '<b>bad</b>' });
  await flush();
  assert.equal(deviceError(page).textContent, TURN_ON_FAILED);
});
