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
  getElementById() { return null; }
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
function load({ prompt, withStars = true, changesAsk = null, onRefetch = null, resolveBody = null } = {}) {
  const doc = new FakeDoc();
  const calls = [];
  const toasts = [];
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
    setTimeout() { return 1; },
    clearTimeout() {},
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
  if (changesAsk) { sandbox.PinalertChangesAsk = changesAsk; }
  vm.createContext(sandbox);
  if (withStars) { vm.runInContext(starsSource, sandbox); }
  vm.runInContext(votesSource, sandbox);
  return { votes: sandbox.PinalertVotes, doc, calls, toasts, answers, sandbox };
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
