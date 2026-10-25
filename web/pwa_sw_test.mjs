// pwa_sw_test.mjs - runs the real web/static/js/sw.js in a Node sandbox and
// checks the service worker of Phase 3.3 (plan 04): one notification for every
// push, built with no noise fields, a tap that opens only a report link, and a
// new subscription sent to the server when the browser replaces the old one.
//
// Run it with: node --test web/pwa_sw_test.mjs
// CI runs it with the other web/*.mjs files (node --test web/*.mjs in
// .github/workflows/ci.yml). It is not embedded in the server.

import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import url from 'node:url';
import vm from 'node:vm';

const here = path.dirname(url.fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, 'static', 'js', 'sw.js'), 'utf8');

const ORIGIN = 'https://pinalert.example';

// loadWorker evaluates sw.js against a fake worker scope. It returns the scope
// (the sandbox, which is also self), the recorded listeners and the calls the
// fakes saw.
function loadWorker(opts = {}) {
  const listeners = {};
  const calls = {
    shown: [],
    opened: [],
    focused: [],
    navigated: [],
    fetched: [],
    subscribed: [],
    skipWaiting: 0,
    claim: 0
  };
  const windowClients = opts.windowClients || [];
  const sandbox = {
    addEventListener: (name, fn) => {
      listeners[name] = listeners[name] || [];
      listeners[name].push(fn);
    },
    registration: {
      showNotification: (title, options) => {
        calls.shown.push({ title, options });
        return Promise.resolve();
      },
      pushManager: {
        subscribe: (options) => {
          calls.subscribed.push(options);
          return Promise.resolve({ toJSON: () => ({ endpoint: 'https://push.example/new', keys: { p256dh: 'p', auth: 'a' } }) });
        }
      }
    },
    skipWaiting: () => { calls.skipWaiting++; return Promise.resolve(); },
    clients: {
      claim: () => { calls.claim++; return Promise.resolve(); },
      matchAll: () => Promise.resolve(windowClients.map((c) => ({
        url: c.url,
        focus() { calls.focused.push(c.url); return Promise.resolve(this); },
        navigate(target) {
          calls.navigated.push(target);
          return c.navigateFails ? Promise.reject(new Error('refused')) : Promise.resolve(this);
        }
      }))),
      openWindow: (target) => { calls.opened.push(target); return Promise.resolve({}); }
    },
    location: { origin: ORIGIN },
    fetch: (target, init) => {
      calls.fetched.push({ target, init });
      if (target === '/api/me/push/status') {
        // opts.status: a registered answer (default), 'not-registered', 'http-error', 'bad-json' or 'network'.
        const mode = opts.status || 'registered';
        if (mode === 'network') return Promise.reject(new Error('offline'));
        if (mode === 'http-error') return Promise.resolve({ ok: false, status: 503, json: () => Promise.resolve({}) });
        if (mode === 'bad-json') return Promise.resolve({ ok: true, status: 200, json: () => Promise.reject(new Error('not json')) });
        const registered = mode === 'registered';
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ registered }) });
      }
      return Promise.resolve({ ok: true, status: 200 });
    },
    console
  };
  sandbox.self = sandbox;
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox, { filename: 'sw.js' });
  return { sandbox, listeners, calls };
}

// dispatch fires one listener with a fake event and returns the promises the
// listener handed to waitUntil.
function dispatch(worker, name, extra = {}) {
  const waits = [];
  const event = Object.assign({ waitUntil: (p) => { waits.push(p); } }, extra);
  for (const fn of worker.listeners[name] || []) fn(event);
  return { event, waits };
}

function pushEvent(payload) {
  return { data: { json: () => (typeof payload === 'string' ? JSON.parse(payload) : payload) } };
}

const GOOD = {
  v: 1,
  kind: 'critical',
  tag: 'report-12',
  title: 'Fire',
  body: '2 km from your home area. Not confirmed yet.',
  url: '/?report=12'
};

test('registers exactly five listeners on self', () => {
  const w = loadWorker();
  assert.deepEqual(Object.keys(w.listeners).sort(), ['activate', 'install', 'notificationclick', 'push', 'pushsubscriptionchange']);
  for (const name of Object.keys(w.listeners)) assert.equal(w.listeners[name].length, 1, name);
});

test('has no fetch handler and holds no cache', () => {
  const w = loadWorker();
  assert.equal(w.listeners.fetch, undefined);
  assert.equal(/\bcaches\b/.test(source.replace(/\/\*[\s\S]*?\*\/|\/\/.*$/gm, '')), false);
});

