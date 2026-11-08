// settings.js: the Username, Name and Home area edit windows of the Settings
// page (Phase 3.4, plan 06, UI-SPEC Surface 6, D-24b, D-02; the Username window
// and the rule that a name or a home area is never emptied or removed are
// Phase 7.1, plan 15).
//
// Self contained, like account-menu.js: it needs no other script of the app,
// only the ids settings.html.tmpl renders, and it leaves quietly on a page that
// has no edit windows. The page behind a window is made inert by a small local
// lock rather than by app.js, which carries the map page's polling and report
// state and has nothing to do here.
//
// What it never does:
//   - It never asks the browser where the device is (D-24b). The home area is a
//     place the person picks from the place search, and only that place's short
//     name and point go to the server, which keeps the name and a 5 km cell.
//   - It never parses markup. Every string, the person's own name and place
//     names from the geocoder included, reaches the page through textContent
//     (T-34-26).
//   - It never closes a window except through the X or Escape (D-02): there is
//     no backdrop handler, no Cancel and no Done. One exception (Quick 261108-fku,
//     owner decision 2026-11-08): inside the 7 day rename wait the Username window
//     opens read only with the server's sentence and one OK button, and OK, Enter
//     and a submit close it the way the X does, since nothing can be saved there.
(function () {
  'use strict';

  var nameWin = document.getElementById('settings-name-window');
  var homeWin = document.getElementById('settings-home-window');
  if (!nameWin || !homeWin) {
    return;
  }

  var PROFILE_URL = '/api/me/profile';
  // kind=area asks for areas only (neighbourhoods, localities, towns, cities,
  // districts), never a street or a building: the stored home area is a cell
  // about 5 km wide.
  var PLACE_URL = '/api/geocode/place?kind=area&q=';
  var SAVE_TIMEOUT_MS = 10000;
  var SEARCH_TIMEOUT_MS = 10000;
  var MIN_QUERY_RUNES = 3;
  var MAX_RESULTS = 5;
  // Suggestions while typing. The search runs once the person stops typing for
  // TYPING_PAUSE_MS, and typed searches go out at least TYPING_GAP_MS apart, so
  // typing alone spends the per account search budget (a burst of 8, then 1 a
  // second) no faster than it refills and can never reach Too many searches.
  // A typed search the server refuses or finds busy (429 or 503) is tried again
  // quietly, at most QUIET_RETRIES times, RETRY_WAIT_MS apart.
  var TYPING_PAUSE_MS = 400;
  var TYPING_GAP_MS = 1000;
  var QUIET_RETRIES = 3;
  var RETRY_WAIT_MS = 1000;
  var NOT_SET = 'Not set';

  // The search wording is the same as the reader lane search of the map page
  // (map-tools.js), so one sentence means one thing across the app.
  var MESSAGE_SEARCHING = 'Searching…';
  var MESSAGE_TOO_SHORT = 'Type at least 3 letters.';
  // Shown when no area matches closely and no typed row could be placed (plan
  // 34), with the rows still listed when there are any.
  var MESSAGE_NOT_FOUND = 'Not found. Add your town, for example: Gayathri Nagar, Vijayawada';
  var MESSAGE_UNAVAILABLE = 'Search unavailable. Try again in a moment.';
  var MESSAGE_RATE_LIMITED = 'Too many searches. Wait a few seconds and try again.';
  var MESSAGE_SAVE_FAILED = 'Could not save. Try again in a moment.';
  // Phase 7.1 plan 15 (D-15, UI-SPEC Surface 8): the name and the home area can
  // be changed but not emptied. The server refuses both too (plan 06).
  var MESSAGE_NAME_EMPTY = "Your name can't be empty.";
  var MESSAGE_CHOOSE_HOME = 'Choose a home area.';
  var MESSAGE_SESSION_ENDED = 'Your session has ended. Reload the page and log in again.';

  var FOCUSABLE_SELECTOR = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

  function el(id) {
    return document.getElementById(id);
  }

  // waitSentence is the wait sentence for a number of days, the server's words.
  function waitSentence(days) {
    return WAIT_PREFIX + days + (days === 1 ? WAIT_ONE_DAY : WAIT_MANY_DAYS);
  }

  // waitDaysLeft is the whole days left until the deadline, rounded up the way
  // usernameWaitDays rounds them and at least 1 while any time is left; 0 means
  // the wait is over (or there is none).
  function waitDaysLeft() {
    var left = waitUntil - Date.now();
    if (waitUntil <= 0 || left <= 0) {
      return 0;
    }
    return Math.max(1, Math.floor((left + DAY_MS - 1) / DAY_MS));
  }

  // daysInSentence reads the day count out of a wait sentence of the server's
  // words; any other text answers the whole gap.
  function daysInSentence(sentence) {
    if (sentence.indexOf(WAIT_PREFIX) !== 0) {
      return WAIT_GAP_DAYS;
    }
    var days = parseInt(sentence.slice(WAIT_PREFIX.length), 10);
    return days >= 1 ? days : WAIT_GAP_DAYS;
  }

  // Phase 7.1 plan 15 (D-07, UI-SPEC Surface 8): the Username window. Every
  // piece is optional here, so a page drawn without it (and the older test
  // pages) keeps the Name and Home area windows working. The live check is the
  // one step 4 uses (username-check.js, settings mode); it is attached the
  // first time the window opens and judges the stored name on each later opening.
  var usernameWin = el('settings-username-window');
  var usernameRow = el('settings-username-row');
  var usernameValue = el('settings-username-value');
  var usernameClose = el('settings-username-close');
  var usernameForm = el('settings-username-form');
  var usernameInput = el('settings-username-input');
  var usernameStatus = el('settings-username-status');
  var usernameSave = el('settings-username-save');
  var usernameError = el('settings-username-error');
  var hasUsername = !!(usernameWin && usernameRow && usernameValue && usernameClose && usernameForm &&
    usernameInput && usernameStatus && usernameSave && usernameError);
  var usernameCheck = null;
  var savingUsername = false;
  var usernameFilled = '';
  // Quick 261108-fku (owner decision 2026-11-08, review fix WR-02): the rename wait.
  // The OK button is optional, so an older hand built page keeps working. The lock
  // follows a deadline, not a sentence drawn once: data-wait-until is the moment the
  // wait ends (an RFC 3339 UTC time the server writes only inside the gap). On each
  // opening the days left are worked out from it, rounded up the way the server
  // rounds them (usernameWaitDays): none left opens the window as usual, otherwise
  // it opens locked with the sentence for that number of days. A change saved in
  // this page sets the deadline to now plus the whole gap. The server still decides
  // on save. The clock is the device's own.
  var usernameOk = el('settings-username-ok');
  var DAY_MS = 24 * 60 * 60 * 1000;
  // WAIT_GAP_DAYS is service.UsernameRenameGap in days, and the sentence parts are
  // service.UsernameWaitMessage word for word. web/settings_username_wait_contract_test.go
  // pins them.
  var WAIT_GAP_DAYS = 7;
  var WAIT_PREFIX = 'You can change your username again in ';
  var WAIT_ONE_DAY = ' day.';
  var WAIT_MANY_DAYS = ' days.';
  var waitUntil = 0;
  if (hasUsername) {
    var deadline = Date.parse(usernameWin.getAttribute('data-wait-until') || '');
    waitUntil = deadline > 0 ? deadline : 0;
  }
  // usernameLocked is true while the window is drawn locked (read only, the wait
  // sentence, OK in place of Save username).
  var usernameLocked = false;
  var lockedSentence = '';

  var nameRow = el('settings-name-row');
  var nameValue = el('settings-name-value');
  var nameClose = el('settings-name-close');
  var nameForm = el('settings-name-form');
  var nameInput = el('settings-name-input');
  var nameSave = el('settings-name-save');
  var nameError = el('settings-name-error');

  var homeRow = el('settings-home-row');
  var homeValue = el('settings-home-value');
  var homeClose = el('settings-home-close');
  var homeForm = el('settings-home-form');
  var homeInput = el('settings-home-input');
  var homeStatus = el('settings-home-status');
  var homeResults = el('settings-home-results');
  var homeSave = el('settings-home-save');
  var homeError = el('settings-home-error');

  var windows = {
    name: { win: nameWin, row: nameRow, close: nameClose },
    home: { win: homeWin, row: homeRow, close: homeClose }
  };
  if (hasUsername) {
    windows.username = { win: usernameWin, row: usernameRow, close: usernameClose };
  }

  var current = null;
  // session numbers the openings of a window. A save remembers the number it was
  // sent under, and an answer that comes back under another number belongs to a
  // window that was closed since: it may update the row, but it never closes,
  // clears or writes into the window open now (code review part 2, WR-03).
  var session = 0;
  // One flag for each window, so the answer of one never clears the other's. A
  // flag is cleared only by the answer of its own save, never by closing or
  // reopening the window: a save still out is still out.
  var savingName = false;
  var savingHome = false;
  // The value each field was filled with when its window was last opened. When
  // a save from an earlier opening answers while the window is open again, the
  // field is refilled with the saved value only if it still holds this one, so
  // a field the person already changed is never overwritten (skeptic S-3).
  var nameFilled = '';
  var homeFilled = '';

  // The picked place, or null. Only a result of the place search can be picked,
  // so a home area is never anything the person typed or the device reported.
  var picked = null;
  var searchSeq = 0;
  var searchRequest = null;
  var searchText = '';
  // Whether the search in flight was started by typing (quiet: no Searching
  // line, no failure line) and not by Enter.
  var searchQuiet = false;
  // The pending typing pause or quiet retry, or null.
  var typingTimer = null;
  // Set while the gap after the last search runs; a typed search that comes due
  // meanwhile waits in waitingText.
  var coolTimer = null;
  var waitingText = null;
  // The text the last request was sent for, so the same text is never sent
  // twice in a row by typing.
  var lastSentText = '';
  // How many quiet retries the text typed last has used.
  var quietFailures = 0;

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

  function runeCount(text) {
    return Array.from(text).length;
  }

  // isShown says whether node and every ancestor up to the window are visible,
  // so the focus trap skips a hidden results list.
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

  // request sends one JSON request with a time limit that also covers reading
  // the body, so a request WebKit leaves unsettled still ends. It resolves to
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

  function patchProfile(body) {
    return request(PROFILE_URL, {
      method: 'PATCH',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    }, SAVE_TIMEOUT_MS).promise;
  }

  // serverSentence is the server's own sentence for a refused request, when it
  // sent one of a sensible length.
  function serverSentence(body) {
    var sent = body && body.error && body.error.message;
    if (typeof sent === 'string' && sent !== '' && sent.length <= 200) {
      return sent;
    }
    return '';
  }

  function saveMessage(answer) {
    if (answer.status === 400 && serverSentence(answer.body) !== '') {
      return serverSentence(answer.body);
    }
    if (answer.status === 401) {
      return MESSAGE_SESSION_ENDED;
    }
    return MESSAGE_SAVE_FAILED;
  }

  // setRow shows a stored value on its row and keeps the data-value attribute
  // the window prefills from. An empty value reads Not set.
  function setRow(row, valueNode, value) {
    var text = typeof value === 'string' ? value : '';
    setText(valueNode, text === '' ? NOT_SET : text);
    row.setAttribute('data-value', text);
  }

  function storedValue(row) {
    return row.getAttribute('data-value') || '';
  }

  // ---- Opening and closing --------------------------------------------------

  function clearPick() {
    picked = null;
    // Save home area is always pressable; only a save still out switches it off
    // (Phase 7.1 plan 15). With nothing picked, a press says what to do.
    homeSave.disabled = savingHome;
    var options = homeResults.querySelectorAll('button');
    for (var i = 0; i < options.length; i++) {
      options[i].setAttribute('aria-selected', 'false');
    }
  }

  function cancelSearch() {
    searchSeq += 1;
    if (searchRequest !== null) {
      searchRequest.abort();
      searchRequest = null;
    }
    searchText = '';
    searchQuiet = false;
  }

  // cancelTyping drops a pending typing pause, quiet retry or search waiting
  // for the gap. The gap itself keeps running.
  function cancelTyping() {
    if (typingTimer !== null) {
      window.clearTimeout(typingTimer);
      typingTimer = null;
    }
    waitingText = null;
  }

  // resetTyping forgets everything typing started, for a window opened or
  // closed.
  function resetTyping() {
    cancelTyping();
    if (coolTimer !== null) {
      window.clearTimeout(coolTimer);
      coolTimer = null;
    }
    lastSentText = '';
    quietFailures = 0;
  }

  function openWindow(which) {
    var target = windows[which];
    if (!target) {
      return;
    }
    if (current !== null && current !== which) {
      closeWindow(current, false);
    }
    current = which;
    session += 1;
    if (which === 'username') {
      hideLine(usernameError);
      var daysLeft = waitDaysLeft();
      if (daysLeft > 0) {
        // Inside the rename wait: read only, the sentence for the days left, OK.
        lockUsername(waitSentence(daysLeft));
      } else {
        if (usernameLocked) {
          // The wait ended since the window last opened: the usual window again.
          unlockUsername();
        }
        usernameFilled = storedValue(usernameRow);
        usernameInput.value = usernameFilled;
        if (usernameCheck === null) {
          attachUsernameCheck();
        } else {
          usernameCheck.refresh();
        }
        updateUsernameSave();
      }
    } else if (which === 'name') {
      hideLine(nameError);
      nameFilled = storedValue(nameRow);
      nameInput.value = nameFilled;
      nameSave.disabled = savingName;
    } else {
      hideLine(homeError);
      hideLine(homeStatus);
      homeResults.textContent = '';
      homeResults.hidden = true;
      cancelSearch();
      resetTyping();
      picked = null;
      homeSave.disabled = savingHome;
      homeFilled = storedValue(homeRow);
      homeInput.value = homeFilled;
    }
    target.win.hidden = false;
    lockPage(target.win);
    if (which === 'username' && usernameLocked) {
      // Review fix WR-01: the sentence goes in after the window is shown, so the
      // status line announces it, and before focus reaches OK, which it describes.
      showWaitSentence(lockedSentence);
    }
    var focusTarget = which === 'username' ? usernameInput : (which === 'name' ? nameInput : homeInput);
    // A locked Username window puts focus on OK, the one action (Quick 261108-fku).
    if (which === 'username' && usernameLocked && usernameOk) {
      focusTarget = usernameOk;
    }
    focusTarget.focus();
    if (which === 'username' && !usernameLocked && typeof usernameInput.select === 'function') {
      usernameInput.select();
    }
  }

  // closeWindow is reached from the X and Escape, and from OK and Enter inside
  // the rename wait (Quick 261108-fku, owner decision 2026-11-08). restoreFocus
  // puts focus back on the row that opened the window.
  function closeWindow(which, restoreFocus) {
    var target = windows[which || current];
    if (!target || current === null || (which && which !== current)) {
      return;
    }
    if (current === 'home') {
      cancelSearch();
      resetTyping();
    }
    // A username check still due when the window shuts is not sent (IN-B05).
    if (current === 'username' && usernameCheck !== null && typeof usernameCheck.cancel === 'function') {
      usernameCheck.cancel();
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
    // Inside the rename wait nothing can be saved, so Enter anywhere but on OK
    // closes the window like OK does (Quick 261108-fku, owner decision
    // 2026-11-08). It is caught here, so it works whether or not the browser
    // would submit a form from a read only field; on OK the click closes.
    if (e.key === 'Enter' && which === 'username' && usernameLocked) {
      if (usernameOk && e.target === usernameOk) {
        return;
      }
      e.preventDefault();
      closeWindow('username', true);
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

  // refillAfterEarlierSave is called when a successful answer lands for a save
  // sent from an earlier opening of a window (sameWindow is false). When that
  // window is open again and its field still holds the value it was filled with
  // at this opening, the field takes the newly saved value, and the buttons are
  // worked out again from the flags. A field the person already changed, and a
  // home area place already picked, are left alone.
  function refillAfterEarlierSave(which, sameWindow, value) {
    if (sameWindow || current !== which) {
      return;
    }
    if (which === 'name') {
      if (nameInput.value === nameFilled) {
        nameInput.value = value;
        nameFilled = value;
      }
      nameSave.disabled = savingName;
      return;
    }
    if (picked === null && homeInput.value === homeFilled) {
      homeInput.value = value;
      homeFilled = value;
    }
    homeSave.disabled = savingHome;
  }

  // ---- Username -------------------------------------------------------------

  // attachUsernameCheck wires the shared live check the first time it is needed.
  // It judges the field at once, as step 4 does.
  function attachUsernameCheck() {
    if (usernameCheck === null && window.PinalertUsernameCheck && typeof window.PinalertUsernameCheck.attach === 'function') {
      usernameCheck = window.PinalertUsernameCheck.attach(usernameInput, usernameStatus, { mode: 'settings', onChange: onUsernameChange });
    }
  }

  // onUsernameChange is the live check's change hook. Quick 261108-fku (owner
  // decision 2026-11-08, D11): a wait answer while the window is in its normal
  // state (the change was made in another tab) switches it to the locked state
  // with the server's own sentence. Every state still redraws Save username.
  function onUsernameChange(state, ctl) {
    if (state === 'wait' && !usernameLocked && current === 'username') {
      // The server's own sentence locks the window now, and its day count sets the
      // deadline for the next openings (rounded up, so it is never early).
      var sentence = String(usernameStatus.textContent || '');
      waitUntil = Date.now() + daysInSentence(sentence) * DAY_MS;
      lockUsername(sentence);
    }
    updateUsernameSave(state, ctl);
  }

  // showWaitSentence writes the wait sentence into the status line with the wait
  // look. It is written through the check (hold) so the status line's classes stay
  // owned by username-check.js.
  function showWaitSentence(sentence) {
    if (usernameCheck !== null && typeof usernameCheck.hold === 'function') {
      usernameCheck.hold(sentence);
    } else {
      setText(usernameStatus, sentence);
    }
  }

  // lockUsername draws the window for the rename wait: the stored name in a read
  // only field, the sentence in the status line with the wait look, the help line
  // as it is, Save username hidden and one OK button. The field is made read only
  // before the live check is attached, so the check never judges it. Nothing is
  // saved from this state.
  // Review fix WR-01: a screen reader does not announce a live region that changes
  // while its window is hidden. So on an opening (the window still hidden) the
  // status line is cleared here and openWindow writes the sentence once the window
  // is shown; a window already open (a wait answer while it is open) gets the
  // sentence at once.
  function lockUsername(sentence) {
    usernameLocked = true;
    lockedSentence = sentence;
    usernameFilled = storedValue(usernameRow);
    usernameInput.value = usernameFilled;
    usernameInput.setAttribute('readonly', '');
    usernameInput.setAttribute('aria-readonly', 'true');
    attachUsernameCheck();
    if (usernameWin.hidden) {
      setText(usernameStatus, '');
    } else {
      showWaitSentence(sentence);
    }
    usernameSave.hidden = true;
    if (usernameOk) {
      usernameOk.hidden = false;
    }
    hideLine(usernameError);
    // A window already open moves focus to OK; an opening focuses it once the
    // window is shown.
    if (!usernameWin.hidden) {
      (usernameOk || usernameInput).focus();
    }
  }

  // unlockUsername draws the usual window again once the wait is over: the field
  // editable, Save username shown, OK hidden. readonly goes first, so the live
  // check judges the stored name again.
  function unlockUsername() {
    usernameLocked = false;
    usernameInput.removeAttribute('readonly');
    usernameInput.removeAttribute('aria-readonly');
    usernameSave.hidden = false;
    if (usernameOk) {
      usernameOk.hidden = true;
    }
  }

  // swapClass puts one class in place of another on a node, keeping the rest.
  function swapClass(node, remove, add) {
    var kept = String(node.className).split(/\s+/).filter(function (name) {
      return name !== '' && name !== remove && name !== add;
    });
    kept.push(add);
    node.className = kept.join(' ');
  }

  // updateUsernameSave draws Save username as on (the primary button) or off
  // (aria-disabled and the btn--off look; never the disabled attribute, so it
  // stays reachable). It is on only when the live check says the name is
  // available and no save is out. The live check calls it on every change.
  function updateUsernameSave(state, ctl) {
    var check = ctl || usernameCheck;
    if (!hasUsername) {
      return;
    }
    if (check && check.isGood() && !savingUsername) {
      usernameSave.removeAttribute('aria-disabled');
      swapClass(usernameSave, 'btn--off', 'btn--primary');
      return;
    }
    usernameSave.setAttribute('aria-disabled', 'true');
    swapClass(usernameSave, 'btn--primary', 'btn--off');
  }

  // showUsernameRefusal puts the answer to a refused save where the person
  // looks: the server's sentence for a 400 or 422 about the username goes under
  // the Username (and switches Save off until the name is typed again); a rate
  // limit, an ended session and anything else go to the general line.
  function showUsernameRefusal(answer) {
    var sent = serverSentence(answer.body);
    var field = answer.body && answer.body.error ? answer.body.error.field : '';
    if ((answer.status === 400 || answer.status === 422) && field === 'username' && sent !== '') {
      usernameCheck.showSaveError(sent);
      return;
    }
    if (answer.status === 401) {
      showLine(usernameError, MESSAGE_SESSION_ENDED);
      return;
    }
    if (answer.status === 429 && sent !== '') {
      showLine(usernameError, sent);
      return;
    }
    showLine(usernameError, MESSAGE_SAVE_FAILED);
  }

  function saveUsername() {
    if (!hasUsername || savingUsername || usernameCheck === null || !usernameCheck.isGood()) {
      return Promise.resolve();
    }
    var wanted = usernameCheck.name();
    savingUsername = true;
    updateUsernameSave();
    hideLine(usernameError);
    var mine = session;
    return patchProfile({ username: wanted }).then(function (answer) {
      savingUsername = false;
      var sameWindow = mine === session;
      if (!answer.ok) {
        if (sameWindow) {
          showUsernameRefusal(answer);
        }
        updateUsernameSave();
        return;
      }
      var stored = answer.body && typeof answer.body.username === 'string' && answer.body.username !== '' ? answer.body.username : wanted;
      var before = storedValue(usernameRow);
      setRow(usernameRow, usernameValue, stored);
      // Quick 261108-fku (owner decision 2026-11-08, C9; review fix WR-02): a name
      // that replaced an earlier one starts the wait, so the deadline in the page
      // is now plus the whole gap and the next opening is locked without a reload.
      // A first username saved from Not set starts no wait (the server keeps the
      // change time empty on a first set).
      if (before !== '' && before !== stored) {
        waitUntil = Date.now() + WAIT_GAP_DAYS * DAY_MS;
      }
      if (sameWindow) {
        closeWindow('username', true);
      } else if (current === 'username' && waitDaysLeft() > 0) {
        // A save from an earlier opening landed in the window open now: it locks.
        lockUsername(waitSentence(waitDaysLeft()));
      } else if (current === 'username' && usernameInput.value === usernameFilled) {
        // A save from an earlier opening landed in the window open now: the
        // untouched field takes the saved name (like the Name window).
        usernameInput.value = stored;
        usernameFilled = stored;
        usernameCheck.refresh();
      }
      updateUsernameSave();
    }, function () {
      savingUsername = false;
      if (mine === session) {
        showLine(usernameError, MESSAGE_SAVE_FAILED);
      }
      updateUsernameSave();
    });
  }

  // ---- Name -----------------------------------------------------------------

  function saveName() {
    if (savingName) {
      return Promise.resolve();
    }
    if (nameInput.value.trim() === '') {
      // Nothing is sent and the stored name stays.
      showLine(nameError, MESSAGE_NAME_EMPTY);
      return Promise.resolve();
    }
    savingName = true;
    nameSave.disabled = true;
    hideLine(nameError);
    var mine = session;
    return patchProfile({ display_name: nameInput.value.trim() }).then(function (answer) {
      savingName = false;
      nameSave.disabled = false;
      var sameWindow = mine === session;
      if (!answer.ok) {
        if (sameWindow) {
          showLine(nameError, saveMessage(answer));
        }
        return;
      }
      var name = answer.body && typeof answer.body.display_name === 'string' ? answer.body.display_name : '';
      setRow(nameRow, nameValue, name);
      refillAfterEarlierSave('name', sameWindow, name);
      if (sameWindow) {
        closeWindow('name', true);
      }
    }, function () {
      savingName = false;
      nameSave.disabled = false;
      if (mine === session) {
        showLine(nameError, MESSAGE_SAVE_FAILED);
      }
    });
  }

  // ---- Home area ------------------------------------------------------------

  function placeShortName(result) {
    if (typeof result.name === 'string' && result.name.trim() !== '') {
      return result.name.trim();
    }
    if (typeof result.display_name === 'string') {
      return result.display_name.split(',')[0].trim();
    }
    return '';
  }

  // noMatch is the server's no_match flag: no row closely matches what was
  // typed, no typed row could be placed, and every lookup that was needed
  // answered. unfinished is the server's unfinished flag: nothing matches
  // closely but a lookup that was needed failed (plan 35). quiet says the
  // typing pause started the search. An unfinished search, or no rows with
  // no_match not true, never says Not found: Enter says Search unavailable
  // (rows stay listed), a typed search says nothing.
  function renderResults(results, noMatch, unfinished, quiet) {
    homeResults.textContent = '';
    var shown = 0;
    var typedShown = false;
    for (var i = 0; i < results.length && shown < MAX_RESULTS; i++) {
      var res = results[i];
      var name = res ? placeShortName(res) : '';
      if (name === '' || typeof res.lat !== 'number' || typeof res.lon !== 'number' || !isFinite(res.lat) || !isFinite(res.lon)) {
        continue;
      }
      shown += 1;
      var item = document.createElement('li');
      item.setAttribute('role', 'presentation');
      var option = document.createElement('button');
      option.setAttribute('type', 'button');
      option.setAttribute('role', 'option');
      option.setAttribute('aria-selected', 'false');
      option.className = 'app-window__option';
      // typed marks the one row that is the person's own text, placed in the
      // town they typed (plan 32). It reads Use "<name>" with the town on the
      // second line, and picks like any other row: the name is the cleaned text
      // to save, the point is a place in that town.
      var typed = res.typed === true;
      typedShown = typedShown || typed;
      var label = document.createElement('span');
      setText(label, typed ? 'Use "' + name + '"' : name);
      option.appendChild(label);
      var second = typed && typeof res.town === 'string' && res.town.trim() !== '' ? res.town.trim() : res.display_name;
      if (typeof second === 'string' && second !== '' && second !== name) {
        var sub = document.createElement('span');
        sub.className = 'app-window__option-sub';
        setText(sub, second);
        option.appendChild(sub);
      }
      option.addEventListener('click', pickPlace.bind(null, option, { name: name, lat: res.lat, lon: res.lon }));
      item.appendChild(option);
      homeResults.appendChild(item);
    }
    if (shown === 0) {
      homeResults.hidden = true;
      if (noMatch) {
        showLine(homeStatus, MESSAGE_NOT_FOUND);
      } else if (quiet) {
        // An empty list the server did not call not found means a lookup did
        // not finish (plan 35): a typed search says nothing about it.
        hideLine(homeStatus);
      } else {
        showLine(homeStatus, MESSAGE_UNAVAILABLE);
      }
      return;
    }
    // A typed row is a match, so the line never goes with one.
    if (noMatch && !typedShown) {
      showLine(homeStatus, MESSAGE_NOT_FOUND);
    } else if (unfinished && !quiet && !typedShown) {
      showLine(homeStatus, MESSAGE_UNAVAILABLE);
    } else {
      hideLine(homeStatus);
    }
    homeResults.hidden = false;
  }

  function pickPlace(option, place) {
    // A pick ends a typing pause or a quiet retry, so a late answer never
    // reopens the list over the pick.
    cancelTyping();
    cancelSearch();
    clearPick();
    picked = place;
    option.setAttribute('aria-selected', 'true');
    homeInput.value = place.name;
    // A pick closes the suggestions (owner, 2026-10-18) and gives focus back
    // to the field.
    homeResults.hidden = true;
    // The status line goes with the list: a Not found line left over from the
    // search that offered this pick no longer applies (plan 35).
    hideLine(homeStatus);
    homeInput.focus();
    homeSave.disabled = savingHome;
    hideLine(homeError);
  }

  // searchPlaces runs one place search for the text in the field. It sends the
  // text only: this page has no map centre and never sends the device position.
  // A failure shows one plain line and leaves the window as it was.
  //
  // typed is true when the typing pause started it. A typed search is quiet:
  // no Searching line, no too short line and no failure line, since the person
  // did not ask for it and their own typing must never read as Search
  // unavailable. Enter shows every line, as before, and takes over a typed
  // search already out for the same text instead of asking again.
  function searchPlaces(query, typed) {
    var quiet = typed === true;
    var q = String(query === undefined ? homeInput.value : query).trim();
    // Enter, or the typed search itself, ends the pending pause.
    cancelTyping();
    if (!quiet) {
      hideLine(homeError);
    }
    if (runeCount(q) < MIN_QUERY_RUNES) {
      cancelSearch();
      if (!quiet) {
        homeResults.hidden = true;
        showLine(homeStatus, MESSAGE_TOO_SHORT);
      }
      return Promise.resolve();
    }
    if (searchRequest !== null && q === searchText) {
      if (!quiet && searchQuiet) {
        searchQuiet = false;
        clearPick();
        homeResults.hidden = true;
        showLine(homeStatus, MESSAGE_SEARCHING);
      }
      return searchRequest.promise.then(function () {}, function () {});
    }
    cancelSearch();
    if (!quiet) {
      clearPick();
      homeResults.hidden = true;
      showLine(homeStatus, MESSAGE_SEARCHING);
    }
    searchSeq += 1;
    var mine = searchSeq;
    searchText = q;
    searchQuiet = quiet;
    lastSentText = q;
    startGap();
    var call = request(PLACE_URL + encodeURIComponent(q), { credentials: 'same-origin' }, SEARCH_TIMEOUT_MS);
    searchRequest = call;
    function finish() {
      if (mine === searchSeq) {
        searchRequest = null;
        searchText = '';
      }
    }
    return call.promise.then(function (answer) {
      if (mine !== searchSeq) {
        return;
      }
      finish();
      if (!answer.ok) {
        // The same text may be sent again after a failure.
        lastSentText = '';
        var busy = answer.status === 429 || answer.status === 503;
        if (searchQuiet && busy) {
          // A refusal for text no longer in the box, or one that lands while
          // newer text waits for its pause, is dropped quietly.
          if (typingTimer !== null || waitingText !== null || q !== homeInput.value.trim()) {
            return;
          }
          if (quietFailures < QUIET_RETRIES) {
            quietFailures += 1;
            retryLater(q);
            return;
          }
        }
        if (!searchQuiet || busy) {
          if (searchQuiet) {
            homeResults.hidden = true;
          }
          var sent = serverSentence(answer.body);
          showLine(homeStatus, sent !== '' ? sent : (answer.status === 429 ? MESSAGE_RATE_LIMITED : MESSAGE_UNAVAILABLE));
        }
        return;
      }
      renderResults(answer.body && Array.isArray(answer.body.results) ? answer.body.results : [],
        !!answer.body && answer.body.no_match === true,
        !!answer.body && answer.body.unfinished === true, searchQuiet);
    }, function () {
      if (mine !== searchSeq) {
        return;
      }
      finish();
      lastSentText = '';
      if (!searchQuiet) {
        showLine(homeStatus, MESSAGE_UNAVAILABLE);
      }
    });
  }

  // startGap starts the wait before the next typed search may go out. A typed
  // search that comes due meanwhile is held in waitingText and sent when the
  // gap ends, if it is still the text in the box.
  function startGap() {
    if (coolTimer !== null) {
      window.clearTimeout(coolTimer);
    }
    coolTimer = window.setTimeout(function () {
      coolTimer = null;
      if (waitingText !== null) {
        var text = waitingText;
        waitingText = null;
        typedSearch(text);
      }
    }, TYPING_GAP_MS);
  }

  function typedSearch(q) {
    if (coolTimer !== null) {
      waitingText = q;
      return;
    }
    searchPlaces(q, true);
  }

  // retryLater asks for q again, quietly. It waits on the typing timer, so
  // anything that ends a typing pause also ends it.
  function retryLater(q) {
    cancelTyping();
    typingTimer = window.setTimeout(function () {
      typingTimer = null;
      searchPlaces(q, true);
    }, RETRY_WAIT_MS);
  }

  // onHomeInput is the typing path: every keystroke restarts the pause, and the
  // search runs once the person stops for TYPING_PAUSE_MS with at least 3
  // letters in the field and text that was not the last one sent. Under 3
  // letters it drops any search and its results quietly.
  function onHomeInput() {
    cancelTyping();
    quietFailures = 0;
    var q = homeInput.value.trim();
    if (runeCount(q) < MIN_QUERY_RUNES) {
      cancelSearch();
      lastSentText = '';
      homeResults.hidden = true;
      hideLine(homeStatus);
      return;
    }
    if (q === lastSentText) {
      return;
    }
    typingTimer = window.setTimeout(function () {
      typingTimer = null;
      typedSearch(q);
    }, TYPING_PAUSE_MS);
  }

  // afterHomeSave settles a save of the home area. sameWindow says
  // whether the window that sent it is still the one open: only then does the
  // answer write a refusal into the window. The row always takes what the
  // server stored, and the buttons always follow the flag and the pick.
  function afterHomeSave(answer, sameWindow) {
    savingHome = false;
    homeSave.disabled = false;
    if (!answer.ok) {
      if (sameWindow) {
        showLine(homeError, saveMessage(answer));
      }
      return false;
    }
    var name = answer.body && typeof answer.body.home_area_name === 'string' ? answer.body.home_area_name : '';
    setRow(homeRow, homeValue, name);
    refillAfterEarlierSave('home', sameWindow, name);
    return true;
  }

  function saveHomeArea() {
    if (savingHome) {
      return Promise.resolve();
    }
    // A home area can be changed but not removed (D-15): with nothing picked
    // nothing is sent, the stored area stays, and the person is told what to do.
    if (picked === null) {
      showLine(homeError, MESSAGE_CHOOSE_HOME);
      return Promise.resolve();
    }
    savingHome = true;
    homeSave.disabled = true;
    hideLine(homeError);
    var mine = session;
    return patchProfile({ home_area: { name: picked.name, lat: picked.lat, lon: picked.lon } }).then(function (answer) {
      if (afterHomeSave(answer, mine === session) && mine === session) {
        closeWindow('home', true);
      }
    }, function () {
      savingHome = false;
      homeSave.disabled = false;
      if (mine === session) {
        showLine(homeError, MESSAGE_SAVE_FAILED);
      }
    });
  }

  // ---- Wiring ---------------------------------------------------------------

  nameRow.addEventListener('click', function () { openWindow('name'); });
  homeRow.addEventListener('click', function () { openWindow('home'); });
  nameClose.addEventListener('click', function () { closeWindow('name', true); });
  homeClose.addEventListener('click', function () { closeWindow('home', true); });
  nameWin.addEventListener('keydown', function (e) { onWindowKeydown('name', e); });
  homeWin.addEventListener('keydown', function (e) { onWindowKeydown('home', e); });

  nameForm.addEventListener('submit', function (e) {
    e.preventDefault();
    saveName();
  });
  homeForm.addEventListener('submit', function (e) {
    e.preventDefault();
    searchPlaces();
  });
  homeInput.addEventListener('input', function () {
    if (picked !== null && homeInput.value !== picked.name) {
      clearPick();
    }
    onHomeInput();
  });
  homeSave.addEventListener('click', function () { saveHomeArea(); });
  if (hasUsername) {
    usernameRow.addEventListener('click', function () { openWindow('username'); });
    usernameClose.addEventListener('click', function () { closeWindow('username', true); });
    usernameWin.addEventListener('keydown', function (e) { onWindowKeydown('username', e); });
    if (usernameOk) {
      usernameOk.addEventListener('click', function () { closeWindow('username', true); });
    }
    usernameForm.addEventListener('submit', function (e) {
      e.preventDefault();
      // Inside the rename wait a submit closes the window like OK (Quick 261108-fku).
      if (usernameLocked) {
        closeWindow('username', true);
        return;
      }
      if (usernameSave.getAttribute('aria-disabled') === 'true') {
        // Review fix WR-B03: a check that failed is asked again by the press,
        // so the person is never left with nothing to press. Nothing is saved.
        if (usernameCheck !== null && !savingUsername && usernameCheck.state() === 'failed') {
          usernameCheck.refresh();
        }
        return;
      }
      saveUsername();
    });
  }

  window.PinalertSettings = {
    openWindow: openWindow,
    closeWindow: closeWindow,
    saveUsername: saveUsername,
    saveName: saveName,
    saveHomeArea: saveHomeArea,
    searchPlaces: searchPlaces
  };
}());
