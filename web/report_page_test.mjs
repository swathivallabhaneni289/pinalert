// report_page_test.mjs - runs the real web/static/js/report-page.js in a Node
// sandbox with a fake document, a fake fetch and a fake clock, and checks the
// report page body (Phase 8 plan 05, PAGE-01, D-01, D-17, D-18, UI-SPEC
// Surface 1 and Surface 4):
//
//   - the page is built once and refreshed in place: the same ol, the same Share
//     button and the same Comments mount point after twenty updates (research
//     Pitfall 6);
//   - the history shows a skeleton while it loads, one row per event when it
//     arrives, the error block with Try again when it fails or times out, the
//     offline sentence without a network, and hides itself on a 404;
//   - it fetches only when it is hosted and open, once per open, once more when
//     the trust signature changes, and every 30 seconds while the tab is
//     visible, with never a second interval;
//   - the time column words are built from getHours, getMinutes and a fixed
//     weekday list, whatever the browser's locale;
//   - Share is hidden until the Share window script exists and then opens it;
//   - a refresh never calls focus().
//
// The child order test at the foot cuts arrangeCard out of the real map.js and
// moves nodes with real parent semantics across a resize (research Pitfall 5b).
//
// Run it with: node --test web/report_page_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what report-page.js and arrangeCard call.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const pageSource = fs.readFileSync(path.join(here, 'static', 'js', 'report-page.js'), 'utf8');
const mapSource = fs.readFileSync(path.join(here, 'static', 'js', 'map.js'), 'utf8');

function between(source, from, to) {
  const start = source.indexOf(from);
  const end = source.indexOf(to, start);
  assert.ok(start >= 0 && end > start, 'the source no longer has the text between "' + from + '" and "' + to + '"');
  return source.slice(start, end);
}

// FakeEl keeps real parent semantics: appending or inserting a node first takes
// it out of its old parent, as a browser does, so a move test means something.
class FakeEl {
  constructor(doc, tag) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.dataset = {};
    this.className = '';
    this.hidden = false;
    this.focusCalls = 0;
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  get firstChild() { return this.children[0] || null; }
  get lastChild() { return this.children[this.children.length - 1] || null; }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasAttribute(n) { return Object.hasOwn(this.attrs, n); }
  removeAttribute(n) { delete this.attrs[n]; }
  detach(node) {
    if (node.parentNode) {
      const siblings = node.parentNode.children;
      siblings.splice(siblings.indexOf(node), 1);
      node.parentNode = null;
    }
  }
  appendChild(node) { this.detach(node); this.children.push(node); node.parentNode = this; return node; }
  insertBefore(node, ref) {
    this.detach(node);
    const at = ref ? this.children.indexOf(ref) : -1;
    if (at < 0) { this.children.push(node); } else { this.children.splice(at, 0, node); }
    node.parentNode = this;
    return node;
  }
  removeChild(node) { this.detach(node); return node; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.focusCalls += 1; this.doc.activeElement = this; }
  fire(type, evt = {}) {
    const event = { type, target: this, stopped: false, preventDefault() {}, stopPropagation() { this.stopped = true; }, ...evt };
    (this.handlers[type] || []).slice().forEach((fn) => fn(event));
    return event;
  }
  hasClass(name) { return this.className.split(' ').includes(name); }
}

function walk(el, out = []) {
  out.push(el);
  el.children.forEach((c) => walk(c, out));
  return out;
}
function findAll(el, cls) { return walk(el).filter((n) => n.hasClass(cls)); }
function find(el, cls) { return findAll(el, cls)[0] || null; }

const flush = async () => { for (let i = 0; i < 20; i += 1) { await Promise.resolve(); } };

