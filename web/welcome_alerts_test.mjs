// welcome_alerts_test.mjs - runs the real web/static/js/welcome.js in a Node
// sandbox and checks step 5 of the first-time flow, Alerts (Phase 3.3, plan 13,
// 03.4 D-27, decision 15, UI-SPEC Surface 1):
//
//   - step 4's Continue saves the profile and Skip saves nothing, and both go to
//     step 5; first-run-done is sent only from step 5;
//   - step 5 reads PinalertNotify.baseState() at once and deviceState(key) after,
//     and shows one of four parts by device state: ready, needs-install, blocked,
//     unsupported (an empty key is unsupported);
//   - Allow alerts calls PinalertNotify.enable(key) inside the click, so the
//     browser prompt keeps its tap; Not now never asks the browser anything;
//   - a refusal, an offline answer and a failure each show their own line;
//   - every string reaches the page through textContent.
//
// Run it with: node --test web/welcome_alerts_test.mjs
// It carries its own copy of the small fake page of welcome_flow_test.mjs,
// because importing that file would run its tests too.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'welcome.js'), 'utf8');

const READY_TITLE = 'Get alerts when it matters';
const KEY = 'test-public-key-placeholder';

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
    this.className = '';
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
  matches(selector) {
    return selector.split(',').map((s) => s.trim()).some((part) => {
      if (part.startsWith('.')) { return this.className.split(/\s+/).includes(part.slice(1)); }
      if (part.startsWith('[tabindex]')) {
        return Object.hasOwn(this.attrs, 'tabindex') && this.attrs.tabindex !== '-1';
      }
      if (part.startsWith('[')) { return false; }
      return this.tagName === part.toUpperCase();
    });
  }
  querySelectorAll(selector) {
    const found = [];
    const walk = (node) => {
      node.children.forEach((c) => {
        if (c.matches(selector)) { found.push(c); }
        walk(c);
      });
    };
    walk(this);
    return found;
  }
  focus() { this.doc.activeElement = this; }
}

const STEP_NAMES = ['Welcome', 'Disclaimer and policy', 'Your location', 'About you', 'Alerts'];

// The markup of welcome.html.tmpl, as the ids and nesting welcome.js reads.
function buildPage({ start = 1, name = '', home = '', key = KEY, hasHome = false } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    addEventListener() {},
    removeEventListener() {}
  };
  const make = (tag, id, parent, cls) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (cls) { el.className = cls; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  doc.documentElement = new FakeEl(doc, 'html');
  doc.body = make('body');
  const root = make('div', 'welcome', doc.body);
  root.setAttribute('data-start-step', String(start));
  root.setAttribute('data-step', String(start));
  root.setAttribute('data-push-key', key);

  const side = make('aside', '', root, 'welcome__side');
  const list = make('ol', '', side, 'welcome__steps');
  STEP_NAMES.forEach((label, i) => {
    const li = make('li', '', list, 'welcome__steps-item');
    li.setAttribute('data-step', String(i + 1));
    const btn = make('button', 'welcome-steps-btn-' + (i + 1), li, 'welcome__steps-button');
    btn.setAttribute('data-step', String(i + 1));
  });

  const pane = make('div', '', root, 'welcome__pane');
  const steps = [1, 2, 3, 4, 5].map((n) => {
    const sec = make('section', 'welcome-step-' + n, pane, 'welcome__step');
    if (n !== start) { sec.hidden = true; }
    const title = make('h1', 'welcome-step-' + n + '-title', sec, 'welcome__title');
    title.setAttribute('tabindex', '-1');
    if (n === 5) { title.own = READY_TITLE; }
    return sec;
  });

  make('button', 'welcome-continue-1', steps[0]);
  [2, 3, 4, 5].forEach((n) => make('button', 'welcome-back-' + n, steps[n - 1], 'welcome__back'));

  make('div', 'welcome-scrollbox', steps[1]).setAttribute('tabindex', '0');
  make('button', 'welcome-disagree', steps[1]);
  make('button', 'welcome-agree', steps[1]);
  const disagreeBox = make('div', 'welcome-disagree-message', steps[1]);
  disagreeBox.setAttribute('tabindex', '-1');
  disagreeBox.hidden = true;
  make('button', 'welcome-reread', disagreeBox);
  make('button', 'welcome-logout', disagreeBox);
  make('p', 'welcome-error-2', steps[1]).hidden = true;

  make('button', 'welcome-allow-location', steps[2]);
  make('button', 'welcome-not-now', steps[2]);

  make('input', 'welcome-name-input', steps[3]).value = name;
  const form4 = make('form', 'welcome-home-form', steps[3]);
  make('input', 'welcome-home-input', form4).value = home;
  make('p', 'welcome-home-status', steps[3]).hidden = true;
  make('ul', 'welcome-home-results', steps[3]).hidden = true;
  make('p', 'welcome-error-4', steps[3]).hidden = true;
  make('button', 'welcome-continue-4', steps[3]);
  make('button', 'welcome-skip', steps[3]);

  const s5 = steps[4];
  s5.setAttribute('data-device-state', 'ready');
  make('div', 'welcome-alerts-ready', s5, 'welcome__alerts-part');
  const nohome = make('p', 'welcome-alerts-nohome', doc.byId['welcome-alerts-ready']);
  nohome.hidden = hasHome;
  make('div', 'welcome-alerts-install', s5, 'welcome__alerts-part').hidden = true;
  make('p', 'welcome-alerts-blocked', s5).hidden = true;
  make('p', 'welcome-alerts-unsupported', s5).hidden = true;
  // Amended 2026-10-25 (review T-04): the status line is an always-present live
  // region, never hidden, empty until the script writes the sentence into it.
  make('p', 'welcome-alerts-status', s5);
  make('p', 'welcome-alerts-error', s5).hidden = true;
  make('button', 'welcome-alerts-allow', s5);
  make('button', 'welcome-alerts-later', s5);
  make('button', 'welcome-alerts-continue', s5).hidden = true;

  return { doc, root, steps };
}

