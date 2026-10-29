// auth_flow_test.mjs - runs the real web/static/js/auth.js in a Node sandbox and
// checks the sign in card (Phase 7.1, plan 12, UI-SPEC Surfaces 1 to 5, sketch 010):
//
//   - the page opens on the panel the server names (data-start-panel); the two
//     choices switch panels in place with the hidden attribute, by click, arrows,
//     Home, End, Enter and Space, and a roving tabindex;
//   - every panel is a form the script submits itself: it posts JSON to the six
//     /api/auth routes, shows the busy words, writes the server's sentence into
//     the panel's own error line and marks the field aria-invalid;
//   - the code field keeps digits only, and a paste such as "482 913" is cleaned
//     before the browser's own cut at six characters (sketch 010 finding 1);
//   - five refused codes switch the code button off, the expiry notice and the
//     Resend wait come from the page's data attributes, a refused Resend shows
//     its sentence inside the code panel;
//   - Change email, Forgot password and Back to sign in keep the email;
//   - nothing ever reads the page address's query string, rewrites the address
//     bar, touches browser storage, or writes markup into the page.
//
// Run it with: node --test web/auth_flow_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The sandbox is this file's own (copied in shape from welcome_flow_test.mjs, which
// this file neither imports nor edits): a minimal element model, fake timers the
// test moves by hand, and a fetch the test answers by hand.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'auth.js'), 'utf8');

const OLD_ACCOUNT_LINE = 'Had an account before passwords? Choose Create account with the same email. Your reports stay with you.';
const ELLIPSIS = '\u2026';

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
    this.value = '';
    this.type = '';
    this.className = '';
    this.selects = 0;
  }
  get textContent() { return this.own + this.children.map((c) => c.textContent).join(''); }
  set textContent(v) {
    this.children.forEach((c) => { c.parentNode = null; });
    this.children = [];
    this.own = String(v);
  }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML must never be used: ' + v); }
  insertAdjacentHTML(where, v) { throw new Error('insertAdjacentHTML must never be used: ' + v); }
  get hidden() { return Object.hasOwn(this.attrs, 'hidden'); }
  set hidden(v) { if (v) { this.attrs.hidden = ''; } else { delete this.attrs.hidden; } }
  setAttribute(n, v) { this.attrs[n] = String(v); }
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
  focus() { this.doc.activeElement = this; }
  select() { this.selects += 1; this.doc.selected = this; }
}

// The markup of login_gate.html.tmpl, as the ids and attributes auth.js reads.
function buildPage({ start = 'create', mode = 'create', secondsLeft = 0, ttl = 600, resend = 45, email = '', done = false } = {}) {
  const doc = {
    activeElement: null,
    selected: null,
    byId: {},
    createElement(tag) { return new FakeEl(doc, tag); },
    getElementById(id) { return doc.byId[id] || null; },
    addEventListener() {},
    removeEventListener() {}
  };
  const make = (tag, id, parent, attrs) => {
    const el = new FakeEl(doc, tag, id);
    if (id) { doc.byId[id] = el; }
    if (attrs) { Object.entries(attrs).forEach(([k, v]) => el.setAttribute(k, v)); }
    if (parent) { parent.appendChild(el); }
    return el;
  };
  doc.body = make('body');
  const card = make('div', 'login-form-panel', doc.body, {
    'data-start-panel': start,
    'data-code-mode': mode,
    'data-code-seconds-left': String(secondsLeft),
    'data-code-ttl-seconds': String(ttl),
    'data-resend-seconds': String(resend),
    'data-password-min': '8'
  });
  if (done) {
    make('p', 'login-done', card).textContent = 'You are logged out of all devices.';
  }
  const title = make('h1', 'login-title', card, { tabindex: '-1' });
  title.textContent = start === 'code' ? (mode === 'reset' ? 'Reset your password' : 'Check your email') : 'Pinalert';
  const choice = make('div', 'login-choice', card, { role: 'tablist' });
  if (start === 'code') { choice.hidden = true; }
  const tabCreate = make('button', 'login-tab-create', choice, {
    role: 'tab', 'aria-selected': start === 'signin' ? 'false' : 'true', tabindex: start === 'signin' ? '-1' : '0'
  });
  const tabSignin = make('button', 'login-tab-signin', choice, {
    role: 'tab', 'aria-selected': start === 'signin' ? 'true' : 'false', tabindex: start === 'signin' ? '0' : '-1'
  });

  const panels = {};
  const section = (name, shown) => {
    const sec = make('section', 'auth-panel-' + name, card);
    if (!shown) { sec.hidden = true; }
    panels[name] = sec;
    make('form', 'auth-form-' + name, sec);
    return doc.byId['auth-form-' + name];
  };
  const formCreate = section('create', start === 'create');
  make('input', 'create-email', formCreate).type = 'email';
  make('input', 'create-password', formCreate, { 'aria-describedby': 'create-rule' }).type = 'password';
  make('button', 'create-show', formCreate, { 'data-show-for': 'create-password', 'aria-label': 'Show password' }).textContent = 'Show';
  make('p', 'create-error', formCreate, { role: 'alert', hidden: '' });
  make('button', 'create-submit', formCreate).textContent = 'Email me a code';

  const formSignin = section('signin', start === 'signin');
  make('p', 'signin-status', formSignin, { role: 'status', hidden: '' });
  make('input', 'signin-email', formSignin).type = 'email';
  make('input', 'signin-password', formSignin).type = 'password';
  make('button', 'signin-show', formSignin, { 'data-show-for': 'signin-password', 'aria-label': 'Show password' }).textContent = 'Show';
  make('p', 'signin-error', formSignin, { role: 'alert', hidden: '' });
  make('button', 'signin-submit', formSignin).textContent = 'Sign in';
  make('button', 'signin-forgot', formSignin).textContent = 'Forgot password';

  const formCode = section('code', start === 'code');
  make('span', 'code-email', formCode).textContent = email;
  make('input', 'code-input', formCode, { maxlength: '6' }).type = 'text';
  const wrap = make('div', 'code-new-wrap', formCode);
  if (mode !== 'reset') { wrap.hidden = true; }
  make('input', 'code-new-password', wrap, { 'aria-describedby': 'code-new-rule' }).type = 'password';
  make('button', 'code-new-show', wrap, { 'data-show-for': 'code-new-password', 'aria-label': 'Show password' }).textContent = 'Show';
  make('p', 'code-status', formCode, { role: 'status', hidden: '' });
  make('p', 'code-error', formCode, { role: 'alert', hidden: '' });
  make('button', 'code-submit', formCode).textContent = mode === 'reset' ? 'Set new password' : 'Create account';
  make('button', 'code-resend', formCode, { 'aria-disabled': 'true' }).textContent = 'Resend in ' + resend + 's';
  make('button', 'code-change-email', formCode).textContent = 'Change email';
  make('span', 'code-ready-live', formCode, { role: 'status' });

  const formForgot = section('forgot', false);
  make('input', 'forgot-email', formForgot).type = 'email';
  make('p', 'forgot-error', formForgot, { role: 'alert', hidden: '' });
  make('button', 'forgot-submit', formForgot).textContent = 'Email me a code';
  make('button', 'forgot-back', formForgot).textContent = 'Back to sign in';

  return { doc, card, panels, title, choice, tabCreate, tabSignin };
}

