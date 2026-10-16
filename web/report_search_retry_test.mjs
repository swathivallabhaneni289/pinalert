// report_search_retry_test.mjs - runs the real web/static/js/report-form.js in
// a Node sandbox and checks the report form's address search while the person
// types (quick 261016-sne): a 429 or a 503 shows no failure line, the list on
// screen stays, and the latest text is tried again once the wait passes (the
// answer's Retry-After, or 1 second), at most 3 times. Newer text cancels a
// pending try. Only when the last try also fails does the failure line show.
//
// Run it with: node --test web/report_search_retry_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what report-form.js calls: a minimal element model,
// fake timers, a fetch the test answers by hand, a fake Pinalert and a
// location object that is always set. Every advance stays under the search's
// own 8 second limit, so that timer never fires on a request still out.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';
import crypto from 'node:crypto';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'report-form.js'), 'utf8');

const FAILED = 'Search failed.';
const TOO_MANY = 'Too many searches. Wait a few seconds and try again.';

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
    this.disabled = false;
    this.required = false;
    this.value = '';
    this.selectedIndex = 0;
    this.tabIndex = 0;
    this.type = '';
    const self = this;
    this.classList = {
      add: (...names) => names.forEach((n) => { if (!self.classes().includes(n)) { self.className = (self.className + ' ' + n).trim(); } }),
      remove: (...names) => { self.className = self.classes().filter((c) => !names.includes(c)).join(' '); },
      toggle: (n, on) => { if (on) { self.classList.add(n); } else { self.classList.remove(n); } },
      contains: (n) => self.classes().includes(n)
    };
  }
  classes() { return this.className.split(/\s+/).filter(Boolean); }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  get previousElementSibling() {
    if (!this.parentNode) { return null; }
    const at = this.parentNode.children.indexOf(this);
    return at > 0 ? this.parentNode.children[at - 1] : null;
  }
  appendChild(node) { return this.insertBefore(node, null); }
  insertBefore(node, ref) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    const at = ref ? this.children.indexOf(ref) : -1;
    if (at === -1) { this.children.push(node); } else { this.children.splice(at, 0, node); }
    node.parentNode = this;
    return node;
  }
  removeChild(node) {
    const at = this.children.indexOf(node);
    if (at !== -1) { this.children.splice(at, 1); }
    node.parentNode = null;
    return node;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  dispatchEvent(evt) {
    if (!evt.target) { evt.target = this; }
    (this.handlers[evt.type] || []).slice().forEach((h) => h(evt));
    return true;
  }
  click() { if (!this.disabled) { this.dispatchEvent({ type: 'click', preventDefault() {} }); } }
  focus() { this.doc.activeElement = this; }
  closest() { return null; }
  descendants() {
    const out = [];
    for (const c of this.children) { out.push(c, ...c.descendants()); }
    return out;
  }
  querySelectorAll(selector) {
    if (!selector.startsWith('.') || selector.includes(',')) { return []; }
    const cls = selector.slice(1);
    return this.descendants().filter((el) => el.classList.contains(cls));
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
}

function setup({ variant } = {}) {
  const doc = {
    activeElement: null,
    handlers: {},
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    addEventListener(type, fn) { (doc.handlers[type] = doc.handlers[type] || []).push(fn); },
    removeEventListener(type, fn) { doc.handlers[type] = (doc.handlers[type] || []).filter((h) => h !== fn); }
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  const backdrop = make('div', 'report-backdrop');
  backdrop.hidden = true;
  const form = make('div', 'report-form', backdrop);
  make('div', 'category-grid', form);
  make('label', '', form);
  make('input', 'severity', form).value = '1';
  make('output', 'severity-readout', form);
  make('textarea', 'description', form);
  make('p', 'form-error', form);
  const input = make('input', 'location-search', form);
  const results = make('div', 'location-search-results', form);
  results.hidden = true;
  const status = make('p', 'location-search-status', form);
  status.hidden = true;
  make('div', 'shelter-fields', form);
  make('select', 'shelter-capacity-status', form);
  make('input', 'shelter-headcount', form);
  make('div', 'discard-confirm', form).hidden = true;
  make('button', 'discard-confirm-discard', form);
  make('button', 'discard-confirm-keep', form);
  make('p', 'form-note', form).hidden = true;
  make('button', 'send-after-wait', form).hidden = true;
  make('button', 'cancel-report', form);
  make('button', 'submit-report', form);
  make('div', 'notice-live');
  const opener = make('button', 'open-report');

  // Fake timers: the test moves the clock.
  let now = 0;
  let nextId = 1;
  const timers = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) { break; }
      timers.delete(due[0]);
      now = due[1].at;
      due[1].fn();
    }
    now = end;
  };

  // fetch is answered by hand. Each call is kept with its abort state.
  const requests = [];
  const fetch = (u, opts) => new Promise((resolve, reject) => {
    const req = { url: u, signal: opts && opts.signal, resolve, reject, aborted: false };
    if (req.signal) {
      req.signal.addEventListener('abort', () => {
        req.aborted = true;
        const err = new Error('aborted');
        err.name = 'AbortError';
        reject(err);
      });
    }
    requests.push(req);
  });
  const answer = (req, status, body, headers) => req.resolve({
    ok: status >= 200 && status < 300,
    status,
    headers: { get: (name) => (headers && Object.hasOwn(headers, name) ? headers[name] : null) },
    json: () => Promise.resolve(body)
  });

  const outcomes = [];
  // failures keeps what the form passed to searchFailureMessage each time.
  const failures = [];
  const Pinalert = {
    CATEGORIES: ['flood', 'road_blocked', 'shelter_open'],
    CATEGORY_LABELS: { flood: 'Flood', road_blocked: 'Road blocked', shelter_open: 'Shelter open' },
    CAPACITY_STATUSES: ['available', 'limited', 'full', 'closed'],
    SEVERITY_LABELS: { low: '1 · Low', medium: '2 · Medium', critical: '3 · Critical' },
    iconClass: (c) => 'icon-glyph icon-glyph--' + c,
    formatWait: (s) => String(s),
    setText: (el, v) => { el.textContent = v === null || v === undefined ? '' : String(v); },
    showToast: () => {},
    lockPage: () => {},
    unlockPage: () => {},
    submitReport: () => new Promise(() => {})
  };
  const picker = {
    current: () => ({ latitude: 17.385, longitude: 78.4867 }),
    choose: () => {},
    isSet: () => true,
    missingMessage: () => 'Set a location.',
    searchFailureMessage: (text, status) => { failures.push([text, status]); return FAILED; },
    noteSearchOutcome: (o) => { outcomes.push(o); }
  };
  class CustomEvent {
    constructor(type, init) { this.type = type; this.detail = init && init.detail; }
  }
  const window = {
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { at: now + ms, fn }); return id; },
    clearTimeout(id) { timers.delete(id); },
    location: { href: 'https://pinalert.example/' },
    PinalertQueue: { save: () => Promise.resolve(true), discard: () => {} }
  };
  const sandbox = { window, document: doc, CustomEvent, crypto, Map, Date, Math, JSON, Promise, Array, Object, isFinite, encodeURIComponent, fetch, AbortController };
  window.Pinalert = Pinalert;
  sandbox.Pinalert = Pinalert;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  const Form = window.PinalertReportForm;
  Form.mount({ backdrop, form, location: picker, variant });
  Form.open({ opener });

  const type = (text) => { input.value = text; input.dispatchEvent({ type: 'input' }); };
  const query = (req) => new URL(req.url, 'https://pinalert.example').searchParams.get('q');
  const rows = () => results.querySelectorAll('.location-search-result');
  const el = (id) => doc.byId[id];
  return { input, status, results, requests, answer, advance, type, query, rows, outcomes, failures, el };
}

