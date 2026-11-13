// person_test.mjs - runs the real web/static/js/person.js in a Node sandbox and
// checks the actions of the profile page (Phase 9 plan 08, UI-SPEC 1d, 1e, 1h):
// Follow and Following redraw from the refetched profile, the More menu keys,
// Remove follower and Block ask first in the system style box with the exact
// words, a refusal shows the server's sentence, and no markup sink is used.
//
// Run it with: node --test web/person_test.mjs
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

const flush = () => new Promise((resolve) => setImmediate(resolve));

const PROFILE_FOLLOWING = {
  followers_count: 13, followers_word: 'Followers', followers_text: '13 Followers',
  following_count: 3, following_word: 'Following', following_text: '3 Following',
  can_follow: false, following: true, follows_you: false, blocked_by_you: false
};
const PROFILE_NOT_FOLLOWING = {
  followers_count: 12, followers_word: 'Followers', followers_text: '12 Followers',
  following_count: 3, following_word: 'Following', following_text: '3 Following',
  can_follow: true, following: false, follows_you: false, blocked_by_you: false
};

// load builds the page of another person's profile and runs person.js on it.
// route(method, url) answers each call with {status, body} or a promise of one.
// stars is the shared star control script (plan 09-16). It is run in the same
// sandbox as person.js when a test asks for it, so the two scripts meet the way
// they do on the page.
const starsSource = fs.readFileSync(path.join(here, 'static', 'js', 'stars.js'), 'utf8');

