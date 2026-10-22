// mute.js owns the Stop updates about this report control of an opened report (Phase 3.2,
// D-11, UI-SPEC Surface C).
//
// The control is one calm row (plan 04-30), built like the Flag control (flag.js)
// and mounted just above it in the report card, so the laptop popup and the phone
// detail pane share one insertion. It shows only to a signed in reader who
// follows the report (posted it or voted on it) or already muted it, and it
// shows on the reader's own report (Q3, owner confirmed 2026-10-18; the one
// function to change if the owner says everyone sees it is shouldShow).
//
// A press posts mute or unmute at once, with no confirmation, because the
// same control undoes it. Success flips the label on every control for the
// report, says so through the page's polite status line (#notice-live) and
// asks the bell to refresh once (the pinalert:updates-refresh event, D-04).
// Failure leaves the label as it was and shows one line under the button.
// The two listeners stop propagation so a tap never reaches a row, card or map
// listener, and they leave the browser's own button activation alone. Every
// string reaches the page through Pinalert.setText or setAttribute, so nothing
// here parses markup. The Low data view loads none of this: it has no bell and
// no mute.
window.PinalertMute = (function () {
  'use strict';

  var MUTE_LABEL = 'Stop updates about this report';
  var UNMUTE_LABEL = 'Get updates about this report again';
  var MUTED_STATUS = 'Updates are off for this report.';
  var UNMUTED_STATUS = 'Updates are on again for this report.';
  var CHANGE_ERROR = "Couldn't change this. Try again.";
  var OFFLINE_ERROR = "You're offline, so this wasn't saved. Try again when you have a signal.";
  var REFRESH_EVENT = 'pinalert:updates-refresh';
  var MUTE_TIMEOUT_MS = 10000;

  // Report id (as a string) to true, for this page session only. A saved mute
  // or unmute fills one of them, so a poll that left before the press cannot
  // turn the label back. An entry is dropped once the server agrees.
  var mutedHere = {};
  var unmutedHere = {};
  // Report id to true while a request for it runs, so a second press waits.
  var sending = {};

  // shouldShow says whether the control is drawn for this report.
  function shouldShow(report) {
    var key = String(report.id);
    return report.viewer_follows === true || report.viewer_muted === true ||
      mutedHere[key] === true || unmutedHere[key] === true;
  }

  // reconcile drops this page's own memory once the server says the same.
  function reconcile(report) {
    var key = String(report.id);
    if (mutedHere[key] === true && report.viewer_muted === true) {
      delete mutedHere[key];
    }
    if (unmutedHere[key] === true && report.viewer_muted !== true) {
      delete unmutedHere[key];
    }
  }

  function isMuted(report) {
    var key = String(report.id);
    if (mutedHere[key] === true) {
      return true;
    }
    if (unmutedHere[key] === true) {
      return false;
    }
    return report.viewer_muted === true;
  }

  function accessibleName(prefix, report) {
    return prefix + ', ' + (Pinalert.CATEGORY_LABELS[report.category] || report.category) +
      ', ' + Pinalert.relativeTime(report.created_at);
  }

  // createMuteControl builds the button and its error line once. The button is
  // born hidden: the first update shows it, so a stranger never sees it flash.
  function createMuteControl(report) {
    var el = document.createElement('button');
    el.setAttribute('type', 'button');
    el.className = 'mute-control';
    el.dataset.muteReportId = String(report.id);
    el.dataset.muted = 'false';
    el.hidden = true;
    Pinalert.setText(el, MUTE_LABEL);

    var error = document.createElement('p');
    error.className = 'inline-error';
    error.setAttribute('role', 'alert');
    error.hidden = true;

    var control = { el: el, error: error };
    el.addEventListener('click', function (evt) {
      evt.stopPropagation();
      press(control);
    });
    el.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.stopPropagation();
      }
    });
    return control;
  }

  // updateMuteControl refreshes the control in place on every poll. It makes no
  // element, moves no focus and leaves a shown error line alone.
  function updateMuteControl(control, report) {
    var el = control.el;
    reconcile(report);
    el.hidden = !shouldShow(report);
    if (el.hidden) {
      control.error.hidden = true;
      return;
    }
    var muted = isMuted(report);
    var word = muted ? UNMUTE_LABEL : MUTE_LABEL;
    if (el.textContent !== word) {
      Pinalert.setText(el, word);
    }
    el.dataset.muted = muted ? 'true' : 'false';
    var name = accessibleName(word, report);
    if (el.getAttribute('aria-label') !== name) {
      el.setAttribute('aria-label', name);
    }
  }

  // markMutedEverywhere flips every control for the report on the page. The
  // controls are found by a loop over the class, never by a selector built from
  // report data.
  function markMutedEverywhere(reportId, muted) {
    var key = String(reportId);
    var word = muted ? UNMUTE_LABEL : MUTE_LABEL;
    var old = muted ? MUTE_LABEL : UNMUTE_LABEL;
    var all = document.querySelectorAll('.mute-control');
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      if (el.dataset.muteReportId !== key) {
        continue;
      }
      if (el.textContent !== word) {
        Pinalert.setText(el, word);
      }
      el.dataset.muted = muted ? 'true' : 'false';
      var label = el.getAttribute('aria-label');
      if (label && label.indexOf(old) === 0) {
        el.setAttribute('aria-label', word + label.slice(old.length));
      }
    }
  }

  // press sends a mute when the control reads Stop updates about this report and
  // an unmute when it reads Get updates about this report again. Offline it sends nothing.
  function press(control) {
    var el = control.el;
    var key = el.dataset.muteReportId;
    if (sending[key] === true) {
      return;
    }
    var muting = el.dataset.muted !== 'true';
    hideError(control);
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      showError(control, OFFLINE_ERROR);
      return;
    }
    sending[key] = true;
    send(control, key, muting);
  }

  // send posts with the shared time limit of app.js (Pinalert.requestTimeout),
  // the page's own timer raced against the post and its body read, as flag.js
  // does, so a post whose aborted request never ends still fails at the limit.
  function send(control, key, muting) {
    var limit = Pinalert.requestTimeout(MUTE_TIMEOUT_MS, 'mute');
    Promise.race([fetch('/api/reports/' + encodeURIComponent(key) + (muting ? '/mute' : '/unmute'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: '{}',
      signal: limit.signal
    }), limit.expired]).then(function (res) {
      return Promise.race([res.text(), limit.expired]).catch(function () {
        return '';
      }).then(function () {
        limit.clear();
        finish(control, key, muting, res.status);
      });
    }, function () {
      limit.clear();
      finish(control, key, muting, 0);
    });
  }

  function finish(control, key, muting, status) {
    delete sending[key];
    if (status < 200 || status >= 300) {
      showError(control, CHANGE_ERROR);
      return;
    }
    if (muting) {
      mutedHere[key] = true;
      delete unmutedHere[key];
    } else {
      unmutedHere[key] = true;
      delete mutedHere[key];
    }
    markMutedEverywhere(key, muting);
    hideError(control);
    announce(muting ? MUTED_STATUS : UNMUTED_STATUS);
    requestBellRefresh();
  }

  // requestBellRefresh asks the bell for one read (D-04). The bell listens on
  // the document; a browser without CustomEvent simply waits for its timer.
  function requestBellRefresh() {
    if (typeof CustomEvent === 'function') {
      document.dispatchEvent(new CustomEvent(REFRESH_EVENT));
    }
  }

  function showError(control, message) {
    Pinalert.setText(control.error, message);
    control.error.hidden = false;
  }

  function hideError(control) {
    control.error.hidden = true;
    Pinalert.setText(control.error, '');
  }

  // announce reads one result through the page's polite status line.
  function announce(text) {
    var live = document.getElementById('notice-live');
    if (!live) {
      return;
    }
    Pinalert.setText(live, '');
    window.setTimeout(function () {
      Pinalert.setText(live, text);
    }, 50);
  }

  return {
    createMuteControl: createMuteControl,
    updateMuteControl: updateMuteControl,
    shouldShow: shouldShow
  };
}());
