// votes_stars_test.mjs - runs the real web/static/js/votes.js and the real
// web/static/js/stars.js in a Node sandbox and checks the skippable star box
// that follows "Yes, mark resolved" (Phase 9 plan 17, D-02, UI-SPEC 3b, V18):
//
//   - the box exists only when the server's resolve answer carries rate_prompt,
//     and only when the star script is on the page;
//   - it is one div.rate-box inside the card's actions group, right after the vote
//     block and before the Share button, never a sibling in the popup wrapper
//     (review M6), so it moves with the card into the laptop panel and the phone
//     detail pane and sits above the timeline (card.page);
//   - Skip removes it, sends nothing, and keeps it away for that report;
//   - a tap saves through the shared control and the box collapses to the line
//     "You rated {username} 4 stars.";
//   - an Undo of the resolve, in the card or in the toast, removes it;
//   - a report the resolve closed (its popup gone after the refetch) gets none;
//   - the first post or vote pop-up is waited for when it is on the page;
//   - every string goes in by textContent.
//
// Run it with: node --test web/votes_stars_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the two scripts call: a small element model with a
// class list, a class selector, closest, contains and a connected check.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const read = (name) => fs.readFileSync(path.join(here, 'static', 'js', name), 'utf8');
const votesSource = read('votes.js');
const starsSource = read('stars.js');
const mapSource = read('map.js');

const NAME = 'star box: ';
const REPORT = 41;
const settle = async () => { for (let i = 0; i < 12; i += 1) { await new Promise((r) => setImmediate(r)); } };

