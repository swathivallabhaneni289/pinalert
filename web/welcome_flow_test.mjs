// welcome_flow_test.mjs - runs the real web/static/js/welcome.js in a Node
// sandbox and checks the first-time flow (Phase 3.4, plan 10, D-27, D-28, D-24b):
//
//   - four steps, one at a time, the title takes focus on each, Tab stays
//     inside, and Escape does nothing because the flow is completed, not closed;
//   - Disagree shows one message and stays on step 2; only Agree records the
//     agreement (POST /api/me/consent) and moves on, and only on a 204;
//   - step 3 asks the browser for the position only when Allow location is
//     pressed, keeps the fix in this browser through PinalertPosition and sends
//     it nowhere; Search a place instead shows a place search whose
//     picked result is kept in memory for step 4 only;
//   - step 4's home area comes only from a picked search result, never from the
//     device fix; nothing is saved until Continue; Skip saves nothing;
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

  const list = make('ol', '', root, 'welcome__steps');
  const items = STEP_NAMES.map((label, i) => {
    const li = make('li', '', list, 'welcome__steps-item');
    li.setAttribute('data-step', String(i + 1));
    li.setAttribute('data-state', i + 1 < start ? 'done' : (i + 1 === start ? 'current' : 'waiting'));
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

  const box = make('div', 'welcome-scrollbox', steps[1], 'welcome__scrollbox');
  box.setAttribute('tabindex', '0');
  make('button', 'welcome-disagree', steps[1]);
  make('button', 'welcome-agree', steps[1]);
  make('p', 'welcome-disagree-message', steps[1]).hidden = true;
  make('p', 'welcome-error-2', steps[1]).hidden = true;

  make('button', 'welcome-allow-location', steps[2]);
  make('button', 'welcome-search-open', steps[2]);
  const block = make('div', 'welcome-search-block', steps[2]);
  block.hidden = true;
  const form3 = make('form', 'welcome-search-form', block);
  make('input', 'welcome-search-input', form3);
  make('p', 'welcome-search-status', block).hidden = true;
  make('ul', 'welcome-search-results', block).hidden = true;
  make('button', 'welcome-continue-3', block);

  const nameInput = make('input', 'welcome-name-input', steps[3]);
  nameInput.value = name;
  const form4 = make('form', 'welcome-home-form', steps[3]);
  const homeInput = make('input', 'welcome-home-input', form4);
  homeInput.value = home;
  make('p', 'welcome-home-prefilled', steps[3]).hidden = true;
  make('p', 'welcome-home-status', steps[3]).hidden = true;
  make('ul', 'welcome-home-results', steps[3]).hidden = true;
  make('p', 'welcome-error-4', steps[3]).hidden = true;
  make('button', 'welcome-continue-4', steps[3]);
  make('button', 'welcome-skip', steps[3]);

  return { doc, root, steps, items };
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
    location: { replace(target) { replaced.push(target); } }
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'welcome.js' });
  return { ...page, api: sandbox.PinalertWelcome, requests, positionCalls, saved, replaced, session, advance, sandbox };
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

const DISAGREE = 'You need to agree to use Pinalert.';

// The places a search answers with, short name first.
const PLACES = [
  { name: 'Indiranagar', display_name: 'Indiranagar, Bengaluru, Karnataka, India', lat: 12.9784, lon: 77.6408 },
  { name: 'Indira Nagar', display_name: 'Indira Nagar, Lucknow, Uttar Pradesh, India', lat: 26.8806, lon: 80.9946 }
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
  assert.equal(page.items[1].getAttribute('aria-current'), 'step');
  assert.equal(page.items[0].getAttribute('aria-current'), null);
  assert.equal(page.requests.length, 0, 'moving through step 1 sends nothing');
});

test('the flow starts at the step the server names', () => {
  const page = load({ start: 3 });
  assert.equal(visibleStep(page), 3);
  assert.equal(page.requests.length, 0, 'starting at step 3 does not record anything again');
  assert.deepEqual(page.items.map((i) => i.getAttribute('data-state')), ['done', 'done', 'current', 'waiting']);
});

