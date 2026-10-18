// welcome_flow_test.mjs - runs the real web/static/js/welcome.js in a Node
// sandbox and checks the first-time flow (Phase 3.4, plan 10, D-27, D-28, D-24b):
//
//   - four steps, one at a time, the title takes focus on each, Tab stays
//     inside, and Escape does nothing because the flow is completed, not closed;
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
//     Continue; Skip saves nothing;
//   - finishing sends the profile (only what the person filled), then
//     first-run-done, then lands on / with the search focus flag set;
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

// The markup of welcome.html.tmpl, as the ids and nesting welcome.js reads.
function buildPage({ start = 1, name = '', home = '' } = {}) {
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

  make('button', 'welcome-continue-1', steps[0]);
  [2, 3, 4].forEach((n) => make('button', 'welcome-back-' + n, steps[n - 1], 'welcome__back'));

  const box = make('div', 'welcome-scrollbox', steps[1], 'welcome__scrollbox');
  box.setAttribute('tabindex', '0');
  make('button', 'welcome-disagree', steps[1]);
  make('button', 'welcome-agree', steps[1]);
  // Amended 2026-10-17 by Phase 3.4 plan 31: Disagree shows a box (its words
  // live in the template) with Read again and Log out inside it.
  const disagreeBox = make('div', 'welcome-disagree-message', steps[1]);
  disagreeBox.setAttribute('tabindex', '-1');
  disagreeBox.hidden = true;
  make('button', 'welcome-reread', disagreeBox);
  make('button', 'welcome-logout', disagreeBox);
  make('p', 'welcome-error-2', steps[1]).hidden = true;

  make('button', 'welcome-allow-location', steps[2]);
  make('button', 'welcome-not-now', steps[2]);

  const nameInput = make('input', 'welcome-name-input', steps[3]);
  nameInput.value = name;
  const form4 = make('form', 'welcome-home-form', steps[3]);
  const homeInput = make('input', 'welcome-home-input', form4);
  homeInput.value = home;
  make('p', 'welcome-home-status', steps[3]).hidden = true;
  make('ul', 'welcome-home-results', steps[3]).hidden = true;
  make('p', 'welcome-error-4', steps[3]).hidden = true;
  make('button', 'welcome-continue-4', steps[3]);
  make('button', 'welcome-skip', steps[3]);

  return { doc, root, steps, items, side };
}

// load evaluates welcome.js against a page. fetch keeps every call for the test
// to answer. getCurrentPosition keeps its callbacks for the test to run.
function load(options) {
  const page = buildPage(options);
  const requests = [];
  const positionCalls = [];
  const saved = [];
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
        requests.push({ url: String(target), init: init || {}, resolve, reject, at: now });
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
    // The side panel is a laptop part: a test sets side.hiddenByCss to play the phone.
    getComputedStyle(node) { return { display: node.hiddenByCss ? 'none' : 'block' }; },
    addEventListener(type, fn) { (listeners[type] = listeners[type] || []).push(fn); }
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'welcome.js' });
  const popTo = (state) => (listeners.popstate || []).forEach((fn) => fn({ type: 'popstate', state }));
  return { ...page, api: sandbox.PinalertWelcome, requests, positionCalls, saved, replaced, session, advance, sandbox, pushes, stateReplaces, popTo };
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

// The places a search answers with, short name first.
const PLACES = [
  { name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru, Karnataka, India', lat: 12.9784, lon: 77.6408, area: 'Indiranagar' },
  { name: 'Indira Nagar', display_name: 'Indira Nagar, Lucknow, Uttar Pradesh, India', lat: 26.8806, lon: 80.9946, area: 'Indira Nagar' }
];

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
  assert.equal(box.children.length, 2, 'the script left the box markup alone');
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

  const stored = load({ start: 3, home: 'Whitefield' });
  click(stored, 'welcome-not-now');
  assert.equal(byId(stored, 'welcome-home-input').value, 'Whitefield', 'a stored home area stays');
  click(stored, 'welcome-continue-4');
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
  click(page, 'welcome-skip');
  assert.equal(visibleStep(page), 2, 'Skip before Agree shows step 2');
  assert.equal(page.requests.length, 0);
});

test('step 4 Continue sends only the filled fields, then first-run-done, then lands on / with the search flag', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-name-input').value = '  Ravi Kumar  ';
  click(page, 'welcome-continue-4');
  click(page, 'welcome-continue-4');

  assert.equal(page.requests.length, 1, 'a second tap while saving must not send again');
  assert.equal(page.requests[0].url, '/api/me/profile');
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.equal(page.requests[0].init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(page.requests[0].init.body), { display_name: 'Ravi Kumar' });
  answer(page.requests[0], 200, { display_name: 'Ravi Kumar', home_area_name: null });
  await flush();

  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].url, '/api/me/first-run-done');
  assert.equal(page.requests[1].init.method, 'POST');
  assert.deepEqual(page.replaced, [], 'no move before first-run-done answers');
  answer(page.requests[1], 204);
  await flush();

  assert.equal(page.session['pinalert.focusSearch'], '1');
  assert.deepEqual(page.replaced, ['/']);
});

