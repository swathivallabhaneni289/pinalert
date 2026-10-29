// person_mute_test.mjs - runs the real web/static/js/person.js in a Node sandbox
// and checks the Mute and Unmute item of the More menu (Phase 9 plan 20, D-12,
// UI-SPEC 1e): it is the first item, it posts once with no confirmation, it
// says the exact status sentence, the label follows the refetched muted flag,
// a failure shows the exact error line, focus returns to More, and no markup
// sink is used.
//
// Run it with: node --test web/person_mute_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what person.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'person.js'), 'utf8');
const template = fs.readFileSync(path.join(here, 'templates', 'person.html.tmpl'), 'utf8');

const flush = () => new Promise((resolve) => setImmediate(resolve));

const PROFILE = (muted) => ({
  followers_count: 12, followers_word: 'Followers', followers_text: '12 Followers',
  following_count: 3, following_word: 'Following', following_text: '3 Following',
  can_follow: true, following: false, follows_you: false, blocked_by_you: false, muted
});

// load builds another person's profile page with the three menu items and runs
// person.js on it. muted is the state the server drew. route(method, address)
// answers each call with {status, body}.
function load({ muted = false, route, online = true } = {}) {
  const sinks = [];
  const state = { active: null };
  const calls = [];
  const els = {};
  const reloads = [];

  function fakeEl(id, extra = {}) {
    const el = {
      id,
      hidden: false,
      attrs: {},
      listeners: [],
      focused: 0,
      textContent: '',
      disabled: false,
      children: [],
      get firstChild() { return this.children[0] || null; },
      addEventListener(type, fn) { this.listeners.push({ type, fn }); },
      removeEventListener() {},
      setAttribute(name, value) { this.attrs[name] = String(value); },
      getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
      removeAttribute(name) { delete this.attrs[name]; },
      appendChild(node) { this.children.push(node); return node; },
      contains(node) { return node === this || (this.kids || []).includes(node); },
      querySelectorAll() { return this.kids || []; },
      focus() { this.focused += 1; state.active = this; },
      fire(type, e = {}) { this.listeners.filter((l) => l.type === type).forEach((l) => l.fn(e)); },
      ...extra
    };
    for (const sink of ['innerHTML', 'outerHTML']) {
      Object.defineProperty(el, sink, { set(v) { sinks.push(sink + ' = ' + v); }, get() { return ''; } });
    }
    el.insertAdjacentHTML = (...args) => { sinks.push('insertAdjacentHTML ' + args.join(' ')); };
    return el;
  }
  function add(id, extra) { els[id] = fakeEl(id, extra); return els[id]; }

  add('person-page', { attrs: { 'data-username': 'asha_k', 'data-self': 'false' } });
  add('notice-live');
  add('person-header');
  add('person-reports-section');
  add('person-action-error', { hidden: true });
  add('person-follow', { textContent: 'Follow' });
  add('person-unfollow', { textContent: 'Following', hidden: true });
  add('person-more');
  const mute = add('person-mute-item', {
    attrs: { role: 'menuitem', 'data-muted': muted ? 'true' : 'false' },
    textContent: muted ? 'Unmute' : 'Mute'
  });
  const removeFollower = add('person-remove-follower', { attrs: { role: 'menuitem' }, hidden: true });
  const block = add('person-block', { attrs: { role: 'menuitem' } });
  add('person-more-menu', { hidden: true, kids: [mute, removeFollower, block] });
  add('person-blocked', { hidden: true });
  add('person-blocked-title');
  add('person-blocked-error', { hidden: true });
  add('person-unblock');
  add('person-followers-button');
  add('person-following-button');
  add('person-followers-count', { textContent: '12' });
  add('person-followers-word', { textContent: 'Followers' });
  add('person-following-count', { textContent: '3' });
  add('person-following-word', { textContent: 'Following' });
  const confirm = add('person-confirm', { open: false });
  confirm.showModal = function () { this.open = true; this.shown = (this.shown || 0) + 1; };
  confirm.close = function () { if (this.open) { this.open = false; this.fire('close'); } };
  add('person-confirm-title');
  add('person-confirm-text');
  add('person-confirm-error', { hidden: true });
  add('person-confirm-cancel');
  add('person-confirm-go');

  const document = {
    readyState: 'complete',
    getElementById: (id) => els[id] || null,
    createElement: (tag) => fakeEl(tag),
    addEventListener() {},
    removeEventListener() {},
    write(...args) { sinks.push('document.write ' + args.join(' ')); },
    get activeElement() { return state.active; }
  };
  const window = {
    setTimeout: () => 1,
    clearTimeout() {},
    confirm() { return false; },
    location: { reload() { reloads.push(1); } }
  };
  const fetch = (address, init = {}) => {
    const method = init.method || 'GET';
    calls.push({ method, address });
    return Promise.resolve(route(method, address)).then((answer) => ({
      status: answer.status,
      ok: answer.status >= 200 && answer.status < 300,
      json: () => (answer.body === undefined ? Promise.reject(new Error('no body')) : Promise.resolve(answer.body))
    }));
  };
  const navigator = { onLine: online };
  const sandbox = vm.createContext({ document, window, fetch, navigator, Promise, JSON, encodeURIComponent, isFinite, Math, Number, parseInt });
  vm.runInContext(source, sandbox, { filename: 'person.js' });
  window.PinalertPerson.init();

  const click = (id) => els[id].fire('click', { target: els[id] });
  const key = (id, k) => {
    const e = { key: k, prevented: false, preventDefault() { this.prevented = true; } };
    els[id].fire('keydown', e);
    return e;
  };
  return { els, calls, sinks, reloads, click, key };
}

