// username_check_test.mjs - runs the real web/static/js/username-check.js in a Node
// sandbox (Phase 7.1, plan 13, D-07, UI-SPEC Surface 7 "The live check").
//
// The one script that checks a username while a person types, used by step 4 of
// the first-time flow and by the Username window of Settings:
//
//   - capitals are folded to lowercase as they are typed and a typed leading @
//     is dropped (the page draws its own @);
//   - the rule line shows at once and the local rule needs no request;
//   - a value that passes the local rule shows "Checking..." and, 300 ms after the
//     last keystroke, asks GET /api/username/check?u= exactly once;
//   - the server's answer sets the state, the words and the look; a stale answer
//     for an older value is ignored; a failed request says so and never counts
//     as good;
//   - every write is textContent, so server text can never be parsed as markup.
//
// Run it with: node --test web/username_check_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what the script calls: an input and a line with the few
// members it touches, timers the test moves by hand and a fetch the test answers.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
// A missing script reads as empty, so every test below fails by itself (and not
// the whole file at import) until the script exists.
let source = '';
try {
  source = fs.readFileSync(path.join(here, 'static', 'js', 'username-check.js'), 'utf8');
} catch (err) {
  source = '';
}

const RULE = 'Use 3 to 20 letters, numbers, dots or underscores.';
const CHECKING = 'Checking\u2026';
const FAILED = 'Couldn\'t check that username. Try again.';

class FakeInput {
  constructor() {
    this.value = '';
    this.handlers = {};
    this.attrs = {};
  }
  addEventListener(type, fn) { (this.handlers[type] = this.handlers[type] || []).push(fn); }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  type(text) {
    this.value = text;
    (this.handlers.input || []).forEach((h) => h({ type: 'input', target: this }));
  }
}

class FakeLine {
  constructor() {
    this.own = '';
    this.className = 'username-status';
    this.attrs = { role: 'status', 'aria-live': 'polite' };
  }
  get textContent() { return this.own; }
  set textContent(v) { this.own = String(v); }
  set innerHTML(v) { throw new Error('innerHTML must never be used: ' + v); }
  set outerHTML(v) { throw new Error('outerHTML must never be used: ' + v); }
  insertAdjacentHTML() { throw new Error('insertAdjacentHTML must never be used'); }
  setAttribute(n, v) { this.attrs[n] = String(v); }
  getAttribute(n) { return Object.hasOwn(this.attrs, n) ? this.attrs[n] : null; }
  hasClass(name) { return this.className.split(/\s+/).includes(name); }
}

// A fresh sandbox with the script loaded, an input, a line, hand-moved timers and
// a fetch whose calls the test answers.
function setup(options = {}) {
  const sandbox = { console };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'username-check.js' });
  const input = new FakeInput();
  // Amended 2026-11-08 by quick 261108-fku (owner decision 2026-11-08): a test may
  // draw the field read only before the script attaches, as the locked Settings
  // window does.
  if (options.readonly) { input.setAttribute('readonly', ''); }
  const line = new FakeLine();
  const requests = [];
  const changes = [];
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
      timers.delete(id);
      timer.fn();
    }
    now = end;
  };
  const fetchFn = (target, init) => new Promise((resolve, reject) => {
    requests.push({ url: String(target), init: init || {}, resolve, reject });
  });
  const control = sandbox.PinalertUsernameCheck.attach(input, line, {
    mode: options.mode || 'signup',
    debounceMs: 300,
    setTimer(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id; },
    clearTimer(id) { timers.delete(id); },
    fetchFn,
    onChange(state) { changes.push(state); }
  });
  return { sandbox, input, line, requests, changes, advance, control, timers };
}

const flush = async () => { for (let i = 0; i < 6; i++) { await new Promise((r) => setImmediate(r)); } };

const reply = (req, status, body) => req.resolve({
  ok: status >= 200 && status < 300,
  status,
  json: () => Promise.resolve(body || {})
});

test('capitals are folded to lowercase as they are typed and a typed leading @ is dropped', () => {
  const page = setup();
  page.input.type('Ravi.K');
  assert.equal(page.input.value, 'ravi.k');
  page.input.type('@Anjali_R');
  assert.equal(page.input.value, 'anjali_r');
  assert.equal(page.control.name(), 'anjali_r');
});

test('with nothing typed the line shows the rule with no icon and nothing is asked', () => {
  const page = setup();
  assert.equal(page.line.textContent, RULE);
  assert.equal(page.line.hasClass('username-status--good'), false);
  assert.equal(page.line.hasClass('username-status--bad'), false);
  assert.equal(page.control.state(), 'rule');
  assert.equal(page.control.isGood(), false);
  page.input.type('');
  assert.equal(page.line.textContent, RULE);
  assert.equal(page.line.hasClass('username-status--bad'), false, 'an empty field is not an error');
  page.advance(2000);
  assert.equal(page.requests.length, 0);
});

