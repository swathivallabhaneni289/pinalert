// updates.js: the bell's own script (Phase 3.2, plan 08).
//
// It keeps the red count on the bell, fills the bell list with the latest 10
// updates and marks updates read. It makes the only two requests of the bell:
// GET /api/updates and POST /api/updates/read.
//
// Why this is its own file. app-bar.js opens and closes the list and must stay
// request free (Phase 3.4 pins that). app.js is pinned at two network calls,
// and its refresh timer runs on Nearby only, while the bell is on four pages
// (Nearby, Official alerts, My activity, Settings), two of which do not load
// app.js. Research Pitfall 1 and D-04: the bell uses its own request on its own
// timer. The roadmap says the bell "rides on the existing 30 second refresh";
// that is met by the same 30 second interval, not by the same request. That
// wording has not been put to the owner; raise it at UAT in plain words.
//
// This file needs no other script and no app global, so it works on every page
// that has the bell. Every word comes from the server and is written with
// textContent or setAttribute; no markup is ever built from a string. A row can
// open only "/?report=" plus an id that passes a digits check. Since quick
// 261118-rz6 the rows that say the reader's own report was removed or stays
// removed open the reader's box instead (why, and Ask for review), the
// #review-ask-box dialog the page renders as a child of body; this file reads
// and sends it through the same request helper, at
// "/api/reports/" plus a checked id plus "/review-ask".
//
// The list is frozen while open: an update that arrives while a person reads
// raises the count behind the list and shows the next time the list opens.
// Opening clears the count at once and marks the shown updates read on the
// server; each row keeps its new mark until the list closes. There is no
// "mark all read" button, and the count carries no live region (the number is
// in the button's name only, research Pitfall 11).
(function () {
  'use strict';

  var trigger = document.getElementById('bell-trigger');
  var panel = document.getElementById('bell-list');
  if (!trigger || !panel) {
    return;
  }

  var LIST_URL = '/api/updates';
  var READ_URL = '/api/updates/read';
  var DEFAULT_INTERVAL_MS = 30000;
  var REQUEST_TIMEOUT_MS = 10000;
  var MAX_ROWS = 10;
  var UNREAD_CAP = 99;
  var LAPTOP_QUERY = '(min-width: 900px)';
  var MIDDLE_DOT = '·';

  // A report id the row link may carry: digits, no leading zero, at most 18.
  var ID_PATTERN = /^[1-9][0-9]{0,17}$/;

  // The address of the reader's box for one report (quick 261118-rz6), built
  // only around an id that passes ID_PATTERN.
  var REVIEW_ASK_PREFIX = '/api/reports/';
  var REVIEW_ASK_SUFFIX = '/review-ask';

  // The plain row that says the reader's own report is hidden for review.
  var KIND_OWN_REVIEW = 'own_review';

  // The only severity classes a dot can get. Anything else gets none.
  var SEVERITY_CLASS = { low: 'sev-low', medium: 'sev-medium', critical: 'sev-critical' };

  var countEl = document.getElementById('bell-count');
  var bodyEl = document.getElementById('bell-body');
  var loadingEl = document.getElementById('bell-loading');
  var errorEl = document.getElementById('bell-error');
  var staleEl = document.getElementById('bell-stale');
  var rowsEl = document.getElementById('bell-rows');
  var emptyEl = panel.querySelector('.bell-list__empty');

  var last = null; // the last good answer: { unread, updates }
  var lastFailed = false; // the most recent poll failed
  var stopped = false; // the person is signed out: stop polling quietly
  var inFlight = false;
  var queued = false;
  var open = false;
  var shownRows = false; // rows were drawn in the current open session
  var firstLink = null;
  var intervalId = null;
  var pollSeq = 0; // how many polls have started
  var freshFrom = 0; // only an answer to a poll with a number from here sets the count
  var readsInFlight = 0;
  var readUpTo = 0; // the largest id this page has asked the server to mark read

  // ---- Words --------------------------------------------------------------

  // clampUnread turns the server's count into a whole number from 0 to 99.
  function clampUnread(value) {
    var n = Number(value);
    if (!isFinite(n) || n < 0) {
      return 0;
    }
    return Math.min(Math.floor(n), UNREAD_CAP);
  }

  // badgeText is the digits on the badge: nothing at zero, 1 to 9, then 9+.
  function badgeText(n) {
    if (!(n > 0)) {
      return '';
    }
    return n >= 10 ? '9+' : String(n);
  }

  // buttonLabel is the bell button's name (P32-C2).
  function buttonLabel(n) {
    if (!(n > 0)) {
      return 'Updates';
    }
    if (n >= UNREAD_CAP) {
      return 'Updates, 99 or more new';
    }
    return 'Updates, ' + n + ' new';
  }

  function text(value) {
    return typeof value === 'string' ? value : '';
  }

  // setBadge shows the count on the bell and in its name, quietly.
  function setBadge(n) {
    var shown = badgeText(n);
    if (countEl) {
      countEl.textContent = shown;
      countEl.hidden = shown === '';
    }
    trigger.setAttribute('aria-label', buttonLabel(n));
  }

  // ---- The request --------------------------------------------------------

  // request sends one request with a time limit that also covers reading the
  // body, so a request WebKit leaves unsettled still ends. It resolves to
  // {status, ok, body} and rejects on a network failure or the limit. It is the
  // helper settings.js uses; this file carries its own copy because it must not
  // depend on another script.
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
    return Promise.race([work, expired]).then(function (answer) {
      clear();
      return answer;
    }, function (err) {
      clear();
      throw err;
    });
  }

  // ---- Drawing the list ---------------------------------------------------

  // boxId is the report id whose box a row opens, or the empty string: only
  // the removed and stays removed rows about the reader's own report carry
  // review_report_id, and only while that report is still removed (quick
  // 261118-rz6, which replaced vvb's Policy link on the removed row). It is
  // checked first, so a box row is never a link.
  function boxId(u) {
    if (u.available === true && ID_PATTERN.test(String(u.review_report_id))) {
      return String(u.review_report_id);
    }
    return '';
  }

  // rowHref is the one address a row opens, or the empty string for a plain
  // row or a box row.
  function rowHref(u) {
    if (boxId(u) !== '') {
      return '';
    }
    if (u.available === true && u.kind !== 'more' && ID_PATTERN.test(String(u.report_id))) {
      return '/?report=' + String(u.report_id);
    }
    return '';
  }

  function isLinkRow(u) {
    return rowHref(u) !== '';
  }

  // marksNew says whether a row may carry the new mark while unread: every link
  // row, every box row (quick 261118-rz6), and the plain row that says the
  // reader's own report is hidden for review (quick 261117-vvb review,
  // 2026-11-18).
  function marksNew(u) {
    return isLinkRow(u) || boxId(u) !== '' || (u.available === true && u.kind === KIND_OWN_REVIEW);
  }

  function severityClass(level) {
    return typeof level === 'string' && Object.prototype.hasOwnProperty.call(SEVERITY_CLASS, level) ? SEVERITY_CLASS[level] : '';
  }

  function span(className, words) {
    var el = document.createElement('span');
    el.setAttribute('class', className);
    el.textContent = words;
    return el;
  }

  // buildRow draws one update (UI-SPEC A3). A link row is one anchor to the
  // report; the "and N more" and "no longer available" shapes are plain blocks
  // that cannot be focused or pressed.
  //
  // The row that says the reader's own report is hidden for review is plain but
  // is news, so it carries the new mark while unread like a link row, and says
  // New. to a screen reader through hidden text, since a plain block has no
  // label (quick 261117-vvb review, 2026-11-18). Other plain rows never do.
  //
  // A box row (quick 261118-rz6) is one button that opens the reader's box for
  // that report. It is named by its label like a link row and carries the new
  // mark the same way.
  function buildRow(u, newIds) {
    var li = document.createElement('li');
    var link = isLinkRow(u);
    var box = boxId(u);
    var ownPlain = !link && box === '' && marksNew(u);
    var isNew = marksNew(u) && newIds[Number(u.id)] === true;
    var classes = 'bell-row';
    if (isNew) {
      classes += ' bell-row--new';
    }
    if (!link && box === '') {
      classes += ' bell-row--plain';
    }
    if (box !== '') {
      classes += ' bell-row--box';
    }
    if (u.kind === 'more') {
      classes += ' bell-row--more';
    }
    var el = document.createElement(link ? 'a' : (box !== '' ? 'button' : 'div'));
    el.setAttribute('class', classes);
    if (link || box !== '') {
      if (link) {
        el.setAttribute('href', rowHref(u));
      } else {
        el.setAttribute('type', 'button');
        el.setAttribute('data-review-ask', box);
        el.setAttribute('aria-haspopup', 'dialog');
      }
      if (text(u.label) !== '') {
        el.setAttribute('aria-label', u.label);
      }
      if (firstLink === null) {
        firstLink = el;
      }
    }
    if (ownPlain && isNew) {
      el.appendChild(span('visually-hidden', 'New. '));
    }
    el.appendChild(span('bell-row__event', text(u.event)));
    if (text(u.report_line) !== '') {
      el.appendChild(span('bell-row__line', u.report_line));
    }
    if (text(u.category) !== '') {
      var what = document.createElement('span');
      what.setAttribute('class', 'bell-row__what');
      var dot = document.createElement('span');
      var sev = severityClass(u.severity_level);
      dot.setAttribute('class', sev === '' ? 'bell-row__sev' : 'bell-row__sev ' + sev);
      dot.setAttribute('aria-hidden', 'true');
      what.appendChild(dot);
      var words = u.category;
      if (text(u.severity) !== '') {
        words += ' ' + MIDDLE_DOT + ' ' + u.severity;
      }
      what.appendChild(document.createTextNode(words));
      el.appendChild(what);
    }
    if (text(u.trust) !== '') {
      el.appendChild(span('bell-row__trust', u.trust));
    }
    el.appendChild(span('bell-row__when', text(u.when)));
    li.appendChild(el);
    return li;
  }

  // showOnly shows one of the four bodies of the list and hides the rest. The
  // body is busy only while it loads.
  function showOnly(which) {
    if (loadingEl) {
      loadingEl.hidden = which !== 'loading';
    }
    if (errorEl) {
      errorEl.hidden = which !== 'error';
    }
    if (rowsEl) {
      rowsEl.hidden = which !== 'rows';
    }
    if (emptyEl) {
      emptyEl.hidden = which !== 'empty';
    }
    if (bodyEl) {
      if (which === 'loading') {
        bodyEl.setAttribute('aria-busy', 'true');
      } else {
        bodyEl.removeAttribute('aria-busy');
      }
    }
  }

  function setStale(on) {
    if (staleEl) {
      staleEl.hidden = !on;
    }
  }

  // render draws the rows. newIds holds the ids to draw with the new mark.
  function render(updates, newIds) {
    if (!rowsEl) {
      return;
    }
    rowsEl.textContent = '';
    firstLink = null;
    for (var i = 0; i < updates.length; i++) {
      if (updates[i] && typeof updates[i] === 'object') {
        rowsEl.appendChild(buildRow(updates[i], newIds || {}));
      }
    }
    showOnly(updates.length > 0 ? 'rows' : 'empty');
  }

  // ---- Read marks ---------------------------------------------------------

  // applyLocalRead drops the unread flag from every row up to what this page
  // has already asked the server to mark, so the next open does not draw it
  // new again from an older answer.
  function applyLocalRead(updates) {
    for (var i = 0; i < updates.length; i++) {
      if (updates[i] && Number(updates[i].id) <= readUpTo) {
        updates[i].unread = false;
      }
    }
  }

  // sendRead asks the server to mark the account's updates up to an id read,
  // then polls for the true count. While it runs, no poll answer may set the
  // count: one asked before the mark finished would bring the old number back.
  function sendRead(upTo) {
    var previous = readUpTo;
    if (upTo > readUpTo) {
      readUpTo = upTo;
    }
    readsInFlight += 1;
    freshFrom = Infinity;
    function done(ok) {
      if (!ok && readUpTo === upTo) {
        readUpTo = previous;
      }
      readsInFlight -= 1;
      if (readsInFlight === 0) {
        freshFrom = pollSeq + 1;
      }
      poll();
    }
    request(READ_URL, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ up_to: upTo })
    }, REQUEST_TIMEOUT_MS).then(function (answer) {
      done(answer.ok === true);
    }, function () {
      done(false);
    });
  }

  // ---- Opening and closing ------------------------------------------------

  // renderOpen draws the last good answer for this open session, clears the
  // count and marks the shown updates read. moveFocus is false when the rows
  // arrive while the person is already reading, so focus does not jump.
  function renderOpen(moveFocus) {
    var updates = last.updates;
    var newIds = {};
    var maxId = 0;
    var anyUnread = false;
    for (var i = 0; i < updates.length; i++) {
      var u = updates[i];
      if (!u || typeof u !== 'object') {
        continue;
      }
      var id = Number(u.id);
      if (isFinite(id) && id > maxId) {
        maxId = id;
      }
      if (u.unread === true) {
        anyUnread = true;
        if (marksNew(u)) {
          newIds[id] = true;
        }
      }
    }
    render(updates, newIds);
    setStale(lastFailed);
    shownRows = true;
    setBadge(0);
    if (anyUnread && maxId > 0) {
      sendRead(maxId);
      applyLocalRead(updates);
    }
    if (moveFocus && firstLink !== null) {
      firstLink.focus();
    }
  }

  function onOpen() {
    open = true;
    shownRows = false;
    if (stopped) {
      setStale(false);
      showOnly('empty');
      return;
    }
    if (last === null) {
      setStale(false);
      showOnly(lastFailed ? 'error' : 'loading');
      return;
    }
    renderOpen(true);
  }

  function onClose() {
    open = false;
    shownRows = false;
  }

  // syncOpenState compares the panel with what this script last saw and runs
  // the open or close step once per change. It runs after the trigger click
  // (this script loads after app-bar.js, so the panel has already toggled) and
  // when the panel's hidden attribute changes (Escape, the X, an outside click).
  function syncOpenState() {
    var nowOpen = !panel.hidden;
    if (nowOpen === open) {
      return;
    }
    if (nowOpen) {
      onOpen();
    } else {
      onClose();
    }
  }

  // ---- The poll -----------------------------------------------------------

  function stop() {
    stopped = true;
    if (intervalId !== null) {
      window.clearInterval(intervalId);
      intervalId = null;
    }
    setBadge(0);
  }

  function onFailure() {
    lastFailed = true;
    if (!open) {
      return;
    }
    if (last === null) {
      showOnly('error');
    } else {
      setStale(true);
    }
  }

  function onAnswer(answer, seq) {
    if (answer.status === 401) {
      stop();
      return;
    }
    var data = answer.body;
    if (answer.ok !== true || !data || !Array.isArray(data.updates)) {
      onFailure();
      return;
    }
    var updates = data.updates.slice(0, MAX_ROWS);
    var fresh = seq >= freshFrom;
    if (!fresh) {
      applyLocalRead(updates);
    }
    last = { unread: clampUnread(data.unread), updates: updates };
    lastFailed = false;
    setStale(false);
    if (fresh) {
      setBadge(last.unread);
    }
    if (open && !shownRows) {
      renderOpen(false);
    }
  }

  function runQueued() {
    if (queued) {
      queued = false;
      poll();
    }
  }

  // poll asks for the bell once. A request during a request queues exactly one
  // more poll, never more.
  function poll() {
    if (stopped) {
      return;
    }
    if (inFlight) {
      queued = true;
      return;
    }
    inFlight = true;
    pollSeq += 1;
    var seq = pollSeq;
    request(LIST_URL, {
      method: 'GET',
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { Accept: 'application/json' }
    }, REQUEST_TIMEOUT_MS).then(function (answer) {
      inFlight = false;
      onAnswer(answer, seq);
      runQueued();
    }, function () {
      inFlight = false;
      onFailure();
      runQueued();
    });
  }

  function retry() {
    if (last === null) {
      setStale(false);
      showOnly('loading');
    }
    poll();
  }

  // ---- My activity: the Updates rows on view -------------------------------

  // partName is the part screen My activity shows, empty on the list.
  function partName() {
    var main = document.querySelector('main.profile-page');
    return main ? text(main.getAttribute('data-part')) : '';
  }

  // markPartRead marks the Updates rows of My activity read once when they are
  // on view: the whole stack on a laptop, the Updates screen on a phone (UI-SPEC
  // B1, B2). It returns true when it sent the mark, which then polls.
  function markPartRead() {
    var list = document.getElementById('updates-rows');
    if (!list) {
      return false;
    }
    var onView = (typeof window.matchMedia === 'function' && window.matchMedia(LAPTOP_QUERY).matches) ||
      partName() === 'updates';
    if (!onView) {
      return false;
    }
    var items = list.querySelectorAll('li');
    var maxId = 0;
    var anyNew = false;
    for (var i = 0; i < items.length; i++) {
      var id = Number(items[i].getAttribute('data-update-id'));
      if (isFinite(id) && id > maxId) {
        maxId = id;
      }
      var cls = text(items[i].getAttribute('class'));
      if (cls.split(' ').indexOf('urow--new') !== -1 || items[i].getAttribute('data-unread') === 'true') {
        anyNew = true;
      }
    }
    if (!anyNew || maxId <= 0) {
      return false;
    }
    sendRead(maxId);
    return true;
  }

  // ---- The reader's box for their own removed report (quick 261118-rz6) ----

  // The box is the #review-ask-box dialog the page renders as a child of body.
  // Without it, or without dialog support, a box row does nothing. Every word
  // in it is the template's or the server's: the Why line and a refused ask's
  // line come from the answer, the rest is in the template.
  var askBox = document.getElementById('review-ask-box');
  var ask = null;
  if (askBox && typeof askBox.showModal === 'function') {
    ask = {
      title: document.getElementById('review-ask-title'),
      why: document.getElementById('review-ask-why'),
      form: document.getElementById('review-ask-form'),
      note: document.getElementById('review-ask-note'),
      asked: document.getElementById('review-ask-asked'),
      none: document.getElementById('review-ask-none'),
      error: document.getElementById('review-ask-error'),
      close: document.getElementById('review-ask-close'),
      send: document.getElementById('review-ask-send'),
      id: '', // the report the open box is about
      opener: null, // the row button that opened it
      busy: false
    };
    for (var part in ask) {
      if (Object.prototype.hasOwnProperty.call(ask, part) && ask[part] === null && part !== 'opener') {
        ask = null;
        break;
      }
    }
  }

  function askAddress(id) {
    return REVIEW_ASK_PREFIX + id + REVIEW_ASK_SUFFIX;
  }

  function showAskError(words) {
    ask.error.textContent = text(words) !== '' ? words : text(ask.error.getAttribute('data-failed'));
    ask.error.hidden = false;
  }

  // renderAsk draws one state of the box from the server's answer: the Why
  // line when there is one, then the note and Ask for review (can_ask), the
  // Asked. line (asked) or the No asks left. line (no_asks_left).
  function renderAsk(body) {
    var why = body && typeof body.why === 'string' ? body.why : '';
    var state = body && typeof body.state === 'string' ? body.state : '';
    ask.why.textContent = why;
    ask.why.hidden = why === '';
    ask.form.hidden = state !== 'can_ask';
    ask.send.hidden = state !== 'can_ask';
    ask.asked.hidden = state !== 'asked';
    ask.none.hidden = state !== 'no_asks_left';
    ask.error.hidden = true;
  }

  function setAskBusy(on) {
    ask.busy = on;
    if (on) {
      ask.send.setAttribute('aria-disabled', 'true');
    } else {
      ask.send.removeAttribute('aria-disabled');
    }
  }

  // openAsk reads the box of one report, then opens it with the title taking
  // the first focus, so no button looks chosen. A report that is no longer the
  // reader's removed report (restored since, say) answers 404: the box stays
  // shut and the bell reads again, which draws that row plain.
  function openAsk(id, opener) {
    if (!ask || ask.busy || askBox.open) {
      return;
    }
    ask.id = id;
    ask.opener = opener;
    ask.note.value = '';
    setAskBusy(true);
    function show() {
      setAskBusy(false);
      askBox.showModal();
      ask.title.focus();
    }
    request(askAddress(id), {
      method: 'GET',
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { Accept: 'application/json' }
    }, REQUEST_TIMEOUT_MS).then(function (answer) {
      if (answer.status === 404) {
        setAskBusy(false);
        poll();
        return;
      }
      if (answer.ok !== true) {
        renderAsk(null);
        showAskError('');
        show();
        return;
      }
      renderAsk(answer.body);
      show();
    }, function () {
      renderAsk(null);
      showAskError('');
      show();
    });
  }

  // sendAsk sends Ask for review with the note. On success the box shows
  // Asked. in place. A refused ask (409) reads the box again, so it shows
  // Asked. or No asks left. A too long note (400) shows the server's line.
  function sendAsk() {
    if (!ask || ask.busy || ask.id === '') {
      return;
    }
    setAskBusy(true);
    ask.error.hidden = true;
    var id = ask.id;
    request(askAddress(id), {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ note: ask.note.value })
    }, REQUEST_TIMEOUT_MS).then(function (answer) {
      setAskBusy(false);
      if (answer.ok === true) {
        renderAsk(answer.body);
        ask.close.focus();
        poll();
        return;
      }
      if (answer.status === 409) {
        setAskBusy(true);
        return request(askAddress(id), {
          method: 'GET',
          credentials: 'same-origin',
          cache: 'no-store',
          headers: { Accept: 'application/json' }
        }, REQUEST_TIMEOUT_MS).then(function (again) {
          setAskBusy(false);
          if (again.ok === true) {
            renderAsk(again.body);
            ask.close.focus();
          } else {
            showAskError('');
          }
        });
      }
      if (answer.status === 404) {
        closeAsk();
        poll();
        return;
      }
      var line = answer.body && answer.body.error && typeof answer.body.error.message === 'string' ? answer.body.error.message : '';
      showAskError(answer.status === 400 ? line : '');
      return null;
    }).then(null, function () {
      setAskBusy(false);
      showAskError('');
    });
  }

  function closeAsk() {
    if (askBox.open) {
      askBox.close();
    }
  }

  // onAskClosed runs however the box closes (Close, or Escape on a keyboard).
  // Focus goes back to the row that opened it, or to the bell when that row
  // sat in the bell list, which is closed by now.
  function onAskClosed() {
    var back = ask.opener;
    ask.opener = null;
    ask.id = '';
    ask.busy = false;
    if (back && panel.contains(back) && panel.hidden) {
      back = trigger;
    }
    if (back && typeof back.focus === 'function') {
      back.focus();
    }
  }

  // onBoxRowClick opens the box for any box row on the page: the bell's and
  // the full updates list's. The id is checked before any address is built.
  function onBoxRowClick(e) {
    var t = e.target;
    var row = t && typeof t.closest === 'function' ? t.closest('[data-review-ask]') : null;
    if (!row) {
      return;
    }
    var id = text(row.getAttribute('data-review-ask'));
    if (!ID_PATTERN.test(id)) {
      return;
    }
    e.preventDefault();
    openAsk(id, row);
  }

  if (ask) {
    document.addEventListener('click', onBoxRowClick);
    ask.close.addEventListener('click', closeAsk);
    ask.send.addEventListener('click', sendAsk);
    askBox.addEventListener('close', onAskClosed);
  }

  // ---- Start --------------------------------------------------------------

  var interval = parseInt(trigger.getAttribute('data-poll-interval-ms'), 10);
  if (!isFinite(interval) || interval <= 0) {
    interval = DEFAULT_INTERVAL_MS;
  }
  intervalId = window.setInterval(function () {
    if (!document.hidden) {
      poll();
    }
  }, interval);

  document.addEventListener('visibilitychange', function () {
    if (!document.hidden) {
      poll();
    }
  });
  // A page restored from the back and forward cache missed its timer.
  window.addEventListener('pageshow', function (e) {
    if (e && e.persisted === true) {
      poll();
    }
  });
  document.addEventListener('pinalert:updates-refresh', poll);

  trigger.addEventListener('click', syncOpenState);
  if (typeof MutationObserver === 'function') {
    new MutationObserver(syncOpenState).observe(panel, { attributes: true, attributeFilter: ['hidden'] });
  }
  var retryButtons = [errorEl, staleEl];
  for (var r = 0; r < retryButtons.length; r++) {
    var button = retryButtons[r] ? retryButtons[r].querySelector('.bell-list__retry') : null;
    if (button) {
      button.addEventListener('click', retry);
    }
  }

  window.PinalertUpdates = {
    refresh: poll,
    render: render,
    badgeText: badgeText,
    buttonLabel: buttonLabel
  };

  if (!markPartRead()) {
    poll();
  }
}());
