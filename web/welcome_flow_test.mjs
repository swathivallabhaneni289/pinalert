// welcome_flow_test.mjs - runs the real web/static/js/welcome.js in a Node
// sandbox and checks the first-time flow (Phase 3.4, plan 10, D-27, D-28, D-24b):
//
//   - four steps (amended 2026-10-27 by quick 261027-t5w, owner: the alerts ask
//     is a pop-up after step 4, tested in welcome_alerts_test.mjs), one at a
//     time, the title takes focus on each, Tab stays inside, and Escape does
//     nothing because the flow is completed, not closed;
//   - Disagree shows the calm box (plan 31) and stays on step 2; Read again
//     scrolls the text box to the top and focuses it; Log out is a plain form;
//     only Agree records the agreement (POST /api/me/consent) and moves on, and
//     only on a 204;
//   - step 3 has two buttons only (plan 33): Allow location asks the browser for
//     the position, keeps the fix in this browser through PinalertPosition and
//     sends it nowhere; Not now moves on without asking; neither sends a place
//     search, and both go to step 4 whatever the browser answers;
//   - step 4's home area comes only from a picked search result of its own,
//     never from step 3 and never from the device fix; nothing is saved until
//     Continue (amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip);
//   - step 4 asks for a Username (with the shared live check), a Name and a Home
//     area, all three required: Continue is switched off until all three are
//     good, guides when pressed while off, sends the changed fields in one
//     profile request and shows a refusal of the username under the Username;
//   - finishing step 4 sends the profile (only what the person filled), then opens
//     the alerts pop-up (the default fake device can show alerts); Not now in
//     the pop-up sends first-run-done and lands on / with the search focus flag
//     set;
//   - every string reaches the page through textContent.
//
// Run it with: node --test web/welcome_flow_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what welcome.js calls: a minimal element model, fake
// timers the test moves by hand and a fetch the test answers by hand.

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
  // A small selector matcher: comma lists of .class, tag name, or the focusable
  // tabindex form welcome.js uses.
  // Amended 2026-10-29 by the Phase 5 review fix (WR-B05): a[href] matches a link
  // with an href, as in the browser.
  matches(selector) {
    return selector.split(',').map((s) => s.trim()).some((part) => {
      if (part === 'a[href]') { return this.tagName === 'A' && Object.hasOwn(this.attrs, 'href'); }
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

// Amended 2026-10-27 by quick 261027-t5w (owner): the flow has four steps. The
// alerts ask is a pop-up (a native dialog outside the flow root) that opens
// after step 4. The page below carries its ids, a default fake PinalertNotify
// answers "maybe", and the cases that finish the flow press Not now in the
// pop-up. The pop-up cases themselves are in welcome_alerts_test.mjs.
const STEP_NAMES = ['Welcome', 'Disclaimer and policy', 'Your location', 'About you'];

// The markup of welcome.html.tmpl, as the ids and nesting welcome.js reads.
// Amended 2026-10-29 by Phase 7.1 plan 13: username is the name the account has
// already (the value the template draws), afterTerms is data-after-terms-step.
function buildPage({ start = 1, name = '', home = '', username = '', afterTerms = 3, marker = true } = {}) {
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
  root.setAttribute('data-after-terms-step', String(afterTerms));

  const side = make('aside', '', root, 'welcome__side');
  const list = make('ol', '', side, 'welcome__steps');
  const items = STEP_NAMES.map((label, i) => {
    const li = make('li', '', list, 'welcome__steps-item');
    li.setAttribute('data-step', String(i + 1));
    li.setAttribute('data-state', i + 1 < start ? 'done' : (i + 1 === start ? 'current' : 'waiting'));
    const btn = make('button', 'welcome-steps-btn-' + (i + 1), li, 'welcome__steps-button');
    btn.setAttribute('data-step', String(i + 1));
    if (i + 1 === start) { btn.setAttribute('aria-current', 'step'); }
    li.button = btn;
    return li;
  });

  const pane = make('div', '', root, 'welcome__pane');
  const steps = [1, 2, 3, 4].map((n) => {
    const sec = make('section', 'welcome-step-' + n, pane, 'welcome__step');
    if (n !== start) { sec.hidden = true; }
    const title = make('h1', 'welcome-step-' + n + '-title', sec, 'welcome__title');
    title.setAttribute('tabindex', '-1');
    return sec;
  });

  // Amended 2026-10-29 by the Phase 5 review fix (WR-B05): the two Try the demo
  // links of the template, before Continue on step 1 and after Log out in the
  // Disagree box.
  make('a', 'welcome-try-demo-1', steps[0]).setAttribute('href', '/demo?from=welcome');
  make('button', 'welcome-continue-1', steps[0]);
  [2, 3, 4].forEach((n) => make('button', 'welcome-back-' + n, steps[n - 1], 'welcome__back'));

  const box = make('div', 'welcome-scrollbox', steps[1], 'welcome__scrollbox');
  box.setAttribute('tabindex', '0');
  // Quick 261027-jhd (owner, 2026-10-27): the end-of-text marker is the last
  // child of the text box, and Agree and Disagree sit in a row of their own
  // (id welcome-consent-actions) that welcome.js holds back until the marker
  // has been seen. The marker is left out when a test passes marker: false.
  if (marker) {
    make('div', 'welcome-scrollbox-end', box, 'welcome__scrollbox-end').setAttribute('aria-hidden', 'true');
  }
  const consentRow = make('div', 'welcome-consent-actions', steps[1], 'welcome__actions welcome__actions--pair');
  make('button', 'welcome-disagree', consentRow);
  make('button', 'welcome-agree', consentRow);
  // Amended 2026-10-17 by Phase 3.4 plan 31: Disagree shows a box (its words
  // live in the template) with Read again and Log out inside it.
  const disagreeBox = make('div', 'welcome-disagree-message', steps[1]);
  disagreeBox.setAttribute('tabindex', '-1');
  disagreeBox.hidden = true;
  make('button', 'welcome-reread', disagreeBox);
  make('button', 'welcome-logout', disagreeBox);
  make('a', 'welcome-try-demo-2', disagreeBox).setAttribute('href', '/demo?from=welcome');
  make('p', 'welcome-error-2', steps[1]).hidden = true;

  make('button', 'welcome-allow-location', steps[2]);
  make('button', 'welcome-not-now', steps[2]);

  // Amended 2026-10-29 by Phase 7.1 plan 13: the Username, its live line and the
  // box round them (the .username-field box the script puts a click on).
  const userBox = make('div', '', steps[3], 'username-field');
  const userInput = make('input', 'welcome-username-input', userBox);
  userInput.value = username;
  make('p', 'welcome-username-status', steps[3], 'username-status');
  const nameInput = make('input', 'welcome-name-input', steps[3]);
  nameInput.value = name;
  const form4 = make('form', 'welcome-home-form', steps[3]);
  const homeInput = make('input', 'welcome-home-input', form4);
  homeInput.value = home;
  make('p', 'welcome-home-status', steps[3]).hidden = true;
  make('ul', 'welcome-home-results', steps[3]).hidden = true;
  make('p', 'welcome-error-4', steps[3]).hidden = true;
  // Amended 2026-10-29 by Phase 7.1 plan 13: Continue starts off (aria-disabled and
  // btn--off, as the template draws it) and there is no Skip button any more.
  const continue4 = make('button', 'welcome-continue-4', steps[3], 'btn btn--off');
  continue4.setAttribute('aria-disabled', 'true');

  // The alerts pop-up (quick 261027-t5w): a native dialog that is a direct child
  // of body and a sibling of the flow root, shaped like the one in
  // logout_confirm_test.mjs.
  root.setAttribute('data-push-key', 'test-public-key-placeholder');
  const dialog = make('dialog', 'welcome-alerts-ask', doc.body);
  dialog.open = false;
  dialog.showCalls = 0;
  dialog.showModal = function () { this.showCalls += 1; this.open = true; };
  dialog.close = function () { this.open = false; };
  make('h2', 'welcome-alerts-ask-title', dialog, 'logout-confirm__title').setAttribute('tabindex', '-1');
  make('button', 'welcome-alerts-later', dialog);
  make('button', 'welcome-alerts-allow', dialog);

  return { doc, root, steps, items, side, dialog };
}

// load evaluates welcome.js against a page. fetch keeps every call for the test
// to answer. getCurrentPosition keeps its callbacks for the test to run.
function load(options) {
  const page = buildPage(options);
  // Quick 261027-jhd: an IntersectionObserver is added only when a test asks for
  // one (observer: true, or 'throws'), so every older case runs without one.
  const observers = [];
  class FakeIO {
    constructor(callback, opts) {
      if (options && options.observer === 'throws') { throw new Error('IntersectionObserver is not allowed here'); }
      this.callback = callback;
      this.options = opts;
      this.targets = [];
      this.disconnected = false;
      observers.push(this);
    }
    observe(target) { this.targets.push(target); }
    disconnect() { this.disconnected = true; }
  }
  const requests = [];
  // Amended 2026-10-29 by Phase 7.1 plan 13: the live username checks are kept
  // apart from the other requests, so the assertions on the requests of the flow
  // itself keep their order and their count.
  const checks = [];
  const positionCalls = [];
  const saved = [];
  const asked = [];
  const replaced = [];
  const session = {};
  const pushes = [];
  const stateReplaces = [];
  const listeners = {};
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
        const list = String(target).startsWith('/api/username/check') ? checks : requests;
        list.push({ url: String(target), init: init || {}, resolve, reject, at: now });
      });
    },
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimeout(id) { timers.delete(id); },
    AbortController: FakeAbortController,
    navigator: {
      geolocation: {
        getCurrentPosition(ok, fail, opts) { positionCalls.push({ ok, fail, opts }); }
      }
    },
    PinalertPosition: {
      markAsked() { asked.push(true); return true; },
      save(lat, lon) { saved.push([lat, lon]); return true; },
      read() { return null; }
    },
    sessionStorage: {
      setItem(k, v) { session[k] = String(v); },
      getItem(k) { return Object.hasOwn(session, k) ? session[k] : null; }
    },
    location: { search: (options && options.search) || '', replace(target) { replaced.push(target); } },
    history: {
      pushState(state, title, address) { pushes.push({ state, address }); },
      replaceState(state, title, address) { stateReplaces.push({ state, address }); }
    },
    // A device that can show alerts: finishing step 4 opens the pop-up. The real
    // answers are tested in welcome_alerts_test.mjs.
    PinalertNotify: {
      baseState() { return 'maybe'; },
      deviceState() { return Promise.resolve('ready'); },
      enable() { return Promise.resolve({ ok: false, state: 'ready', reason: 'failed' }); }
    },
    // The side panel is a laptop part: a test sets side.hiddenByCss to play the phone.
    getComputedStyle(node) { return { display: node.hiddenByCss ? 'none' : 'block' }; },
    addEventListener(type, fn) { (listeners[type] = listeners[type] || []).push(fn); }
  };
  if (options && options.observer) { sandbox.IntersectionObserver = FakeIO; }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(checkSource, sandbox, { filename: 'username-check.js' });
  vm.runInContext(source, sandbox, { filename: 'welcome.js' });
  const popTo = (state) => (listeners.popstate || []).forEach((fn) => fn({ type: 'popstate', state }));
  return { ...page, api: sandbox.PinalertWelcome, requests, checks, positionCalls, saved, asked, replaced, session, advance, sandbox, pushes, stateReplaces, popTo, observers };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: () => (status === 204 ? Promise.reject(new Error('no body')) : Promise.resolve(body || {}))
});

