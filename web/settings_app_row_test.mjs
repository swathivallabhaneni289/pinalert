// settings_app_row_test.mjs - the first About Pinalert row of Settings (quick
// 261118-srq, owner, 7.1 UAT test 8):
//
//   - notify.js installRow and installRowHere answer 'dock' for Safari 17 or
//     newer in a Mac tab, 'install' where the browser offered a one click
//     install, and '' everywhere else (installed apps, iPhones, iPads in
//     desktop mode, older Safari, Firefox, Chrome and Edge with no offer);
//   - install-app.js keeps the browser's offer, shows it once per tap, and
//     says done on an accepted choice or appinstalled, not on a dismissed one.
//
// The row and its window inside the Settings script are tested in
// settings_alerts_test.mjs, next to the other Settings windows.
//
// Run it with: node --test web/settings_app_row_test.mjs
// CI runs it with the other web/*.mjs files. It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const notifySource = fs.readFileSync(path.join(here, 'static', 'js', 'notify.js'), 'utf8');
const installSource = fs.readFileSync(path.join(here, 'static', 'js', 'install-app.js'), 'utf8');

const MAC = (version) => 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/' + version + ' Safari/605.1.15';
const MAC_CHROME = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36';
const MAC_EDGE = MAC_CHROME + ' Edg/131.0.0.0';
const MAC_FIREFOX = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0';
const MAC_DDG = MAC('17.6') + ' Ddg/17.6';
const WIN_CHROME = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36';
const ANDROID_CHROME = 'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36';
const IPHONE = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1';
const IPAD = 'Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1';

// loadNotify evaluates notify.js on a page that is not a secure context and has
// no document, so the load time register and reconnect do nothing.
// standalone true makes isInstalled true, as an app from the Home Screen or Dock.
function loadNotify(navigator, { displayStandalone = false } = {}) {
  const window = {
    matchMedia: (query) => ({ matches: displayStandalone && query === '(display-mode: standalone)' })
  };
  const sandbox = { window, navigator, Promise, JSON, Uint8Array, Error, Date };
  vm.createContext(sandbox);
  vm.runInContext(notifySource, sandbox);
  return sandbox.window.PinalertNotify;
}

// ---- notify.js: which row ---------------------------------------------------

test('installRow answers dock, install or none across the browser matrix', () => {
  const notify = loadNotify({ userAgent: '' });
  const cases = [
    // [name, agent, touch points, installed, offer, want]
    ['Mac Safari 17 in a tab', MAC('17.6'), 0, false, false, 'dock'],
    ['Mac Safari 18 in a tab', MAC('18.5'), 0, false, false, 'dock'],
    ['Mac Safari 26 in a tab', MAC('26.0'), 0, false, false, 'dock'],
    ['Mac Safari 26 on a touch screen Mac reporting one point', MAC('26.0'), 1, false, false, 'dock'],
    ['Mac Safari 16 has no Add to Dock', MAC('16.6'), 0, false, false, ''],
    ['Mac Safari 26 already in the Dock', MAC('26.0'), 0, true, false, ''],
    ['iPad in desktop mode says Macintosh', MAC('26.0'), 5, false, false, ''],
    ['iPhone Safari', IPHONE, 5, false, false, ''],
    ['iPad Safari', IPAD, 5, false, false, ''],
    ['iPhone never gets Install app', IPHONE, 5, false, true, ''],
    ['Chrome on a Mac with no offer', MAC_CHROME, 0, false, false, ''],
    ['Chrome on a Mac with an offer', MAC_CHROME, 0, false, true, 'install'],
    ['Edge on a Mac with no offer', MAC_EDGE, 0, false, false, ''],
    ['Edge on a Mac with an offer', MAC_EDGE, 0, false, true, 'install'],
    ['Chrome on Windows with an offer', WIN_CHROME, 0, false, true, 'install'],
    ['Chrome on Android with an offer', ANDROID_CHROME, 5, false, true, 'install'],
    ['Chrome already installed', MAC_CHROME, 0, true, true, ''],
    ['Firefox on a Mac', MAC_FIREFOX, 0, false, false, ''],
    ['DuckDuckGo on a Mac carries Safari words', MAC_DDG, 0, false, false, ''],
    ['an empty agent', '', 0, false, false, ''],
    ['no agent at all', undefined, undefined, false, false, '']
  ];
  for (const [name, agent, touch, installed, offer, want] of cases) {
    assert.equal(notify.installRow(agent, touch, installed, offer), want, name);
  }
});

test('isMacSafari is Safari 17 or newer on a Mac only', () => {
  const notify = loadNotify({ userAgent: '' });
  assert.equal(notify.isMacSafari(MAC('17.0'), 0), true);
  assert.equal(notify.isMacSafari(MAC('16.6'), 0), false);
  assert.equal(notify.isMacSafari(MAC('26.0'), 5), false);
  assert.equal(notify.isMacSafari(MAC_CHROME, 0), false);
  assert.equal(notify.isMacSafari(MAC_EDGE, 0), false);
  assert.equal(notify.isMacSafari(MAC_FIREFOX, 0), false);
  assert.equal(notify.isMacSafari(IPHONE, 5), false);
});

