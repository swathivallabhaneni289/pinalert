// app-bar.js: the two behaviours of the shared top bar (Phase 3.4, plan 04).
//
// 1. The bell list. A button named Updates opens a small dialog list under it.
//    Phase 3.2 fills the list; until then it holds one honest line and a link.
//    The X, the Escape key and a click outside all close it, and focus returns
//    to the bell. Tab stays inside the list while it is open.
// 2. The back arrow of the phone top bar. When the person came from another
//    page of this site the arrow goes back in history, so a page they were on
//    keeps its place; otherwise the link's own address is followed. No request
//    value ever builds a target (T-34-20): the address is the constant href the
//    server wrote.
//
// Self-contained like account-menu.js: it needs none of the other scripts, so
// every page with the bar can load it alone. Everything it needs is already in
// the markup; it only toggles attributes and moves focus, and never builds DOM
// from a markup string (T-01-03).
(function () {
  'use strict';

  // ---- The back arrow -----------------------------------------------------

  // sameOriginReferrer says whether the page before this one belongs to this
  // site. An empty or unreadable referrer is not the same origin.
  function sameOriginReferrer() {
    if (!document.referrer) {
      return false;
    }
    try {
      return new URL(document.referrer).origin === window.location.origin;
    } catch (err) {
      return false;
    }
  }

  // arrivedByRedirect says whether this page was reached through feed.js's
  // location.replace (a link to a finished report), which marks its address
  // from=link. That redirect keeps the page it left as the referrer but removes
  // it from history, so a same origin referrer does not mean the previous history
  // entry is a page of this site: going back could leave it (code review part 2,
  // WR-05). The arrow then follows its own address.
  function arrivedByRedirect() {
    try {
      return new URLSearchParams(window.location.search).get('from') === 'link';
    } catch (err) {
      return false;
    }
  }

  var back = document.querySelector('a.app-topbar__back');
  if (back) {
    back.addEventListener('click', function (e) {
      // A modified click (new tab, new window) keeps the link's own meaning.
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) {
        return;
      }
      if (!arrivedByRedirect() && sameOriginReferrer() && window.history.length > 1) {
        e.preventDefault();
        window.history.back();
      }
    });
  }

  // ---- The bell list ------------------------------------------------------

  var trigger = document.getElementById('bell-trigger');
  var panel = document.getElementById('bell-list');
  if (!trigger || !panel) {
    return;
  }

  // The same selector modal.js and account-menu.js use for their Tab traps.
  var FOCUSABLE_SELECTOR = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

  var closeButton = panel.querySelector('.bell-list__close');
  var listening = false;

  function isOpen() {
    return !panel.hidden;
  }

  // setOpen is the one place that changes the open state, so the announced
  // state and the visible one cannot disagree. The outside click listener is
  // bound only while the list is open, and in the capture phase, because the
  // person menu stops its own trigger click from bubbling and that must still
  // close this list.
  function setOpen(open) {
    panel.hidden = !open;
    trigger.setAttribute('aria-expanded', open ? 'true' : 'false');
    if (open && !listening) {
      document.addEventListener('click', onDocumentClick, true);
      listening = true;
    } else if (!open && listening) {
      document.removeEventListener('click', onDocumentClick, true);
      listening = false;
    }
  }

  function openList() {
    setOpen(true);
    if (closeButton) {
      closeButton.focus();
    }
  }

  function closeList(restoreFocus) {
    if (!isOpen()) {
      return;
    }
    setOpen(false);
    if (restoreFocus) {
      trigger.focus();
    }
  }

  function onTriggerClick() {
    if (isOpen()) {
      closeList(true);
    } else {
      openList();
    }
  }

  function onDocumentClick(e) {
    if (trigger.contains(e.target) || panel.contains(e.target)) {
      return;
    }
    closeList(false);
  }

  // trapTab keeps Tab and Shift+Tab inside the open list, wrapping at both
  // ends, the same logic account-menu.js uses.
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
      closeList(true);
      return;
    }
    if (e.key === 'Tab') {
      trapTab(e);
    }
  }

  trigger.addEventListener('click', onTriggerClick);
  panel.addEventListener('keydown', onPanelKeydown);
  if (closeButton) {
    closeButton.addEventListener('click', function () {
      closeList(true);
    });
  }
}());
