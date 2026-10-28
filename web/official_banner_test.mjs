// official_banner_test.mjs - runs the real web/static/js/official-banner.js in a
// Node sandbox against the real banner markup of web/templates/index.html.tmpl
// and checks the Nearby banner of Phase 5 plan 15 (ALERT-03, ALERT-04, D-07,
// D-08, D-09, D-16, UI-SPEC Surface 2):
//
//   - drawing: the server's banner pick {level, level_word, event_phrase, area,
//     more} fills the one line: the level class (alert-banner--extreme or
//     alert-banner--severe), the event phrase, the area, "+2", and the hidden
//     words, so the element's accessible name reads "Extreme. Cyclone warning,
//     Krishna district. 2 more alerts." once the aria-hidden parts are dropped;
//     no more alerts hides "+N" and its words; "1 more alert." is singular;
//   - honesty (T-05-80): a view whose banner is null hides the banner; a view
//     whose state is stale or unavailable hides it even if a banner field were
//     present; a level other than severe or extreme, a view that is not shaped
//     like the route's, a failed request and an HTTP error all hide it;
//   - announcing: the first show says "Official alert: Cyclone warning,
//     Krishna district." once in the polite region; the same alert on a refresh
//     says nothing; a move from severe to extreme says it once more; a move to
//     a less serious alert says nothing; after the banner has gone, a new alert
//     is announced again;
//   - following the place (D-09): a pinalert:feed-centre event fetches
//     <base>/official/alerts with its lat and lon; two events within a second
//     fetch once, for the latest place; the same place again fetches nothing;
//     a timer fetches again after 5 minutes; a slow older answer never covers
//     a newer one; the base is window.PinalertApiBase, else /api;
//   - the kept centre (T-05-82): with a data-account-id on the app shell the
//     tab keeps {"account":"<id>","lat":n,"lon":n} under pinalert.nearbyCentre;
//     without one (the Demo) nothing is stored; a storage that throws is
//     silent;
//   - feed text enters the page through textContent only (T-05-81): an event
//     phrase holding markup appears as text, and the fake element refuses any
//     innerHTML write.
//
// Run it with: node --test web/official_banner_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what official-banner.js calls: a minimal element model
// built from the template's own banner markup, a clock and timers the test
// moves by hand and a fetch the test answers by hand.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'official-banner.js'), 'utf8');
const template = fs.readFileSync(path.join(here, 'templates', 'index.html.tmpl'), 'utf8');
const demoTemplate = fs.readFileSync(path.join(here, 'templates', 'demo.html.tmpl'), 'utf8');

class FakeEl {
  constructor(doc, tag) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = '';
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
  }
  get className() { return this.attrs.class || ''; }
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
  }
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
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
}

const hasClass = (el, name) => el.className.split(/\s+/).includes(name);

// accessibleName is what a screen reader reads from the link: the hidden and
// the aria-hidden subtrees are dropped, the visually hidden words stay.
function accessibleName(el) {
  if (el.hidden || el.getAttribute('aria-hidden') === 'true') { return ''; }
  return el.own + el.children.map(accessibleName).join('');
}

