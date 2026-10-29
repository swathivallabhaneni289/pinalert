// person_bell_test.mjs - runs the real web/static/js/person.js in a Node sandbox
// and checks the Notify me bell of the profile (Phase 9 plan 24, D-08, D-10,
// UI-SPEC 1d and 1e): it is hidden until you follow and while you have muted
// the person, one tap posts {"on": bool} once, the pressed state comes from the
// server's answer, the exact status sentences are said, a refusal shows the
// server's sentence and leaves the state, Unfollow takes the bell away and
// puts focus on Follow, and no markup sink is used.
//
// Run it with: node --test web/person_bell_test.mjs
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

const PROFILE = ({ following = false, muted = false, notify = false } = {}) => ({
  followers_count: 12, followers_word: 'Followers', followers_text: '12 Followers',
  following_count: 3, following_word: 'Following', following_text: '3 Following',
  can_follow: !following, following, follows_you: false, blocked_by_you: false, muted, notify
});

// load builds another person's profile page with Follow, Following, the bell
// and the More menu, and runs person.js on it. start is what the server drew.
// route(method, address, body) answers each call with {status, body}.
function load({ following = false, muted = false, notify = false, route, online = true } = {}) {
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
  add('person-follow', { textContent: 'Follow', hidden: following });
  add('person-unfollow', { textContent: 'Following', hidden: !following });
  add('person-bell', {
    attrs: { 'aria-pressed': notify ? 'true' : 'false' },
    hidden: !following || muted
  });
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
  confirm.showModal = function () { this.open = true; };
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
    calls.push({ method, address, body: init.body === undefined ? null : init.body, headers: init.headers || {} });
    return Promise.resolve(route(method, address, init.body)).then((answer) => {
      if (answer.fail) {
        return Promise.reject(new Error('network down'));
      }
      return {
        status: answer.status,
        ok: answer.status >= 200 && answer.status < 300,
        json: () => (answer.body === undefined ? Promise.reject(new Error('no body')) : Promise.resolve(answer.body))
      };
    });
  };
  const navigator = { onLine: online };
  const sandbox = vm.createContext({ document, window, fetch, navigator, Promise, JSON, encodeURIComponent, isFinite, Math, Number, parseInt });
  vm.runInContext(source, sandbox, { filename: 'person.js' });
  window.PinalertPerson.init();

  const click = (id) => els[id].fire('click', { target: els[id] });
  return { els, calls, sinks, reloads, click, bell: els['person-bell'] };
}

// A route that keeps the server's state the way the real one does: follow
// starts the bell off, notify saves what it is told, mute turns it off.
function statefulRoute({ following = false, muted = false, notify = false, notifyAnswer } = {}) {
  const s = { following, muted, notify };
  return (method, address, body) => {
    if (method === 'GET') {
      return { status: 200, body: PROFILE(s) };
    }
    if (address.endsWith('/follow')) { s.following = true; s.notify = false; return { status: 204 }; }
    if (address.endsWith('/unfollow')) { s.following = false; s.notify = false; return { status: 204 }; }
    if (address.endsWith('/unmute')) { s.muted = false; return { status: 204 }; }
    if (address.endsWith('/mute')) { s.muted = true; s.notify = false; return { status: 204 }; }
    if (address.endsWith('/notify')) {
      if (notifyAnswer) { return notifyAnswer(JSON.parse(body)); }
      s.notify = JSON.parse(body).on;
      return { status: 200, body: { notify: s.notify } };
    }
    return { status: 404, body: {} };
  };
}

test('the bell is absent before you follow and present, off, after Follow', async () => {
  const m = load({ route: statefulRoute() });
  assert.equal(m.bell.hidden, true, 'hidden before following');
  m.click('person-follow');
  await flush();
  assert.equal(m.bell.hidden, false, 'shown after Follow');
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false', 'off by default');
  assert.equal(m.els['person-unfollow'].focused, 1, 'focus stays on the control used');
});

