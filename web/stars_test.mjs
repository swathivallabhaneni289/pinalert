// stars_test.mjs - runs the real web/static/js/stars.js in a Node sandbox and
// checks the shared five star control (Phase 9 plan 16, UI-SPEC Surface 3, 3a):
// five buttons named 1 star to 5 stars, a tap saves at once with the five
// disabled and aria-busy while the call is out, the chosen star carries
// aria-pressed and the stars up to it are filled, a refusal shows the server's
// sentence and keeps the old value, Remove my rating shows only with a rating,
// no arrow key does anything, and no markup sink is used.
//
// Run it with: node --test web/stars_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'stars.js'), 'utf8');

const flush = () => new Promise((resolve) => setImmediate(resolve));

// load runs stars.js and mounts one control for asha_k. route(method, url, init)
// answers each call with {status, body} or a promise of one.
function load({ route, current = 0, online = true, extra = {} } = {}) {
  const sinks = [];
  const calls = [];
  const status = [];
  const errors = [];
  let gone = 0;

  function fakeEl(tag) {
    const el = {
      tag,
      className: '',
      hidden: false,
      disabled: false,
      attrs: {},
      listeners: [],
      children: [],
      parentNode: null,
      focused: 0,
      textContent: '',
      setAttribute(name, value) { this.attrs[name] = String(value); },
      getAttribute(name) { return Object.prototype.hasOwnProperty.call(this.attrs, name) ? this.attrs[name] : null; },
      removeAttribute(name) { delete this.attrs[name]; },
      addEventListener(type, fn) { this.listeners.push({ type, fn }); },
      appendChild(node) { node.parentNode = this; this.children.push(node); return node; },
      removeChild(node) { this.children = this.children.filter((c) => c !== node); node.parentNode = null; return node; },
      focus() { this.focused += 1; },
      fire(type, e = {}) { this.listeners.filter((l) => l.type === type).forEach((l) => l.fn(e)); }
    };
    for (const sink of ['innerHTML', 'outerHTML']) {
      Object.defineProperty(el, sink, { set(v) { sinks.push(sink + ' = ' + v); }, get() { return ''; } });
    }
    el.insertAdjacentHTML = (...args) => { sinks.push('insertAdjacentHTML ' + args.join(' ')); };
    return el;
  }

  const document = {
    createElement: (tag) => fakeEl(tag),
    write(...args) { sinks.push('document.write ' + args.join(' ')); }
  };
  const window = { setTimeout: () => 1, clearTimeout() {} };
  const fetch = (address, init = {}) => {
    const method = init.method || 'GET';
    calls.push({ method, address, init });
    return Promise.resolve(route(method, address, init)).then((answer) => ({
      status: answer.status,
      ok: answer.status >= 200 && answer.status < 300,
      json: () => (answer.body === undefined ? Promise.reject(new Error('no body')) : Promise.resolve(answer.body))
    }));
  };
  const navigator = { onLine: online };
  vm.runInNewContext(source, { document, window, fetch, navigator, Promise, JSON, encodeURIComponent, isFinite, Math, Number }, { filename: 'stars.js' });

  const control = window.PinalertStars.createControl({
    username: 'asha_k',
    current,
    onStatus: (line) => status.push(line),
    onError: (line) => errors.push(line),
    ...extra,
    onGone: extra.onGone === undefined ? undefined : () => { gone += 1; extra.onGone(); }
  });
  const fieldset = control.el;
  const row = fieldset.children[1];
  const stars = row.children.slice(0, 5);
  const remove = row.children[5];
  const tap = (n) => stars[n - 1].fire('click', { target: stars[n - 1] });
  const filled = () => stars.map((s) => s.getAttribute('data-filled') === 'true');
  const pressed = () => stars.map((s) => s.getAttribute('aria-pressed'));
  return { control, fieldset, stars, remove, calls, status, errors, sinks, tap, filled, pressed, goneCount: () => gone, window };
}

const OK = () => ({ status: 204 });

test('five buttons are named 1 star to 5 stars under the legend Your rating', () => {
  const m = load({ route: OK });
  assert.equal(m.fieldset.tag, 'fieldset');
  assert.equal(m.fieldset.children[0].tag, 'legend');
  assert.equal(m.fieldset.children[0].textContent, 'Your rating');
  assert.deepEqual(m.stars.map((s) => s.getAttribute('aria-label')), ['1 star', '2 stars', '3 stars', '4 stars', '5 stars']);
  assert.deepEqual(m.stars.map((s) => s.tag), ['button', 'button', 'button', 'button', 'button']);
  assert.deepEqual(m.stars.map((s) => s.getAttribute('type')), ['button', 'button', 'button', 'button', 'button']);
});

