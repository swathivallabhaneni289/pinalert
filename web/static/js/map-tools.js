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
//   - Nothing is sent while the reader types. The box is read on submit only,
//     at least 3 letters, and a second submit during a request is ignored, so
//     the geocoder is never loaded by keystrokes (D-14, T-31-51).
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

  // abortSearch drops the request in flight, if any, and invalidates its
  // answer.
  function abortSearch() {
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
  function renderResults(list) {
    resultsEl.textContent = '';
    lastResults = [];
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
      resultsEl.hidden = true;
      showStatus(MESSAGE_NO_MATCH, false);
      return;
    }
    resultsEl.hidden = false;
    var count = lastResults.length;
    showStatus(count === 1 ? '1 result.' : String(count) + ' results.', true);
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

  // search runs one place search. It sends nothing for fewer than 3 letters,
  // nothing while an earlier search is still in flight, and drops an answer
  // that is no longer the newest. A search that goes out while Back to my
  // location is still waiting cancels it: the latest action wins (IN-07).
  function search(query) {
    var q = String(query === undefined ? inputEl.value : query).trim();
    if (inFlight) {
      return;
    }
    if (Array.from(q).length < MIN_QUERY_RUNES) {
      closeResults();
      showStatus(MESSAGE_TOO_SHORT, false);
      return;
    }
    cancelBack();
    closeResults();
    searchSeq += 1;
    var mySeq = searchSeq;
    inFlight = true;
    showStatus(MESSAGE_SEARCHING, false);

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
            showStatus(messageFor(res.status, body), false);
            return;
          }
          renderResults(body && Array.isArray(body.results) ? body.results : []);
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
        showStatus(MESSAGE_UNAVAILABLE, false);
      });
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

    // A press outside the search closes the results and the status line.
    document.addEventListener('pointerdown', function (evt) {
      if ((!resultsEl.hidden || !statusEl.hidden) && !formEl.contains(evt.target)) {
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