test('a value that breaks the local rule shows the rule with the bad look and makes no request', () => {
  const page = setup();
  ['ab', 'ravi k', '.ravi', 'ravi.', 'ra..vi', '12345', 'a'.repeat(21), 'ravi!'].forEach((bad) => {
    page.input.type(bad);
    assert.equal(page.line.textContent, RULE, bad);
    assert.equal(page.line.hasClass('username-status--bad'), true, bad + ' has the bad look');
    assert.equal(page.control.isGood(), false);
  });
  page.advance(2000);
  assert.equal(page.requests.length, 0, 'the local rule never asks the server');
});

test('a value that passes the rule shows Checking and asks once, 300 ms after the last keystroke', () => {
  const page = setup();
  page.input.type('ravi');
  assert.equal(page.line.textContent, CHECKING);
  assert.equal(page.control.state(), 'checking');
  page.advance(299);
  assert.equal(page.requests.length, 0, 'not before the pause is over');
  page.advance(1);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/username/check?u=ravi');
  assert.equal(page.requests[0].init.credentials, 'same-origin');
});

test('three quick keystrokes make one request, for the last value', () => {
  const page = setup();
  page.input.type('rav');
  page.advance(100);
  page.input.type('ravi');
  page.advance(100);
  page.input.type('ravi.k');
  page.advance(299);
  assert.equal(page.requests.length, 0);
  page.advance(1);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].url, '/api/username/check?u=ravi.k');
});

test('each answer sets the state, the server words and the look', async () => {
  const cases = [
    ['available', 'ravi.k is available.', 'username-status--good', true],
    ['taken', 'ravi.k is taken. Try another.', 'username-status--bad', false],
    ['reserved', 'That name is kept for official use.', 'username-status--bad', false],
    ['invalid', RULE, 'username-status--bad', false],
    ['yours', 'This is your username now.', null, false],
    ['wait', 'You can change your username again in 3 days.', 'username-status--warn', false]
  ];
  for (const [state, message, look, good] of cases) {
    const page = setup({ mode: 'settings' });
    page.input.type('ravi.k');
    page.advance(300);
    reply(page.requests[0], 200, { state, message, days: state === 'wait' ? 3 : undefined });
    await flush();
    assert.equal(page.control.state(), state);
    assert.equal(page.line.textContent, message);
    ['username-status--good', 'username-status--bad', 'username-status--warn'].forEach((cls) => {
      assert.equal(page.line.hasClass(cls), cls === look, state + ' ' + cls);
    });
    assert.equal(page.control.isGood(), good, state + ' isGood');
    assert.equal(page.changes[page.changes.length - 1], state, 'onChange reports the state');
  }
});

test('in the sign up step a name that is already the account\'s own counts as good, in Settings it does not', async () => {
  const signup = setup({ mode: 'signup' });
  signup.input.type('ravi.k');
  signup.advance(300);
  reply(signup.requests[0], 200, { state: 'yours', message: 'This is your username now.' });
  await flush();
  assert.equal(signup.control.isGood(), true);

  const settings = setup({ mode: 'settings' });
  settings.input.type('ravi.k');
  settings.advance(300);
  reply(settings.requests[0], 200, { state: 'yours', message: 'This is your username now.' });
  await flush();
  assert.equal(settings.control.isGood(), false);
});

test('a stale answer for an older value is ignored', async () => {
  const page = setup();
  page.input.type('first');
  page.advance(300);
  page.input.type('second');
  assert.equal(page.control.state(), 'checking');
  reply(page.requests[0], 200, { state: 'available', message: 'first is available.' });
  await flush();
  assert.equal(page.control.state(), 'checking', 'the answer for first does not settle second');
  assert.equal(page.line.textContent, CHECKING);
  assert.equal(page.control.isGood(), false);
  page.advance(300);
  reply(page.requests[1], 200, { state: 'taken', message: 'second is taken. Try another.' });
  await flush();
  assert.equal(page.control.state(), 'taken');
});