// load evaluates report-page.js in a sandbox. fetches keeps every request with
// the hooks to answer it; timers is the fake clock.
function load({ online = true, visible = true, share = false, comments = false } = {}) {
  const doc = {
    activeElement: null,
    visibilityState: visible ? 'visible' : 'hidden',
    all: [],
    createElement(tag) { const el = new FakeEl(doc, tag); doc.all.push(el); return el; }
  };
  const fetches = [];
  const timers = { now: 0, nextId: 1, list: [] };
  const clock = {
    setTimeout(fn, ms) { const id = timers.nextId++; timers.list.push({ id, fn, at: timers.now + ms, every: 0 }); return id; },
    setInterval(fn, ms) { const id = timers.nextId++; timers.list.push({ id, fn, at: timers.now + ms, every: ms }); return id; },
    clearTimeout(id) { timers.list = timers.list.filter((t) => t.id !== id); },
    clearInterval(id) { timers.list = timers.list.filter((t) => t.id !== id); }
  };
  class FakeAbortController {
    constructor() {
      const handlers = [];
      this.signal = { aborted: false, addEventListener(type, fn) { handlers.push(fn); } };
      this.fire = () => handlers.forEach((fn) => fn());
    }
    abort() { if (!this.signal.aborted) { this.signal.aborted = true; this.fire(); } }
  }
  const sandbox = {
    document: doc,
    navigator: { onLine: online },
    Pinalert: {
      setText(el, value) { el.textContent = value === null || value === undefined ? '' : String(value); },
      categoryLabel(c) { return c; }
    },
    AbortController: FakeAbortController,
    fetch(address, opts) {
      return new Promise((resolve, reject) => {
        const call = {
          address, opts, settled: false,
          reply(status, body) {
            call.settled = true;
            resolve({ ok: status >= 200 && status < 300, status, json: () => Promise.resolve(body) });
          },
          fail(err) { call.settled = true; reject(err || new Error('network')); }
        };
        opts.signal.addEventListener('abort', () => {
          if (!call.settled) { call.settled = true; reject(new Error('aborted')); }
        });
        fetches.push(call);
      });
    },
    console
  };
  sandbox.window = sandbox;
  Object.assign(sandbox, clock);
  if (share) {
    sandbox.PinalertShare = { calls: [], open(arg) { this.calls.push(arg); } };
  }
  if (comments) {
    // Amended 2026-10-29 by Phase 8 plan 16 (D-02): a fake comments module, so
    // the page's forwarding of update, open and close can be seen.
    sandbox.PinalertComments = {
      views: [],
      create(rep, mount) {
        const calls = [];
        const view = { calls, rep, mount, el: null };
        for (const name of ['update', 'open', 'close', 'refresh']) { view[name] = (arg) => calls.push(name + (arg ? ':' + arg.id : '')); }
        this.views.push(view);
        return view;
      }
    };
  }
  vm.createContext(sandbox);
  vm.runInContext(pageSource, sandbox, { filename: 'report-page.js' });
  return {
    doc, fetches, sandbox, timers,
    Page: sandbox.PinalertReportPage,
    advance(ms) {
      const to = timers.now + ms;
      for (;;) {
        const due = timers.list.filter((t) => t.at <= to).sort((a, b) => a.at - b.at)[0];
        if (!due) { break; }
        timers.now = due.at;
        if (due.every) { due.at += due.every; } else { timers.list = timers.list.filter((t) => t !== due); }
        due.fn();
      }
      timers.now = to;
    },
    intervals() { return timers.list.filter((t) => t.every > 0).length; }
  };
}

function report(over = {}) {
  return {
    id: 'r1', category: 'flood', created_at: '2026-10-29T08:00:00Z', view: 'active', visibility: 'live',
    your_vote: null, trust: { confirmed_places: 1, disputed_places: 0 }, ...over
  };
}

const eventsBody = {
  events: [
    { kind: 'posted', tone: 'neutral', text: 'Posted as Critical.', at: '2026-10-29T08:00:00Z', yours: false },
    { kind: 'confirmed', tone: 'good', text: 'Confirmed from a second spot.', at: '2026-10-29T08:10:00Z', yours: false },
    { kind: 'disputed', tone: 'bad', text: 'Disputed by someone nearby.', at: '2026-10-29T08:20:00Z', yours: false }
  ]
};

function openHosted(env, rep = report()) {
  const page = env.Page.create(rep);
  page.setHosted(true);
  page.open();
  return page;
}