class FakeEl {
  constructor(doc, tag) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.nodeType = 1;
    this.children = [];
    this.parentNode = null;
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.className = '';
    this.dataset = {};
    this.hidden = false;
    this.disabled = false;
    this.type = '';
    this.id = '';
  }
  get classList() {
    const el = this;
    const names = () => (el.className ? el.className.split(/\s+/).filter(Boolean) : []);
    return {
      add: (c) => { if (!names().includes(c)) { el.className = names().concat(c).join(' '); } },
      remove: (c) => { el.className = names().filter((n) => n !== c).join(' '); },
      contains: (c) => names().includes(c),
      toggle(c, force) {
        const on = force === undefined ? !names().includes(c) : !!force;
        if (on) { this.add(c); } else { this.remove(c); }
        return on;
      }
    };
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  // No markup sink: the scripts must never use these.
  set innerHTML(v) { throw new Error('innerHTML was used: ' + v); }
  get innerHTML() { throw new Error('innerHTML was read'); }
  set outerHTML(v) { throw new Error('outerHTML was used: ' + v); }
  insertAdjacentHTML() { throw new Error('insertAdjacentHTML was used'); }
  get isConnected() {
    for (let n = this; n; n = n.parentNode) { if (n === this.doc.body) { return true; } }
    return false;
  }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  removeAttribute(n) { delete this.attrs[n]; }
  detach(node) { if (node.parentNode) { node.parentNode.removeChild(node); } }
  appendChild(node) { this.detach(node); this.children.push(node); node.parentNode = this; return node; }
  insertBefore(node, ref) {
    this.detach(node);
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
  get nextSibling() {
    if (!this.parentNode) { return null; }
    const sibs = this.parentNode.children;
    return sibs[sibs.indexOf(this) + 1] || null;
  }
  get previousSibling() {
    if (!this.parentNode) { return null; }
    const sibs = this.parentNode.children;
    return sibs[sibs.indexOf(this) - 1] || null;
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  contains(node) {
    for (let n = node; n; n = n.parentNode) { if (n === this) { return true; } }
    return false;
  }
  closest(selector) {
    const klass = selector.startsWith('.') ? selector.slice(1) : null;
    for (let n = this; n; n = n.parentNode) {
      if (klass && n.classList && n.classList.contains(klass)) { return n; }
    }
    return null;
  }
  querySelectorAll(selector) {
    const classes = selector.split(',').map((s) => s.trim()).filter((s) => /^\.[A-Za-z0-9_-]+$/.test(s)).map((s) => s.slice(1));
    const found = [];
    const walk = (node) => {
      node.children.forEach((child) => {
        if (classes.some((c) => child.classList.contains(c))) { found.push(child); }
        walk(child);
      });
    };
    walk(this);
    return found;
  }
  querySelector(selector) { return this.querySelectorAll(selector)[0] || null; }
  getClientRects() { return [{}]; }
  focus() { this.doc.activeElement = this; }
}

class FakeDoc {
  constructor() {
    this.body = new FakeEl(this, 'body');
    this.activeElement = this.body;
  }
  createElement(tag) { return new FakeEl(this, tag); }
  getElementById(id) { return this.ids && this.ids[id] ? this.ids[id] : null; }
  querySelectorAll(selector) { return this.body.querySelectorAll(selector); }
  querySelector(selector) { return this.body.querySelector(selector); }
  dispatchEvent() {}
}

// fire dispatches a click on el and lets it bubble through the delegated
// listeners, the way the browser does.
function fire(el, type = 'click') {
  let stopped = false;
  const evt = { type, target: el, stopPropagation() { stopped = true; }, key: '' };
  for (let n = el; n && !stopped; n = n.parentNode) {
    (n.handlers[type] || []).forEach((fn) => fn(evt));
  }
}

// load evaluates stars.js (unless withStars is false) and votes.js on a small
// page. answers maps a request tail to { status, body }; a resolve answers with
// the prompt it is given.
function load({ prompt, withStars = true, changesAsk = null, onRefetch = null, resolveBody = null, withHost = false, withObserver = false } = {}) {
  const doc = new FakeDoc();
  const calls = [];
  // 2026-10-30 (skeptic SK-W01): a fake MutationObserver that records what it
  // watches, so a test can say "a window opened" by calling its callback.
  const observers = [];
  const toasts = [];
  const timers = [];
  doc.ids = {};
  // 2026-10-30 (09-REVIEW K2): a page that carries the persistent star box host
  // beside the message, as index and profile do. The message is there too.
  let host = null;
  let toastEl = null;
  if (withHost) {
    toastEl = doc.createElement('div');
    toastEl.id = 'toast';
    doc.ids.toast = toastEl;
    doc.body.appendChild(toastEl);
    host = doc.createElement('div');
    host.id = 'rate-host';
    doc.ids['rate-host'] = host;
    doc.body.appendChild(host);
  }
  const answers = { people: { status: 204, body: null } };
  const sandbox = {
    document: doc,
    navigator: { onLine: true },
    sessionStorage: {
      getItem: () => JSON.stringify({ latitude: 12.9716, longitude: 77.5946, at: Date.now() }),
      setItem() {},
      removeItem() {}
    },
    localStorage: { getItem: () => null, setItem() {}, removeItem() {} },
    CustomEvent: class CustomEvent { constructor(type) { this.type = type; } },
    setTimeout(fn, ms) { timers.push({ fn, ms, live: true }); return timers.length; },
    clearTimeout(id) { if (timers[id - 1]) { timers[id - 1].live = false; } },
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    Pinalert: {
      setText(el, text) { el.textContent = text; },
      showToast(message, action) { toasts.push({ message, action }); },
      state: { status: 'ok', focusId: null, view: 'active', category: '', center: { lat: 0, lon: 0 }, reports: [] },
      setFocusId() {},
      fetchReports() {
        calls.push('refetch');
        if (onRefetch) { onRefetch(); }
        return Promise.resolve();
      },
      formatWait: (s) => s + ' seconds',
      serverNow: () => Date.now()
    },
    fetch(target, init) {
      const tail = String(target);
      calls.push((init && init.method ? init.method : 'GET') + ' ' + tail + (init && init.body ? ' ' + init.body : ''));
      let status = 200;
      let body = {};
      if (tail.startsWith('/api/people/')) {
        status = answers.people.status;
        body = answers.people.body;
      } else if (tail.endsWith('/resolve')) {
        body = resolveBody || { visibility: 'live', reason: 'confirmed', undos_left: 2 };
        if (prompt) { body = Object.assign({}, body, { rate_prompt: prompt }); }
      } else if (tail.endsWith('/unresolve')) {
        body = { visibility: 'live', reason: 'confirmed', undos_left: 1 };
      }
      return Promise.resolve({
        ok: status < 400,
        status,
        headers: { get: () => null },
        json: () => (body === null ? Promise.reject(new Error('no body')) : Promise.resolve(body))
      });
    },
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent
  };
  sandbox.window = sandbox;
  if (withObserver) {
    sandbox.MutationObserver = class MutationObserver {
      constructor(callback) { this.callback = callback; this.target = null; this.options = null; this.live = false; observers.push(this); }
      observe(target, options) { this.target = target; this.options = options; this.live = true; }
      disconnect() { this.live = false; }
    };
  }
  if (changesAsk) { sandbox.PinalertChangesAsk = changesAsk; }
  vm.createContext(sandbox);
  if (withStars) { vm.runInContext(starsSource, sandbox); }
  vm.runInContext(votesSource, sandbox);
  return { votes: sandbox.PinalertVotes, doc, calls, toasts, answers, sandbox, timers, host, toastEl, observers };
}

// buildCard mirrors the order map.js buildPopupCard gives the card: a popup
// wrapper holding the card node, whose body holds the actions group (the vote
// block, then Share) and, last, the page node that carries the timeline.
function buildCard(m, id = REPORT) {
  const doc = m.doc;
  const make = (tag, cls) => { const e = doc.createElement(tag); e.className = cls || ''; return e; };
  const popup = make('div', 'leaflet-popup');
  popup.setAttribute('data-popup-open', 'true');
  const el = make('div', 'popup-card');
  const body = make('div', 'popup-card__body');
  const actions = make('div', 'popup-card__actions');
  const block = m.votes.createVoteBlock(id);
  actions.appendChild(block.controls);
  actions.appendChild(block.note);
  actions.appendChild(block.error);
  const share = make('button', 'report-page__share');
  actions.appendChild(share);
  const page = make('div', 'report-page');
  body.appendChild(actions);
  body.appendChild(page);
  el.appendChild(body);
  popup.appendChild(el);
  doc.body.appendChild(popup);
  return { popup, el, body, actions, block, share, page };
}

async function resolve(card) {
  const affirm = card.block.controls.querySelector('.vote-btn--confirm-resolve');
  fire(affirm);
  await settle();
}

const boxesIn = (root) => root.querySelectorAll('.rate-box');
const PROMPT = { username: 'asha_k', profile_path: '/u/asha_k' };

test(NAME + 'a resolve answer with rate_prompt mounts one box right after the vote block, not in the popup wrapper', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  const boxes = boxesIn(m.doc.body);
  assert.equal(boxes.length, 1);
  const box = boxes[0];
  assert.equal(box.parentNode, card.actions, 'inside the actions group');
  assert.equal(box.previousSibling, card.block.error, 'right after the vote block');
  assert.equal(box.nextSibling, card.share, 'before the Share button');
  assert.equal(card.popup.children.length, 1, 'the popup wrapper holds only the card node');
  assert.ok(card.el.contains(box));
  assert.equal(box.dataset.reportId, String(REPORT));
  assert.equal(box.querySelector('.stars-control__legend').textContent, 'Rate asha_k');
  assert.equal(box.querySelector('.rate-box__skip').textContent, 'Skip');
  assert.equal(box.querySelectorAll('.stars-control__star').length, 5);
  assert.equal(box.querySelector('.rate-box__status').getAttribute('role'), 'status');
  assert.equal(m.toasts.length, 1, 'the resolve message was shown as before');
});

test(NAME + 'an answer without rate_prompt mounts nothing', async () => {
  const m = load({ prompt: null });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.toasts.length, 1);
});

