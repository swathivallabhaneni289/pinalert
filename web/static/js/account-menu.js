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

  var MAX_HEIGHT_PROPERTY = '--account-menu-max-height';

  // The open panel sits inside the sticky top chrome, so the page cannot
  // scroll it into view: it has to fit the screen and scroll inside itself.
  // Its height limit is measured from its real top edge, because the bar token
  // underestimates the bar at larger text. The floor is two 44px items, so a
  // very short screen still shows a usable pair and scrolls the rest.

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
        window.addEventListener('resize', measurePanel);
        window.addEventListener('orientationchange', measurePanel);
        outsideClickBound = true;
      }
    } else if (outsideClickBound) {
      document.removeEventListener('click', onDocumentClick);
      window.removeEventListener('resize', measurePanel);
      window.removeEventListener('orientationchange', measurePanel);
      outsideClickBound = false;
    }
  }

  // measurePanel limits the open panel to the space between its own top edge
  // and the bottom of the screen, less one medium space. The browser resolves
  // the tokens, and the max keeps the floor of two touch targets. The panel
  // must be unhidden to have a box, so openMenu calls this after setOpen.
  function measurePanel() {
    var top = panel.getBoundingClientRect().top;
    var room = Math.floor(window.innerHeight - top);
    panel.style.setProperty(
      MAX_HEIGHT_PROPERTY,
      'max(calc(' + room + 'px - var(--space-md)), calc(var(--touch-target-min) * 2))'
    );
  }

  // onPanelFocusIn keeps the focused item visible inside the scrolling panel.
  function onPanelFocusIn(e) {
    if (e.target !== panel && typeof e.target.scrollIntoView === 'function') {
      e.target.scrollIntoView({ block: 'nearest' });
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
    measurePanel();
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
  panel.addEventListener('focusin', onPanelFocusIn);
}());
