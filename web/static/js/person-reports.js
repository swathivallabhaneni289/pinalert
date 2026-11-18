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
// position, or with numbers that cannot be used, the cell is left empty (quick
// 261109-n25, owner decision 2026-11-09: fewer words) and its foot is hidden
// unless it also holds the live row's chevron. A cell with no coordinates is
// left alone.
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

  // showFoot hides the foot of a row that has nothing to show: no distance and
  // no chevron (a finished row). A live row keeps its foot for the chevron.
  function showFoot(cell, hasText) {
    var foot = cell.parentNode;
    if (!foot || typeof foot.querySelector !== 'function') {
      return;
    }
    foot.hidden = !hasText && !foot.querySelector('.person-report__chevron');
  }

  // fillDistances writes every Distance cell of the section from a position
  // {lat, lon}, or nothing when there is none or a cell has unusable numbers.
  function fillDistances(position) {
    var cells = document.querySelectorAll('.person-report__distance');
    for (var i = 0; i < cells.length; i++) {
      var cell = cells[i];
      if (cell.getAttribute('data-lat') === null && cell.getAttribute('data-lon') === null) {
        continue;
      }
      var lat = readNumber(cell, 'data-lat');
      var lon = readNumber(cell, 'data-lon');
      var text = '';
      if (position && isFinite(position.lat) && isFinite(position.lon) && isFinite(lat) && isFinite(lon)) {
        var km = greatCircleKm(position.lat, position.lon, lat, lon);
        text = cell.getAttribute('data-approx') === 'true' ? finishedWords(km) : liveWords(km);
      }
      cell.textContent = text;
      showFoot(cell, text !== '');
    }
  }

  // ---- The Being checked mark of a live row (quick 261118-saj) ----
  //
  // The server draws the mark after the label line of a Critical report people
  // in different places flagged. The row is one link, so the mark is not a
  // button: its words are already part of the link's name. Here a mouse
  // resting on it shows its one line, "Being checked.", and leaving hides it;
  // a tap or click on it shows the line instead of opening the report, until a
  // second press, Escape or a press anywhere else. At most one line is open.
  var openMark = null;

  function tipOf(mark) {
    return mark && typeof mark.querySelector === 'function' ? mark.querySelector('.person-report__check-tip') : null;
  }

  function showMark(mark, pinned) {
    if (openMark && openMark !== mark) {
      hideMark(openMark);
    }
    tipOf(mark).hidden = false;
    mark.setAttribute('data-pinned', pinned ? 'true' : 'false');
    openMark = mark;
  }

  function hideMark(mark) {
    tipOf(mark).hidden = true;
    mark.removeAttribute('data-pinned');
    if (openMark === mark) {
      openMark = null;
    }
  }

  function wireMark(mark) {
    if (!tipOf(mark)) {
      return;
    }
    mark.addEventListener('pointerenter', function (evt) {
      if (evt.pointerType === 'mouse' && tipOf(mark).hidden) {
        showMark(mark, false);
      }
    });
    mark.addEventListener('pointerleave', function (evt) {
      if (evt.pointerType === 'mouse' && mark.getAttribute('data-pinned') !== 'true') {
        hideMark(mark);
      }
    });
    mark.addEventListener('click', function (evt) {
      evt.preventDefault();
      evt.stopPropagation();
      if (!tipOf(mark).hidden && mark.getAttribute('data-pinned') === 'true') {
        hideMark(mark);
        return;
      }
      showMark(mark, true);
    });
  }

  // wireCheckMarks wires every mark the server drew and the two page wide
  // closers, once.
  function wireCheckMarks() {
    var marks = document.querySelectorAll('[data-check-mark]');
    if (!marks || marks.length === 0) {
      return;
    }
    for (var i = 0; i < marks.length; i++) {
      wireMark(marks[i]);
    }
    document.addEventListener('pointerdown', function (evt) {
      if (openMark && !(evt.target && openMark.contains(evt.target))) {
        hideMark(openMark);
      }
    }, true);
    document.addEventListener('keydown', function (evt) {
      if (evt.key === 'Escape' && openMark) {
        hideMark(openMark);
      }
    });
  }

  function init() {
    wireCheckMarks();
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
    greatCircleKm: greatCircleKm,
    wireCheckMarks: wireCheckMarks
  };
}());
