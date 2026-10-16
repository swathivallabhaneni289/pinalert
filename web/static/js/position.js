// position.js: the one small module that reads and saves the last known device
// position in this browser (Phase 3.4, plan 10, D-27).
//
// The position stays in this browser. It is written to local storage under the
// key the Low data view already uses, with the {lat, lon, savedAt} shape that
// view writes, so the first-time flow, Nearby and Ended nearby all remember the
// same fix. It is never sent anywhere for storage: the server keeps no reader
// location.
//
// Storage can throw (private browsing, a full store, a blocked cookie setting),
// so every access is guarded: an unreadable store reads as no position, and a
// failed save is silent because the page still works without a remembered fix.
(function () {
  'use strict';

  var STORED_POSITION_KEY = 'pinalert.lowLastPosition';

  function validPair(lat, lon) {
    return typeof lat === 'number' && typeof lon === 'number' &&
      isFinite(lat) && isFinite(lon) &&
      Math.abs(lat) <= 90 && Math.abs(lon) <= 180;
  }

  // read returns {lat, lon, savedAt} or null. A stored value that is not valid
  // JSON, not finite or out of range counts as no position.
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
    var savedAt = typeof parsed.savedAt === 'number' && isFinite(parsed.savedAt) ? parsed.savedAt : null;
    return { lat: parsed.lat, lon: parsed.lon, savedAt: savedAt };
  }

  // save remembers a fix and returns true when it was stored. A pair that is
  // not a real position is refused, and a storage failure returns false.
  function save(lat, lon) {
    if (!validPair(lat, lon)) {
      return false;
    }
    try {
      window.localStorage.setItem(STORED_POSITION_KEY, JSON.stringify({ lat: lat, lon: lon, savedAt: Date.now() }));
    } catch (err) {
      return false;
    }
    return true;
  }

  window.PinalertPosition = { read: read, save: save };
}());
