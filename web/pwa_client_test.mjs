// pwa_client_test.mjs - runs the real web/static/js/notify.js in a Node sandbox
// (Phase 3.3, plan 06). It checks the device state detector, the order of the
// browser prompt and the subscription calls, the server confirmed "on" state,
// and that the endpoint (a capability address) only ever travels in JSON bodies.
//
// Run it with: node --test web/pwa_client_test.mjs
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

const SUBSCRIPTION_URL = '/api/me/push/subscription';
const STATUS_URL = '/api/me/push/status';
const UNSUBSCRIBE_URL = '/api/me/push/unsubscribe';
const ENDPOINT = 'https://push.example.test/send/abc123';
const IPHONE_UA = 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1';
const MAC_UA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15';
const ANDROID_UA = 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Mobile Safari/537.36';

// A throwaway 65 byte public key (an uncompressed P-256 point shape: 0x04 and
// 64 more bytes). It is a test value made in memory, not a real key.
const KEY_BYTES = [4];
for (let i = 1; i < 65; i++) {
  KEY_BYTES.push((i * 7) % 256);
}
const KEY = Buffer.from(KEY_BYTES).toString('base64url');

// makeEnv builds a fake page. Options:
//   secure, userAgent, maxTouchPoints, standalone, displayStandalone, online,
//   serviceWorker (false removes it), pushManager (false removes it),
//   notification (false removes it), permission, nextPermission,
//   subscribed (an existing browser subscription), subscribedKey (the bytes it
//   was made with; omitted means it does not report a key), account (data-account-id),
//   pushKey (data-push-key on the page; omitted means the page has none),
//   stored (initial local storage), routes (url -> function(body) -> {status, body},
//   or a promise of one, so a test can hold an answer back),
//   manualTimers (true keeps every timer in env.timers until fireTimers runs it),
//   hang ('getSubscription', 'unsubscribe' or 'subscribe': that browser call
//   starts and never settles), nextPermission 'hang' (the prompt never answers).
function makeEnv(options = {}) {
  const o = {
    manualTimers: false,
    hang: '',
    secure: true,
    userAgent: ANDROID_UA,
    maxTouchPoints: 5,
    standalone: false,
    displayStandalone: false,
    online: true,
    serviceWorker: true,
    pushManager: true,
    notification: true,
    permission: 'default',
    nextPermission: 'granted',
    subscribed: false,
    account: '7',
    stored: {},
    routes: {},
    ...options
  };
  const env = {
    calls: [],
    registrations: [],
    requests: [],
    subscribeOptions: [],
    store: new Map(Object.entries(o.stored)),
    permission: o.permission,
    sub: null,
    listeners: [],
    timers: []
  };
  const never = () => new Promise(() => {});

  // keyOf turns bytes into the ArrayBuffer a real subscription reports in
  // options.applicationServerKey.
  function keyOf(bytes) {
    return Uint8Array.from(bytes).buffer;
  }

  function makeSub(keyBytesOfSub) {
    const sub = {
      endpoint: ENDPOINT,
      toJSON() {
        return { endpoint: ENDPOINT, expirationTime: null, keys: { p256dh: 'p256dh-value', auth: 'auth-value' } };
      },
      unsubscribe() {
        env.calls.push('unsubscribe');
        if (o.hang === 'unsubscribe') {
          return never();
        }
        env.sub = null;
        return Promise.resolve(true);
      }
    };
    if (keyBytesOfSub) {
      sub.options = { applicationServerKey: keyOf(keyBytesOfSub) };
    }
    return sub;
  }
  if (o.subscribed) {
    env.sub = makeSub(o.subscribedKey);
  }

  const registration = {
    pushManager: {
      subscribe(opts) {
        env.calls.push('subscribe');
        env.subscribeOptions.push(opts);
        if (o.hang === 'subscribe') {
          return never();
        }
        // Like Chrome and Firefox: a subscription made with another key makes
        // subscribe fail until it is dropped.
        if (env.sub && env.sub.options && env.sub.options.applicationServerKey) {
          const have = Array.from(new Uint8Array(env.sub.options.applicationServerKey));
          if (have.join(',') !== Array.from(opts.applicationServerKey).join(',')) {
            return Promise.reject(new Error('InvalidStateError: a subscription with a different key exists'));
          }
        }
        env.sub = makeSub(Array.from(opts.applicationServerKey));
        return Promise.resolve(env.sub);
      },
      getSubscription() {
        env.calls.push('getSubscription');
        if (o.hang === 'getSubscription') {
          return never();
        }
        return Promise.resolve(env.sub);
      }
    }
  };

  const navigator = {
    userAgent: o.userAgent,
    maxTouchPoints: o.maxTouchPoints,
    onLine: o.online
  };
  if (o.standalone) {
    navigator.standalone = true;
  }
  if (o.serviceWorker) {
    navigator.serviceWorker = {
      register(scriptUrl, opts) {
        env.registrations.push({ scriptUrl, opts });
        return Promise.resolve(registration);
      },
      get ready() {
        env.calls.push('ready');
        return Promise.resolve(registration);
      }
    };
  }

  const window = {
    isSecureContext: o.secure,
    setTimeout: o.manualTimers ? (fn, ms) => {
      env.timers.push({ fn, ms, live: true });
      return env.timers.length;
    } : setTimeout,
    clearTimeout: o.manualTimers ? (id) => {
      if (env.timers[id - 1]) {
        env.timers[id - 1].live = false;
      }
    } : clearTimeout,
    addEventListener: (name) => { env.listeners.push('window:' + name); },
    matchMedia: (query) => ({ matches: o.displayStandalone && query === '(display-mode: standalone)' }),
    localStorage: {
      getItem: (k) => (env.store.has(k) ? env.store.get(k) : null),
      setItem: (k, v) => { env.store.set(k, String(v)); },
      removeItem: (k) => { env.store.delete(k); }
    }
  };
  if (o.pushManager) {
    window.PushManager = function PushManager() {};
  }
  if (o.notification) {
    window.Notification = {
      get permission() {
        return env.permission;
      },
      requestPermission() {
        env.calls.push('requestPermission');
        if (o.nextPermission === 'hang') {
          return never();
        }
        return Promise.resolve(o.nextPermission).then((answer) => {
          env.permission = answer;
          return answer;
        });
      }
    };
  }

  const document = {
    addEventListener: (name) => { env.listeners.push('document:' + name); },
    querySelector: (selector) => {
      if (selector === '[data-account-id]' && o.account !== '') {
        return { getAttribute: (name) => (name === 'data-account-id' ? o.account : null) };
      }
      if (selector === '[data-push-key]' && typeof o.pushKey === 'string') {
        return { getAttribute: (name) => (name === 'data-push-key' ? o.pushKey : null) };
      }
      return null;
    }
  };

  async function fetchFake(address, init = {}) {
    env.calls.push('fetch');
    const body = init.body ? JSON.parse(init.body) : null;
    env.requests.push({ url: address, init, body });
    const route = o.routes[address];
    if (!route) {
      return { ok: false, status: 404, json: () => Promise.resolve({}) };
    }
    const answer = await route(body);
    if (answer instanceof Error) {
      throw answer;
    }
    return {
      ok: answer.status >= 200 && answer.status < 300,
      status: answer.status,
      json: () => (answer.body === undefined ? Promise.reject(new Error('no body')) : Promise.resolve(answer.body))
    };
  }

  const sandbox = {
    window,
    document,
    navigator,
    fetch: fetchFake,
    atob: (text) => atob(text),
    Promise,
    Uint8Array,
    JSON,
    Date,
    Error,
    AbortController
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);
  env.Notify = sandbox.window.PinalertNotify;
  return env;
}