// rating: null draws no rating box (the server said the reader may not rate);
// {yours: n} draws the box with data-your-rating, as the server does when it
// says may_rate. withStars: false leaves stars.js out of the sandbox.
function load({ route, online = true, dialogSupport = true, followsYou = true, rating = null, withStars = true, starsGroup = false } = {}) {
  const sinks = [];
  const state = { active: null };
  const calls = [];

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
      appendChild(node) { node.parentNode = this; this.children.push(node); return node; },
      insertBefore(node, before) {
        node.parentNode = this;
        const at = this.children.indexOf(before);
        this.children.splice(at < 0 ? this.children.length : at, 0, node);
        return node;
      },
      removeChild(node) { this.children = this.children.filter((c) => c !== node); node.parentNode = null; return node; },
      contains(node) { return node === this || this.children.includes(node) || (this.kids || []).includes(node); },
      // 2026-10-30 (09-REVIEW WR-B03): the stars and tag redraw looks nodes up
      // by class and id inside the header body.
      querySelector(selector) {
        const cls = selector.slice(1);
        const walk = (n) => {
          for (const c of n.children || []) {
            if (String(c.className || '').split(' ').includes(cls)) { return c; }
            const d = walk(c);
            if (d) { return d; }
          }
          return null;
        };
        return walk(this);
      },
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

  const els = {};
  function add(id, extra) { els[id] = fakeEl(id, extra); return els[id]; }

  const page = add('person-page', { attrs: { 'data-username': 'asha_k', 'data-self': 'false' } });
  add('notice-live');
  const header = add('person-header');
  add('person-reports-section');
  // Amended 2026-11-13 by quick 261113-k9t (owner decision 2026-11-13, the
  // centred profile): the band wrapper is gone, so the action error line and the
  // rating box are direct children of the header again, the box right before
  // the error line.
  const actionError = add('person-action-error', { hidden: true });
  header.children.push(actionError);
  actionError.parentNode = header;
  if (rating) {
    const box = add('person-rating', { attrs: { 'data-your-rating': String(rating.yours) } });
    header.insertBefore(box, actionError);
  }
  // The counts group (the stars group, Followers, Following) is #person-stats, as
  // the page draws it since quick 261113-t4y (owner decision 2026-11-13, the
  // compact profile). A group built by the script is found by id through it, so a
  // removed one is gone. The reporter tag is on its own line under it.
  const body = add('person-stats');
  const followersCell = fakeEl('followers-cell');
  const followingCell = fakeEl('following-cell');
  body.appendChild(followersCell);
  body.appendChild(followingCell);
  const tagGroup = fakeEl('tag-line');
  tagGroup.appendChild(add('person-tag', { textContent: 'Reliable reporter' }));
  tagGroup.appendChild(add('person-tag-sentence', { textContent: 'old sentence' }));
  if (starsGroup) {
    const group = fakeEl('stars-group', { className: 'person-stars' });
    group.id = 'person-stars';
    group.setAttribute('aria-label', '4.0 stars');
    const total = fakeEl('total', { className: 'person-stars__total' });
    total.appendChild(fakeEl('number', { className: 'person-stars__number', textContent: '4.0' }));
    group.appendChild(total);
    body.insertBefore(group, followersCell);
  }
  const findById = (node, id) => {
    for (const c of node.children || []) {
      if (c.id === id) { return c; }
      const d = findById(c, id);
      if (d) { return d; }
    }
    return null;
  };
  add('person-follow', { textContent: 'Follow' });
  add('person-unfollow', { textContent: 'Following', hidden: true });
  const more = add('person-more');
  const removeFollower = add('person-remove-follower', { attrs: { role: 'menuitem' }, hidden: !followsYou });
  const block = add('person-block', { attrs: { role: 'menuitem' } });
  add('person-more-menu', { hidden: true, kids: [removeFollower, block] });
  more.kids = [];
  add('person-blocked', { hidden: true });
  add('person-blocked-title', { tabindex: '-1' });
  add('person-blocked-error', { hidden: true });
  add('person-unblock');
  add('person-followers-button');
  add('person-following-button');
  add('person-followers-count', { textContent: '12' });
  add('person-followers-word', { textContent: 'Followers' });
  add('person-following-count', { textContent: '3' });
  add('person-following-word', { textContent: 'Following' });
  const confirm = add('person-confirm', { open: false });
  if (dialogSupport) {
    confirm.showModal = function () { this.open = true; };
    confirm.close = function () { if (this.open) { this.open = false; this.fire('close'); } };
  }
  add('person-confirm-title');
  add('person-confirm-text');
  add('person-confirm-error', { hidden: true });
  add('person-confirm-cancel');
  add('person-confirm-go');
  page.attrs['data-username'] = 'asha_k';

  const docListeners = [];
  const asked = [];
  const reloads = [];
  const document = {
    readyState: 'complete',
    getElementById: (id) => els[id] || findById(body, id),
    createElement: (tag) => fakeEl(tag),
    createTextNode: (text) => ({ nodeType: 3, textContent: text }),
    addEventListener(type, fn, capture) { docListeners.push({ type, fn, capture }); },
    removeEventListener(type, fn) {
      const at = docListeners.findIndex((l) => l.type === type && l.fn === fn);
      if (at >= 0) { docListeners.splice(at, 1); }
    },
    write(...args) { sinks.push('document.write ' + args.join(' ')); },
    get activeElement() { return state.active; }
  };
  const window = {
    setTimeout: () => 1,
    clearTimeout() {},
    confirm(q) { asked.push(q); return false; },
    location: { reload() { reloads.push(1); } }
  };
  const fetch = (address, init = {}) => {
    const method = init.method || 'GET';
    calls.push({ method, address, init });
    return Promise.resolve(route(method, address)).then((answer) => ({
      status: answer.status,
      ok: answer.status >= 200 && answer.status < 300,
      json: () => (answer.body === undefined ? Promise.reject(new Error('no body')) : Promise.resolve(answer.body))
    }));
  };
  const navigator = { onLine: online };
  const sandbox = vm.createContext({ document, window, fetch, navigator, Promise, JSON, encodeURIComponent, isFinite, Math, Number, parseInt });
  if (withStars) {
    vm.runInContext(starsSource, sandbox, { filename: 'stars.js' });
  }
  vm.runInContext(source, sandbox, { filename: 'person.js' });
  window.PinalertPerson.init();

  const click = (id) => els[id].fire('click', { target: els[id] });
  const key = (id, k) => {
    const e = { key: k, prevented: false, preventDefault() { this.prevented = true; } };
    els[id].fire('keydown', e);
    return e;
  };
  const outsideClick = () => {
    docListeners.filter((l) => l.type === 'click').forEach((l) => l.fn({ target: {} }));
  };
  return { els, calls, sinks, asked, reloads, docListeners, click, key, outsideClick, api: window.PinalertPerson };
}

