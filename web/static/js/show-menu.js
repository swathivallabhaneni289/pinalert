// show-menu.js - window.PinalertShowMenu: the Show button and its menu above
// the Nearby list (Phase 3.4, D-13, UI-SPEC 2c).
//
// One button reads "Show: All", "Show: Confirmed" or "Show: Disputed" and opens
// a menu of three radio choices and, under a hairline, a fourth item, the Only
// Reliable reporters tick (Phase 3.2, D-26). Arrow keys move through all four,
// Enter or Space chooses and closes, Escape closes and returns focus to the
// button, and a click outside closes. The menu keyboard pattern is
// account-menu.js's.
//
// This module decides nothing about trust. A choice goes to Pinalert.setShow,
// which only assigns; whether a request is needed (All against Disputed read
// different answers) and which reports a choice keeps (Confirmed filters on the
// server's confirmed_by_people) are the store's. Choosing Confirmed against All
// sends no request, it draws the rows already in the store again.
//
// The tick is a checkbox item, not a Show choice: it calls
// PinalertReliable.toggle (reliable.js), which owns its state and its address
// word, so this file still writes nothing to the address.
//
// SECURITY (T-01-03): every word written is a fixed string in this file, and
// every write goes through Pinalert.setText.
window.PinalertShowMenu = (function () {
  'use strict';

  var SHOW_WORDS = ['all', 'confirmed', 'disputed'];

  // The button text per choice (P34-C7).
  var BUTTON_WORDS = {
    all: 'Show: All',
    confirmed: 'Show: Confirmed',
    disputed: 'Show: Disputed'
  };

  var buttonEl = document.getElementById('show-button');
  var labelEl = document.getElementById('show-button-label');
  var menuEl = document.getElementById('show-menu');

  // readAlias reads the page address once, at boot, for the old Disputed
  // link: /?view=disputed, and the older checkbox bookmark show_disputed=true
  // with no view word, both open Nearby on Show Disputed. Any other address
  // opens Show All. It is a closed comparison against fixed words, never an
  // interpolation, and it writes nothing to the address or the history.
  function readAlias(search) {
    var params = new URLSearchParams(typeof search === 'string' ? search : '');
    var word = params.get('view');
    if (word === 'disputed') {
      return 'disputed';
    }
    if (word === null && params.get('show_disputed') === 'true') {
      return 'disputed';
    }
    return 'all';
  }

  function items() {
    return menuEl ? menuEl.querySelectorAll('[role="menuitemradio"], [role="menuitemcheckbox"]') : [];
  }

  function isOpen() {
    return !!menuEl && !menuEl.hidden;
  }

  // sync brings the button text and the checked item in step with the store's
  // choice. aria-checked and the check icon follow the same attribute.
  function sync() {
    var show = window.Pinalert && Pinalert.state ? Pinalert.state.show : 'all';
    if (SHOW_WORDS.indexOf(show) === -1) {
      show = 'all';
    }
    if (labelEl) {
      Pinalert.setText(labelEl, BUTTON_WORDS[show]);
    }
    var reliable = !!(window.Pinalert && Pinalert.state && Pinalert.state.reliable);
    var list = items();
    for (var i = 0; i < list.length; i++) {
      if (list[i].getAttribute('data-reliable') !== null) {
        list[i].setAttribute('aria-checked', reliable ? 'true' : 'false');
      } else {
        list[i].setAttribute('aria-checked', list[i].getAttribute('data-show') === show ? 'true' : 'false');
      }
    }
  }

  // checkedItem is the item that takes focus when the menu opens: the chosen
  // Show choice. The tick is never it, so opening the menu lands on a radio.
  function checkedItem() {
    var list = items();
    for (var i = 0; i < list.length; i++) {
      if (list[i].getAttribute('data-show') !== null && list[i].getAttribute('aria-checked') === 'true') {
        return list[i];
      }
    }
    return list.length > 0 ? list[0] : null;
  }

  function onDocumentClick(evt) {
    var target = evt && evt.target;
    if (buttonEl && buttonEl.contains(target)) {
      return;
    }
    if (menuEl && menuEl.contains(target)) {
      return;
    }
    close(false);
  }

  // The outside click listener is bound only while the menu is open, in the
  // capture phase: the person menu stops its own trigger click from bubbling, so
  // a bubble phase listener never saw it and the Show menu stayed open under the
  // person menu (code review part 2, WR-04). app-bar.js does the same for the
  // bell list.
  var outsideBound = false;

  function open() {
    if (!menuEl || !buttonEl || isOpen()) {
      return;
    }
    // The tick can be turned off outside this menu (the Show everyone buttons
    // of reliable.js), so the items are brought in step as the menu opens.
    sync();
    menuEl.hidden = false;
    buttonEl.setAttribute('aria-expanded', 'true');
    if (!outsideBound) {
      document.addEventListener('click', onDocumentClick, true);
      outsideBound = true;
    }
    var chosen = checkedItem();
    if (chosen) {
      chosen.focus();
    }
  }

  function close(restoreFocus) {
    if (!menuEl || !buttonEl || !isOpen()) {
      return;
    }
    menuEl.hidden = true;
    buttonEl.setAttribute('aria-expanded', 'false');
    if (outsideBound) {
      document.removeEventListener('click', onDocumentClick, true);
      outsideBound = false;
    }
    if (restoreFocus) {
      buttonEl.focus();
    }
  }

  // choose applies one Show word. The store says whether the read changed:
  // when it did, the list is asked for again; when it did not (All against
  // Confirmed), the rows already in the store are drawn again. Choosing the
  // word that is already chosen changes nothing. A change ends a report link,
  // like every other change of what the list shows (CR-01).
  function choose(show) {
    if (SHOW_WORDS.indexOf(show) === -1) {
      return;
    }
    var changed = Pinalert.state.show !== show;
    close(true);
    if (!changed) {
      return;
    }
    var needsRead = Pinalert.setShow(show);
    Pinalert.dropFocus();
    sync();
    if (needsRead) {
      Pinalert.fetchReports();
    } else {
      Pinalert.redraw();
    }
  }

  // toggleReliable flips the Only Reliable reporters tick through reliable.js,
  // which changes the store, the line and the address. The menu closes with
  // focus back on the Show button, like a Show choice.
  function toggleReliable() {
    close(true);
    if (window.PinalertReliable) {
      window.PinalertReliable.toggle();
    }
    sync();
  }

  function move(step) {
    var list = items();
    if (list.length === 0) {
      return;
    }
    var at = -1;
    for (var i = 0; i < list.length; i++) {
      if (list[i] === document.activeElement) {
        at = i;
      }
    }
    var next = at === -1 ? 0 : (at + step + list.length) % list.length;
    list[next].focus();
  }

  function onMenuKeydown(evt) {
    var key = evt.key;
    if (key === 'ArrowDown') {
      evt.preventDefault();
      move(1);
    } else if (key === 'ArrowUp') {
      evt.preventDefault();
      move(-1);
    } else if (key === 'Home') {
      evt.preventDefault();
      var first = items();
      if (first.length > 0) {
        first[0].focus();
      }
    } else if (key === 'End') {
      evt.preventDefault();
      var last = items();
      if (last.length > 0) {
        last[last.length - 1].focus();
      }
    } else if (key === 'Escape') {
      evt.preventDefault();
      close(true);
    } else if (key === 'Tab') {
      close(false);
    } else if (key === 'Enter') {
      // Enter chooses here and the browser's own click is stopped, so the
      // choice is made once. Space is left to the item's click.
      evt.preventDefault();
      var target = evt.target;
      if (target && target.getAttribute && target.getAttribute('data-show')) {
        choose(target.getAttribute('data-show'));
      } else if (target && target.getAttribute && target.getAttribute('data-reliable') !== null) {
        toggleReliable();
      }
    }
  }

  function onMenuClick(evt) {
    var target = evt.target;
    while (target && target.getAttribute && !target.getAttribute('data-show') &&
           target.getAttribute('data-reliable') === null) {
      target = target.parentNode;
    }
    if (target && target.getAttribute && target.getAttribute('data-show')) {
      choose(target.getAttribute('data-show'));
    } else if (target && target.getAttribute && target.getAttribute('data-reliable') !== null) {
      toggleReliable();
    }
  }

  function onButtonClick(evt) {
    if (evt && typeof evt.stopPropagation === 'function') {
      evt.stopPropagation();
    }
    if (isOpen()) {
      close(false);
    } else {
      open();
    }
  }

  function onButtonKeydown(evt) {
    if (evt.key === 'ArrowDown' && !isOpen()) {
      evt.preventDefault();
      open();
    }
  }

  // Mount. The alias is read here, once, and handed to the store before the
  // first request goes out (the first request comes from map.js after a
  // position answer, so this module body runs first). A page without the
  // button mounts nothing.
  if (buttonEl && menuEl && window.Pinalert) {
    Pinalert.setShow(readAlias(window.location ? window.location.search : ''));
    sync();
    buttonEl.addEventListener('click', onButtonClick);
    buttonEl.addEventListener('keydown', onButtonKeydown);
    menuEl.addEventListener('keydown', onMenuKeydown);
    menuEl.addEventListener('click', onMenuClick);
  }

  return {
    readAlias: readAlias,
    open: open,
    close: close,
    choose: choose,
    sync: sync,
    isOpen: isOpen,
    BUTTON_WORDS: BUTTON_WORDS
  };
}());
