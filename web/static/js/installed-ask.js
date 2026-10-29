// installed-ask.js: window.PinalertInstalledAsk, the box that asks about alerts
// once inside the installed iPhone or iPad app (Phase 7.1 plan 13c, D-17, D-18,
// owner decision 2026-10-28).
//
// WHY IT EXISTS. An iPhone Safari tab cannot get alerts, so the sign up alerts
// pop-up never opens there; plan 13b shows that tab the install steps instead,
// and step 3 of the guide says "sign in. Tap Allow when it asks about alerts."
// This script makes that true: after signing in inside the installed app, Nearby
// asks with the same words as the sign up pop-up (the installed_alerts_ask
// partial, kept equal to it by TestInstalledAlertsAskMatchesSignUp). The box is
// drawn by the server only when the page has a push key and the account has a
// home area. This script builds no markup and writes no words; it opens and
// closes the dialog, toggles disabled and aria-busy, and moves focus.
//
// WHEN IT OPENS, all of these, checked in this order:
//   - the page has the dialog and it can showModal;
//   - PinalertNotify.isAppleMobile() and isInstalled() are true (so a Mac Dock
//     app is never asked: Safari on the Mac can already get alerts from the sign
//     up pop-up, and a second subscription on one Mac would notify twice, D-18),
//     and baseState() answers 'maybe';
//   - this device has not asked this account before (the key below);
//   - deviceState(pushKey) resolves to 'ready' (not on, blocked or unsupported;
//     the key is the dialog's data-push-key, which notify.js needs to ask the
//     server whether this device is already on);
//   - then, at the moment it would open, no other dialog is open and the page
//     root carries no data-modal-open. Nearby can hold three small dialogs: this
//     ask (on load), the first post or vote pop-up of Phase 3.3 (after a post or
//     a vote) and the star box of Phase 9 (after Yes, mark resolved). changes-ask.js
//     makes the same check, so no two of these boxes ever stack. When this
//     check skips, the key is not written.
// The Low data view and the Demo never load this script. Any probe that throws
// counts as "do not open".
//
// ONCE PER DEVICE AND ACCOUNT (D-18). The key pinalert.alertsAsked holds a small
// JSON list of account ids, the last 8, so a shared phone does not ask account A
// again after B. One plain id (the older format of pinalert.changesAsked) reads
// as a list of one. The id is added just before the dialog opens, so a tab closed
// mid-question does not ask again. The sign up alerts pop-up writes the same key
// when it opens, so a person who signed up inside the installed app is not asked
// a second time. Storage that cannot be read or written, or a value that is
// neither an id nor a list, means no ask at all.
//
// ALLOW. The first real statement of the tap calls PinalertNotify.enable with the
// dialog's key, before anything is read or awaited, so the browser counts the tap
// as the person's own. Whatever enable answers (a refusal, a rejection, a throw),
// the Reliable switch is then saved only when the box is ticked, with the same
// request the sign up pop-up uses, and the dialog closes. Not now and Escape close
// it and never call enable or send a request. Settings is the way to change your
// mind afterwards. No message is shown after an answer.
(function () {
  'use strict';

  var ASKED_KEY = 'pinalert.alertsAsked';
  var ALERTS_URL = '/api/me/alerts';
  var REQUEST_TIMEOUT_MS = 10000;
  // The shape of an account id, the one notify.js and changes-ask.js use.
  var ACCOUNT_ID_PATTERN = /^[1-9][0-9]{0,18}$/;
  // How many account ids the asked list keeps on one device.
  var ASKED_KEEP = 8;

  // The one run of this page load. A second call returns it, so the dialog can
  // never open twice.
  var current = null;

  // askedList reads the account ids asked on this device, as text. Nothing stored
  // is an empty list, one plain id is a list of one, and a value that is neither
  // is null (do not open). A storage that cannot be read, or text that is not
  // JSON, throws; the caller holds the try.
  function askedList(storage) {
    var raw = storage.getItem(ASKED_KEY);
    if (raw === null || raw === undefined) {
      return [];
    }
    if (ACCOUNT_ID_PATTERN.test(raw)) {
      return [raw];
    }
    var list = JSON.parse(raw);
    if (!Array.isArray(list)) {
      return null;
    }
    return list.filter(function (id) {
      return typeof id === 'string' && ACCOUNT_ID_PATTERN.test(id);
    });
  }

  function pageLocked(doc) {
    var root = doc.documentElement;
    return !!doc.querySelector('dialog[open]') ||
      !!(root && typeof root.hasAttribute === 'function' && root.hasAttribute('data-modal-open'));
  }

  // saveReliable is the one place the Reliable reporter request is built: the
  // same PATCH the sign up pop-up sends. It always resolves, whatever the answer.
  function saveReliable(fetchFn) {
    return new Promise(function (resolve) {
      var timer = null;
      var controller = typeof AbortController === 'function' ? new AbortController() : null;
      function done() {
        if (timer !== null && typeof clearTimeout === 'function') {
          clearTimeout(timer);
        }
        resolve();
      }
      try {
        var init = {
          method: 'PATCH',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ reliable_alerts: true })
        };
        if (controller) {
          init.signal = controller.signal;
          if (typeof setTimeout === 'function') {
            timer = setTimeout(function () {
              controller.abort();
            }, REQUEST_TIMEOUT_MS);
          }
        }
        Promise.resolve(fetchFn(ALERTS_URL, init)).then(done, done);
      } catch (err) {
        done();
      }
    });
  }

  // wire attaches the four listeners of an opened dialog and returns nothing; the
  // closures below hold the state of this one ask.
  function wire(parts, notify, fetchFn) {
    var dialog = parts.dialog;
    var busy = false;
    var finished = false;
    var opener = null;

    function setBusy(on) {
      parts.allow.disabled = on;
      parts.later.disabled = on;
      if (parts.reliable) {
        parts.reliable.disabled = on;
      }
      if (on) {
        dialog.setAttribute('aria-busy', 'true');
      } else {
        dialog.removeAttribute('aria-busy');
      }
    }

    // finish closes the dialog and gives focus back to the element that had it.
    // Nothing is sent from the close event, so one answer is one request at most.
    function finish() {
      if (finished) {
        return;
      }
      finished = true;
      try {
        if (dialog.open && typeof dialog.close === 'function') {
          dialog.close();
        }
      } catch (err) {
        // A dialog that will not close is left as the browser has it.
      }
      busy = false;
      setBusy(false);
      var back = opener;
      opener = null;
      if (back && back.isConnected !== false && typeof back.focus === 'function') {
        back.focus();
      }
    }

    function onAllow() {
      if (busy || finished) {
        return;
      }
      var pending = null;
      try {
        pending = notify.enable(parts.key);
      } catch (err) {
        pending = null;
      }
      busy = true;
      var wantReliable = !!(parts.reliable && parts.reliable.checked);
      setBusy(true);
      function save() {
        if (!wantReliable) {
          return Promise.resolve();
        }
        try {
          return saveReliable(fetchFn);
        } catch (err) {
          return Promise.resolve();
        }
      }
      function done() {
        finish();
      }
      Promise.resolve(pending).then(save, save).then(done, done);
    }

    function onLater() {
      if (busy) {
        return;
      }
      finish();
    }

    // Escape: the browser would close the dialog by itself, so that is prevented
    // and the answer is Not now. While Allow is busy it does nothing more.
    function onCancel(e) {
      e.preventDefault();
      onLater();
    }

    // A browser can close the dialog by itself. Nothing is sent from here.
    function onClose() {
      finish();
    }

    parts.allow.addEventListener('click', onAllow);
    parts.later.addEventListener('click', onLater);
    dialog.addEventListener('cancel', onCancel);
    dialog.addEventListener('close', onClose);

    return function open() {
      opener = parts.doc.activeElement || null;
      dialog.showModal();
      // The question takes the first focus, as the Log out box does, so no button
      // looks chosen.
      parts.title.focus();
    };
  }

  function start(o) {
    var doc = o.doc || (typeof document !== 'undefined' ? document : null);
    var notify = o.notify || (typeof window !== 'undefined' ? window.PinalertNotify : null);
    var fetchFn = o.fetchFn || function (target, init) {
      return fetch(target, init);
    };
    var storage = o.storage;
    if (storage === undefined) {
      try {
        storage = window.localStorage;
      } catch (err) {
        storage = null;
      }
    }
    try {
      if (!doc || !notify || !storage || typeof doc.getElementById !== 'function') {
        return Promise.resolve('skipped');
      }
      var dialog = doc.getElementById('installed-alerts-ask');
      var title = doc.getElementById('installed-alerts-ask-title');
      var later = doc.getElementById('installed-alerts-ask-later');
      var allow = doc.getElementById('installed-alerts-ask-allow');
      var reliable = doc.getElementById('installed-alerts-ask-reliable');
      if (!dialog || !title || !later || !allow || typeof dialog.showModal !== 'function') {
        return Promise.resolve('skipped');
      }
      if (typeof notify.isAppleMobile !== 'function' || typeof notify.isInstalled !== 'function' ||
        typeof notify.baseState !== 'function' || typeof notify.deviceState !== 'function' ||
        typeof notify.enable !== 'function') {
        return Promise.resolve('skipped');
      }
      var key = dialog.getAttribute('data-push-key');
      var account = dialog.getAttribute('data-account-id');
      if (typeof key !== 'string' || key === '' || typeof account !== 'string' || !ACCOUNT_ID_PATTERN.test(account)) {
        return Promise.resolve('skipped');
      }
      if (notify.isAppleMobile() !== true || notify.isInstalled() !== true || notify.baseState() !== 'maybe') {
        return Promise.resolve('skipped');
      }
      var asked = askedList(storage);
      if (asked === null || asked.indexOf(account) !== -1) {
        return Promise.resolve('skipped');
      }
      var parts = { doc: doc, dialog: dialog, title: title, later: later, allow: allow, reliable: reliable, key: key };
      return Promise.resolve(notify.deviceState(key)).then(function (state) {
        if (state !== 'ready') {
          return 'skipped';
        }
        // Another box is open or locks the page now: skip, and do not write the
        // key, so the next visit can ask.
        if (pageLocked(doc)) {
          return 'skipped';
        }
        var list = askedList(storage);
        if (list === null || list.indexOf(account) !== -1) {
          return 'skipped';
        }
        list.push(account);
        storage.setItem(ASKED_KEY, JSON.stringify(list.slice(-ASKED_KEEP)));
        wire(parts, notify, fetchFn)();
        return 'opened';
      }).then(null, function () {
        return 'skipped';
      });
    } catch (err) {
      return Promise.resolve('skipped');
    }
  }

  // run starts the ask once per page load; a second call returns the first run.
  // The options are for tests: { doc, notify, storage, fetchFn }.
  function run(options) {
    if (current === null) {
      current = start(options || {});
    }
    return current;
  }

  window.PinalertInstalledAsk = {
    ASKED_KEY: ASKED_KEY,
    run: run
  };

  // On a page without the dialog or without notify.js this leaves quietly.
  if (typeof document !== 'undefined' && document && document.getElementById('installed-alerts-ask')) {
    run();
  }
})();