test(NAME + 'a page without the star script mounts nothing and still saves the resolve', async () => {
  const m = load({ prompt: PROMPT, withStars: false });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.toasts.length, 1);
  assert.ok(m.calls.some((c) => c.includes('/resolve')));
  assert.equal(card.block.error.hidden, true, 'no error line was shown');
});

test(NAME + 'a prompt with no username mounts nothing', async () => {
  const m = load({ prompt: { username: '', profile_path: '' } });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0);
});

test(NAME + 'Skip removes the box, sends nothing, and a second resolve of the same report shows none', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  const before = m.calls.length;
  fire(boxesIn(m.doc.body)[0].querySelector('.rate-box__skip'));
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.calls.length, before, 'Skip sent no request');
  assert.ok(!m.calls.some((c) => c.includes('/api/people/')));
  // The reader takes the resolve back and marks it again in the same view.
  fire(card.block.controls.querySelector('.vote-btn--takeback'));
  await settle();
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0, 'the box does not come back for this report');
});

test(NAME + 'Skip is not a rating: a different report still gets its own box', async () => {
  const m = load({ prompt: PROMPT });
  const first = buildCard(m, 41);
  await resolve(first);
  fire(boxesIn(m.doc.body)[0].querySelector('.rate-box__skip'));
  const second = buildCard(m, 42);
  await resolve(second);
  assert.equal(boxesIn(m.doc.body).length, 1);
  assert.equal(boxesIn(m.doc.body)[0].dataset.reportId, '42');
});