const ok204 = () => ({ status: 204 });

// The endpoint is a capability address, so it must never be in any request
// address. Every case that makes a request ends with this check.
function assertEndpointOnlyInBodies(env) {
  for (const r of env.requests) {
    assert.ok(!r.url.includes('abc123') && !r.url.includes('push.example'), 'the endpoint must not be in ' + r.url);
    assert.equal(r.init.method, 'POST');
  }
}

test('registers /sw.js at load with scope / and no cache for the worker script', () => {
  const env = makeEnv();
  assert.equal(env.registrations.length, 1);
  assert.equal(env.registrations[0].scriptUrl, '/sw.js');
  assert.equal(env.registrations[0].opts.scope, '/');
  assert.equal(env.registrations[0].opts.updateViaCache, 'none');
  assert.ok(env.Notify, 'window.PinalertNotify must exist');
  assert.equal(env.Notify.SW_URL, '/sw.js');
  assert.equal(env.Notify.INTENT_KEY, 'pinalert.pushIntent');
});

test('without a service worker nothing throws and nothing is registered', () => {
  const env = makeEnv({ serviceWorker: false });
  assert.equal(env.registrations.length, 0);
  assert.equal(env.Notify.baseState(), 'unsupported');
  const insecure = makeEnv({ secure: false });
  assert.equal(insecure.registrations.length, 0, 'a page that is not a secure context registers nothing');
});

test('adds no listener on window or document', () => {
  const env = makeEnv();
  assert.deepEqual(env.listeners, []);
});

test('baseState: a missing PushManager or Notification on a non Apple device is unsupported', () => {
  assert.equal(makeEnv({ pushManager: false }).Notify.baseState(), 'unsupported');
  assert.equal(makeEnv({ notification: false }).Notify.baseState(), 'unsupported');
  assert.equal(makeEnv({ secure: false }).Notify.baseState(), 'unsupported');
  assert.equal(makeEnv().Notify.baseState(), 'maybe');
});

