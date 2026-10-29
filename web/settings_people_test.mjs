// settings_people_test.mjs - runs the real web/static/js/settings-people.js in a
// Node sandbox and checks the people rows of the Settings page (Phase 9 plan 12,
// D-04, D-12, UI-SPEC Surface 7):
//
//   - the Let others follow me switch posts once to /api/me/visibility, says the
//     polite status on success, and restores the old value on a refusal (422 or
//     429, with the server's sentence under the box), on any other failure and
//     offline (nothing is sent);
//   - turning it off while anyone follows opens the confirm box and sends
//     nothing; Cancel and Escape leave the switch on; Turn off saves, and a
//     success sets Followers to 0; with no followers it turns off at once;
//   - the three list rows call PinalertPeopleList.open with the right kind,
//     own true and the person's username, and after a change the counts are
//     read again;
//   - every string reaches the page through textContent.
//
// Run it with: node --test web/settings_people_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'settings-people.js'), 'utf8');

class FakeEl {
  constructor(doc, tag, id) {
    this.doc = doc;
    this.tagName = tag.toUpperCase();
    this.id = id || '';
    this.own = '';
    this.attrs = {};
    this.handlers = {};
    this.disabled = false;
    this.checked = false;
  }
  get textContent() { return this.own; }
  set textContent(v) { this.own = String(v); }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  get open() { return Object.hasOwn(this.attrs, 'open'); }
  get hidden() { return Object.hasOwn(this.attrs, 'hidden'); }
  set hidden(v) { if (v) { this.attrs.hidden = ''; } else { delete this.attrs.hidden; } }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  focus() { this.doc.activeElement = this; }
}

// The ids settings.html.tmpl renders for the people rows.
function buildPage({ followers = 0, following = 2, blocked = 1, on = false, username = 'asha_k', dialog = true } = {}) {
  const doc = { activeElement: null, byId: {} };
  const make = (tag, id) => {
    const el = new FakeEl(doc, tag, id);
    doc.byId[id] = el;
    return el;
  };
  doc.getElementById = (id) => doc.byId[id] || null;
  make('div', 'notice-live');
  const wrap = make('div', 'settings-people-wrap');
  wrap.setAttribute('data-username', username);
  make('input', 'settings-profile-switch').checked = on;
  make('button', 'settings-following-row');
  make('button', 'settings-followers-row');
  make('button', 'settings-blocked-row');
  make('span', 'settings-following-value').textContent = String(following);
  make('span', 'settings-followers-value').textContent = String(followers);
  make('span', 'settings-blocked-value').textContent = String(blocked);
  make('p', 'settings-people-error').hidden = true;
  const box = make('dialog', 'settings-profile-off-confirm');
  make('h2', 'settings-profile-off-title').textContent = 'Turn off your public profile?';
  make('button', 'settings-profile-off-cancel').textContent = 'Cancel';
  make('button', 'settings-profile-off-go').textContent = 'Turn off';
  if (dialog) {
    box.showModal = function showModal() { box.setAttribute('open', ''); };
    box.close = function close() {
      if (!box.open) { return; }
      delete box.attrs.open;
      (box.handlers.close || []).forEach((h) => h({ type: 'close', target: box }));
    };
  }
  return { doc, box };
}

function load(options = {}) {
  const page = buildPage(options);
  const requests = [];
  const opens = [];
  const sandbox = {
    console,
    document: page.doc,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setTimeout() { return 1; },
    clearTimeout() {},
    AbortController: class { constructor() { this.signal = { aborted: false }; } abort() { this.signal.aborted = true; } },
    navigator: { onLine: options.onLine !== false },
    PinalertPeopleList: options.noList ? undefined : { open(o) { opens.push(o); } }
  };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'settings-people.js' });
  return { ...page, requests, opens, api: sandbox.PinalertSettingsPeople };
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
// flip does what a tap on the switch does: the browser toggles it, then change.
const flip = (page) => {
  const sw = el(page, 'settings-profile-switch');
  sw.checked = !sw.checked;
  fire(sw, 'change');
};

