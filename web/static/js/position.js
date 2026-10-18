// position.js: the one small module that reads and saves the last known device
// position in this browser (Phase 3.4, plan 10, D-27).
//
// The position stays in this browser. It is written to local storage under the
// key the Low data view already uses, with the {lat, lon, savedAt} shape that
// view writes plus the account that saved it, so the first-time flow, Nearby and
// Ended nearby all remember the same fix. It is never sent anywhere for storage:
// the server keeps no reader location.
//
// A fix belongs to one account (code review part 2, WR-01). The page says which
// account it is for in its data-account-id attribute, the same attribute the
// offline report queue keys itself by. read returns a fix saved by that account.
// A fix saved by another account is not returned and is removed, so the next
// person on a shared phone never gets the place the last one was at. A fix with
// no owner (written by a browser from before this rule; no page writes one any
// more, since every page that saves a fix carries its account id) is taken over
// by the first account that reads it and belongs to that account from then on. Only
// this one key is ever removed: the offline report queue and the theme choice
// are not touched. No age limit is added.
//
// The same module keeps the "asked once" marker (plan 36). The browser's own
// location question is asked on its own only once per account in a browser; after
// that a page starts from the kept fix and the person refreshes with Back to my
// location. The marker is a small flag under a key that names the account, so a
// second account on a shared phone is asked for itself. A page with no account id
// sets and reads no marker, and a failing store reads as "not asked", which is
// the old behaviour (ask each time), never a broken map.
//
// Storage can throw (private browsing, a full store, a blocked cookie setting),
// so every access is guarded: an unreadable store reads as no position, and a
// failed save or removal is silent because the page still works without a
// remembered fix.
(function () {
  'use strict';

  var STORED_POSITION_KEY = 'pinalert.lowLastPosition';
  // An account id is a plain positive whole number.
  var ACCOUNT_ID_PATTERN = /^[1-9][0-9]{0,18}$/;

  function validPair(lat, lon) {
    return typeof lat === 'number' && typeof lon === 'number' &&
      isFinite(lat) && isFinite(lon) &&
      Math.abs(lat) <= 90 && Math.abs(lon) <= 180;
  }

  // currentAccount is the id of the account this page is for, or '' when the
  // page does not carry one.
  function currentAccount() {
    if (typeof document === 'undefined' || !document || typeof document.querySelector !== 'function') {
      return '';
    }
    var node = document.querySelector('[data-account-id]');
    var raw = node ? node.getAttribute('data-account-id') : '';
    return typeof raw === 'string' && ACCOUNT_ID_PATTERN.test(raw) ? raw : '';
  }

  function forget() {
    try {
      window.localStorage.removeItem(STORED_POSITION_KEY);
    } catch (err) {
      // Nothing more can be done; the fix is still not returned.
    }
  }

  // read returns {lat, lon, savedAt} or null. A stored value that is not valid
  // JSON, not finite or out of range counts as no position. So does one that
  // another account (or no account) saved: that one is also removed.
  function read() {
    var raw = null;
    try {
      raw = window.localStorage.getItem(STORED_POSITION_KEY);
    } catch (err) {
      return null;
    }
    if (!raw) {
      return null;
    }
    var parsed;
    try {
      parsed = JSON.parse(raw);
    } catch (err) {
      return null;
    }
    if (!parsed || !validPair(parsed.lat, parsed.lon)) {
      return null;
    }
    var account = currentAccount();
    if (account === '') {
      // This page cannot say whose it is, so it cannot tell whether the fix is
      // its own. It reads nothing and leaves the value for a page that can.
      return null;
    }
    var savedAt = typeof parsed.savedAt === 'number' && isFinite(parsed.savedAt) ? parsed.savedAt : null;
    if (parsed.account === undefined || parsed.account === '') {
      // No owner: a browser from before this rule wrote it (after plan 27 of
      // Phase 3.4 no page writes an ownerless value). The first account that
      // reads it takes it over, so a second account never gets it from the first.
      store(parsed.lat, parsed.lon, savedAt, account);
    } else if (parsed.account !== account) {
      forget();
      return null;
    }
    return { lat: parsed.lat, lon: parsed.lon, savedAt: savedAt };
  }

  // store writes the one value under the shared key. It answers whether it was
  // stored.
  function store(lat, lon, savedAt, account) {
    try {
      window.localStorage.setItem(STORED_POSITION_KEY,
        JSON.stringify({ lat: lat, lon: lon, savedAt: savedAt, account: account }));
    } catch (err) {
      return false;
    }
    return true;
  }

  // save remembers a fix for the account of this page and returns true when it
  // was stored. A page that carries no account id writes nothing and answers
  // false: no fix is ever stored without an owner (skeptic S-2). A pair that is
  // not a real position is refused, and a storage failure returns false.
  function save(lat, lon) {
    if (!validPair(lat, lon)) {
      return false;
    }
    var account = currentAccount();
    if (account === '') {
      return false;
    }
    return store(lat, lon, Date.now(), account);
  }

  var ASKED_KEY_PREFIX = 'pinalert.locationAsked.';

  // askedKey is the marker key of this page's account, or '' with no account.
  function askedKey() {
    var account = currentAccount();
    return account === '' ? '' : ASKED_KEY_PREFIX + account;
  }

  // askedBefore answers whether this account has already had the browser's
  // location question asked on its behalf in this browser.
  function askedBefore() {
    var key = askedKey();
    if (key === '') {
      return false;
    }
    try {
      return window.localStorage.getItem(key) === '1';
    } catch (err) {
      return false;
    }
  }

  // markAsked records that the question was asked. It answers whether it was
  // stored; a failure is silent.
  function markAsked() {
    var key = askedKey();
    if (key === '') {
      return false;
    }
    try {
      window.localStorage.setItem(key, '1');
    } catch (err) {
      return false;
    }
    return true;
  }

  // permissionState calls back once with 'granted', 'denied', 'prompt' or
  // 'unknown' (no Permissions API, as in much of Safari, or any failure).
  function permissionState(callback) {
    var finished = false;
    function finish(state) {
      if (!finished) {
        finished = true;
        callback(state);
      }
    }
    try {
      if (typeof navigator === 'undefined' || !navigator || !navigator.permissions ||
          typeof navigator.permissions.query !== 'function') {
        finish('unknown');
        return;
      }
      navigator.permissions.query({ name: 'geolocation' }).then(function (status) {
        var state = status && status.state;
        finish(state === 'granted' || state === 'denied' || state === 'prompt' ? state : 'unknown');
      }, function () {
        finish('unknown');
      });
    } catch (err) {
      finish('unknown');
    }
  }

  // startMode decides, for a page that wants the device position at load, what
  // it may do. It calls back once with:
  //   'silent' when the browser has already said yes: a fresh fix is read and no
  //            question is shown;
  //   'reuse'  when the question was asked before and the answer is not yes: the
  //            page starts from the kept fix and does not ask again;
  //   'ask'    the first time: the question is asked now, and the marker is set
  //            before it is, so a dismissed question still counts as asked.
  function startMode(callback) {
    permissionState(function (state) {
      if (state === 'granted') {
        callback('silent');
        return;
      }
      if (askedBefore()) {
        callback('reuse');
        return;
      }
      markAsked();
      callback('ask');
    });
  }

  window.PinalertPosition = {
    read: read,
    save: save,
    askedBefore: askedBefore,
    markAsked: markAsked,
    permissionState: permissionState,
    startMode: startMode
  };
}());