test('baseState: an iPhone that is not an installed app needs the install even without PushManager', () => {
  const env = makeEnv({ userAgent: IPHONE_UA, pushManager: false });
  assert.equal(env.Notify.baseState(), 'needs-install');
  assert.equal(env.Notify.isAppleMobile(), true);
  assert.equal(env.Notify.isInstalled(), false);
});

test('baseState: an iPad that says Macintosh with touch points needs the install, a Mac without touch does not', () => {
  const ipad = makeEnv({ userAgent: MAC_UA, maxTouchPoints: 5, pushManager: false });
  assert.equal(ipad.Notify.baseState(), 'needs-install');
  const mac = makeEnv({ userAgent: MAC_UA, maxTouchPoints: 0 });
  assert.equal(mac.Notify.isAppleMobile(), false);
  assert.equal(mac.Notify.baseState(), 'maybe');
  const oneTouch = makeEnv({ userAgent: MAC_UA, maxTouchPoints: 1 });
  assert.equal(oneTouch.Notify.isAppleMobile(), false, 'one touch point is not more than one');
});

test('baseState: an iPhone running as an installed app is judged by its features', () => {
  const byNavigator = makeEnv({ userAgent: IPHONE_UA, standalone: true });
  assert.equal(byNavigator.Notify.isInstalled(), true);
  assert.equal(byNavigator.Notify.baseState(), 'maybe');
  const byDisplayMode = makeEnv({ userAgent: IPHONE_UA, displayStandalone: true });
  assert.equal(byDisplayMode.Notify.baseState(), 'maybe');
  const installedWithoutPush = makeEnv({ userAgent: IPHONE_UA, standalone: true, pushManager: false });
  assert.equal(installedWithoutPush.Notify.baseState(), 'unsupported');
});

test('baseState: a refused permission is blocked', () => {
  assert.equal(makeEnv({ permission: 'denied' }).Notify.baseState(), 'blocked');
});

test('deviceState: an empty key is unsupported and asks nothing', async () => {
  const env = makeEnv({ permission: 'granted', subscribed: true });
  assert.equal(await env.Notify.deviceState(''), 'unsupported');
  assert.equal(await env.Notify.deviceState(undefined), 'unsupported');
  assert.equal(env.requests.length, 0);
});

test('deviceState: on only when the server says this endpoint is registered for this account', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: true } }) }
  });
  assert.equal(await env.Notify.deviceState(KEY), 'on');
  assert.equal(env.requests.length, 1);
  assert.equal(env.requests[0].url, STATUS_URL);
  assert.deepEqual(env.requests[0].body, { endpoint: ENDPOINT });
  assert.equal(env.requests[0].init.credentials, 'same-origin');
  assert.equal(env.requests[0].init.headers['Content-Type'], 'application/json');
  assertEndpointOnlyInBodies(env);
});

test('deviceState: a subscription the server does not know for this account is ready, never on', async () => {
  const notRegistered = makeEnv({
    permission: 'granted',
    subscribed: true,
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: false } }) }
  });
  assert.equal(await notRegistered.Notify.deviceState(KEY), 'ready');
  const noSubscription = makeEnv({ permission: 'granted', subscribed: false });
  assert.equal(await noSubscription.Notify.deviceState(KEY), 'ready');
  assert.equal(noSubscription.requests.length, 0, 'no subscription, nothing to ask the server');
  const notAsked = makeEnv({ permission: 'default', subscribed: true });
  assert.equal(await notAsked.Notify.deviceState(KEY), 'ready');
  assert.equal(notAsked.requests.length, 0);
});

test('deviceState: a status call that fails gives ready', async () => {
  const failing = makeEnv({
    permission: 'granted',
    subscribed: true,
    routes: { [STATUS_URL]: () => ({ status: 503, body: {} }) }
  });
  assert.equal(await failing.Notify.deviceState(KEY), 'ready');
  const unauthorised = makeEnv({
    permission: 'granted',
    subscribed: true,
    routes: { [STATUS_URL]: () => ({ status: 401, body: {} }) }
  });
  assert.equal(await unauthorised.Notify.deviceState(KEY), 'ready');
  const rejecting = makeEnv({
    permission: 'granted',
    subscribed: true,
    routes: { [STATUS_URL]: () => new Error('network down') }
  });
  assert.equal(await rejecting.Notify.deviceState(KEY), 'ready');
});

test('deviceState: base states come back without any request', async () => {
  const blocked = makeEnv({ permission: 'denied', subscribed: true });
  assert.equal(await blocked.Notify.deviceState(KEY), 'blocked');
  const iphone = makeEnv({ userAgent: IPHONE_UA, pushManager: false });
  assert.equal(await iphone.Notify.deviceState(KEY), 'needs-install');
  assert.equal(blocked.requests.length + iphone.requests.length, 0);
});

