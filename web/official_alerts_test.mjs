// official_alerts_test.mjs - runs the real web/static/js/official-alerts.js in a
// Node sandbox and checks the Official alerts page of Phase 5 plan 09 (ALERT-01,
// ALERT-02, ALERT-04, ROBUST-06, D-06 to D-10, UI-SPEC Surface 1):
//
//   - states: a fresh answer with rows shows the status "Last checked 4 minutes
//     ago", the section "In force now" with its count and the districts line;
//     fresh and empty is the only state that says no official alert is in
//     force; stale shows the notice, "Last known alerts" and "As of <time>";
//     unavailable and never checked show their own words and no list; area
//     unknown and a failed load have their own words, and Try again refetches
//     once, disabled while the request is out;
//   - precedence: load error, then unavailable, then area unknown, then fresh
//     or stale;
//   - the honesty grid: in every state other than fresh and empty, and in
//     fresh with any row, the visible text never holds "No official alerts in
//     force for your area", "No alerts", "all clear" or "nothing in force",
//     including when the page is drawn again after a fresh and empty answer;
//   - loading shows the skeleton and "Checking for official alerts" only;
//   - ticking: the minutes move once a minute from the server's age plus the
//     time that passed here, with no new request;
//   - relative words: just now, 1 minute ago, N minutes ago, 1 hour ago, N hours
//     ago (and days), and for a row "Just now", "N min ago";
//   - rows: the level chip word and class (Extreme, Severe, else neutral), the
//     Official badge, the headline, the area text, the issuer, "Issued 6 min
//     ago", "Until 8:35 pm", "Until about 8:35 pm" when inferred, "Sun 8:35 pm"
//     when not today, "45 alerts like this" on a grouped row; a Demo row has
//     the Demo tag and no badge; the original language tag; the groups come in
//     the order the server sends them;
//   - details: pressing a row toggles aria-expanded and its panel, which draws
//     only the parts present: What it says, What to do, Original text (Telugu)
//     with a lang attribute when the code maps, Areas, Source, No end time was
//     given., and the grouped line;
//   - feed text enters the page through textContent only: a headline of
//     "<img src=x onerror=1>" is text, never an element;
//   - area source: the page's fixed point (the Demo), then the tab's Nearby
//     centre when it belongs to this page's account, then the stored device
//     fix, else none; lat and lon go together or not at all; a value from
//     another account or out of range is ignored; the API base is
//     window.PinalertApiBase, else the page's data-api-base, else /api.
//
// Run it with: node --test web/official_alerts_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what official-alerts.js calls: a minimal element model,
// a clock and interval timer the test moves by hand and a fetch the test
// answers by hand. Clock times are read in Asia/Kolkata so the expected words
// are stable on any machine.

process.env.TZ = 'Asia/Kolkata';

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'official-alerts.js'), 'utf8');

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
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML must never be used: ' + v); }
  insertAdjacentHTML(where, v) { throw new Error('insertAdjacentHTML must never be used: ' + where + v); }
  get hidden() { return Object.hasOwn(this.attrs, 'hidden'); }
  set hidden(v) { if (v) { this.attrs.hidden = ''; } else { delete this.attrs.hidden; } }
  setAttribute(n, v) {
    this.attrs[n] = String(v);
    if (n === 'id') { this.id = String(v); this.doc.byId[this.id] = this; }
    if (n === 'class') { this.className = String(v); }
  }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  appendChild(node) {
    if (node.parentNode) { node.parentNode.removeChild(node); }
    this.children.push(node);
    node.parentNode = this;
    if (node.id) { this.doc.byId[node.id] = node; }
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
  focus() { this.doc.activeElement = this; }
}

const hasClass = (el, name) => (el.className || '').split(/\s+/).includes(name);

// walk visits el and every element below it.
function walk(el, fn) {
  fn(el);
  el.children.forEach((c) => walk(c, fn));
}

const findAll = (root, pred) => {
  const found = [];
  walk(root, (el) => { if (pred(el)) { found.push(el); } });
  return found;
};
const byClass = (root, name) => findAll(root, (el) => hasClass(el, name));

// visibleText is the text a reader sees: hidden subtrees are left out.
function visibleText(el) {
  if (el.hidden) { return ''; }
  return [el.own, ...el.children.map(visibleText)].join(' ');
}

const ids = [
  'official-status', 'official-notice', 'official-notice-text', 'official-section', 'official-title',
  'official-districts', 'official-list', 'official-skeleton', 'official-empty', 'official-empty-title',
  'official-empty-body', 'official-unavailable', 'official-unavailable-title', 'official-unavailable-body',
  'official-noarea', 'official-noarea-title', 'official-noarea-body', 'official-error', 'official-error-title',
  'official-error-body', 'official-retry', 'official-attribution'
];