// load evaluates auth.js against a page. The sandbox has no URL, URLSearchParams or
// Event, so any use of them throws. Reads of the query string, any history call and
// any browser storage are recorded in `touched`; location.assign records its target.
function load(options) {
  const page = buildPage(options);
  const requests = [];
  const assigned = [];
  const touched = [];
  let now = 0;
  let nextId = 1;
  const timers = new Map();
  const advance = (ms) => {
    const end = now + ms;
    for (;;) {
      const due = [...timers.entries()].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
      if (!due) { break; }
      const [id, timer] = due;
      now = timer.at;
      if (timer.every) { timer.at += timer.every; } else { timers.delete(id); }
      timer.fn();
    }
    now = end;
  };
  const sandbox = {
    console,
    document: page.doc,
    fetch(target, init) {
      return new Promise((resolve, reject) => {
        requests.push({ url: String(target), init: init || {}, resolve, reject });
      });
    },
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimeout(id) { timers.delete(id); },
    setInterval(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms, every: ms }); return id; },
    clearInterval(id) { timers.delete(id); },
    addEventListener() {}
  };
  sandbox.location = {
    get search() { touched.push('location.search'); return ''; },
    get hash() { touched.push('location.hash'); return ''; },
    assign(target) { assigned.push(target); },
    replace(target) { touched.push('location.replace ' + target); },
    reload() { touched.push('location.reload'); }
  };
  ['localStorage', 'sessionStorage', 'history'].forEach((name) => {
    Object.defineProperty(sandbox, name, { get() { touched.push(name); return {}; } });
  });
  // Amended 2026-10-29 by the Phase 7.1 review fix (browser): options.globals adds
  // members to the sandbox (an AbortController, a Date), for the WR-B01 and WR-B02 cases.
  Object.assign(sandbox, (options && options.globals) || {});
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'auth.js' });
  return { ...page, requests, assigned, touched, advance, timers, sandbox };
}

const flush = async () => { for (let i = 0; i < 8; i++) { await new Promise((r) => setImmediate(r)); } };

const answer = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: () => Promise.resolve(body || {})
});
const refuse = (req) => req.reject(new TypeError('Failed to fetch'));