test('enable: asks the browser first, in the same tick, before anything is awaited', async () => {
  const env = makeEnv({
    permission: 'default',
    nextPermission: 'granted',
    routes: { [SUBSCRIPTION_URL]: ok204 }
  });
  const pending = env.Notify.enable(KEY);
  assert.deepEqual(env.calls, ['requestPermission'],
    'requestPermission must be the only call made before enable returns');
  const result = await pending;
  assert.equal(result.ok, true);
  assert.equal(env.calls.indexOf('requestPermission'), 0);
  assert.ok(env.calls.indexOf('subscribe') > env.calls.indexOf('requestPermission'));
});

test('enable: a refusal is blocked, posts nothing and does not subscribe', async () => {
  const env = makeEnv({ permission: 'default', nextPermission: 'denied', routes: { [SUBSCRIPTION_URL]: ok204 } });
  const result = await env.Notify.enable(KEY);
  assert.deepEqual({ ok: result.ok, state: result.state, reason: result.reason },
    { ok: false, state: 'blocked', reason: 'refused' });
  assert.equal(env.requests.length, 0);
  assert.ok(!env.calls.includes('subscribe'));
  assert.equal(env.store.has('pinalert.pushIntent'), false);
});

test('enable: a dismissed question is refused but not blocked', async () => {
  const env = makeEnv({ permission: 'default', nextPermission: 'default', routes: { [SUBSCRIPTION_URL]: ok204 } });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, false);
  assert.equal(result.state, 'ready');
  assert.equal(result.reason, 'refused');
  assert.equal(env.requests.length, 0);
});

test('enable: granted subscribes with the server key bytes, posts the subscription and keeps the intent', async () => {
  const env = makeEnv({ permission: 'default', nextPermission: 'granted', routes: { [SUBSCRIPTION_URL]: ok204 } });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, true);
  assert.equal(result.state, 'on');
  assert.equal(env.subscribeOptions.length, 1);
  assert.equal(env.subscribeOptions[0].userVisibleOnly, true);
  assert.deepEqual(Array.from(env.subscribeOptions[0].applicationServerKey), KEY_BYTES);
  assert.equal(env.requests.length, 1);
  const post = env.requests[0];
  assert.equal(post.url, SUBSCRIPTION_URL);
  assert.equal(post.init.method, 'POST');
  assert.equal(post.init.credentials, 'same-origin');
  assert.equal(post.init.headers['Content-Type'], 'application/json');
  assert.deepEqual(post.body, {
    endpoint: ENDPOINT,
    expirationTime: null,
    keys: { p256dh: 'p256dh-value', auth: 'auth-value' }
  });
  assert.equal(env.store.get('pinalert.pushIntent'), '7', 'the intent belongs to the signed in account');
  assertEndpointOnlyInBodies(env);
});

test('enable: permission that is already granted does not ask again', async () => {
  const env = makeEnv({ permission: 'granted', routes: { [SUBSCRIPTION_URL]: ok204 } });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, true);
  assert.ok(!env.calls.includes('requestPermission'));
  assert.ok(env.calls.includes('subscribe'));
});

test('enable: offline sends no request and no prompt', async () => {
  const env = makeEnv({ online: false, routes: { [SUBSCRIPTION_URL]: ok204 } });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'offline');
  assert.equal(env.requests.length, 0);
  assert.ok(!env.calls.includes('requestPermission'));
});

test('enable: a refused session is session, any other failure is failed', async () => {
  const session = makeEnv({ permission: 'granted', routes: { [SUBSCRIPTION_URL]: () => ({ status: 401, body: {} }) } });
  assert.equal((await session.Notify.enable(KEY)).reason, 'session');
  for (const status of [400, 429, 503, 500]) {
    const env = makeEnv({ permission: 'granted', routes: { [SUBSCRIPTION_URL]: () => ({ status, body: {} }) } });
    const result = await env.Notify.enable(KEY);
    assert.equal(result.ok, false, 'status ' + status);
    assert.equal(result.reason, 'failed', 'status ' + status);
    assert.equal(env.store.has('pinalert.pushIntent'), false, 'no intent after a failed registration');
  }
  const network = makeEnv({ permission: 'granted', routes: { [SUBSCRIPTION_URL]: () => new Error('down') } });
  assert.equal((await network.Notify.enable(KEY)).reason, 'failed');
});

test('enable: a 400 carries the server sentence when it sent one', async () => {
  const env = makeEnv({
    permission: 'granted',
    routes: { [SUBSCRIPTION_URL]: () => ({ status: 400, body: { error: { message: 'Not supported here.' } } }) }
  });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.reason, 'failed');
  assert.equal(result.message, 'Not supported here.');
});