const FOLLOW_ROUTE = (profile) => (method, address) => {
  if (method === 'POST') { return { status: 204 }; }
  return { status: 200, body: profile };
};

test('Follow sends one POST and redraws from the refetched profile', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  m.click('person-follow');
  await flush();
  assert.deepEqual(m.calls.map((c) => c.method + ' ' + c.address), [
    'POST /api/people/asha_k/follow',
    'GET /api/people/asha_k'
  ]);
  assert.equal(m.els['person-followers-count'].textContent, '13', 'the count is the server\'s');
  assert.equal(m.els['person-followers-button'].attrs['aria-label'], '13 Followers');
  assert.equal(m.els['person-follow'].hidden, true);
  assert.equal(m.els['person-unfollow'].hidden, false);
  assert.equal(m.els['notice-live'].textContent, 'You followed asha_k.');
  assert.equal(m.els['person-unfollow'].focused, 1, 'focus lands on the button now in its place');
  assert.equal(m.els['person-follow'].disabled, false);
  assert.equal(m.els['person-follow'].getAttribute('aria-busy'), null);
});

// Quick 261113-t4y (owner decision 2026-11-13, the compact profile): a username
// saved from the edit window on your own page changes the name every call is sent
// under, and profile-edit.js tells this script through one document event.
test('a username changed from the edit window changes the address the calls go to', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  const listener = m.docListeners.find((l) => l.type === 'pinalert:username-changed');
  assert.ok(listener, 'person.js listens for the username change');
  listener.fn({ detail: { username: 'asha.new' } });
  m.click('person-follow');
  await flush();
  assert.deepEqual(m.calls.map((c) => c.method + ' ' + c.address), [
    'POST /api/people/asha.new/follow',
    'GET /api/people/asha.new'
  ]);
  // An event with no name changes nothing.
  listener.fn({ detail: {} });
  m.click('person-unfollow');
  await flush();
  assert.equal(m.calls[2].address, '/api/people/asha.new/unfollow');
});

test('the button is disabled with aria-busy while the call is out and its label stays', async () => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const m = load({
    route: (method) => (method === 'POST' ? gate.then(() => ({ status: 204 })) : { status: 200, body: PROFILE_FOLLOWING })
  });
  m.click('person-follow');
  await flush();
  assert.equal(m.els['person-follow'].disabled, true);
  assert.equal(m.els['person-follow'].getAttribute('aria-busy'), 'true');
  assert.equal(m.els['person-follow'].textContent, 'Follow', 'no spinner and no new label');
  m.click('person-follow');
  await flush();
  assert.equal(m.calls.filter((c) => c.method === 'POST').length, 1, 'a second tap sends nothing');
  release();
  await flush();
  assert.equal(m.els['person-follow'].disabled, false);
});

test('Following unfollows at once with no box, and focus goes to Follow', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_NOT_FOLLOWING) });
  m.els['person-follow'].hidden = true;
  m.els['person-unfollow'].hidden = false;
  m.click('person-unfollow');
  await flush();
  assert.equal(m.calls[0].address, '/api/people/asha_k/unfollow');
  assert.equal(m.els['person-confirm'].open, false, 'no confirmation');
  assert.equal(m.els['notice-live'].textContent, 'You unfollowed asha_k.');
  assert.equal(m.els['person-follow'].hidden, false);
  assert.equal(m.els['person-follow'].focused, 1);
});

