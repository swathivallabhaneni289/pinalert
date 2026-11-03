// welcome_alerts_test.mjs - runs the real web/static/js/welcome.js in a Node
// sandbox and checks the alerts pop-up that opens after step 4 of the
// first-time flow (quick 261027-t5w, owner, 2026-10-27). It replaced step 5,
// Alerts, which was a whole page (Phase 3.3, plan 13); the flow has four steps.
//
//   - step 4 finishing (Continue with a saved profile, and Continue with nothing
//     to save; amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip, and
//     Continue needs a username, a name and a home area) opens a small native modal dialog when notify.js baseState
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
//   - a 409 goes back to step 2 (a 409 with field profile goes back to step 4,
//     tested in welcome_flow_test.mjs); any other failure closes the pop-up, shows the
//     step 4 save line, and a second Continue finishes without asking again;
//   - the flow counts four steps;
//   - (plan 03.3-19, the 03.3-UAT gap for test 4, owner 2026-10-27) the pop-up now
//     names the home area in its title, asks for the home area first when none is
//     stored or saved at step 4 (the step 4 place search, a picked place only,
//     Allow disabled until one is picked). Allow calls enable first, then saves
//     the picked home area, then first-run-done, and every failure still reaches
//     first-run-done. Not now and Escape save nothing. A Mac Dock app never gets
//     the pop-up. Amended 2026-11-03 by quick 261103-jal (D5): the pop-up no
//     longer offers a Reliable tick box and sends no alerts request. The cases
//     that cover this start with "named pop-up: ".
//   - Amended 2026-10-29 by Phase 7.1 plan 13b (D-17, D-18): an iPhone or iPad
//     Safari tab (needs-install) now gets the install pop-up, not the alerts
//     pop-up, when the page draws it (install: true in buildPage): once per
//     account through the list in pinalert.installAsked, one Not now button, never
//     enable. Without the install dialog on the page, the needs-install cases
//     above are unchanged. The cases that cover this start with "install pop-up: ".
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
// Amended 2026-10-29 by Phase 7.1 plan 13: step 4 asks for a Username with the
// shared live check, so the page loads username-check.js first, as the template does.
const checkSource = fs.readFileSync(path.join(here, 'static', 'js', 'username-check.js'), 'utf8');

