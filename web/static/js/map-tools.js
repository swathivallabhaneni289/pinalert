// map-tools.js: window.PinalertMapTools, the tools laid over the map. A place
// search (HIST-05) and five category chips (HIST-06), plus the two context
// lines of the list pane that say why the list is not the default one.
//
// This module owns the place search request, the only network call added by
// 03.1. It is a module of its own because app.js keeps exactly two network
// calls (the list and the submit) and feed.js and activity.js keep none. The
// list itself is still read by app.js alone: a picked place or a tapped chip
// is written to the store and the one list request is run again, so the list
// and the map pins always come from one answer (D-15, D-17).
//
// Rules this file keeps:
//   - The box is read on submit, and after the reader stops typing for
//     TYPING_PAUSE_MS (quick 261016-sfu, which replaced the submit only rule
//     of D-14 and T-31-51). Never per keystroke, never under 3 letters. A newer
//     search ends the older one, and Enter for the text already out takes that
//     search over instead of asking again.
//   - A search started by typing is quiet: no Searching line, no too short
//     line and no failure line, since the reader did not ask for it and their
//     own typing must never read as Search unavailable. Enter and the button
//     show every line. A typed search's results and count are shown and read
//     out the same way as after Enter.
//   - A typed search refused (429) or answered busy (503) is tried again
//     quietly (quick 261016-sne): the list on screen stays, and the latest
//     text is asked again after the answer's Retry-After or RETRY_WAIT_MS, at
//     most QUIET_RETRIES times, so suggestions catch up instead of stopping.
//     Only when the last try fails too does the failure line show, since by
//     then the search has really been unavailable for a few seconds. Newer
//     typing, Enter, Escape, Close, a press outside and a pick all cancel a
//     pending try, because it waits on the typing timer. A network error from
//     typing stays quiet and is not tried again.
//   - Dragging or zooming the map never reads the list. Only a pick, a chip,
//     Back to my location and Clear do (D-14).
//   - A failed or refused search shows one plain line and blocks nothing. The
//     report form's own search is another route and another script (D-16).
//   - Names from the geocoder reach the page through Pinalert.setText only
//     (T-31-49). The place and the chip live in the store and nowhere else:
//     nothing is saved and nothing is written to the address (D-18, T-31-52).
window.PinalertMapTools = (function () {
  'use strict';

  var MIN_QUERY_RUNES = 3;
  // How long the reader must stop typing before the search runs by itself,
  // the same pause as the report form's search (SEARCH_DEBOUNCE_MS).
  var TYPING_PAUSE_MS = 300;
  // The quiet retry of a typed search (see the rules at the top), the same
  // numbers as the report form's. A Retry-After longer than RETRY_WAIT_MAX_MS
  // is cut to it.
  var QUIET_RETRIES = 3;
  var RETRY_WAIT_MS = 1000;
  var RETRY_WAIT_MAX_MS = 10000;
  var MAX_RESULTS = 5;
  var RESULT_NAME_MAX = 32;
  // The zoom a GPS start uses.
  var RESULT_ZOOM = 13;
  // The condition phone.css lays the one row phone tools out at.
  var NARROW_QUERY = '(max-width: 899.98px)';

  // The status words (C27 to C32) and the location line (C35).
  var MESSAGE_SEARCHING = 'Searching\u2026';
  var MESSAGE_TOO_SHORT = 'Type at least 3 letters.';
  var MESSAGE_NO_MATCH = 'No matches found. Try a nearby town or landmark.';
  var MESSAGE_UNAVAILABLE = 'Search unavailable. Try again in a moment.';
  var MESSAGE_RATE_LIMITED = 'Too many searches. Wait a few seconds and try again.';
  var MESSAGE_LOCATION_LOST = "Your location isn't available. Showing the default area.";
  // The line above the results when a shorter text found them (quick task
  // 261016-adr). The matched text is what the reader typed, cut down by the
  // server, so it reaches the page through Pinalert.setText only.
  var MESSAGE_PARTIAL_PREFIX = 'No exact match for the full address. Showing places for: ';

  var toolsEl = document.querySelector('.map-tools');
  var formEl = document.getElementById('map-search');
  var openBtn = document.getElementById('map-search-open');
  var inputEl = document.getElementById('map-search-input');
  var closeBtn = document.getElementById('map-search-close');
  var statusEl = document.getElementById('map-search-status');
  var resultsEl = document.getElementById('map-search-results');
  var chipsEl = document.querySelector('.map-chips');
  var placeLineEl = document.getElementById('place-line');
  var filterLineEl = document.getElementById('filter-line');

  // The results on screen, in the server's order.
  var lastResults = [];
  // searchSeq numbers every search and every dismissal, so an answer for an
  // older search, or for one the reader closed, is dropped (T-31-53).
  var searchSeq = 0;
  var inFlight = false;
  var controller = null;
  // The text of the search in flight, and whether it is quiet (started by
  // typing, not by Enter or the button).
  var searchText = '';
  var searchQuiet = false;
  // The text whose answer is on screen, so a pause on the same text asks
  // nothing again.
  var shownText = '';
  // The pending typing pause or quiet retry, or null.
  var typingTimer = null;
  // How many quiet retries the text typed last has used.
  var quietFailures = 0;
  // True while Back to my location waits for the browser's position.
  var backPending = false;
  // True after Back to my location found no position, until the next pick.
  var locationLost = false;
  // The place line and the filter line are rebuilt only when their content
  // changes, so a store notification never drops focus from their button.
  var placeLineKey = null;
  var filterLineKey = null;

  function isNarrow() {
    return !!(window.matchMedia && window.matchMedia(NARROW_QUERY).matches);
  }

  // cutName shortens a place name to RESULT_NAME_MAX characters, the last one
  // an ellipsis, the same cut the store applies to the place line.
  function cutName(name) {
    var runes = Array.from(name);
    if (runes.length <= RESULT_NAME_MAX) {
      return name;
    }
    return runes.slice(0, RESULT_NAME_MAX - 1).join('') + '\u2026';
  }

  // showStatus writes one status line. A quiet line is announced to a screen
  // reader but not drawn (the result count is redundant next to the list).
  function showStatus(message, quiet) {
    Pinalert.setText(statusEl, message);
    statusEl.hidden = false;
    statusEl.classList.toggle('visually-hidden', quiet === true);
  }

  function clearStatus() {
    Pinalert.setText(statusEl, '');
    statusEl.hidden = true;
    statusEl.classList.remove('visually-hidden');
  }

  // closeResults hides the list of results and the status line and forgets the
  // results. It is the one dismissal.
  function closeResults() {
    resultsEl.hidden = true;
    resultsEl.textContent = '';
    lastResults = [];
    shownText = '';
    clearStatus();
  }

  // setOpen expands or collapses the phone row. On a laptop the box is always
  // drawn, so the attribute is only read by the phone rules.
  function setOpen(open) {
    toolsEl.setAttribute('data-search-open', open ? 'true' : 'false');
    openBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
  }

  // focusSearch returns focus to the search control: the search button on a
  // phone, the box on a laptop.
  function focusSearch() {
    if (isNarrow()) {
      openBtn.focus();
    } else {
      inputEl.focus();
    }
  }

  // cancelTyping drops a pending typing pause, if any.
  function cancelTyping() {
    if (typingTimer !== null) {
      window.clearTimeout(typingTimer);
      typingTimer = null;
    }
  }

  // abortSearch drops a pending typing pause and the request in flight, if
  // any, and invalidates its answer.
  function abortSearch() {
    cancelTyping();
    searchSeq += 1;
    inFlight = false;
    if (controller) {
      controller.abort();
      controller = null;
    }
  }

  // validResult is true for a result with a text name and coordinates inside
  // the range the store and the server accept.
  function validResult(r) {
    return !!r && typeof r === 'object' &&
      (typeof r.name === 'string' || typeof r.display_name === 'string') &&
      (r.name || r.display_name) &&
      typeof r.lat === 'number' && isFinite(r.lat) && r.lat >= -90 && r.lat <= 90 &&
      typeof r.lon === 'number' && isFinite(r.lon) && r.lon >= -180 && r.lon <= 180;
  }

  // renderResults draws at most MAX_RESULTS buttons, in the order received.
  // When matched is set, a shorter text than the one typed found them, and one
  // plain line saying so sits above the buttons. It is not a button, so the
  // click handler's closest('.map-search__result') never picks it.
  function renderResults(list, matched) {
    resultsEl.textContent = '';
    lastResults = [];
    var partial = '';
    if (typeof matched === 'string' && matched !== '') {
      partial = MESSAGE_PARTIAL_PREFIX + matched + '.';
      var note = document.createElement('p');
      note.className = 'map-search__note';
      Pinalert.setText(note, partial);
      resultsEl.appendChild(note);
    }
    for (var i = 0; i < list.length && lastResults.length < MAX_RESULTS; i++) {
      if (!validResult(list[i])) {
        continue;
      }
      var result = list[i];
      var index = lastResults.length;
      lastResults.push(result);

      var button = document.createElement('button');
      button.type = 'button';
      button.className = 'map-search__result';
      button.setAttribute('data-result-index', String(index));

      var primaryText = result.name || result.display_name;
      var name = document.createElement('span');
      name.className = 'map-search__result-name';
      Pinalert.setText(name, cutName(primaryText));
      button.appendChild(name);

      if (typeof result.display_name === 'string' && result.display_name !== '' &&
          result.display_name !== primaryText) {
        var full = document.createElement('span');
        full.className = 'map-search__result-full';
        Pinalert.setText(full, result.display_name);
        button.appendChild(full);
      }
      resultsEl.appendChild(button);
    }
    if (lastResults.length === 0) {
      resultsEl.textContent = '';
      resultsEl.hidden = true;
      showStatus(MESSAGE_NO_MATCH, false);
      return;
    }
    resultsEl.hidden = false;
    var count = lastResults.length;
    var countText = count === 1 ? '1 result.' : String(count) + ' results.';
    // The quiet line is read out, not drawn, so it carries the partial
    // match line too.
    showStatus(partial ? partial + ' ' + countText : countText, true);
  }

  // retryWait reads how long to wait before a quiet retry: the answer's
  // Retry-After in whole seconds when it sent a usable one, else
  // RETRY_WAIT_MS, never more than RETRY_WAIT_MAX_MS.
  function retryWait(res) {
    var header = res && res.headers && typeof res.headers.get === 'function'
      ? res.headers.get('Retry-After') : null;
    var seconds = header === null ? NaN : Number(header);
    if (!isFinite(seconds) || seconds <= 0) {
      return RETRY_WAIT_MS;
    }
    return Math.min(Math.max(seconds * 1000, RETRY_WAIT_MS), RETRY_WAIT_MAX_MS);
  }

  // retryLater asks for q again, quietly, after wait milliseconds. It waits on
  // the typing timer, so anything that ends a typing pause also ends it.
  function retryLater(q, wait) {
    cancelTyping();
    typingTimer = window.setTimeout(function () {
      typingTimer = null;
      search(q, true);
    }, wait);
  }

  // messageFor picks the line for a refused or failed answer: the server's own
  // sentence when it sent one, else the fixed words for the status.
  function messageFor(status, body) {
    var sent = body && body.error && body.error.message;
    if (typeof sent === 'string' && sent !== '' && sent.length <= 200) {
      return sent;
    }
    return status === 429 ? MESSAGE_RATE_LIMITED : MESSAGE_UNAVAILABLE;
  }

  // search runs one place search. typed is true when the typing pause started
  // it, which makes it quiet (see the rules at the top). It sends nothing for
  // fewer than 3 letters, ends an older search still in flight, and drops an
  // answer that is no longer the newest. A search from Enter or the button
  // that goes out while Back to my location is still waiting cancels it: the
  // latest action wins (IN-07). Typing alone is not an action that moves the
  // map, so it leaves Back alone.
  function search(query, typed) {
    var quiet = typed === true;
    var q = String(query === undefined ? inputEl.value : query).trim();
    cancelTyping();
    if (Array.from(q).length < MIN_QUERY_RUNES) {
      if (!quiet) {
        abortSearch();
        closeResults();
        showStatus(MESSAGE_TOO_SHORT, false);
      }
      return;
    }
    if (inFlight && q === searchText) {
      // Enter for the text already out takes that search over, so the
      // reader sees Searching and any failure line, without a second request.
      if (!quiet && searchQuiet) {
        searchQuiet = false;
        cancelBack();
        closeResults();
        showStatus(MESSAGE_SEARCHING, false);
      }
      return;
    }
    if (quiet && !inFlight && q === shownText) {
      return;
    }
    abortSearch();
    if (!quiet) {
      cancelBack();
      closeResults();
      showStatus(MESSAGE_SEARCHING, false);
    }
    searchSeq += 1;
    var mySeq = searchSeq;
    inFlight = true;
    searchText = q;
    searchQuiet = quiet;

    var url = '/api/geocode/place?q=' + encodeURIComponent(q);
    var centre = PinalertMap.getCenter();
    if (centre && isFinite(centre.lat) && isFinite(centre.lon)) {
      url += '&lat=' + encodeURIComponent(centre.lat.toFixed(5));
      url += '&lon=' + encodeURIComponent(centre.lon.toFixed(5));
    }

    controller = new AbortController();
    fetch(url, { signal: controller.signal })
      .then(function (res) {
        return res.json().catch(function () {
          return {};
        }).then(function (body) {
          if (mySeq !== searchSeq) {
            return;
          }
          inFlight = false;
          if (!res.ok) {
            // A quiet search's refusal or failure leaves the screen as it
            // was: the list for the text before stays until Enter asks. A
            // 429 or a 503 is tried again quietly, and the failure after the
            // last try shows its line in place of that list, as after Enter.
            var busy = res.status === 429 || res.status === 503;
            if (searchQuiet && busy && quietFailures < QUIET_RETRIES) {
              quietFailures += 1;
              retryLater(q, retryWait(res));
              return;
            }
            var lastTry = searchQuiet && busy;
            if (lastTry) {
              closeResults();
            }
            if (!searchQuiet || lastTry) {
              showStatus(messageFor(res.status, body), false);
            }
            return;
          }
          renderResults(body && Array.isArray(body.results) ? body.results : [],
            body && body.matched_query);
          shownText = q;
        });
      })
      .catch(function (err) {
        if (err && err.name === 'AbortError') {
          return;
        }
        if (mySeq !== searchSeq) {
          return;
        }
        inFlight = false;
        if (!searchQuiet) {
          showStatus(MESSAGE_UNAVAILABLE, false);
        }
      });
  }

  // onInput is the typing path: every keystroke restarts the pause, and the
  // search runs once the reader stops for TYPING_PAUSE_MS with at least 3
  // letters in the box. Under 3 letters drops any search quietly.
  function onInput() {
    // Also cancels a pending quiet retry: newer text wins, and gets its own
    // retries.
    cancelTyping();
    quietFailures = 0;
    var q = inputEl.value.trim();
    if (Array.from(q).length < MIN_QUERY_RUNES) {
      abortSearch();
      closeResults();
      return;
    }
    typingTimer = window.setTimeout(function () {
      typingTimer = null;
      search(q, true);
    }, TYPING_PAUSE_MS);
  }

  // pickResult moves the map and the list to the i'th result. The centre is
  // marked as not the reader's own, so distances read "from {Place}" and the
  // list is read once around the place. The map is moved here and never
  // refetched by a move (D-14).
  function pickResult(i) {
    var pick = lastResults[i];
    if (!pick) {
      return;
    }
    var name = pick.name || pick.display_name;
    // A pick ends a typing pause or a typed search still out, so its late
    // answer never reopens the list.
    abortSearch();
    // A pick wins over a Back to my location still waiting (IN-07).
    cancelBack();
    // A place change ends a report link (CR-01).
    Pinalert.dropFocus();
    Pinalert.setCenter(pick.lat, pick.lon, false);
    Pinalert.setPlace({ name: name, lat: pick.lat, lon: pick.lon });
    locationLost = false;
    PinalertMap.moveTo(pick.lat, pick.lon, RESULT_ZOOM);
    Pinalert.fetchReports();
    closeResults();
    renderPlaceLine(Pinalert.state);
    if (isNarrow()) {
      setOpen(false);
    }
    focusSearch();
  }

  // renderChips marks the chip of the store's category as pressed and no other.
  function renderChips(state) {
    if (!chipsEl) {
      return;
    }
    var chips = chipsEl.querySelectorAll('.map-chip');
    for (var i = 0; i < chips.length; i++) {
      var slug = chips[i].getAttribute('data-category') || '';
      chips[i].setAttribute('aria-pressed', slug === state.category ? 'true' : 'false');
    }
  }

  // onChipPress turns one category on, or back to All on a second press. A
  // word outside the store's chip table is ignored.
  function onChipPress(evt) {
    var chip = evt.target && evt.target.closest ? evt.target.closest('.map-chip') : null;
    if (!chip) {
      return;
    }
    var slug = chip.getAttribute('data-category') || '';
    if (slug !== '' && !Object.prototype.hasOwnProperty.call(Pinalert.CHIP_LABELS, slug)) {
      return;
    }
    var next = slug === Pinalert.state.category ? '' : slug;
    if (next === Pinalert.state.category) {
      return;
    }
    // A chip change ends a report link (CR-01).
    Pinalert.dropFocus();
    Pinalert.setCategory(next);
    renderChips(Pinalert.state);
    renderFilterLine(Pinalert.state);
    Pinalert.fetchReports();
  }

  // lineButton builds the inline text button of a context line.
  function lineButton(text, name, onPress) {
    var button = document.createElement('button');
    button.type = 'button';
    button.className = 'inline-text-button';
    Pinalert.setText(button, text);
    if (name) {
      button.setAttribute('aria-label', name);
    }
    button.addEventListener('click', onPress);
    return button;
  }

  // renderPlaceLine shows "Showing reports near {Place}. Back to my location."
  // while a searched place is active, the location line after a Back that
  // found no position, and nothing otherwise.
  function renderPlaceLine(state) {
    if (!placeLineEl) {
      return;
    }
    var key = state.place ? 'place:' + state.place.name : (locationLost ? 'lost' : '');
    if (key === placeLineKey) {
      return;
    }
    placeLineKey = key;
    placeLineEl.textContent = '';
    if (key === '') {
      placeLineEl.hidden = true;
      return;
    }
    var text = document.createElement('span');
    if (state.place) {
      Pinalert.setText(text, 'Showing reports near ' + state.place.name + '.');
      placeLineEl.appendChild(text);
      placeLineEl.appendChild(lineButton('Back to my location', 'Back to my location', onBackPress));
    } else {
      Pinalert.setText(text, MESSAGE_LOCATION_LOST);
      placeLineEl.appendChild(text);
    }
    placeLineEl.hidden = false;
  }

  // renderFilterLine shows "Filtered by {Flood}. Clear" while a chip is on, so
  // a short list always says why.
  function renderFilterLine(state) {
    if (!filterLineEl) {
      return;
    }
    var word = state.category !== '' && Object.prototype.hasOwnProperty.call(Pinalert.CHIP_LABELS, state.category)
      ? Pinalert.CHIP_LABELS[state.category] : '';
    var key = word;
    if (key === filterLineKey) {
      return;
    }
    filterLineKey = key;
    filterLineEl.textContent = '';
    if (key === '') {
      filterLineEl.hidden = true;
      return;
    }
    var text = document.createElement('span');
    Pinalert.setText(text, 'Filtered by ' + word + '.');
    filterLineEl.appendChild(text);
    filterLineEl.appendChild(lineButton('Clear', 'Clear filter', onClearFilter));
    filterLineEl.hidden = false;
  }

  // onClearFilter turns the chip back to All and reads the list again. Focus
  // goes to the All chip, because the button that was pressed goes away.
  function onClearFilter() {
    Pinalert.dropFocus();
    Pinalert.setCategory('');
    renderChips(Pinalert.state);
    renderFilterLine(Pinalert.state);
    Pinalert.fetchReports();
    var all = chipsEl ? chipsEl.querySelector('.map-chip') : null;
    if (all) {
      all.focus();
    }
  }

  // onBackPress is Back to my location: the map goes back to the reader's
  // position, the place is dropped when that position (or the default area)
  // arrives, not before, and the list is read once (IN-07). When no position
  // can be had the line says so and shows the default area.
  function onBackPress() {
    if (backPending) {
      return;
    }
    backPending = true;
    // Leaving the searched place ends a report link (CR-01).
    Pinalert.dropFocus();
    locationLost = false;
    PinalertMap.recenterOnReader().then(function (result) {
      // A search or a pick made during the wait has already taken over.
      if (result && result.cancelled === true) {
        return;
      }
      backPending = false;
      locationLost = !(result && result.own === true);
      renderPlaceLine(Pinalert.state);
      focusSearch();
    });
  }

  // cancelBack drops a Back to my location that is still waiting, so its late
  // position is ignored and the press can be made again.
  function cancelBack() {
    if (!backPending) {
      return;
    }
    backPending = false;
    PinalertMap.cancelRecenter();
  }

  function onKeydown(evt) {
    if (evt.key !== 'Escape') {
      return;
    }
    // Escape also stops a typing pause or a search still out, even with
    // nothing on screen yet.
    if (typingTimer !== null || inFlight) {
      abortSearch();
    }
    if (!resultsEl.hidden || !statusEl.hidden) {
      abortSearch();
      closeResults();
      return;
    }
    if (isNarrow() && toolsEl.getAttribute('data-search-open') === 'true') {
      setOpen(false);
      openBtn.focus();
    }
  }

  // wired is true once the listeners are attached, so a second init is a no-op.
  var wired = false;

  function init() {
    if (wired || !toolsEl || !formEl || !inputEl || !resultsEl || !statusEl ||
        typeof Pinalert === 'undefined') {
      return;
    }
    wired = true;

    formEl.addEventListener('submit', function (evt) {
      evt.preventDefault();
      search();
    });

    openBtn.addEventListener('click', function () {
      setOpen(true);
      inputEl.focus();
    });

    closeBtn.addEventListener('click', function () {
      abortSearch();
      closeResults();
      setOpen(false);
      openBtn.focus();
    });

    resultsEl.addEventListener('click', function (evt) {
      var button = evt.target && evt.target.closest ? evt.target.closest('.map-search__result') : null;
      if (!button) {
        return;
      }
      pickResult(parseInt(button.getAttribute('data-result-index'), 10));
    });

    formEl.addEventListener('keydown', onKeydown);

    inputEl.addEventListener('input', onInput);

    // A press outside the search closes the results and the status line, and
    // stops a typing pause or a search still out.
    document.addEventListener('pointerdown', function (evt) {
      if ((!resultsEl.hidden || !statusEl.hidden || typingTimer !== null || inFlight) && !formEl.contains(evt.target)) {
        abortSearch();
        closeResults();
      }
    });

    if (chipsEl) {
      chipsEl.addEventListener('click', onChipPress);
    }

    // The filter empty block's Clear filter is wired by feed.js, which runs
    // first; this listener only redraws the chips and the line afterwards.
    var emptyClear = document.getElementById('filter-empty-clear');
    if (emptyClear) {
      emptyClear.addEventListener('click', function () {
        renderChips(Pinalert.state);
        renderFilterLine(Pinalert.state);
      });
    }

    Pinalert.subscribe(function (state) {
      renderChips(state);
      renderPlaceLine(state);
      renderFilterLine(state);
    });
    renderChips(Pinalert.state);
    renderPlaceLine(Pinalert.state);
    renderFilterLine(Pinalert.state);
  }

  init();

  return {
    init: init,
    search: search,
    pickResult: pickResult,
    closeResults: closeResults,
    renderChips: renderChips,
    renderPlaceLine: renderPlaceLine,
    renderFilterLine: renderFilterLine
  };
}());
