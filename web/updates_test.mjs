// updates_test.mjs - runs the real web/static/js/app-bar.js and
// web/static/js/updates.js together in a Node sandbox and checks the bell the
// Go source tests cannot (Phase 3.2, plan 08; D-04, D-06, UI-SPEC A1 to A4):
//
//   - the badge text and the button name for 0, 1 to 9, 10 and 99;
//   - the poll runs on load, on a 30 second timer while the page is visible, on
//     visibilitychange, on pageshow and on the pinalert:updates-refresh event,
//     and a second event during a request queues one more poll and no more;
//   - opening the list clears the count at once, sends the read mark with the
//     largest shown id, draws the rows with their new marks and moves focus to
//     the first link; the list is frozen while open; closing drops the marks;
//   - the loading, error and stale states;
//   - a row is a link only when the server says it is available and its id
//     passes the digits check; every word is written with textContent; the
//     severity class comes from a closed table;
//   - on My activity the Updates rows are marked read once when they are on
//     view (laptop stack on load, the phone part screen), then a poll runs.
//
// Run it with: node --test web/updates_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the two scripts call.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const appBarSource = fs.readFileSync(path.join(here, 'static', 'js', 'app-bar.js'), 'utf8');
const updatesSource = fs.readFileSync(path.join(here, 'static', 'js', 'updates.js'), 'utf8');

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
    this.queries = {};
    this.observers = [];
    this._hidden = false;
  }
  get hidden() { return this._hidden; }
  set hidden(v) {
    const next = Boolean(v);
    if (next === this._hidden) { return; }
    this._hidden = next;
    // A MutationObserver callback runs in a microtask, after the script that
    // changed the attribute has finished, as in a browser.
    this.observers.forEach((cb) => queueMicrotask(() => cb([{ type: 'attributes', attributeName: 'hidden', target: this }])));
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  get firstChild() { return this.children[0] || null; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) { this.children.push(node); node.parentNode = this; return node; }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  removeEventListener(type, fn) { this.handlers[type] = (this.handlers[type] || []).filter((h) => h !== fn); }
  querySelector(selector) { return this.queries[selector] || null; }
  querySelectorAll(selector) { return this.queries[selector + '[]'] || []; }
  focus() { this.doc.activeElement = this; }
  fire(type, evt = {}) {
    const event = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; }, stopPropagation() { this.stopped = true; }, ...evt };
    for (let node = this; node && !event.stopped; node = node.parentNode) {
      (node.handlers[type] || []).slice().forEach((fn) => fn(event));
    }
    return event;
  }
  // find walks the subtree for the first element whose class list holds name.
  find(name) {
    const own = (this.getAttribute('class') || '').split(' ');
    if (own.includes(name)) { return this; }
    for (const child of this.children) {
      const hit = child.find ? child.find(name) : null;
      if (hit) { return hit; }
    }
    return null;
  }
}

class FakeText {
  constructor(text) { this.own = String(text); this.parentNode = null; this.children = []; }
  get textContent() { return this.own; }
  find() { return null; }
}

