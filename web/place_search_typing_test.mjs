// place_search_typing_test.mjs - runs the real web/static/js/map-tools.js in
// a Node sandbox and checks the map's place search while the reader types
// (quick 261016-sfu): one search after a 300 ms pause and at least 3 letters,
// a newer search ends the older one, nothing on screen from the reader's own
// typing when a search fails, and Enter still shows every line. Quick
// 261016-sne adds the quiet retry: a 429 or a 503 from typing tries the latest
// text again after the answer's Retry-After or 1 second, at most 3 times, and
// only the failure after the last try shows the line.
//
// Run it with: node --test web/place_search_typing_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what map-tools.js calls: a minimal element model, fake
// timers, a fetch the test answers by hand, and fake Pinalert and PinalertMap.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'map-tools.js'), 'utf8');

const UNAVAILABLE = 'Search unavailable. Try again in a moment.';
const TOO_SHORT = 'Type at least 3 letters.';
const SEARCHING = 'Searching…';

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
    this.className = '';
    this.hidden = false;
    this.value = '';
    this.type = '';
    const self = this;
    this.classList = {
      add: (n) => { if (!self.classes().includes(n)) { self.className = (self.className + ' ' + n).trim(); } },
      remove: (n) => { self.className = self.classes().filter((c) => c !== n).join(' '); },
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
  appendChild(node) { node.parentNode = this; this.children.push(node); return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  dispatch(evt) { (this.handlers[evt.type] || []).slice().forEach((h) => h(evt)); }
  focus() { this.doc.activeElement = this; }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  closest(selector) {
    const cls = selector.slice(1);
    for (let n = this; n; n = n.parentNode) { if (n.classList && n.classList.contains(cls)) { return n; } }
    return null;
  }
  descendants() {
    const out = [];
    for (const c of this.children) { out.push(c, ...c.descendants()); }
    return out;
  }
  querySelectorAll(selector) {
    const cls = selector.slice(1);
    return this.descendants().filter((el) => el.classList.contains(cls));
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
}

function setup() {
  const doc = {
    activeElement: null,
    handlers: {},
    byId: {},
    byClass: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector(selector) { return doc.byClass[selector] || null; },
    addEventListener(type, fn) { (doc.handlers[type] = doc.handlers[type] || []).push(fn); }
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  const tools = make('div', '');
  tools.className = 'map-tools';
  doc.byClass['.map-tools'] = tools;
  const form = make('form', 'map-search', tools);
  make('button', 'map-search-open', tools);
  const input = make('input', 'map-search-input', form);
  make('button', 'map-search-close', form);
  const status = make('p', 'map-search-status', form);
  status.hidden = true;
  const results = make('div', 'map-search-results', form);
  results.hidden = true;
  const outside = make('div', 'outside');

  // Fake timers: the test moves the clock.
  let now = 0;
  let nextId = 1;
  const timers = new Map();
  const window = {
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { at: now + ms, fn }); return id; },
    clearTimeout(id) { timers.delete(id); },
    matchMedia: () => ({ matches: false })
  };
  const advance = (ms) => {
    now += ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= now).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) { return; }
      timers.delete(due[0]);
      due[1].fn();
    }
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

  const Pinalert = {
    state: { category: '', place: null },
    CHIP_LABELS: {},
    setText: (el, v) => { el.textContent = v === null || v === undefined ? '' : String(v); },
    subscribe: () => {},
    setCenter: () => {},
    setPlace: (p) => { Pinalert.state.place = p; },
    dropFocus: () => {},
    fetchReports: () => {},
    setCategory: () => {}
  };
  const PinalertMap = {
    getCenter: () => ({ lat: 17.385, lon: 78.4867 }),
    moveTo: () => {},
    recenterOnReader: () => Promise.resolve({ own: true }),
    cancelRecenter: () => {}
  };
  const sandbox = { window, document: doc, Pinalert, PinalertMap, fetch, AbortController, Array, String, Object, isFinite, encodeURIComponent, parseInt };
  window.Pinalert = Pinalert;
  window.PinalertMap = PinalertMap;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);

  const type = (text) => { input.value = text; input.dispatch({ type: 'input' }); };
  const enter = () => form.dispatch({ type: 'submit', preventDefault() {} });
  const query = (req) => new URL(req.url, 'https://pinalert.example').searchParams.get('q');
  const rows = () => results.querySelectorAll('.map-search__result');
  return { input, status, results, outside, doc, requests, answer, advance, type, enter, query, rows, timers };
}

