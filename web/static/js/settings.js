// settings.js: the Name and Home area edit windows of the Settings page
// (Phase 3.4, plan 06, UI-SPEC Surface 6, D-24b, D-02).
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
//     no backdrop handler, no Cancel and no Done.
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
  var MESSAGE_SESSION_ENDED = 'Your session has ended. Reload the page and log in again.';

  var FOCUSABLE_SELECTOR = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

  function el(id) {
    return document.getElementById(id);
  }

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
  var homeRemove = el('settings-home-remove');
  var homeError = el('settings-home-error');

  var windows = {
    name: { win: nameWin, row: nameRow, close: nameClose },
    home: { win: homeWin, row: homeRow, close: homeClose }
  };

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
  // so the focus trap skips the hidden Remove button and a hidden results list.
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
    homeSave.disabled = true;
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
    if (which === 'name') {
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
      homeSave.disabled = true;
      homeFilled = storedValue(homeRow);
      homeInput.value = homeFilled;
      homeRemove.hidden = storedValue(homeRow) === '';
      homeRemove.disabled = savingHome;
    }
    target.win.hidden = false;
    lockPage(target.win);
    (which === 'name' ? nameInput : homeInput).focus();
  }

  // closeWindow is reached from the X and Escape only. restoreFocus puts focus
  // back on the row that opened the window.
  function closeWindow(which, restoreFocus) {
    var target = windows[which || current];
    if (!target || current === null || (which && which !== current)) {
      return;
    }
    if (current === 'home') {
      cancelSearch();
      resetTyping();
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
    homeRemove.hidden = storedValue(homeRow) === '';
    homeRemove.disabled = savingHome;
    homeSave.disabled = picked === null || savingHome;
  }

  // ---- Name -----------------------------------------------------------------

  function saveName() {
    if (savingName) {
      return Promise.resolve();
    }
    savingName = true;
    nameSave.disabled = true;
    hideLine(nameError);
    var mine = session;
    // An emptied name is sent as an empty string, which removes the stored one.
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
  // typed and no typed row could be placed.
  function renderResults(results, noMatch) {
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
      showLine(homeStatus, MESSAGE_NOT_FOUND);
      return;
    }
    // A typed row is a match, so the line never goes with one.
    if (noMatch && !typedShown) {
      showLine(homeStatus, MESSAGE_NOT_FOUND);
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
        !!answer.body && answer.body.no_match === true);
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

  // afterHomeSave settles a save or a removal of the home area. sameWindow says
  // whether the window that sent it is still the one open: only then does the
  // answer write a refusal into the window. The row always takes what the
  // server stored, and the buttons always follow the flag and the pick.
  function afterHomeSave(answer, sameWindow) {
    savingHome = false;
    homeSave.disabled = picked === null;
    homeRemove.disabled = false;
    if (!answer.ok) {
      if (sameWindow) {
        showLine(homeError, saveMessage(answer));
      }
      return false;
    }
    var name = answer.body && typeof answer.body.home_area_name === 'string' ? answer.body.home_area_name : '';
    setRow(homeRow, homeValue, name);
    homeRemove.hidden = name === '';
    refillAfterEarlierSave('home', sameWindow, name);
    return true;
  }

  function saveHomeArea() {
    if (savingHome || picked === null) {
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
      homeSave.disabled = picked === null;
      if (mine === session) {
        showLine(homeError, MESSAGE_SAVE_FAILED);
      }
    });
  }

  // removeHomeArea deletes the stored home area at once. The window stays open
  // with an empty field, so the person can pick another place or close it.
  function removeHomeArea() {
    if (savingHome) {
      return Promise.resolve();
    }
    savingHome = true;
    homeRemove.disabled = true;
    hideLine(homeError);
    var mine = session;
    return patchProfile({ home_area: null }).then(function (answer) {
      var sameWindow = mine === session;
      if (afterHomeSave(answer, sameWindow) && sameWindow) {
        homeInput.value = '';
        hideLine(homeStatus);
        homeResults.textContent = '';
        homeResults.hidden = true;
        clearPick();
        homeInput.focus();
      }
    }, function () {
      savingHome = false;
      homeRemove.disabled = false;
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
  homeRemove.addEventListener('click', function () { removeHomeArea(); });

  window.PinalertSettings = {
    openWindow: openWindow,
    closeWindow: closeWindow,
    saveName: saveName,
    saveHomeArea: saveHomeArea,
    removeHomeArea: removeHomeArea,
    searchPlaces: searchPlaces
  };
}());