const settle = () => new Promise((resolve) => setImmediate(resolve));

const banjara = { results: [{ name: 'Banjara Hills', display_name: 'Banjara Hills, Hyderabad, Telangana', lat: 17.4156, lon: 78.4347 }] };
const roadNo12 = { results: [
  { name: 'Road No 12', display_name: 'Road No 12, Banjara Hills, Hyderabad, Telangana', lat: 17.41, lon: 78.43 },
  { name: 'Road No 12 Park', display_name: 'Road No 12 Park, Banjara Hills, Hyderabad, Telangana', lat: 17.42, lon: 78.44 }
] };

// showList types a text, lets the pause pass and answers it with a list.
async function showList(p, text, body) {
  p.type(text);
  p.advance(300);
  p.answer(p.requests[p.requests.length - 1], 200, body);
  await settle();
}

test('a 429 from typing keeps the list, shows no line, and tries the latest text again after 1 second', async () => {
  const p = setup();
  await showList(p, 'Banjara', banjara);
  assert.equal(p.rows().length, 1);

  p.type('Road No 12 Banjara');
  p.advance(300);
  assert.equal(p.requests.length, 2);
  p.answer(p.requests[1], 429, { error: { field: 'q', message: TOO_MANY } });
  await settle();
  assert.equal(p.status.hidden, true, 'no failure line while typing');
  assert.equal(p.results.hidden, false, 'the list on screen stays');
  assert.equal(p.rows().length, 1);
  assert.deepEqual(p.outcomes, ['match'], 'no unavailable outcome is reported for a quiet try');

  p.advance(999);
  assert.equal(p.requests.length, 2, 'nothing before the wait passes');
  p.advance(1);
  assert.equal(p.requests.length, 3, 'the latest text is tried again');
  assert.equal(p.query(p.requests[2]), 'Road No 12 Banjara');

  p.answer(p.requests[2], 200, roadNo12);
  await settle();
  assert.equal(p.rows().length, 2, 'the new list replaces the old one');
  assert.equal(p.status.hidden, true);
});