test('an unrated control draws five outline stars, none pressed, and no Remove my rating', () => {
  const m = load({ route: OK, current: 0 });
  assert.deepEqual(m.filled(), [false, false, false, false, false]);
  assert.deepEqual(m.pressed(), ['false', 'false', 'false', 'false', 'false']);
  assert.equal(m.remove.hidden, true);
  assert.equal(m.remove.textContent, 'Remove my rating');
});

test('a rating of 4 fills stars 1 to 4, presses the fourth and shows Remove my rating', () => {
  const m = load({ route: OK, current: 4 });
  assert.deepEqual(m.filled(), [true, true, true, true, false]);
  assert.deepEqual(m.pressed(), ['false', 'false', 'false', 'true', 'false']);
  assert.equal(m.remove.hidden, false);
});

test('tapping 4 posts {"stars":4} once, disables all five with aria-busy, then marks the fourth', async () => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const m = load({ route: () => gate.then(OK) });
  m.tap(4);
  await flush();
  assert.equal(m.calls.length, 1);
  assert.equal(m.calls[0].method + ' ' + m.calls[0].address, 'POST /api/people/asha_k/rating');
  assert.equal(m.calls[0].init.body, '{"stars":4}');
  assert.equal(m.calls[0].init.headers['Content-Type'], 'application/json');
  assert.equal(m.calls[0].init.credentials, 'same-origin');
  assert.deepEqual(m.stars.map((s) => s.disabled), [true, true, true, true, true]);
  assert.deepEqual(m.stars.map((s) => s.getAttribute('aria-busy')), ['true', 'true', 'true', 'true', 'true']);
  m.tap(2);
  await flush();
  assert.equal(m.calls.length, 1, 'a second tap sends nothing while a call is out');
  release();
  await flush();
  assert.deepEqual(m.stars.map((s) => s.disabled), [false, false, false, false, false]);
  assert.deepEqual(m.stars.map((s) => s.getAttribute('aria-busy')), [null, null, null, null, null]);
  assert.deepEqual(m.pressed(), ['false', 'false', 'false', 'true', 'false']);
  assert.deepEqual(m.filled(), [true, true, true, true, false]);
  assert.equal(m.remove.hidden, false, 'Remove my rating appears once a rating exists');
  assert.deepEqual(m.status, ['You rated asha_k 4 stars.']);
  assert.equal(m.stars[3].focused, 1, 'focus stays on the star that was tapped');
});

test('one star reads "1 star" in the status line', async () => {
  const m = load({ route: OK });
  m.tap(1);
  await flush();
  assert.deepEqual(m.status, ['You rated asha_k 1 star.']);
});

test('a 409 shows the server sentence and leaves the old value', async () => {
  const sentence = 'You can change this rating again tomorrow.';
  const m = load({ route: () => ({ status: 409, body: { error: { field: 'stars', message: sentence } } }), current: 2 });
  m.tap(5);
  await flush();
  assert.deepEqual(m.errors, ['', sentence]);
  assert.deepEqual(m.status, []);
  assert.deepEqual(m.pressed(), ['false', 'true', 'false', 'false', 'false']);
  assert.deepEqual(m.filled(), [true, true, false, false, false]);
  assert.equal(m.stars[0].disabled, false);
});

test('a 400 and a server error say Couldn\'t save your rating', async () => {
  for (const code of [400, 500]) {
    const m = load({ route: () => ({ status: code, body: { error: { field: 'stars', message: 'Choose 1 to 5 stars.' } } }) });
    m.tap(3);
    await flush();
    assert.deepEqual(m.errors, ['', 'Couldn\'t save your rating. Try again.'], String(code));
  }
});

test('a 429 builds the wait from retry_after_seconds, in whole minutes', async () => {
  const cases = [[30, 'Too many tries. Wait 1 minute and try again.'], [61, 'Too many tries. Wait 2 minutes and try again.'], [300, 'Too many tries. Wait 5 minutes and try again.']];
  for (const [seconds, line] of cases) {
    const m = load({ route: () => ({ status: 429, body: { error: { message: 'You\'re doing that very quickly.' }, retry_after_seconds: seconds } }) });
    m.tap(3);
    await flush();
    assert.equal(m.errors[1], line, String(seconds));
  }
});

