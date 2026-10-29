// people_list_test.mjs - runs the real web/static/js/people-list.js in a Node
// sandbox and checks the shared list window of Followers, Following and Blocked
// people (Phase 9 plan 11, D-03, UI-SPEC Surface 4): named rows are links to the
// profile, the others line is plain and never focusable, Show more sends the
// cursor and appends, Unblock posts once and moves focus, Remove all followers
// asks first, Escape closes and returns focus, and no markup sink is used.
//
// Run it with: node --test web/people_list_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what people-list.js calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'people-list.js'), 'utf8');

const flush = () => new Promise((resolve) => setImmediate(resolve));

// load builds the window and its confirm box as fake nodes and runs the script.
// route(method, address) answers each call with {status, body}.
function load({ route }) {
  const sinks = [];
  const state = { active: null };
  const calls = [];
  const els = {};

  function fakeEl(tag, id) {
    const el = {
      tag,
      id: id || '',
      hidden: false,
      className: '',
      attrs: {},
      children: [],
      parentNode: null,
      listeners: [],
      textContent: '',
      disabled: false,
      type: '',
      focused: 0,
      inert: false,
      get firstChild() { return this.children[0] || null; },
      get nextElementSibling() {
        if (!this.parentNode) { return null; }
        const at = this.parentNode.children.indexOf(this);
        return this.parentNode.children[at + 1] || null;
      },
      addEventListener(type, fn) { this.listeners.push({ type, fn }); },
      setAttribute(name, value) { this.attrs[name] = String(value); },
      getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
      removeAttribute(name) { delete this.attrs[name]; },
      appendChild(node) { node.parentNode = this; this.children.push(node); return node; },
      removeChild(node) { this.children = this.children.filter((c) => c !== node); node.parentNode = null; return node; },
      descendants() {
        const out = [];
        for (const c of this.children) { out.push(c, ...c.descendants()); }
        return out;
      },
      contains(node) { return node === this || this.descendants().includes(node); },
      querySelectorAll(selector) {
        const tags = selector.split(',').map((s) => s.trim().replace(/\[.*\]$/, ''));
        return this.descendants().filter((d) => tags.includes(d.tag));
      },
      querySelector(selector) { return this.querySelectorAll(selector)[0] || null; },
      focus() { this.focused += 1; state.active = this; },
      fire(type, e = {}) { this.listeners.filter((l) => l.type === type).forEach((l) => l.fn(e)); }
    };
    for (const sink of ['innerHTML', 'outerHTML']) {
      Object.defineProperty(el, sink, { set(v) { sinks.push(sink + ' = ' + v); }, get() { return ''; } });
    }
    el.insertAdjacentHTML = (...args) => { sinks.push('insertAdjacentHTML ' + args.join(' ')); };
    return el;
  }

  function add(tag, id, extra = {}) {
    const el = fakeEl(tag, id);
    Object.assign(el, extra);
    els[id] = el;
    return el;
  }

  const win = add('div', 'people-list-window', { hidden: true });
  add('h2', 'people-list-title');
  add('p', 'people-list-subtitle', { hidden: true });
  add('button', 'people-list-close');
  add('p', 'people-list-action-error', { hidden: true });
  add('div', 'people-list-loading', { hidden: true });
  add('ul', 'people-list-rows', { hidden: true });
  add('p', 'people-list-others', { hidden: true });
  add('button', 'people-list-more', { hidden: true });
  add('button', 'people-list-remove-all', { hidden: true });
  add('div', 'people-list-empty', { hidden: true });
  add('h3', 'people-list-empty-title');
  add('p', 'people-list-empty-line');
  add('div', 'people-list-error', { hidden: true });
  add('button', 'people-list-retry');
  const confirm = add('dialog', 'people-list-confirm', { open: false });
  confirm.showModal = function () { this.open = true; this.shown = (this.shown || 0) + 1; };
  confirm.close = function () { if (this.open) { this.open = false; this.fire('close'); } };
  add('h2', 'people-list-confirm-title');
  add('p', 'people-list-confirm-error', { hidden: true });
  add('button', 'people-list-confirm-cancel');
  add('button', 'people-list-confirm-go');
  const live = add('div', 'notice-live');
  // Everything the window and box are not part of, for the page lock.
  const pageBody = add('main', 'person-page');
  const body = fakeEl('body');
  body.children = [pageBody, live, win, confirm];
  // The window holds its own buttons so Tab handling has something to find.
  win.appendChild(els['people-list-close']);

  const document = {
    readyState: 'complete',
    body,
    documentElement: fakeEl('html'),
    getElementById: (id) => els[id] || null,
    createElement: (tag) => fakeEl(tag),
    addEventListener() {},
    get activeElement() { return state.active; }
  };
  const window = {
    setTimeout: () => 1,
    clearTimeout() {},
    confirm() { return false; }
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
  const navigator = { onLine: true };
  vm.runInNewContext(source, { document, window, fetch, navigator, Promise, encodeURIComponent });
  return { api: window.PinalertPeopleList, els, calls, sinks, state, confirm, pageBody };
}

const opener = () => {
  const o = { focused: 0, focus() { this.focused += 1; } };
  return o;
};

const LIST = {
  entries: [{ username: 'ravi_m' }, { username: 'a.b' }],
  others_count: 3,
  others_text: 'and 3 others',
  next: ''
};

function rowsOf(h) {
  return h.els['people-list-rows'].children;
}

test('named rows are links to the profile and the others line is a plain, unfocusable line', async () => {
  const h = load({ route: () => ({ status: 200, body: LIST }) });
  const o = opener();
  h.api.open({ kind: 'followers', username: 'asha_k', own: false, opener: o });
  await flush();
  const rows = rowsOf(h);
  assert.equal(rows.length, 2);
  const link = rows[0].children[0];
  assert.equal(link.tag, 'a');
  assert.equal(link.getAttribute('href'), '/u/ravi_m');
  assert.equal(rows[1].children[0].getAttribute('href'), '/u/a.b');
  assert.equal(link.children[0].textContent, 'R');
  assert.equal(link.children[1].textContent, 'ravi_m');
  assert.equal(h.calls[0].address, '/api/people/asha_k/followers');

  const others = h.els['people-list-others'];
  assert.equal(others.hidden, false);
  assert.equal(others.textContent, 'and 3 others');
  assert.equal(others.tag, 'p');
  assert.equal(others.getAttribute('href'), null);
  assert.equal(others.getAttribute('tabindex'), null);
  assert.equal(others.listeners.length, 0, 'the others line has no handler');
  assert.equal(h.els['people-list-title'].textContent, 'Followers');
  assert.equal(h.els['people-list-subtitle'].textContent, 'asha_k');
  assert.equal(h.els['people-list-subtitle'].hidden, false);
  assert.equal(h.els['people-list-remove-all'].hidden, true, 'Remove all followers is only on your own list');
});

test('a list of only others shows the line and no rows', async () => {
  const h = load({ route: () => ({ status: 200, body: { entries: [], others_count: 3, others_text: '3 others', next: '' } }) });
  h.api.open({ kind: 'following', username: 'asha_k', own: false, opener: opener() });
  await flush();
  assert.equal(h.els['people-list-rows'].hidden, true);
  assert.equal(h.els['people-list-others'].textContent, '3 others');
  assert.equal(h.els['people-list-empty'].hidden, true);
});

test('Show more sends the cursor and appends the next page', async () => {
  const pages = [
    { entries: [{ username: 'one_a' }], others_count: 0, others_text: '', next: 'cur sor/1' },
    { entries: [{ username: 'two_b' }], others_count: 2, others_text: 'and 2 others', next: '' }
  ];
  const h = load({ route: () => ({ status: 200, body: pages.shift() }) });
  h.api.open({ kind: 'followers', username: 'asha_k', own: false, opener: opener() });
  await flush();
  const more = h.els['people-list-more'];
  assert.equal(more.hidden, false);
  more.fire('click');
  await flush();
  assert.equal(h.calls[1].address, '/api/people/asha_k/followers?after=cur%20sor%2F1');
  assert.equal(rowsOf(h).length, 2);
  assert.equal(rowsOf(h)[1].children[0].getAttribute('href'), '/u/two_b');
  assert.equal(more.hidden, true, 'the last page has no Show more');
  assert.equal(h.els['people-list-others'].textContent, 'and 2 others');
});

test('empty states use the words of the kind and the owner', async () => {
  const empty = { entries: [], others_count: 0, others_text: '', next: '' };
  const cases = [
    { kind: 'followers', own: true, title: 'No followers yet', line: 'People who follow you will show here.' },
    { kind: 'followers', own: false, title: 'No followers yet', line: 'Nobody follows asha_k yet.' },
    { kind: 'following', own: true, title: 'You are not following anyone', line: 'People you follow show here.' },
    { kind: 'following', own: false, title: 'Not following anyone', line: 'asha_k does not follow anyone yet.' }
  ];
  for (const c of cases) {
    const h = load({ route: () => ({ status: 200, body: empty }) });
    h.api.open({ kind: c.kind, username: 'asha_k', own: c.own, opener: opener() });
    await flush();
    assert.equal(h.els['people-list-empty'].hidden, false);
    assert.equal(h.els['people-list-empty-title'].textContent, c.title);
    assert.equal(h.els['people-list-empty-line'].textContent, c.line);
  }
  const b = load({ route: () => ({ status: 200, body: { entries: [] } }) });
  b.api.open({ kind: 'blocked', opener: opener() });
  await flush();
  assert.equal(b.els['people-list-empty-title'].textContent, 'No blocked people');
  assert.equal(b.els['people-list-empty-line'].textContent, 'People you block show here, with a way to unblock them.');
  assert.equal(b.calls[0].address, '/api/me/blocked');
  assert.equal(b.els['people-list-title'].textContent, 'Blocked people');
  assert.equal(b.els['people-list-subtitle'].hidden, true);
});

test('a failed read shows the error state and Try again reads again', async () => {
  let fail = true;
  const h = load({ route: () => (fail ? { status: 500, body: {} } : { status: 200, body: LIST }) });
  h.api.open({ kind: 'followers', username: 'asha_k', own: false, opener: opener() });
  await flush();
  assert.equal(h.els['people-list-error'].hidden, false);
  fail = false;
  h.els['people-list-retry'].fire('click');
  await flush();
  assert.equal(h.calls.length, 2);
  assert.equal(h.els['people-list-error'].hidden, true);
  assert.equal(rowsOf(h).length, 2);
});

test('a blocked row has no link; Unblock posts once, removes the row and moves focus', async () => {
  const h = load({
    route: (method) => (method === 'GET'
      ? { status: 200, body: { entries: [{ username: 'first_x' }, { username: 'second_y' }] } }
      : { status: 204 })
  });
  let changed = 0;
  h.api.open({ kind: 'blocked', opener: opener(), onChange: () => { changed += 1; } });
  await flush();
  const rows = rowsOf(h);
  assert.equal(rows.length, 2);
  assert.equal(rows[0].querySelectorAll('a').length, 0, 'a blocked row holds no link');
  const button = rows[0].querySelector('button');
  assert.equal(button.textContent, 'Unblock');
  button.fire('click');
  button.fire('click');
  await flush();
  const posts = h.calls.filter((c) => c.method === 'POST');
  assert.equal(posts.length, 1, 'one tap sends one call');
  assert.equal(posts[0].address, '/api/people/first_x/unblock');
  assert.equal(rowsOf(h).length, 1);
  assert.equal(h.els['notice-live'].textContent, 'first_x is unblocked.');
  assert.equal(h.state.active, rowsOf(h)[0].querySelector('button'), 'focus moves to the next row button');
  assert.equal(changed, 1);

  rowsOf(h)[0].querySelector('button').fire('click');
  await flush();
  assert.equal(h.els['people-list-empty'].hidden, false);
  assert.equal(h.state.active, h.els['people-list-empty-title'], 'focus moves to the empty heading');
});

test('a refused Unblock keeps the row and shows the sentence', async () => {
  const h = load({
    route: (method) => (method === 'GET' ? { status: 200, body: { entries: [{ username: 'first_x' }] } } : { status: 500, body: {} })
  });
  h.api.open({ kind: 'blocked', opener: opener() });
  await flush();
  rowsOf(h)[0].querySelector('button').fire('click');
  await flush();
  assert.equal(rowsOf(h).length, 1);
  assert.equal(h.els['people-list-action-error'].hidden, false);
  assert.equal(h.els['people-list-action-error'].textContent, 'Couldn\'t unblock. Try again.');
});

test('Remove all followers asks first, Cancel sends nothing, confirm posts once and shows the empty state', async () => {
  const h = load({
    route: (method) => (method === 'GET' ? { status: 200, body: LIST } : { status: 204 })
  });
  let changed = 0;
  h.api.open({ kind: 'followers', username: 'asha_k', own: true, opener: opener(), onChange: () => { changed += 1; } });
  await flush();
  const removeAll = h.els['people-list-remove-all'];
  assert.equal(removeAll.hidden, false);
  assert.equal(h.els['people-list-subtitle'].hidden, true, 'no subtitle on your own list');

  removeAll.fire('click');
  assert.equal(h.confirm.open, true, 'the box opens first');
  assert.equal(h.calls.filter((c) => c.method === 'POST').length, 0, 'nothing is sent by opening the box');
  assert.equal(h.state.active, h.els['people-list-confirm-title']);

  h.els['people-list-confirm-cancel'].fire('click');
  await flush();
  assert.equal(h.confirm.open, false);
  assert.equal(h.calls.filter((c) => c.method === 'POST').length, 0, 'Cancel sends nothing');

  removeAll.fire('click');
  h.els['people-list-confirm-go'].fire('click');
  h.els['people-list-confirm-go'].fire('click');
  await flush();
  const posts = h.calls.filter((c) => c.method === 'POST');
  assert.equal(posts.length, 1);
  assert.equal(posts[0].address, '/api/me/followers/remove-all');
  assert.equal(h.confirm.open, false);
  assert.equal(h.els['people-list-empty'].hidden, false);
  assert.equal(h.els['people-list-empty-title'].textContent, 'No followers yet');
  assert.equal(h.els['notice-live'].textContent, 'No one follows you now.');
  assert.equal(removeAll.hidden, true);
  assert.equal(changed, 1);
});

test('Remove all followers shows for people who are only counted, and only on your own Followers list', async () => {
  const onlyOthers = { entries: [], others_count: 4, others_text: '4 others', next: '' };
  const own = load({ route: () => ({ status: 200, body: onlyOthers }) });
  own.api.open({ kind: 'followers', username: 'asha_k', own: true, opener: opener() });
  await flush();
  assert.equal(own.els['people-list-remove-all'].hidden, false);

  const following = load({ route: () => ({ status: 200, body: onlyOthers }) });
  following.api.open({ kind: 'following', username: 'asha_k', own: true, opener: opener() });
  await flush();
  assert.equal(following.els['people-list-remove-all'].hidden, true);
});

test('a refused Remove all keeps the box open with the sentence inside it', async () => {
  const h = load({ route: (method) => (method === 'GET' ? { status: 200, body: LIST } : { status: 500, body: {} }) });
  h.api.open({ kind: 'followers', username: 'asha_k', own: true, opener: opener() });
  await flush();
  h.els['people-list-remove-all'].fire('click');
  h.els['people-list-confirm-go'].fire('click');
  await flush();
  assert.equal(h.confirm.open, true);
  assert.equal(h.els['people-list-confirm-error'].hidden, false);
  assert.equal(h.els['people-list-confirm-error'].textContent, 'Couldn\'t remove followers. Try again.');
  assert.equal(rowsOf(h).length, 2, 'the list is untouched');
});

test('Escape closes the window and returns focus to the opener; the X does the same; the page is inert while open', async () => {
  const h = load({ route: () => ({ status: 200, body: LIST }) });
  const o = opener();
  h.api.open({ kind: 'followers', username: 'asha_k', own: false, opener: o });
  await flush();
  assert.equal(h.els['people-list-window'].hidden, false);
  assert.equal(h.pageBody.inert, true, 'the page behind is inert');
  assert.equal(h.els['notice-live'].inert, false, 'the status region stays readable');
  assert.equal(h.state.active, h.els['people-list-title'], 'focus starts on the title');

  let prevented = 0;
  h.els['people-list-window'].fire('keydown', { key: 'Escape', preventDefault() { prevented += 1; } });
  assert.equal(h.els['people-list-window'].hidden, true);
  assert.equal(o.focused, 1);
  assert.equal(h.pageBody.inert, false);

  h.api.open({ kind: 'followers', username: 'asha_k', own: false, opener: o });
  await flush();
  h.els['people-list-close'].fire('click');
  assert.equal(h.els['people-list-window'].hidden, true);
  assert.equal(o.focused, 2);
});

test('Escape with the confirm box open leaves the window to the box', async () => {
  const h = load({ route: () => ({ status: 200, body: LIST }) });
  h.api.open({ kind: 'followers', username: 'asha_k', own: true, opener: opener() });
  await flush();
  h.els['people-list-remove-all'].fire('click');
  assert.equal(h.confirm.open, true);
  h.els['people-list-window'].fire('keydown', { key: 'Escape', preventDefault() {} });
  assert.equal(h.els['people-list-window'].hidden, false, 'the window stays');
});

test('no markup sink is used', async () => {
  const h = load({ route: (method) => (method === 'GET' ? { status: 200, body: { entries: [{ username: '<img src=x>' }], others_count: 1, others_text: '1 other' } } : { status: 204 }) });
  h.api.open({ kind: 'followers', username: 'asha_k', own: true, opener: opener() });
  await flush();
  assert.equal(rowsOf(h)[0].children[0].getAttribute('href'), '/u/%3Cimg%20src%3Dx%3E');
  assert.deepEqual(h.sinks, []);
  assert.equal(/\.(innerHTML|outerHTML)\s*=|insertAdjacentHTML|document\.write/.test(source), false);
});

// ---- plan 09-20: the Muted people kind (UI-SPEC 4b, 4c, D-12) ----------------

test('the muted kind reads /api/me/muted-people and draws plain rows with an outlined Unmute', async () => {
  const h = load({
    route: (method) => (method === 'GET'
      ? { status: 200, body: { entries: [{ username: 'first_x' }, { username: 'second_y' }] } }
      : { status: 204 })
  });
  const o = opener();
  h.api.open({ kind: 'muted', opener: o });
  await flush();
  assert.equal(h.calls[0].address, '/api/me/muted-people');
  assert.equal(h.els['people-list-title'].textContent, 'Muted people');
  assert.equal(h.els['people-list-subtitle'].hidden, true, 'it is your own list, no subtitle');
  assert.equal(h.els['people-list-remove-all'].hidden, true);
  const rows = rowsOf(h);
  assert.equal(rows.length, 2);
  assert.equal(rows[0].querySelectorAll('a').length, 0, 'a muted row holds no link');
  assert.equal(rows[0].children[1].textContent, 'first_x', 'the username is shown');
  const button = rows[0].querySelector('button');
  assert.equal(button.textContent, 'Unmute');
  assert.equal(button.className, 'btn people-row__action', 'an outlined button, not a primary one');
});

test('Unmute posts once, removes the row, says the sentence and moves focus', async () => {
  const h = load({
    route: (method) => (method === 'GET'
      ? { status: 200, body: { entries: [{ username: 'first_x' }, { username: 'second_y' }] } }
      : { status: 204 })
  });
  let changed = 0;
  h.api.open({ kind: 'muted', opener: opener(), onChange: () => { changed += 1; } });
  await flush();
  const button = rowsOf(h)[0].querySelector('button');
  button.fire('click');
  button.fire('click');
  await flush();
  const posts = h.calls.filter((c) => c.method === 'POST');
  assert.equal(posts.length, 1, 'one tap sends one call');
  assert.equal(posts[0].address, '/api/people/first_x/unmute');
  assert.equal(rowsOf(h).length, 1);
  assert.equal(h.els['notice-live'].textContent, 'first_x is unmuted.');
  assert.equal(h.state.active, rowsOf(h)[0].querySelector('button'), 'focus moves to the next row button');
  assert.equal(changed, 1);

  rowsOf(h)[0].querySelector('button').fire('click');
  await flush();
  assert.equal(h.els['people-list-empty'].hidden, false);
  assert.equal(h.state.active, h.els['people-list-empty-title'], 'focus moves to the empty heading');
  assert.equal(h.els['people-list-empty-title'].textContent, 'No muted people');
  assert.equal(h.els['people-list-empty-line'].textContent, 'People you mute show here, with a way to unmute them.');
});

test('a refused Unmute keeps the row and shows Couldn\'t unmute. Try again. above the list', async () => {
  const h = load({
    route: (method) => (method === 'GET' ? { status: 200, body: { entries: [{ username: 'first_x' }] } } : { status: 500, body: {} })
  });
  h.api.open({ kind: 'muted', opener: opener() });
  await flush();
  rowsOf(h)[0].querySelector('button').fire('click');
  await flush();
  assert.equal(rowsOf(h).length, 1);
  assert.equal(h.els['people-list-action-error'].hidden, false);
  assert.equal(h.els['people-list-action-error'].textContent, 'Couldn\'t unmute. Try again.');
  assert.equal(h.els['notice-live'].textContent, '', 'no status line on a failure');
});

test('an empty muted list shows its words and the blocked list keeps Unblock', async () => {
  const m = load({ route: () => ({ status: 200, body: { entries: [] } }) });
  m.api.open({ kind: 'muted', opener: opener() });
  await flush();
  assert.equal(m.els['people-list-empty'].hidden, false);
  assert.equal(m.els['people-list-empty-title'].textContent, 'No muted people');

  const b = load({ route: () => ({ status: 200, body: { entries: [{ username: 'blk_z' }] } }) });
  b.api.open({ kind: 'blocked', opener: opener() });
  await flush();
  assert.equal(rowsOf(b)[0].querySelector('button').textContent, 'Unblock');
});