test('a busy 503 is tried again quietly the same way', async () => {
  const p = setup();
  p.type('Gachibowli');
  p.advance(300);
  p.answer(p.requests[0], 503, { error: { field: 'q', message: 'Search unavailable, try tapping the map instead.' } });
  await settle();
  assert.equal(p.status.hidden, true);
  p.advance(1000);
  assert.equal(p.requests.length, 2);
  p.answer(p.requests[1], 200, banjara);
  await settle();
  assert.equal(p.rows().length, 1);
});

test("the answer's Retry-After sets the wait", async () => {
  const p = setup();
  p.type('Gachibowli');
  p.advance(300);
  p.answer(p.requests[0], 429, {}, { 'Retry-After': '2' });
  await settle();
  p.advance(1999);
  assert.equal(p.requests.length, 1);
  p.advance(1);
  assert.equal(p.requests.length, 2);
});

test('after 3 quiet tries, a fourth failure shows the failure line', async () => {
  const p = setup();
  await showList(p, 'Banjara', banjara);
  p.type('Banjara Hills Road');
  p.advance(300);
  for (let i = 1; i <= 3; i++) {
    p.answer(p.requests[i], 429, { error: { message: TOO_MANY } });
    await settle();
    assert.equal(p.status.hidden, true, 'try ' + i + ' failed quietly');
    p.advance(1000);
    assert.equal(p.requests.length, i + 2, 'the text is tried again after failure ' + i);
  }
  p.answer(p.requests[4], 503, {});
  await settle();
  assert.equal(p.status.hidden, false, 'the last try shows the line');
  assert.equal(p.status.textContent, FAILED);
  assert.equal(p.results.hidden, true, 'the old list goes with the line, as before');
  assert.equal(p.outcomes[p.outcomes.length - 1], 'unavailable');
  p.advance(5000);
  assert.equal(p.requests.length, 5, 'no try after the last one');
});

test('newer text cancels a pending try, and gets its own 3 tries', async () => {
  const p = setup();
  p.type('Banjara Hi');
  p.advance(300);
  p.answer(p.requests[0], 429, {});
  await settle();
  p.advance(500);
  p.type('Banjara Hills');
  p.advance(300);
  assert.equal(p.requests.length, 2);
  assert.equal(p.query(p.requests[1]), 'Banjara Hills', 'only the newer text is asked');
  p.advance(2000);
  assert.equal(p.requests.length, 2, 'the older text is never tried again');

  for (let i = 1; i <= 3; i++) {
    p.answer(p.requests[i], 429, {});
    await settle();
    assert.equal(p.status.hidden, true);
    p.advance(1000);
  }
  assert.equal(p.requests.length, 5, 'the newer text got all 3 tries');
  for (const req of p.requests.slice(1)) {
    assert.equal(p.query(req), 'Banjara Hills');
  }
});

