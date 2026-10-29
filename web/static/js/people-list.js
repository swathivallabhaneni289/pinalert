// people-list.js - window.PinalertPeopleList: the one list window of Followers,
// Following and Blocked people (Phase 9 plan 11, D-03, UI-SPEC Surface 4).
//
// open({kind, username, own, opener, onChange}) opens the window for one list:
//   kind     'followers', 'following' or 'blocked';
//   username the profile whose list it is (empty for 'blocked', which is yours);
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
// rows carry an outlined Unblock button and no link.
//
// SECURITY (T-09-41, T-09-42): every word reaches the page through textContent,
// every node comes from createElement, and an address is built only from a
// username passed through encodeURIComponent. No markup is ever parsed.
window.PinalertPeopleList = (function () {
  'use strict';

  var REQUEST_TIMEOUT_MS = 10000;
  var PEOPLE_URL = '/api/people/';
  var BLOCKED_URL = '/api/me/blocked';
  var REMOVE_ALL_URL = '/api/me/followers/remove-all';

  var TITLES = {
    followers: 'Followers',
    following: 'Following',
    blocked: 'Blocked people'
  };
  var MESSAGE_OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
  var UNBLOCK_FAILED = 'Couldn\'t unblock. Try again.';
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

  function listUrl(after) {
    var base = kind === 'blocked'
      ? BLOCKED_URL
      : PEOPLE_URL + encodeURIComponent(subject) + '/' + kind;
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

  // blockedRow has no link: the username and an outlined Unblock button.
  function blockedRow(name) {
    var li = document.createElement('li');
    li.className = 'people-row people-row--plain';
    li.setAttribute('data-username', name);
    li.appendChild(makeDisc(name));
    li.appendChild(makeName(name));
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'btn people-row__action';
    setText(button, 'Unblock');
    button.addEventListener('click', function () { unblock(name, li, button); });
    li.appendChild(button);
    return li;
  }

  function appendRows(entries) {
    for (var i = 0; i < entries.length; i++) {
      var name = entries[i] && typeof entries[i].username === 'string' ? entries[i].username : '';
      if (name === '') {
        continue;
      }
      els.rows.appendChild(kind === 'blocked' ? blockedRow(name) : namedRow(name));
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

  function failLoad(first) {
    if (first) {
      showState('error');
    } else {
      els.more.hidden = false;
      showActionError('Couldn\'t load this list');
    }
  }

  // ---- Unblock --------------------------------------------------------------------

  function unblock(name, li, button) {
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
    return post(PEOPLE_URL + encodeURIComponent(name) + '/unblock').then(function (answer) {
      if (!answer.ok) {
        settle();
        showActionError(UNBLOCK_FAILED);
        return false;
      }
      busy = false;
      var after = li.nextElementSibling || li.nextSibling;
      els.rows.removeChild(li);
      say(name + ' is unblocked.');
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
      showActionError(UNBLOCK_FAILED);
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
        return fail(REMOVE_ALL_FAILED);
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
      return fail(REMOVE_ALL_FAILED);
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
    own = options.own === true || kind === 'blocked';
    opener = options.opener || null;
    onChange = typeof options.onChange === 'function' ? options.onChange : null;
    next = '';
    busy = false;
    open_ = true;

    setText(els.title, TITLES[kind]);
    if (kind !== 'blocked' && !own && subject !== '') {
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
    on(els.more, 'click', function () {
      els.more.hidden = true;
      load(next);
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