const byId = (page, id) => page.doc.byId[id];
const fire = (el, type, extra) => {
  const event = { type, target: el, key: '', shiftKey: false, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};
const click = (page, id) => fire(byId(page, id), 'click');
const visibleStep = (page) => page.steps.findIndex((s) => !s.hidden) + 1;
// Quick 261027-jhd: tell every watcher that has not been disconnected whether
// the end-of-text marker is in view, the way a browser does after a scroll.
const seeEnd = (page, isIntersecting) => {
  page.observers.filter((o) => !o.disconnected).forEach((o) => {
    o.callback(o.targets.map((target) => ({ target, isIntersecting })), o);
  });
};

// The places a search answers with, short name first.
const PLACES = [
  { name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru, Karnataka, India', lat: 12.9784, lon: 77.6408, area: 'Indiranagar' },
  { name: 'Indira Nagar', display_name: 'Indira Nagar, Lucknow, Uttar Pradesh, India', lat: 26.8806, lon: 80.9946, area: 'Indira Nagar' }
];

// Amended 2026-10-29 by Phase 7.1 plan 13: fillStep4 does what a person does on
// step 4 now that Username, Name and Home area are all required: type a username
// and let the 300 ms pause pass, answer its live check with available, type a
// name, and search for and pick a home area. It leaves Continue for the test to
// press. A part set to null is skipped (the page already holds it). The live
// checks are kept apart from the other requests (page.checks), and the home area
// search request is taken out of page.requests again, so the assertions of a test
// on the requests that come after keep their order and their count.
async function fillStep4(page, options = {}) {
  const o = { username: 'ravi.k', name: 'Ravi Kumar', place: PLACES[0], ...options };
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

// A page whose account already holds all three, so Continue is on and saves nothing.
const STORED = { username: 'asha.k', name: 'Asha', home: 'Whitefield' };

// Walk to step 4 through the buttons, the way a person does.
async function walkToStep4(page) {
  click(page, 'welcome-continue-1');
  click(page, 'welcome-agree');
  answer(page.requests[page.requests.length - 1], 204);
  await flush();
  assert.equal(visibleStep(page), 3);
}

test('Continue on step 1 shows step 2, focuses its title, and updates the progress and step list', () => {
  const page = load();
  assert.equal(visibleStep(page), 1);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-1-title'), 'the first title takes focus on load');
  click(page, 'welcome-continue-1');

  assert.equal(visibleStep(page), 2);
  assert.equal(page.root.getAttribute('data-step'), '2');
  assert.equal(page.root.getAttribute('aria-labelledby'), 'welcome-step-2-title');
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-2-title'));
  // Amended 2026-10-27 by quick 261027-t5w: four steps.
  assert.deepEqual(page.items.map((i) => i.getAttribute('data-state')), ['done', 'current', 'waiting', 'waiting']);
  assert.equal(page.items[1].button.getAttribute('aria-current'), 'step');
  assert.equal(page.items[0].button.getAttribute('aria-current'), null);
  assert.equal(page.requests.length, 0, 'moving through step 1 sends nothing');
});

test('the flow starts at the step the server names', () => {
  const page = load({ start: 3 });
  assert.equal(visibleStep(page), 3);
  assert.equal(page.requests.length, 0, 'starting at step 3 does not record anything again');
  assert.deepEqual(page.items.map((i) => i.getAttribute('data-state')), ['done', 'done', 'current', 'waiting']);
});

// Amended 2026-10-17 by Phase 3.4 plan 31: Disagree no longer shows one line
// that welcome.js writes. It shows the calm box the template draws (the words
// are in welcome.html.tmpl; the owner then cut them to two short lines and the
// buttons Read again and Log out), takes focus into it, records nothing, and
// stays on step 2. Agree is still the only way on, and a step change puts the box away.
test('Disagree shows the box, takes focus into it, sends nothing, and stays on step 2', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  const box = byId(page, 'welcome-disagree-message');
  assert.equal(box.hidden, true, 'the box starts hidden');
  click(page, 'welcome-disagree');

  assert.equal(box.hidden, false);
  // assert.ok, not equal: a failed equal on two elements prints the whole fake page.
  assert.ok(page.doc.activeElement === box, 'focus moves into the box so it is read out');
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0, 'Disagree records nothing');
  assert.equal(byId(page, 'welcome-reread').hidden, false);
  assert.equal(byId(page, 'welcome-logout').hidden, false);
  // Amended 2026-10-29 by the Phase 5 review fix (WR-B05): the fake box now holds
  // the Try the demo link of the template too, so three children, not two.
  assert.equal(box.children.length, 3, 'the script left the box markup alone');
});

test('Agree hides the Disagree box, and a step change puts it away', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  const box = byId(page, 'welcome-disagree-message');
  click(page, 'welcome-disagree');
  assert.equal(box.hidden, false);

  click(page, 'welcome-agree');
  assert.equal(box.hidden, true, 'Agree hides the box at once');
  answer(page.requests[0], 204);
  await flush();
  assert.equal(visibleStep(page), 3);
  assert.equal(box.hidden, true);

  // Back to step 2 and Disagree again, then leave by the Back button.
  page.api.goTo(2);
  click(page, 'welcome-disagree');
  assert.equal(box.hidden, false);
  click(page, 'welcome-back-2');
  assert.equal(visibleStep(page), 1);
  assert.equal(box.hidden, true, 'moving to another step puts the box away');
});

test('Read again scrolls the text box back to the top, focuses it, and sends nothing', () => {
  const page = load();
  click(page, 'welcome-continue-1');
  const scrollbox = byId(page, 'welcome-scrollbox');
  scrollbox.scrollTop = 900;
  click(page, 'welcome-disagree');
  click(page, 'welcome-reread');

  assert.equal(scrollbox.scrollTop, 0);
  assert.ok(page.doc.activeElement === scrollbox, 'focus returns to the text box');
  assert.equal(page.requests.length, 0);
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-disagree-message').hidden, false, 'the box stays until the person agrees or leaves');
});

test('Log out is the existing log out form: the script puts no handler on it and sends nothing', () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-disagree');
  const logout = byId(page, 'welcome-logout');
  assert.equal(logout.handlers.click, undefined, 'a plain submit button of the POST /auth/logout form (pinned in welcome_contract_test.go)');
  const event = fire(logout, 'click');
  assert.equal(event.prevented, false, 'nothing stops the form from posting');
  assert.equal(page.requests.length, 0, 'the script sends no request of its own');
  assert.equal(page.sandbox.window.location.search, '');
});

test('Agree sends POST /api/me/consent once, and moves to step 3 only on a 204', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-agree');
  click(page, 'welcome-agree');

  assert.equal(page.requests.length, 1, 'a second tap while saving must not send again');
  assert.equal(page.requests[0].url, '/api/me/consent');
  assert.equal(page.requests[0].init.method, 'POST');
  assert.equal(visibleStep(page), 2, 'no move before the answer');
  assert.equal(byId(page, 'welcome-agree').disabled, true);

  answer(page.requests[0], 204);
  await flush();
  assert.equal(visibleStep(page), 3);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-3-title'));
  assert.equal(byId(page, 'welcome-agree').disabled, false);
});

test('a refused or failed Agree says so plainly, stays on step 2 and can be tried again', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-agree');
  answer(page.requests[0], 500, {});
  await flush();

  const error = byId(page, 'welcome-error-2');
  assert.equal(visibleStep(page), 2);
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, 'Could not save your agreement. Try again in a moment.');
  assert.equal(byId(page, 'welcome-agree').disabled, false);

  click(page, 'welcome-agree');
  assert.equal(error.hidden, true, 'a new try clears the old error');
  page.requests[1].reject(new Error('offline'));
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(error.textContent, 'Could not save your agreement. Try again in a moment.');

  click(page, 'welcome-agree');
  answer(page.requests[2], 401, {});
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(error.textContent, 'Your session has ended. Reload the page and log in again.');
});

test('Allow location asks the browser once, keeps the fix only through PinalertPosition, and moves on', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-allow-location');
  click(page, 'welcome-allow-location');
  assert.equal(page.positionCalls.length, 1, 'the browser is asked once');
  assert.equal(visibleStep(page), 3, 'no move before the browser answers');

  page.positionCalls[0].ok({ coords: { latitude: 12.97, longitude: 77.59 } });
  await flush();
  assert.deepEqual(page.saved, [[12.97, 77.59]]);
  assert.equal(visibleStep(page), 4);
  assert.equal(page.requests.length, 0, 'the fix is sent nowhere');
  assert.equal(byId(page, 'welcome-home-input').value, '', 'the device fix never fills the home area');
});

test('plan 36: Allow location marks the browser question as asked, Not now does not', async () => {
  const allow = load({ start: 3 });
  assert.equal(allow.asked.length, 0);
  click(allow, 'welcome-allow-location');
  assert.equal(allow.asked.length, 1, 'marked when the question is asked');
  allow.positionCalls[0].fail({ code: 1 });
  await flush();
  assert.equal(allow.asked.length, 1, 'a refusal is still one question asked');

  const notNow = load({ start: 3 });
  click(notNow, 'welcome-not-now');
  assert.equal(notNow.asked.length, 0, 'nothing was asked, so nothing is marked');
});

test('plan 36: a failing marker does not stop Allow location', async () => {
  const page = load({ start: 3 });
  page.sandbox.PinalertPosition.markAsked = () => { throw new Error('blocked'); };
  click(page, 'welcome-allow-location');
  assert.equal(page.positionCalls.length, 1);
  page.positionCalls[0].ok({ coords: { latitude: 12.97, longitude: 77.59 } });
  await flush();
  assert.equal(visibleStep(page), 4);
});

test('a refused Allow location moves on and saves nothing', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-allow-location');
  page.positionCalls[0].fail({ code: 1 });
  await flush();
  assert.deepEqual(page.saved, []);
  assert.equal(visibleStep(page), 4);
  assert.equal(page.requests.length, 0);
});

