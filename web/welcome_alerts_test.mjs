// welcome_alerts_test.mjs - runs the real web/static/js/welcome.js in a Node
// sandbox and checks the alerts pop-up that opens after step 4 of the
// first-time flow (quick 261027-t5w, owner, 2026-10-27). It replaced step 5,
// Alerts, which was a whole page (Phase 3.3, plan 13); the flow has four steps.
//
//   - step 4 finishing (Continue with a saved profile, Continue with nothing to
//     save, and Skip) opens a small native modal dialog when notify.js baseState
//     is maybe or ready and the page has a push key; the dialog title takes
//     focus so no button looks chosen;
//   - Allow calls PinalertNotify.enable(key) inside the click, before anything
//     is awaited, so the browser's own permission popup keeps the tap; Not now
//     and Escape never reach enable or Notification.requestPermission;
//   - whatever the answer (Allow with any enable outcome, a rejection or a
//     throw, Not now, Escape), POST /api/me/first-run-done is sent exactly once
//     and a 204 lands the person on Nearby;
//   - an iPhone Safari tab (needs-install), blocked or unsupported devices, no
//     push key, no PinalertNotify and a dialog without showModal get no pop-up:
//     step 4 finishing sends first-run-done directly;
//   - a 409 goes back to step 2; any other failure closes the pop-up, shows the
//     step 4 save line, and a second Continue finishes without asking again;
//   - the flow counts four steps;
//   - (plan 03.3-19, the 03.3-UAT gap for test 4, owner 2026-10-27) the pop-up now
//     names the home area in its title, asks for the home area first when none is
//     stored or saved at step 4 (the step 4 place search, a picked place only,
//     Allow disabled until one is picked), and offers an unticked Reliable
//     reporter tick box. Allow calls enable first, then saves the picked home
//     area, then the Reliable switch if ticked, then first-run-done, and every
//     failure still reaches first-run-done. Not now and Escape save nothing. A
//     Mac Dock app never gets the pop-up. The cases that cover this start with
//     "named pop-up: ".
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

const KEY = 'test-public-key-placeholder';
const AGREE_FIRST = 'Please agree to the Disclaimer and policy to continue.';
const SAVE_FAILED = 'Could not save. Try again in a moment.';

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

const STEP_NAMES = ['Welcome', 'Disclaimer and policy', 'Your location', 'About you'];

// The markup of welcome.html.tmpl, as the ids and nesting welcome.js reads. The
// pop-up is a native dialog shaped like the one in logout_confirm_test.mjs, and
// it sits outside the flow root, as in the template. Its close() does not fire a
// close event, so nothing here depends on when that event would arrive.
// askHome is the home area the server stored for the account (data-home-area on
// the dialog); it defaults to a stored area so the earlier cases, which are about
// the answers and not the home area, find Allow enabled.
function buildPage({ start = 1, name = '', home = '', key = KEY, dialogSupport = true, askHome = 'Benz Circle' } = {}) {
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
  const steps = [1, 2, 3, 4].map((n) => {
    const sec = make('section', 'welcome-step-' + n, pane, 'welcome__step');
    if (n !== start) { sec.hidden = true; }
    const title = make('h1', 'welcome-step-' + n + '-title', sec, 'welcome__title');
    title.setAttribute('tabindex', '-1');
    return sec;
  });

  make('button', 'welcome-continue-1', steps[0]);
  [2, 3, 4].forEach((n) => make('button', 'welcome-back-' + n, steps[n - 1], 'welcome__back'));

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

  const dialog = make('dialog', 'welcome-alerts-ask', doc.body);
  dialog.open = false;
  dialog.showCalls = 0;
  dialog.closeCalls = 0;
  dialog.throwOnShow = false;
  if (dialogSupport) {
    dialog.showModal = function () {
      this.showCalls += 1;
      if (this.throwOnShow) { throw new Error('InvalidStateError'); }
      this.open = true;
    };
    dialog.close = function () { this.closeCalls += 1; this.open = false; };
  }
  dialog.setAttribute('data-home-area', askHome);
  const askTitle = make('h2', 'welcome-alerts-ask-title', dialog, 'logout-confirm__title');
  askTitle.setAttribute('tabindex', '-1');
  askTitle.textContent = 'Get alerts for critical reports near ' + (askHome || 'your home area') + '?';
  // The home area search block, hidden when an area is stored, and the Reliable
  // tick box (unticked).
  const askHomeBlock = make('div', 'welcome-alerts-home', dialog, 'welcome-ask__home');
  if (askHome) { askHomeBlock.hidden = true; }
  const askForm = make('form', 'welcome-alerts-home-form', askHomeBlock);
  make('input', 'welcome-alerts-home-input', askForm);
  make('p', 'welcome-alerts-home-status', askHomeBlock).hidden = true;
  make('ul', 'welcome-alerts-home-results', askHomeBlock).hidden = true;
  make('input', 'welcome-alerts-reliable', dialog).checked = false;
  make('button', 'welcome-alerts-later', dialog);
  make('button', 'welcome-alerts-allow', dialog);

  return { doc, root, steps, dialog };
}

