// notify_isolation_test.mjs - runs the real web/static/js/app.js in a Node
// sandbox and checks WR-W04: a store listener that throws (for example the
// Low data list when flag.js did not load) can stop neither the listeners
// after it nor the list request, which must not turn into a failed request.
//
// Run it with: node --test web/notify_isolation_test.mjs
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
// toast, a fetch that answers with the given rows, and timers that only
// record what was scheduled.
function loadApp(rows) {
  const timers = [];
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
    setTimeout: (fn, ms) => { timers.push({ fn, ms }); return timers.length; },
    clearTimeout: () => {},
    setInterval: () => 0,
    clearInterval: () => {}
  };
  const fetch = () => Promise.resolve({
    ok: true,
    status: 200,
    headers: { get: () => null },
    json: () => Promise.resolve({ reports: rows })
  });
  const sandbox = {
    window,
    document,
    fetch,
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
  return { Pinalert: sandbox.window.Pinalert, timers };
}

test('a throwing listener stops neither later listeners nor the list request', async () => {
  const { Pinalert, timers } = loadApp([{ id: 1 }, { id: 2 }]);
  const seen = [];
  Pinalert.subscribe(() => { throw new Error('PinalertFlag is not defined'); });
  Pinalert.subscribe((state) => { seen.push({ status: state.status, count: state.reports.length }); });

  await Pinalert.fetchReports();

  const last = seen[seen.length - 1];
  assert.ok(last, 'the second listener was never called');
  assert.equal(last.status, 'ok', 'the second listener must see the applied answer');
  assert.equal(last.count, 2);
  assert.equal(Pinalert.state.status, 'ok', 'a listener error must not mark the request failed');
  assert.equal(Pinalert.state.error, null);
  assert.ok(seen.every((s) => s.status !== 'error'), 'no error state may be announced');

  // The error is thrown again later on a timer, so it still reaches the
  // browser's error log, and it is the listener's own error.
  const rethrow = timers.find((t) => t.ms === 0);
  assert.ok(rethrow, 'the listener error must be thrown again on a timer');
  assert.throws(() => rethrow.fn(), /PinalertFlag is not defined/);
});

test('a listener that throws first still lets every listener run on every notify', async () => {
  const { Pinalert } = loadApp([{ id: 7 }]);
  let calls = 0;
  Pinalert.subscribe(() => { throw new Error('boom'); });
  Pinalert.subscribe(() => { calls += 1; });
  Pinalert.subscribe(() => { throw new Error('boom again'); });
  Pinalert.subscribe(() => { calls += 1; });

  await Pinalert.fetchReports();
  const afterFirst = calls;
  assert.ok(afterFirst >= 2, 'both good listeners must run on the first notify');

  await Pinalert.fetchReports();
  assert.ok(calls >= afterFirst + 2, 'both good listeners must run again on the next notify');
});