test('a failed request or an answer that is not 200 says so and is never good', async () => {
  const offline = setup();
  offline.input.type('ravi.k');
  offline.advance(300);
  offline.requests[0].reject(new Error('offline'));
  await flush();
  assert.equal(offline.control.state(), 'failed');
  assert.equal(offline.line.textContent, FAILED);
  assert.equal(offline.control.isGood(), false);

  // Amended 2026-10-29 by the Phase 7.1 review fix (browser), WR-B03: a 429 is no
  // longer a failed check at once; it keeps Checking and is asked again (see the
  // WR-B03 cases). It is still never good. A 500 stands in for "not 200" here.
  const limited = setup();
  limited.input.type('ravi.k');
  limited.advance(300);
  reply(limited.requests[0], 429, { error: { field: 'username', message: 'Too many tries.' } });
  await flush();
  assert.equal(limited.control.state(), 'checking');
  assert.equal(limited.control.isGood(), false);

  const broken = setup();
  broken.input.type('ravi.k');
  broken.advance(300);
  reply(broken.requests[0], 500, { error: { field: 'username', message: 'Couldn\'t check that username. Try again.' } });
  await flush();
  assert.equal(broken.control.state(), 'failed');
  assert.equal(broken.line.textContent, FAILED);
  assert.equal(broken.control.isGood(), false);

  const odd = setup();
  odd.input.type('ravi.k');
  odd.advance(300);
  reply(odd.requests[0], 200, { state: 'something-new', message: 'x' });
  await flush();
  assert.equal(odd.control.state(), 'failed', 'a state the script does not know is treated as a failed check');
  assert.equal(odd.control.isGood(), false);
});

test('typing again after a failure asks again', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  page.requests[0].reject(new Error('offline'));
  await flush();
  page.input.type('ravi.kk');
  page.advance(300);
  assert.equal(page.requests.length, 2);
  reply(page.requests[1], 200, { state: 'available', message: 'ravi.kk is available.' });
  await flush();
  assert.equal(page.control.isGood(), true);
});

test('showSaveError writes the refusal into the line with the bad look and keeps the name from counting as good', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  reply(page.requests[0], 200, { state: 'available', message: 'ravi.k is available.' });
  await flush();
  assert.equal(page.control.isGood(), true);

  page.control.showSaveError('That username can\'t be used. Try another.');
  assert.equal(page.line.textContent, 'That username can\'t be used. Try another.');
  assert.equal(page.line.hasClass('username-status--bad'), true);
  assert.equal(page.line.hasClass('username-status--good'), false);
  assert.equal(page.control.isGood(), false, 'the same name is not good again until it is typed again');

  page.input.type('ravi.kx');
  assert.equal(page.line.textContent, CHECKING);
});

test('every write is textContent: server markup reaches the line as plain text', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  reply(page.requests[0], 200, { state: 'taken', message: '<img src=x onerror=alert(1)> is taken.' });
  await flush();
  assert.equal(page.line.textContent, '<img src=x onerror=alert(1)> is taken.');
  // The role and aria-live come from the template; the script never changes them.
  assert.equal(page.line.getAttribute('role'), 'status');
  assert.equal(page.line.getAttribute('aria-live'), 'polite');
  page.control.showSaveError('<b>no</b>');
  assert.equal(page.line.textContent, '<b>no</b>');
});

test('the script holds no markup sink', () => {
  assert.ok(source.length > 0, 'username-check.js exists');
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write/.test(source), false);
  assert.ok(source.includes('encodeURIComponent'), 'the value goes through encodeURIComponent');
});

test('a press on the @ in the box puts the cursor in the input', () => {
  const sandbox = { console };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'username-check.js' });
  const input = new FakeInput();
  let focused = 0;
  input.focus = () => { focused += 1; };
  const handlers = [];
  input.parentNode = { className: 'username-field', addEventListener(type, fn) { handlers.push({ type, fn }); } };
  sandbox.PinalertUsernameCheck.attach(input, new FakeLine(), {
    setTimer() { return 1; }, clearTimer() {}, fetchFn() { return new Promise(() => {}); }
  });
  assert.equal(handlers.length, 1);
  assert.equal(handlers[0].type, 'click');
  handlers[0].fn({ target: {} });
  assert.equal(focused, 1);
  handlers[0].fn({ target: input });
  assert.equal(focused, 1, 'a press on the input itself is left alone');
});

test('a value already in the field when the script attaches is checked without a keystroke', () => {
  const sandbox = { console };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'username-check.js' });
  const input = new FakeInput();
  input.value = 'Asha.K';
  const line = new FakeLine();
  const timers = [];
  const control = sandbox.PinalertUsernameCheck.attach(input, line, {
    mode: 'signup',
    setTimer(fn, ms) { timers.push({ fn, ms }); return timers.length; },
    clearTimer() {},
    fetchFn() { return new Promise(() => {}); }
  });
  assert.equal(input.value, 'asha.k');
  assert.equal(control.state(), 'checking');
  assert.equal(timers.length, 1);
  assert.equal(timers[0].ms, 300);
});