test('a 404 goes to onGone when the caller gave one', async () => {
  let reloads = 0;
  const m = load({ route: () => ({ status: 404, body: {} }), extra: { onGone: () => { reloads += 1; } } });
  m.tap(3);
  await flush();
  assert.equal(reloads, 1);
  assert.deepEqual(m.errors, ['']);
});

test('a network failure says Couldn\'t save, and offline says the built line without sending', async () => {
  const down = load({ route: () => Promise.reject(new Error('down')) });
  down.tap(3);
  await flush();
  assert.deepEqual(down.errors, ['', 'Couldn\'t save your rating. Try again.']);
  assert.equal(down.stars[0].disabled, false);

  const off = load({ route: OK, online: false });
  off.tap(3);
  await flush();
  assert.equal(off.calls.length, 0);
  assert.equal(off.errors[off.errors.length - 1], 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.');
});

test('Remove my rating posts clear once and says Your rating was removed.', async () => {
  const m = load({ route: OK, current: 3 });
  m.remove.fire('click', { target: m.remove });
  await flush();
  assert.equal(m.calls.length, 1);
  assert.equal(m.calls[0].method + ' ' + m.calls[0].address, 'POST /api/people/asha_k/rating/clear');
  assert.deepEqual(m.status, ['Your rating was removed.']);
  assert.deepEqual(m.filled(), [false, false, false, false, false]);
  assert.deepEqual(m.pressed(), ['false', 'false', 'false', 'false', 'false']);
  assert.equal(m.remove.hidden, true);
  assert.equal(m.stars[0].focused, 1, 'focus moves to the first star when Remove goes away');
});

test('a failed removal says Couldn\'t remove your rating and keeps the rating', async () => {
  const m = load({ route: () => ({ status: 500, body: {} }), current: 3 });
  m.remove.fire('click', { target: m.remove });
  await flush();
  assert.deepEqual(m.errors, ['', 'Couldn\'t remove your rating. Try again.']);
  assert.deepEqual(m.pressed(), ['false', 'false', 'true', 'false', 'false']);
  assert.equal(m.remove.hidden, false);
});

test('set redraws from a new server value and destroy takes the control out', () => {
  const m = load({ route: OK, current: 0 });
  const parent = { children: [], removeChild(node) { this.children = this.children.filter((c) => c !== node); node.parentNode = null; }, appendChild(node) { node.parentNode = this; this.children.push(node); } };
  parent.appendChild(m.fieldset);
  m.control.set(2);
  assert.deepEqual(m.filled(), [true, true, false, false, false]);
  assert.equal(m.remove.hidden, false);
  m.control.set(0);
  assert.equal(m.remove.hidden, true);
  m.control.destroy();
  assert.equal(parent.children.length, 0);
});

// 2026-10-30 (09-REVIEW IN-B02): set does nothing while a save is out, so a
// profile read that started earlier cannot draw the old rating over the new one.
test('set is ignored while a save is out and works again after it', async () => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const m = load({ route: () => gate.then(OK), current: 2 });
  m.tap(5);
  await flush();
  m.control.set(2);
  assert.deepEqual(m.pressed(), ['false', 'true', 'false', 'false', 'false'], 'no redraw while busy');
  release();
  await flush();
  assert.deepEqual(m.pressed(), ['false', 'false', 'false', 'false', 'true'], 'the answer set the new value');
  m.control.set(3);
  assert.deepEqual(m.pressed(), ['false', 'false', 'true', 'false', 'false']);
});

test('no arrow key changes the value: the star buttons listen for click only', async () => {
  const m = load({ route: OK, current: 2 });
  for (const star of m.stars) {
    assert.deepEqual(star.listeners.map((l) => l.type), ['click']);
  }
  assert.equal(/ArrowRight|ArrowLeft|ArrowUp|ArrowDown|keydown|keyup/.test(source), false);
  assert.equal(m.calls.length, 0);
});

test('the control makes no markup sink and the script text has none', async () => {
  const m = load({ route: OK });
  m.tap(5);
  await flush();
  m.remove.fire('click', {});
  await flush();
  assert.deepEqual(m.sinks, []);
  assert.equal(/innerHTML|outerHTML|insertAdjacentHTML|document\.write|eval\(/.test(source), false);
  assert.ok((source.match(/textContent/g) || []).length >= 2);
});