test(NAME + 'a tap saves through the shared control and collapses the box to one status line', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  const box = boxesIn(m.doc.body)[0];
  const stars = box.querySelectorAll('.stars-control__star');
  stars[3].focus();
  fire(stars[3]);
  await settle();
  assert.ok(m.calls.includes('POST /api/people/asha_k/rating {"stars":4}'), m.calls.join('\n'));
  assert.equal(box.querySelectorAll('.stars-control__star').length, 0, 'the five stars are gone');
  assert.equal(box.querySelector('.rate-box__skip'), null, 'Skip is gone');
  const status = box.querySelector('.rate-box__status');
  assert.equal(status.textContent, 'You rated asha_k 4 stars.');
  assert.equal(status.getAttribute('role'), 'status');
  assert.equal(m.doc.activeElement, status, 'focus that sat on a star moves to the line');
  assert.equal(boxesIn(m.doc.body).length, 1, 'the box stays until the report view closes');
});

test(NAME + 'one star reads 1 star in the line', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  fire(boxesIn(m.doc.body)[0].querySelectorAll('.stars-control__star')[0]);
  await settle();
  assert.equal(boxesIn(m.doc.body)[0].querySelector('.rate-box__status').textContent, 'You rated asha_k 1 star.');
});

test(NAME + 'a refusal shows the server sentence in the box and keeps the stars', async () => {
  const m = load({ prompt: PROMPT });
  m.answers.people = { status: 409, body: { error: { field: 'stars', message: 'You can\'t rate this person right now.' } } };
  const card = buildCard(m);
  await resolve(card);
  const box = boxesIn(m.doc.body)[0];
  fire(box.querySelectorAll('.stars-control__star')[1]);
  await settle();
  const line = box.querySelector('.inline-error');
  assert.equal(line.hidden, false);
  assert.equal(line.textContent, 'You can\'t rate this person right now.');
  assert.equal(box.querySelectorAll('.stars-control__star').length, 5);
  assert.equal(box.querySelector('.rate-box__status').textContent, '');
});

test(NAME + 'a 404 from the rating route removes the box quietly', async () => {
  const m = load({ prompt: PROMPT });
  m.answers.people = { status: 404, body: { error: { field: 'person', message: 'Profile not found.' } } };
  const card = buildCard(m);
  await resolve(card);
  fire(boxesIn(m.doc.body)[0].querySelectorAll('.stars-control__star')[2]);
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 0);
});

// 2026-10-30 (09-REVIEW IN-B05): a 404 while a star has focus must not leave
// focus on a star that is no longer on the page; it goes to Undo as Skip does.
test(NAME + 'a 404 with focus on a star hands focus to Undo, not to a removed star', async () => {
  const m = load({ prompt: PROMPT });
  m.answers.people = { status: 404, body: { error: { field: 'person', message: 'Profile not found.' } } };
  const card = buildCard(m);
  await resolve(card);
  const undo = card.block.takeBack && card.block.takeBack.undoBtn;
  assert.ok(undo, 'the card has an Undo');
  undo.hidden = false; // the refetch is faked here, so show it as the card would
  const star = boxesIn(m.doc.body)[0].querySelectorAll('.stars-control__star')[2];
  star.focus();
  fire(star);
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.notEqual(m.doc.activeElement, star, 'focus is not left on the removed star');
  assert.equal(m.doc.activeElement, undo);
});

test(NAME + 'the Undo in the card removes the box', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1);
  fire(card.block.controls.querySelector('.vote-btn--takeback'));
  await settle();
  assert.ok(m.calls.some((c) => c.includes('/unresolve')));
  assert.equal(boxesIn(m.doc.body).length, 0);
});

test(NAME + 'the Undo in the message removes the box', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1);
  const action = m.toasts[0].action;
  assert.ok(action && typeof action.onClick === 'function', 'the message carries the short Undo');
  await action.onClick();
  assert.equal(boxesIn(m.doc.body).length, 0);
});

test(NAME + 'resolve, Undo, resolve again leaves exactly one box', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  fire(card.block.controls.querySelector('.vote-btn--takeback'));
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 0);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1);
});

