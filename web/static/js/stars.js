// stars.js - window.PinalertStars: the shared five star control (Phase 9 plan
// 16, D-02, UI-SPEC Surface 3, 3a and 3b).
//
// One control draws the rating a reader gives a person: a fieldset with a
// legend, five buttons named "1 star" to "5 stars", and Remove my rating when a
// rating exists. The profile page mounts it today, and the star box that follows
// "Yes, mark resolved" mounts the same control through the same call, so the
// names, the saving and the error words are written once.
//
//   var control = PinalertStars.createControl({
//     username: 'asha_k',        // whose rating this is; the caller's value
//     current: 4,                // the server's your_rating, 0 when unrated
//     legend: 'Your rating',     // optional, this is the default
//     onStatus: function (line) {},  // "You rated asha_k 4 stars." and so on
//     onError: function (line) {},   // a sentence to show, or '' to clear it
//     onGone: function () {}     // optional: the server says the person is gone
//   });
//   parent.appendChild(control.el);
//   control.set(3);       // redraw from a new server value
//   control.destroy();    // take the control out of the page
//
// What it never does:
//   - It decides nothing. The browser computes no star, no tag and no
//     eligibility; the caller draws the control only when the server said the
//     reader may rate, and the server judges every write again.
//   - It never parses markup. Every string reaches the page through
//     textContent or setAttribute, and every node comes from createElement.
//   - It never reacts to the arrow keys. Choosing a star saves to the server, so
//     an arrow walk would send five saves. Tab walks the five buttons, and Enter
//     or Space presses one, as a button does by itself.
//
// SECURITY (T-09-60): no markup sink of any kind. The only address it sends to
// is /api/people/ plus the encoded username the caller gave it.
window.PinalertStars = (function () {
  'use strict';

  var REQUEST_TIMEOUT_MS = 10000;
  var PEOPLE_URL = '/api/people/';
  var STAR_COUNT = 5;

  // The words of the UI-SPEC (P9-C8, P9-C10, P9-C13). Each is shown as it is.
  var DEFAULT_LEGEND = 'Your rating';
  var REMOVE_LABEL = 'Remove my rating';
  var REMOVED_LINE = 'Your rating was removed.';
  var SAVE_FAILED = 'Couldn\'t save your rating. Try again.';
  var REMOVE_FAILED = 'Couldn\'t remove your rating. Try again.';
  var OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';

  function starsWord(n) {
    return n + (n === 1 ? ' star' : ' stars');
  }

  function ratedLine(username, n) {
    return 'You rated ' + username + ' ' + starsWord(n) + '.';
  }

  // waitLine builds the P9-C13 line from the server's retry_after_seconds: whole
  // minutes, rounded up, never less than one.
  function waitLine(body) {
    var seconds = body && typeof body.retry_after_seconds === 'number' ? body.retry_after_seconds : 0;
    var minutes = Math.ceil(seconds / 60);
    if (!(minutes >= 1)) {
      minutes = 1;
    }
    return 'Too many tries. Wait ' + minutes + (minutes === 1 ? ' minute' : ' minutes') + ' and try again.';
  }

  // serverMessage is the sentence the server wrote for a refusal, when it sent
  // one in its error envelope.
  function serverMessage(body) {
    var error = body && body.error;
    if (error && typeof error.message === 'string' && error.message !== '') {
      return error.message;
    }
    return '';
  }

  // request sends one call and resolves to {status, ok, body}. A body that is
  // not JSON (a 204 has none) becomes an empty object.
  function request(url, init) {
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    if (controller !== null) {
      init.signal = controller.signal;
    }
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = window.setTimeout(function () {
        timer = null;
        if (controller !== null) {
          controller.abort();
        }
        reject(new Error('no answer in time'));
      }, REQUEST_TIMEOUT_MS);
    });
    expired.catch(function () {});
    function clear() {
      if (timer !== null) {
        window.clearTimeout(timer);
        timer = null;
      }
    }
    var work = fetch(url, init).then(function (res) {
      return res.json().then(function (body) {
        return body;
      }, function () {
        return {};
      }).then(function (body) {
        return { status: res.status, ok: res.ok, body: body };
      });
    });
    return Promise.race([work, expired]).then(function (answer) {
      clear();
      return answer;
    }, function (err) {
      clear();
      throw err;
    });
  }

  function isOffline() {
    return typeof navigator !== 'undefined' && navigator.onLine === false;
  }

  function createControl(options) {
    var opts = options || {};
    var username = typeof opts.username === 'string' ? opts.username : '';
    var value = clampStars(opts.current);
    var busy = false;
    var destroyed = false;
    var stars = [];

    function clampStars(n) {
      var v = Number(n);
      if (!isFinite(v) || v < 1) {
        return 0;
      }
      return v > STAR_COUNT ? STAR_COUNT : Math.floor(v);
    }

    function tell(fn, line) {
      if (!destroyed && typeof fn === 'function') {
        fn(line);
      }
    }

    var root = document.createElement('fieldset');
    root.className = 'stars-control';

    var legend = document.createElement('legend');
    legend.className = 'stars-control__legend';
    legend.textContent = typeof opts.legend === 'string' && opts.legend !== '' ? opts.legend : DEFAULT_LEGEND;
    root.appendChild(legend);

    var row = document.createElement('div');
    row.className = 'stars-control__row';
    root.appendChild(row);

    function makeStar(n) {
      var button = document.createElement('button');
      button.className = 'stars-control__star';
      button.setAttribute('type', 'button');
      button.setAttribute('aria-label', starsWord(n));
      var glyph = document.createElement('span');
      glyph.className = 'stars-control__glyph';
      glyph.setAttribute('aria-hidden', 'true');
      button.appendChild(glyph);
      button.addEventListener('click', function () { save(n, button); });
      row.appendChild(button);
      return button;
    }

    for (var i = 1; i <= STAR_COUNT; i++) {
      stars.push(makeStar(i));
    }

    var remove = document.createElement('button');
    remove.className = 'btn stars-control__remove';
    remove.setAttribute('type', 'button');
    remove.textContent = REMOVE_LABEL;
    remove.addEventListener('click', function () { clearRating(); });
    row.appendChild(remove);

    // draw shows the stars up to the rating filled, marks the one equal to it
    // pressed, and shows Remove my rating only when a rating exists.
    function draw() {
      for (var n = 1; n <= STAR_COUNT; n++) {
        var star = stars[n - 1];
        star.setAttribute('aria-pressed', n === value ? 'true' : 'false');
        star.setAttribute('data-filled', n <= value ? 'true' : 'false');
      }
      remove.hidden = value === 0;
    }

    function setBusy(on) {
      busy = on;
      var all = stars.concat([remove]);
      for (var n = 0; n < all.length; n++) {
        all[n].disabled = on;
      }
      for (var s = 0; s < stars.length; s++) {
        if (on) {
          stars[s].setAttribute('aria-busy', 'true');
        } else {
          stars[s].removeAttribute('aria-busy');
        }
      }
    }

    function peopleUrl(tail) {
      return PEOPLE_URL + encodeURIComponent(username) + '/rating' + tail;
    }

    // refusal turns a failed answer into the one line to show, or null when the
    // person is gone and the caller redraws the page.
    function refusal(answer, failed) {
      if (answer.status === 404 && typeof opts.onGone === 'function') {
        tell(opts.onGone);
        return null;
      }
      if (answer.status === 409 && serverMessage(answer.body) !== '') {
        return serverMessage(answer.body);
      }
      if (answer.status === 429) {
        return waitLine(answer.body);
      }
      return failed;
    }

    // send runs one call with the five buttons disabled and aria-busy while it
    // is out, then settles. apply runs only on success; focus goes to the button
    // the person used, or to the first star when that one is gone.
    function send(url, init, failed, apply, focusTarget) {
      if (busy || destroyed) {
        return Promise.resolve(false);
      }
      tell(opts.onError, '');
      if (isOffline()) {
        tell(opts.onError, OFFLINE);
        return Promise.resolve(false);
      }
      setBusy(true);
      function settle() {
        setBusy(false);
        var target = focusTarget.hidden === true ? stars[0] : focusTarget;
        if (!destroyed && target && typeof target.focus === 'function') {
          target.focus();
        }
      }
      return request(url, init).then(function (answer) {
        if (!answer.ok) {
          var line = refusal(answer, failed);
          settle();
          if (line !== null) {
            tell(opts.onError, line);
          }
          return false;
        }
        apply();
        draw();
        settle();
        return true;
      }, function () {
        settle();
        tell(opts.onError, isOffline() ? OFFLINE : failed);
        return false;
      });
    }

    function save(n, button) {
      return send(peopleUrl(''), {
        method: 'POST',
        credentials: 'same-origin',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: JSON.stringify({ stars: n })
      }, SAVE_FAILED, function () {
        value = n;
        tell(opts.onStatus, ratedLine(username, n));
      }, button);
    }

    function clearRating() {
      return send(peopleUrl('/clear'), {
        method: 'POST',
        credentials: 'same-origin',
        headers: { Accept: 'application/json' }
      }, REMOVE_FAILED, function () {
        value = 0;
        tell(opts.onStatus, REMOVED_LINE);
      }, remove);
    }

    draw();

    return {
      el: root,
      set: function (n) {
        // A save or a removal is out: its own answer sets the value. A profile
        // read that started earlier must not draw the old rating over it
        // (09-REVIEW IN-B02).
        if (busy) {
          return;
        }
        value = clampStars(n);
        draw();
      },
      destroy: function () {
        destroyed = true;
        if (root.parentNode && typeof root.parentNode.removeChild === 'function') {
          root.parentNode.removeChild(root);
        }
      }
    };
  }

  return { createControl: createControl };
}());