const settle = () => new Promise((resolve) => setImmediate(resolve));

const hyderabad = { results: [{ name: 'Hyderabad', display_name: 'Hyderabad, Telangana', lat: 17.385, lon: 78.4867 }] };

test('one search after a 300 ms pause, none while typing goes on', async () => {
  const p = setup();
  p.type('Hyd');
  p.advance(200);
  p.type('Hyde');
  p.advance(200);
  p.type('Hyder');
  p.advance(299);
  assert.equal(p.requests.length, 0, 'nothing is sent before a 300 ms pause');
  p.advance(1);
  assert.equal(p.requests.length, 1, 'one search after the pause');
  assert.equal(p.query(p.requests[0]), 'Hyder');
  assert.equal(p.status.hidden, true, 'a typed search shows no Searching line');
  assert.notEqual(p.status.textContent, SEARCHING);

  p.answer(p.requests[0], 200, hyderabad);
  await settle();
  assert.equal(p.rows().length, 1, 'the result list shows');
  assert.equal(p.results.hidden, false);
  assert.equal(p.status.textContent, '1 result.', 'the count is read out the way it is after Enter');
  assert.equal(p.status.classList.contains('visually-hidden'), true, 'the count line is read, not drawn');
});

test('under 3 letters sends nothing and shows no line', () => {
  const p = setup();
  p.type('Hy');
  p.advance(1000);
  assert.equal(p.requests.length, 0);
  assert.equal(p.status.hidden, true);
  assert.notEqual(p.status.textContent, TOO_SHORT);
});

test('a newer search ends the older one and its late answer is dropped', async () => {
  const p = setup();
  p.type('Banj');
  p.advance(300);
  p.type('Banjara');
  p.advance(300);
  assert.equal(p.requests.length, 2);
  assert.equal(p.requests[0].aborted, true, 'the older search is aborted');
  assert.equal(p.requests[1].aborted, false);
  p.answer(p.requests[1], 200, hyderabad);
  await settle();
  assert.equal(p.rows().length, 1);
});

test('a failure from typing shows no Search unavailable', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 503, { error: { message: UNAVAILABLE } });
  await settle();
  assert.notEqual(p.status.textContent, UNAVAILABLE);
  assert.equal(p.status.hidden, true);

  p.type('Banjara H');
  p.advance(300);
  p.answer(p.requests[1], 429, { error: { message: 'Too many searches. Wait a few seconds and try again.' } });
  await settle();
  assert.equal(p.status.hidden, true, 'a refusal from typing shows nothing either');

  p.type('Banjara Hi');
  p.advance(300);
  p.requests[2].reject(new TypeError('network'));
  await settle();
  assert.equal(p.status.hidden, true, 'a network error from typing shows nothing');
});

test('Enter shows Searching and the failure line', async () => {
  const p = setup();
  p.input.value = 'Banjara';
  p.enter();
  assert.equal(p.requests.length, 1);
  assert.equal(p.status.textContent, SEARCHING);
  p.answer(p.requests[0], 503, { error: { message: UNAVAILABLE } });
  await settle();
  assert.equal(p.status.textContent, UNAVAILABLE);
  assert.equal(p.status.hidden, false);
});

test('Enter during a pause searches at once and the pause sends nothing more', () => {
  const p = setup();
  p.type('Banjara');
  p.advance(100);
  p.enter();
  assert.equal(p.requests.length, 1);
  p.advance(1000);
  assert.equal(p.requests.length, 1, 'the pending pause was cancelled');
});

