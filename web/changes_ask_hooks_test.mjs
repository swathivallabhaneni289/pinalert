// changes_ask_hooks_test.mjs - runs the real web/static/js/votes.js in a Node
// sandbox and checks the one hook Phase 3.3 plan 20 adds to castVote (the
// 03.3-UAT gap for test 4, owner 2026-10-27): a confirm or a dispute the server
// saved offers the "Tell you when this report changes?" pop-up through
// window.PinalertChangesAsk.offer(). Mark resolved, Reopen and the take back do
// not (P-2), a refused vote or a network failure does not, and a pop-up that
// throws or is missing never changes what castVote returns.
//
// The report form half of the hook is in report_form_session_test.mjs.
//
// Run it with: node --test web/changes_ask_hooks_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.
// The fakes cover only what castVote calls.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'votes.js'), 'utf8');

const NAME = 'changes ask: ';
const LOCATION = { latitude: 12.97, longitude: 77.59 };
const settle = () => new Promise((resolve) => setImmediate(resolve));

// load evaluates votes.js on a small page. answer is what the fake server says:
// { status, body } or an Error for a network failure. changesAsk is the pop-up
// the page has: 'count' (a counting offer), 'throws', or 'none'.
function load({ answer = { status: 200, body: { id: 5 } }, changesAsk = 'count' } = {}) {
  const log = [];
  const sandbox = {
    document: { dispatchEvent(e) { log.push('refresh:' + e.type); } },
    navigator: {},
    sessionStorage: { getItem: () => null, setItem() {}, removeItem() {} },
    localStorage: { getItem: () => null, setItem() {}, removeItem() {} },
    CustomEvent: class CustomEvent { constructor(type) { this.type = type; } },
    setTimeout() { return 1; },
    clearTimeout() {},
    setInterval() { return 1; },
    clearInterval() {},
    addEventListener() {},
    Pinalert: { formatWait: (s) => s + ' seconds', setText() {}, serverNow: () => Date.now() },
    fetch(target, init) {
      log.push('fetch:' + String(target).split('/').pop());
      if (answer instanceof Error) { return Promise.reject(answer); }
      return Promise.resolve({
        ok: answer.status < 400,
        status: answer.status,
        headers: { get: () => null },
        json: () => Promise.resolve(answer.body)
      });
    },
    Promise, Date, Math, JSON, Array, Object, String, Number, isFinite, isNaN, parseFloat, parseInt, encodeURIComponent
  };
  sandbox.window = sandbox;
  let offers = 0;
  if (changesAsk === 'count') {
    sandbox.PinalertChangesAsk = { offer() { offers += 1; log.push('offer'); return Promise.resolve(); } };
  } else if (changesAsk === 'throws') {
    sandbox.PinalertChangesAsk = { offer() { offers += 1; log.push('offer'); throw new Error('pop-up broke'); } };
  }
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return { votes: sandbox.PinalertVotes, log, offers: () => offers };
}

test(NAME + 'a saved confirm offers the pop-up once, after the vote is saved, and returns the body', async () => {
  const m = load();
  const body = await m.votes.castVote(5, 'confirm', { location: LOCATION });
  assert.deepEqual(body, { id: 5 });
  assert.equal(m.offers(), 1);
  assert.deepEqual(m.log, ['fetch:confirm', 'refresh:pinalert:updates-refresh', 'offer']);
});

test(NAME + 'a saved dispute offers the pop-up once and returns the body', async () => {
  const m = load();
  const body = await m.votes.castVote(5, 'dispute', { location: LOCATION });
  assert.deepEqual(body, { id: 5 });
  assert.equal(m.offers(), 1);
});

test(NAME + 'resolve, reopen and take back never offer the pop-up', async () => {
  for (const action of ['resolve', 'reopen', 'unresolve']) {
    const m = load();
    const body = await m.votes.castVote(5, action, { location: LOCATION });
    assert.deepEqual(body, { id: 5 }, action);
    assert.equal(m.offers(), 0, action);
  }
});

test(NAME + 'a refused vote or a network failure never offers the pop-up', async () => {
  const refusals = [
    { status: 400, body: { error: { field: 'body', message: 'Bad vote.' } } },
    { status: 403, body: { error: { field: 'body', message: 'Not allowed.' } } },
    { status: 429, body: { error: { field: 'rate', message: 'Slow down.' }, retry_after_seconds: 30 } },
    new Error('offline')
  ];
  for (const answer of refusals) {
    const m = load({ answer });
    await assert.rejects(m.votes.castVote(5, 'confirm', { location: LOCATION }));
    await settle();
    assert.equal(m.offers(), 0, answer instanceof Error ? 'network failure' : String(answer.status));
  }
});

test(NAME + 'a pop-up that throws never changes what castVote returns', async () => {
  const m = load({ changesAsk: 'throws' });
  const body = await m.votes.castVote(5, 'confirm', { location: LOCATION });
  assert.deepEqual(body, { id: 5 });
  assert.equal(m.offers(), 1, 'it was offered');
});

test(NAME + 'a page without the pop-up script still saves the vote', async () => {
  const m = load({ changesAsk: 'none' });
  const body = await m.votes.castVote(5, 'dispute', { location: LOCATION });
  assert.deepEqual(body, { id: 5 });
  assert.equal(m.offers(), 0);
});