// ---- Plan 33: step 3 is only Allow location or Not now ---------------------

test('Not now goes to step 4 without asking the browser, sending or saving anything', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-not-now');
  await flush();
  assert.equal(visibleStep(page), 4);
  assert.equal(page.positionCalls.length, 0, 'the browser is not asked');
  assert.equal(page.requests.length, 0, 'nothing is sent, and no place search');
  assert.deepEqual(page.saved, [], 'no position is kept');
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-4-title'), 'focus moves to the step title');
});

test('step 3 never fills or empties step 4: the home area is left as the page drew it', async () => {
  const empty = load({ start: 3 });
  click(empty, 'welcome-not-now');
  assert.equal(byId(empty, 'welcome-home-input').value, '');

  // Amended 2026-10-29 by Phase 7.1 plan 13: Continue needs all three, so the
  // stored page holds a username and a name too.
  const stored = load({ start: 3, ...STORED });
  click(stored, 'welcome-not-now');
  assert.equal(byId(stored, 'welcome-home-input').value, 'Whitefield', 'a stored home area stays');
  // Amended 2026-10-27 by quick 261027-t5w: step 4's Continue opens the alerts
  // pop-up; first-run-done is sent by its Not now.
  click(stored, 'welcome-continue-4');
  assert.equal(stored.requests.length, 0);
  assert.equal(visibleStep(stored), 4);
  assert.equal(stored.dialog.open, true);
  click(stored, 'welcome-alerts-later');
  assert.equal(stored.requests.length, 1);
  assert.equal(stored.requests[0].url, '/api/me/first-run-done', 'a stored home area is not sent again');

  const viaAllow = load({ start: 3 });
  click(viaAllow, 'welcome-allow-location');
  viaAllow.positionCalls[0].ok({ coords: { latitude: 12.97, longitude: 77.59 } });
  await flush();
  assert.equal(byId(viaAllow, 'welcome-home-input').value, '');
});

test('Allow location goes to step 4 when the browser has no location API', () => {
  const page = load({ start: 3 });
  delete page.sandbox.navigator.geolocation;
  click(page, 'welcome-allow-location');
  assert.equal(visibleStep(page), 4);
  assert.equal(page.requests.length, 0);
  assert.deepEqual(page.saved, []);
});

test('Allow location goes to step 4 when the browser times out or gives a bad answer', async () => {
  const timedOut = load({ start: 3 });
  click(timedOut, 'welcome-allow-location');
  timedOut.positionCalls[0].fail({ code: 3 });
  assert.equal(visibleStep(timedOut), 4);

  const odd = load({ start: 3 });
  click(odd, 'welcome-allow-location');
  odd.positionCalls[0].ok({});
  assert.equal(visibleStep(odd), 4);
  assert.deepEqual(odd.saved, []);
  assert.equal(odd.requests.length, 0);
});

test('Not now is not a way past the agreement: finishing still needs Agree', async () => {
  const page = load({ start: 1 });
  fire(listButton(page, 3), 'click');
  click(page, 'welcome-not-now');
  // Amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip; Continue is held
  // to the same rule.
  click(page, 'welcome-continue-4');
  assert.equal(visibleStep(page), 2, 'Continue before Agree shows step 2');
  assert.equal(page.requests.length, 0);
});

// Amended 2026-10-29 by Phase 7.1 plan 13: the page holds a username and a home
// area already, so the one changed field is the name.
test('step 4 Continue sends only the filled fields, then first-run-done, then lands on / with the search flag', async () => {
  const page = load({ start: 4, username: 'ravi.k', home: 'Whitefield' });
  byId(page, 'welcome-name-input').value = '  Ravi Kumar  ';
  fire(byId(page, 'welcome-name-input'), 'input');
  click(page, 'welcome-continue-4');
  click(page, 'welcome-continue-4');

  assert.equal(page.requests.length, 1, 'a second tap while saving must not send again');
  assert.equal(page.requests[0].url, '/api/me/profile');
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.equal(page.requests[0].init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(page.requests[0].init.body), { display_name: 'Ravi Kumar' });
  answer(page.requests[0], 200, { display_name: 'Ravi Kumar', home_area_name: null });
  await flush();

  // Amended 2026-10-27 by quick 261027-t5w: the saved profile opens the alerts
  // pop-up, whose Not now sends first-run-done.
  assert.equal(page.requests.length, 1, 'first-run-done is not sent before the pop-up is answered');
  assert.equal(visibleStep(page), 4);
  assert.equal(page.dialog.open, true);
  click(page, 'welcome-alerts-later');
  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].url, '/api/me/first-run-done');
  assert.equal(page.requests[1].init.method, 'POST');
  assert.deepEqual(page.replaced, [], 'no move before first-run-done answers');
  answer(page.requests[1], 204);
  await flush();

  assert.equal(page.session['pinalert.focusSearch'], '1');
  assert.deepEqual(page.replaced, ['/']);
});

// Amended 2026-10-27 by quick 261027-t5w: both ways on from step 4 open the
// alerts pop-up, whose Not now sends first-run-done.
// Amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip and no empty Continue.
// An account that already holds all three presses Continue and nothing is saved;
// a Continue pressed while a field is missing sends nothing and guides.
test('with the username, name and home area all stored there is no profile request, and Continue sends only first-run-done', async () => {
  const stored = load({ start: 4, ...STORED });
  assert.equal(byId(stored, 'welcome-continue-4').getAttribute('aria-disabled'), null, 'Continue is on');
  click(stored, 'welcome-continue-4');
  assert.equal(stored.requests.length, 0);
  assert.equal(stored.dialog.open, true);
  click(stored, 'welcome-alerts-later');
  assert.equal(stored.requests.length, 1);
  assert.equal(stored.requests[0].url, '/api/me/first-run-done');
  answer(stored.requests[0], 204);
  await flush();
  assert.deepEqual(stored.replaced, ['/']);
  assert.equal(stored.session['pinalert.focusSearch'], '1');
});

test('Continue pressed while a field is missing sends nothing, opens nothing and says what to add', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-name-input').value = 'Typed but not enough';
  fire(byId(page, 'welcome-name-input'), 'input');
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 0);
  assert.equal(page.dialog.open, false);
  assert.equal(visibleStep(page), 4);
  assert.equal(byId(page, 'welcome-error-4').textContent, 'Add your username, name and home area.');
  assert.equal(byId(page, 'welcome-error-4').hidden, false);
});

test('a stored name and home area left alone are not sent again', async () => {
  const page = load({ start: 4, ...STORED });
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 0);
  click(page, 'welcome-alerts-later');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/me/first-run-done');
});

// Amended 2026-10-29 by Phase 7.1 plan 13: the account holds a username and a
// name, so the home area is the one thing picked here.
test('the home area sent is the picked place with its own point, never the device fix, and typed text alone is not sent', async () => {
  const page = load({ start: 3, username: STORED.username, name: STORED.name });
  click(page, 'welcome-allow-location');
  page.positionCalls[0].ok({ coords: { latitude: 1.5, longitude: 2.5 } });
  await flush();
  assert.equal(visibleStep(page), 4);

  const home = byId(page, 'welcome-home-input');
  home.value = 'Whitefield';
  fire(byId(page, 'welcome-home-form'), 'submit');
  assert.equal(page.requests[0].url, '/api/geocode/place?kind=area&q=Whitefield');
  answer(page.requests[0], 200, { results: [{ name: 'Whitefield', display_name: 'Whitefield, Bengaluru', lat: 12.9698, lon: 77.7499 }] });
  await flush();
  fire(byId(page, 'welcome-home-results').children[0].children[0], 'click');
  assert.equal(home.value, 'Whitefield');

  click(page, 'welcome-continue-4');
  assert.equal(page.requests[1].url, '/api/me/profile');
  const body = JSON.parse(page.requests[1].init.body);
  assert.deepEqual(body, { home_area: { name: 'Whitefield', lat: 12.9698, lon: 77.7499 } });
  assert.equal(JSON.stringify(page.requests.map((r) => r.init.body)).includes('1.5'), false, 'the device fix is in no request');

  // Amended 2026-10-29 by Phase 7.1 plan 13: text that was never picked is not a
  // home area, so Continue stays off and sends nothing at all.
  const typed = load({ start: 4, username: STORED.username, name: STORED.name });
  byId(typed, 'welcome-home-input').value = 'Somewhere I typed';
  fire(byId(typed, 'welcome-home-input'), 'input');
  click(typed, 'welcome-continue-4');
  assert.equal(typed.requests.length, 0, 'text that was never picked from a search is not a home area');
  assert.equal(typed.dialog.open, false);
  assert.equal(byId(typed, 'welcome-error-4').textContent, 'Add your username, name and home area.');
});

test('a refused profile keeps step 4 with the server sentence and does not finish', async () => {
  const page = load({ start: 4, username: 'ravi.k', home: 'Whitefield' });
  byId(page, 'welcome-name-input').value = 'Ravi';
  fire(byId(page, 'welcome-name-input'), 'input');
  click(page, 'welcome-continue-4');
  answer(page.requests[0], 400, { error: { field: 'display_name', message: 'Use letters, numbers and spaces in your name.' } });
  await flush();

  const error = byId(page, 'welcome-error-4');
  assert.equal(visibleStep(page), 4);
  assert.equal(error.hidden, false);
  assert.equal(error.textContent, 'Use letters, numbers and spaces in your name.');
  assert.equal(page.requests.length, 1, 'first-run-done is not sent after a refused profile');
  assert.deepEqual(page.replaced, []);

  click(page, 'welcome-continue-4');
  assert.equal(error.hidden, true);
  page.requests[1].reject(new Error('offline'));
  await flush();
  assert.equal(error.textContent, 'Could not save. Try again in a moment.');
  assert.equal(page.requests.length, 2);
  assert.deepEqual(page.replaced, []);
});

// Amended 2026-10-29 by Phase 7.1 plan 13: Skip is gone, so the flow is finished
// with fillStep4 and Continue.
test('first-run-done refused for missing consent sends the person back to step 2 and does not leave', async () => {
  const page = load({ start: 4 });
  await fillStep4(page);
  click(page, 'welcome-continue-4');
  answer(page.requests[0], 200);
  await flush();
  click(page, 'welcome-alerts-later');
  answer(page.requests[1], 409, { error: { field: 'terms', message: 'Agree first.' } });
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-error-2').textContent, 'Please agree to the Disclaimer and policy to continue.');
  assert.deepEqual(page.replaced, []);
  assert.equal(page.session['pinalert.focusSearch'], undefined);
});

