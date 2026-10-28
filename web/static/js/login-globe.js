// login-globe.js: the real, slowly turning Earth behind the sign in card
// (Phase 5 plan 07, D-13, UI-SPEC Surface 5, sketch 009 "For the build").
//
// Nothing here runs unless the page wrote the globe markup, which it does only
// when the device has not chosen the Low data view. The form never waits for
// this file: the loader runs after the page has loaded and the browser is idle,
// it asks for WebGL2 before it downloads anything, and it gives up quietly on
// any failure. A failure leaves the globe's empty space and the credit's
// hidden space, so the card never moves.
//
// The libraries are the exact maplibre-gl 5.24.0 files the map page loads. Their
// addresses and SRI hashes arrive as data attributes on this script's own tag
// (written once, in login_globe.html.tmpl), and the pictures come from this
// site's own /globe/tiles/ route, never from another host. No position is read
// and no call is made to the app's data routes: the whole Earth is shown with
// India in the middle, so no request reveals where the visitor is.
//
// Opacity is never set from here. The dim is a rule on the wrapper in
// login-globe.css, and the fade in is a keyframe on the inner element.
(function () {
  'use strict';

  var TURN_SECONDS = 120;       // one full turn every 2 minutes
  var START_CENTRE = [80, 18];  // India in the middle at the start
  var GIVE_UP_MS = 10000;       // a download that hangs fires no error, so give up after 10 seconds
  var MAX_ZOOM = 3;             // sharp enough for the sphere and caps the download
  var TILES_PREFIX = '/globe/tiles/';
  var FRAME_MS = 33;            // about 30 frames a second is plenty at 3 degrees a second
  var BOX = 1.06;               // room around the sphere so its edge is never clipped
  var DEG_PER_MS = 360 / (TURN_SECONDS * 1000);
  // The stylesheet's phone query, word for word (login-globe.css reserves the
  // phone box under it). Keep the two in step.
  var PHONE_QUERY = '(max-width: 899.98px)';

  var script = document.getElementById('login-globe-script');
  var wrapper = document.getElementById('login-globe');
  var host = document.getElementById('login-globe-map');
  var credit = document.getElementById('globe-credit');
  var gate = document.getElementById('login-gate');
  if (!script || !wrapper || !host || !credit || !gate) {
    return;
  }

  var tiles = script.getAttribute('data-tiles') || '';
  var libJS = script.getAttribute('data-lib-js') || '';
  var libJSSRI = script.getAttribute('data-lib-js-sri') || '';
  var libCSS = script.getAttribute('data-lib-css') || '';
  var libCSSSRI = script.getAttribute('data-lib-css-sri') || '';

  var map = null;
  var raf = 0;
  var lastFrame = 0;
  var lng = START_CENTRE[0];
  var ready = false;
  var failed = false;
  var giveUpTimer = 0;
  var reduceQuery = null;

  // canDrawWebGL2 is the same probe the map page uses: a canvas that can give a
  // webgl2 context. It runs before anything is downloaded.
  function canDrawWebGL2() {
    try {
      var gl = document.createElement('canvas').getContext('webgl2');
      if (!gl) {
        return false;
      }
      var lose = gl.getExtension('WEBGL_lose_context');
      if (lose) {
        lose.loseContext();
      }
      return true;
    } catch (e) {
      return false;
    }
  }

  // zoomFor solves for the zoom at which the sphere looks d pixels across in a
  // canvas canvasH pixels tall. The sphere's radius is 512 * 2^zoom / (2 PI) /
  // cos(latitude) pixels at the point under the middle of the screen, and the
  // camera sits 1.5 canvas heights away (the default 36.87 degree field of
  // view), so the sphere looks smaller than that radius.
  function zoomFor(d, canvasH) {
    var ra = d / 2;
    var cam = 1.5 * canvasH;
    var r = (ra * ra + ra * Math.sqrt(ra * ra + cam * cam)) / cam;
    return Math.log2(r * 2 * Math.PI * Math.cos(START_CENTRE[1] * Math.PI / 180) / 512);
  }

  // isPhone asks the stylesheet's own query, so the script and the stylesheet
  // always agree on phone or laptop. The gate's width excludes a classic
  // scrollbar and would disagree between about 900 and 917px wide, so it is only
  // the fallback for a browser with no matchMedia.
  function isPhone() {
    if (typeof window.matchMedia === 'function') {
      return window.matchMedia(PHONE_QUERY).matches;
    }
    return gate.clientWidth < 900;
  }

  // sizeGlobe sizes the sphere. On a phone it reads the box the stylesheet
  // reserved (the wrapper's width) and the sphere is that box divided by BOX, so
  // the whole Earth shows; the stylesheet alone places the box, so no --globe
  // variable is written. On a laptop it reads the sign in area: its width and
  // the height of the first screen below its top. Neither changes when the card
  // swaps for Check your inbox, so the globe never moves. Never a bare viewport
  // height. A box of 0 (the stylesheet did not apply) gives a diameter of 0.
  function sizeGlobe() {
    if (isPhone()) {
      var box = wrapper.clientWidth;
      if (!(box > 0)) {
        return { diameter: 0, boxW: 0, boxH: 0 };
      }
      var pd = box / BOX;
      if (map) {
        map.setZoom(zoomFor(pd, Math.round(box)));
      }
      return { diameter: pd, boxW: box, boxH: box };
    }
    var w = gate.clientWidth;
    var h = Math.max(240, window.innerHeight - gate.offsetTop);
    var d = Math.min(h * 0.92, 900);
    var bw = Math.min(d * BOX, w);
    var bh = Math.min(d * BOX, h);
    wrapper.style.setProperty('--globe-x', Math.round((w - bw) / 2) + 'px');
    wrapper.style.setProperty('--globe-y', Math.round((h - bh) / 2) + 'px');
    wrapper.style.setProperty('--globe-w', Math.round(bw) + 'px');
    wrapper.style.setProperty('--globe-h', Math.round(bh) + 'px');
    if (map) {
      map.setZoom(zoomFor(d, Math.round(bh)));
    }
    return { diameter: d, boxW: bw, boxH: bh };
  }

  function stopLoop() {
    if (raf) {
      cancelAnimationFrame(raf);
    }
    raf = 0;
  }

  // turn moves the centre longitude by 360 degrees per TURN_SECONDS. The centre
  // moves west, so the surface moves west to east, as the Earth turns. A frame
  // that comes sooner than FRAME_MS is skipped and its time carries to the next.
  function turn(now) {
    raf = requestAnimationFrame(turn);
    if (!lastFrame) {
      lastFrame = now;
      return;
    }
    var dt = now - lastFrame;
    if (dt < FRAME_MS) {
      return;
    }
    lastFrame = now;
    lng -= DEG_PER_MS * Math.min(dt, 100);
    if (lng < -180) {
      lng += 360;
    }
    map.jumpTo({ center: [lng, START_CENTRE[1]] });
  }

  function startLoop() {
    if (raf) {
      return;
    }
    lastFrame = 0;
    raf = requestAnimationFrame(turn);
  }

  // followReducedMotion keeps the loop in step with the reduce motion setting
  // and the tab. Under reduce motion the globe is one still frame with India in
  // the middle; a hidden tab pauses the loop.
  function followReducedMotion() {
    if (!ready || failed || !map) {
      return;
    }
    if (reduceQuery && reduceQuery.matches) {
      stopLoop();
      lng = START_CENTRE[0];
      map.jumpTo({ center: [START_CENTRE[0], START_CENTRE[1]] });
      return;
    }
    if (document.hidden) {
      stopLoop();
      return;
    }
    startLoop();
  }

  // giveUp removes the map and leaves the empty space. The wrapper and the
  // credit lose is-ready, so the credit stays out of sight and out of the Tab
  // order, and nothing on the page moves.
  function giveUp(reason) {
    if (failed) {
      return;
    }
    failed = true;
    clearTimeout(giveUpTimer);
    giveUpTimer = 0;
    stopLoop();
    if (map) {
      try {
        map.remove();
      } catch (e) {
        // already gone
      }
    }
    map = null;
    wrapper.classList.remove('is-ready');
    credit.classList.remove('is-ready');
    wrapper.setAttribute('data-globe-off', String(reason));
  }

  // armGiveUp starts the count of GIVE_UP_MS. A hidden tab draws nothing, so a
  // count that ends while the tab is hidden gives nothing up, and showing the
  // tab starts the count again.
  function armGiveUp() {
    clearTimeout(giveUpTimer);
    giveUpTimer = setTimeout(function () {
      giveUpTimer = 0;
      if (ready || failed || document.hidden) {
        return;
      }
      giveUp('nothing drew within 10 seconds');
    }, GIVE_UP_MS);
  }

  function onFirstFrame() {
    if (failed || ready) {
      return;
    }
    ready = true;
    clearTimeout(giveUpTimer);
    giveUpTimer = 0;
    wrapper.classList.add('is-ready');
    credit.classList.add('is-ready');
    followReducedMotion();
  }

  function buildMap() {
    if (failed) {
      return;
    }
    if (typeof maplibregl === 'undefined') {
      giveUp('maplibregl is missing');
      return;
    }
    var size = sizeGlobe();
    if (!(size.diameter > 0)) {
      giveUp('the reserved box has no size');
      return;
    }
    try {
      map = new maplibregl.Map({
        container: host,
        style: {
          version: 8,
          projection: { type: 'globe' },
          sources: {
            earth: { type: 'raster', tiles: [tiles], tileSize: 256, minzoom: 0, maxzoom: MAX_ZOOM }
          },
          layers: [{ id: 'earth', type: 'raster', source: 'earth', paint: { 'raster-fade-duration': 0 } }]
        },
        center: [START_CENTRE[0], START_CENTRE[1]],
        zoom: zoomFor(size.diameter, Math.round(size.boxH)),
        interactive: false,
        attributionControl: false,
        pixelRatio: Math.min(window.devicePixelRatio || 1, 2),
        canvasContextAttributes: { contextType: 'webgl2', antialias: true, powerPreference: 'low-power' },
        fadeDuration: 0,
        refreshExpiredTiles: false
      });
    } catch (e) {
      map = null;
      giveUp('the map could not start');
      return;
    }
    map.on('error', function () {
      if (!ready) {
        giveUp('the map reported an error before its first frame');
      }
    });
    map.on('load', onFirstFrame);
    map.on('webglcontextlost', function () {
      giveUp('the WebGL context was lost');
    });
  }

  // injectLibraries adds the pinned stylesheet and script, each with its SRI
  // hash, crossorigin anonymous and no referrer, and calls done once both have
  // arrived. An error from either gives up.
  function injectLibraries(done) {
    var pending = 2;
    function arrived() {
      pending -= 1;
      if (pending === 0 && !failed) {
        done();
      }
    }
    function broken() {
      giveUp('a map library did not load');
    }
    var css = document.createElement('link');
    css.rel = 'stylesheet';
    css.integrity = libCSSSRI;
    css.crossOrigin = 'anonymous';
    css.referrerPolicy = 'no-referrer';
    css.onload = arrived;
    css.onerror = broken;
    css.href = libCSS;
    document.head.appendChild(css);

    var js = document.createElement('script');
    js.async = true;
    js.integrity = libJSSRI;
    js.crossOrigin = 'anonymous';
    js.referrerPolicy = 'no-referrer';
    js.onload = arrived;
    js.onerror = broken;
    js.src = libJS;
    document.head.appendChild(js);
  }

  function onVisibility() {
    if (!ready && !failed && !document.hidden) {
      armGiveUp();
    }
    followReducedMotion();
  }

  // startGlobe is the whole job: probe, then download, then build. Only an
  // address under our own /globe/tiles/ route is accepted for the pictures.
  function startGlobe() {
    if (tiles.indexOf(TILES_PREFIX) !== 0 || !libJS || !libCSS) {
      return;
    }
    if (!canDrawWebGL2()) {
      return;
    }
    if (!(sizeGlobe().diameter > 0)) {
      return;
    }
    armGiveUp();
    if (typeof window.matchMedia === 'function') {
      reduceQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
      if (typeof reduceQuery.addEventListener === 'function') {
        reduceQuery.addEventListener('change', followReducedMotion);
      } else if (typeof reduceQuery.addListener === 'function') {
        reduceQuery.addListener(followReducedMotion);
      }
    }
    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('resize', sizeGlobe);
    if (typeof ResizeObserver === 'function') {
      new ResizeObserver(sizeGlobe).observe(gate);
    }
    injectLibraries(buildMap);
  }

  // The form is usable from the first paint: nothing is downloaded until the
  // page has loaded and the browser is idle (Safari has no requestIdleCallback).
  function whenIdle() {
    if (typeof window.requestIdleCallback === 'function') {
      window.requestIdleCallback(startGlobe, { timeout: 1500 });
    } else {
      setTimeout(startGlobe, 200);
    }
  }

  window.PinalertLoginGlobe = { canDrawWebGL2: canDrawWebGL2, sizeGlobe: sizeGlobe };

  if (document.readyState === 'complete') {
    whenIdle();
  } else {
    window.addEventListener('load', whenIdle, { once: true });
  }
})();