// A fake PinalertNotify. enable answers as the mode says: 'pending' waits for
// the test, 'ok', 'refused', 'offline', 'failed' resolve at once, 'reject'
// returns a rejected promise, 'throw' throws before returning anything.
// macDock is the answer of isMacDockApp (plan 17); left out, the function does
// not exist, like an older notify.js.
function makeNotify({ base = 'maybe', mode = 'ok', macDock } = {}) {
  const calls = { enable: [], deviceState: [], baseState: 0 };
  const waiting = { enable: null };
  const extra = macDock === undefined ? {} : { isMacDockApp() { return macDock; } };
  return {
    ...extra,
    calls,
    waiting,
    baseState() { calls.baseState += 1; return base; },
    deviceState(key) { calls.deviceState.push(key); return Promise.resolve('ready'); },
    enable(key) {
      calls.enable.push(key);
      if (mode === 'throw') { throw new Error('enable blew up'); }
      if (mode === 'reject') { return Promise.reject(new Error('enable rejected')); }
      if (mode === 'pending') {
        return new Promise((resolve, reject) => { waiting.enable = { resolve, reject }; });
      }
      if (mode === 'ok') { return Promise.resolve({ ok: true, state: 'on' }); }
      return Promise.resolve({ ok: false, state: 'ready', reason: mode });
    }
  };
}

function load(options = {}) {
  const page = buildPage(options);
  const notify = options.notify === undefined ? makeNotify() : options.notify;
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
    // The browser's own prompt: only the Allow click, through enable, may reach it.
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
  const event = { type, target: el, key: '', shiftKey: false, prevented: false, preventDefault() { this.prevented = true; } };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};
const click = (page, id) => fire(byId(page, id), 'click');
const visibleStep = (page) => page.steps.findIndex((s) => !s.hidden) + 1;
const doneRequests = (page) => page.requests.filter((r) => r.url === '/api/me/first-run-done');

// An agreed account at step 4 with nothing filled in, so Continue saves nothing.
const atStep4 = (options = {}) => load({ start: 4, ...options });

// Step 4 Continue with a changed name: the profile PATCH is answered 200, which
// leads to the pop-up.
async function continueWithProfile(page) {
  byId(page, 'welcome-name-input').value = 'Asha';
  click(page, 'welcome-continue-4');
  assert.equal(page.requests[0].url, '/api/me/profile');
  answer(page.requests[0], 200);
  await flush();
}

function assertAskOpen(page) {
  assert.equal(page.dialog.showCalls, 1, 'showModal is called once');
  assert.equal(page.dialog.open, true);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-alerts-ask-title'), 'the title takes focus so no button looks chosen');
  assert.equal(doneRequests(page).length, 0, 'first-run-done waits for the answer');
}

