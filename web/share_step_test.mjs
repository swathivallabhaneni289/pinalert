// share_step_test.mjs - runs the real web/static/js/notify.js in a Node sandbox
// and checks the step 1 picker of the iPhone install guide (quick 261117-kqy,
// 7.1 UAT test 7): shareStep reads a user agent and answers 'menu' (iPhone
// Safari 26 or newer, Share is in the Menu, the three dots), 'bar' (older
// iPhone Safari, Share is in the bottom bar) or 'plain' (everything else), and
// applyShareStep shows only the matching icon and wording of the real
// device_guide.html.tmpl step 1.
//
// Run it with: node --test web/share_step_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs). It is not
// embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'notify.js'), 'utf8');
const guide = fs.readFileSync(path.join(here, 'templates', 'device_guide.html.tmpl'), 'utf8');

// Real agent strings. Safari 26 freezes the OS number at 18_6 or 18_7
// (WebKit bug 298473), so only Version/N tells 26 from 18.
const IOS26 = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1';
const IOS26_1 = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.1 Mobile/15E148 Safari/604.1';
const IOS27 = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/27.0 Mobile/15E148 Safari/604.1';
const IOS18_5 = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1';
const IOS17 = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1';
const CHROME_IOS = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/130.0.6723.90 Mobile/15E148 Safari/604.1';
const FIREFOX_IOS = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/132.0 Mobile/15E148 Safari/605.1.15';
// Edge on iOS carries a Version/ token, so only the EdgiOS token keeps it plain.
const EDGE_IOS = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 EdgiOS/130.0.2849.80 Mobile/15E148 Safari/605.1.15';
const IN_APP = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148';
const IPAD26 = 'Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1';
const MAC = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15';

// loadNotify evaluates notify.js on a page that is not a secure context and has
// no document, so the load time register and reconnect do nothing. The
// navigator object stays the test's, so a test can change the agent.
function loadNotify(navigator) {
  const window = {};
  const sandbox = { window, navigator, Promise, JSON, Uint8Array, Error, Date };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  return sandbox.window.PinalertNotify;
}

test('shareStep answers menu, bar or plain from real user agents', () => {
  const notify = loadNotify({ userAgent: '' });
  const cases = [
    ['iOS 26 Safari, OS frozen at 18_6', IOS26, 5, 'menu'],
    ['iOS 26.1 Safari, OS frozen at 18_7', IOS26_1, 5, 'menu'],
    ['Safari 27', IOS27, 5, 'menu'],
    ['iOS 18.5 Safari', IOS18_5, 5, 'bar'],
    ['iOS 17 Safari', IOS17, 5, 'bar'],
    ['Chrome on iOS', CHROME_IOS, 5, 'plain'],
    ['Firefox on iOS', FIREFOX_IOS, 5, 'plain'],
    ['Edge on iOS, with a Version/ token', EDGE_IOS, 5, 'plain'],
    ['an in-app web view with no Version/', IN_APP, 5, 'plain'],
    ['iPad Safari 26', IPAD26, 5, 'plain'],
    ['a desktop mode iPad', MAC, 5, 'plain'],
    ['a Mac', MAC, 0, 'plain'],
    ['an empty agent', '', 0, 'plain'],
    ['a missing agent', undefined, undefined, 'plain'],
    ['a null agent', null, 0, 'plain']
  ];
  for (const [name, agent, touch, want] of cases) {
    assert.equal(notify.shareStep(agent, touch), want, name);
  }
});

test('every other iOS browser token keeps Safari 26 plain, even with Version/', () => {
  const notify = loadNotify({ userAgent: '' });
  for (const token of ['CriOS/130.0', 'FxiOS/132.0', 'EdgiOS/130.0', 'OPiOS/16.0', 'OPT/5.1', 'YaBrowser/24.1',
    'DuckDuckGo/7', 'GSA/340.0', 'Brave/1.70', 'Focus/132.0']) {
    const agent = IOS26.replace('Mobile/15E148', token + ' Mobile/15E148');
    assert.equal(notify.shareStep(agent, 5), 'plain', token);
  }
});

// fakeGuide builds a guide root from the real template's step 1: one node per
// data-share-for attribute, hidden as the markup says, and the step element
// that carries data-share-step.
function fakeGuide() {
  const li = /<li class="device-guide__step" data-share-step="([a-z]+)">(.*?)<\/li>/s.exec(guide);
  assert.ok(li, 'device_guide.html.tmpl has no step 1 carrying data-share-step');
  const parts = [];
  for (const m of li[2].matchAll(/<span([^>]*)\sdata-share-for="([a-z ]+)"([^>]*)>/g)) {
    const attrs = m[1] + ' ' + m[3];
    parts.push({
      kind: /class="app-icon /.test(attrs) ? 'icon' : 'text',
      icon: (/app-icon--([a-z-]+)/.exec(attrs) || [])[1] || '',
      forAttr: m[2],
      hidden: /(^|\s)hidden(\s|$)/.test(m[3]),
      getAttribute(name) { return name === 'data-share-for' ? this.forAttr : null; }
    });
  }
  const step = {
    attrs: { 'data-share-step': li[1] },
    setAttribute(name, value) { this.attrs[name] = String(value); },
    getAttribute(name) { return this.attrs[name] ?? null; },
    querySelectorAll(sel) { return sel === '[data-share-for]' ? parts : []; }
  };
  const root = { querySelector: (sel) => (sel === '[data-share-step]' ? step : null) };
  return { root, step, parts };
}

function visible(parts) {
  return parts.filter((p) => !p.hidden).map((p) => (p.kind === 'icon' ? 'icon:' + p.icon : 'text:' + p.forAttr));
}

test('the markup without the script shows the plain step only', () => {
  const { step, parts } = fakeGuide();
  assert.equal(step.getAttribute('data-share-step'), 'plain');
  assert.deepEqual(visible(parts), ['icon:share', 'text:plain']);
  assert.equal(parts.length, 5, 'two icons and three wordings');
});

test('applyShareStep shows only the matching icon and wording', () => {
  const nav = { userAgent: '', maxTouchPoints: 5 };
  const notify = loadNotify(nav);
  const cases = [
    [IOS26, 'menu', ['icon:ellipsis', 'text:menu']],
    [IOS18_5, 'bar', ['icon:share', 'text:bar']],
    [CHROME_IOS, 'plain', ['icon:share', 'text:plain']]
  ];
  for (const [agent, want, shown] of cases) {
    nav.userAgent = agent;
    const { root, step, parts } = fakeGuide();
    assert.equal(notify.applyShareStep(root), want);
    assert.equal(step.getAttribute('data-share-step'), want);
    assert.deepEqual(visible(parts), shown, want);
  }
  // Applied twice on the same root, the second answer wins and nothing else stays.
  const { root, parts } = fakeGuide();
  nav.userAgent = IOS26;
  notify.applyShareStep(root);
  nav.userAgent = IOS17;
  notify.applyShareStep(root);
  assert.deepEqual(visible(parts), ['icon:share', 'text:bar']);
});

test('applyShareStep on a root without step 1, or no root, changes nothing', () => {
  const notify = loadNotify({ userAgent: IOS26 });
  assert.equal(notify.applyShareStep(null), '');
  assert.equal(notify.applyShareStep({}), '');
  assert.equal(notify.applyShareStep({ querySelector: () => null }), '');
});
