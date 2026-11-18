// install-app.js: keeps the browser's one click install offer for the Install
// app row of Settings (quick 261118-srq, owner, 7.1 UAT test 8).
//
// Chrome and Edge (on a Mac, Windows, Linux, ChromeOS or Android) fire
// beforeinstallprompt when the site can be installed. The event can come
// before the page's deferred scripts run, so this file loads in the head of
// Settings without defer and keeps the event from the start. Only Settings
// loads it, so no other page changes.
//
// What it does:
//   - Keeps the latest offer and stops the browser's own small install bar on
//     a phone, since the Install app row offers the same thing.
//   - prompt() shows the browser's own confirm box once per offer. The offer
//     is used up either way; a dismissed one can come back when the browser
//     fires the event again.
//   - Remembers an accepted choice or an appinstalled event, so the row hides.
//   - Tells every watcher (the Settings script) when any of that changes.
//
// What it never does: it writes nothing to the page, sends no request and
// stores nothing in the browser. Which row shows, and every word, belong to
// notify.js (installRow) and settings-alerts.js.
(function () {
  'use strict';

  var offer = null;
  var done = false;
  var busy = false;
  var watchers = [];

  function tell() {
    for (var i = 0; i < watchers.length; i++) {
      try {
        watchers[i]();
      } catch (err) {
        // One watcher that throws stops no other.
      }
    }
  }

  function onOffer(e) {
    if (!e || typeof e.prompt !== 'function') {
      return;
    }
    if (typeof e.preventDefault === 'function') {
      e.preventDefault();
    }
    offer = e;
    tell();
  }

  function onInstalled() {
    done = true;
    offer = null;
    tell();
  }

  // prompt shows the kept offer and resolves to 'accepted', 'dismissed' or
  // 'none' (no offer, or one is already showing). It never rejects.
  function prompt() {
    if (busy || offer === null) {
      return Promise.resolve('none');
    }
    var kept = offer;
    offer = null;
    busy = true;
    var shown;
    try {
      shown = kept.prompt();
    } catch (err) {
      busy = false;
      tell();
      return Promise.resolve('none');
    }
    var choice = kept.userChoice && typeof kept.userChoice.then === 'function' ? kept.userChoice : shown;
    return Promise.resolve(choice).then(function (answer) {
      busy = false;
      var outcome = answer && answer.outcome === 'accepted' ? 'accepted' : 'dismissed';
      if (outcome === 'accepted') {
        done = true;
      }
      tell();
      return outcome;
    }, function () {
      busy = false;
      tell();
      return 'dismissed';
    });
  }

  function watch(fn) {
    if (typeof fn === 'function') {
      watchers.push(fn);
    }
  }

  if (typeof window.addEventListener === 'function') {
    window.addEventListener('beforeinstallprompt', onOffer);
    window.addEventListener('appinstalled', onInstalled);
  }

  window.PinalertInstall = {
    hasOffer: function () { return offer !== null; },
    isDone: function () { return done; },
    isBusy: function () { return busy; },
    prompt: prompt,
    watch: watch
  };
}());