test('enable: an unsupported, blocked or keyless device does nothing', async () => {
  const unsupported = makeEnv({ pushManager: false });
  assert.equal((await unsupported.Notify.enable(KEY)).reason, 'unsupported');
  const keyless = makeEnv();
  assert.equal((await keyless.Notify.enable('')).reason, 'unsupported');
  const blocked = makeEnv({ permission: 'denied' });
  const result = await blocked.Notify.enable(KEY);
  assert.equal(result.state, 'blocked');
  assert.equal(result.reason, 'refused');
  assert.equal(unsupported.requests.length + keyless.requests.length + blocked.requests.length, 0);
  assert.ok(!unsupported.calls.includes('requestPermission'));
});

test('disable: tells the server, then the browser, keeps the permission and clears the intent', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    stored: { 'pinalert.pushIntent': '7' },
    routes: { [UNSUBSCRIBE_URL]: ok204 }
  });
  const result = await env.Notify.disable();
  assert.deepEqual({ ok: result.ok, state: result.state }, { ok: true, state: 'ready' });
  assert.equal(env.requests.length, 1);
  assert.equal(env.requests[0].url, UNSUBSCRIBE_URL);
  assert.deepEqual(env.requests[0].body, { endpoint: ENDPOINT });
  assert.ok(env.calls.indexOf('unsubscribe') > env.calls.indexOf('fetch'), 'the server hears first');
  assert.equal(env.permission, 'granted', 'the browser permission is kept');
  assert.equal(env.store.has('pinalert.pushIntent'), false);
  assertEndpointOnlyInBodies(env);
});

test('disable: a failed server call keeps the browser subscription and the intent', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    stored: { 'pinalert.pushIntent': '7' },
    routes: { [UNSUBSCRIBE_URL]: () => ({ status: 503, body: {} }) }
  });
  const result = await env.Notify.disable();
  assert.equal(result.ok, false);
  assert.equal(result.reason, 'failed');
  assert.ok(!env.calls.includes('unsubscribe'));
  assert.equal(env.store.get('pinalert.pushIntent'), '7');
  const offline = makeEnv({ permission: 'granted', subscribed: true, online: false });
  assert.equal((await offline.Notify.disable()).reason, 'offline');
  assert.equal(offline.requests.length, 0);
});

test('disable: a browser with no subscription is already off', async () => {
  const env = makeEnv({ permission: 'granted', subscribed: false, stored: { 'pinalert.pushIntent': '7' } });
  const result = await env.Notify.disable();
  assert.deepEqual({ ok: result.ok, state: result.state }, { ok: true, state: 'ready' });
  assert.equal(env.requests.length, 0);
  assert.equal(env.store.has('pinalert.pushIntent'), false);
});

test('reconcile: re-registers quietly when the server lost the endpoint and the person had turned it on', async () => {
  let registered = false;
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered } }),
      [SUBSCRIPTION_URL]: () => { registered = true; return { status: 204 }; }
    }
  });
  assert.equal(await env.Notify.reconcile(KEY), 'on');
  const urls = env.requests.map((r) => r.url);
  assert.deepEqual(urls, [STATUS_URL, SUBSCRIPTION_URL]);
  assert.equal(env.requests[1].body.endpoint, ENDPOINT);
  assert.ok(!env.calls.includes('requestPermission'), 'reconcile never prompts');
  assertEndpointOnlyInBodies(env);
});

test('reconcile: without the intent, or with another account\'s intent, nothing is posted', async () => {
  const routes = {
    [STATUS_URL]: () => ({ status: 200, body: { registered: false } }),
    [SUBSCRIPTION_URL]: ok204
  };
  const none = makeEnv({ permission: 'granted', subscribed: true, routes });
  assert.equal(await none.Notify.reconcile(KEY), 'ready');
  assert.deepEqual(none.requests.map((r) => r.url), [STATUS_URL]);
  const other = makeEnv({ permission: 'granted', subscribed: true, stored: { 'pinalert.pushIntent': '9' }, routes });
  assert.equal(await other.Notify.reconcile(KEY), 'ready');
  assert.deepEqual(other.requests.map((r) => r.url), [STATUS_URL],
    'a shared phone must not hand the first person\'s device to the next account');
  const noAccount = makeEnv({ permission: 'granted', subscribed: true, account: '', stored: { 'pinalert.pushIntent': '7' }, routes });
  assert.equal(await noAccount.Notify.reconcile(KEY), 'ready');
  assert.deepEqual(noAccount.requests.map((r) => r.url), [STATUS_URL]);
});

test('reconcile: an endpoint the server already knows is on and posts nothing', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    stored: { 'pinalert.pushIntent': '7' },
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: true } }), [SUBSCRIPTION_URL]: ok204 }
  });
  assert.equal(await env.Notify.reconcile(KEY), 'on');
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL]);
});

test('reconcile: a failed quiet post leaves the device ready', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered: false } }),
      [SUBSCRIPTION_URL]: () => ({ status: 503, body: {} })
    }
  });
  assert.equal(await env.Notify.reconcile(KEY), 'ready');
});

const OLD_KEY_BYTES = KEY_BYTES.map((b) => (b + 1) % 256);