test('build once', () => {
  const env = load();
  const page = env.Page.create(report());
  assert.ok(page.el.hasClass('report-page'));
  assert.equal(page.el.tagName, 'SECTION');
  assert.equal(page.el.hidden, true, 'the page is hidden until a host shows it (D-18)');
  const section = find(page.el, 'report-page__timeline');
  assert.ok(section, 'the history section');
  const heading = walk(section).find((n) => n.tagName === 'H2');
  assert.equal(heading.textContent, 'What happened so far');
  assert.equal(walk(section).filter((n) => n.tagName === 'OL' && n.hasClass('report-timeline')).length, 1);
  const comments = find(page.el, 'report-page__comments');
  assert.ok(comments, 'the comments mount point');
  assert.equal(comments.getAttribute('data-report-id'), 'r1');
  assert.equal(comments.children.length, 0, 'the mount point is empty');
  assert.equal(page.share.tagName, 'BUTTON');
  assert.ok(page.share.hasClass('vote-btn') && page.share.hasClass('report-page__share'));
  assert.equal(page.share.textContent, 'Share');
  assert.equal(env.fetches.length, 0, 'building fetches nothing');
  for (const name of ['update', 'setHosted', 'open', 'close', 'refresh']) {
    assert.equal(typeof page[name], 'function', name);
  }
});

test('update keeps nodes', async () => {
  const env = load({ share: true });
  const page = openHosted(env);
  env.fetches[0].reply(200, eventsBody);
  await flush();
  const list = find(page.el, 'report-timeline');
  const rows = findAll(page.el, 'report-timeline__row');
  const comments = find(page.el, 'report-page__comments');
  const share = page.share;
  const nodeCount = walk(page.el).length;
  for (let i = 0; i < 20; i += 1) {
    page.update(report({ trust: { confirmed_places: 1 + (i % 3), disputed_places: i % 2 } }));
  }
  await flush();
  assert.equal(find(page.el, 'report-timeline'), list, 'the same ol');
  assert.equal(page.share, share, 'the same Share button');
  assert.equal(find(page.el, 'report-page__comments'), comments, 'the same comments mount');
  assert.equal(page.share.parentNode, share.parentNode);
  // A refetch that answers with the same events keeps the very same rows.
  env.fetches.slice(1).forEach((call) => { if (!call.settled) { call.reply(200, eventsBody); } });
  await flush();
  const after = findAll(page.el, 'report-timeline__row');
  assert.equal(after.length, rows.length);
  after.forEach((row, i) => assert.equal(row, rows[i], 'row ' + i + ' is the same node'));
  assert.equal(walk(page.el).length, nodeCount, 'no node was added or lost');
  assert.equal(env.doc.all.filter((n) => n.focusCalls > 0).length, 0, 'a refresh never calls focus()');
});

