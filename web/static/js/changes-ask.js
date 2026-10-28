// changes-ask.js: window.PinalertChangesAsk, the pop-up that asks "Tell you when
// this report changes?" the first time a person posts a report or confirms or
// disputes one (Phase 3.3 plan 20, the 03.3-UAT gap for test 4, owner
// 2026-10-27). Decision D-04 as amended: the question is asked where the person
// is, in the flow, and Settings is only for changing their mind. The Changes
// switch stays off until they say yes.
//
// The dialog is the changes_ask partial (the Log out box look). This script
// builds no markup and writes no words; the words are in the partial. It only
// opens and closes the dialog, toggles disabled and aria-busy, and moves focus.
// Plan 21 renders the partial; on a page without it every call here does
// nothing, so votes.js and report-form.js may call offer() anywhere.
//
// WHEN IT OPENS (canOffer), all of these:
//   - the page has the dialog, and the dialog can showModal;
//   - the page carries a push key (data-push-key) and an account id
//     (data-account-id), both set by the server (plan 17);
//   - notify.js says alerts can work here: its base state is maybe or ready
//     (never needs-install, blocked or unsupported, so never an iPhone Safari
//     tab), and this is not a Mac Dock app, whose browser already holds this
//     Mac's subscription (07.1 D-18);
//   - no other dialog is open and the page is not locked by another box
//     (data-modal-open on the root, or this dialog made inert), so it never
//     stacks on another box;
//   - this account has not been asked in this browser (see the key below).
// Any probe that throws counts as "do not open".
//
// ONCE PER ACCOUNT ON A BROWSER (P-1). The key pinalert.changesAsked holds a
// small JSON list of the account ids asked here, the last 8, so a shared phone
// does not ask account A again after B (WR-04). An older value, one plain id,
// reads as a list of one. The list is written just before the pop-up opens, so
// a tab closed mid-question does not ask again. A server side flag would need a
// migration. Storage that cannot be read or written, or a value that is neither
// an id nor a list, means no pop-up. Plan 21 renders the
// dialog only while the account's Changes switch is off, so an account that
// already said yes is never asked.
//
// ALLOW. The first real statement of the tap calls PinalertNotify.enable with
// the page key (turning this device on when it is off), so the browser counts
// the tap as the person's own. Whatever enable answers, the Changes switch is
// then turned on with one request (P-4): Changes is an account switch, and
// another device of the account may be on. Not now and Escape send nothing and
// never call enable; Settings is the way back (D-12). No message is shown after
// an answer (P-5): the pop-up closes and the page is as it was.
//
// SETTLED. offer() decides and opens in the same tick it is called, so
// settled() already returns the pending promise right after a vote or a post.
// A later box waits on it (Phase 9 plan 09-17, the star box after a resolve).
// It resolves at once when nothing is open.
(function () {
  'use strict';

  var ALERTS_URL = '/api/me/alerts';
  var ASKED_KEY = 'pinalert.changesAsked';
  var REQUEST_TIMEOUT_MS = 10000;
  // The shape of an account id, the one notify.js uses.
  var ACCOUNT_ID_PATTERN = /^[1-9][0-9]{0,18}$/;
  // How many account ids the asked list keeps on one browser.
  var ASKED_KEEP = 8;

  var dialog = document.getElementById('changes-ask');
  var title = document.getElementById('changes-ask-title');
  var laterButton = document.getElementById('changes-ask-later');
  var allowButton = document.getElementById('changes-ask-allow');
  var wired = !!(dialog && title && laterButton && allowButton);

  var openKey = '';
  var opener = null;
  var busy = false;
  var current = null;
  var release = null;
  // Counts finished pop-ups, so a late Allow answer can tell its own has gone.
  var finished = 0;

  function attr(selector, name) {
    var node = document.querySelector(selector);
    var value = node ? node.getAttribute(name) : '';
    return typeof value === 'string' ? value : '';
  }

  // askedList reads the account ids asked on this browser, as text. Nothing
  // stored is an empty list, an older single id is a list of one, and a value
  // that is neither is null (do not open). The caller holds the try.
  function askedList() {
    var raw = localStorage.getItem(ASKED_KEY);
    if (raw === null) {
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

  // canOffer returns the push key when the pop-up may open now, else ''.
  // The stored key is written separately, just before the dialog opens.
  function canOffer() {
    try {
      if (!wired || typeof dialog.showModal !== 'function') {
        return '';
      }
      var notify = window.PinalertNotify;
      if (!notify || typeof notify.baseState !== 'function' || typeof notify.enable !== 'function' ||
        typeof notify.isMacDockApp !== 'function') {
        return '';
      }
      var key = attr('[data-push-key]', 'data-push-key');
      var account = attr('[data-account-id]', 'data-account-id');
      if (key === '' || !ACCOUNT_ID_PATTERN.test(account)) {
        return '';
      }
      var base = notify.baseState();
      if (base !== 'maybe' && base !== 'ready') {
        return '';
      }
      if (notify.isMacDockApp() === true) {
        return '';
      }
      var asked = askedList();
      if (asked === null || asked.indexOf(account) !== -1) {
        return '';
      }
      // Another box is open: a native dialog, or a box that locked the page
      // (the report form, the Flag box, the reader window). The lock makes
      // this dialog inert, so it could not be answered (CR-01). The key is not
      // written, so the next post or vote can ask.
      if (document.querySelector('dialog[open]') ||
          document.documentElement.hasAttribute('data-modal-open') ||
          (typeof dialog.closest === 'function' && dialog.closest('[inert]'))) {
        return '';
      }
      return key;
    } catch (err) {
      return '';
    }
  }

  function offer() {
    if (current) {
      return current;
    }
    var key = canOffer();
    if (key === '') {
      return Promise.resolve();
    }
    opener = document.activeElement || null;
    try {
      var list = askedList();
      list.push(attr('[data-account-id]', 'data-account-id'));
      localStorage.setItem(ASKED_KEY, JSON.stringify(list.slice(-ASKED_KEEP)));
      dialog.showModal();
    } catch (err) {
      opener = null;
      return Promise.resolve();
    }
    openKey = key;
    current = new Promise(function (resolve) {
      release = resolve;
    });
    // The question takes the first focus, as the Log out box does, so no
    // button looks chosen.
    title.focus();
    return current;
  }

  function settled() {
    return current || Promise.resolve();
  }

  function setBusy(on) {
    allowButton.disabled = on;
    laterButton.disabled = on;
    if (on) {
      dialog.setAttribute('aria-busy', 'true');
    } else {
      dialog.removeAttribute('aria-busy');
    }
  }

  // saveChanges is the one place the Changes request is built. It resolves
  // whatever happens: a refusal, a failure or a timeout says nothing here, and
  // Settings shows the switch (P-5).
  function saveChanges() {
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
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify({ changes_push: true })
        };
        if (controller) {
          init.signal = controller.signal;
          if (typeof setTimeout === 'function') {
            timer = setTimeout(function () {
              controller.abort();
            }, REQUEST_TIMEOUT_MS);
          }
        }
        fetch(ALERTS_URL, init).then(done, done);
      } catch (err) {
        done();
      }
    });
  }

  // finish closes the pop-up, gives focus back to the element that had it when
  // that element is still on the page (P-6), and resolves the wait. Nothing is
  // sent from the close event, so one answer can never produce two requests.
  // The wait is taken first and the open count moves on, so the close event
  // this raises finds nothing to settle, and a late Allow answer of this
  // pop-up (see onAllow) no longer touches the page.
  function finish() {
    var resolve = release;
    current = null;
    release = null;
    finished += 1;
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
    if (resolve) {
      resolve();
    }
  }

  function onAllow() {
    if (busy) {
      return;
    }
    var pending = null;
    try {
      pending = window.PinalertNotify.enable(openKey);
    } catch (err) {
      pending = null;
    }
    busy = true;
    setBusy(true);
    var mine = finished;
    function save() {
      try {
        return saveChanges();
      } catch (err) {
        return Promise.resolve();
      }
    }
    // The answer still turns Changes on, but it closes the pop-up only while
    // this pop-up is the one open (the browser may have closed it already).
    function done() {
      if (mine === finished) {
        finish();
      }
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

  // A browser can close the dialog by itself, without a cancel event that can be
  // prevented (IN-03). Then the wait is settled and busy cleared, so settled()
  // always resolves. Nothing is sent from here.
  function onClose() {
    if (current) {
      finish();
    }
  }

  if (wired) {
    allowButton.addEventListener('click', onAllow);
    laterButton.addEventListener('click', onLater);
    dialog.addEventListener('cancel', onCancel);
    dialog.addEventListener('close', onClose);
  }

  window.PinalertChangesAsk = {
    offer: offer,
    settled: settled
  };
})();