test('a good push shows one silent notification with exactly the eight option keys', async () => {
  const w = loadWorker();
  const { waits } = dispatch(w, 'push', pushEvent(GOOD));
  assert.equal(waits.length, 1);
  await Promise.all(waits);
  assert.equal(w.calls.shown.length, 1);
  const { title, options } = w.calls.shown[0];
  assert.equal(title, 'Fire');
  assert.deepEqual(Object.keys(options).sort(), ['badge', 'body', 'data', 'icon', 'lang', 'renotify', 'silent', 'tag']);
  assert.equal(options.silent, true);
  assert.equal(options.renotify, false);
  assert.equal(options.tag, 'report-12');
  assert.equal(options.icon, '/icons/app-192.png');
  assert.equal(options.badge, '/icons/badge-96.png');
  assert.equal(options.lang, 'en');
  assert.equal(options.body, GOOD.body);
  assert.equal(options.data.url, '/?report=12');
  assert.equal('vibrate' in options, false);
});

test('the file names the three constants and holds no vibrate token outside comments', () => {
  const code = source.replace(/\/\*[\s\S]*?\*\/|\/\/.*$/gm, '');
  assert.equal(/vibrate/i.test(code), false);
  const w = loadWorker();
  assert.equal(vm.runInContext('SUBSCRIPTION_URL', w.sandbox), '/api/me/push/subscription');
  assert.equal(vm.runInContext('ICON_URL', w.sandbox), '/icons/app-192.png');
  assert.equal(vm.runInContext('BADGE_URL', w.sandbox), '/icons/badge-96.png');
});

test('a push with no data, bad JSON or a missing title or body still shows the plain notice', async () => {
  const cases = {
    'no data': {},
    'null data': { data: null },
    'not json': { data: { json: () => { throw new SyntaxError('bad'); } } },
    'json null': pushEvent('null'),
    'json number': pushEvent('5'),
    'missing title': pushEvent({ body: 'x', url: '/?report=1' }),
    'missing body': pushEvent({ title: 'x', url: '/?report=1' }),
    'title not a string': pushEvent({ title: 3, body: 'x' })
  };
  for (const [name, ev] of Object.entries(cases)) {
    const w = loadWorker();
    const { waits } = dispatch(w, 'push', ev);
    await Promise.all(waits);
    assert.equal(w.calls.shown.length, 1, name);
    const { title, options } = w.calls.shown[0];
    assert.equal(title, 'Pinalert', name);
    assert.equal(options.body, 'Open Pinalert for a new alert near your home area.', name);
    assert.equal(options.tag, 'pinalert', name);
    assert.equal(options.data.url, '/', name);
    assert.equal(options.silent, true, name);
  }
});

test('safeTarget accepts only /?report= followed by 1 to 18 digits', () => {
  const w = loadWorker();
  const safe = (u) => vm.runInContext('safeTarget', w.sandbox)(u);
  assert.equal(safe('/?report=12'), '/?report=12');
  assert.equal(safe('/?report=123456789012345678'), '/?report=123456789012345678');
  const refused = [
    'https://evil.example/?report=1',
    '/?report=1&x=2',
    '/?report=abc',
    'javascript:alert(1)',
    '/?report=1234567890123456789',
    '/?report=',
    '//evil.example/?report=1',
    '',
    undefined,
    null
  ];
  for (const u of refused) assert.equal(safe(u), '/', String(u));
});

test('a refused url in a push becomes / in the notification data', async () => {
  for (const bad of ['https://evil.example/?report=1', '/?report=1&x=2', '/?report=abc', 'javascript:alert(1)', '/?report=1234567890123456789']) {
    const w = loadWorker();
    const { waits } = dispatch(w, 'push', pushEvent(Object.assign({}, GOOD, { url: bad })));
    await Promise.all(waits);
    assert.equal(w.calls.shown[0].options.data.url, '/', bad);
  }
});

test('a tap closes the notification and focuses then navigates an open same origin window', async () => {
  const w = loadWorker({ windowClients: [{ url: ORIGIN + '/settings' }] });
  let closed = 0;
  const { waits } = dispatch(w, 'notificationclick', {
    notification: { close: () => { closed++; }, data: { url: '/?report=12' } }
  });
  assert.equal(waits.length, 1);
  await Promise.all(waits);
  assert.equal(closed, 1);
  assert.deepEqual([...w.calls.focused], [ORIGIN + '/settings']);
  assert.deepEqual([...w.calls.navigated], ['/?report=12']);
  assert.deepEqual([...w.calls.opened], []);
});

test('a tap ignores a window of another origin and opens a new window', async () => {
  const w = loadWorker({ windowClients: [{ url: 'https://other.example/' }] });
  const { waits } = dispatch(w, 'notificationclick', {
    notification: { close: () => {}, data: { url: '/?report=7' } }
  });
  await Promise.all(waits);
  assert.deepEqual([...w.calls.focused], []);
  assert.deepEqual([...w.calls.opened], ['/?report=7']);
});

