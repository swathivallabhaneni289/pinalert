// map.js — window.PinalertMap: Leaflet init, badge pins, safe popups.
//
// This module performs no fetch of its own — app.js owns the one nearby-
// reports fetch. map.js only subscribes to the shared store and reconciles
// Leaflet markers against Pinalert.state.reports.
//
// SECURITY (T-01-03, T-01-17): marker badge markup is assembled from fixed
// strings and Pinalert.iconPath's validated path only — no report-authored
// value ever appears in it. Popups are built as DOM elements (never a
// markup string) and passed to Leaflet's bindPopup/setPopupContent, which
// appendChild()s an Element instead of using innerHTML when given one.
window.PinalertMap = (function () {
  'use strict';

  var map = null;
  var markers = {}; // report id -> L.Marker

  // hasVectorBasemap reports whether this browser can render the
  // OpenFreeMap vector basemap through the MapLibre bridge: both vendor
  // globals must have loaded, and the browser must grant a WebGL2
  // rendering context, which is the version the renderer actually targets
  // — a WebGL1-only probe would let a device through that still cannot
  // paint. When any of that fails, the caller falls back to the raster
  // layer instead.
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

  function init() {
    var container = document.getElementById('map');
    if (!container || typeof L === 'undefined') {
      return;
    }

    // maxBounds/maxBoundsViscosity/minZoom are the vector bridge
    // maintainers' own boilerplate, copied verbatim from their published
    // example — maxBounds' inverted-looking lat/lng ordering prevents a
    // documented renderer bug and must not be "corrected". maxZoom is NOT
    // part of that boilerplate: it moved onto the map because the vector
    // layer extends bare Leaflet's own layer base and registers no zoom
    // limit of its own, which would otherwise leave pinch-zoom unbounded.
    map = L.map(container, {
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
      }).addTo(map);
    } else {
      var rasterLayer = L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        detectRetina: true,
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
      });
      rasterLayer.addTo(map);
      // Re-derive the map's own maximum zoom from the fallback layer's
      // post-construction value. Leaflet's high-density-display branch can
      // decrement that value below the map's fixed ceiling above, and the
      // grid layer renders no tiles above its own maximum — so without
      // this line a fallback visitor on such a display could reach a zoom
      // at which the map goes blank. Reading it back off the layer gets
      // this right on every display with no hardcoded number.
      map.setMaxZoom(rasterLayer.options.maxZoom);
    }

    Pinalert.subscribe(render);
    Pinalert.onSelect(function (id, source) {
      if (source === 'list') {
        flyTo(id);
      }
    });

    centerOnVisitor();
  }

  // centerOnVisitor centres on the visitor's geolocation when granted, and
  // on Pinalert.config's fallback otherwise, then hands the resolved centre
  // to the shared store so the one fetch + polling loop can start.
  function centerOnVisitor() {
    var fallbackLat = Pinalert.config.fallbackLat;
    var fallbackLon = Pinalert.config.fallbackLon;

    function boot(lat, lon, zoom) {
      map.setView([lat, lon], zoom);
      Pinalert.setCenter(lat, lon);
      Pinalert.fetchReports();
      Pinalert.startPolling();
    }

    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        function (pos) {
          boot(pos.coords.latitude, pos.coords.longitude, 13);
        },
        function () {
          boot(fallbackLat, fallbackLon, 11);
        },
        { timeout: 8000 }
      );
    } else {
      boot(fallbackLat, fallbackLon, 11);
    }
  }

  // buildBadgeElement assembles a white category glyph on a solid
  // severity-coloured circular badge (D-11, D-12), desaturating toward gray
  // as the report ages (D-17). Fixed classes/strings plus the validated
  // icon path only.
  function buildBadgeElement(report) {
    var sevClass = Pinalert.severityClass(report);
    var ageClass = 'age-' + Pinalert.ageStage(report);

    var badge = document.createElement('span');
    badge.className = 'icon-badge icon-badge--pin ' + sevClass + ' ' + ageClass;

    var img = document.createElement('img');
    img.src = Pinalert.iconPath(report.category);
    img.alt = '';
    badge.appendChild(img);

    return badge;
  }

  // buildPopupContent constructs the popup as DOM nodes: category label,
  // severity label, relative time, and the description set via
  // Pinalert.setText — never a markup string.
  function buildPopupContent(report) {
    var wrap = document.createElement('div');
    wrap.className = 'map-popup';

    var meta = document.createElement('p');
    meta.className = 'map-popup__meta';
    var categoryLabel = Pinalert.CATEGORY_LABELS[report.category] || report.category;
    var severityLabel = Pinalert.SEVERITY_LABELS[report.severity] || report.severity;
    Pinalert.setText(meta, categoryLabel + ' · ' + severityLabel + ' · ' + Pinalert.relativeTime(report.created_at));

    var description = document.createElement('p');
    description.className = 'map-popup__description';
    Pinalert.setText(description, report.description);

    wrap.appendChild(meta);
    wrap.appendChild(description);
    return wrap;
  }

  function upsertMarker(report) {
    var icon = L.divIcon({
      html: buildBadgeElement(report),
      className: '',
      iconSize: [44, 44]
    });

    var existing = markers[report.id];
    if (existing) {
      existing.setLatLng([report.latitude, report.longitude]);
      existing.setIcon(icon);
      existing.setPopupContent(buildPopupContent(report));
      return;
    }

    var marker = L.marker([report.latitude, report.longitude], { icon: icon }).addTo(map);
    marker.bindPopup(buildPopupContent(report));
    marker.on('click', function () {
      Pinalert.select(report.id, 'map');
    });
    markers[report.id] = marker;
  }

  // render reconciles markers by report id rather than clearing and
  // rebuilding the layer on every poll, which would drop an open popup
  // every 30 seconds.
  function render(state) {
    if (!map) {
      return;
    }

    var seenIds = {};
    for (var i = 0; i < state.reports.length; i++) {
      var report = state.reports[i];
      seenIds[report.id] = true;
      upsertMarker(report);
    }

    var existingIds = Object.keys(markers);
    for (var j = 0; j < existingIds.length; j++) {
      var id = existingIds[j];
      if (!seenIds[id]) {
        map.removeLayer(markers[id]);
        delete markers[id];
      }
    }
  }

  // flyTo pans to a report at zoom 16 and opens its popup — the wide-screen
  // "clicking a list row pans/flies the map" half of D-06's sync contract.
  function flyTo(id) {
    var marker = markers[id];
    if (!marker || !map) {
      return;
    }
    map.flyTo(marker.getLatLng(), 16);
    marker.openPopup();
  }

  // highlight briefly rings the pin at id — plan 01-06 calls this the other
  // direction (clicking a pin scrolls/rings the matching list row).
  function highlight(id) {
    var marker = markers[id];
    if (!marker) {
      return;
    }
    var el = marker.getElement();
    if (!el) {
      return;
    }
    el.classList.add('map-pin--highlight');
    window.setTimeout(function () {
      el.classList.remove('map-pin--highlight');
    }, 1500);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  return {
    flyTo: flyTo,
    highlight: highlight
  };
}());
