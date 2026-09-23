// map.js — window.PinalertMap: Leaflet init, badge pins, safe popups.
//
// This module performs no fetch of its own — app.js owns the one nearby-
// reports fetch. map.js only subscribes to the shared store and reconciles
// Leaflet markers against Pinalert.state.reports.
//
// SECURITY (T-01-03, T-01-17): marker badge markup is assembled from fixed
// strings and Pinalert.iconClass's validated class name only — no report-
// authored value ever appears in it. Popups are built as DOM elements
// (never a markup string) and passed to Leaflet's bindPopup/setPopupContent,
// which appendChild()s an Element instead of using innerHTML when given one.
// The popup now carries interactive controls too, still assembled as DOM
// elements and handed to that same Leaflet API, so adding buttons changes
// nothing about that guarantee. The popup and the pin badge now also carry
// a server-supplied state value, validated against a fixed list before it
// is reflected into a class name or an attribute — the same discipline the
// category value has always been under here. The popup's own OUTER Leaflet
// container now carries that same set of three validated state classes too
// (severity, age, visibility), the same values buildBadgeElement already
// puts on the pin badge, so no new class of value ever reaches the DOM.
window.PinalertMap = (function () {
  'use strict';

  var map = null;
  var markers = {}; // report id -> L.Marker

  // replacePrefixedClass is a deliberate third copy of feed.js's and
  // visibility.js's private helper of the same name and behaviour, not an
  // extraction: feed.js's copy is private to its own IIFE, map.js had none
  // of its own, and visibility.js's own comment already names this helper
  // by name as the pattern map for swapping these classes. Strips every
  // class beginning with prefix off el, then appends newClass.
  function replacePrefixedClass(el, prefix, newClass) {
    var kept = [];
    var current = el.className ? el.className.split(/\s+/) : [];
    for (var i = 0; i < current.length; i++) {
      if (current[i] && current[i].indexOf(prefix) !== 0) {
        kept.push(current[i]);
      }
    }
    kept.push(newClass);
    el.className = kept.join(' ');
  }

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
  // icon class only. The glyph is a span carrying Pinalert.iconClass's
  // mask-based class pair (see main.css's .icon-glyph rules) rather than a
  // replaced-element image, so its color resolves from this badge's own
  // `color` through the ordinary CSS cascade.
  function buildBadgeElement(report) {
    var sevClass = Pinalert.severityClass(report);
    var ageClass = 'age-' + Pinalert.ageStage(report);

    var badge = document.createElement('span');
    badge.className = 'icon-badge icon-badge--pin ' + sevClass + ' ' + ageClass;

    // On this surface the state class goes on the badge element itself,
    // because there is no row ancestor — the same split main.css's own
    // comment documents for the age ramp, and precisely why the
    // stylesheet's hidden-badge rule carries both the compound and the
    // descendant selector form.
    PinalertVisibility.applyVisibilityClass(badge, report);

    var glyph = document.createElement('span');
    glyph.className = Pinalert.iconClass(report.category);
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);

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

    // Unlike the feed row, this surface builds and updates the tag in one
    // call: upsertMarker rebuilds the popup's content on every render
    // rather than updating it in place.
    var tag = PinalertVisibility.createVisibilityTag();
    PinalertVisibility.updateVisibilityTag(tag, report);
    wrap.appendChild(tag);

    // The same vote controls block the feed row mounts, from the same
    // builder — what makes the shared confirm/dispute mechanic
    // structural rather than a convention two call sites have to
    // remember. Unlike the feed row, this surface builds and updates in
    // one call: upsertMarker rebuilds the popup's content on every
    // render rather than updating it in place, an accepted limitation
    // votes.js's own click orchestration documents.
    var block = PinalertVotes.createVoteBlock(report.id);
    PinalertVotes.updateVoteBlock(block, report);
    wrap.appendChild(block.controls);
    wrap.appendChild(block.error);

    return wrap;
  }

  // popupStateClasses returns the space joined triple of state classes this
  // report's popup container must carry: severity, age and visibility, in
  // that order. All three are already validated against fixed allowlists by
  // their own modules (Pinalert.severityClass, Pinalert.ageStage,
  // PinalertVisibility.visibilityState), so no report-authored value ever
  // reaches this class name, the same discipline buildBadgeElement is
  // already under. The age class is included for parity with the feed
  // row's own class list even though no age rule touches --severity-tint,
  // so carrying it cannot introduce a fade-with-age behaviour on the popup
  // tint, and nothing inside .map-popup consumes --severity-current or
  // --badge-glyph-fg today either.
  function popupStateClasses(report) {
    return Pinalert.severityClass(report) + ' age-' + Pinalert.ageStage(report) +
      ' vis-' + PinalertVisibility.visibilityState(report);
  }

  // syncPopupState keeps a marker's popup container carrying the current
  // state classes, without ever rebinding or unbinding the popup itself.
  // render()'s own comment states that dropping an open popup a reader is
  // looking at is exactly the failure its reconcile-by-id loop exists to
  // prevent, and unbinding to pass a fresh className would do exactly that
  // on every 30 second poll. Returns early when the marker carries no
  // popup yet.
  //
  // Two things happen here, and both are needed, not either alone. First,
  // it writes the className option directly onto the popup instance. This
  // is the only path that works for a popup that has never been opened:
  // Leaflet reads options.className exactly once, inside its own layout
  // step on first open, and never again, so a popup opened for the first
  // time after this call still paints with the right classes even though
  // the container does not exist yet. Second, it reads the popup's already
  // built container element, Leaflet's own documented accessor for it, and
  // if one exists (the popup has been opened at least once), it swaps the
  // three prefixed classes on that live element in place: the sev- and
  // age- prefixes through this file's own replacePrefixedClass, and the
  // vis- prefix through PinalertVisibility's own class helper, so visibility
  // state stays owned by the module that validates it. Swapping by prefix
  // rather than replacing the whole class list is what makes this safe on
  // an element Leaflet also owns: it leaves leaflet-popup,
  // leaflet-zoom-animated, leaflet-interactive and any other Leaflet class
  // completely untouched.
  function syncPopupState(marker, report) {
    var popup = marker.getPopup();
    if (!popup) {
      return;
    }

    popup.options.className = popupStateClasses(report);

    var el = popup.getElement();
    if (!el) {
      return;
    }
    replacePrefixedClass(el, 'sev-', Pinalert.severityClass(report));
    replacePrefixedClass(el, 'age-', 'age-' + Pinalert.ageStage(report));
    PinalertVisibility.applyVisibilityClass(el, report);
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
      syncPopupState(existing, report);
      return;
    }

    var marker = L.marker([report.latitude, report.longitude], { icon: icon }).addTo(map);
    marker.bindPopup(buildPopupContent(report), { className: popupStateClasses(report) });
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
