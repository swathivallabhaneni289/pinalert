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

  // ---- The glue ------------------------------------------------------------

  var PHONE_QUERY = '(max-width: 899.98px)';
  var REDUCED_QUERY = '(prefers-reduced-motion: reduce)';

  // The sheet and the map above it ease for 0.3s (phone.css). The settle runs
  // when the sheet's top transition ends, and this timer is the net for a snap
  // that moves nothing, so the map is always measured again.
  var SETTLE_MS = 350;

  // The handle's two accessible names (UI-SPEC 2b). The name says what the
  // button does next: it is the second one only at full.
  var NAME_MORE = 'Show more of the list';
  var NAME_MAP = 'Show the map again';

  var boxEl = null;
  var sheetEl = null;
  var handleEl = null;
  var handleRowEl = null;
  var headEl = null;

  var snap = 'half';
  var tops = snapTops(0);
  var active = false;
  var reduced = false;
  var remembered = null;
  var settleTimer = null;
  var relayoutFrame = null;
  var phoneQuery = null;

  // The pointer drag in flight, the touch gesture in flight, and the time up to
  // which the click that follows a drag is swallowed.
  var drag = null;
  var touch = null;
  var suppressClickUntil = 0;

  function now() {
    return window.performance && typeof window.performance.now === 'function' ? window.performance.now() : Date.now();
  }

  // measure reads the box height and works out the three tops from it.
  function measure() {
    tops = snapTops(boxEl ? boxEl.clientHeight : 0);
    return tops;
  }

  // writeTop puts the sheet top on the box. The sheet and the map both read the
  // custom property (phone.css), so one write moves both.
  function writeTop(px) {
    boxEl.style.setProperty('--sheet-top', px + 'px');
  }

  // applySnap records a position and writes the words that say it: data-snap on
  // the sheet, aria-expanded true only at full, and the handle's name.
  function applySnap(name) {
    snap = name;
    sheetEl.setAttribute('data-snap', name);
    if (handleEl) {
      handleEl.setAttribute('aria-expanded', name === 'full' ? 'true' : 'false');
      handleEl.setAttribute('aria-label', name === 'full' ? NAME_MAP : NAME_MORE);
    }
  }

  // syncMap tells the map the box above the sheet has its final size. The map
  // measures itself again once, with no animation and no pan, and brings the
  // chosen pin in when it fell under the sheet (map.js resizeToSheet). It waits
  // one frame so the CSS height has landed.
  function syncMap() {
    var run = function () {
      if (active && window.PinalertMap && typeof window.PinalertMap.resizeToSheet === 'function') {
        window.PinalertMap.resizeToSheet(tops[snap]);
      }
    };
    if (typeof window.requestAnimationFrame === 'function') {
      window.requestAnimationFrame(run);
    } else {
      run();
    }
  }

  function settle() {
    if (settleTimer !== null) {
      window.clearTimeout(settleTimer);
      settleTimer = null;
    }
    syncMap();
  }

  // withoutEasing writes a top at once, with the transitions switched off for
  // that write only.
  function withoutEasing(fn) {
    boxEl.setAttribute('data-dragging', '');
    fn();
    void sheetEl.offsetHeight;
    boxEl.removeAttribute('data-dragging');
  }

  // setSnap moves the sheet to full, half or short. It eases unless the person
  // asked for reduced motion or opts.instant is true, and it measures the map
  // again when the move has settled. It does nothing on a laptop.
  function setSnap(name, opts) {
    if (!active || ORDER.indexOf(name) === -1) {
      return snap;
    }
    measure();
    applySnap(name);
    if (settleTimer !== null) {
      window.clearTimeout(settleTimer);
      settleTimer = null;
    }
    if (reduced || (opts && opts.instant)) {
      withoutEasing(function () {
        writeTop(tops[name]);
      });
      settle();
    } else {
      writeTop(tops[name]);
      settleTimer = window.setTimeout(settle, SETTLE_MS);
    }
    return name;
  }

  function getSnap() {
    return snap;
  }

  // remember keeps the position the sheet is in, once, before the report detail
  // opens over the map and the sheet drops to short beneath it; restore puts
  // the sheet back there when the detail closes (orchestrator decision 8). A
  // second remember before a restore keeps the first.
  function remember() {
    if (active && remembered === null) {
      remembered = snap;
    }
  }

  function restore() {
    if (remembered === null) {
      return;
    }
    var back = remembered;
    remembered = null;
    setSnap(back);
  }

  // relayout recomputes the tops when the box changed size (a rotation, the
  // address bar of the browser, the keyboard) and puts the sheet on its
  // position again at once. It never runs while a finger holds the sheet.
  function relayout() {
    relayoutFrame = null;
    if (!active || !boxEl || drag !== null || (touch !== null && touch.owner === 'sheet')) {
      return;
    }
    setSnap(snap, { instant: true });
  }

  function scheduleRelayout() {
    if (relayoutFrame !== null) {
      return;
    }
    if (typeof window.requestAnimationFrame === 'function') {
      relayoutFrame = window.requestAnimationFrame(relayout);
    } else {
      relayout();
    }
  }

  // ---- Pointer drag on the handle row and the list head --------------------

  // currentTop is where the sheet is right now, mid ease included, in pixels
  // from the top of the box.
  function currentTop() {
    return sheetEl.offsetTop;
  }

  function clampTop(px) {
    return Math.max(0, Math.min(tops.short, px));
  }

  function startDrag(startY, startSnap) {
    measure();
    return { y: startY, top: currentTop(), cur: currentTop(), startSnap: startSnap, moved: false };
  }

  // moveDrag follows the finger once it has moved more than a tap.
  function moveDrag(state, y) {
    var dy = y - state.y;
    if (!state.moved) {
      if (Math.abs(dy) < TAP_PX) {
        return false;
      }
      state.moved = true;
      boxEl.setAttribute('data-dragging', '');
    }
    state.cur = clampTop(state.top + dy);
    writeTop(state.cur);
    return true;
  }

  // endDrag lets go: the sheet eases to the position the release lands on, and
  // the click that follows the release is swallowed.
  function endDrag(state) {
    if (!state.moved) {
      return;
    }
    suppressClickUntil = now() + 60;
    boxEl.removeAttribute('data-dragging');
    setSnap(nearestSnap(tops, state.cur, state.top, state.startSnap));
  }

  function onPointerDown(evt) {
    if (!active || drag !== null) {
      return;
    }
    if (evt.button !== undefined && evt.button > 0) {
      return;
    }
    if (evt.isPrimary === false) {
      return;
    }
    if (evt.target && typeof evt.target.closest === 'function' && evt.target.closest('.show-menu')) {
      return;
    }
    drag = startDrag(evt.clientY, snap);
    drag.id = evt.pointerId;
    document.addEventListener('pointermove', onPointerMove, { passive: false });
    document.addEventListener('pointerup', onPointerUp);
    document.addEventListener('pointercancel', onPointerUp);
  }

  function onPointerMove(evt) {
    if (drag === null || evt.pointerId !== drag.id) {
      return;
    }
    if (moveDrag(drag, evt.clientY) && evt.cancelable) {
      evt.preventDefault();
    }
  }

  function onPointerUp(evt) {
    if (drag === null || evt.pointerId !== drag.id) {
      return;
    }
    var finished = drag;
    drag = null;
    document.removeEventListener('pointermove', onPointerMove);
    document.removeEventListener('pointerup', onPointerUp);
    document.removeEventListener('pointercancel', onPointerUp);
    endDrag(finished);
  }

  // swallowClick drops the click that follows a drag, before it reaches the
  // handle or the Show button.
  function swallowClick(evt) {
    if (now() < suppressClickUntil) {
      evt.stopPropagation();
      evt.preventDefault();
    }
  }

  // ---- Touch on the list body ----------------------------------------------

  function insideDragZone(target) {
    return !!(target && typeof target.closest === 'function' &&
      target.closest('.sheet__handle-row, .list-head'));
  }

  function onTouchStart(evt) {
    touch = null;
    if (!active || !evt.touches || evt.touches.length !== 1 || insideDragZone(evt.target)) {
      return;
    }
    touch = startDrag(evt.touches[0].clientY, snap);
    touch.owner = null;
  }

  // onTouchMove decides who owns the gesture on its first movement, and takes
  // the gesture from the browser only when the sheet owns it. The listener is
  // not passive, so preventDefault holds (iOS Safari needs both).
  function onTouchMove(evt) {
    if (touch === null || !evt.touches || evt.touches.length !== 1) {
      return;
    }
    var y = evt.touches[0].clientY;
    if (touch.owner === null) {
      var dy = y - touch.y;
      if (Math.abs(dy) < 3) {
        return;
      }
      touch.owner = gestureOwner(snap, sheetEl.scrollTop, dy);
    }
    if (touch.owner !== 'sheet') {
      return;
    }
    if (evt.cancelable) {
      evt.preventDefault();
    }
    moveDrag(touch, y);
  }

  function onTouchEnd() {
    if (touch === null) {
      return;
    }
    var finished = touch;
    touch = null;
    if (finished.owner === 'sheet') {
      endDrag(finished);
    }
  }

  // ---- The handle and the keyboard ------------------------------------------

  function onHandleClick() {
    if (!active) {
      return;
    }
    setSnap(snap === 'full' ? 'half' : 'full');
  }

  function onHandleKey(evt) {
    if (!active) {
      return;
    }
    if (evt.key === 'ArrowUp' || evt.key === 'ArrowDown') {
      evt.preventDefault();
      setSnap(stepSnap(snap, evt.key === 'ArrowUp' ? 'up' : 'down'));
    }
  }

  // onFocusIn grows a short sheet to half when focus lands on something that
  // short sheet clips: a row, or an item of the Show menu. The handle and the
  // Show button are always in view, so they leave the sheet where it is.
  function onFocusIn(evt) {
    if (!active || snap !== 'short') {
      return;
    }
    var target = evt.target;
    if (target === handleEl || (target && target.id === 'show-button')) {
      return;
    }
    if (!keyboardFocus(target)) {
      return;
    }
    setSnap('half');
  }

  // keyboardFocus is false for a focus a press gave (a tap on a row focuses it
  // before the click), so a tap never grows the sheet that the tap is about to
  // drop to short again. A browser without :focus-visible counts every focus.
  function keyboardFocus(target) {
    if (!target || typeof target.matches !== 'function') {
      return true;
    }
    try {
      return target.matches(':focus-visible');
    } catch (e) {
      return true;
    }
  }

  // ---- Boot ----------------------------------------------------------------

  function refreshActive() {
    var phone = !!phoneQuery && phoneQuery.matches;
    if (phone === active) {
      return;
    }
    active = phone;
    if (active) {
      measure();
      setSnap('half', { instant: true });
    } else {
      remembered = null;
      boxEl.style.removeProperty('--sheet-top');
      boxEl.removeAttribute('data-dragging');
    }
  }

  function listen(query, fn) {
    if (!query) {
      return;
    }
    if (typeof query.addEventListener === 'function') {
      query.addEventListener('change', fn);
    } else if (typeof query.addListener === 'function') {
      query.addListener(fn);
    }
  }

  function init() {
    boxEl = document.getElementById('app-shell');
    sheetEl = document.getElementById('report-sheet');
    handleEl = document.getElementById('sheet-handle');
    if (!boxEl || !sheetEl) {
      return;
    }
    handleRowEl = sheetEl.querySelector('.sheet__handle-row');
    headEl = sheetEl.querySelector('.list-head');

    phoneQuery = window.matchMedia ? window.matchMedia(PHONE_QUERY) : null;
    var reducedQuery = window.matchMedia ? window.matchMedia(REDUCED_QUERY) : null;
    reduced = !!reducedQuery && reducedQuery.matches;
    listen(reducedQuery, function () {
      reduced = !!reducedQuery.matches;
    });
    listen(phoneQuery, refreshActive);

    if (handleEl) {
      handleEl.addEventListener('click', onHandleClick);
      handleEl.addEventListener('keydown', onHandleKey);
    }
    [handleRowEl, headEl].forEach(function (el) {
      if (el) {
        el.addEventListener('pointerdown', onPointerDown);
      }
    });
    sheetEl.addEventListener('click', swallowClick, true);
    sheetEl.addEventListener('touchstart', onTouchStart, { passive: true });
    sheetEl.addEventListener('touchmove', onTouchMove, { passive: false });
    sheetEl.addEventListener('touchend', onTouchEnd);
    sheetEl.addEventListener('touchcancel', onTouchEnd);
    sheetEl.addEventListener('focusin', onFocusIn);
    sheetEl.addEventListener('transitionend', function (evt) {
      if (evt.target === sheetEl && evt.propertyName === 'top' && settleTimer !== null) {
        settle();
      }
    });

    // The box is measured with a ResizeObserver, not only the window: the
    // address bar of the browser and the keyboard change the visible height
    // without a classic resize.
    if (typeof window.ResizeObserver === 'function') {
      new window.ResizeObserver(scheduleRelayout).observe(boxEl);
    }
    window.addEventListener('resize', scheduleRelayout);
    window.addEventListener('orientationchange', scheduleRelayout);
    if (window.visualViewport && typeof window.visualViewport.addEventListener === 'function') {
      window.visualViewport.addEventListener('resize', scheduleRelayout);
    }

    refreshActive();
  }

  if (typeof document !== 'undefined') {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

  var api = {
    SHEET_SHORT_HEIGHT: SHEET_SHORT_HEIGHT,
    FLICK_PX: FLICK_PX,
    TAP_PX: TAP_PX,
    snapTops: snapTops,
    nearestSnap: nearestSnap,
    stepSnap: stepSnap,
    gestureOwner: gestureOwner,
    setSnap: setSnap,
    getSnap: getSnap,
    remember: remember,
    restore: restore
  };

  return api;
}());
