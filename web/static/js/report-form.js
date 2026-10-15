// report-form.js: the shared report form (plan 04-10, D-15).
//
// One script holds the form logic for every page that mounts it: the
// category builder, severity, description, shelter fields, validation, the
// payload, submit, the discard confirmation and the typed place search. It
// loads no map library and names none, so a page without a map can mount the
// same form with its own location object.
//
// Location interface. The page passes a location object to mount. Required
// methods:
//   current()                       { latitude, longitude } or null
//   choose(result)                  result is { lat, lon, displayName }
//   isSet()                         whether a location is set
//   missingMessage()                the sentence shown when none is set
//   searchFailureMessage(message)   the sentence shown when a search fails;
//                                   message is the server's own text or undefined
// Optional hooks, called only when present:
//   start(hooks)                    on every open, after the dialog is shown;
//                                   hooks.markTouched marks the form touched
//   reset()                         from the form teardown
//
// SECURITY (T-01-03, T-07-03): every string that reaches the DOM here,
// validation messages and geocoder text included, whether client or server
// authored, is inserted as text content through Pinalert.setText, never
// assembled into markup for the browser to parse. Place names and addresses
// from the geocoding proxy are third party data and follow the same rule.
//
// The in flight label on the submit button keeps its shipped ellipsis
// character (U6).
(function () {
  'use strict';

  var SEVERITY_BY_VALUE = { '1': 'low', '2': 'medium', '3': 'critical' };
  var SEVERITY_POSITIONS = ['1 · Low', '2 · Medium', '3 · Critical'];

  // Location search (D-01, D-03, D-04). SEARCH_DEBOUNCE_MS is a user
  // experience knob only, not the policy compliance control: the actual
  // one request per second ceiling D-02 requires is enforced by the
  // process wide limiter inside internal/geocode, so this number can be
  // tuned for feel without reasoning about compliance.
  var SEARCH_DEBOUNCE_MS = 600;
  // How long one place search may take before it counts as failed, in
  // milliseconds (U8, D-16). A search that runs this long gets the same
  // failure sentence as one the server refused.
  var SEARCH_TIMEOUT_MS = 8000;
  // Must stay equal to the server's own minimum (minGeocodeQueryRunes in
  // internal/api/handlers/geocode.go): a smaller value here would only
  // produce requests the server refuses with a 400.
  var MIN_QUERY_RUNES = 3;
  var SEARCH_NO_MATCH_MESSAGE = 'No matches found.';

  var mounted = false;
  var backdropEl = null;
  var formRoot = null;
  // The location object passed to mount. Named picker, never location, so
  // it cannot shadow window.location.
  var picker = null;
  var formVariant = 'main';

  var cancelButton = null;
  var submitButton = null;
  var categoryGrid = null;
  var severityInput = null;
  var severityReadout = null;
  var descriptionField = null;
  var formError = null;
  var locationSearchInput = null;
  var locationSearchResults = null;
  var locationSearchStatus = null;
  var shelterFields = null;
  var shelterCapacityStatus = null;
  var shelterHeadcount = null;
  var discardConfirm = null;
  var discardConfirmDiscard = null;
  var discardConfirmKeep = null;

  var selectedCategory = null;
  var lastFocusedEl = null;
  var severityWrapper = null;
  var formTouched = false;
  var preDiscardFocusEl = null;
  var searchDebounceTimer = null;
  var searchSeq = 0;
  // Session scoped and emptied by resetForm: both a Nominatim usage policy
  // ask (cache repeated queries) and what keeps a visitor retyping the
  // same place from firing duplicate requests.
  var searchCache = new Map();

  // buildSeverityControl wraps the existing native label, range input and
  // output in a single container (once, at mount) so a sev-low, sev-medium
  // or sev-critical class on that wrapper can drive --severity-current for
  // every descendant via CSS custom property inheritance: the slider fill,
  // the thumb border and the readout text all pick up the same severity
  // colour from one class toggle (D-05, D-11). The native range input itself
  // is left untouched: the browser already implements its correct arrow key,
  // Home and End, Page Up and Down and built in accessible slider
  // semantics, which a hand built widget would have to reimplement.
  function buildSeverityControl() {
    if (severityWrapper) {
      return;
    }
    var label = severityInput.previousElementSibling;
    var isLabel = label && label.tagName === 'LABEL';

    severityWrapper = document.createElement('div');
    severityWrapper.className = 'severity-control';
    severityInput.parentNode.insertBefore(severityWrapper, severityInput);

    if (isLabel) {
      severityWrapper.appendChild(label);
    }
    severityWrapper.appendChild(severityInput);
    severityWrapper.appendChild(severityReadout);

    // Three static position labels beneath the track so the scale is
    // legible before the visitor ever touches it. aria-hidden because the
    // live aria-valuetext announcement and #severity-readout already carry
    // the accessible value; these are a purely visual affordance.
    var positions = document.createElement('div');
    positions.className = 'severity-position-labels';
    positions.setAttribute('aria-hidden', 'true');
    SEVERITY_POSITIONS.forEach(function (text) {
      var span = document.createElement('span');
      Pinalert.setText(span, text);
      positions.appendChild(span);
    });
    severityWrapper.appendChild(positions);
  }

  // buildShelterCapacityOptions populates #shelter-capacity-status from
  // Pinalert.CAPACITY_STATUSES (available, limited, full, closed) rather
  // than relying on the template's static option list, so this select's
  // enum has exactly one source of truth (FOUND-06).
  function buildShelterCapacityOptions() {
    shelterCapacityStatus.textContent = '';
    Pinalert.CAPACITY_STATUSES.forEach(function (status) {
      var option = document.createElement('option');
      option.value = status;
      Pinalert.setText(option, status.charAt(0).toUpperCase() + status.slice(1));
      shelterCapacityStatus.appendChild(option);
    });
  }

  function markTouched() {
    formTouched = true;
  }

  // showShelterFields and hideShelterFields implement FOUND-06's category
  // gating: shelter_open reveals a required capacity status plus an
  // optional headcount; every other category hides and clears both so the
  // submit payload can omit the keys entirely (the server 400s if either
  // key is present on a non shelter report, precisely so a client bug here
  // surfaces immediately).
  function showShelterFields() {
    shelterFields.hidden = false;
    shelterCapacityStatus.required = true;
  }

  function hideShelterFields() {
    shelterFields.hidden = true;
    shelterCapacityStatus.required = false;
    shelterCapacityStatus.selectedIndex = 0;
    shelterHeadcount.value = '';
  }

  // focusField moves focus to the control implicated by a validation
  // failure, either the client's own validate() or a server field and
  // message error, whose field names already match these cases exactly.
  function focusField(field) {
    switch (field) {
      case 'category':
        var selectedTile = categoryGrid.querySelector('.category-tile--selected') ||
          categoryGrid.querySelector('.category-tile');
        if (selectedTile) {
          selectedTile.focus();
        }
        break;
      case 'severity':
        severityInput.focus();
        break;
      case 'description':
        descriptionField.focus();
        break;
      case 'shelter_capacity_status':
        shelterCapacityStatus.focus();
        break;
      case 'shelter_headcount':
        shelterHeadcount.focus();
        break;
      default:
        // 'location' (and any other field without a single focusable
        // control, for example latitude or longitude): nothing to focus.
        break;
    }
  }

  // Discard confirmation (D-03's only data loss path in this phase) reveals
  // #discard-confirm, "Discard this report? Your description and location
  // won't be saved.", only when a field has been touched; an untouched form
  // closes immediately.
  // Buttons: "Discard" (destructive) and "Keep editing".
  // The copy itself lives as static markup in report_form.html.tmpl; this
  // file only owns the show, hide and focus behaviour around it.
  function requestClose() {
    if (!discardConfirm.hidden) {
      return;
    }
    if (formTouched) {
      preDiscardFocusEl = document.activeElement;
      discardConfirm.hidden = false;
      discardConfirmDiscard.focus();
    } else {
      closeForm();
    }
  }

  function keepEditing() {
    discardConfirm.hidden = true;
    if (preDiscardFocusEl && typeof preDiscardFocusEl.focus === 'function') {
      preDiscardFocusEl.focus();
    }
    preDiscardFocusEl = null;
  }

  function confirmDiscard() {
    discardConfirm.hidden = true;
    closeForm();
  }

  // buildCategoryGrid renders exactly nine tiles in Pinalert.CATEGORIES
  // order, laid out 3x3 by modal.css's grid-template-columns. The grid is a
  // proper radio group: role="radio" and aria-checked on every tile, a
  // roving tabindex keeps exactly one tile in the tab order, and arrow keys
  // move the selection (D-01, D-04).
  function buildCategoryGrid() {
    categoryGrid.textContent = '';
    Pinalert.CATEGORIES.forEach(function (category, index) {
      var tile = document.createElement('button');
      tile.type = 'button';
      tile.className = 'category-tile';
      tile.setAttribute('role', 'radio');
      tile.setAttribute('aria-checked', 'false');
      tile.tabIndex = index === 0 ? 0 : -1;
      tile.dataset.category = category;

      var glyph = document.createElement('span');
      glyph.className = Pinalert.iconClass(category);
      glyph.setAttribute('aria-hidden', 'true');

      var label = document.createElement('span');
      Pinalert.setText(label, Pinalert.CATEGORY_LABELS[category] || category);

      tile.appendChild(glyph);
      tile.appendChild(label);
      tile.addEventListener('click', function () {
        selectCategory(category);
        tile.focus();
      });
      tile.addEventListener('keydown', onCategoryTileKeydown);

      categoryGrid.appendChild(tile);
    });
  }

  // onCategoryTileKeydown implements arrow key navigation across the 3x3
  // grid: Left and Right move by one, Up and Down move by a row (three),
  // wrapping at the edges. Moving focus also selects: this is a single
  // select radio group, not a two step focus then activate control.
  function onCategoryTileKeydown(e) {
    var tiles = Array.prototype.slice.call(categoryGrid.querySelectorAll('.category-tile'));
    var currentIndex = tiles.indexOf(e.currentTarget);
    var nextIndex = null;

    switch (e.key) {
      case 'ArrowRight':
        nextIndex = (currentIndex + 1) % tiles.length;
        break;
      case 'ArrowLeft':
        nextIndex = (currentIndex - 1 + tiles.length) % tiles.length;
        break;
      case 'ArrowDown':
        nextIndex = (currentIndex + 3) % tiles.length;
        break;
      case 'ArrowUp':
        nextIndex = (currentIndex - 3 + tiles.length) % tiles.length;
        break;
      default:
        return;
    }

    e.preventDefault();
    var nextTile = tiles[nextIndex];
    selectCategory(nextTile.dataset.category);
    nextTile.focus();
  }

  // selectCategory updates the mutually exclusive tile state, the roving
  // tabindex, and fires a category-change custom event whenever the
  // selection actually changes so the shelter capacity fieldset can react
  // without this function knowing anything about shelter fields.
  function selectCategory(category) {
    var changed = selectedCategory !== category;
    selectedCategory = category;
    var tiles = categoryGrid.querySelectorAll('.category-tile');
    for (var i = 0; i < tiles.length; i++) {
      var tile = tiles[i];
      var isSelected = tile.dataset.category === category;
      tile.classList.toggle('category-tile--selected', isSelected);
      tile.setAttribute('aria-checked', isSelected ? 'true' : 'false');
      tile.tabIndex = isSelected ? 0 : -1;
    }
    if (changed) {
      categoryGrid.dispatchEvent(new CustomEvent('category-change', { detail: { category: category } }));
    }
  }

  function updateSeverityReadout() {
    var severityKey = SEVERITY_BY_VALUE[severityInput.value];
    var label = Pinalert.SEVERITY_LABELS[severityKey];
    // aria-valuetext carries the number plus word form to a screen reader;
    // an announcement of only "2" does not convey "Medium" (D-05).
    severityInput.setAttribute('aria-valuetext', label);
    Pinalert.setText(severityReadout, label);
    if (severityWrapper) {
      severityWrapper.classList.remove('sev-low', 'sev-medium', 'sev-critical');
      severityWrapper.classList.add('sev-' + severityKey);
    }
  }

  // showSearchStatus and clearSearchStatus are the only two functions that
  // touch #location-search-status, carrying D-04's no match and
  // unavailable inline messages.
  function showSearchStatus(message) {
    Pinalert.setText(locationSearchStatus, message);
    locationSearchStatus.hidden = false;
  }

  function clearSearchStatus() {
    Pinalert.setText(locationSearchStatus, '');
    locationSearchStatus.hidden = true;
  }

  // hideDropdown empties the suggestion container's children, not only its
  // hidden attribute. Emptying the children is required, not cosmetic:
  // trapTab collects focusables from the form root with a query for
  // buttons, links, inputs, selects, text areas and tabindex elements, so
  // a leftover result button inside a hidden container would enter the tab
  // cycle and silently swallow a focus call, and a stale row must not be
  // revealed by a later hidden clear before fresh results replace it.
  function hideDropdown() {
    locationSearchResults.textContent = '';
    locationSearchResults.hidden = true;
    locationSearchInput.setAttribute('aria-expanded', 'false');
  }

  // renderResultRow builds one suggestion row entirely from created
  // elements and Pinalert.setText insertions (T-07-03): no markup string is
  // ever assembled here. Nominatim's name field is jsonv2 specific and not
  // always populated, so the fallback collapses to a single line rather
  // than rendering an empty primary row. Tapping a row hands the chosen
  // place to the location object through choose, which owns the one pin
  // placement path for its page, so there is no extra confirmation step
  // between the tap and the placement (D-03) and this file never places a
  // pin itself.
  function renderResultRow(result) {
    var row = document.createElement('button');
    row.type = 'button';
    row.className = 'location-search-result';

    var primary = document.createElement('span');
    Pinalert.setText(primary, result.name || result.display_name);
    row.appendChild(primary);

    if (result.name) {
      var secondary = document.createElement('span');
      secondary.className = 'location-search-result-secondary';
      Pinalert.setText(secondary, result.display_name);
      row.appendChild(secondary);
    }

    row.addEventListener('click', function () {
      markTouched();
      picker.choose({ lat: result.lat, lon: result.lon, displayName: result.display_name });
      clearSearchStatus();
      hideDropdown();
    });

    return row;
  }

  // renderDropdown renders at most the results the server already
  // returned, in the order received: the proxy returns Nominatim's own
  // relevance order, and re-sorting client side risks disagreeing with
  // that ranking for no benefit.
  function renderDropdown(results) {
    if (!results || results.length === 0) {
      hideDropdown();
      showSearchStatus(SEARCH_NO_MATCH_MESSAGE);
      noteSearchOutcome('no_match');
      return;
    }
    clearSearchStatus();
    locationSearchResults.textContent = '';
    results.forEach(function (result) {
      locationSearchResults.appendChild(renderResultRow(result));
    });
    locationSearchResults.hidden = false;
    locationSearchInput.setAttribute('aria-expanded', 'true');
    noteSearchOutcome('match');
  }

  // noteSearchOutcome tells the location object how the last search ended
  // ('match', 'no_match' or 'unavailable') when it asks to know. The page
  // without a map uses it to tell that both ways of placing a report failed.
  function noteSearchOutcome(outcome) {
    if (picker && typeof picker.noteSearchOutcome === 'function') {
      picker.noteSearchOutcome(outcome);
    }
  }

  // searchTimeoutSignal returns a signal that aborts the search request after
  // SEARCH_TIMEOUT_MS, by the browser's own timeout signal when it has one,
  // else by a timer. It returns undefined when neither can be built, and the
  // request then simply has no time limit.
  function searchTimeoutSignal() {
    if (typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
      return AbortSignal.timeout(SEARCH_TIMEOUT_MS);
    }
    if (typeof AbortController === 'function') {
      var controller = new AbortController();
      window.setTimeout(function () {
        controller.abort();
      }, SEARCH_TIMEOUT_MS);
      return controller.signal;
    }
    return undefined;
  }

  // onSearchInput is a pause based debounce: the timer is cleared and
  // rescheduled on every keystroke, so one request is sent per typing
  // pause rather than one per keystroke (D-01, D-02). Query length is
  // measured in code points, not UTF-16 code units, so the client's
  // minimum agrees with the server's rune based minimum for a Devanagari
  // or other multibyte query.
  function onSearchInput() {
    markTouched();
    window.clearTimeout(searchDebounceTimer);
    var query = locationSearchInput.value.trim();
    if (Array.from(query).length < MIN_QUERY_RUNES) {
      hideDropdown();
      clearSearchStatus();
      return;
    }
    searchDebounceTimer = window.setTimeout(function () {
      runSearch(query);
    }, SEARCH_DEBOUNCE_MS);
  }

  // runSearch mirrors votes.js's castVote error idiom exactly. The
  // searchSeq sequence guard is load bearing, not defensive: without it a
  // slow response for an earlier query can arrive last and overwrite the
  // suggestions for the query the visitor is actually looking at. This
  // function must never touch submitButton, never detach a page's own
  // listeners, and never disable the search input, because every one of
  // those would violate D-04. On failure the page's own location object
  // decides the sentence, given the server's message when there is one.
  function runSearch(query) {
    // Incremented unconditionally, before the cache check: a cache hit
    // render must invalidate any older in flight request the same way a new
    // request would, or a slow response for an earlier (non cached) query
    // can still land after this cache hit render and silently overwrite it.
    searchSeq += 1;
    var mySeq = searchSeq;

    var cacheKey = query.toLowerCase();
    if (searchCache.has(cacheKey)) {
      renderDropdown(searchCache.get(cacheKey));
      return;
    }

    fetch('/api/geocode?q=' + encodeURIComponent(query), { signal: searchTimeoutSignal() })
      .then(function (res) {
        return res.json().catch(function () {
          return {};
        }).then(function (body) {
          if (!res.ok) {
            var fieldError = (body && body.error) || {};
            // The Error text is never shown; the catch below reads
            // fieldMessage, which stays undefined when the server sent none.
            var err = new Error(fieldError.message || 'Search request failed');
            err.fieldMessage = fieldError.message;
            throw err;
          }
          if (mySeq !== searchSeq) {
            return;
          }
          searchCache.set(cacheKey, body.results);
          renderDropdown(body.results);
        });
      })
      .catch(function (err) {
        if (mySeq !== searchSeq) {
          return;
        }
        hideDropdown();
        // The page without a map ignores the server's text and shows its own
        // sentence; the main form keeps the server's message (U8). A timeout
        // or a network error carries no server text on either page.
        var serverText = formVariant === 'low' ? '' : (err && err.fieldMessage);
        showSearchStatus(picker.searchFailureMessage(serverText));
        noteSearchOutcome('unavailable');
      });
  }

  // resetSearch clears every piece of search state. All six pieces are
  // required, not optional: resetForm is reached from closeForm, so
  // leaving any of them out lets the previous submission's dropdown,
  // message, or cached results appear in the next one.
  function resetSearch() {
    window.clearTimeout(searchDebounceTimer);
    searchDebounceTimer = null;
    // Discards any in flight response on arrival rather than letting it
    // resolve into a closed or reopened form.
    searchSeq += 1;
    locationSearchInput.value = '';
    hideDropdown();
    clearSearchStatus();
    searchCache.clear();
  }

  // resetForm remains the single teardown path for every piece of the
  // form's state, search included; no parallel teardown function is
  // introduced. The location object clears its own state through its
  // optional reset hook.
  function resetForm() {
    selectedCategory = null;
    var tiles = categoryGrid.querySelectorAll('.category-tile');
    for (var i = 0; i < tiles.length; i++) {
      tiles[i].classList.remove('category-tile--selected');
      tiles[i].setAttribute('aria-checked', 'false');
      tiles[i].tabIndex = i === 0 ? 0 : -1;
    }
    severityInput.value = '1';
    updateSeverityReadout();
    descriptionField.value = '';
    Pinalert.setText(formError, '');
    if (picker && typeof picker.reset === 'function') {
      picker.reset();
    }
    hideShelterFields();
    resetSearch();
    discardConfirm.hidden = true;
    formTouched = false;
    preDiscardFocusEl = null;
  }

  // mount wires the form to a page. options.backdrop is the dialog backdrop
  // (hidden until opened), options.form is the dialog panel that holds the
  // fields, options.location is the location object described in the header
  // comment, and options.variant names the page flavour (default 'main').
  // It runs once; a second call or a call with a missing option does
  // nothing. Every listener is registered here, once, and never on open, so
  // open and close cycles cannot stack duplicate handlers.
  function mount(options) {
    if (mounted) {
      return;
    }
    if (!options || !options.backdrop || !options.form || !options.location) {
      return;
    }
    backdropEl = options.backdrop;
    formRoot = options.form;
    picker = options.location;
    formVariant = options.variant || 'main';

    cancelButton = document.getElementById('cancel-report');
    submitButton = document.getElementById('submit-report');
    categoryGrid = document.getElementById('category-grid');
    severityInput = document.getElementById('severity');
    severityReadout = document.getElementById('severity-readout');
    descriptionField = document.getElementById('description');
    formError = document.getElementById('form-error');
    locationSearchInput = document.getElementById('location-search');
    locationSearchResults = document.getElementById('location-search-results');
    locationSearchStatus = document.getElementById('location-search-status');
    shelterFields = document.getElementById('shelter-fields');
    shelterCapacityStatus = document.getElementById('shelter-capacity-status');
    shelterHeadcount = document.getElementById('shelter-headcount');
    discardConfirm = document.getElementById('discard-confirm');
    discardConfirmDiscard = document.getElementById('discard-confirm-discard');
    discardConfirmKeep = document.getElementById('discard-confirm-keep');

    mounted = true;

    buildSeverityControl();
    buildShelterCapacityOptions();
    updateSeverityReadout();

    categoryGrid.addEventListener('category-change', function (e) {
      markTouched();
      if (e.detail.category === 'shelter_open') {
        showShelterFields();
      } else {
        hideShelterFields();
      }
    });

    cancelButton.addEventListener('click', requestClose);
    submitButton.addEventListener('click', handleSubmit);
    locationSearchInput.addEventListener('input', onSearchInput);
    severityInput.addEventListener('input', updateSeverityReadout);
    severityInput.addEventListener('input', markTouched);
    descriptionField.addEventListener('input', markTouched);
    descriptionField.addEventListener('blur', validateDescriptionOnBlur);
    shelterCapacityStatus.addEventListener('change', markTouched);
    shelterHeadcount.addEventListener('input', markTouched);
    discardConfirmDiscard.addEventListener('click', confirmDiscard);
    discardConfirmKeep.addEventListener('click', keepEditing);
    backdropEl.addEventListener('click', function (e) {
      if (e.target === backdropEl) {
        requestClose();
      }
    });
  }

  // openForm shows the form. options.opener is the element that gets focus
  // back on close; without one, the element focused at open time is used.
  function openForm(options) {
    if (!mounted) {
      return;
    }
    lastFocusedEl = (options && options.opener) || document.activeElement;
    backdropEl.hidden = false;
    Pinalert.lockPage(backdropEl);
    buildCategoryGrid();
    updateSeverityReadout();
    if (typeof picker.start === 'function') {
      picker.start({ markTouched: markTouched });
    }

    var firstTile = categoryGrid.querySelector('.category-tile');
    if (firstTile) {
      firstTile.focus();
    }

    document.addEventListener('keydown', onKeydown);
  }

  function closeForm() {
    backdropEl.hidden = true;
    Pinalert.unlockPage();
    resetForm();
    document.removeEventListener('keydown', onKeydown);
    if (lastFocusedEl) {
      lastFocusedEl.focus();
    }
  }

  function onKeydown(e) {
    if (e.key === 'Escape') {
      if (!discardConfirm.hidden) {
        keepEditing();
      } else {
        requestClose();
      }
      return;
    }
    if (e.key === 'Tab') {
      trapTab(e);
    }
  }

  function trapTab(e) {
    var candidates = formRoot.querySelectorAll(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    // A control inside a hidden part of the panel cannot take focus, so it is
    // never the end the cycle wraps at.
    var focusable = [];
    for (var i = 0; i < candidates.length; i++) {
      if (!candidates[i].closest('[hidden]')) {
        focusable.push(candidates[i]);
      }
    }
    if (focusable.length === 0) {
      return;
    }
    var first = focusable[0];
    var last = focusable[focusable.length - 1];

    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  // validate mirrors the server's rules for usability only: the server
  // (internal/service.ValidateSubmitInput) remains the sole authority and
  // revalidates everything (T-01-21); bypassing this client side check
  // gains nothing. Every message is verbatim from the Copywriting
  // Contract. Severity is checked defensively even though the native range
  // input always carries a value 1 to 3, mirroring how the server treats it
  // as a real validation rule. The location check asks the location object.
  function validate() {
    if (!selectedCategory) {
      return { field: 'category', message: 'Choose a category to continue.' };
    }
    if (['1', '2', '3'].indexOf(severityInput.value) === -1) {
      return { field: 'severity', message: 'Pick a severity level.' };
    }
    var description = descriptionField.value.trim();
    if (description.length < 10) {
      return { field: 'description', message: 'Add a short description (at least 10 characters).' };
    }
    if (!picker.isSet()) {
      return { field: 'location', message: picker.missingMessage() };
    }
    if (selectedCategory === 'shelter_open' && !shelterCapacityStatus.value) {
      return { field: 'shelter_capacity_status', message: 'Choose a shelter capacity status.' };
    }
    return null;
  }

  // validateDescriptionOnBlur is the one on blur usability check (the rest
  // of the Copywriting Contract's messages validate on submit only, since
  // category, severity and location are not meaningfully blurred).
  function validateDescriptionOnBlur() {
    var description = descriptionField.value.trim();
    if (description.length > 0 && description.length < 10) {
      Pinalert.setText(formError, 'Add a short description (at least 10 characters).');
    }
  }

  // buildPayload omits shelter_capacity_status and shelter_headcount
  // entirely for every non shelter category (FOUND-06): the server 400s if
  // either key is present on a non shelter report, precisely so a client bug
  // here surfaces immediately instead of writing meaningless capacity data.
  function buildPayload() {
    var pos = picker.current();
    var payload = {
      category: selectedCategory,
      severity: SEVERITY_BY_VALUE[severityInput.value],
      description: descriptionField.value.trim(),
      latitude: pos.latitude,
      longitude: pos.longitude
    };
    if (selectedCategory === 'shelter_open') {
      payload.shelter_capacity_status = shelterCapacityStatus.value;
      var headcountValue = shelterHeadcount.value.trim();
      if (headcountValue !== '') {
        payload.shelter_headcount = parseInt(headcountValue, 10);
      }
    }
    return payload;
  }

  function handleSubmit() {
    var validationResult = validate();
    if (validationResult) {
      Pinalert.setText(formError, validationResult.message);
      focusField(validationResult.field);
      return;
    }

    var payload = buildPayload();

    submitButton.disabled = true;
    Pinalert.setText(submitButton, 'Posting…');
    Pinalert.setText(formError, '');

    Pinalert.submitReport(payload).then(function () {
      submitButton.disabled = false;
      Pinalert.setText(submitButton, 'Post report');
      closeForm();
      Pinalert.showToast('Report posted.');
    }).catch(function (err) {
      submitButton.disabled = false;
      Pinalert.setText(submitButton, 'Post report');
      // The visitor's input stays intact: the form does not close and no
      // field is cleared on a failed submission.
      Pinalert.setText(formError, (err && err.fieldMessage) || 'Something went wrong. Try again.');
      if (err && err.field) {
        focusField(err.field);
      }
    });
  }

  window.PinalertReportForm = { mount: mount, open: openForm, close: closeForm };
}());