// load evaluates app-bar.js and then updates.js against the bell markup.
//   hidden:  document.hidden at load.
//   part:    { data: 'updates', rows: [{ id, isNew }] } builds a My activity list.
//   laptop:  the answer of matchMedia('(min-width: 900px)').
function load({ hidden = false, part = null, laptop = false, interval = '30000' } = {}) {
  const doc = {
    activeElement: null,
    hidden,
    referrer: '',
    byId: {},
    queries: {},
    handlers: {},
    captured: new Map(),
    createElement(tag) { return new FakeEl(doc, tag); },
    createTextNode(text) { return new FakeText(text); },
    getElementById(id) { return doc.byId[id] || null; },
    querySelector(selector) { return doc.queries[selector] || null; },
    querySelectorAll() { return []; },
    addEventListener(type, fn, options) {
      (doc.handlers[type] = doc.handlers[type] || []).push(fn);
      doc.captured.set(fn, options === true || (!!options && options.capture === true));
    },
    removeEventListener(type, fn, options) {
      const capture = options === true || (!!options && options.capture === true);
      if (doc.captured.get(fn) !== capture) { return; }
      doc.handlers[type] = (doc.handlers[type] || []).filter((h) => h !== fn);
    },
    fireDoc(type, evt = {}) {
      (doc.handlers[type] || []).slice().forEach((fn) => fn({ type, target: doc, ...evt }));
    }
  };
  doc.documentElement = new FakeEl(doc, 'html');

  const trigger = new FakeEl(doc, 'button', 'bell-trigger');
  trigger.setAttribute('aria-label', 'Updates');
  trigger.setAttribute('aria-expanded', 'false');
  trigger.setAttribute('data-poll-interval-ms', interval);
  const count = new FakeEl(doc, 'span', 'bell-count');
  count.setAttribute('aria-hidden', 'true');
  count.hidden = true;
  trigger.appendChild(count);

  const panel = new FakeEl(doc, 'div', 'bell-list');
  panel.hidden = true;
  const closeButton = new FakeEl(doc, 'button');
  closeButton.setAttribute('class', 'bell-list__close');
  const body = new FakeEl(doc, 'div', 'bell-body');
  const loading = new FakeEl(doc, 'div', 'bell-loading');
  loading.hidden = true;
  const errorBox = new FakeEl(doc, 'div', 'bell-error');
  errorBox.hidden = true;
  const errorRetry = new FakeEl(doc, 'button');
  errorRetry.setAttribute('class', 'bell-list__retry');
  errorBox.appendChild(errorRetry);
  errorBox.queries['.bell-list__retry'] = errorRetry;
  const stale = new FakeEl(doc, 'p', 'bell-stale');
  stale.hidden = true;
  const staleRetry = new FakeEl(doc, 'button');
  staleRetry.setAttribute('class', 'bell-list__retry');
  stale.appendChild(staleRetry);
  stale.queries['.bell-list__retry'] = staleRetry;
  const rows = new FakeEl(doc, 'ul', 'bell-rows');
  rows.hidden = true;
  const empty = new FakeEl(doc, 'p');
  empty.setAttribute('class', 'bell-list__empty');
  empty.hidden = false;
  [loading, errorBox, stale, rows, empty].forEach((el) => body.appendChild(el));
  panel.appendChild(closeButton);
  panel.appendChild(body);
  panel.queries['.bell-list__close'] = closeButton;
  panel.queries['.bell-list__empty'] = empty;
  panel.queries['button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])[]'] = [closeButton];
  doc.byId = {
    'bell-trigger': trigger, 'bell-list': panel, 'bell-count': count, 'bell-body': body,
    'bell-loading': loading, 'bell-error': errorBox, 'bell-stale': stale, 'bell-rows': rows
  };

  if (part) {
    const main = new FakeEl(doc, 'main');
    main.setAttribute('data-part', part.data);
    doc.queries['main.profile-page'] = main;
    doc.queries['.profile-page'] = main;
    const list = new FakeEl(doc, 'ul', 'updates-rows');
    const items = part.rows.map((r) => {
      const li = new FakeEl(doc, 'li');
      li.setAttribute('class', r.isNew ? 'urow urow--new' : 'urow');
      li.setAttribute('data-update-id', String(r.id));
      list.appendChild(li);
      return li;
    });
    list.queries['li[]'] = items;
    list.queries['.urow[]'] = items;
    doc.byId['updates-rows'] = list;
  }

  // Timers: the request helper's 10 second limit and the poll interval.
  const timeouts = [];
  const intervals = [];
  const fetches = [];
  const window = {
    setTimeout(fn, ms) { timeouts.push({ fn, ms, cleared: false }); return timeouts.length; },
    clearTimeout(id) { if (timeouts[id - 1]) { timeouts[id - 1].cleared = true; } },
    setInterval(fn, ms) { intervals.push({ fn, ms }); return intervals.length; },
    clearInterval(id) { if (intervals[id - 1]) { intervals[id - 1].cleared = true; } },
    handlers: {},
    addEventListener(type, fn) { (window.handlers[type] = window.handlers[type] || []).push(fn); },
    matchMedia(query) { return { matches: query === '(min-width: 900px)' ? laptop : false, media: query }; },
    location: { origin: 'https://pinalert.test', search: '' },
    history: { length: 1 }
  };
  const sandbox = {
    window,
    document: doc,
    URL,
    URLSearchParams,
    AbortController,
    MutationObserver: class {
      constructor(cb) { this.cb = cb; }
      observe(el) { el.observers.push(this.cb); }
      disconnect() {}
    },
    fetch: (target, init) => new Promise((resolve, reject) => {
      fetches.push({ url: target, init: init || {}, resolve, reject });
    })
  };
  sandbox.globalThis = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(appBarSource, sandbox, { filename: 'app-bar.js' });
  vm.runInContext(updatesSource, sandbox, { filename: 'updates.js' });

  return {
    doc, window, trigger, count, panel, closeButton, body, loading, errorBox, errorRetry,
    stale, staleRetry, rows, empty, fetches, timeouts, intervals, api: sandbox.window.PinalertUpdates,
    // answer resolves the oldest unanswered request with a JSON body.
    answer(body, status = 200) {
      const f = fetches.find((x) => !x.done);
      assert.ok(f, 'no request is waiting for an answer');
      f.done = true;
      f.resolve({ status, ok: status >= 200 && status < 300, json: () => Promise.resolve(body) });
      return f;
    },
    fail() {
      const f = fetches.find((x) => !x.done);
      assert.ok(f, 'no request is waiting for an answer');
      f.done = true;
      f.reject(new Error('network down'));
      return f;
    },
    pending() { return fetches.filter((x) => !x.done); },
    clickBell() { trigger.fire('click'); },
    pressEscape() { panel.fire('keydown', { key: 'Escape' }); },
    isOpen() { return !panel.hidden; }
  };
}