// A fake PinalertNotify. enable and deviceState answer when the test says so.
function makeNotify({ base = 'maybe', device = 'ready', now = false } = {}) {
  const calls = { enable: [], deviceState: [], baseState: 0 };
  const waiting = { enable: null, device: null };
  return {
    calls,
    waiting,
    baseState() { calls.baseState += 1; return base; },
    deviceState(key) {
      calls.deviceState.push(key);
      if (now) { return Promise.resolve(device); }
      return new Promise((resolve) => { waiting.device = resolve; });
    },
    enable(key) {
      calls.enable.push(key);
      return new Promise((resolve, reject) => { waiting.enable = { resolve, reject }; });
    }
  };
}

function load(options = {}) {
  const page = buildPage(options);
  const notify = options.notify === undefined ? makeNotify({ now: true }) : options.notify;
  const requests = [];
  const replaced = [];
  const session = {};
  const permissionCalls = [];
  const sandbox = {
    console,
    document: page.doc,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setTimeout() { return 1; },
    clearTimeout() {},
    AbortController: class { constructor() { this.signal = { aborted: false }; } abort() { this.signal.aborted = true; } },
    navigator: { geolocation: { getCurrentPosition() {} } },
    PinalertPosition: { markAsked() { return true; }, save() { return true; }, read() { return null; } },
    // The browser's own prompt: Not now must never reach it.
    Notification: { permission: 'default', requestPermission() { permissionCalls.push(true); return Promise.resolve('granted'); } },
    sessionStorage: {
      setItem(k, v) { session[k] = String(v); },
      getItem(k) { return Object.hasOwn(session, k) ? session[k] : null; }
    },
    location: { search: options.search || '', replace(target) { replaced.push(target); } },
    history: { pushState() {}, replaceState() {} },
    getComputedStyle() { return { display: 'block' }; },
    addEventListener() {}
  };
  if (notify) { sandbox.PinalertNotify = notify; }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'welcome.js' });
  return { ...page, api: sandbox.PinalertWelcome, notify, requests, replaced, session, permissionCalls, sandbox };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: () => (status === 204 ? Promise.reject(new Error('no body')) : Promise.resolve(body || {}))
});

const byId = (page, id) => page.doc.byId[id];
const fire = (el, type) => {
  const event = { type, target: el, key: '', shiftKey: false, preventDefault() {} };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};