// buildPage makes the banner from the template's own markup: the opening tag
// and every flat span inside it, so the script and the template cannot drift.
function buildPage({ account = '7', withMarkup = true, dataApiBase, demoPage = false } = {}) {
  const doc = {
    byId: {},
    listeners: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    addEventListener(type, fn) { (doc.listeners[type] = doc.listeners[type] || []).push(fn); },
    querySelector() { return null; }
  };
  const make = (tag, id, parent) => {
    const el = new FakeEl(doc, tag);
    if (id) { el.setAttribute('id', id); }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  doc.body = make('body');
  make('div', 'notice-live', doc.body);
  const shell = make('div', 'app-shell', doc.body);
  if (account !== null) { shell.setAttribute('data-account-id', account); }
  if (dataApiBase !== undefined) { shell.setAttribute('data-api-base', dataApiBase); }
  if (!withMarkup) { return { doc, shell }; }

  // Amended 2026-10-29 by plan 05-19: demoPage builds the banner from the Demo
  // page's own markup (demo.html.tmpl), which holds the same inner spans plus the
  // Demo tag. Without the option the real page's markup is used, as before.
  const block = demoPage
    ? demoTemplate.match(/<a class="alert-banner" id="alert-banner" href="\{\{\.AlertsHref\}\}" hidden>([\s\S]*?)<\/a>/)
    : template.match(/<a class="alert-banner" id="alert-banner" href="\/alerts" hidden>([\s\S]*?)<\/a>/);
  assert.ok(block, (demoPage ? 'demo.html.tmpl' : 'index.html.tmpl') + ' has the banner link');
  const banner = make('a', 'alert-banner', shell);
  banner.setAttribute('class', 'alert-banner');
  banner.setAttribute('href', demoPage ? '/demo/alerts' : '/alerts');
  banner.hidden = true;
  const spanPattern = /<span\b([^>]*)>([^<]*)<\/span>/g;
  let m;
  while ((m = spanPattern.exec(block[1])) !== null) {
    const span = make('span', '', banner);
    const attrPattern = /([a-z-]+)(?:="([^"]*)")?/g;
    let a;
    while ((a = attrPattern.exec(m[1])) !== null) {
      span.setAttribute(a[1], a[2] === undefined ? '' : a[2]);
    }
    span.own = m[2];
  }
  return { doc, shell, banner };
}

// The clock: a fixed moment the test moves by hand.
const T0 = Date.parse('2026-10-29T10:34:00Z');

function loadPage(options = {}) {
  const page = buildPage(options);
  const requests = [];
  let now = T0;
  let nextId = 1;
  const timers = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.next <= end).sort((a, b) => a[1].next - b[1].next)[0];
      if (!due) { break; }
      const [id, timer] = due;
      now = timer.next;
      if (timer.every) { timer.next += timer.every; } else { timers.delete(id); }
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
  const sessionStorage = options.throwingStorage
    ? { getItem() { throw new Error('blocked'); }, setItem() { throw new Error('blocked'); } }
    : {
      getItem(k) { return store.has(k) ? store.get(k) : null; },
      setItem(k, v) { store.set(k, String(v)); },
      removeItem(k) { store.delete(k); }
    };
  const sandbox = {
    console,
    document: page.doc,
    Date: FakeDate,
    sessionStorage,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setInterval(fn, every) { const id = nextId++; timers.set(id, { fn, every, next: now + every }); return id; },
    clearInterval(id) { timers.delete(id); },
    setTimeout(fn, delay) { const id = nextId++; timers.set(id, { fn, every: 0, next: now + (delay || 0) }); return id; },
    clearTimeout(id) { timers.delete(id); }
  };
  if (options.apiBase !== undefined) { sandbox.PinalertApiBase = options.apiBase; }
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'official-banner.js' });
  const centre = (lat, lon) => {
    (page.doc.listeners['pinalert:feed-centre'] || []).forEach((fn) => fn({ type: 'pinalert:feed-centre', detail: { lat, lon } }));
  };
  return { ...page, api: sandbox.PinalertOfficialBanner, requests, advance, centre, store, timers, sandbox };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: body === undefined ? () => Promise.reject(new Error('no body')) : () => Promise.resolve(body)
});

const byId = (page, id) => page.doc.byId[id];
const live = (page) => byId(page, 'notice-live').textContent;

function pick(over = {}) {
  return {
    level: 'extreme',
    level_word: 'Extreme',
    event_phrase: 'Cyclone warning',
    area: 'Krishna district',
    more: 2,
    alert_id: 41,
    ...over
  };
}

function view(banner, over = {}) {
  return { state: 'fresh', groups: [], count: 0, banner, ...over };
}

test('the template banner keeps its opening tag and the spans the script writes', () => {
  const page = loadPage();
  for (const id of ['alert-banner', 'alert-banner-level', 'alert-banner-event', 'alert-banner-area', 'alert-banner-more', 'alert-banner-more-sr']) {
    assert.ok(byId(page, id), 'the template has #' + id);
  }
  assert.equal(page.banner.hidden, true, 'the banner starts hidden');
});