test('the script leaves quietly on a page without the people rows', () => {
  const doc = { byId: {}, getElementById() { return null; } };
  const sandbox = { console, document: doc };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'settings-people.js' });
  assert.equal(sandbox.PinalertSettingsPeople, undefined);
});

test('turning the switch on posts once and says the profile is on', async () => {
  const page = load({ on: false });
  flip(page);
  await flush();
  assert.equal(page.requests.length, 1);
  assert.match(page.requests[0].url, /\/api\/me\/visibility$/);
  assert.equal(page.requests[0].init.method, 'PATCH');
  assert.equal(page.requests[0].init.credentials, 'same-origin');
  assert.equal(page.requests[0].init.body, '{"profile_public":true}');
  assert.equal(el(page, 'settings-profile-switch').disabled, true);

  answer(page.requests[0], 200, { profile_public: true });
  await flush();
  assert.equal(el(page, 'settings-profile-switch').checked, true);
  assert.equal(el(page, 'settings-profile-switch').disabled, false);
  assert.equal(el(page, 'notice-live').textContent,
    'Your profile is on. People can follow you while you hold the Reliable reporter tag.');
  assert.equal(el(page, 'settings-people-error').hidden, true);
});

test('a refusal puts the switch back and shows the server sentence', async () => {
  const page = load({ on: false });
  flip(page);
  await flush();
  const sentence = 'Your name can\'t be shown on a public profile. Change it in About you.';
  answer(page.requests[0], 422, { error: { field: 'profile_public', message: sentence } });
  await flush();
  assert.equal(page.requests.length, 1);
  assert.equal(el(page, 'settings-profile-switch').checked, false);
  assert.equal(el(page, 'settings-profile-switch').disabled, false);
  assert.equal(el(page, 'settings-people-error').hidden, false);
  assert.equal(el(page, 'settings-people-error').textContent, sentence);
  assert.equal(el(page, 'notice-live').textContent, '');
});

test('any other failure restores the old value and says Couldn\'t save that', async () => {
  const page = load({ on: false });
  flip(page);
  await flush();
  answer(page.requests[0], 500, { error: { message: 'internal detail' } });
  await flush();
  assert.equal(el(page, 'settings-profile-switch').checked, false);
  assert.equal(el(page, 'settings-people-error').textContent, 'Couldn\'t save that. Try again.');

  // A dropped connection is the same line, and the next try starts clean.
  flip(page);
  await flush();
  assert.equal(page.requests.length, 2);
  assert.equal(el(page, 'settings-people-error').hidden, true);
  page.requests[1].reject(new Error('network'));
  await flush();
  assert.equal(el(page, 'settings-profile-switch').checked, false);
  assert.equal(el(page, 'settings-people-error').textContent, 'Couldn\'t save that. Try again.');
});

test('offline sends nothing and shows the built offline line', async () => {
  const page = load({ on: false, onLine: false });
  flip(page);
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(el(page, 'settings-profile-switch').checked, false);
  assert.equal(el(page, 'settings-people-error').textContent,
    'You\'re offline, so this wasn\'t saved. Try again when you have a signal.');
});

test('turning it off with no followers saves at once, with no box', async () => {
  const page = load({ on: true, followers: 0 });
  flip(page);
  await flush();
  assert.equal(page.box.open, false);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.body, '{"profile_public":false}');
  answer(page.requests[0], 200, { profile_public: false });
  await flush();
  assert.equal(el(page, 'settings-profile-switch').checked, false);
  assert.equal(el(page, 'notice-live').textContent, 'Your profile is off.');
});

test('turning it off while someone follows asks first and sends nothing', async () => {
  const page = load({ on: true, followers: 3 });
  flip(page);
  await flush();
  assert.equal(page.box.open, true);
  assert.equal(page.requests.length, 0);
  assert.equal(el(page, 'settings-profile-switch').checked, true);
  assert.equal(el(page, 'settings-profile-off-title').textContent, 'Turn off your public profile?');
  assert.equal(page.doc.activeElement, el(page, 'settings-profile-off-title'));
});

test('Cancel closes the box and leaves the switch on', async () => {
  const page = load({ on: true, followers: 3 });
  flip(page);
  fire(el(page, 'settings-profile-off-cancel'), 'click');
  await flush();
  assert.equal(page.box.open, false);
  assert.equal(page.requests.length, 0);
  assert.equal(el(page, 'settings-profile-switch').checked, true);
  assert.equal(page.doc.activeElement, el(page, 'settings-profile-switch'));
});