test('Enter for the text already out shows the line without a second request', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  assert.equal(p.requests.length, 1);
  p.enter();
  assert.equal(p.requests.length, 1, 'the same text is not asked twice');
  assert.equal(p.requests[0].aborted, false);
  assert.equal(p.status.textContent, SEARCHING);
  p.answer(p.requests[0], 503, {});
  await settle();
  assert.equal(p.status.textContent, UNAVAILABLE, 'the search Enter took over is no longer quiet');
});

test('Enter with too few letters still says so', () => {
  const p = setup();
  p.input.value = 'Hy';
  p.enter();
  assert.equal(p.status.textContent, TOO_SHORT);
});

test('a press outside with nothing on screen still stops a pending pause', () => {
  const p = setup();
  p.type('Banjara');
  p.advance(100);
  (p.doc.handlers.pointerdown || []).forEach((h) => h({ target: p.outside }));
  p.advance(1000);
  assert.equal(p.requests.length, 0);
});

test('a pause on the text already shown asks nothing again', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 200, hyderabad);
  await settle();
  p.type('Banjara ');
  p.advance(300);
  assert.equal(p.requests.length, 1, 'a trailing space is the same text');
  assert.equal(p.rows().length, 1);
});

test('a press outside during a pause stops it, and a pick stops a typed search', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 200, hyderabad);
  await settle();

  // A pause is pending, then a press outside the search.
  p.type('Banjara Hil');
  (p.doc.handlers.pointerdown || []).forEach((h) => h({ target: p.outside }));
  p.advance(1000);
  assert.equal(p.requests.length, 1, 'the pause was stopped by the press outside');

  // Results shown, a new typed search out, then a pick of a shown row.
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[1], 200, hyderabad);
  await settle();
  p.type('Banjara Hills');
  p.advance(300);
  assert.equal(p.requests.length, 3);
  p.results.dispatch({ type: 'click', target: p.rows()[0] });
  assert.equal(p.requests[2].aborted, true, 'a pick ends the typed search');
  p.advance(1000);
  assert.equal(p.requests.length, 3);
  assert.equal(p.results.hidden, true);
});

const TOO_MANY = 'Too many searches. Wait a few seconds and try again.';
const banjara = { results: [
  { name: 'Banjara Hills', display_name: 'Banjara Hills, Telangana', lat: 17.4156, lon: 78.4347 },
  { name: 'Banjara Hills Road', display_name: 'Banjara Hills Road, Telangana', lat: 17.41, lon: 78.43 }
] };

test('a 429 from typing keeps the list and tries the latest text again after 1 second', async () => {
  const p = setup();
  p.type('Hyderabad');
  p.advance(300);
  p.answer(p.requests[0], 200, hyderabad);
  await settle();
  assert.equal(p.rows().length, 1);

  p.type('Banjara Hills');
  p.advance(300);
  p.answer(p.requests[1], 429, { error: { message: TOO_MANY } });
  await settle();
  assert.equal(p.status.classList.contains('visually-hidden'), true, 'no drawn line, only the earlier count');
  assert.notEqual(p.status.textContent, TOO_MANY);
  assert.equal(p.rows().length, 1, 'the list on screen stays');

  p.advance(999);
  assert.equal(p.requests.length, 2, 'nothing before the wait passes');
  p.advance(1);
  assert.equal(p.requests.length, 3, 'the latest text is tried again');
  assert.equal(p.query(p.requests[2]), 'Banjara Hills');
  p.answer(p.requests[2], 200, banjara);
  await settle();
  assert.equal(p.rows().length, 2, 'suggestions catch up');
});

test("a 503 from typing is tried again after the answer's Retry-After", async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 503, { error: { message: UNAVAILABLE } }, { 'Retry-After': '2' });
  await settle();
  assert.equal(p.status.hidden, true);
  p.advance(1999);
  assert.equal(p.requests.length, 1);
  p.advance(1);
  assert.equal(p.requests.length, 2);
});