test('enable: drops a browser subscription made with another server key, then subscribes with the page key (B-02)', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: OLD_KEY_BYTES,
    routes: { [SUBSCRIPTION_URL]: ok204 }
  });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, true);
  assert.equal(result.state, 'on');
  assert.ok(env.calls.indexOf('unsubscribe') !== -1, 'the old key subscription is dropped');
  assert.ok(env.calls.indexOf('unsubscribe') < env.calls.indexOf('subscribe'), 'dropped before subscribing again');
  assert.deepEqual(Array.from(env.subscribeOptions[0].applicationServerKey), KEY_BYTES);
  assert.deepEqual(env.requests.map((r) => r.url), [SUBSCRIPTION_URL]);
  assert.equal(env.store.get('pinalert.pushIntent'), '7');
  assertEndpointOnlyInBodies(env);
});

test('enable: a subscription made with the page key, or one that reports no key, is left alone', async () => {
  const same = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: KEY_BYTES,
    routes: { [SUBSCRIPTION_URL]: ok204 }
  });
  assert.equal((await same.Notify.enable(KEY)).ok, true);
  assert.ok(!same.calls.includes('unsubscribe'));
  const silent = makeEnv({ permission: 'granted', subscribed: true, routes: { [SUBSCRIPTION_URL]: ok204 } });
  assert.equal((await silent.Notify.enable(KEY)).ok, true);
  assert.ok(!silent.calls.includes('unsubscribe'));
});

test('enable: a prompt is still the first and only call before it returns, with the key check after it (B-02)', async () => {
  const env = makeEnv({
    permission: 'default',
    nextPermission: 'granted',
    subscribed: true,
    subscribedKey: OLD_KEY_BYTES,
    routes: { [SUBSCRIPTION_URL]: ok204 }
  });
  const pending = env.Notify.enable(KEY);
  assert.deepEqual(env.calls, ['requestPermission']);
  assert.equal((await pending).ok, true);
});

test('reconcile: does not re-post a subscription made with another server key (B-02)', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: OLD_KEY_BYTES,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered: false } }),
      [SUBSCRIPTION_URL]: ok204
    }
  });
  assert.equal(await env.Notify.reconcile(KEY), 'ready');
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL]);
});

test('deviceState: a subscription made with another key reads off even when the server still has the row (WR-02)', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: OLD_KEY_BYTES,
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: true } }) }
  });
  assert.equal(await env.Notify.deviceState(KEY), 'ready');
});

test('deviceState: a subscription made with the page key and registered on the server stays on (WR-02)', async () => {
  const same = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: KEY_BYTES,
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: true } }) }
  });
  assert.equal(await same.Notify.deviceState(KEY), 'on');
  const silent = makeEnv({
    permission: 'granted',
    subscribed: true,
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: true } }) }
  });
  assert.equal(await silent.Notify.deviceState(KEY), 'on');
});

test('reconcile: a registered row with another key reads ready and posts nothing, then one enable replaces it (WR-02)', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: OLD_KEY_BYTES,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered: true } }),
      [SUBSCRIPTION_URL]: ok204
    }
  });
  assert.equal(await env.Notify.reconcile(KEY), 'ready');
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL]);
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, true);
  assert.ok(env.calls.indexOf('unsubscribe') !== -1 && env.calls.indexOf('unsubscribe') < env.calls.lastIndexOf('subscribe'));
});

test('reconcile: re-posts a subscription made with the page key', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: KEY_BYTES,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered: false } }),
      [SUBSCRIPTION_URL]: ok204
    }
  });
  assert.equal(await env.Notify.reconcile(KEY), 'on');
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL]);
});

test('keyBytes decodes base64url, with or without padding', () => {
  const env = makeEnv();
  assert.deepEqual(Array.from(env.Notify.keyBytes(KEY)), KEY_BYTES);
  assert.deepEqual(Array.from(env.Notify.keyBytes('-_8')), [251, 255]);
});

// ---- 03.3-UAT test 7 (plan 17): the first page registers the device again ----

// settle lets every pending promise step run. The sandbox has no real waits, so
// one turn of the event loop is enough.
const settle = () => new Promise((resolve) => setImmediate(resolve));

function reconnectEnv(extra = {}) {
  let registered = false;
  return makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: KEY_BYTES,
    pushKey: KEY,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered } }),
      [SUBSCRIPTION_URL]: () => { registered = true; return { status: 204 }; }
    },
    ...extra
  });
}

test('reconnect: the same account back on the first page registers the device again, without a prompt', async () => {
  const env = reconnectEnv();
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL]);
  assert.equal(env.requests[0].body.endpoint, ENDPOINT);
  assert.equal(env.requests[1].body.endpoint, ENDPOINT);
  assert.equal(env.requests[1].body.keys.p256dh, 'p256dh-value');
  assert.ok(!env.calls.includes('requestPermission'), 'a page load never prompts');
  assertEndpointOnlyInBodies(env);
});