test('with both fields empty there is no profile request, and Skip sends only first-run-done', async () => {
  const empty = load({ start: 4 });
  click(empty, 'welcome-continue-4');
  assert.equal(empty.requests.length, 1);
  assert.equal(empty.requests[0].url, '/api/me/first-run-done');
  answer(empty.requests[0], 204);
  await flush();
  assert.deepEqual(empty.replaced, ['/']);

  const skipped = load({ start: 4 });
  byId(skipped, 'welcome-name-input').value = 'Typed but skipped';
  click(skipped, 'welcome-skip');
  assert.equal(skipped.requests.length, 1, 'Skip saves nothing the person typed');
  assert.equal(skipped.requests[0].url, '/api/me/first-run-done');
  answer(skipped.requests[0], 204);
  await flush();
  assert.equal(skipped.session['pinalert.focusSearch'], '1');
  assert.deepEqual(skipped.replaced, ['/']);
});

test('a stored name and home area left alone are not sent again', async () => {
  const page = load({ start: 4, name: 'Asha', home: 'Whitefield' });
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/me/first-run-done');
});

test('the home area sent is the picked place with its own point, never the device fix, and typed text alone is not sent', async () => {
  const page = load({ start: 3 });
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

  const typed = load({ start: 4 });
  byId(typed, 'welcome-home-input').value = 'Somewhere I typed';
  click(typed, 'welcome-continue-4');
  assert.equal(typed.requests.length, 1, 'text that was never picked from a search is not a home area');
  assert.equal(typed.requests[0].url, '/api/me/first-run-done');
});

test('a refused profile keeps step 4 with the server sentence and does not finish', async () => {
  const page = load({ start: 4 });
  byId(page, 'welcome-name-input').value = 'Ravi';
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

test('first-run-done refused for missing consent sends the person back to step 2 and does not leave', async () => {
  const page = load({ start: 4 });
  click(page, 'welcome-skip');
  answer(page.requests[0], 409, { error: { field: 'terms', message: 'Agree first.' } });
  await flush();
  assert.equal(visibleStep(page), 2);
  assert.equal(byId(page, 'welcome-error-2').textContent, 'Please agree to the Disclaimer and policy to continue.');
  assert.deepEqual(page.replaced, []);
  assert.equal(page.session['pinalert.focusSearch'], undefined);
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
  assert.equal(middle.prevented, false, 'Tab between two controls is left to the browser');

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
  assert.equal(fire(phone.root, 'keydown', { key: 'Tab', shiftKey: true }).prevented, false);

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

  // The line goes when the person moves on, and Skip is held to the same rule.
  fire(listButton(page, 4), 'click');
  assert.equal(line.hidden, true);
  click(page, 'welcome-skip');
  assert.equal(visibleStep(page), 2);
  assert.equal(line.textContent, AGREE_FIRST);
  assert.equal(page.requests.length, 0);

  // The public hook is held to it too.
  page.api.goTo(4);
  page.api.finish(true);
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0);
});

test('after Agree, Finish works from step 4 even when the person came back to step 2 first', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-agree');
  answer(page.requests[0], 204);
  await flush();
  click(page, 'welcome-back-3');
  assert.equal(visibleStep(page), 2);
  fire(listButton(page, 4), 'click');
  click(page, 'welcome-skip');
  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].url, '/api/me/first-run-done');
  answer(page.requests[1], 204);
  await flush();
  assert.deepEqual(page.replaced, ['/']);
});

test('the server consent rule still holds: a 409 from first-run-done sends the person back to step 2', async () => {
  const page = load({ start: 4 });
  click(page, 'welcome-skip');
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
  assert.equal(visibleStep(load({ start: 3, search: '' })), 3);
  const loaded = load({ start: 3, search: '?step=4' });
  assert.equal(loaded.stateReplaces[0].address, '?step=4', 'the entry names the step actually shown');
});

test('what was typed or picked is kept while moving between steps', async () => {
  const page = load();
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
  const page = load({ start: 4 });
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

test('an empty answer shows the same not found line', async () => {
  const page = load({ start: 4 });
  typeHome(page, 'Zzqqxx');
  page.advance(400);
  answer(page.requests[0], 200, { results: [] });
  await flush();
  assert.equal(homeStatus(page).textContent, NOT_FOUND);
  assert.equal(homeList(page).hidden, true);
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
  answer(page.requests[0], 200, { results: [] });
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