// buildPage makes the markup of alerts.html.tmpl as the ids official-alerts.js
// reads: every state block starts hidden, as the template ships them.
function buildPage({ withMarkup = true, account = '7', fixedLat, fixedLon, dataApiBase, hazardLine = false } = {}) {
  const doc = {
    activeElement: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    addEventListener() {},
    querySelector() { return null; }
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  doc.body = make('body');
  if (!withMarkup) { return { doc }; }
  const main = make('main', 'official-page', doc.body);
  if (account !== null) { main.setAttribute('data-account-id', account); }
  if (fixedLat !== undefined) { main.setAttribute('data-fixed-lat', fixedLat); }
  if (fixedLon !== undefined) { main.setAttribute('data-fixed-lon', fixedLon); }
  if (dataApiBase !== undefined) { main.setAttribute('data-api-base', dataApiBase); }
  // Nest the parts the way the template does, so a hidden block hides its words.
  const parentOf = {
    'official-notice-text': 'official-notice',
    'official-title': 'official-section', 'official-districts': 'official-section', 'official-list': 'official-section',
    'official-empty-title': 'official-empty', 'official-empty-body': 'official-empty',
    'official-unavailable-title': 'official-unavailable', 'official-unavailable-body': 'official-unavailable',
    'official-noarea-title': 'official-noarea', 'official-noarea-body': 'official-noarea',
    'official-error-title': 'official-error', 'official-error-body': 'official-error', 'official-retry': 'official-error'
  };
  const startHidden = ['official-notice', 'official-section', 'official-skeleton', 'official-empty',
    'official-unavailable', 'official-noarea', 'official-error'];
  for (const id of ids) {
    const tag = id === 'official-retry' ? 'button' : (/-title$/.test(id) && id !== 'official-title' ? 'h2' : 'div');
    make(tag, id, parentOf[id] ? doc.byId[parentOf[id]] : main);
    if (startHidden.includes(id)) { doc.byId[id].hidden = true; }
  }
  doc.byId['official-retry'].textContent = 'Try again';
  // Amended 2026-10-29 by plan 05-22: the second muted status line (hazard events
  // from GDACS) is drawn only by a page that carries it, as alerts.html.tmpl does
  // and the Demo's template does not, so the older cases above run on a page
  // without it and prove the script still works then.
  if (hazardLine) { make('p', 'official-hazard-status', main); }
  return { doc, main };
}

// The clock: 4:04 pm in Asia/Kolkata on Thursday 29 October 2026.
const T0 = Date.parse('2026-10-29T10:34:00Z');

function loadPage(options = {}) {
  const page = buildPage(options);
  const requests = [];
  let now = T0;
  let nextId = 1;
  const intervals = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...intervals.entries()].filter(([, t]) => t.next <= end).sort((a, b) => a[1].next - b[1].next)[0];
      if (!due) { break; }
      const [, timer] = due;
      now = timer.next;
      timer.next += timer.every;
      timer.fn();
    }
    now = end;
  };
  const RealDate = Date;
  class FakeDate extends RealDate {
    static now() { return now; }
    constructor(...args) { if (args.length === 0) { super(now); } else { super(...args); } }
  }
  const store = new Map();
  const sessionStorage = {
    getItem(k) { return store.has(k) ? store.get(k) : null; },
    setItem(k, v) { store.set(k, String(v)); },
    removeItem(k) { store.delete(k); }
  };
  const sandbox = {
    console,
    document: page.doc,
    Date: FakeDate,
    sessionStorage: options.noStorage ? { getItem() { throw new Error('blocked'); } } : sessionStorage,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setInterval(fn, every) { const id = nextId++; intervals.set(id, { fn, every, next: now + every }); return id; },
    clearInterval(id) { intervals.delete(id); },
    setTimeout(fn) { fn(); return 0; }
  };
  if (options.position !== undefined) { sandbox.PinalertPosition = { read() { return options.position; } }; }
  if (options.apiBase !== undefined) { sandbox.PinalertApiBase = options.apiBase; }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'official-alerts.js' });
  return { ...page, api: sandbox.PinalertOfficialAlerts, requests, advance, sandbox, sessionStorage, intervals };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: body === undefined ? () => Promise.reject(new Error('no body')) : () => Promise.resolve(body)
});