test('reconnect: another account\'s intent flag sends no request at all', async () => {
  const env = reconnectEnv({ stored: { 'pinalert.pushIntent': '8' } });
  await settle();
  assert.deepEqual(env.requests, []);
});

test('reconnect: a cleared intent flag sends no request at all', async () => {
  const env = reconnectEnv({ stored: {} });
  await settle();
  assert.deepEqual(env.requests, []);
});

test('reconnect: permission not granted sends no request and does not prompt', async () => {
  for (const permission of ['default', 'denied']) {
    const env = reconnectEnv({ permission });
    await settle();
    assert.deepEqual(env.requests, [], permission);
    assert.ok(!env.calls.includes('requestPermission'), permission);
  }
});

test('reconnect: a kept subscription made with another key is never posted', async () => {
  const env = reconnectEnv({ subscribedKey: OLD_KEY_BYTES });
  await settle();
  assert.ok(!env.requests.some((r) => r.url === SUBSCRIPTION_URL), 'no subscription post on a key mismatch');
  assert.ok(!env.calls.includes('requestPermission'));
});

test('reconnect: a page without a key, with an empty key or without an account sends nothing and throws nothing', async () => {
  const noKey = reconnectEnv({ pushKey: undefined });
  const emptyKey = reconnectEnv({ pushKey: '' });
  const noAccount = reconnectEnv({ account: '' });
  await settle();
  assert.deepEqual(noKey.requests, []);
  assert.deepEqual(emptyKey.requests, []);
  assert.deepEqual(noAccount.requests, []);
});

test('reconnect: nothing is sent where the browser cannot do alerts', async () => {
  const noPush = reconnectEnv({ pushManager: false });
  const insecure = reconnectEnv({ secure: false });
  await settle();
  assert.deepEqual(noPush.requests, []);
  assert.deepEqual(insecure.requests, []);
});

test('reconnect: an endpoint the server already knows costs one status request and no subscription post', async () => {
  const env = reconnectEnv({
    routes: { [STATUS_URL]: () => ({ status: 200, body: { registered: true } }), [SUBSCRIPTION_URL]: ok204 }
  });
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL]);
});

test('reconnect: a failing request is swallowed', async () => {
  const env = reconnectEnv({ routes: { [STATUS_URL]: () => new Error('network down') } });
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL]);
});

test('reconnect: Settings calling reconcile during the load check reuses it, and a later call checks afresh', async () => {
  const env = reconnectEnv();
  const during = env.Notify.reconcile(KEY);
  assert.equal(await during, 'on');
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL],
    'the Settings call added no second status request');
  assert.equal(await env.Notify.reconcile(KEY), 'on');
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL, STATUS_URL],
    'after the load check settled, reconcile asks the server again');
});

test('reconnect: a reconcile with another key during the load check is its own check', async () => {
  const env = reconnectEnv();
  const other = Buffer.from(OLD_KEY_BYTES).toString('base64url');
  await env.Notify.reconcile(other);
  await settle();
  assert.equal(env.requests.filter((r) => r.url === STATUS_URL).length, 2);
});

const MAC_TOUCHLESS = { userAgent: MAC_UA, maxTouchPoints: 0 };

test('isMacDockApp: an installed app on a Mac is true', () => {
  assert.equal(makeEnv({ ...MAC_TOUCHLESS, displayStandalone: true }).Notify.isMacDockApp(), true);
});

test('isMacDockApp: a Safari tab on a Mac is false', () => {
  assert.equal(makeEnv({ ...MAC_TOUCHLESS }).Notify.isMacDockApp(), false);
});

test('isMacDockApp: an installed iPhone app is false', () => {
  assert.equal(makeEnv({ userAgent: IPHONE_UA, standalone: true, displayStandalone: true }).Notify.isMacDockApp(), false);
});

test('isMacDockApp: an installed iPad that says Macintosh, with touch points, is false', () => {
  assert.equal(makeEnv({ userAgent: MAC_UA, maxTouchPoints: 5, standalone: true, displayStandalone: true }).Notify.isMacDockApp(), false);
});

test('isMacDockApp: an installed app with a Windows or Android user agent is false', () => {
  const windows = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36';
  assert.equal(makeEnv({ userAgent: windows, maxTouchPoints: 0, displayStandalone: true }).Notify.isMacDockApp(), false);
  assert.equal(makeEnv({ userAgent: ANDROID_UA, displayStandalone: true }).Notify.isMacDockApp(), false);
});

// ---- WR-02 (03.3-REVIEW): Off in Settings is never undone by the load check ----

