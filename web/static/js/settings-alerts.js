// settings-alerts.js: the Muted reports window and the two account confirm
// windows of the Settings page (Phase 3.2 plan 12, D-11, D-23, UI-SPEC E2, E3).
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
//     handler on a window itself, no Cancel and no Done (D-02).
//   - The Delete rule sentence is not here. It lives in the template only, so
//     the owner's answer to Q2 is a one sentence change.
(function () {
  'use strict';

  var mutedWin = document.getElementById('settings-muted-window');
  var logoutWin = document.getElementById('settings-logout-all-window');
  var deleteWin = document.getElementById('settings-delete-window');
  if (!mutedWin || !logoutWin || !deleteWin) {
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
  var LABEL_DELETE = 'Delete my account';
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
  var deleteError = el('settings-delete-error');
  var deleteGo = el('settings-delete-go');
  var live = el('notice-live');

  var windows = {
    muted: { win: mutedWin, row: mutedRow, close: el('settings-muted-close'), title: el('settings-muted-title') },
    logout: { win: logoutWin, row: logoutRow, close: el('settings-logout-all-close'), title: el('settings-logout-all-title') },
    del: { win: deleteWin, row: deleteRow, close: el('settings-delete-close'), title: el('settings-delete-title') }
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
    } else if (which === 'del') {
      hideLine(deleteError);
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

  // ---- The two confirm windows ----------------------------------------------

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

  function deleteAccount() {
    if (deleting) {
      return Promise.resolve();
    }
    deleting = true;
    hideLine(deleteError);
    deleteGo.disabled = true;
    setText(deleteGo, LABEL_DELETE_BUSY);
    function fail() {
      deleting = false;
      deleteGo.disabled = false;
      setText(deleteGo, LABEL_DELETE);
      showLine(deleteError, MESSAGE_DELETE_FAILED);
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

  // ---- Wiring ---------------------------------------------------------------

  mutedRow.addEventListener('click', function () { openWindow('muted'); });
  logoutRow.addEventListener('click', function () { openWindow('logout'); });
  deleteRow.addEventListener('click', function () { openWindow('del'); });
  windows.muted.close.addEventListener('click', function () { closeWindow('muted', true); });
  windows.logout.close.addEventListener('click', function () { closeWindow('logout', true); });
  windows.del.close.addEventListener('click', function () { closeWindow('del', true); });
  mutedWin.addEventListener('keydown', function (e) { onWindowKeydown('muted', e); });
  logoutWin.addEventListener('keydown', function (e) { onWindowKeydown('logout', e); });
  deleteWin.addEventListener('keydown', function (e) { onWindowKeydown('del', e); });
  mutedRetry.addEventListener('click', function () { loadMuted(); });
  logoutGo.addEventListener('click', function () { logOutAll(); });
  deleteGo.addEventListener('click', function () { deleteAccount(); });

  window.PinalertSettingsAlerts = {
    openWindow: openWindow,
    closeWindow: closeWindow,
    loadMuted: loadMuted,
    unmute: unmute,
    logOutAll: logOutAll,
    deleteAccount: deleteAccount
  };
}());
