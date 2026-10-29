// person.js - window.PinalertPerson: the actions of the profile page (Phase 9
// plan 08, D-12, UI-SPEC 1d, 1e, 1g, 1h).
//
// The page is drawn by the server. This script only does what a person taps:
// Follow and Following (a one tap toggle), the More menu, Mute and Unmute (at
// once, no box), Remove follower and Block (each asks first in a system style
// box), and Unblock. It decides
// nothing about trust: the browser computes no star, no tag, no count and no
// eligibility. After an action it asks the server for the profile again and
// draws the strings and the booleans that come back.
//
// What it never does:
//   - It never parses markup. Every string reaches the page through
//     textContent, and every node it makes comes from createElement. Usernames
//     come from the data-username attribute of the page, which the server wrote
//     through html/template escaping.
//   - It never builds a button. Follow and Following are both in the page, the
//     server hides the one that does not apply, and this script only toggles the
//     hidden flags. The one exception is the Your rating control (plan 09-16),
//     whose five star buttons come from the shared stars.js and are mounted only
//     when the server's may_rate is true.
//   - It never closes the confirm box except through its own two buttons or
//     Escape, and it has no click handler on the box itself, so a tap on the dim
//     page does nothing.
//
// SECURITY (T-09-29): no markup sink of any kind. The only address it sends to
// is /api/people/ plus the encoded username of the page.
window.PinalertPerson = (function () {
  'use strict';

  var REQUEST_TIMEOUT_MS = 10000;
  var PEOPLE_URL = '/api/people/';

  // The words of the UI-SPEC (P9-C8, P9-C13, P9-C22). Each is shown as it is.
  var MESSAGE_OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
  var FAILED = {
    follow: 'Couldn\'t follow. Try again.',
    unfollow: 'Couldn\'t unfollow. Try again.',
    mute: 'Couldn\'t mute. Try again.',
    unmute: 'Couldn\'t unmute. Try again.',
    'remove-follower': 'Couldn\'t remove this follower. Try again.',
    block: 'Couldn\'t block. Try again.',
    unblock: 'Couldn\'t unblock. Try again.'
  };
  var BOX = {
    'remove-follower': {
      title: function (name) { return 'Remove ' + name + ' as a follower?'; },
      texts: ['They stop following you. They can follow you again.'],
      go: 'Remove'
    },
    block: {
      title: function (name) { return 'Block ' + name + '?'; },
      texts: [
        'They can no longer see your profile or follow you, and you stop following each other.',
        'Their reports still show in the feed. You will not see their comments.'
      ],
      go: 'Block'
    }
  };

  var inited = false;
  var page = null;
  var username = '';
  var busy = false;
  var asking = '';
  var blockedNow = false;
  // control is the Your rating control of plan 09-16, or null when the server
  // said this reader may not rate. stars.js draws it; this script only mounts it.
  var control = null;

  var els = {};

  function byId(id) {
    return document.getElementById(id);
  }

  function setText(node, value) {
    if (node) {
      node.textContent = value === null || value === undefined ? '' : String(value);
    }
  }

  function say(message) {
    if (els.live) {
      els.live.textContent = message;
    }
  }

  // errorLine is the line that is on screen: the one in the blocked box while
  // that box shows, the one under the buttons otherwise.
  function errorLine() {
    if (els.blocked && els.blocked.hidden === false && els.blockedError) {
      return els.blockedError;
    }
    return els.error;
  }

  function showError(message) {
    var line = errorLine();
    if (line) {
      setText(line, message);
      line.hidden = false;
    }
  }

  function hideError() {
    var lines = [els.error, els.blockedError];
    for (var i = 0; i < lines.length; i++) {
      if (lines[i]) {
        setText(lines[i], '');
        lines[i].hidden = true;
      }
    }
  }

  function actionUrl(name) {
    return PEOPLE_URL + encodeURIComponent(username) + (name ? '/' + name : '');
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

  function post(name) {
    return request(actionUrl(name), {
      method: 'POST',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    });
  }

  function reload() {
    window.location.reload();
  }

  // serverMessage is the sentence the server wrote for a refusal, when it sent
  // one in its error envelope.
  function serverMessage(answer) {
    var error = answer && answer.body && answer.body.error;
    if (error && typeof error.message === 'string' && error.message !== '') {
      return error.message;
    }
    return '';
  }

  // refusal turns a failed answer into the one line to show. A 404 redraws the
  // whole page from the server (it will say Profile not found), so it returns
  // null.
  function refusal(answer, action) {
    if (answer.status === 404) {
      reload();
      return null;
    }
    if ((answer.status === 409 || answer.status === 429) && serverMessage(answer) !== '') {
      return serverMessage(answer);
    }
    return FAILED[action];
  }

  function offlineOr(action) {
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return MESSAGE_OFFLINE;
    }
    return FAILED[action];
  }

  // ---- Redraw from the server's profile -------------------------------------

  function drawCount(countEl, wordEl, button, count, word, text) {
    setText(countEl, count);
    setText(wordEl, word);
    if (button && typeof text === 'string') {
      button.setAttribute('aria-label', text);
    }
  }

  // ---- The Your rating control (plan 09-16) -----------------------------------

  // mountRating draws the shared star control into the rating box. It does
  // nothing when stars.js did not load or when there is no box.
  function mountRating(yours) {
    var stars = typeof window !== 'undefined' ? window.PinalertStars : null;
    if (control !== null || !els.rating || !stars || typeof stars.createControl !== 'function') {
      return;
    }
    control = stars.createControl({
      username: username,
      current: yours,
      legend: 'Your rating',
      onStatus: say,
      onError: function (line) {
        if (line === '') {
          hideError();
        } else {
          showError(line);
        }
      },
      onGone: reload
    });
    els.rating.hidden = false;
    els.rating.appendChild(control.el);
  }

  // syncRating follows the server's may_rate. When it is true the control is
  // drawn (or redrawn with the server's your_rating); when it turns false the
  // box is taken out with no message, because nothing explains why a reader may
  // not rate (UI-SPEC 3a). The browser decides nothing: it reads one boolean.
  function syncRating(may, yours) {
    if (typeof may !== 'boolean') {
      return;
    }
    if (!may) {
      if (control !== null) {
        control.destroy();
        control = null;
      }
      if (els.rating) {
        els.rating.hidden = true;
      }
      return;
    }
    if (control !== null) {
      control.set(yours);
      return;
    }
    if (!els.rating && els.header && els.error) {
      var host = document.createElement('div');
      host.id = 'person-rating';
      host.className = 'person-rating';
      els.header.insertBefore(host, els.error);
      els.rating = host;
    }
    mountRating(yours);
  }

  function draw(profile) {
    if (!profile || typeof profile !== 'object') {
      return;
    }
    if (profile.blocked_by_you === true) {
      showBlocked();
      return;
    }
    syncRating(profile.may_rate, profile.your_rating);
    drawCount(els.followersCount, els.followersWord, els.followersButton,
      profile.followers_count, profile.followers_word, profile.followers_text);
    drawCount(els.followingCount, els.followingWord, els.followingButton,
      profile.following_count, profile.following_word, profile.following_text);
    if (els.follow) {
      els.follow.hidden = profile.can_follow !== true;
    }
    if (els.unfollow) {
      els.unfollow.hidden = profile.following !== true;
    }
    if (els.removeFollower) {
      els.removeFollower.hidden = profile.follows_you !== true;
    }
    if (typeof profile.muted === 'boolean') {
      drawMute(profile.muted);
    }
  }

  // drawMute sets the first menu item to Mute or Unmute from the server's
  // muted flag. The flag is the viewer's own state only (plan 09-19), so the
  // browser reads one boolean and decides nothing else.
  function drawMute(on) {
    if (!els.mute) {
      return;
    }
    els.mute.textContent = on ? 'Unmute' : 'Mute';
    els.mute.setAttribute('data-muted', on ? 'true' : 'false');
  }

  // refresh asks the server for the profile again. A failed read reloads the
  // page, so the server's own drawing is what the person ends up with.
  function refresh() {
    return request(actionUrl(''), {
      method: 'GET',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' }
    }).then(function (answer) {
      if (!answer.ok) {
        reload();
        return false;
      }
      draw(answer.body);
      return true;
    }, function () {
      reload();
      return false;
    });
  }

  // showBlocked draws the You blocked face in place of the header and the
  // reports, and puts focus on its heading.
  function showBlocked() {
    blockedNow = true;
    if (els.header) {
      els.header.hidden = true;
    }
    if (els.reports) {
      els.reports.hidden = true;
    }
    if (els.blocked) {
      els.blocked.hidden = false;
      if (els.blockedTitle) {
        els.blockedTitle.textContent = 'You blocked ' + username;
        els.blockedTitle.focus();
      }
    }
  }

  // ---- Follow and Following ---------------------------------------------------

  function setBusy(button, on) {
    if (!button) {
      return;
    }
    button.disabled = on;
    if (on) {
      button.setAttribute('aria-busy', 'true');
    } else if (button.removeAttribute) {
      button.removeAttribute('aria-busy');
    }
  }

  // toggle sends follow or unfollow, then redraws from the server's profile. The
  // label never changes while the call is out. Focus ends on the button now in
  // its place.
  function toggle(action, button, message) {
    if (busy || !button) {
      return Promise.resolve(false);
    }
    busy = true;
    hideError();
    setBusy(button, true);
    function done() {
      busy = false;
      setBusy(button, false);
    }
    function fail(text) {
      done();
      if (text !== null) {
        showError(text);
      }
      return false;
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return Promise.resolve(fail(MESSAGE_OFFLINE));
    }
    return post(action).then(function (answer) {
      if (!answer.ok) {
        var line = refusal(answer, action);
        if (action === 'follow' && answer.status === 409 && els.follow) {
          els.follow.hidden = true;
        }
        return fail(line);
      }
      return refresh().then(function (drawn) {
        done();
        if (!drawn) {
          return false;
        }
        say(message);
        var next = action === 'follow' ? els.unfollow : els.follow;
        if (next && next.hidden === false) {
          next.focus();
        } else if (els.more) {
          els.more.focus();
        }
        return true;
      });
    }, function () {
      return fail(offlineOr(action));
    });
  }

  function follow() {
    return toggle('follow', els.follow, 'You followed ' + username + '.');
  }

  function unfollow() {
    return toggle('unfollow', els.unfollow, 'You unfollowed ' + username + '.');
  }

  // ---- Mute and Unmute (plan 09-20, UI-SPEC 1e) -------------------------------

  // isMuted reads the state the server drew: the label and data-muted are
  // written together and only ever redrawn from the refetched profile.
  function isMuted() {
    return !!els.mute && els.mute.getAttribute('data-muted') === 'true';
  }

  // toggleMute sends mute or unmute at once, with no confirmation, then redraws
  // from the server's profile and says the status line. The person is never told:
  // this sends nothing to them and shows nothing of the muted person's own page.
  // Focus ends on More, where the menu that held the item was opened from.
  function toggleMute() {
    if (busy || !els.mute) {
      return Promise.resolve(false);
    }
    var action = isMuted() ? 'unmute' : 'mute';
    var message = action === 'mute'
      ? username + ' is muted. Their reports still show.'
      : username + ' is unmuted.';
    busy = true;
    hideError();
    setBusy(els.mute, true);
    function done() {
      busy = false;
      setBusy(els.mute, false);
    }
    function fail(text) {
      done();
      if (text !== null) {
        showError(text);
      }
      if (els.more) {
        els.more.focus();
      }
      return false;
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return Promise.resolve(fail(MESSAGE_OFFLINE));
    }
    return post(action).then(function (answer) {
      if (!answer.ok) {
        return fail(refusal(answer, action));
      }
      // The server kept it. Draw the expected label now; the refetch below
      // overrides it with the server's own flag when it sends one.
      drawMute(action === 'mute');
      return refresh().then(function (drawn) {
        done();
        if (!drawn) {
          return false;
        }
        say(message);
        if (els.more) {
          els.more.focus();
        }
        return true;
      });
    }, function () {
      return fail(offlineOr(action));
    });
  }

  // ---- The More menu ----------------------------------------------------------

  function menuOpen() {
    return !!els.menu && els.menu.hidden === false;
  }

  function menuItems() {
    var all = els.menu ? els.menu.querySelectorAll('[role="menuitem"]') : [];
    var out = [];
    for (var i = 0; i < all.length; i++) {
      if (all[i].hidden !== true) {
        out.push(all[i]);
      }
    }
    return out;
  }

  var outsideBound = false;

  function onOutsideClick(evt) {
    var target = evt && evt.target;
    if (els.more && els.more.contains(target)) {
      return;
    }
    if (els.menu && els.menu.contains(target)) {
      return;
    }
    closeMenu(false);
  }

  function openMenu() {
    if (!els.menu || !els.more || menuOpen()) {
      return;
    }
    els.menu.hidden = false;
    els.more.setAttribute('aria-expanded', 'true');
    if (!outsideBound) {
      document.addEventListener('click', onOutsideClick, true);
      outsideBound = true;
    }
    var items = menuItems();
    if (items.length > 0) {
      items[0].focus();
    }
  }

  function closeMenu(restoreFocus) {
    if (!els.menu || !els.more || !menuOpen()) {
      return;
    }
    els.menu.hidden = true;
    els.more.setAttribute('aria-expanded', 'false');
    if (outsideBound) {
      document.removeEventListener('click', onOutsideClick, true);
      outsideBound = false;
    }
    if (restoreFocus) {
      els.more.focus();
    }
  }

  function onMoreClick() {
    if (menuOpen()) {
      closeMenu(true);
    } else {
      openMenu();
    }
  }

  function onMoreKeydown(evt) {
    if (evt.key === 'ArrowDown' && !menuOpen()) {
      evt.preventDefault();
      openMenu();
    }
  }

  function onMenuKeydown(evt) {
    var items = menuItems();
    var at = items.indexOf(document.activeElement);
    switch (evt.key) {
      case 'ArrowDown':
        evt.preventDefault();
        if (items.length > 0) {
          items[(at + 1) % items.length].focus();
        }
        break;
      case 'ArrowUp':
        evt.preventDefault();
        if (items.length > 0) {
          items[(at <= 0 ? items.length : at) - 1].focus();
        }
        break;
      case 'Home':
        evt.preventDefault();
        if (items.length > 0) {
          items[0].focus();
        }
        break;
      case 'End':
        evt.preventDefault();
        if (items.length > 0) {
          items[items.length - 1].focus();
        }
        break;
      case 'Escape':
        evt.preventDefault();
        closeMenu(true);
        break;
      case 'Tab':
        closeMenu(false);
        break;
      case 'Enter':
      case ' ':
        evt.preventDefault();
        if (at >= 0) {
          choose(items[at]);
        }
        break;
      default:
        break;
    }
  }

  // choose runs the item the person picked. Remove follower and Block ask first;
  // Mute and Unmute take effect at once.
  function choose(item) {
    closeMenu(false);
    if (item === els.mute) {
      toggleMute();
    } else if (item === els.removeFollower) {
      ask('remove-follower');
    } else if (item === els.block) {
      ask('block');
    }
  }

  // ---- The confirm box --------------------------------------------------------

  function canAskInBox() {
    return !!els.confirm && typeof els.confirm.showModal === 'function';
  }

  function ask(kind) {
    var box = BOX[kind];
    if (!box || busy) {
      return;
    }
    asking = kind;
    if (!canAskInBox()) {
      if (window.confirm(box.title(username))) {
        runConfirmed();
      }
      asking = '';
      return;
    }
    els.confirmTitle.textContent = box.title(username);
    var text = els.confirmText;
    while (text.firstChild) {
      text.removeChild(text.firstChild);
    }
    for (var i = 0; i < box.texts.length; i++) {
      var p = document.createElement('p');
      p.textContent = box.texts[i];
      text.appendChild(p);
    }
    els.confirmGo.textContent = box.go;
    if (els.confirmError) {
      els.confirmError.textContent = '';
      els.confirmError.hidden = true;
    }
    els.confirm.showModal();
    // The native box opens with focus on the question, so no button looks
    // chosen and nothing is sent until the confirm button is pressed.
    els.confirmTitle.focus();
  }

  function closeConfirm() {
    if (els.confirm && els.confirm.open) {
      els.confirm.close();
    }
  }

  // runConfirmed sends the asked action. On success the box closes and the page
  // redraws; on a refusal the box stays open with the sentence inside it.
  function runConfirmed() {
    var kind = asking;
    if (busy || !BOX[kind]) {
      return Promise.resolve(false);
    }
    busy = true;
    if (els.confirmGo) {
      els.confirmGo.disabled = true;
    }
    if (els.confirmCancel) {
      els.confirmCancel.disabled = true;
    }
    function settle() {
      busy = false;
      if (els.confirmGo) {
        els.confirmGo.disabled = false;
      }
      if (els.confirmCancel) {
        els.confirmCancel.disabled = false;
      }
    }
    function fail(text) {
      settle();
      if (text === null) {
        return false;
      }
      if (canAskInBox() && els.confirm.open) {
        setText(els.confirmError, text);
        els.confirmError.hidden = false;
        els.confirmTitle.focus();
      } else {
        showError(text);
      }
      return false;
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return Promise.resolve(fail(MESSAGE_OFFLINE));
    }
    return post(kind).then(function (answer) {
      if (!answer.ok) {
        return fail(refusal(answer, kind));
      }
      settle();
      asking = '';
      if (kind === 'block') {
        // The box closes after the page knows it is blocked, so its close
        // handler does not send focus back to a More button that is hidden.
        blockedNow = true;
        closeConfirm();
        showBlocked();
        return true;
      }
      closeConfirm();
      if (els.removeFollower) {
        els.removeFollower.hidden = true;
      }
      say(username + ' no longer follows you.');
      return refresh().then(function () { return true; });
    }, function () {
      return fail(offlineOr(kind));
    });
  }

  // ---- Unblock ----------------------------------------------------------------

  // unblock undoes the block with no confirmation and reloads, so the server
  // draws a normal profile, or the not found page if they have gone private.
  function unblock() {
    if (busy) {
      return Promise.resolve(false);
    }
    busy = true;
    hideError();
    setBusy(els.unblock, true);
    function fail(text) {
      busy = false;
      setBusy(els.unblock, false);
      if (text !== null) {
        showError(text);
      }
      return false;
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return Promise.resolve(fail(MESSAGE_OFFLINE));
    }
    return post('unblock').then(function (answer) {
      if (!answer.ok) {
        return fail(refusal(answer, 'unblock'));
      }
      say(username + ' is unblocked.');
      reload();
      return true;
    }, function () {
      return fail(offlineOr('unblock'));
    });
  }

  // ---- The Followers and Following windows (plan 09-11) -----------------------

  // openList opens the shared list window for this profile. onChange redraws the
  // header counts from the server after the list changed something (Remove all
  // followers), so the button and the window never disagree.
  function openList(kind, opener) {
    var lists = typeof window !== 'undefined' ? window.PinalertPeopleList : null;
    if (!lists || typeof lists.open !== 'function') {
      return;
    }
    lists.open({
      kind: kind,
      username: username,
      own: !!page && page.getAttribute('data-self') === 'true',
      opener: opener,
      onChange: function () { refresh(); }
    });
  }

  // ---- Wiring -----------------------------------------------------------------

  function on(node, type, fn) {
    if (node) {
      node.addEventListener(type, fn);
    }
  }

  function init() {
    if (inited) {
      return;
    }
    page = byId('person-page');
    if (!page) {
      return;
    }
    inited = true;
    username = page.getAttribute('data-username') || '';
    var ids = {
      live: 'notice-live', header: 'person-header', reports: 'person-reports-section',
      error: 'person-action-error', follow: 'person-follow', unfollow: 'person-unfollow',
      more: 'person-more', menu: 'person-more-menu', mute: 'person-mute-item', removeFollower: 'person-remove-follower',
      block: 'person-block', blocked: 'person-blocked', blockedTitle: 'person-blocked-title',
      blockedError: 'person-blocked-error', unblock: 'person-unblock',
      followersButton: 'person-followers-button', followingButton: 'person-following-button',
      followersCount: 'person-followers-count', followersWord: 'person-followers-word',
      followingCount: 'person-following-count', followingWord: 'person-following-word',
      confirm: 'person-confirm', confirmTitle: 'person-confirm-title',
      confirmText: 'person-confirm-text', confirmError: 'person-confirm-error',
      confirmCancel: 'person-confirm-cancel', confirmGo: 'person-confirm-go',
      rating: 'person-rating'
    };
    for (var key in ids) {
      if (Object.prototype.hasOwnProperty.call(ids, key)) {
        els[key] = byId(ids[key]);
      }
    }
    if (username === '') {
      return;
    }

    // The server drew the rating box only because it said this reader may rate,
    // and wrote their rating into data-your-rating.
    if (els.rating) {
      mountRating(parseInt(els.rating.getAttribute('data-your-rating'), 10) || 0);
    }

    on(els.follow, 'click', function () { follow(); });
    on(els.unfollow, 'click', function () { unfollow(); });
    on(els.more, 'click', onMoreClick);
    on(els.more, 'keydown', onMoreKeydown);
    on(els.menu, 'keydown', onMenuKeydown);
    on(els.mute, 'click', function () { choose(els.mute); });
    on(els.removeFollower, 'click', function () { choose(els.removeFollower); });
    on(els.block, 'click', function () { choose(els.block); });
    on(els.unblock, 'click', function () { unblock(); });
    on(els.followersButton, 'click', function () { openList('followers', els.followersButton); });
    on(els.followingButton, 'click', function () { openList('following', els.followingButton); });
    on(els.confirmGo, 'click', function () { runConfirmed(); });
    on(els.confirmCancel, 'click', function () {
      if (!busy) {
        closeConfirm();
      }
    });
    // Escape is the browser's cancel event, the same as Cancel except while an
    // action is being sent. There is no click handler on the box itself.
    on(els.confirm, 'cancel', function (evt) {
      if (busy) {
        evt.preventDefault();
      }
    });
    on(els.confirm, 'close', function () {
      asking = '';
      if (!blockedNow && els.more) {
        els.more.focus();
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
    init: init,
    follow: follow,
    unfollow: unfollow,
    toggleMute: toggleMute,
    openMenu: openMenu,
    closeMenu: closeMenu,
    ask: ask,
    unblock: unblock,
    refresh: refresh,
    openList: openList
  };
}());
