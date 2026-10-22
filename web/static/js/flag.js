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
  // The note box takes at most NOTE_MAX characters (its maxlength, counted the
  // way the browser counts them). The count shows only once NOTE_COUNT_FROM
  // characters are typed, so a short note shows no extra line (plan 04-31).
  var NOTE_MAX = 300;
  var NOTE_COUNT_FROM = 250;

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
      boxes: backdrop.querySelectorAll('input[name="flag-reason"]'),
      choose: document.getElementById('flag-dialog-choose'),
      text: document.getElementById('flag-dialog-text'),
      count: document.getElementById('flag-dialog-count'),
      textError: document.getElementById('flag-dialog-text-error'),
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
    refs.reasons.addEventListener('change', hideChooseReason);
    refs.text.addEventListener('input', function () {
      updateCount();
      hideTextError();
    });
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
    // A fresh dialog for every report: no ticks, an empty note. A refused or
    // failed send keeps both, because only this function clears them.
    for (var j = 0; j < r.boxes.length; j++) {
      r.boxes[j].checked = false;
    }
    r.text.value = '';
    updateCount();
    hideChooseReason();
    hideTextError();
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

  // checkedReasons reads the ticked slugs from the markup, in the markup's
  // order, so no reason is typed here.
  function checkedReasons() {
    var out = [];
    for (var i = 0; i < refs.boxes.length; i++) {
      if (refs.boxes[i].checked) {
        out.push(refs.boxes[i].value);
      }
    }
    return out;
  }

  // updateCount shows how many characters are left once the note is near
  // NOTE_MAX, and nothing before that.
  function updateCount() {
    var used = refs.text.value.length;
    if (used < NOTE_COUNT_FROM) {
      refs.count.hidden = true;
      Pinalert.setText(refs.count, '');
      return;
    }
    var left = Math.max(0, NOTE_MAX - used);
    Pinalert.setText(refs.count, left === 1 ? '1 character left' : left + ' characters left');
    refs.count.hidden = false;
  }

  function onSendClick() {
    if (sending) {
      return;
    }
    if (!current) {
      return;
    }
    var reasons = checkedReasons();
    if (reasons.length === 0) {
      showChooseReason();
      return;
    }
    // A wait the server already announced is respected without a request: the
    // stored message is shown again with the time that is left.
    if (flagWait !== null && Date.now() < flagWait.untilMs) {
      showError(waitMessageNow());
      return;
    }
    sendFlag(reasons, refs.text.value);
  }

  // waitMessageNow is the stored wait message with its wait phrase replaced by
  // the time left, rounded up.
  function waitMessageNow() {
    var left = Math.ceil((flagWait.untilMs - Date.now()) / 1000);
    if (flagWait.phrase && flagWait.message.indexOf(flagWait.phrase) !== -1) {
      return flagWait.message.replace(flagWait.phrase, Pinalert.formatWait(left));
    }
    return flagWait.message;
  }

  // rememberFlagWait stores the wait of a 429 and returns the message to show
  // now. The wait is per account, so one memory covers every report. A 429 with
  // no readable server message shows the generic sentence.
  function rememberFlagWait(body, headers) {
    var seconds = 0;
    if (body && typeof body.retry_after_seconds === 'number' && body.retry_after_seconds > 0) {
      seconds = body.retry_after_seconds;
    } else if (headers && typeof headers.get === 'function') {
      seconds = parseInt(headers.get('Retry-After'), 10);
    }
    var message = FLAG_ERROR_MESSAGE;
    if (body && body.error && typeof body.error.message === 'string' && body.error.message !== '') {
      message = body.error.message;
    }
    if (seconds > 0) {
      flagWait = {
        untilMs: Date.now() + seconds * 1000,
        message: message,
        phrase: Pinalert.formatWait(seconds)
      };
    } else {
      flagWait = null;
    }
    return message;
  }

  // sendFlag posts the flag with the shared time limit of app.js
  // (Pinalert.requestTimeout). The limit is the page's own timer raced
  // against the post and its body read, so a post whose aborted request never
  // ends (WebKit can leave one unsettled) still shows the offline line at
  // FLAG_TIMEOUT_MS instead of leaving the dialog sending for good. A body
  // that stalls lands in the body fallback, as an aborted body read did. The
  // body is the ticked reasons and the note as typed; the server trims it.
  function sendFlag(reasons, note) {
    setSending(true);
    var limit = Pinalert.requestTimeout(FLAG_TIMEOUT_MS, 'flag');
    Promise.race([fetch('/api/reports/' + encodeURIComponent(current.reportId) + '/flag', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify({ reasons: reasons, note: note }),
      signal: limit.signal
    }), limit.expired]).then(function (res) {
      return Promise.race([res.json(), limit.expired]).catch(function () {
        return {};
      }).then(function (body) {
        limit.clear();
        handleAnswer(res.status, body, res.headers);
      });
    }, function () {
      limit.clear();
      setSending(false);
      showError(FLAG_OFFLINE_MESSAGE);
    });
  }

  // serverField returns the field and the message of an error body, or
  // empty strings.
  function serverField(body) {
    var err = body && body.error;
    if (!err || typeof err.field !== 'string' || typeof err.message !== 'string') {
      return { field: '', message: '' };
    }
    return { field: err.field, message: err.message };
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
    if (status === 429) {
      showError(rememberFlagWait(body, headers));
      return;
    }
    if (status === 404 || status === 410) {
      refs.send.hidden = true;
      showError(FLAG_GONE_MESSAGE);
      refs.back.focus();
      return;
    }
    // A refused or too long note: the server's own sentence under the box,
    // with the ticks and the text kept for an edit.
    var fe = serverField(body);
    if ((status === 422 || status === 400) && fe.field === 'note' && fe.message !== '') {
      showTextError(fe.message);
      return;
    }
    if (status === 400 && fe.field === 'reasons') {
      showChooseReason();
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
    Pinalert.setText(refs.send, on ? SENDING_LABEL : SEND_LABEL);
    if (on) {
      refs.panel.setAttribute('aria-busy', 'true');
      hideChooseReason();
      hideTextError();
      hideError();
    } else {
      refs.panel.removeAttribute('aria-busy');
    }
  }

  // showChooseReason shows the Validation line on the group of tick boxes and
  // moves focus to the first box.
  function showChooseReason() {
    refs.choose.hidden = false;
    refs.reasons.setAttribute('aria-describedby', 'flag-dialog-choose');
    announce(refs.choose.textContent);
    refs.boxes[0].focus();
  }

  function hideChooseReason() {
    refs.choose.hidden = true;
    refs.reasons.removeAttribute('aria-describedby');
  }

  // showTextError shows the server's sentence about the note under the box,
  // ties it to the box and moves focus there. Nothing is cleared.
  function showTextError(message) {
    Pinalert.setText(refs.textError, message);
    refs.textError.setAttribute('role', 'alert');
    refs.textError.hidden = false;
    refs.text.setAttribute('aria-invalid', 'true');
    refs.text.setAttribute('aria-describedby', 'flag-dialog-text-error');
    hideChooseReason();
    hideError();
    refs.text.focus();
  }

  function hideTextError() {
    refs.textError.hidden = true;
    refs.textError.removeAttribute('role');
    Pinalert.setText(refs.textError, '');
    refs.text.removeAttribute('aria-invalid');
    refs.text.removeAttribute('aria-describedby');
  }

  // showError shows one of the failure sentences. The role is set here and
  // removed when the line is hidden, so only a shown failure is announced.
  function showError(message) {
    Pinalert.setText(refs.error, message);
    refs.error.setAttribute('role', 'alert');
    refs.error.hidden = false;
    hideChooseReason();
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

  // focusableInPanel lists the Tab stops of the panel in order: every tick
  // box (each is its own stop), the note box, then the shown, enabled
  // buttons. The trap in onDialogKeydown wraps from the last to the first.
  function focusableInPanel() {
    var list = [];
    for (var i = 0; i < refs.boxes.length; i++) {
      list.push(refs.boxes[i]);
    }
    list.push(refs.text);
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