// flush lets every pending promise settle and every queued microtask run.
function flush() { return new Promise((resolve) => setImmediate(resolve)); }

function row(over = {}) {
  return {
    id: 1, kind: 'reached_live', report_id: 17, available: true, unread: true,
    event: 'Now confirmed', report_line: 'Water over the bridge', category: 'Flood', severity: 'Critical',
    severity_level: 'critical', trust: 'Confirmed by 3 people at different spots.', when: '12 min ago',
    label: 'New. Now confirmed. Flood, Critical. 12 minutes ago', ...over
  };
}


// ---- The words of the badge and the button -----------------------------------

test('badgeText and buttonLabel follow UI-SPEC A1', () => {
  const env = load();
  const { badgeText, buttonLabel } = env.api;
  assert.equal(badgeText(0), '');
  assert.equal(badgeText(1), '1');
  assert.equal(badgeText(9), '9');
  assert.equal(badgeText(10), '9+');
  assert.equal(badgeText(99), '9+');
  assert.equal(buttonLabel(0), 'Updates');
  assert.equal(buttonLabel(1), 'Updates, 1 new');
  assert.equal(buttonLabel(3), 'Updates, 3 new');
  assert.equal(buttonLabel(98), 'Updates, 98 new');
  assert.equal(buttonLabel(99), 'Updates, 99 or more new');
  assert.equal(buttonLabel(150), 'Updates, 99 or more new');
});

// ---- The poll ----------------------------------------------------------------

test('a poll answer sets the badge and the button name, with no live region', async () => {
  const env = load();
  assert.equal(env.fetches.length, 1, 'the bell polls once on load');
  assert.equal(env.fetches[0].url, '/api/updates');
  assert.equal((env.fetches[0].init.method || 'GET'), 'GET');
  env.answer({ unread: 4, updates: [row()] });
  await flush();
  assert.equal(env.count.textContent, '4');
  assert.equal(env.count.hidden, false);
  assert.equal(env.trigger.getAttribute('aria-label'), 'Updates, 4 new');
  assert.equal(env.count.hasAttribute('aria-live'), false);
  assert.equal(env.trigger.hasAttribute('aria-live'), false);
  assert.equal(env.count.getAttribute('aria-hidden'), 'true');
});