test('the More menu keys: open, arrows, Escape, outside click', () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  m.click('person-more');
  assert.equal(m.els['person-more-menu'].hidden, false);
  assert.equal(m.els['person-more'].attrs['aria-expanded'], 'true');
  assert.equal(m.els['person-remove-follower'].focused, 1, 'the first item takes focus');
  const down = m.key('person-more-menu', 'ArrowDown');
  assert.equal(down.prevented, true);
  assert.equal(m.els['person-block'].focused, 1);
  m.key('person-more-menu', 'ArrowDown');
  assert.equal(m.els['person-remove-follower'].focused, 2, 'the arrows wrap');
  m.key('person-more-menu', 'ArrowUp');
  assert.equal(m.els['person-block'].focused, 2);
  m.key('person-more-menu', 'Escape');
  assert.equal(m.els['person-more-menu'].hidden, true);
  assert.equal(m.els['person-more'].attrs['aria-expanded'], 'false');
  assert.equal(m.els['person-more'].focused, 1, 'Escape returns focus to More');

  m.click('person-more');
  assert.equal(m.docListeners.filter((l) => l.type === 'click' && l.capture === true).length, 1);
  m.outsideClick();
  assert.equal(m.els['person-more-menu'].hidden, true, 'an outside click closes it');
  assert.equal(m.docListeners.filter((l) => l.type === 'click').length, 0, 'and the listener is removed');
});

test('a hidden menu item is skipped by the arrows', () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING), followsYou: false });
  m.click('person-more');
  assert.equal(m.els['person-block'].focused, 1, 'Block is the only item');
  m.key('person-more-menu', 'ArrowDown');
  assert.equal(m.els['person-block'].focused, 2);
});

test('Remove follower asks first with the exact words; Cancel sends nothing', () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  m.click('person-more');
  m.key('person-more-menu', 'Enter');
  assert.equal(m.els['person-confirm'].open, true);
  assert.equal(m.els['person-confirm-title'].textContent, 'Remove asha_k as a follower?');
  assert.deepEqual(m.els['person-confirm-text'].children.map((p) => p.textContent),
    ['They stop following you. They can follow you again.']);
  assert.equal(m.els['person-confirm-go'].textContent, 'Remove');
  assert.equal(m.els['person-confirm-title'].focused, 1, 'the question takes focus');
  assert.equal(m.els['person-confirm-cancel'].focused, 0, 'no button looks chosen');
  assert.equal(m.els['person-confirm-go'].focused, 0);
  m.click('person-confirm-cancel');
  assert.equal(m.els['person-confirm'].open, false);
  assert.equal(m.calls.length, 0, 'Cancel sends nothing');
  assert.equal(m.els['person-more'].focused > 0, true, 'focus goes back to More');
});

test('confirming Remove follower sends one POST, says so and drops the item', async () => {
  const m = load({ route: FOLLOW_ROUTE({ ...PROFILE_FOLLOWING, follows_you: false }) });
  m.click('person-remove-follower');
  assert.equal(m.els['person-confirm'].open, true);
  m.click('person-confirm-go');
  await flush();
  assert.equal(m.calls[0].method + ' ' + m.calls[0].address, 'POST /api/people/asha_k/remove-follower');
  assert.equal(m.calls.filter((c) => c.method === 'POST').length, 1);
  assert.equal(m.els['person-confirm'].open, false);
  assert.equal(m.els['notice-live'].textContent, 'asha_k no longer follows you.');
  assert.equal(m.els['person-remove-follower'].hidden, true);
});

test('Block asks with its two texts, then the blocked box gets focus', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  m.click('person-block');
  assert.equal(m.els['person-confirm-title'].textContent, 'Block asha_k?');
  assert.deepEqual(m.els['person-confirm-text'].children.map((p) => p.textContent), [
    'They can no longer see your profile or follow you, and you stop following each other.',
    'Their reports still show in the feed. You will not see their comments.'
  ]);
  assert.equal(m.els['person-confirm-go'].textContent, 'Block');
  m.click('person-confirm-go');
  await flush();
  assert.equal(m.calls.length, 1);
  assert.equal(m.calls[0].address, '/api/people/asha_k/block');
  assert.equal(m.els['person-header'].hidden, true);
  assert.equal(m.els['person-reports-section'].hidden, true);
  assert.equal(m.els['person-blocked'].hidden, false);
  assert.equal(m.els['person-blocked-title'].textContent, 'You blocked asha_k');
  assert.equal(m.els['person-blocked-title'].focused, 1, 'focus lands on the heading');
  assert.equal(m.els['person-more'].focused, 0, 'focus does not go back to a hidden More');
});