for (const base of ['maybe', 'ready']) {
  test('base state ' + base + ': Continue after a saved profile opens the pop-up and sends no first-run-done', async () => {
    const page = atStep4({ notify: makeNotify({ base }) });
    await continueWithProfile(page);
    assertAskOpen(page);
    assert.equal(page.requests.length, 1, 'only the profile request so far');
    assert.equal(visibleStep(page), 4, 'the flow stays on step 4 behind the pop-up');
    assert.equal(page.notify.calls.enable.length, 0);
    assert.equal(page.notify.calls.deviceState.length, 0, 'the decision reads baseState only, no device request');
    assert.equal(page.permissionCalls.length, 0);
  });

  test('base state ' + base + ': Continue with nothing to save and Skip each open the pop-up', () => {
    const cont = atStep4({ notify: makeNotify({ base }) });
    click(cont, 'welcome-continue-4');
    assertAskOpen(cont);
    assert.equal(cont.requests.length, 0);

    const skipped = atStep4({ notify: makeNotify({ base }) });
    byId(skipped, 'welcome-name-input').value = 'Typed but skipped';
    click(skipped, 'welcome-skip');
    assertAskOpen(skipped);
    assert.equal(skipped.requests.length, 0, 'Skip saves nothing');
  });
}

test('Allow calls enable in the same tick, before anything is awaited, and the pop-up shows it is busy', async () => {
  const notify = makeNotify({ mode: 'pending' });
  const page = atStep4({ notify });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-allow');
  // No await since the click returned.
  assert.deepEqual(notify.calls.enable, [KEY]);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, true);
  assert.equal(byId(page, 'welcome-alerts-later').disabled, true);
  assert.equal(page.dialog.getAttribute('aria-busy'), 'true');
  assert.equal(page.requests.length, 0, 'no first-run-done while the browser prompt is open');
  await flush();
  assert.equal(page.requests.length, 0, 'still waiting for the browser');
  assert.equal(page.dialog.open, true, 'the pop-up stays open while the answer is handled');

  notify.waiting.enable.resolve({ ok: true, state: 'on' });
  await flush();
  assert.equal(doneRequests(page).length, 1);
  assert.equal(page.permissionCalls.length, 0, 'welcome.js never asks the browser for permission itself');
});

