// reliable.js - window.PinalertReliable: the Only Reliable reporters tick of
// the Show menu on Nearby (Phase 3.2, NOTIF-04, D-26, 03.4 D-13, UI-SPEC
// Surface D).
//
// This is the one owner of the tick's state change and of its address word.
// show-menu.js only calls toggle, and feed.js only reads the store, so both
// stay free of address writes (TestViewStripWritesNoAddress). The address keeps
// the tick as reliable=1, replaced and never pushed, so a reload or a shared
// link keeps it; this follows the approved UI-SPEC (orchestrator decision 1)
// and differs on purpose from the Show choice, which is written nowhere. The
// tick is not remembered across visits, and leaving Nearby drops it with the
// page.
//
// The browser decides nothing about trust. The tick is a filter over rows
// already in the store (Pinalert.reportKeptByReliable reads the server's
// kept_by_reliable_filter), so a change sends no request: it draws again.
//
// SECURITY (T-03.2-17, T-03.2-08): the address word is compared with the exact
// string 1 and its value is never written into the page or a request. Every
// word written is a fixed string in this file, written through
// Pinalert.setText on nodes made with createElement.
window.PinalertReliable = (function () {
  'use strict';

  // The words of the line (P32-C18). The empty state's words are in the page.
  var LINE_WORDS = 'Showing Reliable reporters and critical reports.';
  var EVERYONE_WORDS = 'Show everyone';

  var lineEl = document.getElementById('reliable-line');
  var menuEl = document.getElementById('show-menu');
  var buttonEl = document.getElementById('show-button');
  var emptyClearEl = document.getElementById('reliable-empty-clear');

  // readFromAddress answers true only for the exact word 1 under the key
  // reliable. Any other value, another case of the key and a missing key are
  // off.
  function readFromAddress(search) {
    var params = new URLSearchParams(typeof search === 'string' ? search : '');
    return params.get('reliable') === '1';
  }

  // writeAddress puts the tick into the page address, or takes it out, and
  // touches no other parameter. The entry is replaced, so Back is not made to
  // step through toggles. history.state and the hash are kept as they are.
  function writeAddress(on) {
    if (!window.history || typeof window.history.replaceState !== 'function' || !window.location) {
      return;
    }
    var params = new URLSearchParams(window.location.search || '');
    if (on) {
      params.set('reliable', '1');
    } else {
      params.delete('reliable');
    }
    var query = params.toString();
    window.history.replaceState(
      window.history.state,
      '',
      window.location.pathname + (query === '' ? '' : '?' + query) + (window.location.hash || '')
    );
  }

  function focusShowButton() {
    if (buttonEl && typeof buttonEl.focus === 'function') {
      buttonEl.focus();
    }
  }

  // renderLine fills the line above the cards while the tick is on: the
  // sentence, then a quiet text button that turns the tick off. It is hidden
  // and emptied otherwise.
  function renderLine() {
    if (!lineEl) {
      return;
    }
    lineEl.textContent = '';
    if (!Pinalert.state.reliable) {
      lineEl.hidden = true;
      return;
    }
    var text = document.createElement('span');
    Pinalert.setText(text, LINE_WORDS);
    lineEl.appendChild(text);
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'inline-text-button';
    Pinalert.setText(button, EVERYONE_WORDS);
    button.addEventListener('click', onEveryonePress);
    lineEl.appendChild(button);
    lineEl.hidden = false;
  }

  // set applies one value. When it changes the store, the report link ends
  // like after any other change of what the list shows (CR-01), the address is
  // rewritten, the line follows, and the rows already in the store are drawn
  // again. Setting the value that is already held changes nothing.
  function set(on) {
    var value = on === true;
    if (!Pinalert.setReliable(value)) {
      return;
    }
    Pinalert.dropFocus();
    writeAddress(value);
    renderLine();
    Pinalert.redraw();
  }

  function toggle() {
    set(!Pinalert.state.reliable);
  }

  // Both Show everyone buttons (the line's and the empty state's) turn the tick
  // off, and the pressed button goes away, so focus goes to the Show button.
  function onEveryonePress() {
    set(false);
    focusShowButton();
  }

  // Mount. The address is read here, once, and handed to the store. Usually
  // that is before the first draw, but with a kept fix map.js asks for the
  // list before this module runs (Phase 3.4 plan 41), so on a slow script load
  // the first answer can already be drawn: then the store is drawn again under
  // the tick, or the list would show everyone until the next poll. A page
  // without the Show menu mounts nothing.
  if (menuEl && window.Pinalert) {
    var changedAtMount = Pinalert.setReliable(readFromAddress(window.location ? window.location.search : ''));
    renderLine();
    if (changedAtMount && Pinalert.state.hasLoaded === true) {
      Pinalert.redraw();
    }
    if (emptyClearEl) {
      emptyClearEl.addEventListener('click', onEveryonePress);
    }
  }

  return {
    readFromAddress: readFromAddress,
    set: set,
    toggle: toggle,
    renderLine: renderLine
  };
}());
