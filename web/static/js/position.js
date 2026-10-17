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
// offline report queue keys itself by. read returns a fix only when it was saved
// by that account; a fix saved by another account, or by a page that could not
// say whose it was (the first time flow, the Low data view, a browser from before
// this rule), is not returned and is removed, so the next person on a shared
// phone never gets the place the last one was at. Only this one key is ever
// removed: the offline report queue and the theme choice are not touched. No age
// limit is added.
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
    if (parsed.account !== account) {
      forget();
      return null;
    }
    var savedAt = typeof parsed.savedAt === 'number' && isFinite(parsed.savedAt) ? parsed.savedAt : null;
    return { lat: parsed.lat, lon: parsed.lon, savedAt: savedAt };
  }

  // save remembers a fix for the account of this page and returns true when it
  // was stored. A page that carries no account id still saves (the first time
  // flow has none) with an empty account, which no page reads back. A pair that
  // is not a real position is refused, and a storage failure returns false.
  function save(lat, lon) {
    if (!validPair(lat, lon)) {
      return false;
    }
    try {
      window.localStorage.setItem(STORED_POSITION_KEY,
        JSON.stringify({ lat: lat, lon: lon, savedAt: Date.now(), account: currentAccount() }));
    } catch (err) {
      return false;
    }
    return true;
  }

  window.PinalertPosition = { read: read, save: save };
}());