test('Disagree shows exactly the one message, stays on step 2, and a step change clears it', async () => {
  const page = load();
  click(page, 'welcome-continue-1');
  click(page, 'welcome-disagree');

  const message = byId(page, 'welcome-disagree-message');
  assert.equal(message.hidden, false);
  assert.equal(message.textContent, DISAGREE);
  assert.equal(visibleStep(page), 2);
  assert.equal(page.requests.length, 0, 'Disagree records nothing');

  click(page, 'welcome-agree');
  answer(page.requests[0], 204);
  await flush();
  assert.equal(visibleStep(page), 3);
  assert.equal(message.hidden, true);
  assert.equal(message.textContent, '');
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
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true);
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

test('Search a place instead shows the search without asking for the position', () => {
  const page = load({ start: 3 });
  assert.equal(byId(page, 'welcome-search-block').hidden, true);
  click(page, 'welcome-search-open');
  assert.equal(byId(page, 'welcome-search-block').hidden, false);
  assert.equal(page.doc.activeElement, byId(page, 'welcome-search-input'));
  assert.equal(page.positionCalls.length, 0);
  assert.equal(visibleStep(page), 3, 'it reveals the search and does not move on');
});

test('prefill from a step 3 search shows the line', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  byId(page, 'welcome-search-input').value = 'Indira';
  fire(byId(page, 'welcome-search-form'), 'submit');

  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/geocode?q=Indira');
  assert.equal(page.requests[0].init.method === undefined || page.requests[0].init.method === 'GET', true);
  answer(page.requests[0], 200, { results: PLACES });
  await flush();

  const list = byId(page, 'welcome-search-results');
  assert.equal(list.hidden, false);
  assert.equal(list.children.length, 2);
  fire(list.children[0].children[0], 'click');
  click(page, 'welcome-continue-3');

  assert.equal(visibleStep(page), 4);
  assert.equal(byId(page, 'welcome-home-input').value, 'Indiranagar');
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, false);
  assert.equal(page.requests.length, 1, 'neither path stores anything on the server');
  assert.deepEqual(page.saved, []);
});

test('no step 3 search shows no line', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  click(page, 'welcome-continue-3');
  assert.equal(visibleStep(page), 4);
  assert.equal(byId(page, 'welcome-home-input').value, '');
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true);

  const viaAllow = load({ start: 3 });
  click(viaAllow, 'welcome-allow-location');
  viaAllow.positionCalls[0].ok({ coords: { latitude: 12.97, longitude: 77.59 } });
  await flush();
  assert.equal(byId(viaAllow, 'welcome-home-input').value, '');
  assert.equal(byId(viaAllow, 'welcome-home-prefilled').hidden, true);
});

test('stored home area shows no line', () => {
  const page = load({ start: 3, home: 'Whitefield' });
  click(page, 'welcome-search-open');
  click(page, 'welcome-continue-3');
  assert.equal(visibleStep(page), 4);
  assert.equal(byId(page, 'welcome-home-input').value, 'Whitefield');
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true);
});

test('typing in step 3 after a pick forgets the pick, so step 4 is not prefilled', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  const input = byId(page, 'welcome-search-input');
  input.value = 'Indira';
  fire(byId(page, 'welcome-search-form'), 'submit');
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  fire(byId(page, 'welcome-search-results').children[0].children[0], 'click');
  input.value = 'Something else';
  fire(input, 'input');
  click(page, 'welcome-continue-3');
  assert.equal(byId(page, 'welcome-home-input').value, '');
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true);
});

