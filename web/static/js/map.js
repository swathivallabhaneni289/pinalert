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
  // Reduced motion is read once. It turns off the map's fade and zoom
  // animations, the animated fly and Leaflet's own autopan (A60, A68).
  var reducedMotion = mediaMatches('(prefers-reduced-motion: reduce)');
  // The two media conditions trust.css switches the popup max height at (A65).
  var NARROW_QUERY = '(max-width: 899.98px)';
  var SHORT_QUERY = '(max-height: 359px)';
  // Autopan paddings in pixels, derived from the spacing tokens (A60).
  var panPad = { topLeft: [0, 0], bottomRight: [0, 0] };

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
    }
    panPad = { topLeft: [spaceMd, top], bottomRight: [spaceMd, bottom] };
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
  // to the shared store so the one fetch + polling loop can start. The own
  // flag tells the store whether the centre is the reader's own position, so
  // the popup shows a distance only when it is the reader's (A95).
  function centerOnVisitor() {
    var fallbackLat = Pinalert.config.fallbackLat;
    var fallbackLon = Pinalert.config.fallbackLon;

    function boot(lat, lon, zoom, own) {
      map.setView([lat, lon], zoom);
      Pinalert.setCenter(lat, lon, own);
      Pinalert.fetchReports();
      Pinalert.startPolling();
    }

    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        function (pos) {
          boot(pos.coords.latitude, pos.coords.longitude, 13, true);
        },
        function () {
          boot(fallbackLat, fallbackLon, 11, false);
        },
        { timeout: 8000 }
      );
    } else {
      boot(fallbackLat, fallbackLon, 11, false);
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
    var shelterLine = document.createElement('p');
    shelterLine.className = 'popup-card__meta-line';
    shelterLine.hidden = true;
    metaLines.appendChild(metaLine);
    metaLines.appendChild(shelterLine);
    meta.appendChild(metaGlyph);
    meta.appendChild(metaLines);

    // The popup holds the always showing trust card (A90). It has no reporter
    // words, so the state line carries the state label alone and the reporter
    // is the last tick row.
    var trust = PinalertTrust.createTrustCard();
    header.appendChild(PinalertTrust.createStateLine('popup-card__stateline', tag, trust));

    // The fade line sits under the card, above the vote buttons (A94).
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
      shelterLine: shelterLine,
      trust: trust,
      fade: fade,
      votes: block,
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

    // Category, relative time and, only when the feed was loaded around the
    // reader's own position, the distance. No severity word: the label above
    // says it (A24, A93, A95).
    var categoryLabel = Pinalert.categoryLabel(report.category);
    var away = Pinalert.feedIsAroundReader() ? Pinalert.formatDistance(report.distance_km) : '';
    var meta = categoryLabel + ' \u00b7 ' + Pinalert.relativeTime(report.created_at);
    if (away) {
      meta += ' \u00b7 ' + away;
    }
    Pinalert.setText(card.metaLine, meta);

    // The fade line is computed from expires_at in the shared formatter and
    // refreshed on every store update, like the relative time above. An empty
    // line stays hidden (A94).
    var fadeText = Pinalert.formatFade(report);
    Pinalert.setText(card.fade, fadeText);
    card.fade.hidden = !fadeText;

    // A shelter adds a second line. The shared formatter returns an empty
    // string for any other category, and an empty line stays hidden (RR-6).
    var shelter = Pinalert.formatShelterLine(report);
    Pinalert.setText(card.shelterLine, shelter || '');
    card.shelterLine.hidden = !shelter;

    PinalertTrust.updateTrustCard(card.trust, report.trust);
    PinalertVotes.updateVoteBlock(card.votes, report);
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

    var popup = evt.popup;
    activePopup = popup;
    var id = reportIdForPopup(popup);
    popupOpener = { id: id, kind: listOpenerId !== null && listOpenerId === id ? 'list' : 'marker' };
    listOpenerId = null;

    container.setAttribute('role', 'dialog');
    container.setAttribute('aria-modal', 'false');
    container.setAttribute('data-popup-open', 'true');
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
    refreshPanPadding();
    applyPanPadding(popup);
    panPopupIntoView(popup);
    if (!openedWhileHidden) {
      card.focus({ preventScroll: true });
    }
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

  // markerForElement returns the marker whose pin element is el, or null.
  function markerForElement(el) {
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
    var popup = evt.popup;
    var el = popup.getElement();
    var id = reportIdForPopup(popup);
    var pressedClose = closePressHadFocus;
    var hadFocus = popupFocusInside || pressedClose;
    var opener = popupOpener;
    closePressHadFocus = false;

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
  }

  // returnFocusToOpener sends focus back to the list row that opened the
  // popup when that row is connected and displayed, else to the pin (A74). The
  // row is found by comparing its data attribute, never by a selector built
  // from report data.
  function returnFocusToOpener(opener) {
    if (!opener) {
      return;
    }
    if (opener.kind === 'list') {
      var rows = document.querySelectorAll('.report-row');
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].dataset.reportId === String(opener.id) && rows[i].isConnected && rows[i].offsetParent !== null) {
          rows[i].focus();
          return;
        }
      }
    }
    var marker = opener.id !== null ? markers[opener.id] : null;
    var pin = marker ? marker.getElement() : null;
    if (pin) {
      pin.focus({ preventScroll: true });
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
    var ids = Object.keys(markers);
    for (var i = 0; i < ids.length; i++) {
      applyPanPadding(markers[ids[i]].getPopup());
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
  // PinalertVisibility.visibilityState), so no report-authored value ever
  // reaches this class name, the same discipline buildBadgeElement is
  // already under. The age class is included for parity with the feed
  // row's own class list even though no age rule touches --severity-tint,
  // so carrying it cannot introduce a fade-with-age behaviour on the popup
  // tint, and nothing inside .map-popup consumes --severity-current or
  // --badge-glyph-fg today either.
  function popupStateClasses(report) {
    return Pinalert.severityClass(report) + ' age-' + PinalertTrust.fadeStage(report) +
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

    var marker = L.marker([report.latitude, report.longitude], { icon: icon }).addTo(map);
    applyPinName(marker, report);
    var card = buildPopupCard(report);
    cards[report.id] = card;
    marker.bindPopup(card.el, { className: popupStateClasses(report), minWidth: 340, maxWidth: 340, keepInView: true, autoPan: !reducedMotion, autoPanPaddingTopLeft: panPad.topLeft, autoPanPaddingBottomRight: panPad.bottomRight });
    marker.on('click', function () {
      Pinalert.select(report.id, 'map');
    });
    observePopupCard(marker, card);
    markers[report.id] = marker;
  }

  // render reconciles markers by report id rather than clearing and
  // rebuilding the layer on every poll, which would drop an open popup
  // every 30 seconds.
  function render(state) {
    if (!map) {
      return;
    }
    removedPopupFocus = false;

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
  }

  // flyTo pans to a report at zoom 16 and opens its popup only when the popup
  // is closed. That is the wide-screen "clicking a list row pans/flies the
  // map" half of D-06's sync contract. The selection callback has usually
  // opened the popup already, and Leaflet's open call on an open popup removes
  // the card node and appends it again, which drops the focus popupopen just
  // gave it and fires no second popupopen. The list names itself as the opener
  // through opts (A74) and the opener is claimed here either way.
  function flyTo(id, opts) {
    var marker = markers[id];
    if (!marker || !map) {
      return;
    }
    if (opts && opts.opener === 'list') {
      listOpenerId = String(id);
    }
    if (reducedMotion) {
      map.setView(marker.getLatLng(), 16, { animate: false });
    } else {
      map.flyTo(marker.getLatLng(), 16);
    }
    if (!marker.isPopupOpen()) {
      marker.openPopup();
    }
    if (listOpenerId !== null && marker.isPopupOpen()) {
      popupOpener = { id: listOpenerId, kind: 'list' };
      listOpenerId = null;
    }
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
    highlight: highlight,
    focusOpenPopup: focusOpenPopup,
    takeRemovedPopupFocus: takeRemovedPopupFocus
  };
}());
