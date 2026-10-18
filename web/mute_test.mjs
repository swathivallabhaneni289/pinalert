// mute_test.mjs - runs the real web/static/js/app.js and
// web/static/js/mute.js together in a Node sandbox and checks the Mute control
// of an opened report, which the Go source tests cannot (Phase 3.2, plan 11;
// D-11, UI-SPEC Surface C, P32-C15 and P32-C16):
//
//   - who sees the control: a reader who follows the report or muted it, or
//     who muted or unmuted it on this page; nobody else (Q3, owner confirmed
//     2026-10-18), and it shows on the reader's own report;
//   - the label and the accessible name for the muted and not muted states;
//   - a press posts to /mute or /unmute with a JSON body, flips the label,
//     keeps focus, speaks through #notice-live and asks the bell to refresh
//     once;
//   - a 404, a 500, a timeout, a dropped request and the offline case leave
//     the label alone and show the right line, and offline sends nothing;
//   - a poll that still carries the old server state does not flip the label
//     back, and the page's own memory is dropped once the server agrees;
//   - the click and the Enter or Space keydown stop propagation.
//
// Run it with: node --test web/mute_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the two scripts call.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');
const muteSource = fs.readFileSync(path.join(here, 'static', 'js', 'mute.js'), 'utf8');

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
    this.dataset = {};
    this.className = '';
    this.hidden = false;
    this.setTextCount = 0;
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
    this.setTextCount += 1;
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) { this.children.push(node); node.parentNode = this; return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.doc.activeElement = this; }
  fire(type, evt = {}) {
    const event = { type, target: this, stopped: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...evt };
    (this.handlers[type] || []).slice().forEach((fn) => fn(event));
    return event;
  }
}

const flush = async () => { for (let i = 0; i < 12; i += 1) { await Promise.resolve(); } };

// load evaluates app.js and then mute.js. fetches keeps every request with the
// hooks to answer it. online is the value of navigator.onLine.
function load({ online = true } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    handlers: {},
    events: [],
    all: [],
    createElement(tag) { const el = new FakeEl(doc, tag); doc.all.push(el); return el; },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector() { return null; },
    querySelectorAll(selector) {
      return selector === '.mute-control' ? doc.all.filter((el) => el.className.split(' ').includes('mute-control')) : [];
    },
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent(evt) { doc.events.push(evt); return true; }
  };
  doc.documentElement = new FakeEl(doc, 'html');
  const live = new FakeEl(doc, 'div', 'notice-live');
  doc.byId['notice-live'] = live;

  const timers = [];
  const fetches = [];
  const window = {
    setTimeout(fn, ms) { timers.push({ fn, ms, done: false }); return timers.length; },
    clearTimeout(id) { if (timers[id - 1]) { timers[id - 1].done = true; } },
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    location: { search: '' }
  };
  const sandbox = {
    window,
    document: doc,
    navigator: { onLine: online },
    fetch: (target, init) => new Promise((resolve, reject) => { fetches.push({ url: target, init, resolve, reject }); }),
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    CustomEvent: function CustomEvent(type) { this.type = type; },
    AbortController,
    URLSearchParams,
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent
  };
  vm.createContext(sandbox);
  vm.runInContext(appSource, sandbox);
  sandbox.Pinalert = window.Pinalert;
  vm.runInContext(muteSource, sandbox);
  return { Pinalert: window.Pinalert, Mute: window.PinalertMute, doc, live, timers, fetches, sandbox };
}

const minutesAgo = (n) => new Date(Date.now() - n * 60000).toISOString();
const report = (extra = {}) => ({ id: 42, category: 'flood', created_at: minutesAgo(12), viewer_follows: true, viewer_muted: false, ...extra });
const answer = (status) => ({ status, ok: status >= 200 && status < 300, text: () => Promise.resolve(''), json: () => Promise.resolve({}) });

// mounted builds one control for a report and runs the first update.
function mounted(m, r = report()) {
  const control = m.Mute.createMuteControl(r);
  m.Mute.updateMuteControl(control, r);
  return control;
}

test('shouldShow is true for a follower, a muted report and a report muted or unmuted on this page', async () => {
  const m = load();
  assert.equal(m.Mute.shouldShow(report({ viewer_follows: true, viewer_muted: false })), true);
  assert.equal(m.Mute.shouldShow(report({ viewer_follows: false, viewer_muted: true })), true);
  assert.equal(m.Mute.shouldShow(report({ viewer_follows: false, viewer_muted: false })), false);
  assert.equal(m.Mute.shouldShow({ id: 7, category: 'fire', created_at: minutesAgo(1) }), false);
  assert.equal(m.Mute.shouldShow(report({ viewer_follows: 'true' })), false);

  // A report the page muted stays visible even when a poll says the reader no
  // longer follows it, until the server agrees.
  const other = report({ id: 8, viewer_follows: true });
  const control = mounted(m, other);
  control.el.fire('click');
  m.fetches[0].resolve(answer(204));
  await flush();
  assert.equal(m.Mute.shouldShow({ ...other, viewer_follows: false, viewer_muted: false }), true);
});