test('timeline states', async () => {
  // Loading: aria-busy and three skeleton rows.
  let env = load();
  let page = openHosted(env);
  let list = find(page.el, 'report-timeline');
  assert.equal(list.getAttribute('aria-busy'), 'true');
  assert.equal(findAll(list, 'report-timeline__row--skeleton').length, 3);
  assert.equal(env.fetches.length, 1);
  assert.equal(env.fetches[0].address, '/api/reports/r1/timeline');
  assert.equal(env.fetches[0].opts.credentials, 'same-origin');

  // Filled: one li per event, a time element, an aria-hidden dot, the text.
  env.fetches[0].reply(200, eventsBody);
  await flush();
  assert.equal(list.getAttribute('aria-busy'), 'false');
  const rows = findAll(list, 'report-timeline__row');
  assert.equal(rows.length, 3);
  assert.equal(findAll(list, 'report-timeline__row--skeleton').length, 0);
  rows.forEach((row, i) => {
    assert.equal(row.tagName, 'LI');
    const time = walk(row).find((n) => n.tagName === 'TIME');
    assert.equal(time.getAttribute('datetime'), eventsBody.events[i].at);
    const dot = find(row, 'report-timeline__dot');
    assert.equal(dot.getAttribute('aria-hidden'), 'true');
    assert.ok(dot.hasClass('report-timeline__dot--' + eventsBody.events[i].tone));
    assert.equal(find(row, 'report-timeline__text').textContent, eventsBody.events[i].text);
  });
  assert.equal(find(page.el, 'report-page__timeline').hidden, false);

  // Server text is text, whatever it holds.
  const wrapped = load();
  const p2 = openHosted(wrapped);
  wrapped.fetches[0].reply(200, { events: [{ kind: 'posted', tone: 'neutral', text: '<b>x</b>', at: '2026-10-29T08:00:00Z', yours: false }] });
  await flush();
  assert.equal(find(p2.el, 'report-timeline__text').textContent, '<b>x</b>');
  assert.equal(findAll(p2.el, 'report-timeline__text')[0].children.length, 0);

  // A 500: the error block, then Try again fetches again and focuses the heading.
  env = load();
  page = openHosted(env);
  env.fetches[0].reply(500, {});
  await flush();
  let errorBlock = find(page.el, 'report-timeline__error');
  assert.equal(errorBlock.hidden, false);
  assert.equal(find(errorBlock, 'report-timeline__error-title').textContent, "Couldn't load the history");
  assert.equal(find(errorBlock, 'report-timeline__error-line').textContent, 'Check your connection and try again.');
  const retry = find(errorBlock, 'report-page__retry');
  assert.equal(retry.textContent, 'Try again');
  assert.equal(find(page.el, 'report-timeline').hidden, true);
  retry.fire('click');
  assert.equal(env.fetches.length, 2, 'Try again fetches again');
  assert.equal(errorBlock.hidden, true);
  assert.equal(env.doc.activeElement.tagName, 'H2', 'Try again keeps focus inside the section');
  env.fetches[1].reply(200, eventsBody);
  await flush();
  assert.equal(findAll(page.el, 'report-timeline__row').length, 3);
  assert.equal(find(page.el, 'report-timeline').hidden, false);

  // A timeout: the fetch is aborted at 10 seconds and the error block shows.
  env = load();
  page = openHosted(env);
  env.advance(9999);
  assert.equal(env.fetches[0].opts.signal.aborted, false);
  env.advance(1);
  assert.equal(env.fetches[0].opts.signal.aborted, true);
  await flush();
  assert.equal(find(page.el, 'report-timeline__error').hidden, false);

  // Offline: the offline sentence and no request.
  env = load({ online: false });
  page = openHosted(env);
  await flush();
  assert.equal(env.fetches.length, 0, 'offline sends nothing');
  errorBlock = find(page.el, 'report-timeline__error');
  assert.equal(errorBlock.hidden, false);
  assert.equal(find(errorBlock, 'report-timeline__error-line').textContent,
    "You're offline, so the history can't load. Try again when you have a signal.");

  // A 404: the whole history section is hidden.
  env = load();
  page = openHosted(env);
  env.fetches[0].reply(404, { error: { field: 'report', message: 'This report is no longer available.' } });
  await flush();
  assert.equal(find(page.el, 'report-page__timeline').hidden, true);
  assert.equal(find(page.el, 'report-timeline__error').hidden, true);

  // A background refresh that fails keeps the rows it has.
  env = load();
  page = openHosted(env);
  env.fetches[0].reply(200, eventsBody);
  await flush();
  page.update(report({ trust: { confirmed_places: 5, disputed_places: 0 } }));
  env.fetches[1].reply(500, {});
  await flush();
  assert.equal(findAll(page.el, 'report-timeline__row').length, 3);
  assert.equal(find(page.el, 'report-timeline__error').hidden, true);
});

