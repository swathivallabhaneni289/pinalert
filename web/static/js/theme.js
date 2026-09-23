// theme.js: the in-app two-state light/dark toggle (quick task 260923-qwi,
// replacing the three-way System/Light/Dark toggle from UAT gap 6, plan
// 02-10, with a true two-state switch, per an explicit user decision).
//
// DELIBERATELY NOT DEFERRED, DELIBERATELY LOADED IN THE DOCUMENT HEAD: this
// script is meant to run while <head> is still parsing, before anything
// paints, so the stored mode is applied before first paint. Moving it to
// the end of the body, or adding a `defer`/`async` attribute to its script
// tag, reintroduces a visible flash of the wrong palette on every load,
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
// two-element list before it is ever reflected into the root attribute,
// the same discipline app.js's iconClass applies to a category read from
// the API. Anything absent, corrupt or unrecognised is treated as no
// stored preference at all and falls through to a one-time value derived
// from the operating system preference, never a live third mode.
(function () {
  'use strict';

  var STORAGE_KEY = 'pinalert_theme';

  // A true two-state list: no follow-the-OS member any more. The operating
  // system preference still seeds the very first visit, through
  // initialMode below, but only once ever, not as a standing third mode
  // that keeps being consulted on every later load.
  var MODES = ['light', 'dark'];

  // ICON_CLASSES maps the CURRENT mode to the glyph that mode shows: a sun
  // in light mode, a moon in dark mode. These values must match the
  // .auth-icon-- modifier names declared in auth.css, or the button renders
  // an empty circle with a green build.
  var ICON_CLASSES = {
    light: 'auth-icon--sun',
    dark: 'auth-icon--moon'
  };

  // ACTION_SENTENCES is keyed by the TARGET mode, not the current one: the
  // accessible name and title must describe the action the next tap
  // performs, so they are always looked up by the mode a flip would land
  // on, never the mode currently showing.
  var ACTION_SENTENCES = {
    light: 'Switch to light mode',
    dark: 'Switch to dark mode'
  };

  // readStoredMode wraps localStorage access in try/catch for the
  // privacy-mode reason votes.js's getVoterLocation documents (a browser in
  // a privacy mode can throw on storage access), and validates the stored
  // string against MODES before ever returning it (T-01-17). A missing,
  // corrupt or unrecognised value returns null, a signal that carries no
  // default of its own, rather than a live mode: initialMode below is what
  // turns that signal into an actual first-visit default.
  function readStoredMode() {
    var stored = null;
    try {
      stored = localStorage.getItem(STORAGE_KEY);
    } catch (e) {
      // Same privacy-mode/quota concern as votes.js's getVoterLocation, a
      // read that cannot happen is treated as absent, not fatal.
    }
    if (stored !== null && MODES.indexOf(stored) !== -1) {
      return stored;
    }
    return null;
  }

  // initialMode runs exactly once, during module evaluation (see below),
  // and is the only place the operating system is ever consulted. A valid
  // stored value always wins; only when readStoredMode returns null does
  // this function fall through to matchMedia. The existence check on
  // window.matchMedia is a plain guard, not a third try block: an unbound
  // call would throw in a real browser if the property happened to be
  // absent, so the guard and the call both name it, deliberately.
  function initialMode() {
    var stored = readStoredMode();
    if (stored !== null) {
      return stored;
    }
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
    return 'light';
  }

  // applyMode is the only place a value reaches the root element in this
  // file: one setAttribute call site, downstream of the validation above.
  // There is no removeAttribute branch any more: with the follow-the-OS
  // mode gone, no state ever wants the attribute absent.
  function applyMode(mode) {
    document.documentElement.setAttribute('data-theme', mode);
  }

  // persistMode mirrors applyMode: one guarded setItem call, no remove
  // branch. Guarded in try/catch for the same reason readStoredMode is.
  function persistMode(mode) {
    try {
      localStorage.setItem(STORAGE_KEY, mode);
    } catch (e) {
      // A write that fails costs only this preference's persistence for
      // the next visit, never this load's correctness: the mode already
      // applied above.
    }
  }

  // Module evaluation happens while <head> is still parsing (see the file
  // header comment), nothing has painted yet, so applying here, rather
  // than waiting for DOMContentLoaded, is what avoids the flash.
  // Persisting here too, immediately, is a deliberate change from the old
  // three-way toggle's behaviour: that version left no storage trace for a
  // reader who never touched the control. Persisting the derived value on
  // this very first evaluation is what guarantees the operating system is
  // consulted once ever, not once per visit with no memory of the last
  // answer.
  var currentMode = initialMode();
  applyMode(currentMode);
  persistMode(currentMode);

  // render applies the given mode's glyph and both accessible-name
  // attributes to the control. It is called once at wire time, to show the
  // truth on load, and again after every click, to reflect the flip.
  function render(control, icon, mode) {
    icon.classList.remove(ICON_CLASSES.light, ICON_CLASSES.dark);
    icon.classList.add(ICON_CLASSES[mode]);

    var nextMode = MODES[(MODES.indexOf(mode) + 1) % MODES.length];
    var sentence = ACTION_SENTENCES[nextMode];
    control.setAttribute('aria-label', sentence);
    control.setAttribute('title', sentence);
  }

  // wireControl looks up the control and its glyph span by id and returns
  // immediately if either is absent, the same guard account-menu.js opens
  // with, so this module is inert on the two pages that render no header
  // (the login gate, verify-outcome). Attaches one click listener that
  // advances to the next mode in MODES by index with a modulo wrap, which
  // for this two-element list is exactly a flip, keeping MODES the single
  // source of truth, then applies, persists and re-renders.
  function wireControl() {
    var control = document.getElementById('theme-toggle');
    var icon = document.getElementById('theme-toggle-icon');
    if (!control || !icon) {
      return;
    }

    render(control, icon, currentMode);

    control.addEventListener('click', function () {
      var nextIndex = (MODES.indexOf(currentMode) + 1) % MODES.length;
      currentMode = MODES[nextIndex];
      applyMode(currentMode);
      persistMode(currentMode);
      render(control, icon, currentMode);
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