// A mute or unmute answers 204 and the refetch carries the new flag.
const flagRoute = (flagAfter) => (method) => (method === 'POST' ? { status: 204 } : { status: 200, body: PROFILE(flagAfter) });

test('Mute posts once with no box, says the exact sentence and flips the label', async () => {
  const m = load({ muted: false, route: flagRoute(true) });
  m.click('person-mute-item');
  await flush();
  assert.deepEqual(m.calls.map((c) => c.method + ' ' + c.address), [
    'POST /api/people/asha_k/mute',
    'GET /api/people/asha_k'
  ]);
  assert.equal(m.els['person-confirm'].shown || 0, 0, 'no confirmation box');
  assert.equal(m.els['notice-live'].textContent, 'asha_k is muted. Their reports still show.');
  assert.equal(m.els['person-mute-item'].textContent, 'Unmute');
  assert.equal(m.els['person-mute-item'].getAttribute('data-muted'), 'true');
  assert.equal(m.els['person-action-error'].hidden, true);
  assert.equal(m.els['person-more'].focused, 1, 'focus lands on More');
  assert.equal(m.els['person-mute-item'].disabled, false);
  assert.equal(m.els['person-mute-item'].getAttribute('aria-busy'), null);
});

test('Unmute posts once, says the exact sentence and flips the label back', async () => {
  const m = load({ muted: true, route: flagRoute(false) });
  m.click('person-mute-item');
  await flush();
  assert.deepEqual(m.calls.map((c) => c.method + ' ' + c.address), [
    'POST /api/people/asha_k/unmute',
    'GET /api/people/asha_k'
  ]);
  assert.equal(m.els['notice-live'].textContent, 'asha_k is unmuted.');
  assert.equal(m.els['person-mute-item'].textContent, 'Mute');
  assert.equal(m.els['person-mute-item'].getAttribute('data-muted'), 'false');
  assert.equal(m.els['person-more'].focused, 1);
});

test('the label follows the refetched muted flag, not the tap', async () => {
  // The refetch says the person is not muted, so the label reads Mute.
  const m = load({ muted: false, route: flagRoute(false) });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.els['person-mute-item'].textContent, 'Mute');
  assert.equal(m.els['person-mute-item'].getAttribute('data-muted'), 'false');
});

test('a refetch with no muted flag leaves the label the tap set', async () => {
  const m = load({
    muted: false,
    route: (method) => (method === 'POST' ? { status: 204 } : { status: 200, body: { followers_count: 12 } })
  });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.els['person-mute-item'].textContent, 'Unmute');
});

