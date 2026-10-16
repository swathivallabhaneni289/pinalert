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
  var PLACE_URL = '/api/geocode/place?q=';
  var SAVE_TIMEOUT_MS = 10000;
  var SEARCH_TIMEOUT_MS = 10000;
  var MIN_QUERY_RUNES = 3;
  var MAX_RESULTS = 5;
  var NOT_SET = 'Not set';

  // The search wording is the same as the reader lane search of the map page
  // (map-tools.js), so one sentence means one thing across the app.
  var MESSAGE_SEARCHING = 'Searching…';
  var MESSAGE_TOO_SHORT = 'Type at least 3 letters.';
  var MESSAGE_NO_MATCH = 'No matches found. Try a nearby town or landmark.';
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
  var saving = false;

  // The picked place, or null. Only a result of the place search can be picked,
  // so a home area is never anything the person typed or the device reported.
  var picked = null;
  var searchSeq = 0;
  var searchRequest = null;
  var searchText = '';

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
    saving = false;
    if (which === 'name') {
      hideLine(nameError);
      nameInput.value = storedValue(nameRow);
      nameSave.disabled = false;
    } else {
      hideLine(homeError);
      hideLine(homeStatus);
      homeResults.textContent = '';
      homeResults.hidden = true;
      cancelSearch();
      picked = null;
      homeSave.disabled = true;
      homeInput.value = storedValue(homeRow);
      homeRemove.hidden = storedValue(homeRow) === '';
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
    }
    target.win.hidden = true;
    unlockPage();
    current = null;
    saving = false;
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

  // ---- Name -----------------------------------------------------------------

  function saveName() {
    if (saving) {
      return Promise.resolve();
    }
    saving = true;
    nameSave.disabled = true;
    hideLine(nameError);
    // An emptied name is sent as an empty string, which removes the stored one.
    return patchProfile({ display_name: nameInput.value.trim() }).then(function (answer) {
      saving = false;
      nameSave.disabled = false;
      if (!answer.ok) {
        showLine(nameError, saveMessage(answer));
        return;
      }
      var name = answer.body && typeof answer.body.display_name === 'string' ? answer.body.display_name : '';
      setRow(nameRow, nameValue, name);
      closeWindow('name', true);
    }, function () {
      saving = false;
      nameSave.disabled = false;
      showLine(nameError, MESSAGE_SAVE_FAILED);
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

  function renderResults(results) {
    homeResults.textContent = '';
    var shown = 0;
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
      var label = document.createElement('span');
      setText(label, name);
      option.appendChild(label);
      if (typeof res.display_name === 'string' && res.display_name !== '' && res.display_name !== name) {
        var sub = document.createElement('span');
        sub.className = 'app-window__option-sub';
        setText(sub, res.display_name);
        option.appendChild(sub);
      }
      option.addEventListener('click', pickPlace.bind(null, option, { name: name, lat: res.lat, lon: res.lon }));
      item.appendChild(option);
      homeResults.appendChild(item);
    }
    if (shown === 0) {
      homeResults.hidden = true;
      showLine(homeStatus, MESSAGE_NO_MATCH);
      return;
    }
    hideLine(homeStatus);
    homeResults.hidden = false;
  }

  function pickPlace(option, place) {
    clearPick();
    picked = place;
    option.setAttribute('aria-selected', 'true');
    homeInput.value = place.name;
    homeSave.disabled = false;
    hideLine(homeError);
  }

  // searchPlaces runs one place search for the text in the field. It sends the
  // text only: this page has no map centre and never sends the device position.
  // A failure shows one plain line and leaves the window as it was.
  function searchPlaces(query) {
    var q = String(query === undefined ? homeInput.value : query).trim();
    hideLine(homeError);
    if (runeCount(q) < MIN_QUERY_RUNES) {
      cancelSearch();
      homeResults.hidden = true;
      showLine(homeStatus, MESSAGE_TOO_SHORT);
      return Promise.resolve();
    }
    if (searchRequest !== null && q === searchText) {
      return searchRequest.promise.then(function () {}, function () {});
    }
    cancelSearch();
    clearPick();
    homeResults.hidden = true;
    showLine(homeStatus, MESSAGE_SEARCHING);
    searchSeq += 1;
    var mine = searchSeq;
    searchText = q;
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
        var sent = serverSentence(answer.body);
        showLine(homeStatus, sent !== '' ? sent : (answer.status === 429 ? MESSAGE_RATE_LIMITED : MESSAGE_UNAVAILABLE));
        return;
      }
      renderResults(answer.body && Array.isArray(answer.body.results) ? answer.body.results : []);
    }, function () {
      if (mine !== searchSeq) {
        return;
      }
      finish();
      showLine(homeStatus, MESSAGE_UNAVAILABLE);
    });
  }

  function afterHomeSave(answer) {
    saving = false;
    homeSave.disabled = picked === null;
    if (!answer.ok) {
      showLine(homeError, saveMessage(answer));
      return false;
    }
    var name = answer.body && typeof answer.body.home_area_name === 'string' ? answer.body.home_area_name : '';
    setRow(homeRow, homeValue, name);
    homeRemove.hidden = name === '';
    return true;
  }

  function saveHomeArea() {
    if (saving || picked === null) {
      return Promise.resolve();
    }
    saving = true;
    homeSave.disabled = true;
    hideLine(homeError);
    return patchProfile({ home_area: { name: picked.name, lat: picked.lat, lon: picked.lon } }).then(function (answer) {
      if (afterHomeSave(answer)) {
        closeWindow('home', true);
      }
    }, function () {
      saving = false;
      homeSave.disabled = picked === null;
      showLine(homeError, MESSAGE_SAVE_FAILED);
    });
  }

  // removeHomeArea deletes the stored home area at once. The window stays open
  // with an empty field, so the person can pick another place or close it.
  function removeHomeArea() {
    if (saving) {
      return Promise.resolve();
    }
    saving = true;
    homeRemove.disabled = true;
    hideLine(homeError);
    return patchProfile({ home_area: null }).then(function (answer) {
      homeRemove.disabled = false;
      if (afterHomeSave(answer)) {
        homeInput.value = '';
        hideLine(homeStatus);
        homeResults.textContent = '';
        homeResults.hidden = true;
        clearPick();
        homeInput.focus();
      }
    }, function () {
      saving = false;
      homeRemove.disabled = false;
      showLine(homeError, MESSAGE_SAVE_FAILED);
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
