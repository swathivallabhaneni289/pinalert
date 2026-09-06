// modal.js — the production report submission modal (plan 01-05).
//
// Replaces plan 01-04's thin stand-in wholesale. Category selection is a
// proper 3x3 radio-group (D-01, D-04), location is a GPS-prefilled
// draggable marker with tap-to-place fallback (D-02), severity is the
// native accessible range input (D-05, D-11), and submission covers the
// full shelter-capacity/validation/discard contract (FOUND-06).
//
// SECURITY (T-01-03): every string that reaches the DOM here — validation
// messages included, whether client- or server-authored — is inserted as
// text content, never assembled into markup for the browser to parse.
(function () {
  'use strict';

  var modal = document.getElementById('report-modal');
  var fabButton = document.getElementById('fab-report');
  var cancelButton = document.getElementById('cancel-report');
  var submitButton = document.getElementById('submit-report');
  var categoryGrid = document.getElementById('category-grid');
  var severityInput = document.getElementById('severity');
  var severityReadout = document.getElementById('severity-readout');
  var descriptionField = document.getElementById('description');
  var formError = document.getElementById('form-error');
  var coordReadout = document.getElementById('coord-readout');
  var locationNotice = document.getElementById('location-notice');
  var toast = document.getElementById('toast');
  var modalMapEl = document.getElementById('modal-map');
  var shelterFields = document.getElementById('shelter-fields');
  var shelterCapacityStatus = document.getElementById('shelter-capacity-status');
  var shelterHeadcount = document.getElementById('shelter-headcount');
  var discardConfirm = document.getElementById('discard-confirm');
  var discardConfirmDiscard = document.getElementById('discard-confirm-discard');
  var discardConfirmKeep = document.getElementById('discard-confirm-keep');

  if (!modal || !fabButton) {
    return;
  }

  var modalMap = null;
  var modalMarker = null;
  var selectedCategory = null;
  var lastFocusedEl = null;
  var toastTimer = null;
  var severityWrapper = null;
  var formTouched = false;
  var preDiscardFocusEl = null;

  var SEVERITY_BY_VALUE = { '1': 'low', '2': 'medium', '3': 'critical' };
  var SEVERITY_POSITIONS = ['1 · Low', '2 · Medium', '3 · Critical'];

  // buildSeverityControl wraps the existing native <label>/<input
  // type="range">/<output> in a single container (once, at init) so a
  // sev-low/sev-medium/sev-critical class on that wrapper can drive
  // --severity-current for every descendant via CSS custom-property
  // inheritance — the slider fill, the thumb border, and the readout text
  // all pick up the same traffic-light colour from one class toggle (D-05,
  // D-11). The native range input itself is left untouched: the browser
  // already implements its correct arrow-key, Home/End, Page-Up/Down and
  // built-in accessible-slider semantics, which a hand-built widget would
  // have to reimplement.
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
    // the accessible value — these are a purely visual affordance.
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
  // than relying on the template's static <option> list, so this select's
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

  // showShelterFields / hideShelterFields implement FOUND-06's category
  // gating: shelter_open reveals a required capacity status plus an
  // optional headcount; every other category hides and clears both so the
  // submit payload can omit the keys entirely (the server 400s if either
  // key is present on a non-shelter report, precisely so a client bug here
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
  // failure — either the client's own validate() or a server {field,
  // message} error, whose field names already match these cases exactly.
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
        // control, e.g. latitude/longitude) — nothing to focus.
        break;
    }
  }

  // Discard confirmation (D-03's only data-loss path in this phase) reveals
  // #discard-confirm — "Discard this report? Your description and location
  // won't be saved." — only when a field has been touched; an untouched
  // modal closes immediately.
  // Buttons: "Discard" (destructive) / "Keep editing".
  // The copy itself lives as static markup in index.html.tmpl; this file
  // only owns the show/hide/focus behaviour around it.
  function requestClose() {
    if (!discardConfirm.hidden) {
      return;
    }
    if (formTouched) {
      preDiscardFocusEl = document.activeElement;
      discardConfirm.hidden = false;
      discardConfirmDiscard.focus();
    } else {
      closeModal();
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
    closeModal();
  }

  // buildCategoryGrid renders exactly nine tiles in Pinalert.CATEGORIES
  // order, laid out 3x3 by modal.css's grid-template-columns. The grid is a
  // proper radio group: role="radio"/aria-checked on every tile, a roving
  // tabindex keeps exactly one tile in the tab order, and arrow keys move
  // the selection (D-01, D-04).
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

      var img = document.createElement('img');
      img.src = Pinalert.iconPath(category);
      img.alt = '';
      img.setAttribute('aria-hidden', 'true');

      var label = document.createElement('span');
      Pinalert.setText(label, Pinalert.CATEGORY_LABELS[category] || category);

      tile.appendChild(img);
      tile.appendChild(label);
      tile.addEventListener('click', function () {
        selectCategory(category);
        tile.focus();
      });
      tile.addEventListener('keydown', onCategoryTileKeydown);

      categoryGrid.appendChild(tile);
    });
  }

  // onCategoryTileKeydown implements arrow-key navigation across the 3x3
  // grid: Left/Right move by one, Up/Down move by a row (three), wrapping
  // at the edges. Moving focus also selects — this is a single-select radio
  // group, not a two-step focus-then-activate control.
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

  // selectCategory updates the mutually-exclusive tile state, the roving
  // tabindex, and fires a category-change custom event whenever the
  // selection actually changes so Task 3's shelter-capacity fieldset can
  // react without this function knowing anything about shelter fields.
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
    // aria-valuetext carries the number-plus-word form to a screen reader —
    // an announcement of only "2" does not convey "Medium" (D-05).
    severityInput.setAttribute('aria-valuetext', label);
    Pinalert.setText(severityReadout, label);
    if (severityWrapper) {
      severityWrapper.classList.remove('sev-low', 'sev-medium', 'sev-critical');
      severityWrapper.classList.add('sev-' + severityKey);
    }
  }

  function updateCoordReadout(lat, lon) {
    Pinalert.setText(coordReadout, lat.toFixed(5) + ', ' + lon.toFixed(5));
  }

  function placeMarker(lat, lon) {
    if (modalMarker) {
      modalMap.removeLayer(modalMarker);
    }
    modalMarker = L.marker([lat, lon], { draggable: true }).addTo(modalMap);
    modalMarker.on('dragend', function () {
      markTouched();
      var pos = modalMarker.getLatLng();
      updateCoordReadout(pos.lat, pos.lng);
    });
    updateCoordReadout(lat, lon);
  }

  // initLocation centres the modal's own Leaflet instance on the visitor's
  // GPS position at zoom 16 with a draggable marker (D-02). Submission is
  // never blocked on geolocation — a denied prompt during an emergency must
  // not become a dead end, so denial/timeout/insecure-context falls back to
  // the configured centre plus tap-to-place.
  function initLocation() {
    if (!modalMap) {
      modalMap = L.map(modalMapEl);
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        detectRetina: true,
        attribution: '&copy; OpenStreetMap contributors'
      }).addTo(modalMap);
    }

    locationNotice.hidden = true;

    var fallbackLat = Pinalert.config.fallbackLat;
    var fallbackLon = Pinalert.config.fallbackLon;

    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        function (pos) {
          var lat = pos.coords.latitude;
          var lon = pos.coords.longitude;
          modalMap.setView([lat, lon], 16);
          placeMarker(lat, lon);
          window.setTimeout(function () {
            modalMap.invalidateSize();
          }, 0);
        },
        function () {
          showLocationDenied(fallbackLat, fallbackLon);
        },
        { timeout: 8000 }
      );
    } else {
      showLocationDenied(fallbackLat, fallbackLon);
    }
  }

  function showLocationDenied(fallbackLat, fallbackLon) {
    locationNotice.hidden = false;
    modalMap.setView([fallbackLat, fallbackLon], 11);
    window.setTimeout(function () {
      modalMap.invalidateSize();
    }, 0);
    modalMap.on('click', function (e) {
      markTouched();
      placeMarker(e.latlng.lat, e.latlng.lng);
    });
  }

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
    if (modalMarker) {
      modalMap.removeLayer(modalMarker);
      modalMarker = null;
    }
    Pinalert.setText(coordReadout, '');
    hideShelterFields();
    discardConfirm.hidden = true;
    formTouched = false;
    preDiscardFocusEl = null;
  }

  function openModal() {
    lastFocusedEl = document.activeElement;
    modal.hidden = false;
    buildCategoryGrid();
    updateSeverityReadout();
    initLocation();

    var firstTile = categoryGrid.querySelector('.category-tile');
    if (firstTile) {
      firstTile.focus();
    }

    document.addEventListener('keydown', onKeydown);
  }

  function closeModal() {
    modal.hidden = true;
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
    var focusable = modal.querySelectorAll(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
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

  function showToast(message) {
    Pinalert.setText(toast, message);
    toast.hidden = false;
    if (toastTimer) {
      window.clearTimeout(toastTimer);
    }
    toastTimer = window.setTimeout(function () {
      toast.hidden = true;
    }, 3000);
  }

  // validate mirrors the server's rules for usability only — the server
  // (internal/service.ValidateSubmitInput) remains the sole authority and
  // revalidates everything (T-01-21); bypassing this client-side check
  // gains nothing. Every message is verbatim from the Copywriting
  // Contract. Severity is checked defensively even though the native range
  // input always carries a value 1-3, mirroring how the server treats it
  // as a real validation rule.
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
    if (!modalMarker) {
      return { field: 'location', message: 'Set a location by dragging the pin or allowing location access.' };
    }
    if (selectedCategory === 'shelter_open' && !shelterCapacityStatus.value) {
      return { field: 'shelter_capacity_status', message: 'Choose a shelter capacity status.' };
    }
    return null;
  }

  // validateDescriptionOnBlur is the one on-blur usability check (the rest
  // of the Copywriting Contract's messages validate on submit only, since
  // category/severity/location aren't meaningfully "blurred").
  function validateDescriptionOnBlur() {
    var description = descriptionField.value.trim();
    if (description.length > 0 && description.length < 10) {
      Pinalert.setText(formError, 'Add a short description (at least 10 characters).');
    }
  }

  // buildPayload omits shelter_capacity_status/shelter_headcount entirely
  // for every non-shelter category (FOUND-06) — the server 400s if either
  // key is present on a non-shelter report, precisely so a client bug here
  // surfaces immediately instead of writing meaningless capacity data.
  function buildPayload() {
    var pos = modalMarker.getLatLng();
    var payload = {
      category: selectedCategory,
      severity: SEVERITY_BY_VALUE[severityInput.value],
      description: descriptionField.value.trim(),
      latitude: pos.lat,
      longitude: pos.lng
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
      closeModal();
      showToast('Report posted.');
    }).catch(function (err) {
      submitButton.disabled = false;
      Pinalert.setText(submitButton, 'Post report');
      // The visitor's input stays intact — the modal does not close and no
      // field is cleared on a failed submission.
      Pinalert.setText(formError, (err && err.fieldMessage) || 'Something went wrong. Try again.');
      if (err && err.field) {
        focusField(err.field);
      }
    });
  }

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

  fabButton.addEventListener('click', openModal);
  cancelButton.addEventListener('click', requestClose);
  submitButton.addEventListener('click', handleSubmit);
  severityInput.addEventListener('input', updateSeverityReadout);
  severityInput.addEventListener('input', markTouched);
  descriptionField.addEventListener('input', markTouched);
  descriptionField.addEventListener('blur', validateDescriptionOnBlur);
  shelterCapacityStatus.addEventListener('change', markTouched);
  shelterHeadcount.addEventListener('input', markTouched);
  discardConfirmDiscard.addEventListener('click', confirmDiscard);
  discardConfirmKeep.addEventListener('click', keepEditing);
  modal.addEventListener('click', function (e) {
    if (e.target === modal) {
      requestClose();
    }
  });
}());
