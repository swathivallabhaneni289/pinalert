// sheet.js - window.PinalertSheet: the phone list as a sheet over the map
// (Phase 3.4, D-16).
//
// Below 900px the Nearby list is a sheet that sits over the map with three
// positions: short (the map almost whole), half (the default, and the position
// after every load) and full (the list right under the top bar). This file has
// two parts. The first is pure: where the three positions sit for a box
// height, which position a release lands on, what the arrow keys do and who
// owns a gesture (the sheet or the list). It reads no element, so node can
// test it (web/sheet_logic_test.mjs). The second part is the glue that moves
// the real sheet, and it stays idle on a page without one.
//
// SCOPE: presentation only. Nothing here reads a report, a count or a trust
// word. The sheet never scrolls the page and never moves the document: the one
// element it scrolls is the sheet itself (rule: no scroll into view call, no
// window scroll call).
window.PinalertSheet = (function () {
  'use strict';

  // The sheet is 208px high at its short position, a number snapped from the
  // mockup's 210 (UI-SPEC 2b and its departure note). Half is 45 percent of the
  // box down from the top, full is the top of the box.
  var SHEET_SHORT_HEIGHT = 208;
  var HALF_FRACTION = 0.45;

  // A release within this many pixels of where the drag began stays on the
  // starting position; a longer move that would still land there moves one
  // step the way it went, a flick (UI-SPEC 2b).
  var FLICK_PX = 32;

  // A press that moves less than this is a tap, not a drag.
  var TAP_PX = 6;

  var ORDER = ['full', 'half', 'short'];

  // ---- The pure part -------------------------------------------------------

  // number returns a finite non negative number, else 0.
  function number(value) {
    return typeof value === 'number' && isFinite(value) && value > 0 ? value : 0;
  }

  // snapTops returns the top of the sheet, in pixels from the top of its box,
  // at each of the three positions. Full is 0, half is 45 percent of the box
  // rounded, short leaves SHEET_SHORT_HEIGHT of sheet and is never above the
  // top of the box. In a box too small to show all three, half comes down to
  // short so the order full, half, short always holds.
  function snapTops(boxHeight) {
    var height = number(boxHeight);
    var short = Math.max(0, height - SHEET_SHORT_HEIGHT);
    var half = Math.min(Math.round(HALF_FRACTION * height), short);
    return { full: 0, half: half, short: short };
  }

  // nearestSnap names the position a release lands on. It is the position whose
  // top is closest to where the sheet was let go. When that is the position the
  // drag began on and the sheet moved more than FLICK_PX, it is a flick: the
  // sheet moves one position the way the finger went, never past full or short.
  function nearestSnap(tops, currentTop, startTop, startSnap) {
    var best = 'half';
    var bestDistance = Infinity;
    for (var i = 0; i < ORDER.length; i++) {
      var distance = Math.abs(tops[ORDER[i]] - currentTop);
      if (distance < bestDistance) {
        bestDistance = distance;
        best = ORDER[i];
      }
    }
    if (best === startSnap && Math.abs(currentTop - startTop) > FLICK_PX) {
      return stepSnap(startSnap, currentTop < startTop ? 'up' : 'down');
    }
    return best;
  }

  // stepSnap is one step from a position: up goes toward full, down toward
  // short, and the ends stay where they are. A position it does not know is
  // treated as half.
  function stepSnap(snap, direction) {
    var index = ORDER.indexOf(snap);
    if (index === -1) {
      index = 1;
    }
    if (direction === 'up') {
      index -= 1;
    } else if (direction === 'down') {
      index += 1;
    }
    return ORDER[Math.max(0, Math.min(ORDER.length - 1, index))];
  }

  // gestureOwner says who a touch gesture belongs to once its first movement
  // is known. deltaY is the finger's move: below 0 is the finger going up.
  //   - the finger up: the sheet owns it while the sheet is not full (it grows
  //     to full first); at full the list scrolls;
  //   - the finger down: the list owns it while it is scrolled below its top
  //     (it scrolls back first); at the top of the list the sheet owns it (at
  //     full it shrinks back to half);
  //   - no movement: the list.
  function gestureOwner(snap, scrollTop, deltaY) {
    if (deltaY < 0) {
      return snap === 'full' ? 'list' : 'sheet';
    }
    if (deltaY > 0) {
      return scrollTop > 0 ? 'list' : 'sheet';
    }
    return 'list';
  }

  var api = {
    SHEET_SHORT_HEIGHT: SHEET_SHORT_HEIGHT,
    FLICK_PX: FLICK_PX,
    TAP_PX: TAP_PX,
    snapTops: snapTops,
    nearestSnap: nearestSnap,
    stepSnap: stepSnap,
    gestureOwner: gestureOwner
  };

  return api;
}());