test('a failed or rate limited step 3 search shows a plain line and Continue still works', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  const input = byId(page, 'welcome-search-input');
  const status = byId(page, 'welcome-search-status');

  input.value = 'ab';
  fire(byId(page, 'welcome-search-form'), 'submit');
  assert.equal(status.textContent, 'Type at least 3 letters.');
  assert.equal(page.requests.length, 0);

  input.value = 'Indira';
  fire(byId(page, 'welcome-search-form'), 'submit');
  answer(page.requests[0], 429, {});
  await flush();
  assert.equal(status.textContent, 'Too many searches. Wait a few seconds and try again.');

  input.value = 'Indiranagar';
  fire(byId(page, 'welcome-search-form'), 'submit');
  page.requests[1].reject(new Error('offline'));
  await flush();
  assert.equal(status.textContent, 'Search unavailable. Try again in a moment.');

  input.value = 'Nowhere at all';
  fire(byId(page, 'welcome-search-form'), 'submit');
  answer(page.requests[2], 200, { results: [] });
  await flush();
  assert.equal(status.textContent, 'No matches found. Try a nearby town or landmark.');

  click(page, 'welcome-continue-3');
  assert.equal(visibleStep(page), 4);
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true);
});

test('a search that never answers still ends, and Continue still works', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  byId(page, 'welcome-search-input').value = 'Indira';
  fire(byId(page, 'welcome-search-form'), 'submit');
  page.advance(11000);
  await flush();
  assert.equal(byId(page, 'welcome-search-status').textContent, 'Search unavailable. Try again in a moment.');
  click(page, 'welcome-continue-3');
  assert.equal(visibleStep(page), 4);
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
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true, 'a step 4 pick is not a step 3 prefill');

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

test('typing after a step 3 prefill forgets the pick and hides the line', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  byId(page, 'welcome-search-input').value = 'Indira';
  fire(byId(page, 'welcome-search-form'), 'submit');
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  fire(byId(page, 'welcome-search-results').children[0].children[0], 'click');
  click(page, 'welcome-continue-3');
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, false);

  const home = byId(page, 'welcome-home-input');
  home.value = 'Indiranagar East';
  fire(home, 'input');
  assert.equal(byId(page, 'welcome-home-prefilled').hidden, true);
  click(page, 'welcome-continue-4');
  assert.equal(page.requests.length, 2, 'only first-run-done follows: the edited text was never picked');
  assert.equal(page.requests[1].url, '/api/me/first-run-done');
});

test('a prefilled home area from step 3 is saved with the picked place point on Continue', async () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  byId(page, 'welcome-search-input').value = 'Indira';
  fire(byId(page, 'welcome-search-form'), 'submit');
  answer(page.requests[0], 200, { results: PLACES });
  await flush();
  fire(byId(page, 'welcome-search-results').children[0].children[0], 'click');
  click(page, 'welcome-continue-3');
  click(page, 'welcome-continue-4');
  assert.deepEqual(JSON.parse(page.requests[1].init.body), { home_area: { name: 'Indiranagar', lat: 12.9784, lon: 77.6408 } });
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

  const box = byId(page, 'welcome-scrollbox');
  const agree = byId(page, 'welcome-agree');
  agree.focus();
  const forward = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(forward.prevented, true);
  assert.equal(page.doc.activeElement, box);

  box.focus();
  const back = fire(page.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(back.prevented, true);
  assert.equal(page.doc.activeElement, agree);

  byId(page, 'welcome-step-2-title').focus();
  const fromTitle = fire(page.root, 'keydown', { key: 'Tab', shiftKey: true });
  assert.equal(fromTitle.prevented, true);
  assert.equal(page.doc.activeElement, agree);

  byId(page, 'welcome-disagree').focus();
  const middle = fire(page.root, 'keydown', { key: 'Tab' });
  assert.equal(middle.prevented, false, 'Tab between two controls is left to the browser');
});

test('the public hooks move the flow the same way the buttons do', async () => {
  const page = load();
  page.api.goTo(2);
  assert.equal(visibleStep(page), 2);
  page.api.disagree();
  assert.equal(byId(page, 'welcome-disagree-message').textContent, DISAGREE);
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

test('step 3 searches any place and does not search while typing', () => {
  const page = load({ start: 3 });
  click(page, 'welcome-search-open');
  const input = byId(page, 'welcome-search-input');
  input.value = 'Indira';
  fire(input, 'input');
  page.advance(5000);
  assert.equal(page.requests.length, 0);
  fire(byId(page, 'welcome-search-form'), 'submit');
  assert.equal(page.requests[0].url, '/api/geocode?q=Indira', 'step 3 keeps the report form search, no areas filter');
});