// 2026-10-30 (09-REVIEW WR-B05): a 409 draws the profile again and the server's
// can_follow decides whether Follow stays. This pin used to answer every call
// with a 409 and expect Follow hidden at once.
const refuseFollow = (message, profile) => (method) => (method === 'POST'
  ? { status: 409, body: { error: { field: 'follow', message } } }
  : { status: 200, body: profile });

test('a 409 shows the server sentence and removes Follow when the person cannot be followed', async () => {
  const m = load({
    route: refuseFollow('You can\'t follow this person right now.', { ...PROFILE_NOT_FOLLOWING, can_follow: false })
  });
  m.click('person-follow');
  await flush();
  assert.equal(m.els['person-action-error'].hidden, false);
  assert.equal(m.els['person-action-error'].textContent, 'You can\'t follow this person right now.');
  assert.equal(m.els['person-follow'].hidden, true);
  assert.equal(m.els['person-follow'].disabled, false);
  assert.equal(m.els['person-more'].focused, 1, 'focus goes to More, not to the page');
});

test('a 409 at the follow cap keeps Follow and its focus', async () => {
  const m = load({
    route: refuseFollow('You can follow up to 200 people.', PROFILE_NOT_FOLLOWING)
  });
  m.click('person-follow');
  await flush();
  assert.equal(m.els['person-action-error'].textContent, 'You can follow up to 200 people.');
  assert.equal(m.els['person-follow'].hidden, false, 'Follow stays for the next try');
  assert.equal(m.els['person-follow'].focused, 1, 'focus stays on the control used');
  assert.equal(m.els['person-follow'].disabled, false);
});

test('a 404 on an action reloads the page so the server shows not found', async () => {
  const m = load({ route: () => ({ status: 404, body: { error: { field: 'username', message: 'Not found' } } }) });
  m.click('person-follow');
  await flush();
  assert.equal(m.reloads.length, 1);
  assert.equal(m.els['person-action-error'].hidden, true, 'no line on a page that is being redrawn');
});

test('other failures show the sentence for that action; 429 shows the server own', async () => {
  const wrong = load({ route: () => ({ status: 500, body: {} }) });
  wrong.click('person-follow');
  await flush();
  assert.equal(wrong.els['person-action-error'].textContent, 'Couldn\'t follow. Try again.');

  const slow = load({ route: () => ({ status: 429, body: { error: { field: 'rate_people_action', message: 'You\'re doing that very quickly. You can try again in 3 seconds.' } } }) });
  slow.click('person-follow');
  await flush();
  assert.equal(slow.els['person-action-error'].textContent, 'You\'re doing that very quickly. You can try again in 3 seconds.');

  const blockFail = load({ route: () => ({ status: 500, body: {} }) });
  blockFail.click('person-block');
  blockFail.click('person-confirm-go');
  await flush();
  assert.equal(blockFail.els['person-confirm-error'].textContent, 'Couldn\'t block. Try again.');
  assert.equal(blockFail.els['person-confirm-error'].hidden, false);
  assert.equal(blockFail.els['person-confirm'].open, true, 'the box stays open with the sentence inside');
  assert.equal(blockFail.els['person-confirm-go'].disabled, false);
});

test('offline sends nothing and says the built line', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING), online: false });
  m.click('person-follow');
  await flush();
  assert.equal(m.calls.length, 0);
  assert.equal(m.els['person-action-error'].textContent, 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.');
});

test('Unblock sends one POST, says so and reloads', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  m.click('person-unblock');
  await flush();
  assert.equal(m.calls[0].method + ' ' + m.calls[0].address, 'POST /api/people/asha_k/unblock');
  assert.equal(m.els['notice-live'].textContent, 'asha_k is unblocked.');
  assert.equal(m.reloads.length, 1);
});