test('the badge shows 9+ from ten and is hidden at zero', async () => {
  const env = load();
  env.answer({ unread: 12, updates: [] });
  await flush();
  assert.equal(env.count.textContent, '9+');
  assert.equal(env.trigger.getAttribute('aria-label'), 'Updates, 12 new');
  env.api.refresh();
  env.answer({ unread: 0, updates: [] });
  await flush();
  assert.equal(env.count.hidden, true);
  assert.equal(env.count.textContent, '');
  assert.equal(env.trigger.getAttribute('aria-label'), 'Updates');
});

test('the timer polls every 30000 ms while the page is visible only', async () => {
  const env = load();
  env.answer({ unread: 0, updates: [] });
  await flush();
  assert.equal(env.intervals.length, 1);
  assert.equal(env.intervals[0].ms, 30000);
  env.doc.hidden = true;
  env.intervals[0].fn();
  assert.equal(env.pending().length, 0, 'no poll while the page is hidden');
  env.doc.hidden = false;
  env.intervals[0].fn();
  assert.equal(env.pending().length, 1, 'a poll while the page is visible');
});

test('the interval falls back to 30000 when the attribute is not a number', async () => {
  const env = load({ interval: 'soon' });
  assert.equal(env.intervals[0].ms, 30000);
});

test('the page coming back polls once: visibilitychange and pageshow', async () => {
  const env = load();
  env.answer({ unread: 0, updates: [] });
  await flush();
  env.doc.hidden = true;
  env.doc.fireDoc('visibilitychange');
  assert.equal(env.pending().length, 0, 'a page going hidden does not poll');
  env.doc.hidden = false;
  env.doc.fireDoc('visibilitychange');
  assert.equal(env.pending().length, 1);
  env.answer({ unread: 0, updates: [] });
  await flush();
  (env.window.handlers.pageshow || []).forEach((fn) => fn({ type: 'pageshow' }));
  assert.equal(env.pending().length, 1, 'pageshow polls');
});

test('pinalert:updates-refresh polls once, and a second event queues one more poll only', async () => {
  const env = load();
  env.answer({ unread: 0, updates: [] });
  await flush();
  env.doc.fireDoc('pinalert:updates-refresh');
  assert.equal(env.fetches.length, 2);
  env.doc.fireDoc('pinalert:updates-refresh');
  env.doc.fireDoc('pinalert:updates-refresh');
  assert.equal(env.fetches.length, 2, 'events during a request start no second request');
  env.answer({ unread: 0, updates: [] });
  await flush();
  assert.equal(env.fetches.length, 3, 'exactly one queued poll runs');
  env.answer({ unread: 0, updates: [] });
  await flush();
  assert.equal(env.fetches.length, 3, 'and no more');
});

test('a 401 stops the bell quietly: no timer poll, no error block', async () => {
  const env = load();
  env.answer({ error: { message: 'x' } }, 401);
  await flush();
  assert.equal(env.intervals[0].cleared, true);
  env.doc.fireDoc('pinalert:updates-refresh');
  assert.equal(env.pending().length, 0);
  env.clickBell();
  await flush();
  assert.equal(env.errorBox.hidden, true);
});

// ---- Opening, the read mark and the frozen list --------------------------------

test('opening clears the count, sends the read mark with the largest id and draws the rows', async () => {
  const env = load();
  env.answer({
    unread: 2,
    updates: [row({ id: 9, report_id: 17, unread: true }), row({ id: 7, report_id: 18, unread: true, event: 'Marked resolved' }), row({ id: 3, report_id: 19, unread: false, event: 'Now disputed' })]
  });
  await flush();
  assert.equal(env.count.textContent, '2');
  env.clickBell();
  await flush();
  assert.equal(env.isOpen(), true);
  assert.equal(env.count.hidden, true, 'the count clears at once');
  assert.equal(env.trigger.getAttribute('aria-label'), 'Updates');
  const post = env.pending().find((f) => f.url === '/api/updates/read');
  assert.ok(post, 'the read mark is sent');
  assert.equal(post.init.method, 'POST');
  assert.deepEqual(JSON.parse(post.init.body), { up_to: 9 });
  assert.equal(env.rows.hidden, false);
  assert.equal(env.empty.hidden, true);
  assert.equal(env.rows.children.length, 3);
  const first = env.rows.children[0].children[0];
  assert.equal(first.tagName, 'A');
  assert.equal(first.getAttribute('href'), '/?report=17');
  assert.equal(first.getAttribute('aria-label'), 'New. Now confirmed. Flood, Critical. 12 minutes ago');
  assert.ok(first.getAttribute('class').split(' ').includes('bell-row--new'));
  const third = env.rows.children[2].children[0];
  assert.equal(third.getAttribute('class').split(' ').includes('bell-row--new'), false, 'a read row has no new mark');
  assert.equal(env.doc.activeElement, first, 'focus moves to the first row link');
});