test('the control is born hidden, then shown for a follower with the Mute this report words', () => {
  const m = load();
  const r = report();
  const control = m.Mute.createMuteControl(r);
  assert.equal(control.el.hidden, true);
  assert.equal(control.el.tagName, 'BUTTON');
  assert.equal(control.el.getAttribute('type'), 'button');
  assert.equal(control.el.className, 'mute-control');
  assert.equal(control.el.dataset.muteReportId, '42');
  assert.equal(control.error.hidden, true);
  assert.equal(control.error.className, 'inline-error');
  assert.equal(control.error.getAttribute('role'), 'alert');

  m.Mute.updateMuteControl(control, r);
  assert.equal(control.el.hidden, false);
  assert.equal(control.el.textContent, 'Mute this report');
  assert.equal(control.el.getAttribute('aria-label'), 'Mute this report, Flood, 12 min ago');
});

test('the control shows on the reader\'s own report and hides for a reader who neither follows nor muted', () => {
  const m = load();
  const own = mounted(m, report({ is_own_report: true, viewer_follows: true }));
  assert.equal(own.el.hidden, false);
  const stranger = mounted(m, report({ id: 9, viewer_follows: false, viewer_muted: false }));
  assert.equal(stranger.el.hidden, true);
});

test('a muted report reads Turn updates back on', () => {
  const m = load();
  const control = mounted(m, report({ viewer_muted: true }));
  assert.equal(control.el.hidden, false);
  assert.equal(control.el.textContent, 'Turn updates back on');
  assert.equal(control.el.getAttribute('aria-label'), 'Turn updates back on, Flood, 12 min ago');
});

test('an update writes the label only when it changed and never moves focus', () => {
  const m = load();
  const r = report();
  const control = mounted(m, r);
  const writes = control.el.setTextCount;
  m.doc.activeElement = null;
  m.Mute.updateMuteControl(control, r);
  m.Mute.updateMuteControl(control, r);
  assert.equal(control.el.setTextCount, writes);
  assert.equal(m.doc.activeElement, null);
});

test('a press on Mute posts to /mute and flips the label, keeps focus, announces and asks the bell to refresh', async () => {
  const m = load();
  const control = mounted(m);
  control.el.focus();
  control.el.fire('click');
  assert.equal(m.fetches.length, 1);
  const req = m.fetches[0];
  assert.equal(req.url, '/api/reports/42/mute');
  assert.equal(req.init.method, 'POST');
  assert.equal(req.init.credentials, 'same-origin');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.equal(req.init.body, '{}');
  req.resolve(answer(204));
  await flush();

  assert.equal(control.el.textContent, 'Turn updates back on');
  assert.equal(control.el.getAttribute('aria-label'), 'Turn updates back on, Flood, 12 min ago');
  assert.equal(m.doc.activeElement, control.el);
  assert.equal(control.error.hidden, true);
  assert.equal(m.doc.events.length, 1);
  assert.equal(m.doc.events[0].type, 'pinalert:updates-refresh');
  m.timers.filter((t) => !t.done).forEach((t) => t.fn());
  assert.equal(m.live.textContent, 'Updates are off for this report.');
});

test('a press on Turn updates back on posts to /unmute and mirrors the Mute press', async () => {
  const m = load();
  const control = mounted(m, report({ viewer_muted: true }));
  control.el.focus();
  control.el.fire('click');
  const req = m.fetches[0];
  assert.equal(req.url, '/api/reports/42/unmute');
  assert.equal(req.init.body, '{}');
  req.resolve(answer(204));
  await flush();

  assert.equal(control.el.textContent, 'Mute this report');
  assert.equal(control.el.getAttribute('aria-label'), 'Mute this report, Flood, 12 min ago');
  assert.equal(m.doc.activeElement, control.el);
  assert.equal(m.doc.events.length, 1);
  m.timers.filter((t) => !t.done).forEach((t) => t.fn());
  assert.equal(m.live.textContent, 'Updates are on again for this report.');
});

test('the report id in the address is encoded', () => {
  const m = load();
  const control = mounted(m, report({ id: 'a/b' }));
  control.el.fire('click');
  assert.equal(m.fetches[0].url, '/api/reports/a%2Fb/mute');
});

for (const status of [404, 500, 429]) {
  test(`a ${status} leaves the label and shows Couldn't change this. Try again.`, async () => {
    const m = load();
    const control = mounted(m);
    control.el.fire('click');
    m.fetches[0].resolve(answer(status));
    await flush();
    assert.equal(control.el.textContent, 'Mute this report');
    assert.equal(control.error.hidden, false);
    assert.equal(control.error.textContent, "Couldn't change this. Try again.");
    assert.equal(m.doc.events.length, 0);
    m.timers.filter((t) => !t.done).forEach((t) => t.fn());
    assert.equal(m.live.textContent, '');
  });
}