test(NAME + 'a resolve that closes the report, whose card is gone after the refetch, shows no box', async () => {
  let card = null;
  const m = load({
    prompt: PROMPT,
    resolveBody: { visibility: 'retracted', reason: 'resolved', undos_left: 2 },
    onRefetch: () => {
      // map.js removes the marker, the popup and the card of a closed report.
      card.popup.setAttribute('data-popup-open', 'false');
      card.popup.parentNode.removeChild(card.popup);
    }
  });
  card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.toasts.length, 1, 'the message still shows');
});

test(NAME + 'a closing popup that is still attached for its fade gets no box', async () => {
  let card = null;
  const m = load({
    prompt: PROMPT,
    onRefetch: () => { card.popup.setAttribute('data-popup-open', 'false'); }
  });
  card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0);
});

test(NAME + 'with the first post or vote pop-up on the page, the box waits until it is answered', async () => {
  let release = null;
  const pending = new Promise((resolveIt) => { release = resolveIt; });
  const m = load({ prompt: PROMPT, changesAsk: { offer() { return Promise.resolve(); }, settled() { return pending; } } });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0, 'not while the pop-up is open');
  release();
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 1);
});

test(NAME + 'a pop-up that is already settled lets the box mount', async () => {
  const m = load({ prompt: PROMPT, changesAsk: { offer() { return Promise.resolve(); }, settled() { return Promise.resolve(); } } });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1);
});

test(NAME + 'review M6: resolving in the laptop bubble and then entering the panel leaves exactly one box, inside the moved card', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1);
  // enterPanel closes the popup and opens it again in the detail pane host: the
  // same card node moves, nothing is rebuilt.
  const panel = m.doc.createElement('div');
  panel.className = 'leaflet-popup';
  panel.setAttribute('data-popup-open', 'true');
  m.doc.body.appendChild(panel);
  panel.appendChild(card.el);
  card.popup.parentNode.removeChild(card.popup);
  const boxes = boxesIn(m.doc.body);
  assert.equal(boxes.length, 1);
  assert.equal(boxesIn(card.el).length, 1, 'the box is inside the moved card');
  assert.equal(boxes[0].parentNode, card.actions);
  assert.equal(card.popup.children.length, 0, 'nothing is left behind in the old wrapper');
  // A later resolve of the same report does not stack a second box.
  fire(card.block.controls.querySelector('.vote-btn--takeback'));
  await settle();
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1);
});

test(NAME + 'review M6: in the phone detail pane the box sits inside the card, before the page node, so it is above the timeline', async () => {
  const m = load({ prompt: PROMPT });
  const card = buildCard(m);
  await resolve(card);
  const box = boxesIn(m.doc.body)[0];
  assert.ok(card.el.contains(box));
  const bodyKids = card.body.children;
  assert.equal(bodyKids[bodyKids.length - 1], card.page, 'the page node is still the last child of the card body');
  assert.ok(bodyKids.indexOf(box.parentNode) < bodyKids.indexOf(card.page), 'the box comes before the page node');
  assert.equal(card.page.querySelector('.rate-box'), null, 'the box is not inside the page node');
});

test(NAME + 'map.js keeps the order the box relies on: the vote block, then Share in the actions group, the page node last', () => {
  const at = (needle) => {
    const i = mapSource.indexOf(needle);
    assert.ok(i > -1, 'map.js lacks ' + needle);
    return i;
  };
  const controls = at('actions.appendChild(block.controls);');
  const note = at('actions.appendChild(block.note);');
  const error = at('actions.appendChild(block.error);');
  const share = at('actions.appendChild(page.share);');
  assert.ok(controls < note && note < error && error < share, 'controls, note, error, then Share');
  assert.ok(at('body.appendChild(actions);') < at('body.appendChild(page.el);'), 'the page node comes after the actions group');
  assert.ok(!/rate-box/.test(mapSource), 'map.js does not build the box');
});