test('a tap with no open window opens one, and an unsafe url opens /', async () => {
  const w = loadWorker();
  let closed = 0;
  const { waits } = dispatch(w, 'notificationclick', {
    notification: { close: () => { closed++; }, data: { url: 'https://evil.example/?report=1' } }
  });
  await Promise.all(waits);
  assert.equal(closed, 1);
  assert.deepEqual([...w.calls.opened], ['/']);

  const bare = loadWorker();
  const second = dispatch(bare, 'notificationclick', { notification: { close: () => {} } });
  await Promise.all(second.waits);
  assert.deepEqual([...bare.calls.opened], ['/']);
});

test('a tap falls back to a new window when the open one cannot be navigated', async () => {
  const w = loadWorker({ windowClients: [{ url: ORIGIN + '/', navigateFails: true }] });
  const { waits } = dispatch(w, 'notificationclick', {
    notification: { close: () => {}, data: { url: '/?report=3' } }
  });
  await Promise.all(waits);
  assert.deepEqual([...w.calls.opened], ['/?report=3']);
});

const OLD_ENDPOINT = 'https://push.example/old';

function changeEvent(key) {
  return { oldSubscription: { endpoint: OLD_ENDPOINT, options: { applicationServerKey: key } } };
}

test('pushsubscriptionchange asks the server about the old endpoint, then subscribes again and posts the new one when it is registered', async () => {
  const w = loadWorker();
  const key = new Uint8Array([1, 2, 3]);
  const { waits } = dispatch(w, 'pushsubscriptionchange', changeEvent(key));
  assert.equal(waits.length, 1);
  await Promise.all(waits);
  assert.equal(w.calls.subscribed.length, 1);
  assert.equal(w.calls.subscribed[0].userVisibleOnly, true);
  assert.equal(w.calls.subscribed[0].applicationServerKey, key);
  assert.equal(w.calls.fetched.length, 2);
  const status = w.calls.fetched[0];
  assert.equal(status.target, '/api/me/push/status');
  assert.equal(status.init.method, 'POST');
  assert.equal(status.init.credentials, 'same-origin');
  assert.equal(status.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(status.init.body), { endpoint: OLD_ENDPOINT });
  const { target, init } = w.calls.fetched[1];
  assert.equal(target, '/api/me/push/subscription');
  assert.equal(init.method, 'POST');
  assert.equal(init.credentials, 'same-origin');
  assert.equal(init.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(init.body), { endpoint: 'https://push.example/new', keys: { p256dh: 'p', auth: 'a' } });
});

for (const mode of ['not-registered', 'http-error', 'bad-json', 'network']) {
  test('pushsubscriptionchange posts nothing when the status answer is ' + mode + ' (shared device, B-01)', async () => {
    const w = loadWorker({ status: mode });
    const { waits } = dispatch(w, 'pushsubscriptionchange', changeEvent(new Uint8Array([1, 2, 3])));
    await Promise.all(waits);
    assert.equal(w.calls.subscribed.length, 0);
    assert.equal(w.calls.fetched.filter((f) => f.target === '/api/me/push/subscription').length, 0);
    assert.equal(w.calls.fetched.length, 1);
    assert.equal(w.calls.fetched[0].target, '/api/me/push/status');
  });
}

test('pushsubscriptionchange with an old subscription that has no endpoint asks and posts nothing', async () => {
  const w = loadWorker();
  const { waits } = dispatch(w, 'pushsubscriptionchange', {
    oldSubscription: { options: { applicationServerKey: new Uint8Array([1]) } }
  });
  await Promise.all(waits);
  assert.equal(w.calls.subscribed.length, 0);
  assert.equal(w.calls.fetched.length, 0);
});

test('pushsubscriptionchange with no old subscription does nothing', async () => {
  const w = loadWorker();
  const { waits } = dispatch(w, 'pushsubscriptionchange', {});
  await Promise.all(waits);
  assert.equal(w.calls.subscribed.length, 0);
  assert.equal(w.calls.fetched.length, 0);
});

test('install skips waiting and activate claims clients, both inside waitUntil', async () => {
  const w = loadWorker();
  const inst = dispatch(w, 'install');
  assert.equal(inst.waits.length, 1);
  await Promise.all(inst.waits);
  assert.equal(w.calls.skipWaiting, 1);
  const act = dispatch(w, 'activate');
  assert.equal(act.waits.length, 1);
  await Promise.all(act.waits);
  assert.equal(w.calls.claim, 1);
});
