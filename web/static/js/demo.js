// demo.js: the Demo strip's height (plan 05-13, UI-SPEC 4a).
//
// Every shell on the page subtracts --disclaimer-bar-height from the screen
// height. On the real pages theme.js writes that token from the laptop notice
// strip. The Demo draws its own strip instead (on a phone too), so this file
// measures the Demo strip and writes its height into the same token on the
// root. It runs after theme.js (it is a deferred script, theme.js measured while
// the page was being read), so the Demo's value is the one that stays. The strip
// wraps and grows at narrow widths and enlarged text, so the measurement is
// repeated when the strip changes size, on every window resize, and once on the
// next frame for fonts that arrive late.
//
// It reads the page and writes one style property. It stores nothing in the
// browser and never asks for a location.
(function () {
  'use strict';

  var PROPERTY = '--disclaimer-bar-height';
  var strip = null;
  var frame = 0;

  // measureStrip writes the strip's real height into the token. A strip that is
  // gone from the page writes nothing, and one that takes no part in layout
  // writes 0px, as theme.js does for its strip.
  function measureStrip() {
    if (!strip || !strip.isConnected) {
      return;
    }
    var height = strip.offsetHeight === 0 ? 0 : strip.getBoundingClientRect().height;
    document.documentElement.style.setProperty(PROPERTY, height + 'px');
  }

  // scheduleMeasure asks for a measurement on the next frame, so a write never
  // resizes the shells inside the strip observer's own delivery (the same care
  // theme.js takes).
  function scheduleMeasure() {
    if (frame || typeof window.requestAnimationFrame !== 'function') {
      return;
    }
    frame = window.requestAnimationFrame(function () {
      frame = 0;
      measureStrip();
    });
  }

  function init() {
    strip = document.querySelector('.demo-strip');
    if (!strip) {
      return;
    }
    measureStrip();
    window.addEventListener('resize', measureStrip);
    if (typeof ResizeObserver === 'function') {
      new ResizeObserver(scheduleMeasure).observe(strip);
    }
    scheduleMeasure();
  }

  window.PinalertDemoStrip = { measureStrip: measureStrip, init: init };
  init();
}());