test('Escape in the box is Cancel', async () => {
  const page = load({ on: true, followers: 3 });
  flip(page);
  const event = fire(page.box, 'cancel', { cancelable: true });
  await flush();
  assert.equal(event.prevented, true);
  assert.equal(page.box.open, false);
  assert.equal(page.requests.length, 0);
  assert.equal(el(page, 'settings-profile-switch').checked, true);
});

test('Turn off saves, and a success sets Followers to 0', async () => {
  const page = load({ on: true, followers: 3 });
  flip(page);
  fire(el(page, 'settings-profile-off-go'), 'click');
  await flush();
  assert.equal(page.box.open, false);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].init.body, '{"profile_public":false}');
  answer(page.requests[0], 200, { profile_public: false });
  await flush();
  assert.equal(el(page, 'settings-profile-switch').checked, false);
  assert.equal(el(page, 'settings-followers-value').textContent, '0');
  assert.equal(el(page, 'notice-live').textContent, 'Your profile is off.');
});

test('a failed Turn off puts the switch back on and shows the line', async () => {
  const page = load({ on: true, followers: 3 });
  flip(page);
  fire(el(page, 'settings-profile-off-go'), 'click');
  await flush();
  answer(page.requests[0], 500, {});
  await flush();
  assert.equal(el(page, 'settings-profile-switch').checked, true);
  assert.equal(el(page, 'settings-followers-value').textContent, '3');
  assert.equal(el(page, 'settings-people-error').textContent, 'Couldn\'t save that. Try again.');
});

test('the three rows open the list window with the right kind', async () => {
  const page = load({ username: 'asha_k' });
  fire(el(page, 'settings-following-row'), 'click');
  fire(el(page, 'settings-followers-row'), 'click');
  fire(el(page, 'settings-blocked-row'), 'click');
  assert.equal(page.opens.length, 3);
  assert.deepEqual(page.opens.map((o) => o.kind), ['following', 'followers', 'blocked']);
  assert.deepEqual(page.opens.map((o) => o.own), [true, true, true]);
  assert.deepEqual(page.opens.map((o) => o.username), ['asha_k', 'asha_k', '']);
  assert.equal(page.opens[0].opener, el(page, 'settings-following-row'));
  assert.equal(page.opens[2].opener, el(page, 'settings-blocked-row'));
  assert.equal(typeof page.opens[0].onChange, 'function');
});

test('after a list changed, the counts are read again from the server', async () => {
  const page = load({ followers: 3, following: 2, blocked: 2, username: 'asha_k' });
  fire(el(page, 'settings-followers-row'), 'click');
  page.opens[0].onChange();
  await flush();
  assert.equal(page.requests.length, 1);
  assert.match(page.requests[0].url, /\/api\/people\/asha_k$/);
  answer(page.requests[0], 200, { followers_count: 0, following_count: 2 });
  await flush();
  assert.equal(el(page, 'settings-followers-value').textContent, '0');
  assert.equal(el(page, 'settings-following-value').textContent, '2');

  fire(el(page, 'settings-blocked-row'), 'click');
  page.opens[1].onChange();
  await flush();
  assert.match(page.requests[1].url, /\/api\/me\/blocked$/);
  answer(page.requests[1], 200, { entries: [{ username: 'x_y' }] });
  await flush();
  assert.equal(el(page, 'settings-blocked-value').textContent, '1');
});

test('a person with no username opens only the Blocked people list', () => {
  const page = load({ username: '' });
  fire(el(page, 'settings-following-row'), 'click');
  fire(el(page, 'settings-followers-row'), 'click');
  assert.equal(page.opens.length, 0);
  fire(el(page, 'settings-blocked-row'), 'click');
  assert.equal(page.opens.length, 1);
  assert.equal(page.opens[0].kind, 'blocked');
});

test('the script uses no markup sink', () => {
  assert.ok(!/innerHTML|outerHTML|insertAdjacentHTML|document\.write/.test(source));
});