const click = (page, id) => fire(byId(page, id), 'click');
const visibleStep = (page) => page.steps.findIndex((s) => !s.hidden) + 1;
const shown = (page, id) => !byId(page, id).hidden;
const text = (page, id) => byId(page, id).textContent;

// An agreed account at step 5, the usual place to test the step itself.
function atStep5(options = {}) {
  const page = load({ start: 3, ...options });
  page.api.goTo(5);
  return page;
}

test('step 4 Continue saves the profile, then goes to step 5 and sends no first-run-done', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-name-input').value = 'Asha';
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/me/profile');
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.equal(visibleStep(page), 4, 'no move before the profile is saved');
  answer(page.requests[0], 200);
  await flush();

  assert.equal(visibleStep(page), 5);
  assert.equal(page.requests.length, 1, 'first-run-done is not sent from step 4');
  assert.deepEqual(page.replaced, []);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-step-5-title'), 'focus moves to the step 5 title');
  assert.equal(page.root.getAttribute('data-step'), '5');
  assert.equal(byId(page, 'welcome-continue-4').disabled, false, 'step 4 buttons are free again when the person comes back');
  assert.equal(byId(page, 'welcome-skip').disabled, false);
});

test('step 4 Skip goes to step 5 with nothing saved and nothing sent', () => {
  const page = load({ start: 4 });
  click(page, 'welcome-skip');
  assert.equal(visibleStep(page), 5);
  assert.equal(page.requests.length, 0);
});

test('?step=5 opens step 5 for an account that has agreed, and Back goes to step 4', () => {
  const page = load({ start: 3, search: '?step=5' });
  assert.equal(visibleStep(page), 5);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-step-5-title'));
  click(page, 'welcome-back-5');
  assert.equal(visibleStep(page), 4);
  assert.equal(page.requests.length, 0, 'looking at a step records nothing');
});

test('ready: shows the ask with Allow alerts and Not now, reads the base state at once and the device state with the key', async () => {
  const notify = makeNotify({ base: 'maybe', device: 'on', now: false });
  const page = load({ start: 3, notify });
  page.api.goTo(5);
  assert.equal(notify.calls.baseState, 1, 'the base state is read on arrival, synchronously');
  assert.deepEqual(notify.calls.deviceState, [KEY]);
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'ready');
  assert.equal(shown(page, 'welcome-alerts-ready'), true);
  assert.equal(shown(page, 'welcome-alerts-install'), false);
  assert.equal(shown(page, 'welcome-alerts-allow'), true);
  assert.equal(shown(page, 'welcome-alerts-later'), true);
  assert.equal(shown(page, 'welcome-alerts-continue'), false);
  assert.equal(text(page, 'welcome-step-5-title'), READY_TITLE);

  // on also shows the ask, as ready does.
  notify.waiting.device('on');
  await flush();
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'ready');
  assert.equal(shown(page, 'welcome-alerts-ready'), true);
  assert.equal(shown(page, 'welcome-alerts-allow'), true);
});

test('needs-install: the guide, the title Install to get alerts, and one Continue with no Allow alerts', () => {
  const page = load({ start: 3, notify: makeNotify({ base: 'needs-install', now: true }) });
  page.api.goTo(5);
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'needs-install');
  assert.equal(shown(page, 'welcome-alerts-install'), true);
  assert.equal(shown(page, 'welcome-alerts-ready'), false);
  assert.equal(text(page, 'welcome-step-5-title'), 'Install to get alerts');
  assert.ok(page.doc.activeElement === byId(page, 'welcome-step-5-title'));
  assert.equal(text(page, 'welcome-step-5-title'), 'Install to get alerts', 'the title is right when the screen reader lands on it');
  assert.equal(shown(page, 'welcome-alerts-allow'), false);
  assert.equal(shown(page, 'welcome-alerts-later'), false);
  assert.equal(shown(page, 'welcome-alerts-continue'), true);
});