test('an extreme pick fills the one line and the accessible name', () => {
  const page = loadPage();
  page.api.render(view(pick()));
  const banner = page.banner;
  assert.equal(banner.hidden, false);
  assert.ok(hasClass(banner, 'alert-banner'));
  assert.ok(hasClass(banner, 'alert-banner--extreme'));
  assert.ok(!hasClass(banner, 'alert-banner--severe'));
  assert.equal(byId(page, 'alert-banner-event').textContent, 'Cyclone warning');
  assert.equal(byId(page, 'alert-banner-area').textContent, 'Krishna district');
  assert.equal(byId(page, 'alert-banner-more').textContent, '+2');
  assert.equal(byId(page, 'alert-banner-more').hidden, false);
  assert.equal(accessibleName(banner).trim(), 'Extreme. Cyclone warning, Krishna district. 2 more alerts.');
});

test('no more alerts hides +N and its words; one more is singular', () => {
  const page = loadPage();
  page.api.render(view(pick({ more: 0 })));
  assert.equal(byId(page, 'alert-banner-more').hidden, true);
  assert.equal(byId(page, 'alert-banner-more').textContent, '');
  assert.equal(byId(page, 'alert-banner-more-sr').textContent, '');
  assert.equal(accessibleName(page.banner).trim(), 'Extreme. Cyclone warning, Krishna district.');
  page.api.render(view(pick({ more: 1 })));
  assert.equal(byId(page, 'alert-banner-more').textContent, '+1');
  assert.equal(byId(page, 'alert-banner-more').hidden, false);
  assert.equal(accessibleName(page.banner).trim(), 'Extreme. Cyclone warning, Krishna district. 1 more alert.');
});

test('a severe pick gets the severe class and drops the extreme one', () => {
  const page = loadPage();
  page.api.render(view(pick()));
  page.api.render(view(pick({ level: 'severe', level_word: 'Severe', event_phrase: 'Heavy rain warning', area: 'Ntr district', more: 0 })));
  assert.ok(hasClass(page.banner, 'alert-banner--severe'));
  assert.ok(!hasClass(page.banner, 'alert-banner--extreme'));
  assert.ok(hasClass(page.banner, 'alert-banner'));
  assert.equal(accessibleName(page.banner).trim(), 'Severe. Heavy rain warning, Ntr district.');
});

test('a view with no banner hides it, whatever the state', () => {
  const page = loadPage();
  page.api.render(view(pick()));
  assert.equal(page.banner.hidden, false);
  page.api.render(view(null));
  assert.equal(page.banner.hidden, true);
  page.api.render(view(undefined));
  assert.equal(page.banner.hidden, true);
  page.api.render(view(null, { state: 'stale' }));
  assert.equal(page.banner.hidden, true);
  page.api.render(view(null, { state: 'unavailable' }));
  assert.equal(page.banner.hidden, true);
});

test('a stale or unavailable view hides the banner even if a banner field were present (ALERT-04)', () => {
  for (const state of ['stale', 'unavailable', 'unknown-state']) {
    const page = loadPage();
    page.api.render(view(pick()));
    assert.equal(page.banner.hidden, false);
    page.api.render(view(pick(), { state }));
    assert.equal(page.banner.hidden, true, state + ' hides it');
  }
});

test('a level other than severe or extreme, or a pick without words, is never drawn', () => {
  const page = loadPage();
  for (const bad of [
    pick({ level: 'moderate', level_word: 'Moderate' }),
    pick({ level: 'unknown', level_word: 'Unknown' }),
    pick({ level: undefined }),
    pick({ event_phrase: '' }),
    pick({ area: '' }),
    'text',
    42
  ]) {
    page.api.render(view(bad));
    assert.equal(page.banner.hidden, true);
  }
  page.api.render(null);
  assert.equal(page.banner.hidden, true);
  page.api.render('nope');
  assert.equal(page.banner.hidden, true);
  page.api.render({ state: 'fresh', banner: pick() });
  assert.equal(page.banner.hidden, false, 'a view without groups still carries the pick (only the state gates it)');
});