test(NAME + 'every string reaches the page by textContent: the scripts use no markup sink', () => {
  for (const [file, source] of [['votes.js', votesSource], ['stars.js', starsSource]]) {
    assert.ok(!/\.innerHTML|\.outerHTML|insertAdjacentHTML|document\.write\(/.test(source), file + ' uses a markup sink');
  }
});

test(NAME + 'a username with markup is shown as text and builds no element', async () => {
  const m = load({ prompt: { username: '<b>x</b>', profile_path: '/u/x' } });
  const card = buildCard(m);
  await resolve(card);
  const box = boxesIn(m.doc.body)[0];
  assert.equal(box.querySelector('.stars-control__legend').textContent, 'Rate <b>x</b>');
  assert.equal(box.querySelectorAll('.stars-control__star').length, 5);
});

// 2026-10-30 (09-REVIEW K2, UI-SPEC 3b): the reader whose resolve closes the
// report loses the card, and the message cannot hold the box (it hides, and any
// later message empties it). The page carries #rate-host beside the message, and
// votes.js mounts the box there when the card is not live.
function closeOnRefetch(holder) {
  return () => {
    holder.card.popup.setAttribute('data-popup-open', 'false');
    if (holder.card.popup.parentNode) { holder.card.popup.parentNode.removeChild(holder.card.popup); }
  };
}

const CLOSED = { visibility: 'retracted', reason: 'resolved', undos_left: 2 };

test(NAME + 'K2: a resolve that closes the report mounts the box in the page host, with no live region and no move of focus', async () => {
  const holder = { card: null };
  const m = load({ prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) });
  holder.card = buildCard(m);
  const focusBefore = m.doc.activeElement;
  await resolve(holder.card);
  const boxes = boxesIn(m.doc.body);
  assert.equal(boxes.length, 1);
  assert.equal(boxes[0].parentNode, m.host, 'the box is inside #rate-host');
  assert.equal(boxes[0].dataset.reportId, String(REPORT));
  assert.equal(boxes[0].querySelector('.stars-control__legend').textContent, 'Rate asha_k');
  assert.equal(boxes[0].querySelectorAll('.stars-control__star').length, 5);
  assert.equal(boxesIn(m.toastEl).length, 0, 'the message holds nothing');
  assert.equal(m.host.getAttribute('role'), null, 'the host is not a live region');
  assert.equal(m.host.getAttribute('aria-live'), null);
  assert.equal(boxes[0].getAttribute('role'), null, 'the box is not a live region');
  assert.equal(boxes[0].getAttribute('aria-live'), null);
  assert.ok(!m.host.contains(m.doc.activeElement), 'focus did not move into the host');
  assert.equal(m.doc.activeElement, focusBefore);
  assert.equal(m.toasts.length, 1, 'the resolve message was shown as before');
});

test(NAME + 'K2: a page without the host shows no box for a closed report', async () => {
  const holder = { card: null };
  const m = load({ prompt: PROMPT, withHost: false, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) });
  holder.card = buildCard(m);
  await resolve(holder.card);
  assert.equal(boxesIn(m.doc.body).length, 0);
});

test(NAME + 'K2: a card that stays open keeps the box in the card, not in the host', async () => {
  const m = load({ prompt: PROMPT, withHost: true });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.host).length, 0);
  assert.equal(boxesIn(card.actions).length, 1);
});

test(NAME + 'K2: Skip in the host removes the box, sends nothing and keeps it away for that report', async () => {
  const holder = { card: null };
  const m = load({ prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) });
  holder.card = buildCard(m);
  await resolve(holder.card);
  const before = m.calls.length;
  fire(boxesIn(m.host)[0].querySelector('.rate-box__skip'));
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.host.children.length, 0);
  assert.equal(m.calls.length, before);
});

test(NAME + 'K2: a rating in the host shows its one line, then the box leaves', async () => {
  const holder = { card: null };
  const m = load({ prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) });
  holder.card = buildCard(m);
  await resolve(holder.card);
  const box = boxesIn(m.host)[0];
  assert.equal(m.timers.filter((t) => t.live && t.ms === 6000).length, 0, 'no leave timer before a rating');
  fire(box.querySelectorAll('.stars-control__star')[3]);
  await settle();
  assert.ok(m.calls.includes('POST /api/people/asha_k/rating {"stars":4}'), m.calls.join('\n'));
  assert.equal(box.querySelector('.rate-box__status').textContent, 'You rated asha_k 4 stars.');
  assert.equal(boxesIn(m.host).length, 1, 'the line is read first');
  const leave = m.timers.filter((t) => t.live && t.ms === 6000);
  assert.equal(leave.length, 1);
  leave[0].fn();
  assert.equal(boxesIn(m.doc.body).length, 0, 'the box leaves after the line');
  assert.equal(m.host.children.length, 0);
});