test('leaving the install variant gives the ready title back', () => {
  const page = load({ start: 3, notify: makeNotify({ base: 'needs-install', now: true }) });
  page.api.goTo(5);
  page.notify.baseState = () => 'maybe';
  click(page, 'welcome-back-5');
  page.api.goTo(5);
  assert.equal(text(page, 'welcome-step-5-title'), READY_TITLE);
  assert.equal(shown(page, 'welcome-alerts-install'), false);
});

test('blocked and unsupported each show their sentence and one Continue', () => {
  for (const state of ['blocked', 'unsupported']) {
    const page = load({ start: 3, notify: makeNotify({ base: state, now: true }) });
    page.api.goTo(5);
    assert.equal(page.steps[4].getAttribute('data-device-state'), state);
    assert.equal(shown(page, 'welcome-alerts-' + state), true);
    assert.equal(shown(page, 'welcome-alerts-ready'), false);
    assert.equal(text(page, 'welcome-step-5-title'), READY_TITLE);
    assert.equal(shown(page, 'welcome-alerts-allow'), false);
    assert.equal(shown(page, 'welcome-alerts-later'), false);
    assert.equal(shown(page, 'welcome-alerts-continue'), true);
  }
});

test('an empty key is an unsupported device and the server is not asked', () => {
  const notify = makeNotify({ base: 'maybe', now: true });
  const page = load({ start: 3, notify, key: '' });
  page.api.goTo(5);
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'unsupported');
  assert.equal(shown(page, 'welcome-alerts-unsupported'), true);
  assert.equal(notify.calls.deviceState.length, 0);
  assert.equal(shown(page, 'welcome-alerts-allow'), false);
});

test('a page with no PinalertNotify at all is an unsupported device', () => {
  const page = load({ start: 3, notify: null });
  page.api.goTo(5);
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'unsupported');
  assert.equal(shown(page, 'welcome-alerts-continue'), true);
});

test('a late device answer does not redraw after the person left step 5 or while busy', async () => {
  const notify = makeNotify({ base: 'maybe', device: 'blocked', now: false });
  const page = load({ start: 3, notify });
  page.api.goTo(5);
  click(page, 'welcome-back-5');
  notify.waiting.device('blocked');
  await flush();
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'ready', 'a stale answer is dropped');

  const busy = makeNotify({ base: 'maybe', now: false });
  const second = load({ start: 3, notify: busy });
  second.api.goTo(5);
  click(second, 'welcome-alerts-allow');
  busy.waiting.device('unsupported');
  await flush();
  assert.equal(second.steps[4].getAttribute('data-device-state'), 'ready', 'a busy step is not redrawn');
});