test('feed strings enter the page as text, never as markup', () => {
  const page = loadPage();
  page.api.render(view(pick({ event_phrase: '<img src=x onerror=1>', area: '<b>Krishna</b>' })));
  assert.equal(byId(page, 'alert-banner-event').textContent, '<img src=x onerror=1>');
  assert.equal(byId(page, 'alert-banner-area').textContent, '<b>Krishna</b>');
  assert.equal(page.banner.children.filter((c) => c.tagName !== 'SPAN').length, 0);
});

test('the first show announces once; a refresh of the same alert says nothing', () => {
  const page = loadPage();
  page.api.render(view(pick()));
  assert.equal(live(page), 'Official alert: Cyclone warning, Krishna district.');
  byId(page, 'notice-live').textContent = '';
  page.api.render(view(pick()));
  page.api.render(view(pick({ more: 3 })));
  assert.equal(live(page), '', 'a quiet refresh says nothing');
});

test('a move from severe to extreme is announced once more; the other way is not', () => {
  const page = loadPage();
  page.api.render(view(pick({ level: 'severe', level_word: 'Severe', event_phrase: 'Heavy rain warning', area: 'Ntr district', more: 0 })));
  assert.equal(live(page), 'Official alert: Heavy rain warning, Ntr district.');
  page.api.render(view(pick()));
  assert.equal(live(page), 'Official alert: Cyclone warning, Krishna district.');
  byId(page, 'notice-live').textContent = '';
  page.api.render(view(pick({ level: 'severe', level_word: 'Severe', event_phrase: 'Heavy rain warning', area: 'Ntr district' })));
  assert.equal(live(page), '', 'a less serious alert is not announced');
});

test('after the banner has gone, a new alert is announced again', () => {
  const page = loadPage();
  page.api.render(view(pick()));
  page.api.render(view(null));
  byId(page, 'notice-live').textContent = '';
  page.api.render(view(pick()));
  assert.equal(live(page), 'Official alert: Cyclone warning, Krishna district.');
});

test('a feed centre event fetches the official route with its lat and lon', async () => {
  const page = loadPage();
  page.centre(16.5062, 80.648);
  page.advance(1000);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/official/alerts?lat=16.5062&lon=80.648');
  assert.equal(page.requests[0].init.credentials, 'same-origin');
  answer(page.requests[0], 200, view(pick()));
  await flush();
  assert.equal(page.banner.hidden, false);
  assert.equal(accessibleName(page.banner).trim(), 'Extreme. Cyclone warning, Krishna district. 2 more alerts.');
});

test('two events within a second fetch once, for the latest place', () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(100);
  page.centre(17.1, 81.2);
  page.advance(2000);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/official/alerts?lat=17.1&lon=81.2');
});

test('the same place again fetches nothing; a new place after the gap fetches', async () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(1000);
  answer(page.requests[0], 200, view(null));
  await flush();
  page.centre(16.5, 80.6);
  page.advance(2000);
  assert.equal(page.requests.length, 1, 'the same place is not asked again');
  page.centre(13.08, 80.27);
  page.advance(2000);
  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].url, '/api/official/alerts?lat=13.08&lon=80.27');
});

test('a place change is never fetched twice within a second of the last fetch', async () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(500);
  assert.equal(page.requests.length, 1);
  answer(page.requests[0], 200, view(null));
  await flush();
  page.advance(100);
  page.centre(13.08, 80.27);
  page.advance(800);
  assert.equal(page.requests.length, 1, 'less than a second after the last fetch');
  page.advance(200);
  assert.equal(page.requests.length, 2);
});

test('a timer fetches again after five minutes, not before', async () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(1000);
  answer(page.requests[0], 200, view(pick()));
  await flush();
  page.advance(290000);
  assert.equal(page.requests.length, 1);
  page.advance(20000);
  assert.equal(page.requests.length, 2);
  assert.equal(page.requests[1].url, page.requests[0].url);
  byId(page, 'notice-live').textContent = '';
  answer(page.requests[1], 200, view(pick()));
  await flush();
  assert.equal(live(page), '', 'the refresh of the same alert is quiet');
  assert.equal(page.banner.hidden, false);
});