// Phase 7.1 review fix (browser), IN-B04: folding while typing keeps the caret
// where the person was typing, and a paste is folded before the field's own cut
// at 20, so a pasted "@name" keeps every letter of the name.
class CaretInput extends FakeInput {
  constructor(doc) {
    super();
    this.ownerDocument = doc;
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.attrs.maxlength = '20';
  }
  setSelectionRange(start, end) { this.selectionStart = start; this.selectionEnd = end; }
  focus() { this.ownerDocument.activeElement = this; }
  typeAt(text, caret) {
    this.value = text;
    this.selectionStart = caret;
    this.selectionEnd = caret;
    (this.handlers.input || []).forEach((h) => h({ type: 'input', target: this }));
  }
  paste(text) {
    const event = {
      type: 'paste',
      prevented: false,
      preventDefault() { this.prevented = true; },
      clipboardData: { getData: (kind) => (kind === 'text' || kind === 'text/plain' ? text : '') }
    };
    (this.handlers.paste || []).forEach((h) => h(event));
    return event;
  }
}

function caretSetup() {
  const sandbox = { console };
  sandbox.window = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'username-check.js' });
  const doc = { activeElement: null };
  const input = new CaretInput(doc);
  const line = new FakeLine();
  const requests = [];
  const control = sandbox.PinalertUsernameCheck.attach(input, line, {
    setTimer() { return 1; },
    clearTimer() {},
    fetchFn(target) { requests.push(String(target)); return new Promise(() => {}); }
  });
  input.focus();
  return { input, line, control, requests };
}

test('IN-B04: a capital typed in the middle keeps the caret there; a dropped @ moves it back one', () => {
  const page = caretSetup();
  page.input.typeAt('raVi', 3);
  assert.equal(page.input.value, 'ravi');
  assert.equal(page.input.selectionStart, 3);
  assert.equal(page.input.selectionEnd, 3);

  page.input.typeAt('@ravi', 5);
  assert.equal(page.input.value, 'ravi');
  assert.equal(page.input.selectionStart, 4);

  page.input.typeAt(' Ra', 1);
  assert.equal(page.input.value, 'ra');
  assert.equal(page.input.selectionStart, 0);
});

test('IN-B04: a pasted "@name" of 21 characters keeps all 19 letters of the name', () => {
  const page = caretSetup();
  const event = page.input.paste('@Ravi.Kumar_12345678');
  assert.equal(event.prevented, true, 'the browser does not cut the paste first');
  assert.equal(page.input.value, 'ravi.kumar_12345678');
  assert.equal(page.input.selectionStart, 19);
  assert.equal(page.control.state(), 'checking', 'the pasted name is judged');

  const long = caretSetup();
  long.input.paste('abcdefghijklmnopqrstuvwxyz');
  assert.equal(long.input.value, 'abcdefghijklmnopqrst', 'cut to 20 after folding');

  const inside = caretSetup();
  inside.input.typeAt('ravi', 2);
  inside.input.paste('@XY');
  assert.equal(inside.input.value, 'raxyvi', 'pasted where the caret is; the @ inside a name is not a leading @');
  assert.equal(inside.input.selectionStart, 4);

  const empty = caretSetup();
  const nothing = empty.input.paste('');
  assert.equal(nothing.prevented, false, 'an empty paste is left to the browser');
});

// Phase 7.1 review fix (browser), WR-B03: a check refused by the per account
// allowance (429) is asked again by itself after the wait the server names, and
// a check WebKit leaves unsettled ends as failed after 10 s, so neither leaves
// Continue or Save off with nothing to press.
const limited = (req, retryAfter, body) => req.resolve({
  ok: false,
  status: 429,
  headers: { get(name) { return name.toLowerCase() === 'retry-after' && retryAfter !== null ? String(retryAfter) : null; } },
  json: () => Promise.resolve(body || { error: { field: 'username', message: 'Too many requests. Try again in a minute.' }, retry_after_seconds: 1 })
});

test('WR-B03: a 429 keeps Checking and asks again for the same value after Retry-After', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  assert.equal(page.requests.length, 1);
  limited(page.requests[0], 1);
  await flush();
  assert.equal(page.control.state(), 'checking');
  assert.equal(page.line.textContent, CHECKING);
  assert.equal(page.control.isGood(), false);
  page.advance(999);
  assert.equal(page.requests.length, 1);
  page.advance(1);
  assert.equal(page.requests.length, 2, 'asked again by itself');
  assert.equal(page.requests[1].url, page.requests[0].url);
  reply(page.requests[1], 200, { state: 'available', message: 'ravi.k is available.' });
  await flush();
  assert.equal(page.control.state(), 'available');
  assert.equal(page.control.isGood(), true);
});