test('no read mark is sent when nothing shown is unread', async () => {
  const env = load();
  env.answer({ unread: 0, updates: [row({ id: 4, unread: false })] });
  await flush();
  env.clickBell();
  await flush();
  assert.equal(env.pending().length, 0);
  assert.equal(env.fetches.length, 1);
});

test('a row shows event, first line, category and severity, trust and time as text', async () => {
  const env = load();
  env.answer({ unread: 0, updates: [row({ unread: false })] });
  await flush();
  env.clickBell();
  await flush();
  const link = env.rows.children[0].children[0];
  assert.equal(link.find('bell-row__event').textContent, 'Now confirmed');
  assert.equal(link.find('bell-row__line').textContent, 'Water over the bridge');
  assert.equal(link.find('bell-row__what').textContent, 'Flood · Critical');
  assert.equal(link.find('bell-row__trust').textContent, 'Confirmed by 3 people at different spots.');
  assert.equal(link.find('bell-row__when').textContent, '12 min ago');
  const sev = link.find('bell-row__sev');
  assert.ok(sev.getAttribute('class').split(' ').includes('sev-critical'));
  assert.equal(sev.getAttribute('aria-hidden'), 'true');
});

test('the severity class comes from a closed table and report text stays text', async () => {
  const env = load();
  env.answer({
    unread: 0,
    updates: [
      row({ id: 3, unread: false, severity_level: 'low', report_line: '<img src=x onerror=alert(1)>' }),
      row({ id: 2, unread: false, severity_level: '"><x onmouseover=1>' }),
      row({ id: 1, unread: false, severity_level: 'medium' })
    ]
  });
  await flush();
  env.clickBell();
  await flush();
  const [a, b, c] = env.rows.children.map((li) => li.children[0]);
  assert.ok(a.find('bell-row__sev').getAttribute('class').split(' ').includes('sev-low'));
  assert.equal(a.find('bell-row__line').textContent, '<img src=x onerror=alert(1)>');
  assert.equal(a.find('bell-row__line').children.length, 0, 'the line holds text, not elements');
  assert.equal(b.find('bell-row__sev').getAttribute('class'), 'bell-row__sev');
  assert.ok(c.find('bell-row__sev').getAttribute('class').split(' ').includes('sev-medium'));
});

test('only an available row with a digits id is a link; the rest are plain', async () => {
  const env = load();
  env.answer({
    unread: 0,
    updates: [
      row({ id: 8, kind: 'more', report_id: undefined, available: true, unread: true, event: 'And 2 more changes', report_line: '', category: '', severity: '', severity_level: '', trust: '', label: 'And 2 more changes. 5 minutes ago', when: '5 min ago' }),
      row({ id: 7, available: false, unread: true, event: 'This report is no longer available', report_line: '', category: '', severity: '', severity_level: '', trust: '' }),
      row({ id: 6, report_id: 0, unread: false }),
      row({ id: 5, report_id: -3, unread: false }),
      row({ id: 4, report_id: 'javascript:alert(1)', unread: false }),
      row({ id: 3, report_id: 1.5, unread: false }),
      row({ id: 2, report_id: 42, unread: false })
    ]
  });
  await flush();
  env.clickBell();
  await flush();
  const els = env.rows.children.map((li) => li.children[0]);
  assert.deepEqual(els.map((e) => e.tagName), ['DIV', 'DIV', 'DIV', 'DIV', 'DIV', 'DIV', 'A']);
  els.slice(0, 6).forEach((e) => {
    assert.equal(e.hasAttribute('href'), false);
    assert.equal(e.hasAttribute('tabindex'), false);
    assert.ok(e.getAttribute('class').split(' ').includes('bell-row--plain'));
    assert.equal(e.getAttribute('class').split(' ').includes('bell-row--new'), false, 'a plain row carries no new mark');
  });
  assert.equal(els[0].find('bell-row__event').textContent, 'And 2 more changes');
  assert.equal(els[1].find('bell-row__event').textContent, 'This report is no longer available');
  assert.equal(els[6].getAttribute('href'), '/?report=42');
});