// Phase 7.1 plan 13: first-run-done answers 409 with field profile while the
// username, the name or the home area is still missing on the server. That is not
// the agreement: the pop-up is put away and the person is back on step 4.
test('a 409 with field profile from first-run-done closes the pop-up and returns to step 4 with its sentence', async () => {
  const page = load({ start: 4, ...STORED });
  click(page, 'welcome-continue-4');
  assert.equal(page.dialog.open, true);
  click(page, 'welcome-alerts-later');
  answer(page.requests[0], 409, { error: { field: 'profile', message: 'Add your username, name and home area.' } });
  await flush();
  assert.equal(page.dialog.open, false, 'the pop-up is closed');
  assert.equal(visibleStep(page), 4, 'not step 2');
  const line = byId(page, 'welcome-error-4');
  assert.equal(line.hidden, false);
  assert.equal(line.textContent, 'Add your username, name and home area.');
  assert.equal(byId(page, 'welcome-error-2').hidden, true, 'no agree line');
  assert.deepEqual(page.replaced, []);
  assert.equal(byId(page, 'welcome-continue-4').disabled, false, 'Continue is free again');
});

test('a 409 with any other field still goes back to step 2 with the agree line', async () => {
  const page = load({ start: 4, ...STORED });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-later');
  answer(page.requests[0], 409, { error: { field: 'terms', message: 'Agree first.' } });
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-error-2').textContent, 'Please agree to the Disclaimer and policy to continue.');
});

test('Escape does nothing and Tab stays inside the current step', () => {
  const page = load();
  click(page, 'welcome-continue-1');
  assert.equal(visibleStep(page), 2);

  const esc = fire(page.root, 'keydown', { key: 'Escape' });
  assert.equal(visibleStep(page), 2);
  assert.equal(page.root.hidden, false);
  assert.equal(esc.prevented, false, 'Escape is left alone');

  // Laptop: the step list buttons come first in the dialog.
  const box = byId(page, 'welcome-scrollbox');
  const agree = byId(page, 'welcome-agree');
  const firstListButton = page.items[0].button;
  agree.focus();
  const forward = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.equal(page.doc.activeElement, firstListButton);

  firstListButton.focus();
  const back = fire(page.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.equal(page.doc.activeElement, agree);

  byId(page, 'welcome-step-2-title').focus();
  const fromTitle = fire(page.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(fromTitle.prevented, false, 'the list and the Back control come before the title, so the browser moves back to them');

  byId(page, 'welcome-disagree').focus();
  const middle = fire(page.root, 'keydown', { key: 'Tab' });
  // Amended 2026-10-29 by the Phase 5 review fix (WR-B05): Tab between two
  // controls is no longer left to the browser (Safari's Tab skips links, so a
  // link in the loop was passed over); the page moves to the next control itself.
  assert.equal(middle.prevented, true, 'the page moves focus between two controls');
  assert.ok(page.doc.activeElement === agree, 'Tab from Disagree lands on Agree, the next control');

  // Phone: the side panel is not drawn, so its buttons are not in the Tab loop
  // and Back is the first control of the step.
  const phone = load();
  phone.side.hiddenByCss = true;
  click(phone, 'welcome-continue-1');
  const phoneBack = byId(phone, 'welcome-back-2');
  const phoneAgree = byId(phone, 'welcome-agree');
  phoneAgree.focus();
  const wrapForward = fire(phone.root, 'keydown', { key: 'Tab' });
  assert.equal(wrapForward.prevented, true);
  assert.equal(phone.doc.activeElement, phoneBack, 'Tab from the last control lands on Back, never on a hidden list button');
  phoneBack.focus();
  const wrapBack = fire(phone.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(wrapBack.prevented, true);
  assert.equal(phone.doc.activeElement, phoneAgree);
  box.focus();
  // Amended 2026-10-29 by the Phase 5 review fix (WR-B05): focus here is still
  // on Agree of the phone page (box belongs to the laptop page), and Shift+Tab
  // from a control in the loop is now moved by the page, not left to the browser.
  assert.equal(fire(phone.root, 'keydown', { key: 'Tab', shiftKey: true }).prevented, true);
  assert.ok(phone.doc.activeElement === byId(phone, 'welcome-disagree'), 'Shift+Tab from Agree lands on Disagree');

  // Step 1 on the phone has nothing before the title, so Shift+Tab from it wraps.
  const first = load();
  first.side.hiddenByCss = true;
  byId(first, 'welcome-step-1-title').focus();
  const fromFirstTitle = fire(first.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(fromFirstTitle.prevented, true);
  assert.equal(first.doc.activeElement, byId(first, 'welcome-continue-1'));
});

test('the public hooks move the flow the same way the buttons do', async () => {
  const page = load();
  page.api.goTo(2);
  assert.equal(visibleStep(page), 2);
  page.api.disagree();
  assert.equal(byId(page, 'welcome-disagree-message').hidden, false);
  page.api.goTo(9);
  assert.equal(visibleStep(page), 2, 'a step that does not exist is refused');
  assert.equal(typeof page.api.agree, 'function');
  assert.equal(typeof page.api.allowLocation, 'function');
  assert.equal(typeof page.api.finish, 'function');
});

// ---- Plan 28: suggestions while typing in the home area (step 4) -----------

const typeHome = (page, text) => {
  const input = byId(page, 'welcome-home-input');
  input.value = text;
  fire(input, 'input');
};
const homeStatus = (page) => byId(page, 'welcome-home-status');
const homeList = (page) => byId(page, 'welcome-home-results');
const AREA_URL = '/api/geocode/place?kind=area&q=';

test('typing in the home area sends nothing under 3 characters and shows no line', () => {
  const page = load({ start: 4 });
  typeHome(page, 'In');
  page.advance(5000);
  assert.equal(page.requests.length, 0);
  assert.equal(homeStatus(page).hidden, true, 'no too short line comes from typing');
  assert.equal(homeList(page).hidden, true);
});

test('one request goes out after the typing pause, in areas mode, with no Searching line', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(399);
  assert.equal(page.requests.length, 0, 'the pause is 400 ms');
  page.advance(1);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, AREA_URL + 'Indira');
  assert.equal(homeStatus(page).hidden, true, 'a typed search is quiet');
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  assert.equal(homeList(page).hidden, false);
  assert.equal(homeList(page).children.length, 2);
  assert.equal(homeList(page).children[0].children[0].getAttribute('role'), 'option');
});

test('at most 5 suggestions show', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(400);
  const many = [];
  for (let i = 0; i < 8; i++) { many.push({ name: 'Area ' + i, display_name: 'Area ' + i + ', Karnataka', lat: 12 + i / 100, lon: 77 }); }
  answer(page.requests[0], 200, { results: many });
  await flush();
  assert.equal(homeList(page).children.length, 5);
});

test('every keystroke restarts the pause', () => {
  const page = load({ start: 4 });
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
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  typeHome(page, 'Indira');
  page.advance(5000);
  assert.equal(page.requests.length, 1);
  typeHome(page, 'Indira ');
  page.advance(5000);
  assert.equal(page.requests.length, 1, 'a trailing space is the same text');
});

test('a request still out is cancelled by a newer one and its late answer is ignored', async () => {
  const page = load({ start: 4 });
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
  answer(page.requests[1], 200, { results: [PLACES[0]] });
  await flush();
  answer(first, 200, { results: PLACES });
  await flush();
  assert.equal(homeList(page).children.length, 1, 'the stale answer did not replace the newer list');
});

test('Enter searches at once, shows Searching, and the pause that was running sends nothing more', () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(100);
  fire(byId(page, 'welcome-home-form'), 'submit');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, AREA_URL + 'Indira');
  assert.equal(homeStatus(page).textContent, 'Searching…');
  page.advance(5000);
  assert.equal(page.requests.length, 1);
});

test('Enter for the text a typed search already sent takes it over and shows the lines', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(400);
  assert.equal(homeStatus(page).hidden, true);
  fire(byId(page, 'welcome-home-form'), 'submit');
  assert.equal(page.requests.length, 1, 'no second request for the same text');
  assert.equal(homeStatus(page).textContent, 'Searching…');
  answer(page.requests[0], 503, {});
  await flush();
  assert.equal(homeStatus(page).textContent, 'Search unavailable. Try again in a moment.');
});

test('a busy typed search is retried quietly and only the last failure shows its line', async () => {
  const page = load({ start: 4 });
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
  const page = load({ start: 4 });
  typeHome(page, 'Bengal');
  page.advance(400);
  answer(page.requests[0], 503, {});
  await flush();
  typeHome(page, 'Bengalu');
  page.advance(5000);
  assert.deepEqual(page.requests.map((r) => r.url), [AREA_URL + 'Bengal', AREA_URL + 'Bengalu']);
});

test('a network failure while typing stays quiet and is not tried again', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Bengal');
  page.advance(400);
  page.requests[0].reject(new Error('offline'));
  await flush();
  page.advance(5000);
  assert.equal(homeStatus(page).hidden, true);
  assert.equal(page.requests.length, 1);
});

test('typing under 3 characters again drops the list and any search out', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  assert.equal(homeList(page).hidden, false);
  typeHome(page, 'In');
  assert.equal(homeList(page).hidden, true);
  assert.equal(homeStatus(page).hidden, true);
  typeHome(page, 'Indira');
  page.advance(1000);
  assert.equal(page.requests.length, 2, 'the text may be sent again once it was dropped');
});

test('a 12 character name typed in one go sends one request; typed slowly it stays under one a second', () => {
  const name = 'Banjara Hill';
  assert.equal(Array.from(name).length, 12);
  const quick = load({ start: 4 });
  for (let i = 1; i <= 12; i++) {
    typeHome(quick, name.slice(0, i));
    quick.advance(100);
  }
  quick.advance(400);
  assert.equal(quick.requests.length, 1);
  assert.equal(quick.requests[0].url, AREA_URL + encodeURIComponent(name));

  const slow = load({ start: 4 });
  for (let i = 1; i <= 12; i++) {
    typeHome(slow, name.slice(0, i));
    slow.advance(450);
  }
  slow.advance(2000);
  const times = slow.requests.map((r) => r.at);
  assert.ok(times.length >= 2 && times.length <= 7, 'requests sent: ' + times.length);
  for (let i = 1; i < times.length; i++) {
    assert.ok(times[i] - times[i - 1] >= 1000, 'typed searches are at least a second apart: ' + times.join(','));
  }
});

test('a pick ends the typing pause and a late answer does not reopen the list', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  typeHome(page, 'Indiran');
  fire(homeList(page).children[0].children[0], 'click');
  assert.equal(byId(page, 'welcome-home-input').value, 'Indiranagar');
  page.advance(5000);
  assert.equal(page.requests.length, 1, 'the pause that was running sent nothing');
});