test('Escape on the box is refused while an action is out', async () => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const m = load({ route: (method) => (method === 'POST' ? gate.then(() => ({ status: 204 })) : { status: 200, body: PROFILE_FOLLOWING }) });
  m.click('person-block');
  m.click('person-confirm-go');
  await flush();
  const e = { prevented: false, preventDefault() { this.prevented = true; } };
  m.els['person-confirm'].fire('cancel', e);
  assert.equal(e.prevented, true);
  assert.equal(m.els['person-confirm-go'].disabled, true);
  release();
  await flush();
});

test('without dialog support the browser confirm asks', () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING), dialogSupport: false });
  m.click('person-block');
  assert.deepEqual(m.asked, ['Block asha_k?']);
  assert.equal(m.calls.length, 0, 'No sends nothing');
});

test('no markup sink is ever used, and the script text has none', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING) });
  m.click('person-follow');
  await flush();
  m.click('person-block');
  m.click('person-confirm-go');
  await flush();
  m.click('person-unblock');
  await flush();
  assert.deepEqual(m.sinks, [], 'nothing was assigned to a markup sink');
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write|eval\(/.test(source), false);
  assert.ok((source.match(/textContent/g) || []).length >= 5);
});

// ---- plan 09-16: the Your rating control (UI-SPEC 3a) ------------------------

const RATE_ROUTE = (profile, rate = () => ({ status: 204 })) => (method, address) => {
  if (method === 'POST') { return rate(address); }
  return { status: 200, body: profile };
};

// ratingParts reads the five star buttons and Remove my rating out of the box.
function ratingParts(m) {
  const box = m.els['person-rating'];
  const fieldset = box.children[0];
  if (!fieldset) { return { box, fieldset: null, stars: [], remove: null }; }
  const row = fieldset.children[1];
  return { box, fieldset, stars: row.children.slice(0, 5), remove: row.children[5] };
}

test('the star control mounts when the server drew the rating box, with the server\'s rating', () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING), rating: { yours: 4 } });
  const parts = ratingParts(m);
  assert.equal(parts.box.children.length, 1);
  assert.equal(parts.fieldset.id, 'fieldset');
  assert.equal(parts.fieldset.children[0].textContent, 'Your rating');
  assert.deepEqual(parts.stars.map((s) => s.getAttribute('aria-label')), ['1 star', '2 stars', '3 stars', '4 stars', '5 stars']);
  assert.deepEqual(parts.stars.map((s) => s.getAttribute('aria-pressed')), ['false', 'false', 'false', 'true', 'false']);
  assert.equal(parts.remove.hidden, false);
  assert.equal(m.calls.length, 0, 'drawing the control sends nothing');
});

test('no rating box from the server means no control and nothing explaining why', () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING) });
  assert.equal(m.els['person-rating'], undefined);
  // Amended 2026-11-13 by quick 261113-k9t (owner decision 2026-11-13, the
  // centred profile): the header holds the action error line directly again, so
  // its children are counted.
  assert.equal(m.els['person-header'].children.length, 1, 'only the action error line is in the header');
  assert.equal(m.els['person-action-error'].hidden, true);
});

test('without stars.js on the page the empty box stays empty and nothing throws', () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING), rating: { yours: 0 }, withStars: false });
  assert.equal(m.els['person-rating'].children.length, 0);
});

test('a tap saves, says so in the live line and clears the error line', async () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING), rating: { yours: 0 } });
  m.els['person-action-error'].textContent = 'old';
  m.els['person-action-error'].hidden = false;
  const parts = ratingParts(m);
  parts.stars[4].fire('click', { target: parts.stars[4] });
  await flush();
  // 2026-10-30 (09-REVIEW WR-B03): a saved rating asks for the profile again.
  assert.deepEqual(m.calls.map((c) => c.method + ' ' + c.address), ['POST /api/people/asha_k/rating', 'GET /api/people/asha_k']);
  assert.equal(m.calls[0].init.body, '{"stars":5}');
  assert.equal(m.els['notice-live'].textContent, 'You rated asha_k 5 stars.');
  assert.equal(m.els['person-action-error'].hidden, true);
  assert.deepEqual(parts.stars.map((s) => s.getAttribute('aria-pressed')), ['false', 'false', 'false', 'false', 'true']);
});