test('after 3 quiet tries, a fourth failure shows the failure line', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  for (let i = 0; i < 3; i++) {
    p.answer(p.requests[i], 429, { error: { message: TOO_MANY } });
    await settle();
    assert.equal(p.status.hidden, true, 'failure ' + (i + 1) + ' is quiet');
    p.advance(1000);
    assert.equal(p.requests.length, i + 2);
  }
  p.answer(p.requests[3], 429, { error: { message: TOO_MANY } });
  await settle();
  assert.equal(p.status.hidden, false, 'the last try shows the line');
  assert.equal(p.status.classList.contains('visually-hidden'), false);
  assert.equal(p.status.textContent, TOO_MANY);
  p.advance(5000);
  assert.equal(p.requests.length, 4, 'no try after the last one');
});

test('newer text cancels a pending try and gets its own 3 tries', async () => {
  const p = setup();
  p.type('Banjara H');
  p.advance(300);
  p.answer(p.requests[0], 503, {});
  await settle();
  p.advance(500);
  p.type('Banjara Hills');
  p.advance(300);
  assert.equal(p.requests.length, 2);
  assert.equal(p.query(p.requests[1]), 'Banjara Hills');
  p.advance(2000);
  assert.equal(p.requests.length, 2, 'the older text is never tried again');
  for (let i = 1; i <= 3; i++) {
    p.answer(p.requests[i], 503, {});
    await settle();
    assert.equal(p.status.hidden, true);
    p.advance(1000);
  }
  assert.equal(p.requests.length, 5);
  p.answer(p.requests[4], 503, {});
  await settle();
  assert.equal(p.status.textContent, UNAVAILABLE);
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
  p.answer(p.requests[1], 200, banjara);
  await settle();
  assert.equal(p.rows().length, 2, 'the newer list shows');
  p.advance(3000);
  assert.equal(p.requests.length, 2, 'the older text is never tried again');
  assert.equal(p.rows().length, 2);
});

test('a refusal for older text after the box drops under 3 letters sends nothing', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  // Typing under 3 letters already ends the search out. Here the box changes
  // with no input event, so only the retry's own check of the text in the
  // box keeps the older text from being asked again.
  p.input.value = 'Ba';
  p.answer(p.requests[0], 429, {});
  await settle();
  p.advance(3000);
  assert.equal(p.requests.length, 1);
  assert.equal(p.results.hidden, true);
  assert.equal(p.status.hidden, true);
});

test('Escape and a press outside stop a pending try', async () => {
  for (const stop of ['escape', 'outside']) {
    const p = setup();
    p.type('Banjara');
    p.advance(300);
    p.answer(p.requests[0], 429, {});
    await settle();
    if (stop === 'escape') {
      p.doc.byId['map-search'].dispatch({ type: 'keydown', key: 'Escape' });
    } else {
      (p.doc.handlers.pointerdown || []).forEach((h) => h({ target: p.outside }));
    }
    p.advance(3000);
    assert.equal(p.requests.length, 1, stop + ' stops the pending try');
  }
});

test('Enter during a pending try searches at once and shows every line', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.answer(p.requests[0], 429, {});
  await settle();
  p.enter();
  assert.equal(p.requests.length, 2);
  assert.equal(p.status.textContent, SEARCHING);
  p.answer(p.requests[1], 429, { error: { message: TOO_MANY } });
  await settle();
  assert.equal(p.status.textContent, TOO_MANY, 'Enter is never quiet');
  p.advance(3000);
  assert.equal(p.requests.length, 2, 'Enter is never tried again by itself');
});

test('a network error from typing is not tried again', async () => {
  const p = setup();
  p.type('Banjara');
  p.advance(300);
  p.requests[0].reject(new TypeError('network'));
  await settle();
  p.advance(3000);
  assert.equal(p.requests.length, 1);
  assert.equal(p.status.hidden, true);
});
