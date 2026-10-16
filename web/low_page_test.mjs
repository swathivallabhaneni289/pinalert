// low_page_test.mjs - runs the real web/static/js/app.js and
// web/static/js/low.js together in a Node sandbox and checks the Low data
// view's behaviour the Go source tests cannot:
//
//   - a failed place search on this page shows the page's own sentences,
//     picked by the answer's status, never the server's text (F-P4-12).
//
// Run it with: node --test web/low_page_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the two scripts call: a minimal element model,
// fake timers the test moves by hand, a fetch the test answers by hand and a
// fake shared report form that keeps the location object low.js gives it.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const appSource = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');
const lowSource = fs.readFileSync(path.join(here, 'static', 'js', 'low.js'), 'utf8');

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
    this.disabled = false;
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
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
  remove() { if (this.parentNode) { this.parentNode.removeChild(this); } }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  querySelector() { return null; }
  querySelectorAll() { return []; }
  focus() { this.doc.activeElement = this; }
}

// The ids low.js and app.js look up on the Low data page.
const PAGE_IDS = [
  'low-list', 'low-basis', 'low-retry-position', 'low-stale', 'low-stale-text', 'low-stale-retry',
  'low-skeleton', 'low-empty', 'low-empty-title', 'low-empty-text', 'low-error', 'low-retry', 'low-full',
  'notice-live', 'report-modal', 'low-report', 'low-location-status'
];

// loadLowPage evaluates app.js and then low.js against a Low data page. fetch
// keeps every call for the test to answer; abortRejects says whether an
// abort ends the call's promise, as Chromium does, or leaves it unsettled, as
// WebKit sometimes does. nativeTimeout gives the sandbox AbortSignal.timeout.
function loadLowPage({ abortRejects = true, nativeTimeout = false } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    addEventListener() {},
    removeEventListener() {}
  };
  doc.documentElement = new FakeEl(doc, 'html');
  PAGE_IDS.forEach((id) => { doc.byId[id] = new FakeEl(doc, id === 'low-list' ? 'ul' : 'div', id); });

  // Fake timers: the test moves the clock. Intervals repeat.
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
      if (timer.every) { timer.at += timer.every; } else { timers.delete(id); }
      timer.fn();
    }
    now = end;
  };

  // Every list request is kept with its abort state for the test to answer.
  const requests = [];
  const fetch = (target) => new Promise((resolve, reject) => {
    const signal = target && target.signal;
    const req = { url: target && target.url, resolve, reject, aborted: false };
    if (signal && typeof signal.addEventListener === 'function') {
      signal.addEventListener('abort', () => {
        req.aborted = true;
        if (abortRejects) {
          const err = new Error('aborted');
          err.name = 'AbortError';
          reject(err);
        }
      });
    }
    requests.push(req);
  });
  const answer = (req, body) => req.resolve({
    ok: true,
    status: 200,
    headers: { get: () => null },
    json: () => Promise.resolve(body)
  });

  class FakeSignal {
    constructor() { this.aborted = false; this.listeners = []; }
    addEventListener(type, fn) { if (type === 'abort') { this.listeners.push(fn); } }
    fire() {
      if (this.aborted) { return; }
      this.aborted = true;
      this.listeners.forEach((fn) => fn());
    }
  }
  class AbortController {
    constructor() { this.signal = new FakeSignal(); }
    abort() { this.signal.fire(); }
  }

  const mounted = [];
  const window = {
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { at: now + (ms || 0), fn }); return id; },
    clearTimeout(id) { timers.delete(id); },
    setInterval(fn, ms) { const id = nextId++; timers.set(id, { at: now + ms, fn, every: ms }); return id; },
    clearInterval(id) { timers.delete(id); },
    addEventListener() {},
    localStorage: { getItem: () => null, setItem: () => {} },
    PinalertReportForm: { mount(options) { mounted.push(options); }, open() {} }
  };
  const sandbox = {
    window,
    document: doc,
    navigator: {},
    fetch,
    Request: function Request(target, init) { this.url = target; this.signal = init && init.signal; },
    AbortController,
    Promise,
    Date,
    Math,
    JSON,
    Array,
    Object,
    String,
    Number,
    isFinite,
    isNaN,
    parseFloat,
    parseInt,
    encodeURIComponent
  };
  if (nativeTimeout) {
    // A native timeout signal that never ends the request, the WebKit case.
    sandbox.AbortSignal = { timeout: () => new FakeSignal() };
  }
  sandbox.PinalertReportForm = window.PinalertReportForm;
  vm.createContext(sandbox);
  vm.runInContext(appSource, sandbox);
  sandbox.Pinalert = window.Pinalert;
  vm.runInContext(lowSource, sandbox);
  return { Pinalert: window.Pinalert, el: (id) => doc.byId[id], requests, answer, advance, mounted };
}

test('a refused search on the Low data page shows the main form words, any other failure the Low sentence (F-P4-12)', () => {
  const page = loadLowPage();
  assert.equal(page.mounted.length, 1, 'low.js mounts the shared report form');
  const location = page.mounted[0].location;
  assert.equal(page.mounted[0].variant, 'low');

  const tooMany = 'Too many searches. Wait a few seconds and try again.';
  const unavailable = 'Search unavailable. Try your position again, or open the full version.';
  // The form passes '' as the server's text on this page, and the status.
  assert.equal(location.searchFailureMessage('', 429), tooMany);
  assert.equal(location.searchFailureMessage('', 503), unavailable);
  assert.equal(location.searchFailureMessage('', undefined), unavailable, 'a timeout or no answer');
  assert.equal(location.searchFailureMessage('', 500), unavailable);
  // The server's text is never shown, whatever it says.
  assert.equal(location.searchFailureMessage('Search unavailable, try tapping the map instead.', 503), unavailable);
  assert.equal(location.searchFailureMessage('Anything at all', 429), tooMany);
});