test('while open the rows are frozen but the badge follows the poll; closing drops the marks', async () => {
  const env = load();
  env.answer({ unread: 1, updates: [row({ id: 5, unread: true })] });
  await flush();
  env.clickBell();
  await flush();
  env.answer({}, 204);
  await flush();
  // The read mark finished; the bell polls again to learn the true count.
  assert.equal(env.pending().length, 1);
  env.answer({ unread: 0, updates: [row({ id: 5, unread: false })] });
  await flush();
  assert.equal(env.count.hidden, true);
  const before = env.rows.children[0].children[0];
  assert.ok(before.getAttribute('class').split(' ').includes('bell-row--new'), 'the row keeps its new mark while open');

  // An update arrives while the list is open.
  env.api.refresh();
  env.answer({ unread: 1, updates: [row({ id: 6, unread: true, event: 'Now disputed' }), row({ id: 5, unread: false })] });
  await flush();
  assert.equal(env.count.hidden, false, 'the badge rises behind the list');
  assert.equal(env.count.textContent, '1');
  assert.equal(env.rows.children.length, 1, 'the rows are not redrawn while open');
  assert.equal(env.rows.children[0].children[0], before);

  // Close, then open again: the new row appears, the old one has no mark.
  env.pressEscape();
  await flush();
  assert.equal(env.isOpen(), false);
  env.clickBell();
  await flush();
  assert.equal(env.rows.children.length, 2);
  const [fresh, old] = env.rows.children.map((li) => li.children[0]);
  assert.ok(fresh.getAttribute('class').split(' ').includes('bell-row--new'));
  assert.equal(old.getAttribute('class').split(' ').includes('bell-row--new'), false);
  const posts = env.fetches.filter((f) => f.url === '/api/updates/read');
  assert.deepEqual(JSON.parse(posts[posts.length - 1].init.body), { up_to: 6 });
});

test('a poll answer that was asked before the read mark finished does not bring the count back', async () => {
  const env = load();
  env.answer({ unread: 2, updates: [row({ id: 5, unread: true })] });
  await flush();
  env.api.refresh();
  env.clickBell();
  await flush();
  // The poll started before the open is still unanswered and still says 2.
  const poll = env.pending().find((f) => f.url === '/api/updates');
  assert.ok(poll);
  poll.done = true;
  poll.resolve({ status: 200, ok: true, json: () => Promise.resolve({ unread: 2, updates: [row({ id: 5, unread: true })] }) });
  await flush();
  assert.equal(env.count.hidden, true);
});

// ---- Loading, error and stale ---------------------------------------------------

test('opened before the first answer: loading, aria-busy, focus stays on the X, then the rows', async () => {
  const env = load();
  env.clickBell();
  await flush();
  assert.equal(env.loading.hidden, false);
  assert.equal(env.body.getAttribute('aria-busy'), 'true');
  assert.equal(env.empty.hidden, true);
  assert.equal(env.doc.activeElement, env.closeButton);
  env.answer({ unread: 1, updates: [row({ id: 5 })] });
  await flush();
  assert.equal(env.loading.hidden, true);
  assert.notEqual(env.body.getAttribute('aria-busy'), 'true');
  assert.equal(env.rows.children.length, 1);
  assert.equal(env.count.hidden, true, 'the count stays cleared');
  const post = env.pending().find((f) => f.url === '/api/updates/read');
  assert.ok(post, 'the read mark is sent for the rows that arrived while open');
  assert.deepEqual(JSON.parse(post.init.body), { up_to: 5 });
  assert.equal(env.doc.activeElement, env.closeButton, 'focus does not jump while the person reads');
});