const el = (page, id) => page.doc.byId[id];
const fire = (target, type, extra) => {
  const event = { type, target, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (target.handlers[type] || []).forEach((h) => h(event));
  return event;
};

// Fresh: the last good check at 4:00 pm local, 240 seconds before the clock.
const FRESH_AT = '2026-10-29T10:30:00Z';

function row(over = {}) {
  return {
    id: 41,
    source: 'sachet',
    level: 'extreme',
    level_word: 'Extreme',
    badge: 'Official',
    demo: false,
    headline: 'Cyclone warning for the coast',
    original_language_only: false,
    area_text: '6 mandals',
    issuer: 'IMD Chennai',
    issued_at: '2026-10-29T10:28:00Z',
    expires_at: '2026-10-29T15:05:00Z',
    expires_inferred: false,
    grouped_count: 1,
    details: {
      description: '', instruction: '', original_headline: '', original_description: '',
      original_lang_label: '', original_lang_code: '', areas: '', source_line: 'NDMA SACHET'
    },
    ...over
  };
}

function view(over = {}) {
  return {
    state: 'fresh',
    age_seconds: 240,
    last_success_at: FRESH_AT,
    area_known: true,
    groups: [],
    count: 0,
    districts_line: 'Ntr district',
    banner: null,
    ...over
  };
}

const group = (kind, label, rows) => ({ kind, label, rows });

const withRows = (over = {}) => view({
  groups: [group('reader', 'Where you are: Ntr district', [row(), row({ id: 42, level: 'severe', level_word: 'Severe', headline: 'Heavy rain' })])],
  count: 2,
  ...over
});

const staleAt = () => ({ state: 'stale', age_seconds: 7200, last_success_at: '2026-10-29T08:35:00Z' });

// open loads the page and answers its first request with body.
async function open(options, body, status = 200) {
  const page = loadPage(options);
  await flush();
  assert.equal(page.requests.length, 1, 'the page makes one request on load');
  answer(page.requests[0], status, body);
  await flush();
  return page;
}

const status = (page) => el(page, 'official-status').textContent;
const shown = (page, id) => !el(page, id).hidden;

const CLAIMS = [/no official alerts in force for your area/i, /no alerts/i, /all clear/i, /nothing in force/i];
const assertNoClaim = (page, label) => {
  const text = visibleText(page.doc.body);
  for (const claim of CLAIMS) {
    assert.doesNotMatch(text, claim, label + ': the visible text holds ' + claim);
  }
};

test('the script leaves quietly on a page without the markup', () => {
  const page = loadPage({ withMarkup: false });
  assert.equal(page.requests.length, 0);
});

test('loading shows the skeleton and the checking words and nothing else', async () => {
  const page = loadPage();
  await flush();
  assert.equal(status(page), 'Checking for official alerts…');
  assert.equal(shown(page, 'official-skeleton'), true);
  for (const id of ['official-section', 'official-empty', 'official-unavailable', 'official-noarea', 'official-error', 'official-notice']) {
    assert.equal(shown(page, id), false, id + ' stays hidden while loading');
  }
  assertNoClaim(page, 'loading');
});

test('fresh with alerts: status, title with count, districts line and groups', async () => {
  const page = await open({}, withRows());
  assert.equal(status(page), 'Last checked 4 minutes ago');
  assert.equal(shown(page, 'official-section'), true);
  assert.equal(shown(page, 'official-skeleton'), false);
  assert.equal(shown(page, 'official-notice'), false);
  assert.equal(el(page, 'official-title').textContent, 'In force now 2');
  assert.equal(el(page, 'official-districts').textContent, 'Ntr district');
  const labels = findAll(el(page, 'official-list'), (e) => e.tagName === 'H3').map((e) => e.textContent);
  assert.deepEqual(labels, ['Where you are: Ntr district']);
  assertNoClaim(page, 'fresh with rows');
});

test('groups are drawn in the order the server sends them', async () => {
  const body = view({
    groups: [
      group('reader', 'Where you are: Ntr district', [row({ id: 1 })]),
      group('home', 'Your home area: Guntur district', [row({ id: 2 })]),
      group('state', 'Across Andhra Pradesh', [row({ id: 3 })]),
      group('state', 'Across Telangana', [row({ id: 5 })]),
      group('unplaced', 'Could not be placed on a map', [row({ id: 4 })])
    ],
    count: 5
  });
  const page = await open({}, body);
  const labels = findAll(el(page, 'official-list'), (e) => e.tagName === 'H3').map((e) => e.textContent);
  assert.deepEqual(labels, [
    'Where you are: Ntr district', 'Your home area: Guntur district', 'Across Andhra Pradesh',
    'Across Telangana', 'Could not be placed on a map'
  ]);
});

test('fresh and empty is the only state that says no official alert is in force', async () => {
  const page = await open({}, view());
  assert.equal(status(page), 'Last checked 4 minutes ago');
  assert.equal(shown(page, 'official-empty'), true);
  assert.equal(el(page, 'official-empty-title').textContent, 'No official alerts in force for your area');
  assert.equal(el(page, 'official-empty-body').textContent,
    'Nothing is in force for where you are or for your home area. Check your state disaster management authority for anything else.');
  assert.equal(shown(page, 'official-section'), false);
  assert.equal(shown(page, 'official-notice'), false);
});

test('stale: notice, Last known alerts, As of, and never In force now', async () => {
  const page = await open({}, withRows(staleAt()));
  assert.equal(status(page), 'Last checked 2 hours ago');
  assert.equal(shown(page, 'official-notice'), true);
  assert.equal(el(page, 'official-notice-text').textContent, 'We could not check for alerts since 2:05 pm (2 hours ago).');
  assert.equal(el(page, 'official-title').textContent, 'Last known alerts 2');
  assert.equal(el(page, 'official-districts').textContent, 'As of 2:05 pm');
  assert.doesNotMatch(visibleText(page.doc.body), /in force now/i);
  assert.equal(shown(page, 'official-empty'), false);
  assertNoClaim(page, 'stale with rows');
});

test('stale with no rows keeps the notice and the heading and says nothing reassuring', async () => {
  const page = await open({}, view(staleAt()));
  assert.equal(shown(page, 'official-notice'), true);
  assert.equal(shown(page, 'official-empty'), false);
  assert.equal(shown(page, 'official-section'), true);
  assert.equal(el(page, 'official-title').textContent, 'Last known alerts');
  assert.equal(el(page, 'official-districts').textContent, 'As of 2:05 pm');
  assert.equal(shown(page, 'official-list'), false);
  assertNoClaim(page, 'stale and empty');
});

test('unavailable with a last good time says it could not be checked and lists nothing', async () => {
  const page = await open({}, view({ state: 'unavailable', age_seconds: 25200, last_success_at: '2026-10-29T03:40:00Z' }));
  assert.equal(status(page), 'Last checked 7 hours ago');
  assert.equal(shown(page, 'official-unavailable'), true);
  assert.equal(el(page, 'official-unavailable-title').textContent, 'Official alerts could not be checked');
  assert.equal(el(page, 'official-unavailable-body').textContent,
    'Pinalert could not reach the official feed since 9:10 am (7 hours ago). Check your state disaster management authority for warnings.');
  assert.equal(shown(page, 'official-section'), false);
  assertNoClaim(page, 'unavailable');
});

test('never checked says Not checked yet and the never body', async () => {
  const page = await open({}, view({ state: 'unavailable', age_seconds: null, last_success_at: null }));
  assert.equal(status(page), 'Not checked yet');
  assert.equal(el(page, 'official-unavailable-title').textContent, 'Official alerts could not be checked');
  assert.equal(el(page, 'official-unavailable-body').textContent,
    'Pinalert has not been able to check the official feed yet. Check your state disaster management authority for warnings.');
  assertNoClaim(page, 'never checked');
});

test('area unknown says the page needs the area, not that nothing is in force', async () => {
  const page = await open({}, view({ area_known: false }));
  assert.equal(shown(page, 'official-noarea'), true);
  assert.equal(el(page, 'official-noarea-title').textContent, 'Official alerts need your area');
  assert.equal(el(page, 'official-noarea-body').textContent,
    'Open Nearby and allow location, or add a home area in Settings, then come back.');
  assert.equal(shown(page, 'official-empty'), false);
  assert.equal(shown(page, 'official-section'), false);
  assertNoClaim(page, 'area unknown');
});

test('precedence: unavailable beats area unknown, area unknown beats the empty claim', async () => {
  const page = await open({}, view({ state: 'unavailable', area_known: false, age_seconds: 25200, last_success_at: '2026-10-29T03:40:00Z' }));
  assert.equal(shown(page, 'official-unavailable'), true);
  assert.equal(shown(page, 'official-noarea'), false);
  assertNoClaim(page, 'unavailable and area unknown');
  const stale = await open({}, view({ ...staleAt(), area_known: false }));
  assert.equal(shown(stale, 'official-noarea'), true);
  assert.equal(shown(stale, 'official-notice'), true);
  assertNoClaim(stale, 'stale and area unknown');
});

test('the honesty grid: no state but fresh and empty ever shows the claim, even after it was shown', async () => {
  const page = await open({}, view());
  assert.match(visibleText(page.doc.body), CLAIMS[0]);
  const shapes = [
    ['fresh with rows', withRows()],
    ['fresh, area unknown', view({ area_known: false })],
    ['stale with rows', withRows(staleAt())],
    ['stale, empty', view(staleAt())],
    ['stale, area unknown', view({ ...staleAt(), area_known: false })],
    ['unavailable', view({ state: 'unavailable', age_seconds: 25200, last_success_at: '2026-10-29T03:40:00Z' })],
    ['unavailable, area unknown', view({ state: 'unavailable', area_known: false, age_seconds: null, last_success_at: null })],
    ['never checked', view({ state: 'unavailable', age_seconds: null, last_success_at: null })]
  ];
  for (const [label, body] of shapes) {
    page.api.render(body);
    assertNoClaim(page, label);
    assert.equal(shown(page, 'official-empty'), false, label + ': the empty block is hidden');
  }
  page.api.render(view());
  assert.match(visibleText(page.doc.body), CLAIMS[0], 'fresh and empty still says it');
});

test('a failed load shows the error words and Try again fetches once, disabled while out', async () => {
  const page = await open({}, { error: 'nope' }, 503);
  assert.equal(status(page), 'Not checked');
  assert.equal(shown(page, 'official-error'), true);
  assert.equal(el(page, 'official-error-title').textContent, "Couldn't load official alerts");
  assert.equal(el(page, 'official-error-body').textContent, 'Check your connection and try again.');
  assert.equal(el(page, 'official-retry').textContent, 'Try again');
  assertNoClaim(page, 'load error');

  fire(el(page, 'official-retry'), 'click');
  await flush();
  assert.equal(page.requests.length, 2);
  assert.equal(el(page, 'official-retry').disabled, true);
  fire(el(page, 'official-retry'), 'click');
  await flush();
  assert.equal(page.requests.length, 2, 'a press while the request is out does nothing');
  answer(page.requests[1], 200, withRows());
  await flush();
  assert.equal(shown(page, 'official-error'), false);
  assert.equal(el(page, 'official-title').textContent, 'In force now 2');
  assert.equal(el(page, 'official-retry').disabled, false);
});

test('a network failure is the same error state', async () => {
  const page = loadPage();
  await flush();
  page.requests[0].reject(new Error('offline'));
  await flush();
  assert.equal(shown(page, 'official-error'), true);
  assert.equal(status(page), 'Not checked');
});

test('the minutes move once a minute with no new request', async () => {
  const page = await open({}, withRows());
  assert.equal(status(page), 'Last checked 4 minutes ago');
  page.advance(60000);
  assert.equal(status(page), 'Last checked 5 minutes ago');
  page.advance(60000);
  assert.equal(status(page), 'Last checked 6 minutes ago');
  assert.equal(page.requests.length, 1);
});

test('the stale sentence keeps its age in step too', async () => {
  const page = await open({}, withRows(staleAt()));
  page.advance(60 * 60000);
  assert.equal(el(page, 'official-notice-text').textContent, 'We could not check for alerts since 2:05 pm (3 hours ago).');
  assert.equal(status(page), 'Last checked 3 hours ago');
});

test('relative words', () => {
  const api = loadPage({ withMarkup: false, account: null }).api;
  assert.equal(api, undefined, 'no markup, no export');
  const page = loadPage();
  const words = (s) => {
    page.api.renderStatus(view({ age_seconds: s }), 0);
    return status(page);
  };
  assert.equal(words(0), 'Last checked just now');
  assert.equal(words(59), 'Last checked just now');
  assert.equal(words(60), 'Last checked 1 minute ago');
  assert.equal(words(119), 'Last checked 1 minute ago');
  assert.equal(words(240), 'Last checked 4 minutes ago');
  assert.equal(words(3599), 'Last checked 59 minutes ago');
  assert.equal(words(3600), 'Last checked 1 hour ago');
  assert.equal(words(7200), 'Last checked 2 hours ago');
  assert.equal(words(25200), 'Last checked 7 hours ago');
  assert.equal(words(-30), 'Last checked just now');
  page.api.renderStatus(view({ age_seconds: null, last_success_at: null }), 0);
  assert.equal(status(page), 'Not checked yet');
  assert.equal(page.api.relativeIssued(5), 'Just now');
  assert.equal(page.api.relativeIssued(-100), 'Just now');
  assert.equal(page.api.relativeIssued(360), '6 min ago');
  assert.equal(page.api.relativeIssued(3600), '1 hour ago');
  assert.equal(page.api.relativeIssued(10800), '3 hours ago');
});

test('until: a clock time, about when inferred, a weekday when not today', () => {
  const page = loadPage();
  assert.equal(page.api.untilText('2026-10-29T15:05:00Z', false), 'Until 8:35 pm');
  assert.equal(page.api.untilText('2026-10-29T15:05:00Z', true), 'Until about 8:35 pm');
  assert.equal(page.api.untilText('2026-11-01T15:05:00Z', false), 'Until Sun 8:35 pm');
  assert.equal(page.api.untilText('2026-10-29T18:25:00Z', false), 'Until 11:55 pm');
  assert.equal(page.api.untilText('2026-10-29T18:35:00Z', false), 'Until Fri 12:05 am');
});

const rowsOf = (page) => byClass(el(page, 'official-list'), 'official-row');
const textOf = (node) => node.textContent.replace(/\s+/g, ' ').trim();

test('a row shows chip, badge, headline, area, issuer, issued and until', async () => {
  const page = await open({}, withRows());
  const [first, second] = rowsOf(page);
  const text = textOf(first);
  for (const want of ['Extreme', 'Official', 'Cyclone warning for the coast', '6 mandals', 'IMD Chennai', 'Issued 6 min ago', 'Until 8:35 pm']) {
    assert.ok(text.includes(want), 'row text holds ' + want + ': ' + text);
  }
  assert.equal(first.tagName, 'BUTTON');
  assert.equal(first.getAttribute('aria-expanded'), 'false');
  assert.ok(first.getAttribute('aria-controls'));
  const chip = byClass(first, 'official-chip')[0];
  assert.ok(hasClass(chip, 'official-chip--extreme'));
  assert.equal(textOf(chip), 'Extreme');
  assert.ok(hasClass(byClass(second, 'official-chip')[0], 'official-chip--severe'));
  assert.equal(byClass(first, 'official-badge')[0].textContent.trim(), 'Official');
});

test('moderate, minor and unknown share the neutral chip', async () => {
  const body = view({
    groups: [group('reader', 'Where you are: Ntr district', ['moderate', 'minor', 'unknown', 'surprise'].map((level, i) =>
      row({ id: 10 + i, level, level_word: level })))],
    count: 4
  });
  const page = await open({}, body);
  for (const r of rowsOf(page)) {
    const chip = byClass(r, 'official-chip')[0];
    assert.ok(hasClass(chip, 'official-chip--neutral'), 'neutral chip for ' + textOf(chip));
  }
});

test('until shows about when inferred and a weekday when not today', async () => {
  const body = view({
    groups: [group('reader', 'Where you are: Ntr district', [
      row({ id: 1, expires_inferred: true }),
      row({ id: 2, expires_at: '2026-11-01T15:05:00Z' })
    ])],
    count: 2
  });
  const page = await open({}, body);
  const [a, b] = rowsOf(page);
  assert.ok(textOf(a).includes('Until about 8:35 pm'), textOf(a));
  assert.ok(textOf(b).includes('Until Sun 8:35 pm'), textOf(b));
});

test('a grouped row says how many alerts are like it', async () => {
  const body = view({
    groups: [group('state', 'Across Andhra Pradesh', [row({ id: 9, level: 'severe', level_word: 'Severe', headline: 'Lightning', grouped_count: 45 })])],
    count: 1
  });
  const page = await open({}, body);
  assert.ok(textOf(rowsOf(page)[0]).includes('45 alerts like this'));
});

test('a Demo row has the Demo tag and no Official badge', async () => {
  const body = view({
    groups: [group('reader', 'Where you are: Ntr district', [row({ demo: true, badge: '' })])],
    count: 1
  });
  const page = await open({}, body);
  const r = rowsOf(page)[0];
  assert.equal(byClass(r, 'demo-tag').length, 1);
  assert.equal(textOf(byClass(r, 'demo-tag')[0]), 'Demo');
  assert.equal(byClass(r, 'official-badge').length, 0);
  assert.doesNotMatch(textOf(r), /Official/);
});

test('an original language only row carries the Original language tag and its lang', async () => {
  const body = view({
    groups: [group('reader', 'Where you are: Ntr district', [row({
      original_language_only: true,
      headline: 'తుఫాను హెచ్చరిక',
      details: { ...row().details, original_lang_label: 'Telugu', original_lang_code: 'te' }
    })])],
    count: 1
  });
  const page = await open({}, body);
  const r = rowsOf(page)[0];
  assert.ok(textOf(r).includes('Original language'));
  const headline = byClass(r, 'official-row__headline')[0];
  assert.equal(headline.getAttribute('lang'), 'te');
});

test('feed text is text: markup in a headline never becomes an element', async () => {
  const payload = '<img src=x onerror=1>';
  const body = view({
    groups: [group('reader', 'Where you are: <b>Ntr</b>', [row({
      headline: payload, area_text: payload, issuer: payload,
      details: { ...row().details, description: payload, instruction: payload, areas: payload, source_line: payload }
    })])],
    count: 1
  });
  const page = await open({}, body);
  const r = rowsOf(page)[0];
  assert.ok(textOf(r).includes(payload));
  const tags = new Set();
  walk(el(page, 'official-list'), (e) => tags.add(e.tagName));
  assert.ok(!tags.has('IMG') && !tags.has('B'), 'no element came from feed text');
  fire(r, 'click');
  const details = el(page, r.getAttribute('aria-controls'));
  assert.ok(details.textContent.includes(payload));
});

test('pressing a row toggles its panel and aria-expanded', async () => {
  const page = await open({}, withRows());
  const r = rowsOf(page)[0];
  const panel = el(page, r.getAttribute('aria-controls'));
  assert.equal(panel.hidden, true);
  fire(r, 'click');
  assert.equal(r.getAttribute('aria-expanded'), 'true');
  assert.equal(panel.hidden, false);
  fire(r, 'click');
  assert.equal(r.getAttribute('aria-expanded'), 'false');
  assert.equal(panel.hidden, true);
  const second = rowsOf(page)[1];
  assert.notEqual(second.getAttribute('aria-controls'), r.getAttribute('aria-controls'));
});

test('the panel draws only the parts the feed carried', async () => {
  const none = await open({}, withRows());
  const bare = rowsOf(none)[0];
  fire(bare, 'click');
  const bareText = textOf(el(none, bare.getAttribute('aria-controls')));
  assert.ok(bareText.includes('Source'));
  assert.ok(bareText.includes('NDMA SACHET'));
  for (const absent of ['What it says', 'What to do', 'Original text', 'Areas', 'No end time was given.', 'alerts like this']) {
    assert.ok(!bareText.includes(absent), 'a bare panel does not hold ' + absent);
  }

  const full = view({
    groups: [group('reader', 'Where you are: Ntr district', [row({
      expires_inferred: true,
      grouped_count: 45,
      details: {
        description: 'Heavy rain.', instruction: 'Stay indoors.', original_headline: 'భారీ వర్షం',
        original_description: 'భారీ వర్షం కురుస్తుంది', original_lang_label: 'Telugu', original_lang_code: 'te',
        areas: 'Vijayawada, Guntur', source_line: 'NDMA SACHET'
      }
    })])],
    count: 1
  });
  const page = await open({}, full);
  const r = rowsOf(page)[0];
  fire(r, 'click');
  const panel = el(page, r.getAttribute('aria-controls'));
  const text = textOf(panel);
  for (const want of [
    'What it says', 'Heavy rain.', 'What to do', 'Stay indoors.', 'Original text (Telugu)', 'భారీ వర్షం',
    'Areas', 'Vijayawada, Guntur', 'Source', 'NDMA SACHET', 'No end time was given.',
    '45 alerts like this are in force. This shows the newest.'
  ]) {
    assert.ok(text.includes(want), 'panel holds ' + want + ': ' + text);
  }
  const langs = findAll(panel, (e) => e.getAttribute('lang') === 'te');
  assert.ok(langs.length >= 1, 'the original text carries lang te');
});

test('a language code that does not map leaves lang unset', async () => {
  const body = view({
    groups: [group('reader', 'Where you are: Ntr district', [row({
      details: { ...row().details, original_headline: 'x', original_lang_label: 'TL', original_lang_code: '' }
    })])],
    count: 1
  });
  const page = await open({}, body);
  const r = rowsOf(page)[0];
  fire(r, 'click');
  const panel = el(page, r.getAttribute('aria-controls'));
  assert.ok(textOf(panel).includes('Original text (TL)'));
  assert.equal(findAll(panel, (e) => e.hasAttribute('lang')).length, 0);
});

test('a hostile lang code is never written to the page', async () => {
  const body = view({
    groups: [group('reader', 'Where you are: Ntr district', [row({
      details: { ...row().details, original_headline: 'x', original_lang_label: 'Telugu', original_lang_code: 'te" onload="1' }
    })])],
    count: 1
  });
  const page = await open({}, body);
  fire(rowsOf(page)[0], 'click');
  assert.equal(findAll(el(page, 'official-list'), (e) => e.hasAttribute('lang')).length, 0);
});

test('area source: the page fixed point wins (the Demo)', async () => {
  const page = loadPage({ fixedLat: '16.5062', fixedLon: '80.6480', position: { lat: 1, lon: 2 } });
  await flush();
  assert.ok(page.requests[0].url.includes('lat=16.5062'), page.requests[0].url);
  assert.ok(page.requests[0].url.includes('lon=80.648'), page.requests[0].url);
});

test('area source: the tab centre of this account, then the stored fix, then none', async () => {
  const centre = JSON.stringify({ account: '7', lat: 17.38, lon: 78.48 });
  const a = loadPage({ position: { lat: 1, lon: 2 } });
  a.sessionStorage.setItem('pinalert.nearbyCentre', centre);
  assert.deepEqual(JSON.parse(JSON.stringify(a.api.areaSource())), { lat: 17.38, lon: 78.48 });

  const other = loadPage({ position: { lat: 1, lon: 2 } });
  other.sessionStorage.setItem('pinalert.nearbyCentre', JSON.stringify({ account: '8', lat: 17.38, lon: 78.48 }));
  assert.deepEqual(JSON.parse(JSON.stringify(other.api.areaSource())), { lat: 1, lon: 2 });

  const none = loadPage();
  assert.equal(none.api.areaSource(), null);
  await flush();
  assert.equal(none.requests[0].url, '/api/official/alerts');
});

test('area source: bad values are ignored and lat and lon go together', async () => {
  const bad = [
    JSON.stringify({ account: '7', lat: 91, lon: 78 }),
    JSON.stringify({ account: '7', lat: 17 }),
    JSON.stringify({ account: '7', lat: 'x', lon: 'y' }),
    'not json'
  ];
  for (const value of bad) {
    const page = loadPage({ position: { lat: 12.5, lon: 77.5 } });
    page.sessionStorage.setItem('pinalert.nearbyCentre', value);
    assert.deepEqual(JSON.parse(JSON.stringify(page.api.areaSource())), { lat: 12.5, lon: 77.5 }, value);
  }
  const lone = loadPage({ fixedLat: '16.5' });
  assert.equal(lone.api.areaSource(), null, 'a fixed latitude without a longitude is not a point');
  const blocked = loadPage({ noStorage: true, position: { lat: 12.5, lon: 77.5 } });
  assert.deepEqual(JSON.parse(JSON.stringify(blocked.api.areaSource())), { lat: 12.5, lon: 77.5 });
});

test('area source: a page with no account takes no stored centre', () => {
  const page = loadPage({ account: null });
  page.sessionStorage.setItem('pinalert.nearbyCentre', JSON.stringify({ account: '', lat: 17.38, lon: 78.48 }));
  assert.equal(page.api.areaSource(), null);
});

test('the request carries the credentials of the page and the default base', async () => {
  const plain = loadPage();
  await flush();
  assert.equal(plain.requests[0].init.credentials, 'same-origin');
  assert.ok(plain.requests[0].url.startsWith('/api/official/alerts'));
});

test('the page data-api-base replaces /api', async () => {
  const page = loadPage({ dataApiBase: '/demo/api' });
  await flush();
  assert.ok(page.requests[0].url.startsWith('/demo/api/official/alerts'), page.requests[0].url);
});

test('window.PinalertApiBase beats the page attribute', async () => {
  const page = loadPage({ dataApiBase: '/from/attr', apiBase: '/win/api' });
  await flush();
  assert.ok(page.requests[0].url.startsWith('/win/api/official/alerts'), page.requests[0].url);
});

// ---- Plan 05-22: the second status line, hazard events from GDACS ----------
// Added 2026-10-29 by plan 05-22 (ALERT-04 for each official feed). The line is
// muted, sits under the status line, and is drawn from hazards_state,
// hazards_age_seconds and hazards_last_success_at. The SACHET line, the notice
// and every other block are unchanged in every case below.

const hazardLine = (page) => el(page, 'official-hazard-status').textContent;
const withHazards = (over = {}) => view({
  hazards_state: 'fresh',
  hazards_age_seconds: 360,
  hazards_last_success_at: '2026-10-29T10:28:00Z',
  ...over
});

test('hazard line: fresh says when hazard events from GDACS were last checked', async () => {
  const page = await open({ hazardLine: true }, withHazards());
  assert.equal(hazardLine(page), 'Hazard events from GDACS: last checked 6 minutes ago');
  assert.equal(status(page), 'Last checked 4 minutes ago', 'the SACHET line is unchanged');
});

test('hazard line: fresh and under a minute says just now', async () => {
  const page = await open({ hazardLine: true }, withHazards({ hazards_age_seconds: 20 }));
  assert.equal(hazardLine(page), 'Hazard events from GDACS: last checked just now');
});

test('hazard line: stale says they could not be checked since a time', async () => {
  const page = await open({ hazardLine: true }, withHazards({
    hazards_state: 'stale', hazards_age_seconds: 7200, hazards_last_success_at: '2026-10-29T08:35:00Z'
  }));
  assert.equal(hazardLine(page), 'Hazard events from GDACS could not be checked since 2:05 pm.');
  assert.equal(status(page), 'Last checked 4 minutes ago', 'the SACHET line is unchanged');
});

test('hazard line: unavailable with a last good time says the same, with the weekday when not today', async () => {
  const page = await open({ hazardLine: true }, withHazards({
    hazards_state: 'unavailable', hazards_age_seconds: 90000, hazards_last_success_at: '2026-10-28T08:35:00Z'
  }));
  assert.equal(hazardLine(page), 'Hazard events from GDACS could not be checked since Wed 2:05 pm.');
});

test('hazard line: never checked says they have not been checked yet', async () => {
  const page = await open({ hazardLine: true }, withHazards({
    hazards_state: 'unavailable', hazards_age_seconds: null, hazards_last_success_at: null
  }));
  assert.equal(hazardLine(page), 'Hazard events from GDACS have not been checked yet.');
  assert.equal(status(page), 'Last checked 4 minutes ago', 'the SACHET line is unchanged');
});

test('hazard line: an answer with no hazard fields draws no line and does not fail', async () => {
  const page = await open({ hazardLine: true }, view());
  assert.equal(hazardLine(page), '');
  assert.ok(el(page, 'official-hazard-status').hidden, 'an empty line is hidden');
  assert.ok(shown(page, 'official-section') || shown(page, 'official-empty'), 'the page still draws');
});

test('hazard line: a word the route never sends draws no line', async () => {
  const page = await open({ hazardLine: true }, withHazards({ hazards_state: 'weird' }));
  assert.equal(hazardLine(page), '');
});

test('hazard line: the minutes move with the first line, with no new request', async () => {
  const page = await open({ hazardLine: true }, withHazards());
  page.advance(120000);
  assert.equal(hazardLine(page), 'Hazard events from GDACS: last checked 8 minutes ago');
  assert.equal(status(page), 'Last checked 6 minutes ago');
  assert.equal(page.requests.length, 1);
});

test('hazard line: shown with every state of the SACHET feed, none of them changing its words', async () => {
  for (const base of [withRows(), staleAt(), { state: 'unavailable', age_seconds: null, last_success_at: null }]) {
    const page = await open({ hazardLine: true }, withHazards(base));
    assert.match(hazardLine(page), /^Hazard events from GDACS/, 'state ' + base.state);
  }
});

test('hazard line: cleared while loading and after a failed load', async () => {
  const page = await open({ hazardLine: true }, withHazards());
  assert.notEqual(hazardLine(page), '');
  page.api.render(null);
  assert.equal(hazardLine(page), '', 'a failed draw clears it');
  const loading = loadPage({ hazardLine: true });
  assert.equal(hazardLine(loading), '', 'nothing is claimed while loading');
});

test('hazard line: the words are text, never markup', async () => {
  const page = await open({ hazardLine: true }, withHazards({ hazards_last_success_at: '<img src=x onerror=1>' }));
  assert.doesNotMatch(visibleText(page.doc.body), /<img/);
});

test('hazard line: the Official alerts page carries the line and the Demo page does not', () => {
  const real = fs.readFileSync(path.join(here, 'templates', 'alerts.html.tmpl'), 'utf8');
  const demo = fs.readFileSync(path.join(here, 'templates', 'demo_alerts.html.tmpl'), 'utf8');
  assert.equal(real.split('id="official-hazard-status"').length - 1, 1, 'the real page has the line once');
  assert.ok(real.indexOf('id="official-status"') < real.indexOf('id="official-hazard-status"'), 'under the status line');
  assert.ok(real.includes('class="official-status official-status--hazards"'));
  assert.ok(!demo.includes('official-hazard-status'), 'the Demo never draws a GDACS freshness notice');
});
