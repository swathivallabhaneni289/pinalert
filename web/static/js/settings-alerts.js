// settings-alerts.js: the Muted reports window, the Log out of all devices
// window and the Delete my account box of the Settings page (Phase 3.2 plan 12,
// D-11, D-23, UI-SPEC E2, E3),
// and the Notifications rows (Phase 3.2 plan 15, NOTIF-05, UI-SPEC E1).
//
// Owner decision 2026-11-03 (quick 261103-jal): the Notifications box has two
// rows. The distance select "How far from my home area" is saved at once, and
// without a home area it is an Add a home area first button. Phase 3.3 plan 14
// (PWA-03, UI-SPEC Surface 2) added the Alerts on this device row with its four
// variants and the install guide window; it is the other row and the only
// switch. The Reliable, Critical, official and Changes switches are gone. The
// device row only asks notify.js (plan 06) what
// this device can do and tells it to turn on or off. It never reads a position,
// never decides a word outside the UI-SPEC strings below, and shows the switch
// on only when notify.js reports that the server confirmed this endpoint for
// this account. It does not clear the intent flag when a person logs out: that
// flag lives in notify.js, and this file stores nothing in the browser.
//
// A separate file from settings.js on purpose, so settings.js and its contract
// stay as built. It is self contained like settings.js: it needs no other
// script of the app, only the ids settings.html.tmpl renders, and it leaves
// quietly on a page that does not have them. Its small helpers are copies of
// settings.js's own, not imports.
//
// What it never does:
//   - It never parses markup. A muted report's text is a reporter's words, and
//     it reaches the page through textContent; every node is made with
//     createElement (T-03.2-08).
//   - It never sends a person anywhere but the two fixed sign in addresses, and
//     the only thing from a response that reaches a URL is a report id that was
//     checked to be digits (T-03.2-22).
//   - It never closes a window except through the X or Escape: there is no
//     handler on a window itself, no Cancel and no Done (D-02). The one
//     exception is the Delete box (owner, 2026-10-25: "yes exactly how the
//     logout option is looking and it should work like any delete my account
//     thing work"): a native dialog like Log out, answered by its Cancel and
//     Delete buttons, with Escape the same as Cancel except while the delete is
//     being sent. It still has no handler on the dialog itself for a click, so
//     a tap on the dim page does nothing.
//   - The Delete rule sentence is not here. It lives in the template only, so
//     the owner's answer to Q2 is a one sentence change.
(function () {
  'use strict';

  var mutedWin = document.getElementById('settings-muted-window');
  var logoutWin = document.getElementById('settings-logout-all-window');
  var deleteBox = document.getElementById('settings-delete-confirm');
  if (!mutedWin || !logoutWin || !deleteBox) {
    return;
  }

  var MUTED_URL = '/api/me/muted-reports';
  var LOGOUT_ALL_URL = '/api/me/logout-all';
  var DELETE_URL = '/api/me/delete';
  var LOGGED_OUT_PAGE = '/login?done=all-devices';
  var DELETED_PAGE = '/login?done=deleted';
  var REQUEST_TIMEOUT_MS = 10000;
  // A report id is a positive whole number of at most 18 digits. Anything else
  // never becomes part of an address.
  var REPORT_ID_PATTERN = /^[1-9][0-9]{0,17}$/;

  var MESSAGE_GONE = 'This report is no longer available';
  var MESSAGE_UNMUTED = 'Updates are on again for that report.';
  var MESSAGE_UNMUTE_FAILED = 'Couldn\'t turn updates back on. Try again.';
  var MESSAGE_LOGOUT_FAILED = 'Couldn\'t log you out of all devices. Try again.';
  var MESSAGE_DELETE_FAILED = 'Couldn\'t delete your account. Try again.';
  var LABEL_UNMUTE = 'Unmute this report';
  var LABEL_LOGOUT = 'Log out of all devices';
  var LABEL_LOGOUT_BUSY = 'Logging out…';
  var LABEL_DELETE = 'Delete';
  var LABEL_DELETE_BUSY = 'Deleting…';
  var MIDDLE_DOT = ' · ';

  var FOCUSABLE_SELECTOR = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

  function el(id) {
    return document.getElementById(id);
  }

  var mutedRow = el('settings-muted-row');
  var mutedValue = el('settings-muted-value');
  var mutedLoading = el('settings-muted-loading');
  var mutedList = el('settings-muted-list');
  var mutedEmpty = el('settings-muted-empty');
  var mutedEmptyTitle = el('settings-muted-empty-title');
  var mutedError = el('settings-muted-error');
  var mutedRetry = el('settings-muted-retry');
  var logoutRow = el('settings-logout-all-row');
  var logoutError = el('settings-logout-all-error');
  var logoutGo = el('settings-logout-all-go');
  var deleteRow = el('settings-delete-row');
  var deleteTitle = el('settings-delete-title');
  var deleteError = el('settings-delete-error');
  var deleteCancel = el('settings-delete-cancel');
  var deleteGo = el('settings-delete-go');
  var live = el('notice-live');

  // canAskInBox is true when the browser can show the Delete box as a native
  // modal. Without it the browser's own confirm asks the same question.
  var canAskInBox = typeof deleteBox.showModal === 'function';
  var DELETE_QUESTION = deleteTitle && deleteTitle.textContent ? deleteTitle.textContent.trim() : 'Delete your account?';

  var windows = {
    muted: { win: mutedWin, row: mutedRow, close: el('settings-muted-close'), title: el('settings-muted-title') },
    logout: { win: logoutWin, row: logoutRow, close: el('settings-logout-all-close'), title: el('settings-logout-all-title') }
  };

  var current = null;
  // loadSeq numbers the loads of the muted list, so an answer that comes back
  // after the window was closed or reloaded is dropped.
  var loadSeq = 0;
  var loadRequest = null;
  var loggingOut = false;
  var deleting = false;
  var nextTextId = 1;

  // ---- Page lock ------------------------------------------------------------

  var lockedNodes = [];

  // lockPage silences every child of the body except the open window, so
  // assistive technology and Tab stay inside it, and marks the root element so
  // the stylesheet stops the page behind scrolling.
  function lockPage(win) {
    unlockPage();
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

  // unlockPage undoes lockPage exactly: only what it silenced is restored.
  function unlockPage() {
    for (var i = 0; i < lockedNodes.length; i++) {
      var entry = lockedNodes[i];
      if (entry.attr === 'inert') {
        entry.node.inert = false;
        entry.node.removeAttribute('inert');
      } else {
        entry.node.removeAttribute('aria-hidden');
      }
    }
    lockedNodes = [];
    document.documentElement.removeAttribute('data-modal-open');
  }

  // ---- Small helpers --------------------------------------------------------

  function setText(node, value) {
    node.textContent = value === null || value === undefined ? '' : String(value);
  }

  function showLine(node, message) {
    setText(node, message);
    node.hidden = false;
  }

  function hideLine(node) {
    setText(node, '');
    node.hidden = true;
  }

  // make builds one element: a class, optional text.
  function make(tag, className, text) {
    var node = document.createElement(tag);
    if (className) {
      node.setAttribute('class', className);
    }
    if (text !== undefined) {
      setText(node, text);
    }
    return node;
  }

  function say(message) {
    if (live) {
      setText(live, message);
    }
  }

  // isShown says whether node and every ancestor up to the window are visible,
  // so the focus trap skips hidden parts.
  function isShown(node, win) {
    for (var n = node; n && n !== win; n = n.parentNode) {
      if (n.hidden) {
        return false;
      }
    }
    return true;
  }

  function focusables(win) {
    var all = win.querySelectorAll(FOCUSABLE_SELECTOR);
    var out = [];
    for (var i = 0; i < all.length; i++) {
      if (!all[i].disabled && isShown(all[i], win)) {
        out.push(all[i]);
      }
    }
    return out;
  }

  // request sends one request with a time limit that also covers reading the
  // body, so a request WebKit leaves unsettled still ends. It resolves to
  // {status, ok, body} and rejects on a network failure or the limit.
  function request(url, init, ms) {
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
      }, ms);
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
    var promise = Promise.race([work, expired]).then(function (answer) {
      clear();
      return answer;
    }, function (err) {
      clear();
      throw err;
    });
    return {
      promise: promise,
      abort: function () {
        clear();
        if (controller !== null) {
          controller.abort();
        }
      }
    };
  }

  // ---- Opening and closing --------------------------------------------------

  function openWindow(which) {
    var target = windows[which];
    if (!target) {
      return;
    }
    if (current !== null && current !== which) {
      closeWindow(current, false);
    }
    current = which;
    if (which === 'logout') {
      hideLine(logoutError);
    }
    target.win.hidden = false;
    lockPage(target.win);
    if (which === 'muted') {
      loadMuted();
    }
    target.title.focus();
  }

  // closeWindow is reached from the X and Escape only. restoreFocus puts focus
  // back on the row that opened the window.
  function closeWindow(which, restoreFocus) {
    var target = windows[which || current];
    if (!target || current === null || (which && which !== current)) {
      return;
    }
    if (current === 'muted') {
      loadSeq += 1;
      if (loadRequest !== null) {
        loadRequest.abort();
        loadRequest = null;
      }
    }
    target.win.hidden = true;
    unlockPage();
    current = null;
    if (restoreFocus !== false) {
      target.row.focus();
    }
  }

  function onWindowKeydown(which, e) {
    if (e.key === 'Escape') {
      closeWindow(which, true);
      return;
    }
    if (e.key !== 'Tab') {
      return;
    }
    var win = windows[which].win;
    var items = focusables(win);
    if (items.length === 0) {
      return;
    }
    var first = items[0];
    var last = items[items.length - 1];
    var active = document.activeElement;
    if (e.shiftKey && (active === first || !win.contains(active))) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && (active === last || !win.contains(active))) {
      e.preventDefault();
      first.focus();
    }
  }

  // ---- Muted reports --------------------------------------------------------

  function showMutedState(state) {
    mutedLoading.hidden = state !== 'loading';
    mutedList.hidden = state !== 'list';
    mutedEmpty.hidden = state !== 'empty';
    mutedError.hidden = state !== 'error';
  }

  function validId(value) {
    return REPORT_ID_PATTERN.test(String(value));
  }

  // renderMutedRow makes one li: a text block (the report's text or the gone
  // sentence, then Category and Severity) and the Unmute this report button.
  // The failure line for a refused unmute sits under the text, hidden.
  function renderMutedRow(report) {
    var li = make('li', 'muted-row');
    var textBlock = make('div', 'muted-row__text');
    var textId = 'settings-muted-text-' + nextTextId;
    nextTextId += 1;
    textBlock.setAttribute('id', textId);
    var hasText = report.available === true && typeof report.text === 'string' && report.text !== '';
    textBlock.appendChild(make('p', 'muted-row__title', hasText ? report.text : MESSAGE_GONE));
    if (report.available === true) {
      var category = typeof report.category === 'string' ? report.category : '';
      var severity = typeof report.severity === 'string' ? report.severity : '';
      if (category !== '' || severity !== '') {
        var what = category !== '' && severity !== '' ? category + MIDDLE_DOT + severity : category + severity;
        textBlock.appendChild(make('p', 'muted-row__what', what));
      }
    }
    var failure = make('p', 'inline-error');
    failure.setAttribute('role', 'alert');
    failure.hidden = true;
    textBlock.appendChild(failure);
    li.appendChild(textBlock);

    var button = make('button', 'btn', LABEL_UNMUTE);
    button.setAttribute('type', 'button');
    button.setAttribute('aria-describedby', textId);
    var busy = false;
    button.addEventListener('click', function () {
      if (busy) {
        return;
      }
      busy = true;
      button.disabled = true;
      hideLine(failure);
      unmute(report.report_id, li, failure).then(function (done) {
        busy = false;
        if (!done) {
          button.disabled = false;
        }
      });
    });
    li.appendChild(button);
    return li;
  }

  function renderMuted(reports) {
    mutedList.textContent = '';
    for (var i = 0; i < reports.length; i++) {
      var report = reports[i];
      if (!report || typeof report !== 'object' || !validId(report.report_id)) {
        continue;
      }
      mutedList.appendChild(renderMutedRow(report));
    }
    showMutedState(mutedList.children.length > 0 ? 'list' : 'empty');
  }

  function showMutedError() {
    showMutedState('error');
  }

  // loadMuted asks for the list and draws it. The loading rows show first.
  function loadMuted() {
    loadSeq += 1;
    var mine = loadSeq;
    if (loadRequest !== null) {
      loadRequest.abort();
    }
    showMutedState('loading');
    var req = request(MUTED_URL, { method: 'GET', credentials: 'same-origin', headers: { Accept: 'application/json' } }, REQUEST_TIMEOUT_MS);
    loadRequest = req;
    return req.promise.then(function (answer) {
      if (mine !== loadSeq) {
        return;
      }
      loadRequest = null;
      if (!answer.ok || !answer.body || !Array.isArray(answer.body.reports)) {
        showMutedError();
        return;
      }
      renderMuted(answer.body.reports);
    }, function () {
      if (mine !== loadSeq) {
        return;
      }
      loadRequest = null;
      showMutedError();
    });
  }

  function lowerCount() {
    var n = parseInt(mutedValue.textContent, 10);
    if (isNaN(n) || n < 1) {
      n = 1;
    }
    setText(mutedValue, n - 1);
  }

  // unmute posts the unmute for one report. It resolves to true when the mute
  // is cleared, and false when it failed (the failure line then says so under
  // the row). The row leaves at once on success, the count drops, focus moves
  // to the next Unmute this report button or to the empty heading.
  function unmute(reportId, li, failure) {
    if (!validId(reportId)) {
      return Promise.resolve(false);
    }
    var url = '/api/reports/' + encodeURIComponent(String(reportId)) + '/unmute';
    return request(url, { method: 'POST', credentials: 'same-origin', headers: { Accept: 'application/json' } }, REQUEST_TIMEOUT_MS).promise.then(function (answer) {
      if (!answer.ok) {
        if (failure) {
          showLine(failure, MESSAGE_UNMUTE_FAILED);
        }
        return false;
      }
      var before = mutedList.querySelectorAll('button');
      var at = 0;
      for (var i = 0; i < before.length; i++) {
        if (li.contains(before[i])) {
          at = i;
        }
      }
      if (li.parentNode) {
        li.parentNode.removeChild(li);
      }
      lowerCount();
      say(MESSAGE_UNMUTED);
      var after = mutedList.querySelectorAll('button');
      if (after.length === 0) {
        showMutedState('empty');
        mutedEmptyTitle.focus();
      } else {
        after[Math.min(at, after.length - 1)].focus();
      }
      return true;
    }, function () {
      if (failure) {
        showLine(failure, MESSAGE_UNMUTE_FAILED);
      }
      return false;
    });
  }

  // ---- The Log out of all devices window and the Delete box -------------------

  function logOutAll() {
    if (loggingOut) {
      return Promise.resolve();
    }
    loggingOut = true;
    hideLine(logoutError);
    logoutGo.disabled = true;
    setText(logoutGo, LABEL_LOGOUT_BUSY);
    function fail() {
      loggingOut = false;
      logoutGo.disabled = false;
      setText(logoutGo, LABEL_LOGOUT);
      showLine(logoutError, MESSAGE_LOGOUT_FAILED);
    }
    return request(LOGOUT_ALL_URL, { method: 'POST', credentials: 'same-origin' }, REQUEST_TIMEOUT_MS).promise.then(function (answer) {
      if (!answer.ok) {
        fail();
        return;
      }
      window.location.assign(LOGGED_OUT_PAGE);
    }, fail);
  }

  // openDeleteConfirm asks the Delete question. The native box opens with focus
  // on the question, so no button looks chosen and nothing is sent until Delete
  // is pressed. The page is inert behind a native modal and auth.css stops it
  // scrolling, so lockPage is not used.
  function openDeleteConfirm() {
    if (deleting) {
      return;
    }
    if (!canAskInBox) {
      if (window.confirm(DELETE_QUESTION)) {
        deleteAccount();
      }
      return;
    }
    if (current !== null) {
      closeWindow(current, false);
    }
    hideLine(deleteError);
    deleteBox.showModal();
    deleteTitle.focus();
  }

  function deleteAccount() {
    if (deleting) {
      return Promise.resolve();
    }
    deleting = true;
    hideLine(deleteError);
    deleteGo.disabled = true;
    deleteCancel.disabled = true;
    setText(deleteGo, LABEL_DELETE_BUSY);
    function fail() {
      deleting = false;
      deleteGo.disabled = false;
      deleteCancel.disabled = false;
      setText(deleteGo, LABEL_DELETE);
      if (!canAskInBox) {
        window.alert(MESSAGE_DELETE_FAILED);
        return;
      }
      // A browser may force the box shut on a second Escape even though the
      // cancel was refused, so it opens again before the sentence shows.
      if (!deleteBox.open) {
        deleteBox.showModal();
      }
      showLine(deleteError, MESSAGE_DELETE_FAILED);
      // showModal runs the browser's own focusing steps, which can land on
      // Cancel, and a disabled Delete drops focus to the body. The question
      // takes it back, as when the box first opened.
      deleteTitle.focus();
    }
    return request(DELETE_URL, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirm: true })
    }, REQUEST_TIMEOUT_MS).promise.then(function (answer) {
      if (!answer.ok) {
        fail();
        return;
      }
      window.location.assign(DELETED_PAGE);
    }, fail);
  }

  // ---- Notifications (Phase 3.2 plan 15, NOTIF-05, UI-SPEC E1; two rows since quick 261103-jal) ----
  //
  // The select "How far from my home area" saves at once through PATCH
  // /api/me/alerts, with no Save button. A change that fails puts the old value
  // back and says so in one line. Without a home area the select is replaced by
  // an Add a home area first button that opens the Home area window. The section
  // follows the home area: settings.js writes the Home area row's data-value when
  // a home area is saved, and a MutationObserver on that attribute redraws the
  // row, so settings.js stays as it was built. The other rows of the section are
  // gone (owner decision 2026-11-03): the device row below is the only switch.

  var ALERTS_URL = '/api/me/alerts';
  var MESSAGE_ALERTS_FAILED = 'Couldn\'t save that. Try again.';
  var MESSAGE_ALERTS_OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
  var RADIUS_PATTERN = /^[1-9][0-9]{0,2}$/;

  // The distance row has two variants inside one wrapper: the select, and the
  // button that asks for a home area. A row hides itself with the hidden
  // attribute, which places.css respects.
  var radiusRow = el('settings-radius-row');
  var radiusSelect = el('settings-radius-select');
  var radiusNoHome = el('settings-radius-nohome');
  var alertsError = el('settings-alerts-error');
  var deviceError = el('settings-device-error');
  var homeRowNode = el('settings-home-row');
  var homeWindow = el('settings-home-window');
  var alertsReady = !!(radiusRow && radiusSelect && radiusNoHome &&
    alertsError && deviceError && homeRowNode && homeWindow);

  // The one field the alert route takes from this page, with the control that
  // sends it. The select holds the option text.
  var CONTROLS = alertsReady ? { alert_radius_km: radiusSelect } : {};

  // saved holds the last value the server confirmed, which is what a failed save
  // puts back.
  var saved = {};
  if (alertsReady) {
    saved.alert_radius_km = String(radiusSelect.value);
  }
  var homeIsSet = alertsReady ? (homeRowNode.getAttribute('data-value') || '') !== '' : false;
  // openedFromAlerts is true while the Home area window was opened from the
  // distance row's button, so focus goes back to the matching control when it
  // closes.
  var openedFromAlerts = false;

  function putBack() {
    radiusSelect.value = saved.alert_radius_km;
  }

  // saveAlerts sends one change ({alert_radius_km}) and resolves to true when the
  // server kept it. The select is disabled until the answer. Offline, nothing is
  // sent. The device error line and this one never show together.
  function saveAlerts(body) {
    if (!alertsReady || !body || typeof body !== 'object') {
      return Promise.resolve(false);
    }
    if (!Object.prototype.hasOwnProperty.call(body, 'alert_radius_km')) {
      return Promise.resolve(false);
    }
    function fail(message) {
      CONTROLS.alert_radius_km.disabled = false;
      putBack();
      showLine(alertsError, message);
      return false;
    }
    hideLine(alertsError);
    hideLine(deviceError);
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return Promise.resolve(fail(MESSAGE_ALERTS_OFFLINE));
    }
    CONTROLS.alert_radius_km.disabled = true;
    var init = {
      method: 'PATCH',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ alert_radius_km: body.alert_radius_km })
    };
    return request(ALERTS_URL, init, REQUEST_TIMEOUT_MS).promise.then(function (answer) {
      if (!answer.ok || !answer.body || typeof answer.body !== 'object') {
        return fail(MESSAGE_ALERTS_FAILED);
      }
      CONTROLS.alert_radius_km.disabled = false;
      if (typeof answer.body.alert_radius_km === 'number') {
        saved.alert_radius_km = String(answer.body.alert_radius_km);
        radiusSelect.value = saved.alert_radius_km;
      }
      return true;
    }, function () {
      return fail(MESSAGE_ALERTS_FAILED);
    });
  }

  // redrawAlerts draws the distance row for a person with or without a home
  // area: the select with one, the Add a home area first button without. A home
  // area can no longer be removed in Settings (Phase 7.1 plan 15, D-15), so the
  // no home state is only a defensive state for data repair and says nothing.
  function redrawAlerts(hasHome) {
    if (!alertsReady) {
      return;
    }
    homeIsSet = hasHome === true;
    radiusRow.hidden = !homeIsSet;
    radiusNoHome.hidden = homeIsSet;
  }

  // syncHome follows the Home area row: it redraws only when the home area
  // went from none to some or from some to none, never when it changed place.
  function syncHome() {
    var now = (homeRowNode.getAttribute('data-value') || '') !== '';
    if (now !== homeIsSet) {
      redrawAlerts(now);
    }
  }

  // openHomeFromRadius opens the Home area window from the distance row's button
  // and remembers it, so focus can go back to the matching control.
  function openHomeFromRadius() {
    if (window.PinalertSettings && typeof window.PinalertSettings.openWindow === 'function') {
      openedFromAlerts = true;
      window.PinalertSettings.openWindow('home');
    }
  }

  function wireAlerts() {
    radiusSelect.addEventListener('change', function () {
      if (!RADIUS_PATTERN.test(String(radiusSelect.value))) {
        radiusSelect.value = saved.alert_radius_km;
        return;
      }
      saveAlerts({ alert_radius_km: parseInt(radiusSelect.value, 10) });
    });
    radiusNoHome.addEventListener('click', openHomeFromRadius);
    if (typeof MutationObserver === 'function') {
      new MutationObserver(syncHome).observe(homeRowNode, { attributes: true, attributeFilter: ['data-value'] });
      // When the Home area window opened from the button closes, focus goes to
      // the select if a home area now exists, or back to the button.
      new MutationObserver(function () {
        syncHome();
        if (homeWindow.hidden && openedFromAlerts) {
          openedFromAlerts = false;
          (homeIsSet ? radiusSelect : radiusNoHome).focus();
        }
      }).observe(homeWindow, { attributes: true, attributeFilter: ['hidden'] });
    }
  }

  if (alertsReady) {
    wireAlerts();
  }

  // ---- Alerts on this device (Phase 3.3 plan 14, PWA-03, D-10 to D-12, UI-SPEC 2a) ----
  //
  // One row for this device, drawn in one of four variants: the switch (ready or
  // on), the install row (an iPhone tab, opens the guide window), and two plain
  // sentences (blocked, unsupported). notify.js decides the state and does the
  // work; every word on screen is here. The switch shows on only when notify.js
  // says the server confirmed this endpoint for this account. A tap on the
  // switch calls notify.js from the change handler itself, with nothing awaited
  // first, because the browser's prompt needs the tap. Turning it on or off
  // changes nothing else in the section. Since quick 261103-jal the device
  // switch is the only way to stop phone notes.

  var MESSAGE_DEVICE_ON = 'Alerts are on for this device.';
  var MESSAGE_DEVICE_OFF = 'Alerts are off for this device.';
  var MESSAGE_TURN_ON_FAILED = 'Couldn\'t turn this on. Try again.';
  var MESSAGE_TURN_OFF_FAILED = 'Couldn\'t turn this off. Try again.';

  var deviceOnRow = el('settings-device-on');
  var deviceSwitch = el('settings-device-switch');
  var installRow = el('settings-device-install');
  var blockedRow = el('settings-device-blocked');
  var unsupportedRow = el('settings-device-unsupported');
  var guideWin = el('device-guide-window');
  var guideClose = el('device-guide-close');
  var guideTitle = el('device-guide-title');
  var deviceReady = !!(deviceOnRow && deviceSwitch && installRow && blockedRow && unsupportedRow &&
    deviceError && guideWin && guideClose && guideTitle);

  var deviceKey = '';
  // deviceOn is what the server confirmed. deviceBusy is true while a tap works.
  // deviceSeq counts the taps, so a late answer from the load check is dropped.
  var deviceOn = false;
  var deviceBusy = false;
  var deviceWant = false;
  var deviceSeq = 0;

  // drawDevice shows one variant of the row. 'ready' and 'on' are the switch;
  // an unknown state is drawn as unsupported, never as a broken switch.
  function drawDevice(state) {
    if (!deviceReady) {
      return;
    }
    var variant = state === 'maybe' || state === 'busy' ? 'ready' : state;
    if (['ready', 'on', 'needs-install', 'blocked', 'unsupported'].indexOf(variant) === -1) {
      variant = 'unsupported';
    }
    deviceOnRow.hidden = !(variant === 'ready' || variant === 'on');
    installRow.hidden = variant !== 'needs-install';
    blockedRow.hidden = variant !== 'blocked';
    unsupportedRow.hidden = variant !== 'unsupported';
    deviceOn = variant === 'on';
    deviceSwitch.checked = deviceOn;
    deviceSwitch.disabled = false;
    deviceSwitch.removeAttribute('aria-busy');
  }

  // readPushKey reads the public key the page carries. An empty key means the
  // server cannot send, so the device is drawn as unsupported.
  function readPushKey() {
    var node = typeof document.querySelector === 'function' ? document.querySelector('[data-push-key]') : null;
    var raw = node ? node.getAttribute('data-push-key') : '';
    return typeof raw === 'string' ? raw : '';
  }

  function hasNotify() {
    return !!window.PinalertNotify && typeof window.PinalertNotify.baseState === 'function';
  }

  // finishDevice draws what a settled tap ended in. want is what the person
  // asked for (true is on).
  function finishDevice(want, result) {
    if (want) {
      if (result.ok === true) {
        drawDevice('on');
        say(MESSAGE_DEVICE_ON);
        return;
      }
      if (result.reason === 'refused') {
        // Blocked only when the browser itself now blocks; a dismissed prompt
        // leaves the switch, since the browser will ask again.
        drawDevice(result.state === 'blocked' ? 'blocked' : 'ready');
        say(MESSAGE_DEVICE_OFF);
        return;
      }
      if (result.reason === 'unsupported') {
        drawDevice(result.state);
        return;
      }
      drawDevice('ready');
      showLine(deviceError, result.reason === 'offline' ? MESSAGE_ALERTS_OFFLINE : MESSAGE_TURN_ON_FAILED);
      return;
    }
    if (result.ok === true) {
      drawDevice('ready');
      say(MESSAGE_DEVICE_OFF);
      return;
    }
    drawDevice('on');
    showLine(deviceError, result.reason === 'offline' ? MESSAGE_ALERTS_OFFLINE : MESSAGE_TURN_OFF_FAILED);
  }

  // onDeviceSwitch runs from the switch's change event, which is the tap.
  function onDeviceSwitch() {
    if (deviceBusy) {
      deviceSwitch.checked = deviceWant;
      return;
    }
    if (!hasNotify()) {
      drawDevice('unsupported');
      return;
    }
    var want = deviceSwitch.checked === true;
    deviceBusy = true;
    deviceWant = want;
    deviceSeq += 1;
    hideLine(deviceError);
    hideLine(alertsError);
    deviceSwitch.disabled = true;
    deviceSwitch.setAttribute('aria-busy', 'true');
    var work;
    try {
      if (want) {
        work = window.PinalertNotify.enable(deviceKey);
      } else {
        work = window.PinalertNotify.disable();
      }
    } catch (err) {
      work = Promise.reject(err);
    }
    Promise.resolve(work).then(function (result) {
      deviceBusy = false;
      finishDevice(want, result && typeof result === 'object' ? result : {});
    }, function () {
      deviceBusy = false;
      finishDevice(want, { ok: false, reason: 'failed' });
    });
  }

  // reconcileDevice draws the base state at once, then the server's word. When
  // permission was given and the server lost the registration, notify.js
  // registers it again quietly. An answer that comes after a tap is dropped.
  function reconcileDevice() {
    deviceKey = readPushKey();
    if (!hasNotify() || deviceKey === '') {
      drawDevice('unsupported');
      return;
    }
    var base = window.PinalertNotify.baseState();
    if (base !== 'maybe') {
      drawDevice(base);
      return;
    }
    drawDevice('ready');
    var seq = deviceSeq;
    var asked = window.PinalertNotify.reconcile(deviceKey);
    Promise.resolve(asked).then(function (state) {
      if (deviceBusy || seq !== deviceSeq) {
        return;
      }
      drawDevice(state);
    }, function () {
      // The row keeps the switch it has; nothing is said about a failed check.
    });
  }

  if (deviceReady) {
    windows.guide = { win: guideWin, row: installRow, close: guideClose, title: guideTitle };
    deviceSwitch.addEventListener('change', onDeviceSwitch);
    installRow.addEventListener('click', function () { openWindow('guide'); });
    guideClose.addEventListener('click', function () { closeWindow('guide', true); });
    guideWin.addEventListener('keydown', function (e) { onWindowKeydown('guide', e); });
    reconcileDevice();
  }

  // ---- Wiring ---------------------------------------------------------------

  mutedRow.addEventListener('click', function () { openWindow('muted'); });
  logoutRow.addEventListener('click', function () { openWindow('logout'); });
  deleteRow.addEventListener('click', function () { openDeleteConfirm(); });
  windows.muted.close.addEventListener('click', function () { closeWindow('muted', true); });
  windows.logout.close.addEventListener('click', function () { closeWindow('logout', true); });
  mutedWin.addEventListener('keydown', function (e) { onWindowKeydown('muted', e); });
  logoutWin.addEventListener('keydown', function (e) { onWindowKeydown('logout', e); });
  mutedRetry.addEventListener('click', function () { loadMuted(); });
  logoutGo.addEventListener('click', function () { logOutAll(); });
  // The Delete box: Cancel and Delete answer it, and Escape (the browser's
  // cancel event) is the same as Cancel except while the delete is being sent.
  // There is no click handler on the dialog, so a tap on the dim page does
  // nothing.
  deleteCancel.addEventListener('click', function () {
    if (!deleting) {
      deleteBox.close();
    }
  });
  deleteGo.addEventListener('click', function () { deleteAccount(); });
  deleteBox.addEventListener('cancel', function (e) {
    if (deleting) {
      e.preventDefault();
    }
  });
  deleteBox.addEventListener('close', function () { deleteRow.focus(); });

  window.PinalertSettingsAlerts = {
    openWindow: openWindow,
    closeWindow: closeWindow,
    loadMuted: loadMuted,
    unmute: unmute,
    logOutAll: logOutAll,
    openDeleteConfirm: openDeleteConfirm,
    deleteAccount: deleteAccount,
    saveAlerts: saveAlerts,
    redrawAlerts: redrawAlerts,
    drawDevice: drawDevice
  };
}());
