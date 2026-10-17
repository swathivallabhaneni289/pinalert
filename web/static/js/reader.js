// reader.js: the reader of the Disclaimer and the Policy (Phase 3.4, plan 08,
// UI-SPEC Surface 8c, D-31).
//
// It has two jobs and does either one only on a page that has the markup for it.
//
//   The window. Nearby and Settings carry the reader as a window
//   (reader_dialog.html.tmpl). A link or button marked data-open-reader with
//   disclaimer or policy opens it: the title becomes Disclaimer or Policy, the
//   box scrolls to the top (Disclaimer) or so the Policy heading is at its top
//   (Policy), and focus moves to the title. The X, Escape and a click on the
//   dark backdrop close it, and focus goes back to the link that opened it. The
//   page behind is locked while it is open. Without script the openers are plain
//   links to /disclaimer and /policy#policy and still work.
//
//   The standalone page (reader.html.tmpl, /disclaimer and /policy). The X goes
//   back when the visitor came from this site and otherwise stays a plain link
//   to / (T-34-37). /policy opens with the box scrolled so the Policy heading is
//   at the top. A part link scrolls the box and moves the current mark.
//
// What it never does: it never parses markup (every string it sets goes through
// textContent), and it only moves the box itself, never the page.
(function () {
  'use strict';

  var TITLES = { disclaimer: 'Disclaimer', policy: 'Policy' };
  // The same selector modal.js, account-menu.js and settings.js use for their
  // Tab traps.
  var FOCUSABLE_SELECTOR = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

  function el(id) {
    return document.getElementById(id);
  }

  // scrollBoxTo puts the heading with the given id at the top of the box. The
  // box scrolls, never the page behind it. The distance is measured from the
  // rectangles so it does not depend on which element is the offset parent.
  function scrollBoxTo(box, headingId) {
    var heading = el(headingId);
    if (!box || !heading || headingId === 'disclaimer') {
      if (box) {
        box.scrollTop = 0;
      }
      return;
    }
    box.scrollTop = heading.getBoundingClientRect().top - box.getBoundingClientRect().top + box.scrollTop;
  }

  // ---- The window ----------------------------------------------------------

  var win = el('reader-window');
  if (win) {
    initWindow(win);
  }

  function initWindow(win) {
    var title = el('reader-title');
    var closeButton = el('reader-close');
    var box = el('reader-box');
    if (!title || !closeButton || !box) {
      return;
    }
    title.setAttribute('tabindex', '-1');

    var opener = null;
    var lockedNodes = [];
    var usedAppLock = false;

    // The app page lock walks from the window to the body and silences every
    // sibling. A page without app.js falls back to a small local lock.
    function lockPage() {
      unlockPage();
      if (window.Pinalert && typeof window.Pinalert.lockPage === 'function') {
        window.Pinalert.lockPage(win);
        usedAppLock = true;
        return;
      }
      usedAppLock = false;
      var kids = Array.prototype.slice.call(document.body.children);
      for (var i = 0; i < kids.length; i++) {
        var node = kids[i];
        if (node === win || node.tagName === 'SCRIPT') {
          continue;
        }
        if ('inert' in node) {
          if (node.inert === true) {
            continue;
          }
          node.inert = true;
          lockedNodes.push({ node: node, attr: 'inert' });
        } else {
          if (node.getAttribute('aria-hidden') === 'true') {
            continue;
          }
          node.setAttribute('aria-hidden', 'true');
          lockedNodes.push({ node: node, attr: 'aria-hidden' });
        }
      }
      document.documentElement.setAttribute('data-modal-open', 'true');
    }

    function unlockPage() {
      if (usedAppLock) {
        usedAppLock = false;
        if (window.Pinalert && typeof window.Pinalert.unlockPage === 'function') {
          window.Pinalert.unlockPage();
        }
        return;
      }
      for (var i = 0; i < lockedNodes.length; i++) {
        var entry = lockedNodes[i];
        if (entry.attr === 'inert') {
          entry.node.inert = false;
          entry.node.removeAttribute('inert');
        } else {
          entry.node.removeAttribute('aria-hidden');
        }
      }
      if (lockedNodes.length > 0) {
        lockedNodes = [];
        document.documentElement.removeAttribute('data-modal-open');
      }
    }

    function isOpen() {
      return !win.hidden;
    }

    function open(part, from) {
      var which = part === 'policy' ? 'policy' : 'disclaimer';
      opener = from || null;
      title.textContent = TITLES[which];
      win.hidden = false;
      lockPage();
      // The scroll is set after the window is shown, so it has a layout.
      scrollBoxTo(box, which);
      title.focus();
    }

    function close() {
      if (!isOpen()) {
        return;
      }
      win.hidden = true;
      unlockPage();
      var back = opener;
      opener = null;
      if (back && typeof back.focus === 'function') {
        back.focus();
      }
    }

    // tabStops lists what Tab can reach in the open window, in page order: the
    // X, the text box, and every link inside the text (the Call 112 phone link
    // and the contact email links). It is read from the live window each time,
    // so a link in the text is never left out (code review part 2, WR-02). The
    // title (tabindex -1) is not a stop, and nothing inside a hidden part is.
    function tabStops() {
      var found = win.querySelectorAll(FOCUSABLE_SELECTOR);
      var list = [];
      for (var i = 0; i < found.length; i++) {
        if (!found[i].closest('[hidden]')) {
          list.push(found[i]);
        }
      }
      return list;
    }

    // Tab stays inside the window: the X, the text box, the links in the text,
    // then round again.
    function keepFocusInside(event) {
      var list = tabStops();
      if (list.length === 0) {
        return;
      }
      var first = list[0];
      var last = list[list.length - 1];
      var active = document.activeElement;
      if (event.shiftKey && (active === first || active === title || !win.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && (active === last || !win.contains(active))) {
        event.preventDefault();
        first.focus();
      }
    }

    closeButton.addEventListener('click', close);

    win.addEventListener('click', function (event) {
      // Only a click on the dark backdrop itself closes it; a click inside the
      // panel never reaches here as the backdrop (D-31).
      if (event.target === win) {
        close();
      }
    });

    win.addEventListener('keydown', function (event) {
      if (event.key === 'Escape') {
        event.preventDefault();
        close();
      } else if (event.key === 'Tab') {
        keepFocusInside(event);
      }
    });

    // One click handler for every opener on the page, so a link added later
    // works too. A click with a modifier key, or a middle click, is left to the
    // browser, which opens the plain link in a new tab.
    document.addEventListener('click', function (event) {
      if (event.button > 0 || event.ctrlKey || event.metaKey || event.shiftKey) {
        return;
      }
      for (var node = event.target; node; node = node.parentNode) {
        var part = node.getAttribute ? node.getAttribute('data-open-reader') : null;
        if (part === 'disclaimer' || part === 'policy') {
          event.preventDefault();
          open(part, node);
          return;
        }
      }
    });

    window.PinalertReader = { open: open, close: close };
  }

  // ---- The standalone page -------------------------------------------------

  var page = el('reader-page');
  if (page) {
    initPage(page);
  }

  function initPage(page) {
    var box = el('reader-box');
    var closeLink = el('reader-close-link');
    var partDisclaimer = el('reader-part-disclaimer');
    var partPolicy = el('reader-part-policy');
    if (!box) {
      return;
    }

    // The X goes back only when the visitor came from this site and has
    // somewhere to go back to; otherwise it stays the plain link to / (T-34-37).
    function cameFromHere() {
      try {
        return new URL(document.referrer).origin === window.location.origin;
      } catch (e) {
        return false;
      }
    }

    if (closeLink) {
      closeLink.addEventListener('click', function (event) {
        if (window.history.length > 1 && cameFromHere()) {
          event.preventDefault();
          window.history.back();
        }
      });
    }

    function markCurrent(part) {
      if (partDisclaimer) {
        if (part === 'disclaimer') {
          partDisclaimer.setAttribute('aria-current', 'page');
        } else {
          partDisclaimer.removeAttribute('aria-current');
        }
      }
      if (partPolicy) {
        if (part === 'policy') {
          partPolicy.setAttribute('aria-current', 'page');
        } else {
          partPolicy.removeAttribute('aria-current');
        }
      }
    }

    function show(part) {
      scrollBoxTo(box, part);
      markCurrent(part);
    }

    if (partDisclaimer) {
      partDisclaimer.addEventListener('click', function (event) {
        event.preventDefault();
        show('disclaimer');
      });
    }
    if (partPolicy) {
      partPolicy.addEventListener('click', function (event) {
        event.preventDefault();
        show('policy');
      });
    }

    // /policy, or any address that ends in #policy, opens at the Policy part.
    if (page.getAttribute('data-reader-page') === 'policy' || window.location.hash === '#policy') {
      show('policy');
    }
  }
})();
