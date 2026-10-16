// modal.js: the main page's Leaflet location picker for the shared report
// form (report-form.js, plan 04-10).
//
// The form logic lives in report-form.js and reads location only through a
// small interface. This file is the Leaflet implementation of that interface.
// It keeps the pieces that need a map: the modal map, the one draggable
// marker, tap to place in every location state, the coordinate readout, the
// main form's one missing location sentence and the main form's rule of
// showing the server's own search failure message, or the fallback below
// when the server sent none. It then mounts the form and opens it from the report button.
//
// SECURITY (T-01-03, T-07-03): every string that reaches the DOM here is
// inserted as text content, never assembled into markup for the browser to
// parse. The coordinate readout goes through Pinalert.setText like every
// other string, so third party place text stays inert.
(function () {
  'use strict';

  var modal = document.getElementById('report-modal');
  var fabButton = document.getElementById('fab-report');
  var coordReadout = document.getElementById('coord-readout');
  var locationNotice = document.getElementById('location-notice');
  var modalMapEl = document.getElementById('modal-map');

  if (!modal || !fabButton) {
    return;
  }

  var panel = modal.querySelector('.modal-panel');

  var modalMap = null;
  var modalMarker = null;
  // The hooks the form hands to start, kept so a pin move can mark the form
  // touched.
  var touchHooks = null;

  // Same zoom initLocation's GPS success callback uses, so a searched
  // location and a GPS location arrive at identical map state.
  var SEARCH_RESULT_ZOOM = 16;
  // Byte identical to geocodeUnavailableMessage in
  // internal/api/handlers/geocode.go, on purpose, so the visitor reads one
  // wording whether the failure came from upstream or from this browser's
  // own fetch.
  var SEARCH_UNAVAILABLE_MESSAGE = 'Search unavailable, try tapping the map instead.';

  // markFormTouched tells the form a pin move counts as an edit.
  function markFormTouched() {
    if (touchHooks && typeof touchHooks.markTouched === 'function') {
      touchHooks.markTouched();
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
      markFormTouched();
      var pos = modalMarker.getLatLng();
      updateCoordReadout(pos.lat, pos.lng);
    });
    updateCoordReadout(lat, lon);
  }

  // placeMarkerAtTap moves the pin to the tapped spot. It does what a drag
  // does: the form counts it as an edit and the readout shows the new place.
  // It runs whether location is on, off or denied (backlog 999.6, F1 in
  // 02-UAT.md). A tap on the pin itself never reaches the map, because a
  // Leaflet marker does not pass its own clicks on, and Leaflet sends no
  // click after the map or the pin was dragged.
  function placeMarkerAtTap(e) {
    markFormTouched();
    placeMarker(e.latlng.lat, e.latlng.lng);
  }

  // hasVectorBasemap reports whether this browser can render the
  // OpenFreeMap vector basemap through the MapLibre bridge: both vendor
  // globals must have loaded, and the browser must grant a WebGL2
  // rendering context, which is the version the renderer actually targets
  // — a WebGL1-only probe would let a device through that still cannot
  // paint. When any of that fails, the caller falls back to the raster
  // layer instead. A private copy of the primary map's own probe, by this
  // codebase's established convention of duplicating these small pieces
  // per file rather than sharing them.
  //
  // This probe is the only safety net for renderer failures in this file.
  // The vector construction below is deliberately NOT wrapped in a second
  // try/catch: Leaflet defers a layer's add hook until the map's first
  // view is set, so a renderer failure would throw asynchronously from
  // that later call, outside any try/catch placed around the construction
  // itself.
  function hasVectorBasemap() {
    if (typeof L.maplibreGL !== 'function') {
      return false;
    }
    if (typeof maplibregl === 'undefined') {
      return false;
    }
    try {
      return !!document.createElement('canvas').getContext('webgl2');
    } catch (e) {
      return false;
    }
  }

  // initLocation centres the modal's own Leaflet instance on the visitor's
  // GPS position at zoom 16 with a draggable marker (D-02). Submission is
  // never blocked on geolocation: a denied prompt during an emergency must
  // not become a dead end, so denial, timeout or an insecure context falls
  // back to the configured centre. A tap places or moves the pin in every
  // case; its handler is added once, when the map is built (IN-W10).
  function initLocation() {
    // Mirrors map.js's init() guard: if Leaflet's own script failed to
    // load, L is undefined and every call below would throw an uncaught
    // ReferenceError instead of degrading gracefully. Without Leaflet
    // there is no way to show a location picker at all, so bail out
    // before touching modalMap or scheduling the geolocation callback.
    if (typeof L === 'undefined') {
      return;
    }
    if (!modalMap) {
      // Same maintainer boilerplate and the same maxZoom correction as the
      // primary map, duplicated here per this file's own convention. No
      // deferral, no animation-frame wrapper, and no visibility check is
      // needed around the branch below: Leaflet's addLayer routes through
      // its own ready mechanism, so the vector layer's add hook does not
      // run until this map's first view is set, which happens inside the
      // async geolocation callback further down, by which point the modal
      // is already open and its container has real dimensions.
      modalMap = L.map(modalMapEl, {
        maxBounds: [[180, -Infinity], [-180, Infinity]],
        maxBoundsViscosity: 1,
        minZoom: 1,
        maxZoom: 19
      });

      if (hasVectorBasemap()) {
        L.maplibreGL({
          style: 'https://tiles.openfreemap.org/styles/liberty',
          attributionControl: {
            customAttribution:
              '<a href="https://openfreemap.org" target="_blank" rel="noopener">OpenFreeMap</a> ' +
              '<a href="https://www.openmaptiles.org/" target="_blank" rel="noopener">&copy; OpenMapTiles</a> ' +
              'Data from <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a>'
          }
        }).addTo(modalMap);
      } else {
        var rasterLayer = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
          maxZoom: 19,
          detectRetina: true,
          attribution: '&copy; OpenStreetMap contributors'
        });
        rasterLayer.addTo(modalMap);
        // Re-derive the map's own maximum zoom from the fallback layer's
        // post-construction value — same reasoning as the primary map: a
        // high-density display can lower it below the map's fixed
        // ceiling above, and the grid layer renders no tiles beyond its
        // own maximum.
        modalMap.setMaxZoom(rasterLayer.options.maxZoom);
      }

      // Added here, once per map, so a tap moves the pin in every location
      // state and a reopen never stacks a second handler (IN-W10).
      modalMap.on('click', placeMarkerAtTap);
    }

    locationNotice.hidden = true;

    var fallbackLat = Pinalert.config.fallbackLat;
    var fallbackLon = Pinalert.config.fallbackLon;

    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        function (pos) {
          // A place chosen before the position answered (a saved report's
          // location or a typed search result) is kept (D-20).
          if (hasMarker()) {
            return;
          }
          var lat = pos.coords.latitude;
          var lon = pos.coords.longitude;
          modalMap.setView([lat, lon], 16);
          placeMarker(lat, lon);
          window.setTimeout(function () {
            modalMap.invalidateSize();
          }, 0);
        },
        function () {
          if (hasMarker()) {
            return;
          }
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
  }

  // startLocation runs on every open of the form, after the dialog is shown.
  function startLocation(hooks) {
    touchHooks = hooks || null;
    initLocation();
  }

  // clearMarker is the form teardown hook: it removes the pin and clears the
  // coordinate readout.
  function clearMarker() {
    if (modalMarker) {
      modalMap.removeLayer(modalMarker);
      modalMarker = null;
    }
    Pinalert.setText(coordReadout, '');
  }

  // currentPosition reports the pin position, or null when no pin is set.
  function currentPosition() {
    if (!modalMarker) {
      return null;
    }
    var pos = modalMarker.getLatLng();
    return { latitude: pos.lat, longitude: pos.lng };
  }

  function hasMarker() {
    return !!modalMarker;
  }

  // chooseSearchResult is called by the form when a suggestion is tapped. It
  // calls the existing placeMarker and modalMap.setView, the same pair
  // initLocation's GPS success callback calls, so there is exactly one pin
  // placement code path in this file and no extra confirmation step between
  // the tap and the placement (D-03). The pin placeMarker creates is already
  // draggable with its own dragend handler, so it stays adjustable afterward
  // with no extra work.
  function chooseSearchResult(result) {
    placeMarker(result.lat, result.lon);
    modalMap.setView([result.lat, result.lon], SEARCH_RESULT_ZOOM);
  }

  // missingLocationMessage shows only when no pin is set, so it names a tap,
  // the way to make one, rather than a drag, which needs a pin already there.
  function missingLocationMessage() {
    return 'Set a location by searching, tapping the map, or allowing location access.';
  }

  function searchUnavailableMessage(serverMessage) {
    return serverMessage || SEARCH_UNAVAILABLE_MESSAGE;
  }

  var leafletLocation = {
    current: currentPosition,
    choose: chooseSearchResult,
    isSet: hasMarker,
    missingMessage: missingLocationMessage,
    searchFailureMessage: searchUnavailableMessage,
    start: startLocation,
    reset: clearMarker
  };

  // If the form script failed to load, the page keeps working without the
  // form, as initLocation's own missing library guard does for the map.
  if (!window.PinalertReportForm) {
    return;
  }

  window.PinalertReportForm.mount({
    backdrop: modal,
    form: panel,
    location: leafletLocation,
    variant: 'main'
  });

  fabButton.addEventListener('click', function () {
    window.PinalertReportForm.open({ opener: fabButton });
  });
}());