test('WR-B03: with no Retry-After the body wait is used, and a keystroke cancels the second ask', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  limited(page.requests[0], null, { error: { message: 'Too many requests. Try again in a minute.' }, retry_after_seconds: 2 });
  await flush();
  page.advance(1999);
  assert.equal(page.requests.length, 1);
  page.input.type('ravi.kumar');
  page.advance(300);
  assert.equal(page.requests.length, 2);
  assert.ok(page.requests[1].url.endsWith('ravi.kumar'));
  page.advance(5000);
  assert.equal(page.requests.length, 2, 'the retry of the older value was cancelled');
});

test('WR-B03: refused again and again, the check gives up as failed with the server sentence', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  for (let i = 0; i < 4; i++) {
    limited(page.requests[i], 1);
    await flush();
    page.advance(1000);
  }
  assert.equal(page.requests.length, 4, 'three asks again, no more');
  assert.equal(page.control.state(), 'failed');
  assert.equal(page.line.textContent, 'Too many requests. Try again in a minute.');
  assert.equal(page.control.isGood(), false);
});

test('WR-B03: a check that never settles ends as failed after 10 s; one answered in time stays', async () => {
  const page = setup();
  page.input.type('ravi.k');
  page.advance(300);
  page.advance(9999);
  await flush();
  assert.equal(page.control.state(), 'checking');
  page.advance(1);
  await flush();
  assert.equal(page.control.state(), 'failed');
  assert.equal(page.line.textContent, FAILED);
  // The answer that comes after the limit changes nothing.
  reply(page.requests[0], 200, { state: 'available', message: 'ravi.k is available.' });
  await flush();
  assert.equal(page.control.state(), 'failed');

  const quick = setup();
  quick.input.type('ravi.k');
  quick.advance(300);
  reply(quick.requests[0], 200, { state: 'available', message: 'ravi.k is available.' });
  await flush();
  quick.advance(20000);
  await flush();
  assert.equal(quick.control.state(), 'available', 'no leftover limit marks a good answer failed');
  assert.equal(quick.timers.size, 0);
});

// Quick 261108-fku (owner decision 2026-11-08): the locked Username window of
// Settings. hold(message) shows a wait sentence the page already has and drops
// anything due; a read only field is never judged.
test('hold shows the sentence in the wait look, drops a check that is due and never counts as good', async () => {
  const page = setup({ mode: 'settings' });
  page.input.type('ravi.kumar');
  assert.equal(page.control.state(), 'checking');
  const sentence = 'You can change your username again in 7 days.';
  page.control.hold(sentence);
  page.advance(1000);
  await flush();
  assert.equal(page.requests.length, 0, 'the check that was due is dropped');
  assert.equal(page.line.textContent, sentence);
  assert.equal(page.line.hasClass('username-status--warn'), true);
  assert.equal(page.control.state(), 'wait');
  assert.equal(page.control.isGood(), false);
  assert.equal(page.changes[page.changes.length - 1], 'wait', 'the page is told');
});

test('hold drops an answer that is still out', async () => {
  const page = setup({ mode: 'settings' });
  page.input.type('ravi.kumar');
  page.advance(300);
  assert.equal(page.requests.length, 1);
  page.control.hold('You can change your username again in 3 days.');
  reply(page.requests[0], 200, { state: 'available', message: 'ravi.kumar is available.' });
  await flush();
  assert.equal(page.control.state(), 'wait');
  assert.equal(page.control.isGood(), false);
  assert.equal(page.line.textContent, 'You can change your username again in 3 days.');
});

test('a read only field is not judged at attach and an input event on it sends nothing', async () => {
  const page = setup({ mode: 'settings', readonly: true });
  assert.equal(page.line.textContent, '', 'attach leaves the line alone');
  assert.equal(page.changes.length, 0);
  page.input.type('ravi.kumar');
  page.advance(1000);
  await flush();
  assert.equal(page.requests.length, 0);
  assert.equal(page.line.textContent, '');
  assert.equal(page.timers.size, 0);
});

test('a paste into a read only field is left to the browser', () => {
  const page = setup({ mode: 'settings', readonly: true });
  let prevented = false;
  (page.input.handlers.paste || []).forEach((h) => h({
    clipboardData: { getData: () => 'ravi.k' },
    preventDefault() { prevented = true; }
  }));
  assert.equal(prevented, false);
  assert.equal(page.input.value, '');
  assert.equal(page.requests.length, 0);
});