test('a refusal for older text that lands during the newer pause is dropped', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  assert.equal(p.requests.length, 1);
  p.type('Banjara Hills');
  p.advance(100);
  p.answer(p.requests[0], 429, {});
  await settle();
  p.advance(200);
  assert.equal(p.requests.length, 2, 'the newer text goes out at its pause');
  assert.equal(p.query(p.requests[1]), 'Banjara Hills');
  p.answer(p.requests[1], 200, roadNo12);
  await settle();
  assert.equal(p.rows().length, 2, 'the newer list shows');
  p.advance(3000);
  assert.equal(p.requests.length, 2, 'the older text is never tried again');
  assert.equal(p.rows().length, 2);
  assert.equal(p.status.hidden, true);
});

test('a refusal for older text after the box drops under 3 letters sends nothing', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.type('Ba');
  p.answer(p.requests[0], 429, {});
  await settle();
  p.advance(3000);
  assert.equal(p.requests.length, 1);
  assert.equal(p.results.hidden, true);
  assert.equal(p.status.hidden, true);
});

test('under 3 letters cancels a pending try', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 429, {});
  await settle();
  p.type('Ba');
  p.advance(3000);
  assert.equal(p.requests.length, 1);
  assert.equal(p.status.hidden, true);
});

test('closing the form cancels a pending try', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 429, {});
  await settle();
  p.el('cancel-report').click();
  p.el('discard-confirm-discard').click();
  assert.equal(p.el('report-backdrop').hidden, true);
  p.advance(3000);
  assert.equal(p.requests.length, 1);
});

test('a network error or a 500 still shows the failure line at once', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.requests[0].reject(new TypeError('network'));
  await settle();
  assert.equal(p.status.textContent, FAILED);
  assert.equal(p.status.hidden, false);
  p.advance(3000);
  assert.equal(p.requests.length, 1, 'a network error is not tried again');

  p.type('Banjara Hills');
  p.advance(300);
  p.answer(p.requests[1], 500, {});
  await settle();
  assert.equal(p.status.textContent, FAILED);
  p.advance(3000);
  assert.equal(p.requests.length, 2, 'a 500 is not tried again');
});

// The Low data page shares this search (F-P4-12): the same quiet tries, and
// after the last one the page is given the status but never the server's text.
for (const [code, body] of [[429, { error: { field: 'q', message: TOO_MANY } }],
  [503, { error: { field: 'q', message: 'Search unavailable, try tapping the map instead.' } }]]) {
  test('on the Low data page a ' + code + ' is tried again quietly, then the page gets the status and no server text', async () => {
    const p = setup({ variant: 'low' });
    p.type('Banjara Hills');
    p.advance(300);
    for (let i = 0; i < 3; i++) {
      p.answer(p.requests[i], code, body);
      await settle();
      assert.equal(p.status.hidden, true, 'try ' + (i + 1) + ' failed quietly');
      assert.deepEqual(p.failures, [], 'no sentence is asked for during the quiet tries');
      p.advance(1000);
      assert.equal(p.requests.length, i + 2);
    }
    p.answer(p.requests[3], code, body);
    await settle();
    assert.deepEqual(p.failures, [['', code]]);
    assert.equal(p.status.textContent, FAILED);
    assert.equal(p.outcomes[p.outcomes.length - 1], 'unavailable');
  });
}

test('on the main form the page gets the server text and the status', async () => {
  const p = setup();
  p.type('Banjara Hills');
  p.advance(300);
  for (let i = 0; i < 3; i++) {
    p.answer(p.requests[i], 429, { error: { message: TOO_MANY } });
    await settle();
    p.advance(1000);
  }
  p.answer(p.requests[3], 429, { error: { message: TOO_MANY } });
  await settle();
  assert.deepEqual(p.failures, [[TOO_MANY, 429]]);
});
