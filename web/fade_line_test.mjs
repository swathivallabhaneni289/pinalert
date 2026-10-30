// fade_line_test.mjs - runs the real web/static/js/app.js in a Node sandbox and
// pins the fade line of the map popup (backlog 999.12): the line ends with
// "unless confirmed again" exactly while the server's confirm_extends is true,
// and reads as before otherwise.
//
// Run it with: node --test web/fade_line_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs in
// .github/workflows/ci.yml). It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'app.js'), 'utf8');

// loadApp evaluates app.js against a page with no shell, no list pane and no
// toast, the smallest sandbox that lets the module load.
function loadApp() {
  const rootAttrs = {};
  const document = {
    getElementById: () => null,
    querySelector: () => null,
    documentElement: {
      setAttribute: (name, value) => { rootAttrs[name] = String(value); },
      hasAttribute: (name) => Object.hasOwn(rootAttrs, name),
      removeAttribute: (name) => { delete rootAttrs[name]; }
    }
  };
  const window = {
    setTimeout: () => 0,
    clearTimeout: () => {},
    setInterval: () => 0,
    clearInterval: () => {}
  };
  const sandbox = {
    window,
    document,
    fetch: () => Promise.reject(new Error('no network in this test')),
    Request: function Request(target) { this.url = target; },
    AbortController: function AbortController() { this.signal = {}; this.abort = () => {}; },
    Promise,
    Date,
    Math,
    JSON,
    isFinite,
    parseFloat,
    parseInt,
    encodeURIComponent
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return sandbox.window.Pinalert;
}

const Pinalert = loadApp();
const minute = 60 * 1000;
const hour = 60 * minute;

// fade builds a report whose expires_at is offsetMs from now; flag is added
// only when it is given.
function fade(offsetMs, flag) {
  const report = { expires_at: new Date(Date.now() + offsetMs).toISOString() };
  if (flag !== undefined) {
    report.confirm_extends = flag;
  }
  return Pinalert.formatFade(report);
}

test('7 hours left with confirm_extends true carries the promise', () => {
  assert.equal(fade(7.5 * hour, true), 'Fades in 7 hrs unless confirmed again.');
});

test('7 hours left with confirm_extends false has no promise', () => {
  assert.equal(fade(7.5 * hour, false), 'Fades in 7 hrs.');
});

test('7 hours left with no confirm_extends key has no promise', () => {
  assert.equal(fade(7.5 * hour), 'Fades in 7 hrs.');
});

test('90 minutes left reads 1 hr and carries the promise when true', () => {
  assert.equal(fade(90 * minute, true), 'Fades in 1 hr unless confirmed again.');
});

test('40 minutes left carries the promise when true', () => {
  assert.equal(fade(40 * minute - 5000, true), 'Fades in 40 min unless confirmed again.');
});

test('40 minutes left has no promise when false', () => {
  assert.equal(fade(40 * minute - 5000, false), 'Fades in 40 min.');
});

test('a report that already faded reads nothing, flag or not', () => {
  assert.equal(fade(-5 * minute, true), '');
  assert.equal(fade(-5 * minute, false), '');
});

test('a missing expires_at reads nothing', () => {
  assert.equal(Pinalert.formatFade({ confirm_extends: true }), '');
  assert.equal(Pinalert.formatFade({}), '');
});