test('the home area line shows only when there is no home area', async () => {
  const stored = atStep5({ hasHome: true });
  assert.equal(shown(stored, 'welcome-alerts-nohome'), false, 'a stored home area hides the line');

  const none = atStep5({ hasHome: false });
  assert.equal(shown(none, 'welcome-alerts-nohome'), true, 'skipping step 4 with no stored area shows the line');

  const saved = load({ start: 4, hasHome: false });
  // A home area saved in step 4 hides the line on step 5.
  const input = byId(saved, 'welcome-home-input');
  input.value = 'Indiranagar';
  assert.ok(input);
  // Pick through the search: type, search, pick the first row.
  fire(byId(saved, 'welcome-home-form'), 'submit');
  answer(saved.requests[0], 200, { results: [{ name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru', lat: 12.9784, lon: 77.6408, area: 'Indiranagar' }] });
  await flush();
  const option = byId(saved, 'welcome-home-results').querySelectorAll('button')[0];
  fire(option, 'click');
  click(saved, 'welcome-continue-4');
  answer(saved.requests[1], 200);
  await flush();
  assert.equal(visibleStep(saved), 5);
  assert.equal(shown(saved, 'welcome-alerts-nohome'), false, 'a home area saved in step 4 hides the line');
});

test('Allow alerts calls enable(key) inside the click, disables both buttons and marks the step busy', () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  // Nothing was awaited: the call is already recorded when the click returns.
  assert.deepEqual(page.notify.calls.enable, [KEY]);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, true);
  assert.equal(byId(page, 'welcome-alerts-later').disabled, true);
  assert.equal(page.steps[4].getAttribute('aria-busy'), 'true');
  click(page, 'welcome-alerts-allow');
  assert.equal(page.notify.calls.enable.length, 1, 'a second tap while it runs does nothing');
});

test('Allow alerts, then ok true: first-run-done is sent and the person lands on Nearby with the search focus flag', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.resolve({ ok: true, state: 'on' });
  await flush();
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/me/first-run-done');
  assert.equal(page.requests[0].init.method, 'POST');
  assert.deepEqual(page.replaced, [], 'no move before first-run-done answers');
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
  assert.equal(page.session['pinalert.focusSearch'], '1');
});

test('a refusal shows Alerts are off on this device. and one Continue, then Continue ends the flow', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.resolve({ ok: false, state: 'blocked', reason: 'refused' });
  await flush();

  // Amended 2026-10-25 (review T-04): the status line is shown by its text, the
  // node itself is always present and never hidden.
  assert.equal(text(page, 'welcome-alerts-status'), 'Alerts are off on this device.');
  assert.equal(shown(page, 'welcome-alerts-allow'), false);
  assert.equal(shown(page, 'welcome-alerts-later'), false);
  assert.equal(shown(page, 'welcome-alerts-continue'), true);
  assert.equal(byId(page, 'welcome-alerts-continue').disabled, false);
  assert.equal(page.steps[4].getAttribute('aria-busy'), null);
  assert.equal(page.requests.length, 0, 'a refusal alone does not finish the flow');

  click(page, 'welcome-alerts-continue');
  assert.equal(page.requests[0].url, '/api/me/first-run-done');
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('an offline answer shows the offline line and brings both buttons back', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.resolve({ ok: false, state: 'ready', reason: 'offline' });
  await flush();
  assert.equal(text(page, 'welcome-alerts-error'), "You're offline, so this wasn't saved. Try again when you have a signal.");
  assert.equal(shown(page, 'welcome-alerts-error'), true);
  assert.equal(shown(page, 'welcome-alerts-allow'), true);
  assert.equal(shown(page, 'welcome-alerts-later'), true);
  assert.equal(shown(page, 'welcome-alerts-continue'), false);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
  assert.equal(byId(page, 'welcome-alerts-later').disabled, false);
  assert.equal(page.steps[4].getAttribute('aria-busy'), null);
  assert.equal(page.requests.length, 0);
});

test('a failed or ended session shows Couldn\'t turn alerts on. Try again. and both buttons come back', async () => {
  for (const reason of ['failed', 'session', 'something-new']) {
    const page = atStep5();
    click(page, 'welcome-alerts-allow');
    page.notify.waiting.enable.resolve({ ok: false, state: 'ready', reason });
    await flush();
    assert.equal(text(page, 'welcome-alerts-error'), "Couldn't turn alerts on. Try again.", reason);
    assert.equal(shown(page, 'welcome-alerts-allow'), true);
    assert.equal(shown(page, 'welcome-alerts-later'), true);
    assert.equal(page.steps[4].getAttribute('aria-busy'), null);
  }
});

test('a rejected enable is a failure too, and the server sentence of a 400 is shown when there is one', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.reject(new Error('boom'));
  await flush();
  assert.equal(text(page, 'welcome-alerts-error'), "Couldn't turn alerts on. Try again.");

  const sentence = atStep5();
  click(sentence, 'welcome-alerts-allow');
  sentence.notify.waiting.enable.resolve({ ok: false, state: 'ready', reason: 'failed', message: 'That notification service is not supported.' });
  await flush();
  assert.equal(text(sentence, 'welcome-alerts-error'), 'That notification service is not supported.');

  // A try again clears the old line.
  click(sentence, 'welcome-alerts-allow');
  assert.equal(shown(sentence, 'welcome-alerts-error'), false);
});

test('Allow alerts on a device that turns out unsupported redraws that variant', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.resolve({ ok: false, state: 'needs-install', reason: 'unsupported' });
  await flush();
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'needs-install');
  assert.equal(text(page, 'welcome-step-5-title'), 'Install to get alerts');
  assert.equal(shown(page, 'welcome-alerts-continue'), true);
  assert.equal(shown(page, 'welcome-alerts-allow'), false);
});

