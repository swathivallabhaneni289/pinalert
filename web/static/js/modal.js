// modal.js — the thinnest submission modal that genuinely works end to end.
//
// Plan 01-05 replaces this file wholesale with the full UI-SPEC version
// (3x3 icon grid layout, animated slider ramp, discard confirmation,
// shelter fields). This file deliberately does not build those — it uses
// the same DOM contract ids (#category-grid, #discard-confirm,
// #shelter-fields) as plain/minimal stand-ins so the Walking Skeleton is
// real and demonstrable one wave earlier.
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

  if (!modal || !fabButton) {
    return;
  }

  var modalMap = null;
  var modalMarker = null;
  var selectedCategory = null;
  var lastFocusedEl = null;
  var toastTimer = null;

  var SEVERITY_BY_VALUE = { '1': 'low', '2': 'medium', '3': 'critical' };

  function buildCategoryGrid() {
    categoryGrid.textContent = '';
    Pinalert.CATEGORIES.forEach(function (category) {
      var tile = document.createElement('button');
      tile.type = 'button';
      tile.className = 'category-tile';
      tile.setAttribute('role', 'radio');
      tile.setAttribute('aria-checked', 'false');
      tile.dataset.category = category;

      var img = document.createElement('img');
      img.src = Pinalert.iconPath(category);
      img.alt = '';

      var label = document.createElement('span');
      Pinalert.setText(label, Pinalert.CATEGORY_LABELS[category] || category);

      tile.appendChild(img);
      tile.appendChild(label);
      tile.addEventListener('click', function () {
        selectCategory(category);
      });

      categoryGrid.appendChild(tile);
    });
  }

  function selectCategory(category) {
    selectedCategory = category;
    var tiles = categoryGrid.querySelectorAll('.category-tile');
    for (var i = 0; i < tiles.length; i++) {
      var tile = tiles[i];
      var isSelected = tile.dataset.category === category;
      tile.classList.toggle('category-tile--selected', isSelected);
      tile.setAttribute('aria-checked', isSelected ? 'true' : 'false');
    }
  }

  function updateSeverityReadout() {
    var label = Pinalert.SEVERITY_LABELS[SEVERITY_BY_VALUE[severityInput.value]];
    severityInput.setAttribute('aria-valuetext', label);
    Pinalert.setText(severityReadout, label);
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
      var pos = modalMarker.getLatLng();
      updateCoordReadout(pos.lat, pos.lng);
    });
    updateCoordReadout(lat, lon);
  }

  function initLocation() {
    if (!modalMap) {
      modalMap = L.map(modalMapEl);
      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
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
      placeMarker(e.latlng.lat, e.latlng.lng);
    });
  }

  function resetForm() {
    selectedCategory = null;
    var tiles = categoryGrid.querySelectorAll('.category-tile');
    for (var i = 0; i < tiles.length; i++) {
      tiles[i].classList.remove('category-tile--selected');
      tiles[i].setAttribute('aria-checked', 'false');
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
      closeModal();
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

  function validate() {
    if (!selectedCategory) {
      return 'Choose a category to continue.';
    }
    var description = descriptionField.value.trim();
    if (description.length < 10) {
      return 'Add a short description (at least 10 characters).';
    }
    if (!modalMarker) {
      return 'Set a location by dragging the pin or allowing location access.';
    }
    return null;
  }

  function handleSubmit() {
    var validationMessage = validate();
    if (validationMessage) {
      Pinalert.setText(formError, validationMessage);
      return;
    }

    var pos = modalMarker.getLatLng();
    var payload = {
      category: selectedCategory,
      severity: SEVERITY_BY_VALUE[severityInput.value],
      description: descriptionField.value.trim(),
      latitude: pos.lat,
      longitude: pos.lng
    };

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
      Pinalert.setText(formError, (err && err.fieldMessage) || 'Something went wrong. Try again.');
    });
  }

  fabButton.addEventListener('click', openModal);
  cancelButton.addEventListener('click', closeModal);
  submitButton.addEventListener('click', handleSubmit);
  severityInput.addEventListener('input', updateSeverityReadout);
}());