const ENABLE_MODES = ['ok', 'refused', 'offline', 'failed', 'reject', 'throw'];
for (const mode of ENABLE_MODES) {
  test('Allow with enable ' + mode + ' still sends first-run-done once and leaves on a 204', async () => {
    const notify = makeNotify({ mode });
    const page = atStep4({ notify });
    click(page, 'welcome-continue-4');
    click(page, 'welcome-alerts-allow');
    await flush();

    assert.equal(notify.calls.enable.length, 1);
    assert.equal(doneRequests(page).length, 1);
    assert.equal(page.requests.length, 1);
    assert.equal(page.requests[0].init.method, 'POST');
    assert.deepEqual(page.replaced, [], 'no move before first-run-done answers');
    answer(page.requests[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
    assert.equal(page.session['pinalert.focusSearch'], '1');
  });
}

test('Not now sends first-run-done once, never calls enable or requestPermission, and leaves on a 204', async () => {
  const notify = makeNotify();
  const page = atStep4({ notify });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-later');
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, true);
  assert.equal(byId(page, 'welcome-alerts-later').disabled, true);
  assert.equal(page.dialog.getAttribute('aria-busy'), 'true');
  assert.equal(doneRequests(page).length, 1);
  answer(page.requests[0], 204);
  await flush();

  assert.deepEqual(page.replaced, ['/']);
  assert.equal(page.session['pinalert.focusSearch'], '1');
  assert.equal(notify.calls.enable.length, 0);
  assert.equal(page.permissionCalls.length, 0);
});

test('Escape is Not now: the cancel event is prevented, one first-run-done goes out, enable is never called', async () => {
  const notify = makeNotify();
  const page = atStep4({ notify });
  click(page, 'welcome-continue-4');
  const event = fire(page.dialog, 'cancel');
  assert.equal(event.prevented, true, 'the browser does not close the pop-up by itself');
  assert.equal(doneRequests(page).length, 1);
  assert.equal(notify.calls.enable.length, 0);
  assert.equal(page.permissionCalls.length, 0);
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('Escape while the pop-up is busy sends nothing more and is still prevented', async () => {
  const notify = makeNotify({ mode: 'pending' });
  const page = atStep4({ notify });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-allow');
  const event = fire(page.dialog, 'cancel');
  assert.equal(event.prevented, true);
  notify.waiting.enable.resolve({ ok: false, reason: 'refused' });
  await flush();
  assert.equal(page.requests.length, 1, 'one first-run-done, from the Allow answer');
  assert.equal(notify.calls.enable.length, 1);
});

const NO_POPUP = [
  ['needs-install (an iPhone Safari tab)', { notify: makeNotify({ base: 'needs-install' }) }],
  ['blocked', { notify: makeNotify({ base: 'blocked' }) }],
  ['unsupported', { notify: makeNotify({ base: 'unsupported' }) }],
  ['an unknown base state', { notify: makeNotify({ base: 'something-new' }) }],
  ['an empty push key', { key: '' }],
  ['no PinalertNotify at all', { notify: null }],
  ['a dialog without showModal', { dialogSupport: false }]
];
for (const [label, options] of NO_POPUP) {
  test('no pop-up for ' + label + ': step 4 finishing sends first-run-done directly', async () => {
    const page = atStep4(options);
    click(page, 'welcome-continue-4');
    assert.equal(page.dialog.showCalls, 0, 'showModal is never called');
    assert.equal(page.dialog.open, false);
    assert.equal(doneRequests(page).length, 1, 'first-run-done goes straight out');
    assert.equal(page.requests[0].init.method, 'POST');
    if (page.notify) {
      assert.equal(page.notify.calls.enable.length, 0);
    }
    assert.equal(page.permissionCalls.length, 0);
    answer(page.requests[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
    assert.equal(page.session['pinalert.focusSearch'], '1');

    const skipped = load({ start: 4, ...options });
    click(skipped, 'welcome-skip');
    assert.equal(skipped.dialog.showCalls, 0);
    assert.equal(doneRequests(skipped).length, 1);
  });
}

test('a showModal that throws sends first-run-done directly', async () => {
  const page = atStep4();
  page.dialog.throwOnShow = true;
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.open, false);
  assert.equal(doneRequests(page).length, 1);
  assert.equal(page.notify.calls.enable.length, 0);
});

test('exactly one first-run-done: a double Allow, Allow then Escape, and Not now then Allow', async () => {
  const doubled = atStep4({ notify: makeNotify({ mode: 'pending' }) });
  click(doubled, 'welcome-continue-4');
  click(doubled, 'welcome-alerts-allow');
  click(doubled, 'welcome-alerts-allow');
  doubled.notify.waiting.enable.resolve({ ok: true });
  await flush();
  assert.equal(doubled.notify.calls.enable.length, 1);
  assert.equal(doneRequests(doubled).length, 1);

  const escaped = atStep4();
  click(escaped, 'welcome-continue-4');
  click(escaped, 'welcome-alerts-allow');
  fire(escaped.dialog, 'cancel');
  await flush();
  assert.equal(doneRequests(escaped).length, 1);

  const laterThenAllow = atStep4();
  click(laterThenAllow, 'welcome-continue-4');
  click(laterThenAllow, 'welcome-alerts-later');
  click(laterThenAllow, 'welcome-alerts-allow');
  await flush();
  assert.equal(laterThenAllow.notify.calls.enable.length, 0);
  assert.equal(doneRequests(laterThenAllow).length, 1);
});

test('a 409 closes the pop-up and sends the person back to step 2 with the agree line', async () => {
  const page = atStep4();
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-later');
  answer(page.requests[0], 409, { error: { field: 'terms', message: 'Agree first.' } });
  await flush();
  assert.equal(page.dialog.open, false, 'the pop-up is closed');
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-error-2').textContent, AGREE_FIRST);
  assert.deepEqual(page.replaced, []);
  assert.equal(page.session['pinalert.focusSearch'], undefined);
});

for (const [label, fail, opener] of [
  ['a 500 after Continue', (req) => answer(req, 500), 'welcome-continue-4'],
  ['a network failure after Skip', (req) => req.reject(new Error('offline')), 'welcome-skip']
]) {
  test(label + ' closes the pop-up, shows the step 4 line, and a second Continue finishes without asking again', async () => {
    const page = atStep4();
    click(page, opener);
    assertAskOpen(page);
    click(page, 'welcome-alerts-allow');
    await flush();
    assert.equal(doneRequests(page).length, 1);
    fail(page.requests[0]);
    await flush();

    assert.equal(page.dialog.open, false, 'the pop-up is closed');
    assert.equal(page.dialog.getAttribute('aria-busy'), null);
    assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
    assert.equal(byId(page, 'welcome-alerts-later').disabled, false);
    assert.equal(visibleStep(page), 4);
    assert.equal(byId(page, 'welcome-error-4').hidden, false);
    assert.equal(byId(page, 'welcome-error-4').textContent, SAVE_FAILED);
    assert.equal(byId(page, 'welcome-continue-4').disabled, false);
    assert.equal(byId(page, 'welcome-skip').disabled, false);
    assert.ok(page.doc.activeElement === byId(page, opener), 'focus goes back to the button that was pressed');
    assert.deepEqual(page.replaced, []);

    click(page, 'welcome-continue-4');
    assert.equal(page.dialog.showCalls, 1, 'the pop-up is not shown again');
    assert.equal(page.notify.calls.enable.length, 1, 'enable is not called again');
    assert.equal(doneRequests(page).length, 2, 'a second first-run-done goes out directly');
    answer(page.requests[1], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
  });
}

test('an account that has not agreed goes to step 2 and never sees the pop-up', () => {
  const page = load({ start: 1 });
  page.api.goTo(4);
  click(page, 'welcome-continue-4');
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-error-2').textContent, AGREE_FIRST);
  assert.equal(page.dialog.showCalls, 0);
  assert.equal(page.requests.length, 0);

  page.api.goTo(4);
  click(page, 'welcome-skip');
  assert.equal(page.dialog.showCalls, 0);
  assert.equal(page.requests.length, 0);
});

test('changing step while the pop-up is open closes it without answering, and the next Continue asks again', () => {
  const page = atStep4();
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.open, true);
  page.api.goTo(3);
  assert.equal(page.dialog.open, false);
  assert.equal(page.requests.length, 0, 'nothing was answered, so nothing is sent');
  assert.equal(byId(page, 'welcome-continue-4').disabled, false);
  assert.equal(byId(page, 'welcome-skip').disabled, false);

  page.api.goTo(4);
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.showCalls, 2, 'the person is asked again');
  assert.equal(page.dialog.open, true);
});

test('the pop-up lives outside the flow root, so its keys never reach the step Tab trap', () => {
  const page = atStep4();
  assert.equal(page.root.contains(page.dialog), false);
  assert.equal(page.doc.body.contains(page.dialog), true);
});

test('the flow has four steps: ?step=5 falls back to the start step and Back exists on steps 2 to 4 only', () => {
  const page = load({ start: 3, search: '?step=5' });
  assert.equal(visibleStep(page), 3);
  assert.equal(page.steps.length, 4);
  assert.equal(byId(page, 'welcome-step-5'), undefined);
  assert.equal(byId(page, 'welcome-back-1'), undefined);
  for (const n of [2, 3, 4]) {
    assert.ok(byId(page, 'welcome-back-' + n), 'Back on step ' + n);
  }
  assert.equal(byId(page, 'welcome-back-5'), undefined);
  page.api.goTo(5);
  assert.equal(visibleStep(page), 3, 'a step that does not exist is refused');

  const agreed = load({ start: 4, search: '?step=5' });
  assert.equal(visibleStep(agreed), 4);
  click(agreed, 'welcome-back-4');
  assert.equal(visibleStep(agreed), 3);
  assert.equal(agreed.requests.length, 0, 'looking at a step records nothing');
});

// ---- Plan 03.3-19: the pop-up names the home area, asks for it when it is
// missing, and offers the Reliable reporter tick box (03.3-UAT gap for test 4) ----

const AREA_URL = '/api/geocode/place?kind=area&q=';
const ALERTS_ADDRESS = '/api/me/alerts';
const PROFILE_ADDRESS = '/api/me/profile';
const DONE_ADDRESS = '/api/me/first-run-done';
const titleFor = (area) => 'Get alerts for critical reports near ' + area + '?';
const TITLE_NO_HOME = titleFor('your home area');
const HEBBAL = { name: 'Hebbal', display_name: 'Hebbal, Bengaluru, Karnataka', lat: 13.0358, lon: 77.597 };

const askTitleText = (page) => byId(page, 'welcome-alerts-ask-title').textContent;
const askHomeBlock = (page) => byId(page, 'welcome-alerts-home');
const reliableBox = (page) => byId(page, 'welcome-alerts-reliable');
const bodyOf = (req) => JSON.parse(req.init.body);
const sent = (page) => page.requests.map((r) => (r.init.method || 'GET') + ' ' + r.url);

// Search in the pop-up the way Enter does, answer it with one place and pick it.
async function pickInAsk(page, text = 'Hebbal', place = HEBBAL) {
  byId(page, 'welcome-alerts-home-input').value = text;
  fire(byId(page, 'welcome-alerts-home-form'), 'submit');
  const req = page.requests[page.requests.length - 1];
  assert.equal(req.url, AREA_URL + encodeURIComponent(text), 'the pop-up search sends typed text only, in areas mode');
  answer(req, 200, { results: [place] });
  await flush();
  const option = byId(page, 'welcome-alerts-home-results').children[0].children[0];
  fire(option, 'click');
}

// The same for step 4's own search.
async function pickAtStep4(page, text = 'Hebbal', place = HEBBAL) {
  byId(page, 'welcome-home-input').value = text;
  fire(byId(page, 'welcome-home-form'), 'submit');
  answer(page.requests[page.requests.length - 1], 200, { results: [place] });
  await flush();
  fire(byId(page, 'welcome-home-results').children[0].children[0], 'click');
}

test('named pop-up: a stored home area is named in the title, the search is hidden, Allow is on, the box is unticked', () => {
  const page = atStep4({ askHome: 'Benz Circle' });
  click(page, 'welcome-continue-4');
  assertAskOpen(page);
  assert.equal(askTitleText(page), titleFor('Benz Circle'));
  assert.equal(askHomeBlock(page).hidden, true);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
  assert.equal(reliableBox(page).checked, false);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-alerts-ask-title'), 'the title has focus');
});

test('named pop-up: a home area saved at step 4 in this visit names the title and hides the search', async () => {
  const page = atStep4({ askHome: '' });
  await pickAtStep4(page);
  click(page, 'welcome-continue-4');
  const profile = page.requests[page.requests.length - 1];
  assert.equal(profile.url, PROFILE_ADDRESS);
  assert.deepEqual(bodyOf(profile).home_area, { name: 'Hebbal', lat: 13.0358, lon: 77.597 });
  answer(profile, 200);
  await flush();
  assert.equal(page.dialog.open, true);
  assert.equal(askTitleText(page), titleFor('Hebbal'));
  assert.equal(askHomeBlock(page).hidden, true);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
});

test('named pop-up: with no home area the pop-up asks for one first and Allow waits for a pick', async () => {
  const page = atStep4({ askHome: '' });
  click(page, 'welcome-continue-4');
  assertAskOpen(page);
  assert.equal(askTitleText(page), TITLE_NO_HOME);
  assert.equal(askHomeBlock(page).hidden, false);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, true);
  assert.equal(byId(page, 'welcome-alerts-later').disabled, false, 'Not now stays available');

  // Allow does nothing while it is off, even if its handler is reached.
  click(page, 'welcome-alerts-allow');
  assert.equal(page.notify.calls.enable.length, 0);

  await pickInAsk(page);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
  assert.equal(askTitleText(page), titleFor('Hebbal'));

  // Typing after the pick forgets it: Allow is off again and the title goes back.
  const input = byId(page, 'welcome-alerts-home-input');
  input.value = 'Hebba';
  fire(input, 'input');
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, true);
  assert.equal(askTitleText(page), TITLE_NO_HOME);
  assert.equal(page.notify.calls.enable.length, 0);
});

test('named pop-up: Allow with the box ticked calls enable in the tick, then sends one Reliable request, then first-run-done', async () => {
  const notify = makeNotify({ mode: 'pending' });
  const page = atStep4({ notify });
  click(page, 'welcome-continue-4');
  reliableBox(page).checked = true;
  click(page, 'welcome-alerts-allow');
  assert.deepEqual(notify.calls.enable, [KEY], 'enable is called in the same tick, with the page key');
  assert.equal(page.requests.length, 0, 'no request before enable settles');
  assert.equal(reliableBox(page).disabled, true);
  assert.equal(page.dialog.getAttribute('aria-busy'), 'true');

  notify.waiting.enable.resolve({ ok: true, state: 'on' });
  await flush();
  assert.deepEqual(sent(page), ['PATCH ' + ALERTS_ADDRESS]);
  assert.deepEqual(bodyOf(page.requests[0]), { reliable_alerts: true });
  assert.equal(page.requests[0].init.headers['Content-Type'], 'application/json');
  answer(page.requests[0], 200);
  await flush();
  assert.deepEqual(sent(page), ['PATCH ' + ALERTS_ADDRESS, 'POST ' + DONE_ADDRESS]);
  answer(page.requests[1], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('named pop-up: Allow with the box unticked sends no alerts request and one first-run-done', async () => {
  const page = atStep4();
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-allow');
  await flush();
  assert.deepEqual(sent(page), ['POST ' + DONE_ADDRESS]);
  assert.equal(page.notify.calls.enable.length, 1);
  assert.equal(page.requests.filter((r) => r.url === ALERTS_ADDRESS).length, 0);
});

test('named pop-up: Allow after a pick saves the home area, then Reliable, then first-run-done, in that order after enable', async () => {
  const page = atStep4({ askHome: '' });
  click(page, 'welcome-continue-4');
  await pickInAsk(page);
  reliableBox(page).checked = true;
  const before = page.requests.length;
  click(page, 'welcome-alerts-allow');
  assert.deepEqual(page.notify.calls.enable, [KEY]);
  assert.equal(page.requests.length, before, 'nothing is sent in the tick');
  await flush();
  const profile = page.requests[before];
  assert.equal(profile.url, PROFILE_ADDRESS);
  assert.equal(profile.init.method, 'PATCH');
  assert.deepEqual(bodyOf(profile), { home_area: { name: 'Hebbal', lat: 13.0358, lon: 77.597 } });
  assert.equal(page.requests.length, before + 1, 'one request at a time');
  answer(profile, 200, {});
  await flush();
  assert.equal(page.requests[before + 1].url, ALERTS_ADDRESS);
  assert.deepEqual(bodyOf(page.requests[before + 1]), { reliable_alerts: true });
  answer(page.requests[before + 1], 200);
  await flush();
  assert.equal(page.requests[before + 2].url, DONE_ADDRESS);
  assert.equal(page.requests.length, before + 3);
});

test('named pop-up: Allow after a pick with the box unticked saves only the home area', async () => {
  const page = atStep4({ askHome: '' });
  click(page, 'welcome-continue-4');
  await pickInAsk(page);
  const before = page.requests.length;
  click(page, 'welcome-alerts-allow');
  await flush();
  answer(page.requests[before], 200);
  await flush();
  assert.deepEqual(sent(page).slice(before), ['PATCH ' + PROFILE_ADDRESS, 'POST ' + DONE_ADDRESS]);
});

for (const [label, fail] of [
  ['a 500', (req) => answer(req, 500)],
  ['a 400', (req) => answer(req, 400, { error: { message: 'Pick a place.' } })],
  ['a network failure', (req) => req.reject(new Error('offline'))]
]) {
  test('named pop-up: when the home area save fails (' + label + ') there is no Reliable request, and first-run-done still goes out once', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    reliableBox(page).checked = true;
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    fail(page.requests[before]);
    await flush();
    assert.equal(page.requests.filter((r) => r.url === ALERTS_ADDRESS).length, 0);
    assert.equal(doneRequests(page).length, 1);
    answer(doneRequests(page)[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
  });

  test('named pop-up: when the Reliable save fails (' + label + ') first-run-done still goes out once', async () => {
    const page = atStep4();
    click(page, 'welcome-continue-4');
    reliableBox(page).checked = true;
    click(page, 'welcome-alerts-allow');
    await flush();
    assert.equal(page.requests[0].url, ALERTS_ADDRESS);
    fail(page.requests[0]);
    await flush();
    assert.equal(doneRequests(page).length, 1);
    assert.equal(page.requests.length, 2);
    answer(page.requests[1], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
  });
}

for (const mode of ENABLE_MODES) {
  test('named pop-up: with enable ' + mode + ' the saves still run and first-run-done is sent once', async () => {
    const notify = makeNotify({ mode });
    const page = atStep4({ notify, askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    reliableBox(page).checked = true;
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    answer(page.requests[before], 200);
    await flush();
    answer(page.requests[before + 1], 200);
    await flush();
    assert.deepEqual(sent(page).slice(before), ['PATCH ' + PROFILE_ADDRESS, 'PATCH ' + ALERTS_ADDRESS, 'POST ' + DONE_ADDRESS]);
    assert.equal(notify.calls.enable.length, 1);
    assert.equal(doneRequests(page).length, 1);
  });
}

for (const [label, leave] of [
  ['Not now', (page) => click(page, 'welcome-alerts-later')],
  ['Escape', (page) => fire(page.dialog, 'cancel')]
]) {
  test('named pop-up: ' + label + ' after a pick and a ticked box saves nothing and never calls enable', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    reliableBox(page).checked = true;
    const before = page.requests.length;
    leave(page);
    await flush();
    assert.deepEqual(sent(page).slice(before), ['POST ' + DONE_ADDRESS]);
    assert.equal(page.notify.calls.enable.length, 0);
    assert.equal(page.permissionCalls.length, 0);
    assert.equal(page.requests.filter((r) => r.url === PROFILE_ADDRESS || r.url === ALERTS_ADDRESS).length, 0);
  });
}

test('named pop-up: inside a Mac Dock app there is no pop-up and first-run-done goes out directly', () => {
  const page = atStep4({ notify: makeNotify({ macDock: true }) });
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.showCalls, 0, 'showModal is never called');
  assert.equal(doneRequests(page).length, 1);
  assert.equal(page.notify.calls.enable.length, 0);

  const browser = atStep4({ notify: makeNotify({ macDock: false }) });
  click(browser, 'welcome-continue-4');
  assert.equal(browser.dialog.showCalls, 1, 'a Mac browser tab still gets the pop-up');
});

test('named pop-up: after a failed first-run-done the pop-up frees Allow by the home area rule', async () => {
  const page = atStep4({ askHome: '' });
  click(page, 'welcome-continue-4');
  await pickInAsk(page);
  const before = page.requests.length;
  click(page, 'welcome-alerts-allow');
  await flush();
  answer(page.requests[before], 200);
  await flush();
  answer(page.requests[before + 1], 500);
  await flush();
  assert.equal(page.dialog.open, false);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false, 'a place is known, so Allow is on');
  assert.equal(reliableBox(page).disabled, false);
  assert.equal(byId(page, 'welcome-error-4').textContent, SAVE_FAILED);
});