// ---- Plan 30: moving back and forth between the four steps ------------------

const listButton = (page, n) => page.items[n - 1].button;
const AGREE_FIRST = 'Please agree to the Disclaimer and policy to continue.';
const stepStates = (page) => page.items.map((i) => i.getAttribute('data-state'));
const currentMarks = (page) => page.items.map((i) => i.button.getAttribute('aria-current'));

test('Back on steps 2, 3 and 4 shows the previous step and moves focus to its title', () => {
  const page = load({ start: 4 });
  click(page, 'welcome-back-4');
  assert.equal(visibleStep(page), 3);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-3-title'));
  click(page, 'welcome-back-3');
  assert.equal(visibleStep(page), 2);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-2-title'));
  click(page, 'welcome-back-2');
  assert.equal(visibleStep(page), 1);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-1-title'));
  assert.equal(page.root.getAttribute('data-step'), '1');
  assert.deepEqual(stepStates(page), ['current', 'waiting', 'waiting', 'waiting']);
  assert.equal(page.requests.length, 0, 'moving back sends nothing');
});

test('a click on any step list button shows that step, in any order, marked aria-current', () => {
  const page = load();
  for (const n of [4, 2, 3, 1, 4, 1]) {
    fire(listButton(page, n), 'click');
    assert.equal(visibleStep(page), n, 'step ' + n);
    assert.equal(page.root.getAttribute('data-step'), String(n));
    assert.equal(page.doc.activeElement, byId(page, 'welcome-step-' + n + '-title'));
    assert.deepEqual(currentMarks(page), [1, 2, 3, 4].map((k) => (k === n ? 'step' : null)));
    assert.deepEqual(stepStates(page), [1, 2, 3, 4].map((k) => (k < n ? 'done' : (k === n ? 'current' : 'waiting'))));
  }
  assert.equal(page.requests.length, 0, 'looking at a step records nothing');
});

test('Finish before Agree shows step 2 with a line asking to agree first, and sends nothing', () => {
  const page = load();
  fire(listButton(page, 4), 'click');
  assert.equal(visibleStep(page), 4);
  click(page, 'welcome-continue-4');
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0, 'no profile and no first-run-done without the agreement');
  const line = byId(page, 'welcome-error-2');
  assert.equal(line.hidden, false);
  assert.equal(line.textContent, AGREE_FIRST);
  assert.equal(page.replaced.length, 0, 'the person did not leave the flow');

  // The line goes when the person moves on, and a second press is held to the same
  // rule (amended 2026-10-29 by Phase 7.1 plan 13: there is no Skip).
  fire(listButton(page, 4), 'click');
  assert.equal(line.hidden, true);
  click(page, 'welcome-continue-4');
  assert.equal(visibleStep(page), 2);
  assert.equal(line.textContent, AGREE_FIRST);
  assert.equal(page.requests.length, 0);

  // The public hook is held to it too.
  page.api.goTo(4);
  page.api.finish();
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0);
});

// Amended 2026-10-29 by Phase 7.1 plan 13: Skip is gone; the three fields are
// filled with fillStep4 and Continue saves them before the pop-up.
test('after Agree, Finish works from step 4 even when the person came back to step 2 first', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-agree');
  answer(page.requests[0], 204);
  await flush();
  click(page, 'welcome-back-3');
  assert.equal(visibleStep(page), 2);
  fire(listButton(page, 4), 'click');
  await fillStep4(page);
  click(page, 'welcome-continue-4');
  answer(page.requests[1], 200);
  await flush();
  click(page, 'welcome-alerts-later');
  assert.equal(page.requests.length, 3);
  assert.equal(page.requests[2].url, '/api/me/first-run-done');
  answer(page.requests[2], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('the server consent rule still holds: a 409 from first-run-done sends the person back to step 2', async () => {
  const page = load({ start: 4, ...STORED });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-alerts-later');
  answer(page.requests[0], 409, { error: { message: 'agree first' } });
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-error-2').textContent, AGREE_FIRST);
  assert.equal(page.replaced.length, 0);
});

test('each move pushes a history entry with the step in the address; the first load only replaces', () => {
  const page = load();
  assert.equal(page.pushes.length, 0, 'loading pushes nothing');
  assert.equal(page.stateReplaces.length, 1);
  assert.equal(page.stateReplaces[0].address, '?step=1');
  assert.equal(page.stateReplaces[0].state.pinalertStep, 1);

  click(page, 'welcome-continue-1');
  fire(listButton(page, 4), 'click');
  click(page, 'welcome-back-4');
  assert.deepEqual(page.pushes.map((p) => p.address), ['?step=2', '?step=4', '?step=3']);
  assert.deepEqual(page.pushes.map((p) => p.state.pinalertStep), [2, 4, 3]);

  // Showing the step already shown adds nothing.
  fire(listButton(page, 3), 'click');
  assert.equal(page.pushes.length, 3);
});

test('the browser back and forward (popstate) show the step in the entry and push nothing', () => {
  const page = load();
  click(page, 'welcome-continue-1');
  fire(listButton(page, 3), 'click');
  const pushed = page.pushes.length;

  page.popTo({ pinalertStep: 2 });
  assert.equal(visibleStep(page), 2);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-step-2-title'));
  assert.deepEqual(stepStates(page), ['done', 'current', 'waiting', 'waiting']);
  page.popTo({ pinalertStep: 1 });
  assert.equal(visibleStep(page), 1);
  page.popTo({ pinalertStep: 3 });
  assert.equal(visibleStep(page), 3);
  assert.equal(page.pushes.length, pushed, 'going through history does not add to it');

  // An entry with no step of ours, or a step that does not exist, changes nothing.
  page.popTo(null);
  page.popTo({ pinalertStep: 9 });
  assert.equal(visibleStep(page), 3);
});

test('an address step is honoured only when the account may see it', () => {
  // Has not agreed (the server starts at step 1): steps 1 and 2 only.
  assert.equal(visibleStep(load({ search: '?step=2' })), 2);
  assert.equal(visibleStep(load({ search: '?step=1' })), 1);
  for (const search of ['?step=3', '?step=4', '?step=9', '?step=0', '?step=abc', '?step=', '?x=1', '']) {
    assert.equal(visibleStep(load({ search })), 1, 'unagreed ' + search);
  }
  assert.equal(load({ search: '?step=4' }).requests.length, 0);

  // Has agreed (the server starts at step 3): any of the four, never past them.
  assert.equal(visibleStep(load({ start: 3, search: '?step=1' })), 1);
  assert.equal(visibleStep(load({ start: 3, search: '?step=2' })), 2);
  assert.equal(visibleStep(load({ start: 3, search: '?step=4' })), 4);
  assert.equal(visibleStep(load({ start: 3, search: '?a=1&step=4' })), 4);
  assert.equal(visibleStep(load({ start: 3, search: '?step=9' })), 3);
  // Quick 261027-t5w: there is no step 5 any more, so its address falls back.
  assert.equal(visibleStep(load({ start: 3, search: '?step=5' })), 3);
  assert.equal(visibleStep(load({ start: 4, search: '?step=5' })), 4);
  assert.equal(visibleStep(load({ start: 3, search: '' })), 3);
  const loaded = load({ start: 3, search: '?step=4' });
  assert.equal(loaded.stateReplaces[0].address, '?step=4', 'the entry names the step actually shown');
});

// Amended 2026-10-29 by Phase 7.1 plan 13: the account holds a username, so the
// name and the home area are what this walk fills and sends.
test('what was typed or picked is kept while moving between steps', async () => {
  const page = load({ username: STORED.username });
  fire(listButton(page, 3), 'click');
  fire(listButton(page, 4), 'click');
  assert.equal(byId(page, 'welcome-home-input').value, '', 'step 3 leaves nothing in the home area');
  byId(page, 'welcome-name-input').value = 'Asha';
  fire(byId(page, 'welcome-name-input'), 'input');
  typeHome(page, 'Indira');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  fire(homeList(page).children[0].children[0], 'click');
  assert.equal(byId(page, 'welcome-home-input').value, 'Indiranagar');
  // Owner, 2026-10-18: the suggestions go away once one is chosen.
  assert.equal(homeList(page).hidden, true, 'a pick closes the list');
  assert.equal(page.doc.activeElement, byId(page, 'welcome-home-input'), 'focus goes back to the field');

  // Away to step 3 and step 1 and back: nothing is lost or re-filled.
  click(page, 'welcome-back-4');
  assert.equal(visibleStep(page), 3);
  fire(listButton(page, 1), 'click');
  page.popTo({ pinalertStep: 4 });
  assert.equal(visibleStep(page), 4);
  assert.equal(byId(page, 'welcome-name-input').value, 'Asha');
  assert.equal(byId(page, 'welcome-home-input').value, 'Indiranagar', 'the picked home area is still there');

  // Agreement then finishing sends exactly what was picked.
  fire(listButton(page, 2), 'click');
  click(page, 'welcome-agree');
  const consent = page.requests[page.requests.length - 1];
  assert.equal(consent.url, '/api/me/consent');
  answer(consent, 204);
  await flush();
  fire(listButton(page, 4), 'click');
  click(page, 'welcome-continue-4');
  const patch = page.requests[page.requests.length - 1];
  assert.equal(patch.init.method, 'PATCH');
  assert.deepEqual(JSON.parse(patch.init.body), {
    display_name: 'Asha',
    home_area: { name: 'Indiranagar', lat: 12.9784, lon: 77.6408 }
  });
});

// ---- Plan 32: the typed row of the home area (step 4) ----------------------
// OpenStreetMap lacks many neighbourhoods as areas, so the server may start the
// list with a row that is the person's own text placed in the town they typed.

const TYPED_ROW = { name: 'Gayathri Nagar, Vijayawada', display_name: 'Vijayawada', town: 'Vijayawada', typed: true, lat: 16.5115306, lon: 80.6160469 };
const OWNER_TEXT = 'Gayathri Nagar, Vijayawada, Andhra Pradesh, 520008';

test('the typed row shows first as Use "name" with the town on the second line', async () => {
  const page = load({ start: 4 });
  typeHome(page, OWNER_TEXT);
  page.advance(400);
  assert.equal(page.requests[0].url, AREA_URL + encodeURIComponent(OWNER_TEXT), 'the browser sends the text as typed; the server cleans it');
  answer(page.requests[0], 200, { results: [TYPED_ROW, ...PLACES] });
  await flush();
  const list = homeList(page);
  assert.equal(list.hidden, false);
  assert.equal(list.children.length, 3);
  const first = list.children[0].children[0];
  assert.equal(first.getAttribute('role'), 'option');
  assert.equal(first.children[0].textContent, 'Use "Gayathri Nagar, Vijayawada"');
  assert.equal(first.children[1].textContent, 'Vijayawada', 'the second line names the town it was placed in');
  assert.equal(list.children[1].children[0].children[0].textContent, 'Indiranagar', 'other rows are unchanged');
});

test('picking the typed row puts the cleaned name in the field and saves it with the row position', async () => {
  const page = load({ start: 4, username: STORED.username, name: STORED.name });
  typeHome(page, OWNER_TEXT);
  page.advance(400);
  answer(page.requests[0], 200, { results: [TYPED_ROW, ...PLACES] });
  await flush();
  fire(homeList(page).children[0].children[0], 'click');
  assert.equal(byId(page, 'welcome-home-input').value, 'Gayathri Nagar, Vijayawada', 'the field takes the cleaned name, not the Use wording');
  click(page, 'welcome-continue-4');
  assert.equal(page.requests[1].url, '/api/me/profile');
  assert.deepEqual(JSON.parse(page.requests[1].init.body), { home_area: { name: 'Gayathri Nagar, Vijayawada', lat: 16.5115306, lon: 80.6160469 } });
});

test('the typed row counts as one of the five rows shown', async () => {
  const page = load({ start: 4 });
  typeHome(page, OWNER_TEXT);
  page.advance(400);
  const many = [];
  for (let i = 0; i < 5; i++) { many.push({ name: 'Area ' + i, display_name: 'Area ' + i + ', Vijayawada', lat: 16.5 + i / 100, lon: 80.6 }); }
  answer(page.requests[0], 200, { results: [TYPED_ROW, ...many] });
  await flush();
  assert.equal(homeList(page).children.length, 5);
  assert.equal(homeList(page).children[0].children[0].children[0].textContent, 'Use "Gayathri Nagar, Vijayawada"');
});

test('only a row marked typed gets the Use wording, and the typed text reaches the page as text', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Indira');
  page.advance(400);
  const hostile = { name: 'Use <b>x</b>', display_name: 'Use <b>x</b>, Pune', lat: 18.5, lon: 73.8 };
  answer(page.requests[0], 200, { results: [hostile, { name: '<i>Typed</i>, Pune', display_name: 'Pune', town: 'Pune', typed: true, lat: 18.5, lon: 73.8 }] });
  await flush();
  const rows = homeList(page).children;
  assert.equal(rows[0].children[0].children[0].textContent, 'Use <b>x</b>', 'a row not marked typed has no Use wording');
  assert.equal(rows[1].children[0].children[0].textContent, 'Use "<i>Typed</i>, Pune"');
});

test('a typed row with no town falls back to the display name for its second line', async () => {
  const page = load({ start: 4 });
  typeHome(page, OWNER_TEXT);
  page.advance(400);
  answer(page.requests[0], 200, { results: [{ name: 'Gayathri Nagar, Vijayawada', display_name: 'Vijayawada', typed: true, lat: 16.5, lon: 80.6 }] });
  await flush();
  assert.equal(homeList(page).children[0].children[0].children[1].textContent, 'Vijayawada');
});

// ---- Plan 34: the not found line and the search URL (step 4) ---------------
// When no area matches closely and no typed row could be placed, the server
// answers no_match true (or no rows at all) and the status line asks for the
// town. The list stays where it is.

const NOT_FOUND = 'Not found. Add your town, for example: Gayathri Nagar, Vijayawada';

test('no_match shows the not found line and keeps the rows', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Benz Circle');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES, no_match: true });
  await flush();
  assert.equal(homeStatus(page).hidden, false);
  assert.equal(homeStatus(page).textContent, NOT_FOUND);
  assert.equal(homeList(page).hidden, false, 'the rows stay next to the line');
  assert.equal(homeList(page).children.length, 2);
});