test(NAME + 'K2: Undo in the message removes the box from the host, and so does Undo in the card', async () => {
  for (const via of ['message', 'card']) {
    const holder = { card: null };
    const m = load({ prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) });
    holder.card = buildCard(m);
    await resolve(holder.card);
    assert.equal(boxesIn(m.host).length, 1);
    if (via === 'message') {
      await m.toasts[0].action.onClick();
    } else {
      fire(holder.card.block.controls.querySelector('.vote-btn--takeback'));
      await settle();
    }
    assert.equal(boxesIn(m.doc.body).length, 0, 'removed by Undo in the ' + via);
  }
});

test(NAME + 'K2: a 404 on a rating in the host removes the box and hands focus to the message Undo', async () => {
  const holder = { card: null };
  const m = load({ prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) });
  holder.card = buildCard(m);
  const undo = m.doc.createElement('button');
  undo.className = 'toast__action';
  m.toastEl.appendChild(undo);
  await resolve(holder.card);
  m.answers.people = { status: 404, body: { error: { field: 'person', message: 'Profile not found.' } } };
  const star = boxesIn(m.host)[0].querySelectorAll('.stars-control__star')[2];
  star.focus();
  fire(star);
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.doc.activeElement, undo);
});

test(NAME + 'K2: with the first post or vote pop-up on the page the host box waits too', async () => {
  let release = null;
  const pending = new Promise((resolveIt) => { release = resolveIt; });
  const holder = { card: null };
  const m = load({
    prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder),
    changesAsk: { offer() { return Promise.resolve(); }, settled() { return pending; } }
  });
  holder.card = buildCard(m);
  await resolve(holder.card);
  assert.equal(boxesIn(m.doc.body).length, 0);
  release();
  await settle();
  assert.equal(boxesIn(m.host).length, 1);
});

// 2026-10-30 (skeptic SK-W01): the box in the page host must never cover a
// control. It leaves when another report opens (map.js calls
// dismissHostedRateBoxes on every popup open), when any window opens (a watch
// on the hidden and open attributes that runs only while a hosted box exists),
// and after a fixed time of its own, unless focus is inside it.
async function hostedBox(extra = {}) {
  const holder = { card: null };
  const m = load(Object.assign({ prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder) }, extra));
  holder.card = buildCard(m);
  await resolve(holder.card);
  assert.equal(boxesIn(m.host).length, 1, 'the hosted box is there');
  return m;
}

test(NAME + 'SK-W01: opening another report drops the hosted box', async () => {
  const m = await hostedBox();
  assert.equal(typeof m.votes.dismissHostedRateBoxes, 'function');
  m.votes.dismissHostedRateBoxes();
  assert.equal(boxesIn(m.doc.body).length, 0);
  assert.equal(m.host.children.length, 0);
});

test(NAME + 'SK-W01: dropping hosted boxes leaves the box inside an open card alone', async () => {
  const m = load({ prompt: PROMPT, withHost: true });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(card.actions).length, 1);
  assert.equal(typeof m.votes.dismissHostedRateBoxes, 'function');
  m.votes.dismissHostedRateBoxes();
  assert.equal(boxesIn(card.actions).length, 1, 'the card keeps its own box');
});

test(NAME + 'SK-W01: map.js tells votes.js on every popup open', () => {
  const at = mapSource.indexOf('function onPopupOpen(evt) {');
  assert.ok(at > -1, 'map.js lacks onPopupOpen');
  const head = mapSource.slice(at, at + 1400);
  assert.ok(head.includes('PinalertVotes.dismissHostedRateBoxes()'), 'onPopupOpen does not drop the hosted box');
  assert.ok(head.indexOf('dismissHostedRateBoxes') < head.indexOf('isOfficialPopup'), 'the official card path drops it too');
});

test(NAME + 'SK-W01: a window opening drops the hosted box, a window closing does not', async () => {
  const m = await hostedBox({ withObserver: true });
  assert.equal(m.observers.length, 1, 'one watch started with the hosted box');
  const watch = m.observers[0];
  assert.equal(watch.live, true);
  assert.deepEqual(Array.from(watch.options.attributeFilter).sort(), ['hidden', 'open']);
  assert.equal(watch.options.subtree, true);
  const win = m.doc.createElement('div');
  win.className = 'app-window people-list-window';
  win.hidden = true;
  watch.callback([{ target: win, attributeName: 'hidden' }]);
  assert.equal(boxesIn(m.host).length, 1, 'a window that is still hidden changes nothing');
  const other = m.doc.createElement('div');
  other.className = 'something-else';
  other.hidden = false;
  watch.callback([{ target: other, attributeName: 'hidden' }]);
  assert.equal(boxesIn(m.host).length, 1, 'a node that is not a window changes nothing');
  win.hidden = false;
  watch.callback([{ target: win, attributeName: 'hidden' }]);
  assert.equal(boxesIn(m.doc.body).length, 0, 'the open window took the box away');
  assert.equal(watch.live, false, 'the watch ends with the last hosted box');
});