test('one tap posts {"on":true} once, sets pressed and says the exact sentence', async () => {
  const m = load({ following: true, route: statefulRoute({ following: true }) });
  m.click('person-bell');
  await flush();
  const posts = m.calls.filter((c) => c.method === 'POST');
  assert.equal(posts.length, 1);
  assert.equal(posts[0].address, '/api/people/asha_k/notify');
  assert.deepEqual(JSON.parse(posts[0].body), { on: true });
  assert.equal(posts[0].headers['Content-Type'], 'application/json');
  assert.equal(m.bell.getAttribute('aria-pressed'), 'true');
  assert.equal(m.els['notice-live'].textContent, 'You will be notified when asha_k posts.');
  assert.equal(m.els['person-action-error'].hidden, true);
  assert.equal(m.bell.disabled, false);
  assert.equal(m.bell.getAttribute('aria-busy'), null);
  assert.equal(m.bell.focused, 1, 'focus stays on the bell');
});

test('a second tap turns it off and says the other sentence', async () => {
  const m = load({ following: true, route: statefulRoute({ following: true }) });
  m.click('person-bell');
  await flush();
  m.click('person-bell');
  await flush();
  const posts = m.calls.filter((c) => c.method === 'POST');
  assert.equal(posts.length, 2);
  assert.deepEqual(JSON.parse(posts[1].body), { on: false });
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false');
  assert.equal(m.els['notice-live'].textContent, 'You will not be notified when asha_k posts.');
});

test('the pressed state follows the server answer, not the tap', async () => {
  const m = load({
    following: true,
    route: statefulRoute({ following: true, notifyAnswer: () => ({ status: 200, body: { notify: false } }) })
  });
  m.click('person-bell');
  await flush();
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false');
  assert.equal(m.els['notice-live'].textContent, 'You will not be notified when asha_k posts.');
});

test('the bell is disabled and busy while the call is out, and a second tap sends nothing', async () => {
  let release = null;
  const m = load({
    following: true,
    route: (method) => (method === 'POST'
      ? new Promise((resolve) => { release = () => resolve({ status: 200, body: { notify: true } }); })
      : { status: 200, body: PROFILE({ following: true }) })
  });
  m.click('person-bell');
  await flush();
  assert.equal(m.bell.disabled, true);
  assert.equal(m.bell.getAttribute('aria-busy'), 'true');
  m.click('person-bell');
  await flush();
  assert.equal(m.calls.filter((c) => c.method === 'POST').length, 1);
  release();
  await flush();
  assert.equal(m.bell.disabled, false);
  assert.equal(m.bell.getAttribute('aria-pressed'), 'true');
});

test('a 409 shows the server sentence and leaves the state', async () => {
  const cap = 'You can turn on Notify me for up to 50 people.';
  const m = load({
    following: true,
    route: statefulRoute({
      following: true,
      notifyAnswer: () => ({ status: 409, body: { error: { field: 'notify', message: cap } } })
    })
  });
  m.click('person-bell');
  await flush();
  assert.equal(m.els['person-action-error'].textContent, cap);
  assert.equal(m.els['person-action-error'].hidden, false);
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false');
  assert.equal(m.els['notice-live'].textContent, '');
  assert.equal(m.bell.disabled, false);
  assert.equal(m.bell.focused, 1);
});

test('the unavailable 409 shows its server sentence', async () => {
  const line = 'Couldn\'t change this. Try again.';
  const m = load({
    following: true,
    route: statefulRoute({
      following: true,
      notifyAnswer: () => ({ status: 409, body: { error: { field: 'notify', message: line } } })
    })
  });
  m.click('person-bell');
  await flush();
  assert.equal(m.els['person-action-error'].textContent, line);
});

// 2026-10-30 (09-REVIEW WR-B03): a 409 draws the profile again, so a bell for a
// follow that has ended is taken away and focus goes to More.
test('a 409 on a follow that has ended takes the bell away', async () => {
  const line = 'Couldn\'t change this. Try again.';
  const m = load({
    following: true,
    route: statefulRoute({
      following: false,
      notifyAnswer: () => ({ status: 409, body: { error: { field: 'notify', message: line } } })
    })
  });
  m.click('person-bell');
  await flush();
  assert.deepEqual(m.calls.map((c) => c.method), ['POST', 'GET']);
  assert.equal(m.bell.hidden, true, 'the bell is gone');
  assert.equal(m.els['person-follow'].hidden, false);
  assert.equal(m.els['person-action-error'].textContent, line);
  assert.equal(m.els['person-more'].focused, 1, 'focus is not left on a hidden bell');
});