test('Not now sends first-run-done and leaves, and never calls enable or the browser prompt', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-later');
  assert.equal(page.notify.calls.enable.length, 0, 'enable is never called');
  assert.equal(page.permissionCalls.length, 0, 'Notification.requestPermission is never called');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/me/first-run-done');
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
  assert.equal(page.session['pinalert.focusSearch'], '1');
  assert.equal(page.notify.calls.enable.length, 0);
  assert.equal(page.permissionCalls.length, 0);
});

test('Continue in the blocked, unsupported and install variants sends first-run-done and leaves', async () => {
  for (const base of ['blocked', 'unsupported', 'needs-install']) {
    const page = load({ start: 3, notify: makeNotify({ base, now: true }) });
    page.api.goTo(5);
    click(page, 'welcome-alerts-continue');
    assert.equal(page.requests[0].url, '/api/me/first-run-done', base);
    answer(page.requests[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/'], base);
    assert.equal(page.permissionCalls.length, 0, base);
    assert.equal(page.notify.calls.enable.length, 0, base);
  }
});

test('a first-run-done failure keeps the person on step 5 with the save failed message and the buttons back', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-later');
  assert.equal(byId(page, 'welcome-alerts-later').disabled, true);
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(visibleStep(page), 5);
  assert.deepEqual(page.replaced, []);
  assert.equal(text(page, 'welcome-alerts-error'), 'Could not save. Try again in a moment.');
  assert.equal(shown(page, 'welcome-alerts-error'), true);
  assert.equal(byId(page, 'welcome-alerts-later').disabled, false);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
  assert.equal(page.steps[4].getAttribute('aria-busy'), null);
  assert.equal(shown(page, 'welcome-error-4'), false, 'the step 4 line is not used');
});

test('a 409 from first-run-done goes back to step 2 as before', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-later');
  answer(page.requests[0], 409, {});
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(text(page, 'welcome-error-2'), 'Please agree to the Disclaimer and policy to continue.');
  assert.deepEqual(page.replaced, []);
});

test('step 5 actions need the agreement, like every way to finish', () => {
  const page = load({ start: 1 });
  page.api.goTo(5);
  click(page, 'welcome-alerts-allow');
  assert.equal(page.notify.calls.enable.length, 0, 'the browser prompt is not run before the agreement');
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0);
  page.api.goTo(5);
  click(page, 'welcome-alerts-later');
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0);
});

// T-04 (review 2026-10-25): VoiceOver may miss a live region that is unhidden in
// the same moment its text is set, so the status line is always present and the
// script only ever writes its text. The spy counts every write to hidden.
test('the refusal line is an always-present live region: only its text is written, never hidden', async () => {
  const page = load({ start: 3 });
  const status = byId(page, 'welcome-alerts-status');
  let hiddenWrites = 0;
  Object.defineProperty(status, 'hidden', { get: () => false, set: () => { hiddenWrites += 1; } });
  page.api.goTo(5);
  assert.equal(text(page, 'welcome-alerts-status'), '', 'empty until a refusal');
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.resolve({ ok: false, state: 'blocked', reason: 'refused' });
  await flush();
  assert.equal(text(page, 'welcome-alerts-status'), 'Alerts are off on this device.');
  click(page, 'welcome-back-5');
  page.notify.baseState = () => 'blocked';
  page.api.goTo(5);
  assert.equal(text(page, 'welcome-alerts-status'), '');
  assert.equal(hiddenWrites, 0, 'the status line was hidden or unhidden; only its text may change');
});

test('coming back to step 5 after a refusal draws it fresh', async () => {
  const page = atStep5();
  click(page, 'welcome-alerts-allow');
  page.notify.waiting.enable.resolve({ ok: false, state: 'blocked', reason: 'refused' });
  await flush();
  click(page, 'welcome-back-5');
  page.notify.baseState = () => 'blocked';
  page.api.goTo(5);
  assert.equal(text(page, 'welcome-alerts-status'), '', 'the old status line is gone (T-04: emptied, not hidden)');
  assert.equal(page.steps[4].getAttribute('data-device-state'), 'blocked');
  assert.equal(shown(page, 'welcome-alerts-blocked'), true);
});