test('a refusal shows the server sentence under the box and leaves the rating', async () => {
  const sentence = 'You can change this rating again tomorrow.';
  const m = load({
    route: RATE_ROUTE(PROFILE_FOLLOWING, () => ({ status: 409, body: { error: { field: 'stars', message: sentence } } })),
    rating: { yours: 2 }
  });
  const parts = ratingParts(m);
  parts.stars[0].fire('click', {});
  await flush();
  assert.equal(m.els['person-action-error'].textContent, sentence);
  assert.equal(m.els['person-action-error'].hidden, false);
  assert.deepEqual(parts.stars.map((s) => s.getAttribute('aria-pressed')), ['false', 'true', 'false', 'false', 'false']);
});

test('Remove my rating says Your rating was removed. in the live line', async () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING), rating: { yours: 3 } });
  const parts = ratingParts(m);
  parts.remove.fire('click', {});
  await flush();
  assert.equal(m.calls[0].address, '/api/people/asha_k/rating/clear');
  assert.equal(m.els['notice-live'].textContent, 'Your rating was removed.');
  assert.equal(parts.remove.hidden, true);
});

test('a refetch with may_rate false removes the box with no message', async () => {
  const m = load({ route: FOLLOW_ROUTE({ ...PROFILE_FOLLOWING, may_rate: false, your_rating: 0 }), rating: { yours: 3 } });
  m.click('person-follow');
  await flush();
  const box = m.els['person-rating'];
  assert.equal(box.children.length, 0, 'the control is taken out');
  assert.equal(box.hidden, true);
  assert.equal(m.els['notice-live'].textContent, 'You followed asha_k.', 'no line about the rating');
  assert.equal(m.els['person-action-error'].hidden, true);
});

test('a refetch with may_rate true redraws the control with the server\'s your_rating', async () => {
  const m = load({ route: FOLLOW_ROUTE({ ...PROFILE_FOLLOWING, may_rate: true, your_rating: 2 }), rating: { yours: 3 } });
  const parts = ratingParts(m);
  m.click('person-follow');
  await flush();
  assert.equal(m.els['person-rating'].children.length, 1, 'still one control, not two');
  assert.deepEqual(parts.stars.map((s) => s.getAttribute('aria-pressed')), ['false', 'true', 'false', 'false', 'false']);
});

test('a refetch that turns may_rate true on a page with no box builds the box above the error line', async () => {
  const m = load({ route: FOLLOW_ROUTE({ ...PROFILE_FOLLOWING, may_rate: true, your_rating: 0 }) });
  m.click('person-follow');
  await flush();
  // Amended 2026-11-13 by quick 261113-k9t (owner decision 2026-11-13, the
  // centred profile): the band wrapper is gone, so the box is built in the
  // header, right before the action error line.
  const head = m.els['person-header'];
  assert.equal(head.children.length, 2);
  const host = head.children[0];
  assert.equal(host.id, 'person-rating');
  assert.equal(head.children[1], m.els['person-action-error']);
  assert.equal(host.children[0].id, 'fieldset');
});

test('a refetch that says nothing about may_rate leaves the control alone', async () => {
  const m = load({ route: FOLLOW_ROUTE(PROFILE_FOLLOWING), rating: { yours: 3 } });
  m.click('person-follow');
  await flush();
  assert.equal(m.els['person-rating'].children.length, 1);
});

test('a 404 on a rating redraws the page from the server', async () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING, () => ({ status: 404, body: {} })), rating: { yours: 0 } });
  const parts = ratingParts(m);
  parts.stars[2].fire('click', {});
  await flush();
  assert.equal(m.reloads.length, 1);
});