// serverEnv is a page whose fake server keeps one row: a subscription post
// adds it when the post arrives, an unsubscribe removes it. The subscription
// post can be held back, so a test can put the load check between its status
// answer and the arrival of its post.
function serverEnv(extra = {}) {
  const server = { registered: false };
  server.held = new Promise((resolve) => { server.release = resolve; });
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: KEY_BYTES,
    pushKey: KEY,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => ({ status: 200, body: { registered: server.registered } }),
      [SUBSCRIPTION_URL]: () => server.held.then(() => { server.registered = true; return { status: 204 }; }),
      [UNSUBSCRIBE_URL]: () => { server.registered = false; return { status: 204 }; }
    },
    ...extra
  });
  return { env, server };
}

test('disable: Off tapped while the load check is posting waits for it, so the endpoint stays off (WR-02)', async () => {
  const { env, server } = serverEnv();
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL], 'the load check is posting');
  let result = null;
  env.Notify.disable().then((r) => { result = r; });
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL],
    'disable sends nothing while the load check is out');
  server.release();
  await settle();
  await settle();
  assert.ok(result, 'disable answered');
  assert.equal(result.ok, true);
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL, SUBSCRIPTION_URL, UNSUBSCRIBE_URL]);
  assert.equal(server.registered, false, 'the server ends with no row for this endpoint');
  assert.equal(env.store.has('pinalert.pushIntent'), false);
});

test('reconnect: an intent flag cleared while the status request is out stops the re-register (WR-02)', async () => {
  const env = makeEnv({
    permission: 'granted',
    subscribed: true,
    subscribedKey: KEY_BYTES,
    pushKey: KEY,
    stored: { 'pinalert.pushIntent': '7' },
    routes: {
      [STATUS_URL]: () => { env.store.delete('pinalert.pushIntent'); return { status: 200, body: { registered: false } }; },
      [SUBSCRIPTION_URL]: ok204
    }
  });
  await settle();
  assert.deepEqual(env.requests.map((r) => r.url), [STATUS_URL], 'no subscription post after the flag was cleared');
});

test('enable: never waits for the load check before its first browser call (WR-02)', async () => {
  const { env, server } = serverEnv({ permission: 'default' });
  await settle();
  const before = env.calls.length;
  env.Notify.enable(KEY);
  assert.deepEqual(env.calls.slice(before), ['requestPermission'], 'the prompt is asked in the tap');
  server.release();
  await settle();
});

// ---- CR-02 (03.3-REVIEW): every browser push call inside enable has a time limit ----

// fireTimers runs every timer still set once, the way time passing would.
function fireTimers(env) {
  const live = env.timers.filter((t) => t.live);
  live.forEach((t) => { t.live = false; t.fn(); });
  return live;
}

for (const hang of ['getSubscription', 'unsubscribe', 'subscribe']) {
  test('enable: a ' + hang + ' that never settles ends in a failure after the time limit (CR-02)', async () => {
    const env = makeEnv({
      permission: 'granted',
      manualTimers: true,
      hang,
      // Only a subscription made with another key is unsubscribed.
      subscribed: hang === 'unsubscribe',
      subscribedKey: hang === 'unsubscribe' ? OLD_KEY_BYTES : undefined,
      routes: { [SUBSCRIPTION_URL]: ok204 }
    });
    let result = null;
    env.Notify.enable(KEY).then((r) => { result = r; });
    await settle();
    assert.ok(env.calls.includes(hang), hang + ' was started');
    assert.equal(result, null, 'still waiting for the browser');
    const fired = fireTimers(env);
    assert.ok(fired.some((t) => t.ms === env.Notify.REQUEST_TIMEOUT_MS), 'a time limit was running');
    await settle();
    assert.ok(result, 'enable answered');
    assert.deepEqual({ ok: result.ok, state: result.state, reason: result.reason }, { ok: false, state: 'ready', reason: 'failed' });
    assert.equal(env.requests.length, 0, 'no subscription was posted');
    assert.equal(env.store.has('pinalert.pushIntent'), false);
  });
}

test('enable: the permission prompt has no time limit, it is the person\'s own choice (CR-02)', async () => {
  const env = makeEnv({ permission: 'default', nextPermission: 'hang', manualTimers: true });
  let result = null;
  env.Notify.enable(KEY).then((r) => { result = r; });
  await settle();
  assert.deepEqual(env.calls, ['requestPermission']);
  assert.equal(env.timers.filter((t) => t.live).length, 0, 'no timer races the prompt');
  assert.equal(result, null);
});

test('enable: with time limits in place the browser calls still start at once and in order (CR-02)', async () => {
  const env = makeEnv({
    permission: 'granted',
    manualTimers: true,
    subscribed: true,
    subscribedKey: OLD_KEY_BYTES,
    routes: { [SUBSCRIPTION_URL]: ok204 }
  });
  const result = await env.Notify.enable(KEY);
  assert.equal(result.ok, true);
  assert.deepEqual(env.calls.filter((c) => c !== 'fetch'), ['ready', 'getSubscription', 'unsubscribe', 'subscribe']);
  assert.equal(env.timers.filter((t) => t.live).length, 0, 'every limit was cleared once its call answered');
});