test('an empty answer the server calls not found shows the not found line', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Zzqqxx');
  page.advance(400);
  answer(page.requests[0], 200, { results: [], no_match: true });
  await flush();
  assert.equal(homeStatus(page).textContent, NOT_FOUND);
  assert.equal(homeList(page).hidden, true);
});

// ---- Plan 35: an unfinished search never says not found, a pick hides the line

test('an empty answer without no_match shows no line for a typed search', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Gayathri Nagar Vijayawada');
  page.advance(400);
  answer(page.requests[0], 200, { results: [] });
  await flush();
  assert.equal(homeStatus(page).hidden, true, 'a lookup did not finish, so no Not found');
  assert.equal(homeList(page).hidden, true);
});

test('an empty answer without no_match shows Search unavailable for Enter, never Not found', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-home-input').value = 'Gayathri Nagar Vijayawada';
  fire(byId(page, 'welcome-home-form'), 'submit');
  answer(page.requests[0], 200, { results: [] });
  await flush();
  assert.equal(homeStatus(page).hidden, false);
  assert.equal(homeStatus(page).textContent, 'Search unavailable. Try again in a moment.');
  assert.notEqual(homeStatus(page).textContent, NOT_FOUND);
});

test('a typed search that did not finish clears a Not found line an earlier search left', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Zzqqxx');
  page.advance(400);
  answer(page.requests[0], 200, { results: [], no_match: true });
  await flush();
  assert.equal(homeStatus(page).textContent, NOT_FOUND);
  typeHome(page, 'Gayathri Nagar Vijayawada');
  page.advance(1000);
  assert.equal(page.requests.length, 2);
  answer(page.requests[1], 200, { results: [] });
  await flush();
  assert.equal(homeStatus(page).hidden, true, 'the old Not found line does not outlive the new, unfinished search');
});

test('rows with unfinished true show Search unavailable for Enter and keep the rows', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-home-input').value = 'Benz Circle';
  fire(byId(page, 'welcome-home-form'), 'submit');
  answer(page.requests[0], 200, { results: PLACES, unfinished: true });
  await flush();
  assert.equal(homeStatus(page).hidden, false);
  assert.equal(homeStatus(page).textContent, 'Search unavailable. Try again in a moment.');
  assert.equal(homeList(page).hidden, false, 'the rows stay listed');
});

test('rows with unfinished true show no line for a typed search, and unfinished is read only when true', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Benz Circle');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES, unfinished: true });
  await flush();
  assert.equal(homeStatus(page).hidden, true, 'a typed search stays quiet');
  assert.equal(homeList(page).hidden, false);
  for (const flag of ['true', 1, 'yes', null]) {
    const other = load({ start: 4 });
    byId(other, 'welcome-home-input').value = 'Benz Circle';
    fire(byId(other, 'welcome-home-form'), 'submit');
    answer(other.requests[0], 200, { results: PLACES, unfinished: flag });
    await flush();
    assert.equal(homeStatus(other).hidden, true, 'unfinished ' + JSON.stringify(flag) + ' is not true');
  }
});

test('picking a suggestion hides the status line as well as the list', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Benz Circle');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES, no_match: true });
  await flush();
  assert.equal(homeStatus(page).hidden, false, 'the Not found line is up beside the rows');
  fire(homeList(page).children[0].children[0], 'click');
  assert.equal(homeList(page).hidden, true, 'the list closes');
  assert.equal(homeStatus(page).hidden, true, 'the status line goes with it');
});

test('rows without no_match show no line, and the line goes when a later search matches', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Benz Circle');
  page.advance(400);
  answer(page.requests[0], 200, { results: PLACES, no_match: true });
  await flush();
  assert.equal(homeStatus(page).hidden, false);
  typeHome(page, 'Koramangala');
  page.advance(1000);
  assert.equal(page.requests.length, 2);
  answer(page.requests[1], 200, { results: PLACES });
  await flush();
  assert.equal(homeStatus(page).hidden, true, 'a matching search shows no line');
});

test('no_match is read only when it is true, and a typed row never goes with the line', async () => {
  for (const flag of ['true', 1, 'yes', null]) {
    const page = load({ start: 4 });
    typeHome(page, 'Benz Circle');
    page.advance(400);
    answer(page.requests[0], 200, { results: PLACES, no_match: flag });
    await flush();
    assert.equal(homeStatus(page).hidden, true, 'no_match ' + JSON.stringify(flag) + ' is not true');
  }
  const page = load({ start: 4 });
  typeHome(page, 'Benz Circle Vijayawada');
  page.advance(400);
  answer(page.requests[0], 200, { results: [TYPED_ROW, ...PLACES], no_match: true });
  await flush();
  assert.equal(homeStatus(page).hidden, true, 'a typed row is a match');
});

test('the not found line also shows for Enter, and the old no matches line is gone', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-home-input').value = 'Zzqqxx';
  fire(byId(page, 'welcome-home-form'), 'submit');
  answer(page.requests[0], 200, { results: [], no_match: true });
  await flush();
  assert.equal(homeStatus(page).textContent, NOT_FOUND);
  assert.ok(!/No matches found/.test(homeStatus(page).textContent));
});

test('the home area search sends only the text: no lat or lon, even with the page centre and a stored fix', () => {
  const page = load({ start: 4 });
  // A stored device fix and the configured fallback centre (Bengaluru) are both
  // there for the page to find; neither may reach the search.
  page.sandbox.PinalertPosition.read = () => ({ lat: 12.9716, lon: 77.5946, savedAt: 1 });
  page.sandbox.Pinalert = { config: { fallbackLat: 12.9716, fallbackLon: 77.5946 } };
  typeHome(page, 'Benz Circle Vijayawada');
  page.advance(400);
  assert.equal(page.requests[0].url, AREA_URL + encodeURIComponent('Benz Circle Vijayawada'));
  assert.ok(!/[?&](lat|lon)=/.test(page.requests[0].url), 'no near point: the fallback centre is never sent');
});

// ---- Quick 261027-jhd (owner, 2026-10-27): Agree and Disagree appear after reading
// Step 2 works like the terms screen of any other app: Agree and Disagree stay
// hidden until the end of the text in the text box has been in view, then they
// appear in the same place. welcome.js holds the row back (data-unread on
// #welcome-consent-actions) only when the reveal can run, and drops the hold for
// good at the first sight of the end marker. Visibility keeps the row's place
// in the layout; that is pinned in welcome_contract_test.go.

const heldBack = (page) => byId(page, 'welcome-consent-actions').hasAttribute('data-unread');

