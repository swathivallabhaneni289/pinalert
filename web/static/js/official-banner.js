// official-banner.js: the Nearby banner (Phase 5, plan 15, ALERT-03, ALERT-04,
// D-07, D-08, D-09, D-16, UI-SPEC Surface 2).
//
// One slim line at the top of the Nearby list tells the reader, in a few words,
// that a Severe or Extreme official alert covers the place they are looking at:
// the level icon, "Cyclone warning", a dot, "Krishna district", "+2" when more
// are eligible, a chevron. The whole row is one link to Official alerts.
//
// The server picks; this script only draws (D-16). GET <base>/official/alerts
// answers a view whose banner field is the pick: a Severe or Extreme alert on
// the reader's district, or an Extreme alert across the reader's state, only
// while the feed is fresh. The script adds one more gate of its own and trusts
// nothing else: it shows the banner only for a view whose state is exactly
// fresh and whose pick is a Severe or Extreme alert with words. A stale or
// unavailable view, a missing pick, a failed request, an HTTP error or an
// answer that is not shaped like the route's all leave the banner hidden, so an
// old alert is never shown as current (ALERT-04, T-05-80).
//
// Trust boundary (T-05-81). The event phrase and the area are written with
// textContent only. No markup is built from a string.
//
// The place (D-09). app.js announces pinalert:feed-centre on the document after
// each report list it applied, with {lat, lon}: the reader's fix or the place
// they searched. The banner follows it: a changed place is fetched once after a
// short pause (at most one request a second), the same place again is not
// fetched, and the answer is refreshed every five minutes (T-05-83). The tab
// keeps the place under pinalert.nearbyCentre, keyed by account, so Official
// alerts asks about the same place (T-05-82); a page with no account (the Demo)
// stores nothing.
//
// Announcing. When the banner first shows, or moves to a more serious alert,
// the polite region #notice-live says "Official alert: Cyclone warning,
// Krishna district." once. A refresh of the same alert says nothing. Nothing
// here makes a sound.
//
// Reuse. The script reads its API base like official-alerts.js, so the Demo
// (window.PinalertApiBase) runs the same code against its own sandbox.
(function () {
  'use strict';

  var REFRESH_MS = 300000;
  var NEARBY_CENTRE_KEY = 'pinalert.nearbyCentre';
  var FEED_CENTRE_EVENT = 'pinalert:feed-centre';
  var DEFAULT_BASE = '/api';
  // A changed place waits this long for more movement before it is fetched.
  var SETTLE_MS = 500;
  // No two fetches for a changed place are closer than this.
  var MIN_GAP_MS = 1000;
  // A request with no answer, body included, is given up after this long
  // (Phase 5 review fix WR-B01, the same limit as the report list in app.js).
  var REQUEST_LIMIT_MS = 10000;

  var bannerEl = document.getElementById('alert-banner');
  var levelEl = document.getElementById('alert-banner-level');
  var eventEl = document.getElementById('alert-banner-event');
  var areaEl = document.getElementById('alert-banner-area');
  var moreEl = document.getElementById('alert-banner-more');
  var moreSrEl = document.getElementById('alert-banner-more-sr');
  var shellEl = document.getElementById('app-shell');
  // The Demo tag exists on the Demo's Nearby only (plan 05-19). It is optional:
  // the real page has no such element and keeps drawing its banner without it.
  var demoEl = document.getElementById('alert-banner-demo');

  var required = [bannerEl, levelEl, eventEl, areaEl, moreEl, moreSrEl];
  for (var i = 0; i < required.length; i += 1) {
    if (!required[i]) {
      return;
    }
  }

  var wanted = null; // the place Nearby last announced, {lat, lon}
  var fetched = null; // the place the last request was made for
  var lastFetchAt = 0;
  var settleTimer = null;
  var refreshTimer = null;
  var requestSeq = 0;
  var shownRank = 0; // 0 hidden, 1 Severe, 2 Extreme: what the last draw showed

  // ---- The pick -----------------------------------------------------------

  function rankOf(level) {
    if (level === 'extreme') {
      return 2;
    }
    if (level === 'severe') {
      return 1;
    }
    return 0;
  }

  function text(value) {
    return typeof value === 'string' ? value : '';
  }

  // pickOf is the pick to draw, or null. Only a fresh view with a Severe or
  // Extreme pick that has words to show gives one.
  function pickOf(view) {
    if (!view || typeof view !== 'object' || view.state !== 'fresh') {
      return null;
    }
    var banner = view.banner;
    if (!banner || typeof banner !== 'object') {
      return null;
    }
    var rank = rankOf(banner.level);
    var event = text(banner.event_phrase);
    var area = text(banner.area);
    if (rank === 0 || event === '' || area === '') {
      return null;
    }
    var word = text(banner.level_word);
    if (word === '') {
      word = rank === 2 ? 'Extreme' : 'Severe';
    }
    var more = banner.more;
    if (typeof more !== 'number' || !isFinite(more) || more < 1) {
      more = 0;
    }
    return { rank: rank, word: word, event: event, area: area, more: Math.floor(more), demo: banner.demo === true };
  }

  // ---- Drawing ------------------------------------------------------------

  // hideQuiet hides the banner and keeps what was last shown, so the same
  // alert coming back after a failed request is not announced again.
  function hideQuiet() {
    bannerEl.hidden = true;
    if (demoEl) {
      demoEl.hidden = true;
    }
  }

  function hide() {
    hideQuiet();
    shownRank = 0;
  }

  // announce says the one sentence of UI-SPEC 2b in the polite region, only when
  // the banner first shows or moves to a more serious alert.
  function announce(pick) {
    var more = pick.rank > shownRank;
    shownRank = pick.rank;
    if (!more) {
      return;
    }
    var live = document.getElementById('notice-live');
    if (live) {
      live.textContent = 'Official alert: ' + pick.event + ', ' + pick.area + '.';
    }
  }

  // render draws one view. The banner is a link whose name is its content: the
  // hidden level first, then the visible words, then the hidden count.
  function render(view) {
    var pick = pickOf(view);
    if (!pick) {
      hide();
      return;
    }
    bannerEl.setAttribute('class', pick.rank === 2 ? 'alert-banner alert-banner--extreme' : 'alert-banner alert-banner--severe');
    levelEl.textContent = pick.word + '. ';
    eventEl.textContent = pick.event;
    areaEl.textContent = pick.area;
    if (pick.more > 0) {
      moreEl.textContent = '+' + pick.more;
      moreEl.hidden = false;
      moreSrEl.textContent = pick.more + (pick.more === 1 ? ' more alert.' : ' more alerts.');
    } else {
      moreEl.textContent = '';
      moreEl.hidden = true;
      moreSrEl.textContent = '';
    }
    // The Demo tag shows only for the server's sample alert (banner.demo is the
    // boolean true), never for a real one.
    if (demoEl) {
      demoEl.hidden = !pick.demo;
    }
    bannerEl.hidden = false;
    announce(pick);
  }

  // ---- The request --------------------------------------------------------

  function validPoint(lat, lon) {
    return typeof lat === 'number' && typeof lon === 'number' && isFinite(lat) && isFinite(lon) &&
      Math.abs(lat) <= 90 && Math.abs(lon) <= 180;
  }

  function apiBase() {
    var base = window.PinalertApiBase;
    if (typeof base !== 'string' || base === '') {
      base = shellEl ? text(shellEl.getAttribute('data-api-base')) : '';
    }
    if (base === '') {
      base = DEFAULT_BASE;
    }
    return base.replace(/\/+$/, '');
  }

  // fetchBanner asks the route about one place and draws the answer. An answer
  // that is not the newest request's is dropped, and any failure hides the
  // banner: a feed that cannot be reached cannot vouch for an alert.
  function fetchBanner(lat, lon) {
    requestSeq += 1;
    var mine = requestSeq;
    if (typeof fetch !== 'function') {
      hideQuiet();
      return;
    }
    var target = apiBase() + '/official/alerts?lat=' + encodeURIComponent(String(lat)) +
      '&lon=' + encodeURIComponent(String(lon));
    // The time limit (Phase 5 review fix WR-B01) is the page's own timer, never
    // the request's promise, which WebKit can leave unsettled: at the limit the
    // request is aborted and the banner hides, as for any failed request.
    var controller = typeof AbortController === 'function' ? new AbortController() : null;
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = setTimeout(function () {
        timer = null;
        reject(new Error('official banner: no answer in time'));
        if (controller) {
          controller.abort();
        }
      }, REQUEST_LIMIT_MS);
    });
    expired.catch(function () {});
    var init = { credentials: 'same-origin', headers: { Accept: 'application/json' } };
    if (controller) {
      init.signal = controller.signal;
    }
    var answered = fetch(target, init).then(function (res) {
      if (!res.ok) {
        throw new Error('status ' + res.status);
      }
      return Promise.race([res.json(), expired]);
    });
    Promise.race([answered, expired]).then(function (body) {
      stopTimer(timer);
      if (mine === requestSeq) {
        render(body);
      }
    }, function () {
      stopTimer(timer);
      if (mine === requestSeq) {
        hideQuiet();
      }
    });
  }

  function stopTimer(timer) {
    if (timer !== null) {
      clearTimeout(timer);
    }
  }

  // ---- The place ----------------------------------------------------------

  // rememberCentre keeps the place for this tab and this account, so Official
  // alerts asks about the same place. A page with no account stores nothing, and
  // a storage that refuses is silent.
  function rememberCentre(lat, lon) {
    var account = shellEl ? text(shellEl.getAttribute('data-account-id')) : '';
    if (account === '') {
      return;
    }
    try {
      window.sessionStorage.setItem(NEARBY_CENTRE_KEY, JSON.stringify({ account: account, lat: lat, lon: lon }));
    } catch (err) {
      // The tab keeps no centre; Official alerts then falls back to the stored fix.
    }
  }

  function samePlace(a, b) {
    return !!a && !!b && a.lat === b.lat && a.lon === b.lon;
  }

  function settle() {
    settleTimer = null;
    if (!wanted || samePlace(wanted, fetched)) {
      return;
    }
    fetched = wanted;
    lastFetchAt = Date.now();
    fetchBanner(fetched.lat, fetched.lon);
  }

  function refresh() {
    if (!fetched) {
      return;
    }
    lastFetchAt = Date.now();
    fetchBanner(fetched.lat, fetched.lon);
  }

  function onCentre(event) {
    var detail = event && event.detail;
    if (!detail || !validPoint(detail.lat, detail.lon)) {
      return;
    }
    wanted = { lat: detail.lat, lon: detail.lon };
    rememberCentre(wanted.lat, wanted.lon);
    if (refreshTimer === null) {
      refreshTimer = setInterval(refresh, REFRESH_MS);
    }
    if (settleTimer !== null) {
      clearTimeout(settleTimer);
      settleTimer = null;
    }
    if (samePlace(wanted, fetched)) {
      return;
    }
    // The alert on show was fetched for another place: it goes now, before the
    // new place is asked about, so it never sits over a place it was not
    // fetched for (Phase 5 review fix WR-B01). The rank is kept, so the same
    // alert coming back for the new place is not announced again.
    hideQuiet();
    var delay = Math.max(SETTLE_MS, lastFetchAt + MIN_GAP_MS - Date.now());
    settleTimer = setTimeout(settle, delay);
  }

  // follow starts listening for the place Nearby is showing.
  function follow() {
    document.addEventListener(FEED_CENTRE_EVENT, onCentre);
  }

  window.PinalertOfficialBanner = { render: render };

  follow();
}());