test('the rating control adds no markup sink and uses only the rating addresses', async () => {
  const m = load({ route: RATE_ROUTE(PROFILE_FOLLOWING), rating: { yours: 1 } });
  const parts = ratingParts(m);
  parts.stars[3].fire('click', {});
  await flush();
  parts.remove.fire('click', {});
  await flush();
  assert.deepEqual(m.sinks, []);
  for (const call of m.calls) {
    // 2026-10-30 (09-REVIEW WR-B03): the profile read after a rating is allowed.
    assert.match(call.address, /^\/api\/people\/asha_k(\/rating(\/clear)?)?$/);
  }
});

// ---- 2026-10-30 (09-REVIEW WR-B03): a rating redraws the stars, the tag and the follow

const AFTER_RATING = {
  followers_count: 11, followers_word: 'Followers', followers_text: '11 Followers',
  following_count: 3, following_word: 'Following', following_text: '3 Following',
  can_follow: true, following: false, follows_you: false, blocked_by_you: false, notify: false,
  stars_number: '2.8', stars_label: '2.8 stars',
  stars_from_reports: '3.6 from reports', stars_from_ratings: '1.0 from ratings by 6 people',
  tag_word: 'Mixed record', tag_sentence: '4 of their last 9 reports were confirmed.'
};

test('a saved rating redraws the stars, the tag and the follow from the server', async () => {
  const m = load({ route: RATE_ROUTE(AFTER_RATING), rating: { yours: 0 }, starsGroup: true });
  m.els['person-unfollow'].hidden = false;
  m.els['person-follow'].hidden = true;
  const parts = ratingParts(m);
  parts.stars[0].fire('click', {});
  await flush();
  const group = m.els['person-stats'].children.find((c) => c.id === 'person-stars');
  assert.ok(group, 'the stars group is still drawn');
  assert.equal(group.getAttribute('aria-label'), '2.8 stars');
  assert.equal(group.querySelector('.person-stars__number').textContent, '2.8');
  assert.equal(m.els['person-tag'].textContent, 'Mixed record');
  assert.equal(m.els['person-tag-sentence'].textContent, '4 of their last 9 reports were confirmed.');
  // Amended 2026-11-09 (quick 261109-n25, fewer words): the server still sends
  // the two parts, and the page draws neither of them.
  const names = group.children.map((c) => c.id);
  assert.deepEqual(names.filter((n) => String(n).startsWith('person-')), []);
  assert.equal(m.els['person-unfollow'].hidden, true, 'the ended follow is taken away');
  assert.equal(m.els['person-follow'].hidden, false);
  assert.deepEqual(m.sinks, []);
});

test('the stars group goes when the server sends no number and is built when one arrives', async () => {
  const none = { ...AFTER_RATING, stars_number: '', stars_label: '', stars_from_reports: '', stars_from_ratings: '' };
  const m = load({ route: RATE_ROUTE(none), rating: { yours: 0 }, starsGroup: true });
  const parts = ratingParts(m);
  parts.stars[0].fire('click', {});
  await flush();
  const wrap = m.els['person-stats'];
  assert.equal(wrap.children.some((c) => c.id === 'person-stars'), false);

  const back = load({ route: RATE_ROUTE(AFTER_RATING), rating: { yours: 0 } });
  const p2 = ratingParts(back);
  p2.stars[0].fire('click', {});
  await flush();
  const wrap2 = back.els['person-stats'];
  const built = wrap2.children.find((c) => c.id === 'person-stars');
  assert.ok(built, 'built from the server strings');
  // Amended 2026-11-13 (quick 261113-k9t, the centred profile): the stars total is
  // the first cell of the counts row, in front of Followers and Following, and not
  // just in front of the reporter cell.
  assert.equal(wrap2.children.indexOf(built), 0, 'the stars group is the first cell of the counts row');
  // Amended 2026-11-13 (quick 261113-t4y): the reporter tag is not a cell of the
  // counts group, so it holds three cells.
  assert.equal(wrap2.children.length, 3, 'stars, Followers, Following');
  assert.equal(built.querySelector('.person-stars__word').textContent, 'Stars');
  assert.equal(built.getAttribute('role'), 'group');
  assert.equal(built.getAttribute('aria-label'), '2.8 stars');
  assert.equal(built.querySelector('.person-stars__number').textContent, '2.8');
  assert.deepEqual(back.sinks, []);
});