test('step 2 holds Agree and Disagree back until the end marker is seen, and Tab never walks out of the dialog meanwhile', () => {
  const page = load({ observer: true });
  assert.equal(heldBack(page), true, 'the row is held back before step 2 is ever shown');
  assert.equal(page.observers.length, 1, 'exactly one watcher');
  const watcher = page.observers[0];
  assert.ok(watcher.options.root === byId(page, 'welcome-scrollbox'), 'the text box is the root, so reading to the end with VoiceOver counts');
  assert.equal(watcher.options.rootMargin, '0px 0px 24px 0px');
  assert.equal(watcher.options.threshold, 0);
  assert.equal(watcher.targets.length, 1);
  assert.ok(watcher.targets[0] === byId(page, 'welcome-scrollbox-end'), 'the marker is the one target');

  click(page, 'welcome-continue-1');
  assert.equal(visibleStep(page), 2);
  assert.equal(heldBack(page), true, 'arriving at step 2 does not show the buttons');

  // Laptop: from the text box Tab wraps to the first control; Agree and Disagree are not in the loop.
  const box = byId(page, 'welcome-scrollbox');
  box.focus();
  const forward = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.ok(page.doc.activeElement === page.items[0].button, 'Tab wraps to the first step list button');
  const back = fire(page.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.ok(page.doc.activeElement === box, 'Shift+Tab from the first control lands on the text box');

  // Phone: the side panel is not drawn, so Back is the first control.
  const phone = load({ observer: true });
  phone.side.hiddenByCss = true;
  click(phone, 'welcome-continue-1');
  const phoneBox = byId(phone, 'welcome-scrollbox');
  phoneBox.focus();
  assert.equal(fire(phone.root, 'keydown', { key: 'Tab' }).prevented, true);
  assert.ok(phone.doc.activeElement === byId(phone, 'welcome-back-2'));
  assert.equal(fire(phone.root, 'keydown', { key: 'Tab', shiftKey: true }).prevented, true);
  assert.ok(phone.doc.activeElement === phoneBox);

  // An entry that is not in view changes nothing.
  seeEnd(page, false);
  assert.equal(heldBack(page), true);
  assert.equal(watcher.disconnected, false);

  // The end is seen: the row shows, the watcher is done, and Tab is left to the browser.
  seeEnd(page, true);
  assert.equal(heldBack(page), false);
  assert.equal(watcher.disconnected, true);
  box.focus();
  // Amended 2026-10-29 by the Phase 5 review fix (WR-B05): the page moves focus
  // itself now (it was left to the browser); the next control is still Disagree.
  assert.equal(fire(page.root, 'keydown', { key: 'Tab' }).prevented, true);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-disagree'), 'Tab from the text box now reaches Disagree');
  byId(page, 'welcome-agree').focus();
  assert.equal(fire(page.root, 'keydown', { key: 'Tab' }).prevented, true);
  assert.ok(page.doc.activeElement === page.items[0].button, 'Tab from Agree wraps to the first control as before');
  assert.equal(page.requests.length, 0, 'reading sends nothing');
});

test('a text that fits shows Agree and Disagree at once', () => {
  const page = load({ observer: true });
  // The first report a browser makes already has the marker in view.
  seeEnd(page, true);
  assert.equal(heldBack(page), false);
  assert.equal(page.observers[0].disconnected, true);
  click(page, 'welcome-continue-1');
  assert.equal(heldBack(page), false);
  assert.equal(page.requests.length, 0);
});

test('with no IntersectionObserver, a constructor that throws, or no marker, Agree and Disagree show at once', () => {
  const none = load();
  assert.equal(heldBack(none), false, 'no IntersectionObserver in the browser');

  const throws = load({ observer: 'throws' });
  assert.equal(heldBack(throws), false, 'a constructor that throws never locks anyone out');
  assert.equal(throws.observers.length, 0);

  const noMarker = load({ observer: true, marker: false });
  assert.equal(heldBack(noMarker), false, 'a page with no end marker has nothing to watch');
  assert.equal(noMarker.observers.length, 0);

  click(none, 'welcome-continue-1');
  click(none, 'welcome-agree');
  assert.equal(none.requests.length, 1, 'Agree works');
});

test('an account that already agreed is never held back, and Back to step 2 shows the buttons', () => {
  const page = load({ observer: true, start: 3 });
  assert.equal(page.observers.length, 0, 'no watcher for an account that has agreed');
  assert.equal(heldBack(page), false);
  page.api.goTo(2);
  assert.equal(visibleStep(page), 2);
  assert.equal(heldBack(page), false);
});

test('once shown, Agree and Disagree stay shown: Disagree, Read again, Back, a failed Agree and returning never hide them', async () => {
  const page = load({ observer: true });
  click(page, 'welcome-continue-1');
  seeEnd(page, true);
  assert.equal(heldBack(page), false);

  const box = byId(page, 'welcome-scrollbox');
  box.scrollTop = 900;
  click(page, 'welcome-disagree');
  assert.equal(heldBack(page), false);
  assert.equal(byId(page, 'welcome-disagree-message').hidden, false);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-disagree-message'), 'focus is in the Disagree box');

  click(page, 'welcome-reread');
  assert.equal(box.scrollTop, 0);
  assert.ok(page.doc.activeElement === box, 'Read again focuses the text box');
  assert.equal(heldBack(page), false, 'Read again does not hold the buttons back again');

  click(page, 'welcome-back-2');
  assert.equal(visibleStep(page), 1);
  click(page, 'welcome-continue-1');
  page.api.goTo(1);
  page.api.goTo(2);
  assert.equal(heldBack(page), false);

  click(page, 'welcome-agree');
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(heldBack(page), false, 'a failed Agree leaves the buttons shown');
  assert.equal(page.requests.length, 1, 'only the one consent request was sent');
  assert.equal(page.requests[0].url, '/api/me/consent');

  // Seeing the end again, or not seeing it, changes nothing now.
  seeEnd(page, false);
  seeEnd(page, true);
  assert.equal(heldBack(page), false);
});

// ---- Try the demo is inside the Tab loop (Phase 5 review fix WR-B05) ----
// Added 2026-10-29. The focus trap counts links, so both Try the demo links
// are reached by Tab and Shift+Tab, and Tab from them stays in the flow.

test('WR-B05: in the Disagree box Tab goes from Log out to Try the demo and then wraps', () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-disagree');
  assert.equal(byId(page, 'welcome-disagree-message').hidden, false);
  byId(page, 'welcome-logout').focus();
  const fromLogout = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(fromLogout.prevented, true, 'the page moves focus itself: the Tab of Safari would skip the link');
  assert.ok(page.doc.activeElement === byId(page, 'welcome-try-demo-2'), 'Tab from Log out lands on Try the demo');
  const fromLink = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(fromLink.prevented, true, 'Tab from the last control wraps');
  assert.ok(page.doc.activeElement === page.items[0].button, 'to the first control of the flow, never out of it');
  page.items[0].button.focus();
  const back = fire(page.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.ok(page.doc.activeElement === byId(page, 'welcome-try-demo-2'), 'Shift+Tab from the first control reaches Try the demo');
});

test('WR-B05: on a phone step 1 Tab and Shift+Tab reach Try the demo every time', () => {
  const phone = load();
  phone.side.hiddenByCss = true;
  const link = byId(phone, 'welcome-try-demo-1');
  const go = byId(phone, 'welcome-continue-1');
  go.focus();
  const forward = fire(phone.root, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.ok(phone.doc.activeElement === link, 'Tab from Continue wraps to Try the demo');
  const back = fire(phone.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.ok(phone.doc.activeElement === go, 'Shift+Tab from Try the demo wraps to Continue');
  go.focus();
  assert.equal(fire(phone.root, 'keydown', { key: 'Tab', shiftKey: true }).prevented, true);
  assert.ok(phone.doc.activeElement === link, 'Shift+Tab from Continue lands on Try the demo');
});

test('WR-B05: a hidden input or a control with tabindex -1 is never a Tab stop', () => {
  // Added 2026-10-29 by the Phase 5 review fix (WR-B05): the page moves Tab focus
  // itself, so it must skip what the browser's own Tab skips.
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-disagree');
  const box = byId(page, 'welcome-disagree-message');
  const reread = byId(page, 'welcome-reread');
  const logout = byId(page, 'welcome-logout');
  const hidden = new logout.constructor(page.doc, 'input');
  hidden.type = 'hidden';
  box.children.splice(box.children.indexOf(logout), 0, hidden);
  hidden.parentNode = box;
  const skipped = new logout.constructor(page.doc, 'button');
  skipped.setAttribute('tabindex', '-1');
  box.children.splice(box.children.indexOf(logout), 0, skipped);
  skipped.parentNode = box;
  reread.focus();
  assert.equal(fire(page.root, 'keydown', { key: 'Tab' }).prevented, true);
  assert.ok(page.doc.activeElement === logout, 'Tab from Read again passes over the hidden input and the tabindex -1 button to Log out');
});

// ---- Phase 7.1 plan 13: step 4 asks for all three ---------------------------

const continueOff = (page) => byId(page, 'welcome-continue-4').getAttribute('aria-disabled') === 'true';
const classesOf = (page, id) => byId(page, id).className.split(/\s+/);
const userLine = (page) => byId(page, 'welcome-username-status');

test('Continue starts switched off with aria-disabled and btn--off, never the disabled attribute', () => {
  const page = load({ start: 4 });
  assert.equal(continueOff(page), true);
  assert.ok(classesOf(page, 'welcome-continue-4').includes('btn--off'));
  assert.ok(!classesOf(page, 'welcome-continue-4').includes('btn--primary'));
  assert.equal(byId(page, 'welcome-continue-4').disabled, false, 'still reachable by keyboard');
  assert.equal(userLine(page).textContent, 'Use 3 to 20 letters, numbers, dots or underscores.', 'the rule already shows');
});

test('Continue switches on only when the username, the name and the home area are all good, and off again when one goes', async () => {
  const page = load({ start: 4 });
  await fillStep4(page, { name: null, place: null });
  assert.equal(continueOff(page), true, 'a username alone is not enough');
  await fillStep4(page, { username: null, place: null });
  assert.equal(continueOff(page), true, 'a name and a username are not enough');
  await fillStep4(page, { username: null, name: null });
  assert.equal(continueOff(page), false, 'all three are good');
  assert.ok(classesOf(page, 'welcome-continue-4').includes('btn--primary'));
  assert.ok(!classesOf(page, 'welcome-continue-4').includes('btn--off'));
  assert.equal(byId(page, 'welcome-continue-4').getAttribute('aria-disabled'), null);

  // Emptying the name, retyping the username or editing the picked place switches it off.
  const name = byId(page, 'welcome-name-input');
  name.value = '  ';
  fire(name, 'input');
  assert.equal(continueOff(page), true, 'a name with no letter or digit is not a name');
  name.value = 'Ravi';
  fire(name, 'input');
  assert.equal(continueOff(page), false);
  const user = byId(page, 'welcome-username-input');
  user.value = 'ravi.kk';
  fire(user, 'input');
  assert.equal(continueOff(page), true, 'a new username is good only after its check says so');
  const home = byId(page, 'welcome-home-input');
  home.value = 'Indiranagar, edited';
  fire(home, 'input');
  assert.equal(continueOff(page), true);
});

test('only an available username counts: taken, reserved, invalid and a failed check keep Continue off', async () => {
  for (const [status, body] of [[200, { state: 'taken', message: 'ravi.k is taken. Try another.' }],
    [200, { state: 'reserved', message: 'That name is kept for official use.' }],
    [200, { state: 'invalid', message: 'Use 3 to 20 letters, numbers, dots or underscores.' }],
    [429, { error: { field: 'username', message: 'Too many tries.' } }]]) {
    const page = load({ start: 4, name: 'Ravi', home: 'Whitefield' });
    const user = byId(page, 'welcome-username-input');
    user.value = 'ravi.k';
    fire(user, 'input');
    page.advance(300);
    answer(page.checks[0], status, body);
    await flush();
    assert.equal(continueOff(page), true, JSON.stringify(body));
  }
});

test('the check asks 300 ms after typing stops, folds capitals and drops a typed @', () => {
  const page = load({ start: 4 });
  const user = byId(page, 'welcome-username-input');
  user.value = '@Ravi.K';
  fire(user, 'input');
  assert.equal(user.value, 'ravi.k');
  assert.equal(page.checks.length, 0);
  page.advance(299);
  assert.equal(page.checks.length, 0);
  page.advance(1);
  assert.equal(page.checks.length, 1);
  assert.equal(page.checks[0].url, '/api/username/check?u=ravi.k');
  assert.equal(page.requests.length, 0, 'the check is not a request of the flow');
});

test('a press on Continue while it is off moves focus to the first field that is not good and says what to add', async () => {
  const page = load({ start: 4 });
  click(page, 'welcome-continue-4');
  assert.ok(page.doc.activeElement === byId(page, 'welcome-username-input'), 'the username comes first');
  assert.equal(byId(page, 'welcome-error-4').textContent, 'Add your username, name and home area.');
  assert.equal(page.requests.length, 0);

  await fillStep4(page, { name: null, place: null });
  click(page, 'welcome-continue-4');
  assert.ok(page.doc.activeElement === byId(page, 'welcome-name-input'), 'then the name');

  await fillStep4(page, { username: null, place: null });
  click(page, 'welcome-continue-4');
  assert.ok(page.doc.activeElement === byId(page, 'welcome-home-input'), 'then the home area');
  assert.equal(page.requests.length, 0);
  assert.equal(page.dialog.open, false);
});

test('the line goes when the last missing field is filled in', async () => {
  const page = load({ start: 4 });
  click(page, 'welcome-continue-4');
  assert.equal(byId(page, 'welcome-error-4').hidden, false);
  await fillStep4(page);
  assert.equal(byId(page, 'welcome-error-4').hidden, true);
});

test('Continue sends the username, the name and the home area in one profile request, then the pop-up', async () => {
  const page = load({ start: 4 });
  await fillStep4(page, { username: 'ravi.k', name: '  Ravi Kumar ', place: PLACES[0] });
  click(page, 'welcome-continue-4');
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 1, 'one request, however often it is pressed');
  assert.equal(page.requests[0].url, '/api/me/profile');
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.deepEqual(JSON.parse(page.requests[0].init.body), {
    username: 'ravi.k',
    display_name: 'Ravi Kumar',
    home_area: { name: 'Indiranagar', lat: 12.9784, lon: 77.6408 }
  });
  answer(page.requests[0], 200);
  await flush();
  assert.equal(page.dialog.open, true, 'the end of the flow is as built: the pop-up');
  click(page, 'welcome-alerts-later');
  assert.equal(page.requests[1].url, '/api/me/first-run-done');
  answer(page.requests[1], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('a username the account already has is not sent again and counts as filled in', async () => {
  const page = load({ start: 4, username: 'ravi.k' });
  assert.equal(continueOff(page), true, 'the name and the home area are still missing');
  await fillStep4(page, { username: null });
  assert.equal(continueOff(page), false);
  click(page, 'welcome-continue-4');
  assert.deepEqual(Object.keys(JSON.parse(page.requests[0].init.body)).sort(), ['display_name', 'home_area']);
});

test('a username refused on save shows its sentence under the Username and keeps step 4', async () => {
  const cases = [
    [400, { error: { field: 'username', message: 'ravi.k is taken. Try another.' } }, 'ravi.k is taken. Try another.'],
    [400, { error: { field: 'username', message: 'You can change your username again in 3 days.' } }, 'You can change your username again in 3 days.'],
    [422, { error: { field: 'username', message: 'That username can\'t be used. Try another.' } }, 'That username can\'t be used. Try another.']
  ];
  for (const [status, body, sentence] of cases) {
    const page = load({ start: 4 });
    await fillStep4(page);
    click(page, 'welcome-continue-4');
    answer(page.requests[0], status, body);
    await flush();
    assert.equal(visibleStep(page), 4);
    assert.equal(userLine(page).textContent, sentence);
    assert.ok(userLine(page).className.split(/\s+/).includes('username-status--bad'));
    assert.equal(byId(page, 'welcome-error-4').hidden, true, 'nothing is written in the general line');
    assert.equal(continueOff(page), true, 'the same name is not good again until it is typed again');
    assert.ok(page.doc.activeElement === byId(page, 'welcome-username-input'));
    assert.equal(page.dialog.open, false);
    assert.equal(page.requests.length, 1, 'first-run-done is not sent');
    assert.equal(byId(page, 'welcome-continue-4').disabled, false);
  }
});

test('another refusal of the profile (a 400 for the name, a 429) shows in the general line', async () => {
  const name = load({ start: 4 });
  await fillStep4(name);
  click(name, 'welcome-continue-4');
  answer(name.requests[0], 400, { error: { field: 'display_name', message: 'Use letters, numbers and spaces in your name.' } });
  await flush();
  assert.equal(byId(name, 'welcome-error-4').textContent, 'Use letters, numbers and spaces in your name.');

  const limited = load({ start: 4 });
  await fillStep4(limited);
  click(limited, 'welcome-continue-4');
  answer(limited.requests[0], 429, { error: { field: 'username', message: 'Too many tries. Wait a moment.' } });
  await flush();
  assert.equal(byId(limited, 'welcome-error-4').textContent, 'Too many tries. Wait a moment.');
  assert.equal(continueOff(limited), false, 'the name is still good, so a later press can try again');
});

test('after a save the same values are not sent again on a retry', async () => {
  const page = load({ start: 4 });
  await fillStep4(page);
  click(page, 'welcome-continue-4');
  answer(page.requests[0], 200);
  await flush();
  click(page, 'welcome-alerts-later');
  answer(page.requests[1], 409, { error: { field: 'profile', message: 'Add your username, name and home area.' } });
  await flush();
  assert.equal(visibleStep(page), 4);
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 3, 'nothing changed, so the next request is first-run-done again, not a second profile request');
  assert.equal(page.requests[2].url, '/api/me/first-run-done');
  assert.equal(page.dialog.open, false, 'the pop-up is not asked twice');
});

test('Agree goes to step 3 by default and straight to step 4 when the server says so', async () => {
  const first = load();
  click(first, 'welcome-continue-1');
  click(first, 'welcome-agree');
  answer(first.requests[0], 204);
  await flush();
  assert.equal(visibleStep(first), 3);

  const returning = load({ start: 2, afterTerms: 4 });
  click(returning, 'welcome-agree');
  answer(returning.requests[0], 204);
  await flush();
  assert.equal(visibleStep(returning), 4, 'an account that finished before fills in in one pass');
  assert.equal(returning.doc.activeElement, byId(returning, 'welcome-step-4-title'));

  const odd = load({ start: 2, afterTerms: 7 });
  click(odd, 'welcome-agree');
  answer(odd.requests[0], 204);
  await flush();
  assert.equal(visibleStep(odd), 3, 'a value that is not 3 or 4 means 3');
});

test('a page that starts at step 4 for an account that has agreed works as the flow of a returning account', () => {
  const page = load({ start: 4, afterTerms: 4, username: 'asha.k' });
  assert.equal(visibleStep(page), 4);
  assert.deepEqual(stepStates(page), ['done', 'done', 'done', 'current']);
  assert.equal(page.requests.length, 0);
});

test('the Username box is wired to the shared check and Continue keeps its place in the Tab loop while off', () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-continue-4').focus();
  const forward = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true, 'the page moves focus itself from the last control');
  const stops = [];
  for (let i = 0; i < 12; i++) {
    stops.push(page.doc.activeElement.id);
    fire(page.root, 'keydown', { key: 'Tab' });
  }
  assert.ok(stops.includes('welcome-continue-4'), 'an aria-disabled Continue is still a Tab stop');
});

// Phase 7.1 review fix (browser), WR-B03: when the username is typed and only its
// check failed, Continue asks again instead of telling the person to add a username
// they have already typed; while a check is still out it only puts focus there.
test('WR-B03: Continue after a failed check asks again and does not say "Add your username"', async () => {
  const page = load({ start: 4, name: 'Ravi', home: 'Whitefield' });
  const user = byId(page, 'welcome-username-input');
  user.value = 'ravi.k';
  fire(user, 'input');
  page.advance(300);
  page.checks[0].reject(new TypeError('Failed to fetch'));
  await flush();
  assert.equal(continueOff(page), true);

  click(page, 'welcome-continue-4');
  assert.notEqual(byId(page, 'welcome-error-4').textContent, 'Add your username, name and home area.');
  assert.equal(byId(page, 'welcome-error-4').hidden, true);
  assert.ok(page.doc.activeElement === user, 'focus goes to the username');
  page.advance(300);
  assert.equal(page.checks.length, 2, 'a new check is sent');
  assert.equal(page.checks[1].url, '/api/username/check?u=ravi.k');

  // While that check is out, a second press sends nothing more.
  click(page, 'welcome-continue-4');
  page.advance(300);
  assert.equal(page.checks.length, 2);
  assert.equal(byId(page, 'welcome-error-4').hidden, true);

  answer(page.checks[1], 200, { state: 'available', message: 'ravi.k is available.' });
  await flush();
  assert.equal(continueOff(page), false);
  assert.equal(page.requests.length, 0);

  // With the name missing as well, the line still says what to add.
  const both = load({ start: 4, home: 'Whitefield' });
  const user2 = byId(both, 'welcome-username-input');
  user2.value = 'ravi.k';
  fire(user2, 'input');
  both.advance(300);
  both.checks[0].reject(new TypeError('Failed to fetch'));
  await flush();
  click(both, 'welcome-continue-4');
  assert.equal(byId(both, 'welcome-error-4').textContent, 'Add your username, name and home area.');
});
