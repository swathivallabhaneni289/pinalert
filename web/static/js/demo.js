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

  // The practice flow (plan 05-17, D-01, D-05, UI-SPEC 4d).
  //
  // A visitor posts a practice report through the real Post a report window. The
  // app raises pinalert:report-posted once the new report is in the store. This
  // file then shows the toast, opens the report on its trust block, and follows
  // the feed state while the pretend neighbours' votes come due: it reloads the
  // feed every PRACTICE_POLL_MS until the server says the report is confirmed,
  // and then says so once in the polite status line.
  //
  // Every trust word comes from the server (the report's trust object, which the
  // real rules wrote). This file holds no trust sentence of its own: it only
  // adds the practice prefix to the server's confirmed sentence (TRUST-02,
  // T-05-92). Everything it writes is text content (T-05-93). Only the Demo page
  // takes part: on any other page the post event is ignored.
  // Two seconds, at most half the pretend neighbours' gap (NeighbourInterval,
  // 4 s), so the climb shows near 4 s and 8 s (Phase 5 review fix IN-B02).
  var PRACTICE_POLL_MS = 2000;
  // The poll gives up after this many ticks (two minutes): the neighbours
  // finish within seconds, so a report that never shows up is a dropped visit or
  // a filtered list and polling forever would only be noise (T-05-94).
  var PRACTICE_POLL_LIMIT = 60;
  var POSTED_TOAST = 'Practice report posted. Pretend neighbours will confirm it in a few seconds.';
  var CONFIRMED_PREFIX = 'Practice report confirmed. ';
  var REPORT_POSTED_EVENT = 'pinalert:report-posted';
  var STATUS_ID = 'feed-status';
  var WELCOME_WORD = 'welcome';

  // watch is the one practice report being followed, or null.
  var watch = null;

  function app() {
    return window.Pinalert || null;
  }

  // isDemoPage also reads the page itself (Phase 5 review fix CR-B02): when
  // api-base.js failed to load, window.PinalertDemoPage is unset, but the page
  // root still says data-demo="1", and an ended visit must still restart the
  // Demo rather than leave the page asking a visit that is gone.
  function isDemoPage() {
    if (window.PinalertDemoPage === true) {
      return true;
    }
    var shell = typeof document.getElementById === 'function' ? document.getElementById('app-shell') : null;
    if (shell && shell.getAttribute('data-demo') === '1') {
      return true;
    }
    if (typeof document.querySelector !== 'function') {
      return false;
    }
    try {
      return !!document.querySelector('main[data-demo="1"]');
    } catch (err) {
      return false;
    }
  }

  // reportInState finds the report with this id in the feed state, or null.
  function reportInState(id) {
    var pinalert = app();
    var list = pinalert && pinalert.state && pinalert.state.reports;
    if (!Array.isArray(list)) {
      return null;
    }
    for (var i = 0; i < list.length; i += 1) {
      if (list[i] && String(list[i].id) === String(id)) {
        return list[i];
      }
    }
    return null;
  }

  // stopWatching ends the poll and the state subscription of the current watch.
  function stopWatching() {
    if (!watch) {
      return;
    }
    var ending = watch;
    watch = null;
    window.clearInterval(ending.timer);
    if (typeof ending.unsubscribe === 'function') {
      ending.unsubscribe();
    }
  }

  // announceConfirmed writes the practice announcement into the polite status
  // line: the practice prefix and the server's own confirmed sentence.
  function announceConfirmed(sentence) {
    var el = document.getElementById(STATUS_ID);
    if (!el) {
      return;
    }
    var text = CONFIRMED_PREFIX + (typeof sentence === 'string' ? sentence : '');
    var pinalert = app();
    if (pinalert && typeof pinalert.setText === 'function') {
      pinalert.setText(el, text);
    } else {
      el.textContent = text;
    }
  }

  // checkWatched looks at the watched report in the feed state. When the server
  // says it is confirmed it announces that once and ends the watch.
  function checkWatched(mine) {
    if (watch !== mine) {
      return;
    }
    var report = reportInState(mine.id);
    if (!report || report.confirmed_by_people !== true) {
      return;
    }
    var trust = report.trust || {};
    stopWatching();
    announceConfirmed(trust.confirmed_sentence);
  }

  // watchPractice follows the practice report with this id. A watch that is
  // already running is ended first, so only the newest practice report counts.
  function watchPractice(id) {
    stopWatching();
    var pinalert = app();
    if (!pinalert || typeof pinalert.subscribe !== 'function') {
      return;
    }
    var mine = { id: id, timer: null, unsubscribe: null, ticks: 0 };
    watch = mine;
    mine.unsubscribe = window.Pinalert.subscribe(function () {
      checkWatched(mine);
    });
    mine.timer = window.setInterval(function () {
      if (watch !== mine) {
        return;
      }
      mine.ticks += 1;
      if (mine.ticks > PRACTICE_POLL_LIMIT) {
        stopWatching();
        return;
      }
      if (typeof pinalert.fetchReports === 'function') {
        pinalert.fetchReports();
      }
    }, PRACTICE_POLL_MS);
    // The report may already be confirmed in the state it was posted into.
    checkWatched(mine);
  }

  // onPosted runs for a created practice report. The post window is still
  // closing when the event arrives, so the toast and the opening wait one tick:
  // the report then opens on its trust block (the detail sheet on the phone, the
  // popup on the laptop) as if its row or its pin had been tapped.
  function onPosted(report) {
    if (!report || report.id === undefined || report.id === null) {
      return;
    }
    var id = report.id;
    var pinalert = app();
    window.setTimeout(function () {
      if (pinalert && typeof pinalert.showToast === 'function') {
        pinalert.showToast(POSTED_TOAST);
      }
      var map = window.PinalertMap;
      var opened = !!(map && typeof map.openReport === 'function' && map.openReport(id));
      if (!opened && pinalert && typeof pinalert.openReport === 'function') {
        pinalert.openReport(id);
      }
    }, 0);
    watchPractice(id);
  }

  // onEndedVisit is called by the app when the Demo API says this visit is gone
  // (404, field visit). The page starts again at /demo and keeps from=welcome
  // when it came with it; no other value of from is carried.
  function onEndedVisit() {
    var search = window.location && typeof window.location.search === 'string' ? window.location.search : '';
    var from = /(?:^|[?&])from=([^&]*)/.exec(search);
    var target = '/demo';
    if (from && from[1] === WELCOME_WORD) {
      target += '?from=' + WELCOME_WORD;
    }
    stopWatching();
    window.location.replace(target);
  }

  if (isDemoPage()) {
    document.addEventListener(REPORT_POSTED_EVENT, function (e) {
      onPosted(e && e.detail ? e.detail.report : null);
    });
    window.PinalertDemoEnded = onEndedVisit;
  }

  // One visit across the three places (plan 05-19, D-01, D-05).
  //
  // The bars of a Demo page link to the other places with ?v=<visit>, so
  // Nearby, Official alerts and My activity show one world in one visit. The
  // visit is only a hand over: as soon as a page loads it takes the v parameter
  // off its own address with history.replaceState, so a reload or a copied
  // address starts a fresh Demo (leaving or reloading starts the Demo fresh), and
  // nothing is kept in the browser. The other parameters (from, post) stay.
  //
  // The wordmark and the Report links are written by the shared bar with the
  // bare Demo address, so this file adds the visit to them from the page's own
  // data-visit, and from=welcome when the address says so. A visit of any other
  // shape than the ids the server hands out is never written into a link.
  var VISIT_PATTERN = /^[A-Za-z0-9_-]{22}$/;
  var VISIT_PARAM = 'v';

  function searchOf() {
    var loc = window.location;
    return loc && typeof loc.search === 'string' ? loc.search : '';
  }

  // dropVisit rewrites the address without the v parameter. An address without
  // it, and a page without history, are left as they are.
  function dropVisit() {
    var loc = window.location;
    var hist = window.history;
    if (!loc || !hist || typeof hist.replaceState !== 'function') {
      return;
    }
    var search = searchOf();
    var pairs = search.replace(/^\?/, '').split('&').filter(function (pair) {
      return pair !== '';
    });
    var kept = pairs.filter(function (pair) {
      return pair.split('=')[0] !== VISIT_PARAM;
    });
    if (kept.length === pairs.length) {
      return;
    }
    var path = typeof loc.pathname === 'string' ? loc.pathname : '/demo';
    var hash = typeof loc.hash === 'string' ? loc.hash : '';
    hist.replaceState(hist.state, '', path + (kept.length ? '?' + kept.join('&') : '') + hash);
  }

  // carryVisit adds the visit to the bar links that do not name one.
  function carryVisit() {
    if (typeof document.querySelector !== 'function' || typeof document.querySelectorAll !== 'function') {
      return;
    }
    var holder = document.querySelector('[data-visit]');
    var visit = holder ? holder.getAttribute('data-visit') : null;
    if (typeof visit !== 'string' || !VISIT_PATTERN.test(visit)) {
      return;
    }
    var from = /(?:^|[?&])from=welcome(?:&|$)/.test(searchOf()) ? '&from=' + WELCOME_WORD : '';
    var links = document.querySelectorAll('a[data-open-report], a.app-topbar__brand');
    for (var i = 0; i < links.length; i += 1) {
      var href = links[i].getAttribute('href');
      if (typeof href !== 'string' || href.indexOf('/demo') !== 0 || /[?&]v=/.test(href)) {
        continue;
      }
      links[i].setAttribute('href', href + (href.indexOf('?') === -1 ? '?' : '&') + VISIT_PARAM + '=' + visit + from);
    }
  }

  carryVisit();
  dropVisit();

  window.PinalertDemoPractice = {
    PRACTICE_POLL_MS: PRACTICE_POLL_MS,
    onPosted: onPosted,
    watchPractice: watchPractice,
    stopWatching: stopWatching,
    announceConfirmed: announceConfirmed,
    onEndedVisit: onEndedVisit
  };
}());
