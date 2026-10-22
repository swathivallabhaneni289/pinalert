// settings_alerts_test.mjs - runs the real web/static/js/settings-alerts.js in a
// Node sandbox and checks the Muted reports window and the two account confirm
// windows of the Settings page (Phase 3.2 plan 12, D-11, D-23, UI-SPEC E2, E3):
//
//   - Muted reports loads GET /api/me/muted-reports, lists each report with its
//     text, category and severity or "This report is no longer available", and
//     shows the empty and error states;
//   - Unmute this report posts to the report's own unmute address, removes the
//     row at once, lowers the count, moves focus on and says so politely;
//   - Log out of all devices and Delete my account each post once, show their
//     busy word, go to their fixed sign in address on success and show their
//     sentence on failure;
//   - every window opens with focus on its title, locks the page, keeps Tab
//     inside and closes with the X or Escape only;
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
function buildPage({ count = 0, withMarkup = true, home = true, on = false, radius = 5 } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    observers: [],
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

  const del = make('div', 'settings-delete-window', doc.body);
  del.hidden = true;
  const delPanel = make('div', '', del);
  const delHead = make('div', '', delPanel);
  make('h2', 'settings-delete-title', delHead);
  make('button', 'settings-delete-close', delHead);
  make('p', 'settings-delete-error', delPanel).hidden = true;
  make('button', 'settings-delete-go', delPanel).textContent = 'Delete my account';

  // Plan 15: the Notifications section, the Home area row and window.
  const alertsOn = make('div', 'settings-alerts-on', main);
  alertsOn.hidden = !home;
  const sw = make('input', 'settings-reliable-switch', alertsOn);
  sw.checked = on;
  const select = make('select', 'settings-radius-select', alertsOn);
  select.value = String(radius);
  const alertsOff = make('div', 'settings-alerts-off', main);
  alertsOff.hidden = home;
  make('button', 'settings-reliable-nohome', alertsOff);
  make('p', 'settings-alerts-error', main).hidden = true;
  const homeRow = make('button', 'settings-home-row', main);
  homeRow.setAttribute('data-value', home ? 'Kochi' : '');
  const homeWin = make('div', 'settings-home-window', doc.body);
  homeWin.hidden = true;
  make('p', 'settings-home-alerts-note', homeWin).hidden = !on;

  return { doc, bar, main, bottom, muted, logout, del, alertsOn, alertsOff, sw, select, homeRow, homeWin };
}

// loadAlerts evaluates settings-alerts.js against a page. fetch keeps every
// call for the test to answer. Any use of the device position throws.
function loadAlerts(options) {
  const page = buildPage(options);
  const requests = [];
  const assigned = [];
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

  const sandbox = {
    console,
    document: page.doc,
    MutationObserver: FakeMutationObserver,
    PinalertSettings: { openWindow(which) { opened.push(which); } },
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
    location: { assign(target) { assigned.push(String(target)); } }
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'settings-alerts.js' });
  return { ...page, api: sandbox.PinalertSettingsAlerts, requests, assigned, advance, sandbox, opened };
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

test('Delete my account sends confirm true, shows Deleting and goes to the deleted sign in page', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  assert.equal(page.doc.activeElement, byId(page, 'settings-delete-title'));

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

test('a failed Delete my account shows its sentence and gives the button back', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  const go = byId(page, 'settings-delete-go');
  fire(go, 'click');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  const error = byId(page, 'settings-delete-error');
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, 'Couldn\'t delete your account. Try again.');
  assert.equal(go.disabled, false);
  assert.equal(go.textContent, 'Delete my account');
  assert.deepEqual(page.assigned, []);
});

test('a network failure on Delete my account shows the same sentence', async () => {
  const page = loadAlerts();
  fire(byId(page, 'settings-delete-row'), 'click');
  fire(byId(page, 'settings-delete-go'), 'click');
  await flush();
  page.requests[0].reject(new Error('offline'));
  await flush();
  assert.equal(byId(page, 'settings-delete-error').textContent, 'Couldn\'t delete your account. Try again.');
  assert.deepEqual(page.assigned, []);
});