test('when it fetches', async () => {
  const env = load();
  const page = env.Page.create(report());
  // Open on a page no host has shown fetches nothing.
  page.open();
  assert.equal(env.fetches.length, 0);
  assert.equal(env.intervals(), 0);
  page.close();

  // Hosted then opened: one fetch and one interval.
  page.setHosted(true);
  assert.equal(env.fetches.length, 0, 'hosting alone fetches nothing');
  page.open();
  assert.equal(env.fetches.length, 1);
  assert.equal(env.intervals(), 1);
  page.open();
  assert.equal(env.intervals(), 1, 'a second open never starts a second interval');
  env.fetches.forEach((c) => { if (!c.settled) { c.reply(200, eventsBody); } });
  await flush();
  const base = env.fetches.length;

  // An unchanged signature does not refetch, a changed one does, once.
  page.update(report());
  assert.equal(env.fetches.length, base);
  page.update(report({ trust: { confirmed_places: 2, disputed_places: 0 } }));
  assert.equal(env.fetches.length, base + 1);
  page.update(report({ trust: { confirmed_places: 2, disputed_places: 0 } }));
  assert.equal(env.fetches.length, base + 1);
  page.update(report({ trust: { confirmed_places: 2, disputed_places: 1 } }));
  assert.equal(env.fetches.length, base + 2);
  page.update(report({ trust: { confirmed_places: 2, disputed_places: 1 }, visibility: 'hidden' }));
  assert.equal(env.fetches.length, base + 3);
  page.update(report({ trust: { confirmed_places: 2, disputed_places: 1 }, visibility: 'hidden', view: 'resolved' }));
  assert.equal(env.fetches.length, base + 4);
  env.fetches.forEach((c) => { if (!c.settled) { c.reply(200, eventsBody); } });
  await flush();

  // The interval fetches while visible and skips while hidden.
  const before = env.fetches.length;
  env.advance(30000);
  assert.equal(env.fetches.length, before + 1, 'the 30 second poll');
  env.fetches.forEach((c) => { if (!c.settled) { c.reply(200, eventsBody); } });
  await flush();
  env.doc.visibilityState = 'hidden';
  env.advance(30000);
  assert.equal(env.fetches.length, before + 1, 'nothing while the tab is hidden');
  env.doc.visibilityState = 'visible';

  // Close stops the interval; a hidden host stops it too.
  page.close();
  assert.equal(env.intervals(), 0);
  env.advance(60000);
  assert.equal(env.fetches.length, before + 1);
  page.open();
  assert.equal(env.intervals(), 1);
  page.setHosted(false);
  assert.equal(env.intervals(), 0, 'a page no host shows polls nothing');
  assert.equal(page.el.hidden, true);
  assert.equal(page.share.hidden, true);
});

test('a late answer after close is dropped', async () => {
  const env = load();
  const page = openHosted(env);
  page.close();
  env.fetches[0].reply(200, eventsBody);
  await flush();
  assert.equal(findAll(page.el, 'report-timeline__text').length, 0, 'a closed page draws nothing from an old answer');
});

test('time words', () => {
  const env = load();
  const fmt = env.Page.formatTimelineTime;
  assert.equal(typeof fmt, 'function');
  const day = (y, m, d, h, min, s = 0) => new Date(y, m, d, h, min, s);
  const now = day(2026, 9, 26, 15, 0); // a Monday
  assert.equal(fmt(day(2026, 9, 26, 14, 10).toISOString(), now), '2:10 pm');
  assert.equal(fmt(day(2026, 9, 26, 0, 5).toISOString(), now), '12:05 am');
  assert.equal(fmt(day(2026, 9, 26, 12, 0).toISOString(), now), '12:00 pm');
  assert.equal(fmt(day(2026, 9, 26, 9, 7).toISOString(), now), '9:07 am');
  // Another day: the short weekday first. 2026-10-26 is a Monday.
  assert.equal(fmt(day(2026, 9, 26, 14, 10).toISOString(), day(2026, 9, 27, 9, 0)), 'Mon 2:10 pm');
  assert.equal(fmt(day(2026, 9, 25, 23, 59).toISOString(), day(2026, 9, 27, 9, 0)), 'Sun 11:59 pm');
  // Your own line made just now.
  const justNow = day(2026, 9, 26, 14, 59, 40).toISOString();
  assert.equal(fmt(justNow, now, true), 'Now');
  assert.equal(fmt(justNow, now, false), '2:59 pm');
  assert.equal(fmt(day(2026, 9, 26, 14, 58, 0).toISOString(), now, true), '2:58 pm', 'older than a minute is a clock time');
  assert.equal(fmt('not a time', now), '');
});

test('share hidden without the window script', () => {
  let env = load();
  let page = env.Page.create(report());
  page.setHosted(true);
  assert.equal(page.share.hidden, true, 'no PinalertShare, no Share button');
  page.update(report());
  assert.equal(page.share.hidden, true);

  env = load({ share: true });
  page = env.Page.create(report({ category: 'flood' }));
  assert.equal(page.share.hidden, true, 'not shown before a host shows the page');
  page.setHosted(true);
  assert.equal(page.share.hidden, false);
  const event = page.share.fire('click');
  assert.equal(event.stopped, true);
  assert.equal(env.sandbox.PinalertShare.calls.length, 1);
  assert.deepEqual({ ...env.sandbox.PinalertShare.calls[0], opener: undefined }, { reportId: 'r1', category: 'flood', opener: undefined });
  assert.equal(env.sandbox.PinalertShare.calls[0].opener, page.share);
  assert.equal(page.share.hasAttribute('aria-disabled'), false, 'Share is never greyed by distance');
});

