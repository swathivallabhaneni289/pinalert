// notify.js: the one browser side module for alerts on this device (Phase 3.3,
// plan 06, UI-SPEC Surfaces 2 and 4, D-10 to D-13).
//
// What it does:
//   - Registers the service worker /sw.js at page load, scope /, so the worker
//     is known early (Safari wants that) and the app can be installed.
//   - Answers what this device can do: baseState() from the browser alone, and
//     deviceState(key) with the server's word added. The states are
//     'unsupported', 'needs-install', 'blocked', 'on' and 'ready'.
//   - Turns this device on (enable) and off (disable), in the order browsers
//     require, and quietly re-registers a subscription the server lost
//     (reconcile).
//
// Every 'on' is confirmed by the server for this account and this endpoint. The
// local subscription alone never makes a device 'on', so a browser that someone
// else signed in to cannot show the previous person's switch as on (UI-SPEC 2a,
// T-03.3-10).
//
// The endpoint of a subscription is a capability address, so it travels only in
// JSON bodies of POST requests, never in a request address (T-03.3-02).
//
// The "intent" flag in local storage says this account turned the device on
// here. It holds the account id, so reconcile re-registers a subscription only
// for the account that turned it on; the next account on a shared phone is never
// handed the first person's device.
//
// What it never does:
//   - It sends no position, writes no text to the page, builds no markup and
//     adds no listener on the window or the document. It reacts only to calls
//     from welcome.js and settings-alerts.js, which draw every word on screen.
//   - It does not touch the browser's own install prompt. Android and laptop
//     browsers keep their own install control; there is no custom Install
//     button (decision 5, UI-SPEC 4c).
//   - It does not decide any wording. It returns states and reasons only.
(function () {
  'use strict';

  var SUBSCRIPTION_URL = '/api/me/push/subscription';
  var STATUS_URL = '/api/me/push/status';
  var UNSUBSCRIBE_URL = '/api/me/push/unsubscribe';
  var SW_URL = '/sw.js';
  var INTENT_KEY = 'pinalert.pushIntent';
  var REQUEST_TIMEOUT_MS = 10000;
  // An account id is a plain positive whole number, like position.js reads it.
  var ACCOUNT_ID_PATTERN = /^[1-9][0-9]{0,18}$/;

  function hasText(value) {
    return typeof value === 'string' && value !== '';
  }

  function secureContext() {
    return window.isSecureContext === true;
  }

  function hasServiceWorker() {
    return typeof navigator !== 'undefined' && !!navigator && !!navigator.serviceWorker;
  }

  // isAppleMobile is true on an iPhone or iPad. An iPad that asks for the
  // desktop site says Macintosh, so more than one touch point settles it.
  function isAppleMobile() {
    if (typeof navigator === 'undefined' || !navigator) {
      return false;
    }
    var agent = typeof navigator.userAgent === 'string' ? navigator.userAgent : '';
    if (/iPhone|iPad/.test(agent)) {
      return true;
    }
    return /Macintosh/.test(agent) && typeof navigator.maxTouchPoints === 'number' &&
      navigator.maxTouchPoints > 1;
  }

  // isInstalled is true when the page runs as an app from the Home Screen.
  function isInstalled() {
    if (typeof navigator !== 'undefined' && navigator && navigator.standalone === true) {
      return true;
    }
    try {
      return typeof window.matchMedia === 'function' &&
        window.matchMedia('(display-mode: standalone)').matches === true;
    } catch (err) {
      return false;
    }
  }

  // baseState is what the browser alone can say, with no request:
  //   'needs-install' an iPhone or iPad that is not running as an installed app
  //                   (checked first: PushManager is missing in that tab);
  //   'unsupported'   not a secure context, or no service worker, PushManager
  //                   or Notification;
  //   'blocked'       the person refused notifications for this site;
  //   'maybe'         everything else; the server decides between on and ready.
  function baseState() {
    if (isAppleMobile() && !isInstalled()) {
      return 'needs-install';
    }
    if (!secureContext() || !hasServiceWorker() || !window.PushManager || !window.Notification) {
      return 'unsupported';
    }
    if (window.Notification.permission === 'denied') {
      return 'blocked';
    }
    return 'maybe';
  }

  function currentAccount() {
    if (typeof document === 'undefined' || !document || typeof document.querySelector !== 'function') {
      return '';
    }
    var node = document.querySelector('[data-account-id]');
    var raw = node ? node.getAttribute('data-account-id') : '';
    return typeof raw === 'string' && ACCOUNT_ID_PATTERN.test(raw) ? raw : '';
  }

  // The intent flag. A failing store reads as no intent and a failed write is
  // ignored: the device still works, it only cannot be re-registered later.
  function intentIsSet() {
    var account = currentAccount();
    if (account === '') {
      return false;
    }
    try {
      return window.localStorage.getItem(INTENT_KEY) === account;
    } catch (err) {
      return false;
    }
  }

  function setIntent() {
    var account = currentAccount();
    if (account === '') {
      return;
    }
    try {
      window.localStorage.setItem(INTENT_KEY, account);
    } catch (err) {
      // Nothing more can be done.
    }
  }

  function clearIntent() {
    try {
      window.localStorage.removeItem(INTENT_KEY);
    } catch (err) {
      // Nothing more can be done.
    }
  }

  // keyBytes decodes a base64url string (the server's public key) to a
  // Uint8Array, with or without padding.
  function keyBytes(text) {
    var padded = String(text).replace(/-/g, '+').replace(/_/g, '/');
    while (padded.length % 4 !== 0) {
      padded += '=';
    }
    var raw = atob(padded);
    var out = new Uint8Array(raw.length);
    for (var i = 0; i < raw.length; i++) {
      out[i] = raw.charCodeAt(i);
    }
    return out;
  }

  // asBytes reads an ArrayBuffer or a typed array as a Uint8Array, or null.
  function asBytes(value) {
    if (!value || typeof value !== 'object') {
      return null;
    }
    if (ArrayBuffer.isView(value)) {
      return new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
    }
    if (Object.prototype.toString.call(value) === '[object ArrayBuffer]') {
      return new Uint8Array(value);
    }
    return null;
  }

  // sameKey says whether a browser subscription was made with the page's server
  // key. A subscription that does not report its key counts as the same, so only
  // a key that is known to differ is ever dropped or left alone.
  function sameKey(subscription, key) {
    var have = asBytes(subscription && subscription.options && subscription.options.applicationServerKey);
    if (have === null) {
      return true;
    }
    var want = keyBytes(key);
    if (have.length !== want.length) {
      return false;
    }
    for (var i = 0; i < have.length; i++) {
      if (have[i] !== want[i]) {
        return false;
      }
    }
    return true;
  }

  // withTimeout ends a wait that never settles (for example a worker that never
  // became ready) after REQUEST_TIMEOUT_MS.
  function withTimeout(promise) {
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = window.setTimeout(function () {
        timer = null;
        reject(new Error('no answer in time'));
      }, REQUEST_TIMEOUT_MS);
    });
    expired.catch(function () {});
    function clear() {
      if (timer !== null) {
        window.clearTimeout(timer);
        timer = null;
      }
    }
    return Promise.race([promise, expired]).then(function (answer) {
      clear();
      return answer;
    }, function (err) {
      clear();
      throw err;
    });
  }

  // post sends one JSON POST with a time limit. It resolves to
  // {status, ok, body} and rejects on a network failure or the limit.
  function post(address, payload) {
    var init = {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    };
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    if (controller !== null) {
      init.signal = controller.signal;
    }
    var work = fetch(address, init).then(function (res) {
      return res.json().then(function (body) {
        return body;
      }, function () {
        return {};
      }).then(function (body) {
        return { status: res.status, ok: res.ok, body: body };
      });
    });
    return withTimeout(work).catch(function (err) {
      if (controller !== null) {
        controller.abort();
      }
      throw err;
    });
  }

  function serverSentence(body) {
    var sent = body && body.error && body.error.message;
    if (typeof sent === 'string' && sent !== '' && sent.length <= 200) {
      return sent;
    }
    return '';
  }

  function readyRegistration() {
    return withTimeout(Promise.resolve(navigator.serviceWorker.ready));
  }

  // currentSubscription resolves to this browser's push subscription or null.
  function currentSubscription() {
    return readyRegistration().then(function (registration) {
      return registration.pushManager.getSubscription();
    });
  }

  // register puts the service worker in place. A failure is ignored: the page
  // works without the worker, only alerts and installing need it.
  function register() {
    if (!secureContext() || !hasServiceWorker()) {
      return Promise.resolve(null);
    }
    try {
      return Promise.resolve(navigator.serviceWorker.register(SW_URL, {
        scope: '/',
        updateViaCache: 'none'
      })).catch(function () {
        return null;
      });
    } catch (err) {
      return Promise.resolve(null);
    }
  }

  // inspect works out the device state and keeps what it found out. The result
  // is {state, sub, known}: sub is this browser's subscription when there is
  // one, and known is true when the server answered and said this endpoint is
  // not registered for this account.
  function inspect(key) {
    if (!hasText(key)) {
      return Promise.resolve({ state: 'unsupported' });
    }
    var base = baseState();
    if (base !== 'maybe') {
      return Promise.resolve({ state: base });
    }
    if (window.Notification.permission !== 'granted') {
      return Promise.resolve({ state: 'ready' });
    }
    return currentSubscription().then(function (sub) {
      if (!sub) {
        return { state: 'ready' };
      }
      return post(STATUS_URL, { endpoint: sub.endpoint }).then(function (answer) {
        if (answer.ok && answer.body && answer.body.registered === true) {
          // The server still holds the row, but a subscription made with
          // another key (the operator rotated the pair) can never be sent to.
          // Read it as off, so one tap runs enable, which drops it.
          if (!sameKey(sub, key)) {
            return { state: 'ready', sub: sub, known: true };
          }
          return { state: 'on', sub: sub };
        }
        return {
          state: 'ready',
          sub: sub,
          known: answer.ok && !!answer.body && answer.body.registered === false
        };
      });
    }).catch(function () {
      return { state: 'ready' };
    });
  }

  // deviceState resolves to 'unsupported', 'needs-install', 'blocked', 'on' or
  // 'ready'. 'on' only when the server confirms this endpoint for this account.
  function deviceState(key) {
    return inspect(key).then(function (found) {
      return found.state;
    });
  }

  // askPermission calls Notification.requestPermission in the same tick, then
  // resolves to 'granted', 'denied' or 'default'. It handles both the promise
  // form and the older callback form.
  function askPermission() {
    return new Promise(function (resolve) {
      var finished = false;
      function done(answer) {
        if (!finished) {
          finished = true;
          resolve(answer);
        }
      }
      var asked = window.Notification.requestPermission(done);
      if (asked && typeof asked.then === 'function') {
        asked.then(done, function () {
          done('default');
        });
      }
    });
  }

  function isOffline() {
    return typeof navigator !== 'undefined' && !!navigator && navigator.onLine === false;
  }

  function failure(err) {
    return { ok: false, state: 'ready', reason: isOffline() ? 'offline' : 'failed' };
  }

  // enable turns this device on. Call it straight from the click handler, with
  // nothing awaited before it: when the question has not been asked yet,
  // Notification.requestPermission is the first thing it does, because Safari
  // refuses a prompt that does not come directly from the tap. It resolves to
  // {ok, state, reason}; reason is 'refused', 'offline', 'failed', 'session' or
  // 'unsupported', and a 400 may also carry the server's sentence as message.
  function enable(key) {
    var base = baseState();
    if (!hasText(key)) {
      return Promise.resolve({ ok: false, state: 'unsupported', reason: 'unsupported' });
    }
    if (base === 'needs-install' || base === 'unsupported') {
      return Promise.resolve({ ok: false, state: base, reason: 'unsupported' });
    }
    if (base === 'blocked') {
      return Promise.resolve({ ok: false, state: 'blocked', reason: 'refused' });
    }
    if (isOffline()) {
      return Promise.resolve({ ok: false, state: 'ready', reason: 'offline' });
    }
    var asked = window.Notification.permission === 'granted'
      ? Promise.resolve('granted')
      : askPermission();
    return asked.then(function (answer) {
      if (answer === 'denied') {
        return { ok: false, state: 'blocked', reason: 'refused' };
      }
      if (answer !== 'granted') {
        return { ok: false, state: 'ready', reason: 'refused' };
      }
      return readyRegistration().then(function (registration) {
        // A subscription made with another server key (the operator changed
        // the key pair) makes subscribe fail for good, so drop it first.
        return Promise.resolve(registration.pushManager.getSubscription()).then(function (existing) {
          if (existing && !sameKey(existing, key)) {
            return existing.unsubscribe();
          }
          return null;
        }).then(function () {
          return registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: keyBytes(key)
          });
        });
      }).then(function (subscription) {
        return post(SUBSCRIPTION_URL, subscription.toJSON());
      }).then(function (answer2) {
        if (answer2.status === 204) {
          setIntent();
          return { ok: true, state: 'on' };
        }
        if (answer2.status === 401) {
          return { ok: false, state: 'ready', reason: 'session' };
        }
        var result = { ok: false, state: 'ready', reason: 'failed' };
        if (answer2.status === 400 && serverSentence(answer2.body) !== '') {
          result.message = serverSentence(answer2.body);
        }
        return result;
      });
    }).catch(failure);
  }

  // disable turns this device off: the server drops the endpoint first, then
  // the browser unsubscribes. The browser permission is kept, so turning on
  // again asks nothing. A failed server call keeps the browser subscription, so
  // the device and the server never disagree about it.
  function disable() {
    if (isOffline()) {
      return Promise.resolve({ ok: false, state: 'on', reason: 'offline' });
    }
    return currentSubscription().then(function (subscription) {
      if (!subscription) {
        clearIntent();
        return { ok: true, state: 'ready' };
      }
      return post(UNSUBSCRIBE_URL, { endpoint: subscription.endpoint }).then(function (answer) {
        if (answer.status === 401) {
          return { ok: false, state: 'on', reason: 'session' };
        }
        if (answer.status !== 204) {
          return { ok: false, state: 'on', reason: 'failed' };
        }
        clearIntent();
        return Promise.resolve(subscription.unsubscribe()).then(function () {
          return { ok: true, state: 'ready' };
        }, function () {
          // The server already dropped the endpoint, so the device is off for
          // this account whatever the browser kept.
          return { ok: true, state: 'ready' };
        });
      });
    }).catch(function () {
      return { ok: false, state: 'on', reason: isOffline() ? 'offline' : 'failed' };
    });
  }

  // reconcile resolves to the device state, and first re-registers the browser's
  // subscription when the server lost it and this account had turned the device
  // on here. It never asks the browser for anything.
  function reconcile(key) {
    return inspect(key).then(function (found) {
      if (found.state !== 'ready' || !found.sub || found.known !== true || !intentIsSet()) {
        return found.state;
      }
      // A subscription made with another server key could never be sent to.
      if (!sameKey(found.sub, key)) {
        return found.state;
      }
      return post(SUBSCRIPTION_URL, found.sub.toJSON()).then(function (answer) {
        return answer.status === 204 ? 'on' : 'ready';
      }).catch(function () {
        return 'ready';
      });
    });
  }

  register();

  window.PinalertNotify = {
    SUBSCRIPTION_URL: SUBSCRIPTION_URL,
    STATUS_URL: STATUS_URL,
    UNSUBSCRIBE_URL: UNSUBSCRIBE_URL,
    SW_URL: SW_URL,
    INTENT_KEY: INTENT_KEY,
    REQUEST_TIMEOUT_MS: REQUEST_TIMEOUT_MS,
    register: register,
    baseState: baseState,
    deviceState: deviceState,
    enable: enable,
    disable: disable,
    reconcile: reconcile,
    isAppleMobile: isAppleMobile,
    isInstalled: isInstalled,
    keyBytes: keyBytes
  };
}());
