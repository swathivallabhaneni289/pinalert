// app_bar_back_test.mjs - runs the real web/static/js/app-bar.js in a Node sandbox
// and checks the back arrow of the phone top bar (Phase 3.4, plan 04 and, for
// the redirect case, plan 25, code review part 2 WR-05):
//
//   - when the person came from another page of this site, the arrow goes back
//     in history;
//   - a page reached through feed.js's location.replace (a link to a finished
//     report, marked from=link in its address) has a same origin referrer that
//     is no longer the previous history entry, so the arrow follows its own
//     address instead, and never leaves the site;
//   - a modified click keeps the link's own meaning.
//
// Run it with: node --test web/app_bar_back_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'app-bar.js'), 'utf8');

// load runs app-bar.js on a page with a back arrow. referrer, search and the
// history length are what the browser would report.
function load({ referrer = 'https://pinalert.example/', search = '', length = 3 } = {}) {
  let click = null;
  const backCalls = [];
  const link = { addEventListener(type, fn) { if (type === 'click') { click = fn; } } };
  const doc = {
    referrer,
    querySelector(selector) { return selector === 'a.app-topbar__back' ? link : null; },
    getElementById() { return null; },
    addEventListener() {},
    removeEventListener() {}
  };
  const sandbox = {
    document: doc,
    URL,
    URLSearchParams,
    window: null
  };
  sandbox.window = {
    location: { origin: 'https://pinalert.example', search, pathname: '/profile' },
    history: { length, back() { backCalls.push('back'); } }
  };
  sandbox.location = sandbox.window.location;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'app-bar.js' });
  const press = (extra = {}) => {
    const event = { defaultPrevented: false, button: 0, metaKey: false, ctrlKey: false, shiftKey: false, altKey: false, prevented: false, preventDefault() { this.prevented = true; }, ...extra };
    click(event);
    return event;
  };
  return { press, backCalls };
}

test('from another page of this site the arrow goes back in history', () => {
  const m = load();
  const event = m.press();
  assert.equal(event.prevented, true);
  assert.deepEqual(m.backCalls, ['back']);
});

test('with no referrer, or one from another site, or a single history entry, the arrow follows its own address', () => {
  for (const options of [{ referrer: '' }, { referrer: 'https://other.example/x' }, { length: 1 }]) {
    const m = load(options);
    const event = m.press();
    assert.equal(event.prevented, false, JSON.stringify(options));
    assert.deepEqual(m.backCalls, []);
  }
});

test('WR-05: after the redirect to a finished report the arrow follows its address even with a same origin referrer', () => {
  const m = load({ search: '?part=ended&report=9&from=link', referrer: 'https://pinalert.example/?report=9', length: 2 });
  const event = m.press();
  assert.equal(event.prevented, false, 'the link is followed, the browser does not go back to the other site');
  assert.deepEqual(m.backCalls, []);
});

test('WR-05: only the exact marker counts, any other value of from changes nothing', () => {
  for (const search of ['?part=ended&from=elsewhere', '?part=ended&from=', '?part=ended&xfrom=link', '?part=ended']) {
    const m = load({ search });
    m.press();
    assert.deepEqual(m.backCalls, ['back'], search);
  }
});

test('a modified click keeps the link\'s own meaning', () => {
  for (const extra of [{ metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }, { button: 1 }, { defaultPrevented: true }]) {
    const m = load();
    const event = m.press(extra);
    assert.equal(event.prevented, false);
    assert.deepEqual(m.backCalls, []);
  }
});