test('the page script has no markup sink', () => {
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write/.test(pageSource), false);
});

// ---------------------------------------------------------------------------
// Child order across a resize (research Pitfall 5b)
// ---------------------------------------------------------------------------

function buildOrderFixture() {
  const doc = { createElement(tag) { return new FakeEl(doc, tag); } };
  const make = (cls) => { const n = doc.createElement('div'); n.className = cls; return n; };
  const body = make('popup-card__body');
  const header = make('header');
  const title = make('title');
  const meta = make('meta');
  const trust = make('trust');
  const fade = make('fade');
  const actions = make('actions');
  const muteEl = make('mute');
  const muteError = make('mute-error');
  const flag = make('flag');
  const pageEl = make('page');
  [header, title, meta, trust, fade, actions, pageEl].forEach((n) => body.appendChild(n));
  actions.appendChild(muteEl);
  actions.appendChild(muteError);
  actions.appendChild(flag);
  const card = {
    trust: { el: trust },
    fade,
    actions,
    mute: { el: muteEl, error: muteError },
    flag: { el: flag },
    page: { el: pageEl }
  };
  return { body, card, names: () => body.children.map((n) => n.className) };
}

test('child order across a resize', () => {
  const source = between(mapSource, '  function arrangeCard(card, phone) {', '\n  }\n') + '\n  }\n';
  const sandbox = {};
  vm.createContext(sandbox);
  vm.runInContext(source + '\nthis.arrangeCard = arrangeCard;', sandbox, { filename: 'arrangeCard' });

  const fixture = buildOrderFixture();
  sandbox.arrangeCard(fixture.card, true);
  assert.deepEqual(fixture.names(),
    ['header', 'title', 'meta', 'fade', 'actions', 'trust', 'mute', 'mute-error', 'flag', 'page'],
    'the phone order: Stop updates and Flag above the page');

  sandbox.arrangeCard(fixture.card, false);
  assert.equal(fixture.body.lastChild.className, 'page', 'the laptop pass leaves the page last');
  assert.deepEqual(fixture.names(), ['header', 'title', 'meta', 'trust', 'fade', 'actions', 'page']);

  sandbox.arrangeCard(fixture.card, true);
  assert.deepEqual(fixture.names(),
    ['header', 'title', 'meta', 'fade', 'actions', 'trust', 'mute', 'mute-error', 'flag', 'page'],
    'back to the phone after a resize');
  sandbox.arrangeCard(fixture.card, true);
  assert.equal(fixture.body.lastChild.className, 'page', 'a repeat placement keeps the page last');
});

// Amended 2026-10-29 by Phase 8 plan 16 (D-02): the page builds the comments
// section into its mount point and forwards update, open and close, only while
// a host shows it.
test('the comments section is built into the mount and follows the page', () => {
  const env = load({ comments: true });
  const page = env.Page.create(report());
  const views = env.sandbox.PinalertComments.views;
  assert.equal(views.length, 1, 'one comments section per page');
  const mount = find(page.el, 'report-page__comments');
  assert.equal(views[0].mount, mount, 'built into the page mount point');
  assert.equal(views[0].rep.id, 'r1');
  page.open();
  assert.deepEqual(views[0].calls, [], 'not hosted: the comments fetch nothing');
  page.setHosted(true);
  assert.deepEqual(views[0].calls, ['open'], 'hosted and open: the comments open');
  page.update(report({ comment_count: 3 }));
  assert.deepEqual(views[0].calls, ['open', 'update:r1']);
  page.setHosted(false);
  assert.equal(views[0].calls[views[0].calls.length - 1], 'close');
  page.setHosted(true);
  page.close();
  assert.equal(views[0].calls[views[0].calls.length - 1], 'close');
});
