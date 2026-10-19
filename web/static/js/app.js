// app.js — window.Pinalert, the single shared client store.
//
// Plans 01-05 and 01-06 build entirely against this surface: the one
// GET /api/reports fetch, the submit call, cross-panel selection, and the
// formatting helpers both the map and the list need for identical output.
// Nothing else in the client fetches reports — see map.js, which reads only
// from Pinalert.state.
//
// SECURITY (T-01-03): report descriptions are authored by anonymous
// strangers and rendered in other visitors' browsers. Every piece of
// report-authored data must reach the DOM through Pinalert.setText (a
// text-node assignment) or an equivalent, never through a markup string a
// browser then parses. html/template protects only the server-rendered
// shell (index.html.tmpl); this file is the whole defence for anything
// JavaScript builds afterward.
window.Pinalert = (function () {
  'use strict';

  // The nine categories a new report can use, in the server's declared order
  // (service.PostableCategories). They feed the report form's tiles. Traffic
  // took the Rescue needed slot on 2026-10-16 by owner decision.
  var CATEGORIES = [
    'flood', 'earthquake', 'fire', 'storm_cyclone', 'road_blocked',
    'power_outage', 'shelter_open', 'traffic', 'other'
  ];

  // Categories a report stored before 2026-10-16 may still carry. No new
  // report can use them, but old rows keep their label, icon and chip.
  var RETIRED_CATEGORIES = ['rescue_needed'];

  // Every category the server may send (service.Categories).
  var KNOWN_CATEGORIES = CATEGORIES.concat(RETIRED_CATEGORIES);

  var SEVERITIES = ['low', 'medium', 'critical'];

  var CAPACITY_STATUSES = ['available', 'limited', 'full', 'closed'];

  // Numeric-plus-word form, per D-05: the raw slider value "2" alone does
  // not convey "Medium" to a screen reader or a sighted reader either.
  var SEVERITY_LABELS = {
    low: '1 · Low',
    medium: '2 · Medium',
    critical: '3 · Critical'
  };

  // The plain word for the visible severity label on rows and the popup
  // (A26, A45). One table for every client surface, keys equal to SEVERITIES.
  var SEVERITY_WORDS = { low: 'Low', medium: 'Medium', critical: 'Critical' };

  var CATEGORY_LABELS = {
    flood: 'Flood',
    earthquake: 'Earthquake',
    fire: 'Fire',
    storm_cyclone: 'Storm/Cyclone damage',
    road_blocked: 'Road blocked',
    power_outage: 'Power outage',
    shelter_open: 'Shelter open',
    traffic: 'Traffic',
    rescue_needed: 'Rescue needed',
    other: 'Other'
  };

  // The words of the view parameter the store sends (03.4 D-13, D-14). active
  // stays the store default and sends no parameter, because the Low data view
  // uses the same store and keeps its built Active list. Nearby sets its own
  // show state at boot, which reads all (Live, Provisional and Hidden by
  // dispute in one answer) or disputed. The Past tab left Nearby in Phase 3.4,
  // so there is no past word any more.
  var VIEWS = ['active', 'all', 'disputed'];

  // The three choices of the Show menu (03.4 D-13). all is the default.
  var SHOWS = ['all', 'confirmed', 'disputed'];

  // The plain word of each category chip (03.1 D-17, 03.4 D-24a). Only these
  // four have a chip besides All; the other categories show under All.
  // Traffic took the Rescue chip on 2026-10-16. A rescue report posted before
  // that day shows under All until it fades.
  var CHIP_LABELS = {
    flood: 'Flood',
    traffic: 'Traffic',
    shelter_open: 'Shelter',
    power_outage: 'Power'
  };

  // The longest place name the feed shows. A longer name is cut and ends with
  // an ellipsis character, so the whole text is at most this many characters.
  var PLACE_NAME_MAX = 32;

  var shellEl = document.getElementById('app-shell');

  function readNumberAttr(name, fallback) {
    if (!shellEl) {
      return fallback;
    }
    var raw = shellEl.getAttribute(name);
    if (raw === null || raw === '') {
      return fallback;
    }
    var parsed = parseFloat(raw);
    return isNaN(parsed) ? fallback : parsed;
  }

  // Fallback centre, default radius, and poll interval, read from the
  // shell's data attributes (set server-side by handlers.Page) so none of
  // this is hard-coded in JS.
  var config = {
    fallbackLat: readNumberAttr('data-fallback-lat', 12.9716),
    fallbackLon: readNumberAttr('data-fallback-lon', 77.5946),
    fallbackZoom: readNumberAttr('data-fallback-zoom', 11),
    defaultRadiusKm: readNumberAttr('data-default-radius-km', 10),
    pollIntervalMs: readNumberAttr('data-poll-interval-ms', 30000)
  };

  var state = {
    reports: [],
    status: 'idle', // idle | loading | ok | error
    error: null,
    // hasLoaded turns true on the first list answer that is applied and never
    // goes back. lastUpdatedAt is the device clock time of that newest applied
    // answer, null before it. Both are display and flow state only.
    hasLoaded: false,
    lastUpdatedAt: null,
    selectedId: null,
    // own is true only when the centre is the reader's own position, a fix the
    // browser gave. The fallback centre is never the reader (A94).
    center: { lat: config.fallbackLat, lon: config.fallbackLon, own: false },
    // The report read the store asks for (one of VIEWS). The Low data view
    // keeps the active default (no parameter). Nearby moves to all or disputed
    // through setShow at boot, before its first request.
    view: 'active',
    // The Show choice of Nearby (one of SHOWS). all and confirmed read the
    // all answer, disputed reads the disputed answer. Confirmed is a filter
    // over the all answer, see visibleReports.
    show: 'all',
    // The Only Reliable reporters tick of the Show menu (Phase 3.2, NOTIF-04,
    // D-26). Not remembered: a fresh open starts off, and reliable.js reads
    // the page address once at boot. It is a filter over rows already in the
    // store, so turning it on or off sends no request.
    reliable: false,
    // '' (All) or one category slug. Nothing is hidden until a chip is
    // tapped (03.1 D-17), and a fresh open starts with no filter (D-18).
    category: '',
    // null, or { name, lat, lon } of the place the reader searched for.
    place: null,
    // null, or the report id string from ?report=ID. It is sent with every
    // list request while the linked report is the reader's open selection, so
    // every answer carries a fresh focus object (CR-01). It goes when the
    // server answers that the report cannot be shown, and dropFocus clears it
    // when the linked popup closes or the tab, the place or the chip changes.
    focusId: null,
    // The focus object of the newest answer, or null when that answer had
    // none. Every answer replaces it, so it is never older than the list.
    focus: null,
    // The last response's recent counts and cut flags, null when it had none.
    recent: null,
    cut: null
  };

  var listeners = [];
  var selectListeners = [];
  var pollTimer = null;
  // fetchSeq numbers every fetchReports call and lastAppliedSeq is the number
  // of the newest call whose success has been written to the store. A response
  // is dropped only when its own number is below lastAppliedSeq, that is when
  // newer data is already on screen, because on a slow link an older response
  // can land after a newer one and put back older rows, for example counters
  // from before a vote that has already been recorded (WR-01). Judging by the
  // newest request started instead would drop every response whenever latency
  // exceeds the poll interval, and the list would never update.
  var fetchSeq = 0;
  var lastAppliedSeq = 0;
  // writeFloor is the number of the newest refetch that followed a write (a
  // vote, a submitted report). It is raised when that refetch STARTS, so it
  // holds whether the refetch later succeeds or fails. A response from a call
  // started before the write may carry rows the server read before the write,
  // and is dropped even when the refetch failed and nothing newer was applied
  // (WR-01). Plain polls never raise it, so a slow link cannot freeze the list.
  var writeFloor = 0;
  // newestFetch is the promise of the most recently started call. A call whose
  // response is dropped because the tab, the chip or the centre changed hands
  // this promise back, so its caller waits for the data that will replace it.
  var newestFetch = null;
  // rowsFor is the tab, the category and the centre the rows in state.reports
  // were fetched for, so a failed refetch can tell rows that no longer match
  // the store from rows that are merely a little old (WR-08).
  var rowsFor = { view: 'active', category: '', lat: config.fallbackLat, lon: config.fallbackLon };
  // clockOffsetMs is the server's clock minus this device's clock, learned
  // from the Date header of every successful report response. Report ages and
  // the fade stage compare the server's created_at and expires_at with "now",
  // so "now" must be the server's now: a phone clock set a day behind would
  // otherwise show a report about to expire as fresh (WR-06). Zero until the
  // first response, and unchanged when a response carries no usable header.
  var clockOffsetMs = 0;
  // toastTimer is showToast's own module-level state (Task 1, 02-07): kept
  // here rather than inside the function so repeated calls collapse onto
  // one timer, exactly as modal.js's private implementation — the one this
  // function replaces — already did.
  var toastTimer = null;
  // The toast's other module level state: the lengths in milliseconds, which
  // toast is current (a late answer for an older toast is dropped), whether
  // the current one holds an action, whether a press is in flight and whether
  // the pause listeners are attached yet (once, on first use).
  var TOAST_MS = 3000;
  var TOAST_ACTION_MS = 8000;
  var TOAST_RESUME_MS = 4000;
  // The post limits (D-18): a post that gets no answer in 10 seconds counts
  // as unanswered, and these answers from a proxy also mean "try later".
  var SUBMIT_TIMEOUT_MS = 10000;
  var QUEUEABLE_STATUSES = [502, 503, 504];
  // A list request with no answer in 10 seconds counts as a failed poll.
  var LIST_TIMEOUT_MS = 10000;
  var toastSeq = 0;
  var toastHasAction = false;
  var toastPending = false;
  var toastPauseWired = false;
  // True while a mouse pointer is over the toast, kept from the pointer
  // events. :hover is not read, because on a touch screen it can stay on after
  // a tap and would keep the timer stopped for good (WR-04).
  var toastPointerInside = false;

  // The page lock (P4-A26). The ids of the two elements that must stay live
  // while a dialog holds the page: the toast and the announcer. The nodes the
  // lock silenced, so the release can restore exactly those, and the scroll
  // position the page had when it was locked.
  var LOCK_EXEMPT_IDS = ['toast', 'notice-live'];
  var lockedNodes = [];
  var lockScrollY = 0;

  // nowMs is the current time on the server's clock, as best this device can
  // tell. It is the only clock the age and fade code reads.
  function nowMs() {
    return Date.now() + clockOffsetMs;
  }

  // noteServerClock records the offset from one response's Date header. The
  // header has one second resolution and ignores transit time, which is far
  // finer than the day sized clock errors it exists to correct.
  function noteServerClock(headerValue) {
    var serverMs = headerValue ? new Date(headerValue).getTime() : NaN;
    if (isFinite(serverMs)) {
      clockOffsetMs = serverMs - Date.now();
    }
  }

  // notify calls every listener with the state. Each call is isolated: a
  // listener that throws (for example because one of the page's scripts did
  // not load) cannot stop the listeners after it, and its error never reaches
  // the caller, where fetchReports would read it as a failed request (WR-W04).
  // The error is thrown again on a timer, so it still shows in the browser's
  // own error log.
  function notify() {
    for (var i = 0; i < listeners.length; i++) {
      try {
        listeners[i](state);
      } catch (e) {
        rethrowLater(e);
      }
    }
  }

  function rethrowLater(e) {
    window.setTimeout(function () {
      throw e;
    }, 0);
  }

  // subscribe registers fn to be called with state on every transition.
  // Returns an unsubscribe function.
  function subscribe(fn) {
    listeners.push(fn);
    return function unsubscribe() {
      var idx = listeners.indexOf(fn);
      if (idx !== -1) {
        listeners.splice(idx, 1);
      }
    };
  }

  // setCenter stores the feed centre and where it came from. The caller says
  // own only for the reader's own position, so a distance read from this feed
  // is shown as "away" only when it is the reader's distance (A94).
  function setCenter(lat, lon, own) {
    state.center = { lat: lat, lon: lon, own: own === true };
  }

  // feedIsAroundReader is true when the feed was loaded around the reader's
  // own position, the only time distance_km is the reader's distance.
  function feedIsAroundReader() {
    return state.center.own === true;
  }

  // setView, setCategory, setPlace, clearPlace, setFocusId and dropFocus mirror
  // setCenter's shape exactly: a plain assignment onto state, no notify, no
  // fetch. The caller decides when to refetch. A value outside the closed
  // list is ignored, so the store can never hold a word the server would
  // answer with a 400.
  function setView(view) {
    if (VIEWS.indexOf(view) !== -1) {
      state.view = view;
    }
  }

  // setShow keeps the Show choice of Nearby (03.4 D-13, D-14) and the read it
  // needs: Show Disputed reads the disputed answer, All and Confirmed read the
  // all answer. A word outside SHOWS is ignored. Like the setters around it it
  // only assigns: no notify, no fetch. It answers true when the read changed,
  // so the caller knows to refetch; Confirmed against All is only a filter
  // over rows already in the store, so the caller redraws with redraw.
  function setShow(show) {
    if (SHOWS.indexOf(show) === -1) {
      return false;
    }
    var before = state.view;
    state.show = show;
    state.view = show === 'disputed' ? 'disputed' : 'all';
    return state.view !== before;
  }

  // setReliable keeps the Only Reliable reporters tick. Like the setters around
  // it it only assigns: no notify, no fetch. Only a true boolean turns it on.
  // It answers true when the value changed, so the caller knows to redraw.
  function setReliable(on) {
    var next = on === true;
    var changed = state.reliable !== next;
    state.reliable = next;
    return changed;
  }

  // reportInShow says whether one report belongs to a Show choice. It reads
  // two server authored words and nothing else: view (active or disputed) and
  // confirmed_by_people (the server's own answer to "confirmed by people
  // nearby", TRUST-02). The browser holds no threshold, no count and no
  // judgement of its own. A disputed report is never in Confirmed.
  function reportInShow(report, show) {
    if (!report) {
      return false;
    }
    if (show === 'confirmed') {
      return report.confirmed_by_people === true && report.view !== 'disputed';
    }
    if (show === 'disputed') {
      return report.view === 'disputed';
    }
    return true;
  }

  // reportKeptByReliable says whether the Only Reliable reporters tick lets
  // one report stay. With the tick off every report stays. With it on a report
  // stays only when the server's own word kept_by_reliable_filter is true (a
  // Reliable reporter's report, any critical report, or an old Rescue report).
  // It reads that one word and nothing else: the browser holds no tag, no
  // severity and no count of its own (TRUST-02).
  function reportKeptByReliable(report, on) {
    if (!on) {
      return true;
    }
    return !!report && report.kept_by_reliable_filter === true;
  }

  // visibleReports is the one set of reports the list rows and the map pins
  // both draw (03.4 D-13, 3.2 D-26). It is state.reports under the Show
  // choice and the Only Reliable reporters tick, as a new array, so the
  // store's own array is never changed.
  function visibleReports() {
    var out = [];
    for (var i = 0; i < state.reports.length; i++) {
      if (reportInShow(state.reports[i], state.show) &&
          reportKeptByReliable(state.reports[i], state.reliable)) {
        out.push(state.reports[i]);
      }
    }
    return out;
  }

  // disputedLast is a stable partition by the server's view word: every report
  // that is not disputed first, in the order given, then every disputed report
  // in the order given. It never judges a report itself (D-14).
  function disputedLast(reports) {
    var live = [];
    var disputed = [];
    for (var i = 0; i < reports.length; i++) {
      if (reports[i] && reports[i].view === 'disputed') {
        disputed.push(reports[i]);
      } else {
        live.push(reports[i]);
      }
    }
    return live.concat(disputed);
  }

  // redraw tells every listener to draw the store again, for a change that
  // needs no request (Confirmed against All).
  function redraw() {
    notify();
  }

  function setCategory(slug) {
    if (slug === '' || KNOWN_CATEGORIES.indexOf(slug) !== -1) {
      state.category = slug;
    }
  }

  // setPlace keeps the place the reader searched for. The name is cut to
  // PLACE_NAME_MAX characters with a trailing ellipsis character. A value
  // without a text name and two finite coordinates is ignored.
  function setPlace(place) {
    if (!place || typeof place.name !== 'string' || place.name === '' ||
        typeof place.lat !== 'number' || !isFinite(place.lat) ||
        typeof place.lon !== 'number' || !isFinite(place.lon)) {
      return;
    }
    var name = place.name;
    if (name.length > PLACE_NAME_MAX) {
      name = name.slice(0, PLACE_NAME_MAX - 1) + '\u2026';
    }
    state.place = { name: name, lat: place.lat, lon: place.lon };
  }

  function clearPlace() {
    state.place = null;
  }

  // dropFocus ends a report link: the id is no longer sent and the focus
  // object goes, so the pin the link put on the map leaves at the next render.
  // Like the setters above it only assigns: no notify, no fetch.
  function dropFocus() {
    state.focusId = null;
    state.focus = null;
  }

  // setFocusId keeps the report id of a ?report=ID link, as digits only. Every
  // list request carries it until the link ends (see state.focusId).
  function setFocusId(id) {
    if (id === null) {
      state.focusId = null;
      return;
    }
    var text = String(id);
    if (/^[1-9][0-9]{0,17}$/.test(text)) {
      state.focusId = text;
    }
  }

  // openReport opens one report on Nearby by its id (Phase 3.4 plan 39): the
  // one public way in, used by an address with ?report=ID on load, by the
  // browser's back and forward, and by any later caller. A report with a pin
  // opens at once (PinalertMap.openReport). Any other id is kept as a report
  // link and the list is read again, so the server's answer opens it, or says
  // it cannot be shown; before the first answer the first list request
  // carries it, as an address on load always did. It returns false for an id
  // that is not a report id.
  function openReport(id) {
    var text = id === null || id === undefined ? '' : String(id);
    if (!/^[1-9][0-9]{0,17}$/.test(text)) {
      return false;
    }
    if (window.PinalertMap && typeof window.PinalertMap.openReport === 'function' &&
        window.PinalertMap.openReport(text)) {
      return true;
    }
    setFocusId(text);
    if (state.hasLoaded) {
      fetchReports();
    }
    return true;
  }

  // markListLoading marks the list pane while the rows in the store belong to
  // another tab, chip or centre than the one just asked for (WR-03). The
  // heading and the note change at once on a tab press, so without the mark
  // the old rows would sit readable under the new heading until the answer
  // lands. feed.css hides the rows, the count, the empty blocks and the error
  // panel under the mark and shows the skeleton, and it hides the map's report
  // pins and the map label under the same mark. The store keeps the rows, so
  // a failed request can still tell rows that are merely a little old from
  // rows of another set (WR-08). The pane is looked up when needed, so a page
  // without it is not affected.
  function markListLoading(on) {
    var pane = document.querySelector('.pane--list');
    if (!pane) {
      return;
    }
    if (on) {
      pane.setAttribute('data-loading', 'true');
    } else {
      pane.removeAttribute('data-loading');
    }
  }

  // fetchReports performs the one and only GET /api/reports call. A
  // background refresh (status already "ok") leaves status untouched so the
  // list does not flash skeletons on every 30s poll. A refetch while rows are
  // on screen is a background refresh whatever the status is, so a poll or a
  // Retry after a failure never hides the list the reader is looking at
  // (WR-08). A call made after the first success is a background refresh
  // whatever the previous status was and whether or not rows are left on
  // screen. Only a fetch made before any list has loaded moves to the loading
  // state. A request that gets no answer within 10 seconds is abandoned and
  // counts as a failed poll, like a network failure.
  //
  // The promise resolves once the store reflects data at least as new as this
  // call started, and it never rejects. A call whose response is dropped
  // resolves at once when newer data is already applied, and otherwise with
  // the newest call's promise, so a caller that refetches after a write
  // (votes.js, the submit path) never treats the store as fresh before the
  // newest data has landed.
  function fetchReports(options) {
    var isBackgroundRefresh = state.status === 'ok' || state.reports.length > 0;
    if (state.hasLoaded === true) {
      isBackgroundRefresh = true;
    }
    var mySeq = ++fetchSeq;
    if (options && options.afterWrite === true && mySeq > writeFloor) {
      writeFloor = mySeq;
    }
    var sentView = state.view;
    var sentCategory = state.category;
    var sentLat = state.center.lat;
    var sentLon = state.center.lon;
    var sentFocusId = state.focusId;
    var pending;
    // A response is stale when newer data was applied after this call
    // started or before the newest post write refetch started, or when the
    // reader has changed the tab, the chip or the centre since this request
    // was built. A stale response must change nothing, neither rows nor the
    // error state, so rows never show under the wrong tab.
    function isStale() {
      return mySeq < lastAppliedSeq || mySeq < writeFloor || sentView !== state.view ||
        sentCategory !== state.category || sentLat !== state.center.lat || sentLon !== state.center.lon;
    }
    // rowsMatchStore is true when the rows on screen were fetched for the tab,
    // the chip and the centre the store holds now.
    function rowsMatchStore() {
      return rowsFor.view === state.view && rowsFor.category === state.category &&
        rowsFor.lat === state.center.lat && rowsFor.lon === state.center.lon;
    }
    // What a dropped call resolves with: nothing when newer data is already
    // in the store, else the newest call's promise.
    function whenFresh() {
      if (mySeq < lastAppliedSeq) {
        return undefined;
      }
      return newestFetch !== pending ? newestFetch : undefined;
    }
    if (!isBackgroundRefresh) {
      state.status = 'loading';
      notify();
    }
    // Rows of another set stay in the store but leave the screen until this
    // request or a newer one is applied. A request that goes back to the set
    // the rows belong to (a tab pressed and pressed back) shows them again.
    markListLoading(isBackgroundRefresh && !rowsMatchStore());

    var url = '/api/reports' +
      '?lat=' + encodeURIComponent(state.center.lat) +
      '&lon=' + encodeURIComponent(state.center.lon) +
      '&radius_km=' + encodeURIComponent(config.defaultRadiusKm);

    // This is the only report fetch in the client (see this file's own
    // header comment), and both the list and the map render from the one
    // state.reports array it fills, so one set of parameters on one URL
    // decides both surfaces' rows and pins together. There is no
    // map-specific query to add and none must ever be added: a second fetch
    // would be the first place the two surfaces could disagree about what
    // exists, which is exactly what TRUST-02 forbids. The view word is one of
    // two fixed strings (the store only ever holds a word from VIEWS), so it
    // needs no encoding; the category and the report id are encoded like the
    // coordinates above, and the server checks all three. Omitting a
    // parameter when it holds its default keeps the Active, unfiltered URL
    // byte-identical to Phase 1's (the Low data view still sends it that way).
    if (sentView === 'all' || sentView === 'disputed') {
      url += '&view=' + sentView;
    }
    if (sentCategory !== '') {
      url += '&category=' + encodeURIComponent(sentCategory);
    }
    if (sentFocusId !== null) {
      url += '&report=' + encodeURIComponent(sentFocusId);
    }

    // The first request of the page is flagged on the root element once, so
    // the head script can start its slow notice timer from it.
    if (!document.documentElement.hasAttribute('data-feed-requested')) {
      document.documentElement.setAttribute('data-feed-requested', 'true');
    }
    // A request that stalls is abandoned after 10 seconds and counts as a
    // failed poll. The limit is this call's own timer (requestTimeout), never
    // the request's promise: WebKit can leave an aborted request's promise
    // unsettled for good, and the stale line, a refetch after a write and the
    // next list request of the Low data view all wait on this call. So when
    // the timer fires, the call fails at once, whether or not the request ever
    // ends. The timer also aborts the request, so it stops on the network too.
    // The limit covers the wait for the headers and for the body. The signal
    // travels in a Request so the call below keeps its one argument form.
    var limit = requestTimeout(LIST_TIMEOUT_MS, 'fetchReports');
    url = new Request(url, { signal: limit.signal });

    var answered = fetch(url)
      .then(function (res) {
        if (!res.ok) {
          throw new Error('fetchReports: HTTP ' + res.status);
        }
        noteServerClock(res.headers && res.headers.get('Date'));
        return res.json();
      });
    // Whichever comes first wins: the answer, or the timer. A late end of the
    // other one changes nothing.
    pending = Promise.race([answered, limit.expired])
      .then(function (body) {
        limit.clear();
        if (isStale()) {
          return whenFresh();
        }
        state.reports = (body && body.reports) || [];
        // The three optional objects are kept as the server sent them. This
        // file renders none of them. Every answer replaces the focus object,
        // so the linked report is never drawn from an older answer than the
        // list (CR-01). A focus is taken only while the link it answers is
        // still the store's: an answer sent before the link ended must not
        // bring its pin back.
        state.recent = (body && body.recent) || null;
        state.cut = (body && body.cut) || null;
        state.focus = null;
        if (body && body.focus) {
          if (sentFocusId !== null && sentFocusId === state.focusId) {
            state.focus = body.focus;
          }
        }
        // A report the server cannot show (too old, unavailable) is a final
        // answer, so its id is not sent again.
        if (sentFocusId !== null && state.focusId === sentFocusId &&
            (state.focus === null || state.focus.status !== 'ok')) {
          state.focusId = null;
        }
        rowsFor = { view: sentView, category: sentCategory, lat: sentLat, lon: sentLon };
        lastAppliedSeq = mySeq;
        state.status = 'ok';
        state.error = null;
        state.hasLoaded = true;
        state.lastUpdatedAt = Date.now();
        markListLoading(false);
        notify();
      })
      .catch(function (err) {
        limit.clear();
        if (isStale()) {
          return whenFresh();
        }
        // Rows fetched for another tab, chip or centre must not stay on
        // screen under a strip that no longer matches them, so they are
        // dropped. Rows that are only a little old stay (WR-08).
        if (!rowsMatchStore()) {
          state.reports = [];
          state.recent = null;
          state.cut = null;
        }
        state.status = 'error';
        state.error = err;
        markListLoading(false);
        notify();
      });
    newestFetch = pending;
    return pending;
  }

  // startPolling refreshes on config.pollIntervalMs. Safe to call more than
  // once — only the first call schedules a timer.
  function startPolling() {
    if (pollTimer !== null) {
      return;
    }
    pollTimer = window.setInterval(function () {
      fetchReports();
    }, config.pollIntervalMs);
  }

  // requestTimeout gives one request a time limit of ms milliseconds that
  // ends on time even when the request does not. It is the one time limit of
  // every request in the client: the list, the post, both searches, the flag
  // and the moderation reads (Pinalert.requestTimeout).
  //
  // The limit is the page's own timer, never the request's promise: WebKit
  // can leave an aborted request's promise unsettled for good, so a limit
  // that waits for the abort to end the request can wait forever. When the
  // timer fires, expired rejects with an Error named TimeoutError, and only
  // then is the request aborted, so a caller that races its request against
  // expired fails at the limit whether or not the request ever ends, and
  // never sees the abort's own AbortError first. A caller races the body read
  // against expired too, so a body that stalls lands where an aborted body
  // read always landed. The abort stops the request on the network.
  //
  // signal goes on the request. controller ends the request early (a newer
  // search ends the older one); it is null in a browser without one, where
  // the browser's own timeout signal stops the request instead. clear stops
  // the timer once the answer, body included, has come. what names the
  // caller in the error's text, which is never shown.
  function requestTimeout(ms, what) {
    var controller = null;
    var signal;
    if (typeof AbortController !== 'undefined') {
      controller = new AbortController();
      signal = controller.signal;
    } else if (typeof AbortSignal !== 'undefined' && typeof AbortSignal.timeout === 'function') {
      signal = AbortSignal.timeout(ms);
    }
    var timer = null;
    var expired = new Promise(function (resolve, reject) {
      timer = window.setTimeout(function () {
        timer = null;
        var limitErr = new Error((what || 'request') + ': no answer in ' + (ms / 1000) + ' seconds');
        limitErr.name = 'TimeoutError';
        reject(limitErr);
        if (controller !== null) {
          controller.abort();
        }
      }, ms);
    });
    // A limit that fires after its race was already decided rejects nobody's
    // promise, so it must not count as an unhandled rejection.
    expired.catch(function () {});
    return {
      signal: signal,
      controller: controller,
      expired: expired,
      clear: function () {
        if (timer !== null) {
          window.clearTimeout(timer);
          timer = null;
        }
      }
    };
  }

  // submitReport POSTs to /api/reports, resolves with the created report or
  // rejects with an Error carrying .field/.fieldMessage from the server's
  // {field, message} shape, and refreshes the store on success. A rejection
  // also carries .status (0 when nothing answered) and .queueable (true when
  // no answer came, the request timed out or the answer was a 502, 503 or
  // 504); a 429 adds .retryAfterSeconds, .serious and .emergencyNumber
  // (P4-A13). The created report passes through unchanged, so its held flag
  // reaches the caller. A post that hangs, even one whose aborted request
  // never ends, fails at SUBMIT_TIMEOUT_MS as no answer, so the form saves
  // the report on the device and the queue's send pass ends with the row kept.
  // A body that stalls after the headers lands in the body fallback, as an
  // aborted body read always did, never in the no answer path: the post may
  // have made the report already.
  function submitReport(payload) {
    var timeout = requestTimeout(SUBMIT_TIMEOUT_MS, 'submitReport');
    return Promise.race([fetch('/api/reports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
      signal: timeout.signal
    }), timeout.expired]).then(function (res) {
      return Promise.race([res.json(), timeout.expired]).catch(function () {
        return {};
      }).then(function (body) {
        timeout.clear();
        if (!res.ok) {
          var fieldError = (body && body.error) || {
            field: 'body',
            message: 'Something went wrong. Try again.'
          };
          var err = new Error(fieldError.message);
          err.field = fieldError.field;
          err.fieldMessage = fieldError.message;
          err.status = res.status;
          err.queueable = QUEUEABLE_STATUSES.indexOf(res.status) !== -1;
          if (res.status === 429) {
            var wait = body && body.retry_after_seconds;
            if (typeof wait !== 'number' || !isFinite(wait) || wait < 0) {
              wait = parseInt(res.headers.get('Retry-After'), 10);
            }
            err.retryAfterSeconds = (typeof wait === 'number' && isFinite(wait) && wait >= 0) ? wait : 60;
            err.serious = !!body && body.serious === true;
            var number = body && body.emergency_number;
            err.emergencyNumber = (err.serious && typeof number === 'string' && number !== '') ? number : '';
          }
          throw err;
        }
        return (body && body.report) || null;
      });
    }, function () {
      timeout.clear();
      var err = new Error('Something went wrong. Try again.');
      err.field = 'body';
      err.fieldMessage = 'Something went wrong. Try again.';
      err.status = 0;
      err.queueable = true;
      throw err;
    }).then(function (report) {
      return fetchReports({ afterWrite: true }).then(function () {
        return report;
      }, function () {
        return report;
      });
    });
  }

  // select carries a selection between panels. source is 'map' or 'list' so
  // neither panel echoes an event it originated back to itself.
  function select(id, source) {
    state.selectedId = id;
    for (var i = 0; i < selectListeners.length; i++) {
      selectListeners[i](id, source);
    }
  }

  function onSelect(fn) {
    selectListeners.push(fn);
    return function unsubscribe() {
      var idx = selectListeners.indexOf(fn);
      if (idx !== -1) {
        selectListeners.splice(idx, 1);
      }
    };
  }

  function severityClass(report) {
    var severity = report && report.severity;
    if (SEVERITIES.indexOf(severity) === -1) {
      severity = 'low';
    }
    return 'sev-' + severity;
  }

  // iconClass validates category against KNOWN_CATEGORIES before building a class
  // name (T-01-17): an unexpected value from the API can never be reflected
  // into className unchecked. Falls back to the "other" category for
  // anything unrecognised. Returns the fixed base class plus the
  // category-specific class, space-separated, so a caller can assign the
  // pair straight to an element's className.
  function iconClass(category) {
    var validated = KNOWN_CATEGORIES.indexOf(category) === -1 ? 'other' : category;
    return 'icon-glyph icon-glyph--' + validated;
  }

  // categoryLabel gives the display word for a category: the table entry for
  // an own property, else the Other label. An unexpected API value is never
  // printed raw, and an inherited key such as constructor is not a category.
  function categoryLabel(category) {
    if (Object.prototype.hasOwnProperty.call(CATEGORY_LABELS, category)) {
      return CATEGORY_LABELS[category];
    }
    return CATEGORY_LABELS.other;
  }

  // formatShelterLine is the one shelter status and headcount line for the
  // feed meta and the popup meta (A24). It returns an empty string for any
  // report that is not a shelter. The status word is built only for a value
  // in the closed status list and the headcount only for an integer, so an
  // unexpected payload value never reaches the text (T-01-17).
  function formatShelterLine(report) {
    if (!report || report.category !== 'shelter_open') {
      return '';
    }
    var parts = [];
    var status = report.shelter_capacity_status;
    if (CAPACITY_STATUSES.indexOf(status) !== -1) {
      parts.push(status.charAt(0).toUpperCase() + status.slice(1));
    }
    var headcount = report.shelter_headcount;
    if (Number.isInteger(headcount)) {
      parts.push(headcount + (headcount === 1 ? ' person' : ' people'));
    }
    return parts.join(' · ');
  }

  function relativeTime(iso) {
    var then = new Date(iso).getTime();
    var now = nowMs();
    var diffSeconds = Math.max(0, Math.round((now - then) / 1000));

    if (diffSeconds < 60) {
      return 'just now';
    }
    var diffMinutes = Math.round(diffSeconds / 60);
    if (diffMinutes < 60) {
      return diffMinutes + ' min ago';
    }
    var diffHours = Math.round(diffMinutes / 60);
    if (diffHours < 24) {
      return diffHours + (diffHours === 1 ? ' hour ago' : ' hours ago');
    }
    var diffDays = Math.round(diffHours / 24);
    return diffDays + (diffDays === 1 ? ' day ago' : ' days ago');
  }

  // formatWait words a wait the server announced, rounded up: N seconds under
  // a minute, N minutes under an hour, else N hours, with the singular at 1.
  // These are the same words the server uses in its rate limit sentences.
  function formatWait(seconds) {
    var s = Math.ceil(Number(seconds));
    if (!isFinite(s) || s < 1) {
      s = 1;
    }
    if (s < 60) {
      return s + (s === 1 ? ' second' : ' seconds');
    }
    if (s < 3600) {
      var minutes = Math.ceil(s / 60);
      return minutes + (minutes === 1 ? ' minute' : ' minutes');
    }
    var hours = Math.ceil(s / 3600);
    return hours + (hours === 1 ? ' hour' : ' hours');
  }

  function updatedAgo(timestampMs) {
    // The age of the last applied list answer for the stale line, in whole
    // units rounded down, so time already passed is never overstated. It reads
    // the device clock, the same clock lastUpdatedAt was taken on, because the
    // line measures an interval on one clock and the server corrected clock
    // shifts whenever a response teaches a new offset.
    if (typeof timestampMs !== 'number' || !isFinite(timestampMs)) {
      return 'just now';
    }
    var minutes = Math.floor(Math.max(0, Date.now() - timestampMs) / 60000);
    if (minutes < 1) {
      return 'just now';
    }
    if (minutes === 1) {
      return '1 minute ago';
    }
    if (minutes < 60) {
      return minutes + ' minutes ago';
    }
    var hours = Math.floor(minutes / 60);
    return hours === 1 ? '1 hour ago' : hours + ' hours ago';
  }

  function writtenLine(report) {
    // The one Written line for every surface (the feed row, the popup, later
    // the Low data row and the moderator row). The server sends written_at only
    // when the report was uploaded more than 30 minutes after it was written, so
    // no threshold is applied here. It is display only: it is never an input to
    // the age stage, the order or the expiry, which all count from upload.
    if (!report || !report.written_at) {
      return '';
    }
    return 'Written ' + relativeTime(report.written_at);
  }

  // formatDistance turns a feed report's distance_km into the "away" words of
  // the popup meta line (A94): one decimal under 10 km, "under 100 m" below a
  // tenth of a km and a whole number from 10 km. It returns an empty string for
  // a missing or invalid value, so nothing is invented for it.
  function formatDistance(km) {
    if (typeof km !== 'number' || !isFinite(km) || km < 0) {
      return '';
    }
    if (km < 0.1) {
      return 'under 100 m away';
    }
    if (km < 9.95) {
      return km.toFixed(1) + ' km away';
    }
    return Math.round(km) + ' km away';
  }

  // distanceText is the one distance sentence of a row, a card and a popup
  // (C20, C21). The browser only formats: the server owns the number, and for
  // an approximate report the coarse point that number was measured to (D-23).
  // The words depend on what the feed is centred on:
  //   an approximate report   "about {n} km away" (the reader's own centre) or
  //                           "about {n} km from {Place}"; n is a whole number
  //                           of km, at least 1, never "under 100 m"
  //   a searched place        "{d} km from {Place}" or "under 100 m from {Place}"
  //   the reader's own centre formatDistance's "away" words
  //   any other centre        '' (the fallback centre is nobody's position)
  function distanceText(report) {
    var km = report ? report.distance_km : null;
    if (typeof km !== 'number' || !isFinite(km) || km < 0) {
      return '';
    }
    var place = state.place;
    if (report.approximate === true) {
      var whole = Math.max(1, Math.round(km));
      if (place) {
        return 'about ' + whole + ' km from ' + place.name;
      }
      return state.center.own === true ? 'about ' + whole + ' km away' : '';
    }
    if (place) {
      if (km < 0.1) {
        return 'under 100 m from ' + place.name;
      }
      var amount = km < 9.95 ? km.toFixed(1) : String(Math.round(km));
      return amount + ' km from ' + place.name;
    }
    if (state.center.own === true) {
      return formatDistance(km);
    }
    return '';
  }

  // formatFade is the fade line of the popup (A95), from the report's own
  // expires_at on the server's clock, like relativeTime. One hour or more
  // reads in whole hours rounded down, under an hour in minutes rounded up,
  // and an expired report or a missing expires_at reads nothing. It never says
  // that a confirmation extends a report's life, because nothing updates
  // expires_at after creation (T-03-31-03).
  function formatFade(report) {
    var expires = new Date(report && report.expires_at).getTime();
    if (!isFinite(expires)) {
      return '';
    }
    var remainingMs = expires - nowMs();
    if (remainingMs <= 0) {
      return '';
    }
    var hourMs = 3600000;
    if (remainingMs >= hourMs) {
      var hours = Math.floor(remainingMs / hourMs);
      return 'Fades in ' + hours + (hours === 1 ? ' hr.' : ' hrs.');
    }
    return 'Fades in ' + Math.ceil(remainingMs / 60000) + ' min.';
  }

  // ageStage implements D-15/D-17: a report fades over roughly the last
  // quarter of its lifetime, expressed as three discrete desaturation
  // stages. remainingFraction > 0.25 -> fresh; 0.125 <= remainingFraction
  // <= 0.25 -> aging; remainingFraction < 0.125 -> stale.
  function ageStage(report) {
    var created = new Date(report.created_at).getTime();
    var expires = new Date(report.expires_at).getTime();
    var now = nowMs();
    var lifetime = expires - created;
    if (lifetime <= 0) {
      return 'stale';
    }
    var remainingFraction = (expires - now) / lifetime;
    if (remainingFraction > 0.25) {
      return 'fresh';
    }
    if (remainingFraction >= 0.125) {
      return 'aging';
    }
    return 'stale';
  }

  // setText assigns value to el as a text node only — see the file-level
  // security note above. Every report-authored string must go through this
  // (or an equivalent .textContent assignment) rather than a markup-parsing
  // sink.
  function setText(el, value) {
    el.textContent = value === null || value === undefined ? '' : String(value);
  }

  // showToast (Task 1, 02-07) is modal.js's own toast implementation,
  // promoted here verbatim so app.js becomes the toast element's single
  // owner — the profile page loads main.css and app.js but not modal.css/
  // modal.js, and a page with no modal must not link the modal module just
  // to get one shared component. The element is resolved LAZILY, inside
  // this function, rather than cached at module-evaluation time: that
  // keeps this module inert on a page that has no #toast element at all,
  // with no error thrown — the call simply no-ops.
  //
  // The optional second argument (03.1 plan 14, D-22) is { label, ariaLabel,
  // onClick }. With it the toast holds a text span and one button, stays for
  // 8 seconds instead of 3, and its timer is paused while the pointer or
  // focus is on it and restarts at 4 seconds on leave. Focus is never moved
  // into the toast, so a timer is never the only route to the action (the
  // persistent control in the opened report is the keyboard route). A press
  // calls onClick, which returns the sentence to show next (or a promise of
  // it); the toast then holds that sentence with no action for 3 seconds. A
  // call with one argument is exactly what it was: text only, 3 seconds.
  //
  // The optional third argument is the length in milliseconds of a plain
  // toast, used when it is a positive number and otherwise the default of 3
  // seconds. An action toast always keeps its own 8 seconds. A caller with no
  // action passes null in the second place. While a toast shows, the root
  // element carries data-toast-open, which the notice card reads to hide.
  function showToast(message, action, durationMs) {
    var toastEl = document.getElementById('toast');
    if (!toastEl) {
      return;
    }
    var hasAction = !!(action && action.label && typeof action.onClick === 'function');
    toastSeq += 1;
    var mine = toastSeq;
    toastHasAction = hasAction;
    toastPending = false;
    // A toast that was hidden under the mouse got no pointerleave, so the
    // flag starts clear for every new toast; a real hover sets it again.
    toastPointerInside = false;
    if (hasAction) {
      setText(toastEl, '');
      toastEl.classList.add('toast--action');
      var textEl = document.createElement('span');
      textEl.className = 'toast__text';
      setText(textEl, message);
      var actionEl = document.createElement('button');
      actionEl.type = 'button';
      actionEl.className = 'toast__action';
      setText(actionEl, action.label);
      if (action.ariaLabel) {
        actionEl.setAttribute('aria-label', action.ariaLabel);
      }
      actionEl.addEventListener('click', function () {
        if (toastPending || mine !== toastSeq) {
          return;
        }
        toastPending = true;
        actionEl.disabled = true;
        if (toastTimer) {
          window.clearTimeout(toastTimer);
          toastTimer = null;
        }
        function showResult(next) {
          if (mine !== toastSeq) {
            return;
          }
          if (typeof next === 'string' && next !== '') {
            showToast(next);
          } else {
            toastPending = false;
            toastEl.hidden = true;
            document.documentElement.removeAttribute('data-toast-open');
          }
        }
        var outcome = null;
        try {
          outcome = action.onClick();
        } catch (e) {
          outcome = null;
        }
        Promise.resolve(outcome).then(showResult, function () {
          showResult(null);
        });
      });
      toastEl.appendChild(textEl);
      toastEl.appendChild(actionEl);
    } else {
      toastEl.classList.remove('toast--action');
      setText(toastEl, message);
    }
    toastEl.hidden = false;
    document.documentElement.setAttribute('data-toast-open', 'true');
    if (!toastPauseWired) {
      toastPauseWired = true;
      toastEl.addEventListener('pointerenter', pauseToast);
      toastEl.addEventListener('focusin', pauseToast);
      toastEl.addEventListener('pointerleave', resumeToast);
      toastEl.addEventListener('focusout', resumeToast);
    }
    var plainMs = (typeof durationMs === 'number' && durationMs > 0) ? durationMs : TOAST_MS;
    startToastTimer(toastEl, hasAction ? TOAST_ACTION_MS : plainMs);
  }

  // startToastTimer (re)starts the one hide timer.
  function startToastTimer(toastEl, ms) {
    if (toastTimer) {
      window.clearTimeout(toastTimer);
    }
    toastTimer = window.setTimeout(function () {
      toastTimer = null;
      toastEl.hidden = true;
      document.documentElement.removeAttribute('data-toast-open');
    }, ms);
  }

  // pauseToast stops the hide timer while the reader has a mouse pointer or
  // focus on a toast that holds an action. A plain toast keeps its fixed 3
  // seconds. Only a mouse pauses from the pointer: a touch or a pen fires
  // pointerenter as it lands, and a tap on the text must not stop the timer
  // (WR-04). Focus pauses for every kind of input.
  function pauseToast(evt) {
    if (evt && evt.type === 'pointerenter') {
      if (evt.pointerType !== 'mouse') {
        return;
      }
      toastPointerInside = true;
    }
    if (!toastHasAction || toastPending) {
      return;
    }
    if (toastTimer) {
      window.clearTimeout(toastTimer);
      toastTimer = null;
    }
  }

  // resumeToast restarts the hide timer at 4 seconds when the mouse or the
  // focus leaves a visible toast that holds an action and the timer was
  // paused. It does nothing while either is still on the toast, nothing for a
  // request in flight, and nothing while the timer still runs (a touch that
  // lifts never paused it, so it keeps its own time).
  function resumeToast(evt) {
    var toastEl = evt.currentTarget;
    if (evt.type === 'pointerleave') {
      toastPointerInside = false;
    }
    if (!toastHasAction || toastPending || toastEl.hidden || toastTimer !== null) {
      return;
    }
    if (toastPointerInside || toastEl.contains(document.activeElement)) {
      return;
    }
    startToastTimer(toastEl, TOAST_RESUME_MS);
  }

  // lockOutsideNode silences one element that is outside the dialog: it takes
  // the element out of the tab order and away from assistive technology. The
  // toast and the announcer are never silenced; a container that holds one of
  // them is not silenced either, its children are handled one by one so a
  // live region inside it keeps speaking. An element that already carries the
  // state is left alone and not recorded, so the release never clears a state
  // some other code set.
  function lockOutsideNode(node) {
    if (!node || node.nodeType !== 1 || LOCK_EXEMPT_IDS.indexOf(node.id) !== -1) {
      return;
    }
    var tag = node.tagName;
    if (tag === 'SCRIPT' || tag === 'STYLE' || tag === 'LINK' || tag === 'TEMPLATE') {
      return;
    }
    for (var i = 0; i < LOCK_EXEMPT_IDS.length; i++) {
      var exempt = document.getElementById(LOCK_EXEMPT_IDS[i]);
      if (exempt && node.contains(exempt)) {
        var kids = node.children;
        for (var k = 0; k < kids.length; k++) {
          lockOutsideNode(kids[k]);
        }
        return;
      }
    }
    if (node.inert === true || node.getAttribute('aria-hidden') === 'true') {
      return;
    }
    if ('inert' in HTMLElement.prototype) {
      node.inert = true;
      lockedNodes.push({ node: node, attr: 'inert' });
    } else {
      node.setAttribute('aria-hidden', 'true');
      lockedNodes.push({ node: node, attr: 'aria-hidden' });
    }
  }

  // lockPage locks the page behind a dialog (P4-A26). It walks from the
  // backdrop up to the body and silences every sibling on the way, never the
  // body's children wholesale, because on the main page the backdrop and the
  // toast sit inside the same container. It also marks the root element so the
  // stylesheet stops the page scrolling and the Escape listener of the trust
  // module stands aside.
  function lockPage(backdrop) {
    if (lockedNodes.length > 0) {
      unlockPage();
    }
    if (!backdrop) {
      return;
    }
    lockScrollY = window.scrollY || 0;
    var onPath = backdrop;
    while (onPath && onPath !== document.body) {
      var parent = onPath.parentElement;
      if (!parent) {
        break;
      }
      var siblings = parent.children;
      for (var i = 0; i < siblings.length; i++) {
        if (siblings[i] !== onPath) {
          lockOutsideNode(siblings[i]);
        }
      }
      onPath = parent;
    }
    document.documentElement.setAttribute('data-modal-open', 'true');
  }

  // unlockPage undoes lockPage exactly: it restores only the elements the lock
  // silenced, clears the root marker and puts the scroll position back.
  function unlockPage() {
    for (var i = 0; i < lockedNodes.length; i++) {
      var entry = lockedNodes[i];
      if (entry.attr === 'inert') {
        entry.node.inert = false;
        entry.node.removeAttribute('inert');
      } else {
        entry.node.removeAttribute('aria-hidden');
      }
    }
    lockedNodes = [];
    document.documentElement.removeAttribute('data-modal-open');
    if (window.scrollY !== lockScrollY) {
      window.scrollTo(0, lockScrollY);
    }
  }

  return {
    CATEGORIES: CATEGORIES,
    KNOWN_CATEGORIES: KNOWN_CATEGORIES,
    SEVERITIES: SEVERITIES,
    CAPACITY_STATUSES: CAPACITY_STATUSES,
    SEVERITY_LABELS: SEVERITY_LABELS,
    SEVERITY_WORDS: SEVERITY_WORDS,
    CATEGORY_LABELS: CATEGORY_LABELS,
    CHIP_LABELS: CHIP_LABELS,
    VIEWS: VIEWS,
    SHOWS: SHOWS,
    categoryLabel: categoryLabel,
    config: config,
    state: state,
    subscribe: subscribe,
    setCenter: setCenter,
    feedIsAroundReader: feedIsAroundReader,
    setView: setView,
    setShow: setShow,
    setReliable: setReliable,
    reportInShow: reportInShow,
    reportKeptByReliable: reportKeptByReliable,
    visibleReports: visibleReports,
    disputedLast: disputedLast,
    redraw: redraw,
    setCategory: setCategory,
    setPlace: setPlace,
    clearPlace: clearPlace,
    setFocusId: setFocusId,
    openReport: openReport,
    dropFocus: dropFocus,
    fetchReports: fetchReports,
    startPolling: startPolling,
    submitReport: submitReport,
    requestTimeout: requestTimeout,
    select: select,
    onSelect: onSelect,
    severityClass: severityClass,
    iconClass: iconClass,
    formatShelterLine: formatShelterLine,
    relativeTime: relativeTime,
    updatedAgo: updatedAgo,
    formatWait: formatWait,
    writtenLine: writtenLine,
    formatDistance: formatDistance,
    distanceText: distanceText,
    formatFade: formatFade,
    serverNow: nowMs,
    ageStage: ageStage,
    setText: setText,
    showToast: showToast,
    lockPage: lockPage,
    unlockPage: unlockPage
  };
}());