test('Mute is the first item: opening the menu focuses it, Enter chooses it', async () => {
  const m = load({ muted: false, route: flagRoute(true) });
  m.click('person-more');
  assert.equal(m.els['person-more-menu'].hidden, false);
  assert.equal(m.els['person-mute-item'].focused, 1, 'the first item takes focus');
  const enter = m.key('person-more-menu', 'Enter');
  assert.equal(enter.prevented, true);
  await flush();
  assert.equal(m.els['person-more-menu'].hidden, true, 'the menu closes');
  assert.equal(m.calls[0].address, '/api/people/asha_k/mute');
  assert.equal(m.els['person-mute-item'].textContent, 'Unmute');
});

test('a second tap while the call is out sends nothing', async () => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const m = load({
    muted: false,
    route: (method) => (method === 'POST' ? gate.then(() => ({ status: 204 })) : { status: 200, body: PROFILE(true) })
  });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.els['person-mute-item'].disabled, true);
  assert.equal(m.els['person-mute-item'].getAttribute('aria-busy'), 'true');
  assert.equal(m.els['person-mute-item'].textContent, 'Mute', 'the label does not change while it is out');
  m.click('person-mute-item');
  await flush();
  assert.equal(m.calls.filter((c) => c.method === 'POST').length, 1);
  release();
  await flush();
  assert.equal(m.els['person-mute-item'].disabled, false);
});

test('a failed Mute shows Couldn\'t mute. Try again. and keeps the label', async () => {
  const m = load({ muted: false, route: () => ({ status: 500 }) });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.els['person-action-error'].textContent, 'Couldn\'t mute. Try again.');
  assert.equal(m.els['person-action-error'].hidden, false);
  assert.equal(m.els['person-mute-item'].textContent, 'Mute');
  assert.equal(m.els['notice-live'].textContent, '', 'no status line on a failure');
  assert.equal(m.els['person-more'].focused, 1);
  assert.equal(m.els['person-mute-item'].disabled, false);
});

test('a failed Unmute shows Couldn\'t unmute. Try again. and keeps the label', async () => {
  const m = load({ muted: true, route: () => ({ status: 500 }) });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.els['person-action-error'].textContent, 'Couldn\'t unmute. Try again.');
  assert.equal(m.els['person-mute-item'].textContent, 'Unmute');
});

test('a network failure shows the same line for the action', async () => {
  const m = load({ muted: false, route: () => Promise.reject(new Error('down')) });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.els['person-action-error'].textContent, 'Couldn\'t mute. Try again.');
});

test('a 404 reloads the page so the server shows not found', async () => {
  const m = load({ muted: false, route: () => ({ status: 404 }) });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.reloads.length, 1);
  assert.equal(m.els['person-action-error'].hidden, true, 'no line while the page redraws');
});

test('offline sends nothing and says the built line', async () => {
  const m = load({ muted: false, route: flagRoute(true), online: false });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.calls.length, 0);
  assert.equal(m.els['person-action-error'].textContent, 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.');
});

test('the person is never named in a request body and no confirm box is used', async () => {
  const m = load({ muted: false, route: flagRoute(true) });
  m.click('person-mute-item');
  await flush();
  assert.equal(m.calls.every((c) => c.address.startsWith('/api/people/asha_k')), true);
  assert.equal(m.els['person-confirm'].open, false);
});

test('no markup sink is used, and the template draws Mute first with the server flag', async () => {
  const m = load({ muted: false, route: flagRoute(true) });
  m.click('person-mute-item');
  await flush();
  m.click('person-mute-item');
  await flush();
  assert.deepEqual(m.sinks, [], 'nothing was assigned to a markup sink');
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write|eval\(/.test(source), false);
  assert.equal(source.split('is muted. Their reports still show.').length - 1, 1, 'the muted sentence is written once');

  const menu = template.slice(template.indexOf('id="person-more-menu"'));
  const mutePos = menu.indexOf('id="person-mute-item"');
  const removePos = menu.indexOf('id="person-remove-follower"');
  assert.ok(mutePos > 0 && mutePos < removePos, 'Mute is the first item of the menu');
  assert.ok(template.includes('data-muted="{{if .Profile.Muted}}true{{else}}false{{end}}">{{if .Profile.Muted}}Unmute{{else}}Mute{{end}}</button>'));
});
