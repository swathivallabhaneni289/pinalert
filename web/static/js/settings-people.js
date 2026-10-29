// settings-people.js: the people rows of the Settings page, People and reports
// (Phase 9 plan 12, D-04, D-12, UI-SPEC Surface 7).
//
//   - Let others follow me is a switch saved at once with
//     PATCH /api/me/visibility. On 200 it says "Your profile is on. People can
//     follow you while you hold the Reliable reporter tag." or "Your profile is
//     off." politely. A refusal (422, the Name or username sentence, or a 429
//     with the server's own sentence) puts the old value back and shows the
//     server's sentence under the box. Any other failure puts the old value back
//     and shows "Couldn't save that. Try again."; offline nothing is sent and
//     the built offline line shows.
//   - Turning it off while the Followers count is above 0 asks first, in a small
//     native box like Log out (UI-SPEC 7b, P9-C22): Cancel leaves it on, Turn
//     off saves. With nobody following it turns off at once. The box closes only
//     through its two buttons and Escape (which is Cancel), never by a tap on
//     the dim page.
//   - The Following, Followers and Blocked people rows open the shared list
//     window (people-list.js) with own true. After the list changed something
//     the row's count is read again from the server.
//
// A separate file from settings.js and settings-alerts.js on purpose, so both
// stay as built. It is self contained: it needs only the ids settings.html.tmpl
// renders and window.PinalertPeopleList, and it leaves quietly on a page that
// does not have them.
//
// SECURITY (T-09-46): textContent only, no markup sink of any kind. The only
// addresses it sends to are /api/me/visibility, /api/me/blocked and
// /api/people/ plus the encoded username the server wrote into the page.
window.PinalertSettingsPeople = (function () {
  'use strict';

  var sw = document.getElementById('settings-profile-switch');
  var box = document.getElementById('settings-profile-off-confirm');
  var errorLine = document.getElementById('settings-people-error');
  var wrap = document.getElementById('settings-people-wrap');
  var followingRow = document.getElementById('settings-following-row');
  var followersRow = document.getElementById('settings-followers-row');
  var blockedRow = document.getElementById('settings-blocked-row');
  var followingValue = document.getElementById('settings-following-value');
  var followersValue = document.getElementById('settings-followers-value');
  var blockedValue = document.getElementById('settings-blocked-value');
  var offGo = document.getElementById('settings-profile-off-go');
  var offCancel = document.getElementById('settings-profile-off-cancel');
  var offTitle = document.getElementById('settings-profile-off-title');
  if (!sw || !box || !errorLine || !wrap || !followingRow || !followersRow || !blockedRow ||
      !followingValue || !followersValue || !blockedValue || !offGo || !offCancel) {
    return undefined;
  }

  var VISIBILITY_URL = '/api/me/visibility';
  var BLOCKED_URL = '/api/me/blocked';
  var PEOPLE_URL = '/api/people/';
  var REQUEST_TIMEOUT_MS = 10000;

  var MESSAGE_ON = 'Your profile is on. People can follow you while you hold the Reliable reporter tag.';
  var MESSAGE_OFF = 'Your profile is off.';
  var MESSAGE_FAILED = 'Couldn\'t save that. Try again.';
  var MESSAGE_OFFLINE = 'You\'re offline, so this wasn\'t saved. Try again when you have a signal.';
  var OFF_QUESTION = 'Turn off your public profile?';

  var live = document.getElementById('notice-live');
  var username = wrap.getAttribute('data-username') || '';
  var canAskInBox = typeof box.showModal === 'function';

  // saved is the last value the server confirmed, which is what a failed save
  // puts back.
  var saved = sw.checked === true;
  var saving = false;

  function setText(node, value) {
    node.textContent = value === null || value === undefined ? '' : String(value);
  }

  function showError(message) {
    setText(errorLine, message);
    errorLine.hidden = false;
  }

  function hideError() {
    setText(errorLine, '');
    errorLine.hidden = true;
  }

  function say(message) {
    if (live) {
      setText(live, message);
    }
  }

  // request sends one call with a time limit that also covers reading the body.
  // It resolves to {status, ok, body} and rejects on a network failure or the
  // limit. A body that is not JSON becomes an empty object.
  function request(url, init) {
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    if (controller !== null) {
      init.signal = controller.signal;
    }
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = setTimeout(function () {
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
        clearTimeout(timer);
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

  // serverMessage is the sentence the server wrote for a refusal, when it sent
  // one in its error envelope.
  function serverMessage(answer) {
    var error = answer && answer.body && answer.body.error;
    if (error && typeof error.message === 'string' && error.message !== '') {
      return error.message;
    }
    return '';
  }

  function offline() {
    return typeof navigator !== 'undefined' && navigator.onLine === false;
  }

  // ---- The switch -----------------------------------------------------------

  // save sends the switch value and resolves to true when the server kept it.
  // The switch is disabled until the answer. Every failure puts the last
  // confirmed value back.
  function save(value) {
    if (saving) {
      return Promise.resolve(false);
    }
    hideError();
    function fail(message) {
      saving = false;
      sw.disabled = false;
      sw.checked = saved;
      showError(message);
      return false;
    }
    if (offline()) {
      sw.checked = saved;
      showError(MESSAGE_OFFLINE);
      return Promise.resolve(false);
    }
    saving = true;
    sw.disabled = true;
    return request(VISIBILITY_URL, {
      method: 'PATCH',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ profile_public: value === true })
    }).then(function (answer) {
      if (!answer.ok) {
        if ((answer.status === 422 || answer.status === 429) && serverMessage(answer) !== '') {
          return fail(serverMessage(answer));
        }
        return fail(MESSAGE_FAILED);
      }
      saving = false;
      sw.disabled = false;
      saved = !!(answer.body && answer.body.profile_public === true);
      sw.checked = saved;
      if (saved) {
        say(MESSAGE_ON);
      } else {
        // Turning it off ended every follow on the server (D-12).
        setText(followersValue, '0');
        say(MESSAGE_OFF);
      }
      return true;
    }, function () {
      return fail(MESSAGE_FAILED);
    });
  }

  function followerCount() {
    var n = parseInt(followersValue.textContent, 10);
    return isNaN(n) ? 0 : n;
  }

  // closeBox closes the confirm box when it is open.
  function closeBox() {
    if (canAskInBox && box.open) {
      box.close();
    }
  }

  // ask opens the confirm box, or, in a browser without native dialogs, the
  // browser's own confirm with the same question.
  function ask() {
    if (canAskInBox) {
      if (!box.open) {
        box.showModal();
      }
      if (offTitle) {
        offTitle.focus();
      }
      return;
    }
    if (typeof confirm === 'function' && confirm(OFF_QUESTION)) {
      save(false);
    }
  }

  sw.addEventListener('change', function () {
    if (saving) {
      sw.checked = saved;
      return;
    }
    if (sw.checked !== true && followerCount() > 0) {
      // Keep it on while the box asks; Cancel leaves it as it is.
      sw.checked = true;
      ask();
      return;
    }
    save(sw.checked === true);
  });

  offCancel.addEventListener('click', function () {
    sw.checked = saved;
    closeBox();
  });

  offGo.addEventListener('click', function () {
    closeBox();
    // The switch shows off while the save is out; a failure puts it back on.
    sw.checked = false;
    save(false);
  });

  // Escape is Cancel. The box has no handler for a click on itself, so a tap on
  // the dim page does nothing.
  box.addEventListener('cancel', function (e) {
    if (e && typeof e.preventDefault === 'function') {
      e.preventDefault();
    }
    sw.checked = saved;
    closeBox();
  });

  box.addEventListener('close', function () {
    sw.focus();
  });

  // ---- The list rows ---------------------------------------------------------

  // refreshCounts reads the counts again after a list changed something. A
  // failed read leaves the shown count as it was; the next page load is right.
  function refreshCounts(kind) {
    function get(url) {
      return request(url, { method: 'GET', credentials: 'same-origin', headers: { Accept: 'application/json' } });
    }
    if (kind === 'blocked') {
      return get(BLOCKED_URL).then(function (answer) {
        if (answer.ok && answer.body && Array.isArray(answer.body.entries)) {
          setText(blockedValue, answer.body.entries.length);
        }
      }, function () {});
    }
    if (username === '') {
      return Promise.resolve();
    }
    return get(PEOPLE_URL + encodeURIComponent(username)).then(function (answer) {
      if (!answer.ok || !answer.body) {
        return;
      }
      if (typeof answer.body.followers_count === 'number') {
        setText(followersValue, answer.body.followers_count);
      }
      if (typeof answer.body.following_count === 'number') {
        setText(followingValue, answer.body.following_count);
      }
    }, function () {});
  }

  // openList opens the shared list window for one row. Followers and Following
  // are read through the person's own username; a person with no username has
  // no lists yet, so those two rows do nothing (the first time flow gives every
  // account a username before Settings can be reached).
  function openList(kind, opener) {
    var lists = typeof window !== 'undefined' ? window.PinalertPeopleList : null;
    if (!lists || typeof lists.open !== 'function') {
      return;
    }
    if (kind !== 'blocked' && username === '') {
      return;
    }
    lists.open({
      kind: kind,
      username: kind === 'blocked' ? '' : username,
      own: true,
      opener: opener,
      onChange: function () { refreshCounts(kind); }
    });
  }

  followingRow.addEventListener('click', function () { openList('following', followingRow); });
  followersRow.addEventListener('click', function () { openList('followers', followersRow); });
  blockedRow.addEventListener('click', function () { openList('blocked', blockedRow); });

  return {
    save: save,
    openList: openList,
    refreshCounts: refreshCounts
  };
}());