const KEY = 'test-public-key-placeholder';
const AGREE_FIRST = 'Please agree to the Disclaimer and policy to continue.';
const SAVE_FAILED = 'Could not save. Try again in a moment.';
const HOME_NOT_SAVED = 'Could not save your home area. Try again in a moment.';

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
// askErrorLine false draws the pop-up as it was before WR-05, without its error line.
// Amended 2026-10-29 by Phase 7.1 plan 13: username is the name the account has
// already (the value the template draws on step 4).
// Amended 2026-10-29 by Phase 7.1 plan 13b: install true draws the install pop-up
// (welcome-install-ask) after the alerts pop-up, as the template does, and
// accountId is the id the server writes on the flow root. Left out, the page is
// as it was, so the earlier cases never meet the install pop-up. installShow
// false draws it without showModal.
function buildPage({ start = 1, name = '', home = '', username = '', key = KEY, dialogSupport = true, askHome = 'Benz Circle', askErrorLine = true, install = false, accountId = '', installShow = true } = {}) {
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
  if (accountId) { root.setAttribute('data-account-id', accountId); }

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

  const userBox = make('div', '', steps[3], 'username-field');
  make('input', 'welcome-username-input', userBox).value = username;
  make('p', 'welcome-username-status', steps[3], 'username-status');
  make('input', 'welcome-name-input', steps[3]).value = name;
  const form4 = make('form', 'welcome-home-form', steps[3]);
  make('input', 'welcome-home-input', form4).value = home;
  make('p', 'welcome-home-status', steps[3]).hidden = true;
  make('ul', 'welcome-home-results', steps[3]).hidden = true;
  make('p', 'welcome-error-4', steps[3]).hidden = true;
  // Amended 2026-10-29 by Phase 7.1 plan 13: Continue starts off, as the template
  // draws it, and there is no Skip button any more.
  const continue4 = make('button', 'welcome-continue-4', steps[3], 'btn btn--off');
  continue4.setAttribute('aria-disabled', 'true');

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
  // The home area search block, hidden when an area is stored. Amended
  // 2026-11-03 by quick 261103-jal (D5): the Reliable tick box is gone.
  const askHomeBlock = make('div', 'welcome-alerts-home', dialog, 'welcome-ask__home');
  if (askHome) { askHomeBlock.hidden = true; }
  const askForm = make('form', 'welcome-alerts-home-form', askHomeBlock);
  make('input', 'welcome-alerts-home-input', askForm);
  make('p', 'welcome-alerts-home-status', askHomeBlock).hidden = true;
  make('ul', 'welcome-alerts-home-results', askHomeBlock).hidden = true;
  if (askErrorLine) {
    make('p', 'welcome-alerts-ask-error', dialog, 'inline-error').hidden = true;
  }
  make('button', 'welcome-alerts-later', dialog);
  make('button', 'welcome-alerts-allow', dialog);

  let installDialog = null;
  if (install) {
    installDialog = make('dialog', 'welcome-install-ask', doc.body);
    installDialog.open = false;
    installDialog.showCalls = 0;
    installDialog.closeCalls = 0;
    installDialog.throwOnShow = false;
    if (installShow) {
      installDialog.showModal = function () {
        this.showCalls += 1;
        if (this.throwOnShow) { throw new Error('InvalidStateError'); }
        this.open = true;
      };
      installDialog.close = function () { this.closeCalls += 1; this.open = false; };
    }
    installDialog.setAttribute('data-home-area', askHome);
    const installTitle = make('h2', 'welcome-install-ask-title', installDialog, 'logout-confirm__title');
    installTitle.setAttribute('tabindex', '-1');
    installTitle.textContent = 'Get alerts for critical reports near ' + (askHome || 'your home area');
    make('button', 'welcome-install-later', installDialog);
  }

  return { doc, root, steps, dialog, installDialog };
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
  // Amended 2026-10-29 by Phase 7.1 plan 13: the live username checks are kept
  // apart from the other requests, so the assertions on the requests of the flow
  // keep their order and their count.
  const checks = [];
  const replaced = [];
  const session = {};
  // Amended 2026-10-29 by Phase 7.1 plan 13b: the browser's local storage, for the
  // list in pinalert.installAsked. options.storage is an object of starting values,
  // or 'throw-get' / 'throw-set' for a storage that cannot be read or written. Left
  // out, the sandbox has none, which reads as a storage that cannot be read.
  const stored = Object.assign({}, typeof options.storage === 'object' ? options.storage : {});
  const storageWrites = [];
  const localStorage = options.storage === undefined ? undefined : {
    getItem(k) {
      if (options.storage === 'throw-get') { throw new Error('SecurityError'); }
      return Object.hasOwn(stored, k) ? stored[k] : null;
    },
    setItem(k, v) {
      if (options.storage === 'throw-set') { throw new Error('QuotaExceededError'); }
      storageWrites.push([k, String(v)]);
      stored[k] = String(v);
    }
  };
  const permissionCalls = [];
  // Timers the test moves by hand (the live check waits 300 ms after typing).
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
  const sandbox = {
    console,
    document: page.doc,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        const list = String(target).startsWith('/api/username/check') ? checks : requests;
        list.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimeout(id) { timers.delete(id); },
    AbortController: class { constructor() { this.signal = { aborted: false }; } abort() { this.signal.aborted = true; } },
    navigator: { geolocation: { getCurrentPosition() {} } },
    PinalertPosition: { markAsked() { return true; }, save() { return true; }, read() { return null; } },
    // The browser's own prompt: only the Allow click, through enable, may reach it.
    Notification: { permission: 'default', requestPermission() { permissionCalls.push(true); return Promise.resolve('granted'); } },
    sessionStorage: {
      setItem(k, v) { session[k] = String(v); },
      getItem(k) { return Object.hasOwn(session, k) ? session[k] : null; }
    },
    localStorage,
    location: { search: options.search || '', replace(target) { replaced.push(target); } },
    history: { pushState() {}, replaceState() {} },
    getComputedStyle() { return { display: 'block' }; },
    addEventListener() {}
  };
  if (notify) { sandbox.PinalertNotify = notify; }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(checkSource, sandbox, { filename: 'username-check.js' });
  vm.runInContext(source, sandbox, { filename: 'welcome.js' });
  return { ...page, api: sandbox.PinalertWelcome, notify, requests, checks, advance, replaced, session, permissionCalls, sandbox, stored, storageWrites };
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

// An agreed account at step 4.
// Amended 2026-10-29 by Phase 7.1 plan 13: Continue needs a username, a name and a
// home area, so the default account already holds all three and Continue saves
// nothing, as it did when step 4 was optional. The home area is drawn in step 4's
// field (Benz Circle), whatever the pop-up's data-home-area says: a case that
// passes askHome: '' reaches the pop-up's own home area search, the defensive path
// the pop-up keeps for an account that gets there with no home area known.
const STORED = { username: 'asha.k', name: 'Asha', home: 'Benz Circle' };
const atStep4 = (options = {}) => load({ start: 4, ...STORED, ...options });

// An agreed account at step 4 that holds nothing yet, for the cases that fill the
// three fields in.
const emptyStep4 = (options = {}) => load({ start: 4, ...options });

// fillStep4 does what a person does on step 4 now that all three fields are
// required: type a username and let the 300 ms pause pass, answer its live check
// with available, type a name, and search for and pick a home area. A part set to
// null is skipped. The home area search request is taken out of page.requests
// again, so the assertions on the requests that come after keep their order.
async function fillStep4(page, options = {}) {
  const o = { username: 'ravi.k', name: 'Ravi Kumar', place: { name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru', lat: 12.9784, lon: 77.6408 }, ...options };
  if (o.username !== null) {
    const user = byId(page, 'welcome-username-input');
    user.value = o.username;
    fire(user, 'input');
    page.advance(300);
    answer(page.checks[page.checks.length - 1], 200, { state: 'available', message: o.username + ' is available.' });
    await flush();
  }
  if (o.name !== null) {
    const name = byId(page, 'welcome-name-input');
    name.value = o.name;
    fire(name, 'input');
  }
  if (o.place !== null) {
    byId(page, 'welcome-home-input').value = o.place.name;
    fire(byId(page, 'welcome-home-form'), 'submit');
    const search = page.requests[page.requests.length - 1];
    answer(search, 200, { results: [o.place] });
    await flush();
    fire(byId(page, 'welcome-home-results').children[0].children[0], 'click');
    page.requests.splice(page.requests.indexOf(search), 1);
  }
}

// Step 4 Continue with a changed name: the profile PATCH is answered 200, which
// leads to the pop-up.
async function continueWithProfile(page) {
  byId(page, 'welcome-name-input').value = 'Asha Rao';
  fire(byId(page, 'welcome-name-input'), 'input');
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

  // Amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip. Continue opens the
  // pop-up with nothing to save, and again after the three fields were filled in
  // and saved.
  test('base state ' + base + ': Continue with nothing to save, and Continue after the fields were filled in, each open the pop-up', async () => {
    const cont = atStep4({ notify: makeNotify({ base }) });
    click(cont, 'welcome-continue-4');
    assertAskOpen(cont);
    assert.equal(cont.requests.length, 0);

    const filled = emptyStep4({ notify: makeNotify({ base }) });
    await fillStep4(filled);
    click(filled, 'welcome-continue-4');
    answer(filled.requests[0], 200);
    await flush();
    assertAskOpen(filled);
    assert.equal(filled.requests.length, 1, 'only the profile request');
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

    // Amended 2026-10-29 by Phase 7.1 plan 13: no Skip; the same holds after the
    // three fields were filled in and saved.
    const filled = load({ start: 4, ...options });
    await fillStep4(filled);
    click(filled, 'welcome-continue-4');
    answer(filled.requests[0], 200);
    await flush();
    assert.equal(filled.dialog.showCalls, 0);
    assert.equal(doneRequests(filled).length, 1);
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

// Amended 2026-10-29 by Phase 7.1 plan 13: the second case used Skip; it now
// reaches the pop-up by filling the three fields in and pressing Continue.
for (const [label, fail, open] of [
  ['a 500 after Continue', (req) => answer(req, 500), async () => {
    const page = atStep4();
    click(page, 'welcome-continue-4');
    return page;
  }],
  ['a network failure after the three fields were filled in', (req) => req.reject(new Error('offline')), async () => {
    const page = emptyStep4();
    await fillStep4(page);
    click(page, 'welcome-continue-4');
    answer(page.requests[0], 200);
    await flush();
    return page;
  }]
]) {
  test(label + ' closes the pop-up, shows the step 4 line, and a second Continue finishes without asking again', async () => {
    const page = await open();
    assertAskOpen(page);
    click(page, 'welcome-alerts-allow');
    await flush();
    assert.equal(doneRequests(page).length, 1);
    fail(doneRequests(page)[0]);
    await flush();

    assert.equal(page.dialog.open, false, 'the pop-up is closed');
    assert.equal(page.dialog.getAttribute('aria-busy'), null);
    assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
    assert.equal(byId(page, 'welcome-alerts-later').disabled, false);
    assert.equal(visibleStep(page), 4);
    assert.equal(byId(page, 'welcome-error-4').hidden, false);
    assert.equal(byId(page, 'welcome-error-4').textContent, SAVE_FAILED);
    assert.equal(byId(page, 'welcome-continue-4').disabled, false);
    assert.ok(page.doc.activeElement === byId(page, 'welcome-continue-4'), 'focus goes back to the button that was pressed');
    assert.deepEqual(page.replaced, []);

    click(page, 'welcome-continue-4');
    assert.equal(page.dialog.showCalls, 1, 'the pop-up is not shown again');
    assert.equal(page.notify.calls.enable.length, 1, 'enable is not called again');
    assert.equal(doneRequests(page).length, 2, 'a second first-run-done goes out directly');
    answer(doneRequests(page)[1], 204);
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

  // Amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip; a second press of
  // Continue is held to the same rule.
  page.api.goTo(4);
  click(page, 'welcome-continue-4');
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

// ---- Plan 03.3-19: the pop-up names the home area and asks for it when it is
// missing (03.3-UAT gap for test 4) ----
//
// Amended 2026-11-03 by quick 261103-jal (owner, two row notifications box, D5,
// D9 f): the Reliable reporter tick box is gone from the pop-up, and Allow no
// longer sends a request to the alerts address. The cases that ticked the box and
// expected the Reliable request now expect the order enable, the home area save
// when one was picked, then first-run-done, and no alerts request at all.

const AREA_URL = '/api/geocode/place?kind=area&q=';
const ALERTS_ADDRESS = '/api/me/alerts';
const PROFILE_ADDRESS = '/api/me/profile';
const DONE_ADDRESS = '/api/me/first-run-done';
const titleFor = (area) => 'Get alerts for critical reports near ' + area + '?';
const TITLE_NO_HOME = titleFor('your home area');
const HEBBAL = { name: 'Hebbal', display_name: 'Hebbal, Bengaluru, Karnataka', lat: 13.0358, lon: 77.597 };

const askTitleText = (page) => byId(page, 'welcome-alerts-ask-title').textContent;
const askHomeBlock = (page) => byId(page, 'welcome-alerts-home');
const alertsRequests = (page) => page.requests.filter((r) => r.url === ALERTS_ADDRESS);
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

test('named pop-up: a stored home area is named in the title, the search is hidden, Allow is on, and there is no tick box', () => {
  const page = atStep4({ askHome: 'Benz Circle' });
  click(page, 'welcome-continue-4');
  assertAskOpen(page);
  assert.equal(askTitleText(page), titleFor('Benz Circle'));
  assert.equal(askHomeBlock(page).hidden, true);
  assert.equal(byId(page, 'welcome-alerts-allow').disabled, false);
  assert.ok(!byId(page, 'welcome-alerts-reliable'), 'the Reliable tick box is gone');
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

test('named pop-up: Allow calls enable in the tick, then sends one first-run-done and no alerts request', async () => {
  const notify = makeNotify({ mode: 'pending' });
  const page = atStep4({ notify });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-allow');
  assert.deepEqual(notify.calls.enable, [KEY], 'enable is called in the same tick, with the page key');
  assert.equal(page.requests.length, 0, 'no request before enable settles');
  assert.equal(page.dialog.getAttribute('aria-busy'), 'true');

  notify.waiting.enable.resolve({ ok: true, state: 'on' });
  await flush();
  assert.deepEqual(sent(page), ['POST ' + DONE_ADDRESS]);
  assert.equal(alertsRequests(page).length, 0, 'no request goes to the alerts address');
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('named pop-up: Allow with a home area stored sends no alerts request and one first-run-done', async () => {
  const page = atStep4();
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-allow');
  await flush();
  assert.deepEqual(sent(page), ['POST ' + DONE_ADDRESS]);
  assert.equal(page.notify.calls.enable.length, 1);
  assert.equal(alertsRequests(page).length, 0);
});

test('named pop-up: Allow after a pick saves the home area, then first-run-done, in that order after enable', async () => {
  const page = atStep4({ askHome: '' });
  click(page, 'welcome-continue-4');
  await pickInAsk(page);
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
  assert.equal(page.requests[before + 1].url, DONE_ADDRESS);
  assert.equal(page.requests.length, before + 2);
  assert.equal(alertsRequests(page).length, 0);
});

test('named pop-up: Allow after a pick saves only the home area before first-run-done', async () => {
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
  // WR-05 (03.3-REVIEW): a home area that was not saved is said in the pop-up,
  // which stays open. Allow tries the save again without a second enable, and
  // Not now still finishes sign up.
  test('named pop-up: when the home area save fails (' + label + ') the pop-up stays open with one error line, and no first-run-done request goes out', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    fail(page.requests[before]);
    await flush();
    assert.equal(page.requests.length, before + 1, 'nothing more is sent');
    assert.equal(alertsRequests(page).length, 0);
    assert.equal(doneRequests(page).length, 0);
    assert.equal(page.dialog.open, true, 'the pop-up stays open');
    const line = byId(page, 'welcome-alerts-ask-error');
    assert.equal(line.hidden, false);
    assert.equal(line.textContent, label === 'a 400' ? 'Pick a place.' : HOME_NOT_SAVED);
    assert.equal(page.dialog.getAttribute('aria-busy'), null);
    assert.equal(byId(page, 'welcome-alerts-allow').disabled, false, 'Allow can try again');
    assert.equal(byId(page, 'welcome-alerts-later').disabled, false);
    assert.deepEqual(page.replaced, []);
  });

  test('named pop-up: after a failed home area save (' + label + ') Allow saves it again without a second enable, then first-run-done', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    fail(page.requests[before]);
    await flush();
    click(page, 'welcome-alerts-allow');
    assert.equal(page.notify.calls.enable.length, 1, 'enable, and so the prompt, runs once');
    assert.equal(page.permissionCalls.length, 0);
    assert.equal(byId(page, 'welcome-alerts-ask-error').hidden, true, 'the line goes while it tries again');
    await flush();
    assert.equal(page.requests[before + 1].url, PROFILE_ADDRESS);
    assert.deepEqual(bodyOf(page.requests[before + 1]), { home_area: { name: 'Hebbal', lat: 13.0358, lon: 77.597 } });
    answer(page.requests[before + 1], 200);
    await flush();
    assert.deepEqual(sent(page).slice(before), ['PATCH ' + PROFILE_ADDRESS, 'PATCH ' + PROFILE_ADDRESS, 'POST ' + DONE_ADDRESS]);
    assert.equal(doneRequests(page).length, 1);
    answer(doneRequests(page)[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
  });

  test('named pop-up: after a failed home area save (' + label + ') Not now still finishes sign up and saves nothing more', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    fail(page.requests[before]);
    await flush();
    click(page, 'welcome-alerts-later');
    await flush();
    assert.deepEqual(sent(page).slice(before), ['PATCH ' + PROFILE_ADDRESS, 'POST ' + DONE_ADDRESS]);
    assert.equal(page.notify.calls.enable.length, 1);
    answer(doneRequests(page)[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
  });

  test('named pop-up: a page without the error line (' + label + ') finishes as before, with no alerts request', async () => {
    const page = atStep4({ askHome: '', askErrorLine: false });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    fail(page.requests[before]);
    await flush();
    assert.equal(alertsRequests(page).length, 0);
    assert.equal(doneRequests(page).length, 1);
  });
}

for (const mode of ENABLE_MODES) {
  test('named pop-up: with enable ' + mode + ' the saves still run and first-run-done is sent once', async () => {
    const notify = makeNotify({ mode });
    const page = atStep4({ notify, askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    answer(page.requests[before], 200);
    await flush();
    assert.deepEqual(sent(page).slice(before), ['PATCH ' + PROFILE_ADDRESS, 'POST ' + DONE_ADDRESS]);
    assert.equal(notify.calls.enable.length, 1);
    assert.equal(doneRequests(page).length, 1);
    assert.equal(alertsRequests(page).length, 0);
  });
}

for (const [label, leave] of [
  ['Not now', (page) => click(page, 'welcome-alerts-later')],
  ['Escape', (page) => fire(page.dialog, 'cancel')]
]) {
  test('named pop-up: ' + label + ' after a pick saves nothing and never calls enable', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
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
  assert.equal(byId(page, 'welcome-error-4').textContent, SAVE_FAILED);
});

// The browser can close the pop-up by itself while Allow is still busy (Chrome's
// close watcher: a second Escape, or the Android back gesture, closes a modal
// dialog with no cancelable cancel event). If the home area save then fails,
// there is nobody to show the error line to: sign up finishes as it does when
// the page has no error line, instead of staying stuck with the buttons off.
for (const [label, fail] of [
  ['a 500', (req) => answer(req, 500)],
  ['a network failure', (req) => req.reject(new Error('offline'))]
]) {
  test('named pop-up: closed by the browser while the home save is out and it then fails (' + label + ') first-run-done still goes out and Continue is freed', async () => {
    const page = atStep4({ askHome: '' });
    click(page, 'welcome-continue-4');
    await pickInAsk(page);
    const before = page.requests.length;
    click(page, 'welcome-alerts-allow');
    await flush();
    page.dialog.open = false; // closed by the browser: no cancel event, no close listener
    fail(page.requests[before]);
    await flush();
    assert.equal(byId(page, 'welcome-alerts-ask-error').hidden, true, 'nothing is written into a closed dialog');
    assert.equal(alertsRequests(page).length, 0, 'no alerts request');
    assert.equal(doneRequests(page).length, 1, 'sign up finishes');
    answer(doneRequests(page)[0], 500);
    await flush();
    assert.equal(byId(page, 'welcome-continue-4').disabled, false);
  });
}

// ---------------------------------------------------------------------------
// The install pop-up (Phase 7.1 plan 13b, D-17, D-18, UI-SPEC Surface 10a),
// added 2026-10-29. On a needs-install device (an iPhone or iPad Safari tab) the
// end of step 4 opens one small box with the install guide and one button, Not
// now, once per account on a browser (the list in pinalert.installAsked). It never
// turns alerts on and never asks the browser for anything, and whatever happens
// sign up finishes. The cases start with "install pop-up: ".
// ---------------------------------------------------------------------------

const INSTALL_KEY = 'pinalert.installAsked';
const installOf = (page) => page.installDialog;
const installPage = (options = {}) => atStep4({
  install: true,
  accountId: '42',
  storage: {},
  notify: makeNotify({ base: 'needs-install' }),
  ...options
});

function assertInstallOpen(page) {
  const box = installOf(page);
  assert.equal(box.showCalls, 1, 'showModal is called once');
  assert.equal(box.open, true);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-install-ask-title'), 'the title takes focus so no button looks chosen');
  assert.equal(doneRequests(page).length, 0, 'first-run-done waits for Not now');
  assert.equal(page.dialog.showCalls, 0, 'the alerts pop-up does not open');
  assert.equal(page.notify.calls.enable.length, 0, 'the install pop-up never turns alerts on');
  assert.equal(page.permissionCalls.length, 0, 'the install pop-up never asks the browser for anything');
}

test('install pop-up: a needs-install device opens it after step 4, names the home area, records the account and sends no first-run-done', () => {
  const page = installPage();
  click(page, 'welcome-continue-4');
  assertInstallOpen(page);
  assert.equal(byId(page, 'welcome-install-ask-title').textContent, 'Get alerts for critical reports near Benz Circle');
  assert.deepEqual(JSON.parse(page.stored[INSTALL_KEY]), ['42']);
  assert.equal(page.requests.length, 0);
  assert.equal(visibleStep(page), 4, 'the flow stays on step 4 behind the pop-up');
});

test('install pop-up: the title names the place saved at step 4 in this visit, and your home area when none is known', async () => {
  const none = installPage({ askHome: '' });
  byId(none, 'welcome-name-input').value = 'Asha Rao';
  fire(byId(none, 'welcome-name-input'), 'input');
  click(none, 'welcome-continue-4');
  answer(none.requests[0], 200);
  await flush();
  assertInstallOpen(none);
  assert.equal(byId(none, 'welcome-install-ask-title').textContent, 'Get alerts for critical reports near your home area');

  const filled = installPage({ askHome: '', start: 4, username: '', name: '', home: '' });
  await fillStep4(filled);
  click(filled, 'welcome-continue-4');
  answer(filled.requests[0], 200);
  await flush();
  assertInstallOpen(filled);
  assert.equal(byId(filled, 'welcome-install-ask-title').textContent, 'Get alerts for critical reports near Indiranagar');
});

test('install pop-up: a home area with markup is written as text, never parsed', () => {
  const page = installPage({ askHome: '<b>x</b>' });
  click(page, 'welcome-continue-4');
  assert.equal(byId(page, 'welcome-install-ask-title').textContent, 'Get alerts for critical reports near <b>x</b>');
});

test('install pop-up: Not now closes it and sends first-run-done once, a second press sends nothing, a 204 lands on Nearby', async () => {
  const page = installPage();
  click(page, 'welcome-continue-4');
  click(page, 'welcome-install-later');
  click(page, 'welcome-install-later');
  assert.equal(installOf(page).open, false);
  assert.equal(doneRequests(page).length, 1);
  assert.equal(page.notify.calls.enable.length, 0);
  assert.equal(page.permissionCalls.length, 0);
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('install pop-up: Escape is Not now, the cancel event is prevented and one first-run-done goes out', async () => {
  const page = installPage();
  click(page, 'welcome-continue-4');
  const event = fire(installOf(page), 'cancel');
  assert.equal(event.prevented, true, 'the browser does not close the pop-up by itself');
  fire(installOf(page), 'cancel');
  assert.equal(installOf(page).open, false);
  assert.equal(doneRequests(page).length, 1);
  assert.equal(page.notify.calls.enable.length, 0);
  assert.equal(page.permissionCalls.length, 0);
  answer(page.requests[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('install pop-up: a showModal that throws, and a dialog without showModal, send first-run-done at once', () => {
  const throwing = installPage();
  installOf(throwing).throwOnShow = true;
  click(throwing, 'welcome-continue-4');
  assert.equal(installOf(throwing).open, false);
  assert.equal(doneRequests(throwing).length, 1);

  const plain = installPage({ installShow: false });
  click(plain, 'welcome-continue-4');
  assert.equal(doneRequests(plain).length, 1);
  assert.equal(plain.storageWrites.length, 0, 'a box that cannot open is not recorded as shown');

  const missing = atStep4({ accountId: '42', storage: {}, notify: makeNotify({ base: 'needs-install' }) });
  click(missing, 'welcome-continue-4');
  assert.equal(doneRequests(missing).length, 1, 'a page without the install dialog finishes directly');
});

test('install pop-up: once per account, a list holding this account id does not open it again and sign up finishes at once', () => {
  const page = installPage({ storage: { [INSTALL_KEY]: JSON.stringify(['7', '42']) } });
  click(page, 'welcome-continue-4');
  assert.equal(installOf(page).showCalls, 0);
  assert.equal(page.dialog.showCalls, 0);
  assert.equal(doneRequests(page).length, 1);
  assert.equal(page.storageWrites.length, 0);
});

test('install pop-up: other accounts on a shared phone do not hide it, and their ids stay in the list', () => {
  const list = installPage({ storage: { [INSTALL_KEY]: JSON.stringify(['7', '9']) } });
  click(list, 'welcome-continue-4');
  assertInstallOpen(list);
  assert.deepEqual(JSON.parse(list.stored[INSTALL_KEY]), ['7', '9', '42']);

  const single = installPage({ storage: { [INSTALL_KEY]: '7' } });
  click(single, 'welcome-continue-4');
  assertInstallOpen(single);
  assert.deepEqual(JSON.parse(single.stored[INSTALL_KEY]), ['7', '42'], 'one plain id reads as a list of one');

  const single42 = installPage({ storage: { [INSTALL_KEY]: '42' } });
  click(single42, 'welcome-continue-4');
  assert.equal(installOf(single42).showCalls, 0, 'one plain id of this account counts as shown');
  assert.equal(doneRequests(single42).length, 1);
});

test('install pop-up: the list keeps the last 8 ids, a ninth drops the oldest', () => {
  const eight = ['1', '2', '3', '4', '5', '6', '7', '8'];
  const page = installPage({ storage: { [INSTALL_KEY]: JSON.stringify(eight) } });
  click(page, 'welcome-continue-4');
  assertInstallOpen(page);
  assert.deepEqual(JSON.parse(page.stored[INSTALL_KEY]), ['2', '3', '4', '5', '6', '7', '8', '42']);
});

test('install pop-up: a stored value that is neither an id nor a list counts as not shown and is replaced', () => {
  for (const junk of ['not json', '{"a":1}', '"42"', '[42, null, {}]', 'null']) {
    const page = installPage({ storage: { [INSTALL_KEY]: junk } });
    click(page, 'welcome-continue-4');
    assertInstallOpen(page);
    assert.deepEqual(JSON.parse(page.stored[INSTALL_KEY]), ['42'], 'junk ' + junk);
  }
});

test('install pop-up: a storage that cannot be read or written, or none at all, still opens it and nothing throws', () => {
  for (const storage of ['throw-get', 'throw-set', undefined]) {
    const page = installPage({ storage });
    click(page, 'welcome-continue-4');
    assertInstallOpen(page);
  }
});

test('install pop-up: it opens only for needs-install, so it never opens beside the alerts pop-up', () => {
  for (const base of ['maybe', 'ready']) {
    const page = installPage({ notify: makeNotify({ base }) });
    click(page, 'welcome-continue-4');
    assert.equal(installOf(page).showCalls, 0, base);
    assert.equal(page.dialog.showCalls, 1, base + ': the alerts pop-up behaves as before');
    // Amended 2026-10-29 by Phase 7.1 plan 13c: the alerts pop-up now writes the
    // account id to pinalert.alertsAsked when it opens, so this line looks at the
    // install key only. The install key is still never written here.
    assert.equal(page.storageWrites.filter(([k]) => k === INSTALL_KEY).length, 0, base);
  }
  for (const base of ['blocked', 'unsupported', 'surprise']) {
    const page = installPage({ notify: makeNotify({ base }) });
    click(page, 'welcome-continue-4');
    assert.equal(installOf(page).showCalls, 0, base);
    assert.equal(page.dialog.showCalls, 0, base);
    assert.equal(doneRequests(page).length, 1, base + ': first-run-done at once');
  }
  const noNotify = installPage({ notify: null });
  click(noNotify, 'welcome-continue-4');
  assert.equal(installOf(noNotify).showCalls, 0);
  assert.equal(doneRequests(noNotify).length, 1);
  const throwing = installPage({ notify: { baseState() { throw new Error('boom'); } } });
  click(throwing, 'welcome-continue-4');
  assert.equal(installOf(throwing).showCalls, 0);
  assert.equal(doneRequests(throwing).length, 1);
});

test('install pop-up: a 409 with field profile returns to step 4, any other 409 to step 2, and the pop-up is closed', async () => {
  const profile = installPage();
  click(profile, 'welcome-continue-4');
  click(profile, 'welcome-install-later');
  answer(profile.requests[0], 409, { error: { field: 'profile', message: 'Add your username, name and home area.' } });
  await flush();
  assert.equal(installOf(profile).open, false);
  assert.equal(visibleStep(profile), 4);
  assert.equal(byId(profile, 'welcome-error-4').textContent, 'Add your username, name and home area.');

  const terms = installPage();
  click(terms, 'welcome-continue-4');
  click(terms, 'welcome-install-later');
  answer(terms.requests[0], 409, { error: { field: 'terms', message: 'Agree first.' } });
  await flush();
  assert.equal(installOf(terms).open, false);
  assert.equal(visibleStep(terms), 2);
  assert.equal(byId(terms, 'welcome-error-2').textContent, AGREE_FIRST);
});

test('install pop-up: after a failed first-run-done the next Continue records the flow again without showing the box a second time', async () => {
  const page = installPage();
  click(page, 'welcome-continue-4');
  click(page, 'welcome-install-later');
  answer(page.requests[0], 500);
  await flush();
  assert.equal(byId(page, 'welcome-error-4').textContent, SAVE_FAILED);
  click(page, 'welcome-continue-4');
  assert.equal(installOf(page).showCalls, 1, 'not shown again');
  assert.equal(doneRequests(page).length, 2);
  answer(page.requests[1], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('install pop-up: a step change under the open box puts it away without sending anything', () => {
  const page = installPage();
  click(page, 'welcome-continue-4');
  assert.equal(installOf(page).open, true);
  page.api.goTo(3);
  assert.equal(installOf(page).open, false);
  assert.equal(page.requests.length, 0);
  assert.equal(byId(page, 'welcome-continue-4').disabled, false);
});

test('install pop-up: it lives outside the flow root, so its keys never reach the step Tab trap', () => {
  const page = installPage();
  assert.equal(page.root.contains(installOf(page)), false);
  assert.equal(page.doc.body.contains(installOf(page)), true);
});

test('install pop-up: the script keeps it away from the browser prompt and from markup sinks', () => {
  const body = (name) => {
    const at = source.indexOf('function ' + name);
    assert.ok(at !== -1, name);
    return source.slice(at, source.indexOf('\n  }\n', at));
  };
  for (const name of ['canInstallAsk', 'openInstallAsk', 'onInstallLater', 'onInstallCancel', 'rememberInstallAsk', 'installAskedList']) {
    assert.doesNotMatch(body(name), /enable|requestPermission|Notification|innerHTML|insertAdjacentHTML/, name);
  }
});

// ---------------------------------------------------------------------------
// The asked key shared with the installed app (Phase 7.1 plan 13c, D-17, D-18),
// added 2026-10-29. When the alerts pop-up opens, welcome.js adds the account id
// to the list in pinalert.alertsAsked, the key installed-ask.js reads on Nearby,
// so a person who signed up inside the installed app is not asked a second time.
// The cases start with "asked key: ".
// ---------------------------------------------------------------------------

const ALERTS_KEY = 'pinalert.alertsAsked';
const alertsPage = (options = {}) => atStep4({ accountId: '42', storage: {}, notify: makeNotify({ base: 'maybe' }), ...options });

test('asked key: opening the alerts pop-up adds the account id to the list, just before showModal', () => {
  const page = alertsPage();
  click(page, 'welcome-continue-4');
  assertAskOpen(page);
  assert.deepEqual(JSON.parse(page.stored[ALERTS_KEY]), ['42']);
  assert.equal(page.storageWrites.filter(([k]) => k === INSTALL_KEY).length, 0, 'the install key is not touched');
});

test('asked key: other accounts stay in the list, an older single id is a list of one, a ninth drops the oldest', () => {
  const list = alertsPage({ storage: { [ALERTS_KEY]: JSON.stringify(['7', '9']) } });
  click(list, 'welcome-continue-4');
  assert.deepEqual(JSON.parse(list.stored[ALERTS_KEY]), ['7', '9', '42']);

  const single = alertsPage({ storage: { [ALERTS_KEY]: '7' } });
  click(single, 'welcome-continue-4');
  assert.deepEqual(JSON.parse(single.stored[ALERTS_KEY]), ['7', '42']);

  const eight = ['1', '2', '3', '4', '5', '6', '7', '8'];
  const full = alertsPage({ storage: { [ALERTS_KEY]: JSON.stringify(eight) } });
  click(full, 'welcome-continue-4');
  assert.deepEqual(JSON.parse(full.stored[ALERTS_KEY]), ['2', '3', '4', '5', '6', '7', '8', '42']);

  // Amended 2026-10-29 by the Phase 7.1 review fix (browser), WR-B06: an account
  // already in the list is not asked again (D-18), so the pop-up does not open and
  // the list is left as it is; before, it opened and moved the id to the end.
  const twice = alertsPage({ storage: { [ALERTS_KEY]: JSON.stringify(['42', '7']) } });
  click(twice, 'welcome-continue-4');
  assert.equal(twice.dialog.showCalls, 0);
  assert.deepEqual(JSON.parse(twice.stored[ALERTS_KEY]), ['42', '7'], 'the list is left as it was');
});

// Phase 7.1 review fix (browser), IN-B06: the id is added to the list just before
// showModal; when showModal throws, the box never opened, so the id is taken back
// out (the other ids stay) and the flow still finishes.
test('IN-B06: a showModal that throws takes this account back out of the alerts list', () => {
  const page = alertsPage({ storage: { [ALERTS_KEY]: JSON.stringify(['7']) } });
  page.dialog.throwOnShow = true;
  click(page, 'welcome-continue-4');
  assert.deepEqual(JSON.parse(page.stored[ALERTS_KEY]), ['7']);
  assert.equal(doneRequests(page).length, 1);
});

test('IN-B06: a showModal that throws takes this account back out of the install list', () => {
  const page = installPage({ storage: { [INSTALL_KEY]: JSON.stringify(['7']) } });
  installOf(page).throwOnShow = true;
  click(page, 'welcome-continue-4');
  assert.deepEqual(JSON.parse(page.stored[INSTALL_KEY]), ['7']);
  assert.equal(doneRequests(page).length, 1);
});

// Phase 7.1 review fix (browser), WR-B06: the sign up pop-up asks once per device
// and account (D-18) and never piles a question on a device that is already on
// (the owner: notifications few, never piled up). Every way past it still records
// the flow as done.
const INTENT = 'pinalert.pushIntent';
const notifyWithIntent = () => ({ ...makeNotify({ base: 'maybe' }), INTENT_KEY: INTENT });

test('WR-B06: an account already in pinalert.alertsAsked goes straight to first-run-done', async () => {
  for (const stored of [JSON.stringify(['7', '42']), '42']) {
    const page = alertsPage({ storage: { [ALERTS_KEY]: stored } });
    click(page, 'welcome-continue-4');
    assert.equal(page.dialog.showCalls, 0, stored);
    assert.equal(page.dialog.open, false);
    assert.equal(page.notify.calls.enable.length, 0);
    assert.equal(doneRequests(page).length, 1, 'the flow is recorded as done');
    answer(doneRequests(page)[0], 204);
    await flush();
    assert.deepEqual(page.replaced, ['/']);
    assert.equal(page.storageWrites.length, 0, 'nothing is written');
  }
});

test('WR-B06: a device whose alerts are already on for this account is not asked again', async () => {
  const page = alertsPage({ notify: notifyWithIntent(), storage: { [INTENT]: '42' } });
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.showCalls, 0);
  assert.equal(page.notify.calls.enable.length, 0);
  assert.equal(page.notify.calls.deviceState.length, 0, 'no request is made to find out');
  assert.equal(doneRequests(page).length, 1);
  answer(doneRequests(page)[0], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('WR-B06: alerts on for another account, or a flag that cannot be read, still asks this one', () => {
  const other = alertsPage({ notify: notifyWithIntent(), storage: { [INTENT]: '7' } });
  click(other, 'welcome-continue-4');
  assertAskOpen(other);

  const blocked = alertsPage({ notify: notifyWithIntent(), storage: 'throw-get' });
  click(blocked, 'welcome-continue-4');
  assertAskOpen(blocked);
});

test('WR-B06: a failed first-run-done after the skipped pop-up frees Continue, and it does not ask on the retry', async () => {
  const page = alertsPage({ storage: { [ALERTS_KEY]: '42' } });
  click(page, 'welcome-continue-4');
  answer(doneRequests(page)[0], 500, { error: { message: 'x' } });
  await flush();
  assert.equal(byId(page, 'welcome-continue-4').disabled, false);
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.showCalls, 0);
  assert.equal(doneRequests(page).length, 2);
});

test('asked key: a value that is not an id or a list is replaced, and the pop-up still opens', () => {
  for (const junk of ['not json', '{"a":1}', '[', '42x']) {
    const page = alertsPage({ storage: { [ALERTS_KEY]: junk } });
    click(page, 'welcome-continue-4');
    assertAskOpen(page);
    assert.deepEqual(JSON.parse(page.stored[ALERTS_KEY]), ['42'], junk);
  }
});

test('asked key: a storage that cannot be read or written, or none at all, does not stop the pop-up or the flow', async () => {
  for (const storage of ['throw-get', 'throw-set', undefined]) {
    const page = alertsPage({ storage });
    click(page, 'welcome-continue-4');
    assertAskOpen(page);
    click(page, 'welcome-alerts-later');
    await flush();
    assert.equal(doneRequests(page).length, 1, String(storage) + ': the flow still finishes');
  }
});

test('asked key: no account id on the flow root means nothing is written, and the pop-up opens', () => {
  const page = alertsPage({ accountId: '' });
  click(page, 'welcome-continue-4');
  assertAskOpen(page);
  assert.equal(page.storageWrites.length, 0);
});

test('asked key: nothing is written when the pop-up does not open (no showModal, a blocked or unsupported device)', () => {
  const noModal = alertsPage({ dialogSupport: false });
  click(noModal, 'welcome-continue-4');
  assert.equal(noModal.storageWrites.length, 0);

  for (const base of ['blocked', 'unsupported']) {
    const page = alertsPage({ notify: makeNotify({ base }) });
    click(page, 'welcome-continue-4');
    assert.equal(page.storageWrites.length, 0, base);
  }
});

test('asked key: a showModal that throws does not stop first-run-done', () => {
  const page = alertsPage();
  page.dialog.throwOnShow = true;
  click(page, 'welcome-continue-4');
  assert.equal(doneRequests(page).length, 1);
});
