// flag.js owns the Flag this report control and the one Flag dialog.
//
// The control is one quiet text button per report. It lives in the opened
// report (the popup card, which is the map popup on a laptop and the full
// screen sheet on a phone) and on the Low data row, never on a main page list
// row (D-09, P4-A7 amended). Its two listeners stop propagation so a tap or a
// key press never reaches a row, card or map listener, and they leave the
// browser's own button activation alone. A flag never hides, dims or relabels
// a report for anyone: the only visible change is this reader's own control
// text. The feature is online only and saves nothing on the device. There is
// no outside reporting feature in this flow. Every string reaches the page
// through Pinalert.setText or setAttribute, and the dialog is static template
// markup, so nothing here parses markup.
window.PinalertFlag = (function () {
  'use strict';

  var FLAG_LABEL = 'Flag this report';
  var FLAGGED_LABEL = 'Flagged';
  var SEND_LABEL = 'Send to moderators';
  var SENDING_LABEL = 'Sending';
  var FLAG_SENT_TOAST = 'Flag sent. Moderators will see it.';
  var FLAG_ALREADY_TOAST = 'You already flagged this report. Moderators will see it.';
  var FLAG_OFFLINE_MESSAGE = "You're offline, so this wasn't sent. Flags are never saved for later. Try again when you have a signal.";
  var FLAG_GONE_MESSAGE = 'This report is no longer available.';
  var FLAG_ERROR_MESSAGE = 'Something went wrong. Try again.';
  var FLAG_TIMEOUT_MS = 10000;

  // Report id (as a string) to true, for this page session only. A 201 or a 409
  // fills it, so a poll that left before the flag cannot turn a control back.
  var flaggedHere = {};
  var refs = null;
  var current = null;
  var sending = false;
  var flagWait = null;

  function isFlagged(report) {
    return report.viewer_flagged === true || flaggedHere[String(report.id)] === true;
  }

  function accessibleName(prefix, report) {
    return prefix + ', ' + (Pinalert.CATEGORY_LABELS[report.category] || report.category) +
      ', ' + Pinalert.relativeTime(report.created_at);
  }

  // createFlagControl builds the button once. It is born hidden: the first
  // update shows it, so a reader's own report never flashes it.
  function createFlagControl(report) {
    var el = document.createElement('button');
    el.setAttribute('type', 'button');
    el.className = 'flag-control';
    el.dataset.flagReportId = String(report.id);
    el.hidden = true;
    Pinalert.setText(el, FLAG_LABEL);
    el.addEventListener('click', function (evt) {
      evt.stopPropagation();
      if (el.getAttribute('aria-disabled') === 'true') {
        return;
      }
      openFlagDialog(el);
    });
    el.addEventListener('keydown', function (evt) {
      if (evt.key === 'Enter' || evt.key === ' ' || evt.key === 'Spacebar') {
        evt.stopPropagation();
      }
    });
    return { el: el };
  }

  // updateFlagControl refreshes the control in place on every poll. It makes no
  // element, moves no focus and never reads the dialog.
  function updateFlagControl(control, report) {
    var el = control.el;
    el.hidden = report.is_own_report === true;
    if (el.hidden) {
      return;
    }
    var flagged = isFlagged(report);
    var word = flagged ? FLAGGED_LABEL : FLAG_LABEL;
    if (el.textContent !== word) {
      Pinalert.setText(el, word);
    }
    el.setAttribute('aria-label', accessibleName(word, report));
    if (flagged) {
      el.setAttribute('aria-disabled', 'true');
    } else {
      el.removeAttribute('aria-disabled');
    }
  }

  // markFlaggedEverywhere flips every control for the report on the page. The
  // controls are found by a loop over the class, never by a selector built from
  // report data.
  function markFlaggedEverywhere(reportId) {
    var key = String(reportId);
    flaggedHere[key] = true;
    var all = document.querySelectorAll('.flag-control');
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      if (el.dataset.flagReportId !== key) {
        continue;
      }
      Pinalert.setText(el, FLAGGED_LABEL);
      el.setAttribute('aria-disabled', 'true');
      var label = el.getAttribute('aria-label');
      if (label && label.indexOf(FLAG_LABEL) === 0) {
        el.setAttribute('aria-label', FLAGGED_LABEL + label.slice(FLAG_LABEL.length));
      }
    }
  }

  function surfaceOf(el) {
    if (el.closest('.popup-card')) {
      return 'popup';
    }
    if (el.closest('.report-row--low')) {
      return 'low';
    }
    return 'row';
  }

  function isShown(el) {
    return !!el && el.isConnected && !el.hidden && el.getClientRects().length > 0;
  }

  // dialogRefs finds the static dialog once and binds its listeners once.
  function dialogRefs() {
    if (refs) {
      return refs;
    }
    var backdrop = document.getElementById('flag-dialog');
    if (!backdrop) {
      return null;
    }
    refs = {
      backdrop: backdrop,
      panel: backdrop.querySelector('.modal-panel'),
      title: document.getElementById('flag-dialog-title'),
      reasons: document.getElementById('flag-dialog-reasons'),
      radios: backdrop.querySelectorAll('input[name="flag-reason"]'),
      note: document.getElementById('flag-dialog-note'),
      error: document.getElementById('flag-dialog-error'),
      send: document.getElementById('flag-dialog-send'),
      back: document.getElementById('flag-dialog-back')
    };
    refs.send.addEventListener('click', onSendClick);
    refs.back.addEventListener('click', function () {
      if (!sending) {
        closeFlagDialog();
      }
    });
    backdrop.addEventListener('click', function (evt) {
      if (evt.target === refs.backdrop && !sending) {
        closeFlagDialog();
      }
    });
    backdrop.addEventListener('keydown', onDialogKeydown);
    refs.reasons.addEventListener('change', hideNote);
    return refs;
  }

  function openFlagDialog(opener) {
    var r = dialogRefs();
    if (!r || current) {
      return;
    }
    current = {
      reportId: opener.dataset.flagReportId,
      opener: opener,
      surface: surfaceOf(opener),
      rowNext: null,
      rowPrev: null
    };
    // Remember the list item for this report and its neighbours, so focus has
    // somewhere real to go if the control itself is gone when the dialog closes.
    var row = opener.closest('.report-row');
    if (!row) {
      var items = document.querySelectorAll('.report-row, .past-card');
      for (var i = 0; i < items.length; i++) {
        if (items[i].dataset.reportId === current.reportId) {
          row = items[i];
          break;
        }
      }
    }
    if (row) {
      current.rowNext = row.nextElementSibling;
      current.rowPrev = row.previousElementSibling;
    }
    for (var j = 0; j < r.radios.length; j++) {
      r.radios[j].checked = false;
    }
    hideNote();
    hideError();
    r.send.hidden = false;
    setSending(false);
    r.backdrop.hidden = false;
    Pinalert.lockPage(r.backdrop);
    r.title.focus();
  }

  // closeFlagDialog unlocks the page before it returns focus, because an inert
  // element cannot take focus (P4-A26). It does nothing while a request runs.
  function closeFlagDialog() {
    if (sending) {
      return;
    }
    if (!current) {
      return;
    }
    var state = current;
    current = null;
    refs.backdrop.hidden = true;
    Pinalert.unlockPage();
    returnFocus(state);
  }

  // checkedReason reads the slug from the markup, so no reason is typed here.
  function checkedReason() {
    for (var i = 0; i < refs.radios.length; i++) {
      if (refs.radios[i].checked) {
        return refs.radios[i].value;
      }
    }
    return '';
  }

  function onSendClick() {
    if (sending) {
      return;
    }
    if (!current) {
      return;
    }
    var reason = checkedReason();
    if (reason === '') {
      showNote();
      return;
    }
    sendFlag(reason);
  }

  function sendFlag(reason) {
    setSending(true);
    var signal;
    var timer = null;
    if (typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
      signal = AbortSignal.timeout(FLAG_TIMEOUT_MS);
    } else if (typeof AbortController === 'function') {
      var controller = new AbortController();
      signal = controller.signal;
      timer = window.setTimeout(function () {
        controller.abort();
      }, FLAG_TIMEOUT_MS);
    }
    fetch('/api/reports/' + encodeURIComponent(current.reportId) + '/flag', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify({ reason: reason }),
      signal: signal
    }).then(function (res) {
      return res.json().catch(function () {
        return {};
      }).then(function (body) {
        if (timer !== null) {
          window.clearTimeout(timer);
        }
        handleAnswer(res.status, body, res.headers);
      });
    }, function () {
      if (timer !== null) {
        window.clearTimeout(timer);
      }
      setSending(false);
      showError(FLAG_ERROR_MESSAGE);
    });
  }

  function handleAnswer(status, body, headers) {
    setSending(false);
    if (status === 201) {
      finishFlagged(FLAG_SENT_TOAST);
      return;
    }
    if (status === 409) {
      finishFlagged(FLAG_ALREADY_TOAST);
      return;
    }
    showError(FLAG_ERROR_MESSAGE);
  }

  function finishFlagged(toast) {
    markFlaggedEverywhere(current.reportId);
    closeFlagDialog();
    Pinalert.showToast(toast);
  }

  function setSending(on) {
    sending = on;
    refs.send.disabled = on;
    refs.back.disabled = on;
    if (on) {
      hideNote();
      hideError();
    }
  }

  function showNote() {
    refs.note.hidden = false;
    refs.radios[0].setAttribute('aria-describedby', 'flag-dialog-note');
    announce(refs.note.textContent);
    refs.radios[0].focus();
  }

  function hideNote() {
    refs.note.hidden = true;
    refs.radios[0].removeAttribute('aria-describedby');
  }

  // showError shows one of the failure sentences. The role is set here and
  // removed when the line is hidden, so only a shown failure is announced.
  function showError(message) {
    Pinalert.setText(refs.error, message);
    refs.error.setAttribute('role', 'alert');
    refs.error.hidden = false;
    hideNote();
    // A button that was disabled while sending can drop focus; keep it inside
    // the panel so Escape and Tab still reach the dialog.
    if (!refs.panel.contains(document.activeElement)) {
      (refs.send.hidden ? refs.back : refs.send).focus();
    }
  }

  function hideError() {
    refs.error.hidden = true;
    refs.error.removeAttribute('role');
    Pinalert.setText(refs.error, '');
  }

  // announce reads a passive prompt once through the page's status live region.
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

  function focusableInPanel() {
    var list = [];
    var first = refs.radios[0];
    for (var i = 0; i < refs.radios.length; i++) {
      if (refs.radios[i].checked) {
        first = refs.radios[i];
        break;
      }
    }
    list.push(first);
    var buttons = refs.panel.querySelectorAll('button');
    for (var j = 0; j < buttons.length; j++) {
      if (!buttons[j].disabled && isShown(buttons[j])) {
        list.push(buttons[j]);
      }
    }
    return list;
  }

  function onDialogKeydown(evt) {
    if (evt.key === 'Escape') {
      evt.stopPropagation();
      evt.preventDefault();
      if (!sending) {
        closeFlagDialog();
      }
      return;
    }
    if (evt.key !== 'Tab') {
      return;
    }
    var list = focusableInPanel();
    var firstEl = list[0];
    var lastEl = list[list.length - 1];
    var active = document.activeElement;
    if (!refs.panel.contains(active)) {
      evt.preventDefault();
      firstEl.focus();
      return;
    }
    if (evt.shiftKey) {
      if (active === firstEl || active === refs.title) {
        evt.preventDefault();
        lastEl.focus();
      }
    } else if (active === lastEl) {
      evt.preventDefault();
      firstEl.focus();
    }
  }

  // focusRowTarget focuses a list item: main page rows and Past cards carry a
  // tabindex, a Low data row does not, so its own Flag control takes focus.
  function focusRowTarget(row) {
    if (!isShown(row)) {
      return false;
    }
    if (row.hasAttribute('tabindex')) {
      row.focus();
      return document.activeElement === row;
    }
    var control = row.querySelector('.flag-control');
    if (control && isShown(control)) {
      control.focus();
      return document.activeElement === control;
    }
    return false;
  }

  // returnFocus puts focus back where the reader was, stopping at the first
  // target that takes it (P4-A26).
  function returnFocus(state) {
    if (isShown(state.opener)) {
      state.opener.focus();
      if (document.activeElement === state.opener) {
        return;
      }
    }
    var controls = document.querySelectorAll('.flag-control');
    for (var i = 0; i < controls.length; i++) {
      var el = controls[i];
      if (el.dataset.flagReportId === state.reportId && surfaceOf(el) === state.surface && isShown(el)) {
        el.focus();
        if (document.activeElement === el) {
          return;
        }
      }
    }
    if (state.surface === 'popup' && window.PinalertMap && typeof PinalertMap.focusMarker === 'function' &&
        PinalertMap.focusMarker(state.reportId)) {
      return;
    }
    var items = document.querySelectorAll('.report-row, .past-card');
    for (var j = 0; j < items.length; j++) {
      if (items[j].dataset.reportId === state.reportId && focusRowTarget(items[j])) {
        return;
      }
    }
    if (state.rowNext && focusRowTarget(state.rowNext)) {
      return;
    }
    if (state.rowPrev && focusRowTarget(state.rowPrev)) {
      return;
    }
    var mapEl = document.getElementById('map');
    if (isShown(mapEl)) {
      mapEl.focus();
      return;
    }
    var reportButton = document.getElementById('low-report');
    if (isShown(reportButton)) {
      reportButton.focus();
    }
  }

  return {
    createFlagControl: createFlagControl,
    updateFlagControl: updateFlagControl
  };
}());