test(NAME + 'SK-W01: a dialog element that opens drops the hosted box too', async () => {
  const m = await hostedBox({ withObserver: true });
  const dlg = m.doc.createElement('dialog');
  dlg.open = true;
  m.observers[0].callback([{ target: dlg, attributeName: 'open' }]);
  assert.equal(boxesIn(m.doc.body).length, 0);
});

test(NAME + 'SK-W01: a box inside an open card starts no watch', async () => {
  const m = load({ prompt: PROMPT, withHost: true, withObserver: true });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(m.observers.length, 0);
});

test(NAME + 'SK-W01: the hosted box has a fixed end of its own, and focus inside it holds it once', async () => {
  const m = await hostedBox();
  const caps = m.timers.filter((t) => t.live && t.ms === 30000);
  assert.equal(caps.length, 1, 'one end timer of 30 seconds');
  const star = boxesIn(m.host)[0].querySelectorAll('.stars-control__star')[1];
  star.focus();
  caps[0].live = false; // a timer that fired is spent
  caps[0].fn();
  assert.equal(boxesIn(m.host).length, 1, 'a reader with focus in the box is not cut off');
  const again = m.timers.filter((t) => t.live && t.ms === 30000);
  assert.equal(again.length, 1, 'the end timer is set again');
  m.doc.activeElement = m.doc.body;
  again[0].live = false;
  again[0].fn();
  assert.equal(boxesIn(m.doc.body).length, 0, 'the box ended');
  assert.equal(m.timers.filter((t) => t.live && (t.ms === 30000 || t.ms === 6000)).length, 0, 'no timer is left running');
});

test(NAME + 'SK-W01: a box inside an open card has no end timer', async () => {
  const m = load({ prompt: PROMPT, withHost: true });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(m.timers.filter((t) => t.live && t.ms === 30000).length, 0);
});

test(NAME + 'SK-W01: Skip stops the end timer', async () => {
  const m = await hostedBox();
  fire(boxesIn(m.host)[0].querySelector('.rate-box__skip'));
  assert.equal(m.timers.filter((t) => t.live && t.ms === 30000).length, 0);
});

// 2026-10-30 (skeptic SK-I04): an Undo that lands while the first pop-up is
// still open must stop the box that is waiting for it.
test(NAME + 'SK-I04: a toast Undo taken while the pop-up is open means no hosted box after it', async () => {
  let release = null;
  const pending = new Promise((resolveIt) => { release = resolveIt; });
  const holder = { card: null };
  const m = load({
    prompt: PROMPT, withHost: true, resolveBody: CLOSED, onRefetch: closeOnRefetch(holder),
    changesAsk: { offer() { return Promise.resolve(); }, settled() { return pending; } }
  });
  holder.card = buildCard(m);
  await resolve(holder.card);
  assert.equal(boxesIn(m.doc.body).length, 0, 'waiting for the pop-up');
  await m.toasts[0].action.onClick();
  release();
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 0, 'no box for a resolve that no longer stands');
});

test(NAME + 'SK-I04: a card Undo taken while the pop-up is open means no box after it, and a later resolve is offered one', async () => {
  let release = null;
  const pending = new Promise((resolveIt) => { release = resolveIt; });
  const m = load({
    prompt: PROMPT, withHost: true,
    changesAsk: { offer() { return Promise.resolve(); }, settled() { return pending; } }
  });
  const card = buildCard(m);
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 0, 'waiting for the pop-up');
  fire(card.block.controls.querySelector('.vote-btn--takeback'));
  await settle();
  release();
  await settle();
  assert.equal(boxesIn(m.doc.body).length, 0, 'no box for a resolve that was taken back');
  await resolve(card);
  assert.equal(boxesIn(m.doc.body).length, 1, 'the next resolve of the same report is offered a box');
});