test('a request that gets no answer in time leaves the label and shows the same line', async () => {
  const m = load();
  const control = mounted(m);
  control.el.fire('click');
  assert.equal(m.timers.length >= 1, true);
  assert.equal(m.timers[0].ms, 10000);
  m.timers[0].fn();
  await flush();
  assert.equal(control.el.textContent, 'Mute this report');
  assert.equal(control.error.hidden, false);
  assert.equal(control.error.textContent, "Couldn't change this. Try again.");
  assert.equal(m.doc.events.length, 0);
});

test('a dropped request leaves the label and shows the same line', async () => {
  const m = load();
  const control = mounted(m);
  control.el.fire('click');
  m.fetches[0].reject(new TypeError('network'));
  await flush();
  assert.equal(control.el.textContent, 'Mute this report');
  assert.equal(control.error.textContent, "Couldn't change this. Try again.");
});

test('offline sends nothing and shows the offline line', () => {
  const m = load({ online: false });
  const control = mounted(m);
  control.el.fire('click');
  assert.equal(m.fetches.length, 0);
  assert.equal(control.el.textContent, 'Mute this report');
  assert.equal(control.error.hidden, false);
  assert.equal(control.error.textContent, "You're offline, so this wasn't saved. Try again when you have a signal.");
});

test('the next press clears the old error, and a success hides it', async () => {
  const m = load();
  const control = mounted(m);
  control.el.fire('click');
  m.fetches[0].resolve(answer(500));
  await flush();
  assert.equal(control.error.hidden, false);
  control.el.fire('click');
  assert.equal(control.error.hidden, true);
  m.fetches[1].resolve(answer(204));
  await flush();
  assert.equal(control.error.hidden, true);
  assert.equal(control.el.textContent, 'Turn updates back on');
});

test('a second press while the first is still running sends nothing more', () => {
  const m = load();
  const control = mounted(m);
  control.el.fire('click');
  control.el.fire('click');
  assert.equal(m.fetches.length, 1);
});

test('a poll with the old server state does not flip the label back, and the server agreeing hands the label back', async () => {
  const m = load();
  const r = report();
  const control = mounted(m, r);
  control.el.fire('click');
  m.fetches[0].resolve(answer(204));
  await flush();
  assert.equal(control.el.textContent, 'Turn updates back on');

  // The poll left before the mute landed: it still says not muted.
  m.Mute.updateMuteControl(control, report({ viewer_muted: false }));
  assert.equal(control.el.textContent, 'Turn updates back on');
  assert.equal(control.el.hidden, false);

  // The server now agrees; the page's memory is dropped...
  m.Mute.updateMuteControl(control, report({ viewer_muted: true }));
  assert.equal(control.el.textContent, 'Turn updates back on');
  // ...so a later change made elsewhere (an Unmute in Settings) shows.
  m.Mute.updateMuteControl(control, report({ viewer_muted: false }));
  assert.equal(control.el.textContent, 'Mute this report');
});

test('an unmute on this page also wins over a stale poll', async () => {
  const m = load();
  const control = mounted(m, report({ viewer_muted: true }));
  control.el.fire('click');
  m.fetches[0].resolve(answer(204));
  await flush();
  m.Mute.updateMuteControl(control, report({ viewer_muted: true }));
  assert.equal(control.el.textContent, 'Mute this report');
  m.Mute.updateMuteControl(control, report({ viewer_muted: false, viewer_follows: false }));
  assert.equal(control.el.textContent, 'Mute this report');
  assert.equal(control.el.hidden, true);
});

test('every control for the report on the page flips, and no other report\'s does', async () => {
  const m = load();
  const a = mounted(m, report({ id: 42 }));
  const b = mounted(m, report({ id: 42 }));
  const c = mounted(m, report({ id: 43 }));
  a.el.fire('click');
  m.fetches[0].resolve(answer(204));
  await flush();
  assert.equal(a.el.textContent, 'Turn updates back on');
  assert.equal(b.el.textContent, 'Turn updates back on');
  assert.equal(b.el.getAttribute('aria-label'), 'Turn updates back on, Flood, 12 min ago');
  assert.equal(c.el.textContent, 'Mute this report');
});

test('the click and the Enter, Space and Spacebar keydown stop propagation', () => {
  const m = load();
  const control = mounted(m);
  assert.equal(control.el.fire('click').stopped, true);
  for (const key of ['Enter', ' ', 'Spacebar']) {
    assert.equal(control.el.fire('keydown', { key }).stopped, true, key);
  }
  assert.equal(control.el.fire('keydown', { key: 'a' }).stopped, false);
});

test('the refresh event is skipped, not thrown, where CustomEvent does not exist', async () => {
  const m = load();
  const control = mounted(m);
  m.sandbox.CustomEvent = undefined;
  control.el.fire('click');
  m.fetches[0].resolve(answer(204));
  await flush();
  assert.equal(control.el.textContent, 'Turn updates back on');
  assert.equal(m.doc.events.length, 0);
});
