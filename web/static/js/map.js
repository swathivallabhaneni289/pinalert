// map.js — window.PinalertMap: Leaflet init, badge pins, safe popups.
//
// This module performs no fetch of its own — app.js owns the one nearby-
// reports fetch. map.js only subscribes to the shared store and reconciles
// Leaflet markers against Pinalert.state.reports.
//
// SECURITY (T-01-03, T-01-17, T-03-06): marker badge markup is assembled from
// fixed strings and Pinalert.iconClass's validated class name only, with no
// report-authored value anywhere in it. Each popup is built exactly once per
// marker as a tree of DOM elements (never a markup string) and handed to
// Leaflet as an Element. Every 30 second poll then calls updatePopupCard,
// which writes text, classes and attributes on the existing nodes, so an open
// popup, an open confirmation and an open trust panel survive a poll. Strings
// reach the card only through Pinalert.setText and class names only through
// validated helpers. The pin badge and the popup container carry a
// server-supplied state value, validated against a fixed list before it is
// reflected into a class name or an attribute, the same discipline the
// category value has always been under here. The popup's own outer Leaflet
// container carries the same set of three validated state classes (severity,
// age, visibility) that buildBadgeElement puts on the pin badge, so no new
// class of value ever reaches the DOM.
window.PinalertMap = (function () {
  'use strict';

  var map = null;
  var markers = {}; // report id -> L.Marker
  var cards = {}; // report id -> popup card (see buildPopupCard)
  var popupTitleSeq = 0;

  // A finished Rescue report is approximate (03.1 D-23): the server sends a
  // point about a kilometre off and an empty description, and the map must not
  // suggest more precision than that. No code driven zoom for such a pin goes
  // past this level, and no circle is drawn. Live reports keep zoom 16.
  var MAX_APPROX_ZOOM = 14;
  // The fixed words of a finished Rescue report (C17, C18, C19). The title
  // takes the place of the blank description; the label takes the shelter
  // line's slot; the privacy line sits under it.
  var FINISHED_RESCUE_TITLE = 'Rescue needed';
  var APPROX_LABEL = 'Approximate location';
  var PRIVACY_LINE = 'Details of finished rescue requests are hidden to protect privacy.';

  // The report id of a /?report=ID link whose popup this script has opened, so
  // a later poll or a tab change never opens it a second time.
  var openedFocusId = null;

  // Numbers every Back to my location and every cancel of one, so a position
  // that arrives after a newer search or pick is ignored (IN-07).
  var recenterSeq = 0;

  // The open Leaflet popup, or null.
  var activePopup = null;
  // True while focus sits inside the open popup. Kept by focusin and focusout
  // because Leaflet can remove the container before popupclose fires.
  var popupFocusInside = false;
  // True from a press on the popup close button, made while focus was inside
  // the popup, until that popup closes. WebKit moves focus to the map
  // container on that press, which clears popupFocusInside first (A74).
  var closePressHadFocus = false;
  // Who opened the open popup, as { id, kind } with kind 'list' or 'marker' (A74).
  var popupOpener = null;
  // The report id a list row asked flyTo to open, consumed by the next popupopen.
  var listOpenerId = null;
  // True when the popup opened while its pane was hidden, so it was measured empty (A59).
  var openedWhileHidden = false;
  // True when a poll removed the report whose popup held focus. feed.js reads
  // and clears it through takeRemovedPopupFocus (A67).
  var removedPopupFocus = false;
  // True only while render removes a marker, so the popup close does not also
  // pull focus back to a pin that is gone.
  var removingPopup = false;
  // True only while a resize that crossed 900px closes the open popup, so that
  // close does not end a report link: the reader did not close it, and the
  // report is still in the set.
  var resizeClosing = false;
  // Reduced motion is read once. It turns off the map's fade and zoom
  // animations, the animated fly and Leaflet's own autopan (A60, A68).
  var reducedMotion = mediaMatches('(prefers-reduced-motion: reduce)');
  // The two media conditions trust.css switches the popup max height at (A65).
  // NARROW_QUERY is also the condition phone.css lays the report detail out as
  // a full screen sheet at, so the pane choice below follows the stylesheet.
  var NARROW_QUERY = '(max-width: 899.98px)';
  var SHORT_QUERY = '(max-height: 359px)';
  // The last answer to NARROW_QUERY, so a resize that crosses 900px closes the
  // open popup and one that does not (an address bar, a keyboard) leaves it.
  var lastNarrow = false;
  // Autopan paddings in pixels, derived from the spacing tokens (A60).
  var panPad = { topLeft: [0, 0], bottomRight: [0, 0] };
  // How many tools row corrections ran in a row for one popup, so a pan the
  // map's bounds undo is not tried for ever (keepCardClearOfTools).
  var TOOLS_TRIES = 3;
  var toolsTried = { popup: null, count: 0 };
  // True only while keepCardClearOfTools pans. A pan with no animation fires
  // moveend before it returns, and that nested check would see the card clear
  // and reset the count, so the cap above could never be reached.
  var clearingTools = false;

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

  // mediaMatches says whether a media query holds right now.
  function mediaMatches(query) {
    return !!(window.matchMedia && window.matchMedia(query).matches);
  }

  // readSpaceToken reads one spacing token from the root element as a pixel
  // number, or 0 when it is not a number.
  function readSpaceToken(name) {
    var value = parseFloat(window.getComputedStyle(document.documentElement).getPropertyValue(name));
    return isNaN(value) ? 0 : value;
  }

  // refreshPanPadding derives the autopan paddings from the same tokens and
  // media conditions as the popup max height in trust.css (A60, A65). Below
  // 900px the card must clear the account trigger at the top and the column of
  // floating buttons at the bottom; elsewhere, and on a very short screen,
  // every side is one medium space.
  function refreshPanPadding() {
    var spaceMd = readSpaceToken('--space-md');
    var spaceLg = readSpaceToken('--space-lg');
    var touchMin = readSpaceToken('--touch-target-min');
    var top = spaceMd;
    var bottom = spaceMd;
    if (mediaMatches(NARROW_QUERY) && !mediaMatches(SHORT_QUERY)) {
      top = spaceMd + touchMin + spaceMd;
      bottom = spaceLg + touchMin + spaceMd;
      // The account trigger hangs under the unofficial bar, so measure how far
      // down the map container its bottom edge reaches and clear that (P4-A4).
      // The sum above stays as the fallback when either box has no size.
      var header = document.querySelector('.app-header');
      if (map && header) {
        var headerRect = header.getBoundingClientRect();
        var mapRect = map.getContainer().getBoundingClientRect();
        if (headerRect.height > 0 && mapRect.height > 0) {
          top = Math.max(spaceMd, headerRect.bottom - mapRect.top + spaceMd);
        }
      }
    }
    // The tools layer sits over the top of the map, so the card must also
    // clear it (03.1 P4-A4). The larger of the two values wins.
    top = Math.max(top, toolsReserve());
    panPad = { topLeft: [spaceMd, top], bottomRight: [spaceMd, bottom] };
  }

  // toolsReserve is how far down from the top of the map container the card
  // must stay to clear the tools layer: the measured bottom of .map-tools plus
  // one small space. It is measured, never a literal, because the layer is one
  // row on a phone and two on a laptop. It is 0 when there is no layer or when
  // the layer is not drawn (the phone detail sheet hides it).
  function toolsReserve() {
    var container = document.getElementById('map');
    var tools = document.querySelector('.map-tools');
    if (!container || !tools) {
      return 0;
    }
    var toolsBox = tools.getBoundingClientRect();
    if (toolsBox.width === 0 && toolsBox.height === 0) {
      return 0;
    }
    var gap = readSpaceToken('--space-sm');
    return Math.max(0, toolsBox.bottom - container.getBoundingClientRect().top + gap);
  }

  // applyPanPadding writes the current paddings onto one popup, because a popup
  // reads its options when it opens and the viewport may have changed since.
  function applyPanPadding(popup) {
    if (!popup) {
      return;
    }
    popup.options.autoPanPaddingTopLeft = panPad.topLeft;
    popup.options.autoPanPaddingBottomRight = panPad.bottomRight;
  }

  // phoneDetail says whether the report detail opens as the full screen sheet
  // of phone.css: the same condition the stylesheet switches at.
  function phoneDetail() {
    return mediaMatches(NARROW_QUERY);
  }

  // applyPopupPlacement writes where one popup opens and whether Leaflet pans
  // for it. A popup reads both options every time it opens, and the viewport
  // may have crossed 900px since it was bound. On a phone the popup opens in
  // the detail pane, which sits outside the map pane's transform and which
  // phone.css lays out as a sheet, so Leaflet's own autopan must be off before
  // the open: it runs inside the open, ahead of the popupopen event.
  function applyPopupPlacement(popup, card) {
    if (!popup) {
      return;
    }
    var phone = phoneDetail();
    popup.options.pane = phone ? 'detailPane' : 'popupPane';
    popup.options.autoPan = !reducedMotion && !phone;
    arrangeCard(card, phone);
  }

  // arrangeCard puts the fade line and the actions (Confirm and Dispute, Mark
  // resolved, Flag) right under the report's title and meta line on a phone,
  // before the trust details, so the buttons show without scrolling (owner,
  // 2026-10-21: "make sure this confirm and dispute buttons are more
  // visible"). The laptop popup keeps its order (owner: do not disturb the
  // laptop). The nodes move rather than being reordered by CSS, so the reading
  // and tab order stay the same as what is seen.
  function arrangeCard(card, phone) {
    if (!card || !card.trust || !card.trust.el || !card.fade || !card.actions) {
      return;
    }
    var body = card.fade.parentNode;
    var trust = card.trust.el;
    var fade = card.fade;
    var actions = card.actions;
    if (!body || trust.parentNode !== body || actions.parentNode !== body) {
      return;
    }
    var flag = card.flag && card.flag.el ? card.flag.el : null;
    // Mute and its error line sit just above Flag at the bottom of the report
    // on both sizes, two calm rows (plan 04-30, owner, 2026-10-22). Mute
    // comes before Flag in the tab order wherever they are.
    var mute = card.mute && card.mute.el ? card.mute : null;
    var lastParent = phone ? body : actions;
    if (phone) {
      body.insertBefore(fade, trust);
      body.insertBefore(actions, trust);
    } else {
      body.insertBefore(trust, fade);
    }
    if (mute) {
      lastParent.appendChild(mute.el);
      if (mute.error) {
        lastParent.appendChild(mute.error);
      }
    }
    // Flag goes last: under the trust details on a phone (owner, 2026-10-21:
    // "the flag the report thing can go down"), and as the last child of the
    // actions block on a laptop.
    if (flag) {
      lastParent.appendChild(flag);
    }
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

  // LAPTOP_SETTLE_MS is how long the map box must hold one size before the map
  // is told about it.
  var LAPTOP_SETTLE_MS = 150;

  // watchMapBox tells the map its new size once the laptop layout has settled
  // (Phase 3.4 plan 20, D-03). On a laptop the map is the right column of the
  // grid, so the window, the 900px breakpoint, the top bar or a strip line can
  // each change the size of its box; Leaflet's own window listener does not see
  // all of those. A ResizeObserver on the map element waits for it to hold one
  // size for LAPTOP_SETTLE_MS and then measures once, with no animation and no
  // pan, never on every frame. The phone keeps its own settle path (sheet.js,
  // resizeToSheet), whose height ease would otherwise fire this on each frame,
  // so a phone width does nothing here. Browsers without ResizeObserver keep
  // Leaflet's window listener alone.
  function watchMapBox(container) {
    if (typeof ResizeObserver !== 'function') {
      return;
    }
    var timer = 0;
    new ResizeObserver(function () {
      if (timer) {
        window.clearTimeout(timer);
      }
      timer = window.setTimeout(function () {
        timer = 0;
        if (map && !phoneDetail()) {
          map.invalidateSize({ animate: false, pan: false });
        }
      }, LAPTOP_SETTLE_MS);
    }).observe(container);
  }

  function init() {
    var container = document.getElementById('map');
    if (!container || typeof L === 'undefined') {
      return;
    }

    refreshPanPadding();

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
      maxZoom: 19,
      fadeAnimation: !reducedMotion,
      markerZoomAnimation: !reducedMotion,
      zoomAnimation: !reducedMotion
    });

    // The report detail pane lives in its own page level host, not inside the
    // map container: Safari on the iPhone clips a fixed position popup to the
    // map container's overflow box, which cut the report view off at the map's
    // bottom edge and hid Back and Report (iPhone 17 Pro simulator, 2026-10-21;
    // desktop WebKit does not clip). The host keeps the leaflet-container class
    // so every existing rule still matches, and phone.css gives it no size and
    // no clipping. Presses and wheel turns on the sheet stop here, the same
    // protection Leaflet gives its own popup container, so they never reach the
    // map under it.
    var detailHost = document.createElement('div');
    detailHost.className = 'leaflet-container report-view-host';
    document.body.appendChild(detailHost);
    var detailPane = map.createPane('detailPane', detailHost);
    L.DomEvent.disableClickPropagation(detailPane);
    L.DomEvent.disableScrollPropagation(detailPane);
    lastNarrow = phoneDetail();

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

    map.on('popupopen', onPopupOpen);
    map.on('popupclose', onPopupClose);
    map.on('resize', onMapResize);
    map.on('moveend', onMapMoveEnd);
    container.addEventListener('focusin', onMapFocusIn);
    container.addEventListener('focusout', onMapFocusOut);
    container.addEventListener('pointerdown', onMapPointerDown);
    container.addEventListener('keydown', onMapKeydown);
    watchMapBox(container);
    // The browser's back and forward close and open a report (plan 39).
    window.addEventListener('popstate', onReportPopState);
    wireReportGone();

    Pinalert.subscribe(render);
    Pinalert.onSelect(function (id, source) {
      applyChosenPin(id);
      if (source === 'list') {
        // The phone map view to come back to, before the fly moves it (plan 39).
        if (!activePopup) {
          rememberNearbyView();
        }
        flyTo(id);
      }
    });

    centerOnVisitor();
  }

  // centerOnVisitor centres on the visitor's geolocation when granted, and
  // on Pinalert.config's fallback otherwise, then hands the resolved centre
  // to the shared store so the one fetch + polling loop can start. The own
  // flag tells the store whether the centre is the reader's own position, so
  // the popup shows a distance only when it is the reader's (A94).
  function centerOnVisitor() {
    var fallbackLat = Pinalert.config.fallbackLat;
    var fallbackLon = Pinalert.config.fallbackLon;
    var fallbackZoom = Pinalert.config.fallbackZoom;
    // reusing is true while the map starts from the kept fix: that fix is not a
    // new reading, so it is not saved again (its time stays honest).
    var reusing = false;

    function boot(lat, lon, zoom, own) {
      map.setView([lat, lon], zoom);
      Pinalert.setCenter(lat, lon, own);
      if (!reusing) {
        saveReaderFix(lat, lon, own);
      }
      Pinalert.fetchReports();
      Pinalert.startPolling();
    }

    function useFix() {
      locateReader(
        function (pos) {
          boot(pos.coords.latitude, pos.coords.longitude, 13, true);
        },
        function () {
          boot(fallbackLat, fallbackLon, fallbackZoom, false);
        }
      );
    }

    // The browser's own question is asked on its own only once per account in
    // a browser (plan 36). After that the map starts from the kept fix, else
    // the home area centre or the configured fallback, and Back to my location
    // is how the person refreshes. A browser that already allows location is
    // read silently, because nothing is shown for that.
    askMode(function (mode) {
      if (mode !== 'reuse') {
        useFix();
        return;
      }
      var kept = keptReaderFix();
      if (kept) {
        reusing = true;
        boot(kept.lat, kept.lon, 13, true);
        return;
      }
      // Not a call to the browser: the fallback centre, as when location is off.
      boot(fallbackLat, fallbackLon, fallbackZoom, false);
    });
  }

  // askMode calls back once with 'silent', 'reuse' or 'ask' (see
  // PinalertPosition.startMode). A page without that module, or a browser with
  // no location API to ask, behaves as before: 'ask' (the failure branch of the
  // location step then answers the fallback).
  function askMode(callback) {
    var position = window.PinalertPosition;
    if (!position || typeof position.startMode !== 'function') {
      callback('ask');
      return;
    }
    position.startMode(callback);
  }

  // keptReaderFix is the device fix this account kept in this browser, or null.
  function keptReaderFix() {
    var position = window.PinalertPosition;
    if (!position || typeof position.read !== 'function') {
      return null;
    }
    return position.read();
  }

  // markReaderAsked notes that the person has been asked on this browser, so a
  // tap on Back to my location also counts as the one question.
  function markReaderAsked() {
    var position = window.PinalertPosition;
    if (position && typeof position.markAsked === 'function') {
      position.markAsked();
    }
  }

  // saveReaderFix keeps a device fix in this browser through PinalertPosition
  // (Phase 3.4 D-20), so My activity can show Ended nearby and its distances.
  // Only the reader's own fix is saved, never the fallback centre, and the
  // position goes nowhere else: it stays in this browser. A page without the
  // module saves nothing.
  function saveReaderFix(lat, lon, own) {
    if (own === true && window.PinalertPosition && typeof window.PinalertPosition.save === 'function') {
      window.PinalertPosition.save(lat, lon);
    }
  }

  // locateReader asks the browser for the reader's position once. A missing
  // geolocation API, a refusal and a timeout all answer through onFail, so the
  // caller has one fallback branch. The first centring and the return to the
  // reader share this step.
  function locateReader(onFix, onFail) {
    if (!navigator.geolocation) {
      onFail();
      return;
    }
    navigator.geolocation.getCurrentPosition(onFix, onFail, { timeout: 8000 });
  }

  // moveTo puts the map on a place, with the target in the middle of the part
  // of the map the tools layer leaves free: the target is shifted down by half
  // the measured reserve, so a pin there is never lost under the tools. It
  // never reads the list; the caller refetches once (D-14).
  function moveTo(lat, lon, zoom) {
    if (!map) {
      return;
    }
    var shift = toolsReserve() / 2;
    var target = map.project([lat, lon], zoom).subtract([0, shift]);
    map.setView(map.unproject(target, zoom), zoom, { animate: !reducedMotion });
  }

  // recenterOnReader is Back to my location: it runs the geolocation step
  // again, stores the centre with its own flag (a fix is the reader's own, the
  // fallback never is), moves the map there, reads the list once and resolves
  // with { own }. It does not start polling: the poll loop is already running.
  // The searched place is dropped only here, in the same step as the centre,
  // so during the wait for a position the place line, the distance words and
  // the list all still describe the searched area (IN-07).
  // A place search or a picked result made during the wait cancels it
  // (cancelRecenter): the latest action wins, and a position that arrives
  // after that changes nothing and resolves with { cancelled: true }.
  function recenterOnReader() {
    recenterSeq += 1;
    var mine = recenterSeq;
    // The person tapped, so the browser may ask; it counts as the one question.
    markReaderAsked();
    return new Promise(function (resolve) {
      function done(lat, lon, zoom, own) {
        if (mine !== recenterSeq) {
          resolve({ cancelled: true });
          return;
        }
        Pinalert.clearPlace();
        clearPlaceMark();
        Pinalert.setCenter(lat, lon, own);
        saveReaderFix(lat, lon, own);
        moveTo(lat, lon, zoom);
        Pinalert.fetchReports();
        resolve({ own: own });
      }
      locateReader(
        function (pos) {
          done(pos.coords.latitude, pos.coords.longitude, 13, true);
        },
        function () {
          done(Pinalert.config.fallbackLat, Pinalert.config.fallbackLon, Pinalert.config.fallbackZoom, false);
        }
      );
    });
  }

  // cancelRecenter makes a pending Back to my location ignore its position.
  function cancelRecenter() {
    recenterSeq += 1;
  }

  // The place mark (plan 04-32): a calm mark at the place picked in the place
  // search, so the map points at it. It is a solid pin in a dark neutral with a
  // white edge (main.css .place-mark), never a severity colour. It takes no click
  // and no focus, sits under every report pin, and is read by no screen reader:
  // the place line already names the place. It goes when Back to my location
  // lands (recenterOnReader) or another place is picked.
  var placeMark = null;
  var PLACE_MARK_Z_OFFSET = -1000;

  // setPlaceMark draws the place mark at lat, lon, taking it off any earlier place.
  function setPlaceMark(lat, lon) {
    if (!map) {
      return;
    }
    clearPlaceMark();
    var icon = L.divIcon({ html: buildPlaceMarkPin(), className: 'place-mark', iconSize: [32, 42], iconAnchor: [16, 41] });
    placeMark = L.marker([lat, lon], {
      icon: icon,
      interactive: false,
      keyboard: false,
      zIndexOffset: PLACE_MARK_Z_OFFSET
    }).addTo(map);
    var el = placeMark.getElement();
    if (el) {
      el.setAttribute('aria-hidden', 'true');
    }
  }

  // buildPlaceMarkPin draws the pin of the place mark (owner 2026-10-25, the
  // outline glyph "looks odd"): a teardrop in a 32 by 42 box with a round head
  // and a centre dot. Its tip, at (16, 41), is the iconAnchor, so the tip sits
  // on the place.
  function buildPlaceMarkPin() {
    var ns = 'http://www.w3.org/2000/svg';
    var svg = document.createElementNS(ns, 'svg');
    svg.setAttribute('viewBox', '0 0 32 42');
    svg.setAttribute('width', '32');
    svg.setAttribute('height', '42');
    svg.setAttribute('focusable', 'false');
    var body = document.createElementNS(ns, 'path');
    body.setAttribute('class', 'place-mark__body');
    body.setAttribute('d', 'M16 41C11 34 2 24.5 2 15A14 14 0 1 1 30 15C30 24.5 21 34 16 41Z');
    var dot = document.createElementNS(ns, 'circle');
    dot.setAttribute('class', 'place-mark__dot');
    dot.setAttribute('cx', '16');
    dot.setAttribute('cy', '15');
    dot.setAttribute('r', '5');
    svg.appendChild(body);
    svg.appendChild(dot);
    return svg;
  }

  // clearPlaceMark takes the place mark off the map, if one is drawn.
  function clearPlaceMark() {
    if (placeMark) {
      placeMark.remove();
      placeMark = null;
    }
  }

  // getZoom is the map's zoom, or null before the map exists.
  function getZoom() {
    return map ? map.getZoom() : null;
  }

  // getCenter is the visible centre of the map as { lat, lon }, the bias of a
  // place search, or null before the map exists. The longitude is wrapped
  // because the map scrolls sideways without a bound and the server refuses a
  // longitude outside 180.
  function getCenter() {
    if (!map) {
      return null;
    }
    var centre = map.getCenter().wrap();
    return { lat: centre.lat, lon: centre.lng };
  }

  // The marker offset that lifts the chosen pin above its neighbours. Leaflet
  // writes the stacking order of a marker inline from its position and this
  // offset, so a z-index on the badge inside it would not lift it.
  var CHOSEN_Z_OFFSET = 1000;

  // isChosen says whether a report is the one the person last chose, in the
  // list or on the map. The choice lives in the shared store (Pinalert.select).
  function isChosen(id) {
    var chosen = Pinalert.state.selectedId;
    return chosen !== null && chosen !== undefined && String(chosen) === String(id);
  }

  // applyChosenPin moves the grown look to the pin of report id and takes it
  // off every other pin. The class sits on the badge, which the next poll
  // builds again (buildBadgeElement reads the same selection), and the raise
  // sits on the marker, which keeps its offset through setIcon.
  function applyChosenPin(id) {
    var ids = Object.keys(markers);
    for (var i = 0; i < ids.length; i++) {
      var marker = markers[ids[i]];
      var chosen = id !== null && id !== undefined && String(id) === ids[i];
      var el = marker.getElement();
      var badge = el ? el.querySelector('.icon-badge--pin') : null;
      if (badge) {
        badge.classList.toggle('map-pin--chosen', chosen);
      }
      marker.setZIndexOffset(chosen ? CHOSEN_Z_OFFSET : 0);
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
    var ageClass = 'age-' + PinalertTrust.fadeStage(report);

    var badge = document.createElement('span');
    badge.className = 'icon-badge icon-badge--pin ' + sevClass + ' ' + ageClass;

    // The chosen pin grows (D-17) and a disputed pin is grey (D-14). Both are
    // fixed class names, set here from the shared selection and from the
    // server's own view word, so the next poll's new badge keeps them and no
    // report-authored value reaches a class name. The disputed test is the one
    // the list row reads (report.view === 'disputed'), never a count.
    if (isChosen(report.id)) {
      badge.classList.add('map-pin--chosen');
    }
    if (report.view === 'disputed') {
      badge.classList.add('icon-badge--disputed');
    }

    // On this surface the state class goes on the badge element itself,
    // because there is no row ancestor — the same split main.css's own
    // comment documents for the age ramp, and precisely why the
    // stylesheet's hidden-badge rule carries both the compound and the
    // descendant selector form.
    PinalertVisibility.applyVisibilityClass(badge, report);

    // A finished report also takes its ring modifier (03.1 D-13), so the
    // stylesheet can draw it as a ring and never as a filled severity dot.
    var ring = finishedPinClass(report);
    if (ring !== '') {
      badge.classList.add(ring);
    }

    var glyph = document.createElement('span');
    glyph.className = Pinalert.iconClass(report.category);
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);

    return badge;
  }

  // finishedPinClass names the ring modifier of a finished pin from the same
  // display state the header word and the pin class read: a Resolved report
  // gets the solid ring and an Expired one the dashed ring. Any other state
  // answers the empty string and the pin stays a filled dot. Both names are
  // fixed strings, so no report-authored value reaches a class name.
  function finishedPinClass(report) {
    var state = PinalertVisibility.displayState(report);
    if (state === 'retracted') {
      return 'icon-badge--resolved';
    }
    if (state === 'expired') {
      return 'icon-badge--expired';
    }
    return '';
  }

  // closedLineText is the line a finished report shows in place of the fade
  // sentence (C44, C45), or the empty string for a report that is not
  // finished. A Resolved report reads the server's closed words and the time
  // ("Resolved by the poster, 2 days ago."); an Expired one reads "Expired 3
  // days ago, after 8 hrs.". The words are the server's own (closed_text and
  // lifetime_text); the browser only joins them to the relative time of the
  // server's history time, and compares no date.
  function closedLineText(report) {
    var history = report && report.history;
    if (!history || typeof history.at !== 'string') {
      return '';
    }
    var when = Pinalert.relativeTime(history.at);
    if (history.state === 'resolved') {
      var who = typeof history.closed_text === 'string' && history.closed_text !== '' ? history.closed_text : 'Resolved';
      return who + ', ' + when + '.';
    }
    if (history.state === 'expired') {
      var lifetime = typeof history.lifetime_text === 'string' && history.lifetime_text !== '' ? ', ' + history.lifetime_text : '';
      return 'Expired ' + when + lifetime + '.';
    }
    return '';
  }

  // buildPopupCard constructs the popup card once per marker, as DOM nodes
  // only (D-20, A21). The header is the badge row (badge, severity label)
  // then the state line (state label, reporter words) (A80), then come the
  // title, the meta lines, the trust block and the actions. The
  // card has no footer and no comments link (A11). The report text is the
  // title and reports have no edit path, so it is written once here. Every
  // other field is written by updatePopupCard, which this builder ends with.
  function buildPopupCard(report) {
    var el = document.createElement('div');
    el.className = 'popup-card';
    el.setAttribute('tabindex', '-1');

    var body = document.createElement('div');
    body.className = 'popup-card__body';

    var header = document.createElement('div');
    header.className = 'popup-card__header';
    var pills = document.createElement('div');
    pills.className = 'popup-card__pills';
    var badge = document.createElement('span');
    badge.className = 'icon-badge icon-badge--sm';
    var glyph = document.createElement('span');
    glyph.setAttribute('aria-hidden', 'true');
    badge.appendChild(glyph);
    var severity = PinalertTrust.createSeverityLabel();
    var tag = PinalertVisibility.createVisibilityTag();
    pills.appendChild(badge);
    pills.appendChild(severity);
    header.appendChild(pills);

    var titleEl = document.createElement('p');
    titleEl.className = 'popup-card__title';
    titleEl.id = 'pinalert-popup-title-' + (++popupTitleSeq);
    Pinalert.setText(titleEl, report.description);

    var meta = document.createElement('div');
    meta.className = 'popup-card__meta';
    var metaGlyph = document.createElement('span');
    metaGlyph.setAttribute('aria-hidden', 'true');
    var metaLines = document.createElement('div');
    metaLines.className = 'popup-card__meta-lines';
    var metaLine = document.createElement('p');
    metaLine.className = 'popup-card__meta-line';
    // The Written line of a late report, directly under the category and time
    // line. It stays hidden until the report carries written_at (D-19).
    var writtenLine = document.createElement('p');
    writtenLine.className = 'popup-card__meta-line';
    writtenLine.hidden = true;
    var shelterLine = document.createElement('p');
    shelterLine.className = 'popup-card__meta-line';
    shelterLine.hidden = true;
    // The privacy line of a finished Rescue report (C19). It is hidden for
    // every other report and styled by trust.css as its own paragraph.
    var privacyLine = document.createElement('p');
    privacyLine.className = 'popup-card__privacy';
    privacyLine.hidden = true;
    metaLines.appendChild(metaLine);
    metaLines.appendChild(writtenLine);
    metaLines.appendChild(shelterLine);
    metaLines.appendChild(privacyLine);
    meta.appendChild(metaGlyph);
    meta.appendChild(metaLines);

    // The popup holds the always showing trust card (A91, A92). It has no reporter
    // words, so the state line carries the state label alone and the reporter
    // is the last tick row.
    var trust = PinalertTrust.createTrustCard();
    header.appendChild(PinalertTrust.createStateLine('popup-card__stateline', tag, trust));

    // The fade line sits under the card, above the vote buttons (A95).
    var fade = document.createElement('p');
    fade.className = 'popup-card__fade';
    fade.hidden = true;

    var actions = document.createElement('div');
    actions.className = 'popup-card__actions';
    var block = PinalertVotes.createVoteBlock(report.id);
    actions.appendChild(block.controls);
    // The too far sentence slot sits between the buttons and the error and is
    // owned by votes.js.
    actions.appendChild(block.note);
    actions.appendChild(block.error);
    // The Mute control (Phase 3.2, D-11) comes first, then the Flag control, so
    // the two are rows with Mute just above Flag (plan 04-30; arrangeCard puts
    // both at the bottom of the report on a phone). It is hidden for
    // a reader who neither follows the report nor muted it. A failed mute.js
    // load leaves the card without it.
    var muteControl = window.PinalertMute ? PinalertMute.createMuteControl(report) : null;
    if (muteControl) {
      actions.appendChild(muteControl.el);
      actions.appendChild(muteControl.error);
    }
    // The Flag control is the last element of the actions block, outside the
    // vote controls. Phase 3.1 moved it off the list row into the opened report.
    // A failed flag.js load leaves the card without it rather than stopping
    // the map after its first pin (WR-W04).
    var flagControl = window.PinalertFlag ? PinalertFlag.createFlagControl(report) : null;
    if (flagControl) {
      actions.appendChild(flagControl.el);
    }

    body.appendChild(header);
    body.appendChild(titleEl);
    body.appendChild(meta);
    body.appendChild(trust.el);
    body.appendChild(fade);
    body.appendChild(actions);
    el.appendChild(body);

    var card = {
      el: el,
      glyph: glyph,
      severity: severity,
      tag: tag,
      title: titleEl,
      metaGlyph: metaGlyph,
      metaLine: metaLine,
      writtenLine: writtenLine,
      shelterLine: shelterLine,
      privacy: privacyLine,
      trust: trust,
      fade: fade,
      votes: block,
      mute: muteControl,
      actions: actions,
      flag: flagControl,
      approximate: false,
      observer: null,
      frame: 0
    };
    updatePopupCard(card, report);
    return card;
  }

  // updatePopupCard refreshes the card on every poll by writing text, classes
  // and attributes on the nodes buildPopupCard made. It builds nothing and
  // never touches focus, the trust panel's open state or the vote controls'
  // own state, which is what lets a poll leave an open popup alone (A19).
  function updatePopupCard(card, report) {
    card.glyph.className = Pinalert.iconClass(report.category);
    card.metaGlyph.className = Pinalert.iconClass(report.category);

    PinalertTrust.updateSeverityLabel(card.severity, report);
    PinalertVisibility.updateVisibilityTag(card.tag, report);

    // A finished Rescue report arrives approximate with a blank description,
    // so its title is the fixed Rescue needed words (C17). Any other report
    // shows its own text. The node is written only when the words change, so
    // a poll never replaces the text under a reader's selection.
    var approximate = report.approximate === true;
    card.approximate = approximate;
    var titleText = approximate ? FINISHED_RESCUE_TITLE : report.description;
    if (card.title.textContent !== titleText) {
      Pinalert.setText(card.title, titleText);
    }

    // Category, relative time and the distance sentence of the store: the
    // reader's own distance, a distance from a searched place or an
    // approximate one, and nothing around the default centre. No severity
    // word: the label above says it (A24, A94, C20, C21).
    var categoryLabel = Pinalert.categoryLabel(report.category);
    var away = Pinalert.distanceText(report);
    var meta = categoryLabel + ' \u00b7 ' + Pinalert.relativeTime(report.created_at);
    if (away) {
      meta += ' \u00b7 ' + away;
    }
    Pinalert.setText(card.metaLine, meta);
    var written = Pinalert.writtenLine(report);
    Pinalert.setText(card.writtenLine, written);
    card.writtenLine.hidden = !written;

    // The line under the card has two voices in one slot. A finished report
    // shows its closed line (C44, C45) and never the fade sentence, because a
    // Resolved report may still have time left on its clock. A live report
    // shows the fade line, computed from its own end time in the shared
    // formatter and refreshed on every store update, like the relative time
    // above. An empty line stays hidden (A95).
    var closedText = closedLineText(report);
    var fadeText = closedText !== '' ? closedText : Pinalert.formatFade(report);
    Pinalert.setText(card.fade, fadeText);
    card.fade.hidden = !fadeText;
    card.fade.classList.toggle('popup-card__fade--closed', closedText !== '');

    // A shelter adds a second line, and so does an approximate report, whose
    // second line is the fixed Approximate location label (C18) in the same
    // slot. The shared formatter returns an empty string for any other
    // category, and an empty line stays hidden (RR-6). The privacy line sits
    // under it and shows for an approximate report only (C19).
    var shelter = approximate ? APPROX_LABEL : Pinalert.formatShelterLine(report);
    Pinalert.setText(card.shelterLine, shelter || '');
    card.shelterLine.hidden = !shelter;
    Pinalert.setText(card.privacy, approximate ? PRIVACY_LINE : '');
    card.privacy.hidden = !approximate;

    PinalertTrust.updateTrustCard(card.trust, report.trust);
    PinalertVotes.updateVoteBlock(card.votes, report);
    if (card.mute && window.PinalertMute) {
      PinalertMute.updateMuteControl(card.mute, report);
    }
    if (card.flag && window.PinalertFlag) {
      PinalertFlag.updateFlagControl(card.flag, report);
    }
  }

  // observePopupCard repositions an open popup when its card changes height,
  // for example when the resolve confirmation opens (A32). Browsers without
  // ResizeObserver keep the last position.
  function observePopupCard(marker, card) {
    if (typeof ResizeObserver !== 'function') {
      return;
    }
    card.observer = new ResizeObserver(function () {
      if (card.frame) {
        return;
      }
      card.frame = window.requestAnimationFrame(function () {
        card.frame = 0;
        relayoutOpenPopup(marker, card);
      });
    });
    card.observer.observe(card.el);
  }

  // relayoutOpenPopup moves the open popup to where it already is, which
  // makes Leaflet recompute its position and re-run the keep in view pan. It
  // never calls the popup content refresh, because that re-appends the card
  // and drops keyboard focus (A32).
  function relayoutOpenPopup(marker, card) {
    var popup = marker.getPopup();
    if (!popup || !popup.isOpen()) {
      return;
    }
    popup.setLatLng(popup.getLatLng());
    panPopupIntoView(popup);
  }

  // onPopupOpen is the one popupopen handler. It moves Leaflet's close anchor
  // in front of the card, so the close button is the first element a keyboard
  // or screen reader user meets in the popup (A20). Leaflet builds the anchor
  // once per popup and appends it last, so the move happens on the first open
  // and later opens find it already first. Moving the node keeps Leaflet's own
  // click listener. It then names the popup as a non modal dialog, marks the
  // popup and its pin for the Escape handler in trust.js, records who opened
  // it and puts focus on the card when the map pane is shown (A23, A74).
  function onPopupOpen(evt) {
    var container = evt.popup.getElement();
    if (!container) {
      return;
    }
    var close = container.querySelector('.leaflet-popup-close-button');
    if (close && close !== container.firstChild) {
      container.insertBefore(close, container.firstChild);
    }
    // On a phone the anchor is Back in the report view's top row, beside the
    // heading Report (plan 39). The laptop popup keeps Leaflet's own name.
    if (close) {
      close.setAttribute('aria-label', phoneDetail() ? BACK_LABEL : CLOSE_LABEL);
    }
    ensureReportHeading(container, close);

    var popup = evt.popup;
    activePopup = popup;
    var id = reportIdForPopup(popup);
    popupOpener = { id: id, kind: listOpenerId !== null && listOpenerId === id ? 'list' : 'marker' };
    listOpenerId = null;

    container.setAttribute('role', 'dialog');
    container.setAttribute('aria-modal', 'false');
    container.setAttribute('data-popup-open', 'true');
    document.documentElement.setAttribute('data-popup-open', 'true');
    var title = container.querySelector('.popup-card__title');
    if (title && title.id) {
      container.setAttribute('aria-labelledby', title.id);
    }
    var pin = id !== null && markers[id] ? markers[id].getElement() : null;
    if (pin) {
      pin.setAttribute('data-popup-open', 'true');
    }

    var card = popupCardOf(popup);
    openedWhileHidden = !card || card.offsetParent === null;
    // The address and, on a phone, the map view to come back to (plan 39).
    // It runs before the strip centring below moves the map.
    noteReportOpened(id);
    refreshPanPadding();
    applyPanPadding(popup);
    panPopupIntoView(popup);
    toolsTried = { popup: popup, count: 0 };
    keepCardClearOfTools(popup);
    // The laptop card takes focus as before. On a phone focus goes to the
    // heading of the report view, Report, in the top row (plan 39).
    if (!openedWhileHidden && !phoneDetail()) {
      card.focus({ preventScroll: true });
    }
    if (phoneDetail()) {
      // A list row flies the map to the pin again right after this open
      // (feed.js activateRow), which left the pin at the bottom edge of the
      // strip, under the report. The centring therefore runs one turn later:
      // its setView stops that fly (plan 39).
      window.setTimeout(function () {
        if (activePopup === popup) {
          centerPinInStrip(popup, container);
        }
      }, 0);
      focusReportHeading(container);
    }
  }

  // centerPinInStrip puts the open report's pin in the middle of the short map
  // strip that shows above the sheet on a phone. The strip height is read from
  // where the sheet starts, not assumed. setView stops a running fly, and the
  // offset then moves the pin from the middle of the whole map to the middle
  // of the strip. It runs at once, with no animation.
  function centerPinInStrip(popup, popupEl) {
    if (!map || !popup) {
      return;
    }
    var strip = popupEl.getBoundingClientRect().top - map.getContainer().getBoundingClientRect().top;
    if (!(strip > 0)) {
      return;
    }
    // A live pin is shown at street level at least. An approximate one is
    // shown at the ceiling exactly, the same level the fly from a list row
    // ends at, so the strip does not imply a street level spot for a blurred
    // point and a phone matches a laptop (T-31-58). This call stops a running
    // fly, whose zoom has not yet left its start, so it must set the level
    // itself and not read the map's current one.
    var id = reportIdForPopup(popup);
    var approximate = id !== null && cards[id] && cards[id].approximate === true;
    var zoom = approximate ? MAX_APPROX_ZOOM : Math.max(map.getZoom(), 16);
    map.setView(popup.getLatLng(), zoom, { animate: false });
    map.panBy([0, Math.round(map.getSize().y / 2 - strip / 2)], { animate: false });
  }

  // reportIdForPopup returns the id of the marker that owns this popup, or
  // null.
  function reportIdForPopup(popup) {
    var ids = Object.keys(markers);
    for (var i = 0; i < ids.length; i++) {
      if (markers[ids[i]].getPopup() === popup) {
        return ids[i];
      }
    }
    return null;
  }

  // markerForElement returns the marker of a list row or a Past card (found
  // by its report id as a key, never through a selector built from report
  // data), or the marker whose pin element is el, or null.
  function markerForElement(el) {
    if (el && typeof el.closest === 'function') {
      var opener = el.closest('.report-row[data-report-id], .past-card[data-report-id]');
      if (opener && opener.dataset.reportId && markers[opener.dataset.reportId]) {
        return markers[opener.dataset.reportId];
      }
    }
    var ids = Object.keys(markers);
    for (var i = 0; i < ids.length; i++) {
      if (markers[ids[i]].getElement() === el) {
        return markers[ids[i]];
      }
    }
    return null;
  }

  // popupCardOf returns the card element inside an open popup, or null.
  function popupCardOf(popup) {
    var el = popup ? popup.getElement() : null;
    return el ? el.querySelector('.popup-card') : null;
  }

  // pinName is the accessible name of a pin: the category, the severity word,
  // the relative time and, when one shows, the state word (A45). Pins stay
  // colour only to the eye (owner decision Q5). Every part comes from a fixed
  // table or the shared time formatter.
  function pinName(report) {
    var parts = [Pinalert.categoryLabel(report.category)];
    if (Object.prototype.hasOwnProperty.call(Pinalert.SEVERITY_WORDS, report.severity)) {
      parts.push(Pinalert.SEVERITY_WORDS[report.severity]);
    }
    parts.push(Pinalert.relativeTime(report.created_at));
    var state = PinalertVisibility.stateWord(report);
    if (typeof state === 'string' && state !== '') {
      parts.push(state);
    }
    // A finished Rescue pin sits at a blurred point, and the name says so
    // (D-23).
    if (report.approximate === true) {
      parts.push('approximate location');
    }
    return parts.join(', ');
  }

  // applyPinName writes the name as an attribute, never as markup.
  function applyPinName(marker, report) {
    var el = marker.getElement();
    if (el) {
      el.setAttribute('aria-label', pinName(report));
    }
  }

  // onMapFocusIn keeps popupFocusInside true while focus is in the open popup.
  function onMapFocusIn(evt) {
    var el = activePopup ? activePopup.getElement() : null;
    popupFocusInside = !!(el && el.contains(evt.target));
  }

  // onMapFocusOut clears the flag only when focus moves to a node outside the
  // popup. A null target leaves it alone, because some browsers fire focusout
  // with no target when Leaflet removes a focused container at once.
  function onMapFocusOut(evt) {
    var el = activePopup ? activePopup.getElement() : null;
    if (el && evt.relatedTarget && !el.contains(evt.relatedTarget)) {
      popupFocusInside = false;
    }
  }

  // onMapPointerDown records whether a press landed on the popup close button
  // while focus was inside the popup. The press comes before the browser moves
  // focus, so popupFocusInside is still true here. Every press sets the answer
  // again.
  function onMapPointerDown(evt) {
    var target = evt.target;
    closePressHadFocus = !!(popupFocusInside && target && target.closest && target.closest('.leaflet-popup-close-button'));
  }

  // onMapKeydown adds Space to what Enter already does on the map (A49):
  // Space opens a focused pin and presses the popup close button. Enter stays
  // Leaflet's own.
  function onMapKeydown(evt) {
    if (evt.key !== ' ' && evt.key !== 'Spacebar') {
      return;
    }
    var target = evt.target;
    if (!target || !target.classList) {
      return;
    }
    if (target.classList.contains('leaflet-popup-close-button')) {
      evt.preventDefault();
      map.closePopup();
      return;
    }
    if (target.classList.contains('leaflet-marker-icon')) {
      var marker = markerForElement(target);
      if (marker) {
        evt.preventDefault();
        marker.openPopup();
      }
    }
  }

  // onPopupClose clears the marks and returns focus to the opener when focus
  // was in the popup and has not gone elsewhere (A74). The popup's trust card
  // is always showing, so there is no panel to collapse.
  // Focus on the map container counts as lost only after a close button press,
  // because that is where WebKit puts it on a mouse press.
  function onPopupClose(evt) {
    // The report detail is closed: the phone list sheet that dropped to short
    // beneath it returns to the position it had (D-17, orchestrator decision 8).
    // It does nothing on a laptop or when the sheet was not moved for this report.
    if (window.PinalertSheet && typeof window.PinalertSheet.restore === 'function') {
      window.PinalertSheet.restore();
    }
    var popup = evt.popup;
    var el = popup.getElement();
    var id = reportIdForPopup(popup);
    var pressedClose = closePressHadFocus;
    var hadFocus = popupFocusInside || pressedClose;
    var opener = popupOpener;
    closePressHadFocus = false;

    document.documentElement.removeAttribute('data-popup-open');
    if (el) {
      el.removeAttribute('data-popup-open');
    }
    var pin = id !== null && markers[id] ? markers[id].getElement() : null;
    if (pin) {
      pin.removeAttribute('data-popup-open');
    }

    popupFocusInside = false;
    popupOpener = null;
    openedWhileHidden = false;
    if (activePopup === popup) {
      activePopup = null;
    }

    var active = document.activeElement;
    var lost = !active || active === document.body || !!(el && el.contains(active)) ||
      (pressedClose && active === map.getContainer());
    if (hadFocus && lost && !removingPopup) {
      returnFocusToOpener(opener);
    }

    // Every close of the linked popup ends the link but the one a resize
    // makes, which the reader did not ask for and which leaves the report in
    // the set. A redraw that keeps the report updates its marker in place
    // and never closes its popup, so a redraw closes it only by removing the
    // pin: the report has left the tab or the chip the reader is on (a
    // resolve from Active, a take back off Past, a fade). Keeping the link
    // then would send report=ID on every poll and could bring the pin back
    // with no popup.
    if (id !== null && String(id) === openedFocusId && !resizeClosing) {
      endLinkOnClose();
    }

    // The report's address ends, or moves to the next report when one opens in
    // the same turn (plan 39).
    noteReportClosed();
  }

  // endLinkOnClose ends a report link when its popup closes (CR-01): the
  // store stops asking for the report and forgets its focus object. The pin
  // only the link put on the map goes at a redraw run after Leaflet has
  // finished the close, because removing a marker inside its own popup close
  // would remove that popup a second time. When a redraw made the close, the
  // pin is already gone and the later redraw changes nothing. The focus
  // return of onPopupClose above stays synchronous; the redraw only catches a
  // focus the removed pin was holding. The once per link mark is cleared
  // too: the one way the same id is linked again is the toast Undo of a mark
  // that moved the linked report off the map, and its popup opens again then.
  function endLinkOnClose() {
    openedFocusId = null;
    Pinalert.dropFocus();
    window.setTimeout(redrawAfterLinkEnded, 0);
  }

  // redrawAfterLinkEnded draws the pins again once a report link has ended,
  // which takes off the pin the link alone kept. When the pin it takes off
  // held focus (the close above hands focus back to the pin), focus goes to
  // the map, never to the page body.
  function redrawAfterLinkEnded() {
    var before = document.activeElement;
    render(Pinalert.state);
    if (map && before && before !== document.body && !before.isConnected) {
      map.getContainer().focus({ preventScroll: true });
    }
  }

  // returnFocusToOpener sends focus back to the list row that opened the
  // popup when that row is connected and displayed, else to the pin (A74). The
  // row is found by comparing its data attribute, never by a selector built
  // from report data. When neither takes focus, as while another set loads
  // and its loading mark hides the rows and the pins, focus goes to the map,
  // never to the page body.
  function returnFocusToOpener(opener) {
    if (!opener) {
      return;
    }
    if (opener.kind === 'list') {
      var rows = document.querySelectorAll('.report-row, .past-card');
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].dataset.reportId === String(opener.id) && rows[i].isConnected && rows[i].offsetParent !== null) {
          rows[i].focus();
          if (document.activeElement === rows[i]) {
            return;
          }
          break;
        }
      }
    }
    var marker = opener.id !== null ? markers[opener.id] : null;
    var pin = marker ? marker.getElement() : null;
    if (pin) {
      pin.focus({ preventScroll: true });
      if (document.activeElement === pin) {
        return;
      }
    }
    if (map) {
      map.getContainer().focus({ preventScroll: true });
    }
  }

  // popupHoldsFocus is true only when focus is inside this popup right now and
  // on a control that still takes part in layout. A focus already lost before
  // the refetch, such as the hidden Mark resolved button, is not claimed here,
  // so the Mark resolved focus path keeps sending it to the map (A52).
  function popupHoldsFocus(popup) {
    var el = popup ? popup.getElement() : null;
    if (!popupFocusInside || !el || !el.contains(document.activeElement)) {
      return false;
    }
    return document.activeElement.getClientRects().length > 0;
  }

  // takeRemovedPopupFocus says whether a poll just removed the popup that held
  // focus, and clears the answer so it is read once (A67).
  function takeRemovedPopupFocus() {
    var had = removedPopupFocus;
    removedPopupFocus = false;
    return had;
  }

  // onMapResize recomputes the autopan paddings, applies them to every bound
  // popup so one first opened after a breakpoint change pans with current
  // values, and re-measures the open popup, but never while focus is inside it
  // (A59, A68). Leaflet fires the resize event from its size check, which the
  // window resize event and a phone rotation both reach.
  function onMapResize() {
    refreshPanPadding();
    // A resize that crosses 900px closes the open popup through the existing
    // close path, which returns focus to the opener, so the next open lands in
    // the right pane. A resize that does not cross it does nothing new.
    var narrow = phoneDetail();
    var crossed = narrow !== lastNarrow;
    lastNarrow = narrow;
    if (crossed && activePopup) {
      resizeClosing = true;
      map.closePopup();
      resizeClosing = false;
    }
    var ids = Object.keys(markers);
    for (var i = 0; i < ids.length; i++) {
      applyPanPadding(markers[ids[i]].getPopup());
      applyPopupPlacement(markers[ids[i]].getPopup(), cards[ids[i]]);
    }
    var el = activePopup ? activePopup.getElement() : null;
    if (el && !el.contains(document.activeElement)) {
      activePopup.update();
      var card = popupCardOf(activePopup);
      if (card && card.offsetParent !== null) {
        openedWhileHidden = false;
      }
    }
  }

  // onMapMoveEnd stands in for the keep in view pan that Leaflet skips when
  // autopan is off under reduced motion (A68).
  function onMapMoveEnd() {
    if (activePopup) {
      panPopupIntoView(activePopup);
      keepCardClearOfTools(activePopup);
    }
  }

  // keepCardClearOfTools moves the map at once when the open card's top edge
  // sits above the top reserve, which is never less than the bottom of the
  // tools layer (the place search and the chips) plus one small space, so the
  // close button is never under the Search button (browser finding F-P4-7).
  // Leaflet's own autopan runs inside the open, ahead of popupopen, and a list
  // row press flies the map while that pan runs, so the card could settle
  // under the tools row with no correction. This runs after every open and
  // every move, at any motion setting. It refreshes the paddings first, so
  // Leaflet's keep in view pan reads the same numbers, and it only ever moves
  // the card down. The phone sheet is fixed in the detail pane and is left
  // alone, as is a map the reader is still dragging. The map's bounds can undo
  // a pan (the whole world shown at a low zoom), so at most TOOLS_TRIES
  // corrections run in a row before the card is seen clear again or another
  // popup opens, and the two can never pan back and forth for ever. The
  // moveend of its own pan is skipped (clearingTools), so only a later, separate
  // move can clear the count.
  function keepCardClearOfTools(popup) {
    if (clearingTools || !map || !popup || !popup.isOpen() || phoneDetail()) {
      return;
    }
    if (map.dragging && map.dragging.moving()) {
      return;
    }
    refreshPanPadding();
    applyPanPadding(popup);
    var el = popup.getElement();
    if (!el) {
      return;
    }
    var box = map.getContainer().getBoundingClientRect();
    var rect = el.getBoundingClientRect();
    if (box.height === 0 || rect.height === 0) {
      return;
    }
    if (toolsTried.popup !== popup) {
      toolsTried = { popup: popup, count: 0 };
    }
    var dy = Math.round(rect.top - box.top - panPad.topLeft[1]);
    if (dy >= 0) {
      toolsTried.count = 0;
      return;
    }
    if (toolsTried.count >= TOOLS_TRIES) {
      return;
    }
    toolsTried.count += 1;
    clearingTools = true;
    try {
      map.panBy([0, dy], { animate: false });
    } finally {
      clearingTools = false;
    }
  }

  // panPopupIntoView moves the map at once, with no animation, so the open
  // card sits inside the padded view (A68). It acts only under reduced motion,
  // when Leaflet's own autopan is off. The offset is clamped per axis with the
  // top left edge winning, as Leaflet does, so a card larger than the padded
  // view settles with its top left inside and two calls in a row can never
  // pan back and forth.
  function panPopupIntoView(popup) {
    if (!reducedMotion || !map || !popup || !popup.isOpen()) {
      return;
    }
    // On a phone the detail is a fixed sheet, so there is nothing to pan into
    // view.
    if (phoneDetail()) {
      return;
    }
    if (map.dragging && map.dragging.moving()) {
      return;
    }
    var el = popup.getElement();
    if (!el) {
      return;
    }
    var box = map.getContainer().getBoundingClientRect();
    var rect = el.getBoundingClientRect();
    if (box.width === 0 || box.height === 0 || rect.width === 0 || rect.height === 0) {
      return;
    }
    var left = rect.left - box.left;
    var right = rect.right - box.left;
    var top = rect.top - box.top;
    var bottom = rect.bottom - box.top;
    var padL = panPad.topLeft[0];
    var padT = panPad.topLeft[1];
    var padR = panPad.bottomRight[0];
    var padB = panPad.bottomRight[1];
    var dx = 0;
    var dy = 0;
    if (right > box.width - padR) {
      dx = right - (box.width - padR);
    }
    if (left - dx < padL) {
      dx = left - padL;
    }
    if (bottom > box.height - padB) {
      dy = bottom - (box.height - padB);
    }
    if (top - dy < padT) {
      dy = top - padT;
    }
    dx = Math.round(dx);
    dy = Math.round(dy);
    if (dx !== 0 || dy !== 0) {
      map.panBy([dx, dy], { animate: false });
    }
  }

  // focusOpenPopup lands focus on the open card when its pane is shown and
  // says whether it did (A51). A popup opened while its pane was hidden was
  // measured empty, so it is re-measured here, unless focus is already inside
  // it (A59).
  function focusOpenPopup() {
    var card = popupCardOf(activePopup);
    if (!card || card.offsetParent === null) {
      return false;
    }
    if (openedWhileHidden && !activePopup.getElement().contains(document.activeElement)) {
      activePopup.update();
      openedWhileHidden = false;
    }
    card.focus({ preventScroll: true });
    return true;
  }

  // popupStateClasses returns the space joined triple of state classes this
  // report's popup container must carry: severity, age and visibility, in
  // that order. All three are already validated against fixed allowlists by
  // their own modules (Pinalert.severityClass, PinalertTrust.fadeStage,
  // PinalertVisibility.displayState, which also answers Expired for a
  // finished report, the same word the pin and the header show), so no report-authored value ever
  // reaches this class name, the same discipline buildBadgeElement is
  // already under. The age class is included for parity with the feed
  // row's own class list even though no age rule touches --severity-tint,
  // so carrying it cannot introduce a fade-with-age behaviour on the popup
  // tint, and nothing inside .map-popup consumes --severity-current or
  // --badge-glyph-fg today either.
  function popupStateClasses(report) {
    return Pinalert.severityClass(report) + ' age-' + PinalertTrust.fadeStage(report) +
      ' vis-' + PinalertVisibility.displayState(report);
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
    replacePrefixedClass(el, 'age-', 'age-' + PinalertTrust.fadeStage(report));
    PinalertVisibility.applyVisibilityClass(el, report);
  }

  function upsertMarker(report) {
    var icon = L.divIcon({
      html: buildBadgeElement(report),
      className: '',
      iconSize: [44, 44],
      popupAnchor: [0, -22]
    });

    var existing = markers[report.id];
    if (existing) {
      var at = existing.getLatLng();
      if (at.lat !== report.latitude || at.lng !== report.longitude) {
        existing.setLatLng([report.latitude, report.longitude]);
      }
      existing.setIcon(icon);
      applyPinName(existing, report);
      updatePopupCard(cards[report.id], report);
      syncPopupState(existing, report);
      return;
    }

    var marker = L.marker([report.latitude, report.longitude], {
      icon: icon,
      zIndexOffset: isChosen(report.id) ? CHOSEN_Z_OFFSET : 0
    }).addTo(map);
    applyPinName(marker, report);
    var card = buildPopupCard(report);
    cards[report.id] = card;
    marker.bindPopup(card.el, { className: popupStateClasses(report), minWidth: 340, maxWidth: 340, keepInView: true, autoPan: !reducedMotion, autoPanPaddingTopLeft: panPad.topLeft, autoPanPaddingBottomRight: panPad.bottomRight });
    applyPopupPlacement(marker.getPopup(), card);
    marker.on('click', function () {
      Pinalert.select(report.id, 'map');
    });
    observePopupCard(marker, card);
    markers[report.id] = marker;
  }

  // updateMapLabel writes the one line label at the bottom left of the map
  // (C41, D-13). It names the set only while Show Disputed is chosen, so a
  // reader who sees only pins that are in dispute knows which set the map
  // shows. The words are fixed strings written through the text sink, and the
  // label is hidden and emptied otherwise. It reads only the store's choice.
  function updateMapLabel(state) {
    var labelEl = document.getElementById('map-label');
    if (!labelEl) {
      return;
    }
    var words = '';
    if (state.view === 'disputed') {
      words = 'Showing disputed reports';
    }
    Pinalert.setText(labelEl, words);
    labelEl.hidden = words === '';
  }

  // render reconciles markers by report id rather than clearing and
  // rebuilding the layer on every poll, which would drop an open popup
  // every 30 seconds.
  function render(state) {
    if (!map) {
      return;
    }
    removedPopupFocus = false;
    updateMapLabel(state);

    // The pins are the list's reports under the Show choice (the same set the
    // rows draw, one call for both) plus, when a report link asked for one
    // outside the list, that one report under the Show choice and chip that
    // fit it (D-11, D-13). A copy is made so the store's own array is never
    // changed.
    var shown = Pinalert.visibleReports();
    var linked = focusPinReport(state);
    if (linked !== null) {
      shown = shown.concat([linked]);
    }

    var seenIds = {};
    for (var i = 0; i < shown.length; i++) {
      var report = shown[i];
      seenIds[report.id] = true;
      upsertMarker(report);
    }

    var existingIds = Object.keys(markers);
    for (var j = 0; j < existingIds.length; j++) {
      var id = existingIds[j];
      if (!seenIds[id]) {
        if (markers[id].isPopupOpen()) {
          removedPopupFocus = popupHoldsFocus(markers[id].getPopup());
        }
        removingPopup = true;
        map.removeLayer(markers[id]);
        removingPopup = false;
        delete markers[id];
        if (cards[id]) {
          if (cards[id].frame) {
            window.cancelAnimationFrame(cards[id].frame);
          }
          if (cards[id].observer) {
            cards[id].observer.disconnect();
          }
          delete cards[id];
        }
      }
    }

    openFocusOnce(state);
    noteFocusAnswer(state);
  }

  // belongsToShow says whether a linked report is a member of the Show choice
  // the reader is on, from the server's own view word on the report. Nearby
  // draws only reports that are active or disputed: a finished report is no
  // longer drawn here, its link lands on My activity (Phase 3.4, orchestrator
  // decision 6). The Confirmed filter reads the server's confirmed_by_people.
  // Nothing here reads a date or a count.
  function belongsToShow(report, show) {
    var own = report ? report.view : '';
    if (own !== 'active' && own !== 'disputed') {
      return false;
    }
    return Pinalert.reportInShow(report, show);
  }

  // focusPinReport answers the linked report that needs its own pin, or null.
  // It needs the store's focus to be ok, a report that is not already among
  // the drawn pins (the list's own pin is used then), a place on the map, a
  // Show choice the reader is on and a chip, when one is on, that matches its
  // category. The focus object is the newest answer's (CR-01), so the pin and
  // its popup always show the server's current state, and the store drops it
  // when the link ends. The Show rule keeps the pin off a set the report does
  // not belong to (D-13), and the Only Reliable reporters tick keeps it off
  // when the server's word says the tick leaves the report out (3.2 D-26).
  function focusPinReport(state) {
    var focus = state.focus;
    if (!focus || focus.status !== 'ok' || !focus.report) {
      return null;
    }
    var report = focus.report;
    if (typeof report.latitude !== 'number' || typeof report.longitude !== 'number') {
      return null;
    }
    var drawn = Pinalert.visibleReports();
    for (var i = 0; i < drawn.length; i++) {
      if (String(drawn[i].id) === String(report.id)) {
        return null;
      }
    }
    if (!belongsToShow(report, state.show)) {
      return null;
    }
    if (!Pinalert.reportKeptByReliable(report, state.reliable)) {
      return null;
    }
    if (state.category !== '' && report.category !== state.category) {
      return null;
    }
    return report;
  }

  // openFocusOnce opens the popup of a /?report=ID link, once per link. It
  // waits until the pin exists under the Show choice the reader is on, and a
  // later poll, Show change or return never opens it again. A link to a
  // finished report never gets here: the list leaves Nearby for My activity
  // (feed.js handleFocus). It reads the store and moves
  // the map only; the list is not touched. The selection is announced as the
  // map's own, which lets the list highlight the row when it is in the list.
  function openFocusOnce(state) {
    var focus = state.focus;
    if (!focus || focus.status !== 'ok' || !focus.report) {
      return;
    }
    var id = String(focus.report.id);
    if (openedFocusId === id) {
      return;
    }
    if (!markers[id]) {
      return;
    }
    openedFocusId = id;
    if (!activePopup) {
      rememberNearbyView();
    }
    Pinalert.select(id, 'map');
    flyTo(id);
  }

  // zoomFor is the zoom a code driven move to a report ends at: street level
  // (16) for a live report, and the approximate ceiling for a finished Rescue
  // report, whose point is blurred and must not read as a street level spot.
  function zoomFor(id) {
    var card = cards[id];
    if (card && card.approximate === true) {
      return MAX_APPROX_ZOOM;
    }
    return 16;
  }

  // flyTo pans to a report and opens its popup only when the popup
  // is closed. That is the wide-screen "clicking a list row pans/flies the
  // map" half of D-06's sync contract. The selection callback has usually
  // opened the popup already, and Leaflet's open call on an open popup removes
  // the card node and appends it again, which drops the focus popupopen just
  // gave it and fires no second popupopen. The list names itself as the opener
  // through opts (A74) and the opener is claimed here either way.
  function flyTo(id, opts) {
    var marker = markers[id];
    if (!marker || !map) {
      return false;
    }
    if (opts && opts.opener === 'list') {
      listOpenerId = String(id);
    }
    var zoom = zoomFor(id);
    if (reducedMotion) {
      map.setView(marker.getLatLng(), zoom, { animate: false });
    } else {
      map.flyTo(marker.getLatLng(), zoom);
    }
    if (!marker.isPopupOpen()) {
      marker.openPopup();
    }
    if (listOpenerId !== null && marker.isPopupOpen()) {
      popupOpener = { id: listOpenerId, kind: 'list' };
      listOpenerId = null;
    }
    return true;
  }

  // PIN_HALF is half the 44 by 44 marker box: a pin whose centre is this far
  // from an edge is whole inside it.
  var PIN_HALF = 22;

  // resizeToSheet runs when the phone list sheet has settled (sheet.js, after
  // a snap, a resize or an orientation change). The map element's height
  // follows the sheet's top by CSS (phone.css, the sheet top plus 16px), so
  // here the map only measures itself again, with no animation and no pan of
  // its own, and then brings the chosen pin inside the part of the map that
  // shows above the sheet, so no pin is ever left under it (D-16, D-03). A
  // sheet at full hides the whole map, so there is nothing to bring in. The
  // report detail sheet covers the map from the 192px line down and
  // centerPinInStrip has already put the pin in the strip above it, so an
  // open phone detail is left alone. Whether the MapLibre basemap follows a
  // resize of its Leaflet container is confirmed in a browser, not assumed.
  function resizeToSheet(sheetTop) {
    if (!map) {
      return;
    }
    map.invalidateSize({ animate: false, pan: false });
    if (!(sheetTop > 0) || (phoneDetail() && activePopup)) {
      return;
    }
    var id = Pinalert.state.selectedId;
    var marker = id === null || id === undefined ? null : markers[String(id)];
    if (!marker) {
      return;
    }
    var spaceMd = readSpaceToken('--space-md');
    map.panInside(marker.getLatLng(), {
      paddingTopLeft: [spaceMd + PIN_HALF, toolsReserve() + PIN_HALF],
      paddingBottomRight: [spaceMd + PIN_HALF, spaceMd + PIN_HALF],
      animate: !reducedMotion
    });
  }

  // focusMarker puts focus on the pin of report id when it is on the page, so a
  // closed dialog can hand focus back to the pin of a popup that has gone. It
  // does not scroll or pan the map. It returns whether a pin took the focus.
  function focusMarker(id) {
    var marker = markers[String(id)] || markers[id];
    if (!marker) {
      return false;
    }
    var el = marker.getElement ? marker.getElement() : null;
    if (!el || !el.isConnected) {
      return false;
    }
    el.focus({ preventScroll: true });
    return true;
  }

  // ---- The report view and its address (plan 39) ----
  // On a phone an opened report fills the screen inside Nearby: the top row
  // reads Back, the title Report, the bell and the gear, a short map strip
  // keeps every pin with this one grown and centred, and the report scrolls
  // under it to its last control (phone.css). It is still the report's own
  // popup in the detail pane, so the card, the trust block, the votes, Mark
  // resolved, Undo and Flag are the same nodes and behave as before. The laptop
  // keeps its popup over the map exactly as it was (owner, 2026-10-19).
  // On both sizes an open report has its own address, ?report=ID: opening one
  // from Nearby adds one history entry, another pin switches the report in that
  // entry, and Back, Escape or the browser's back close it. A page that arrived
  // with the address adds no entry, and closing strips the id. Pinalert.openReport
  // (app.js) hands its id to openReport here, as does an address on load.

  var REPORT_PARAM = 'report';
  var HISTORY_KEY = 'pinalertReport';
  var REPORT_ID_TEXT = /^[1-9][0-9]{0,17}$/;
  // The phone's Back is Leaflet's own close anchor in the top row, so its name
  // is Back there. The laptop keeps Leaflet's own name.
  var BACK_LABEL = 'Back';
  var CLOSE_LABEL = 'Close popup';
  var REPORT_TITLE = 'Report';
  // 'pushed' when this script added the address entry of the open report,
  // 'url' when the page arrived with it, null when none is open.
  var historyMode = null;
  // The timer every close starts. An open in the same turn clears it: Leaflet
  // closes the open report first when another one opens, which is a switch.
  var closeSettle = 0;
  // True while the browser's back or forward closes the open report, so that
  // close does not move the history again.
  var closeFromHistory = false;
  // The id the browser's back or forward asked to open, so its open adds no entry.
  var historyOpenId = null;
  // Phone: where the map was before the first report opened, put back on close.
  var nearbyView = null;
  // The last focus answer the not available line was shown for, and what had
  // focus before it showed.
  var goneFocus = null;
  var goneOpener = null;

  // currentReportParam is the report id in the address, or null.
  function currentReportParam() {
    var value = new URLSearchParams(window.location.search).get(REPORT_PARAM);
    return value !== null && REPORT_ID_TEXT.test(value) ? value : null;
  }

  // writeReportAddress puts id in the address (or takes it out for null),
  // keeping every other parameter. how is 'push' for a new entry; anything else
  // replaces the current one. An entry this script made carries HISTORY_KEY.
  function writeReportAddress(id, how) {
    var history = window.history;
    if (!history || typeof history.replaceState !== 'function' || typeof history.pushState !== 'function') {
      return;
    }
    var params = new URLSearchParams(window.location.search);
    if (id === null) {
      params.delete(REPORT_PARAM);
    } else {
      params.set(REPORT_PARAM, id);
    }
    var query = params.toString();
    var address = window.location.pathname + (query ? '?' + query : '') + window.location.hash;
    var entry = null;
    if (id !== null && (how === 'push' || historyMode === 'pushed')) {
      entry = {};
      entry[HISTORY_KEY] = id;
    }
    if (how === 'push') {
      history.pushState(entry, '', address);
    } else {
      history.replaceState(entry, '', address);
    }
  }

  // rememberNearbyView keeps where the phone map is before the first report
  // opens: the place at its top left corner and the zoom. The corner, not the
  // centre, because the map's height follows the list sheet and every resize
  // keeps the top left corner where it is (sheet.js, resizeToSheet), so the
  // corner is what still matches once the sheet is back. A switch keeps the
  // first answer; the laptop keeps nothing, so its map stays where the report
  // left it, as before.
  function rememberNearbyView() {
    if (nearbyView !== null || !map || !phoneDetail()) {
      return;
    }
    nearbyView = { corner: map.containerPointToLatLng([0, 0]), zoom: map.getZoom() };
  }

  // restoreNearbyView puts the phone map back where it was, with no animation:
  // the remembered place in the top left corner again at the remembered zoom.
  function restoreNearbyView() {
    var view = nearbyView;
    nearbyView = null;
    if (view === null || !map) {
      return;
    }
    var corner = map.project(view.corner, view.zoom);
    var size = map.getSize();
    map.setView(map.unproject([corner.x + size.x / 2, corner.y + size.y / 2], view.zoom), view.zoom, { animate: false });
  }

  // noteReportOpened runs on every open (onPopupOpen). An open right after a
  // close in the same turn is a switch and keeps the one entry. Otherwise the
  // first open from Nearby adds the entry, unless the address already names
  // this report (a page loaded from it, or the browser's back or forward).
  function noteReportOpened(id) {
    var key = String(id);
    var fromHistory = historyOpenId === key;
    historyOpenId = null;
    hideReportGone(false);
    if (closeSettle) {
      window.clearTimeout(closeSettle);
      closeSettle = 0;
      if (!fromHistory && currentReportParam() !== key) {
        writeReportAddress(key, 'replace');
      }
      return;
    }
    rememberNearbyView();
    if (fromHistory) {
      var state = window.history ? window.history.state : null;
      historyMode = state && state[HISTORY_KEY] ? 'pushed' : 'url';
      return;
    }
    // The address already names this report: a page loaded from it. An entry
    // this script pushed before a reload keeps its mark, so closing still goes
    // back to Nearby instead of leaving a second Nearby entry behind it.
    if (currentReportParam() === key) {
      var current = window.history ? window.history.state : null;
      historyMode = current && current[HISTORY_KEY] === key ? 'pushed' : 'url';
      return;
    }
    historyMode = 'pushed';
    writeReportAddress(key, 'push');
  }

  // noteReportClosed runs on every close (onPopupClose). What the close means
  // for the address and the phone map is settled a turn later, so an open that
  // follows at once (a switch) can cancel it.
  function noteReportClosed() {
    var fromHistory = closeFromHistory;
    closeFromHistory = false;
    if (closeSettle) {
      window.clearTimeout(closeSettle);
    }
    closeSettle = window.setTimeout(function () {
      closeSettle = 0;
      settleReportClosed(fromHistory);
    }, 0);
  }

  // settleReportClosed ends the address of a closed report: back one entry
  // when this script added it (the browser lands on Nearby), else the id is
  // taken out of the address. A close the browser made moves nothing. The
  // phone map goes back to where it was before the report opened.
  function settleReportClosed(fromHistory) {
    var mode = historyMode;
    historyMode = null;
    if (!fromHistory) {
      var state = window.history ? window.history.state : null;
      if (mode === 'pushed' && state && state[HISTORY_KEY]) {
        window.history.back();
      } else if (currentReportParam() !== null) {
        writeReportAddress(null, 'replace');
      }
    }
    restoreNearbyView();
  }

  // onReportPopState follows the browser's back and forward. An address with
  // no report closes the open one (or the not available line); an address with
  // a report opens it.
  function onReportPopState() {
    var key = currentReportParam();
    hideReportGone(false);
    if (key === null) {
      historyMode = null;
      if (activePopup && map) {
        closeFromHistory = true;
        map.closePopup();
        closeFromHistory = false;
      }
      return;
    }
    if (activePopup && reportIdForPopup(activePopup) === key) {
      return;
    }
    historyOpenId = key;
    Pinalert.openReport(key);
  }

  // openReport opens one report that has a pin, by id, and says whether it
  // did. It is the map half of the one public way in, Pinalert.openReport
  // (app.js), which an address on load uses too. The report opens as a tap on
  // its pin opens it: the report first, then the selection, so the phone sheet
  // is not raised under it. For any other id it answers false and app.js asks
  // the server for it as a report link, the same path an address on load
  // takes: focusPinReport decides whether it gets a pin, openFocusOnce opens
  // it, and noteFocusAnswer shows the not available line on a phone when the
  // server cannot show it. map.js itself never writes the link or reads the list.
  function openReport(id) {
    var key = id === null || id === undefined ? '' : String(id);
    if (!REPORT_ID_TEXT.test(key) || !map || !markers[key]) {
      return false;
    }
    if (activePopup && reportIdForPopup(activePopup) === key) {
      return true;
    }
    if (!activePopup) {
      rememberNearbyView();
    }
    flyTo(key);
    Pinalert.select(key, 'map');
    return true;
  }

  // noteFocusAnswer shows the not available line on a phone once per answer
  // that says a linked report cannot be shown. The laptop keeps the toast
  // feed.js shows, as before.
  function noteFocusAnswer(state) {
    var focus = state ? state.focus : null;
    if (!focus || focus.status === 'ok' || focus === goneFocus) {
      return;
    }
    goneFocus = focus;
    if (phoneDetail()) {
      showReportGone();
    }
  }

  // showReportGone opens the not available view and moves focus to its heading.
  function showReportGone() {
    var panel = document.getElementById('report-gone');
    if (!panel) {
      return;
    }
    goneOpener = document.activeElement;
    panel.hidden = false;
    document.documentElement.setAttribute('data-report-gone', 'true');
    var title = document.getElementById('report-gone-title');
    if (title) {
      title.focus({ preventScroll: true });
    }
  }

  // hideReportGone closes the not available view. With returnFocus it hands
  // focus back to what had it, else to the map, never to the page body.
  function hideReportGone(returnFocus) {
    var panel = document.getElementById('report-gone');
    if (!panel || panel.hidden) {
      return;
    }
    panel.hidden = true;
    document.documentElement.removeAttribute('data-report-gone');
    var opener = goneOpener;
    goneOpener = null;
    if (!returnFocus) {
      return;
    }
    if (opener && opener !== document.body && opener.isConnected && typeof opener.focus === 'function') {
      opener.focus({ preventScroll: true });
      if (document.activeElement === opener) {
        return;
      }
    }
    if (map) {
      map.getContainer().focus({ preventScroll: true });
    }
  }

  // closeReportGone is Back and Escape on the not available view: it closes
  // and takes the id out of the address.
  function closeReportGone() {
    hideReportGone(true);
    if (currentReportParam() !== null) {
      writeReportAddress(null, 'replace');
    }
  }

  // wireReportGone connects Back and Escape on the not available view.
  function wireReportGone() {
    var panel = document.getElementById('report-gone');
    var back = document.getElementById('report-gone-back');
    if (back) {
      back.addEventListener('click', closeReportGone);
    }
    if (panel) {
      panel.addEventListener('keydown', function (evt) {
        if (evt.key === 'Escape') {
          evt.preventDefault();
          evt.stopPropagation();
          closeReportGone();
        }
      });
    }
  }

  // ensureReportHeading gives a popup container its heading, Report, once,
  // right after the close anchor. phone.css shows it in the top row beside
  // Back; everywhere else it is not drawn, so the laptop popup is unchanged.
  function ensureReportHeading(container, close) {
    var heading = container.querySelector('.report-view__title');
    if (heading) {
      return heading;
    }
    heading = document.createElement('h2');
    heading.className = 'report-view__title';
    heading.setAttribute('tabindex', '-1');
    Pinalert.setText(heading, REPORT_TITLE);
    container.insertBefore(heading, close ? close.nextSibling : container.firstChild);
    return heading;
  }

  // focusReportHeading moves focus to the heading of the phone report view.
  function focusReportHeading(container) {
    var heading = container.querySelector('.report-view__title');
    if (heading && heading.getClientRects().length > 0) {
      heading.focus({ preventScroll: true });
    }
  }
  // ---- End of the report view (plan 39) ----

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  return {
    flyTo: flyTo,
    moveTo: moveTo,
    recenterOnReader: recenterOnReader,
    cancelRecenter: cancelRecenter,
    getCenter: getCenter,
    getZoom: getZoom,
    setPlaceMark: setPlaceMark,
    clearPlaceMark: clearPlaceMark,
    resizeToSheet: resizeToSheet,
    focusOpenPopup: focusOpenPopup,
    takeRemovedPopupFocus: takeRemovedPopupFocus,
    focusMarker: focusMarker,
    openReport: openReport
  };
}());