test('a refresh that finds the alert gone hides the banner', async () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(1000);
  answer(page.requests[0], 200, view(pick()));
  await flush();
  assert.equal(page.banner.hidden, false);
  page.advance(300000);
  answer(page.requests[1], 200, view(null));
  await flush();
  assert.equal(page.banner.hidden, true);
});

test('a slow older answer never covers a newer one', async () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(1000);
  page.centre(13.08, 80.27);
  page.advance(2000);
  assert.equal(page.requests.length, 2);
  answer(page.requests[1], 200, view(null));
  await flush();
  assert.equal(page.banner.hidden, true);
  answer(page.requests[0], 200, view(pick()));
  await flush();
  assert.equal(page.banner.hidden, true, 'the older answer for the first place is dropped');
});

test('a failed request or an HTTP error hides the banner (the feed cannot be vouched for)', async () => {
  const page = loadPage();
  page.api.render(view(pick()));
  page.centre(16.5, 80.6);
  page.advance(1000);
  page.requests[0].reject(new Error('offline'));
  await flush();
  assert.equal(page.banner.hidden, true);
  page.api.render(view(pick()));
  page.advance(300000);
  answer(page.requests[1], 503);
  await flush();
  assert.equal(page.banner.hidden, true);
  page.api.render(view(pick()));
  page.advance(300000);
  answer(page.requests[2], 200, undefined);
  await flush();
  assert.equal(page.banner.hidden, true, 'an unreadable body is not an empty pick, it is no pick');
});

test('a quiet failure does not announce the same alert again when it returns', async () => {
  const page = loadPage();
  page.centre(16.5, 80.6);
  page.advance(1000);
  answer(page.requests[0], 200, view(pick()));
  await flush();
  byId(page, 'notice-live').textContent = '';
  page.advance(300000);
  page.requests[1].reject(new Error('offline'));
  await flush();
  page.advance(300000);
  answer(page.requests[2], 200, view(pick()));
  await flush();
  assert.equal(page.banner.hidden, false);
  assert.equal(live(page), '');
});

test('events with a bad point are ignored', () => {
  const page = loadPage();
  page.centre(NaN, 80.6);
  page.centre(95, 80.6);
  page.centre(16.5, 200);
  page.centre('16.5', '80.6');
  page.centre(undefined, undefined);
  page.advance(5000);
  assert.equal(page.requests.length, 0);
});

test('the base is window.PinalertApiBase, else the shell data-api-base, else /api', () => {
  const demo = loadPage({ apiBase: '/api/demo/abc' });
  demo.centre(16.5, 80.6);
  demo.advance(1000);
  assert.equal(demo.requests[0].url, '/api/demo/abc/official/alerts?lat=16.5&lon=80.6');
  const trailing = loadPage({ apiBase: '/api/demo/abc/' });
  trailing.centre(16.5, 80.6);
  trailing.advance(1000);
  assert.equal(trailing.requests[0].url, '/api/demo/abc/official/alerts?lat=16.5&lon=80.6');
  const attr = loadPage({ dataApiBase: '/x/api' });
  attr.centre(1, 2);
  attr.advance(1000);
  assert.equal(attr.requests[0].url, '/x/api/official/alerts?lat=1&lon=2');
});

test('the tab keeps the centre for this account, exactly', () => {
  const page = loadPage({ account: '7' });
  page.centre(16.5062, 80.648);
  const raw = page.store.get('pinalert.nearbyCentre');
  assert.equal(raw, '{"account":"7","lat":16.5062,"lon":80.648}');
  assert.deepEqual(JSON.parse(raw), { account: '7', lat: 16.5062, lon: 80.648 });
  page.centre(13.08, 80.27);
  assert.deepEqual(JSON.parse(page.store.get('pinalert.nearbyCentre')), { account: '7', lat: 13.08, lon: 80.27 });
});

test('with no account (the Demo) nothing is stored', () => {
  const none = loadPage({ account: null });
  none.centre(16.5, 80.6);
  none.advance(1000);
  assert.equal(none.store.size, 0);
  assert.equal(none.requests.length, 1, 'the Demo still follows the place');
  const empty = loadPage({ account: '' });
  empty.centre(16.5, 80.6);
  assert.equal(empty.store.size, 0);
});

