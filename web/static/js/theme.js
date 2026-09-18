// theme.js — the in-app System/Light/Dark toggle (UAT gap 6, plan 02-10).
//
// DELIBERATELY NOT DEFERRED, DELIBERATELY LOADED IN THE DOCUMENT HEAD: this
// script is meant to run while <head> is still parsing, before anything
// paints, so the stored mode is applied before first paint. Moving it to
// the end of the body, or adding a `defer`/`async` attribute to its script
// tag, reintroduces a visible flash of the wrong palette on every load —
// the whole reason this feature exists. See
// TestThemeScriptLoadsBeforeFirstPaintOnEveryFullPage
// (web/theme_contract_test.go), which fails the build if that ever
// regresses.
//
// Self-contained, mirroring account-menu.js's own contract: this module has
// no dependency on the app shell's global client store or any of the other
// four app-shell scripts, because it loads on the login gate and
// verify-outcome pages, where none of them exist.
//
// SECURITY (T-01-17): the stored mode is validated against a fixed
// three-element list before it is ever reflected into the root attribute —
// the same discipline app.js's iconClass applies to a category from the
// API. Anything absent, corrupt or unrecognised means follow-the-OS.
(function () {
  'use strict';

  var STORAGE_KEY = 'pinalert_theme';

  // Follow-the-OS mode is listed first: it is both this fixed list's
  // default member and the mode the app ships in before this module ever
  // runs.
  var MODES = ['system', 'light', 'dark'];

  var LABELS = {
    system: 'System',
    light: 'Light',
    dark: 'Dark'
  };

  // readStoredMode wraps localStorage access in try/catch for the
  // privacy-mode reason votes.js's getVoterLocation documents (a browser in
  // a privacy mode can throw on storage access), and validates the stored
  // string against MODES before ever returning it (T-01-17): anything
  // absent, corrupt or unrecognised returns the follow-the-OS mode.
  function readStoredMode() {
    var stored = null;
    try {
      stored = localStorage.getItem(STORAGE_KEY);
    } catch (e) {
      // Same privacy-mode/quota concern as votes.js's getVoterLocation — a
      // read that cannot happen is treated as absent, not fatal. A fresh
      // follow-the-OS default follows below.
    }
    if (stored !== null && MODES.indexOf(stored) !== -1) {
      return stored;
    }
    return MODES[0];
  }

  // applyMode is the only place a value reaches the DOM in this file: one
  // setAttribute call site and one removeAttribute call site, both
  // downstream of readStoredMode's validation above. The follow-the-OS mode
  // removes the attribute entirely, letting main.css's own dark-preference
  // media block take over; any other mode sets it, which main.css's
  // higher-specificity data-theme override blocks read.
  function applyMode(mode) {
    if (mode === MODES[0]) {
      document.documentElement.removeAttribute('data-theme');
    } else {
      document.documentElement.setAttribute('data-theme', mode);
    }
  }

  // persistMode mirrors applyMode's own branch: the follow-the-OS mode
  // removes the stored key rather than writing it, so a reader who never
  // touches the control leaves no trace at all. Guarded in try/catch for
  // the same reason readStoredMode is.
  function persistMode(mode) {
    try {
      if (mode === MODES[0]) {
        localStorage.removeItem(STORAGE_KEY);
      } else {
        localStorage.setItem(STORAGE_KEY, mode);
      }
    } catch (e) {
      // A write that fails costs only this preference's persistence for the
      // next visit, never this load's correctness — the mode already
      // applied above.
    }
  }

  // Module evaluation happens while <head> is still parsing (see the file
  // header comment) — nothing has painted yet, so applying here, rather
  // than waiting for DOMContentLoaded, is what avoids the flash.
  var currentMode = readStoredMode();
  applyMode(currentMode);

  // wireControl looks up the control and its label span by id and returns
  // immediately if either is absent — the same guard account-menu.js opens
  // with — so this module is inert on the two pages that render no header
  // (the login gate, verify-outcome). Writes the current label directly via
  // a textContent assignment, never routed through the shared store's own
  // text helper, which does not exist on those two pages. Attaches one
  // click listener that advances to the next mode in MODES, wrapping
  // around, then applies, persists and re-labels.
  function wireControl() {
    var control = document.getElementById('theme-toggle');
    var label = document.getElementById('theme-toggle-label');
    if (!control || !label) {
      return;
    }

    label.textContent = LABELS[currentMode];

    control.addEventListener('click', function () {
      var nextIndex = (MODES.indexOf(currentMode) + 1) % MODES.length;
      currentMode = MODES[nextIndex];
      applyMode(currentMode);
      persistMode(currentMode);
      label.textContent = LABELS[currentMode];
    });
  }

  // Guarded with a document.readyState check so this module still works if
  // it is ever moved later in the document than the head.
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', wireControl);
  } else {
    wireControl();
  }
}());
