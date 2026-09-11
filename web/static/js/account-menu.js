// account-menu.js — the shared account header's dropdown behaviour
// (plan 01.1-06, D-12, UI-SPEC item 14).
//
// Self-contained: this file has no dependency on the app shell's global
// store or any of the other four scripts in web/static/js/ (app.js,
// map.js, modal.js, feed.js). Plan 01.1-07's profile page loads this file
// alone, without those four, so a reference to that global or any DOM id
// those other scripts own would throw there. Everything this file needs
// is already server-rendered by web/templates/account_header.html.tmpl —
// it only toggles attributes and moves focus, never builds DOM from a
// markup string (T-01-03).
(function () {
  'use strict';

  var trigger = document.getElementById('account-menu-trigger');
  var panel = document.getElementById('account-menu');

  // Guard: on any page that renders no header (none exist yet, but a
  // future template might legitimately omit one), a missing trigger or
  // panel must return immediately rather than throw on the very first
  // DOM lookup below.
  if (!trigger || !panel) {
    return;
  }

  // FOCUSABLE_SELECTOR mirrors modal.js's own trapTab selector string
  // exactly, rather than inventing a second, subtly different one that
  // could drift out of step with it.
  var FOCUSABLE_SELECTOR = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

  var outsideClickBound = false;

  function isOpen() {
    return !panel.hidden;
  }

  // setOpen is the single function that toggles the panel's visible
  // state — aria-expanded is assigned here and nowhere else, so the
  // announced state can never disagree with the visible one.
  function setOpen(open) {
    panel.hidden = !open;
    trigger.setAttribute('aria-expanded', open ? 'true' : 'false');

    if (open) {
      if (!outsideClickBound) {
        document.addEventListener('click', onDocumentClick);
        outsideClickBound = true;
      }
    } else if (outsideClickBound) {
      document.removeEventListener('click', onDocumentClick);
      outsideClickBound = false;
    }
  }

  function focusFirstMenuItem() {
    var items = panel.querySelectorAll(FOCUSABLE_SELECTOR);
    if (items.length > 0) {
      items[0].focus();
    }
  }

  function openMenu() {
    setOpen(true);
    focusFirstMenuItem();
  }

  function closeMenu(restoreFocus) {
    if (!isOpen()) {
      return;
    }
    setOpen(false);
    if (restoreFocus) {
      trigger.focus();
    }
  }

  function onTriggerClick(e) {
    e.stopPropagation();
    if (isOpen()) {
      closeMenu(false);
    } else {
      openMenu();
    }
  }

  // onDocumentClick closes the menu on a click landing outside both the
  // trigger and the panel. Bound only while the menu is open (see
  // setOpen above) so a closed menu costs the page nothing on every
  // stray click elsewhere.
  function onDocumentClick(e) {
    if (trigger.contains(e.target) || panel.contains(e.target)) {
      return;
    }
    closeMenu(false);
  }

  // trapTab keeps Tab/Shift+Tab cycling within the open panel, wrapping
  // at both ends — the same wrap-around logic modal.js's own trapTab
  // implements, reused here rather than reinvented.
  function trapTab(e) {
    var focusable = panel.querySelectorAll(FOCUSABLE_SELECTOR);
    if (focusable.length === 0) {
      return;
    }
    var first = focusable[0];
    var last = focusable[focusable.length - 1];

    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  function onPanelKeydown(e) {
    if (e.key === 'Escape') {
      closeMenu(true);
      return;
    }
    if (e.key === 'Tab') {
      trapTab(e);
    }
  }

  trigger.addEventListener('click', onTriggerClick);
  panel.addEventListener('keydown', onPanelKeydown);
}());