test('another failure, a network failure and offline show the exact lines', async () => {
  const a = load({
    following: true,
    route: statefulRoute({ following: true, notifyAnswer: () => ({ status: 500, body: {} }) })
  });
  a.click('person-bell');
  await flush();
  assert.equal(a.els['person-action-error'].textContent, 'Couldn\'t change this. Try again.');
  assert.equal(a.bell.getAttribute('aria-pressed'), 'false');

  const b = load({
    following: true,
    route: statefulRoute({ following: true, notifyAnswer: () => ({ fail: true }) })
  });
  b.click('person-bell');
  await flush();
  assert.equal(b.els['person-action-error'].textContent, 'Couldn\'t change this. Try again.');

  const c = load({ following: true, online: false, route: statefulRoute({ following: true }) });
  c.click('person-bell');
  await flush();
  assert.equal(c.calls.length, 0, 'nothing is sent offline');
  assert.equal(c.els['person-action-error'].textContent, 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.');
});

test('a 404 reloads so the server draws Not found', async () => {
  const m = load({
    following: true,
    route: statefulRoute({ following: true, notifyAnswer: () => ({ status: 404, body: {} }) })
  });
  m.click('person-bell');
  await flush();
  assert.equal(m.reloads.length, 1);
  assert.equal(m.els['person-action-error'].hidden, true);
});

test('Mute removes the bell and Unmute brings it back off', async () => {
  const m = load({ following: true, notify: true, route: statefulRoute({ following: true, notify: true }) });
  assert.equal(m.bell.hidden, false);
  assert.equal(m.bell.getAttribute('aria-pressed'), 'true');
  m.click('person-mute-item');
  await flush();
  assert.equal(m.bell.hidden, true, 'hidden while muted');
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false');
  m.click('person-mute-item');
  await flush();
  assert.equal(m.bell.hidden, false, 'back after Unmute');
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false', 'back off');
});

test('Unfollow removes the bell and focus lands on Follow', async () => {
  const m = load({ following: true, notify: true, route: statefulRoute({ following: true, notify: true }) });
  m.click('person-unfollow');
  await flush();
  assert.equal(m.bell.hidden, true);
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false');
  assert.equal(m.els['person-follow'].hidden, false);
  assert.equal(m.els['person-follow'].focused, 1);
  // Following again starts off.
  m.click('person-follow');
  await flush();
  assert.equal(m.bell.hidden, false);
  assert.equal(m.bell.getAttribute('aria-pressed'), 'false');
});

test('a hidden bell sends nothing when it is tapped', async () => {
  const m = load({ route: statefulRoute() });
  m.click('person-bell');
  await flush();
  assert.equal(m.calls.length, 0);
});

test('the bell is the button after Following and before More, named and pressed', () => {
  const at = (needle) => {
    const i = template.indexOf(needle);
    assert.ok(i >= 0, 'template lacks ' + needle);
    return i;
  };
  assert.ok(at('id="person-unfollow"') < at('id="person-bell"'));
  assert.ok(at('id="person-bell"') < at('id="person-more"'));
  assert.match(template, /id="person-bell"[^>]*aria-pressed=/);
  assert.match(template, /Notify me when \{\{\.Profile\.Username\}\} posts/);
});

test('no markup sink, and no wording promises a sound or a vibration', async () => {
  const m = load({ following: true, route: statefulRoute({ following: true }) });
  m.click('person-bell');
  await flush();
  m.click('person-bell');
  await flush();
  assert.deepEqual(m.sinks, []);
  assert.doesNotMatch(source, /\.innerHTML|\.outerHTML|insertAdjacentHTML|document\.write/);
  const words = source.match(/'You will (not )?be notified[^']*'/g) || [];
  assert.equal(words.length, 2);
  for (const w of words) {
    assert.doesNotMatch(w, /buzz|vibrat|sound|ring|beep/i);
  }
});