test('the X closes a window and returns focus to the row that opened it', async () => {
  const page = loadAlerts();
  const row = byId(page, 'settings-delete-row');
  fire(row, 'click');
  await flush();
  fire(byId(page, 'settings-delete-close'), 'click');
  assert.equal(page.del.hidden, true);
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
  fire(byId(page, 'settings-delete-row'), 'click');
  await flush();
  const close = byId(page, 'settings-delete-close');
  const go = byId(page, 'settings-delete-go');
  go.focus();
  const forward = fire(page.del, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.equal(page.doc.activeElement, close);
  const back = fire(page.del, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.equal(page.doc.activeElement, go);
});

test('the script never asks for the device position and exposes its handles', () => {
  const page = loadAlerts();
  for (const name of ['openWindow', 'closeWindow', 'loadMuted', 'unmute', 'logOutAll', 'deleteAccount', 'saveAlerts', 'redrawAlerts']) {
    assert.equal(typeof page.api[name], 'function', name);
  }
});

// ---- Plan 15: the Notifications section (NOTIF-05, UI-SPEC E1) --------------

const SAVE_FAILED = 'Couldn\'t save that. Try again.';
const OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
const HOME_REMOVED = 'Home area removed. Reliable reporter alerts are off.';

const ok = (reliable, km) => ({ reliable_alerts: reliable, alert_radius_km: km, has_home_area: true });
const errorLine = (page) => byId(page, 'settings-alerts-error');
const note = (page) => byId(page, 'settings-home-alerts-note');
const live = (page) => byId(page, 'notice-live');

// turnOn flips the switch the way a tap does: the checkbox changes, then the
// change event fires.
const turnSwitch = (page, state) => {
  page.sw.checked = state;
  fire(page.sw, 'change');
};

test('turning the switch on sends PATCH reliable_alerts, locks the switch and keeps the answer', async () => {
  const page = loadAlerts();
  turnSwitch(page, true);
  await flush();

  assert.equal(page.requests.length, 1);
  const req = page.requests[0];
  assert.match(req.url, /\/api\/me\/alerts$/);
  assert.equal(req.init.method, 'PATCH');
  assert.equal(req.init.credentials, 'same-origin');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.equal(req.init.body, '{"reliable_alerts":true}');
  assert.equal(page.sw.disabled, true, 'the sender is disabled while the save is in flight');
  assert.equal(page.select.disabled, false);
  assert.equal(note(page).hidden, false, 'the Home area note shows while the switch is on');

  answer(req, 200, ok(true, 5));
  await flush();
  assert.equal(page.sw.checked, true);
  assert.equal(page.sw.disabled, false);
  assert.equal(errorLine(page).hidden, true);
  assert.equal(note(page).hidden, false);
});

test('turning the switch off sends reliable_alerts false and hides the Home area note', async () => {
  const page = loadAlerts({ on: true });
  assert.equal(note(page).hidden, false);
  turnSwitch(page, false);
  await flush();
  assert.equal(page.requests[0].init.body, '{"reliable_alerts":false}');
  assert.equal(note(page).hidden, true);
  answer(page.requests[0], 200, ok(false, 5));
  await flush();
  assert.equal(page.sw.checked, false);
  assert.equal(note(page).hidden, true);
});

test('a failed save puts the switch back and says so in one error line', async () => {
  const page = loadAlerts();
  turnSwitch(page, true);
  await flush();
  answer(page.requests[0], 500, { error: 'boom' });
  await flush();
  assert.equal(page.sw.checked, false);
  assert.equal(page.sw.disabled, false);
  assert.equal(errorLine(page).hidden, false);
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
  assert.equal(note(page).hidden, true);
});

test('a 400 puts the switch back with the same sentence', async () => {
  const page = loadAlerts();
  turnSwitch(page, true);
  await flush();
  answer(page.requests[0], 400, { field: 'reliable_alerts', error: 'Add a home area first.' });
  await flush();
  assert.equal(page.sw.checked, false);
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

test('a network failure puts the switch back with the same sentence', async () => {
  const page = loadAlerts({ on: true });
  turnSwitch(page, false);
  await flush();
  page.requests[0].reject(new Error('network down'));
  await flush();
  assert.equal(page.sw.checked, true);
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
  assert.equal(note(page).hidden, false);
});

test('a save that never answers ends in the same sentence', async () => {
  const page = loadAlerts();
  turnSwitch(page, true);
  await flush();
  page.advance(10001);
  await flush();
  assert.equal(page.sw.checked, false);
  assert.equal(page.sw.disabled, false);
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});

test('offline, nothing is sent and the offline sentence shows', async () => {
  const page = loadAlerts();
  page.sandbox.navigator.onLine = false;
  turnSwitch(page, true);
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(page.sw.checked, false);
  assert.equal(errorLine(page).hidden, false);
  assert.equal(errorLine(page).textContent, OFFLINE);
});

test('the next save clears the earlier error line', async () => {
  const page = loadAlerts();
  turnSwitch(page, true);
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(errorLine(page).hidden, false);
  turnSwitch(page, true);
  await flush();
  assert.equal(errorLine(page).hidden, true);
  answer(page.requests[1], 200, ok(true, 5));
  await flush();
  assert.equal(errorLine(page).hidden, true);
});

test('picking a distance sends alert_radius_km as a number and keeps the answer', async () => {
  const page = loadAlerts({ radius: 5 });
  page.select.value = '10';
  fire(page.select, 'change');
  await flush();
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.equal(page.requests[0].init.body, '{"alert_radius_km":10}');
  assert.equal(page.select.disabled, true);
  assert.equal(page.sw.disabled, false);
  answer(page.requests[0], 200, ok(false, 10));
  await flush();
  assert.equal(page.select.value, '10');
  assert.equal(page.select.disabled, false);
  assert.equal(errorLine(page).hidden, true);
});

test('a failed distance save puts the previous option back, and a second pick goes back to the last saved one', async () => {
  const page = loadAlerts({ radius: 5 });
  page.select.value = '2';
  fire(page.select, 'change');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(page.select.value, '5');
  assert.equal(errorLine(page).textContent, SAVE_FAILED);

  page.select.value = '1';
  fire(page.select, 'change');
  await flush();
  answer(page.requests[1], 200, ok(false, 1));
  await flush();
  page.select.value = '10';
  fire(page.select, 'change');
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

test('the Add a home area first row opens the Home area window', async () => {
  const page = loadAlerts({ home: false });
  fire(byId(page, 'settings-reliable-nohome'), 'click');
  assert.deepEqual(page.opened, ['home']);
});

test('closing the Home area window after saving one shows the switch and focuses it', async () => {
  const page = loadAlerts({ home: false });
  assert.equal(page.alertsOn.hidden, true);
  assert.equal(page.alertsOff.hidden, false);
  fire(byId(page, 'settings-reliable-nohome'), 'click');
  page.homeWin.hidden = false;
  await flush();

  // What settings.js does after a save: the row takes the stored name, the
  // window closes, focus goes to the Home area row.
  page.homeRow.setAttribute('data-value', 'Kochi');
  page.homeWin.hidden = true;
  page.homeRow.focus();
  await flush();

  assert.equal(page.alertsOn.hidden, false);
  assert.equal(page.alertsOff.hidden, true);
  assert.equal(page.sw.checked, false, 'a new home area does not switch the alert on');
  assert.equal(page.doc.activeElement, page.sw);
  assert.equal(page.requests.length, 0);
});

test('closing the Home area window with no home area puts focus back on the row that opened it', async () => {
  const page = loadAlerts({ home: false });
  const nohome = byId(page, 'settings-reliable-nohome');
  fire(nohome, 'click');
  page.homeWin.hidden = false;
  await flush();
  page.homeWin.hidden = true;
  page.homeRow.focus();
  await flush();
  assert.equal(page.alertsOn.hidden, true);
  assert.equal(page.alertsOff.hidden, false);
  assert.equal(page.doc.activeElement, nohome);
});

test('a Home area window opened from the Home area row keeps focus where settings.js put it', async () => {
  const page = loadAlerts({ home: false });
  page.homeWin.hidden = false;
  await flush();
  page.homeRow.setAttribute('data-value', 'Kochi');
  page.homeWin.hidden = true;
  page.homeRow.focus();
  await flush();
  assert.equal(page.alertsOn.hidden, false);
  assert.equal(page.doc.activeElement, page.homeRow);
});

test('removing the home area while the alert is on turns the switch off, says so and sends nothing', async () => {
  const page = loadAlerts({ home: true, on: true });
  page.homeRow.setAttribute('data-value', '');
  await flush();

  assert.equal(page.alertsOn.hidden, true);
  assert.equal(page.alertsOff.hidden, false);
  assert.equal(page.sw.checked, false);
  assert.equal(note(page).hidden, true);
  assert.equal(live(page).textContent, HOME_REMOVED);
  assert.equal(page.requests.length, 0);
});

test('removing the home area while the alert is off redraws without a status line', async () => {
  const page = loadAlerts({ home: true, on: false });
  page.homeRow.setAttribute('data-value', '');
  await flush();
  assert.equal(page.alertsOn.hidden, true);
  assert.equal(page.alertsOff.hidden, false);
  assert.equal(live(page).textContent, '');
});

test('changing the home area to another place changes nothing in the section', async () => {
  const page = loadAlerts({ home: true, on: true, radius: 10 });
  page.homeRow.setAttribute('data-value', 'Thrissur');
  await flush();
  assert.equal(page.alertsOn.hidden, false);
  assert.equal(page.alertsOff.hidden, true);
  assert.equal(page.sw.checked, true);
  assert.equal(page.select.value, '10');
  assert.equal(live(page).textContent, '');
  assert.equal(note(page).hidden, false);
});

test('a failed switch save after the home area was removed leaves the switch off', async () => {
  const page = loadAlerts({ home: true, on: true });
  turnSwitch(page, false);
  await flush();
  page.homeRow.setAttribute('data-value', '');
  await flush();
  page.requests[0].reject(new Error('down'));
  await flush();
  assert.equal(page.sw.checked, false);
  assert.equal(note(page).hidden, true);
});

test('saveAlerts and redrawAlerts are the same paths the controls use', async () => {
  const page = loadAlerts({ home: false });
  page.api.redrawAlerts(true);
  assert.equal(page.alertsOn.hidden, false);
  assert.equal(page.alertsOff.hidden, true);
  page.api.redrawAlerts(false);
  assert.equal(page.alertsOn.hidden, true);
  assert.equal(page.alertsOff.hidden, false);

  const pending = page.api.saveAlerts({ alert_radius_km: 2 });
  await flush();
  assert.equal(page.requests[0].init.body, '{"alert_radius_km":2}');
  answer(page.requests[0], 200, ok(false, 2));
  assert.equal(await pending, true);
  assert.equal(page.select.value, '2');
});

test('every alert string reaches the page as text', async () => {
  const page = loadAlerts();
  // innerHTML and outerHTML throw in the fake element, so a markup sink would fail here.
  turnSwitch(page, true);
  await flush();
  answer(page.requests[0], 500, { error: '<b>bad</b>' });
  await flush();
  assert.equal(errorLine(page).textContent, SAVE_FAILED);
});