test('a storage that throws is silent and the banner still follows the place', () => {
  const page = loadPage({ throwingStorage: true });
  page.centre(16.5, 80.6);
  page.advance(1000);
  assert.equal(page.requests.length, 1);
});

test('a page without the banner markup does nothing', () => {
  const page = loadPage({ withMarkup: false });
  page.centre(16.5, 80.6);
  page.advance(1000);
  assert.equal(page.requests.length, 0);
});

// Amended 2026-10-29 by plan 05-19 (D-05, D-07, ROBUST-06): the cases below are
// new and change none of the cases above. The Demo's banner carries a Demo tag
// right after the level icon; the script shows it only when the server's pick
// says demo is true, and the real page, which has no such element, is untouched.

test('the Demo banner markup holds the same spans plus the Demo tag right after the icon', () => {
  const page = loadPage({ demoPage: true, account: null });
  for (const id of ['alert-banner', 'alert-banner-level', 'alert-banner-event', 'alert-banner-area', 'alert-banner-more', 'alert-banner-more-sr', 'alert-banner-demo']) {
    assert.ok(byId(page, id), 'the Demo page has #' + id);
  }
  const order = page.banner.children.map((c) => (c.id || c.className));
  const icon = order.indexOf('alert-banner__icon');
  assert.ok(icon !== -1, 'the icon span is in the banner');
  assert.equal(order[icon + 1], 'alert-banner-demo', 'the Demo tag follows the icon');
  assert.equal(order[icon + 2], 'alert-banner-event', 'the event words follow the tag');
  assert.equal(byId(page, 'alert-banner-demo').hidden, true, 'the tag starts hidden');
  assert.equal(byId(page, 'alert-banner-demo').textContent, 'Demo');
});

test('a pick with demo true shows the Demo tag; demo false or missing hides it', () => {
  const page = loadPage({ demoPage: true, account: null });
  const tag = byId(page, 'alert-banner-demo');
  page.api.render(view(pick({ demo: true })));
  assert.equal(page.banner.hidden, false);
  assert.equal(tag.hidden, false, 'a sample alert shows the Demo tag');
  assert.ok(accessibleName(page.banner).includes('Demo'), 'the tag is part of the link name');
  page.api.render(view(pick({ demo: false })));
  assert.equal(tag.hidden, true, 'demo false hides the tag');
  page.api.render(view(pick({ demo: true })));
  page.api.render(view(pick()));
  assert.equal(tag.hidden, true, 'a pick with no demo field hides the tag');
  page.api.render(view(pick({ demo: 'true' })));
  assert.equal(tag.hidden, true, 'only the boolean true counts');
});

test('the Demo tag goes with the banner when the banner is hidden', () => {
  const page = loadPage({ demoPage: true, account: null });
  const tag = byId(page, 'alert-banner-demo');
  page.api.render(view(pick({ demo: true })));
  page.api.render(view(null));
  assert.equal(page.banner.hidden, true);
  assert.equal(tag.hidden, true);
  page.api.render(view(pick({ demo: true }), { state: 'stale' }));
  assert.equal(page.banner.hidden, true);
  assert.equal(tag.hidden, true, 'a stale view never leaves a Demo tag showing');
});

test('the real page has no Demo tag and a demo flag changes nothing there', () => {
  const page = loadPage();
  assert.equal(byId(page, 'alert-banner-demo'), undefined);
  page.api.render(view(pick({ demo: true })));
  assert.equal(page.banner.hidden, false, 'the banner still draws');
  assert.equal(accessibleName(page.banner).trim(), 'Extreme. Cyclone warning, Krishna district. 2 more alerts.');
});

test('the Demo banner follows the place on the visit API base and stores nothing', () => {
  const page = loadPage({ demoPage: true, account: null, apiBase: '/demo/api/v/abc' });
  page.centre(16.5424, 80.6386);
  page.advance(1000);
  assert.equal(page.requests.length, 1);
  assert.ok(page.requests[0].url.startsWith('/demo/api/v/abc/official/alerts?'), page.requests[0].url);
  assert.equal(page.store.size, 0);
});
