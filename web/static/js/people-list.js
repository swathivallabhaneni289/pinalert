// people-list.js - window.PinalertPeopleList: the one list window of Followers,
// Following, Muted people and Blocked people (Phase 9 plan 11, D-03, UI-SPEC
// Surface 4; the muted kind is plan 20, D-12).
//
// open({kind, username, own, opener, onChange}) opens the window for one list:
//   kind     'followers', 'following', 'muted' or 'blocked';
//   username the profile whose list it is (empty for 'muted' and 'blocked', which
//            are yours);
//   own      true on your own profile;
//   opener   the control that opened it, which gets focus back when it closes;
//   onChange a function called after the list changed the profile, for example
//            after Remove all followers, so the opener can redraw its counts.
//
// The server decides who is named. A named person is a row that opens their
// profile. Everyone else is one plain line the server worded, such as "and 3
// others": it is never a link and never takes focus. This script counts nobody
// and decides nothing about who may be shown.
//
// The window is closed only by its X and Escape (Escape closes the confirm box
// first when it is open). The page behind it is inert while it is open. Blocked
// and Muted rows carry an outlined Unblock or Unmute button and no link, and
// always show the username (a muted person may have gone private since).
//
// SECURITY (T-09-41, T-09-42): every word reaches the page through textContent,
// every node comes from createElement, and an address is built only from a
// username passed through encodeURIComponent. No markup is ever parsed.
window.PinalertPeopleList = (function () {
  'use strict';

  var REQUEST_TIMEOUT_MS = 10000;
  var PEOPLE_URL = '/api/people/';
  var BLOCKED_URL = '/api/me/blocked';
  var MUTED_URL = '/api/me/muted-people';
  var REMOVE_ALL_URL = '/api/me/followers/remove-all';

  var TITLES = {
    followers: 'Followers',
    following: 'Following',
    muted: 'Muted people',
    blocked: 'Blocked people'
  };
  var MESSAGE_OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
  // The header the server's unmute and unblock answers carry (handlers.ChangedHeader).
  var CHANGED_HEADER = 'Pinalert-Changed';
  // The calm line when the server says the tap changed nothing, for example the
  // person changed their username since the list was read (09-REVIEW IN-B03).
  var CURSOR_REFUSED_LINE = 'That page could not be read.';
  var NOTHING_CHANGED_LINE ='That person is no longer listed under that name. The list is up to date now.';
  var UNBLOCK_FAILED = 'Couldn\'t unblock. Try again.';
  var UNMUTE_FAILED = 'Couldn\'t unmute. Try again.';
  // The two lists whose rows are plain (no link) and end in an outlined button
  // that undoes the action at once (UI-SPEC 4b). Both share one row and one
  // focus routine.
  var UNDO = {
    blocked: { path: '/unblock', button: 'Unblock', done: ' is unblocked.', failed: UNBLOCK_FAILED },
    muted: { path: '/unmute', button: 'Unmute', done: ' is unmuted.', failed: UNMUTE_FAILED }
  };
  var REMOVE_ALL_FAILED = 'Couldn\'t remove followers. Try again.';

  var els = null;
  var inited = false;
  // The state of the open list.
  var open_ = false;
  var kind = '';
  var subject = '';
  var own = false;
  var opener = null;
  var onChange = null;
  var next = '';
  var loadSeq = 0;
  var busy = false;
  var lockedNodes = [];

  function byId(id) {
    return document.getElementById(id);
  }

  function setText(node, value) {
    if (node) {
      node.textContent = value === null || value === undefined ? '' : String(value);
    }
  }

  function say(message) {
    var live = byId('notice-live');
    if (live) {
      live.textContent = message;
    }
  }

  // ---- The words of the empty states (UI-SPEC 4c) ---------------------------

  function emptyWords() {
    if (kind === 'followers') {
      return {
        title: 'No followers yet',
        line: own ? 'People who follow you will show here.' : 'Nobody follows ' + subject + ' yet.'
      };
    }
    if (kind === 'following') {
      return own
        ? { title: 'You are not following anyone', line: 'People you follow show here.' }
        : { title: 'Not following anyone', line: subject + ' does not follow anyone yet.' };
    }
    if (kind === 'muted') {
      return { title: 'No muted people', line: 'People you mute show here, with a way to unmute them.' };
    }
    return { title: 'No blocked people', line: 'People you block show here, with a way to unblock them.' };
  }

  // ---- Page lock ---------------------------------------------------------------

  // lockPage silences every child of the body except the window, the confirm
  // box (a native modal that makes the rest inert itself) and the polite status
  // region, so Tab and assistive technology stay inside the window while the
  // status line is still read aloud.
  function lockPage() {
    unlockPage();
    var kids = Array.prototype.slice.call(document.body.children);
    for (var i = 0; i < kids.length; i++) {
      var node = kids[i];
      if (node === els.win || node === els.confirm || node.tagName === 'SCRIPT' || node.id === 'notice-live') {
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

  // ---- Requests -----------------------------------------------------------------

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
        // changed is the server's Pinalert-Changed header on an unmute or an
        // unblock: '1' a row was deleted, '0' nothing was there (09-REVIEW
        // IN-B03). Null when the header is absent.
        var changed = res.headers && typeof res.headers.get === 'function' ? res.headers.get(CHANGED_HEADER) : null;
        return { status: res.status, ok: res.ok, body: body, changed: changed };
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

  // refusalLine is the line for a failed answer: the server's own sentence for
  // a 409 or a 429 when it sent one, the built line otherwise (09-REVIEW IN-B01).
  function refusalLine(answer, fallback) {
    var error = answer && answer.body && answer.body.error;
    if ((answer.status === 409 || answer.status === 429) && error &&
        typeof error.message === 'string' && error.message !== '') {
      return error.message;
    }
    return fallback;
  }

  // rejectionLine is the line for a call that did not get an answer: the
  // offline line when the signal is gone, the built line otherwise.
  function rejectionLine(fallback) {
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return MESSAGE_OFFLINE;
    }
    return fallback;
  }

  function listUrl(after) {
    var base;
    if (kind === 'blocked') {
      base = BLOCKED_URL;
    } else if (kind === 'muted') {
      base = MUTED_URL;
    } else {
      base = PEOPLE_URL + encodeURIComponent(subject) + '/' + kind;
    }
    return after ? base + '?after=' + encodeURIComponent(after) : base;
  }

  function post(url) {
    return request(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    });
  }

  // ---- Drawing ------------------------------------------------------------------

  function showState(name) {
    els.loading.hidden = name !== 'loading';
    els.rows.hidden = name !== 'list';
    els.empty.hidden = name !== 'empty';
    els.error.hidden = name !== 'error';
    if (name !== 'list') {
      els.others.hidden = true;
      els.more.hidden = true;
      els.removeAll.hidden = true;
    }
  }

  function showActionError(message) {
    setText(els.actionError, message);
    els.actionError.hidden = false;
  }

  function hideActionError() {
    setText(els.actionError, '');
    els.actionError.hidden = true;
  }

  // initialOf is the letter of the disc: the first ASCII letter of the username.
  function initialOf(name) {
    var m = /[A-Za-z]/.exec(name);
    return m ? m[0].toUpperCase() : '?';
  }

  function makeDisc(name) {
    var disc = document.createElement('span');
    disc.className = 'people-row__disc';
    disc.setAttribute('aria-hidden', 'true');
    setText(disc, initialOf(name));
    return disc;
  }

  function makeName(name) {
    var label = document.createElement('span');
    label.className = 'people-row__name';
    setText(label, name);
    return label;
  }

  // namedRow is a link row: the letter disc, the username and a chevron.
  function namedRow(name) {
    var li = document.createElement('li');
    li.className = 'people-row';
    var link = document.createElement('a');
    link.className = 'people-row__link';
    link.setAttribute('href', '/u/' + encodeURIComponent(name));
    link.appendChild(makeDisc(name));
    link.appendChild(makeName(name));
    var chevron = document.createElement('span');
    chevron.className = 'app-icon app-icon--chevron-right people-row__chevron';
    chevron.setAttribute('aria-hidden', 'true');
    link.appendChild(chevron);
    li.appendChild(link);
    return li;
  }

  // undoRow has no link: the username and an outlined Unblock or Unmute button,
  // the word following the list it is drawn in.
  function undoRow(name) {
    var undo = UNDO[kind];
    var li = document.createElement('li');
    li.className = 'people-row people-row--plain';
    li.setAttribute('data-username', name);
    li.appendChild(makeDisc(name));
    li.appendChild(makeName(name));
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn people-row__action';
    setText(button, undo.button);
    button.addEventListener('click', function () { undoOne(undo, name, li, button); });
    li.appendChild(button);
    return li;
  }

  // firstNewRow is the first row the last appendRows drew, or null when it drew
  // none. Show more puts focus on it (09-REVIEW WR-B06).
  var firstNewRow = null;

  function appendRows(entries) {
    firstNewRow = null;
    for (var i = 0; i < entries.length; i++) {
      var name = entries[i] && typeof entries[i].username === 'string' ? entries[i].username : '';
      if (name === '') {
        continue;
      }
      var row = UNDO[kind] ? undoRow(name) : namedRow(name);
      if (firstNewRow === null) {
        firstNewRow = row;
      }
      els.rows.appendChild(row);
    }
  }

  function clearRows() {
    while (els.rows.firstChild) {
      els.rows.removeChild(els.rows.firstChild);
    }
  }

  function showEmpty(focus) {
    var words = emptyWords();
    setText(els.emptyTitle, words.title);
    setText(els.emptyLine, words.line);
    showState('empty');
    if (focus) {
      els.emptyTitle.focus();
    }
  }

  // draw shows one page of an answer. first is true for the first page of the
  // list, false for a Show more page that is appended.
  function draw(body, first) {
    var entries = body && Array.isArray(body.entries) ? body.entries : [];
    var othersCount = body && typeof body.others_count === 'number' ? body.others_count : 0;
    var othersText = body && typeof body.others_text === 'string' ? body.others_text : '';
    if (first) {
      clearRows();
    }
    appendRows(entries);
    next = body && typeof body.next === 'string' ? body.next : '';

    var anyRows = els.rows.firstChild !== null;
    if (!anyRows && othersCount === 0) {
      showEmpty(false);
      return;
    }
    showState('list');
    els.rows.hidden = !anyRows;
    if (othersCount > 0 && othersText !== '') {
      setText(els.others, othersText);
      els.others.hidden = false;
    } else {
      els.others.hidden = true;
    }
    els.more.hidden = next === '';
    // Remove all followers is drawn on your own Followers list whenever someone
    // follows you, named or only counted.
    els.removeAll.hidden = !(kind === 'followers' && own && (anyRows || othersCount > 0));
  }

  // ---- Loading ------------------------------------------------------------------

  function load(after) {
    var first = !after;
    loadSeq += 1;
    var seq = loadSeq;
    hideActionError();
    if (first) {
      showState('loading');
    }
    return request(listUrl(after), {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    }).then(function (answer) {
      if (seq !== loadSeq || !open_) {
        return false;
      }
      if (!answer.ok) {
        if (!first && answer.status === 400) {
          return startAgain(answer);
        }
        failLoad(first);
        return false;
      }
      draw(answer.body, first);
      return true;
    }, function () {
      if (seq === loadSeq && open_) {
        failLoad(first);
      }
      return false;
    });
  }

  // startAgain answers a Show more the server refused with a 400: the cursor no
  // longer opens (one issued before a server restart). Sending the same cursor
  // again can never work, so the list is read again from the top with an empty
  // cursor, and the server's own sentence is shown once in the window's message
  // place after the read (the read clears it). If the read from the top fails
  // too, the whole list shows its error state with Try again, which also starts
  // from the top, so nothing loops (skeptic SK-W02, 2026-10-30).
  function startAgain(answer) {
    var error = answer && answer.body && answer.body.error;
    var sentence = error && typeof error.message === 'string' && error.message !== '' ? error.message : CURSOR_REFUSED_LINE;
    return load('').then(function (read) {
      if (read && open_) {
        showActionError(sentence);
      }
      return read;
    });
  }

  function failLoad(first) {
    if (first) {
      showState('error');
    } else {
      els.more.hidden = false;
      showActionError('Couldn\'t load this list');
    }
  }

  // ---- Unblock and Unmute ---------------------------------------------------------

  // undoOne sends the undo of one row (unblock or unmute, no confirmation),
  // removes the row, says the status line and moves focus to the next row's
  // button, or to the empty heading when the list is now empty.
  function undoOne(undo, name, li, button) {
    if (busy) {
      return Promise.resolve(false);
    }
    busy = true;
    hideActionError();
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    function settle() {
      busy = false;
      button.disabled = false;
      button.removeAttribute('aria-busy');
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      settle();
      showActionError(MESSAGE_OFFLINE);
      return Promise.resolve(false);
    }
    return post(PEOPLE_URL + encodeURIComponent(name) + undo.path).then(function (answer) {
      if (!answer.ok) {
        settle();
        showActionError(refusalLine(answer, undo.failed));
        return false;
      }
      if (answer.changed === '0') {
        // The server found nothing to undo under this name (it was renamed since
        // the list was read, or the row is already gone). Saying "unmuted" would
        // be untrue, so the list is read again and the count is left alone.
        settle();
        return load('').then(function (read) {
          if (!read) {
            return false;
          }
          // The line goes in the window's own message place, after the read (the
          // read clears it), and no "is unmuted" sentence is said.
          showActionError(NOTHING_CHANGED_LINE);
          var buttons = els.rows.querySelectorAll('button');
          if (buttons.length > 0) {
            buttons[0].focus();
          } else if (!els.empty.hidden) {
            els.emptyTitle.focus();
          }
          return false;
        });
      }
      busy = false;
      var after = li.nextElementSibling || li.nextSibling;
      els.rows.removeChild(li);
      say(name + undo.done);
      if (els.rows.firstChild === null) {
        showEmpty(true);
      } else {
        var target = after && typeof after.querySelector === 'function' ? after.querySelector('button') : null;
        if (!target) {
          var buttons = els.rows.querySelectorAll('button');
          target = buttons.length > 0 ? buttons[buttons.length - 1] : null;
        }
        if (target) {
          target.focus();
        }
      }
      if (typeof onChange === 'function') {
        onChange();
      }
      return true;
    }, function () {
      settle();
      showActionError(rejectionLine(undo.failed));
      return false;
    });
  }

  // ---- Remove all followers -------------------------------------------------------

  function canAskInBox() {
    return !!els.confirm && typeof els.confirm.showModal === 'function';
  }

  function askRemoveAll() {
    if (busy) {
      return;
    }
    if (!canAskInBox()) {
      if (window.confirm('Remove all followers?')) {
        runRemoveAll();
      }
      return;
    }
    setText(els.confirmError, '');
    els.confirmError.hidden = true;
    els.confirm.showModal();
    // The native box opens with focus on the question, so no button looks
    // chosen and nothing is sent until Remove all is pressed.
    els.confirmTitle.focus();
  }

  function closeConfirm() {
    if (els.confirm && els.confirm.open) {
      els.confirm.close();
    }
  }

  function runRemoveAll() {
    if (busy) {
      return Promise.resolve(false);
    }
    busy = true;
    els.confirmGo.disabled = true;
    els.confirmCancel.disabled = true;
    function settle() {
      busy = false;
      els.confirmGo.disabled = false;
      els.confirmCancel.disabled = false;
    }
    function fail(message) {
      settle();
      if (canAskInBox() && els.confirm.open) {
        setText(els.confirmError, message);
        els.confirmError.hidden = false;
        els.confirmTitle.focus();
      } else {
        showActionError(message);
      }
      return false;
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return Promise.resolve(fail(MESSAGE_OFFLINE));
    }
    return post(REMOVE_ALL_URL).then(function (answer) {
      if (!answer.ok) {
        return fail(refusalLine(answer, REMOVE_ALL_FAILED));
      }
      settle();
      closeConfirm();
      clearRows();
      next = '';
      showEmpty(true);
      say('No one follows you now.');
      if (typeof onChange === 'function') {
        onChange();
      }
      return true;
    }, function () {
      return fail(rejectionLine(REMOVE_ALL_FAILED));
    });
  }

  // ---- Opening and closing --------------------------------------------------------

  function focusables() {
    var all = els.win.querySelectorAll('a[href], button');
    var out = [];
    for (var i = 0; i < all.length; i++) {
      if (!all[i].disabled && all[i].hidden !== true) {
        var hiddenAbove = false;
        var node = all[i].parentNode;
        while (node && node !== els.win) {
          if (node.hidden === true) {
            hiddenAbove = true;
            break;
          }
          node = node.parentNode;
        }
        if (!hiddenAbove) {
          out.push(all[i]);
        }
      }
    }
    return out;
  }

  function openWindow(options) {
    init();
    if (!els || !options || !TITLES[options.kind]) {
      return;
    }
    kind = options.kind;
    subject = typeof options.username === 'string' ? options.username : '';
    own = options.own === true || kind === 'blocked' || kind === 'muted';
    opener = options.opener || null;
    onChange = typeof options.onChange === 'function' ? options.onChange : null;
    next = '';
    busy = false;
    open_ = true;

    setText(els.title, TITLES[kind]);
    if (!UNDO[kind] && !own && subject !== '') {
      setText(els.subtitle, subject);
      els.subtitle.hidden = false;
    } else {
      setText(els.subtitle, '');
      els.subtitle.hidden = true;
    }
    hideActionError();
    els.win.hidden = false;
    lockPage();
    els.title.focus();
    load('');
  }

  function closeWindow(restoreFocus) {
    if (!open_) {
      return;
    }
    open_ = false;
    loadSeq += 1;
    closeConfirm();
    els.win.hidden = true;
    unlockPage();
    if (restoreFocus !== false && opener && typeof opener.focus === 'function') {
      opener.focus();
    }
  }

  function onWindowKeydown(evt) {
    if (evt.key === 'Escape') {
      if (els.confirm && els.confirm.open) {
        return;
      }
      evt.preventDefault();
      closeWindow(true);
      return;
    }
    if (evt.key !== 'Tab') {
      return;
    }
    var items = focusables();
    if (items.length === 0) {
      return;
    }
    var first = items[0];
    var last = items[items.length - 1];
    var active = document.activeElement;
    if (evt.shiftKey && (active === first || !els.win.contains(active))) {
      evt.preventDefault();
      last.focus();
    } else if (!evt.shiftKey && (active === last || !els.win.contains(active))) {
      evt.preventDefault();
      first.focus();
    }
  }

  function on(node, type, fn) {
    if (node) {
      node.addEventListener(type, fn);
    }
  }

  function init() {
    if (inited) {
      return;
    }
    var win = byId('people-list-window');
    if (!win) {
      return;
    }
    inited = true;
    els = {
      win: win,
      title: byId('people-list-title'),
      subtitle: byId('people-list-subtitle'),
      close: byId('people-list-close'),
      actionError: byId('people-list-action-error'),
      loading: byId('people-list-loading'),
      rows: byId('people-list-rows'),
      others: byId('people-list-others'),
      more: byId('people-list-more'),
      removeAll: byId('people-list-remove-all'),
      empty: byId('people-list-empty'),
      emptyTitle: byId('people-list-empty-title'),
      emptyLine: byId('people-list-empty-line'),
      error: byId('people-list-error'),
      retry: byId('people-list-retry'),
      confirm: byId('people-list-confirm'),
      confirmTitle: byId('people-list-confirm-title'),
      confirmError: byId('people-list-confirm-error'),
      confirmCancel: byId('people-list-confirm-cancel'),
      confirmGo: byId('people-list-confirm-go')
    };
    on(els.close, 'click', function () { closeWindow(true); });
    on(win, 'keydown', onWindowKeydown);
    on(els.retry, 'click', function () { load(''); });
    // Show more stays on the page, disabled and busy, while the next page loads,
    // so focus is never dropped. It then goes to the first new row, or back to
    // the button when there is none (09-REVIEW WR-B06).
    on(els.more, 'click', function () {
      if (els.more.disabled) {
        return;
      }
      els.more.disabled = true;
      els.more.setAttribute('aria-busy', 'true');
      firstNewRow = null;
      load(next).then(function (ok) {
        els.more.disabled = false;
        els.more.removeAttribute('aria-busy');
        if (!open_) {
          return;
        }
        var target = null;
        if (ok && firstNewRow !== null && typeof firstNewRow.querySelector === 'function') {
          target = firstNewRow.querySelector(UNDO[kind] ? 'button' : 'a');
        }
        if (target) {
          target.focus();
        } else if (els.more.hidden === false) {
          els.more.focus();
        } else if (els.rows.hidden === false) {
          var links = els.rows.querySelectorAll(UNDO[kind] ? 'button' : 'a');
          if (links.length > 0) {
            links[links.length - 1].focus();
          }
        }
      });
    });
    on(els.removeAll, 'click', askRemoveAll);
    on(els.confirmGo, 'click', function () { runRemoveAll(); });
    on(els.confirmCancel, 'click', function () {
      if (!busy) {
        closeConfirm();
      }
    });
    // Escape is the browser's cancel event, the same as Cancel except while the
    // call is being sent. There is no click handler on the box itself, so a tap
    // on the dim page does nothing.
    on(els.confirm, 'cancel', function (evt) {
      if (busy) {
        evt.preventDefault();
      }
    });
    on(els.confirm, 'close', function () {
      if (open_ && els.removeAll && els.removeAll.hidden === false) {
        els.removeAll.focus();
      }
    });
  }

  if (typeof document !== 'undefined') {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init);
    } else {
      init();
    }
  }

  return {
    open: openWindow,
    close: closeWindow
  };
}());