const byId = (page, id) => page.doc.byId[id];
const fire = (el, type, extra) => {
  const event = { type, target: el, key: '', shiftKey: false, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
  (el.handlers[type] || []).forEach((h) => h(event));
  return event;
};
const click = (page, id) => fire(byId(page, id), 'click');
const submit = (page, name) => fire(byId(page, 'auth-form-' + name), 'submit');
const text = (page, id) => byId(page, id).textContent;
const shownPanel = (page) => Object.keys(page.panels).filter((k) => !page.panels[k].hidden);
const body = (req) => JSON.parse(req.init.body);
const isFocused = (page, id) => page.doc.activeElement === byId(page, id);
const isOff = (page, id) => byId(page, id).getAttribute('aria-disabled') === 'true';

// assertClean is the no address rule: no query string read, no history call, no
// browser storage, and no navigation other than location.assign.
function assertClean(page) {
  assert.deepEqual(page.touched, [], 'auth.js must never read the query string, rewrite the address or touch browser storage');
}

async function fillCreate(page, emailText, password) {
  byId(page, 'create-email').value = emailText;
  byId(page, 'create-password').value = password;
}

// Walk Create account to the code screen the way a person does.
async function toCodeScreen(page, emailText = 'priya@example.com', password = 'pinalert-demo') {
  await fillCreate(page, emailText, password);
  submit(page, 'create');
  answer(page.requests[page.requests.length - 1], 200, { sent: true });
  await flush();
}

test('the page opens on the panel the server names, with the choice tabs in step', () => {
  const create = load({ start: 'create' });
  assert.deepEqual(shownPanel(create), ['create']);
  assert.equal(create.tabCreate.getAttribute('aria-selected'), 'true');
  assert.equal(create.tabCreate.getAttribute('tabindex'), '0');
  assert.equal(create.tabSignin.getAttribute('aria-selected'), 'false');
  assert.equal(create.tabSignin.getAttribute('tabindex'), '-1');
  assert.equal(create.requests.length, 0);

  const signin = load({ start: 'signin' });
  assert.deepEqual(shownPanel(signin), ['signin']);
  assert.equal(signin.tabSignin.getAttribute('aria-selected'), 'true');
  assertClean(create);
  assertClean(signin);
});

test('the two choices switch panels in place: click, arrows, Home, End, Enter and Space', () => {
  const page = load({ start: 'create' });
  click(page, 'login-tab-signin');
  assert.deepEqual(shownPanel(page), ['signin']);
  assert.equal(page.tabSignin.getAttribute('aria-selected'), 'true');
  assert.equal(page.tabSignin.getAttribute('tabindex'), '0');
  assert.equal(page.tabCreate.getAttribute('tabindex'), '-1');
  assert.ok(page.doc.activeElement === page.tabSignin, 'focus stays on the choice just pressed');
  assert.equal(text(page, 'login-title'), 'Pinalert');

  fire(page.tabSignin, 'keydown', { key: 'ArrowLeft' });
  assert.deepEqual(shownPanel(page), ['create']);
  assert.ok(page.doc.activeElement === page.tabCreate);

  const right = fire(page.tabCreate, 'keydown', { key: 'ArrowRight' });
  assert.equal(right.prevented, true);
  assert.deepEqual(shownPanel(page), ['signin']);
  assert.ok(page.doc.activeElement === page.tabSignin, 'Right arrow moves focus with the choice');

  fire(page.tabSignin, 'keydown', { key: 'Home' });
  assert.deepEqual(shownPanel(page), ['create']);
  fire(page.tabCreate, 'keydown', { key: 'End' });
  assert.deepEqual(shownPanel(page), ['signin']);

  fire(page.tabCreate, 'keydown', { key: 'Enter' });
  assert.deepEqual(shownPanel(page), ['create']);
  fire(page.tabSignin, 'keydown', { key: ' ' });
  assert.deepEqual(shownPanel(page), ['signin']);

  fire(page.tabSignin, 'keydown', { key: 'a' });
  assert.deepEqual(shownPanel(page), ['signin'], 'other keys do nothing');
  assert.equal(page.requests.length, 0, 'switching posts nothing');
  assertClean(page);
});

test('Create account posts JSON, shows Sending, then the code screen in create mode', async () => {
  const page = load({ start: 'create' });
  await fillCreate(page, 'priya@example.com', 'pinalert-demo');
  const event = submit(page, 'create');
  assert.equal(event.prevented, true, 'the script takes the form over');

  assert.equal(page.requests.length, 1);
  const req = page.requests[0];
  assert.equal(req.url, '/api/auth/sign-up');
  assert.equal(req.init.method, 'POST');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(body(req), { email: 'priya@example.com', password: 'pinalert-demo' });
  const button = byId(page, 'create-submit');
  assert.equal(button.textContent, 'Sending' + ELLIPSIS);
  assert.equal(button.getAttribute('aria-busy'), 'true');

  submit(page, 'create');
  assert.equal(page.requests.length, 1, 'a second press while waiting sends nothing');

  answer(req, 200, { sent: true });
  await flush();
  assert.equal(button.textContent, 'Email me a code');
  assert.equal(button.getAttribute('aria-busy'), null);

  assert.deepEqual(shownPanel(page), ['code']);
  assert.equal(page.choice.hidden, true, 'the two choices are hidden on the code screen');
  assert.equal(text(page, 'login-title'), 'Check your email');
  assert.ok(isFocused(page, 'login-title'), 'the title takes focus');
  assert.equal(text(page, 'code-email'), 'priya@example.com');
  assert.equal(byId(page, 'code-new-wrap').hidden, true);
  assert.equal(text(page, 'code-submit'), 'Create account');

  const resend = byId(page, 'code-resend');
  assert.equal(resend.textContent, 'Resend in 45s');
  assert.equal(resend.getAttribute('aria-disabled'), 'true');
  page.advance(1000);
  assert.equal(resend.textContent, 'Resend in 44s');
  page.advance(43000);
  assert.equal(resend.textContent, 'Resend in 1s');
  page.advance(1000);
  assert.equal(resend.textContent, 'Resend code');
  assert.equal(resend.getAttribute('aria-disabled'), null);
  assert.equal(text(page, 'code-ready-live'), 'Resend code', 'the wait announces once, when it is over');
  assert.equal(page.touched.length, 0);
  assertClean(page);
});

test('Create account refusals write the sentence into create-error and mark the field', async () => {
  const page = load({ start: 'create' });
  await fillCreate(page, 'priya@example.com', 'short');
  submit(page, 'create');
  answer(page.requests[0], 400, { error: { field: 'password', message: 'Use at least 8 characters.' } });
  await flush();

  const line = byId(page, 'create-error');
  assert.equal(line.hidden, false);
  assert.equal(line.textContent, 'Use at least 8 characters.');
  const pw = byId(page, 'create-password');
  assert.equal(pw.getAttribute('aria-invalid'), 'true');
  assert.ok(pw.getAttribute('aria-describedby').split(' ').includes('create-error'), 'the error is described');
  assert.ok(pw.getAttribute('aria-describedby').split(' ').includes('create-rule'), 'the rule line stays described');
  assert.equal(byId(page, 'create-email').getAttribute('aria-invalid'), null);
  assert.ok(isFocused(page, 'create-password'));
  assert.deepEqual(shownPanel(page), ['create']);
  assert.equal(byId(page, 'create-submit').textContent, 'Email me a code');

  // A second try clears the line and the mark first, then restores the rule description.
  submit(page, 'create');
  assert.equal(line.hidden, true);
  assert.equal(pw.getAttribute('aria-invalid'), null);
  assert.equal(pw.getAttribute('aria-describedby'), 'create-rule');
  answer(page.requests[1], 400, { error: { field: 'email', message: 'Enter a valid email address.' } });
  await flush();
  assert.equal(line.textContent, 'Enter a valid email address.');
  assert.equal(byId(page, 'create-email').getAttribute('aria-invalid'), 'true');
  assert.equal(pw.getAttribute('aria-invalid'), null);

  // 429 with no sentence, a 429 from the server, offline, and a server problem.
  submit(page, 'create');
  answer(page.requests[2], 429, {});
  await flush();
  assert.equal(line.textContent, 'Too many requests. Try again in a minute.');
  assert.equal(byId(page, 'create-email').getAttribute('aria-invalid'), null, 'a limit does not blame the email field');

  submit(page, 'create');
  answer(page.requests[3], 429, { error: { field: 'email', message: 'Too many codes for this email. Try again in an hour.' }, retry_after_seconds: 3600 });
  await flush();
  assert.equal(line.textContent, 'Too many codes for this email. Try again in an hour.');
  assert.equal(byId(page, 'create-email').getAttribute('aria-invalid'), null, 'field email on a limit is not a bad email');

  submit(page, 'create');
  refuse(page.requests[4]);
  await flush();
  assert.equal(line.textContent, "You're offline. Try again when you have a signal.");

  submit(page, 'create');
  answer(page.requests[5], 500, {});
  await flush();
  assert.equal(line.textContent, 'Something went wrong. Try again.');
  assert.equal(byId(page, 'create-submit').getAttribute('aria-busy'), null);
  assert.deepEqual(shownPanel(page), ['create']);
  assertClean(page);
});

test('the code field keeps digits only and a paste is cleaned before the browser cuts it', () => {
  const page = load({ start: 'code', email: 'priya@example.com', secondsLeft: 500 });
  const input = byId(page, 'code-input');

  input.value = '48a2-9 1b3';
  fire(input, 'input');
  assert.equal(input.value, '482913');

  input.value = '4829133';
  fire(input, 'input');
  assert.equal(input.value, '482913', 'never longer than the field allows');

  for (const pasted of ['482 913', '482-913', '482\u00a0913', ' 482 913 \n']) {
    input.value = '';
    const event = fire(input, 'paste', { clipboardData: { getData: () => pasted } });
    assert.equal(event.prevented, true, 'the paste default is stopped for ' + JSON.stringify(pasted));
    assert.equal(input.value, '482913', 'cleaned ' + JSON.stringify(pasted));
  }

  input.value = '12';
  const empty = fire(input, 'paste', { clipboardData: { getData: () => '' } });
  assert.equal(empty.prevented, false, 'an empty paste is left to the browser');
  assert.equal(input.value, '12');

  input.value = '';
  fire(input, 'paste', { clipboardData: { getData: () => '1234567890' } });
  assert.equal(input.value, '123456', 'a long paste stops at six');
  assertClean(page);
});

test('the code in create mode posts {code} and goes only to / or /low', async () => {
  const page = load({ start: 'create' });
  await toCodeScreen(page);
  const input = byId(page, 'code-input');
  input.value = '482913';
  submit(page, 'code');
  const req = page.requests[1];
  assert.equal(req.url, '/api/auth/sign-up/code');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(body(req), { code: '482913' });
  assert.equal(byId(page, 'code-submit').textContent, 'Checking' + ELLIPSIS);
  assert.equal(byId(page, 'code-submit').getAttribute('aria-busy'), 'true');
  answer(req, 200, { next: '/' });
  await flush();
  assert.deepEqual(page.assigned, ['/']);

  const low = load({ start: 'create' });
  await toCodeScreen(low);
  byId(low, 'code-input').value = '111111';
  submit(low, 'code');
  answer(low.requests[1], 200, { next: '/low' });
  await flush();
  assert.deepEqual(low.assigned, ['/low']);

  const odd = load({ start: 'create' });
  await toCodeScreen(odd);
  byId(odd, 'code-input').value = '111111';
  submit(odd, 'code');
  answer(odd.requests[1], 200, { next: 'https://evil.example/' });
  await flush();
  assert.deepEqual(odd.assigned, ['/'], 'a target outside the list goes to /');
  assertClean(page);
  assertClean(low);
  assertClean(odd);
});

test('a refused code keeps the digits selected; five refusals switch the button off until a new code', async () => {
  const page = load({ start: 'create' });
  await toCodeScreen(page);
  const input = byId(page, 'code-input');
  const line = byId(page, 'code-error');
  const submitButton = byId(page, 'code-submit');

  for (let n = 1; n <= 4; n++) {
    input.value = '00000' + n;
    submit(page, 'code');
    answer(page.requests[page.requests.length - 1], 400, { error: { field: 'code', message: 'That code is wrong or has expired.' } });
    await flush();
    assert.equal(line.hidden, false);
    assert.equal(line.textContent, 'That code is wrong or has expired.');
    assert.equal(input.value, '00000' + n, 'the digits stay');
    assert.ok(isFocused(page, 'code-input'));
    assert.ok(page.doc.selected === input, 'the digits are selected');
    assert.equal(input.getAttribute('aria-invalid'), 'true');
    assert.equal(isOff(page, 'code-submit'), false);
  }

  input.value = '000005';
  submit(page, 'code');
  answer(page.requests[page.requests.length - 1], 400, { error: { field: 'code', message: 'That code is wrong or has expired.' } });
  await flush();
  assert.equal(line.textContent, 'Too many wrong tries. Get a new code.');
  assert.equal(isOff(page, 'code-submit'), true);
  assert.equal(input.value, '', 'the field is cleared');
  assert.ok(isFocused(page, 'login-title'), 'Resend is still waiting, so focus goes to the title');

  const before = page.requests.length;
  input.value = '123456';
  const event = submit(page, 'code');
  assert.equal(event.prevented, true);
  assert.equal(page.requests.length, before, 'Enter on a switched off button sends nothing');
  click(page, 'code-submit');
  assert.equal(page.requests.length, before, 'a press on the switched off button sends nothing');

  // Resend ready: focus goes to Resend on the lock; a good resend resets count and button.
  page.advance(45000);
  click(page, 'code-resend');
  const resend = page.requests[page.requests.length - 1];
  assert.equal(resend.url, '/api/auth/code/resend');
  answer(resend, 200, { sent: true });
  await flush();
  assert.equal(isOff(page, 'code-submit'), false);
  assert.equal(input.getAttribute('aria-invalid'), null);
  input.value = '654321';
  submit(page, 'code');
  answer(page.requests[page.requests.length - 1], 400, { error: { field: 'code', message: 'That code is wrong or has expired.' } });
  await flush();
  assert.equal(line.textContent, 'That code is wrong or has expired.', 'the count started again from zero');
  assertClean(page);
});

test('only a refused code counts toward the five: limits and password refusals do not', async () => {
  const page = load({ start: 'signin' });
  click(page, 'signin-forgot');
  byId(page, 'forgot-email').value = 'priya@example.com';
  submit(page, 'forgot');
  answer(page.requests[0], 200, { sent: true });
  await flush();
  const input = byId(page, 'code-input');
  for (let n = 0; n < 6; n++) {
    input.value = '123456';
    byId(page, 'code-new-password').value = 'x';
    submit(page, 'code');
    const req = page.requests[page.requests.length - 1];
    if (n % 2 === 0) {
      answer(req, 400, { error: { field: 'password', message: 'Use at least 8 characters.' } });
    } else {
      answer(req, 429, {});
    }
    await flush();
    assert.equal(isOff(page, 'code-submit'), false, 'attempt ' + n + ' did not count as a wrong code');
  }
  assert.equal(isOff(page, 'code-submit'), false, 'six non code refusals in a row never switch the button off');
  assertClean(page);
});

test('the expiry notice comes from the page: the lifetime, or the seconds left after a reload', () => {
  const fresh = load({ start: 'create', ttl: 600 });
  fresh.doc.byId['create-email'].value = 'priya@example.com';
  fresh.doc.byId['create-password'].value = 'pinalert-demo';
  submit(fresh, 'create');
  answer(fresh.requests[0], 200, { sent: true });
  return flush().then(() => {
    assert.equal(byId(fresh, 'code-status').hidden, true);
    fresh.advance(599000);
    assert.equal(byId(fresh, 'code-status').hidden, true, 'one second before the end nothing shows');
    fresh.advance(1000);
    assert.equal(byId(fresh, 'code-status').hidden, false);
    assert.equal(text(fresh, 'code-status'), 'The code has expired. Get a new one.');
    assert.equal(isOff(fresh, 'code-submit'), false, 'the button stays pressable');
    submit(fresh, 'code');
    assert.equal(fresh.requests.length, 2, 'a press still asks the server');

    const reloaded = load({ start: 'code', mode: 'create', email: 'priya@example.com', secondsLeft: 90, ttl: 600 });
    assert.equal(byId(reloaded, 'code-status').hidden, true);
    reloaded.advance(89000);
    assert.equal(byId(reloaded, 'code-status').hidden, true);
    reloaded.advance(1000);
    assert.equal(text(reloaded, 'code-status'), 'The code has expired. Get a new one.');
    assertClean(fresh);
    assertClean(reloaded);
  });
});

test('Resend posts {}, says a new code is on its way and restarts the wait; a refusal shows inside the code panel', async () => {
  const page = load({ start: 'code', mode: 'create', email: 'priya@example.com', secondsLeft: 500, resend: 45 });
  const resend = byId(page, 'code-resend');
  click(page, 'code-resend');
  assert.equal(page.requests.length, 0, 'pressing Resend while it waits does nothing');

  page.advance(45000);
  assert.equal(resend.textContent, 'Resend code');
  click(page, 'code-resend');
  assert.equal(page.requests.length, 1);
  const req = page.requests[0];
  assert.equal(req.url, '/api/auth/code/resend');
  assert.equal(req.init.method, 'POST');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(body(req), {});
  assert.equal(resend.textContent, 'Sending' + ELLIPSIS);
  assert.equal(resend.getAttribute('aria-busy'), 'true');
  click(page, 'code-resend');
  assert.equal(page.requests.length, 1, 'a second press while sending sends nothing');

  answer(req, 200, { sent: true });
  await flush();
  assert.equal(text(page, 'code-status'), 'A new code is on its way.');
  assert.equal(byId(page, 'code-status').hidden, false);
  assert.equal(resend.textContent, 'Resend in 45s');
  assert.equal(resend.getAttribute('aria-disabled'), 'true');
  assert.equal(resend.getAttribute('aria-busy'), null);

  page.advance(45000);
  click(page, 'code-resend');
  answer(page.requests[1], 429, { error: { field: 'email', message: 'Too many codes for this email. Try again in an hour.' }, retry_after_seconds: 3600 });
  await flush();
  const line = byId(page, 'code-error');
  assert.equal(line.hidden, false);
  assert.equal(line.textContent, 'Too many codes for this email. Try again in an hour.');
  assert.equal(resend.textContent, 'Resend code', 'a refused resend leaves the button ready');
  assert.equal(resend.getAttribute('aria-busy'), null);
  assert.deepEqual(shownPanel(page), ['code']);

  click(page, 'code-resend');
  answer(page.requests[2], 429, {});
  await flush();
  assert.equal(line.textContent, 'Too many requests. Try again in a minute.');
  click(page, 'code-resend');
  refuse(page.requests[3]);
  await flush();
  assert.equal(line.textContent, "You're offline. Try again when you have a signal.");
  assertClean(page);
});

test('Change email returns to the panel the person came from with the email filled; nothing is posted', async () => {
  const page = load({ start: 'create' });
  await toCodeScreen(page, 'priya@example.com', 'pinalert-demo');
  byId(page, 'create-email').value = '';
  click(page, 'code-change-email');
  assert.deepEqual(shownPanel(page), ['create']);
  assert.equal(page.choice.hidden, false);
  assert.equal(byId(page, 'create-email').value, 'priya@example.com');
  assert.equal(byId(page, 'create-password').value, 'pinalert-demo', 'the typed password stays');
  assert.ok(isFocused(page, 'create-email'));
  assert.equal(page.requests.length, 1, 'nothing more is posted');
  assert.equal(page.timers.size, 0, 'the code screen timers stop');

  // From the reset flow it goes back to Forgot password.
  const reset = load({ start: 'signin' });
  byId(reset, 'signin-email').value = 'priya@example.com';
  click(reset, 'signin-forgot');
  submit(reset, 'forgot');
  answer(reset.requests[0], 200, { sent: true });
  await flush();
  byId(reset, 'forgot-email').value = '';
  click(reset, 'code-change-email');
  assert.deepEqual(shownPanel(reset), ['forgot']);
  assert.equal(byId(reset, 'forgot-email').value, 'priya@example.com');
  assert.ok(isFocused(reset, 'forgot-email'));
  assertClean(page);
  assertClean(reset);
});

test('Sign in posts the email and password; a wrong one shows both sentences on every try', async () => {
  const page = load({ start: 'signin' });
  byId(page, 'signin-email').value = 'priya@example.com';
  byId(page, 'signin-password').value = 'not-the-password';
  submit(page, 'signin');
  const req = page.requests[0];
  assert.equal(req.url, '/api/auth/sign-in');
  assert.equal(req.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(body(req), { email: 'priya@example.com', password: 'not-the-password' });
  assert.equal(byId(page, 'signin-submit').textContent, 'Signing in' + ELLIPSIS);
  assert.equal(byId(page, 'signin-submit').getAttribute('aria-busy'), 'true');

  answer(req, 400, { error: { field: 'credentials', message: 'Email or password is wrong.' } });
  await flush();
  const line = byId(page, 'signin-error');
  assert.equal(line.hidden, false);
  assert.equal(byId(page, 'signin-submit').textContent, 'Sign in');
  assert.ok(line.textContent.includes('Email or password is wrong.'));
  assert.ok(line.textContent.includes(OLD_ACCOUNT_LINE));
  assert.equal(line.textContent, 'Email or password is wrong.' + OLD_ACCOUNT_LINE);
  assert.equal(byId(page, 'signin-password').getAttribute('aria-invalid'), 'true');
  assert.ok(isFocused(page, 'signin-password'));

  // The second wrong try shows both again, not twice.
  submit(page, 'signin');
  assert.equal(line.hidden, true);
  answer(page.requests[1], 400, { error: { field: 'credentials', message: 'Email or password is wrong.' } });
  await flush();
  assert.equal(line.textContent, 'Email or password is wrong.' + OLD_ACCOUNT_LINE);

  // A locked sign in shows the server's sentence and no old account line.
  submit(page, 'signin');
  answer(page.requests[2], 429, { error: { field: 'email', message: 'Too many tries. Wait 5 minutes, or reset your password.' }, retry_after_seconds: 300 });
  await flush();
  assert.equal(line.textContent, 'Too many tries. Wait 5 minutes, or reset your password.');
  assert.equal(byId(page, 'signin-submit').getAttribute('aria-busy'), null);

  submit(page, 'signin');
  refuse(page.requests[3]);
  await flush();
  assert.equal(line.textContent, "You're offline. Try again when you have a signal.");

  submit(page, 'signin');
  answer(page.requests[4], 200, { next: '/low' });
  await flush();
  assert.deepEqual(page.assigned, ['/low']);
  assertClean(page);
});

test('Forgot password: copies the email, sends it, takes the code and a new password, returns to Sign in', async () => {
  const page = load({ start: 'signin' });
  byId(page, 'signin-email').value = 'priya@example.com';
  click(page, 'signin-forgot');
  assert.deepEqual(shownPanel(page), ['forgot']);
  assert.equal(page.choice.hidden, true);
  assert.equal(text(page, 'login-title'), 'Reset your password');
  assert.equal(byId(page, 'forgot-email').value, 'priya@example.com');
  assert.ok(isFocused(page, 'forgot-email'));

  submit(page, 'forgot');
  const req = page.requests[0];
  assert.equal(req.url, '/api/auth/reset');
  assert.deepEqual(body(req), { email: 'priya@example.com' });
  assert.equal(byId(page, 'forgot-submit').textContent, 'Sending' + ELLIPSIS);
  answer(req, 200, { sent: true });
  await flush();
  assert.equal(byId(page, 'forgot-submit').textContent, 'Email me a code');

  assert.deepEqual(shownPanel(page), ['code']);
  assert.equal(text(page, 'login-title'), 'Reset your password');
  assert.equal(byId(page, 'code-new-wrap').hidden, false);
  assert.equal(text(page, 'code-submit'), 'Set new password');
  assert.equal(text(page, 'code-email'), 'priya@example.com');
  assert.equal(byId(page, 'code-new-password').hasAttribute('required'), true);

  byId(page, 'code-input').value = '482913';
  byId(page, 'code-new-password').value = 'a new long password';
  submit(page, 'code');
  const codeReq = page.requests[1];
  assert.equal(codeReq.url, '/api/auth/reset/code');
  assert.deepEqual(body(codeReq), { code: '482913', password: 'a new long password' });
  answer(codeReq, 200, { password_set: true });
  await flush();

  assert.deepEqual(shownPanel(page), ['signin']);
  assert.equal(page.choice.hidden, false);
  assert.equal(page.tabSignin.getAttribute('aria-selected'), 'true');
  assert.equal(byId(page, 'signin-email').value, 'priya@example.com');
  assert.equal(byId(page, 'signin-password').value, '');
  assert.ok(isFocused(page, 'signin-password'));
  assert.equal(byId(page, 'signin-status').hidden, false);
  assert.equal(text(page, 'signin-status'), 'Password set. Sign in to continue.');
  assert.equal(byId(page, 'code-new-password').value, '', 'the new password is not kept');
  assert.deepEqual(page.assigned, [], 'a reset signs nobody in');

  // The status is cleared by the next try.
  byId(page, 'signin-password').value = 'a new long password';
  submit(page, 'signin');
  assert.equal(byId(page, 'signin-status').hidden, true);
  assertClean(page);
});

test('Forgot password refusals stay in forgot-error and Back to sign in keeps the email', async () => {
  const page = load({ start: 'signin' });
  click(page, 'signin-forgot');
  byId(page, 'forgot-email').value = 'priya@example.com';
  submit(page, 'forgot');
  answer(page.requests[0], 400, { error: { field: 'email', message: 'Enter a valid email address.' } });
  await flush();
  assert.equal(text(page, 'forgot-error'), 'Enter a valid email address.');
  assert.equal(byId(page, 'forgot-error').hidden, false);
  assert.equal(byId(page, 'forgot-email').getAttribute('aria-invalid'), 'true');

  byId(page, 'forgot-email').value = 'priya@example.org';
  click(page, 'forgot-back');
  assert.deepEqual(shownPanel(page), ['signin']);
  assert.equal(byId(page, 'signin-email').value, 'priya@example.org');
  assert.ok(isFocused(page, 'signin-email'));
  assert.equal(byId(page, 'forgot-error').hidden, true);
  assert.equal(page.requests.length, 1);
  assertClean(page);
});

test('a refused new password in reset mode goes to code-error and does not count as a wrong code', async () => {
  const page = load({ start: 'code', mode: 'reset', email: 'priya@example.com', secondsLeft: 400 });
  byId(page, 'code-input').value = '123456';
  byId(page, 'code-new-password').value = 'short';
  submit(page, 'code');
  answer(page.requests[0], 400, { error: { field: 'password', message: 'Use at least 8 characters.' } });
  await flush();
  assert.equal(text(page, 'code-error'), 'Use at least 8 characters.');
  assert.equal(byId(page, 'code-new-password').getAttribute('aria-invalid'), 'true');
  assert.ok(byId(page, 'code-new-password').getAttribute('aria-describedby').split(' ').includes('code-new-rule'));
  assert.ok(isFocused(page, 'code-new-password'));
  assert.equal(byId(page, 'code-input').getAttribute('aria-invalid'), null);
  assertClean(page);
});

test('Show and Hide switch the type and the words, and leaving the panel or submitting puts them back', async () => {
  const page = load({ start: 'create' });
  const input = byId(page, 'create-password');
  const show = byId(page, 'create-show');
  assert.equal(input.type, 'password');
  click(page, 'create-show');
  assert.equal(input.type, 'text');
  assert.equal(show.textContent, 'Hide');
  assert.equal(show.getAttribute('aria-label'), 'Hide password');
  click(page, 'create-show');
  assert.equal(input.type, 'password');
  assert.equal(show.textContent, 'Show');
  assert.equal(show.getAttribute('aria-label'), 'Show password');

  click(page, 'create-show');
  click(page, 'login-tab-signin');
  assert.equal(input.type, 'password', 'leaving the panel returns it to password');
  assert.equal(show.textContent, 'Show');

  click(page, 'login-tab-create');
  click(page, 'create-show');
  byId(page, 'create-email').value = 'priya@example.com';
  input.value = 'pinalert-demo';
  submit(page, 'create');
  assert.equal(input.type, 'password', 'submitting returns it to password');
  assert.equal(show.getAttribute('aria-label'), 'Show password');

  const signin = load({ start: 'signin' });
  click(signin, 'signin-show');
  assert.equal(byId(signin, 'signin-password').type, 'text');
  assert.equal(byId(signin, 'signin-show').textContent, 'Hide');
  assertClean(page);
  assertClean(signin);
});

test('the page opens on the code screen in reset mode, reading the address from the page text', () => {
  const page = load({ start: 'code', mode: 'reset', email: 'priya@example.com', secondsLeft: 300, resend: 45 });
  assert.deepEqual(shownPanel(page), ['code']);
  assert.equal(byId(page, 'code-new-wrap').hidden, false);
  assert.equal(byId(page, 'code-resend').textContent, 'Resend in 45s');
  assert.equal(page.requests.length, 0);

  byId(page, 'code-input').value = '123456';
  byId(page, 'code-new-password').value = 'a new long password';
  submit(page, 'code');
  assert.equal(page.requests[0].url, '/api/auth/reset/code');

  // Change email goes back to Forgot password with the address from the page.
  const again = load({ start: 'code', mode: 'reset', email: 'priya@example.com', secondsLeft: 300 });
  click(again, 'code-change-email');
  assert.deepEqual(shownPanel(again), ['forgot']);
  assert.equal(byId(again, 'forgot-email').value, 'priya@example.com');

  const create = load({ start: 'code', mode: 'create', email: 'ravi@example.com', secondsLeft: 300 });
  click(create, 'code-change-email');
  assert.deepEqual(shownPanel(create), ['create']);
  assert.equal(byId(create, 'create-email').value, 'ravi@example.com');
  assertClean(page);
  assertClean(again);
  assertClean(create);
});

test('the line above the title goes when the card leaves the entry panels or the choice changes', async () => {
  const page = load({ start: 'signin', done: true });
  assert.equal(byId(page, 'login-done').hidden, false);
  click(page, 'login-tab-create');
  assert.equal(byId(page, 'login-done').hidden, true);

  const other = load({ start: 'create', done: true });
  await toCodeScreen(other);
  assert.equal(byId(other, 'login-done').hidden, true);

  // The line is only there for some visits: a page without it must not break the script.
  const plain = load({ start: 'create' });
  click(plain, 'login-tab-signin');
  assert.deepEqual(shownPanel(plain), ['signin']);
});

test('a full walk touches no query string, address rewrite, storage or markup sink', async () => {
  const page = load({ start: 'create' });
  await toCodeScreen(page);
  click(page, 'code-change-email');
  click(page, 'login-tab-signin');
  click(page, 'signin-forgot');
  byId(page, 'forgot-email').value = 'priya@example.com';
  submit(page, 'forgot');
  answer(page.requests[page.requests.length - 1], 200, { sent: true });
  await flush();
  byId(page, 'code-input').value = '123456';
  byId(page, 'code-new-password').value = 'a new long password';
  submit(page, 'code');
  answer(page.requests[page.requests.length - 1], 200, { password_set: true });
  await flush();
  submit(page, 'signin');
  answer(page.requests[page.requests.length - 1], 200, { next: '/' });
  await flush();
  assert.deepEqual(page.assigned, ['/']);
  assertClean(page);
  for (const req of page.requests) {
    assert.ok(!req.url.includes('?'), 'no request carries a query string: ' + req.url);
    assert.ok(!req.url.includes('@'), 'no address in a request address: ' + req.url);
  }
});

test('the script holds the two pinned sentences once and never mentions the alert flag', () => {
  const count = (needle) => source.split(needle).length - 1;
  assert.equal(count('Too many requests. Try again in a minute.'), 1);
  assert.equal(count('Email or password is wrong.'), 1);
  assert.equal(count('pushIntent'), 0);
  assert.equal(count('localStorage'), 0);
  assert.equal(count('sessionStorage'), 0);
  assert.equal(count('location.search'), 0);
  assert.equal(count('replaceState'), 0);
  assert.equal(count('pushState'), 0);
  assert.equal(count('?sent='), 0);
  assert.equal(source.includes('\u2014'), false);
  assert.equal(source.includes('\u2013'), false);
});

// Phase 7.1 review fix (browser), WR-B01: every request of the card has a time
// limit that also covers reading the body, as request() in welcome.js has. A
// request WebKit leaves unsettled (an iPhone tab suspended on the way to Mail)
// ends after 10 s as a request with no answer: the button is free again, the
// offline line shows, and a second press posts again.
test('WR-B01: a request that never settles ends after 10 s and frees its button', async () => {
  const aborted = [];
  class FakeAbort {
    constructor() { this.signal = { aborted: false }; }
    abort() { this.signal.aborted = true; aborted.push(true); }
  }
  const page = load({ start: 'create', globals: { AbortController: FakeAbort } });
  await fillCreate(page, 'priya@example.com', 'pinalert-demo');
  submit(page, 'create');
  assert.equal(page.requests.length, 1);
  assert.ok(page.requests[0].init.signal, 'the request carries an abort signal');
  page.advance(9999);
  await flush();
  assert.equal(byId(page, 'create-submit').getAttribute('aria-busy'), 'true', 'still waiting before the limit');
  page.advance(1);
  await flush();
  assert.equal(byId(page, 'create-submit').getAttribute('aria-busy'), null);
  assert.equal(text(page, 'create-submit'), 'Email me a code');
  assert.equal(text(page, 'create-error'), "You're offline. Try again when you have a signal.");
  assert.equal(byId(page, 'create-error').hidden, false);
  assert.deepEqual(aborted, [true], 'the stuck request is aborted');
  submit(page, 'create');
  assert.equal(page.requests.length, 2, 'a second press posts again');
  // A late answer to the first request changes nothing.
  answer(page.requests[0], 200, { sent: true });
  await flush();
  assert.deepEqual(shownPanel(page), ['create']);
  assertClean(page);
});

test('WR-B01: the limit covers the body read, and the timer goes once the answer is in', async () => {
  const page = load({ start: 'signin' });
  byId(page, 'signin-email').value = 'priya@example.com';
  byId(page, 'signin-password').value = 'pinalert-demo';
  submit(page, 'signin');
  // Headers came back, the body never does.
  page.requests[0].resolve({ ok: true, status: 200, json: () => new Promise(() => {}) });
  await flush();
  assert.equal(byId(page, 'signin-submit').getAttribute('aria-busy'), 'true');
  page.advance(10000);
  await flush();
  assert.equal(byId(page, 'signin-submit').getAttribute('aria-busy'), null);
  assert.equal(text(page, 'signin-submit'), 'Sign in');
  assert.ok(text(page, 'signin-error').includes("You're offline."));
  assert.deepEqual(page.assigned, []);

  // A request answered in time leaves no timer behind.
  const quick = load({ start: 'signin' });
  submit(quick, 'signin');
  answer(quick.requests[0], 400, { error: { field: 'credentials', message: 'x' } });
  await flush();
  assert.equal(quick.timers.size, 0, 'the limit timer is cleared once the answer is in');
  assertClean(page);
});

test('WR-B01: Resend and the code submit are bounded too', async () => {
  const page = load({ start: 'create' });
  await toCodeScreen(page);
  byId(page, 'code-input').value = '123456';
  submit(page, 'code');
  page.advance(10000);
  await flush();
  assert.equal(byId(page, 'code-submit').getAttribute('aria-busy'), null);
  assert.equal(text(page, 'code-submit'), 'Create account');
  assert.equal(text(page, 'code-error'), "You're offline. Try again when you have a signal.");

  page.advance(45000);
  click(page, 'code-resend');
  const before = page.requests.length;
  assert.equal(page.requests[before - 1].url, '/api/auth/code/resend');
  page.advance(10000);
  await flush();
  assert.equal(byId(page, 'code-resend').getAttribute('aria-busy'), null);
  assert.equal(text(page, 'code-resend'), 'Resend code');
  click(page, 'code-resend');
  assert.equal(page.requests.length, before + 1, 'Resend can be pressed again');
});
