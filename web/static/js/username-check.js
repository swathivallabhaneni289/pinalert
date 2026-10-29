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
//   cancel()              stops a check that is due and drops one still out
//                         (Settings calls it when its window shuts).
//
// options: {mode: 'signup' | 'settings' (default 'signup'), debounceMs (300),
// onChange(state, controller), setTimer, clearTimer, fetchFn}. The timers and the
// fetch default to the window's own and exist so tests can drive them.
//
// What it does while a person types:
//   - capitals become lowercase, a typed leading @ is dropped (the page draws its
//     own @ beside the input), and spaces round the value are removed; the caret
//     stays where the person was typing, and a paste is folded before the
//     field's own cut at its maxlength;
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
  // Review fix WR-B03: one check may take 10 s, a 429 is asked again at most three
  // times, and a wait longer than 30 s named by the server is cut to 30 s.
  var REQUEST_TIMEOUT_MS = 10000;
  var MAX_RETRIES = 3;
  var MAX_RETRY_SECONDS = 30;

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
    // The pause before a check, or the wait before a check is asked again.
    var timer = null;
    // How many times this judgement was asked again after a 429.
    var retries = 0;

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

    // limitSentence is the server's own sentence of a refusal, when it sent one.
    function limitSentence(body) {
      var sent = body && body.error && body.error.message;
      if (typeof sent === 'string' && sent !== '' && sent.length <= MAX_SENTENCE) {
        return sent;
      }
      return FAILED_MESSAGE;
    }

    // retryWait is the wait a 429 names, in ms: the Retry-After header, else the
    // body's retry_after_seconds, else one second, and never more than the cap.
    function retryWait(res, body) {
      var secs = NaN;
      try {
        if (res.headers && typeof res.headers.get === 'function') {
          secs = parseInt(res.headers.get('Retry-After'), 10);
        }
      } catch (err) {
        secs = NaN;
      }
      if (!(secs > 0) && body && typeof body.retry_after_seconds === 'number') {
        secs = body.retry_after_seconds;
      }
      if (!(secs > 0)) {
        secs = 1;
      }
      return Math.min(Math.ceil(secs), MAX_RETRY_SECONDS) * 1000;
    }

    function readBody(res) {
      if (typeof res.json !== 'function') {
        return Promise.resolve(null);
      }
      return Promise.resolve(res.json()).then(function (body) { return body; }, function () { return null; });
    }

    // ask sends one check (review fix WR-B03). It has a time limit, so a check
    // WebKit leaves unsettled ends as failed. A 429 from the per account allowance
    // keeps Checking and asks again by itself once the server's wait is over, up
    // to MAX_RETRIES times, so the person is never left with nothing to press.
    function ask(value, mine) {
      var controller = typeof AbortController === 'function' ? new AbortController() : null;
      var init = { credentials: 'same-origin', headers: { Accept: 'application/json' } };
      if (controller !== null) {
        init.signal = controller.signal;
      }
      var settled = false;
      var limit = setTimer(function () {
        limit = null;
        if (controller !== null) {
          controller.abort();
        }
        end(null);
      }, REQUEST_TIMEOUT_MS);

      // end takes the outcome once: null for a failed check, {limited, wait,
      // message} for a 429, or {body} for an answer.
      function end(outcome) {
        if (settled) {
          return;
        }
        settled = true;
        if (limit !== null) {
          clearTimer(limit);
          limit = null;
        }
        if (mine !== seq) {
          return;
        }
        if (outcome !== null && outcome.limited) {
          if (retries >= MAX_RETRIES) {
            show('failed', outcome.message);
            return;
          }
          retries += 1;
          cancelPending();
          timer = setTimer(function () {
            timer = null;
            if (mine === seq) {
              ask(value, mine);
            }
          }, outcome.wait);
          return;
        }
        var body = outcome === null ? null : outcome.body;
        if (!body || typeof body.state !== 'string' || SERVER_STATES.indexOf(body.state) === -1) {
          show('failed', FAILED_MESSAGE);
          return;
        }
        show(body.state, sentenceOr(body, RULE_MESSAGE));
      }

      var call;
      try {
        call = fetchFn(CHECK_URL + encodeURIComponent(value), init);
      } catch (err) {
        call = Promise.reject(err);
      }
      Promise.resolve(call).then(function (res) {
        if (res && res.status === 429) {
          return readBody(res).then(function (body) {
            end({ limited: true, wait: retryWait(res, body), message: limitSentence(body) });
          });
        }
        if (!res || res.status !== 200) {
          end(null);
          return null;
        }
        return readBody(res).then(function (body) {
          end({ body: body });
        });
      }).then(null, function () {
        end(null);
      });
    }

    // caretOf is where the caret is while the person is in the input, or null
    // (not focused, or a browser that does not say).
    function caretOf() {
      var doc = input.ownerDocument;
      if (!doc || doc.activeElement !== input || typeof input.selectionStart !== 'number' ||
          typeof input.setSelectionRange !== 'function') {
        return null;
      }
      return input.selectionStart;
    }

    function placeCaret(at) {
      try {
        input.setSelectionRange(at, at);
      } catch (err) {
        // The caret stays where the browser put it.
      }
    }

    // onPaste folds what is pasted before the field's own cut at maxlength, so a
    // pasted "@ravi.kumar_12345678" keeps every letter of the name. The pasted
    // text loses its own spaces round it and its leading @, goes where the caret
    // is, and the whole value is folded, cut and judged (review fix IN-B04).
    function onPaste(e) {
      var data = e && e.clipboardData;
      var pasted = data && typeof data.getData === 'function' ? data.getData('text') : '';
      if (!pasted) {
        return;
      }
      e.preventDefault();
      pasted = String(pasted).replace(/^\s+|\s+$/g, '').replace(/^@+/, '');
      var value = String(input.value === null || input.value === undefined ? '' : input.value);
      var start = typeof input.selectionStart === 'number' ? input.selectionStart : value.length;
      var end = typeof input.selectionEnd === 'number' ? input.selectionEnd : start;
      var head = value.slice(0, start) + pasted;
      var max = typeof input.getAttribute === 'function' ? parseInt(input.getAttribute('maxlength'), 10) : NaN;
      var next = fold(head + value.slice(end));
      if (max > 0) {
        next = next.slice(0, max);
      }
      input.value = next;
      var caret = Math.min(fold(head).length, next.length);
      if (typeof input.setSelectionRange === 'function') {
        placeCaret(caret);
      }
      judge();
    }

    // judge looks at the value in the input: folds it, shows the local rule, or
    // shows Checking and starts the pause before the request.
    function judge() {
      seq += 1;
      retries = 0;
      cancelPending();
      var raw = String(input.value === null || input.value === undefined ? '' : input.value);
      var folded = fold(raw);
      if (raw !== folded) {
        // Writing the value puts the caret at the end, so a capital typed in the
        // middle would send it there; it goes back where it was, less what the
        // fold dropped in front of it (review fix IN-B04).
        var caret = caretOf();
        input.value = folded;
        if (caret !== null) {
          var dropped = raw.length - raw.replace(/^\s+/, '').replace(/^@+/, '').length;
          placeCaret(Math.max(0, Math.min(folded.length, caret - dropped)));
        }
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
      refresh: judge,
      // cancel stops a check that is due and drops the answer of one still out,
      // so a shut window spends none of the account's allowance (review fix
      // IN-B05). The line keeps what it shows; the next refresh judges again.
      cancel: function () {
        seq += 1;
        cancelPending();
      }
    };

    input.addEventListener('input', judge);
    input.addEventListener('paste', onPaste);
    // A press anywhere in the box, the @ included, puts the cursor in the input.
    var box = input.parentNode;
    if (box && typeof box.addEventListener === 'function' && (' ' + String(box.className) + ' ').indexOf(' username-field ') !== -1) {
      box.addEventListener('click', function (e) {
        if (e.target !== input && typeof input.focus === 'function') {
          input.focus();
        }
      });
    }
    judge();
    return controller;
  }

  window.PinalertUsernameCheck = { attach: attach };
}());