test('installRowHere reads this browser: agent, touch points and installed', () => {
  assert.equal(loadNotify({ userAgent: MAC('26.0'), maxTouchPoints: 0 }).installRowHere(false), 'dock');
  assert.equal(loadNotify({ userAgent: MAC('26.0'), maxTouchPoints: 0 }, { displayStandalone: true }).installRowHere(false), '');
  assert.equal(loadNotify({ userAgent: MAC('26.0'), maxTouchPoints: 5 }).installRowHere(false), '');
  assert.equal(loadNotify({ userAgent: IPHONE, maxTouchPoints: 5, standalone: false }).installRowHere(true), '');
  assert.equal(loadNotify({ userAgent: MAC_CHROME, maxTouchPoints: 0 }).installRowHere(true), 'install');
  assert.equal(loadNotify({ userAgent: MAC_CHROME, maxTouchPoints: 0 }).installRowHere(false), '');
  assert.equal(loadNotify({ userAgent: MAC_CHROME, maxTouchPoints: 0 }, { displayStandalone: true }).installRowHere(true), '');
  assert.equal(loadNotify({ userAgent: MAC_FIREFOX, maxTouchPoints: 0 }).installRowHere(false), '');
  // Only a real true counts as an offer.
  assert.equal(loadNotify({ userAgent: MAC_CHROME, maxTouchPoints: 0 }).installRowHere('yes'), '');
});

// ---- install-app.js: the kept offer -----------------------------------------

function loadInstall() {
  const listeners = {};
  const window = {
    addEventListener(type, fn) { (listeners[type] = listeners[type] || []).push(fn); }
  };
  const sandbox = { window, Promise };
  vm.createContext(sandbox);
  vm.runInContext(installSource, sandbox, { filename: 'install-app.js' });
  const fire = (type, event) => (listeners[type] || []).forEach((fn) => fn(event));
  return { api: sandbox.window.PinalertInstall, listeners, fire };
}

// fakeOffer stands in for a BeforeInstallPromptEvent: prompt() counts, and
// userChoice settles when the test says.
function fakeOffer() {
  const offer = { prompts: 0, prevented: 0 };
  offer.userChoice = new Promise((resolve) => { offer.choose = (outcome) => resolve({ outcome, platform: 'web' }); });
  offer.prompt = () => { offer.prompts += 1; return Promise.resolve(); };
  offer.preventDefault = () => { offer.prevented += 1; };
  offer.type = 'beforeinstallprompt';
  return offer;
}

const flush = async () => { for (let i = 0; i < 6; i++) { await new Promise((r) => setImmediate(r)); } };

test('the script listens for the offer and for appinstalled, on the window', () => {
  const { listeners } = loadInstall();
  assert.equal((listeners.beforeinstallprompt || []).length, 1);
  assert.equal((listeners.appinstalled || []).length, 1);
});

test('an offer is kept, the browser bar is stopped, and watchers are told', () => {
  const { api, fire } = loadInstall();
  let told = 0;
  api.watch(() => { told += 1; });
  assert.equal(api.hasOffer(), false);
  const offer = fakeOffer();
  fire('beforeinstallprompt', offer);
  assert.equal(api.hasOffer(), true);
  assert.equal(offer.prevented, 1);
  assert.equal(told, 1);
});

test('an event with no prompt is not kept', () => {
  const { api, fire } = loadInstall();
  fire('beforeinstallprompt', { preventDefault() {} });
  assert.equal(api.hasOffer(), false);
});

test('prompt shows the offer once; a second tap while it shows does nothing', async () => {
  const { api, fire } = loadInstall();
  const offer = fakeOffer();
  fire('beforeinstallprompt', offer);
  const first = api.prompt();
  const second = api.prompt();
  assert.equal(offer.prompts, 1);
  assert.equal(api.isBusy(), true);
  assert.equal(await second, 'none');
  offer.choose('accepted');
  assert.equal(await first, 'accepted');
  assert.equal(offer.prompts, 1);
  assert.equal(api.isDone(), true);
  assert.equal(api.hasOffer(), false);
  assert.equal(api.isBusy(), false);
});

test('a dismissed choice is not done; the used offer is gone until the browser offers again', async () => {
  const { api, fire } = loadInstall();
  const offer = fakeOffer();
  fire('beforeinstallprompt', offer);
  const asked = api.prompt();
  offer.choose('dismissed');
  assert.equal(await asked, 'dismissed');
  assert.equal(api.isDone(), false);
  assert.equal(api.hasOffer(), false);
  assert.equal(await api.prompt(), 'none');
  assert.equal(offer.prompts, 1, 'a used offer is never shown again');
  const again = fakeOffer();
  fire('beforeinstallprompt', again);
  assert.equal(api.hasOffer(), true);
  const next = api.prompt();
  again.choose('accepted');
  assert.equal(await next, 'accepted');
  assert.equal(again.prompts, 1);
});

test('appinstalled marks it done and drops the offer', () => {
  const { api, fire } = loadInstall();
  let told = 0;
  api.watch(() => { told += 1; });
  fire('beforeinstallprompt', fakeOffer());
  fire('appinstalled', { type: 'appinstalled' });
  assert.equal(api.isDone(), true);
  assert.equal(api.hasOffer(), false);
  assert.equal(told, 2);
});

test('a prompt that throws resolves none and never rejects', async () => {
  const { api, fire } = loadInstall();
  const offer = fakeOffer();
  offer.prompt = () => { throw new Error('not allowed'); };
  fire('beforeinstallprompt', offer);
  assert.equal(await api.prompt(), 'none');
  assert.equal(api.isBusy(), false);
});

test('a watcher that throws stops no other watcher', async () => {
  const { api, fire } = loadInstall();
  const seen = [];
  api.watch(() => { throw new Error('boom'); });
  api.watch(() => { seen.push('second'); });
  fire('beforeinstallprompt', fakeOffer());
  await flush();
  assert.deepEqual(seen, ['second']);
});

test('the script writes nothing to the page, sends nothing and stores nothing', () => {
  const code = installSource.split('\n').filter((line) => !line.trim().startsWith('//')).join('\n');
  assert.doesNotMatch(code, /innerHTML|textContent|createElement|document\.|fetch\(|localStorage|sessionStorage/);
});