test('first load failed: the error block shows with Try again, which polls', async () => {
  const env = load();
  env.fail();
  await flush();
  env.clickBell();
  await flush();
  assert.equal(env.errorBox.hidden, false);
  assert.equal(env.empty.hidden, true);
  assert.equal(env.rows.hidden, true);
  env.errorRetry.fire('click');
  assert.equal(env.pending().length, 1, 'Try again polls');
  env.answer({ unread: 0, updates: [] });
  await flush();
  assert.equal(env.errorBox.hidden, true);
  assert.equal(env.empty.hidden, false);
});

test('a failure after a good list shows the stale line above the rows and keeps the count', async () => {
  const env = load();
  env.answer({ unread: 3, updates: [row({ id: 5, unread: false })] });
  await flush();
  env.api.refresh();
  env.fail();
  await flush();
  assert.equal(env.count.textContent, '3', 'the count keeps its last value');
  env.clickBell();
  await flush();
  assert.equal(env.stale.hidden, false);
  assert.equal(env.rows.hidden, false);
  assert.equal(env.errorBox.hidden, true);
  env.staleRetry.fire('click');
  const retry = env.pending().find((f) => f.url === '/api/updates');
  assert.ok(retry, 'Try again polls');
  env.answer({ unread: 0, updates: [row({ id: 5, unread: false })] });
  await flush();
  assert.equal(env.stale.hidden, true);
});

test('an empty list shows the built empty line', async () => {
  const env = load();
  env.answer({ unread: 0, updates: [] });
  await flush();
  env.clickBell();
  await flush();
  assert.equal(env.empty.hidden, false);
  assert.equal(env.rows.hidden, true);
  assert.equal(env.doc.activeElement, env.closeButton);
});

// ---- My activity ------------------------------------------------------------------

test('My activity on a laptop marks the Updates rows read once, then polls', async () => {
  const env = load({ part: { data: '', rows: [{ id: 12, isNew: true }, { id: 14, isNew: false }, { id: 9, isNew: true }] }, laptop: true });
  const post = env.fetches.find((f) => f.url === '/api/updates/read');
  assert.ok(post, 'the part is on view on load');
  assert.deepEqual(JSON.parse(post.init.body), { up_to: 14 });
  assert.equal(env.fetches.filter((f) => f.url === '/api/updates').length, 0, 'the poll waits for the mark');
  env.answer({}, 204);
  await flush();
  assert.equal(env.fetches.filter((f) => f.url === '/api/updates').length, 1, 'then one poll');
  assert.equal(env.fetches.filter((f) => f.url === '/api/updates/read').length, 1, 'the mark is sent once');
});

test('My activity on a phone marks the rows read only on the Updates part screen', async () => {
  const rowsIn = [{ id: 3, isNew: true }];
  const list = load({ part: { data: '', rows: rowsIn }, laptop: false });
  assert.equal(list.fetches.some((f) => f.url === '/api/updates/read'), false, 'the hub list does not mark');
  const other = load({ part: { data: 'posted', rows: rowsIn }, laptop: false });
  assert.equal(other.fetches.some((f) => f.url === '/api/updates/read'), false);
  const screen = load({ part: { data: 'updates', rows: rowsIn }, laptop: false });
  assert.deepEqual(JSON.parse(screen.fetches.find((f) => f.url === '/api/updates/read').init.body), { up_to: 3 });
});

test('My activity sends no mark when no row is new', async () => {
  const env = load({ part: { data: 'updates', rows: [{ id: 3, isNew: false }] }, laptop: true });
  assert.equal(env.fetches.some((f) => f.url === '/api/updates/read'), false);
  assert.equal(env.fetches.filter((f) => f.url === '/api/updates').length, 1);
});

test('a page without the bell does nothing', () => {
  const sandbox = { window: {}, document: { getElementById: () => null, addEventListener() {} } };
  vm.createContext(sandbox);
  assert.doesNotThrow(() => vm.runInContext(updatesSource, sandbox, { filename: 'updates.js' }));
});
