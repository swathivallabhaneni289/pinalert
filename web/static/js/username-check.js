// username-check.js: the one live username check (Phase 7.1, plan 13, D-07,
// UI-SPEC Surface 7 "The live check"). Step 4 of the first-time flow uses it
// now and the Username window of Settings uses it too (plan 15), so the two
// places never differ.
//
// window.PinalertUsernameCheck.attach(input, line, options) wires a text input
// and a status line, and returns a controller:
//
//   isGood()              true only when the name can be used: the server said
//                         available, or (mode 'signup' only) yours, since an
//                         account filling in step 4 may keep the name it has.
//                         Never true for wait, taken, reserved, invalid, a failed
//                         check or a save refusal.
//   state()               one of rule, checking, available, taken, invalid,
//                         reserved, yours, wait, failed or refused (a refusal
//                         shown by showSaveError; it stays until the person types).
//   name()                the folded value in the input.
//   showSaveError(text)   writes a refusal that came back when the name was saved
//                         (the screening, a lost race) into the line with the bad
//                         look. isGood() is false until the value changes.
//   refresh()             judges the value now in the input, for a value set by
//                         the page and not by typing (Settings opens its window
//                         with the stored name; step 4 is drawn with one).
//
// options: {mode: 'signup' | 'settings' (default 'signup'), debounceMs (300),
// onChange(state, controller), setTimer, clearTimer, fetchFn}. The timers and the
// fetch default to the window's own and exist so tests can drive them.
//
// What it does while a person types:
//   - capitals become lowercase, a typed leading @ is dropped (the page draws its
//     own @ beside the input), and spaces round the value are removed;
//   - the local rule shows at once and asks nothing: 3 to 20 letters, numbers,
//     dots or underscores, at least one letter, no full stop at the start or the
//     end, no two in a row (the server is the authority; this only saves a trip);
//   - a value that passes shows Checking and, DEBOUNCE ms after the last
//     keystroke, asks GET /api/username/check?u=. A late answer for an older value
//     is ignored.
//
// The only words held here are the rule, the checking line and the offline line;
// every other sentence is the server's own message. The status line is written
// with textContent and only its class changes (the icon is drawn by CSS from the
// class); its role and aria-live belong to the template and are never touched.
(function () {
  'use strict';

  var CHECK_URL = '/api/username/check?u=';
  var RULE_MESSAGE = 'Use 3 to 20 letters, numbers, dots or underscores.';
  var CHECKING_MESSAGE = 'Checking\u2026';
  var FAILED_MESSAGE = 'Couldn\'t check that username. Try again.';
  var DEFAULT_DEBOUNCE_MS = 300;
  var MAX_SENTENCE = 200;

  var BASE_CLASS = 'username-status';
  var GOOD_CLASS = 'username-status--good';
  var BAD_CLASS = 'username-status--bad';
  var WARN_CLASS = 'username-status--warn';

  // The look of each state: the class that draws the icon and the tone, or ''.
  var LOOK = {
    rule: '',
    checking: '',
    available: GOOD_CLASS,
    taken: BAD_CLASS,
    invalid: BAD_CLASS,
    reserved: BAD_CLASS,
    yours: '',
    wait: WARN_CLASS,
    failed: BAD_CLASS,
    refused: BAD_CLASS
  };

  // The states the server may answer with.
  var SERVER_STATES = ['available', 'taken', 'invalid', 'reserved', 'yours', 'wait'];

  var SHAPE = /^[a-z0-9_.]{3,20}$/;

  // passesRule is the local rule on a folded value.
  function passesRule(value) {
    if (!SHAPE.test(value)) {
      return false;
    }
    if (!/[a-z]/.test(value)) {
      return false;
    }
    if (value.charAt(0) === '.' || value.charAt(value.length - 1) === '.') {
      return false;
    }
    return value.indexOf('..') === -1;
  }

  // fold turns what was typed into the name that is kept: no leading @, no
  // surrounding spaces, lowercase.
  function fold(raw) {
    return String(raw === null || raw === undefined ? '' : raw).replace(/^\s+|\s+$/g, '').replace(/^@+/, '').toLowerCase();
  }

  function attach(input, line, options) {
    var opts = options || {};
    var mode = opts.mode === 'settings' ? 'settings' : 'signup';
    var debounceMs = typeof opts.debounceMs === 'number' ? opts.debounceMs : DEFAULT_DEBOUNCE_MS;
    var setTimer = typeof opts.setTimer === 'function' ? opts.setTimer : function (fn, ms) { return window.setTimeout(fn, ms); };
    var clearTimer = typeof opts.clearTimer === 'function' ? opts.clearTimer : function (id) { window.clearTimeout(id); };
    var fetchFn = typeof opts.fetchFn === 'function' ? opts.fetchFn : function (target, init) { return window.fetch(target, init); };
    var onChange = typeof opts.onChange === 'function' ? opts.onChange : null;

    var current = 'rule';
    // Bumped on every judgement, so an answer for an older value is dropped.
    var seq = 0;
    var timer = null;

    function show(state, message) {
      current = state;
      line.textContent = message;
      var look = LOOK[state];
      line.className = look === '' ? BASE_CLASS : BASE_CLASS + ' ' + look;
      if (onChange !== null) {
        onChange(state, controller);
      }
    }

    function cancelPending() {
      if (timer !== null) {
        clearTimer(timer);
        timer = null;
      }
    }

    function sentenceOr(body, fallback) {
      var sent = body && body.message;
      if (typeof sent === 'string' && sent !== '' && sent.length <= MAX_SENTENCE) {
        return sent;
      }
      return fallback;
    }

    function ask(value, mine) {
      var call;
      try {
        call = fetchFn(CHECK_URL + encodeURIComponent(value), { credentials: 'same-origin', headers: { Accept: 'application/json' } });
      } catch (err) {
        call = Promise.reject(err);
      }
      Promise.resolve(call).then(function (res) {
        if (!res || res.status !== 200) {
          return null;
        }
        return res.json().then(function (body) { return body; }, function () { return null; });
      }).then(function (body) {
        if (mine !== seq) {
          return;
        }
        if (!body || typeof body.state !== 'string' || SERVER_STATES.indexOf(body.state) === -1) {
          show('failed', FAILED_MESSAGE);
          return;
        }
        show(body.state, sentenceOr(body, RULE_MESSAGE));
      }, function () {
        if (mine !== seq) {
          return;
        }
        show('failed', FAILED_MESSAGE);
      });
    }

    // judge looks at the value in the input: folds it, shows the local rule, or
    // shows Checking and starts the pause before the request.
    function judge() {
      seq += 1;
      cancelPending();
      var folded = fold(input.value);
      if (input.value !== folded) {
        input.value = folded;
      }
      if (folded === '') {
        show('rule', RULE_MESSAGE);
        return;
      }
      if (!passesRule(folded)) {
        // Something is typed but breaks the rule: the same words, now in the bad look.
        current = 'rule';
        line.textContent = RULE_MESSAGE;
        line.className = BASE_CLASS + ' ' + BAD_CLASS;
        if (onChange !== null) {
          onChange('rule', controller);
        }
        return;
      }
      show('checking', CHECKING_MESSAGE);
      var mine = seq;
      timer = setTimer(function () {
        timer = null;
        ask(folded, mine);
      }, debounceMs);
    }

    var controller = {
      isGood: function () {
        if (current === 'available') {
          return true;
        }
        return mode === 'signup' && current === 'yours';
      },
      state: function () {
        return current;
      },
      name: function () {
        return fold(input.value);
      },
      showSaveError: function (message) {
        // Nothing still out may overwrite the refusal.
        seq += 1;
        cancelPending();
        show('refused', String(message === null || message === undefined ? '' : message));
      },
      refresh: judge
    };

    input.addEventListener('input', judge);
    judge();
    return controller;
  }

  window.PinalertUsernameCheck = { attach: attach };
}());
