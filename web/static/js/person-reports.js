// person-reports.js - window.PinalertPersonReports: the distance cells of the
// Recent reports section of a profile (Phase 9 plan 11, D-06, D-07, UI-SPEC
// Surface 5).
//
// The rows are drawn by the server, the way My activity draws its rows. This
// script only fills each Distance cell, in the browser, from the position
// Nearby last kept (PinalertPosition.read), exactly as My activity does. The
// position is read here and measured here: it is never sent anywhere, so the
// server learns no reader location.
//
// A live report's cell carries the exact point the map already shows and reads
// "{d} km". A finished report's cell carries only the coarse cell centre the
// server wrote (data-approx is true) and reads "About {d} km", at least 1, so
// the reader is never given a finished report's exact place. With no known
// position, every cell reads Unknown. A cell with no coordinates is left alone.
//
// The distance is for display only. It is never an input to a vote, a count or
// any trust figure.
//
// SECURITY: every word reaches the page through textContent. No markup is
// parsed here.
window.PinalertPersonReports = (function () {
  'use strict';

  // The earth's mean radius in kilometres, for the great circle distance.
  var EARTH_RADIUS_KM = 6371;
  var UNKNOWN = 'Unknown';

  // greatCircleKm is the haversine distance between two points, in the arctan
  // form of the same formula ended-nearby.js uses.
  function greatCircleKm(lat1, lon1, lat2, lon2) {
    var rad = Math.PI / 180;
    var dLat = (lat2 - lat1) * rad;
    var dLon = (lon2 - lon1) * rad;
    var a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
      Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
    a = Math.min(1, Math.max(0, a));
    return 2 * EARTH_RADIUS_KM * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  }

  // liveWords is the words of a live cell: under 100 m below a tenth of a
  // kilometre, one decimal under 10 km, a whole number from there.
  function liveWords(km) {
    if (km < 0.1) {
      return 'under 100 m';
    }
    if (km < 9.95) {
      return km.toFixed(1) + ' km';
    }
    return Math.round(km) + ' km';
  }

  // finishedWords is the words of a finished cell: about, and a whole number of
  // at least 1, the same words the server writes for the JSON route.
  function finishedWords(km) {
    return 'About ' + Math.max(1, Math.round(km)) + ' km';
  }

  function readNumber(cell, name) {
    var raw = cell.getAttribute(name);
    if (raw === null || raw === '') {
      return NaN;
    }
    return Number(raw);
  }

  // fillDistances writes every Distance cell of the section from a position
  // {lat, lon}, or Unknown when there is none or a cell has unusable numbers.
  function fillDistances(position) {
    var cells = document.querySelectorAll('.person-report__distance');
    for (var i = 0; i < cells.length; i++) {
      var cell = cells[i];
      if (cell.getAttribute('data-lat') === null && cell.getAttribute('data-lon') === null) {
        continue;
      }
      var lat = readNumber(cell, 'data-lat');
      var lon = readNumber(cell, 'data-lon');
      var text = UNKNOWN;
      if (position && isFinite(position.lat) && isFinite(position.lon) && isFinite(lat) && isFinite(lon)) {
        var km = greatCircleKm(position.lat, position.lon, lat, lon);
        text = cell.getAttribute('data-approx') === 'true' ? finishedWords(km) : liveWords(km);
      }
      cell.textContent = text;
    }
  }

  function init() {
    var kept = null;
    if (window.PinalertPosition && typeof window.PinalertPosition.read === 'function') {
      kept = window.PinalertPosition.read();
    }
    fillDistances(kept);
  }

  if (typeof document !== 'undefined' && document.querySelector && document.querySelector('.person-report__distance')) {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

  return {
    fillDistances: fillDistances,
    greatCircleKm: greatCircleKm
  };
}());
